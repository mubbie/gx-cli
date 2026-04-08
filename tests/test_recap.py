"""Tests for gx recap."""

from __future__ import annotations

import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def test_recap_default(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["recap"])
    assert result.exit_code == 0
    assert "Initial commit" in result.output or "activity" in result.output.lower()


def test_recap_no_activity(git_repo):
    os.chdir(git_repo)
    # Ask for commits from a nonexistent author
    result = runner.invoke(app, ["recap", "@nonexistent-person-xyz"])
    assert result.exit_code == 0
    assert "No activity" in result.output or "commits" in result.output.lower()


def test_recap_days(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["recap", "-d", "7"])
    assert result.exit_code == 0


def test_recap_all_authors(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["recap", "--all"])
    assert result.exit_code == 0
    assert "Test User" in result.output or "activity" in result.output.lower()


def test_recap_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["recap"])
    assert result.exit_code != 0
