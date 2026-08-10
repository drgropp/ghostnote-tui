package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/drgropp/ghostnote-tui/scribe"
)

type fakePathScribeRunner struct {
	result    scribe.Result
	err       error
	operation scribe.Operation
	path      string
}

func (f *fakePathScribeRunner) RunPath(_ context.Context, operation scribe.Operation, path string) (scribe.Result, error) {
	f.operation = operation
	f.path = path
	return f.result, f.err
}

func TestScribeCLIUsageValidation(t *testing.T) {
	var stdout, stderr strings.Builder
	handled, code := runCLI([]string{"scribe"}, &stdout, &stderr)
	if !handled || code != 2 {
		t.Fatalf("handled=%v code=%d", handled, code)
	}
	if !strings.Contains(stderr.String(), scribeCLIUsage) {
		t.Fatalf("stderr = %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	handled, code = runCLI([]string{"scribe", "--help"}, &stdout, &stderr)
	if !handled || code != 0 || !strings.Contains(stdout.String(), scribeCLIUsage) {
		t.Fatalf("handled=%v code=%d stdout=%q", handled, code, stdout.String())
	}
}

func TestNonScribeArgumentsLeaveTUIPathUntouched(t *testing.T) {
	handled, code := runCLI([]string{"anything-else"}, &strings.Builder{}, &strings.Builder{})
	if handled || code != 0 {
		t.Fatalf("handled=%v code=%d", handled, code)
	}
}

func TestScribeCLIPassesThroughMarkdown(t *testing.T) {
	runner := &fakePathScribeRunner{result: scribe.Result{Markdown: "# Summary\n"}}
	var stdout, stderr strings.Builder
	handled, code := runCLIWithRunner(
		[]string{"scribe", "summarize", "note.md"},
		&stdout,
		&stderr,
		runner,
	)
	if !handled || code != 0 || stderr.Len() != 0 {
		t.Fatalf("handled=%v code=%d stderr=%q", handled, code, stderr.String())
	}
	if runner.operation != scribe.Summarize || runner.path != "note.md" {
		t.Fatalf("operation=%q path=%q", runner.operation, runner.path)
	}
	if stdout.String() != "# Summary\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestScribeCLIPreservesBackendExitCode(t *testing.T) {
	runner := &fakePathScribeRunner{err: &scribe.ProcessError{
		Code:       2,
		Diagnostic: "error: invalid input",
		Err:        errors.New("exit status 2"),
	}}
	var stdout, stderr strings.Builder
	handled, code := runCLIWithRunner(
		[]string{"scribe", "stats", "missing.md"},
		&stdout,
		&stderr,
		runner,
	)
	if !handled || code != 2 || stderr.String() != "error: invalid input\n" {
		t.Fatalf("handled=%v code=%d stderr=%q", handled, code, stderr.String())
	}
}
