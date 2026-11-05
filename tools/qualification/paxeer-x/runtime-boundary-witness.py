#!/usr/bin/env python3
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
EVIDENCE = Path(os.environ.get("LAYERX_BOUNDARY_EVIDENCE", "/root/lx-ops/qualification/runtime-boundary"))
SCHEMA = "paxeer-x.runtime-boundary-artifacts.v1"
REQUIRED_CASES = frozenset({
    "integer_boundaries_restore_and_execute_the_real_engine_step",
    "nested_frames_locals_memory_growth_and_globals_restore_exactly",
    "independent_captures_have_identical_commitments_and_metering",
    "genuine_abi_storage_write_replays_with_the_matching_semantic_snapshot",
    "a_boundary_rejects_a_different_validated_module",
    "insufficient_retained_bytes_and_sparse_sampling_are_refused",
    "isolated_execution_cannot_manufacture_semantic_authority",
    "unreachable_preserves_the_original_trap_refusal",
})


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def digest(path):
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def source_inputs(root=ROOT):
    paths = {root / "rust-toolchain.toml", root / "tools/qualification/paxeer-x/runtime-boundary-witness.py"}
    # Include workspace declarations, SDK path dependencies and patched vendor crates.
    for directory, children, files in os.walk(root / "programs"):
        children[:] = [name for name in children if name not in {"target", ".git"}]
        for name in files:
            paths.add(Path(directory) / name)
    if (root / ".cargo").is_dir():
        paths.update(path for path in (root / ".cargo").rglob("*") if path.is_file())
    require(len(paths) > 2, "complete programs production source inventory required")
    return {str(path.relative_to(root)): digest(path) for path in sorted(paths)}


def identity():
    return {name: subprocess.check_output(["git", "rev-parse", ref], cwd=ROOT, text=True).strip()
            for name, ref in (("revision", "HEAD"), ("tree", "HEAD^{tree}"))}


def artifact(path):
    path = Path(path).resolve(strict=True)
    require(path.is_file(), "regular artifact required")
    return {"path": str(path), "sha256": digest(path)}


def publish_manifest(evidence, source, sources, binary, build_log, build_exit):
    require(build_exit == 0, "successful no-run build required")
    require(bool(sources), "complete production source inventory required")
    value = {"schema": SCHEMA, "source": source, "sources": sources,
             "executable": artifact(binary), "build_log": artifact(build_log),
             "build_exit": build_exit, "required_cases": sorted(REQUIRED_CASES)}
    path = evidence / "source.json"
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w") as output:
        json.dump(value, output, sort_keys=True)
        output.write("\n")
    return path


def admit_manifest(manifest, source, sources):
    require(set(source) == {"revision", "tree"} and all(
        isinstance(value, str) and re.fullmatch(r"[0-9a-f]{40}", value) for value in source.values()),
        "exact source revision and tree required")
    require(manifest.get("schema") == SCHEMA and manifest.get("source") == source,
            "revision-bound boundary manifest required")
    require(bool(sources) and manifest.get("sources") == sources,
            "exact complete production source inventory required")
    require(manifest.get("build_exit") == 0, "successful no-run build required")
    require(manifest.get("required_cases") == sorted(REQUIRED_CASES),
            "complete boundary case inventory required")
    for name in ("executable", "build_log"):
        saved = manifest[name]
        require(artifact(saved["path"]) == saved, "changed boundary artifact: " + name)
    binary = Path(manifest["executable"]["path"])
    require(os.access(binary, os.X_OK), "boundary artifact must be executable")
    return binary


def case_results(output, exit_code):
    require(exit_code == 0, "boundary executable failed")
    passed = re.findall(r"^test ([A-Za-z0-9_:]+) \.\.\. ok$", output, re.M)
    summaries = re.findall(
        r"^test result: ok\. (\d+) passed; (\d+) failed; (\d+) ignored; (\d+) measured; (\d+) filtered out;?", output, re.M)
    require(len(passed) == len(set(passed)) and REQUIRED_CASES <= set(passed),
            "required boundary cases did not execute")
    require(len(summaries) == 1 and tuple(map(int, summaries[0])) == (len(passed), 0, 0, 0, 0),
            "empty, ignored, filtered or incomplete boundary execution")
    return len(passed)


def main():
    manifest = json.loads((EVIDENCE / "source.json").read_text())
    source, sources = identity(), source_inputs()
    binary = admit_manifest(manifest, source, sources)
    log = EVIDENCE / "verify.log"
    with log.open("w") as output:
        result = subprocess.run([str(binary), "--nocapture", "--test-threads=1"], cwd=ROOT,
                                stdout=output, stderr=subprocess.STDOUT, timeout=540)
    (EVIDENCE / "verify.exit").write_text(str(result.returncode) + "\n")
    count = case_results(log.read_text(), result.returncode)
    admit_manifest(manifest, identity(), source_inputs())
    require(identity() == source, "source revision changed during boundary gate")
    print(f"PAXEER_X_GATE tests={count} skipped=0")
    print("exit=0 log=" + str(log))
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, KeyError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
