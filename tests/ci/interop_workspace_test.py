"""Run the interop workspace with individually owned baseline failures."""

import argparse
import json
from pathlib import Path
import re
import shlex
import subprocess


ROOT = Path(__file__).resolve().parents[2]
MANIFEST = "services/interop/Cargo.toml"
BASELINE = ROOT / "tests/ci/data/interop_test_baseline.json"
RETIRING_TASKS = {
    "layerx-bridge-relayer": "22.39.124",
    "layerx-committed-proof": "22.39.125",
    "layerx-fiat": "22.39.125",
    "layerx-portable": "22.39.125",
    "layerx-mirror": "22.39.125",
    "layerx-gas-station": "22.39.126",
    "layerx-interop-gateway": "22.39.126",
    "layerx-interop-service": "22.39.126",
    "layerx-migrate": "22.39.126",
    "layerx-ucp": "22.39.127",
    "layerx-x402": "22.39.127",
    "x-websearch": "22.39.127",
}


def read_baseline():
    entries = json.loads(BASELINE.read_text())
    if not isinstance(entries, list):
        raise ValueError(f"{BASELINE}: expected a list of individual failures")
    seen = set()
    for entry in entries:
        if not isinstance(entry, dict) or set(entry) != {
            "member", "target", "qualified_test_name", "first_error_line", "retiring_task"
        }:
            raise ValueError(f"{BASELINE}: invalid baseline entry: {entry!r}")
        if not all(isinstance(value, str) and value.strip() for value in entry.values()):
            raise ValueError(f"{BASELINE}: fields must be nonempty strings: {entry!r}")
        if RETIRING_TASKS.get(entry["member"]) != entry["retiring_task"]:
            raise ValueError(f"{BASELINE}: missing or incorrect retiring task: {entry!r}")
        if not re.fullmatch(r"--lib|--(?:test|bin) [\w-]+", entry["target"]):
            raise ValueError(f"{BASELINE}: expected an individual Cargo target: {entry!r}")
        if not re.fullmatch(r"\w+(?:::\w+)*", entry["qualified_test_name"]):
            raise ValueError(f"{BASELINE}: expected a qualified test name: {entry!r}")
        if "\n" in entry["first_error_line"] or "\r" in entry["first_error_line"]:
            raise ValueError(f"{BASELINE}: expected one error line: {entry!r}")
        key = (entry["member"], entry["target"], entry["qualified_test_name"])
        if key in seen:
            raise ValueError(f"{BASELINE}: duplicate failure: {key!r}")
        seen.add(key)
    return entries


def workspace_test_command(cargo="cargo", manifest_path=MANIFEST):
    executable = shlex.split(cargo)
    if not executable:
        raise ValueError("The Cargo command must not be empty")
    return [
        *executable, "test", "--locked", "--manifest-path", manifest_path,
        "--workspace", "--no-fail-fast", "--", "--exact",
        *(argument for entry in read_baseline()
          for argument in ("--skip", entry["qualified_test_name"])),
    ]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cargo", default="cargo")
    parser.add_argument("--manifest-path", default=MANIFEST)
    args = parser.parse_args()
    command = workspace_test_command(args.cargo, args.manifest_path)
    for entry in read_baseline():
        print(
            f"Interop baseline: {entry['member']} {entry['target']} "
            f"{entry['qualified_test_name']} | retiring task {entry['retiring_task']} | "
            f"{entry['first_error_line']}",
            flush=True,
        )
    print(shlex.join(command), flush=True)
    return subprocess.run(command, cwd=ROOT, check=False).returncode


if __name__ == "__main__":
    raise SystemExit(main())
