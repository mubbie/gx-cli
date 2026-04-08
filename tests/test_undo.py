"""Tests for gx undo."""

from __future__ import annotations

import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


def test_undo_not_git_repo(tmp_path):
    result = runner.invoke(app, ["undo"], env={"GIT_DIR": str(tmp_path / ".git")})
    # Should fail since tmp_path is not a git repo
    assert result.exit_code != 0 or "Not a git repository" in result.output


def test_undo_nothing_to_undo(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["undo"], input="n\n")
    # Should detect the initial commit or say nothing to undo
    assert result.exit_code == 0 or "Nothing to undo" in result.output


def test_undo_staged_files(git_repo):
    os.chdir(git_repo)
    # Stage a file
    (git_repo / "new.txt").write_text("hello")
    run_git(["add", "new.txt"], cwd=git_repo)

    result = runner.invoke(app, ["undo"], input="y\n")
    assert result.exit_code == 0
    assert "staged" in result.output.lower() or "Unstaged" in result.output


def test_undo_commit(git_repo):
    os.chdir(git_repo)
    # Make a commit
    (git_repo / "feature.txt").write_text("feature code")
    run_git(["add", "feature.txt"], cwd=git_repo)
    run_git(["commit", "-m", "Add feature"], cwd=git_repo)

    result = runner.invoke(app, ["undo"], input="y\n")
    assert result.exit_code == 0
    assert "commit" in result.output.lower() or "Undone" in result.output


def test_undo_dry_run(git_repo):
    os.chdir(git_repo)
    (git_repo / "feature.txt").write_text("feature code")
    run_git(["add", "feature.txt"], cwd=git_repo)
    run_git(["commit", "-m", "Add feature"], cwd=git_repo)

    result = runner.invoke(app, ["undo", "--dry-run"])
    assert result.exit_code == 0
    assert "DRY RUN" in result.output


def test_undo_history_empty(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["undo", "--history"])
    assert result.exit_code == 0
    assert "No undo/redo history" in result.output or "History" in result.output


def test_redo_nothing(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["redo"])
    assert "Nothing to redo" in result.output
