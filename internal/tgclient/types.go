package tgclient

import "github.com/bongdo/telegram-tui/internal/domain"

type AuthKind string

const (
	AuthMethod   AuthKind = "method"
	AuthPhone    AuthKind = "phone"
	AuthCode     AuthKind = "code"
	AuthPassword AuthKind = "password"
)

// Event is emitted by the Telegram bridge and consumed by the TUI.
type Event interface{ event() }

type StatusEvent struct {
	Text string
	Err  error
}

func (StatusEvent) event() {}

type AuthPrompt struct {
	Kind AuthKind
	Hint string
}

func (AuthPrompt) event() {}

type SelfEvent struct {
	Name     string
	Username string
}

func (SelfEvent) event() {}

type DialogsEvent struct {
	Chats   []domain.Chat
	Folders []domain.Folder
	Cached  bool
}

func (DialogsEvent) event() {}

type HistoryEvent struct {
	PeerKey  string
	Messages []domain.Message
	Err      error
}

func (HistoryEvent) event() {}

type NewMessageEvent struct {
	PeerKey string
	Chat    domain.Chat
	Message domain.Message
}

func (NewMessageEvent) event() {}

type EditMessageEvent struct {
	PeerKey   string
	MessageID int
	Text      string
	Edited    bool
}

func (EditMessageEvent) event() {}

type ReactionsEvent struct {
	PeerKey   string
	MessageID int
	Reactions []domain.Reaction
}

func (ReactionsEvent) event() {}

type DeleteEvent struct {
	PeerKey string
	IDs     []int
}

func (DeleteEvent) event() {}

type MediaReadyEvent struct {
	PeerKey   string
	MessageID int
	Kind      string
	Path      string
	Err       error
}

func (MediaReadyEvent) event() {}

type QRTokenEvent struct {
	URL string
}

func (QRTokenEvent) event() {}

type NoticeEvent struct {
	Text string
	Err  error
}

func (NoticeEvent) event() {}
