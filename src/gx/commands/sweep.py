"""gx sweep -- Clean up merged branches and stale refs."""

from __future__ import annotations

import typer

from gx.utils.display import (
    confirm_action,
    console,
    print_error,
    print_success,
)
from gx.utils.git import (
    GitError,
    ensure_git_repo,
    get_current_branch,
    get_head_branch,
    run_git,
    time_ago,
)


def _get_merged_branches(head_branch: str) -> list[dict]:
    """Get branches fully merged into head_branch."""
    output = run_git(["branch", "--merged", head_branch], check=False)
    if not output:
        return []

    current = ""
    try:
        current = get_current_branch()
    except GitError:
        pass

    branches = []
    for line in output.splitlines():
        name = line.strip().lstrip("* ")
        if not name or name == head_branch or name == current:
            continue
        # Get merge date
        try:
            date_out = run_git(["log", "-1", "--format=%aI", name], check=False)
            date_str = time_ago(date_out) if date_out else "unknown"
        except GitError:
            date_str = "unknown"
        branches.append({"name": name, "date": date_str})
    return branches


def _get_squash_merged_branches(head_branch: str, already_merged: set[str]) -> list[dict]:
    """Detect branches that were squash-merged using git cherry."""
    output = run_git(["branch", "--format=%(refname:short)"], check=False)
    if not output:
        return []

    current = ""
    try:
        current = get_current_branch()
    except GitError:
        pass

    candidates = []
    for line in output.splitlines():
        name = line.strip()
        if not name or name == head_branch or name == current or name in already_merged:
            continue

        # Use git cherry to check if all commits have equivalents on head
        try:
            cherry_output = run_git(["cherry", head_branch, name], check=False)
        except GitError:
            continue

        if not cherry_output:
            continue

        # If all lines start with "-", all patches are present on head_branch
        lines = cherry_output.splitlines()
        if lines and all(line.strip().startswith("-") for line in lines):
            try:
                date_out = run_git(["log", "-1", "--format=%aI", name], check=False)
                date_str = time_ago(date_out) if date_out else "unknown"
            except GitError:
                date_str = "unknown"
            candidates.append({"name": name, "date": date_str})

    return candidates


def _get_stale_remote_refs() -> list[str]:
    """Get remote tracking refs that no longer exist on the remote."""
    try:
        # Dry-run prune to see what would be pruned
        output = run_git(["remote", "prune", "origin", "--dry-run"], check=False)
    except GitError:
        return []

    refs = []
    for line in output.splitlines():
        line = line.strip()
        if line.startswith("[would prune]"):
            ref = line.replace("[would prune]", "").strip()
            refs.append(ref)
        elif "origin/" in line and "prune" in line.lower():
            # Alternative format
            parts = line.split()
            for part in parts:
                if "origin/" in part:
                    refs.append(part)
                    break
    return refs


def sweep(
    dry_run: bool = typer.Option(False, "--dry-run", help="Show what would be cleaned."),
    yes: bool = typer.Option(False, "-y", "--yes", help="Skip confirmation prompts."),
) -> None:
    """Clean up merged branches, prune stale refs, and tidy up."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    head_branch = get_head_branch()

    console.print()
    console.print("[bold]Scanning for cleanup opportunities...[/bold]")
    console.print()

    merged = _get_merged_branches(head_branch)
    merged_names = {b["name"] for b in merged}
    squash_merged = _get_squash_merged_branches(head_branch, merged_names)
    stale_refs = _get_stale_remote_refs()

    if not merged and not squash_merged and not stale_refs:
        print_success("Nothing to clean up. Repository is tidy!")
        return

    # Display findings
    if merged:
        console.print("[bold]Merged branches (safe to delete):[/bold]")
        for b in merged:
            console.print(f"  {b['name']:<30} merged {b['date']}")
        console.print()

    if squash_merged:
        console.print("[bold]Likely squash-merged branches:[/bold]")
        for b in squash_merged:
            console.print(f"  {b['name']:<30} last commit {b['date']} (all patches found on {head_branch})")
        console.print()

    if stale_refs:
        console.print("[bold]Stale remote tracking refs:[/bold]")
        for ref in stale_refs:
            console.print(f"  {ref}")
        console.print()

    total_branches = len(merged) + len(squash_merged)
    n_merged, n_squash, n_stale = len(merged), len(squash_merged), len(stale_refs)
    console.print(f"Summary: {n_merged} merged, {n_squash} likely squash-merged, {n_stale} stale refs")
    console.print()

    if dry_run:
        console.print("[dry_run]DRY RUN -- no changes will be made[/dry_run]")
        console.print()
        console.print(f"Would delete: {total_branches} branches, {len(stale_refs)} stale refs")
        return

    # Confirm and delete
    if merged:
        if yes or confirm_action("Delete merged branches?"):
            for b in merged:
                try:
                    run_git(["branch", "-d", b["name"]])
                    print_success(f"Deleted {b['name']}")
                except GitError as e:
                    print_error(f"Failed to delete {b['name']}: {e}")

    if squash_merged:
        if yes or confirm_action("Delete likely squash-merged branches?"):
            for b in squash_merged:
                try:
                    run_git(["branch", "-D", b["name"]])
                    print_success(f"Deleted {b['name']}")
                except GitError as e:
                    print_error(f"Failed to delete {b['name']}: {e}")

    if stale_refs:
        if yes or confirm_action("Prune stale remote tracking refs?"):
            try:
                run_git(["remote", "prune", "origin"])
                print_success(f"Pruned {len(stale_refs)} stale remote tracking refs")
            except GitError as e:
                print_error(f"Failed to prune: {e}")

    # Clean up stack config for deleted branches
    try:
        from gx.utils.stack import clean_deleted_branches
        clean_deleted_branches()
    except ImportError:
        pass

    console.print()
    print_success("Cleanup complete.")
