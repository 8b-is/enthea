# Zero-copy snapshots and `SIGBUS`

Status: recorded, no code change — the current arena has no file backing.

## Context

`mmap` reserves address space eagerly and loads blocks on first touch. If the
backing inode shrinks (`ftruncate`, `O_TRUNC`) while a mapping is live, or a
page past EOF is touched, the kernel cannot back the virtual page and delivers
`SIGBUS`, which kills the process. Catching it in user space (`sigaction` +
`sigsetjmp`) requires a jump buffer around every dereference, inhibits
dead-store and aliasing optimisation, and is unsound once async tasks migrate
across OS threads.

## Finding in this repository

- `lang/arena_mmap.go` maps `MAP_PRIVATE | MAP_ANON`. Anonymous memory has no
  backing inode, so no truncation can invalidate it and a fault cannot raise
  `SIGBUS` for a shrunken file. The arena is not a reader of external data.
- `pure/ring.go` opens a file with `O_TRUNC` and calls `Truncate`, but performs
  positional I/O only; it never calls `Mmap`. A resize surfaces as a short
  read, not a signal.

No file-backed mapping exists, so there is nothing here to harden. Per the
project rule that changes must fix a real issue, this repository is unchanged.

## Invariant if a file-backed arena is added

Four layers, in this order:

1. **Immutable inode replacement** — write a new temporary inode, `fsync`,
   reduce it to `0400`, then `rename` into place. Never truncate a live file.
2. **Unlinked-inode lifetime** — rely on POSIX keeping a replaced inode's
   blocks until the last `munmap`; readers survive rotation.
3. **Pre-map `fstat` envelope** — require physical `st_size` to equal the
   header's declared length before mapping, so virtual size never exceeds EOF.
4. **Prefault and verify** — `MADV_WILLNEED`, then verify a footer digest at
   open, so late I/O errors fail at construction rather than mid-query.

Canonical, tested implementation: `8b-is/aurora` PR #5,
`tools/zero_copy_snapshot.py` (stdlib only, `AURSNAP1` format). Reuse that
design rather than inventing a second layout.
