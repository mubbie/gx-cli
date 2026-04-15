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


def _blame_file(filepath: str, since: str | None = None, cwd: str | None = None) -> dict[str, int]:
    """Run git blame on a file and return {author: line_count}."""
    args = ["blame", "--line-porcelain"]
    if since:
        args.extend(["--since", since])
    args.append(filepath)
    try:
        output = run_git(args, cwd=cwd)
    except GitError:
        return {}

    counts: dict[str, int] = defaultdict(int)
    for line in output.splitlines():
        if line.startswith("author "):
            author = line[7:].strip()
            if author != "Not Committed Yet":
                counts[author] += 1
    return dict(counts)


def _get_tracked_files(directory: str, cwd: str | None = None) -> list[str]:
    """Get tracked files in a directory."""
    output = run_git(["ls-files", directory], cwd=cwd)
    if not output:
        return []
    return output.splitlines()


def _get_author_last_edit(author: str, path: str, cwd: str | None = None) -> str:
    """Get last edit date for an author on a path."""
    try:
        output = run_git(
            ["log", "-1", f"--author={author}", "--format=%aI", "--", path],
            cwd=cwd,
        )
        return time_ago(output) if output else "unknown"
    except GitError:
        return "unknown"


def _repo_level(n: int, since: str | None, show_email: bool) -> None:
    """Show top contributors to the entire repo."""
    args = ["shortlog", "-sne", "--all"]
    if since:
        args.extend(["--since", since])
    output = run_git(args)
    if not output:
        print_info("No contributors found.")
        return

    # Parse shortlog output: "  123\tName <email>"
    raw_entries: list[dict[str, str | int]] = []
    for line in output.splitlines():
        line = line.strip()
        if not line:
            continue
        parts = line.split("\t", 1)
        if len(parts) != 2:
            continue
        commits = int(parts[0].strip())
        name_email = parts[1].strip()
        if "<" in name_email and ">" in name_email:
            name = name_email[: name_email.index("<")].strip()
            email = name_email[name_email.index("<") + 1 : name_email.index(">")].lower()
        else:
            name = name_email
            email = ""
        raw_entries.append({"name": name, "email": email, "commits": commits})

    # Deduplicate by email: merge entries with the same email,
    # keep the name with the most commits as the display name
    by_email: dict[str, dict[str, str | int]] = {}
    for entry in raw_entries:
        e_email = str(entry["email"])
        e_name = str(entry["name"])
        e_commits = int(entry["commits"])
        key = e_email if e_email else f"__name__{e_name.lower()}"
        if key in by_email:
            by_email[key]["commits"] = int(by_email[key]["commits"]) + e_commits
            if len(e_name) > len(str(by_email[key]["name"])):
                by_email[key]["name"] = e_name
        else:
            by_email[key] = {"name": e_name, "email": e_email, "commits": e_commits}

    contributors = sorted(by_email.values(), key=lambda c: c["commits"], reverse=True)

    # Get current git user (both name and email for matching)
    try:
        current_user_name = run_git(["config", "user.name"])
    except GitError:
        current_user_name = ""
    try:
        current_user_email = run_git(["config", "user.email"]).lower()
    except GitError:
        current_user_email = ""

    rows = []
    for i, c in enumerate(contributors[:n], 1):
        name = str(c["name"])
        email_addr = str(c["email"])
        is_you = (
            (current_user_email and email_addr == current_user_email)
            or (current_user_name and name == current_user_name)
        )
        display_name = "You" if is_you else name
        if show_email and email_addr:
            display_name += f" <{email_addr}>"
        last_active = _get_author_last_edit(name, ".")
        rows.append([str(i), display_name, str(c["commits"]), last_active])

    console.print()
    print_table(
        headers=["#", "Author", "Commits", "Last Active"],
        rows=rows,
        title="Top contributors",
    )


def _file_level(filepath: str, n: int, since: str | None, show_email: bool) -> None:
    """Show who knows a specific file best."""
    if not os.path.exists(filepath):
        print_error(f"File not found: {filepath}")
        raise typer.Exit(1)

    counts = _blame_file(filepath, since=since)
    if not counts:
        print_info(f"No blame data for {filepath}")
        return

    total = sum(counts.values())
    sorted_authors = sorted(counts.items(), key=lambda x: x[1], reverse=True)

    try:
        current_user = run_git(["config", "user.name"])
    except GitError:
        current_user = ""

    rows = []
    for i, (author, lines) in enumerate(sorted_authors[:n], 1):
        display_name = "You" if author == current_user else author
        pct = f"{lines / total * 100:.1f}%"
        last_edit = _get_author_last_edit(author, filepath)
        rows.append([str(i), display_name, str(lines), pct, last_edit])

    console.print()
    print_table(
        headers=["#", "Author", "Lines", "%", "Last Edit"],
        rows=rows,
        title=f"Ownership of {filepath} ({total} lines)",
    )


def _dir_level(directory: str, n: int, since: str | None, show_email: bool, no_limit: bool) -> None:
    """Show who knows a directory best."""
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

    total_counts: dict[str, int] = defaultdict(int)
    files_touched: dict[str, set] = defaultdict(set)

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        console=console,
    ) as progress:
        task = progress.add_task(f"Analyzing {len(files)} files...", total=len(files))

        def blame_one(f: str) -> tuple[str, dict[str, int]]:
            return f, _blame_file(f, since=since)

        with ThreadPoolExecutor(max_workers=8) as pool:
            futures = {pool.submit(blame_one, f): f for f in files}
            for future in as_completed(futures):
                filepath, counts = future.result()
                for author, lines in counts.items():
                    total_counts[author] += lines
                    files_touched[author].add(filepath)
                progress.advance(task)

    total_lines = sum(total_counts.values())
    if total_lines == 0:
        print_info(f"No blame data for {directory}")
        return

    sorted_authors = sorted(total_counts.items(), key=lambda x: x[1], reverse=True)

    try:
        current_user = run_git(["config", "user.name"])
    except GitError:
        current_user = ""

    rows = []
    for i, (author, lines) in enumerate(sorted_authors[:n], 1):
        display_name = "You" if author == current_user else author
        pct = f"{lines / total_lines * 100:.1f}%"
        ft = str(len(files_touched.get(author, set())))
        last_edit = _get_author_last_edit(author, directory)
        rows.append([str(i), display_name, str(lines), pct, ft, last_edit])

    console.print()
    print_table(
        headers=["#", "Author", "Lines", "%", "Files Touched", "Last Edit"],
        rows=rows,
        title=f"Ownership of {directory} ({len(files)} files, {total_lines} lines)",
    )


def who(
    path: str = typer.Argument(None, help="File or directory to analyze (omit for repo-level)."),
    n: int = typer.Option(DEFAULT_WHO_LIMIT, "-n", "--number", help="Number of contributors to show."),
    since: str = typer.Option(None, "--since", help="Only consider commits after this date."),
    email: bool = typer.Option(False, "--email", help="Show email addresses."),
    no_limit: bool = typer.Option(False, "--no-limit", help="Remove file cap for directory analysis."),
) -> None:
    """Show who knows a file, directory, or repo best."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    if path is None:
        _repo_level(n, since, email)
    elif os.path.isdir(path):
        _dir_level(path, n, since, email, no_limit)
    else:
        _file_level(path, n, since, email)
