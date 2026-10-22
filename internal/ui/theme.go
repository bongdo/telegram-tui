package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme carries the handful of Telegram Web A tokens we actually use.
// Values are taken from telegram-tt's src/styles/themes.json (light/dark pairs).
type Theme struct {
	Dark bool

	BG         color.Color // app background
	Panel      color.Color // sidebar/profile background
	PanelAlt   color.Color // search field, input, badges
	Hover      color.Color
	Selected   color.Color
	Border     color.Color
	ChatBG     color.Color // conversation wallpaper approximation
	DateBG     color.Color
	DateText   color.Color
	Text       color.Color
	Muted      color.Color
	Faint      color.Color
	Primary    color.Color
	OnPrimary  color.Color
	ChatActive color.Color
	Online     color.Color
	Error      color.Color

	OwnBG   color.Color
	OwnText color.Color
	OwnMeta color.Color
	InBG    color.Color
	InText  color.Color
	InMeta  color.Color
}

func darkTheme() Theme {
	return Theme{
		Dark:       true,
		BG:         lipgloss.Color("#212121"),
		Panel:      lipgloss.Color("#212121"),
		PanelAlt:   lipgloss.Color("#2C2C2C"),
		Hover:      lipgloss.Color("#2C2C2C"),
		Selected:   lipgloss.Color("#2C2C2C"),
		Border:     lipgloss.Color("#303030"),
		ChatBG:     lipgloss.Color("#0F0F0F"),
		DateBG:     lipgloss.Color("#2C2C2C"),
		DateText:   lipgloss.Color("#AAAAAA"),
		Text:       lipgloss.Color("#FFFFFF"),
		Muted:      lipgloss.Color("#AAAAAA"),
		Faint:      lipgloss.Color("#7A7A7A"),
		Primary:    lipgloss.Color("#8774E1"),
		OnPrimary:  lipgloss.Color("#FFFFFF"),
		ChatActive: lipgloss.Color("#766AC8"),
		Online:     lipgloss.Color("#4DCD5E"),
		Error:      lipgloss.Color("#E53935"),
		OwnBG:      lipgloss.Color("#766AC8"),
		OwnText:    lipgloss.Color("#FFFFFF"),
		OwnMeta:    lipgloss.Color("#DED9F5"),
		InBG:       lipgloss.Color("#212121"),
		InText:     lipgloss.Color("#FFFFFF"),
		InMeta:     lipgloss.Color("#AAAAAA"),
	}
}

func lightTheme() Theme {
	return Theme{
		Dark:       false,
		BG:         lipgloss.Color("#FFFFFF"),
		Panel:      lipgloss.Color("#FFFFFF"),
		PanelAlt:   lipgloss.Color("#F4F4F5"),
		Hover:      lipgloss.Color("#F4F4F5"),
		Selected:   lipgloss.Color("#F4F4F5"),
		Border:     lipgloss.Color("#DADCE0"),
		ChatBG:     lipgloss.Color("#E7EBEF"),
		DateBG:     lipgloss.Color("#FFFFFF"),
		DateText:   lipgloss.Color("#707579"),
		Text:       lipgloss.Color("#000000"),
		Muted:      lipgloss.Color("#707579"),
		Faint:      lipgloss.Color("#A2ACB4"),
		Primary:    lipgloss.Color("#3390EC"),
		OnPrimary:  lipgloss.Color("#FFFFFF"),
		ChatActive: lipgloss.Color("#3390EC"),
		Online:     lipgloss.Color("#4FAE4E"),
		Error:      lipgloss.Color("#E53935"),
		OwnBG:      lipgloss.Color("#EEFFDE"),
		OwnText:    lipgloss.Color("#000000"),
		OwnMeta:    lipgloss.Color("#4FAE4E"),
		InBG:       lipgloss.Color("#FFFFFF"),
		InText:     lipgloss.Color("#000000"),
		InMeta:     lipgloss.Color("#707579"),
	}
}

func (t Theme) toggle() Theme {
	if t.Dark {
		return lightTheme()
	}
	return darkTheme()
}
