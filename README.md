# `$v`

## V - AI Build Error Analyzer

## Description
`V` is a powerful command-line tool designed to streamline the debugging process by providing AI-powered suggestions for build errors. It integrates seamlessly into your development workflow, allowing you to analyze error logs from various sources, including piped input or direct command execution. By leveraging the Groq API, `V` offers intelligent insights while ensuring sensitive information is sanitized before being sent for analysis.

## Features
- **AI-Powered Analysis**: Utilizes the Groq API to analyze complex error logs, identify root causes, and suggest specific, actionable fixes.
- **Flexible Input Modes**:
    - **Piped Input**: Analyze error messages directly piped from other commands or files. Ideal for quick analysis of existing logs.
    - **Command Execution**: Execute any command (e.g., `npm run build`, `make`, `go test`) and automatically capture its standard output and error streams for AI analysis.
- **Robust Data Sanitization**: Automatically detects and redacts sensitive patterns such as API keys, AWS credentials, email addresses, IP addresses, file paths, SSH private keys, and passwords in URLs. This ensures your confidential data remains secure during AI processing.
- **Dry Run Mode (`--dry-run`)**: Preview the sanitized input that would be sent to the AI without actually making an API call. This is invaluable for verifying that sensitive data is correctly redacted.
- **Debug Mode (`--debug`)**: Display both the raw and sanitized inputs, along with the AI's raw response, which is useful for troubleshooting and understanding the tool's behavior.
- **Configurable Command Timeout (`--timeout N`)**: Set a maximum execution time for commands run through `V`, preventing long-running or stuck processes from consuming excessive resources.

## Installation

To get `V` up and running, follow these steps:

1.  **Prerequisites**:
    *   **Go**: Ensure you have Go (version 1.18 or higher recommended) installed on your system. You can download it from [golang.org](https://golang.org/dl/).
    *   **Groq API Key**: Obtain an API key from [Groq](https://groq.com/). This key is essential for `V` to communicate with the AI model.

2.  **Clone the Repository**:
    First, clone the `V` repository to your local machine.
    ```bash
    git clone https://github.com/your-username/v.git # Replace with your actual repo URL
    cd v
    ```

3.  **Build the Executable**:
    Navigate to the `v` directory (where `main.go` is located) and build the executable.
    ```bash
    go build -o v.exe # For Windows
    # or
    go build -o v     # For Linux/macOS
    ```
    This command compiles the Go source code and creates an executable file named `v.exe` (or `v`).

4.  **Set Up Groq API Key**:
    `V` needs your Groq API key to function. You can provide it in one of two ways:

    *   **Using a `.env` file (Recommended for local development)**:
        Create a file named `.env` in the same directory as your `v` executable (or in the project root if you run `v` from there). Add the following line to it:
        ```
        GROQ_API_KEY=your_groq_api_key_here
        ```
        Replace `your_groq_api_key_here` with your actual Groq API key.

    *   **Using an Environment Variable**:
        Set the `GROQ_API_KEY` environment variable directly in your shell. This is often preferred for CI/CD environments or if you don't want a `.env` file.
        ```bash
        # For Linux/macOS (add to your .bashrc, .zshrc, etc. for persistence)
        export GROQ_API_KEY="your_groq_api_key_here"

        # For Windows Command Prompt
        set GROQ_API_KEY=your_groq_api_key_here

        # For Windows PowerShell
        $env:GROQ_API_KEY="your_groq_api_key_here"
        ```

5.  **Add to PATH (Optional but Recommended)**:
    To run `V` from any directory, move the compiled executable (`v.exe` or `v`) to a directory that is included in your system's `PATH` environment variable.
    *   **Linux/macOS**: `/usr/local/bin`
    *   **Windows**: Create a `bin` folder (e.g., `C:\Users\YourUser\bin`) and add it to your `PATH`.

## Usage

`V` offers two primary modes of operation: analyzing piped input or executing a command.

### 1. Analyzing Piped Input

Pipe the output of any command or the content of a file directly into `V`.

**Syntax:**
```bash
<command_output> | v [flags]
< file.log | v [flags]
```

**Examples:**

*   **Analyze a simple error message:**
    ```bash
    echo "Error: Uncaught TypeError: Cannot read properties of undefined (reading 'map')" | v
    ```

*   **Analyze a build log from a file:**
    ```bash
    cat build_error.log | v
    ```

*   **Analyze `npm run build` output in dry-run mode (to see sanitized input):**
    ```bash
    npm run build 2>&1 | v --dry-run
    ```
    *(`2>&1` redirects stderr to stdout, ensuring all error messages are piped)*

### 2. Executing a Command

Run a command directly through `V`. `V` will execute the command, capture its output (stdout and stderr), and then send it to the AI for analysis.

**Syntax:**
```bash
v [flags] <command> [command_arguments...]
```

**Important Note on Flags:**
`V`'s flags (like `--dry-run`, `--debug`, `--timeout`) must be placed *before* the command you want to execute. If your command also has flags, you can separate `V`'s flags from your command's flags using `--`.

**Examples:**

*   **Analyze `npm run build` errors:**
    ```bash
    v npm run build
    ```

*   **Analyze `make` errors:**
    ```bash
    v make clean all
    ```

*   **Analyze `go build` errors:**
    ```bash
    v go build ./...
    ```

*   **Run a Docker build and analyze its output:**
    ```bash
    v docker build .
    ```

*   **Execute a command with a custom timeout (e.g., 60 seconds for `npm test`):**
    ```bash
    v --timeout 60 npm test
    ```

*   **Pass flags to your command (e.g., `--verbose` to `npm run build`):**
    ```bash
    v -- npm run build --verbose
    ```
    *(The `--` tells `V` to stop parsing its own flags and pass everything that follows directly to the executed command.)*

## Flags

Here's a detailed list of the command-line flags available for `V`:

*   **`--dry-run`**:
    *   **Description**: If set, `V` will process the input (either piped or from a command's output), sanitize it, and then print the sanitized content to the console. It will *not* make any calls to the Groq AI API.
    *   **Usage**: `v --dry-run <command>` or `echo "error" | v --dry-run`
    *   **Example**:
        ```bash
        v --dry-run npm run build
        # Output will show:
        # Dry Run Mode:
        # --- Sanitized Input Sent to AI ---
        # [Sanitized build output]
        # -----------------------------
        ```

*   **`--debug`**:
    *   **Description**: Enables verbose debugging output. When used, `V` will print both the raw input (before sanitization) and the sanitized input that is sent to the AI. This is helpful for understanding how `V` processes your data and for troubleshooting sanitization rules.
    *   **Usage**: `v --debug <command>` or `echo "error" | v --debug`
    *   **Example**:
        ```bash
        echo "My API key is ABC123XYZ" | v --debug
        # Output will show:
        # Raw Input: My API key is ABC123XYZ
        # Sanitized Input: My API key is [REDACTED]
        # ... (AI analysis will follow)
        ```

*   **`--timeout N`**:
    *   **Description**: Specifies the maximum duration, in seconds, that `V` will wait for an executed command to complete. If the command exceeds this time, `V` will terminate it and proceed to analyze any output captured up to that point. A note about the timeout will be appended to the analyzed output.
    *   **Default**: `30` seconds.
    *   **Usage**: `v --timeout 60 <command>`
    *   **Example**:
        ```bash
        v --timeout 10 sleep 20 # 'sleep 20' will be terminated after 10 seconds
        # Output will include:
        # ⚠️  Command timed out after 10 seconds. Analyzing captured output...
        # ... (AI analysis)
        # [Command timed out after 10 seconds]
        ```

*   **`--help`, `-h`**:
    *   **Description**: Displays the help message, showing all available flags and usage examples.
    *   **Usage**: `v --help` or `v -h`

## Sanitization Guidelines

`V` employs a set of regular expressions to identify and redact sensitive information from your error logs. This is crucial for privacy and security when sending data to external AI services. The following types of information are currently sanitized:

*   AWS Access Keys (e.g., `AKIA...`, `A3T...`)
*   AWS Secret Access Keys
*   Generic API Keys, Tokens, and Secrets (patterns like `api_key`, `token`, `secret` followed by alphanumeric strings)
*   Email Addresses
*   IP Addresses (IPv4)
*   File Paths (both Windows and Unix-like formats)
*   SSH Private Keys (PEM format)
*   Passwords embedded in URLs

If you encounter sensitive information that is not being redacted, please consider contributing to improve the sanitization patterns.

## Contributing
We welcome contributions to `V`! If you have suggestions for new features, improvements to sanitization patterns, bug reports, or want to contribute code, please feel free to:
1.  Open an issue on the [GitHub repository](https://github.com/your-username/v/issues).
2.  Fork the repository and submit a pull request.

## License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
