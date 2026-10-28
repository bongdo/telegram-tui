package ui

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bongdo/telegram-tui/internal/chafa"
)

type ctxKind int

const (
	ctxNone ctxKind = iota
	ctxMessage
	ctxChat
)

type viewerState struct {
	open    bool
	loading bool
	path    string
	title   string
	lines   []string
	err     string
	panel   rect
}

type imageReadyMsg struct {
	path  string
	lines []string
	err   error
}

var quickReactions = []string{"❤️", "👍", "🔥", "😂", "😢", "🎉"}

var emojiGrid = []string{
	"😀", "😃", "😄", "😁", "😆", "😅", "😂", "🙂",
	"🙃", "😉", "😊", "😍", "🥰", "😘", "😎", "🤔",
	"😐", "😴", "😮", "😢", "😭", "😡", "👍", "👎",
	"👏", "🙏", "💪", "❤️", "🔥", "🎉", "✅", "⭐",
}

const (
	ctxWidth    = 26
	emojiCols   = 8
	emojiRows   = 4
	emojiCell   = 3
	emojiPanelW = emojiCols*emojiCell + 2
	emojiPanelH = emojiRows + 3
)

func (m *model) ctxItems() []string {
	if m.ctxKind == ctxChat {
		if m.ctxChat < 0 || m.ctxChat >= len(m.chats) {
			return nil
		}
		chat := &m.chats[m.ctxChat]
		mute := "Mute"
		if chat.Muted {
			mute = "Unmute"
		}
		pin := "Pin"
		if chat.Pinned {
			pin = "Unpin"
		}
		return []string{"Open", "Mark as read", mute, pin}
	}
	items := []string{"Reply", "Copy text"}
	if chat := m.activeChat(); chat != nil && m.ctxMsg >= 0 && m.ctxMsg < len(chat.Messages) {
		msg := &chat.Messages[m.ctxMsg]
		if msg.Out && !msg.Service && msg.Text != "" {
			items = append(items, "Edit")
		}
	}
	return append(items, "Forward", "Delete")
}

func (m *model) ctxMenuHeight() int {
	n := len(m.ctxItems())
	if m.ctxKind == ctxMessage {
		return n + 4
	}
	return n + 2
}

func (m *model) ctxMenuRect() rect {
	return rect{m.ctxX, m.ctxY, ctxWidth, m.ctxMenuHeight()}
}

func (m *model) ctxReactionRects() []rect {
	if m.ctxKind != ctxMessage {
		return nil
	}
	out := make([]rect, 0, len(quickReactions))
	for i := range quickReactions {
		out = append(out, rect{m.ctxX + 1 + i*4, m.ctxY + 1, 4, 1})
	}
	return out
}

func (m *model) ctxItemRects() []rect {
	items := m.ctxItems()
	start := 1
	if m.ctxKind == ctxMessage {
		start = 3
	}
	out := make([]rect, 0, len(items))
	for i := range items {
		out = append(out, rect{m.ctxX + 1, m.ctxY + start + i, ctxWidth - 2, 1})
	}
	return out
}

func (m *model) openCtx(kind ctxKind, x, y, msg, chat int) {
	if m.w < ctxWidth {
		return
	}
	m.ctxKind = kind
	m.ctxMsg = msg
	m.ctxChat = chat
	m.ctxSel = 0
	m.ctxX = clamp(x, 0, maxInt(0, m.w-ctxWidth))
	m.ctxY = clamp(y, 0, maxInt(0, m.h-m.ctxMenuHeight()))
}

func (m *model) renderCtxMenu() ([]string, int, int) {
	t := m.theme
	bg := t.PanelAlt
	border := t.Primary
	lines := []string{paint("╭"+strings.Repeat("─", ctxWidth-2)+"╮", border, bg)}

	if m.ctxKind == ctxMessage {
		var row strings.Builder
		rects := m.ctxReactionRects()
		for i, e := range quickReactions {
			cellBG := bg
			cellFG := t.Text
			if m.hover && rects[i].contains(m.hoverX, m.hoverY) {
				cellBG = t.Selected
				cellFG = t.Primary
			}
			row.WriteString(paint(padCenter(e, 4), cellFG, cellBG, false))
		}
		lines = append(lines, paint("│", border, bg)+row.String()+paint("│", border, bg))
		lines = append(lines, paint("├"+strings.Repeat("─", ctxWidth-2)+"┤", t.Border, bg))
	}

	for i, label := range m.ctxItems() {
		fg := t.Text
		rowBG := bg
		bold := false
		rects := m.ctxItemRects()
		if (m.hover && rects[i].contains(m.hoverX, m.hoverY)) || i == m.ctxSel {
			fg = t.OnPrimary
			rowBG = t.Primary
			bold = true
		}
		lines = append(lines, paint("│", border, bg)+t.pad(" "+label, ctxWidth-2, fg, rowBG, bold)+paint("│", border, bg))
	}
	lines = append(lines, paint("╰"+strings.Repeat("─", ctxWidth-2)+"╯", border, bg))
	return lines, m.ctxX, m.ctxY
}

func (m *model) ctxClick(e tea.Mouse) tea.Cmd {
	if !m.ctxMenuRect().contains(e.X, e.Y) {
		m.ctxKind = ctxNone
		return nil
	}
	if m.ctxKind == ctxMessage {
		for i, r := range m.ctxReactionRects() {
			if r.contains(e.X, e.Y) {
				m.toggleReaction(m.ctxMsg, quickReactions[i])
				m.ctxKind = ctxNone
				return nil
			}
		}
	}
	for i, r := range m.ctxItemRects() {
		if r.contains(e.X, e.Y) {
			return m.ctxAction(i)
		}
	}
	return nil
}

func (m *model) ctxAction(i int) tea.Cmd {
	kind := m.ctxKind
	m.ctxKind = ctxNone
	if kind == ctxChat {
		if m.ctxChat < 0 || m.ctxChat >= len(m.chats) {
			return nil
		}
		chat := &m.chats[m.ctxChat]
		switch i {
		case 0:
			m.active = m.ctxChat
			m.selected = 0
			ids := m.filteredChats()
			for pos, id := range ids {
				if id == m.ctxChat {
					m.selected = pos
					break
				}
			}
			chat.Unread = 0
			m.msgScroll = 0
			m.compactPane = paneConversation
			m.setFocus(focusMessages)
			m.ensureHistory()
		case 1:
			chat.Unread = 0
			maxID := 0
			for _, msg := range chat.Messages {
				if msg.ID > maxID {
					maxID = msg.ID
				}
			}
			if m.bridge != nil {
				m.bridge.MarkRead(chat.PeerKey, maxID)
			}
			m.notice = chat.Name + " marked as read"
		case 2:
			chat.Muted = !chat.Muted
			if chat.Muted {
				m.notice = chat.Name + " muted"
			} else {
				m.notice = chat.Name + " unmuted"
			}
		case 3:
			chat.Pinned = !chat.Pinned
			if chat.Pinned {
				m.notice = chat.Name + " pinned"
			} else {
				m.notice = chat.Name + " unpinned"
			}
		}
		return nil
	}

	chat := m.activeChat()
	if chat == nil || m.ctxMsg < 0 || m.ctxMsg >= len(chat.Messages) {
		return nil
	}
	items := m.ctxItems()
	if i < 0 || i >= len(items) {
		return nil
	}
	msg := &chat.Messages[m.ctxMsg]
	m.selectedMsg = msg.ID
	switch items[i] {
	case "Reply":
		m.replyToSelected()
	case "Copy text":
		text := msg.Text
		if text == "" && msg.Voice != nil {
			text = fmt.Sprintf("Voice message · 0:%02d", msg.Voice.Duration)
		}
		if text == "" {
			text = msg.Sender
		}
		return tea.SetClipboard(text)
	case "Edit":
		m.beginEdit()
	case "Forward":
		m.beginForward()
	case "Delete":
		id := msg.ID
		if m.bridge != nil && chat.PeerKey != "" {
			m.bridge.Delete(chat.PeerKey, []int{id})
		}
		chat.Messages = append(chat.Messages[:m.ctxMsg], chat.Messages[m.ctxMsg+1:]...)
		m.msgScroll = 0
		m.msgVersion++
		m.notice = "message deleted"
	}
	return nil
}

func (m *model) toggleReaction(msgIndex int, emoji string) {
	chat := m.activeChat()
	if chat == nil || msgIndex < 0 || msgIndex >= len(chat.Messages) {
		return
	}
	m.msgVersion++
	msg := &chat.Messages[msgIndex]
	for i := range msg.Reactions {
		r := &msg.Reactions[i]
		if r.Emoji != emoji {
			continue
		}
		add := false
		if r.Mine {
			r.Count--
			r.Mine = false
		} else {
			r.Count++
			r.Mine = true
			add = true
		}
		if r.Count <= 0 {
			msg.Reactions = append(msg.Reactions[:i], msg.Reactions[i+1:]...)
		}
		m.sendReaction(chat, msg.ID, emoji, add)
		return
	}
	msg.Reactions = append(msg.Reactions, Reaction{Emoji: emoji, Count: 1, Mine: true})
	m.sendReaction(chat, msg.ID, emoji, true)
}

func (m *model) sendReaction(chat *Chat, msgID int, emoji string, add bool) {
	if m.bridge == nil || chat == nil || chat.PeerKey == "" || emoji == "◈" {
		return
	}
	m.bridge.SendReaction(chat.PeerKey, msgID, emoji, add)
}
func (m *model) emojiList() []string {
	out := make([]string, 0, emojiCols*emojiRows)
	seen := make(map[string]bool, len(m.recentEmoji)+len(emojiGrid))
	for _, e := range m.recentEmoji {
		if e != "" && !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	for _, e := range emojiGrid {
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	for len(out) < emojiCols*emojiRows {
		out = append(out, " ")
	}
	return out[:emojiCols*emojiRows]
}

func (m *model) noteEmoji(emoji string) {
	if emoji == "" || emoji == " " {
		return
	}
	next := []string{emoji}
	for _, e := range m.recentEmoji {
		if e != emoji {
			next = append(next, e)
		}
	}
	if len(next) > 16 {
		next = next[:16]
	}
	m.recentEmoji = next
	if m.bridge != nil {
		m.bridge.AddRecentEmoji(emoji)
	}
}

func (m *model) canEmoji() bool {
	return m.w >= emojiPanelW && m.h >= emojiPanelH+2
}

func (m *model) emojiPanelRect(g layout) rect {
	x := g.center.x + g.center.w - emojiPanelW - 1
	x = clamp(x, 0, maxInt(0, m.w-emojiPanelW))
	y := m.composerTop(g) - emojiPanelH
	y = clamp(y, 1, maxInt(1, m.h-emojiPanelH-1))
	return rect{x, y, emojiPanelW, emojiPanelH}
}

func (m *model) emojiCellRects(g layout) []rect {
	p := m.emojiPanelRect(g)
	list := m.emojiList()
	out := make([]rect, 0, len(list))
	for i := range list {
		col := i % emojiCols
		row := i / emojiCols
		out = append(out, rect{p.x + 1 + col*emojiCell, p.y + 2 + row, emojiCell, 1})
	}
	return out
}

func (m *model) renderEmojiPanel(g layout) ([]string, int, int) {
	t := m.theme
	bg := t.PanelAlt
	border := t.Primary
	p := m.emojiPanelRect(g)
	lines := []string{paint("╭"+strings.Repeat("─", emojiPanelW-2)+"╮", border, bg)}

	emojiFG := t.Muted
	stickersFG := t.Muted
	if m.emojiTab == 0 {
		emojiFG = t.OnPrimary
	} else {
		stickersFG = t.OnPrimary
	}
	emojiTabBG := bg
	stickersTabBG := bg
	if m.emojiTab == 0 {
		emojiTabBG = t.Primary
	} else {
		stickersTabBG = t.Primary
	}
	tabs := paint(" Emoji ", emojiFG, emojiTabBG, true)
	tabs += paint("  ", t.Text, bg, false)
	tabs += paint(" Stickers ", stickersFG, stickersTabBG, false)
	tabs += t.fill(emojiPanelW-2-ansiWidth(tabs), bg)
	lines = append(lines, paint("│", border, bg)+tabs+paint("│", border, bg))

	if m.emojiTab == 1 {
		lines = append(lines,
			paint("│"+t.pad("  No sticker packs yet", emojiPanelW-2, t.Text, bg, false)+"│", border, bg),
			paint("│"+t.pad("  They arrive with Telegram media", emojiPanelW-2, t.Muted, bg, false)+"│", border, bg),
			paint("│"+t.pad("", emojiPanelW-2, t.Text, bg, false)+"│", border, bg),
			paint("│"+t.pad("  Recent emoji are saved locally", emojiPanelW-2, t.Muted, bg, false)+"│", border, bg),
		)
	} else {
		rects := m.emojiCellRects(g)
		list := m.emojiList()
		for row := 0; row < emojiRows; row++ {
			var line strings.Builder
			for col := 0; col < emojiCols; col++ {
				idx := row*emojiCols + col
				cellBG := bg
				if m.hover && rects[idx].contains(m.hoverX, m.hoverY) {
					cellBG = t.Selected
				}
				line.WriteString(paint(padCenter(list[idx], emojiCell), t.Text, cellBG, false))
			}
			lines = append(lines, paint("│", border, bg)+line.String()+paint("│", border, bg))
		}
	}
	lines = append(lines, paint("╰"+strings.Repeat("─", emojiPanelW-2)+"╯", border, bg))
	return lines, p.x, p.y
}

func (m *model) emojiClick(g layout, e tea.Mouse) tea.Cmd {
	p := m.emojiPanelRect(g)
	if !p.contains(e.X, e.Y) {
		m.emojiOpen = false
		return nil
	}
	if e.Y == p.y+1 {
		if e.X < p.x+10 {
			m.emojiTab = 0
		} else {
			m.emojiTab = 1
		}
		return nil
	}
	if m.emojiTab != 0 {
		return nil
	}
	list := m.emojiList()
	for i, r := range m.emojiCellRects(g) {
		if r.contains(e.X, e.Y) {
			if list[i] == " " {
				return nil
			}
			m.ta.InsertString(list[i])
			m.noteEmoji(list[i])
			m.setFocus(focusComposer)
			m.syncInput()
			return nil
		}
	}
	return nil
}

func (m *model) viewerSize() (int, int) {
	return clamp(m.w-8, 10, 90), clamp(m.h-9, 3, 28)
}

func (m *model) openPhoto(msg Message) tea.Cmd {
	if msg.Photo == nil {
		return nil
	}
	path := msg.Photo.File
	if path == "" {
		if m.requestMedia(msg.ID, "photo") {
			return nil
		}
		path = os.Getenv("TG_TUI_DEMO_IMAGE")
	}
	if path == "" {
		m.notice = "mock photo: set TG_TUI_DEMO_IMAGE=/path/image.png"
		return nil
	}
	if !chafa.Available() {
		m.notice = "chafa not found in PATH"
		return nil
	}
	contentW, contentH := m.viewerSize()
	m.ctxKind = ctxNone
	m.emojiOpen = false
	m.viewer = viewerState{
		open:    true,
		loading: true,
		path:    path,
		title:   fmt.Sprintf("Photo · %d×%d", msg.Photo.W, msg.Photo.H),
	}
	return func() tea.Msg {
		lines, err := chafa.Render(path, contentW, contentH)
		return imageReadyMsg{path: path, lines: lines, err: err}
	}
}

func (m *model) renderViewer() ([]string, int, int) {
	t := m.theme
	bg := t.ChatBG
	border := t.Primary
	contentW, contentH := m.viewerSize()
	panelW := contentW + 2
	panelH := contentH + 3
	x := (m.w - panelW) / 2
	y := (m.h - panelH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	m.viewer.panel = rect{x, y, panelW, panelH}

	lines := []string{paint("╭"+strings.Repeat("─", panelW-2)+"╮", border, bg)}
	header := t.pad(" "+m.viewer.title, contentW-2, t.Text, bg, true)
	closeFG := t.Muted
	if m.hover && m.viewerCloseRect().contains(m.hoverX, m.hoverY) {
		closeFG = t.Primary
	}
	header += paint("✕ ", closeFG, bg, false)
	lines = append(lines, paint("│", border, bg)+header+paint("│", border, bg))

	for i := 0; i < contentH; i++ {
		var text string
		switch {
		case m.viewer.loading && i == 0:
			text = " Rendering with chafa…"
		case m.viewer.err != "" && i == 0:
			text = " " + m.viewer.err
		case i < len(m.viewer.lines):
			text = m.viewer.lines[i]
		}
		lines = append(lines, paint("│", border, bg)+t.padANSI(text, contentW, bg)+paint("│", border, bg))
	}
	lines = append(lines, paint("╰"+strings.Repeat("─", panelW-2)+"╯", border, bg))
	return lines, x, y
}

func (m *model) viewerCloseRect() rect {
	p := m.viewer.panel
	return rect{p.x + p.w - 3, p.y + 1, 2, 1}
}

func (m *model) viewerClick(e tea.Mouse) tea.Cmd {
	p := m.viewer.panel
	if !p.contains(e.X, e.Y) || m.viewerCloseRect().contains(e.X, e.Y) {
		m.viewer = viewerState{}
	}
	return nil
}

func (m *model) composeOverlays(base string) string {
	switch {
	case m.auth != nil:
		lines, x, y := m.renderAuth()
		return composeAt(base, lines, x, y)
	case m.attach != nil:
		lines, x, y := m.renderAttach()
		return composeAt(base, lines, x, y)
	case m.qr != nil:
		lines, x, y := m.renderQR()
		return composeAt(base, lines, x, y)
	case m.fwd != nil:
		lines, x, y := m.renderForward()
		return composeAt(base, lines, x, y)
	case m.viewer.open:
		lines, x, y := m.renderViewer()
		return composeAt(base, lines, x, y)
	case m.ctxKind != ctxNone:
		lines, x, y := m.renderCtxMenu()
		return composeAt(base, lines, x, y)
	case m.emojiOpen:
		lines, x, y := m.renderEmojiPanel(m.layout())
		return composeAt(base, lines, x, y)
	}
	return base
}

func composeAt(base string, panel []string, x, y int) string {
	if len(panel) == 0 {
		return base
	}
	baseLayer := lipgloss.NewLayer(base)
	panelLayer := lipgloss.NewLayer(strings.Join(panel, "\n")).X(x).Y(y).Z(1)
	return lipgloss.NewCompositor(baseLayer, panelLayer).Render()
}

func ansiWidth(s string) int {
	return lipgloss.Width(s)
}
