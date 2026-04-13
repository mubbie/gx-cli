"""Tests for gx handoff."""

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
def repo_with_commits(git_repo):
    """Repo with main + feature branch with commits."""
    repo = git_repo
    run_git(["checkout", "-b", "feature/work"], cwd=repo)
    (repo / "a.py").write_text("# new file\n")
    run_git(["add", "a.py"], cwd=repo)
    run_git(["commit", "-m", "Add a.py"], cwd=repo)
    (repo / "b.py").write_text("# another file\n")
    run_git(["add", "b.py"], cwd=repo)
    run_git(["commit", "-m", "Add b.py"], cwd=repo)
    return repo


@pytest.fixture
def stacked_repo(git_repo):
    """Repo with stack config."""
    repo = git_repo
    run_git(["checkout", "-b", "feature/stacked"], cwd=repo)
    (repo / "s.py").write_text("# stacked\n")
    run_git(["add", "s.py"], cwd=repo)
    run_git(["commit", "-m", "Add stacked file"], cwd=repo)

    gx_dir = repo / ".git" / "gx"
    gx_dir.mkdir(parents=True, exist_ok=True)
    config = {
        "branches": {
            "feature/stacked": {"parent": "main", "parent_head": "abc"},
        },
        "metadata": {"main_branch": "main"},
    }
    (gx_dir / "stack.json").write_text(json.dumps(config))
    return repo


def test_handoff_default(repo_with_commits):
    os.chdir(repo_with_commits)
    result = runner.invoke(app, ["handoff"])
    assert result.exit_code == 0
    assert "feature/work" in result.output
    assert "Commits" in result.output
    assert "a.py" in result.output
    assert "b.py" in result.output
    assert "Files:" in result.output


def test_handoff_against(repo_with_commits):
    os.chdir(repo_with_commits)
    result = runner.invoke(app, ["handoff", "--against", "main"])
    assert result.exit_code == 0
    assert "vs main" in result.output


def test_handoff_stacked_uses_parent(stacked_repo):
    os.chdir(stacked_repo)
    result = runner.invoke(app, ["handoff"])
    assert result.exit_code == 0
    assert "on main" in result.output


def test_handoff_markdown(repo_with_commits):
    os.chdir(repo_with_commits)
    result = runner.invoke(app, ["handoff", "--markdown"])
    assert result.exit_code == 0
    assert "## feature/work" in result.output
    assert "### Commits" in result.output
    assert "### Files Changed" in result.output
    assert "- `" in result.output


def test_handoff_no_changes(git_repo):
    os.chdir(git_repo)
    run_git(["checkout", "-b", "feature/empty"], cwd=git_repo)
    result = runner.invoke(app, ["handoff"])
    assert result.exit_code == 0
    assert "No changes" in result.output


def test_handoff_against_nonexistent(repo_with_commits):
    os.chdir(repo_with_commits)
    result = runner.invoke(app, ["handoff", "--against", "nonexistent"])
    assert result.exit_code != 0
    assert "does not exist" in result.output


def test_handoff_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["handoff"])
    assert result.exit_code != 0


def test_handoff_copy_flag(repo_with_commits):
    os.chdir(repo_with_commits)
    # Just verify the flag doesn't crash -- clipboard may not be available in CI
    result = runner.invoke(app, ["handoff", "--copy"])
    assert result.exit_code == 0
    assert "Commits" in result.output
