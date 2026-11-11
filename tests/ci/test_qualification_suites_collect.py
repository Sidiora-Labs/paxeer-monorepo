"""Keep the complete qualification directory collectable at the current tree."""

import importlib.util
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tomllib
import xml.etree.ElementTree as ET

import pytest


ROOT = Path(__file__).resolve().parents[2]


@pytest.fixture(scope="session")
def qualification_evidence(tmp_path_factory):
    revision = subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=ROOT, text=True
    ).strip()
    result = subprocess.run(
        [sys.executable, "-m", "pytest", "-q", "tools/qualification"],
        cwd=ROOT,
        capture_output=True,
        text=True,
    )
    output = result.stdout + result.stderr
    log = tmp_path_factory.mktemp("qualification-evidence") / "full-suite.log"
    log.write_text(output)
    assert subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=ROOT, text=True
    ).strip() == revision, "Candidate changed while the full suite ran"
    evidence = {
        "revision": revision,
        "command": "python3 -m pytest -q tools/qualification",
        "exit": result.returncode,
        "log": str(log),
    }
    log.with_suffix(".json").write_text(json.dumps(evidence))
    return evidence


def test_qualification_suites_pass(qualification_evidence):
    output = Path(qualification_evidence["log"]).read_text()
    assert qualification_evidence["exit"] == 0, output
    assert re.search(r"\b[1-9][0-9]* passed\b", output), output


def test_repair_record_settles_every_baseline_entry(qualification_evidence):
    directory = ROOT / "tools/qualification"
    baseline = tomllib.loads((directory / "repair_22_31_621.kvx").read_text())
    record = tomllib.loads((directory / "repair_22_31_623.kvx").read_text())
    failures = baseline["failure"]
    settlements = record["case"]
    proof = record["evidence"]["full_suite"]
    assert proof["producer"] == "tests/ci/test_qualification_suites_collect.py:qualification_evidence"
    assert proof["command"] == qualification_evidence["command"]
    assert qualification_evidence["exit"] == 0, Path(qualification_evidence["log"]).read_text()
    assert re.fullmatch(r"[0-9a-f]{40}", qualification_evidence["revision"])
    assert Path(qualification_evidence["log"]).is_file()
    assert Path(qualification_evidence["log"]).stat().st_size > 0
    if os.environ.get("HF_GATE_INTERNAL") == "1":
        rows = [json.loads(line) for line in Path(record["repair"]["evidence_ledger"]).read_text().splitlines()]
        build = [row for row in rows if row["kind"] == "build"][-1]
        assert build["exit"] == 0
        assert build["revision"] == qualification_evidence["revision"]
        assert Path(build["log"]).is_file()
    assert len(failures) == baseline["continuation"]["baseline_case_entries"]
    assert record["repair"]["baseline_entries"] == len(failures)
    assert set(settlements) == set(failures)
    assert len({entry["case"] for entry in settlements.values()}) == len(failures)
    assert record["repair"]["baseline_revision"] == (
        baseline["qualification"]["verify-1"]["revision"]
    )
    grouped = []
    unresolved = []
    for family, groups in record["family"].items():
        for reason, group in groups.items():
            assert group["entries"], (family, reason)
            assert group["count"] == len(group["entries"]), (family, reason)
            for identifier in group["entries"]:
                entry = settlements[identifier]
                assert entry["module_family"] == family
                assert entry["reason_family"] == reason
                grouped.append(identifier)
    assert sorted(grouped) == sorted(failures)
    for identifier, original in failures.items():
        entry = settlements[identifier]
        for field in ("case", "module", "module_family", "reason", "outcome", "source"):
            assert entry[field] == original[field], (identifier, field)
        assert entry["settlement"].strip(), identifier
        assert entry["live_rule_source"].strip(), identifier
        assert (ROOT / entry["live_rule_source"].split(":", 1)[0]).is_file(), identifier
        if entry["status"] != "settled":
            unresolved.append(f"{entry['case']}: {entry['status']}: {entry['settlement']}")
            continue
        assert entry["verification_ref"] == "evidence.full_suite", identifier
    inventories = (
        (record["continuation_4"]["case"], 23),
        (record["continuation_4_repair"]["case"], 22),
        (record["continuation_5"]["case"], 5),
    )
    continuation = record["implementation"]["continuation_5"]
    assert continuation["remaining_cases"] == 4
    assert continuation["merged_prerequisite_cases"] == 1
    for entry in record["continuation_5"]["case"].values():
        assert re.fullmatch(r"[0-9a-f]{40}", entry["producer_commit"])
        assert subprocess.check_output(
            ["git", "cat-file", "-t", entry["producer_commit"]], cwd=ROOT, text=True
        ).strip() == "commit"
    assert record["implementation"]["continuation_4"]["remaining_cases"] == 23
    repair = record["continuation_4_repair"]
    assert repair["remaining_cases"] == 22
    observed = record["qualification"]["continuation4_verify1"]
    assert observed["revision"] == repair["observed_revision"]
    assert re.fullmatch(r"[0-9a-f]{40}", observed["revision"])
    assert observed["command"] == record["evidence"]["final_gate"]["command"]
    assert observed["exit"] == 1
    assert observed["log"] == "/out/logs/verify-1.log"
    grouped_repairs = []
    for reason, group in repair["family"].items():
        assert group["count"] == len(group["entries"])
        assert group["entries"]
        for identifier in group["entries"]:
            assert repair["case"][identifier]["reason_family"] == reason
            grouped_repairs.append(identifier)
    assert sorted(grouped_repairs) == sorted(repair["case"])
    for continuation, count in inventories:
        assert len(continuation) == count
        assert len({entry["case"] for entry in continuation.values()}) == count
        for identifier, entry in continuation.items():
            assert entry["settlement"].strip(), identifier
            assert (ROOT / entry["live_rule_source"]).is_file(), identifier
            assert entry["verification_ref"] == "evidence.full_suite", identifier
            assert (ROOT / entry["case"].split("::", 1)[0]).is_file(), identifier
            if entry["status"] != "settled":
                unresolved.append(f"{entry['case']}: {entry['status']}: {entry['settlement']}")
    current = record["continuation_6"]
    implementation = record["implementation"]["continuation_6"]
    before = record["local_run"]["before"]
    after = record["local_run"]["after"]
    current_cases = {entry["case"] for entry in current["case"].values()}
    assert len(current_cases) == len(current["case"]) == implementation["remaining_cases"]
    assert sorted(current_cases) == before["failed_cases"]
    assert before["failed"] == len(current_cases) and before["exit"] == 1
    assert after["exit"] == 0 and after["failed_cases"] == []
    assert after["skipped"] == before["skipped"]
    assert set(after["settled_cases"]) == current_cases
    for evidence in (before, after):
        assert re.fullmatch(r"[0-9a-f]{40}", evidence["revision"])
        assert Path(evidence["log"]).is_file()
    previous = {
        entry["case"]: entry
        for entry in record["continuation_5_repair"]["case"].values()
        if entry["status"] == "recorded-inherited"
    }
    assert len(previous) == implementation["previous_inherited_cases"]
    assert set(previous) <= current_cases
    assert sorted(current_cases - set(previous)) == implementation["additional_cases"]
    for case, entry in previous.items():
        identifier = entry["resolution_ref"].removeprefix("continuation_6.case.")
        assert current["case"][identifier]["case"] == case
        assert current["case"][identifier]["status"] == "settled"
    grouped_current = []
    producer_commits = set()
    for reason, group in current["family"].items():
        assert group["entries"] and len(group["entries"]) == group["count"]
        assert group["producer_commits"]
        producer_commits.update(group["producer_commits"])
        for producer in group["producers"]:
            assert (ROOT / producer).is_file()
        for identifier in group["entries"]:
            entry = current["case"][identifier]
            assert entry["reason_family"] == reason
            assert entry["producer_commits"] == group["producer_commits"]
            assert entry["observed_evidence_ref"] == "local_run.before"
            assert entry["verification_ref"] == "evidence.full_suite"
            assert entry["status"] == "settled" and entry["settlement"].strip()
            grouped_current.append(identifier)
    assert sorted(grouped_current) == sorted(current["case"])
    for revision in producer_commits:
        assert re.fullmatch(r"[0-9a-f]{40}", revision)
        assert subprocess.check_output(
            ["git", "cat-file", "-t", revision], cwd=ROOT, text=True
        ).strip() == "commit"
    module_cases = []
    for module in current["module"].values():
        assert (ROOT / module["file"]).is_file()
        names = [current["case"][key]["case"] for key in module["case_entries"]]
        assert names == module["before_failed_cases"] == module["after_passed_cases"]
        assert all(case.split("::", 1)[0] == module["file"] for case in names)
        module_cases.extend(names)
    assert sorted(module_cases) == sorted(current_cases)
    assert set(after["sources"]) == {module["file"] for module in current["module"].values()}
    for path, digest in after["sources"].items():
        assert hashlib.sha256((ROOT / path).read_bytes()).hexdigest() == digest
    report = Path(after["report"])
    assert hashlib.sha256(report.read_bytes()).hexdigest() == after["report_sha256"]
    actual_passes = {
        node.attrib["classname"].replace(".", "/") + ".py::" + node.attrib["name"]
        for node in ET.parse(report).iter("testcase")
        if not any(child.tag in {"failure", "error", "skipped"} for child in node)
    }
    assert current_cases <= actual_passes
    repair = record["continuation_6_gate_repair"]
    observed = record["qualification"]["continuation6_verify1"]
    final_local = record["local_run"]["after_gate_repair"]
    assert observed["exit"] == 1
    assert set(observed["failed_cases"]) == {entry["case"] for entry in repair["case"].values()}
    assert final_local["exit"] == 0 and final_local["failed_cases"] == []
    assert final_local["skipped"] == before["skipped"]
    report = Path(final_local["report"])
    assert hashlib.sha256(report.read_bytes()).hexdigest() == final_local["report_sha256"]
    actual_passes = {
        node.attrib["classname"].replace(".", "/") + ".py::" + node.attrib["name"]
        for node in ET.parse(report).iter("testcase")
        if not any(child.tag in {"failure", "error", "skipped"} for child in node)
    }
    assert current_cases <= actual_passes
    repaired_cases = set()
    for entry in repair["case"].values():
        assert entry["status"] == "settled" and entry["settlement"].strip()
        assert entry["verification_ref"] == "evidence.full_suite"
        assert (ROOT / entry["live_rule_source"]).is_file()
        assert entry["case"] in actual_passes
        repaired_cases.add(entry["case"])
    assert repaired_cases | current_cases == set(final_local["settled_cases"])
    assert set(final_local["sources"]) == {
        case.split("::", 1)[0] for case in repaired_cases | current_cases
    }
    for path, digest in final_local["sources"].items():
        assert hashlib.sha256((ROOT / path).read_bytes()).hexdigest() == digest
    assert not unresolved, "Unsettled baseline entries:\n" + "\n".join(unresolved)


def test_qualification_modules_collect():
    result = subprocess.run(
        [sys.executable, "-m", "pytest", "--collect-only", "-q", "tools/qualification"],
        cwd=ROOT,
        capture_output=True,
        text=True,
    )
    output = result.stdout + result.stderr
    assert result.returncode == 0, output
    assert re.search(r"\b[1-9][0-9]* tests? collected\b", output), output
    assert "ERROR collecting" not in output, output


def test_gateway_env_items_are_read_by_name():
    path = ROOT / "tools/qualification/audit_repairs/test_repair_22_33_1395.py"
    spec = importlib.util.spec_from_file_location("gateway_receipt_key_audit", path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)

    entries = module.CONTAINER["env"]
    assert len(module.ENV) == len(entries)
    assert set(module.ENV) == {item["name"] for item in entries}
    for item in entries:
        if "value" in item:
            assert module.ENV[item["name"]] == item["value"]
        else:
            assert module.ENV[item["name"]] in item["valueFrom"].values()
    for name, key in (
        ("LAYERX_GATEWAY_FAUCET_URL", "faucet-url"),
        ("LAYERX_GATEWAY_REDIS_URL", "redis-url"),
    ):
        assert module.ENV[name] == {
            "name": "layerx-gateway-upstreams",
            "key": key,
            "optional": False,
        }
