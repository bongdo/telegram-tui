package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"tg-matrix-tui/internal/ui"
)

func main() {
	p := tea.NewProgram(ui.New(), tea.WithFilter(ui.Filter))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "telegram tui:", err)
		os.Exit(1)
	}
}
