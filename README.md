# LLMC - Command Line LLM Client

A command-line tool for interacting with various LLM APIs. Currently supports OpenAI and Google's Gemini.

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
model = "gpt-3.5-turbo"  # or "gemini-pro"
token = "your-api-token"
prompt_dirs = ["/path/to/prompts", "/another/prompt/directory"]  # Multiple directories supported
```

### Method 2: Environment Variables

You can also configure the tool using environment variables. Environment variables take precedence over configuration file settings.

```bash
# Set provider (openai or gemini)
export LLMC_PROVIDER="openai"

# Set API base URL
export LLMC_BASE_URL="https://api.openai.com/v1"

# Set model name
export LLMC_MODEL="gpt-3.5-turbo"

# Set API token
export LLMC_TOKEN="your-api-token"

# Set prompt directories (comma-separated)
export LLMC_PROMPT_DIRS="/path/to/prompts,/another/prompt/directory"
```

You can add these to your shell profile (e.g., `~/.bashrc`, `~/.zshrc`) to make them persistent:

```bash
# Add to your shell profile
echo 'export LLMC_PROVIDER="openai"' >> ~/.bashrc
echo 'export LLMC_TOKEN="your-api-token"' >> ~/.bashrc
echo 'export LLMC_MODEL="gpt-3.5-turbo"' >> ~/.bashrc
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

You can also create prompt files in multiple directories. The tool will search for prompt files in all configured directories in the order they are specified in the configuration.

List available prompt templates:
```bash
# List all available prompts
llmc prompt

# List prompts with directory information
llmc prompt --with-dir
```

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

## Debug Mode

Enable verbose output with the `-v` flag:
```bash
llmc chat -v "Hello"
```