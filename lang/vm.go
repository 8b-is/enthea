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
	opZero   = 0x00
	opOne    = 0x0f
	opLdi    = 0x10
	opLd     = 0x11
	opSt     = 0x12
	opTadd   = 0x13
	opTmul   = 0x14
	opJmp    = 0x15
	opJz     = 0x16
	opJnz    = 0x17
	opCall   = 0x18
	opRet    = 0x19
	opPush   = 0x1a
	opPop    = 0x1b
	opQdot   = 0x1c // MLX-QUANT seam: ternary-quantized dot product
	opUltra  = 0x1d // the b1.58 hard activation: soft interior → {-1,0,+1}
	opNop    = 0x1e
	opHalt   = 0x1f
	opLix    = 0x20 // indexed load:  d ← arena[a]
	opSix    = 0x21 // indexed store: arena[a] ← d
	opMov    = 0x22 // raw byte copy (address domain): d ← s, no clamp
	opAadd   = 0x23 // raw byte add (address domain): d ← a+b, no clamp
	opCwrite = 0x24 // quant-ctx: slide a token into the context window
	opCand   = 0x25 // quant-ctx: r = Context AND over the window
	opCsum   = 0x26 // quant-ctx: r = Context sum (the gate's vote)
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
	// quant-ctx: a sliding window of ternary tokens in the arena (kompress-ultra's
	// Circulator — the living context, quantized to {-1,0,+1}).
	ctxBase int
	ctxLen  int
	ctxPos  int
}

// NewVM maps a fresh arena, loads `prog` into it, and returns the machine.
func NewVM(prog []byte, arenaSize int) (*VM, error) {
	return NewVMAt(prog, arenaSize, 0)
}

// NewVMAt is NewVM with the program placed at an arena base. Data may then
// live below the program (low addresses 0..base-1); the program rides high.
func NewVMAt(prog []byte, arenaSize, progBase int) (*VM, error) {
	a, err := NewArena(arenaSize)
	if err != nil {
		return nil, fmt.Errorf("lang: NewVMAt: %w", err)
	}
	if progBase < 0 || progBase >= arenaSize {
		a.Close()
		return nil, fmt.Errorf("lang: program base %d out of arena %d", progBase, arenaSize)
	}
	if len(prog) > arenaSize-progBase {
		a.Close()
		return nil, fmt.Errorf("lang: program %d bytes exceeds arena %d at base %d", len(prog), arenaSize, progBase)
	}
	copy(a.data[progBase:], prog)
	ctxLen := 8
	ctxBase := progBase + len(prog) + 16
	if ctxBase+ctxLen >= arenaSize-64 {
		a.Close()
		return nil, fmt.Errorf("lang: no room for the context window (program too large)")
	}
	vm := &VM{arena: a, prog: a.data[progBase : progBase+len(prog)], heapOff: progBase + len(prog), sp: arenaSize - 1, stackLo: arenaSize - 1, ctxBase: ctxBase, ctxLen: ctxLen}
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

// fetchAddr reads a 16-bit little-endian address (the widened ISA).
func (v *VM) fetchAddr() int {
	lo := v.fetchByte()
	hi := v.fetchByte()
	return int(lo) | int(hi)<<8
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
		addr := v.fetchAddr()
		if addr < 0 || addr >= len(v.arena.data) {
			v.err = fmt.Errorf("lang: load from %d outside arena", addr)
			return
		}
		v.regs[r%16] = Cell(int8(v.arena.data[addr]))
	}
	ops[opSt] = func(v *VM) {
		addr := v.fetchAddr()
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
	ops[opJmp] = func(v *VM) { v.ip = v.fetchAddr() }
	ops[opJz] = func(v *VM) {
		r := v.fetchByte()
		addr := v.fetchAddr()
		if v.regs[r%16] == 0 {
			v.ip = addr
		}
	}
	ops[opJnz] = func(v *VM) {
		r := v.fetchByte()
		addr := v.fetchAddr()
		if v.regs[r%16] != 0 {
			v.ip = addr
		}
	}
	ops[opCall] = func(v *VM) {
		addr := v.fetchAddr()
		if v.sp-1 <= v.heapOff {
			v.err = fmt.Errorf("lang: call stack overflow at %d", v.ip-2)
			return
		}
		// return address: 16-bit, pushed as lo then hi
		v.arena.data[v.sp] = byte(v.ip)
		v.arena.data[v.sp-1] = byte(v.ip >> 8)
		v.sp -= 2
		if v.sp < v.stackLo {
			v.stackLo = v.sp
		}
		v.ip = addr
	}
	ops[opRet] = func(v *VM) {
		if v.sp+2 > len(v.arena.data)-1 {
			v.err = fmt.Errorf("lang: return from empty stack")
			return
		}
		v.sp += 2
		v.ip = int(v.arena.data[v.sp]) | int(v.arena.data[v.sp-1])<<8
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
		w := v.fetchAddr()
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
	ops[opLix] = func(v *VM) {
		d := v.fetchByte()
		a := v.fetchByte()
		if int(v.regs[a%16]) < 0 || int(v.regs[a%16]) >= len(v.arena.data) {
			v.err = fmt.Errorf("lang: indexed load from %d outside arena", v.regs[a%16])
			return
		}
		v.regs[d%16] = Cell(int8(v.arena.data[v.regs[a%16]]))
	}
	ops[opSix] = func(v *VM) {
		a := v.fetchByte()
		d := v.fetchByte()
		if int(v.regs[a%16]) < 0 || int(v.regs[a%16]) >= len(v.arena.data) {
			v.err = fmt.Errorf("lang: indexed store to %d outside arena", v.regs[a%16])
			return
		}
		v.arena.data[v.regs[a%16]] = byte(v.regs[d%16])
	}
	ops[opMov] = func(v *VM) {
		d := v.fetchByte()
		s := v.fetchByte()
		v.regs[d%16] = v.regs[s%16] // raw, no clamp: addresses are bytes
	}
	ops[opAadd] = func(v *VM) {
		d := v.fetchByte()
		a := v.fetchByte()
		b := v.fetchByte()
		v.regs[d%16] = Cell(int8(byte(v.regs[a%16] + v.regs[b%16]))) // raw, no clamp
	}
	ops[opCwrite] = func(v *VM) {
		r := v.fetchByte()
		v.arena.data[v.ctxBase+v.ctxPos] = byte(v.regs[r%16]) // slide in
		v.ctxPos = (v.ctxPos + 1) % v.ctxLen                  // evict the oldest
	}
	ops[opCand] = func(v *VM) {
		r := v.fetchByte()
		acc := Cell(13) // trits [1,1,1]: the identity of tritwise AND
		for i := 0; i < v.ctxLen; i++ {
			acc = tritAnd(acc, Cell(int8(v.arena.data[v.ctxBase+i])))
		}
		v.regs[r%16] = acc
	}
	ops[opCsum] = func(v *VM) {
		r := v.fetchByte()
		sum := 0
		for i := 0; i < v.ctxLen; i++ {
			sum += int(int8(v.arena.data[v.ctxBase+i]))
		}
		v.regs[r%16] = clamp(sum)
	}
}

// tritAnd is the tritwise AND of two cells, with the unknown propagating:
// the Context AND over the window keeps the Bayesian interior honest.
func tritAnd(a, b Cell) Cell {
	ta := decodeCell(a)
	tb := decodeCell(b)
	var out [3]int8
	for i := 0; i < 3; i++ {
		if ta[i] == -1 || tb[i] == -1 {
			out[i] = -1
		} else if ta[i] == 1 && tb[i] == 1 {
			out[i] = 1
		}
	}
	return encodeCell(out)
}

// CtxLen reports the quant-ctx window size.
func (v *VM) CtxLen() int { return v.ctxLen }

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
