package trellis

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// assertUniformLineWidths verifies that every non-empty rendered line has the
// exact same terminal display width, proving borders stay strictly aligned.
func assertUniformLineWidths(t *testing.T, rendered string) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	if len(lines) == 0 {
		t.Fatal("rendered output has no lines")
	}
	expected := DisplayWidth(lines[0])
	for i, line := range lines {
		w := DisplayWidth(line)
		if w != expected {
			t.Fatalf("line %d has display width %d, want %d\nline: %q\nfull:\n%s", i, w, expected, line, rendered)
		}
	}
}

func TestTableEmpty(t *testing.T) {
	tbl := NewTable()
	if got := tbl.String(); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
	var buf bytes.Buffer
	if err := tbl.Render(&buf); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected 0 bytes written, got %d", buf.Len())
	}
}

func TestTableThemes(t *testing.T) {
	themes := []struct {
		name     string
		theme    Theme
		contains []string
	}{
		{name: "rounded", theme: ThemeRounded, contains: []string{"╭", "╮", "╰", "╯", "│", "├", "┼", "┤"}},
		{name: "ascii", theme: ThemeASCII, contains: []string{"+", "-", "|"}},
		{name: "double", theme: ThemeDouble, contains: []string{"╔", "╗", "╚", "╝", "║", "╠", "╬", "╣"}},
		{name: "markdown", theme: ThemeMarkdown, contains: []string{"|  ID | Name  | Score |", "| --: | :---- | :---: |"}},
		{name: "minimal", theme: ThemeMinimal, contains: []string{"│", "─", "┼"}},
	}

	for _, tc := range themes {
		t.Run(tc.name, func(t *testing.T) {
			tbl := NewTable(WithTheme(tc.theme), WithMaxWidth(0))
			tbl.SetHeaders(
				Header{Title: "ID", Align: AlignRight},
				Header{Title: "Name", Align: AlignLeft},
				Header{Title: "Score", Align: AlignCenter},
			)
			tbl.AddRow("1", "Alpha", "99")
			tbl.AddRow("2", "Beta", "88")

			out := tbl.String()
			assertUniformLineWidths(t, out)
			for _, sub := range tc.contains {
				if !strings.Contains(out, sub) {
					t.Fatalf("theme %s output missing %q:\n%s", tc.name, sub, out)
				}
			}
		})
	}
}

func TestTableThemeTSVAndNone(t *testing.T) {
	for _, theme := range []Theme{ThemeNone, ThemeTSV} {
		tbl := NewTable(WithTheme(theme))
		tbl.SetHeaders(Header{Title: "ID"}, Header{Title: "Name"})
		tbl.AddRow("1", "Alpha")
		tbl.AddRow("2", "Beta")

		want := "ID\tName\n1\tAlpha\n2\tBeta\n"
		if got := tbl.String(); got != want {
			t.Fatalf("TSV output = %q, want %q", got, want)
		}
	}
}

func TestThemeByName(t *testing.T) {
	valid := []string{"rounded", "MARKDOWN", "md", "ascii", "double", "minimal", "none", "tsv"}
	for _, name := range valid {
		if _, ok := ThemeByName(name); !ok {
			t.Fatalf("ThemeByName(%q) returned false", name)
		}
	}
	if _, ok := ThemeByName("unknown"); ok {
		t.Fatal("expected unknown theme to return false")
	}
}

func TestTableAlignmentAndRowSeparator(t *testing.T) {
	tbl := NewTable(WithTheme(ThemeASCII), WithRowSeparator(true), WithMaxWidth(0))
	tbl.SetHeaders(
		Header{Title: "L", Align: AlignLeft, Width: 5},
		Header{Title: "C", Align: AlignCenter, Width: 5},
		Header{Title: "R", Align: AlignRight, Width: 5},
	)
	tbl.AddRow("a", "b", "c")
	tbl.AddRow("xy", "xy", "xy")

	want := strings.Join([]string{
		"+-------+-------+-------+",
		"| L     |   C   |     R |",
		"+-------+-------+-------+",
		"| a     |   b   |     c |",
		"+-------+-------+-------+",
		"| xy    |  xy   |    xy |",
		"+-------+-------+-------+",
		"",
	}, "\n")

	if got := tbl.String(); got != want {
		t.Fatalf("mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestTableDefaultHeaderOnlySeparator(t *testing.T) {
	tbl := NewTable(WithTheme(ThemeASCII), WithMaxWidth(0))
	tbl.SetHeaders(Header{Title: "A"}, Header{Title: "B"})
	tbl.AddRow("1", "2")
	tbl.AddRow("3", "4")

	out := tbl.String()
	sepCount := strings.Count(out, "+---+---+")
	// Expect top border, header separator, and bottom border (3 total).
	if sepCount != 3 {
		t.Fatalf("expected 3 horizontal borders (no per-row separator by default), got %d:\n%s", sepCount, out)
	}
}

func TestTableUnicodeEmojiANSIAndMultiline(t *testing.T) {
	tbl := NewTable(
		WithTheme(ThemeRounded),
		WithMaxWidth(80),
	)
	tbl.SetHeaders(
		Header{Title: "ID", Align: AlignRight},
		Header{Title: "Service", Align: AlignLeft},
		Header{Title: "Health", Align: AlignCenter},
		Header{Title: "Description", MaxWidth: 28},
	)
	tbl.AddRow("1", "Payment Gateway", "\x1b[32m✅ Healthy\x1b[0m", "Menangani transaksi QRIS dan Virtual Account dengan auto-retry.")
	tbl.AddRow("2", "こんにちは", "\x1b[33m⚠️ High Load\x1b[0m", "Line 1\nLine 2\nLine 3")
	tbl.AddRow("3", "👨‍💻 Worker", "⏸️ Paused", "")

	out := tbl.String()
	assertUniformLineWidths(t, out)

	if !strings.Contains(out, "\x1b[32m✅ Healthy\x1b[0m") {
		t.Fatalf("ANSI sequence was stripped from rendered output:\n%s", out)
	}
	if !strings.Contains(out, "Line 2") || !strings.Contains(out, "Line 3") {
		t.Fatalf("multiline cell missing lines:\n%s", out)
	}
}

func TestTableSingleWordOverflowAndANSIWrap(t *testing.T) {
	tbl := NewTable(WithTheme(ThemeASCII), WithMaxWidth(0))
	tbl.SetHeaders(
		Header{Title: "Token", MaxWidth: 8},
		Header{Title: "Styled", MaxWidth: 6},
	)
	tbl.AddRow("supercalifragilistic", "\x1b[31mHelloWorld\x1b[0m")
	tbl.AddRow("こんにちは世界", "normal")

	out := tbl.String()
	assertUniformLineWidths(t, out)

	// Wrapped styled chunks must close ANSI sequences before the table border.
	if !strings.Contains(out, "\x1b[31mHelloW\x1b[0m") {
		t.Fatalf("expected wrapped ANSI chunk to close cleanly before border:\n%s", out)
	}
	if !strings.Contains(out, "\x1b[31morld\x1b[0m") {
		t.Fatalf("expected continuation ANSI chunk to reopen style:\n%s", out)
	}
}

func TestTableWrapTinyWidthWideRune(t *testing.T) {
	// Ensure wrapping a 2-cell rune into MaxWidth=1 makes progress without hanging.
	chunks := WrapText("漢字", 1)
	if len(chunks) != 2 || chunks[0] != "漢" || chunks[1] != "字" {
		t.Fatalf("WrapText(\"漢字\", 1) = %#v, want [\"漢\", \"字\"]", chunks)
	}
}

func TestPublicTextWrappers(t *testing.T) {
	if got := StripANSI("\x1b[32mOK\x1b[0m"); got != "OK" {
		t.Fatalf("StripANSI = %q", got)
	}
	if got := RuneWidth('漢'); got != 2 {
		t.Fatalf("RuneWidth = %d", got)
	}
	if got := AlignText("x", 3, AlignRight); got != "  x" {
		t.Fatalf("AlignText = %q", got)
	}
}

func TestTableMismatchedColumnCountsAndHeaderless(t *testing.T) {
	t.Run("mismatched_with_headers", func(t *testing.T) {
		tbl := NewTable(WithTheme(ThemeASCII), WithMaxWidth(0))
		tbl.SetHeaders(Header{Title: "Col1"}, Header{Title: "Col2"})
		tbl.AddRow("only-one")
		tbl.AddRow("a", "b", "extra-1", "extra-2")

		out := tbl.String()
		assertUniformLineWidths(t, out)
		if !strings.Contains(out, "extra-2") {
			t.Fatalf("expected dynamically expanded column:\n%s", out)
		}
	})

	t.Run("headerless", func(t *testing.T) {
		tbl := NewTable(WithTheme(ThemeRounded), WithMaxWidth(0))
		tbl.AddRow("a", "b")
		tbl.AddRow("1", "2", "3")

		out := tbl.String()
		assertUniformLineWidths(t, out)
		// Headerless table should have top and bottom rounded borders but no header separator '├'.
		if strings.Contains(out, "├") {
			t.Fatalf("headerless table should not contain header separator:\n%s", out)
		}
	})
}

func TestTableMaxWidthClampingAndColumnsEnv(t *testing.T) {
	t.Setenv("COLUMNS", "42")
	tbl := NewTable(WithTheme(ThemeASCII))
	tbl.SetHeaders(
		Header{Title: "ID"},
		Header{Title: "Payload"},
	)
	tbl.AddRow("1", "This is a very long payload string that must wrap cleanly inside 42 terminal columns.")

	out := tbl.String()
	assertUniformLineWidths(t, out)

	firstLine := strings.Split(out, "\n")[0]
	if w := DisplayWidth(firstLine); w > 42 {
		t.Fatalf("rendered width %d exceeds COLUMNS=42:\n%s", w, out)
	}
}

func TestTableRender1000RowsLatency(t *testing.T) {
	tbl := NewTable(WithTheme(ThemeRounded), WithMaxWidth(100))
	tbl.SetHeaders(
		Header{Title: "ID", Align: AlignRight},
		Header{Title: "Service", Align: AlignLeft},
		Header{Title: "Status", Align: AlignCenter},
		Header{Title: "Latency", Align: AlignRight},
	)
	for i := 0; i < 1000; i++ {
		tbl.AddRow(
			fmt.Sprintf("%d", i+1),
			"payment-gateway-service",
			"✅ Healthy",
			"12.4ms",
		)
	}

	var buf bytes.Buffer
	start := time.Now()
	if err := tbl.Render(&buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed >= 50*time.Millisecond {
		t.Fatalf("1000-row render took too long: %v", elapsed)
	}
	t.Logf("1000-row Render to bytes.Buffer took %v (%d bytes)", elapsed, buf.Len())
}

func BenchmarkTableRender1000Rows(b *testing.B) {
	tbl := NewTable(WithTheme(ThemeRounded), WithMaxWidth(100))
	tbl.SetHeaders(
		Header{Title: "ID", Align: AlignRight},
		Header{Title: "Service", Align: AlignLeft},
		Header{Title: "Status", Align: AlignCenter},
		Header{Title: "Latency", Align: AlignRight},
	)
	for i := 0; i < 1000; i++ {
		tbl.AddRow(
			fmt.Sprintf("%d", i+1),
			"payment-gateway-service",
			"✅ Healthy",
			"12.4ms",
		)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := tbl.Render(io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

func TestTreeRenderHierarchy(t *testing.T) {
	tree := NewTree("Root Project")
	sub := tree.AddChild("internal")
	sub.AddChild("mcp")
	sub.AddChild("ui")
	tree.AddChild("cmd")

	want := strings.Join([]string{
		"Root Project",
		"├── internal",
		"│   ├── mcp",
		"│   └── ui",
		"└── cmd",
		"",
	}, "\n")

	if got := tree.String(); got != want {
		t.Fatalf("tree output mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}

	var buf bytes.Buffer
	if err := tree.Render(&buf); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if buf.String() != want {
		t.Fatalf("Render buffer mismatch:\ngot:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestTreeDeepANSIAndBadges(t *testing.T) {
	tree := NewTree("workspace").SetBadge("[root]")
	svc := tree.AddChildWithBadge("\x1b[36mmcp-inspector-tui\x1b[0m", "✅")
	internal := svc.AddChild("internal")
	internal.AddChild("tui").AddChildWithBadge("model.go", "\x1b[32m[clean]\x1b[0m")
	tree.AddChild("trellis").AddChild("table.go\n(core engine)")

	want := strings.Join([]string{
		"workspace [root]",
		"├── \x1b[36mmcp-inspector-tui\x1b[0m ✅",
		"│   └── internal",
		"│       └── tui",
		"│           └── model.go \x1b[32m[clean]\x1b[0m",
		"└── trellis",
		"    └── table.go",
		"        (core engine)",
		"",
	}, "\n")

	if got := tree.String(); got != want {
		t.Fatalf("tree output mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestTreeNilSafety(t *testing.T) {
	var tree *Tree
	if got := tree.String(); got != "" {
		t.Fatalf("expected empty string for nil tree, got %q", got)
	}
	var buf bytes.Buffer
	if err := tree.Render(&buf); err != nil {
		t.Fatalf("unexpected error rendering nil tree: %v", err)
	}
}

func TestUnclosedANSISingleLineCell(t *testing.T) {
	tbl := NewTable(WithTheme(ThemeASCII), WithMaxWidth(0))
	tbl.SetHeaders(Header{Title: "Status"})
	tbl.AddRow("\x1b[32mOK")

	out := tbl.String()
	assertUniformLineWidths(t, out)
	if !strings.Contains(out, "\x1b[32mOK\x1b[0m") {
		t.Fatalf("expected unclosed ANSI color on single-line cell to close before border:\n%s", out)
	}
}

func TestConcurrentRender(t *testing.T) {
	tbl := NewTable(WithTheme(ThemeRounded), WithMaxWidth(60), WithRowSeparator(true))
	tbl.SetHeaders(
		Header{Title: "ID", Align: AlignRight},
		Header{Title: "Name", Align: AlignLeft},
		Header{Title: "Status", Align: AlignCenter},
	)
	for i := 0; i < 25; i++ {
		tbl.AddRow(fmt.Sprintf("%d", i), "こんにちは 🚀", "\x1b[32mOK\x1b[0m")
	}

	expected := tbl.String()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < 10; k++ {
				var buf bytes.Buffer
				if err := tbl.Render(&buf); err != nil {
					t.Errorf("concurrent Render error: %v", err)
					return
				}
				if buf.String() != expected {
					t.Errorf("concurrent output mismatch")
					return
				}
			}
		}()
	}
	wg.Wait()
}

func FuzzTableRender(f *testing.F) {
	f.Add("ID", "Payload", "1", "Hello こんにちは 🚀", 40, 0, false)
	f.Add("ColA", "ColB", "\x1b[31mWARN\x1b[0m", "line1\nline2", 18, 1, true)
	f.Add("", "", "\x1b漢", "👨‍💻👨‍👩‍👧", 10, 2, false)

	themes := []Theme{ThemeRounded, ThemeASCII, ThemeDouble, ThemeMarkdown, ThemeMinimal}
	f.Fuzz(func(t *testing.T, h1, h2, c1, c2 string, maxW, themeIdx int, rowSep bool) {
		if maxW < 0 {
			maxW = 0
		}
		if maxW > 120 {
			maxW = 120
		}
		if themeIdx < 0 {
			themeIdx = -themeIdx
		}
		theme := themes[themeIdx%len(themes)]

		tbl := NewTable(
			WithTheme(theme),
			WithMaxWidth(maxW),
			WithRowSeparator(rowSep),
		)
		tbl.SetHeaders(
			Header{Title: h1, Align: AlignLeft},
			Header{Title: h2, Align: AlignRight},
		)
		tbl.AddRow(c1, c2)
		tbl.AddRow(c2)
		tbl.AddRow(c1, c2, "extra")

		out := tbl.String()
		if out != "" {
			assertUniformLineWidths(t, out)
		}
	})
}

