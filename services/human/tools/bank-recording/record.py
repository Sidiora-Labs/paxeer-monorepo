"""Record native-pointer transfers and their node-issued consensus evidence."""

import argparse
import base64
import json
from pathlib import Path
import time
import urllib.request

from eth_account import Account
from eth_abi import encode
from eth_utils import keccak, to_checksum_address


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("environment", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    environment = dict(line.split("=", 1) for line in args.environment.read_text().splitlines())
    endpoint = environment["LOCAL_NODE_SEQUENCER_RPC"]

    def rpc(method, params):
        body = json.dumps(dict(jsonrpc="2.0", id=1, method=method, params=params)).encode()
        request = urllib.request.Request(endpoint, body, {"Content-Type": "application/json"})
        with urllib.request.urlopen(request, timeout=10) as response:
            answer = json.load(response)
        if "error" in answer:
            raise RuntimeError(answer["error"])
        return answer["result"]

    def pending(method, params):
        deadline = time.monotonic() + 30
        while True:
            try:
                return rpc(method, params)
            except RuntimeError as error:
                if "no block is open" not in str(error) or time.monotonic() >= deadline:
                    raise
                time.sleep(0.02)

    def calldata(signature, types, values):
        return "0x" + (keccak(text=signature)[:4] + encode(types, values)).hex()

    def receipt(tx_hash, rung):
        deadline = time.monotonic() + 90
        while time.monotonic() < deadline:
            answer = rpc("pax_sequencerReceipt", [tx_hash])
            if answer and answer["rung"] == rung:
                return answer
            time.sleep(0.01)
        raise RuntimeError("receipt did not reach the requested rung")

    def send(secret, to, data, gas=1_000_000):
        sender = Account.from_key(secret).address
        tx = dict(type=2, chainId=125,
                  nonce=int(pending("eth_getTransactionCount", [sender, "pending"]), 16),
                  maxPriorityFeePerGas=1, maxFeePerGas=1_000_000_000_000,
                  gas=gas, to=to_checksum_address(to),
                  value=0, data=data)
        signed = Account.sign_transaction(tx, secret)
        raw = "0x" + signed.raw_transaction.hex()
        tx_hash = "0x" + signed.hash.hex()
        try:
            assert pending("eth_sendRawTransaction", [raw]) == tx_hash
        except RuntimeError as error:
            if "transaction held at submitted awaiting nonce" not in str(error):
                raise
            submitted = rpc("pax_sequencerReceipt", [tx_hash])
            assert submitted["txHash"] == tx_hash and int(submitted["nonce"], 16) == tx["nonce"]
        return tx_hash, raw

    query = calldata("getNativePointer(string)", ["string"], ["uhpx"])
    view = "0x" + "00" * 18 + "100a"
    answer = pending("eth_call", [{"to": view, "data": query}, "pending"])
    if int(answer[-64:], 16) == 0:
        tx_hash, _ = send(environment["LOCAL_NODE_KEY"], "0x" + "00" * 18 + "100b",
                          calldata("addNativePointer(string)", ["string"], ["uhpx"]), 5_000_000)
        assert receipt(tx_hash, "committed")["status"] == "0x1"
        answer = pending("eth_call", [{"to": view, "data": query}, "pending"])
    assert len(bytes.fromhex(answer[2:])) == 96 and int(answer[-64:], 16) == 1
    pointer = "0x" + answer[26:66]
    secret = "0x" + "42" * 32
    sender = Account.from_key(secret).address
    tx_hash, _ = send(environment["LOCAL_NODE_KEY"], pointer,
                      calldata("transfer(address,uint256)", ["address", "uint256"], [sender, 10**12]))
    assert receipt(tx_hash, "committed")["status"] == "0x1"
    nonce = int(pending("eth_getTransactionCount", [sender, "pending"]), 16)
    records = []
    for recipient, amount in [("0x" + "22" * 20, 90), ("0x" + "55" * 20, 40)]:
        before = pending("eth_getBalance", [recipient, "pending"])
        tx_hash, raw = send(secret, pointer,
                            calldata("transfer(address,uint256)", ["address", "uint256"], [recipient, amount]))
        receipted = receipt(tx_hash, "receipted")
        assert receipted["returnData"] == "0x" + "00" * 31 + "01"
        committed = receipt(tx_hash, "committed")
        height = int(committed["blockNumber"], 16)
        batch = rpc("px_getSequenceBatch", [str(height)])
        proof = batch["proof"]
        b64 = lambda value: base64.b64encode(bytes.fromhex(value[2:])).decode()
        records.append(dict(
            raw=raw, recipient=recipient, balanceBefore=before,
            balanceAfter=pending("eth_getBalance", [recipient, "pending"]),
            view=receipted, landing=rpc("eth_getTransactionReceipt", [tx_hash]),
            height=height, hash=committed["blockHash"],
            block=rpc("eth_getBlockByNumber", [hex(height), True]),
            material=rpc("px_getCommit", [str(height)]),
            validators=rpc("px_getValidators", [str(height)]),
            batch=batch["tx"], activityIds=batch["activityIds"],
            proof=dict(total=str(proof["total"]), index=str(proof["index"]),
                       leaf_hash=b64(proof["leafHash"]), aunts=[b64(aunt) for aunt in proof["aunts"]])))
        records[-1]["appHash"] = "0x" + records[-1]["material"]["signedHeader"]["header"]["app_hash"].removeprefix("0x")
    keys = rpc("px_getReceiptKeys", [])["keys"]
    key = next(key for key in keys if key["keyId"] == records[0]["view"]["keyId"])
    document = dict(producer="Native-pointer transfers and consensus evidence served by the node",
                    key=key, nonce=nonce, pointer=pointer, pointerQuery=query, pointerView=answer,
                    secret=secret, sender=sender.lower(), records=records)
    args.output.write_text(json.dumps(document, indent=2) + "\n")


if __name__ == "__main__":
    main()
