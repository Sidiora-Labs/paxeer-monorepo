"""Committed authz results and strict delegation queries for the YAML suites."""

import argparse
import json
import re
import subprocess
import sys
import time


SEND_LIMIT_REASON = "requested amount is more than spend limit"
UNBOND_AUTH_REASON = "authorization not found"


def json_object(raw):
    value = json.loads(raw)
    if not isinstance(value, dict):
        raise ValueError("Expected a JSON object")
    return value


def submission_hash(raw):
    response = json_object(raw)
    if type(response.get("code")) is not int or response["code"] != 0:
        raise ValueError(f"CheckTx did not accept the transaction: {raw}")
    txhash = response.get("txhash")
    if not isinstance(txhash, str) or not re.fullmatch(r"[0-9a-fA-F]{64}", txhash):
        raise ValueError("Missing or invalid submitted transaction hash")
    return txhash


def committed_code(raw, txhash, expected_code, expected_reason):
    response = json_object(raw)
    if not isinstance(response.get("txhash"), str) or response["txhash"].upper() != txhash.upper():
        raise ValueError("Committed result does not match the submitted transaction")
    height = response.get("height")
    if type(height) not in (int, str) or not re.fullmatch(r"[1-9][0-9]*", str(height)):
        raise ValueError("Transaction result has no committed block height")
    code = response.get("code")
    if type(code) is not int or code != expected_code:
        raise ValueError(f"Unexpected committed execution code: {raw}")
    if expected_code != 0:
        if response.get("codespace") != "sdk":
            raise ValueError(f"Unexpected committed execution codespace: {raw}")
        log = response.get("raw_log")
        if not isinstance(log, str) or not expected_reason or expected_reason not in log:
            raise ValueError(f"Missing intended authorization error: {raw}")
    return code


def delegation_amount(returncode, stdout, stderr, delegator, validator):
    if returncode != 0:
        message = f"delegation with delegator {delegator} not found for validator {validator}"
        prefix = "rpc error: code = NotFound desc = "
        # Direct gRPC and the SDK's ABCI -> gRPC key-not-found translation.
        missing = {
            f"Error: {prefix}{message}",
            f"Error: {prefix}{prefix}{message}: key not found",
        }
        if returncode == 1 and not stdout.strip() and stderr.strip() in missing:
            return 0
        raise ValueError(f"Delegation query failed ({returncode}): {stderr}")
    response = json_object(stdout)
    delegation = response.get("delegation")
    if not isinstance(delegation, dict) or (
        delegation.get("delegator_address") != delegator
        or delegation.get("validator_address") != validator
    ):
        raise ValueError("Delegation query returned the wrong address pair")
    balance = response.get("balance")
    if not isinstance(balance, dict) or balance.get("denom") != "uhpx":
        raise ValueError("Delegation query returned no uhpx balance")
    amount = balance.get("amount")
    if not isinstance(amount, str) or not re.fullmatch(r"0|[1-9][0-9]*", amount):
        raise ValueError("Delegation query returned an invalid amount")
    return int(amount)


def execute(grantee, transaction, expectation):
    expected_code, expected_reason = {
        "send-limit": (5, SEND_LIMIT_REASON),
        "unbond-denied": (4, UNBOND_AUTH_REASON),
        "unbond-success": (0, ""),
    }[expectation]
    submitted = subprocess.run(
        [
            "paxd", "tx", "authz", "exec", transaction, "--from", grantee,
            "--fees", "2000uhpx", "--chain-id", "pax", "-y",
            "--broadcast-mode=sync", "--output=json",
        ],
        input="12345678\n", text=True, capture_output=True, check=True, timeout=30,
    )
    txhash = submission_hash(submitted.stdout)
    deadline = time.monotonic() + 30
    last_error = "No committed transaction result"
    while time.monotonic() < deadline:
        result = subprocess.run(
            ["paxd", "q", "tx", txhash, "--output=json"],
            text=True, capture_output=True, check=False,
            timeout=max(0.01, deadline - time.monotonic()),
        )
        if result.returncode == 0:
            return committed_code(result.stdout, txhash, expected_code, expected_reason)
        last_error = result.stderr
        time.sleep(0.5)
    raise TimeoutError(f"Transaction {txhash} was not committed within 30s: {last_error}")


def query_delegation(delegator, validator):
    result = subprocess.run(
        ["paxd", "q", "staking", "delegation", delegator, validator, "--output=json"],
        text=True, capture_output=True, check=False, timeout=30,
    )
    return delegation_amount(
        result.returncode, result.stdout, result.stderr, delegator, validator,
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    execution = commands.add_parser("exec")
    execution.add_argument("grantee")
    execution.add_argument("transaction")
    execution.add_argument("expectation", choices=("send-limit", "unbond-denied", "unbond-success"))
    delegation = commands.add_parser("delegation")
    delegation.add_argument("delegator")
    delegation.add_argument("validator")
    args = parser.parse_args()
    try:
        if args.command == "exec":
            value = execute(args.grantee, args.transaction, args.expectation)
        else:
            value = query_delegation(args.delegator, args.validator)
    except (ValueError, OSError, subprocess.SubprocessError, TimeoutError) as error:
        print(error, file=sys.stderr)
        return 1
    print(value)
    return 0


if __name__ == "__main__":
    sys.exit(main())
