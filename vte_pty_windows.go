//go:build windows

package terminal

import (
	"io"
	"os/exec"

	"github.com/malivvan/pty"
)

// startPTY starts c inside a Windows pseudo-console (ConPTY) of the given size
// and returns that pseudo-console as the session's terminal, together with the
// handle to the running process.
//
// Windows has no pseudo-terminal device: a command that expects a terminal is
// hosted by the console host instead, and the host drives it through ConPTY.
func startPTY(c *exec.Cmd, w, h int) (io.ReadWriteCloser, *sessionCmd, error) {
	p, err := pty.NewConPTY(w, h, 0)
	if err != nil {
		return nil, nil, err
	}

	// ConPTY spawns the process itself, inside the pseudo-console, so the
	// command is built from the terminal rather than started directly.
	args := c.Args
	if len(args) > 0 {
		args = args[1:]
	}
	pc := p.Command(c.Path, args...)
	pc.Env = c.Env
	pc.Dir = c.Dir
	if err := pc.Start(); err != nil {
		_ = p.Close()
		return nil, nil, err
	}
	return p, &sessionCmd{process: pc.Process, wait: pc.Wait}, nil
}
