"""Tests for gx oops."""

from __future__ import annotations

import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


def test_oops_amend_message(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["oops", "-m", "Better initial commit"], input="y\n")
    assert result.exit_code == 0
    assert "amended" in result.output.lower()

    # Verify message changed
    log = run_git(["log", "-1", "--format=%s"], cwd=git_repo)
    assert "Better initial commit" in log.stdout


def test_oops_add_file(git_repo):
    os.chdir(git_repo)
    (git_repo / "forgotten.txt").write_text("oops forgot this")
    result = runner.invoke(app, ["oops", "--add", "forgotten.txt"], input="y\n")
    assert result.exit_code == 0
    assert "added" in result.output.lower() or "File" in result.output


def test_oops_no_commits(tmp_path):
    repo = tmp_path / "empty"
    repo.mkdir()
    subprocess.run(["git", "init"], cwd=repo, capture_output=True)
    os.chdir(repo)
    result = runner.invoke(app, ["oops", "-m", "test"])
    assert result.exit_code != 0 or "No commits" in result.output


def test_oops_dry_run(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["oops", "-m", "New message", "--dry-run"])
    assert result.exit_code == 0
    assert "DRY RUN" in result.output or "Would" in result.output


def test_oops_nonexistent_file(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["oops", "--add", "does_not_exist.txt"])
    assert result.exit_code != 0 or "not found" in result.output.lower()


def test_oops_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["oops", "-m", "test"])
    assert result.exit_code != 0
