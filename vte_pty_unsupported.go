//go:build plan9 || js || wasip1

package terminal

import (
	"io"
	"os/exec"

	"github.com/malivvan/pty"
)

// startPTY reports that this platform has no pseudo-terminal a command can be
// started on.
//
// A session can still be driven on these systems with StartWithPty, which takes
// any io.ReadWriteCloser; the js/wasm demo bridges the browser that way.
func startPTY(*exec.Cmd, int, int) (io.ReadWriteCloser, *sessionCmd, error) {
	return nil, nil, pty.ErrUnsupported
}
