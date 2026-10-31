#!/usr/bin/env python3
"""Qualify deployment validation through native dispatch and the program runtime."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[3]
COMMAND = ["timeout", "30m", "python3",
           "tools/qualification/paxeer-x/programs_all_form_validation.py"]
SCHEMA = "paxeer-x.programs-all-form-artifacts.v1"


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def digest(path):
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def private_directory(path, create=False):
    path = Path(path).absolute()
    require(not path.is_symlink(), "private directory must not be a symlink")
    if create:
        path.mkdir(mode=0o700, parents=True, exist_ok=True)
    path = path.resolve(strict=True)
    info = path.stat()
    require(path != ROOT and ROOT not in path.parents,
            "qualification evidence must remain outside the repository")
    require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid()
            and not info.st_mode & 0o077,
            "qualification directory must be private and owned by the caller")
    return path


def write_private(path, value):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, "w") as stream:
        json.dump(value, stream, indent=2, sort_keys=True)
        stream.write("\n")
        stream.flush()
        os.fsync(stream.fileno())


def load_private(path):
    private_directory(path.parent)
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(descriptor) as stream:
        info = os.fstat(stream.fileno())
        require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1
                and info.st_uid == os.geteuid() and not info.st_mode & 0o077,
                "artifact manifest must be a private caller-owned regular file")
        return json.load(stream)


def capture(command):
    return subprocess.check_output(command, cwd=ROOT, text=True, timeout=30).strip()


def identity():
    revision = capture(["git", "rev-parse", "HEAD^{commit}"])
    require(re.fullmatch(r"[0-9a-f]{40}", revision), "invalid candidate revision")
    require(not capture(["git", "status", "--porcelain=v1", "--untracked-files=normal"]),
            "candidate source must be clean before building or qualifying")
    return {"revision": revision, "tree": capture(["git", "rev-parse", "HEAD^{tree}"])}


def artifact(path, executable=False):
    path = Path(path).absolute()
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and not path.is_symlink() and info.st_size > 0,
            "missing or empty regular candidate artifact")
    require(not executable or os.access(path, os.X_OK), "candidate artifact is not executable")
    return {"path": str(path), "sha256": digest(path), "bytes": info.st_size}


def logged(command, log, environment, deadline, cwd=ROOT):
    remaining = deadline - time.monotonic()
    require(remaining > 0, "qualification deadline expired")
    descriptor = os.open(log, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, "wb") as stream:
        stream.write(("COMMAND " + json.dumps(command) + "\n").encode())
        stream.flush()
        process = subprocess.Popen(command, cwd=cwd, env=environment,
                                   stdin=subprocess.DEVNULL, stdout=stream,
                                   stderr=subprocess.STDOUT, start_new_session=True)
        try:
            status = process.wait(timeout=remaining)
        finally:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait(timeout=5)
        stream.write(f"\nEXIT {status}\n".encode())
        stream.flush()
        os.fsync(stream.fileno())
    return status


def build(output):
    directory = private_directory(output, create=True)
    require(not any(directory.iterdir()), "build output must be a fresh empty private directory")
    source = identity()
    native_dir = directory / "native"
    cargo_dir = directory / "cargo"
    library = native_dir / "liblayerx.a"
    header = native_dir / "generated/lxp_checkpoint_settlement.h"
    sandbox = cargo_dir / "debug/liblayerx_programs_sandbox.a"
    binary = directory / "programs_lifecycle"
    jobs = int(os.environ.get("PAXEER_X_PROGRAM_VALIDATION_BUILD_JOBS", "4"))
    require(1 <= jobs <= 16, "build jobs must be between 1 and 16")
    environment = dict(os.environ, CARGO_TARGET_DIR=str(cargo_dir),
                       CARGO_BUILD_JOBS=str(jobs), CARGO_PROFILE_DEV_DEBUG="0",
                       CARGO_INCREMENTAL="0", PYTHONDONTWRITEBYTECODE="1")
    commands = [
        ["make", "--no-print-directory", "-j" + str(jobs),
         "BUILD_DIR=" + str(native_dir), "LXP_REVISION=" + source["revision"],
         str(library), str(header)],
        ["cargo", "build", "--locked", "--manifest-path", str(ROOT / "programs/Cargo.toml"),
         "-p", "layerx-programs-sandbox", "--features", "host-ffi"],
        ["cc", "-Iinclude", "-I" + str(header.parent), "-std=c17", "-pedantic", "-Werror",
         "-Wall", "-Wextra", "-Wconversion", "-Wshadow", "-Wvla", "-fno-strict-aliasing",
         "-ffp-contract=off", "-O2", "tests/programs/test_lifecycle.c",
         "-Wl,--start-group", str(library), str(sandbox), "-Wl,--end-group",
         "-lcrypto", "-lsqlite3", "-pthread", "-ldl", "-lm", "-o", str(binary)],
    ]
    commands.append(["cargo", "test", "--locked", "--manifest-path", str(ROOT / "programs/Cargo.toml"),
                     "-p", "layerx-programs-runtime", "--lib", "--no-run", "--message-format=json",
                     "deployment_admission_"])
    record = {"schema": SCHEMA, "source": source, "commands": commands,
              "features": {"sandbox": ["host-ffi"]}, "logs": [], "exit_code": None}
    write_private(directory / "build-inputs.json", record)
    deadline = time.monotonic() + 1740
    try:
        for index, command in enumerate(commands):
            log = directory / f"build-{index + 1}.log"
            cwd = ROOT / "programs" if index in (1, 3) else ROOT
            status = logged(command, log, environment, deadline, cwd=cwd)
            record["logs"].append({"command": command, "cwd": str(cwd),
                                   "exit_code": status, "log": artifact(log)})
            require(status == 0, "focused artifact build failed; inspect private build log")
        require(identity() == source, "candidate source changed during artifact production")
        executables = []
        build_finished = False
        for line in (directory / "build-4.log").read_text().splitlines():
            if not line.startswith("{"):
                continue
            event = json.loads(line)
            if event.get("reason") == "build-finished":
                build_finished = event.get("success") is True
            if (event.get("reason") == "compiler-artifact"
                    and event.get("target", {}).get("name") == "layerx_programs_runtime"
                    and event.get("profile", {}).get("test") is True and event.get("executable")):
                require(event.get("features") == [], "unexpected runtime test features")
                executables.append(event["executable"])
        require(build_finished and len(executables) == 1, "missing unique compiled runtime test artifact")
        runtime_tests = directory / "runtime_tests"
        shutil.copyfile(executables[0], runtime_tests)
        runtime_tests.chmod(0o700)
        record["artifacts"] = {"native": artifact(binary, executable=True),
                               "runtime-tests": artifact(runtime_tests, executable=True),
                               "native-library": artifact(library),
                               "sandbox-staticlib": artifact(sandbox),
                               "generated-header": artifact(header)}
        record["exit_code"] = 0
        write_private(directory / "manifest.json", record)
        print("PROGRAM_VALIDATION_NATIVE_MANIFEST " + str(directory / "manifest.json"))
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError):
        record["exit_code"] = 1
        write_private(directory / "build-failure.json", record)
        raise


def required_native_cases():
    required = {}
    for phase in ("initial", "snapshot_replay"):
        for form in ("legacy", "framed"):
            for abi in range(1, 5):
                for name in ("malformed_body", "float_type", "ambient_import", "valid"):
                    required[(name, form, abi, phase)] = 0 if name == "valid" else -3
            required[("unsupported_abi", form, 5, phase)] = -101
            for name in ("module_bytes", "function_count", "local_stack", "operand_stack", "call_depth"):
                required[(name, form, 1, phase)] = -3
        required[("invalid_capability", "framed", 1, phase)] = -3
    return required


def case_identity(case):
    return case["name"], case["form"], case["abi"], case["phase"]


def native_cases(output):
    cases = [json.loads(line[len("ALL_FORM_CASE "):]) for line in output.splitlines()
             if line.startswith("ALL_FORM_CASE ")]
    summaries = re.findall(r"^ALL_FORM_VALIDATION cases=(\d+) skipped=0$", output, re.M)
    required = required_native_cases()
    require(summaries == [str(len(required))] and len(cases) == len(required),
            "native case accounting is absent, inconsistent or skipped")
    identities = [case_identity(case) for case in cases]
    require(len(identities) == len(set(identities)) and set(identities) == set(required),
            "native matrix is incomplete or duplicated")
    for case in cases:
        expected = required[case_identity(case)]
        require(type(case["result"]) is int and type(case["expected"]) is int
                and case["result"] == case["expected"] == expected,
                "native result differs from canonical expected result")
        if expected != 0:
            require(case["lifecycle_present"] is False and case["new_artifacts"] == 0
                    and case["staged"] == 0 and case["effects"] == 0
                    and case["state_unchanged"] is True,
                    "refused deployment retained new executable artifact or lifecycle state")
        else:
            require(case["artifact_present"] is True and case["lifecycle_present"] is True,
                    "valid deployment did not persist its executable artifact and recorded ABI")
    return cases


def payload_name(case):
    return f"{case['phase']}.{case['name']}.{case['form']}.abi{case['abi']}.bin"


def qualify(manifest):
    base = os.environ.get("PAXEER_X_EVIDENCE_DIR")
    if base:
        base = private_directory(base)
    directory = Path(tempfile.mkdtemp(prefix="programs-all-form-", dir=base or "/var/tmp"))
    private_directory(directory)
    result = {"task": "6.5", "command": COMMAND, "cases": [], "tests": 0,
              "skipped": 0, "exit_code": 1, "status": "incomplete",
              "qualification_complete": False, "boundary": "registered native kernel dispatch",
              "logs": [], "evidence_directory": str(directory)}
    try:
        result["candidate_revision"] = capture(["git", "rev-parse", "HEAD^{commit}"])
        result["source"] = identity()
        require(manifest, "PAXEER_X_PROGRAM_VALIDATION_MANIFEST is required; produce candidate artifacts explicitly")
        path = Path(manifest).absolute()
        saved = load_private(path)
        require(saved.get("schema") == SCHEMA and saved.get("source") == result["source"],
                "artifact manifest does not bind the exact clean candidate")
        require(saved.get("exit_code") == 0 and saved.get("features") == {"sandbox": ["host-ffi"]},
                "successful source-bound sandbox artifact production is required")
        require(set(saved["artifacts"]) == {"native", "runtime-tests", "native-library", "sandbox-staticlib", "generated-header"},
                "candidate artifact inventory is incomplete")
        require(len(saved["logs"]) == 4 and len(saved["commands"]) == 4,
                "candidate build command evidence is incomplete")
        for index, row in enumerate(saved["logs"]):
            require(row["exit_code"] == 0 and row["command"] == saved["commands"][index]
                    and artifact(row["log"]["path"]) == row["log"],
                    "candidate build log evidence does not match the producer")
        for name, row in saved["artifacts"].items():
            require(artifact(row["path"], executable=name in ("native", "runtime-tests")) == row,
                    "candidate artifact changed since production")
        result["manifest"] = artifact(path)
        staged = directory / "programs_lifecycle"
        shutil.copyfile(saved["artifacts"]["native"]["path"], staged)
        staged.chmod(0o700)
        require(digest(staged) == saved["artifacts"]["native"]["sha256"], "staged candidate differs")
        state = directory / "state"
        state.mkdir(mode=0o700)
        command = [str(staged), "--all-form-validation"]
        environment = {key: value for key, value in os.environ.items()
                       if not key.startswith(("LAYERX_", "PAXEER_X_"))}
        environment.update(TMPDIR=str(state), PYTHONDONTWRITEBYTECODE="1",
                           PAXEER_X_PROGRAM_VALIDATION_INPUTS=str(state))
        log = directory / "native.log"
        status = logged(command, log, environment, time.monotonic() + 180, cwd=state)
        result["logs"].append({"command": command, "exit_code": status, "log": artifact(log)})
        require(status == 0, "native admission cases failed; inspect private native.log")
        result["cases"] = native_cases(log.read_text())
        result["tests"] = len(result["cases"])
        require(identity() == result["source"], "candidate source changed during native qualification")
        require(digest(staged) == saved["artifacts"]["native"]["sha256"], "candidate executable changed during run")
        result["native_checks_passed"] = True
        expected_files = {payload_name(case) for case in result["cases"]}
        require({file.name for file in state.glob("*.bin")} == expected_files,
                "native payload producer omitted or duplicated a case")
        result["native_inputs"] = [artifact(state / name) for name in sorted(expected_files)]
        runtime_command = [saved["artifacts"]["runtime-tests"]["path"], "deployment_admission_", "--test-threads=1"]
        runtime_log = directory / "runtime-tests.log"
        runtime_status = logged(runtime_command, runtime_log, environment, time.monotonic() + 90)
        result["logs"].append({"command": runtime_command, "exit_code": runtime_status, "log": artifact(runtime_log)})
        runtime_output = runtime_log.read_text()
        runtime_passed = re.findall(r"^test ([A-Za-z0-9_:]+) \.\.\. ok$", runtime_output, re.M)
        expected_runtime = {"engine::deployment_admission_tests::deployment_admission_calls_recorded_abis",
                            "engine::deployment_admission_tests::deployment_admission_bounds_and_recursive_runtime_guard"}
        require(runtime_status == 0 and len(runtime_passed) == 2 and set(runtime_passed) == expected_runtime
                and re.findall(r"test result: ok\. (\d+) passed; (\d+) failed; (\d+) ignored;", runtime_output) == [("2", "0", "0")],
                "focused runtime execution or recursion guard cases are missing, failed or skipped")
        result["runtime_cases"] = runtime_passed
        result["tests"] += len(runtime_passed)
        require(identity() == result["source"], "source changed during runtime qualification")
        for name, row in saved["artifacts"].items():
            require(artifact(row["path"], executable=name in ("native", "runtime-tests")) == row,
                    "candidate artifact changed during qualification")
        result["qualification_complete"] = True
        result["status"] = "passed"
        result["exit_code"] = 0
    except (OSError, ValueError, KeyError, TypeError, RuntimeError, subprocess.SubprocessError) as error:
        result["failure"] = str(error)
    write_private(directory / "result.json", result)
    print(f"PAXEER_X_GATE tests={result['tests']} skipped=0 complete={str(result['qualification_complete']).lower()}")
    print("Evidence: " + str(directory / "result.json"))
    return result["exit_code"]


def main():
    os.umask(0o077)
    def interrupted(_signum, _frame):
        raise RuntimeError("qualification interrupted before completion")
    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", default=os.environ.get("PAXEER_X_PROGRAM_VALIDATION_MANIFEST"))
    parser.add_argument("--build-output", help="explicitly build the focused native candidate into a new private directory")
    arguments = parser.parse_args()
    try:
        if arguments.build_output:
            build(arguments.build_output)
            return 0
        return qualify(arguments.manifest)
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError):
        print("program validation refused before evidence initialization or during explicit build", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
