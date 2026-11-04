#!/usr/bin/env python3
"""Build a supply-checked SID holder list at one finalized block."""
import argparse
import csv
import hashlib
import json
from pathlib import Path
import re
import sys
import urllib.request

SID = "0x21f7b20a555199fa73a238b1a91fd0f549068fee"
ZERO = "0x" + "0" * 40
TRANSFER = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
LOG_RANGE = 2000
QUERY = """
SELECT '0x' || encode(transaction_hash, 'hex'), log_index,
       '0x' || encode(from_address_hash, 'hex'), '0x' || encode(to_address_hash, 'hex'), amount,
       block_number
FROM token_transfers WHERE token_contract_address_hash = decode(%s, 'hex')
AND block_number <= %s AND block_consensus = TRUE
"""


def hex_bytes(value, size):
    if not isinstance(value, str) or not re.fullmatch(r"0x[0-9a-fA-F]{%d}" % (size * 2), value):
        sys.exit(f"invalid {size}-byte hex value: {value!r}")
    return value.lower()


def quantity(value):
    if not isinstance(value, str) or not re.fullmatch(r"0x(?:0|[1-9a-fA-F][0-9a-fA-F]*)", value):
        sys.exit(f"invalid RPC quantity: {value!r}")
    return int(value, 16)


def transfer(tx, idx, frm, to, amount, height):
    row = (hex_bytes(tx, 32), int(idx), hex_bytes(frm, 20),
           hex_bytes(to, 20), int(amount), int(height))
    if row[1] < 0 or row[4] < 0 or row[4] >= 2**256 or row[5] < 0:
        sys.exit("invalid transfer index, amount or block number")
    return row


def read_source(source, block_number):
    """Read only transfers through the pinned height; CSVs must include block_number."""
    if source.startswith(("postgres://", "postgresql://")):
        import psycopg2

        with psycopg2.connect(source) as conn, conn.cursor() as cur:
            cur.execute(QUERY, (SID[2:], block_number))
            for row in cur:
                yield transfer(*row)
        return
    with open(source, newline="") as f:
        reader = csv.DictReader(f)
        fields = {"transaction_hash", "log_index", "from_address", "to_address", "amount", "block_number"}
        if not fields.issubset(reader.fieldnames or []):
            sys.exit("source CSV requires transaction_hash,log_index,from_address,to_address,amount,block_number")
        for row in reader:
            if int(row["block_number"]) > block_number:
                continue
            yield transfer(row["transaction_hash"], row["log_index"], row["from_address"],
                           row["to_address"], row["amount"], row["block_number"])


def add_transfer(seen, row):
    tx, idx, frm, to, amount, height = row
    prev = seen.setdefault((tx, idx), (frm, to, amount, height))
    if prev != (frm, to, amount, height):
        sys.exit(f"sources disagree on transfer {tx}:{idx}: {prev} vs {(frm, to, amount, height)}")


def balances(sources, block_number):
    seen = {}
    for source in sources:
        for row in read_source(source, block_number):
            if row[5] > block_number:
                sys.exit("source returned a transfer after the pinned block")
            add_transfer(seen, row)
    held = {}
    for frm, to, amount, _ in seen.values():
        held[frm] = held.get(frm, 0) - amount
        held[to] = held.get(to, 0) + amount
    held.pop(ZERO, None)
    # Keep zero and negative history candidates until every address is reconciled.
    return held, seen


def rpc_call(rpc, method, params):
    body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).encode()
    req = urllib.request.Request(rpc, body, {"content-type": "application/json"})
    with urllib.request.urlopen(req, timeout=30) as resp:
        reply = json.load(resp)
    if (not isinstance(reply, dict) or reply.get("jsonrpc") != "2.0"
            or reply.get("id") != 1 or "error" in reply or "result" not in reply):
        sys.exit(f"{method} failed: {reply}")
    return reply["result"]


def committed_block(rpc, tag):
    block = rpc_call(rpc, "eth_getBlockByNumber", [tag, False])
    if not isinstance(block, dict):
        sys.exit(f"committed block unavailable: {tag}")
    return {"number": quantity(block.get("number")), "hash": hex_bytes(block.get("hash"), 32)}


def token_uint(rpc, data, block):
    result = rpc_call(rpc, "eth_call", [{"to": SID, "data": data},
                                      {"blockHash": block["hash"], "requireCanonical": True}])
    return int(hex_bytes(result, 32), 16)


def balance_of(rpc, holder, block):
    data = "0x70a08231" + holder[2:].rjust(64, "0")
    return token_uint(rpc, data, block)


def chain_transfers(rpc, block):
    """Fetch the complete canonical Transfer history, bounded by the pinned height."""
    seen = {}
    for start in range(0, block["number"] + 1, LOG_RANGE):
        end = min(start + LOG_RANGE - 1, block["number"])
        logs = rpc_call(rpc, "eth_getLogs", [{"address": SID, "topics": [TRANSFER],
                                            "fromBlock": hex(start), "toBlock": hex(end)}])
        if not isinstance(logs, list):
            sys.exit("invalid Transfer log response")
        for log in logs:
            if not isinstance(log, dict):
                sys.exit("invalid Transfer log")
            topics = log.get("topics")
            if (log.get("removed") is not False or hex_bytes(log.get("address"), 20) != SID
                    or not isinstance(topics, list) or len(topics) != 3 or topics[0] != TRANSFER):
                sys.exit("invalid or removed SID Transfer log")
            frm, to = (hex_bytes(topic, 32) for topic in topics[1:])
            if frm[2:26] != "0" * 24 or to[2:26] != "0" * 24:
                sys.exit("invalid Transfer address padding")
            height = quantity(log.get("blockNumber"))
            log_hash = hex_bytes(log.get("blockHash"), 32)
            if not start <= height <= end or (height == block["number"] and log_hash != block["hash"]):
                sys.exit("Transfer log does not belong to the pinned range")
            row = transfer(log.get("transactionHash"), quantity(log.get("logIndex")),
                           "0x" + frm[-40:], "0x" + to[-40:],
                           int(hex_bytes(log.get("data"), 32), 16), height)
            if (row[0], row[1]) in seen:
                sys.exit("duplicate canonical Transfer log")
            add_transfer(seen, row)
    return seen


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--source", action="append", required=True,
                    help="explorer Postgres DSN or CSV path (repeat for the old paxscan copy)")
    ap.add_argument("--rpc", required=True, help="archive JSON-RPC URL with finalized blocks and EIP-1898 support")
    ap.add_argument("--out", required=True, help="holder list JSON path; evidence is written beside it")
    args = ap.parse_args(argv)

    block = committed_block(args.rpc, "finalized")
    chain_id = quantity(rpc_call(args.rpc, "eth_chainId", []))
    held, history = balances(args.source, block["number"])
    canonical = chain_transfers(args.rpc, block)
    # Include addresses absent from an incomplete source, so none escape reconciliation.
    for frm, to, _, _ in canonical.values():
        for address in (frm, to):
            if address != ZERO:
                held.setdefault(address, 0)
    for holder in sorted(held):
        onchain = balance_of(args.rpc, holder, block)
        if onchain != held[holder]:
            print(f"mismatch {holder}: transfers {held[holder]} chain {onchain}", file=sys.stderr)
        held[holder] = onchain
    supply = token_uint(args.rpc, "0x18160ddd", block)
    if history != canonical:
        sys.exit("transfer history is incomplete or disagrees with canonical history through the pinned block")
    if sum(held.values()) != supply:
        sys.exit(f"holder sum {sum(held.values())} does not match totalSupply {supply}")
    history_supply = sum(amount * ((frm == ZERO) - (to == ZERO))
                         for frm, to, amount, _ in canonical.values())
    if history_supply != supply:
        sys.exit(f"transfer history supply {history_supply} does not match totalSupply {supply}")
    if committed_block(args.rpc, hex(block["number"])) != block:
        sys.exit("pinned block changed during reconciliation")

    holders = [{"address": a, "balance": str(b)} for a, b in sorted(held.items()) if b > 0]
    payload = (json.dumps(holders, indent=2) + "\n").encode()
    evidence = {"chain_id": chain_id, "token": SID, "block": block, "commitment": "finalized",
                "history_from_block": 0, "history_through_block": block["number"],
                "transfer_count": len(canonical), "total_supply": str(supply),
                "holder_count": len(holders), "holders_sha256": hashlib.sha256(payload).hexdigest()}
    Path(args.out + ".evidence.json").write_text(json.dumps(evidence, indent=2) + "\n")
    Path(args.out).write_bytes(payload)
    print(f"holders {len(holders)} sum {supply}")


if __name__ == "__main__":
    main()
