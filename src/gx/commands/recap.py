"""gx recap -- Show what you (or someone else) did recently."""

from __future__ import annotations

import os
from collections import defaultdict
from datetime import datetime, timedelta, timezone

import typer

from gx.utils.config import DEFAULT_RECAP_DAYS, RECAP_MAX_COMMITS
from gx.utils.display import console, print_error, print_info
from gx.utils.git import GitError, ensure_git_repo, get_repo_root, run_git


def _get_current_user() -> str:
    try:
        return run_git(["config", "user.name"])
    except GitError:
        return ""


def _get_commits(
    author: str | None,
    days: int,
    all_authors: bool,
    cwd: str | None = None,
    limit: int = RECAP_MAX_COMMITS,
) -> list[dict]:
    """Get commits matching criteria."""
    since = (datetime.now(timezone.utc) - timedelta(days=days)).strftime("%Y-%m-%dT%H:%M:%S")
    fmt = "%h %aI %s"
    args = ["log", "--all", "--oneline", f"--format={fmt}", f"--since={since}", f"-{limit}"]

    if not all_authors and author:
        args.extend([f"--author={author}"])

    try:
        output = run_git(args, cwd=cwd)
    except GitError:
        return []

    if not output:
        return []

    commits = []
    for line in output.splitlines():
        parts = line.split(" ", 2)
        if len(parts) < 3:
            continue
        commits.append({
            "hash": parts[0],
            "date": parts[1],
            "message": parts[2],
        })
    return commits


def _get_diffstat(days: int, author: str | None, cwd: str | None = None) -> str:
    """Get summary diffstat."""
    since = (datetime.now(timezone.utc) - timedelta(days=days)).strftime("%Y-%m-%dT%H:%M:%S")
    args = ["log", "--all", f"--since={since}", "--shortstat", "--format="]
    if author:
        args.extend([f"--author={author}"])
    try:
        output = run_git(args, cwd=cwd)
        # Aggregate stats
        files_changed = 0
        insertions = 0
        deletions = 0
        for line in output.splitlines():
            line = line.strip()
            if not line:
                continue
            if "file" in line:
                parts = line.split(",")
                for part in parts:
                    part = part.strip()
                    if "file" in part:
                        files_changed += int(part.split()[0])
                    elif "insertion" in part:
                        insertions += int(part.split()[0])
                    elif "deletion" in part:
                        deletions += int(part.split()[0])
        if files_changed:
            return f"{files_changed} files changed, +{insertions} -{deletions}"
        return ""
    except GitError:
        return ""


def _group_by_date(commits: list[dict]) -> dict[str, list[dict]]:
    """Group commits by date."""
    grouped: dict[str, list[dict]] = defaultdict(list)
    now = datetime.now(timezone.utc)
    for commit in commits:
        try:
            dt = datetime.fromisoformat(commit["date"].replace("Z", "+00:00"))
            diff = now.date() - dt.date()
            if diff.days == 0:
                key = "Today"
            elif diff.days == 1:
                key = "Yesterday"
            else:
                key = dt.strftime("%b %d")
        except (ValueError, TypeError):
            key = "Unknown"
        grouped[key].append(commit)
    return grouped


def recap(
    author: str = typer.Argument(None, help="Filter by author (e.g., @kim). Omit for your own commits."),
    days: int = typer.Option(DEFAULT_RECAP_DAYS, "-d", "--days", help="Number of days to look back."),
    all_authors: bool = typer.Option(False, "--all", help="Show all contributors."),
    limit: int = typer.Option(RECAP_MAX_COMMITS, "--limit", help="Max commits to display."),
) -> None:
    """Show what you (or someone else) did recently."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    # Resolve author
    author_filter = author
    if author_filter and author_filter.startswith("@"):
        author_filter = author_filter[1:]
    elif not author_filter and not all_authors:
        author_filter = _get_current_user()
        if not author_filter:
            print_error("Cannot determine your git user. Set git config user.name or specify an author.")
            raise typer.Exit(1)

    _recap_single_repo(author_filter, days, all_authors, limit)


def _recap_single_repo(author: str | None, days: int, all_authors: bool, limit: int) -> None:
    try:
        root = get_repo_root()
        repo_name = os.path.basename(root)
    except GitError:
        repo_name = "unknown"

    if all_authors:
        _recap_all_authors(days, limit)
        return

    author_display = "Your" if author == _get_current_user() else f"{author}'s"
    commits = _get_commits(author, days, all_authors=False, limit=limit)

    if not commits:
        day_str = f"{days} day{'s' if days != 1 else ''}"
        print_info(f"No activity in the last {day_str}.")
        return

    console.print()
    day_label = f"{days} day{'s' if days != 1 else ''}"
    console.print(f"[bold]{author_display} activity in the last {day_label} ({repo_name}):[/bold]")
    console.print()

    grouped = _group_by_date(commits)
    for date_label, date_commits in grouped.items():
        console.print(f"  [bold]{date_label}:[/bold]")
        for c in date_commits:
            try:
                dt = datetime.fromisoformat(c["date"].replace("Z", "+00:00"))
                time_str = dt.strftime("%H:%M")
            except (ValueError, TypeError):
                time_str = "     "
            console.print(f"    [dim]{c['hash']}[/dim]  {time_str}  {c['message']}")
        console.print()

    diffstat = _get_diffstat(days, author)
    total_days = len(grouped)
    console.print(f"  {len(commits)} commits across {total_days} day{'s' if total_days != 1 else ''}")
    if diffstat:
        console.print(f"  {diffstat}")


def _recap_all_authors(days: int, limit: int) -> None:
    """Show all contributors' activity."""
    try:
        root = get_repo_root()
        repo_name = os.path.basename(root)
    except GitError:
        repo_name = "unknown"

    since = (datetime.now(timezone.utc) - timedelta(days=days)).strftime("%Y-%m-%dT%H:%M:%S")
    fmt = "%h %aI %an %s"
    args = ["log", "--all", f"--format={fmt}", f"--since={since}", f"-{limit}"]

    try:
        output = run_git(args)
    except GitError:
        print_info("No activity found.")
        return

    if not output:
        print_info(f"No activity in the last {days} day{'s' if days != 1 else ''}.")
        return

    # Fetch with tab-separated format for reliable parsing
    fmt2 = "%h\t%aI\t%an\t%s"
    args2 = ["log", "--all", f"--format={fmt2}", f"--since={since}", f"-{limit}"]
    try:
        output = run_git(args2)
    except GitError:
        return

    by_author = defaultdict(list)
    current_user = _get_current_user()

    for line in output.splitlines():
        parts = line.split("\t", 3)
        if len(parts) < 4:
            continue
        by_author[parts[2]].append({
            "hash": parts[0],
            "date": parts[1],
            "message": parts[3],
        })

    console.print()
    console.print(f"[bold]Team activity in the last {days} day{'s' if days != 1 else ''} ({repo_name}):[/bold]")
    console.print()

    total = 0
    for author_name, commits in by_author.items():
        display_name = "You" if author_name == current_user else author_name
        console.print(f"  [bold]{display_name}[/bold] ({len(commits)} commit{'s' if len(commits) != 1 else ''}):")
        for c in commits:
            try:
                dt = datetime.fromisoformat(c["date"].replace("Z", "+00:00"))
                time_str = dt.strftime("%H:%M")
            except (ValueError, TypeError):
                time_str = "     "
            console.print(f"    [dim]{c['hash']}[/dim]  {time_str}  {c['message']}")
        console.print()
        total += len(commits)

    console.print(f"  {total} commits from {len(by_author)} contributor{'s' if len(by_author) != 1 else ''}")
