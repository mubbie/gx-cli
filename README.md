# gx

[![CI](https://github.com/mubbie/gx-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/mubbie/gx-cli/actions/workflows/ci.yml)
[![GitHub Release](https://img.shields.io/github/v/release/mubbie/gx-cli)](https://github.com/mubbie/gx-cli/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Git Productivity Toolkit

- All destructive commands require confirmation and support `--dry-run`

## Install

**Homebrew** (macOS / Linux, recommended):

```
brew tap mubbie/tap
brew install gx-git
```

**Go:**

```
go install github.com/mubbie/gx-cli@latest
```

**pip / pipx** (Python 3.9+):

```
pipx install gx-git
```

**Binary download:**

Grab the latest binary from [GitHub Releases](https://github.com/mubbie/gx-cli/releases) and add it to your PATH.

## Commands

**Setup:**

| Command | Description |
|---------|-------------|
| [`gx init`](#gx-init) | Initialize gx stacking for this repo |

**Everyday:**

| Command | Description |
|---------|-------------|
| [`gx undo`](#gx-undo) | Smart undo: figures out what to undo |
| [`gx redo`](#gx-undo) | Redo the last undo |
| [`gx oops`](#gx-oops) | Quick-fix the last commit |
| [`gx switch`](#gx-switch) | Branch switcher with search |
| [`gx context`](#gx-context) | Repo status at a glance |
| [`gx sweep`](#gx-sweep) | Clean up merged branches and stale refs |
| [`gx shelf`](#gx-shelf) | Visual stash manager |

**Insight:**

| Command | Description |
|---------|-------------|
| [`gx who`](#gx-who) | Who knows this code best |
| [`gx recap`](#gx-recap) | What did I (or my team) do recently |
| [`gx drift`](#gx-drift) | How far you've diverged from the HEAD branch |
| [`gx conflicts`](#gx-conflicts) | Preview merge conflicts before merging |
| [`gx handoff`](#gx-handoff) | Branch summary for PRs, Slack, or standups |
| [`gx view`](#gx-view) | Quick status of your current stack |

**Stacking:**

| Command | Description |
|---------|-------------|
| [`gx stack`](#gx-stack) | Create a stacked branch with tracked parent |
| [`gx sync`](#gx-sync) | Rebase and push a chain of stacked branches |
| [`gx retarget`](#gx-retarget) | Rebase a branch onto a new base |
| [`gx graph`](#gx-graph) | Visualize the branch stack tree |
| `gx up` | Move up the stack (to child) |
| `gx down` | Move down the stack (to parent) |
| `gx top` | Jump to the top of the stack |
| `gx bottom` | Jump to the bottom of the stack |
| `gx parent` | Print the parent branch name (for scripting) |

**Utility:**

| Command | Description |
|---------|-------------|
| [`gx nuke`](#gx-nuke) | Delete branches with confidence |
| `gx update` | Update gx to the latest version |

---

## gx init

One-time setup: creates `.git/gx/`, configures the trunk branch, writes initial `stack.json`.

```
gx init                    # Auto-detect trunk branch
gx init --trunk develop    # Explicitly set trunk branch
gx init --force            # Re-initialize (preserves relationships)
```

```
$ gx init

Detected trunk branch: main

OK Initialized gx in this repo.
  Trunk: main
  Stack config: .git/gx/stack.json

  Get started:
    gx stack feature/my-thing main    Create your first stacked branch
    gx graph                          View your stack
```

Stacking commands still work without explicit `gx init`. If no `stack.json` exists, it auto-initializes silently. `gx init` is recommended but never required.

| Flag | Default | Description |
|------|---------|-------------|
| `--trunk` | auto-detected | Explicitly set the trunk branch |
| `--force` | false | Re-initialize (preserves relationships, resets metadata) |

---

## gx undo

Smart undo. Detects the last git action and reverses it by walking the reflog. Works regardless of whether the action was performed via `gx` or raw git commands.

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

Detected: commit "Add search endpoint" (a1b2c3d, 2 minutes ago)

  Action:  Soft reset to previous commit. Your changes will be preserved in staging.
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
| `--since` | | Only consider commits after this date |
| `--email` | false | Show email addresses |
| `--no-limit` | false | Remove the 200-file cap for directory analysis |

Directory-level analysis runs blame concurrently across files. Capped at 200 files by default to keep things fast.

---

## gx nuke

Delete branches (local, remote, and tracking refs).

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
| `--orphans` | false | Delete all orphaned branches from the stack graph |

**Safety:** Cannot nuke the current branch or the HEAD branch (main/master). Unmerged branches get a prominent warning with commit count. Warns if the branch has dependents in the stack.

---

## gx recap

Show what you (or someone else) did recently.

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

Detects squash-merged branches using `git cherry`. Branches where all patches already exist on the HEAD branch are flagged as "likely squash-merged" and confirmed separately.

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | false | Show what would be cleaned |
| `-y, --yes` | false | Skip confirmation prompts |

**Safety:** Never deletes the current branch or HEAD branch. Squash-merged branches are confirmed separately with lower-confidence framing.

---

## gx oops

Quick-fix the last commit. Amend the message, add forgotten files, or both.

```
gx oops                           # Opens editor to amend message
gx oops -m "Better message"       # Amend with new message inline
gx oops --add src/forgot.ts       # Add a forgotten file to last commit
gx oops --add src/forgot.ts -m "Updated message"  # Both at once
gx oops --dry-run                 # See what would change
```

```
$ gx oops --add src/auth/refresh.ts -m "Fix auth token refresh, include refresh util"

Last commit: "Fix auth token refresh" (a1b2c3d, 5 min ago)

  Adding to last commit:
    + src/auth/refresh.ts (modified, 12 lines changed)

  Amending message:
    Before: "Fix auth token refresh"
    After:  "Fix auth token refresh, include refresh util"

? Proceed? [y/N] y

✓ File added and commit message amended.
```

| Flag | Default | Description |
|------|---------|-------------|
| `-m, --message` | | New commit message (skips editor) |
| `--add` | | File(s) to add to the last commit |
| `--dry-run` | false | Show what would change |
| `--force` | false | Allow amending even if already pushed |

**Safety:** Refuses to amend if the last commit has been pushed to remote (override with `--force`).

---

## gx context

Enhanced repo status at a glance. Everything you need to know about your current state.

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
gx drift --parent     # Compare against stack parent (what the PR reviewer sees)
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
| `--parent, -p` | false | Compare against stack parent instead of main |

---

## gx switch

Branch switcher with search.

```
gx switch       # Interactive branch picker
gx switch -     # Switch to previous branch
```

Lists all local branches sorted by recent activity. Enter a number to switch, type text to filter the list, `q` to cancel.

```
$ gx switch

    1  feature/payments                          2h ago          kim
    2  fix/login-bug                             5h ago          you
    3  feature/auth-v2                           3d ago          james
    4  feature/search                            1d ago          you

Enter a number to switch, text to filter, q to cancel
> auth

    1  feature/auth-v2                           3d ago          james

Enter a number to switch, text to filter, q to cancel
> 1

✓ Switched to feature/auth-v2
```

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

✓ No conflicts. Clean merge.
  12 files would be modified
```

Uses `git merge-tree` to simulate the merge entirely in memory. Nothing is modified on disk.

---

## gx handoff

Generate a clean, copy-pasteable summary of your current branch (commits, files changed, stats) for PR descriptions, Slack, or standups.

```
gx handoff                  # Summary vs auto-detected base (stack parent or main)
gx handoff --against main   # Summary against a specific branch
gx handoff --copy           # Also copy to clipboard
gx handoff --markdown       # Markdown format for PR descriptions
```

Auto-detects the base: uses the stack parent if the branch is in `stack.json`, otherwise falls back to main.

**Plain text:**

```
$ gx handoff

feature/auth-v2 (on feature/auth-v1)

Commits (3):
  a1b2c3d  Add auth token validation
  d4e5f6g  Add auth middleware
  h7i8j9k  Scaffold auth module

4 files changed, +142 -38

Files:
  src/auth/middleware.ts
  src/auth/validation.ts
  src/auth/types.ts
  tests/auth/middleware.test.ts
```

**Markdown (`--markdown`):**

```
$ gx handoff --markdown

## feature/auth-v2
**Base:** feature/auth-v1 · **3 commits** · 4 files changed (+142 -38)

### Commits
- `a1b2c3d` Add auth token validation
- `d4e5f6g` Add auth middleware
- `h7i8j9k` Scaffold auth module

### Files Changed
- `src/auth/middleware.ts`
- `src/auth/validation.ts`
- `src/auth/types.ts`
- `tests/auth/middleware.test.ts`
```

| Flag | Default | Description |
|------|---------|-------------|
| `--against` | auto-detected | Compare against a specific branch |
| `--copy, -c` | false | Copy output to system clipboard |
| `--markdown, --md` | false | Format as markdown |

---

## gx view

Quick, focused view of the stack you're currently in. Lighter than `gx graph` (which shows all branches).

```
gx view
```

```
$ gx view

main
  <- feature/auth-v1          2 ahead   3h ago
  <- feature/auth-v2          3 ahead   1h ago    <
  <- feature/auth-v3          1 ahead   20m ago
```

Shows PR status if `gh` CLI is installed (omitted silently otherwise). On trunk, lists all stacks. On an untracked branch, shows info vs trunk and suggests `gx stack`.

No flags, no arguments. Just run it.

---

## gx shelf

Visual stash manager. Browse, preview diffs, and apply or drop stashes interactively.

```
gx shelf                      # Open interactive stash browser (TUI)
gx shelf push "message"       # Quick stash with a descriptive message
gx shelf push                 # Quick stash (auto-generates message)
gx shelf list                 # Non-interactive list
gx shelf clear                # Drop all stashes (requires confirmation)
gx shelf clear --dry-run      # Show what would be cleared
```

The interactive browser is a split-pane TUI (powered by [Textual](https://textual.textualize.io/)): stash list on the left, full diff preview on the right. Navigate with arrow keys, Enter to pop, Space to apply (keep stash), `d` to drop, `/` to search.

```
$ gx shelf list

3 stashes:

  #   Age           Branch              Message
  0   2 hours ago   feature/auth-v2     WIP on feature/auth-v2
  1   3 days ago    feature/search      Fix token refresh
  2   2 weeks ago   main                WIP on main
```

```
$ gx shelf push "Fix token refresh"

✓ Stashed working directory: "Fix token refresh"
  Run `gx shelf` to browse.
```

**Safety:** Pop/apply conflicts exit the TUI cleanly with resolution instructions. Drop requires confirmation.

---

## gx stack

Create a new branch on top of a parent branch, with the relationship tracked in `.git/gx/stack.json`.

```
gx stack feature/auth-v2 feature/auth-v1   # Direct usage
gx stack                                    # Interactive prompt
```

```
$ gx stack feature/auth-v2 feature/auth-v1

✓ Created feature/auth-v2 on top of feature/auth-v1
  Relationship saved to stack config.
```

In interactive mode, shows a searchable branch list for parent selection, then prompts for the new branch name.

---

## gx sync

Rebase and push a chain of stacked branches in sequence, keeping the entire stack up to date.

```
gx sync --stack                              # Auto-detect and sync full stack
gx sync main feature/auth-v1 feature/auth-v2 # Explicit chain
gx sync --dry-run --stack                    # Show what would happen
```

```
$ gx sync --stack

Auto-detected stack from config:
  main -> feature/auth-v1 -> feature/auth-v2

Syncing 2 branches...

  Rebasing stack onto main (using --update-refs)...
  ✓ Rebased feature/auth-v1
  ✓ Rebased feature/auth-v2

  Pushing updated branches...
  ✓ Pushed feature/auth-v1
  ✓ Pushed feature/auth-v2

✓ Stack sync complete. 2 branches updated.
```

Uses `git rebase --update-refs` on Git 2.38+ (single operation for the whole chain). Falls back to `--onto` iteration on older Git versions. Stops on first conflict with resolution instructions.

| Flag | Default | Description |
|------|---------|-------------|
| `--stack` | false | Auto-detect and sync the current branch's full stack |
| `--dry-run` | false | Show what would happen |

**Safety:** Uses `--force-with-lease` for pushes. Stops on first conflict. Confirms for chains of 5+ branches. Returns to original branch after sync.

---

## gx retarget

Rebase a branch onto a new base. Updates the stack config and attempts to auto-retarget the PR via `gh` CLI.

```
gx retarget feature/auth-v2 main        # Move feature/auth-v2 onto main
gx retarget feature/auth-v2 main --dry-run
```

```
$ gx retarget feature/auth-v2 main

Retargeting feature/auth-v2 onto main...

  Old parent: feature/auth-v1
  New parent: main

  Fetching latest from remote...
  Rebasing feature/auth-v2 onto origin/main (using --onto)...
  ✓ Rebased and pushed feature/auth-v2
  ✓ PR for feature/auth-v2 automatically retargeted to main

  Stack config updated: feature/auth-v2 -> main (was: feature/auth-v1)
```

Uses `git rebase --onto` to slice out only the branch's own commits, avoiding duplication of the old parent's commits. If `gh` CLI is installed and authenticated, auto-retargets the PR; otherwise shows a reminder.

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | false | Show what would happen |

---

## gx graph

Visualize the branch stack as a tree with health indicators.

```
gx graph
```

```
$ gx graph

Branch Stack:

|-- main
|   |-- feature/auth-v1       * HEAD  (+2/-0)
|   |   `-- feature/auth-v2            (+3/-1)
|   |-- feature/search                 + merged
|   `-- fix/login-bug                  (+1/-0)

Orphaned Branches:
`-- experiment/old-idea                ! orphaned

Legend: * current branch  + merged  (+ahead/-behind)  ! orphaned
Relationships stored in .git/gx/stack.json
```

The tree builder auto-discovers relationships for branches created outside of `gx` and saves them to the config, so the graph gets more accurate over time.

---

## Stack Navigation

Navigate up and down the stack without remembering branch names.

```
gx up         # Move to child branch (one step up)
gx down       # Move to parent branch (one step down)
gx top        # Jump to the tip of the stack
gx bottom     # Jump to the base of the stack
```

```
$ gx up
OK Moved up: feature/auth-v1 -> feature/auth-v2

$ gx down
OK Moved down: feature/auth-v2 -> feature/auth-v1

$ gx top
OK Jumped to top: feature/auth-v1 -> feature/auth-v4

$ gx bottom
OK Jumped to bottom: feature/auth-v4 -> feature/auth-v1
```

When a branch has multiple children (fork in the stack), these commands list the options and suggest `gx switch`.

---

## gx parent

Print the parent branch name to stdout. No formatting, no Rich. Just a raw string for composability.

```
$ gx parent
feature/auth-v1
```

Falls back to the HEAD branch (main) if the branch isn't in the stack. Exits with non-zero if already on main.

Useful as a building block:

```bash
git diff $(gx parent)...HEAD        # diff against parent
git log $(gx parent)..HEAD          # commits unique to your branch
git rebase $(gx parent)             # manual rebase
```

---

## Tech Stack

**Go (Homebrew / binary):**
- [Cobra](https://github.com/spf13/cobra): CLI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss): Terminal styling

**Python (pip / pipx):**
- [Typer](https://typer.tiangolo.com/): CLI framework
- [Rich](https://rich.readthedocs.io/): Terminal formatting
- [Textual](https://textual.textualize.io/): TUI for interactive stash browser

## License

[MIT](LICENSE)
