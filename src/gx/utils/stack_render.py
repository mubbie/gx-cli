"""Tree rendering for gx graph using Rich."""

from __future__ import annotations

from gx.utils.display import console
from gx.utils.stack import BranchNode, BranchStack


def render_branch_stack(stack: BranchStack) -> None:
    """Render the full branch stack tree."""
    if not stack.roots and not stack.orphans:
        console.print("No branches found.")
        return

    console.print()
    console.print("[bold]Branch Stack:[/bold]")
    console.print()

    for i, root in enumerate(stack.roots):
        is_last = i == len(stack.roots) - 1 and not stack.orphans
        _render_node(root, prefix="", is_last=is_last)

    if stack.orphans:
        console.print()
        console.print("[yellow bold]Orphaned Branches:[/yellow bold]")
        for i, orphan in enumerate(stack.orphans):
            _render_node(orphan, prefix="", is_last=(i == len(stack.orphans) - 1))

    console.print()
    console.print(
        "[dim]Legend: * current branch  "
        "+ merged  "
        "(+ahead/-behind)  "
        "! orphaned[/dim]"
    )
    console.print("[dim]Relationships stored in .git/gx/stack.json[/dim]")
    console.print()


def _render_node(node: BranchNode, prefix: str, is_last: bool) -> None:
    """Render a single node and its children recursively."""
    connector = "`-- " if is_last else "|-- "

    # Build status indicators
    indicators: list[str] = []
    if node.is_head:
        indicators.append("[green bold]* HEAD[/green bold]")
    if node.is_merged:
        indicators.append("[dim]+ merged[/dim]")
    elif node.is_orphan:
        indicators.append("[yellow]! orphaned[/yellow]")
    elif node.ahead > 0 or node.behind > 0:
        indicators.append(f"[cyan](+{node.ahead}/-{node.behind})[/cyan]")

    indicator_str = "  ".join(indicators)
    spacer = "  " if indicator_str else ""

    # Color the branch name
    if node.is_head:
        name_str = f"[green bold]{node.name}[/green bold]"
    elif node.is_merged:
        name_str = f"[dim]{node.name}[/dim]"
    elif node.is_orphan:
        name_str = f"[yellow]{node.name}[/yellow]"
    else:
        name_str = f"[blue]{node.name}[/blue]"

    console.print(f"{prefix}{connector}{name_str}{spacer}{indicator_str}")

    child_prefix = prefix + ("    " if is_last else "|   ")
    for i, child in enumerate(node.children):
        _render_node(child, child_prefix, is_last=(i == len(node.children) - 1))
