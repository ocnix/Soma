package effect

import (
	"math"

	"github.com/gopxl/beep/v2"
)

// Limiter is a soft saturator that clamps signal peaks gracefully instead
// of clipping. Uses tanh so values up to ~±0.7 pass nearly untouched while
// peaks above that taper smoothly to ±1.0. Sits at the very end of the
// chain as a safety net against the sum of music + noise + binaural +
// reverb exceeding the speaker's headroom.
//
// drive < 1 keeps the curve gentle; drive of ~0.95 leaves room above 0 dBFS
// without sounding compressed.
type Limiter struct {
	src   beep.Streamer
	drive float64
}

func NewLimiter(src beep.Streamer, drive float64) *Limiter {
	if drive <= 0 {
		drive = 1
	}
	return &Limiter{src: src, drive: drive}
}

func (l *Limiter) Stream(samples [][2]float64) (int, bool) {
	n, ok := l.src.Stream(samples)
	if n == 0 {
		return n, ok
	}
	d := l.drive
	for i := 0; i < n; i++ {
		samples[i][0] = math.Tanh(samples[i][0] * d)
		samples[i][1] = math.Tanh(samples[i][1] * d)
	}
	return n, ok
}

func (l *Limiter) Err() error { return l.src.Err() }
