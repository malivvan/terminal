//go:build unix

// Package main relays a remote shell over SSH into a local TERMINAL terminal. It
// dials an SSH server, requests a PTY + interactive shell, wraps the session's
// stdin/stdout as an io.ReadWriteCloser, and hands it to vt.StartWithPty. Local
// key/mouse events are forwarded; window resizes are propagated to the remote
// side via SSH "window-change".
//
// The server-side counterpart to this client is pty.ApplyTerminalModes in
// github.com/malivvan/pty, which a host uses to honour the requested modes.
//
// Run with:
//
//	go run ./demo/32_ssh_pty_proxy -addr host:22 -user me
//	                               [-pass secret] [-key ~/.ssh/id_ed25519]
package main

import (
	"flag"
	"io"
	"log"
	"os"

	"github.com/gdamore/tcell/v3"
	"github.com/malivvan/crypto/ssh"
	"github.com/malivvan/terminal"
)

// sshPTY adapts an *ssh.Session to the io.ReadWriteCloser (+ Resize) the VT
// expects from a pty.
type sshPTY struct {
	sess *ssh.ClientSession
	in   io.WriteCloser
	out  io.Reader
}

func (p *sshPTY) Read(b []byte) (int, error)  { return p.out.Read(b) }
func (p *sshPTY) Write(b []byte) (int, error) { return p.in.Write(b) }
func (p *sshPTY) Close() error {
	_ = p.in.Close()
	return p.sess.Close()
}

// Resize is picked up by vt.Resize via an interface assertion.
func (p *sshPTY) Resize(w, h int) error { return p.sess.WindowChange(h, w) }

func authMethods(pass, keyPath string) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if keyPath != "" {
		data, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if pass != "" {
		methods = append(methods, ssh.Password(pass))
	}
	return methods, nil
}

func main() {
	addr := flag.String("addr", "localhost:22", "SSH server address host:port")
	user := flag.String("user", os.Getenv("USER"), "SSH username")
	pass := flag.String("pass", "", "SSH password (optional)")
	key := flag.String("key", "", "path to private key (optional)")
	flag.Parse()

	methods, err := authMethods(*pass, *key)
	if err != nil {
		log.Fatal("auth:", err)
	}
	if len(methods) == 0 {
		log.Fatal("provide -pass and/or -key for authentication")
	}

	client, err := ssh.Dial("tcp", *addr, &ssh.ClientConfig{
		User:            *user,
		Auth:            methods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // demo only
	})
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		log.Fatal("session:", err)
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		log.Fatal(err)
	}
	defer screen.Fini()
	w, h := screen.Size()

	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err := sess.RequestPty("xterm-256color", h, w, modes); err != nil {
		log.Fatal("request pty:", err)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := sess.Shell(); err != nil {
		log.Fatal("shell:", err)
	}

	vt := terminal.NewWithSize(w, h)
	vt.SetSurface(screen)
	if err := vt.StartWithPty(&sshPTY{sess: sess, in: stdin, out: stdout}); err != nil {
		log.Fatal(err)
	}
	defer vt.Close()

	redraw := make(chan struct{}, 1)
	vt.SetRedrawHandler(func() {
		select {
		case redraw <- struct{}{}:
		default:
		}
	})
	vt.Attach(func(ev tcell.Event) {
		select {
		case screen.EventQ() <- ev:
		default:
		}
	})

	draw := func() {
		vt.Draw()
		screen.Show()
	}
	draw()

	for {
		select {
		case <-redraw:
		case ev, ok := <-screen.EventQ():
			if !ok {
				return
			}
			switch ev := ev.(type) {
			case *terminal.EventClosed:
				return
			case *tcell.EventResize:
				nw, nh := ev.Size()
				vt.Resize(nw, nh)
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyF10 {
					return
				}
				vt.HandleEvent(ev)
			default:
				vt.HandleEvent(ev)
			}
		}
		draw()
	}
}
