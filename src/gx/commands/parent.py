"""gx parent: Print the stack parent branch name."""

from __future__ import annotations

import sys

import typer

from gx.utils.git import GitError, ensure_git_repo, get_current_branch, get_head_branch
from gx.utils.stack import get_parent


def parent() -> None:
    """Print the parent branch name (for composability with other git commands)."""
    try:
        ensure_git_repo()
    except GitError:
        raise typer.Exit(1)

    try:
        current = get_current_branch()
    except GitError:
        raise typer.Exit(1)

    head = get_head_branch()
    if current == head:
        raise typer.Exit(1)

    p = get_parent(current)
    sys.stdout.write((p or head) + "\n")
