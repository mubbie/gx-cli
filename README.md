# gx

[![CI](https://github.com/mubbie/gx-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/mubbie/gx-cli/actions/workflows/ci.yml)
[![GitHub Release](https://img.shields.io/github/v/release/mubbie/gx-cli)](https://github.com/mubbie/gx-cli/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Git Productivity Toolkit. All destructive commands require confirmation and support `--dry-run`.

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
| [`gx shelf`](#gx-shelf) | Stash manager |

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

OK Initialized gx in this repo.
  Trunk: main
  Stack config: .git/gx/stack.json

  Get started:
    gx stack feature/my-thing main    Create your first stacked branch
    gx graph                          View your stack
```

Stacking commands still work without explicit `gx init`. If no `stack.json` exists, it auto-initializes silently.

| Flag | Default | Description |
|------|---------|-------------|
| `--trunk` | auto-detected | Explicitly set the trunk branch |
| `--force` | false | Re-initialize (preserves relationships, resets metadata) |

---

## gx undo

Smart undo. Detects the last git action and reverses it by walking the reflog.

```
gx undo              # Undo the last thing
gx redo              # Redo (undo the undo)
gx undo --dry-run    # See what it would do
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

  Action:   Soft reset to previous commit. Your changes will be preserved in staging.
  Command:  git reset --soft HEAD~1

? Proceed with undo? [y/N] y

OK Undone. Your changes from that commit are now staged.
> Run `gx redo` to reverse this.
```

---

## gx who

Show who knows a file or repo best. Deduplicates authors by email, email username, and name.

```
gx who                    # Top contributors to the entire repo
gx who -n 10              # Show top 10 contributors
gx who --since 6months    # Only consider recent contributions
```

```
$ gx who

Top contributors

 #  Author       Email                           Commits  Last Active
 ---------------------------------------------------------------
 1  You          kim@work.com, kim@personal.com  234      2 hours ago
 2  James Wilson james@company.com               189      1 day ago
 3  Alex Kim     alex@company.com                 98      3 days ago

You: Kim Choi <kim@work.com>
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n, --number` | 5 | Number of contributors to show |
| `--since` | | Only consider commits after this date |
| `--no-limit` | false | Remove the 200-file cap for directory analysis |

---

## gx nuke

Delete branches (local, remote, and tracking refs).

```
gx nuke feature/old-thing          # Delete local + remote + tracking
gx nuke feature/old-thing --local  # Delete local only
gx nuke feature/old-thing --dry-run
gx nuke "feature/*"                # Glob pattern support
gx nuke --orphans                  # Delete orphaned branches from the stack
```

```
$ gx nuke feature/old-auth

  feature/old-auth is NOT merged into main.

WARN feature/old-auth has 2 dependent branches. They will become orphaned.

? Proceed with deletion? [y/N] y

OK Deleted local branch feature/old-auth
OK Deleted remote tracking ref origin/feature/old-auth
OK Deleted remote branch origin/feature/old-auth
```

| Flag | Default | Description |
|------|---------|-------------|
| `--local` | false | Only delete local branch |
| `--dry-run` | false | Show what would be deleted |
| `-y, --yes` | false | Skip confirmation |
| `--orphans` | false | Delete all orphaned branches from the stack |

**Safety:** Cannot nuke the current branch or the HEAD branch.

---

## gx recap

Show what you (or someone else) did recently.

```
gx recap                 # Your commits in the last 24 hours
gx recap @kim            # Kim's commits
gx recap --all           # Whole team's activity
gx recap -d 7            # Last 7 days
```

```
$ gx recap -d 7

Your activity in the last 7 days (my-project):

  Today:
    a1b2c3d  14:32  Add search endpoint
    d4e5f6g  11:15  Add search index util

  Yesterday:
    m1n2o3p  17:45  Fix pagination bug

  3 commits
```

| Flag | Default | Description |
|------|---------|-------------|
| `@<name>` | current git user | Filter by author |
| `-d, --days` | 1 | Number of days to look back |
| `--all` | false | Show all contributors |
| `--limit` | 100 | Max commits to display |

---

## gx sweep

Clean up merged branches and stale refs.

```
gx sweep              # Interactive cleanup
gx sweep --dry-run    # See what would be cleaned
gx sweep -y           # Auto-confirm
```

```
$ gx sweep

Scanning for cleanup opportunities...

Merged branches (safe to delete):
  feature/auth-v1
  fix/typo-readme

Likely squash-merged branches:
  feature/onboarding

Summary: 2 merged, 1 likely squash-merged, 0 stale refs

? Delete merged branches? [y/N] y
OK Deleted feature/auth-v1
OK Deleted fix/typo-readme
OK Cleanup complete.
```

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | false | Show what would be cleaned |
| `-y, --yes` | false | Skip confirmation prompts |

---

## gx oops

Quick-fix the last commit. Amend the message, add forgotten files, or both.

```
gx oops                           # Opens editor to amend message
gx oops -m "Better message"       # Amend with new message
gx oops --add src/forgot.ts       # Add a forgotten file
gx oops --add src/forgot.ts -m "Updated message"
gx oops --dry-run
```

```
$ gx oops --add src/auth/refresh.ts -m "Fix auth token refresh"

Last commit: "Fix auth token" (a1b2c3d)

  Adding to last commit:
    + src/auth/refresh.ts

  Amending message:
    Before: "Fix auth token"
    After:  "Fix auth token refresh"

? Proceed? [y/N] y

OK File added and commit message amended.
```

| Flag | Default | Description |
|------|---------|-------------|
| `-m, --message` | | New commit message |
| `--add` | | File(s) to add to the last commit |
| `--dry-run` | false | Show what would change |
| `--force` | false | Allow amending even if already pushed |

**Safety:** Refuses to amend if the last commit has been pushed (override with `--force`).

---

## gx context

Repo status at a glance.

```
gx context     # Full context summary
gx ctx         # Alias
```

```
$ gx context

Branch:  feature/search
Tracking:  origin/feature/search (up to date)
vs main:  3 ahead, 2 behind

Last commit:  a1b2c3d "Add search endpoint" (2 hours ago)

Working tree:
  Modified:  3 files
  Staged:  1 file
  Untracked:  2 files

Stash:  2 entries

WARN Rebase in progress
```

---

## gx drift

Show how far your branch has diverged from the HEAD branch.

```
gx drift              # Compare against HEAD branch
gx drift develop      # Compare against a specific branch
gx drift --full       # Show all commits (no truncation)
gx drift --parent     # Compare against stack parent
```

```
$ gx drift

feature/search is 3 ahead, 2 behind main

Commits on your branch (not on main):
  a1b2c3d  2h ago       Add search endpoint
  d4e5f6g  5h ago       Add search index util
  h7i8j9k  1d ago       Scaffold search module

Commits on main (not on your branch):
  x1y2z3a  3h ago       Fix auth token refresh
  b4c5d6e  1d ago       Update CI pipeline

Files diverged: 8 files changed, 142 insertions(+), 38 deletions(-)
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

```
$ gx switch

    1  feature/payments                          2h ago          kim
    2  fix/login-bug                             5h ago          you
    3  feature/auth-v2                           3d ago          james

Enter a number to switch, text to filter, q to cancel
> auth

    1  feature/auth-v2                           3d ago          james

Enter a number to switch, text to filter, q to cancel
> 1

OK Switched to feature/auth-v2
```

---

## gx conflicts

Preview merge conflicts before merging, without touching the working tree.

```
gx conflicts              # Check against HEAD branch
gx conflicts develop      # Check against a specific branch
```

```
$ gx conflicts

Checking feature/search against main...

X 3 conflicts found

  src/api/auth.ts          (you + kim)
  src/utils/helpers.ts     (you + james)
  package.json

  14 other files merge cleanly
```

Uses `git merge-tree` to simulate the merge in memory. Falls back to the 3-arg form on Git < 2.38.

---

## gx handoff

Generate a branch summary for PRs, Slack, or standups.

```
gx handoff                  # Summary vs auto-detected base
gx handoff --against main   # Summary against a specific branch
gx handoff --copy           # Also copy to clipboard
gx handoff --markdown       # Markdown format
```

Auto-detects the base: uses the stack parent if the branch is in `stack.json`, otherwise falls back to main.

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

| Flag | Default | Description |
|------|---------|-------------|
| `--against` | auto-detected | Compare against a specific branch |
| `--copy, -c` | false | Copy to system clipboard |
| `--markdown, --md` | false | Format as markdown |

---

## gx view

Quick status of your current stack. Lighter than `gx graph`.

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

Shows PR status if `gh` CLI is installed. On trunk, lists all stacks.

---

## gx shelf

Stash manager with push, list, and clear.

```
gx shelf                      # List stashes (same as gx shelf list)
gx shelf push "message"       # Stash with a descriptive message
gx shelf push                 # Stash with auto-generated message
gx shelf list                 # Non-interactive list
gx shelf clear                # Drop all stashes (requires confirmation)
gx shelf clear --dry-run
```

```
$ gx shelf list

3 stashes:

 #  Age          Branch              Message
 ──────────────────────────────────────────────────────
 0  2 hours ago  feature/auth-v2     WIP on feature/auth-v2
 1  3 days ago   feature/search      Fix token refresh
 2  2 weeks ago  main                WIP on main
```

```
$ gx shelf push "Fix token refresh"

OK Stashed working directory: "Fix token refresh"
  Run `gx shelf` to browse.
```

---

## gx stack

Create a new branch on top of a parent branch, with the relationship tracked in `.git/gx/stack.json`.

```
gx stack feature/auth-v2 feature/auth-v1
```

```
$ gx stack feature/auth-v2 feature/auth-v1

OK Created feature/auth-v2 on top of feature/auth-v1
  Relationship saved to stack config.
```

---

## gx sync

Rebase and push a chain of stacked branches in sequence.

```
gx sync --stack                              # Auto-detect and sync full stack
gx sync main feature/auth-v1 feature/auth-v2 # Explicit chain
gx sync --dry-run --stack
```

```
$ gx sync --stack

Syncing stack: main -> feature/auth-v1 -> feature/auth-v2

  Rebasing stack onto main (using --update-refs)...
  OK Rebased feature/auth-v1
  OK Rebased feature/auth-v2

  Pushing updated branches...
  OK Pushed feature/auth-v1
  OK Pushed feature/auth-v2

OK Stack sync complete. 2 branches updated.
```

Uses `--update-refs` on Git 2.38+ (single operation). Falls back to `--onto` on older Git.

| Flag | Default | Description |
|------|---------|-------------|
| `--stack` | false | Auto-detect and sync the current branch's full stack |
| `--dry-run` | false | Show what would happen |

**Safety:** Uses `--force-with-lease` for pushes. Stops on first conflict. Confirms for chains of 5+ branches.

---

## gx retarget

Rebase a branch onto a new base. Updates stack config and attempts to auto-retarget the PR via `gh`.

```
gx retarget feature/auth-v2 main
gx retarget feature/auth-v2 main --dry-run
```

```
$ gx retarget feature/auth-v2 main

Retargeting feature/auth-v2 onto main...

  Old parent: feature/auth-v1
  New parent: main

  Fetching latest from remote...
  Rebasing feature/auth-v2 onto origin/main (using --onto)...
  OK Rebased and pushed feature/auth-v2
  OK PR for feature/auth-v2 automatically retargeted to main

  Stack config updated: feature/auth-v2 -> main (was: feature/auth-v1)
```

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | false | Show what would happen |

---

## gx graph

Visualize the branch stack tree.

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

Auto-discovers relationships for branches created outside `gx`.

---

## Stack Navigation

Navigate the stack without remembering branch names.

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

When a branch has multiple children, lists the options and suggests `gx switch`.

---

## gx parent

Print the parent branch name to stdout. No formatting, just a raw string for composability.

```
$ gx parent
feature/auth-v1
```

Falls back to the HEAD branch if not in the stack. Exits with non-zero on main.

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
