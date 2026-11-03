"""Rewrite captured block transactions for this chain's protobuf registry.

Run with python3 rpc/tests/mock_data/retype.py. Only the encoded
block transaction strings change; protobuf lengths and captured state stay intact.
"""

import base64
import json
from pathlib import Path


SOURCE = b"/seiprotocol.seichain."
TARGET = b"/paxprotocol.paxchain."


def normalize_fixture(path: Path) -> int:
    original = path.read_text()
    fixture = json.loads(original)
    replacements = {}
    for trace in fixture["result"]["tendermint"]["traces"]:
        if trace["endpoint"] not in ("Block", "BlockByHash"):
            continue
        for encoded in trace["response"]["block"]["data"]["txs"] or []:
            transaction = base64.b64decode(encoded, validate=True)
            rewritten = transaction.replace(SOURCE, TARGET)
            if len(rewritten) != len(transaction):
                raise ValueError(f"{path}: transaction length changed")
            if rewritten != transaction:
                replacements[encoded] = base64.b64encode(rewritten).decode("ascii")

    normalized = original
    for encoded, rewritten in replacements.items():
        normalized = normalized.replace(json.dumps(encoded), json.dumps(rewritten))
    if normalized != original:
        path.write_text(normalized)
    return len(replacements)


def main() -> None:
    if len(SOURCE) != len(TARGET):
        raise ValueError("type URL namespaces must have the same byte length")
    root = Path(__file__).resolve().parent
    for directory in ("blocks", "transactions"):
        for path in sorted((root / directory).glob("*.json")):
            changed = normalize_fixture(path)
            if changed:
                print(f"{path.relative_to(root)}: normalized {changed} transactions")


if __name__ == "__main__":
    main()
