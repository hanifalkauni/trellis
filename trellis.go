package trellis

import (
	"strings"

	"github.com/hanifalkauni/trellis/internal/text"
)

// DefaultMaxWidth is the fallback maximum table width in terminal columns when
// neither WithMaxWidth nor the COLUMNS environment variable is specified.
const DefaultMaxWidth = 80

// Alignment defines how text is horizontally aligned inside a table column.
type Alignment int

const (
	// AlignLeft aligns text to the left edge of the column (default).
	AlignLeft Alignment = Alignment(text.AlignLeft)
	// AlignCenter centers text horizontally within the column.
	AlignCenter Alignment = Alignment(text.AlignCenter)
	// AlignRight aligns text to the right edge of the column.
	AlignRight Alignment = Alignment(text.AlignRight)
)

// Header configures a table column's title, horizontal alignment, and width
// constraints.
type Header struct {
	Title    string
	Align    Alignment
	MaxWidth int // Maximum visual cell width before word-wrapping (0 = unbounded)
	Width    int // Fixed visual cell width when > 0
}

// Cell represents a single cell value within a table row.
type Cell struct {
	Content string
}

// Row represents a single horizontal record of cell strings in a table.
type Row struct {
	Cells []string
}

// NewRow constructs a Row from the provided cell strings.
func NewRow(cells ...string) Row {
	cp := make([]string, len(cells))
	copy(cp, cells)
	return Row{Cells: cp}
}

// Option configures a Table during construction via NewTable.
type Option func(*Table)

// WithTheme sets the border rendering theme for the table.
func WithTheme(theme Theme) Option {
	return func(t *Table) {
		t.theme = theme
	}
}

// WithMaxWidth sets the maximum overall visual width (in terminal cells) for
// the rendered table. A value <= 0 disables total table width clamping.
func WithMaxWidth(width int) Option {
	return func(t *Table) {
		t.maxWidth = width
		t.hasExplicitMaxWidth = true
	}
}

// WithRowSeparator controls whether horizontal divider lines are drawn between
// every data row. By default, only the header separator is drawn.
func WithRowSeparator(enabled bool) Option {
	return func(t *Table) {
		t.rowSeparator = enabled
	}
}

// Theme defines the box-drawing characters and layout flags used to render a
// Table.
type Theme struct {
	Name          string
	TopLeft       string
	TopMid        string
	TopRight      string
	MidLeft       string
	MidMid        string
	MidRight      string
	BotLeft       string
	BotMid        string
	BotRight      string
	Horizontal    string
	Vertical      string
	ShowTop       bool
	ShowBottom    bool
	ShowHeaderSep bool
	ShowOuterSide bool
	IsMarkdown    bool
	IsRawTSV      bool
}

var (
	// ThemeRounded renders modern rounded Unicode borders (╭─┬─╮, │, ├─┼─┤, ╰─┴─╯).
	ThemeRounded = Theme{
		Name:          "rounded",
		TopLeft:       "╭",
		TopMid:        "┬",
		TopRight:      "╮",
		MidLeft:       "├",
		MidMid:        "┼",
		MidRight:      "┤",
		BotLeft:       "╰",
		BotMid:        "┴",
		BotRight:      "╯",
		Horizontal:    "─",
		Vertical:      "│",
		ShowTop:       true,
		ShowBottom:    true,
		ShowHeaderSep: true,
		ShowOuterSide: true,
	}

	// ThemeASCII renders classic 7-bit ASCII borders (+---+ and |   |).
	ThemeASCII = Theme{
		Name:          "ascii",
		TopLeft:       "+",
		TopMid:        "+",
		TopRight:      "+",
		MidLeft:       "+",
		MidMid:        "+",
		MidRight:      "+",
		BotLeft:       "+",
		BotMid:        "+",
		BotRight:      "+",
		Horizontal:    "-",
		Vertical:      "|",
		ShowTop:       true,
		ShowBottom:    true,
		ShowHeaderSep: true,
		ShowOuterSide: true,
	}

	// ThemeDouble renders double-line Unicode box borders (╔═╦═╗, ║, ╠═╬═╣, ╚═╩═╝).
	ThemeDouble = Theme{
		Name:          "double",
		TopLeft:       "╔",
		TopMid:        "╦",
		TopRight:      "╗",
		MidLeft:       "╠",
		MidMid:        "╬",
		MidRight:      "╣",
		BotLeft:       "╚",
		BotMid:        "╩",
		BotRight:      "╝",
		Horizontal:    "═",
		Vertical:      "║",
		ShowTop:       true,
		ShowBottom:    true,
		ShowHeaderSep: true,
		ShowOuterSide: true,
	}

	// ThemeMarkdown renders GitHub-Flavored Markdown tables with alignment
	// indicators (:---, :---:, ---:).
	ThemeMarkdown = Theme{
		Name:          "markdown",
		MidLeft:       "|",
		MidMid:        "|",
		MidRight:      "|",
		Horizontal:    "-",
		Vertical:      "|",
		ShowTop:       false,
		ShowBottom:    false,
		ShowHeaderSep: true,
		ShowOuterSide: true,
		IsMarkdown:    true,
	}

	// ThemeMinimal renders borderless outer walls with single inner column
	// dividers (│) and a horizontal header separator (─ / ┼).
	ThemeMinimal = Theme{
		Name:          "minimal",
		MidMid:        "┼",
		Horizontal:    "─",
		Vertical:      "│",
		ShowTop:       false,
		ShowBottom:    false,
		ShowHeaderSep: true,
		ShowOuterSide: false,
	}

	// ThemeNone renders plain tab-separated values without borders or padding.
	ThemeNone = Theme{
		Name:     "none",
		IsRawTSV: true,
	}

	// ThemeTSV is an alias for ThemeNone, rendering raw tab-separated values.
	ThemeTSV = Theme{
		Name:     "tsv",
		IsRawTSV: true,
	}
)

// ThemeByName resolves a theme preset by its case-insensitive name
// ("rounded", "markdown", "ascii", "double", "minimal", "none", "tsv").
func ThemeByName(name string) (Theme, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "rounded":
		return ThemeRounded, true
	case "markdown", "md":
		return ThemeMarkdown, true
	case "ascii":
		return ThemeASCII, true
	case "double":
		return ThemeDouble, true
	case "minimal":
		return ThemeMinimal, true
	case "none":
		return ThemeNone, true
	case "tsv":
		return ThemeTSV, true
	default:
		return Theme{}, false
	}
}

// StripANSI removes ANSI escape sequences from s while preserving printable text.
func StripANSI(s string) string {
	return text.StripANSI(s)
}

// RuneWidth returns the terminal display cell width (0, 1, or 2) of r.
func RuneWidth(r rune) int {
	return text.RuneWidth(r)
}

// DisplayWidth calculates the deterministic terminal cell width of s,
// neutralizing ANSI escape codes and respecting East Asian Wide/Emoji widths.
func DisplayWidth(s string) int {
	return text.DisplayWidth(s)
}

// WrapText splits s on newlines and greedily wraps paragraphs to fit within
// maxWidth terminal display cells.
func WrapText(s string, maxWidth int) []string {
	return text.WrapText(s, maxWidth)
}

// AlignText returns s padded with spaces to reach width cells according to align.
func AlignText(s string, width int, align Alignment) string {
	return text.AlignText(s, width, int(align))
}

