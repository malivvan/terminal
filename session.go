package terminal

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// Frame represents a single timestamped event in a session recording.
type Frame struct {
	// Timestamp is the time elapsed since recording start.
	Timestamp time.Duration
	// EventType is "input", "output", or "snapshot".
	EventType string
	// Input holds the bytes injected by the automation (for input frames).
	Input []byte
	// Output holds the bytes emitted by the terminal (for output frames).
	Output []byte
	// ScreenSnapshot is an optional periodic screen capture.
	ScreenSnapshot string
}

// Recording holds a complete recorded terminal session.
type Recording struct {
	Start    time.Time
	Frames   []Frame
	Metadata map[string]string

	mu       sync.Mutex
	h        *Terminal
	done     chan struct{}
	lastSnap time.Time
	lastTS   time.Duration
}

// advanceStamp returns the timestamp to record for an event observed
// `observed` after the recording start, given that the previous frame was
// stamped `prev`.
//
// Timestamps have to increase strictly for replay and Seek to be
// deterministic, but a clock is only as fine-grained as its timer tick — on
// Windows the tick is around 15ms, and rougher still in virtualised runners.
// Events recorded back-to-back therefore read the same clock value, which
// used to stamp the first frame of a recording as an exact zero and later
// frames identically to it: Seek then skipped over frames, and Current
// looked frozen. Nudging such a frame a nanosecond past its predecessor keeps
// the recorded order stable without inventing a meaningful delay.
func advanceStamp(observed, prev time.Duration) time.Duration {
	if observed <= prev {
		return prev + time.Nanosecond
	}
	return observed
}

// stamp returns the timestamp for the next frame to be appended. Callers must
// hold r.mu so that Frames stays ordered by timestamp.
func (r *Recording) stamp() time.Duration {
	ts := advanceStamp(time.Since(r.Start), r.lastTS)
	r.lastTS = ts
	return ts
}

// NewRecorder wraps a Terminal terminal and starts recording all
// input and output events, plus periodic screen snapshots (every second).
func NewRecorder(h *Terminal) *Recording {
	r := &Recording{
		Start:    time.Now(),
		Metadata: map[string]string{},
		h:        h,
		done:     make(chan struct{}),
	}
	w, hh := h.Size()
	r.Metadata["width"] = fmt.Sprintf("%d", w)
	r.Metadata["height"] = fmt.Sprintf("%d", hh)

	// Drain current output so we start clean.
	h.Output()

	// Start background snapshot goroutine.
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.recordSnapshot()
			case <-r.done:
				return
			}
		}
	}()
	return r
}

// recordSnapshot appends a snapshot frame with the current screen state.
func (r *Recording) recordSnapshot() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Frames = append(r.Frames, Frame{
		Timestamp:      r.stamp(),
		EventType:      "snapshot",
		ScreenSnapshot: r.h.String(),
	})
}

// Feed records input bytes and feeds them to the underlying terminal.
// Any output bytes produced by the terminal are also recorded.
func (r *Recording) Feed(b []byte) (int, error) {
	n, err := r.h.Feed(b)
	if err != nil {
		return n, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.h.Output()
	r.Frames = append(r.Frames, Frame{
		Timestamp: r.stamp(),
		EventType: "input",
		Input:     append([]byte(nil), b...),
		Output:    out,
	})
	return n, nil
}

// FeedString is a convenience wrapper around Feed.
func (r *Recording) FeedString(s string) (int, error) {
	return r.Feed([]byte(s))
}

// Snapshot forces an immediate screen snapshot frame.
func (r *Recording) Snapshot() {
	r.recordSnapshot()
}

// ExportAsciinema exports the session recording in asciinema v2 JSON format.
// See https://docs.asciinema.org/manual/asciicast/v2/
func (r *Recording) ExportAsciinema() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	type header struct {
		Version   int               `json:"version"`
		Width     int               `json:"width"`
		Height    int               `json:"height"`
		Timestamp int64             `json:"timestamp,omitempty"`
		Env       map[string]string `json:"env,omitempty"`
	}
	w, h := r.h.Size()
	hdr := header{
		Version: 2,
		Width:   w,
		Height:  h,
		Env:     map[string]string{"TERM": "xterm-256color"},
	}

	out, err := json.Marshal(hdr)
	if err != nil {
		return nil, err
	}
	out = append(out, '\n')

	// Convert frames to asciinema format: [time, "output", input]
	// We replay output bytes followed by input bytes as "output" for simplicity.
	for _, f := range r.Frames {
		data := ""
		if len(f.Output) > 0 {
			data += string(f.Output)
		}
		if len(f.Input) > 0 {
			data += string(f.Input)
		}
		if data == "" && f.ScreenSnapshot == "" {
			continue
		}
		if data == "" {
			data = f.ScreenSnapshot
		}
		sec := f.Timestamp.Seconds()
		line, _ := json.Marshal([]interface{}{sec, "o", data})
		out = append(out, line...)
		out = append(out, '\n')
	}
	return out, nil
}

// ExportRaw returns the recording as a JSON array of frames, suitable
// for replay or analysis.
func (r *Recording) ExportRaw() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return json.Marshal(r)
}

// Close stops the recording and releases the background goroutine.
func (r *Recording) Close() error {
	close(r.done)
	r.Snapshot()
	return nil
}

// Player replays a previously recorded session.
type Player struct {
	frames  []Frame
	pos     int
	meta    map[string]string
	start   time.Time
	elapsed time.Duration
}

// NewPlayer creates a Player from raw exported JSON data.
func NewPlayer(data []byte) (*Player, error) {
	var rec Recording
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("session: invalid recording data: %w", err)
	}
	return &Player{
		frames: rec.Frames,
		meta:   rec.Metadata,
		start:  rec.Start,
	}, nil
}

// NewPlayerFromReader reads a raw JSON recording from an io.Reader.
func NewPlayerFromReader(r io.Reader) (*Player, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return NewPlayer(data)
}

// Next advances to the next frame and returns it. Returns io.EOF when
// all frames have been consumed.
func (p *Player) Next() (*Frame, error) {
	if p.pos >= len(p.frames) {
		return nil, io.EOF
	}
	f := &p.frames[p.pos]
	p.elapsed = f.Timestamp
	p.pos++
	return f, nil
}

// Current returns the current frame without advancing.
func (p *Player) Current() Frame {
	if p.pos >= len(p.frames) {
		return Frame{}
	}
	return p.frames[p.pos]
}

// Seek moves the playback position to the frame at or after the given
// offset from the start of the recording.
func (p *Player) Seek(offset time.Duration) error {
	for i, f := range p.frames {
		if f.Timestamp >= offset {
			p.pos = i
			p.elapsed = f.Timestamp
			return nil
		}
	}
	p.pos = len(p.frames)
	p.elapsed = offset
	return io.EOF
}

// Pos returns the current frame index and total frame count.
func (p *Player) Pos() (current, total int) {
	return p.pos, len(p.frames)
}

// Elapsed returns the elapsed playback time.
func (p *Player) Elapsed() time.Duration {
	return p.elapsed
}

// Metadata returns the recording metadata.
func (p *Player) Metadata() map[string]string {
	return p.meta
}
