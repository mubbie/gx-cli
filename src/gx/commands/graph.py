"""gx graph: Visualize the branch stack tree."""

from __future__ import annotations

import typer

from gx.utils.display import print_error
from gx.utils.git import GitError, ensure_git_repo
from gx.utils.stack import build_branch_stack
from gx.utils.stack_render import render_branch_stack


def graph() -> None:
    """Visualize the branch stack tree."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    stack = build_branch_stack()
    render_branch_stack(stack)
