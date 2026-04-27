package ui

// rect is a hit region in screen-cell coordinates (1-cell granularity).
type rect struct {
	x, y, w, h int
	enabled    bool
}

func (r rect) contains(x, y int) bool {
	return r.enabled && x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

// fraction returns x's position within the rect as 0..1.
func (r rect) fraction(x int) float64 {
	if !r.enabled || r.w <= 1 {
		return 0
	}
	f := float64(x-r.x) / float64(r.w-1)
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return f
}

// clickRegions tracks rectangular hit zones rendered on the playing screen.
// They're populated during View() and consumed during MouseMsg handling.
type clickRegions struct {
	progress     rect
	chip8D       rect
	chipReverb   rect
	chipNoise    rect
	chipBinaural rect
	chipPause    rect
}

func (c *clickRegions) reset() {
	*c = clickRegions{}
}
