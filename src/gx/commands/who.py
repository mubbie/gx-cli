"""gx who: Show who knows a file, directory, or repo best."""

from __future__ import annotations

import os
from collections import defaultdict
from concurrent.futures import ThreadPoolExecutor, as_completed

import typer
from rich.progress import Progress, SpinnerColumn, TextColumn

from gx.utils.config import DEFAULT_WHO_LIMIT, WHO_MAX_FILES
from gx.utils.display import console, print_error, print_info, print_table, print_warning
from gx.utils.git import GitError, ensure_git_repo, run_git, time_ago


def _blame_file(filepath: str, cwd: str | None = None) -> dict[str, dict]:
    """Run git blame on a file and return per-author stats.

    Returns {author: {"lines": int, "email": str, "last_ts": str}}.
    """
    args = ["blame", "--line-porcelain", filepath]
    try:
        output = run_git(args, cwd=cwd)
    except GitError:
        return {}

    stats: dict[str, dict] = {}
    cur_author = ""
    cur_email = ""
    cur_ts = ""
    for line in output.splitlines():
        if line.startswith("author "):
            cur_author = line[7:].strip()
        elif line.startswith("author-mail "):
            cur_email = line[12:].strip().strip("<>")
        elif line.startswith("author-time "):
            cur_ts = line[12:].strip()
        elif line.startswith("\t"):
            # End of a blame block; accumulate
            if cur_author and cur_author != "Not Committed Yet":
                if cur_author not in stats:
                    stats[cur_author] = {"lines": 0, "email": cur_email, "last_ts": cur_ts}
                stats[cur_author]["lines"] += 1
                # Keep the most recent timestamp
                if cur_ts > stats[cur_author]["last_ts"]:
                    stats[cur_author]["last_ts"] = cur_ts
                # Keep an email if we have one
                if cur_email and not stats[cur_author]["email"]:
                    stats[cur_author]["email"] = cur_email
    return stats


def _ts_to_relative(ts: str) -> str:
    """Convert a unix timestamp string to a relative time."""
    try:
        from datetime import datetime, timezone

        dt = datetime.fromtimestamp(int(ts), tz=timezone.utc)
        return time_ago(dt.isoformat())
    except (ValueError, TypeError, OSError):
        return "unknown"


def _get_tracked_files(directory: str, cwd: str | None = None) -> list[str]:
    """Get tracked files in a directory."""
    output = run_git(["ls-files", directory], cwd=cwd)
    if not output:
        return []
    return output.splitlines()


def _repo_level(n: int) -> None:
    """Show top contributors to the entire repo (fast, shortlog only)."""
    args = ["shortlog", "-sne", "HEAD"]
    output = run_git(args)
    if not output:
        print_info("No contributors found.")
        return

    # Parse shortlog output: "  123\tName <email>"
    rows: list[list[str]] = []
    for i, line in enumerate(output.splitlines()):
        if i >= n:
            break
        line = line.strip()
        if not line:
            continue
        parts = line.split("\t", 1)
        if len(parts) != 2:
            continue
        commits = parts[0].strip()
        name_email = parts[1].strip()
        if "<" in name_email and ">" in name_email:
            name = name_email[: name_email.index("<")].strip()
            email = name_email[name_email.index("<") + 1 : name_email.index(">")]
        else:
            name = name_email
            email = ""
        rows.append([str(len(rows) + 1), name, commits, email])

    console.print()
    print_table(
        headers=["#", "Author", "Commits", "Email"],
        rows=rows,
        title="Top contributors",
    )


def _file_level(filepath: str, n: int) -> None:
    """Show who knows a specific file best (blame-based)."""
    if not os.path.exists(filepath):
        print_error(f"File not found: {filepath}")
        raise typer.Exit(1)

    stats = _blame_file(filepath)
    if not stats:
        print_info(f"No blame data for {filepath}")
        return

    total = sum(s["lines"] for s in stats.values())
    sorted_authors = sorted(stats.items(), key=lambda x: x[1]["lines"], reverse=True)

    rows: list[list[str]] = []
    for i, (author, s) in enumerate(sorted_authors[:n], 1):
        pct = f"{s['lines'] / total * 100:.1f}%"
        last_edit = _ts_to_relative(s["last_ts"]) if s["last_ts"] else "unknown"
        rows.append([str(i), author, str(s["lines"]), pct, s.get("email", ""), last_edit])

    console.print()
    print_table(
        headers=["#", "Author", "Lines", "%", "Email", "Last Edit"],
        rows=rows,
        title=f"Ownership of {filepath} ({total} lines)",
    )


def _dir_level(directory: str, n: int, no_limit: bool) -> None:
    """Show who knows a directory best (concurrent blame)."""
    files = _get_tracked_files(directory)
    if not files:
        print_info(f"No tracked files in {directory}")
        return

    if len(files) > WHO_MAX_FILES and not no_limit:
        print_warning(
            f"{directory} contains {len(files)} tracked files. Analyzing first {WHO_MAX_FILES}.\n"
            f"  Use --no-limit to analyze all (this may take a while)."
        )
        files = files[:WHO_MAX_FILES]

    # Per-author aggregation: lines, files touched, email
    total_lines_by_author: dict[str, int] = defaultdict(int)
    files_touched: dict[str, set[str]] = defaultdict(set)
    author_email: dict[str, str] = {}

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        console=console,
    ) as progress:
        task = progress.add_task(f"Analyzing {len(files)} files...", total=len(files))

        def blame_one(f: str) -> tuple[str, dict[str, dict]]:
            return f, _blame_file(f)

        with ThreadPoolExecutor(max_workers=8) as pool:
            futures = {pool.submit(blame_one, f): f for f in files}
            for future in as_completed(futures):
                filepath, stats = future.result()
                for author, s in stats.items():
                    total_lines_by_author[author] += s["lines"]
                    files_touched[author].add(filepath)
                    if s.get("email") and author not in author_email:
                        author_email[author] = s["email"]
                progress.advance(task)

    total_lines = sum(total_lines_by_author.values())
    if total_lines == 0:
        print_info(f"No blame data for {directory}")
        return

    sorted_authors = sorted(total_lines_by_author.items(), key=lambda x: x[1], reverse=True)

    rows: list[list[str]] = []
    for i, (author, lines) in enumerate(sorted_authors[:n], 1):
        pct = f"{lines / total_lines * 100:.1f}%"
        ft = str(len(files_touched.get(author, set())))
        email = author_email.get(author, "")
        rows.append([str(i), author, str(lines), pct, ft, email])

    console.print()
    print_table(
        headers=["#", "Author", "Lines", "%", "Files", "Email"],
        rows=rows,
        title=f"Ownership of {directory} ({len(files)} files, {total_lines} lines)",
    )


def who(
    path: str = typer.Argument(None, help="File or directory to analyze (omit for repo-level)."),
    n: int = typer.Option(DEFAULT_WHO_LIMIT, "-n", "--number", help="Number of contributors to show."),
    no_limit: bool = typer.Option(False, "--no-limit", help="Remove file cap for directory analysis."),
) -> None:
    """Show who knows a file, directory, or repo best."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    if path is None:
        _repo_level(n)
    elif os.path.isdir(path):
        _dir_level(path, n, no_limit)
    else:
        _file_level(path, n)
