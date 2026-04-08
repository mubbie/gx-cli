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


def load_stack_config() -> dict:
    """Load .git/gx/stack.json. Returns empty config if missing."""
    path = _config_path()
    if not path.exists():
        return {"relationships": {}, "metadata": {"main_branch": get_head_branch()}}
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
        if "relationships" not in data:
            data["relationships"] = {}
        if "metadata" not in data:
            data["metadata"] = {"main_branch": get_head_branch()}
        return dict(data)
    except (json.JSONDecodeError, OSError):
        return {"relationships": {}, "metadata": {"main_branch": get_head_branch()}}


def save_stack_config(config: dict) -> None:
    """Write config to .git/gx/stack.json."""
    path = _config_path()
    path.parent.mkdir(parents=True, exist_ok=True)
    config.setdefault("metadata", {})
    config["metadata"]["last_updated"] = datetime.now(timezone.utc).isoformat()
    path.write_text(json.dumps(config, indent=2), encoding="utf-8")


def record_relationship(child: str, parent: str) -> None:
    """Add or update a parent-child relationship."""
    config = load_stack_config()
    config["relationships"][child] = parent
    save_stack_config(config)


def get_parent(branch: str) -> str | None:
    """Return the parent branch, or None."""
    config = load_stack_config()
    result = config["relationships"].get(branch)
    return str(result) if result is not None else None


def get_children(branch: str) -> list[str]:
    """Return all branches that have this branch as their parent."""
    config = load_stack_config()
    return [child for child, parent in config["relationships"].items() if parent == branch]


def get_stack_chain(branch: str) -> list[str]:
    """Walk up from branch to root, return the full chain ordered root-first."""
    config = load_stack_config()
    rels = config["relationships"]
    chain = [branch]
    current = branch
    seen: set[str] = {branch}
    while current in rels:
        parent = rels[current]
        if parent in seen:
            break  # cycle guard
        chain.append(parent)
        seen.add(parent)
        current = parent
    chain.reverse()
    return chain


def get_descendants(branch: str) -> list[str]:
    """Walk down from branch, return all descendants in topological order."""
    config = load_stack_config()
    rels = config["relationships"]
    # Build child map
    child_map: dict[str, list[str]] = {}
    for child, parent in rels.items():
        child_map.setdefault(parent, []).append(child)

    result: list[str] = []

    def _walk(node: str) -> None:
        for child in child_map.get(node, []):
            result.append(child)
            _walk(child)

    _walk(branch)
    return result


def remove_branch(branch: str) -> None:
    """Remove a branch from relationships (both as child and parent)."""
    config = load_stack_config()
    rels = config["relationships"]
    rels.pop(branch, None)
    # Orphan any children
    for child, parent in list(rels.items()):
        if parent == branch:
            del rels[child]
    save_stack_config(config)


def clean_deleted_branches() -> None:
    """Remove entries for branches that no longer exist locally."""
    config = load_stack_config()
    rels = config["relationships"]
    to_remove = []
    for child in list(rels.keys()):
        if not branch_exists(child):
            to_remove.append(child)
    for branch in to_remove:
        del rels[branch]
    if to_remove:
        save_stack_config(config)


# ---------------------------------------------------------------------------
# Tree builder
# ---------------------------------------------------------------------------

def build_branch_stack() -> BranchStack:
    """Construct the full branch hierarchy with self-healing."""
    main = get_head_branch()
    config = load_stack_config()
    rels = config["relationships"]

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
    for child in list(rels.keys()):
        if child not in branch_shas:
            del rels[child]

    # Step 3: Self-heal — detect parents for branches without relationships
    config_changed = False
    for branch in branch_shas:
        if branch == main:
            continue
        if branch in rels and rels[branch] in branch_shas:
            continue
        # Try to detect parent via merge-base closeness
        detected = _detect_parent(branch, main, branch_shas)
        if detected:
            rels[branch] = detected
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
        nodes[name] = BranchNode(
            name=name,
            commit_sha=sha,
            is_head=(name == current),
        )

    # Step 5: Wire up parent-child links
    for child_name, parent_name in rels.items():
        if child_name in nodes and parent_name in nodes:
            nodes[parent_name].children.append(nodes[child_name])

    # Step 6: Calculate ahead/behind for each relationship
    for child_name, parent_name in rels.items():
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

        # Check if merged
        try:
            merged_out = run_git(["branch", "--merged", parent_name], check=False)
            merged_names = [b.strip().lstrip("* ") for b in merged_out.splitlines()]
            if child_name in merged_names:
                nodes[child_name].is_merged = True
        except GitError:
            pass

    # Step 7: Classify roots and orphans
    children_in_rels = set(rels.keys())
    roots: list[BranchNode] = []
    orphans: list[BranchNode] = []

    for name, node in nodes.items():
        if name not in children_in_rels:
            if node.children or name == main:
                roots.append(node)
            elif name != main:
                node.is_orphan = True
                orphans.append(node)

    # Ensure main is first root
    roots.sort(key=lambda n: (n.name != main, n.name))

    # Sort children alphabetically at each level
    def _sort_children(node: BranchNode) -> None:
        node.children.sort(key=lambda n: n.name)
        for child in node.children:
            _sort_children(child)

    for root in roots:
        _sort_children(root)

    return BranchStack(
        roots=roots,
        all_nodes=nodes,
        main_branch=main,
        orphans=orphans,
    )


def _detect_parent(
    branch: str,
    main: str,
    branch_shas: dict[str, str],
) -> str | None:
    """Detect the most likely parent for a branch via merge-base."""
    best_parent = None
    best_distance = float("inf")

    for candidate in branch_shas:
        if candidate == branch:
            continue
        try:
            mb = run_git(["merge-base", branch, candidate])
            # Distance = commits between merge-base and branch
            count_str = run_git(["rev-list", "--count", f"{mb}..{branch}"])
            distance = int(count_str)
            if distance < best_distance:
                best_distance = distance
                best_parent = candidate
        except (GitError, ValueError):
            continue

    return best_parent
