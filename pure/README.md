# pure — the Turing shrine

One side-effect-free kernel, written four ways. Out of respect for Turing and
ALL before and around.

```
NAND ──► NOT · AND · OR · XOR      every computable function is NAND
  │      (Sheffer stroke, 1913)
  │      all 16 binary Boolean    functional completeness, proven in
  │      functions are NAND       TestFunctionalCompleteness — every
  │      expressions (Post, 1921) 4-bit truth table, all four rows
  │
  └──► the ternary quantize        the BitNet b1.58 kernel, proven pure
          scale = max(mean|w|, 1e-7)
          codes = round(clamp(w/scale, -1, 1)) ∈ {−1, 0, +1}
```

## The four tongues

| File | The language | Role |
|---|---|---|
| `ternary.go` | Go | the oracle — `pure.Ternary4` |
| `kernels/ternary.c` | C | the reference, pure and general |
| `kernels/ternary4_arm64.S` | AArch64 | hand-written, Mach-O + ELF |
| `kernels/ternary4_x86_64.S` | x86-64 | hand-written, Mach-O + ELF |
| `kernels/nand.c` | C | the seed — NAND builds every gate |
| `run.c` | C | the only I/O in the shrine (the harness) |

## The purity proof

```
go test ./pure/ -v
```

- **`TestKernelsMatchProve`** — compiles the C + hand-written assembly for the
  running architecture with the system `cc`, runs them, and proves the codes
  are **bit-identical** to the Go oracle (`[1 -1 0 0]` on the worked example).
  The scale matches within **1 ulp** — the only difference between C, Go, and
  hand-rolled assembly is floating-point associativity, which is exactly the
  honest claim: same function, same value, up to the machine's rounding.
- **`TestNandSeedProve`** — the universal gate is sound: NAND alone builds
  NOT, AND, OR, XOR with every truth table.
- **`ExampleTernary4`** — the worked example, runnable: `[1 -1 0 0] 1.15`.

## No side effects

The kernels are pure functions — no syscalls, no globals, no allocation, no
I/O. `run.c` is the only thing that prints, and it is only built and executed
by the test. The shipped module stays 100% pure Go; the shrine is build-gated,
never linked into `enthea`.

*NAND is the seed · the quant is the bloom · a function is a function*
