"""Qualify the real shell evidence consumer without starting a chain/toolchain."""

import importlib.util
import json
from pathlib import Path
import re
import subprocess
import sys

import pytest


ROOT = Path(__file__).resolve().parents[3]
CONTRACTS = ROOT / "tests/integration/contracts"
SCRIPTS = ("verify_flatkv_statesync_crash_recovery.sh", "verify_flatkv_total_loss_recovery.sh")
ADDRESS = "0x" + "12" * 20
SLOT = "0x" + "00" * 32
VALUE = "0x" + "00" * 31 + "2a"
CODE = "0x602a60005260206000f3"
ACCEPTED = 'level=info msg="Snapshot accepted, restoring" height=300 format=1 hash=ABCD1234\n'
CHUNK = 'level=info msg="Applied snapshot chunk to ABCI app" height=300 format=1 chunk={} total=3\n'
RESTORED = 'level=info msg="Snapshot restored" height=300 format=1 hash=ABCD1234\n'
HANDOFF = ('level=info msg="switching to consensus reactor" height=301 '
           'blocks_synced=1 state_synced=true max_peer_height=305\n')
IMPORT = ACCEPTED + ''.join(CHUNK.format(i) for i in range(3)) + RESTORED + HANDOFF


def consumer():
    spec = importlib.util.spec_from_file_location("flatkv_checks_741", CONTRACTS / "flatkv_contract_checks.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def source(name):
    return (CONTRACTS / name).read_text()


def function(text, name):
    match = re.search(r"^" + name + r"\(\) \{\n(.*?)^\}", text, re.M | re.S)
    assert match, name
    return match[1]


def ordered(text, *parts):
    cursor = 0
    for part in parts:
        cursor = text.index(part, cursor) + len(part)


def row(key, value):
    return f"Key: {key.hex().upper()}, Value: {value.hex().upper()}\n"


@pytest.fixture
def state(tmp_path):
    """Files use the committed dump/ABCI/Pebble formats pinned below."""
    fixture = tmp_path / "fixture"
    fixture.mkdir()
    for name, value in [("contract_addr", ADDRESS), ("storage_slot", SLOT),
                        ("storage_expected", VALUE), ("code_expected", CODE)]:
        (fixture / f"flatkv_evm_{name}.txt").write_text(value + "\n")
    (fixture / "flatkv_evm_deploy_receipt.json").write_text(json.dumps(
        {"result": {"status": "0x1", "blockNumber": "0x29"}}))
    dump = tmp_path / "dump"
    dump.mkdir()
    address = bytes.fromhex(ADDRESS[2:])
    envelope = b"\x00" + (41).to_bytes(8, "big")
    records = {
        "account": (b"evm/\x0a" + address, envelope + bytes(32) + (1).to_bytes(8, "big") + b"\x11" * 32),
        "storage": (b"evm/\x03" + address + bytes(32), envelope + bytes.fromhex(VALUE[2:])),
        "code": (b"evm/\x07" + address, envelope + bytes.fromhex(CODE[2:])),
    }
    for bucket, pair in records.items():
        (dump / bucket).write_text(f"Bucket {bucket} at version 305\n" + row(*pair))
    chunk_root = tmp_path / "chunks"
    queue = chunk_root / "tm-statesync12345"
    queue.mkdir(parents=True)
    (queue / "0").write_bytes(b"persisted snapshot chunk peer payload")
    flatkv = tmp_path / "flatkv"
    working = flatkv / "working/account"
    working.mkdir(parents=True)
    (working / "000001.log").write_bytes(b"persisted importer Pebble WAL peer bytes")
    logfile = tmp_path / "paxd.log"
    logfile.write_text(ACCEPTED + CHUNK.format(0))
    return {"fixture": fixture, "dump": dump, "records": records,
            "chunks": chunk_root, "flatkv": flatkv, "log": logfile,
            "partial": tmp_path / "partial.json"}


@pytest.mark.parametrize("name", SCRIPTS)
def test_shell_syntax_and_guarded_fixture_path(name):
    parsed = subprocess.run(["bash", "-n", str(CONTRACTS / name)],
                            capture_output=True, text=True, timeout=10)
    assert parsed.returncode == 0, parsed.stderr
    text = source(name)
    dump = function(text, "assert_flatkv_dump_contains_fixture")
    ordered(dump, 'if ! ensure_paxdb "$node"; then', 'return 1',
            'if ! docker exec "$node" bash -lc', 'set -euo pipefail',
            'build/paxdb dump-flatkv --db-dir $FLATKV_DIR --height $COMPARE_VERSION',
            "python3 '$RECOVERY_CHECKS' fixture", "'$COMPARE_VERSION'", 'then', 'return 1')
    assert "grep" not in dump
    remote = dump.split('bash -lc "', 1)[1].rsplit('"; then', 1)[0]
    remote = remote.replace('\\"', '"').replace('\\$', '$')
    parsed_remote = subprocess.run(["bash", "-n"], input=remote,
                                   capture_output=True, text=True, timeout=10)
    assert parsed_remote.returncode == 0, parsed_remote.stderr
    recovered = function(text, "assert_flatkv_recovered")
    ordered(recovered, 'local failed=0', 'assert_recovery_path || failed=1',
            'select_compare_version || exit 1',
            'assert_flatkv_dump_contains_fixture "$DONOR_NODE" || failed=1',
            'assert_flatkv_dump_contains_fixture "$VICTIM_NODE" || failed=1',
            'assert_evm_fixture_queries "$VICTIM_NODE" || failed=1',
            'if [ "$failed" -ne 0 ]; then', 'exit 1')
    select = function(text, "select_compare_version")
    assert 'donor_height - 2 : victim_height - 2' in select
    assert '[ "$COMPARE_VERSION" -lt "$RECOVERED_SNAPSHOT_HEIGHT" ]' in select
    ordered(text, "stop_height=$(node_height", "before.log", 'configure_statesync "$VICTIM_NODE"', "start_victim")


def test_statesync_kill_window_requires_persisted_progress(state):
    module = consumer()
    module.main(["partial", str(state["log"]), str(state["chunks"]),
                 str(state["flatkv"]), str(state["partial"])])
    saved = json.loads(state["partial"].read_text())
    assert (saved["snapshot_height"], saved["chunk"], saved["total"]) == (300, 0, 3)
    assert saved["chunk_sha256"] == module.hashlib.sha256(
        (state["chunks"] / "tm-statesync12345/0").read_bytes()).hexdigest()
    text = source(SCRIPTS[0])
    kill = function(text, "wait_for_statesync_log_and_kill")
    ordered(kill, "Applied snapshot chunk to ABCI app", 'pkill -STOP -x paxd',
            'if docker exec "$node" python3 "$RECOVERY_CHECKS" partial',
            'pkill -KILL -x paxd', 'return 0', 'pkill -CONT -x paxd',
            'crash window unobserved', 'return 1')
    assert "killing $node anyway" not in text
    assert 'temp-dir = \\"${CHUNK_ROOT}\\"' in function(text, "configure_statesync")
    ordered(text, 'wait_for_statesync_log_and_kill "$VICTIM_NODE"',
            'survived the injected state-sync SIGKILL',
            'mv "$RECOVERY_LOG" "$RECOVERY_EVIDENCE/interrupted.log"',
            'echo "Restarting', 'start_victim', 'wait_for_catchup', 'assert_flatkv_recovered')


@pytest.mark.parametrize("bad", ["intent", "final", "completed", "missing-chunk", "no-import-writes", "published", "wrong-height", "ambiguous"])
def test_statesync_kill_window_rejects_unobserved_or_finished_import(state, bad):
    if bad == "intent":
        state["log"].write_text('msg="This node needs state sync"\n' + ACCEPTED)
    elif bad == "final":
        state["log"].write_text(ACCEPTED + CHUNK.format(2))
    elif bad == "completed":
        state["log"].write_text(IMPORT)
    elif bad == "missing-chunk":
        (state["chunks"] / "tm-statesync12345/0").unlink()
    elif bad == "no-import-writes":
        (state["flatkv"] / "working/account/000001.log").unlink()
    elif bad == "published":
        (state["flatkv"] / "working/SNAPSHOT_BASE").write_text("snapshot-00000000000000000000\n")
    elif bad == "wrong-height":
        state["log"].write_text(ACCEPTED + CHUNK.format(0).replace("height=300", "height=299"))
    else:
        duplicate = state["chunks"] / "tm-statesyncOTHER"
        duplicate.mkdir()
        (duplicate / "0").write_bytes(b"unattributed stale chunk")
    with pytest.raises(ValueError):
        consumer().partial_import(state["log"], state["chunks"], state["flatkv"], state["partial"])
    assert not state["partial"].exists()


def test_statesync_restart_reopens_partial_and_recovers_same_snapshot(state):
    def run_consumer(*args):
        completed = subprocess.run(
            [sys.executable, str(CONTRACTS / "flatkv_contract_checks.py"), *map(str, args)],
            capture_output=True, text=True, timeout=10,
        )
        assert completed.returncode == 0, completed.stderr
        return completed.stdout
    # The actual consumer process writes and fsyncs the checkpoint, then exits.
    run_consumer("partial", state["log"], state["chunks"], state["flatkv"], state["partial"])
    saved = state["partial"].read_bytes()
    state["log"].write_text(IMPORT)
    (state["flatkv"] / "working/SNAPSHOT_BASE").write_text("snapshot-00000000000000000300\n")
    # A new interpreter reopens that checkpoint; no live consumer state crosses the restart.
    proof = json.loads(run_consumer("imported", state["log"], "250", "305", state["flatkv"], state["partial"]))
    assert state["partial"].read_bytes() == saved
    assert proof["snapshot_height"] == 300
    assert proof["snapshot_hash"] == "ABCD1234"
    assert proof["partial_outcome"] == "discarded-and-reimported-same-snapshot"
    restored = json.loads(run_consumer("fixture", state["dump"], state["fixture"], "305"))
    assert (restored["contract"], restored["storage"], restored["code"], restored["version"]) == (ADDRESS, VALUE, CODE, 305)


@pytest.mark.parametrize("bad", ["replay", "unknown", "different", "missing-zero", "missing-last", "unpublished", "wrong-offer", "ahead", "changed-chunk"])
def test_statesync_resume_rejects_fallback_and_incomplete_recovery(state, bad):
    module = consumer()
    module.partial_import(state["log"], state["chunks"], state["flatkv"], state["partial"])
    log = IMPORT
    marker = state["flatkv"] / "working/SNAPSHOT_BASE"
    marker.write_text("snapshot-00000000000000000300\n")
    if bad == "replay":
        log = IMPORT.replace("state_synced=true", "state_synced=false")
    elif bad == "unknown":
        log = ACCEPTED
    elif bad == "different":
        log = IMPORT.replace("ABCD1234", "ABCDEF99")
    elif bad == "missing-zero":
        log = IMPORT.replace(CHUNK.format(0), "")
    elif bad == "missing-last":
        log = IMPORT.replace(CHUNK.format(2), "")
    elif bad == "unpublished":
        marker.write_text("snapshot-00000000000000000000\n")
    elif bad == "wrong-offer":
        log = IMPORT.replace(ACCEPTED, ACCEPTED.replace("height=300", "height=299"))
    elif bad == "changed-chunk":
        (state["chunks"] / "tm-statesync12345/0").write_bytes(b"corrupt after restart")
    else:
        marker.write_text("snapshot-00000000000000000400\n")
    state["log"].write_text(log)
    with pytest.raises(ValueError):
        module.recovered_import(state["log"], 250, 305, state["flatkv"], state["partial"])


@pytest.mark.parametrize("name", SCRIPTS)
def test_restored_rows_bind_composite_keys_typed_values_and_selected_version(state, name, capsys):
    consumer().main(["fixture", str(state["dump"]), str(state["fixture"]), "305"])
    proof = json.loads(capsys.readouterr().out)
    assert proof["storage_key"] == (b"evm/\x03" + bytes.fromhex(ADDRESS[2:] + SLOT[2:])).hex()
    assert (proof["contract"], proof["storage"], proof["code"], proof["version"]) == (ADDRESS, VALUE, CODE, 305)
    assert "'$COMPARE_VERSION'" in function(source(name), "assert_flatkv_dump_contains_fixture")


@pytest.mark.parametrize("bad", ["split-rows", "wrong-slot", "wrong-contract", "wrong-height", "wrong-value", "wrong-tag", "short", "duplicate", "wrong-version", "wrong-code", "wrong-account", "failed-receipt"])
def test_restored_rows_reject_unrelated_or_malformed_evidence(state, bad):
    path = state["dump"] / "storage"
    key, value = state["records"]["storage"]
    header = "Bucket storage at version 305\n"
    if bad == "split-rows":
        path.write_text(header + row(key, value[:-1] + b"\x01") + row(key[:-1] + b"\x01", value))
    elif bad == "wrong-slot":
        path.write_text(header + row(key[:-1] + b"\x01", value))
    elif bad == "wrong-contract":
        path.write_text(header + row(b"evm/\x03" + b"\x34" * 20 + bytes(32), value))
    elif bad == "wrong-height":
        path.write_text(header + row(key, b"\x00" + (40).to_bytes(8, "big") + value[9:]))
    elif bad == "wrong-value":
        path.write_text(header + row(key, value[:-1] + b"\x01"))
    elif bad == "wrong-tag":
        path.write_text(header + row(key, b"\x01" + value[1:]))
    elif bad == "short":
        path.write_text(header + row(key, value[:-1]))
    elif bad == "duplicate":
        path.write_text(header + row(key, value) * 2)
    elif bad == "wrong-version":
        path.write_text(path.read_text().replace("version 305", "version 304"))
    elif bad == "wrong-code":
        (state["dump"] / "code").write_text("Bucket code at version 305\n" + row(b"other-contract", state["records"]["code"][1]))
    elif bad == "wrong-account":
        (state["dump"] / "account").write_text("Bucket account at version 305\n" + row(b"other-contract", state["records"]["account"][1]))
    else:
        (state["fixture"] / "flatkv_evm_deploy_receipt.json").write_text('{"result":{"status":"0x0","blockNumber":"0x29"}}')
    with pytest.raises(ValueError):
        consumer().restored_fixture(state["dump"], state["fixture"], 305)


def test_total_loss_requires_published_snapshot_import(state):
    state["log"].write_text(IMPORT)
    (state["flatkv"] / "working/SNAPSHOT_BASE").write_text("snapshot-00000000000000000300\n")
    assert consumer().recovered_import(state["log"], 250, 305, state["flatkv"])["snapshot_height"] == 300
    path = function(source(SCRIPTS[1]), "assert_recovery_path")
    assert 'python3 "$RECOVERY_CHECKS" imported' in path
    assert "replay" not in path and "|| true" not in path
    state["log"].write_text(IMPORT.replace("state_synced=true", "state_synced=false"))
    with pytest.raises(ValueError):
        consumer().recovered_import(state["log"], 250, 305, state["flatkv"])


def test_replay_is_explicit_separately_asserted_scenario(state):
    text = source(SCRIPTS[0])
    assert 'RECOVERY_SCENARIO=${FLATKV_STATESYNC_RECOVERY_SCENARIO:-snapshot-import}' in text
    path = function(text, "assert_recovery_path")
    ordered(path, '[ "$RECOVERY_SCENARIO" = "blocksync-replay" ]',
            'python3 "$RECOVERY_CHECKS" replay', 'return 0', 'python3 "$RECOVERY_CHECKS" imported', 'partial.json')
    ordered(text, 'if [ "$RECOVERY_SCENARIO" = "blocksync-replay" ]; then',
            'cp /root/.pax/data/priv_validator_state.json', 'rm -rf /root/.pax/data /root/.pax/wasm',
            'enable = false', 'echo "Restarting', 'start_victim')
    state["log"].write_text(HANDOFF.replace("state_synced=true", "state_synced=false"))
    assert consumer().replay_recovery(state["log"], 305)["recovery_path"] == "blocksync-replay"
    for log in ["", IMPORT, HANDOFF.replace("blocks_synced=1", "blocks_synced=0").replace("state_synced=true", "state_synced=false")]:
        state["log"].write_text(log)
        with pytest.raises(ValueError):
            consumer().replay_recovery(state["log"], 305)


def test_real_producer_formats_and_safe_discard_match_consumer():
    dump = (ROOT / "storage/tools/cmd/paxdb/operations/dump_flatkv.go").read_text()
    assert '"Bucket %s at version %d\\n"' in dump and '"Key: %X, Value: %X\\n"' in dump
    keys = (ROOT / "storage/common/keys/evm.go").read_text()
    for name, value in [("stateKeyPrefix", "03"), ("codeKeyPrefix", "07"), ("nonceKeyPrefix", "0a")]:
        assert re.search(name + r"\s*= \[\]byte\{0x" + value + r"\}", keys)
    for bucket in ("storage", "code", "account"):
        schema = (ROOT / f"storage/state_db/sc/flatkv/vtype/{bucket}_data.go").read_text()
        assert "binary.BigEndian.PutUint64" in schema
        assert f"{bucket.title()}DataVersion0 {bucket.title()}DataVersion = 0" in schema
    chunks = (ROOT / "consensus/internal/statesync/chunks.go").read_text()
    assert 'os.MkdirTemp(tempDir, "tm-statesync")' in chunks
    assert 'os.WriteFile(path, chunk.Chunk, 0600)' in chunks
    syncer = (ROOT / "consensus/internal/statesync/syncer.go").read_text()
    assert 'logger.Info("Applied snapshot chunk to ABCI app", "height", chunk.Height,' in syncer
    assert '"chunk", chunk.Index, "total", chunks.Size()' in syncer
    store = (ROOT / "storage/state_db/sc/flatkv/store.go").read_text()
    reset = store[store.index("func (s *CommitStore) resetForImport() error") :]
    assert 'atomicRemoveDir(filepath.Join(dir, workingDirName))' in reset
    importer = (ROOT / "storage/state_db/sc/flatkv/importer.go").read_text()
    ordered(importer, 'os.Remove(marker)', 'func (imp *KVImporter) Close()',
            'imp.store.FinalizeImport(imp.version)', 'imp.store.WriteSnapshot("")')


def test_record_maps_every_finding_and_records_proof_limits():
    record = Path(__file__).with_name("repair_22_33_741.kvx").read_text()
    for finding in ("statesync-kill-window", "statesync-restored-row", "statesync-resume-fallback",
                    "total-loss-restored-row", "total-loss-path-label"):
        section = record.split(f"[finding.{finding}]", 1)[1].split("\n[", 1)[0]
        names = re.findall(r'"(test_[a-z0-9_]+)"', section)
        assert names and all(callable(globals().get(name)) for name in names)
    assert "baseline_not_run" in record and "review_only" in record
    assert "[wiring.blocksync_scenario]" in record
