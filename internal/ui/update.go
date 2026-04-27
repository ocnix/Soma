package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"soma/internal/config"
)

// ── Home ─────────────────────────────────────────────────────────────────────

func (m *model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.inboxOpen {
		return m.handleInbox(msg)
	}

	switch msg.String() {
	case "esc":
		m.teardown()
		return m, tea.Quit
	case "ctrl+q":
		m.teardown()
		return m, tea.Quit
	case "t":
		m.cycleTheme()
		return m, nil
	case "?":
		// Random play from recents.
		if it, ok := config.RandomRecent(); ok {
			return m.startLoadingArg(it.Source)
		}
		return m, nil
	case "down":
		if len(m.recents) > 0 {
			if m.recentsCursor < 0 {
				m.recentsCursor = 0
			} else if m.recentsCursor < len(m.recents)-1 {
				m.recentsCursor++
			}
			m.input.Blur()
		}
		return m, nil
	case "up":
		if m.recentsCursor >= 0 {
			m.recentsCursor--
			if m.recentsCursor < 0 {
				m.recentsCursor = -1
				m.input.Focus()
			}
		}
		return m, nil
	case "enter":
		if m.recentsCursor >= 0 && m.recentsCursor < len(m.recents) {
			return m.startLoadingArg(m.recents[m.recentsCursor].Source)
		}
		val := strings.TrimSpace(m.input.Value())
		if val == "" {
			return m, nil
		}
		return m.startLoadingArg(val)
	}

	if m.recentsCursor < 0 {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) startLoadingArg(arg string) (tea.Model, tea.Cmd) {
	m.loadingArg = arg
	m.screen = screenLoading
	m.err = nil
	return m, tea.Batch(m.startLoadCmd(arg), m.spinner.Tick)
}

func (m *model) cycleTheme() {
	t := nextTheme(m.profile.Theme)
	m.profile.Theme = t.Name
	applyTheme(t)
	m.saveProfile()
}

// ── Playing ──────────────────────────────────────────────────────────────────

func (m *model) updatePlaying(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.inboxOpen {
		return m.handleInbox(msg)
	}
	if m.sleepOpen {
		return m.handleSleepPicker(msg)
	}
	if m.focusLabOpen {
		// Focus Lab eats most keys; allow `f`/esc to close, `q` to quit.
		switch msg.String() {
		case "f", "esc":
			m.focusLabOpen = false
			return m, nil
		case "q":
			m.teardown()
			return m, tea.Quit
		}
		return m.handleFocusLab(msg)
	}

	switch msg.String() {
	case "q":
		m.teardown()
		return m, tea.Quit
	case "esc":
		m.endSession()
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
			m.profile.VolumeDb = m.sess.State.Snapshot().VolumeDb
			m.saveProfile()
		}
	case "down":
		if m.sess != nil {
			m.sess.AdjustVolume(-2)
			m.profile.VolumeDb = m.sess.State.Snapshot().VolumeDb
			m.saveProfile()
		}
	case "[":
		if m.sess != nil {
			m.sess.AdjustRate(-0.05)
			m.profile.Rate = m.sess.State.Snapshot().RateHz
			m.saveProfile()
		}
	case "]":
		if m.sess != nil {
			m.sess.AdjustRate(0.05)
			m.profile.Rate = m.sess.State.Snapshot().RateHz
			m.saveProfile()
		}
	case "r":
		if m.sess != nil {
			m.sess.ToggleReverb()
			m.profile.ReverbOn = m.sess.State.Snapshot().ReverbOn
			m.saveProfile()
		}
	case "d":
		if m.sess != nil {
			m.sess.ToggleEffect()
		}
	case "n":
		if m.sess != nil {
			m.sess.CycleNoise()
			m.profile.NoiseMode = m.sess.State.Snapshot().NoiseMode
			m.saveProfile()
		}
	case "N":
		if m.sess != nil {
			m.sess.AdjustNoiseVolume(0.05)
			m.profile.NoiseVolume = m.sess.State.Snapshot().NoiseVolume
			m.saveProfile()
		}
	case "b":
		if m.sess != nil {
			m.sess.ToggleBinaural()
			m.profile.BinauralOn = m.sess.State.Snapshot().BinauralOn
			m.saveProfile()
		}
	case "f":
		m.focusLabOpen = !m.focusLabOpen
		m.focusLabCursor = 0
	case "g":
		m.openInbox()
	case "s":
		m.sleepOpen = true
		m.sleepCursor = 0
	case "p":
		m.togglePomodoro()
	case "m":
		m.minimalist = !m.minimalist
	case "t":
		m.cycleTheme()
	case "S":
		// shape toggle (sine ↔ random walk)
		if m.sess != nil {
			cur := m.sess.State.Snapshot().Shape
			next := "sine"
			if cur == "sine" {
				next = "random"
			}
			m.sess.SetShape(next)
			m.profile.Shape = next
			m.saveProfile()
		}
	}
	return m, nil
}

// ── Mouse ────────────────────────────────────────────────────────────────────

func (m *model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.screen != screenPlaying || m.sess == nil || m.focusLabOpen || m.inboxOpen || m.sleepOpen {
		return m, nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.sess.AdjustVolume(2)
		m.profile.VolumeDb = m.sess.State.Snapshot().VolumeDb
		m.saveProfile()
		return m, nil
	case tea.MouseButtonWheelDown:
		m.sess.AdjustVolume(-2)
		m.profile.VolumeDb = m.sess.State.Snapshot().VolumeDb
		m.saveProfile()
		return m, nil
	}

	// Only react to actual clicks (release events).
	if msg.Action != tea.MouseActionRelease {
		return m, nil
	}
	x, y := msg.X, msg.Y

	// Click on progress bar → seek.
	if m.regions.progress.contains(x, y) {
		f := m.regions.progress.fraction(x)
		m.sess.SeekFraction(f)
		return m, nil
	}
	// Click on chips.
	if m.regions.chip8D.contains(x, y) {
		m.sess.ToggleEffect()
	}
	if m.regions.chipReverb.contains(x, y) {
		m.sess.ToggleReverb()
		m.profile.ReverbOn = m.sess.State.Snapshot().ReverbOn
		m.saveProfile()
	}
	if m.regions.chipNoise.contains(x, y) {
		m.sess.CycleNoise()
		m.profile.NoiseMode = m.sess.State.Snapshot().NoiseMode
		m.saveProfile()
	}
	if m.regions.chipBinaural.contains(x, y) {
		m.sess.ToggleBinaural()
		m.profile.BinauralOn = m.sess.State.Snapshot().BinauralOn
		m.saveProfile()
	}
	if m.regions.chipPause.contains(x, y) {
		m.sess.TogglePause()
	}
	return m, nil
}

// ── Inbox ────────────────────────────────────────────────────────────────────

func (m *model) openInbox() {
	m.inboxOpen = true
	m.inboxInput.SetValue("")
	m.inboxInput.Focus()
}

func (m *model) handleInbox(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inboxOpen = false
		m.inboxInput.Blur()
		return m, nil
	case "enter":
		val := strings.TrimSpace(m.inboxInput.Value())
		if val != "" {
			_ = config.AppendInbox(val)
		}
		m.inboxOpen = false
		m.inboxInput.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.inboxInput, cmd = m.inboxInput.Update(msg)
	return m, cmd
}

// ── Sleep timer picker ───────────────────────────────────────────────────────

var sleepOptions = []struct {
	Label string
	Dur   time.Duration
}{
	{"25 minutes", 25 * time.Minute},
	{"50 minutes", 50 * time.Minute},
	{"90 minutes", 90 * time.Minute},
	{"cancel timer", 0},
}

func (m *model) handleSleepPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.sleepOpen = false
	case "up":
		if m.sleepCursor > 0 {
			m.sleepCursor--
		}
	case "down":
		if m.sleepCursor < len(sleepOptions)-1 {
			m.sleepCursor++
		}
	case "enter":
		opt := sleepOptions[m.sleepCursor]
		if opt.Dur == 0 {
			m.sleepEnd = nil
		} else {
			t := time.Now().Add(opt.Dur)
			m.sleepEnd = &t
		}
		m.sleepOpen = false
	}
	return m, nil
}

// ── Pomodoro ─────────────────────────────────────────────────────────────────

const (
	pomodoroFocus = 25 * time.Minute
	pomodoroBreak = 5 * time.Minute
)

func (m *model) togglePomodoro() {
	if m.pomodoroOn {
		m.pomodoroOn = false
		return
	}
	m.pomodoroOn = true
	m.pomodoroPhase = "focus"
	m.pomodoroEnd = time.Now().Add(pomodoroFocus)
	m.pomodoroCycle = 1
}

func (m *model) advancePomodoro() {
	if m.pomodoroPhase == "focus" {
		m.pomodoroPhase = "break"
		m.pomodoroEnd = time.Now().Add(pomodoroBreak)
		if m.sess != nil {
			m.sess.SetPaused(true)
		}
	} else {
		m.pomodoroPhase = "focus"
		m.pomodoroEnd = time.Now().Add(pomodoroFocus)
		m.pomodoroCycle++
		if m.sess != nil {
			m.sess.SetPaused(false)
		}
	}
}

// ── Focus Lab (sliders) ──────────────────────────────────────────────────────

type labParam struct {
	Name string
	Get  func(*model) float64
	Set  func(*model, float64)
	Min  float64
	Max  float64
	Step float64
	Unit string
}

func (m *model) labParams() []labParam {
	return []labParam{
		{
			Name: "rate",
			Get:  func(m *model) float64 { return m.profile.Rate },
			Set: func(m *model, v float64) {
				m.profile.Rate = v
				if m.sess != nil {
					m.sess.SetRate(v)
				}
			},
			Min: 0.02, Max: 1.0, Step: 0.02, Unit: "Hz",
		},
		{
			Name: "depth",
			Get:  func(m *model) float64 { return m.profile.Depth },
			Set: func(m *model, v float64) {
				m.profile.Depth = v
				if m.sess != nil {
					m.sess.SetDepth(v)
				}
			},
			Min: 0, Max: 1, Step: 0.05, Unit: "",
		},
		{
			Name: "reverb mix",
			Get:  func(m *model) float64 { return m.profile.ReverbMix },
			Set: func(m *model, v float64) {
				m.profile.ReverbMix = v
				if m.sess != nil {
					m.sess.SetReverbMix(v)
				}
			},
			Min: 0, Max: 1, Step: 0.05, Unit: "",
		},
		{
			Name: "noise volume",
			Get:  func(m *model) float64 { return m.profile.NoiseVolume },
			Set: func(m *model, v float64) {
				m.profile.NoiseVolume = v
				if m.sess != nil {
					m.sess.SetNoiseVolume(v)
				}
			},
			Min: 0, Max: 1, Step: 0.05, Unit: "",
		},
		{
			Name: "binaural beat",
			Get:  func(m *model) float64 { return m.profile.BinauralBeat / 40 },
			Set: func(m *model, v float64) {
				hz := v * 40
				if hz < 1 {
					hz = 1
				}
				m.profile.BinauralBeat = hz
				if m.sess != nil {
					m.sess.SetBinauralBeat(hz)
				}
			},
			Min: 0, Max: 1, Step: 0.05, Unit: "Hz",
		},
	}
}

func (m *model) handleFocusLab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	params := m.labParams()
	switch msg.String() {
	case "up":
		if m.focusLabCursor > 0 {
			m.focusLabCursor--
		}
	case "down":
		if m.focusLabCursor < len(params)-1 {
			m.focusLabCursor++
		}
	case "left", "h", "-":
		m.adjustLab(-1)
	case "right", "l", "+":
		m.adjustLab(+1)
	case "1":
		m.applyPreset("calm")
	case "2":
		m.applyPreset("focus")
	case "3":
		m.applyPreset("stim")
	case "4":
		m.applyPreset("random")
	}
	return m, nil
}

func (m *model) adjustLab(dir int) {
	params := m.labParams()
	p := params[m.focusLabCursor]
	cur := p.Get(m)
	next := cur + float64(dir)*p.Step
	if next < p.Min {
		next = p.Min
	}
	if next > p.Max {
		next = p.Max
	}
	p.Set(m, next)
	m.saveProfile()
}

// ── Presets ──────────────────────────────────────────────────────────────────

func (m *model) applyPreset(name string) {
	switch name {
	case "calm":
		m.profile.Rate = 0.08
		m.profile.Depth = 0.6
		m.profile.Shape = "sine"
		m.profile.ReverbOn = true
		m.profile.ReverbMix = 0.30
	case "focus":
		m.profile.Rate = 0.15
		m.profile.Depth = 0.85
		m.profile.Shape = "sine"
		m.profile.ReverbOn = true
		m.profile.ReverbMix = 0.18
	case "stim":
		m.profile.Rate = 0.30
		m.profile.Depth = 1.0
		m.profile.Shape = "sine"
		m.profile.ReverbOn = false
		m.profile.ReverbMix = 0.10
	case "random":
		m.profile.Rate = 0.18
		m.profile.Depth = 0.9
		m.profile.Shape = "random"
		m.profile.ReverbOn = true
		m.profile.ReverbMix = 0.22
	}
	if m.sess != nil {
		m.sess.SetRate(m.profile.Rate)
		m.sess.SetDepth(m.profile.Depth)
		m.sess.SetShape(m.profile.Shape)
		m.sess.SetReverbMix(m.profile.ReverbMix)
		// reverb on/off via toggle if mismatched
		curOn := m.sess.State.Snapshot().ReverbOn
		if curOn != m.profile.ReverbOn {
			m.sess.ToggleReverb()
		}
	}
	m.saveProfile()
}

// titleFromPath strips directories and extension; used for recents.
func titleFromPath(path string) string {
	base := path
	if i := strings.LastIndexAny(path, "/\\"); i >= 0 {
		base = path[i+1:]
	}
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	return base
}

