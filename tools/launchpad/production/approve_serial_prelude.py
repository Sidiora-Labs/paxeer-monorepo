"""Reproduce the task 22.33.973 approval from the committed reviewed inventory."""

import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import re

SOURCE = Path(__file__).resolve().parent
FIXTURES = SOURCE / "tests/fixtures/repair_22_33_973"
LEGACY_PRELUDE_SHA256 = "9899f10158a075d02a90a22a9f826762e86b8f018791a5f2bd93296d70737c64"
APPROVAL = "22.33.973"


def serial_gate():
    loader = importlib.util.spec_from_file_location("prelude_approval_gate", SOURCE / "serial_gate.py")
    gate = importlib.util.module_from_spec(loader)
    loader.loader.exec_module(gate)
    return gate


def migrate_approved_prelude(source, destination):
    gate = serial_gate()
    document = json.loads(gate._safe_file(source, LEGACY_PRELUDE_SHA256))
    paths = document["paths"]
    if (
        document["path_count"] != 387
        or len(paths) != 387
        or len({entry["path"] for entry in paths}) != 387
        or document["base_ownership_sha256"] != gate.EXACT_SERIAL_OWNERSHIP_SHA256
        or document["stage_prelude"] != gate.APPROVED_STAGE_PRELUDE
        or set(document["root_exclusive_control_paths"]) != {
            "tools/production/serial_gate.py", "tools/production/tests/test_serial_gate.py"
        }
        or any(entry["path"] in gate.PRELUDE_CONTROL_PATHS for entry in paths)
    ):
        raise gate.SerialGateError("approval requires the exact historical prelude inventory")
    gate._safe_file(SOURCE / "control_plane.py")
    # Preserve every historical patch allocation. The new reader is recorded as
    # root-owned source and must never become an additional patch permission.
    document["paths"] = sorted([*paths, dict(gate.PRELUDE_CONTROL_ENTRY)], key=lambda entry: entry["path"])
    document["path_count"] = len(document["paths"])
    document["root_exclusive_control_paths"] = sorted(gate.PRELUDE_CONTROL_PATHS)
    data = (json.dumps(document, sort_keys=True, indent=2) + "\n").encode()
    destination = Path(destination)
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_bytes(data)
    return hashlib.sha256(data).hexdigest()


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--approval", required=True, choices=[APPROVAL])
    parser.parse_args(argv)
    target = FIXTURES / "approved-prelude-ownership.json"
    identity = migrate_approved_prelude(FIXTURES / "reviewed-prelude-ownership.json", target)
    runner = SOURCE / "serial_gate.py"
    source, count = re.subn(
        r"(?m)^PRELUDED_SERIAL_OWNERSHIP_SHA256='[0-9a-f]{64}'$",
        f"PRELUDED_SERIAL_OWNERSHIP_SHA256='{identity}'",
        runner.read_text(),
    )
    if count != 1:
        raise ValueError("exactly one generated prelude binding required")
    runner.write_text(source)
    registry_path = SOURCE / "bootstrap/check-registry.json"
    registry = json.loads(registry_path.read_bytes())
    task = registry["tasks"]["1.99"]
    task["serial_gate_policy"]["serial_integration"]["source_policy"]["ownership_sha256"] = identity
    gate = serial_gate()
    task["policy_sha256"] = gate.digest(task["serial_gate_policy"])
    registry_path.write_text(json.dumps(registry, indent=2) + "\n")
    review = {
        "approval": "task:22.33.973/do_9",
        "source_sha256": LEGACY_PRELUDE_SHA256,
        "ownership_sha256": identity,
        "path_count": 388,
        "patch_path_count": 387,
        "added_control": gate.PRELUDE_CONTROL_ENTRY,
        "control_source_sha256": hashlib.sha256(gate._safe_file(SOURCE / "control_plane.py")).hexdigest(),
        "root_exclusive_control_paths": sorted(gate.PRELUDE_CONTROL_PATHS),
        "generator_command": "python3 tools/launchpad/production/approve_serial_prelude.py --approval 22.33.973",
    }
    (SOURCE / "bootstrap/prelude-review-22.33.973.json").write_text(json.dumps(review, indent=2) + "\n")
    print(json.dumps(review))


if __name__ == "__main__":
    main()
