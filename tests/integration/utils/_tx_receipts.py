#!/usr/bin/env python3
"""Resolve sync submissions through finalized block bytes and execution results."""

import argparse
import base64
import gzip
import hashlib
import json
import math
from pathlib import Path
import re
import shlex
import subprocess
import sys
import time
from urllib.error import URLError
from urllib.request import Request, urlopen


class Unavailable(Exception):
    """The node has not made the requested committed data available yet."""


def uint(value, name, positive=False):
    if isinstance(value, bool) or not re.fullmatch(r"[0-9]+", str(value)):
        raise ValueError(f"invalid {name}")
    number = int(value)
    if positive and number == 0:
        raise ValueError(f"invalid {name}")
    return number


def remaining(deadline):
    seconds = deadline - time.monotonic()
    if seconds <= 0:
        raise TimeoutError("transaction confirmation deadline exceeded")
    return seconds


class RPC:
    def __init__(self, node):
        self.node = re.sub(r"^tcp://", "http://", node).rstrip("/")
        if not self.node.startswith(("http://", "https://")):
            raise ValueError("node must be an HTTP(S) or tcp RPC URL")

    def call(self, method, params, deadline):
        payload = {"jsonrpc": "2.0", "id": 1, "method": method, "params": params}
        request = Request(self.node, json.dumps(payload).encode(),
                          {"Content-Type": "application/json"})
        try:
            with urlopen(request, timeout=remaining(deadline)) as response:
                body = json.load(response)
        except (URLError, TimeoutError, OSError) as error:
            raise Unavailable(f"{method}: {error}") from error
        remaining(deadline)
        if body.get("error") is not None:
            raise Unavailable(f"{method}: {body['error']}")
        if body.get("id") != 1 or not isinstance(body.get("result"), dict):
            raise ValueError(f"malformed {method} RPC response")
        return body["result"]


def block_transactions(result, chain_id, height=None):
    block = result["block"]
    header = block["header"]
    observed = uint(header["height"], "block height", positive=True)
    if header["chain_id"] != chain_id or (height is not None and observed != height):
        raise ValueError("block chain or height does not match requested transaction")
    transactions = block["data"]["txs"]
    if transactions is None:  # Go serializes an empty transaction slice as null.
        transactions = []
    if not isinstance(transactions, list):
        raise ValueError("invalid block transaction list")
    return observed, [base64.b64decode(tx, validate=True) for tx in transactions]


def resolve(rpc, context, submission, deadline, interval):
    check_code = uint(submission["code"], "CheckTx code")
    if check_code != 0:
        print(f"CheckTx rejected: {submission.get('raw_log', '')}", file=sys.stderr)
        return {"stage": "check_tx", "code": check_code}
    txhash = submission["txhash"].upper()
    if not re.fullmatch(r"[0-9A-F]{64}", txhash):
        raise ValueError("accepted CheckTx has no valid transaction hash")
    print(f"CheckTx accepted {txhash}; awaiting committed execution", file=sys.stderr)
    height = uint(context["height"], "pre-submission height", positive=True) + 1
    last_error = "transaction has not appeared in a committed block"
    while remaining(deadline):
        try:
            block = rpc.call("block", {"height": str(height)}, deadline)
            _, transactions = block_transactions(block, context["chain_id"], height)
            matches = [index for index, tx in enumerate(transactions)
                       if hashlib.sha256(tx).hexdigest().upper() == txhash]
            if len(matches) > 1:
                raise ValueError("duplicate transaction identity in committed block")
            if matches:
                results = rpc.call("block_results", {"height": str(height)}, deadline)
                if uint(results["height"], "result height", positive=True) != height:
                    raise ValueError("execution result height does not match block")
                executions = results["txs_results"]
                if not isinstance(executions, list) or len(executions) != len(transactions):
                    raise ValueError("execution results do not match block transaction positions")
                execution = executions[matches[0]]
                if not isinstance(execution, dict):
                    raise ValueError("missing committed execution result")
                # ExecTxResult.Code uses json:"code,omitempty": omitted means proto3 zero.
                code = uint(execution.get("code", 0), "execution code")
                remaining(deadline)
                return {"stage": "execution", "code": code, "height": height,
                        "txhash": txhash, "events": execution.get("events", [])}
            height += 1
            continue
        except Unavailable as error:
            last_error = str(error)
        seconds = deadline - time.monotonic()
        if seconds <= 0:
            break
        time.sleep(min(interval, seconds))
    raise TimeoutError(f"cannot establish committed execution of {txhash}: {last_error}")


def require_success(receipt):
    if receipt["stage"] != "execution" or receipt["code"] != 0:
        raise ValueError(f"{receipt['stage']} failed with code {receipt['code']}")


def event_attributes(receipt, event_type):
    require_success(receipt)
    events = [event for event in receipt["events"] if event["type"] == event_type]
    if len(events) != 1:
        raise ValueError(f"expected one transaction-specific {event_type} event")
    attributes = {}
    for attribute in events[0]["attributes"]:
        # ABCI EventAttribute's protobuf JSON representation encodes byte slices as base64.
        key = base64.b64decode(attribute["key"], validate=True).decode()
        value = base64.b64decode(attribute["value"], validate=True).decode()
        if key not in {"code_id", "_contract_address"}:
            continue  # Store events may legitimately repeat feature attributes.
        if key in attributes:
            raise ValueError(f"ambiguous {event_type} attribute {key}")
        attributes[key] = value
    return attributes


def verify_store(code_id, info, creator, wasm_bytes):
    if wasm_bytes.startswith(b"\x1f\x8b"):
        wasm_bytes = gzip.decompress(wasm_bytes)
    checksum = hashlib.sha256(wasm_bytes).hexdigest().upper()
    if (uint(info["code_id"], "stored code ID", positive=True) != code_id
            or info["creator"] != creator or info["data_hash"].upper() != checksum):
        raise ValueError("stored Wasm checksum, creator or code ID does not match submission")


def verify_instantiate(address, info, history, code_id, creator, label, init_msg):
    metadata = info["contract_info"]
    if (info["address"] != address or metadata["creator"] != creator
            or uint(metadata["code_id"], "contract code ID", positive=True) != code_id
            or metadata["label"] != label):
        raise ValueError("contract identity, creator, code ID or label does not match submission")
    entries = history["entries"]
    if not entries:
        raise ValueError("contract has no initialization history")
    initial = entries[0]
    # QueryContractHistory returns RawContractMessage as JSON, not base64 bytes.
    if (initial["operation"] != "CONTRACT_CODE_HISTORY_OPERATION_TYPE_INIT"
            or uint(initial["code_id"], "initial code ID", positive=True) != code_id
            or json.dumps(initial["msg"], sort_keys=True) != json.dumps(init_msg, sort_keys=True)):
        raise ValueError("contract initialization state does not match submitted message")


def cli(paxdbin, args, deadline, as_json=True):
    completed = subprocess.run(shlex.split(paxdbin) + args, check=True,
                               capture_output=True, text=True, timeout=remaining(deadline))
    remaining(deadline)
    return json.loads(completed.stdout) if as_json else completed.stdout.strip()


def prepare(paxdbin, chain_id, deadline):
    node = cli(paxdbin, ["config", "node"], deadline, as_json=False)
    block = RPC(node).call("block", {}, deadline)
    height, _ = block_transactions(block, chain_id)
    return {"node": node, "height": height, "chain_id": chain_id}


def finish(mode, receipt, query, creator=None, wasm_bytes=None, code_id=None,
           label=None, init_msg=None):
    if mode == "code":
        return receipt["code"]
    require_success(receipt)
    if mode == "height":
        return receipt["height"]
    if mode == "store":
        attributes = event_attributes(receipt, "store_code")
        stored_id = uint(attributes["code_id"], "event code ID", positive=True)
        info = query(["wasm", "code-info", str(stored_id)], receipt["height"])
        verify_store(stored_id, info, creator, wasm_bytes)
        return stored_id
    if mode != "instantiate":
        raise ValueError("unknown transaction result mode")
    attributes = event_attributes(receipt, "instantiate")
    if uint(attributes["code_id"], "event code ID", positive=True) != code_id:
        raise ValueError("instantiation event names another code ID")
    address = attributes["_contract_address"]
    if not address:
        raise ValueError("instantiation event has no contract address")
    info = query(["wasm", "contract", address], receipt["height"])
    history = query(["wasm", "contract-history", address], receipt["height"])
    verify_instantiate(address, info, history, code_id, creator, label, init_msg)
    return address


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=["prepare", "height", "code", "store", "instantiate"])
    parser.add_argument("--paxdbin", required=True)
    parser.add_argument("--chain-id", required=True)
    parser.add_argument("--timeout", type=float, required=True)
    parser.add_argument("--interval", type=float, required=True)
    parser.add_argument("--context")
    parser.add_argument("--creator")
    parser.add_argument("--wasm-path")
    parser.add_argument("--code-id")
    parser.add_argument("--label")
    parser.add_argument("--init-msg")
    args = parser.parse_args()
    if not all(math.isfinite(v) and v > 0 for v in (args.timeout, args.interval)):
        raise ValueError("transaction timeout and polling interval must be finite and positive")
    deadline = time.monotonic() + args.timeout
    if args.mode == "prepare":
        print(json.dumps(prepare(args.paxdbin, args.chain_id, deadline)))
        return
    context = json.loads(args.context)
    if context["chain_id"] != args.chain_id:
        raise ValueError("submission context is for another chain")
    receipt = resolve(RPC(context["node"]), context, json.load(sys.stdin), deadline, args.interval)

    def query(command, height):
        return cli(args.paxdbin, ["q"] + command + ["--height", str(height), "-o", "json"], deadline)

    result = finish(args.mode, receipt, query, creator=args.creator,
                    wasm_bytes=Path(args.wasm_path).read_bytes() if args.wasm_path else None,
                    code_id=uint(args.code_id, "requested code ID", positive=True) if args.code_id else None,
                    label=args.label, init_msg=json.loads(args.init_msg) if args.init_msg else None)
    remaining(deadline)
    print(result)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, TypeError, OSError, TimeoutError, Unavailable,
            subprocess.SubprocessError) as error:
        print(f"transaction confirmation failed: {error}", file=sys.stderr)
        sys.exit(1)
