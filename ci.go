package main

import (
	"fmt"
	"os"
	"strings"
)

// CIMeta holds context for CI annotations.
type CIMeta struct {
	Command  string
	ExitCode int
}

// isCI returns true if running in a CI environment.
func isCI() bool {
	return os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != ""
}

// formatCIOutput returns a GitHub Actions workflow command string.
func formatCIOutput(analysis string, meta CIMeta) string {
	level := "error"
	title := fmt.Sprintf("Command failed: %s", meta.Command)
	if meta.ExitCode == 0 {
		level = "notice"
		title = "Notice from v"
	}
	escaped := escapeWorkflowCommand(analysis)
	return fmt.Sprintf("::%s title=%s::%s", level, title, escaped)
}

// escapeWorkflowCommand escapes newlines and carriage returns for GitHub workflow commands.
func escapeWorkflowCommand(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "%0D%0A")
	s = strings.ReplaceAll(s, "\n", "%0A")
	s = strings.ReplaceAll(s, "\r", "%0D")
	return s
}
