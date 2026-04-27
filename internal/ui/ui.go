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

	prog := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.prog = prog

	if initialPath != "" {
		m.screen = screenLoading
		m.loadingArg = initialPath
		go func() { prog.Send(m.startLoadCmd(initialPath)()) }()
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
	case "ctrl+q":
		m.teardown()
		return m, tea.Quit
	case "esc":
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
	case "left":
		if m.sess != nil {
			m.sess.SeekRelative(-5 * time.Second)
		}
	case "right":
		if m.sess != nil {
			m.sess.SeekRelative(5 * time.Second)
		}
	case "up":
		if m.sess != nil {
			m.sess.AdjustVolume(2)
		}
	case "down":
		if m.sess != nil {
			m.sess.AdjustVolume(-2)
		}
	case "[":
		if m.sess != nil {
			m.sess.AdjustRate(-0.05)
		}
	case "]":
		if m.sess != nil {
			m.sess.AdjustRate(0.05)
		}
	case "r":
		if m.sess != nil {
			m.sess.ToggleReverb()
		}
	case "d":
		if m.sess != nil {
			m.sess.ToggleEffect()
		}
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
	colBean     = lipgloss.Color("130")
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
	keyStyle   = lipgloss.NewStyle().Bold(true).Foreground(colGold)
	errStyle   = lipgloss.NewStyle().Foreground(colRed)
	pauseStyle = lipgloss.NewStyle().Bold(true).Foreground(colYellow)
	onStyle    = lipgloss.NewStyle().Foreground(colGreen).Bold(true)
	offStyle   = lipgloss.NewStyle().Faint(true).Foreground(colDim)
	beanStyle  = lipgloss.NewStyle().Foreground(colBean)

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colEspresso).
			Padding(0, 2)
	titleBarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colGold).
			Padding(0, 2)
)

// ── View ─────────────────────────────────────────────────────────────────────

func (m *model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
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

func (m *model) header() string {
	left := titleStyle.Render("soma")
	mid := subStyle.Render("· 8D audio for focus ·")
	right := beanStyle.Render("☕")
	bar := fmt.Sprintf("%s   %s   %s", left, mid, right)
	return titleBarStyle.Width(m.width - 2).Render(bar)
}

func (m *model) viewHome() string {
	header := m.header()

	tagline := lipgloss.NewStyle().Foreground(colCream).Italic(true).
		Render(`"coffee is the modern soma — first drunk by sufis to focus through the night."`)

	prompt := labelStyle.Render("what do you want to spin?") + "\n" + m.input.View()
	help := helpStyle.Render(keyStyle.Render("enter") + " play   " + keyStyle.Render("esc") + " quit")

	var errLine string
	if m.err != nil {
		errLine = "\n\n" + errStyle.Render("× "+m.err.Error())
	}

	body := lipgloss.JoinVertical(lipgloss.Center,
		"",
		tagline,
		"",
		"",
		prompt,
		"",
		help,
	) + errLine

	bodyW := m.width - 2
	bodyH := m.height - lipgloss.Height(header) - 2

	centered := lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Center, body)
	pane := paneStyle.Width(bodyW).Render(centered)

	return lipgloss.JoinVertical(lipgloss.Left, header, pane)
}

func (m *model) viewLoading() string {
	header := m.header()

	what := strings.TrimSpace(m.loadingArg)
	if len(what) > m.width-10 {
		what = what[:m.width-10] + "…"
	}

	body := lipgloss.JoinVertical(lipgloss.Center,
		m.spinner.View()+"  "+statStyle.Render("brewing..."),
		"",
		subStyle.Render(what),
		"",
		"",
		helpStyle.Render(keyStyle.Render("esc")+" cancel"),
	)

	bodyW := m.width - 2
	bodyH := m.height - lipgloss.Height(header) - 2
	centered := lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Center, body)
	pane := paneStyle.Width(bodyW).Render(centered)
	return lipgloss.JoinVertical(lipgloss.Left, header, pane)
}

func (m *model) viewPlaying() string {
	if m.sess == nil {
		return ""
	}
	s := m.sess.State.Snapshot()

	header := m.header()

	// Status pane (sticky bottom): controls + values + help
	statusPane := m.renderStatusPane(s, m.width-2)
	progress := m.renderProgress(s, m.width-2)
	levels := m.renderLevels(s, m.width-2)
	titleLine := m.renderNowPlaying(s, m.width-2)

	// Compute remaining vertical space for the orbital.
	used := lipgloss.Height(header) +
		lipgloss.Height(titleLine) +
		lipgloss.Height(levels) +
		lipgloss.Height(progress) +
		lipgloss.Height(statusPane) +
		2 // small breathing room
	orbitH := m.height - used
	if orbitH < 6 {
		orbitH = 6
	}
	orbitW := minInt(m.width-4, orbitH*4) // keep aspect-ish (cells are ~2:1)
	if orbitW < 16 {
		orbitW = 16
	}
	orbital := orbitalView(s.Phase, orbitW, orbitH)
	orbitalCentered := lipgloss.Place(m.width-2, orbitH, lipgloss.Center, lipgloss.Center, orbital)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		titleLine,
		orbitalCentered,
		levels,
		progress,
		statusPane,
	)
}

// ── Sections ─────────────────────────────────────────────────────────────────

func (m *model) renderNowPlaying(s anySnapshot, width int) string {
	playMark := dotStyle.Render("▸")
	if s.Paused {
		playMark = pauseStyle.Render("‖")
	}
	title := s.Title
	if len(title) > width-8 {
		title = title[:width-8] + "…"
	}
	line := playMark + "  " + trackStyle.Render(title)
	return lipgloss.NewStyle().Width(width).Padding(1, 2).Render(line)
}

func (m *model) renderLevels(s anySnapshot, width int) string {
	innerW := width - 8
	if innerW < 20 {
		innerW = 20
	}
	barW := innerW - 14 // room for "L  " + label "  -12.3 dB"
	if barW < 10 {
		barW = 10
	}
	left := subStyle.Render("L ") + vuBar(s.LevelL, barW) + "  " + dbLabel(s.LevelL)
	right := subStyle.Render("R ") + vuBar(s.LevelR, barW) + "  " + dbLabel(s.LevelR)
	body := left + "\n" + right
	return lipgloss.NewStyle().Padding(0, 2).Width(width).Render(body)
}

func (m *model) renderProgress(s anySnapshot, width int) string {
	innerW := width - 4
	timeL := fmtDur(s.Elapsed)
	timeR := fmtDur(s.Duration)
	barW := innerW - len(timeL) - len(timeR) - 4
	if barW < 10 {
		barW = 10
	}
	frac := 0.0
	if s.Duration > 0 {
		frac = float64(s.Elapsed) / float64(s.Duration)
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	pos := int(frac * float64(barW-1))

	var sb strings.Builder
	for i := 0; i < barW; i++ {
		switch {
		case i == pos:
			sb.WriteString(dotStyle.Render("●"))
		case i < pos:
			sb.WriteString(lipgloss.NewStyle().Foreground(colGold).Render("═"))
		default:
			sb.WriteString(railStyle.Render("─"))
		}
	}
	body := statStyle.Render(timeL) + "  " + sb.String() + "  " + statStyle.Render(timeR)
	return lipgloss.NewStyle().Padding(0, 2).Width(width).Render(body)
}

func (m *model) renderStatusPane(s anySnapshot, width int) string {
	mode := s.Mode
	if mode == "" {
		mode = "8D"
	}
	modeChip := onStyle.Render(" 8D ")
	if mode == "dry" {
		modeChip = offStyle.Render(" dry ")
	}
	reverbChip := offStyle.Render(" reverb off ")
	if s.ReverbOn {
		reverbChip = onStyle.Render(" reverb on ")
	}
	pausedChip := ""
	if s.Paused {
		pausedChip = "  " + pauseStyle.Render(" PAUSED ")
	}

	values := fmt.Sprintf("%s %s    %s %s    %s    %s%s",
		labelStyle.Render("vol"), statStyle.Render(fmtDb(s.VolumeDb)),
		labelStyle.Render("rate"), statStyle.Render(fmt.Sprintf("%.2f Hz", s.RateHz)),
		modeChip,
		reverbChip,
		pausedChip,
	)

	help1 := helpStyle.Render(
		keyStyle.Render("space") + " pause   " +
			keyStyle.Render("←/→") + " seek   " +
			keyStyle.Render("↑/↓") + " vol   " +
			keyStyle.Render("[ / ]") + " rate   " +
			keyStyle.Render("r") + " reverb   " +
			keyStyle.Render("d") + " 8D",
	)
	help2 := helpStyle.Render(
		keyStyle.Render("esc") + " back to home   " +
			keyStyle.Render("q") + " quit",
	)

	body := values + "\n\n" + help1 + "\n" + help2

	return paneStyle.Width(width).Render(body)
}

// ── Orbital visualizer ───────────────────────────────────────────────────────

func orbitalView(phase float64, w, h int) string {
	if w < 12 {
		w = 12
	}
	if h < 6 {
		h = 6
	}
	grid := make([][]rune, h)
	style := make([][]int, h) // 0 = blank, 1 = rail, 2 = dot, 3 = center
	for i := range grid {
		grid[i] = make([]rune, w)
		style[i] = make([]int, w)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	rx := float64(w-1) / 2 * 0.85
	ry := float64(h-1) / 2 * 0.85

	// Faint orbit ellipse.
	for t := 0.0; t < 2*math.Pi; t += 0.04 {
		x := cx + rx*math.Sin(t)
		y := cy - ry*math.Cos(t)
		ix, iy := int(math.Round(x)), int(math.Round(y))
		if ix >= 0 && ix < w && iy >= 0 && iy < h && grid[iy][ix] == ' ' {
			grid[iy][ix] = '·'
			style[iy][ix] = 1
		}
	}

	// Center mark — your head.
	icx, icy := int(math.Round(cx)), int(math.Round(cy))
	if icx >= 0 && icx < w && icy >= 0 && icy < h {
		grid[icy][icx] = '◌'
		style[icy][icx] = 3
	}

	// Trailing comet (last several positions, dimmer further back).
	for k := 1; k <= 6; k++ {
		t := phase - float64(k)*0.12
		x := cx + rx*math.Sin(t)
		y := cy - ry*math.Cos(t)
		ix, iy := int(math.Round(x)), int(math.Round(y))
		if ix >= 0 && ix < w && iy >= 0 && iy < h {
			if style[iy][ix] != 2 { // don't overwrite the head
				grid[iy][ix] = '∙'
				style[iy][ix] = 1
			}
		}
	}

	// The orbiting dot.
	dx := cx + rx*math.Sin(phase)
	dy := cy - ry*math.Cos(phase)
	ix, iy := int(math.Round(dx)), int(math.Round(dy))
	if ix >= 0 && ix < w && iy >= 0 && iy < h {
		grid[iy][ix] = '●'
		style[iy][ix] = 2
	}

	var sb strings.Builder
	for i, row := range grid {
		for j, r := range row {
			switch style[i][j] {
			case 1:
				sb.WriteString(railStyle.Render(string(r)))
			case 2:
				sb.WriteString(dotStyle.Render(string(r)))
			case 3:
				sb.WriteString(beanStyle.Render(string(r)))
			default:
				sb.WriteRune(r)
			}
		}
		if i < len(grid)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// ── Widgets ──────────────────────────────────────────────────────────────────

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

func fmtDb(db float64) string {
	if db <= -50 {
		return "muted"
	}
	if db == 0 {
		return "0 dB"
	}
	return fmt.Sprintf("%+.0f dB", db)
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// anySnapshot lets us pass either state.Snapshot or anything with the same
// fields without importing state directly here. (We do already import state
// in this package indirectly via player; this alias keeps view code clean.)
type anySnapshot = struct {
	Title    string
	Mode     string
	RateHz   float64
	VolumeDb float64
	ReverbOn bool
	Duration time.Duration
	Elapsed  time.Duration
	Pan      float64
	Phase    float64
	LevelL   float64
	LevelR   float64
	Finished bool
	Paused   bool
}
