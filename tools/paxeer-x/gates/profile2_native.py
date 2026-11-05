"""Profile-2 multiasset qualification over the retained merged-chain fixture."""

import argparse
import importlib.util
import os
import json
import re
from pathlib import Path
import subprocess
import time

from compiler_manifest import private_json, require, source_identity, write_private


ROOT = Path(__file__).resolve().parents[3]
SCHEMA = "paxeer-x.registry-multiasset-winddown.v2"
REQUIRED = {
    "two-assets-funded", "complete-exits", "missing-exit-refused",
    "current-proof-read", "proof-substitution-refused", "native-restart",
    "tombstone-history", "exit-conservation", "historical-replay",
    "wrong-asset-refused", "wrong-owner-refused",
}


def merged_fixture():
    path = ROOT / "tools/qualification/paxeer-x/programs_accounts_winddown.py"
    specification = importlib.util.spec_from_file_location("profile2_merged_winddown", path)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


def inventory(rows):
    require(isinstance(rows, list) and len(rows) == len(REQUIRED)
            and all(isinstance(row, dict) and isinstance(row.get("id"), str) for row in rows)
            and {row["id"] for row in rows} == REQUIRED,
            "complete exact profile-2 acceptance inventory required")
    return rows


def balances(reads, results):
    values = {}
    for row, result in zip(reads, results, strict=True):
        if row["method"] == "getValueAccount":
            key = tuple(row["arguments"])
            require(key not in values, "duplicate balance observation")
            raw = bytes.fromhex(result[2:])
            require(len(raw) == 64 and int.from_bytes(raw[:32], "big") < 2**128
                    and int.from_bytes(raw[32:], "big") in (0, 1), "canonical account balance required")
            values[key] = int.from_bytes(raw[:32], "big")
    require(len(values) >= 2, "both native asset account balances required")
    return values


def case(session, row, history):
    require(set(row) == {"id", "before", "steps", "after", "restart", "expected_deltas",
                         "committed_proofs"}, "closed merged profile-2 case required")
    name = row["id"]
    require(name in REQUIRED and type(row["restart"]) is bool, "declared profile-2 case required")
    prior_history = dict(history)
    before = session.observations(row["before"])
    require([(read["method"], read["arguments"]) for read in row["before"]]
            == [(read["method"], read["arguments"]) for read in row["after"]],
            "identical native accounts, routes and heads must be observed")
    require(any(read["method"] == "getStateHead" for read in row["before"]),
            "current committed state head observation required")
    methods, receipts, refusals = [], [], 0
    require(isinstance(row["steps"], list), "actual native Programs actions required")
    for step in row["steps"]:
        require(set(step) == {"method", "arguments", "raw_transaction", "refusal"},
                "closed native Programs step required")
        method = step["method"]
        arguments = session.node.json_arguments(method, step["arguments"])
        methods.append((method, arguments))
        if step["raw_transaction"] is not None:
            require(step["refusal"] is None, "signed transition must have committed success")
            receipts.append(session.node.program_send(step["raw_transaction"], method, arguments))
        else:
            require(isinstance(step["refusal"], dict)
                    and set(step["refusal"]) == {"code", "message", "data"}, "exact native refusal required")
            try:
                session.node.program_read(method, arguments)
            except session_module().CHAIN.RpcError as error:
                require(error.error == step["refusal"], "native refusal differs")
                refusals += 1
            else:
                require(False, "invalid native action admitted")
    if name == "two-assets-funded":
        registrations = [arguments for method, arguments in methods if method == "registerAccount"]
        require(len(registrations) >= 2 and all(arguments[1] == 2 for arguments in registrations)
                and len({arguments[2] for arguments in registrations}) >= 2,
                "owner opt-in to profile 2 for two distinct assets required")
        assets = {leg[1] for method, arguments in methods if method == "transfer" for leg in arguments[1]}
        require(all(leg[3] > 0 for method, arguments in methods
                    if method == "transfer" for leg in arguments[1])
                and {arguments[2] for arguments in registrations} <= assets,
                "both assets must be funded by native bank-backed Programs transfers")
    if name == "complete-exits":
        require(len({tuple(arguments) for method, arguments in methods
                     if method in ("exitValue", "exitValueBounded")}) >= 2,
                "both native account exits required")
    if name == "tombstone-history":
        require(any(method == "tombstone" for method, _ in methods), "actual tombstone required")
    after = session.observations(row["after"])
    prior, following = balances(row["before"], before), balances(row["after"], after)
    require(prior.keys() == following.keys(), "balance account substitution refused")
    expected = row["expected_deltas"]
    require(isinstance(expected, list) and len(expected) == len(prior)
            and all(type(delta) is int for delta in expected)
            and [following[key] - prior[key] for key in prior] == expected,
            "exact per-account native conservation required")
    if name == "exit-conservation":
        require(sum(expected) == 0 and receipts, "conserved actual exit movement required")
    proofs = row["committed_proofs"]
    require(isinstance(proofs, list) and proofs, "committed node proof receipts required")
    substitutions = 0
    for proof in proofs:
        require(set(proof) == {"transaction_hash", "expected", "substitution"}
                and type(proof["substitution"]) is bool, "closed committed proof observation required")
        actual = session.node.committed(proof["transaction_hash"])
        if proof["substitution"]:
            require(name == "proof-substitution-refused" and proof["expected"] != actual,
                    "proof substitution must differ from verified committed evidence")
            substitutions += 1
        else:
            require(actual == proof["expected"], "current committed proof differs")
            history[actual["transaction_hash"]] = actual
    if name.endswith("refused"):
        require((substitutions > 0 if name == "proof-substitution-refused" else refusals > 0)
                and not receipts and after == before and all(delta == 0 for delta in expected),
                "refusal must preserve all native state, routes and balances")
    if name == "native-restart":
        require(row["restart"], "native restart cannot be omitted")
    for receipt in receipts:
        history[receipt["transaction_hash"]] = receipt
    if name in {"native-restart", "tombstone-history", "historical-replay"}:
        require(prior_history and any(proof["transaction_hash"] in prior_history for proof in proofs),
                "earlier committed history must be revisited")
        if row["restart"]:
            session.restart()
            require(session.observations(row["after"]) == after, "reopened native state changed")
        for transaction, receipt in history.items():
            require(session.node.committed(transaction) == receipt, "recovered committed proof changed")
    elif row["restart"]:
        session.restart()
        require(session.observations(row["after"]) == after, "reopened native state changed")
    return {"id": name, "receipts": receipts, "before": before, "after": after,
            "refusals": refusals, "substitutions": substitutions}


_SESSION_MODULE = None


def session_module():
    global _SESSION_MODULE
    if _SESSION_MODULE is None:
        _SESSION_MODULE = merged_fixture()
    return _SESSION_MODULE


def build(evidence, manifest):
    source = source_identity(ROOT)
    with (evidence / "paxd-build.log").open("xb") as log:
        result = subprocess.run(["make", "paxd-build"], cwd=ROOT, stdin=subprocess.DEVNULL,
                                stdout=log, stderr=subprocess.STDOUT, timeout=1100)
    require(result.returncode == 0, "merged paxd build failed")
    artifact = session_module().artifact(ROOT / "build/paxd")
    command = ["cargo", "test", "--locked", "--manifest-path", "programs/Cargo.toml",
               "-p", "layerx-programs-registry", "--test", "winddown", "--no-run",
               "--message-format=json"]
    log = evidence / "registry-build.jsonl"
    with log.open("xb") as stream:
        result = subprocess.run(command, cwd=ROOT, stdin=subprocess.DEVNULL,
                                stdout=stream, stderr=subprocess.PIPE, timeout=1100)
    require(result.returncode == 0, "retained registry compilation failed")
    executables = set()
    for line in log.read_text().splitlines():
        event = json.loads(line)
        if (event.get("reason") == "compiler-artifact"
                and event.get("target", {}).get("name") == "winddown"
                and event.get("profile", {}).get("test") and event.get("executable")):
            executables.add(event["executable"])
    require(len(executables) == 1, "exact retained registry executable required")
    registry = session_module().artifact(next(iter(executables)))
    require(source_identity(ROOT) == source, "source changed during paxd build")
    write_private(manifest, {"schema": SCHEMA, "source": source, "required": sorted(REQUIRED),
                             "artifacts": {"paxd": artifact, "registry": registry}})


def verify(evidence, manifest, profile_path):
    module = session_module()
    value = private_json(manifest, ROOT)
    require(set(value) == {"schema", "source", "required", "artifacts"}
            and value["schema"] == SCHEMA and value["source"] == source_identity(ROOT)
            and value["required"] == sorted(REQUIRED) and set(value["artifacts"]) == {"paxd", "registry"},
            "current merged paxd artifact manifest required")
    artifact = value["artifacts"]["paxd"]
    for saved in value["artifacts"].values():
        require(module.artifact(saved["path"]) == saved, "compiled executable changed")
    profile = private_json(profile_path, ROOT)
    require(profile["source_revision"] == value["source"]["revision"], "profile source revision mismatch")
    rows = inventory(profile["cases"]["104.30.5"])
    fixture = module.PaxdFixture(profile, artifact["path"], evidence)
    history, results = {}, []
    try:
        fixture.start()
        session = module.WinddownSession(fixture.node, fixture.restart)
        for row in rows:
            result = case(session, row, history)
            write_private(evidence / (row["id"] + ".json"), result)
            results.append(result)
        require(source_identity(ROOT) == value["source"]
                and module.artifact(artifact["path"]) == artifact, "candidate changed during qualification")
        registry = value["artifacts"]["registry"]
        log = evidence / "retained-registry.log"
        with log.open("xb") as stream:
            result = subprocess.run([registry["path"], "--nocapture", "--test-threads=1"],
                                    cwd=ROOT, stdin=subprocess.DEVNULL, stdout=stream,
                                    stderr=subprocess.STDOUT, timeout=600)
        require(result.returncode == 0, "retained registry corpus failed")
        summary = re.findall(r"^test result: ok\. (\d+) passed; (\d+) failed; (\d+) ignored;",
                             log.read_text(), re.M)
        require(len(summary) == 1 and int(summary[0][0]) > 0 and summary[0][1:] == ("0", "0"),
                "retained registry corpus empty or skipped")
        for saved in value["artifacts"].values():
            require(module.artifact(saved["path"]) == saved, "compiled executable changed during corpus")
        require(source_identity(ROOT) == value["source"], "source changed during retained corpus")
        return results, int(summary[0][0])
    finally:
        fixture.stop()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--build-artifacts", action="store_true")
    arguments = parser.parse_args()
    os.umask(0o077)
    raw = os.environ.get("PAXEER_X_REGISTRY_WINDDOWN_EVIDENCE")
    require(raw, "private profile-2 evidence directory required")
    evidence = Path(raw)
    require(evidence.is_absolute() and evidence.resolve() == evidence
            and evidence != ROOT and ROOT not in evidence.parents,
            "canonical profile-2 evidence outside source required")
    info = evidence.stat()
    require(evidence.is_dir() and info.st_uid == os.geteuid()
            and not info.st_mode & 0o077, "owned private profile-2 evidence required")
    raw_manifest = os.environ.get("PAXEER_X_REGISTRY_WINDDOWN_ARTIFACTS")
    require(raw_manifest, "explicit merged paxd manifest required")
    manifest = Path(raw_manifest)
    require(manifest.parent == evidence and manifest.resolve() == manifest, "manifest must belong to private evidence")
    if arguments.build_artifacts:
        build(evidence, manifest)
    else:
        raw_profile = os.environ.get("PAXEER_X_PROFILE2_NATIVE_FIXTURE")
        require(raw_profile, "private paxd native funding and committed proof fixture required")
        run = evidence / ("run-" + str(time.time_ns()))
        run.mkdir(mode=0o700)
        results, registry_count = verify(run, manifest, raw_profile)
        write_private(run / "result.json", {"schema": SCHEMA, "cases": results, "tests": len(results) + registry_count, "skipped": 0})
        print("PAXEER_X_GATE tests=" + str(len(results) + registry_count) + " skipped=0")


if __name__ == "__main__":
    main()
