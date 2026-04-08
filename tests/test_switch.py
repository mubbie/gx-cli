"""Tests for gx switch."""

from __future__ import annotations

import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


def test_switch_previous(git_repo_with_branches):
    repo = git_repo_with_branches
    os.chdir(repo)

    # We're on main, switch to feature first
    run_git(["checkout", "feature/auth"], cwd=repo)
    run_git(["checkout", "main"], cwd=repo)

    result = runner.invoke(app, ["switch", "-"])
    assert result.exit_code == 0
    assert "Switched" in result.output or "feature/auth" in result.output


def test_switch_no_previous(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["switch", "-"])
    assert result.exit_code != 0 or "No previous" in result.output or "error" in result.output.lower()


def test_switch_single_branch(git_repo):
    os.chdir(git_repo)
    # Only main exists, so no other branch to switch to
    result = runner.invoke(app, ["switch"])
    assert result.exit_code == 0
    assert "No other branches" in result.output or "No branches" in result.output


def test_switch_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["switch"])
    assert result.exit_code != 0
