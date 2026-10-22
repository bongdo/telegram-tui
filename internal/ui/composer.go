package ui

import "strings"

func (m *model) composerTop(g layout) int {
	return g.center.y + g.contentH - g.composerH
}

func (m *model) replyCloseRect(g layout) rect {
	return rect{g.center.x + g.center.w - 2, m.composerTop(g), 1, 1}
}

func (m *model) inputRect(g layout) rect {
	top := m.composerTop(g)
	if m.reply != nil || m.editingID != 0 {
		top++
	}
	return rect{
		x: g.center.x + 5,
		y: top + 1,
		w: maxInt(1, g.center.w-11),
		h: m.inputLines(),
	}
}

func (m *model) sendRect(g layout) rect {
	y := g.center.y + g.contentH - 2
	return rect{g.center.x + g.center.w - 3, y, 1, 1}
}

func (m *model) emojiRect(g layout) rect {
	y := g.center.y + g.contentH - 2
	return rect{g.center.x + g.center.w - 5, y, 1, 1}
}

func (m *model) attachRect(g layout) rect {
	y := g.center.y + g.contentH - 2
	return rect{g.center.x + 1, y, 3, 1}
}

func (m *model) renderComposer(g layout) []string {
	t := m.theme
	bg := t.Panel
	w := g.center.w
	if w < 8 {
		w = 8
	}
	lines := make([]string, 0, g.composerH)

	if m.reply != nil || m.editingID != 0 {
		text := "Editing message"
		if m.reply != nil {
			text = "Reply to " + m.reply.Sender + ": " + m.reply.Text
		}
		row := paint(" ", t.Text, bg, false)
		row += paint("▎", t.Primary, bg, true)
		row += paint(" ", t.Text, bg, false)
		row += t.pad(text, w-5, t.Muted, bg, false)
		closeFG := t.Muted
		if m.hover && m.replyCloseRect(g).contains(m.hoverX, m.hoverY) {
			closeFG = t.Primary
		}
		row += paint("✕", closeFG, bg, false)
		row += paint(" ", t.Text, bg, false)
		lines = append(lines, row)
	}

	borderFG := t.Border
	if m.focus == focusComposer {
		borderFG = t.Primary
	}
	innerW := w - 2
	lines = append(lines, paint("╭"+strings.Repeat("─", innerW)+"╮", borderFG, bg, false))

	inputW := maxInt(1, w-11)
	value := m.ta.Value()
	textLines := strings.Split(value, "\n")
	inputLines := m.inputLines()
	taView := strings.Split(m.ta.View(), "\n")

	for i := 0; i < inputLines; i++ {
		last := i == inputLines-1
		var text string
		if i < len(taView) {
			text = taView[i]
		} else if i < len(textLines) {
			text = textLines[i]
		}
		text = strings.TrimSuffix(text, "\r")
		text = t.padANSI(text, inputW, bg)

		row := paint("│", borderFG, bg, false)
		if last {
			attachFG := t.Muted
			if m.hover && m.attachRect(g).contains(m.hoverX, m.hoverY) {
				attachFG = t.Primary
			}
			row += paint(" + ", attachFG, bg, true)
			row += paint(" ", t.Text, bg, false)
			row += text
			emojiFG := t.Muted
			if m.hover && m.emojiRect(g).contains(m.hoverX, m.hoverY) {
				emojiFG = t.Primary
			}
			sendFG := t.Muted
			if strings.TrimSpace(value) != "" {
				sendFG = t.Primary
			}
			if m.hover && m.sendRect(g).contains(m.hoverX, m.hoverY) {
				sendFG = t.Primary
			}
			row += paint(" ", t.Text, bg, false)
			row += paint("☺", emojiFG, bg, false)
			row += paint(" ", t.Text, bg, false)
			row += paint("➤", sendFG, bg, false)
			row += paint(" ", t.Text, bg, false)
			row += paint("│", borderFG, bg, false)
		} else {
			row += paint("   ", t.Text, bg, false)
			row += paint(" ", t.Text, bg, false)
			row += text
			row += paint("     ", t.Text, bg, false)
			row += paint("│", borderFG, bg, false)
		}
		lines = append(lines, row)
	}

	lines = append(lines, paint("╰"+strings.Repeat("─", innerW)+"╯", borderFG, bg, false))
	return lines
}
