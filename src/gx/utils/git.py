"""Shared git helpers used across gx commands."""

from __future__ import annotations

import subprocess
from datetime import datetime, timezone


class GitError(Exception):
    """Raised when a git command fails."""


def run_git(args: list[str], cwd: str | None = None, check: bool = True) -> str:
    """Run a git command and return stdout. Raises GitError on failure if check=True."""
    try:
        result = subprocess.run(
            ["git"] + args,
            capture_output=True,
            text=True,
            cwd=cwd,
            timeout=30,
        )
        if check and result.returncode != 0:
            stderr = result.stderr.strip()
            raise GitError(stderr or f"git {' '.join(args)} failed with code {result.returncode}")
        return result.stdout.strip()
    except FileNotFoundError:
        raise GitError("git is not installed or not in PATH")
    except subprocess.TimeoutExpired:
        raise GitError(f"git {' '.join(args)} timed out after 30 seconds")


def ensure_git_repo(cwd: str | None = None) -> None:
    """Raise GitError if not inside a git repository."""
    try:
        run_git(["rev-parse", "--is-inside-work-tree"], cwd=cwd)
    except GitError:
        raise GitError("Not a git repository. Run this from inside a git project.")


def get_current_branch(cwd: str | None = None) -> str:
    """Return the current branch name. Raises GitError if in detached HEAD."""
    branch = run_git(["symbolic-ref", "--short", "HEAD"], cwd=cwd, check=False)
    if not branch:
        raise GitError("HEAD is detached. Not on any branch.")
    return branch


def get_head_branch(cwd: str | None = None) -> str:
    """Detect the repo's HEAD branch (main, master, develop, etc.)."""
    # Strategy 1: check remote HEAD
    try:
        output = run_git(["remote", "show", "origin"], cwd=cwd, check=False)
        for line in output.splitlines():
            if "HEAD branch:" in line:
                return line.split("HEAD branch:")[-1].strip()
    except GitError:
        pass

    # Strategy 2: check common branch names
    for name in ("main", "master", "develop"):
        if branch_exists(name, cwd=cwd):
            return name

    # Strategy 3: git config
    try:
        default = run_git(["config", "--get", "init.defaultBranch"], cwd=cwd, check=False)
        if default and branch_exists(default, cwd=cwd):
            return default
    except GitError:
        pass

    return "main"


def get_repo_root(cwd: str | None = None) -> str:
    return run_git(["rev-parse", "--show-toplevel"], cwd=cwd)


def is_clean_working_tree(cwd: str | None = None) -> bool:
    output = run_git(["status", "--porcelain"], cwd=cwd)
    return len(output) == 0


def get_last_commit(cwd: str | None = None) -> dict:
    fmt = "%H%n%h%n%s%n%an%n%aI"
    try:
        output = run_git(["log", "-1", f"--format={fmt}"], cwd=cwd)
    except GitError:
        return {}
    lines = output.splitlines()
    if len(lines) < 5:
        return {}
    return {
        "hash": lines[0],
        "short_hash": lines[1],
        "message": lines[2],
        "author": lines[3],
        "date": lines[4],
    }


def get_stash_count(cwd: str | None = None) -> int:
    output = run_git(["stash", "list"], cwd=cwd, check=False)
    if not output:
        return 0
    return len(output.splitlines())


def branch_exists(name: str, remote: bool = False, cwd: str | None = None) -> bool:
    if remote:
        result = run_git(["branch", "-r", "--list", f"origin/{name}"], cwd=cwd, check=False)
    else:
        result = run_git(["branch", "--list", name], cwd=cwd, check=False)
    return len(result.strip()) > 0


def get_merge_base(branch_a: str, branch_b: str, cwd: str | None = None) -> str:
    return run_git(["merge-base", branch_a, branch_b], cwd=cwd)


def get_reflog_entries(n: int = 20, cwd: str | None = None) -> list[dict]:
    fmt = "%H%n%gd%n%gs%n%gD%n%ci"
    try:
        output = run_git(["reflog", f"--format={fmt}", "-n", str(n)], cwd=cwd)
    except GitError:
        return []
    entries = []
    lines = output.splitlines()
    i = 0
    while i + 4 < len(lines):
        entries.append({
            "hash": lines[i],
            "ref": lines[i + 1],
            "action": lines[i + 2],
            "description": lines[i + 3],
            "timestamp": lines[i + 4],
        })
        i += 5
    return entries


def is_commit_pushed(ref: str = "HEAD", cwd: str | None = None) -> bool:
    """Check if a commit exists on the remote tracking branch."""
    try:
        branch = get_current_branch(cwd=cwd)
        tracking = run_git(
            ["rev-parse", "--abbrev-ref", f"{branch}@{{upstream}}"],
            cwd=cwd,
            check=False,
        )
        if not tracking:
            return False
        result = run_git(
            ["branch", "-r", "--contains", ref],
            cwd=cwd,
            check=False,
        )
        return len(result.strip()) > 0
    except GitError:
        return False


def time_ago(date_str: str) -> str:
    """Convert an ISO date string to a human-readable relative time."""
    try:
        dt = datetime.fromisoformat(date_str.replace("Z", "+00:00"))
        now = datetime.now(timezone.utc)
        diff = now - dt
        seconds = int(diff.total_seconds())
        if seconds < 60:
            return "just now"
        minutes = seconds // 60
        if minutes < 60:
            return f"{minutes} min ago"
        hours = minutes // 60
        if hours < 24:
            return f"{hours} hour{'s' if hours != 1 else ''} ago"
        days = hours // 24
        if days < 30:
            return f"{days} day{'s' if days != 1 else ''} ago"
        months = days // 30
        return f"{months} month{'s' if months != 1 else ''} ago"
    except (ValueError, TypeError):
        return date_str
