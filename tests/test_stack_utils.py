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

    (repo / "README.md").write_text("# Test\n")
    git("add", "README.md")
    git("commit", "-m", "Initial commit")

    git("checkout", "-b", "feature/a")
    (repo / "a.py").write_text("# Feature A\n")
    git("add", "a.py")
    git("commit", "-m", "Add feature A")

    git("checkout", "-b", "feature/b")
    (repo / "b.py").write_text("# Feature B\n")
    git("add", "b.py")
    git("commit", "-m", "Add feature B")

    git("checkout", "-b", "feature/c")
    (repo / "c.py").write_text("# Feature C\n")
    git("add", "c.py")
    git("commit", "-m", "Add feature C")

    # Write new-format stack.json
    gx_dir = repo / ".git" / "gx"
    gx_dir.mkdir(parents=True, exist_ok=True)
    config = {
        "branches": {
            "feature/a": {"parent": "main", "parent_head": "abc123"},
            "feature/b": {"parent": "feature/a", "parent_head": "def456"},
            "feature/c": {"parent": "feature/b", "parent_head": "ghi789"},
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
    assert "feature/a" in config["branches"]
    assert config["branches"]["feature/a"]["parent"] == "main"


def test_load_missing_config(git_repo):
    os.chdir(git_repo)
    from gx.utils.stack import load_stack_config

    config = load_stack_config()
    assert config["branches"] == {}


def test_migration_from_old_format(git_repo):
    """Test auto-migration from old relationships format."""
    os.chdir(git_repo)
    # Create a branch so merge-base works
    run_git(["checkout", "-b", "feature/x"], cwd=git_repo)
    (git_repo / "x.py").write_text("x\n")
    run_git(["add", "x.py"], cwd=git_repo)
    run_git(["commit", "-m", "x"], cwd=git_repo)
    run_git(["checkout", "main"], cwd=git_repo)

    # Write old-format config
    gx_dir = git_repo / ".git" / "gx"
    gx_dir.mkdir(parents=True, exist_ok=True)
    old_config = {"relationships": {"feature/x": "main"}, "metadata": {"main_branch": "main"}}
    (gx_dir / "stack.json").write_text(json.dumps(old_config))

    from gx.utils.stack import load_stack_config

    config = load_stack_config()
    assert "branches" in config
    assert "relationships" not in config
    assert config["branches"]["feature/x"]["parent"] == "main"
    assert config["branches"]["feature/x"]["parent_head"] != ""


def test_record_relationship(git_repo):
    os.chdir(git_repo)
    from gx.utils.stack import load_stack_config, record_relationship

    record_relationship("feature/x", "main")
    config = load_stack_config()
    assert config["branches"]["feature/x"]["parent"] == "main"
    assert config["branches"]["feature/x"]["parent_head"] != ""


def test_get_parent(stacked_repo):
    os.chdir(stacked_repo)
    from gx.utils.stack import get_parent

    assert get_parent("feature/a") == "main"
    assert get_parent("feature/b") == "feature/a"
    assert get_parent("nonexistent") is None


def test_get_parent_head(stacked_repo):
    os.chdir(stacked_repo)
    from gx.utils.stack import get_parent_head

    assert get_parent_head("feature/a") == "abc123"
    assert get_parent_head("nonexistent") is None


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


def test_topo_sort(stacked_repo):
    os.chdir(stacked_repo)
    from gx.utils.stack import topo_sort

    # Provide branches out of order
    result = topo_sort(["feature/c", "feature/a", "main", "feature/b"])
    assert result.index("main") < result.index("feature/a")
    assert result.index("feature/a") < result.index("feature/b")
    assert result.index("feature/b") < result.index("feature/c")


def test_remove_branch(stacked_repo):
    os.chdir(stacked_repo)
    from gx.utils.stack import get_parent, remove_branch

    remove_branch("feature/b")
    assert get_parent("feature/b") is None


def test_clean_deleted_branches(stacked_repo):
    os.chdir(stacked_repo)
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


def test_cycle_detection(stacked_repo):
    """Cycle in config should not infinite-loop."""
    os.chdir(stacked_repo)

    # Write a cyclic config
    gx_dir = stacked_repo / ".git" / "gx"
    config = {
        "branches": {
            "feature/a": {"parent": "feature/b", "parent_head": "x"},
            "feature/b": {"parent": "feature/a", "parent_head": "y"},
        },
        "metadata": {"main_branch": "main"},
    }
    (gx_dir / "stack.json").write_text(json.dumps(config))

    from gx.utils.stack import get_stack_chain

    # Should not hang — cycle guard breaks the loop
    chain = get_stack_chain("feature/a")
    assert len(chain) <= 3  # At most a, b, a would be caught
