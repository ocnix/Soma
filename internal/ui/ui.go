package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"soma/internal/player"
	"soma/internal/source"
)

// Run boots the interactive TUI. If initialPath is non-empty, the home screen
// is skipped and playback starts immediately.
func Run(initialPath string, rate float64, dry bool) error {
	ti := textinput.New()
	ti.Placeholder = "paste a youtube link, or path to an mp3..."
	ti.Focus()
	ti.CharLimit = 1024
	ti.Width = 60
	ti.Prompt = "▸ "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colGold).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colCream)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colGold)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colGold)

	m := &model{
		screen:  screenHome,
		input:   ti,
		spinner: sp,
		rate:    rate,
		dry:     dry,
	}

	prog := tea.NewProgram(m, tea.WithAltScreen())
	m.prog = prog

	if initialPath != "" {
		m.screen = screenLoading
		m.loadingArg = initialPath
		go func() {
			prog.Send(m.startLoadCmd(initialPath)())
		}()
	}

	_, err := prog.Run()
	return err
}

// ── Screens & messages ───────────────────────────────────────────────────────

type screen int

const (
	screenHome screen = iota
	screenLoading
	screenPlaying
)

type tickMsg time.Time
type startedMsg struct{ sess *player.Session }
type errMsg struct{ err error }
type finishedMsg struct{}

// ── Model ────────────────────────────────────────────────────────────────────

type model struct {
	screen screen

	input   textinput.Model
	spinner spinner.Model
	rate    float64
	dry     bool

	loadingArg string
	sess       *player.Session
	err        error

	width, height int

	prog *tea.Program
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(tick(), m.spinner.Tick)
}

func tick() tea.Cmd {
	return tea.Tick(time.Second/30, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *model) startLoadCmd(arg string) tea.Cmd {
	return func() tea.Msg {
		path, cleanup, err := source.Resolve(arg)
		if err != nil {
			return errMsg{err}
		}
		sess, err := player.Start(path, m.rate, m.dry)
		if err != nil {
			cleanup()
			return errMsg{err}
		}
		sess.OnClose(cleanup)
		// Pipe the session's Done channel back as a tea message.
		go func() {
			<-sess.Done
			m.prog.Send(finishedMsg{})
		}()
		return startedMsg{sess}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		// Global quit.
		if msg.String() == "ctrl+c" {
			m.teardown()
			return m, tea.Quit
		}
		switch m.screen {
		case screenHome:
			return m.updateHome(msg)
		case screenLoading:
			if msg.String() == "esc" {
				m.screen = screenHome
				m.err = nil
				return m, nil
			}
		case screenPlaying:
			return m.updatePlaying(msg)
		}

	case startedMsg:
		m.sess = msg.sess
		m.screen = screenPlaying
		m.err = nil
		return m, tick()

	case errMsg:
		m.err = msg.err
		m.screen = screenHome
		m.input.Focus()
		return m, nil

	case finishedMsg:
		if m.sess != nil {
			m.sess.Close()
			m.sess = nil
		}
		m.screen = screenHome
		m.input.SetValue("")
		m.input.Focus()
		return m, nil

	case tickMsg:
		return m, tick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if m.screen == screenHome {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.teardown()
		return m, tea.Quit
	case "enter":
		val := strings.TrimSpace(m.input.Value())
		if val == "" {
			return m, nil
		}
		m.loadingArg = val
		m.screen = screenLoading
		m.err = nil
		return m, tea.Batch(m.startLoadCmd(val), m.spinner.Tick)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) updatePlaying(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.teardown()
		return m, tea.Quit
	case "esc":
		if m.sess != nil {
			m.sess.Close()
			m.sess = nil
		}
		m.screen = screenHome
		m.input.SetValue("")
		m.input.Focus()
		return m, nil
	case " ", "space":
		if m.sess != nil {
			m.sess.TogglePause()
		}
		return m, nil
	}
	return m, nil
}

func (m *model) teardown() {
	if m.sess != nil {
		m.sess.Close()
		m.sess = nil
	}
}

// ── Styling ──────────────────────────────────────────────────────────────────

var (
	colCream    = lipgloss.Color("223")
	colGold     = lipgloss.Color("214")
	colEspresso = lipgloss.Color("94")
	colDim      = lipgloss.Color("241")
	colDimmer   = lipgloss.Color("237")
	colGreen    = lipgloss.Color("78")
	colYellow   = lipgloss.Color("221")
	colRed      = lipgloss.Color("203")
	colError    = lipgloss.Color("203")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colGold)
	subStyle   = lipgloss.NewStyle().Faint(true).Foreground(colDim)
	labelStyle = lipgloss.NewStyle().Foreground(colDim).Italic(true)
	trackStyle = lipgloss.NewStyle().Bold(true).Foreground(colCream)
	dotStyle   = lipgloss.NewStyle().Foreground(colGold).Bold(true)
	railStyle  = lipgloss.NewStyle().Foreground(colEspresso)
	bgStyle    = lipgloss.NewStyle().Foreground(colDimmer)
	statStyle  = lipgloss.NewStyle().Foreground(colCream)
	helpStyle  = lipgloss.NewStyle().Faint(true).Foreground(colDim)
	errStyle   = lipgloss.NewStyle().Foreground(colError)
	pauseStyle = lipgloss.NewStyle().Bold(true).Foreground(colYellow)
	boxStyle   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colEspresso).
			Padding(1, 3)
)

// ── View ─────────────────────────────────────────────────────────────────────

func (m *model) View() string {
	switch m.screen {
	case screenHome:
		return m.viewHome()
	case screenLoading:
		return m.viewLoading()
	case screenPlaying:
		return m.viewPlaying()
	}
	return ""
}

func (m *model) viewHome() string {
	header := titleStyle.Render("soma") + "  " + subStyle.Render("· 8D audio for focus ·") + "  " + dotStyle.Render("☕")

	tagline := subStyle.Render("the modern soma — coffee for the ears.")

	prompt := labelStyle.Render("what do you want to spin?") + "\n" + m.input.View()

	help := helpStyle.Render("[enter] play   [q] quit")

	parts := []string{header, "", tagline, "", prompt, "", help}
	if m.err != nil {
		parts = append(parts, "", errStyle.Render("× "+m.err.Error()))
	}
	return boxStyle.Render(strings.Join(parts, "\n"))
}

func (m *model) viewLoading() string {
	header := titleStyle.Render("soma") + "  " + dotStyle.Render("☕")
	what := subStyle.Render(m.loadingArg)
	body := strings.Join([]string{
		header,
		"",
		m.spinner.View() + " " + statStyle.Render("brewing..."),
		"",
		what,
		"",
		helpStyle.Render("[esc] cancel   [q] quit"),
	}, "\n")
	return boxStyle.Render(body)
}

func (m *model) viewPlaying() string {
	if m.sess == nil {
		return boxStyle.Render(subStyle.Render("...\n"))
	}
	s := m.sess.State.Snapshot()

	barW := 44

	header := titleStyle.Render("soma") + "  " + subStyle.Render("· 8D audio for focus ·") + "  " + dotStyle.Render("☕")

	playMark := "▸"
	if s.Paused {
		playMark = "‖"
	}
	track := trackStyle.Render(playMark + " " + s.Title)

	pan := labelStyle.Render("spatial position") + "\n" + panBar(s.Pan, barW)

	meters := labelStyle.Render("levels") + "\n" +
		subStyle.Render("L ") + vuBar(s.LevelL, barW) + "  " + dbLabel(s.LevelL) + "\n" +
		subStyle.Render("R ") + vuBar(s.LevelR, barW) + "  " + dbLabel(s.LevelR)

	timeStr := fmt.Sprintf("%s / %s", fmtDur(s.Elapsed), fmtDur(s.Duration))
	modeStr := fmt.Sprintf("%s · %.2f Hz", s.Mode, s.RateHz)
	pausedTag := ""
	if s.Paused {
		pausedTag = "    " + pauseStyle.Render("PAUSED")
	}
	status := statStyle.Render(timeStr) + "    " + subStyle.Render(modeStr) + pausedTag

	help := helpStyle.Render("[space] pause   [esc] back   [q] quit")

	body := strings.Join([]string{
		header, "",
		track, "",
		pan, "",
		meters, "",
		status, "",
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
