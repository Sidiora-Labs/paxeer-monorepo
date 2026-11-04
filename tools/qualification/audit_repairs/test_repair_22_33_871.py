"""Prove f004 using the production shell scanner and committed workspace inputs."""

import json
from pathlib import Path
import shutil
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCANNER = Path("scripts/workspace/check-platform-dependencies.sh")
REGRESSION = Path("scripts/workspace/check-platform-dependencies-test.sh")
INVENTORY = Path("tools/workspace/platform-dependency-roots.txt")
ROWS = (
    "rust|services/platform/Cargo.toml|locked-workspace-root",
    "rust|services/platform/hosted/core/Cargo.toml|covered-by-platform-cargo",
    "npm|services/platform/middleware/agent/package.json|covered-by-root-npm",
)


def run_shell(*args, cwd):
    return subprocess.run(
        args,
        cwd=cwd,
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )


@pytest.fixture
def workspace(tmp_path):
    """Copy real peer files; do not manufacture replacement manifests or locks."""
    inventory = (ROOT / INVENTORY).read_text(encoding="utf-8").splitlines()
    for row in ROWS:
        assert inventory.count(row) == 1
    paths = [Path(row.split("|")[1]) for row in ROWS]
    paths += [SCANNER, REGRESSION, Path("package.json"), Path("services/platform/Cargo.lock")]
    for path in paths:
        destination = tmp_path / path
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / path, destination)
        assert destination.read_bytes() == (ROOT / path).read_bytes()
    destination = tmp_path / INVENTORY
    destination.parent.mkdir(parents=True)
    destination.write_text(
        "# kind|path|classification\n" + "\n".join(reversed(ROWS)) + "\n",
        encoding="utf-8",
    )
    return tmp_path


def test_f004_shell_syntax_and_guarded_commands():
    for path in (SCANNER, REGRESSION):
        result = run_shell("bash", "-n", str(ROOT / path), cwd=ROOT)
        assert result.returncode == 0, result.stdout + result.stderr
    source = (ROOT / SCANNER).read_text(encoding="utf-8")
    assert "find services/platform -type d" in source
    assert "find platform " not in source
    for command in (
        "set -eu",
        "inventory=${1:-tools/workspace/platform-dependency-roots.txt}",
        "-name target -o -name node_modules -o -name build -o -name .build",
        "-prune -o",
        'LC_ALL=C sort > "$actual"',
        "awk -F '|' '!/^#/ && NF == 3 { print $2 }'",
        'LC_ALL=C sort > "$declared"',
        'cmp "$actual" "$declared"',
        'test -s "$path"',
        'test -s "${path%/Cargo.toml}/Cargo.lock"',
        'test "$kind" = rust',
        'test "$kind" = swift-lock',
        "grep -q '\"revision\"' \"$path\"",
        'test "$kind" = npm',
        'directory=${path%/package.json}',
        'grep -Fq',
        'test -s "${path%/package.json}/package-lock.json"',
        '*) echo "unknown Platform dependency classification: $classification" >&2; exit 1',
    ):
        assert command in source, command


def test_f004_committed_producer_inventory_consumer(workspace):
    platform = tomllib.loads(
        (workspace / "services/platform/Cargo.toml").read_text(encoding="utf-8")
    )
    assert "hosted/core" in platform["workspace"]["members"]
    core = tomllib.loads(
        (workspace / "services/platform/hosted/core/Cargo.toml").read_text(encoding="utf-8")
    )
    assert core["package"]["name"]
    lock = tomllib.loads(
        (workspace / "services/platform/Cargo.lock").read_text(encoding="utf-8")
    )
    assert core["package"]["name"] in {package["name"] for package in lock["package"]}
    npm = json.loads((workspace / "package.json").read_text(encoding="utf-8"))
    assert "services/platform/middleware/agent" in npm["workspaces"]
    member = json.loads(
        (workspace / "services/platform/middleware/agent/package.json").read_text(encoding="utf-8")
    )
    assert member["name"] == "@sidiora/layerx-agent-middleware"
    for args in ((), (str(INVENTORY),)):
        result = run_shell("sh", str(ROOT / SCANNER), *args, cwd=workspace)
        assert result.returncode == 0, result.stdout + result.stderr


@pytest.mark.parametrize("case", ("missing-root", "extra-root", "unlocked-npm"))
def test_f004_negative_inventory_cases(workspace, case):
    rows = list(ROWS)
    if case == "missing-root":
        rows.remove(ROWS[1])
    elif case == "extra-root":
        rows.append(
            "rust|services/platform/hosted/absent/Cargo.toml|covered-by-platform-cargo"
        )
    else:
        assert not (workspace / "services/platform/middleware/agent/package-lock.json").exists()
        rows[2] = rows[2].replace("covered-by-root-npm", "locked-npm-independent")
    inventory = workspace / f"{case}.txt"
    inventory.write_text("\n".join(rows) + "\n", encoding="utf-8")
    result = run_shell("sh", str(ROOT / SCANNER), str(inventory), cwd=workspace)
    assert result.returncode != 0, f"Scanner accepted {case}: {result.stdout}"
    if case in ("missing-root", "extra-root"):
        assert "differ" in result.stdout or "EOF" in result.stderr


def test_f004_retains_existing_shell_regression(workspace):
    result = run_shell("sh", str(workspace / REGRESSION), cwd=workspace)
    assert result.returncode == 0, result.stdout + result.stderr
    assert result.stdout.strip() == "check-platform-dependencies regression: ok"


def test_f004_repair_record_maps_finding_to_proofs():
    record = tomllib.loads(
        Path(__file__).with_name("repair_22_33_871.kvx").read_text(encoding="utf-8")
    )
    finding = record["finding"]["f004"]
    assert finding["audit"] == "22.30.183"
    assert finding["file"] == str(SCANNER)
    assert finding["tests"] == [
        "test_f004_shell_syntax_and_guarded_commands",
        "test_f004_committed_producer_inventory_consumer",
        "test_f004_negative_inventory_cases",
        "test_f004_retains_existing_shell_regression",
    ]
    for name in finding["tests"]:
        assert callable(globals()[name])
    assert record["qualification"]["evidence_ledger"] == "/out/run/gates.jsonl"
