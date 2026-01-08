# LLMC - Command Line LLM Client

A command-line tool for interacting with various LLM APIs. Currently supports OpenAI and Google's Gemini with built-in web search capabilities.

**Supported Platforms:** Linux and macOS

## Installation

```bash
# Using Go (installs to $HOME/go/bin or $GOPATH/bin)
go install github.com/longkey1/llmc@latest

# Or download the latest release from GitHub
# Visit https://github.com/longkey1/llmc/releases
```

### File Locations

After installation, LLMC uses the following directories:

#### Configuration Files

LLMC searches for configuration files in the following order (later files override earlier ones):

1. **System-wide configuration** (optional, searched in order):
   - `/etc/llmc/config.toml` - Standard system config location
   - `/usr/local/etc/llmc/config.toml` - Alternative system config location
2. **User configuration**: `$HOME/.config/llmc/config.toml` - User-specific settings (higher priority)
3. **Custom configuration**: `--config /path/to/config.toml` - Overrides all other configs

If a system-wide configuration exists, user configuration values will be merged on top of it, allowing users to override specific settings while inheriting others.

#### Prompt Directories

- **Prompt directories** (searched in order, later takes precedence):
  1. `/usr/share/llmc/prompts` - System package prompts (lowest priority, optional)
  2. `/usr/local/share/llmc/prompts` - Local install prompts (low priority, optional)
  3. `$HOME/.config/llmc/prompts` - User-specific prompts (highest priority)

You can add custom prompt directories by editing the `prompt_dirs` array in your configuration file.

## Configuration

LLMC supports multiple configuration methods with the following priority (higher priority overrides lower):

1. **Command-line flags** (highest priority)
2. **Environment variables** (with `LLMC_` prefix)
3. **User configuration file** (`$HOME/.config/llmc/config.toml`)
4. **System-wide configuration file** (`/etc/llmc/config.toml` or `/usr/local/etc/llmc/config.toml`)
5. **Default values** (lowest priority)

### Method 1: Configuration File (Recommended)

#### User Configuration

1. Initialize the configuration:
```bash
llmc init
```

This will create a configuration file at `$HOME/.config/llmc/config.toml` with default settings.

2. View current configuration:
```bash
# Show all configuration values
llmc config

# Show specific field only
llmc config provider
llmc config model
llmc config baseurl
llmc config token
llmc config promptdirs
llmc config configfile

# Example outputs:
# llmc config provider    → openai
# llmc config model       → gpt-4.1
# llmc config promptdirs  → /path/to/prompts,/another/prompt/directory
```

This will display all current configuration values, with the API token masked for security. You can also specify a field name to show only that field's value. The `promptdirs` field displays directories as comma-separated values.

3. Edit the configuration file to set your API keys and preferences:
```toml
provider = "openai"  # or "gemini"
base_url = "https://api.openai.com/v1"  # or Gemini's API URL
model = "your-model-name"  # Specify the model you want to use
token = "your-api-token"
prompt_dirs = ["/path/to/prompts", "/another/prompt/directory"]  # Multiple directories supported
enable_web_search = false  # Enable web search by default (default: false)
```

#### System-Wide Configuration

System administrators can provide organization-wide defaults:

```bash
# Create system-wide config directory
sudo mkdir -p /etc/llmc

# Create system-wide configuration
sudo tee /etc/llmc/config.toml > /dev/null <<EOF
provider = "openai"
base_url = "https://api.openai.com/v1"
model = "gpt-4o"
# Don't include token in system-wide config - users should set this individually
# No need to set prompt_dirs - defaults will be used
enable_web_search = false
EOF
```

Users can then override specific settings in their `$HOME/.config/llmc/config.toml`:

```toml
# Only override what you need - inherits other settings from system config
token = "your-personal-api-token"
model = "gpt-4o-mini"  # Override the system default
```

**Note**: Use verbose mode to see which configuration files are loaded:
```bash
llmc -v chat "Hello"
# Output will show:
# Loaded system-wide config: /etc/llmc/config.toml
# Merged user config: /home/user/.config/llmc/config.toml
```

### Method 2: Environment Variables

You can also configure the tool using environment variables. Environment variables take precedence over configuration file settings.

```bash
# Set provider (openai or gemini)
export LLMC_PROVIDER="openai"

# Set API base URL
export LLMC_BASE_URL="https://api.openai.com/v1"

# Set model name
export LLMC_MODEL="your-model-name"

# Set API token
export LLMC_TOKEN="your-api-token"

# Set prompt directories (comma-separated)
export LLMC_PROMPT_DIRS="/path/to/prompts,/another/prompt/directory"

# Enable web search (true or false)
export LLMC_ENABLE_WEB_SEARCH=true
```

You can add these to your shell profile (e.g., `~/.bashrc`, `~/.zshrc`) to make them persistent:

```bash
# Add to your shell profile
echo 'export LLMC_PROVIDER="openai"' >> ~/.bashrc
echo 'export LLMC_TOKEN="your-api-token"' >> ~/.bashrc
echo 'export LLMC_MODEL="your-model-name"' >> ~/.bashrc
source ~/.bashrc
```

**Note**: Configuration priority order: command-line flags > environment variables > prompt template (for `model` and `web_search` only) > user configuration file > system-wide configuration file > defaults. See the "Configuration Priority" section below for details.

**Note**: Prompt directories in environment variables use comma (`,`) as separator.

## Usage

### Basic Usage

```bash
# Simple chat
llmc chat "Hello, how are you?"

# Read from stdin
echo "Hello, how are you?" | llmc chat

# Use default editor (from EDITOR environment variable)
llmc chat -e

# List available models (fetches from API)
llmc models
```

### Using Prompts

#### Default Prompt Directories

LLMC searches for prompts in multiple directories with the following priority:

1. **`/usr/share/llmc/prompts`** - System package prompts (lowest priority)
   - Used when LLMC is installed via package manager (apt, yum, etc.)
   - Requires administrator privileges to create/modify
2. **`/usr/local/share/llmc/prompts`** - Local install prompts (low priority)
   - Used when LLMC is installed via `go install` or manual build
   - Requires administrator privileges to create/modify
3. **`$HOME/.config/llmc/prompts`** - User-specific prompts (highest priority)
   - Takes precedence over all system prompts
   - Can override system prompts by using the same filename

#### Creating Prompts

Create a prompt file (e.g., `$HOME/.config/llmc/prompts/example.toml`):
```toml
system = "You are a helpful assistant. {{input}}"
user = "Please help me with: {{input}}"
model = "gpt-4o"  # Optional: overrides the default model for this prompt
web_search = false  # Optional: disable web search for this prompt
```

You can also create prompt files in multiple directories. The tool will search for prompt files in all configured directories in the order they are specified in the configuration. If the same prompt file name exists in multiple directories, the file from the later directory in the configuration will be used (later directories take precedence).

List available prompt templates:
```bash
# List all available prompts (shows in table format with file paths)
llmc prompt

# List prompts with verbose output (shows duplicate file warnings)
llmc prompt --verbose
```

The prompt list is displayed in a table format showing the prompt name and the full file path. When using `--verbose`, the tool will show warnings if the same prompt file name exists in multiple directories, indicating which directory's file will be used.

Use the prompt:
```bash
llmc chat --prompt example "What is the capital of France?"
```

### Command Line Arguments

```bash
# Specify provider
llmc chat --provider openai "Hello"

# Specify model
llmc chat --model gpt-4 "Hello"

# Specify base URL
llmc chat --base-url "https://api.openai.com/v1" "Hello"

# Use prompt template
llmc chat --prompt example "Hello"

# Pass arguments to prompt template
llmc chat --prompt example --arg name:John --arg age:30 "Hello"

# Use default editor
llmc chat -e
# or
llmc chat --editor

# Enable web search for real-time information
llmc chat --web-search "What's the latest news about AI?"
```

### Input Methods

The tool supports three input methods, with the following priority:

1. Editor (when `-e` or `--editor` is specified):
   - Opens the default editor specified by the `EDITOR` environment variable
   - All other input methods (arguments and stdin) are ignored
   - Example: `llmc chat -e`

2. Command line arguments:
   - Used when arguments are provided and editor is not specified
   - Example: `llmc chat "Hello, world!"`

3. Standard input:
   - Used when no arguments are provided and editor is not specified
   - Example: `echo "Hello, world!" | llmc chat`

### Argument Format

Arguments can be passed to prompt templates using the `--arg` flag:

```bash
# Basic format
llmc chat --arg key:value

# Multiple arguments
llmc chat --arg name:John --arg age:30

# Values with special characters
llmc chat --arg path:"C:\Users\name\file.txt"
llmc chat --arg url:"https://example.com:8080"
llmc chat --arg message:"Hello \"World\""
```

Special character handling:
- Use `\:` to include a colon in the value
- Use `\"` to include a double quote in the value
- Use `\\` to include a backslash in the value
- Values can be wrapped in double quotes for better readability

Note: `input` is a reserved keyword and cannot be used as an argument key.

## Prompt Template Format

Prompt templates are TOML files with the following structure:
```toml
system = "System prompt with optional {{input}} placeholder"
user = "User prompt with optional {{input}} placeholder"
model = "optional-model-name"  # Optional: overrides the default model for this prompt
web_search = true  # Optional: enables web search for this prompt (default: false)
```

The `{{input}}` placeholder will be replaced with the user's message. Additional placeholders can be defined using the `--arg` flag.

### Template-Specific Settings

- **model**: Override the default model for this specific prompt
- **web_search**: Enable or disable web search for this prompt, useful for templates that always need real-time information

### Multiple Prompt Directories

#### Default Behavior

By default, LLMC searches in three directories:
```toml
prompt_dirs = [
  "/usr/share/llmc/prompts",            # System package (lowest priority)
  "/usr/local/share/llmc/prompts",      # Local install (low priority)
  "$HOME/.config/llmc/prompts"          # User-specific (highest priority)
]
```

Later directories in the array take precedence. If a prompt file with the same name exists in multiple directories, the file from the later directory will be used.

#### Custom Configuration

You can configure additional directories in your configuration file:
```toml
prompt_dirs = ["/path/to/dir1", "/path/to/dir2", "/path/to/dir3"]
```

**Priority Rules:**
- Later directories override earlier ones
- If `/path/to/dir1/example.toml` and `/path/to/dir3/example.toml` both exist, the tool uses `/path/to/dir3/example.toml`

#### System Administrator Setup

To provide organization-wide prompts, choose the appropriate location based on installation method:

**For `go install` or manual builds:**
```bash
# Create local install prompt directory
sudo mkdir -p /usr/local/share/llmc/prompts

# Add sample prompts
sudo cp your-prompts/*.toml /usr/local/share/llmc/prompts/
```

**For package manager installations:**
```bash
# Create system package prompt directory
sudo mkdir -p /usr/share/llmc/prompts

# Add sample prompts
sudo cp your-prompts/*.toml /usr/share/llmc/prompts/
```

Users can then override these by creating files with the same names in their `$HOME/.config/llmc/prompts` directory.

#### Viewing Prompt Locations

Use the `--verbose` flag to see which file will be used for each prompt:
```bash
llmc prompt --verbose
```

The prompt list shows the full file path for each prompt, making it easy to see which directory each prompt comes from.

## Web Search Support

LLMC supports web search functionality using native API features from both OpenAI and Gemini, allowing models to access up-to-date information from the internet.

### Enabling Web Search

Web search can be enabled through multiple methods with the following priority order (higher priority overrides lower):

**1. Command-line flag (highest priority, per-query)**
```bash
# Enable web search for a single query
llmc chat --web-search "What are the latest developments in quantum computing?"

# Disable web search even if enabled in other configurations
llmc chat --web-search=false "Historical question"
```

**2. Environment variable (session-wide)**
```bash
export LLMC_ENABLE_WEB_SEARCH=true
llmc chat "Latest news about SpaceX"
```

**3. Prompt template (template-specific)**
```toml
# In your prompt template file (e.g., research.toml)
system = "You are a research assistant with access to real-time information"
user = "{{input}}"
web_search = true  # This template always uses web search
```

```bash
llmc chat --prompt research "Current state of AI research"
```

**4. Configuration file (default behavior)**
```toml
# In $HOME/.config/llmc/config.toml
enable_web_search = true
```

### Priority Examples

```bash
# Example 1: Command-line flag overrides all
export LLMC_ENABLE_WEB_SEARCH=true
llmc chat --prompt research --web-search=false "question"
# Result: Web search is DISABLED (flag takes priority)

# Example 2: Environment variable overrides template and config
export LLMC_ENABLE_WEB_SEARCH=false
llmc chat --prompt research "question"  # research.toml has web_search=true
# Result: Web search is DISABLED (env var takes priority over template and config)

# Example 3: Template overrides config files
# config.toml has enable_web_search=false
llmc chat --prompt research "question"  # research.toml has web_search=true
# Result: Web search is ENABLED (template takes priority over config files)
```

### Provider-Specific Details

**OpenAI (Responses API)**
- Uses OpenAI's Responses API with the `web_search` tool
- Supported with recent OpenAI models (e.g., gpt-4o, o-series)
- If you use an unsupported model, you'll receive a helpful error message with suggestions
- Search pricing applies per query (check OpenAI's pricing page for current rates)

**Gemini (Google Search Grounding)**
- Uses Gemini's built-in `google_search` tool with grounding
- Supported with current Gemini models (e.g., gemini-2.0-flash, gemini-2.5-pro)
- Search pricing applies per query (check Google's pricing page for current rates)

### Citation Format

When web search is enabled, responses include source citations in a consistent format:

```
[Model's response text incorporating search results...]

---
Sources:
[1] Article Title - https://example.com/article1
[2] Another Source - https://example.com/article2
[3] Third Source - https://example.com/article3
```

### Examples

```bash
# Get current information with citations
llmc chat --web-search "Who won the 2026 World Cup?"

# Use with prompt templates
llmc chat --web-search --prompt research "Latest AI research papers"

# Combine with verbose mode to see configuration
llmc chat -v --web-search "Current stock price of AAPL"
```

### Checking Configuration

```bash
# Check if web search is enabled in your configuration
llmc config websearch

# View all configuration including web search setting
llmc config
```

## Configuration Priority

All configuration settings follow the same priority order:

1. **Command-line flags** (highest priority)
2. **Environment variables** (with `LLMC_` prefix)
3. **Prompt template** (for `model` and `web_search` only)
4. **User configuration file** (`$HOME/.config/llmc/config.toml`)
5. **System-wide configuration file** (`/etc/llmc/config.toml` or `/usr/local/etc/llmc/config.toml`)
6. **Default values** (lowest priority)

### Example: Model Selection Priority

```bash
# Priority demonstration for model selection
export LLMC_MODEL="gpt-4o-mini"

# Scenario 1: Command-line flag wins
llmc chat --model gpt-4o "Hello"
# Uses: gpt-4o (from flag)

# Scenario 2: Environment variable used
llmc chat "Hello"
# Uses: gpt-4o-mini (from env var)

# Scenario 3: Prompt template overrides config files but not env var
llmc chat --prompt example "Hello"  # example.toml has model="o3"
# Uses: gpt-4o-mini (env var takes priority over template and config files)
```

## Listing Available Models

View all available models for your configured provider by fetching real-time data from the API:

```bash
# List models for the currently configured provider
llmc models
```

**Note**: This command uses the provider configured in your config file (or via `LLMC_PROVIDER` environment variable) and requires a valid API token for that provider.

The output shows:
- **MODEL ID**: The identifier to use with `--model` flag
- **DEFAULT**: Indicates which model is currently configured (marked as "Yes")
- **DESCRIPTION**:
  - For OpenAI: Model creation date in JST (e.g., "Created: 2024-05-13 12:00:00 JST")
  - For Gemini: Detailed model description from the API

### Example Output

**OpenAI:**
```
Available models for openai:

MODEL ID      DEFAULT    DESCRIPTION
------------  ---------  ----------------------------------
gpt-4o        Yes        Created: 2024-05-13 12:00:00 JST
gpt-4o-mini              Created: 2024-07-18 12:00:00 JST
```

**Gemini:**
```
Available models for gemini:

MODEL ID           DEFAULT    DESCRIPTION
-----------------  ---------  --------------------------------------------------
gemini-2.5-flash   Yes        Stable version of Gemini 2.5 Flash, our mid-size...
gemini-2.5-pro                Stable release (June 17th, 2025) of Gemini 2.5 Pro
```

The models list is fetched directly from each provider's API, ensuring you always see the most up-to-date available models. Models are automatically sorted by ID in descending order (newest/latest versions first).

## Model Compatibility

LLMC uses provider-specific APIs:

**OpenAI**: Uses Responses API with support for GPT-4, GPT-5, and O-series models (o3, o4). The `llmc models` command fetches the latest available models from OpenAI's API, filtered to show only compatible models with Responses API.

**Gemini**: Supports all Gemini models that support the `generateContent` method. The `llmc models` command fetches the latest available models from Google's Gemini API.

The models list is dynamically retrieved from the provider's API, so you'll always see the most current available models without needing to update the tool. If you use an unsupported model, you'll receive a helpful error message with suggestions.

## Debug Mode

Enable verbose output with the `-v` flag:
```bash
llmc chat -v "Hello"
```