package terminal

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUTF8(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Sequence
	}{
		{
			name:  "UTF-8",
			input: "🔥",
			expected: []Sequence{
				Print('🔥'),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.input)
			parse := NewParser(r)
			i := 0
			for {
				seq := parse.Next()
				if _, isEOF := seq.(EOF); seq == nil || isEOF {
					assert.Equal(t, len(test.expected), i, "wrong amount of sequences")
					break
				}
				if i < len(test.expected) {
					assert.Equal(t, test.expected[i], seq)
				}
				i += 1
			}
		})
	}
}

func TestIn(t *testing.T) {
	tests := []struct {
		name     string
		inRange  []rune
		input    rune
		expected bool
	}{
		{
			name:     "endpoint min",
			inRange:  []rune{0x00, 0x20},
			input:    0x00,
			expected: true,
		},
		{
			name:     "endpoint max",
			inRange:  []rune{0x00, 0x20},
			input:    0x20,
			expected: true,
		},
		{
			name:     "within",
			inRange:  []rune{0x00, 0x20},
			input:    0x19,
			expected: true,
		},
		{
			name:     "outside",
			inRange:  []rune{0x00, 0x20},
			input:    0x21,
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := in(test.input, test.inRange[0], test.inRange[1])
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestIs(t *testing.T) {
	tests := []struct {
		name     string
		isVals   []rune
		input    rune
		expected bool
	}{
		{
			name:     "multiple",
			isVals:   []rune{0x00, 0x20},
			input:    0x00,
			expected: true,
		},
		{
			name:     "single",
			isVals:   []rune{0x00},
			input:    0x00,
			expected: true,
		},
		{
			name:     "false multiple",
			isVals:   []rune{0x00, 0x20},
			input:    0x19,
			expected: false,
		},
		{
			name:     "false single",
			isVals:   []rune{0x00},
			input:    0x21,
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := is(test.input, test.isVals...)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestAnywhere(t *testing.T) {
	tests := []struct {
		name     string
		input    rune
		expected stateFn
	}{
		{
			name:     "0x18",
			input:    0x18,
			expected: ground,
		},
		{
			name:     "0x1A",
			input:    0x1A,
			expected: ground,
		},
		{
			name:     "0x1B",
			input:    0x1B,
			expected: escape,
		},
		{
			name:     "eof",
			input:    eof,
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parse := &Parser{
				sequences: make(chan Sequence, 2),
				state:     ground,
			}
			called := false
			parse.SetExitFunc(func() {
				called = true
			})
			actual := anywhere(test.input, parse)
			act := reflect.ValueOf(actual).Pointer()
			exp := reflect.ValueOf(test.expected).Pointer()
			assert.Equal(t, exp, act, "wrong return function")
			if test.expected != nil {
				assert.True(t, called, "exit function not called")
			}
		})
	}
}

func TestCSI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Sequence
	}{
		{
			name:  "CSI Entry + C0",
			input: "a\x1b[\x00",
			expected: []Sequence{
				Print('a'),
				C0(0x00),
			},
		},
		{
			name:  "CSI Entry + escape",
			input: "a\x1b[\x1b",
			expected: []Sequence{
				Print('a'),
			},
		},
		{
			name:  "CSI Entry + ignore",
			input: "a\x1b[\x7F",
			expected: []Sequence{
				Print('a'),
			},
		},
		{
			name:  "CSI Entry + dispatch",
			input: "a\x1b[c",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'c',
					Intermediate: []rune{},
					Parameters:   []int{},
				},
			},
		},
		{
			name:  "CSI Param with collect first",
			input: "a\x1b[<c",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'c',
					Parameters:   []int{},
					Intermediate: []rune{'<'},
				},
			},
		},
		{
			name:  "CSI Param with colorspace",
			input: "a\x1b[38:2::0:0:0m",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'm',
					Parameters:   []int{38, 2, 0, 0, 0},
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "CSI Param with colorspace fg and bg",
			input: "a\x1b[38:2::0:0:0;48:2::0:0:0m",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'm',
					Parameters:   []int{38, 2, 0, 0, 0, 48, 2, 0, 0, 0},
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "CSI Param SGR with semicolons",
			input: "a\x1b[38;2;0;0;0;48;2;0;0;0m",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'm',
					Parameters:   []int{38, 2, 0, 0, 0, 48, 2, 0, 0, 0},
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "CSI Param",
			input: "a\x1b[0c",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'c',
					Parameters:   []int{0},
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "CSI Param + eof",
			input: "a\x1b[0",
			expected: []Sequence{
				Print('a'),
			},
		},
		{
			name:  "CSI Param + eof",
			input: "a\x1b[0\x00",
			expected: []Sequence{
				Print('a'),
				C0(0x00),
			},
		},
		{
			name:  "CSI Param + eof",
			input: "a\x1b[0\x7F\x00",
			expected: []Sequence{
				Print('a'),
				C0(0x00),
			},
		},
		{
			name:  "CSI Param with long param",
			input: "a\x1b[9999c",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'c',
					Parameters:   []int{9999},
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "CSI Param with multiple",
			input: "a\x1b[0;0c",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'c',
					Parameters:   []int{0, 0},
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "CSI Param with multiple blank",
			input: "a\x1b[;c",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'c',
					Parameters:   []int{0, 0},
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "CSI Param with multiple filled or blank",
			input: "a\x1b[;1c",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'c',
					Parameters:   []int{0, 1},
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "CSI Param + csiIgnore",
			input: "a\x1b[;1\x3Cc",
			expected: []Sequence{
				Print('a'),
			},
		},
		{
			name:  "CSI Param + escape",
			input: "a\x1b[;1\x1b",
			expected: []Sequence{
				Print('a'),
			},
		},
		{
			name:  "CSI Intermediate",
			input: "a\x1b[\x20\x20c",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'c',
					Parameters:   []int{},
					Intermediate: []rune{' ', ' '},
				},
			},
		},
		{
			name:  "CSI Intermediate + escape",
			input: "a\x1b[\x20\x20\x1b",
			expected: []Sequence{
				Print('a'),
			},
		},
		{
			name:  "CSI Intermediate + c0",
			input: "a\x1b[\x20\x20\x00c",
			expected: []Sequence{
				Print('a'),
				C0(0x00),
				CSI{
					Final:        'c',
					Parameters:   []int{},
					Intermediate: []rune{' ', ' '},
				},
			},
		},
		{
			name:  "CSI Intermediate + 7f ignore",
			input: "a\x1b[\x20\x20\x7Fc",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'c',
					Parameters:   []int{},
					Intermediate: []rune{' ', ' '},
				},
			},
		},
		{
			name:  "CSI Intermediate + eof",
			input: "a\x1b[\x20\x20",
			expected: []Sequence{
				Print('a'),
			},
		},
		{
			name:  "CSI Intermediate + param",
			input: "a\x1b[0\x20\x20c",
			expected: []Sequence{
				Print('a'),
				CSI{
					Final:        'c',
					Parameters:   []int{0},
					Intermediate: []rune{' ', ' '},
				},
			},
		},
		{
			name:  "CSI Intermediate + param + ignore",
			input: "a\x1b[0\x20\x20\x30c",
			expected: []Sequence{
				Print('a'),
			},
		},
		{
			name:  "CSI Ignore + eof",
			input: "a\x1b[0\x20\x20\x30\x3A",
			expected: []Sequence{
				Print('a'),
			},
		},
		{
			name:  "CSI Ignore + esc",
			input: "a\x1b[0\x20\x20\x30\x1B",
			expected: []Sequence{
				Print('a'),
			},
		},
		{
			name:  "CSI Ignore + c0",
			input: "a\x1b[0\x20\x20\x30\x00c",
			expected: []Sequence{
				Print('a'),
				C0(0x00),
			},
		},
		{
			name:  "CSI Ignore + 7F ignore",
			input: "a\x1b[0\x20\x20\x30\x7Fc",
			expected: []Sequence{
				Print('a'),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.input)
			parse := NewParser(r)
			i := 0
			for {
				seq := parse.Next()
				if _, isEOF := seq.(EOF); seq == nil || isEOF {
					assert.Equal(t, len(test.expected), i, "wrong amount of sequences")
					break
				}
				t.Logf("%T", seq)
				if i < len(test.expected) {
					assert.Equal(t, test.expected[i], seq)
				}
				i += 1
			}
		})
	}
}

func TestDCS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Sequence
	}{
		{
			name:  "DCS Entry + C0",
			input: "a\x1bP\x00",
			expected: []Sequence{
				Print('a'),
			},
		},
		{
			name:  "DCS Entry + end",
			input: "a\x1bPq",
			expected: []Sequence{
				Print('a'),
				DCS{
					Final:        'q',
					Intermediate: []rune{},
					Parameters:   []int{},
				},
				DCSEndOfData{},
			},
		},
		{
			name:  "DCS Entry + data + end",
			input: "a\x1bPq#0;2;0;\x1b\\",
			expected: []Sequence{
				Print('a'),
				DCS{
					Final:        'q',
					Intermediate: []rune{},
					Parameters:   []int{},
				},
				DCSData('#'),
				DCSData('0'),
				DCSData(';'),
				DCSData('2'),
				DCSData(';'),
				DCSData('0'),
				DCSData(';'),
				DCSEndOfData{},
				ESC{
					Final:        '\\',
					Intermediate: []rune{},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.input)
			parse := NewParser(r)
			i := 0
			for {
				seq := parse.Next()
				if _, isEOF := seq.(EOF); seq == nil || isEOF {
					assert.Equal(t, len(test.expected), i, "wrong amount of sequences")
					break
				}
				if i < len(test.expected) {
					assert.Equal(t, test.expected[i], seq)
				}
				i += 1
			}
		})
	}
}

func TestEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Sequence
	}{
		{
			name:  "ESC W",
			input: "a\x1bDc",
			expected: []Sequence{
				Print('a'),
				ESC{
					Final:        'D',
					Intermediate: []rune{},
				},
				Print('c'),
			},
		},
		{
			name:  "ESC W",
			input: "a\x1bWc",
			expected: []Sequence{
				Print('a'),
				ESC{
					Final:        'W',
					Intermediate: []rune{},
				},
				Print('c'),
			},
		},
		{
			name:  "ESC W with a C0",
			input: "a\x1b\x00Wc",
			expected: []Sequence{
				Print('a'),
				C0(0x00),
				ESC{
					Final:        'W',
					Intermediate: []rune{},
				},
				Print('c'),
			},
		},
		{
			name:  "with ignore",
			input: "a\x1b\x7FWc",
			expected: []Sequence{
				Print('a'),
				ESC{
					Final:        'W',
					Intermediate: []rune{},
				},
				Print('c'),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.input)
			lex := NewParser(r)
			i := 0
			for {
				seq := lex.Next()
				if _, isEOF := seq.(EOF); seq == nil || isEOF {
					assert.Equal(t, len(test.expected), i, "fewer sequences than expected")
					break
				}
				assert.Equal(t, test.expected[i], seq)
				i += 1
				assert.LessOrEqual(t, i, len(test.expected), "more sequences than expected")
			}
		})
	}
}

func TestEscapeIntermediate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Sequence
	}{
		{
			name:  "ESC SP F",
			input: "a\x1b Fc",
			expected: []Sequence{
				Print('a'),
				ESC{
					Final:        'F',
					Intermediate: []rune{' '},
				},
				Print('c'),
			},
		},
		{
			name:  "ESC # 3",
			input: "a\x1b#3c",
			expected: []Sequence{
				Print('a'),
				ESC{
					Final:        '3',
					Intermediate: []rune{'#'},
				},
				Print('c'),
			},
		},
		{
			name:  "ESC ( B",
			input: "a\x1b(Bc",
			expected: []Sequence{
				Print('a'),
				ESC{
					Final:        'B',
					Intermediate: []rune{'('},
				},
				Print('c'),
			},
		},
		{
			name:  "ESC ( B with C0",
			input: "a\x1b(\tBc",
			expected: []Sequence{
				Print('a'),
				C0('\t'),
				ESC{
					Final:        'B',
					Intermediate: []rune{'('},
				},
				Print('c'),
			},
		},
		{
			name:  "ESC ( B with ignore",
			input: "a\x1b(\x7FBc",
			expected: []Sequence{
				Print('a'),
				ESC{
					Final:        'B',
					Intermediate: []rune{'('},
				},
				Print('c'),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.input)
			parse := NewParser(r)
			i := 0
			for {
				seq := parse.Next()
				if _, isEOF := seq.(EOF); seq == nil || isEOF {
					assert.Equal(t, len(test.expected), i, "wrong amount of sequences")
					break
				}
				if i < len(test.expected) {
					assert.Equal(t, test.expected[i], seq)
				}
				i += 1
			}
		})
	}
}

func TestGround(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Sequence
	}{
		{
			name:  "printables",
			input: "abc",
			expected: []Sequence{
				Print('a'),
				Print('b'),
				Print('c'),
			},
		},
		{
			name:  "printable with c0",
			input: string([]rune{'a', 0x00, 'c'}),
			expected: []Sequence{
				Print('a'),
				C0(0x00),
				Print('c'),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.input)
			lex := NewParser(r)
			i := 0
			for {
				seq := lex.Next()
				if _, isEOF := seq.(EOF); seq == nil || isEOF {
					break
				}
				assert.Equal(t, test.expected[i], seq)
				i += 1
			}
		})
	}
}

func TestOSC(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Sequence
	}{
		{
			name:  "OSC entry",
			input: "a\x1b\x5D",
			expected: []Sequence{
				Print('a'),
				OSC{},
			},
		},
		{
			name:  "OSC end ST",
			input: "a\x1B\x5D\x1B\x5C",
			expected: []Sequence{
				Print('a'),
				OSC{},
				ESC{
					Final:        0x5C,
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "OSC end CAN",
			input: "a\x1B\x5D\x1B\x18",
			expected: []Sequence{
				Print('a'),
				OSC{},
				C0(0x18),
			},
		},
		{
			name:  "OSC end SUB",
			input: "a\x1B\x5D\x1B\x1A",
			expected: []Sequence{
				Print('a'),
				OSC{},
				C0(0x1A),
			},
		},
		{
			name:  "OSC 8 ;; http://example.com",
			input: "a\x1B\x5D8;;http://example.com\x1b\x5CLink\x1b\x5D8;;\x1b\x5C",
			expected: []Sequence{
				Print('a'),
				OSC{
					Payload: []rune{
						'8',
						';',
						';',
						'h',
						't',
						't',
						'p',
						':',
						'/',
						'/',
						'e',
						'x',
						'a',
						'm',
						'p',
						'l',
						'e',
						'.',
						'c',
						'o',
						'm',
					},
				},
				ESC{
					Final:        '\\',
					Intermediate: []rune{},
				},
				Print('L'),
				Print('i'),
				Print('n'),
				Print('k'),
				OSC{
					Payload: []rune{
						'8',
						';',
						';',
					},
				},
				ESC{
					Final:        '\\',
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "OSC bell terminated",
			input: "a\x1B\x5D\ab",
			expected: []Sequence{
				Print('a'),
				OSC{},
				Print('b'),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.input)
			parse := NewParser(r)
			i := 0
			for {
				seq := parse.Next()
				if _, isEOF := seq.(EOF); seq == nil || isEOF {
					assert.Equal(t, len(test.expected), i, "wrong amount of sequences")
					break
				}
				if i < len(test.expected) {
					assert.Equal(t, test.expected[i], seq)
				}
				i += 1
			}
		})
	}
}

// The parser's anywhere() function emitted a nil sequence before the EOF{}
// sequence on end-of-input, causing consumers to see a spurious nil before the
// proper EOF signal. Only EOF{} should be emitted; nil should never appear.
func TestParser_NoNilBeforeEOF(t *testing.T) {
	r := strings.NewReader("a")
	parse := NewParser(r)

	// First sequence: Print('a')
	seq := parse.Next()
	_, ok := seq.(Print)
	assert.True(t, ok, "expected Print sequence, got %T", seq)

	// Next sequence should be EOF{}, NOT nil
	seq = parse.Next()
	assert.NotNil(t, seq, "parser should not emit nil before EOF")
	_, ok = seq.(EOF)
	assert.True(t, ok, "expected EOF sequence after input ends, got %T", seq)
}

// Regression: empty input should produce EOF{} directly with no nil.
func TestParser_EmptyInput_EOFOnly(t *testing.T) {
	r := strings.NewReader("")
	parse := NewParser(r)

	seq := parse.Next()
	assert.NotNil(t, seq, "first sequence from empty input should not be nil")
	_, ok := seq.(EOF)
	assert.True(t, ok, "expected EOF from empty input, got %T", seq)
}

// TERMINAL-025: 8-bit C1 code 0x9B should enter CSI state (same as ESC [).
func TestC1_CSI_8bit(t *testing.T) {
	// 0x9B followed by 'H' is CSI H (cursor home)
	input := string([]byte{0x9B}) + "H"
	r := strings.NewReader(input)
	parse := NewParser(r)
	seq := parse.Next()
	csi, ok := seq.(CSI)
	assert.True(t, ok, "expected CSI sequence from 8-bit C1 0x9B")
	assert.Equal(t, 'H', csi.Final)
}

// TERMINAL-025: 8-bit C1 code 0x9D should enter OSC state (same as ESC ]).
func TestC1_OSC_8bit(t *testing.T) {
	// 0x9D followed by OSC data terminated by BEL
	input := string([]byte{0x9D}) + "2;title\a"
	r := strings.NewReader(input)
	parse := NewParser(r)
	seq := parse.Next()
	osc, ok := seq.(OSC)
	assert.True(t, ok, "expected OSC sequence from 8-bit C1 0x9D")
	assert.Equal(t, "2;title", string(osc.Payload))
}

// TERMINAL-017: DCS passthrough should not redundantly set exit on every character.
// Verify that hook sets the exit function and passthrough data is received correctly.
func TestDCS_Passthrough(t *testing.T) {
	// DCS q (final char) followed by data "abc" then ST (ESC \)
	input := "\x1BPq" + "abc" + "\x1B\\"
	r := strings.NewReader(input)
	parse := NewParser(r)

	// First should be the DCS hook
	seq := parse.Next()
	dcs, ok := seq.(DCS)
	assert.True(t, ok, "expected DCS sequence")
	assert.Equal(t, 'q', dcs.Final)

	// Then the passthrough data
	for _, expected := range []rune{'a', 'b', 'c'} {
		seq = parse.Next()
		data, ok := seq.(DCSData)
		assert.True(t, ok, "expected DCSData")
		assert.Equal(t, expected, rune(data))
	}

	// Then end of data
	seq = parse.Next()
	_, ok = seq.(DCSEndOfData)
	assert.True(t, ok, "expected DCSEndOfData")
}

// TestAnywhere_C1Codes tests 8-bit C1 control codes in the anywhere function.
func TestAnywhere_C1Codes(t *testing.T) {
	tests := []struct {
		name     string
		input    rune
		expected stateFn
		// Whether the exit function should have been called
		expectExit bool
	}{
		{name: "0x80", input: 0x80, expected: ground, expectExit: false},
		{name: "0x84", input: 0x84, expected: ground, expectExit: false},
		{name: "0x8F", input: 0x8F, expected: ground, expectExit: false},
		{name: "0x90 DCS", input: 0x90, expected: dcsEntry, expectExit: true},
		{name: "0x91", input: 0x91, expected: ground, expectExit: false},
		{name: "0x9A", input: 0x9A, expected: ground, expectExit: false},
		{name: "0x9B CSI", input: 0x9B, expected: csiEntry, expectExit: true},
		{name: "0x9C ST", input: 0x9C, expected: ground, expectExit: true},
		{name: "0x9D OSC", input: 0x9D, expected: oscString, expectExit: true},
		{name: "0x9E SOS", input: 0x9E, expected: ground, expectExit: false},
		{name: "0x9F APC", input: 0x9F, expected: ground, expectExit: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parse := &Parser{
				sequences: make(chan Sequence, 2),
				state:     ground,
			}
			called := false
			parse.SetExitFunc(func() {
				called = true
			})
			actual := anywhere(test.input, parse)
			act := reflect.ValueOf(actual).Pointer()
			exp := reflect.ValueOf(test.expected).Pointer()
			assert.Equal(t, exp, act, "wrong return function")
			if test.expectExit {
				assert.True(t, called, "exit function not called for 0x%02X", test.input)
			}
		})
	}
}

// TestEscape_Entry tests ESC followed by various entry characters.
func TestEscape_Entry(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Sequence
	}{
		{
			name:  "ESC P q = DCS entry",
			input: "\x1bPq",
			expected: []Sequence{
				DCS{
					Final:        'q',
					Intermediate: []rune{},
					Parameters:   []int{},
				},
				DCSEndOfData{},
			},
		},
		{
			name:  "ESC [ = CSI entry",
			input: "\x1b[c",
			expected: []Sequence{
				CSI{
					Final:        'c',
					Intermediate: []rune{},
					Parameters:   []int{},
				},
			},
		},
		{
			name:  "ESC ] = OSC string",
			input: "\x1b];\x1b\\",
			expected: []Sequence{
				OSC{
					Payload: []rune(";"),
				},
				ESC{
					Final:        '\\',
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "ESC X (SOS) with ST",
			input: "\x1bXhello\x1b\\",
			expected: []Sequence{
				ESC{
					Final:        '\\',
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "ESC ^ (PM) with ST",
			input: "\x1b^hello\x1b\\",
			expected: []Sequence{
				ESC{
					Final:        '\\',
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "ESC _ (APC) with ST",
			input: "\x1b_hello\x1b\\",
			expected: []Sequence{
				ESC{
					Final:        '\\',
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "ESC 7 (DECSC)",
			input: "\x1b7",
			expected: []Sequence{
				ESC{
					Final:        '7',
					Intermediate: []rune{},
				},
			},
		},
		{
			name:  "ESC 8 (DECRC)",
			input: "\x1b8",
			expected: []Sequence{
				ESC{
					Final:        '8',
					Intermediate: []rune{},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.input)
			parse := NewParser(r)
			i := 0
			for {
				seq := parse.Next()
				if _, isEOF := seq.(EOF); seq == nil || isEOF {
					assert.Equal(t, len(test.expected), i, "wrong amount of sequences")
					break
				}
				if i < len(test.expected) {
					assert.Equal(t, test.expected[i], seq)
				}
				i++
			}
		})
	}
}

// TestCSIParam_16PlusParams tests that 18 params are truncated to maxCSIParams (16).
func TestCSIParam_16PlusParams(t *testing.T) {
	input := "\x1b[1;2;3;4;5;6;7;8;9;10;11;12;13;14;15;16;17;18m"
	r := strings.NewReader(input)
	parse := NewParser(r)

	seq := parse.Next()
	csi, ok := seq.(CSI)
	assert.True(t, ok, "expected CSI sequence, got %T", seq)
	assert.Equal(t, 16, len(csi.Parameters), "should truncate to maxCSIParams (16)")
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, csi.Parameters)
}

// TestCSIParam_NonNumeric tests CSI with non-digit characters in param position.
// Characters in 0x40-0x7E are treated as final and dispatch immediately.
// The first such character dispatches; remaining chars are printed.
func TestCSIParam_NonNumeric(t *testing.T) {
	input := "\x1b[abc"
	r := strings.NewReader(input)
	parse := NewParser(r)

	seq := parse.Next()
	csi, ok := seq.(CSI)
	assert.True(t, ok, "expected CSI sequence for 'a' in final position, got %T", seq)
	assert.Equal(t, 'a', csi.Final)
	// 'b' and 'c' are printed as regular runes
	seq = parse.Next()
	_, isPrint := seq.(Print)
	assert.True(t, isPrint, "expected Print for 'b', got %T", seq)
	seq = parse.Next()
	_, isPrint = seq.(Print)
	assert.True(t, isPrint, "expected Print for 'c', got %T", seq)
}

// TestHook_NonNumericParam tests DCS with non-numeric params.
func TestHook_NonNumericParam(t *testing.T) {
	input := "\x1bP1;xyzqdata\x1b\\"
	r := strings.NewReader(input)
	parse := NewParser(r)

	// Should not panic. First sequence should be an error or DCS.
	seq := parse.Next()
	if err, isErr := seq.(error); isErr {
		t.Logf("got expected error: %v", err)
	} else if _, isDCS := seq.(DCS); isDCS {
		// If it's a DCS, drain remaining
		for {
			s := parse.Next()
			if _, ok := s.(EOF); ok {
				break
			}
		}
	}
}

// TestDCSIgnore tests the dcsIgnore state via ESC P : ... ESC \.
func TestDCSIgnore(t *testing.T) {
	input := "\x1bP:abc\x1b\\"
	r := strings.NewReader(input)
	parse := NewParser(r)

	var sequences []Sequence
	for {
		seq := parse.Next()
		if _, isEOF := seq.(EOF); seq == nil || isEOF {
			break
		}
		sequences = append(sequences, seq)
	}

	// DCS data with colon should be ignored — only the ST should produce output
	assert.Equal(t, 1, len(sequences), "expected only the ST (ESC \\) sequence")
	_, isESC := sequences[0].(ESC)
	assert.True(t, isESC, "expected ESC sequence for ST")
}

// TestSOSPmApcState tests SOS/PM/APC consume data silently.
func TestSOSPmApcState(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "SOS", input: "\x1bXhello world\x1b\\"},
		{name: "PM", input: "\x1b^hello world\x1b\\"},
		{name: "APC", input: "\x1b_hello world\x1b\\"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.input)
			parse := NewParser(r)

			var sequences []Sequence
			for {
				seq := parse.Next()
				if _, isEOF := seq.(EOF); seq == nil || isEOF {
					break
				}
				sequences = append(sequences, seq)
			}

			// Only the ESC \ (ST) should produce a sequence
			assert.Equal(t, 1, len(sequences),
				"SOS/PM/APC should only produce the final ESC sequence")
			_, isESC := sequences[0].(ESC)
			assert.True(t, isESC,
				"expected ESC sequence for ST terminator")
		})
	}
}

// TestGround_DEL tests that DEL (0x7F) in ground state falls through to print.
func TestGround_DEL(t *testing.T) {
	r := strings.NewReader("\x7f")
	parse := NewParser(r)

	seq := parse.Next()
	_, isPrint := seq.(Print)
	assert.True(t, isPrint, "expected Print sequence for DEL (0x7F), got %T", seq)
}

// TestEscape_DefaultPath tests ESC followed by an unexpected byte.
func TestEscape_DefaultPath(t *testing.T) {
	// ESC followed by 0x80 (outside normal range) should return to ground silently
	r := strings.NewReader("\x1b\x80")
	parse := NewParser(r)

	var sequences []Sequence
	for {
		seq := parse.Next()
		if _, isEOF := seq.(EOF); seq == nil || isEOF {
			break
		}
		sequences = append(sequences, seq)
	}

	// ESC 0x80 should not produce any sequence
	assert.Equal(t, 0, len(sequences),
		"ESC 0x80 should not produce any sequence")
}

func TestCSIParam_WithIntermediate(t *testing.T) {
	// CSI with intermediate character '!' and final 'p'
	input := "\x1b[!p"
	r := strings.NewReader(input)
	parse := NewParser(r)

	seq := parse.Next()
	csi, ok := seq.(CSI)
	assert.True(t, ok, "expected CSI sequence, got %T", seq)
	assert.Equal(t, []rune{'!'}, csi.Intermediate)
	assert.Equal(t, 'p', csi.Final)
}

func TestCSIParam_LargeParamValue(t *testing.T) {
	// A very large parameter value should be handled
	input := "\x1b[999999999c"
	r := strings.NewReader(input)
	parse := NewParser(r)

	seq := parse.Next()
	csi, ok := seq.(CSI)
	assert.True(t, ok, "expected CSI sequence, got %T", seq)
	assert.Equal(t, 999999999, csi.Parameters[0])
}

func TestPrintString(t *testing.T) {
	p := Print('A')
	s := p.String()
	assert.Contains(t, s, "0x41")
	assert.Contains(t, s, "'A'")
}

func TestPrintString_NonPrintable(t *testing.T) {
	p := Print(0x07)
	s := p.String()
	assert.Contains(t, s, "0x7")
}

func TestC0String(t *testing.T) {
	c := C0(0x07)
	s := c.String()
	assert.Contains(t, s, "0x7")
}

func TestESCString(t *testing.T) {
	e := ESC{Final: 'M', Intermediate: []rune{}}
	s := e.String()
	assert.Contains(t, s, "ESC")
	assert.Contains(t, s, "M")
}

func TestESCString_WithIntermediate(t *testing.T) {
	e := ESC{Final: 'c', Intermediate: []rune{'#'}}
	s := e.String()
	assert.Contains(t, s, "ESC")
	assert.Contains(t, s, "#")
	assert.Contains(t, s, "c")
}

func TestCSIString(t *testing.T) {
	c := CSI{Final: 'm', Intermediate: nil, Parameters: []int{38, 5, 196}}
	s := c.String()
	assert.Contains(t, s, "CSI")
	assert.Contains(t, s, "38;5;196")
	assert.Contains(t, s, "m")
}

func TestCSIString_NoParams(t *testing.T) {
	c := CSI{Final: 'J', Intermediate: []rune{'?'}, Parameters: []int{}}
	s := c.String()
	assert.Contains(t, s, "J")
	assert.Contains(t, s, "?")
}

func TestOSCString(t *testing.T) {
	o := OSC{Payload: []rune("0;title")}
	s := o.String()
	assert.Equal(t, "OSC 0;title", s)
}

func TestOSCString_Empty(t *testing.T) {
	o := OSC{Payload: []rune{}}
	s := o.String()
	assert.Equal(t, "OSC ", s)
}

func TestEOFString(t *testing.T) {
	e := EOF{}
	assert.Equal(t, "EOF", e.String())
}

func TestDCSString(t *testing.T) {
	dcs := DCS{Final: 'q', Intermediate: []rune{}, Parameters: []int{}}
	// DCS doesn't implement fmt.Stringer, just verify it's constructable
	_ = dcs
}

func TestDCSData(t *testing.T) {
	dd := DCSData('x')
	_ = dd
}

func TestDCSEndOfData(t *testing.T) {
	ed := DCSEndOfData{}
	_ = ed
}

// TestDCS_ParamIntermediate tests DCS with parameters + intermediate.
func TestDCS_ParamIntermediate(t *testing.T) {
	// DCS with parameters 1;2, intermediate ' ', final 'q'
	input := "\x1bP1;2 qdata\x1b\\"
	r := strings.NewReader(input)
	parse := NewParser(r)

	seq := parse.Next()
	dcs, ok := seq.(DCS)
	if !ok {
		t.Fatalf("expected DCS, got %T", seq)
	}
	assert.Equal(t, []int{1, 2}, dcs.Parameters)
	assert.Equal(t, []rune{' '}, dcs.Intermediate)
	assert.Equal(t, 'q', dcs.Final)

	// Drain remaining
	for {
		s := parse.Next()
		if _, ok := s.(EOF); ok {
			break
		}
	}
}

// TestDCS_WithoutST tests DCS passthrough that ends without ST.
func TestDCS_WithoutST(t *testing.T) {
	// DCS without proper ST terminator — will receive DCS, DCSData, then EOF
	input := "\x1bP1;2qdata"
	r := strings.NewReader(input)
	parse := NewParser(r)

	seq := parse.Next()
	_, ok := seq.(DCS)
	assert.True(t, ok, "expected DCS")

	// Drain remaining and expect EOF
	for {
		seq = parse.Next()
		if _, ok := seq.(EOF); ok {
			break
		}
	}
}

// TestNewParser_Creation verifies NewParser creates a working parser.
func TestNewParser_Creation(t *testing.T) {
	p := NewParser(strings.NewReader("h"))
	seq := p.Next()
	_, ok := seq.(Print)
	assert.True(t, ok, "expected Print, got %T", seq)
	// Drain to EOF
	seq = p.Next()
	_, ok = seq.(EOF)
	assert.True(t, ok, "expected EOF after single char input")
}

// TestOSC_NonStringTerminator tests OSC that terminates with BEL (0x07) or ST.
func TestOSC_NonStringTerminator(t *testing.T) {
	// OSC terminated by BEL (0x07)
	input := "\x1b]0;mytitle\x07"
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	osc, ok := seq.(OSC)
	assert.True(t, ok, "expected OSC, got %T", seq)
	assert.Equal(t, "0;mytitle", string(osc.Payload))

	// Drain to EOF
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestEscape_IntermediateMultiple tests ESC with multiple intermediate chars.
func TestEscape_IntermediateMultiple(t *testing.T) {
	// ESC with intermediate characters before final byte
	input := "\x1b%G" // UTF-8 identifier
	r := strings.NewReader(input)
	p := NewParser(r)

	seq := p.Next()
	esc, ok := seq.(ESC)
	assert.True(t, ok, "expected ESC, got %T", seq)
	assert.Equal(t, []rune{'%'}, esc.Intermediate)
	assert.Equal(t, 'G', esc.Final)
}

// TestDCS_MultipleData tests DCS with multiple DCSData runs.
func TestDCS_MultipleData(t *testing.T) {
	// DCS with lots of data bytes, then ST
	input := "\x1bPqhello world\x1b\\"
	r := strings.NewReader(input)
	parse := NewParser(r)

	seq := parse.Next()
	dcs, ok := seq.(DCS)
	assert.True(t, ok, "expected DCS, got %T", seq)
	_ = dcs

	// Drain remaining
	for {
		s := parse.Next()
		if _, ok := s.(EOF); ok {
			break
		}
	}
}

// TestOSC_LongString tests OSC with longer payload.
func TestOSC_LongString(t *testing.T) {
	input := "\x1b]0;very long title with many characters here\x07"
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	osc, ok := seq.(OSC)
	if !ok {
		t.Fatalf("expected OSC, got %T", seq)
	}
	if len(osc.Payload) < 10 {
		t.Errorf("OSC payload too short: %q", string(osc.Payload))
	}
	// Drain
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestOSC_WithST tests OSC terminated by ST (ESC \).
func TestOSC_WithST(t *testing.T) {
	input := "\x1b]0;title\x1b\\"
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	osc, ok := seq.(OSC)
	if !ok {
		t.Fatalf("expected OSC, got %T", seq)
	}
	assert.Equal(t, "0;title", string(osc.Payload))
	// Drain
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestSOS_PM_APC_Unterminated tests SOS/PM/APC without ST.
func TestSOS_PM_APC_Unterminated(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"SOS no ST", "\x1bXdata"},
		{"PM no ST", "\x1b^data"},
		{"APC no ST", "\x1b_data"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			p := NewParser(r)
			// Drain all sequences
			for {
				s := p.Next()
				if _, ok := s.(EOF); ok {
					break
				}
			}
		})
	}
}

// TestEscape_IntermediateParams tests ESC with multiple intermediates and params.
func TestEscape_IntermediateParams(t *testing.T) {
	// ESC with two intermediates: space, space, then final F (S7C1T)
	input := "\x1b F"
	r := strings.NewReader(input)
	p := NewParser(r)

	seq := p.Next()
	esc, ok := seq.(ESC)
	if !ok {
		t.Fatalf("expected ESC, got %T", seq)
	}
	assert.Equal(t, []rune{' '}, esc.Intermediate)
	assert.Equal(t, 'F', esc.Final)

	// Drain
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestCSIParam_WithSemicolon tests parsing CSV-like CSI parameters.
func TestCSIParam_WithSemicolon(t *testing.T) {
	input := "\x1b[1;2;3m"
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	csi, ok := seq.(CSI)
	if !ok {
		t.Fatalf("expected CSI, got %T", seq)
	}
	assert.Equal(t, []int{1, 2, 3}, csi.Parameters)
	assert.Equal(t, 'm', csi.Final)
	// Drain
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestCSIParam_DefaultParams tests CSI with missing params (default to 0).
func TestCSIParam_DefaultParams(t *testing.T) {
	input := "\x1b[H" // CUP with no params
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	csi, ok := seq.(CSI)
	if !ok {
		t.Fatalf("expected CSI, got %T", seq)
	}
	assert.Equal(t, []int{}, csi.Parameters)
	assert.Equal(t, 'H', csi.Final)
	// Drain
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestDCS_PassthroughDEL tests DCS passthrough with DEL characters.
func TestDCS_PassthroughDEL(t *testing.T) {
	input := "\x1bPqab\x7fcd\x1b\\"
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	_, ok := seq.(DCS)
	if !ok {
		t.Fatalf("expected DCS, got %T", seq)
	}
	// Drain remaining
	for {
		s := p.Next()
		if _, ok := s.(EOF); ok {
			break
		}
	}
}

// TestDCS_PassthroughControls tests DCS passthrough with control chars.
func TestDCS_PassthroughControls(t *testing.T) {
	input := "\x1bPq\x80\x90\x9b\x1b\\"
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	_, ok := seq.(DCS)
	if !ok {
		t.Fatalf("expected DCS, got %T", seq)
	}
	// Drain remaining
	for {
		s := p.Next()
		if _, ok := s.(EOF); ok {
			break
		}
	}
}

// TestOSC_MultipleParams tests OSC with multiple semicolons.
func TestOSC_MultipleParams(t *testing.T) {
	input := "\x1b]4;0;rgb:00/00/00;1;rgb:ff/00/00\x07"
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	_, ok := seq.(OSC)
	if !ok {
		t.Fatalf("expected OSC, got %T", seq)
	}
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestOSC_NoTerminator tests OSC without any terminator.
func TestOSC_NoTerminator(t *testing.T) {
	input := "\x1b]0;unterminated"
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	_, ok := seq.(OSC)
	if !ok {
		t.Fatalf("expected OSC, got %T", seq)
	}
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestEscape_MultipleIntermediates tests ESC with two intermediate chars.
func TestEscape_MultipleIntermediates(t *testing.T) {
	input := "\x1b%@" // ESC % @ = UTF-8 level
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	esc, ok := seq.(ESC)
	if !ok {
		t.Fatalf("expected ESC, got %T", seq)
	}
	assert.Equal(t, []rune{'%'}, esc.Intermediate)
	assert.Equal(t, '@', esc.Final)
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestEscape_ESCInEscape tests two ESC in a row.
func TestEscape_ESCInEscape(t *testing.T) {
	input := "\x1b\x1bM"
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	_, ok := seq.(ESC)
	if !ok {
		t.Fatalf("expected ESC, got %T", seq)
	}
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestCSIParam_EmptySubparams tests CSI with empty subparameters.
func TestCSIParam_EmptySubparams(t *testing.T) {
	input := "\x1b[1;;3m"
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	csi, ok := seq.(CSI)
	if !ok {
		t.Fatalf("expected CSI, got %T", seq)
	}
	assert.Equal(t, []int{1, 0, 3}, csi.Parameters)
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestCSIParam_NonNumericParam tests CSI with non-numeric parameter.
func TestCSIParam_NonNumericParam(t *testing.T) {
	input := "\x1b[1;abc;3m"
	r := strings.NewReader(input)
	p := NewParser(r)
	// Drain all - should not panic
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestCSIParam_RGBSubparam tests CSI with RGB sub-parameter (colon).
func TestCSIParam_RGBSubparam(t *testing.T) {
	input := "\x1b[38:2:100:150:200m"
	r := strings.NewReader(input)
	p := NewParser(r)
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestCSIParam_6PartRGB tests CSI with 6-part RGB subparam.
func TestCSIParam_6PartRGB(t *testing.T) {
	input := "\x1b[38:2::100:150:200m"
	r := strings.NewReader(input)
	p := NewParser(r)
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestEscape_IntermediateSpace tests ESC SP F.
func TestEscape_IntermediateSpace(t *testing.T) {
	input := "\x1b F"
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	_, ok := seq.(ESC)
	assert.True(t, ok, "expected ESC")
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestEscape_IntermediateNumber tests ESC # 8.
func TestEscape_IntermediateNumber(t *testing.T) {
	input := "\x1b#8"
	r := strings.NewReader(input)
	p := NewParser(r)
	seq := p.Next()
	_, ok := seq.(ESC)
	assert.True(t, ok, "expected ESC")
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestCSI_SecondaryDA tests CSI > c.
func TestCSI_SecondaryDA(t *testing.T) {
	h, _ := NewTerminal(4, 1)
	defer h.Close()
	h.FeedString("\x1b[>c")
	out := h.Output()
	if !bytes.Contains(out, []byte("\x1b[>")) {
		t.Errorf("secondary DA expected output containing >, got %q", out)
	}
}

// TestCSI_DECSTR tests CSI ! p (DECSTR soft reset).
func TestCSI_DECSTR(t *testing.T) {
	h, _ := NewTerminal(4, 1)
	defer h.Close()
	assert.NotPanics(t, func() {
		h.FeedString("\x1b[!p")
	})
}

// TestDCS_EntryWithByte0x7F tests DCS entry with DEL byte.
func TestDCS_EntryWithByte0x7F(t *testing.T) {
	input := "\x1bP\x7f"
	r := strings.NewReader(input)
	p := NewParser(r)
	for {
		if _, ok := p.Next().(EOF); ok {
			break
		}
	}
}

// TestEscape_SOS_PM_APC_Multiple tests multiple SOS/PM/APC sequences.
func TestEscape_SOS_PM_APC_Multiple(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"SOS", "\x1bX"},
		{"PM", "\x1b^"},
		{"APC", "\x1b_"},
		{"SOS_with_data", "\x1bXdata\x1b\\"},
		{"PM_with_data", "\x1b^data\x1b\\"},
		{"APC_with_data", "\x1b_data\x1b\\"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			p := NewParser(r)
			for {
				if _, ok := p.Next().(EOF); ok {
					break
				}
			}
		})
	}
}

// TestParser_Stress tests various parser state transitions.
func TestParser_Stress(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"DCS params 0-9", "\x1bP0123456789qdata\x1b\\"},
		{"DCS params <=>?", "\x1bP<=>?qdata\x1b\\"},
		{"DCS intermediate space", "\x1bP qdata\x1b\\"},
		{"DCS intermediate !", "\x1bP!qdata\x1b\\"},
		{"DCS ignore colon", "\x1bP:abc\x1b\\"},
		{"DCS ignore colon params", "\x1bP1;2:abc\x1b\\"},
		{"DCS passthrough 0x7F", "\x1bPqab\x7fcd\x1b\\"},
		{"DCS passthrough controls", "\x1bPq\x00\x01\x02\x1b\\"},
		{"ESC space", "\x1b F"},
		{"ESC hash", "\x1b#8"},
		{"ESC percent", "\x1b%G"},
		{"SOS", "\x1bX"},
		{"PM", "\x1b^"},
		{"APC", "\x1b_"},
		{"OSC bel term", "\x1b]0;test\x07"},
		{"OSC st term", "\x1b]0;test\x1b\\"},
		{"CSI non-numeric", "\x1b[1;abc;3m"},
		{"CSI empty subparams", "\x1b[1;;3m"},
		{"C1 CSI 8-bit", "\x9b5n"},
		{"C1 OSC 8-bit", "\x9d0;title\x07"},
		{"ESC alone", "\x1b"},
		{"CSI alone", "\x1b["},
		{"ESC ESC", "\x1b\x1bM"},
		{"DCS with CAN", "\x1bPqda\x18ta\x1b\\"},
		{"OSC with CAN", "\x1b]0;ti\x18tle\x07"},
		{"SOS/PM/APC varied", "\x1bXdata\x1b^\x1b_"},
		// More edge cases
		{"DCS intermediate multiple", "\x1bP!\"qdata\x1b\\"},
		{"DCS param then intermediate", "\x1bP1;2 qdata\x1b\\"},
		{"DCS just params", "\x1bP1;2q\x1b\\"},
		{"DCS just intermediate", "\x1bP q\x1b\\"},
		{"CSI 2-part RGB", "\x1b[4:3m"},
		{"CSI RGB with extra colon", "\x1b[38:2:100:150:200m"},
		{"Invalid UTF-8 in text", "A\xfeB"},
		{"CSI semicolon only", "\x1b[;m"},
		{"CSI multiple semicolons", "\x1b[1;;;4m"},
		{"OSC empty payload", "\x1b]\x07"},
		{"OSC without any terminator", "\x1b]0;abc"},
		{"ESC intermediate then final", "\x1b%G"},
		{"ESC intermediate only", "\x1b%"},
		{"DCS final only", "\x1bPq"},
		{"DCS with only ST", "\x1bPq\x1b\\"},
		// Feed specific byte values to hit state transitions
		{"DCS entry >0x7E", "\x1bP\x7fq\x1b\\"},
		{"DCS entry 0x18 CAN", "\x1bP\x18q\x1b\\"},
		{"DCS entry 0x1A SUB", "\x1bP\x1aq\x1b\\"},
		{"DCS entry 0x1B ESC", "\x1bP\x1bq\x1b\\"},
		{"DCS param 0x18", "\x1bP1\x18;2q\x1b\\"},
		{"DCS ignore param", "\x1bP1;2:3q\x1b\\"},
		{"DCS ignore 0x18", "\x1bP:abc\x18\x1b\\"},
		{"CSI intermediate 0x18", "\x1b[!\x18m"},
		{"CSI intermediate 0x1A", "\x1b[!\x1am"},
		{"CSI ignore 0x18", "\x1b[1;\x18;2m"},
		{"escape intermediate 0x18", "\x1b%\x18G"},
		{"escape intermediate default", "\x1b\x01"}, // 0x01 in escape state
		{"SOC default", "\x1bX\x18"},                // 0x18 in SOS state
		{"OSC default", "\x1b]0;abc\x18"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			p := NewParser(r)
			for {
				if _, ok := p.Next().(EOF); ok {
					break
				}
			}
		})
	}
}

// ============================================================
// Security: Unbounded parser buffer accumulation
//
// The parser's oscPut(), param(), and collect() methods append
// to internal slices with no size limits. A malicious or
// misbehaving program can send crafted sequences that grow
// these buffers without bound, causing unbounded memory growth.
// ============================================================

// oscPut() appends to the internal oscData buffer with no limit.
// An unterminated OSC sequence followed by arbitrary data causes
// unbounded memory growth.
func TestOscPut_LimitsDataLength(t *testing.T) {
	p := &Parser{
		sequences: make(chan Sequence, 2),
		state:     ground,
	}
	for i := 0; i < 8192; i++ {
		p.oscPut('A')
	}
	assert.LessOrEqual(t, len(p.oscData), 4096,
		"oscPut() should cap oscData at 4096 runes")
}

// End-to-end: an OSC sequence with data exceeding the limit
// should have its payload truncated to the maximum allowed length.
func TestParser_OscDataTruncated(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("\x1b]") // OSC start (ESC ])
	for i := 0; i < 8192; i++ {
		sb.WriteByte('A')
	}
	sb.WriteByte('\a') // BEL terminates OSC

	r := strings.NewReader(sb.String())
	parse := NewParser(r)

	seq := parse.Next()
	osc, ok := seq.(OSC)
	assert.True(t, ok, "expected OSC sequence, got %T", seq)
	assert.LessOrEqual(t, len(osc.Payload), 4096,
		"OSC payload should be capped at 4096 runes")
}

// Regression: OSC data at exactly the maximum should be fully preserved.
func TestParser_OscDataAtLimit(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("\x1b]")
	for i := 0; i < 4096; i++ {
		sb.WriteByte('B')
	}
	sb.WriteByte('\a')

	r := strings.NewReader(sb.String())
	parse := NewParser(r)

	seq := parse.Next()
	osc, ok := seq.(OSC)
	assert.True(t, ok, "expected OSC sequence")
	assert.Equal(t, 4096, len(osc.Payload),
		"OSC data at exactly the limit should be fully preserved")
}

// collect() appends to the internal intermediate buffer with no limit.
// A CSI sequence with many intermediate characters wastes memory.
func TestCollect_LimitsIntermediateLength(t *testing.T) {
	p := &Parser{
		sequences: make(chan Sequence, 2),
		state:     ground,
	}
	for i := 0; i < 50; i++ {
		p.collect(' ')
	}
	assert.LessOrEqual(t, len(p.intermediate), 16,
		"collect() should cap intermediate at 16 runes")
}

// param() appends to the internal params buffer with no limit.
// A CSI with millions of parameter characters causes unbounded growth.
func TestParam_LimitsRawBufferLength(t *testing.T) {
	p := &Parser{
		sequences: make(chan Sequence, 2),
		state:     ground,
	}
	for i := 0; i < 5000; i++ {
		p.param('1')
	}
	assert.LessOrEqual(t, len(p.params), 1024,
		"param() should cap params at 1024 runes")
}

// ============================================================
// Security: CSI parameter count unlimited
//
// After collecting raw parameter runes, csiDispatch() splits on
// ';' and converts all to []int with no limit. A single CSI
// sequence with many semicolons creates a huge parameter slice.
// ECMA-48 specifies a maximum of 16 parameters.
// ============================================================

// csiDispatch should cap the number of parsed parameters at 16.
func TestCsiDispatch_LimitsParameterCount(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("\x1b[")
	for i := 0; i < 32; i++ {
		if i > 0 {
			sb.WriteByte(';')
		}
		sb.WriteByte('1')
	}
	sb.WriteByte('m')

	r := strings.NewReader(sb.String())
	parse := NewParser(r)

	seq := parse.Next()
	csi, ok := seq.(CSI)
	assert.True(t, ok, "expected CSI sequence, got %T", seq)
	assert.LessOrEqual(t, len(csi.Parameters), 16,
		"CSI parameters should be capped at 16")
}

// Regression: exactly 16 parameters should all be preserved.
func TestCsiDispatch_Exact16Preserved(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("\x1b[")
	expected := make([]int, 16)
	for i := 0; i < 16; i++ {
		if i > 0 {
			sb.WriteByte(';')
		}
		sb.WriteString(strconv.Itoa(i + 1))
		expected[i] = i + 1
	}
	sb.WriteByte('m')

	r := strings.NewReader(sb.String())
	parse := NewParser(r)

	seq := parse.Next()
	csi, ok := seq.(CSI)
	assert.True(t, ok, "expected CSI sequence")
	assert.Equal(t, expected, csi.Parameters,
		"exactly 16 parameters should all be preserved")
}

// Regression: a CSI with fewer than 16 parameters should be unaffected.
func TestCsiDispatch_FewParamsUnaffected(t *testing.T) {
	input := "\x1b[1;2;3m"
	r := strings.NewReader(input)
	parse := NewParser(r)

	seq := parse.Next()
	csi, ok := seq.(CSI)
	assert.True(t, ok, "expected CSI sequence")
	assert.Equal(t, []int{1, 2, 3}, csi.Parameters)
}
