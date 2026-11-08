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
    ROOT / "services/platform/hosted/internal/Cargo.toml",
    ROOT / "services/platform/ramps/toolkit/Cargo.toml",
)
# Task 22.39.93 removes these exact exceptions as it relocates the fixtures.
KEY_MATERIAL_BASELINE = set(
    json.loads(
        (ROOT / "tests/ci/data/key_material_fixture_paths.json").read_text(encoding="utf-8")
    )
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
    assert misplaced == KEY_MATERIAL_BASELINE, (
        "Tracked key material must live under tests, fixtures or testdata.\n"
        "Unexpected paths:\n"
        + "\n".join(sorted(misplaced - KEY_MATERIAL_BASELINE))
        + "\nBaseline entries to remove after relocation:\n"
        + "\n".join(sorted(KEY_MATERIAL_BASELINE - misplaced))
    )


def test_fixture_key_dirs_have_test_only_readme():
    for manifest in TLS_MANIFESTS:
        with manifest.open("rb") as source:
            package = tomllib.load(source)["package"]
        relative = PurePosixPath(package["metadata"]["test-fixtures"]["tls"])
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
