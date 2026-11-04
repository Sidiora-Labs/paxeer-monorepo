#!/bin/sh
set -eu
if [ "${1:-}" = --help ] || [ "${1:-}" = -h ]; then
  cat <<'HELP'
Usage: sandbox-journey.sh [--help]

Run on-ramp and off-ramp sandbox orders and verify their transaction receipts.
Required environment:
  LAYERX_RAMP_RECEIPT_VERIFIER_BINARY  Built ramp-verify-receipt CLI
  LAYERX_RAMP_RECEIPT_VERIFIER_CONFIG  Protected configuration with receipt keys
  LAYERX_RAMP_URL
  LAYERX_RAMP_CA_PEM
  LAYERX_RAMP_CUSTOMER_TOKEN
  LAYERX_RAMP_OPERATOR_URL
  LAYERX_RAMP_OPERATOR_TOKEN
  LAYERX_RAMP_ON_QUOTE_ID
  LAYERX_RAMP_OFF_QUOTE_ID
  LAYERX_RAMP_OFF_GRANT_JSON
  LAYERX_RAMP_OFF_RECEIVER_SEQUENCE    Receiver sequence for draws

The on-ramp send reads its pending nonce from the merged node.
PAXEER_X_RAMP_FULL_CONTRACT=1 selects the protected full-contract journey inputs.
HELP
  exit 0
fi
if [ "${PAXEER_X_RAMP_FULL_CONTRACT:-0}" = 1 ]; then
root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
exec python3 - "$root" <<'PY'
import http.client
import ipaddress
import json
import os
from pathlib import Path
import re
import socket
import ssl
import stat
import subprocess
import sys
import time
from urllib.parse import urlsplit

root = Path(sys.argv[1]).resolve()
sys.path.insert(0, str(root / "services/agent/sdk/python"))

LABEL = "External custody: this independent market maker controls the off-platform funds and payout."
CASE_NAMES = (
    "on_ramp_receipt", "off_ramp_receipt", "external_custody_labels",
    "principal_isolation", "operator_authorization", "payer_grant_required",
    "customer_authority_rejected", "out_of_order_refused", "order_idempotency",
    "rebalance_own_account_finality", "rebalance_idempotency", "recovery_same_identifiers",
)
cases = dict.fromkeys(CASE_NAMES, False)
report = {"schema_version": 1, "source_revision": None, "deployment_profile": None,
          "runtime_source_bound": False, "qualified": False, "cases": cases}
evidence = None
sequence = 0
current_step = "configuration"
deadline = time.monotonic() + 1500


def require(condition, code):
    if not condition:
        raise RuntimeError(code)


def env(name):
    value = os.environ.get(name, "")
    require(bool(value), "required_environment:" + name)
    return value


def protected_file(name):
    path = Path(env(name))
    require(path.is_absolute(), "protected_path_absolute:" + name)
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and not path.is_symlink()
            and info.st_uid == os.getuid() and info.st_nlink == 1
            and stat.S_IMODE(info.st_mode) == 0o600,
            "protected_file_permissions:" + name)
    require(not path.resolve().is_relative_to(root), "protected_file_in_repository:" + name)
    return path


def read_protected(name):
    path = protected_file(name)
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(descriptor, "rb") as stream:
        info = os.fstat(stream.fileno())
        require(info.st_uid == os.getuid() and stat.S_IMODE(info.st_mode) == 0o600
                and info.st_nlink == 1 and stat.S_ISREG(info.st_mode), "protected_file_changed")
        data = stream.read(1_048_577)
    require(len(data) <= 1_048_576, "protected_file_bound")
    return data


def hex32(value):
    require(isinstance(value, str) and re.fullmatch(r"[0-9a-f]{64}", value) is not None,
            "canonical_hex32")
    return bytes.fromhex(value)


def octets(value, count=32):
    require(isinstance(value, list) and len(value) == count
            and all(type(item) is int and 0 <= item <= 255 for item in value), "canonical_bytes")
    return bytes(value)


def integer(value, minimum=0, maximum=2**64 - 1):
    require(type(value) is int and minimum <= value <= maximum, "integer_bound")
    return value


def private_address(value):
    address = ipaddress.ip_address(value)
    if isinstance(address, ipaddress.IPv6Address) and address.ipv4_mapped:
        address = address.ipv4_mapped
    return address.is_loopback or any(address in network for network in (
        ipaddress.ip_network("10.0.0.0/8"), ipaddress.ip_network("172.16.0.0/12"),
        ipaddress.ip_network("192.168.0.0/16"), ipaddress.ip_network("fc00::/7")))


def origin(name):
    value = urlsplit(env(name))
    require(value.scheme == "https" and value.hostname and not value.username
            and not value.password and value.path in ("", "/") and not value.query
            and not value.fragment, "private_https_origin:" + name)
    addresses = socket.getaddrinfo(value.hostname, value.port or 443, type=socket.SOCK_STREAM)
    require(addresses and all(private_address(item[4][0]) for item in addresses),
            "private_dns_required:" + name)
    return value, addresses[0][4][0]


def save(name, value):
    path = evidence / name
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, "w", encoding="utf-8") as stream:
        json.dump(value, stream, sort_keys=True, separators=(",", ":"))
        stream.write("\n")


def request(endpoint, actor, method, path, body=None, statuses=(200,), error=None):
    global sequence
    require(time.monotonic() < deadline, "journey_deadline")
    require(path.startswith("/") and not any(character in path for character in "\r\n?#"), "request_path")
    if endpoint == "customer" and public_prefix == "/v1/ramp":
        if path == "/readyz":
            path = "/v1/ramp/readyz"
        else:
            require(path.startswith("/v1/orders"), "unified_ramp_route")
            path = public_prefix + path[len("/v1"):]
    parsed, pinned = origins[endpoint]
    connection = http.client.HTTPSConnection(parsed.hostname, parsed.port or 443,
                                            timeout=20, context=context)
    raw = socket.create_connection((pinned, parsed.port or 443), timeout=20)
    require(private_address(raw.getpeername()[0]), "connected_peer_not_private")
    try:
        connection.sock = context.wrap_socket(raw, server_hostname=parsed.hostname)
        headers = {"Accept": "application/json", "Connection": "close"}
        if actor is not None:
            headers["Authorization"] = "Bearer " + tokens[actor]
        payload = None if body is None else json.dumps(body, separators=(",", ":")).encode()
        if payload is not None:
            headers["Content-Type"] = "application/json"
            headers["Content-Length"] = str(len(payload))
        connection.request(method, path, body=payload, headers=headers)
        response = connection.getresponse()
        encoded = response.read(4_194_305)
        require(len(encoded) <= 4_194_304, "response_bound")
        require(response.getheader("Content-Type", "").split(";", 1)[0] == "application/json",
                "response_content_type")
        value = json.loads(encoded)
        require(isinstance(value, dict), "response_object")
        sequence += 1
        save(f"ramp-response-{sequence:04d}.json", {"step": current_step, "status": response.status,
                                                 "body": value})
        require(response.status in statuses, "unexpected_http_status:" + str(response.status))
        if error is not None:
            require(value.get("error") == error, "typed_refusal_mismatch")
        return value, response.status
    finally:
        connection.close()
        raw.close()


def custody(value):
    require(value.get("external_custody_label") == LABEL, "external_custody_label")


def presentation(snapshot, done=False):
    value = snapshot["presentation"]
    custody(value)
    if done:
        require(snapshot["stage"] == "done" and value["status"] == "done", "done_status")
        evidence = snapshot["evidence"]
        require(any(octets(evidence["tx_hash"])) and any(octets(evidence["receipt_digest"]))
                and evidence["rung"] == "committed"
                and value["receipt_digest"] == evidence["receipt_digest"],
                "done_requires_transaction_receipt")
    else:
        require(snapshot["stage"] != "done" and value["status"] != "done", "premature_done")
    return value


def read_order(digest):
    return request("customer", "customer", "GET", "/v1/orders/" + digest.hex())[0]


def work(digest, action, values=None, status=(200,), error=None):
    body = {"order_digest": list(digest), "action": action, "account_sequence": None}
    if values is not None:
        body.update(values)
    return request("operator", "operator", "POST", "/internal/v1/work", body, status, error)[0]


def wait_stage(digest, expected):
    for _ in range(60):
        snapshot = read_order(digest)
        custody(snapshot["presentation"])
        if snapshot["stage"] == expected:
            return snapshot
        require(snapshot["stage"] not in (
            "compliance_refused", "provider_refused", "layerx_refused", "manual_review",
            "provider_reversed", "reversed", "done"), "unexpected_terminal_stage")
        presentation(snapshot)
        time.sleep(3)
    raise RuntimeError("stage_deadline")


def verifier_layerx_config():
    config = json.loads(read_protected("LAYERX_RAMP_RECEIPT_VERIFIER_CONFIG"))
    require(set(config) == {"client_tls", "layerx"}, "verifier_config_schema")
    layerx = config["layerx"]
    require(set(layerx) == {
        "rpc_endpoint", "rpc_trust_anchor_der", "chain_id", "operator_address",
        "receipt_keys", "max_fee_per_gas", "max_priority_fee_per_gas", "gas_limit_cap",
        "signer_endpoint", "signer_token_file", "actor_did"}, "verifier_layerx_schema")
    require(type(layerx["chain_id"]) is int and layerx["chain_id"] == 125,
            "verifier_chain_id")
    require(isinstance(layerx["receipt_keys"], list) and layerx["receipt_keys"],
            "independent_receipt_key_pins")
    for pin in layerx["receipt_keys"]:
        require(set(pin) == {"keyId", "publicKey", "firstHeight", "lastHeight"},
                "receipt_key_schema")
        for field in ("keyId", "publicKey"):
            require(isinstance(pin[field], str)
                    and re.fullmatch(r"0x[0-9a-f]{64}", pin[field])
                    and any(bytes.fromhex(pin[field][2:])), "receipt_key_encoding")
        integer(pin["firstHeight"])
        integer(pin["lastHeight"], pin["firstHeight"])
    return layerx


def validate_cli_receipt(snapshot, verified, layerx):
    view = presentation(snapshot, True)
    require(any(octets(view["provider_evidence_digest"])), "done_requires_provider_evidence")
    transaction = "0x" + octets(snapshot["evidence"]["tx_hash"]).hex()
    digest = octets(view["order_digest"]).hex()
    require(verified["build_revision"] == expected_revision
            and verified["build_source_digest"] == expected_digest
            and verified["external_custody_label"] == LABEL,
            "verifier_source_and_custody_binding")
    require(verified["verified"] is True and verified["maintained_batch"] is True
            and verified["order_digest"] == digest
            and octets(snapshot["order"]["order_digest"]).hex() == digest
            and verified["transaction_hash"] == transaction
            and view["transaction_hash"] == transaction
            and verified["receipt_digest"] == octets(view["receipt_digest"]).hex()
            and verified["chain_id"] == layerx["chain_id"] == 125
            and verified["rung"] == snapshot["evidence"]["rung"] == "committed",
            "maintained_receipt_identity")
    height = integer(verified["height"], 1)
    require(any(pin["firstHeight"] <= height <= pin["lastHeight"]
                for pin in layerx["receipt_keys"]), "receipt_height_pin_range")
    return {key: verified[key] for key in
            ("order_digest", "transaction_hash", "receipt_digest", "height", "rung")}


def verify_done(snapshot):
    layerx = verifier_layerx_config()
    view = presentation(snapshot, True)
    transaction = "0x" + octets(snapshot["evidence"]["tx_hash"]).hex()
    digest = octets(view["order_digest"])
    bound = read_order(digest)["order"]
    require(bound == snapshot["order"] and octets(bound["order_digest"]) == digest,
            "bound_order_identity")
    order_name = "receipt-order-" + transaction[2:] + ".json"
    save(order_name, bound)
    order_path = evidence / order_name
    result_path = evidence / ("verified-receipt-" + transaction[2:] + ".json")
    require(not result_path.exists(), "fresh_verifier_output_required")
    remaining = min(180, deadline - time.monotonic())
    require(remaining > 0, "receipt_verifier_deadline")
    completed = subprocess.run(
        [env("LAYERX_RAMP_RECEIPT_VERIFIER_BINARY"), "--config-file",
         str(protected_file("LAYERX_RAMP_RECEIPT_VERIFIER_CONFIG")), "--order-file",
         str(order_path), "--transaction-hash", transaction, "--output-file", str(result_path)],
        stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        timeout=remaining, check=False)
    require(completed.returncode == 0 and len(completed.stdout) <= 16384,
            "maintained_receipt_verification")
    descriptor = os.open(result_path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, "rb") as stream:
        info = os.fstat(stream.fileno())
        require(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid()
                and stat.S_IMODE(info.st_mode) == 0o600 and info.st_nlink == 1,
                "protected_verifier_result")
        encoded = stream.read(16385)
    require(len(encoded) <= 16384, "bounded_verifier_result")
    return validate_cli_receipt(snapshot, json.loads(encoded), layerx)


def create(terms, grant, body_extra=None, statuses=(201,), error=None):
    body = {"order_id": terms["order_id"], "quote_id": terms["quote_id"], "payer_grant": grant}
    if body_extra:
        body.update(body_extra)
    return request("customer", "customer", "POST", "/v1/orders", body, statuses, error)[0]


try:
    public_prefix = os.environ.get("LAYERX_RAMP_PUBLIC_PATH_PREFIX", "/v1")
    require(public_prefix in ("/v1", "/v1/ramp"), "declared_public_route_profile")
    report["public_path_prefix"] = public_prefix
    require(env("LAYERX_DEPLOYMENT_PROFILE") == "private-network", "private_profile_required")
    report["deployment_profile"] = "private-network"
    evidence_path = Path(env("PAXEER_X_EVIDENCE_DIR"))
    require(evidence_path.is_absolute() and not evidence_path.is_symlink(), "evidence_path")
    info = evidence_path.stat()
    require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.getuid()
            and stat.S_IMODE(info.st_mode) == 0o700 and not evidence_path.resolve().is_relative_to(root),
            "evidence_permissions")
    evidence = evidence_path
    inputs = json.loads(read_protected("LAYERX_RAMP_INPUTS_FILE"))
    for side in ("on", "off"):
        terms = inputs[side]
        require(isinstance(terms["order_id"], str)
                and re.fullmatch(r"[A-Za-z0-9_.-]{1,100}", terms["order_id"]), "order_id")
        require(isinstance(terms["quote_id"], str) and terms["quote_id"], "quote_id")
        if side == "off":
            integer(terms["account_sequence"])
        integer(int(terms["amount"]), 1, 2**128 - 1)
        for field in ("asset", "from_account", "to_account", "context"):
            hex32(terms[field])
    require(inputs["on"]["order_id"] != inputs["off"]["order_id"], "distinct_orders_required")
    grant = list(hex32(inputs["off"]["payer_grant"]))
    require(any(grant), "payer_grant_required")
    octets(inputs["off"]["canonical_receive_payload"], 733)
    rebalance = inputs["rebalance"]
    for field in ("asset", "idempotency_key"):
        require(any(hex32(rebalance[field])), "rebalance_binding")
    require(isinstance(rebalance["operator_account"], str)
            and rebalance["operator_account"].startswith("agent:did:layerx:")
            and len(rebalance["operator_account"]) <= 512, "operator_agent_account")
    integer(int(rebalance["amount"]), 1, 2**128 - 2)
    integer(rebalance["required_confirmations"], 1)
    tokens = {}
    for actor, variable in (("customer", "CUSTOMER"), ("other", "OTHER_CUSTOMER"),
                            ("operator", "OPERATOR")):
        token = read_protected("LAYERX_RAMP_" + variable + "_TOKEN_FILE").decode("ascii").removesuffix("\n")
        require(re.fullmatch(r"[A-Za-z0-9._~+/=-]{16,4096}", token), "token_encoding")
        tokens[actor] = token
    require(len(set(tokens.values())) == len(tokens), "distinct_principals_required")
    context = ssl.create_default_context(cafile=env("LAYERX_RAMP_CA_PEM"))
    context.minimum_version = ssl.TLSVersion.TLSv1_2
    context.load_cert_chain(env("LAYERX_RAMP_CLIENT_CERT_PEM"), str(protected_file("LAYERX_RAMP_CLIENT_KEY_FILE")))
    origins = {"customer": origin("LAYERX_RAMP_URL"), "operator": origin("LAYERX_RAMP_OPERATOR_URL")}
    revision = subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"], text=True).strip()
    expected_revision = env("LAYERX_RAMP_EXPECTED_REVISION")
    expected_digest = env("LAYERX_RAMP_EXPECTED_SOURCE_DIGEST")
    require(re.fullmatch(r"[0-9a-f]{40}", expected_revision) and revision == expected_revision,
            "driver_revision_mismatch")
    hex32(expected_digest)
    report["source_revision"] = revision
    current_step = "runtime_source_binding"
    recovered = request("operator", "operator", "POST", "/internal/v1/journal/recover", {})[0]
    require(recovered.get("recovered") is True, "verified_journal_recovery")
    for endpoint in ("customer", "operator"):
        ready = request(endpoint, None, "GET", "/readyz")[0]
        require(ready.get("ready") is True and ready.get("external_custody") is True
                and ready.get("build_revision") == expected_revision
                and ready.get("build_source_digest") == expected_digest
                and ready.get("journal", {}).get("ready") is True
                and ready["journal"]["halted"] is False
                and ready["journal"]["recovery_required"] is False, "runtime_source_mismatch")
        custody(ready)
        for contract in ("provider", "compliance", "paxeer"):
            require(ready.get(contract + "_contract") == "layerx-ramp-" + contract + "-v1", "runtime_contract")
    report["runtime_source_bound"] = True
    report["unified_endpoint_exercised"] = public_prefix == "/v1/ramp"
    report["runtime_source_digest"] = expected_digest
    current_step = "authorization_refusals"
    invalid_off = dict(inputs["off"], order_id=inputs["off"]["order_id"] + "-missing-grant")
    create(invalid_off, None, statuses=(400,), error="request_refused")
    invalid_on = dict(inputs["on"], order_id=inputs["on"]["order_id"] + "-unexpected-grant")
    create(invalid_on, grant, statuses=(400,), error="request_refused")
    cases["payer_grant_required"] = True
    create(inputs["on"], None, {"operator_account": rebalance["operator_account"]},
           (400,), "order_invalid")
    cases["customer_authority_rejected"] = True
    on_created = create(inputs["on"], None)
    custody(on_created)
    on_digest = octets(on_created["order_digest"])
    request("customer", None, "GET", "/v1/orders/" + on_digest.hex(), statuses=(401,), error="authentication_required")
    request("customer", "other", "GET", "/v1/orders/" + on_digest.hex(), statuses=(404,), error="order_not_found")
    cases["principal_isolation"] = True
    work_body = {"order_digest": list(on_digest), "action": "compliance", "account_sequence": None}
    request("operator", None, "POST", "/internal/v1/work", work_body, (401,), "operator_authentication_required")
    request("operator", "customer", "POST", "/internal/v1/work", work_body, (403,), "operator_authentication_refused")
    cases["operator_authorization"] = True
    presentation(read_order(on_digest))
    work(on_digest, "submit_layerx", status=(409,), error="operation_conflict")
    cases["out_of_order_refused"] = True
    repeated = create(inputs["on"], None)
    require(octets(repeated["order_digest"]) == on_digest, "order_replay_identity")
    create(inputs["on"], None, {"quote_id": inputs["on"]["quote_id"] + "-changed"}, (409,), "order_id_conflict")
    current_step = "on_ramp"
    presentation(work(on_digest, "compliance"))
    work(on_digest, "submit_provider")
    wait_stage(on_digest, "provider_settled")
    work(on_digest, "submit_layerx")
    on_done = wait_stage(on_digest, "done")
    report["on"] = verify_done(on_done)
    cases["on_ramp_receipt"] = True
    current_step = "off_ramp"
    off_created = create(inputs["off"], grant)
    custody(off_created)
    off_digest = octets(off_created["order_digest"])
    presentation(work(off_digest, "compliance"))
    work(off_digest, "submit_layerx", {"account_sequence": inputs["off"]["account_sequence"]},
         (400,), "native_payer_grant_authorization_required")
    work(off_digest, "submit_layerx", {"account_sequence": inputs["off"]["account_sequence"],
         "canonical_receive_payload": inputs["off"]["canonical_receive_payload"]})
    wait_stage(off_digest, "layerx_verified")
    work(off_digest, "submit_provider")
    off_done = wait_stage(off_digest, "done")
    report["off"] = verify_done(off_done)
    cases["off_ramp_receipt"] = True
    current_step = "order_recovery"
    for side, grant_value, digest, done in (("on", None, on_digest, on_done), ("off", grant, off_digest, off_done)):
        replay = create(inputs[side], grant_value)
        require(replay == done["presentation"], "completed_order_replay_changed")
        recovered = read_order(digest)
        require(recovered == done, "completed_order_recovery_changed")
    cases["order_idempotency"] = True
    current_step = "paxeer_own_account_rebalance"
    key = hex32(rebalance["idempotency_key"])
    submit = {"action": "submit", "asset": list(hex32(rebalance["asset"])),
              "amount": int(rebalance["amount"]), "idempotency_key": list(key)}
    request("operator", "customer", "POST", "/internal/v1/rebalances", submit,
            (403,), "operator_authentication_refused")
    request("operator", "operator", "POST", "/internal/v1/rebalances",
            dict(submit, operator_account=rebalance["operator_account"]), (400,), "rebalance_invalid")
    started, status = request("operator", "operator", "POST", "/internal/v1/rebalances", submit, (202, 503))
    if status == 503:
        require(started.get("error") == "paxeer_unavailable", "rebalance_failure_kind")
        started = request("operator", "operator", "POST", "/internal/v1/rebalances",
                          {"action": "reconcile", "idempotency_key": list(key)}, (202,))[0]
    custody(started)
    require(started.get("settlement_domain") == "paxeer", "rebalance_domain")
    operation = started["operation_id"]
    transaction = started["transaction_hash"]
    require(isinstance(operation, str) and operation and isinstance(transaction, str)
            and re.fullmatch(r"0x[0-9a-f]{64}", transaction), "rebalance_real_identity")
    poll = {"action": "poll", "idempotency_key": list(key), "operation_id": operation,
            "transaction_hash": transaction}
    for _ in range(120):
        observed = request("operator", "operator", "POST", "/internal/v1/rebalances", poll)[0]
        custody(observed)
        require(observed["operation_id"] == operation and observed["transaction_hash"] == transaction
                and observed["settlement_domain"] == "paxeer", "rebalance_identity_changed")
        required = integer(observed["required_confirmations"], rebalance["required_confirmations"])
        if observed["status"] == "final":
            integer(observed["confirmations"], required)
            break
        require(observed["status"] in ("announced", "missing", "pooled", "confirming", "displaced", "broadcast_unknown"),
                "rebalance_status")
        time.sleep(3)
    else:
        raise RuntimeError("rebalance_finality_deadline")
    rebalance_path = "/internal/v1/rebalances/" + key.hex()
    final = request("operator", "operator", "GET", rebalance_path)[0]
    require(final["status"] == "final" and final["operation_id"] == operation
            and final["transaction_hash"] == transaction
            and octets(final["idempotency_key"]) == key
            and final["operator_account"] == rebalance["operator_account"]
            and octets(final["asset"]) == hex32(rebalance["asset"])
            and final["amount"] == int(rebalance["amount"])
            and re.fullmatch(r"0x[0-9a-f]{64}", final["block_hash"]), "own_account_finality_binding")
    integer(final["confirmations"], required)
    custody(final)
    cases["rebalance_own_account_finality"] = True
    for replay in (submit, {"action": "reconcile", "idempotency_key": list(key)}):
        resumed = request("operator", "operator", "POST", "/internal/v1/rebalances", replay, (202,))[0]
        require(resumed["operation_id"] == operation and resumed["transaction_hash"] == transaction
                and resumed["status"] == "final", "rebalance_replay_changed")
    request("operator", "operator", "POST", "/internal/v1/rebalances",
            dict(submit, amount=submit["amount"] + 1), (409,), "operation_conflict")
    recovered = request("operator", "operator", "GET", rebalance_path)[0]
    require(recovered == final, "rebalance_recovery_changed")
    cases["rebalance_idempotency"] = True
    cases["recovery_same_identifiers"] = True
    cases["external_custody_labels"] = True
    report["rebalance"] = {"idempotency_key": key.hex(), "operation_id": operation,
                            "transaction_hash": transaction, "block_hash": final["block_hash"],
                            "confirmations": final["confirmations"], "required_confirmations": required,
                            "settlement_domain": "paxeer"}
    require(all(cases.values()), "incomplete_cases")
    report["qualified"] = True
    save("ramp-sandbox-result.json", report)
    print("ramp sandbox journey passed; protected evidence recorded")
except Exception as failure:
    report["failed_step"] = current_step
    report["failure_type"] = type(failure).__name__
    if type(failure) is RuntimeError:
        report["failure_code"] = str(failure)
    if evidence is not None:
        try:
            save("ramp-sandbox-result.json", report)
        except Exception:
            pass
    print("ramp sandbox journey refused at " + current_step + "; protected evidence retained", file=sys.stderr)
    sys.exit(1)
PY
fi
umask 077
: "${LAYERX_RAMP_RECEIPT_VERIFIER_BINARY:?}"
: "${LAYERX_RAMP_RECEIPT_VERIFIER_CONFIG:?}"
journey_directory=$(mktemp -d)
trap 'rm -rf "$journey_directory"' EXIT HUP INT TERM

: "${LAYERX_RAMP_URL:?}"
: "${LAYERX_RAMP_CA_PEM:?}"
: "${LAYERX_RAMP_CUSTOMER_TOKEN:?}"
: "${LAYERX_RAMP_OPERATOR_URL:?}"
: "${LAYERX_RAMP_OPERATOR_TOKEN:?}"
: "${LAYERX_RAMP_ON_QUOTE_ID:?}"
: "${LAYERX_RAMP_OFF_QUOTE_ID:?}"
: "${LAYERX_RAMP_OFF_GRANT_JSON:?}"
: "${LAYERX_RAMP_OFF_RECEIVER_SEQUENCE:?}"

customer() {
  curl --fail-with-body --silent --show-error --cacert "${LAYERX_RAMP_CA_PEM}" \
    -H "Authorization: Bearer ${LAYERX_RAMP_CUSTOMER_TOKEN}" "$@"
}

operator() {
  curl --fail-with-body --silent --show-error --cacert "${LAYERX_RAMP_CA_PEM}" \
    -H "Authorization: Bearer ${LAYERX_RAMP_OPERATOR_TOKEN}" "$@"
}

create_order() {
  direction="$1"
  quote="$2"
  grant="$3"
  order_id="sandbox-${direction}-${GITHUB_RUN_ID:-manual}-${GITHUB_RUN_ATTEMPT:-1}"
  customer -H 'Content-Type: application/json' -X POST "${LAYERX_RAMP_URL}/v1/orders" \
    --data "{\"order_id\":\"${order_id}\",\"quote_id\":\"${quote}\",\"payer_grant\":${grant}}"
}

work() {
  digest="$1"
  action="$2"
  sequence="$3"
  operator -H 'Content-Type: application/json' -X POST "${LAYERX_RAMP_OPERATOR_URL}/internal/v1/work" \
    --data "{\"order_digest\":${digest},\"action\":\"${action}\",\"account_sequence\":${sequence}}"
}

wait_stage() {
  digest_hex="$1"
  expected="$2"
  attempt=0
  while [ "${attempt}" -lt 60 ]; do
    response="$(customer "${LAYERX_RAMP_URL}/v1/orders/${digest_hex}")"
    stage="$(printf '%s' "${response}" | jq -r '.stage')"
    case "${stage}" in
      "${expected}") printf '%s' "${response}"; return 0 ;;
      compliance_refused|provider_refused|layerx_refused|manual_review|provider_reversed|reversed)
        printf '%s\n' "${response}" >&2
        return 1
        ;;
    esac
    attempt=$((attempt + 1))
    sleep 5
  done
  return 1
}

operator -H 'Content-Type: application/json' -X POST \
  "${LAYERX_RAMP_OPERATOR_URL}/internal/v1/journal/recover" --data '{}' \
  | jq -e '.recovered == true' >/dev/null
customer "${LAYERX_RAMP_URL}/readyz" \
  | jq -e '.ready == true and .journal.ready == true and .journal.halted == false and .journal.recovery_required == false' >/dev/null

receipt_evidence() {
  jq -e '.evidence |
    def hash32: type == "array" and length == 32 and
      all(.[]; type == "number" and . == floor and . >= 0 and . <= 255) and
      any(.[]; . != 0);
    if (.tx_hash | hash32) and (.receipt_digest | hash32) and
       .rung == "committed"
    then . else error("transaction receipt evidence required") end'
}

verify_done() {
  response="$1"
  keys=$(printf '%s' "$response" | receipt_evidence)
  tx_hash="0x$(printf '%s' "$keys" | jq -r '.tx_hash[]' | awk '{printf "%02x", $1}')"
  receipt_digest="$(printf '%s' "$keys" | jq -r '.receipt_digest[]' | awk '{printf "%02x", $1}')"
  order_digest="$(printf '%s' "$response" | jq -r '.order.order_digest[]' | awk '{printf "%02x", $1}')"
  # Each CLI output is created exclusively; keep per-transaction protected files.
  order_file="$journey_directory/order-${tx_hash}.json"
  result_file="$journey_directory/receipt-${tx_hash}.json"
  printf '%s' "$response" | jq -e '.order' >"$order_file"
  "$LAYERX_RAMP_RECEIPT_VERIFIER_BINARY" --config-file "$LAYERX_RAMP_RECEIPT_VERIFIER_CONFIG" \
    --order-file "$order_file" --transaction-hash "$tx_hash" --output-file "$result_file"
  printf '%s' "$response" | jq -e --slurpfile proof "$result_file" \
    --arg tx_hash "$tx_hash" --arg receipt_digest "$receipt_digest" --arg order_digest "$order_digest" \
    '.stage == "done" and .presentation.status == "done" and $proof[0].verified == true and
     $proof[0].maintained_batch == true and $proof[0].chain_id == 125 and
     $proof[0].order_digest == $order_digest and
     .presentation.transaction_hash == $tx_hash and
     $proof[0].transaction_hash == $tx_hash and
     $proof[0].receipt_digest == $receipt_digest and
     ($proof[0].height | type == "number" and . == floor and . > 0) and
     $proof[0].rung == "committed" and .evidence.rung == "committed" and
     .presentation.receipt_digest == .evidence.receipt_digest and
     .presentation.external_custody_label == $proof[0].external_custody_label' >/dev/null
}

on_created="$(create_order on-ramp "${LAYERX_RAMP_ON_QUOTE_ID}" null)"
on_digest="$(printf '%s' "${on_created}" | jq -c '.order_digest')"
on_hex="$(printf '%s' "${on_digest}" | jq -r '.[]' | awk '{printf "%02x", $1}')"
work "${on_digest}" compliance null >/dev/null
work "${on_digest}" submit_provider null >/dev/null
wait_stage "${on_hex}" provider_settled >/dev/null
work "${on_digest}" submit_layerx null >/dev/null
on_done="$(wait_stage "${on_hex}" done)"

off_created="$(create_order off-ramp "${LAYERX_RAMP_OFF_QUOTE_ID}" "${LAYERX_RAMP_OFF_GRANT_JSON}")"
off_digest="$(printf '%s' "${off_created}" | jq -c '.order_digest')"
off_hex="$(printf '%s' "${off_digest}" | jq -r '.[]' | awk '{printf "%02x", $1}')"
work "${off_digest}" compliance null >/dev/null
work "${off_digest}" submit_layerx "${LAYERX_RAMP_OFF_RECEIVER_SEQUENCE}" >/dev/null
wait_stage "${off_hex}" layerx_verified >/dev/null
work "${off_digest}" submit_provider null >/dev/null
off_done="$(wait_stage "${off_hex}" done)"

printf '%s' "${on_done}" | jq -e '.presentation.status == "done" and .presentation.receipt_digest != null and .presentation.external_custody_label != ""' >/dev/null
printf '%s' "${off_done}" | jq -e '.presentation.status == "done" and .presentation.receipt_digest != null and .presentation.provider_evidence_digest != null and .presentation.external_custody_label != ""' >/dev/null

verify_done "$on_done"
verify_done "$off_done"
