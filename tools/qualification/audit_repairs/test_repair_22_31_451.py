"""Offline dependency and import proof for launchpad load repair 22.31.451."""

import compileall
import importlib.metadata
import importlib.util
from pathlib import Path
import re
import sys

from packaging.requirements import Requirement
from packaging.utils import canonicalize_name
from packaging.version import Version
import pytest


ROOT = Path(__file__).resolve().parents[3]
LOAD = ROOT / "tools/launchpad/production/load"
# Read with gh api /advisories/<GHSA> during implementation. Every advisory
# names a patched release, so each dependency path is repaired by upgrading.
ADVISORY_MINIMUMS = (
    ("GHSA-m2h6-j472-rp4c", "cryptography", "49.0.0"),
    ("GHSA-jwv3-5hgf-82ww", "cryptography", "49.0.0"),
    ("GHSA-g6cj-pr64-35w5", "cryptography", "50.0.0"),
    ("GHSA-537c-gmf6-5ccf", "cryptography", "48.0.1"),
    ("GHSA-p423-j2cm-9vmq", "cryptography", "46.0.7"),
    ("GHSA-m959-cc7f-wv43", "cryptography", "46.0.6"),
    ("GHSA-r6ph-v2qm-q3c2", "cryptography", "46.0.5"),
    ("GHSA-6w46-j5rx-g56g", "pytest", "9.0.3"),
)


def pinned_versions(path, *, hashed=False):
    versions = {}
    for line in path.read_text().replace("\\\n", " ").splitlines():
        line = line.split("#", 1)[0].strip()
        if not line:
            continue
        requirement_text, *hashes = line.split()
        requirement = Requirement(requirement_text)
        assert requirement.url is None and requirement.marker is None
        assert not requirement.extras
        specifiers = list(requirement.specifier)
        assert len(specifiers) == 1 and specifiers[0].operator == "=="
        name = canonicalize_name(requirement.name)
        assert name not in versions, f"duplicate pin: {name}"
        versions[name] = Version(specifiers[0].version)
        if hashed:
            assert hashes, f"missing artifact hashes: {name}"
            assert all(re.fullmatch(r"--hash=sha256:[0-9a-f]{64}", h) for h in hashes)
        else:
            assert not hashes
    assert versions
    return versions


@pytest.mark.parametrize("advisory,package,minimum", ADVISORY_MINIMUMS)
def test_advisory_patched_manifest_lock_and_install(advisory, package, minimum):
    manifest = pinned_versions(LOAD / "requirements.txt")
    locked = pinned_versions(LOAD / "requirements.lock", hashed=True)
    assert manifest[package] >= Version(minimum), advisory
    assert locked[package] == manifest[package], advisory
    assert Version(importlib.metadata.version(package)) == locked[package], advisory


def test_frozen_lock_covers_requirements_and_installed_dependency_closure():
    manifest = pinned_versions(LOAD / "requirements.txt")
    locked = pinned_versions(LOAD / "requirements.lock", hashed=True)
    assert set(manifest) == {"cryptography", "jsonschema", "pytest"}
    assert manifest["cryptography"] >= Version("50.0.0")
    assert manifest["pytest"] >= Version("9.0.3")
    assert manifest["jsonschema"] == Version("4.25.1")
    assert manifest.items() <= locked.items()
    for name, version in locked.items():
        distribution = importlib.metadata.distribution(name)
        assert Version(distribution.version) == version, name
        for text in distribution.requires or ():
            dependency = Requirement(text)
            if dependency.marker is None or dependency.marker.evaluate({"extra": ""}):
                dependency_name = canonicalize_name(dependency.name)
                assert dependency_name in locked, (name, dependency_name)
                assert locked[dependency_name] in dependency.specifier, (name, text)


def test_load_tooling_compiles_and_imports_by_path():
    assert compileall.compile_dir(str(LOAD), quiet=2, force=True)
    paths = sorted(LOAD.glob("*.py"))
    assert paths
    names = {path.stem for path in paths}
    previous = {name: sys.modules[name] for name in names if name in sys.modules}
    old_path = sys.path[:]
    try:
        sys.path.insert(0, str(LOAD))
        for name in names:
            sys.modules.pop(name, None)
        for path in paths:
            spec = importlib.util.spec_from_file_location(path.stem, path)
            assert spec is not None and spec.loader is not None, path
            module = importlib.util.module_from_spec(spec)
            sys.modules[path.stem] = module
            spec.loader.exec_module(module)
            assert Path(module.__file__).resolve() == path.resolve()
    finally:
        sys.path[:] = old_path
        for name in names:
            sys.modules.pop(name, None)
        sys.modules.update(previous)
