package state

import (
	"sync"
	"time"
)

// State is shared between the audio goroutine (writer) and the UI goroutine
// (reader). The audio chain pushes pan/level/elapsed; the UI takes snapshots.
type State struct {
	mu sync.RWMutex

	title     string
	mode      string
	rateHz    float64
	volumeDb  float64
	reverbOn  bool
	duration  time.Duration
	elapsed   time.Duration
	pan       float64
	phase     float64
	levelL    float64
	levelR    float64
	finished  bool
	paused    bool
}

type Snapshot struct {
	Title    string
	Mode     string
	RateHz   float64
	VolumeDb float64
	ReverbOn bool
	Duration time.Duration
	Elapsed  time.Duration
	Pan      float64
	Phase    float64
	LevelL   float64
	LevelR   float64
	Finished bool
	Paused   bool
}

func New(title, mode string, rateHz float64, duration time.Duration) *State {
	return &State{title: title, mode: mode, rateHz: rateHz, duration: duration}
}

func (s *State) SetPan(p float64) {
	s.mu.Lock()
	s.pan = p
	s.mu.Unlock()
}

func (s *State) SetPhase(p float64) {
	s.mu.Lock()
	s.phase = p
	s.mu.Unlock()
}

func (s *State) SetLevels(l, r float64) {
	s.mu.Lock()
	s.levelL, s.levelR = l, r
	s.mu.Unlock()
}

func (s *State) SetElapsed(d time.Duration) {
	s.mu.Lock()
	s.elapsed = d
	s.mu.Unlock()
}

func (s *State) MarkFinished() {
	s.mu.Lock()
	s.finished = true
	s.mu.Unlock()
}

func (s *State) SetPaused(p bool) {
	s.mu.Lock()
	s.paused = p
	s.mu.Unlock()
}

func (s *State) SetMode(mode string) {
	s.mu.Lock()
	s.mode = mode
	s.mu.Unlock()
}

func (s *State) SetRate(hz float64) {
	s.mu.Lock()
	s.rateHz = hz
	s.mu.Unlock()
}

func (s *State) SetVolumeDb(db float64) {
	s.mu.Lock()
	s.volumeDb = db
	s.mu.Unlock()
}

func (s *State) SetReverb(on bool) {
	s.mu.Lock()
	s.reverbOn = on
	s.mu.Unlock()
}

func (s *State) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Snapshot{
		Title:    s.title,
		Mode:     s.mode,
		RateHz:   s.rateHz,
		VolumeDb: s.volumeDb,
		ReverbOn: s.reverbOn,
		Duration: s.duration,
		Elapsed:  s.elapsed,
		Pan:      s.pan,
		Phase:    s.phase,
		LevelL:   s.levelL,
		LevelR:   s.levelR,
		Finished: s.finished,
		Paused:   s.paused,
	}
}
