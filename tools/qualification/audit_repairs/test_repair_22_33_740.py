"""Source contracts plus real in-process consumers; no node/toolchain is required."""

import importlib.util
import json
from pathlib import Path
import re
import shlex
import subprocess

import pytest


ROOT = Path(__file__).resolve().parents[3]
CONTRACTS = ROOT / "tests/integration/contracts"
RECORD = Path(__file__).with_name("repair_22_33_740.kvx")
SCRIPTS = (
    "deploy_dex_contract.sh", "deploy_flatkv_evm_fixture.sh",
    "deploy_timelocked_token_contract.sh", "verify_flatkv_evm_migrate.sh",
    "verify_flatkv_evm_store.sh", "verify_flatkv_only_statesync.sh",
    "verify_flatkv_partial_loss_fails_loudly.sh",
)
ADDRESS = "0x" + "12" * 20
SLOT = "0x" + "00" * 32
VALUE = "0x" + "00" * 31 + "2a"
CODE = "0x602a60005260206000f3"
RESTORED = {"contract": ADDRESS, "storage": VALUE, "code": CODE}
IMPORT_LOG = (
    'level=info msg="Snapshot accepted, restoring" height=300 format=1\n'
    'level=info msg="Snapshot restored" height=300 format=1 hash=ABCD1234\n'
    'level=info msg="switching to consensus reactor" height=301 '
    'blocks_synced=1 state_synced=true max_peer_height=305\n'
)


def checks_module():
    spec = importlib.util.spec_from_file_location(
        "flatkv_contract_checks_740", CONTRACTS / "flatkv_contract_checks.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def source(name):
    return (CONTRACTS / name).read_text(encoding="utf-8")


def function(text, name):
    match = re.search(r"^" + re.escape(name) + r"\(\) \{\n(.*?)^\}", text, re.M | re.S)
    assert match, name
    return match[1]


def ordered(text, *parts):
    position = 0
    for part in parts:
        position = text.index(part, position) + len(part)


def row(key=None, value=None, height=41, tag=0):
    if key is None:
        key = b"evm/\x03" + bytes.fromhex(ADDRESS[2:] + SLOT[2:])
    if value is None:
        value = bytes([tag]) + height.to_bytes(8, "big") + bytes.fromhex(VALUE[2:])
    return f"Key: {key.hex().upper()}, Value: {value.hex().upper()}\n"


@pytest.mark.parametrize("name", SCRIPTS)
def test_shell_syntax(name):
    # Parse separately: bash -n with several filenames parses only its first file.
    result = subprocess.run(["bash", "-n", str(CONTRACTS / name)],
                            capture_output=True, text=True, timeout=10)
    assert result.returncode == 0, result.stderr


def test_retired_dex_contracts_use_supported_product_operations():
    text = source("deploy_dex_contract.sh")
    assert 'set -euo pipefail' in text
    assert '[ "$1" != "cw20_base" ]' in text
    assert "store_wasm contracts/wasm/cw20_base.wasm" in text
    assert not re.search(r"mars|saturn|venus|tx dex|register-contract|register-pairs", text)
    assert (ROOT / "contracts/wasm/cw20_base.wasm").read_bytes().startswith(b"\x00asm")
    init = re.search(r"'(\{name:.*\})'", text)[1]
    assert 'initial_balances:[{address:$owner,amount:"1000000"}]' in init
    burn = re.search(r"'(\{\"burn\":.*\})'", text)[1]
    assert json.loads(burn) == {"burn": {"amount": "1"}}
    ordered(text, "instantiate_wasm", "submit_tx_and_wait", '[ "$code" = "0" ]',
            '_wait_until "CW20 burn committed"', '.total_supply == "999999"',
            '.data.balance == "999999"', 'cw20_base-addr.txt')


def test_fixture_self_oracle_checks_embedded_runtime_and_slot_before_writing():
    text = source("deploy_flatkv_evm_fixture.sh")
    constants = dict(re.findall(r"^(STORAGE_[A-Z_]+)=(0x[0-9a-f]+)$", text, re.M))
    init = bytes.fromhex(constants["STORAGE_CONTRACT_INIT_CODE"][2:])
    runtime = bytes.fromhex(constants["STORAGE_CONTRACT_RUNTIME_CODE"][2:])
    # Decode constructor instructions and its RETURN copy window independently.
    assert init[:5] == bytes.fromhex("602a600055")  # PUSH1 42; PUSH1 0; SSTORE
    assert init[5:17] == bytes.fromhex("600a6011600039600a6000f3")
    assert init[17:] == runtime == checks_module().RUNTIME
    assert int(constants["STORAGE_SLOT_VALUE"], 16) == 42
    assert int(constants["STORAGE_SLOT_ZERO"], 16) == 0
    guard = 'if [ "${storage_expected,,}" != "$STORAGE_SLOT_VALUE" ] || [ "${code_expected,,}" != "$STORAGE_CONTRACT_RUNTIME_CODE" ]; then'
    ordered(text, 'require_success_receipt "contract deployment"',
            'storage_expected=$(query_storage', 'code_expected=$(query_code', guard,
            "exit 1", 'write_fixture "flatkv_evm_storage_expected.txt"',
            'write_fixture "flatkv_evm_code_expected.txt"')


def test_timelock_key_output_is_isolated_disposable_and_public_only():
    text = source("deploy_timelocked_token_contract.sh")
    ordered(text, "set +x", "keys list", "umask 077", "disposable_keyring=$(mktemp -d)",
            "trap 'rm -rf -- \"$disposable_keyring\"' EXIT", "new_public_address()")
    create = function(text, "new_public_address")
    commands = [shlex.split(line.replace("\\", "")) for line in
                create.replace("\\\n", " ").splitlines() if "keys " in line]
    assert len(commands) == 2
    add, show = commands
    assert add[1:3] == ["keys", "add"]
    for command in commands:
        assert command[command.index("--keyring-backend") + 1] == "test"
        assert command[command.index("--keyring-dir") + 1] == "$disposable_keyring"
    assert ">/dev/null" in add and "2>&1" in add
    assert "return" in add and "1" in add
    assert show[1:3] == ["keys", "show"] and "-a" in show
    assert text.count("keys add") == 1
    assert re.findall(r"=\$\(new_public_address (\w+)\) \|\| exit 1", text) == [
        "admin1", "admin2", "admin3", "admin4", "op", "staking_reward_dest", "unlocked_dest"
    ]
    assert text.count("bank_send_and_wait admin ") == 7


def test_migration_height_boundary_covers_every_completion_and_buffer():
    text = source("verify_flatkv_evm_migrate.sh")
    ordered(text, "declare -A completion_heights=()", 'complete=$(echo "$status_json"',
            'version_at=$(echo "$status_json"', 'height=$(node_height "$node")',
            'completion_heights[$node]=$completion_height', 'if $all_done; then',
            'comparison_boundary=$((max_completion_height + COMPARE_BUFFER))',
            'progress_target=$((comparison_boundary + COMPARE_BUFFER + 1))',
            'wait_for_cluster_height_sync "$progress_target" "$COMPLETION_TIMEOUT"',
            'COMPARE_VERSION=$(pick_compare_height)',
            'if [ "$COMPARE_VERSION" -le "$comparison_boundary" ]; then',
            'exit 1', 'digest=$(flatkv_dump_digest "$node" "$COMPARE_VERSION")')
    assert 'if [ "$version_at" -gt "$completion_height" ]; then' in text
    assert 'if [ "$completion_height" -gt "$max_completion_height" ]; then' in text
    assert '[[ "$height" =~ ^[1-9][0-9]*$ ]]' in text
    assert '[[ "$version_at" =~ ^[1-9][0-9]*$ ]]' in text
    # Independent arithmetic examples include unequal validator completion heights.
    for completions, buffer in [([8, 12, 9, 11], 2), ([600, 800, 700, 750], 5)]:
        boundary = max(completions) + buffer
        selected = boundary + buffer + 1 - buffer
        assert selected > boundary > max(completions)
    pick = function(text, "pick_compare_height")
    assert 'h=$(node_height "pax-node-$i")' in pick
    assert 'echo $((min - COMPARE_BUFFER))' in pick


def test_storage_dump_row_identity_uses_real_schema_and_receipt(tmp_path):
    module = checks_module()
    dump = tmp_path / "storage"
    dump.write_text("Bucket storage at version 50\n" + row())
    directory = tmp_path / "fixture"
    directory.mkdir()
    for name, value in [("contract_addr", ADDRESS), ("storage_slot", SLOT),
                        ("storage_expected", VALUE)]:
        (directory / f"flatkv_evm_{name}.txt").write_text(value + "\n")
    (directory / "flatkv_evm_deploy_receipt.json").write_text(
        json.dumps({"result": {"status": "0x1", "blockNumber": "0x29"}})
    )
    module.main(["storage", str(dump), str(directory)])
    assert module.storage_row(dump.read_text(), ADDRESS, SLOT, VALUE, 41) == {
        "key": (b"evm/\x03" + bytes.fromhex(ADDRESS[2:] + SLOT[2:])).hex(),
        "value": VALUE[2:], "height": 41, "tag": 0,
    }
    script = source("verify_flatkv_evm_store.sh")
    ordered(script, 'build/paxdb dump-flatkv --db-dir "$flatkv_dir"',
            'python3 "$paxhome/tests/integration/contracts/flatkv_contract_checks.py" storage',
            '"$storage_dump" "$paxhome/tests/integration/contracts"', "verification passed")
    # Pin both physical-key producer and serialization version, not just row length.
    keys = (ROOT / "storage/state_db/sc/flatkv/ktype/ktype.go").read_text()
    assert 'result[len(mod)] = \'/\'' in keys
    assert "result[len(mod)+1] = prefixByte" in keys
    serializer = (ROOT / "storage/state_db/sc/flatkv/vtype/storage_data.go").read_text()
    assert "StorageDataVersion0 StorageDataVersion = 0" in serializer
    assert "binary.BigEndian.PutUint64" in serializer
    assert "copy(s.data[storageValueStart:storageDataLength], value[:])" in serializer


@pytest.mark.parametrize("bad", [
    row(key=b"evm/\x03" + bytes.fromhex("34" * 20 + SLOT[2:])),
    row(key=b"evm/\x03" + bytes.fromhex(ADDRESS[2:] + "00" * 31 + "01")),
    row(key=b"evm/\x07" + bytes.fromhex(ADDRESS[2:] + SLOT[2:])),
    row(height=40), row(tag=1), row(value=b"\x00" * 41),
    row(value=b"\x00" * 40), row() + row(),
    row(value=b"\x00") + row(key=b"different-row"), "Key: nothex, Value: 00\n",
])
def test_storage_dump_row_identity_rejects_wrong_or_separate_rows(bad):
    with pytest.raises(ValueError):
        checks_module().storage_row(bad, ADDRESS, SLOT, VALUE, 41)


def test_only_statesync_fallback_requires_completed_import_and_handoff():
    module = checks_module()
    proof = module.snapshot_import(IMPORT_LOG, 250, 305)
    assert proof == {"snapshot_height": 300, "snapshot_hash": "ABCD1234",
                     "snapshot_format": 1, "recovered_height": 305}
    text = source("verify_flatkv_only_statesync.sh")
    assert "BLOCKSYNC FALLBACK" not in text and "log_recovery_path" not in text
    require = function(text, "require_snapshot_import")
    assert 'docker exec "$node" cat "$logfile" > "$recovery_evidence_dir/import.log"' in require
    assert 'imported_snapshot_height=$(python3 "$checks" snapshot' in require
    assert '"$MIN_DONOR_HEIGHT" "$(node_height "$node")"' in require
    assert '"$recovery_evidence_dir/${node}-fixture.json"' in require
    main = text.split('\nassert_flatkv_only_layout\n', 1)[1]
    ordered(main, 'stop_victim\n', 'cp \\"\\$logfile\\" \\"\\$logfile.before-statesync\\"',
            'rm -rf /root/.pax/data', 'start_victim\n',
            'assert_evm_fixture_queries "$VICTIM_NODE"', 'require_snapshot_import "$VICTIM_NODE"',
            'assert_flatkv_digests_match\n', 'stop_victim\n', 'start_victim\n',
            'assert_evm_fixture_queries "$VICTIM_NODE"', 'python3 "$checks" reopen',
            'assert_flatkv_digests_match\n', 'echo "PASS:')
    producer = (ROOT / "consensus/internal/statesync/syncer.go").read_text()
    ordered(producer, "err = s.applyChunks", "s.verifyApp(ctx", 'logger.Info("Snapshot restored"')
    assert '"hash", snapshot.Hash' in producer


@pytest.mark.parametrize("log,minimum,height", [
    ("", 250, 305),
    (IMPORT_LOG.replace("state_synced=true", "state_synced=false"), 250, 305),
    (IMPORT_LOG.replace('msg="Snapshot restored"', 'msg="Snapshot accepted, restoring"'), 250, 305),
    (IMPORT_LOG, 301, 305), (IMPORT_LOG, 250, 299),
    (IMPORT_LOG.replace("hash=ABCD1234", ""), 250, 305),
    ("\n".join(reversed(IMPORT_LOG.splitlines())), 250, 305),
    (IMPORT_LOG + 'level=info msg="switching to consensus reactor" height=301 state_synced=false\n', 250, 305),
])
def test_only_statesync_fallback_rejects_replay_unknown_stale_and_failed_import(log, minimum, height):
    with pytest.raises(ValueError):
        checks_module().snapshot_import(log, minimum, height)


def test_only_statesync_restart_reopens_retained_height_and_restored_state(tmp_path):
    # Exercise the actual script's evidence producer, close it, create a fresh
    # consumer, reopen persisted state, and reject state/height regression.
    logfile = tmp_path / "import.log"
    logfile.write_text(IMPORT_LOG)
    statefile = tmp_path / "state.json"
    statefile.write_text(json.dumps(RESTORED))
    evidence = tmp_path / "import.json"
    producer = checks_module()
    producer.main(["snapshot", str(logfile), "250", "305", str(statefile), str(evidence)])
    del producer
    consumer = checks_module()
    proof = consumer.reopen_recovery(evidence, 310, RESTORED)
    assert proof["snapshot_height"] == 300
    assert proof["restored_state"] == RESTORED
    consumer.main(["reopen", str(evidence), "310", str(statefile)])
    with pytest.raises(ValueError, match="regressed"):
        consumer.reopen_recovery(evidence, 304, RESTORED)
    with pytest.raises(ValueError, match="embedded contract"):
        consumer.reopen_recovery(evidence, 310, {**RESTORED, "storage": SLOT})
    with pytest.raises(ValueError, match="changed"):
        consumer.reopen_recovery(evidence, 310, {**RESTORED, "contract": "0x" + "34" * 20})
    with pytest.raises(FileExistsError):
        consumer.save_recovery(evidence, proof)


def test_only_statesync_stop_proof_bounds_escalation_and_guards_wipe():
    text = source("verify_flatkv_only_statesync.sh")
    stop = function(text, "stop_victim")
    ordered(stop, 'pkill -TERM -x paxd', 'while [ "$elapsed" -lt "$STOP_TIMEOUT" ]',
            'pkill -KILL -x paxd', 'while [ "$elapsed" -lt "$KILL_TIMEOUT" ]',
            'if docker exec "$VICTIM_NODE" pgrep -x paxd', "refusing state removal", "return 1")
    assert stop.count('[ "$rc" -eq 1 ] || return "$rc"') == 6
    assert '|| true' not in stop
    assert stop.count('sleep 1') == 2
    assert stop.count('elapsed=$((elapsed + 1))') == 2
    main = text.split('\nassert_flatkv_only_layout\n', 1)[1]
    ordered(main, 'stop_victim\n',
            'cp /root/.pax/data/priv_validator_state.json /tmp/flatkv-only-priv-validator-state.json',
            'rm -rf /root/.pax/data',
            'mv /tmp/flatkv-only-priv-validator-state.json /root/.pax/data/priv_validator_state.json')
    # Parse the newly changed remote fixture command as well as the outer shell.
    remote = text.split('if ! docker exec "$node" bash -lc \'', 1)[1].split(
        "\n  ' >", 1
    )[0]
    result = subprocess.run(["bash", "-n"], input=remote, text=True,
                            capture_output=True, timeout=10)
    assert result.returncode == 0, result.stderr


def test_partial_loss_attribution_accepts_exact_missing_state_and_version():
    module = checks_module()
    message = "failed to load FlatKV version: FlatKV version mismatch: requested 41, reached 0"
    assert module.missing_state_error("FlatKV store opened version=0\nError: " + message, 41) == 41
    producer = (ROOT / "storage/state_db/sc/flatkv/store.go").read_text()
    assert '"FlatKV version mismatch: requested %d, reached %d"' in producer
    wrapper = (ROOT / "storage/state_db/sc/composite/store.go").read_text()
    assert 'fmt.Errorf("failed to load FlatKV version: %w", err)' in wrapper
    text = source("verify_flatkv_partial_loss_fails_loudly.sh")
    assert "grep -Eiq 'flatkv|version|missing|LoadVersion|reconcile|state_commit'" not in text
    ordered(text, 'deleted_version=$(docker exec', 'msg="committed state"',
            'if ! [[ "$deleted_version" =~ ^[1-9][0-9]*$ ]]',
            'before-partial-loss', 'rm -rf', 'Restarting $VICTIM_NODE',
            'cat "$logfile" > "$evidence_dir/startup.log"',
            'python3 "$checks" missing "$evidence_dir/startup.log" "$deleted_version"',
            'PASS: $VICTIM_NODE failed loudly')
    assert "wait_for_catchup" in text and "assert_flatkv_digests_match" in text


@pytest.mark.parametrize("log", [
    "FlatKV store opened version=41\nError: address already in use",
    "panic: missing version", "Error: failed to load cosmos version: database corrupt",
    "Error: failed to load FlatKV version: FlatKV version mismatch: requested 42, reached 0",
    "Error: failed to load FlatKV version: FlatKV version mismatch: requested 41, reached 1",
    "Error: failed to load FlatKV version: FlatKV version mismatch: requested 41, reached 01",
    "FlatKV version mismatch: requested 41, reached 0",
    "Error: failed to load FlatKV version: FlatKV version mismatch: requested 41, reached 0\npanic: unrelated startup failure",
])
def test_partial_loss_attribution_rejects_unrelated_startup_failures(log):
    with pytest.raises(ValueError):
        checks_module().missing_state_error(log, 41)


def test_repair_record_maps_all_eight_findings_and_limits():
    text = RECORD.read_text()
    expected = {
        "retired-dex-contracts": "test_retired_dex_contracts_use_supported_product_operations",
        "fixture-self-oracle": "test_fixture_self_oracle_checks_embedded_runtime_and_slot_before_writing",
        "timelock-key-output": "test_timelock_key_output_is_isolated_disposable_and_public_only",
        "migration-height-boundary": "test_migration_height_boundary_covers_every_completion_and_buffer",
        "storage-dump-row-identity": "test_storage_dump_row_identity_uses_real_schema_and_receipt",
        "only-statesync-fallback": "test_only_statesync_restart_reopens_retained_height_and_restored_state",
        "only-statesync-stop-proof": "test_only_statesync_stop_proof_bounds_escalation_and_guards_wipe",
        "partial-loss-attribution": "test_partial_loss_attribution_accepts_exact_missing_state_and_version",
    }
    assert set(re.findall(r"^\[finding\.([^]]+)\]", text, re.M)) == set(expected)
    for finding, test in expected.items():
        section = text.split(f"[finding.{finding}]", 1)[1].split("\n[", 1)[0]
        assert test in section
        assert callable(globals()[test])
    for fact in ("review_only =", "baseline =", "evidence_ledger =", "[wiring.timelock_consumers]",
                 "[wiring.blocksync_scenario]", "[wiring.missing_state_producer]", "[gotcha.1]"):
        assert fact in text
