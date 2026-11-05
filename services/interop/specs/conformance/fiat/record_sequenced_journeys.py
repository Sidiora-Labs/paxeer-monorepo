"""Import fiat payment recordings without changing signed producer output.

This authenticates recorded receipts and checks source correspondence. It does
not execute the EVM or the Rust adapter.
"""

import argparse
from copy import deepcopy
from hashlib import sha256
import json
from pathlib import Path

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey


RECORDER = "services/interop/specs/conformance/fiat/record_sequenced_journeys.py"
LEDGER = bytes.fromhex("000000000000000000000000000000000000101a")
PAYMENT_EXACT = bytes.fromhex(
    "c4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1"
)


def unhex(value):
    if not isinstance(value, str) or not value.startswith("0x"):
        raise ValueError("expected prefixed hex")
    return bytes.fromhex(value[2:])


def rlp(data, offset=0):
    """Decode canonical RLP, retaining list and byte-string distinctions."""
    if offset >= len(data):
        raise ValueError("truncated RLP")
    tag = data[offset]
    if tag < 128:
        return data[offset:offset + 1], offset + 1
    is_list = tag >= 192
    short = 192 if is_list else 128
    long = 247 if is_list else 183
    if tag <= long:
        size, start = tag - short, offset + 1
    else:
        width = tag - long
        start = offset + 1 + width
        length = data[offset + 1:start]
        if len(length) != width or length[0] == 0:
            raise ValueError("invalid RLP length")
        size = int.from_bytes(length, "big")
        if size < 56:
            raise ValueError("noncanonical long RLP")
    end = start + size
    if end > len(data):
        raise ValueError("truncated RLP payload")
    if not is_list:
        if size == 1 and data[start] < 128:
            raise ValueError("noncanonical RLP byte")
        return data[start:end], end
    values = []
    cursor = start
    while cursor < end:
        value, cursor = rlp(data, cursor)
        if cursor > end:
            raise ValueError("RLP child exceeds list")
        values.append(value)
    return values, end


def payment_call(case):
    raw = unhex(case["rawTransaction"])
    if not raw or raw[0] != 2:
        raise ValueError("expected signed EIP-1559 transaction")
    fields, end = rlp(raw, 1)
    if not isinstance(fields, list) or len(fields) != 12 or end != len(raw):
        raise ValueError("invalid signed EIP-1559 fields")
    if fields[0] != b"\x7d" or fields[5] != LEDGER or fields[6] != b"":
        raise ValueError("expected zero-value chain-125 ledger call")
    if not isinstance(fields[8], list) or fields[9] not in (b"", b"\x01"):
        raise ValueError("invalid access list or signature parity")
    for scalar in fields[10:12]:
        if not isinstance(scalar, bytes) or not 1 <= len(scalar) <= 32 or scalar[0] == 0:
            raise ValueError("invalid signature scalar")
    call = fields[7]
    if not isinstance(call, bytes) or len(call) != 196 or call[:4] != bytes.fromhex("78ea11db"):
        raise ValueError("expected exact payExact calldata")
    if any(call[100:116]) or any(call[132:156]):
        raise ValueError("noncanonical payExact integer padding")
    return call


def provider_reference(journey):
    facts = journey["provider_facts"]
    rail = {"card": 1, "bank": 2, "rtp": 3}[facts["rail"]]
    if facts["class"] != "settled":
        raise ValueError("credit requires settled provider facts")
    return sha256(
        b"LayerX/interop/fiat/idempotency/v1\0"
        + facts["provider"].encode()
        + b"\0"
        + facts["settlement"].encode()
        + bytes([rail, 3])
    ).digest()


def validate_capture(journey, case):
    """Require the reference in submitted bytes before inspecting the receipt."""
    call = payment_call(case)
    reference = provider_reference(journey)
    if call[164:196] != reference:
        raise ValueError(f"{journey['name']}: submitted reference is not provider-derived")
    facts = journey["provider_facts"]
    if (
        call[36:68] != unhex(facts["destination"])
        or call[68:100] != unhex(facts["asset"])
        or int.from_bytes(call[116:132], "big") != int(facts["amount"])
    ):
        raise ValueError("submitted payment terms differ from provider facts")
    view = case["receiptJson"]
    if view["rung"] != "receipted" or view["status"] != "0x1":
        raise ValueError("credit requires a successful execution receipt")
    receipt = unhex(case["signedReceipt"])
    if receipt != unhex(view["receipt"]) or receipt[:6] != bytes.fromhex("000252010002"):
        raise ValueError("receipt bytes differ from producer view")
    # Protocol v2: activity blob, sequence, previous-sequence-hash blob.
    if receipt[6:10] != (32).to_bytes(4, "big") or receipt[10:42] != unhex(view["txHash"]):
        raise ValueError("receipt activity differs from transaction identity")
    sequence = int(view["seq"], 16)
    if sequence <= 0 or int.from_bytes(receipt[42:50], "big") != sequence:
        raise ValueError("receipt sequence differs from producer view")
    if receipt[50:54] != (32).to_bytes(4, "big") or receipt[54:86] != unhex(case["prevSeqHash"]):
        raise ValueError("receipt previous sequence hash differs from recording")
    if receipt[-69:-64] != bytes.fromhex("0100000040"):
        raise ValueError("invalid sequencer signature framing")
    digest = "0x" + sha256(b"LXP/v1/receipt\0" + receipt[:-69] + b"\0").hexdigest()
    if digest != case["receiptDigest"]:
        raise ValueError("receipt digest differs from original signed bytes")
    pin = case["pinnedKey"]
    if (
        view["keyId"] != pin["keyId"]
        or view["signingKey"] != pin["publicKey"]
        or not pin["firstHeight"] <= int(view["height"], 16) <= pin["lastHeight"]
    ):
        raise ValueError("receipt key is outside the recorded authorization")
    try:
        Ed25519PublicKey.from_public_bytes(unhex(pin["publicKey"])).verify(
            receipt[-64:], unhex(digest)
        )
    except InvalidSignature as error:
        raise ValueError("invalid sequencer receipt signature") from error
    expected_topics = [PAYMENT_EXACT, reference, call[4:36], call[36:68]]
    logs = view["logs"]
    if len(logs) != 1:
        raise ValueError("expected one PaymentExact credit")
    log = logs[0]
    data = unhex(log["data"])
    if (
        unhex(log["address"]) != LEDGER
        or [unhex(topic) for topic in log["topics"]] != expected_topics
        or data != call[68:100] + call[100:132]
        or log["logIndex"] != "0x0"
    ):
        raise ValueError("PaymentExact log differs from submitted calldata")
    body = LEDGER + b"\x04" + b"".join(expected_topics) + sha256(data).digest()
    if receipt[:-69].count(body) != 1:
        raise ValueError("PaymentExact log is not in the unchanged signed receipt")
    return digest


def regenerate(journeys, root):
    """Validate every credit before returning any updated journey metadata."""
    root = Path(root).resolve()
    fixtures = {}
    result = deepcopy(journeys)
    for journey in result:
        declared = journey["sequencer"]
        fixture_path = (root / declared["fixture"]).resolve()
        fixture_path.relative_to(root)
        if fixture_path not in fixtures:
            fixture = json.loads(fixture_path.read_text(encoding="utf-8"))
            if fixture["chainId"] != 125:
                raise ValueError("expected chain-125 producer recording")
            cases = {case["id"]: case for case in fixture["cases"]}
            if len(cases) != len(fixture["cases"]):
                raise ValueError("duplicate producer case identity")
            fixtures[fixture_path] = cases
        case = fixtures[fixture_path][declared["case"]]
        journey["recorder"] = RECORDER
        if journey["expect"].get("state") != "credited":
            continue
        digest = validate_capture(journey, case)
        call = payment_call(case)
        declared.update(
            transaction_hash=case["receiptJson"]["txHash"],
            receipt_digest=digest,
            payment_reference="0x" + provider_reference(journey).hex(),
            idempotency_key="0x" + provider_reference(journey).hex(),
            payment_calldata="0x" + call.hex(),
            counterparty="0x" + call[4:36].hex(),
            expires_at=int.from_bytes(call[156:164], "big"),
        )
        declared.pop("source_receipt_digest", None)
        declared.pop("test_signing_seed", None)
        journey["expect"]["receipt_digest"] = digest
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args()
    root = Path(__file__).resolve().parents[5]
    path = Path(__file__).with_name("sequenced-journeys.json")
    journeys = json.loads(path.read_text(encoding="utf-8"))
    result = regenerate(journeys, root)
    path.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
