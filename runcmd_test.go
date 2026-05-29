package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHelperProcess is not a real test. It is executed as the "editor" by
// TestRuncmdSpacedFile and writes the arguments it received (one per line) to
// the file named by the MEMO_HELPER_OUT environment variable.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	out := os.Getenv("MEMO_HELPER_OUT")
	// os.Args is: <testbin> -test.run=TestHelperProcess -- <editor args...>
	var fileArgs []string
	seenSep := false
	for _, a := range os.Args[1:] {
		if !seenSep {
			if a == "--" {
				seenSep = true
			}
			continue
		}
		fileArgs = append(fileArgs, a)
	}
	_ = os.WriteFile(out, []byte(strings.Join(fileArgs, "\n")), 0644)
	os.Exit(0)
}

// TestRuncmdSpacedFile verifies that runcmd passes a path containing spaces to
// the editor as a single argument. Regression test for "memo edit" failing to
// open files whose names contain spaces.
func TestRuncmdSpacedFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "args.txt")
	file := filepath.Join(dir, "2024-01-01-hello world.md")
	if err := os.WriteFile(file, []byte("memo"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("MEMO_HELPER_OUT", out)

	editor := shellquote(os.Args[0]) + " -test.run=TestHelperProcess --"
	var cfg config
	if err := cfg.runcmd(editor, "", file); err != nil {
		t.Fatalf("runcmd failed: %v", err)
	}

	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("helper did not write output: %v", err)
	}
	got := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(got) != 1 {
		t.Fatalf("editor received %d args, want 1: %q", len(got), got)
	}
	if got[0] != file {
		t.Fatalf("editor received %q, want %q", got[0], file)
	}
}
