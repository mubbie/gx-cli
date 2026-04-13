"""Tests for gx up / gx down."""

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
    repo = tmp_path / "nav-repo"
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
        "branches": {
            "feature/a": {"parent": "main", "parent_head": "abc"},
            "feature/b": {"parent": "feature/a", "parent_head": "def"},
        },
        "metadata": {"main_branch": "main"},
    }
    (gx_dir / "stack.json").write_text(json.dumps(config))

    git("checkout", "feature/a")
    return repo


def test_up(stacked_repo):
    os.chdir(stacked_repo)
    result = runner.invoke(app, ["up"])
    assert result.exit_code == 0
    assert "Moved up" in result.output
    assert "feature/b" in result.output


def test_up_at_top(stacked_repo):
    os.chdir(stacked_repo)
    run_git(["checkout", "feature/b"], cwd=stacked_repo)
    result = runner.invoke(app, ["up"])
    assert result.exit_code == 0
    assert "top of the stack" in result.output


def test_down(stacked_repo):
    os.chdir(stacked_repo)
    result = runner.invoke(app, ["down"])
    assert result.exit_code == 0
    assert "main" in result.output


def test_down_to_trunk(stacked_repo):
    os.chdir(stacked_repo)
    result = runner.invoke(app, ["down"])
    assert result.exit_code == 0
    assert "trunk" in result.output


def test_down_not_in_stack(stacked_repo):
    os.chdir(stacked_repo)
    run_git(["checkout", "main"], cwd=stacked_repo)
    result = runner.invoke(app, ["down"])
    assert result.exit_code == 0
    assert "not in the stack" in result.output


def test_up_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["up"])
    assert result.exit_code != 0


def test_down_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["down"])
    assert result.exit_code != 0
