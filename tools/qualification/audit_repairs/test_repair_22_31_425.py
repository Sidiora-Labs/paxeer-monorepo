"""Prove keyring byte recovery in-process and the actual shell command guards.

SQLite is a persistent storage peer fixture, not a replacement paxd backend.
The real backup producer emits the image consumed by the migration recovery code.
The declared build gate parses Bash; no external binaries run inside this gate.
"""

import base64
import hashlib
import io
import json
from pathlib import Path
import re
import runpy
import shlex
import shutil
import sqlite3
import tarfile
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
OPS = ROOT / "tools/ops"
SCRIPT = OPS / "keyring-migrate.sh"
RECORD = Path(__file__).with_name("repair_22_31_425.kvx")
ROWS = [
    ("alice", "address-alice", b"private-alice"),
    ("bob", "address-bob", b"private-bob"),
]


@pytest.fixture
def component():
    return runpy.run_path(str(OPS / "keyring_recovery.py"))


@pytest.fixture
def recovery_image(tmp_path, component):
    home, kernel = tmp_path / "node", tmp_path / "kernel"
    (home / "config").mkdir(parents=True)
    (home / "data").mkdir()
    keyring = home / "keyring-test"
    keyring.mkdir()
    kernel.mkdir()
    encoded = base64.b64encode(bytes(range(64))).decode()
    for name in ("node_key.json", "priv_validator_key.json"):
        path = home / "config" / name
        path.write_text(json.dumps({"priv_key": {"value": encoded}}))
        path.chmod(0o600)
    (home / "data/priv_validator_state.json").write_text('{"height":"125"}')
    database = keyring / "keys.db"
    with sqlite3.connect(database) as store:
        store.execute("CREATE TABLE keys (name TEXT PRIMARY KEY, address TEXT, private BLOB)")
        store.executemany("INSERT INTO keys VALUES (?, ?, ?)", ROWS)
    # Stop the peer after an uncommitted mutation, so the recovery must reopen
    # committed bytes rather than retain an in-memory connection or state.
    store = sqlite3.connect(database)
    store.execute("UPDATE keys SET private = X'00' WHERE name = 'alice'")
    store.close()
    database.chmod(0o600)
    manifest = component["snapshot"](keyring)
    archive = tmp_path / "snapshot.tar.gz"
    producer = runpy.run_path(str(OPS / "backup/checkpoint.py"))
    producer["capture"](
        archive, tmp_path / "backup.json", [home, kernel], home, kernel, [database]
    )
    return home, archive, manifest


def read_keys(database):
    with sqlite3.connect(database.as_uri() + "?mode=ro", uri=True) as store:
        assert store.execute("PRAGMA integrity_check").fetchall() == [("ok",)]
        return store.execute("SELECT name, address, private FROM keys ORDER BY name").fetchall()


def rewrite_image(source, destination, *, corrupt=None, drop=None, extra=None):
    with tarfile.open(source, "r:gz") as original, tarfile.open(destination, "w:gz") as changed:
        for member in original.getmembers():
            if member.name == drop:
                continue
            if member.isfile():
                content = original.extractfile(member).read()
                if member.name == corrupt:
                    content = content[: len(content) // 2]
                    member.size = len(content)
                changed.addfile(member, io.BytesIO(content))
            else:
                changed.addfile(member)
        if extra is not None:
            changed.addfile(extra, io.BytesIO(b"x") if extra.isfile() else None)


def test_key_backup_content_restarts_and_recovers_original_keys(component, recovery_image, tmp_path):
    home, archive, manifest = recovery_image
    before = (home / "keyring-test/keys.db").read_bytes()
    transport = tmp_path / "store"
    transport.mkdir()
    stored = transport / "recovery.tar.gz"
    shutil.copyfile(archive, stored)
    checksum = hashlib.sha256(stored.read_bytes()).hexdigest()
    (transport / "recovery.sha256").write_text(checksum)
    shutil.rmtree(home)
    del component
    # A fresh component only has the producer's persisted image and the source
    # manifest; no live database or old Python namespace can supply the keys.
    reopened = runpy.run_path(str(OPS / "keyring_recovery.py"))
    expected = json.loads(json.dumps(manifest))
    assert hashlib.sha256(stored.read_bytes()).hexdigest() == (transport / "recovery.sha256").read_text()
    destination = tmp_path / "private-recovered-home"
    keyring = reopened["restore_keyring"](stored, home, destination, expected)
    assert reopened["snapshot"](keyring) == manifest
    assert (keyring / "keys.db").read_bytes() == before
    assert read_keys(keyring / "keys.db") == ROWS
    assert not (destination / "config").exists()
    assert destination.stat().st_mode & 0o777 == 0o700
    assert keyring.stat().st_mode & 0o777 == 0o700
    assert (keyring / "keys.db").stat().st_mode & 0o777 == 0o600
    # Close and reopen once more; both addresses and private contents persist.
    assert read_keys(keyring / "keys.db") == ROWS


@pytest.mark.parametrize("damage", [
    "content", "missing", "extra-key", "truncated", "duplicate", "traversal",
    "absolute", "symlink", "hardlink", "fifo",
])
def test_key_backup_content_rejects_damaged_archive_preserving_source(component, recovery_image, tmp_path, damage):
    home, archive, manifest = recovery_image
    source = home / "keyring-test"
    before = component["snapshot"](source)
    key_name = str(source / "keys.db").lstrip("/")
    bad = tmp_path / "bad.tar.gz"
    if damage == "content":
        rewrite_image(archive, bad, corrupt=key_name)
        with tarfile.open(archive) as good, tarfile.open(bad) as changed:
            assert good.getnames() == changed.getnames()
    elif damage == "missing":
        rewrite_image(archive, bad, drop=key_name)
    elif damage == "truncated":
        bad.write_bytes(archive.read_bytes()[: len(archive.read_bytes()) // 2])
    else:
        name = {
            "extra-key": str(source / "unexpected.key").lstrip("/"),
            "duplicate": key_name,
            "traversal": "../escape",
            "absolute": "/escape",
            "symlink": "linked",
            "hardlink": "hardlinked",
            "fifo": "pipe",
        }[damage]
        extra = tarfile.TarInfo(name)
        extra.size = 1
        if damage in ("symlink", "hardlink", "fifo"):
            extra.type = {
                "symlink": tarfile.SYMTYPE,
                "hardlink": tarfile.LNKTYPE,
                "fifo": tarfile.FIFOTYPE,
            }[damage]
            extra.linkname, extra.size = key_name, 0
        rewrite_image(archive, bad, extra=extra)
    destination = tmp_path / "rejected-home"
    with pytest.raises((ValueError, tarfile.TarError, EOFError)):
        component["restore_keyring"](bad, home, destination, manifest)
    assert not destination.exists()
    assert component["snapshot"](source) == before
    assert read_keys(source / "keys.db") == ROWS
    # The same unchanged source manifest still recovers the real producer image.
    recovered = component["restore_keyring"](archive, home, tmp_path / "good-home", manifest)
    assert read_keys(recovered / "keys.db") == ROWS


@pytest.mark.parametrize("change", ["bytes", "add", "delete", "directory", "symlink"])
def test_key_backup_content_detects_source_changes(component, recovery_image, change):
    home, archive, manifest = recovery_image
    source = home / "keyring-test"
    if change == "bytes":
        (source / "keys.db").write_bytes(b"changed")
    elif change == "add":
        (source / "new.key").write_bytes(b"new")
    elif change == "delete":
        (source / "keys.db").unlink()
    elif change == "directory":
        (source / "new-directory").mkdir()
    else:
        (source / "linked").symlink_to(source / "keys.db")
    with pytest.raises(ValueError):
        component["check_source"](source, manifest)
    assert source.exists()


def test_key_backup_content_refuses_existing_home(component, recovery_image, tmp_path):
    home, archive, manifest = recovery_image
    destination = tmp_path / "existing"
    destination.mkdir()
    sentinel = destination / "live.key"
    sentinel.write_bytes(b"untouched")
    with pytest.raises(ValueError, match="must not exist"):
        component["restore_keyring"](archive, home, destination, manifest)
    assert sentinel.read_bytes() == b"untouched"


def test_key_backup_content_positively_empty_keyring(component, recovery_image, tmp_path):
    home, archive, manifest = recovery_image
    (home / "keyring-test/keys.db").unlink()
    empty = component["snapshot"](home / "keyring-test")
    assert empty == {".": {"kind": "directory"}}
    producer = runpy.run_path(str(OPS / "backup/checkpoint.py"))
    empty_archive = tmp_path / "empty.tar.gz"
    producer["capture"](
        empty_archive, tmp_path / "empty.json", [home, tmp_path / "kernel"],
        home, tmp_path / "kernel", [],
    )
    recovered = component["restore_keyring"](empty_archive, home, tmp_path / "empty-home", empty)
    assert component["snapshot"](recovered) == empty
    assert component["enumerate_keys"]([]) == ""
    assert list(recovered.iterdir()) == []


def test_key_enumeration_distinguishes_empty_and_recovers_addresses(component, recovery_image, tmp_path):
    assert component["enumerate_keys"]([]) == ""
    home, archive, manifest = recovery_image
    recovered = component["restore_keyring"](archive, home, tmp_path / "enumerated", manifest)
    rows = read_keys(recovered / "keys.db")
    peer_output = [{"name": name, "address": address} for name, address, private in rows]
    assert component["enumerate_keys"](peer_output[::-1]) == "alice address-alice\nbob address-bob"
    assert [private for name, address, private in rows] == [b"private-alice", b"private-bob"]


@pytest.mark.parametrize("value", [
    None, {}, "", [None], [{}], [{"name": "alice"}],
    [{"name": "", "address": "a"}],
    [{"name": "alice", "address": ""}],
    [{"name": "alice bob", "address": "a"}],
    [{"name": "alice", "address": "a\nb"}],
    [{"name": "-flag", "address": "a"}],
    [{"name": "a\x00", "address": "a"}],
    [{"name": "alice", "address": "a"}, {"name": "alice", "address": "a"}],
])
def test_key_enumeration_rejects_unreadable_or_ambiguous_results(component, value):
    with pytest.raises(ValueError):
        component["enumerate_keys"](value)


def section(source, start, end):
    return source[source.index(start):source.index(end, source.index(start))]


def assert_order(source, facts):
    positions = [source.index(fact) for fact in facts]
    assert positions == sorted(positions)


def test_key_enumeration_shell_preserves_source_on_every_failed_prerequisite():
    source = SCRIPT.read_text()
    assert "set -euo pipefail\numask 077" in source
    keys = section(source, "keys() {", "# recover_keys")
    assert 'keys list --home "$1" --keyring-backend "$2" --output json' in keys
    assert 'python3 "$here/keyring_recovery.py" keys' in keys
    assert "||" not in keys
    main = source[source.index('failures=0'):]
    assert_order(main, [
        'snapshot "$test_dir" >"$scratch/source.json" || {',
        'want=$(keys "$home" test) || {\n\t\tkept source-enumeration\n\t\tcontinue',
        'n=$(grep -c . <<<"$want" || true)',
        'have=$(keys "$home" file) || {\n\t\tkept passphrase\n\t\tcontinue',
        'stop=conflict',
        'keys export "$name" --home "$home" --keyring-backend test >"$scratch/key.armor" 2>/dev/null || {\n\t\t\tstop=export',
        'keys import "$name" "$scratch/key.armor" --home "$home" --keyring-backend file >/dev/null 2>&1 || {\n\t\t\tstop=import',
        '[ -z "$stop" ] || {\n\t\tkept "$stop"\n\t\tcontinue',
        'recover_keys "$home" file "$want" || {\n\t\tkept file-recovery\n\t\tcontinue',
        'archive=$(verify_backup "$home" "$scratch/source.json" "$want" 2>"$err") || {',
        'kept in-use\n\t\tcontinue',
        'current=$(keys "$home" test) || {\n\t\tkept source-enumeration\n\t\tcontinue',
        '[ "$current" = "$want" ] && python3 "$here/keyring_recovery.py" check "$test_dir" "$scratch/source.json" || {\n\t\tkept source-changed\n\t\tcontinue',
        'rm -rf -- "$test_dir"',
    ])
    assert source.count('rm -rf -- "$test_dir"') == 1
    assert '|| want=""' not in source and '|| have=""' not in source
    assert '<(printf' not in source and 'comm -23' not in source
    assert re.search(r'kept\(\) \{.*failures=\$\(\(failures \+ 1\)\)', main, re.S)
    assert main.rstrip().endswith('[ "$failures" -eq 0 ]')


def test_key_enumeration_destination_reopens_private_material_and_address():
    recovery = section(SCRIPT.read_text(), "recover_keys() (", "# verify_backup")
    assert_order(recovery, [
        'listed=$(keys "$1" "$2") || exit 1',
        '[ "$2" = test ] && [ "$listed" != "$3" ]',
        'proof=$(mktemp -d) || exit 1',
        '[ "$current" = "$addr" ] || exit 1',
        'keys show "$name" --address --home "$1" --keyring-backend "$2"',
        'keys export "$name" --home "$1" --keyring-backend "$2" >"$proof/key.armor" 2>/dev/null || exit 1',
        '[ -s "$proof/key.armor" ] || exit 1',
        'keys import "$name" "$proof/key.armor" --home "$proof/home" --keyring-backend file >/dev/null 2>&1 || exit 1',
        'keys show "$name" --address --home "$proof/home" --keyring-backend file',
        'rm -f -- "$proof/key.armor" || exit 1',
        'done <<<"$3"',
    ])
    assert recovery.count('[ "$address" = "$addr" ] || exit 1') == 2
    assert recovery.count('2>/dev/null) || exit 1') == 2
    assert "trap 'rm -rf -- \"$proof\"' EXIT" in recovery
    assert '|| true' not in recovery


def test_key_backup_content_shell_restores_bytes_then_actual_backend():
    source = SCRIPT.read_text()
    backup = section(source, "verify_backup() (", "# in_use")
    assert_order(backup, [
        'load_env "" || die',
        'work=$(mktemp -d) || die',
        'store "cat $dir/$(q "$name")" >"$work/$name" || die',
        'store "cat $dir/$(q "$name").sha256" >"$work/$name.sha256" || die',
        'sha256sum --quiet -c "$name.sha256" >/dev/null 2>&1) || die "checksum"',
        '[ "$written" -ge "${changed%.*}" ] || die "stale"',
        'age -d -i "$BACKUP_IDENTITY" "$work/$name" >"$work/snapshot.tar.gz" 2>/dev/null || die "decrypt"',
        'python3 "$here/keyring_recovery.py" restore "$work/snapshot.tar.gz" "$1" "$work/recovered" "$2" || die "content"',
        'recover_keys "$work/recovered" test "$3" || die "key-recovery"',
        'echo "$name"',
    ])
    assert 'tar -tzf' not in source
    assert 'trap \'rm -rf "$work"\' EXIT' in backup
    command = next(line.strip() for line in backup.splitlines() if line.strip().startswith('python3 '))
    assert shlex.split(command) == [
        'python3', '$here/keyring_recovery.py', 'restore', '$work/snapshot.tar.gz', '$1', '$work/recovered', '$2', '||', 'die', 'content'
    ]


def test_key_recovery_record_maps_findings_and_limits():
    record = tomllib.loads(RECORD.read_text())
    task = record["task"]["22"]["31"]["425"]
    assert task["audit"] == "22.30.174"
    assert task["base_revision"] == "264615cba89456c37eeb19535cb16b81846f6a68"
    proofs = record["finding"]
    assert set(proofs) == {"key-enumeration", "key-backup-content"}
    declared = set(re.findall(r"(?m)^def (test_\w+)\(", Path(__file__).read_text()))
    for finding in proofs.values():
        assert set(finding["tests"]) <= declared
        assert finding["tests"]
    assert "SQLite" in task["runtime_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification" in task["baseline"]
    assert "hf-done" in task["evidence"]
    assert task["wiring_requests"] == []
