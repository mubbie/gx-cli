"""Shared test fixtures for gx tests."""

from __future__ import annotations

import os
import subprocess

import pytest


@pytest.fixture
def git_repo(tmp_path):
    """Create a temporary git repo with an initial commit."""
    repo = tmp_path / "test-repo"
    repo.mkdir()

    def run(args, cwd=repo):
        result = subprocess.run(
            ["git"] + args,
            capture_output=True,
            text=True,
            cwd=cwd,
        )
        return result

    run(["init", "-b", "main"])
    run(["config", "user.name", "Test User"])
    run(["config", "user.email", "test@example.com"])

    # Initial commit
    readme = repo / "README.md"
    readme.write_text("# Test\n")
    run(["add", "README.md"])
    run(["commit", "-m", "Initial commit"])

    return repo


@pytest.fixture
def git_repo_with_branches(git_repo):
    """Create a repo with multiple branches."""
    repo = git_repo

    def run(args):
        return subprocess.run(
            ["git"] + args,
            capture_output=True,
            text=True,
            cwd=repo,
        )

    # Create feature branch
    run(["checkout", "-b", "feature/auth"])
    (repo / "auth.py").write_text("# Auth module\n")
    run(["add", "auth.py"])
    run(["commit", "-m", "Add auth module"])

    # Create another branch
    run(["checkout", "main"])
    run(["checkout", "-b", "feature/search"])
    (repo / "search.py").write_text("# Search module\n")
    run(["add", "search.py"])
    run(["commit", "-m", "Add search module"])

    # Go back to main
    run(["checkout", "main"])

    return repo
