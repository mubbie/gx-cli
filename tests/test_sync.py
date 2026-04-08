"""Tests for gx sync."""

from __future__ import annotations

import json
import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


@pytest.fixture
def stacked_repo(tmp_path):
    """Creates a repo with: main -> feature/a -> feature/b with stack config."""
    repo = tmp_path / "sync-repo"
    repo.mkdir()

    def git(*args):
        return run_git(list(args), cwd=repo)

    git("init", "-b", "main")
    git("config", "user.name", "Test User")
    git("config", "user.email", "test@example.com")

    (repo / "README.md").write_text("# Test\n")
    git("add", "README.md")
    git("commit", "-m", "Initial commit")

    git("checkout", "-b", "feature/a")
    (repo / "a.py").write_text("# A\n")
    git("add", "a.py")
    git("commit", "-m", "Add A")

    git("checkout", "-b", "feature/b")
    (repo / "b.py").write_text("# B\n")
    git("add", "b.py")
    git("commit", "-m", "Add B")

    gx_dir = repo / ".git" / "gx"
    gx_dir.mkdir(parents=True, exist_ok=True)
    config = {
        "relationships": {
            "feature/a": "main",
            "feature/b": "feature/a",
        },
        "metadata": {"main_branch": "main"},
    }
    (gx_dir / "stack.json").write_text(json.dumps(config))

    git("checkout", "feature/b")
    return repo


def test_sync_dry_run(stacked_repo):
    os.chdir(stacked_repo)
    result = runner.invoke(app, ["sync", "--stack", "--dry-run"])
    assert result.exit_code == 0
    assert "DRY RUN" in result.output


def test_sync_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["sync", "--stack"])
    assert result.exit_code != 0


def test_sync_no_stack(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["sync", "--stack"])
    assert result.exit_code == 0
    # Should say need at least 2 branches or nothing to sync
    assert "Need at least" in result.output or "No stack" in result.output
