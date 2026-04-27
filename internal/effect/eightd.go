package effect

import (
	"math"
	"sync"

	"github.com/gopxl/beep/v2"

	"soma/internal/state"
)

// EightD wraps a stereo streamer and pans it left↔right with a slow LFO,
// producing the "8D audio" feel. Equal-power panning preserves perceived
// loudness across the sweep. Headphones required.
type EightD struct {
	src        beep.Streamer
	sampleRate float64
	st         *state.State

	mu     sync.RWMutex
	rateHz float64
	bypass bool
	phase  float64
}

func NewEightD(src beep.Streamer, sampleRate, rateHz float64, st *state.State) *EightD {
	return &EightD{src: src, sampleRate: sampleRate, rateHz: rateHz, st: st}
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
	e.mu.RUnlock()

	if bypass {
		if e.st != nil {
			e.st.SetPan(0)
		}
		return n, ok
	}

	inc := 2 * math.Pi * rate / e.sampleRate
	for i := 0; i < n; i++ {
		mono := (samples[i][0] + samples[i][1]) * 0.5
		pan := math.Sin(e.phase)
		theta := (pan + 1) * math.Pi / 4
		samples[i][0] = mono * math.Cos(theta) * math.Sqrt2
		samples[i][1] = mono * math.Sin(theta) * math.Sqrt2

		e.phase += inc
		if e.phase > 2*math.Pi {
			e.phase -= 2 * math.Pi
		}
	}
	if e.st != nil {
		e.st.SetPan(math.Sin(e.phase))
		e.st.SetPhase(e.phase)
	}
	return n, ok
}

func (e *EightD) Err() error { return e.src.Err() }
