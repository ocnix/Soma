package effect

import (
	"math"

	"github.com/gopxl/beep/v2"

	"soma/internal/state"
)

// Meter taps a stereo stream and writes smoothed peak L/R levels into State.
// Fast attack, slow release — matches how analog VU meters feel.
type Meter struct {
	src      beep.Streamer
	st       *state.State
	l, r     float64
}

func NewMeter(src beep.Streamer, st *state.State) *Meter {
	return &Meter{src: src, st: st}
}

func (m *Meter) Stream(samples [][2]float64) (int, bool) {
	n, ok := m.src.Stream(samples)
	if n == 0 {
		return n, ok
	}
	var pl, pr float64
	for i := 0; i < n; i++ {
		l := math.Abs(samples[i][0])
		r := math.Abs(samples[i][1])
		if l > pl {
			pl = l
		}
		if r > pr {
			pr = r
		}
	}
	const attack = 0.7
	const release = 0.12
	m.l = smooth(m.l, pl, attack, release)
	m.r = smooth(m.r, pr, attack, release)
	if m.st != nil {
		m.st.SetLevels(m.l, m.r)
	}
	return n, ok
}

func (m *Meter) Err() error { return m.src.Err() }

func smooth(prev, current, attack, release float64) float64 {
	if current > prev {
		return prev + (current-prev)*attack
	}
	return prev * (1 - release)
}
