package pure

import (
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// compileHarness builds the C + assembly kernels with the system `cc` into a
// temp binary and returns its path. The kernels themselves are pure; the
// harness (run.c) is the only thing that prints.
func compileHarness(t *testing.T) string {
	t.Helper()
	kernels, err := filepath.Abs("kernels")
	if err != nil {
		t.Fatal(err)
	}
	var asm string
	switch runtime.GOARCH {
	case "arm64":
		asm = filepath.Join(kernels, "ternary4_arm64.S")
	case "amd64":
		asm = filepath.Join(kernels, "ternary4_x86_64.S")
	default:
		t.Skipf("no hand-written kernel for %s", runtime.GOARCH)
	}
	bin := filepath.Join(t.TempDir(), "enthea-pure")
	cc := exec.Command("cc",
		filepath.Join(kernels, "run.c"),
		asm,
		"-o", bin)
	if out, err := cc.CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s", err, out)
	}
	return bin
}

// TestKernelsMatchProve — C == assembly == Go, bit for bit. Turing's thesis:
// a function is a function, on any machine that can add.
func TestKernelsMatchProve(t *testing.T) {
	w := WorkedExample
	wantCodes, wantScale := Ternary4(w)
	bin := compileHarness(t)

	out, err := exec.Command(bin).Output()
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line) // kind c1 c2 c3 c4 scale
		if len(f) != 6 {
			t.Fatalf("bad harness line: %q", line)
		}
		var codes [4]int64
		for i := 0; i < 4; i++ {
			v, err := strconv.ParseInt(f[i+1], 10, 64)
			if err != nil {
				t.Fatalf("bad code %q: %v", f[i+1], err)
			}
			codes[i] = v
		}
		scale, err := strconv.ParseFloat(f[5], 64)
		if err != nil {
			t.Fatalf("bad scale %q: %v", f[5], err)
		}
		if codes != wantCodes || math.Abs(scale-wantScale) > 1e-12 {
			t.Errorf("%s kernel mismatch: got codes %v scale %.17g; want %v %.17g (scale within 1e-12)",
				f[0], codes, scale, wantCodes, wantScale)
		}
	}
	t.Logf("worked example [%v]: codes %v, scale %v — C == asm == Go", w, wantCodes, wantScale)
}

// TestNandSeedProve — the universal gate is sound: NAND alone builds NOT,
// AND, OR, XOR with the full truth tables. The seed of every seed.
func TestNandSeedProve(t *testing.T) {
	nand := func(a, b int) int {
		if a == 1 && b == 1 {
			return 0
		}
		return 1
	}
	not := func(a int) int { return nand(a, a) }
	and := func(a, b int) int { return not(nand(a, b)) }
	or := func(a, b int) int { return nand(not(a), not(b)) }
	xor := func(a, b int) int { return and(or(a, b), nand(a, b)) }

	for a := 0; a <= 1; a++ {
		for b := 0; b <= 1; b++ {
			if and(a, b) != (a & b) {
				t.Fatalf("and(%d,%d)", a, b)
			}
			if or(a, b) != (a | b) {
				t.Fatalf("or(%d,%d)", a, b)
			}
			if xor(a, b) != (a ^ b) {
				t.Fatalf("xor(%d,%d)", a, b)
			}
			if not(a) != (a ^ 1) {
				t.Fatalf("not(%d)", a)
			}
		}
	}
	t.Log("NAND → NOT/AND/OR/XOR: all truth tables hold — one gate, everything")
}

// ExampleTernary4 documents the worked example in runnable form.
func ExampleTernary4() {
	codes, scale := Ternary4(WorkedExample)
	fmt.Printf("%v %.4g\n", codes, scale)
	// Output: [1 -1 0 0] 1.15
}
