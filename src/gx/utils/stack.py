"""Stack config I/O, data model, and branch tree builder."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path

from gx.utils.git import (
    GitError,
    branch_exists,
    get_current_branch,
    get_head_branch,
    get_repo_root,
    run_git,
)

# ---------------------------------------------------------------------------
# Data model
# ---------------------------------------------------------------------------

@dataclass
class BranchMeta:
    parent: str
    parent_head: str
    pr_number: int | None = None


@dataclass
class BranchNode:
    name: str
    commit_sha: str = ""
    is_head: bool = False
    is_orphan: bool = False
    is_merged: bool = False
    children: list["BranchNode"] = field(default_factory=list)
    ahead: int = 0
    behind: int = 0


@dataclass
class BranchStack:
    roots: list[BranchNode] = field(default_factory=list)
    all_nodes: dict[str, BranchNode] = field(default_factory=dict)
    main_branch: str = "main"
    orphans: list[BranchNode] = field(default_factory=list)


# ---------------------------------------------------------------------------
# Config I/O
# ---------------------------------------------------------------------------

def _config_path() -> Path:
    try:
        root = get_repo_root()
    except GitError:
        return Path(".git/gx/stack.json")
    return Path(root) / ".git" / "gx" / "stack.json"


def _migrate_config(data: dict) -> dict:
    """Migrate old 'relationships' format to new 'branches' format."""
    if "relationships" in data and "branches" not in data:
        branches: dict[str, dict] = {}
        for child, parent in data["relationships"].items():
            try:
                parent_head = run_git(["merge-base", child, parent])
            except GitError:
                parent_head = ""
            branches[child] = {"parent": parent, "parent_head": parent_head}
        data["branches"] = branches
        del data["relationships"]
    return data


def load_stack_config() -> dict:
    """Load .git/gx/stack.json. Returns empty config if missing."""
    path = _config_path()
    if not path.exists():
        return {"branches": {}, "metadata": {"main_branch": get_head_branch()}}
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
        data = _migrate_config(data)
        if "branches" not in data:
            data["branches"] = {}
        if "metadata" not in data:
            data["metadata"] = {"main_branch": get_head_branch()}
        return dict(data)
    except (json.JSONDecodeError, OSError):
        return {"branches": {}, "metadata": {"main_branch": get_head_branch()}}


def save_stack_config(config: dict) -> None:
    """Write config to .git/gx/stack.json."""
    path = _config_path()
    path.parent.mkdir(parents=True, exist_ok=True)
    config.setdefault("metadata", {})
    config["metadata"]["last_updated"] = datetime.now(timezone.utc).isoformat()
    path.write_text(json.dumps(config, indent=2), encoding="utf-8")


def record_relationship(child: str, parent: str) -> None:
    """Add or update a parent-child relationship with parent_head."""
    config = load_stack_config()
    try:
        parent_head = run_git(["rev-parse", parent])
    except GitError:
        parent_head = ""
    config["branches"][child] = {"parent": parent, "parent_head": parent_head}
    save_stack_config(config)


def update_parent_head(child: str, new_parent_head: str) -> None:
    """Update the parent_head for a branch after sync/retarget."""
    config = load_stack_config()
    if child in config["branches"]:
        config["branches"][child]["parent_head"] = new_parent_head
        save_stack_config(config)


def _get_branches(config: dict) -> dict:
    """Get the branches dict from a loaded config."""
    return dict(config.get("branches", {}))


def get_parent(branch: str) -> str | None:
    """Return the parent branch name, or None."""
    return get_parent_config(load_stack_config(), branch)


def get_parent_config(config: dict, branch: str) -> str | None:
    """Return the parent branch name from a loaded config, or None."""
    entry = _get_branches(config).get(branch)
    if entry is None:
        return None
    return str(entry["parent"])


def get_parent_head(branch: str) -> str | None:
    """Return the stored parent_head SHA for a branch, or None."""
    return get_parent_head_config(load_stack_config(), branch)


def get_parent_head_config(config: dict, branch: str) -> str | None:
    """Return the stored parent_head SHA from a loaded config, or None."""
    entry = _get_branches(config).get(branch)
    if entry is None:
        return None
    head = entry.get("parent_head", "")
    return str(head) if head else None


def get_children(branch: str) -> list[str]:
    """Return all branches that have this branch as their parent."""
    return get_children_config(load_stack_config(), branch)


def get_children_config(config: dict, branch: str) -> list[str]:
    """Return all branches that have this branch as their parent, from a loaded config."""
    return [
        child for child, meta in _get_branches(config).items()
        if meta.get("parent") == branch
    ]


def get_stack_chain(branch: str) -> list[str]:
    """Walk up from branch to root, return the full chain ordered root-first."""
    config = load_stack_config()
    branches = config["branches"]
    chain = [branch]
    visited: set[str] = {branch}
    current = branch
    while current in branches:
        parent = branches[current]["parent"]
        if parent in visited:
            break
        visited.add(parent)
        chain.append(parent)
        current = parent
    chain.reverse()
    return chain


def get_descendants(branch: str) -> list[str]:
    """Walk down from branch, return all descendants in topological order."""
    config = load_stack_config()
    branches = config["branches"]
    child_map: dict[str, list[str]] = {}
    for child, meta in branches.items():
        child_map.setdefault(meta["parent"], []).append(child)

    result: list[str] = []
    visited: set[str] = {branch}

    def _walk(node: str) -> None:
        for child in sorted(child_map.get(node, [])):
            if child in visited:
                continue
            visited.add(child)
            result.append(child)
            _walk(child)

    _walk(branch)
    return result


def topo_sort(branches_list: list[str]) -> list[str]:
    """Sort branches so parents always come before children (Kahn's algorithm)."""
    config = load_stack_config()
    branches = config["branches"]
    branch_set = set(branches_list)

    children_of: dict[str, list[str]] = {b: [] for b in branches_list}
    in_degree: dict[str, int] = {b: 0 for b in branches_list}

    for branch in branches_list:
        if branch in branches:
            parent = branches[branch]["parent"]
            if parent in branch_set:
                children_of.setdefault(parent, []).append(branch)
                in_degree[branch] += 1

    queue = sorted([b for b in branches_list if in_degree[b] == 0])
    result: list[str] = []

    while queue:
        current = queue.pop(0)
        result.append(current)
        for child in sorted(children_of.get(current, [])):
            in_degree[child] -= 1
            if in_degree[child] == 0:
                queue.append(child)

    if len(result) != len(branches_list):
        missing = set(branches_list) - set(result)
        result.extend(sorted(missing))

    return result


def remove_branch(branch: str) -> None:
    """Remove a branch from config (both as child and parent)."""
    config = load_stack_config()
    branches = config["branches"]
    branches.pop(branch, None)
    for child, meta in list(branches.items()):
        if meta.get("parent") == branch:
            del branches[child]
    save_stack_config(config)


def clean_deleted_branches() -> None:
    """Remove entries for branches that no longer exist locally."""
    config = load_stack_config()
    branches = config["branches"]
    to_remove = [child for child in branches if not branch_exists(child)]
    for branch in to_remove:
        del branches[branch]
    if to_remove:
        save_stack_config(config)


# ---------------------------------------------------------------------------
# Tree builder
# ---------------------------------------------------------------------------

def build_branch_stack() -> BranchStack:
    """Construct the full branch hierarchy with self-healing."""
    main = get_head_branch()
    config = load_stack_config()
    branches = config["branches"]

    # Step 1: Get all local branches with SHAs
    fmt = "%(refname:short)\t%(objectname)"
    output = run_git(["for-each-ref", "--format=" + fmt, "refs/heads/"])
    branch_shas: dict[str, str] = {}
    for line in output.splitlines():
        parts = line.split("\t", 1)
        if len(parts) == 2:
            branch_shas[parts[0]] = parts[1]

    if not branch_shas:
        return BranchStack(main_branch=main)

    # Step 2: Clean stale entries
    for child in list(branches.keys()):
        if child not in branch_shas:
            del branches[child]

    # Step 3: Self-heal unknown branches
    config_changed = False
    for branch in branch_shas:
        if branch == main:
            continue
        if branch in branches and branches[branch]["parent"] in branch_shas:
            continue
        detected = _detect_parent(branch, main, branch_shas)
        if detected:
            try:
                parent_head = run_git(["merge-base", branch, detected])
            except GitError:
                parent_head = ""
            branches[branch] = {"parent": detected, "parent_head": parent_head}
            config_changed = True

    if config_changed:
        save_stack_config(config)

    # Step 4: Build nodes
    try:
        current = get_current_branch()
    except GitError:
        current = ""

    nodes: dict[str, BranchNode] = {}
    for name, sha in branch_shas.items():
        nodes[name] = BranchNode(name=name, commit_sha=sha, is_head=(name == current))

    # Step 5: Wire up parent-child links (with cycle detection)
    visited_edges: set[str] = set()
    for child_name, meta in branches.items():
        parent_name = meta["parent"]
        edge = f"{child_name}->{parent_name}"
        if edge in visited_edges:
            continue
        visited_edges.add(edge)
        if child_name in nodes and parent_name in nodes:
            nodes[parent_name].children.append(nodes[child_name])

    # Step 6: Calculate ahead/behind
    for child_name, meta in branches.items():
        parent_name = meta["parent"]
        if child_name not in nodes or parent_name not in nodes:
            continue
        try:
            ab = run_git([
                "rev-list", "--left-right", "--count",
                f"{child_name}...{parent_name}",
            ])
            ahead, behind = ab.split()
            nodes[child_name].ahead = int(ahead)
            nodes[child_name].behind = int(behind)
        except (GitError, ValueError):
            pass
        try:
            merged_out = run_git(["branch", "--merged", parent_name], check=False)
            merged_names = [b.strip().lstrip("* ") for b in merged_out.splitlines()]
            if child_name in merged_names:
                nodes[child_name].is_merged = True
        except GitError:
            pass

    # Step 7: Classify roots and orphans
    children_in_config = set(branches.keys())
    roots: list[BranchNode] = []
    orphans: list[BranchNode] = []

    for name, node in nodes.items():
        if name not in children_in_config:
            if node.children or name == main:
                roots.append(node)
            elif name != main:
                node.is_orphan = True
                orphans.append(node)

    roots.sort(key=lambda n: (n.name != main, n.name))

    def _sort_children(node: BranchNode, seen: set[str] | None = None) -> None:
        if seen is None:
            seen = set()
        if node.name in seen:
            return
        seen.add(node.name)
        node.children.sort(key=lambda n: n.name)
        for child in node.children:
            _sort_children(child, seen)

    for root in roots:
        _sort_children(root)

    return BranchStack(roots=roots, all_nodes=nodes, main_branch=main, orphans=orphans)


def _detect_parent(
    branch: str, main: str, branch_shas: dict[str, str],
) -> str | None:
    """Detect the most likely parent for a branch via merge-base."""
    best_parent = None
    best_distance = float("inf")
    for candidate in branch_shas:
        if candidate == branch:
            continue
        try:
            mb = run_git(["merge-base", branch, candidate])
            count_str = run_git(["rev-list", "--count", f"{mb}..{branch}"])
            distance = int(count_str)
            if distance < best_distance:
                best_distance = distance
                best_parent = candidate
        except (GitError, ValueError):
            continue
    return best_parent
