"""gx stack -- Create a new branch on top of a parent branch."""

from __future__ import annotations

from typing import Optional

import typer

from gx.utils.display import (
    console,
    print_error,
    print_info,
    print_success,
    print_warning,
)
from gx.utils.git import (
    GitError,
    branch_exists,
    ensure_git_repo,
    is_clean_working_tree,
    run_git,
    time_ago,
)
from gx.utils.stack import record_relationship


def _is_valid_branch_name(name: str) -> bool:
    """Check if a string is a valid git branch name."""
    if not name or name.startswith("-"):
        return False
    # Basic git ref validation
    invalid_patterns = ["..", "~", "^", ":", "\\", " ", "[", "?", "*"]
    for pat in invalid_patterns:
        if pat in name:
            return False
    if name.endswith(".") or name.endswith(".lock") or name.endswith("/"):
        return False
    if name.startswith("/"):
        return False
    return True


def _pick_parent() -> str | None:
    """Interactive parent branch picker."""
    fmt = "%(refname:short)\t%(authordate:iso8601)\t%(authorname)"
    output = run_git(["branch", f"--format={fmt}", "--sort=-committerdate"])
    if not output:
        print_info("No branches available.")
        return None

    branches = []
    for line in output.splitlines():
        parts = line.split("\t", 2)
        if len(parts) >= 3:
            branches.append({"name": parts[0], "date": parts[1], "author": parts[2]})

    query = ""
    while True:
        filtered = (
            [b for b in branches if query in b["name"].lower()]
            if query
            else branches
        )

        console.print()
        console.print("[bold]Select parent branch:[/bold]")
        console.print()
        if not filtered:
            console.print("  [dim]No branches match your search.[/dim]")
        else:
            for i, b in enumerate(filtered, 1):
                age = time_ago(b["date"])
                console.print(f"  {i:>3}  {b['name']:<40} {age:<15} {b['author']}")

        console.print()
        try:
            if filtered:
                hint = "[dim]Enter a number to select, text to filter, q to cancel[/dim]"
            else:
                hint = "[dim]Enter text to filter, q to cancel[/dim]"
            console.print(hint)
            choice = console.input("> ").strip()
        except (EOFError, KeyboardInterrupt):
            console.print()
            return None

        if choice.lower() == "q" or choice == "":
            return None

        try:
            idx = int(choice) - 1
            if 0 <= idx < len(filtered):
                return str(filtered[idx]["name"])
            console.print(f"  [red]Invalid number. Pick 1-{len(filtered)}.[/red]")
            continue
        except ValueError:
            pass

        query = choice.lower()


def stack(
    new_branch: Optional[str] = typer.Argument(None, help="Name for the new branch."),
    parent: Optional[str] = typer.Argument(None, help="Parent branch to stack on."),
) -> None:
    """Create a new branch on top of a parent branch."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    # Interactive mode if no args
    if parent is None:
        parent = _pick_parent()
        if parent is None:
            print_info("Cancelled.")
            raise typer.Exit(0)

    if new_branch is None:
        try:
            console.print()
            new_branch = console.input("New branch name: ").strip()
        except (EOFError, KeyboardInterrupt):
            console.print()
            print_info("Cancelled.")
            raise typer.Exit(0)

        if not new_branch:
            print_info("Cancelled.")
            raise typer.Exit(0)

    # Validate
    if not _is_valid_branch_name(new_branch):
        print_error(f"Invalid branch name: {new_branch}")
        raise typer.Exit(1)

    if branch_exists(new_branch):
        print_error(f"Branch '{new_branch}' already exists.")
        raise typer.Exit(1)

    if not branch_exists(parent):
        print_error(f"Parent branch '{parent}' does not exist.")
        raise typer.Exit(1)

    if not is_clean_working_tree():
        print_warning("You have uncommitted changes. They will carry over to the new branch.")

    # Create the branch
    try:
        run_git(["checkout", "-b", new_branch, parent])
    except GitError as e:
        print_error(f"Failed to create branch: {e}")
        raise typer.Exit(1)

    # Record the relationship
    record_relationship(new_branch, parent)

    console.print()
    print_success(f"Created {new_branch} on top of {parent}")
    console.print("  Relationship saved to stack config.")
