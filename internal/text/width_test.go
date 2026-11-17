package text

import (
	"reflect"
	"testing"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "plain_ascii", in: "hello world", want: "hello world"},
		{name: "sgr_color", in: "\x1b[32mOK\x1b[0m", want: "OK"},
		{name: "sgr_truecolor_mixed", in: "\x1b[1;38;2;255;128;0mWARN\x1b[0m: high load", want: "WARN: high load"},
		{name: "osc_bel", in: "\x1b]8;;https://example.com\x07Click\x1b]8;;\x07", want: "Click"},
		{name: "osc_st", in: "\x1b]0;Window Title\x1b\\Prompt", want: "Prompt"},
		{name: "iso2022_and_2byte", in: "\x1b(Bhello\x1bMworld", want: "helloworld"},
		{name: "trailing_bare_esc", in: "abc\x1b", want: "abc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := StripANSI(tc.in)
			if got != tc.want {
				t.Fatalf("StripANSI(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRuneWidth(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want int
	}{
		{name: "null", r: 0, want: 0},
		{name: "control_lf", r: '\n', want: 0},
		{name: "control_del", r: 0x7F, want: 0},
		{name: "soft_hyphen", r: 0x00AD, want: 0},
		{name: "zwj", r: '\u200D', want: 0},
		{name: "zws", r: '\u200B', want: 0},
		{name: "vs16", r: '\uFE0F', want: 0},
		{name: "combining_acute", r: '\u0301', want: 0},
		{name: "ascii_a", r: 'a', want: 1},
		{name: "ascii_space", r: ' ', want: 1},
		{name: "hiragana", r: 'こ', want: 2},
		{name: "katakana", r: 'カ', want: 2},
		{name: "kanji", r: '漢', want: 2},
		{name: "hangul", r: '한', want: 2},
		{name: "fullwidth_A", r: 'Ａ', want: 2},
		{name: "rocket", r: '🚀', want: 2},
		{name: "warning", r: '⚠', want: 2},
		{name: "check", r: '✅', want: 2},
		{name: "pause", r: '⏸', want: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := RuneWidth(tc.r); got != tc.want {
				t.Fatalf("RuneWidth(%U) = %d, want %d", tc.r, got, tc.want)
			}
		})
	}
}

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "empty", in: "", want: 0},
		{name: "ascii", in: "Payment Gateway", want: 15},
		{name: "cjk_hiragana", in: "こんにちは", want: 10},
		{name: "cjk_mixed", in: "Hello 世界!", want: 11},
		{name: "cjk_hangul", in: "안녕하세요", want: 10},
		{name: "fullwidth_ascii", in: "ＡＢＣ", want: 6},
		{name: "emoji_rocket", in: "🚀", want: 2},
		{name: "emoji_warning_vs16", in: "⚠️", want: 2},
		{name: "emoji_check", in: "✅", want: 2},
		{name: "emoji_pause_vs16", in: "⏸️", want: 2},
		{name: "emoji_heart_vs16", in: "❤️", want: 2},
		{name: "emoji_skin_tone", in: "👍🏽", want: 2},
		{name: "emoji_zwj_technologist", in: "👨‍💻", want: 2},
		{name: "emoji_zwj_family", in: "👨‍👩‍👧", want: 2},
		{name: "emoji_zwj_flag", in: "🏳️‍🌈", want: 2},
		{name: "emoji_zwj_skin_tone", in: "👩🏽‍💻", want: 2},
		{name: "emoji_plus_text", in: "✅ Healthy", want: 10},
		{name: "warning_plus_text", in: "⚠️ High Load", want: 12},
		{name: "ansi_wrapped_ascii", in: "\x1b[32mOK\x1b[0m", want: 2},
		{name: "ansi_wrapped_cjk_emoji", in: "\x1b[1;36m🚀 こんにちは\x1b[0m", want: 13},
		{name: "combining_diacritic", in: "cafe\u0301", want: 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DisplayWidth(tc.in)
			if got != tc.want {
				t.Fatalf("DisplayWidth(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		maxWidth int
		want     []string
	}{
		{
			name:     "no_wrap_fast_path",
			in:       "short",
			maxWidth: 10,
			want:     []string{"short"},
		},
		{
			name:     "crlf_unbounded",
			in:       "line1\r\nline2",
			maxWidth: 0,
			want:     []string{"line1", "line2"},
		},
		{
			name:     "greedy_word_wrap",
			in:       "Menangani transaksi QRIS dan Virtual Account dengan auto-retry.",
			maxWidth: 30,
			want:     []string{"Menangani transaksi QRIS dan", "Virtual Account dengan", "auto-retry."},
		},
		{
			name:     "single_word_cjk_overflow",
			in:       "こんにちは世界",
			maxWidth: 6,
			want:     []string{"こんに", "ちは世", "界"},
		},
		{
			name:     "tiny_width_wide_rune",
			in:       "漢字",
			maxWidth: 1,
			want:     []string{"漢", "字"},
		},
		{
			name:     "ansi_sgr_across_lines",
			in:       "\x1b[32mLine 1\nLine 2\x1b[0m",
			maxWidth: 0,
			want:     []string{"\x1b[32mLine 1\x1b[0m", "\x1b[32mLine 2\x1b[0m"},
		},
		{
			name:     "ansi_sgr_word_split",
			in:       "\x1b[31mHelloWorld\x1b[0m",
			maxWidth: 6,
			want:     []string{"\x1b[31mHelloW\x1b[0m", "\x1b[31morld\x1b[0m"},
		},
		{
			name:     "zwj_emoji_stays_intact",
			in:       "👨‍💻ab",
			maxWidth: 2,
			want:     []string{"👨‍💻", "ab"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := WrapText(tc.in, tc.maxWidth)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("WrapText(%q, %d) = %#v, want %#v", tc.in, tc.maxWidth, got, tc.want)
			}
		})
	}
}

func TestAlignText(t *testing.T) {
	if got := AlignText("hi", 5, AlignLeft); got != "hi   " {
		t.Fatalf("AlignLeft = %q", got)
	}
	if got := AlignText("hi", 5, AlignCenter); got != " hi  " {
		t.Fatalf("AlignCenter = %q", got)
	}
	if got := AlignText("hi", 5, AlignRight); got != "   hi" {
		t.Fatalf("AlignRight = %q", got)
	}
	if got := AlignText("toolong", 3, AlignCenter); got != "toolong" {
		t.Fatalf("AlignText overflow = %q", got)
	}
}

func TestMalformedANSIAndZeroWidthWrap(t *testing.T) {
	// Bare ESC before multibyte UTF-8 rune must only strip ESC, not the first UTF-8 byte.
	if got := StripANSI("\x1b漢"); got != "漢" {
		t.Fatalf("StripANSI(\"\\x1b漢\") = %q, want \"漢\"", got)
	}
	if got := DisplayWidth("\x1b漢"); got != 2 {
		t.Fatalf("DisplayWidth(\"\\x1b漢\") = %d, want 2", got)
	}

	// Malformed ESC + spaces must not swallow subsequent text.
	if got := StripANSI("\x1b   text"); got != "   text" {
		t.Fatalf("StripANSI(\"\\x1b   text\") = %q, want \"   text\"", got)
	}

	// Unterminated OSC stops at newline.
	if got := StripANSI("\x1b]unterminated\nnext"); got != "\nnext" {
		t.Fatalf("StripANSI unterminated OSC = %q, want \"\\nnext\"", got)
	}

	// Standalone trailing ANSI reset must not inject an extra space or line break.
	wrapped := WrapText("\x1b[32mhello world \x1b[0m", 11)
	if len(wrapped) != 1 || wrapped[0] != "\x1b[32mhello world\x1b[0m" {
		t.Fatalf("WrapText trailing reset = %#v", wrapped)
	}

	// Leading standalone ANSI SGR before a word must not duplicate SGR.
	wrapped2 := WrapText("\x1b[32m hello", 5)
	if len(wrapped2) != 1 || wrapped2[0] != "\x1b[32mhello\x1b[0m" {
		t.Fatalf("WrapText leading SGR = %#v", wrapped2)
	}

	// MeasureSingleLine fast paths and unclosed SGR closing.
	if s, w, ok := MeasureSingleLine("plain", 10); !ok || s != "plain" || w != 5 {
		t.Fatalf("MeasureSingleLine plain = (%q, %d, %v)", s, w, ok)
	}
	if _, _, ok := MeasureSingleLine("too long", 4); ok {
		t.Fatal("expected MeasureSingleLine overflow to return ok=false")
	}
	if _, _, ok := MeasureSingleLine("a\nb", 10); ok {
		t.Fatal("expected MeasureSingleLine multiline to return ok=false")
	}
	if s, w, ok := MeasureSingleLine("\x1b[32mOK", 10); !ok || s != "\x1b[32mOK\x1b[0m" || w != 2 {
		t.Fatalf("MeasureSingleLine unclosed SGR = (%q, %d, %v)", s, w, ok)
	}
}

func FuzzDisplayWidthAndWrap(f *testing.F) {
	seeds := []struct {
		s     string
		limit int
	}{
		{"", 0},
		{"hello world", 5},
		{"\x1b[32m✅ Healthy\x1b[0m", 6},
		{"こんにちは世界", 3},
		{"👨‍💻👨‍👩‍👧🏳️‍🌈👍🏽", 2},
		{"\x1b漢\n\x1b]osc\nnext", 4},
	}
	for _, seed := range seeds {
		f.Add(seed.s, seed.limit)
	}

	f.Fuzz(func(t *testing.T, s string, limit int) {
		if limit < 0 {
			limit = 0
		}
		if limit > 120 {
			limit = 120
		}
		w := DisplayWidth(s)
		if w < 0 {
			t.Fatalf("negative DisplayWidth(%q) = %d", s, w)
		}
		stripped := StripANSI(s)
		if StripANSI(stripped) != stripped {
			t.Fatalf("StripANSI not idempotent on %q", s)
		}
		lines := WrapText(s, limit)
		for _, ln := range lines {
			if DisplayWidth(ln) < 0 {
				t.Fatalf("negative line width for %q", ln)
			}
		}
	})
}

func BenchmarkDisplayWidth(b *testing.B) {
	sample := "\x1b[32m✅ Healthy\x1b[0m - こんにちは 🚀"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if w := DisplayWidth(sample); w == 0 {
			b.Fatal("unexpected zero width")
		}
	}
}

