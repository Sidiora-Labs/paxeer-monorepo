"""Audit 22.30.182: node-produced evidence through the real beta and SDK transports.

The recordings prove native execution and the node's recorded committed/reopen
observations. Replaying them does not run a node or independently prove a BFT
commit. The inventory check requires the current node persistence boundaries.
"""

from __future__ import annotations

import ast
from copy import deepcopy
import json
from pathlib import Path
import sys

import pytest

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed, encode_call
from layerx_sdk.verifier import (
    ReceiptKey,
    SequencerReceiptVerificationError,
    verify_sequencer_receipt,
)
from layerx_sdk.x402_rpc import PaymentRpc
from tools.qualification import beta_driver
from tools.qualification import test_beta_driver as beta_tests
from tools.qualification import test_release_runner as release_tests


TASK_RECORD = {
    "task": "22.33.864",
    "findings": {
        "beta-driver-old-emulator": [
            "test_beta_driver_old_emulator_native_pax_journey",
            "test_beta_driver_old_emulator_committed_receipt_reopen",
        ],
        "release-runner-batch-cases": [
            "test_release_runner_batch_cases_require_current_node_boundaries",
            "test_release_runner_batch_cases_refuse_missing_evidence",
        ],
    },
    "baseline": "Attempt 1 at bd517d8b1adb9cc768cd907f37fe651f025df6ab: 8 passed; test_release_runner_batch_cases_require_current_node_boundaries failed on the retired production inventories. No duplicate baseline run: the explicit gate order forbids a pre-change verify run.",
    "wiring_requests": [
        "Makefile (domain 16): replace standalone kernel make targets still used by non-crash programs qualification with merged-node targets.",
    ],
    "cli_security": "tools/platform/cli/tests/commands.rs and credential.rs are retained; the Python gate does not claim to execute Rust CLI security tests.",
}


def unhex(value: str) -> bytes:
    return bytes.fromhex(value.removeprefix("0x"))


def receipt_key(case: dict) -> ReceiptKey:
    pin = case["pinnedKey"]
    return ReceiptKey(
        unhex(pin["keyId"]), unhex(pin["publicKey"]), pin["firstHeight"], pin["lastHeight"]
    )


def verify_native(case: dict, answer: dict):
    raw = unhex(case["rawTransaction"])
    transaction, signature = decode_signed(raw)
    assert transaction.chain_id == 125
    assert transaction.value == 10**18
    assert transaction.data == b""
    assert transaction.to == unhex(answer["to"])
    assert transaction.recover_signer(signature).lower() == answer["from"].lower()
    assert "0x" + keccak256(raw).hex() == answer["txHash"]
    verified = verify_sequencer_receipt(unhex(answer["receipt"]), raw, (receipt_key(case),))
    assert verified.receipt.result_code == 0
    assert verified.receipt.activity_id == keccak256(raw)
    assert verified.receipt.from_account == unhex(answer["from"]).rjust(32, b"\0")
    assert verified.receipt.to_account == transaction.to.rjust(32, b"\0")
    assert verified.receipt.fee_charged == int(answer["feeAmount"], 16)
    assert verified.height == int(answer["height"], 16)
    assert verified.receipt_digest == unhex(case["receiptDigest"])
    assert answer["paymentDenom"] == answer["feeDenom"] == "uhpx"
    assert answer["status"] == "0x1"
    return verified


def test_beta_driver_old_emulator_native_pax_journey(tmp_path):
    peer = beta_tests.LocalPaxdStack(tmp_path)
    peer.start()
    try:
        client = PaymentRpc(peer.url)
        case = peer.native
        tx_hash = client.send(case["rawTransaction"])
        answer = client.receipt(tx_hash)
        assert answer == case["receiptJson"]
        assert verify_native(case, answer).level == "sequencer-signed"
        # The native RPC vector establishes receipted execution, not commitment.
        assert answer["rung"] == "receipted"
        peer.assert_consumed()
        assert [r["id"] for r in peer.requests] == [1, 2]
        assert [r["method"] for r in peer.requests] == [
            "eth_sendRawTransaction", "pax_sequencerReceipt"
        ]
        environment = peer.driver_environment()
        assert environment["LAYERX_NETWORK_ID"] == "125"
        assert environment["LAYERX_CURRENCY"] == "PAX"
        assert environment["LAYERX_SOURCE"] == answer["from"]
        assert environment["LAYERX_DESTINATION"] == answer["to"]
        assert "LAYERX_SEQUENCER_TRUST_ANCHOR" not in environment
        assert "LAYERX_CREDENTIAL_STORE" not in environment
    finally:
        peer.stop()


def test_beta_driver_native_pax_uses_production_case_transport(tmp_path):
    peer = beta_tests.LocalPaxdStack(tmp_path)
    peer.start()
    try:
        stack = beta_driver.Stack(peer.url, peer.url, peer.url, peer.url)
        ctx = beta_driver.CaseContext(
            "platform-qualify-adoption", "python/first-payment", tmp_path,
            stack, beta_tests.SOURCE_IDENTITY, {},
        )
        response = ctx.http_json(
            stack.node, method="POST",
            payload={"jsonrpc": "2.0", "id": 1, "method": "eth_sendRawTransaction",
                     "params": [peer.native["rawTransaction"]]},
        )
        assert response == {"jsonrpc": "2.0", "id": 1, "result": peer.native["receiptJson"]["txHash"]}
        response = ctx.http_json(
            stack.node, method="POST",
            payload={"jsonrpc": "2.0", "id": 2, "method": "pax_sequencerReceipt",
                     "params": [response["result"]]},
        )
        assert response["id"] == 2
        verified = verify_native(peer.native, response["result"])
        result_path = ctx.result(
            network_id=125, currency="PAX", rung=response["result"]["rung"],
            receipt_digest=verified.receipt_digest.hex(),
        )
        artifact = ctx.artifacts[0]
        assert artifact["kind"] == "result"
        assert beta_driver.file_digest(result_path) == artifact["sha256"]
        assert json.loads(result_path.read_text())["currency"] == "PAX"
        peer.assert_consumed()
    finally:
        peer.stop()


def observe_program(peer, phase):
    case = peer.recovery["case"]
    phase_record = peer.recovery["phases"][phase]
    peer.start(phase)
    try:
        client = PaymentRpc(peer.url)
        tx_hash = case["receiptJson"]["txHash"]
        if phase == "after_merge":
            assert client.send(case["rawTransaction"]) == tx_hash
        answer = client.receipt(tx_hash)
        raw = unhex(case["rawTransaction"])
        transaction, _ = decode_signed(raw)
        assert transaction.chain_id == 125
        verified = verify_sequencer_receipt(unhex(answer["receipt"]), raw, (receipt_key(case),))
        assert verified.receipt.result_code == 0
        assert verified.receipt_digest == unhex(case["receiptDigest"])
        assert answer == case["receiptJson"]
        assert answer["rung"] == "committed"
        status = client.status(tx_hash)
        assert status["transactionHash"] == tx_hash
        assert status["blockHash"] == answer["blockHash"]
        assert status["blockNumber"] == answer["height"] == hex(verified.height)
        assert status["status"] == "0x1"
        data = encode_call("getProgram", ["bytes32"], [bytes.fromhex(phase_record["program_id"])])
        params = [{"to": "0x000000000000000000000000000000000000101d",
                   "data": "0x" + data.hex()}, status["blockNumber"]]
        assert params == phase_record["state"]["params"]
        state = client.call("eth_call", params)
        assert state == phase_record["expected_program_state"]
        encoded = unhex(state)
        assert len(encoded) == 6 * 32
        assert encoded[32:64] != bytes(32)
        assert encoded[64:96] != bytes(32)
        assert int.from_bytes(encoded[96:128], "big") == 1
        assert encoded[160:] == bytes(31) + b"\x01"
        peer.assert_consumed()
        return answer, status, state, deepcopy(peer.requests)
    finally:
        peer.stop()


def test_beta_driver_old_emulator_committed_receipt_reopen(tmp_path):
    before = beta_tests.LocalPaxdStack(tmp_path)
    recording = before.recovery_path.read_bytes()
    assert before.recovery["merge_height"] < before.recovery["anchor_store_delete_height"]
    first = observe_program(before, "after_merge")
    assert before.server is None and before.thread is None
    # Discard the peer, reopen the committed recording, and use a new real client.
    del before
    reopened = beta_tests.LocalPaxdStack(tmp_path)
    second = observe_program(reopened, "after_anchor_delete_reopen")
    assert first[:3] == second[:3]
    assert [r["method"] for r in first[3]] == [
        "eth_sendRawTransaction", "pax_sequencerReceipt", "eth_getTransactionReceipt", "eth_call"
    ]
    assert [r["method"] for r in second[3]] == [
        "pax_sequencerReceipt", "eth_getTransactionReceipt", "eth_call"
    ]
    assert reopened.recovery_path.read_bytes() == recording


@pytest.mark.parametrize("corruption", ["signature", "key"])
def test_native_beta_receipt_refuses_corrupted_authority(tmp_path, corruption):
    peer = beta_tests.LocalPaxdStack(tmp_path)
    peer.start()
    try:
        if corruption == "signature":
            receipt = bytearray(unhex(peer.native["receiptJson"]["receipt"]))
            receipt[-1] ^= 1
            peer.native["receiptJson"]["receipt"] = "0x" + receipt.hex()
        client = PaymentRpc(peer.url)
        tx_hash = client.send(peer.native["rawTransaction"])
        answer = client.receipt(tx_hash)
        case = deepcopy(peer.native)
        if corruption == "key":
            case["pinnedKey"]["publicKey"] = "0x" + "00" * 32
        with pytest.raises(SequencerReceiptVerificationError):
            verify_native(case, answer)
        peer.assert_consumed()
    finally:
        peer.stop()


def test_cli_key_and_credential_security_cases_are_retained():
    path = ROOT / "tools/platform/cli/tests/credential.rs"
    source = path.read_text()
    for name in (
        "imported_seed_is_never_written_to_configuration",
        "configuration_file_is_owner_only",
        "created_key_reports_its_secret_storage_and_hides_the_secret",
        "api_token_is_never_written_to_configuration",
        "duplicate_key_names_are_refused_with_a_machine_readable_error",
    ):
        assert f"fn {name}()" in source
    tree = ast.parse((ROOT / "tools/qualification/test_beta_driver.py").read_text())
    assert any(isinstance(node, ast.FunctionDef) and node.name == "cli_json" for node in tree.body)


def test_release_runner_batch_cases_require_current_node_boundaries():
    release_tests.ReleaseRunnerTests().test_programs_gate_binds_local_conservation_and_live_crash_matrix()


def test_release_runner_batch_cases_refuse_missing_evidence():
    release_tests.ReleaseRunnerTests().test_programs_evidence_validation_uses_exact_inventory_and_real_files()


def test_beta_driver_refuses_to_overwrite_existing_case_evidence(tmp_path, capsys):
    output = tmp_path / "occupied"
    (output / "cases").mkdir(parents=True)
    urls = ["http://127.0.0.1:1"] * 4
    arguments = ["run", "--gate", "multichain-qualify", "--output", str(output),
                 "--source-identity", beta_tests.SOURCE_IDENTITY]
    for (_, flag), url in zip(beta_driver.STACK_FLAGS, urls):
        arguments.extend((flag, url))
    assert beta_driver.beta_driver(arguments, {}) == 2
    assert "already holds case evidence" in capsys.readouterr().err
    assert not (output / "evidence.json").exists()
