"""Keep the emulator response-code hex files aligned with their execution fixture."""

import argparse
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
FIXTURE = ROOT / "tests/fixtures/programs/emulator-response-code"
FIELDS = {
    "receipt.hex": ("receipt",),
    "terminal.hex": ("terminal_payload",),
    "call-graph.hex": ("call_graph",),
    "signed-activity.hex": ("retained_signed_activity",),
    "sequencer-public.hex": ("authority", "sequencer_public_key"),
}


def fixture_bytes(document, keys):
    value = document
    for key in keys:
        value = value[key]
    if not isinstance(value, str) or not value or value != value.lower():
        raise ValueError(f"invalid fixture hexadecimal: {'.'.join(keys)}")
    decoded = bytes.fromhex(value)
    if decoded.hex() != value:
        raise ValueError(f"noncanonical fixture hexadecimal: {'.'.join(keys)}")
    return value + "\n"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    arguments = parser.parse_args()
    document = json.loads((FIXTURE / "execution.json").read_text())
    receipt = bytes.fromhex(document["receipt"])
    if len(receipt) < 490 or receipt[-490:-486] != b"PRG4":
        raise ValueError("response-code fixture has no V4 terminal outcome")
    for name, keys in FIELDS.items():
        path = FIXTURE / name
        contents = fixture_bytes(document, keys)
        if arguments.check:
            if path.read_text() != contents:
                raise ValueError(f"response-code fixture copy differs: {name}")
        else:
            path.write_text(contents)


if __name__ == "__main__":
    main()
