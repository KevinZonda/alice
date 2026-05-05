# Alice

[![Dev CI](https://github.com/Alice-space/alice/actions/workflows/ci.yml/badge.svg)](https://github.com/Alice-space/alice/actions/workflows/ci.yml)
[![Main Release](https://github.com/Alice-space/alice/actions/workflows/main-release.yml/badge.svg)](https://github.com/Alice-space/alice/actions/workflows/main-release.yml)

A Feishu long-connection connector for CLI-based LLM agents (Codex, Claude, Gemini, Kimi, OpenCode).

Alice runs as a local multi-bot runtime — receives Feishu messages over WebSocket, routes them into `chat` or `work` scenes, calls the configured LLM CLI, and sends replies, files, and images back.

## Documentation

Full documentation is at **[alice-space.github.io/alice](https://alice-space.github.io/alice/)**.

| | |
|--|--|
| [Tutorials](https://alice-space.github.io/alice/en/tutorials/quick-start.html) | Get Alice running in 5 minutes |
| [How-To Guides](https://alice-space.github.io/alice/en/how-to/install.html) | Task-focused recipes |
| [Configuration Reference](https://alice-space.github.io/alice/en/reference/configuration.html) | Every config key documented |
| [Architecture](https://alice-space.github.io/alice/en/development/architecture.html) | Code-level architecture |

[中文文档 »](https://alice-space.github.io/alice/zh/)

## Quick Start

```bash
npm install -g @alice_space/alice
alice setup
# edit ~/.alice/config.yaml
alice --feishu-websocket
```

## Development

```bash
make check   # fmt, vet, test, race
make build
make run
```

Contribution guide: [CONTRIBUTING.md](./CONTRIBUTING.md)

## License

[MIT](./LICENSE)
