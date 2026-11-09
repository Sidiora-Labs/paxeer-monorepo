"""Finding 031: check both declared interface ends and Android's real source path.

These are source-contract proofs. They do not execute a JVM or substitute a
Python implementation for Android transport, receipt verification, or tracking.
The execution gap is recorded in repair_22_31_418.kvx.
"""

import hashlib
import json
from pathlib import Path
import re

import pytest


ROOT = Path(__file__).resolve().parents[3]
ANDROID = ROOT / "sdk/integrations/android/src/main/java/com/sidiora/layerx/android"
JVM = ROOT / "sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk"
HUMAN = ROOT / "services/human/crates/layerx-human-service/src/server"
SCHEMA = ROOT / "services/human/schema/human-api/journeys.kvx"


def source(name):
    return (ANDROID / name).read_text()


def compact(value):
    return re.sub(r"\s+", " ", value).strip()


def method(text, declaration):
    """Extract balanced Java/Rust source, ignoring strings and comments."""
    start = text.index(declaration)
    opening = text.index("{", start)
    tokens = re.compile(r'"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\'|//[^\n]*|/\*.*?\*/|[{}]', re.S)
    depth = 0
    for match in tokens.finditer(text, opening):
        if match.group() == "{":
            depth += 1
        elif match.group() == "}":
            depth -= 1
            if depth == 0:
                return text[opening + 1 : match.start()]
    raise AssertionError(f"Unbalanced method: {declaration}")


def section(path, name):
    text = path.read_text()
    match = re.search(r"(?ms)^\[" + re.escape(name) + r"\]\n(.*?)(?=^\[|\Z)", text)
    assert match is not None, name
    return {
        key: json.loads(value)
        for key, value in re.findall(r"(?m)^([\w.-]+)\s*=\s*(.+)$", match[1])
    }


def contains(body, *facts):
    normalized = compact(body)
    for fact in facts:
        assert compact(fact) in normalized, fact


def test_finding_031_normative_states_and_evidence_selection():
    gate = source("ReceiptGate.java")
    states = method(gate, "private static State localState(")
    selected = set()
    for case in re.findall(r"case (.*?) ->", states):
        selected.update(re.findall(r'"([a-z-]+)"', case))
    assert selected == set(section(SCHEMA, "type.JourneyState")["variants"])
    contains(
        states,
        'case "done", "done-finalised" -> null;',
        'case "refused" -> new Refused(state);',
        'case "closed" -> new Refused(state);',
        '"awaiting-approval" ->',
        'new Pending(journey.path("journey_id").textValue());',
        'default -> throw MobileIntegrationException.of(MobileIntegrationException.Code.DECODE_FAILURE);',
        '!journey.path("state").isTextual()',
        '!journey.path("journey_id").isTextual()',
    )
    assert "receipt_ref" not in gate
    assert '"settled"' not in gate and '"completed"' not in gate
    contains(
        method(gate, "private static List<Reference> references("),
        '!journey.path("evidence").isArray() || !journey.path("stages").isArray()',
        'collect(journey.path("evidence"), references, seen);',
        'for (JsonNode stage : journey.path("stages"))',
        'collect(stage.path("evidence"), references, seen);',
    )
    contains(
        method(gate, "private static void collect("),
        '!reference.path("evidence_id").isTextual()',
        '!reference.path("class").isTextual()',
        '!reference.path("verification").isTextual()',
        'id.matches("evd_[0-9a-f]{64}")',
        '"layerx-receipt".equals(reference.path("class").textValue())',
        'verification.equals("receipt-verified") || verification.equals("checkpoint-finalised")',
        '&& seen.add(id)',
        'references.add(new Reference(id, verification));',
    )


@pytest.mark.parametrize(
    "operation,method_name,parameter",
    [("journey.get", "journey", "journey_id"), ("evidence.get", "evidence", "evidence_id")],
)
def test_finding_031_both_route_contracts_and_authenticated_transport(operation, method_name, parameter):
    producer = section(SCHEMA, f"operation.{operation}")
    assert producer["method"] == "GET"
    consumer_routes = (JVM / "GeneratedContract.java").read_text()
    route = re.search(
        r'Map.entry\("' + re.escape(operation) + r'", new OperationCatalog.Route\("(\w+)", "([^"]+)", List.of\("(\w+)"\), false, true\)\)',
        consumer_routes,
    )
    assert route is not None
    assert route.groups() == (producer["method"], producer["path"], parameter)
    mobile = source("MobileClient.java")
    contains(
        method(mobile, f"public CompletionStage<JsonNode> {method_name}("),
        f'String value = pathValue({"journeyId" if method_name == "journey" else "evidenceId"});',
        f'return authorized(() -> client.human("{operation}", null, JsonNode.class,',
        f'new ProductionClient.Options(null, Map.of("{parameter}", value)))',
    )
    contains(method(mobile, "private <T> CompletionStage<T> authorized("), "sessions.invalidate();", "return attempt(operation).toCompletableFuture();")
    transport = source("AndroidHttpTransport.java")
    exchange = method(transport, "private <T> T exchange(")
    contains(exchange, "sessions.token().authorize(connection);", "return decode(status, encoded, responseType);")
    assert exchange.index("authorize(connection)") < exchange.index("getResponseCode()")
    contains(method(transport, "private URI resolve("), 'call.pathParameters().require(parameter)', 'path.replace("{" + parameter + "}", encodePath(value))')
    contains(method(transport, "private <T> T decode("), '!envelope.path("ok").booleanValue()', 'mapper.convertValue(envelope.get("result"), type)')
    production = (JVM / "ProductionClient.java").read_text()
    contains(production, "transport.<T>call(new ProductionTransport.Call(")


def test_finding_031_producer_material_matches_resolver():
    producer = (HUMAN / "production_reads.rs").read_text()
    material = method(producer, "pub fn managed_receipt_material(")
    contains(
        material,
        'Sha256::digest(&evidence.canonical_receipt)',
        'evidence.canonical_receipt != evidence.signed_receipt',
        'Rung::Receipted => "receipt-verified"',
        'Rung::Committed => "committed"',
        'PortableReceiptV2::export(',
        '"content_type":"application/vnd.layerx.receipt-proof-v2+json"',
        '"bytes_base64":STANDARD.encode(portable)',
    )
    fields = section(SCHEMA, "type.EvidenceMaterial")["required"]
    assert {field.split(":")[0] for field in fields} == {
        "evidence_id", "class", "verification", "content_type", "bytes_base64"
    }
    resolver = method(source("HumanReceiptResolver.java"), "static Payment verify(")
    for field in fields:
        assert f'"{field.split(":")[0]}"' in resolver
    contains(
        resolver,
        'id.equals(text(material, "evidence_id"))',
        '"layerx-receipt".equals(text(material, "class"))',
        'verification.equals(text(material, "verification"))',
        'Base64.getDecoder().decode(encoded)',
        'Base64.getEncoder().encodeToString(bytes).equals(encoded)',
        'type.equals("application/vnd.layerx.receipt-proof-v2+json")',
        'type.equals("application/vnd.layerx.receipt")',
        'if (proof.has("rawTransaction"))',
        '"layerx-receipt-proof-v2".equals(text(proof, "format"))',
        '"receipted".equals(text(proof, "verificationLevel"))',
        'shape(proof, fields);',
    )
    portable = (ROOT / "services/interop/crates/layerx-portable/src/receipt.rs").read_text()
    contains(portable, 'deny_unknown_fields', 'raw_transaction: Option<String>', 'inclusion_height: String')
    assert '"receiptJson"' not in resolver
    assert '"raw_transaction"' not in resolver


def test_finding_031_local_verification_precedes_payment_completion():
    resolver_source = source("HumanReceiptResolver.java")
    resolver = method(resolver_source, "static Payment verify(")
    contains(
        resolver,
        'id.equals(ReceiptGate.evidenceId(signed))',
        'require(signed.length <= 1_048_576',
        'keys.entries().stream()',
        'MessageDigest.isEqual(key.keyId(), keyId)',
        'height.compareTo(pin.firstHeight()) >= 0 && height.compareTo(pin.lastHeight()) <= 0',
        'Signature.getInstance("Ed25519")',
        'System.arraycopy(pin.publicKey(), 0, x509, 12, 32)',
        'verifier.update(digest);',
        'require(verifier.verify(signature));',
        'require(wire.integer(4).signum() == 0);',
        'require(wire.remaining() == 0 && sequence.signum() > 0);',
        'require(MessageDigest.isEqual(digest, hex(proof, "receiptDigest", 32)))',
        'require(inclusion.compareTo(height) >= 0);',
        'require(pin.firstHeight().equals(quantity(key, "firstHeight")));',
        'require(pin.lastHeight().equals(quantity(key, "lastHeight")));',
        'MessageDigest.isEqual(pin.publicKey(), hex(key, "publicKey", 32))',
        'require(MessageDigest.isEqual(activity, transactionHash));',
        'require(MessageDigest.isEqual(resulting, sha256(',
        'throw MobileIntegrationException.of(MobileIntegrationException.Code.VERIFICATION_FAILURE);',
    )
    assert resolver.index("verifier.verify(signature)") < resolver.index("return new Payment(")
    contains(resolver_source, 'STRICT_DUPLICATE_DETECTION', 'FAIL_ON_TRAILING_TOKENS', 'count <= remaining()', 'require(actual.equals(expected));')
    gate = source("ReceiptGate.java")
    resolve = method(gate, "private CompletionStage<State> resolve(")
    contains(
        resolve,
        'client.evidence(reference.id()).thenCompose(material ->',
        'HumanReceiptResolver.verify( material, reference.id(), reference.verification(), receiptKeys)',
        'MessageDigest.isEqual(payment.asset(), expectation.asset())',
        'MessageDigest.isEqual(payment.recipient(), expectation.recipient())',
        'payment.amount().equals(expectation.amount().value())',
        'new Verified("receipted", payment.digest())',
        'return resolve(references, index + 1, expectation);',
        'return CompletableFuture.failedFuture(',
    )
    assert resolve.index("HumanReceiptResolver.verify(") < resolve.index('new Verified(')
    assert 'new Verified("committed"' not in gate
    contains(method(gate, "private LocalVerifier.SequencerReceiptVerification verifyEvidence("), 'LocalVerifier.verifySequencerReceipt(', 'verification.requireSuccess();', 'MessageDigest.isEqual(carried.publicKey(), pinned.publicKey())')
    contains(method(gate, "private static boolean matches("), 'receipt.asset()', 'expectation.asset()', 'receipt.to()', 'expectation.recipient()', 'expectation.amount().value().equals(receipt.amount())')
    contains(method(gate, "public State project("), 'reference.id().equals(evidenceId(evidence.signedReceipt()))', 'verifyEvidence(evidence)', 'matches(verification.receipt(), expectation)')


def test_finding_031_reopened_tracking_source_path_has_no_cached_success():
    gate = source("ReceiptGate.java")
    track = method(gate, "public CompletionStage<State> track(")
    contains(
        track,
        'return client.journey(journeyId).thenCompose(journey ->',
        'journeyId.equals(journey.path("journey_id").textValue())',
        'return projectAsync(journey, expectation);',
    )
    assert 'Verified' not in track and 'settle(' not in track
    contains(method(gate, "public CompletionStage<State> projectAsync("), 'State state = localState(journey);', 'return resolve(references(journey), 0, expectation);')
    factory = source("LayerXAndroid.java")
    contains(method(factory, "public ReceiptGate gate()"), 'new ReceiptGate(client, configuration.receiptKeys().orElseThrow(')
    contains(factory, 'new MobileClient(new ProductionClient(transport, mapper, null), sessions, mapper)')
    contains(method(factory, "public void close()"), 'transport.close();', 'sessions.close();')
    producer = (HUMAN / "production_reads.rs").read_text()
    contains(method(producer, "fn public_journeys("), 'JourneyEngine::list(scope)', 'native_journey_json(scope, &entry)')
    contains(method(producer, "fn native_journey_json("), "native_journey_json_with(scope, journey, receipt_keys)")
    contains(method(producer, "fn native_journey_json_with("), '.verified_leg_evidence(index)', 'evidence_ref(digest, verification)')


def test_finding_031_recorded_node_receipt_digest_contract():
    """Fixture integrity and source preimage facts; not Java signature execution."""
    vectors = json.loads((ROOT / "sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json").read_text())
    cases = vectors["cases"]
    assert len(cases) >= 2
    for vector in cases:
        signed = bytes.fromhex(vector["signedReceipt"][2:])
        assert signed == bytes.fromhex(vector["receiptJson"]["receipt"][2:])
        assert signed[:6] == bytes.fromhex("000252010002")
        assert signed[-69:-64] == bytes.fromhex("0100000040")
        digest = hashlib.sha256(b"LXP/v1/receipt\0" + signed[:-69] + b"\0").hexdigest()
        assert "0x" + digest == vector["receiptDigest"]
        assert int(vector["receiptJson"]["status"], 16) == 1
        pin = vector["pinnedKey"]
        height = int(vector["receiptJson"]["height"], 16)
        assert pin["firstHeight"] <= height <= pin["lastHeight"]
        assert pin["keyId"] == vector["receiptJson"]["keyId"]
        assert pin["publicKey"] == vector["receiptJson"]["signingKey"]
    contains(source("HumanReceiptResolver.java"), 'Arrays.copyOf(signed, signatureOffset + 1)', 'unsigned[signatureOffset] = 0;', 'sha256("LXP/v1/receipt\\0".getBytes(StandardCharsets.UTF_8), unsigned)')
    contains(method(source("ReceiptGate.java"), "static String evidenceId("), 'MessageDigest.getInstance("SHA-256").digest(signedReceipt)')


def test_finding_031_record_names_proofs_and_execution_limit():
    record = Path(__file__).with_name("repair_22_31_418.kvx")
    task = section(record, "task.22.31.418")
    assert task["finding"] == "031"
    declared = set(task["proofs"])
    actual = set(re.findall(r"(?m)^def (test_finding_031_\w+)\(", Path(__file__).read_text()))
    assert declared == actual
    assert "not executed" in task["review_only"]
    assert "No pre-change verification" in task["baseline"]
    assert task["base_revision"] == "47246c9a1c9bbb5b892987ea79d024a770f979de"
    assert task["wiring_requests"]
