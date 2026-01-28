# LLMC - Simple Command Line LLM Client

A command-line tool for interacting with various LLM APIs. Currently supports OpenAI, Google's Gemini, and Anthropic Claude with built-in web search capabilities.

**Supported Platforms:** Linux and macOS

## Installation

```bash
# Using Go (installs to $HOME/go/bin or $GOPATH/bin)
go install github.com/longkey1/llmc@latest

# Or download the latest release from GitHub
# Visit https://github.com/longkey1/llmc/releases
```

## Quick Start

```bash
# 1. Initialize configuration
llmc init

# 2. Edit configuration file to set your API key
# File: $HOME/.config/llmc/config.toml
# Set: openai_token = "$OPENAI_API_KEY"
#   or gemini_token = "$GEMINI_API_KEY"
#   or anthropic_token = "$ANTHROPIC_API_KEY"

# 3. Start chatting
llmc chat "Hello, how are you?"

# 4. Try interactive mode
llmc chat --new-session -i "Let's have a conversation"
```

## Basic Usage

### Simple Chat

```bash
# Simple chat
llmc chat "Hello, how are you?"

# Read from stdin
echo "Hello, how are you?" | llmc chat

# Use default editor (from EDITOR environment variable)
llmc chat -e

# Specify model (format: provider:model)
llmc chat --model openai:gpt-4 "Hello"
llmc chat -m gemini:gemini-2.0-flash "Hello"
llmc chat -m anthropic:claude-3-5-sonnet-20241022 "Hello"
```

### Using Prompts

Create a prompt file (e.g., `$HOME/.config/llmc/prompts/example.toml`):
```toml
system = "You are a helpful assistant. {{input}}"
user = "Please help me with: {{input}}"
model = "openai:gpt-4o"  # Optional: overrides the default model for this prompt
web_search = false  # Optional: disable web search for this prompt
```

Use the prompt:
```bash
# List available prompts
llmc prompts

# Use a prompt
llmc chat --prompt example "What is the capital of France?"

# Pass arguments to prompt template
llmc chat --prompt example --arg name:John --arg age:30 "Hello"
```

### Session Support

LLMC supports conversation sessions to maintain conversation history across multiple interactions:

```bash
# Create a new session with an initial message
llmc chat --new-session "Hello, I'm starting a new conversation"
# → Session created: 550e8400

# Continue with session ID
llmc chat -s 550e8400 "What did we discuss earlier?"

# Use the latest session
llmc chat -s latest "What was my last question?"

# List all sessions
llmc sessions list

# Show session details
llmc sessions show 550e8400

# Rename a session
llmc sessions rename 550e8400 "project-meeting"

# Delete a session
llmc sessions delete 550e8400
```

### Interactive Mode

Start an interactive chat session with continuous conversation:

```bash
# Start interactive mode with a new session
llmc chat --new-session -i "Hello"

# Start interactive mode with an existing session
llmc chat -s 550e8400 -i
```

Interactive mode features:
- **`You>` prompt**: Type your messages naturally
- **Spinner animation**: Shows "Waiting for response..." while processing
- **Auto-save**: Session is saved after each turn
- **Special commands**:
  - `/help` or `/h` - Show available commands
  - `/info` or `/i` - Display session information
  - `/clear` or `/c` - Clear screen (Unix/Linux only)
  - `/exit` or `/quit` or `/q` - Exit interactive mode
  - `Ctrl+D` - Exit interactive mode

Example interactive session:
```
=== Interactive Session [550e8400] ===
Provider: openai, Model: gpt-4o-mini
Type '/help' for commands, '/exit' or 'Ctrl+D' to quit
===================================

You> What's the capital of France?
⠋ Waiting for response...

Assistant> The capital of France is Paris.

You> /exit
Goodbye!
```

## Features

### Web Search Support

Enable web search to access up-to-date information from the internet:

```bash
# Enable web search for a single query
llmc chat --web-search "What are the latest developments in quantum computing?"

# Use with prompt templates
llmc chat --web-search --prompt research "Latest AI research papers"

# Enable by default in config file
# Add to $HOME/.config/llmc/config.toml:
# enable_web_search = true
```

Web search can be enabled through multiple methods with priority order:
1. Command-line flag (highest)
2. Environment variable
3. Prompt template
4. Configuration file (lowest)

**Provider Support:**
- **OpenAI**: Uses Responses API with `web_search` tool (gpt-4o, o-series)
- **Gemini**: Uses Google Search Grounding with `google_search` tool
- **Anthropic**: Web search not natively supported in Messages API

Responses include source citations:
```
[Model's response incorporating search results...]

---
Sources:
[1] Article Title - https://example.com/article1
[2] Another Source - https://example.com/article2
```

**Known Issue: Gemini Empty Responses**

Gemini's web search occasionally returns empty responses. Use `--ignore-web-search-errors` to automatically retry without web search:

```bash
llmc chat --web-search --ignore-web-search-errors "query"

# Or enable by default in config:
# ignore_web_search_errors = true
```

### Session Management

#### Session Storage

Sessions are stored as JSON files in `$HOME/.config/llmc/sessions/` (or next to your custom config file).

#### Session Features

**Creating Sessions:**
```bash
# Create with initial message
llmc chat --new-session "Hello"

# Create with name
llmc chat --new-session --session-name "project-discussion" "Let's discuss"

# Create with specific model
llmc chat --new-session -m gemini:gemini-2.5-flash "Hello"

# Create with prompt template
llmc chat --new-session --prompt code-review "Review this code"
```

**Session IDs:**
Session IDs work like Git commit hashes:
- **Full UUID**: 36 characters (e.g., `550e8400-e29b-41d4-a716-446655440000`)
- **Short ID**: 8 characters displayed by default (e.g., `550e8400`)
- **Minimum prefix**: 4 characters required for commands (e.g., `550e`)

**Continuing Sessions:**
```bash
# Use 8-character short ID (recommended)
llmc chat -s 550e8400 "Continue our discussion"

# Use minimum 4-character prefix
llmc chat -s 550e "Tell me more"

# Use latest session
llmc chat -s latest "What was my last question?"
```

**Managing Sessions:**
```bash
# List all sessions
llmc sessions list

# Show session details and history
llmc sessions show 550e8400

# Rename a session
llmc sessions rename 550e8400 "new-name"

# Delete a session
llmc sessions delete 550e8400

# Delete old sessions (default: older than 30 days)
llmc sessions clear

# Delete sessions created before a specific date
llmc sessions clear --before 2024-01-01

# Delete all sessions
llmc sessions clear --all
```

#### Session Summarization

When sessions become too long, summarize them:

```bash
llmc sessions summarize 550e8400
# Summarizing 50 messages from session 550e8400...
# New session created: 9a3f92d1 (parent: 550e8400)
```

The summarization feature:
- Preserves the original session completely
- Creates a new session with `ParentID` linking to the original
- Places the summary as the first user message for context
- Inherits system prompt and template from original

#### Session Message Threshold

LLMC warns when sessions become too long (default: 50 messages):

```bash
llmc chat -s 550e8400 "Continue"
# Warning: Session 550e8400 has 55 messages (threshold: 50).
# Options:
#   1. Continue anyway with --ignore-threshold flag
#   2. Summarize session: llmc sessions summarize 550e8400
#   3. Start a new session: llmc chat --new-session
```

Configure threshold in config file:
```toml
session_message_threshold = 50  # 0 to disable warnings
```

Or bypass for a single command:
```bash
llmc chat -s 550e8400 --ignore-threshold "Continue anyway"
```

#### Session Retention

LLMC can automatically clean up old sessions to keep your session directory manageable. The `sessions clear` command respects parent-child relationships and will not delete parent sessions that are still referenced by child sessions.

**Default Behavior:**
```bash
# Delete sessions older than 30 days (default retention period)
llmc sessions clear

# The command will show what will be deleted:
# "Are you sure you want to delete 15 sessions older than 30 days (created before 2024-12-29)? [y/N]:"
```

**Custom Date Range:**
```bash
# Delete sessions created before a specific date
llmc sessions clear --before 2024-01-01
llmc sessions clear --before 2024-12      # Accepts YYYY-MM format
llmc sessions clear --before 2024         # Accepts YYYY format

# Delete all sessions (ignores retention setting)
llmc sessions clear --all
```

**Configure Retention Period:**

Set a custom retention period in your config file:
```toml
session_retention_days = 30  # Number of days to retain sessions (default: 30)
```

Or using environment variable:
```bash
export LLMC_SESSION_RETENTION_DAYS=90
```

**Parent Session Protection:**

Sessions with child sessions (from summarization) are automatically protected:
```bash
llmc sessions clear
# Notice: The following sessions were not deleted (referenced by child sessions):
#   - abcd1234 (created: 2023-12-15)
#
# Are you sure you want to delete 8 sessions older than 30 days? [y/N]:
```

#### Session Best Practices

1. **Use descriptive names**: `llmc sessions rename <id> "feature-planning"`
2. **Summarize long sessions**: Keep sessions under 50 messages for optimal performance
3. **Organize by topic**: Create separate sessions for different conversations
4. **Use interactive mode**: For back-and-forth discussions
5. **Leverage prompt templates**: Create sessions with pre-configured system prompts
6. **Clean up regularly**: Use `llmc sessions clear` to remove old sessions periodically

### Listing Available Models

View all available models by fetching real-time data from provider APIs:

```bash
# List models from all providers
llmc models

# List models for a specific provider
llmc models openai
llmc models gemini
llmc models anthropic
```

**Note**: Requires valid API token for each provider.

The output shows:
- **MODEL**: Full identifier in `provider:model` format
- **MODEL ID**: Model ID without provider prefix
- **DEFAULT**: Currently configured model (marked as "Yes")
- **DESCRIPTION**: Creation date (OpenAI) or description (Gemini)

Example output:
```
Available models for openai:

MODEL              MODEL ID      DEFAULT    DESCRIPTION
-----------------  ------------  ---------  ----------------------------------
openai:gpt-4o      gpt-4o        Yes        Created: 2024-05-13 12:00:00 JST
openai:gpt-4o-mini gpt-4o-mini              Created: 2024-07-18 12:00:00 JST

Use a model with: llmc chat --model <model> [message]
```

## Configuration

### Quick Configuration

1. Initialize configuration:
```bash
llmc init
```

2. Edit the configuration file at `$HOME/.config/llmc/config.toml`:
```toml
model = "openai:gpt-4.1"  # Format: provider:model
openai_token = "$OPENAI_API_KEY"  # Use $VAR or ${VAR} for environment variables
gemini_token = "${GEMINI_API_KEY}"
anthropic_token = "${ANTHROPIC_API_KEY}"
enable_web_search = false
session_retention_days = 30  # Delete sessions older than 30 days (default)
```

3. View current configuration:
```bash
# Show all configuration
llmc config

# Show specific field
llmc config model
llmc config openai_token
```

### Configuration Priority

All settings follow this priority order (higher overrides lower):

1. **Command-line flags** (highest priority)
2. **Environment variables** (with `LLMC_` prefix)
3. **Prompt template** (for `model` and `web_search` only)
4. **User configuration file** (`$HOME/.config/llmc/config.toml`)
5. **System-wide configuration** (`/etc/llmc/config.toml` or `/usr/local/etc/llmc/config.toml`)
6. **Default values** (lowest priority)

### Environment Variables

Configure using environment variables:

```bash
# Set model (format: provider:model)
export LLMC_MODEL="openai:gpt-4"

# Set API tokens
export LLMC_OPENAI_TOKEN="your-openai-api-token"
export LLMC_GEMINI_TOKEN="your-gemini-api-token"
export LLMC_ANTHROPIC_TOKEN="your-anthropic-api-token"

# Set API base URLs (optional)
export LLMC_OPENAI_BASE_URL="https://api.openai.com/v1"
export LLMC_GEMINI_BASE_URL="https://generativelanguage.googleapis.com/v1beta"
export LLMC_ANTHROPIC_BASE_URL="https://api.anthropic.com/v1"

# Set prompt directories (comma-separated)
export LLMC_PROMPT_DIRS="/path/to/prompts,/another/directory"

# Enable web search
export LLMC_ENABLE_WEB_SEARCH=true

# Automatically retry without web search if it fails (Gemini-specific)
export LLMC_IGNORE_WEB_SEARCH_ERRORS=true

# Set session message threshold
export LLMC_SESSION_MESSAGE_THRESHOLD=50

# Set session retention days
export LLMC_SESSION_RETENTION_DAYS=30
```

Add to your shell profile for persistence:
```bash
echo 'export LLMC_MODEL="openai:gpt-4"' >> ~/.bashrc
echo 'export LLMC_OPENAI_TOKEN="your-token"' >> ~/.bashrc
source ~/.bashrc
```

### Advanced Configuration

#### System-Wide Configuration

System administrators can provide organization-wide defaults:

```bash
# Create system-wide config
sudo mkdir -p /etc/llmc
sudo tee /etc/llmc/config.toml > /dev/null <<EOF
model = "openai:gpt-4o"
openai_base_url = "https://api.openai.com/v1"
gemini_base_url = "https://generativelanguage.googleapis.com/v1beta"
enable_web_search = false
EOF
```

Users override specific settings in `$HOME/.config/llmc/config.toml`:
```toml
# Only override what you need
openai_token = "$OPENAI_API_KEY"
model = "openai:gpt-4o-mini"
```

Use verbose mode to see which configs are loaded:
```bash
llmc -v chat "Hello"
# Output:
# Loaded system-wide config: /etc/llmc/config.toml
# Merged user config: /home/user/.config/llmc/config.toml
```

#### Configuration File Format

Complete configuration file example:

```toml
model = "openai:gpt-4.1"  # Format: provider:model
openai_base_url = "https://api.openai.com/v1"  # Optional: supports $VAR or ${VAR}
openai_token = "$OPENAI_API_KEY"  # Use environment variable reference
gemini_base_url = "https://generativelanguage.googleapis.com/v1beta"  # Optional
gemini_token = "${GEMINI_API_KEY}"  # Both $VAR and ${VAR} supported
anthropic_base_url = "https://api.anthropic.com/v1"  # Optional
anthropic_token = "${ANTHROPIC_API_KEY}"  # Both $VAR and ${VAR} supported
prompt_dirs = ["/path/to/prompts", "/another/directory"]  # Multiple directories
enable_web_search = false  # Enable web search by default
ignore_web_search_errors = false  # Auto-retry without web search (Gemini-specific)
session_message_threshold = 50  # Warn when session exceeds message count (0 to disable)
session_retention_days = 30  # Number of days to retain sessions (default: 30)
```

#### Viewing Configuration

```bash
# Show all configuration
llmc config

# Show specific fields
llmc config model                    # → openai:gpt-4.1
llmc config openai_base_url          # → https://api.openai.com/v1
llmc config openai_token             # → sk-... (masked)
llmc config gemini_base_url          # → https://generativelanguage.googleapis.com/v1beta
llmc config gemini_token             # → ... (masked)
llmc config anthropic_base_url       # → https://api.anthropic.com/v1
llmc config anthropic_token          # → ... (masked)
llmc config promptdirs               # → /path/to/prompts,/another/directory
llmc config websearch                # → false
llmc config ignorewebsearcherrors    # → false
llmc config sessionretentiondays     # → 30
llmc config configfile               # → /home/user/.config/llmc/config.toml
```

### File Locations

#### Configuration Files

LLMC searches for configuration files in the following order (later files override earlier ones):

1. **System-wide configuration** (optional, searched in order):
   - `/etc/llmc/config.toml` - Standard system config location
   - `/usr/local/etc/llmc/config.toml` - Alternative system config location
2. **User configuration**: `$HOME/.config/llmc/config.toml` - User-specific settings (higher priority)
3. **Custom configuration**: `--config /path/to/config.toml` - Overrides all other configs

#### Prompt Directories

LLMC searches for prompts in multiple directories with the following priority (later takes precedence):

1. **`/usr/share/llmc/prompts`** - System package prompts (lowest priority)
   - Used when installed via package manager (apt, yum, etc.)
2. **`/usr/local/share/llmc/prompts`** - Local install prompts (low priority)
   - Used when installed via `go install` or manual build
3. **`$HOME/.config/llmc/prompts`** - User-specific prompts (highest priority)
   - Can override system prompts by using the same filename

You can add custom directories in your configuration file:
```toml
prompt_dirs = ["/path/to/dir1", "/path/to/dir2", "/path/to/dir3"]
```

**Priority Rules:**
- Later directories override earlier ones
- If `dir1/example.toml` and `dir3/example.toml` exist, the tool uses `dir3/example.toml`

#### System Administrator Setup

Provide organization-wide prompts:

**For `go install` or manual builds:**
```bash
sudo mkdir -p /usr/local/share/llmc/prompts
sudo cp your-prompts/*.toml /usr/local/share/llmc/prompts/
```

**For package manager installations:**
```bash
sudo mkdir -p /usr/share/llmc/prompts
sudo cp your-prompts/*.toml /usr/share/llmc/prompts/
```

#### Viewing Prompt Locations

```bash
# List all prompts with file paths
llmc prompts

# Show verbose output with duplicate warnings
llmc prompts --verbose
```

The prompt list displays:
- **PROMPT**: Prompt name (relative path from prompt directory)
- **MODEL**: Model specified in prompt (or default in parentheses)
- **WEB SEARCH**: enabled/disabled (or default in parentheses)
- **FILE PATH**: Full path to prompt file

Example:
```
PROMPT           MODEL                      WEB SEARCH  FILE PATH
---------------  -------------------------  ----------  -----------------------------------------------
commit           (gemini:gemini-2.5-flash)  (disabled)  /home/user/.config/llmc/prompts/commit.toml
code-review      openai:gpt-4o              enabled     /home/user/.config/llmc/prompts/code-review.toml
```

Values in parentheses indicate defaults from configuration.

#### Session Storage

Sessions are stored as JSON files:
- If using `$HOME/.config/llmc/config.toml`: sessions in `$HOME/.config/llmc/sessions/`
- If using `--config /path/to/config.toml`: sessions in `/path/to/sessions/`

### Prompt Template Format

Prompt templates are TOML files with the following structure:

```toml
system = "System prompt with optional {{input}} placeholder"
user = "User prompt with optional {{input}} placeholder"
model = "optional-model-name"  # Optional: overrides default model
web_search = true  # Optional: enables web search
```

The `{{input}}` placeholder is replaced with the user's message. Additional placeholders can be passed via `--arg` flag:

```bash
# Basic format
llmc chat --arg key:value

# Multiple arguments
llmc chat --arg name:John --arg age:30

# Values with special characters
llmc chat --arg path:"C:\Users\name\file.txt"
llmc chat --arg url:"https://example.com:8080"
```

Special character handling:
- Use `\:` to include a colon in the value
- Use `\"` to include a double quote
- Use `\\` to include a backslash
- Values can be wrapped in double quotes

**Note**: `input` is a reserved keyword and cannot be used as an argument key.

### Input Methods

The tool supports three input methods with the following priority:

1. **Editor** (when `-e` or `--editor` is specified):
   - Opens default editor from `EDITOR` environment variable
   - Example: `llmc chat -e`

2. **Command line arguments**:
   - Used when arguments are provided and editor is not specified
   - Example: `llmc chat "Hello, world!"`

3. **Standard input**:
   - Used when no arguments are provided and editor is not specified
   - Example: `echo "Hello, world!" | llmc chat`

## Development

For developers working on the LLMC codebase:

### Building from Source

**IMPORTANT:** Always use `make` commands for building and testing. Do not use `go build` or `go test` directly.

```bash
# Build the binary (outputs to ./bin/llmc)
make build

# Run tests
make test

# Format code
make fmt

# Vet code
make vet

# Tidy dependencies
make tidy

# Clean build artifacts
make clean

# Show all available make targets
make help
```

### Running Without Installing

```bash
# Run directly with go run
go run main.go [command]

# Examples
go run main.go chat "Your message"
go run main.go prompts
go run main.go config
```

### Release Management

```bash
# Create a new release (dry run)
make release type=patch    # v1.2.3 -> v1.2.4
make release type=minor    # v1.2.3 -> v1.3.0
make release type=major    # v1.2.3 -> v2.0.0

# Execute release (pushes tag to trigger GitHub Actions)
make release type=patch dryrun=false

# Re-release existing tag (useful for fixing releases)
make re-release tag=v1.2.3 dryrun=false
```

GitHub Actions automatically builds and publishes binaries via GoReleaser when tags are pushed.

## Debug Mode

Enable verbose output with the `-v` flag:
```bash
llmc chat -v "Hello"
```

## Model Compatibility

LLMC uses provider-specific APIs:

**OpenAI**: Uses Responses API with support for GPT-4, GPT-5, and O-series models (o3, o4). The `llmc models openai` command fetches the latest available models from OpenAI's API, filtered to show only compatible models with Responses API.

**Gemini**: Supports all Gemini models that support the `generateContent` method. The `llmc models gemini` command fetches the latest available models from Google's Gemini API.

**Anthropic**: Uses Messages API with support for Claude 3 and Claude 4 models (Opus, Sonnet, Haiku). The `llmc models anthropic` command fetches the latest available models from Anthropic's API.

The models list is dynamically retrieved from each provider's API, so you'll always see the most current available models without needing to update the tool.
