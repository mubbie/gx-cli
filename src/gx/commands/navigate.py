"""gx up / gx down — Stack navigation."""

from __future__ import annotations

import typer

from gx.utils.display import print_error, print_info, print_success, print_warning
from gx.utils.git import (
    GitError,
    ensure_git_repo,
    get_current_branch,
    get_head_branch,
    is_clean_working_tree,
    run_git,
)
from gx.utils.stack import get_children, get_parent


def up() -> None:
    """Move up the stack (to child branch)."""
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

    children = get_children(current)
    if not children:
        print_info("Already at the top of the stack.")
        raise typer.Exit(0)

    if len(children) > 1:
        print_info(f"Multiple branches stacked on {current}:")
        for child in sorted(children):
            print_info(f"  {child}")
        print_info("Use `gx switch` to pick one.")
        raise typer.Exit(0)

    target = children[0]

    if not is_clean_working_tree():
        print_warning("You have uncommitted changes. They may conflict with the target branch.")

    try:
        run_git(["checkout", target])
        print_success(f"Moved up: {current} -> {target}")
    except GitError as e:
        print_error(f"Failed to switch: {e}")
        raise typer.Exit(1)


def down() -> None:
    """Move down the stack (to parent branch)."""
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

    parent = get_parent(current)
    if parent is None:
        print_info(f"{current} is not in the stack. Use `gx switch` to navigate.")
        raise typer.Exit(0)

    if not is_clean_working_tree():
        print_warning("You have uncommitted changes. They may conflict with the target branch.")

    try:
        run_git(["checkout", parent])
    except GitError as e:
        print_error(f"Failed to switch: {e}")
        raise typer.Exit(1)

    head_branch = get_head_branch()
    if parent == head_branch:
        print_info(f"Moved down to {parent} (trunk).")
    else:
        print_success(f"Moved down: {current} -> {parent}")
