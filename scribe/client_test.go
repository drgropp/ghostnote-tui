package scribe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func helperRunner(t *testing.T, mode string) Runner {
	t.Helper()
	t.Setenv("GHOSTNOTE_SCRIBE_HELPER", mode)
	return Runner{
		Executable: os.Args[0],
		prefixArgs: []string{"-test.run=TestScribeHelperProcess", "--"},
	}
}

func TestRunPathReturnsMarkdown(t *testing.T) {
	result, err := helperRunner(t, "success").RunPath(context.Background(), Stats, "note.md")
	if err != nil {
		t.Fatal(err)
	}
	if result.Operation != Stats || result.Source != "note.md" {
		t.Fatalf("result metadata = %+v", result)
	}
	if !strings.Contains(result.Markdown, "stats note.md") {
		t.Fatalf("markdown = %q", result.Markdown)
	}
}

func TestRunTextUsesStandardInput(t *testing.T) {
	result, err := helperRunner(t, "stdin").RunText(context.Background(), Summarize, "# Current note")
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != "-" || !strings.Contains(result.Markdown, "# Current note") {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunPreservesExitCodeAndDiagnostic(t *testing.T) {
	_, err := helperRunner(t, "failure").RunPath(context.Background(), Project, "notes")
	var processError *ProcessError
	if !errors.As(err, &processError) {
		t.Fatalf("error = %T %v", err, err)
	}
	if processError.Code != 2 || processError.Diagnostic != "error: invalid project" {
		t.Fatalf("process error = %+v", processError)
	}
}

func TestTextRejectsDirectoryCommands(t *testing.T) {
	for _, operation := range []Operation{Project, Related} {
		if _, err := helperRunner(t, "success").RunText(context.Background(), operation, "note"); err == nil {
			t.Fatalf("%s accepted text input", operation)
		}
	}
}

func TestScribeHelperProcess(t *testing.T) {
	mode := os.Getenv("GHOSTNOTE_SCRIBE_HELPER")
	if mode == "" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) > 0 {
		args = args[1:]
	}
	switch mode {
	case "success":
		fmt.Printf("# Result\n\n%s\n", strings.Join(args, " "))
	case "stdin":
		content, _ := io.ReadAll(os.Stdin)
		fmt.Printf("# Result\n\n%s: %s\n", strings.Join(args, " "), content)
	case "failure":
		fmt.Fprintln(os.Stderr, "error: invalid project")
		os.Exit(2)
	}
	os.Exit(0)
}
