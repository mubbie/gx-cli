# gx

[![CI](https://github.com/mubbie/gx-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/mubbie/gx-cli/actions/workflows/ci.yml)
[![GitHub Release](https://img.shields.io/github/v/release/mubbie/gx-cli)](https://github.com/mubbie/gx-cli/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Git Productivity Toolkit. 25 commands for everyday git, stacked PRs, and code insight.

**[Documentation](https://gx.mubbie.dev)** | **[Releases](https://github.com/mubbie/gx-cli/releases)**

## Install

```bash
# Homebrew (macOS / Linux)
brew tap mubbie/tap && brew install gx-git

# Go
go install github.com/mubbie/gx-cli@latest

# pip / pipx (Python)
pipx install gx-git

# Or download a binary from GitHub Releases
```

## Commands

```
gx: Git Productivity Toolkit

Setup:
  init

Everyday:
  undo, redo, oops, switch, context, sweep, shelf

Insight:
  who, recap, drift, conflicts, handoff, view

Stacking:
  stack, sync, retarget, graph, up, down, top, bottom, parent

Utility:
  nuke, update
```

## Quick Start

```bash
gx context                    # Where am I?
gx switch                     # Pick a branch
gx undo                       # Undo the last git action
gx oops -m "better message"   # Fix the last commit
gx who                        # Who knows this code
gx shelf push "wip"           # Stash with a name
```

Stacked PRs:

```bash
gx stack feature/auth main           # Create a tracked branch
gx stack feature/tests feature/auth  # Stack another on top
gx graph                             # Visualize the stack
gx sync --stack                      # Rebase and push the chain
gx up / gx down                     # Navigate the stack
gx handoff --markdown --copy         # Generate PR summary
```

See the **[full documentation](https://gx.mubbie.dev)** for detailed usage, flags, and examples for every command.

## Contributing

1. Fork the repo
2. Create a feature branch (`gx stack feature/my-thing main`)
3. Make your changes
4. Run `go build && go vet ./...`
5. Open a PR

## License

[MIT](LICENSE)
