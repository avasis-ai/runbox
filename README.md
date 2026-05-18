# Runbox

Turn any remote machine into an agent runtime.

Run Codex, Claude Code, OpenClaw, tests, builds, and long-running jobs on your Mac mini, GPU PC, workstation, or edge box — without SSH pain.

[![Sponsor](https://img.shields.io/badge/Sponsor-%E2%9D%A4-%23ea4aaa?logo=github)](https://github.com/sponsors/avasis-ai) [![Release](https://img.shields.io/github/v/release/avasis-ai/runbox?include_prereleases)](https://github.com/avasis-ai/runbox/releases) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

```
runbox init mini --host mini --user abhay --workdir ~/project
runbox doctor mini
runbox fix mini --all
runbox exec mini "pytest -q"
runbox agent run mini codex
```

## Why

Developers are using remote machines as always-on AI worker boxes. But the workflow is still:

```bash
ssh user@100.x.y.z          # what was the IP again?
tmux attach                  # which session?
cd ~/repo && source env      # every single time
```

For humans, this is annoying. For CLI agents (Codex, Claude Code, OpenClaw), it's broken.

**Runbox fixes this.** One config, zero passwords, persistent sessions.

## Install

One-line install:

```bash
curl -fsSL https://raw.githubusercontent.com/avasis-ai/runbox/main/install.sh | bash
```

Or with Go:

```bash
go install github.com/avasis-ai/runbox/cmd/runbox@latest
```

Or [download a binary](https://github.com/avasis-ai/runbox/releases) directly (macOS/Linux, arm64/amd64).

Or build from source:

```bash
git clone https://github.com/avasis-ai/runbox.git
cd runbox
go build -o /usr/local/bin/runbox ./cmd/runbox/
```

## Quick Start

```bash
# Register a machine
runbox init mini --host mini --user abhay --workdir ~/projects/main

# Diagnose what's wrong
runbox doctor mini

# Fix everything automatically
runbox fix mini --all

# Run a command
runbox exec mini "pytest -q"

# Open a shell
runbox shell mini

# Run an agent
runbox agent add codex --command codex
runbox agent run mini codex

# Sync files
runbox sync mini ./repo ~/repo

# View logs
runbox logs mini
```

## Commands

### Machine Lifecycle

| Command | Description |
|---------|-------------|
| `runbox init <name>` | Register a remote machine |
| `runbox list` | List registered machines |
| `runbox info <name>` | Show machine details |
| `runbox remove <name>` | Remove a machine |

### Diagnostics & Fixes

| Command | Description |
|---------|-------------|
| `runbox doctor` | Check local dependencies |
| `runbox doctor <name>` | Full diagnostic for a machine |
| `runbox fix <name> --all` | Apply all recommended fixes |
| `runbox fix <name> --auth` | Set up passwordless SSH |
| `runbox fix <name> --ssh-config` | Generate SSH config entry |
| `runbox fix <name> --multiplex` | Enable SSH multiplexing |
| `runbox fix <name> --remote-runtime` | Create remote dirs |

### Execution

| Command | Description |
|---------|-------------|
| `runbox exec <name> "cmd"` | Execute a remote command |
| `runbox exec <name> --json "cmd"` | Execute with JSON output |
| `runbox shell <name>` | Open interactive shell |

### Sessions

| Command | Description |
|---------|-------------|
| `runbox session create <name> <session>` | Create tmux session |
| `runbox session attach <name> <session>` | Attach to session |
| `runbox session list <name>` | List sessions |
| `runbox session exec <name> <session> "cmd"` | Send command to session |
| `runbox session kill <name> <session>` | Kill session |

### Agents

| Command | Description |
|---------|-------------|
| `runbox agent add <name> --command <cmd>` | Register agent profile |
| `runbox agent list` | List agent profiles |
| `runbox agent run <machine> <agent>` | Run agent in tmux |
| `runbox agent run <machine> <agent> --detached` | Run in background |

### Sync & Logs

| Command | Description |
|---------|-------------|
| `runbox sync <name> <local> <remote>` | Push files to remote |
| `runbox pull <name> <remote> <local>` | Pull files from remote |
| `runbox logs <name>` | View logs |
| `runbox logs <name> <session>` | View session output |
| `runbox logs <name> --tail` | Tail recent output |

## Configuration

`~/.runbox/config.yaml`:

```yaml
version: 1
machines:
  mini:
    host: mini
    user: abhay
    port: 22
    workdir: ~/projects/main
    transport: ssh
    auth: ssh-agent
    multiplex: true
  edgebox:
    host: edgebox
    user: ubuntu
    port: 22
    workdir: ~/ojas
    transport: ssh
    auth: tailscale-ssh
    multiplex: true
agents:
  codex:
    command: codex
    cwd: ~/projects/main
  claude:
    command: claude
    cwd: ~/projects/main
  openclaw:
    command: openclaw
    default_args: ["start"]
    cwd: ~/openclaw
```

## How It Works

Runbox is built on boring, trusted primitives:

- **SSH** — transport layer
- **Tailscale / MagicDNS** — hostnames instead of IPs
- **ssh-agent** — no repeated passwords
- **OpenSSH ControlMaster** — fast repeated commands
- **tmux** — persistent sessions
- **rsync** — file sync

No custom daemon. No VPN. No hosted relay. Your machines, your network.

## Safety

- Never overwrites SSH config outside marked `BEGIN RUNBOX` / `END RUNBOX` blocks
- Never creates passwordless keys silently
- Policy engine blocks dangerous commands (`rm -rf /`, fork bombs)
- Requires approval for `sudo`, `rm -rf`, `git push`, `docker system prune`

## Built With

- [Go](https://go.dev) — single static binary, cross-platform
- [Cobra](https://github.com/spf13/cobra) — CLI framework
- [YAML](https://github.com/go-yaml/yaml) — config

## License

MIT

---

## Support Runbox

If Runbox saves you from SSH pain, consider supporting its development.

[![Sponsor](https://img.shields.io/badge/Sponsor-%E2%9D%A4-%23ea4aaa?logo=github)](https://github.com/sponsors/avasis-ai)

Your sponsorship funds: remote daemon, web UI, team mode, fleet management, MCP server, and more. See [SPONSORS.md](SPONSORS.md) for details.

*Building open-source tools takes real work. Your support keeps them free for everyone.*
