package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"soma/internal/library"
)

const libraryRowsMax = 200

// openLibrary switches to the library screen, kicks off a scan if no cache.
func (m *model) openLibrary() (tea.Model, tea.Cmd) {
	m.screen = screenLibrary
	m.libQuery = ""
	m.libCursor = 0
	m.libQueryFocus = true
	m.recentsCursor = -1
	m.input.Blur()

	if m.lib.IsEmpty() && !m.libScanning {
		m.libScanning = true
		return m, m.scanLibraryCmd()
	}
	return m, nil
}

func (m *model) scanLibraryCmd() tea.Cmd {
	idx := m.lib
	return func() tea.Msg {
		_ = idx.Scan()
		return libraryScannedMsg{count: len(idx.Items())}
	}
}

func (m *model) updateLibrary(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenHome
		m.input.Focus()
		return m, nil
	case "q":
		m.teardown()
		return m, tea.Quit
	case "tab":
		m.libQueryFocus = !m.libQueryFocus
		return m, nil
	case "down":
		filtered := m.libVisible()
		if m.libCursor < len(filtered)-1 {
			m.libCursor++
		}
		m.libQueryFocus = false
		return m, nil
	case "up":
		if m.libCursor > 0 {
			m.libCursor--
		}
		if m.libCursor == 0 {
			m.libQueryFocus = true
		}
		return m, nil
	case "ctrl+r":
		if !m.libScanning {
			m.libScanning = true
			return m, m.scanLibraryCmd()
		}
		return m, nil
	case "enter":
		filtered := m.libVisible()
		if m.libCursor >= 0 && m.libCursor < len(filtered) {
			return m.startLoadingArg(filtered[m.libCursor].Path)
		}
		return m, nil
	case "backspace":
		if len(m.libQuery) > 0 {
			m.libQuery = m.libQuery[:len(m.libQuery)-1]
			m.libCursor = 0
		}
		return m, nil
	}

	// Otherwise, accept printable runes as query input.
	if k := msg.String(); len(k) == 1 && k[0] >= 32 && k[0] < 127 {
		m.libQuery += k
		m.libCursor = 0
		m.libQueryFocus = true
	}
	return m, nil
}

func (m *model) libVisible() []library.Item {
	return m.lib.Filter(m.libQuery, libraryRowsMax)
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m *model) viewLibrary() string {
	header := m.header()

	bodyW := m.width - 2
	bodyH := m.height - lipgloss.Height(header) - 2

	titleLine := titleStyle.Render("📁  library") + "  " + subStyle.Render(m.lib.Root())

	queryPrompt := "search ▸ "
	queryDisplay := m.libQuery
	if m.libQueryFocus {
		queryDisplay += "▏" // cursor
	}
	queryStyle := lipgloss.NewStyle().Foreground(colCream).Underline(true)
	queryLine := labelStyle.Render(queryPrompt) + queryStyle.Render(queryDisplay+strings.Repeat(" ", maxInt(40-len(queryDisplay), 0)))

	var listBody string
	if m.libScanning {
		listBody = m.spinner.View() + "  " + subStyle.Render("scanning "+m.lib.Root()+" …")
	} else if m.lib.IsEmpty() {
		listBody = subStyle.Render("(no audio found in " + m.lib.Root() + " — drop some mp3 / wav / flac files in there, then ctrl+r to rescan)")
	} else {
		filtered := m.libVisible()
		total := len(m.lib.Items())
		showing := len(filtered)

		var rows []string
		// Compute a visible window around the cursor.
		visibleRows := bodyH - 8
		if visibleRows < 5 {
			visibleRows = 5
		}
		start := 0
		if m.libCursor >= visibleRows {
			start = m.libCursor - visibleRows + 1
		}
		end := start + visibleRows
		if end > showing {
			end = showing
		}

		for i := start; i < end; i++ {
			it := filtered[i]
			line := it.Display()
			if len(line) > bodyW-6 {
				line = line[:bodyW-6] + "…"
			}
			marker := "  "
			style := subStyle
			if i == m.libCursor {
				marker = dotStyle.Render("▸ ")
				style = trackStyle
			}
			rows = append(rows, marker+style.Render(line))
		}
		if len(rows) == 0 {
			rows = append(rows, subStyle.Render("  (no matches)"))
		}
		footer := subStyle.Render(fmt.Sprintf("%d / %d", showing, total))
		listBody = strings.Join(rows, "\n") + "\n\n" + footer
	}

	help := helpStyle.Render(
		keyStyle.Render("type") + " filter   " +
			keyStyle.Render("↑/↓") + " select   " +
			keyStyle.Render("enter") + " play   " +
			keyStyle.Render("ctrl+r") + " rescan   " +
			keyStyle.Render("esc") + " back",
	)

	body := strings.Join([]string{
		titleLine,
		"",
		queryLine,
		"",
		listBody,
		"",
		help,
	}, "\n")

	pane := paneStyle.Width(bodyW).Render(body)
	if lipgloss.Height(pane) > bodyH {
		// fit guard — should rarely trigger
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, pane)
}
