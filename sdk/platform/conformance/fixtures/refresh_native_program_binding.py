"""Record the native-v4 call with the Python SDK's real EIP-1559 signer."""

import hashlib
import json
from pathlib import Path
import sys


def main():
    fixtures = Path(__file__).resolve().parent
    root = fixtures.parents[3]
    sys.path.insert(0, str(root / "services/agent/sdk/python"))
    sys.path.insert(0, str(root / "tests/agent/sdk/python"))
    from test_evm_transaction import plain_transaction, signed_raw

    source = fixtures / "native-program-call-v4.json"
    fixture = json.loads(source.read_text())
    transaction = plain_transaction(
        chain_id=fixture["evm"]["chainId"],
        to=bytes.fromhex(fixture["evm"]["to"][2:]),
        gas_limit=2_000_000,
        data=bytes.fromhex(fixture["evm"]["calldata"][2:]),
    )
    recording = {
        "provenance": {
            "generator": str(Path(__file__).relative_to(root)),
            "source": str(source.relative_to(root)),
            "sourceSha256": hashlib.sha256(source.read_bytes()).hexdigest(),
            "signer": "tests/agent/sdk/python/test_evm_transaction.py:signed_raw",
        },
        "raw_transaction": signed_raw(transaction).hex(),
    }
    (fixtures / "native-program-binding-v4.json").write_text(
        json.dumps(recording, indent=2) + "\n"
    )


if __name__ == "__main__":
    main()
