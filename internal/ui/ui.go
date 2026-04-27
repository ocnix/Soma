package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"soma/internal/state"
)

// Run takes over the terminal and renders the playback UI until the user
// quits or the track finishes.
func Run(st *state.State, done <-chan struct{}) error {
	p := tea.NewProgram(model{st: st}, tea.WithAltScreen())

	go func() {
		<-done
		p.Send(finishedMsg{})
	}()

	_, err := p.Run()
	return err
}

type tickMsg time.Time
type finishedMsg struct{}

type model struct {
	st            *state.State
	width, height int
	quitting      bool
}

func (m model) Init() tea.Cmd {
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(time.Second/30, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		return m, tick()
	case finishedMsg:
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// ── Styling ──────────────────────────────────────────────────────────────────

var (
	// Coffee palette: cream, espresso, gold, mocha.
	colCream    = lipgloss.Color("223")
	colGold     = lipgloss.Color("214")
	colEspresso = lipgloss.Color("94")
	colDim      = lipgloss.Color("241")
	colDimmer   = lipgloss.Color("237")
	colGreen    = lipgloss.Color("78")
	colYellow   = lipgloss.Color("221")
	colRed      = lipgloss.Color("203")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colGold)
	subStyle   = lipgloss.NewStyle().Faint(true).Foreground(colDim)
	labelStyle = lipgloss.NewStyle().Foreground(colDim).Italic(true)
	trackStyle = lipgloss.NewStyle().Bold(true).Foreground(colCream)
	dotStyle   = lipgloss.NewStyle().Foreground(colGold).Bold(true)
	railStyle  = lipgloss.NewStyle().Foreground(colEspresso)
	bgStyle    = lipgloss.NewStyle().Foreground(colDimmer)
	statStyle  = lipgloss.NewStyle().Foreground(colCream)
	helpStyle  = lipgloss.NewStyle().Faint(true).Foreground(colDim)
	boxStyle   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colEspresso).
			Padding(1, 3)
)

func (m model) View() string {
	if m.quitting {
		return subStyle.Render("thanks for spinning ☕") + "\n"
	}
	s := m.st.Snapshot()

	barW := 44

	header := titleStyle.Render("soma") + "  " + subStyle.Render("· 8D audio for focus ·") + "  " + dotStyle.Render("☕")

	track := trackStyle.Render("▸ " + s.Title)

	pan := labelStyle.Render("spatial position") + "\n" + panBar(s.Pan, barW)

	meters := labelStyle.Render("levels") + "\n" +
		subStyle.Render("L ") + vuBar(s.LevelL, barW) + "  " + dbLabel(s.LevelL) + "\n" +
		subStyle.Render("R ") + vuBar(s.LevelR, barW) + "  " + dbLabel(s.LevelR)

	timeStr := fmt.Sprintf("%s / %s", fmtDur(s.Elapsed), fmtDur(s.Duration))
	modeStr := fmt.Sprintf("%s · %.2f Hz", s.Mode, s.RateHz)
	status := statStyle.Render(timeStr) + "    " + subStyle.Render(modeStr)

	help := helpStyle.Render("[q] quit")

	body := strings.Join([]string{
		header,
		"",
		track,
		"",
		pan,
		"",
		meters,
		"",
		status,
		"",
		help,
	}, "\n")

	return boxStyle.Render(body)
}

// ── Widgets ──────────────────────────────────────────────────────────────────

func panBar(pan float64, width int) string {
	inner := width - 4
	if inner < 8 {
		inner = 8
	}
	pos := int(((pan + 1) / 2) * float64(inner-1))
	if pos < 0 {
		pos = 0
	}
	if pos >= inner {
		pos = inner - 1
	}

	var sb strings.Builder
	sb.WriteString(subStyle.Render("L "))
	for i := 0; i < inner; i++ {
		switch {
		case i == pos:
			sb.WriteString(dotStyle.Render("●"))
		case i == 0 || i == inner-1:
			sb.WriteString(railStyle.Render("├"))
		case i == inner/2:
			sb.WriteString(railStyle.Render("┼"))
		default:
			sb.WriteString(railStyle.Render("─"))
		}
	}
	sb.WriteString(subStyle.Render(" R"))
	return sb.String()
}

func vuBar(level float64, width int) string {
	bars := int(level * float64(width))
	if bars > width {
		bars = width
	}
	yellowAt := int(0.65 * float64(width))
	redAt := int(0.88 * float64(width))

	var sb strings.Builder
	for i := 0; i < width; i++ {
		switch {
		case i >= bars:
			sb.WriteString(bgStyle.Render("░"))
		case i >= redAt:
			sb.WriteString(lipgloss.NewStyle().Foreground(colRed).Render("█"))
		case i >= yellowAt:
			sb.WriteString(lipgloss.NewStyle().Foreground(colYellow).Render("█"))
		default:
			sb.WriteString(lipgloss.NewStyle().Foreground(colGreen).Render("█"))
		}
	}
	return sb.String()
}

func dbLabel(level float64) string {
	if level < 1e-4 {
		return subStyle.Render("  -∞ dB")
	}
	db := 20 * math.Log10(level)
	return subStyle.Render(fmt.Sprintf("%5.1f dB", db))
}

func fmtDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}
