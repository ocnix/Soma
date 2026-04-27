package effect

import (
	"math"
	"math/rand"
	"sync"
)

// NoiseMode selects the spectrum/timbre of generated ambient sound.
type NoiseMode int

const (
	NoiseOff NoiseMode = iota
	NoiseWhite
	NoisePink
	NoiseBrown
	NoiseRain
	NoiseOcean
	NoiseWind
	NoiseFire
)

func NoiseFromString(s string) NoiseMode {
	switch s {
	case "white":
		return NoiseWhite
	case "pink":
		return NoisePink
	case "brown":
		return NoiseBrown
	case "rain":
		return NoiseRain
	case "ocean":
		return NoiseOcean
	case "wind":
		return NoiseWind
	case "fire":
		return NoiseFire
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
	case NoiseRain:
		return "rain"
	case NoiseOcean:
		return "ocean"
	case NoiseWind:
		return "wind"
	case NoiseFire:
		return "fire"
	default:
		return "off"
	}
}

const noiseModeCount = 8

// Noise is a beep.Streamer that synthesises noise / ambience.
type Noise struct {
	mu         sync.RWMutex
	mode       NoiseMode
	volume     float64 // 0..1 linear
	sampleRate float64

	rng *rand.Rand

	// Voss-McCartney pink noise state.
	pinkRows [16]float64
	pinkSum  float64
	pinkCtr  int

	// Brown noise leaky integrator.
	brown float64

	// Slow LFO phases for ocean / wind.
	oceanPhase float64
	windPhase  float64
	windSlow   float64

	// Active rain droplets and fire crackles.
	drops    []droplet
	crackles []droplet
}

type droplet struct {
	age      int      // samples since spawn
	freq     float64  // tonal hint
	phase    float64  // current phase
	envelope float64  // current amplitude
	decay    float64  // per-sample decay multiplier
	gain     float64
}

// SampleRate is needed for time-based modulations; default to 44100 if unset.
const defaultSR = 44100.0

func NewNoise(mode NoiseMode, volume float64) *Noise {
	n := &Noise{
		mode:       mode,
		volume:     volume,
		sampleRate: defaultSR,
		rng:        rand.New(rand.NewSource(2)),
	}
	for i := range n.pinkRows {
		n.pinkRows[i] = (n.rng.Float64()*2 - 1)
		n.pinkSum += n.pinkRows[i]
	}
	return n
}

func (n *Noise) SetSampleRate(sr float64) {
	if sr <= 0 {
		return
	}
	n.mu.Lock()
	n.sampleRate = sr
	n.mu.Unlock()
}

func (n *Noise) SetMode(m NoiseMode) {
	n.mu.Lock()
	n.mode = m
	n.drops = nil
	n.crackles = nil
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
	sr := n.sampleRate
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
			v = n.pinkTick()

		case NoiseBrown:
			v = n.brownTick() * 3.5 // makeup gain

		case NoiseRain:
			v = n.rainTick(sr)

		case NoiseOcean:
			v = n.oceanTick(sr)

		case NoiseWind:
			v = n.windTick(sr)

		case NoiseFire:
			v = n.fireTick(sr)
		}
		v *= vol
		samples[i][0] = v
		samples[i][1] = v
	}
	return len(samples), true
}

func (n *Noise) Err() error { return nil }

// ── primitives ───────────────────────────────────────────────────────────────

func (n *Noise) pinkTick() float64 {
	n.pinkCtr++
	row := trailingZeros(n.pinkCtr) % len(n.pinkRows)
	n.pinkSum -= n.pinkRows[row]
	n.pinkRows[row] = n.rng.Float64()*2 - 1
	n.pinkSum += n.pinkRows[row]
	return n.pinkSum / float64(len(n.pinkRows))
}

func (n *Noise) brownTick() float64 {
	n.brown += (n.rng.Float64()*2 - 1) * 0.02
	if n.brown > 1 {
		n.brown = 1
	} else if n.brown < -1 {
		n.brown = -1
	}
	return n.brown
}

// ── ambient modes ────────────────────────────────────────────────────────────

func (n *Noise) rainTick(sr float64) float64 {
	// Steady drizzle: pink noise base + a small high-pass-ish layer.
	base := n.pinkTick()*0.55 + (n.rng.Float64()*2-1)*0.18

	// Spawn droplets occasionally — ~120/sec at 44.1k.
	if n.rng.Float64() < 120/sr {
		n.drops = append(n.drops, droplet{
			freq:     1500 + n.rng.Float64()*3500,
			envelope: 0.7 + n.rng.Float64()*0.3,
			decay:    0.9985 + n.rng.Float64()*0.001,
			gain:     0.25,
		})
		if len(n.drops) > 32 {
			n.drops = n.drops[1:]
		}
	}
	// Mix active droplets — short tonal bursts modulated by quick decay.
	var dropMix float64
	out := n.drops[:0]
	for _, d := range n.drops {
		d.age++
		d.envelope *= d.decay
		if d.envelope < 0.005 {
			continue
		}
		d.phase += 2 * math.Pi * d.freq / sr
		if d.phase > 2*math.Pi {
			d.phase -= 2 * math.Pi
		}
		dropMix += math.Sin(d.phase) * d.envelope * d.gain
		out = append(out, d)
	}
	n.drops = out
	return base + dropMix
}

func (n *Noise) oceanTick(sr float64) float64 {
	// Brown noise (deep texture) modulated by a very slow LFO (~0.12 Hz).
	n.oceanPhase += 2 * math.Pi * 0.12 / sr
	if n.oceanPhase > 2*math.Pi {
		n.oceanPhase -= 2 * math.Pi
	}
	wave := 0.5 + 0.5*math.Sin(n.oceanPhase) // 0..1
	// Add a slight breath modulation around it.
	wave *= 0.7 + 0.3*math.Sin(n.oceanPhase*2.7)
	return n.brownTick() * 4.5 * wave
}

func (n *Noise) windTick(sr float64) float64 {
	// Two-lane wind: a slow gust LFO plus a faster swirl. Brown for warmth.
	n.windPhase += 2 * math.Pi * 0.08 / sr
	if n.windPhase > 2*math.Pi {
		n.windPhase -= 2 * math.Pi
	}
	n.windSlow += 2 * math.Pi * 0.31 / sr
	if n.windSlow > 2*math.Pi {
		n.windSlow -= 2 * math.Pi
	}
	gust := 0.4 + 0.4*math.Sin(n.windPhase) + 0.2*math.Sin(n.windSlow)
	if gust < 0 {
		gust = 0
	}
	// Mix brown (low) and pink (mid) for a more "moving air" feel.
	return (n.brownTick()*3.0 + n.pinkTick()*0.4) * gust
}

func (n *Noise) fireTick(sr float64) float64 {
	// Brown noise base for the warm hum.
	base := n.brownTick() * 2.8

	// Occasional crackles — brief bursts of noise with quick decay.
	if n.rng.Float64() < 25/sr {
		n.crackles = append(n.crackles, droplet{
			envelope: 0.6 + n.rng.Float64()*0.5,
			decay:    0.992 + n.rng.Float64()*0.005,
			gain:     0.55,
		})
		if len(n.crackles) > 12 {
			n.crackles = n.crackles[1:]
		}
	}
	var crk float64
	out := n.crackles[:0]
	for _, c := range n.crackles {
		c.envelope *= c.decay
		if c.envelope < 0.008 {
			continue
		}
		crk += (n.rng.Float64()*2 - 1) * c.envelope * c.gain
		out = append(out, c)
	}
	n.crackles = out
	return base + crk
}

// ── helpers ──────────────────────────────────────────────────────────────────

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
