"""gx nuke -- Delete branches with confidence."""

from __future__ import annotations

import fnmatch

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
    get_head_branch,
    run_git,
    time_ago,
)


def _get_local_branches() -> list[str]:
    output = run_git(["branch", "--format=%(refname:short)"])
    return [b.strip() for b in output.splitlines() if b.strip()]


def _get_branch_info(branch: str) -> dict:
    """Get metadata about a branch."""
    info: dict = {"name": branch, "local": False, "remote": False, "merged": False}

    info["local"] = branch_exists(branch)
    info["remote"] = branch_exists(branch, remote=True)

    # Check if merged into HEAD branch
    try:
        head = get_head_branch()
        merged_output = run_git(["branch", "--merged", head], check=False)
        merged_branches = [b.strip().lstrip("* ") for b in merged_output.splitlines()]
        info["merged"] = branch in merged_branches
    except GitError:
        pass

    # Last commit info
    try:
        ref = branch if info["local"] else f"origin/{branch}"
        log_out = run_git(["log", "-1", "--format=%h %s %aI", ref], check=False)
        if log_out:
            parts = log_out.split(" ", 2)
            if len(parts) >= 3:
                # The date is at the end after the message
                # Format: hash message date
                tokens = log_out.rsplit(" ", 1)
                info["last_commit_msg"] = tokens[0].split(" ", 1)[-1] if len(tokens) > 1 else ""
                info["last_commit_date"] = tokens[-1] if len(tokens) > 1 else ""
    except GitError:
        pass

    # Count unmerged commits
    try:
        head = get_head_branch()
        ref = branch if info["local"] else f"origin/{branch}"
        count_output = run_git(["rev-list", "--count", f"{head}..{ref}"], check=False)
        info["unmerged_commits"] = int(count_output) if count_output else 0
    except (GitError, ValueError):
        info["unmerged_commits"] = 0

    return info


def _resolve_branches(pattern: str) -> list[str]:
    """Resolve a branch pattern (exact or glob) to branch names."""
    all_branches = _get_local_branches()

    # Check for exact match first
    if pattern in all_branches:
        return [pattern]

    # Try glob matching
    matches = fnmatch.filter(all_branches, pattern)

    # Also check remote branches if no local matches
    if not matches:
        try:
            remote_output = run_git(["branch", "-r", "--format=%(refname:short)"])
            remote_branches = [
                b.strip().replace("origin/", "", 1)
                for b in remote_output.splitlines()
                if b.strip() and "HEAD" not in b
            ]
            matches = fnmatch.filter(remote_branches, pattern)
        except GitError:
            pass

    return matches


def nuke(
    pattern: str = typer.Argument(..., help="Branch name or glob pattern to delete."),
    local: bool = typer.Option(False, "--local", help="Only delete local branch."),
    dry_run: bool = typer.Option(False, "--dry-run", help="Show what would be deleted."),
    yes: bool = typer.Option(False, "-y", "--yes", help="Skip confirmation."),
    orphans: bool = typer.Option(False, "--orphans", help="Delete all orphaned branches from the stack."),
) -> None:
    """Delete branches with confidence -- local, remote, and tracking refs."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    # Handle --orphans mode
    if orphans:
        _nuke_orphans(dry_run, yes)
        return

    branches = _resolve_branches(pattern)
    if not branches:
        print_error(f"No branches matching '{pattern}' found.")
        raise typer.Exit(1)

    try:
        current = get_current_branch()
    except GitError:
        current = ""

    head_branch = get_head_branch()

    # Safety checks
    for branch in branches:
        if branch == current:
            print_error(f"Cannot nuke '{branch}' -- it's the current branch. Switch to another branch first.")
            raise typer.Exit(1)
        if branch == head_branch:
            print_error(f"Cannot nuke '{head_branch}' -- it's the HEAD branch. This is blocked for safety.")
            raise typer.Exit(1)

    # Gather info
    infos = [_get_branch_info(b) for b in branches]

    if dry_run:
        actions = []
        if len(infos) > 1:
            actions.append(f"Matching branches ({len(infos)}):")
            actions.append("")
        for info in infos:
            name = info["name"]
            local_str = "yes" if info["local"] else "no"
            remote_str = "yes" if info["remote"] else "no"
            merged_str = "yes" if info["merged"] else "no"
            date = time_ago(info.get("last_commit_date", "")) if info.get("last_commit_date") else "unknown"
            actions.append(f"  {name}")
            actions.append(f"    Local: {local_str}  Remote: {remote_str}  Merged: {merged_str}  Last commit: {date}")

        total_local = sum(1 for i in infos if i["local"])
        total_remote = sum(1 for i in infos if i["remote"])
        actions.append("")
        actions.append(f"Would delete: {total_local} local, {total_remote} remote branches")
        print_dry_run(actions)
        return

    # Show what will be deleted
    for info in infos:
        console.print()
        console.print(f"[bold]Branch: {info['name']}[/bold]")
        console.print()

        if info["local"]:
            date = time_ago(info.get("last_commit_date", "")) if info.get("last_commit_date") else ""
            msg = info.get("last_commit_msg", "")
            detail = f'(last commit: {date}, "{msg}")' if date else ""
            console.print(f"  [red]\u2717[/red] Local branch          {detail}")
        if not local:
            if info["remote"]:
                console.print(f"  [red]\u2717[/red] Remote tracking ref   (origin/{info['name']})")
                console.print("  [red]\u2717[/red] Remote branch         (origin)")
            elif info.get("remote"):
                console.print("  - Remote              (not found)")

        if not info["merged"]:
            console.print()
            console.print(f"  This branch is [bold red]NOT[/bold red] merged into {head_branch}.")
            if info.get("unmerged_commits", 0) > 0:
                print_warning(f"You may lose {info['unmerged_commits']} commits.")

    has_unmerged = any(not i["merged"] for i in infos)

    if not yes or has_unmerged:
        if not confirm_action("Proceed with deletion?"):
            print_info("Cancelled.")
            raise typer.Exit(0)

    # Warn about stack children
    try:
        from gx.utils.stack import get_children
        for info in infos:
            children = get_children(info["name"])
            if children:
                child_list = ", ".join(children)
                print_warning(
                    f"{info['name']} has {len(children)} dependent branch(es) "
                    f"in your stack ({child_list}). They will become orphaned."
                )
    except ImportError:
        pass

    # Delete
    for info in infos:
        name = info["name"]
        if info["local"]:
            try:
                flag = "-d" if info["merged"] else "-D"
                run_git(["branch", flag, name])
                print_success(f"Deleted local branch {name}")
            except GitError as e:
                print_error(f"Failed to delete local branch {name}: {e}")

        if not local and info["remote"]:
            try:
                run_git(["branch", "-dr", f"origin/{name}"], check=False)
                print_success(f"Deleted remote tracking ref origin/{name}")
            except GitError:
                pass
            try:
                run_git(["push", "origin", "--delete", name])
                print_success(f"Deleted remote branch origin/{name}")
            except GitError as e:
                print_error(f"Failed to delete remote branch {name}: {e}")

        # Clean up stack config
        try:
            from gx.utils.stack import remove_branch
            remove_branch(name)
        except ImportError:
            pass


def _nuke_orphans(dry_run: bool, yes: bool) -> None:
    """Delete all orphaned branches from the stack."""
    from gx.utils.stack import build_branch_stack

    stack = build_branch_stack()
    if not stack.orphans:
        print_info("No orphaned branches found.")
        return

    console.print()
    console.print("[bold]Orphaned branches:[/bold]")
    for orphan in stack.orphans:
        console.print(f"  {orphan.name}")
    console.print()

    if dry_run:
        console.print(f"[yellow]DRY RUN[/yellow] -- would delete {len(stack.orphans)} orphaned branches.")
        return

    if not yes:
        if not confirm_action(f"Delete {len(stack.orphans)} orphaned branches?"):
            print_info("Cancelled.")
            return

    for orphan in stack.orphans:
        try:
            run_git(["branch", "-D", orphan.name])
            print_success(f"Deleted {orphan.name}")
            try:
                from gx.utils.stack import remove_branch
                remove_branch(orphan.name)
            except ImportError:
                pass
        except GitError as e:
            print_error(f"Failed to delete {orphan.name}: {e}")
