"""Offline proof of the Python SDK's default test discovery contract."""

import ast
from fnmatch import fnmatch
from hashlib import sha256
from importlib.util import module_from_spec, spec_from_file_location
from pathlib import Path
import tomllib


ROOT = Path(__file__).resolve().parents[3]
SDK = ROOT / "services/agent/sdk/python"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_19.kvx"


def record():
    return tomllib.loads(RECORD.read_text(encoding="utf-8"))


def test_manifest_includes_both_suites_without_filtering():
    manifest = tomllib.loads((SDK / "pyproject.toml").read_text(encoding="utf-8"))
    assert manifest["tool"]["pytest"]["ini_options"] == {
        "pythonpath": ["."],
        "testpaths": ["../../../../tests/agent/sdk/python", "tests"],
    }
    assert manifest["build-system"] == {
        "requires": [],
        "build-backend": "build_backend",
        "backend-path": ["."],
    }
    assert manifest["project"]["optional-dependencies"] == {
        "derivation": ["cryptography>=41"],
    }


def test_pytest_consumes_the_owning_sdk_collection_contract(monkeypatch):
    path = SDK / "tests/test_collection_contract.py"
    spec = spec_from_file_location("sdk_collection_contract_22_33_19", path)
    assert spec is not None and spec.loader is not None
    module = module_from_spec(spec)
    spec.loader.exec_module(module)
    module.test_default_sdk_collection_contract(monkeypatch)


def test_existing_receipt_restart_and_rejection_suites_are_preserved():
    baseline = record()["baseline"]
    local = SDK / "tests"
    for name, digest in baseline["local_sha256"].items():
        source = (local / name).read_bytes()
        assert sha256(source).hexdigest() == digest, name
        tree = ast.parse(source, filename=name)
        assert any(
            isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
            for node in tree.body
        ), name
    central = ROOT / "tests/agent/sdk/python"
    paths = sorted(central.glob("test_*.py"))
    digest = sha256()
    for path in paths:
        digest.update(path.name.encode() + b"\0" + path.read_bytes() + b"\0")
    assert len(paths) == baseline["central_module_count"]
    assert digest.hexdigest() == baseline["central_sha256"]
    assert {p.name for p in local.glob("test_*.py")} == (
        set(baseline["local_sha256"]) | {"test_collection_contract.py"}
    )


def test_documented_gate_covers_the_complete_discovery_inventory():
    documentation = (SDK / "TESTING.md").read_text(encoding="utf-8")
    assert "cd services/agent/sdk/python\npython3 -m pytest -q" in documentation
    assert "../../../../tests/agent/sdk/python" in documentation
    assert (
        "tests/test_collection_contract.py::test_default_sdk_collection_contract"
        in documentation
    )
    assert "Future `test_*.py` modules in either directory" in documentation
    assert "does\nnot execute the SDK suites" in documentation
    manifest = tomllib.loads((SDK / "pyproject.toml").read_text(encoding="utf-8"))
    roots = manifest["tool"]["pytest"]["ini_options"]["testpaths"]
    inventory = {
        path.resolve()
        for root in roots
        for path in (SDK / root).rglob("*.py")
        if fnmatch(path.name, "test_*.py") or fnmatch(path.name, "*_test.py")
    }
    for name in record()["baseline"]["local_sha256"]:
        assert f"`{name}`" in documentation
        assert (SDK / "tests" / name).resolve() in inventory
    assert inventory >= {
        path.resolve()
        for path in (ROOT / "tests/agent/sdk/python").glob("test_*.py")
    }
    assert (SDK / "tests/test_collection_contract.py").resolve() in inventory


def test_record_maps_the_finding_and_limits_the_claimed_evidence():
    data = record()
    repair = data["repair"]
    assert repair["task"] == "22.33.19"
    assert repair["base_revision"] == "39bdf7bceaa86b05bbba7248df1e01f4f36022e3"
    assert repair["build_cmd"] == "python3 --version"
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_19.py"
    )
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    assert "not executed" in repair["review_only"]
    assert data["baseline"]["verification_run"] is False
    assert data["baseline"]["observed_failing_tests"] == []
    assert "explicit task order" in data["baseline"]["reason"]
    finding = data["finding"]["local-test-discovery"]
    assert finding["audit"] == "22.30.100"
    assert finding["file"] == "services/agent/sdk/python/pyproject.toml"
    assert finding["severity"] == "medium"
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert len(finding["tests"]) == len(declared)
    requests = data["wiring"]["requests"]
    assert any(
        "spec/layerx-merge/spec.kvx" in request
        and "Domain 9" in request
        and "22.9.11" in request
        for request in requests
    )
    assert any(
        ".github/workflows/xweb-test.yml" in request
        and "Domain 9" in request
        for request in requests
    )
