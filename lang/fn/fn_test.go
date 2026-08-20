package fn

import (
	"testing"

	"github.com/8b-is/enthea/lang"
)

func compileRun(t *testing.T, src string) (lang.Cell, *lang.VM) {
	t.Helper()
	fns, main, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	prog, err := Compile(fns, main)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	vm, err := lang.NewVM(prog, 4096)
	if err != nil {
		t.Fatalf("newvm: %v", err)
	}
	t.Cleanup(func() { vm.Arena().Close() })
	if err := vm.Run(200000); err != nil {
		t.Fatalf("run: %v", err)
	}
	return vm.Regs()[0], vm
}

// TestPureLetters — the sixteen letters composed purely, against the
// machine's tritwise semantics: xor(and(1,1), or(nota(1),1)) = 12, because
// ¬[1,0,0] = [0,1,1] = 12 and or(12,1) = [1,1,1] = 13, then xor(1,13) = 12.
func TestPureLetters(t *testing.T) {
	src := `let a = 1 in let b = 1 in xor(and(a, b), or(nota(a), b))`
	got, _ := compileRun(t, src)
	if got != 12 {
		t.Fatalf("xor(and(1,1), or(nota(1),1)) = %d, want 12 (tritwise)", got)
	}
	t.Log("the letters compose purely, tritwise — the machine and the shrine agree")
}

// TestUltraClassifier — the flagship: a real ultra classifier as a pure
// expression. Weights [1,-1,1,1] dot [1,0,-1,1] = 1 → ultra → +1.
func TestUltraClassifier(t *testing.T) {
	src := `let x0 = 1 in let x1 = 0 in let x2 = -1 in let x3 = 1 in
ultra(add(mul(x0, 1), add(mul(x1, -1), add(mul(x2, 1), mul(x3, 1)))))`
	got, _ := compileRun(t, src)
	if got != 1 {
		t.Fatalf("ultra(dot) = %d, want 1", got)
	}
	t.Log("the ultra classifier runs as a pure expression over letters and ternary arithmetic")
}

// TestFactorialFn — recursion in the pure language: fact(3) = 6, computed by
// the machine on its own arena.
func TestFactorialFn(t *testing.T) {
	src := `fn fact(n) = if(iszero(n), 1, mul(n, fact(sub(n, 1))))
fact(3)`
	got, _ := compileRun(t, src)
	if got != 6 {
		t.Fatalf("fact(3) = %d, want 6", got)
	}
	t.Log("fact(3)=6 — pure recursion lowered to the arena's own call stack")
}

// TestTailRecursionFn — tail recursion is a jump: 13 levels of tail calls
// use a constant stack (the assembler turned the last `call` into `jmp`).
func TestTailRecursionFn(t *testing.T) {
	src := `fn count(n) = if(iszero(n), 0, count(sub(n, 1)))
count(13)`
	got, vm := compileRun(t, src)
	if got != 0 {
		t.Fatalf("count(13) = %d, want 0", got)
	}
	if vm.StackPeak() > 2 {
		t.Fatalf("tail recursion grew the stack: peak %d", vm.StackPeak())
	}
	t.Log("13 tail-recursive iterations on a constant stack — pure TCO")
}
