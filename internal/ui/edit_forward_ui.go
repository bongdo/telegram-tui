package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type forwardState struct {
	peerKey   string
	chatIndex int
	msgID     int
	query     textinput.Model
	sel       int
}

func (m *model) beginEdit() {
	chat := m.activeChat()
	idx := m.selectedMessageIndex()
	if chat == nil || idx < 0 {
		return
	}
	msg := &chat.Messages[idx]
	if !msg.Out || msg.Service || msg.Text == "" {
		m.notice = "only your text messages can be edited"
		return
	}
	m.reply = nil
	m.replyToID = 0
	m.editingID = msg.ID
	m.ta.SetValue(msg.Text)
	m.ta.CursorEnd()
	m.setFocus(focusComposer)
	m.syncInput()
}

func (m *model) finishEdit() {
	m.editingID = 0
	m.ta.Reset()
	m.ta.SetHeight(1)
	m.syncInput()
}

func (m *model) cancelEdit() {
	if m.editingID == 0 {
		return
	}
	m.finishEdit()
	m.notice = "edit cancelled"
}

func (m *model) beginForward() {
	chat := m.activeChat()
	idx := m.selectedMessageIndex()
	if chat == nil || idx < 0 {
		m.notice = "select a message to forward"
		return
	}
	f := &forwardState{peerKey: chat.PeerKey, chatIndex: m.active, msgID: chat.Messages[idx].ID}
	f.query = textinput.New()
	f.query.Prompt = ""
	f.query.Placeholder = "Search chats"
	f.query.CharLimit = 64
	f.query.Focus()
	m.fwd = f
	m.ctxKind = ctxNone
	m.emojiOpen = false
	m.applyInputStyles()
}

func (m *model) forwardChats() []int {
	f := m.fwd
	if f == nil {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(f.query.Value()))
	out := make([]int, 0, len(m.chats))
	for i := range m.chats {
		if i == f.chatIndex {
			continue
		}
		chat := &m.chats[i]
		if q != "" {
			hay := strings.ToLower(chat.Name + " " + chat.Username + " " + chat.Preview)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		out = append(out, i)
	}
	return out
}

func (m *model) forwardPanelRect() rect {
	width := clamp(m.w-8, 32, 52)
	if width > m.w-2 {
		width = m.w - 2
	}
	rows := minInt(len(m.forwardChats()), maxInt(1, m.h-9))
	height := rows + 5
	if height > m.h-2 {
		height = m.h - 2
	}
	x := (m.w - width) / 2
	y := (m.h - height) / 2
	return rect{x, y, width, height}
}

func (m *model) forwardRowRects() []rect {
	p := m.forwardPanelRect()
	chats := m.forwardChats()
	rows := minInt(len(chats), p.h-5)
	out := make([]rect, 0, rows)
	for i := 0; i < rows; i++ {
		out = append(out, rect{p.x + 1, p.y + 4 + i, p.w - 2, 1})
	}
	return out
}

func (m *model) renderForward() ([]string, int, int) {
	t := m.theme
	bg := t.PanelAlt
	border := t.Primary
	p := m.forwardPanelRect()
	contentW := p.w - 2
	lines := []string{paint("╭"+strings.Repeat("─", contentW)+"╮", border, bg)}

	title := " Forward message"
	if m.fwd.chatIndex >= 0 && m.fwd.chatIndex < len(m.chats) {
		title = " Forward from " + trunc(m.chats[m.fwd.chatIndex].Name, maxInt(1, contentW-18))
	}
	lines = append(lines, paint("│", border, bg)+t.pad(title, contentW, t.Text, bg, true)+paint("│", border, bg))

	m.fwd.query.SetWidth(maxInt(1, contentW-2))
	field := strings.SplitN(m.fwd.query.View(), "\n", 2)[0]
	lines = append(lines, paint("│", border, bg)+paint(" ", t.Text, bg, false)+t.padANSI(field, contentW-1, bg)+paint("│", border, bg))
	lines = append(lines, paint("├"+strings.Repeat("─", contentW)+"┤", t.Border, bg))

	chats := m.forwardChats()
	rows := p.h - 5
	if m.fwd.sel >= len(chats) {
		m.fwd.sel = maxInt(0, len(chats)-1)
	}
	if len(chats) == 0 {
		lines = append(lines, paint("│", border, bg)+t.pad("  No chats found", contentW, t.Muted, bg, false)+paint("│", border, bg))
	} else {
		for i := 0; i < rows; i++ {
			if i >= len(chats) {
				lines = append(lines, paint("│", border, bg)+t.fill(contentW, bg)+paint("│", border, bg))
				continue
			}
			chat := &m.chats[chats[i]]
			fg, rowBG := t.Text, bg
			if i == m.fwd.sel {
				fg, rowBG = t.OnPrimary, t.Primary
			} else if m.hover && m.forwardRowRects()[i].contains(m.hoverX, m.hoverY) {
				rowBG = t.Selected
			}
			label := " " + chat.Name
			if chat.Archived {
				label += " · archive"
			}
			lines = append(lines, paint("│", border, bg)+t.pad(label, contentW, fg, rowBG, false)+paint("│", border, bg))
		}
	}
	lines = append(lines, paint("╰"+strings.Repeat("─", contentW)+"╯", border, bg))
	return lines, p.x, p.y
}

func (m *model) handleForwardKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.Keystroke()
	chats := m.forwardChats()
	switch key {
	case "esc", "ctrl+c":
		m.fwd = nil
		return nil
	case "up", "k":
		if m.fwd.sel > 0 {
			m.fwd.sel--
		}
		return nil
	case "down", "j":
		if m.fwd.sel < len(chats)-1 {
			m.fwd.sel++
		}
		return nil
	case "enter":
		if len(chats) > 0 {
			m.forwardTo(chats[m.fwd.sel])
		}
		return nil
	}
	var cmd tea.Cmd
	m.fwd.query, cmd = m.fwd.query.Update(msg)
	m.fwd.sel = 0
	return cmd
}

func (m *model) forwardClick(e tea.Mouse, _ layout) tea.Cmd {
	p := m.forwardPanelRect()
	if !p.contains(e.X, e.Y) {
		m.fwd = nil
		return nil
	}
	chats := m.forwardChats()
	for i, r := range m.forwardRowRects() {
		if r.contains(e.X, e.Y) && i < len(chats) {
			m.forwardTo(chats[i])
			return nil
		}
	}
	return nil
}

func (m *model) forwardTo(chatIndex int) {
	f := m.fwd
	if f == nil || chatIndex < 0 || chatIndex >= len(m.chats) || f.chatIndex < 0 || f.chatIndex >= len(m.chats) {
		return
	}
	src := &m.chats[f.chatIndex]
	dst := &m.chats[chatIndex]
	if f.chatIndex == chatIndex {
		m.fwd = nil
		return
	}
	m.fwd = nil
	if m.bridge != nil && src.PeerKey != "" && dst.PeerKey != "" {
		m.bridge.Forward(src.PeerKey, dst.PeerKey, []int{f.msgID})
		m.notice = "forwarding to " + dst.Name + "…"
		return
	}

	var copyMsg *Message
	for i := range src.Messages {
		if src.Messages[i].ID == f.msgID {
			copyMsg = &src.Messages[i]
			break
		}
	}
	if copyMsg == nil {
		return
	}
	nextID := 1
	for i := range dst.Messages {
		if dst.Messages[i].ID >= nextID {
			nextID = dst.Messages[i].ID + 1
		}
	}
	forwarded := Message{
		ID:          nextID,
		Text:        copyMsg.Text,
		Time:        time.Now().Format("15:04"),
		Out:         true,
		ForwardFrom: src.Name,
		Voice:       copyMsg.Voice,
		Photo:       copyMsg.Photo,
		Round:       copyMsg.Round,
		Checks:      "✓",
	}
	dst.Messages = append(dst.Messages, forwarded)
	dst.Preview = "You: " + oneLine(mediaPlaceholder(forwarded))
	dst.Time = forwarded.Time
	m.msgVersion++
	m.notice = "forwarded to " + dst.Name
}

func mediaPlaceholder(msg Message) string {
	switch {
	case msg.Text != "":
		return msg.Text
	case msg.Voice != nil:
		return "Voice message"
	case msg.Photo != nil:
		return "Photo"
	case msg.Round != nil:
		return "Round video"
	default:
		return fmt.Sprintf("message #%d", msg.ID)
	}
}
