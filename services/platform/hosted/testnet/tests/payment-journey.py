import argparse
import hashlib
import json
from pathlib import Path
import re
import struct
import sys
import time

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec, utils
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
from cryptography.hazmat.primitives.serialization import load_pem_private_key

from journey_common import JourneyRpc, RpcError, fixed, protected, write

sys.path.insert(0, str(Path(__file__).resolve().parents[5] / "services/agent/sdk/python"))
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import Type2Transaction, encode_call
from layerx_sdk.verifier import _decode_protocol_receipt


BANK = "0x0000000000000000000000000000000000001001"
NATIVE_DENOM = "uhpx"
NATIVE_SCALE = 10**12
SECP256K1_ORDER = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141


def quantity(value):
    assert isinstance(value, str) and re.fullmatch(
        r"0x(?:0|[1-9a-fA-F][0-9a-fA-F]*)", value
    )
    return int(value, 16)


def hex_bytes(value, size=None):
    assert isinstance(value, str) and value.startswith("0x")
    decoded = bytes.fromhex(value[2:])
    assert size is None or len(decoded) == size
    return decoded


def verify_sequencer_receipt(receipt, sequencer, transaction_hash, source, nonce, value):
    canonical = hex_bytes(receipt["receipt"])
    decoded, unsigned = _decode_protocol_receipt(canonical)
    Ed25519PublicKey.from_public_bytes(sequencer).verify(
        decoded.sequencer_signature,
        hashlib.sha256(b"LXP/v1/receipt\0" + unsigned).digest(),
    )
    assert decoded.protocol_version == 2 and decoded.module_id > 11
    assert decoded.module_version == 1 and decoded.parameter_version == 0
    assert decoded.operation == 2 and decoded.result_code == 0
    assert decoded.activity_id == hex_bytes(transaction_hash, 32)
    assert hex_bytes(receipt["txHash"], 32) == decoded.activity_id
    assert hex_bytes(receipt["signingKey"], 32) == sequencer
    assert decoded.from_account == hex_bytes(source, 20).rjust(32, b"\0")
    assert decoded.to_account == hex_bytes(BANK, 20).rjust(32, b"\0")
    assert hex_bytes(receipt["from"], 20) == hex_bytes(source, 20)
    assert hex_bytes(receipt["to"], 20) == hex_bytes(BANK, 20)
    assert decoded.from_sequence == quantity(receipt["nonce"]) == nonce
    assert decoded.amount == value and quantity(receipt["status"]) == 1
    assert decoded.global_sequence == quantity(receipt["seq"])
    assert decoded.timestamp == quantity(receipt["timestamp"])
    assert decoded.batch_id == quantity(receipt["height"]).to_bytes(32, "big")
    assert receipt["rung"] == "committed"
    assert quantity(receipt["blockNumber"]) >= quantity(receipt["height"])
    hex_bytes(receipt["blockHash"], 32)
    assert hex_bytes(receipt["keyId"], 32) == decoded.context_hash
    assert receipt["feeDenom"] == NATIVE_DENOM
    assert decoded.asset == hashlib.sha256(NATIVE_DENOM.encode()).digest()
    assert decoded.fee_charged == quantity(receipt["feeAmount"]) > 0
    assert decoded.fee_charged == quantity(receipt["gasUsed"]) * quantity(
        receipt["effectiveGasPrice"]
    )
    assert (
        decoded.from_balance_after
        == decoded.from_balance_before - value - decoded.fee_charged
    )
    assert decoded.activity_root == hashlib.sha256(
        hex_bytes(receipt["returnData"])
    ).digest()
    assert hex_bytes(receipt["returnData"]) == (1).to_bytes(32, "big")
    assert receipt["vmError"] == ""
    summary = (
        struct.pack(
            ">QQQI",
            quantity(receipt["gasUsed"]),
            quantity(receipt["cumulativeGasUsed"]),
            quantity(receipt["effectiveGasPrice"]),
            quantity(receipt["index"]),
        )
        + hex_bytes(receipt["contractAddress"], 20)
        + hex_bytes(receipt["sponsor"], 20)
        + hashlib.sha256(receipt["vmError"].encode()).digest()
        + struct.pack(">II", len(hex_bytes(receipt["returnData"])), len(receipt["logs"]))
        + bytes([len(NATIVE_DENOM)])
        + NATIVE_DENOM.encode()
    )
    assert hex_bytes(receipt["sponsor"], 20) == bytes(20)
    assert len(decoded.effects) == len(receipt["logs"]) + 1
    for index, effect in enumerate(decoded.effects):
        assert effect.module_id == decoded.module_id and effect.ordinal == index
        assert not effect.monetary and effect.event_type == 0
        assert effect.transfer_set_root == bytes(32)
        if index == 0:
            assert effect.kind == 1 and effect.body == summary
        else:
            log = receipt["logs"][index - 1]
            body = (
                hex_bytes(log["address"], 20)
                + bytes([len(log["topics"])])
                + b"".join(hex_bytes(topic, 32) for topic in log["topics"])
                + hashlib.sha256(hex_bytes(log["data"])).digest()
            )
            assert effect.kind == 3 and effect.body == body
    return decoded


def main():
    parser = argparse.ArgumentParser()
    for name in ["gateway", "did", "destination", "asset", "amount", "sequencer-key"]:
        parser.add_argument("--" + name, required=True)
    for name in ["ca", "auth-config", "signer", "output"]:
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--source-auth-config", type=Path)
    parser.add_argument("--destination-auth-config", type=Path)
    parser.add_argument("--network-id", type=int, required=True)
    args = parser.parse_args()
    assert args.gateway.startswith("https://") and 0 < args.network_id < 2**32
    assert args.asset == NATIVE_DENOM, "native bank denomination required"
    assert args.did.startswith("did:layerx:") and args.destination.startswith("did:layerx:")
    public_key = fixed(args.did.removeprefix("did:layerx:"))
    destination_public_key = fixed(args.destination.removeprefix("did:layerx:"))
    sequencer = fixed(args.sequencer_key)
    amount = int(args.amount)
    assert 0 < amount < 2**128 // NATIVE_SCALE and str(amount) == args.amount
    seed = protected(args.signer)
    key = (
        ec.derive_private_key(int.from_bytes(seed, "big"), ec.SECP256K1())
        if len(seed) == 32
        else load_pem_private_key(seed, password=None)
    )
    assert isinstance(key, ec.EllipticCurvePrivateKey) and isinstance(
        key.curve, ec.SECP256K1
    )
    public = key.public_key().public_numbers()
    source = "0x" + keccak256(
        public.x.to_bytes(32, "big") + public.y.to_bytes(32, "big")
    )[-20:].hex()
    source_auth = (
        args.source_auth_config
        if args.source_auth_config is not None
        else args.auth_config.with_name("source.curl")
    )
    destination_auth = (
        args.destination_auth_config
        if args.destination_auth_config is not None
        else args.auth_config.with_name("destination.curl")
    )
    protected(source_auth)
    protected(destination_auth)
    args.output.mkdir(mode=0o700, parents=True, exist_ok=True)
    client = JourneyRpc(args.gateway, args.ca, args.auth_config, args.output)
    assert quantity(client.rpc("eth_chainId", [])) == args.network_id
    funding = []
    for did, public, auth in [
        (args.did, public_key, source_auth),
        (args.destination, destination_public_key, destination_auth),
    ]:
        deadline = time.monotonic() + 120
        while True:
            try:
                funded = client.rpc(
                    "px_requestFunds", [did, public.hex()], auth_config=auth
                )
                break
            except RpcError as error:
                assert error.code == -32001 and isinstance(error.data, dict), str(error)
                assert error.data.get("code") == "faucet_claim_pending", str(error)
                assert time.monotonic() < deadline, "funding did not complete"
                time.sleep(0.1)
        assert (
            funded["funded"] is True
            and funded["funding_id"]
            and funded["transaction_id"]
        )
        assert int(funded["amount"]) > 0
        funding.append(funded)

    def account(did):
        snapshot = client.rpc("px_getBalances", [did])
        assert (
            snapshot["account"]["layerx_did"] == did
            and snapshot["account"]["bound"] is True
        )
        hex_bytes(snapshot["account"]["evm_address"], 20)
        assert snapshot["account"]["pax_address"]
        assert type(snapshot["height"]) is int and snapshot["height"] >= 0
        return snapshot

    def balance(snapshot):
        matches = [coin for coin in snapshot["balances"] if coin["denom"] == args.asset]
        assert len(matches) == 1, "one funded native balance required"
        value = matches[0]["amount"]
        assert re.fullmatch(r"0|[1-9][0-9]*", value)
        return int(value)

    source_before, before = account(args.did), account(args.destination)
    assert source_before["account"]["evm_address"].lower() == source
    destination = before["account"]["evm_address"]
    assert destination.lower() != source
    source_wei_before = quantity(client.rpc("eth_getBalance", [source, "latest"]))
    destination_wei_before = quantity(client.rpc("eth_getBalance", [destination, "latest"]))
    assert source_wei_before // NATIVE_SCALE == balance(source_before)
    assert destination_wei_before // NATIVE_SCALE == balance(before)
    nonce = quantity(client.rpc("eth_getTransactionCount", [source, "pending"]))
    gas_price = quantity(client.rpc("eth_gasPrice", []))
    assert gas_price > 0 and balance(source_before) > amount
    data = encode_call("sendNative", ["string"], [before["account"]["pax_address"]])
    value = amount * NATIVE_SCALE
    gas = quantity(client.rpc("eth_estimateGas", [{
        "from": source,
        "to": BANK,
        "value": hex(value),
        "data": "0x" + data.hex(),
        "maxFeePerGas": hex(gas_price * 2),
        "maxPriorityFeePerGas": "0x0",
    }]))
    transaction = Type2Transaction(
        args.network_id, nonce, 0, gas_price * 2, gas, hex_bytes(BANK, 20), value, data
    )
    digest = transaction.signing_digest()
    r, s = utils.decode_dss_signature(
        key.sign(digest, ec.ECDSA(utils.Prehashed(hashes.SHA256())))
    )
    s = min(s, SECP256K1_ORDER - s)
    for recovery in (27, 28):
        signature = r.to_bytes(32, "big") + s.to_bytes(32, "big") + bytes([recovery])
        if transaction.recover_signer(signature).lower() == source:
            raw = transaction.attach_signature(signature, source)
            break
    else:
        raise AssertionError("payment signer recovery failed")
    write(args.output / "payment.raw", raw)
    raw_transaction = "0x" + raw.hex()
    transaction_hash = client.rpc("eth_sendRawTransaction", [raw_transaction])
    assert hex_bytes(transaction_hash, 32) == keccak256(raw)
    deadline = time.monotonic() + 120
    while True:
        receipt = client.rpc("pax_sequencerReceipt", [transaction_hash])
        if receipt is not None:
            assert receipt["txHash"].lower() == transaction_hash.lower()
            assert receipt["rung"] in ("submitted", "receipted", "committed")
            if receipt["rung"] == "committed":
                break
        assert time.monotonic() < deadline, "payment did not commit"
        time.sleep(0.1)
    verified = verify_sequencer_receipt(
        receipt, sequencer, transaction_hash, source, nonce, value
    )
    source_after, after = account(args.did), account(args.destination)
    assert (
        source_after["account"] == source_before["account"]
        and after["account"] == before["account"]
    )
    assert min(source_after["height"], after["height"]) >= quantity(receipt["blockNumber"])
    source_wei_after = quantity(client.rpc("eth_getBalance", [source, "latest"]))
    destination_wei_after = quantity(client.rpc("eth_getBalance", [destination, "latest"]))
    assert source_wei_after == source_wei_before - value - verified.fee_charged
    assert destination_wei_after == destination_wei_before + value
    assert (
        verified.from_balance_before == source_wei_before
        and verified.from_balance_after == source_wei_after
    )
    assert balance(source_after) == source_wei_after // NATIVE_SCALE
    assert balance(after) == balance(before) + amount
    nonce_after = client.rpc("eth_getTransactionCount", [source, "latest"])
    assert quantity(nonce_after) == nonce + 1
    try:
        client.rpc("eth_sendRawTransaction", [raw_transaction])
    except RpcError as error:
        assert error.code == -32000 and error.message == "already known", str(error)
        replay_refusal = {"code": error.code, "message": error.message}
    else:
        raise AssertionError("payment replay must refuse the already-known transaction")
    assert (
        client.rpc("px_getBalances", [args.did])["balances"] == source_after["balances"]
    ), "payment replay changed the debit account"
    assert (
        client.rpc("px_getBalances", [args.destination])["balances"] == after["balances"]
    ), "payment replay changed the destination balance"
    assert quantity(client.rpc("eth_getBalance", [source, "latest"])) == source_wei_after
    assert quantity(client.rpc("eth_getBalance", [destination, "latest"])) == destination_wei_after
    assert client.rpc("eth_getTransactionCount", [source, "latest"]) == nonce_after
    assert client.rpc("pax_sequencerReceipt", [transaction_hash]) == receipt
    write(args.output / "receipt.json", json.dumps({"result": receipt}, indent=2))
    result = {
        "activity_id": transaction_hash,
        "funding": funding,
        "before": before,
        "after": after,
        "source_before": source_before,
        "source_after": source_after,
        "nonce_before": nonce,
        "nonce_after": quantity(nonce_after),
        "fee_charged": str(verified.fee_charged),
        "result": receipt,
        "replay_refusal": replay_refusal,
        "replayed": True,
    }
    write(args.output / "result.json", json.dumps(result, indent=2))
    print(json.dumps({
        "activity_id": transaction_hash,
        "amount": args.amount,
        "destination_balance": str(balance(after)),
        "replayed": True,
    }))


if __name__ == "__main__":
    main()
