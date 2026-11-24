"""Recorded native RPC peers for the optional shell transport integration.

The component under test remains upgrade-fleet.sh and the installed inspector.
Each unit gets independent loopback node/Comet transports and an advancing
in-memory canonical head. This is peer state, not a native deployment claim.
"""
import copy
import json
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

capture, output = map(Path, sys.argv[1:])
exchanges = {row["method"]: row["result"] for row in json.loads(capture.read_text())["rpc_exchanges"]}
servers = []
endpoints = []


def handler(owner, state):
    class Peer(BaseHTTPRequestHandler):
        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            method = request["method"]
            expected = "comet" if method in ("status", "commit") else "node"
            if self.path != "/" or expected != owner or method not in exchanges:
                self.send_error(400)
                return
            with state["lock"]:
                if method == "eth_blockNumber":
                    state["height"] += 1
                height = state["height"]
                result = copy.deepcopy(exchanges[method])
                if method == "eth_blockNumber":
                    result = hex(height)
                elif method == "status":
                    result["sync_info"]["latest_block_height"] = str(height)
                elif method == "commit":
                    if request["params"] != {"height": str(height)}:
                        self.send_error(400)
                        return
                    result["signed_header"]["header"]["height"] = str(height)
                    result["signed_header"]["commit"]["height"] = str(height)
            body = json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": result}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_args):
            pass

    return Peer


for unit in range(3):
    state = {"height": 23869863, "lock": threading.Lock()}
    urls = []
    for owner in ("node", "comet"):
        server = ThreadingHTTPServer(("127.0.0.1", 0), handler(owner, state))
        servers.append(server)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        urls.append(f"http://127.0.0.1:{server.server_port}")
    endpoints.append(" ".join(urls))
output.write_text("\n".join(endpoints) + "\n")
threading.Event().wait()
