"""Ensure every wallet project definition is accepted by the project loader."""

from __future__ import annotations

import importlib.util
from collections import Counter
from pathlib import Path
import tomllib


ROOT = Path(__file__).resolve().parents[4]
FOLDER = ROOT / "deploy/projects/wallet"
TOOL_PATH = ROOT / "tools/deploy/platform-project.py"


_tool_spec = importlib.util.spec_from_file_location("platform_project", TOOL_PATH)
if _tool_spec is None or _tool_spec.loader is None:
    raise RuntimeError(f"cannot load project tool from {TOOL_PATH}")
_platform_project = importlib.util.module_from_spec(_tool_spec)
_tool_spec.loader.exec_module(_platform_project)


def test_wallet_folder_definitions_load_as_project_services() -> None:
    meta, services = _platform_project.load(FOLDER)
    project = tomllib.loads((FOLDER / "project.kvx").read_text(encoding="utf-8"))
    expected = Counter()
    for path in sorted(FOLDER.glob("*.kvx")):
        if path.name == "project.kvx":
            continue
        document = tomllib.loads(path.read_text(encoding="utf-8"))
        expected.update(document["service"].keys())

    actual = Counter(service["name"] for service in services)
    assert actual == expected
    assert meta["repo"] == project["meta"]["repo"]
    assert all(
        service["repo"] == project["meta"]["repo"]
        for service in services
        if service["kind"] == "repo"
    )
