// Package tgclient connects the TUI to a Telegram user account through
// gotd/td. The bridge owns all network state and posts domain events to the UI;
// the UI never touches MTProto types.
package tgclient

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	"tg-matrix-tui/internal/domain"
	"tg-matrix-tui/internal/store"
)

type Bridge struct {
	cfg Config

	events    chan Event
	authInput chan string

	mu       sync.RWMutex
	client   *telegram.Client
	api      *tg.Client
	peers    map[string]tg.InputPeerClass
	chats    map[string]domain.Chat
	folders  []domain.Folder
	selfID   int64
	store    *store.Store
	storeErr error

	ctx     context.Context
	cancel  context.CancelFunc
	started bool
}

func New(cfg Config) *Bridge {
	var (
		st  *store.Store
		err error
	)
	if cfg.DBPath != "" {
		st, err = store.Open(cfg.DBPath)
	}
	return &Bridge{
		cfg:       cfg,
		events:    make(chan Event, 128),
		authInput: make(chan string, 1),
		peers:     make(map[string]tg.InputPeerClass),
		chats:     make(map[string]domain.Chat),
		store:     st,
		storeErr:  err,
	}
}
func (b *Bridge) Events() <-chan Event { return b.events }

func (b *Bridge) Start() {
	b.mu.Lock()
	if b.started {
		b.mu.Unlock()
		return
	}
	b.started = true
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.mu.Unlock()
	go b.run()
}

func (b *Bridge) Close() {
	b.mu.Lock()
	cancel := b.cancel
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	_ = b.store.Close()
}

func (b *Bridge) SubmitAuth(value string) {
	select {
	case b.authInput <- strings.TrimSpace(value):
	default:
	}
}

func (b *Bridge) context() context.Context {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ctx
}

func (b *Bridge) emit(e Event) {
	ctx := b.context()
	if ctx == nil {
		return
	}
	select {
	case b.events <- e:
	case <-ctx.Done():
	}
}

func (b *Bridge) ask(ctx context.Context, prompt AuthPrompt) (string, error) {
	// Drop a stale answer if the user submitted while no prompt was active.
	select {
	case <-b.authInput:
	default:
	}
	select {
	case b.events <- prompt:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	select {
	case v := <-b.authInput:
		if strings.TrimSpace(v) == "" {
			return "", errors.New("empty input")
		}
		return strings.TrimSpace(v), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (b *Bridge) run() {
	ctx := b.context()
	if b.storeErr != nil {
		b.emit(StatusEvent{Text: "cache error", Err: b.storeErr})
	} else if b.store != nil {
		if chats, err := b.store.LoadChats(); err == nil && len(chats) > 0 {
			folders, _ := b.store.LoadFolders()
			b.emit(DialogsEvent{Chats: chats, Folders: folders, Cached: true})
		}
	}
	dispatcher := tg.NewUpdateDispatcher()
	loggedIn := qrlogin.OnLoginToken(&dispatcher)
	b.registerHandlers(dispatcher)

	waiter := floodwait.NewWaiter().
		WithMaxWait(10 * time.Minute).
		WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
			b.emit(StatusEvent{Text: fmt.Sprintf("flood wait %s", wait.Duration.Round(time.Second))})
		})

	opts := telegram.Options{
		SessionStorage: &session.FileStorage{Path: b.cfg.SessionPath},
		UpdateHandler:  dispatcher,
		Middlewares:    []telegram.Middleware{waiter},
		DC:             b.cfg.DC,
	}
	if b.cfg.TestDC {
		opts.DCList = dcs.Test()
	}
	client := telegram.NewClient(b.cfg.AppID, b.cfg.AppHash, opts)
	b.mu.Lock()
	b.client = client
	b.mu.Unlock()

	err := waiter.Run(ctx, func(ctx context.Context) error {
		return client.Run(ctx, func(ctx context.Context) error {
			b.emit(StatusEvent{Text: "connecting"})
			status, err := client.Auth().Status(ctx)
			if err != nil {
				return fmt.Errorf("auth status: %w", err)
			}
			if !status.Authorized {
				b.emit(StatusEvent{Text: "sign in"})
				method, err := b.ask(ctx, AuthPrompt{Kind: AuthMethod, Hint: "Choose phone or QR login"})
				if err != nil {
					return fmt.Errorf("sign in method: %w", err)
				}
				if strings.EqualFold(method, "qr") {
					if err := b.qrAuth(ctx, client, loggedIn); err != nil {
						return fmt.Errorf("qr sign in: %w", err)
					}
				} else {
					flow := auth.NewFlow(authenticator{b: b}, auth.SendCodeOptions{})
					if err := client.Auth().IfNecessary(ctx, flow); err != nil {
						return fmt.Errorf("sign in: %w", err)
					}
				}
			}

			api := client.API()
			b.mu.Lock()
			b.api = api
			b.mu.Unlock()
			b.emit(StatusEvent{Text: "connected"})

			if self, err := client.Self(ctx); err == nil {
				b.mu.Lock()
				b.selfID = self.ID
				b.mu.Unlock()
				b.emit(SelfEvent{Name: userName(self), Username: self.Username})
			}
			if err := b.loadDialogsWithRetry(ctx); err != nil {
				return fmt.Errorf("dialogs: %w", err)
			}

			<-ctx.Done()
			return nil
		})
	})
	if err != nil && ctx.Err() == nil {
		b.emit(StatusEvent{Text: "disconnected", Err: err})
	}
}

func (b *Bridge) registerHandlers(d tg.UpdateDispatcher) {
	d.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		b.incoming(e, u.Message)
		return nil
	})
	d.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		b.incoming(e, u.Message)
		return nil
	})
	d.OnEditMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateEditMessage) error {
		b.edited(e, u.Message)
		return nil
	})
	d.OnEditChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateEditChannelMessage) error {
		b.edited(e, u.Message)
		return nil
	})
	d.OnDeleteMessages(func(ctx context.Context, e tg.Entities, u *tg.UpdateDeleteMessages) error {
		if b.store != nil {
			_ = b.store.DeleteMessages("", u.Messages)
		}
		b.emit(DeleteEvent{IDs: u.Messages})
		return nil
	})
	d.OnDeleteChannelMessages(func(ctx context.Context, e tg.Entities, u *tg.UpdateDeleteChannelMessages) error {
		key := fmt.Sprintf("channel:%d", u.ChannelID)
		if b.store != nil {
			_ = b.store.DeleteMessages(key, u.Messages)
		}
		b.emit(DeleteEvent{PeerKey: key, IDs: u.Messages})
		return nil
	})
	d.OnMessageReactions(func(ctx context.Context, e tg.Entities, u *tg.UpdateMessageReactions) error {
		key := peerKey(u.Peer)
		if key == "" {
			return nil
		}
		reactions := convertReactions(u.Reactions)
		if b.store != nil {
			_ = b.store.UpdateReactions(key, u.MsgID, reactions)
		}
		b.emit(ReactionsEvent{
			PeerKey:   key,
			MessageID: u.MsgID,
			Reactions: reactions,
		})
		return nil
	})
}

func (b *Bridge) loadDialogs(ctx context.Context) error {
	folders, err := b.loadFolders(ctx)
	if err != nil {
		b.emit(NoticeEvent{Text: "folders: " + err.Error()})
	}

	mainChats, err := b.fetchDialogs(ctx, 0, false)
	if err != nil {
		return err
	}
	archiveChats, err := b.fetchDialogs(ctx, 1, true)
	if err != nil {
		b.emit(NoticeEvent{Text: "archive: " + err.Error()})
	}

	out := append(mainChats, archiveChats...)
	for i := range out {
		fillChatFolders(&out[i], folders)
	}
	b.mu.Lock()
	b.folders = folders
	b.mu.Unlock()
	if b.store != nil {
		_ = b.store.SaveChats(out)
		_ = b.store.SaveFolders(folders)
	}
	b.emit(DialogsEvent{Chats: out, Folders: folders})
	b.emit(StatusEvent{Text: "connected"})
	return nil
}

func (b *Bridge) fetchDialogs(ctx context.Context, folderID int, archived bool) ([]domain.Chat, error) {
	req := &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100,
	}
	if folderID != 0 {
		req.SetFolderID(folderID)
	}
	res, err := b.api.MessagesGetDialogs(ctx, req)
	if err != nil {
		return nil, err
	}
	var rawDialogs []tg.DialogClass
	var rawMessages []tg.MessageClass
	var rawChats []tg.ChatClass
	var rawUsers []tg.UserClass
	switch m := res.(type) {
	case *tg.MessagesDialogs:
		rawDialogs, rawMessages, rawChats, rawUsers = m.Dialogs, m.Messages, m.Chats, m.Users
	case *tg.MessagesDialogsSlice:
		rawDialogs, rawMessages, rawChats, rawUsers = m.Dialogs, m.Messages, m.Chats, m.Users
	default:
		return nil, fmt.Errorf("unexpected dialogs response %T", res)
	}

	chatsArr := tg.ChatClassArray(rawChats)
	entities := peer.NewEntities(
		tg.UserClassArray(rawUsers).NotEmptyToMap(),
		chatsArr.ChatToMap(),
		chatsArr.ChannelToMap(),
	)
	last := make(map[int]tg.NotEmptyMessage, len(rawMessages))
	for _, item := range rawMessages {
		switch m := item.(type) {
		case *tg.Message:
			last[m.ID] = m
		case *tg.MessageService:
			last[m.ID] = m
		}
	}

	var out []domain.Chat
	for _, item := range rawDialogs {
		dialog, ok := item.(*tg.Dialog)
		if !ok || dialog.FolderID != folderID {
			continue
		}
		chat, input, ok := b.chatFromDialog(dialog, entities)
		if !ok {
			continue
		}
		preview, t := previewOf(last[dialog.TopMessage], entities)
		chat.Preview = preview
		chat.Time = t
		chat.Unread = dialog.UnreadCount
		chat.Pinned = dialog.Pinned
		chat.Muted = dialog.NotifySettings.MuteUntil > int(time.Now().Unix())
		chat.Archived = archived

		b.mu.Lock()
		b.peers[chat.PeerKey] = input
		b.chats[chat.PeerKey] = chat
		b.mu.Unlock()
		out = append(out, chat)
	}
	return out, nil
}

func (b *Bridge) loadFolders(ctx context.Context) ([]domain.Folder, error) {
	out := []domain.Folder{
		{ID: domain.FolderAll, Title: "All"},
		{ID: domain.FolderArchive, Title: "Archive"},
	}
	res, err := b.api.MessagesGetDialogFilters(ctx)
	if err != nil {
		return out, err
	}
	for _, item := range res.Filters {
		switch f := item.(type) {
		case *tg.DialogFilter:
			out = append(out, domain.Folder{
				ID:              f.ID,
				Title:           strings.TrimSpace(f.Title.Text),
				Emoji:           f.Emoticon,
				Pinned:          b.inputPeerKeys(f.PinnedPeers),
				Include:         b.inputPeerKeys(f.IncludePeers),
				Exclude:         b.inputPeerKeys(f.ExcludePeers),
				Contacts:        f.Contacts,
				NonContacts:     f.NonContacts,
				Bots:            f.Bots,
				Groups:          f.Groups,
				Broadcast:       f.Broadcasts,
				ExcludeMuted:    f.ExcludeMuted,
				ExcludeRead:     f.ExcludeRead,
				ExcludeArchived: f.ExcludeArchived,
			})
		case *tg.DialogFilterChatlist:
			out = append(out, domain.Folder{
				ID:      f.ID,
				Title:   strings.TrimSpace(f.Title.Text),
				Emoji:   f.Emoticon,
				Pinned:  b.inputPeerKeys(f.PinnedPeers),
				Include: b.inputPeerKeys(f.IncludePeers),
			})
		}
	}
	return out, nil
}

func (b *Bridge) inputPeerKeys(peers []tg.InputPeerClass) []string {
	out := make([]string, 0, len(peers))
	for _, p := range peers {
		switch p := p.(type) {
		case *tg.InputPeerUser:
			out = append(out, fmt.Sprintf("user:%d", p.UserID))
		case *tg.InputPeerChat:
			out = append(out, fmt.Sprintf("chat:%d", p.ChatID))
		case *tg.InputPeerChannel:
			out = append(out, fmt.Sprintf("channel:%d", p.ChannelID))
		case *tg.InputPeerSelf:
			b.mu.RLock()
			id := b.selfID
			b.mu.RUnlock()
			if id != 0 {
				out = append(out, fmt.Sprintf("user:%d", id))
			}
		}
	}
	return out
}

func fillChatFolders(chat *domain.Chat, folders []domain.Folder) {
	chat.Folders = nil
	if chat.Archived {
		chat.Folders = append(chat.Folders, domain.FolderArchive)
	}
	for _, f := range folders {
		if f.ID == domain.FolderAll || f.ID == domain.FolderArchive {
			continue
		}
		if folderMatches(f, chat) {
			chat.Folders = append(chat.Folders, f.ID)
		}
	}
}

func folderMatches(f domain.Folder, chat *domain.Chat) bool {
	if containsPeer(f.Pinned, chat.PeerKey) {
		return true
	}
	if containsPeer(f.Exclude, chat.PeerKey) {
		return false
	}
	match := containsPeer(f.Include, chat.PeerKey)
	switch chat.PeerKind {
	case "user":
		if chat.Bot {
			match = match || f.Bots
		} else if chat.Contact {
			match = match || f.Contacts
		} else {
			match = match || f.NonContacts
		}
	case "chat":
		match = match || f.Groups
	case "channel":
		match = match || f.Broadcast
	}
	if !match {
		return false
	}
	if f.ExcludeMuted && chat.Muted {
		return false
	}
	if f.ExcludeRead && chat.Unread == 0 {
		return false
	}
	if f.ExcludeArchived && chat.Archived {
		return false
	}
	return true
}

func containsPeer(list []string, key string) bool {
	for _, item := range list {
		if item == key {
			return true
		}
	}
	return false
}
func (b *Bridge) incoming(e tg.Entities, m tg.MessageClass) {
	msg, ok := m.(*tg.Message)
	if !ok {
		return
	}
	key := peerKey(msg.PeerID)
	if key == "" {
		return
	}
	entities := peer.EntitiesFromUpdate(e)
	dm := b.convertMessage(msg, entities, key)
	if dm.ID == 0 {
		return
	}
	preview := previewOfMessage(msg)
	now := time.Unix(int64(msg.Date), 0).Format("15:04")

	b.mu.RLock()
	chat, ok := b.chats[key]
	folders := append([]domain.Folder(nil), b.folders...)
	b.mu.RUnlock()
	if !ok {
		chat = b.chatFromPeer(msg.PeerID, entities)
	}
	if chat.Name == "" {
		chat.Name = key
	}
	chat.Preview = preview
	chat.Time = now
	fillChatFolders(&chat, folders)
	b.mu.Lock()
	b.chats[key] = chat
	b.mu.Unlock()

	if b.store != nil {
		_ = b.store.UpsertMessage(key, dm)
	}
	b.emit(NewMessageEvent{PeerKey: key, Chat: chat, Message: dm})
}

func (b *Bridge) edited(e tg.Entities, m tg.MessageClass) {
	msg, ok := m.(*tg.Message)
	if !ok {
		return
	}
	key := peerKey(msg.PeerID)
	if key == "" {
		return
	}
	if b.store != nil {
		_ = b.store.UpdateText(key, msg.ID, msg.Message, msg.EditDate != 0)
	}
	b.emit(EditMessageEvent{
		PeerKey:   key,
		MessageID: msg.ID,
		Text:      msg.Message,
		Edited:    msg.EditDate != 0,
	})
}

func (b *Bridge) LoadHistory(key string) {
	b.mu.RLock()
	api := b.api
	inputPeer := b.peers[key]
	b.mu.RUnlock()
	if inputPeer == nil || api == nil {
		if b.store != nil {
			msgs, _ := b.store.LoadMessages(key, 100)
			b.emit(HistoryEvent{PeerKey: key, Messages: withDaySeparators(msgs)})
			return
		}
		b.emit(HistoryEvent{PeerKey: key, Err: errors.New("chat is not ready")})
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(b.context(), 10*time.Minute)
		defer cancel()

		var (
			res tg.MessagesMessagesClass
			err error
		)
		for attempt := 0; attempt < 10; attempt++ {
			res, err = api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
				Peer:  inputPeer,
				Limit: 60,
			})
			if err == nil {
				break
			}
			wait, ok := tgerr.AsFloodWait(err)
			if !ok {
				break
			}
			b.emit(StatusEvent{Text: fmt.Sprintf("flood wait %s", wait.Round(time.Second))})
			timer := time.NewTimer(wait + time.Second)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				err = ctx.Err()
			}
		}
		if err != nil {
			b.emit(HistoryEvent{PeerKey: key, Err: err})
			return
		}

		var raw []tg.MessageClass
		var rawChats []tg.ChatClass
		var rawUsers []tg.UserClass
		switch m := res.(type) {
		case *tg.MessagesMessages:
			raw, rawChats, rawUsers = m.Messages, m.Chats, m.Users
		case *tg.MessagesMessagesSlice:
			raw, rawChats, rawUsers = m.Messages, m.Chats, m.Users
		case *tg.MessagesChannelMessages:
			raw, rawChats, rawUsers = m.Messages, m.Chats, m.Users
		default:
			b.emit(HistoryEvent{PeerKey: key, Err: fmt.Errorf("unexpected history response %T", res)})
			return
		}
		chatsArr := tg.ChatClassArray(rawChats)
		entities := peer.NewEntities(
			tg.UserClassArray(rawUsers).NotEmptyToMap(),
			chatsArr.ChatToMap(),
			chatsArr.ChannelToMap(),
		)

		var out []domain.Message
		var maxID int
		for _, item := range raw {
			msg, ok := item.(*tg.Message)
			if !ok {
				continue
			}
			dm := b.convertMessage(msg, entities, key)
			if dm.ID == 0 {
				continue
			}
			if msg.ID > maxID {
				maxID = msg.ID
			}
			out = append(out, dm)
		}
		reverseMessages(out)
		out = withDaySeparators(out)
		if b.store != nil {
			_ = b.store.SaveMessages(key, out)
		}
		b.emit(StatusEvent{Text: "connected"})
		b.emit(HistoryEvent{PeerKey: key, Messages: out})
		if maxID > 0 {
			b.MarkRead(key, maxID)
		}
	}()
}
func (b *Bridge) SendText(key, text string, replyTo int) {
	b.mu.RLock()
	api := b.api
	peer := b.peers[key]
	b.mu.RUnlock()
	if api == nil || peer == nil {
		b.emit(NoticeEvent{Text: "send failed", Err: errors.New("chat is not ready")})
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(b.context(), 30*time.Second)
		defer cancel()
		builder := message.NewSender(api).To(peer)
		var err error
		if replyTo > 0 {
			_, err = builder.Reply(replyTo).Text(ctx, text)
		} else {
			_, err = builder.Text(ctx, text)
		}
		if err != nil {
			b.emit(NoticeEvent{Text: "send failed", Err: err})
		} else {
			b.emit(StatusEvent{Text: "connected"})
		}
	}()
}

func (b *Bridge) EditText(key string, msgID int, text string) {
	b.mu.RLock()
	api := b.api
	peer := b.peers[key]
	b.mu.RUnlock()
	if api == nil || peer == nil || msgID == 0 {
		b.emit(NoticeEvent{Text: "edit failed", Err: errors.New("chat is not ready")})
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(b.context(), 30*time.Second)
		defer cancel()
		if _, err := message.NewSender(api).To(peer).Edit(msgID).Text(ctx, text); err != nil {
			b.emit(NoticeEvent{Text: "edit failed", Err: err})
			return
		}
		b.emit(EditMessageEvent{PeerKey: key, MessageID: msgID, Text: text, Edited: true})
	}()
}

func (b *Bridge) Forward(fromKey, toKey string, ids []int) {
	b.mu.RLock()
	api := b.api
	from := b.peers[fromKey]
	to := b.peers[toKey]
	b.mu.RUnlock()
	if api == nil || from == nil || to == nil || len(ids) == 0 {
		b.emit(NoticeEvent{Text: "forward failed", Err: errors.New("chat is not ready")})
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(b.context(), 30*time.Second)
		defer cancel()
		if _, err := message.NewSender(api).To(to).ForwardIDs(from, ids[0], ids[1:]...).Send(ctx); err != nil {
			b.emit(NoticeEvent{Text: "forward failed", Err: err})
			return
		}
		b.emit(NoticeEvent{Text: "forwarded"})
	}()
}

func (b *Bridge) SendReaction(key string, msgID int, emoji string, add bool) {
	b.mu.RLock()
	api := b.api
	peer := b.peers[key]
	b.mu.RUnlock()
	if api == nil || peer == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(b.context(), 20*time.Second)
		defer cancel()
		req := &tg.MessagesSendReactionRequest{Peer: peer, MsgID: msgID}
		if add {
			req.Reaction = []tg.ReactionClass{&tg.ReactionEmoji{Emoticon: emoji}}
		}
		if _, err := api.MessagesSendReaction(ctx, req); err != nil {
			b.emit(NoticeEvent{Text: "reaction failed", Err: err})
		}
	}()
}

func (b *Bridge) MarkRead(key string, maxID int) {
	b.mu.RLock()
	api := b.api
	peer := b.peers[key]
	b.mu.RUnlock()
	if api == nil || peer == nil || maxID <= 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(b.context(), 10*time.Second)
		defer cancel()
		_, _ = api.MessagesReadHistory(ctx, &tg.MessagesReadHistoryRequest{Peer: peer, MaxID: maxID})
		b.mu.Lock()
		if chat, ok := b.chats[key]; ok {
			chat.Unread = 0
			b.chats[key] = chat
		}
		b.mu.Unlock()
	}()
}

func (b *Bridge) Delete(key string, ids []int) {
	b.mu.RLock()
	api := b.api
	peer := b.peers[key]
	b.mu.RUnlock()
	if api == nil || peer == nil || len(ids) == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(b.context(), 20*time.Second)
		defer cancel()
		var err error
		if ch, ok := peer.(*tg.InputPeerChannel); ok {
			_, err = api.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
				Channel: &tg.InputChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash},
				ID:      ids,
			})
		} else {
			_, err = api.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{ID: ids})
		}
		if err != nil {
			b.emit(NoticeEvent{Text: "delete failed", Err: err})
		}
	}()
}

func (b *Bridge) RecentEmoji(limit int) []string {
	if b.store == nil {
		return nil
	}
	out, _ := b.store.RecentEmoji(limit)
	return out
}

func (b *Bridge) AddRecentEmoji(emoji string) {
	if b.store == nil {
		return
	}
	_ = b.store.AddRecentEmoji(emoji)
}
func (b *Bridge) SendFile(key, path string, replyTo int) {
	if _, err := os.Stat(path); err != nil {
		b.emit(NoticeEvent{Text: "upload failed", Err: err})
		return
	}
	b.mu.RLock()
	api := b.api
	peer := b.peers[key]
	b.mu.RUnlock()
	if api == nil || peer == nil {
		b.emit(StatusEvent{Text: "upload failed", Err: errors.New("chat is not ready")})
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(b.context(), 10*time.Minute)
		defer cancel()
		b.emit(NoticeEvent{Text: "uploading " + filepath.Base(path)})
		u := uploader.NewUploader(api)
		sender := message.NewSender(api).WithUploader(u)
		upload, err := u.FromPath(ctx, path)
		if err != nil {
			b.emit(NoticeEvent{Text: "upload failed", Err: err})
			return
		}
		builder := sender.To(peer)
		var media message.MediaOption
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jpg", ".jpeg", ".png", ".webp":
			media = message.UploadedPhoto(upload)
		default:
			doc := message.UploadedDocument(upload).Filename(filepath.Base(path))
			if mimeType := mime.TypeByExtension(filepath.Ext(path)); mimeType != "" {
				doc.MIME(mimeType)
			}
			media = doc
		}
		if replyTo > 0 {
			_, err = builder.Reply(replyTo).Media(ctx, media)
		} else {
			_, err = builder.Media(ctx, media)
		}
		if err != nil {
			b.emit(NoticeEvent{Text: "upload failed", Err: err})
			return
		}
		b.emit(NoticeEvent{Text: "file sent"})
		b.emit(StatusEvent{Text: "connected"})
	}()
}

func (b *Bridge) qrAuth(ctx context.Context, client *telegram.Client, loggedIn qrlogin.LoggedIn) error {
	_, err := client.QR().Auth(ctx, loggedIn, func(ctx context.Context, token qrlogin.Token) error {
		b.emit(QRTokenEvent{URL: token.URL()})
		return nil
	})
	if err == nil {
		return nil
	}
	if !tgerr.Is(err, "SESSION_PASSWORD_NEEDED") {
		return err
	}
	password, err := (authenticator{b: b}).Password(ctx)
	if err != nil {
		return err
	}
	_, err = client.Auth().Password(ctx, password)
	return err
}
func (b *Bridge) loadDialogsWithRetry(ctx context.Context) error {
	for {
		err := b.loadDialogs(ctx)
		if err == nil {
			return nil
		}
		wait, ok := tgerr.AsFloodWait(err)
		if !ok {
			return err
		}
		b.emit(StatusEvent{Text: fmt.Sprintf("flood wait %s", wait.Round(time.Second))})
		timer := time.NewTimer(wait + time.Second)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}
