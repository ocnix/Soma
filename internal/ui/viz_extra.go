package ui

import (
	"math"
	"math/rand"
	"time"
)

// ── Fractal kaleidoscope ─────────────────────────────────────────────────────

// fractalView renders a 6-fold kaleidoscope of audio-reactive ripples with a
// slow rotation and a twisting fractal interference pattern. Trippy but
// remains readable. Driven from wall-clock so it stays smooth at any LFO
// rate; FFT bands modulate intensity at concentric radii.
func fractalView(phase float64, bands []float64, lvlL, lvlR float64, w, h int, paused bool) string {
	c := newCanvas(w, h)
	const aspectY = 0.5

	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	maxR := math.Min(float64(w)/2, float64(h)/(2*aspectY)) * 1.05

	t := float64(time.Now().UnixNano()) / 1e9

	// Slow global rotation — wall-clock driven.
	rot := t * 0.18

	const sectors = 6.0
	sectorAngle := 2 * math.Pi / sectors

	// FFT-derived ripple boost (averaged).
	boost := 0.0
	if len(bands) > 0 {
		var sum float64
		for _, b := range bands {
			sum += b
		}
		boost = sum / float64(len(bands))
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dy := -(float64(y) - cy) / aspectY
			r := math.Sqrt(dx*dx + dy*dy)
			if r > maxR {
				continue
			}
			theta := math.Atan2(dy, dx) + rot

			// Fold into one sector and mirror within it.
			s := math.Mod(theta+math.Pi*4, sectorAngle)
			if s > sectorAngle/2 {
				s = sectorAngle - s
			}

			// Compose pattern: radial ripples + angular harmonics + twist.
			ripple := 0.5 + 0.5*math.Sin(r*0.45-t*1.6)
			arms := 0.5 + 0.5*math.Cos(s*7+r*0.18)
			twist := 0.5 + 0.5*math.Sin(s*4+t*0.7+math.Sin(r*0.2)*0.8)

			// FFT bands: a ring around radius r/maxR maps to one band.
			bandLevel := 0.0
			if len(bands) > 0 {
				bi := int((r / maxR) * float64(len(bands)))
				if bi >= len(bands) {
					bi = len(bands) - 1
				}
				bandLevel = bands[bi]
			}

			value := ripple*arms*0.7 + twist*0.4 + bandLevel*0.6 + boost*0.15
			value = clamp(value, 0, 1.4)

			switch {
			case value > 1.05:
				c.put(x, y, '*', 5)
			case value > 0.85:
				c.put(x, y, '+', 3)
			case value > 0.65:
				c.put(x, y, '○', 5)
			case value > 0.50:
				c.put(x, y, ':', 2)
			case value > 0.35:
				c.put(x, y, '·', 1)
			}
		}
	}

	// Dim center mark.
	c.put(int(cx), int(cy), '◍', 4)

	if paused {
		bigPause(c, int(cx), int(cy))
	}
	return c.render()
}

// ── Conway's Game of Life ────────────────────────────────────────────────────
//
// Cells follow standard B3/S23 rules. Audio level seeds new live cells in a
// random region — louder = more seeds. Bass band spawns gliders so the life
// keeps moving; treble adds blinkers. Cells track age so we can render
// them with brightness based on how long they've survived.

var (
	lifeGrid    [][]uint8
	lifeNext    [][]uint8
	lifeW, lifeH int
	lifeStepCtr int
	lifeRng     = rand.New(rand.NewSource(7))
	lifeLastBass float64
)

func lifeView(phase float64, bands []float64, lvlL, lvlR float64, w, h int, paused bool) string {
	c := newCanvas(w, h)

	// Resize / initialize grid.
	if lifeGrid == nil || lifeW != w || lifeH != h {
		lifeGrid = makeGrid(w, h)
		lifeNext = makeGrid(w, h)
		lifeW, lifeH = w, h
		seedRandom(lifeGrid, w*h/8) // start with some life
	}

	// Step every other UI frame for ~15 Hz evolution.
	lifeStepCtr++
	if lifeStepCtr%2 == 0 && !paused {
		stepLife()
	}

	// Audio-driven seeding.
	if !paused {
		level := math.Max(lvlL, lvlR)
		if level > 0.20 {
			n := int(level * 8)
			seedCluster(lifeGrid, n, lifeRng)
		}
		if len(bands) > 0 {
			bass := bands[0]
			if bass > 0.55 && bass-lifeLastBass > 0.10 {
				spawnGlider(lifeGrid, lifeRng)
			}
			lifeLastBass = bass
			if len(bands) > 8 {
				treble := bands[len(bands)-2]
				if treble > 0.60 {
					spawnBlinker(lifeGrid, lifeRng)
				}
			}
		}
	}

	// Render: glyph by age.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := lifeGrid[y][x]
			if a == 0 {
				continue
			}
			switch {
			case a > 60:
				c.put(x, y, '@', 5)
			case a > 30:
				c.put(x, y, '#', 3)
			case a > 10:
				c.put(x, y, '+', 2)
			case a > 3:
				c.put(x, y, '·', 2)
			default:
				c.put(x, y, '·', 1)
			}
		}
	}

	if paused {
		cx := w / 2
		cy := h / 2
		bigPause(c, cx, cy)
	}
	return c.render()
}

func makeGrid(w, h int) [][]uint8 {
	g := make([][]uint8, h)
	for i := range g {
		g[i] = make([]uint8, w)
	}
	return g
}

func stepLife() {
	for y := 0; y < lifeH; y++ {
		for x := 0; x < lifeW; x++ {
			n := neighbors(lifeGrid, x, y, lifeW, lifeH)
			alive := lifeGrid[y][x] > 0
			switch {
			case alive && (n == 2 || n == 3):
				age := lifeGrid[y][x]
				if age < 250 {
					age++
				}
				lifeNext[y][x] = age
			case !alive && n == 3:
				lifeNext[y][x] = 1
			default:
				lifeNext[y][x] = 0
			}
		}
	}
	lifeGrid, lifeNext = lifeNext, lifeGrid
}

func neighbors(grid [][]uint8, x, y, w, h int) int {
	count := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			if nx < 0 || nx >= w || ny < 0 || ny >= h {
				continue
			}
			if grid[ny][nx] > 0 {
				count++
			}
		}
	}
	return count
}

func seedRandom(grid [][]uint8, n int) {
	h := len(grid)
	if h == 0 {
		return
	}
	w := len(grid[0])
	for i := 0; i < n; i++ {
		x := lifeRng.Intn(w)
		y := lifeRng.Intn(h)
		grid[y][x] = 1
	}
}

// seedCluster drops a small group of cells in a random spot so the audio
// pulse feels like it spawns "something coherent" rather than scattered noise.
func seedCluster(grid [][]uint8, n int, rng *rand.Rand) {
	h := len(grid)
	if h == 0 {
		return
	}
	w := len(grid[0])
	cx := rng.Intn(w)
	cy := rng.Intn(h)
	for i := 0; i < n; i++ {
		x := cx + rng.Intn(5) - 2
		y := cy + rng.Intn(3) - 1
		if x < 0 || x >= w || y < 0 || y >= h {
			continue
		}
		grid[y][x] = 1
	}
}

// Glider pattern (moves diagonally).
var gliderShape = [][2]int{
	{1, 0}, {2, 1}, {0, 2}, {1, 2}, {2, 2},
}

func spawnGlider(grid [][]uint8, rng *rand.Rand) {
	h := len(grid)
	if h < 4 {
		return
	}
	w := len(grid[0])
	if w < 4 {
		return
	}
	x0 := rng.Intn(w - 3)
	y0 := rng.Intn(h - 3)
	for _, p := range gliderShape {
		grid[y0+p[1]][x0+p[0]] = 1
	}
}

// Blinker (3 cells in a row).
func spawnBlinker(grid [][]uint8, rng *rand.Rand) {
	h := len(grid)
	if h < 2 {
		return
	}
	w := len(grid[0])
	if w < 4 {
		return
	}
	x0 := rng.Intn(w - 3)
	y0 := rng.Intn(h - 1)
	grid[y0][x0] = 1
	grid[y0][x0+1] = 1
	grid[y0][x0+2] = 1
}
