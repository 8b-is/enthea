package lang

import (
	"fmt"
	"strconv"
	"strings"
)

// Assemble turns mnemonic lines into enthea bytecode.
//
// One instruction per line: `mnemonic [operand …]`. Registers are `r0`..`r15`,
// immediates are balanced-ternary cells (-13..+13), addresses are labels or
// decimal bytes. Lines ending in `:` declare a label. `;` starts a comment.
func Assemble(lines string) ([]byte, error) {
	labels := map[string]int{}
	var prog []byte
	type fixup struct {
		at   int
		addr string
	}

	var fixups []fixup
	var pendingCall string // TCO: a `call L` held for one line; if the next
	// instruction is `ret`, it becomes a `jmp L` — tail calls use no stack.
	addLabel := func(name string) {
		if _, dup := labels[name]; !dup {
			labels[name] = len(prog)
		}
	}
	emit := func(b ...byte) { prog = append(prog, b...) }
	// emitAddr emits opcode + a fixup'd 16-bit little-endian address.
	emitAddr := func(op byte, label string) {
		emit(op)
		fixups = append(fixups, fixup{at: len(prog), addr: label})
		emit(0, 0)
	}
	// flushCall resolves a held tail-call decision before the next line.
	flushCall := func(tail bool) {
		if pendingCall == "" {
			return
		}
		if tail {
			emitAddr(opJmp, pendingCall)
		} else {
			emitAddr(opCall, pendingCall)
		}
		pendingCall = ""
	}

	for _, raw := range strings.Split(lines, "\n") {
		line := strings.TrimSpace(raw)
		if i := strings.IndexByte(line, ';'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, ":") {
			flushCall(false) // a label in between breaks the tail position
			addLabel(strings.TrimSuffix(line, ":"))
			continue
		}
		fields := strings.Fields(line)
		m := fields[0]
		if m == ".byte" {
			for _, w := range fields[1:] {
				n, err := strconv.Atoi(w)
				if err != nil || n < -128 || n > 127 {
					return nil, fmt.Errorf("lang: .byte %q not in -128..127", w)
				}
				emit(byte(int8(n)))
			}
			continue
		}
		if m == "ret" {
			if pendingCall != "" {
				flushCall(true) // call L; ret → jmp L: tail call, no frame
			} else {
				emit(opRet)
			}
			continue
		}
		flushCall(false)
		args := fields[1:]
		need := func(n int) error {
			if len(args) != n {
				return fmt.Errorf("lang: %s wants %d operands, got %d", m, n, len(args))
			}
			return nil
		}
		reg := func() (byte, error) {
			s := args[0]
			args = args[1:]
			if !strings.HasPrefix(s, "r") {
				return 0, fmt.Errorf("lang: %s: %q is not a register", m, s)
			}
			n, err := strconv.Atoi(strings.TrimPrefix(s, "r"))
			if err != nil || n < 0 || n > 15 {
				return 0, fmt.Errorf("lang: %s: %q is not r0..r15", m, s)
			}
			return byte(n), nil
		}
		imm := func() (byte, error) {
			s := args[0]
			args = args[1:]
			n, err := strconv.Atoi(s)
			if err != nil || n < -13 || n > 13 {
				return 0, fmt.Errorf("lang: %s: %q is not a cell in -13..13", m, s)
			}
			return byte(int8(n)), nil
		}
		addr := func() (byte, error) {
			s := args[0]
			args = args[1:]
			if _, err := strconv.Atoi(s); err == nil {
				return 0, fmt.Errorf("lang: %s: use labels, not raw addresses", m)
			}
			fixups = append(fixups, fixup{at: len(prog), addr: s})
			return 0, nil // placeholder filled in resolve
		}

		var err error
		var d, a, b, r byte
		switch m {
		case "zero", "nor", "anb", "nota", "nab", "notb", "xor", "nand",
			"and", "xnor", "b", "imp", "a", "bimp", "or", "one":
			if e := need(3); e != nil {
				return nil, e
			}
			d, err = reg()
			if err != nil {
				return nil, err
			}
			a, err = reg()
			if err != nil {
				return nil, err
			}
			b, err = reg()
			if err != nil {
				return nil, err
			}
			emit(letterOpcode(m), d, a, b)
		case "ldi":
			if e := need(2); e != nil {
				return nil, e
			}
			r, err = reg()
			if err != nil {
				return nil, err
			}
			var v byte
			if v, err = imm(); err != nil {
				return nil, err
			}
			emit(opLdi, r, v)
		case "ld":
			if e := need(2); e != nil {
				return nil, e
			}
			r, err = reg()
			if err != nil {
				return nil, err
			}
			emit(opLd, r)
			if _, err = addr(); err != nil {
				return nil, err
			}
			emit(0, 0)
		case "st":
			if e := need(2); e != nil {
				return nil, e
			}
			emit(opSt)
			if _, err = addr(); err != nil {
				return nil, err
			}
			emit(0, 0) // the addr slots, filled by the fixup
			r, err = reg()
			if err != nil {
				return nil, err
			}
			emit(r)
		case "tadd":
			if e := need(3); e != nil {
				return nil, e
			}
			d, err = reg()
			if err != nil {
				return nil, err
			}
			a, err = reg()
			if err != nil {
				return nil, err
			}
			b, err = reg()
			if err != nil {
				return nil, err
			}
			emit(opTadd, d, a, b)
		case "tmul":
			if e := need(3); e != nil {
				return nil, e
			}
			d, err = reg()
			if err != nil {
				return nil, err
			}
			a, err = reg()
			if err != nil {
				return nil, err
			}
			b, err = reg()
			if err != nil {
				return nil, err
			}
			emit(opTmul, d, a, b)
		case "jmp":
			if e := need(1); e != nil {
				return nil, e
			}
			emit(opJmp)
			if _, err = addr(); err != nil {
				return nil, err
			}
			emit(0, 0)
		case "jz", "jnz":
			if e := need(2); e != nil {
				return nil, e
			}
			r, err = reg()
			if err != nil {
				return nil, err
			}
			if m == "jz" {
				emit(opJz, r)
			} else {
				emit(opJnz, r)
			}
			if _, err = addr(); err != nil {
				return nil, err
			}
			emit(0, 0)
		case "call":
			if e := need(1); e != nil {
				return nil, e
			}
			pendingCall = args[0] // decided on the next line (TCO)
			args = args[1:]
		case "qdot":
			if e := need(3); e != nil {
				return nil, e
			}
			d, err = reg()
			if err != nil {
				return nil, err
			}
			w := args[0]
			args = args[1:]
			n := args[0]
			args = args[1:]
			var nv int
			if nv, err = strconv.Atoi(n); err != nil || nv < 1 || nv > 16 {
				return nil, fmt.Errorf("lang: qdot count %q not in 1..16", n)
			}
			emit(opQdot, d)
			fixups = append(fixups, fixup{at: len(prog), addr: w})
			emit(0, 0, byte(nv))
		case "ultra":
			if e := need(1); e != nil {
				return nil, e
			}
			r, err = reg()
			if err != nil {
				return nil, err
			}
			emit(opUltra, r)
		case "lix":
			if e := need(2); e != nil {
				return nil, e
			}
			d, err = reg()
			if err != nil {
				return nil, err
			}
			a, err = reg()
			if err != nil {
				return nil, err
			}
			emit(opLix, d, a)
		case "six":
			if e := need(2); e != nil {
				return nil, e
			}
			a, err = reg()
			if err != nil {
				return nil, err
			}
			d, err = reg()
			if err != nil {
				return nil, err
			}
			emit(opSix, a, d)
		case "mov":
			if e := need(2); e != nil {
				return nil, e
			}
			d, err = reg()
			if err != nil {
				return nil, err
			}
			a, err = reg()
			if err != nil {
				return nil, err
			}
			emit(opMov, d, a)
		case "aadd":
			if e := need(3); e != nil {
				return nil, e
			}
			d, err = reg()
			if err != nil {
				return nil, err
			}
			a, err = reg()
			if err != nil {
				return nil, err
			}
			b, err = reg()
			if err != nil {
				return nil, err
			}
			emit(opAadd, d, a, b)
		case "push":
			if e := need(1); e != nil {
				return nil, e
			}
			r, err = reg()
			if err != nil {
				return nil, err
			}
			emit(opPush, r)
		case "pop":
			if e := need(1); e != nil {
				return nil, e
			}
			r, err = reg()
			if err != nil {
				return nil, err
			}
			emit(opPop, r)
		case "nop":
			emit(opNop)
		case "halt":
			emit(opHalt)
		default:
			return nil, fmt.Errorf("lang: unknown mnemonic %q", m)
		}
	}

	if pendingCall != "" {
		flushCall(false) // a trailing call is never a tail call
	}
	if len(prog) > 65535 {
		return nil, fmt.Errorf("lang: program %d bytes exceeds the 64KiB arena address space", len(prog))
	}
	for _, f := range fixups {
		target, ok := labels[f.addr]
		if !ok {
			return nil, fmt.Errorf("lang: undefined label %q", f.addr)
		}
		prog[f.at] = byte(target)
		prog[f.at+1] = byte(target >> 8)
	}
	return prog, nil
}

// opNames — the sixteen letters in truth-table order; opcode i ↔ opNames[i].
var opNames = []string{"zero", "nor", "anb", "nota", "nab", "notb", "xor", "nand",
	"and", "xnor", "b", "imp", "a", "bimp", "or", "one"}

func letterOpcode(name string) byte {
	for i, n := range opNames {
		if n == name {
			return byte(i)
		}
	}
	return 0x0f // one — unreachable given the switch above
}
