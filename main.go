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
)

const version = "0.1.6"

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

// firstLine is a tiny helper for doctor output.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

var _ io.ReadWriteCloser = (nopCloser{})
