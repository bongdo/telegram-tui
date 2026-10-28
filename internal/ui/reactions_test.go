package ui

import (
	"strings"
	"testing"
	"time"

	"tg-matrix-tui/internal/domain"
	"tg-matrix-tui/internal/tgclient"
)

func TestReactionsUpdateRenders(t *testing.T) {
	m := newModel()
	resize(m, 120, 40)
	chat := m.activeChat()
	if chat == nil {
		t.Fatal("no active chat")
	}
	chat.PeerKey = "user:9"
	chat.HistoryLoaded = true
	chat.Messages = []domain.Message{{
		ID:     10,
		Sender: "Alice",
		Text:   "hi",
		Time:   "12:00",
		Date:   time.Now().Unix(),
	}}
	m.msgVersion++

	m.handleTGEvent(tgclient.ReactionsEvent{
		PeerKey:   "user:9",
		MessageID: 10,
		Reactions: []domain.Reaction{{Emoji: "👍", Count: 2}},
	})

	if got := m.View().Content; !strings.Contains(got, "👍 2") {
		t.Fatalf("reaction not rendered:\n%s", got)
	}
}
