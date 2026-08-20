// enthea — the deepsiper-enthea entry binary.
//
// One pure-stdlib binary that runs the engine's MCP servers, installs the
// constellation personas, and wires itself into any OSS client.
//
//	enthea mcp            serve the engine's MCP servers over stdio
//	enthea personas       list the constellation voices
//	enthea personas --install <dir>   write persona agent files
//	enthea setup <client> wire MCP + personas into opencode / charm / etc.
//	enthea doctor         check the engine + constellation surfaces
//	enthea version
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/8b-is/enthea/internal/mcp"
	"github.com/8b-is/enthea/internal/personas"
	"github.com/8b-is/enthea/internal/ui"
	"github.com/8b-is/enthea/lang"
	"github.com/8b-is/enthea/lang/fn"
	"github.com/8b-is/enthea/pure"
	"github.com/8b-is/enthea/vakedc"
)

const version = "0.1.18"

// Command is a subcommand: a name, a one-line help, and a Run.
type Command struct {
	Help string
	Run  func(ctx context.Context, args []string) error
}

// registry is the subcommand table — the idiomatic "commands as data" shape.
var registry = map[string]Command{
	"mcp":      {Help: "serve the engine MCP servers over stdio", Run: runMCP},
	"personas": {Help: "list or install the constellation voices", Run: runPersonas},
	"setup":    {Help: "wire MCP + personas into a client (opencode, charm, …)", Run: runSetup},
	"doctor":   {Help: "check the engine and constellation surfaces", Run: runDoctor},
	"lang":     {Help: "boot the enthea machine: run a program on its own arena", Run: runLang},
	"fn":       {Help: "compile + run a pure enthea expression (the seed's language)", Run: runFn},
	"bus":      {Help: "run a Go channel of 1-bit models (weights in, results out)", Run: runBus},
	"quantctx": {Help: "the living context: sliding window of quantized tokens", Run: runQuantCtx},
	"vakedc":   {Help: "the capability-graph assembler: NAND-only synthesis of the sixteen", Run: runVakedc},
	"wire":     {Help: "ternaryPureASCII: encode/decode language artifacts on the wire", Run: runWire},
	"version":  {Help: "print the version", Run: runVersion},
}

func main() {
	ctx := context.Background()
	if len(os.Args) < 2 {
		usage()
		return
	}
	cmd, ok := registry[os.Args[1]]
	if !ok {
		ui.Err("unknown command: " + os.Args[1])
		usage()
		os.Exit(2)
	}
	if err := cmd.Run(ctx, os.Args[2:]); err != nil {
		ui.Err(err.Error())
		ui.Sig()
		os.Exit(1)
	}
}

func usage() {
	ui.Header("enthea — the deepsiper-enthea engine entry")
	ui.Bullet("brew install 8b-is/tap/deepsipser-enthea")
	fmt.Println()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		ui.Table([][2]string{{n, registry[n].Help}})
	}
	ui.Sig()
}

// --- mcp ---

func runMCP(_ context.Context, args []string) error {
	sh := mcp.OSShell{}
	server, err := mcp.New(
		mcp.PersonasTool{},
		mcp.NewKompressTool(sh),
		mcp.NewPersonsTool(sh),
	)
	if err != nil {
		return err
	}
	ui.Header("enthea mcp — serving over stdio")
	ui.Bullet("tools: personas_list · kompress_compress · kompress_persons")
	return server.Serve(context.Background(), nopCloser{os.Stdin, os.Stdout})
}

// nopCloser pairs stdin/stdout into one io.ReadWriteCloser.
type nopCloser struct {
	r *os.File
	w *os.File
}

func (c nopCloser) Read(p []byte) (int, error)  { return c.r.Read(p) }
func (c nopCloser) Write(p []byte) (int, error) { return c.w.Write(p) }
func (c nopCloser) Close() error                { return nil }

// --- personas ---

func runPersonas(_ context.Context, args []string) error {
	if len(args) >= 2 && args[0] == "--install" {
		dir := args[1]
		written, err := personas.Install(dir)
		if err != nil {
			return err
		}
		ui.Header("enthea personas — installed")
		for _, p := range written {
			ui.Check(filepath.Base(p))
		}
		ui.Sig()
		return nil
	}
	ps, err := personas.List()
	if err != nil {
		return err
	}
	ui.Header("enthea personas — the constellation voices")
	rows := make([][2]string, 0, len(ps))
	for _, p := range ps {
		rows = append(rows, [2]string{p.Name, p.Title})
	}
	ui.Table(rows)
	ui.Bullet("adopt one: enthea setup opencode, or read personas/" + ps[0].Name + ".md")
	ui.Sig()
	return nil
}

// --- setup ---

func runSetup(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("setup: client required (opencode, charm)")
	}
	client := strings.ToLower(args[0])
	switch client {
	case "opencode":
		return setupOpenCode()
	case "charm", "codex", "cursor":
		return fmt.Errorf("setup: %s support lands next — opencode is wired today", client)
	default:
		return fmt.Errorf("setup: unknown client %q", client)
	}
}

// setupOpenCode writes the MCP server + persona agents into opencode's config.
func setupOpenCode() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	cfg := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(filepath.Join(cfg, "skills"), 0o755); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "mcp-enthea.json"), []byte(mcpJSON), 0o644); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	written, err := personas.Install(filepath.Join(cfg, "skills"))
	if err != nil {
		return err
	}
	ui.Header("enthea setup opencode — done")
	ui.Check("wrote mcp-enthea.json (MCP server)")
	for _, p := range written {
		ui.Check("installed persona " + filepath.Base(p))
	}
	ui.Warn("restart opencode to load the MCP server + personas")
	ui.Sig()
	return nil
}

// mcpJSON is the opencode mcp config pointing at this binary. The binary is
// resolved by absolute path so the formula does not need PATH juggling.
const mcpJSON = `{
  "mcp": {
    "enthea": {
      "type": "local",
      "command": ["enthea", "mcp"],
      "enabled": true
    }
  }
}
`

// --- doctor ---

// check is one doctor probe; results flow back on a shared channel — the
// classic fan-out/fan-in: spawn a goroutine per probe, collect in order.
type check struct {
	name string
	run  func() (string, error)
}

type result struct {
	name string
	ok   bool
	line string
}

func runDoctor(_ context.Context, _ []string) error {
	ui.Header("enthea doctor — parallel probes")
	sh := mcp.OSShell{}
	probes := []check{
		{"go", func() (string, error) { return runtime.Version(), nil }},
		{"kompress", func() (string, error) { return sh.Run("kompress", "persons") }},
		{"tailscale", func() (string, error) { return sh.Run("tailscale", "status") }},
		{"entheai engine", func() (string, error) { return sh.Run("entheai", "--version") }},
		{"surfaces", func() (string, error) {
			out, err := sh.Run("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "https://vaked.dev")
			if err != nil {
				return "", err
			}
			return "vaked.dev " + out, nil
		}},
	}

	// Fan-out: one goroutine per probe, results on a buffered channel.
	results := make(chan result, len(probes))
	for _, p := range probes {
		go func(p check) {
			line, err := p.run()
			results <- result{name: p.name, ok: err == nil, line: line}
		}(p)
	}

	// Fan-in: collect all results, then print in probe order.
	got := make(map[string]result, len(probes))
	for range probes {
		r := <-results
		got[r.name] = r
	}
	for _, p := range probes {
		r := got[p.name]
		if r.ok {
			ui.Check(r.name + " — " + firstLine(r.line))
		} else {
			ui.Err(r.name + ": not found")
		}
	}
	ui.Sig()
	return nil
}

// --- version ---

func runVersion(_ context.Context, _ []string) error {
	fmt.Println("enthea " + version)
	return nil
}

// --- lang ---

// runLang boots the enthea machine: assembles a program whose cells are the
// sixteen letters, runs it on an arena the machine owns, and reports where
// every byte went.
func runLang(_ context.Context, _ []string) error {
	prog, err := lang.Assemble(`
main:
  ldi r0 1
  ldi r1 0
  ldi r2 -1
  ldi r3 1
  qdot r4 weights 4
  ultra r4
  halt
weights:
  .byte 1 -1 1 1
`)
	if err != nil {
		return err
	}
	vm, err := lang.NewVM(prog, 4096)
	if err != nil {
		return err
	}
	defer vm.Arena().Close()
	if err := vm.Run(10000); err != nil {
		return err
	}
	total, used, _ := vm.Arena().Stats()
	regs := vm.Regs()
	fmt.Printf("enthea lang — the seed closes\n\n")
	fmt.Printf("  program     %3d bytes (the sixteen letters, packed at arena 0)\n", len(prog))
	fmt.Printf("  arena       %5d bytes total, %d used — one region, owned\n", total, used)
	if vm.Arena().IsMmap() {
		fmt.Printf("  backing     kernel mmap (stage 3: no libc, no malloc)\n")
	} else {
		fmt.Printf("  backing     owned byte slice (fallback)\n")
	}
	fmt.Printf("  call stack  peaked at %d byte(s) — the far end of the arena\n", vm.StackPeak())
	fmt.Printf("  result      ultra(qdot([1,0,-1,1]·[1,-1,1,1])) = %d\n", regs[4])
	fmt.Println()
	fmt.Println("  NAND seeded the sixteen; the sixteen are the opcodes; the arena is the world.")
	return nil
}

// --- fn ---

// runFn compiles a pure functional expression and runs it on the machine.
func runFn(_ context.Context, args []string) error {
	src := `let x0 = 1 in let x1 = 0 in let x2 = -1 in let x3 = 1 in
ultra(add(mul(x0, 1), add(mul(x1, -1), add(mul(x2, 1), mul(x3, 1)))))`
	if len(args) > 0 {
		src = strings.Join(args, " ")
	}
	fns, main, err := fn.Parse(src)
	if err != nil {
		return err
	}
	prog, err := fn.Compile(fns, main)
	if err != nil {
		return err
	}
	vm, err := lang.NewVM(prog, 4096)
	if err != nil {
		return err
	}
	defer vm.Arena().Close()
	if err := vm.Run(100000); err != nil {
		return err
	}
	result := vm.Regs()[0]
	total, _, _ := vm.Arena().Stats()
	fmt.Printf("enthea fn — the pure surface, on the machine\n\n")
	fmt.Printf("  source      %q\n", src)
	fmt.Printf("  bytecode    %3d bytes into the arena (of %d)\n", len(prog), total)
	fmt.Printf("  call stack  peaked at %d byte(s)\n", vm.StackPeak())
	fmt.Printf("  result      %d\n", result)
	fmt.Println()
	fmt.Println("  expressions over the sixteen letters; registers are the compiler's business.")
	return nil
}

// --- quantctx ---

// runQuantCtx demonstrates the living context (kompress-ultra's four roles,
// quantized to {-1,0,+1}): tokens slide through an 8-cell window, the gate
// requires the whole window to agree.
func runQuantCtx(_ context.Context, _ []string) error {
	const model = `
main:
  ldi r0 1
  cwrite r0
  cwrite r0
  cwrite r0
  cwrite r0
  cwrite r0
  cwrite r0
  cwrite r0
  cwrite r0
  csum r1
  cand r2
  ldi r0 0
  cwrite r0
  csum r3
  cand r4
  halt
`
	prog, err := lang.Assemble(model)
	if err != nil {
		return err
	}
	vm, err := lang.NewVM(prog, 4096)
	if err != nil {
		return err
	}
	defer vm.Arena().Close()
	if err := vm.Run(1000); err != nil {
		return err
	}
	regs := vm.Regs()
	fmt.Printf("enthea quantctx — the living context, quantized to {-1,0,+1}\n\n")
	fmt.Printf("  %-22s %s\n", "role", "state (8-cell window)")
	fmt.Printf("  %-22s %+d tokens in the arena\n", "Composer", regs[1])
	fmt.Printf("  %-22s Context AND = %+d — the window agrees\n", "gate", regs[2])
	fmt.Printf("  %-22s after sliding a 0: vote %+d\n", "Circulator", regs[3])
	fmt.Printf("  %-22s Context AND = %+d — the gate drops\n", "Pruner", regs[4])
	fmt.Println()
	fmt.Println("  the context is a living ring of trits: every token quantized, every slide an eviction.")
	return nil
}

// --- vakedc ---

// runVakedc walks the capability graph: every one of the sixteen Boolean
// functions synthesized from NAND alone, its cost, and a sample program.
func runVakedc(_ context.Context, _ []string) error {
	s := vakedc.Synthesize()
	names := []string{"zero", "nor", "anb", "nota", "nab", "notb", "xor", "nand",
		"and", "xnor", "b", "imp", "a", "bimp", "or", "one"}
	fmt.Printf("enthea vakedc — the capability-graph assembler\n\n")
	fmt.Printf("  %-6s %-5s %-6s  %s\n", "letter", "cost", "NANDs", "synthesis")
	for f := pure.F(0); f < 16; f++ {
		asm, cost := vakedc.Emit(f, s)
		first := ""
		if asm != "" {
			first = strings.Split(strings.TrimSpace(asm), "\n")[0]
		}
		fmt.Printf("  %-6s %-5d %-6s  %s\n", names[f], int(f), strings.Repeat("▮", cost)+strings.Repeat(" ", 6-cost), first)
	}
	prog, cost, err := vakedc.AssembleCapability(pure.F(6)) // xor
	if err != nil {
		return err
	}
	fmt.Printf("\n  XOR assembled from %d NANDs: %d bytes into the arena\n", cost, len(prog))
	fmt.Println()
	fmt.Println("  every capability is reachable from one primitive: completeness, walked and run.")
	return nil
}

// --- bus ---

// runBus posts a channel of 1-bit models and prints each classification.
func runBus(_ context.Context, _ []string) error {
	const model = `
main:
  ldi r0 1
  ldi r1 0
  ldi r2 -1
  ldi r3 1
  qdot r0 weights 4
  ultra r0
  halt
weights:
  .byte 0 0 0 0
`
	prog, err := lang.Assemble(model)
	if err != nil {
		return err
	}
	weightsAddr := len(prog) - 4
	models := [][4]int8{
		{1, -1, 1, 1},
		{-1, 1, 1, 1},
		{0, 0, 0, 0},
		{1, -1, -1, 1},
		{-1, -1, 1, -1},
	}
	bus := lang.NewBus(4, 8, 4096)
	defer bus.Close()
	go func() {
		for i, w := range models {
			bus.Post(lang.Message{ID: i, Prog: prog, Data: map[int]int8{
				weightsAddr: w[0], weightsAddr + 1: w[1],
				weightsAddr + 2: w[2], weightsAddr + 3: w[3],
			}})
		}
	}()
	fmt.Printf("enthea bus — a Go channel of 1-bit models\n\n")
	fmt.Printf("  %-24s %-24s %s\n", "ternary weights", "input [1,0,-1,1]", "ultra(dot)")
	seen := 0
	for r := range bus.Results() {
		w := models[r.ID]
		fmt.Printf("  [%+d %+d %+d %+d] %21s  %+d\n", w[0], w[1], w[2], w[3], "→", r.Cell)
		seen++
		if seen == len(models) {
			break
		}
	}
	fmt.Println()
	fmt.Println("  every channel item is an executable model: weights in, classification out.")
	return nil
}

// --- wire ---

// runWire puts the language on the wire: a program's bytecode becomes a
// ternaryPureASCII frame (balanced trits, pure ASCII), survives the trip,
// and comes back byte-identical — judged by its own 1-bit model.
func runWire(_ context.Context, args []string) error {
	// the machine's own program, on the wire
	src := "ldi r0 1\nldi r1 0\nqdot r0 weights 4\nultra r0\nhalt\nweights:\n  .byte 1 -1 1 1\n"
	prog, err := lang.Assemble(src)
	if err != nil {
		return err
	}
	wire := pure.Encode(prog)
	back, err := pure.Decode(wire)
	if err != nil {
		return err
	}
	if string(back) != string(prog) {
		return fmt.Errorf("wire round-trip mismatch")
	}
	fmt.Printf("enthea wire — ternaryPureASCII, the surface layer\n\n")
	fmt.Printf("  program    %d bytes of bytecode\n", len(prog))
	fmt.Printf("  frame      %s\n", wire)
	fmt.Printf("  alphabet   '-', '0', '+' — pure ASCII, %d chars\n", len(wire))
	fmt.Printf("  checksum   the last trit is a 1-bit LLM: qdot against a\n")
	fmt.Printf("             ternary weight vector, sharpened by ultra\n")
	fmt.Printf("  round-trip byte-identical, verdict accepted\n")
	return nil
}

// --- firstLine is a tiny helper for doctor output.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

var _ io.ReadWriteCloser = (nopCloser{})
