package fn

import (
	"fmt"
	"strings"

	"github.com/8b-is/enthea/lang"
)

// r0 is the calling convention (argument in, result out); r1 is a permanent
// zero; r2..r15 are compiler-managed temporaries.
const zeroReg = 1

// cg lowers pure functional expressions to the register bytecode.
type cg struct {
	out  *strings.Builder
	env  map[string]byte
	free []byte // stack of free temporaries, r15 down to r2
}

func freshTemps() []byte {
	// r2..r15, pushed high-first so fresh() pops from the low end.
	free := make([]byte, 0, 14)
	for r := byte(15); r >= 2; r-- {
		free = append(free, r)
	}
	return free
}

func (c *cg) fresh() byte {
	if len(c.free) == 0 {
		panic("fn: out of temporary registers")
	}
	r := c.free[len(c.free)-1]
	c.free = c.free[:len(c.free)-1]
	return r
}

func (c *cg) freeTemps(rs ...byte) {
	for _, r := range rs {
		if r != zeroReg {
			c.free = append(c.free, r)
		}
	}
}

// allocated returns every temporary currently live (allocated, not free).
// These are callee-saved across calls.
func (c *cg) allocated() []byte {
	free := map[byte]bool{}
	for _, r := range c.free {
		free[r] = true
	}
	var out []byte
	for r := byte(2); r <= 15; r++ {
		if !free[r] {
			out = append(out, r)
		}
	}
	return out
}

// mov emits d = s as a raw byte copy (the address domain: no clamp, so
// pointers survive; cell values ≤ 13 copy identically).
func (c *cg) mov(d, s byte) {
	fmt.Fprintf(c.out, "mov r%d r%d\n", d, s)
}

// Compile lowers a program to enthea bytecode.
func Compile(fns []*Fn, main Expr) ([]byte, error) {
	c := &cg{out: &strings.Builder{}, free: freshTemps()}
	c.out.WriteString("zero r1 r0 r0\n") // r1 = 0, forever
	c.expr(main, true, map[string]byte{})
	c.out.WriteString("halt\n")
	for _, f := range fns {
		fmt.Fprintf(c.out, "%s:\n", f.Name)
		param := c.fresh()
		c.mov(param, 0) // r0 → param
		env := map[string]byte{f.Param: param}
		c.expr(f.Body, true, env)
		c.out.WriteString("ret\n")
		c.freeTemps(param)
	}
	prog, err := lang.Assemble(c.out.String())
	if err != nil {
		return nil, fmt.Errorf("fn: %w", err)
	}
	return prog, nil
}

// expr compiles an expression. In tail mode the value lands in r0 and the
// caller owns the `ret` (so `call f` in tail position becomes `jmp f` by the
// assembler's TCO). Otherwise it returns the register holding the value.
func (c *cg) expr(e Expr, tail bool, env map[string]byte) byte {
	switch x := e.(type) {
	case Lit:
		if tail {
			fmt.Fprintf(c.out, "ldi r0 %d\n", int(x))
			return 0
		}
		r := c.fresh()
		fmt.Fprintf(c.out, "ldi r%d %d\n", r, int(x))
		return r

	case Var:
		r, ok := env[string(x)]
		if !ok {
			panic("fn: unbound variable " + string(x))
		}
		if tail {
			c.mov(0, r)
			return 0
		}
		out := c.fresh()
		c.mov(out, r)
		return out

	case *Let:
		v := c.expr(x.Val, false, env)
		env[string(x.Name)] = v
		// shadow-restore is unnecessary: temps are monotonically live here
		return c.expr(x.Body, tail, env)

	case *If:
		cond := c.expr(x.Cond, false, env)
		thenL, endL := "L"+uniq(), "L"+uniq()
		// jz fires when the condition is zero — which selects the THEN path
		fmt.Fprintf(c.out, "jz r%d %s\n", cond, thenL)
		c.freeTemps(cond) // dead after the jump
		if tail {
			// else-path first (fall-through), then-path, both end in ret
			c.expr(x.Else, true, env)
			c.out.WriteString("ret\n")
			fmt.Fprintf(c.out, "%s:\n", thenL)
			c.expr(x.Then, true, env)
			c.out.WriteString("ret\n")
			return 0
		}
		res := c.fresh()
		te := c.expr(x.Else, false, env)
		c.mov(res, te)
		c.freeTemps(te)
		fmt.Fprintf(c.out, "jmp %s\n", endL)
		fmt.Fprintf(c.out, "%s:\n", thenL)
		t := c.expr(x.Then, false, env)
		c.mov(res, t)
		c.freeTemps(t, cond)
		fmt.Fprintf(c.out, "%s:\n", endL)
		return res

	case *Prim:
		return c.prim(x, tail, env)

	case *Call:
		a := c.expr(x.Arg, false, env)
		c.mov(0, a) // r0 = argument
		c.freeTemps(a)
		if tail {
			// tail call: the caller's registers are dead; just jump through
			// the assembler's `call L; ret` → `jmp L` rewrite
			fmt.Fprintf(c.out, "call %s\n", x.Fn)
			return 0
		}
		// callee-saved registers: spill every live temporary across the call,
		// so recursive frames never clobber each other
		live := c.allocated()
		for _, r := range live {
			fmt.Fprintf(c.out, "push r%d\n", r)
		}
		fmt.Fprintf(c.out, "call %s\n", x.Fn)
		for i := len(live) - 1; i >= 0; i-- {
			fmt.Fprintf(c.out, "pop r%d\n", live[i])
		}
		r := c.fresh()
		c.mov(r, 0) // result out of r0
		return r
	}
	panic("fn: unknown expression")
}

func (c *cg) prim(p *Prim, tail bool, env map[string]byte) byte {
	letters := []string{"zero", "nor", "anb", "nota", "nab", "notb", "xor", "nand",
		"and", "xnor", "b", "imp", "a", "bimp", "or", "one"}
	name := p.Name
	switch name {
	case "iszero":
		// jz tests a register against zero; the if-compiler does the rest.
		if tail {
			return 0
		}
		return c.expr(p.Args[0], false, env)
	case "load":
		a := c.expr(p.Args[0], false, env)
		if tail {
			fmt.Fprintf(c.out, "lix r0 r%d\n", a) // dereference into r0
			c.freeTemps(a)
			return 0
		}
		out := c.fresh()
		fmt.Fprintf(c.out, "lix r%d r%d\n", out, a)
		c.freeTemps(a)
		return out
	case "aadd":
		a := c.expr(p.Args[0], false, env)
		b := c.expr(p.Args[1], false, env)
		if tail {
			c.mov(0, a)
			return 0
		}
		out := c.fresh()
		fmt.Fprintf(c.out, "aadd r%d r%d r%d\n", out, a, b)
		c.freeTemps(a, b)
		return out
	case "store":
		a := c.expr(p.Args[0], false, env)
		v := c.expr(p.Args[1], false, env)
		fmt.Fprintf(c.out, "six r%d r%d\n", a, v)
		c.freeTemps(a, v)
		if tail {
			return 0
		}
		out := c.fresh()
		c.mov(out, zeroReg)
		return out
	case "ultra":
		v := c.expr(p.Args[0], false, env)
		if tail {
			fmt.Fprintf(c.out, "ultra r%d\n", v)
			c.mov(0, v)
			return 0
		}
		fmt.Fprintf(c.out, "ultra r%d\n", v)
		return v
	case "neg":
		v := c.expr(p.Args[0], false, env)
		neg := c.fresh()
		fmt.Fprintf(c.out, "ldi r%d -1\n", neg)
		out := c.fresh()
		fmt.Fprintf(c.out, "tmul r%d r%d r%d\n", out, v, neg)
		c.freeTemps(v, neg)
		if tail {
			c.mov(0, out)
			return 0
		}
		return out
	case "add", "sub", "mul":
		a := c.expr(p.Args[0], false, env)
		b := c.expr(p.Args[1], false, env)
		var out byte
		switch name {
		case "add":
			out = c.fresh()
			fmt.Fprintf(c.out, "tadd r%d r%d r%d\n", out, a, b)
		case "mul":
			out = c.fresh()
			fmt.Fprintf(c.out, "tmul r%d r%d r%d\n", out, a, b)
		case "sub":
			neg := c.fresh()
			fmt.Fprintf(c.out, "ldi r%d -1\n", neg)
			negb := c.fresh()
			fmt.Fprintf(c.out, "tmul r%d r%d r%d\n", negb, b, neg)
			out = c.fresh()
			fmt.Fprintf(c.out, "tadd r%d r%d r%d\n", out, a, negb)
			c.freeTemps(neg, negb)
		}
		c.freeTemps(a, b)
		if tail {
			c.mov(0, out)
			return 0
		}
		return out
	}
	// the sixteen letters, as pure combinators: constants (zero/one), unary
	// (nota/notb/a/b), and binary (the rest)
	constant := map[string]bool{"zero": true, "one": true}
	unary := map[string]bool{"nota": true, "notb": true, "a": true, "b": true}
	for i, l := range letters {
		if l == name {
			var a, b byte
			if constant[l] {
				a, b = zeroReg, zeroReg
			} else if unary[l] {
				a = c.expr(p.Args[0], false, env)
				b = a
			} else {
				a = c.expr(p.Args[0], false, env)
				b = c.expr(p.Args[1], false, env)
			}
			out := c.fresh()
			fmt.Fprintf(c.out, "%s r%d r%d r%d\n", letters[i], out, a, b)
			if unary[l] {
				c.freeTemps(a)
			} else {
				c.freeTemps(a, b)
			}
			if tail {
				c.mov(0, out)
				return 0
			}
			return out
		}
	}
	panic("fn: unknown primitive " + name)
}

var uniqCounter int

func uniq() string {
	uniqCounter++
	return fmt.Sprintf("%d", uniqCounter)
}
