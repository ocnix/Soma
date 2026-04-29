package effect

import (
	"math"
	"sync"

	"github.com/gopxl/beep/v2"
)

// Crossfeed is a Bauer-style headphone crossfeed: a small low-passed copy of
// each channel is bled into the other, simulating the natural acoustic
// crossfeed your head provides when listening to speakers. Reduces the
// fatiguing "in-your-head" image that's especially harsh on IEMs (which
// have zero physical crosstalk between drivers).
//
// Implementation: 1st-order low-pass filter at ~700 Hz on the cross feed
// (mimics head shadowing), then mix with the dry channel.
//
//   y_left  = dry_left  + amount * lp(dry_right)
//   y_right = dry_right + amount * lp(dry_left)
//
// Output is normalised so the total gain stays the same regardless of mix.
type Crossfeed struct {
	mu     sync.RWMutex
	amount float64 // 0..1, typical 0.25–0.5
	a      float64 // LP coefficient

	lpL, lpR float64

	src        beep.Streamer
	sampleRate float64
}

func NewCrossfeed(src beep.Streamer, sampleRate, amount float64) *Crossfeed {
	c := &Crossfeed{
		src:        src,
		sampleRate: sampleRate,
		amount:     clamp01(amount),
	}
	c.recomputeCoef()
	return c
}

func (c *Crossfeed) SetAmount(a float64) {
	c.mu.Lock()
	c.amount = clamp01(a)
	c.mu.Unlock()
}

func (c *Crossfeed) recomputeCoef() {
	const cutoff = 700.0 // Hz — head-shadow rolloff
	// 1-pole low-pass: y = a*x + (1-a)*y_prev, a = 1 - exp(-2π·fc/fs)
	c.a = 1 - math.Exp(-2*math.Pi*cutoff/c.sampleRate)
}

func (c *Crossfeed) Stream(samples [][2]float64) (int, bool) {
	n, ok := c.src.Stream(samples)
	if n == 0 {
		return n, ok
	}

	c.mu.RLock()
	amount := c.amount
	a := c.a
	c.mu.RUnlock()

	if amount <= 0 {
		return n, ok
	}

	// Normalisation: sum dry+wet must not exceed peak of dry alone.
	norm := 1.0 / (1.0 + amount)

	for i := 0; i < n; i++ {
		l := samples[i][0]
		r := samples[i][1]

		// LP-filter the cross signal so only low-mid frequencies bleed.
		c.lpL = c.lpL + a*(l-c.lpL)
		c.lpR = c.lpR + a*(r-c.lpR)

		samples[i][0] = (l + amount*c.lpR) * norm
		samples[i][1] = (r + amount*c.lpL) * norm
	}
	return n, ok
}

func (c *Crossfeed) Err() error { return c.src.Err() }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
