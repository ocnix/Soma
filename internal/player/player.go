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
	ctrl       *beep.Ctrl
	eightd     *effect.EightD
	reverb     *effect.Reverb
	volume     *effects.Volume
}

// db ↔ log2 conversion (beep's effects.Volume is log-based, base 2).
const db2Log = 1.0 / 6.020599913 // 1 / (20 * log10(2))

func dbToLog(db float64) float64 { return db * db2Log }

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

func (s *Session) OnClose(fn func()) {
	s.onClose = append(s.onClose, fn)
}

func (s *Session) TogglePause() bool {
	speaker.Lock()
	defer speaker.Unlock()
	s.ctrl.Paused = !s.ctrl.Paused
	s.State.SetPaused(s.ctrl.Paused)
	return s.ctrl.Paused
}

// AdjustVolume moves the master volume by deltaDb. Range is clamped to
// [-60, +6] dB; below -50 dB, the stream is muted entirely.
func (s *Session) AdjustVolume(deltaDb float64) {
	speaker.Lock()
	defer speaker.Unlock()

	cur := s.volume.Volume / db2Log
	cur += deltaDb
	if cur < -60 {
		cur = -60
	}
	if cur > 6 {
		cur = 6
	}
	s.volume.Volume = dbToLog(cur)
	s.volume.Silent = cur <= -50
	s.State.SetVolumeDb(cur)
}

// SeekRelative moves the play position by d (positive = forward).
func (s *Session) SeekRelative(d time.Duration) {
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

// AdjustRate changes the 8D LFO frequency by deltaHz.
func (s *Session) AdjustRate(deltaHz float64) {
	if s.eightd == nil {
		return
	}
	cur := s.State.Snapshot().RateHz
	s.eightd.SetRate(cur + deltaHz)
}

func (s *Session) ToggleEffect() {
	if s.eightd == nil {
		return
	}
	dry := s.State.Snapshot().Mode == "8D"
	s.eightd.SetBypass(dry)
}

func (s *Session) ToggleReverb() {
	if s.reverb == nil {
		return
	}
	on := !s.State.Snapshot().ReverbOn
	s.reverb.SetBypass(!on)
	s.State.SetReverb(on)
}

// Start begins playback in the speaker goroutine and returns immediately.
func Start(path string, rateHz float64, dry bool) (*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	stream, format, err := decode(f, strings.ToLower(filepath.Ext(path)))
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("decode: %w", err)
	}

	if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
		stream.Close()
		f.Close()
		return nil, fmt.Errorf("speaker init: %w", err)
	}

	mode := "8D"
	if dry {
		mode = "dry"
	}
	duration := format.SampleRate.D(stream.Len())
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	st := state.New(title, mode, rateHz, duration)
	st.SetReverb(true)

	var chain beep.Streamer = stream
	eightD := effect.NewEightD(chain, float64(format.SampleRate), rateHz, st)
	eightD.SetBypass(dry)
	chain = eightD

	reverb := effect.NewReverb(chain, float64(format.SampleRate), 0.18)
	chain = reverb

	chain = effect.NewMeter(chain, st)

	vol := &effects.Volume{Streamer: chain, Base: 2, Volume: 0}
	chain = vol

	ctrl := &beep.Ctrl{Streamer: chain}

	done := make(chan struct{})
	speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
		st.MarkFinished()
		close(done)
	})))

	stopPoll := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopPoll:
				return
			case <-done:
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
		speaker.Clear()
		_ = stream.Close()
		_ = f.Close()
		speaker.Close()
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
		volume:     vol,
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
