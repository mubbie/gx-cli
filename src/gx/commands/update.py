"""gx update — Self-update gx to the latest version."""

from __future__ import annotations

import shutil
import subprocess
import sys

import typer

from gx import __version__
from gx.utils.display import console, print_error, print_success


def _get_latest_version() -> str | None:
    """Query PyPI for the latest version of gx-git."""
    try:
        result = subprocess.run(
            [sys.executable, "-m", "pip", "index", "versions", "gx-git"],
            capture_output=True,
            text=True,
            timeout=15,
        )
        # Output: "gx-git (0.2.0)"
        if result.returncode == 0 and "(" in result.stdout:
            return result.stdout.split("(")[1].split(")")[0].strip()
    except (subprocess.TimeoutExpired, FileNotFoundError):
        pass

    # Fallback: pip install --dry-run
    try:
        result = subprocess.run(
            [sys.executable, "-m", "pip", "install", "gx-git==999.0.0"],
            capture_output=True,
            text=True,
            timeout=15,
        )
        # Error message lists available versions
        stderr = result.stderr
        if "from versions:" in stderr:
            versions_str = stderr.split("from versions:")[1].split(")")[0].strip()
            versions = [v.strip() for v in versions_str.split(",") if v.strip()]
            if versions:
                return versions[-1]
    except (subprocess.TimeoutExpired, FileNotFoundError):
        pass

    return None


def update() -> None:
    """Update gx to the latest version."""
    console.print(f"Current version: {__version__}")
    console.print("Checking for updates...")

    latest = _get_latest_version()
    if latest is None:
        print_error("Could not check for updates. Check your internet connection.")
        raise typer.Exit(1)

    if latest == __version__:
        print_success(f"Already up to date (v{__version__}).")
        return

    console.print(f"New version available: {latest}")
    console.print()

    # Detect installer: pipx or pip
    pipx = shutil.which("pipx")
    if pipx:
        cmd = [pipx, "upgrade", "gx-git"]
        console.print("Upgrading via pipx...")
    else:
        cmd = [sys.executable, "-m", "pip", "install", "--upgrade", "gx-git"]
        console.print("Upgrading via pip...")

    try:
        result = subprocess.run(cmd, timeout=60)
        if result.returncode == 0:
            print_success(f"Updated to v{latest}.")
        else:
            print_error("Update failed. Try manually: pipx upgrade gx-git")
            raise typer.Exit(1)
    except subprocess.TimeoutExpired:
        print_error("Update timed out.")
        raise typer.Exit(1)
    except FileNotFoundError:
        print_error("Could not find pip or pipx. Try manually: pip install --upgrade gx-git")
        raise typer.Exit(1)
