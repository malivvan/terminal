// Package main runs three headless VTs concurrently, feeds each of them
// a ticker output on its own goroutine, and periodically composites
// their visible screen state side by side into stdout for five seconds.
//
// This illustrates the fundamental multiplexing pattern: N independent
// terminal state machines advancing in parallel, aggregated by the host.
package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/malivvan/terminal"
)

func feeder(h *terminal.Terminal, color int, name string, tick time.Duration, stop <-chan struct{}) {
	i := 0
	prefix := fmt.Sprintf("\x1b[1;%dm%s\x1b[0m ", color, name)
	for {
		select {
		case <-stop:
			return
		case <-time.After(tick):
			_, _ = h.FeedString(fmt.Sprintf("%stick %04d\r\n", prefix, i))
			i++
		}
	}
}

func main() {
	panes := []*terminal.Terminal{}
	for i := 0; i < 3; i++ {
		h, err := terminal.NewTerminal(20, 5)
		if err != nil {
			panic(err)
		}
		panes = append(panes, h)
	}
	defer func() {
		for _, h := range panes {
			h.Close()
		}
	}()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); feeder(panes[0], 31, "red   ", 120*time.Millisecond, stop) }()
	go func() { defer wg.Done(); feeder(panes[1], 32, "green ", 200*time.Millisecond, stop) }()
	go func() { defer wg.Done(); feeder(panes[2], 34, "blue  ", 300*time.Millisecond, stop) }()

	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			close(stop)
			wg.Wait()
			return
		case <-ticker.C:
			var b strings.Builder
			ll := make([][]string, 3)
			for i, h := range panes {
				ll[i] = h.Lines()
			}
			rows := len(ll[0])
			for r := 0; r < rows; r++ {
				fmt.Fprintf(&b, "%-20s | %-20s | %-20s\n",
					strings.TrimRight(ll[0][r], " "),
					strings.TrimRight(ll[1][r], " "),
					strings.TrimRight(ll[2][r], " "))
			}
			fmt.Print("\x1b[H\x1b[2J")
			fmt.Print(b.String())
		}
	}
}
