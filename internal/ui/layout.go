package ui

import "strings"

const (
	statusHeight   = 1
	leftHeaderRows = 4 // header, search, folders, divider
	maxInputLines  = 4
)

type rect struct {
	x, y, w, h int
}

func (r rect) contains(x, y int) bool {
	return r.w > 0 && r.h > 0 && x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

func (r rect) empty() bool { return r.w <= 0 || r.h <= 0 }

type layout struct {
	w, h      int
	compact   bool
	contentH  int
	left      rect
	center    rect
	right     rect
	profile   bool
	composerH int
	status    rect
}

func (m *model) leftWidth() int {
	switch {
	case m.w >= 120:
		return 30
	case m.w >= 100:
		return 28
	default:
		return 26
	}
}

func (m *model) centerWidth() int {
	if m.w < compactWidth {
		return m.w
	}
	w := m.w - m.leftWidth() - 1
	if m.profileOpen && m.w >= 100 {
		right := 30
		if m.w >= 120 {
			right = 32
		}
		w -= right + 1
	}
	return w
}

func (m *model) inputLines() int {
	n := strings.Count(m.ta.Value(), "\n") + 1
	if n < 1 {
		n = 1
	}
	if n > maxInputLines {
		n = maxInputLines
	}
	return n
}

func (m *model) composerHeight() int {
	h := m.inputLines() + 2 // rounded border
	if m.reply != nil || m.editingID != 0 {
		h++
	}
	return h
}

const compactWidth = 84

func (m *model) layout() layout {
	g := layout{w: m.w, h: m.h}
	g.contentH = m.h - statusHeight
	if g.contentH < 1 {
		g.contentH = m.h
	}
	g.status = rect{0, g.contentH, m.w, statusHeight}
	g.profile = m.profileOpen && m.w >= 100 && m.w >= compactWidth

	if m.w < compactWidth {
		g.compact = true
		if m.compactPane == paneConversation && m.active >= 0 {
			g.center = rect{0, 0, m.w, g.contentH}
		} else {
			g.left = rect{0, 0, m.w, g.contentH}
		}
		g.composerH = m.composerHeight()
		return g
	}

	lw := m.leftWidth()
	g.left = rect{0, 0, lw, g.contentH}
	cx := lw + 1
	cw := m.w - lw - 1
	if g.profile {
		rw := 30
		if m.w >= 120 {
			rw = 32
		}
		cw -= rw + 1
		g.right = rect{m.w - rw, 0, rw, g.contentH}
	}
	g.center = rect{cx, 0, cw, g.contentH}
	g.composerH = m.composerHeight()
	return g
}
