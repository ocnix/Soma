package player

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"

	"soma/internal/effect"
	"soma/internal/state"
)

// Config bundles the initial parameters for a session. Comes from the saved
// profile on disk + any CLI overrides.
type Config struct {
	Rate         float64
	Depth        float64
	Shape        string // "sine" | "random"
	Dry          bool
	ReverbOn     bool
	ReverbMix    float64
	NoiseMode    string  // "off" | "white" | "pink" | "brown"
	NoiseVolume  float64 // 0..1
	BinauralOn   bool
	BinauralBeat float64
	Crossfeed    float64
	VolumeDb     float64
}

// Session is a running playback. State is the shared snapshot the UI reads.
// Done closes when the track finishes naturally. Close releases resources.
type Session struct {
	State *state.State
	Done  <-chan struct{}

	close   func()
	onClose []func()
	mu      sync.Mutex
	closed  bool

	stream     beep.StreamSeekCloser
	sampleRate beep.SampleRate
	live       bool

	ctrl      *beep.Ctrl
	eightd    *effect.EightD
	reverb    *effect.Reverb
	noise     *effect.Noise
	binaural  *effect.Binaural
	crossfeed *effect.Crossfeed
	volume    *effects.Volume

	startedAt time.Time
}

// SetCrossfeed adjusts the headphone crossfeed amount (0..1).
func (s *Session) SetCrossfeed(v float64) {
	if s.crossfeed == nil {
		return
	}
	s.crossfeed.SetAmount(v)
	s.State.SetCrossfeed(v)
}

// IsLive reports whether the session is a live (un-seekable, no-duration)
// stream rather than a finite track.
func (s *Session) IsLive() bool { return s.live }

// db ↔ log2 conversion (beep's effects.Volume is log-based, base 2).
const db2Log = 1.0 / 6.020599913 // 1 / (20 * log10(2))

func dbToLog(db float64) float64 { return db * db2Log }

// SpeakerSampleRate is the fixed rate the speaker is initialized at. All
// tracks are resampled to this rate so we only call speaker.Init once for
// the whole process. 48 kHz matches the macOS CoreAudio DAC native rate,
// avoiding a second resample stage in the OS audio path.
const SpeakerSampleRate beep.SampleRate = 48000

// ResampleQuality is the order of beep.Resample's polyphase filter.
// 4 is the beep default; 6 is closer to "audiophile" with negligible CPU.
const ResampleQuality = 6

var (
	speakerOnce    sync.Once
	speakerInitErr error
)

func ensureSpeaker() error {
	speakerOnce.Do(func() {
		speakerInitErr = speaker.Init(SpeakerSampleRate, SpeakerSampleRate.N(time.Second/10))
	})
	return speakerInitErr
}

func (s *Session) StartedAt() time.Time { return s.startedAt }

func (s *Session) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()

	if s.close != nil {
		s.close()
	}
	for _, fn := range s.onClose {
		fn()
	}
}

func (s *Session) OnClose(fn func()) { s.onClose = append(s.onClose, fn) }

// ── Pause / seek / volume ────────────────────────────────────────────────────

func (s *Session) TogglePause() bool {
	speaker.Lock()
	defer speaker.Unlock()
	s.ctrl.Paused = !s.ctrl.Paused
	s.State.SetPaused(s.ctrl.Paused)
	return s.ctrl.Paused
}

func (s *Session) SetPaused(p bool) {
	speaker.Lock()
	defer speaker.Unlock()
	s.ctrl.Paused = p
	s.State.SetPaused(p)
}

func (s *Session) AdjustVolume(deltaDb float64) {
	s.SetVolume(s.State.Snapshot().VolumeDb + deltaDb)
}

func (s *Session) SetVolume(db float64) {
	speaker.Lock()
	defer speaker.Unlock()
	if db < -60 {
		db = -60
	}
	if db > 6 {
		db = 6
	}
	s.volume.Volume = dbToLog(db)
	s.volume.Silent = db <= -50
	s.State.SetVolumeDb(db)
}

func (s *Session) SeekRelative(d time.Duration) {
	if s.live {
		return
	}
	speaker.Lock()
	defer speaker.Unlock()
	pos := s.stream.Position() + s.sampleRate.N(d)
	if pos < 0 {
		pos = 0
	}
	if pos >= s.stream.Len() {
		pos = s.stream.Len() - 1
	}
	_ = s.stream.Seek(pos)
	s.State.SetElapsed(s.sampleRate.D(pos))
}

func (s *Session) SeekFraction(f float64) {
	if s.live {
		return
	}
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	speaker.Lock()
	defer speaker.Unlock()
	pos := int(float64(s.stream.Len()) * f)
	if pos < 0 {
		pos = 0
	}
	if pos >= s.stream.Len() {
		pos = s.stream.Len() - 1
	}
	_ = s.stream.Seek(pos)
	s.State.SetElapsed(s.sampleRate.D(pos))
}

// ── 8D controls ──────────────────────────────────────────────────────────────

func (s *Session) AdjustRate(deltaHz float64) {
	if s.eightd == nil {
		return
	}
	s.eightd.SetRate(s.State.Snapshot().RateHz + deltaHz)
}

func (s *Session) SetRate(hz float64) {
	if s.eightd != nil {
		s.eightd.SetRate(hz)
	}
}

func (s *Session) SetDepth(d float64) {
	if s.eightd != nil {
		s.eightd.SetDepth(d)
	}
}

func (s *Session) SetShape(name string) {
	if s.eightd != nil {
		s.eightd.SetShape(effect.ShapeFromString(name))
	}
}

func (s *Session) ToggleEffect() {
	if s.eightd == nil {
		return
	}
	dry := s.State.Snapshot().Mode == "8D"
	s.eightd.SetBypass(dry)
}

// ── Reverb ───────────────────────────────────────────────────────────────────

func (s *Session) ToggleReverb() {
	if s.reverb == nil {
		return
	}
	on := !s.State.Snapshot().ReverbOn
	s.reverb.SetBypass(!on)
	s.State.SetReverb(on)
}

func (s *Session) SetReverbMix(m float64) {
	if s.reverb == nil {
		return
	}
	s.reverb.SetMix(m)
	s.State.SetReverbMix(m)
}

// ── Noise ────────────────────────────────────────────────────────────────────

func (s *Session) CycleNoise() {
	if s.noise == nil {
		return
	}
	cur := effect.NoiseFromString(s.State.Snapshot().NoiseMode)
	next := effect.NoiseMode((int(cur) + 1) % 13)
	s.noise.SetMode(next)
	s.State.SetNoiseMode(next.String())
}

func (s *Session) SetNoiseMode(name string) {
	if s.noise == nil {
		return
	}
	m := effect.NoiseFromString(name)
	s.noise.SetMode(m)
	s.State.SetNoiseMode(m.String())
}

func (s *Session) AdjustNoiseVolume(delta float64) {
	if s.noise == nil {
		return
	}
	cur := s.State.Snapshot().NoiseVolume + delta
	if cur < 0 {
		cur = 0
	}
	if cur > 1 {
		cur = 1
	}
	s.noise.SetVolume(cur)
	s.State.SetNoiseVolume(cur)
}

func (s *Session) SetNoiseVolume(v float64) {
	if s.noise == nil {
		return
	}
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	s.noise.SetVolume(v)
	s.State.SetNoiseVolume(v)
}

// ── Binaural ─────────────────────────────────────────────────────────────────

func (s *Session) ToggleBinaural() {
	if s.binaural == nil {
		return
	}
	on := !s.State.Snapshot().BinauralOn
	s.binaural.SetEnabled(on)
	s.State.SetBinauralOn(on)
}

func (s *Session) SetBinauralBeat(hz float64) {
	if s.binaural == nil {
		return
	}
	s.binaural.SetBeat(hz)
	s.State.SetBinauralBeat(hz)
}

// ── Start ────────────────────────────────────────────────────────────────────

// Start begins playback in the speaker goroutine and returns immediately.
func Start(path string, cfg Config) (*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	stream, format, err := decode(f, strings.ToLower(filepath.Ext(path)))
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("decode: %w", err)
	}

	if err := ensureSpeaker(); err != nil {
		stream.Close()
		f.Close()
		return nil, fmt.Errorf("speaker init: %w", err)
	}

	mode := "8D"
	if cfg.Dry {
		mode = "dry"
	}
	duration := format.SampleRate.D(stream.Len())
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	st := state.New(title, mode, cfg.Rate, duration)
	st.SetDepth(cfg.Depth)
	st.SetShape(cfg.Shape)
	st.SetReverb(cfg.ReverbOn)
	st.SetReverbMix(cfg.ReverbMix)
	st.SetNoiseMode(cfg.NoiseMode)
	st.SetNoiseVolume(cfg.NoiseVolume)
	st.SetBinauralOn(cfg.BinauralOn)
	st.SetBinauralBeat(cfg.BinauralBeat)
	st.SetCrossfeed(cfg.Crossfeed)
	st.SetVolumeDb(cfg.VolumeDb)

	// Resample to the speaker's fixed rate if the track's native rate differs.
	var music beep.Streamer = stream
	if format.SampleRate != SpeakerSampleRate {
		music = beep.Resample(ResampleQuality, format.SampleRate, SpeakerSampleRate, music)
	}

	// All downstream effects operate at the speaker's rate.
	chainSR := float64(SpeakerSampleRate)
	eightD := effect.NewEightD(music, chainSR, cfg.Rate, cfg.Depth, effect.ShapeFromString(cfg.Shape), st)
	eightD.SetBypass(cfg.Dry)
	music = eightD

	reverb := effect.NewReverb(music, chainSR, cfg.ReverbMix)
	reverb.SetBypass(!cfg.ReverbOn)
	music = reverb

	noise := effect.NewNoise(effect.NoiseFromString(cfg.NoiseMode), cfg.NoiseVolume)
	noise.SetSampleRate(chainSR)
	binaural := effect.NewBinaural(chainSR, cfg.BinauralBeat, 0.10)
	binaural.SetEnabled(cfg.BinauralOn)

	doneOnce := sync.Once{}
	done := make(chan struct{})
	closeDone := func() {
		doneOnce.Do(func() {
			st.MarkFinished()
			close(done)
		})
	}

	// Music with end-of-track callback. When the decoder is exhausted, Seq
	// fires the callback exactly once, and the Mixer auto-removes the
	// finished streamer. Noise + binaural keep going until Session.Close.
	musicWithEnd := beep.Seq(music, beep.Callback(closeDone))

	mixer := &beep.Mixer{}
	mixer.Add(musicWithEnd)
	mixer.Add(noise)
	mixer.Add(binaural)

	spectrum := effect.NewSpectrum(mixer, chainSR, 1024, 32, st)
	metered := effect.NewMeter(spectrum, st)

	cross := effect.NewCrossfeed(metered, chainSR, cfg.Crossfeed)

	// Pre-volume headroom of -1.5 dB so the limiter rarely engages on
	// well-mastered material; tanh limiter catches the rest.
	const headroomDb = -1.5
	headroom := &effects.Volume{Streamer: cross, Base: 2, Volume: dbToLog(headroomDb)}

	vol := &effects.Volume{Streamer: headroom, Base: 2, Volume: dbToLog(cfg.VolumeDb)}
	if cfg.VolumeDb <= -50 {
		vol.Silent = true
	}

	limited := effect.NewLimiter(vol, 0.95)

	ctrl := &beep.Ctrl{Streamer: limited}

	speaker.Play(ctrl)

	stopPoll := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopPoll:
				return
			case <-ticker.C:
				speaker.Lock()
				pos := stream.Position()
				speaker.Unlock()
				st.SetElapsed(format.SampleRate.D(pos))
			}
		}
	}()

	closed := false
	closeFn := func() {
		if closed {
			return
		}
		closed = true
		close(stopPoll)
		closeDone()
		// speaker.Clear removes our streamers from the package-level mixer.
		// We deliberately do NOT call speaker.Close — beep can't re-init,
		// and the docs explicitly recommend leaving it open for app lifetime.
		speaker.Clear()
		_ = stream.Close()
		_ = f.Close()
	}

	return &Session{
		State:      st,
		Done:       done,
		close:      closeFn,
		stream:     stream,
		sampleRate: format.SampleRate,
		ctrl:       ctrl,
		eightd:     eightD,
		reverb:     reverb,
		noise:      noise,
		binaural:   binaural,
		crossfeed:  cross,
		volume:     vol,
		startedAt:  time.Now(),
	}, nil
}

func decode(f *os.File, ext string) (beep.StreamSeekCloser, beep.Format, error) {
	switch ext {
	case ".mp3":
		return mp3.Decode(f)
	case ".wav":
		return wav.Decode(f)
	case ".flac":
		return flac.Decode(f)
	default:
		return nil, beep.Format{}, fmt.Errorf("unsupported format %q (try mp3, wav, flac)", ext)
	}
}
