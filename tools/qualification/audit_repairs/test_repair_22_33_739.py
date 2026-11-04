"""Offline proof of the repaired YAML, native wiring and evidence predicates.

No paxd, Docker, native build or live endpoint runs here. The only subprocess is
bash -n, explicitly required for the embedded shell commands by this task.
"""

import ast
import base64
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import shlex
import subprocess
import tomllib

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
CHAIN = ROOT / "tests/integration/chain_operation"


def load_module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


PROBE = load_module("snapshot_probe", CHAIN / "snapshot_probe.py")
RUNNER = load_module("chain_runner", ROOT / "tests/integration/scripts/runner.py")


def source(path):
    return (ROOT / path).read_text()


@pytest.mark.parametrize("filename,receiver", [
    ("snapshot_operation.yaml", "pax-node-0"),
    ("statesync_operation.yaml", "pax-rpc-node"),
])
def test_yaml_drives_donor_transport_clean_receiver_and_comparison(filename, receiver):
    cases = yaml.safe_load((CHAIN / filename).read_text())
    assert len(cases) == 1
    case = cases[0]
    assert len(case["inputs"]) == 3
    steps = case["inputs"]
    assert [s["env"] for s in steps] == ["BOOTSTRAP", "IMPORT", "RESTORED"]
    assert [s["node"] for s in steps] == ["pax-node-0", receiver, "pax-node-0"]
    commands = []
    for step in steps:
        script = step["cmd"]
        parsed = subprocess.run(["bash", "-n"], input=script, text=True, capture_output=True)
        assert parsed.returncode == 0, parsed.stderr
        assert script.startswith("set -euo pipefail;")
        tokens = shlex.split(script.split(";", 1)[1])
        assert tokens[:2] == ["python3", "tests/integration/chain_operation/snapshot_probe.py"]
        commands.append(tokens[2:])
        argv = RUNNER.TestRunner._command_argv(script, True, step["node"], {"BOOTSTRAP": "{}"})
        assert argv[:6] == ["docker", "exec", "-e", "BOOTSTRAP={}", "--", step["node"]]
        assert argv[-1].endswith(script)
    assert commands == [
        ["prepare", "--snapshot-dir", "./build/generated/node_0/snapshots"],
        ["restore", "--bootstrap", "$BOOTSTRAP"],
        ["compare", "--bootstrap", "$BOOTSTRAP", "--evidence", "$IMPORT",
         "--snapshot-dir", "./build/generated/node_0/snapshots"],
    ]
    assert case["verifiers"] == [{"type": "eval", "expr": 'RESTORED == "verified"'}]
    runner = RUNNER.TestRunner()
    assert runner.verify_result({"RESTORED": "verified"}, case["verifiers"][0])
    assert not runner.verify_result({"RESTORED": "100"}, case["verifiers"][0])
    assert not runner.verify_result({"RESTORED": "true"}, case["verifiers"][0])


def test_snapshot_directory_only_requires_nonempty_contiguous_content(tmp_path):
    snapshot = tmp_path / "100" / "1"
    snapshot.mkdir(parents=True)
    with pytest.raises(ValueError, match="empty or noncontiguous"):
        PROBE.snapshot_metadata(tmp_path, 100, 1)
    (snapshot / "0").write_bytes(b"")
    with pytest.raises(ValueError, match="empty snapshot chunk"):
        PROBE.snapshot_metadata(tmp_path, 100, 1)
    (snapshot / "0").write_bytes(b"first chunk")
    (snapshot / "2").write_bytes(b"last chunk")
    with pytest.raises(ValueError, match="noncontiguous"):
        PROBE.snapshot_metadata(tmp_path, 100, 1)
    (snapshot / "2").rename(snapshot / "1")
    metadata = PROBE.snapshot_metadata(tmp_path, 100, 1)
    assert metadata == {"height": 100, "format": 1, "chunks": 2,
                        "hash": hashlib.sha256(b"first chunklast chunk").hexdigest()}
    (snapshot / "1").write_bytes(b"corruption")
    assert PROBE.snapshot_metadata(tmp_path, 100, 1)["hash"] != metadata["hash"]


def test_statesync_receiver_configuration_is_clean_isolated_and_native():
    bootstrap = {"peer": "abc@192.168.10.10:26656", "trust_height": 100,
                 "trust_hash": "ab" * 32}
    config, app = PROBE.receiver_config(
        source("docker/localnode/config/config.toml"),
        source("docker/localnode/config/app.toml"), bootstrap, 31001, 31002, Path("/tmp/probe"),
    )
    parsed, application = tomllib.loads(config), tomllib.loads(app)
    assert parsed["mode"] == "full"
    assert parsed["statesync"] == dict(
        tomllib.loads(source("docker/localnode/config/config.toml"))["statesync"],
        **{"enable": True, "use-local-snapshot": False, "rpc-servers":
           "http://192.168.10.10:26657,http://192.168.10.11:26657",
           "trust-height": 100, "trust-hash": "ab" * 32, "discovery-time": "5s"},
    )
    assert parsed["p2p"]["persistent-peers"] == bootstrap["peer"]
    assert parsed["p2p"]["pex"] is False
    assert parsed["p2p"]["allow-duplicate-ip"] is True
    assert parsed["rpc"]["laddr"] == "tcp://127.0.0.1:31001"
    assert parsed["p2p"]["laddr"] == "tcp://127.0.0.1:31002"
    assert parsed["rpc"]["pprof-laddr"] == ""
    assert parsed["instrumentation"]["prometheus"] is False
    for section in ("api", "grpc", "grpc-web", "rosetta"):
        assert application[section]["enable"] is False
    assert application["evm"]["http_enabled"] is False
    assert application["evm"]["ws_enabled"] is False
    assert application["state-store"]["ss-enable"] is True
    assert application["state-sync"]["snapshot-directory"] == "/tmp/probe/snapshots"
    assert application["state-sync"]["snapshot-interval"] == 0
    # Preserve the actual storage mode, not a substitute implementation.
    assert application["state-commit"] == tomllib.loads(
        source("docker/localnode/config/app.toml"))["state-commit"]
    for key in ("enable", "rpc-servers", "trust-height", "trust-hash", "use-local-snapshot"):
        assert 'mapstructure:"' + key + '"' in source("consensus/config/config.go")
    body = ast.get_source_segment((CHAIN / "snapshot_probe.py").read_text(),
                                 next(n for n in ast.parse((CHAIN / "snapshot_probe.py").read_text()).body
                                      if isinstance(n, ast.FunctionDef) and n.name == "restore"))
    assert body.index("TemporaryDirectory") < body.index('"init"') < body.index("receiver_config")
    assert '"priv_validator_state.json"' in body
    assert '"fresh receiver already contains application or consensus state"' in body
    assert body.index('"paxd", "start"') < body.index("import_outcome(log)") < body.index("bank_state(")
    assert "finally:" in body and "process.terminate()" in body and "process.kill()" in body
    assert 'PAX_LOG_FORMAT="json"' in body and 'PAX_LOG_OUTPUT="stdout"' in body
    assert PROBE.BUDGET_SECONDS == 90
    with pytest.raises(TimeoutError):
        PROBE.remaining(0)


def test_native_producer_transport_consumer_source_contract():
    store = source("sdk/snapshots/store.go")
    assert "snapshotHasher := sha256.New()" in store
    assert "io.MultiWriter(file, chunkHasher, snapshotHasher)" in store
    assert "snapshot.Hash = snapshotHasher.Sum(nil)" in store
    assert "snapshot.Chunks = index" in store
    assert "return snapshot, s.saveSnapshot(snapshot)" in store
    assert "strconv.FormatUint(uint64(chunk), 10)" in store
    syncer = source("consensus/internal/statesync/syncer.go")
    for event in ("discovered and added new snapshot", "Snapshot accepted, restoring",
                  "Applied snapshot chunk to ABCI app", "Verified ABCI app", "Snapshot restored"):
        assert 'logger.Info("' + event + '"' in syncer
    assert syncer.index("s.applyChunks(ctx, chunks, fetchStartTime)") < syncer.index(
        "s.verifyApp(ctx, snapshot, state.Version.Consensus.App)") < syncer.index('logger.Info("Snapshot restored"')
    assert "!bytes.Equal(snapshot.trustedAppHash, resp.LastBlockAppHash)" in syncer
    assert "uint64(resp.LastBlockHeight) != snapshot.Height" in syncer
    assert 'paxlog.NewLogger("tendermint", "internal", "statesync")' in source(
        "consensus/internal/statesync/stateprovider.go")
    assert '.With("logger", name)' in source("core/deps/log/seilog.go")
    probe = (CHAIN / "snapshot_probe.py").read_text()
    assert '"block?height=" + str(positive(height) + 1)' in probe
    assert '"--height", str(height), "--node", url' in probe
    assert '"balances", address, "--denom", "uhpx"' in probe
    assert 'validate_comparison(bootstrap, evidence, metadata, state)' in probe
    cli = source("sdk/x/bank/client/cli/query.go")
    assert 'Use:   "balances [address]"' in cli and 'flags.AddQueryFlagsToCmd(cmd)' in cli


def parser_events():
    # Predicate inputs use the checked native log schema, not a simulated node.
    encoded = base64.b64encode(bytes(range(32))).decode()
    identity = {"height": 100, "format": 1, "hash": encoded}
    return [
        dict(identity, msg="discovered and added new snapshot", peer="donor"),
        dict(identity, msg="Snapshot accepted, restoring"),
        {"height": 100, "format": 1, "chunk": 0, "total": 2,
         "msg": "Applied snapshot chunk to ABCI app"},
        {"height": 100, "format": 1, "chunk": 1, "total": 2,
         "msg": "Applied snapshot chunk to ABCI app"},
        {"height": 100, "appHash": encoded, "msg": "Verified ABCI app"},
        dict(identity, msg="Snapshot restored"),
    ]


def log_text(events):
    return "\n".join(json.dumps(dict(e, logger="tendermint/internal/statesync")) for e in events)


def test_statesync_height_only_cannot_substitute_for_completed_import():
    events = parser_events()
    outcome = PROBE.import_outcome(log_text(events))
    assert outcome == {"height": 100, "format": 1, "chunks": 2,
                       "hash": bytes(range(32)).hex(), "app_hash": bytes(range(32)).hex(),
                       "peer": "donor"}
    for index in range(len(events)):
        with pytest.raises(ValueError):
            PROBE.import_outcome(log_text(events[:index] + events[index + 1:]))
    with pytest.raises(ValueError, match="completed import"):
        PROBE.import_outcome('{"latest_block_height": "100"}')
    with pytest.raises(ValueError):
        PROBE.import_outcome(log_text(list(reversed(events))))
    for field, value in (("height", 101), ("format", 2), ("hash", base64.b64encode(b"x" * 32).decode())):
        altered = copy.deepcopy(events)
        altered[1][field] = value
        with pytest.raises(ValueError):
            PROBE.import_outcome(log_text(altered))


def test_same_height_known_state_and_digest_reject_divergence():
    metadata = {"height": 100, "format": 1, "chunks": 2, "hash": "ab" * 32}
    bootstrap = {"chain_id": "pax", "address": "known", "peer": "donor@192.168.10.10:26656"}
    donor = {"height": 100, "chain_id": "pax", "address": "known", "amount": "10",
             "denom": "uhpx", "header_app_hash": "cd" * 32}
    evidence = dict(metadata, **{k: v for k, v in donor.items() if k != "height"},
                    app_hash="cd" * 32, fresh_home=True, peer="donor")
    assert PROBE.validate_comparison(bootstrap, evidence, metadata, donor) == "verified"
    for key, wrong in (("height", 101), ("format", 2), ("chunks", 1), ("hash", "ef" * 32),
                       ("chain_id", "other"), ("address", "other"), ("amount", "11"),
                       ("denom", "other"), ("app_hash", "ef" * 32),
                       ("header_app_hash", "ef" * 32), ("fresh_home", False), ("peer", "other")):
        with pytest.raises(ValueError):
            PROBE.validate_comparison(bootstrap, dict(evidence, **{key: wrong}), metadata, donor)
    for key, wrong in (("height", 101), ("amount", "11"), ("header_app_hash", "ef" * 32)):
        with pytest.raises(ValueError):
            PROBE.validate_comparison(bootstrap, evidence, metadata, dict(donor, **{key: wrong}))


def test_record_maps_both_findings_and_limits_the_evidence_claim():
    record = tomllib.loads(Path(__file__).with_name("repair_22_33_739.kvx").read_text())
    declared = {n.name for n in ast.parse(Path(__file__).read_text()).body
                if isinstance(n, ast.FunctionDef) and n.name.startswith("test_")}
    assert set(record["finding"]) == {"snapshot-directory-only", "statesync-height-only"}
    for finding in record["finding"].values():
        assert finding["audit"] == "22.30.172"
        assert finding["tests"] and set(finding["tests"]) <= declared
        assert finding["wiring_requests"] == []
    task = record["task"]["22"]["33"]["739"]
    assert "not run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_739.py"
