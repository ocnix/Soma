package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ── Orbital visualizer ───────────────────────────────────────────────────────

func orbitalView(phase float64, w, h int, paused bool) string {
	if w < 12 {
		w = 12
	}
	if h < 6 {
		h = 6
	}
	grid := make([][]rune, h)
	style := make([][]int, h) // 0 = blank, 1 = rail, 2 = dot, 3 = center, 4 = pause
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

	for t := 0.0; t < 2*math.Pi; t += 0.04 {
		x := cx + rx*math.Sin(t)
		y := cy - ry*math.Cos(t)
		ix, iy := int(math.Round(x)), int(math.Round(y))
		if ix >= 0 && ix < w && iy >= 0 && iy < h && grid[iy][ix] == ' ' {
			grid[iy][ix] = '·'
			style[iy][ix] = 1
		}
	}

	icx, icy := int(math.Round(cx)), int(math.Round(cy))
	if icx >= 0 && icx < w && icy >= 0 && icy < h {
		grid[icy][icx] = '◌'
		style[icy][icx] = 3
	}

	for k := 1; k <= 6; k++ {
		t := phase - float64(k)*0.12
		x := cx + rx*math.Sin(t)
		y := cy - ry*math.Cos(t)
		ix, iy := int(math.Round(x)), int(math.Round(y))
		if ix >= 0 && ix < w && iy >= 0 && iy < h {
			if style[iy][ix] != 2 {
				grid[iy][ix] = '∙'
				style[iy][ix] = 1
			}
		}
	}

	dx := cx + rx*math.Sin(phase)
	dy := cy - ry*math.Cos(phase)
	ix, iy := int(math.Round(dx)), int(math.Round(dy))
	if ix >= 0 && ix < w && iy >= 0 && iy < h {
		grid[iy][ix] = '●'
		style[iy][ix] = 2
	}

	// Big pause overlay: render two large vertical bars centered on the head.
	if paused {
		pauseGlyphs := []rune{'┃', '┃'}
		for di := -1; di <= 1; di++ {
			row := icy + di
			if row < 0 || row >= h {
				continue
			}
			for k, g := range pauseGlyphs {
				col := icx + (k*2 - 1)
				if col < 0 || col >= w {
					continue
				}
				grid[row][col] = g
				style[row][col] = 4
			}
		}
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
			case 4:
				sb.WriteString(pauseStyle.Render(string(r)))
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

// ── Bars and labels ──────────────────────────────────────────────────────────

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

// horizSlider draws a 0..1 fill bar with a ● at the current position.
func horizSlider(value float64, width int) string {
	if width < 4 {
		width = 4
	}
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	pos := int(value * float64(width-1))
	var sb strings.Builder
	for i := 0; i < width; i++ {
		switch {
		case i == pos:
			sb.WriteString(dotStyle.Render("●"))
		case i < pos:
			sb.WriteString(lipgloss.NewStyle().Foreground(colGold).Render("─"))
		default:
			sb.WriteString(railStyle.Render("─"))
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
