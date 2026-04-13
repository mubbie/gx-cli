"""Tests for gx parent."""

from __future__ import annotations

import json
import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


def test_parent_on_main(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["parent"])
    assert result.exit_code != 0


def test_parent_fallback_to_main(git_repo):
    os.chdir(git_repo)
    run_git(["checkout", "-b", "feature/x"], cwd=git_repo)
    result = runner.invoke(app, ["parent"])
    assert result.exit_code == 0
    assert "main" in result.output.strip()


def test_parent_from_stack(git_repo):
    os.chdir(git_repo)
    run_git(["checkout", "-b", "feature/a"], cwd=git_repo)
    (git_repo / "a.py").write_text("a\n")
    run_git(["add", "a.py"], cwd=git_repo)
    run_git(["commit", "-m", "a"], cwd=git_repo)

    gx_dir = git_repo / ".git" / "gx"
    gx_dir.mkdir(parents=True, exist_ok=True)
    config = {
        "branches": {"feature/a": {"parent": "main", "parent_head": "x"}},
        "metadata": {"main_branch": "main"},
    }
    (gx_dir / "stack.json").write_text(json.dumps(config))

    result = runner.invoke(app, ["parent"])
    assert result.exit_code == 0
    assert result.output.strip() == "main"


def test_parent_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["parent"])
    assert result.exit_code != 0
