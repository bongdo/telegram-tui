package ui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tg-matrix-tui/internal/chafa"
	"tg-matrix-tui/internal/domain"
	"tg-matrix-tui/internal/tgclient"
)

func TestMain(m *testing.M) {
	os.Setenv("TG_TUI_DEMO", "1")
	os.Exit(m.Run())
}

func resize(m *model, w, h int) {
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
}

func TestLayoutFillsWidth(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	g := m.layout()

	if got := g.left.w + 1 + g.center.w; got != 120 {
		t.Fatalf("wide columns = %d, want 120", got)
	}
	if g.center.w < 40 {
		t.Fatalf("center too narrow: %d", g.center.w)
	}

	m.profileOpen = true
	m.syncInput()
	g = m.layout()
	if got := g.left.w + 1 + g.center.w + 1 + g.right.w; got != 120 {
		t.Fatalf("profile columns = %d, want 120", got)
	}
}

func TestViewLinesHaveTerminalWidth(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
	}{
		{"wide", 120, 36},
		{"wide-min", 84, 24},
		{"profile", 132, 40},
		{"profile-min", 100, 30},
		{"compact", 70, 24},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel()
			resize(m, tc.w, tc.h)
			if strings.HasPrefix(tc.name, "profile") {
				m.profileOpen = true
				m.syncInput()
			}
			if tc.name == "compact" {
				m.compactPane = paneConversation
			}
			lines := strings.Split(m.View().Content, "\n")
			if len(lines) != tc.h {
				t.Fatalf("lines = %d, want %d", len(lines), tc.h)
			}
			for i, line := range lines {
				if got := ansi.StringWidth(line); got != tc.w {
					t.Fatalf("line %d width = %d, want %d: %q", i, got, tc.w, ansi.Strip(line))
				}
			}
		})
	}
}

func TestMotionFilterDropsRepeatedCell(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.hover, m.hoverX, m.hoverY = true, 7, 9

	if got := Filter(m, tea.MouseMotionMsg{X: 7, Y: 9}); got != nil {
		t.Fatalf("duplicate motion kept: %v", got)
	}
	if got := Filter(m, tea.MouseMotionMsg{X: 8, Y: 9}); got == nil {
		t.Fatal("motion to a new cell dropped")
	}
	if got := Filter(m, tea.KeyPressMsg{}); got == nil {
		t.Fatal("non-motion message dropped")
	}

	fresh := newModel()
	resize(fresh, 120, 36)
	if got := Filter(fresh, tea.MouseMotionMsg{X: 0, Y: 0}); got == nil {
		t.Fatal("first motion at 0,0 dropped before hover was set")
	}
}

func TestReplyComposerFillsWidth(t *testing.T) {
	m := newModel()
	resize(m, 100, 30)
	m.profileOpen = true
	m.reply = &Reply{Sender: "Mira", Text: "keep the web layout"}
	m.syncInput()

	lines := strings.Split(m.View().Content, "\n")
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != 100 {
			t.Fatalf("line %d width = %d, want 100: %q", i, got, ansi.Strip(line))
		}
	}
}

func TestClickOpensChat(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.setFocus(focusChats)

	m.handleClick(tea.Mouse{X: 2, Y: leftHeaderRows, Button: tea.MouseLeft})
	ids := m.filteredChats()
	if m.active != ids[0] {
		t.Fatalf("active = %d, want %d", m.active, ids[0])
	}
}

func TestMouseComposerSends(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	g := m.layout()
	ir := m.inputRect(g)
	m.handleClick(tea.Mouse{X: ir.x + 1, Y: ir.y, Button: tea.MouseLeft})
	if m.focus != focusComposer {
		t.Fatalf("focus = %v, want composer", m.focus)
	}

	m.handleKey(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m.handleKey(tea.KeyPressMsg{Code: 'i', Text: "i"})
	if got := m.ta.Value(); got != "hi" {
		t.Fatalf("value = %q, want %q", got, "hi")
	}

	before := len(m.activeChat().Messages)
	m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := len(m.activeChat().Messages); got != before+1 {
		t.Fatalf("messages = %d, want %d", got, before+1)
	}
	if m.ta.Value() != "" {
		t.Fatalf("composer not cleared: %q", m.ta.Value())
	}
}

func TestMouseQuitButton(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	g := m.layout()
	qr := m.quitRect(g)
	cmd := m.handleClick(tea.Mouse{X: qr.x + 1, Y: qr.y, Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatal("quit click returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("quit click returned %T, want tea.QuitMsg", cmd())
	}
}

func composerCell(t *testing.T, m *model, x, y int) tea.Mouse {
	t.Helper()
	return tea.Mouse{X: x, Y: y, Button: tea.MouseLeft}
}

func TestComposerDoubleClickSelectsWord(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.setFocus(focusComposer)
	m.ta.SetValue("hello world")
	m.syncInput()
	g := m.layout()
	ir := m.inputRect(g)
	m.handleClick(composerCell(t, m, ir.x, ir.y))
	m.Update(tea.MouseReleaseMsg{})
	m.handleClick(composerCell(t, m, ir.x, ir.y))
	m.Update(tea.MouseReleaseMsg{})

	if !m.ta.HasSelection() {
		t.Fatal("double click did not start a selection")
	}
	if got := m.ta.SelectedText(); got != "hello" {
		t.Fatalf("selected = %q, want %q", got, "hello")
	}
}

func TestComposerDragSelects(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.setFocus(focusComposer)
	m.ta.SetValue("hello world")
	m.syncInput()
	g := m.layout()
	ir := m.inputRect(g)
	m.handleClick(composerCell(t, m, ir.x, ir.y))
	m.Update(tea.MouseMotionMsg{X: ir.x + 2, Y: ir.y, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{})

	if !m.ta.HasSelection() {
		t.Fatal("drag did not start a selection")
	}
	if got := m.ta.SelectedText(); got != "he" {
		t.Fatalf("selected = %q, want %q", got, "he")
	}
}

func TestComposerClickOutsideClearsSelection(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.setFocus(focusComposer)
	m.ta.SetValue("hello world")
	g := m.layout()
	ir := m.inputRect(g)
	m.handleClick(composerCell(t, m, ir.x, ir.y))
	m.Update(tea.MouseReleaseMsg{})
	m.handleClick(composerCell(t, m, ir.x, ir.y))
	m.Update(tea.MouseReleaseMsg{})
	if !m.ta.HasSelection() {
		t.Fatal("composer click did not start a selection")
	}
	m.handleClick(composerCell(t, m, m.layout().center.x+10, 5))
	if m.ta.HasSelection() {
		t.Fatal("click outside the composer did not clear the selection")
	}
}

func TestComposerCopySelectionKey(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.setFocus(focusComposer)
	m.ta.SetValue("hello")
	g := m.layout()
	ir := m.inputRect(g)
	m.handleClick(composerCell(t, m, ir.x, ir.y))
	m.Update(tea.MouseReleaseMsg{})
	m.handleClick(composerCell(t, m, ir.x, ir.y))
	m.Update(tea.MouseReleaseMsg{})
	if !m.ta.HasSelection() {
		t.Fatal("copy test setup: no selection")
	}
	if cmd := m.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl | tea.ModShift}); cmd == nil {
		t.Fatal("copy selection returned no command")
	}
}

func TestWheelScrollsMessages(t *testing.T) {
	m := newModel()
	resize(m, 120, 24)
	g := m.layout()
	m.msgScroll = 0
	cmd := m.handleWheel(tea.Mouse{X: g.center.x + 10, Y: g.center.y + 6, Button: tea.MouseWheelUp})
	if cmd == nil {
		t.Fatal("wheel up scheduled no tick")
	}
	if _, ok := cmd().(wheelTickMsg); !ok {
		t.Fatal("wheel up scheduled the wrong message")
	}
	m.applyWheel()
	if m.msgScroll == 0 {
		t.Fatal("wheel up did not scroll the conversation")
	}
	m.handleWheel(tea.Mouse{X: g.center.x + 10, Y: g.center.y + 6, Button: tea.MouseWheelDown})
	m.applyWheel()
	if m.msgScroll != 0 {
		t.Fatalf("wheel down left scroll at %d", m.msgScroll)
	}
}

func TestWheelCoalesces(t *testing.T) {
	m := newModel()
	resize(m, 120, 24)
	g := m.layout()
	m.msgScroll = 0
	cmds := 0
	for i := 0; i < 20; i++ {
		if cmd := m.handleWheel(tea.Mouse{X: g.center.x + 10, Y: g.center.y + 6, Button: tea.MouseWheelUp}); cmd != nil {
			cmds++
		}
	}
	if cmds != 1 {
		t.Fatalf("scheduled %d ticks, want 1", cmds)
	}
	mv := m.messageView(g)
	want := minInt(60, maxInt(0, len(mv.canvas.lines)-mv.h))
	m.applyWheel()
	if m.msgScroll != want {
		t.Fatalf("coalesced scroll = %d, want %d", m.msgScroll, want)
	}
}

func assertView(t *testing.T, m *model, w, h int) {
	t.Helper()
	lines := strings.Split(m.View().Content, "\n")
	if len(lines) != h {
		t.Fatalf("lines = %d, want %d", len(lines), h)
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != w {
			t.Fatalf("line %d width = %d, want %d: %q", i, got, w, ansi.Strip(line))
		}
	}
}

func visibleMessageHit(t *testing.T, m *model) (tea.Mouse, int) {
	t.Helper()
	g := m.layout()
	mv := m.messageView(g)
	for _, hit := range mv.canvas.hits {
		if hit.bubble.y >= mv.start && hit.bubble.y < mv.start+mv.h {
			x := g.center.x + hit.bubble.x + 1
			y := g.center.y + centerHeaderHeight + mv.topPad + (hit.bubble.y - mv.start)
			return tea.Mouse{X: x, Y: y, Button: tea.MouseRight}, hit.msg
		}
	}
	t.Fatal("no visible message")
	return tea.Mouse{}, 0
}

func TestRightClickMessageMenu(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	click, msg := visibleMessageHit(t, m)
	m.handleRightClick(click)
	if m.ctxKind != ctxMessage || m.ctxMsg != msg {
		t.Fatalf("ctx = %v msg=%d, want message/%d", m.ctxKind, m.ctxMsg, msg)
	}
	assertView(t, m, 120, 36)
}

func TestReactionFromContextMenu(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	click, msg := visibleMessageHit(t, m)
	m.handleRightClick(click)
	r := m.ctxReactionRects()[0]
	m.handleClick(tea.Mouse{X: r.x + 1, Y: r.y, Button: tea.MouseLeft})
	got := m.activeChat().Messages[msg]
	found := false
	for _, reaction := range got.Reactions {
		if reaction.Emoji == quickReactions[0] && reaction.Mine && reaction.Count > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("reaction %q was not applied: %+v", quickReactions[0], got.Reactions)
	}
}

func TestEmojiPanelInserts(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	g := m.layout()
	r := m.emojiRect(g)
	m.handleClick(tea.Mouse{X: r.x, Y: r.y, Button: tea.MouseLeft})
	if !m.emojiOpen {
		t.Fatal("emoji panel did not open")
	}
	assertView(t, m, 120, 36)
	cell := m.emojiCellRects(g)[0]
	m.handleClick(tea.Mouse{X: cell.x + 1, Y: cell.y, Button: tea.MouseLeft})
	if !strings.Contains(m.ta.Value(), emojiGrid[0]) {
		t.Fatalf("composer = %q, want %q", m.ta.Value(), emojiGrid[0])
	}
}

func TestChatContextMarksRead(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	// Alice is the second visible row and starts with unread messages.
	m.handleRightClick(tea.Mouse{X: 2, Y: leftHeaderRows + 2, Button: tea.MouseRight})
	if m.ctxKind != ctxChat {
		t.Fatalf("ctx = %v, want chat", m.ctxKind)
	}
	items := m.ctxItemRects()
	m.handleClick(tea.Mouse{X: items[1].x + 1, Y: items[1].y, Button: tea.MouseLeft})
	if got := m.chats[m.ctxChat].Unread; got != 0 {
		t.Fatalf("unread = %d, want 0", got)
	}
}

func TestPhotoViewerWithChafa(t *testing.T) {
	if !chafa.Available() {
		t.Skip("chafa is not installed")
	}
	path := filepath.Join(t.TempDir(), "demo.png")
	img := image.NewRGBA(image.Rect(0, 0, 24, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 24; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 10), B: 180, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	t.Setenv("TG_TUI_DEMO_IMAGE", path)

	m := newModel()
	resize(m, 120, 36)
	m.notice = ""
	cmd := m.openPhoto(Message{Photo: &Photo{W: 24, H: 24}})
	if cmd == nil {
		t.Fatalf("openPhoto returned nil: %s", m.notice)
	}
	m.Update(cmd())
	if m.viewer.loading {
		t.Fatal("viewer is still loading")
	}
	if m.viewer.err != "" {
		t.Fatalf("viewer error: %s", m.viewer.err)
	}
	if len(m.viewer.lines) == 0 {
		t.Fatal("viewer has no image lines")
	}
	assertView(t, m, 120, 36)
}

func TestPhotoWithoutDemoImageShowsNotice(t *testing.T) {
	t.Setenv("TG_TUI_DEMO_IMAGE", "")
	m := newModel()
	resize(m, 120, 36)
	cmd := m.openPhoto(Message{Photo: &Photo{W: 10, H: 10}})
	if cmd != nil {
		t.Fatal("openPhoto returned a command without an image path")
	}
	if !strings.Contains(m.notice, "TG_TUI_DEMO_IMAGE") {
		t.Fatalf("notice = %q", m.notice)
	}
}

func TestCompactRightClickMessage(t *testing.T) {
	m := newModel()
	resize(m, 70, 24)
	m.compactPane = paneConversation
	click, _ := visibleMessageHit(t, m)
	m.handleRightClick(click)
	if m.ctxKind != ctxMessage {
		t.Fatalf("ctx = %v, want message", m.ctxKind)
	}
	assertView(t, m, 70, 24)
}

func TestKeyboardMenuAndReaction(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.setFocus(focusMessages)
	m.selectedMsg = m.activeChat().Messages[len(m.activeChat().Messages)-1].ID
	m.handleKey(tea.KeyPressMsg{Code: 'm', Text: "m"})
	if m.ctxKind != ctxMessage {
		t.Fatalf("ctx = %v, want message", m.ctxKind)
	}
	idx := m.ctxMsg
	m.handleKey(tea.KeyPressMsg{Code: '1', Text: "1"})
	found := false
	for _, r := range m.activeChat().Messages[idx].Reactions {
		if r.Emoji == quickReactions[0] && r.Mine {
			found = true
		}
	}
	if !found {
		t.Fatal("keyboard reaction was not applied")
	}
}

func TestKeyboardEmojiPanel(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.handleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if !m.emojiOpen {
		t.Fatal("emoji panel did not open")
	}
	assertView(t, m, 120, 36)
}

func TestAuthOverlayWidth(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.auth = &authState{kind: tgclient.AuthCode, hint: "Code sent in the Telegram app (5 digits)"}
	m.authField.Focus()
	m.authField.SetValue("12345")
	assertView(t, m, 120, 36)
}

func TestTelegramEventsReplaceMock(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.handleTGEvent(tgclient.DialogsEvent{Chats: []domain.Chat{
		{PeerKey: "user:1", Name: "Alice", Folder: domain.FolderPersonal},
	}})
	if len(m.chats) != 1 || m.chats[0].Name != "Alice" || m.active != 0 {
		t.Fatalf("dialogs not applied: %+v", m.chats)
	}
	m.handleTGEvent(tgclient.HistoryEvent{
		PeerKey:  "user:1",
		Messages: []domain.Message{{ID: 1, Text: "hi"}},
	})
	if !m.chats[0].HistoryLoaded || len(m.chats[0].Messages) != 1 {
		t.Fatalf("history not applied: %+v", m.chats[0])
	}
	m.handleTGEvent(tgclient.NewMessageEvent{
		PeerKey: "user:1",
		Chat:    domain.Chat{PeerKey: "user:1", Name: "Alice"},
		Message: domain.Message{ID: 2, Text: "new"},
	})
	if len(m.chats[0].Messages) != 2 {
		t.Fatalf("message not appended: %+v", m.chats[0].Messages)
	}
}

func TestAuthPanelLineWidths(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.auth = &authState{kind: tgclient.AuthPhone, hint: "International format"}
	m.authField.Focus()
	lines, _, _ := m.renderAuth()
	p := m.authPanelRect()
	if len(lines) != p.h {
		t.Fatalf("auth lines = %d, want %d", len(lines), p.h)
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != p.w {
			t.Fatalf("auth line %d width = %d, want %d: %q", i, got, p.w, ansi.Strip(line))
		}
	}
}

func TestOverlayPanelWidths(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	click, _ := visibleMessageHit(t, m)
	m.handleRightClick(click)
	lines, _, _ := m.renderCtxMenu()
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != ctxWidth {
			t.Fatalf("ctx line %d width = %d, want %d", i, got, ctxWidth)
		}
	}
	m.ctxKind = ctxNone
	m.emojiOpen = true
	lines, _, _ = m.renderEmojiPanel(m.layout())
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != emojiPanelW {
			t.Fatalf("emoji line %d width = %d, want %d: %q", i, got, emojiPanelW, ansi.Strip(line))
		}
	}
}

func TestAttachPanelWidth(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.attach = &attachState{}
	m.attachField.Focus()
	m.attachField.SetValue("/tmp/file.pdf")
	lines, _, _ := m.renderAttach()
	p := m.attachPanelRect()
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != p.w {
			t.Fatalf("attach line %d width = %d, want %d: %q", i, got, p.w, ansi.Strip(line))
		}
	}
	assertView(t, m, 120, 36)
}

func TestRecentEmojiOrder(t *testing.T) {
	m := newModel()
	m.noteEmoji("🔥")
	m.noteEmoji("👍")
	list := m.emojiList()
	if list[0] != "👍" || list[1] != "🔥" {
		t.Fatalf("recent list = %v", list[:3])
	}
}

func TestAuthMethodPanel(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.auth = &authState{kind: tgclient.AuthMethod, hint: "Choose"}
	lines, _, _ := m.renderAuth()
	p := m.authPanelRect()
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != p.w {
			t.Fatalf("line %d width = %d, want %d", i, got, p.w)
		}
	}
}

func TestQRPanel(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.showQR("tg://login?token=0123456789abcdef")
	if m.qr == nil {
		t.Fatalf("no QR: %s", m.notice)
	}
	assertView(t, m, 120, 36)
}

func TestFolderFilteringAndCounts(t *testing.T) {
	m := newModel()
	m.chats = []Chat{
		{PeerKey: "user:1", Name: "Archived", Archived: true, Unread: 4, Folders: []int{folderArchive}},
		{PeerKey: "user:2", Name: "Work", Unread: 2, Folders: []int{folderPersonal}},
		{PeerKey: "user:3", Name: "Both", Unread: 1, Folders: []int{folderPersonal, folderWork}},
	}
	m.folders = []Folder{
		{ID: folderAll, Title: "All"},
		{ID: folderArchive, Title: "Archive"},
		{ID: folderPersonal, Title: "Personal"},
		{ID: folderWork, Title: "Work"},
	}
	m.active = -1
	m.folder = folderAll
	if ids := m.filteredChats(); len(ids) != 2 {
		t.Fatalf("all chats = %d, want 2 (archive excluded)", len(ids))
	}
	if got := m.folderUnread(folderAll); got != 3 {
		t.Fatalf("all unread = %d, want 3", got)
	}
	m.setFolder(folderArchive)
	if ids := m.filteredChats(); len(ids) != 1 || ids[0] != 0 {
		t.Fatalf("archive chats = %v, want [0]", ids)
	}
	m.setFolder(folderPersonal)
	if ids := m.filteredChats(); len(ids) != 2 {
		t.Fatalf("personal chats = %v, want 2", ids)
	}
	if got := m.folderUnread(folderPersonal); got != 3 {
		t.Fatalf("personal unread = %d, want 3", got)
	}
	m.setFolder(folderWork)
	if ids := m.filteredChats(); len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("work chats = %v, want [2]", ids)
	}
}

func TestEditOwnMessage(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.active = 3
	m.selectedMsg = 3
	m.beginEdit()
	if m.editingID != 3 {
		t.Fatalf("editingID = %d, want 3", m.editingID)
	}
	if m.ta.Value() == "" {
		t.Fatal("composer was not pre-filled")
	}
	m.ta.SetValue("edited in the TUI")
	m.send()
	msg := m.chats[3].Messages[2]
	if msg.Text != "edited in the TUI" || !msg.Edited {
		t.Fatalf("message after edit = %+v", msg)
	}
	if m.editingID != 0 || m.ta.Value() != "" {
		t.Fatalf("edit mode not cleared: id=%d text=%q", m.editingID, m.ta.Value())
	}
}

func TestForwardToChat(t *testing.T) {
	m := newModel()
	resize(m, 120, 36)
	m.active = 3
	m.selectedMsg = 5
	m.beginForward()
	if m.fwd == nil {
		t.Fatal("forward picker did not open")
	}
	assertView(t, m, 120, 36)
	m.forwardTo(1)
	if m.fwd != nil {
		t.Fatal("forward picker did not close")
	}
	msgs := m.chats[1].Messages
	last := msgs[len(msgs)-1]
	if last.ForwardFrom != "Charm TUI" || !last.Out {
		t.Fatalf("forwarded message = %+v", last)
	}
	if last.Text != "The new chat layout is in main. Hit-testing works from the first click." {
		t.Fatalf("forwarded text = %q", last.Text)
	}
}

func BenchmarkMessageView(b *testing.B) {
	m := newModel()
	m.w, m.h = 120, 40
	m.active = 3
	for i := 0; i < b.N; i++ {
		m.msgVersion++
		g := m.layout()
		m.messageView(g)
	}
}

func BenchmarkMessageViewCached(b *testing.B) {
	m := newModel()
	m.w, m.h = 120, 40
	m.active = 3
	g := m.layout()
	m.messageView(g)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.messageView(g)
	}
}

func BenchmarkView(b *testing.B) {
	m := newModel()
	m.w, m.h = 120, 40
	m.active = 3
	m.View()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.View()
	}
}

// BenchmarkViewHoverSweep models a pointer sweeping over the chat list, the
// worst case for full-frame rendering: hover lands on a new row each frame.
func BenchmarkViewHoverSweep(b *testing.B) {
	m := newModel()
	m.w, m.h = 120, 40
	m.active = 3
	m.View()
	m.hover = true
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.hoverX = 5 + i%20
		m.hoverY = 4 + (i/20)%20
		m.View()
	}
}
