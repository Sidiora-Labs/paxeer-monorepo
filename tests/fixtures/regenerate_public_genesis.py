import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]
DIRECTORIES = ("public-testnet-genesis", "public-testnet-genesis-v3")
AUDIT = ROOT / "tests/protocol/lxp_test_audit_m966.h"


def regenerate_public_genesis(kernel_suite):
    audit = AUDIT.read_text()
    for name in DIRECTORIES:
        fixture = ROOT / "tests/fixtures" / name
        expected = json.loads((fixture / "expected.json").read_text())
        seed = hashlib.sha256(f"layerx-public-fixture:{name}".encode()).digest()
        with tempfile.TemporaryDirectory(prefix=".regenerate-", dir=fixture) as work:
            work = Path(work)
            key = work / "signer.key"
            key.write_bytes(seed)
            key.chmod(0o600)
            public = subprocess.run(
                ["openssl", "pkey", "-inform", "DER", "-pubout", "-outform", "DER"],
                input=bytes.fromhex("302e020100300506032b657004220420") + seed,
                capture_output=True,
                check=True,
            ).stdout[-32:]
            output = work / "genesis"
            subprocess.run(
                [
                    str(kernel_suite),
                    "--build-genesis-artifacts",
                    str(fixture / "genesis-request.lxgb"),
                    str(key),
                    str(output),
                ],
                cwd=ROOT,
                check=True,
            )
            registration = (output / "paxeer-registration-request.lxrr").read_bytes()
            if (
                len(public) != 32
                or len(registration) != 73
                or registration[:5] != b"LXRR\x01"
                or int.from_bytes(registration[5:9], "big") != expected["network_id"]
            ):
                raise ValueError(f"Unexpected producer output for {name}")
            old_hash = expected["sha256"]["genesis.manifest"]
            if audit.count(old_hash) != 1:
                raise ValueError(f"Audit must pin exactly one manifest hash for {name}")
            root, receipt = registration[9:41].hex(), registration[41:].hex()
            if root != expected["canonical_state_root"]:
                expected["previous_canonical_state_root"] = expected["canonical_state_root"]
                expected["previous_receipt_state_root"] = expected["receipt_state_root"]
            expected["canonical_state_root"] = root
            expected["receipt_state_root"] = receipt
            expected["sequencer_public_key"] = public.hex()
            for artifact in expected["sha256"]:
                data = (output / artifact).read_bytes()
                (fixture / artifact).write_bytes(data)
                expected["sha256"][artifact] = hashlib.sha256(data).hexdigest()
            (fixture / "sequencer.public").write_bytes(public)
            (fixture / "expected.json").write_text(
                json.dumps(expected, indent=2, sort_keys=True) + "\n"
            )
            audit = audit.replace(old_hash, expected["sha256"]["genesis.manifest"], 1)
            print(f"{name}: {expected['sha256']['genesis.manifest']}")
    AUDIT.write_text(audit)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--kernel-suite", type=Path, required=True)
    args = parser.parse_args()
    regenerate_public_genesis(args.kernel_suite.resolve())


if __name__ == "__main__":
    main()
