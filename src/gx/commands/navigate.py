"""gx up / gx down / gx top / gx bottom — Stack navigation."""

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


def top() -> None:
    """Jump to the top of the stack (furthest from trunk)."""
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

    target = current
    visited: set[str] = {current}

    while True:
        children = sorted(get_children(target))
        if not children:
            break
        if len(children) > 1:
            print_info(f"Stack branches at {target}. Cannot determine a single top.")
            print_info(f"Children: {', '.join(children)}")
            print_info("Use `gx switch` to pick a specific branch.")
            raise typer.Exit(0)
        next_branch = children[0]
        if next_branch in visited:
            print_warning("Cycle detected in stack config.")
            break
        visited.add(next_branch)
        target = next_branch

    if target == current:
        print_info("Already at the top of the stack.")
        raise typer.Exit(0)

    if not is_clean_working_tree():
        print_warning("You have uncommitted changes. They may conflict with the target branch.")

    try:
        run_git(["checkout", target])
        print_success(f"Jumped to top: {current} -> {target}")
    except GitError as e:
        print_error(f"Failed to switch: {e}")
        raise typer.Exit(1)


def bottom() -> None:
    """Jump to the bottom of the stack (closest to trunk)."""
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

    head_branch = get_head_branch()

    # If on trunk or not in stack, try to enter the stack
    parent = get_parent(current)
    if current == head_branch or parent is None:
        children = sorted(get_children(current))
        if not children:
            print_info("No stack branches found.")
            raise typer.Exit(0)
        if len(children) == 1:
            if not is_clean_working_tree():
                print_warning("You have uncommitted changes.")
            try:
                run_git(["checkout", children[0]])
                print_success(f"Jumped to bottom: {current} -> {children[0]}")
            except GitError as e:
                print_error(f"Failed to switch: {e}")
                raise typer.Exit(1)
        else:
            print_info(f"Multiple stacks branching from {current}:")
            for child in children:
                print_info(f"  {child}")
            print_info("Use `gx switch` to pick one.")
        return

    # Walk down toward trunk to find the bottom-most managed branch
    target = current
    visited: set[str] = {current}

    while True:
        p = get_parent(target)
        if p is None or p == head_branch or p not in visited and get_parent(p) is None:
            break
        if p in visited:
            print_warning("Cycle detected in stack config.")
            break
        visited.add(p)
        target = p

    if target == current:
        print_info("Already at the bottom of the stack.")
        raise typer.Exit(0)

    if not is_clean_working_tree():
        print_warning("You have uncommitted changes. They may conflict with the target branch.")

    try:
        run_git(["checkout", target])
        print_success(f"Jumped to bottom: {current} -> {target}")
    except GitError as e:
        print_error(f"Failed to switch: {e}")
        raise typer.Exit(1)
