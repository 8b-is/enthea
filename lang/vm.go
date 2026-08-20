package lang

import "fmt"

// Cell is a trit cell: one balanced-ternary byte, three trits, range -13..+13.
type Cell = int8

// Balanced-ternary trit encoding: each trit t ∈ {-1,0,+1}; a cell is three
// trits (t2·9 + t1·3 + t0). -1 is the unknown, propagated by the letters.
func decodeCell(c Cell) [3]int8 { return [3]int8{int8(c % 3), int8((c / 3) % 3), int8((c / 9) % 3)} }

func encodeCell(t [3]int8) Cell { return Cell(t[0] + 3*t[1] + 9*t[2]) }

// clamp keeps a balanced-ternary result inside the 3-trit cell.
func clamp(v int) Cell {
	if v > 13 {
		return 13
	}
	if v < -13 {
		return -13
	}
	return Cell(v)
}

// opcodes — 0x00..0x0f are the sixteen Boolean letters; 0x10+ the spine.
const (
	opZero  = 0x00
	opOne   = 0x0f
	opLdi   = 0x10
	opLd    = 0x11
	opSt    = 0x12
	opTadd  = 0x13
	opTmul  = 0x14
	opJmp   = 0x15
	opJz    = 0x16
	opJnz   = 0x17
	opCall  = 0x18
	opRet   = 0x19
	opPush  = 0x1a
	opPop   = 0x1b
	opQdot  = 0x1c // MLX-QUANT seam: ternary-quantized dot product
	opUltra = 0x1d // the b1.58 hard activation: soft interior → {-1,0,+1}
	opNop   = 0x1e
	opHalt  = 0x1f
)

// Registers is the register file: 16 trit cells.
type Registers [16]Cell

// VM is the enthea bytecode machine. All memory — program, heap, stack —
// lives inside one arena the VM owns.
type VM struct {
	arena *Arena
	prog  []byte
	regs  Registers
	ip    int
	sp    int // call stack, growing down from the far end of the arena
	halt  bool
	err   error
	steps int
	// heapOff is the bump cursor after the program; the heap grows up from it.
	heapOff int
	stackLo int // deepest stack pointer reached
}

// NewVM maps a fresh arena, loads `prog` into it, and returns the machine.
func NewVM(prog []byte, arenaSize int) (*VM, error) {
	a, err := NewArena(arenaSize)
	if err != nil {
		return nil, err
	}
	if len(prog) > arenaSize {
		a.Close()
		return nil, fmt.Errorf("lang: program %d bytes exceeds arena %d", len(prog), arenaSize)
	}
	copy(a.data, prog)
	vm := &VM{arena: a, prog: a.data[:len(prog)], heapOff: len(prog), sp: arenaSize - 1, stackLo: arenaSize - 1}
	return vm, nil
}

// Arena exposes the owned region.
func (v *VM) Arena() *Arena { return v.arena }

// Run executes until halt, an error, or the step budget.
func (v *VM) Run(maxSteps int) error {
	for !v.halt && v.err == nil {
		if v.steps >= maxSteps {
			return fmt.Errorf("lang: step budget %d exhausted", maxSteps)
		}
		v.step()
	}
	return v.err
}

// Regs returns the register file after execution.
func (v *VM) Regs() Registers { return v.regs }

// StackPeak is the deepest the call stack reached, in bytes (0 = no calls).
func (v *VM) StackPeak() int { return len(v.arena.data) - 1 - v.stackLo }

// --- direct-threaded dispatch ---------------------------------------------
//
// The switch is replaced by a table of native handlers — one slot per
// opcode, the sixteen letters as sixteen entries. This is the classic
// threaded-code technique (Forth, CPython, Lua): dispatch is a single
// indirect call, the interpreter loop is a table, and the alphabet is the
// program counter.

type opFn func(v *VM)

var ops [256]opFn

func (v *VM) step() {
	v.steps++
	op := v.prog[v.ip]
	v.ip++
	ops[op](v)
}

func (v *VM) fetchByte() byte {
	if v.ip >= len(v.prog) {
		v.err = fmt.Errorf("lang: operands run past the program at %d", v.ip)
		return 0
	}
	b := v.prog[v.ip]
	v.ip++
	return b
}

func init() {
	// the sixteen letters, each its own table slot
	for f := 0; f < 16; f++ {
		idx := byte(f)
		ops[idx] = func(v *VM) { v.execLetter(idx) }
	}
	ops[opLdi] = func(v *VM) {
		r := v.fetchByte()
		imm := Cell(int8(v.fetchByte()))
		v.regs[r%16] = imm
	}
	ops[opLd] = func(v *VM) {
		r := v.fetchByte()
		addr := int(v.fetchByte())
		if addr < 0 || addr >= len(v.arena.data) {
			v.err = fmt.Errorf("lang: load from %d outside arena", addr)
			return
		}
		v.regs[r%16] = Cell(int8(v.arena.data[addr]))
	}
	ops[opSt] = func(v *VM) {
		addr := int(v.fetchByte())
		r := v.fetchByte()
		if addr < 0 || addr >= len(v.arena.data) {
			v.err = fmt.Errorf("lang: store to %d outside arena", addr)
			return
		}
		v.arena.data[addr] = byte(v.regs[r%16])
	}
	ops[opTadd] = func(v *VM) {
		d, a, b := v.fetchByte(), v.fetchByte(), v.fetchByte()
		v.regs[d%16] = clamp(int(v.regs[a%16]) + int(v.regs[b%16]))
	}
	ops[opTmul] = func(v *VM) {
		d, a, b := v.fetchByte(), v.fetchByte(), v.fetchByte()
		v.regs[d%16] = clamp(int(v.regs[a%16]) * int(v.regs[b%16]))
	}
	ops[opJmp] = func(v *VM) { v.ip = int(v.fetchByte()) }
	ops[opJz] = func(v *VM) {
		r := v.fetchByte()
		addr := int(v.fetchByte())
		if v.regs[r%16] == 0 {
			v.ip = addr
		}
	}
	ops[opJnz] = func(v *VM) {
		r := v.fetchByte()
		addr := int(v.fetchByte())
		if v.regs[r%16] != 0 {
			v.ip = addr
		}
	}
	ops[opCall] = func(v *VM) {
		addr := int(v.fetchByte())
		if v.sp <= v.heapOff {
			v.err = fmt.Errorf("lang: call stack overflow at %d", v.ip-1)
			return
		}
		v.arena.data[v.sp] = byte(v.ip)
		v.sp--
		if v.sp < v.stackLo {
			v.stackLo = v.sp
		}
		v.ip = addr
	}
	ops[opRet] = func(v *VM) {
		if v.sp >= len(v.arena.data)-1 {
			v.err = fmt.Errorf("lang: return from empty stack")
			return
		}
		v.sp++
		v.ip = int(v.arena.data[v.sp])
	}
	ops[opPush] = func(v *VM) {
		r := v.fetchByte()
		if v.sp <= v.heapOff {
			v.err = fmt.Errorf("lang: data stack overflow at %d", v.ip-1)
			return
		}
		v.arena.data[v.sp] = byte(v.regs[r%16])
		v.sp--
		if v.sp < v.stackLo {
			v.stackLo = v.sp
		}
	}
	ops[opPop] = func(v *VM) {
		r := v.fetchByte()
		if v.sp >= len(v.arena.data)-1 {
			v.err = fmt.Errorf("lang: pop from empty stack")
			return
		}
		v.sp++
		v.regs[r%16] = Cell(int8(v.arena.data[v.sp]))
	}
	ops[opQdot] = func(v *VM) {
		// MLX-QUANT seam: a ternary-quantized dot product. Inputs are the
		// first n registers; the n weights {-1,0,+1} live in the arena at w.
		d := v.fetchByte()
		w := int(v.fetchByte())
		n := int(v.fetchByte())
		if n < 1 || n > 16 {
			v.err = fmt.Errorf("lang: qdot count %d out of 1..16", n)
			return
		}
		if w < 0 || w+n > len(v.arena.data) {
			v.err = fmt.Errorf("lang: qdot weights %d..%d outside arena", w, w+n)
			return
		}
		sum := 0
		for i := 0; i < n; i++ {
			wi := int(int8(v.arena.data[w+i])) // the ternary weight {-1,0,+1}
			xi := int(v.regs[i%16])            // the ternary input
			sum += wi * xi
		}
		v.regs[d%16] = clamp(sum)
	}
	ops[opUltra] = func(v *VM) {
		// the b1.58 hard activation: the soft interior is sharpened to the
		// ternary alphabet {0, +1} / {-1}.
		r := v.fetchByte()
		c := v.regs[r%16]
		switch {
		case c > 0:
			v.regs[r%16] = 1
		case c < 0:
			v.regs[r%16] = -1
		default:
			v.regs[r%16] = 0
		}
	}
	ops[opNop] = func(v *VM) {}
	ops[opHalt] = func(v *VM) { v.halt = true }
}

// execLetter applies one of the sixteen Boolean functions tritwise to two
// source registers. An unknown (-1) trit in either source propagates.
func (v *VM) execLetter(f byte) {
	d := v.fetchByte()
	a := v.fetchByte()
	b := v.fetchByte()
	ta := decodeCell(v.regs[a%16])
	tb := decodeCell(v.regs[b%16])
	var out [3]int8
	for i := 0; i < 3; i++ {
		if ta[i] == -1 || tb[i] == -1 {
			out[i] = -1
			continue
		}
		k := 0
		if ta[i] == 1 {
			k |= 2
		}
		if tb[i] == 1 {
			k |= 1
		}
		if (f>>uint(k))&1 == 1 {
			out[i] = 1
		}
	}
	v.regs[d%16] = encodeCell(out)
}
