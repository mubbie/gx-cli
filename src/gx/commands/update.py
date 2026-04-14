"""gx update: Self-update gx to the latest version."""

from __future__ import annotations

import os
import shutil
import subprocess
import sys

import typer

from gx import __version__
from gx.utils.display import console, print_error, print_success


def _detect_install_method() -> str:
    """Detect how gx was installed: homebrew, pipx, or pip."""
    gx_path = shutil.which("gx")
    if not gx_path:
        return "unknown"

    resolved = os.path.realpath(gx_path)

    if "Cellar" in resolved or "homebrew" in resolved.lower() or "linuxbrew" in resolved.lower():
        return "homebrew"

    if "pipx" in resolved:
        return "pipx"

    return "pip"


def _get_latest_version() -> str | None:
    """Query PyPI for the latest version of gx-git."""
    try:
        result = subprocess.run(
            [sys.executable, "-m", "pip", "index", "versions", "gx-git"],
            capture_output=True,
            text=True,
            timeout=15,
        )
        if result.returncode == 0 and "(" in result.stdout:
            return result.stdout.split("(")[1].split(")")[0].strip()
    except (subprocess.TimeoutExpired, FileNotFoundError):
        pass

    try:
        result = subprocess.run(
            [sys.executable, "-m", "pip", "install", "gx-git==999.0.0"],
            capture_output=True,
            text=True,
            timeout=15,
        )
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

    method = _detect_install_method()

    if method == "homebrew":
        console.print("Installed via Homebrew. Updating...")
        console.print()
        try:
            result = subprocess.run(["brew", "upgrade", "gx-git"], timeout=120)
            if result.returncode == 0:
                print_success("Updated via Homebrew. Run `gx --version` to verify.")
            else:
                print_error("Update failed. Try: brew upgrade gx-git")
                raise typer.Exit(1)
        except subprocess.TimeoutExpired:
            print_error("Update timed out. Try: brew upgrade gx-git")
            raise typer.Exit(1)
        except FileNotFoundError:
            print_error("brew not found. Try: brew upgrade gx-git")
            raise typer.Exit(1)
        return

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

    if method == "pipx":
        pipx = shutil.which("pipx")
        if pipx:
            cmd = [pipx, "upgrade", "gx-git"]
        else:
            cmd = [sys.executable, "-m", "pip", "install", "--upgrade", "gx-git"]
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
