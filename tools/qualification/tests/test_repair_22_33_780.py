"""Task 22.33.780 repair record.

Finding release-split-network is proved by test_merged_release_evidence_round_trip,
test_retired_release_evidence_is_refused and test_required_merged_cases_cannot_be_omitted.
The production evidence producer, HTTP client, JSON codec, file transport and
consumer run against an in-process recorded-evidence store. These tests qualify
the release evidence contract; they do not claim to run paxd consensus itself.
Wiring request: tools/qualification/beta_driver.py (domain 16): migrate Stack/STACK_FLAGS/CLI to the sole node endpoint.
Wiring request: tools/qualification/test_release_runner.py and tools/qualification/test_beta_driver.py (domain 16): migrate split-stack fixtures.
Wiring request: deployment-managed $LAYERX_QUALIFICATION_DRIVER (domain 16): emit the merged journey, fault and performance inventories below.
Baseline: not run; the explicit gate order forbids testing before implementation
and running verify_cmd by hand. No pre-change failure is claimed.
Evidence: the final hf-done JSON records the exact candidate revision and private
log; the runner publishes it. This record is written before deriving that revision.
Gotcha: spec/ and GOTCHA.kvx are outside scope; record task notes here instead.
No Python formatter is configured for these paths; follow their existing style.
"""

from __future__ import annotations

import hashlib
import json
import shutil
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread

import pytest

from tools.qualification import beta_driver
from tools.qualification.release_runner import (
    EVIDENCE_SPECS,
    FAULT_STAGES,
    SCHEMA,
    URL_ENVIRONMENTS,
    QualificationFailure,
    atomic_json,
    file_digest,
    qualification_components,
    validate_evidence,
)


SOURCE_IDENTITY = hashlib.sha256(b"recorded merged release source").hexdigest()
DRIVER_DIGEST = file_digest(Path(beta_driver.__file__))
RECORDED_JOURNEYS = (
    "onboarding", "wallet-binding", "deposit", "move-money", "create-agent",
    "approval-grant", "approval-reject", "withdrawal", "node-consensus",
    "receipt-verification", "node-restart",
)
RECORDED_FAULT_STAGES = {
    "deposit": (
        "wallet-signed-before-registration", "registered", "custody-submitted",
        "custody-acknowledged", "finality-observed", "proof-built", "credit-prepared",
        "credit-submitted", "credit-block-committed", "receipt-published",
        "receipt-verified", "acknowledged",
    ),
    "move-money": (
        "registered", "leg-prepared", "leg-submitted", "leg-tracked",
        "block-committed", "receipt-published", "receipt-verified", "acknowledged",
    ),
    "withdrawal": (
        "registered", "debit-prepared", "debit-submitted", "debit-block-committed",
        "debit-receipt-published", "debit-receipt-verified", "payout-submitted",
        "payout-receipt-published", "payout-verified", "acknowledged",
    ),
    "node-consensus": (
        "proposal-received", "vote-persisted", "block-committed", "receipt-published",
        "receipt-verified", "acknowledged",
    ),
    "node-restart": (
        "uncommitted-proposal-discarded", "committed-block-replayed", "state-recovered",
        "receipt-recovered", "duplicate-submission-refused", "acknowledged",
    ),
}
RECORDED_CASES = {
    "human-qualify-journeys": frozenset(
        f"{shell}/{journey}"
        for shell in ("mobile", "desktop")
        for journey in RECORDED_JOURNEYS
    ),
    "human-qualify-faults": frozenset(
        f"{journey}/{stage}/{mode}"
        for journey, stages in RECORDED_FAULT_STAGES.items()
        for stage in stages
        for mode in ("crash", "disconnect", "duplicate-delivery")
    ),
    "human-qualify-perf": frozenset((
        "mobile/performance-budget", "desktop/performance-budget",
        "soak/deposit-awaiting-finality", "soak/withdrawal-awaiting-commit",
        "soak/node-restart-recovery", "soak/unactioned-approval",
        "soak/store-deletion-rebuild",
    )),
}
RECORDED_KINDS = {
    "human-qualify-journeys": frozenset(("receipt", "proof", "timing")),
    "human-qualify-faults": frozenset(("injection", "result", "ledger-proof")),
    "human-qualify-perf": frozenset(("metrics", "resource-curve", "latency-curve")),
}


@pytest.fixture(scope="module")
def transported_evidence(tmp_path_factory):
    producer_root = tmp_path_factory.mktemp("producer")
    consumer_root = tmp_path_factory.mktemp("consumer")
    server = ThreadingHTTPServer(
        ("127.0.0.1", 0), partial(SimpleHTTPRequestHandler, directory=str(producer_root))
    )
    endpoint = f"http://127.0.0.1:{server.server_port}"
    environment = {
        "LAYERX_QUALIFICATION_NODE_URL": endpoint + "/node",
        "LAYERX_QUALIFICATION_AGENT_URL": endpoint + "/agentd",
        "LAYERX_QUALIFICATION_HUMAN_URL": endpoint + "/human",
    }
    components = qualification_components(environment)
    # Only the existing driver's transport/codec is used. Its retired fourth
    # CLI endpoint is deliberately not used to produce the merged header.
    stack = beta_driver.Stack(
        components["node"], components["agentd"], components["human_service"],
        components["node"],
    )
    for gate, cases in RECORDED_CASES.items():
        records = []
        root = producer_root / gate
        for case_id in sorted(cases):
            context = beta_driver.CaseContext(
                gate, case_id, root, stack, SOURCE_IDENTITY, {}
            )
            for kind in sorted(RECORDED_KINDS[gate]):
                context.artifact_json(kind, kind + ".json", {
                    "case": case_id, "gate": gate, "kind": kind,
                    "source_identity": SOURCE_IDENTITY,
                })
            records.append({
                "id": case_id, "status": "passed", "artifacts": context.artifacts,
            })
        atomic_json(root / "evidence.json", {
            "schema": SCHEMA, "gate": gate, "real_stack": True,
            "source_identity": SOURCE_IDENTITY, "driver_sha256": DRIVER_DIGEST,
            "components": components, "cases": records,
        })

    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    client = beta_driver.CaseContext(
        "human-qualify-faults", "transport", consumer_root,
        stack, SOURCE_IDENTITY, {},
    )
    try:
        for gate in RECORDED_CASES:
            manifest = client.http_json(endpoint + f"/{gate}/evidence.json")
            root = consumer_root / gate
            for case in manifest["cases"]:
                for artifact in case["artifacts"]:
                    relative = artifact["path"]
                    status, content = client.http(endpoint + f"/{gate}/{relative}")
                    assert status == 200
                    destination = root / relative
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    destination.write_bytes(content)
                    assert file_digest(destination) == artifact["sha256"]
            atomic_json(root / "evidence.json", manifest)
        yield consumer_root, components
    finally:
        server.shutdown()
        thread.join(timeout=5)
        server.server_close()
        assert not thread.is_alive()


def read_evidence(root, gate, components):
    return validate_evidence(
        root / "evidence.json", root, gate, components, SOURCE_IDENTITY, DRIVER_DIGEST
    )


@pytest.mark.parametrize("gate", tuple(RECORDED_CASES))
def test_merged_release_evidence_round_trip(transported_evidence, gate):
    root, components = transported_evidence
    evidence = read_evidence(root / gate, gate, components)
    assert set(URL_ENVIRONMENTS) == {"node", "agentd", "human_service"}
    assert evidence["components"] == components
    assert {case["id"] for case in evidence["cases"]} == RECORDED_CASES[gate]
    assert EVIDENCE_SPECS[gate].artifact_kinds == RECORDED_KINDS[gate]
    assert FAULT_STAGES == RECORDED_FAULT_STAGES
    # Reopen the persisted evidence after all producer handles have closed.
    assert read_evidence(root / gate, gate, components) == evidence


@pytest.mark.parametrize("gate,required", (
    ("human-qualify-journeys", "mobile/node-consensus"),
    ("human-qualify-journeys", "desktop/receipt-verification"),
    ("human-qualify-journeys", "mobile/node-restart"),
    ("human-qualify-faults", "node-consensus/block-committed/crash"),
    ("human-qualify-faults", "node-consensus/receipt-verified/disconnect"),
    ("human-qualify-faults", "node-restart/receipt-recovered/crash"),
    ("human-qualify-faults", "node-restart/duplicate-submission-refused/duplicate-delivery"),
    ("human-qualify-faults", "deposit/custody-acknowledged/crash"),
    ("human-qualify-faults", "withdrawal/payout-verified/disconnect"),
    ("human-qualify-perf", "soak/withdrawal-awaiting-commit"),
    ("human-qualify-perf", "soak/node-restart-recovery"),
))
def test_required_merged_cases_cannot_be_omitted(transported_evidence, tmp_path, gate, required):
    store, components = transported_evidence
    root = tmp_path / gate
    shutil.copytree(store / gate, root)
    manifest = json.loads((root / "evidence.json").read_text())
    assert required in {case["id"] for case in manifest["cases"]}
    manifest["cases"] = [case for case in manifest["cases"] if case["id"] != required]
    atomic_json(root / "evidence.json", manifest)
    with pytest.raises(QualificationFailure, match="case inventory mismatch"):
        read_evidence(root, gate, components)


@pytest.mark.parametrize("gate,old_case", (
    ("human-qualify-journeys", "mobile/emergency-exit"),
    ("human-qualify-journeys", "desktop/withdrawal-claim"),
    ("human-qualify-faults", "withdrawal/checkpoint-observed/crash"),
    ("human-qualify-faults", "withdrawal-claim/challenge-window-observed/disconnect"),
    ("human-qualify-faults", "emergency-exit/checkpoint-read/duplicate-delivery"),
    ("human-qualify-perf", "soak/withdrawal-awaiting-checkpoint"),
))
def test_retired_release_evidence_is_refused(transported_evidence, tmp_path, gate, old_case):
    store, components = transported_evidence
    root = tmp_path / gate
    shutil.copytree(store / gate, root)
    manifest = json.loads((root / "evidence.json").read_text())
    assert old_case not in {case["id"] for case in manifest["cases"]}
    manifest["cases"][0]["id"] = old_case
    atomic_json(root / "evidence.json", manifest)
    with pytest.raises(QualificationFailure, match="case inventory mismatch"):
        read_evidence(root, gate, components)


def test_separate_settlement_endpoint_cannot_be_qualified(transported_evidence, tmp_path):
    store, components = transported_evidence
    gate = "human-qualify-faults"
    root = tmp_path / gate
    shutil.copytree(store / gate, root)
    split = dict(components, paxeer_testnet="https://settlement.example.test")
    manifest = json.loads((root / "evidence.json").read_text())
    manifest["components"] = split
    atomic_json(root / "evidence.json", manifest)
    with pytest.raises(QualificationFailure, match="exact real-stack endpoints"):
        read_evidence(root, gate, components)
    with pytest.raises(QualificationFailure, match="one merged node"):
        read_evidence(root, gate, split)


@pytest.mark.parametrize("damage", ("digest", "missing", "failed", "header", "endpoint"))
def test_merged_evidence_preserves_safety_checks(transported_evidence, tmp_path, damage):
    store, components = transported_evidence
    gate = "human-qualify-faults"
    root = tmp_path / gate
    shutil.copytree(store / gate, root)
    manifest = json.loads((root / "evidence.json").read_text())
    case = manifest["cases"][0]
    artifact = case["artifacts"][0]
    if damage == "digest":
        (root / artifact["path"]).write_bytes(b"changed after transport")
        expected = "artifact digest does not match"
    elif damage == "missing":
        case["artifacts"].pop(0)
        expected = "missing artifacts"
    elif damage == "failed":
        case["status"] = "failed"
        expected = "is not passed"
    elif damage == "header":
        manifest["source_identity"] = "unqualified revision"
        expected = "source_identity.*does not match"
    else:
        manifest["components"]["node"] += "/different-node"
        expected = "exact real-stack endpoints"
    atomic_json(root / "evidence.json", manifest)
    with pytest.raises(QualificationFailure, match=expected):
        read_evidence(root, gate, components)


def test_merged_endpoint_configuration_preserves_url_guards():
    environment = {
        "LAYERX_QUALIFICATION_NODE_URL": "https://node.example.test",
        "LAYERX_QUALIFICATION_AGENT_URL": "https://agent.example.test",
        "LAYERX_QUALIFICATION_HUMAN_URL": "https://human.example.test",
    }
    expected = {
        "node": environment["LAYERX_QUALIFICATION_NODE_URL"],
        "agentd": environment["LAYERX_QUALIFICATION_AGENT_URL"],
        "human_service": environment["LAYERX_QUALIFICATION_HUMAN_URL"],
    }
    assert qualification_components(environment) == expected
    assert qualification_components(dict(
        environment, LAYERX_QUALIFICATION_PAXEER_URL="https://retired.example.test"
    )) == expected
    for variable in environment:
        missing = dict(environment)
        del missing[variable]
        with pytest.raises(QualificationFailure, match="absolute HTTP or HTTPS URL"):
            qualification_components(missing)
    for invalid, message in (
        ("", "absolute HTTP or HTTPS URL"),
        ("file:///node", "absolute HTTP or HTTPS URL"),
        ("https://user:password@node.example.test", "must not contain credentials"),
        ("https://node.example.test?query=1", "must not contain a query"),
        ("https://node.example.test#fragment", "must not contain a query or fragment"),
        (expected["agentd"], "must be distinct URLs"),
    ):
        with pytest.raises(QualificationFailure, match=message):
            qualification_components(dict(environment, LAYERX_QUALIFICATION_NODE_URL=invalid))


def test_merged_commit_and_recovery_order_preserve_custody():
    for journey, committed, published, verified in (
        ("deposit", "credit-block-committed", "receipt-published", "receipt-verified"),
        ("move-money", "block-committed", "receipt-published", "receipt-verified"),
        ("withdrawal", "debit-block-committed", "debit-receipt-published", "debit-receipt-verified"),
        ("node-consensus", "block-committed", "receipt-published", "receipt-verified"),
    ):
        stages = FAULT_STAGES[journey]
        assert stages.index(committed) < stages.index(published) < stages.index(verified)
        assert stages.index(verified) < stages.index("acknowledged")
    deposit = FAULT_STAGES["deposit"]
    assert deposit.index("custody-acknowledged") < deposit.index("credit-block-committed")
    withdrawal = FAULT_STAGES["withdrawal"]
    assert withdrawal.index("debit-receipt-verified") < withdrawal.index("payout-submitted")
    assert withdrawal.index("payout-verified") < withdrawal.index("acknowledged")
    restart = FAULT_STAGES["node-restart"]
    assert restart.index("committed-block-replayed") < restart.index("receipt-recovered")
    assert restart.index("state-recovered") < restart.index("receipt-recovered")
    assert restart.index("duplicate-submission-refused") < restart.index("acknowledged")
