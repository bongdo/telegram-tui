package ui

import (
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type attachState struct {
	err string
}

const attachPanelH = 6

func (m *model) openAttach() {
	if m.bridge == nil {
		m.notice = "file upload needs a Telegram session"
		return
	}
	chat := m.activeChat()
	if chat == nil || chat.PeerKey == "" {
		m.notice = "open a Telegram chat first"
		return
	}
	m.attach = &attachState{}
	m.attachField.SetValue("")
	m.attachField.Focus()
	m.emojiOpen = false
	m.ctxKind = ctxNone
	m.viewer = viewerState{}
}

func (m *model) attachPanelRect() rect {
	w := clamp(m.w-8, 36, 72)
	if w > m.w-2 {
		w = m.w - 2
	}
	h := attachPanelH
	x := (m.w - w) / 2
	y := (m.h - h) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return rect{x, y, w, h}
}

func (m *model) attachInputRect() rect {
	p := m.attachPanelRect()
	return rect{p.x + 2, p.y + 2, p.w - 4, 1}
}

func (m *model) attachSendRect() rect {
	p := m.attachPanelRect()
	contentW := p.w - 2
	start := p.x + 1 + (contentW-24)/2
	return rect{start, p.y + 4, 10, 1}
}

func (m *model) attachCancelRect() rect {
	send := m.attachSendRect()
	return rect{send.x + 12, send.y, 12, 1}
}

func (m *model) handleAttachKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Keystroke() {
	case "ctrl+c":
		return m.close()
	case "esc":
		m.attach = nil
		m.attachField.Blur()
		return nil
	case "enter":
		m.submitAttach()
		return nil
	}
	var cmd tea.Cmd
	m.attachField, cmd = m.attachField.Update(msg)
	return cmd
}

func (m *model) attachClick(e tea.Mouse) tea.Cmd {
	if !m.attachPanelRect().contains(e.X, e.Y) {
		m.attach = nil
		m.attachField.Blur()
		return nil
	}
	if m.attachSendRect().contains(e.X, e.Y) {
		m.submitAttach()
		return nil
	}
	if m.attachCancelRect().contains(e.X, e.Y) {
		m.attach = nil
		m.attachField.Blur()
		return nil
	}
	if m.attachInputRect().contains(e.X, e.Y) {
		m.attachField.Focus()
	}
	return nil
}

func (m *model) submitAttach() {
	chat := m.activeChat()
	path := strings.TrimSpace(m.attachField.Value())
	path = strings.Trim(path, "\"")
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	if path == "" {
		m.notice = "file path is required"
		return
	}
	if m.bridge == nil || chat == nil || chat.PeerKey == "" {
		m.notice = "open a Telegram chat first"
		return
	}
	m.bridge.SendFile(chat.PeerKey, path, m.replyToID)
	m.attach = nil
	m.attachField.Reset()
	m.attachField.Blur()
	m.reply = nil
	m.replyToID = 0
	m.syncInput()
	m.notice = "uploading " + path
}

func (m *model) renderAttach() ([]string, int, int) {
	t := m.theme
	bg := t.PanelAlt
	border := t.Primary
	p := m.attachPanelRect()
	contentW := p.w - 2
	lines := []string{paint("╭"+strings.Repeat("─", contentW)+"╮", border, bg)}
	lines = append(lines, paint("│", border, bg)+t.pad(" Send file", contentW, t.Text, bg, true)+paint("│", border, bg))

	inputW := maxInt(1, contentW-2)
	m.attachField.SetWidth(maxInt(1, inputW-1))
	field := strings.SplitN(m.attachField.View(), "\n", 2)[0]
	row := paint("│", border, bg) + paint(" ", t.Text, bg) + t.padANSI(field, inputW, bg) + paint(" ", t.Text, bg) + paint("│", border, bg)
	lines = append(lines, row)
	lines = append(lines, paint("│", border, bg)+t.fill(contentW, bg)+paint("│", border, bg))

	sendRect := m.attachSendRect()
	cancelRect := m.attachCancelRect()
	sendFG := t.OnPrimary
	sendBG := t.Primary
	if m.hover && sendRect.contains(m.hoverX, m.hoverY) {
		sendBG = t.ChatActive
	}
	cancelFG := t.Text
	cancelBG := bg
	if m.hover && cancelRect.contains(m.hoverX, m.hoverY) {
		cancelBG = t.Selected
	}
	start := sendRect.x - (p.x + 1)
	buttons := t.fill(start, bg)
	buttons += t.pad(" Send ", 10, sendFG, sendBG, true)
	buttons += t.fill(2, bg)
	buttons += t.pad(" Cancel ", 12, cancelFG, cancelBG, false)
	buttons += t.fill(contentW-start-24, bg)
	lines = append(lines, paint("│", border, bg)+buttons+paint("│", border, bg))
	lines = append(lines, paint("╰"+strings.Repeat("─", contentW)+"╯", border, bg))
	return lines, p.x, p.y
}
