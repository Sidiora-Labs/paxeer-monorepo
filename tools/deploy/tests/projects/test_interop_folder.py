"""Ensure every interop definition is accepted by the project tool."""

import importlib.util
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
FOLDER = ROOT / "deploy/projects/interop"
TOOL = ROOT / "tools/deploy/platform-project.py"


def project_tool():
    spec = importlib.util.spec_from_file_location("platform_project", TOOL)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_interop_folder_loads_every_service_and_uses_project_repository():
    meta, services = project_tool().load(FOLDER)
    definition_files = sorted(FOLDER.glob("*.kvx"))
    service_names = [
        section.removeprefix("service.")
        for path in definition_files
        for section in tomllib.loads(path.read_text()).get("service", {})
    ]

    assert sorted(service["name"] for service in services) == sorted(service_names)
    repository = meta["repo"]
    assert all(
        service["repo"] == repository
        for service in services
        if service["kind"] == "repo"
    )
