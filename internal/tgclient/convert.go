package tgclient

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/tg"

	"tg-matrix-tui/internal/domain"
)

func (b *Bridge) chatFromDialog(dialog *tg.Dialog, entities peer.Entities) (domain.Chat, tg.InputPeerClass, bool) {
	input, err := entities.ExtractPeer(dialog.Peer)
	if err != nil {
		return domain.Chat{}, nil, false
	}
	chat := b.chatFromPeer(dialog.Peer, entities)
	chat.Pinned = dialog.Pinned
	chat.Unread = dialog.UnreadCount
	chat.Muted = dialog.NotifySettings.MuteUntil > int(time.Now().Unix())
	return chat, input, chat.PeerKey != ""
}

func (b *Bridge) chatFromPeer(p tg.PeerClass, entities peer.Entities) domain.Chat {
	chat := domain.Chat{PeerKey: peerKey(p), Folder: domain.FolderPersonal, Color: colorFor(p)}
	switch p := p.(type) {
	case *tg.PeerUser:
		chat.PeerKind = "user"
		user := entities.Users()[p.UserID]
		if user == nil {
			return chat
		}
		chat.ID = int(user.ID)
		b.mu.RLock()
		self := b.selfID
		b.mu.RUnlock()
		if self != 0 && user.ID == self {
			chat.Name = "Saved Messages"
			chat.Status = "your cloud storage"
			chat.Username = ""
			chat.Phone = ""
			return chat
		}
		chat.Name = userName(user)
		chat.Username = username(user.Username)
		chat.Phone = user.Phone
		chat.Verified = user.Verified
		chat.Contact = user.Contact
		chat.Bot = user.Bot
		chat.Status, chat.Online = userStatus(user.Status)
	case *tg.PeerChat:
		chat.PeerKind = "chat"
		group := entities.Chats()[p.ChatID]
		if group == nil {
			return chat
		}
		chat.ID = int(group.ID)
		chat.Name = group.Title
		chat.Status = fmt.Sprintf("%d members", group.ParticipantsCount)
	case *tg.PeerChannel:
		chat.PeerKind = "channel"
		channel := entities.Channels()[p.ChannelID]
		if channel == nil {
			return chat
		}
		if channel.Megagroup {
			chat.PeerKind = "chat"
		}
		chat.ID = int(channel.ID)
		chat.Name = channel.Title
		chat.Username = username(channel.Username)
		chat.Verified = channel.Verified
		chat.Folder = domain.FolderChannels
		label := "members"
		if channel.Broadcast {
			label = "subscribers"
		}
		chat.Status = fmt.Sprintf("%d %s", channel.ParticipantsCount, label)
	}
	if chat.Name == "" {
		chat.Name = chat.PeerKey
	}
	return chat
}

func (b *Bridge) convertMessage(msg *tg.Message, entities peer.Entities, key string) domain.Message {
	out := domain.Message{
		ID:     msg.ID,
		Out:    msg.Out,
		Text:   msg.Message,
		Time:   time.Unix(int64(msg.Date), 0).Format("15:04"),
		Date:   int64(msg.Date),
		Edited: msg.EditDate != 0,
	}
	if out.Out {
		out.Checks = "✓✓"
	}

	from := msg.FromID
	if from == nil {
		from = msg.PeerID
	}
	out.Sender, out.Color = b.sender(from, entities, key)

	if fwd, ok := msg.GetFwdFrom(); ok {
		name := fwd.FromName
		if name == "" && fwd.FromID != nil {
			name, _ = b.sender(fwd.FromID, entities, key)
		}
		out.ForwardFrom = name
	}

	if reply, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && reply.ReplyToMsgID != 0 {
		out.ReplyToID = reply.ReplyToMsgID
		out.Reply = b.replyPreview(key, reply.ReplyToMsgID)
		if reply.QuoteText != "" {
			out.Reply.Text = reply.QuoteText
		}
	}

	if media := msg.Media; media != nil {
		switch m := media.(type) {
		case *tg.MessageMediaPhoto:
			out.Photo = photoOf(m)
			if out.Photo == nil && out.Text == "" {
				out.Text = "Photo"
			}
		case *tg.MessageMediaDocument:
			out.Voice = voiceOf(m)
			out.Round = roundOf(m)
			if out.Voice == nil && out.Round == nil && out.Text == "" {
				out.Text = mediaLabel(media)
			}
		default:
			if out.Text == "" {
				out.Text = mediaLabel(media)
			}
		}
	}
	if msg.Views > 0 {
		out.Views = compactNumber(msg.Views)
	}
	out.Reactions = convertReactions(msg.Reactions)
	return out
}

func (b *Bridge) sender(from tg.PeerClass, entities peer.Entities, key string) (string, int) {
	switch p := from.(type) {
	case *tg.PeerUser:
		if u := entities.Users()[p.UserID]; u != nil {
			return userName(u), colorFor(from)
		}
	case *tg.PeerChat:
		if c := entities.Chats()[p.ChatID]; c != nil {
			return c.Title, colorFor(from)
		}
	case *tg.PeerChannel:
		if c := entities.Channels()[p.ChannelID]; c != nil {
			return c.Title, colorFor(from)
		}
	}
	b.mu.RLock()
	name := b.chats[key].Name
	b.mu.RUnlock()
	return name, colorFor(from)
}

func (b *Bridge) replyPreview(key string, id int) *domain.Reply {
	b.mu.RLock()
	defer b.mu.RUnlock()
	chat, ok := b.chats[key]
	if ok {
		for i := len(chat.Messages) - 1; i >= 0; i-- {
			m := chat.Messages[i]
			if m.ID == id {
				text := m.Text
				if text == "" {
					text = mediaPlaceholder(m)
				}
				return &domain.Reply{Sender: m.Sender, Text: shorten(text, 60)}
			}
		}
	}
	return &domain.Reply{Sender: "", Text: fmt.Sprintf("message #%d", id)}
}

func mediaPlaceholder(m domain.Message) string {
	switch {
	case m.Voice != nil:
		return "Voice message"
	case m.Photo != nil:
		return "Photo"
	case m.Round != nil:
		return "Round video"
	}
	return m.Text
}

func userName(u *tg.User) string {
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		name = u.Username
	}
	if name == "" {
		name = fmt.Sprintf("user %d", u.ID)
	}
	return name
}

func username(s string) string {
	if s == "" {
		return ""
	}
	return "@" + s
}

func userStatus(status tg.UserStatusClass) (string, bool) {
	switch s := status.(type) {
	case *tg.UserStatusOnline:
		return "online", true
	case *tg.UserStatusOffline:
		if s.WasOnline == 0 {
			return "offline", false
		}
		return "last seen " + time.Unix(int64(s.WasOnline), 0).Format("2 Jan 15:04"), false
	case *tg.UserStatusRecently:
		return "last seen recently", false
	case *tg.UserStatusLastWeek:
		return "last seen within a week", false
	case *tg.UserStatusLastMonth:
		return "last seen within a month", false
	default:
		return "last seen recently", false
	}
}

func peerKey(p tg.PeerClass) string {
	switch p := p.(type) {
	case *tg.PeerUser:
		return fmt.Sprintf("user:%d", p.UserID)
	case *tg.PeerChat:
		return fmt.Sprintf("chat:%d", p.ChatID)
	case *tg.PeerChannel:
		return fmt.Sprintf("channel:%d", p.ChannelID)
	default:
		return ""
	}
}

func colorFor(p tg.PeerClass) int {
	var id int64
	switch p := p.(type) {
	case *tg.PeerUser:
		id = p.UserID
	case *tg.PeerChat:
		id = p.ChatID
	case *tg.PeerChannel:
		id = p.ChannelID
	}
	if id < 0 {
		id = -id
	}
	return int(id % 7)
}

func previewOf(msg tg.NotEmptyMessage, entities peer.Entities) (string, string) {
	switch m := msg.(type) {
	case *tg.Message:
		return previewOfMessage(m), time.Unix(int64(m.Date), 0).Format("15:04")
	case *tg.MessageService:
		return "service message", time.Unix(int64(m.Date), 0).Format("15:04")
	default:
		return "", ""
	}
}

func previewOfMessage(msg *tg.Message) string {
	text := msg.Message
	if text == "" {
		text = mediaLabel(msg.Media)
	}
	if text == "" {
		text = "message"
	}
	text = shorten(strings.ReplaceAll(text, "\n", " "), 90)
	if msg.Out {
		return "You: " + text
	}
	return text
}

func mediaLabel(media tg.MessageMediaClass) string {
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		return "Photo"
	case *tg.MessageMediaDocument:
		if voiceOf(m) != nil {
			return "Voice message"
		}
		if roundOf(m) != nil {
			return "Round video"
		}
		doc, ok := m.Document.AsNotEmpty()
		if !ok {
			return "File"
		}
		for _, a := range doc.Attributes {
			switch attr := a.(type) {
			case *tg.DocumentAttributeSticker:
				return "Sticker"
			case *tg.DocumentAttributeAnimated:
				return "GIF"
			case *tg.DocumentAttributeVideo:
				if attr.RoundMessage {
					return "Round video"
				}
				return "Video"
			case *tg.DocumentAttributeAudio:
				if attr.Voice {
					return "Voice message"
				}
				return "Audio"
			}
		}
		return "File"
	case *tg.MessageMediaPoll:
		return "Poll"
	case *tg.MessageMediaGeo, *tg.MessageMediaGeoLive:
		return "Location"
	case *tg.MessageMediaContact:
		return "Contact"
	case *tg.MessageMediaWebPage:
		return ""
	default:
		return ""
	}
}

func voiceOf(media *tg.MessageMediaDocument) *domain.Voice {
	doc, ok := media.Document.AsNotEmpty()
	if !ok {
		return nil
	}
	for _, a := range doc.Attributes {
		attr, ok := a.(*tg.DocumentAttributeAudio)
		if !ok || !attr.Voice {
			continue
		}
		return &domain.Voice{
			Duration: attr.Duration,
			Waveform: waveform(attr.Waveform),
		}
	}
	return nil
}

func roundOf(media *tg.MessageMediaDocument) *domain.Round {
	doc, ok := media.Document.AsNotEmpty()
	if !ok {
		return nil
	}
	for _, a := range doc.Attributes {
		attr, ok := a.(*tg.DocumentAttributeVideo)
		if !ok || !attr.RoundMessage {
			continue
		}
		return &domain.Round{Duration: int(attr.Duration)}
	}
	return nil
}

func photoOf(media *tg.MessageMediaPhoto) *domain.Photo {
	photo, ok := media.Photo.AsNotEmpty()
	if !ok {
		return nil
	}
	out := &domain.Photo{}
	for _, size := range photo.Sizes {
		switch s := size.(type) {
		case *tg.PhotoSize:
			if s.W*s.H > out.W*out.H {
				out.W, out.H = s.W, s.H
			}
		case *tg.PhotoSizeProgressive:
			if s.W*s.H > out.W*out.H {
				out.W, out.H = s.W, s.H
			}
		case *tg.PhotoCachedSize:
			if s.W*s.H > out.W*out.H {
				out.W, out.H = s.W, s.H
			}
		}
	}
	return out
}

func convertReactions(reactions tg.MessageReactions) []domain.Reaction {
	if len(reactions.Results) == 0 {
		return nil
	}
	out := make([]domain.Reaction, 0, len(reactions.Results))
	for _, r := range reactions.Results {
		emoji := ""
		switch reaction := r.Reaction.(type) {
		case *tg.ReactionEmoji:
			emoji = reaction.Emoticon
		case *tg.ReactionCustomEmoji:
			emoji = "◈"
		default:
			continue
		}
		out = append(out, domain.Reaction{
			Emoji: emoji,
			Count: r.Count,
			Mine:  r.ChosenOrder != 0,
		})
	}
	return out
}

func waveform(data []byte) string {
	const bars = "▁▂▃▄▅▆▇█"
	var out []rune
	bit := 0
	for bit+5 <= len(data)*8 {
		v := 0
		for j := 0; j < 5; j++ {
			pos := bit + j
			b := (data[pos/8] >> uint(pos%8)) & 1
			v |= int(b) << j
		}
		out = append(out, []rune(bars)[v*len([]rune(bars))/32])
		bit += 5
	}
	if len(out) == 0 {
		return strings.Repeat("▁", 20)
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return string(out)
}

func reverseMessages(msgs []domain.Message) {
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
}

func withDaySeparators(msgs []domain.Message) []domain.Message {
	out := make([]domain.Message, 0, len(msgs)+4)
	prev := ""
	for _, m := range msgs {
		day := ""
		if m.Date > 0 {
			t := time.Unix(m.Date, 0)
			day = t.Format("2006-01-02")
			if day != prev {
				out = append(out, domain.Message{Service: true, Text: dayLabel(t)})
				prev = day
			}
		}
		out = append(out, m)
	}
	return out
}

func dayLabel(t time.Time) string {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())
	switch {
	case day.Equal(today):
		return "Today"
	case day.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	default:
		if t.Year() == now.Year() {
			return t.Format("2 Jan")
		}
		return t.Format("2 Jan 2006")
	}
}

func compactNumber(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func shorten(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}
