"""Offline source qualification for audit 22.30.163/039.

Parse the package command and recorded producer transcript, then check the
receipt suites against the production producer, transport and consumer sources.
This gate does not compile TypeScript or execute the Node test suites.
"""

import ast
import hashlib
import json
from pathlib import Path
import re
import shlex
import tomllib


ROOT = Path(__file__).resolve().parents[3]
AGENTS = "sdk/integrations/agents/"
DAEMON = "services/agent/crates/layerx-agentd/src/agent_rpc_dispatch.rs"
MIDDLEWARE = "services/platform/middleware/agent/src/daemon.ts"
FIXTURE = "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
RECORD = Path(__file__).with_name("repair_22_33_626.kvx")
TOKEN = re.compile(
    r'//[^\n]*|/\*.*?\*/|"(?:\\.|[^"\\\r\n])*"|\'(?:\\.|[^\'\\\r\n])*\''
    r"|`(?:\\.|[^`\\])*`|\.\.\.|[A-Za-z_$][\w$]*|[^\s]",
    re.S,
)
DAEMON_TESTS = (
    "daemonProviderConsumesCurrentProducerResponse",
    "daemonServiceProviderWiresCurrentReceiptEvidence",
    "daemonProviderBindsCurrentReceiptEvidence",
    "daemonProviderVerifiesSignedSequencerReceipt",
    "daemonFinalityStagesExcludeSettlementAnchored",
    "daemonProviderRefusesObsoleteStatusFields",
)
TOOL_TESTS = (
    "serviceTransportRefusesObsoleteStatusFields",
    "paymentToolVerifiesCurrentProducerReceipt",
    "trackingToolVerifiesCurrentProducerReceipt",
    "trackingToolRefusesUntrustedCurrentProducerReceipt",
    "trackingToolVerifiesExecutedReceiptAndRefusesSignedRevert",
    "paymentToolVerifiesSignedSequencerReceipt",
    "serviceClientReadsSignedReceiptFields",
    "trackToolReadsRecordedDaemonSubmissions",
    "trackToolRefusesMismatchedEvidenceAndMalformedSubmissions",
)


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def block_after(text, marker):
    """Read a balanced source block, excluding braces inside strings/comments."""
    start = text.index(marker) + len(marker)
    depth = 0
    opening = None
    for token in TOKEN.finditer(text, start):
        value = token.group()
        if value == "{":
            if opening is None:
                opening = token.start()
            depth += 1
        elif value == "}" and opening is not None:
            depth -= 1
            if depth == 0:
                return text[opening:token.end()]
    raise AssertionError(f"No complete block after {marker!r}")


def tokens(text):
    return [
        token.group() for token in TOKEN.finditer(text)
        if not token.group().startswith(("//", "/*"))
    ]


def object_keys(block):
    """Read top-level property declarations without counting nested properties."""
    values = tokens(block)
    assert values[0] == "{" and values[-1] == "}"
    depth = 0
    keys = []
    for index, value in enumerate(values):
        if depth == 1 and values[index - 1] in ("{", ","):
            if re.fullmatch(r"[A-Za-z_$][\w$]*", value):
                assert values[index + 1] in (":", ",", "}"), value
                keys.append(value)
        if value in ("{", "[", "("):
            depth += 1
        elif value in ("}", "]", ")"):
            depth -= 1
    assert len(keys) == len(set(keys)), keys
    return set(keys)


def exact_fields(text, variable):
    match = re.search(r"exact\(" + re.escape(variable) + r",\s*(\[[^\]]*\])", text)
    assert match, variable
    return set(json.loads(match[1]))


def require_facts(text, facts):
    for fact in facts:
        assert fact in text, f"Missing source fact: {fact}"


def suite(name):
    return source(AGENTS + "test/" + name + ".test.mjs")


def function(text, name):
    return block_after(text, "function " + name + "(")


def test_039_package_command_executes_all_four_suites():
    manifest = json.loads(source(AGENTS + "package.json"))
    assert shlex.split(manifest["scripts"]["pretest"]) == ["npm", "run", "build"]
    assert shlex.split(manifest["scripts"]["test"]) == [
        "node", "--test", "test/*.test.mjs",
    ]
    suites = {path.name for path in (ROOT / AGENTS / "test").glob("*.test.mjs")}
    assert {
        "dependency-contracts.test.mjs", "daemon-providers-receipts.test.mjs",
        "tools-receipts.test.mjs", "a2a-store.test.mjs",
    } <= suites
    assert all((ROOT / AGENTS / "test" / name).is_file() for name in suites)
    assert manifest["scripts"]["build"] == "node scripts/patch-dependencies.mjs && tsc -p tsconfig.json"
    config = json.loads(source(AGENTS + "tsconfig.json"))
    assert config["compilerOptions"]["strict"] is True
    assert config["compilerOptions"]["exactOptionalPropertyTypes"] is True


def test_039_receipt_tests_are_declared_registered_and_assertive():
    for name, names in (("daemon-providers-receipts", DAEMON_TESTS), ("tools-receipts", TOOL_TESTS)):
        text = suite(name)
        assert re.findall(r"^test\((\w+)\);$", text, re.M) == list(names)
        assert not re.search(r"\b(?:test|it)\.(?:skip|todo|only)\b|\b(?:skip|todo|only)\s*:", text)
        assert not re.search(r"\b(?:mock|stub)\s*\(|\bt\.mock\b|process\.env|child_process", text)
        for name in names:
            body = function(text, name)
            assert "await " in body and "assert." in body, name
        require_facts(text, (
            "../../../platform/conformance/fixtures/sequencer-receipt-v1.json",
            'import test from "node:test"',
        ))


def test_039_daemon_status_and_portable_key_heights_match_consumers():
    text = suite("daemon-providers-receipts")
    provider = source(AGENTS + "src/daemon-providers.ts")
    verification = provider.split("export async function verifySequencerReceipt(", 1)[1].split(
        "function receiptHeight(", 1
    )[0]
    receipt = block_after(function(text, "daemonReceipt"), "return ")
    assert object_keys(receipt) == exact_fields(verification, "receipt") == {
        "raw_transaction", "signed_receipt", "receipt_key", "verification_level",
    }
    assert object_keys(block_after(receipt, "receipt_key:")) == exact_fields(verification, "key") == {
        "key_id", "public_key", "first_height", "last_height",
    }
    assert exact_fields(verification, "status") == {"state", "rung"}
    require_facts(text, (
        'const status = (rung = Rung.Receipted) => ({ state: receiptedStatus.state, rung });',
        'const height = (value) => `0x${BigInt(value).toString(16)}`;',
        "first_height: String(entry.pinnedKey.firstHeight)",
        "last_height: String(entry.pinnedKey.lastHeight)",
        "firstHeight: height(entry.pinnedKey.firstHeight)",
        "lastHeight: height(entry.pinnedKey.lastHeight)",
        "verifySequencerReceipt(daemonReceipt(entry), status(), context(entry), receiptDigest)",
        'vectors.filter((entry) => entry.id.startsWith("V1-"))',
        "assert.deepEqual(verified.evidence, {",
    ))
    require_facts(verification, (
        'status["rung"] !== Rung.Receipted',
        "await verifySignedSequencerReceipt(signedBytes, Buffer.from(rawTransaction",
        'Buffer.from(verified.receipt.activityId).toString("hex") !== context.activityId',
        "verified.receipt.resultCode !== 0",
        'receiptDigest: `0x${receiptDigest}`',
        'firstHeight: `0x${BigInt(firstHeight).toString(16)}`',
        'lastHeight: `0x${BigInt(lastHeight).toString(16)}`',
    ))
    require_facts(source("services/agent/crates/layerx-agentd/src/agent_rpc.rs"), (
        '"state": "achieved",', '"rung": level_name(*level)',
    ))


def test_039_daemon_signature_binding_and_finality_refusals_remain():
    text = suite("daemon-providers-receipts")
    body = function(text, "daemonProviderVerifiesSignedSequencerReceipt")
    require_facts(body, (
        "changedSignature[changedSignature.length - 1] ^= 1",
        "changedTransaction[changedTransaction.length - 1] ^= 1",
        'public_key: "99".repeat(32)', 'key_id: "99".repeat(32)',
        'first_height: "1001"', 'first_height: "0", last_height: "999"',
        'activityId: "99".repeat(32)', 'context(vector), "99".repeat(32)',
        'entry.id === "V2-layerxledger-revert"',
        'refuses("verification-failure")', 'refuses("decode-failure")',
        'authorised_batch: {}', 'signed_receipt: "0x" + receipt.signed_receipt',
        'raw_transaction: ""', 'public_key: "00"',
        '["-1", "01", "18446744073709551616"]', 'first_height: "2000"',
    ))
    levels = function(text, "daemonFinalityStagesExcludeSettlementAnchored")
    require_facts(levels, (
        '["SequencerSigned", "BatchIncluded", "StateProven", "CheckpointFinalised"]',
        '["SettlementAnchored", "Unverified", "Unknown"]',
        'status("submitted")', 'status("committed")',
        '{ state: "pending", requested: "receipted", achieved: "submitted" }',
        '{ state: "achieved", level: "SequencerSigned" }',
        '{ state: "achieved", rung: "receipted", level: "SequencerSigned" }',
        'refuses("verification-failure")', 'refuses("decode-failure")',
    ))


def test_039_real_portable_producer_flows_through_http_to_service_consumer():
    text = suite("tools-receipts")
    producer_fixture = function(text, "portableReceipt")
    require_facts(producer_fixture, (
        "const submission = decodeDaemonSubmitResult({",
        'preparation_ref: "signed-payment", state: "Receipted"',
        "const resolved = await resolveDaemonReceipt(submission, bytes(entry.rawTransaction), [{",
        "first_height: BigInt(entry.pinnedKey.firstHeight)",
        "last_height: BigInt(entry.pinnedKey.lastHeight)",
        "return resolved.portableReceipt;",
    ))
    middleware = source(MIDDLEWARE)
    require_facts(middleware, (
        "export async function resolveDaemonReceipt(",
        "await verifySequencerReceipt(signedReceipt, raw, [key])",
        "if (trusted === undefined || receiptDigest === undefined) throw new PlatformSdkError",
        'firstHeight: `0x${trusted.first_height.toString(16)}`',
        'lastHeight: `0x${trusted.last_height.toString(16)}`',
    ))
    producer = block_after(middleware, "const portableReceipt: PortableReceiptV2 = Object.freeze(")
    consumer = block_after(source(AGENTS + "src/services.ts"), "public async resolve(receiptRef: string)")
    consumed = block_after(consumer, "return ")
    assert object_keys(producer) == object_keys(consumed) == {
        "format", "verificationLevel", "rawTransaction", "signedReceipt", "receiptDigest", "receiptKey",
    }
    assert object_keys(block_after(producer, "receiptKey: Object.freeze(")) == object_keys(
        block_after(consumed, "receiptKey:")
    ) == {"keyId", "publicKey", "firstHeight", "lastHeight"}
    require_facts(consumer, (
        "this.#endpoint.call({ receipt_ref: receiptRef })",
        'format !== "layerx-receipt-proof-v2"',
        'verificationLevel !== "receipted" && verificationLevel !== "committed"',
        'receiptHeight(key["firstHeight"])', 'receiptHeight(key["lastHeight"])',
        'receiptHexField(response, "receiptDigest", 32)',
        'receiptHexField(key, "keyId", 32)', 'receiptHexField(key, "publicKey", 32)',
    ))
    fixture = function(text, "receiptFixture")
    require_facts(fixture, (
        "proof: await portableReceipt()", "createServer(async (request, response)",
        'server.listen(0, "127.0.0.1")', 'JSON.parse(Buffer.concat(chunks).toString("utf8"))',
        'assert.equal(request.headers.authorization, "Bearer receipt-fixture-token")',
        "response.end(JSON.stringify(", 'verification_status: fixture.verification',
        'verification: { state: "achieved", rung: "receipted" }',
        "new LayerXServiceEndpoint(", "new LayerXReceiptResolver(fixture.endpoint)",
    ))
    require_facts(function(text, "executor"), (
        "readDeclaredConfig(environment)", "authenticatedAgentTransport(environment, config)",
        "new ProductionClient(new LayerXAgentTransport(authenticated.transport))",
        "new AgentMiddleware({", "new AgentToolExecutor({",
    ))


def test_039_payment_fixture_uses_the_recorded_native_payment_effect():
    vectors = json.loads(source(FIXTURE))
    assert vectors["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    assert vectors["chainId"] == 125
    vector = next(entry for entry in vectors["cases"] if entry["id"] == "V1-layerxledger-pay-exact")
    assert vector["expected"]["verifier"] == "accepts"
    assert vector["receiptJson"]["status"] == "0x1"
    log, = vector["receiptJson"]["logs"]
    assert log["address"] == vector["receiptJson"]["to"] == "0x000000000000000000000000000000000000101a"
    assert len(log["topics"]) == 4
    assert log["topics"][0] == "0xc4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1"
    data = bytes.fromhex(log["data"][2:])
    assert len(data) == 64 and int.from_bytes(data[32:], "big") == 1_000_000
    assert any(data[:32]) and any(bytes.fromhex(log["topics"][3][2:]))
    text = suite("tools-receipts")
    body = function(text, "paymentToolVerifiesSignedSequencerReceipt")
    require_facts(text, ('entry.id === "V1-layerxledger-pay-exact"',))
    require_facts(body, (
        "...paymentFacts(fixture.proof)",
        'assert.deepEqual(renderOutcome(await tools.execute("layerx_verify_receipt", input)), expected);',
        "receiptDigest: hex(keccak256(bytes(vector.signedReceipt)))",
        'path: "/receipt", method: "POST", body: { receipt_ref: "payment-receipt" }',
    ))
    facts = source(AGENTS + "test/receipt-producer-fixture.mjs")
    require_facts(function(facts, "paymentFacts"), (
        "sdk.decodeRawEip1559(Buffer.from(proof.rawTransaction.slice(2), \"hex\"))",
        "assert.equal(transaction.data.length, 196)",
        "transaction.data.slice(68, 100)", "transaction.data.slice(100, 132)",
        "transaction.data.slice(36, 68)",
    ))
    seller = block_after(source("services/platform/middleware/seller/src/index.ts"), "export async function verifyPaymentReceipt(")
    require_facts(seller, (
        "decodePaymentExactLog(log)", "payments.length !== 1", "payment.amount !== amount",
        "!equalBytes(payment.assetId, parseHex32(requirements.asset))",
        "!equalBytes(payment.to, parseHex32(requirements.payTo))",
    ))
    require_facts(source(AGENTS + "src/tools.ts"), (
        "const network = `eip155:${decodeRawEip1559(receiptBytes(evidence.rawTransaction)).chainId}`",
        "verification = await verifyPaymentReceipt({",
        "...evidence, receiptDigest: `0x${toHex(verified.receiptDigest)}`",
        "await verifySequencerReceipt(signedReceipt, receiptBytes(evidence.rawTransaction), this.#config.receiptKeys)",
        "evidence.receiptDigest !== `0x${toHex(keccak256(signedReceipt))}`",
    ))


def test_039_recorded_daemon_track_transcript_and_strict_decoder_agree():
    daemon = source(DAEMON)
    marker = 'const MIDDLEWARE_SUBMISSION_TRANSCRIPT: &str = r#"'
    original = json.loads(daemon.split(marker, 1)[1].split('"#;', 1)[0])
    recorded = json.loads(source(AGENTS + "test/fixtures/daemon-submission-transcript.json"))
    # Repair the obsolete reference using the current owner rule. Every other
    # field, including the signed receipt and its signature, stays recorded.
    assert original["committed"]["submission"]["receipt_ref"] == "ac73faf315ec0dd1f7a3ff5ffddf39fdedd5550696c74489038fe5153e9b6a69"
    original["committed"]["submission"]["receipt_ref"] = hashlib.sha256(
        b"LXP/v1/receipt\0" + bytes.fromhex(original["submit"]["signed_receipt"])
    ).hexdigest()
    assert recorded == original
    producer = block_after(daemon, "pub(crate) fn decode_tracked(")
    producer_fields = set(re.findall(r'out\.insert\(\s*"(\w+)"\.into\(\)', producer))
    assert producer_fields == {
        "tx_hash", "submission_ref", "state", "receipt_ref", "result_code", "rung", "evidence", "transitions",
    }
    tracked = function(suite("tools-receipts"), "trackedSubmission")
    assert object_keys(block_after(tracked, "return ")) == producer_fields - {"result_code"}
    assert set(recorded["committed"]["submission"]) == producer_fields - {"result_code"}
    assert set(recorded["pending"]["submission"]) == producer_fields - {"result_code", "receipt_ref"}
    assert recorded["pending"]["submission"]["rung"] == "submitted"
    assert recorded["committed"]["submission"]["rung"] == "committed"
    assert hashlib.sha256(b"LXP/v1/receipt\0" + bytes.fromhex(recorded["submit"]["signed_receipt"])).hexdigest() == recorded["committed"]["submission"]["receipt_ref"]
    owner = block_after(source("services/agent/crates/layerx-agentd/src/human_runtime.rs"), "pub fn write_submission(")
    require_facts(owner, ("layerx_wire::hash::receipt_digest(receipt.canonical_bytes())",))
    hashing = source("services/agent/crates/layerx-wire/src/hash.rs")
    require_facts(hashing, ('Self::Receipt => b"LXP/v1/receipt\\0"',))
    require_facts(block_after(hashing, "pub fn receipt_digest("), ("Domain::Receipt",))
    require_facts(producer, ('"submission_ref".into(), hexv(&tx_hash)', 'rung_name(achieved)', 'if evidence_count > MAX_EVIDENCE', 'if transition_count > MAX_TRANSITIONS'))
    track_dispatch = daemon.split("Operation::Track => {", 1)[1].split("Operation::ApprovalList", 1)[0]
    require_facts(track_dispatch, ("owner.track(peer, &request.submission_ref)", "decode_tracked"))
    tools = source(AGENTS + "src/tools.ts")
    decoder = tools.split("function decodeTrackedSubmission(", 1)[1].split("async function verifySequencerReceipt(", 1)[0]
    require_facts(decoder, (
        'const fields = ["tx_hash", "submission_ref", "state", "rung", "evidence", "transitions"',
        '...(executed ? ["receipt_ref"] : state === "Failed" ? ["result_code"] : [])',
        "Object.keys(submission).some((field) => !fields.includes(field))",
        'txHash !== expectedReference || submission["submission_ref"] !== txHash',
        'evidence.length > 64', 'transitions.length > 64',
        'state === "Committed" && rung !== "committed"',
        'transition === "Executed" || transition === "Failed"',
        'BigInt(canonicalInteger(item, "at")) > 0xffff_ffff_ffff_ffffn',
        'BigInt(code) < -2147483648n || BigInt(code) > 2147483647n',
    ))
    track = block_after(tools, "async #track(")
    require_facts(track, (
        "decodeTrackedSubmission(response, submissionRef)",
        'submission.state === "Executed" || submission.state === "Committed"',
        "await this.#receipts.resolve(submission.receiptRef)",
        "await verifySequencerReceipt(receiptBytes(evidence.signedReceipt), receiptBytes(evidence.rawTransaction), this.#config.receiptKeys)",
        'toHex(verified.receipt.activityId) !== submission.txHash',
        'evidence.receiptDigest !== `0x${receiptDigest}`',
        'createHash("sha256").update("LXP/v1/receipt\\0").update(receiptBytes(evidence.signedReceipt)).digest("hex") !== submission.receiptRef',
        'output["verificationLevel"] = verified.level',
    ))
    recorded_test = function(suite("tools-receipts"), "trackToolReadsRecordedDaemonSubmissions")
    require_facts(recorded_test, (
        "fixture.proof = await portableReceipt(entry)",
        "structuredClone(daemonTranscript.pending.submission)",
        "structuredClone(daemonTranscript.committed.submission)",
        "assert.equal(fixture.requests.length, 1)", "assert.equal(fixture.requests.length, 3)",
        'state: "Unknown", verificationLevel: "Submitted"',
        'state: "Committed", receiptDigest: hex(keccak256(bytes(entry.signedReceipt)))',
    ))


def test_039_service_and_tool_refusals_are_preserved_and_extended():
    text = suite("tools-receipts")
    payment = function(text, "paymentToolVerifiesSignedSequencerReceipt")
    require_facts(payment, (
        '["signedReceipt", changedByte(vector.signedReceipt)]',
        '["rawTransaction", changedByte(vector.rawTransaction)]',
        'amount: "1000000000000000001"', 'asset: "55".repeat(32)', 'recipient: "66".repeat(32)',
        'keyId: `0x${"77".repeat(32)}`', 'publicKey: `0x${"88".repeat(32)}`',
        'firstHeight: Number(BigInt(vector.receiptJson.height)) + 1', 'firstHeight: 0, lastHeight: 999',
        'entry.id === "V2-layerxledger-revert"',
        'untrusted.execute("layerx_verify_receipt", input)', 'untrusted.execute("layerx_track", trackInput)',
        'refused("layerx_verify_receipt")', 'refused("layerx_track")',
    ))
    service = function(text, "serviceClientReadsSignedReceiptFields")
    require_facts(service, (
        '["receipted", "committed"]', 'assert.deepEqual(evidence, fixture.proof)',
        '["authorized_batch", "authorizedBatch", "canonical_receipt", "canonicalReceipt"]',
        'format: "layerx-receipt-proof-v1"', 'verificationLevel: "SettlementAnchored"',
        '["rawTransaction", "signedReceipt", "receiptDigest"]',
        '[field]: undefined', '[field]: "0x"', '[field]: "abcd"',
        'receiptDigest: "0x00"', 'receiptKey: undefined', '["keyId", "publicKey"]',
        '"-1", "01", "18446744073709551616", 1000',
        '"0x00", "0x01", "0x10000000000000000", "0x", "0X1"', 'firstHeight: "0x7d0"',
        'await assert.rejects(fixture.receipts.resolve("payment-receipt"), { code: "service-refused" })',
    ))
    track = function(text, "trackToolRefusesMismatchedEvidenceAndMalformedSubmissions")
    require_facts(track, (
        'receiptDigest: `0x${"99".repeat(32)}`', 'entry.id === "V1-pax-transfer"',
        'tx_hash: "99".repeat(32)', 'submission_ref: "99".repeat(32)', 'receipt_ref: "99".repeat(32)',
        'receipt_ref: undefined', 'state: "UnknownState"', 'state: "Committed", rung: "receipted"',
        'rung: "submitted"', 'rung: "sequencer-signed"', 'verification_level: "Receipted"',
        'evidence: new Array(65)', 'transitions: new Array(65)', 'digest: "00"',
        'to: "Executed"', 'at: "18446744073709551616"',
        'activity_id: tracked.tx_hash, submission: tracked, receipt: null',
        'assert.deepEqual(await tools.execute("layerx_track", input), refused)',
    ))


def test_039_external_daemon_receipt_mismatch_is_recorded_as_wiring():
    producer = block_after(source(DAEMON), "fn receipt_value(")
    producer_fields = set(re.findall(r'out\.insert\(\s*"(\w+)"\.into\(\)', producer))
    assert {"canonical_bytes", "authorised_batch", "signed_receipt", "receipt_key_id", "public_key", "first_height", "last_height", "rung"} <= producer_fields
    assert "raw_transaction" not in producer_fields and "receipt_key" not in producer_fields
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    wiring = record["wiring"]["daemon_receipt"]
    assert wiring["path"] == DAEMON and wiring["owner_domain"] == 15
    assert wiring["request"] and wiring["status"] == "outside-scope; runtime integration remains unqualified"
    assert record["meta"]["proof_kind"] == "offline-source-facts"


def test_039_record_maps_the_finding_tests_and_private_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["meta"]["task"] == "22.33.626"
    assert record["meta"]["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    finding = record["finding"]["039"]
    assert finding["audit"] == "22.30.163" and finding["class"] == "test-gap"
    assert finding["file"] == AGENTS + "package.json" and finding["line"] == 60
    tests = {node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    assert set(finding["tests"]) == tests
    assert finding["runtime_tests"] == list(DAEMON_TESTS + TOOL_TESTS)
    assert record["qualification"]["build_cmd"] == "python3 --version"
    assert record["qualification"]["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_626.py"
    assert record["qualification"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert record["qualification"]["verified_revision"]
    assert record["qualification"]["baseline_run"] is False
    assert record["qualification"]["baseline_failures"] == []
    assert record["qualification"]["review_only"]
