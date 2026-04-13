"""gx handoff: Branch summary generator for PRs, Slack, or standups."""

from __future__ import annotations

import platform
import subprocess
from dataclasses import dataclass
from typing import Optional

import typer

from gx.utils.display import console, print_error, print_info, print_success, print_warning
from gx.utils.git import (
    GitError,
    branch_exists,
    ensure_git_repo,
    get_current_branch,
    get_head_branch,
    run_git,
)
from gx.utils.stack import get_parent


@dataclass
class HandoffData:
    branch: str
    base: str
    commits: list[tuple[str, str]]  # (short_hash, message)
    stat_summary: str
    files: list[str]


def _gather_data(base: str) -> HandoffData:
    current = get_current_branch()

    # Commits
    log_output = run_git(["log", "--oneline", "--no-decorate", f"{base}..HEAD"], check=False)
    commits: list[tuple[str, str]] = []
    for line in log_output.strip().splitlines():
        if line:
            parts = line.split(" ", 1)
            if len(parts) == 2:
                commits.append((parts[0], parts[1]))

    # File stats summary
    stat_output = run_git(["diff", "--stat", f"{base}...HEAD"], check=False)
    stat_summary = ""
    if stat_output:
        stat_lines = stat_output.strip().splitlines()
        if stat_lines:
            stat_summary = stat_lines[-1].strip()

    # Changed files
    files_output = run_git(["diff", "--name-only", f"{base}...HEAD"], check=False)
    files = [f for f in files_output.strip().splitlines() if f]

    return HandoffData(
        branch=current,
        base=base,
        commits=commits,
        stat_summary=stat_summary,
        files=files,
    )


def _format_plain(data: HandoffData, is_stacked: bool) -> str:
    if is_stacked:
        header = f"{data.branch} (on {data.base})"
    else:
        header = f"{data.branch} (vs {data.base})"

    lines = [header, ""]
    lines.append(f"Commits ({len(data.commits)}):")
    for short_hash, message in data.commits:
        lines.append(f"  {short_hash}  {message}")
    lines.append("")
    lines.append(data.stat_summary)
    lines.append("")
    lines.append("Files:")
    for f in data.files:
        lines.append(f"  {f}")

    return "\n".join(lines)


def _format_markdown(data: HandoffData) -> str:
    lines = [
        f"## {data.branch}",
        f"**Base:** {data.base} "
        f"· **{len(data.commits)} commit{'s' if len(data.commits) != 1 else ''}** "
        f"· {data.stat_summary}",
        "",
        "### Commits",
    ]
    for short_hash, message in data.commits:
        lines.append(f"- `{short_hash}` {message}")
    lines.append("")
    lines.append("### Files Changed")
    for f in data.files:
        lines.append(f"- `{f}`")

    return "\n".join(lines)


def _copy_to_clipboard(text: str) -> bool:
    system = platform.system()
    try:
        if system == "Darwin":
            subprocess.run(["pbcopy"], input=text.encode(), check=True)
        elif system == "Linux":
            try:
                subprocess.run(
                    ["xclip", "-selection", "clipboard"],
                    input=text.encode(), check=True,
                )
            except FileNotFoundError:
                subprocess.run(
                    ["xsel", "--clipboard", "--input"],
                    input=text.encode(), check=True,
                )
        elif system == "Windows":
            subprocess.run(["clip"], input=text.encode(), check=True)
        else:
            return False
        return True
    except (subprocess.CalledProcessError, FileNotFoundError):
        return False


def handoff(
    against: Optional[str] = typer.Option(None, "--against", help="Compare against a specific branch."),
    copy: bool = typer.Option(False, "--copy", "-c", help="Copy output to clipboard."),
    markdown: bool = typer.Option(False, "--markdown", "--md", help="Output in markdown format."),
) -> None:
    """Generate a branch summary for PRs, Slack, or standups."""
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

    # Determine base
    is_stacked = False
    if against:
        if not branch_exists(against):
            print_error(f"Branch '{against}' does not exist.")
            raise typer.Exit(1)
        base = against
    else:
        parent = get_parent(current)
        if parent:
            base = parent
            is_stacked = True
        else:
            base = get_head_branch()

    # Gather
    data = _gather_data(base)

    if not data.commits:
        print_info(f"No changes between {current} and {base}.")
        raise typer.Exit(0)

    # Format
    if markdown:
        output = _format_markdown(data)
    else:
        output = _format_plain(data, is_stacked)

    console.print(output)

    # Clipboard
    if copy:
        if _copy_to_clipboard(output):
            console.print()
            print_success("Copied to clipboard.")
        else:
            console.print()
            print_warning("Could not copy to clipboard.")
