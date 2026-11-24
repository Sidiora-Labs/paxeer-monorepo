#!/usr/bin/env python3
"""Loopback transport peers for the standalone shell harness.

Bringup's inspector, issuer, renderer, custody and SSH payload execute unchanged.
These peers speak the committed registry and event-admission contracts; they
never emulate bringup decisions and never contact a deployed service.
"""
import http.server
import json
import pathlib
import ssl
import sys
import threading
import time

mode, directory, identity = sys.argv[1:4]
state = pathlib.Path(directory)
identity = pathlib.Path(identity)
state.mkdir(parents=True, exist_ok=True)
program = "c" * 64
code = "07" + "00" * 31
event = {
    "address": "0x000000000000000000000000000000000000101d",
    "topics": ["0x2765c1020c93cab47e389edaa5b5c68834bdd76f7a5d384adb460fc86e688709", "0x" + program],
    "data": "0x" + code,
    "blockNumber": "0x101", "blockHash": "0x" + "12" * 32,
    "transactionHash": "0x" + "56" * 32, "transactionIndex": "0x0",
    "logIndex": "0x0", "removed": False,
}


def setting(name, default=""):
    path = state / name
    return path.read_text().strip() if path.exists() else default


class Peer(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def send(self, status, value):
        data = json.dumps(value).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if mode == "registry":
            if self.path == "/healthz":
                self.send(503 if setting("ready", "true") != "true" else 200,
                          {"status": "ready", "service": "program-registry"})
                return
            if self.path != "/v1/programs/registry/" + program or self.headers.get("Authorization") != "Bearer " + setting("token"):
                self.send(401, {"error": "unauthorized"})
                return
            now = int(time.time() * 1000)
            self.send(200, {
                "program_id": program, "lifecycle": "active", "observed_sequence": 300,
                "observed_at": now, "valid_through": now + 10000,
                "versions": [{"version": 1, "code_hash": code, "source": {"status": "verified"}}],
                "receipt": {"verification": "receipt-verified", "deployment_receipt_digest": "a" * 64},
            })
            return
        if mode == "internal":
            ready = setting("ready", "true") == "true"
            self.send(200 if ready else 503, {"ready": ready})
            return
        if self.headers.get("Authorization") != "Bearer " + setting("token"):
            self.send(401, {"error": "unauthorized"})
            return
        kind = sys.argv[4] if mode == "source" else self.path.rsplit("/", 1)[-1]
        path = "/internal/v1/producer-readiness" if mode == "source" else "/internal/v1/readiness/" + kind
        if self.path != path or kind not in {"journey", "approval", "payment", "program"}:
            self.send(404, {"error": "not_found"})
            return
        self.send(200, {"schema": "layerx.event-admission.v1",
                        "role": "source-producer" if mode == "source" else "webhook-trigger",
                        "kind": kind, "ready": True, "generation": 1, "principals": 1,
                        "principal_digest": mode == "source" and kind in {"payment", "program"},
                        "fresh_for_ms": 10000})


class Chain(Peer):
    def do_POST(self):
        request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        method, params = request["method"], request["params"]
        with (state / "chain-calls").open("a") as output:
            output.write(json.dumps(request) + "\n")
        if method == "eth_chainId":
            result = "0x7d"
        elif method == "eth_blockNumber":
            result = "0x12c"
        elif method == "eth_getLogs":
            page = params[0]
            result = [event] if int(page["fromBlock"], 16) <= 257 <= int(page["toBlock"], 16) and page["topics"][1] == "0x" + program else []
        elif method == "eth_getBlockByNumber":
            result = {"hash": event["blockHash"], "number": "0x101", "transactions": [event["transactionHash"]]}
        elif method == "eth_getTransactionReceipt":
            result = {"status": "0x1", "transactionHash": event["transactionHash"],
                      "blockHash": event["blockHash"], "blockNumber": "0x101", "logs": [event]}
        else:
            self.send(400, {"error": "unexpected_method"})
            return
        self.send(200, {"jsonrpc": "2.0", "id": request["id"], "result": result})


server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Peer)
context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
context.minimum_version = ssl.TLSVersion.TLSv1_2
context.load_cert_chain(identity / "cert.pem", identity / "key.pem")
context.load_verify_locations(identity / "ca.pem")
context.verify_mode = ssl.CERT_REQUIRED
server.socket = context.wrap_socket(server.socket, server_side=True)
(state / "port").write_text(str(server.server_port))
if mode == "registry":
    chain = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Chain)
    (state / "chain-port").write_text(str(chain.server_port))
    threading.Thread(target=chain.serve_forever, daemon=True).start()
server.serve_forever()
