"""Offline production helpers, source contracts, real files and loopback peers.

The only subprocess is the explicitly required bash syntax parser. No paxd,
Ignite, statik, Go compiler or downloaded binary is built or executed here.
"""

import ast
import contextlib
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import shlex
import sqlite3
import stat
import subprocess
import tarfile
import tempfile
import threading
import sys
import tomllib

import pytest

ROOT = Path(__file__).resolve().parents[3]
SCRIPTS = ROOT / "scripts"
RECORD = Path(__file__).with_name("repair_22_33_803.kvx")
EXPORT = ROOT / "node/testdata/v6120-halt-export.json"
BANK = ROOT / "sdk/contrib/rosetta/configuration/testdata/bank-success.json"
SENDER = "cosmos1w537ue6nmewqqdcqfhcn0jsntg8qjpcs4wf75w"
RECIPIENT = "cosmos19g9cm8ymzchq2qkcdv3zgqtwayj9asv3hjv5u5"


def load(name):
    spec = importlib.util.spec_from_file_location(name, SCRIPTS / f"{name}.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


snapshot = load("snapshot_cycle")
faucet = load("faucet_launcher")
swagger = load("swagger_assets")


def source(name):
    return (SCRIPTS / name).read_text()


def function_source(module, name):
    tree = ast.parse(Path(module.__file__).read_text())
    function = next(n for n in tree.body if isinstance(n, ast.FunctionDef) and n.name == name)
    return ast.get_source_segment(Path(module.__file__).read_text(), function)


def test_shell_syntax_and_entrypoints_use_production_helpers():
    for name in ("run_snapshot.sh", "start_faucet.sh", "update-swagger-ui-statik.sh"):
        result = subprocess.run(["bash", "-n", str(SCRIPTS / name)], capture_output=True, text=True, check=False)
        assert result.returncode == 0, result.stderr
    assert "set -euo pipefail" in source("run_snapshot.sh")
    assert "set -euo pipefail" in source("start_faucet.sh")
    assert shlex.split(source("start_faucet.sh").splitlines()[-1]) == [
        "exec", "python3", "$SCRIPT_DIR/faucet_launcher.py", "$@",
    ]
    assert "set -eu" in source("update-swagger-ui-statik.sh")


def test_finding_22_waits_for_clean_halt_before_export_snapshot_and_restart():
    text = source("run_snapshot.sh")
    commands = [
        'flock -n 9',
        '"$CONTROLLER" stop "$PAX_DIR"',
        '"$CONTROLLER" check-stopped "$PAX_DIR"',
        'while true; do',
        '"$PAXD_BIN" start --trace --home "$PAX_DIR" --chain-id "$CHAIN_ID"',
        'PAXD_PID=$!',
        'if wait "$PAXD_PID"; then',
        "grep -Fq 'halting node per configuration'",
        '"$PAXD_BIN" export --home "$PAX_DIR" --height -1 --streaming=false',
        'snapshot_cycle.py" committed',
        '"$PAXD_BIN" snapshot --height "$HALT_HEIGHT" --home "$PAX_DIR"',
        'HALT_HEIGHT=$(python3 "$SCRIPT_DIR/snapshot_cycle.py" publish',
    ]
    offsets = [text.index(command) for command in commands]
    assert offsets == sorted(offsets)
    assert '"$CONTROLLER" check-stopped "$PAX_DIR"' in text[offsets[7]:offsets[8]]
    assert '--snapshot-dir "$STAGING/snapshot"' in text
    assert 'kill -TERM "$PAXD_PID"' in text and 'trap cleanup EXIT' in text
    assert 'trap \'exit 130\' INT' in text and 'trap \'exit 143\' TERM' in text
    assert 'sleep 10' not in text and 'tendermint snapshot' not in text
    assert text.count('"$PAXD_BIN" start ') == 1
    native = (ROOT / "daemon/paxd/cmd/snapshot.go").read_text()
    assert 'cmd.Flags().Int64("height"' in native
    assert 'cmd.Flags().String("snapshot-dir"' in native
    assert '_, err = snapshotManager.Create(uint64(height))' in native
    assert 'return fmt.Errorf("failed to create snapshot: %w", err)' in native
    commit = (ROOT / "sdk/baseapp/abci.go").read_text()
    assert commit.index('app.cms.Commit(true)') < commit.index('if halt {')
    assert 'halting node per configuration' in commit
    assert 'height := app.LastBlockHeight() + 1' in (ROOT / "node/export.go").read_text()


def test_finding_22_committed_export_requires_exact_height_chain_and_state(tmp_path):
    value = json.loads(EXPORT.read_text())
    height, chain = int(value["initial_height"]) - 1, value["chain_id"]
    assert snapshot.committed(EXPORT, height, chain) == value
    for bad_height, bad_chain in ((height - 1, chain), (height + 1, chain), (height, "another-chain")):
        with pytest.raises(ValueError):
            snapshot.committed(EXPORT, bad_height, bad_chain)
    value["app_state"] = {}
    path = tmp_path / "empty-export.json"
    path.write_text(json.dumps(value))
    with pytest.raises(ValueError, match="missing application state"):
        snapshot.committed(path, height, chain)


def test_finding_22_configuration_preserves_unrelated_native_toml(tmp_path):
    path = tmp_path / "app.toml"
    path.write_text('''pruning = "default"
pruning-keep-recent = "0"
pruning-keep-every = "0"
pruning-interval = "10"
halt-height = 0
minimum-gas-prices = "0uhpx"
[state-sync]
snapshot-interval = 0
''')
    snapshot.configure(path, "20")
    config = tomllib.loads(path.read_text())
    assert config == {
        "pruning": "custom", "pruning-keep-recent": "1", "pruning-keep-every": "0",
        "pruning-interval": "1", "halt-height": 20, "minimum-gas-prices": "0uhpx",
        "state-sync": {"snapshot-interval": 0},
    }
    path.write_text(path.read_text().replace('pruning-keep-every = "0"\n', ''))
    before = path.read_bytes()
    with pytest.raises(ValueError, match="missing configuration key"):
        snapshot.configure(path, "30")
    assert path.read_bytes() == before


def stage_snapshot(tmp_path):
    """A durable SQLite storage peer, never a replacement paxd executable.

    The publication layer consumes opaque chunks; this peer produces a real
    SQLite backup and metadata using the native directory layout. It proves
    publication/reopen integrity, not native snapshot encoding or consensus.
    """
    value = json.loads(EXPORT.read_text())
    height, chain = int(value["initial_height"]) - 1, value["chain_id"]
    stage = Path(tempfile.mkdtemp(dir=tmp_path, prefix=".snapshot-"))
    (stage / "committed.json").write_bytes(EXPORT.read_bytes())
    chunks = stage / "snapshot" / str(height) / "1"
    chunks.mkdir(parents=True)
    with contextlib.closing(sqlite3.connect(":memory:")) as store:
        store.execute("create table committed(height integer, chain text)")
        store.execute("insert into committed values (?, ?)", (height, chain))
        store.commit()
        with contextlib.closing(sqlite3.connect(chunks / "0")) as backup:
            store.backup(backup)
    metadata = stage / "snapshot" / "metadata.db"
    metadata.mkdir()
    with contextlib.closing(sqlite3.connect(metadata / "manifest.sqlite")) as db:
        db.execute("create table snapshots(height integer, format integer, chunks integer)")
        db.execute("insert into snapshots values (?, 1, 1)", (height,))
        db.commit()
    return stage, height, chain


def test_finding_22_snapshot_publication_recovers_next_height_after_reopen(tmp_path):
    stage, height, chain = stage_snapshot(tmp_path)
    assert snapshot.resume(tmp_path, height, 10, chain) == height
    assert snapshot.publish(stage, tmp_path, height, 10, chain) == height + 10
    assert not stage.exists()
    reopened = load("snapshot_cycle")
    assert reopened.resume(tmp_path, height, 10, chain) == height + 10
    published = tmp_path / str(height)
    receipt = json.loads((published / "receipt.json").read_text())
    assert receipt["files"] == reopened.verify_snapshot(published, height)
    with contextlib.closing(sqlite3.connect(published / "snapshot" / str(height) / "1/0")) as backup:
        assert backup.execute("select height, chain from committed").fetchone() == (height, chain)
    with pytest.raises(ValueError, match="interval differs"):
        reopened.resume(tmp_path, height, 11, chain)
    with (published / "snapshot" / str(height) / "1/0").open("ab") as stream:
        stream.write(b"corruption")
    with pytest.raises(ValueError, match="integrity"):
        reopened.resume(tmp_path, height, 10, chain)


@pytest.mark.parametrize("damage", ["missing_metadata", "missing_chunk", "gap", "empty", "symlink"])
def test_finding_22_incomplete_snapshot_never_advances(tmp_path, damage):
    stage, height, chain = stage_snapshot(tmp_path)
    chunk = stage / "snapshot" / str(height) / "1/0"
    if damage == "missing_metadata":
        (stage / "snapshot/metadata.db/manifest.sqlite").unlink()
    elif damage == "missing_chunk":
        chunk.unlink()
    elif damage == "gap":
        chunk.rename(chunk.with_name("1"))
    elif damage == "empty":
        chunk.write_bytes(b"")
    else:
        chunk.rename(chunk.with_name("original"))
        chunk.symlink_to(chunk.with_name("original"))
    with pytest.raises(ValueError):
        snapshot.publish(stage, tmp_path, height, 10, chain)
    assert not (tmp_path / str(height)).exists()
    assert snapshot.resume(tmp_path, height, 10, chain) == height


def test_finding_23_release_pin_platform_selection_and_guard_order():
    assert faucet.RELEASE == "0.0.3"
    expected = {
        ("Linux", "x86_64"): "02c815c127939a4996bcdb6c7a543cc099907e82426b427b5a03de9fe34270ab",
        ("Linux", "aarch64"): "490c04909a1cfc64ceca2a0d1bae8d3ff620581359fcb80fccdf8b2129e82976",
        ("Darwin", "x86_64"): "7337fbcba05e9db93f97a565d03629137bbb763857a530fabe48fa330628d124",
        ("Darwin", "arm64"): "562f0620b5f2edf1d2aba53b5bbece99db04285ff8335804bf03b25b3e1b78af",
    }
    assert faucet.RELEASE_SHA256 == expected
    for (system, machine), digest in expected.items():
        url, actual = faucet.release_artifact(system, machine)
        assert url.startswith("https://github.com/ignite/faucet/releases/download/v0.0.3/faucet_0.0.3_")
        assert actual == digest
    with pytest.raises(ValueError, match="unsupported"):
        faucet.release_artifact("Linux", "unknown")
    text = function_source(faucet, "launch")
    guards = ['private_directory(staging)', 'download(url, archive)', 'verify_archive(archive, digest, staging / "faucet")',
              'owned_file(executable, executable=True)', 'subprocess.Popen(command']
    assert [text.index(g) for g in guards] == sorted(text.index(g) for g in guards)
    download = function_source(faucet, "download")
    assert 'response.status == 200' in download
    assert 'scheme == "https"' in download
    assert 'os.O_EXCL | os.O_NOFOLLOW' in download
    assert 'total <= MAX_ARTIFACT' in download
    assert 'hashlib.file_digest(stream, "sha256").hexdigest() == digest' in function_source(faucet, "verify_archive")
    assert 'finally:\n            stop_child(child)' in text
    shell = source("start_faucet.sh")
    assert all(retired not in shell for retired in ('sudo', 'password', '/tmp/faucet_install.sh', 'get.starport.network', 'newd'))


def artifact(tmp_path, members=None):
    directory = tmp_path / "private"
    directory.mkdir(mode=0o700)
    archive = directory / "release.tar.gz"
    # Inert bytes exercise authentication/extraction only and are never executed.
    with tarfile.open(archive, "w:gz") as bundle:
        for name, payload, kind in members or [("faucet", b"authentication fixture bytes", tarfile.REGTYPE)]:
            member = tarfile.TarInfo(name)
            member.type = kind
            if kind == tarfile.REGTYPE:
                member.size = len(payload)
                bundle.addfile(member, io.BytesIO(payload))
            else:
                member.linkname = "outside"
                bundle.addfile(member)
    archive.chmod(0o600)
    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    return directory, archive, digest


def test_finding_23_verifies_digest_before_extracting_private_owned_file(tmp_path):
    directory, archive, digest = artifact(tmp_path)
    output = directory / "faucet"
    with pytest.raises(ValueError, match="digest mismatch"):
        faucet.verify_archive(archive, "0" * 64, output)
    assert not output.exists()
    assert faucet.verify_archive(archive, digest, output) == output
    assert output.read_bytes() == b"authentication fixture bytes"
    assert stat.S_IMODE(output.stat().st_mode) == 0o500
    faucet.owned_file(output, executable=True)
    output.chmod(0o700)
    with pytest.raises(ValueError, match="permissions"):
        faucet.owned_file(output, executable=True)
    directory.chmod(0o755)
    with pytest.raises(ValueError, match="0700"):
        faucet.private_directory(directory)


@pytest.mark.parametrize("members", [
    [("faucet", b"", tarfile.SYMTYPE)],
    [("../faucet", b"inert", tarfile.REGTYPE)],
    [("/faucet", b"inert", tarfile.REGTYPE)],
    [("faucet", b"a", tarfile.REGTYPE), ("faucet", b"b", tarfile.REGTYPE)],
    [("README", b"missing executable", tarfile.REGTYPE)],
])
def test_finding_23_rejects_unsafe_or_incomplete_archives(tmp_path, members):
    directory, archive, digest = artifact(tmp_path, members)
    with pytest.raises(ValueError):
        faucet.verify_archive(archive, digest, directory / "verified")
    assert not (directory / "verified").exists()


def test_finding_23_rejects_linked_and_shared_downloads(tmp_path):
    directory, archive, digest = artifact(tmp_path)
    os.link(archive, directory / "hardlink")
    with pytest.raises(ValueError, match="ownership or type"):
        faucet.verify_archive(archive, digest, directory / "verified")
    (directory / "hardlink").unlink()
    archive.rename(directory / "original")
    archive.symlink_to(directory / "original")
    with pytest.raises(ValueError, match="ownership or type"):
        faucet.verify_archive(archive, digest, directory / "verified")


def test_finding_24_resolves_one_binary_home_chain_and_native_query_contract(tmp_path):
    home = tmp_path / "chain home"
    (home / "config").mkdir(parents=True)
    (home / "config/genesis.json").write_text('{"chain_id":"disposable-chain"}')
    client = home / "config/client.toml"
    client.write_text('chain-id = "disposable-chain"\nnode = "tcp://127.0.0.1:26657"\nkeyring-backend = "test"\n')
    # Only resolve the gate interpreter; no chain executable is required or run.
    config = faucet.local_chain(home, sys.executable)
    assert Path(config["binary"]).is_absolute()
    assert config["home"] == str(home.resolve()) and config["chain"] == "disposable-chain"
    args = faucet.faucet_command("/private/faucet", config, "admin", "uhpx", 100, 1, 8000)
    flags = dict(zip(args[1::2], args[2::2]))
    assert flags == {
        "--cli-name": config["binary"], "--home": config["home"], "--node": config["node"],
        "--account-name": "admin", "--keyring-backend": "test", "--sdk-version": "stargate",
        "--denoms": "uhpx", "--credit-amount": "100", "--max-credit": "1000",
        "--fee-amount": "1", "--port": "8000", "--mnemonic": "", "--keyring-password": "",
    }
    query = faucet.query_command(config, "query", "tx", "F" * 64)
    assert query == [config["binary"], "query", "tx", "F" * 64, "--home", str(home.resolve()),
                     "--node", "tcp://127.0.0.1:26657", "--chain-id", "disposable-chain", "--output", "json"]
    native = (ROOT / "sdk/x/auth/client/cli/query.go").read_text()
    assert 'Use:   "txs"' in native and 'Use:   "tx --type=' in native
    assert 'cmd.Flags().String(flagEvents' in native
    text = function_source(faucet, "launch")
    assert 'require(args.disposable' in text
    assert 'status["node_info"]["network"] == config["chain"]' in text
    assert text.index('request_funding(endpoint') < text.index('config, "query", "txs"') < text.index('config, "query", "tx", hashes[0]')
    assert text.index('funding_transaction(confirmed') < text.index('print(f"Faucet ready')
    client.write_text(client.read_text().replace("disposable-chain", "wrong-chain"))
    with pytest.raises(ValueError, match="chain IDs differ"):
        faucet.local_chain(home, sys.executable)


def test_finding_24_real_request_transport_and_committed_response_consumer():
    captured = []
    recorded = json.loads(BANK.read_text())

    class Peer(BaseHTTPRequestHandler):
        def do_POST(self):
            value = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            captured.append(value)
            # The peer consumes the real launcher's request; transaction evidence
            # is a committed native SDK transcript, not a fabricated response.
            assert value == {"address": RECIPIENT, "coins": ["100stake"]}
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b"{}")

        def do_GET(self):
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(BANK.read_bytes())

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Peer)
    thread = threading.Thread(target=server.serve_forever)
    thread.start()
    try:
        endpoint = f"http://127.0.0.1:{server.server_port}"
        faucet.request_funding(endpoint, RECIPIENT, "stake", 100)
        response = faucet.http_json(endpoint + "/recorded-transaction")
        assert faucet.funding_transaction(response, SENDER, RECIPIENT, "stake", 100, 2) == recorded["txhash"]
        assert captured == [{"address": RECIPIENT, "coins": ["100stake"]}]
    finally:
        server.shutdown()
        thread.join()
        server.server_close()


@pytest.mark.parametrize("change", ["failed", "old", "hash", "sender", "recipient", "denom", "amount"])
def test_finding_24_rejects_uncommitted_failed_or_unrelated_funding(change):
    value = json.loads(BANK.read_text())
    sender, recipient, denom, amount, after = SENDER, RECIPIENT, "stake", 100, 2
    if change == "failed":
        value["code"] = 5
    elif change == "old":
        after = 3
    elif change == "hash":
        value["txhash"] = "unconfirmed"
    elif change == "sender":
        sender = RECIPIENT
    elif change == "recipient":
        recipient = SENDER
    elif change == "denom":
        denom = "uhpx"
    else:
        amount = 101
    with pytest.raises(ValueError):
        faucet.funding_transaction(value, sender, recipient, denom, amount, after)


SHARED_HEADER = "// Code generated from the shared Swagger archive. See tests/ci/repair_22_39_62.kvx."
STATIK_HEADER = "// Code generated by statik. DO NOT EDIT."


def raw_statik():
    # Preserve the real embedded archive while reconstructing statik's input to
    # the legacy publisher. The shared archive now serves both namespaces.
    actual = (ROOT / "docs/swagger/statik.go").read_text()
    assert actual.startswith(SHARED_HEADER + "\n")
    assert actual.count('\tfs.Register(data)\n') == 1
    assert actual.count('\tfs.RegisterWithNamespace(swaggerNamespace, data)\n') == 1
    return (actual.replace(SHARED_HEADER, STATIK_HEADER, 1)
            .replace('const swaggerNamespace = "swagger"\n\n', '')
            .replace('\tfs.RegisterWithNamespace(swaggerNamespace, data)\n', ''))


def test_finding_25_portable_namespace_publishes_the_real_generated_payload(tmp_path):
    raw = raw_statik()
    generated, destination = tmp_path / "generated.go", tmp_path / "statik.go"
    generated.write_text(raw)
    destination.write_text("previous artifact")
    swagger.publish(generated, destination)
    expected = ((ROOT / "docs/swagger/statik.go").read_text()
                .replace(SHARED_HEADER, STATIK_HEADER, 1)
                .replace("\tfs.Register(data)\n", "").encode())
    assert destination.read_bytes() == expected
    assert destination.stat().st_mode & 0o777 == 0o644
    assert destination.read_text().count('const swaggerNamespace = "swagger"') == 1
    assert destination.read_text().count('fs.RegisterWithNamespace(swaggerNamespace, data)') == 1
    assert re.search(r'(?m)^\tdata := .*$', raw)[0] == re.search(r'(?m)^\tdata := .*$', destination.read_text())[0]
    # Supported previous statik output is checked and migrated too.
    legacy = raw.replace('fs.Register(data)', 'fs.RegisterWithNamespace("swagger", data)')
    assert swagger.namespace(legacy).encode() == expected


@pytest.mark.parametrize("damage", ["missing", "duplicate", "wrong_namespace", "wrong_package", "truncated"])
def test_finding_25_invalid_generation_preserves_previous_artifact(tmp_path, damage):
    raw = raw_statik()
    if damage == "missing":
        raw = raw.replace('fs.Register(data)', '')
    elif damage == "duplicate":
        raw = raw.replace('fs.Register(data)', 'fs.Register(data)\n\tfs.Register(data)')
    elif damage == "wrong_namespace":
        raw = raw.replace('fs.Register(data)', 'fs.RegisterWithNamespace("wrong", data)')
    elif damage == "wrong_package":
        raw = raw.replace('package swagger', 'package wrong')
    else:
        raw = raw[:100]
    generated, destination = tmp_path / "generated.go", tmp_path / "statik.go"
    generated.write_text(raw)
    destination.write_bytes(b"previous good artifact")
    with pytest.raises(ValueError):
        swagger.publish(generated, destination)
    assert destination.read_bytes() == b"previous good artifact"
    assert not list(tmp_path.glob(".statik-*"))


def test_finding_25_generation_failures_precede_atomic_publish():
    shell = source("update-swagger-ui-statik.sh")
    commands = ["set -eu", 'STAGING=$(mktemp -d ./docs/.swagger-statik.XXXXXXXX)',
                "ignite generate openapi -y", 'statik -src=./docs/swagger-ui -dest="$STAGING" -f -p swagger',
                'python3 "$SCRIPT_DIR/swagger_assets.py" "$STAGING/swagger/statik.go" docs/swagger/statik.go']
    assert [shell.index(c) for c in commands] == sorted(shell.index(c) for c in commands)
    assert 'sed -i' not in shell and '|| true' not in shell
    assert 'trap \'rm -rf -- "$STAGING"\' EXIT' in shell
    text = function_source(swagger, "publish")
    assert text.index('namespace(Path(source).read_text())') < text.index('tempfile.mkstemp')
    assert text.index('os.fsync') < text.index('os.replace')
    consumer = (ROOT / "node/app.go").read_text()
    assert '_ "github.com/Sidiora-Labs/Paxeer-X-Network/docs/swagger"' in consumer
    assert 'statikFS, err := fs.NewWithNamespace("swagger")' in consumer
    assert 'http.FileServer(statikFS)' in consumer
    assert 'rtr.PathPrefix("/swagger/")' in consumer
    assert 'github.com/rakyll/statik/fs' in (ROOT / "docs/swagger/statik.go").read_text()


def test_record_maps_every_finding_to_declared_proofs_and_records_limits():
    record = tomllib.loads(RECORD.read_text())
    declared = {n.name for n in ast.parse(Path(__file__).read_text()).body if isinstance(n, ast.FunctionDef)}
    assert set(record["finding"]) == {"22", "23", "24", "25"}
    for finding, value in record["finding"].items():
        assert value["tests"] and set(value["tests"]) <= declared
        assert any(name.startswith(f"test_finding_{finding}_") for name in value["tests"])
        assert value["repair"] and value["review_only"]
    task = record["task"]["22"]["33"]["803"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_803.py"
    assert "No baseline failures are claimed" in task["baseline"]
    assert len(record["wiring"]["requests"]) == 2
