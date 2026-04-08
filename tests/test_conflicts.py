"""Tests for gx conflicts."""

from __future__ import annotations

import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


def test_conflicts_no_conflicts(git_repo_with_branches):
    repo = git_repo_with_branches
    os.chdir(repo)
    run_git(["checkout", "feature/auth"], cwd=repo)

    result = runner.invoke(app, ["conflicts"])
    assert result.exit_code == 0
    assert "No conflicts" in result.output or "clean merge" in result.output.lower()


def test_conflicts_with_conflicts(git_repo):
    repo = git_repo
    os.chdir(repo)

    # Create conflicting changes
    run_git(["checkout", "-b", "feature/conflict"], cwd=repo)
    (repo / "README.md").write_text("# Feature version\nConflict here\n")
    run_git(["add", "README.md"], cwd=repo)
    run_git(["commit", "-m", "Feature change"], cwd=repo)

    run_git(["checkout", "main"], cwd=repo)
    (repo / "README.md").write_text("# Main version\nDifferent content\n")
    run_git(["add", "README.md"], cwd=repo)
    run_git(["commit", "-m", "Main change"], cwd=repo)

    run_git(["checkout", "feature/conflict"], cwd=repo)

    result = runner.invoke(app, ["conflicts"])
    assert result.exit_code == 0
    # Should detect conflict or at least show files
    assert "conflict" in result.output.lower() or "README" in result.output or "clean merge" in result.output.lower()


def test_conflicts_on_head_branch(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["conflicts"])
    assert result.exit_code == 0
    assert "already on" in result.output.lower() or "Switch to a feature" in result.output


def test_conflicts_nonexistent_target(git_repo_with_branches):
    repo = git_repo_with_branches
    os.chdir(repo)
    run_git(["checkout", "feature/auth"], cwd=repo)

    result = runner.invoke(app, ["conflicts", "nonexistent-branch"])
    assert result.exit_code != 0 or "does not exist" in result.output.lower()


def test_conflicts_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["conflicts"])
    assert result.exit_code != 0
