# Feishu -> Codex Connector (Go, Long Connection)

A minimal connector that:

1. Uses **Feishu official Go SDK** (`github.com/larksuite/oapi-sdk-go/v3`) long connection (`ws`) mode.
2. Receives `im.message.receive_v1` text events.
3. Calls `codex exec` for each text message.
4. Sends the reply back to Feishu.

This mode works **without a public IP** because it uses Feishu long connection (WebSocket), not webhook callbacks.

## Why Go instead of Rust

Feishu currently provides official server SDKs for Go/Java/Python/Node, and official long connection support is in the official Go SDK. There is no official Rust SDK.

## Requirements

- Go 1.23+ (tested on Go 1.26)
- `codex` CLI installed and logged in (`codex login status`)
- A Feishu app with:
  - Bot capability enabled
  - Event subscription to `im.message.receive_v1`
  - Required message permissions
  - Long connection mode enabled in Feishu platform settings

## Quickstart

```bash
cp .env.example .env
# edit .env
set -a; source .env; set +a

# install dependencies
go mod tidy

# run tests
go test ./...

# start connector
go run ./cmd/connector
```

## Pre-commit checks

Run all checks manually:

```bash
make check
```

Install git pre-commit hook (runs `make check` before commit):

```bash
make precommit-install
```

## Environment variables

Required:

- `FEISHU_APP_ID`
- `FEISHU_APP_SECRET`

Optional:

- `FEISHU_BASE_URL` (default `https://open.feishu.cn`)
- `CODEX_COMMAND` (default `codex`)
- `CODEX_TIMEOUT_SECS` (default `120`)
- `WORKSPACE_DIR` (default `.`)
- `CODEX_PROMPT_PREFIX` (default short Chinese instruction)
- `FAILURE_MESSAGE` (default fallback message when Codex fails)
- `QUEUE_CAPACITY` (default `256`)
- `WORKER_CONCURRENCY` (default `1`)
- `LOG_LEVEL` (`debug|info|warn|error`, default `info`)

## Runtime behavior

- Non-text messages are ignored.
- Mention tags like `<at ...>...</at>` are removed from text before sending to Codex.
- Reply target priority: `chat_id`, fallback to sender `open_id`.
- On Codex failure/timeout, sends `FAILURE_MESSAGE`.

## Project layout

- `cmd/connector/main.go`: bootstrap and lifecycle
- `internal/config/config.go`: env config and validation
- `internal/codex/codex.go`: Codex CLI call + JSONL parsing
- `internal/connector/connector.go`: long connection, queue, workers, Feishu send
