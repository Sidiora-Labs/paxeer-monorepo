"""Exercise the real checkpoint producer/consumer without an external toolchain.

SQLite stores are real persistent peer fixtures, not a claim about paxd's storage
format. Deployment-specific node/kernel semantics and age/SSH/lifecycle commands
are source-reviewed contracts; their runtime is explicitly outside this gate.
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
BACKUP = ROOT / "tools/ops/backup"
RECORD = Path(__file__).with_name("repair_22_31_424.kvx")


@pytest.fixture
def component():
    return runpy.run_path(str(BACKUP / "checkpoint.py"))


def create_store(path, height=125):
    with sqlite3.connect(path) as store:
        store.execute("CREATE TABLE committed (height INTEGER, root TEXT, balance INTEGER)")
        store.execute("INSERT INTO committed VALUES (?, ?, ?)", (height, "root-125", 730))
    # Closing with an uncommitted write must leave the committed state intact.
    store = sqlite3.connect(path)
    store.execute("UPDATE committed SET height = 126, balance = 0")
    store.close()


@pytest.fixture
def state(tmp_path):
    node, kernel = tmp_path / "node", tmp_path / "kernel"
    (node / "config").mkdir(parents=True)
    (node / "data").mkdir()
    kernel.mkdir()
    encoded = base64.b64encode(bytes(range(64))).decode()
    for name in ("priv_validator_key.json", "node_key.json"):
        key = node / "config" / name
        key.write_text(json.dumps({"priv_key": {"type": "tendermint/PrivKeyEd25519", "value": encoded}}))
        key.chmod(0o600)
    (node / "data/priv_validator_state.json").write_text('{"height":"125","round":0,"step":3}')
    kernel_key = kernel / "private.key"
    kernel_key.write_bytes(bytes(range(32)))
    kernel_key.chmod(0o600)
    create_store(node / "data/committed.db")
    create_store(kernel / "committed.db")
    return {
        "node": node,
        "kernel": kernel,
        "keys": [kernel_key],
        "sources": [node, kernel],
        "archive": tmp_path / "snapshot.tar.gz",
        "manifest": tmp_path / "manifest.json",
    }


def capture(component, state):
    return component["capture"](
        state["archive"], state["manifest"], state["sources"],
        state["node"], state["kernel"], state["keys"],
    )


def recovered(root, source):
    return root / str(source).lstrip("/")


def assert_reopened_state(root, state):
    for source in (state["node"] / "data/committed.db", state["kernel"] / "committed.db"):
        # mode=ro proves recovery by reopening the extracted database, without
        # manufacturing an empty database when the recovered file is missing.
        with sqlite3.connect(recovered(root, source).as_uri() + "?mode=ro", uri=True) as store:
            assert store.execute("PRAGMA integrity_check").fetchone() == ("ok",)
            assert store.execute("SELECT height, root, balance FROM committed").fetchall() == [(125, "root-125", 730)]
    signing = json.loads(recovered(root, state["node"] / "data/priv_validator_state.json").read_text())
    assert signing == {"height": "125", "round": 0, "step": 3}


def rewrite_image(source, target, *, drop=None, corrupt=None, extra=None):
    """Adversarially mutate an image emitted by the production producer."""
    with tarfile.open(source, "r:gz") as original, tarfile.open(target, "w:gz") as changed:
        for member in original.getmembers():
            if member.name == drop:
                continue
            if member.isfile():
                content = original.extractfile(member).read()
                if member.name == corrupt:
                    content = bytes([content[0] ^ 1]) + content[1:]
                changed.addfile(member, io.BytesIO(content))
            else:
                changed.addfile(member)
        if extra is not None:
            changed.addfile(extra, io.BytesIO(b"x") if extra.isfile() else None)


def test_backup_consistency_restarts_and_recovers_committed_state(component, state, tmp_path):
    manifest = capture(component, state)
    key_bytes = {name: (Path("/") / name).read_bytes() for name in manifest["required_keys"]}
    # Persist the actual producer's bytes in a filesystem store and discard all
    # source state. A new consumer namespace only has the stored recovery image.
    store = tmp_path / "store"
    store.mkdir()
    stored = store / "checkpoint.tar.gz"
    shutil.copyfile(state["archive"], stored)
    digest = hashlib.sha256(stored.read_bytes()).hexdigest()
    (store / "checkpoint.sha256").write_text(digest)
    shutil.rmtree(state["node"])
    shutil.rmtree(state["kernel"])
    del component, manifest
    reopened = runpy.run_path(str(BACKUP / "checkpoint.py"))
    assert hashlib.sha256(stored.read_bytes()).hexdigest() == (store / "checkpoint.sha256").read_text()
    destination = tmp_path / "reopened"
    expected = json.loads(state["manifest"].read_text())
    assert reopened["restore"](stored, destination, expected) == expected
    assert_reopened_state(destination, state)
    for name, content in key_bytes.items():
        key = destination / name
        assert key.read_bytes() == content
        if "key" in Path(name).name:
            assert key.stat().st_mode & 0o777 == 0o600
    assert destination.stat().st_mode & 0o777 == 0o700


@pytest.mark.parametrize("failure", ["node-root", "kernel-root", "missing-key", "empty-key", "empty-signing-state", "invalid-key"])
def test_backup_consistency_rejects_incomplete_capture(component, state, failure):
    if failure == "node-root":
        state["sources"] = [state["node"] / "config", state["kernel"]]
    elif failure == "kernel-root":
        state["sources"] = [state["node"]]
    elif failure == "missing-key":
        state["keys"][0].unlink()
    elif failure == "empty-key":
        state["keys"][0].write_bytes(b"")
    elif failure == "empty-signing-state":
        (state["node"] / "data/priv_validator_state.json").write_bytes(b"")
    else:
        (state["node"] / "config/node_key.json").write_text('{"priv_key":{}}')
    with pytest.raises(ValueError):
        capture(component, state)
    assert not state["archive"].exists()
    assert not state["archive"].with_name("snapshot.tar.gz.part").exists()


def test_backup_consistency_rejects_symlink_sources(component, state):
    (state["kernel"] / "linked-state").symlink_to(state["node"] / "data/committed.db")
    with pytest.raises(ValueError, match="symlink"):
        capture(component, state)
    assert not state["archive"].exists()


@pytest.mark.parametrize("damage", ["truncated", "missing-key", "changed-state", "wrong-manifest", "duplicate", "traversal", "symlink"])
def test_backup_consistency_bad_candidate_preserves_recoverable_checkpoint(component, state, tmp_path, damage):
    manifest = capture(component, state)
    previous = tmp_path / "previous.tar.gz"
    shutil.copyfile(state["archive"], previous)
    previous_bytes = previous.read_bytes()
    bad = tmp_path / "bad.tar.gz"
    expected = manifest
    if damage == "truncated":
        bad.write_bytes(previous_bytes[: len(previous_bytes) // 2])
    elif damage == "missing-key":
        rewrite_image(previous, bad, drop=manifest["required_keys"][0])
    elif damage == "changed-state":
        rewrite_image(previous, bad, corrupt=str(state["kernel"] / "committed.db").lstrip("/"))
    elif damage == "wrong-manifest":
        shutil.copyfile(previous, bad)
        expected = {**manifest, "version": 2}
    else:
        name = {"duplicate": component["MANIFEST"], "traversal": "../escape", "symlink": "linked"}[damage]
        extra = tarfile.TarInfo(name)
        extra.size = 1
        if damage == "symlink":
            extra.type, extra.linkname, extra.size = tarfile.SYMTYPE, "/tmp/escape", 0
        rewrite_image(previous, bad, extra=extra)
    destination = tmp_path / "bad-restore"
    with pytest.raises((ValueError, tarfile.TarError, EOFError)):
        component["restore"](bad, destination, expected)
    assert not destination.exists()
    assert previous.read_bytes() == previous_bytes
    good = tmp_path / "good-restore"
    component["restore"](previous, good, manifest)
    assert_reopened_state(good, state)


def test_backup_consistency_dump_is_in_same_verified_image(component, state, tmp_path):
    dump = tmp_path / "wallet.dump"
    dump.write_bytes(b"CREATE TABLE wallet (id INTEGER PRIMARY KEY);\n")
    manifest = component["capture"](
        state["archive"], state["manifest"], state["sources"],
        state["node"], state["kernel"], state["keys"], dump,
    )
    destination = tmp_path / "restore"
    component["restore"](state["archive"], destination, manifest)
    assert (destination / "db/wallet-db.dump").read_bytes() == dump.read_bytes()
    assert_reopened_state(destination, state)


def test_backup_consistency_refuses_existing_restore_destination(component, state, tmp_path):
    manifest = capture(component, state)
    destination = tmp_path / "existing"
    destination.mkdir()
    sentinel = destination / "live-state"
    sentinel.write_bytes(b"untouched")
    with pytest.raises(ValueError, match="must not exist"):
        component["restore"](state["archive"], destination, manifest)
    assert sentinel.read_bytes() == b"untouched"


def test_backup_consistency_shell_lifecycle_and_retention_guards():
    source = (BACKUP / "backup.sh").read_text()
    for required in (
        "BACKUP_NODE_HOME", "BACKUP_KERNEL_DIR", "BACKUP_REQUIRED_KEYS",
        "BACKUP_IDENTITY", "BACKUP_QUIESCE_CMD", "BACKUP_RESUME_CMD", "BACKUP_VALIDATE_CMD",
    ):
        assert re.search(r'\$\{' + required + r':\?', source)
    facts = [
        'flock -n 9 || die',
        'resume_needed=1\nsh -c "$BACKUP_QUIESCE_CMD" >&2 || die',
        'sh -c "$BACKUP_DB_DUMP_CMD" >"$stage/wallet-db.dump" || die',
        'python3 "$here/checkpoint.py" "${capture_args[@]}" || die',
        'sh -c "$BACKUP_RESUME_CMD" >&2 || die',
        'resume_needed=0\n\nname=',
        'age -R "$BACKUP_RECIPIENTS_FILE" "$stage/snapshot.tar.gz" >"$stage/archive.age" || die',
        'store "mkdir -p $dir && umask 077 && cat > $dir/$n.part" <"$stage/archive.age" || die',
        'store "cat $dir/$n.part" >"$stage/returned.age" || die',
        '[ "${actual%% *}" = "$expected" ] || die',
        'age -d -i "$BACKUP_IDENTITY" "$stage/returned.age" >"$stage/returned.tar.gz" || die',
        'python3 "$here/checkpoint.py" restore "$stage/returned.tar.gz" "$stage/recovered"',
        '--expected-manifest "$stage/manifest.json" || die',
        'BACKUP_RESTORE_ROOT="$stage/recovered" BACKUP_SNAPSHOT_MANIFEST="$stage/manifest.json"',
        'BACKUP_NODE_HOME="$stage/recovered$BACKUP_NODE_HOME"',
        'BACKUP_KERNEL_DIR="$stage/recovered$BACKUP_KERNEL_DIR"',
        'sh -c "$BACKUP_VALIDATE_CMD" >&2 || die',
        'mv -- $n.sha256.part $n.sha256 && mv -- $n.part $n" || die',
        'published=1',
        'head -n -$BACKUP_RETENTION',
    ]
    positions = [source.index(fact) for fact in facts]
    assert positions == sorted(positions)
    assert 'trap cleanup EXIT' in source
    assert 'if [ "$resume_needed" -eq 1 ]; then\n    sh -c "$BACKUP_RESUME_CMD" >&2 ||' in source
    assert 'if [ "$published" -eq 0 ]; then' in source
    assert source.count('head -n -$BACKUP_RETENTION') == 1
    assert 'rc[0]=0' not in source
    assert 'keeping archive' not in source
    assert 'set +e' not in source
    assert 'set -euo pipefail' in source
    assert 'for src in $BACKUP_SOURCES; do capture_args+=(--source "$src"); done' in source
    assert 'for key in $BACKUP_REQUIRED_KEYS; do capture_args+=(--required-key "$key"); done' in source
    # Lex the real consumer invocations and assert their exact API, without a
    # fake Bash, age, SSH or validator implementation in the Python gate.
    line = next(line for line in source.splitlines() if line.startswith('python3 "$here/checkpoint.py" restore'))
    assert shlex.split(line.removesuffix("\\")) == [
        "python3", "$here/checkpoint.py", "restore", "$stage/returned.tar.gz", "$stage/recovered",
    ]


def test_backup_consistency_public_restore_uses_verified_consumer():
    source = (BACKUP / "restore.sh").read_text()
    facts = [
        'sha256sum --quiet -c "$want.sha256") || die',
        'age -d -i "$BACKUP_IDENTITY" "$work/$want" >"$work/snapshot.tar.gz" || die',
        'python3 "$here/checkpoint.py" restore "$work/snapshot.tar.gz" "$dest" || die',
    ]
    assert [source.index(fact) for fact in facts] == sorted(source.index(fact) for fact in facts)
    assert 'tar -xzpf' not in source
    assert 'mkdir -p "$dest"' not in source
    helper = (BACKUP / "checkpoint.py").read_text()
    assert 'capture(' in helper and 'restore(args.archive, args.destination, expected)' in helper
    example = (BACKUP / "backup.env.example").read_text()
    assert 'BACKUP_SOURCES="/root/.paxd /var/lib/layerx/kernel"' in example
    for name in ("BACKUP_QUIESCE_CMD", "BACKUP_RESUME_CMD", "BACKUP_VALIDATE_CMD"):
        assert f"{name}=\n" in example
    assert "committed height/root agreement" in example


def test_backup_consistency_record_names_proofs_and_limits():
    record = tomllib.loads(RECORD.read_text())
    task = record["task"]["22"]["31"]["424"]
    assert task["finding"] == "backup-consistency"
    assert task["audit"] == "22.30.174"
    assert task["base_revision"] == "081dc08ced37d64df4d676e4b2a2d07ef444115a"
    tests = set(re.findall(r"(?m)^def (test_backup_consistency_\w+)\(", Path(__file__).read_text()))
    assert set(task["proofs"]) == tests
    assert "No pre-change verification" in task["baseline"]
    assert "hf-done" in task["evidence"]
    assert "SQLite" in task["runtime_proof"]
    assert "not executed" in task["review_only"]
    assert "domain 16" in task["wiring_requests"][0]
