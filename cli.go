package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/drgropp/ghostnote-tui/scribe"
)

const scribeCLIUsage = `usage: ghostnote-tui scribe <summarize|project|tasks|related|stats> <note-or-directory>`

func runCLI(args []string, stdout, stderr io.Writer) (bool, int) {
	return runCLIWithRunner(args, stdout, stderr, scribe.Default())
}

type pathScribeRunner interface {
	RunPath(context.Context, scribe.Operation, string) (scribe.Result, error)
}

func runCLIWithRunner(args []string, stdout, stderr io.Writer, runner pathScribeRunner) (bool, int) {
	if len(args) == 0 || strings.ToLower(args[0]) != "scribe" {
		return false, 0
	}
	if len(args) == 2 && (args[1] == "-h" || args[1] == "--help" || args[1] == "help") {
		fmt.Fprintln(stdout, scribeCLIUsage)
		return true, 0
	}
	if len(args) != 3 {
		fmt.Fprintln(stderr, "error:", scribeCLIUsage)
		return true, 2
	}
	operation, err := scribe.ParseOperation(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return true, 2
	}
	result, err := runner.RunPath(context.Background(), operation, args[2])
	if err != nil {
		code := 1
		var processError *scribe.ProcessError
		if errors.As(err, &processError) {
			code = processError.Code
		}
		fmt.Fprintln(stderr, formatScribeError(err))
		return true, code
	}
	fmt.Fprint(stdout, result.Markdown)
	return true, 0
}

func formatScribeError(err error) string {
	message := strings.TrimSpace(err.Error())
	if strings.HasPrefix(message, "error:") {
		return message
	}
	return "error: " + message
}
