package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const centerHeaderHeight = 2

type lineKind int

const (
	lineText lineKind = iota
	lineSender
	lineReply
	lineMeta
	lineVoice
	lineReaction
)

type bubbleLine struct {
	text      string
	meta      string
	kind      lineKind
	senderIdx int
}

type messageCanvasHit struct {
	msg    int
	bubble rect // coordinates relative to the message area canvas
	voice  *rect
	barX   int
	barY   int
	barW   int
}

type messageCanvas struct {
	lines []string
	hits  []messageCanvasHit
}

type messageView struct {
	canvas messageCanvas
	start  int
	topPad int
	h      int
}

func (m *model) messagesHeight(g layout) int {
	h := g.contentH - centerHeaderHeight - g.composerH
	if h < 1 {
		h = 1
	}
	return h
}

type canvasKey struct {
	peer       string
	width      int
	dark       bool
	version    uint64
	selected   int
	hover      int
	playing    int
	voiceFrame int
	plays      bool
	paused     bool
}

func (k canvasKey) withoutHover() canvasKey {
	k.hover = 0
	return k
}

type messageCanvasCache struct {
	key    canvasKey
	canvas messageCanvas
	valid  bool
}

func (m *model) messageView(g layout) messageView {
	h := m.messagesHeight(g)
	canvas := m.canvasFor(g)
	start, topPad, scroll := sliceCanvas(canvas, h, m.msgScroll)
	m.msgScroll = scroll
	return messageView{canvas: canvas, start: start, topPad: topPad, h: h}
}

func sliceCanvas(canvas messageCanvas, h, scroll int) (start, topPad, clamped int) {
	maxScroll := maxInt(0, len(canvas.lines)-h)
	clamped = clamp(scroll, 0, maxScroll)
	start = len(canvas.lines) - h - clamped
	if start < 0 {
		start = 0
	}
	topPad = maxInt(0, h-len(canvas.lines))
	return start, topPad, clamped
}

func (m *model) canvasFor(g layout) messageCanvas {
	chat := m.activeChat()
	if chat == nil {
		return messageCanvas{}
	}
	base := m.canvasKey(g, chat.PeerKey)
	hover := 0
	if m.canvas.valid && m.canvas.key.withoutHover() == base {
		hover = m.hoveredMessageID(g, m.canvas.canvas)
	}
	base.hover = hover
	if m.canvas.valid && m.canvas.key == base {
		return m.canvas.canvas
	}

	canvas := m.buildMessageCanvas(g, hover, m.hoverX-g.center.x, m.selectedMsg)
	if hover == 0 && m.hover {
		if id := m.hoveredMessageID(g, canvas); id != 0 {
			base.hover = id
			canvas = m.buildMessageCanvas(g, id, m.hoverX-g.center.x, m.selectedMsg)
		}
	}
	m.canvas = messageCanvasCache{key: base, canvas: canvas, valid: true}
	return canvas
}

func (m *model) canvasKey(g layout, peer string) canvasKey {
	key := canvasKey{
		peer:     peer,
		width:    g.center.w,
		dark:     m.theme.Dark,
		version:  m.msgVersion,
		selected: m.selectedMsg,
		playing:  m.playingID,
		plays:    m.audioPlays,
		paused:   m.audioPaused,
	}
	if key.playing != 0 {
		key.voiceFrame = int(m.playProgress * 12)
	}
	return key
}

func (m *model) hoveredMessageID(g layout, canvas messageCanvas) int {
	chat := m.activeChat()
	if !m.hover || chat == nil {
		return 0
	}
	h := m.messagesHeight(g)
	start, topPad, _ := sliceCanvas(canvas, h, m.msgScroll)
	line := m.hoverY - (g.center.y + centerHeaderHeight) - topPad + start
	x := m.hoverX - g.center.x
	for _, hit := range canvas.hits {
		if hit.bubble.contains(x, line) && hit.msg >= 0 && hit.msg < len(chat.Messages) {
			return chat.Messages[hit.msg].ID
		}
	}
	return 0
}

func (m *model) buildMessageCanvas(g layout, hoverID, hoverX, selectedID int) messageCanvas {
	chat := m.activeChat()
	if chat == nil {
		return messageCanvas{}
	}
	t := m.theme
	areaW := g.center.w
	if areaW < 4 {
		return messageCanvas{}
	}

	var out messageCanvas
	for mi := range chat.Messages {
		msg := &chat.Messages[mi]
		if msg.Service {
			label := " " + msg.Text + " "
			lw := ansi.StringWidth(label)
			x := (areaW - lw) / 2
			if x < 0 {
				x = 0
			}
			line := t.fill(x, t.ChatBG) + paint(trunc(label, areaW), t.DateText, t.DateBG, false)
			line += t.fill(areaW-x-ansi.StringWidth(trunc(label, areaW)), t.ChatBG)
			out.lines = append(out.lines, line)
			continue
		}

		maxBubble := minInt(56, areaW-8)
		if maxBubble < 20 {
			maxBubble = maxInt(12, areaW-2)
		}
		contentMax := maxBubble - 4

		var cls []bubbleLine
		if !msg.Out && msg.Sender != "" && chat.PeerKind == "chat" {
			cls = append(cls, bubbleLine{text: msg.Sender, kind: lineSender, senderIdx: msg.Color})
		}
		if msg.ForwardFrom != "" {
			cls = append(cls, bubbleLine{
				text: "↱ Forwarded from " + msg.ForwardFrom,
				kind: lineReply,
			})
		}
		if msg.Reply != nil {
			cls = append(cls, bubbleLine{
				text: "▎ " + msg.Reply.Sender + ": " + oneLine(msg.Reply.Text),
				kind: lineReply,
			})
		}
		if msg.Text != "" {
			for _, l := range strings.Split(lipgloss.Wrap(msg.Text, contentMax, ""), "\n") {
				cls = append(cls, bubbleLine{text: l, kind: lineText})
			}
		}

		voiceIdx := -1
		if msg.Voice != nil {
			voiceIdx = len(cls)
			dur := formatDuration(msg.Voice.Duration)
			cls = append(cls, bubbleLine{
				text: "▶ " + trunc(msg.Voice.Waveform, 12) + " " + dur + " 1×",
				kind: lineVoice,
			})
		}
		if msg.Photo != nil {
			cls = append(cls, bubbleLine{
				text: fmt.Sprintf("▦  Photo · %d×%d", msg.Photo.W, msg.Photo.H),
				kind: lineText,
			})
			if msg.Photo.Caption != "" {
				for _, l := range strings.Split(lipgloss.Wrap(msg.Photo.Caption, contentMax, ""), "\n") {
					cls = append(cls, bubbleLine{text: l, kind: lineText})
				}
			}
		}
		if msg.Round != nil {
			cls = append(cls, bubbleLine{
				text: "◉  Round video · " + formatDuration(msg.Round.Duration),
				kind: lineText,
			})
		}
		if len(msg.Reactions) > 0 {
			var parts []string
			for _, r := range msg.Reactions {
				parts = append(parts, fmt.Sprintf("%s %d", r.Emoji, r.Count))
			}
			cls = append(cls, bubbleLine{text: " " + strings.Join(parts, "   ") + " ", kind: lineReaction})
		}
		if len(cls) == 0 {
			cls = append(cls, bubbleLine{text: "", kind: lineText})
		}

		natural := 0
		for _, cl := range cls {
			if w := ansi.StringWidth(cl.text); w > natural {
				natural = w
			}
		}
		bw := clamp(natural+4, 16, maxBubble)
		cw := bw - 4

		meta := msg.Time
		if msg.Edited {
			meta = "edited " + meta
		}
		if msg.Out {
			checks := msg.Checks
			if checks == "" {
				checks = "✓✓"
			}
			meta += " " + checks
		}
		if msg.Views != "" {
			meta += " ▪ " + msg.Views
		}
		if meta != "" {
			last := &cls[len(cls)-1]
			if ansi.StringWidth(last.text)+2+ansi.StringWidth(meta) <= cw {
				last.meta = meta
			} else {
				cls = append(cls, bubbleLine{text: meta, kind: lineMeta})
			}
		}

		bubbleBG := t.InBG
		fg := t.InText
		metaFG := t.InMeta
		if msg.Out {
			bubbleBG = t.OwnBG
			fg = t.OwnText
			metaFG = t.OwnMeta
		}

		start := len(out.lines)
		bubbleH := len(cls) + 2
		x := 1
		if msg.Out {
			x = areaW - bw - 2
		}
		if x < 1 {
			x = 1
		}
		hovered := msg.ID == hoverID && hoverX >= x && hoverX < x+bw
		edge := bubbleBG
		if hovered || msg.ID == selectedID {
			edge = t.Primary
		}

		out.lines = append(out.lines, m.alignBubble(areaW, x, paint("╭"+strings.Repeat("─", bw-2)+"╮", edge, t.ChatBG, false)))
		for li, cl := range cls {
			content := m.renderBubbleContent(cl, cw, fg, metaFG, bubbleBG, msg, li == voiceIdx)
			line := paint("│", edge, t.ChatBG, false)
			line += paint(" ", fg, bubbleBG, false)
			line += content
			line += paint(" ", fg, bubbleBG, false)
			line += paint("│", edge, t.ChatBG, false)
			out.lines = append(out.lines, m.alignBubble(areaW, x, line))
		}
		out.lines = append(out.lines, m.alignBubble(areaW, x, paint("╰"+strings.Repeat("─", bw-2)+"╯", edge, t.ChatBG, false)))

		hit := messageCanvasHit{
			msg:    mi,
			bubble: rect{x, start, bw, bubbleH},
		}
		if voiceIdx >= 0 {
			playX := x + 2
			playY := start + 1 + voiceIdx
			hit.voice = &rect{playX, playY, 1, 1}
			hit.barX = playX + 2
			hit.barY = playY
			hit.barW = minInt(12, maxInt(1, cw-10))
		}
		out.hits = append(out.hits, hit)
	}
	return out
}

func (m *model) alignBubble(areaW, x int, bubble string) string {
	if x < 0 {
		x = 0
	}
	right := areaW - x - ansi.StringWidth(bubble)
	if right < 0 {
		right = 0
	}
	return m.theme.fill(x, m.theme.ChatBG) + bubble + m.theme.fill(right, m.theme.ChatBG)
}

func (m *model) renderBubbleContent(cl bubbleLine, cw int, fg, metaFG, bg color.Color, msg *Message, voice bool) string {
	t := m.theme
	if voice && msg.Voice != nil {
		return m.renderVoice(cl, cw, fg, metaFG, bg, msg)
	}

	lineFG := fg
	bold := false
	switch cl.kind {
	case lineSender:
		lineFG = lipgloss.Color(peerColors[cl.senderIdx%len(peerColors)])
		bold = true
	case lineReply:
		lineFG = metaFG
	case lineMeta:
		lineFG = metaFG
	case lineReaction:
		lineFG = metaFG
		bold = true
	}

	if cl.kind == lineMeta {
		return t.padRight(cl.text, cw, lineFG, bg, bold)
	}
	if cl.meta != "" {
		tw := ansi.StringWidth(cl.text)
		mw := ansi.StringWidth(cl.meta)
		gap := cw - tw - mw
		if gap < 1 {
			return t.pad(cl.text, cw, lineFG, bg, bold)
		}
		return paint(cl.text, lineFG, bg, bold) +
			paint(spaces(gap), lineFG, bg, bold) +
			paint(cl.meta, metaFG, bg, false)
	}
	return t.pad(cl.text, cw, lineFG, bg, bold)
}

func (m *model) renderVoice(cl bubbleLine, cw int, fg, metaFG, bg color.Color, msg *Message) string {
	t := m.theme
	waveW := minInt(12, maxInt(4, cw-10))
	wave := []rune(trunc(msg.Voice.Waveform, waveW))
	for len(wave) < waveW {
		wave = append(wave, '▁')
	}

	play := "▶"
	if m.playingID == msg.ID && m.audioPlays {
		if m.audioPaused {
			play = "▶"
		} else {
			play = "⏸"
		}
	}

	var b strings.Builder
	b.WriteString(paint(play, fg, bg, true))
	b.WriteString(paint(" ", fg, bg, false))
	played := 0
	if m.playingID == msg.ID {
		played = int(m.playProgress * float64(len(wave)))
	}
	for i, r := range wave {
		c := metaFG
		if i < played {
			c = fg
		}
		b.WriteString(paint(string(r), c, bg, false))
	}
	dur := formatDuration(msg.Voice.Duration)
	b.WriteString(paint(" "+dur+" 1×", metaFG, bg, false))
	line := b.String()
	lw := ansi.StringWidth(line)
	if cl.meta != "" {
		if gap := cw - lw - ansi.StringWidth(cl.meta); gap >= 1 {
			line += paint(spaces(gap), fg, bg, false) + paint(cl.meta, metaFG, bg, false)
			lw = cw
		}
	}
	if lw > cw {
		return t.padANSI(line, cw, bg)
	}
	return line + paint(spaces(cw-lw), fg, bg, false)
}

func formatDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func (m *model) renderCenter(g layout) []string {
	t := m.theme
	bg := t.Panel
	w, h := g.center.w, g.contentH
	if w <= 0 || h <= 0 {
		return nil
	}
	chat := m.activeChat()
	if chat == nil {
		label := "  Select a chat"
		if m.bridge != nil {
			label = "  Connecting to Telegram…"
		}
		lines := []string{t.pad(label, w, t.Muted, bg, false)}
		for len(lines) < h {
			lines = append(lines, t.fill(w, bg))
		}
		return lines[:h]
	}

	lines := make([]string, 0, h)
	avatarTop, avatarBottom := t.avatar(chat.Name, chat.Color)

	var prefixTop, prefixBottom string
	nameW := w - 14
	btnX := w - 9
	if m.w < compactWidth {
		backFG := t.Muted
		if m.hover && m.hoverY < 2 && m.hoverX >= 1 && m.hoverX < 3 {
			backFG = t.Primary
		}
		prefixTop = paint(" ‹ ", backFG, bg, true) + avatarTop + paint(" ", t.Text, bg, false)
		prefixBottom = paint("   ", t.Text, bg, false) + avatarBottom + paint(" ", t.Text, bg, false)
		nameW = w - 16
		btnX = w - 9
	} else {
		prefixTop = paint(" ", t.Text, bg, false) + avatarTop + paint(" ", t.Text, bg, false)
		prefixBottom = paint(" ", t.Text, bg, false) + avatarBottom + paint(" ", t.Text, bg, false)
	}
	if nameW < 4 {
		nameW = 4
	}

	searchFg, callFg, infoFg := t.Muted, t.Muted, t.Muted
	if m.hover && m.hoverY < 2 && m.hoverX >= btnX {
		switch {
		case m.hoverX < btnX+2:
			searchFg = t.Primary
		case m.hoverX < btnX+4:
			callFg = t.Primary
		case m.hoverX < btnX+6:
			infoFg = t.Primary
		}
	}
	buttons := paint("⌕ ", searchFg, bg, false) + paint("☎ ", callFg, bg, false) + paint("ⓘ ", infoFg, bg, false)

	row0 := prefixTop + t.pad(chat.Name, nameW, t.Text, bg, true) + buttons + t.fill(3, bg)
	status := chat.Status
	if status == "" {
		status = "last seen recently"
	}
	statusFG := t.Muted
	if chat.Online {
		statusFG = t.Online
	}
	row1 := prefixBottom + t.pad(status, nameW, statusFG, bg, false) + t.fill(9, bg)
	lines = append(lines, row0, row1)

	mv := m.messageView(g)
	if len(chat.Messages) == 0 && m.historyLoading[chat.PeerKey] {
		lines = append(lines, t.pad("  Loading messages…", w, t.Muted, t.ChatBG, false))
		for len(lines) < centerHeaderHeight+mv.h {
			lines = append(lines, t.fill(w, t.ChatBG))
		}
		lines = append(lines, m.renderComposer(g)...)
		for len(lines) < h {
			lines = append(lines, t.fill(w, bg))
		}
		return lines[:h]
	}
	for i := 0; i < mv.h; i++ {
		idx := mv.start - mv.topPad + i
		if idx >= 0 && idx < len(mv.canvas.lines) {
			lines = append(lines, mv.canvas.lines[idx])
		} else {
			lines = append(lines, t.fill(w, t.ChatBG))
		}
	}

	lines = append(lines, m.renderComposer(g)...)
	for len(lines) < h {
		lines = append(lines, t.fill(w, bg))
	}
	return lines[:h]
}

func (m *model) messageHit(g layout, x, y int) (msgIndex int, voice bool, seek float64, ok bool) {
	mv := m.messageView(g)
	xr := x - g.center.x
	yr := y - (g.center.y + centerHeaderHeight) - mv.topPad
	if yr < 0 {
		return 0, false, 0, false
	}
	fullY := yr + mv.start
	for _, hit := range mv.canvas.hits {
		if hit.voice != nil && hit.voice.contains(xr, fullY) {
			return hit.msg, true, 0, true
		}
		if hit.bubble.contains(xr, fullY) {
			if fullY == hit.barY && hit.barX > 0 && xr >= hit.barX && xr < hit.barX+hit.barW {
				frac := float64(xr-hit.barX) / float64(maxInt(1, hit.barW))
				return hit.msg, true, frac, true
			}
			return hit.msg, false, 0, true
		}
	}
	return 0, false, 0, false
}
