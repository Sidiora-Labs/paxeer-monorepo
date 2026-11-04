"""Offline source qualification for audit 22.30.79 finding 028.

Native Rust serialization and Node/TypeScript execution are review-only here;
the gate checks their real source wiring without emulating either implementation.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib


ROOT = Path(__file__).resolve().parents[3]
DASHBOARD = ROOT / "services/platform/hosted/dashboard"
WEB = DASHBOARD / "web"
RECORD = Path(__file__).with_name("repair_22_33_1411.kvx")


def read(path):
    return path.read_text(encoding="utf-8")


def require(source, *facts):
    for fact in facts:
        assert fact in source, fact


def rust_fields(source, name):
    match = re.search(r"pub struct " + name + r"\s*\{(.*?)\n\}", source, re.S)
    assert match, name
    return set(re.findall(r"^\s*(?:pub )?(\w+):", match[1], re.M))


def test_028_static_command_runs_both_suites_and_typecheck():
    manifest = json.loads(read(WEB / "package.json"))
    argv = shlex.split(manifest["scripts"]["test:static"])
    assert argv == [
        "node", "tests/ui-contract.mjs", "&&",
        "node", "tests/evidence-decoding.mjs", "&&", "tsc", "--noEmit",
    ]
    assert manifest["type"] == "module"
    assert manifest["engines"]["node"] == ">=24.0.0"
    for filename in (argv[1], argv[4]):
        assert (WEB / filename).is_file()


def test_028_existing_four_evidence_refusals_are_preserved():
    source = read(WEB / "tests/evidence-decoding.mjs")
    refusals = [
        'assert.throws(() => decodeRequests([{ at: 1, operation_digest: "operation", outcome: "completed", verification: "paxeer-finalised" }]), /request evidence/);',
        'assert.throws(() => decodeDeliveries([{ delivery: "delivery", endpoint: "endpoint", event: "event", state: { state: "delivered", status: 500 }, verification: "unverified", receipt_digest: null }]), /accepting status/);',
        'assert.throws(() => decodeReceipt({ activity_id: "a".repeat(64), event: "event", receipt_digest: "b".repeat(64), verification: "unverified", settled: true }), /not verified/);',
        'assert.throws(() => decodeReceipt({ activity_id: "a".repeat(64), event: "event", receipt_digest: "b".repeat(64), verification: "invented", settled: true }), /Unknown protocol verification/);',
    ]
    for refusal in refusals:
        assert source.count(refusal) == 1
    assert source.count("assert.throws(") == 4
    require(source, 'from "../app/data.ts"')
    assert not re.search(r"\b(?:skip|todo|only)\s*\(", source)


def test_028_positive_decode_loads_rust_fixture_and_asserts_projection():
    source = read(WEB / "tests/evidence-decoding.mjs")
    require(
        source,
        'import { test } from "node:test";',
        "async function rust_overview_decodes_without_inventing_evidence() {",
        'await readFile(new URL("./fixtures/rust-overview.json", import.meta.url), "utf8")',
        "const overview = decodeOverview(JSON.parse(serialized));",
        "assert.deepEqual(overview, {",
        "test(rust_overview_decodes_without_inventing_evidence);",
        'principal: "developer-fixture"',
        'outcome: "completed", verification: "unverified"',
        'amount: "10", asset: undefined',
        'settlement_verification: "unverified", receipt_digest: undefined, settled: false',
        'name: "state", value: "pending", verification: "unverified", receipt_digest: undefined',
        "requests_allowed: 10, requests_used: 1, requests_remaining: 9",
        "endpoints: [],", "dead_letters: [],",
    )
    assert source.index("decodeOverview(JSON.parse(serialized))") < source.index(
        "assert.deepEqual(overview, {"
    )


def test_028_fixture_covers_actual_rust_serialized_models():
    fixture = json.loads(read(WEB / "tests/fixtures/rust-overview.json"))
    models = read(DASHBOARD / "src/model.rs")
    events = read(DASHBOARD.parent / "webhooks/src/events.rs")
    for name, value in (
        ("Overview", fixture), ("UsageSummary", fixture["usage"]),
        ("KeyView", fixture["keys"][0]), ("RequestSummary", fixture["requests"]),
        ("RequestRecord", fixture["recent_requests"][0]),
        ("DeliverySummary", fixture["deliveries"]),
        ("PaymentView", fixture["payments"][0]),
    ):
        assert set(value) == rust_fields(models, name), name
        assert re.search(
            r"#\[derive\([^\n]*Serialize[^\n]*\)\]\s*pub struct " + name, models
        ), name
    payment = fixture["payments"][0]
    fact = payment["facts"][0]
    assert set(fact) == rust_fields(events, "ProtocolFact")
    assert fixture["principal"] == fixture["keys"][0]["principal"]
    assert fixture["usage"]["requests_remaining"] == 9
    assert fixture["keys"][0]["window_resets_at"] == 60
    assert fixture["recent_requests"][0]["operation"] is None
    assert fixture["recent_requests"][0]["verification"] == "unverified"
    assert payment["asset"] is None and payment["receipt_digest"] is None
    assert payment["settled"] is False
    assert payment["verification"] == payment["settlement_verification"] == "unverified"
    assert fact == {
        "name": "state", "value": "pending", "verification": "unverified",
        "receipt_digest": None,
    }
    assert fixture["endpoints"] == fixture["dead_letters"] == []
    assert fixture["deliveries"]["next_attempt_at"] is None
    assert all(value == 0 for key, value in fixture["deliveries"].items()
               if key != "next_attempt_at")


def test_028_producer_serializer_transport_and_consumer_sources_are_connected():
    producer = read(WEB / "tests/rust_overview_fixture.rs")
    require(
        producer,
        "use layerx_platform_dashboard::{", "Overview, PaymentView,",
        "#[test]", "fn rust_overview_serialization_matches_web_fixture()",
        "let overview = Overview {", "RequestOutcome::Completed",
        'ProtocolFact::unverified("state", "pending")?',
        "let serialized = serde_json::to_string(&overview)?;",
        "let emitted: serde_json::Value = serde_json::from_str(&serialized)?;",
        'serde_json::from_str(include_str!("fixtures/rust-overview.json"))?',
        "assert_eq!(emitted, fixture);",
    )
    require(
        read(DASHBOARD / "src/service.rs"),
        "pub fn overview(", "Ok(Overview {", ".map(PaymentView::of)",
        "recent_requests: gateway.recent_requests,", "payments,",
    )
    require(
        read(DASHBOARD / "src/main.rs"),
        '["v1", "dashboard", "overview"] => config',
        ".overview(principal, at)", ".map(serde_json::to_value)",
        "serde_json::to_string(value)", "|body| Reply::json(200, body)",
    )
    require(
        read(WEB / "app/page.tsx"),
        'fetch(`/v1/dashboard${path}`, { credentials: "include", cache: "no-store", signal })',
        "return decode(await response.json());", 'read("/overview", decodeOverview, signal)',
        "setOverview(summary);", "<OverviewPanel value={overview} />",
    )
    require(
        read(WEB / "app/data.ts"),
        "export function decodeOverview(value: unknown): Overview",
        "recent_requests: decodeRequests(row.recent_requests)",
        "dead_letters: decodeDeliveries(row.dead_letters)",
        "payments: array(row.payments, payment)",
        'function optional(value: unknown): string | undefined { return value == null ? undefined : text(value); }',
        'throw new Error("Unsupported settlement")',
    )


def test_028_record_maps_tests_limitations_and_private_revision_evidence():
    record = tomllib.loads(read(RECORD))
    task = record["task"]["22"]["33"]["1411"]
    finding = record["finding"]["028"]
    assert finding["audit"] == "22.30.79"
    assert finding["file"] == "services/platform/hosted/dashboard/web/package.json"
    assert finding["class"] == "test-gap" and finding["severity"] == "medium"
    names = {
        node.name for node in ast.parse(read(Path(__file__))).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == names - {
        "test_028_record_maps_tests_limitations_and_private_revision_evidence",
    }
    assert task["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1411.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    require(task["evidence"], "same candidate revision", "command, exit code and log path")
    require(task["review_only"], "not executed", "not a recorded production response")
    require(task["baseline"], "No pre-change verification", "No baseline failures are claimed")
    assert len(task["wiring_requests"]) == 1
    require(task["wiring_requests"][0], "services/platform/hosted/dashboard/Cargo.toml",
            "domain 8", "web/tests/rust_overview_fixture.rs")
    require(record["pre_existing"]["verification_vocabulary"]["observed"],
            "block-committed", "checkpoint-finalised", "paxeer-finalised")
