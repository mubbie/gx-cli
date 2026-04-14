"""gx view: Quick status of your current stack."""

from __future__ import annotations

import json
import shutil
import subprocess
from dataclasses import dataclass

import typer

from gx.utils.display import console, print_error, print_info
from gx.utils.git import GitError, ensure_git_repo, get_current_branch, run_git, time_ago
from gx.utils.stack import get_descendants, get_stack_chain, load_stack_config


@dataclass
class ViewRow:
    branch: str
    ahead: int
    behind: int
    age: str
    is_current: bool
    pr: dict | None


def _get_ahead_behind(branch: str, parent: str) -> tuple[int, int]:
    try:
        ab = run_git(["rev-list", "--left-right", "--count", f"{branch}...{parent}"])
        ahead, behind = ab.split()
        return int(ahead), int(behind)
    except (GitError, ValueError):
        return 0, 0


def _get_last_commit_age(branch: str) -> str:
    try:
        date_str = run_git(["log", "-1", "--format=%aI", branch])
        return time_ago(date_str)
    except GitError:
        return ""


def _has_gh() -> bool:
    return shutil.which("gh") is not None


def _get_pr_status(branch: str) -> dict | None:
    try:
        result = subprocess.run(
            ["gh", "pr", "view", branch, "--json", "number,state,reviewDecision"],
            capture_output=True, text=True, timeout=10,
        )
        if result.returncode != 0:
            return None
        data = json.loads(result.stdout)
        return dict(data) if isinstance(data, dict) else None
    except (FileNotFoundError, subprocess.TimeoutExpired, json.JSONDecodeError):
        return None


def _format_pr(pr: dict) -> str:
    number = pr.get("number", "")
    state = pr.get("state", "")
    review = pr.get("reviewDecision", "")

    if state == "MERGED":
        return f"#{number}  + merged"
    if review == "APPROVED":
        return f"#{number}  + approved"
    if review == "CHANGES_REQUESTED":
        return f"#{number}  x changes requested"
    if state == "OPEN":
        return f"#{number}  o reviewing"
    return f"#{number}  {state.lower()}"


def view() -> None:
    """Show the current stack at a glance."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    try:
        current = get_current_branch()
    except GitError:
        print_error("Not on a branch. Checkout a branch to view its stack.")
        raise typer.Exit(1)

    config = load_stack_config()
    branches = config.get("branches", {})
    trunk = config.get("metadata", {}).get("main_branch", "main")

    # On trunk: show summary of stacks
    if current == trunk:
        _show_trunk_summary(trunk, branches)
        return

    # Not in stack: show basic info
    if current not in branches:
        _show_untracked(current, trunk)
        return

    # Get full chain
    chain = get_stack_chain(current)
    descendants = get_descendants(current)

    # Merge chain + descendants, dedup
    all_branches = []
    seen: set[str] = set()
    for b in chain:
        if b != trunk and b not in seen:
            all_branches.append(b)
            seen.add(b)
    for b in descendants:
        if b not in seen:
            all_branches.append(b)
            seen.add(b)

    has_gh = _has_gh()

    # Build rows
    rows: list[ViewRow] = []
    for branch in all_branches:
        meta = branches.get(branch, {})
        parent = meta.get("parent", trunk)
        ahead, behind = _get_ahead_behind(branch, parent)
        age = _get_last_commit_age(branch)
        pr = _get_pr_status(branch) if has_gh else None
        rows.append(ViewRow(
            branch=branch,
            ahead=ahead, behind=behind,
            age=age,
            is_current=(branch == current),
            pr=pr,
        ))

    _render_view(trunk, rows, has_gh)


def _show_trunk_summary(trunk: str, branches: dict) -> None:
    console.print()
    console.print(f"You're on [bold]{trunk}[/bold] (trunk).")
    console.print()

    # Find direct children of trunk
    direct_children = sorted(
        [name for name, meta in branches.items() if meta.get("parent") == trunk]
    )
    if not direct_children:
        console.print("  No stacked branches yet. Use `gx stack` to start.")
        return

    console.print("Stacks branching from " + trunk + ":")
    for child in direct_children:
        # Count descendants
        chain = [child]
        current = child
        visited = {child}
        while True:
            kids = sorted(
                [n for n, m in branches.items() if m.get("parent") == current and n not in visited]
            )
            if not kids:
                break
            chain.append(kids[0])
            visited.add(kids[0])
            current = kids[0]

        if len(chain) == 1:
            console.print(f"  <- {child}")
        else:
            console.print(f"  <- {' -> '.join(chain)}   ({len(chain)} branches)")

    console.print()
    console.print("Use `gx graph` to see the full tree.")


def _show_untracked(current: str, trunk: str) -> None:
    console.print()
    print_info(f"Current branch ({current}) is not part of a stack.")
    ahead, behind = _get_ahead_behind(current, trunk)
    console.print(f"  vs {trunk}: {ahead} ahead, {behind} behind")
    console.print()
    console.print("  Tip: Use `gx stack` to start stacking, or `gx graph` to see all branches.")


def _render_view(trunk: str, rows: list[ViewRow], has_gh: bool) -> None:
    console.print()
    console.print(f"[bold]{trunk}[/bold]")

    for row in rows:
        parts: list[str] = [f"  <- {row.branch:<30}"]

        if has_gh and row.pr:
            parts.append(f"{_format_pr(row.pr):<22}")
        elif has_gh:
            parts.append(f"{'  no PR':<22}")

        if row.ahead == 0:
            parts.append("0 ahead   ")
        else:
            parts.append(f"{row.ahead} ahead   ")

        parts.append(f"{row.age}")

        if row.is_current:
            parts.append("  [green bold]<[/green bold]")

        console.print("".join(parts))

    console.print()
