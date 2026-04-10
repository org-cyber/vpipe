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
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
)

// ==============================
// Version
// ==============================

const version = "2.1.0"

// ==============================
// Spinner
// ==============================

// startSpinner launches an animated terminal spinner in a goroutine.
// Call the returned stop() function to halt it and clear the line.
func startSpinner(label string) (stop func()) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	done := make(chan struct{})

	go func() {
		i := 0
		for {
			select {
			case <-done:
				fmt.Printf("\r\033[K")
				return
			default:
				color.New(color.FgHiBlue).Printf("\r%s %s ", frames[i%len(frames)], label)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()

	return func() {
		close(done)
		// Flush stdout to ensure spinner is visible before stopping
		fmt.Fprint(os.Stdout, "\r\033[K")
		os.Stdout.Sync()
		time.Sleep(90 * time.Millisecond)
	}
}

// ==============================
// Precompiled Regex Patterns
// ==============================

var sanitizePatterns = []*regexp.Regexp{
	// AWS Access Keys
	regexp.MustCompile(`(?i)\b(A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}\b`),
	// AWS Secret Keys
	regexp.MustCompile(`(?i)aws[_\-]secret[_\-]access[_\-]key["']?\s*[:=]\s*["']?[A-Za-z0-9/\+=]{40}["']?`),
	// Generic API keys / tokens / secrets
	regexp.MustCompile(`(?i)\b[A-Z0-9_]*(api[_\-]?key|token|secret|password|passwd|pwd)\b\s*[:=]\s*["']?[^\s"']+["']?`),
	// Emails
	regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
	// IP addresses (v4)
	regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`),
	// Unix absolute paths
	regexp.MustCompile(`(?:/home/\w+/[^\s\n\t"']+)+|(?:/Users/\w+/[^\s\n\t"']+)+`),
	// SSH / PEM Private keys
	regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
	// Passwords embedded in URLs
	regexp.MustCompile(`:\/\/[^:\/\s]*:[^@\/\s]*@`),
	// JWT tokens
	regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`),
}

// errorSignalPatterns are keywords used to score lines for smart truncation.
var errorSignalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(error|err|fatal|panic|fail(ed|ure)?|exception|traceback|abort|crash|undefined|cannot|could not|no such|permission denied|timeout|timed out|killed|segfault|nil pointer|stack overflow|out of memory|oom|syntax error|type error|runtime error|unhandled|unexpected|invalid|missing|not found|denied|refused)\b`),
}

// ==============================
// Supported AI Providers
// ==============================

type Provider string

const (
	ProviderGroq      Provider = "groq"
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
)

// ==============================
// Structs
// ==============================

// ChatMessage is a generic provider-agnostic message.
type ChatMessage struct {
	Role    string
	Content string
}

// Config holds runtime configuration resolved from flags + env.
type Config struct {
	Provider Provider
	APIKey   string
	Model    string
}

// ProviderConfig maps a provider to its defaults.
type ProviderConfig struct {
	DefaultModel string
	EnvKey       string
}

var providerDefaults = map[Provider]ProviderConfig{
	ProviderGroq:      {DefaultModel: "llama-3.3-70b-versatile", EnvKey: "GROQ_API_KEY"},
	ProviderOpenAI:    {DefaultModel: "gpt-4o-mini", EnvKey: "OPENAI_API_KEY"},
	ProviderAnthropic: {DefaultModel: "claude-haiku-4-5-20251001", EnvKey: "ANTHROPIC_API_KEY"},
}

// ==============================
// Groq / OpenAI compatible structs
// ==============================

type openAIRequest struct {
	Model       string      `json:"model"`
	Messages    []openAIMsg `json:"messages"`
	Temperature float64     `json:"temperature"`
	MaxTokens   int         `json:"max_tokens"`
}

type openAIMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// ==============================
// Anthropic structs
// ==============================

type anthropicRequest struct {
	Model     string         `json:"model"`
	MaxTokens int            `json:"max_tokens"`
	System    string         `json:"system"`
	Messages  []anthropicMsg `json:"messages"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type anthropicErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// ==============================
// Config loader
// ==============================
// ==============================
// Main
// ==============================

func main() {
	dryRun := flag.Bool("dry-run", false, "Show sanitized input without calling AI")
	debug := flag.Bool("debug", false, "Show raw inputs and outputs for debugging")
	timeout := flag.Int("timeout", 30, "Timeout for command execution in seconds")
	help := flag.Bool("help", false, "Show help")
	envFile := flag.String("env-file", "", "Path to .env file ")
	ciMode := flag.Bool("ci", false, "force GitHub Actions workflow command output")
	noCiMode := flag.Bool("no-ci", false, "force console output (disable CI auto-detection)")
	h := flag.Bool("h", false, "Show help")
	ver := flag.Bool("version", false, "Show version")
	provider := flag.String("provider", "groq", "AI provider: groq | openai | anthropic")
	model := flag.String("model", "", "Override the model (e.g. gpt-4o, claude-opus-4-6)")
	maxTokens := flag.Int("max-tokens", 600, "Maximum tokens in AI response")
	flag.Parse()

	if *ver {
		fmt.Printf("v %s\n", version)
		return
	}

	if *help || *h {
		showHelp()
		return
	}

	commandArgs := flag.Args()

	if len(commandArgs) > 0 {
		config, err := loadConfig(*provider, *model, *envFile)
		if err != nil {
			color.Red("❌ " + err.Error())
			os.Exit(1)
		}
		_ = executeCommand(commandArgs, *dryRun, *debug, *timeout, *maxTokens, config)
		return
	}

	config, err := loadConfig(*provider, *model, *envFile)
	if err != nil {
		color.Yellow("⚠️  " + err.Error())
		os.Exit(1)
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		showHelp()
		os.Exit(1)
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		color.Red(fmt.Sprintf("❌ Error reading input: %v", err))
		os.Exit(1)
	}

	if len(input) == 0 {
		color.Yellow("⚠️  Empty input received.")
		return
	}

	if *debug {
		color.HiBlack("── Raw Input ──────────────────────────")
		fmt.Println(string(input))
	}

	sanitizedInput := sanitizeInput(string(input))

	if *debug {
		color.HiBlack("── Sanitized Input ────────────────────")
		fmt.Println(sanitizedInput)
	}

	if *dryRun {
		color.Yellow("🔎 Dry Run — Sanitized Input:")
		color.Cyan(sanitizedInput)
		return
	}

	stop := startSpinner(fmt.Sprintf("Analyzing with %s / %s", config.Provider, config.Model))
	aiResponse, err := callAI(sanitizedInput, *maxTokens, config)
	stop()

	if err != nil {
		color.Red(fmt.Sprintf("❌ AI Error: %v", err))
		os.Exit(1)
	}

	args := []string{"(stdin)"}
	exitCode := 0
	if *ciMode || (isCI() && !*noCiMode) {
		meta := CIMeta{
			Command:  strings.Join(args, ""),
			ExitCode: exitCode,
		}
		color.Cyan(formatCIOutput(aiResponse, meta))

	} else {
		fmt.Println(color.CyanString("\n🤖 AI Analysis:"))
		printAnalysis(aiResponse)
	}

}

func loadConfig(providerFlag, modelFlag, envFile string) (*Config, error) {
	// Priority 1: Environment variables already set (CI/CD, explicit exports)
	// Skip godotenv if keys are already in environment
	if envFile != "" {
		_ = godotenv.Load(envFile)

	}
	// Priority 2: .env in current working directory (project-specific)
	_ = godotenv.Load()

	// Priority 3: .env in executable's directory (global install)
	// Only check if no keys found yet
	if !anyAPIKeySet() {
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			envPath := filepath.Join(exeDir, ".env")
			_ = godotenv.Load(envPath)
		}
	}

	// Priority 4: .env in user's home config directory (~/.config/vpipe/.env)
	if !anyAPIKeySet() {
		if home, err := os.UserHomeDir(); err == nil {
			configDir := filepath.Join(home, ".config", "vpipe")
			envPath := filepath.Join(configDir, ".env")
			_ = godotenv.Load(envPath)
		}
	}

	provider := Provider(strings.ToLower(providerFlag))
	defaults, ok := providerDefaults[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %q — choose: groq, openai, anthropic", providerFlag)
	}

	// Resolve API key (environment now has priority from all sources above)
	apiKey := os.Getenv(defaults.EnvKey)
	if apiKey == "" {
		return nil, fmt.Errorf("missing %s environment variable for provider %q (checked: current dir, vpipe dir, ~/.config/vpipe/.env, system env)", defaults.EnvKey, provider)
	}

	// Resolve model: flag > env > default
	model := modelFlag
	if model == "" {
		model = os.Getenv("V_MODEL")
	}
	if model == "" {
		model = defaults.DefaultModel
	}

	return &Config{
		Provider: provider,
		APIKey:   apiKey,
		Model:    model,
	}, nil
}

// Helper to check if any provider key is set
func anyAPIKeySet() bool {
	for _, p := range providerDefaults {
		if os.Getenv(p.EnvKey) != "" {
			return true
		}
	}
	return false
}

// ==============================
// Execute Command Mode
// ==============================

func executeCommand(args []string, dryRun, debug bool, timeout, maxTokens int, config *Config) error {
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
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	stdOutText := stdout.String()
	stdErrText := stderr.String()

	timedOut := ctx.Err() == context.DeadlineExceeded
	if timedOut {
		color.Yellow("⚠️  Command timed out")
	}

	// Only proceed if there is something to analyze
	if stdOutText == "" && stdErrText == "" && exitCode == 0 {
		color.Green("✅ Command succeeded with no output.")
		return nil
	}

	// Build a well-structured analysis payload separating stdout / stderr
	analysisInput := buildAnalysisPayload(strings.Join(args, " "), exitCode, timedOut, stdOutText, stdErrText)

	if debug {
		color.HiBlack("── Raw Analysis Payload ───────────────")
		fmt.Println(analysisInput)
	}

	sanitized := sanitizeInput(analysisInput)

	if debug {
		color.HiBlack("── Sanitized Payload ──────────────────")
		fmt.Println(sanitized)
	}

	if dryRun {
		color.Yellow("🔎 Dry Run — Sanitized Payload:")
		color.Cyan(sanitized)
		return nil
	}

	stop := startSpinner(fmt.Sprintf("Analyzing with %s / %s", config.Provider, config.Model))
	aiResponse, err := callAI(sanitized, maxTokens, config)
	stop()

	if err != nil {
		color.Red(fmt.Sprintf("❌ AI Error: %v", err))
		return err
	}

	printAnalysis(aiResponse)
	return nil
}

// buildAnalysisPayload formats the command context into a structured prompt section.
func buildAnalysisPayload(cmd string, exitCode int, timedOut bool, stdout, stderr string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Command: %s\n", cmd))
	sb.WriteString(fmt.Sprintf("Exit Code: %d\n", exitCode))
	sb.WriteString(fmt.Sprintf("Timed Out: %v\n", timedOut))

	if stdout != "" {
		sb.WriteString("\n--- STDOUT ---\n")
		sb.WriteString(stdout)
	}

	if stderr != "" {
		sb.WriteString("\n--- STDERR ---\n")
		sb.WriteString(stderr)
	}

	return sb.String()
}

// ==============================
// Output Formatting
// ==============================

func printAnalysis(response string) {
	fmt.Println()
	color.HiGreen("━━━━━━━━━━ AI Analysis ━━━━━━━━━━")
	color.White(response)
	color.HiGreen("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	color.Magenta("⚠️  Always verify suggestions before applying.")
	fmt.Println()
}

// ==============================
// Smart Sanitization & Truncation
// ==============================

func preserveFilenamePaths(input string) string {
	re := regexp.MustCompile(`([A-Za-z]:\\(?:[^\\/:*?"<>|\r\n]+\\)*([^\\/:*?"<>|\r\n]+):\d+(?::\d+)?)`)

	return re.ReplaceAllStringFunc(input, func(match string) string {
		parts := strings.Split(match, `\`)
		if len(parts) == 0 {
			return match
		}
		return parts[len(parts)-1]
	})
}

// sanitizeInput removes sensitive data and intelligently truncates long inputs,
// prioritising lines that contain error signals.
func sanitizeInput(input string) string {
	// Step 1: Redact secrets
	result := preserveFilenamePaths(input)

	for _, pattern := range sanitizePatterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}

	// Redact OS username and hostname
	if username := os.Getenv("USERNAME"); username != "" {
		p := regexp.MustCompile(`\b` + regexp.QuoteMeta(username) + `\b`)
		result = p.ReplaceAllString(result, "[USER]")
	}
	if computer := os.Getenv("COMPUTERNAME"); computer != "" {
		p := regexp.MustCompile(`\b` + regexp.QuoteMeta(computer) + `\b`)
		result = p.ReplaceAllString(result, "[MACHINE]")
	}

	// Step 2: Smart truncation — keep high-signal lines
	const maxChars = 6000
	if len(result) > maxChars {
		result = smartTruncate(result, maxChars)
	}

	return result
}

// smartTruncate scores each line by its error-signal density and keeps the
// highest-scoring lines up to maxChars, preserving their original order.
func smartTruncate(input string, maxChars int) string {
	lines := strings.Split(input, "\n")

	type scoredLine struct {
		index int
		text  string
		score int
	}

	scored := make([]scoredLine, len(lines))
	for i, line := range lines {
		s := 0
		for _, p := range errorSignalPatterns {
			matches := p.FindAllString(line, -1)
			s += len(matches)
		}
		// Bonus for short lines (stack frames / error summaries are usually short)
		if len(line) < 120 {
			s++
		}
		scored[i] = scoredLine{index: i, text: line, score: s}
	}

	// Sort by score descending (stable)
	sort.SliceStable(scored, func(a, b int) bool {
		return scored[a].score > scored[b].score
	})

	// Greedily pick lines until we approach maxChars
	selected := make([]bool, len(lines))
	total := 0
	for _, sl := range scored {
		lineLen := len(sl.text) + 1 // +1 for newline
		if total+lineLen > maxChars {
			break
		}
		selected[sl.index] = true
		total += lineLen
	}

	// Reconstruct in original order
	var sb strings.Builder
	skipping := false
	for i, line := range lines {
		if selected[i] {
			if skipping {
				sb.WriteString("... [lines omitted] ...\n")
				skipping = false
			}
			sb.WriteString(line)
			sb.WriteString("\n")
		} else {
			skipping = true
		}
	}

	if skipping {
		sb.WriteString("... [lines omitted] ...\n")
	}

	return strings.TrimSpace(sb.String())
}

// ==============================
// AI Dispatch
// ==============================

const systemPrompt = `
You are a senior software engineer and debugging expert.

Analyze the command output using evidence from the error message and stack trace.

Reasoning priority (strict order):
1. Identify the exact immediate error message or exception type.
2. Identify the top-most stack frame or file/line number where the failure originates.
3. Explain the most direct technical cause shown by the trace.
4. Only infer secondary causes (configuration, dependencies, environment variables, permissions) if the trace explicitly supports them.

Never speculate beyond the evidence in the log.
Do not assume missing environment variables, authentication issues, or configuration problems unless directly shown.

Respond in exactly this structured format:

Root cause:
- One concise sentence explaining the direct failure.

Why it happened:
- Brief technical explanation based strictly on the stack trace and stderr.

Suggested fix:
- Step 1: ...
- Step 2: ...
- Step 3: ... (add more steps if necessary)

Next command to try:
- The single most useful command the developer should run next.

Rules:
- Be direct, specific, and actionable.
- Focus on the immediate failure source first.
- Prefer file names, line numbers, and exact exception types.
- If STDERR and STDOUT are both provided, prioritize STDERR.
`

func callAI(errorLog string, maxTokens int, cfg *Config) (string, error) {
	messages := []ChatMessage{
		{Role: "user", Content: "Please analyze this error and suggest a fix:\n\n" + errorLog},
	}

	switch cfg.Provider {
	case ProviderGroq, ProviderOpenAI:
		return callOpenAICompatible(messages, maxTokens, cfg)
	case ProviderAnthropic:
		return callAnthropic(messages, maxTokens, cfg)
	default:
		return "", fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}

// ==============================
// OpenAI-compatible (Groq + OpenAI)
// ==============================

func callOpenAICompatible(messages []ChatMessage, maxTokens int, cfg *Config) (string, error) {
	urls := map[Provider]string{
		ProviderGroq:   "https://api.groq.com/openai/v1/chat/completions",
		ProviderOpenAI: "https://api.openai.com/v1/chat/completions",
	}

	apiURL := urls[cfg.Provider]

	msgs := []openAIMsg{{Role: "system", Content: systemPrompt}}
	for _, m := range messages {
		msgs = append(msgs, openAIMsg{Role: m.Role, Content: m.Content})
	}

	reqBody := openAIRequest{
		Model:       cfg.Model,
		Temperature: 0.2,
		MaxTokens:   maxTokens,
		Messages:    msgs,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	body, statusCode, err := doHTTPRequest(apiURL, "Bearer "+cfg.APIKey, jsonData)
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK {
		var apiErr openAIErrorResponse
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("API error (%d): %s", statusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("API error (%d): %s", statusCode, string(body))
	}

	var resp openAIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty AI response")
	}

	return resp.Choices[0].Message.Content, nil
}

// ==============================
// Anthropic
// ==============================

func callAnthropic(messages []ChatMessage, maxTokens int, cfg *Config) (string, error) {
	apiURL := "https://api.anthropic.com/v1/messages"

	msgs := make([]anthropicMsg, len(messages))
	for i, m := range messages {
		msgs[i] = anthropicMsg{Role: m.Role, Content: m.Content}
	}

	reqBody := anthropicRequest{
		Model:     cfg.Model,
		MaxTokens: maxTokens,
		System:    systemPrompt,
		Messages:  msgs,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	var httpResp *http.Response
	var reqErr error

	for attempt := 1; attempt <= 3; attempt++ {
		httpResp, reqErr = client.Do(req)
		if reqErr == nil {
			break
		}
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	if reqErr != nil {
		return "", fmt.Errorf("request failed after retries: %w", reqErr)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", err
	}

	if httpResp.StatusCode != http.StatusOK {
		var apiErr anthropicErrorResponse
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("API error (%d): %s", httpResp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("API error (%d): %s", httpResp.StatusCode, string(body))
	}

	var resp anthropicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("empty AI response")
}

// ==============================
// HTTP helper (shared retry logic)
// ==============================

func doHTTPRequest(apiURL, authHeader string, jsonData []byte) ([]byte, int, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	var (
		resp   *http.Response
		reqErr error
	)

	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("Content-Type", "application/json")

		resp, reqErr = client.Do(req)
		if reqErr == nil {
			break
		}
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	if reqErr != nil {
		return nil, 0, fmt.Errorf("request failed after retries: %w", reqErr)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return body, resp.StatusCode, nil
}

// ==============================
// Unit-testable sanitize helpers
// ==============================

// HasErrorSignal returns true if the line contains at least one error keyword.
func HasErrorSignal(line string) bool {
	for _, p := range errorSignalPatterns {
		if p.MatchString(line) {
			return true
		}
	}
	return false
}

// ==============================
// Help
// ==============================

func showHelp() {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)

	bold.Println("v — AI Build Error Analyzer")
	fmt.Println("Version:", version)
	fmt.Println()
	bold.Println("Usage:")
	cyan.Println("  v [flags] [command [args...]]   # run a command and analyze output")
	cyan.Println("  <command> 2>&1 | v [flags]      # pipe error output into v")
	fmt.Println()
	bold.Println("Flags:")
	fmt.Println("  --provider string    AI provider: groq | openai | anthropic  (default: groq)")
	fmt.Println("  --model string       Override model (env: V_MODEL)")
	fmt.Println("  --max-tokens int     Max tokens in AI response               (default: 600)")
	fmt.Println("  --timeout int        Command execution timeout in seconds     (default: 30)")
	fmt.Println("  --dry-run            Show sanitized input without calling AI")
	fmt.Println("  --debug              Show raw + sanitized payloads")
	fmt.Println("  --version            Print version and exit")
	fmt.Println("  --help               Show this help")
	fmt.Println()
	bold.Println("Environment Variables:")
	fmt.Println("  GROQ_API_KEY         Required for --provider groq")
	fmt.Println("  OPENAI_API_KEY       Required for --provider openai")
	fmt.Println("  ANTHROPIC_API_KEY    Required for --provider anthropic")
	fmt.Println("  V_MODEL              Default model override (lowest priority)")
	fmt.Println()
	bold.Println("Examples:")
	cyan.Println("  v go build ./...")
	cyan.Println("  cargo build 2>&1 | v")
	cyan.Println("  npm run build 2>&1 | v --provider openai --model gpt-4o")
	cyan.Println("  make 2>&1 | v --provider anthropic --max-tokens 800")
	cyan.Println("  v --dry-run go test ./...")
	fmt.Println()
}
