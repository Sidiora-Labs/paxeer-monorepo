#!/usr/bin/env python3
"""Native dry run. BUNDLE.json names a disposable provisioned node home,
paxd and sidecar executables, receipt_key, actor_key, consumer_bytecode (forge
creation bytecode), endpoint, gateway_endpoint, assets and sidecars. Each sidecar
entry supplies attestor_key, receiver_key and submitter_key. Keys must already
be funded/registered by the native-chain fixture owner. All endpoints are local;
the home is copied and only the copy is started. No chain state is fabricated.
"""

from __future__ import annotations

import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from urllib.error import HTTPError
from urllib.parse import quote, urlsplit
from urllib.request import ProxyHandler, Request, build_opener

ROOT = Path(__file__).resolve().parents[5]
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import _rlp, _rlp_integer, encode_call
from layerx_sdk.verifier import verify_sequencer_receipt
from layerx_sdk.x402 import verify_payment_receipt
from layerx_sdk.x402_rpc import PaymentRpc

PRECOMPILE = "0x" + "00" * 18 + "1019"
LEDGER = "0x" + "00" * 18 + "101a"
OPENER = build_opener(ProxyHandler({}))


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def unhex(value):
    return bytes.fromhex(value.removeprefix("0x"))


def native_windows(genesis):
    return [dict(key_id=base64.b64decode(key["key_id"], validate=True).hex(),
                 public_key=base64.b64decode(key["public_key"], validate=True).hex(),
                 first_height=int(key["first_height"]), last_height=int(key["last_height"]))
            for key in genesis["app_state"]["sequencer"]["receipt_keys"]]


def sidecar_settings(template, windows, endpoint, gateway, listen, data, site, peers, assets):
    config = copy.deepcopy(template)
    config.pop("kernel_network_id", None)
    config.update(listen=listen, data_dir=str(data), seeds=[site], peers=peers,
                  crawl_interval_seconds=3600, assets=assets)
    config["fetch"]["allow_loopback"] = True
    config["evm"].update(endpoint=endpoint, confirmations=1)
    config["gateway"] = {"endpoint": gateway, "receipt_keys": copy.deepcopy(windows)}
    return config


def receipt_digest(receipt):
    # Match canonical_receipt_digest: replace the signed option (flag, length,
    # signature) with the unsigned option's zero flag, preserving the full body.
    require(len(receipt) >= 69 and receipt[-69:-64] == b"\x01\x00\x00\x00\x40", "receipt signature framing")
    return hashlib.sha256(b"LXP/v1/receipt\0" + receipt[:-69] + b"\x00").hexdigest()


def http_get(url, headers=None):
    try:
        response = OPENER.open(Request(url, headers=headers or {}), timeout=15)
    except HTTPError as response:
        return response.code, {k.lower(): v for k, v in response.headers.items()}, response.read()
    with response:
        return response.status, {k.lower(): v for k, v in response.headers.items()}, response.read()


class OwnedProcess:
    def __init__(self, argv, log, env=None):
        self.argv, self.log, self.env = argv, Path(log), env
        self.process = None

    def start(self):
        require(self.process is None, "process already started")
        with self.log.open("ab") as stream:
            self.process = subprocess.Popen(self.argv, cwd=ROOT, env=self.env,
                                            stdin=subprocess.DEVNULL, stdout=stream, stderr=stream)

    def stop(self):
        require(self.process is not None, "process not started")
        self.process.terminate()
        self.process.wait(timeout=30)
        self.process = None


def wait(probe, processes, label, seconds=150):
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        require(all(item.process is not None and item.process.poll() is None for item in processes),
                "owned process exited while waiting for " + label)
        answer = probe()
        if answer:
            return answer
        time.sleep(0.25)
    raise AssertionError("timed out: " + label)


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


class NativeScenario:
    def __init__(self, bundle, work):
        self.bundle, self.work = bundle, Path(work)
        for field in ("endpoint", "gateway_endpoint"):
            parsed = urlsplit(bundle[field])
            require(parsed.scheme == "http" and parsed.hostname == "127.0.0.1"
                    and not parsed.username and not parsed.password, "native endpoints must be loopback")
        self.rpc = PaymentRpc(bundle["gateway_endpoint"])
        self.rpc_id = 0
        self.processes, self.sidecars = [], []
        self.home = self.work / "node"
        shutil.copytree(bundle["home"], self.home)
        self.genesis = json.loads((self.home / "config/genesis.json").read_text())
        self.windows = native_windows(self.genesis)
        self.chain_id = bundle["chain_id"]
        require(type(self.chain_id) is int and self.chain_id > 0, "invalid chain id")
        import tomllib
        consensus = tomllib.loads((self.home / "config/config.toml").read_text())
        require(not consensus["p2p"]["persistent_peers"] and not consensus["p2p"]["seeds"],
                "native fixture must have no external consensus peers")
        self.node = OwnedProcess([bundle["paxd"], "start", "--home", str(self.home),
                                  "--sequencer.receipt-key-file", bundle["receipt_key"]], self.work / "paxd.log")
        self.processes.append(self.node)

    def call(self, method, params):
        # Reads and writes go directly to the owned paxd; there is no Front.
        from layerx_sdk.x402_rpc import PaymentRpcError
        self.rpc_id += 1
        request = Request(self.bundle["endpoint"], data=json.dumps(
            {"jsonrpc": "2.0", "id": self.rpc_id, "method": method, "params": params}).encode(),
            headers={"Content-Type": "application/json"})
        with OPENER.open(request, timeout=30) as response:
            answer = json.load(response)
        require(answer.get("jsonrpc") == "2.0" and answer.get("id") == self.rpc_id,
                "native JSON-RPC envelope mismatch")
        if "error" in answer:
            raise PaymentRpcError(answer["error"]["code"], answer["error"].get("data"))
        return answer["result"]

    def view(self, address, method, kinds=(), values=()):
        data = encode_call(method, kinds, values)
        return unhex(self.call("eth_call", [{"to": address, "data": "0x" + data.hex()}, "latest"]))

    def send(self, address, data, value=0, *, duplicate=False):
        from eth_keys import keys

        key = keys.PrivateKey(unhex(Path(self.bundle["actor_key"]).read_text().strip()))
        owner = "0x" + key.public_key.to_canonical_address().hex()
        nonce = int(self.call("eth_getTransactionCount", [owner, "pending"]), 16)
        price = int(self.call("eth_gasPrice", []), 16)
        fields = (_rlp_integer(nonce), _rlp_integer(price), _rlp_integer(5_000_000),
                  unhex(address), _rlp_integer(value), data)
        unsigned = _rlp(fields + (_rlp_integer(self.chain_id), b"", b""))
        signature = key.sign_msg_hash(keccak256(unsigned))
        raw = _rlp(fields + (_rlp_integer(self.chain_id * 2 + 35 + signature.v),
                            _rlp_integer(signature.r), _rlp_integer(signature.s)))
        if duplicate:
            from layerx_sdk.x402_rpc import PaymentRpcError
            try:
                transaction = self.call("eth_sendRawTransaction", ["0x" + raw.hex()])
            except PaymentRpcError as error:
                detail = error.data
                if isinstance(detail, dict):
                    detail = detail.get("data")
                require(isinstance(detail, str), "duplicate returned no revert data")
                reason = unhex(detail) if detail.startswith("0x") else detail.encode()
                require(b"LXP_XWEB_ALREADY_FULFILLED" in reason, "unexpected duplicate refusal")
                return None, None, raw
            receipt = wait(lambda: self.call("eth_getTransactionReceipt", [transaction]),
                           self.processes, "duplicate transaction")
            require(receipt["status"] == "0x0" and not receipt["logs"], "duplicate fulfil did not revert atomically")
        else:
            transaction = self.rpc.send("0x" + raw.hex())
            receipt = wait(lambda: self.rpc.status(transaction), self.processes, "transaction receipt")
            require(receipt["status"] == "0x1", "native transaction reverted")
        return transaction, receipt, raw

    def start(self):
        self.node.start()
        def ready():
            try:
                return self.call("eth_chainId", []) == hex(self.chain_id)
            except OSError:
                return False
        wait(ready, self.processes, "paxd")
        pins = self.rpc.receipt_keys()
        observed = [dict(key_id=k.key_id.hex(), public_key=k.public_key.hex(),
                         first_height=k.first_height, last_height=k.last_height) for k in pins]
        require(observed == self.windows, "node keys differ from the native genesis trust windows")
        self.pins = pins
        from eth_keys import keys
        members = self.view(PRECOMPILE, "getAttestors")
        word = lambda start: int.from_bytes(members[start:start+32], "big")
        base = word(0)
        count = word(base)
        signers = [members[base+32+word(base+32+index*32)+12:base+32+word(base+32+index*32)+32]
                   for index in range(count)]
        expected = [keys.PrivateKey(unhex(Path(slot["attestor_key"]).read_text().strip())).public_key.to_canonical_address()
                    for slot in self.bundle["sidecars"]]
        require(sorted(signers) == sorted(expected), "sidecar keys differ from native attestor membership")
        require(0 < word(32) <= len(expected), "invalid native attestor threshold")
        self.server = ThreadingHTTPServer(("127.0.0.1", 0),
                                          lambda *a, **kw: SimpleHTTPRequestHandler(*a, directory=str(Path(__file__).parent / "fixtures/site"), **kw))
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.site = f"http://127.0.0.1:{self.server.server_port}/index.html"
        template = json.loads((ROOT / "services/interop/crates/x-websearch/tests/fixtures/config/valid.json").read_text())
        ports = [free_port() for _ in self.bundle["sidecars"]]
        urls = [f"http://127.0.0.1:{port}" for port in ports]
        for index, slot in enumerate(self.bundle["sidecars"]):
            data = self.work / f"sidecar-{index}"
            data.mkdir(mode=0o700)
            config = sidecar_settings(template, self.windows, self.bundle["endpoint"],
                                      self.bundle["gateway_endpoint"], f"127.0.0.1:{ports[index]}",
                                      data, self.site, [url for url in urls if url != urls[index]], self.bundle["assets"])
            config["evm"]["chain_id"] = self.chain_id
            path = self.work / f"config-{index}.json"
            path.write_text(json.dumps(config))
            env = {k: v for k, v in os.environ.items() if not k.startswith("X_WEBSEARCH_")}
            for role in ("attestor", "receiver"):
                env[f"X_WEBSEARCH_{role.upper()}_KEY_FILE"] = slot[role + "_key"]
            # Start without submitters so observed work is durably pending.
            process = OwnedProcess([self.bundle["sidecar"], "--config", str(path)], self.work / f"sidecar-{index}.log", env)
            process.start()
            self.processes.append(process)
            self.sidecars.append((process, data, urls[index], slot))
        # Consumer creation is a real signed deployment, never injected storage.
        bytecode = unhex(Path(self.bundle["consumer_bytecode"]).read_text().strip())
        _, receipt, _ = self.send("0x", bytecode)
        self.consumer = receipt["contractAddress"]
        require(len(self.call("eth_getCode", [self.consumer, "latest"])) > 2, "consumer was not deployed")

    def request_state(self, request_id):
        return self.view(PRECOMPILE, "getRequest", ("uint64",), (request_id,))

    def callbacks(self, request_id, low):
        return self.call("eth_getLogs", [{"address": self.consumer, "fromBlock": low,
                         "toBlock": "latest", "topics": ["0x" + keccak256(b"Answered(uint64,bytes32,uint32,bytes)").hex(),
                                                           "0x" + request_id.to_bytes(32, "big").hex()]}])

    def lifecycle(self):
        fee = int.from_bytes(self.view(PRECOMPILE, "fee"), "big")
        _, receipt, _ = self.send(self.consumer, encode_call("ask", ("uint8", "bytes", "uint64"),
                                                            (1, self.site.encode(), 400_000)), fee)
        topic = "0x" + keccak256(b"XWebRequested(uint64,address,uint8,bytes,uint64,uint256,uint64)").hex()
        logs = [log for log in receipt["logs"] if log["address"].lower() == PRECOMPILE and log["topics"][0] == topic]
        require(len(logs) == 1, "request did not traverse 0x1019")
        request_id = int(logs[0]["topics"][1], 16)
        before = self.request_state(request_id)
        require(int.from_bytes(before[8*32:9*32], "big") == 0, "request is not pending")
        def observed():
            return all((data / "watch/pending.json").is_file()
                       and any(entry["request"]["request_id"] == request_id for entry in json.loads((data / "watch/pending.json").read_text())["entries"])
                       for _, data, _, _ in self.sidecars)
        wait(observed, self.processes, "sidecar durable pending work")
        def attestations():
            try:
                records = [self.attestation(url, request_id) for _, _, url, _ in self.sidecars]
                return records if all(records) else None
            except OSError:
                return None
        records = wait(attestations, self.processes, "sidecar attestations before restart")
        for process, _, _, _ in self.sidecars:
            process.stop()
        self.node.stop()
        # Reopen the same home and data directories, retaining pending journals.
        self.node.start()
        def recovered():
            try:
                return self.request_state(request_id) == before
            except OSError:
                return False
        wait(recovered, [self.node], "recovered native pending request")
        for process, _, _, _ in self.sidecars:
            process.start()
        restored = wait(attestations, self.processes, "recovered sidecar attestations")
        require(restored == records, "sidecar reopen changed retained attestation state")
        require(self.request_state(request_id) == before, "pending work completed before submitters resumed")
        for process, _, _, slot in self.sidecars:
            process.stop()
            process.env["X_WEBSEARCH_SUBMITTER_KEY_FILE"] = slot["submitter_key"]
            process.start()
        wait(lambda: self.callbacks(request_id, receipt["blockNumber"]), self.processes, "module callback")
        after = self.request_state(request_id)
        require(int.from_bytes(after[8*32:9*32], "big") == 1, "module did not mark request fulfilled")
        result = self.view(PRECOMPILE, "getResult", ("uint64",), (request_id,))[32:]
        require(int.from_bytes(result[6*32:7*32], "big") == 1, "native callback was not delivered")
        require(0 < int.from_bytes(result[7*32:8*32], "big") <= 400_000, "callback gas exceeded request budget")
        answer = self.view(self.consumer, "answer", ("uint64",), (request_id,))
        require(int.from_bytes(answer[32:64], "big") == 1, "consumer stored no answer")
        require(answer[64:96] == result[2*32:3*32], "consumer digest differs from native result")
        require(answer[96:128] == result[96:128], "consumer full length differs from native result")
        # A second signed fulfil reaches the module with the real attestations.
        response_at = int.from_bytes(result[32:64], "big")
        length = int.from_bytes(result[response_at:response_at+32], "big")
        response = result[response_at+32:response_at+32+length]
        answer_body = answer[32:]
        answer_at = int.from_bytes(answer_body[96:128], "big")
        answer_length = int.from_bytes(answer_body[answer_at:answer_at+32], "big")
        require(answer_body[answer_at+32:answer_at+32+answer_length] == response, "consumer response differs from module result")
        signatures = sorted((unhex(record["signature"]) for record in records), key=lambda sig: self.signer(sig, records[0]))
        calldata = encode_call("fulfil", ("uint64", "bytes", "bytes32", "uint32", "bytes[]"),
                               (request_id, response, result[64:96], int.from_bytes(result[96:128], "big"), signatures))
        self.send(PRECOMPILE, calldata, duplicate=True)
        # Reopen the sidecars once more to exercise redelivery from retained state.
        for process, _, _, _ in self.sidecars:
            process.stop()
            process.start()
        height = int(self.call("eth_blockNumber", []), 16)
        wait(lambda: int(self.call("eth_blockNumber", []), 16) >= height + 3, self.processes, "redelivery blocks")
        require(len(self.callbacks(request_id, receipt["blockNumber"])) == 1, "consumer callback count differs from one")
        require(self.view(self.consumer, "answer", ("uint64",), (request_id,)) == answer, "recovery changed consumer answer")
        return {"request_id": request_id, "pending_recovered": True, "callbacks": 1}

    def attestation(self, url, request_id):
        code, _, body = http_get(f"{url}/attestations/{request_id}")
        return json.loads(body) if code == 200 else None

    @staticmethod
    def signer(signature, record):
        from eth_keys import keys
        sig = keys.Signature(signature[:64] + bytes([signature[64] - 27]))
        return sig.recover_public_key_from_msg_hash(unhex(record["digest"])).to_canonical_address()

    def payment(self):
        url = self.sidecars[0][2] + "/fetch?url=" + quote(self.site, safe="")
        from eth_keys import keys
        actor = keys.PrivateKey(unhex(Path(self.bundle["actor_key"]).read_text().strip()))
        payer_address = "0x" + actor.public_key.to_canonical_address().hex()
        headers = {"LAYERX-PAYER-ADDRESS": payer_address}
        code, challenge_headers, _ = http_get(url, headers)
        require(code == 402, "unpaid request was not challenged")
        required = json.loads(base64.b64decode(challenge_headers["payment-required"], validate=True))
        offer = next(offer for offer in required["accepts"] if offer["scheme"] == "exact" and offer["extra"]["layerx"]["currency"] == "USDC")
        asset, payee = unhex(offer["asset"]), unhex(offer["payTo"])
        payer = self.view(LEDGER, "accountId", ("bytes32", "bytes32"), (bytes(12) + actor.public_key.to_canonical_address(), asset))
        reference = os.urandom(32)
        data = encode_call("payExact", ("bytes32", "bytes32", "bytes32", "uint128", "uint64", "bytes32"),
                           (payer, payee, asset, int(offer["amount"]), int(time.time()*1000)+60_000, reference))
        transaction, _, raw = self.send(LEDGER, data)
        def receipted():
            result = self.rpc.receipt(transaction)
            return result if result is not None and result["rung"] in ("receipted", "committed") else None
        evidence = wait(receipted, self.processes, "payment receipt")
        signed = unhex(evidence["receipt"])
        verified = verify_payment_receipt(signed, raw, self.pins, amount=offer["amount"],
                                         asset=asset.hex(), pay_to=payee.hex(), payer=payer.hex())
        digest = receipt_digest(signed)
        payload = {"x402Version": 2, "accepted": offer, "resource": required["resource"],
                   "payload": {"txHash": transaction, "receiptDigest": digest,
                               "verificationLevel": "receipted", "idempotencyKey": reference.hex()}}
        headers["PAYMENT-SIGNATURE"] = base64.b64encode(json.dumps(payload).encode()).decode()
        code, response_headers, body = http_get(url, headers)
        require(code == 200, f"real relay payment refused: {code}")
        settlement = json.loads(base64.b64decode(response_headers["payment-response"], validate=True))
        returned = base64.b64decode(settlement["extensions"]["layerx"]["receipt"], validate=True)
        require(returned == signed, "sidecar returned a different receipt")
        checked = verify_sequencer_receipt(returned, raw, self.pins)
        require(checked.height == verified.height, "response receipt height changed")
        require(settlement["success"] and settlement["transaction"] == transaction, "settlement transaction mismatch")
        binding = settlement["extensions"]["x-websearch"]
        require(binding["receiptDigest"] == digest and binding["responseDigest"] == hashlib.sha256(body).hexdigest(), "response is not bound to paid receipt")
        # Reopen the production payment store through the sidecar and replay the
        # same paid request. The stored response must survive without a new debit.
        process = self.sidecars[0][0]
        process.stop()
        process.start()
        def replay():
            try:
                return http_get(url, headers)
            except OSError:
                return None
        replayed = wait(replay, self.processes, "paid response after reopen")
        replay_code, replay_headers, replay_body = replayed
        require(replay_code == 200 and replay_body == body, "recovered paid response changed")
        require(replay_headers["payment-response"] == response_headers["payment-response"], "recovered settlement changed")
        return {"transaction": transaction, "height": checked.height, "receipt_digest": digest}

    def close(self):
        for process in reversed(self.processes):
            if process.process is not None:
                process.stop()
        if hasattr(self, "server"):
            self.server.shutdown()
            self.server.server_close()


def main():
    bundle = json.loads(Path(sys.argv[1]).read_text())
    with tempfile.TemporaryDirectory(prefix="xweb-native-") as work:
        scenario = NativeScenario(bundle, work)
        try:
            scenario.start()
            result = {"lifecycle": scenario.lifecycle(), "payment": scenario.payment()}
            print(json.dumps(result, sort_keys=True))
        finally:
            scenario.close()


if __name__ == "__main__":
    main()
