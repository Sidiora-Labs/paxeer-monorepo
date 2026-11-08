import json
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[2]
LAUNCHPAD = ROOT / "apps/launchpad"
SDK_IMPORTER = "../../sdk/launchpad/packages/sdk"
DEPENDENCY_GROUPS = ("devDependencies", "dependencies", "optionalDependencies")


def test_launchpad_lockfile_importers_match_manifests():
    lock = yaml.safe_load((LAUNCHPAD / "pnpm-lock.yaml").read_text())
    workspace = yaml.safe_load((LAUNCHPAD / "pnpm-workspace.yaml").read_text())
    manifest = json.loads((LAUNCHPAD / "package.json").read_text())
    overrides = manifest["pnpm"]["overrides"]
    workspace_dirs = {"."}
    for pattern in workspace["packages"]:
        workspace_dirs.update(
            str(path.relative_to(LAUNCHPAD))
            for path in LAUNCHPAD.glob(pattern)
            if (path / "package.json").is_file()
        )
    assert set(lock["importers"]) == workspace_dirs

    for importer, locked in lock["importers"].items():
        manifest_path = LAUNCHPAD / importer / "package.json"
        manifest = json.loads(manifest_path.read_text())
        owners = {
            name: group
            for group in DEPENDENCY_GROUPS
            for name in manifest.get(group, {})
        }
        for group in DEPENDENCY_GROUPS:
            expected = {
                name: overrides.get(name, specifier)
                for name, specifier in manifest.get(group, {}).items()
                if owners[name] == group
            }
            actual = {
                name: dependency["specifier"]
                for name, dependency in locked.get(group, {}).items()
            }
            assert actual == expected, f"{importer}: {group} differs from {manifest_path}"


def test_launchpad_lockfile_has_pyodide():
    lock = yaml.safe_load((LAUNCHPAD / "pnpm-lock.yaml").read_text())
    manifest = json.loads((LAUNCHPAD / SDK_IMPORTER / "package.json").read_text())
    assert manifest["devDependencies"]["pyodide"] == "0.29.3"
    pyodide = lock["importers"][SDK_IMPORTER]["devDependencies"]["pyodide"]
    assert pyodide["specifier"] == manifest["devDependencies"]["pyodide"]
    assert pyodide["version"].split("(", 1)[0] == "0.29.3"
    assert lock["packages"]["pyodide@0.29.3"]["resolution"]["integrity"]
    assert f"pyodide@{pyodide['version']}" in lock["snapshots"]
