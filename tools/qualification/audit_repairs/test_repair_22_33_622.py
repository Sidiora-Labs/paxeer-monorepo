"""Offline source qualification for audit 22.30.163/033.

The bounded environment has no JVM toolchain. These checks inspect the real
Human producer, Android transport/resolver/gate, Java regression and committed
receipt bytes; they do not execute or emulate those production components.
"""

import ast
import base64
import hashlib
import json
from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
ANDROID = "sdk/integrations/android/src/main/java/com/sidiora/layerx/android/"
TEST_PATH = "sdk/integrations/android/src/test/java/com/sidiora/layerx/android/ReceiptGateTest.java"
FIXTURE_PATH = "sdk/integrations/android/src/test/java/com/sidiora/layerx/android/human-receipt-journey-22-33-622.json"
PRODUCER_PATH = "services/human/crates/layerx-human-service/src/server/production_reads.rs"
VECTOR_PATH = "sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json"
RECORD_PATH = "tools/qualification/audit_repairs/repair_22_33_622.kvx"


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def java_method(text, name):
    """Extract a declaration body with balanced braces, ignoring comments/strings."""
    token = re.compile(r'//[^\n]*|/\*.*?\*/|"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\'', re.S)
    masked = token.sub(lambda match: " " * len(match[0]), text)
    declaration = re.search(
        r"\b(?:public|private|protected)?\s*(?:static\s+)?(?:synchronized\s+)?"
        r"[\w<>?,.\[\] ]+\s+" + re.escape(name) + r"\s*\([^)]*\)\s*(?:throws\s+[\w, ]+)?\s*\{",
        masked,
    )
    assert declaration, f"missing declared Java method {name}"
    opening = masked.index("{", declaration.start())
    depth = 1
    for index in range(opening + 1, len(masked)):
        depth += (masked[index] == "{") - (masked[index] == "}")
        if depth == 0:
            return text[opening + 1:index]
    raise AssertionError(f"unclosed Java method {name}")


def require(text, *facts):
    for fact in facts:
        assert fact in text, f"missing source fact: {fact}"


def fixture_and_vector():
    fixture = json.loads(source(FIXTURE_PATH))
    vector = next(case for case in json.loads(source(VECTOR_PATH))["cases"] if case["id"] == "V1-pax-transfer")
    return fixture, vector


def test_033_current_producer_completion_and_evidence_contract():
    fixture, vector = fixture_and_vector()
    producer = source(PRODUCER_PATH)
    native = producer.split("fn native_journey_json(", 1)[1].split("/// Labels one custody", 1)[0]
    require(native, '"journey_id": status.journey_id().as_str()', '"state": state',
            '"stages": stages', '"evidence": evidence', "journey.verified_leg_evidence".replace(".", "\n            .", 1),
            "evidence_ref(digest, verification)", '"stage_id": format!("stg_{}_{index}"',
            '"copy_key": format!("journey.stage.{}", phase_label(*phase))')
    require(producer, 'JourneyState::Done => "done"', 'JourneyPhase::ReceiptVerified => "done"',
            'JourneyPhase::ReceiptVerified => "receipt-verified"',
            '"evidence_id": format!("evd_{}", hex(&digest))',
            '"class": "layerx-receipt"', "PortableReceiptV2::export_with_transaction(",
            '"content_type": "application/vnd.layerx.receipt-proof-v2+json"',
            '"bytes_base64": STANDARD.encode(bytes)', '"inclusion_height": inclusion_height')
    schema_source = source("services/human/schema/human-api/journeys.kvx")
    state_section = re.search(r"^\[type\.JourneyState\]\n(.*?)(?=^\[|\Z)", schema_source, re.M | re.S)
    assert state_section is not None
    state_schema = tomllib.loads(state_section[1])
    assert {"done", "done-finalised"} <= set(state_schema["variants"])
    assert "native journey projections render done" in state_schema["done-finalised"]["semantics"]
    journey = fixture["journey"]["result"]
    assert fixture["journey"]["ok"] is True
    assert journey["kind"] == "move" and journey["state"] == "done"
    assert set(journey) == {"journey_id", "kind", "state", "state_copy_key", "stages", "evidence", "started_at", "updated_at"}
    assert journey["state_copy_key"] == "journey.state.done"
    assert "receipt_ref" not in journey
    assert len(journey["stages"]) == len(journey["evidence"]) == 1
    stage = journey["stages"][0]
    assert stage == {"stage_id": f"stg_{journey['journey_id']}_0", "copy_key": "journey.stage.receipt-verified",
                     "state": "done", "evidence": journey["evidence"]}
    signed = bytes.fromhex(vector["signedReceipt"][2:])
    reference = {"evidence_id": "evd_" + hashlib.sha256(signed).hexdigest(),
                 "class": "layerx-receipt", "verification": "receipt-verified"}
    assert journey["evidence"] == [reference]
    assert "not a captured Human response" in fixture["provenance"]["kind"]
    assert fixture["provenance"]["receipt_source"] == VECTOR_PATH


def test_033_portable_material_is_bound_to_committed_producer_bytes():
    fixture, vector = fixture_and_vector()
    material = fixture["evidence"]["result"]
    assert fixture["evidence"]["ok"] is True
    assert material["evidence_id"] == fixture["journey"]["result"]["evidence"][0]["evidence_id"]
    assert material["class"] == "layerx-receipt"
    assert material["verification"] == "receipt-verified" and material["rung"] == "receipted"
    assert material["inclusion_height"] == int(vector["receiptJson"]["height"], 16)
    assert material["content_type"] == "application/vnd.layerx.receipt-proof-v2+json"
    encoded = base64.b64decode(material["bytes_base64"], validate=True)
    assert base64.b64encode(encoded).decode() == material["bytes_base64"]
    proof = json.loads(encoded)
    assert set(proof) == {"format", "verificationLevel", "signedReceipt", "receiptDigest", "receiptKey", "inclusionHeight", "rawTransaction"}
    assert proof["format"] == "layerx-receipt-proof-v2" and proof["verificationLevel"] == "receipted"
    for field in ("signedReceipt", "receiptDigest", "rawTransaction"):
        assert proof[field] == vector[field]
    assert proof["signedReceipt"] == vector["receiptJson"]["receipt"]
    expected_key = {key: hex(value) if isinstance(value, int) else value for key, value in vector["pinnedKey"].items()}
    assert proof["receiptKey"] == expected_key
    assert proof["inclusionHeight"] == vector["receiptJson"]["height"]
    signed = bytes.fromhex(proof["signedReceipt"][2:])
    assert signed[-69:-64] == b"\x01\x00\x00\x00\x40"
    unsigned = signed[:-69] + b"\x00"
    assert "0x" + hashlib.sha256(b"LXP/v1/receipt\x00" + unsigned).hexdigest() == proof["receiptDigest"]


def test_033_java_regression_uses_http_client_resolution_and_configured_gate():
    test = source(TEST_PATH)
    original = java_method(test, "verifiesSignedSequencerReceipt")
    require(original, "new HumanPeer(humanFixture())", "LayerXAndroid.create(human.configuration(vector)",
            "gate = android.gate();", "gate.track(human.journeyId(), expectation())",
            'List.of("POST /session", "GET /v1/journeys/" + human.journeyId()',
            '"GET /v1/evidence/" + human.evidenceId()', "human.requests()",
            'assertEquals("receipted", verified.level())', "verified.receiptDigest()")
    assert "receipt_ref" not in test and 'state\\\":\\\"settled' not in test
    assert "android.gate(reference ->" not in original
    peer = java_method(test, "respond")
    require(test, 'HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0)',
            'server.createContext("/", this::respond)', "server.start()", "server.stop(0)")
    require(peer, 'request.equals("POST /session")', 'put("audience", "layerx-human-api")',
            '"Bearer android-receipt-test-session".equals(', 'getFirst("Authorization")',
            'fixture.required("journey")', 'fixture.required("evidence")',
            "exchange.sendResponseHeaders(404, -1)", "exchange.getResponseBody().write(body)")
    require(java_method(test, "close"), "server.stop(0)", "assertEquals(List.of(), violations)")
    entry = java_method(source(ANDROID + "LayerXAndroid.java"), "gate")
    require(entry, "new ReceiptGate(client, configuration.receiptKeys().orElseThrow(")
    mobile = source(ANDROID + "MobileClient.java")
    require(java_method(mobile, "journey"), 'client.human("journey.get"', 'Map.of("journey_id", value)')
    require(java_method(mobile, "evidence"), 'client.human("evidence.get"', 'Map.of("evidence_id", value)')
    transport = source(ANDROID + "AndroidHttpTransport.java")
    require(java_method(transport, "exchange"), "sessions.token().authorize(connection)",
            "connection.getResponseCode()", "return decode(status, encoded, responseType)")
    require(java_method(transport, "decode"), 'envelope.path("ok").isBoolean()',
            'mapper.convertValue(envelope.get("result"), type)')
    gate = source(ANDROID + "ReceiptGate.java")
    require(java_method(gate, "track"), "client.journey(journeyId)", "projectAsync(journey, expectation)")
    require(java_method(gate, "resolve"), "client.evidence(reference.id())", "HumanReceiptResolver.verify(",
            "payment.asset()", "payment.recipient()", "payment.amount()", 'new Verified("receipted", payment.digest())')


def test_033_tamper_and_pin_assertions_remain_and_cover_transport():
    test = source(TEST_PATH)
    original = java_method(test, "verifiesSignedSequencerReceipt")
    require(original, "exposedTransaction[0] ^= 1", "exposedReceipt[0] ^= 1",
            'assertArrayEquals(bytes(vector, "rawTransaction"), evidence.rawTransaction())',
            'assertArrayEquals(bytes(vector, "signedReceipt"), evidence.signedReceipt())',
            "tamperedReceipt[tamperedReceipt.length - 1] ^= 1",
            "assertRefused(gate, new ReceiptGate.Evidence(evidence.rawTransaction(), tamperedReceipt, pin))",
            "otherTransaction[otherTransaction.length - 1] ^= 1",
            "assertRefused(gate, new ReceiptGate.Evidence(otherTransaction, evidence.signedReceipt(), pin))",
            "otherAsset[0] ^= 1", "otherRecipient[0] ^= 1", "expected.amount().value().add(BigInteger.ONE)",
            "MobileIntegrationException.Code.VERIFICATION_FAILURE, error.code()")
    pins = java_method(test, "refusesReceiptUnderUnpinnedKey")
    require(pins, "height, height", "height.add(BigInteger.ONE)", "height.subtract(BigInteger.ONE)",
            "otherId[0] ^= 1", "otherPublicKey[0] ^= 1", "assertRefused(gate(claimed, unpinned), claimed)",
            "assertRefused(gate(evidence, pin), claimed)", "withoutReceipts.receiptKeys().isEmpty()",
            "MobileIntegrationException.Code.INVALID_CONFIGURATION, error.code()")
    regression = java_method(test, "completedHumanJourneyRequiresMatchingPinnedEvidence")
    require(regression, 'List.of("done", "done-finalised")', 'put("verification", "checkpoint-finalised")',
            'put("rung", "committed")', 'List.of("signature", "transaction", "digest", "evidence-id", "public-key")',
            'MessageDigest.getInstance("SHA-256")', 'material.put("evidence_id", id)',
            'journey.required("evidence").get(0)).put("evidence_id", id)',
            "assertHumanRefused(response, vector, expectation())", "assertHumanRefused(fixture, otherPin, expectation())",
            "assertHumanRefused(fixture, vector, mismatch)", "otherAsset[0] ^= 1", "otherRecipient[0] ^= 1",
            "expected.amount().value().add(BigInteger.ONE)")
    require(java_method(test, "assertHumanRefused"), "assertThrows(CompletionException.class",
            "android.gate().track(human.journeyId(), expectation)",
            "assertInstanceOf(MobileIntegrationException.class, error.getCause())",
            "MobileIntegrationException.Code.VERIFICATION_FAILURE, cause.code()", "assertEquals(3, human.requests().size())")
    assert len(re.findall(r"@Test\s+void\s+", test)) == 3
    assert not re.search(r"@Disabled|@Ignore|Assumptions\.|assumeTrue|assumeFalse", test)
    resolver = source(ANDROID + "HumanReceiptResolver.java")
    require(resolver, "id.equals(ReceiptGate.evidenceId(signed))", "require(verifier.verify(signature))",
            "height.compareTo(pin.firstHeight()) >= 0", "height.compareTo(pin.lastHeight()) <= 0",
            'hex(key, "publicKey", 32)', 'hex(proof, "receiptDigest", 32)',
            "MessageDigest.isEqual(activity, transactionHash)")


def test_033_record_discloses_proof_limits_and_names_every_test():
    record = tomllib.loads(source(RECORD_PATH))
    finding = record["finding"]["033"]
    assert finding["audit"] == "22.30.163"
    assert finding["file"] == TEST_PATH
    assert finding["class"] == "test-gap" and finding["severity"] == "medium"
    functions = {node.name for node in ast.parse(source("tools/qualification/audit_repairs/test_repair_22_33_622.py")).body
                 if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    assert set(finding["tests"]) == functions
    task = record["task"]["22"]["33"]["622"]
    assert task["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_622.py"
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert "not a captured Human response" in task["review_only"]
    assert "final hf-done JSON" in task["verified_revision"]
    assert PRODUCER_PATH in record["wiring"]["human_completion_transcript"]["request"]
    assert "domain 15" in record["wiring"]["human_completion_transcript"]["request"]
