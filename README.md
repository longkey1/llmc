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

1. Initialize the configuration:
```bash
llmc init
```

This will create a configuration file at `$HOME/.config/llmc/config.toml` with default settings.

2. Edit the configuration file to set your API keys and preferences:
```toml
provider = "openai"  # or "gemini"
base_url = "https://api.openai.com/v1"  # or Gemini's API URL
model = "gpt-3.5-turbo"  # or "gemini-pro"
token = "your-api-token"
prompt_dir = "/path/to/prompts"
```

## Usage

### Basic Usage

```bash
# Simple chat
llmc chat "Hello, how are you?"

# Read from stdin
echo "Hello, how are you?" | llmc chat
```

### Using Prompts

Create a prompt file (e.g., `$HOME/.config/llmc/prompts/example.toml`):
```toml
system = "You are a helpful assistant. {{input}}"
user = "Please help me with: {{input}}"
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
```

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
```

The `{{input}}` placeholder will be replaced with the user's message. Additional placeholders can be defined using the `--arg` flag.

## Debug Mode

Enable verbose output with the `-v` flag:
```bash
llmc chat -v "Hello"
```

## License

[License information here]
