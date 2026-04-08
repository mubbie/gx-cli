"""gx context — Enhanced repo status at a glance."""

from __future__ import annotations

import os

import typer

from gx.utils.display import console, print_error, print_warning
from gx.utils.git import (
    GitError,
    ensure_git_repo,
    get_current_branch,
    get_head_branch,
    get_last_commit,
    get_repo_root,
    get_stash_count,
    run_git,
    time_ago,
)


def context() -> None:
    """Repo status at a glance \u2014 branch, commits, working tree, and more."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    console.print()

    # Branch
    try:
        branch = get_current_branch()
    except GitError:
        try:
            head_hash = run_git(["rev-parse", "--short", "HEAD"])
            console.print(f"[bold]Branch:[/bold]       (detached HEAD at {head_hash})")
            print_warning("You are in detached HEAD state.")
        except GitError:
            console.print("[bold]Branch:[/bold]       (unknown)")
        branch = None

    if branch:
        console.print(f"[bold]Branch:[/bold]       {branch}")

        # Tracking info
        try:
            tracking = run_git(
                ["rev-parse", "--abbrev-ref", f"{branch}@{{upstream}}"],
                check=False,
            )
            if tracking:
                # Check ahead/behind tracking branch
                try:
                    ab = run_git(
                        ["rev-list", "--left-right", "--count", f"{branch}...{tracking}"]
                    )
                    ahead, behind = ab.split()
                    if ahead == "0" and behind == "0":
                        console.print(f"[bold]Tracking:[/bold]     {tracking} (up to date)")
                    else:
                        parts = []
                        if int(ahead) > 0:
                            parts.append(f"{ahead} ahead")
                        if int(behind) > 0:
                            parts.append(f"{behind} behind")
                        console.print(f"[bold]Tracking:[/bold]     {tracking} ({', '.join(parts)})")
                except GitError:
                    console.print(f"[bold]Tracking:[/bold]     {tracking}")
        except GitError:
            pass

        # vs HEAD branch
        head_branch = get_head_branch()
        if branch != head_branch:
            try:
                ab = run_git(
                    ["rev-list", "--left-right", "--count", f"{branch}...{head_branch}"]
                )
                ahead, behind = ab.split()
                console.print(f"[bold]vs {head_branch}:[/bold]{'':>{8 - len(head_branch)}} {ahead} ahead, {behind} behind")
            except GitError:
                pass

    console.print()

    # Last commit
    commit = get_last_commit()
    if commit:
        age = time_ago(commit["date"])
        console.print(f'[bold]Last commit:[/bold]  {commit["short_hash"]} "{commit["message"]}" ({age})')
    else:
        console.print("[bold]Last commit:[/bold]  No commits yet")

    console.print()

    # Working tree status
    try:
        status_output = run_git(["status", "--porcelain"])
    except GitError:
        status_output = ""

    if not status_output:
        console.print("[bold]Working tree:[/bold] clean")
    else:
        modified = 0
        staged = 0
        untracked = 0
        for line in status_output.splitlines():
            if not line or len(line) < 2:
                continue
            index_status = line[0]
            work_status = line[1]
            if line.startswith("??"):
                untracked += 1
            else:
                if index_status not in (" ", "?"):
                    staged += 1
                if work_status not in (" ", "?"):
                    modified += 1

        console.print("[bold]Working tree:[/bold]")
        if modified:
            console.print(f"  Modified:   {modified} file{'s' if modified != 1 else ''}")
        if staged:
            console.print(f"  Staged:     {staged} file{'s' if staged != 1 else ''}")
        if untracked:
            console.print(f"  Untracked:  {untracked} file{'s' if untracked != 1 else ''}")

    console.print()

    # Stash count
    stash_count = get_stash_count()
    if stash_count > 0:
        console.print(f"[bold]Stash:[/bold]        {stash_count} entr{'ies' if stash_count != 1 else 'y'}")
    else:
        console.print("[bold]Stash:[/bold]        empty")

    # Check for active operations
    try:
        root = get_repo_root()
    except GitError:
        root = ""

    if root:
        merge_head = os.path.join(root, ".git", "MERGE_HEAD")
        rebase_merge = os.path.join(root, ".git", "rebase-merge")
        rebase_apply = os.path.join(root, ".git", "rebase-apply")
        cherry_pick = os.path.join(root, ".git", "CHERRY_PICK_HEAD")

        if os.path.exists(merge_head):
            console.print()
            print_warning("Merge in progress")
        elif os.path.isdir(rebase_merge):
            # Try to get rebase progress
            try:
                msgnum = open(os.path.join(rebase_merge, "msgnum")).read().strip()
                end = open(os.path.join(rebase_merge, "end")).read().strip()
                console.print()
                print_warning(f"Rebase in progress ({msgnum}/{end} commits applied)")
            except (OSError, FileNotFoundError):
                console.print()
                print_warning("Rebase in progress")
        elif os.path.isdir(rebase_apply):
            console.print()
            print_warning("Rebase in progress")
        elif os.path.exists(cherry_pick):
            console.print()
            print_warning("Cherry-pick in progress")

    console.print()
