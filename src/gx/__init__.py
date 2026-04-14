"""gx: Git Productivity Toolkit."""

from __future__ import annotations

import os


def _get_version() -> str:
    # 1. Try bundled VERSION file (PyInstaller builds)
    version_file = os.path.join(os.path.dirname(__file__), "VERSION")
    if os.path.exists(version_file):
        try:
            with open(version_file) as f:
                return f.read().strip()
        except OSError:
            pass

    # 2. Try importlib.metadata (pip/pipx installs)
    try:
        from importlib.metadata import version
        return version("gx-git")
    except Exception:
        pass

    # 3. Fallback
    return "dev"


__version__ = _get_version()
