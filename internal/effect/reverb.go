package effect

import (
	"sync"

	"github.com/gopxl/beep/v2"
)

// Reverb is a tiny Schroeder reverb: 4 parallel comb filters into 2 series
// allpass filters. The wet signal is mono-summed and mixed equally into both
// channels — that gives the dry-panned source a sense of "room" without
// fighting the 8D pan.
type Reverb struct {
	src       beep.Streamer
	combs     []*comb
	allpasses []*allpass

	mu     sync.RWMutex
	mix    float64
	bypass bool
}

func NewReverb(src beep.Streamer, sampleRate, mix float64) *Reverb {
	sr := sampleRate
	r := &Reverb{
		src: src,
		mix: mix,
		combs: []*comb{
			newComb(int(0.0297*sr), 0.84),
			newComb(int(0.0371*sr), 0.83),
			newComb(int(0.0411*sr), 0.82),
			newComb(int(0.0437*sr), 0.81),
		},
		allpasses: []*allpass{
			newAllpass(int(0.005 * sr)),
			newAllpass(int(0.0017 * sr)),
		},
	}
	return r
}

func (r *Reverb) SetMix(m float64) {
	if m < 0 {
		m = 0
	}
	if m > 1 {
		m = 1
	}
	r.mu.Lock()
	r.mix = m
	r.mu.Unlock()
}

func (r *Reverb) SetBypass(b bool) {
	r.mu.Lock()
	r.bypass = b
	r.mu.Unlock()
}

func (r *Reverb) Stream(samples [][2]float64) (int, bool) {
	n, ok := r.src.Stream(samples)
	if n == 0 {
		return n, ok
	}

	r.mu.RLock()
	bypass := r.bypass
	mix := r.mix
	r.mu.RUnlock()

	if bypass || mix <= 0 {
		return n, ok
	}

	for i := 0; i < n; i++ {
		in := (samples[i][0] + samples[i][1]) * 0.5

		var wet float64
		for _, c := range r.combs {
			wet += c.process(in)
		}
		wet /= float64(len(r.combs))
		for _, a := range r.allpasses {
			wet = a.process(wet)
		}

		samples[i][0] = samples[i][0]*(1-mix) + wet*mix
		samples[i][1] = samples[i][1]*(1-mix) + wet*mix
	}
	return n, ok
}

func (r *Reverb) Err() error { return r.src.Err() }

// ── primitives ───────────────────────────────────────────────────────────────

type comb struct {
	buf      []float64
	pos      int
	feedback float64
}

func newComb(size int, fb float64) *comb {
	if size < 1 {
		size = 1
	}
	return &comb{buf: make([]float64, size), feedback: fb}
}

func (c *comb) process(in float64) float64 {
	out := c.buf[c.pos]
	c.buf[c.pos] = in + out*c.feedback
	c.pos++
	if c.pos >= len(c.buf) {
		c.pos = 0
	}
	return out
}

type allpass struct {
	buf []float64
	pos int
}

func newAllpass(size int) *allpass {
	if size < 1 {
		size = 1
	}
	return &allpass{buf: make([]float64, size)}
}

func (a *allpass) process(in float64) float64 {
	delayed := a.buf[a.pos]
	out := -in + delayed
	a.buf[a.pos] = in + delayed*0.5
	a.pos++
	if a.pos >= len(a.buf) {
		a.pos = 0
	}
	return out
}
