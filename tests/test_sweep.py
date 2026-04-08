"""Tests for gx sweep."""

from __future__ import annotations

import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


def test_sweep_nothing_to_clean(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["sweep"])
    assert result.exit_code == 0
    assert "tidy" in result.output.lower() or "Nothing" in result.output


def test_sweep_merged_branch(git_repo_with_branches):
    repo = git_repo_with_branches
    os.chdir(repo)

    # Merge feature/auth into main
    run_git(["merge", "feature/auth"], cwd=repo)

    result = runner.invoke(app, ["sweep", "-y"])
    assert result.exit_code == 0
    assert "Deleted" in result.output or "feature/auth" in result.output


def test_sweep_dry_run(git_repo_with_branches):
    repo = git_repo_with_branches
    os.chdir(repo)
    run_git(["merge", "feature/auth"], cwd=repo)

    result = runner.invoke(app, ["sweep", "--dry-run"])
    assert result.exit_code == 0
    assert "DRY RUN" in result.output


def test_sweep_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["sweep"])
    assert result.exit_code != 0
