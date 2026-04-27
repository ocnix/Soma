package effect

import (
	"math"
	"sync"

	"github.com/gopxl/beep/v2"

	"soma/internal/dsp"
	"soma/internal/state"
)

// Spectrum is a passthrough streamer that taps the audio, runs an FFT every
// FFTSize samples, and pushes log-spaced band magnitudes to State for the UI
// to render.
type Spectrum struct {
	src        beep.Streamer
	sampleRate float64
	st         *state.State

	fftSize  int
	numBands int

	mu     sync.Mutex
	buf    []float64 // ring of mono samples
	bufPos int

	window []float64
	bands  []float64 // smoothed output
}

func NewSpectrum(src beep.Streamer, sampleRate float64, fftSize, numBands int, st *state.State) *Spectrum {
	if fftSize <= 0 {
		fftSize = 1024
	}
	// Force power of two.
	if fftSize&(fftSize-1) != 0 {
		fftSize = 1024
	}
	if numBands <= 0 {
		numBands = 32
	}

	s := &Spectrum{
		src:        src,
		sampleRate: sampleRate,
		st:         st,
		fftSize:    fftSize,
		numBands:   numBands,
		buf:        make([]float64, fftSize),
		window:     make([]float64, fftSize),
		bands:      make([]float64, numBands),
	}
	dsp.HannWindow(s.window)
	return s
}

func (s *Spectrum) Stream(samples [][2]float64) (int, bool) {
	n, ok := s.src.Stream(samples)
	if n == 0 {
		return n, ok
	}
	for i := 0; i < n; i++ {
		s.buf[s.bufPos] = (samples[i][0] + samples[i][1]) * 0.5
		s.bufPos++
		if s.bufPos >= s.fftSize {
			s.bufPos = 0
			s.computeFFT()
		}
	}
	return n, ok
}

func (s *Spectrum) Err() error { return s.src.Err() }

func (s *Spectrum) computeFFT() {
	re := make([]float64, s.fftSize)
	im := make([]float64, s.fftSize)
	// Read the ring buffer in temporal order, applying a Hann window.
	for i := 0; i < s.fftSize; i++ {
		idx := (s.bufPos + i) % s.fftSize
		re[i] = s.buf[idx] * s.window[i]
	}

	dsp.FFT(re, im)

	half := s.fftSize / 2
	mags := make([]float64, half)
	for i := 0; i < half; i++ {
		mags[i] = math.Sqrt(re[i]*re[i]+im[i]*im[i]) / float64(half)
	}

	// Log-spaced bands. Skip DC; cap at Nyquist.
	minBin := 1.0
	maxBin := float64(half)
	out := make([]float64, s.numBands)
	for b := 0; b < s.numBands; b++ {
		lo := int(minBin * math.Pow(maxBin/minBin, float64(b)/float64(s.numBands)))
		hi := int(minBin * math.Pow(maxBin/minBin, float64(b+1)/float64(s.numBands)))
		if hi <= lo {
			hi = lo + 1
		}
		if hi > half {
			hi = half
		}
		var sum float64
		for k := lo; k < hi; k++ {
			sum += mags[k]
		}
		avg := sum / float64(hi-lo)

		// Convert to perceptual 0..1 via dB-like scaling.
		if avg < 1e-6 {
			avg = 1e-6
		}
		db := 20 * math.Log10(avg)
		norm := (db + 60) / 60
		if norm < 0 {
			norm = 0
		}
		if norm > 1 {
			norm = 1
		}
		out[b] = norm
	}

	// Smoothing: fast attack, slow release per band.
	const attack = 0.6
	const release = 0.18
	s.mu.Lock()
	for b := 0; b < s.numBands; b++ {
		cur := s.bands[b]
		next := out[b]
		if next > cur {
			cur = cur + (next-cur)*attack
		} else {
			cur = cur * (1 - release)
		}
		s.bands[b] = cur
	}
	bandsCopy := make([]float64, s.numBands)
	copy(bandsCopy, s.bands)
	s.mu.Unlock()

	if s.st != nil {
		s.st.SetBands(bandsCopy)
	}
}
