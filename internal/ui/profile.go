package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func padCenter(s string, w int) string {
	s = trunc(s, w)
	n := w - ansi.StringWidth(s)
	if n <= 0 {
		return s
	}
	left := n / 2
	return spaces(left) + s + spaces(n-left)
}

func (m *model) profileCloseRect(g layout) rect {
	return rect{g.right.x + g.right.w - 2, g.right.y, 2, 1}
}

func (m *model) profileActionRects(g layout) (message, mute, more rect) {
	w := g.right.w
	const total = 23
	start := g.right.x + (w-total)/2
	y := g.right.y + 9
	return rect{start, y, 9, 1}, rect{start + 10, y, 6, 1}, rect{start + 17, y, 6, 1}
}

func (m *model) profileTabRects(g layout) (info, media, files rect) {
	y := g.right.y + 11
	return rect{g.right.x + 2, y, 4, 1}, rect{g.right.x + 8, y, 5, 1}, rect{g.right.x + 15, y, 5, 1}
}

func (m *model) renderRight(g layout) []string {
	t := m.theme
	bg := t.Panel
	w, h := g.right.w, g.contentH
	if w <= 0 || h <= 0 {
		return nil
	}
	chat := m.activeChat()
	if chat == nil {
		return fillPanel(t, w, h)
	}
	lines := make([]string, 0, h)

	row0 := paint(" Info", t.Text, bg, true)
	row0 += t.fill(w-7, bg)
	closeFG := t.Muted
	if m.hover && m.profileCloseRect(g).contains(m.hoverX, m.hoverY) {
		closeFG = t.Primary
	}
	row0 += paint("✕ ", closeFG, bg, false)
	lines = append(lines, row0)
	lines = append(lines, paint(strings.Repeat("─", w), t.Border, bg, false))

	// Large avatar, 9×3.
	avColor := lipgloss.Color(peerColors[chat.Color%len(peerColors)])
	avX := (w - 9) / 2
	if avX < 0 {
		avX = 0
	}
	avText := string(initials(chat.Name))
	lines = append(lines, t.fill(w, bg))
	lines = append(lines, t.fill(avX, bg)+t.fill(9, avColor)+t.fill(w-avX-9, bg))
	lines = append(lines, t.fill(avX, bg)+paint(padCenter(avText, 9), lipgloss.Color("#FFFFFF"), avColor, true)+t.fill(w-avX-9, bg))
	lines = append(lines, t.fill(avX, bg)+t.fill(9, avColor)+t.fill(w-avX-9, bg))

	lines = append(lines, paint(padCenter(chat.Name, w), t.Text, bg, true))
	status := chat.Status
	if status == "" {
		status = "last seen recently"
	}
	statusFG := t.Muted
	if chat.Online {
		statusFG = t.Online
	}
	lines = append(lines, paint(padCenter(status, w), statusFG, bg, false))
	lines = append(lines, t.fill(w, bg))

	msgR, muteR, moreR := m.profileActionRects(g)
	muteLabel := " Mute "
	if chat.Muted {
		muteLabel = "Unmute"
	}
	actions := t.fill(msgR.x-g.right.x, bg)
	actions += m.actionButton(" Message ", msgR, bg)
	actions += t.fill(muteR.x-(msgR.x+msgR.w), bg)
	actions += m.actionButton(muteLabel, muteR, bg)
	actions += t.fill(moreR.x-(muteR.x+muteR.w), bg)
	actions += m.actionButton(" More ", moreR, bg)
	actions += t.fill(w-(moreR.x+moreR.w-g.right.x), bg)
	lines = append(lines, actions)
	lines = append(lines, t.fill(w, bg))

	infoR, mediaR, filesR := m.profileTabRects(g)
	var tabs string
	tabs += paint(" ", t.Text, bg, false)
	for _, tab := range []struct {
		label string
		r     rect
		idx   int
	}{
		{"Info", infoR, 0}, {"Media", mediaR, 1}, {"Files", filesR, 2},
	} {
		if tab.r.x > g.right.x+ansi.StringWidth(tabs) {
			tabs += t.fill(tab.r.x-(g.right.x+ansi.StringWidth(tabs)), bg)
		}
		fg := t.Muted
		bold := false
		if m.profileTab == tab.idx {
			fg = t.Primary
			bold = true
		} else if m.hover && tab.r.contains(m.hoverX, m.hoverY) {
			fg = t.Primary
		}
		tabs += paint(tab.label, fg, bg, bold)
	}
	tabs += t.fill(w-ansi.StringWidth(tabs), bg)
	lines = append(lines, tabs)
	lines = append(lines, paint(strings.Repeat("─", w), t.Border, bg, false))

	switch m.profileTab {
	case 0:
		lines = append(lines, m.profileInfo(chat, w, bg)...)
	case 1:
		lines = append(lines, t.pad("  Shared media", w, t.Text, bg, true))
		lines = append(lines, t.pad("  12 photos · 4 videos", w, t.Muted, bg, false))
		lines = append(lines, t.pad("  7 files · 3 links", w, t.Muted, bg, false))
	case 2:
		lines = append(lines, t.pad("  Files", w, t.Text, bg, true))
		lines = append(lines, t.pad("  No files in the mock yet", w, t.Muted, bg, false))
	}

	for len(lines) < h {
		lines = append(lines, t.fill(w, bg))
	}
	return lines[:h]
}

func (m *model) actionButton(label string, r rect, bg color.Color) string {
	t := m.theme
	buttonBG := bg
	if m.hover && r.contains(m.hoverX, m.hoverY) {
		buttonBG = t.PanelAlt
	}
	return t.pad(label, r.w, t.Primary, buttonBG, false)
}

func (m *model) profileInfo(chat *Chat, w int, bg color.Color) []string {
	t := m.theme
	var out []string
	row := func(label, value string) {
		out = append(out, t.pad("  "+label, 14, t.Muted, bg, false)+t.pad(value, maxInt(1, w-14), t.Text, bg, false))
	}
	if chat.Phone != "" {
		row("Phone", chat.Phone)
	}
	if chat.Username != "" {
		row("Username", chat.Username)
	}
	out = append(out, t.pad("  Bio", w, t.Muted, bg, false))
	if chat.About == "" {
		out = append(out, t.pad("  —", w, t.Muted, bg, false))
	} else {
		for _, line := range strings.Split(lipgloss.Wrap(chat.About, maxInt(8, w-4), ""), "\n") {
			out = append(out, t.pad("  "+line, w, t.Text, bg, false))
		}
	}
	out = append(out, t.fill(w, bg))
	notif := "Enabled"
	if chat.Muted {
		notif = "Muted"
	}
	out = append(out, t.pad("  Notifications", 18, t.Muted, bg, false)+t.pad(notif, maxInt(1, w-18), t.Text, bg, false))
	out = append(out, t.pad("  Folder", 14, t.Muted, bg, false)+t.pad(folderName(chat.Folder), maxInt(1, w-14), t.Text, bg, false))
	return out
}

func folderName(f int) string {
	switch f {
	case folderPersonal:
		return "Chats"
	case folderWork:
		return "Work"
	case folderChannels:
		return "Channels"
	default:
		return "All"
	}
}

func fillPanel(t Theme, w, h int) []string {
	out := make([]string, h)
	for i := range out {
		out[i] = t.fill(w, t.Panel)
	}
	return out
}
