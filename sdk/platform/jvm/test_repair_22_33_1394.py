"""Guard README verification contracts against production JVM sources and recordings.

This Python-only gate checks documentation, public signatures and producer wire
shapes. It does not execute Java, compile snippets or claim cryptographic JVM
qualification; the scoped repair record names that gate-policy conflict.
"""

import ast
import json
from pathlib import Path
import re
import tomllib


JVM = Path(__file__).resolve().parent
ROOT = JVM.parents[2]
VERIFY = JVM / "src/main/java/com/sidiora/layerx/sdk/verify"
FIXTURES = ROOT / "sdk/platform/conformance/fixtures"
README = JVM / "README.md"
RECORD = JVM / "repair-22.33.1394.kvx"


def source(name):
    return (VERIFY / f"{name}.java").read_text()


def guidance():
    return README.read_text().split("## Local verification\n", 1)[1].split(
        "\n## Streaming", 1
    )[0]


def snippet(class_name):
    matches = [
        block
        for block in re.findall(r"```java\n(.*?)\n```", guidance(), re.S)
        if f"public final class {class_name} {{" in block
    ]
    assert len(matches) == 1, f"Expected one complete {class_name} snippet"
    return matches[0]


def compact(value):
    return " ".join(value.split())


def record_parameters(name, record):
    declaration = re.search(
        rf"public record {record}\((.*?)\)\s*\{{", source(name), re.S
    )
    assert declaration is not None, f"Production record {record} is absent"
    return compact(declaration[1])


def portable_fields(constant):
    declaration = re.search(
        rf"Set<String> {constant} = Set\.of\((.*?)\);",
        source("PortableReceipt"),
        re.S,
    )
    assert declaration is not None
    return set(re.findall(r'"([A-Za-z]+)"', declaration[1]))


def test_jvm_readme_verifier_uses_current_rungs_and_no_retired_entry_points():
    text = guidance()
    enum = re.search(
        r"public enum VerificationLevel\s*\{(.*?);", source("LocalVerifier"), re.S
    )
    assert enum is not None
    levels = re.findall(r'([A-Z]+)\("([a-z]+)"\)', enum[1])
    assert levels == [("RECEIPTED", "receipted"), ("COMMITTED", "committed")]
    for name, wire in levels:
        assert f"`{name}`" in text and f"`{wire}`" in text
    assert "`submitted` is pending and has no verifiable receipt" in text
    assert "does not establish block commitment" in text
    for retired in (
        "AuthorizedReceiptBatch", "verifyReceipt(", "kernel batch",
        "CHECKPOINT_FINALISED", "SETTLED", "ANCHORED",
    ):
        assert retired not in README.read_text()
    assert "HumanModels.HumanVerificationLevel" in text
    assert "does not add another network finality step" in text


def test_jvm_readme_receipt_snippet_matches_production_key_and_transaction_api():
    code = compact(snippet("ReceiptExample"))
    assert record_parameters("LocalVerifier", "ReceiptKey") == (
        "byte[] keyId, byte[] publicKey, BigInteger firstHeight, BigInteger lastHeight"
    )
    assert record_parameters("LocalVerifier", "ReceiptKeys") == "List<ReceiptKey> entries"
    assert (
        "new LocalVerifier.ReceiptKey( trustedKeyId, trustedPublicKey, firstHeight, lastHeight)"
        in code
    )
    assert "new LocalVerifier.ReceiptKeys(List.of(" in code
    assert (
        "public static SequencerReceiptVerification verifySequencerReceipt(byte[] receiptBytes, "
        "byte[] rawTransaction, ReceiptKeys keys)" in compact(source("LocalVerifier"))
    )
    assert "LocalVerifier.verifySequencerReceipt( signedReceipt, rawTransaction, pins)" in code
    assert code.index("verifySequencerReceipt(") < code.index("verified.requireSuccess();")
    assert code.index("verified.requireSuccess();") < code.index("return verified;")
    text = compact(guidance())
    for requirement in (
        "raw EIP-1559 transaction locally", "`eth_sendRawTransaction`",
        "Do not substitute calldata", "independently trusted key pins",
        "nonzero 32-byte key ID", "32-byte Ed25519 public key",
        "inclusive unsigned height range", "`status()` is `1` for success and `0` for revert",
        "expected log, asset, recipient and amount",
    ):
        assert requirement in text


def test_jvm_readme_portable_snippet_requires_commitment_and_retained_transaction():
    code = compact(snippet("PortableExample"))
    assert record_parameters("ConsensusTrust", "ConsensusTrust") == (
        "String chainId, long height, byte[] validatorsHash"
    )
    assert (
        "public static Verification verify(byte[] bundle, LocalVerifier.ReceiptKeys receiptKeys, "
        "ConsensusTrust consensusTrust, List<ConsensusTrust.Step> transitions)"
        in compact(source("PortableReceipt"))
    )
    assert "new ConsensusTrust( trustedChainId, trustedHeight, trustedValidatorsHash)" in code
    assert "byte[] snapshot = bundle.clone();" in code
    verify_at = code.index("PortableReceipt.verify(snapshot, pinnedKeys, trust, transitions)")
    level_at = code.index('if (!"committed".equals(verified.verificationLevel()))')
    parse_at = code.index("new ObjectMapper().readTree(snapshot)")
    transaction_at = code.index(
        "if (!MessageDigest.isEqual(retainedRawTransaction, carriedTransaction))"
    )
    success_at = code.index("verified.requireSuccess();")
    assert verify_at < level_at < parse_at < transaction_at < success_at
    assert success_at < code.index("return verified;")
    assert code.count("throw PlatformSdkException.verificationFailure();") == 2
    assert 'document.required("rawTransaction").textValue().substring(2)' in code
    assert "HexFormat.of().parseHex(" in code
    assert "throws IOException" in code
    assert "new LocalVerifier.ReceiptKey" not in code
    assert "receiptKey" not in code and "validatorSet" not in code
    for imported in re.findall(r"import (com\.sidiora\.[\w.]+);", snippet("PortableExample")):
        assert (JVM / "src/main/java" / (imported.replace(".", "/") + ".java")).is_file()
    assert "<artifactId>jackson-databind</artifactId>" in (JVM / "pom.xml").read_text()


def test_jvm_readme_portable_shapes_match_unmodified_producer_evidence():
    published = json.loads((FIXTURES / "vector-v5-portable.json").read_text())
    bundles = published["bundles"]
    assert [bundle["verificationLevel"] for bundle in bundles] == ["receipted", "committed"]
    assert 'public static final String FORMAT = "layerx-receipt-proof-v2";' in source(
        "PortableReceipt"
    )
    text = guidance()
    for bundle, constant in zip(bundles, ("RECEIPTED_FIELDS", "COMMITTED_FIELDS"), strict=True):
        assert bundle["format"] == "layerx-receipt-proof-v2"
        assert set(bundle) == portable_fields(constant)
        for field in bundle:
            assert f"`{field}`" in text
        assert bundle["signedReceipt"] == bundle["receiptJson"]["receipt"]
        assert bundle["receiptKey"]["keyId"] == bundle["receiptJson"]["keyId"]
        height = int(bundle["receiptJson"]["height"], 16)
        assert int(bundle["receiptKey"]["firstHeight"], 16) <= height
        assert height <= int(bundle["receiptKey"]["lastHeight"], 16)
    assert bundles[0]["rawTransaction"].startswith("0x02")
    converted = json.loads((FIXTURES / "vector-v4-committed.json").read_text())["positive"]
    for field in (
        "rawTransaction", "signedReceipt", "receiptDigest", "receiptJson",
        "blockHeader", "receiptInclusion", "commit", "validatorSet",
    ):
        assert bundles[1][field] == converted[field]
    assert set(bundles[1]["receiptInclusion"]) >= {"NextHeader", "NextCommit", "NextValidators"}
    link = "../conformance/fixtures/vector-v5-portable.json"
    assert f"]({link})" in text and (JVM / link).is_file()


def test_jvm_readme_explains_independent_consensus_and_receipt_trust():
    text = compact(guidance())
    for requirement in (
        "independently obtained consensus chain ID, positive height and 32-byte validator-set hash",
        "not the EVM chain ID `125`", "empty list when the heights match",
        "bundle's own key and validator set do not establish trust",
        "receipt-only `PortableReceipt.verify(bundle, pinnedKeys)` overload refuses",
        "receipt pin authenticates the sequencer signature",
        "cannot substitute for the consensus trust anchor",
        "Never initialise `trustedValidatorsHash` from the bundle's own `validatorSet`",
        "successor header and commit", "next validator hash",
        "fixture keys and consensus roots are test material, never production trust anchors",
        "Supplying consensus trust alone does not require commitment",
    ):
        assert requirement in text
    portable = source("PortableReceipt")
    assert "require(consensusTrust != null);" in portable
    assert "consensusTrust.authenticate(header, transitions);" in portable
    assert "committed = Committed.verify(" in portable
    assert "chainId.equals(header.chainId())" in source("ConsensusTrust")
    assert "target.height() - height == transitions.size()" in source("ConsensusTrust")


def test_jvm_readme_repair_record_names_tests_and_gate_limits():
    record = tomllib.loads(RECORD.read_text())
    repair = record["repair"]["22"]["33"]["1394"]
    assert repair["audit"] == "22.30.77" and repair["acceptance"] == "22998.1"
    finding = record["finding"]["jvm-readme-verifier"]
    functions = {
        node.name
        for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == functions - {
        "test_jvm_readme_repair_record_names_tests_and_gate_limits"
    }
    assert "No pre-change verification" in repair["baseline"]
    assert "hf-build" in repair["evidence"] and "hf-done" in repair["evidence"]
    assert "candidate revision" in repair["evidence"]
    assert "does not execute the JVM" in repair["proof_limit"]
    assert "spec/layerx-merge/spec.kvx" in record["wiring"]["task_record"]
    assert "owning domain 8" in record["wiring"]["jvm_execution"]
