# v — AI Build Error Analyzer

<p align="center">
  <img src="gold.png" alt="v logo" width="160" />
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-2.1.0-blue" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.25.6+-00ADD8?style=flat&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-green" alt="License">
</p>

**v** is a command-line tool that uses AI to analyze build errors and provide actionable debugging suggestions. It automatically captures command output, sanitizes sensitive data, and sends it to an AI provider for intelligent analysis.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
  - [From Source](#from-source)
  - [Pre-built Binaries](#pre-built-binaries)
- [Configuration](#configuration)
  - [Environment Variables](#environment-variables)
  - [Supported AI Providers](#supported-ai-providers)
  - [.env File](#env-file)
- [Usage](#usage)
  - [Command Execution Mode](#command-execution-mode)
  - [Piped Input Mode](#piped-input-mode)
  - [Flag Placement](#flag-placement)
- [Examples](#examples)
  - [Basic Examples](#basic-examples)
  - [Provider Examples](#provider-examples)
  - [Advanced Examples](#advanced-examples)
- [Command-Line Flags](#command-line-flags)
- [Sanitization](#sanitization)
- [Smart Truncation](#smart-truncation)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

---

## Features

- **Multi-Provider Support**: Works with Groq, OpenAI, and Anthropic AI providers
- **Dual Input Modes**: Execute commands directly or pipe output for analysis
- **Automatic Sanitization**: Redacts 20+ patterns including API keys, passwords, IPs, emails
- **Smart Truncation**: Intelligently preserves error-prone lines when input exceeds token limits
- **Dry Run Mode**: Preview sanitized input without making API calls
- **Debug Mode**: Inspect raw and sanitized payloads for troubleshooting
- **Configurable Timeout**: Prevent hung processes with customizable timeouts

---

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/your-repo/v.git
cd v

# Build the executable
# Windows
go build -o v.exe

# Linux/macOS
go build -o v
```

### Pre-built Binaries (Recommended)

Download the correct binary for your platform from the `release/` directory or GitHub Releases:

| OS | Architecture | Binary |
|-----|--------------|--------|
| Windows | x64 | `v-windows-amd64.exe` |
| Linux | x64 | `v-linux-amd64` |
| macOS | ARM64 (Apple Silicon) | `v-darwin-arm64` |
| macOS | x64 | `v-darwin-amd64` |

#### macOS / Linux

```bash
chmod +x release/v-linux-amd64
sudo mv release/v-linux-amd64 /usr/local/bin/v
```

If you prefer a user-local install:

```bash
mkdir -p "$HOME/bin"
mv release/v-linux-amd64 "$HOME/bin/v"
printf '%s\n' 'export PATH="$HOME/bin:$PATH"' 'export PATH="$PATH:$HOME/bin"' >> ~/.bashrc
source ~/.bashrc
```

> If the binary is not named `v`, you can also rename it before moving it:
> `mv release/v-linux-amd64 /usr/local/bin/v`

#### Windows

1. Download `v-windows-amd64.exe`.
2. Move it to a folder on your `PATH`, such as `C:\tools` or `C:\Program Files\v`.
3. If needed, add the folder to `PATH`:

```powershell
setx PATH "$env:PATH;C:\tools"
```

4. Open a new terminal and run `v.exe`.

> If you want the command to be exactly `v`, rename the file to `v.exe` before moving it.

### Setup

Before using the tool, set the API key for your chosen provider:

```bash
# For Groq
export GROQ_API_KEY="your_groq_api_key"

# For OpenAI
export OPENAI_API_KEY="your_openai_api_key"

# For Anthropic
export ANTHROPIC_API_KEY="your_anthropic_api_key"
```

Optionally set a default model:

```bash
export V_MODEL="gpt-4o-mini"
```

On Windows PowerShell:

```powershell
setx GROQ_API_KEY "your_groq_api_key"
setx V_MODEL "gpt-4o-mini"
```

### Usage

Run a command directly through `v`:

```bash
v go build ./...
v npm run build
```

Pipe build or test output into `v`:

```bash
go test ./... 2>&1 | v
cat build.log | v
```

Common flags:

```bash
v --version
v --dry-run go test ./...
v --provider openai --model gpt-4o-mini npm run build
```

### Add to PATH for global access

On macOS/Linux, install into a directory already on your `PATH` like `/usr/local/bin` or add your custom bin directory to `PATH`.

On Windows, place `v.exe` in a directory already on `PATH`, or add the install directory to `PATH` and reopen your terminal.

This makes `v` available as a global CLI command from any folder.

---

## Configuration

### Environment Variables

| Variable | Required | Description | Default Model |
|----------|----------|-------------|---------------|
| `GROQ_API_KEY` | For Groq provider | API key from [Groq](https://console.groq.com/) | llama-3.3-70b-versatile |
| `OPENAI_API_KEY` | For OpenAI provider | API key from [OpenAI](https://platform.openai.com/) | gpt-4o-mini |
| `ANTHROPIC_API_KEY` | For Anthropic provider | API key from [Anthropic](https://console.anthropic.com/) | claude-haiku-4-5-20251001 |
| `V_MODEL` | Optional | Override default model (lowest priority) | Provider-specific |

### Supported AI Providers

**Groq** (Default)
- Fast inference with free tier available
- Get API key: https://console.groq.com/

**OpenAI**
- GPT-4o, GPT-4o-mini, and other models
- Get API key: https://platform.openai.com/api-keys

**Anthropic**
- Claude models (Haiku, Sonnet, Opus)
- Get API key: https://console.anthropic.com/

### .env File

v supports multiple ways to configure environment variables with a priority system. Create `.env` files in any of these locations:

#### Priority Order (Highest to Lowest):

1. **Explicit file** via `--env-file` flag
2. **Current working directory** (project-specific)
3. **Executable directory** (global install)
4. **User config directory** (global access)

#### Option 1: Project-Specific (.env in current directory)

Create a `.env` file in your project root:

```bash
# Choose one or more providers
GROQ_API_KEY=gsk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
OPENAI_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ANTHROPIC_API_KEY=sk-ant-api03-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
V_MODEL=gpt-4o-mini  # Optional model override
```

#### Option 2: Global Access (Recommended for CLI usage)

For system-wide access from any directory, create a global config:

**Windows (PowerShell):**
```powershell
# Create the config directory
mkdir -p "$env:USERPROFILE\.config\vpipe"

# Create and edit the .env file
notepad "$env:USERPROFILE\.config\vpipe\.env"
```

**Linux/macOS:**
```bash
# Create the config directory
mkdir -p "$HOME/.config/vpipe"

# Create and edit the .env file
nano "$HOME/.config/vpipe/.env"
# or
code "$HOME/.config/vpipe/.env"
```

**Add your API keys to the global .env file:**
```bash
GROQ_API_KEY=your_groq_api_key_here
OPENAI_API_KEY=your_openai_api_key_here
ANTHROPIC_API_KEY=your_anthropic_api_key_here
V_MODEL=optional_model_override
```

**Benefits of global config:**
- ✅ Works from any directory
- ✅ Single setup for all projects
- ✅ Lower priority than local configs (can be overridden per-project)
- ✅ Secure (only accessible by your user account)

#### Option 3: Explicit File Path

Use the `--env-file` flag to specify any `.env` file location:

```bash
v --env-file /path/to/custom/.env go build ./...
v --env-file ./config/prod.env npm run build
```

#### Option 4: System Environment Variables

Set environment variables directly in your shell (highest precedence except explicit --env-file):

```bash
# Linux/macOS
export GROQ_API_KEY="your_key"
export V_MODEL="gpt-4o"

# Windows PowerShell
$env:GROQ_API_KEY = "your_key"
$env:V_MODEL = "gpt-4o"

# Windows Command Prompt
set GROQ_API_KEY=your_key
set V_MODEL=gpt-4o
```

**Note:** System environment variables take precedence over all `.env` files except when using `--env-file`.

---

## Usage

### Command Execution Mode

Run a command directly through v. It executes the command, captures stdout/stderr, and analyzes the output:

```bash
v <command> [args...]
```

**Example:**
```bash
v go build ./...
v npm run build
v cargo build 2>&1
```

### Piped Input Mode

Pipe output from any command into v for analysis:

```bash
<command> | v
<command> 2>&1 | v
cat error.log | v
```

**Example:**
```bash
npm run build 2>&1 | v
go test ./... 2>&1 | v
docker build . 2>&1 | v
```

### Flag Placement

v flags must come **before** the command. Use `--` to separate v flags from command arguments:

```bash
# v flag before command
v --timeout 60 npm run build

# Separate v flags from command args using --
v --dry-run -- npm run build --verbose
```

---

## Examples

### Basic Examples

**Analyze a failed Go build:**
```bash
v go build ./...
```

**Analyze npm build errors:**
```bash
v npm run build
```

**Analyze cargo errors:**
```bash
v cargo build
```

**Pipe existing error log:**
```bash
cat build_errors.log | v
```

**Analyze make output:**
```bash
make 2>&1 | v
```

### Provider Examples

**Use OpenAI instead of Groq:**
```bash
v --provider openai go build ./...
```

**Use Anthropic with custom model:**
```bash
v --provider anthropic --model claude-sonnet-4-20250514 npm run build
```

**Set default model via environment:**
```bash
export V_MODEL=gpt-4o
v npm run build
```

### Advanced Examples

**Dry run — preview sanitized input:**
```bash
v --dry-run go build ./...
```

**Debug mode — see raw + sanitized payloads:**
```bash
v --debug npm run build
```

**Custom timeout for long-running commands:**
```bash
v --timeout 120 go test ./... -v
```

**Combine multiple flags:**
```bash
v --provider openai --model gpt-4o --max-tokens 800 --debug go build ./...
```

**Preview what would be sent (dry-run with piped input):**
```bash
go build ./... 2>&1 | v --dry-run
```

**Use specific model override:**
```bash
v --model llama-3.1-70b-versatile npm run build
```

---

## Command-Line Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--provider` | - | `groq` | AI provider: `groq`, `openai`, or `anthropic` |
| `--model` | - | (provider default) | Override the AI model |
| `--max-tokens` | - | `600` | Maximum tokens in AI response |
| `--timeout` | - | `30` | Command timeout in seconds |
| `--dry-run` | - | `false` | Show sanitized input without calling AI |
| `--debug` | - | `false` | Show raw and sanitized payloads |
| `--version` | - | `false` | Print version and exit |
| `--help` | `-h` | `false` | Show help message |

---

## Sanitization

v automatically redacts sensitive patterns before sending data to AI:

| Pattern | Example |
|---------|---------|
| AWS Access Keys | `AKIAIOSFODNN7EXAMPLE` |
| AWS Secret Keys | `aws_secret_access_key=...` |
| API Keys/Tokens | `api_key=abc123...` |
| Email Addresses | `user@example.com` |
| IPv4 Addresses | `192.168.1.100` |
| Windows Paths | `C:\Users\John\file.txt` |
| Unix Paths | `/home/user/project` |
| SSH Private Keys | `-----BEGIN PRIVATE KEY-----...` |
| Passwords in URLs | `postgres://admin:pass@localhost` |
| JWT Tokens | `eyJhbGci...` |
| Environment Username | `USERNAME` value |
| Environment Hostname | `COMPUTERNAME` value |

**Example dry-run output showing sanitization:**
```bash
$ echo "My API key is sk-1234567890abcdef" | v --dry-run
🔎 Dry Run — Sanitized Input:
My API key is [REDACTED]
```

---

## Smart Truncation

When input exceeds 6,000 characters, v uses intelligent truncation:

1. **Scores each line** based on error signal keywords (error, fail, panic, exception, etc.)
2. **Preserves short lines** (stack traces, error summaries)
3. **Selects highest-scoring lines** up to the character limit
4. **Maintains original order** with `[lines omitted]` markers

This ensures the AI receives the most relevant error information even from large logs.

---

## Troubleshooting

**"missing GROQ_API_KEY environment variable"**
- Set the appropriate API key for your chosen provider
- Check `.env` file is in the correct directory

**"unsupported provider"**
- Use: `groq`, `openai`, or `anthropic`

**Command times out**
- Increase timeout: `v --timeout 120 <command>`

**Empty AI response**
- Check API key is valid
- Try `--debug` to see request/response details

**Sanitization not working as expected**
- Use `--dry-run` to preview output
- Use `--debug` to see raw vs sanitized

---

## Testing

Run the included test suite:

```bash
go test -v ./...
```

Tests cover:
- Sanitization of AWS keys, emails, IPs, JWTs
- Smart truncation preserving error lines
- Error signal detection
- Configuration loading

---

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

---

## License

MIT License — see [LICENSE](LICENSE) file for details.

---

## Version

Current version: **2.1.0**