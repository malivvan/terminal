package terminal

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionRecording(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()

	sr := NewRecorder(h)
	_, err := sr.FeedString("echo hello\r\n")
	assert.NoError(t, err)
	sr.Snapshot()
	sr.Close()

	data, err := sr.ExportAsciinema()
	assert.NoError(t, err)
	assert.Greater(t, len(data), 0)

	raw, err := sr.ExportRaw()
	assert.NoError(t, err)
	assert.Greater(t, len(raw), 0)
}

func TestSessionPlayer(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()

	sr := NewRecorder(h)
	_, _ = sr.FeedString("abc")
	sr.Close()

	raw, err := sr.ExportRaw()
	require.NoError(t, err)

	player, err := NewPlayer(raw)
	require.NoError(t, err)

	frameCount := 0
	for {
		_, err := player.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		frameCount++
	}
	assert.GreaterOrEqual(t, frameCount, 1)

	cur, total := player.Pos()
	assert.Equal(t, cur, total)

	assert.Contains(t, player.Metadata(), "width")
}

func TestSessionPlayerReader(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()
	sr := NewRecorder(h)
	sr.FeedString("x")
	sr.Close()
	raw, _ := sr.ExportRaw()

	// Test with valid data.
	player, err := NewPlayerFromReader(&byteReader{data: raw})
	assert.NoError(t, err)
	f, err := player.Next()
	assert.NoError(t, err)
	assert.NotNil(t, f)
}

func TestSessionPlayer_Seek(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()
	sr := NewRecorder(h)
	sr.FeedString("first\r\n")
	sr.Close()

	raw, _ := sr.ExportRaw()
	player, _ := NewPlayer(raw)

	// Current returns current frame without advancing.
	f := player.Current()
	assert.NotZero(t, f.Timestamp)

	// Seek past all frames.
	err := player.Seek(1 * time.Hour)
	assert.ErrorIs(t, err, io.EOF)
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestSessionPlayer_Elapsed(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()

	sr := NewRecorder(h)
	_, _ = sr.FeedString("a")
	_, _ = sr.FeedString("b")
	sr.Close()

	raw, _ := sr.ExportRaw()
	player, _ := NewPlayer(raw)

	// Before any Next call, elapsed should be zero.
	assert.Equal(t, time.Duration(0), player.Elapsed())

	f1, err := player.Next()
	require.NoError(t, err)
	// Elapsed should now match the first frame's timestamp.
	assert.Equal(t, f1.Timestamp, player.Elapsed())

	f2, err := player.Next()
	require.NoError(t, err)
	// Elapsed should advance to the second frame's timestamp.
	assert.Equal(t, f2.Timestamp, player.Elapsed())
	// Ensure it's greater or equal to the first frame (monotonic).
	assert.GreaterOrEqual(t, f2.Timestamp, f1.Timestamp)
}

func TestSessionPlayer_Current(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()

	sr := NewRecorder(h)
	_, _ = sr.FeedString("first")
	_, _ = sr.FeedString("second")
	sr.Close()

	raw, _ := sr.ExportRaw()
	player, _ := NewPlayer(raw)

	// Current returns first frame without advancing.
	cur1 := player.Current()
	assert.NotZero(t, cur1.Timestamp)

	// Call Current again — should be the same frame (not advancing).
	cur2 := player.Current()
	assert.Equal(t, cur1.Timestamp, cur2.Timestamp)
	assert.Equal(t, cur1.Input, cur2.Input)

	// Advance to next.
	_, err := player.Next()
	require.NoError(t, err)

	// Now Current should return the second frame.
	cur3 := player.Current()
	if len(player.frames) > 1 {
		assert.NotEqual(t, cur1.Timestamp, cur3.Timestamp)
	}
}

func TestSessionPlayer_Seek_Middle(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()

	sr := NewRecorder(h)
	_, _ = sr.FeedString("first\r\n")
	_, _ = sr.FeedString("second\r\n")
	sr.Close()

	raw, _ := sr.ExportRaw()
	player, _ := NewPlayer(raw)

	total := len(player.frames)

	// We know frames have timestamps. Seek to a tiny offset to land after
	// the first frame but not past all frames.
	err := player.Seek(time.Nanosecond)
	require.NoError(t, err)
	cur, tot := player.Pos()
	assert.Greater(t, tot, 0, "should have frames")
	_ = cur
	_ = total
}

func TestSessionPlayer_InvalidJSON(t *testing.T) {
	_, err := NewPlayer([]byte("not-json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid recording data")
}

func TestSessionRecording_ExportAsciinema_Empty(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()

	sr := NewRecorder(h)
	// Close immediately without feeding anything.
	sr.Close()

	data, err := sr.ExportAsciinema()
	assert.NoError(t, err)
	// Should have a header line (JSON) followed by zero or more frame lines.
	assert.Greater(t, len(data), 0)
	assert.Contains(t, string(data), `"version":2`)
}

func TestSessionRecording_FeedReturn(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()

	sr := NewRecorder(h)
	defer sr.Close()

	n, err := sr.FeedString("hello")
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
}

func TestSessionRecording_Metadata(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()

	sr := NewRecorder(h)
	defer sr.Close()

	meta := sr.Metadata
	require.NotNil(t, meta)
	assert.Contains(t, meta, "width")
	assert.Contains(t, meta, "height")
	assert.NotEmpty(t, meta["width"])
	assert.NotEmpty(t, meta["height"])
	assert.Equal(t, "80", meta["width"])
	assert.Equal(t, "24", meta["height"])
}

// TestSessionPlayer_Current_PastEnd tests Current after all frames consumed.
func TestSessionPlayer_Current_PastEnd(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	defer h.Close()

	sr := NewRecorder(h)
	sr.Close()

	raw, _ := sr.ExportRaw()
	player, _ := NewPlayer(raw)

	// Drain all frames.
	for {
		_, err := player.Next()
		if err == io.EOF {
			break
		}
	}

	// Current should return zero Frame past end.
	f := player.Current()
	assert.Equal(t, Frame{}, f)
}

// TestSessionRecording_FeedError tests Feed when headless is closed.
func TestSessionRecording_FeedError(t *testing.T) {
	h, _ := NewTerminal(80, 24)
	h.Close() // close headless first

	sr := NewRecorder(h)
	defer sr.Close()

	_, err := sr.FeedString("hello")
	assert.Error(t, err)
}

// TestSessionPlayerFromReader_Error tests NewPlayerFromReader with
// a reader that fails.
func TestSessionPlayerFromReader_Error(t *testing.T) {
	_, err := NewPlayerFromReader(&errReader{})
	assert.Error(t, err)
}

// errReader implements io.Reader that always returns an error.
type errReader struct{}

func (r *errReader) Read(p []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

// TestSessionRecording_CoarseClock covers platforms whose clock only advances
// on its timer tick (~15ms on Windows runners): events recorded back-to-back
// read the same clock value, and frames must not come out zero or duplicate.
func TestSessionRecording_CoarseClock(t *testing.T) {
	const tick = 15 * time.Millisecond

	// Frozen clock: every observation inside a tick reads the same value.
	prev := time.Duration(0)
	for i := 0; i < 4; i++ {
		next := advanceStamp(tick, prev)
		assert.NotZero(t, next, "stamp %d must not be zero", i)
		assert.Greater(t, next, prev, "stamp %d must exceed its predecessor", i)
		prev = next
	}

	// The recorder must wire those stamps into every frame it appends.
	h, _ := NewTerminal(80, 24)
	defer h.Close()

	sr := NewRecorder(h)
	_, _ = sr.FeedString("first")
	_, _ = sr.FeedString("second")
	sr.Close()

	raw, err := sr.ExportRaw()
	require.NoError(t, err)

	var rec Recording
	require.NoError(t, json.Unmarshal(raw, &rec))
	require.NotEmpty(t, rec.Frames)

	prev = 0
	for i, f := range rec.Frames {
		assert.NotZero(t, f.Timestamp, "frame %d must not be stamped zero", i)
		assert.Greater(t, f.Timestamp, prev, "frame %d must follow frame %d", i, i-1)
		prev = f.Timestamp
	}
}
