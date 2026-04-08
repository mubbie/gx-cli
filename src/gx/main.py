"""gx — Git Productivity Toolkit. Typer app entry point."""

from __future__ import annotations

import typer

from gx import __version__
from gx.commands import (
    conflicts,
    context,
    drift,
    graph,
    nuke,
    oops,
    recap,
    retarget,
    stack_cmd,
    sweep,
    switch,
    sync,
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

# Everyday
app.command(help="Smart undo \u2014 detects the last git action and reverses it.")(undo.undo)
app.command(name="redo", help="Redo the last undo.")(undo.redo)
app.command()(oops.oops)
app.command()(switch.switch)
app.command()(context.context)
app.command(name="ctx", hidden=True)(context.context)
app.command()(sweep.sweep)

# Insight
app.command()(who.who)
app.command()(recap.recap)
app.command()(drift.drift)
app.command()(conflicts.conflicts)

# Stacking
app.command(name="stack")(stack_cmd.stack)
app.command()(sync.sync)
app.command()(retarget.retarget)
app.command()(graph.graph)

# Utility
app.command()(nuke.nuke)
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
