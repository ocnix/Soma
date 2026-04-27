package effect

import (
	"math"

	"github.com/gopxl/beep/v2"

	"soma/internal/state"
)

// EightD wraps a stereo streamer and pans it left↔right with a slow LFO,
// producing the "8D audio" feel. Equal-power panning preserves perceived
// loudness across the sweep. Headphones required.
type EightD struct {
	src        beep.Streamer
	sampleRate float64
	rateHz     float64
	phase      float64
	st         *state.State
}

func NewEightD(src beep.Streamer, sampleRate, rateHz float64, st *state.State) *EightD {
	return &EightD{src: src, sampleRate: sampleRate, rateHz: rateHz, st: st}
}

func (e *EightD) Stream(samples [][2]float64) (int, bool) {
	n, ok := e.src.Stream(samples)
	if n == 0 {
		return n, ok
	}

	inc := 2 * math.Pi * e.rateHz / e.sampleRate
	for i := 0; i < n; i++ {
		// Mono-sum so energy follows the pan, not the source's L/R imbalance.
		mono := (samples[i][0] + samples[i][1]) * 0.5

		pan := math.Sin(e.phase)             // -1..1
		theta := (pan + 1) * math.Pi / 4     // 0..π/2
		samples[i][0] = mono * math.Cos(theta) * math.Sqrt2
		samples[i][1] = mono * math.Sin(theta) * math.Sqrt2

		e.phase += inc
		if e.phase > 2*math.Pi {
			e.phase -= 2 * math.Pi
		}
	}
	if e.st != nil {
		e.st.SetPan(math.Sin(e.phase))
	}
	return n, ok
}

func (e *EightD) Err() error { return e.src.Err() }
