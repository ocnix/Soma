package ui

import "github.com/charmbracelet/lipgloss"

// Theme defines the palette for the whole UI. Switch with `t` on any screen.
type Theme struct {
	Name    string
	Display string

	Cream    lipgloss.Color
	Gold     lipgloss.Color
	Bean     lipgloss.Color
	Espresso lipgloss.Color
	Dim      lipgloss.Color
	Dimmer   lipgloss.Color
	Green    lipgloss.Color
	Yellow   lipgloss.Color
	Red      lipgloss.Color
}

// Themes are the available palettes, in cycle order.
var Themes = []Theme{
	{
		Name: "coffee", Display: "Coffee",
		Cream: "223", Gold: "214", Bean: "130", Espresso: "94",
		Dim: "241", Dimmer: "237", Green: "78", Yellow: "221", Red: "203",
	},
	{
		Name: "midnight", Display: "Midnight",
		Cream: "189", Gold: "147", Bean: "111", Espresso: "61",
		Dim: "243", Dimmer: "237", Green: "115", Yellow: "186", Red: "210",
	},
	{
		Name: "forest", Display: "Forest",
		Cream: "193", Gold: "108", Bean: "100", Espresso: "65",
		Dim: "243", Dimmer: "237", Green: "78", Yellow: "186", Red: "203",
	},
	{
		Name: "cream", Display: "Cream",
		Cream: "230", Gold: "172", Bean: "130", Espresso: "137",
		Dim: "245", Dimmer: "250", Green: "71", Yellow: "172", Red: "167",
	},
}

func themeByName(name string) Theme {
	for _, t := range Themes {
		if t.Name == name {
			return t
		}
	}
	return Themes[0]
}

func nextTheme(current string) Theme {
	for i, t := range Themes {
		if t.Name == current {
			return Themes[(i+1)%len(Themes)]
		}
	}
	return Themes[0]
}

// Mutable color + style globals. View code reads these directly. Call
// applyTheme to swap the palette; the underlying lipgloss styles are rebuilt.
var (
	colCream    lipgloss.Color
	colGold     lipgloss.Color
	colBean     lipgloss.Color
	colEspresso lipgloss.Color
	colDim      lipgloss.Color
	colDimmer   lipgloss.Color
	colGreen    lipgloss.Color
	colYellow   lipgloss.Color
	colRed      lipgloss.Color

	titleStyle    lipgloss.Style
	subStyle      lipgloss.Style
	labelStyle    lipgloss.Style
	trackStyle    lipgloss.Style
	dotStyle      lipgloss.Style
	railStyle     lipgloss.Style
	bgStyle       lipgloss.Style
	statStyle     lipgloss.Style
	helpStyle     lipgloss.Style
	keyStyle      lipgloss.Style
	errStyle      lipgloss.Style
	pauseStyle    lipgloss.Style
	onStyle       lipgloss.Style
	offStyle      lipgloss.Style
	beanStyle     lipgloss.Style
	paneStyle     lipgloss.Style
	titleBarStyle lipgloss.Style
)

func init() {
	applyTheme(Themes[0])
}

func applyTheme(t Theme) {
	colCream = t.Cream
	colGold = t.Gold
	colBean = t.Bean
	colEspresso = t.Espresso
	colDim = t.Dim
	colDimmer = t.Dimmer
	colGreen = t.Green
	colYellow = t.Yellow
	colRed = t.Red

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colGold)
	subStyle = lipgloss.NewStyle().Faint(true).Foreground(colDim)
	labelStyle = lipgloss.NewStyle().Foreground(colDim).Italic(true)
	trackStyle = lipgloss.NewStyle().Bold(true).Foreground(colCream)
	dotStyle = lipgloss.NewStyle().Foreground(colGold).Bold(true)
	railStyle = lipgloss.NewStyle().Foreground(colEspresso)
	bgStyle = lipgloss.NewStyle().Foreground(colDimmer)
	statStyle = lipgloss.NewStyle().Foreground(colCream)
	helpStyle = lipgloss.NewStyle().Faint(true).Foreground(colDim)
	keyStyle = lipgloss.NewStyle().Bold(true).Foreground(colGold)
	errStyle = lipgloss.NewStyle().Foreground(colRed)
	pauseStyle = lipgloss.NewStyle().Bold(true).Foreground(colYellow)
	onStyle = lipgloss.NewStyle().Foreground(colGreen).Bold(true)
	offStyle = lipgloss.NewStyle().Faint(true).Foreground(colDim)
	beanStyle = lipgloss.NewStyle().Foreground(colBean)

	paneStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colEspresso).
		Padding(0, 2)
	titleBarStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colGold).
		Padding(0, 2)
}
