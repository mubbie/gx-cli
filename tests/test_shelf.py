"""Tests for gx shelf."""

from __future__ import annotations

import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


@pytest.fixture
def repo_with_stash(git_repo):
    """Create a repo with a stash entry."""
    repo = git_repo
    (repo / "temp.txt").write_text("stash me")
    run_git(["add", "temp.txt"], cwd=repo)
    run_git(["stash", "push", "-m", "Test stash"], cwd=repo)
    return repo


def test_shelf_push(git_repo):
    os.chdir(git_repo)
    (git_repo / "work.txt").write_text("in progress")
    run_git(["add", "work.txt"], cwd=git_repo)

    result = runner.invoke(app, ["shelf", "push", "My work in progress"])
    assert result.exit_code == 0
    assert "Stashed" in result.output

    # Verify stash exists
    stash = run_git(["stash", "list"], cwd=git_repo)
    assert "My work in progress" in stash.stdout


def test_shelf_push_auto_message(git_repo):
    os.chdir(git_repo)
    (git_repo / "work.txt").write_text("in progress")
    run_git(["add", "work.txt"], cwd=git_repo)

    result = runner.invoke(app, ["shelf", "push"])
    assert result.exit_code == 0
    assert "gx-shelf" in result.output


def test_shelf_push_clean_tree(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["shelf", "push", "nothing"])
    assert result.exit_code == 0
    assert "Nothing to stash" in result.output


def test_shelf_list_empty(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["shelf", "list"])
    assert result.exit_code == 0
    assert "No stashes" in result.output


def test_shelf_list_with_stashes(repo_with_stash):
    os.chdir(repo_with_stash)
    result = runner.invoke(app, ["shelf", "list"])
    assert result.exit_code == 0
    assert "Test stash" in result.output
    assert "1 stash" in result.output


def test_shelf_clear_empty(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["shelf", "clear"])
    assert result.exit_code == 0
    assert "No stashes" in result.output


def test_shelf_clear_dry_run(repo_with_stash):
    os.chdir(repo_with_stash)
    result = runner.invoke(app, ["shelf", "clear", "--dry-run"])
    assert result.exit_code == 0
    assert "DRY RUN" in result.output

    # Stash should still exist
    stash = run_git(["stash", "list"], cwd=repo_with_stash)
    assert "Test stash" in stash.stdout


def test_shelf_clear_confirmed(repo_with_stash):
    os.chdir(repo_with_stash)
    result = runner.invoke(app, ["shelf", "clear"], input="y\n")
    assert result.exit_code == 0
    assert "cleared" in result.output

    # Stash should be gone
    stash = run_git(["stash", "list"], cwd=repo_with_stash)
    assert stash.stdout.strip() == ""


def test_shelf_clear_cancelled(repo_with_stash):
    os.chdir(repo_with_stash)
    result = runner.invoke(app, ["shelf", "clear"], input="n\n")
    assert result.exit_code == 0
    assert "Cancelled" in result.output

    # Stash should still exist
    stash = run_git(["stash", "list"], cwd=repo_with_stash)
    assert "Test stash" in stash.stdout


def test_shelf_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["shelf", "push", "test"])
    assert result.exit_code != 0


def test_shelf_no_stashes_interactive(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["shelf"])
    assert result.exit_code == 0
    assert "No stashes" in result.output
