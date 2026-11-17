package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/hanifalkauni/trellis"
)

type cliConfig struct {
	themeName    string
	maxWidth     int
	delimiter    string
	noHeader     bool
	rowSeparator bool
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "trellis: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	var cfg cliConfig
	fs := flag.NewFlagSet("trellis", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.themeName, "theme", "rounded", "border theme (rounded, markdown, ascii, double, minimal, none, tsv)")
	fs.IntVar(&cfg.maxWidth, "max-width", 0, "maximum table width in terminal columns (0 = unbounded)")
	fs.StringVar(&cfg.delimiter, "delimiter", "", "CSV field delimiter (auto-detects ',' or '\\t' if empty)")
	fs.BoolVar(&cfg.noHeader, "no-header", false, "treat the first record as a data row instead of headers")
	fs.BoolVar(&cfg.rowSeparator, "row-separator", false, "render horizontal separators between every data row")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	theme, ok := trellis.ThemeByName(cfg.themeName)
	if !ok {
		return fmt.Errorf("unknown theme %q (valid: rounded, markdown, ascii, double, minimal, none, tsv)", cfg.themeName)
	}

	var inputData []byte
	var err error
	if fs.NArg() > 0 {
		inputData, err = os.ReadFile(fs.Arg(0))
		if err != nil {
			return fmt.Errorf("reading %s: %w", fs.Arg(0), err)
		}
	} else {
		inputData, err = io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
	}

	// Strip optional UTF-8 BOM.
	inputData = bytes.TrimPrefix(inputData, []byte("\xEF\xBB\xBF"))
	trimmed := bytes.TrimSpace(inputData)
	if len(trimmed) == 0 {
		return nil
	}

	tbl := trellis.NewTable(
		trellis.WithTheme(theme),
		trellis.WithMaxWidth(cfg.maxWidth),
		trellis.WithRowSeparator(cfg.rowSeparator),
	)

	if trimmed[0] == '[' {
		if err := populateFromJSON(tbl, trimmed, cfg.noHeader); err != nil {
			return err
		}
	} else {
		if err := populateFromCSV(tbl, trimmed, cfg.delimiter, cfg.noHeader); err != nil {
			return err
		}
	}

	return tbl.Render(stdout)
}

func populateFromJSON(tbl *trellis.Table, data []byte, noHeader bool) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var items []map[string]any
	if err := dec.Decode(&items); err != nil {
		return fmt.Errorf("parsing JSON array of objects: %w", err)
	}
	if len(items) == 0 {
		return nil
	}

	keySet := make(map[string]struct{})
	for _, obj := range items {
		for k := range obj {
			keySet[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		return nil
	}

	if !noHeader {
		headers := make([]trellis.Header, len(keys))
		for i, k := range keys {
			headers[i] = trellis.Header{Title: k}
		}
		tbl.SetHeaders(headers...)
	}

	row := make([]string, len(keys))
	for _, obj := range items {
		for i, k := range keys {
			row[i] = formatJSONValue(obj[k])
		}
		tbl.AddRow(row...)
	}
	return nil
}

func formatJSONValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case json.Number:
		return val.String()
	case bool:
		return strconv.FormatBool(val)
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprint(val)
		}
		return string(b)
	}
}

func populateFromCSV(tbl *trellis.Table, data []byte, delimiter string, noHeader bool) error {
	delimRune, err := resolveDelimiter(data, delimiter)
	if err != nil {
		return err
	}

	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = delimRune
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	records, err := r.ReadAll()
	if err != nil {
		return fmt.Errorf("parsing CSV/TSV: %w", err)
	}
	if len(records) == 0 {
		return nil
	}

	startIdx := 0
	if !noHeader {
		headers := make([]trellis.Header, len(records[0]))
		for i, title := range records[0] {
			headers[i] = trellis.Header{Title: title}
		}
		tbl.SetHeaders(headers...)
		startIdx = 1
	}

	for i := startIdx; i < len(records); i++ {
		tbl.AddRow(records[i]...)
	}
	return nil
}

func resolveDelimiter(data []byte, delimiter string) (rune, error) {
	if delimiter != "" {
		if delimiter == `\t` {
			return '\t', nil
		}
		r, size := utf8.DecodeRuneInString(delimiter)
		if r == utf8.RuneError || size != len(delimiter) {
			return 0, fmt.Errorf("delimiter %q must be a single character", delimiter)
		}
		return r, nil
	}

	// Auto-detect ',' vs '\t' from the first line.
	firstLine := data
	if nl := bytes.IndexByte(data, '\n'); nl != -1 {
		firstLine = data[:nl]
	}
	tabs := bytes.Count(firstLine, []byte{'\t'})
	commas := bytes.Count(firstLine, []byte{','})
	if tabs > commas {
		return '\t', nil
	}
	return ',', nil
}
