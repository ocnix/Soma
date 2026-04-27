package player

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gopxl/beep/v2"
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
	close func()
}

func (s *Session) Close() {
	if s.close != nil {
		s.close()
	}
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
	st := state.New(filepath.Base(path), mode, rateHz, duration)

	var chain beep.Streamer = stream
	if !dry {
		chain = effect.NewEightD(chain, float64(format.SampleRate), rateHz, st)
	}
	chain = effect.NewMeter(chain, st)

	done := make(chan struct{})
	speaker.Play(beep.Seq(chain, beep.Callback(func() {
		st.MarkFinished()
		close(done)
	})))

	// Track elapsed by polling decoder position.
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

	closeFn := func() {
		close(stopPoll)
		speaker.Clear()
		_ = stream.Close()
		_ = f.Close()
		speaker.Close()
	}

	return &Session{State: st, Done: done, close: closeFn}, nil
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
