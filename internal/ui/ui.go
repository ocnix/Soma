package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"soma/internal/config"
	"soma/internal/player"
	"soma/internal/source"
)

// Run boots the interactive TUI. If initialPath is non-empty, the home screen
// is skipped and playback starts immediately with the given profile.
func Run(initialPath string, profile config.Profile) error {
	applyTheme(themeByName(profile.Theme))

	ti := textinput.New()
	ti.Placeholder = "paste a youtube link, or path to an mp3..."
	ti.Focus()
	ti.CharLimit = 1024
	ti.Width = 60
	ti.Prompt = "▸ "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colGold).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colCream)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colGold)

	inboxInput := textinput.New()
	inboxInput.Placeholder = "what's on your mind? (esc to cancel)"
	inboxInput.CharLimit = 280
	inboxInput.Width = 60
	inboxInput.Prompt = "✎ "
	inboxInput.PromptStyle = lipgloss.NewStyle().Foreground(colGold)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colGold)

	m := &model{
		screen:        screenHome,
		input:         ti,
		inboxInput:    inboxInput,
		spinner:       sp,
		profile:       profile,
		recents:       config.LoadRecents(),
		recentsCursor: -1,
		todayTotal:    config.TodayTotal(),
		streak:        config.Streak(),
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
type sleepFiredMsg struct{}

// ── Model ────────────────────────────────────────────────────────────────────

type model struct {
	screen screen

	input      textinput.Model
	inboxInput textinput.Model
	spinner    spinner.Model

	profile config.Profile

	loadingArg string
	sess       *player.Session
	err        error

	width, height int

	// home extras
	recents       []config.RecentItem
	recentsCursor int // -1 = on input, 0..n-1 = on a recent
	todayTotal    time.Duration
	streak        int

	// overlays
	focusLabOpen     bool
	focusLabCursor   int // 0..numLabSliders-1
	inboxOpen        bool
	sleepOpen        bool
	sleepCursor      int // 0..3
	minimalist       bool
	pomodoroOn       bool
	pomodoroPhase    string // "focus" | "break"
	pomodoroEnd      time.Time
	pomodoroCycle    int
	sleepEnd         *time.Time

	// mouse hit regions on the playing screen
	regions clickRegions

	prog *tea.Program
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(tick(), m.spinner.Tick)
}

func tick() tea.Cmd {
	return tea.Tick(time.Second/30, func(t time.Time) tea.Msg { return tickMsg(t) })
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

	case tea.MouseMsg:
		return m.updateMouse(msg)

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
		m.endSession()
		return m, nil

	case sleepFiredMsg:
		// fade-and-stop is just a hard stop in v1; sleep timer ends the session.
		m.endSession()
		return m, nil

	case tickMsg:
		// Pomodoro / sleep timer firing.
		now := time.Now()
		if m.sess != nil && m.pomodoroOn && now.After(m.pomodoroEnd) {
			m.advancePomodoro()
		}
		if m.sleepEnd != nil && now.After(*m.sleepEnd) {
			m.sleepEnd = nil
			return m, func() tea.Msg { return sleepFiredMsg{} }
		}
		return m, tick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if m.screen == screenHome && !m.inboxOpen {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	if m.inboxOpen {
		var cmd tea.Cmd
		m.inboxInput, cmd = m.inboxInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

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

// ── Helpers ──────────────────────────────────────────────────────────────────

func (m *model) startLoadCmd(arg string) tea.Cmd {
	return func() tea.Msg {
		path, cleanup, err := source.Resolve(arg)
		if err != nil {
			return errMsg{err}
		}
		cfg := player.Config{
			Rate:         m.profile.Rate,
			Depth:        m.profile.Depth,
			Shape:        m.profile.Shape,
			ReverbOn:     m.profile.ReverbOn,
			ReverbMix:    m.profile.ReverbMix,
			NoiseMode:    m.profile.NoiseMode,
			NoiseVolume:  m.profile.NoiseVolume,
			BinauralOn:   m.profile.BinauralOn,
			BinauralBeat: m.profile.BinauralBeat,
			VolumeDb:     m.profile.VolumeDb,
		}
		sess, err := player.Start(path, cfg)
		if err != nil {
			cleanup()
			return errMsg{err}
		}
		sess.OnClose(cleanup)

		// Add to recents (best-effort).
		go func(p string) {
			title := titleFromPath(p)
			_ = config.AddRecent(config.RecentItem{
				Source: arg, // store original arg so YouTube re-resolves cleanly
				Title:  title,
				At:     time.Now(),
			})
		}(path)

		go func() {
			<-sess.Done
			m.prog.Send(finishedMsg{})
		}()
		return startedMsg{sess}
	}
}

func (m *model) teardown() {
	m.endSession()
}

// endSession closes the active session, logs the session, refreshes recents,
// and returns to the home screen.
func (m *model) endSession() {
	if m.sess != nil {
		_ = config.AppendSession(m.sess.StartedAt(), time.Now())
		m.sess.Close()
		m.sess = nil
	}
	m.screen = screenHome
	m.input.SetValue("")
	m.input.Focus()
	m.focusLabOpen = false
	m.inboxOpen = false
	m.sleepOpen = false
	m.sleepEnd = nil
	m.pomodoroOn = false
	m.recents = config.LoadRecents()
	m.todayTotal = config.TodayTotal()
	m.streak = config.Streak()
	m.recentsCursor = -1
}

// saveProfile persists current profile to disk; called when settings change.
func (m *model) saveProfile() {
	_ = config.SaveProfile(m.profile)
}
