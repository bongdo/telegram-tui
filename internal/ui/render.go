package ui

import (
	"fmt"
	"image/color"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

func trunc(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// paint styles plain text. ansi.Style yields the same SGR as a lipgloss style,
// without lipgloss's per-call property lookups, which dominated View cost.
func paint(s string, fg, bg color.Color, bold ...bool) string {
	if s == "" {
		return ""
	}
	if strings.ContainsRune(s, '\t') {
		s = strings.ReplaceAll(s, "\t", "    ") // lipgloss default tab width
	}
	st := ansi.Style{}
	if len(bold) > 0 && bold[0] {
		st = st.Bold()
	}
	return st.ForegroundColor(fg).BackgroundColor(bg).Styled(s)
}

func (t Theme) pad(s string, w int, fg, bg color.Color, bold bool) string {
	s = trunc(s, w)
	n := w - ansi.StringWidth(s)
	if n > 0 {
		s += spaces(n)
	}
	return paint(s, fg, bg, bold)
}

func (t Theme) padRight(s string, w int, fg, bg color.Color, bold bool) string {
	s = trunc(s, w)
	n := w - ansi.StringWidth(s)
	if n > 0 {
		s = spaces(n) + s
	}
	return paint(s, fg, bg, bold)
}

func (t Theme) padANSI(s string, w int, bg color.Color) string {
	s = ansi.Truncate(s, w, "…")
	if n := w - ansi.StringWidth(s); n > 0 {
		s += t.fill(n, bg)
	}
	return s
}

func (t Theme) fill(w int, bg color.Color) string {
	if w <= 0 {
		return ""
	}
	return ansi.Style{}.BackgroundColor(bg).Styled(spaces(w))
}

func initials(name string) []rune {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return []rune("?")
	}
	first := []rune(fields[0])
	if len(fields) == 1 {
		if len(first) > 2 {
			first = first[:2]
		}
		return []rune(strings.ToUpper(string(first)))
	}
	second := []rune(fields[1])
	return []rune{unicode.ToUpper(first[0]), unicode.ToUpper(second[0])}
}

func (t Theme) avatar(name string, colorIndex int) (string, string) {
	c := lipgloss.Color(peerColors[colorIndex%len(peerColors)])
	white := lipgloss.Color("#FFFFFF")
	in := initials(name)
	top, bottom := "   ", "   "
	switch len(in) {
	case 0:
	case 1:
		top = " " + string(in[0]) + " "
	default:
		top = " " + string(in[0]) + " "
		bottom = " " + string(in[1]) + " "
	}
	return paint(top, white, c, true), paint(bottom, white, c, true)
}

type folderTab struct {
	label  string
	folder int
	x, w   int
}

func (m *model) folderSpecs() []folderTab {
	if m.bridge == nil && len(m.folders) == 0 {
		out := []folderTab{
			{label: "All", folder: folderAll},
			{label: "Chats", folder: folderPersonal},
			{label: "Work", folder: folderWork},
			{label: "Channels", folder: folderChannels},
		}
		for i := range out {
			if count := m.folderUnread(out[i].folder); count > 0 {
				out[i].label += " " + fmt.Sprintf("%d", count)
			}
		}
		return out
	}
	out := make([]folderTab, 0, len(m.folders))
	for _, f := range m.folders {
		label := f.Title
		if label == "" {
			label = fmt.Sprintf("Folder %d", f.ID)
		}
		if f.Emoji != "" {
			label = f.Emoji + " " + label
		}
		if count := m.folderUnread(f.ID); count > 0 {
			label += " " + fmt.Sprintf("%d", count)
		}
		out = append(out, folderTab{label: label, folder: f.ID})
	}
	return out
}

func (m *model) folderTabs(width int) []folderTab {
	specs := m.folderSpecs()
	if len(specs) == 0 {
		return nil
	}
	start := 0
	total := 0
	for _, tab := range specs {
		total += ansi.StringWidth(tab.label) + 1
	}
	if total > width-1 {
		for i, tab := range specs {
			if tab.folder == m.folder {
				start = i
				break
			}
		}
	}
	out := make([]folderTab, 0, len(specs))
	cursor := 1
	for i := start; i < len(specs); i++ {
		tab := specs[i]
		if cursor >= width {
			break
		}
		label := trunc(tab.label, maxInt(1, width-cursor))
		tab.label = label
		tab.x = cursor
		tab.w = ansi.StringWidth(label)
		out = append(out, tab)
		cursor += tab.w + 1
	}
	return out
}

func (m *model) folderUnread(folder int) int {
	total := 0
	for i := range m.chats {
		chat := &m.chats[i]
		switch folder {
		case folderAll:
			if chat.Archived {
				continue
			}
		case folderArchive:
			if !chat.Archived {
				continue
			}
		default:
			if !chatInFolder(chat, folder) {
				continue
			}
		}
		total += chat.Unread
	}
	return total
}

func chatInFolder(chat *Chat, folder int) bool {
	if folder == folderAll {
		return true
	}
	if len(chat.Folders) > 0 {
		for _, id := range chat.Folders {
			if id == folder {
				return true
			}
		}
		return false
	}
	return chat.Folder == folder
}

func (m *model) headerButtons(g layout) (search, call, info rect) {
	x := g.center.x + g.center.w - 9
	y := g.center.y
	return rect{x, y, 2, 2}, rect{x + 2, y, 2, 2}, rect{x + 4, y, 2, 2}
}

func (m *model) leftHeaderButtons(g layout) (theme, compose rect) {
	w := g.left.w
	return rect{w - 5, 0, 1, 1}, rect{w - 3, 0, 1, 1}
}

func (m *model) searchClearRect(g layout) rect {
	return rect{g.left.x + g.left.w - 2, 1, 1, 1}
}

func (m *model) quitRect(g layout) rect {
	return rect{m.w - 6, g.status.y, 6, 1}
}

func (m *model) renderStatusBar(g layout) string {
	t := m.theme
	bg := t.PanelAlt
	leftW := minInt(20, maxInt(1, m.w/3))
	rightW := minInt(24, maxInt(1, m.w/3))
	const quitW = 6
	if leftW+rightW+quitW > m.w {
		rightW = maxInt(0, m.w-leftW-quitW)
	}
	label, labelColor := m.statusView()
	left := t.pad(" "+label, leftW, labelColor, bg, true)

	right := ""
	rightColor := t.Muted
	if m.notice != "" {
		right = oneLine(m.notice)
		rightColor = t.Error
	} else if m.player != nil {
		right = "mpv ready"
	} else {
		right = "mpv missing"
		rightColor = t.Error
	}
	rightCell := t.padRight(right, rightW, rightColor, bg, false)

	quitFG := t.Muted
	if m.hover && m.quitRect(g).contains(m.hoverX, m.hoverY) {
		quitFG = t.Primary
	}
	quit := t.pad(" quit ", quitW, quitFG, bg, true)

	hintText := "Mouse: click · wheel · Tab focus · / search · i write · Esc back"
	hintW := m.w - leftW - rightW - quitW
	if hintW < 0 {
		hintW = 0
	}
	hint := t.pad("  "+hintText, hintW, t.Muted, bg, false)
	return left + hint + rightCell + quit
}

func (m *model) renderLeft(g layout) []string {
	t := m.theme
	bg := t.Panel
	w, h := g.left.w, g.contentH
	if w <= 0 || h <= 0 {
		return nil
	}
	lines := make([]string, 0, h)

	// Header: menu/title + theme + compose.
	themeIcon := "☾"
	if !t.Dark {
		themeIcon = "☀"
	}
	themeFg := t.Muted
	if m.hover && m.hoverY == 0 && m.hoverX >= w-5 && m.hoverX < w-4 {
		themeFg = t.Primary
	}
	composeFg := t.Muted
	if m.hover && m.hoverY == 0 && m.hoverX >= w-3 && m.hoverX < w-2 {
		composeFg = t.Primary
	}
	row0 := t.pad(" ☰  Telegram", w-5, t.Text, bg, true)
	row0 += paint(themeIcon, themeFg, bg, false)
	row0 += paint(" ", t.Text, bg, false)
	row0 += paint("✎", composeFg, bg, false)
	row0 += paint("  ", t.Text, bg, false)
	lines = append(lines, row0)

	// Search field.
	boxBG := t.PanelAlt
	iconFG := t.Muted
	if m.focus == focusSearch {
		iconFG = t.Primary
	}
	query := m.search.Value()
	var field string
	if m.focus == focusSearch {
		field = m.search.View()
		field = strings.SplitN(field, "\n", 2)[0]
	} else if query == "" {
		field = paint("Search", t.Muted, boxBG, false)
	} else {
		field = paint(trunc(query, w-8), t.Text, boxBG, false)
	}
	row1 := paint(" ", t.Text, bg, false)
	row1 += paint("⌕ ", iconFG, boxBG, false)
	row1 += t.padANSI(field, w-6, boxBG)
	if query != "" {
		clearFG := t.Muted
		if m.hover && m.searchClearRect(g).contains(m.hoverX, m.hoverY) {
			clearFG = t.Primary
		}
		row1 += paint(" ✕", clearFG, boxBG, false)
	} else {
		row1 += paint("  ", t.Text, boxBG, false)
	}
	row1 += paint(" ", t.Text, bg, false)
	lines = append(lines, row1)

	// Folder tabs.
	row2 := paint(" ", t.Text, bg, false)
	cursor := 1
	for _, tab := range m.folderTabs(w) {
		if tab.x < cursor {
			continue
		}
		if tab.x > cursor {
			row2 += t.fill(tab.x-cursor, bg)
		}
		switch {
		case m.folder == tab.folder:
			row2 += paint(tab.label, t.OnPrimary, t.Primary, true)
		case m.hover && m.hoverY == 2 && m.hoverX >= g.left.x+tab.x && m.hoverX < g.left.x+tab.x+tab.w:
			row2 += paint(tab.label, t.Primary, bg, false)
		default:
			row2 += paint(tab.label, t.Muted, bg, false)
		}
		cursor = tab.x + tab.w
	}
	if cursor < w {
		row2 += t.fill(w-cursor, bg)
	}
	lines = append(lines, row2)

	// Divider.
	lines = append(lines, paint(strings.Repeat("─", w), t.Border, bg, false))

	ids := m.filteredChats()
	visible := (h - leftHeaderRows) / 2
	if visible < 0 {
		visible = 0
	}
	if m.chatScroll > maxInt(0, len(ids)-visible) {
		m.chatScroll = maxInt(0, len(ids)-visible)
	}
	if m.chatScroll < 0 {
		m.chatScroll = 0
	}

	if len(ids) == 0 {
		lines = append(lines, t.pad("  No chats found", w, t.Muted, bg, false))
		for len(lines) < h {
			lines = append(lines, t.fill(w, bg))
		}
		return lines[:h]
	}

	for vis := 0; vis < visible; vis++ {
		pos := m.chatScroll + vis
		if pos >= len(ids) {
			break
		}
		chat := &m.chats[ids[pos]]
		rowY := leftHeaderRows + vis*2
		isActive := ids[pos] == m.active
		isHover := m.hover && m.hoverX < w && m.hoverY >= rowY && m.hoverY < rowY+2
		rowBG := bg
		fg := t.Text
		muted := t.Muted
		switch {
		case isActive:
			rowBG = t.ChatActive
			fg = t.OnPrimary
			muted = t.OnPrimary
		case isHover:
			rowBG = t.Hover
		}

		avatarTop, avatarBottom := t.avatar(chat.Name, chat.Color)
		name := chat.Name
		if chat.Verified {
			name += " ✓"
		}
		if chat.Pinned {
			name += " ⚑"
		}
		preview := chat.Preview
		if chat.Muted {
			preview = "⊘ " + preview
		}
		if ids[pos] == m.active && m.reply != nil {
			preview = "↩ " + preview
		}

		textW := w - 11
		if textW < 4 {
			textW = 4
		}
		line1 := paint(" ", fg, rowBG, false) + avatarTop + paint(" ", fg, rowBG, false)
		line1 += t.pad(name, textW, fg, rowBG, chat.Unread > 0 || isActive)
		line1 += t.padRight(chat.Time, 5, muted, rowBG, false)
		line1 += paint(" ", fg, rowBG, false)

		line2 := paint(" ", fg, rowBG, false) + avatarBottom + paint(" ", fg, rowBG, false)
		line2 += t.pad(preview, textW, muted, rowBG, false)
		line2 += m.badge(chat, rowBG, isActive)
		line2 += paint(" ", fg, rowBG, false)

		lines = append(lines, line1, line2)
	}

	for len(lines) < h {
		lines = append(lines, t.fill(w, bg))
	}
	return lines[:h]
}

func (m *model) badge(chat *Chat, rowBG color.Color, isActive bool) string {
	t := m.theme
	if chat.Unread <= 0 {
		return t.pad("", 5, t.Muted, rowBG, false)
	}
	label := fmt.Sprintf("%d", chat.Unread)
	if chat.Unread > 99 {
		label = "99+"
	}
	badgeBG := t.Primary
	badgeFG := t.OnPrimary
	if isActive {
		badgeBG = t.OnPrimary
		badgeFG = t.ChatActive
	}
	inner := t.padRight(label, 3, badgeFG, badgeBG, true)
	// 1 space, then a 3-cell pill, then 1 space = 5 cells.
	return paint(" ", t.Text, rowBG, false) + inner + paint(" ", t.Text, rowBG, false)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m *model) statusView() (string, color.Color) {
	t := m.theme
	switch {
	case m.auth != nil:
		return "● Sign in", t.Primary
	case m.tgErr != "":
		return "● Error", t.Error
	case m.bridge == nil:
		return "● Demo data", t.Muted
	case m.tgStatus == "connected":
		if m.selfName != "" {
			return "● " + m.selfName, t.Online
		}
		return "● Connected", t.Online
	case m.tgStatus == "connecting":
		return "● Connecting", t.Primary
	case m.tgStatus != "":
		return "● " + m.tgStatus, t.Muted
	default:
		return "● Telegram", t.Primary
	}
}
