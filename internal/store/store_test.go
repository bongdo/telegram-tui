package store

import (
	"path/filepath"
	"testing"

	"tg-matrix-tui/internal/domain"
)

func TestCacheRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	chats := []domain.Chat{
		{PeerKey: "user:1", Name: "Alice", Pinned: true, Messages: []domain.Message{{ID: 1, Text: "ignored"}}},
		{PeerKey: "channel:7", Name: "Channel", Folder: domain.FolderChannels},
	}
	if err := s.SaveChats(chats); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadChats()
	if err != nil || len(loaded) != 2 {
		t.Fatalf("chats = %+v, %v", loaded, err)
	}
	if loaded[0].Name != "Alice" || !loaded[0].Pinned {
		t.Fatalf("first chat = %+v", loaded[0])
	}

	msgs := []domain.Message{
		{ID: 1, Text: "one", Date: 100, Voice: &domain.Voice{Duration: 3}},
		{ID: 2, Text: "two", Date: 200},
	}
	if err := s.SaveMessages("user:1", msgs); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadMessages("user:1", 10)
	if err != nil || len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("messages = %+v, %v", got, err)
	}
	if err := s.UpdateText("user:1", 1, "edited", true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMedia("user:1", 1, "voice", "/tmp/v.ogg"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.LoadMessages("user:1", 10)
	if got[0].Text != "edited" || !got[0].Edited || got[0].Voice == nil || got[0].Voice.File != "/tmp/v.ogg" {
		t.Fatalf("updated message = %+v", got[0])
	}

	if err := s.AddRecentEmoji("🔥"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRecentEmoji("👍"); err != nil {
		t.Fatal(err)
	}
	emoji, err := s.RecentEmoji(2)
	if err != nil || len(emoji) != 2 || emoji[0] != "👍" {
		t.Fatalf("recent = %+v, %v", emoji, err)
	}

	folders := []domain.Folder{
		{ID: domain.FolderAll, Title: "All"},
		{ID: 7, Title: "Work", Emoji: "💼", Include: []string{"chat:42"}, ExcludeMuted: true},
	}
	if err := s.SaveFolders(folders); err != nil {
		t.Fatal(err)
	}
	gotFolders, err := s.LoadFolders()
	if err != nil || len(gotFolders) != 2 {
		t.Fatalf("folders = %+v, %v", gotFolders, err)
	}
	if gotFolders[1].Title != "Work" || !gotFolders[1].ExcludeMuted || len(gotFolders[1].Include) != 1 {
		t.Fatalf("folder = %+v", gotFolders[1])
	}
}
