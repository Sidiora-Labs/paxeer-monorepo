"""Every platform service definition loads with the project repository."""

import importlib.util
import tomllib
from collections import Counter
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
FOLDER = ROOT / "deploy/projects/platform"


def test_platform_folder_loads_every_service_with_project_repository() -> None:
    definitions = [
        tomllib.loads(path.read_text()) for path in sorted(FOLDER.glob("*.kvx"))
    ]
    expected = Counter(
        name for definition in definitions for name in definition.get("service", {})
    )
    assert expected
    project = tomllib.loads((FOLDER / "project.kvx").read_text())["meta"]

    spec = importlib.util.spec_from_file_location(
        "platform_folder_project", ROOT / "tools/deploy/platform-project.py"
    )
    assert spec is not None and spec.loader is not None
    tool = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(tool)
    meta, services = tool.load(FOLDER)

    assert meta == project
    assert Counter(service["name"] for service in services) == expected
    for service in services:
        if service["kind"] == "repo":
            assert service["repo"] == project["repo"], service["name"]
