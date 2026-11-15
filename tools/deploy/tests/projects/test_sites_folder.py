import importlib.util
import tomllib
from collections import Counter
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
FOLDER = ROOT / "deploy/projects/sites"
TOOL = ROOT / "tools/deploy/platform-project.py"


def project_tool():
    spec = importlib.util.spec_from_file_location("platform_project", TOOL)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_sites_folder_loads_every_service_and_uses_project_repository():
    meta, services = project_tool().load(FOLDER)
    documents = [
        tomllib.loads(path.read_text(encoding="utf-8"))
        for path in sorted(FOLDER.glob("*.kvx"))
    ]
    expected = Counter(
        name for document in documents for name in document.get("service", {})
    )
    actual = Counter(service["name"] for service in services)
    assert expected
    assert actual == expected

    project = tomllib.loads((FOLDER / "project.kvx").read_text(encoding="utf-8"))
    repository = project["meta"]["repo"]
    assert meta["repo"] == repository
    assert all(
        service["repo"] == repository
        for service in services
        if service["kind"] == "repo"
    )
