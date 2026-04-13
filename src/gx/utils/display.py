"""Shared Rich formatting helpers."""

from __future__ import annotations

import os
import sys

from rich.console import Console
from rich.table import Table
from rich.theme import Theme

_theme = Theme(
    {
        "success": "bold green",
        "error": "bold red",
        "warning": "bold yellow",
        "info": "bold blue",
        "dry_run": "bold yellow",
    }
)


def _should_use_color() -> bool:
    if os.environ.get("NO_COLOR") or os.environ.get("GX_NO_COLOR"):
        return False
    if not hasattr(sys.stdout, "isatty"):
        return False
    return sys.stdout.isatty()


console = Console(
    theme=_theme,
    no_color=not _should_use_color(),
    highlight=False,
)


def confirm_action(message: str) -> bool:
    """Show a yes/no confirmation prompt. Returns True if user confirms."""
    try:
        response = console.input(f"\n? {message} \\[y/N] ")
        return response.strip().lower() in ("y", "yes")
    except (EOFError, KeyboardInterrupt):
        console.print()
        return False


def print_success(message: str) -> None:
    console.print(f"[success]OK[/success] {message}")


def print_error(message: str) -> None:
    console.print(f"[error]ERROR[/error] {message}")


def print_warning(message: str) -> None:
    console.print(f"[warning]WARN[/warning] {message}")


def print_info(message: str) -> None:
    console.print(f"[info]>[/info] {message}")


def print_dry_run(actions: list[str]) -> None:
    console.print()
    console.print("[dry_run]DRY RUN. No changes will be made[/dry_run]")
    console.print()
    for action in actions:
        console.print(f"  {action}")
    console.print()


def print_table(headers: list[str], rows: list[list[str]], title: str | None = None) -> None:
    table = Table(title=title, show_header=True, header_style="bold", padding=(0, 2))
    for header in headers:
        table.add_column(header)
    for row in rows:
        table.add_row(*row)
    console.print(table)
