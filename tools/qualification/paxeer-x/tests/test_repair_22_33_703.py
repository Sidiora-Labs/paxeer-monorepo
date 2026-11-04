"""mcp-recovery-rung-contract: real qualifier -> HTTP -> committed dispatch recording."""

from contextlib import contextmanager
from copy import deepcopy
import importlib.util
import hashlib
import json
from pathlib import Path
import sys

import pytest


QUAL = Path(__file__).resolve().parents[1]
ROOT = QUAL.parents[2]
sys.path.insert(0, str(QUAL / "fixtures"))
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
from mcp_recovery_recording import (
    CREDENTIAL,
    RECORDING,
    RecordedAgentd,
    producer_recording,
    recorded_phase,
)
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt


@pytest.fixture(scope="module")
def qualifier():
    spec = importlib.util.spec_from_file_location(
        "repair_22_33_703_qualifier", QUAL / "mcp_daemon_revocation.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


@contextmanager
def serving(qualifier, tmp_path):
    peer = RecordedAgentd(tmp_path / "peer")
    phase = recorded_phase(qualifier, peer, tmp_path / "evidence")
    peer.start()
    try:
        status, response = phase.rpc(peer.envelope("submit"))
        assert status == 200
        assert response["value"] == peer.recording["pending"]["submission"]
        assert response["verification_status"] == {"state": "achieved", "rung": "submitted"}
        yield peer, phase
    finally:
        peer.stop()


def test_committed_recording_is_the_real_producers_signed_receipt(qualifier):
    recording = producer_recording()
    key = recording["receipt_key"]
    verified = verify_sequencer_receipt(
        bytes.fromhex(recording["committed"]["receipt"]["canonical_bytes"]),
        bytes.fromhex(recording["raw_transaction"]),
        (ReceiptKey(bytes.fromhex(key["keyId"][2:]), bytes.fromhex(key["publicKey"][2:]),
                    int(key["firstHeight"], 16), int(key["lastHeight"], 16)),),
    )
    facts = qualifier.READINESS.receipt_facts(
        verified.canonical_bytes.hex(), recording["pending"]["activity_id"]
    )
    assert verified.level == "sequencer-signed"
    assert verified.receipt.activity_id.hex() == recording["pending"]["activity_id"]
    assert facts["receipt_sha256"] == hashlib.sha256(verified.canonical_bytes).hexdigest()
    signed = verified.canonical_bytes
    # Canonical Some(signature): one flag byte, the u32 length, then 64 signature bytes.
    assert signed[-69:-64] == b"\x01\x00\x00\x00\x40"
    unsigned = signed[:-69] + b"\0"
    domain = b"LXP/v1/receipt\0"
    derived = hashlib.sha256(domain + unsigned).hexdigest()
    recorded = recording["committed"]["submission"]["receipt_ref"]
    diagnostic = json.dumps({
        "derived": derived, "recorded": recorded,
        "producer_digest": verified.receipt_digest.hex(),
        "domain_hex": domain.hex(), "unsigned_canonical_hex": unsigned.hex(),
        "signed_canonical_hex": signed.hex(),
        "recorded_submission": recording["committed"]["submission"],
    }, sort_keys=True)
    assert derived == verified.receipt_digest.hex() == recorded, diagnostic
    assert derived != hashlib.sha256(domain + signed).hexdigest()
    header = recording["committed"]["receipt"]["authorised_batch"]
    assert header["sequencer_public_key"] == key["publicKey"][2:]
    assert header["previous_state_root"] == facts["previous_state_root"]
    assert header["resulting_state_root"] == facts["resulting_state_root"]


def test_unknown_recovers_committed_receipt_across_stop_and_reopen(qualifier, tmp_path):
    with serving(qualifier, tmp_path) as (peer, phase):
        before = peer.state.read_bytes()
        result = phase.unknown()
        reference = peer.recording["pending"]["activity_id"]
        assert result == {
            "activity_id": reference, "state": "Committed", "submission_ref": reference
        }
        assert phase.unknown_before == peer.recording["pending"]["submission"]
        assert peer.starts == 2 and peer.stops == 1
        assert peer.state.read_bytes() == before
        assert json.loads(before)["transmissions"] == 1
        assert [request["operation"] for request in peer.requests] == [
            "submit", "track", "track", "program.receipt"
        ]
        assert all(request["credential"] == CREDENTIAL for request in peer.requests)
        transcript = json.loads(RECORDING.read_text())
        exchanges = [json.loads(path.read_text()) for path in sorted(phase.directory.glob("rpc-*.json"))]
        assert exchanges == transcript["exchanges"]
        assert peer.requests == transcript["requests"]
        assert result == transcript["recovered"]
        peer.stop()

        reopened = RecordedAgentd(peer.directory)
        reopened.start()
        try:
            restored = recorded_phase(qualifier, reopened, tmp_path / "reopened-evidence")
            _, tracked = restored.rpc(reopened.envelope("unknown_track"), verified=True)
            submission = qualifier.tracked_evidence(tracked, reference)
            _, receipt = restored.rpc(reopened.envelope("unknown_receipt"), verified=True)
            facts = qualifier.recovered_receipt(receipt, submission)
            assert facts["receipt_ref"] == submission["receipt_ref"]
            assert reopened.state.read_bytes() == before
            assert [request["operation"] for request in reopened.requests] == ["track", "program.receipt"]
        finally:
            reopened.stop()
    responses = [json.loads(path.read_text()) for path in (tmp_path / "evidence").glob("rpc-*.json")]
    assert len(responses) == 4
    assert all(path.stat().st_mode & 0o077 == 0 for path in (tmp_path / "evidence").iterdir())


def test_pending_restart_preserves_unknown_without_receipt_or_transmission(qualifier, tmp_path):
    with serving(qualifier, tmp_path) as (peer, phase):
        peer.recovery_stage = "pending"
        before = peer.state.read_bytes()
        assert phase.unknown()["state"] == "Unknown"
        assert peer.state.read_bytes() == before
        assert [request["operation"] for request in peer.requests] == ["submit", "track", "track"]


def test_verified_rpc_accepts_current_committed_rung(qualifier, tmp_path):
    with serving(qualifier, tmp_path) as (peer, phase):
        peer.stop()
        peer.start()
        status, response = phase.rpc(peer.envelope("unknown_track"), verified=True)
        assert status == 200
        assert response["verification_status"] == {"state": "achieved", "rung": "committed"}
        assert response["value"]["state"] == "Committed"


@pytest.mark.parametrize("verification", [
    {"state": "achieved", "level": "StateProven"},
    {"state": "achieved", "level": "CheckpointFinalised"},
    {"state": "achieved", "rung": "submitted"},
    {"state": "achieved", "rung": "StateProven"},
    {"state": "pending", "requested": "committed", "achieved": "receipted"},
    {"state": "unverified", "requested": "committed", "achieved": "receipted"},
    {"state": "absent", "requested": "committed"},
    {"state": "achieved", "rung": ["committed"]},
    None,
])
def test_verified_rpc_rejects_retired_pending_and_unverified_rungs(qualifier, tmp_path, verification):
    with serving(qualifier, tmp_path) as (peer, phase):
        def change(operation, stage, response):
            response["verification_status"] = deepcopy(verification)
        peer.transform = change
        with pytest.raises(qualifier.READINESS.HTTP.Failure, match="protocol evidence"):
            phase.rpc(peer.envelope("unknown_track"), verified=True)
        assert json.loads(peer.state.read_text())["transmissions"] == 1


@pytest.mark.parametrize("field", ["tenant", "session_id", "token_id", "generation"])
def test_receipt_lookup_keeps_every_credential_coordinate(qualifier, tmp_path, field):
    with serving(qualifier, tmp_path) as (peer, phase):
        peer.lookup_credential[field] = "foreign" if field == "tenant" else "99"
        with pytest.raises(qualifier.READINESS.HTTP.Failure, match="tenant, invocation or requested rung"):
            phase.unknown()
        assert [request["operation"] for request in peer.requests] == ["submit", "track", "track"]


@pytest.mark.parametrize("change", [
    "transaction", "reference", "digest", "missing-digest", "rung", "state", "request-id"
])
def test_restart_rejects_misbound_submission(qualifier, tmp_path, change):
    with serving(qualifier, tmp_path) as (peer, phase):
        def corrupt(operation, stage, response):
            if operation != "track" or stage != "committed":
                return
            submission = response["value"]
            if change == "transaction":
                submission["tx_hash"] = "99" * 32
            elif change == "reference":
                submission["submission_ref"] = "99" * 32
            elif change == "digest":
                submission["receipt_ref"] = "99" * 32
            elif change == "missing-digest":
                del submission["receipt_ref"]
            elif change == "rung":
                submission["rung"] = "receipted"
            elif change == "state":
                submission["state"] = "Acknowledged"
            else:
                response["request_id"] = "9999"
        peer.transform = corrupt
        with pytest.raises(qualifier.READINESS.HTTP.Failure):
            phase.unknown()
        assert json.loads(peer.state.read_text())["transmissions"] == 1


@pytest.mark.parametrize("change", [
    "missing", "transaction", "canonical-identity", "truncated", "receipt-rung", "status-rung",
    "retired-level", "retired-receipt-level", "header-missing", "header-before", "header-after",
    "header-key", "header-batch", "header-asset", "request-id", "signature-framing",
])
def test_recovery_rejects_missing_or_misbound_receipt_and_header(qualifier, tmp_path, change):
    with serving(qualifier, tmp_path) as (peer, phase):
        def corrupt(operation, stage, response):
            if operation != "program.receipt":
                return
            receipt = response["value"]["receipt"]
            if change == "missing":
                response["value"] = {"found": False}
            elif change == "transaction":
                receipt["tx_hash"] = "99" * 32
            elif change == "canonical-identity":
                raw = bytearray.fromhex(receipt["canonical_bytes"])
                raw[10] ^= 1
                receipt["canonical_bytes"] = raw.hex()
            elif change == "truncated":
                receipt["canonical_bytes"] = receipt["canonical_bytes"][:20]
            elif change == "signature-framing":
                raw = bytearray.fromhex(receipt["canonical_bytes"])
                raw[-69] = 0
                receipt["canonical_bytes"] = raw.hex()
            elif change == "receipt-rung":
                receipt["rung"] = "receipted"
            elif change == "status-rung":
                response["verification_status"]["rung"] = "receipted"
            elif change == "retired-level":
                response["verification_status"] = {"state": "achieved", "level": "StateProven"}
            elif change == "retired-receipt-level":
                receipt["verification_level"] = receipt.pop("rung")
            elif change == "header-missing":
                del receipt["authorised_batch"]
            elif change == "header-before":
                receipt["authorised_batch"]["previous_state_root"] = "99" * 32
            elif change == "header-after":
                receipt["authorised_batch"]["resulting_state_root"] = "99" * 32
            elif change == "header-key":
                receipt["authorised_batch"]["sequencer_public_key"] = "00" * 32
            elif change == "header-batch":
                receipt["authorised_batch"]["batch_id"] = "00" * 32
            elif change == "header-asset":
                receipt["authorised_batch"]["asset"] = "00" * 32
            else:
                response["request_id"] = "9999"
        peer.transform = corrupt
        before = peer.state.read_bytes()
        with pytest.raises(qualifier.READINESS.HTTP.Failure):
            phase.unknown()
        assert peer.state.read_bytes() == before
        assert [request["operation"] for request in peer.requests].count("submit") == 1


def test_real_http_transport_refuses_cross_tenant_tracking(qualifier, tmp_path):
    with serving(qualifier, tmp_path) as (peer, phase):
        request = peer.envelope("unknown_track")
        request["credential"]["tenant"] = "foreign-tenant"
        status, response = phase.rpc(request)
        assert status == 403
        assert response["class"] == "PolicyRefusal"
        assert "value" not in response
        assert json.loads(peer.state.read_text())["transmissions"] == 1
