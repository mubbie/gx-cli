"""Tests for gx retarget."""

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
    repo = tmp_path / "retarget-repo"
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

    git("checkout", "main")
    return repo


def test_retarget_dry_run(stacked_repo):
    os.chdir(stacked_repo)
    result = runner.invoke(app, ["retarget", "feature/b", "main", "--dry-run"])
    assert result.exit_code == 0
    assert "DRY RUN" in result.output


def test_retarget_already_on_target(stacked_repo):
    os.chdir(stacked_repo)
    result = runner.invoke(app, ["retarget", "feature/a", "main"])
    assert result.exit_code == 0
    assert "already based" in result.output or "Nothing to do" in result.output


def test_retarget_nonexistent_branch(stacked_repo):
    os.chdir(stacked_repo)
    result = runner.invoke(app, ["retarget", "nonexistent", "main"])
    assert result.exit_code != 0


def test_retarget_nonexistent_target(stacked_repo):
    os.chdir(stacked_repo)
    result = runner.invoke(app, ["retarget", "feature/b", "nonexistent"])
    assert result.exit_code != 0


def test_retarget_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["retarget", "feature/b", "main"])
    assert result.exit_code != 0


def test_retarget_no_target(stacked_repo):
    os.chdir(stacked_repo)
    result = runner.invoke(app, ["retarget", "feature/b"])
    assert result.exit_code != 0
    assert "required" in result.output.lower() or "Missing" in result.output
