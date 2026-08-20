#!/usr/bin/env nu
# enthea — universal installer (nushell edition).
# Same as install.sh, but in nushell for the pipes-and-tables crowd.
#
#   nu install.nu
let version = ($env.ENTHEA_VERSION? | default "latest")
let dest = ($env.ENTHEA_DEST? | default $"($env.HOME)/.local/bin")
let base = $"https://github.com/8b-is/enthea/releases/download/($version)"

let os = (uname -s | str downcase)
let raw_arch = (uname -m)
let arch = match $raw_arch {
  "x86_64" | "amd64" => "amd64"
  "aarch64" | "arm64" => "arm64"
  _ => $raw_arch
}
let file = $"enthea-($os)-($arch)"
let url = $"($base)/($file)"

print $"enthea ($version) ($os)/($arch) -> ($dest)"
mkdir -p $dest
http get $url | save --force $"($dest)/enthea"
chmod +x $"($dest)/enthea"

print $"\nenthea installed to ($dest)/enthea"
if ($"($dest)" not-in ($env.PATH | split row (char esep))) {
  print $'add it to your PATH:  export PATH="($dest):$PATH"'
}
print "then: enthea setup opencode   # wire MCP + personas into your client"
