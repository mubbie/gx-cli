"""gx — Git Productivity Toolkit. Typer app entry point."""

from __future__ import annotations

import typer

from gx import __version__
from gx.commands import (
    conflicts,
    context,
    drift,
    nuke,
    oops,
    recap,
    sweep,
    switch,
    undo,
    update,
    who,
)

app = typer.Typer(
    name="gx",
    help="gx \u2014 Git Productivity Toolkit",
    add_completion=False,
    no_args_is_help=True,
    rich_markup_mode="rich",
)

# Register commands
app.command()(undo.undo)
app.command(name="redo")(undo.redo)
app.command()(who.who)
app.command()(nuke.nuke)
app.command()(recap.recap)
app.command()(sweep.sweep)
app.command()(oops.oops)
app.command()(context.context)
app.command(name="ctx")(context.context)
app.command()(drift.drift)
app.command()(switch.switch)
app.command()(conflicts.conflicts)
app.command()(update.update)


def version_callback(value: bool) -> None:
    if value:
        typer.echo(f"gx {__version__}")
        raise typer.Exit()


@app.callback()
def main(
    version: bool = typer.Option(
        False, "--version", "-V", help="Show version and exit.", callback=version_callback, is_eager=True
    ),
) -> None:
    """gx \u2014 Git Productivity Toolkit"""
