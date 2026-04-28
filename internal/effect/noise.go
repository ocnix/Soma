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
	NoiseBowl  // singing bowl / Tibetan bell — meditative
	NoiseStorm // rain + thunder rumbles
	NoiseForest
	NoiseCreek
	NoiseNight // crickets + faint wind
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
	case "bowl":
		return NoiseBowl
	case "storm":
		return NoiseStorm
	case "forest":
		return NoiseForest
	case "creek":
		return NoiseCreek
	case "night":
		return NoiseNight
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
	case NoiseBowl:
		return "bowl"
	case NoiseStorm:
		return "storm"
	case NoiseForest:
		return "forest"
	case NoiseCreek:
		return "creek"
	case NoiseNight:
		return "night"
	default:
		return "off"
	}
}

const noiseModeCount = 13

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

	// Bowl (Tibetan singing bowl) state.
	bowlSamples int
	bowls       []bowlStrike

	// Thunder for storm mode.
	thunderSamples int
	thunders       []thunderEvent

	// Forest birds and night crickets.
	birds         []birdChirp
	cricketPhase  float64
	cricketBuzzN  int

	// Creek bubble LFO.
	creekPhase float64
}

type bowlVoice struct {
	freq, phase, amp float64
}

type bowlStrike struct {
	envelope float64
	decay    float64
	voices   []bowlVoice
}

type thunderEvent struct {
	envelope float64
	decay    float64
}

type birdChirp struct {
	freqStart, freqEnd float64
	duration, age      int
	phase, envelope    float64
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

		case NoiseBowl:
			v = n.bowlTick(sr)

		case NoiseStorm:
			v = n.rainTick(sr)*0.7 + n.thunderTick(sr)

		case NoiseForest:
			v = n.forestTick(sr)

		case NoiseCreek:
			v = n.creekTick(sr)

		case NoiseNight:
			v = n.nightTick(sr)
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

// ── meditative + nature modes ────────────────────────────────────────────────

// bowlTick: Tibetan singing bowl. A struck bowl has an inharmonic spectrum
// (~1, 2.76, 5.40, 7.18 ratios are typical). Each strike rings for ~10 s,
// new strikes spawn every 25-45 s. A faint brown-noise hum fills the silence.
func (n *Noise) bowlTick(sr float64) float64 {
	if n.bowlSamples <= 0 {
		f := 220 + n.rng.Float64()*180 // 220-400 Hz fundamental
		n.bowls = append(n.bowls, bowlStrike{
			envelope: 1.0,
			decay:    0.99996, // long, gentle decay
			voices: []bowlVoice{
				{freq: f, amp: 0.7},
				{freq: f * 2.756, amp: 0.4},
				{freq: f * 5.405, amp: 0.22},
				{freq: f * 7.182, amp: 0.13},
			},
		})
		n.bowlSamples = int(sr * (25 + n.rng.Float64()*20))
	}
	n.bowlSamples--

	var sum float64
	out := n.bowls[:0]
	for _, b := range n.bowls {
		b.envelope *= b.decay
		if b.envelope < 0.0008 {
			continue
		}
		var sample float64
		for i := range b.voices {
			inc := 2 * math.Pi * b.voices[i].freq / sr
			b.voices[i].phase += inc
			if b.voices[i].phase > 2*math.Pi {
				b.voices[i].phase -= 2 * math.Pi
			}
			sample += math.Sin(b.voices[i].phase) * b.voices[i].amp
		}
		sum += sample * b.envelope * 0.45
		out = append(out, b)
	}
	n.bowls = out

	// Faint brown-noise bed so the silence doesn't feel dead.
	return sum + n.brownTick()*0.35
}

func (n *Noise) thunderTick(sr float64) float64 {
	// Schedule a thunder event every 15-60 seconds.
	if n.thunderSamples <= 0 {
		n.thunders = append(n.thunders, thunderEvent{
			envelope: 1.0,
			decay:    0.99988 + n.rng.Float64()*0.00006,
		})
		n.thunderSamples = int(sr * (15 + n.rng.Float64()*45))
	}
	n.thunderSamples--

	var sum float64
	out := n.thunders[:0]
	for _, t := range n.thunders {
		t.envelope *= t.decay
		if t.envelope < 0.001 {
			continue
		}
		// Brown noise is naturally low-frequency — perfect for rumble.
		sum += n.brownTick() * 5.5 * t.envelope
		out = append(out, t)
	}
	n.thunders = out
	return sum * 0.55
}

func (n *Noise) forestTick(sr float64) float64 {
	// Wind through trees: gentle brown.
	base := n.brownTick() * 1.6

	// Bird chirps occasionally (~1/sec on average).
	if n.rng.Float64() < 1.2/sr {
		n.birds = append(n.birds, birdChirp{
			freqStart: 2000 + n.rng.Float64()*4000,
			freqEnd:   1500 + n.rng.Float64()*3500,
			duration:  int(sr * (0.05 + n.rng.Float64()*0.15)),
			envelope:  0.45 + n.rng.Float64()*0.25,
		})
		if len(n.birds) > 8 {
			n.birds = n.birds[1:]
		}
	}

	var sum float64
	out := n.birds[:0]
	for _, b := range n.birds {
		b.age++
		if b.age >= b.duration {
			continue
		}
		t := float64(b.age) / float64(b.duration)
		f := b.freqStart + (b.freqEnd-b.freqStart)*t
		b.phase += 2 * math.Pi * f / sr
		if b.phase > 2*math.Pi {
			b.phase -= 2 * math.Pi
		}
		// Triangle envelope (attack-release).
		env := 1 - math.Abs(2*t-1)
		sum += math.Sin(b.phase) * env * b.envelope
		out = append(out, b)
	}
	n.birds = out
	return base + sum*0.32
}

func (n *Noise) creekTick(sr float64) float64 {
	// Pink noise modulated by a few overlapping LFOs to give a "bubbly" feel.
	n.creekPhase += 2 * math.Pi * 4.0 / sr
	if n.creekPhase > 2*math.Pi {
		n.creekPhase -= 2 * math.Pi
	}
	bubble := 0.6 + 0.4*math.Sin(n.creekPhase) + 0.15*math.Sin(n.creekPhase*2.7)
	if bubble < 0.2 {
		bubble = 0.2
	}
	return n.pinkTick() * bubble * 1.0
}

func (n *Noise) nightTick(sr float64) float64 {
	// Gentle wind bed.
	base := n.brownTick() * 0.65

	// Crickets — periodic chirps every ~0.7s.
	n.cricketPhase += 2 * math.Pi * 1.4 / sr
	if n.cricketPhase > 2*math.Pi {
		n.cricketPhase -= 2 * math.Pi
		n.cricketBuzzN = int(sr * 0.06) // 60ms chirp
	}
	var crk float64
	if n.cricketBuzzN > 0 {
		crk = (n.rng.Float64()*2 - 1) * 0.35
		n.cricketBuzzN--
	}
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
