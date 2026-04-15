# gx

[![CI](https://github.com/mubbie/gx-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/mubbie/gx-cli/actions/workflows/ci.yml)
[![GitHub Release](https://img.shields.io/github/v/release/mubbie/gx-cli)](https://github.com/mubbie/gx-cli/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Git Productivity Toolkit. 25 commands for everyday git friction, stacked PRs, branch management, and code insight. Works in any git repo, no config required.

All destructive commands require confirmation and support `--dry-run`.

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

Grab the latest binary from [GitHub Releases](https://github.com/mubbie/gx-cli/releases) for macOS (ARM/Intel), Linux (x64/ARM), or Windows (x64/ARM).

## Quick Start

```bash
gx context                    # See where you are
gx switch                     # Pick a branch interactively
gx undo                       # Undo the last git action
gx oops -m "better message"   # Fix the last commit message
gx who                        # Who knows this repo best
gx shelf push "wip"           # Stash with a name you'll remember
```

For stacked PRs:

```bash
gx init                       # One-time setup (optional, auto-detects)
gx stack feature/auth main    # Create a branch tracked in the stack
gx stack feature/tests feature/auth   # Stack another on top
gx graph                      # Visualize the stack
gx sync --stack               # Rebase and push the whole chain
gx up / gx down               # Navigate the stack
```

## Commands

**Setup:**

| Command | Description |
|---------|-------------|
| [`gx init`](#gx-init) | Initialize gx stacking for this repo |

**Everyday:**

| Command | Description |
|---------|-------------|
| [`gx undo`](#gx-undo) | Smart undo: detects and reverses the last git action |
| [`gx redo`](#gx-undo) | Redo the last undo |
| [`gx oops`](#gx-oops) | Amend the last commit (message, files, or both) |
| [`gx switch`](#gx-switch) | Interactive branch picker with search |
| [`gx context`](#gx-context) | Branch, tracking, working tree, stash at a glance |
| [`gx sweep`](#gx-sweep) | Delete merged branches and prune stale refs |
| [`gx shelf`](#gx-shelf) | Stash manager (interactive, push, list, clear) |

**Insight:**

| Command | Description |
|---------|-------------|
| [`gx who`](#gx-who) | Top contributors by commit count (deduped by email) |
| [`gx recap`](#gx-recap) | Your (or your team's) recent activity |
| [`gx drift`](#gx-drift) | How far you've diverged from main (or stack parent) |
| [`gx conflicts`](#gx-conflicts) | Preview merge conflicts without merging |
| [`gx handoff`](#gx-handoff) | Generate a PR/Slack/standup summary |
| [`gx view`](#gx-view) | Quick view of your current stack |

**Stacking:**

| Command | Description |
|---------|-------------|
| [`gx stack`](#gx-stack) | Create a branch with tracked parent relationship |
| [`gx sync`](#gx-sync) | Rebase and push a stacked branch chain |
| [`gx retarget`](#gx-retarget) | Move a branch to a new base |
| [`gx graph`](#gx-graph) | Visualize the branch stack tree |
| [`gx up`](#stack-navigation) | Move to child branch (one step up the stack) |
| [`gx down`](#stack-navigation) | Move to parent branch (one step down) |
| [`gx top`](#stack-navigation) | Jump to the tip of the stack |
| [`gx bottom`](#stack-navigation) | Jump to the base of the stack |
| [`gx parent`](#gx-parent) | Print the parent branch name (for scripting) |

**Utility:**

| Command | Description |
|---------|-------------|
| [`gx nuke`](#gx-nuke) | Delete branches (local + remote + tracking) |
| [`gx update`](#gx-update) | Update gx to the latest version |

---

## gx init

One-time setup for stacking. Creates `.git/gx/stack.json` to track branch relationships.

```
gx init                    # Auto-detect trunk branch
gx init --trunk develop    # Explicitly set trunk
gx init --force            # Re-initialize (keeps existing relationships)
```

| Flag | Default | Description |
|------|---------|-------------|
| `--trunk <branch>` | auto-detected | Set the trunk branch explicitly |
| `--force` | false | Re-initialize, preserving existing branch relationships |

Not strictly required. All stacking commands auto-initialize if needed.

```
$ gx init

OK Initialized gx in this repo.
  Trunk: main
  Stack config: .git/gx/stack.json

  Get started:
    gx stack feature/my-thing main    Create your first stacked branch
    gx graph                          View your stack
```

If already initialized:

```
$ gx init

> gx is already initialized.
  Trunk: main
  Tracked branches: 4
  Config: .git/gx/stack.json

  Run with --force to re-initialize.
```

---

## gx undo

Detects the last git action and reverses it. Works on actions performed via `gx` or raw git commands.

```
gx undo              # Undo the last thing
gx redo              # Redo (undo the undo)
gx undo --dry-run    # Preview without executing
gx undo --history    # Show undo/redo history
```

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | false | Show what would be undone without doing it |
| `--history` | false | Display the undo/redo history table |

Detection priority:

| State Detected | Undo Action |
|----------------|-------------|
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

```
$ gx undo --dry-run

DRY RUN. No changes will be made

  Would run: git reset --soft HEAD~1
  Result:    Soft reset to previous commit. Your changes will be preserved in staging.
```

```
$ gx undo --history

Undo/Redo History (last 10):

 #  Time         Action  Description                        Status
 ────────────────────────────────────────────────────────────────────
 1  2 min ago    commit  Undo commit "Add search endpoint"  active
 2  1 hour ago   stage   Undo 3 staged files                redone
```

History is stored in `.git/gx/undo_history.json` and survives app restarts.

---

## gx oops

Amend the last commit. Change the message, add forgotten files, or both.

```
gx oops                                 # Open editor to amend message
gx oops -m "Better message"             # Amend message inline
gx oops --add src/forgot.ts             # Add a file to the last commit
gx oops --add src/forgot.ts -m "Fixed"  # Both at once
gx oops --dry-run                       # Preview
```

| Flag | Default | Description |
|------|---------|-------------|
| `-m, --message <text>` | | New commit message (skips editor if provided) |
| `--add <file>` | | File(s) to add to the last commit (repeatable) |
| `--dry-run` | false | Show what would change |
| `--force` | false | Allow amending even if the commit was already pushed |

Skips files with no actual changes (warns instead of silently amending to an identical tree).

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

**Safety:** Refuses if the commit has been pushed to remote. Use `--force` to override (rewrites shared history).

---

## gx switch

Interactive branch picker. Lists branches sorted by recent activity, type to filter, pick by number.

```
gx switch       # Interactive picker
gx switch -     # Toggle to previous branch (like cd -)
```

```
$ gx switch

    1  feature/payments          2 hours ago     kim
    2  fix/login-bug             5 hours ago     you
    3  feature/auth-v2           3 days ago      james
    4  feature/search            1 day ago       you

Enter a number to switch, text to filter, q to cancel
> auth

    1  feature/auth-v2           3 days ago      james

Enter a number to switch, text to filter, q to cancel
> 1

OK Switched to feature/auth-v2
```

Warns if you have uncommitted changes. If only one other branch exists, switches immediately.

---

## gx context

Everything you need to know about your repo state in one command.

```
gx context      # Full summary
gx ctx          # Same (alias)
```

Shows: branch name, remote tracking status, ahead/behind main, last commit, working tree (modified/staged/untracked counts), stash count, and active operations (merge/rebase/cherry-pick).

```
$ gx context

Branch:  feature/search
Tracking:  origin/feature/search (2 ahead)
vs main:  5 ahead, 1 behind

Last commit:  a1b2c3d "Add search endpoint" (2 hours ago)

Working tree:
  Modified:  3 files
  Staged:  1 file
  Untracked:  2 files

Stash:  2 entries

WARN Rebase in progress
```

---

## gx sweep

Finds and deletes branches that have been merged (or squash-merged) into the trunk. Also prunes stale remote tracking refs.

```
gx sweep              # Interactive (asks before each category)
gx sweep --dry-run    # Preview only
gx sweep -y           # Auto-confirm all
```

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | false | Show what would be cleaned without doing it |
| `-y, --yes` | false | Skip all confirmation prompts |

Detects squash-merged branches via `git cherry` (patches that exist on trunk but weren't merged traditionally). These are confirmed separately with a lower-confidence label.

```
$ gx sweep

Scanning for cleanup opportunities...

Merged branches (safe to delete):
  feature/auth-v1
  fix/typo-readme

Likely squash-merged branches:
  feature/onboarding

Stale remote tracking refs:
  origin/feature/deleted-branch

Summary: 2 merged, 1 likely squash-merged, 1 stale refs

? Delete merged branches? [y/N] y
OK Deleted feature/auth-v1
OK Deleted fix/typo-readme
? Delete likely squash-merged branches? [y/N] y
OK Deleted feature/onboarding
? Prune stale remote tracking refs? [y/N] y
OK Pruned 1 stale remote tracking refs
OK Cleanup complete.
```

Also cleans up deleted branches from `.git/gx/stack.json`.

---

## gx shelf

Stash manager. Interactive picker for browsing, applying, popping, and dropping stashes.

### `gx shelf` (interactive)

Shows all stashes with file change stats. Select by number + action letter in one input.

```
$ gx shelf

10 stashes:

   0  6 weeks ago       WIP on main: e1e2e17346 Merged PR 1940542...
                         main  13 files +1945 -1957
   1  4 months ago      WIP on mubarakidoko/fixPostBuildout...
                         mubarakidoko/fixPostBuildoutPipeline  177 files +6951 -2636

  <n>a = apply  <n>p = pop (apply+drop)  <n>d = drop
  text = filter  q = cancel
> 0a
OK Applied stash@{0} (stash kept)
```

Actions:
- `0a` or `0` - **Apply** stash 0 (keeps the stash in the list)
- `0p` - **Pop** stash 0 (apply + remove from list)
- `0d` - **Drop** stash 0 (delete without applying, confirms first)
- Type text to filter stashes by message or branch name

### `gx shelf push`

```
gx shelf push "Fix token refresh"    # Stash with a message
gx shelf push                        # Auto-message: "gx-shelf: <branch> <timestamp>"
gx shelf push -u "include untracked" # Also stash untracked files
```

| Flag | Default | Description |
|------|---------|-------------|
| `-u, --include-untracked` | false | Also stash untracked files |

### `gx shelf list`

Non-interactive list, same format as `gx shelf` but without the picker.

### `gx shelf clear`

```
gx shelf clear              # Drop ALL stashes (confirms first)
gx shelf clear --dry-run    # Show what would be dropped
```

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | false | Preview without dropping |

---

## gx who

Show top contributors to the repo by commit count. Deduplicates authors who use multiple names/emails via union-find on email address, email username, and display name.

```
gx who              # Top 5 contributors
gx who -n 10        # Top 10
gx who --since 6m   # Only commits in the last 6 months
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n, --number <n>` | 5 | Number of contributors to show |
| `--since <date>` | | Only count commits after this date (e.g. `6months`, `2024-01-01`) |
| `--no-limit` | false | Remove file cap for directory-level analysis |

```
$ gx who

Top contributors

 #  Author          Email                                       Commits  Last Active
 ─────────────────────────────────────────────────────────────────────────────────────
 1  Joshua Etim     65484909+joshuaetim@users.noreply.github..  953      1 month ago
 2  maro            marookegbero@gmail.com                      138      7 months ago
 3  You             66747577+mubbie@users.noreply.gith...       53       28 months ago
 4  Adisa-Shobi     s.oadisa.dev@gmail.com                      45       31 months ago
 5  Nanu Oghenetega nanumichael27@gmail.com                     19       31 months ago

You: Mubbie Idoko <66747577+mubbie@users.noreply.github.com>
```

The "You" row is matched by your `git config user.email`.

---

## gx recap

Show recent commit activity, grouped by date.

```
gx recap                 # Your commits in the last 24 hours
gx recap -d 7            # Last 7 days
gx recap @kim            # Kim's commits (substring match on author name/email)
gx recap --all           # Everyone's activity
gx recap --all -d 7      # Team activity, last 7 days
```

| Flag | Default | Description |
|------|---------|-------------|
| `@<name>` (argument) | current git user | Filter by author name (substring match) |
| `-d, --days <n>` | 1 | Number of days to look back |
| `--all` | false | Show all contributors |
| `--limit <n>` | 100 | Max commits to display |

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

```
$ gx recap --all -d 1

Team activity in the last 1 day (my-project):

  Kim (2 commits):
    x1y2z3a  15:00  Fix auth token refresh
    b4c5d6e  10:30  Add token rotation

  You (3 commits):
    a1b2c3d  14:32  Add search endpoint
    d4e5f6g  11:15  Add search index util
    h7i8j9k  09:03  Scaffold search module

  5 commits from 2 contributors
```

---

## gx drift

Show how far your branch has diverged from main (or any target branch), with commit lists and file stats.

```
gx drift                # vs main (or auto-detected trunk)
gx drift develop        # vs a specific branch
gx drift --parent       # vs stack parent (what the PR reviewer sees)
gx drift --full         # Show all commits, no truncation
```

| Flag | Default | Description |
|------|---------|-------------|
| `--full` | false | Show all commits (default truncates at 20) |
| `-p, --parent` | false | Compare against the stack parent instead of trunk |

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

`--parent` is useful for stacked branches where `gx drift` against main includes the parent's commits. `gx drift --parent` shows only what your branch adds on top of its parent.

---

## gx conflicts

Preview merge conflicts before actually merging. Nothing is modified on disk.

```
gx conflicts              # Check against main
gx conflicts develop      # Check against a specific branch
```

Uses `git merge-tree --write-tree` (Git 2.38+) to simulate the merge in memory. Falls back to the 3-arg `git merge-tree` on older Git versions.

```
$ gx conflicts

Checking feature/search against main...

X 3 conflicts found

  src/api/auth.ts          (you + kim)
  src/utils/helpers.ts     (you + james)
  package.json

  14 other files merge cleanly
```

```
$ gx conflicts

Checking feature/search against main...

OK No conflicts. Clean merge.
  12 files would be modified
```

---

## gx handoff

Generate a copy-pasteable summary of your branch for PR descriptions, Slack messages, or standup notes.

```
gx handoff                  # Plain text, auto-detect base
gx handoff --markdown       # Markdown format
gx handoff --copy           # Also copy to clipboard
gx handoff --against main   # Override the comparison base
```

| Flag | Default | Description |
|------|---------|-------------|
| `--against <branch>` | auto-detected | Compare against a specific branch |
| `-c, --copy` | false | Copy output to system clipboard (pbcopy/xclip/clip) |
| `--markdown, --md` | false | Format as markdown |

Auto-detects the base: uses the stack parent if the branch is tracked in `.git/gx/stack.json`, otherwise falls back to main.

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

**Markdown:**

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

---

## gx view

Focused view of the stack you're currently in. Shows only your stack, not all branches.

```
gx view
```

No flags. Just run it.

```
$ gx view

main
  <- feature/auth-v1     #42  + approved      2 ahead   3h ago
  <- feature/auth-v2     #43  o reviewing     3 ahead   1h ago    <
  <- feature/auth-v3          no PR           1 ahead   20m ago
```

- `<` marks the current branch
- PR status shown if `gh` CLI is installed (omitted silently otherwise)
- PR statuses: `+ approved`, `+ merged`, `o reviewing`, `x changes`, `no PR`

On trunk, shows a summary of all stacks:

```
$ gx view

You're on main (trunk)

Stacks branching from main:
  <- feature/auth-v1 -> auth-v2 -> auth-v3   (3 branches)
  <- feature/search                            (1 branch)

Use `gx graph` to see the full tree.
```

On an untracked branch:

```
$ gx view

> Current branch (experiment/thing) is not part of a stack.
  vs main: 5 ahead, 0 behind

  Tip: Use `gx stack` to start stacking, or `gx graph` to see all branches.
```

---

## gx stack

Create a new branch on top of a parent, with the relationship tracked in `.git/gx/stack.json`.

```
gx stack <new-branch> <parent-branch>
```

```
$ gx stack feature/auth-v2 feature/auth-v1

OK Created feature/auth-v2 on top of feature/auth-v1
  Relationship saved to stack config.
```

Records the parent's HEAD SHA at creation time (`parent_head`), used by `gx sync` and `gx retarget` for precise `--onto` rebasing.

---

## gx sync

Rebase and push a chain of stacked branches in sequence.

```
gx sync --stack                              # Auto-detect full stack from current branch
gx sync main feature/auth-v1 feature/auth-v2 # Explicit chain (topologically sorted)
gx sync --stack --dry-run                    # Preview
```

| Flag | Default | Description |
|------|---------|-------------|
| `--stack` | false | Auto-detect the full stack chain from the current branch |
| `--dry-run` | false | Show what would happen without executing |

**Rebase strategy:**
- Git 2.38+: uses `git rebase --update-refs` (single operation for the whole chain, preferred)
- Git < 2.38: falls back to `git rebase --onto` iteration with pre-captured SHAs
- Validates that the chain is linear before using `--update-refs` (falls back to `--onto` for branched stacks)

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

On conflict, stops immediately with resolution instructions:

```
ERROR Rebase conflict encountered

  Conflicting files:
    src/auth/login.ts

  To resolve:
    1. Fix the conflicts in the listed files
    2. Run: git add . && git rebase --continue
    3. Run: gx sync feature/auth-v1 feature/auth-v2
       (to continue syncing the rest of the stack)
```

**Safety:** Uses `--force-with-lease` (not `--force`). Confirms for chains of 5+ branches. Returns to your original branch after completion.

---

## gx retarget

Move a branch to a new base. Uses `git rebase --onto` to slice out only the branch's own commits. Auto-retargets the PR via `gh` CLI if available.

```
gx retarget <branch> <new-target>
gx retarget <branch> <new-target> --dry-run
```

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | false | Show what would happen |

If only one argument is given, retargets the current branch.

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

Uses the stored `parent_head` SHA for precise `--onto` targeting. Falls back to `git merge-base` if the branch was created outside `gx`.

---

## gx graph

Visualize the complete branch stack as a tree.

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

Auto-discovers relationships for branches created outside `gx` (via merge-base heuristic) and saves them to the config. The graph gets more accurate over time.

---

## Stack Navigation

Navigate the stack without remembering branch names.

| Command | Description |
|---------|-------------|
| `gx up` | Move to the child branch (one step away from trunk) |
| `gx down` | Move to the parent branch (one step toward trunk) |
| `gx top` | Jump to the tip of the stack (furthest from trunk) |
| `gx bottom` | Jump to the base of the stack (closest to trunk) |

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

When a branch has multiple children (fork in the stack):

```
$ gx up

> Multiple branches stacked on feature/auth-v1:
>   feature/auth-api
>   feature/auth-ui
> Use `gx switch` to pick one.
```

Warns about uncommitted changes before switching.

---

## gx parent

Print the parent branch name to stdout. No formatting, no color. Designed for shell composability.

```
$ gx parent
feature/auth-v1
```

Falls back to the trunk branch if the current branch isn't in the stack. Exits with non-zero status if already on trunk.

Use it as a building block:

```bash
git diff $(gx parent)...HEAD        # Diff against stack parent
git log $(gx parent)..HEAD          # Commits unique to your branch
git rebase $(gx parent)             # Manual rebase onto parent
gh pr create --base $(gx parent)    # Create PR targeting the stack parent
```

---

## gx nuke

Delete branches with confidence. Handles local branch, remote tracking ref, and remote branch in one command.

```
gx nuke feature/old-thing          # Delete local + remote + tracking
gx nuke feature/old-thing --local  # Local only
gx nuke feature/old-thing --dry-run
gx nuke "feature/*"                # Glob pattern
gx nuke --orphans                  # Delete all orphaned branches from the stack
```

| Flag | Default | Description |
|------|---------|-------------|
| `--local` | false | Only delete the local branch |
| `--dry-run` | false | Show what would be deleted |
| `-y, --yes` | false | Skip confirmation prompt |
| `--orphans` | false | Delete all branches marked as orphaned in `gx graph` |

```
$ gx nuke feature/old-auth

  feature/old-auth is NOT merged into main.

WARN feature/old-auth has 2 dependent branches: feature/auth-v2, feature/auth-v3. They will become orphaned.

? Proceed with deletion? [y/N] y

OK Deleted local branch feature/old-auth
OK Deleted remote tracking ref origin/feature/old-auth
OK Deleted remote branch origin/feature/old-auth
```

**Safety:**
- Cannot delete the current branch (switch first)
- Cannot delete the trunk branch (blocked entirely)
- Warns about unmerged commits
- Warns about stack dependents that will become orphaned
- Cleans up `.git/gx/stack.json` after deletion

Also resolves remote-only branches (where the local was already deleted but `origin/foo` remains).

---

## gx update

Check for updates and upgrade gx.

```
gx update
```

Detects how gx was installed:
- **Homebrew:** runs `brew upgrade gx-git`
- **Other:** shows manual upgrade instructions for pip/go/binary

```
$ gx update

Current version: 2.2.0
Installed via Homebrew. Updating...

OK Updated via Homebrew. Run `gx --version` to verify.
```

---

## Stack Configuration

All stacking state is stored in `.git/gx/stack.json` (inside `.git/`, never committed). Format:

```json
{
  "branches": {
    "feature/auth-v1": {
      "parent": "main",
      "parent_head": "a1b2c3d4e5f6"
    },
    "feature/auth-v2": {
      "parent": "feature/auth-v1",
      "parent_head": "d4e5f6g7h8i9"
    }
  },
  "metadata": {
    "main_branch": "main",
    "last_updated": "2026-04-15T14:32:00Z"
  }
}
```

- `parent`: the branch this was stacked on
- `parent_head`: the parent's HEAD SHA when the relationship was created/last synced (used for precise `--onto` rebasing)
- Auto-migrates from older formats transparently

Undo history is stored separately in `.git/gx/undo_history.json`.

---

## Tech Stack

**Go (Homebrew / binary):**
- [Cobra](https://github.com/spf13/cobra): CLI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss): Terminal styling and colors

**Python (pip / pipx):**
- [Typer](https://typer.tiangolo.com/): CLI framework
- [Rich](https://rich.readthedocs.io/): Terminal formatting
- [Textual](https://textual.textualize.io/): TUI components

Both versions share the same `.git/gx/` format and command interface.

## License

[MIT](LICENSE)
