package text

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// wideInterval represents an inclusive Unicode code point range [lo, hi]
// whose runes occupy 2 terminal display cells.
type wideInterval struct {
	lo rune
	hi rune
}

// wideRanges contains sorted, non-overlapping inclusive intervals for East Asian
// Wide/Fullwidth characters (UAX #11) and Unicode Emoji presentation blocks.
var wideRanges = []wideInterval{
	{0x1100, 0x115F},   // Hangul Jamo Choseong
	{0x231A, 0x231B},   // Watch, Hourglass
	{0x2329, 0x232A},   // Left/Right-Pointing Angle Bracket
	{0x23E9, 0x23FA},   // Fast-forward, Pause (⏸), Record, etc.
	{0x25FD, 0x25FE},   // White/Black Medium Small Square
	{0x2614, 0x2615},   // Umbrella with Rain Drops, Hot Beverage
	{0x2648, 0x2653},   // Zodiac signs
	{0x267F, 0x267F},   // Wheelchair Symbol
	{0x2693, 0x2693},   // Anchor
	{0x26A0, 0x26A1},   // Warning Sign (⚠), High Voltage (⚡)
	{0x26AA, 0x26AB},   // Medium White/Black Circle
	{0x26BD, 0x26BE},   // Soccer Ball, Baseball
	{0x26C4, 0x26C5},   // Snowman, Sun Behind Cloud
	{0x26CE, 0x26CE},   // Ophiuchus
	{0x26D4, 0x26D4},   // No Entry
	{0x26EA, 0x26EA},   // Church
	{0x26F2, 0x26F3},   // Fountain, Flag in Hole
	{0x26F5, 0x26F5},   // Sailboat
	{0x26FA, 0x26FA},   // Tent
	{0x26FD, 0x26FD},   // Fuel Pump
	{0x2705, 0x2705},   // Check Mark Button (✅)
	{0x270A, 0x270B},   // Raised Fist, Raised Hand
	{0x2728, 0x2728},   // Sparkles (✨)
	{0x274C, 0x274C},   // Cross Mark (❌)
	{0x274E, 0x274E},   // Cross Mark Button
	{0x2753, 0x2755},   // Question/Exclamation Marks
	{0x2757, 0x2757},   // Exclamation Mark
	{0x2795, 0x2797},   // Plus, Minus, Divide
	{0x27B0, 0x27B0},   // Curly Loop
	{0x27BF, 0x27BF},   // Double Curly Loop
	{0x2B1B, 0x2B1C},   // Black/White Large Square
	{0x2B50, 0x2B50},   // Star
	{0x2B55, 0x2B55},   // Hollow Red Circle
	{0x2E80, 0x303E},   // CJK Radicals, Kangxi, Symbols and Punctuation
	{0x3040, 0x33FF},   // Hiragana, Katakana, Bopomofo, Hangul Compat, Enclosed CJK
	{0x3400, 0x4DBF},   // CJK Unified Ideographs Extension A
	{0x4E00, 0xA4CF},   // CJK Unified Ideographs, Yi Syllables & Radicals
	{0xA960, 0xA97F},   // Hangul Jamo Extended-A
	{0xAC00, 0xD7A3},   // Hangul Syllables
	{0xF900, 0xFAFF},   // CJK Compatibility Ideographs
	{0xFE10, 0xFE19},   // Vertical Forms
	{0xFE30, 0xFE6F},   // CJK Compatibility Forms & Small Form Variants
	{0xFF01, 0xFF60},   // Fullwidth ASCII Variants
	{0xFFE0, 0xFFE6},   // Fullwidth Symbol Variants
	{0x1F000, 0x1FAFF}, // Mahjong, Domino, Playing Cards, Emoji & Pictographs
	{0x20000, 0x2FFFD}, // CJK Unified Ideographs Extensions B-F / SIP
	{0x30000, 0x3FFFD}, // CJK Unified Ideographs Extensions G+ / TIP
}

// AnsiSeqLen returns the byte length of an ANSI/VT escape sequence starting at
// s[i], where s[i] == '\x1b'. If s[i:] is a bare ESC at end of string, it
// returns 1.
func AnsiSeqLen(s string, i int) int {
	n := len(s)
	if i+1 >= n {
		return 1
	}
	next := s[i+1]
	switch next {
	case '[':
		// CSI sequence: ESC [ <params/intermediates 0x20..0x3F>* <final 0x40..0x7E>
		j := i + 2
		for j < n {
			b := s[j]
			if b >= 0x40 && b <= 0x7E {
				return j + 1 - i
			}
			if b < 0x20 || b > 0x3F {
				return j - i
			}
			j++
		}
		return n - i
	case ']':
		// OSC sequence: ESC ] ... (BEL '\x07' or ST "\x1b\\")
		j := i + 2
		for j < n {
			if s[j] == '\x07' {
				return j + 1 - i
			}
			if s[j] == '\n' || s[j] == '\r' {
				return j - i
			}
			if s[j] == '\x1b' && j+1 < n && s[j+1] == '\\' {
				return j + 2 - i
			}
			j++
		}
		return n - i
	default:
		// 3-byte ECMA-35 announcement: ESC SP <0x40..0x5F>
		if next == 0x20 {
			if i+2 < n && s[i+2] >= 0x40 && s[i+2] <= 0x5F {
				return 3
			}
			return 1
		}
		// ISO 2022 intermediate bytes (0x21..0x2F, up to 2 bytes) followed by final byte (0x30..0x7E)
		if next >= 0x21 && next <= 0x2F {
			j := i + 2
			limit := i + 4
			if limit > n {
				limit = n
			}
			for j < limit && s[j] >= 0x21 && s[j] <= 0x2F {
				j++
			}
			if j < n && s[j] >= 0x30 && s[j] <= 0x7E {
				return j + 1 - i
			}
			return 1
		}
		// 2-byte escape sequence (ESC + 0x40..0x7E)
		if next >= 0x40 && next <= 0x7E {
			return 2
		}
		return 1
	}
}

// StripANSI removes ANSI escape sequences (CSI SGR colors/styles, OSC hyperlinks,
// and standard 2-byte ESC sequences) from s while preserving all other text.
func StripANSI(s string) string {
	idx := strings.IndexByte(s, '\x1b')
	if idx == -1 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:idx])
	for i := idx; i < len(s); {
		if s[i] == '\x1b' {
			i += AnsiSeqLen(s, i)
			continue
		}
		nextEsc := strings.IndexByte(s[i:], '\x1b')
		if nextEsc == -1 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(s[i : i+nextEsc])
		i += nextEsc
	}
	return b.String()
}

// inRanges reports whether r falls inside sorted, non-overlapping wideRanges.
func inRanges(r rune, ranges []wideInterval) bool {
	lo, hi := 0, len(ranges)-1
	for lo <= hi {
		mid := lo + (hi-lo)/2
		iv := ranges[mid]
		if r < iv.lo {
			hi = mid - 1
		} else if r > iv.hi {
			lo = mid + 1
		} else {
			return true
		}
	}
	return false
}

// IsSkinToneModifier reports whether r is an Emoji Fitzpatrick skin-tone modifier.
func IsSkinToneModifier(r rune) bool {
	return r >= 0x1F3FB && r <= 0x1F3FF
}

// IsEmojiRune reports whether r is an emoji base character or symbol commonly
// joined in ZWJ sequences.
func IsEmojiRune(r rune) bool {
	if r >= 0x1F000 && r <= 0x1FAFF {
		return true
	}
	if (r >= 0x2600 && r <= 0x27BF) || (r >= 0x23E9 && r <= 0x23FA) || r == 0x2B50 || r == 0x2B55 {
		return true
	}
	return false
}

// RuneWidth returns the terminal cell width (0, 1, or 2) of a single Unicode rune.
func RuneWidth(r rune) int {
	if r == 0 || r < 0x20 || (r >= 0x7F && r < 0xA0) || r == 0x00AD {
		return 0
	}
	if r >= 0x20 && r < 0x7F {
		return 1
	}
	switch r {
	case '\u200B', '\u200C', '\u200D', '\u2060', '\uFEFF':
		return 0
	}
	if (r >= 0xFE00 && r <= 0xFE0F) || (r >= 0xE0100 && r <= 0xE01EF) {
		return 0
	}
	if (r >= 0x1160 && r <= 0x11FF) || (r >= 0xD7B0 && r <= 0xD7FF) {
		return 0
	}
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
		return 0
	}
	if inRanges(r, wideRanges) {
		return 2
	}
	return 1
}

// DisplayWidth calculates the deterministic terminal cell width of s.
// ANSI escape sequences are ignored without heap allocation, East Asian Wide
// and Emoji runes count as 2 cells, and ZWJ/modifier/combining sequences are
// handled cleanly.
func DisplayWidth(s string) int {
	// Fast ASCII prefix scan: printable 7-bit ASCII (0x20..0x7E) always has
	// width 1 per byte and cannot be an ANSI ESC (0x1B) or multibyte rune.
	i := 0
	for i < len(s) {
		b := s[i]
		if b < 0x20 || b >= 0x7F {
			break
		}
		i++
	}
	if i == len(s) {
		return len(s)
	}

	width := i
	prevRuneWidth := 0
	if i > 0 {
		prevRuneWidth = 1
	}
	prevWasEmoji := false
	afterZWJ := false

	for i < len(s) {
		if s[i] == '\x1b' {
			i += AnsiSeqLen(s, i)
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size

		if r == '\u200D' {
			afterZWJ = prevWasEmoji
			prevRuneWidth = 0
			continue
		}

		if r == '\uFE0F' {
			if prevRuneWidth == 1 {
				width++
				prevRuneWidth = 2
				prevWasEmoji = true
			}
			continue
		}

		if IsSkinToneModifier(r) && prevWasEmoji {
			// Skin tone modifier combines with preceding emoji without adding cells.
			continue
		}

		if afterZWJ && IsEmojiRune(r) {
			afterZWJ = false
			prevWasEmoji = true
			prevRuneWidth = 2
			continue
		}

		afterZWJ = false
		rw := RuneWidth(r)
		width += rw

		if rw > 0 {
			prevRuneWidth = rw
			prevWasEmoji = IsEmojiRune(r)
		} else {
			// Preserve prevWasEmoji across zero-width combining marks/variation selectors
			// so skin-tone or ZWJ sequences remain intact.
			if r != '\uFE0E' && !unicode.Is(unicode.Mn, r) {
				prevRuneWidth = 0
				prevWasEmoji = false
			}
		}
	}

	return width
}

