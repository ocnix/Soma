package ui

import (
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// VizMode selects the playing-screen visualizer. `v` cycles through them.
type VizMode int

const (
	VizSphere VizMode = iota
	VizOrbit
	VizLissajous
	VizTunnel
	VizSpectrum
	VizZen
	VizFractal
	VizLife
)

var vizCycle = []VizMode{VizSphere, VizSpectrum, VizFractal, VizLife, VizZen, VizLissajous, VizTunnel, VizOrbit}

func (v VizMode) Name() string {
	switch v {
	case VizSphere:
		return "sphere"
	case VizLissajous:
		return "lissajous"
	case VizTunnel:
		return "tunnel"
	case VizSpectrum:
		return "spectrum"
	case VizZen:
		return "zen"
	case VizFractal:
		return "fractal"
	case VizLife:
		return "life"
	default:
		return "orbit"
	}
}

func nextViz(cur VizMode) VizMode {
	for i, v := range vizCycle {
		if v == cur {
			return vizCycle[(i+1)%len(vizCycle)]
		}
	}
	return VizSphere
}

func vizFromString(s string) VizMode {
	switch s {
	case "sphere":
		return VizSphere
	case "lissajous":
		return VizLissajous
	case "tunnel":
		return VizTunnel
	case "spectrum":
		return VizSpectrum
	case "zen":
		return VizZen
	case "fractal":
		return VizFractal
	case "life":
		return VizLife
	case "orbit":
		return VizOrbit
	}
	return VizSphere
}

// renderViz routes to the right visualizer.
func renderViz(mode VizMode, phase, levelL, levelR float64, bands []float64, w, h int, paused bool) string {
	switch mode {
	case VizSphere:
		return sphereView(phase, levelL, levelR, w, h, paused)
	case VizLissajous:
		return lissajousView(phase, levelL, levelR, w, h, paused)
	case VizTunnel:
		return tunnelView(phase, levelL, levelR, w, h, paused)
	case VizSpectrum:
		return spectrumView(phase, bands, levelL, levelR, w, h, paused)
	case VizZen:
		return zenView(phase, levelL, levelR, w, h, paused)
	case VizFractal:
		return fractalView(phase, bands, levelL, levelR, w, h, paused)
	case VizLife:
		return lifeView(phase, bands, levelL, levelR, w, h, paused)
	default:
		return orbitalView(phase, w, h, paused)
	}
}

// ── Canvas primitives ────────────────────────────────────────────────────────

type canvas struct {
	w, h   int
	grid   [][]rune
	style  [][]int // 0 blank, 1 dim, 2 mid, 3 bright, 4 head, 5 dot, 6 trail, 7 pause
	zbuf   [][]float64
}

func newCanvas(w, h int) *canvas {
	if w < 8 {
		w = 8
	}
	if h < 4 {
		h = 4
	}
	c := &canvas{w: w, h: h}
	c.grid = make([][]rune, h)
	c.style = make([][]int, h)
	c.zbuf = make([][]float64, h)
	for i := 0; i < h; i++ {
		c.grid[i] = make([]rune, w)
		c.style[i] = make([]int, w)
		c.zbuf[i] = make([]float64, w)
		for j := 0; j < w; j++ {
			c.grid[i][j] = ' '
			c.zbuf[i][j] = -math.Inf(1)
		}
	}
	return c
}

// putZ sets a cell only if z is closer than what's already there.
func (c *canvas) putZ(x, y int, z float64, r rune, style int) {
	if x < 0 || x >= c.w || y < 0 || y >= c.h {
		return
	}
	if z > c.zbuf[y][x] {
		c.zbuf[y][x] = z
		c.grid[y][x] = r
		c.style[y][x] = style
	}
}

// put sets a cell unconditionally (use for foreground sprites).
func (c *canvas) put(x, y int, r rune, style int) {
	if x < 0 || x >= c.w || y < 0 || y >= c.h {
		return
	}
	c.grid[y][x] = r
	c.style[y][x] = style
}

func (c *canvas) render() string {
	var sb strings.Builder
	for i := 0; i < c.h; i++ {
		for j := 0; j < c.w; j++ {
			r := c.grid[i][j]
			switch c.style[i][j] {
			case 1:
				sb.WriteString(bgStyle.Render(string(r)))
			case 2:
				sb.WriteString(railStyle.Render(string(r)))
			case 3:
				sb.WriteString(lipgloss.NewStyle().Foreground(colCream).Render(string(r)))
			case 4:
				sb.WriteString(beanStyle.Render(string(r)))
			case 5:
				sb.WriteString(dotStyle.Render(string(r)))
			case 6:
				sb.WriteString(railStyle.Render(string(r)))
			case 7:
				sb.WriteString(pauseStyle.Render(string(r)))
			default:
				sb.WriteRune(r)
			}
		}
		if i < c.h-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// ── Stars (shared background) ────────────────────────────────────────────────

// drawStars sprinkles a deterministic starfield twinkling per frame.
func drawStars(c *canvas, t float64) {
	r := rand.New(rand.NewSource(42))
	density := (c.w * c.h) / 80
	if density < 12 {
		density = 12
	}
	chars := []rune{'.', '·', '∙', '*'}
	for i := 0; i < density; i++ {
		x := r.Intn(c.w)
		y := r.Intn(c.h)
		// Per-star twinkling phase.
		ph := r.Float64() * 6.28
		alpha := 0.5 + 0.5*math.Sin(t*0.5+ph)
		idx := int(alpha * float64(len(chars)-1))
		if idx < 0 {
			idx = 0
		}
		// Don't overwrite the foreground.
		if c.grid[y][x] == ' ' {
			c.put(x, y, chars[idx], 1)
		}
	}
}

// ── Sphere ───────────────────────────────────────────────────────────────────

// sphereView is the showpiece: a wireframe sphere of stars, slowly rotating,
// with the sound source orbiting on a tilted ring. Depth-shaded; star
// background; bloom rings pulsing at the center with audio level.
func sphereView(phase, lvlL, lvlR float64, w, h int, paused bool) string {
	c := newCanvas(w, h)

	// Cell aspect — terminals are ~2:1 (taller than wide). Compensate.
	const aspectY = 0.5

	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	radius := math.Min(float64(w)/2.4, float64(h)/(2*aspectY)*0.85)

	// Background twinkling stars.
	drawStars(c, phase)

	// Slow yaw rotation so the sphere has visible depth motion.
	yaw := phase * 0.4
	cosY, sinY := math.Cos(yaw), math.Sin(yaw)

	// Sphere wireframe — sample lat/lon densely.
	shade := []rune{'·', ':', '∶', '∷', '∴', '✦'}

	for theta := -math.Pi / 2; theta <= math.Pi/2+0.001; theta += 0.18 {
		for phi := 0.0; phi < 2*math.Pi; phi += 0.07 {
			x := math.Cos(theta) * math.Sin(phi)
			y := math.Sin(theta)
			z := math.Cos(theta) * math.Cos(phi)
			// Rotate around Y.
			xr := x*cosY + z*sinY
			zr := -x*sinY + z*cosY
			yr := y

			sx := int(math.Round(cx + xr*radius))
			sy := int(math.Round(cy - yr*radius*aspectY))

			// Depth shade: closer (zr near 1) brighter.
			t := (zr + 1) / 2
			idx := int(t * float64(len(shade)-1))
			if idx < 0 {
				idx = 0
			}
			if idx >= len(shade) {
				idx = len(shade) - 1
			}
			styleClass := 1 // dim
			if t > 0.55 {
				styleClass = 2
			}
			if t > 0.85 {
				styleClass = 3
			}
			c.putZ(sx, sy, zr, shade[idx], styleClass)
		}
	}

	// Equator ring (highlighted).
	for phi := 0.0; phi < 2*math.Pi; phi += 0.04 {
		x := math.Sin(phi)
		z := math.Cos(phi)
		xr := x*cosY + z*sinY
		zr := -x*sinY + z*cosY
		sx := int(math.Round(cx + xr*radius))
		sy := int(math.Round(cy))
		c.putZ(sx, sy, zr+0.05, '─', 2)
	}

	// Audio-reactive bloom: when loud, radiate concentric rings from center.
	level := math.Max(lvlL, lvlR)
	if level > 0.15 {
		ringR := radius * (0.25 + 0.35*level + 0.2*math.Sin(phase*4))
		drawRing(c, cx, cy, ringR, aspectY, '·', 2)
	}

	// Center "head" — pulsing dot scaled by VU.
	headChar := '◍'
	if level > 0.6 {
		headChar = '◉'
	}
	c.put(int(cx), int(cy), headChar, 4)

	// Sound source: orbits on a tilted ring on the sphere.
	tilt := 0.35
	cosT, sinT := math.Cos(tilt), math.Sin(tilt)
	sx, sy, sz := orbitPoint(phase, cosT, sinT)
	// Apply yaw.
	sxR := sx*cosY + sz*sinY
	szR := -sx*sinY + sz*cosY
	syR := sy

	dx := int(math.Round(cx + sxR*radius))
	dy := int(math.Round(cy - syR*radius*aspectY))
	// Bigger glyph when on the front hemisphere; faint when behind.
	srcCh := '●'
	srcStyle := 5
	if szR < -0.2 {
		srcCh = '∘'
		srcStyle = 2
	}
	c.put(dx, dy, srcCh, srcStyle)

	// Comet trail behind the sound source.
	trail := []rune{'∙', '·', '·', '·', '·'}
	for k, ch := range trail {
		p := phase - float64(k+1)*0.07
		tx, ty, tz := orbitPoint(p, cosT, sinT)
		txR := tx*cosY + tz*sinY
		tzR := -tx*sinY + tz*cosY
		tyR := ty
		ix := int(math.Round(cx + txR*radius))
		iy := int(math.Round(cy - tyR*radius*aspectY))
		if ix == dx && iy == dy {
			continue
		}
		// Only draw on the front-facing half of the sphere for clarity.
		if tzR > 0 {
			c.put(ix, iy, ch, 6)
		}
	}

	// Pause overlay.
	if paused {
		bigPause(c, int(cx), int(cy))
	}

	return c.render()
}

// orbitPoint is a unit-sphere point at LFO phase on a ring tilted by (cosT, sinT).
func orbitPoint(phase, cosT, sinT float64) (x, y, z float64) {
	x = math.Sin(phase) * cosT
	y = math.Cos(phase) * sinT
	z = math.Cos(phase) * cosT
	return
}

func drawRing(c *canvas, cx, cy, r, aspectY float64, ch rune, style int) {
	for t := 0.0; t < 2*math.Pi; t += 0.06 {
		x := int(math.Round(cx + r*math.Sin(t)))
		y := int(math.Round(cy - r*math.Cos(t)*aspectY))
		if x >= 0 && x < c.w && y >= 0 && y < c.h && c.grid[y][x] == ' ' {
			c.put(x, y, ch, style)
		}
	}
}

func bigPause(c *canvas, cx, cy int) {
	for di := -1; di <= 1; di++ {
		row := cy + di
		for k := -1; k <= 1; k += 2 {
			c.put(cx+k, row, '┃', 7)
		}
	}
}

// ── Lissajous ────────────────────────────────────────────────────────────────

// lissajousView draws an evolving Lissajous curve with the sound source
// riding the curve. Numerator ratios shift slowly so the figure morphs.
func lissajousView(phase, lvlL, lvlR float64, w, h int, paused bool) string {
	c := newCanvas(w, h)
	const aspectY = 0.5

	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	rx := float64(w-1) / 2 * 0.9
	ry := float64(h-1) / 2 * 0.9

	drawStars(c, phase*0.5)

	// Slowly evolving frequency ratios for the figure.
	a := 3.0
	b := 2.0 + math.Sin(phase*0.07)
	delta := phase * 0.5

	// Trace the curve.
	for t := 0.0; t < 2*math.Pi; t += 0.005 {
		px := math.Sin(a*t + delta)
		py := math.Sin(b * t)
		x := int(math.Round(cx + px*rx))
		y := int(math.Round(cy - py*ry*aspectY))
		c.put(x, y, '·', 2)
	}

	// Sound source riding the curve at "current" t = phase.
	tNow := phase * 0.7
	px := math.Sin(a*tNow + delta)
	py := math.Sin(b * tNow)
	dx := int(math.Round(cx + px*rx))
	dy := int(math.Round(cy - py*ry*aspectY))

	// Comet trail.
	for k := 1; k <= 6; k++ {
		tt := tNow - float64(k)*0.04
		tx := math.Sin(a*tt + delta)
		ty := math.Sin(b * tt)
		ix := int(math.Round(cx + tx*rx))
		iy := int(math.Round(cy - ty*ry*aspectY))
		c.put(ix, iy, '∙', 6)
	}
	c.put(dx, dy, '●', 5)

	// Center marker.
	level := math.Max(lvlL, lvlR)
	if level > 0.15 {
		drawRing(c, cx, cy, math.Min(rx, ry/aspectY)*0.3*(0.6+level), aspectY, '·', 2)
	}
	c.put(int(cx), int(cy), '◍', 4)

	if paused {
		bigPause(c, int(cx), int(cy))
	}
	return c.render()
}

// ── Spectrum (radial FFT) ────────────────────────────────────────────────────

// spectrumView renders a radial spectrum — bars fan out from the center, one
// per FFT band. Bass at top, sweeping clockwise into treble. Sound source
// orbits on the inner ring; head pulses with overall level.
func spectrumView(phase float64, bands []float64, lvlL, lvlR float64, w, h int, paused bool) string {
	c := newCanvas(w, h)
	const aspectY = 0.5

	cx := float64(w-1) / 2
	cy := float64(h-1) / 2

	innerR := math.Min(float64(w)/8, float64(h)/(4*aspectY))
	if innerR < 3 {
		innerR = 3
	}
	maxR := math.Min(float64(w)/2, float64(h)/(2*aspectY)) * 0.95
	barLen := maxR - innerR

	drawStars(c, phase*0.3)

	if len(bands) == 0 {
		// No FFT data yet — show a faint orbit + center.
		drawRing(c, cx, cy, innerR, aspectY, '·', 1)
		c.put(int(cx), int(cy), '◍', 4)
		if paused {
			bigPause(c, int(cx), int(cy))
		}
		return c.render()
	}

	n := len(bands)
	// Bar palette by intensity along the bar.
	for b := 0; b < n; b++ {
		angle := -math.Pi/2 + float64(b)/float64(n)*2*math.Pi
		mag := bands[b]
		mag = clamp(mag, 0, 1)
		// Slight low-end emphasis so kicks pop.
		mag = math.Pow(mag, 0.85)

		barL := mag * barLen
		// Step radially outward, drawing one cell per radial unit.
		steps := int(math.Round(barL))
		for r := 0; r <= steps; r++ {
			rad := innerR + float64(r)
			x := int(math.Round(cx + rad*math.Sin(angle)))
			y := int(math.Round(cy - rad*math.Cos(angle)*aspectY))
			t := 0.0
			if steps > 0 {
				t = float64(r) / float64(steps)
			}
			ch := '·'
			style := 1
			switch {
			case t < 0.3:
				ch, style = '·', 1
			case t < 0.6:
				ch, style = ':', 2
			case t < 0.85:
				ch, style = '+', 3
			default:
				ch, style = '*', 5
			}
			c.put(x, y, ch, style)
		}
	}

	// Inner ring suggesting "where the orbit is."
	drawRing(c, cx, cy, innerR, aspectY, '·', 2)

	// Sound source on the inner ring.
	dx := int(math.Round(cx + innerR*math.Sin(phase)))
	dy := int(math.Round(cy - innerR*math.Cos(phase)*aspectY))
	c.put(dx, dy, '●', 5)

	// Comet trail.
	for k := 1; k <= 5; k++ {
		p := phase - float64(k)*0.07
		ix := int(math.Round(cx + innerR*math.Sin(p)))
		iy := int(math.Round(cy - innerR*math.Cos(p)*aspectY))
		if ix == dx && iy == dy {
			continue
		}
		c.put(ix, iy, '∙', 6)
	}

	// Center head — pulses.
	level := math.Max(lvlL, lvlR)
	headChar := '◍'
	if level > 0.5 {
		headChar = '◉'
	}
	c.put(int(cx), int(cy), headChar, 4)

	if paused {
		bigPause(c, int(cx), int(cy))
	}
	return c.render()
}

// ── Zen (breathing enso) ─────────────────────────────────────────────────────

// zenView is a slow, calming visualizer modelled on box-breathing (4-4-4-4).
// A single enso circle softly expands on inhale, holds, contracts on exhale,
// and holds again. The screen prompts what to do this beat. Stars are sparse;
// movement is slow. No comet, no spinning sphere. Just the breath.
func zenView(phase, lvlL, lvlR float64, w, h int, paused bool) string {
	c := newCanvas(w, h)
	const aspectY = 0.5

	cx := float64(w-1) / 2
	cy := float64(h-1) / 2

	innerR := math.Min(float64(w)/8, float64(h)/(4*aspectY))
	if innerR < 2 {
		innerR = 2
	}
	maxR := math.Min(float64(w)/2, float64(h)/(2*aspectY)) * 0.85

	// Very sparse, very slow stars.
	drawStars(c, phase*0.2)

	// Box-breathing: 4 phases of equal length. Total cycle 16s. Driven
	// from wall-clock time so the animation is smooth at the UI's 30 Hz
	// tick rate instead of stuttering at the audio buffer rate (~10 Hz).
	cycleSec := 16.0
	secs := float64(time.Now().UnixNano()) / 1e9
	tNorm := math.Mod(secs, cycleSec) / cycleSec
	// 4 phases at 0.25 each.
	var label string
	var size float64 // 0..1, current radius fraction
	switch {
	case tNorm < 0.25:
		// inhale — expand
		t := tNorm / 0.25
		size = easeInOut(t)
		label = "inhale"
	case tNorm < 0.50:
		// hold — full
		size = 1
		label = "hold"
	case tNorm < 0.75:
		// exhale — contract
		t := (tNorm - 0.50) / 0.25
		size = 1 - easeInOut(t)
		label = "exhale"
	default:
		// hold — empty
		size = 0
		label = "hold"
	}

	curR := innerR + (maxR-innerR)*size

	// Draw the enso ring — slightly imperfect, with a subtle "ink break"
	// on one side (a real enso is drawn in one breath stroke).
	const breakStart = 5.4 // radians (lower-right gap)
	const breakEnd = 5.9
	for t := 0.0; t < 2*math.Pi; t += 0.025 {
		if t > breakStart && t < breakEnd {
			continue
		}
		x := int(math.Round(cx + curR*math.Sin(t)))
		y := int(math.Round(cy - curR*math.Cos(t)*aspectY))
		// Inkier on the "drawing direction" side; faint elsewhere.
		ch := '·'
		style := 2
		if t < math.Pi {
			ch = '○'
			style = 5
		}
		// Re-mark with brighter glyph at a few "ink puddles."
		if math.Abs(math.Mod(t-0.4, 1.6)) < 0.12 {
			ch = '●'
			style = 5
		}
		c.put(x, y, ch, style)
	}

	// Center dot — your breath.
	headChar := '·'
	if size > 0.6 {
		headChar = '◌'
	}
	if size > 0.9 {
		headChar = '◍'
	}
	c.put(int(cx), int(cy), headChar, 4)

	// Labels above and below.
	titleStr := "breathe"
	subStr := label
	if paused {
		subStr = "paused — esc to exit"
	}
	titleX := int(cx) - len([]rune(titleStr))/2
	subX := int(cx) - len([]rune(subStr))/2
	for i, r := range titleStr {
		c.put(titleX+i, 1, r, 3)
	}
	for i, r := range subStr {
		c.put(subX+i, h-2, r, 4)
	}

	if paused {
		bigPause(c, int(cx), int(cy))
	}
	return c.render()
}

// easeInOut maps 0..1 → 0..1 with a smooth S-curve (cosine ease).
func easeInOut(t float64) float64 {
	return 0.5 - 0.5*math.Cos(t*math.Pi)
}

// ── Tunnel ───────────────────────────────────────────────────────────────────

// tunnelView is a 3D tunnel of rings flying toward you, with the sound
// orbiting on one of the rings. Hypnotic and very "in space."
func tunnelView(phase, lvlL, lvlR float64, w, h int, paused bool) string {
	c := newCanvas(w, h)
	const aspectY = 0.5

	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	maxR := math.Min(float64(w)/2, float64(h)/(2*aspectY)) * 0.95

	drawStars(c, phase*0.7)

	// Rings at varying depths. Depth d wraps around so they "fly toward you".
	const numRings = 8
	for k := 0; k < numRings; k++ {
		// Each ring is at a virtual depth that decreases over time.
		rawZ := math.Mod(phase*0.9+float64(k)/float64(numRings), 1.0)
		// Closer = bigger.
		ringR := maxR * (0.05 + rawZ*0.95)
		// Bright when far, fade as it gets close to edge.
		t := 1 - math.Abs(rawZ-0.5)*1.4
		styleClass := 1
		if t > 0.4 {
			styleClass = 2
		}
		if t > 0.8 {
			styleClass = 3
		}
		ch := '·'
		if t > 0.5 {
			ch = '∙'
		}
		if t > 0.85 {
			ch = '*'
		}
		drawRing(c, cx, cy, ringR, aspectY, ch, styleClass)
	}

	// Sound source: orbits at a fixed depth at the current phase.
	soundDepth := 0.55
	soundR := maxR * (0.05 + soundDepth*0.95)
	dx := int(math.Round(cx + soundR*math.Sin(phase)))
	dy := int(math.Round(cy - soundR*math.Cos(phase)*aspectY))
	c.put(dx, dy, '●', 5)

	// Trail.
	for k := 1; k <= 6; k++ {
		p := phase - float64(k)*0.07
		ix := int(math.Round(cx + soundR*math.Sin(p)))
		iy := int(math.Round(cy - soundR*math.Cos(p)*aspectY))
		c.put(ix, iy, '∙', 6)
	}

	// Center "you" — pulses with audio.
	level := math.Max(lvlL, lvlR)
	headChar := '◍'
	if level > 0.5 {
		headChar = '◉'
	}
	c.put(int(cx), int(cy), headChar, 4)

	if paused {
		bigPause(c, int(cx), int(cy))
	}
	return c.render()
}
