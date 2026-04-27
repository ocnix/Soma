package effect

import (
	"math"
	"sync"
)

// Binaural plays a slightly-detuned sine in each ear. With headphones, the
// brain perceives a "beat" at the difference frequency. Common targets:
// alpha (~8 Hz, relaxed focus) and beta (~14 Hz, alert focus).
//
// Carrier is a pleasant low tone (200 Hz) so it doesn't fight the music.
// Live-tunable: enabled, beat (Hz), volume.
type Binaural struct {
	sampleRate float64

	mu       sync.RWMutex
	enabled  bool
	beat     float64 // Hz — difference between L and R
	carrier  float64 // Hz — base tone
	volume   float64 // 0..1 linear

	phaseL, phaseR float64
}

func NewBinaural(sampleRate, beat, volume float64) *Binaural {
	return &Binaural{
		sampleRate: sampleRate,
		beat:       beat,
		carrier:    200,
		volume:     volume,
	}
}

func (b *Binaural) SetEnabled(on bool) {
	b.mu.Lock()
	b.enabled = on
	b.mu.Unlock()
}

func (b *Binaural) SetBeat(hz float64) {
	if hz < 1 {
		hz = 1
	}
	if hz > 40 {
		hz = 40
	}
	b.mu.Lock()
	b.beat = hz
	b.mu.Unlock()
}

func (b *Binaural) SetVolume(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	b.mu.Lock()
	b.volume = v
	b.mu.Unlock()
}

func (b *Binaural) Stream(samples [][2]float64) (int, bool) {
	b.mu.RLock()
	on := b.enabled
	beat := b.beat
	carrier := b.carrier
	vol := b.volume
	b.mu.RUnlock()

	if !on || vol <= 0 {
		for i := range samples {
			samples[i][0] = 0
			samples[i][1] = 0
		}
		return len(samples), true
	}

	fL := carrier - beat/2
	fR := carrier + beat/2
	incL := 2 * math.Pi * fL / b.sampleRate
	incR := 2 * math.Pi * fR / b.sampleRate

	for i := range samples {
		samples[i][0] = math.Sin(b.phaseL) * vol
		samples[i][1] = math.Sin(b.phaseR) * vol
		b.phaseL += incL
		b.phaseR += incR
		if b.phaseL > 2*math.Pi {
			b.phaseL -= 2 * math.Pi
		}
		if b.phaseR > 2*math.Pi {
			b.phaseR -= 2 * math.Pi
		}
	}
	return len(samples), true
}

func (b *Binaural) Err() error { return nil }
