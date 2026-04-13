"""gx retarget — Rebase a branch onto a new base."""

from __future__ import annotations

import shutil
import subprocess
from typing import Optional

import typer

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
    branch_exists,
    ensure_git_repo,
    get_current_branch,
    run_git,
)
from gx.utils.stack import get_parent, get_parent_head, record_relationship


def retarget(
    branch: Optional[str] = typer.Argument(None, help="Branch to retarget."),
    new_target: Optional[str] = typer.Argument(None, help="New base branch."),
    dry_run: bool = typer.Option(False, "--dry-run", help="Show what would happen."),
) -> None:
    """Rebase a branch onto a new base and update stack config."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    # Default branch to current if not specified
    if branch is None:
        try:
            branch = get_current_branch()
        except GitError as e:
            print_error(str(e))
            raise typer.Exit(1)

    if new_target is None:
        print_error("New target branch is required. Usage: gx retarget <branch> <new-target>")
        raise typer.Exit(1)

    # Validate
    if not branch_exists(branch):
        print_error(f"Branch '{branch}' does not exist.")
        raise typer.Exit(1)

    if not branch_exists(new_target):
        print_error(f"Target branch '{new_target}' does not exist.")
        raise typer.Exit(1)

    # Find old parent — prefer stored parent_head (exact SHA) for --onto
    old_parent = get_parent(branch)
    old_parent_ref = get_parent_head(branch) or old_parent
    if old_parent_ref is None:
        try:
            old_parent_ref = run_git(["merge-base", branch, new_target])
            old_parent = old_parent_ref
            print_warning(
                f"No saved parent for {branch}. Using merge-base as old parent."
            )
        except GitError:
            print_error(f"Cannot determine old parent for {branch}.")
            raise typer.Exit(1)

    if old_parent == new_target:
        print_info(f"{branch} is already based on {new_target}. Nothing to do.")
        raise typer.Exit(0)

    console.print()
    console.print(f"[bold]Retargeting {branch} onto {new_target}...[/bold]")
    console.print()
    console.print(f"  Old parent: {old_parent}")
    console.print(f"  New parent: {new_target}")
    console.print()

    if dry_run:
        # Determine the remote ref to use
        remote_target = f"origin/{new_target}" if branch_exists(new_target, remote=True) else new_target
        print_dry_run([
            f"Would retarget {branch}:",
            f"  Current parent: {old_parent}",
            f"  New parent:     {new_target}",
            "",
            "Steps:",
            "  1. Fetch remote",
            f"  2. Run: git rebase --onto {remote_target} {old_parent_ref} {branch}",
            "  3. Push with --force-with-lease",
            "  4. Update stack config",
            "  5. Attempt PR retarget via gh CLI",
        ])
        return

    if not confirm_action(f"Retarget {branch} onto {new_target}?"):
        print_info("Cancelled.")
        raise typer.Exit(0)

    # Fetch remote
    console.print("  Fetching latest from remote...")
    try:
        run_git(["fetch", "origin"], check=False)
    except GitError:
        pass

    # Determine remote ref
    remote_target = f"origin/{new_target}" if branch_exists(new_target, remote=True) else new_target

    # Rebase with --onto
    console.print(f"  Rebasing {branch} onto {remote_target} (using --onto)...")
    try:
        result = subprocess.run(
            ["git", "rebase", "--onto", remote_target, old_parent_ref, branch],
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=120,
        )
        if result.returncode != 0:
            print_error("Rebase conflict encountered")
            console.print()
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
            console.print("    1. Fix the conflicts")
            console.print("    2. Run: git add . && git rebase --continue")
            console.print(f"    3. Run: gx retarget {branch} {new_target}")
            raise typer.Exit(1)
    except subprocess.TimeoutExpired:
        print_error("Rebase timed out.")
        raise typer.Exit(1)

    # Push
    try:
        run_git(["push", "--force-with-lease", "origin", branch])
        print_success(f"Rebased and pushed {branch}")
    except GitError as e:
        print_warning(f"Rebased but failed to push: {e}")

    # Update stack config
    record_relationship(branch, new_target)
    console.print(f"  Stack config updated: {branch} -> {new_target} (was: {old_parent})")

    # Attempt auto-retarget via gh CLI
    gh = shutil.which("gh")
    if gh:
        try:
            result = subprocess.run(
                [gh, "pr", "edit", branch, "--base", new_target],
                capture_output=True,
                text=True,
                timeout=15,
            )
            if result.returncode == 0:
                print_success(f"PR for {branch} automatically retargeted to {new_target}")
            else:
                print_warning(f"Could not auto-retarget PR. Retarget it manually to {new_target}.")
        except (subprocess.TimeoutExpired, FileNotFoundError):
            print_warning(f"Could not auto-retarget PR. Retarget it manually to {new_target}.")
    else:
        print_warning(f"gh CLI not found. Remember to retarget the PR for {branch} to {new_target}.")
