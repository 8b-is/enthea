// Package pure — the Turing shrine. One side-effect-free kernel written in
// Go (the oracle), C, and hand-written assembly; the test proves they all
// produce bit-identical output. No cgo in this package: the C and assembly
// are compiled by the test harness with the system `cc`, keeping the shipped
// module 100% pure Go.
package pure

import "math"

// Ternary4 is the Go oracle for the pure kernels: the BitNet b1.58 group
// quantize on four weights.
//
//	scale = max(mean(|w|), 1e-7)
//	codes[i] = round-half-away-from-zero(clamp(w[i]/scale, -1, 1))
func Ternary4(w [4]float64) (codes [4]int64, scale float64) {
	var sum float64
	for _, x := range w {
		sum += math.Abs(x)
	}
	scale = sum / 4
	if scale < 1e-7 {
		scale = 1e-7
	}
	for i, x := range w {
		v := x / scale
		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
		codes[i] = int64(math.Round(v))
	}
	return
}

// WorkedExample is the session's canonical case: scale 1.15, codes {1,-1,0,0}.
var WorkedExample = [4]float64{3.0, -1.0, 0.2, -0.4}
