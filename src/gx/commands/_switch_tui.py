"""Textual TUI for gx switch — fuzzy branch picker."""

from __future__ import annotations

from textual import work
from textual.app import App, ComposeResult
from textual.containers import Vertical
from textual.reactive import reactive
from textual.widgets import Header, Input, Static

from gx.utils.git import GitError, run_git, time_ago


class BranchItem(Static):
    """A single branch row."""

    def __init__(self, branch: dict, **kwargs) -> None:
        self.branch = branch
        age = time_ago(branch["date"])
        ahead_behind = branch.get("ahead_behind", "")
        text = f"  {branch['name']:<35} {ahead_behind:<20} {age:<15} {branch['author']}"
        super().__init__(text, **kwargs)
        self.add_class("branch-item")

    def on_click(self) -> None:
        self.app.exit(self.branch["name"])


class SwitchApp(App):
    """Fuzzy branch picker TUI."""

    CSS = """
    Screen {
        background: $surface;
    }
    #branch-list {
        height: 1fr;
        overflow-y: auto;
    }
    .branch-item {
        height: 1;
        padding: 0 1;
    }
    .branch-item:hover {
        background: $accent;
    }
    .branch-item.highlighted {
        background: $accent;
        color: $text;
    }
    #search {
        dock: bottom;
        margin: 0 1;
    }
    #hint {
        dock: bottom;
        height: 1;
        color: $text-muted;
        text-align: center;
    }
    """

    BINDINGS = [
        ("escape", "quit", "Cancel"),
        ("up", "move_up", "Up"),
        ("down", "move_down", "Down"),
        ("enter", "select", "Select"),
    ]

    highlight_index: reactive[int] = reactive(0)

    def __init__(self, branches: list[dict], head_branch: str) -> None:
        super().__init__()
        self.all_branches = branches
        self.filtered_branches = list(branches)
        self.head_branch = head_branch

    def compose(self) -> ComposeResult:
        yield Header(show_clock=False)
        yield Vertical(id="branch-list")
        yield Input(placeholder="Type to filter branches...", id="search")
        yield Static("\u2191\u2193 Navigate  Enter Select  Esc Cancel", id="hint")

    def on_mount(self) -> None:
        self._render_branches()
        self._load_ahead_behind()
        self.query_one("#search", Input).focus()

    def _render_branches(self) -> None:
        container = self.query_one("#branch-list", Vertical)
        container.remove_children()
        for i, branch in enumerate(self.filtered_branches):
            item = BranchItem(branch, id=f"branch-{i}")
            if i == self.highlight_index:
                item.add_class("highlighted")
            container.mount(item)

    def on_input_changed(self, event: Input.Changed) -> None:
        query = event.value.lower()
        if query:
            self.filtered_branches = [
                b for b in self.all_branches
                if query in b["name"].lower()
            ]
        else:
            self.filtered_branches = list(self.all_branches)
        self.highlight_index = 0
        self._render_branches()

    def watch_highlight_index(self, old: int, new: int) -> None:
        # Update highlighting
        container = self.query_one("#branch-list", Vertical)
        children = list(container.children)
        if 0 <= old < len(children):
            children[old].remove_class("highlighted")
        if 0 <= new < len(children):
            children[new].add_class("highlighted")
            children[new].scroll_visible()

    def action_move_up(self) -> None:
        if self.highlight_index > 0:
            self.highlight_index -= 1

    def action_move_down(self) -> None:
        if self.highlight_index < len(self.filtered_branches) - 1:
            self.highlight_index += 1

    def action_select(self) -> None:
        if self.filtered_branches:
            idx = min(self.highlight_index, len(self.filtered_branches) - 1)
            self.exit(self.filtered_branches[idx]["name"])

    async def action_quit(self) -> None:  # type: ignore[override]
        self.exit(None)

    @work(thread=True)
    def _load_ahead_behind(self) -> None:
        """Load ahead/behind counts in background."""
        for branch in self.all_branches:
            try:
                ab = run_git([
                    "rev-list", "--left-right", "--count",
                    f"{branch['name']}...{self.head_branch}"
                ])
                ahead, behind = ab.split()
                branch["ahead_behind"] = f"{ahead} ahead, {behind} behind"
            except (GitError, ValueError):
                branch["ahead_behind"] = ""

        self.call_from_thread(self._render_branches)


def run_switch_tui(branches: list[dict], head_branch: str) -> str | None:
    """Run the Textual TUI and return the selected branch name."""
    app = SwitchApp(branches, head_branch)
    result = app.run()
    return result
