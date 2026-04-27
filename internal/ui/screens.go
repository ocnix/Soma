package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"soma/internal/state"
)

// ── Header ───────────────────────────────────────────────────────────────────

func (m *model) header() string {
	left := titleStyle.Render("soma")
	mid := subStyle.Render("· 8D audio for focus ·")
	right := beanStyle.Render("☕  ") + subStyle.Render(themeByName(m.profile.Theme).Display)
	bar := fmt.Sprintf("%s   %s   %s", left, mid, right)
	return titleBarStyle.Width(m.width - 2).Render(bar)
}

// ── Home ─────────────────────────────────────────────────────────────────────

func (m *model) viewHome() string {
	header := m.header()

	logo := lipgloss.NewStyle().Foreground(colGold).Render(strings.TrimRight(somaLogo, "\n"))
	tagline := lipgloss.NewStyle().Foreground(colCream).Italic(true).
		Render(`"coffee is the modern soma — first drunk by sufis to focus through the night."`)

	prompt := labelStyle.Render("what do you want to spin?") + "\n" + m.input.View()

	recentsBlock := m.renderRecents()

	statsLine := m.renderStats()

	help := helpStyle.Render(
		keyStyle.Render("enter") + " play   " +
			keyStyle.Render("↑/↓") + " navigate   " +
			keyStyle.Render("?") + " random   " +
			keyStyle.Render("t") + " theme   " +
			keyStyle.Render("esc") + " quit",
	)

	var errLine string
	if m.err != nil {
		errLine = "\n" + errStyle.Render("× "+m.err.Error())
	}

	bodyParts := []string{
		"",
		logo,
		tagline,
		"",
		prompt,
		"",
	}
	if recentsBlock != "" {
		bodyParts = append(bodyParts, recentsBlock, "")
	}
	if statsLine != "" {
		bodyParts = append(bodyParts, statsLine, "")
	}
	bodyParts = append(bodyParts, help)

	body := lipgloss.JoinVertical(lipgloss.Center, bodyParts...) + errLine

	bodyW := m.width - 2
	bodyH := m.height - lipgloss.Height(header) - 2
	centered := lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Center, body)
	pane := paneStyle.Width(bodyW).Render(centered)

	if m.inboxOpen {
		return overlayInbox(lipgloss.JoinVertical(lipgloss.Left, header, pane), m)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, pane)
}

func (m *model) renderRecents() string {
	if len(m.recents) == 0 {
		return subStyle.Render("(no recents yet — your last 10 tracks will land here)")
	}
	maxShown := 5
	if len(m.recents) < maxShown {
		maxShown = len(m.recents)
	}
	var lines []string
	lines = append(lines, labelStyle.Render("recents"))
	for i := 0; i < maxShown; i++ {
		it := m.recents[i]
		title := it.Title
		if len(title) > 60 {
			title = title[:60] + "…"
		}
		marker := "  "
		style := subStyle
		if i == m.recentsCursor {
			marker = dotStyle.Render("▸ ")
			style = trackStyle
		}
		lines = append(lines, marker+style.Render(title))
	}
	return strings.Join(lines, "\n")
}

func (m *model) renderStats() string {
	if m.todayTotal == 0 && m.streak == 0 {
		return ""
	}
	parts := []string{}
	if m.todayTotal > 0 {
		mins := int(m.todayTotal.Minutes())
		parts = append(parts, fmt.Sprintf("today: %dm", mins))
	}
	if m.streak > 0 {
		parts = append(parts, fmt.Sprintf("streak: %dd", m.streak))
	}
	return subStyle.Render(strings.Join(parts, " · "))
}

// ── Loading ──────────────────────────────────────────────────────────────────

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

// ── Playing ──────────────────────────────────────────────────────────────────

func (m *model) viewPlaying() string {
	if m.sess == nil {
		return ""
	}
	s := m.sess.State.Snapshot()
	m.regions.reset()

	if m.minimalist {
		return m.viewMinimalist(s)
	}

	header := m.header()

	statusPane := m.renderStatusPane(s, m.width-2)
	progress := m.renderProgress(s, m.width-2, lipgloss.Height(header)+1+lipgloss.Height(statusPane))
	levels := m.renderLevels(s, m.width-2)
	titleLine := m.renderNowPlaying(s, m.width-2)

	used := lipgloss.Height(header) +
		lipgloss.Height(titleLine) +
		lipgloss.Height(levels) +
		lipgloss.Height(progress) +
		lipgloss.Height(statusPane) +
		2
	orbitH := m.height - used
	if orbitH < 6 {
		orbitH = 6
	}
	orbitW := minInt(m.width-4, orbitH*4)
	if orbitW < 16 {
		orbitW = 16
	}
	if m.focusLabOpen {
		orbitW = minInt(orbitW, m.width-44)
		if orbitW < 16 {
			orbitW = 16
		}
	}

	orb := renderViz(m.vizMode, s.Phase, s.LevelL, s.LevelR, orbitW, orbitH, s.Paused)
	orbCentered := lipgloss.Place(m.width-2, orbitH, lipgloss.Center, lipgloss.Center, orb)

	main := lipgloss.JoinVertical(lipgloss.Left,
		header,
		titleLine,
		orbCentered,
		levels,
		progress,
		statusPane,
	)

	if m.focusLabOpen {
		main = composeWithFocusLab(main, m, m.height)
	}
	if m.inboxOpen {
		main = overlayInbox(main, m)
	}
	if m.sleepOpen {
		main = overlaySleep(main, m)
	}
	return main
}

// Minimal view — visualizer + tiny status line only.
func (m *model) viewMinimalist(s state.Snapshot) string {
	w, h := m.width, m.height
	orb := renderViz(m.vizMode, s.Phase, s.LevelL, s.LevelR, w-4, h-4, s.Paused)
	orbCentered := lipgloss.Place(w, h-2, lipgloss.Center, lipgloss.Center, orb)
	footer := helpStyle.Render(
		fmtDur(s.Elapsed) + " / " + fmtDur(s.Duration) + "   " +
			keyStyle.Render("v") + " viz   " +
			keyStyle.Render("m") + " exit minimal   " +
			keyStyle.Render("esc") + " back",
	)
	footer = lipgloss.PlaceHorizontal(w, lipgloss.Center, footer)
	return orbCentered + "\n" + footer
}

// ── Sections ─────────────────────────────────────────────────────────────────

func (m *model) renderNowPlaying(s state.Snapshot, width int) string {
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

func (m *model) renderLevels(s state.Snapshot, width int) string {
	innerW := width - 8
	if innerW < 20 {
		innerW = 20
	}
	barW := innerW - 14
	if barW < 10 {
		barW = 10
	}
	left := subStyle.Render("L ") + vuBar(s.LevelL, barW) + "  " + dbLabel(s.LevelL)
	right := subStyle.Render("R ") + vuBar(s.LevelR, barW) + "  " + dbLabel(s.LevelR)
	body := left + "\n" + right
	return lipgloss.NewStyle().Padding(0, 2).Width(width).Render(body)
}

// renderProgress returns the progress bar AND records its hit region for mouse.
// rowHint is the absolute row this section is rendered at (used to register
// the click region).
func (m *model) renderProgress(s state.Snapshot, width int, rowHint int) string {
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
	frac = clamp(frac, 0, 1)
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

	// Compute the absolute X of the bar inside the pane.
	// "  " (left padding) + len(timeL) + "  " spacer.
	barX := 2 + len(timeL) + 2
	m.regions.progress = rect{x: barX, y: rowHint, w: barW, h: 1, enabled: true}

	body := statStyle.Render(timeL) + "  " + sb.String() + "  " + statStyle.Render(timeR)
	return lipgloss.NewStyle().Padding(0, 2).Width(width).Render(body)
}

func (m *model) renderStatusPane(s state.Snapshot, width int) string {
	mode := s.Mode
	if mode == "" {
		mode = "8D"
	}
	modeChip := chipText(" 8D ", true)
	if mode == "dry" {
		modeChip = chipText(" dry ", false)
	}
	reverbChip := chipText(" reverb off ", false)
	if s.ReverbOn {
		reverbChip = chipText(" reverb on ", true)
	}
	noiseChip := chipText(" noise off ", false)
	if s.NoiseMode != "" && s.NoiseMode != "off" {
		noiseChip = chipText(" "+s.NoiseMode+" noise ", true)
	}
	binChip := chipText(" binaural off ", false)
	if s.BinauralOn {
		binChip = chipText(fmt.Sprintf(" binaural %.0fHz ", s.BinauralBeat), true)
	}
	pauseChip := chipText(" ▸ playing ", true)
	if s.Paused {
		pauseChip = pauseStyle.Render(" ‖ PAUSED ")
	}

	values := fmt.Sprintf("%s %s    %s %s    %s %s    %s %s",
		labelStyle.Render("vol"), statStyle.Render(fmtDb(s.VolumeDb)),
		labelStyle.Render("rate"), statStyle.Render(fmt.Sprintf("%.2f Hz", s.RateHz)),
		labelStyle.Render("depth"), statStyle.Render(fmt.Sprintf("%.0f%%", s.Depth*100)),
		labelStyle.Render("shape"), statStyle.Render(s.Shape),
	)

	chips := pauseChip + "  " + modeChip + " " + reverbChip + " " + noiseChip + " " + binChip

	timersLine := m.renderTimerLine()

	help1 := helpStyle.Render(
		keyStyle.Render("space") + " pause   " +
			keyStyle.Render("←/→") + " seek   " +
			keyStyle.Render("↑/↓") + " vol   " +
			keyStyle.Render("[/]") + " rate   " +
			keyStyle.Render("S") + " shape   " +
			keyStyle.Render("d") + " 8D   " +
			keyStyle.Render("r") + " reverb",
	)
	help2 := helpStyle.Render(
		keyStyle.Render("n") + " noise   " +
			keyStyle.Render("N") + " noise+vol   " +
			keyStyle.Render("b") + " binaural   " +
			keyStyle.Render("f") + " focus lab   " +
			keyStyle.Render("g") + " inbox   " +
			keyStyle.Render("s") + " sleep   " +
			keyStyle.Render("p") + " pomodoro   " +
			keyStyle.Render("m") + " min",
	)
	help3 := helpStyle.Render(
		keyStyle.Render("v") + " viz (" + m.vizMode.Name() + ")   " +
			keyStyle.Render("t") + " theme   " +
			keyStyle.Render("esc") + " back   " +
			keyStyle.Render("q") + " quit",
	)

	parts := []string{values, "", chips}
	if timersLine != "" {
		parts = append(parts, timersLine)
	}
	parts = append(parts, "", help1, help2, help3)
	body := strings.Join(parts, "\n")

	return paneStyle.Width(width).Render(body)
}

func chipText(text string, on bool) string {
	if on {
		return onStyle.Render(text)
	}
	return offStyle.Render(text)
}

func (m *model) renderTimerLine() string {
	parts := []string{}
	if m.sleepEnd != nil {
		left := time.Until(*m.sleepEnd)
		if left < 0 {
			left = 0
		}
		parts = append(parts, dotStyle.Render("⏱  ")+statStyle.Render("sleep in "+fmtDur(left)))
	}
	if m.pomodoroOn {
		left := time.Until(m.pomodoroEnd)
		if left < 0 {
			left = 0
		}
		phase := m.pomodoroPhase
		parts = append(parts, dotStyle.Render("◷  ")+
			statStyle.Render(fmt.Sprintf("pomodoro %s · %s · cycle %d", phase, fmtDur(left), m.pomodoroCycle)))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "    ")
}
