// Package dsp provides small DSP helpers used by the audio chain.
package dsp

import "math"

// FFT computes an in-place radix-2 Cooley-Tukey FFT. The input arrays must
// have the same length and that length must be a power of two. re and im are
// the real and imaginary parts.
func FFT(re, im []float64) {
	n := len(re)
	// Bit-reversal permutation.
	j := 0
	for i := 1; i < n; i++ {
		bit := n / 2
		for j&bit != 0 {
			j ^= bit
			bit /= 2
		}
		j ^= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	// Butterflies.
	for size := 2; size <= n; size *= 2 {
		half := size / 2
		angle := -2 * math.Pi / float64(size)
		wRealStep := math.Cos(angle)
		wImagStep := math.Sin(angle)
		for i := 0; i < n; i += size {
			wr, wi := 1.0, 0.0
			for k := 0; k < half; k++ {
				aRe, aIm := re[i+k], im[i+k]
				bRe := re[i+k+half]*wr - im[i+k+half]*wi
				bIm := re[i+k+half]*wi + im[i+k+half]*wr
				re[i+k] = aRe + bRe
				im[i+k] = aIm + bIm
				re[i+k+half] = aRe - bRe
				im[i+k+half] = aIm - bIm
				wr, wi = wr*wRealStep-wi*wImagStep, wr*wImagStep+wi*wRealStep
			}
		}
	}
}

// HannWindow fills w with a Hann window of length len(w). Multiplying input
// samples by this before FFT reduces spectral leakage.
func HannWindow(w []float64) {
	n := len(w)
	if n == 0 {
		return
	}
	for i := 0; i < n; i++ {
		w[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n-1))
	}
}
