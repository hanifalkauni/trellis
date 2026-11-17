# trellis

[![Go Version](https://img.shields.io/badge/go-1.24%2B-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-0-success.svg)](go.mod)

Ultra-light, zero-dependency, memory-efficient ASCII/Unicode **Table** and **Tree** formatting library for Go 1.24+, plus a standalone CLI utility (`cmd/trellis`).

---

## Highlights & Guarantees

- **100% Pure Standard Library (Zero External Dependencies)**: No third-party modules or `golang.org/x/*` packages in `go.mod`.
- **Deterministic Display Cell Width (`East Asian Width + Emoji + ANSI`)**:
  - ANSI Escape Codes (CSI SGR color sequences, OSC hyperlinks) are measured with zero printable width while preserved intact in rendered terminal output.
  - Fullwidth / Wide characters (UAX #11 CJK Kanji, Katakana, Hiragana, Hangul) occupy 2 terminal cells.
  - Emojis, Variation Selector-16 (`\uFE0F`), Fitzpatrick skin-tone modifiers, and Zero-Width Joiner (ZWJ) sequences (`👨‍💻`, `👨‍👩‍👧`, `🏳️‍🌈`) align cleanly with vertical borders.
- **Smart Greedy Word-Wrapping & Multiline Rows**:
  - Handles per-column `MaxWidth` and fixed `Width`.
  - Seamlessly handles embedded `\n` newlines across neighboring cells.
  - Active ANSI color sequences are cleanly closed before line breaks and reopened on continuation lines without bleeding into table border lines.
- **7 Built-in Theme Presets**: Rounded Unicode, GitHub-Flavored Markdown, Classic ASCII, Double-Line, Minimal, Raw TSV, and None.
- **Unified Tree View Formatter**: Hierarchical recursive tree branch rendering (`├── `, `└── `, `│   `) with status badges and colored labels.
- **High Performance & Low Allocation**: 1,000-row table rendering in ~1.5–2ms with flat buffer reuse and precomputed border templates.

---

## Installation

### As a Go Library

```bash
go get github.com/hanifalkauni/trellis
```

### As a Standalone CLI Utility

```bash
go install github.com/hanifalkauni/trellis/cmd/trellis@latest
```

---

## Quickstart & Library Usage

### 1. Table Formatting

```go
package main

import (
	"os"

	"github.com/hanifalkauni/trellis"
)

func main() {
	// Initialize table with options
	t := trellis.NewTable(
		trellis.WithTheme(trellis.ThemeRounded),
		trellis.WithMaxWidth(80),
		trellis.WithRowSeparator(false),
	)

	// Set column headers and alignment
	t.SetHeaders(
		trellis.Header{Title: "ID", Align: trellis.AlignRight},
		trellis.Header{Title: "Service", Align: trellis.AlignLeft},
		trellis.Header{Title: "Health", Align: trellis.AlignCenter},
		trellis.Header{Title: "Description", MaxWidth: 30},
	)

	// Add data rows (supports ANSI color, CJK, Emojis, and multi-line strings)
	t.AddRow("1", "Payment Gateway", "✅ Healthy", "Menangani transaksi QRIS dan Virtual Account dengan auto-retry.")
	t.AddRow("2", "Notification Worker", "⚠️ High Load", "Antrean email tertunda karena rate limit SMTP server.")

	// Render directly to any io.Writer (e.g. os.Stdout)
	_ = t.Render(os.Stdout)

	// Or retrieve the formatted string
	// str := t.String()
}
```

#### Rendered Output:

```text
╭────┬─────────────────────┬─────────────┬────────────────────────────────╮
│ ID │ Service             │   Health    │ Description                    │
├────┼─────────────────────┼─────────────┼────────────────────────────────┤
│  1 │ Payment Gateway     │ ✅ Healthy  │ Menangani transaksi QRIS dan   │
│    │                     │             │ Virtual Account dengan         │
│    │                     │             │ auto-retry.                    │
│  2 │ Notification Worker │ ⚠️ High Load │ Antrean email tertunda karena  │
│    │                     │             │ rate limit SMTP server.        │
╰────┴─────────────────────┴─────────────┴────────────────────────────────╯
```

---

### 2. Tree Formatting

```go
package main

import (
	"os"

	"github.com/hanifalkauni/trellis"
)

func main() {
	tree := trellis.NewTree("Root Project").SetBadge("[v1.0.0]")
	
	internal := tree.AddChild("internal")
	internal.AddChild("text")
	internal.AddChildWithBadge("mcp", "✅")
	
	tree.AddChild("cmd").AddChild("trellis")

	_ = tree.Render(os.Stdout)
}
```

#### Rendered Output:

```text
Root Project [v1.0.0]
├── internal
│   ├── text
│   └── mcp ✅
└── cmd
    └── trellis
```

---

## Themes & Visual Presets

| Theme | Constant | Preview |
| :--- | :--- | :--- |
| **Rounded** (Default) | `trellis.ThemeRounded` | `╭─┬─╮`, `│ │ │`, `├─┼─┤`, `╰─┴─╯` |
| **Markdown** (GitHub PR ready) | `trellis.ThemeMarkdown` | `\| --- \| :---: \| ---: \|` |
| **ASCII** | `trellis.ThemeASCII` | `+---+---+`, `\|   \|   \|` |
| **Double** | `trellis.ThemeDouble` | `╔═══╦═══╗`, `║   ║   ║`, `╚═══╩═══╝` |
| **Minimal** | `trellis.ThemeMinimal` | Borderless outer walls, single inner dividers `│` |
| **TSV / None** | `trellis.ThemeTSV` | Raw tab-separated output (`\t`), zero box formatting |

---

## Standalone CLI Usage (`cmd/trellis`)

The `trellis` CLI ingests CSV, TSV, or JSON (`[]map[string]any`) from `stdin` or file arguments.

### Examples

```bash
# Pipe CSV into a rounded Unicode table
cat services.csv | trellis --theme rounded

# Pipe JSON array of objects into GitHub-Flavored Markdown
cat metrics.json | trellis --theme markdown

# Pipe tab-separated or pipe-delimited data with explicit delimiter
cat report.tsv | trellis --delimiter "\t" --theme ascii

# Wrap long descriptions within a terminal width of 80 columns
trellis --theme rounded --max-width 80 data.csv
```

### CLI Flags

| Flag | Default | Description |
| :--- | :--- | :--- |
| `--theme` | `rounded` | Border theme (`rounded`, `markdown`, `ascii`, `double`, `minimal`, `none`, `tsv`) |
| `--max-width` | `0` | Maximum table width in terminal columns (`0` = unbounded, fallback to `COLUMNS` env) |
| `--delimiter` | auto | CSV delimiter (auto-detects `,` or `\t` when omitted) |
| `--no-header` | `false` | Treat the first line as a data row instead of column headers |
| `--row-separator` | `false` | Draw horizontal dividers between every single data row |

---

## Benchmarks & Performance

Measured on Go 1.24 (AMD Ryzen 5, 8 threads):

| Benchmark Target | Ops / Latency | Memory / Op | Allocations |
| :--- | :--- | :--- | :--- |
| **1,000 Rows Table Render** | `~1.2 ms` | `387 KB/op` | `25 allocs/op` |
| **Display Width Calculation** | `~700 ns` | `0 B/op` | `0 allocs/op` |

---

## Project Structure

```text
trellis/
├── go.mod
├── trellis.go              # Public API: Option, Header, Alignment, Theme presets, and public helpers
├── table.go                # Core Table layout, dynamic auto-fit, and rendering engine
├── tree.go                 # Hierarchical Tree and Node renderer
├── trellis_test.go         # Full table & tree test suite, benchmarks, and concurrency tests
├── cmd/
│   └── trellis/
│       ├── main.go         # Standalone CLI binary (CSV/TSV/JSON pipeline)
│       └── main_test.go    # CLI unit & ingestion tests
├── internal/
│   └── text/
│       ├── width.go        # Deterministic cell width engine (ANSI, East Asian, Emojis, ZWJ)
│       ├── wrap.go         # Greedy word-wrapping & ANSI-safe continuation splitter
│       └── width_test.go   # Unicode edge-case tests & fuzzing suite
└── .github/
    └── workflows/
        └── ci.yml          # GitHub Actions cross-platform CI matrix
```

---

## License

MIT License — see [LICENSE](LICENSE) for details.
