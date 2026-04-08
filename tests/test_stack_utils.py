"""Tests for stack utils."""

from __future__ import annotations

import json
import os
import subprocess

import pytest


def run_git(args, cwd):
    return subprocess.run(["git"] + args, capture_output=True, text=True, cwd=cwd)


@pytest.fixture
def stacked_repo(tmp_path):
    """Creates a repo with: main -> feature/a -> feature/b -> feature/c."""
    repo = tmp_path / "stacked-repo"
    repo.mkdir()

    def git(*args):
        return run_git(list(args), cwd=repo)

    git("init", "-b", "main")
    git("config", "user.name", "Test User")
    git("config", "user.email", "test@example.com")

    # Initial commit on main
    (repo / "README.md").write_text("# Test\n")
    git("add", "README.md")
    git("commit", "-m", "Initial commit")

    # feature/a on top of main
    git("checkout", "-b", "feature/a")
    (repo / "a.py").write_text("# Feature A\n")
    git("add", "a.py")
    git("commit", "-m", "Add feature A")

    # feature/b on top of feature/a
    git("checkout", "-b", "feature/b")
    (repo / "b.py").write_text("# Feature B\n")
    git("add", "b.py")
    git("commit", "-m", "Add feature B")

    # feature/c on top of feature/b
    git("checkout", "-b", "feature/c")
    (repo / "c.py").write_text("# Feature C\n")
    git("add", "c.py")
    git("commit", "-m", "Add feature C")

    # Write stack.json
    gx_dir = repo / ".git" / "gx"
    gx_dir.mkdir(parents=True, exist_ok=True)
    config = {
        "relationships": {
            "feature/a": "main",
            "feature/b": "feature/a",
            "feature/c": "feature/b",
        },
        "metadata": {"main_branch": "main"},
    }
    (gx_dir / "stack.json").write_text(json.dumps(config))

    git("checkout", "main")
    return repo


def test_load_stack_config(stacked_repo):
    os.chdir(stacked_repo)
    from gx.utils.stack import load_stack_config

    config = load_stack_config()
    assert "feature/a" in config["relationships"]
    assert config["relationships"]["feature/a"] == "main"


def test_load_missing_config(git_repo):
    os.chdir(git_repo)
    from gx.utils.stack import load_stack_config

    config = load_stack_config()
    assert config["relationships"] == {}


def test_record_relationship(git_repo):
    os.chdir(git_repo)
    from gx.utils.stack import load_stack_config, record_relationship

    record_relationship("feature/x", "main")
    config = load_stack_config()
    assert config["relationships"]["feature/x"] == "main"


def test_get_parent(stacked_repo):
    os.chdir(stacked_repo)
    from gx.utils.stack import get_parent

    assert get_parent("feature/a") == "main"
    assert get_parent("feature/b") == "feature/a"
    assert get_parent("nonexistent") is None


def test_get_children(stacked_repo):
    os.chdir(stacked_repo)
    from gx.utils.stack import get_children

    children = get_children("main")
    assert "feature/a" in children


def test_get_stack_chain(stacked_repo):
    os.chdir(stacked_repo)
    from gx.utils.stack import get_stack_chain

    chain = get_stack_chain("feature/c")
    assert chain == ["main", "feature/a", "feature/b", "feature/c"]


def test_get_descendants(stacked_repo):
    os.chdir(stacked_repo)
    from gx.utils.stack import get_descendants

    desc = get_descendants("feature/a")
    assert "feature/b" in desc
    assert "feature/c" in desc


def test_remove_branch(stacked_repo):
    os.chdir(stacked_repo)
    from gx.utils.stack import get_parent, remove_branch

    remove_branch("feature/b")
    assert get_parent("feature/b") is None


def test_clean_deleted_branches(stacked_repo):
    os.chdir(stacked_repo)
    # Delete feature/c locally
    run_git(["branch", "-D", "feature/c"], cwd=stacked_repo)

    from gx.utils.stack import clean_deleted_branches, get_parent

    clean_deleted_branches()
    assert get_parent("feature/c") is None


def test_build_branch_stack(stacked_repo):
    os.chdir(stacked_repo)
    from gx.utils.stack import build_branch_stack

    stack = build_branch_stack()
    assert stack.main_branch == "main"
    assert "main" in stack.all_nodes
    assert "feature/a" in stack.all_nodes
