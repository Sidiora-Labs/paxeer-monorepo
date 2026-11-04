"""22.33.809: recorded native receipts and a loopback Comet transcript peer.

Task record (scope permits this module, not spec/):
44 -> test_f44_workspace_and_binary_identity, test_f44_identity_mismatch_refuses.
45 -> test_f45_committed_payment, test_f45_verification_fails_closed.
46 -> test_f46_claims_require_case_endpoint_and_commit, test_f46_missing_inputs_fail_case.
47 -> test_f47_catalogue_targets_native_producers, test_f47_restart_and_idempotent_retry.
Baseline: not run; the ordered task instructions require implementation before
hf-build/hf-done and prohibit a separate invocation of the verify command.
Qualification revision, command, exit and private log are recorded by hf-done.
Wiring request: tools/qualification/release_runner.py (domain 16) owns renaming the
historical crash-before/after-batch-wal case IDs to native WAL names.
Wiring request: node/sequencer (domain 11) owns deployed fault-injection controls;
local native restart tests cannot qualify a deployed recovery journey.
Wiring request: services/platform/docs/samples (domain 15) must export each
journey's exact transaction identity before those cases can claim real_stack.
"""
from __future__ import annotations

import base64
import copy
import hashlib
import json
import re
import sqlite3
import sys
import threading
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlsplit

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))

from tools.qualification import beta_driver as driver
from layerx_sdk.committed import verify_commit
from layerx_sdk.evm_transaction import decode_signed_transaction
from layerx_sdk.x402_rpc import PaymentRpc

RECORDING = json.loads((ROOT / "daemon/committed-fixture/vectors/vector-v4-committed.json").read_text())
SETTLEMENT = RECORDING["settlement"]
PAYMENT = {
    "rawTransaction": SETTLEMENT["raw_transaction"],
    "receiptJson": SETTLEMENT["receipt"],
    "signedReceipt": SETTLEMENT["receipt"]["receipt"],
    "pinnedKey": {"keyId": SETTLEMENT["receipt"]["keyId"],
                  "publicKey": SETTLEMENT["receipt"]["signingKey"],
                  "firstHeight": 10, "lastHeight": 10},
}
LIGHT = json.loads((ROOT / "consensus/types/testdata/light_commit_v1.json").read_text())


def raw(value):
    return bytes.fromhex(value.removeprefix("0x"))


def b64(value):
    return base64.b64encode(value).decode()


class RecordedPaxd:
    """Durable transcript peer; production driver and SDK perform all verification.

    Every receipt, native TxRaw envelope, proof, header and commit comes from
    the actual Go recorder, unchanged. Its historic EVM chain identity is pinned
    explicitly; this fixture is never relabelled as a chain-125 recording.
    """

    def __init__(self, database):
        self.db = sqlite3.connect(database, check_same_thread=False)
        self.db.execute("CREATE TABLE IF NOT EXISTS transactions (hash TEXT PRIMARY KEY, raw TEXT, receipt TEXT)")
        self.db.commit()
        self.requests = []
        self.mutate = lambda kind, value: value
        self.raw = raw(PAYMENT["rawTransaction"])
        self.tx_hash = PAYMENT["receiptJson"]["txHash"]
        self.height = int(PAYMENT["receiptJson"]["height"], 16)
        self.chain_id = decode_signed_transaction(self.raw)[0].chain_id
        self.validators = [{"address": validator["address"],
                            "public_key": base64.b64decode(validator["pub_key"]["value"]).hex(),
                            "voting_power": int(validator["voting_power"])} for validator in SETTLEMENT["validators"]]
        self.header = copy.deepcopy(SETTLEMENT["execution_header"])
        self.commit = copy.deepcopy(SETTLEMENT["execution_commit"])
        self.envelope = base64.b64decode(SETTLEMENT["transaction_proof"]["data"])
        self.batch = base64.b64decode(SETTLEMENT["sequence_batch_proof"]["data"])
        self.block_hash = SETTLEMENT["receipt"]["blockHash"]
        self.receipt = copy.deepcopy(SETTLEMENT["receipt"])

    def close(self):
        self.db.close()

    def count(self):
        return self.db.execute("SELECT count(*) FROM transactions").fetchone()[0]

    def rpc(self, request):
        assert set(request) == {"jsonrpc", "id", "method", "params"}
        assert request["jsonrpc"] == "2.0" and type(request["id"]) is int
        method, params = request["method"], request["params"]
        self.requests.append((method, params))
        if method == "eth_sendRawTransaction":
            assert params == [PAYMENT["rawTransaction"]]
            with self.db:
                self.db.execute("INSERT OR IGNORE INTO transactions VALUES (?, ?, ?)",
                                (self.tx_hash, PAYMENT["rawTransaction"], json.dumps(self.receipt)))
            result = self.tx_hash
        elif method == "eth_chainId":
            assert params == []
            result = hex(self.chain_id)
        elif method == "eth_blockNumber":
            assert params == []
            result = hex(self.height)
        else:
            assert params == [self.tx_hash]
            row = self.db.execute("SELECT raw, receipt FROM transactions WHERE hash=?", (self.tx_hash,)).fetchone()
            assert row is not None
            if method == "eth_getRawTransactionByHash":
                result = row[0]
            elif method == "pax_sequencerReceipt":
                result = json.loads(row[1])
            elif method == "eth_getTransactionReceipt":
                result = {"transactionHash": self.tx_hash, "blockNumber": hex(self.height),
                          "blockHash": self.block_hash, "transactionIndex": "0x1", "status": "0x1"}
            else:
                raise AssertionError(method)
        envelope = {"jsonrpc": "2.0", "id": request["id"], "result": self.mutate(method, copy.deepcopy(result))}
        return self.mutate("envelope", envelope)

    def comet(self, path):
        url = urlsplit(path)
        self.requests.append((url.path, parse_qs(url.query)))
        if url.path == "/commit":
            assert parse_qs(url.query) == {"height": [str(self.height)]}
            result = {"signed_header": {"header": self.header, "commit": self.commit}, "canonical": True}
        elif url.path == "/block":
            assert parse_qs(url.query) == {"height": [str(self.height)]}
            result = {"block_id": self.commit["block_id"],
                      "block": {"header": self.header, "data": {"txs": [b64(self.batch), b64(self.envelope)]}}}
        elif url.path == "/tx":
            digest = hashlib.sha256(self.envelope).hexdigest().upper()
            assert parse_qs(url.query) == {"hash": ["0x" + digest], "prove": ["true"]}
            result = {"hash": digest, "height": str(self.height), "index": 1,
                      "tx_result": SETTLEMENT["exec_tx_result"], "tx": b64(self.envelope),
                      "proof": SETTLEMENT["transaction_proof"]}

        else:
            raise AssertionError(path)
        return {"jsonrpc": "2.0", "id": -1, "result": self.mutate(url.path, copy.deepcopy(result))}


@contextmanager
def server(peer, port=0):
    errors = []

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            self.exchange(True)

        def do_GET(self):
            self.exchange(False)

        def exchange(self, post):
            try:
                assert self.headers["Accept"] in (None, "application/json")
                if post:
                    assert self.path == "/"
                    assert self.headers["Content-Type"] == "application/json"
                    request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                    response = peer.rpc(request)
                else:
                    response = peer.comet(self.path)
                data = driver.json_bytes(response)
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)
            except Exception as error:
                errors.append(error)
                self.send_error(500)

    httpd = ThreadingHTTPServer(("127.0.0.1", port), Handler)
    worker = threading.Thread(target=httpd.serve_forever, daemon=True)
    worker.start()
    try:
        yield f"http://127.0.0.1:{httpd.server_port}"
    finally:
        httpd.shutdown()
        httpd.server_close()
        worker.join()
        assert not errors, repr(errors)


def trust_file(directory, peer, endpoint):
    path = directory / "independent-trust.json"
    path.write_bytes(driver.json_bytes({"node_url": endpoint, "comet_url": endpoint,
        "chain_id": peer.header["chain_id"], "evm_chain_id": peer.chain_id,
        "receipt_keys": [PAYMENT["pinnedKey"]], "validators": peer.validators}))
    return path


def context(directory, endpoint, trust=None, *, case_id="rust/first-payment"):
    environment = {} if trust is None else {driver.NETWORK_TRUST_INPUT: str(trust)}
    return driver.CaseContext("platform-qualify-adoption", case_id, directory,
                              driver.Stack(endpoint, endpoint, endpoint, endpoint), "recorded-source", environment)


@pytest.fixture
def live_peer(tmp_path):
    peer = RecordedPaxd(tmp_path / "peer.sqlite")
    try:
        with server(peer) as endpoint:
            assert PaymentRpc(endpoint).send(PAYMENT["rawTransaction"]) == peer.tx_hash
            yield peer, endpoint, trust_file(tmp_path, peer, endpoint)
    finally:
        peer.close()


def test_f44_workspace_and_binary_identity():
    assert driver.AGENT_MANIFEST == ROOT / "services/agent/Cargo.toml"
    identity = driver.agent_workspace_identity()
    assert set(identity["packages"]) == {"layerx-agentd", "layerx-agentd-host"}
    assert identity["packages"]["layerx-agentd-host"]["binary"] == "layerx-agentd"
    assert identity["sha256"] == driver.file_digest(driver.AGENT_MANIFEST)


@pytest.mark.parametrize("change", ("member", "package", "binary", "source"))
def test_f44_identity_mismatch_refuses(tmp_path, change):
    # Concrete Cargo manifests are the producer interface, parsed by production.
    workspace = tmp_path / "Cargo.toml"
    workspace.write_text(driver.AGENT_MANIFEST.read_text())
    for member in ("layerx-agentd", "layerx-agentd-host"):
        target = tmp_path / "crates" / member
        target.mkdir(parents=True)
        (target / "Cargo.toml").write_text((driver.AGENT_MANIFEST.parent / "crates" / member / "Cargo.toml").read_text())
    source = tmp_path / "crates/layerx-agentd/src/main.rs"
    source.parent.mkdir()
    source.write_text((driver.AGENT_MANIFEST.parent / "crates/layerx-agentd/src/main.rs").read_text())
    assert driver.agent_workspace_identity(workspace)["packages"]["layerx-agentd-host"]["binary"] == "layerx-agentd"
    if change == "member":
        workspace.write_text(workspace.read_text().replace('"crates/layerx-agentd",', '"crates/absent",'))
    elif change == "package":
        manifest = tmp_path / "crates/layerx-agentd/Cargo.toml"
        manifest.write_text(manifest.read_text().replace('name = "layerx-agentd"', 'name = "unrelated"', 1))
    elif change == "binary":
        manifest = tmp_path / "crates/layerx-agentd-host/Cargo.toml"
        manifest.write_text(manifest.read_text().replace('name = "layerx-agentd"', 'name = "unrelated"', 1))
    else:
        source.unlink()
    with pytest.raises(driver.CaseFailure):
        driver.agent_workspace_identity(workspace)


def test_recorded_go_commit_checks_the_comet_codec():
    assert verify_commit(LIGHT["header"], LIGHT["commit"],
        trusted_validators=tuple(LIGHT["validators"]), chain_id=LIGHT["header"]["chain_id"]).hex() == LIGHT["header_hash"]


def test_f45_committed_payment(tmp_path, live_peer):
    peer, endpoint, trust = live_peer
    ctx = context(tmp_path / "positive", endpoint, trust)
    receipt = PaymentRpc(endpoint).receipt(peer.tx_hash)
    verification = driver.verify_first_payment_receipt(ctx, peer.tx_hash, receipt)
    assert verification["performed"] is True and verification["verified"] is True
    assert verification["receipt_sha256"] == hashlib.sha256(raw(PAYMENT["signedReceipt"])).hexdigest()
    assert verification["height"] == peer.height
    assert verification["block_hash"] == peer.block_hash
    assert verification["endpoint"] == endpoint
    assert verification["trust_sha256"] == driver.file_digest(trust)
    assert verification["case"] == ctx.case_id
    assert verification["native_transaction_sha256"] == hashlib.sha256(peer.envelope).hexdigest()
    assert {name for name, _ in peer.requests} >= {
        "pax_sequencerReceipt", "eth_getTransactionReceipt", "eth_getRawTransactionByHash", "eth_chainId", "/commit", "/block", "/tx"}
    artifact = json.loads((ctx.directory / "artifacts/paxd-commit.json").read_text())
    assert artifact["commit_response"]["result"]["signed_header"]["header"] == peer.header
    assert artifact["commit_response"]["result"]["signed_header"]["commit"] == SETTLEMENT["execution_commit"]
    assert artifact["transaction_response"]["result"]["proof"] == SETTLEMENT["transaction_proof"]
    assert artifact["block_response"]["result"]["block"]["data"]["txs"][1] == SETTLEMENT["transaction_proof"]["data"]
    assert artifact["transaction_response"]["result"]["proof"]["proof"]["index"] == "1"


@pytest.mark.parametrize("fault", (
    "missing-receipt", "submitted", "bad-signature", "self-authority", "wrong-block", "failed-execution",
    "missing-commit", "header-tamper", "insufficient-quorum", "proof-tamper", "wrong-position",
    "wrong-height", "rpc-envelope", "missing-pins", "wrong-key-range", "wrong-validator", "wrong-chain",
    "native-wrapper", "wrong-native-hash", "noncanonical", "missing-proof", "wrong-live-chain",
))
def test_f45_verification_fails_closed(tmp_path, live_peer, fault):
    peer, endpoint, trust = live_peer
    pinned = json.loads(trust.read_text())
    if fault == "missing-pins":
        pinned["receipt_keys"] = []
    elif fault == "wrong-key-range":
        pinned["receipt_keys"][0]["firstHeight"] = peer.height + 1
        pinned["receipt_keys"][0]["lastHeight"] = peer.height + 10
    elif fault == "wrong-validator":
        pinned["validators"][0]["public_key"] = "ff" * 32
    elif fault == "wrong-chain":
        pinned["chain_id"] = "another-network"
    trust.write_bytes(driver.json_bytes(pinned))

    def corrupt(kind, value):
        if kind == "pax_sequencerReceipt":
            if fault == "missing-receipt":
                return None
            if fault == "submitted":
                return {key: value[key] for key in ("txHash", "nonce")} | {"rung": "submitted"}
            if fault == "bad-signature":
                data = raw(value["receipt"])
                value["receipt"] = "0x" + (data[:-1] + bytes([data[-1] ^ 1])).hex()
            if fault == "self-authority":
                key = Ed25519PrivateKey.from_private_bytes(b"\x29" * 32)
                data = raw(value["receipt"])
                digest = hashlib.sha256(b"LXP/v1/receipt\0" + data[:-69] + b"\0").digest()
                value["receipt"] = "0x" + (data[:-64] + key.sign(digest)).hex()
                value["signingKey"] = "0x" + key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex()
                value["authority"] = {"sequencer_public_key": value["signingKey"]}
            if fault == "wrong-block":
                value["blockHash"] = "0x" + "ff" * 32
        elif kind == "/commit":
            if fault == "missing-commit":
                value.pop("signed_header")
            elif fault == "header-tamper":
                value["signed_header"]["header"]["app_hash"] = "ff" * 32
            elif fault == "insufficient-quorum":
                value["signed_header"]["commit"]["signatures"] = []
            elif fault == "noncanonical":
                value["canonical"] = False
        elif kind == "/block":
            if fault == "native-wrapper":
                changed = peer.envelope.replace(b"LegacyTx", b"UnknownX")
                assert changed != peer.envelope
                value["block"]["data"]["txs"][1] = b64(changed)
            elif fault == "wrong-native-hash":
                value["block_id"]["hash"] = "ff" * 32
        elif kind == "/tx":
            if fault == "failed-execution":
                value["tx_result"]["code"] = 7
            elif fault == "proof-tamper":
                value["proof"]["proof"]["aunts"][0] = b64(b"\xff" * 32)
            elif fault == "wrong-position":
                value["proof"]["proof"]["index"] = "0"
            elif fault == "wrong-height":
                value["height"] = str(peer.height + 1)
            elif fault == "missing-proof":
                value.pop("proof")
        elif kind == "eth_chainId" and fault == "wrong-live-chain":
            return hex(peer.chain_id + 1)
        elif kind == "envelope" and fault == "rpc-envelope":
            value["id"] += 1
        return value

    peer.mutate = corrupt
    ctx = context(tmp_path / fault, endpoint, trust)
    with pytest.raises(driver.CaseFailure, match="verification|trust"):
        driver.verify_paxd_commit(ctx, peer.tx_hash)
    assert ctx.paxd_evidence == []
    assert not (ctx.directory / "artifacts/paxd-commit.json").exists()


def test_f46_missing_inputs_fail_case(tmp_path):
    endpoint = "http://127.0.0.1:1"
    record = driver.run_case("platform-qualify-adoption", "rust/first-payment", tmp_path,
        driver.Stack(endpoint, endpoint, endpoint, endpoint), "source", {})
    assert record["status"] == "failed"
    assert driver.NETWORK_TRUST_INPUT in record["reason"]
    assert record["real_stack"] is False and record["paxd_evidence"] == []
    assert record["executions"] == []


def test_f46_claims_require_case_endpoint_and_commit(tmp_path, live_peer):
    peer, endpoint, trust = live_peer
    ctx = context(tmp_path / "live", endpoint, trust)
    driver.verify_first_payment_receipt(ctx, peer.tx_hash, peer.receipt)
    result = json.loads(ctx.result().read_text())
    assert result["scope"] == "paxd-committed" and result["real_stack"] is True
    identity = driver.agent_workspace_identity()
    record = {"id": ctx.case_id, "status": "passed", "scope": result["scope"],
              "real_stack": result["real_stack"], "paxd_evidence": result["paxd_evidence"]}
    manifest = driver.qualification_manifest(ctx.gate, ctx.source_identity, ctx.stack, ctx.started_at, [record], identity)
    assert manifest["real_stack"] is True
    for field, value in (("case", "another-case"), ("endpoint", "http://127.0.0.1:1"), ("performed", False), ("verified", False)):
        changed = copy.deepcopy(record)
        changed["paxd_evidence"][0][field] = value
        assert driver.qualification_manifest(ctx.gate, "source", ctx.stack, ctx.started_at, [changed], identity)["real_stack"] is False
    for field in ("transaction_hash", "block_hash", "height", "receipt_sha256", "commit_response_sha256"):
        changed = copy.deepcopy(record)
        del changed["paxd_evidence"][0][field]
        assert driver.qualification_manifest(ctx.gate, "source", ctx.stack, ctx.started_at, [changed], identity)["real_stack"] is False
    local = context(tmp_path / "local", endpoint, case_id="local-suite")
    local_result = json.loads(local.result().read_text())
    assert local_result["real_stack"] is False and local_result["scope"] == "local-suite"
    local_record = {"id": local.case_id, "status": "passed", **{name: local_result[name] for name in ("scope", "real_stack", "paxd_evidence")}}
    mixed = driver.qualification_manifest(ctx.gate, "source", ctx.stack, ctx.started_at, [record, local_record], identity)
    assert mixed["real_stack"] is False
    assert driver.qualification_manifest(ctx.gate, "source", ctx.stack, ctx.started_at, [], identity)["real_stack"] is False
    changed_stack = driver.Stack("http://127.0.0.1:1", endpoint, endpoint, endpoint)
    changed = driver.CaseContext(ctx.gate, "wrong-endpoint", tmp_path / "wrong-endpoint", changed_stack,
                                 "source", {driver.NETWORK_TRUST_INPUT: str(trust)})
    with pytest.raises(driver.CaseFailure, match="endpoint"):
        driver.verify_first_payment_receipt(changed, peer.tx_hash, peer.receipt)


def test_f47_catalogue_targets_native_producers():
    assert set(driver.CRASH_POINTS) <= driver.CASES["programs-qualify"].keys()
    selected = set()
    for case_id, (package, tests, agent) in driver.CRASH_POINTS.items():
        command = driver.native_test_command(package, tests)
        assert command[:3] == ("go", "test", package)
        assert "-count=1" in command and command[-1].startswith("^(") and command[-1].endswith(")$")
        sources = "\n".join(path.read_text() for path in (ROOT / package.removeprefix("./")).glob("*_test.go"))
        for name in tests:
            assert re.search(r"^func " + re.escape(name) + r"\(t \*testing.T\)", sources, re.MULTILINE), (case_id, name)
            assert re.fullmatch(command[-1], name)
            selected.add(name)
        if agent:
            suite, name = agent
            source = ROOT / "services/agent/crates/layerx-agentd/tests" / (suite + ".rs")
            assert "fn " + name + "(" in source.read_text()
    assert {"TestWALGroupSyncBeforeRelease", "TestWALCorruptTailStopsReplay",
            "TestCrashAfterReceiptReplaysIdenticalReceipt", "TestRepair475_ReconcileSealAfterCommitCrash"} <= selected
    source = Path(driver.__file__).read_text()
    for retired in ("test-finality-evidence", "test-batch-wal-recovery", "qualify-replay",
                    "custody_claim_without_a_settlement_domain_declaration_is_rejected",
                    "foreign_settlement_domain_on_a_custody_claim_is_rejected"):
        assert retired not in source
    for tests in ((), ("Missing",), ("TestAnything|.*",)):
        with pytest.raises(driver.CaseFailure, match="explicit test identities"):
            driver.native_test_command("./node", tests)


def test_f47_restart_and_idempotent_retry(tmp_path):
    database = tmp_path / "restart.sqlite"
    peer = RecordedPaxd(database)
    with server(peer) as endpoint:
        rpc = PaymentRpc(endpoint)
        assert rpc.send(PAYMENT["rawTransaction"]) == peer.tx_hash
        assert rpc.send(PAYMENT["rawTransaction"]) == peer.tx_hash
        trust = trust_file(tmp_path, peer, endpoint)
        before = context(tmp_path / "before", endpoint, trust)
        first = driver.verify_first_payment_receipt(before, peer.tx_hash, rpc.receipt(peer.tx_hash))
        before.result()
        assert peer.count() == 1
        port = urlsplit(endpoint).port
    peer.close()
    # Drop both the transport peer and driver context, then reopen durable bytes.
    del peer, rpc, before
    recovered = RecordedPaxd(database)
    try:
        with server(recovered, port) as endpoint:
            assert recovered.count() == 1
            rpc = PaymentRpc(endpoint)
            assert rpc.send(PAYMENT["rawTransaction"]) == recovered.tx_hash
            assert recovered.count() == 1
            after = context(tmp_path / "after", endpoint, trust)
            second = driver.verify_first_payment_receipt(after, recovered.tx_hash, rpc.receipt(recovered.tx_hash))
            assert second == first
            assert json.loads(after.result().read_text())["real_stack"] is True
            receipt = rpc.receipt(recovered.tx_hash)
            receipt["txHash"] = "0x" + "ff" * 32
            rejected = context(tmp_path / "changed-identity", endpoint, trust)
            with pytest.raises(driver.CaseFailure, match="identity"):
                driver.verify_first_payment_receipt(rejected, recovered.tx_hash, receipt)
            assert rejected.paxd_evidence == []
    finally:
        recovered.close()
