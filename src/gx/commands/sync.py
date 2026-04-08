"""gx sync — Rebase and push a chain of stacked branches."""

from __future__ import annotations

import subprocess
from typing import List, Optional

import typer

from gx.utils.config import SYNC_CONFIRM_THRESHOLD
from gx.utils.display import (
    confirm_action,
    console,
    print_dry_run,
    print_error,
    print_info,
    print_success,
    print_warning,
)
from gx.utils.git import (
    GitError,
    ensure_git_repo,
    get_current_branch,
    run_git,
    supports_update_refs,
)
from gx.utils.stack import get_descendants, get_stack_chain


def _sync_with_update_refs(chain: list[str]) -> bool:
    """Sync using --update-refs (Git 2.38+). Returns True on success."""
    root = chain[0]
    tip = chain[-1]

    console.print(f"  Rebasing stack onto {root} (using --update-refs)...")

    try:
        run_git(["checkout", tip])
    except GitError as e:
        print_error(f"Failed to checkout {tip}: {e}")
        return False

    try:
        # Use longer timeout for rebase
        result = subprocess.run(
            ["git", "rebase", "--update-refs", root],
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=120,
        )
        if result.returncode != 0:
            _handle_rebase_failure(result.stderr, chain)
            return False
    except subprocess.TimeoutExpired:
        print_error("Rebase timed out.")
        return False

    for branch in chain[1:]:
        print_success(f"Rebased {branch}")

    return True


def _sync_with_onto(chain: list[str]) -> bool:
    """Sync using --onto iteration (Git < 2.38 fallback). Returns True on success."""
    console.print("  Rebasing stack (using --onto fallback)...")

    for i in range(1, len(chain)):
        parent = chain[i - 1]
        branch = chain[i]

        # Record old SHA before rebase
        try:
            old_sha = run_git(["rev-parse", parent])
        except GitError:
            old_sha = ""

        if i == 1:
            # First branch: simple rebase
            try:
                result = subprocess.run(
                    ["git", "rebase", parent, branch],
                    capture_output=True,
                    text=True,
                    encoding="utf-8",
                    errors="replace",
                    timeout=120,
                )
                if result.returncode != 0:
                    _handle_rebase_failure(result.stderr, chain[i:])
                    return False
            except subprocess.TimeoutExpired:
                print_error(f"Rebase of {branch} timed out.")
                return False
        else:
            # Subsequent branches: use --onto with old/new parent SHAs
            try:
                new_parent_sha = run_git(["rev-parse", parent])
            except GitError as e:
                print_error(f"Failed to get SHA for {parent}: {e}")
                return False

            try:
                result = subprocess.run(
                    ["git", "rebase", "--onto", new_parent_sha, old_sha, branch],
                    capture_output=True,
                    text=True,
                    encoding="utf-8",
                    errors="replace",
                    timeout=120,
                )
                if result.returncode != 0:
                    _handle_rebase_failure(result.stderr, chain[i:])
                    return False
            except subprocess.TimeoutExpired:
                print_error(f"Rebase of {branch} timed out.")
                return False

        print_success(f"Rebased {branch}")

    return True


def _handle_rebase_failure(stderr: str, remaining: list[str]) -> None:
    """Show conflict info and resolution steps."""
    print_error("Rebase conflict encountered")
    console.print()

    # Try to find conflicting files
    try:
        conflicts = run_git(["diff", "--name-only", "--diff-filter=U"], check=False)
        if conflicts:
            console.print("  Conflicting files:")
            for f in conflicts.splitlines():
                console.print(f"    {f}")
            console.print()
    except GitError:
        pass

    console.print("  To resolve:")
    console.print("    1. Fix the conflicts in the listed files")
    console.print("    2. Run: git add . && git rebase --continue")
    if len(remaining) > 1:
        rest = " ".join(remaining)
        console.print(f"    3. Run: gx sync {rest}")
        console.print("       (to continue syncing the rest of the stack)")
    console.print()
    console.print("  Sync stopped. Downstream branches were not updated.")


def _push_branches(branches: list[str]) -> None:
    """Push branches with --force-with-lease."""
    console.print()
    console.print("  Pushing updated branches...")
    for branch in branches:
        try:
            run_git(["push", "--force-with-lease", "origin", branch])
            print_success(f"Pushed {branch}")
        except GitError as e:
            print_warning(f"Failed to push {branch}: {e}")


def sync(
    branches: Optional[List[str]] = typer.Argument(None, help="Branches to sync in order."),
    stack_flag: bool = typer.Option(
        False, "--stack", help="Auto-detect and sync the current branch's full stack."
    ),
    dry_run: bool = typer.Option(False, "--dry-run", help="Show what would happen."),
) -> None:
    """Rebase and push a chain of stacked branches in sequence."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    try:
        original_branch = get_current_branch()
    except GitError:
        original_branch = ""

    # Determine the chain
    if stack_flag:
        chain = _auto_detect_chain()
        if not chain:
            print_info("No stack detected for the current branch.")
            raise typer.Exit(0)
    elif branches:
        chain = list(branches)
    else:
        # Interactive: show the stack and let user confirm
        chain = _auto_detect_chain()
        if not chain:
            print_error("No branches specified. Use --stack or provide branch names.")
            raise typer.Exit(1)

    if len(chain) < 2:
        print_info("Need at least 2 branches (root + 1 child) to sync.")
        raise typer.Exit(0)

    root = chain[0]
    sync_branches = chain[1:]

    console.print()
    console.print(f"[bold]Syncing stack:[/bold] {' -> '.join(chain)}")
    console.print()

    use_update_refs = supports_update_refs()

    if dry_run:
        strategy = "--update-refs" if use_update_refs else "--onto (fallback)"
        major, minor, _ = __import__("gx.utils.git", fromlist=["get_git_version"]).get_git_version()
        actions = [
            f"Would sync stack: {' -> '.join(chain)}",
            "",
            f"Git {major}.{minor} detected — will use {strategy} strategy",
        ]
        if use_update_refs:
            actions.append(f"Step 1: Checkout {chain[-1]} (tip of stack)")
            actions.append(f"Step 2: Run: git rebase --update-refs {root}")
        else:
            for i, branch in enumerate(sync_branches, 1):
                parent = chain[i - 1]
                actions.append(f"Step {i}: Rebase {branch} onto {parent}")
        actions.append(
            f"Push {', '.join(sync_branches)} with --force-with-lease"
        )
        print_dry_run(actions)
        return

    # Confirm for long chains
    if len(sync_branches) >= SYNC_CONFIRM_THRESHOLD:
        if not confirm_action(f"Sync {len(sync_branches)} branches?"):
            print_info("Cancelled.")
            raise typer.Exit(0)

    # Execute sync
    if use_update_refs:
        success = _sync_with_update_refs(chain)
    else:
        print_info("Git < 2.38 detected. Using --onto fallback.")
        success = _sync_with_onto(chain)

    if success:
        _push_branches(sync_branches)
        console.print()
        print_success(f"Stack sync complete. {len(sync_branches)} branches updated.")

    # Return to original branch
    if original_branch:
        try:
            run_git(["checkout", original_branch])
        except GitError:
            pass


def _auto_detect_chain() -> list[str]:
    """Auto-detect the full stack chain from the current branch."""
    try:
        current = get_current_branch()
    except GitError:
        return []

    # Walk up to find the root
    chain_up = get_stack_chain(current)

    # Walk down to find descendants
    descendants = get_descendants(current)

    # Combine: chain_up already includes current, descendants are below
    return chain_up + descendants
