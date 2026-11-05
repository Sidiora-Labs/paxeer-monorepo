"""Production Solana recovery, using committed RPC accounts over loopback HTTP.

The shell's external writers are checked as guarded source commands. No Solana,
Rust, Node, funded signer or live endpoint is needed by this gate.
"""

import base64
from contextlib import contextmanager
from copy import deepcopy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import subprocess
import threading
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
DEPLOY = ROOT / "services/bridge/deploy"
SCRIPT = DEPLOY / "deploy-solana-program.sh"
HELPER = DEPLOY / "solana-run-journal.py"
FIXTURES = DEPLOY / "tests/fixtures"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_485.kvx"
SPEC = importlib.util.spec_from_file_location("solana_run_journal", HELPER)
RECOVERY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RECOVERY)


def configuration():
    return json.loads((FIXTURES / "chains/solana/config.json").read_text())


def recorded_deployment():
    return json.loads((FIXTURES / "records/solana.json").read_text())


class RecordedAccountStore:
    """Serve the committed account readbacks, including a selected durable prefix."""

    def __init__(self):
        transcript = json.loads((FIXTURES / "rpc/solana.json").read_text())
        entries = transcript["endpoints"]["solana"]["*"]
        self.genesis = entries[0]["result"]
        self.accounts = {
            entry["params"][0]: deepcopy(entry["result"]["value"])
            for entry in entries if entry["method"] == "getAccountInfo"
        }
        self.addresses = list(self.accounts)[1:]
        self.present = set()
        self.slot = entries[1]["result"]["context"]["slot"]
        self.requests = []
        self.error = False

    def answer(self, request):
        self.requests.append(request)
        if self.error:
            return {"jsonrpc": "2.0", "id": request["id"], "error": {"code": -32000}}
        if request["method"] == "getGenesisHash":
            result = self.genesis
        elif request["method"] == "getMultipleAccounts":
            result = {
                "context": {"slot": self.slot},
                "value": [self.accounts[key] if key in self.present else None
                          for key in request["params"][0]],
            }
        else:
            raise AssertionError("unexpected RPC method")
        return {"jsonrpc": "2.0", "id": request["id"], "result": result}


@contextmanager
def rpc_peer(store):
    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            response = json.dumps(store.answer(request)).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(response)))
            self.end_headers()
            self.wfile.write(response)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield RECOVERY.Rpc(f"http://127.0.0.1:{server.server_port}")
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


def prepared_journal(tmp_path, deployed=True):
    journal = RECOVERY.RunJournal(tmp_path / "deployment.journal.json")
    record = recorded_deployment()
    assert journal.prepare(
        configuration(), record["program_id"], record["genesis_hash"],
        record["publisher"], record["program_elf_sha256"], record["program_elf_bytes"],
    ) == "deploy"
    assert journal.load()["phase"] == "deploy_intent"
    assert journal.load()["intent"] == {
        "kind": "deploy", "program_id": record["program_id"],
        "elf_sha256": record["program_elf_sha256"],
    }
    if deployed:
        journal.deployed({"programId": record["program_id"], "signature": record["deployment_signature"]})
    return journal


def test_wrong_keypair_refused_before_broadcast(capsys):
    vector = json.loads((FIXTURES / "solana-deploy/vault-authority.json").read_text())
    # These are distinct, valid public keys from committed deployment vectors.
    with pytest.raises(ValueError, match="refusing before broadcast"):
        RECOVERY.main(["identity", vector["program_id"], vector["vault_authority"]])
    assert capsys.readouterr().out == ""
    assert RECOVERY.require_program_identity(vector["program_id"], vector["program_id"]) == vector["program_id"]
    assert RECOVERY.require_program_identity("PLACEHOLDER:first-deploy", vector["program_id"]) == vector["program_id"]
    for invalid in ("", "not-a-key", "11111111111111111111111111111111"):
        with pytest.raises(ValueError):
            RECOVERY.require_program_identity(vector["program_id"], invalid)


def test_shell_parsed_and_every_writer_guarded():
    result = subprocess.run(["bash", "-n", str(SCRIPT)], capture_output=True, text=True)
    assert result.returncode == 0, result.stderr
    source = SCRIPT.read_text()
    assert 'KEYGEN="$toolchain/solana-keygen"' in source
    derive = 'supplied_program_id=$("$KEYGEN" pubkey "$program_keypair")'
    identity = 'python3 "$JOURNAL" identity "$configured_program_id" "$supplied_program_id"'
    assert source.count(derive) == 2
    assert source.count(identity) == 2
    first_derive, first_guard = source.index(derive), source.index(identity)
    preflight = source.index('if [ "$preflight_only" -eq 1 ]; then')
    prepare = source.index('deployment_action=$(python3 "$JOURNAL" prepare')
    broadcast = source.index('"${deploy[@]}" >')
    assert first_derive < first_guard < preflight < prepare < broadcast
    assert source.index(identity, first_guard + len(identity)) < prepare
    assert '|| fail "the supplied program keypair does not identify the configured program"' in source
    assert source.index('flock -n "$journal_lock" || fail') < prepare
    assert 'case $deployment_action in\ndeploy)' in source
    assert source.index('python3 "$JOURNAL" deployed') > broadcast
    resume = source.split('resume)\n', 1)[1].split(';;', 1)[0]
    assert 'python3 "$JOURNAL" deployment' in resume and '"${deploy[@]}"' not in resume
    planner = source.index('action=$(python3 "$JOURNAL" next "$journal" "$config" "$rpc" "$rooted_slot")')
    initialise = source.index('"$admin" initialise --config "$config"')
    register = source.index('"$admin" register-asset --config "$config"')
    assert planner < initialise < register
    assert source.count('"$admin" initialise ') == source.count('"$admin" register-asset ') == 1
    loop = source[planner:source.index('\nregistered=$work/assets.json')]
    assert '|| fail "custody reconciliation refused further writes"' in loop
    assert 'case $action in\n    ready) break ;;\n    initialise)' in loop
    assert 'index=$action' in loop
    assert '--program-id "$program_id"' in loop
    assert '--mint "${asset_mints[index]}" --asset-id "${asset_ids[index]}"' in loop
    assert '--decimals "${asset_decimals[index]}" --per-tx-cap "${asset_per_tx[index]}"' in loop
    assert '--total-cap "${asset_total[index]}"' in loop
    assert 'python3 "$JOURNAL" publish "$journal" "$record" "$work/record.json" deployed' in source
    assert 'python3 "$JOURNAL" publish "$journal" "$record" "$work/record.json" ready' in source
    assert '> "$record"' not in source


def test_pdas_and_layout_match_committed_program_readbacks():
    store = RecordedAccountStore()
    config = configuration()
    program = config["solana"]["program_id"]
    address, bump = RECOVERY.derive_address(program, [b"config"])
    assert address == store.addresses[0]
    raw = base64.b64decode(store.accounts[address]["data"][0])
    assert len(raw) == 1367 and raw[:10] == b"PXBRCFG0\0\1"
    assert raw[77] == bump
    vault = json.loads((FIXTURES / "solana-deploy/vault-authority.json").read_text())
    assert RECOVERY.derive_address(program, [b"vault-authority"]) == (vault["vault_authority"], vault["bump"])
    for asset, address in zip(config["assets"], store.addresses[1:], strict=True):
        derived, bump = RECOVERY.derive_address(program, [b"asset", RECOVERY.decode_key(asset["address"])])
        assert derived == address
        raw = base64.b64decode(store.accounts[address]["data"][0])
        assert len(raw) == 89 and raw[:10] == b"PXBRAST0\0\1" and raw[64] == bump
    rust = (ROOT / "contracts/bridge/solana/src/state.rs").read_text()
    for fact in ('b"PXBRCFG0"', 'b"PXBRAST0"', 'CONFIG_ATTESTORS_OFFSET: usize = 87',
                 'ASSET_BYTES: usize = 89', 'bytes[77] = self.config_bump',
                 'bytes[78] = self.vault_bump', 'bytes[65..73]', 'bytes[73..81]'):
        assert fact in rust


def test_restart_after_confirmed_initialise_and_each_asset(tmp_path):
    config = configuration()
    journal = prepared_journal(tmp_path)
    store = RecordedAccountStore()
    with rpc_peer(store) as rpc:
        assert journal.reconcile(config, rpc) == "initialise"
        assert journal.load()["intent"]["kind"] == "initialise"
        assert journal.load()["intent"]["owner"] == config["owner"]
        path = journal.path
        del journal  # Stop with only the pre-broadcast checkpoint persisted.
        store.present.add(store.addresses[0])  # Recorded finalized initialization.
        journal = RECOVERY.RunJournal(path)
        assert journal.load()["phase"] == "initialise_intent"
        assert journal.reconcile(config, rpc) == "0"
        assert journal.load()["intent"]["mint"] == config["assets"][0]["address"]
        del journal
        store.present.add(store.addresses[1])  # First asset confirmed before stop.
        journal = RECOVERY.RunJournal(path)
        assert journal.reconcile(config, rpc) == "1"
        assert journal.load()["intent"]["mint"] == config["assets"][1]["address"]
        del journal
        store.present.add(store.addresses[2])
        journal = RECOVERY.RunJournal(path)
        assert journal.reconcile(config, rpc) == "ready"
        assert journal.load()["intent"] is None
        assert journal.load()["phase"] == "ready"
        assert RECOVERY.RunJournal(path).reconcile(config, rpc) == "ready"
    queries = [x for x in store.requests if x["method"] == "getMultipleAccounts"]
    assert len(queries) == 5
    assert queries[0]["params"] == [store.addresses, {
        "encoding": "base64", "commitment": "finalized", "minContextSlot": 0,
    }]
    assert all(x["params"][1]["minContextSlot"] == store.slot for x in queries[1:])
    assert path.stat().st_mode & 0o777 == 0o600
    assert not list(tmp_path.glob(".deployment.journal.json.*"))
    record_path = tmp_path / "record.json"
    RECOVERY.RunJournal(path).publish(record_path, recorded_deployment(), "ready")
    published = json.loads(record_path.read_text())
    assert published["version"] == 1 and published["phase"] == "ready"
    assert published["assets"] == recorded_deployment()["assets"]
    assert record_path.stat().st_mode & 0o777 == 0o600


def test_resume_preexisting_accounts_without_reinitialising(tmp_path):
    journal = prepared_journal(tmp_path)
    store = RecordedAccountStore()
    store.present.update(store.addresses[:2])
    with rpc_peer(store) as rpc:
        assert journal.reconcile(configuration(), rpc) == "1"
    assert journal.load()["intent"]["kind"] == "register-asset"


@pytest.mark.parametrize("stage", ["initialise", "asset"])
def test_unresolved_intent_never_authorizes_a_duplicate_write(tmp_path, stage):
    journal = prepared_journal(tmp_path)
    store = RecordedAccountStore()
    if stage == "asset":
        store.present.add(store.addresses[0])
    with rpc_peer(store) as rpc:
        assert journal.reconcile(configuration(), rpc) == ("0" if stage == "asset" else "initialise")
        before = journal.path.read_bytes()
        with pytest.raises(ValueError, match="intent is unresolved"):
            RECOVERY.RunJournal(journal.path).reconcile(configuration(), rpc)
        assert journal.path.read_bytes() == before
    assert journal.load()["phase"] != "ready"


@pytest.mark.parametrize("field", ["program", "genesis", "publisher", "elf", "owner", "caps"])
def test_resume_binds_every_write_to_original_inputs(tmp_path, field):
    journal = prepared_journal(tmp_path)
    config, record = configuration(), recorded_deployment()
    args = [record["program_id"], record["genesis_hash"], record["publisher"],
            record["program_elf_sha256"], record["program_elf_bytes"]]
    if field == "program":
        args[0] = record["publisher"]
    elif field == "genesis":
        args[1] = record["publisher"]
    elif field == "publisher":
        args[2] = record["program_id"]
    elif field == "elf":
        args[3] = "0" * 64
    elif field == "owner":
        config["owner"] = record["publisher"]
    else:
        config["assets"][0]["total_cap"] = "1"
    before = journal.path.read_bytes()
    with pytest.raises(ValueError):
        journal.prepare(config, *args)
    assert journal.path.read_bytes() == before


def test_deployment_resume_uses_retained_response_and_ambiguous_deploy_stops(tmp_path):
    journal = prepared_journal(tmp_path, deployed=False)
    record = recorded_deployment()
    args = [configuration(), record["program_id"], record["genesis_hash"],
            record["publisher"], record["program_elf_sha256"], record["program_elf_bytes"]]
    with pytest.raises(ValueError, match="deployment intent is unresolved"):
        RECOVERY.RunJournal(journal.path).prepare(*args)
    with pytest.raises(ValueError, match="before broadcast"):
        journal.deployed({"programId": record["publisher"], "signature": record["deployment_signature"]})
    assert journal.load()["phase"] == "deploy_intent"
    journal.deployed({"programId": record["program_id"], "signature": record["deployment_signature"]})
    reopened = RECOVERY.RunJournal(journal.path)
    assert reopened.prepare(*args) == "resume"
    assert reopened.load()["deployment"]["signature"] == record["deployment_signature"]
    config = configuration()
    config["solana"]["program_id"] = "PLACEHOLDER:first-deploy"
    args[0] = config
    assert reopened.prepare(*args) == "resume"


@pytest.mark.parametrize("account,offset", [
    (0, 0), (0, 8), (0, 10), (0, 42), (0, 74), (0, 75), (0, 76), (0, 77), (0, 78), (0, 87),
    (1, 0), (1, 8), (1, 10), (1, 42), (1, 62), (1, 63), (1, 64), (1, 65), (1, 73),
])
def test_conflicting_config_or_asset_stops_before_any_new_intent(tmp_path, account, offset):
    journal = prepared_journal(tmp_path)
    store = RecordedAccountStore()
    store.present.update(store.addresses)
    stored = store.accounts[store.addresses[account]]
    raw = bytearray(base64.b64decode(stored["data"][0]))
    raw[offset] ^= 1
    stored["data"][0] = base64.b64encode(raw).decode()
    before = journal.path.read_bytes()
    with rpc_peer(store) as rpc, pytest.raises(ValueError, match="existing custody"):
        journal.reconcile(configuration(), rpc)
    assert journal.path.read_bytes() == before


@pytest.mark.parametrize("fault", ["rpc-error", "wrong-genesis", "foreign-owner", "encoding", "executable", "stale", "before-deployment-root"])
def test_failed_or_foreign_readback_cannot_advance_phase(tmp_path, fault):
    journal = prepared_journal(tmp_path)
    store = RecordedAccountStore()
    store.present.update(store.addresses)
    if fault == "rpc-error":
        store.error = True
    elif fault == "wrong-genesis":
        store.genesis = "different-genesis"
    elif fault == "foreign-owner":
        store.accounts[store.addresses[0]]["owner"] = recorded_deployment()["publisher"]
    elif fault == "encoding":
        store.accounts[store.addresses[1]]["data"][1] = "jsonParsed"
    elif fault == "executable":
        store.accounts[store.addresses[0]]["executable"] = True
    elif fault == "stale":
        state = journal.load()
        state["observed_slot"] = store.slot + 1
        RECOVERY.atomic_json(journal.path, state)
    before = journal.path.read_bytes()
    with rpc_peer(store) as rpc, pytest.raises(ValueError):
        journal.reconcile(configuration(), rpc, store.slot + 1 if fault == "before-deployment-root" else 0)
    assert journal.path.read_bytes() == before


def test_all_accounts_are_checked_before_issuing_missing_asset_intent(tmp_path):
    journal = prepared_journal(tmp_path)
    store = RecordedAccountStore()
    store.present.update([store.addresses[0], store.addresses[2]])
    store.accounts[store.addresses[2]]["owner"] = recorded_deployment()["publisher"]
    with rpc_peer(store) as rpc, pytest.raises(ValueError, match="foreign owner"):
        journal.reconcile(configuration(), rpc)
    assert journal.load()["intent"] is None
    assert journal.load()["phase"] == "deployed"


def test_partial_checkpoint_cannot_claim_ready(tmp_path):
    journal = prepared_journal(tmp_path)
    path = tmp_path / "record.json"
    with pytest.raises(ValueError, match="not reconciled"):
        journal.publish(path, recorded_deployment(), "ready")
    assert not path.exists()
    journal.publish(path, recorded_deployment(), "deployed")
    assert json.loads(path.read_text())["phase"] == "deployed"
    assert json.loads(path.read_text())["version"] == 1


def test_atomic_persistence_and_task_record():
    source = HELPER.read_text()
    assert source.index('output.flush()') < source.index('os.fsync(output.fileno())') < source.index('os.replace(temporary, path)')
    assert 'os.fsync(descriptor)' in source
    record = tomllib.loads(RECORD.read_text())
    assert record["finding"]["006"]["test"] == "test_wrong_keypair_refused_before_broadcast"
    assert record["finding"]["005"]["test"] == "test_restart_after_confirmed_initialise_and_each_asset"
    assert record["proof"]["review_only"]
    assert record["qualification"]["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_485.py"
    assert record["qualification"]["verified_revision_source"]
    assert record["baseline"]["executed"] is False
