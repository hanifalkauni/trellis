package trellis

import (
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/hanifalkauni/trellis/internal/text"
)

// Table formats tabular data with cell-accurate display width, word-wrapping,
// column alignment, and modular border themes.
type Table struct {
	theme               Theme
	maxWidth            int
	hasExplicitMaxWidth bool
	rowSeparator        bool
	headers             []Header
	rows                []Row
}

// NewTable constructs a new Table with ThemeRounded and applies any options.
// If WithMaxWidth is not specified, it checks the COLUMNS environment variable
// and falls back to DefaultMaxWidth (80).
func NewTable(opts ...Option) *Table {
	t := &Table{
		theme:    ThemeRounded,
		maxWidth: DefaultMaxWidth,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(t)
		}
	}
	if !t.hasExplicitMaxWidth {
		if colsStr := os.Getenv("COLUMNS"); colsStr != "" {
			if cols, err := strconv.Atoi(strings.TrimSpace(colsStr)); err == nil && cols > 0 {
				t.maxWidth = cols
			}
		}
	}
	return t
}

// SetHeaders configures the table column headers and returns the Table for
// fluent chaining.
func (t *Table) SetHeaders(headers ...Header) *Table {
	t.headers = make([]Header, len(headers))
	copy(t.headers, headers)
	return t
}

// AddRow appends a row of cell values to the table and returns the Table for
// fluent chaining. Rows with fewer or more cells than the configured headers
// are automatically padded or expanded during rendering.
func (t *Table) AddRow(cells ...string) *Table {
	t.rows = append(t.rows, NewRow(cells...))
	return t
}

// String renders the table and returns the resulting formatted string.
func (t *Table) String() string {
	var b strings.Builder
	t.renderToBuilder(&b)
	return b.String()
}

// Render writes the formatted table to w.
func (t *Table) Render(w io.Writer) error {
	var b strings.Builder
	t.renderToBuilder(&b)
	if b.Len() == 0 {
		return nil
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func (t *Table) renderToBuilder(b *strings.Builder) {
	numCols := len(t.headers)
	for _, r := range t.rows {
		if len(r.Cells) > numCols {
			numCols = len(r.Cells)
		}
	}
	if numCols == 0 {
		return
	}

	if t.theme.IsRawTSV {
		t.renderTSV(b, numCols)
		return
	}

	hasHeaders := len(t.headers) > 0
	headers := make([]Header, numCols)
	copy(headers, t.headers)

	// Per-column wrap limit and initial widths.
	colLimit := make([]int, numCols)
	isFixed := make([]bool, numCols)
	for c := 0; c < numCols; c++ {
		if headers[c].Width > 0 {
			colLimit[c] = headers[c].Width
			isFixed[c] = true
		} else if headers[c].MaxWidth > 0 {
			colLimit[c] = headers[c].MaxWidth
		}
	}

	// Wrap header lines.
	var wrappedHeaders []cellLayout
	colWidths := make([]int, numCols)
	if hasHeaders {
		wrappedHeaders = make([]cellLayout, numCols)
		for c := 0; c < numCols; c++ {
			cl, maxW := measureCell(headers[c].Title, colLimit[c])
			wrappedHeaders[c] = cl
			if maxW > colWidths[c] {
				colWidths[c] = maxW
			}
		}
	}

	// Wrap data rows into a flat contiguous slice to minimize heap allocations.
	wrappedCells := make([]cellLayout, len(t.rows)*numCols)
	for rIdx, row := range t.rows {
		base := rIdx * numCols
		for c := 0; c < numCols; c++ {
			val := ""
			if c < len(row.Cells) {
				val = row.Cells[c]
			}
			cl, maxW := measureCell(val, colLimit[c])
			wrappedCells[base+c] = cl
			if maxW > colWidths[c] {
				colWidths[c] = maxW
			}
		}
	}

	// Apply fixed widths and Markdown minimum width (--- / :--- / :---: / ---:).
	minColWidth := 0
	if t.theme.IsMarkdown && hasHeaders {
		minColWidth = 3
	}
	for c := 0; c < numCols; c++ {
		if isFixed[c] {
			colWidths[c] = headers[c].Width
		}
		if colWidths[c] < minColWidth {
			colWidths[c] = minColWidth
		}
	}

	// Enforce table-level maxWidth by water-filling flexible columns if needed.
	if t.maxWidth > 0 {
		overhead := t.borderOverhead(numCols)
		budget := t.maxWidth - overhead
		if budget < numCols {
			budget = numCols
		}
		total := 0
		for _, w := range colWidths {
			total += w
		}
		if total > budget {
			adjusted := clampColumnWidths(colWidths, isFixed, minColWidth, budget)
			for c := 0; c < numCols; c++ {
				if adjusted[c] < colWidths[c] {
					colLimit[c] = adjusted[c]
					newWidth := 0
					if hasHeaders {
						cl, maxW := measureCell(headers[c].Title, colLimit[c])
						wrappedHeaders[c] = cl
						if maxW > newWidth {
							newWidth = maxW
						}
					}
					for rIdx, row := range t.rows {
						val := ""
						if c < len(row.Cells) {
							val = row.Cells[c]
						}
						cl, maxW := measureCell(val, colLimit[c])
						wrappedCells[rIdx*numCols+c] = cl
						if maxW > newWidth {
							newWidth = maxW
						}
					}
					if newWidth < minColWidth {
						newWidth = minColWidth
					}
					colWidths[c] = newWidth
				}
			}
		}
	}

	// Pre-build reusable horizontal borders.
	var topBorder, headerSepBorder, rowSepBorder, botBorder string
	if t.theme.ShowTop {
		topBorder = t.buildBorderLine(colWidths, t.theme.TopLeft, t.theme.TopMid, t.theme.TopRight, t.theme.Horizontal)
	}
	if t.theme.ShowHeaderSep && hasHeaders {
		if t.theme.IsMarkdown {
			headerSepBorder = t.buildMarkdownSepLine(colWidths, headers)
		} else {
			headerSepBorder = t.buildBorderLine(colWidths, t.theme.MidLeft, t.theme.MidMid, t.theme.MidRight, t.theme.Horizontal)
		}
	}
	if t.rowSeparator && !t.theme.IsMarkdown && len(t.rows) > 1 {
		rowSepBorder = t.buildBorderLine(colWidths, t.theme.MidLeft, t.theme.MidMid, t.theme.MidRight, t.theme.Horizontal)
	}
	if t.theme.ShowBottom {
		botBorder = t.buildBorderLine(colWidths, t.theme.BotLeft, t.theme.BotMid, t.theme.BotRight, t.theme.Horizontal)
	}

	// Precompute vertical divider strings for inner row loop.
	var leftBorder, midBorder, rightBorder string
	if t.theme.ShowOuterSide {
		leftBorder = t.theme.Vertical + " "
		midBorder = " " + t.theme.Vertical + " "
		rightBorder = " " + t.theme.Vertical
	} else {
		midBorder = " " + t.theme.Vertical + " "
	}

	// Estimate output byte size accurately to avoid strings.Builder reallocations.
	estRowBytes := len(leftBorder) + len(rightBorder) + (numCols-1)*len(midBorder) + 1 + numCols*4
	for _, w := range colWidths {
		estRowBytes += w
	}
	totalEst := len(topBorder) + len(headerSepBorder) + len(botBorder) + 4
	rowCount := len(t.rows)
	if hasHeaders {
		rowCount++
	}
	totalEst += rowCount * estRowBytes
	if rowSepBorder != "" && len(t.rows) > 1 {
		totalEst += (len(t.rows) - 1) * (len(rowSepBorder) + 1)
	}
	b.Grow(totalEst)

	// Emit top border.
	if topBorder != "" {
		b.WriteString(topBorder)
		b.WriteByte('\n')
	}

	// Emit header row + header separator.
	if hasHeaders {
		writeCellRow(b, wrappedHeaders, colWidths, headers, leftBorder, midBorder, rightBorder)
		if headerSepBorder != "" {
			b.WriteString(headerSepBorder)
			b.WriteByte('\n')
		}
	}

	// Emit data rows.
	for rIdx := 0; rIdx < len(t.rows); rIdx++ {
		if rIdx > 0 && rowSepBorder != "" {
			b.WriteString(rowSepBorder)
			b.WriteByte('\n')
		}
		base := rIdx * numCols
		writeCellRow(b, wrappedCells[base:base+numCols], colWidths, headers, leftBorder, midBorder, rightBorder)
	}

	// Emit bottom border.
	if botBorder != "" {
		b.WriteString(botBorder)
		b.WriteByte('\n')
	}
}

func (t *Table) renderTSV(b *strings.Builder, numCols int) {
	if len(t.headers) > 0 {
		for c := 0; c < numCols; c++ {
			if c > 0 {
				b.WriteByte('\t')
			}
			if c < len(t.headers) {
				b.WriteString(t.headers[c].Title)
			}
		}
		b.WriteByte('\n')
	}
	for _, row := range t.rows {
		for c := 0; c < numCols; c++ {
			if c > 0 {
				b.WriteByte('\t')
			}
			if c < len(row.Cells) {
				b.WriteString(row.Cells[c])
			}
		}
		b.WriteByte('\n')
	}
}

func (t *Table) borderOverhead(numCols int) int {
	vw := text.DisplayWidth(t.theme.Vertical)
	if t.theme.ShowOuterSide {
		return (numCols+1)*vw + numCols*2
	}
	if numCols <= 1 {
		return 0
	}
	return (numCols - 1) * (vw + 2)
}

func (t *Table) buildBorderLine(colWidths []int, left, mid, right, horiz string) string {
	var b strings.Builder
	numCols := len(colWidths)
	if t.theme.ShowOuterSide {
		b.WriteString(left)
		for c, w := range colWidths {
			if c > 0 {
				b.WriteString(mid)
			}
			for i := 0; i < w+2; i++ {
				b.WriteString(horiz)
			}
		}
		b.WriteString(right)
		return b.String()
	}

	// ThemeMinimal: no outer walls; spaces only adjacent to inner dividers.
	for c, w := range colWidths {
		if c > 0 {
			b.WriteString(mid)
		}
		padLeft := 0
		if c > 0 {
			padLeft = 1
		}
		padRight := 0
		if c < numCols-1 {
			padRight = 1
		}
		seg := w + padLeft + padRight
		for i := 0; i < seg; i++ {
			b.WriteString(horiz)
		}
	}
	return b.String()
}

func (t *Table) buildMarkdownSepLine(colWidths []int, headers []Header) string {
	var b strings.Builder
	b.WriteByte('|')
	for c, w := range colWidths {
		b.WriteByte(' ')
		switch headers[c].Align {
		case AlignRight:
			for i := 0; i < w-1; i++ {
				b.WriteByte('-')
			}
			b.WriteByte(':')
		case AlignCenter:
			b.WriteByte(':')
			for i := 0; i < w-2; i++ {
				b.WriteByte('-')
			}
			b.WriteByte(':')
		default:
			b.WriteByte(':')
			for i := 0; i < w-1; i++ {
				b.WriteByte('-')
			}
		}
		b.WriteString(" |")
	}
	return b.String()
}

type cellLayout struct {
	single      string
	singleWidth int
	lines       []string
	lineWidths  []int
}

func measureCell(s string, maxWidth int) (cellLayout, int) {
	if single, w, ok := text.MeasureSingleLine(s, maxWidth); ok {
		return cellLayout{single: single, singleWidth: w}, w
	}
	lines := text.WrapText(s, maxWidth)
	widths := make([]int, len(lines))
	maxW := 0
	for i, ln := range lines {
		w := text.DisplayWidth(ln)
		widths[i] = w
		if w > maxW {
			maxW = w
		}
	}
	return cellLayout{lines: lines, lineWidths: widths}, maxW
}

func writeCellRow(b *strings.Builder, cells []cellLayout, colWidths []int, headers []Header, leftBorder, midBorder, rightBorder string) {
	maxLines := 1
	for i := range cells {
		if len(cells[i].lines) > maxLines {
			maxLines = len(cells[i].lines)
		}
	}

	numCols := len(colWidths)
	for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
		if leftBorder != "" {
			b.WriteString(leftBorder)
		}
		for c := 0; c < numCols; c++ {
			if c > 0 {
				b.WriteString(midBorder)
			}

			val := ""
			vw := 0
			cl := &cells[c]
			if cl.lines == nil {
				if lineIdx == 0 {
					val = cl.single
					vw = cl.singleWidth
				}
			} else if lineIdx < len(cl.lines) {
				val = cl.lines[lineIdx]
				vw = cl.lineWidths[lineIdx]
			}
			text.WriteAlignedWidth(b, val, vw, colWidths[c], int(headers[c].Align))
		}
		if rightBorder != "" {
			b.WriteString(rightBorder)
		}
		b.WriteByte('\n')
	}
}

// clampColumnWidths reduces flexible column widths via water-filling so the
// sum of all column widths fits within budget while leaving narrow columns
// untouched whenever possible.
func clampColumnWidths(colWidths []int, isFixed []bool, minColWidth int, budget int) []int {
	n := len(colWidths)
	res := make([]int, n)
	copy(res, colWidths)

	floor := minColWidth
	if floor < 1 {
		floor = 1
	}

	// Shrink flexible columns first, keeping fixed columns untouched unless
	// fixed columns alone exceed budget.
	shrinkPass := func(allowShrinkFixed bool) {
		sum := 0
		maxW := 0
		for c, w := range res {
			sum += w
			if (!isFixed[c] || allowShrinkFixed) && w > maxW {
				maxW = w
			}
		}
		if sum <= budget || maxW <= floor {
			return
		}

		// Binary search for largest water-fill cap in [floor, maxW].
		lo, hi := floor, maxW
		bestCap := floor
		for lo <= hi {
			mid := lo + (hi-lo)/2
			total := 0
			for c, w := range res {
				if isFixed[c] && !allowShrinkFixed {
					total += w
					continue
				}
				cw := w
				if cw > mid {
					cw = mid
				}
				if cw < floor && w >= floor {
					cw = floor
				}
				total += cw
			}
			if total <= budget {
				bestCap = mid
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}

		// Apply bestCap.
		used := 0
		for c, w := range res {
			if isFixed[c] && !allowShrinkFixed {
				used += w
				continue
			}
			if w > bestCap {
				res[c] = bestCap
			}
			used += res[c]
		}

		// Distribute remaining budget (+1 cell) to columns that were capped.
		rem := budget - used
		for c := 0; c < n && rem > 0; c++ {
			if isFixed[c] && !allowShrinkFixed {
				continue
			}
			if res[c] < colWidths[c] {
				res[c]++
				rem--
			}
		}
	}

	shrinkPass(false)
	sum := 0
	for _, w := range res {
		sum += w
	}
	if sum > budget {
		shrinkPass(true)
	}
	return res
}
