// Package lang — the enthea language (ANDROMEDA stage 1).
//
// The whole world is one arena: program, heap, and call stack share a single
// region that the VM owns. On unix the arena is a kernel mmap; elsewhere it
// falls back to an owned byte slice. Either way it is one flat region the VM
// allocates by bumping — never free'd.
package lang

import "fmt"

// Arena is a single owned region with a bump allocator.
type Arena struct {
	data []byte
	mmap bool
	off  int
	peak int
}

// Alloc bumps `n` bytes and returns their offset.
func (a *Arena) Alloc(n int) (int, error) {
	if n < 0 {
		return 0, fmt.Errorf("lang: negative allocation %d", n)
	}
	if a.off+n > len(a.data) {
		return 0, fmt.Errorf("lang: arena exhausted (%d used, %d wanted of %d)", a.off, n, len(a.data))
	}
	off := a.off
	a.off += n
	if a.off > a.peak {
		a.peak = a.off
	}
	return off, nil
}

// View is the full arena as one byte slice.
func (a *Arena) View() []byte { return a.data }

// Stats reports (total, used, peak) in bytes.
func (a *Arena) Stats() (total, used, peak int) {
	return len(a.data), a.off, a.peak
}
