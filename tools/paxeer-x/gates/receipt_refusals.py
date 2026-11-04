"""Check the V3 SDK copy against the retained sequencer receipt producer output."""

import argparse
import json
from pathlib import Path

SOURCE = "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
OUTPUT = "sdk/platform/conformance/fixtures/vector-v3-receipt-refusals.json"
PRODUCER = "rpc/sequencer_receipt_vectors_test.go"


def expected_cases(root):
    source = json.loads((root / SOURCE).read_text())
    if source["provenance"]["generator"] != PRODUCER:
        raise ValueError("receipt fixture must come from the retained sequencer producer")
    cases = [case for case in source["cases"] if case["vector"] == "V3"]
    if len(cases) != 6 or len({case["id"] for case in cases}) != 6:
        raise ValueError("all six genuine receipt refusal cases required")
    if any(not case["expected"]["refusedAt"] or not case["signedReceipt"] for case in cases):
        raise ValueError("signed refusal evidence required")
    return cases


def check(root):
    expected = expected_cases(root)
    output = json.loads((root / OUTPUT).read_text())
    if (output["name"] != "vector-v3-receipt-refusals"
            or output["chainId"] != 125
            or output["provenance"]["source"] != SOURCE
            or output["provenance"]["generator"] != PRODUCER
            or output["cases"] != expected):
        raise ValueError("receipt refusal fixture drift; regenerate with " + PRODUCER)
    return len(expected)


def main():
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument("--check", action="store_true", required=True)
    parser.parse_args()
    count = check(Path(__file__).resolve().parents[3])
    print("receipt refusal fixture: " + str(count) + " producer cases match")


if __name__ == "__main__":
    main()
