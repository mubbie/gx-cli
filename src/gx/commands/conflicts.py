"""gx conflicts — Preview merge conflicts before actually merging."""

from __future__ import annotations

import re

import typer

from gx.utils.display import console, print_error, print_info, print_success
from gx.utils.git import (
    GitError,
    ensure_git_repo,
    get_current_branch,
    get_head_branch,
    get_merge_base,
    run_git,
)


def _parse_merge_tree_conflicts(output: str) -> list[dict]:
    """Parse git merge-tree output for conflict markers."""
    conflicts = []
    current_file = None

    # merge-tree output contains conflict sections
    # Look for lines indicating conflicted files
    for line in output.splitlines():
        # Check for file paths in conflict markers
        if line.startswith("changed in both"):
            # Extract filename from the context
            continue
        if line.startswith("  base"):
            continue
        if line.startswith("  our"):
            continue
        if line.startswith("  their"):
            continue

        # Look for +<<<<<<< .our pattern
        if "<<<<<<<" in line:
            continue
        if "=======" in line:
            continue
        if ">>>>>>>" in line:
            continue

    return conflicts


def _find_conflicts_merge_tree(target_branch: str) -> tuple[list[dict], int]:
    """Use git merge-tree to find conflicts without modifying working tree."""
    try:
        merge_base = get_merge_base("HEAD", target_branch)
    except GitError:
        raise GitError(f"No common ancestor between current branch and {target_branch}.")

    # git merge-tree (new form, Git 2.38+)
    try:
        output = run_git(["merge-tree", "--write-tree", "HEAD", target_branch], check=False)
        # If exit code 0, no conflicts
        # Parse output for conflict info
        return _parse_new_merge_tree(output, target_branch)
    except GitError:
        pass

    # Fallback: old-style merge-tree
    try:
        output = run_git(["merge-tree", merge_base, "HEAD", target_branch], check=False)
        return _parse_old_merge_tree(output, target_branch)
    except GitError:
        return [], 0


def _parse_new_merge_tree(output: str, target_branch: str) -> tuple[list[dict], int]:
    """Parse new-style merge-tree output (Git 2.38+)."""
    conflicts = []
    clean_files = 0
    in_conflicts = False

    for line in output.splitlines():
        if line.startswith("CONFLICT"):
            # e.g., "CONFLICT (content): Merge conflict in src/auth.ts"
            match = re.search(r"Merge conflict in (.+)", line)
            if match:
                filepath = match.group(1).strip()
                conflicts.append({
                    "file": filepath,
                    "lines": "",
                    "authors": "",
                })
            elif "CONFLICT" in line:
                # Other conflict types (rename, delete, etc.)
                conflicts.append({
                    "file": line.replace("CONFLICT ", "").strip(),
                    "lines": "",
                    "authors": "",
                })

    # Count clean files from diff
    try:
        merge_base = get_merge_base("HEAD", target_branch)
        stat = run_git(["diff", "--stat", f"{merge_base}..{target_branch}"], check=False)
        total_files = sum(1 for line in stat.splitlines() if "|" in line)
        clean_files = max(0, total_files - len(conflicts))
    except GitError:
        pass

    return conflicts, clean_files


def _parse_old_merge_tree(output: str, target_branch: str) -> tuple[list[dict], int]:
    """Parse old-style merge-tree output."""
    conflicts = []
    seen_files = set()

    # Old merge-tree shows conflicts with markers in the output
    current_file = None
    for line in output.splitlines():
        # Detect "changed in both" sections
        if line.startswith("changed in both"):
            continue
        # File paths often appear after mode lines
        if line.strip().startswith("--- a/") or line.strip().startswith("+++ b/"):
            filepath = line.strip()[6:]
            if filepath not in seen_files:
                current_file = filepath
        # Conflict markers
        if "<<<<<<<" in line and current_file and current_file not in seen_files:
            conflicts.append({
                "file": current_file,
                "lines": "",
                "authors": "",
            })
            seen_files.add(current_file)

    # If we couldn't parse well, try a different approach
    if not conflicts and output:
        # Look for any file paths in the output
        for line in output.splitlines():
            if line.startswith("+<<<<<<"):
                continue
            # Match file paths
            match = re.match(r"^[a-f0-9]+ [a-f0-9]+ [a-f0-9]+\t(.+)$", line)
            if match:
                filepath = match.group(1)
                if filepath not in seen_files:
                    conflicts.append({"file": filepath, "lines": "", "authors": ""})
                    seen_files.add(filepath)

    clean_files = 0
    try:
        merge_base = get_merge_base("HEAD", target_branch)
        stat = run_git(["diff", "--stat", f"{merge_base}..{target_branch}"], check=False)
        total_files = sum(1 for line in stat.splitlines() if "|" in line)
        clean_files = max(0, total_files - len(conflicts))
    except GitError:
        pass

    return conflicts, clean_files


def _get_conflict_authors(filepath: str, target_branch: str) -> str:
    """Determine who wrote the conflicting lines on each side."""
    try:
        our_author = run_git(["log", "-1", "--format=%an", "--", filepath], check=False)
    except GitError:
        our_author = ""

    try:
        their_author = run_git(
            ["log", "-1", "--format=%an", target_branch, "--", filepath],
            check=False,
        )
    except GitError:
        their_author = ""

    if our_author and their_author and our_author != their_author:
        # Check if current user
        try:
            current_user = run_git(["config", "user.name"])
            if our_author == current_user:
                our_author = "you"
            if their_author == current_user:
                their_author = "you"
        except GitError:
            pass
        return f"({our_author} + {their_author})"
    elif our_author:
        return f"({our_author})"
    return ""


def conflicts(
    target: str = typer.Argument(None, help="Branch to check conflicts against (default: HEAD branch)."),
    dry_run: bool = typer.Option(False, "--dry-run", help="Always read-only (supported for consistency)."),
) -> None:
    """Preview merge conflicts before merging \u2014 without touching the working tree."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    try:
        current = get_current_branch()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    target_branch = target or get_head_branch()

    if current == target_branch:
        print_info(f"You're already on {target_branch}. Switch to a feature branch to check conflicts.")
        raise typer.Exit(0)

    # Verify target exists
    try:
        run_git(["rev-parse", "--verify", target_branch])
    except GitError:
        print_error(f"Branch '{target_branch}' does not exist.")
        raise typer.Exit(1)

    console.print()
    console.print(f"Checking [bold]{current}[/bold] against [bold]{target_branch}[/bold]...")
    console.print()

    try:
        found_conflicts, clean_files = _find_conflicts_merge_tree(target_branch)
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    if not found_conflicts:
        print_success("No conflicts \u2014 clean merge")

        # Show what would be modified
        try:
            merge_base = get_merge_base("HEAD", target_branch)
            stat = run_git(["diff", "--stat", f"{merge_base}..{target_branch}"], check=False)
            if stat:
                lines = stat.splitlines()
                if lines:
                    summary = lines[-1].strip()
                    console.print(f"  {summary}")
        except GitError:
            pass
        return

    # Enrich conflicts with author info
    for conflict in found_conflicts:
        conflict["authors"] = _get_conflict_authors(conflict["file"], target_branch)

    console.print(f"[red]\u2717 {len(found_conflicts)} conflict{'s' if len(found_conflicts) != 1 else ''} found[/red]")
    console.print()

    for conflict in found_conflicts:
        line_info = f"     {conflict['lines']}" if conflict["lines"] else ""
        author_info = f"     {conflict['authors']}" if conflict["authors"] else ""
        console.print(f"  {conflict['file']}{line_info}{author_info}")

    if clean_files > 0:
        console.print()
        console.print(f"  {clean_files} other file{'s' if clean_files != 1 else ''} merge cleanly")
