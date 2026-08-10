// Package scribe invokes GhostScribe as GhostNote's shared analysis backend.
package scribe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Operation is a stable GhostScribe command name.
type Operation string

const (
	Summarize Operation = "summarize"
	Project   Operation = "project"
	Tasks     Operation = "tasks"
	Related   Operation = "related"
	Stats     Operation = "stats"
)

// ParseOperation accepts only the shared GhostScribe integration commands.
func ParseOperation(value string) (Operation, error) {
	operation := Operation(strings.ToLower(strings.TrimSpace(value)))
	switch operation {
	case Summarize, Project, Tasks, Related, Stats:
		return operation, nil
	default:
		return "", fmt.Errorf("unsupported scribe command %q", value)
	}
}

// SupportsText reports whether an operation can consume an in-memory note.
func (o Operation) SupportsText() bool {
	switch o {
	case Summarize, Tasks, Stats:
		return true
	default:
		return false
	}
}

// Result is successful human-facing Markdown from GhostScribe.
type Result struct {
	Operation Operation
	Source    string
	Markdown  string
}

// ProcessError preserves GhostScribe's stable exit code and stderr diagnostic.
type ProcessError struct {
	Code       int
	Diagnostic string
	Err        error
}

func (e *ProcessError) Error() string {
	if e.Diagnostic != "" {
		return e.Diagnostic
	}
	return e.Err.Error()
}

func (e *ProcessError) Unwrap() error { return e.Err }

// Runner invokes one configured GhostScribe executable without a shell.
type Runner struct {
	Executable string
	prefixArgs []string
}

// Default discovers GhostScribe through GHOSTSCRIBE_BIN, then PATH.
func Default() Runner {
	executable := strings.TrimSpace(os.Getenv("GHOSTSCRIBE_BIN"))
	if executable == "" {
		executable = "ghost-scribe"
	}
	return Runner{Executable: executable}
}

// RunPath invokes any stable operation on a filesystem path.
func (r Runner) RunPath(ctx context.Context, operation Operation, path string) (Result, error) {
	if _, err := ParseOperation(string(operation)); err != nil {
		return Result{}, err
	}
	path = strings.TrimSpace(path)
	if path == "" || path == "-" {
		return Result{}, errors.New("scribe path is required")
	}
	return r.run(ctx, operation, path, nil)
}

// RunText invokes a note-scoped operation using in-memory Markdown on stdin.
func (r Runner) RunText(ctx context.Context, operation Operation, markdown string) (Result, error) {
	if _, err := ParseOperation(string(operation)); err != nil {
		return Result{}, err
	}
	if !operation.SupportsText() {
		return Result{}, fmt.Errorf("%s requires a filesystem path", operation)
	}
	return r.run(ctx, operation, "-", strings.NewReader(markdown))
}

func (r Runner) run(ctx context.Context, operation Operation, source string, stdin io.Reader) (Result, error) {
	executable := strings.TrimSpace(r.Executable)
	if executable == "" {
		return Result{}, errors.New("GhostScribe executable is not configured")
	}
	args := append(append([]string(nil), r.prefixArgs...), string(operation), source)
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdin = stdin

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		code := 1
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			code = exitError.ExitCode()
		}
		return Result{}, &ProcessError{
			Code:       code,
			Diagnostic: strings.TrimSpace(stderr.String()),
			Err:        err,
		}
	}
	return Result{
		Operation: operation,
		Source:    source,
		Markdown:  stdout.String(),
	}, nil
}
