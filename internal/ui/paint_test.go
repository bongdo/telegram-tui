package ui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)

func oldPaint(s string, fg, bg color.Color, bold bool) string {
	st := lipgloss.NewStyle().Foreground(fg).Background(bg)
	if bold {
		st = st.Bold(true)
	}
	return st.Render(s)
}

func TestPaintMatchesLipgloss(t *testing.T) {
	th := darkTheme()
	colors := []color.Color{th.Text, th.Muted, th.Primary, th.OnPrimary, th.Panel, th.PanelAlt, th.ChatActive, th.Hover, th.Error, lipgloss.Color("#7f6aed")}
	for _, fg := range colors {
		for _, bg := range colors {
			for _, bold := range []bool{false, true} {
				for _, s := range []string{"abc", "  ", "Привет ✓", "a\tb", "x"} {
					got := paint(s, fg, bg, bold)
					want := oldPaint(s, fg, bg, bold)
					if got != want {
						t.Fatalf("paint(%q, %v, %v, %v)\n got %q\nwant %q", s, fg, bg, bold, got, want)
					}
				}
			}
		}
	}
}
