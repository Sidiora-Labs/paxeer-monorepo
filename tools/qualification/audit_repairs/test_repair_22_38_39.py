"""Cross-check the served inference interfaces, source contract and series manifest."""

from pathlib import Path
import re
import sys

import pytest


ROOT = Path(__file__).resolve().parents[3]
SPEC = ROOT / "spec/layerx-merge"
sys.path.insert(0, str(SPEC))

from programme.interfaces_reconcile import _symbol, interfaces_reconcile
from programme.kvx import load


RECORD = load(ROOT / "tools/qualification/audit_repairs/repair_22_38_39.kvx")["series"]
REGISTRY = load(SPEC / "interfaces.kvx")
INTERFACES = {
    name: row
    for name, row in REGISTRY.items()
    if name.startswith("interface.22.inference-")
}
TASKS = load(SPEC / "spec.kvx")
CLIENT_TESTS = {
    "22.38.31": "services/agent/sdk/typescript/test/inference-market.test.ts",
    "22.38.32": "services/agent/crates/layerx-sdk/tests/inference_market.rs",
    "22.38.33": "apps/explorer/ui/pages/PaxeerXInference.spec.tsx",
    "22.38.34": "apps/wallet/src/surfaces/InferenceView.test.tsx",
}


def test_inference_interface_inventory():
    assert len(INTERFACES) == 15
    assert len(RECORD["interfaces"]) == len(set(RECORD["interfaces"]))
    assert set(INTERFACES) == set(RECORD["interfaces"])


@pytest.mark.parametrize("name", RECORD["interfaces"])
def test_inference_interface_is_served_at_its_declaration(name):
    row = INTERFACES[name]
    assert row["status"] == "served", name
    source = ROOT / row["path"]
    assert source.is_file(), row["path"]
    lines = source.read_text(encoding="utf-8").splitlines()
    assert 0 < row["line"] <= len(lines), name
    declaration = " ".join(" ".join(lines[row["line"] - 1 :]).split())
    assert declaration.startswith(" ".join(row["signature"].split())), name
    symbol, literal = _symbol(row["kind"], row["signature"])
    assert not literal, name
    assert re.fullmatch(r"[A-Za-z_$][A-Za-z0-9_$]*", symbol), name
    assert row["callers"], name
    for caller in row["callers"]:
        path = ROOT / caller
        assert path.is_file(), caller
        assert re.search(
            rf"\b{re.escape(symbol)}\b", path.read_text(encoding="utf-8")
        ), (name, caller)


def test_inference_interfaces_reconcile_as_served():
    report = interfaces_reconcile(SPEC, ROOT, write=False)
    assert report["interfaces"]["rows"] >= len(INTERFACES)
    findings = {
        row["interface"]: row
        for section, row in report.items()
        if section.startswith("row.")
    }
    assert not (set(INTERFACES) & set(findings)), {
        name: findings[name] for name in INTERFACES if name in findings
    }


def test_protocol_names_every_served_f11_operation():
    row = INTERFACES["interface.22.inference-f11-operation-table"]
    assert row["status"] == "served"
    source = (ROOT / row["path"]).read_text(encoding="utf-8")
    operations = re.findall(
        r"^pub const (\w+): Operation = Operation\((0x0B[0-9A-F]{2})\);$",
        source,
        re.MULTILINE,
    )
    assert len(operations) == 17
    assert len({name for name, _ in operations}) == 17
    assert {int(selector, 16) for _, selector in operations} == (
        set(range(0x0B01, 0x0B0D)) | set(range(0x0B10, 0x0B15))
    )
    metadata = re.findall(r"OperationMetadata\s*\{([^{}]+)\}", source, re.DOTALL)
    served = []
    for entry in metadata:
        if re.search(r"\bfeature:\s*11\s*,", entry):
            operation = re.search(r"\boperation:\s*(\w+)\s*,", entry)
            name = re.search(r'\bname:\s*"(\w+)"\s*,', entry)
            assert operation is not None and name is not None
            assert operation[1] == name[1]
            served.append(name[1])
    assert sorted(served) == sorted(name for name, _ in operations)
    protocol = (ROOT / RECORD["protocol"]).read_text(encoding="utf-8")
    documented = re.findall(
        r"^\| `(0x0B[0-9A-F]{2})` \| (\w+) \|", protocol, re.MULTILINE
    )
    assert sorted(documented) == sorted(
        (selector, name) for name, selector in operations
    )


@pytest.mark.parametrize("task_id,path", CLIENT_TESTS.items())
def test_explorer_wallet_and_sdk_tests_are_present(task_id, path):
    assert task_id in RECORD["tasks"]
    assert path in TASKS[f"task.{task_id}"]["tests"]
    source = ROOT / path
    assert source.is_file(), path
    assert source.read_text(encoding="utf-8").strip(), path


def test_series_manifest_lists_qualified_tasks():
    tasks = RECORD["tasks"]
    assert tasks
    assert len(tasks) == len(set(tasks))
    assert tasks == sorted(tasks, key=lambda task: int(task.rsplit(".", 1)[1]))
    qualified = {
        name.removeprefix("task.")
        for name, row in TASKS.items()
        if name.startswith("task.22.38.")
        and name != "task.22.38.39"
        and row["status"] == "done"
    }
    assert set(tasks) == qualified
    for task_id in tasks:
        task = TASKS[f"task.{task_id}"]
        assert task["status"] == "done", task_id
        assert task["build_cmd"] and task["verify_cmd"], task_id


def test_contract_matches_the_served_registry():
    contract = (ROOT / RECORD["contract"]).read_text(encoding="utf-8")
    assert re.search(
        r"^\| Revision \| repository at [0-9a-f]{40} \|$", contract, re.MULTILINE
    )
    table = {}
    for line in contract.splitlines():
        if line.startswith("| interface."):
            cells = [cell.strip() for cell in line.strip("|").split("|")]
            name, kind, location, signature, task_id, status = cells
            assert name not in table, name
            table[name] = status
            row = REGISTRY[name]
            assert status == row["status"], name
            if name in INTERFACES:
                assert kind == row["kind"], name
                assert location == f'{row["path"]}:{row["line"]}', name
                assert signature == row["signature"], name
                assert task_id in RECORD["tasks"], name
                anchor = row["contract"].split("#")[1]
                assert f"\n## {anchor}\n" in contract, name
    assert len(table) == 43
    assert set(INTERFACES) <= set(table)
    for status in ("served", "new"):
        count = list(table.values()).count(status)
        assert f"| Interfaces with status {status} | {count} |" in contract
