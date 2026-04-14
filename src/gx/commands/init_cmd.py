"""gx init: Initialize gx stacking for this repo."""

from __future__ import annotations

from typing import Optional

import typer

from gx.utils.display import console, print_error, print_info, print_success, print_warning
from gx.utils.git import GitError, branch_exists, ensure_git_repo, run_git
from gx.utils.stack import _config_path, load_stack_config, save_stack_config


def init(
    trunk: Optional[str] = typer.Option(None, "--trunk", help="Explicitly set the trunk branch."),
    force: bool = typer.Option(False, "--force", help="Re-initialize (preserves relationships)."),
) -> None:
    """Initialize gx stacking for this repo."""
    try:
        ensure_git_repo()
    except GitError as e:
        print_error(str(e))
        raise typer.Exit(1)

    path = _config_path()
    already_initialized = path.exists()

    # Already initialized and no --force
    if already_initialized and not force:
        config = load_stack_config()
        current_trunk = config.get("metadata", {}).get("main_branch", "unknown")
        branch_count = len(config.get("branches", {}))
        console.print()
        print_info("gx is already initialized.")
        console.print(f"  Trunk: {current_trunk}")
        console.print(f"  Tracked branches: {branch_count}")
        console.print("  Config: .git/gx/stack.json")
        console.print()
        console.print("  Run with --force to re-initialize.")
        return

    # Validate --trunk if provided
    if trunk:
        if not branch_exists(trunk):
            print_error(f"Branch '{trunk}' does not exist.")
            raise typer.Exit(1)
    else:
        trunk = _detect_trunk()
        if not trunk:
            trunk = "main"
            print_warning("Could not auto-detect trunk branch. Defaulting to 'main'.")

    if force and already_initialized:
        print_warning("Re-initializing gx. Existing branch relationships will be preserved.")
        config = load_stack_config()
        config.setdefault("metadata", {})
        config["metadata"]["main_branch"] = trunk
        save_stack_config(config)
        console.print()
        print_success(f"Re-initialized gx with trunk branch: {trunk}")
    else:
        config = {
            "branches": {},
            "metadata": {"main_branch": trunk},
        }
        save_stack_config(config)
        console.print()
        print_success("Initialized gx in this repo.")
        console.print(f"  Trunk: {trunk}")
        console.print("  Stack config: .git/gx/stack.json")
        console.print()
        console.print("  Get started:")
        console.print(f"    gx stack feature/my-thing {trunk}    Create your first stacked branch")
        console.print("    gx graph                          View your stack")


def _detect_trunk() -> str | None:
    """Detect trunk branch. Returns None if unable."""
    try:
        output = run_git(["remote", "show", "origin"], check=False)
        for line in output.splitlines():
            if "HEAD branch:" in line:
                name = line.split(":")[-1].strip()
                if name and branch_exists(name):
                    return name
    except GitError:
        pass

    for name in ("main", "master", "develop"):
        if branch_exists(name):
            return name

    return None
