"""gx shelf: Visual stash manager."""

from __future__ import annotations

from dataclasses import dataclass
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


@dataclass
class StashEntry:
    index: int
    stash_id: str
    hash: str
    relative_time: str
    message: str
    branch: str
    file_stats: str


def _get_stash_list() -> list[StashEntry]:
    """Parse git stash list into StashEntry objects."""
    try:
        output = run_git(["stash", "list", "--format=%gd|%H|%ar|%s"])
    except GitError:
        return []
    if not output:
        return []

    entries: list[StashEntry] = []
    for line in output.strip().splitlines():
        parts = line.split("|", 3)
        if len(parts) < 4:
            continue
        stash_id = parts[0]
        try:
            index = int(stash_id.split("{")[1].rstrip("}"))
        except (ValueError, IndexError):
            index = len(entries)
        message = parts[3]
        branch = _parse_branch(message)

        # Get file stats for this stash
        try:
            stat_out = run_git(["stash", "show", "--stat", stash_id], check=False)
            # Last line is like " 3 files changed, 10 insertions(+), 2 deletions(-)"
            stat_lines = stat_out.strip().splitlines()
            file_stats = stat_lines[-1].strip() if stat_lines else ""
        except GitError:
            file_stats = ""

        entries.append(StashEntry(
            index=index,
            stash_id=stash_id,
            hash=parts[1],
            relative_time=parts[2],
            message=message,
            branch=branch,
            file_stats=file_stats,
        ))
    return entries


def _parse_branch(message: str) -> str:
    """Extract branch name from stash message like 'On feature/auth: WIP'."""
    if message.startswith("On "):
        colon = message.find(":")
        if colon > 3:
            return message[3:colon]
    if message.startswith("WIP on "):
        colon = message.find(":")
        if colon > 7:
            return message[7:colon]
    return ""


def _display_stashes(stashes: list[StashEntry], filter_text: str = "") -> None:
    """Print the stash list, optionally filtered."""
    filtered = stashes
    if filter_text:
        lower = filter_text.lower()
        filtered = [
            s for s in stashes
            if lower in s.message.lower() or lower in s.branch.lower()
        ]

    if not filtered:
        if filter_text:
            console.print(f"  No stashes matching '{filter_text}'")
        else:
            console.print("  No stashes.")
        return

    for s in filtered:
        branch_str = f" [{s.branch}]" if s.branch else ""
        stats_str = f"  ({s.file_stats})" if s.file_stats else ""
        console.print(
            f"  [bold]{s.index}[/bold]  {s.relative_time:<15} {s.message}{branch_str}{stats_str}"
        )


def _run_inline_picker(stashes: list[StashEntry]) -> None:
    """Run the inline interactive stash picker.

    Commands:
        <n>a  - apply stash #n (keep it)
        <n>p  - pop stash #n (apply + drop)
        <n>d  - drop stash #n
        text  - filter stashes by text
        q     - quit
    """
    console.print()
    _display_stashes(stashes)
    console.print()
    console.print("[dim]Commands: <n>a=apply  <n>p=pop  <n>d=drop  text=filter  q=quit[/dim]")

    while True:
        try:
            choice = console.input("\n> ").strip()
        except (EOFError, KeyboardInterrupt):
            console.print()
            return

        if not choice:
            continue

        if choice.lower() == "q":
            return

        # Check for action commands: <number><action>
        if len(choice) >= 2 and choice[-1] in ("a", "p", "d", "A", "P", "D"):
            num_part = choice[:-1]
            action = choice[-1].lower()
            try:
                idx = int(num_part)
            except ValueError:
                # Not a number prefix, treat as filter text
                _display_stashes(stashes, filter_text=choice)
                continue

            # Find matching stash
            target = None
            for s in stashes:
                if s.index == idx:
                    target = s
                    break

            if target is None:
                print_error(f"No stash with index {idx}")
                continue

            if action == "a":
                _do_apply(target.stash_id)
                return
            elif action == "p":
                _do_pop(target.stash_id)
                return
            elif action == "d":
                _do_drop(target.stash_id)
                # Refresh list after drop and continue
                stashes = _get_stash_list()
                if not stashes:
                    print_info("No more stashes.")
                    return
                console.print()
                _display_stashes(stashes)
                continue
        else:
            # Treat as filter text
            _display_stashes(stashes, filter_text=choice)


@shelf_app.callback(invoke_without_command=True)
def shelf_default(ctx: typer.Context) -> None:
    """Visual stash manager. Browse, apply, and drop stashes interactively."""
    if ctx.invoked_subcommand is not None:
        return

    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    stashes = _get_stash_list()
    if not stashes:
        print_info("No stashes. Use `gx shelf push` to save work.")
        raise typer.Exit(0)

    _run_inline_picker(stashes)


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
                print_info("Nothing to stash. Working tree is clean.")
                raise typer.Exit(0)
        else:
            print_info("Nothing to stash. Working tree is clean.")
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

    stashes = _get_stash_list()
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

    stashes = _get_stash_list()
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
