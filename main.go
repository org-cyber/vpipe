package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"

	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
)

// ==============================
// Precompiled Regex Patterns
// ==============================

var sanitizePatterns = []*regexp.Regexp{
	// AWS Keys
	regexp.MustCompile(`(?i)\b(A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}\b`),

	// AWS Secret Keys
	regexp.MustCompile(`(?i)aws[_\-]secret[_\-]access[_\-]key["']?\s*[:=]\s*["']?[A-Za-z0-9/\+=]{40}["']?`),

	// Generic API keys
	regexp.MustCompile(`(?i)(api[_\-]key|token|secret)["']?\s*[:=]\s*["']?[A-Za-z0-9_\-]{32,}["']?`),

	// Emails
	regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),

	// IP addresses
	regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`),

	// Windows paths
	regexp.MustCompile(`[A-Za-z]:\\(?:[^\\/:*?"<>|\r\n]+\\)*[^\\/:*?"<>|\r\n]*`),

	// Unix paths (more conservative)
	regexp.MustCompile(`(?:/[^ \n\t]+)+`),

	// SSH Private keys
	regexp.MustCompile(`(?s)-----BEGIN (RSA )?PRIVATE KEY-----(.*?)-----END (RSA )?PRIVATE KEY-----`),

	// Password in URL
	regexp.MustCompile(`:\/\/[^:\/]*:[^@\/]*@`),
}

// ==============================
// Structs
// ==============================

type GroqRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type GroqErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type Config struct {
	APIKey string
}

// ==============================
// Main
// ==============================
func loadConfig() (*Config, error) {
	_ = godotenv.Load()

	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY is missing")
	}

	return &Config{
		APIKey: apiKey,
	}, nil
}

func main() {
	dryRun := flag.Bool("dry-run", false, "Show sanitized input without calling AI")
	debug := flag.Bool("debug", false, "Show raw inputs and outputs for debugging")
	timeout := flag.Int("timeout", 30, "Timeout for command execution in seconds")
	help := flag.Bool("help", false, "Show help")
	h := flag.Bool("h", false, "Show help")
	flag.Parse()

	if *help || *h {
		showHelp()
		return
	}

	commandArgs := flag.Args()

	if len(commandArgs) > 0 {
		config, err := loadConfig()
		if err != nil {
			color.Red("❌ " + err.Error())
			os.Exit(1)
		}

		err = executeCommand(commandArgs, *dryRun, *timeout, config)
		return
	}

	config, err := loadConfig()
	if err != nil {
		color.Yellow("⚠️  No .env file found. Ensure GROQ_API_KEY is set.")
		os.Exit(1)
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		showHelp()
		os.Exit(1)
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		color.Red(fmt.Sprintf("Error reading input: %v", err))
		os.Exit(1)
	}

	if len(input) == 0 {
		color.Yellow("⚠️  Empty input received.")
		return
	}

	sanitizedInput := sanitizeInput(string(input))

	if *debug {
		fmt.Println("Raw Input:", string(input))
		fmt.Println("Sanitized Input:", sanitizedInput)
	}

	if *dryRun {
		color.Yellow("Dry Run Mode:")
		color.Cyan(sanitizedInput)
		return
	}

	color.HiBlue("🔍 Analyzing error...")

	aiResponse, err := callGroqAPI(sanitizedInput, config.APIKey)
	if err != nil {
		color.Red(fmt.Sprintf("❌ AI Error: %v", err))
		os.Exit(1)
	}

	fmt.Println()
color.HiGreen("━━━━━━━━━━ AI Analysis ━━━━━━━━━━")
color.White(aiResponse)
color.HiBlack("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
fmt.Println()
	fmt.Println()
	color.Magenta("⚠️  Always verify suggestions before applying!")
	color.HiBlack("─────────────────────────────────────────")
}

// ==============================
// Execute Command Mode
// ==============================

func executeCommand(args []string, dryRun bool, timeout int, config *Config) error {
	color.Cyan(fmt.Sprintf("🔧 Running: %s", strings.Join(args, " ")))

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); //checks if the error came from the command process itself
		ok {
			exitCode = exitErr.ExitCode() //extracts the real exit code e.g 0 = sucess, 1 = generic failure etc.
		} else {
			exitCode = -1
		}
	}

	stdOutText := stdout.String()
	stdErrText := stderr.String()

	timedOut := ctx.Err() == context.DeadlineExceeded
	if timedOut {
		color.Yellow("⚠️ Command timed out")
	}

	if stdOutText == "" && stdErrText == "" {
		return err
	}

	analysisInput := fmt.Sprintf(
		"Command: %s\nExit Code: %d\nTimed Out: %v\n\nSTDOUT:\n%s\n\nSTDERR:\n%s",
		strings.Join(args, " "),
		exitCode,
		ctx.Err() == context.DeadlineExceeded,
		stdOutText,
		stdErrText,
	)

	sanitized := sanitizeInput(analysisInput)

	if dryRun {
		color.Yellow("Dry Run Mode:")
		color.Cyan(sanitized)
		return nil
	}

	color.HiBlue("🔍 Analyzing error...")

	aiResponse, err := callGroqAPI(sanitized, config.APIKey)
	if err != nil {
		color.Red(fmt.Sprintf("❌ AI Error: %v", err))
		return err
	}

	fmt.Println()
	color.Green("💡 Suggested Fix:")
	color.White(aiResponse)
	color.HiBlack("─────────────────────────────────────────")

	return nil
}

// ==============================
// Sanitization
// ==============================

func sanitizeInput(input string) string {
	const maxLen = 5000
	const chunkSize = 2500

	if len(input) > maxLen {
		start := input[:chunkSize]
		end := input[len(input)-chunkSize:]

		input = start +
			"\n\n... [middle truncated for brevity] ...\n\n" +
			end
	}
	result := input

	for _, pattern := range sanitizePatterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}

	if username := os.Getenv("USERNAME"); username != "" {
		userPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(username) + `\b`)
		result = userPattern.ReplaceAllString(result, "[USER]")
	}

	if computer := os.Getenv("COMPUTERNAME"); computer != "" {
		machinePattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(computer) + `\b`)
		result = machinePattern.ReplaceAllString(result, "[MACHINE]")
	}

	return result
}

// ==============================
// Groq API Call
// ==============================

func callGroqAPI(errorLog string, apiKey string) (string, error) {
	url := "https://api.groq.com/openai/v1/chat/completions"

	reqBody := GroqRequest{
		Model:       "llama-3.1-8b-instant",
		Temperature: 0.2,
		MaxTokens:   220,
		Messages: []Message{
			{
				Role: "system",
				Content: `You are a senior software engineer and debugging expert.

Analyze the provided command output carefully.

Respond in exactly this format:

Root cause:
- short explanation of the main issue

Why it happened:
- likely technical reason

Suggested fix:
- 2 to 4 clear actionable steps

Next command to try:
- one command the developer should run next

Keep the response under 180 words.
Be direct and practical.`,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf(
				"Please analyze this command failure and suggest a fix:\n\n%s",
				errorLog,
			),
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	var resp *http.Response
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err = client.Do(req)
		if err == nil {
			break
		}

		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	if err != nil {
		return "", fmt.Errorf("request failed after retries: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	//  IMPORTANT FIX
	if resp.StatusCode != http.StatusOK {
		var apiErr GroqErrorResponse

		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, apiErr.Error.Message)
		}

		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var groqResp GroqResponse
	err = json.Unmarshal(body, &groqResp)
	if err != nil {
		return "", err
	}

	if len(groqResp.Choices) == 0 {
		return "", fmt.Errorf("empty AI response")
	}

	return groqResp.Choices[0].Message.Content, nil
}

// ==============================
// Help
// ==============================

func showHelp() {
	fmt.Println("AI Build Error Analyzer")
	fmt.Println("=======================")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  v [flags] [command]")
	fmt.Println("  input | v")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --dry-run")
	fmt.Println("  --debug")
	fmt.Println("  --timeout N")
	fmt.Println("  --help")
}
