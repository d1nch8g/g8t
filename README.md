## Description

g8t is a command-line tool that helps you execute tasks using AI assistants. It supports multiple AI providers and allows configuration of various execution parameters.

## Configuration

The following options can be configured when running `g8t`:

- `--verbose`, `-v`: Enable verbose output.
- `--quiet`, `-q`: Suppress non-essential output.
- `--dry-run`, `-d`: Show commands without executing them.
- `--max-commands`, `-m <number>`: Maximum number of commands to execute.
- `--provider`, `-p <provider>`: Specify AI provider (openai, claude, gemini, yandex, ollama).

To use g8t, you need to provide a task description as a command-line argument. For example: `g8t "Summarize article in article.md and print output to summary.md"`. You will be prompted to configure the tool on first use. After that, you can edit `~/.g8t.yml` to switch providers or update settings.

## Configuration

g8t supports multiple AI providers: Yandex, OpenAI, DeepSeek, Claude, Gemini, and Ollama. You need to configure the API keys and model names for your chosen provider. The configuration is stored in `~/.g8t.yml`.

## Installation

To install the project, follow these steps:

1. Ensure that you have Go installed on your system.
2. Install it with Go:

```sh
go install github.com/d1nch8g/g8t/cmd/g8t@latest
```
