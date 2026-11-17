package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLICSVFromStdin(t *testing.T) {
	csvData := "ID,Service,Status\n1,Auth,OK\n2,Billing,WARN\n"
	var out, errBuf bytes.Buffer
	err := run([]string{"--theme", "markdown"}, strings.NewReader(csvData), &out, &errBuf)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "| ID  | Service | Status |") || !strings.Contains(got, "| 1   | Auth    | OK     |") {
		t.Fatalf("unexpected markdown output:\n%s", got)
	}
}

func TestCLITSVAutoDetectAndDelimiterFlag(t *testing.T) {
	tsvData := "Host\tPort\nlocalhost\t8080\n"
	var out, errBuf bytes.Buffer
	if err := run([]string{"--theme", "ascii"}, strings.NewReader(tsvData), &out, &errBuf); err != nil {
		t.Fatalf("run TSV auto-detect failed: %v", err)
	}
	if !strings.Contains(out.String(), "| Host      | Port |") {
		t.Fatalf("unexpected TSV output:\n%s", out.String())
	}

	out.Reset()
	pipeData := "Host|Port\nlocalhost|8080\n"
	if err := run([]string{"--theme", "ascii", "--delimiter", "|", "--no-header"}, strings.NewReader(pipeData), &out, &errBuf); err != nil {
		t.Fatalf("run pipe delimiter failed: %v", err)
	}
	if !strings.Contains(out.String(), "| Host      | Port |") || !strings.Contains(out.String(), "| localhost | 8080 |") {
		t.Fatalf("unexpected pipe-delimited output:\n%s", out.String())
	}
	if strings.Count(out.String(), "+-----------+------+") != 2 {
		t.Fatalf("expected only top and bottom borders with --no-header:\n%s", out.String())
	}
}

func TestCLIJSONIngestion(t *testing.T) {
	jsonData := `[
		{"id": 1042, "service": "fuzzspec", "healthy": true},
		{"id": 2088, "service": "dba-architect", "healthy": false, "note": "high latency"}
	]`
	var out, errBuf bytes.Buffer
	err := run([]string{"--theme", "rounded", "--row-separator"}, strings.NewReader(jsonData), &out, &errBuf)
	if err != nil {
		t.Fatalf("run JSON failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{"healthy", "id", "note", "service", "1042", "2088", "fuzzspec", "high latency", "├"} {
		if !strings.Contains(got, want) {
			t.Fatalf("JSON output missing %q:\n%s", want, got)
		}
	}
}

func TestCLIFileArgumentAndMaxWidth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(path, []byte("Name,Description\napi,Handles incoming HTTP requests and routes them cleanly\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	if err := run([]string{"--theme", "ascii", "--max-width", "35", path}, strings.NewReader(""), &out, &errBuf); err != nil {
		t.Fatalf("run file input failed: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) < 5 {
		t.Fatalf("expected wrapped output lines, got:\n%s", out.String())
	}
	if len(lines[0]) > 35 {
		t.Fatalf("expected line width <= 35, got %d:\n%s", len(lines[0]), out.String())
	}
}

func TestCLIErrorsAndEdgeCases(t *testing.T) {
	var out, errBuf bytes.Buffer
	if err := run([]string{"--help"}, strings.NewReader(""), &out, &errBuf); err != nil {
		t.Fatalf("expected --help to return nil, got %v", err)
	}
	if err := run(nil, strings.NewReader("   \n"), &out, &errBuf); err != nil || out.Len() != 0 {
		t.Fatalf("expected empty stdin to produce empty output, err=%v out=%q", err, out.String())
	}
	if err := run(nil, strings.NewReader("[]"), &out, &errBuf); err != nil || out.Len() != 0 {
		t.Fatalf("expected empty JSON array to produce empty output, err=%v out=%q", err, out.String())
	}

	// Literal "\t" delimiter flag, UTF-8 BOM, null, and nested JSON array/object.
	jsonNested := "\xEF\xBB\xBF" + `[{"a": null, "b": [1,2], "c": {"k": "v"}}]`
	out.Reset()
	if err := run([]string{"--theme", "ascii", "--no-header"}, strings.NewReader(jsonNested), &out, &errBuf); err != nil {
		t.Fatalf("nested JSON failed: %v", err)
	}
	if !strings.Contains(out.String(), `[1,2]`) || !strings.Contains(out.String(), `{"k":"v"}`) {
		t.Fatalf("unexpected nested JSON output:\n%s", out.String())
	}

	out.Reset()
	if err := run([]string{"--theme", "tsv", "--delimiter", `\t`}, strings.NewReader("A\tB\n1\t2\n"), &out, &errBuf); err != nil {
		t.Fatalf("literal \\t delimiter failed: %v", err)
	}
	if out.String() != "A\tB\n1\t2\n" {
		t.Fatalf("unexpected TSV output: %q", out.String())
	}

	if err := run([]string{"--theme", "nonexistent"}, strings.NewReader("a,b\n"), &out, &errBuf); err == nil {
		t.Fatal("expected error for unknown theme")
	}
	if err := run([]string{"--delimiter", "::"}, strings.NewReader("a::b\n"), &out, &errBuf); err == nil {
		t.Fatal("expected error for multi-char delimiter")
	}
	if err := run(nil, strings.NewReader("[invalid json"), &out, &errBuf); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if err := run([]string{filepath.Join(t.TempDir(), "does-not-exist.csv")}, strings.NewReader(""), &out, &errBuf); err == nil {
		t.Fatal("expected error for missing input file")
	}
}
