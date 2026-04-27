package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// overlayCenter returns the base view with the modal centered on top. Bubble
// Tea doesn't have true overlays — we cheat by replacing the screen with a
// dim version of the base + the modal floating in the middle. For simplicity
// we just show the modal full-size in the center of the screen, dimming the
// base content visually.
func overlayCenter(base, modal string, width, height int) string {
	// Dim base by re-rendering through the dim style. Lipgloss can't easily
	// re-style ANSI strings, so we just place the modal over a faint
	// background area drawn from the base.
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, modal)
}

// ── Inbox modal ──────────────────────────────────────────────────────────────

func overlayInbox(base string, m *model) string {
	title := titleStyle.Render("✎  distraction inbox")
	help := helpStyle.Render(keyStyle.Render("enter") + " save   " + keyStyle.Render("esc") + " cancel")
	body := strings.Join([]string{
		title,
		"",
		subStyle.Render("jot the thought, get back to focus."),
		"",
		m.inboxInput.View(),
		"",
		help,
	}, "\n")

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colGold).
		Padding(1, 3).
		Width(64).
		Render(body)

	return overlayCenter(base, modal, m.width, m.height)
}

// ── Sleep timer modal ────────────────────────────────────────────────────────

func overlaySleep(base string, m *model) string {
	title := titleStyle.Render("⏱  sleep timer")
	var rows []string
	rows = append(rows, title, "")
	for i, opt := range sleepOptions {
		marker := "  "
		style := subStyle
		if i == m.sleepCursor {
			marker = dotStyle.Render("▸ ")
			style = trackStyle
		}
		rows = append(rows, marker+style.Render(opt.Label))
	}
	rows = append(rows, "", helpStyle.Render(
		keyStyle.Render("↑/↓")+" select   "+keyStyle.Render("enter")+" set   "+keyStyle.Render("esc")+" cancel",
	))

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colGold).
		Padding(1, 3).
		Width(48).
		Render(strings.Join(rows, "\n"))

	return overlayCenter(base, modal, m.width, m.height)
}

// ── Focus Lab ────────────────────────────────────────────────────────────────

func composeWithFocusLab(base string, m *model, height int) string {
	lab := m.renderFocusLab()
	return overlayCenter(base, lab, m.width, m.height)
}

func (m *model) renderFocusLab() string {
	header := titleStyle.Render("🧪  focus lab")
	subhead := subStyle.Render("personalize the 8D feel for your brain")

	// Presets row.
	presets := []string{"calm", "focus", "stim", "random walk"}
	keys := []string{"1", "2", "3", "4"}
	var pBuf strings.Builder
	pBuf.WriteString(labelStyle.Render("presets") + "\n  ")
	for i, p := range presets {
		pBuf.WriteString(keyStyle.Render("["+keys[i]+"] ") + statStyle.Render(p))
		if i < len(presets)-1 {
			pBuf.WriteString("   ")
		}
	}

	// Sliders.
	params := m.labParams()
	barW := 28
	var sBuf strings.Builder
	sBuf.WriteString(labelStyle.Render("sliders") + "\n")
	for i, p := range params {
		val := p.Get(m)
		frac := (val - p.Min) / (p.Max - p.Min)
		marker := "   "
		nameStyle := subStyle
		if i == m.focusLabCursor {
			marker = dotStyle.Render(" ▸ ")
			nameStyle = trackStyle
		}
		// raw value display
		valDisp := fmt.Sprintf("%.2f", val)
		if p.Name == "binaural beat" {
			valDisp = fmt.Sprintf("%.0f", val*40) // un-normalize
		}
		if p.Unit != "" {
			valDisp += " " + p.Unit
		}
		sBuf.WriteString(fmt.Sprintf("%s%s\n      %s   %s\n",
			marker, nameStyle.Render(p.Name),
			horizSlider(frac, barW), subStyle.Render(valDisp)))
	}

	help := helpStyle.Render(
		keyStyle.Render("↑/↓") + " param   " +
			keyStyle.Render("←/→ or +/-") + " adjust   " +
			keyStyle.Render("1-4") + " preset   " +
			keyStyle.Render("f or esc") + " close",
	)

	body := strings.Join([]string{
		header, subhead,
		"",
		pBuf.String(),
		"",
		sBuf.String(),
		help,
	}, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colGold).
		Padding(1, 3).
		Width(56).
		Render(body)
}
