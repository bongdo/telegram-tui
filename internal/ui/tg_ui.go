package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"rsc.io/qr"

	"github.com/bongdo/telegram-tui/internal/domain"
	"github.com/bongdo/telegram-tui/internal/tgclient"
)

type authState struct {
	kind tgclient.AuthKind
	hint string
}

func waitBridge(b *tgclient.Bridge) tea.Cmd {
	if b == nil {
		return nil
	}
	return func() tea.Msg {
		return <-b.Events()
	}
}

func (m *model) handleTGEvent(ev tgclient.Event) tea.Cmd {
	switch e := ev.(type) {
	case tgclient.NoticeEvent:
		if e.Err != nil {
			m.notice = e.Err.Error()
		} else {
			m.notice = e.Text
		}

	case tgclient.StatusEvent:
		m.tgStatus = e.Text
		if e.Err != nil {
			m.tgErr = e.Err.Error()
			m.notice = e.Err.Error()
			m.qr = nil
		} else if e.Text == "connected" {
			m.tgErr = ""
			m.qr = nil
		}

	case tgclient.AuthPrompt:
		m.auth = &authState{kind: e.Kind, hint: e.Hint}
		m.authField.SetValue("")
		if e.Kind == tgclient.AuthPassword {
			m.authField.EchoMode = textinput.EchoPassword
		} else {
			m.authField.EchoMode = textinput.EchoNormal
		}
		m.authField.Focus()
		m.emojiOpen = false
		m.ctxKind = ctxNone
		m.viewer = viewerState{}

	case tgclient.QRTokenEvent:
		m.showQR(e.URL)

	case tgclient.SelfEvent:
		m.qr = nil
		m.selfName = e.Name
	case tgclient.DialogsEvent:
		prev := m.activePeerKey()
		oldMessages := make(map[string][]domain.Message, len(m.chats))
		for _, chat := range m.chats {
			oldMessages[chat.PeerKey] = chat.Messages
		}
		m.chats = e.Chats
		if len(e.Folders) > 0 {
			m.folders = e.Folders
		}
		if !m.validFolder(m.folder) {
			m.folder = folderAll
		}
		for i := range m.chats {
			if len(m.chats[i].Messages) == 0 {
				m.chats[i].Messages = oldMessages[m.chats[i].PeerKey]
			}
		}
		m.active = 0
		m.selected = 0
		if prev != "" {
			for i := range m.chats {
				if m.chats[i].PeerKey == prev {
					m.active = i
					break
				}
			}
		}
		ids := m.filteredChats()
		m.selected = 0
		found := false
		for pos, id := range ids {
			if id == m.active {
				m.selected = pos
				found = true
				break
			}
		}
		if !found {
			if len(ids) > 0 {
				m.active = ids[0]
			} else {
				m.active = -1
			}
		}
		if !e.Cached {
			for i := range m.chats {
				m.chats[i].HistoryLoaded = false
			}
		}
		m.msgVersion++
		m.chatScroll = 0
		m.msgScroll = 0
		m.historyLoading = make(map[string]bool)
		m.profileOpen = false
		m.reply = nil
		m.replyToID = 0
		m.selectedMsg = 0
		m.compactPane = paneConversation
		m.syncInput()
		m.ensureHistory()
	case tgclient.HistoryEvent:
		delete(m.historyLoading, e.PeerKey)
		chat := m.chatByKey(e.PeerKey)
		if chat == nil {
			return nil
		}
		if e.Err != nil {
			chat.HistoryLoaded = false
			m.notice = "history: " + e.Err.Error()
			if len(chat.Messages) == 0 {
				chat.Messages = []domain.Message{{Service: true, Text: "Failed to load history"}}
			}
			return nil
		}
		chat.HistoryLoaded = true
		chat.Messages = e.Messages
		m.msgVersion++
		if m.activePeerKey() == e.PeerKey {
			m.msgScroll = 0
			m.selectedMsg = 0
		}

	case tgclient.NewMessageEvent:
		chat := m.chatByKey(e.PeerKey)
		if chat == nil {
			m.chats = append(m.chats, e.Chat)
			chat = &m.chats[len(m.chats)-1]
		}
		chat.Preview = e.Chat.Preview
		chat.Time = e.Chat.Time
		chat.Messages = insertMessage(chat.Messages, e.Message)
		m.msgVersion++
		active := m.activePeerKey() == e.PeerKey
		if !e.Message.Out && !active {
			chat.Unread++
		}
		if active && m.bridge != nil {
			m.bridge.MarkRead(e.PeerKey, e.Message.ID)
		}

	case tgclient.EditMessageEvent:
		chat := m.chatByKey(e.PeerKey)
		if chat == nil {
			return nil
		}
		for i := range chat.Messages {
			if chat.Messages[i].ID == e.MessageID {
				chat.Messages[i].Text = e.Text
				chat.Messages[i].Edited = e.Edited
				break
			}
		}
		m.msgVersion++
		if m.editingID == e.MessageID {
			m.finishEdit()
		}

	case tgclient.ReactionsEvent:
		chat := m.chatByKey(e.PeerKey)
		if chat == nil {
			return nil
		}
		for i := range chat.Messages {
			if chat.Messages[i].ID == e.MessageID {
				chat.Messages[i].Reactions = e.Reactions
				break
			}
		}
		m.msgVersion++

	case tgclient.DeleteEvent:
		key := e.PeerKey
		if key == "" {
			key = m.activePeerKey()
		}
		chat := m.chatByKey(key)
		if chat == nil {
			return nil
		}
		remove := make(map[int]bool, len(e.IDs))
		for _, id := range e.IDs {
			remove[id] = true
		}
		kept := chat.Messages[:0]
		for _, msg := range chat.Messages {
			if !remove[msg.ID] {
				kept = append(kept, msg)
			}
		}
		chat.Messages = kept
		m.msgVersion++

	case tgclient.MediaReadyEvent:
		return m.handleMediaReady(e)
	}
	return nil
}

func (m *model) handleMediaReady(e tgclient.MediaReadyEvent) tea.Cmd {
	chat := m.chatByKey(e.PeerKey)
	if chat == nil {
		return nil
	}
	idx := -1
	for i := range chat.Messages {
		if chat.Messages[i].ID == e.MessageID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	pending := m.pendingMedia[mediaKey(e.PeerKey, e.MessageID)]
	delete(m.pendingMedia, mediaKey(e.PeerKey, e.MessageID))
	if e.Err != nil {
		m.notice = "media: " + e.Err.Error()
		return nil
	}
	msg := &chat.Messages[idx]
	switch e.Kind {
	case "voice":
		if msg.Voice != nil {
			msg.Voice.File = e.Path
		}
	case "round":
		if msg.Round != nil {
			msg.Round.File = e.Path
		}
	case "photo":
		if msg.Photo != nil {
			msg.Photo.File = e.Path
		}
	}
	m.msgVersion++
	switch pending {
	case "voice":
		return m.playVoice(*msg)
	case "round":
		return m.playRound(*msg)
	case "photo":
		return m.openPhoto(*msg)
	}
	return nil
}

func (m *model) chatByKey(key string) *Chat {
	if key == "" {
		return nil
	}
	for i := range m.chats {
		if m.chats[i].PeerKey == key {
			return &m.chats[i]
		}
	}
	return nil
}

func (m *model) activePeerKey() string {
	if c := m.activeChat(); c != nil {
		return c.PeerKey
	}
	return ""
}

func (m *model) ensureHistory() {
	if m.bridge == nil {
		return
	}
	chat := m.activeChat()
	if chat == nil || chat.PeerKey == "" {
		return
	}
	if chat.HistoryLoaded {
		m.bridge.MarkRead(chat.PeerKey, lastMessageID(chat))
		return
	}
	if m.historyLoading[chat.PeerKey] {
		return
	}
	m.historyLoading[chat.PeerKey] = true
	m.bridge.LoadHistory(chat.PeerKey)
}

func lastMessageID(chat *Chat) int {
	if chat == nil {
		return 0
	}
	for i := len(chat.Messages) - 1; i >= 0; i-- {
		if !chat.Messages[i].Service && chat.Messages[i].ID > 0 {
			return chat.Messages[i].ID
		}
	}
	return 0
}

func mediaKey(peer string, id int) string {
	return fmt.Sprintf("%s|%d", peer, id)
}

func insertMessage(msgs []domain.Message, msg domain.Message) []domain.Message {
	for i := range msgs {
		if msgs[i].ID == msg.ID && !msgs[i].Service {
			msgs[i] = msg
			return msgs
		}
	}
	if msg.Date > 0 {
		day := time.Unix(msg.Date, 0).Format("2006-01-02")
		var lastDay string
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Date > 0 {
				lastDay = time.Unix(msgs[i].Date, 0).Format("2006-01-02")
				break
			}
		}
		if day != lastDay {
			msgs = append(msgs, domain.Message{Service: true, Text: messageDayLabel(time.Unix(msg.Date, 0))})
		}
	}
	return append(msgs, msg)
}

func (m *model) handleAuthKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.Keystroke()
	switch key {
	case "ctrl+c":
		return m.close()
	}
	if m.auth != nil && m.auth.kind == tgclient.AuthMethod {
		switch key {
		case "enter", "1", "p":
			m.submitAuthValue("phone")
		case "2", "q":
			m.submitAuthValue("qr")
		}
		return nil
	}
	switch key {
	case "enter":
		m.submitAuth()
		return nil
	}
	var cmd tea.Cmd
	m.authField, cmd = m.authField.Update(msg)
	return cmd
}

func (m *model) submitAuth() {
	m.submitAuthValue(m.authField.Value())
}

func (m *model) submitAuthValue(value string) {
	if m.auth == nil || m.bridge == nil {
		return
	}
	value = strings.TrimSpace(value)
	if value == "" {
		m.notice = "value is required"
		return
	}
	m.bridge.SubmitAuth(value)
	m.auth = nil
	m.authField.Reset()
	m.authField.Blur()
	m.notice = "waiting for Telegram…"
}

func (m *model) authPanelRect() rect {
	w := clamp(m.w-8, 30, 56)
	if w > m.w-2 {
		w = m.w - 2
	}
	h := 8
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

func (m *model) authInputRect() rect {
	p := m.authPanelRect()
	return rect{p.x + 2, p.y + 4, p.w - 4, 1}
}

func (m *model) authSubmitRect() rect {
	p := m.authPanelRect()
	const bw = 12
	return rect{p.x + (p.w-bw)/2, p.y + 6, bw, 1}
}

func (m *model) authMethodRects() (phone, qr rect) {
	p := m.authPanelRect()
	const total = 22
	start := p.x + (p.w-total)/2
	y := p.y + 6
	return rect{start, y, 10, 1}, rect{start + 14, y, 8, 1}
}

func (m *model) authClick(e tea.Mouse, _ layout) tea.Cmd {
	if m.auth != nil && m.auth.kind == tgclient.AuthMethod {
		phone, qr := m.authMethodRects()
		if phone.contains(e.X, e.Y) {
			m.submitAuthValue("phone")
		} else if qr.contains(e.X, e.Y) {
			m.submitAuthValue("qr")
		}
		return nil
	}
	if m.authSubmitRect().contains(e.X, e.Y) {
		m.submitAuth()
		return nil
	}
	if m.authInputRect().contains(e.X, e.Y) {
		m.authField.Focus()
	}
	return nil
}

func (m *model) renderAuth() ([]string, int, int) {
	t := m.theme
	bg := t.PanelAlt
	p := m.authPanelRect()
	contentW := p.w - 2
	lines := []string{paint("╭"+strings.Repeat("─", contentW)+"╮", t.Primary, bg)}

	if m.auth.kind == tgclient.AuthMethod {
		lines = append(lines,
			paint("│", t.Primary, bg)+t.pad(" Sign in to Telegram", contentW, t.Text, bg, true)+paint("│", t.Primary, bg),
			paint("│", t.Primary, bg)+t.pad(" Phone code or QR from another device", contentW, t.Muted, bg, false)+paint("│", t.Primary, bg),
			paint("│", t.Primary, bg)+t.fill(contentW, bg)+paint("│", t.Primary, bg),
			paint("│", t.Primary, bg)+t.pad(" Phone: code arrives in Telegram", contentW, t.Text, bg, false)+paint("│", t.Primary, bg),
			paint("│", t.Primary, bg)+t.pad(" QR: Settings → Devices → Link Desktop", contentW, t.Text, bg, false)+paint("│", t.Primary, bg),
		)
		phoneR, qrR := m.authMethodRects()
		start := phoneR.x - (p.x + 1)
		phoneBG := t.Primary
		qrBG := bg
		if m.hover && phoneR.contains(m.hoverX, m.hoverY) {
			phoneBG = t.ChatActive
		}
		if m.hover && qrR.contains(m.hoverX, m.hoverY) {
			qrBG = t.Selected
		}
		buttons := t.fill(start, bg)
		buttons += t.pad(" Phone ", 10, t.OnPrimary, phoneBG, true)
		buttons += t.fill(4, bg)
		buttons += t.pad(" QR ", 8, t.Primary, qrBG, false)
		buttons += t.fill(contentW-start-22, bg)
		lines = append(lines, paint("│", t.Primary, bg)+buttons+paint("│", t.Primary, bg))
		lines = append(lines, paint("╰"+strings.Repeat("─", contentW)+"╯", t.Primary, bg))
		return lines, p.x, p.y
	}

	title := " Sign in to Telegram"
	switch m.auth.kind {
	case tgclient.AuthPhone:
		title = " Phone number"
	case tgclient.AuthCode:
		title = " Login code"
	case tgclient.AuthPassword:
		title = " Two-factor password"
	}
	lines = append(lines,
		paint("│", t.Primary, bg)+t.pad(title, contentW, t.Text, bg, true)+paint("│", t.Primary, bg),
		paint("│", t.Primary, bg)+t.pad(" "+m.auth.hint, contentW, t.Muted, bg, false)+paint("│", t.Primary, bg),
		paint("│", t.Primary, bg)+t.fill(contentW, bg)+paint("│", t.Primary, bg),
	)

	inputW := maxInt(1, contentW-2)
	m.authField.SetWidth(maxInt(1, inputW-1))
	field := strings.SplitN(m.authField.View(), "\n", 2)[0]
	row := paint("│", t.Primary, bg)
	row += paint(" ", t.Text, bg)
	row += t.padANSI(field, inputW, bg)
	row += paint(" ", t.Text, bg)
	row += paint("│", t.Primary, bg)
	lines = append(lines, row)
	lines = append(lines, paint("│", t.Primary, bg)+t.fill(contentW, bg)+paint("│", t.Primary, bg))

	buttonFG := t.OnPrimary
	buttonBG := t.Primary
	if m.hover && m.authSubmitRect().contains(m.hoverX, m.hoverY) {
		buttonBG = t.ChatActive
	}
	button := t.pad(" Continue ", 12, buttonFG, buttonBG, true)
	pad := (contentW - 12) / 2
	lines = append(lines, paint("│", t.Primary, bg)+t.fill(pad, bg)+button+t.fill(contentW-pad-12, bg)+paint("│", t.Primary, bg))
	lines = append(lines, paint("╰"+strings.Repeat("─", contentW)+"╯", t.Primary, bg))
	return lines, p.x, p.y
}
func messageDayLabel(t time.Time) string {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())
	switch {
	case day.Equal(today):
		return "Today"
	case day.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	default:
		if t.Year() == now.Year() {
			return t.Format("2 Jan")
		}
		return t.Format("2 Jan 2006")
	}
}

func (m *model) requestMedia(id int, kind string) bool {
	chat := m.activeChat()
	if m.bridge == nil || chat == nil || chat.PeerKey == "" || id == 0 {
		return false
	}
	key := mediaKey(chat.PeerKey, id)
	if m.pendingMedia[key] == kind {
		return true
	}
	m.pendingMedia[key] = kind
	m.bridge.DownloadMedia(chat.PeerKey, id)
	m.notice = "downloading " + kind + "…"
	return true
}

type qrState struct {
	url   string
	lines []string
}

const qrQuiet = 4

func qrLines(url string) ([]string, bool) {
	code, err := qr.Encode(url, qr.L)
	if err != nil {
		return nil, false
	}
	size := code.Size + qrQuiet*2
	dark := func(x, y int) bool {
		mx, my := x-qrQuiet, y-qrQuiet
		return mx >= 0 && my >= 0 && mx < code.Size && my < code.Size && code.Black(mx, my)
	}
	lines := make([]string, 0, (size+1)/2)
	for y := 0; y < size; y += 2 {
		var b strings.Builder
		b.Grow(size)
		for x := 0; x < size; x++ {
			switch top, bottom := dark(x, y), dark(x, y+1); {
			case top && bottom:
				b.WriteString("█")
			case top:
				b.WriteString("▀")
			case bottom:
				b.WriteString("▄")
			default:
				b.WriteString(" ")
			}
		}
		lines = append(lines, b.String())
	}
	return lines, true
}

func (m *model) showQR(url string) {
	lines, ok := qrLines(url)
	if !ok {
		m.notice = "QR code could not be generated"
		return
	}
	width := ansi.StringWidth(lines[0])
	if width+2 > m.w-2 || len(lines)+4 > m.h-2 {
		m.qr = nil
		m.notice = "QR code does not fit; enlarge the terminal or choose phone"
		return
	}
	m.auth = nil
	m.tgStatus = "scan QR"
	m.qr = &qrState{url: url, lines: lines}
}

func (m *model) renderQR() ([]string, int, int) {
	t := m.theme
	bg := lipgloss.Color("#000000")
	qrFG := lipgloss.Color("#000000")
	qrBG := lipgloss.Color("#FFFFFF")
	border := t.Primary
	contentW := ansi.StringWidth(m.qr.lines[0])
	panelW := contentW + 2
	panelH := len(m.qr.lines) + 3
	x := (m.w - panelW) / 2
	y := (m.h - panelH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	lines := []string{paint("╭"+strings.Repeat("─", contentW)+"╮", border, bg)}
	header := t.pad(" Scan with Telegram · Settings → Devices", contentW, qrFG, qrBG, true)
	lines = append(lines, paint("│", border, bg)+header+paint("│", border, bg))
	for _, line := range m.qr.lines {
		lines = append(lines, paint("│", border, bg)+paint(line, qrFG, qrBG)+paint("│", border, bg))
	}
	lines = append(lines, paint("╰"+strings.Repeat("─", contentW)+"╯", border, bg))
	return lines, x, y
}
