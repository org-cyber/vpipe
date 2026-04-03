package main

import (
	"strings"
	"testing"
)

// ==============================
// sanitizeInput Tests
// ==============================

func TestSanitizeInput_RedactsAWSKey(t *testing.T) {
	input := "export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"
	out := sanitizeInput(input)
	if strings.Contains(out, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("expected AWS key to be redacted, got: %s", out)
	}
}

func TestSanitizeInput_RedactsEmail(t *testing.T) {
	input := "contact: admin@example.com for support"
	out := sanitizeInput(input)
	if strings.Contains(out, "admin@example.com") {
		t.Errorf("expected email to be redacted, got: %s", out)
	}
}

func TestSanitizeInput_RedactsIPAddress(t *testing.T) {
	input := "Connecting to server at 192.168.1.100"
	out := sanitizeInput(input)
	if strings.Contains(out, "192.168.1.100") {
		t.Errorf("expected IP address to be redacted, got: %s", out)
	}
}

func TestSanitizeInput_RedactsPasswordInURL(t *testing.T) {
	input := "DB_URL=postgres://admin:supersecret@localhost/mydb"
	out := sanitizeInput(input)
	if strings.Contains(out, "supersecret") {
		t.Errorf("expected password in URL to be redacted, got: %s", out)
	}
}

func TestSanitizeInput_RedactsJWT(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyMTIzIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	out := sanitizeInput(jwt)
	if strings.Contains(out, jwt) {
		t.Errorf("expected JWT to be redacted, got: %s", out)
	}
}

func TestSanitizeInput_PreservesNormalText(t *testing.T) {
	input := "Build failed: cannot find package github.com/fatih/color"
	out := sanitizeInput(input)
	if !strings.Contains(out, "Build failed") {
		t.Errorf("expected normal text to be preserved, got: %s", out)
	}
}

// ==============================
// smartTruncate Tests
// ==============================

func TestSmartTruncate_ReturnsInputUnderLimit(t *testing.T) {
	input := "error: something went wrong\nfailed to compile"
	out := smartTruncate(input, 6000)
	if !strings.Contains(out, "error") {
		t.Errorf("expected error lines to be preserved")
	}
}

func TestSmartTruncate_PrefersErrorLines(t *testing.T) {
	// Build a long input where the error line is in the middle
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "info: this is a normal log line with lots of padding text here")
	}
	lines[100] = "fatal: failed to connect to database — connection refused"

	input := strings.Join(lines, "\n")
	out := smartTruncate(input, 2000)

	if !strings.Contains(out, "fatal: failed to connect") {
		t.Errorf("expected high-signal error line to survive truncation, got output length %d", len(out))
	}
}

func TestSmartTruncate_OutputUnderLimit(t *testing.T) {
	var lines []string
	for i := 0; i < 500; i++ {
		lines = append(lines, "error: something failed here badly and this is a long description of the problem")
	}
	input := strings.Join(lines, "\n")
	out := smartTruncate(input, 3000)
	if len(out) > 3200 { // small overhead for omission markers
		t.Errorf("expected output under limit, got %d chars", len(out))
	}
}

// ==============================
// HasErrorSignal Tests
// ==============================

func TestHasErrorSignal_DetectsError(t *testing.T) {
	cases := []string{
		"error: file not found",
		"FATAL: nil pointer dereference",
		"Exception in thread main",
		"Build FAILED",
		"permission denied",
		"connection refused",
		"timed out",
	}
	for _, c := range cases {
		if !HasErrorSignal(c) {
			t.Errorf("expected error signal in: %q", c)
		}
	}
}

func TestHasErrorSignal_IgnoresNormalLines(t *testing.T) {
	cases := []string{
		"Compiling main.go",
		"Running tests...",
		"[INFO] Build started",
		"ok  github.com/user/repo  0.123s",
	}
	for _, c := range cases {
		if HasErrorSignal(c) {
			t.Errorf("did not expect error signal in: %q", c)
		}
	}
}

// ==============================
// loadConfig Tests
// ==============================

func TestLoadConfig_UnknownProvider(t *testing.T) {
	_, err := loadConfig("fakeprovider", "")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestBuildAnalysisPayload_ContainsAllFields(t *testing.T) {
	payload := buildAnalysisPayload("go build ./...", 1, false, "some stdout", "some stderr")
	for _, want := range []string{"Command:", "Exit Code:", "STDOUT", "STDERR", "some stdout", "some stderr"} {
		if !strings.Contains(payload, want) {
			t.Errorf("expected payload to contain %q", want)
		}
	}
}

func TestBuildAnalysisPayload_NoStdout(t *testing.T) {
	payload := buildAnalysisPayload("make build", 2, false, "", "Makefile:10: *** missing separator.  Stop.")
	if strings.Contains(payload, "STDOUT") {
		t.Error("expected no STDOUT section when stdout is empty")
	}
	if !strings.Contains(payload, "STDERR") {
		t.Error("expected STDERR section in payload")
	}
}
