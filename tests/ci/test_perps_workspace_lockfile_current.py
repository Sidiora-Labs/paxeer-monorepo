import glob
import json
import os
from pathlib import Path
import subprocess

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
PERPS = ROOT / "apps/perps"
DEPENDENCY_GROUPS = ("devDependencies", "dependencies", "optionalDependencies")
VENDORED_ROOTS = ("core/deps", "programs/vendor", "tools/hyperfide")


def _tracked_lockfiles():
    paths = subprocess.check_output(
        ["git", "ls-files", "-z", "--", "pnpm-lock.yaml", "**/pnpm-lock.yaml"],
        cwd=ROOT,
    ).decode().split("\0")
    return sorted(
        path
        for path in paths
        if path
        and not any(path.startswith(root + "/") for root in VENDORED_ROOTS)
        and not {"vendor", "vendored", "node_modules", "third_party"}.intersection(
            Path(path).parts
        )
    )


def _read_lockfile(path):
    lock = {
        "importers": {},
        "overrides": {},
        "packages": {},
        "snapshots": {},
        "settings": {},
    }
    documents = list(yaml.safe_load_all(path.read_text()))
    assert documents, f"{path}: empty lockfile"
    for document in documents:
        assert isinstance(document, dict) and "lockfileVersion" in document, path
        for section in lock:
            for name, value in document.get(section, {}).items():
                if section == "importers":
                    importer = lock[section].setdefault(name, {})
                    for group, dependencies in value.items():
                        assert group not in importer, (path, name, group)
                        importer[group] = dependencies
                else:
                    assert name not in lock[section], (path, section, name)
                    lock[section][name] = value
    assert lock["importers"], f"{path}: no importers"
    return lock


def _manifest_errors(lock_path, lock):
    errors = []
    for importer, locked in lock["importers"].items():
        manifest_path = lock_path.parent / importer / "package.json"
        if not manifest_path.is_file():
            errors.append(f"{lock_path}: {importer}: missing {manifest_path}")
            continue
        manifest = json.loads(manifest_path.read_text())
        if lock["settings"].get("autoInstallPeers", False):
            declared = {
                name
                for group in DEPENDENCY_GROUPS
                for name in manifest.get(group, {})
            }
            manifest["dependencies"] = {
                **{
                    name: specifier
                    for name, specifier in manifest.get("peerDependencies", {}).items()
                    if name not in declared
                },
                **manifest.get("dependencies", {}),
            }
        owners = {
            name: group
            for group in DEPENDENCY_GROUPS
            for name in manifest.get(group, {})
        }
        for group in DEPENDENCY_GROUPS:
            expected = {
                name: lock["overrides"].get(name, specifier)
                for name, specifier in manifest.get(group, {}).items()
                if owners[name] == group
            }
            actual = {
                name: dependency["specifier"]
                for name, dependency in locked.get(group, {}).items()
            }
            if actual != expected:
                errors.append(
                    f"{lock_path.relative_to(ROOT)}: {importer}: {group}: "
                    f"manifest={expected!r}, lockfile={actual!r}"
                )
    return errors


def test_perps_lock_importers_match_members():
    lock_path = PERPS / "pnpm-lock.yaml"
    lock = _read_lockfile(lock_path)
    workspace = yaml.safe_load((PERPS / "pnpm-workspace.yaml").read_text())
    members = {"."}
    excluded = set()
    for pattern in workspace["packages"]:
        negative = pattern.startswith("!")
        matches = {
            os.path.relpath(PERPS / path, PERPS)
            for path in glob.glob(pattern.lstrip("!"), root_dir=PERPS, recursive=True)
            if (PERPS / path / "package.json").is_file()
        }
        if negative:
            excluded.update(matches)
        else:
            members.update(matches)
    assert set(lock["importers"]) == members - excluded
    assert "../../services/wallet/gateway" in members - excluded
    assert workspace["sharedWorkspaceLockfile"] is True
    assert not (errors := _manifest_errors(lock_path, lock)), "\n".join(errors)

    gateway = "../../services/wallet/gateway"
    manifest = json.loads((PERPS / gateway / "package.json").read_text())
    for name, version in {
        "@electric-sql/pglite": "0.5.15",
        "@electric-sql/pglite-socket": "0.2.18",
    }.items():
        assert manifest["devDependencies"][name] == version
        dependency = lock["importers"][gateway]["devDependencies"][name]
        assert dependency["specifier"] == version
        assert dependency["version"].split("(", 1)[0] == version
        assert lock["packages"][f"{name}@{version}"]["resolution"]["integrity"]
        assert f"{name}@{dependency['version']}" in lock["snapshots"]


@pytest.mark.parametrize("lockfile", _tracked_lockfiles())
def test_every_pnpm_lock_importer_matches_manifest(lockfile):
    lock_path = ROOT / lockfile
    lock = _read_lockfile(lock_path)
    assert not (errors := _manifest_errors(lock_path, lock)), "\n".join(errors)
