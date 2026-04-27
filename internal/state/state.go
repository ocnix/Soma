package state

import (
	"sync"
	"time"
)

// State is shared between the audio goroutine (writer) and the UI goroutine
// (reader). Use the setters from any goroutine; Snapshot returns a value copy.
type State struct {
	mu sync.RWMutex

	// track / playback
	title    string
	duration time.Duration
	elapsed  time.Duration
	finished bool
	paused   bool

	// 8D effect
	mode   string  // "8D" | "dry"
	rateHz float64 // LFO frequency
	depth  float64 // 0..1
	shape  string  // "sine" | "random"
	pan    float64 // current pan, -1..1
	phase  float64 // raw LFO phase (used for orbital viz)

	// reverb
	reverbOn  bool
	reverbMix float64

	// noise + binaural
	noiseMode    string // "off" | "white" | "pink" | "brown"
	noiseVolume  float64
	binauralOn   bool
	binauralBeat float64

	// metering
	levelL float64
	levelR float64

	// master
	volumeDb float64
}

type Snapshot struct {
	Title        string
	Duration     time.Duration
	Elapsed      time.Duration
	Finished     bool
	Paused       bool
	Mode         string
	RateHz       float64
	Depth        float64
	Shape        string
	Pan          float64
	Phase        float64
	ReverbOn     bool
	ReverbMix    float64
	NoiseMode    string
	NoiseVolume  float64
	BinauralOn   bool
	BinauralBeat float64
	LevelL       float64
	LevelR       float64
	VolumeDb     float64
}

func New(title, mode string, rateHz float64, duration time.Duration) *State {
	return &State{title: title, mode: mode, rateHz: rateHz, duration: duration}
}

func (s *State) SetTitle(t string)             { s.mu.Lock(); s.title = t; s.mu.Unlock() }
func (s *State) SetPan(p float64)              { s.mu.Lock(); s.pan = p; s.mu.Unlock() }
func (s *State) SetPhase(p float64)            { s.mu.Lock(); s.phase = p; s.mu.Unlock() }
func (s *State) SetLevels(l, r float64)        { s.mu.Lock(); s.levelL, s.levelR = l, r; s.mu.Unlock() }
func (s *State) SetElapsed(d time.Duration)    { s.mu.Lock(); s.elapsed = d; s.mu.Unlock() }
func (s *State) MarkFinished()                 { s.mu.Lock(); s.finished = true; s.mu.Unlock() }
func (s *State) SetPaused(p bool)              { s.mu.Lock(); s.paused = p; s.mu.Unlock() }
func (s *State) SetMode(m string)              { s.mu.Lock(); s.mode = m; s.mu.Unlock() }
func (s *State) SetRate(hz float64)            { s.mu.Lock(); s.rateHz = hz; s.mu.Unlock() }
func (s *State) SetDepth(d float64)            { s.mu.Lock(); s.depth = d; s.mu.Unlock() }
func (s *State) SetShape(sh string)            { s.mu.Lock(); s.shape = sh; s.mu.Unlock() }
func (s *State) SetVolumeDb(db float64)        { s.mu.Lock(); s.volumeDb = db; s.mu.Unlock() }
func (s *State) SetReverb(on bool)             { s.mu.Lock(); s.reverbOn = on; s.mu.Unlock() }
func (s *State) SetReverbMix(m float64)        { s.mu.Lock(); s.reverbMix = m; s.mu.Unlock() }
func (s *State) SetNoiseMode(m string)         { s.mu.Lock(); s.noiseMode = m; s.mu.Unlock() }
func (s *State) SetNoiseVolume(v float64)      { s.mu.Lock(); s.noiseVolume = v; s.mu.Unlock() }
func (s *State) SetBinauralOn(on bool)         { s.mu.Lock(); s.binauralOn = on; s.mu.Unlock() }
func (s *State) SetBinauralBeat(hz float64)    { s.mu.Lock(); s.binauralBeat = hz; s.mu.Unlock() }
func (s *State) SetDuration(d time.Duration)   { s.mu.Lock(); s.duration = d; s.mu.Unlock() }

func (s *State) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Snapshot{
		Title:        s.title,
		Duration:     s.duration,
		Elapsed:      s.elapsed,
		Finished:     s.finished,
		Paused:       s.paused,
		Mode:         s.mode,
		RateHz:       s.rateHz,
		Depth:        s.depth,
		Shape:        s.shape,
		Pan:          s.pan,
		Phase:        s.phase,
		ReverbOn:     s.reverbOn,
		ReverbMix:    s.reverbMix,
		NoiseMode:    s.noiseMode,
		NoiseVolume:  s.noiseVolume,
		BinauralOn:   s.binauralOn,
		BinauralBeat: s.binauralBeat,
		LevelL:       s.levelL,
		LevelR:       s.levelR,
		VolumeDb:     s.volumeDb,
	}
}
