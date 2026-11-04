"""Repair 22.33.1509, finding python-retired-contract.

Proofs: test_python_retired_contract_integration, test_daemon_type2_recovery,
test_daemon_three_rung_decoder, and test_retired_kernel_frames_are_refused.
Baseline: not executed; the ordered bounded-gate instructions prohibit a
pre-implementation verify. No baseline failure or daemon execution is claimed.
Wiring request: spec/layerx-merge/spec.kvx, owning domain 9 / task runner,
record this finding-to-test mapping and the final gate revision/evidence.
The servers below are contract fixtures; the SDK codecs and clients are real.
"""

from dataclasses import replace
import json

import pytest

from layerx_sdk import IdempotencyKey, PlatformSdkError, SdkErrorCode
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import EvmTransactionError, decode_signed

import test_integration as integration


@pytest.mark.parametrize("method", [
    "test_agent_http_transport_preserves_program_contract_and_authentication",
    "test_program_call_ambiguity_retains_canonical_signed_evidence",
    "test_agent_http_transport_never_follows_redirects",
])
def test_python_retired_contract_integration(method):
    case = integration.PythonSdkIntegration(method)
    getattr(case, method)()


def test_daemon_type2_recovery(tmp_path):
    request = integration.prepared_program_call()
    raw = request.signed_activity
    transaction, signature = decode_signed(raw)
    assert transaction.chain_id == 125 and raw[0] == 2
    assert transaction.value == 0
    assert transaction.to.hex() == "000000000000000000000000000000000000101d"
    assert transaction.data == integration._encode_programs_call(request.native_call, request.fee_limit)
    assert transaction.attach_signature(signature, transaction.recover_signer(signature)) == raw
    identity = keccak256(raw).hex()
    retained = {"tx_hash": identity, "raw_transaction": "0x" + raw.hex(), "idempotency_key": identity}
    journal = tmp_path / "signed-submission.json"
    journal.write_text(json.dumps(retained))
    transactions, submissions = {}, []

    def dispatch(path, headers, body):
        integration.check_daemon_request(path, headers, body)
        if body["operation"] == "submit":
            assert set(body["request"]) == {"raw_transaction"}
            assert body["request"] == {"raw_transaction": retained["raw_transaction"]}
            assert body["idempotency_key"] == identity
            received = bytes.fromhex(body["request"]["raw_transaction"][2:])
            decoded, received_signature = decode_signed(received)
            assert decoded == transaction and received_signature == signature
            assert decoded.recover_signer(received_signature) == transaction.recover_signer(signature)
            assert keccak256(received).hex() == identity
            submissions.append(received)
            transactions[identity] = received
            if len(submissions) == 1:
                return 0, None
        else:
            assert body["operation"] == "track"
            assert body["request"] == {"submission_ref": identity}
            assert body["idempotency_key"] is None
            assert transactions[identity] == raw
        return 200, {
            "request_id": body["request_id"],
            "value": integration.submission_status(identity),
            "verification_status": {"state": "achieved", "rung": "submitted"},
        }

    server_tls, client_tls = integration.daemon_tls(tmp_path)
    with integration.integration_server(dispatch, tls=server_tls) as (endpoint, before):
        client = integration.daemon_client(endpoint, client_tls)
        with pytest.raises(PlatformSdkError) as error:
            client.agent("submit", {"raw_transaction": retained["raw_transaction"]},
                         idempotency_key=IdempotencyKey(identity))
        assert error.value.code == SdkErrorCode.UNKNOWN_OUTCOME
        assert error.value.retry == "unknown-outcome"
        assert transactions == {identity: raw}
        assert len(before) == 1
        del client, request

    recovered = json.loads(journal.read_text())
    assert recovered == retained
    assert bytes.fromhex(recovered["raw_transaction"][2:]) == raw
    assert recovered["tx_hash"] == keccak256(raw).hex()
    with integration.integration_server(dispatch, tls=server_tls) as (endpoint, after):
        client = integration.daemon_client(endpoint, client_tls)
        tracked = client.agent("track", {"submission_ref": recovered["tx_hash"]})
        assert tracked.value == integration.submission_status(identity)
        assert tracked.verification_status == {"state": "achieved", "rung": "submitted"}
        submitted = client.agent("submit", {"raw_transaction": recovered["raw_transaction"]},
                                 idempotency_key=IdempotencyKey(recovered["idempotency_key"]))
        assert submitted.value == tracked.value
        assert submitted.verification_status == tracked.verification_status
        assert [row[2]["operation"] for row in before + after] == ["submit", "track", "submit"]
    assert submissions == [raw, raw]
    assert transactions == {identity: raw}
    assert journal.read_text() == json.dumps(retained)


@pytest.mark.parametrize("rung", ["submitted", "receipted", "committed"])
def test_daemon_three_rung_decoder(tmp_path, rung):
    identity = "44" * 32
    expected = integration.submission_status(identity)
    expected["rung"] = rung
    expected["state"] = {"submitted": "Submitted", "receipted": "Executed", "committed": "Committed"}[rung]
    if rung != "submitted":
        expected["receipt_ref"] = "55" * 32

    def dispatch(path, headers, body):
        integration.check_daemon_request(path, headers, body)
        assert body["operation"] == "track"
        assert body["request"] == {"submission_ref": identity}
        assert body["idempotency_key"] is None
        return 200, {"request_id": body["request_id"], "value": expected,
                     "verification_status": {"state": "achieved", "rung": rung}}

    server_tls, client_tls = integration.daemon_tls(tmp_path)
    with integration.integration_server(dispatch, tls=server_tls) as (endpoint, observed):
        response = integration.daemon_client(endpoint, client_tls).agent("track", {"submission_ref": identity})
        assert response.value == expected
        assert response.verification_status == {"state": "achieved", "rung": rung}
        assert len(observed) == 1


@pytest.mark.parametrize("retired", ["sequencer-signed", "anchored", "finalised", "finalized"])
@pytest.mark.parametrize("variant", ["achieved", "requested"])
def test_daemon_retired_verification_vocabulary_is_refused(tmp_path, retired, variant):
    def dispatch(path, headers, body):
        integration.check_daemon_request(path, headers, body)
        assert body["operation"] == "read.balance"
        assert body["request"]["requested_verification_level"] == "committed"
        verification = {"state": "achieved", "rung": retired} if variant == "achieved" else {
            "state": "unverified", "requested": retired, "achieved": "submitted",
            "reason": "evidence_unavailable",
        }
        return 200, {"request_id": body["request_id"], "value": integration.daemon_balance(body["request"]),
                     "verification_status": verification}

    server_tls, client_tls = integration.daemon_tls(tmp_path)
    with integration.integration_server(dispatch, tls=server_tls) as (endpoint, observed):
        with pytest.raises(PlatformSdkError) as error:
            integration.daemon_client(endpoint, client_tls).agent("read.balance", {
                "account": "11" * 32, "asset": "33" * 32,
                "requested_verification_level": "committed",
            })
        assert error.value.code == SdkErrorCode.VERIFICATION_FAILURE
        assert error.value.request_id == observed[0][2]["request_id"]
        assert len(observed) == 1


def test_retired_kernel_frames_are_refused():
    request = integration.prepared_program_call()
    legacy = integration.retired_kernel_program_call(request.program_id, "33" * 32)
    assert legacy[:2] == b"\x00\x01"
    assert legacy[2:4] == b"\x10\x01"
    with pytest.raises(EvmTransactionError) as error:
        decode_signed(legacy)
    assert error.value.code == "invalid_transaction"

    def unexpected_request(path, headers, body):
        raise AssertionError("retired kernel frame reached the network")

    with integration.integration_server(unexpected_request) as (endpoint, observed):
        operations = integration.program_operations(endpoint)
        for invalid in (replace(request, signed_activity=legacy), integration.ProgramCall(
            request.program_id, b"\xaa", 1, 0, (), legacy,
        )):
            with pytest.raises(EvmTransactionError) as error:
                operations.submit(invalid, IdempotencyKey("33" * 32))
            assert error.value.code == "invalid_transaction"
        assert observed == []
