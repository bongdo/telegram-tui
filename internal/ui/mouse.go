package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *model) handleClick(e tea.Mouse) tea.Cmd {
	if e.Button != tea.MouseLeft {
		return nil
	}
	g := m.layout()
	if g.status.contains(e.X, e.Y) && m.quitRect(g).contains(e.X, e.Y) {
		return m.close()
	}
	if m.auth != nil {
		return m.authClick(e, g)
	}
	if m.attach != nil {
		return m.attachClick(e)
	}
	if m.qr != nil {
		return nil
	}
	if m.fwd != nil {
		return m.forwardClick(e, g)
	}
	m.notice = ""
	m.hoverX, m.hoverY, m.hover = e.X, e.Y, true

	if m.viewer.open {
		return m.viewerClick(e)
	}
	if m.ctxKind != ctxNone {
		return m.ctxClick(e)
	}
	if m.emojiOpen {
		return m.emojiClick(g, e)
	}
	if g.status.contains(e.X, e.Y) {
		if m.quitRect(g).contains(e.X, e.Y) {
			return m.close()
		}
		return nil
	}

	if g.compact {
		if m.compactPane == paneConversation && m.active >= 0 {
			return m.centerClick(g, e)
		}
		m.leftClick(g, e)
		return nil
	}

	switch {
	case g.profile && g.right.contains(e.X, e.Y):
		m.rightClick(g, e)
	case g.center.contains(e.X, e.Y):
		return m.centerClick(g, e)
	case g.left.contains(e.X, e.Y):
		m.leftClick(g, e)
	}
	return nil
}

func (m *model) handleRightClick(e tea.Mouse) {
	if m.auth != nil || m.attach != nil || m.qr != nil {
		return
	}
	if m.fwd != nil {
		m.fwd = nil
		return
	}
	m.hoverX, m.hoverY, m.hover = e.X, e.Y, true
	if m.viewer.open {
		m.viewer = viewerState{}
		return
	}
	if m.emojiOpen {
		m.emojiOpen = false
		return
	}
	if m.ctxKind != ctxNone {
		m.ctxKind = ctxNone
		return
	}

	g := m.layout()
	if g.compact {
		if m.compactPane == paneChats {
			m.chatMenu(g, e)
			return
		}
	} else if g.left.contains(e.X, e.Y) {
		m.chatMenu(g, e)
		return
	}
	if !g.center.contains(e.X, e.Y) {
		return
	}
	msgTop := g.center.y + centerHeaderHeight
	mv := m.messageView(g)
	if e.Y < msgTop || e.Y >= msgTop+mv.h {
		return
	}
	idx, _, _, ok := m.messageHit(g, e.X, e.Y)
	if !ok {
		return
	}
	chat := m.activeChat()
	if chat == nil || idx < 0 || idx >= len(chat.Messages) {
		return
	}
	m.selectedMsg = chat.Messages[idx].ID
	m.setFocus(focusMessages)
	m.openCtx(ctxMessage, e.X, e.Y, idx, -1)
}

func (m *model) chatMenu(g layout, e tea.Mouse) {
	if e.Y < leftHeaderRows {
		return
	}
	pos := m.chatScroll + (e.Y-leftHeaderRows)/2
	ids := m.filteredChats()
	if pos < 0 || pos >= len(ids) {
		return
	}
	m.openCtx(ctxChat, e.X, e.Y, -1, ids[pos])
}

func (m *model) leftClick(g layout, e tea.Mouse) {
	t, c := m.leftHeaderButtons(g)
	if t.contains(e.X, e.Y) {
		m.toggleTheme()
		return
	}
	if c.contains(e.X, e.Y) {
		m.setFocus(focusComposer)
		m.notice = "new message: pick a chat first"
		return
	}
	if e.Y == 1 {
		if m.search.Value() != "" && m.searchClearRect(g).contains(e.X, e.Y) {
			m.search.Reset()
			m.chatScroll = 0
			m.selected = 0
		}
		m.setFocus(focusSearch)
		return
	}
	if e.Y == 2 {
		for _, tab := range m.folderTabs(g.left.w) {
			if e.X >= g.left.x+tab.x && e.X < g.left.x+tab.x+tab.w {
				m.setFolder(tab.folder)
				return
			}
		}
		return
	}
	if e.Y >= leftHeaderRows {
		pos := m.chatScroll + (e.Y-leftHeaderRows)/2
		ids := m.filteredChats()
		if pos >= 0 && pos < len(ids) {
			m.openFiltered(pos)
			m.setFocus(focusChats)
		}
	}
}

func (m *model) centerClick(g layout, e tea.Mouse) tea.Cmd {
	// Compact back arrow.
	if g.compact && e.Y < 2 && e.X >= g.center.x+1 && e.X < g.center.x+3 {
		m.compactPane = paneChats
		return nil
	}

	searchR, callR, infoR := m.headerButtons(g)
	switch {
	case searchR.contains(e.X, e.Y):
		m.setFocus(focusSearch)
		return nil
	case callR.contains(e.X, e.Y):
		m.notice = "calls are not part of the mock yet"
		return nil
	case infoR.contains(e.X, e.Y):
		m.toggleProfile()
		return nil
	}
	if e.Y < g.center.y+centerHeaderHeight {
		m.toggleProfile()
		return nil
	}

	msgTop := g.center.y + centerHeaderHeight
	mv := m.messageView(g)
	if e.Y >= msgTop && e.Y < msgTop+mv.h {
		m.ta.ClearSelection()
		idx, voice, seek, ok := m.messageHit(g, e.X, e.Y)
		if !ok {
			return nil
		}
		chat := m.activeChat()
		if chat == nil || idx < 0 || idx >= len(chat.Messages) {
			return nil
		}
		msg := &chat.Messages[idx]
		wasSelected := m.selectedMsg == msg.ID
		now := time.Now()
		double := wasSelected && now.Sub(m.lastClick) < 450*time.Millisecond
		m.selectedMsg = msg.ID
		m.lastClick = now

		if voice {
			if seek > 0 && m.playingID == msg.ID {
				if m.player != nil {
					if err := m.player.Seek(seek); err != nil {
						m.notice = err.Error()
					}
				}
				return nil
			}
			m.setFocus(focusMessages)
			return m.playVoice(*msg)
		}
		if msg.Photo != nil {
			m.setFocus(focusMessages)
			return m.openPhoto(*msg)
		}
		if msg.Round != nil {
			m.setFocus(focusMessages)
			return m.playRound(*msg)
		}

		if double {
			m.replyToSelected()
		} else {
			m.setFocus(focusMessages)
		}
		return nil
	}

	top := m.composerTop(g)
	if e.Y >= top {
		if (m.reply != nil || m.editingID != 0) && m.replyCloseRect(g).contains(e.X, e.Y) {
			if m.editingID != 0 {
				m.cancelEdit()
			} else {
				m.reply = nil
				m.replyToID = 0
				m.syncInput()
			}
			return nil
		}
		if m.sendRect(g).contains(e.X, e.Y) {
			m.send()
			return nil
		}
		if m.emojiRect(g).contains(e.X, e.Y) {
			if !m.canEmoji() {
				m.notice = "terminal is too small for the emoji panel"
				return nil
			}
			m.emojiOpen = !m.emojiOpen
			m.ctxKind = ctxNone
			return nil
		}
		if m.attachRect(g).contains(e.X, e.Y) {
			m.openAttach()
			return nil
		}
		if ir := m.inputRect(g); ir.contains(e.X, e.Y) {
			m.composerSel = true
			m.ta.BeginSelection(e.X-ir.x, e.Y-ir.y)
			now := time.Now()
			double := now.Sub(m.composerLastClick) < 450*time.Millisecond
			m.composerLastClick = now
			if double {
				row := m.ta.Line()
				if start, end, ok := m.composerWordRange(); ok {
					x1, y1 := m.taCoord(row, start)
					x2, y2 := m.taCoord(row, end)
					m.ta.BeginSelection(x2, y2)
					m.ta.ExtendSelection(x1, y1)
					m.ta.EndSelection()
				}
			}
		} else {
			m.ta.ClearSelection()
		}
		m.setFocus(focusComposer)
	}
	return nil
}

func (m *model) rightClick(g layout, e tea.Mouse) {
	if m.profileCloseRect(g).contains(e.X, e.Y) {
		m.profileOpen = false
		m.syncInput()
		return
	}
	infoR, mediaR, filesR := m.profileTabRects(g)
	switch {
	case infoR.contains(e.X, e.Y):
		m.profileTab = 0
	case mediaR.contains(e.X, e.Y):
		m.profileTab = 1
	case filesR.contains(e.X, e.Y):
		m.profileTab = 2
	}
	msgR, muteR, moreR := m.profileActionRects(g)
	switch {
	case msgR.contains(e.X, e.Y):
		m.setFocus(focusComposer)
	case muteR.contains(e.X, e.Y):
		if c := m.activeChat(); c != nil {
			c.Muted = !c.Muted
			if c.Muted {
				m.notice = c.Name + " muted"
			} else {
				m.notice = c.Name + " unmuted"
			}
		}
	case moreR.contains(e.X, e.Y):
		m.notice = "more actions are not wired yet"
	}
}

type wheelTickMsg struct{}

// Filter drops mouse-motion events that report the cell the pointer already
// sits on. High-rate pointers emit many of those; each one would otherwise cost
// a full Update+View. The last real position is never lost, because a movement
// to a new cell always passes.
func Filter(m tea.Model, msg tea.Msg) tea.Msg {
	motion, ok := msg.(tea.MouseMotionMsg)
	if !ok {
		return msg
	}
	if mv, ok := m.(*model); ok && mv.hover {
		if e := motion.Mouse(); e.X == mv.hoverX && e.Y == mv.hoverY {
			return nil
		}
	}
	return msg
}

func (m *model) handleWheel(e tea.Mouse) tea.Cmd {
	if m.auth != nil || m.attach != nil || m.qr != nil || m.viewer.open || m.ctxKind != ctxNone || m.emojiOpen || m.fwd != nil {
		return nil
	}
	up := e.Button == tea.MouseWheelUp
	if !up && e.Button != tea.MouseWheelDown {
		return nil
	}
	delta := 1
	if up {
		delta = -1
	}

	g := m.layout()
	if g.left.contains(e.X, e.Y) && e.Y == 2 {
		m.cycleFolder(delta)
		return nil
	}

	switch {
	case g.compact && m.compactPane == paneChats:
		m.wheelChats += delta
	case g.compact:
		m.wheelMsgs -= delta * 3
	case g.left.contains(e.X, e.Y):
		m.wheelChats += delta
	case g.center.contains(e.X, e.Y):
		m.wheelMsgs -= delta * 3
	}
	if m.wheelTicking || (m.wheelChats == 0 && m.wheelMsgs == 0) {
		return nil
	}
	m.wheelTicking = true
	return tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg { return wheelTickMsg{} })
}

func (m *model) applyWheel() {
	m.wheelTicking = false
	if m.wheelChats != 0 {
		g := m.layout()
		ids := m.filteredChats()
		visible := (g.contentH - leftHeaderRows) / 2
		maxScroll := maxInt(0, len(ids)-visible)
		m.chatScroll = clamp(m.chatScroll+m.wheelChats, 0, maxScroll)
		m.wheelChats = 0
	}
	if m.wheelMsgs != 0 {
		g := m.layout()
		mv := m.messageView(g)
		maxScroll := maxInt(0, len(mv.canvas.lines)-mv.h)
		m.msgScroll = clamp(m.msgScroll+m.wheelMsgs, 0, maxScroll)
		m.wheelMsgs = 0
	}
}
