"""Guard vendored Rust sources and require reasons for registered patches."""

import hashlib
import json
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[2]
REFRESHED_FILES = {
    "wasm-instrument": (
        "Cargo.toml",
        "src/gas_metering/mod.rs",
        "src/gas_metering/validation.rs",
    ),
    "wasmi-0.31.2": (
        "src/engine/mod.rs",
        "src/execution_trace.rs",
        "src/store.rs",
    ),
}


@pytest.fixture(scope="module")
def vendored_manifests():
    manifests = tuple(
        sorted(
            path
            for path in ROOT.rglob(".cargo-checksum.json")
            if "vendor" in path.relative_to(ROOT).parts[:-1]
        )
    )
    assert manifests, "No vendored crate checksum manifests found"
    assert any(
        path.is_relative_to(ROOT / "programs/vendor") for path in manifests
    ), "No programs/vendor crate checksum manifests found"
    return manifests


def test_every_vendored_crate_matches_its_manifest(vendored_manifests):
    failures = []
    for manifest_path in vendored_manifests:
        crate = manifest_path.parent
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        files = manifest["files"]
        assert isinstance(files, dict) and files, (
            f"{manifest_path.relative_to(ROOT)}: empty or invalid file manifest"
        )
        for name, expected in sorted(files.items()):
            relative = Path(name)
            label = f"{crate.relative_to(ROOT)}/{name}"
            if (
                relative.is_absolute()
                or ".." in relative.parts
                or relative.as_posix() != name
            ):
                failures.append(f"{label}: noncanonical checksum path")
                continue
            source = crate / relative
            if source.resolve() != source.absolute() or not source.is_file():
                failures.append(f"{label}: missing file, symlink, or special file")
                continue
            actual = hashlib.sha256(source.read_bytes()).hexdigest()
            if actual != expected:
                failures.append(f"{label}: expected {expected}, got {actual}")
    assert not failures, "Vendored checksum mismatches:\n" + "\n".join(failures)


def test_patched_vendored_crates_record_reasons(vendored_manifests):
    vendor = ROOT / "programs/vendor"
    inventory = ROOT / "tests/ci/data/vendored_patched_crates.txt"
    names = [
        line.strip()
        for line in inventory.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]
    assert names, "No patched vendored crates registered"
    assert len(names) == len(set(names)), "Duplicate patched vendored crate entries"
    assert all(
        Path(name).parts == (name,) and name not in {".", ".."} for name in names
    ), "Patched crate entries must be single directory names"
    assert set(names) == set(REFRESHED_FILES), (
        "Patched crate inventory must match the refreshed-file records"
    )
    expected_records = {vendor / name / "PATCHES.md" for name in names}
    actual_records = set(vendor.rglob("PATCHES.md"))
    assert actual_records == expected_records, (
        "Patched crate inventory and PATCHES.md records differ: "
        f"missing={sorted(expected_records - actual_records)}, "
        f"unregistered={sorted(actual_records - expected_records)}"
    )

    failures = []
    for name in names:
        crate = vendor / name
        manifest_path = crate / ".cargo-checksum.json"
        assert manifest_path in vendored_manifests, f"{name}: no checksum manifest"
        files = json.loads(manifest_path.read_text(encoding="utf-8"))["files"]
        patches = (crate / "PATCHES.md").read_text(encoding="utf-8")
        assert patches.strip(), f"{name}: PATCHES.md is empty"
        for filename in REFRESHED_FILES[name]:
            if filename not in files:
                failures.append(f"{name}/{filename}: missing checksum entry")
            if f"`{filename}`" not in patches:
                failures.append(f"{name}: PATCHES.md does not name {filename}")
    assert not failures, "Undocumented vendored patches:\n" + "\n".join(failures)
