# engram-monitor-tui

[![CI](https://github.com/svg153/engram-monitor-tui/actions/workflows/ci.yml/badge.svg)](https://github.com/svg153/engram-monitor-tui/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/svg153/engram-monitor-tui)](https://github.com/svg153/engram-monitor-tui/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.18%2B-00ADD8?logo=go)](https://golang.org)

`engram-monitor-tui` is a **Go CLI + TUI** for inspecting and administering a local [Engram](https://github.com/Gentleman-Programming/engram) memory server without relying on the web dashboard.

It is the terminal sibling of the current `engram-monitor` web UI: same HTTP boundary, similar feature surface, but optimized for keyboard-driven workflows and shell automation.

## Features

- **Interactive TUI** with dashboard, sessions, memories, topics, timeline, prompts, and empty sessions
- **Session and topic drill-down** from the terminal
- **Observation editing** for `title`, `content`, `type`, `scope`, and `topic_key`
- **Admin actions** for deleting prompts and empty sessions
- **JSON export/import**
- **Project merge** support
- **Direct CLI commands** for automation and quick inspection

## Install

### Build locally

```bash
git clone https://github.com/svg153/engram-monitor-tui.git
cd engram-monitor-tui
task build
./bin/engram-monitor-tui version
```

### Install into your user PATH

```bash
task install
engram-monitor-tui version
```

By default `task install` writes the binary to:

```bash
$HOME/.local/bin/engram-monitor-tui
```

## Requirements

- Go `1.18+`
- a running Engram server, normally at `http://127.0.0.1:7437`

## Quick start

### 1. Open the TUI

```bash
./bin/engram-monitor-tui tui --addr http://127.0.0.1:7437
```

### 2. List projects from the CLI

```bash
./bin/engram-monitor-tui projects list
```

### 3. Search memories from scripts

```bash
./bin/engram-monitor-tui memories search --query "topic key" --json
```

### 4. Export current data

```bash
./bin/engram-monitor-tui export --out ./engram-export.json
```

## Commands

### TUI

```bash
./bin/engram-monitor-tui tui --addr http://127.0.0.1:7437
```

Keybindings:

- `1..7` switch views
- `tab` / `shift+tab` move focus
- `j/k` or arrows navigate
- `/` focus search
- `p` cycle project filter
- `t` cycle type filter
- `o` cycle scope filter
- `enter` drill into sessions/topics
- `esc` go back / close active mode
- `e` edit selected observation
- `d` delete selected prompt or empty session
- `T` load session timeline for the selected observation
- `x` export JSON
- `i` import JSON
- `m` merge projects
- `r` reload
- `q` quit

### Direct CLI commands

```bash
./bin/engram-monitor-tui health --json
./bin/engram-monitor-tui projects list
./bin/engram-monitor-tui sessions list --project my-project
./bin/engram-monitor-tui prompts list --json
./bin/engram-monitor-tui memories search --query "decision" --type architecture
./bin/engram-monitor-tui export --out ./engram-export.json
./bin/engram-monitor-tui merge-projects --from old-name --to new-name
```

These commands are intended for:

- shell automation
- quick inspection without opening the TUI
- piping into `jq`, CI jobs, or scripts via `--json`

## Development

Task targets:

```bash
task fmt
task vet
task test
task coverage-html
task build
task ci
```

## Project structure

```text
cmd/engram-monitor-tui/    CLI entrypoint
internal/api/              Engram HTTP client + service contract
internal/app/              Bubble Tea state, update loop, rendering
internal/cli/              Direct command runner for automation
internal/model/            Domain and derived view models
docs/                      GitHub Pages content
```

## Security

If you find a security issue, please do not open a public issue. Use the repository's security advisory flow or private contact first.

## Notes

- The TUI derives sessions, topics, and timeline groupings client-side from Engram observations, following the same broad approach used by the current monitor.
- Import preflight currently reports possible duplicates by existing observation/prompt IDs. It does not yet replicate every heuristic from the web monitor import validation flow.
