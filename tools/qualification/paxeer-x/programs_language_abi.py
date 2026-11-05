#!/usr/bin/env python3
import argparse
import hashlib
import json
import os
import re
from pathlib import Path
import shutil
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[3]
LANGUAGES = ("c", "rust", "assemblyscript")
CASES = ("basic", "oracle", "refusal", "call", "account", "denied")
FIELDS = ("result_code", "response_hex", "refusal_hex", "effects_hex", "events_hex", "calldata_hex")


def sha(path):
    with Path(path).open("rb") as source:
        return hashlib.file_digest(source, "sha256").hexdigest()


def save(path, value):
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    path.chmod(0o600)


def inventory():
    paths = subprocess.run(["git", "ls-files", "-z", "--", "Makefile", "src", "include",
        "programs", "tests/programs", "tests/test_programs_oracle_read.c",
        "tools/qualification/paxeer-x/programs_language_abi.py"], cwd=ROOT,
        check=True, capture_output=True).stdout.split(b"\0")
    return {os.fsdecode(p): sha(ROOT / os.fsdecode(p)) for p in paths if p}


def run(command, evidence, name, deadline, stdout=None):
    remaining = int(deadline - time.time())
    if remaining <= 0:
        raise TimeoutError("task cutoff reached")
    with (evidence / (name + ".log")).open("wb") as log:
        process = subprocess.run(["timeout", "--signal=TERM", "--kill-after=5s", str(remaining),
            *map(str, command)], cwd=ROOT, stdout=stdout or log, stderr=log)
    (evidence / (name + ".exit")).write_text(str(process.returncode) + "\n")
    if process.returncode:
        raise subprocess.CalledProcessError(process.returncode, command)


def compiler(name, override):
    value = os.environ.get(override) or shutil.which(name)
    if not value or not Path(value).is_file():
        raise FileNotFoundError("missing genuine toolchain: " + name)
    return Path(value).absolute()


def admitted_abis():
    policy = (ROOT / "programs/sdk/rust/src/abi_policy.rs").read_text()
    constants = {name: int(value) for name, value in re.findall(
        r"pub const (ABI_V\d+_VERSION): u16 = (\d+);", policy)}
    match = re.search(r"const fn supported\(.*?match requested \{(.*?)=>\s*\{\s*Ok\(\(\)\)", policy, re.S)
    if not match:
        raise RuntimeError("canonical ABI admission policy could not be read")
    names = re.findall(r"ABI_V\d+_VERSION", match[1])
    versions = tuple(constants[name] for name in names)
    if not versions or len(set(versions)) != len(versions) or tuple(sorted(versions)) != versions:
        raise RuntimeError("canonical admitted ABI inventory is invalid")
    return versions


def unallocated_abi():
    versions = admitted_abis()
    return next(version for version in range(1, 65536) if version not in versions)


def canonical_rows():
    base = ROOT / "programs/crates/layerx-programs-runtime/src/abi"
    source = (base / "manifest.rs").read_text()
    rows = []
    for version in admitted_abis():
        if version == 1:
            table = re.search(r"pub const HOST_FUNCTIONS:.*?= \[(.*?)\n\];",
                              (base / "mod.rs").read_text(), re.S)
            entries = re.findall(r'name: "([^"\n]+)",\s*signature: "([^"\n]+)"', table[1]) if table else []
        else:
            table = re.search(r"pub const ABI_V" + str(version) + r"_HOST_FUNCTIONS:.*?=\s*\[(.*?)\];", source, re.S)
            entries = re.findall(r'host\(\s*"([^"\n]+)",\s*"([^"\n]+)"\s*,?\s*\)', table[1]) if table else []
        if not entries:
            raise RuntimeError("missing canonical versioned host table")
        rows.extend((version, "layerx_v" + str(version), name, signature) for name, signature in entries)
    return rows


def manifest_parity(path=None):
    rows = canonical_rows()
    if len({(row[1], row[2]) for row in rows}) != len(rows):
        raise RuntimeError("duplicate canonical host function")
    checksums = dict(line.split() for line in (ROOT / "programs/abi-frozen.sha256").read_text().splitlines()
                     if line and not line.startswith("#"))
    for version in admitted_abis():
        encoded = version.to_bytes(2, "big")
        previous = None
        for introduced, module, name, signature in rows:
            if introduced > version:
                continue
            if module != previous:
                encoded += module.encode() + b"\0"
                previous = module
            encoded += (name + signature).encode() + b"\0"
        vector = (ROOT / f"programs/tests/vectors/abi-v{version}.hex").read_bytes()
        frozen = bytes.fromhex(vector.decode("ascii"))
        if encoded != frozen or hashlib.sha256(vector).hexdigest() != checksums[str(version)]:
            raise RuntimeError("canonical host table differs from frozen runtime manifest")
    # The published historical TSV is frozen separately; its v5 publication belongs to the SDK owner.
    shared = ROOT / "programs/sdk/abi-manifest.tsv" if path is None else Path(path)
    observed = [tuple(line.split("\t")) for line in shared.read_text().splitlines()]
    expected = [tuple(map(str, row)) for row in rows if path is not None or row[0] <= 4]
    if observed != expected:
        raise RuntimeError("shared ABI table differs from canonical host tables")
    return rows


def write_manifest(path):
    rows = manifest_parity()
    with Path(path).open("x") as stream:
        stream.write("".join("\t".join(map(str, row)) + "\n" for row in rows))
    manifest_parity(path)
    return Path(path)


LINKER_TESTS = (
    "finding_015_import_admission_matches_every_runtime_namespace",
    "finding_015_v5_sdk_import_reaches_the_real_validator_and_linker",
)


def linker_output(output):
    passed = re.findall(r"^test ([A-Za-z0-9_:]+) \.\.\. ok$", output, re.M)
    summaries = re.findall(r"^test result: ok\. (\d+) passed; (\d+) failed; (\d+) ignored;", output, re.M)
    if (len(passed) != len(LINKER_TESTS) or set(passed) != set(LINKER_TESTS)
            or summaries != [(str(len(LINKER_TESTS)), "0", "0")]):
        raise RuntimeError("historical and v5 real guest validation/linker cases absent, failed or skipped")
    return passed


def rejected_artifacts(path, evidence):
    original = Path(path).read_bytes()
    if original[:8] != b"\0asm\1\0\0\0":
        raise RuntimeError("compiled WASM artifact required")
    def leb(cursor):
        value = 0
        for shift in range(0, 35, 7):
            byte = original[cursor]
            cursor += 1
            value |= (byte & 127) << shift
            if not byte & 128:
                return value, cursor
        raise RuntimeError("malformed compiled WASM length")
    sections = {}
    cursor = 8
    while cursor < len(original):
        identifier = original[cursor]
        length, start = leb(cursor + 1)
        end = start + length
        if end > len(original):
            raise RuntimeError("compiled WASM section exceeds artifact")
        sections[identifier] = (start, end)
        cursor = end
    start, end = sections[2]
    count, cursor = leb(start)
    oracle_type = None
    import_offset = None
    for _ in range(count):
        length, cursor = leb(cursor)
        if cursor + length > end:
            raise RuntimeError("compiled import module exceeds section")
        module = original[cursor:cursor + length]
        cursor += length
        length, cursor = leb(cursor)
        if cursor + length >= end:
            raise RuntimeError("compiled import name exceeds section")
        name_offset = cursor
        name = original[cursor:cursor + length]
        cursor += length
        if original[cursor] != 0:
            raise RuntimeError("compiled guest requires canonical function imports")
        type_index, cursor = leb(cursor + 1)
        if cursor > end:
            raise RuntimeError("compiled import type exceeds section")
        if module == b"layerx_v3" and name == b"oracle_read":
            if oracle_type is not None:
                raise RuntimeError("duplicate committed oracle import")
            oracle_type = type_index
            import_offset = name_offset
    if cursor != end or oracle_type is None:
        raise RuntimeError("compiled guest does not exercise committed oracle import")
    wrong_import = bytearray(original)
    wrong_import[import_offset + 5] = ord("f")
    start, end = sections[1]
    count, cursor = leb(start)
    if oracle_type >= count:
        raise RuntimeError("committed oracle type index exceeds section")
    type_offset = None
    for index in range(count):
        if cursor >= end or original[cursor] != 0x60:
            raise RuntimeError("compiled guest requires canonical function types")
        parameters, cursor = leb(cursor + 1)
        if cursor + parameters > end:
            raise RuntimeError("compiled function parameters exceed section")
        if index == oracle_type:
            if parameters == 0 or original[cursor] not in (0x7f, 0x7e):
                raise RuntimeError("committed oracle import requires integer parameters")
            type_offset = cursor
        cursor += parameters
        results, cursor = leb(cursor)
        if cursor + results > end:
            raise RuntimeError("compiled function results exceed section")
        cursor += results
    if cursor != end or type_offset is None:
        raise RuntimeError("compiled guest has no committed oracle function type")
    wrong_signature = bytearray(original)
    wrong_signature[type_offset] = 0x7e if original[type_offset] == 0x7f else 0x7f
    floating = bytearray(original)
    floating[type_offset] = 0x7d
    for name, value in [("unknown-import", wrong_import), ("wrong-signature", wrong_signature), ("floating-point", floating)]:
        artifact = evidence / (name + ".wasm")
        artifact.write_bytes(value)
        yield name, artifact


def refuse(command, evidence, name, deadline):
    remaining = int(deadline - time.time())
    if remaining <= 0:
        raise TimeoutError("task cutoff reached")
    with (evidence / (name + ".log")).open("wb") as log:
        result = subprocess.run(["timeout", str(remaining), *map(str, command)], cwd=ROOT, stdout=log, stderr=log)
    (evidence / (name + ".exit")).write_text(str(result.returncode) + "\n")
    if result.returncode == 0 or result.returncode in (124, 127):
        raise RuntimeError("required refusal accepted, or gate unavailable")


def build(evidence, target, deadline):
    source = inventory()
    revision = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT,
        check=True, capture_output=True, text=True).stdout.strip()
    clang = compiler("clang", "PAXEER_X_PROGRAMS_CLANG")
    node = compiler("node", "PAXEER_X_PROGRAMS_NODE")
    cargo = compiler("cargo", "PAXEER_X_PROGRAMS_CARGO")
    rustc = compiler("rustc", "PAXEER_X_PROGRAMS_RUSTC")
    asc = Path(os.environ.get("PAXEER_X_PROGRAMS_ASC", str(ROOT / "programs/sdk/assemblyscript/node_modules/assemblyscript/bin/asc.js")))
    if not asc.is_file():
        raise FileNotFoundError("missing genuine AssemblyScript 0.27.31 compiler")
    target.mkdir(parents=True, exist_ok=True)
    manifest_parity()
    run([cargo, "build", "--offline", "--locked", "--manifest-path", ROOT / "programs/Cargo.toml",
        "-p", "layerx-program-sdk", "--target", "wasm32-unknown-unknown", "--target-dir", target / "rust-engine",
        "--config", 'target.wasm32-unknown-unknown.rustflags=["-C","target-feature=-reference-types","-C","link-arg=--compress-relocations"]'],
        evidence, "build-rust-sdk", deadline)
    rlibs = list((target / "rust-engine/wasm32-unknown-unknown/debug/deps").glob("liblayerx_program_sdk-*.rlib"))
    if len(rlibs) != 1:
        raise RuntimeError("exact freshly built SDK artifact required")
    run([rustc, "--edition=2021", "--target", "wasm32-unknown-unknown", "--crate-type", "cdylib",
        "-C", "opt-level=2", "-C", "strip=debuginfo", "-C", "panic=abort",
        "-C", "target-feature=-reference-types", "-C", "link-arg=--compress-relocations",
        "--extern", "layerx_program_sdk=" + str(rlibs[0]),
        "-L", "dependency=" + str(rlibs[0].parent), ROOT / "programs/sdk/rust/tests/fixtures/language_abi.rs",
        "-o", target / "rust.wasm"], evidence, "build-rust-guest", deadline)
    run([clang, "--target=wasm32-unknown-unknown", "-std=c17", "-Oz", "-ffreestanding", "-fno-builtin",
        "-nostdlib", "-I", ROOT / "programs/sdk/c/include", "-Wl,--no-entry", "-Wl,--export-memory",
        "-Wl,--gc-sections", *sorted((ROOT / "programs/sdk/c/src").glob("*.c")),
        ROOT / "programs/sdk/c/tests/language_abi.c", "-o", target / "c.wasm"], evidence, "build-c-guest", deadline)
    run([node, asc, ROOT / "programs/sdk/assemblyscript/tests/language_abi.ts", "--config",
        ROOT / "programs/sdk/assemblyscript/asconfig.json", "--outFile", target / "assemblyscript.wasm"],
        evidence, "build-assemblyscript-guest", deadline)
    run(["cc", "-std=c17", "-O2", ROOT / "programs/sdk/c/tools/determinism_lint.c", "-o", target / "c-lint"],
        evidence, "build-c-linter", deadline)
    run([cargo, "build", "--offline", "--locked", "--manifest-path", ROOT / "programs/Cargo.toml",
        "-p", "layerx-program-lint", "--bin", "layerx-program-lint", "--target-dir", target / "runtime"],
        evidence, "build-rust-linter", deadline)
    run([cargo, "test", "--offline", "--locked", "--manifest-path", ROOT / "programs/Cargo.toml",
        "-p", "layerx-program-lint", "--test", "repair_22_33_247", "--no-run", "--message-format=json",
        "--target-dir", target / "runtime"], evidence, "build-abi-linker", deadline)
    linker_paths = []
    for line in (evidence / "build-abi-linker.log").read_text().splitlines():
        if not line.startswith("{"):
            continue
        event = json.loads(line)
        if (event.get("reason") == "compiler-artifact"
                and event.get("target", {}).get("name") == "repair_22_33_247"
                and event.get("profile", {}).get("test") is True and event.get("executable")):
            linker_paths.append(event['executable'])
    if len(linker_paths) != 1:
        raise RuntimeError("unique canonical ABI guest/linker artifact required")
    fragment = evidence / "native.mk"
    fragment.write_text(""".PHONY: programs-language-abi-build
programs-language-abi-build: $(BUILD_DIR)/tests/programs_language_abi
$(BUILD_DIR)/tests/programs_language_abi: tests/programs/test_language_abi.c $(LIBRARY) $(PROGRAMS_RUNTIME_LIB) | programs-build
\t@mkdir -p $(@D)
\t$(CC) $(CPPFLAGS) $(CFLAGS) $< $(LIBRARY) $(PROGRAMS_RUNTIME_LIB) $(LIBRARY) $(EXTRA_LDFLAGS) $(PROGRAMS_NATIVE_LDLIBS) -lssl -lcrypto -pthread -ldl -lm -o $@
""")
    run(["flock", "/root/lx-cargo/native-build.lock", "make", "-j6", "-f", "Makefile", "-f", fragment,
        "BUILD_DIR=" + str(target / "native"), "PROGRAMS_TARGET_DIR=" + str(target / "runtime"),
        "PROGRAMS_RUNTIME_LIB=" + str(target / "runtime/debug/liblayerx_programs_sandbox.a"),
        "programs-language-abi-build"], evidence, "build-native", deadline)
    if inventory() != source:
        raise RuntimeError("candidate source changed during build")
    artifacts = {language: {"path": str(target / (language + ".wasm")), "sha256": sha(target / (language + ".wasm"))}
        for language in LANGUAGES}
    for name, path in [("abi_linker", Path(linker_paths[0])), ("native", target / "native/tests/programs_language_abi"), ("c_lint", target / "c-lint"),
        ("rust_lint", target / "runtime/debug/layerx-program-lint")]:
        artifacts[name] = {"path": str(path), "sha256": sha(path)}
    save(evidence / "artifacts.json", {"revision": revision, "source": source, "artifacts": artifacts,
        "abi_versions": list(admitted_abis()), "compiler": {"clang": str(clang), "rustc": str(rustc), "asc": str(asc)}})


def verify(evidence, deadline):
    manifest = json.loads((evidence / "artifacts.json").read_text())
    revision = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT,
        check=True, capture_output=True, text=True).stdout.strip()
    if manifest["revision"] != revision or manifest["source"] != inventory() or manifest["abi_versions"] != list(admitted_abis()):
        raise RuntimeError("candidate source identity mismatch")
    artifacts = manifest["artifacts"]
    for row in artifacts.values():
        if sha(row["path"]) != row["sha256"]:
            raise RuntimeError("candidate artifact missing or changed")
    expanded = write_manifest(evidence / "abi-manifest.tsv")
    shared = ROOT / "programs/sdk/abi-manifest.tsv"
    run([artifacts['abi_linker']['path'], '--exact', *LINKER_TESTS, '--test-threads=1'],
        evidence, 'abi-linker', deadline)
    linker_output((evidence / 'abi-linker.log').read_text())
    for language in LANGUAGES:
        for version in admitted_abis():
            selected = expanded if version >= 5 else shared
            commands = [("c", [artifacts["c_lint"]["path"], "--abi-version", str(version), "--abi-manifest", selected, artifacts[language]["path"]]),
                        ("assemblyscript", ["node", ROOT / "programs/sdk/assemblyscript/tools/determinism-lint.mjs", "--abi-version", str(version), "--abi-manifest", selected, artifacts[language]["path"]]),
                        ("rust", [artifacts["rust_lint"]["path"], "--abi-version", str(version), "--artifact", artifacts[language]["path"]])]
            for label, command in commands:
                name = f"lint-{label}-{language}-v{version}"
                if version >= 4:
                    run(command, evidence, name, deadline)
                else:
                    refuse(command, evidence, name, deadline)
        for version in (0, unallocated_abi(), 65535):
            for label, command in [("c", [artifacts["c_lint"]["path"], "--abi-version", str(version), "--abi-manifest", shared, artifacts[language]["path"]]),
                                   ("assemblyscript", ["node", ROOT / "programs/sdk/assemblyscript/tools/determinism-lint.mjs", "--abi-version", str(version), "--abi-manifest", shared, artifacts[language]["path"]]),
                                   ("rust", [artifacts["rust_lint"]["path"], "--abi-version", str(version), "--artifact", artifacts[language]["path"]])]:
                refuse(command, evidence, f"refuse-{label}-{language}-v{version}", deadline)
    for name, artifact in rejected_artifacts(artifacts["rust"]["path"], evidence):
        for label, command in [("c", [artifacts["c_lint"]["path"], "--abi-version", "4", "--abi-manifest", shared, artifact]),
            ("assemblyscript", ["node", ROOT / "programs/sdk/assemblyscript/tools/determinism-lint.mjs",
                "--abi-version", "4", "--abi-manifest", shared, artifact]),
            ("rust", [artifacts["rust_lint"]["path"], "--abi-version", "4", "--artifact", artifact])]:
            refuse(command, evidence, "refuse-" + label + "-" + name, deadline)
    with (evidence / "native-cases.json").open("wb") as output:
        run([artifacts["native"]["path"], *[artifacts[language]["path"] for language in LANGUAGES]],
            evidence, "native-cases", deadline, stdout=output)
    result = json.loads((evidence / "native-cases.json").read_text())
    if result.get("schema") != "layerx.program-language-abi.v1" or set(result.get("languages", {})) != set(LANGUAGES):
        raise RuntimeError("missing genuine cross-language corpus")
    reference = None
    for language in LANGUAGES:
        cases = result["languages"][language]["cases"]
        if set(cases) != set(CASES):
            raise RuntimeError("required behavior cases missing or skipped")
        for case in cases.values():
            if set(case) != set(FIELDS) | {"terminal_hex"} or type(case["result_code"]) is not int:
                raise RuntimeError("incomplete actual canonical outcome")
            for name in FIELDS[1:]:
                bytes.fromhex(case[name])
            if not bytes.fromhex(case["terminal_hex"]):
                raise RuntimeError("missing actual receipt-visible terminal evidence")
        semantic = {name: {field: cases[name][field] for field in FIELDS} for name in CASES}
        if reference is not None and semantic != reference:
            raise RuntimeError("cross-language canonical behavior differs")
        reference = semantic
    save(evidence / "qualification.json", {"revision": revision, "command": "timeout 30m python3 tools/qualification/paxeer-x/programs_language_abi.py",
        "exit_code": 0, "cases": result, "log_path": str(evidence / "native-cases.log")})


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--build", action="store_true")
    arguments = parser.parse_args()
    evidence = Path(os.environ.get("PAXEER_X_PROGRAMS_LANGUAGE_EVIDENCE", "/root/lx-ops/paxeer-x-integration-2026-10-03/qualification/task7.3"))
    evidence.mkdir(mode=0o700, parents=True, exist_ok=True)
    info = evidence.stat()
    if info.st_uid != os.geteuid() or info.st_mode & 0o077 or ROOT in evidence.resolve().parents:
        raise RuntimeError("protected evidence outside source checkout required")
    deadline = min(float(os.environ.get("PAXEER_X_TASK_DEADLINE_EPOCH", str(time.time() + 1800))), time.time() + 1800)
    target = Path(os.environ.get("PAXEER_X_PROGRAMS_LANGUAGE_TARGET", "/root/lx-target/task7.3"))
    try:
        if arguments.build:
            build(evidence, target, deadline)
        else:
            verify(evidence, deadline)
    except FileNotFoundError as error:
        print(str(error), file=sys.stderr)
        return 78
    except subprocess.CalledProcessError as error:
        return error.returncode
    except TimeoutError as error:
        print(str(error), file=sys.stderr)
        return 124
    except (RuntimeError, ValueError, KeyError) as error:
        print(str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
