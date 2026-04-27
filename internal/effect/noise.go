package effect

import (
	"math/rand"
	"sync"
)

// NoiseMode selects the spectrum of generated noise.
type NoiseMode int

const (
	NoiseOff NoiseMode = iota
	NoiseWhite
	NoisePink
	NoiseBrown
)

func NoiseFromString(s string) NoiseMode {
	switch s {
	case "white":
		return NoiseWhite
	case "pink":
		return NoisePink
	case "brown":
		return NoiseBrown
	default:
		return NoiseOff
	}
}

func (m NoiseMode) String() string {
	switch m {
	case NoiseWhite:
		return "white"
	case NoisePink:
		return "pink"
	case NoiseBrown:
		return "brown"
	default:
		return "off"
	}
}

// Noise is a beep.Streamer that generates white / pink / brown noise.
// Live-tunable: mode, volume.
//
// Pink: Voss-McCartney algorithm (cheap and good enough for ambience).
// Brown: leaky integrator over white noise.
type Noise struct {
	mu     sync.RWMutex
	mode   NoiseMode
	volume float64 // 0..1 linear

	rng *rand.Rand

	// Voss-McCartney rows for pink noise.
	pinkRows [16]float64
	pinkSum  float64
	pinkCtr  int

	// Leaky-integrator state for brown.
	brown float64
}

func NewNoise(mode NoiseMode, volume float64) *Noise {
	n := &Noise{
		mode:   mode,
		volume: volume,
		rng:    rand.New(rand.NewSource(2)),
	}
	for i := range n.pinkRows {
		n.pinkRows[i] = (n.rng.Float64()*2 - 1)
		n.pinkSum += n.pinkRows[i]
	}
	return n
}

func (n *Noise) SetMode(m NoiseMode) {
	n.mu.Lock()
	n.mode = m
	n.mu.Unlock()
}

func (n *Noise) SetVolume(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	n.mu.Lock()
	n.volume = v
	n.mu.Unlock()
}

func (n *Noise) Stream(samples [][2]float64) (int, bool) {
	n.mu.RLock()
	mode := n.mode
	vol := n.volume
	n.mu.RUnlock()

	if mode == NoiseOff || vol <= 0 {
		for i := range samples {
			samples[i][0] = 0
			samples[i][1] = 0
		}
		return len(samples), true
	}

	for i := range samples {
		var v float64
		switch mode {
		case NoiseWhite:
			v = n.rng.Float64()*2 - 1
		case NoisePink:
			// Voss-McCartney: pick which row to update by trailing zeros of ctr.
			n.pinkCtr++
			row := trailingZeros(n.pinkCtr) % len(n.pinkRows)
			n.pinkSum -= n.pinkRows[row]
			n.pinkRows[row] = n.rng.Float64()*2 - 1
			n.pinkSum += n.pinkRows[row]
			v = n.pinkSum / float64(len(n.pinkRows))
		case NoiseBrown:
			// Leaky integrator + clamp.
			n.brown += (n.rng.Float64()*2 - 1) * 0.02
			if n.brown > 1 {
				n.brown = 1
			} else if n.brown < -1 {
				n.brown = -1
			}
			v = n.brown * 3.5 // makeup gain — brown is naturally quiet
		}
		v *= vol
		samples[i][0] = v
		samples[i][1] = v
	}
	return len(samples), true
}

func (n *Noise) Err() error { return nil }

func trailingZeros(x int) int {
	if x == 0 {
		return 0
	}
	z := 0
	for x&1 == 0 {
		x >>= 1
		z++
	}
	return z
}
