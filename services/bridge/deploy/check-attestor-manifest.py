import json
import os
from pathlib import Path
import re
import sys


GOVERNANCE_STEP = "governance signer-set approval (22.31.402; decision 2522)"
MAX_ATTESTORS = 64


def unique_fields(pairs):
    fields = {}
    for key, value in pairs:
        if key in fields:
            raise ValueError(f"duplicate JSON field {key}")
        fields[key] = value
    return fields


def check_manifest(path):
    manifest = json.loads(path.read_text(), object_pairs_hook=unique_fields)
    if not isinstance(manifest, dict):
        raise ValueError("manifest must be a JSON object")
    if "pending_governance" in manifest:
        raise ValueError(
            f"pending_governance={json.dumps(manifest['pending_governance'])}: "
            f"awaiting {GOVERNANCE_STEP}; bridge governance must supply the "
            "approved public signer set and threshold before the upgrade, "
            "replace the signer slots and remove pending_governance at approval"
        )
    if set(manifest) != {"attestors", "threshold"}:
        raise ValueError("manifest must contain only attestors and threshold")
    signers = manifest["attestors"]
    if not isinstance(signers, list) or not 1 <= len(signers) <= MAX_ATTESTORS:
        raise ValueError(f"attestors must contain between 1 and {MAX_ATTESTORS} signers")
    previous = None
    for index, signer in enumerate(signers):
        if not isinstance(signer, str) or not re.fullmatch(
            r"0x[0-9a-fA-F]{40}", signer
        ):
            raise ValueError(f"attestors[{index}]: expected a live 20-byte address")
        address = bytes.fromhex(signer[2:])
        if len(set(address)) == 1:
            raise ValueError(f"attestors[{index}]: repeated-byte placeholder address")
        if previous is not None and address <= previous:
            raise ValueError(
                "attestors must be distinct and in strictly ascending order"
            )
        previous = address
    threshold = manifest["threshold"]
    if type(threshold) is not int or not 1 <= threshold <= len(signers):
        raise ValueError(f"threshold must be an integer between 1 and {len(signers)}")


def main():
    path = Path(
        os.environ.get("PAXEER_BRIDGE_ATTESTOR_MANIFEST")
        or Path(__file__).with_name("attestors.json")
    )
    try:
        check_manifest(path)
    except (OSError, UnicodeError, ValueError) as error:
        print(f"attestor manifest: {path}: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
