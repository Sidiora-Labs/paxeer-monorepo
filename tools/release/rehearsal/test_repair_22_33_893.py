"""Release/client wire qualification; no native runtime or external service is required.

The component under repair is programme.py. Its peers serve recorded node receipts
and in-process validator/ABI transcripts over actual HTTP; no programme method or
transport is replaced. These tests qualify the scenario driver, not a paxd binary.
"""

import base64
import copy
import hashlib
import importlib.util
import json
import sqlite3
import sys
import threading
from contextlib import contextmanager
from dataclasses import replace
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec, ed25519, utils

ROOT = Path(__file__).resolve().parents[3]
MODULE = importlib.util.spec_from_file_location(
    "rehearsal_programme", ROOT / "tools/release/rehearsal/programme.py"
)
P = importlib.util.module_from_spec(MODULE)
MODULE.loader.exec_module(P)

from layerx_sdk.ai_market import _encode_programs_call
from layerx_sdk.committed import _header_hash, _vote_bytes, verify_commit
from layerx_sdk.evm_transaction import Type2Transaction, decode_signed_transaction, encode_call
from layerx_sdk.native_program_call import decode_native_program_call
from layerx_sdk.program_lifecycle import NativeProgramDeploy, decode_lifecycle_calldata
from layerx_sdk.xweb_api import decode_xweb_api_payload, encode_xweb_api_payload

FIXTURES = ROOT / "sdk/platform/conformance/fixtures"
CANDIDATE = {
    "revision": "7988614413df79a83ae525988b7636ae368938a2",
    "binary_sha256": hashlib.sha256(b"disposable release candidate").hexdigest(),
    "version": "v6.12.0",
}
GATEWAY = {"key_id": "rehearsal", "secret": "lxp_live_" + "45" * 32}
TASK_RECORD = {
    "task": "22.33.893",
    "base_revision": CANDIDATE["revision"],
    "findings": {
        "programme-retired-helper-import": [
            "test_release_signing_without_relay_import",
            "test_wallet_real_rpc_and_legacy_signing",
            "test_price_submission_preserves_recorded_vector",
        ],
        "programme-merged-lifecycle-gap": [
            "test_merged_transaction_to_commit",
            "test_current_guest_deployment_and_call",
            "test_native_xweb_request_fulfil_and_result",
            "test_tenant_isolation_real_agent_envelope",
            "test_persisted_restart_closes_and_reopens_peer_and_driver",
        ],
    },
    "baseline": "Unrun: explicit implementation-first/hf-done-only order overrides the baseline step; the test module did not exist and the initial ledger had no gate runs. No baseline failure is claimed.",
    "wiring_requests": [
        "tools/release/signing.py — release/client domain 16: extract ReleaseSigning's shared API and migrate the remaining callers of services/platform/hosted/paxeer/evm.py before retiring that tree; this task removes the rehearsal dependency in place using the supported SDK codecs.",
        "tools/release/rehearse-v6120.sh — release domain 16: invoke all five merged CHECKS with a protected signed-transaction manifest, measured revision/binary SHA256/version, validator pins and agent credentials; restart the same candidate node homes between the before/after check_persisted_restart invocations and retain their evidence in the final rehearsal receipt.",
    ],
    "proof_boundary": "Real release client, signing, ABI/native payload, agent envelope, Comet signature/inclusion verification, HTTP transport and reopened SQLite transcript state; no native execution or four-node binary rollout certification is claimed by this Python gate.",
    "qualification": "The bounded hf-done record supplies the verified candidate revision, command, exit code and private log; this source record makes no independent gate claim.",
    "gotchas": [
        "Comet commits to a protobuf Cosmos EVM wrapper, not raw signed EVM bytes; both identities are checked before finality is accepted.",
        "The EIP-155 digest must come from the committed wallet attestor golden vector; an initially mistyped hand-copied digest failed qualification and was replaced with that exact source equality.",
        "JSON key order is not committed transaction order; restart replays and records transactions by committed height and nonce so deployment precedes its call.",
    ],
}


def load(name):
    return json.loads((FIXTURES / name).read_text())


def unhex(value):
    return bytes.fromhex(value.removeprefix("0x"))


def hexed(value):
    return "0x" + bytes(value).hex()


def b64(value):
    return base64.b64encode(value).decode()


def varint(value):
    result = bytearray()
    while value > 127:
        result.append((value & 127) | 128)
        value >>= 7
    return bytes(result) + bytes([value])


def protobuf(number, value):
    if isinstance(value, int):
        return b"" if value == 0 else varint(number * 8) + varint(value)
    value = value.encode() if isinstance(value, str) else bytes(value)
    return varint(number * 8 + 2) + varint(len(value)) + value


def cosmos_wrap(raw):
    """Producer framing from api/{eth,evm}/tx.proto and Cosmos TxRaw/TxBody."""
    tx, signature = decode_signed_transaction(raw)
    parity = signature[64] - 27
    if isinstance(tx, Type2Transaction):
        kind = "DynamicFeeTx"
        values = {
            1: str(tx.chain_id), 2: tx.nonce,
            3: str(tx.max_priority_fee_per_gas), 4: str(tx.max_fee_per_gas),
            5: tx.gas_limit, 6: hexed(tx.to), 7: str(tx.value), 8: tx.data,
            10: bytes([parity]) if parity else b"", 11: signature[:32], 12: signature[32:64],
        }
    else:
        kind = "LegacyTx"
        v = 2 * tx.chain_id + 35 + parity
        values = {
            1: tx.nonce, 2: str(tx.gas_price), 3: tx.gas_limit, 4: hexed(tx.to),
            5: str(tx.value), 6: tx.data, 7: v.to_bytes((v.bit_length() + 7) // 8, "big"),
            8: signature[:32], 9: signature[32:64],
        }
    encoded = b"".join(protobuf(number, value) for number, value in values.items())
    any_tx = protobuf(1, "/paxprotocol.paxchain.eth." + kind) + protobuf(2, encoded)
    msg = protobuf(1, any_tx)
    any_msg = protobuf(1, "/paxprotocol.paxchain.evm.MsgEVMTransaction") + protobuf(2, msg)
    return protobuf(1, protobuf(1, any_msg)) + protobuf(2, b"")


def signed_call(data, *, nonce=1, to=P.PROGRAMS, value=0):
    """Independent signer and the supported SDK's canonical type-2 transaction producer."""
    tx = Type2Transaction(125, nonce, 1, 2, 2_000_000, unhex(to), value, data)
    key = ec.derive_private_key(1, ec.SECP256K1())
    r, s = utils.decode_dss_signature(
        key.sign(tx.signing_digest(), ec.ECDSA(utils.Prehashed(hashes.SHA256())))
    )
    s = min(s, P.accounts._N - s)
    for parity in (0, 1):
        signature = r.to_bytes(32, "big") + s.to_bytes(32, "big") + bytes([27 + parity])
        address = "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf"
        if tx.recover_signer(signature) == address:
            return tx.attach_signature(signature, address)
    raise AssertionError("signature must recover to the independent producer")


def merkle(leaves):
    if len(leaves) == 1:
        return hashlib.sha256(b"\0" + leaves[0]).digest()
    split = 1 << ((len(leaves) - 1).bit_length() - 1)
    return hashlib.sha256(b"\1" + merkle(leaves[:split]) + merkle(leaves[split:])).digest()


class ValidatorProducer:
    def __init__(self):
        self.template = json.loads((ROOT / "consensus/types/testdata/light_commit_v1.json").read_text())
        self.keys = [ed25519.Ed25519PrivateKey.from_private_bytes(hashlib.sha256(bytes([i])).digest())
                     for i in range(1, 5)]
        self.keys.sort(key=lambda key: hashlib.sha256(key.public_key().public_bytes_raw()).digest()[:20])
        self.validators = [
            {"address": hashlib.sha256(key.public_key().public_bytes_raw()).digest()[:20].hex(),
             "public_key": key.public_key().public_bytes_raw().hex(), "voting_power": 10}
            for key in self.keys
        ]

    def produce(self, wrapped, height):
        header = copy.deepcopy(self.template["header"])
        header["height"] = str(height)
        header["validators_hash"] = merkle([
            protobuf(1, protobuf(1, bytes.fromhex(v["public_key"]))) + protobuf(2, v["voting_power"])
            for v in self.validators
        ]).hex()
        header["next_validators_hash"] = header["validators_hash"]
        header["data_hash"] = merkle([hashlib.sha256(wrapped).digest()]).hex()
        commit = copy.deepcopy(self.template["commit"])
        commit["height"] = height
        commit["block_id"]["hash"] = _header_hash(header).hex()
        for signature, validator, key in zip(commit["signatures"], self.validators, self.keys):
            signature["validator_address"] = validator["address"]
            signature["signature"] = b64(key.sign(_vote_bytes(header["chain_id"], commit, signature)))
            del signature["vote_sign_bytes"]
        commit["height"] = str(height)  # actual Comet JSON uses a string height
        return {"header": header, "commit": commit}


def event_log(event, values, height):
    topics, plain_types, plain = [hexed(event.topic)], [], []
    for kind, indexed, name in event.inputs:
        if indexed:
            topics.append(hexed(P.encode_values([kind], [values[name]])))
        else:
            plain_types.append(kind)
            plain.append(values[name])
    return {"address": P.XWEB, "topics": topics,
            "data": hexed(P.encode_values(plain_types, plain)),
            "blockNumber": hex(height), "logIndex": "0x0"}


class TranscriptPeer:
    """A persistent loopback transcript peer; never substitutes a programme component."""

    def __init__(self, path):
        self.db = sqlite3.connect(path, check_same_thread=False)
        self.db.execute("CREATE TABLE IF NOT EXISTS records (kind TEXT, id TEXT, body TEXT, PRIMARY KEY(kind,id))")
        self.db.execute("CREATE TABLE IF NOT EXISTS submissions (hash TEXT PRIMARY KEY, count INTEGER)")
        self.validator = ValidatorProducer()
        self.observed = []
        self.errors = []
        self.damage = None
        self.wallet_expectation = None
        self.web = P.Abi.load("precompiles/xweb/abi.json")
        self.programs = P.Abi.load("precompiles/layerxprograms/abi.json")

    def put(self, kind, identity, value):
        self.db.execute("INSERT OR REPLACE INTO records VALUES (?, ?, ?)", (kind, str(identity), json.dumps(value)))
        self.db.commit()

    def get(self, kind, identity):
        row = self.db.execute("SELECT body FROM records WHERE kind=? AND id=?", (kind, str(identity))).fetchone()
        assert row is not None, (kind, identity)
        return json.loads(row[0])

    def add(self, raw, height, logs=(), sequencer=None):
        wrapped = cosmos_wrap(raw)
        signed = self.validator.produce(wrapped, height)
        identity = hexed(P.keccak256(raw))
        tx, signature = decode_signed_transaction(raw)
        receipt = {
            "transactionHash": identity, "transactionIndex": "0x0", "blockNumber": hex(height),
            "blockHash": "0x" + signed["commit"]["block_id"]["hash"], "status": "0x1",
            "from": tx.recover_signer(signature), "to": hexed(tx.to),
            "gasUsed": "0x5208", "effectiveGasPrice": "0x1", "logs": list(logs),
        }
        self.put("receipt", identity, receipt)
        self.put("raw", identity, hexed(raw))
        self.put("commit", height, {"signed_header": signed, "canonical": True})
        self.put("tx", hexed(hashlib.sha256(wrapped).digest()), {
            "hash": hashlib.sha256(wrapped).hexdigest().upper(), "height": str(height), "index": 0,
            "tx_result": {"code": 0, "data": "", "log": "", "events": []}, "tx": b64(wrapped),
            "proof": {"root_hash": signed["header"]["data_hash"], "data": b64(wrapped),
                      "proof": {"total": "1", "index": "0", "leaf_hash": signed["header"]["data_hash"], "aunts": []}},
        })
        self.put("head", "height", height)
        if sequencer is not None:
            self.put("sequencer", identity, sequencer)
        return {"raw_transaction": hexed(raw), "comet_transaction": b64(wrapped), "height": height}

    def state(self, to, fn, args, values):
        # ABI encode on the server side; the real release client decodes on the other side.
        data = fn.encode(*args)
        self.put("call", to + hexed(data), hexed(P.encode_values(fn.outputs, values)))
        return {"to": to, "data": hexed(data), "expected": self.get("call", to + hexed(data))}

    def dispatch(self, method, params):
        if method == "eth_chainId":
            return "0x7d"
        if method == "abci_info":
            return {"response": {"version": CANDIDATE["version"], "app_version": "1"}}
        if method == "eth_blockNumber":
            return hex(self.get("head", "height"))
        if method == "eth_sendRawTransaction":
            raw = unhex(params[0])
            tx, signature = decode_signed_transaction(raw)
            assert tx.chain_id == 125 and tx.recover_signer(signature)
            identity = hexed(P.keccak256(raw))
            if self.wallet_expectation is not None:
                sender, target, data, value = self.wallet_expectation
                assert tx.nonce == 0 and tx.gas_limit == 21000 * 12 // 10 + 21000
                assert tx.recover_signer(signature).lower() == sender
                assert tx.to == unhex(target) and tx.data == data and tx.value == value
                self.add(raw, 1001)
            assert self.get("raw", identity) == params[0]
            self.db.execute("INSERT INTO submissions VALUES (?,1) ON CONFLICT(hash) DO UPDATE SET count=count+1", (identity,))
            self.db.commit()
            return identity
        if method == "eth_getTransactionReceipt":
            result = self.get("receipt", params[0])
            if self.damage == "receipt-height":
                result["blockNumber"] = hex(int(result["blockNumber"], 16) + 1)
            if self.damage == "receipt-block":
                result["blockHash"] = "0x" + "dd" * 32
            if self.damage == "receipt-status":
                result["status"] = "0x0"
            return result
        if method == "pax_sequencerReceipt":
            return self.get("sequencer", params[0])
        if method == "tx":
            assert params["prove"] is True
            result = self.get("tx", params["hash"])
            if self.damage == "inclusion":
                result["proof"]["proof"]["aunts"] = [b64(b"x" * 32)]
            return result
        if method == "commit":
            result = self.get("commit", params["height"])
            if self.damage == "commit-power":
                result["signed_header"]["commit"]["signatures"] = result["signed_header"]["commit"]["signatures"][:2]
            return result
        if method == "eth_call":
            assert params[1].startswith("0x")  # scenario calls always pin a committed block
            result = self.get("call", params[0]["to"] + params[0]["data"])
            if self.damage == "state":
                result = result[:-2] + "ff"
            return result
        if method == "eth_getBalance":
            return self.get("balance", params[0] + ":" + params[1])
        if method in ("eth_estimateGas", "eth_gasPrice", "eth_getTransactionCount"):
            return {"eth_estimateGas": "0x5208", "eth_gasPrice": "0x1", "eth_getTransactionCount": "0x0"}[method]
        raise AssertionError((method, params))

    def agent(self, envelope, authorization):
        assert authorization == "LayerX-Key rehearsal:" + GATEWAY["secret"]
        assert set(envelope) == {"version", "request_id", "operation", "request", "credential", "idempotency_key"}
        assert envelope["version"] == 1 and envelope["operation"] == "session.list"
        assert set(envelope["request"]) == {"context"} and envelope["idempotency_key"] is None
        credential = envelope["credential"]
        tenant = self.get("tenant", credential["session_id"])
        context = envelope["request"]["context"]
        assert set(context) == {"tenant", "agent_did", "authority_ref", "permitted_activity_types", "expiry", "client", "policy_version"}
        if (credential != {key: tenant[key] for key in ("tenant", "session_id", "token_id", "generation")}
                or context["tenant"] != tenant["tenant"] or context["agent_did"] != tenant["context"]["agent_did"]):
            return 403, {"class": "PolicyRefusal", "protocol_result_code": None, "retriability": "Terminal",
                         "reason": "tenant_session_mismatch", "request_id": envelope["request_id"]}
        return 200, {"request_id": envelope["request_id"], "value": tenant["expected_sessions"],
                     "verification_status": {"state": "achieved", "rung": "submitted"}}

    @contextmanager
    def serve(self):
        peer = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                try:
                    assert self.headers["Content-Type"] == "application/json"
                    body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                    peer.observed.append((self.path, dict(self.headers), body))
                    if self.path == "/v1/agent/rpc":
                        status, result = peer.agent(body, self.headers["Authorization"])
                    else:
                        assert self.path in ("/", "/comet")
                        assert set(body) == {"jsonrpc", "id", "method", "params"}
                        assert body["jsonrpc"] == "2.0"
                        result = {"jsonrpc": "2.0", "id": body["id"],
                                  "result": peer.dispatch(body["method"], body["params"])}
                        status = 200
                    encoded = json.dumps(result).encode()
                    self.send_response(status)
                    self.send_header("Content-Type", "application/json")
                    self.send_header("Content-Length", str(len(encoded)))
                    self.end_headers()
                    self.wfile.write(encoded)
                except Exception as error:
                    peer.errors.append(error)
                    self.send_error(500)

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        worker = threading.Thread(target=server.serve_forever, daemon=True)
        worker.start()
        try:
            yield f"http://127.0.0.1:{server.server_port}"
        finally:
            server.shutdown()
            server.server_close()
            worker.join()
            assert not self.errors, self.errors

    def close(self):
        self.db.close()


def scenario(peer, transactions):
    return {"candidate": CANDIDATE, "consensus_chain_id": peer.validator.template["header"]["chain_id"],
            "validators": peer.validator.validators, "transactions": transactions}


def network(tmp_path, url, document, *, phase=None):
    source = tmp_path / "scenario.json"
    source.write_text(json.dumps(document, sort_keys=True))
    gateway = tmp_path / "gateway.json"
    gateway.write_text(json.dumps(GATEWAY))
    args = ["check_merged_transaction", "--halt-height", "9", "--merged-scenario", str(source),
            "--candidate-revision", CANDIDATE["revision"], "--candidate-sha256", CANDIDATE["binary_sha256"],
            "--candidate-version", CANDIDATE["version"], "--agent-endpoint", url,
            "--gateway-key-file", str(gateway), "--restart-evidence", str(tmp_path / "restart.json")]
    for _ in range(4):
        args.extend(("--evm", url, "--comet", url + "/comet"))
    if phase is not None:
        args.extend(("--restart-phase", phase))
    return P.Net(P.parse_args(args))


@pytest.fixture
def peer(tmp_path):
    value = TranscriptPeer(tmp_path / "peer.sqlite")
    yield value
    value.close()


def transfer(peer):
    vector = next(row for row in load("sequencer-receipt-v1.json")["cases"] if row["id"] == "V1-pax-transfer")
    raw = unhex(vector["rawTransaction"])
    tx, _ = decode_signed_transaction(raw)
    entry = peer.add(raw, 1000, sequencer=vector["receiptJson"])
    peer.put("balance", hexed(tx.to) + ":0x3e7", "0x7")
    peer.put("balance", hexed(tx.to) + ":0x3e8", hex(7 + tx.value))
    result = scenario(peer, {"transfer": entry})
    result["receipt_keys"] = {"keys": [vector["pinnedKey"]]}
    return result


def deployed_program(peer):
    fixture = load("native-program-deploy-v4.json")
    # The fixture's WASM bytes stay intact. The supported lifecycle producer encodes ABI 5.
    previous = NativeProgramDeploy.decode(bytes.fromhex(fixture["payload_hex"]))
    deploy = replace(previous, guest_abi=5, interface=None)
    data = encode_call("deploy", ("bytes32", "uint16", "uint8", "bytes32", "bytes32", "bytes", "bytes"),
                       (deploy.program_id, 5, deploy.policy, deploy.authority, deploy.new_hash, b"", deploy.wasm))
    deploy_tx = peer.add(signed_call(data), 1000)
    call = replace(decode_native_program_call(bytes.fromhex(load("native-program-call-v4.json")["payload_hex"])), guest_abi=5)
    call_tx = peer.add(signed_call(_encode_programs_call(call, 1000), nonce=2), 1001)
    query = peer.state(P.PROGRAMS, peer.programs.fn("getProgram"), [deploy.program_id],
                       [deploy.policy, deploy.authority, deploy.new_hash, 5, 1, True])
    head = peer.state(P.PROGRAMS, peer.programs.fn("getStateHead"), [deploy.program_id], [b"retained native state"])
    result = scenario(peer, {"deploy": deploy_tx, "call": call_tx})
    result["state_queries"] = [query, head]
    result["expected_state_head"] = hexed(b"retained native state")
    return result, deploy


def native_web(peer):
    recorded = json.loads((ROOT / "modules/xweb/types/testdata/api-vectors.json").read_text())["vectors"][0]
    payload = unhex(recorded["payload"])
    assert encode_xweb_api_payload(decode_xweb_api_payload(payload)) == payload
    request = peer.web.fn("request")
    requester = "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf"
    request_log = event_log(peer.web.events["XWebRequested"],
                            {"requestId": 7, "requester": requester, "kind": 3, "payload": payload,
                             "callbackGas": 100000, "paid": 50, "timeoutHeight": 1100}, 1000)
    request_tx = peer.add(signed_call(request.encode(3, payload, 100000), to=P.XWEB, value=50), 1000, [request_log])
    response = b'{"price":"12.5"}'
    digest = P.keccak256(response)
    preimage = (b"PAXEERX_WEB_V1" + b"\x01" + (125).to_bytes(32, "big")
                + bytes(12) + unhex(requester) + (7).to_bytes(8, "big") + b"\x03"
                + P.keccak256(payload) + digest + P.keccak256(response) + len(response).to_bytes(4, "big"))
    assert len(preimage) == 188
    parity, r, s = P.evm.sign_digest(2, P.keccak256(preimage))
    signature = r.to_bytes(32, "big") + s.to_bytes(32, "big") + bytes([27 + parity])
    args = [7, response, digest, len(response), [signature]]
    fulfilled = event_log(peer.web.events["XWebFulfilled"],
                          {"requestId": 7, "requester": requester, "contentDigest": digest,
                           "fullLength": len(response), "level": 0, "callback": 1, "callbackGasUsed": 42}, 1001)
    fulfil_tx = peer.add(signed_call(peer.web.fn("fulfil").encode(*args), nonce=2, to=P.XWEB), 1001, [fulfilled])
    peer.state(P.XWEB, peer.web.fn("getResult"), [7],
               [(7, response, digest, len(response), [hexed(P.evm.address_of(2))], 1001, 1, 42, 0)])
    peer.state(P.XWEB, peer.web.fn("getRequest"), [7],
               [(7, requester, 3, P.keccak256(payload), 100000, 50, 1000, 1100, 1, 0, P.ZERO_ADDRESS)])
    document = scenario(peer, {"request": request_tx, "fulfil": fulfil_tx})
    document["xweb_timeout_blocks"] = 100
    return document


def test_release_signing_without_relay_import(tmp_path):
    # Execute a fresh module load with a root at which only the supported SDK is available.
    # The retired tree is never renamed, patched or imported by this test.
    absent = tmp_path / "root"
    absent.mkdir()
    sdk = absent / "services/agent/sdk"
    sdk.mkdir(parents=True)
    (sdk / "python").symlink_to(ROOT / "services/agent/sdk/python", target_is_directory=True)
    source = Path(P.__file__).read_text()
    source = source.replace('ROOT = Path(__file__).resolve().parents[3]', f'ROOT = Path({str(absent)!r})')
    namespace = {"__file__": P.__file__, "__name__": "release_signing_without_relay"}
    exec(compile(source, P.__file__, "exec"), namespace)
    signing = namespace["evm"]
    assert signing.address_of(1).hex() == "7e5f4552091a69125d5dfcb7b8c2659029395bdf"
    assert signing.compressed_public_key(1).hex() == "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
    key = tmp_path / "key"
    key.write_text("0x" + "00" * 31 + "01")
    assert signing.read_key(key) == 1
    for invalid in ("00" * 32, "ff" * 32, "01", "01" * 33):
        key.write_text(invalid)
        with pytest.raises(ValueError):
            signing.read_key(key)
    assert "_load_evm" not in namespace
    assert "paxeer_evm" not in sys.modules


def test_recorded_legacy_transaction_wrapper_and_signing_vectors():
    recorded = load("vector-v4-committed.json")["positive"]
    raw = unhex(recorded["rawTransaction"])
    wrapped = base64.b64decode(recorded["receiptInclusion"]["Transaction"])
    P.bind_comet_transaction(wrapped, raw)
    tx, signature = decode_signed_transaction(raw)
    assert tx.recover_signer(signature).lower() == recorded["receiptJson"]["from"].lower()
    for byte in (0, len(raw) // 2, len(raw) - 1):
        changed = bytearray(raw)
        changed[byte] ^= 1
        with pytest.raises(P.CheckFailed, match="does not contain"):
            P.bind_comet_transaction(wrapped, bytes(changed))
    # EIP-155's canonical signing digest, independent of this implementation.
    tx = P.transactions.LegacyTransaction(1, 9, 20_000_000_000, 21000, bytes.fromhex("35" * 20), 10**18, b"")
    golden = json.loads((ROOT / "services/wallet/schema/attestor-api/golden/evm.review.response.json").read_text())
    assert tx.signing_digest() == unhex(golden["signing_hash"])
    assert tx.to == unhex(golden["to"]) and tx.value == int(golden["value"]) and tx.chain_id == golden["chain_id"]
    raw = P.evm.sign_transaction(int("46" * 32, 16), 1, 9, 20_000_000_000, 21000, tx.to, tx.value, b"")
    signed, signature = decode_signed_transaction(raw)
    assert signed == tx
    assert signed.recover_signer(signature).lower() == "0x9d8a62f656a8d1615c1294fd71e9cfb3e4855a4f"


def test_price_submission_preserves_recorded_vector():
    vector = json.loads((ROOT / "services/platform/hosted/feeder/tests/fixtures/cosmos_tx_vector.json").read_text())
    secret = int(vector["secret_key_hex"], 16)
    observations = [tuple(row[key] for key in ("market_id", "sequence", "index_price", "mark_price", "source_timestamp_ms"))
                    for row in vector["observations"]]
    body, auth, doc = P.submission_parts(secret, vector["chain_id"], vector["account_number"],
                                         vector["sequence"], vector["gas_limit"], observations)
    assert doc.hex() == vector["sign_doc_hex"]
    assert P.tx_raw(body, auth, bytes.fromhex(vector["signature_hex"])).hex() == vector["tx_hex"]
    assert P.evm.compressed_public_key(secret).hex() == vector["public_key_hex"]
    signed_doc, raw = P.sign_submission(secret, vector["chain_id"], vector["account_number"],
                                       vector["sequence"], vector["gas_limit"], observations)
    assert signed_doc == doc
    signature = P.pb_get(P.pb_fields(raw), 3)
    public = ec.EllipticCurvePublicKey.from_encoded_point(ec.SECP256K1(), bytes.fromhex(vector["public_key_hex"]))
    public.verify(utils.encode_dss_signature(int.from_bytes(signature[:32], "big"), int.from_bytes(signature[32:], "big")),
                  hashlib.sha256(doc).digest(), ec.ECDSA(utils.Prehashed(hashes.SHA256())))
    assert P.decode_tx(raw)["fee"] == []


def test_wallet_real_rpc_and_legacy_signing(tmp_path, peer):
    # Feed the actual wallet signer through gas/nonce discovery and receipt consumption.
    with peer.serve() as url:
        document = transfer(peer)
        net = network(tmp_path, url, document)
        wallet = P.Wallet(net, 1)
        peer.wallet_expectation = (wallet.address, P.USDL, b"real calldata", 3)
        receipt = wallet.send(P.USDL, b"real calldata", value=3)
        assert receipt["blockNumber"] == 1001 and receipt["status"] == "0x1"
        assert [body["method"] for _, _, body in peer.observed] == [
            "eth_estimateGas", "eth_chainId", "eth_getTransactionCount", "eth_gasPrice",
            "eth_sendRawTransaction", "eth_getTransactionReceipt",
        ]


def test_comet_codec_matches_committed_node_transcript():
    fixture = json.loads((ROOT / "consensus/types/testdata/light_commit_v1.json").read_text())
    assert verify_commit(fixture["header"], fixture["commit"], trusted_validators=tuple(fixture["validators"]),
                         chain_id=fixture["header"]["chain_id"]).hex() == fixture["header_hash"]


def test_merged_transaction_to_commit(tmp_path, peer):
    document = transfer(peer)
    with peer.serve() as url:
        result = P.check_merged_transaction(network(tmp_path, url, document))
    assert result["candidate"] == CANDIDATE
    assert result["committed"][0]["height"] == 1000
    assert result["committed"][0]["transaction_hash"] == hexed(P.keccak256(unhex(document["transactions"]["transfer"]["raw_transaction"])))
    methods = [body["method"] for _, _, body in peer.observed]
    assert methods.count("pax_sequencerReceipt") == 4 and methods.count("commit") == 4
    assert methods.count("tx") == 4 and methods.count("eth_sendRawTransaction") == 1
    assert all(headers["Content-Type"] == "application/json" for _, headers, _ in peer.observed)


@pytest.mark.parametrize("damage", ["receipt-height", "receipt-block", "receipt-status", "inclusion", "commit-power"])
def test_uncommitted_or_mismatched_evidence_is_refused(tmp_path, peer, damage):
    document = transfer(peer)
    peer.damage = damage
    with peer.serve() as url, pytest.raises((P.CheckFailed, ValueError)):
        P.check_merged_transaction(network(tmp_path, url, document))


def test_candidate_identity_is_checked_before_submission(tmp_path, peer):
    document = transfer(peer)
    document["candidate"] = {**CANDIDATE, "revision": "f" * 40}
    with peer.serve() as url, pytest.raises(P.CheckFailed, match="candidate identity"):
        P.check_merged_transaction(network(tmp_path, url, document))
    assert peer.db.execute("SELECT count(*) FROM submissions").fetchone()[0] == 0


def test_current_guest_deployment_and_call(tmp_path, peer):
    document, deploy = deployed_program(peer)
    with peer.serve() as url:
        result = P.check_current_guest_deployment(network(tmp_path, url, document))
    assert result["program_id"] == deploy.program_id.hex()
    assert [row["height"] for row in result["committed"]] == [1000, 1001]
    assert peer.db.execute("SELECT count(*) FROM submissions").fetchone()[0] == 2
    queries = [body["params"] for _, _, body in peer.observed if body["method"] == "eth_call"]
    assert len(queries) == 8 and all(query[1] == "0x3e9" for query in queries)


def test_guest_abi_drift_is_refused_before_submission(tmp_path, peer):
    document, deploy = deployed_program(peer)
    old = replace(deploy, guest_abi=4)
    data = encode_call("deploy", ("bytes32", "uint16", "uint8", "bytes32", "bytes32", "bytes", "bytes"),
                       (old.program_id, 4, old.policy, old.authority, old.new_hash, b"", old.wasm))
    document["transactions"]["deploy"] = peer.add(signed_call(data), 1002)
    with peer.serve() as url, pytest.raises(P.CheckFailed, match="current guest ABI"):
        P.check_current_guest_deployment(network(tmp_path, url, document))
    assert peer.db.execute("SELECT count(*) FROM submissions").fetchone()[0] == 0


def test_native_xweb_request_fulfil_and_result(tmp_path, peer):
    document = native_web(peer)
    with peer.serve() as url:
        result = P.check_native_xweb(network(tmp_path, url, document))
    assert result["request_id"] == 7 and result["candidate"] == CANDIDATE
    assert [entry["height"] for entry in result["committed"]] == [1000, 1001]
    assert peer.db.execute("SELECT count(*) FROM submissions").fetchone()[0] == 2


def test_native_xweb_cross_request_fulfilment_is_refused(tmp_path, peer):
    document = native_web(peer)
    tx, _ = decode_signed_transaction(unhex(document["transactions"]["fulfil"]["raw_transaction"]))
    fn = peer.web.fn("fulfil")
    args = list(P.decode_values(fn.inputs, tx.data[4:]))
    args[0] = 8
    document["transactions"]["fulfil"] = peer.add(signed_call(fn.encode(*args), nonce=2, to=P.XWEB), 1002)
    with peer.serve() as url, pytest.raises(P.CheckFailed, match="another request"):
        P.check_native_xweb(network(tmp_path, url, document))
    assert peer.db.execute("SELECT count(*) FROM submissions").fetchone()[0] == 1


def tenants(peer):
    result = []
    for index, name in enumerate(("tenant-a", "tenant-b"), 1):
        agent = "did:layerx:" + f"{index + 20:064x}"
        context = {"tenant": name, "agent_did": agent, "authority_ref": "authority-1",
                   "permitted_activity_types": ["1"], "expiry": "2000000000",
                   "client": "release-rehearsal", "policy_version": "1"}
        sessions = {"sessions": [{"session_id": f"{index:064x}", "token_id": f"{index + 10:064x}",
                                 "agent_did": agent, "generation": "1", "open": True,
                                 "expiry_sequence": "2000", "sequence": "1000"}]}
        row = {"tenant": name, "session_id": f"{index:064x}", "token_id": f"{index + 10:064x}",
               "generation": 1, "context": context, "expected_sessions": sessions}
        peer.put("tenant", row["session_id"], {**row, "generation": "1"})
        result.append(row)
    return result


def test_tenant_isolation_real_agent_envelope(tmp_path, peer):
    document = transfer(peer)
    document["transactions"]["anchor"] = document["transactions"].pop("transfer")
    document["tenants"] = tenants(peer)
    with peer.serve() as url:
        result = P.check_tenant_isolation(network(tmp_path, url, document))
    assert result["candidate"] == CANDIDATE and result["committed"][0]["height"] == 1000
    exchanges = [(headers, body) for path, headers, body in peer.observed if path == "/v1/agent/rpc"]
    assert len(exchanges) == 6
    assert all(headers["Authorization"] == "LayerX-Key rehearsal:" + GATEWAY["secret"] for headers, _ in exchanges)
    assert [body["credential"]["tenant"] for _, body in exchanges] == ["tenant-a", "tenant-b", "tenant-a", "tenant-b", "tenant-a", "tenant-b"]


def test_tenant_isolation_refuses_leaked_sessions(tmp_path, peer):
    document = transfer(peer)
    document["transactions"]["anchor"] = document["transactions"].pop("transfer")
    document["tenants"] = tenants(peer)
    row = peer.get("tenant", document["tenants"][0]["session_id"])
    row["expected_sessions"] = document["tenants"][1]["expected_sessions"]
    peer.put("tenant", row["session_id"], row)
    with peer.serve() as url, pytest.raises(P.CheckFailed, match="unexpected sessions"):
        P.check_tenant_isolation(network(tmp_path, url, document))


def test_persisted_restart_closes_and_reopens_peer_and_driver(tmp_path):
    path = tmp_path / "peer.sqlite"
    before = TranscriptPeer(path)
    document, _ = deployed_program(before)
    with before.serve() as url:
        original = P.check_persisted_restart(network(tmp_path, url, document, phase="before"))
    counts = before.db.execute("SELECT hash,count FROM submissions ORDER BY hash").fetchall()
    before.close()
    evidence_bytes = (tmp_path / "restart.json").read_bytes()
    assert (tmp_path / "restart.json").stat().st_mode & 0o777 == 0o600
    after = TranscriptPeer(path)
    try:
        with after.serve() as url:
            recovered = P.check_persisted_restart(network(tmp_path, url, document, phase="after"))
        assert recovered == original
        assert after.db.execute("SELECT hash,count FROM submissions ORDER BY hash").fetchall() == counts
        assert (tmp_path / "restart.json").read_bytes() == evidence_bytes
        assert not any(body.get("method") == "eth_sendRawTransaction" for _, _, body in after.observed)
        assert recovered["candidate"] == CANDIDATE
        assert len(recovered["state"]) == 2 and recovered["committed"][-1]["height"] == 1001
    finally:
        after.close()


def test_restart_refuses_changed_persisted_state(tmp_path, peer):
    document, _ = deployed_program(peer)
    with peer.serve() as url:
        P.check_persisted_restart(network(tmp_path, url, document, phase="before"))
    peer.damage = "state"
    with peer.serve() as url, pytest.raises(P.CheckFailed, match="persisted native state"):
        P.check_persisted_restart(network(tmp_path, url, document, phase="after"))


def test_registry_preserves_market_checks_and_exposes_merged_checks():
    market = {
        "check_launchpad_interface", "check_usdl_erc20_custody", "check_pool_parity",
        "check_fee_right_and_pool_token", "check_perps_order_types", "check_perps_margin",
        "check_perps_economics", "check_perps_markets", "check_perps_log_rebuild",
        "check_spot_interface", "check_spot_settlement", "check_spot_logs", "check_sidecar_submissions",
        "check_price_freshness", "check_price_submission_bounds", "check_plan_and_version",
    }
    merged = {
        "check_merged_transaction", "check_current_guest_deployment", "check_native_xweb",
        "check_tenant_isolation", "check_persisted_restart",
    }
    assert set(P.CHECKS) == market | merged
    for name in merged:
        assert callable(P.CHECKS[name])
        assert P.parse_args([name, "--halt-height", "9"]).check == name
    assert set(TASK_RECORD["findings"]) == {"programme-retired-helper-import", "programme-merged-lifecycle-gap"}
    for names in TASK_RECORD["findings"].values():
        assert all(callable(globals()[name]) for name in names)


def test_cli_driver_emits_candidate_bound_evidence(tmp_path, peer, capsys):
    document = transfer(peer)
    with peer.serve() as url:
        net = network(tmp_path, url, document)
        args = ["check_merged_transaction", "--halt-height", "9", "--merged-scenario", net.args.merged_scenario,
                "--candidate-revision", CANDIDATE["revision"], "--candidate-sha256", CANDIDATE["binary_sha256"],
                "--candidate-version", CANDIDATE["version"]]
        for _ in range(4):
            args.extend(("--evm", url, "--comet", url + "/comet"))
        assert P.main(args) == 0
    evidence = json.loads(capsys.readouterr().out)
    assert evidence["check"] == "check_merged_transaction"
    assert evidence["candidate"] == CANDIDATE
    assert evidence["committed"][0]["height"] == 1000


def test_native_xweb_wrong_attestor_binding_is_refused(tmp_path, peer):
    document = native_web(peer)
    identity = hexed(P.keccak256(unhex(document["transactions"]["fulfil"]["raw_transaction"])))
    # Alter only the result's attestor identity; all signed transactions and commits stay valid.
    fn = peer.web.fn("getResult")
    encoded = peer.get("call", P.XWEB + hexed(fn.encode(7)))
    result = list(P.decode_values(fn.outputs, unhex(encoded))[0])
    result[4] = [hexed(P.evm.address_of(3))]
    peer.state(P.XWEB, fn, [7], [result])
    assert peer.get("receipt", identity)["status"] == "0x1"
    with peer.serve() as url, pytest.raises(P.CheckFailed, match="attestors are not bound"):
        P.check_native_xweb(network(tmp_path, url, document))


def test_merged_transaction_tampered_sequencer_receipt_is_refused(tmp_path, peer):
    document = transfer(peer)
    identity = hexed(P.keccak256(unhex(document["transactions"]["transfer"]["raw_transaction"])))
    receipt = peer.get("sequencer", identity)
    raw = bytearray(unhex(receipt["receipt"]))
    raw[-1] ^= 1
    receipt["receipt"] = hexed(raw)
    peer.put("sequencer", identity, receipt)
    with peer.serve() as url, pytest.raises(P.PlatformSdkError):
        P.check_merged_transaction(network(tmp_path, url, document))
