# LLMC - Simple Command Line LLM Client

A command-line tool for interacting with various LLM APIs. Currently supports OpenAI, Google's Gemini, and Anthropic Claude with built-in web search capabilities, plus local models via Ollama.

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
# (Ollama needs no token; just run a local Ollama server)

# 3. Start chatting
llmc chat "Hello, how are you?"

# 4. Try interactive mode
llmc sessions start
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
llmc chat -m ollama:llama3:latest "Hello"  # Local model via Ollama

# Use a config-defined model alias (see "Model Aliases")
llmc chat -m @sonnet "Hello"
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
llmc sessions start

# Start interactive mode with an existing session
llmc sessions start 550e8400

# Start interactive mode with latest session
llmc sessions start latest
```

Interactive mode features:
- **`You>` prompt**: Type your messages naturally
- **Multiline editing**: The input area grows vertically; insert newlines with `Ctrl+J`
- **External editor**: Compose long messages in `$EDITOR` with `Ctrl+G`
- **Spinner animation**: Shows "Waiting for response..." while processing
- **Auto-save**: Session is saved after each turn
- **Input history**: Command history persisted across sessions (stored in `~/.config/llmc/history`)
- **Special commands**:
  - `/help` or `/h` - Show available commands
  - `/info` or `/i` - Display session information
  - `/clear` or `/c` - Clear screen (Unix/Linux only)
  - `/exit` or `/quit` or `/q` - Exit interactive mode
  - `Ctrl+D` - Exit interactive mode

#### Input Editing Keybindings

**Sending and Newlines:**
- `Enter` - Send message
- `Ctrl+J` (or `Ctrl+O`) - Insert newline
- `\` at end of line + `Enter` - Insert newline (continuation)
- `Alt+Enter` - Force send (ignores continuation)

**External Editor:**
- `Ctrl+G` - Edit the current input in `$EDITOR`; the saved content is placed back into the prompt

**Cursor Movement:**
- `←` / `→` / `↑` / `↓` - Move cursor (also across lines in multiline input)
- `Ctrl+A` - Move to beginning of line
- `Ctrl+E` - Move to end of line

**Editing:**
- `Backspace` - Delete character before cursor
- `Delete` / `Ctrl+D` - Delete character at cursor (exits on an empty line)
- `Ctrl+W` - Delete word before cursor
- `Ctrl+U` - Delete from cursor to beginning of line
- `Ctrl+K` - Delete from cursor to end of line

**History:**
- `↑` on the first line / `Alt+P` - Previous history entry
- `↓` on the last line / `Alt+N` - Next history entry
- `Ctrl+R` - Search history

**Other:**
- `Ctrl+C` - Clear current input (exits if input is empty)
- `Ctrl+D` - Exit interactive mode (when the current line is empty)

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

### Tool Calling (Built-in Tools)

Enable tool calling to let the model fetch external resources on its own
during a conversation. When enabled, the model can decide to call built-in
tools, llmc executes them locally, and the results are fed back to the model
until it produces a final answer.

```bash
# Enable tools for a single query
llmc chat --tools "Summarize https://example.com/article"

# Works in interactive mode too
llmc sessions start --tools

# Skip confirmation prompts (exec_command / write_file)
llmc chat --tools --yes "Run the tests and summarize the failures"

# Enable by default in config file
# Add to $HOME/.config/llmc/config.toml:
# enable_tools = true
```

**Built-in tools:**

| Tool | Description | Confirmation |
|------|-------------|--------------|
| `fetch_url` | HTTP GET a URL and return the body as text (http/https only, 256KB limit, 30s timeout) | No |
| `read_file` | Read a local file (relative paths resolve from the current directory, 256KB limit) | Deny list |
| `exec_command` | Run a shell command via `sh -c` and return stdout/stderr and the exit code (64KB limit, 60s timeout) | Yes |
| `write_file` | Write text to a local file (the parent directory must exist, 1MB limit) | Yes |

Tool calling can be enabled through multiple methods with the same priority
order as web search: flag (`--tools`) > environment variable
(`LLMC_ENABLE_TOOLS`) > prompt template (`tools = true`) > config file
(`enable_tools`).

**Confirmation and safety:**
- `exec_command` and `write_file` show what will be executed and ask for
  `[y/N]` confirmation before running. `--yes` skips the prompt, and
  `exec_allowed_commands` pre-approves specific commands.
- When stdin is not a TTY (e.g. piped input), anything that would need
  confirmation is automatically denied and the model is told to proceed
  without it.
- `fetch_url` can reach any http/https URL, including internal addresses, and
  has no allow list. Enable tools only for input you trust.
- File paths are restricted by the rules below, with credential directories
  denied by default.
- A single turn is limited to 10 tool-loop iterations.

#### Command Allow and Deny Rules

By default every `exec_command` invocation asks for `[y/N]` confirmation. Use
the allow rules to pre-approve routine commands so they stop prompting, and the
deny rules to block commands outright. Rules are a table of command name to
subcommands, where an empty list (or `["*"]`) covers every subcommand:

```toml
# $HOME/.config/llmc/config.toml
[exec_allowed_commands]
git = ["status", "diff", "log"]   # git status / git diff / git log only
go = ["test", "build", "vet"]
ls = []                           # any ls invocation
cat = ["*"]                       # same as []

[exec_denied_commands]
git = ["push"]                    # carve an exception out of an allow rule
```

If you prefer a single line, TOML inline tables work too:

```toml
exec_allowed_commands = { git = ["status", "diff"], ls = [] }
```

The command name must match the rule key exactly, and the rest of the command
matches a listed subcommand by whole-word prefix, so `status` covers
`status --short` but not `status-ish`. Rules apply to every command in the
shell line (split on unquoted `;`, `|`, `&` and newlines).

- **Allowed** → runs without a confirmation prompt.
- **Denied** → refused, ahead of the allow rules and of `--yes`. Use it to
  block a subcommand of an otherwise allowed command.
- **Neither** → the usual `[y/N]` prompt.

Auto-approval needs every command in the line to be allowed: `ls && curl
example.com` still prompts when only `ls` is allowed. Lines using **command
substitution** (`$(...)`, backticks, `<(...)`) or **redirection** (`>`, `>>`,
`<`) are never auto-approved, because prefix matching cannot see what the
substituted command is or what the redirection would overwrite — `ls >
~/.ssh/authorized_keys` is not a read-only command. Quoted characters don't
count, so `git log --grep "a > b"` is fine.

**The allow rules are not a sandbox.** Matching a shell string cannot be a
security boundary, so by default the rules only decide whether you are *asked*,
never whether a command *may run*.

To make the allow rules a gate instead, refuse everything else:

```toml
exec_unlisted = "deny"   # "confirm" (default) | "deny"
```

In `deny` mode nothing runs unless it matches an allow rule, and lines with
substitution or redirection are refused rather than confirmed. This is a much
stronger posture, but still not a sandbox: allowing a command grants everything
that command itself can do — `find` has `-exec`, `git` has `-c core.pager=`,
and allowing `sh` or `env` allows anything. When `deny` mode leaves nothing
runnable, `exec_command` isn't advertised to the model at all.

#### File Path Allow and Deny Rules

`write_file` and `read_file` take the same shape of rules, on paths instead of
commands:

```toml
write_allowed_paths = ["."]                     # skip confirmation for these
write_denied_paths = ["~/.ssh", ".git", ".env"] # always refuse
write_unlisted = "confirm"                      # "confirm" (default) | "deny"

read_denied_paths = ["~/.ssh", "~/.config/llmc", "*.pem"]
```

A rule **containing a path separator** covers that path and everything under
it, after `~` expansion and resolution against the current directory: `~/.ssh`
covers `~/.ssh/config`. A rule **without a separator** is matched as a glob
against each path component: `.git` covers any `.git` directory at any depth,
and `*.pem` covers any `.pem` file.

`write_file` behaves like `exec_command`: allowed paths write without a prompt,
denied paths are refused ahead of both the allow rules and `--yes`, and
anything else prompts (or is refused with `write_unlisted = "deny"`).
`read_file` needs no confirmation, so it has deny rules only — everything not
denied simply reads.

Paths are **canonicalized before matching**: made absolute, with `..` resolved
and symlinks in the parent chain followed. Without that, `./a/../../../etc/hosts`
or a symlinked directory would walk straight past the rules. The confirmation
prompt shows the canonical path for the same reason. `write_file` additionally
**refuses to write through a symlink**, since that would modify a file outside
the path you approved. Reads may follow symlinks, but the deny rules are
checked against the real target.

**Defaults ship with a deny list**, so credentials are out of reach without any
configuration:

| Setting | Default |
|---------|---------|
| `read_denied_paths` | `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.config/llmc`, `.env`, `*.pem` |
| `write_denied_paths` | `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.config/llmc`, `.git` |

`~/.config/llmc` is on both lists because it holds your provider tokens. Set
the lists to `[]` if you really want them unrestricted.

Note that these rules do not cover writes made **through `exec_command`**: a
shell command you approve can write anywhere, and even in `exec_unlisted =
"deny"` mode, allowing something like `tee` or `sed` allows writes. Path rules
and command rules are complementary; neither subsumes the other.

#### Environment Variables Passed to Commands

By default `exec_command` inherits your environment minus anything that looks
like a credential, so the model cannot read your API keys back out with
`env`. Variables are matched by **name**: `LLMC_*` plus any name containing
`TOKEN`, `SECRET`, `PASSWORD`, `API_KEY`, `ACCESS_KEY`, `CREDENTIAL`,
`PRIVATE_KEY` or `SESSION_KEY`.

```toml
# "filtered" (default) | "minimal" | "all"
exec_env_mode = "filtered"

# Pass these through regardless of the mode
exec_env_passthrough = ["GITHUB_TOKEN"]
```

| Mode | Behavior |
|------|----------|
| `filtered` | Inherit everything except credential-looking names (default) |
| `minimal` | Pass only `PATH`, `HOME`, `USER`, `SHELL`, `LANG`, `LC_ALL`, `TERM`, `TZ`, `PWD` |
| `all` | Inherit the parent environment unchanged (no filtering) |

`read_file` is covered separately by `read_denied_paths`, which denies
`~/.config/llmc` by default so the config file's tokens are out of reach too.

**Provider support:** all four providers (OpenAI, Gemini, Anthropic, Ollama
with a tool-capable model). Web search and tools can be combined on OpenAI
and Gemini; note that some Gemini models reject the combination and return an
API error. Anthropic still rejects `--web-search` regardless of tools.

Tool calls and their results are recorded in session history (shown as
`Tool` entries in `llmc sessions show`) and count toward the session message
threshold. They are skipped when summarizing a session, and stripped from the
request when you continue a tools-enabled session with tools disabled.

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

# Delete a specific session
llmc sessions delete 550e8400

# Delete old sessions (default: older than retention period)
llmc sessions delete

# Delete sessions created before a specific date
llmc sessions delete --before 2024-01-01

# Delete all sessions (including protected parent sessions)
llmc sessions delete --all
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

LLMC can automatically clean up old sessions to keep your session directory manageable. The `sessions delete` command (without an ID) respects parent-child relationships and will not delete parent sessions that are still referenced by child sessions.

**Default Behavior:**
```bash
# Delete sessions older than 30 days (default retention period)
llmc sessions delete

# The command will show what will be deleted:
# "Are you sure you want to delete 15 sessions older than 30 days (created before 2024-12-29)? [y/N]:"
```

**Custom Date Range:**
```bash
# Delete sessions created before a specific date
llmc sessions delete --before 2024-01-01
llmc sessions delete --before 2024-12      # Accepts YYYY-MM format
llmc sessions delete --before 2024         # Accepts YYYY format

# Delete all sessions including protected parent sessions
llmc sessions delete --all
```

**Configure Retention Period:**

Set a custom retention period in your config file:
```toml
session_retention_days = 30  # Number of days to retain sessions (default: 30)
                              # Set to 0 to disable auto-deletion
```

Or using environment variable:
```bash
export LLMC_SESSION_RETENTION_DAYS=90
```

**Disabling Auto-Deletion:**

Set `session_retention_days = 0` to disable automatic session cleanup. Running `llmc sessions delete` without arguments or flags will show a notice and exit without deleting anything:
```bash
llmc sessions delete
# Auto-deletion is disabled (session_retention_days = 0).
# Use --before or --all to delete sessions explicitly.
```

You can still delete sessions explicitly using `--before` or `--all`:
```bash
llmc sessions delete --before 2024-01-01
llmc sessions delete --all
```

**Parent Session Protection:**

When using date-based deletion, sessions with child sessions (from summarization) are automatically protected:
```bash
llmc sessions delete
# Notice: The following sessions were not deleted (referenced by child sessions):
#   - abcd1234 (created: 2023-12-15)
#
# Are you sure you want to delete 8 sessions older than 30 days? [y/N]:
```

Note: `--all` bypasses this protection and deletes every session unconditionally.

#### Session Best Practices

1. **Use descriptive names**: `llmc sessions rename <id> "feature-planning"`
2. **Summarize long sessions**: Keep sessions under 50 messages for optimal performance
3. **Organize by topic**: Create separate sessions for different conversations
4. **Use interactive mode**: For back-and-forth discussions
5. **Leverage prompt templates**: Create sessions with pre-configured system prompts
6. **Clean up regularly**: Use `llmc sessions delete` to remove old sessions periodically

### Model Aliases

Define short aliases for models in your config file and use them anywhere a
model can be specified, prefixed with `@`:

```toml
# $HOME/.config/llmc/config.toml
[model_aliases]
sonnet = "openai:anthropic/claude-sonnet-*"   # wildcard: resolves to the newest match
gpt    = "openai:openai/gpt-5*"
pinned = "openai:anthropic/claude-sonnet-4-6" # no wildcard: used as-is (no API call)
```

```bash
llmc chat -m @sonnet "Hello"
# -> resolves to e.g. openai:anthropic/claude-sonnet-5
```

**Wildcard resolution:**
- `*` in the alias value matches any characters in the model ID. The matching
  models are fetched from the provider's model list API, and the newest one is
  selected.
- "Newest" is determined by comparing numeric version components in the ID
  (`claude-sonnet-5` > `claude-sonnet-4-6` > `claude-sonnet-4-5`,
  `gpt-5.6` > `gpt-5.2`). Snapshot dates (`-20250929`, `@20250929`,
  `-2025-08-07`) are compared only between IDs with equal versions, and
  undated IDs are preferred over dated ones.
- Values without `*` are used as-is, with no API call — useful for pinning.
- This works with OpenAI-compatible proxies such as LiteLLM, including
  route-prefixed model IDs (e.g., `anthropic/claude-sonnet-5`,
  `vertex_ai/gemini-2.5-flash`).

**Behavior:**
- Aliases are accepted from the `-m/--model` flag, the `LLMC_MODEL`
  environment variable, prompt templates (`model = "@sonnet"`), and the
  `model` setting in the config file.
- New sessions store the resolved concrete model, so continuing a session
  never changes its model.
- Using `@name` that is not defined in `[model_aliases]` is an error.

**Checking what an alias resolves to:**

```bash
# Prints the resolved provider:model to stdout
llmc models resolve @sonnet
# -> openai:anthropic/claude-sonnet-5

# Wildcard patterns can be tested directly
llmc models resolve "openai:anthropic/claude-opus-*"

# Usable in scripts
llmc chat -m "$(llmc models resolve @sonnet)" "Hello"
```

### Listing Available Models

View all available models by fetching real-time data from provider APIs:

```bash
# List models from all providers (skips providers without tokens)
llmc models

# List models for a specific provider
llmc models openai
llmc models gemini
llmc models anthropic
llmc models ollama       # Locally installed Ollama models
```

**Token Requirements:**
- When listing **all providers** (`llmc models`): Providers without configured tokens are silently skipped (Ollama needs no token, so it is queried; a warning is shown if the server is unreachable)
- When listing a **specific provider** (`llmc models openai`): Returns an error if the token is not configured

The output shows:
- **MODEL**: Full identifier in `provider:model` format
- **MODEL ID**: Model ID without provider prefix
- **DESCRIPTION**: Creation date (OpenAI/Anthropic) or description (Gemini)
- **ALIAS**: Config-defined aliases that currently resolve to this model
  (e.g., `@sonnet`); aliases that match no model are reported as warnings
- **DEFAULT**: Currently configured model (marked as "Yes")

Example output:
```
Available models for openai:

MODEL              MODEL ID      DESCRIPTION                          ALIAS   DEFAULT
-----------------  ------------  -----------------------------------  ------  ----------
openai:gpt-5-mini  gpt-5-mini    Created: 2025-08-06 05:32:08 JST     @mini   Yes
openai:gpt-4o      gpt-4o        Created: 2024-05-13 12:00:00 JST
openai:gpt-4o-mini gpt-4o-mini   Created: 2024-07-18 12:00:00 JST

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
enable_web_search = false
enable_tools = false
exec_unlisted = "confirm"   # Unlisted commands: "confirm" | "deny"
exec_env_mode = "filtered"  # Environment passed to exec_command
session_retention_days = 30  # Delete sessions older than 30 days (default)

# Option 1: Reference environment variables (recommended for security)
openai_token = "$OPENAI_API_KEY"      # or "${OPENAI_API_KEY}"
gemini_token = "$GEMINI_API_KEY"      # or "${GEMINI_API_KEY}"
anthropic_token = "$ANTHROPIC_API_KEY"  # or "${ANTHROPIC_API_KEY}"

# Option 2: Set tokens directly (not recommended for shared configs)
# openai_token = "sk-..."
# gemini_token = "..."
# anthropic_token = "..."

# Option 3: Use environment variables (no config file changes needed)
# export LLMC_OPENAI_TOKEN="sk-..."
# export LLMC_GEMINI_TOKEN="..."
# export LLMC_ANTHROPIC_TOKEN="..."
```

3. Set your API token (choose one method):
```bash
# Method 1: Set environment variable (simplest)
export OPENAI_API_KEY="sk-..."
# Then reference it in config: openai_token = "$OPENAI_API_KEY"

# Method 2: Use LLMC-specific environment variable (no config file changes)
export LLMC_OPENAI_TOKEN="sk-..."

# Method 3: Set directly in config file (not recommended for shared configs)
# openai_token = "sk-..."
```

4. View current configuration:
```bash
# Show all configuration
llmc config

# Show specific field
llmc config model
llmc config openai_token  # Shows masked token or "(not set)"
```

### Configuration Priority

All settings follow this priority order (higher overrides lower):

1. **Command-line flags** (highest priority)
2. **Environment variables** (with `LLMC_` prefix)
3. **Prompt template** (for `model`, `web_search`, and `tools` only)
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
export LLMC_OLLAMA_TOKEN="your-ollama-token"  # Optional: only for authenticated remote servers

# Set API base URLs (optional)
export LLMC_OPENAI_BASE_URL="https://api.openai.com/v1"
export LLMC_GEMINI_BASE_URL="https://generativelanguage.googleapis.com/v1beta"
export LLMC_ANTHROPIC_BASE_URL="https://api.anthropic.com/v1"
export LLMC_OLLAMA_BASE_URL="http://localhost:11434/v1"

# Set prompt directories (comma-separated)
export LLMC_PROMPT_DIRS="/path/to/prompts,/another/directory"

# Enable web search
export LLMC_ENABLE_WEB_SEARCH=true

# Enable built-in tool calling
export LLMC_ENABLE_TOOLS=true

# Tool policy. exec_allowed_commands / exec_denied_commands are nested tables
# and can only be set in the config file (like model_aliases).
export LLMC_EXEC_UNLISTED=confirm
export LLMC_EXEC_ENV_MODE=filtered
export LLMC_EXEC_ENV_PASSTHROUGH="GITHUB_TOKEN"
export LLMC_WRITE_ALLOWED_PATHS="."
export LLMC_WRITE_DENIED_PATHS="~/.ssh,.git"
export LLMC_WRITE_UNLISTED=confirm
export LLMC_READ_DENIED_PATHS="~/.ssh,*.pem"

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

# API tokens - Environment variable references (recommended)
# Supports both $VAR and ${VAR} syntax
openai_token = "$OPENAI_API_KEY"        # Expands from environment variable
gemini_token = "${GEMINI_API_KEY}"      # Both syntaxes work
anthropic_token = "$ANTHROPIC_API_KEY"
# ollama_token = "..."                  # Optional: local Ollama needs no token

# API base URLs (optional - uses defaults if not set)
# Also supports environment variable expansion
openai_base_url = "https://api.openai.com/v1"
gemini_base_url = "https://generativelanguage.googleapis.com/v1beta"
anthropic_base_url = "https://api.anthropic.com/v1"
ollama_base_url = "http://localhost:11434/v1"

# Prompt directories (optional - uses defaults if not set)
prompt_dirs = ["/path/to/prompts", "/another/directory"]

# Feature flags
enable_web_search = false  # Enable web search by default
enable_tools = false       # Enable built-in tool calling by default

# Tool policy (see "Tool Calling")
exec_unlisted = "confirm"    # Unlisted commands: "confirm" | "deny"
exec_env_mode = "filtered"   # "filtered" | "minimal" | "all"
exec_env_passthrough = []    # Variables passed to exec_command regardless of mode
write_allowed_paths = []     # Paths that skip the write confirmation prompt
write_denied_paths = ["~/.ssh", "~/.aws", "~/.gnupg", "~/.config/llmc", ".git"]
write_unlisted = "confirm"   # Unlisted write paths: "confirm" | "deny"
read_denied_paths = ["~/.ssh", "~/.aws", "~/.gnupg", "~/.config/llmc", ".env", "*.pem"]

[exec_allowed_commands]      # Command name -> subcommands that skip the prompt ([] = all)
# git = ["status", "diff"]

[exec_denied_commands]       # Always refused, even with --yes
# git = ["push"]

# Session management
session_message_threshold = 50  # Warn when session exceeds message count (0 to disable)
session_retention_days = 30     # Number of days to retain sessions (default: 30, 0 to disable)

# Model aliases (optional) - use as "@name", e.g. llmc chat -m @sonnet
[model_aliases]
sonnet = "openai:anthropic/claude-sonnet-*"  # "*" resolves to the newest matching model
pinned = "openai:anthropic/claude-sonnet-4-6"
```

#### Viewing Configuration

```bash
# Show all configuration
llmc config

# Show specific fields
llmc config model                    # → openai:gpt-4.1
llmc config openai_base_url          # → https://api.openai.com/v1
llmc config openai_token             # → sk-... (masked) or "(not set)"
llmc config gemini_base_url          # → https://generativelanguage.googleapis.com/v1beta
llmc config gemini_token             # → ... (masked) or "(not set)"
llmc config anthropic_base_url       # → https://api.anthropic.com/v1
llmc config anthropic_token          # → ... (masked) or "(not set)"
llmc config ollama_base_url          # → http://localhost:11434/v1
llmc config ollama_token             # → ... (masked) or "(not set)"
llmc config promptdirs               # → /path/to/prompts,/another/directory
llmc config websearch                # → false
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

#### Interactive Mode History

Interactive mode command history is persisted to disk:
- History file: `$HOME/.config/llmc/history` (plain text, one entry per line; multiline input is flattened to a single line)
- History is shared across all interactive sessions
- `↑` on the first line, `↓` on the last line, `Alt+P`/`Alt+N`, and `Ctrl+R` navigate through history

### Prompt Template Format

Prompt templates are TOML files with the following structure:

```toml
system = "System prompt with optional {{input}} placeholder"
user = "User prompt with optional {{input}} placeholder"
model = "optional-model-name"  # Optional: overrides default model
web_search = true  # Optional: enables web search
tools = true  # Optional: enables built-in tools
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

**Ollama**: Uses Ollama's OpenAI-compatible Chat Completions API (`http://localhost:11434/v1` by default) to run local models. No API token is required for a local server; set `ollama_token` only when connecting to a remote server behind an authenticating proxy. Web search is not supported. The `llmc models ollama` command lists locally installed models.

The models list is dynamically retrieved from each provider's API, so you'll always see the most current available models without needing to update the tool.

## License

MIT
