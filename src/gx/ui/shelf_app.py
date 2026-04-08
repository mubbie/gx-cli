"""Textual TUI for gx shelf — interactive stash browser."""

from __future__ import annotations

from dataclasses import dataclass

from textual import work
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal
from textual.widgets import Footer, Header, Input, ListItem, ListView, RichLog, Static

from gx.utils.config import SHELF_DIFF_CACHE_MAX
from gx.utils.git import GitError, run_git


@dataclass
class StashEntry:
    index: int
    stash_id: str
    hash: str
    relative_time: str
    message: str
    branch: str


def get_stash_list() -> list[StashEntry]:
    """Parse git stash list into StashEntry objects."""
    try:
        output = run_git(["stash", "list", "--format=%gd|%H|%ar|%s"])
    except GitError:
        return []
    if not output:
        return []

    entries: list[StashEntry] = []
    for line in output.strip().splitlines():
        parts = line.split("|", 3)
        if len(parts) < 4:
            continue
        stash_id = parts[0]
        try:
            index = int(stash_id.split("{")[1].rstrip("}"))
        except (ValueError, IndexError):
            index = len(entries)
        message = parts[3]
        branch = _parse_branch(message)
        entries.append(StashEntry(
            index=index,
            stash_id=stash_id,
            hash=parts[1],
            relative_time=parts[2],
            message=message,
            branch=branch,
        ))
    return entries


def _parse_branch(message: str) -> str:
    """Extract branch name from stash message like 'On feature/auth: WIP'."""
    if message.startswith("On "):
        colon = message.find(":")
        if colon > 3:
            return message[3:colon]
    if message.startswith("WIP on "):
        colon = message.find(":")
        if colon > 7:
            return message[7:colon]
    return ""


def _format_diff(diff: str) -> str:
    """Add Rich markup to diff output for syntax highlighting."""
    lines = []
    for line in diff.splitlines():
        if line.startswith("+++") or line.startswith("---"):
            lines.append(f"[bold]{_escape(line)}[/bold]")
        elif line.startswith("+"):
            lines.append(f"[green]{_escape(line)}[/green]")
        elif line.startswith("-"):
            lines.append(f"[red]{_escape(line)}[/red]")
        elif line.startswith("@@"):
            lines.append(f"[cyan]{_escape(line)}[/cyan]")
        elif line.startswith("diff "):
            lines.append(f"[bold cyan]{_escape(line)}[/bold cyan]")
        else:
            lines.append(_escape(line))
    return "\n".join(lines)


def _escape(text: str) -> str:
    """Escape Rich markup characters in diff text."""
    return text.replace("[", "\\[")


class StashItem(Static):
    """A single stash entry in the list."""

    def __init__(self, entry: StashEntry, **kwargs) -> None:  # type: ignore[override]
        self.entry = entry
        time_str = entry.relative_time
        msg = entry.message
        if len(msg) > 50:
            msg = msg[:47] + "..."
        text = f"{entry.stash_id}  {time_str}\n{msg}"
        super().__init__(text, **kwargs)


class StashBrowser(App):  # type: ignore[type-arg]
    """Full-screen stash browser with split-pane layout."""

    CSS = """
    #main-area {
        height: 1fr;
    }
    #stash-list {
        width: 40%;
        border-right: solid $primary;
        overflow-y: auto;
    }
    #diff-preview {
        width: 60%;
    }
    .stash-item {
        padding: 0 1;
        height: 3;
    }
    .stash-item:hover {
        background: $accent;
    }
    .stash-item.--highlight {
        background: $accent;
    }
    #search-bar {
        display: none;
        dock: bottom;
        height: 1;
    }
    #search-bar.visible {
        display: block;
    }
    #empty-state {
        width: 100%;
        height: 100%;
        content-align: center middle;
        color: $text-muted;
    }
    """

    BINDINGS = [
        Binding("up", "cursor_up", "Up"),
        Binding("down", "cursor_down", "Down"),
        Binding("enter", "pop_stash", "Pop"),
        Binding("space", "apply_stash", "Apply (keep)"),
        Binding("d", "drop_stash", "Drop"),
        Binding("slash", "search", "Search"),
        Binding("tab", "focus_diff", "Focus diff"),
        Binding("escape", "quit_app", "Exit"),
    ]

    _diff_cache: dict[str, str] = {}
    _stashes: list[StashEntry] = []
    _filtered: list[StashEntry] = []
    _highlight: int = 0
    _confirming_drop: bool = False
    _search_active: bool = False
    _result_action: str | None = None

    def compose(self) -> ComposeResult:
        yield Header(show_clock=False)
        with Horizontal(id="main-area"):
            yield ListView(id="stash-list")
            yield RichLog(id="diff-preview", highlight=False, markup=True)
        yield Input(placeholder="Filter stashes...", id="search-bar")
        yield Footer()

    def on_mount(self) -> None:
        self._stashes = get_stash_list()
        self._filtered = list(self._stashes)
        self._render_list()
        if self._filtered:
            self._load_diff_for_current()

    def _render_list(self) -> None:
        lv = self.query_one("#stash-list", ListView)
        lv.clear()
        if not self._filtered:
            lv.append(ListItem(Static("No stashes."), id="empty-item"))
            return
        for i, entry in enumerate(self._filtered):
            item = ListItem(StashItem(entry), id=f"s-{i}")
            lv.append(item)
        self._highlight = min(self._highlight, len(self._filtered) - 1)
        if self._filtered:
            lv.index = self._highlight

    def on_list_view_highlighted(self, event: ListView.Highlighted) -> None:
        if event.item is None:
            return
        lv = self.query_one("#stash-list", ListView)
        idx = lv.index
        if idx is not None and 0 <= idx < len(self._filtered):
            self._highlight = idx
            self._load_diff_for_current()

    @work(thread=True)
    def _load_diff_for_current(self) -> None:
        if not self._filtered:
            return
        idx = min(self._highlight, len(self._filtered) - 1)
        stash_id = self._filtered[idx].stash_id
        if stash_id in self._diff_cache:
            diff = self._diff_cache[stash_id]
        else:
            try:
                diff = run_git(["stash", "show", "-p", stash_id])
            except GitError:
                diff = "(failed to load diff)"
            # Evict oldest if cache full
            if len(self._diff_cache) >= SHELF_DIFF_CACHE_MAX:
                oldest = next(iter(self._diff_cache))
                del self._diff_cache[oldest]
            self._diff_cache[stash_id] = diff

        self.call_from_thread(self._show_diff, diff, stash_id)

    def _show_diff(self, diff: str, stash_id: str) -> None:
        preview = self.query_one("#diff-preview", RichLog)
        preview.clear()
        preview.write(_format_diff(diff))

    # -- Actions --

    def action_cursor_up(self) -> None:
        lv = self.query_one("#stash-list", ListView)
        lv.action_cursor_up()

    def action_cursor_down(self) -> None:
        lv = self.query_one("#stash-list", ListView)
        lv.action_cursor_down()

    def action_pop_stash(self) -> None:
        if not self._filtered:
            return
        entry = self._filtered[self._highlight]
        self._result_action = f"pop:{entry.stash_id}"
        self.exit(self._result_action)

    def action_apply_stash(self) -> None:
        if not self._filtered:
            return
        entry = self._filtered[self._highlight]
        self._result_action = f"apply:{entry.stash_id}"
        self.exit(self._result_action)

    def action_drop_stash(self) -> None:
        if not self._filtered:
            return
        entry = self._filtered[self._highlight]
        self._result_action = f"drop:{entry.stash_id}"
        self.exit(self._result_action)

    def action_search(self) -> None:
        search_bar = self.query_one("#search-bar", Input)
        search_bar.add_class("visible")
        search_bar.focus()
        self._search_active = True

    def on_input_submitted(self, event: Input.Submitted) -> None:
        query = event.value.strip().lower()
        search_bar = self.query_one("#search-bar", Input)
        search_bar.remove_class("visible")
        self._search_active = False

        if query:
            self._filtered = [
                s for s in self._stashes
                if query in s.message.lower() or query in s.branch.lower()
            ]
        else:
            self._filtered = list(self._stashes)
        self._highlight = 0
        self._render_list()
        if self._filtered:
            self._load_diff_for_current()
        self.query_one("#stash-list", ListView).focus()

    def action_focus_diff(self) -> None:
        self.query_one("#diff-preview", RichLog).focus()

    def action_quit_app(self) -> None:
        if self._search_active:
            search_bar = self.query_one("#search-bar", Input)
            search_bar.remove_class("visible")
            search_bar.value = ""
            self._search_active = False
            self._filtered = list(self._stashes)
            self._render_list()
            self.query_one("#stash-list", ListView).focus()
        else:
            self.exit(None)


def launch_shelf_browser() -> str | None:
    """Launch the TUI and return the action string or None."""
    app = StashBrowser()
    result = app.run()
    return str(result) if result is not None else None
