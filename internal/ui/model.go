package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bongdo/telegram-tui/internal/audio"
	"github.com/bongdo/telegram-tui/internal/domain"
	"github.com/bongdo/telegram-tui/internal/tgclient"
)

type pane int

const (
	paneChats pane = iota
	paneConversation
)

type focusArea int

const (
	focusChats focusArea = iota
	focusSearch
	focusComposer
	focusMessages
)

type audioTickMsg struct{}

type model struct {
	w, h int

	chats    []Chat
	folders  []domain.Folder
	active   int
	selected int
	folder   int

	msgVersion   uint64
	canvas       messageCanvasCache
	editingID    int
	fwd          *forwardState
	wheelChats   int
	wheelMsgs    int
	wheelTicking bool

	theme Theme

	search         textinput.Model
	ta             textarea.Model
	focus          focusArea
	compactPane    pane
	profileOpen    bool
	profileTab     int
	reply          *Reply
	chatScroll     int
	msgScroll      int
	selectedMsg    int
	hoverX, hoverY int
	hover          bool
	notice         string

	ctxKind ctxKind
	ctxX    int
	ctxY    int
	ctxMsg  int
	ctxChat int
	ctxSel  int

	emojiOpen bool
	emojiTab  int
	viewer    viewerState

	lastClick    time.Time
	lastClickMsg int

	bridge         *tgclient.Bridge
	tgStatus       string
	tgErr          string
	selfName       string
	auth           *authState
	authField      textinput.Model
	historyLoading map[string]bool
	pendingMedia   map[string]string
	recentEmoji    []string
	attach         *attachState
	attachField    textinput.Model
	qr             *qrState
	replyToID      int

	player       *audio.Player
	audioErr     string
	playingID    int
	playProgress float64
	playPos      time.Duration
	audioPlays   bool
	audioPaused  bool
	audioTicking bool
}

// New returns the root Bubble Tea model.
func New() tea.Model {
	return newModel()
}

func newModel() *model {
	m := &model{
		w:              120,
		h:              36,
		chats:          mockChats(),
		active:         3,
		selected:       3,
		folder:         folderAll,
		theme:          darkTheme(),
		compactPane:    paneConversation,
		historyLoading: make(map[string]bool),
		pendingMedia:   make(map[string]string),
	}

	m.search = textinput.New()
	m.search.Prompt = ""
	m.search.Placeholder = "Search"
	m.search.CharLimit = 64
	m.search.Blur()

	m.ta = textarea.New()
	m.ta.Prompt = ""
	m.ta.Placeholder = "Message"
	m.ta.ShowLineNumbers = false
	m.ta.CharLimit = 4096
	m.ta.SetWidth(40)
	m.ta.SetHeight(1)
	m.ta.KeyMap.InsertNewline.SetKeys("shift+enter", "ctrl+j")
	m.ta.Blur()

	m.authField = textinput.New()
	m.authField.Prompt = ""
	m.authField.CharLimit = 128
	m.authField.Blur()

	m.attachField = textinput.New()
	m.attachField.Prompt = ""
	m.attachField.Placeholder = "path/to/file"
	m.attachField.CharLimit = 1024
	m.attachField.Blur()

	cfg, enabled, cfgErr := tgclient.ConfigFromEnv()
	switch {
	case cfgErr != nil:
		m.tgStatus = "config error"
		m.tgErr = cfgErr.Error()
		m.notice = cfgErr.Error()
	case enabled:
		m.startBridge(cfg)
	default:
		m.tgStatus = "demo"
	}

	if p, err := audio.New(); err != nil {
		m.audioErr = err.Error()
	} else {
		m.player = p
	}

	m.applyInputStyles()
	m.syncInput()
	return m
}

func (m *model) Init() tea.Cmd {
	if m.bridge != nil {
		m.bridge.Start()
		return waitBridge(m.bridge)
	}
	return nil
}

// startBridge creates the Telegram client. Network work begins in Init or, for
// the login screen path, right after the credentials are saved.
func (m *model) startBridge(cfg tgclient.Config) {
	m.bridge = tgclient.New(cfg)
	m.tgStatus = "connecting"
	m.tgErr = ""
	m.chats = nil
	m.folders = []domain.Folder{
		{ID: domain.FolderAll, Title: "All"},
		{ID: domain.FolderArchive, Title: "Archive"},
	}
	m.active = 0
	m.selected = 0
	m.chatScroll = 0
	m.msgScroll = 0
	m.recentEmoji = m.bridge.RecentEmoji(16)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.syncInput()
		return m, nil
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	case tea.MouseClickMsg:
		e := msg.Mouse()
		if e.Button == tea.MouseRight {
			m.handleRightClick(e)
			return m, nil
		}
		return m, m.handleClick(e)
	case tea.MouseMotionMsg:
		e := msg.Mouse()
		m.hoverX, m.hoverY, m.hover = e.X, e.Y, true
		return m, nil
	case tea.MouseWheelMsg:
		return m, m.handleWheel(msg.Mouse())
	case wheelTickMsg:
		m.applyWheel()
		return m, nil
	case tgclient.Event:
		cmd := m.handleTGEvent(msg)
		if m.bridge != nil {
			return m, tea.Batch(cmd, waitBridge(m.bridge))
		}
		return m, cmd
	case imageReadyMsg:
		if m.viewer.open && m.viewer.path == msg.path {
			m.viewer.loading = false
			m.viewer.lines = msg.lines
			if msg.err != nil {
				m.viewer.err = msg.err.Error()
			} else {
				m.viewer.err = ""
			}
		}
		return m, nil
	case audioTickMsg:
		m.audioTicking = false
		if m.player != nil {
			st := m.player.State()
			m.audioPlays = st.Playing
			m.audioPaused = st.Paused
			m.playProgress = st.Progress
			m.playPos = st.Position
			if st.Playing {
				return m, m.tickAudio()
			}
			m.playingID = 0
		}
		return m, nil
	}
	return m, nil
}

func (m *model) View() tea.View {
	g := m.layout()
	if m.w <= 0 || m.h <= 0 {
		return tea.NewView("")
	}

	body := make([]string, 0, g.contentH)
	if g.compact {
		if m.compactPane == paneConversation && m.active >= 0 {
			body = m.renderCenter(g)
		} else {
			body = m.renderLeft(g)
		}
	} else {
		left := m.renderLeft(g)
		center := m.renderCenter(g)
		if g.profile {
			right := m.renderRight(g)
			body = joinColumns("│", left, center, right)
		} else {
			body = joinColumns("│", left, center)
		}
	}

	lines := append(body, m.renderStatusBar(g))
	base := strings.Join(lines, "\n")
	view := tea.NewView(m.composeOverlays(base))
	view.AltScreen = true
	view.MouseMode = tea.MouseModeAllMotion
	view.WindowTitle = "Telegram TUI"
	return view
}

func joinColumns(sep string, columns ...[]string) []string {
	h := 0
	for _, c := range columns {
		if len(c) > h {
			h = len(c)
		}
	}
	out := make([]string, h)
	for i := 0; i < h; i++ {
		parts := make([]string, len(columns))
		for j, c := range columns {
			if i < len(c) {
				parts[j] = c[i]
			}
		}
		out[i] = strings.Join(parts, sep)
	}
	return out
}

func (m *model) filteredChats() []int {
	q := strings.ToLower(strings.TrimSpace(m.search.Value()))
	ids := make([]int, 0, len(m.chats))
	for i := range m.chats {
		c := &m.chats[i]
		if q == "" {
			switch {
			case m.folder == folderAll:
				if c.Archived {
					continue
				}
			case m.folder == folderArchive:
				if !c.Archived {
					continue
				}
			default:
				if !chatInFolder(c, m.folder) {
					continue
				}
			}
		}
		if q != "" {
			hay := strings.ToLower(c.Name + " " + c.Preview + " " + c.Username)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		ids = append(ids, i)
	}
	return ids
}

func (m *model) openFiltered(pos int) {
	ids := m.filteredChats()
	if pos < 0 || pos >= len(ids) {
		return
	}
	m.selected = pos
	m.active = ids[pos]
	m.chats[ids[pos]].Unread = 0
	m.msgScroll = 0
	m.selectedMsg = 0
	m.reply = nil
	m.replyToID = 0
	m.profileOpen = false
	m.emojiOpen = false
	m.compactPane = paneConversation
	m.syncInput()
	m.ensureHistory()
}

func (m *model) activeChat() *Chat {
	if m.active < 0 || m.active >= len(m.chats) {
		return nil
	}
	return &m.chats[m.active]
}

func (m *model) setFocus(f focusArea) {
	m.focus = f
	if f == focusSearch {
		m.search.Focus()
	} else {
		m.search.Blur()
	}
	if f == focusComposer {
		m.ta.Focus()
	} else {
		m.ta.Blur()
	}
}

func (m *model) cycleFocus() {
	order := []focusArea{focusChats, focusSearch, focusComposer, focusMessages}
	cur := 0
	for i, f := range order {
		if f == m.focus {
			cur = i
		}
	}
	m.setFocus(order[(cur+1)%len(order)])
}

func (m *model) syncInput() {
	w := m.centerWidth() - 11
	if w < 8 {
		w = 8
	}
	m.ta.SetWidth(w)
	m.ta.SetHeight(m.inputLines())

	lw := m.leftWidth()
	if m.w < compactWidth {
		lw = m.w
	}
	if lw < 14 {
		lw = 14
	}
	m.search.SetWidth(lw - 6)
}

func (m *model) applyInputStyles() {
	dark := m.theme.Dark
	ts := textarea.DefaultStyles(dark)
	ts.Focused.Base = lipgloss.NewStyle().Background(m.theme.Panel)
	ts.Blurred.Base = lipgloss.NewStyle().Background(m.theme.Panel)
	ts.Focused.Text = lipgloss.NewStyle().Foreground(m.theme.Text).Background(m.theme.Panel)
	ts.Blurred.Text = lipgloss.NewStyle().Foreground(m.theme.Text).Background(m.theme.Panel)
	ts.Focused.Placeholder = lipgloss.NewStyle().Foreground(m.theme.Muted).Background(m.theme.Panel)
	ts.Blurred.Placeholder = lipgloss.NewStyle().Foreground(m.theme.Muted).Background(m.theme.Panel)
	ts.Cursor.Color = m.theme.Primary
	m.ta.SetStyles(ts)

	is := textinput.DefaultStyles(dark)
	is.Focused.Text = lipgloss.NewStyle().Foreground(m.theme.Text).Background(m.theme.PanelAlt)
	is.Blurred.Text = lipgloss.NewStyle().Foreground(m.theme.Text).Background(m.theme.PanelAlt)
	is.Focused.Placeholder = lipgloss.NewStyle().Foreground(m.theme.Muted).Background(m.theme.PanelAlt)
	is.Blurred.Placeholder = lipgloss.NewStyle().Foreground(m.theme.Muted).Background(m.theme.PanelAlt)
	is.Cursor.Color = m.theme.Primary
	m.search.SetStyles(is)
	m.authField.SetStyles(is)
	m.attachField.SetStyles(is)
	if m.fwd != nil {
		m.fwd.query.SetStyles(is)
	}
}

func (m *model) toggleTheme() {
	m.theme = m.theme.toggle()
	m.applyInputStyles()
}

func (m *model) send() {
	text := strings.TrimSpace(m.ta.Value())
	if text == "" {
		return
	}
	c := m.activeChat()
	if c == nil {
		return
	}

	if m.editingID != 0 {
		if m.bridge != nil && c.PeerKey != "" {
			m.bridge.EditText(c.PeerKey, m.editingID, text)
			m.notice = "saving edit…"
			return
		}
		for i := range c.Messages {
			if c.Messages[i].ID == m.editingID {
				c.Messages[i].Text = text
				c.Messages[i].Edited = true
				break
			}
		}
		m.msgVersion++
		m.finishEdit()
		return
	}

	if m.bridge != nil && c.PeerKey != "" {
		m.bridge.SendText(c.PeerKey, text, m.replyToID)
		m.clearComposer()
		return
	}

	id := 0
	for _, msg := range c.Messages {
		if msg.ID > id {
			id = msg.ID
		}
	}
	now := time.Now().Format("15:04")
	c.Messages = append(c.Messages, Message{
		ID:        id + 1,
		Text:      text,
		Time:      now,
		Out:       true,
		Reply:     m.reply,
		ReplyToID: m.replyToID,
		Checks:    "✓",
	})
	c.Preview = "You: " + oneLine(text)
	c.Time = now
	m.msgVersion++
	m.clearComposer()
}

func (m *model) clearComposer() {
	m.ta.Reset()
	m.ta.SetHeight(1)
	m.editingID = 0
	m.reply = nil
	m.replyToID = 0
	m.msgScroll = 0
	m.selectedMsg = 0
	m.emojiOpen = false
	m.syncInput()
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) > 48 {
		return string([]rune(s)[:47]) + "…"
	}
	return s
}

func (m *model) close() tea.Cmd {
	if m.bridge != nil {
		m.bridge.Close()
	}
	if m.player != nil {
		m.player.Close()
	}
	return tea.Quit
}

func (m *model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.Keystroke()
	if m.auth != nil {
		return m.handleAuthKey(msg)
	}
	if m.attach != nil {
		return m.handleAttachKey(msg)
	}
	if m.fwd != nil {
		return m.handleForwardKey(msg)
	}

	if key == "ctrl+c" {
		return m.close()
	}
	if m.qr != nil {
		return nil
	}
	if key == "ctrl+t" {
		m.toggleTheme()
		return nil
	}
	m.notice = ""

	if key == "esc" {
		switch {
		case m.viewer.open:
			m.viewer = viewerState{}
			return nil
		case m.ctxKind != ctxNone:
			m.ctxKind = ctxNone
			return nil
		case m.emojiOpen:
			m.emojiOpen = false
			return nil
		case m.editingID != 0:
			m.cancelEdit()
			return nil
		}
	}
	if m.viewer.open {
		return nil
	}
	if m.ctxKind != ctxNone {
		items := m.ctxItems()
		switch key {
		case "up", "k":
			m.ctxSel--
			if m.ctxSel < 0 {
				m.ctxSel = 0
			}
		case "down", "j":
			m.ctxSel++
			if m.ctxSel >= len(items) {
				m.ctxSel = len(items) - 1
			}
		case "enter":
			return m.ctxAction(m.ctxSel)
		case "1", "2", "3", "4", "5", "6":
			if m.ctxKind == ctxMessage {
				i := int(key[0] - '1')
				m.toggleReaction(m.ctxMsg, quickReactions[i])
				m.ctxKind = ctxNone
			}
		}
		return nil
	}

	if m.focus == focusSearch {
		switch key {
		case "esc":
			m.setFocus(focusChats)
			m.search.Reset()
			m.chatScroll = 0
			return nil
		case "enter":
			m.openFiltered(0)
			m.setFocus(focusMessages)
			return nil
		case "up", "down":
			m.moveChat(key == "up")
			return nil
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.selected = 0
		m.chatScroll = 0
		return cmd
	}

	if m.focus == focusComposer {
		switch key {
		case "esc":
			m.setFocus(focusMessages)
			return nil
		case "enter":
			m.send()
			return nil
		}
		var cmd tea.Cmd
		m.ta, cmd = m.ta.Update(msg)
		m.syncInput()
		return cmd
	}

	switch key {
	case "q":
		return m.close()
	case "/":
		m.setFocus(focusSearch)
	case "i":
		m.setFocus(focusComposer)
	case "a":
		m.openAttach()
	case "e":
		if !m.canEmoji() {
			m.notice = "terminal is too small for the emoji panel"
			break
		}
		m.emojiOpen = !m.emojiOpen
		m.ctxKind = ctxNone
	case "f":
		if m.focus == focusMessages {
			m.beginForward()
		}
	case "ctrl+e":
		if m.focus == focusMessages {
			m.beginEdit()
		}
	case "m":
		if m.focus == focusMessages {
			m.openMessageMenu()
		} else if m.focus == focusChats {
			m.openChatMenu()
		}
	case "o":
		if m.focus == focusMessages {
			return m.openSelectedPhoto()
		}
	case "tab":
		m.cycleFocus()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.setFolderIndex(int(key[0] - '1'))
	case "esc":
		switch {
		case m.profileOpen:
			m.profileOpen = false
			m.syncInput()
		case m.w < compactWidth && m.compactPane == paneConversation:
			m.compactPane = paneChats
		case m.reply != nil:
			m.reply = nil
			m.replyToID = 0
			m.syncInput()
		case m.focus != focusChats:
			m.setFocus(focusChats)
		}
	case "up":
		if m.focus == focusMessages {
			m.scrollMessages(-1)
		} else {
			m.moveChat(true)
		}
	case "down":
		if m.focus == focusMessages {
			m.scrollMessages(1)
		} else {
			m.moveChat(false)
		}
	case "k":
		if m.focus == focusMessages {
			m.scrollMessages(-1)
		} else {
			m.moveChat(true)
		}
	case "j":
		if m.focus == focusMessages {
			m.scrollMessages(1)
		} else {
			m.moveChat(false)
		}
	case "pgup":
		m.scrollMessages(-8)
	case "pgdown":
		m.scrollMessages(8)
	case "enter":
		if m.focus == focusMessages {
			m.setFocus(focusComposer)
		} else {
			m.openFiltered(m.selected)
			m.setFocus(focusMessages)
		}
	case "r":
		if m.focus == focusMessages && m.selectedMsg != 0 {
			m.replyToSelected()
		}
	}
	return nil
}

func (m *model) moveChat(up bool) {
	ids := m.filteredChats()
	if len(ids) == 0 {
		return
	}
	pos := m.selected
	if up {
		pos--
	} else {
		pos++
	}
	if pos < 0 {
		pos = 0
	}
	if pos >= len(ids) {
		pos = len(ids) - 1
	}
	m.openFiltered(pos)
	m.ensureSelectedVisible()
}

func (m *model) ensureSelectedVisible() {
	g := m.layout()
	visible := (g.contentH - leftHeaderRows) / 2
	if visible < 1 {
		return
	}
	if m.selected < m.chatScroll {
		m.chatScroll = m.selected
	}
	if m.selected >= m.chatScroll+visible {
		m.chatScroll = m.selected - visible + 1
	}
	if m.chatScroll < 0 {
		m.chatScroll = 0
	}
}

func (m *model) setFolder(f int) {
	if !m.validFolder(f) {
		return
	}
	m.folder = f
	m.selected = 0
	m.chatScroll = 0
	m.msgScroll = 0
	ids := m.filteredChats()
	if len(ids) > 0 {
		m.active = ids[0]
	} else {
		m.active = -1
	}
	m.ensureHistory()
}

func (m *model) validFolder(f int) bool {
	if m.bridge == nil && len(m.folders) == 0 {
		return f >= folderAll && f <= folderChannels
	}
	for _, folder := range m.folders {
		if folder.ID == f {
			return true
		}
	}
	return false
}

func (m *model) setFolderIndex(i int) {
	tabs := m.folderSpecs()
	if i < 0 || i >= len(tabs) {
		return
	}
	m.setFolder(tabs[i].folder)
}

func (m *model) cycleFolder(delta int) {
	tabs := m.folderSpecs()
	if len(tabs) == 0 || delta == 0 {
		return
	}
	cur := 0
	for i := range tabs {
		if tabs[i].folder == m.folder {
			cur = i
			break
		}
	}
	next := (cur + delta) % len(tabs)
	if next < 0 {
		next += len(tabs)
	}
	m.setFolder(tabs[next].folder)
}

func (m *model) scrollMessages(delta int) {
	m.msgScroll += delta
	if m.msgScroll < 0 {
		m.msgScroll = 0
	}
}

func (m *model) replyToSelected() {
	c := m.activeChat()
	if c == nil {
		return
	}
	if m.editingID != 0 {
		m.finishEdit()
	}
	for _, msg := range c.Messages {
		if msg.ID != m.selectedMsg {
			continue
		}
		text := msg.Text
		if msg.Voice != nil {
			text = fmt.Sprintf("Voice message · 0:%02d", msg.Voice.Duration)
		} else if msg.Photo != nil {
			text = "Photo"
		} else if msg.Round != nil {
			text = "Round video"
		}
		sender := msg.Sender
		if msg.Out {
			sender = "You"
		}
		if sender == "" {
			sender = c.Name
		}
		m.reply = &Reply{Sender: sender, Text: oneLine(text)}
		m.replyToID = msg.ID
		m.setFocus(focusComposer)
		m.syncInput()
		return
	}
}

func (m *model) selectedMessageIndex() int {
	chat := m.activeChat()
	if chat == nil {
		return -1
	}
	if m.selectedMsg != 0 {
		for i := range chat.Messages {
			if chat.Messages[i].ID == m.selectedMsg {
				return i
			}
		}
	}
	for i := len(chat.Messages) - 1; i >= 0; i-- {
		if !chat.Messages[i].Service {
			return i
		}
	}
	return -1
}

func (m *model) openMessageMenu() {
	idx := m.selectedMessageIndex()
	if idx < 0 {
		return
	}
	chat := m.activeChat()
	m.selectedMsg = chat.Messages[idx].ID
	m.openCtx(ctxMessage, m.w/3, m.h/3, idx, -1)
}

func (m *model) openChatMenu() {
	if m.active < 0 || m.active >= len(m.chats) {
		return
	}
	m.openCtx(ctxChat, m.w/3, m.h/3, -1, m.active)
}

func (m *model) openSelectedPhoto() tea.Cmd {
	idx := m.selectedMessageIndex()
	if idx < 0 {
		return nil
	}
	chat := m.activeChat()
	msg := chat.Messages[idx]
	m.selectedMsg = msg.ID
	if msg.Photo == nil {
		m.notice = "selected message has no photo"
		return nil
	}
	return m.openPhoto(msg)
}

func (m *model) playVoice(msg Message) tea.Cmd {
	if msg.Voice == nil {
		return nil
	}
	path := msg.Voice.File
	if path == "" {
		if m.requestMedia(msg.ID, "voice") {
			return nil
		}
		path = os.Getenv("TG_TUI_DEMO_AUDIO")
	}
	return m.startAudio(path, msg.Voice.Duration, msg.ID, "mock voice note: set TG_TUI_DEMO_AUDIO=/path/voice.ogg")
}

func (m *model) playRound(msg Message) tea.Cmd {
	if msg.Round == nil {
		return nil
	}
	path := msg.Round.File
	if path == "" {
		if m.requestMedia(msg.ID, "round") {
			return nil
		}
		path = os.Getenv("TG_TUI_DEMO_ROUND_VIDEO")
	}
	return m.startAudio(path, msg.Round.Duration, msg.ID, "mock round note: set TG_TUI_DEMO_ROUND_VIDEO=/path/round.mp4")
}
func (m *model) startAudio(path string, seconds, id int, hint string) tea.Cmd {
	if m.player == nil {
		m.notice = m.audioErr
		return nil
	}
	if path == "" {
		m.notice = hint
		return nil
	}
	if m.playingID == id && m.audioPlays {
		if err := m.player.TogglePause(); err != nil {
			m.notice = err.Error()
		}
		return nil
	}
	if err := m.player.Play(path, time.Duration(seconds)*time.Second); err != nil {
		m.notice = err.Error()
		return nil
	}
	m.playingID = id
	m.audioPlays = true
	m.audioPaused = false
	m.playProgress = 0
	return m.tickAudio()
}
func (m *model) tickAudio() tea.Cmd {
	if m.audioTicking {
		return nil
	}
	m.audioTicking = true
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg { return audioTickMsg{} })
}

func (m *model) toggleProfile() {
	if m.w < 100 {
		m.notice = "profile panel needs ≥100 columns"
		return
	}
	m.profileOpen = !m.profileOpen
	m.emojiOpen = false
	m.syncInput()
}
