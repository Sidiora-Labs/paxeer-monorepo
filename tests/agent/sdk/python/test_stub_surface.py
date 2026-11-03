from __future__ import annotations

import ast
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4] / "services/agent/sdk/python"
sys.path.insert(0, str(ROOT))

import layerx_sdk  # noqa: E402


def test_stub_names_are_exported() -> None:
    stub = ROOT / "layerx_sdk/__init__.pyi"
    declarations = ast.parse(stub.read_text(), filename=str(stub))
    names = {
        alias.name
        for node in declarations.body
        if isinstance(node, ast.ImportFrom) and node.level == 1
        for alias in node.names
        if alias.asname == alias.name
    }
    assert names, "package stub declares no re-exports"
    missing = sorted(name for name in names if not hasattr(layerx_sdk, name))
    assert not missing, f"package stub names missing from layerx_sdk: {missing}"
