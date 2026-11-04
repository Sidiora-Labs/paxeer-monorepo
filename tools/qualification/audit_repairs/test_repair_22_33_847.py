"""Source-contract and filesystem proofs; no Rust/Go/hosted toolchain is needed."""

import ast
import importlib.util
import hashlib
import json
import math
import os
from pathlib import Path
import re
import subprocess
import sys
import tomllib

import pytest

ROOT = Path(__file__).resolve().parents[3]
GATES = ROOT / "tools/paxeer-x/gates"
SCRIPTS = ("0.4", "103.6.11", "103.6.13", "103.6.5", "104.17.3", "104.17.4")


def source(name):
    return (GATES / (name + ".sh")).read_text()


def python_source(name):
    return source(name).split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]


def declarations(text, names):
    tree = ast.parse(text)
    selected = [
        node for node in tree.body
        if (isinstance(node, (ast.FunctionDef, ast.ClassDef)) and node.name in names)
        or (isinstance(node, ast.Assign) and any(isinstance(t, ast.Name) and t.id in names for t in node.targets))
        or isinstance(node, (ast.Import, ast.ImportFrom))
    ]
    namespace = {}
    exec(compile(ast.Module(body=selected, type_ignores=[]), "<production declarations>", "exec"), namespace)
    return namespace


def function(text, name):
    return next(node for node in ast.parse(text).body if isinstance(node, ast.FunctionDef) and node.name == name)


def header(name):
    return re.search(r"^# paxeer-x-services: (.+)$", source(name), re.M)[1].split()


def catalogue():
    text = (ROOT / "tools/paxeer-x/candidate.py").read_text()
    ns = declarations(text, {"catalogue", "keys", "require", "Invalid"})
    return ns["catalogue"](ROOT / "spec/layerx-merge/spec.kvx")


def builder():
    spec = importlib.util.spec_from_file_location("hosted_build_847", GATES / "hosted_testnet_build.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_f039_retained_route_producers_use_declared_service_ids():
    expected = {
        "agent-mcp", "agentd", "bridge-relayer", "embedded-wallet-gateway", "explorer",
        "gas-station", "human-kms", "human-service", "interop-gateway", "interop-service",
        "launchpad-gateway", "launchpad-indexer", "mirror-publisher", "mirror-signer",
        "perps-data", "platform-agent-boundary", "platform-ai-artifacts", "platform-ai-evaluator",
        "platform-core-boundary", "platform-dashboard", "platform-faucet", "platform-gateway",
        "platform-identity", "platform-indexer", "platform-paxai-worker", "platform-price-feeder",
        "platform-program-registry", "platform-receipt-authority", "platform-testnet-control",
        "rollout-controller", "sequencer", "wallet-attestors", "x-websearch", "x402-seller-middleware",
    }
    assert set(header("0.4")) == expected
    assert len(header("0.4")) == len(expected)
    assert expected <= catalogue().keys()
    assert 'exec timeout 15m python3 tools/paxeer-x/tests/routes_test.py' in source("0.4")
    dispatcher = (ROOT / "tools/paxeer-x/verify-task.sh").read_text().split("<<'PY'\n")[1].rsplit("\nPY", 1)[0]
    binding = ast.unparse(function(dispatcher, "bind"))
    assert "if name not in services:" in binding
    assert "raise Refuse('mismatch'" in binding


def test_f010_composite_resolves_real_workspaces_and_preserves_corpus():
    text = python_source("103.6.11")
    build = ast.unparse(function(text, "build"))
    assert "'services/' + workspace + '/Cargo.toml'" in build
    for workspace, package in [("agent", "layerx-crypto"), ("agent", "layerx-proof"), ("human", "layerx-intents")]:
        manifest = tomllib.loads((ROOT / "services" / workspace / "Cargo.toml").read_text())
        assert "workspace" in manifest
        assert (ROOT / "services" / workspace / "crates" / package / "Cargo.toml").is_file()
    assert "('layerx-crypto', ['--test', 'verify'])" in build
    assert "('layerx-proof', [])" in build
    assert "('human', 'layerx-intents', ['--test', 'compile', '--test', 'differential'])" in build
    ns = declarations(text, {"NATIVE", "CONTRACTS", "COMMIT_CASES"})
    assert ns["NATIVE"] == ["test_state_commitment_transition", "test_genesis_manifest", "test_genesis_module_table", "test_genesis_builder_cli", "test_daemon_bootstrap_artifact"]
    assert ns["CONTRACTS"] == ["ContractManager"]
    assert ns["COMMIT_CASES"] == {"TestRealStateCreditAndClockBounds", "TestRealStateCreditRefusesMissingAuthority", "TestJSONRequestRefusesAmbiguousFields"}
    qualify = ast.unparse(function(text, "qualify"))
    assert "set(passed) == cases" in qualify
    assert "(len(cases), 0, 0)" in qualify
    assert "every scoped contract case must pass without skips" in qualify


# Run only the real deadline/persistence declarations in a fresh Python process.
# The build/qualify compiler and native suites are source-proven below, not simulated.
DEADLINE_DRIVER = r'''
import ast, json, pathlib, sys, time
text = pathlib.Path(sys.argv[1]).read_text().split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]
names = {"require", "private", "save", "load", "remaining", "restore_deadline", "DEADLINE", "LAST"}
tree = ast.parse(text)
body = [n for n in tree.body if isinstance(n, (ast.Import, ast.ImportFrom))
        or isinstance(n, ast.FunctionDef) and n.name in names
        or isinstance(n, ast.Assign) and any(isinstance(t, ast.Name) and t.id in names for t in n.targets)]
ns = {}
exec(compile(ast.Module(body=body, type_ignores=[]), "<gate deadline>", "exec"), ns)
p = pathlib.Path(sys.argv[2])
if sys.argv[3] == "build":
    ns["save"](p, {"deadline_epoch": ns["DEADLINE"]})
    print(json.dumps(ns["DEADLINE"]))
else:
    built = ns["load"](p)
    ns["restore_deadline"](built)
    print(json.dumps([ns["DEADLINE"], ns["remaining"](1800)]))
'''


def deadline_process(name, path, phase, explicit=None):
    environment = {key: value for key, value in os.environ.items() if key != "PAXEER_X_TASK_DEADLINE_EPOCH"}
    if explicit is not None:
        environment["PAXEER_X_TASK_DEADLINE_EPOCH"] = explicit
    return subprocess.run(
        [sys.executable, "-c", DEADLINE_DRIVER, str(GATES / (name + ".sh")), str(path), phase],
        env=environment, capture_output=True, text=True, timeout=10,
    )


@pytest.mark.parametrize("name", ["103.6.11", "103.6.13"])
def test_f011_f012_deadline_survives_two_processes_and_reopen(name, tmp_path):
    manifest = tmp_path / "build-manifest.json"
    built = deadline_process(name, manifest, "build")
    assert built.returncode == 0, built.stderr
    original = json.loads(built.stdout)
    assert math.isfinite(original)
    verified = deadline_process(name, manifest, "qualify")
    assert verified.returncode == 0, verified.stderr
    recovered, remaining = json.loads(verified.stdout)
    assert recovered == original
    assert 0 < remaining < 1800
    assert json.loads(manifest.read_text())["deadline_epoch"] == original
    text = python_source(name)
    assert "'deadline_epoch': DEADLINE" in ast.unparse(function(text, "build"))
    consumer = ast.unparse(function(text, "qualify"))
    assert consumer.index("load(directory / 'build-manifest.json')") < consumer.index("restore_deadline(built)") < consumer.index("checked(")


@pytest.mark.parametrize("name", ["103.6.11", "103.6.13"])
@pytest.mark.parametrize("deadline", [0, -1, True, "tomorrow", float("nan"), float("inf"), 1])
def test_f011_f012_refuse_invalid_or_exhausted_recorded_deadlines(name, deadline, tmp_path):
    manifest = tmp_path / "build-manifest.json"
    manifest.write_text(json.dumps({"deadline_epoch": deadline}))
    manifest.chmod(0o600)
    result = deadline_process(name, manifest, "qualify")
    assert result.returncode != 0
    assert "deadline" in result.stderr.lower() or "TimeoutExpired" in result.stderr


@pytest.mark.parametrize("name", ["103.6.11", "103.6.13"])
def test_f011_f012_refuse_extending_explicit_deadline(name, tmp_path):
    manifest = tmp_path / "build-manifest.json"
    built = deadline_process(name, manifest, "build")
    assert built.returncode == 0, built.stderr
    original = json.loads(built.stdout)
    same = deadline_process(name, manifest, "qualify", str(original))
    assert same.returncode == 0, same.stderr
    later = deadline_process(name, manifest, "qualify", str(original + 1800))
    assert later.returncode != 0
    assert "original task deadline changed" in later.stderr


def test_f040_identity_binding_preserves_full_tenant_restart_refusal_corpus():
    assert header("103.6.5") == ["platform-identity"]
    assert "platform-identity" in catalogue()
    text = source("103.6.5")
    required = re.search(r"required=\(\n(.*?)\n\)", text, re.S)[1].split()
    assert len(required) == len(set(required)) == 39
    assert hashlib.sha256("\n".join(required).encode()).hexdigest() == "9e9d7dfd5d33609f36b7dab720361032034d9732f354273d0e7c5dd7c9d25108"
    assert {
        "state_survives_a_restart", "principal_tenant_is_required_bounded_and_echoed",
        "a_second_tenant_cannot_claim_a_bound_subject", "sessions_belong_to_the_tenant_of_their_principal",
        "registry_resolver_authenticates_and_retains_authority_across_restart",
        "session_capacity_excludes_expired_after_restart", "store::tests::cross_tenant_state_on_disk_is_refused",
        "store::tests::failed_journal_write_requires_restart", "wrong_service_tokens_are_refused",
    } <= set(required)
    assert 'cargo test "${manifest[@]}"' in text
    assert 'cargo clippy "${manifest[@]}" -- -D warnings' in text
    assert 'grep -qxF "test $name ... ok"' in text
    assert '((tests >= 39))' in text and '((skipped == 0))' in text
    assert "--all-targets" in text


@pytest.mark.parametrize("name, expected", [
    ("104.17.3", {"platform-testnet-control", "platform-faucet", "platform-gateway", "platform-core-boundary", "platform-identity", "platform-receipt-authority", "human-service"}),
    ("104.17.4", {"platform-gateway"}),
])
def test_f041_f042_declared_producers_keep_private_evidence_dimension(name, expected):
    assert set(header(name)) == expected
    assert expected <= catalogue().keys()
    assert "# paxeer-x-evidence-dimension: private-profile (caller-owned private evidence)" in source(name)
    assert "private-profile" not in header(name)
    assert "results[0][1:] != ('0', '0')" in python_source(name)


def test_f013_consumer_checks_manifest_before_all_existing_corpora():
    text = python_source("104.17.3")
    ast.parse(text)
    assert text.index("built = builder.verify_artifacts") < text.index("for name in ('run-testnet.sh'")
    assert text.count("builder.verify_artifacts(root, Path(filename))") == 2
    assert "binary = built['binaries'][name]['path']" in text
    assert "os.environ.get(name, binary) != binary" in text
    assert "set(passed) != declared" in text
    assert "len(passed) != len(set(passed))" in text
    for corpus in ["'--list'", "'--nocapture'", "'--test-threads=1'", "faucet-quota.py", "hosted-smoke.sh", "record.get('skipped') != 0"]:
        assert corpus in text
    for name, (manifest, target, kind) in builder().TARGETS.items():
        parsed = tomllib.loads((ROOT / manifest).read_text())
        if kind == "lib":
            assert parsed["package"]["name"].replace("-", "_") == target
        else:
            assert target in {row["name"] for row in parsed["bin"]}
        assert name in text


def test_f013_producer_compiles_exact_targets_and_consumer_checks_closure():
    text = (GATES / "hosted_testnet_build.py").read_text()
    build = ast.unparse(function(text, "build"))
    assert "'cargo', 'test', '--locked', '--manifest-path', 'services/platform/Cargo.toml'" in build
    assert "'--no-run', '--message-format=json'" in build
    assert build.index("result.returncode == 0") < build.index("compiler_artifacts(root, rows)") < build.index("save(")
    assert "source_identity(root) == before" in build
    verify = ast.unparse(function(text, "verify_artifacts"))
    for guard in ["built['identity'] == source_identity(root)", "digest(log) == built['compiler_log']['sha256']", "selected == built['compiler_artifacts']", "checked_artifact(built['binaries'][name], row['executable'])"]:
        assert guard in verify
    identity = ast.unparse(function(text, "source_identity"))
    assert "'--cached', '--others', '--exclude-standard'" in identity
    assert "'revision'" in identity and "'tree'" in identity and "'hashes'" in identity
    with pytest.raises(RuntimeError, match="complete genuine compiler"):
        builder().compiler_artifacts(ROOT, [])


def test_f013_source_closure_detects_extensionless_fixture_and_dependency_changes(tmp_path):
    module = builder()
    names = ["Cargo.lock", "Cargo.toml", "build.rs", "fixture.bin", "Makefile", ".cargo/config.toml", "src/lib.rs"]
    for name in names:
        path = tmp_path / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(name)
    before = module.source_hashes(tmp_path, names)
    assert set(before) == set(names)
    for name in names:
        path = tmp_path / name
        path.write_text(name + " changed")
        assert module.source_hashes(tmp_path, names) != before
        path.write_text(name)
    (tmp_path / "untracked.rs").write_text("source")
    assert module.source_hashes(tmp_path, names + ["untracked.rs"]) != before
    (tmp_path / "Cargo.lock").unlink()
    with pytest.raises(RuntimeError, match="unavailable input"):
        module.source_hashes(tmp_path, names)


def test_f013_real_private_manifest_transport_and_digest_refusals(tmp_path):
    module = builder()
    # Committed Cargo input exercises transport, without impersonating a compiler.
    payload = tomllib.loads((ROOT / "services/platform/hosted/testnet/Cargo.toml").read_text())
    path = tmp_path / "manifest.json"
    module.save(path, payload)
    assert module.load(path) == payload
    assert path.stat().st_mode & 0o777 == 0o600
    with pytest.raises(FileExistsError):
        module.save(path, payload)
    path.chmod(0o644)
    with pytest.raises(RuntimeError, match="private regular"):
        module.load(path)
    path.chmod(0o600)
    linked = tmp_path / "linked.json"
    linked.symlink_to(path)
    with pytest.raises(RuntimeError, match="canonical"):
        module.load(linked)
    hardlink = tmp_path / "hardlink.json"
    os.link(path, hardlink)
    with pytest.raises(RuntimeError, match="canonical"):
        module.load(path)
    with pytest.raises(KeyError, match="schema"):
        # This is deliberately a Cargo manifest, never fabricated compiler evidence.
        hardlink.unlink()
        module.verify_artifacts(ROOT, path)
    executable = tmp_path / "retained-gate.sh"
    executable.write_bytes((GATES / "0.4.sh").read_bytes())
    executable.chmod(0o700)
    row = module.artifact(executable)
    assert row["sha256"] == module.digest(executable)
    assert row["path"] == str(executable)
    assert module.checked_artifact(row, executable) == executable
    executable.write_bytes(executable.read_bytes() + b"\n# changed source\n")
    with pytest.raises(RuntimeError, match="digest changed"):
        module.checked_artifact(row, executable)
    executable.chmod(0o600)
    with pytest.raises(RuntimeError, match="executable compiler artifact"):
        module.artifact(executable)
    with pytest.raises(RuntimeError, match="duplicate manifest"):
        module.duplicate_free([("revision", "one"), ("revision", "two")])


def test_record_maps_every_finding_and_out_of_scope_wiring():
    record = tomllib.loads((ROOT / "tools/qualification/audit_repairs/repair_22_33_847.kvx").read_text())
    assert set(record["finding"]) == {"f039", "f010", "f011", "f012", "f040", "f041", "f013", "f042"}
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef)}
    for finding in record["finding"].values():
        assert finding["tests"] and set(finding["tests"]) <= declared
    assert record["task"]["22"]["33"]["847"]["baseline"].startswith("Not run")
    assert len(record["wiring"]) >= 2
