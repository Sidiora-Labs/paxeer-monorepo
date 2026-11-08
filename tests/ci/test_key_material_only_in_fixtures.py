"""Keep first-party key material in documented test fixture directories."""

import json
from pathlib import Path, PurePosixPath
import subprocess
import tomllib


ROOT = Path(__file__).resolve().parents[2]
KEY_SUFFIXES = {".pem", ".der", ".p12", ".key", ".jks", ".pfx"}
FIXTURE_DIRECTORIES = {"tests", "fixtures", "testdata"}
VENDORED_TREES = (PurePosixPath("core/deps"), PurePosixPath("programs/vendor"))
TLS_MANIFESTS = (
    ROOT / "services/agent/crates/layerx-sdk/Cargo.toml",
    ROOT / "services/platform/hosted/internal/Cargo.toml",
    ROOT / "services/platform/ramps/reference/Cargo.toml",
    ROOT / "services/platform/ramps/toolkit/Cargo.toml",
)


def _tracked_key_paths():
    result = subprocess.run(
        ["git", "ls-files", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    return [
        path
        for name in result.stdout.split("\0")
        if name
        and (path := PurePosixPath(name)).suffix.lower() in KEY_SUFFIXES
        and not any(path.is_relative_to(tree) for tree in VENDORED_TREES)
    ]


def test_key_like_files_live_under_fixtures():
    paths = _tracked_key_paths()
    assert paths, "Expected tracked first-party key fixtures"
    misplaced = {
        str(path)
        for path in paths
        if not FIXTURE_DIRECTORIES.intersection(path.parts[:-1])
    }
    assert not misplaced, (
        "Tracked key material must live under tests, fixtures or testdata.\n"
        "Unexpected paths:\n"
        + "\n".join(sorted(misplaced))
    )


def test_key_material_baseline_is_empty():
    baseline = ROOT / "tests/ci/data/key_material_baseline.txt"
    assert baseline.read_bytes() == b"", "Key material has no baseline exceptions"


def test_key_material_only_in_fixtures():
    inventory = ROOT / "tests/ci/data/key_material_fixture_paths.json"
    paths = json.loads(inventory.read_text(encoding="utf-8"))
    assert paths, "Expected an inventory of key fixtures"
    tracked = set(_tracked_key_paths())
    for name in paths:
        path = PurePosixPath(name)
        assert not path.is_absolute() and ".." not in path.parts, name
        assert ("tests", "fixtures") in zip(path.parts[:-1], path.parts[1:-1]), name
        assert path in tracked, f"Key fixture is not tracked: {name}"
        assert (ROOT / path).is_file(), f"Key fixture is missing: {name}"


def test_fixture_key_dirs_have_test_only_readme():
    for manifest in TLS_MANIFESTS:
        with manifest.open("rb") as source:
            package = tomllib.load(source)["package"]
        fixtures = package["metadata"]["test-fixtures"]
        assert "tls" in fixtures, manifest
        for fixture in fixtures.values():
            relative = PurePosixPath(fixture)
            assert relative.parts[:2] == ("tests", "fixtures"), manifest
            assert not relative.is_absolute() and ".." not in relative.parts, manifest
            directory = manifest.parent / relative
            assert directory.is_dir(), directory
            material = [
                path for path in directory.iterdir() if path.suffix.lower() in KEY_SUFFIXES
            ]
            assert material, f"No key material found in {directory}"
            readme = directory / "README.md"
            assert readme.is_file(), f"Missing test-only material notice: {readme}"
            lines = readme.read_text(encoding="utf-8").strip().splitlines()
            assert len(lines) == 1, f"Expected a one-line notice: {readme}"
            notice = lines[0].lower()
            assert "generated test data" in notice, readme
            assert "no production use" in notice, readme
