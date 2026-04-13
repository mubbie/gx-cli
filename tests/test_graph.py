"""Tests for gx graph."""

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
    repo = tmp_path / "stacked-repo"
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

    gx_dir = repo / ".git" / "gx"
    gx_dir.mkdir(parents=True, exist_ok=True)
    config = {
        "branches": {"feature/a": {"parent": "main", "parent_head": "abc"}},
        "metadata": {"main_branch": "main"},
    }
    (gx_dir / "stack.json").write_text(json.dumps(config))

    git("checkout", "main")
    return repo


def test_graph_basic(stacked_repo):
    os.chdir(stacked_repo)
    result = runner.invoke(app, ["graph"])
    assert result.exit_code == 0
    assert "main" in result.output
    assert "feature/a" in result.output


def test_graph_empty_repo(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["graph"])
    assert result.exit_code == 0
    assert "main" in result.output


def test_graph_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["graph"])
    assert result.exit_code != 0
