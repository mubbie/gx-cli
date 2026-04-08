"""gx switch — Branch switcher with search and rich context."""

from __future__ import annotations

import typer

from gx.utils.display import console, print_error, print_info, print_success, print_warning
from gx.utils.git import (
    GitError,
    ensure_git_repo,
    get_current_branch,
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


def _pick_branch(branches: list[dict]) -> str | None:
    """Interactive branch picker with search support."""
    console.print()
    console.print("[bold]Select a branch:[/bold]")

    query = ""
    while True:
        filtered = [b for b in branches if query in b["name"].lower()] if query else branches

        console.print()
        if not filtered:
            console.print("  [dim]No branches match your search.[/dim]")
        else:
            for i, b in enumerate(filtered, 1):
                age = time_ago(b["date"])
                console.print(f"  {i:>3}  {b['name']:<40} {age:<15} {b['author']}")

        console.print()
        try:
            prompt = f"Search or pick [1-{len(filtered)}]" if filtered else "Search"
            choice = console.input(f"{prompt} (q to cancel): ").strip()
        except (EOFError, KeyboardInterrupt):
            console.print()
            return None

        if choice.lower() == "q" or choice == "":
            return None

        # Try as a number
        try:
            idx = int(choice) - 1
            if 0 <= idx < len(filtered):
                return str(filtered[idx]["name"])
            console.print(f"  [red]Invalid number. Pick 1-{len(filtered)}.[/red]")
            continue
        except ValueError:
            pass

        # Otherwise treat as search query
        query = choice.lower()


def switch(
    shortcut: str = typer.Argument(None, help='Use "-" to switch to previous branch.'),
) -> None:
    """Branch switcher with search and rich context."""
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

    selected = _pick_branch(selectable)

    if selected is None:
        print_info("Cancelled.")
        raise typer.Exit(0)

    try:
        run_git(["switch", selected])
        print_success(f"Switched to {selected}")
    except GitError as e:
        print_error(f"Failed to switch: {e}")
        raise typer.Exit(1)
