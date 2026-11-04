"""Produce and verify run-bound coverage inventories used by coverage-gate.sh."""

import argparse
import hashlib
import json
from pathlib import Path
import re
import time
import uuid


class InventoryError(ValueError):
    pass


def profile_name(module):
    return module.replace("/", "_") + ".out"


def module_inventory(modules):
    modules = list(modules)
    if not modules or len(set(modules)) != len(modules):
        raise InventoryError("expected module inventory is empty or duplicated")
    if any(not re.fullmatch(r"[A-Za-z0-9_./-]+", m) for m in modules):
        raise InventoryError("invalid module name")
    if len({profile_name(m) for m in modules}) != len(modules):
        raise InventoryError("module profile names collide")
    return modules


def write_manifest(path, manifest):
    path = Path(path)
    temporary = path.with_name(path.name + ".tmp")
    temporary.write_text(json.dumps(manifest, sort_keys=True) + "\n")
    temporary.replace(path)


def begin_run(cover_dir, manifest_path, modules):
    modules = module_inventory(modules)
    Path(cover_dir).mkdir(parents=True, exist_ok=True)
    manifest = {
        "version": 1,
        "run_id": uuid.uuid4().hex,
        "started_ns": time.time_ns(),
        "modules": modules,
        "results": {},
    }
    write_manifest(manifest_path, manifest)
    return manifest["run_id"]


def unique_keys(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise InventoryError(f"duplicate manifest key: {key}")
        result[key] = value
    return result


def read_manifest(manifest_path, run_id):
    try:
        manifest = json.loads(Path(manifest_path).read_text(), object_pairs_hook=unique_keys)
    except (OSError, ValueError) as error:
        raise InventoryError(f"cannot read run manifest: {error}") from error
    if not isinstance(manifest, dict) or manifest.get("version") != 1:
        raise InventoryError("invalid run manifest version")
    if not run_id or manifest.get("run_id") != run_id:
        raise InventoryError("stale run manifest: run id mismatch")
    started = manifest.get("started_ns")
    if type(started) is not int or not 0 < started <= time.time_ns():
        raise InventoryError("invalid run start time")
    if not isinstance(manifest.get("modules"), list):
        raise InventoryError("missing module inventory")
    module_inventory(manifest["modules"])
    if not isinstance(manifest.get("results"), dict):
        raise InventoryError("invalid module results")
    return manifest


def profile_facts(cover_dir, module, started_ns):
    path = Path(cover_dir) / profile_name(module)
    if not path.is_file() or path.is_symlink():
        raise InventoryError(f"missing regular profile: {module}")
    before = path.stat()
    data = path.read_bytes()
    after = path.stat()
    if (before.st_mtime_ns, before.st_size) != (after.st_mtime_ns, after.st_size):
        raise InventoryError(f"profile changed while reading: {module}")
    if before.st_mtime_ns < started_ns:
        raise InventoryError(f"stale profile: {module}")
    try:
        lines = data.decode("utf-8").splitlines()
    except UnicodeDecodeError as error:
        raise InventoryError(f"invalid coverage encoding: {module}") from error
    if not lines or lines[0] not in ("mode: set", "mode: count", "mode: atomic"):
        raise InventoryError(f"invalid coverage header: {module}")
    for line in lines[1:]:
        if not re.fullmatch(r"\S+:\d+\.\d+,\d+\.\d+ \d+ \d+", line):
            raise InventoryError(f"invalid coverage statement: {module}")
    return {
        "sha256": hashlib.sha256(data).hexdigest(),
        "mtime_ns": before.st_mtime_ns,
        "size": len(data),
    }, any(int(line.split()[-2]) > 0 for line in lines[1:])


def record_result(cover_dir, manifest_path, run_id, module, empty=None):
    """Call only after successful package discovery and, for profiles, go test."""
    manifest = read_manifest(manifest_path, run_id)
    if "completed_ns" in manifest or module not in manifest["modules"]:
        raise InventoryError("run is complete or module is unexpected")
    if module in manifest["results"]:
        raise InventoryError(f"duplicate result: {module}")
    if empty == "no-packages":
        if (Path(cover_dir) / profile_name(module)).exists():
            raise InventoryError(f"empty module has a retained profile: {module}")
        result = {"status": "empty", "reason": empty}
    else:
        facts, has_statements = profile_facts(cover_dir, module, manifest["started_ns"])
        if empty == "no-statements" and not has_statements:
            result = {"status": "empty", "reason": empty, "profile": facts}
        elif empty is None and has_statements:
            result = {"status": "profile", "profile": facts}
        else:
            raise InventoryError(f"profile needs an accurate explicit empty result: {module}")
    manifest["results"][module] = result
    write_manifest(manifest_path, manifest)


def validate_results(cover_dir, manifest, modules):
    expected = module_inventory(modules)
    if set(manifest["modules"]) != set(expected):
        raise InventoryError("run manifest differs from expected module inventory")
    if set(manifest["results"]) != set(expected):
        missing = sorted(set(expected) - set(manifest["results"]))
        raise InventoryError(f"missing or unexpected module results: {missing}")
    profiles = set()
    empty_modules = []
    for module in expected:
        result = manifest["results"][module]
        if not isinstance(result, dict):
            raise InventoryError(f"invalid result: {module}")
        if result == {"status": "empty", "reason": "no-packages"}:
            empty_modules.append((module, "no packages"))
            continue
        facts, has_statements = profile_facts(cover_dir, module, manifest["started_ns"])
        profiles.add(profile_name(module))
        if result.get("profile") != facts:
            raise InventoryError(f"profile changed since recording: {module}")
        if has_statements and result == {"status": "profile", "profile": facts}:
            continue
        if not has_statements and result == {
            "status": "empty", "reason": "no-statements", "profile": facts
        }:
            empty_modules.append((module, "no measurable statements"))
            continue
        raise InventoryError(f"invalid explicit empty result: {module}")
    present = {p.name for p in Path(cover_dir).glob("*.out")}
    if profiles != present:
        raise InventoryError("unrecorded or retained coverage profiles")
    return empty_modules


def finish_run(cover_dir, manifest_path, run_id):
    manifest = read_manifest(manifest_path, run_id)
    if "completed_ns" in manifest:
        raise InventoryError("run already completed")
    validate_results(cover_dir, manifest, manifest["modules"])
    manifest["completed_ns"] = time.time_ns()
    write_manifest(manifest_path, manifest)


def check_run(cover_dir, manifest_path, run_id, modules):
    manifest = read_manifest(manifest_path, run_id)
    completed = manifest.get("completed_ns")
    if type(completed) is not int or not manifest["started_ns"] <= completed <= time.time_ns():
        raise InventoryError("incomplete or invalid coverage run")
    empty_modules = validate_results(cover_dir, manifest, modules)
    if any(r.get("profile", {}).get("mtime_ns", 0) > completed for r in manifest["results"].values()):
        raise InventoryError("profile postdates completed run")
    return empty_modules


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("begin", "record", "finish", "check"))
    parser.add_argument("cover_dir")
    parser.add_argument("manifest")
    parser.add_argument("--modules")
    parser.add_argument("--run-id")
    parser.add_argument("--module")
    parser.add_argument("--empty", choices=("no-packages", "no-statements"))
    args = parser.parse_args(argv)
    try:
        if args.action in ("begin", "check") and not args.modules:
            raise InventoryError("expected --modules inventory is required")
        if args.action == "begin":
            print(begin_run(args.cover_dir, args.manifest, args.modules.split()))
        elif args.action == "record":
            record_result(args.cover_dir, args.manifest, args.run_id, args.module, args.empty)
        elif args.action == "finish":
            finish_run(args.cover_dir, args.manifest, args.run_id)
        else:
            for module, reason in check_run(args.cover_dir, args.manifest, args.run_id, args.modules.split()):
                print(f"{module}: EMPTY ({reason})")
    except (InventoryError, OSError) as error:
        parser.exit(1, f"coverage-gate: {error}\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
