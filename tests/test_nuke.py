"""Tests for gx nuke."""

from __future__ import annotations

import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


def test_nuke_branch(git_repo_with_branches):
    repo = git_repo_with_branches
    os.chdir(repo)
    result = runner.invoke(app, ["nuke", "feature/auth"], input="y\n")
    assert result.exit_code == 0
    assert "Deleted" in result.output

    # Verify branch is gone
    branches = run_git(["branch", "--list", "feature/auth"], cwd=repo)
    assert "feature/auth" not in branches.stdout


def test_nuke_current_branch(git_repo_with_branches):
    repo = git_repo_with_branches
    os.chdir(repo)
    # Switch to feature/auth first
    run_git(["checkout", "feature/auth"], cwd=repo)
    result = runner.invoke(app, ["nuke", "feature/auth"])
    assert result.exit_code != 0 or "current branch" in result.output.lower()


def test_nuke_head_branch(git_repo_with_branches):
    repo = git_repo_with_branches
    os.chdir(repo)
    result = runner.invoke(app, ["nuke", "main"])
    assert result.exit_code != 0 or "HEAD branch" in result.output


def test_nuke_nonexistent(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["nuke", "does-not-exist"])
    assert result.exit_code != 0


def test_nuke_dry_run(git_repo_with_branches):
    repo = git_repo_with_branches
    os.chdir(repo)
    result = runner.invoke(app, ["nuke", "feature/auth", "--dry-run"])
    assert result.exit_code == 0
    assert "DRY RUN" in result.output

    # Branch should still exist
    branches = run_git(["branch", "--list", "feature/auth"], cwd=repo)
    assert "feature/auth" in branches.stdout


def test_nuke_local_only(git_repo_with_branches):
    repo = git_repo_with_branches
    os.chdir(repo)
    result = runner.invoke(app, ["nuke", "feature/auth", "--local"], input="y\n")
    assert result.exit_code == 0
    assert "Deleted" in result.output
