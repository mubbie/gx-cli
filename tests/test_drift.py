"""Tests for gx drift."""

from __future__ import annotations

import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


def test_drift_on_main(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["drift"])
    assert result.exit_code == 0
    assert "on main" in result.output.lower() or "Switch to a feature" in result.output


def test_drift_with_divergence(git_repo_with_branches):
    repo = git_repo_with_branches
    os.chdir(repo)
    # Switch to feature branch
    run_git(["checkout", "feature/auth"], cwd=repo)

    result = runner.invoke(app, ["drift"])
    assert result.exit_code == 0
    assert "ahead" in result.output or "behind" in result.output or "diverge" in result.output.lower()


def test_drift_no_divergence(git_repo):
    os.chdir(git_repo)
    # Create and switch to branch at same point as main
    run_git(["checkout", "-b", "feature/aligned"], cwd=git_repo)
    result = runner.invoke(app, ["drift"])
    assert result.exit_code == 0
    assert "No divergence" in result.output or "up to date" in result.output.lower()


def test_drift_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["drift"])
    assert result.exit_code != 0


def test_drift_full_flag(git_repo_with_branches):
    repo = git_repo_with_branches
    os.chdir(repo)
    run_git(["checkout", "feature/auth"], cwd=repo)
    result = runner.invoke(app, ["drift", "--full"])
    assert result.exit_code == 0
