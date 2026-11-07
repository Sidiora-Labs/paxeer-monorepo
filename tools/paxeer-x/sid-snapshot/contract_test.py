#!/usr/bin/env python3
"""Real SID transfer/snapshot integration on a disposable, unlocked paxd chain."""

import argparse
import csv
import hashlib
import ipaddress
import json
from pathlib import Path
import tempfile
import time
from urllib.parse import urlsplit

import snapshot


FIELDS = ["transaction_hash", "log_index", "from_address", "to_address", "amount", "block_number"]


def loopback_rpc(value):
    endpoint = urlsplit(value)
    if (endpoint.scheme != "http" or endpoint.username or endpoint.password
            or endpoint.path not in ("", "/") or endpoint.query or endpoint.fragment
            or not endpoint.hostname):
        raise argparse.ArgumentTypeError("use a disposable loopback HTTP root RPC")
    try:
        local = ipaddress.ip_address(endpoint.hostname).is_loopback
    except ValueError:
        local = False
    if not local:
        raise argparse.ArgumentTypeError("RPC must be a loopback IP; this test moves real SID")
    return value


def committed_receipt(rpc, tx_hash):
    """A successful receipt is usable only when its canonical block is finalized."""
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        receipt = snapshot.rpc_call(rpc, "eth_getTransactionReceipt", [tx_hash])
        if receipt is not None:
            assert receipt["transactionHash"].lower() == tx_hash
            assert snapshot.quantity(receipt["status"]) == 1, receipt
            height = snapshot.quantity(receipt["blockNumber"])
            finalized = snapshot.committed_block(rpc, "finalized")
            if finalized["number"] >= height:
                block = snapshot.migration_block(rpc, height)
                assert block["hash"] == snapshot.hex_bytes(receipt["blockHash"], 32), receipt
                return block
        time.sleep(0.1)
    raise AssertionError(f"transaction did not reach committed success: {tx_hash}")


def send_sid(rpc, sender, recipient, amount):
    assert amount > 0
    data = "0xa9059cbb" + recipient[2:].rjust(64, "0") + format(amount, "064x")
    tx_hash = snapshot.hex_bytes(snapshot.rpc_call(rpc, "eth_sendTransaction", [
        {"from": sender, "to": snapshot.SID, "data": data, "gas": "0x7a120"}
    ]), 32)
    return committed_receipt(rpc, tx_hash)


def write_history(path, history):
    with path.open("w", newline="") as stream:
        writer = csv.writer(stream)
        writer.writerow(FIELDS)
        for (tx_hash, index), (sender, recipient, amount, height) in sorted(history.items()):
            writer.writerow([tx_hash, index, sender, recipient, amount, height])


def require_refusal(argv, output, message):
    try:
        snapshot.main(argv)
    except SystemExit as exc:
        assert message in str(exc), exc
    else:
        raise AssertionError(f"invalid snapshot accepted: {message}")
    assert not output.exists()
    assert not Path(str(output) + ".evidence.json").exists()


def run_contract_snapshot(rpc, holder, recipient, new_holder, work):
    assert snapshot.quantity(snapshot.rpc_call(rpc, "eth_chainId", [])) == 125
    assert snapshot.rpc_call(rpc, "eth_getCode", [snapshot.SID, "latest"]) != "0x"
    initial = snapshot.migration_block(rpc, None)
    initial_history = snapshot.chain_transfers(rpc, initial)
    prior_candidates = {address for row in initial_history.values() for address in row[:2]}
    assert new_holder not in prior_candidates, "new holder must have no previous SID Transfer history"
    amount = snapshot.balance_of(rpc, holder, initial)
    assert amount >= 4, "holder needs at least four base units of real SID"
    assert snapshot.balance_of(rpc, recipient, initial) == 0
    assert snapshot.balance_of(rpc, new_holder, initial) == 0
    supply = snapshot.token_uint(rpc, "0x18160ddd", initial)

    # The first transfer gives holder a zero history balance, before funds return.
    zero_block = send_sid(rpc, holder, recipient, amount)
    assert snapshot.balance_of(rpc, holder, zero_block) == 0
    old_history = snapshot.chain_transfers(rpc, zero_block)
    old_source = work / "old.csv"
    write_history(old_source, old_history)
    indexed, _ = snapshot.balances([str(old_source)], zero_block["number"])
    assert indexed[holder] == 0

    returned = amount // 4
    send_sid(rpc, recipient, holder, returned)
    block = send_sid(rpc, recipient, new_holder, returned)
    assert block["number"] > zero_block["number"]
    assert snapshot.balance_of(rpc, holder, block) == returned
    assert snapshot.balance_of(rpc, recipient, block) == amount - 2 * returned
    assert snapshot.balance_of(rpc, new_holder, block) == returned
    assert snapshot.token_uint(rpc, "0x18160ddd", block) == supply
    history = snapshot.chain_transfers(rpc, block)
    assert history != old_history
    complete_source = work / "explorer.csv"
    write_history(complete_source, history)
    candidates, _ = snapshot.balances([str(complete_source)], block["number"])
    expected = {address: snapshot.balance_of(rpc, address, block) for address in candidates}
    assert sum(expected.values()) == supply

    # Advance the actual chain after selecting the migration height.
    later = send_sid(rpc, holder, recipient, 1)
    assert later["number"] > block["number"]
    assert snapshot.balance_of(rpc, holder, later) == returned - 1
    # Include future explorer rows; the production CSV reader must cut them off.
    write_history(complete_source, snapshot.chain_transfers(rpc, later))
    output = work / "holders.json"
    argv = ["--rpc", rpc, "--block", str(block["number"]), "--source", str(old_source),
            "--source", str(complete_source), "--out", str(output)]
    snapshot.main(argv)
    holders = json.loads(output.read_text())
    assert holders == [{"address": address, "balance": str(balance)}
                       for address, balance in sorted(expected.items()) if balance > 0]
    assert sum(int(row["balance"]) for row in holders) == supply
    evidence = json.loads(Path(str(output) + ".evidence.json").read_text())
    assert evidence["block"] == block
    assert evidence["total_supply"] == str(supply)
    assert evidence["holders_sha256"] == hashlib.sha256(output.read_bytes()).hexdigest()
    assert evidence["transfer_count"] == len(history)

    stale_output = work / "stale.json"
    require_refusal(["--rpc", rpc, "--block", str(block["number"]), "--source", str(old_source),
                     "--out", str(stale_output)], stale_output, "history is incomplete")

    # Preserve the legacy conflicting-source case with genuine producer rows.
    conflict = dict(history)
    key = next(iter(conflict))
    sender, receiver, value, height = conflict[key]
    conflict[key] = (sender, receiver, value + 1, height)
    bad_source = work / "conflict.csv"
    write_history(bad_source, conflict)
    bad_output = work / "conflict.json"
    require_refusal(["--rpc", rpc, "--block", str(block["number"]),
                     "--source", str(complete_source), "--source", str(bad_source),
                     "--out", str(bad_output)], bad_output, "sources disagree")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--rpc", required=True, type=loopback_rpc)
    parser.add_argument("--holder", required=True, help="unlocked funded SID holder on the disposable chain")
    parser.add_argument("--recipient", required=True, help="unlocked zero-balance SID recipient")
    parser.add_argument("--new-holder", required=True, help="address with no prior SID Transfer history")
    args = parser.parse_args()
    addresses = [snapshot.hex_bytes(value, 20) for value in (args.holder, args.recipient, args.new_holder)]
    assert len(set(addresses)) == 3 and snapshot.ZERO not in addresses
    with tempfile.TemporaryDirectory(prefix="sid-contract-snapshot-") as directory:
        run_contract_snapshot(args.rpc, *addresses, Path(directory))
    print("real SID fixed committed snapshot: passed")


if __name__ == "__main__":
    main()
