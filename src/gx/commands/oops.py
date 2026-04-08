"""gx oops — Quick-fix the last commit."""

from __future__ import annotations

import os
from typing import List, Optional

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
    ensure_git_repo,
    get_last_commit,
    is_commit_pushed,
    run_git,
)


def oops(
    message: Optional[str] = typer.Option(None, "-m", "--message", help="New commit message."),
    add: Optional[List[str]] = typer.Option(None, "--add", help="File(s) to add to the last commit."),
    dry_run: bool = typer.Option(False, "--dry-run", help="Show what would change."),
    force: bool = typer.Option(False, "--force", help="Allow amending even if already pushed."),
) -> None:
    """Quick-fix the last commit \u2014 amend message, add forgotten files, or both."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    commit = get_last_commit()
    if not commit:
        print_error("No commits yet.")
        raise typer.Exit(1)

    # Safety: check if pushed
    if is_commit_pushed() and not force:
        print_error(
            "The last commit has already been pushed to remote.\n"
            "  Amending it would rewrite shared history.\n"
            "  Use --force to override (dangerous)."
        )
        raise typer.Exit(1)

    if is_commit_pushed() and force:
        print_warning("The last commit has been pushed. Amending will require a force push.")

    # Validate --add files
    if add:
        for filepath in add:
            if filepath != "." and not os.path.exists(filepath):
                print_error(f"File not found: {filepath}")
                raise typer.Exit(1)

    # If neither message nor add specified, open editor for message
    if message is None and not add:
        # Amend with editor
        console.print()
        console.print(f'[bold]Last commit:[/bold] "{commit["message"]}" ({commit["short_hash"]})')
        console.print()

        if dry_run:
            print_dry_run(["Would open editor to amend commit message."])
            return

        if not confirm_action("Open editor to amend commit message?"):
            print_info("Cancelled.")
            raise typer.Exit(0)

        try:
            import subprocess
            subprocess.run(["git", "commit", "--amend"], check=True)
            print_success("Commit message amended.")
        except (subprocess.CalledProcessError, FileNotFoundError) as e:
            print_error(f"Failed to amend: {e}")
            raise typer.Exit(1)
        return

    # Show what will happen
    console.print()
    console.print(f'[bold]Last commit:[/bold] "{commit["message"]}" ({commit["short_hash"]})')
    console.print()

    actions = []
    dry_actions = []

    if add:
        # Check for actual changes in files
        files_to_add = []
        for filepath in add:
            if filepath == ".":
                files_to_add.append(".")
                actions.append("Adding all unstaged changes to last commit")
                dry_actions.append("Would add all unstaged changes to last commit")
            else:
                # Check if file has changes
                diff = run_git(["diff", "--name-only", filepath], check=False)
                staged = run_git(["diff", "--cached", "--name-only", filepath], check=False)
                status = run_git(["status", "--porcelain", filepath], check=False)
                if not diff and not staged and not status:
                    print_info(f"No changes in {filepath} — skipping.")
                    continue
                files_to_add.append(filepath)
                actions.append(f"  + {filepath}")
                dry_actions.append(f"Would add {filepath} to last commit")

        if not files_to_add:
            print_info("No files with changes to add.")
            raise typer.Exit(0)

        console.print("  [bold]Adding to last commit:[/bold]")
        for a in actions:
            console.print(f"    {a}")
        console.print()

    if message:
        console.print("  [bold]Amending message:[/bold]")
        console.print(f'    Before: "{commit["message"]}"')
        console.print(f'    After:  "{message}"')
        console.print()
        dry_actions.append(f'Would change message from "{commit["message"]}" to "{message}"')

    if not message and add:
        console.print(f'  Commit message stays: "{commit["message"]}"')
        console.print()

    if dry_run:
        print_dry_run(dry_actions)
        return

    if not confirm_action("Proceed?"):
        print_info("Cancelled.")
        raise typer.Exit(0)

    # Execute
    try:
        if add:
            for filepath in add:
                run_git(["add", filepath])

        amend_args = ["commit", "--amend"]
        if message:
            amend_args.extend(["-m", message])
        else:
            amend_args.append("--no-edit")
        run_git(amend_args)
    except GitError as e:
        print_error(f"Failed: {e}")
        raise typer.Exit(1)

    console.print()
    if message and add:
        print_success("File added and commit message amended.")
    elif message:
        print_success("Commit message amended.")
    elif add:
        print_success("File added to last commit.")
