import subprocess
from pathlib import Path, PurePosixPath

from programme.kvx import check_format

ROOT = Path(__file__).resolve().parents[4]
DATA = ROOT / "spec/layerx-merge/programme/data/external-gaps.kvx"
RULING = (
    "recorded external gap; no task regenerates the consumer "
    "until the artefact is committed"
)


def test_every_gap_names_existing_consumer() -> None:
    sections = check_format(DATA)
    expected = {
        "launchpad/spec/production-system/integration/contracts/v1/openapi.json": (
            "sdk/launchpad/packages/sdk/scripts/generate.py"
        ),
        "launchpad/spec/production-system/integration/contracts/v1/asyncapi.json": (
            "sdk/launchpad/packages/sdk/scripts/generate.py"
        ),
        "launchpad/spec/production-system/integration/contracts/v2/openapi.json": (
            "sdk/launchpad/packages/sdk/src/operations.gen.ts"
        ),
        "launchpad/spec/production-system/integration/contracts/v2/asyncapi.json": (
            "sdk/launchpad/packages/sdk/src/operations.gen.ts"
        ),
        "launchpad/spec/production-system/spec.kvx": (
            "tools/launchpad/production/qualify.py"
        ),
        "launchpad/evidence": "tools/launchpad/production/qualify.py",
        "launchpad/spec/production-system/handoffs": (
            "services/launchpad/user/internal/account/account_test.go"
        ),
        "launchpad/spec/production-system/integration/migrations": (
            "services/launchpad/api/internal/rest/production_integration_test.go"
        ),
    }
    assert set(sections) == {"meta"} | {
        f"gap.{number}" for number in range(1, len(expected) + 1)
    }
    gaps = {name: row for name, row in sections.items() if name.startswith("gap.")}
    assert len(gaps) == len(expected)
    for name, row in gaps.items():
        assert set(row) == {"artefact", "consumer", "owner_domain", "ruling"}, name
        assert type(row["owner_domain"]) is int, name
        assert row["owner_domain"] == 4, name
        assert row["ruling"] == RULING, name
        for field in ("artefact", "consumer"):
            value = row[field]
            assert isinstance(value, str) and value, (name, field)
            path = PurePosixPath(value)
            assert not path.is_absolute(), (name, field, value)
            assert ".." not in path.parts, (name, field, value)
            assert path.as_posix() == value, (name, field, value)
    assert {row["artefact"]: row["consumer"] for row in gaps.values()} == expected

    paths = sorted(set(expected) | set(expected.values()))
    result = subprocess.run(
        ["git", "ls-tree", "-r", "--name-only", "-z", "HEAD", "--", *paths],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    tracked = set(result.stdout.split("\0")) - {""}
    for name, row in gaps.items():
        consumer = row["consumer"]
        artefact = row["artefact"]
        assert consumer in tracked, f"{name}: consumer absent at HEAD: {consumer}"
        assert (ROOT / consumer).is_file(), f"{name}: consumer absent: {consumer}"
        assert not any(
            path == artefact or path.startswith(artefact + "/") for path in tracked
        ), f"{name}: artefact already committed at HEAD: {artefact}"
        source = ROOT / artefact
        assert not source.exists(), f"{name}: artefact already present: {artefact}"
        assert not source.is_symlink(), f"{name}: artefact is a symlink: {artefact}"
