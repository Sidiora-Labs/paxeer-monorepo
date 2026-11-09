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


def read_baseline(baseline_path=BASELINE):
    baseline_path = Path(baseline_path)
    entries = json.loads(baseline_path.read_text())
    if not isinstance(entries, list):
        raise ValueError(f"{baseline_path}: expected a list of individual failures")
    seen = set()
    for entry in entries:
        if not isinstance(entry, dict) or set(entry) != {
            "member", "target", "qualified_test_name", "first_error_line", "retiring_task"
        }:
            raise ValueError(f"{baseline_path}: invalid baseline entry: {entry!r}")
        if not all(isinstance(value, str) and value.strip() for value in entry.values()):
            raise ValueError(f"{baseline_path}: fields must be nonempty strings: {entry!r}")
        if RETIRING_TASKS.get(entry["member"]) != entry["retiring_task"]:
            raise ValueError(f"{baseline_path}: missing or incorrect retiring task: {entry!r}")
        if not re.fullmatch(r"--lib|--(?:test|bin) [\w-]+", entry["target"]):
            raise ValueError(f"{baseline_path}: expected an individual Cargo target: {entry!r}")
        if not re.fullmatch(r"\w+(?:::\w+)*", entry["qualified_test_name"]):
            raise ValueError(f"{baseline_path}: expected a qualified test name: {entry!r}")
        if "\n" in entry["first_error_line"] or "\r" in entry["first_error_line"]:
            raise ValueError(f"{baseline_path}: expected one error line: {entry!r}")
        key = (entry["member"], entry["target"], entry["qualified_test_name"])
        if key in seen:
            raise ValueError(f"{baseline_path}: duplicate failure: {key!r}")
        seen.add(key)
    return entries


def workspace_test_command(cargo="cargo", manifest_path=MANIFEST, baseline_path=BASELINE):
    executable = shlex.split(cargo)
    if not executable:
        raise ValueError("The Cargo command must not be empty")
    entries = read_baseline(baseline_path)
    metadata = subprocess.run(
        [*executable, "metadata", "--locked", "--no-deps", "--format-version", "1",
         "--manifest-path", str(manifest_path)],
        cwd=ROOT, capture_output=True, text=True, check=True,
    )
    workspace = json.loads(metadata.stdout)
    member_ids = set(workspace["workspace_members"])
    commands = []
    identities = set()
    for package in workspace["packages"]:
        if package["id"] not in member_ids:
            continue
        base = [
            *executable, "test", "--locked", "--manifest-path", str(manifest_path),
            "--package", package["name"], "--no-fail-fast",
        ]
        for target in package["targets"]:
            kind = target["kind"]
            if any(item in kind for item in ("lib", "rlib", "dylib", "cdylib", "staticlib", "proc-macro")):
                selector = ["--lib"]
            elif len(kind) == 1 and kind[0] in ("bin", "test", "example", "bench"):
                selector = [f"--{kind[0]}", target["name"]]
            else:
                continue
            if target["test"]:
                identity = (package["name"], shlex.join(selector))
                identities.add(identity)
                skips = [
                    argument for entry in entries
                    if (entry["member"], entry["target"]) == identity
                    for argument in ("--skip", entry["qualified_test_name"])
                ]
                commands.append([*base, *selector, "--", "--exact", *skips])
            if selector == ["--lib"] and target["doctest"]:
                commands.append([*base, "--doc"])
    for entry in entries:
        if (entry["member"], entry["target"]) not in identities:
            raise ValueError(f"{baseline_path}: baseline target is not a workspace test target: {entry!r}")
    return commands


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cargo", default="cargo")
    parser.add_argument("--manifest-path", default=MANIFEST)
    parser.add_argument("--baseline-path", type=Path, default=BASELINE)
    args = parser.parse_args()
    commands = workspace_test_command(args.cargo, args.manifest_path, args.baseline_path)
    for entry in read_baseline(args.baseline_path):
        print(
            f"Interop baseline: {entry['member']} {entry['target']} "
            f"{entry['qualified_test_name']} | retiring task {entry['retiring_task']} | "
            f"{entry['first_error_line']}",
            flush=True,
        )
    result = 0
    for command in commands:
        print(shlex.join(command), flush=True)
        returncode = subprocess.run(command, cwd=ROOT, check=False).returncode
        if returncode and not result:
            result = returncode if returncode > 0 else 1
    return result


if __name__ == "__main__":
    raise SystemExit(main())
