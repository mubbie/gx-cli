"""Tests for gx who."""

from __future__ import annotations

import os
import subprocess

import pytest
from typer.testing import CliRunner

from gx.main import app

runner = CliRunner()


def test_who_repo_level(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["who"])
    assert result.exit_code == 0
    assert "Test User" in result.output or "contributor" in result.output.lower()


def test_who_file_level(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["who", "README.md"])
    assert result.exit_code == 0
    assert "README.md" in result.output or "Test User" in result.output


def test_who_file_not_found(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["who", "nonexistent.txt"])
    assert result.exit_code != 0 or "not found" in result.output.lower()


def test_who_with_limit(git_repo):
    os.chdir(git_repo)
    result = runner.invoke(app, ["who", "-n", "1"])
    assert result.exit_code == 0


def test_who_not_git_repo(tmp_path):
    os.chdir(tmp_path)
    result = runner.invoke(app, ["who"])
    assert result.exit_code != 0
