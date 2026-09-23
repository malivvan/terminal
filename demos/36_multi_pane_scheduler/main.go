// Package main demonstrates fair, deterministic scheduling across several
// independent Terminal terminals, each advancing on its own logical clock.
// A virtual scheduler ticks a shared logical time; every pane produces output
// at its own interval, and panes that come due on the same tick are serviced
// round-robin so none can starve the others.
//
// Run with:
//
//	go run ./demo/36_multi_pane_scheduler
package main

import (
	"fmt"
	"strings"

	"github.com/malivvan/terminal"
)

type pane struct {
	name     string
	interval int // logical ticks between frames (independent clock)
	h        *terminal.Terminal
	frames   []string
	next     int // index into frames
	served   int
}

func (p *pane) due(clock int) bool {
	return p.next < len(p.frames) && clock%p.interval == 0
}

func (p *pane) step() {
	_, _ = p.h.FeedString(p.frames[p.next])
	p.next++
	p.served++
}

func newPane(name string, interval, n int) *pane {
	h, err := terminal.NewTerminal(24, 3)
	if err != nil {
		panic(err)
	}
	frames := make([]string, n)
	for i := range frames {
		if i == 0 {
			frames[i] = fmt.Sprintf("\x1b[2J\x1b[H%s: frame %d", name, i)
		} else {
			frames[i] = fmt.Sprintf("\r\n%s: frame %d", name, i)
		}
	}
	return &pane{name: name, interval: interval, h: h, frames: frames}
}

func main() {
	panes := []*pane{
		newPane("fast", 1, 8), // ticks every step
		newPane("med", 2, 6),  // every other step
		newPane("slow", 3, 4), // every third step
	}

	fmt.Println("virtual scheduler (round-robin among simultaneously-due panes):")
	rr := 0 // rotating start index guarantees fairness on contended ticks
	const ticks = 12
	for clock := 0; clock < ticks; clock++ {
		var served []string
		for i := 0; i < len(panes); i++ {
			p := panes[(rr+i)%len(panes)]
			if p.due(clock) {
				p.step()
				served = append(served, p.name)
			}
		}
		rr = (rr + 1) % len(panes)
		if len(served) > 0 {
			fmt.Printf("  t=%02d serviced: %s\n", clock, strings.Join(served, ", "))
		}
	}

	fmt.Println("\nservice counts (fairness):")
	for _, p := range panes {
		fmt.Printf("  %-5s interval=%d served=%d frames\n", p.name, p.interval, p.served)
	}

	fmt.Println("\nfinal pane screens:")
	for _, p := range panes {
		fmt.Printf("  [%s]\n", p.name)
		for _, l := range p.h.Lines() {
			if s := strings.TrimRight(l, " "); s != "" {
				fmt.Printf("    %s\n", s)
			}
		}
		_ = p.h.Close()
	}
}
