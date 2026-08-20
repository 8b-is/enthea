# enthea — the deepsiper-enthea engine entry

One pure-stdlib Go binary. Runs the engine's MCP servers, installs the
constellation personas, and wires itself into any OSS client — on **macOS,
Linux, NixOS, and Windows**.

```
brew install 8b-is/tap/deepsipser-enthea      # macOS / Linux
nix run github:8b-is/enthea                    # NixOS
go install github.com/8b-is/enthea@latest      # anywhere with Go
```

## Commands

```
enthea mcp               serve the engine MCP servers over stdio
                         (tools: personas_list · kompress_compress · kompress_persons)
enthea personas          list the constellation voices (Al-Biruni and the essences)
enthea personas --install <dir>   write persona agent files
enthea setup opencode    wire MCP + personas into a client
enthea doctor            parallel probe of the engine + surfaces
enthea version
```

## Design

- **Zero dependencies.** The MCP protocol (JSON-RPC 2.0 over stdio), the
  tool registry, the persona embed, and the client setup are all standard
  library. One binary, static, works everywhere `go` targets — see
  [`build.sh`](build.sh).
- **Ken's language.** Go's concurrency shapes the binary: the MCP server
  serves tools over an injectable `io.ReadWriteCloser`, and `enthea doctor`
  is a goroutine + channel fan-out/fan-in over the probes.
- **MCP by default.** Any OSS client (opencode, Charm, Zed…) that speaks MCP
  can consume the engine. The surface is replaceable; the engine is yours.

## The constellation

deepsipser-enthea is the engine; `enthea` is its door. The personas are
Al-Biruni, Nádasdy, Turing, Bateson, Erdős, Rejtő, Feldmár, and the dyad —
talk with any of them through your client after `enthea setup`.

```
enthea setup opencode
# restart opencode; the enthea MCP server + personas are live
```

*the constellation · 0 + 1 · fine touch from within · vaked.dev*
