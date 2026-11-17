package text

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Horizontal text alignment constants used by WriteAligned and AlignText.
const (
	AlignLeft   = 0
	AlignCenter = 1
	AlignRight  = 2
)

// updateSGR tracks the active ANSI SGR color/style sequence as escape codes
// are encountered so wrapped lines can close and reopen styles without bleeding
// into table borders.
func updateSGR(active string, seq string) string {
	if !strings.HasPrefix(seq, "\x1b[") || !strings.HasSuffix(seq, "m") {
		return active
	}
	if seq == "\x1b[0m" || seq == "\x1b[m" {
		return ""
	}
	if strings.HasPrefix(seq, "\x1b[0;") {
		return seq
	}
	return active + seq
}

// splitWords splits a single paragraph line on ASCII space boundaries without
// splitting inside ANSI escape sequences. Trailing or consecutive separator
// spaces between words are collapsed at line breaks.
func splitWords(line string) []string {
	var words []string
	start := -1
	for i := 0; i < len(line); {
		if line[i] == '\x1b' {
			if start == -1 {
				start = i
			}
			i += AnsiSeqLen(line, i)
			continue
		}
		if line[i] == ' ' {
			if start != -1 {
				words = append(words, line[start:i])
				start = -1
			}
			i++
			continue
		}
		if start == -1 {
			start = i
		}
		i++
	}
	if start != -1 {
		words = append(words, line[start:])
	}
	return words
}

// cluster represents an indivisible visual unit (grapheme cluster) with its
// raw byte slice, visual display width, and ANSI SGR state after the cluster.
type cluster struct {
	text         string
	width        int
	sgrBefore    string
	sgrAfter     string
	hasPrintable bool
}

// scanClusters breaks a word into indivisible grapheme clusters while tracking
// ANSI SGR state across each cluster.
func scanClusters(s string, initialSGR string) []cluster {
	var clusters []cluster
	sgr := initialSGR
	i := 0
	n := len(s)

	for i < n {
		clusterStart := i
		sgrBefore := sgr

		// Consume any leading ANSI escape sequences.
		for i < n && s[i] == '\x1b' {
			seqLen := AnsiSeqLen(s, i)
			sgr = updateSGR(sgr, s[i:i+seqLen])
			i += seqLen
		}
		if i >= n {
			// Trailing ANSI sequence(s) after the last printable cluster.
			if len(clusters) > 0 {
				clusters[len(clusters)-1].text += s[clusterStart:i]
				clusters[len(clusters)-1].sgrAfter = sgr
			} else {
				clusters = append(clusters, cluster{
					text:         s[clusterStart:i],
					width:        0,
					sgrBefore:    sgrBefore,
					sgrAfter:     sgr,
					hasPrintable: false,
				})
			}
			break
		}

		// Decode base rune.
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		rw := RuneWidth(r)
		isEmoji := IsEmojiRune(r)

		// Consume attached variation selectors, combining marks, skin tones, and ZWJ chains.
		for i < n {
			if s[i] == '\x1b' {
				break
			}
			nr, nsize := utf8.DecodeRuneInString(s[i:])
			if nr == '\uFE0F' {
				if rw == 1 {
					rw = 2
					isEmoji = true
				}
				i += nsize
				continue
			}
			if (nr >= 0xFE00 && nr <= 0xFE0E) || (nr >= 0xE0100 && nr <= 0xE01EF) {
				i += nsize
				continue
			}
			if IsSkinToneModifier(nr) && isEmoji {
				i += nsize
				continue
			}
			if nr == '\u200D' && isEmoji {
				// Look ahead for a joined emoji rune after ZWJ.
				nextIdx := i + nsize
				if nextIdx < n && s[nextIdx] != '\x1b' {
					jr, jsize := utf8.DecodeRuneInString(s[nextIdx:])
					if IsEmojiRune(jr) {
						i = nextIdx + jsize
						isEmoji = true
						continue
					}
				}
				i += nsize
				continue
			}
			if unicode.Is(unicode.Mn, nr) || unicode.Is(unicode.Me, nr) {
				i += nsize
				continue
			}
			break
		}

		clusters = append(clusters, cluster{
			text:         s[clusterStart:i],
			width:        rw,
			sgrBefore:    sgrBefore,
			sgrAfter:     sgr,
			hasPrintable: true,
		})
	}

	return clusters
}

// splitWordByWidth breaks an oversized word along grapheme cluster boundaries
// into chunks that each fit within maxWidth (making at least 1 cluster of
// progress when maxWidth < clusterWidth).
func splitWordByWidth(word string, maxWidth int, activeSGR *string) []string {
	clusters := scanClusters(word, *activeSGR)
	if len(clusters) == 0 {
		return nil
	}

	var chunks []string
	var cur strings.Builder
	curWidth := 0
	curOpenSGR := *activeSGR
	curSGR := *activeSGR

	flush := func() {
		if cur.Len() == 0 {
			return
		}
		if curSGR != "" {
			cur.WriteString("\x1b[0m")
		}
		chunks = append(chunks, cur.String())
		cur.Reset()
		curWidth = 0
		curOpenSGR = curSGR
	}

	for _, cl := range clusters {
		if curWidth > 0 && curWidth+cl.width > maxWidth {
			flush()
			if curOpenSGR != "" {
				cur.WriteString(curOpenSGR)
			}
		} else if cur.Len() == 0 && curOpenSGR != "" && !strings.HasPrefix(cl.text, curOpenSGR) {
			cur.WriteString(curOpenSGR)
		}
		cur.WriteString(cl.text)
		curWidth += cl.width
		curSGR = cl.sgrAfter
	}

	if cur.Len() > 0 {
		chunks = append(chunks, cur.String())
	}
	*activeSGR = curSGR
	return chunks
}

// trackLineSGR computes the active ANSI SGR state at the end of line given an
// initial SGR state.
func trackLineSGR(line string, initialSGR string) string {
	sgr := initialSGR
	for i := 0; i < len(line); {
		if line[i] == '\x1b' {
			seqLen := AnsiSeqLen(line, i)
			sgr = updateSGR(sgr, line[i:i+seqLen])
			i += seqLen
			continue
		}
		i++
	}
	return sgr
}

// WrapText splits s on newlines and greedily wraps paragraphs to fit within
// maxWidth terminal display cells. Words longer than maxWidth are cleanly split
// on grapheme cluster boundaries. Active ANSI escape sequences are closed at
// line breaks and reopened on continuation lines so styles never bleed into
// surrounding borders.
func WrapText(s string, maxWidth int) []string {
	if strings.ContainsRune(s, '\r') {
		s = strings.ReplaceAll(s, "\r\n", "\n")
	}

	// Fast path for single-line strings that require no wrapping.
	if !strings.ContainsRune(s, '\n') {
		if maxWidth <= 0 || DisplayWidth(s) <= maxWidth {
			return []string{s}
		}
	}

	paragraphs := strings.Split(s, "\n")
	if maxWidth <= 0 {
		out := make([]string, 0, len(paragraphs))
		activeSGR := ""
		for _, p := range paragraphs {
			line := p
			if activeSGR != "" && !strings.HasPrefix(line, activeSGR) {
				line = activeSGR + line
			}
			activeSGR = trackLineSGR(p, activeSGR)
			if activeSGR != "" && !strings.HasSuffix(line, "\x1b[0m") {
				line += "\x1b[0m"
			}
			out = append(out, line)
		}
		return out
	}

	out := make([]string, 0, len(paragraphs))
	activeSGR := ""

	for _, p := range paragraphs {
		if DisplayWidth(p) <= maxWidth {
			line := p
			if activeSGR != "" {
				line = activeSGR + line
			}
			activeSGR = trackLineSGR(p, activeSGR)
			if activeSGR != "" {
				line += "\x1b[0m"
			}
			out = append(out, line)
			continue
		}

		words := splitWords(p)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}

		var cur strings.Builder
		curWidth := 0
		curSGR := activeSGR

		for _, word := range words {
			ww := DisplayWidth(word)
			if ww == 0 {
				if cur.Len() == 0 && curSGR != "" {
					cur.WriteString(curSGR)
				}
				cur.WriteString(word)
				curSGR = trackLineSGR(word, curSGR)
				continue
			}
			if curWidth == 0 {
				if ww <= maxWidth {
					if cur.Len() == 0 && curSGR != "" {
						cur.WriteString(curSGR)
					}
					cur.WriteString(word)
					curWidth = ww
					curSGR = trackLineSGR(word, curSGR)
				} else {
					prefix := ""
					if cur.Len() > 0 {
						if cur.String() != curSGR {
							prefix = cur.String()
						}
						cur.Reset()
					}
					chunks := splitWordByWidth(word, maxWidth, &curSGR)
					if prefix != "" && len(chunks) > 0 {
						chunks[0] = prefix + chunks[0]
					}
					for idx, ch := range chunks {
						if idx < len(chunks)-1 {
							out = append(out, ch)
						} else {
							cur.WriteString(ch)
							curWidth = DisplayWidth(ch)
						}
					}
				}
				continue
			}

			if curWidth+1+ww <= maxWidth {
				cur.WriteByte(' ')
				cur.WriteString(word)
				curWidth += 1 + ww
				curSGR = trackLineSGR(word, curSGR)
				continue
			}

			// Flush completed line.
			if curSGR != "" {
				cur.WriteString("\x1b[0m")
			}
			out = append(out, cur.String())
			cur.Reset()
			curWidth = 0

			if ww <= maxWidth {
				if curSGR != "" {
					cur.WriteString(curSGR)
				}
				cur.WriteString(word)
				curWidth = ww
				curSGR = trackLineSGR(word, curSGR)
			} else {
				chunks := splitWordByWidth(word, maxWidth, &curSGR)
				for idx, ch := range chunks {
					if idx < len(chunks)-1 {
						out = append(out, ch)
					} else {
						cur.WriteString(ch)
						curWidth = DisplayWidth(ch)
					}
				}
			}
		}

		if cur.Len() > 0 {
			if curSGR != "" {
				cur.WriteString("\x1b[0m")
			}
			out = append(out, cur.String())
		}
		activeSGR = curSGR
	}

	return out
}

// writeSpaces writes n ASCII space bytes to b.
func writeSpaces(b *strings.Builder, n int) {
	const spaces = "                                                                "
	for n > 0 {
		if n >= len(spaces) {
			b.WriteString(spaces)
			n -= len(spaces)
		} else {
			b.WriteString(spaces[:n])
			break
		}
	}
}

// WriteAlignedWidth writes s padded with spaces to reach width cells, using
// the precomputed displayWidth of s.
func WriteAlignedWidth(b *strings.Builder, s string, displayWidth, width, align int) {
	if displayWidth >= width {
		b.WriteString(s)
		return
	}
	pad := width - displayWidth
	switch align {
	case AlignRight:
		writeSpaces(b, pad)
		b.WriteString(s)
	case AlignCenter:
		left := pad / 2
		right := pad - left
		writeSpaces(b, left)
		b.WriteString(s)
		writeSpaces(b, right)
	default:
		b.WriteString(s)
		writeSpaces(b, pad)
	}
}

// WriteAligned writes s padded with spaces to fit width using align.
func WriteAligned(b *strings.Builder, s string, width, align int) {
	WriteAlignedWidth(b, s, DisplayWidth(s), width, align)
}

// MeasureSingleLine checks in a fast path whether s fits on a single line
// within maxWidth without wrapping. If s fits as a single line, it returns the
// (ANSI-closed) string, its display width, and ok=true; otherwise ok=false.
func MeasureSingleLine(s string, maxWidth int) (string, int, bool) {
	isASCII := true
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b < 0x20 || b >= 0x7F {
			isASCII = false
			break
		}
	}
	if isASCII {
		if maxWidth <= 0 || len(s) <= maxWidth {
			return s, len(s), true
		}
		return "", 0, false
	}

	if strings.ContainsRune(s, '\n') || strings.ContainsRune(s, '\r') {
		return "", 0, false
	}
	w := DisplayWidth(s)
	if maxWidth > 0 && w > maxWidth {
		return "", 0, false
	}
	if strings.IndexByte(s, '\x1b') != -1 && trackLineSGR(s, "") != "" {
		return s + "\x1b[0m", w, true
	}
	return s, w, true
}

// AlignText returns s padded with spaces to reach width cells according to align.
func AlignText(s string, width, align int) string {
	w := DisplayWidth(s)
	if w >= width {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + (width - w))
	WriteAligned(&b, s, width, align)
	return b.String()
}

