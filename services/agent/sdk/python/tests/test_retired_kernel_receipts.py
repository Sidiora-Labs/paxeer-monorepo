from importlib.util import find_spec
from typing import get_type_hints

import layerx_sdk
import layerx_sdk.ai_market as ai_market
import layerx_sdk.program_wire as program_wire
import layerx_sdk.programs as programs
import layerx_sdk.verifier as verifier
import layerx_sdk.x402 as x402


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
    )
    for name in retired:
        assert not hasattr(layerx_sdk, name), name
        assert not hasattr(verifier, name), name
        assert name not in layerx_sdk.__all__, name
    assert not hasattr(programs, "verify_program_receipt")
    assert not hasattr(ai_market, "CheckpointVerification")
    assert not hasattr(ai_market.SnapshotBinding, "require_checkpoint")
    assert not hasattr(ai_market.OperationJournal, "finalize")
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
    assert "checkpoint" not in x402.PaymentCommitmentEvidence.__dataclass_fields__
    assert get_type_hints(x402.PaymentCommitmentEvidence)["proof"] is verifier.MerkleProof
    for name in ("verify_sequencer_receipt", "verify_payment_receipt"):
        assert callable(getattr(x402, name)), name
    for name in ("PaymentCommitmentEvidence", "verify_payment_receipt"):
        assert getattr(layerx_sdk, name) is getattr(x402, name)
        assert name in layerx_sdk.__all__, name
    for name in ("AuthorizedReceiptBatch", "ReceiptKey", "verify_sequencer_receipt"):
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
