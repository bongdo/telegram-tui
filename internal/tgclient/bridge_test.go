package tgclient

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/tg"

	"tg-matrix-tui/internal/domain"
)

func testEntities() peer.Entities {
	return peer.NewEntities(
		map[int64]*tg.User{
			1: {ID: 1, AccessHash: 11, FirstName: "Alice", Username: "alice", Status: &tg.UserStatusOnline{}},
		},
		map[int64]*tg.Chat{
			42: {ID: 42, Title: "Group Chat", ParticipantsCount: 8},
		},
		map[int64]*tg.Channel{
			7: {ID: 7, AccessHash: 77, Title: "Channel", Username: "chan", ParticipantsCount: 1200, Broadcast: true},
		},
	)
}

func TestChatFromDialog(t *testing.T) {
	b := New(Config{})
	dialog := &tg.Dialog{Peer: &tg.PeerUser{UserID: 1}, UnreadCount: 3, Pinned: true}
	chat, input, ok := b.chatFromDialog(dialog, testEntities())
	if !ok || input == nil {
		t.Fatal("dialog was not resolved")
	}
	if chat.PeerKey != "user:1" || chat.Name != "Alice" || chat.Unread != 3 || !chat.Pinned {
		t.Fatalf("chat = %+v", chat)
	}
	if !chat.Online {
		t.Fatal("Alice should be online")
	}
}

func TestConvertTextMessage(t *testing.T) {
	b := New(Config{})
	msg := &tg.Message{
		ID:      10,
		PeerID:  &tg.PeerUser{UserID: 1},
		FromID:  &tg.PeerUser{UserID: 1},
		Date:    int(time.Now().Unix()),
		Message: "hello",
		Out:     false,
		Views:   1200,
		Reactions: tg.MessageReactions{Results: []tg.ReactionCount{
			{Reaction: &tg.ReactionEmoji{Emoticon: "👍"}, Count: 2, ChosenOrder: 1},
		}},
	}
	got := b.convertMessage(msg, testEntities(), "user:1")
	if got.Sender != "Alice" || got.Text != "hello" {
		t.Fatalf("message = %+v", got)
	}
	if got.Date == 0 || got.Views != "1.2K" {
		t.Fatalf("date/views = %d/%q", got.Date, got.Views)
	}
	if len(got.Reactions) != 1 || !got.Reactions[0].Mine {
		t.Fatalf("reactions = %+v", got.Reactions)
	}
}

func TestConvertVoice(t *testing.T) {
	b := New(Config{})
	doc := &tg.Document{
		ID:         5,
		AccessHash: 55,
		MimeType:   "audio/ogg",
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeAudio{Voice: true, Duration: 7, Waveform: []byte{1, 2, 3, 4, 5}},
		},
	}
	msg := &tg.Message{
		ID:     11,
		PeerID: &tg.PeerUser{UserID: 1},
		Date:   int(time.Now().Unix()),
		Media:  &tg.MessageMediaDocument{Document: doc},
	}
	got := b.convertMessage(msg, testEntities(), "user:1")
	if got.Voice == nil || got.Voice.Duration != 7 || got.Voice.Waveform == "" {
		t.Fatalf("voice = %+v", got.Voice)
	}
	if got.Text != "" {
		t.Fatalf("voice text = %q, want empty", got.Text)
	}
}

func TestAuthAskRoundTrip(t *testing.T) {
	b := New(Config{})
	b.ctx, b.cancel = context.WithCancel(context.Background())
	defer b.cancel()

	type result struct {
		value string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		value, err := b.ask(b.ctx, AuthPrompt{Kind: AuthPhone, Hint: "phone"})
		done <- result{value, err}
	}()

	select {
	case ev := <-b.events:
		prompt, ok := ev.(AuthPrompt)
		if !ok || prompt.Kind != AuthPhone {
			t.Fatalf("event = %#v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no auth prompt")
	}
	b.SubmitAuth("+15550100")
	select {
	case got := <-done:
		if got.err != nil || got.value != "+15550100" {
			t.Fatalf("ask = %q, %v", got.value, got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("ask did not return")
	}
}

func TestDaySeparators(t *testing.T) {
	now := time.Now()
	msgs := []domain.Message{
		{ID: 1, Date: now.AddDate(0, 0, -1).Unix(), Text: "old"},
		{ID: 2, Date: now.Unix(), Text: "new"},
	}
	got := withDaySeparators(msgs)
	if len(got) != 4 || !got[0].Service || !got[2].Service {
		t.Fatalf("separators = %+v", got)
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("TG_TUI_DEMO", "")
	t.Setenv("TG_API_ID", "")
	t.Setenv("TG_API_HASH", "")
	t.Setenv("APP_ID", "")
	t.Setenv("APP_HASH", "")
	if cfg, ok, err := ConfigFromEnv(); !ok || err != nil || cfg.AppID != defaultAppID {
		t.Fatalf("built-in config: ok=%v id=%d err=%v", ok, cfg.AppID, err)
	}

	t.Setenv("TG_TUI_DEMO", "1")
	t.Setenv("TG_API_ID", "12345")
	t.Setenv("TG_API_HASH", "abcdef")
	if _, ok, err := ConfigFromEnv(); ok || err != nil {
		t.Fatalf("demo override: ok=%v err=%v", ok, err)
	}
	t.Setenv("TG_TUI_DEMO", "")
	t.Setenv("TG_API_HASH", "")
	t.Setenv("TG_API_ID", "12345")
	if _, ok, err := ConfigFromEnv(); ok || err == nil {
		t.Fatalf("half config: ok=%v err=%v", ok, err)
	}

	t.Setenv("TG_API_HASH", "abcdef")
	t.Setenv("TG_SESSION", filepath.Join(t.TempDir(), "session.json"))
	cfg, ok, err := ConfigFromEnv()
	if !ok || err != nil || cfg.AppID != 12345 || cfg.AppHash != "abcdef" {
		t.Fatalf("config = %+v ok=%v err=%v", cfg, ok, err)
	}
}

func TestFolderMembership(t *testing.T) {
	folders := []domain.Folder{
		{ID: domain.FolderAll, Title: "All"},
		{ID: domain.FolderArchive, Title: "Archive"},
		{ID: 2, Title: "Work", Groups: true, ExcludeMuted: true},
		{ID: 3, Title: "People", Contacts: true},
		{ID: 4, Title: "Pins", Pinned: []string{"channel:7"}},
	}
	chats := []domain.Chat{
		{PeerKey: "chat:42", PeerKind: "chat"},
		{PeerKey: "chat:43", PeerKind: "chat", Muted: true},
		{PeerKey: "user:1", PeerKind: "user", Contact: true},
		{PeerKey: "user:2", PeerKind: "user"},
		{PeerKey: "channel:7", PeerKind: "channel", Archived: true},
	}
	for i := range chats {
		fillChatFolders(&chats[i], folders)
	}
	if len(chats[0].Folders) != 1 || chats[0].Folders[0] != 2 {
		t.Fatalf("group folders = %v", chats[0].Folders)
	}
	if len(chats[1].Folders) != 0 {
		t.Fatalf("muted group folders = %v", chats[1].Folders)
	}
	if len(chats[2].Folders) != 1 || chats[2].Folders[0] != 3 {
		t.Fatalf("contact folders = %v", chats[2].Folders)
	}
	if len(chats[3].Folders) != 0 {
		t.Fatalf("non-contact folders = %v", chats[3].Folders)
	}
	if len(chats[4].Folders) != 2 || chats[4].Folders[0] != domain.FolderArchive || chats[4].Folders[1] != 4 {
		t.Fatalf("archived pinned folders = %v", chats[4].Folders)
	}
}

func TestIncomingNewUserChat(t *testing.T) {
	b := New(Config{})
	b.ctx, b.cancel = context.WithCancel(context.Background())
	defer b.cancel()
	b.folders = []domain.Folder{{ID: 2, Title: "People", Contacts: true}}

	done := make(chan struct{})
	go func() {
		b.incoming(tg.Entities{Users: map[int64]*tg.User{
			1: {ID: 1, AccessHash: 11, FirstName: "Alice", Contact: true},
		}}, &tg.Message{
			ID:      10,
			PeerID:  &tg.PeerUser{UserID: 1},
			Date:    int(time.Now().Unix()),
			Message: "hello",
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("incoming deadlocked on a new user chat")
	}
	chat, ok := b.chats["user:1"]
	if !ok || chat.Name != "Alice" {
		t.Fatalf("chat = %+v", chat)
	}
	if len(chat.Folders) != 1 || chat.Folders[0] != 2 {
		t.Fatalf("folders = %v", chat.Folders)
	}
}
