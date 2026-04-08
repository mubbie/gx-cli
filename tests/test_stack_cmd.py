"""Tests for gx stack."""

from __future__ import annotations

import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


def test_stack_create(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["stack", "feature/new", "main"])
    assert result.exit_code == 0
    assert "Created" in result.output
    assert "feature/new" in result.output

    # Verify branch exists
    branches = run_git(["branch", "--list", "feature/new"], cwd=git_repo)
    assert "feature/new" in branches.stdout


def test_stack_already_exists(git_repo):
    os.chdir(git_repo)
    run_git(["checkout", "-b", "feature/existing"], cwd=git_repo)
    run_git(["checkout", "main"], cwd=git_repo)
    result = runner.invoke(app, ["stack", "feature/existing", "main"])
    assert result.exit_code != 0 or "already exists" in result.output


def test_stack_invalid_parent(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["stack", "feature/new", "nonexistent"])
    assert result.exit_code != 0 or "does not exist" in result.output


def test_stack_invalid_name(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["stack", "bad name with spaces", "main"])
    assert result.exit_code != 0 or "Invalid" in result.output


def test_stack_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["stack", "feature/new", "main"])
    assert result.exit_code != 0


def test_stack_records_relationship(git_repo):
    os.chdir(git_repo)
    runner.invoke(app, ["stack", "feature/tracked", "main"])

    from gx.utils.stack import get_parent

    assert get_parent("feature/tracked") == "main"
