"""gx shelf — Visual stash manager."""

from __future__ import annotations

from datetime import datetime, timezone
from typing import Optional

import typer

from gx.utils.display import (
    confirm_action,
    console,
    print_dry_run,
    print_error,
    print_info,
    print_success,
    print_table,
    print_warning,
)
from gx.utils.git import GitError, ensure_git_repo, get_current_branch, is_clean_working_tree, run_git

shelf_app = typer.Typer(
    name="shelf",
    help="Visual stash manager.",
    invoke_without_command=True,
)


@shelf_app.callback(invoke_without_command=True)
def shelf_default(ctx: typer.Context) -> None:
    """Visual stash manager -- browse, apply, and drop stashes interactively."""
    if ctx.invoked_subcommand is not None:
        return

    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    from gx.ui.shelf_app import get_stash_list, launch_shelf_browser

    stashes = get_stash_list()
    if not stashes:
        print_info("No stashes. Use `gx shelf push` to save work.")
        raise typer.Exit(0)

    result = launch_shelf_browser()

    if result is None:
        return

    # Process the action returned from the TUI
    if ":" not in result:
        return

    action, stash_id = result.split(":", 1)

    if action == "pop":
        _do_pop(stash_id)
    elif action == "apply":
        _do_apply(stash_id)
    elif action == "drop":
        _do_drop(stash_id)


def _do_pop(stash_id: str) -> None:
    """Pop a stash (apply + drop)."""
    try:
        run_git(["stash", "pop", stash_id])
        print_success(f"Popped {stash_id}")
    except GitError as e:
        err = str(e)
        if "conflict" in err.lower() or "CONFLICT" in err:
            print_warning("Stash applied but conflicts detected. Stash was NOT dropped.")
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
            console.print("    1. Fix the conflicts in the listed files")
            console.print("    2. Run: git add <resolved-files>")
            console.print(f"    3. Drop the stash manually: git stash drop {stash_id}")
        else:
            print_error(f"Failed to pop {stash_id}: {e}")


def _do_apply(stash_id: str) -> None:
    """Apply a stash (keep it in the list)."""
    try:
        run_git(["stash", "apply", stash_id])
        print_success(f"Applied {stash_id} (stash kept)")
    except GitError as e:
        err = str(e)
        if "conflict" in err.lower() or "CONFLICT" in err:
            print_warning("Stash applied but conflicts detected.")
            try:
                conflicts = run_git(["diff", "--name-only", "--diff-filter=U"], check=False)
                if conflicts:
                    console.print("  Conflicting files:")
                    for f in conflicts.splitlines():
                        console.print(f"    {f}")
            except GitError:
                pass
        else:
            print_error(f"Failed to apply {stash_id}: {e}")


def _do_drop(stash_id: str) -> None:
    """Drop a stash after confirmation."""
    if not confirm_action(f"Drop {stash_id}?"):
        print_info("Cancelled.")
        return
    try:
        run_git(["stash", "drop", stash_id])
        print_success(f"Dropped {stash_id}")
    except GitError as e:
        print_error(f"Failed to drop {stash_id}: {e}")


@shelf_app.command()
def push(
    message: Optional[str] = typer.Argument(None, help="Stash message."),
    include_untracked: bool = typer.Option(False, "-u", "--include-untracked", help="Also stash untracked files."),
) -> None:
    """Stash current work with a descriptive message."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    if is_clean_working_tree():
        # Check for untracked files if -u flag
        if include_untracked:
            untracked = run_git(["ls-files", "--others", "--exclude-standard"], check=False)
            if not untracked:
                print_info("Nothing to stash -- working tree is clean.")
                raise typer.Exit(0)
        else:
            print_info("Nothing to stash -- working tree is clean.")
            raise typer.Exit(0)

    # Auto-generate message if not provided
    if message is None:
        try:
            branch = get_current_branch()
        except GitError:
            branch = "unknown"
        ts = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M")
        message = f"gx-shelf: {branch} {ts}"

    args = ["stash", "push", "-m", message]
    if include_untracked:
        args.insert(2, "-u")

    try:
        run_git(args)
    except GitError as e:
        print_error(f"Failed to stash: {e}")
        raise typer.Exit(1)

    print_success(f'Stashed working directory: "{message}"')
    console.print("  Run `gx shelf` to browse.")


@shelf_app.command(name="list")
def list_stashes() -> None:
    """Non-interactive stash list."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    from gx.ui.shelf_app import get_stash_list

    stashes = get_stash_list()
    if not stashes:
        print_info("No stashes.")
        return

    console.print(f"\n{len(stashes)} stash{'es' if len(stashes) != 1 else ''}:\n")

    rows = []
    for s in stashes:
        rows.append([str(s.index), s.relative_time, s.branch or "--", s.message])

    print_table(
        headers=["#", "Age", "Branch", "Message"],
        rows=rows,
    )


@shelf_app.command()
def clear(
    dry_run: bool = typer.Option(False, "--dry-run", help="Show what would be dropped."),
) -> None:
    """Drop all stashes."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    from gx.ui.shelf_app import get_stash_list

    stashes = get_stash_list()
    if not stashes:
        print_info("No stashes to clear.")
        return

    console.print(f"\nThis will permanently delete ALL {len(stashes)} stashes:\n")
    for s in stashes:
        console.print(f"  {s.stash_id}  {s.relative_time:<15} {s.message}")
    console.print()

    if dry_run:
        print_dry_run([f"Would drop {len(stashes)} stashes."])
        return

    print_warning("This cannot be undone.")

    if not confirm_action("Drop all stashes?"):
        print_info("Cancelled.")
        return

    try:
        run_git(["stash", "clear"])
        print_success("All stashes cleared.")
    except GitError as e:
        print_error(f"Failed to clear stashes: {e}")
        raise typer.Exit(1)
