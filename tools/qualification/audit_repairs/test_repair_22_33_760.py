"""Offline source-contract qualification, not execution of native paxd upgrades.

Exercise the real rehearsal's validators and inspect its native producer/RPC/
consumer path. No node double, shell, compiler, credential or live RPC is used.
"""

import ast
import copy
import hashlib
import importlib.util
from pathlib import Path
import shlex
import tomllib

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
MODULE = ROOT / "tests/integration/upgrade_module"
SCRIPT = MODULE / "merge_upgrade_proof.py"
SOURCE = SCRIPT.read_text()
TREE = ast.parse(SOURCE)
SPEC = importlib.util.spec_from_file_location("repair_upgrade_proof", SCRIPT)
PROOF = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROOF)


def source(path):
    return (ROOT / path).read_text()


def function(name):
    matches = [node for node in ast.walk(TREE)
               if isinstance(node, ast.FunctionDef) and node.name == name]
    assert len(matches) == 1
    return ast.get_source_segment(SOURCE, matches[0])


def ordered(text, *facts):
    cursor = 0
    for fact in facts:
        cursor = text.index(fact, cursor) + len(fact)


@pytest.mark.parametrize("kind,version,sha", [
    ("major", "v2.0.0", "8503b9506aa566a65c7d1b2ea66c2cf0c9ae39c46d56eed809923c0369dcc42b"),
    ("minor", "v1.0.3", "6eebdada39f3538b9f4d62154480ce1ba1343227fcc734bbe16a0887ee674990"),
])
def test_historical_fixtures_retain_every_command_and_assertion(kind, version, sha):
    text = (MODULE / f"{kind}_upgrade_test.yaml").read_text()
    suite = yaml.safe_load(text)
    assert len(suite) == 1
    name = f"Historical {version} {kind.title()} Release Upgrade"
    assert suite[0]["name"] == name
    original = text.replace(name, f"Test {kind.title()} Release Upgrade", 1)
    assert hashlib.sha256(original.encode()).hexdigest() == sha
    assert suite[0]["inputs"][0]["cmd"] == f'echo "{version}"'
    assert suite[0]["verifiers"]


def scenario(kind):
    cases = yaml.safe_load((MODULE / f"current_{kind}_upgrade_test.yaml").read_text())
    assert len(cases) == 1
    case = cases[0]
    assert case["name"] == f"Current merged-network {kind} upgrade state and restart proof"
    assert len(case["inputs"]) == 1
    step = case["inputs"][0]
    command = shlex.split(step["cmd"])
    assert command[:2] == ["python3", "tests/integration/upgrade_module/merge_upgrade_proof.py"]
    assert command[2:] == [
        "--kind", kind, "--kit", f"/run/upgrade-proof/{kind}.json", "--kit-sha256",
        f"$(cat /run/upgrade-proof/{kind}.sha256)",
        "--evidence", f"/run/upgrade-proof/{kind}-evidence.json",
    ]
    assert step["node"] == "pax-node-0" and step["timeout"] == 1200
    assert step["env"] == "UPGRADE_STATE_PROOF"
    assert case["verifiers"] == [{"type": "eval", "expr": 'UPGRADE_STATE_PROOF == "PASS"'}]
    return function("run")


def test_major_merge_state_coverage():
    run = scenario("major")
    assert 'plan["name"] == "v7.0"' in function("validate_kit")
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")
    ordered(run, 'node.start(kit["old"])', "node.scheduled()", "before_receipts = produce(",
            "before = snapshot(", 'assert_state(before["state"], kit["before"])',
            'if kind == "major":', "node.process.wait(timeout=180)",
            "node.process.returncode != 0", 'node.home / "data/upgrade-info.json"',
            'halt["info"] == kit["plan"]["info"]', 'node.start(kit["new"])',
            "node.applied()", "preserved = snapshot(",
            'assert_state(preserved["state"], before["state"])',
            "retained(node.rpc, before, before_receipts)")
    validation = function("validate_kit")
    assert 'info["leaders"] and info["receipt_keys"] and info.get("last_seq", 0) == 0' in validation
    upgrade = source("node/upgrades.go")
    assert "app.UpgradeKeeper.SetUpgradeHandler(MergeUpgrade, app.mergeUpgradePlan)" in upgrade
    assert "state.LastSeq != 0" in upgrade and "state.ReceiptKeys" in upgrade


def test_minor_merge_state_coverage():
    run = scenario("minor")
    assert 'json.loads(plan["info"])["upgradeType"] == "minor"' in function("validate_kit")
    ordered(run, "before = snapshot(", 'int(before["height"], 16) < kit["plan"]["height"]',
            "else:\n            node.stop()", 'node.start(kit["new"])',
            'node.rpc.call("eth_blockNumber", [])', "node.applied()",
            'assert_state(preserved["state"], before["state"])',
            "after_receipts = produce(", "after = snapshot(",
            'assert_state(after["state"], kit["after"])')
    upgrade = source("node/upgrades.go")
    assert "for _, upgradeName := range upgradesList" in upgrade
    assert "app.UpgradeKeeper.SetUpgradeHandler(upgradeName" in upgrade
    assert "return app.mm.RunMigrations(ctx, app.configurator, fromVM)" in upgrade


def test_digest_pins_governance_and_owned_process_lifecycle():
    ordered(function("run"), 'pinned_binary(kit["old"])', 'pinned_binary(kit["new"])',
            "node = OwnedNode(", 'node.start(kit["old"])')
    ordered(function("pinned_binary"), "path.resolve(strict=True) == path", "os.access(path, os.X_OK)",
            'digest(path) == row["sha256"]', 'stream.read(4) == b"\\x7fELF"',
            'subprocess.run([str(path), "version"]', 'result.stdout.strip() == row["version"]')
    validation = function("validate_kit")
    assert 'old["sha256"] != new["sha256"]' in validation
    assert 'old["source_revision"] != new["source_revision"]' in validation
    assert 'kit["evm_chain_id"] == 125' in validation
    assert 'url.hostname == "127.0.0.1"' in validation
    assert 'home.name == "disposable-upgrade-home"' in validation
    ordered(function("start"), "self.binary = pinned_binary(row)",
            '"--home", str(self.home)', 'self.home / "sequencer-wal"', "subprocess.Popen(args")
    assert 'if key != "UPGRADE_VERSION_LIST"' in SOURCE
    ordered(function("stop"), "os.killpg(self.process.pid, signal.SIGTERM)",
            "self.process.wait(timeout=30)")
    assert '"query", "upgrade", "plan"' in function("scheduled")
    assert 'actual["info"] == expected["info"]' in function("scheduled")
    assert '"query", "upgrade", "applied", plan["name"]' in function("applied")
    assert 'json.loads(result.stdout)["header"]["height"]' in function("applied")
    cli = source("sdk/x/upgrade/client/cli/query.go")
    assert 'Use:   "applied [upgrade-name]"' in cli
    assert 'MarshalJSONIndent(headers.BlockMetas[0]' in cli
    ordered(function("main"), "digest(args.kit) == args.kit_sha256", "evidence = run(",
            'args.evidence.open("x")', 'print("PASS")')


def test_kernel_evm_state_reads_use_real_abi_at_one_committed_height():
    snapshot = function("snapshot")
    ordered(snapshot, 'height = node.call("eth_blockNumber", [])',
            'node.call("eth_getBlockByNumber", [height, False])', 'block["number"] == height')
    assert PROOF.LEDGER == "0x000000000000000000000000000000000000101a"
    assert PROOF.PROGRAMS == "0x000000000000000000000000000000000000101d"
    for surface, name, types in [
        ("layerxledger", "balanceOf", ["bytes32", "bytes32"]),
        ("layerxledger", "nextSequence", ["bytes32", "bytes32"]),
        ("layerxprograms", "getStateHead", ["bytes32"]),
    ]:
        abi = yaml.safe_load(source(f"precompiles/{surface}/abi.json"))
        matches = [row for row in abi if row.get("type") == "function" and row["name"] == name]
        assert len(matches) == 1 and matches[0]["stateMutability"] == "view"
        assert [row["type"] for row in matches[0]["inputs"]] == types
        assert f'"{name}", {types!r}'.replace("'", '"') in snapshot
    assert 'evm.encode_call(method, types, args)' in snapshot
    assert 'node.call("eth_call", [{"to": address, "data": data}, height])' in snapshot
    assert 'node.call("eth_getBalance", [watch["evm_account"], height])' in snapshot
    assert 'node.call("eth_getStorageAt", [watch["contract"], watch["storage_slot"], height])' in snapshot
    assert '"latest"' not in snapshot
    assert PROOF.STATE_KEYS == {"kernel_balance", "kernel_sequence", "kernel_work", "evm_balance", "evm_storage"}


def test_receipts_prove_real_producer_transport_and_commit_consumer_path():
    producer = function("produce")
    ordered(producer, "evm.decode_signed(raw)", "tx.chain_id == 125", "tx.to.hex()",
            'node.call("eth_sendRawTransaction", [row["raw"]])',
            'reference == "0x" + evm.keccak256(raw).hex()', "node.wait_committed(reference)")
    assert '{"kernel": LEDGER, "work": PROGRAMS, "evm": watch["contract"]}' in producer
    chain = source("tools/qualification/paxeer-x/fixtures/merged_chain.py")
    for fact in (
        "connection.request('POST', path, data", "connection.close()",
        "result['id'] == self.sequence", "'error' in result", "raise RpcError(result['error'])",
        "self.call('pax_sequencerReceipt', [transaction_hash])", "row.get('rung') == 'committed'",
        "_EXECUTION.verified_execution(row, transaction, {'receipt_keys': self.receipt_keys})",
        "self.call('eth_getTransactionReceipt', [transaction_hash])",
        "standard.get('blockHash') == row['blockHash']", "standard.get('status') == row['status']",
        "transaction_hash in block.get('transactions', [])", "return self.committed(transaction_hash)",
    ):
        assert fact in chain, fact
    verifier = source("tools/qualification/paxeer-x/paid-web-contract.py")
    assert "verify_sequencer_receipt(rpc_bytes(row[\"receipt\"]), transaction, pins)" in verifier
    send = source("rpc/send.go")
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    receipt = source("rpc/sequencer_receipt.go")
    assert 'RungCommitted Rung = "committed"' in receipt
    assert 'result.Rung, result.BlockNumber, result.BlockHash = RungCommitted' in receipt


def test_restart_reopens_same_home_and_rechecks_state_blocks_and_receipts():
    run = function("run")
    ordered(run, "after_receipts = produce(", "after = snapshot(",
            'assert_state(after["state"], kit["after"])', "node.stop()", 'node.start(kit["new"])',
            'int(after["height"], 16)', "recovered = snapshot(",
            'assert_state(recovered["state"], after["state"])',
            "retained(node.rpc, before, before_receipts)", "retained(node.rpc, after, after_receipts)",
            "node.applied()", "return evidence", "finally:\n        node.stop()")
    assert 'node.call("eth_getBlockByNumber", [saved["height"], False]) == saved["block"]' in function("retained")
    assert 'node.committed(receipt["transaction_hash"]) == receipt' in function("retained")
    assert '"--home", str(self.home)' in function("start")


@pytest.mark.parametrize("axis", sorted(PROOF.STATE_KEYS))
def test_state_validator_rejects_loss_in_every_preserved_domain(axis):
    state = {key: "0x" + (index + 1).to_bytes(32, "big").hex()
             for index, key in enumerate(sorted(PROOF.STATE_KEYS))}
    PROOF.assert_state(state, copy.deepcopy(state))
    corrupted = dict(state, **{axis: "0x" + "00" * 32})
    with pytest.raises(ValueError, match="kernel/EVM state changed"):
        PROOF.assert_state(corrupted, state)
    missing = dict(state)
    del missing[axis]
    with pytest.raises(ValueError, match="state fields"):
        PROOF.assert_state(missing, state)


def test_artifact_guard_refuses_changed_bytes_and_non_native_artifacts(tmp_path):
    artifact = tmp_path / "paxd"
    artifact.write_bytes(SCRIPT.read_bytes())
    artifact.chmod(0o700)
    row = {"path": str(artifact), "sha256": PROOF.digest(artifact), "version": "unused"}
    with pytest.raises(ValueError, match="native paxd required"):
        PROOF.pinned_binary(row)
    artifact.write_bytes(artifact.read_bytes() + b"\n")
    with pytest.raises(ValueError, match="artifact digest differs"):
        PROOF.pinned_binary(row)


def test_repair_record_maps_both_findings_and_distinguishes_source_from_runtime_proof():
    record = tomllib.loads(Path(__file__).with_name("repair_22_33_760.kvx").read_text())
    assert set(record["finding"]) == {"major-merge-state-coverage", "minor-merge-state-coverage"}
    tests = {node.name for node in ast.parse(Path(__file__).read_text()).body
             if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    for kind in ("major", "minor"):
        finding = record["finding"][f"{kind}-merge-state-coverage"]
        assert finding["audit"] == "22.30.172"
        assert finding["file"] == f"tests/integration/upgrade_module/{kind}_upgrade_test.yaml"
        assert f"test_{kind}_merge_state_coverage" in finding["tests"]
        assert set(finding["tests"]) <= tests
    repair = record["repair"]
    assert repair["base_revision"] == "76c050242a1edcad6212aa52fac15670ecb5612d"
    assert repair["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_760.py"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "No pre-change verification was run" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    assert "same candidate revision" in repair["evidence"]
    assert "tools/release" in repair["wiring_request"] and "domain 16" in repair["wiring_request"]
    assert "pre-merge" in repair["activation_limit"]
