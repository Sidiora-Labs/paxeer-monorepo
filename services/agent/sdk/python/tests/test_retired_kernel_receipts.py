import ast
import json
from copy import deepcopy
from dataclasses import replace
from importlib.util import find_spec, module_from_spec, spec_from_file_location
from pathlib import Path
from collections.abc import Mapping

import pytest
from typing import get_type_hints

import layerx_sdk
import layerx_sdk.ai_market as ai_market
import layerx_sdk.program_wire as program_wire
import layerx_sdk.programs as programs
import layerx_sdk.verifier as verifier
import layerx_sdk.x402 as x402
import layerx_sdk.x402_rpc as x402_rpc
from layerx_sdk.committed import CommitTrust, verify_committed_transaction


def test_kernel_receipt_paths_are_gone():
    retired = (
        "verify_receipt",
        "verify_receipt_outcome",
        "verify_program_lifecycle_receipt",
        "verify_program_receipt_outcome_v5",
        "verify_batch_inclusion",
        "verify_checkpoint",
        "decode_program_receipt_outcome",
        "programs_module_version_for_protocol",
        "ProtocolReceipt",
        "ProgramReceiptOutcome",
        "ReceiptEffect",
        "SequencerAuthorization",
        "InclusionVerification",
        "CheckpointAttestation",
        "GuarantorKey",
        "CheckpointCertificate",
        "CheckpointVerificationInput",
        "CheckpointVerification",
        "BatchHeader",
        "decode_batch_header",
        "verify_merkle_inclusion",
        "verify_program_receipt",
        "AuthorizedReceiptBatch",
        "PaymentCommitmentEvidence",
        "_SequencerAuthorization",
    )
    for name in retired:
        assert not hasattr(layerx_sdk, name), name
        assert not hasattr(verifier, name), name
        assert name not in layerx_sdk.__all__, name
    assert not hasattr(programs, "verify_program_receipt")
    assert not hasattr(ai_market, "CheckpointVerification")
    assert not hasattr(ai_market.SnapshotBinding, "require_checkpoint")
    assert (
        get_type_hints(ai_market.OperationJournal.finalize)["finality"]
        == Mapping[str, object]
    )
    for name in ("submit", "resend_exact", "expire"):
        assert callable(getattr(ai_market.OperationJournal, name)), name
    for name in (
        "PaymentCheckpointEvidence",
        "verify_payment_commitment_evidence",
        "RpcCheckpointAuthority",
        "rpc_checkpoint_evidence",
    ):
        assert not hasattr(layerx_sdk, name), name
        assert not hasattr(x402, name), name
        assert name not in layerx_sdk.__all__, name
    assert find_spec("layerx_sdk.x402_checkpoint") is None
    assert not hasattr(x402, "PaymentCommitmentEvidence")
    assert not hasattr(x402_rpc, "rpc_batch_evidence")
    assert get_type_hints(x402.PaymentEvidence, globalns={**vars(x402), "CommitTrust": CommitTrust})["finality"] == tuple[CommitTrust, dict] | None
    for name in ("verify_sequencer_receipt", "verify_payment_receipt"):
        assert callable(getattr(x402, name)), name
    for name in ("PaymentEvidence", "verify_payment_receipt"):
        assert getattr(layerx_sdk, name) is getattr(x402, name)
        assert name in layerx_sdk.__all__, name
    for name in ("ReceiptKey", "verify_sequencer_receipt"):
        assert hasattr(layerx_sdk, name), name
        assert hasattr(verifier, name), name
        assert getattr(layerx_sdk, name) is getattr(verifier, name)
        assert name in layerx_sdk.__all__, name
    for name in (
        "decode_and_verify_program_terminal",
        "_verify_pre_runtime",
        "_bind_metadata",
        "_receipt_usage",
        "ProgramReceiptOutcome",
        "ProtocolReceipt",
    ):
        assert not hasattr(program_wire, name), name
    for name in (
        "bind_signed_program_lifecycle",
        "bind_retained_program_call",
        "DecodedSignedProgramCall",
    ):
        assert callable(getattr(program_wire, name)), name


@pytest.fixture(scope="module")
def program_execution_evidence():
    root = Path(__file__).resolve().parents[5]
    corpus = json.loads(
        (root / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
    )
    vector = next(case for case in corpus["cases"] if case["id"] == "V1-layerxprograms-call")
    pin = vector["pinnedKey"]
    key = verifier.ReceiptKey(
        bytes.fromhex(pin["keyId"][2:]), bytes.fromhex(pin["publicKey"][2:]),
        pin["firstHeight"], pin["lastHeight"],
    )
    spec = spec_from_file_location("program_receipt_signatures", root / "sdk/integrations/fastapi/layerx_fastapi/signatures.py")
    assert spec is not None and spec.loader is not None
    module = module_from_spec(spec)
    spec.loader.exec_module(module)
    return vector, key, module.LayerXSignatureVerifier()


def _verify_recorded_program(vector, key, signatures, **changes):
    transaction = bytes.fromhex(vector["rawTransaction"][2:])
    call, _ = programs.decode_signed(transaction)
    return programs._submission(
        vector["receiptJson"], signatures,
        programs.ProgramTrustContext(key.public_key, network_id=call.chain_id, receipt_keys=(key,)),
        **{
            "program_id": call.data[4:36].hex(),
            "activity_id": vector["receiptJson"]["txHash"][2:],
            "retained_signed_activity": transaction.hex(),
            **changes,
        },
    )


def test_committed_program_execution_uses_retained_verifier(program_execution_evidence):
    vector, key, signatures = program_execution_evidence
    result = _verify_recorded_program(vector, key, signatures)
    assert result == vector["receiptJson"]
    assert result["status"] == "0x1"
    assert result["to"] == "0x000000000000000000000000000000000000101d"
    assert result["logs"][0]["topics"][1] == "0x" + programs.decode_signed(
        bytes.fromhex(vector["rawTransaction"][2:])
    )[0].data[4:36].hex()
    verified = verifier.verify_sequencer_receipt(
        bytes.fromhex(result["receipt"][2:]), bytes.fromhex(vector["rawTransaction"][2:]), (key,)
    )
    assert verified.receipt.result_code == 0
    assert verified.height == int(result["height"], 16)
    assert verified.receipt_digest.hex() == vector["receiptDigest"][2:]


def test_program_execution_refuses_tampered_signature_and_transaction(program_execution_evidence):
    vector, key, signatures = program_execution_evidence
    tampered = deepcopy(vector)
    signed = bytes.fromhex(tampered["receiptJson"]["receipt"][2:])
    tampered["receiptJson"]["receipt"] = "0x" + (signed[:-1] + bytes([signed[-1] ^ 1])).hex()
    with pytest.raises(verifier.SequencerReceiptVerificationError) as raised:
        _verify_recorded_program(tampered, key, signatures)
    assert raised.value.check == "bad_signature"
    raw = bytes.fromhex(vector["rawTransaction"][2:])
    with pytest.raises(verifier.SequencerReceiptVerificationError) as raised:
        _verify_recorded_program(vector, key, signatures, retained_signed_activity=(raw[:-1] + bytes([raw[-1] ^ 1])).hex())
    assert raised.value.check == "hash_mismatch"


@pytest.mark.parametrize("field,value", (
    ("seq", "0x6"), ("height", "0x3eb"), ("nonce", "0x5"),
    ("status", "0x0"), ("timestamp", "0x1"),
    ("txHash", "0x" + "11" * 32), ("keyId", "0x" + "11" * 32),
    ("from", "0x" + "11" * 20), ("to", "0x" + "11" * 20),
    ("returnData", "0x00"), ("gasUsed", "0x1"), ("feeAmount", "0x0"),
    ("signingKey", "0x" + "11" * 32),
))
def test_program_execution_binds_the_receipt_view(program_execution_evidence, field, value):
    vector, key, signatures = program_execution_evidence
    tampered = deepcopy(vector)
    tampered["receiptJson"][field] = value
    with pytest.raises(ValueError, match="binding failed"):
        _verify_recorded_program(tampered, key, signatures)


def test_program_execution_binds_logs_and_expected_program(program_execution_evidence):
    vector, key, signatures = program_execution_evidence
    tampered = deepcopy(vector)
    tampered["receiptJson"]["logs"][0]["data"] = "0x00"
    with pytest.raises(ValueError, match="program log binding failed"):
        _verify_recorded_program(tampered, key, signatures)
    with pytest.raises(ValueError, match="program transaction binding failed"):
        _verify_recorded_program(vector, key, signatures, program_id="11" * 32)
    with pytest.raises(ValueError, match="program sequencer receipt binding failed"):
        _verify_recorded_program(vector, key, signatures, activity_id="11" * 32)
    with pytest.raises(ValueError, match="retained signed transaction required"):
        _verify_recorded_program(vector, key, signatures, retained_signed_activity=None)


@pytest.mark.parametrize("change,check", (
    ({"key_id": b"\x11" * 32}, "unpinned_key"),
    ({"first_height": 1003}, "key_out_of_range"),
    ({"last_height": 1001}, "key_out_of_range"),
    ({"public_key": b"\x11" * 32}, "bad_signature"),
))
def test_program_execution_requires_pinned_receipt_keys(program_execution_evidence, change, check):
    vector, key, signatures = program_execution_evidence
    with pytest.raises(verifier.SequencerReceiptVerificationError) as raised:
        _verify_recorded_program(vector, replace(key, **change), signatures)
    assert raised.value.check == check


def test_program_receipt_calls_and_declarations_stay_retired():
    retired = {"verify_program_receipt", "verify_program_lifecycle_receipt",
               "AuthorizedReceiptBatch", "PaymentCommitmentEvidence",
               "_SequencerAuthorization", "rpc_batch_evidence"}
    for path in Path(programs.__file__).parent.glob("*.py*"):
        if path.suffix not in {".py", ".pyi"}:
            continue
        tree = ast.parse(path.read_text())
        for node in ast.walk(tree):
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
                assert node.name not in retired, (path, node.name)
            elif isinstance(node, ast.Name):
                assert node.id not in retired, (path, node.id)
            elif isinstance(node, ast.Attribute):
                assert node.attr not in retired, (path, node.attr)
            elif isinstance(node, ast.ImportFrom):
                assert not retired.intersection(alias.name for alias in node.names), path


def test_execution_receipt_adapter_uses_the_retained_verifier(program_execution_evidence):
    vector, key, _ = program_execution_evidence
    raw = bytes.fromhex(vector["rawTransaction"][2:])
    signed = bytes.fromhex(vector["signedReceipt"][2:])
    verified = verifier.verify_sequencer_receipt(signed, raw, (key,))
    receipt = verified.receipt
    execution = {
        "state": "executed", "receipt": signed.hex(),
        "program_id": programs.decode_signed(raw)[0].data[4:36].hex(),
        "activity_id": receipt.activity_id.hex(), "batch_id": receipt.batch_id.hex(),
        "global_sequence": str(receipt.global_sequence), "module_version": receipt.module_version,
        "result_code": receipt.result_code, "state_root": receipt.resulting_state_root.hex(),
        "receipt_digest": verified.receipt_digest.hex(),
        "authority": {
            "batch_id": receipt.batch_id.hex(), "asset": receipt.asset.hex(),
            "previous_state_root": receipt.previous_state_root.hex(),
            "resulting_state_root": receipt.resulting_state_root.hex(),
            "sequencer_public_key": key.public_key.hex(),
        },
    }
    trust = programs.ProgramTrustContext(key.public_key, network_id=125, receipt_keys=(key,))
    assert programs._verify_execution_receipt(execution, trust, expected_signed_activity=raw) == verified
    with pytest.raises(ValueError, match="program execution receipt binding failed"):
        programs._verify_execution_receipt(
            {**execution, "state_root": "11" * 32}, trust, expected_signed_activity=raw
        )
    with pytest.raises(ValueError, match="program execution receipt binding failed"):
        programs._verify_execution_receipt(
            {**execution, "result_code": 1}, trust, expected_signed_activity=raw
        )
    with pytest.raises(ValueError, match="retained program transaction mismatch"):
        programs._verify_execution_receipt(
            {**execution, "retained_signed_activity": "11"}, trust, expected_signed_activity=raw
        )
    with pytest.raises(verifier.SequencerReceiptVerificationError) as raised:
        programs._verify_execution_receipt(
            execution, replace(trust, receipt_keys=()), expected_signed_activity=raw
        )
    assert raised.value.check == "unpinned_key"
    with pytest.raises(ValueError, match="program transaction binding failed"):
        programs._verify_execution_receipt(
            execution, replace(trust, network_id=126), expected_signed_activity=raw
        )


def test_lifecycle_recovery_uses_retained_receipt_verification(program_execution_evidence):
    vector, key, signatures = program_execution_evidence
    raw = bytes.fromhex(vector["rawTransaction"][2:])
    activity = vector["receiptJson"]["txHash"][2:]
    result = {"activity_id": activity, "receipt": vector["signedReceipt"][2:]}
    verified = programs.verify_lifecycle_recovery(
        result, activity, key.public_key, signatures,
        signed_activity=raw, receipt_keys=(key,),
    )
    assert verified.receipt.activity_id.hex() == activity
    assert verified.receipt.result_code == 0
    signed = bytes.fromhex(result["receipt"])
    with pytest.raises(verifier.SequencerReceiptVerificationError) as raised:
        programs.verify_lifecycle_recovery(
            {**result, "receipt": (signed[:-1] + bytes([signed[-1] ^ 1])).hex()},
            activity, key.public_key, signatures, signed_activity=raw, receipt_keys=(key,),
        )
    assert raised.value.check == "bad_signature"



def test_retained_receipt_and_commit_verification_are_public(program_execution_evidence):
    from test_repair_22_31_147 import commit_producer, finality_bundle
    vector, key, _ = program_execution_evidence
    raw = bytes.fromhex(vector["rawTransaction"][2:])
    receipt = bytes.fromhex(vector["signedReceipt"][2:])
    verified = layerx_sdk.verify_sequencer_receipt(receipt, raw, (key,))
    produced = commit_producer.__wrapped__()(raw, verified.height + 1)
    trust, evidence = finality_bundle(produced)
    committed = trust.verify(raw, verified.height, evidence)
    assert committed.transaction_hash == verified.receipt.activity_id
    assert committed.block_hash.hex() == produced["header_hash"]
    assert committed.height == verified.height + 1
    assert verify_committed_transaction(
        raw, verified.height, **evidence, chain_id=produced["header"]["chain_id"],
        trusted_validators=tuple(produced["validators"]),
    ) == committed
    for transaction in (raw + b"\0", raw[:-1]):
        with pytest.raises(ValueError, match="inclusion identity"):
            trust.verify(transaction, verified.height, evidence)
