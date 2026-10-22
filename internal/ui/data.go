package ui

import "tg-matrix-tui/internal/domain"

const (
	folderAll      = domain.FolderAll
	folderArchive  = domain.FolderArchive
	folderPersonal = domain.FolderPersonal
	folderWork     = domain.FolderWork
	folderChannels = domain.FolderChannels
)

type Chat = domain.Chat
type Message = domain.Message
type Reply = domain.Reply
type Voice = domain.Voice
type Photo = domain.Photo
type Round = domain.Round
type Reaction = domain.Reaction
type Folder = domain.Folder

// Peer avatar palette, close to Telegram's generated avatar colors.
var peerColors = []string{
	"#E17076", "#FAA774", "#A695E7", "#7BC862", "#6EC9CB", "#65AADD", "#EE7AAE",
}

func mockChats() []Chat {
	chats := []Chat{
		{
			ID: 1, Name: "Saved Messages", Preview: "TUI notes and links", Time: "12:04",
			Folder: folderPersonal, Color: 5, Status: "your cloud storage",
			About: "Messages you send here stay in your cloud storage.",
			Messages: []Message{
				{ID: 1, Service: true, Text: "Today"},
				{ID: 2, Out: true, Text: "Telegram Web A tokens: left 420px, messages 728px, right 424px, header 56px.", Time: "12:02"},
				{ID: 3, Out: true, Text: "The TUI keeps the same proportions: 30 / flex / 32 cells.", Time: "12:04"},
			},
		},
		{
			ID: 2, Name: "Alice", Preview: "see you at 19:00", Time: "12:31", Unread: 2,
			Online: true, Folder: folderPersonal, Color: 6, Status: "online",
			Username: "@alice", Phone: "+1 202 555 0143",
			About: "Coffee, keyboards, and unreasonable terminal setups.",
			Messages: []Message{
				{ID: 1, Service: true, Text: "Today"},
				{ID: 2, Sender: "Alice", Color: 6, Text: "did you finish the mouse hit-testing?", Time: "12:28"},
				{ID: 3, Out: true, Text: "Yes. Click opens a chat, wheel follows the cursor, hover is already there.", Time: "12:29"},
				{ID: 4, Sender: "Alice", Color: 6, Text: "see you at 19:00", Time: "12:31", Reactions: []Reaction{{Emoji: "♥", Count: 1, Mine: true}}},
			},
		},
		{
			ID: 3, Name: "NixOS Community", Preview: "Mira: the flake is in the repo", Time: "12:22", Unread: 12,
			Folder: folderPersonal, Color: 3, Status: "8 214 members",
			About: "Unofficial NixOS user group. Be excellent to each other.",
			Messages: []Message{
				{ID: 1, Service: true, Text: "Today"},
				{ID: 2, Sender: "Mira", Color: 6, Text: "the flake is in the repo, mpv is wired through the shell", Time: "12:22"},
				{ID: 3, Sender: "Jonas", Color: 4, Text: "works on aarch64 too", Time: "12:23"},
			},
		},
		{
			ID: 4, Name: "Charm TUI", Preview: "Mira: hit-testing works from the first click", Time: "12:40", Unread: 3,
			Pinned: true, Folder: folderWork, Color: 2, Status: "42 members, 5 online",
			About: "Bubble Tea, Lip Gloss and terminal interfaces.",
			Messages: []Message{
				{ID: 1, Service: true, Text: "Yesterday"},
				{ID: 2, Sender: "Kai", Color: 5, Text: "We should keep the web layout: list on the left, conversation in the middle, profile on demand.", Time: "18:12"},
				{ID: 3, Out: true, Text: "Agreed. Mouse first, keyboard parity, no hidden click targets.", Time: "18:14", Edited: true},
				{ID: 4, Service: true, Text: "Today"},
				{ID: 5, Sender: "Mira", Color: 6, Text: "The new chat layout is in main. Hit-testing works from the first click.",
					Time: "12:34", Reply: &Reply{Sender: "Kai", Text: "Keep the web layout"}, Reactions: []Reaction{{Emoji: "♥", Count: 3}, {Emoji: "+1", Count: 2}}},
				{ID: 6, Out: true, Text: "Checked it in a 60×20 tmux pane. The sidebar collapses correctly.", Time: "12:35", Views: "4"},
				{ID: 7, Sender: "Kai", Color: 5, Text: "Mouse routing feels native:", Time: "12:36",
					Photo: &Photo{W: 1280, H: 720, Caption: "three-column layout"}},
				{ID: 8, Sender: "Mira", Color: 6, Text: "", Time: "12:37",
					Voice: &Voice{Duration: 12, Waveform: "▁▂▃▅▇▆▄▂▁▂▄▆▇▅▃▂▁▃▅▆"}},
				{ID: 9, Out: true, Text: "", Time: "12:38",
					Voice: &Voice{Duration: 7, Waveform: "▁▃▅▇▅▃▂▁▂▄▆▇▅▂▁"}},
				{ID: 10, Sender: "Kai", Color: 5, Text: "", Time: "12:39",
					Round: &Round{Duration: 42}},
				{ID: 11, Sender: "Mira", Color: 6, Text: "Profile panel opens from the title, closes on Esc or the ✕ button.", Time: "12:40",
					Reactions: []Reaction{{Emoji: "♥", Count: 2, Mine: true}}},
			},
		},
		{
			ID: 5, Name: "Bob", Preview: "sent a file", Time: "11:58",
			Folder: folderPersonal, Color: 4, Status: "last seen 12 minutes ago",
			Username: "@bob", Phone: "+1 202 555 0188",
			Messages: []Message{
				{ID: 1, Service: true, Text: "Today"},
				{ID: 2, Sender: "Bob", Color: 4, Text: "sent a file", Time: "11:58"},
			},
		},
		{
			ID: 6, Name: "Design Weekly", Preview: "Issue #48: dense interfaces", Time: "11:20",
			Muted: true, Verified: true, Folder: folderChannels, Color: 1, Status: "18 902 subscribers",
			About:    "A weekly letter about interfaces that respect attention.",
			Username: "@designweekly",
			Messages: []Message{
				{ID: 1, Service: true, Text: "Today"},
				{ID: 2, Sender: "Design Weekly", Color: 1, Text: "Issue #48: dense interfaces that still breathe.", Time: "11:20", Views: "12.4K"},
			},
		},
		{
			ID: 7, Name: "Matrix Bridge", Preview: "bridge: telegram → matrix is up", Time: "10:44",
			Folder: folderWork, Color: 0, Status: "12 members",
			About: "Bridge room between Telegram and Matrix.",
			Messages: []Message{
				{ID: 1, Service: true, Text: "Today"},
				{ID: 2, Sender: "bridge", Color: 0, Text: "telegram → matrix is up", Time: "10:44"},
			},
		},
		{
			ID: 8, Name: "Mom", Preview: "call me when you can", Time: "09:31", Unread: 1,
			Folder: folderPersonal, Color: 6, Status: "last seen recently",
			Phone: "+1 202 555 0101",
			Messages: []Message{
				{ID: 1, Service: true, Text: "Today"},
				{ID: 2, Sender: "Mom", Color: 6, Text: "call me when you can", Time: "09:31"},
			},
		},
		{
			ID: 9, Name: "Rustlang", Preview: "New RFC: async drop", Time: "Yesterday", Unread: 27,
			Muted: true, Folder: folderChannels, Color: 1, Status: "94 102 subscribers",
			About: "News and discussion about the Rust programming language.",
			Messages: []Message{
				{ID: 1, Service: true, Text: "Yesterday"},
				{ID: 2, Sender: "Rustlang", Color: 1, Text: "New RFC: async drop", Time: "21:10", Views: "8.7K"},
			},
		},
		{
			ID: 10, Name: "Work Chat", Preview: "standup at 10", Time: "Yesterday",
			Folder: folderWork, Color: 5, Status: "9 members",
			Messages: []Message{
				{ID: 1, Service: true, Text: "Yesterday"},
				{ID: 2, Sender: "Dana", Color: 5, Text: "standup at 10", Time: "19:40"},
			},
		},
	}
	for i := range chats {
		switch chats[i].Name {
		case "NixOS Community", "Charm TUI", "Matrix Bridge", "Work Chat":
			chats[i].PeerKind = "chat"
		case "Design Weekly", "Rustlang":
			chats[i].PeerKind = "channel"
		default:
			chats[i].PeerKind = "user"
		}
	}
	return chats
}
