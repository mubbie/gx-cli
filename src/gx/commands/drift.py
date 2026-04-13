"""gx drift — Show how far your branch has diverged from the HEAD branch."""

from __future__ import annotations

import typer

from gx.utils.config import DRIFT_MAX_COMMITS
from gx.utils.display import console, print_error, print_info, print_success
from gx.utils.git import (
    GitError,
    ensure_git_repo,
    get_current_branch,
    get_head_branch,
    get_merge_base,
    run_git,
    time_ago,
)


def _get_commits_between(from_ref: str, to_ref: str, limit: int | None = None) -> list[dict]:
    """Get commits in to_ref that are not in from_ref."""
    fmt = "%h\t%aI\t%an\t%s"
    args = ["log", f"--format={fmt}", f"{from_ref}..{to_ref}"]
    if limit:
        args.append(f"-{limit}")
    try:
        output = run_git(args)
    except GitError:
        return []

    if not output:
        return []

    commits = []
    for line in output.splitlines():
        parts = line.split("\t", 3)
        if len(parts) < 4:
            continue
        commits.append({
            "hash": parts[0],
            "date": parts[1],
            "author": parts[2],
            "message": parts[3],
        })
    return commits


def drift(
    target: str = typer.Argument(None, help="Branch to compare against (default: HEAD branch)."),
    full: bool = typer.Option(False, "--full", help="Show all commits (no truncation)."),
    parent: bool = typer.Option(False, "--parent", "-p", help="Compare against stack parent."),
) -> None:
    """Show how far your branch has diverged from the HEAD branch."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    try:
        current = get_current_branch()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    if parent and target:
        print_error("Cannot use --parent with an explicit target branch.")
        raise typer.Exit(1)

    if parent:
        from gx.utils.stack import get_parent as stack_get_parent
        stack_parent = stack_get_parent(current)
        if stack_parent is None:
            print_error(f"{current} is not in the stack. Use `gx drift` without --parent.")
            raise typer.Exit(1)
        target_branch = stack_parent
    else:
        target_branch = target or get_head_branch()

    if current == target_branch:
        print_info(f"You're on {target_branch}. Switch to a feature branch to see drift.")
        raise typer.Exit(0)

    try:
        merge_base = get_merge_base("HEAD", target_branch)
    except GitError:
        print_error(f"No common ancestor between {current} and {target_branch}.")
        raise typer.Exit(1)

    # Count ahead/behind
    try:
        ab = run_git(["rev-list", "--left-right", "--count", f"HEAD...{target_branch}"])
        ahead, behind = ab.split()
        ahead_n = int(ahead)
        behind_n = int(behind)
    except GitError:
        ahead_n = behind_n = 0

    console.print()

    if ahead_n == 0 and behind_n == 0:
        print_success(f"No divergence \u2014 your branch is based on the latest {target_branch}.")
        return

    console.print(f"[bold]{current}[/bold] is {ahead_n} ahead, {behind_n} behind [bold]{target_branch}[/bold]")
    console.print()

    limit = None if full else DRIFT_MAX_COMMITS

    # Commits on your branch
    if ahead_n > 0:
        your_commits = _get_commits_between(merge_base, "HEAD", limit=limit)
        console.print(f"[bold]Commits on your branch (not on {target_branch}):[/bold]")
        for c in your_commits:
            age = time_ago(c["date"])
            console.print(f"  [dim]{c['hash']}[/dim]  {age:<12} {c['message']}")
        if not full and ahead_n > DRIFT_MAX_COMMITS:
            console.print(f"  (and {ahead_n - DRIFT_MAX_COMMITS} more \u2014 use --full to see all)")
        console.print()

    # Commits on target
    if behind_n > 0:
        target_commits = _get_commits_between(merge_base, target_branch, limit=limit)
        console.print(f"[bold]Commits on {target_branch} (not on your branch):[/bold]")
        for c in target_commits:
            age = time_ago(c["date"])
            author = f" ({c['author']})" if c["author"] else ""
            console.print(f"  [dim]{c['hash']}[/dim]  {age:<12} {c['message']}{author}")
        if not full and behind_n > DRIFT_MAX_COMMITS:
            console.print(f"  (and {behind_n - DRIFT_MAX_COMMITS} more \u2014 use --full to see all)")
        console.print()

    # File divergence stats
    try:
        stat_output = run_git(["diff", "--stat", f"{merge_base}..HEAD"])
        if stat_output:
            lines = stat_output.splitlines()
            if lines:
                summary = lines[-1].strip()
                console.print(f"Files diverged: {summary}")
                console.print()
    except GitError:
        pass
