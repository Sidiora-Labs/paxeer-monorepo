"""Exercise the production journal; inspect the shell's external-tool boundaries.

No replacement cast/forge, credentials, endpoint, or compiler is used. Public
identities and resolved settings come from the committed Anvil dry-run recording.
Bash is used only as the explicitly required syntax parser.
"""

import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess

import pytest


ROOT = Path(__file__).resolve().parents[3]
DEPLOY = ROOT / "services/bridge/deploy"
SCRIPT = DEPLOY / "deploy-evm-chain.sh"
JOURNAL_SOURCE = DEPLOY / "evm-run-journal.py"
SPEC = importlib.util.spec_from_file_location("evm_run_journal", JOURNAL_SOURCE)
JOURNAL = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(JOURNAL)
RECORDING = json.loads((DEPLOY / "tests/fixtures/evm_dry_run.json").read_text())


def new_journal(tmp_path):
    return JOURNAL.RunJournal(tmp_path / ".ethereum.run", tmp_path / "ethereum.json", "ethereum")


def configuration():
    result = json.loads(json.dumps(RECORDING["configuration"]))
    result["deployer"] = RECORDING["record"]["deployer"]
    return result


def test_bridge_shell_syntax_and_guarded_broadcast():
    parsed = subprocess.run(["bash", "-n", str(SCRIPT)], capture_output=True, text=True)
    assert parsed.returncode == 0, parsed.stderr
    source = SCRIPT.read_text()
    derive = source.index('local_signer=$(cast wallet address --private-key "$deploy_key")')
    guard = source.index('python3 "$JOURNAL_TOOL" signer "$configured_deployer" "$local_signer"')
    prepare = source.index('action=$(journal_command prepare "$config" "$local_signer")')
    branch = source.index('if [ "$action" = broadcast ]; then')
    intent = source.index('journal_command begin || fail')
    broadcast = source.index('    forge script --root')
    resume = source.index('journal_command capture || fail')
    readback = source.index('deployed_code=$(cast code "$vault")')
    publish = source.index('journal_command publish "$work/record.json" || fail')
    assert derive < guard < prepare < branch < intent < broadcast < resume < readback < publish
    assert re.search(r'"\$local_signer" > /dev/null \\\n\s+\|\| fail', source)
    assert source.count('forge script --root') == 1
    assert 'FOUNDRY_BROADCAST="$journal/broadcast"' in source
    assert '> "$journal/deploy.log" 2>&1' in source
    assert 'flock -n 9 || fail' in source
    assert source.index('flock -n 9 || fail') < source.index('config=$(journal_command config)')
    assert 'if [ "$preflight_only" -eq 0 ] && [ -e "$journal" ]; then' in source
    assert source.index('config=$(journal_command config)') < source.index('\n    apply_overlay\n')
    traps = re.findall(r'^trap (.+)$', source, re.MULTILINE)
    assert traps and all('$journal' not in trap and '$record' not in trap for trap in traps)
    assert '> "$work/record.json"' in source
    assert '> "$record"' not in source
    # Retain the post-broadcast configured-deployer check as a second provenance guard.
    assert '"$(lower "$deployer")" != "$(lower "$configured_deployer")"' in source
    assert '"$(lower "$deployer")" = "$(lower "$local_signer")"' in source
    for check in ('[ "$deployed_hash" = "$built_hash" ]',
                  '[ "$deployed_threshold" = "$threshold" ]',
                  '[ "$(lower "$deployed_attestors")" = "$expected_attestors" ]',
                  '[ "${reported[0]}" = "${asset_per_tx[index]}" ]',
                  '[ "${reported[1]}" = "${asset_total[index]}" ]'):
        assert source.index(check) < publish


def test_bridge_deployer_mismatch_refused_before_broadcast(tmp_path):
    config = configuration()
    wrong = config["owner"]
    assert wrong.lower() != config["deployer"].lower()
    with pytest.raises(ValueError, match="refusing before broadcast"):
        new_journal(tmp_path).prepare(config, wrong)
    assert list(tmp_path.iterdir()) == []
    signer = config["deployer"]
    assert JOURNAL.require_signer(signer.upper().replace("0X", "0x"), signer) == signer.lower()
    assert JOURNAL.require_signer("", signer) == signer.lower()
    for invalid in ("", "0x" + "0" * 40, "not-a-signer"):
        with pytest.raises(ValueError):
            JOURNAL.require_signer(signer, invalid)


def test_bridge_broadcast_recovery_reopens_same_vault(tmp_path):
    config = configuration()
    journal = new_journal(tmp_path)
    assert journal.prepare(config, config["deployer"]) == "broadcast"
    journal.begin_broadcast()
    # This is the production identity checkpoint called by capture, with the
    # actual vault address from the committed deployment recording.
    journal.retain_vault(RECORDING["record"]["vault"])
    persisted = journal.path.read_bytes()
    del journal
    recovered = new_journal(tmp_path)
    assert recovered.path.read_bytes() == persisted
    assert recovered.load()["vault"] == RECORDING["record"]["vault"].lower()
    assert recovered.prepare(config, config["deployer"]) == "readback"
    with pytest.raises(ValueError, match="already attempted broadcast"):
        recovered.begin_broadcast()
    with pytest.raises(ValueError, match="changed the retained vault"):
        recovered.retain_vault(config["owner"])
    with pytest.raises(ValueError, match="broadcast uncertain"):
        recovered.capture()
    assert not recovered.record.exists()
    assert recovered.load()["vault"] == RECORDING["record"]["vault"].lower()
    assert recovered.prepare(config, config["deployer"]) == "readback"


def test_bridge_unrecorded_broadcast_never_redeploys_after_restart(tmp_path):
    config = configuration()
    journal = new_journal(tmp_path)
    journal.prepare(config, config["deployer"])
    journal.begin_broadcast()
    del journal
    recovered = new_journal(tmp_path)
    assert recovered.load()["phase"] == "broadcasting"
    assert recovered.prepare(config, config["deployer"]) == "readback"
    with pytest.raises(ValueError, match="broadcast uncertain"):
        recovered.capture()
    with pytest.raises(ValueError, match="not qualified for readback"):
        recovered.publish(RECORDING["record"])
    assert not recovered.record.exists()


def test_bridge_config_provenance_survives_overlay_removal(tmp_path):
    overlay = tmp_path / "temporary-overlay"
    overlay.mkdir()
    config = configuration()
    # Unknown extension fields must never be copied into the retained snapshot.
    config["credentials"] = {"token": "DO-NOT-RETAIN"}
    config["environment"]["password"] = "DO-NOT-RETAIN"
    config["assets"][0]["credential"] = "DO-NOT-RETAIN"
    resolved = overlay / "config.json"
    resolved.write_text(json.dumps(config))
    journal = new_journal(tmp_path)
    journal.prepare(json.loads(resolved.read_text()), config["deployer"])
    expected_digest = hashlib.sha256(journal.snapshot.read_bytes()).hexdigest()
    resolved.unlink()
    overlay.rmdir()
    del journal
    recovered = new_journal(tmp_path)
    state = recovered.load()
    snapshot = json.loads(recovered.snapshot.read_text())
    assert Path(state["configuration"]) == recovered.snapshot
    assert state["configuration_sha256"] == expected_digest
    assert recovered.snapshot.parent.parent == recovered.record.parent
    assert recovered.directory.stat().st_mode & 0o777 == 0o700
    assert recovered.snapshot.stat().st_mode & 0o777 == 0o600
    assert recovered.path.stat().st_mode & 0o777 == 0o600
    assert b"DO-NOT-RETAIN" not in recovered.snapshot.read_bytes()
    for key in ("owner", "deployer", "attestors", "threshold", "chain_id", "native", "finality_depth"):
        assert snapshot[key] == config[key]
    assert snapshot["assets"] == JOURNAL.resolved_configuration(config)["assets"]
    assert snapshot["environment"] == RECORDING["configuration"]["environment"]
    assert recovered.prepare(snapshot, config["deployer"]) == "broadcast"
    changed = configuration()
    changed["owner"] = config["deployer"]
    with pytest.raises(ValueError, match="configuration differs"):
        recovered.prepare(changed, config["deployer"])
    recovered.snapshot.write_text("{}")
    with pytest.raises(ValueError, match="digest changed"):
        recovered.load()


def test_bridge_snapshot_rejects_credentials_in_environment_values(tmp_path):
    config = configuration()
    config["environment"]["rpc_url"] = "https://user:credential@rpc.invalid"
    with pytest.raises(ValueError, match="variable names only"):
        new_journal(tmp_path).prepare(config, config["deployer"])
    assert not new_journal(tmp_path).directory.exists()


def test_bridge_journal_refuses_cross_chain_record_and_signer_drift(tmp_path):
    config = configuration()
    journal = new_journal(tmp_path)
    journal.prepare(config, config["deployer"])
    with pytest.raises(ValueError, match="another chain"):
        JOURNAL.RunJournal(journal.directory, journal.record, "base").load()
    with pytest.raises(ValueError, match="another chain"):
        JOURNAL.RunJournal(journal.directory, tmp_path / "other.json", "ethereum").load()
    config["deployer"] = config["owner"]
    with pytest.raises(ValueError, match="signer differs from the retained"):
        journal.prepare(config, config["owner"])


def test_bridge_atomic_publication_and_retained_source_contract(tmp_path):
    target = tmp_path / "record.json"
    # Exercise the same atomic writer used for checkpoints and final publication
    # against the real recorded deployment, then reopen it from the filesystem.
    JOURNAL.atomic_json(target, RECORDING["record"])
    assert json.loads(target.read_text()) == RECORDING["record"]
    assert target.stat().st_mode & 0o777 == 0o600
    JOURNAL.atomic_json(target, RECORDING["configuration"])
    assert json.loads(target.read_text()) == RECORDING["configuration"]
    assert list(tmp_path.iterdir()) == [target]
    source = JOURNAL_SOURCE.read_text()
    capture = source[source.index('    def capture(self):'):source.index('    def publish(self, candidate):')]
    for fact in ('broadcast["transactions"]', 'broadcast["receipts"]',
                 'self.retain_vault(vault)', 'transactions=transactions', 'receipts=receipts',
                 'receipt["transactionHash"]', 'receipt["status"]', 'creates[0]["hash"]',
                 'state["signer"]', 'retained submitted transaction'):
        assert fact in capture
    assert capture.index('atomic_json(self.path, state)') < capture.index('broadcast receipts are incomplete')
    publish = source[source.index('    def publish(self, candidate):'):source.index('\ndef main(args):')]
    assert 'configuration_sha256=state["configuration_sha256"]' in publish
    assert 'configuration=str(self.snapshot)' in publish
    assert publish.index('actual != expected') < publish.index('atomic_json(self.record, result)')
    assert publish.index('atomic_json(self.record, result)') < publish.index('state["phase"] = "complete"')
    writer = source[source.index('def atomic_json('):source.index('\ndef resolved_configuration(')]
    assert writer.index('os.fsync(output.fileno())') < writer.index('os.replace(temporary, path)')
    assert writer.index('os.replace(temporary, path)') < writer.index('sync_directory(path.parent)')
