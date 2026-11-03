import importlib.util
import json
from pathlib import Path

import pytest


SCRIPT = Path(__file__).resolve().parents[1] / "lockfloor.py"
SPEC = importlib.util.spec_from_file_location("lockfloor", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


@pytest.fixture
def lock_path(tmp_path):
    (tmp_path / "package.json").write_text(
        json.dumps({"name": "range-test", "packageManager": "pnpm@10.0.0"})
    )
    lock = tmp_path / "pnpm-lock.yaml"
    lock.write_text(
        "lockfileVersion: '9.0'\n"
        "packages:\n"
        "  '@opentelemetry/core@1.30.1': {}\n"
        "  '@opentelemetry/core@2.8.0': {}\n"
        "snapshots:\n"
        "  '@opentelemetry/core@1.30.1': {}\n"
        "  '@opentelemetry/core@2.8.0': {}\n"
    )
    return lock


def write_floors(lock, floor):
    floors = lock.parent / "floors.json"
    floors.write_text(json.dumps({
        "manifests": {str(lock.resolve()): {"@opentelemetry/core": floor}}
    }))
    return floors


def test_scoped_floor_allows_older_major_but_plain_floor_rejects_it(lock_path, capsys):
    floors = write_floors(lock_path, {"minimum": "2.8.0", "applies_from": "2.0.0"})
    assert MODULE.lockfloor(floors) == 0
    captured = capsys.readouterr()
    assert (captured.out, captured.err) == ("", "")

    floors = write_floors(lock_path, "2.8.0")
    assert MODULE.lockfloor(floors) == 1
    captured = capsys.readouterr()
    assert captured.out == f"{lock_path}\t@opentelemetry/core\t1.30.1\t2.8.0\n"
    assert captured.err == ""


@pytest.mark.parametrize("version", ["2.0.0", "2.7.9"])
def test_scoped_floor_rejects_instances_in_advisory_range(lock_path, capsys, version):
    lock_path.write_text(lock_path.read_text().replace("1.30.1", version))
    floors = write_floors(lock_path, {"minimum": "2.8.0", "applies_from": "2.0.0"})
    assert MODULE.lockfloor(floors) == 1
    captured = capsys.readouterr()
    assert captured.out == f"{lock_path}\t@opentelemetry/core\t{version}\t2.8.0\n"
    assert captured.err == ""


@pytest.mark.parametrize(
    "floor",
    [
        {"applies_from": "2.0.0"},
        {"minimum": 2.8, "applies_from": "2.0.0"},
        {"minimum": "2.8.0"},
        {"minimum": "2.8.0", "applies_from": 2},
    ],
)
def test_scoped_floor_requires_version_strings(lock_path, floor):
    floors = write_floors(lock_path, floor)
    with pytest.raises(ValueError, match="package floors must be version strings"):
        MODULE.lockfloor(floors)


@pytest.mark.parametrize("field", ["minimum", "applies_from"])
def test_scoped_floor_validates_both_versions(lock_path, field):
    floor = {"minimum": "2.8.0", "applies_from": "2.0.0"}
    floor[field] = "latest"
    floors = write_floors(lock_path, floor)
    with pytest.raises(ValueError, match="invalid version: 'latest'"):
        MODULE.lockfloor(floors)
