"""Source contract and in-process production checkpoint proof for backup-test-contract.

The age-backed shell integration is inspected, not executed by this offline gate.
Only bash's syntax parser is invoked; checkpoint peers are committed file fixtures.
"""

import hashlib
import importlib.util
import json
from pathlib import Path
import re
import shutil
import subprocess
import threading
import time
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
BACKUP = ROOT / "tools/ops/backup"
RECORD = Path(__file__).with_name("repair_22_33_768.kvx")


def load_module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


checkpoint = load_module("repair_22_33_768_checkpoint", BACKUP / "checkpoint.py")
fixture = load_module("repair_22_33_768_fixture", BACKUP / "recovery_fixture.py")


def capture(root, archive, manifest):
    return checkpoint.capture(
        archive, manifest, [root / "validator", root / "kernel"],
        root / "validator", root / "kernel", [], root / "dump.sql",
    )


def recovered_root(destination, source):
    return destination / str(source).lstrip("/")


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def test_backup_test_contract_shell_syntax_and_real_tools():
    result = subprocess.run(
        ["bash", "-n", str(BACKUP / "test.sh")],
        capture_output=True, text=True, check=False,
    )
    assert result.returncode == 0, result.stderr
    shell = (BACKUP / "test.sh").read_text()
    assert "set -euo pipefail" in shell
    assert 'for tool in age age-keygen python3; do' in shell
    assert 'command -v "$tool" >/dev/null || fail' in shell
    assert 'age-keygen -o "$tmp/id.txt"' in shell
    assert 'age-keygen -y "$tmp/id.txt" >"$tmp/recipients.txt"' in shell
    assert not re.search(r"\b(?:fake|mock)\s*\(\)|go install|PATH=|exit 1 warning", shell)
    assert not re.search(r"(?:^|\n)warn=|fake tar|fake age", shell)
    environment = shell.split('cat >"$tmp/backup.env" <<ENV\n', 1)[1].split('\nENV', 1)[0]
    values = dict(re.findall(r"^([A-Z_]+)=(.*)$", environment, re.M))
    assert values["BACKUP_HOST"] == ""
    assert values["BACKUP_RETENTION"] == "3"
    assert values["BACKUP_NODE_HOME"] == "$fx/validator"
    assert values["BACKUP_KERNEL_DIR"] == "$fx/kernel"
    assert values["BACKUP_SOURCES"] == '"$fx/validator $fx/kernel"'
    assert values["BACKUP_LOCK_FILE"] == "$tmp/backup.lock"
    assert 'priv_validator_key.json' in values["BACKUP_REQUIRED_KEYS"]
    assert 'node_key.json' in values["BACKUP_REQUIRED_KEYS"]
    assert values["BACKUP_QUIESCE_CMD"] == '"mkdir $fx/.held"'
    assert values["BACKUP_RESUME_CMD"] == '"rmdir $fx/.held"'
    assert values["BACKUP_VALIDATE_CMD"] == (
        "'python3 \"$here/recovery_fixture.py\" validate "
        '"\\$BACKUP_NODE_HOME" "\\$BACKUP_KERNEL_DIR"\''
    )


def test_backup_test_contract_roundtrip_generations_and_retention():
    shell = (BACKUP / "test.sh").read_text()
    for command in (
        'name=$("$here/backup.sh" "$tmp/backup.env")',
        'assert_restore latest "$tmp/out" "$fx"',
        'for generation in 2 3 4 5; do',
        'python3 "$here/recovery_fixture.py" seed "$fx" "$generation"',
        'good=$("$here/backup.sh" "$tmp/backup.env")',
        'assert_restore "$good" "$tmp/out-$generation" "$fx"',
        '"$here/restore.sh" "$archive" "$destination" "$tmp/backup.env" >/dev/null',
        'diff -r "$expected/validator" "$destination$fx/validator" || fail',
        'diff -r "$expected/kernel" "$destination$fx/kernel" || fail',
        'cmp "$expected/dump.sql" "$destination/db/wallet-db.dump" || fail',
        'if grep -aq \'fixture-private\' "$tmp/store/$name"; then fail',
        '[ "$(find "$tmp/store" -name \'*.age\' | wc -l)" -eq 3 ] || fail',
        '[ "$(find "$tmp/store" -name \'*.sha256\' | wc -l)" -eq 3 ] || fail',
        '[ ! -e "$tmp/store/$name" ] || fail',
        '= 600 ] || fail "mode not preserved"',
    ):
        assert command in shell, command
    production = (BACKUP / "backup.sh").read_text()
    stages = [
        'sh -c "$BACKUP_QUIESCE_CMD"',
        'python3 "$here/checkpoint.py" "${capture_args[@]}" || die',
        'sh -c "$BACKUP_RESUME_CMD" >&2 || die',
        'age -R "$BACKUP_RECIPIENTS_FILE"',
        'cat > $dir/$n.part',
        'store "cat $dir/$n.part"',
        'age -d -i "$BACKUP_IDENTITY"',
        'python3 "$here/checkpoint.py" restore',
        'sh -c "$BACKUP_VALIDATE_CMD" >&2 || die',
        'mv -- $n.part $n',
        'head -n -$BACKUP_RETENTION',
    ]
    positions = [production.index(stage) for stage in stages]
    assert positions == sorted(positions)
    for guard in (
        '|| die "incomplete capture; nothing pruned"',
        '|| die "encryption failed"',
        '|| die "upload failed; nothing pruned"',
        '|| die "readback failed; nothing pruned"',
        '[ "${actual%% *}" = "$expected" ] || die "transport checksum mismatch; nothing pruned"',
        '|| die "decrypt failed; nothing pruned"',
        '|| die "isolated restore failed; nothing pruned"',
        '|| die "committed state/key validation failed; nothing pruned"',
        '|| die "publish failed; nothing pruned"',
    ):
        assert guard in production, guard
    restore = (BACKUP / "restore.sh").read_text()
    stages = ['sha256sum --quiet -c', 'age -d -i', 'python3 "$here/checkpoint.py" restore']
    positions = [restore.index(stage) for stage in stages]
    assert positions == sorted(positions)


def test_backup_test_contract_failed_capture_preserves_recoverable_bytes():
    shell = (BACKUP / "test.sh").read_text()
    for command in (
        'cp -a "$fx" "$tmp/known-good"',
        '(cd "$tmp/store" && sha256sum -- * | sort) >"$tmp/store-before"',
        'cmp "$tmp/store-before" "$tmp/store-after" || fail',
        '[ -f "$tmp/store/$good" ] && [ -f "$tmp/store/$good.sha256" ] || fail',
        '[ ! -e "$fx/.held" ] || fail "failed capture did not resume writers"',
        '[ -z "$(find "$tmp/store" -name \'*.part\')" ] || fail',
        'mv "$fx/validator/config/node_key.json" "$tmp/node-key.json"',
        "printf '%s\\n' 'invalid-age-recipient' >\"$tmp/recipients.txt\"",
        'fail "encryption with invalid recipients succeeded"',
        "grep -q 'encryption failed'",
        '[ ! -s "$tmp/failed-name" ] || fail "failed encryption returned an archive name"',
        'python3 "$here/recovery_fixture.py" churn "$fx/kernel/volume/changing.bin"',
        'fail "incomplete capture succeeded"',
        'fail "changing source capture succeeded"',
        'wait "$writer"',
        'assert_restore "$good" "$tmp/after-failure" "$tmp/known-good"',
    ):
        assert command in shell, command
    assert shell.count('if "$here/backup.sh" "$tmp/backup.env" >"$tmp/failed-name"') == 3
    assert shell.count("grep -q 'incomplete capture; nothing pruned'") == 2
    assert shell.count('[ ! -s "$tmp/failed-name" ] || fail') == 3
    assert shell.count('\nassert_unchanged\n') == 3
    assert shell.index('wait "$writer"', shell.index('writer=$!')) < shell.index(
        'assert_restore "$good" "$tmp/after-failure"'
    )


def test_backup_test_contract_tamper_rejected_before_output():
    shell = (BACKUP / "test.sh").read_text()
    for command in (
        'data[-1] ^= 1',
        'fail "tampered checksum restored"',
        "grep -q 'checksum mismatch'",
        '[ ! -e "$tmp/bad-checksum" ] || fail',
        '(cd "$tmp/store" && sha256sum "$good" >"$good.sha256")',
        'fail "tampered ciphertext restored with a matching transport checksum"',
        "grep -q 'archive does not decrypt'",
        '[ ! -e "$tmp/bad-age" ] || fail',
        'assert_restore latest "$tmp/final" "$tmp/known-good"',
    ):
        assert command in shell, command


def test_production_checkpoint_reopens_distinct_committed_generations(tmp_path):
    root = tmp_path / "fixture"
    facts = []
    for generation in (1, 2):
        fixture.seed(root, generation)
        archive = tmp_path / f"capture-{generation}.tar.gz"
        manifest_path = tmp_path / f"manifest-{generation}.json"
        manifest = capture(root, archive, manifest_path)
        assert json.loads(manifest_path.read_text()) == manifest
        # The actual producer's bytes cross a file-backed transport before reopening.
        transported = tmp_path / f"transported-{generation}.tar.gz"
        shutil.copyfile(archive, transported)
        assert digest(archive) == digest(transported)
        facts.append((manifest, transported))
    payload = str(root / "kernel/volume/state.bin").lstrip("/")
    assert facts[0][0]["entries"][payload]["sha256"] != facts[1][0]["entries"][payload]["sha256"]
    # Both writers are now at generation 2; reopening each closed image recovers its own state.
    for generation, (manifest, transported) in enumerate(facts, start=1):
        destination = tmp_path / f"restored-{generation}"
        assert checkpoint.restore(transported, destination, manifest) == manifest
        recovered = recovered_root(destination, root)
        assert fixture.validate(recovered / "validator", recovered / "kernel")["height"] == generation
        assert (destination / "db/wallet-db.dump").read_text() == f"INSERT INTO wallets VALUES ({generation});\n"
        assert (recovered / "validator/config/priv_validator_key.json").stat().st_mode & 0o777 == 0o600


@pytest.mark.parametrize("damage", ["missing-key", "empty-signing-state"])
def test_production_incomplete_capture_keeps_known_good_recovery(tmp_path, damage):
    root = tmp_path / "fixture"
    fixture.seed(root, 1)
    good = tmp_path / "known-good.tar.gz"
    manifest = capture(root, good, tmp_path / "good.json")
    before = digest(good)
    fixture.seed(root, 2)
    if damage == "missing-key":
        (root / "validator/config/node_key.json").unlink()
    else:
        (root / "validator/data/priv_validator_state.json").write_bytes(b"")
    failed = tmp_path / "failed.tar.gz"
    with pytest.raises(ValueError, match="required key or signing state missing/empty"):
        capture(root, failed, tmp_path / "failed.json")
    assert not failed.exists()
    assert not failed.with_name(failed.name + ".part").exists()
    assert not (tmp_path / "failed.json").exists()
    assert digest(good) == before
    destination = tmp_path / "reopened-after-failure"
    checkpoint.restore(good, destination, manifest)
    recovered = recovered_root(destination, root)
    assert fixture.validate(recovered / "validator", recovered / "kernel")["height"] == 1


def test_production_changing_source_rejected_and_good_image_reopens(tmp_path):
    root = tmp_path / "fixture"
    fixture.seed(root, 1)
    good = tmp_path / "known-good.tar.gz"
    manifest = capture(root, good, tmp_path / "good.json")
    before = digest(good)
    changing = root / "kernel/volume/changing.bin"
    changing.write_bytes(bytes(32 * 1024 * 1024))
    ready, stop = tmp_path / "ready", tmp_path / "stop"
    errors = []

    def write_source():
        try:
            fixture.churn(changing, ready, stop)
        except Exception as error:
            errors.append(error)

    writer = threading.Thread(target=write_source, daemon=True)
    writer.start()
    failed = tmp_path / "changing.tar.gz"
    try:
        deadline = time.monotonic() + 5
        while not ready.exists() and writer.is_alive() and time.monotonic() < deadline:
            time.sleep(0.001)
        assert ready.exists(), "real source writer did not start"
        with pytest.raises(ValueError, match="source.*changed"):
            capture(root, failed, tmp_path / "changing.json")
    finally:
        stop.touch()
        writer.join(timeout=5)
    assert not writer.is_alive()
    assert errors == []
    assert int.from_bytes(changing.read_bytes()[:8], "little") > 1
    assert not failed.exists()
    assert not failed.with_name(failed.name + ".part").exists()
    assert not (tmp_path / "changing.json").exists()
    assert digest(good) == before
    destination = tmp_path / "reopened-after-writer-stopped"
    checkpoint.restore(good, destination, manifest)
    recovered = recovered_root(destination, root)
    assert fixture.validate(recovered / "validator", recovered / "kernel")["height"] == 1
    assert not (recovered / "kernel/volume/changing.bin").exists()


def test_production_truncated_image_rejected_before_destination(tmp_path):
    root = tmp_path / "fixture"
    fixture.seed(root, 1)
    good = tmp_path / "good.tar.gz"
    capture(root, good, tmp_path / "good.json")
    broken = tmp_path / "truncated.tar.gz"
    broken.write_bytes(good.read_bytes()[:32])
    destination = tmp_path / "must-not-exist"
    with pytest.raises((EOFError, checkpoint.tarfile.ReadError)):
        checkpoint.restore(broken, destination)
    assert not destination.exists()


def test_backup_test_contract_record_maps_proof_and_limits():
    record = tomllib.loads(RECORD.read_text())
    finding = record["finding"]["backup-test-contract"]
    assert finding["audit"] == "22.30.174"
    assert finding["file"] == "tools/ops/backup/test.sh"
    assert finding["class"] == "test-gap"
    assert finding["severity"] == "medium"
    assert finding["tests"] == [
        "test_backup_test_contract_shell_syntax_and_real_tools",
        "test_backup_test_contract_roundtrip_generations_and_retention",
        "test_backup_test_contract_failed_capture_preserves_recoverable_bytes",
        "test_backup_test_contract_tamper_rejected_before_output",
        "test_production_checkpoint_reopens_distinct_committed_generations",
        "test_production_incomplete_capture_keeps_known_good_recovery",
        "test_production_changing_source_rejected_and_good_image_reopens",
        "test_production_truncated_image_rejected_before_destination",
    ]
    task = record["task"]["22"]["33"]["768"]
    assert task["wiring_requests"] == []
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "No unchanged-tree verification was run" in task["baseline"]
    assert "age-backed shell integration is not executed" in task["review_only"]
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_768.py"
