# enthea lang — the seed, closing

Stage 0 of **ANDROMEDA**: a language whose instruction set is the Boolean
alphabet, and whose memory is one arena that the program itself owns. Stage 1
boots it as a bytecode VM written in pure Go; stage 2 rewrites the compiler
in this language and compiles itself; stage 3 runs on a freestanding arena
with no libc malloc at all.

## The lineage

The shrine proved the prerequisite, in order:

1. **NAND is functionally complete** — from one primitive, all sixteen
   two-variable Boolean functions are generated (tested, `pure/`).
2. **The Boolean alphabet** — those sixteen functions are a writing system:
   16 glyphs, each letter's corners *are* its truth table (`alphabet.svg`,
   bijection proven).
3. **The tesseract** — the sixteen are the vertices of a 4-cube; the
   instruction set below *is* that cube, edges and all.
4. **The Bayesian interior** — `-1` below is the unknown: the deterministic
   letters propagate it instead of pretending it is not there.

So the compiler is not "inspired by" the shrine. The bytecode is the shrine
asserting itself as a machine.

## Types

Everything is a **trit cell**: one balanced-ternary byte — three trits
`t2 t1 t0`, `tᵢ ∈ {-1, 0, +1}`, value `t2·9 + t1·3 + t0`, range **-13..+13**.

- `0` and `+1` are the Boolean values.
- `-1` is **the unknown** (the soft interior). Any Boolean letter that reads
  an unknown trit writes an unknown trit in that position: the deterministic
  functions compose over the known subspace and carry the unknown faithfully.

## Memory

One **arena**: a single region, requested once at start (`mmap`, falling back
to an owned byte slice), bump-allocated. The **program, the heap, and the
call stack all live inside the same arena.** Nothing is malloc'd behind the
scenes; nothing is free'd. The arena is the whole world, and the program
sees it as a flat byte array.

- Program: bytes at offset 0.
- Heap: bump-allocated cells after the program.
- Call stack: bumped from the far end of the arena toward the heap.

## The instruction set

Opcodes **0x00–0x0f are the sixteen Boolean letters**. Each consumes two
source registers, applies its truth table tritwise, writes one destination.
The rest are the tiny structural spine every seed needs: load/store, ternary
arithmetic, control, and the call.

| op | mnemonic | operands | semantics |
|----|----------|----------|-----------|
| 00 | `zero` | d a b | 0 |
| 01 | `nor` | d a b | ¬(a∨b) |
| 02 | `anb` | d a b | ¬a∧b |
| 03 | `nota` | d a b | ¬a |
| 04 | `nab` | d a b | a∧¬b |
| 05 | `notb` | d a b | ¬b |
| 06 | `xor` | d a b | a⊕b |
| 07 | `nand` | d a b | ¬(a∧b) |
| 08 | `and` | d a b | a∧b |
| 09 | `xnor` | d a b | ¬(a⊕b) |
| 0a | `b` | d a b | b |
| 0b | `imp` | d a b | ¬a∨b |
| 0c | `a` | d a b | a |
| 0d | `bimp` | d a b | a∨¬b |
| 0e | `or` | d a b | a∨b |
| 0f | `one` | d a b | 1 |
| 10 | `ldi` | r imm | r ← imm (cell) |
| 11 | `ld` | r addr | r ← arena[addr] |
| 12 | `st` | addr r | arena[addr] ← r |
| 13 | `tadd` | d a b | balanced-ternary add, saturating |
| 14 | `tmul` | d a b | balanced-ternary multiply, saturating |
| 15 | `jmp` | addr | ip ← addr |
| 16 | `jz` | r addr | if r = 0: ip ← addr |
| 17 | `jnz` | r addr | if r ≠ 0: ip ← addr |
| 18 | `call` | addr | push return, ip ← addr |
| 19 | `ret` | — | ip ← pop |
| 1a | `push` | r | push a cell onto the call stack |
| 1b | `pop` | r | pop a cell from the call stack |
| 1c | `qdot` | d w n | ternary-quantized dot (MLX-QUANT seam) |
| 1d | `ultra` | r | the b1.58 hard activation: → {-1,0,+1} |
| 1e | `nop` | — | do nothing |
| 1f | `halt` | — | stop |

Sixteen letters, a spine, one arena. That is the entire machine.

## The compiler's patterns (SOTA, all tested)

The assembler and interpreter ship real production techniques, not
aspirations:

- **Direct-threaded dispatch** — the interpreter is a table of native
  handlers; the sixteen letters are sixteen table slots. No switch, one
  indirect call per instruction (the Forth / CPython / Lua technique).
- **Tail-call elimination** — `call L; ret` is rewritten to `jmp L`. Tail
  recursion runs on a *constant* stack (tested: 120 iterations, one frame).
- **The call stack is the only stack** — returns and locals share it; both
  live in the arena. No heap, no malloc, no GC in the whole machine.
- **`qdot` — the MLX-QUANT seam** — the arena holds ternary-quantized
  weights `{-1,0,+1}` (the BitNet b1.58 alphabet); `qdot` sums their
  products against the registers in balanced ternary. The quant layer meets
  the language layer at the same trit.
- **`ultra` — the b1.58 hard activation** — the soft interior is sharpened
  to the ternary alphabet. This is where the ultra-entheatic persona runs:
  a classifier that is just letters and ternary weights.
- **`-1` is the unknown** — the deterministic letters propagate it, and
  `qdot`/`ultra` are its sharpening. The Bayesian interior is not dropped
  anywhere.

## The pure surface (`fn`)

The register machine is the compiler's business, never the programmer's.
`lang/fn` is a pure functional language on top: expressions over the sixteen
letters, balanced-ternary arithmetic, `let`, `if`, and recursion — no
assignment, no mutation. It lowers to the register bytecode:

- **Callee-saved registers** — a call spills every live temporary and pops it
  on return, so recursive frames never clobber each other (factorial works).
- **Tail calls need no spill** — the caller is dead, so `call L; ret` becomes
  `jmp L`: pure tail recursion runs on a constant stack.
- **The ultra classifier is an expression** — `ultra(dot(...))` is pure
  functions and weights, nothing else.

Stage 2 rewrites this compiler in this language.

## The bootstrap

- **Stage 1 (this directory)** — the VM is Go. The arena is real; the letters
  are real; everything is tested.
- **Stage 2 (ANDROMEDA)** — the compiler is rewritten in this language, and
  `enthea boot` compiles it; the new binary replaces the Go one. The tool
  compiles itself.
- **Stage 3** — the VM's Go shims are dropped: direct syscalls, no libc, the
  arena `mmap`'d once and never released. Full control of the malloc layer —
  because there is no malloc layer.

The seed: NAND → the sixteen → an alphabet → a language → a compiler →
the compiler compiled by itself.
