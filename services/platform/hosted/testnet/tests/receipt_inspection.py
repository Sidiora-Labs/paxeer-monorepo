import argparse
import copy
import hashlib
import json
from pathlib import Path
import re
import ssl
import struct
import sys
import urllib.parse
import urllib.request

from journey_common import NoRedirect, RpcError, write

sys.path.insert(0, str(Path(__file__).resolve().parents[5] / "services/agent/sdk/python"))
from layerx_sdk.verifier import SequencerReceiptVerificationError, verify_sequencer_receipt
from layerx_sdk.x402_rpc import decode_receipt_keys


def hex_bytes(value, size=None):
    assert isinstance(value, str) and value.startswith("0x"), "invalid node hex"
    decoded = bytes.fromhex(value[2:])
    assert size is None or len(decoded) == size, "invalid node hex size"
    return decoded


def quantity(value):
    assert isinstance(value, str) and re.fullmatch(
        r"0x(?:0|[1-9a-fA-F][0-9a-fA-F]{0,15})", value
    ), "invalid node quantity"
    return int(value, 16)


class NodeReceiptRpc:
    def __init__(self, endpoint, ca, output):
        url = urllib.parse.urlsplit(endpoint)
        assert (
            url.scheme == "https" and url.hostname and url.path in ("", "/")
            and not url.username and not url.password and not url.query and not url.fragment
        ), "receipt inspection requires the HTTPS root node endpoint"
        self.endpoint = endpoint.rstrip("/") + "/"
        self.output = output
        self.counter = 0
        self.opener = urllib.request.build_opener(
            NoRedirect(), urllib.request.HTTPSHandler(context=ssl.create_default_context(cafile=str(ca)))
        )

    def receipt(self, transaction_hash):
        self.counter += 1
        body = json.dumps({
            "jsonrpc": "2.0", "id": self.counter,
            "method": "pax_sequencerReceipt", "params": [transaction_hash],
        }).encode()
        write(self.output / f"{self.counter:02d}-request.json", body)
        request = urllib.request.Request(
            self.endpoint, data=body, headers={"Content-Type": "application/json"}, method="POST"
        )
        with self.opener.open(request, timeout=30) as response:
            raw = response.read(8388609)
            assert len(raw) <= 8388608, "oversized node response"
        write(self.output / f"{self.counter:02d}-response.json", raw)
        value = json.loads(raw)
        assert value.get("jsonrpc") == "2.0" and type(value.get("id")) is int
        assert value["id"] == self.counter and (("error" in value) != ("result" in value))
        if "error" in value:
            raise RpcError("pax_sequencerReceipt", value["error"])
        return value["result"]


def verify_node_receipt(result, raw_transaction, receipt_keys, transaction_hash):
    assert isinstance(result, dict), "missing node receipt"
    assert result["rung"] == "committed", "payment has not reached the committed rung"
    verified = verify_sequencer_receipt(
        hex_bytes(result["receipt"]), raw_transaction, receipt_keys
    )
    receipt = verified.receipt
    assert receipt.result_code == 0 and quantity(result["status"]) == 1, "payment failed"
    assert hex_bytes(transaction_hash, 32) == receipt.activity_id, "payment binding mismatch"
    assert hex_bytes(result["txHash"], 32) == receipt.activity_id, "txHash mismatch"
    assert quantity(result["nonce"]) == receipt.from_sequence, "nonce mismatch"
    assert quantity(result["seq"]) == receipt.global_sequence, "sequence mismatch"
    assert quantity(result["height"]) == verified.height, "height mismatch"
    assert quantity(result["timestamp"]) == receipt.timestamp, "timestamp mismatch"
    assert hex_bytes(result["keyId"], 32) == receipt.context_hash, "keyId mismatch"
    assert any(
        key.key_id == receipt.context_hash
        and key.first_height <= verified.height <= key.last_height
        and key.public_key == hex_bytes(result["signingKey"], 32)
        for key in receipt_keys
    ), "signingKey differs from the independent receipt key"
    assert receipt.from_account == hex_bytes(result["from"], 20).rjust(32, b"\0"), "from mismatch"
    assert receipt.to_account == hex_bytes(result["to"], 20).rjust(32, b"\0"), "to mismatch"
    assert receipt.asset == hashlib.sha256(result["paymentDenom"].encode()).digest(), "paymentDenom mismatch"
    assert receipt.fee_charged == int(result["feeAmount"], 16), "fee mismatch"
    assert quantity(result["blockNumber"]) >= verified.height, "block predates receipt"
    assert any(hex_bytes(result["blockHash"], 32)), "missing committed block hash"
    returned = hex_bytes(result["returnData"])
    assert receipt.activity_root == hashlib.sha256(returned).digest(), "returnData mismatch"
    fee_denom = result["feeDenom"].encode()
    assert 0 < len(fee_denom) <= 64
    summary = (
        struct.pack(">QQQI", quantity(result["gasUsed"]), quantity(result["cumulativeGasUsed"]),
                    quantity(result["effectiveGasPrice"]), quantity(result["index"]))
        + hex_bytes(result["contractAddress"], 20) + hex_bytes(result["sponsor"], 20)
        + hashlib.sha256(result["vmError"].encode()).digest()
        + struct.pack(">II", len(returned), len(result["logs"]))
        + bytes([len(fee_denom)]) + fee_denom
    )
    assert len(receipt.effects) == len(result["logs"]) + 1, "log count mismatch"
    for index, effect in enumerate(receipt.effects):
        assert effect.module_id == receipt.module_id and effect.ordinal == index
        assert not effect.monetary and effect.event_type == 0 and effect.transfer_set_root == bytes(32)
        if index == 0:
            assert effect.kind == 1 and effect.body == summary, "execution summary mismatch"
        else:
            log = result["logs"][index - 1]
            assert quantity(log["logIndex"]) == index - 1, "log index mismatch"
            assert len(log["topics"]) <= 4
            body = (
                hex_bytes(log["address"], 20) + bytes([len(log["topics"])])
                + b"".join(hex_bytes(topic, 32) for topic in log["topics"])
                + hashlib.sha256(hex_bytes(log["data"])).digest()
            )
            assert effect.kind == 3 and effect.body == body, "log mismatch"
    return verified


def inspect_payment(client, payment_directory, receipt_keys):
    payment = json.loads((payment_directory / "result.json").read_text())
    stored = json.loads((payment_directory / "receipt.json").read_text())["result"]
    raw_transaction = (payment_directory / "payment.raw").read_bytes()
    transaction_hash = payment["activity_id"]
    assert payment["replayed"] is True and payment["replay_refusal"] == {
        "code": -32000, "message": "already known"
    }, "payment replay was not refused"
    assert payment["result"] == stored, "payment artifacts disagree"
    verify_node_receipt(stored, raw_transaction, receipt_keys, transaction_hash)
    result = client.receipt(transaction_hash)
    verified = verify_node_receipt(result, raw_transaction, receipt_keys, transaction_hash)
    assert result == stored, "node receipt differs from the payment evidence"
    corrupt = copy.deepcopy(result)
    damaged = bytearray(verified.canonical_bytes)
    damaged[-1] ^= 1
    corrupt["receipt"] = "0x" + damaged.hex()
    try:
        verify_node_receipt(corrupt, raw_transaction, receipt_keys, transaction_hash)
    except SequencerReceiptVerificationError as error:
        assert error.check == "bad_signature", "corruption failed at the wrong check"
    else:
        raise AssertionError("corrupted node receipt was accepted")
    assert client.receipt(transaction_hash) == result, "receipt changed after payment replay"
    return {
        "verified": True, "activity_id": transaction_hash, "rung": result["rung"],
        "verification_level": verified.level, "height": verified.height,
        "key_id": "0x" + verified.receipt.context_hash.hex(),
        "receipt_digest": "0x" + verified.receipt_digest.hex(),
        "result_code": verified.receipt.result_code, "replayed": True,
        "corruption_refused": True,
    }


def main(argv=None):
    parser = argparse.ArgumentParser()
    parser.add_argument("--node", required=True)
    for name in ("ca", "receipt-keys", "payment", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    args = parser.parse_args(argv)
    keys = decode_receipt_keys(json.loads(args.receipt_keys.read_text()))
    args.output.mkdir(mode=0o700, parents=True, exist_ok=False)
    result = inspect_payment(NodeReceiptRpc(args.node, args.ca, args.output), args.payment, keys)
    write(args.output / "verification.json", json.dumps(result, indent=2))
    print(json.dumps(result))


if __name__ == "__main__":
    main()
