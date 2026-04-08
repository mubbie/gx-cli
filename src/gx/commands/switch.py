"""gx switch — Fuzzy-find branch switcher with rich context."""

from __future__ import annotations

import typer

from gx.utils.display import console, print_error, print_info, print_success, print_warning
from gx.utils.git import (
    GitError,
    ensure_git_repo,
    get_current_branch,
    get_head_branch,
    is_clean_working_tree,
    run_git,
    time_ago,
)


def _get_branches_with_info() -> list[dict]:
    """Get all local branches with metadata."""
    fmt = "%(refname:short)\t%(authordate:iso8601)\t%(authorname)\t%(objectname:short)"
    output = run_git(["branch", f"--format={fmt}", "--sort=-committerdate"])
    if not output:
        return []

    branches = []
    try:
        current = get_current_branch()
    except GitError:
        current = ""

    for line in output.splitlines():
        parts = line.split("\t", 3)
        if len(parts) < 4:
            continue
        branches.append({
            "name": parts[0],
            "date": parts[1],
            "author": parts[2],
            "hash": parts[3],
            "current": parts[0] == current,
        })
    return branches


def switch(
    shortcut: str = typer.Argument(None, help='Use "-" to switch to previous branch.'),
) -> None:
    """Fuzzy branch switcher with rich context."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    # Handle gx switch -
    if shortcut == "-":
        try:
            run_git(["switch", "-"])
            new_branch = get_current_branch()
            print_success(f"Switched to {new_branch}")
        except GitError as e:
            if "no previous branch" in str(e).lower() or "invalid reference" in str(e).lower():
                print_error("No previous branch to switch to.")
            else:
                print_error(f"Failed to switch: {e}")
            raise typer.Exit(1)
        return

    # Warn about dirty working tree
    if not is_clean_working_tree():
        print_warning("You have uncommitted changes. They may conflict with the target branch.")

    branches = _get_branches_with_info()
    if not branches:
        print_info("No branches to switch to.")
        raise typer.Exit(0)

    # Filter out current branch
    try:
        current = get_current_branch()
    except GitError:
        current = ""

    selectable = [b for b in branches if b["name"] != current]
    if not selectable:
        print_info("No other branches to switch to.")
        raise typer.Exit(0)

    if len(selectable) == 1:
        target = selectable[0]["name"]
        try:
            run_git(["switch", target])
            print_success(f"Switched to {target}")
        except GitError as e:
            print_error(f"Failed to switch: {e}")
            raise typer.Exit(1)
        return

    # Launch Textual TUI
    try:
        from gx.commands._switch_tui import run_switch_tui
        selected = run_switch_tui(selectable, get_head_branch())
    except ImportError:
        # Fallback to simple selection if Textual is not available or TUI fails
        selected = _fallback_picker(selectable)

    if selected is None:
        print_info("Cancelled.")
        raise typer.Exit(0)

    try:
        run_git(["switch", selected])
        print_success(f"Switched to {selected}")
    except GitError as e:
        print_error(f"Failed to switch: {e}")
        raise typer.Exit(1)


def _fallback_picker(branches: list[dict]) -> str | None:
    """Simple numbered list picker as fallback."""
    console.print()
    console.print("[bold]Select a branch:[/bold]")
    console.print()
    for i, b in enumerate(branches, 1):
        age = time_ago(b["date"])
        console.print(f"  {i:>3}  {b['name']:<40} {age:<15} {b['author']}")

    console.print()
    try:
        choice = console.input("Enter number (or q to cancel): ")
        if choice.strip().lower() in ("q", ""):
            return None
        idx = int(choice.strip()) - 1
        if 0 <= idx < len(branches):
            return branches[idx]["name"]
        return None
    except (ValueError, EOFError, KeyboardInterrupt):
        return None
