"""Bounded HPX repair qualification: real state, shell entrypoints and source contracts.

No native node or network is needed. BankStore is the permitted in-memory peer
store: it commits transfers atomically and emits receipts consumed by production
recovery code. Native signature creation is a source-contract check, not a live
signing claim.
"""
import ast
import base64
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import sqlite3
import subprocess

import pytest

ROOT = Path(__file__).resolve().parents[3]
HPX = ROOT / "tools/hpx"


def load_module(name):
    spec = importlib.util.spec_from_file_location(name, HPX / f"{name}.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


fleet = load_module("fleet_state")
release = load_module("release_identity")
ACCOUNT_A = "pax1" + "q" * 38
ACCOUNT_B = "pax1" + "p" * 38
VALIDATOR_A = "paxvaloper1" + "q" * 38
VALIDATOR_B = "paxvaloper1" + "p" * 38
CONTEXT = {"chain": "hyperpax_125-1", "rpc": "tcp://127.0.0.1:26657", "denom": "uhpx"}


def operator(host):
    account, validator = (ACCOUNT_A, VALIDATOR_A) if host == "host-a" else (ACCOUNT_B, VALIDATOR_B)
    return f"OPERATOR {host} {account} {validator} 0x{'11' * 20}"


def full_roster(directory):
    state = fleet.FleetState(directory)
    for host in state.begin(["host-a", "host-b"]):
        state.record(host, operator(host))
    return state


def phase(name):
    source = (HPX / "stake-fleet.sh").read_text()
    start = source.index(f"phase_{name}(){{")
    return source[start:source.index("\n}\n", start) + 2]


class BankStore:
    """Concrete transactional peer store; JSON receipts cross into HPX unchanged."""
    def __init__(self):
        self.db = sqlite3.connect(":memory:")
        self.db.executescript("CREATE TABLE balances(account TEXT PRIMARY KEY, amount INTEGER); CREATE TABLE receipts(hash TEXT PRIMARY KEY, body TEXT);")
        self.db.executemany("INSERT INTO balances VALUES (?, ?)", [("sender", 1000), (ACCOUNT_A, 0), (ACCOUNT_B, 0)])
        self.db.commit()

    def balance(self, account):
        return self.db.execute("SELECT amount FROM balances WHERE account=?", (account,)).fetchone()[0]

    def submit(self, payment, directory):
        # The store validates the journal's exact public transfer envelope.
        message = payment["signed"]
        raw = json.dumps(message, sort_keys=True).encode()
        assert hashlib.sha256(raw).hexdigest().upper() == payment["hash"]
        assert message["to"] == payment["account"] and message["amount"] == payment["amount"]
        if self.transaction(payment["hash"]) is not None:
            raise fleet.FleetError("duplicate transfer")
        code = 0 if self.balance(message["from"]) >= message["amount"] else 5
        with self.db:
            if code == 0:
                self.db.execute("UPDATE balances SET amount=amount-? WHERE account=?", (message["amount"], message["from"]))
                self.db.execute("UPDATE balances SET amount=amount+? WHERE account=?", (message["amount"], message["to"]))
            response = {"txhash": payment["hash"], "height": "1", "code": code}
            self.db.execute("INSERT INTO receipts VALUES (?, ?)", (payment["hash"], json.dumps(response)))

    def transaction(self, txhash):
        row = self.db.execute("SELECT body FROM receipts WHERE hash=?", (txhash,)).fetchone()
        return json.loads(row[0]) if row else None


def prepare(state, account=ACCOUNT_A, amount=450):
    message = {"from": "sender", "to": account, "amount": amount}
    encoded = base64.b64encode(json.dumps(message, sort_keys=True).encode()).decode()
    return state.prepare(account, amount, message, encoded)


def test_repaired_shells_parse_and_guard_production_commands():
    for name in ("fleet-keygen.sh", "stake-fleet.sh", "publish.sh"):
        result = subprocess.run(["bash", "-n", str(HPX / name)], capture_output=True, text=True)
        assert result.returncode == 0, result.stderr
    for name in ("fleet-keygen.sh", "stake-fleet.sh"):
        source = (HPX / name).read_text()
        assert 'begin "${' in source and 'record "$ip" "$line"' in source
        assert 'complete || die "incomplete' in source
        assert 'flock -n 9 || die' in source
        assert 'sort -u "${OPERATORS_FILE}.tmp" > "$OPERATORS_FILE"' not in source
        assert 'if [ "$?" -ne 0 ] || ! python3' in source


def test_f29_roster_survives_stop_reopen_and_partial_discovery(tmp_path):
    with fleet.locked(tmp_path) as state:
        state.begin(["host-a", "host-b"])
        state.record("host-a", operator("host-a"))
    with fleet.locked(tmp_path) as reopened:
        assert reopened.begin(["host-b"]) == ["host-a", "host-b"]
        reopened.record("host-b", operator("host-b"))
        assert reopened.data["hosts"]["host-a"]["identity"][1] == ACCOUNT_A
        assert not reopened.data["hosts"]["host-a"]["verified"]
        with pytest.raises(fleet.FleetError, match="incomplete"):
            reopened.complete()
        assert ACCOUNT_A in (tmp_path / "operators.tsv").read_text()
    with fleet.locked(tmp_path) as final:
        for host in final.begin(["host-a"]):
            final.record(host, operator(host))
        assert final.complete() == [ACCOUNT_A, ACCOUNT_B]


def test_f30_legacy_roster_retained_and_consumers_block_incomplete_state(tmp_path):
    (tmp_path / "operators.tsv").write_text(f"host-a\thost-a\t{ACCOUNT_A}\t{VALIDATOR_A}\n")
    state = fleet.FleetState(tmp_path)
    state.begin(["host-b"])
    state.record("host-b", operator("host-b"))
    assert ACCOUNT_A in (tmp_path / "operators.tsv").read_text()
    with pytest.raises(fleet.FleetError, match="incomplete"):
        state.plan("sender", 1000, 100, CONTEXT)
    for name in ("fund", "stake_cmds", "stake"):
        body = phase(name)
        assert 'python3 "$STATE" "$WORKDIR" complete || die' in body
        if name == "fund":
            assert body.index('"$WORKDIR" complete') < body.index('"$WORKDIR" fund')
        else:
            assert body.index('"$WORKDIR" complete') < body.index('while IFS=')


def test_roster_refuses_changed_duplicate_and_malformed_identities(tmp_path):
    state = full_roster(tmp_path)
    state.begin(["host-a", "host-b"])
    with pytest.raises(fleet.FleetError, match="identity changed"):
        state.record("host-a", operator("host-b"))
    with pytest.raises(fleet.FleetError, match="invalid"):
        state.record("host-a", "OPERATOR host-a wrong wrong")
    with pytest.raises(fleet.FleetError, match="incomplete"):
        state.complete()
    assert state.data["hosts"]["host-a"]["identity"][1] == ACCOUNT_A
    state.begin(["host-c"])
    with pytest.raises(fleet.FleetError, match="duplicate"):
        state.record("host-c", operator("host-b"))


def test_f31_plan_persists_original_allocation_and_only_deficits(tmp_path):
    state = full_roster(tmp_path)
    original = copy.deepcopy(state.plan("sender", 1000, 100, CONTEXT))
    assert original["targets"] == {ACCOUNT_A: 450, ACCOUNT_B: 450}
    del state
    reopened = fleet.FleetState(tmp_path)
    assert reopened.plan("sender", 200, 100, CONTEXT) == original
    assert reopened.deficit(ACCOUNT_A, 175) == 275
    assert reopened.deficit(ACCOUNT_B, 450) == 0
    assert reopened.deficit(ACCOUNT_B, 600) == 0
    with pytest.raises(fleet.FleetError, match="context changed"):
        reopened.plan("another-sender", 200, 100, CONTEXT)
    with pytest.raises(fleet.FleetError, match="context changed"):
        reopened.plan("sender", 200, 100, dict(CONTEXT, chain="another-chain"))
    with pytest.raises(fleet.FleetError, match="freezes"):
        reopened.begin(["host-c"])


def test_f31_f32_recovery_after_commit_before_receipt_keeps_one_payment(tmp_path):
    state = full_roster(tmp_path)
    state.plan("sender", 1000, 100, CONTEXT)
    payment = prepare(state)
    stored_hash = payment["hash"]
    peer = BankStore()
    peer.submit(payment, tmp_path)
    # Stop the HPX component after the peer commits but before it records a receipt.
    del payment, state
    reopened = fleet.FleetState(tmp_path)
    payment = reopened.data["payments"][0]
    fleet.settle(reopened, peer, payment, attempts=1, pause=None, submit=False)
    receipt = fleet.FleetState(tmp_path).data["payments"][0]
    assert receipt["state"] == "committed" and receipt["hash"] == stored_hash
    assert receipt["receipt"]["height"] == "1" and receipt["receipt"]["code"] == 0
    assert peer.balance("sender") == 550 and peer.balance(ACCOUNT_A) == 450
    assert peer.db.execute("SELECT COUNT(*) FROM receipts").fetchone()[0] == 1
    assert reopened.deficit(ACCOUNT_A, peer.balance(ACCOUNT_A)) == 0



def test_f31_f32_full_funding_driver_reconciles_before_new_signing(tmp_path):
    state = full_roster(tmp_path)
    original = copy.deepcopy(state.plan("sender", 1000, 100, CONTEXT))
    peer = BankStore()
    first = prepare(state, ACCOUNT_B)
    peer.submit(first, tmp_path)
    state.receipt(first, peer.transaction(first["hash"]), peer.balance(ACCOUNT_B))
    second = prepare(state, ACCOUNT_A)
    peer.submit(second, tmp_path)
    del state
    reopened = fleet.FleetState(tmp_path)
    fleet.fund(reopened, peer, "sender", 100, CONTEXT, True, attempts=1, pause=None)
    saved = fleet.FleetState(tmp_path)
    assert saved.data["plan"] == original
    assert peer.balance("sender") == 100
    assert peer.balance(ACCOUNT_A) == peer.balance(ACCOUNT_B) == 450
    assert len(saved.data["payments"]) == 2
    assert all(p["state"] == "committed" for p in saved.data["payments"])
    assert peer.db.execute("SELECT COUNT(*) FROM receipts").fetchone()[0] == 2


def test_f31_dry_run_reports_only_original_deficit_after_restart(tmp_path, capsys):
    state = full_roster(tmp_path)
    original = copy.deepcopy(state.plan("sender", 1000, 100, CONTEXT))
    del state
    peer = BankStore()
    peer.db.execute("UPDATE balances SET amount=175 WHERE account=?", (ACCOUNT_A,))
    peer.db.execute("UPDATE balances SET amount=200 WHERE account='sender'")
    fleet.fund(fleet.FleetState(tmp_path), peer, "sender", 100, CONTEXT, False)
    output = capsys.readouterr().out
    assert f"{ACCOUNT_A}: outstanding 275uhpx" in output
    assert f"{ACCOUNT_B}: outstanding 450uhpx" in output
    assert fleet.FleetState(tmp_path).data["plan"] == original
    assert fleet.FleetState(tmp_path).data["payments"] == []
    assert peer.db.execute("SELECT COUNT(*) FROM receipts").fetchone()[0] == 0

def test_f32_prepared_payment_rebroadcasts_exact_bytes_after_restart(tmp_path):
    state = full_roster(tmp_path)
    state.plan("sender", 1000, 100, CONTEXT)
    prepared = copy.deepcopy(prepare(state))
    del state
    reopened = fleet.FleetState(tmp_path)
    peer = BankStore()
    fleet.settle(reopened, peer, reopened.data["payments"][0], attempts=1, pause=None, submit=False)
    saved = fleet.FleetState(tmp_path).data["payments"][0]
    assert saved["signed"] == prepared["signed"] and saved["hash"] == prepared["hash"]
    assert saved["state"] == "committed" and peer.balance(ACCOUNT_A) == 450


@pytest.mark.parametrize("change,balance,message", [
    ({"height": "0"}, 450, "not committed"),
    ({"code": 9}, 450, "transaction failed"),
    ({"txhash": "A" * 64}, 450, "hash mismatch"),
    ({}, 449, "below allocation"),
])
def test_f32_unresolved_results_never_become_receipts(tmp_path, change, balance, message):
    state = full_roster(tmp_path)
    state.plan("sender", 1000, 100, CONTEXT)
    payment = prepare(state)
    response = dict({"txhash": payment["hash"], "height": "7", "code": 0}, **change)
    with pytest.raises(fleet.FleetError, match=message):
        state.receipt(payment, json.loads(json.dumps(response)), balance)
    assert fleet.FleetState(tmp_path).data["payments"][0]["state"] == "prepared"
    with pytest.raises(fleet.FleetError, match="unresolved"):
        prepare(state, ACCOUNT_B)


def test_f32_missing_application_code_is_not_success(tmp_path):
    state = full_roster(tmp_path)
    state.plan("sender", 1000, 100, CONTEXT)
    payment = prepare(state)
    with pytest.raises(fleet.FleetError, match="not committed"):
        state.receipt(payment, {"txhash": payment["hash"], "height": "7"}, 450)
    with pytest.raises(fleet.FleetError, match="unresolved payment"):
        fleet.settle(state, BankStore(), payment, attempts=0, pause=None, submit=True)
    assert fleet.FleetState(tmp_path).data["payments"][0]["state"] == "prepared"


def test_f32_native_transport_journals_signed_hash_before_broadcast():
    source = (HPX / "fleet_state.py").read_text()
    ast.parse(source)
    fund = source[source.index("def fund("):source.index("def settle(")]
    assert fund.index("state.plan(") < fund.index("peer.sign(")
    assert fund.index("state.prepare(") < fund.index("submit=True")
    assert fund.index("submit=False") < fund.index("peer.sign(")
    assert '"--generate-only"' in source and '"tx", "sign"' in source
    assert '"tx", "encode"' in source and '"--broadcast-mode", "sync"' in source
    assert '"q", "tx", txhash' in source
    assert 'int(tx["code"]) == 0' in source
    assert '"code" in tx' in source
    assert 'self.run("tx", "broadcast", str(path)' in source
    assert '"$WORKDIR" fund' in phase("fund")


def test_f33_dry_run_only_public_queries_and_association_uses_keyring():
    body = phase("fund_wallet")
    preview = body[body.index("if ! broadcasting;"):body.index('if key_exists "$SRC_KEY";')]
    assert 'key_addr "$SRC_KEY"' in preview and 'bank_uhpx "$public_src"' in preview
    assert 'return 0' in preview
    assert not re.search(r"keys (?:add|export|mnemonic)|associate-address|tx bank", preview)
    assert '--unarmored-hex' not in body and 'keys export' not in body
    assert 'associate-address --from "$SRC_KEY" "${KB[@]}"' in body
    source = (HPX / "stake-fleet.sh").read_text()
    assert 'KEYRING_BACKEND="${HPX_KEYRING_BACKEND:-file}"' in source
    # Check the native consumer's supported no-positional-secret branch.
    native = (ROOT / "modules/evm/client/cli/tx.go").read_text()
    native = native[native.index('func CmdAssociateAddress()'):native.index('func CmdSend()')]
    assert 'cobra.MaximumNArgs(1)' in native
    assert 'kb.Key(clientCtx.GetFromName())' in native
    assert 'if len(args) == 1' in native


def test_f34_configuration_loaded_before_defaults_and_shell_wins(tmp_path):
    config = tmp_path / "fleet.env"
    config.write_text('HPX_RPC=tcp://file.invalid:26657\nHPX_CHAIN_ID=file-chain\nHPX_GAS_PRICES=0.2uhpx\n')
    env = {"PATH": os.defpath, "HPX_ENV_FILE": str(config)}
    file_result = subprocess.run(["bash", str(HPX / "stake-fleet.sh"), "help"], env=env, text=True, capture_output=True)
    assert file_result.returncode == 0, file_result.stderr
    assert "HPX_RPC=tcp://file.invalid:26657" in file_result.stdout
    assert "HPX_CHAIN_ID=file-chain" in file_result.stdout
    assert "HPX_GAS_PRICES=0.2uhpx" in file_result.stdout
    env.update(HPX_RPC="tcp://shell.invalid:26657", HPX_CHAIN_ID="shell-chain")
    shell_result = subprocess.run(["bash", str(HPX / "stake-fleet.sh"), "help"], env=env, text=True, capture_output=True)
    assert shell_result.returncode == 0, shell_result.stderr
    assert "HPX_RPC=tcp://shell.invalid:26657" in shell_result.stdout
    assert "HPX_CHAIN_ID=shell-chain" in shell_result.stdout
    assert "HPX_GAS_PRICES=0.2uhpx" in shell_result.stdout
    source = (HPX / "stake-fleet.sh").read_text()
    assert 'REPO_ROOT="$(cd "${SELF_DIR}/../.." && pwd)"' in source
    assert 'ENV_FILE="${HPX_ENV_FILE:-${REPO_ROOT}/.env}"' in source
    assert source.index('. "$ENV_FILE"') < source.index('readonly RPC=')
    env["HPX_ENV_FILE"] = str(tmp_path / "missing")
    missing = subprocess.run(["bash", str(HPX / "stake-fleet.sh"), "help"], env=env, text=True, capture_output=True)
    assert missing.returncode != 0 and "selected env file not found" in missing.stderr


def identity_inputs():
    manifest = json.loads((ROOT / "tools/release/version.json").read_text())
    revision, digest = "a" * 40, "b" * 64
    info = {"version": manifest["version"], "commit": revision, "server_name": "paxd"}
    provenance = {"version": manifest["version"], "source_revision": revision, "paxd_sha256": digest}
    return manifest, info, provenance, digest, revision, manifest["version"]


def test_f35_validated_identity_agrees_with_manifest_and_build_record():
    values = identity_inputs()
    assert release.validate_release(*values) == values[0]["version"]
    assert values[0]["hpx_release_identity"] == {"schema": 1, "provenance_required": True}
    source = (HPX / "publish.sh").read_text()
    assert 'tools/release/version.json' in source
    assert '${HPX_PAXD_VERSION:-$(jq -er' in source
    assert '"$stage/paxd" version --long --output json' in source
    assert '"$stage/build-provenance.json" "$stage/paxd" "$source_revision" "$PAXD_VERSION"' in source
    validation = source.index('python3 "$HPX_DIR/release_identity.py"')
    assert validation < source.index('> "$stage/paxd.sha256"')
    assert validation < source.index('> "$stage/checksums.txt"')
    assert validation < source.index('mv -Tf "$next_link" "$ARTIFACTS_ROOT/current"')
    assert '"$BUILD_PROVENANCE"' in source[source.index('required=('):source.index('for file in')]


@pytest.mark.parametrize("target,key,value", [
    (0, "version", "v0.0.1"), (1, "version", "v0.0.1"),
    (1, "commit", "c" * 40), (1, "server_name", "other"),
    (2, "version", "v0.0.1"), (2, "source_revision", "c" * 40),
    (2, "paxd_sha256", "c" * 64),
])
def test_f35_mismatched_release_metadata_is_refused(target, key, value):
    values = list(identity_inputs())
    values[target][key] = value
    with pytest.raises(ValueError):
        release.validate_release(*values)


def test_f35_missing_provenance_or_advertised_override_is_refused():
    values = list(identity_inputs())
    values[2] = {}
    with pytest.raises(ValueError):
        release.validate_release(*values)
    values = list(identity_inputs())
    values[5] = "v0.0.1"
    with pytest.raises(ValueError):
        release.validate_release(*values)


def test_f42_documented_paths_identity_and_artifact_mirror():
    doc = (HPX / "README.md").read_text()
    assert 'Artifact and library mirror: `https://node.hyperpaxeer.com`' in doc
    assert 'tools/hpx/publish.sh' in doc and 'tools/hpx/registry' in doc
    assert 'tools/hpx/hosting/fly.toml' in doc
    assert 'does **not** read' not in doc
    assert 'tools/release/version.json' in doc and 'HPX_BUILD_PROVENANCE' in doc
    assert 'before writing checksums or switching' in doc
    for field in ('version', 'source_revision', 'paxd_sha256'):
        assert f'"{field}"' in doc
    assert 'Explicit shell values override file values' in doc
    assert 'original' in doc and 'signed bytes' in doc
    assert not re.search(r'(?m)^\s*(?:sudo )?hpx/(?:publish|hosting|registry)', doc)
