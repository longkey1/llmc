# LLMC - Command Line LLM Client

A command-line tool for interacting with various LLM APIs. Currently supports OpenAI and Google's Gemini with built-in web search capabilities.

## Installation

```bash
# Using Go
go install github.com/longkey1/llmc@latest

# Or download the latest release from GitHub
# Visit https://github.com/longkey1/llmc/releases
```

## Configuration

### Method 1: Configuration File (Recommended)

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
model = "gpt-4.1"  # OpenAI: gpt-4o, gpt-4.1, o3, o4-mini, gpt-5 / Gemini: gemini-2.0-flash, etc.
token = "your-api-token"
prompt_dirs = ["/path/to/prompts", "/another/prompt/directory"]  # Multiple directories supported
enable_web_search = false  # Enable web search by default (default: false)
```

### Method 2: Environment Variables

You can also configure the tool using environment variables. Environment variables take precedence over configuration file settings.

```bash
# Set provider (openai or gemini)
export LLMC_PROVIDER="openai"

# Set API base URL
export LLMC_BASE_URL="https://api.openai.com/v1"

# Set model name
export LLMC_MODEL="gpt-4.1"

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
echo 'export LLMC_MODEL="gpt-4.1"' >> ~/.bashrc
source ~/.bashrc
```

**Note**: Environment variables override configuration file settings. If both are set, the environment variable value will be used.

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
```

### Using Prompts

Create a prompt file (e.g., `$HOME/.config/llmc/prompts/example.toml`):
```toml
system = "You are a helpful assistant. {{input}}"
user = "Please help me with: {{input}}"
model = "gpt-4"  # Optional: overrides the default model for this prompt
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
```

The `{{input}}` placeholder will be replaced with the user's message. Additional placeholders can be defined using the `--arg` flag.

### Multiple Prompt Directories

When you have multiple prompt directories configured, the tool searches for prompt files in the order specified in your configuration. If the same prompt file name exists in multiple directories, the file from the later directory will be used (later directories take precedence over earlier ones).

For example, if your configuration has:
```toml
prompt_dirs = ["/path/to/dir1", "/path/to/dir2", "/path/to/dir3"]
```

And both `/path/to/dir1/example.toml` and `/path/to/dir3/example.toml` exist, the tool will use `/path/to/dir3/example.toml`.

You can use the `--verbose` flag with the `prompt` command to see warnings about duplicate files:
```bash
llmc prompt --verbose
```

The prompt list will show the full file path for each prompt, making it easy to see which directory each prompt comes from.

## Web Search Support

LLMC supports web search functionality using native API features from both OpenAI and Gemini, allowing models to access up-to-date information from the internet.

### Enabling Web Search

**Method 1: Command-line flag (per-query)**
```bash
# Enable web search for a single query
llmc chat --web-search "What are the latest developments in quantum computing?"

# Combine with other flags
llmc chat --web-search --model gpt-4o "Latest news about SpaceX"
```

**Method 2: Configuration file (default behavior)**
```toml
# In $HOME/.config/llmc/config.toml
enable_web_search = true
```

**Method 3: Environment variable**
```bash
export LLMC_ENABLE_WEB_SEARCH=true
```

### Provider-Specific Details

**OpenAI (Responses API)**
- Uses OpenAI's Responses API with the `web_search` tool
- **Supported models only**: gpt-4o, gpt-4.1, o3, o4-mini, gpt-5 series
- **Important**: Older models like gpt-3.5-turbo are not supported with Responses API
- If you use an unsupported model, you'll receive a helpful error message with suggestions
- Pricing: $30 per 1,000 queries for gpt-4o search, $25 for gpt-4o-mini search

**Gemini (Google Search Grounding)**
- Uses Gemini's built-in `google_search` tool with grounding
- **All current Gemini models supported**: gemini-2.0-flash, gemini-2.5-pro, etc.
- Billing started January 5, 2026: $14 per 1,000 search queries

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

## Model Compatibility

### OpenAI Models

LLMC uses OpenAI's Responses API, which supports the following models:
- **gpt-4o** series (gpt-4o, gpt-4o-mini)
- **gpt-4.1** series (gpt-4.1, gpt-4.1-mini, gpt-4.1-nano)
- **o-series** reasoning models (o3, o4-mini)
- **gpt-5** series (gpt-5, gpt-5.1, gpt-5.2)

**Note**: Older models like gpt-3.5-turbo are not supported. If you attempt to use an unsupported model, you'll receive a clear error message:

```
Error: Model 'gpt-3.5-turbo' is not supported with Responses API.

Supported models: gpt-4o, gpt-4.1, o3, o4-mini, gpt-5 series

Please change your model with --model flag or in config file.
Example: llmc chat --model gpt-4o "your question"
```

### Gemini Models

All current Gemini models are supported, including:
- gemini-2.0-flash (default)
- gemini-2.5-pro
- gemini-2.5-flash
- gemini-2.5-flash-lite
- gemini-1.5-pro
- gemini-1.5-flash

## Debug Mode

Enable verbose output with the `-v` flag:
```bash
llmc chat -v "Hello"
```