//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package terminal

import (
	"io"
	"os/exec"

	"github.com/malivvan/pty"
)

// startPTY starts c on a freshly allocated pseudo-terminal of the given size
// and returns the master end of that terminal together with the handle to the
// running process.
//
// The command is put in a new session with the slave end of the terminal as its
// controlling terminal, so it behaves like a program started from an
// interactive shell.
func startPTY(c *exec.Cmd, w, h int) (io.ReadWriteCloser, *sessionCmd, error) {
	p, err := pty.StartWithSize(c, ptyWinsize(w, h))
	if err != nil {
		return nil, nil, err
	}
	return p, &sessionCmd{process: c.Process, wait: c.Wait}, nil
}
