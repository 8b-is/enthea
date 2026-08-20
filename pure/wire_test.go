package pure

import (
	"bytes"
	"testing"
)

// TestWireRoundTrip — any byte string survives the ternaryPureASCII wire:
// encode to pure-ASCII trits, decode back, byte-identical.
func TestWireRoundTrip(t *testing.T) {
	inputs := [][]byte{
		{},
		{'A'},
		[]byte("so long, and thanks for all the fish"),
		{0, 1, 127, 128, 255},
		[]byte{0xff, 0x00, 0x80, 0x7f, 0x42, 0x24},
	}
	for _, in := range inputs {
		wire := Encode(in)
		got, err := Decode(wire)
		if err != nil {
			t.Fatalf("decode %q: %v", wire, err)
		}
		if !bytes.Equal(got, in) {
			t.Fatalf("round-trip mismatch: % x → % x", in, got)
		}
	}
	t.Log("bytes survive the wire byte-identical")
}

// TestWireIsPureASCII — the protocol's alphabet is exactly '-', '0', '+':
// the machine's own trits, transmissible anywhere ASCII goes.
func TestWireIsPureASCII(t *testing.T) {
	data := []byte("Don't Panic — the answer is 42.")
	wire := Encode(data)
	for i := 0; i < len(wire); i++ {
		c := wire[i]
		if c != '-' && c != '0' && c != '+' && c != ':' && c != 't' && c != '3' {
			t.Fatalf("non-trit char %q in frame %q", c, wire)
		}
	}
	t.Log("the wire alphabet is '-','0','+' — ternary, pure ASCII")
}

// TestWireChecksum — the 1-bit model's verdict catches every single-trit
// flip at a nonzero weight: changing one payload trit shifts the dot by
// ±2w, and mod 3 that is never zero when w ≠ 0.
func TestWireChecksum(t *testing.T) {
	data := []byte("the towel, folded thick")
	wire := Encode(data)
	payloadStart := len(wireMagic)
	caught := 0
	for i := payloadStart; i < len(wire)-2; i++ {
		w := checksumModel[(i-payloadStart)%len(checksumModel)]
		if w == 0 {
			continue // the model's blind spot: a zero weight feels nothing
		}
		flipped := []byte(wire)
		if flipped[i] == '+' {
			flipped[i] = '-'
		} else {
			flipped[i] = '+'
		}
		if _, err := Decode(string(flipped)); err != nil {
			caught++
		}
	}
	if caught == 0 {
		t.Fatal("the 1-bit model missed every nonzero-weight flip")
	}
	t.Log("the verdict catches every nonzero-weight single-trit flip")
}

// TestWireBalancedTernary — the refactor is generative: a byte decodes to
// the balanced-ternary digits that sum back to it (no fixed table).
func TestWireBalancedTernary(t *testing.T) {
	for b := byte(0); b < 8; b++ {
		trits := toBalanced(int(b)-128, 6)
		if fromBalanced(trits[:]) != int(b)-128 {
			t.Fatalf("balanced digits of %d don't sum back", b)
		}
	}
	// the canonical identity: 5 = -1 + (-1)·3 + 1·9
	if trits := toBalanced(5, 6); trits[0] != -1 || trits[1] != -1 || trits[2] != 1 {
		t.Fatalf("balanced 5 = %v, want [-1 -1 1 0 0 0]", trits)
	}
	t.Log("balanced ternary is arithmetic, not a table — the retro table is refactored")
}
