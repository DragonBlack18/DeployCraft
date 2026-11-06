// Package runner executes explicit programs without adding an intermediary shell.
// Windows processes are created inside a kill-on-close Job Object.
package runner

import (
	"bytes"
	"sync"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

const tailLimit = 16 * 1024

type nativeProcess struct{}
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
}

// Write retains at most tailLimit bytes, even for a single very large write.
func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(p)
	if n >= tailLimit {
		t.buf = append(t.buf[:0], p[n-tailLimit:]...)
	} else {
		if excess := len(t.buf) + n - tailLimit; excess > 0 {
			t.buf = t.buf[excess:]
		}
		t.buf = append(t.buf, p...)
	}
	return n, nil
}
func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(bytes.ToValidUTF8(t.buf, []byte("?")))
}
func New() domain.Process { return nativeProcess{} }
