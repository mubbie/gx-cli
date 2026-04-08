"""Tests for gx context."""

from __future__ import annotations

import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


def test_context_basic(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["context"])
    assert result.exit_code == 0
    assert "Branch:" in result.output or "branch" in result.output.lower()
    assert "Last commit:" in result.output or "commit" in result.output.lower()


def test_context_alias(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["ctx"])
    assert result.exit_code == 0
    assert "Branch:" in result.output


def test_context_clean_tree(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["context"])
    assert result.exit_code == 0
    assert "clean" in result.output


def test_context_dirty_tree(git_repo):
    os.chdir(git_repo)
    (git_repo / "dirty.txt").write_text("uncommitted")
    result = runner.invoke(app, ["context"])
    assert result.exit_code == 0
    assert "Untracked" in result.output or "Modified" in result.output


def test_context_with_staged(git_repo):
    os.chdir(git_repo)
    (git_repo / "staged.txt").write_text("staged content")
    run_git(["add", "staged.txt"], cwd=git_repo)
    result = runner.invoke(app, ["context"])
    assert result.exit_code == 0
    assert "Staged" in result.output


def test_context_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["context"])
    assert result.exit_code != 0
