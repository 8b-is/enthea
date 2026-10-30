// The triangle as a Peirce triad. A triangle wave is not just a signal: it
// is semiosis made sample-exact. The continuous phase is the object (O); the
// signed int16 samples are the sign (S); the ternary codes the quantizer
// yields are the interpretant (I). Meaning is the triad, not any one term.
//
// The sign must be a *well-formed* sign: a proper signed triangle, spanning
// [-A, A] with no MSB flip. An unsigned 0x0000..0xFFFF sweep is a broken
// sign — its upper half is read as negative, so the wave is cut in half and
// violently swapped at every sign-bit crossing. That is the fracture this
// generator refuses.
package pure

// TriadAmp is the triangle's peak: 32767, never 32768. int16 is asymmetric
// (+32767 / -32768); touching the asymmetric extreme would break the sign's
// symmetry and mislead the interpretant. The peak stays at the symmetric
// bound.
const TriadAmp = 32767

// Triangle16 yields `samples` signed int16 samples of a triangle wave with
// `cycles` full periods. The phase is a rational in [0,1); each sample is
//
//	phase < 1/4  ->  +4*phase        (rise to the +peak)
//	phase < 3/4  ->  2 - 4*phase     (fall through zero to the -peak)
//	else         ->  4*phase - 4     (rise back to zero)
//
// A piecewise-linear ramp has no instantaneous jumps: every adjacent-sample
// delta is bounded by the per-sample slope, so there is no MSB flip, no
// phase-wrap crack, no vertical wall of energy.
func Triangle16(samples, cycles int) []int16 {
	out := make([]int16, samples)
	if samples <= 0 || cycles <= 0 {
		return out
	}
	for i := 0; i < samples; i++ {
		// phase = (cycles * i) % samples / samples
		// We use integer arithmetic to calculate the numerator of the fractional part.
		// num = (cycles * i) % samples
		// v = 4 * (num/samples) or similar. To avoid floats, we scale by TriadAmp.
		// Let's use a scale of 1,000,000 for the phase calculation to maintain precision,
		// or better, use the exact rational logic.
		num := (cycles * i) % samples
		// To avoid float, we calculate the value in a scaled integer space.
		// We want v * TriadAmp. Since v is based on 4 * (num/samples),
		// v * TriadAmp = 4 * num * TriadAmp / samples.
		// We must handle the piecewise logic with integer comparisons.
		// p < 0.25  =>  num/samples < 1/4  =>  4*num < samples
		// p < 0.75  =>  num/samples < 3/4  =>  4*num < 3*samples
		
		var val int64
		fourNum := 4 * int64(num)
		samples64 := int64(samples)

		if fourNum < samples64 {
			// v = 4 * num / samples
			val = (fourNum * TriadAmp) / samples64
		} else if fourNum < 3*samples64 {
			// v = 2 - 4*num/samples = (2*samples - 4*num) / samples
			val = ((2*samples64 - fourNum) * TriadAmp) / samples64
		} else {
			// v = 4*num/samples - 4 = (4*num - 4*samples) / samples
			val = ((fourNum - 4*samples64) * TriadAmp) / samples64
		}
		out[i] = int16(val)
	}
	return out
}

// Triangle16Signed is the same wave with the object phase alongside the
// sign — the (O, S) pair, kept together so the interpretation has a ground.
func Triangle16Signed(samples, cycles int) (phase []float64, sign []int16) {
	sign = Triangle16(samples, cycles)
	phase = make([]float64, len(sign))
	for i := range sign {
		p := float64(cycles) * float64(i) / float64(samples)
		phase[i] = p - float64(int(p))
	}
	return
}

// Interpret passes the sign through Ternary4 in groups of four, producing
// the interpretant: one ternary code per group. The triad is complete —
// object (phase), sign (samples), interpretant (codes) — and the codes must
// agree with the sign's polarity wherever the sign is strong enough to mean.
func Interpret(sign []int16) []int64 {
	codes := make([]int64, 0, (len(sign)+3)/4)
	for i := 0; i < len(sign); i += 4 {
		var w [4]float64
		for j := 0; j < 4 && i+j < len(sign); j++ {
			w[j] = float64(sign[i+j])
		}
		c, _ := Ternary4(w)
		codes = append(codes, c[:]...)
	}
	return codes
}
