package tui

import "github.com/charmbracelet/lipgloss"

// Palette holds the colors the TUI uses. Centralized so swapping themes
// later (light/dark, custom) is a single edit. Names describe semantic
// role, not hex values.
type Palette struct {
	Primary  lipgloss.Color // headlines, focused things
	Muted    lipgloss.Color // secondary text, separators
	Success  lipgloss.Color // running timers, completed entries
	Warning  lipgloss.Color // paused timers, idle states
	Danger   lipgloss.Color // errors
	Subtle   lipgloss.Color // fine grid lines, hints
	Fg       lipgloss.Color // default foreground
	BgSubtle lipgloss.Color // panel backgrounds
}

// DefaultPalette is a calm "modern terminal" palette. Tuned for dark
// backgrounds; works on light too because we don't rely on background
// fills for legibility.
var DefaultPalette = Palette{
	Primary:  lipgloss.Color("#7aa2f7"),
	Muted:    lipgloss.Color("#565f89"),
	Success:  lipgloss.Color("#9ece6a"),
	Warning:  lipgloss.Color("#e0af68"),
	Danger:   lipgloss.Color("#f7768e"),
	Subtle:   lipgloss.Color("#414868"),
	Fg:       lipgloss.Color("#c0caf5"),
	BgSubtle: lipgloss.Color("#1a1b26"),
}

// Styles bundles every prebuilt style. Built once on startup; do not
// mutate at runtime — lipgloss styles are values, not refs.
type Styles struct {
	Title     lipgloss.Style
	Section   lipgloss.Style
	Panel     lipgloss.Style
	Hint      lipgloss.Style
	StateOK   lipgloss.Style
	StateWarn lipgloss.Style
	StateBad  lipgloss.Style
	Muted     lipgloss.Style
	Bold      lipgloss.Style
	Footer    lipgloss.Style
	Key       lipgloss.Style
	KeyDesc   lipgloss.Style
}

// NewStyles builds the prebuilt styles from a palette.
func NewStyles(p Palette) Styles {
	return Styles{
		Title:     lipgloss.NewStyle().Foreground(p.Primary).Bold(true),
		Section:   lipgloss.NewStyle().Foreground(p.Primary).Bold(true).MarginBottom(1),
		Panel:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Subtle).Padding(0, 1),
		Hint:      lipgloss.NewStyle().Foreground(p.Muted).Italic(true),
		StateOK:   lipgloss.NewStyle().Foreground(p.Success),
		StateWarn: lipgloss.NewStyle().Foreground(p.Warning),
		StateBad:  lipgloss.NewStyle().Foreground(p.Danger),
		Muted:     lipgloss.NewStyle().Foreground(p.Muted),
		Bold:      lipgloss.NewStyle().Foreground(p.Fg).Bold(true),
		Footer:    lipgloss.NewStyle().Foreground(p.Muted).MarginTop(1),
		Key:       lipgloss.NewStyle().Foreground(p.Primary).Bold(true),
		KeyDesc:   lipgloss.NewStyle().Foreground(p.Muted),
	}
}
