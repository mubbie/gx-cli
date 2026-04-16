"""gx undo: Smart undo/redo via reflog walking."""

from __future__ import annotations

import json
import os
import uuid
from datetime import datetime, timezone
from pathlib import Path

import typer

from gx.utils.config import UNDO_HISTORY_MAX_ENTRIES
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
from gx.utils.git import (
    GitError,
    ensure_git_repo,
    get_last_commit,
    get_reflog_entries,
    get_repo_root,
    is_clean_working_tree,
    run_git,
    time_ago,
)


def _history_path() -> Path:
    try:
        root = get_repo_root()
    except GitError:
        return Path(".git/gx/undo_history.json")
    return Path(root) / ".git" / "gx" / "undo_history.json"


def _load_history() -> list[dict]:
    path = _history_path()
    if not path.exists():
        return []
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
        return list(data.get("entries", []))
    except (json.JSONDecodeError, OSError):
        return []


def _save_history(entries: list[dict]) -> None:
    path = _history_path()
    path.parent.mkdir(parents=True, exist_ok=True)
    entries = entries[-UNDO_HISTORY_MAX_ENTRIES:]
    path.write_text(json.dumps({"entries": entries}, indent=2), encoding="utf-8")


def _add_history_entry(
    action_detected: str,
    description: str,
    undo_command: str,
    pre_ref: str,
    post_ref: str,
) -> None:
    entries = _load_history()
    entries.append({
        "id": str(uuid.uuid4()),
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "action_detected": action_detected,
        "description": description,
        "undo_command": undo_command,
        "pre_state_ref": pre_ref,
        "post_state_ref": post_ref,
        "undone": False,
    })
    _save_history(entries)


def _detect_state() -> dict | None:
    """Detect the most recent undoable action using priority-based state inspection."""
    try:
        root = get_repo_root()
    except GitError:
        return None

    # Priority 1: Active merge conflict
    if os.path.exists(os.path.join(root, ".git", "MERGE_HEAD")):
        return {
            "type": "merge_conflict",
            "description": "merge conflict in progress",
            "command": "git merge --abort",
            "action_msg": "Abort the merge. Returns to pre-merge state.",
        }

    # Priority 2: Rebase in progress
    rebase_merge = os.path.join(root, ".git", "rebase-merge")
    rebase_apply = os.path.join(root, ".git", "rebase-apply")
    if os.path.isdir(rebase_merge) or os.path.isdir(rebase_apply):
        return {
            "type": "rebase",
            "description": "rebase in progress",
            "command": "git rebase --abort",
            "action_msg": "Abort the rebase. Returns to pre-rebase state.",
        }

    # Priority 3: Staged files
    staged = run_git(["diff", "--cached", "--name-only"])
    if staged:
        count = len(staged.splitlines())
        return {
            "type": "stage",
            "description": f"{count} staged file{'s' if count != 1 else ''}",
            "command": "git reset HEAD",
            "action_msg": "Unstage all files. Changes stay in your working tree.",
        }

    # Priority 4+: Walk reflog for recent actions
    entries = get_reflog_entries(10)
    for entry in entries:
        action = entry.get("action", "")

        # Amend
        if "amend" in action.lower():
            return {
                "type": "amend",
                "description": "amended commit",
                "command": "git reset --soft HEAD@{1}",
                "action_msg": "Restore pre-amend state. Your changes will be preserved.",
            }

        # Commit
        if action.lower().startswith("commit"):
            commit = get_last_commit()
            if not commit:
                continue
            # Check for merge commit
            try:
                parents = run_git(["rev-list", "--parents", "-n", "1", "HEAD"])
                parent_count = len(parents.split()) - 1
            except GitError:
                parent_count = 1

            if parent_count >= 2:
                return {
                    "type": "merge_commit",
                    "description": f'merge commit "{commit["message"]}" ({commit["short_hash"]})',
                    "command": "git reset --hard HEAD~1",
                    "action_msg": "Reset to before the merge commit. WARNING: this is a hard reset.",
                    "needs_confirm_extra": True,
                }

            msg = commit["message"]
            short = commit["short_hash"]
            age = time_ago(commit["date"])
            return {
                "type": "commit",
                "description": f'commit "{msg}" ({short}, {age})',
                "command": "git reset --soft HEAD~1",
                "action_msg": "Soft reset to previous commit. Your changes will be preserved in staging.",
            }

        continue  # Skip non-matching entries and check subsequent ones

    return None


def undo(
    dry_run: bool = typer.Option(False, "--dry-run", help="See what would be undone without doing it."),
    history: bool = typer.Option(False, "--history", help="Show undo/redo history."),
) -> None:
    """Smart undo. Detects the last git action and reverses it."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    if history:
        _show_history()
        return

    state = _detect_state()
    if state is None:
        print_info("Nothing to undo.")
        raise typer.Exit(0)

    console.print()
    console.print(f"[bold]Detected:[/bold] {state['description']}")
    console.print()
    console.print(f"  Action:  {state['action_msg']}")
    console.print(f"  Command: {state['command']}")

    if dry_run:
        print_dry_run([
            f"Would run: {state['command']}",
            f"Result:    {state['action_msg']}",
        ])
        return

    if not confirm_action("Proceed with undo?"):
        print_info("Cancelled.")
        raise typer.Exit(0)

    # Capture pre-state
    try:
        pre_ref = run_git(["rev-parse", "HEAD"])
    except GitError:
        pre_ref = ""

    # Execute the undo
    try:
        cmd_parts = state["command"].replace("git ", "").split()
        run_git(cmd_parts)
    except GitError as e:
        print_error(f"Undo failed: {e}")
        raise typer.Exit(1)

    # Capture post-state
    try:
        post_ref = run_git(["rev-parse", "HEAD"])
    except GitError:
        post_ref = ""

    # Record history
    _add_history_entry(
        action_detected=state["type"],
        description=f"Undo {state['description']}",
        undo_command=state["command"],
        pre_ref=pre_ref,
        post_ref=post_ref,
    )

    console.print()
    if state["type"] == "stage":
        print_success("Unstaged files. Your changes are preserved in the working tree.")
    elif state["type"] == "commit":
        print_success("Undone. Your changes from that commit are now staged.")
        print_info("Run `gx redo` to reverse this.")
    elif state["type"] == "merge_conflict":
        print_success("Merge aborted. Working tree restored to pre-merge state.")
    elif state["type"] == "rebase":
        print_success("Rebase aborted. Working tree restored to pre-rebase state.")
    elif state["type"] == "amend":
        print_success("Restored pre-amend state.")
    elif state["type"] == "merge_commit":
        print_success("Merge commit undone.")
    else:
        print_success("Undone.")


def redo() -> None:
    """Redo the last undo."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    entries = _load_history()
    if not entries:
        print_info("Nothing to redo.")
        raise typer.Exit(0)

    # Find the last undo entry that hasn't been redone
    last_undo = None
    for entry in reversed(entries):
        if not entry.get("undone", False):
            last_undo = entry
            break

    if last_undo is None:
        print_info("Nothing to redo.")
        raise typer.Exit(0)

    # Validate state
    try:
        current_head = run_git(["rev-parse", "HEAD"])
    except GitError:
        current_head = ""

    if current_head and last_undo.get("post_state_ref") and current_head != last_undo["post_state_ref"]:
        print_error("Cannot redo. Repo state has changed since last undo.")
        console.print(f"  Expected HEAD at {last_undo['post_state_ref'][:7]}, but found {current_head[:7]}.")
        console.print("  You may have run other git commands since the undo.")
        print_info("Use `gx undo --history` to review past actions.")
        raise typer.Exit(1)

    console.print()
    console.print(f"[bold]Redoing:[/bold] {last_undo['description']}")

    if not confirm_action("Proceed with redo?"):
        print_info("Cancelled.")
        raise typer.Exit(0)

    # Check for uncommitted changes before hard reset
    if not is_clean_working_tree():
        print_warning("You have uncommitted changes that would be lost by redo.")
        if not confirm_action("Proceed anyway?"):
            print_info("Cancelled.")
            raise typer.Exit(0)

    # Redo by resetting to pre-state
    pre_ref = last_undo.get("pre_state_ref", "")
    if not pre_ref:
        print_error("Cannot redo. No pre-state reference found.")
        raise typer.Exit(1)

    action_type = last_undo.get("action_detected", "")

    try:
        if action_type == "commit":
            # Re-commit the staged changes with original message
            # First go back to the pre-undo state
            run_git(["reset", "--hard", pre_ref])
        elif action_type == "stage":
            # Re-stage: add all back
            run_git(["add", "-A"])
        else:
            run_git(["reset", "--hard", pre_ref])
    except GitError as e:
        print_error(f"Redo failed: {e}")
        raise typer.Exit(1)

    # Mark as redone
    last_undo["undone"] = True
    _save_history(entries)

    console.print()
    print_success("Redone.")


def _show_history() -> None:
    entries = _load_history()
    if not entries:
        print_info("No undo/redo history.")
        return

    console.print()
    console.print("[bold]Undo/Redo History (last 10):[/bold]")
    console.print()

    rows = []
    for i, entry in enumerate(reversed(entries[:10]), 1):
        ts = time_ago(entry.get("timestamp", ""))
        action = entry.get("action_detected", "?")
        desc = entry.get("description", "")
        status = "redone" if entry.get("undone") else "active"
        rows.append([str(i), ts, action, desc, status])

    print_table(
        headers=["#", "Time", "Action", "Description", "Status"],
        rows=rows,
    )
