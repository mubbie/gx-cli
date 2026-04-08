# gx — Git Productivity Toolkit

[![CI](https://github.com/mubbie/gx-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/mubbie/gx-cli/actions/workflows/ci.yml)
[![PyPI](https://img.shields.io/pypi/v/gx-git)](https://pypi.org/project/gx-git/)
[![Python](https://img.shields.io/pypi/pyversions/gx-git)](https://pypi.org/project/gx-git/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A terminal-based Git utility that bundles 10 focused productivity commands into a single CLI. Each command solves a specific daily friction point that currently requires multiple git commands, obscure flags, or manual effort.

- Each command does one thing well
- All destructive commands require confirmation and support `--dry-run`
- Output is human-friendly with colors, tables, and clear formatting
- Zero configuration required — works out of the box in any git repo
- No external services or authentication needed — everything runs locally

## Install

**Recommended** (isolated environment via [pipx](https://pipx.pypa.io/)):

```
pipx install gx-git
```

**Alternative** (via pip):

```
pip install gx-git
```

Requires Python 3.9+.

## Commands

| Command | Description |
|---------|-------------|
| [`gx undo`](#gx-undo) | Smart undo — figures out what to undo |
| [`gx redo`](#gx-undo) | Redo the last undo |
| [`gx who`](#gx-who) | Who knows this code best |
| [`gx nuke`](#gx-nuke) | Delete branches with confidence |
| [`gx recap`](#gx-recap) | What did I (or my team) do recently |
| [`gx sweep`](#gx-sweep) | Clean up merged branches and stale refs |
| [`gx oops`](#gx-oops) | Quick-fix the last commit |
| [`gx context`](#gx-context) | Repo status at a glance |
| [`gx drift`](#gx-drift) | How far you've diverged from the HEAD branch |
| [`gx switch`](#gx-switch) | Fuzzy branch switcher |
| [`gx conflicts`](#gx-conflicts) | Preview merge conflicts before merging |

---

## gx undo

Smart undo — detects the last git action and reverses it by walking the reflog. Works regardless of whether the action was performed via `gx` or raw git commands.

```
gx undo              # Undo the last thing
gx redo              # Redo (undo the undo)
gx undo --dry-run    # See what it would do without doing it
gx undo --history    # See your undo/redo history
```

Detects and reverses, in priority order:

| State | Undo Action |
|-------|-------------|
| Active merge conflict | `git merge --abort` |
| Rebase in progress | `git rebase --abort` |
| Staged files | `git reset HEAD` (unstages all) |
| Amended commit | `git reset --soft HEAD@{1}` |
| Merge commit | `git reset --hard HEAD~1` |
| Regular commit | `git reset --soft HEAD~1` (keeps changes staged) |

```
$ gx undo

🔍 Detected: commit "Add search endpoint" (a1b2c3d, 2 minutes ago)

  Action:  Soft reset to previous commit — your changes will be preserved in staging.
  Command: git reset --soft HEAD~1

? Proceed with undo? [y/N] y

✓ Undone. Your changes from that commit are now staged.
  Run `gx redo` to reverse this.
```

Undo/redo state is tracked via reflog entries and a supplemental history file, so it survives app restarts and works even if you run raw git commands in between.

---

## gx who

Show who knows a file, directory, or repo best based on git blame and log data.

```
gx who                    # Top contributors to the entire repo
gx who src/index.js       # Who knows this file best
gx who src/               # Who knows this directory best
gx who -n 10              # Show top 10 contributors
gx who --since 6months    # Only consider recent contributions
```

```
$ gx who src/auth/login.ts

Ownership of src/auth/login.ts (142 lines):

  #   Author              Lines    %      Last Edit
  1   Kim Choi          68       47.9%  3 days ago
  2   You                 42       29.6%  1 hour ago
  3   James Wilson        32       22.5%  2 weeks ago
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n, --number` | 5 | Number of contributors to show |
| `--since` | — | Only consider commits after this date |
| `--email` | false | Show email addresses |
| `--no-limit` | false | Remove the 200-file cap for directory analysis |

Directory-level analysis runs blame concurrently across files. Capped at 200 files by default to keep things fast.

---

## gx nuke

Delete branches with confidence — local, remote, and tracking refs.

```
gx nuke feature/old-thing          # Delete local + remote + tracking
gx nuke feature/old-thing --local  # Delete local only
gx nuke feature/old-thing --dry-run  # See what would be deleted
gx nuke "feature/*"                # Glob pattern support
```

```
$ gx nuke feature/old-auth

Branch: feature/old-auth

  ✗ Local branch          (last commit: 3 weeks ago, "Fix token refresh")
  ✗ Remote tracking ref   (origin/feature/old-auth)
  ✗ Remote branch         (origin)

  This branch is NOT merged into main.
  ⚠ You may lose 4 commits.

? Proceed with deletion? [y/N] y

✓ Deleted local branch feature/old-auth
✓ Deleted remote tracking ref origin/feature/old-auth
✓ Deleted remote branch origin/feature/old-auth
```

| Flag | Default | Description |
|------|---------|-------------|
| `--local` | false | Only delete local branch |
| `--dry-run` | false | Show what would be deleted |
| `-y, --yes` | false | Skip confirmation (still blocked for unmerged + HEAD branch) |

**Safety:** Cannot nuke the current branch or the HEAD branch (main/master). Unmerged branches get a prominent warning with commit count.

---

## gx recap

Show what you (or someone else) did recently across one or more repos.

```
gx recap                 # Your commits in the last 24 hours
gx recap @kim          # Kim's commits in the last 24 hours
gx recap --all           # Whole team's activity in the last 24 hours
gx recap -d 7            # Last 7 days
```

```
$ gx recap

Your activity in the last 24 hours (my-project):

  Today:
    a1b2c3d  14:32  Add search endpoint
    d4e5f6g  11:15  Add search index util
    h7i8j9k  09:03  Scaffold search module

  Yesterday:
    m1n2o3p  17:45  Fix pagination bug

  4 commits across 2 days
  8 files changed, +142 -38
```

| Flag | Default | Description |
|------|---------|-------------|
| `@<name>` | current git user | Filter by author (substring match) |
| `-d, --days` | 1 | Number of days to look back |
| `--all` | false | Show all contributors |
| `--limit` | 100 | Max commits to display |

---

## gx sweep

Clean up merged branches, prune stale remote tracking refs, and tidy up in one command.

```
gx sweep              # Interactive cleanup
gx sweep --dry-run    # See what would be cleaned
gx sweep -y           # Auto-confirm
```

```
$ gx sweep

Scanning for cleanup opportunities...

Merged branches (safe to delete):
  feature/auth-v1         merged 2 weeks ago
  fix/typo-readme         merged 3 days ago

Likely squash-merged branches:
  feature/onboarding      last commit 3 weeks ago (all patches found on main)

Stale remote tracking refs:
  origin/feature/deleted-branch

Summary: 2 merged, 1 likely squash-merged, 1 stale refs

? Delete merged branches? [y/N] y
? Delete likely squash-merged branches? [y/N] y
? Prune stale remote tracking refs? [y/N] y

✓ Cleanup complete.
```

Detects squash-merged branches using `git cherry` — branches where all patches already exist on the HEAD branch are flagged as "likely squash-merged" and confirmed separately.

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | false | Show what would be cleaned |
| `-y, --yes` | false | Skip confirmation prompts |

**Safety:** Never deletes the current branch or HEAD branch. Squash-merged branches are confirmed separately with lower-confidence framing.

---

## gx oops

Quick-fix the last commit — amend the message, add forgotten files, or both.

```
gx oops                           # Opens editor to amend message
gx oops -m "Better message"       # Amend with new message inline
gx oops --add src/forgot.ts       # Add a forgotten file to last commit
gx oops --add src/forgot.ts -m "Updated message"  # Both at once
gx oops --dry-run                 # See what would change
```

```
$ gx oops --add src/auth/refresh.ts -m "Fix auth token refresh — include refresh util"

Last commit: "Fix auth token refresh" (a1b2c3d, 5 min ago)

  Adding to last commit:
    + src/auth/refresh.ts (modified, 12 lines changed)

  Amending message:
    Before: "Fix auth token refresh"
    After:  "Fix auth token refresh — include refresh util"

? Proceed? [y/N] y

✓ File added and commit message amended.
```

| Flag | Default | Description |
|------|---------|-------------|
| `-m, --message` | — | New commit message (skips editor) |
| `--add` | — | File(s) to add to the last commit |
| `--dry-run` | false | Show what would change |
| `--force` | false | Allow amending even if already pushed |

**Safety:** Refuses to amend if the last commit has been pushed to remote (override with `--force`).

---

## gx context

Enhanced repo status at a glance — everything you need to know about your current state.

```
gx context     # Full context summary
gx ctx         # Alias
```

```
$ gx context

Branch:       feature/search
Tracking:     origin/feature/search (up to date)
vs main:      3 ahead, 2 behind

Last commit:  a1b2c3d "Add search endpoint" (2 hours ago)

Working tree:
  Modified:   3 files
  Staged:     1 file
  Untracked:  2 files

Stash:        2 entries

⚠ Rebase in progress (2/5 commits applied)
```

Shows: current branch, tracking status, ahead/behind HEAD branch, last commit, working tree status, stash count, and active operations (merge/rebase/cherry-pick).

---

## gx drift

Show how far your branch has diverged from the HEAD branch, with commit details and file stats.

```
gx drift              # Compare against HEAD branch
gx drift develop      # Compare against a specific branch
gx drift --full       # Show all commits (no truncation)
```

```
$ gx drift

feature/search is 3 ahead, 2 behind main

Commits on your branch (not on main):
  a1b2c3d  2h ago   Add search endpoint
  d4e5f6g  5h ago   Add search index util
  h7i8j9k  1d ago   Scaffold search module

Commits on main (not on your branch):
  x1y2z3a  3h ago   Fix auth token refresh (kim)
  b4c5d6e  1d ago   Update CI pipeline (james)

Files diverged: 8 modified, 2 added
```

| Flag | Default | Description |
|------|---------|-------------|
| `--full` | false | Show all commits (no truncation at 20) |

---

## gx switch

Fuzzy-find branch switcher with rich context.

```
gx switch       # Interactive fuzzy branch picker
gx switch -     # Switch to previous branch
```

Launches an interactive TUI (powered by [Textual](https://textual.textualize.io/)) showing all local branches with last commit date, author, and ahead/behind counts (loaded asynchronously). Type to fuzzy-filter, arrow keys to navigate, Enter to select.

```
$ gx switch

  feature/payments    3 ahead, 1 behind   2h ago    kim
  fix/login-bug       1 ahead, 0 behind   5h ago    you
  feature/auth-v2     12 ahead, 8 behind   3d ago    james
> feature/search      0 ahead, 0 behind   1d ago    you

Search: sea█

↑↓ Navigate  Enter Select  Esc Cancel
```

Falls back to a simple numbered list picker in non-interactive terminals.

---

## gx conflicts

Preview merge conflicts before actually merging, without touching the working tree.

```
gx conflicts              # Check against HEAD branch
gx conflicts develop      # Check against a specific branch
```

```
$ gx conflicts

Checking feature/search against main...

✗ 3 conflicts found

  src/api/auth.ts          (you + kim)
  src/utils/helpers.ts     (you + james)
  package.json             (dependency versions)

  14 other files merge cleanly
```

```
$ gx conflicts

Checking feature/search against main...

✓ No conflicts — clean merge
  12 files would be modified
```

Uses `git merge-tree` to simulate the merge entirely in memory. Nothing is modified on disk.

---

## Tech Stack

- [Typer](https://typer.tiangolo.com/) — CLI framework
- [Rich](https://rich.readthedocs.io/) — Terminal formatting (colors, tables, spinners)
- [Textual](https://textual.textualize.io/) — TUI components (fuzzy branch picker)

## License

[MIT](LICENSE)
