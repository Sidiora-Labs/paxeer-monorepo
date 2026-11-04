"""Offline source assertions for finding 005; these do not execute Android/Java."""

from pathlib import Path
import re
import tomllib
import xml.etree.ElementTree as ET


ROOT = Path(__file__).resolve().parents[3]
ANDROID = "sdk/integrations/android/"
APP = ANDROID + "sample-app/app/"
SAMPLE = APP + "src/main/java/com/sidiora/layerx/android/sample/"
SHARED = ANDROID + "src/sample/java/com/sidiora/layerx/android/sample/"
LIB = ANDROID + "src/main/java/com/sidiora/layerx/android/"
RECORD = Path(__file__).with_name("repair_22_33_613.kvx")


def source(path):
    text = (ROOT / path).read_text(encoding="utf-8")
    # Preserve literals while removing comments so comments cannot satisfy checks.
    return re.sub(
        r'"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\'|//[^\n]*|/\*[\s\S]*?\*/',
        lambda match: "" if match[0].startswith(("//", "/*")) else match[0],
        text,
    )


def body(text, declaration):
    match = re.search(declaration + r"\s*\{", text)
    assert match, declaration
    start = match.end()
    depth = 1
    for token in re.finditer(r'"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\'|[{}]', text[start:]):
        if token[0] == "{":
            depth += 1
        elif token[0] == "}":
            depth -= 1
            if depth == 0:
                return text[start:start + token.start()]
    raise AssertionError(f"Unclosed body: {declaration}")


def compact(text):
    return re.sub(r"\s+", "", text)


def test_holder_binds_required_publishable_receipt_resource_before_startup():
    holder = source(SAMPLE + "LayerXHolder.java")
    build = body(holder, r"private static LayerXHolder build\(Context context\)")
    assert 'import com.sidiora.layerx.android.MobileIntegrationException;' in holder
    assert re.search(
        r'int receiptKeysResource = resources\.getIdentifier\(\s*'
        r'"layerx_receipt_keys", "string", context\.getPackageName\(\)\);',
        build,
    )
    missing = body(build, r"if \(receiptKeysResource == 0\)")
    assert compact(missing) == (
        "throwMobileIntegrationException.of(MobileIntegrationException.Code.INVALID_CONFIGURATION);"
    )
    binding = (
        "declared.put(PublishableConfiguration.RECEIPT_KEYS_KEY,"
        "resources.getString(receiptKeysResource));"
    )
    ordered = compact(build)
    assert ordered.count("PublishableConfiguration.RECEIPT_KEYS_KEY") == 1
    assert ordered.index("if(receiptKeysResource==0)") < ordered.index(binding)
    assert ordered.index(binding) < ordered.index("PublishableConfiguration.of(declared)")
    assert ordered.index("PublishableConfiguration.of(declared)") < ordered.index(
        "LayerXAndroid.create(context,configuration)"
    ) < ordered.index("newWalletModel(mobile,receipts)")
    assert "catch" not in build
    assert "R.string.layerx_receipt_keys" not in holder


def test_declared_receipt_pins_reach_gate_without_relay_trust():
    config = source(LIB + "PublishableConfiguration.java")
    assert 'RECEIPT_KEYS_KEY = "layerx.receipt_keys";' in config
    assert "case RECEIPT_KEYS_KEY -> receiptKeys = parseReceiptKeys(value);" in config
    parse = body(config, r"private static LocalVerifier\.ReceiptKeys parseReceiptKeys\(String value\)")
    for check in (
        "DeserializationFeature.FAIL_ON_TRAILING_TOKENS", "!entries.isArray()", "entries.isEmpty()",
        "entry.size() != 4", 'entry.path("firstHeight").isIntegralNumber()',
        'entry.path("lastHeight").isIntegralNumber()', "new LocalVerifier.ReceiptKey(",
        "new LocalVerifier.ReceiptKeys(keys)", "throw invalid();",
    ):
        assert check in parse
    mobile = body(
        source(LIB + "LayerXAndroid.java"),
        r"public ReceiptGate gate\(ReceiptGate\.ReceiptResolver receipts\)",
    )
    assert "newReceiptGate(receipts,configuration.receiptKeys().orElseThrow(" in compact(mobile)
    assert "MobileIntegrationException.Code.INVALID_CONFIGURATION" in mobile
    model = body(source(SHARED + "WalletModel.java"),
                 r"public WalletModel\(LayerXAndroid mobile, ReceiptGate\.ReceiptResolver receipts\)")
    assert 'mobile.gate(Objects.requireNonNull(receipts, "receipts"))' in model


def test_sample_payment_source_path_preserves_signed_receipt_and_payment_checks():
    activity = body(source(SAMPLE + "WalletActivity.java"), r"private WalletModel\.Snapshot pay\(\)")
    assert "LayerXHolder.shared(this).model()" in activity
    assert "model.pay(request, expectation," in activity
    pay = body(source(SHARED + "WalletModel.java"),
               r"public synchronized Snapshot pay\(JsonNode quoteRequest, ReceiptGate\.Expectation expectation, IdempotencyKey key\)")
    assert pay.index("mobile.client().quote(request)") < pay.index("mobile.client().commit(")
    assert "gate.project(journey, expectation)" in pay
    resolver = source(SHARED + "RelayReceiptResolver.java")
    for check in ('connection.setRequestMethod("GET")', "return evidence(mapper.readTree(encoded));",
                  'hex(field(payload, "raw_transaction"))', 'hex(field(payload, "signed_receipt"))',
                  "new ReceiptGate.Evidence(rawTransaction, signedReceipt, receiptKey)"):
        assert check in resolver
    gate = source(LIB + "ReceiptGate.java")
    project = body(gate, r"public State project\(JsonNode journey, Expectation expectation\)")
    assert "receipts.resolve(reference.id())" in project
    assert "evidenceId(evidence.signedReceipt())" in project
    assert "verifyEvidence(evidence)" in project
    assert "if (matches(verification.receipt(), expectation)) return verified(verification);" in project
    verify = body(gate, r"private LocalVerifier\.SequencerReceiptVerification verifyEvidence\(Evidence evidence\)")
    assert "LocalVerifier.verifySequencerReceipt(evidence.signedReceipt(),evidence.rawTransaction(),receiptKeys)" in compact(verify)
    for field in ("keyId", "publicKey", "firstHeight", "lastHeight"):
        assert f"carried.{field}()" in verify and f"pinned.{field}()" in verify
    assert "verification.requireSuccess();" in verify
    matches = body(gate, r"private static boolean matches\(LocalVerifier\.ProtocolReceipt receipt, Expectation expectation\)")
    for check in ("MessageDigest.isEqual(receipt.asset(), expectation.asset())",
                  "MessageDigest.isEqual(receipt.to(), expectation.recipient())",
                  "expectation.amount().value().equals(receipt.amount())"):
        assert check in matches
    verifier = body(source("sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/verify/LocalVerifier.java"),
                    r"public static SequencerReceiptVerification verifySequencerReceipt\(byte\[\] receiptBytes,\s*byte\[\] rawTransaction, ReceiptKeys keys\)")
    for check in ("candidate.keyId(), receipt.contextHash()", "height.compareTo(key.firstHeight()) < 0",
                  "height.compareTo(key.lastHeight()) > 0", "!verifyEd25519(key.publicKey(), receipt.sequencerSignature(), digest)",
                  "fail(ReceiptCheck.SIGNATURE)", "!equal(transactionHash, receipt.activityId())"):
        assert check in verifier


def test_sample_build_and_manifest_wire_real_sources_and_launcher():
    gradle = source(APP + "build.gradle.kts")
    resources = dict(re.findall(
        r'resValue\("string", "([^"]+)", declared\("([^"]+)", "[^"]*"\)\)', gradle
    ))
    assert resources["layerx_service_url"] == "layerx.serviceUrl"
    assert resources["layerx_event_public_key"] == "layerx.eventPublicKey"
    assert resources["layerx_receipt_relay_url"] == "layerx.receiptRelayUrl"
    assert 'java.srcDirs("src/main/java", "../../src/sample/java")' in gradle
    assert 'namespace = "com.sidiora.layerx.android.sample"' in gradle
    manifest = ET.parse(ROOT / APP / "src/main/AndroidManifest.xml").getroot()
    name = "{http://schemas.android.com/apk/res/android}name"
    launcher = manifest.find("application/activity")
    assert launcher is not None and launcher.attrib[name] == ".WalletActivity"
    assert launcher.find("intent-filter/action").attrib[name] == "android.intent.action.MAIN"


def test_record_maps_finding_wiring_limits_and_private_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["005"]
    assert finding["audit"] == "22.30.163"
    assert finding["file"] == SAMPLE + "LayerXHolder.java"
    assert finding["status"] == "source_repaired_pending_gradle_wiring_and_runtime_proof"
    assert set(finding["tests"]) == {
        name for name, value in globals().items() if name.startswith("test_") and callable(value)
    }
    wiring = record["wiring"]["receipt_resource"]
    assert wiring["path"] == APP + "build.gradle.kts"
    assert wiring["owner_domain"] == 15
    for fact in ("layerx.receiptKeysJson", "layerx_receipt_keys", "layerx.receipt_keys",
                 "declaredKeys", "resource shrinking", "startup", "signed payment"):
        assert fact in wiring["request"]
    repair = record["repair"]
    assert repair["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_613.py"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "No baseline verification was run" in repair["baseline"]
    assert "not executed" in repair["review_only"]
