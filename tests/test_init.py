"""Tests for gx init."""

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


def test_init_fresh(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["init"])
    assert result.exit_code == 0
    assert "Initialized" in result.output
    assert (git_repo / ".git" / "gx" / "stack.json").exists()


def test_init_detects_trunk(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["init"])
    assert result.exit_code == 0
    config = json.loads((git_repo / ".git" / "gx" / "stack.json").read_text())
    assert config["metadata"]["main_branch"] == "main"


def test_init_explicit_trunk(git_repo):
    os.chdir(git_repo)
    run_git(["checkout", "-b", "develop"], cwd=git_repo)
    run_git(["checkout", "main"], cwd=git_repo)
    result = runner.invoke(app, ["init", "--trunk", "develop"])
    assert result.exit_code == 0
    config = json.loads((git_repo / ".git" / "gx" / "stack.json").read_text())
    assert config["metadata"]["main_branch"] == "develop"


def test_init_trunk_nonexistent(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["init", "--trunk", "nonexistent"])
    assert result.exit_code != 0
    assert "does not exist" in result.output


def test_init_already_initialized(git_repo):
    os.chdir(git_repo)
    runner.invoke(app, ["init"])
    result = runner.invoke(app, ["init"])
    assert result.exit_code == 0
    assert "already initialized" in result.output


def test_init_force(git_repo):
    os.chdir(git_repo)
    runner.invoke(app, ["init"])
    # Add a relationship first
    from gx.utils.stack import record_relationship
    record_relationship("feature/x", "main")

    result = runner.invoke(app, ["init", "--force"])
    assert result.exit_code == 0
    assert "Re-initialized" in result.output

    # Verify relationships preserved
    config = json.loads((git_repo / ".git" / "gx" / "stack.json").read_text())
    assert "feature/x" in config["branches"]


def test_init_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["init"])
    assert result.exit_code != 0
