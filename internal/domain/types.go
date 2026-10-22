// Package domain holds the data the UI renders, independent of Telegram RPC
// types and terminal layout.
package domain

const (
	FolderAll      = 0
	FolderArchive  = 1
	FolderPersonal = 2
	FolderWork     = 3
	FolderChannels = 4
)

// Folder is a server-side chat folder. All and Archive are virtual entries
// supplied by the bridge; custom folders carry Telegram's filter rules.
type Folder struct {
	ID              int
	Title           string
	Emoji           string
	Pinned          []string
	Include         []string
	Exclude         []string
	Contacts        bool
	NonContacts     bool
	Bots            bool
	Groups          bool
	Broadcast       bool
	ExcludeMuted    bool
	ExcludeRead     bool
	ExcludeArchived bool
}

type Chat struct {
	ID            int
	PeerKey       string // stable key: "user:123", "chat:42", "channel:7"
	PeerKind      string // "user", "chat" or "channel"
	Name          string
	Preview       string
	Time          string
	Unread        int
	Online        bool
	Muted         bool
	Pinned        bool
	Verified      bool
	Folder        int   // legacy single-folder value used by mock data
	Folders       []int // server folder memberships (custom IDs plus FolderArchive)
	Archived      bool
	Contact       bool
	Bot           bool
	Status        string
	Color         int
	About         string
	Username      string
	Phone         string
	HistoryLoaded bool
	Messages      []Message
}

type Message struct {
	ID          int
	Sender      string
	Color       int
	Text        string
	Time        string
	Out         bool
	Service     bool
	Reply       *Reply
	ReplyToID   int
	Voice       *Voice
	Photo       *Photo
	Round       *Round
	ForwardFrom string
	Reactions   []Reaction
	Edited      bool
	Date        int64 // unix seconds, 0 for mock
	Views       string
	Checks      string
}

type Reply struct {
	Sender string
	Text   string
}

type Voice struct {
	Duration int // seconds
	Waveform string
	File     string // local path after download
}

type Photo struct {
	W, H    int
	Caption string
	File    string
}

type Round struct {
	Duration int
	File     string
}

type Reaction struct {
	Emoji string
	Count int
	Mine  bool
}
