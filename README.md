# Robotron

A personal robot assistant for Telegram built with OpenAI.

## Dependencies

The following dependencies must be installed in your system to run Robotron:

- [FFmpeg](https://ffmpeg.org/)

## Configuration

Robotron is configured through environment variables:

| Name                      | Description                               |
| ------------------------- | ----------------------------------------- |
| `ROBOTRON_TELEGRAM_TOKEN` | Telegram bot token                        |
| `ROBOTRON_OPENAI_API_KEY` | OpenAI API key                            |
| `ROBOTRON_LOG_LEVEL`      | Log level (default: INFO)                 |
| `ROBOTRON_ALLOWED_USERS`  | Comma separated list of allowed user IDs  |
| `ROBOTRON_MEASURE_UNITS`  | Units of measure to use (default: metric) |

## Development

To develop Robotron, install the following tools:

- [Go 1.20](https://go.dev/dl/)
- [Just](https://github.com/casey/just)
- [Watchexec](https://github.com/watchexec/watchexec)
- [Golangci-Lint](https://golangci-lint.run/usage/install/)
