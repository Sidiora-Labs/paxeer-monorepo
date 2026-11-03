"""Refresh fiat journey receipt metadata from the shared sequencer recording."""

import json
from pathlib import Path


def main():
    root = Path(__file__).resolve().parents[4]
    path = root / "services/interop/specs/conformance/fiat/sequenced-journeys.json"
    journeys = json.loads(path.read_text())
    for journey in journeys:
        declared = journey["sequencer"]
        fixture = json.loads((root / declared["fixture"]).read_text())
        case = next(case for case in fixture["cases"] if case["id"] == declared["case"])
        declared["transaction_hash"] = case["receiptJson"]["txHash"]
        if declared["rung"] == "submitted":
            if declared["receipt_digest"] is not None:
                raise ValueError("submitted journeys cannot carry a receipt digest")
        elif "source_receipt_digest" in declared:
            if declared["source_receipt_digest"] != case["receiptDigest"]:
                raise ValueError(
                    f"{journey['name']}: source receipt changed; re-sign the payment first"
                )
        else:
            declared["receipt_digest"] = case["receiptDigest"]
            if "receipt_digest" in journey["expect"]:
                journey["expect"]["receipt_digest"] = case["receiptDigest"]
    path.write_text(json.dumps(journeys, indent=2) + "\n")


if __name__ == "__main__":
    main()
