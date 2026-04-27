package effect

import (
	"math"
	"math/rand"
	"sync"

	"github.com/gopxl/beep/v2"

	"soma/internal/state"
)

// Shape selects the LFO waveform that drives the pan.
type Shape int

const (
	ShapeSine Shape = iota
	ShapeRandom
)

func ShapeFromString(s string) Shape {
	if s == "random" {
		return ShapeRandom
	}
	return ShapeSine
}

func (s Shape) String() string {
	if s == ShapeRandom {
		return "random"
	}
	return "sine"
}

// EightD wraps a stereo streamer and pans it left↔right with an LFO,
// producing the "8D audio" feel. Equal-power panning preserves perceived
// loudness across the sweep. Headphones required.
//
// Live-tunable: rate, depth, shape, bypass.
type EightD struct {
	src        beep.Streamer
	sampleRate float64
	st         *state.State

	mu     sync.RWMutex
	rateHz float64
	depth  float64 // 0..1, scales the pan amplitude
	shape  Shape
	bypass bool

	// Sine-shape state.
	phase float64

	// Random-walk state. Smoothed brownian motion: target wanders, current
	// follows with a slew limit. Produces an organic, unpredictable pan.
	rngSrc  *rand.Rand
	walk    float64 // current pan position, -1..1
	target  float64 // current target the walk drifts toward
	stepCtr int
}

func NewEightD(src beep.Streamer, sampleRate, rateHz, depth float64, shape Shape, st *state.State) *EightD {
	if depth < 0 {
		depth = 0
	}
	if depth > 1 {
		depth = 1
	}
	return &EightD{
		src:        src,
		sampleRate: sampleRate,
		rateHz:     rateHz,
		depth:      depth,
		shape:      shape,
		st:         st,
		rngSrc:     rand.New(rand.NewSource(1)),
	}
}

func (e *EightD) SetRate(hz float64) {
	if hz < 0.01 {
		hz = 0.01
	}
	if hz > 2.0 {
		hz = 2.0
	}
	e.mu.Lock()
	e.rateHz = hz
	e.mu.Unlock()
	if e.st != nil {
		e.st.SetRate(hz)
	}
}

func (e *EightD) SetDepth(d float64) {
	if d < 0 {
		d = 0
	}
	if d > 1 {
		d = 1
	}
	e.mu.Lock()
	e.depth = d
	e.mu.Unlock()
	if e.st != nil {
		e.st.SetDepth(d)
	}
}

func (e *EightD) SetShape(s Shape) {
	e.mu.Lock()
	e.shape = s
	e.mu.Unlock()
	if e.st != nil {
		e.st.SetShape(s.String())
	}
}

func (e *EightD) SetBypass(b bool) {
	e.mu.Lock()
	e.bypass = b
	e.mu.Unlock()
	if e.st != nil {
		mode := "8D"
		if b {
			mode = "dry"
		}
		e.st.SetMode(mode)
	}
}

func (e *EightD) Stream(samples [][2]float64) (int, bool) {
	n, ok := e.src.Stream(samples)
	if n == 0 {
		return n, ok
	}

	e.mu.RLock()
	bypass := e.bypass
	rate := e.rateHz
	depth := e.depth
	shape := e.shape
	e.mu.RUnlock()

	if bypass {
		if e.st != nil {
			e.st.SetPan(0)
		}
		return n, ok
	}

	// Random-walk parameters.
	// Re-pick a target once every (1/rate) seconds, on average.
	targetEverySamples := int(e.sampleRate / math.Max(rate, 0.01))
	if targetEverySamples < 200 {
		targetEverySamples = 200
	}
	// Slew rate of the walk: how fast `walk` chases `target`. Rate-scaled
	// so faster rates = snappier transitions.
	slew := rate / e.sampleRate * 8

	inc := 2 * math.Pi * rate / e.sampleRate

	for i := 0; i < n; i++ {
		var pan float64
		switch shape {
		case ShapeRandom:
			e.stepCtr++
			if e.stepCtr >= targetEverySamples {
				e.stepCtr = 0
				// Pick a new target uniformly in [-1, 1].
				e.target = e.rngSrc.Float64()*2 - 1
			}
			diff := e.target - e.walk
			if diff > slew {
				diff = slew
			} else if diff < -slew {
				diff = -slew
			}
			e.walk += diff
			pan = e.walk
		default:
			pan = math.Sin(e.phase)
			e.phase += inc
			if e.phase > 2*math.Pi {
				e.phase -= 2 * math.Pi
			}
		}

		pan *= depth

		// Mono-sum the source so the energy follows the pan.
		mono := (samples[i][0] + samples[i][1]) * 0.5

		theta := (pan + 1) * math.Pi / 4
		samples[i][0] = mono * math.Cos(theta) * math.Sqrt2
		samples[i][1] = mono * math.Sin(theta) * math.Sqrt2
	}

	if e.st != nil {
		var p, ph float64
		if shape == ShapeRandom {
			p = e.walk
			// Map walk to a pseudo-phase so the orbital still spins; use
			// asin so the moving dot reflects pan position smoothly.
			ph = math.Asin(math.Max(-1, math.Min(1, e.walk))) + math.Pi/2
		} else {
			p = math.Sin(e.phase) * depth
			ph = e.phase
		}
		e.st.SetPan(p)
		e.st.SetPhase(ph)
	}
	return n, ok
}

func (e *EightD) Err() error { return e.src.Err() }
