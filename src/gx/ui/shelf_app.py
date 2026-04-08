"""Textual TUI for gx shelf — interactive stash browser."""

from __future__ import annotations

from dataclasses import dataclass

from textual import work
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal
from textual.widgets import Footer, Header, ListItem, ListView, RichLog, Static

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
    """

    BINDINGS = [
        Binding("up", "cursor_up", "Up"),
        Binding("down", "cursor_down", "Down"),
        Binding("enter", "pop_stash", "Pop"),
        Binding("space", "apply_stash", "Apply (keep)"),
        Binding("d", "drop_stash", "Drop"),
        Binding("tab", "focus_diff", "Focus diff"),
        Binding("escape", "quit_app", "Exit"),
    ]

    _diff_cache: dict[str, str] = {}
    _stashes: list[StashEntry] = []
    _highlight: int = 0

    def compose(self) -> ComposeResult:
        yield Header(show_clock=False)
        with Horizontal(id="main-area"):
            yield ListView(id="stash-list")
            yield RichLog(id="diff-preview", highlight=False, markup=True)
        yield Footer()

    def on_mount(self) -> None:
        self._stashes = get_stash_list()
        self._populate_list()
        if self._stashes:
            self._load_diff_for_current()

    def _populate_list(self) -> None:
        """Populate the list once on mount."""
        lv = self.query_one("#stash-list", ListView)
        if not self._stashes:
            lv.append(ListItem(Static("No stashes."), id="empty-item"))
            return
        for i, entry in enumerate(self._stashes):
            msg = entry.message
            if len(msg) > 50:
                msg = msg[:47] + "..."
            text = f"{entry.stash_id}  {entry.relative_time}\n{msg}"
            lv.append(ListItem(Static(text), id=f"s-{i}"))

    def on_list_view_highlighted(self, event: ListView.Highlighted) -> None:
        if event.item is None:
            return
        lv = self.query_one("#stash-list", ListView)
        idx = lv.index
        if idx is not None and 0 <= idx < len(self._stashes):
            self._highlight = idx
            self._load_diff_for_current()

    @work(thread=True)
    def _load_diff_for_current(self) -> None:
        if not self._stashes:
            return
        idx = min(self._highlight, len(self._stashes) - 1)
        stash_id = self._stashes[idx].stash_id
        if stash_id in self._diff_cache:
            diff = self._diff_cache[stash_id]
        else:
            try:
                diff = run_git(["stash", "show", "-p", stash_id])
            except GitError:
                diff = "(failed to load diff)"
            if len(self._diff_cache) >= SHELF_DIFF_CACHE_MAX:
                oldest = next(iter(self._diff_cache))
                del self._diff_cache[oldest]
            self._diff_cache[stash_id] = diff

        self.call_from_thread(self._show_diff, diff)

    def _show_diff(self, diff: str) -> None:
        preview = self.query_one("#diff-preview", RichLog)
        preview.clear()
        preview.write(_format_diff(diff))

    # -- Actions --

    def action_cursor_up(self) -> None:
        self.query_one("#stash-list", ListView).action_cursor_up()

    def action_cursor_down(self) -> None:
        self.query_one("#stash-list", ListView).action_cursor_down()

    def action_pop_stash(self) -> None:
        if not self._stashes:
            return
        entry = self._stashes[self._highlight]
        self.exit(f"pop:{entry.stash_id}")

    def action_apply_stash(self) -> None:
        if not self._stashes:
            return
        entry = self._stashes[self._highlight]
        self.exit(f"apply:{entry.stash_id}")

    def action_drop_stash(self) -> None:
        if not self._stashes:
            return
        entry = self._stashes[self._highlight]
        self.exit(f"drop:{entry.stash_id}")

    def action_focus_diff(self) -> None:
        self.query_one("#diff-preview", RichLog).focus()

    def action_quit_app(self) -> None:
        self.exit(None)


def launch_shelf_browser() -> str | None:
    """Launch the TUI and return the action string or None."""
    app = StashBrowser()
    result = app.run()
    return str(result) if result is not None else None
