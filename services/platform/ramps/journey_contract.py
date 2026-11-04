"""Strict request and BFT observation contracts shared by both ramp journeys."""

import json
import os
from pathlib import Path
import re
import stat
import sys


def require(condition, code):
    if not condition:
        raise RuntimeError(code)


def hash_bytes(value):
    require(isinstance(value, str) and re.fullmatch(r"[0-9a-f]{64}", value)
            and any(bytes.fromhex(value)), "nonzero_hash32")
    return list(bytes.fromhex(value))


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "duplicate_input_field")
        result[key] = value
    return result


def signed_transaction(value):
    require(isinstance(value, str) and re.fullmatch(r"02(?:[0-9a-f]{2}){1,131071}", value),
            "signed_eip1559_transaction")
    return list(bytes.fromhex(value))


def order_request(terms):
    require(isinstance(terms, dict), "order_terms")
    body = {key: terms[key] for key in ("order_id", "quote_id")}
    require(isinstance(body["order_id"], str)
            and re.fullmatch(r"[A-Za-z0-9_.-]{1,100}", body["order_id"]), "order_id")
    require(isinstance(body["quote_id"], str) and 0 < len(body["quote_id"]) <= 256, "quote_id")
    require(not {"payer_grant", "payer_grant_purpose", "canonical_receive_payload",
                 "account_sequence"}.intersection(terms), "retired_order_input")
    authority = {"customer_budget_id", "customer_budget_purpose"}.intersection(terms)
    if authority:
        require(len(authority) == 2, "complete_customer_authority")
        body.update({key: hash_bytes(terms[key]) for key in sorted(authority)})
    return body


def work_request(digest, action, raw_transaction=None):
    require(isinstance(digest, list) and len(digest) == 32 and any(digest)
            and all(type(byte) is int and 0 <= byte <= 255 for byte in digest), "order_digest")
    require(action in {"compliance", "submit_provider", "reconcile_provider",
                       "submit_layerx", "resolve_layerx", "submit_customer_payment"}, "work_action")
    body = {"order_digest": digest, "action": action, "account_sequence": None}
    if action == "submit_customer_payment":
        body["raw_transaction"] = signed_transaction(raw_transaction)
    else:
        require(raw_transaction is None, "unexpected_signed_transaction")
    return body


def validate_inputs(inputs):
    require(isinstance(inputs, dict) and set(inputs) == {"on", "off", "rebalance"}, "input_schema")
    common = {"order_id", "quote_id", "amount", "asset", "from_account", "to_account", "context"}
    for side in ("on", "off"):
        terms = inputs[side]
        expected = common if side == "on" else common | {
            "customer_budget_id", "customer_budget_purpose", "raw_transaction"}
        require(isinstance(terms, dict) and set(terms) == expected, "order_input_schema")
        order_request(terms)
        require(isinstance(terms["amount"], str) and re.fullmatch(r"[1-9][0-9]*", terms["amount"])
                and int(terms["amount"]) < 2**128, "order_amount")
        for field in ("asset", "from_account", "to_account", "context"):
            hash_bytes(terms[field])
    signed_transaction(inputs["off"]["raw_transaction"])
    require(inputs["on"]["order_id"] != inputs["off"]["order_id"], "distinct_orders_required")
    terms = inputs["rebalance"]
    require(isinstance(terms, dict) and set(terms) == {
        "asset", "idempotency_key", "operator_account", "amount"}, "rebalance_input_schema")
    for field in ("asset", "idempotency_key"):
        hash_bytes(terms[field])
    require(isinstance(terms["operator_account"], str)
            and terms["operator_account"].startswith("agent:did:layerx:")
            and len(terms["operator_account"]) <= 512, "operator_agent_account")
    require(isinstance(terms["amount"], str) and re.fullmatch(r"[1-9][0-9]*", terms["amount"])
            and int(terms["amount"]) < 2**128 - 1, "rebalance_amount")


def load_inputs(path):
    path = Path(path)
    require(path.is_absolute() and not path.resolve().is_relative_to(Path(__file__).resolve().parents[3]),
            "protected_inputs_path")
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, "rb") as stream:
        info = os.fstat(stream.fileno())
        require(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid()
                and stat.S_IMODE(info.st_mode) == 0o600 and info.st_nlink == 1,
                "protected_inputs_permissions")
        raw = stream.read(1_048_577)
    require(len(raw) <= 1_048_576, "protected_inputs_bound")
    inputs = json.loads(raw, object_pairs_hook=unique_object)
    validate_inputs(inputs)
    return inputs


def observe_rebalance(value, terms, previous=None):
    require(isinstance(value, dict) and value["settlement_domain"] == "paxeer", "rebalance_domain")
    require(value["idempotency_key"] == hash_bytes(terms["idempotency_key"])
            and value["asset"] == hash_bytes(terms["asset"])
            and type(value["amount"]) is int and value["amount"] == int(terms["amount"])
            and value["operator_account"] == terms["operator_account"], "rebalance_terms")
    require(isinstance(value["operation_id"], str) and 0 < len(value["operation_id"]) <= 512,
            "rebalance_operation")
    require(isinstance(value["transaction_hash"], str)
            and re.fullmatch(r"0x[0-9a-f]{64}", value["transaction_hash"])
            and any(bytes.fromhex(value["transaction_hash"][2:])), "rebalance_transaction")
    ranks = {"submitted": 0, "receipted": 1, "committed": 2}
    rung = value["rung"]
    require(rung in ranks and value["status"] == rung, "rebalance_rung")
    if rung == "committed":
        require(type(value["block_height"]) is int and value["block_height"] > 0
                and value["block_height"] < 2**64, "committed_height")
        require(isinstance(value["block_hash"], str)
                and re.fullmatch(r"0x[0-9a-f]{64}", value["block_hash"])
                and any(bytes.fromhex(value["block_hash"][2:])), "committed_block")
    else:
        require(value["block_hash"] is None and value["block_height"] is None,
                "no_credit_before_authenticated_commit")
    if previous is not None:
        require(value["operation_id"] == previous["operation_id"]
                and value["transaction_hash"] == previous["transaction_hash"], "rebalance_identity_changed")
        require(ranks[rung] >= ranks[previous["rung"]], "rebalance_rung_regressed")
        if previous["rung"] == "committed":
            require(value["block_hash"] == previous["block_hash"]
                    and value["block_height"] == previous["block_height"], "committed_identity_changed")
    return rung == "committed"


def main():
    operation = sys.argv[1]
    if operation == "inputs":
        print(json.dumps(load_inputs(sys.argv[2]), separators=(",", ":")))
        return
    terms = json.load(sys.stdin)
    if operation == "order":
        result = order_request(terms)
    elif operation == "work":
        result = work_request(terms["order_digest"], terms["action"], terms.get("raw_transaction"))
    else:
        raise RuntimeError("unknown_contract_operation")
    print(json.dumps(result, separators=(",", ":")))


if __name__ == "__main__":
    main()
