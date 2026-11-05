import argparse
import copy
import http.client
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import ssl
import stat
import subprocess
import threading
from urllib.parse import urlsplit

from journey_common import root_endpoint


class WalletReceiptProxy:
    def __init__(self, endpoint, ca, certificate, key):
        self.corrupt = False
        self.mutations = []
        self.rpc_failures = []
        self.failures = []
        self.forwarded_paths = []
        endpoint = urlsplit(root_endpoint(endpoint))
        assert endpoint.scheme == "https" and endpoint.hostname == "localhost"
        upstream_context = ssl.create_default_context(cafile=str(ca))
        server_context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        server_context.load_cert_chain(str(certificate), str(key))
        state = self

        class Proxy(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *_args):
                pass

            def do_POST(self):
                if self.path != "/":
                    state.failures.append("wallet must post to the node root")
                    self.send_error(404)
                    return
                length = int(self.headers["Content-Length"])
                assert 0 < length <= 2097152
                body = self.rfile.read(length)
                rpc_method = json.loads(body)["method"]
                headers = {
                    key: value
                    for key, value in self.headers.items()
                    if key.lower() not in ("host", "connection", "content-length")
                }
                upstream = http.client.HTTPSConnection(
                    endpoint.hostname, endpoint.port, context=upstream_context, timeout=60
                )
                try:
                    state.forwarded_paths.append((self.path, endpoint.path))
                    assert self.path == endpoint.path == "/"
                    upstream.request("POST", endpoint.path, body, headers)
                    answer = upstream.getresponse()
                    result = answer.read(9 * 1048576 + 1)
                    assert len(result) <= 9 * 1048576
                    value = json.loads(result)
                    if (value.get("error") or {}).get("code") == -32601:
                        state.rpc_failures.append(rpc_method)
                    if state.corrupt and rpc_method == "pax_sequencerReceipt":
                        payload = value.get("result")
                        receipt = (
                            payload.get("receipt") if isinstance(payload, dict) else None
                        )
                        if isinstance(receipt, str) and receipt:
                            prefix = "0x" if receipt.startswith("0x") else ""
                            changed = bytearray.fromhex(receipt[len(prefix) :])
                            assert changed, "sequencer receipt is empty"
                            changed[-1] ^= 1
                            value["result"]["receipt"] = prefix + changed.hex()
                            result = json.dumps(value).encode()
                            state.mutations.append(True)
                    self.send_response(answer.status)
                    self.send_header("Content-Type", "application/json")
                    self.send_header("Content-Length", str(len(result)))
                    self.send_header("Connection", "close")
                    self.end_headers()
                    self.wfile.write(result)
                    self.wfile.flush()
                    self.close_connection = True
                finally:
                    upstream.close()

            def finish(self):
                super().finish()
                try:
                    self.connection.unwrap().close()
                except (ssl.SSLError, OSError):
                    self.connection.close()

        server = ThreadingHTTPServer(("127.0.0.1", 0), Proxy)
        server.socket = server_context.wrap_socket(server.socket, server_side=True)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.server, self.thread = server, thread
        self.endpoint = f"https://localhost:{server.server_port}/"

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)
        assert not self.thread.is_alive(), "wallet proxy did not stop"
        assert not self.failures, self.failures


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--wallet", type=Path, required=True)
    parser.add_argument("--request", type=Path, required=True)
    parser.add_argument("--ca", type=Path, required=True)
    parser.add_argument("--certificate", type=Path, required=True)
    parser.add_argument("--key", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    descriptor = os.open(args.request, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, "rb") as handle:
        info = os.fstat(handle.fileno())
        assert stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
        assert stat.S_IMODE(info.st_mode) == 0o600 and info.st_nlink == 1
        request = json.load(handle)
    outcomes = []
    with WalletReceiptProxy(
        request["configuration"]["endpoint"], args.ca, args.certificate, args.key
    ) as proxy:
        request["configuration"]["endpoint"] = proxy.endpoint
        for commitment in ["executed", "batched"]:
            current = copy.deepcopy(request)
            current["commitment"] = commitment
            for corrupt in [False, True]:
                proxy.corrupt = corrupt
                try:
                    result = subprocess.run(
                        [str(args.wallet)],
                        input=json.dumps(current).encode(),
                        stdout=subprocess.PIPE,
                        stderr=subprocess.PIPE,
                        timeout=120,
                        check=False,
                    )
                except subprocess.TimeoutExpired:
                    if proxy.rpc_failures:
                        raise RuntimeError(
                            "JSON-RPC -32601 method not found: "
                            + ", ".join(proxy.rpc_failures)
                        ) from None
                    raise
                if proxy.rpc_failures:
                    raise RuntimeError(
                        "JSON-RPC -32601 method not found: " + ", ".join(proxy.rpc_failures)
                    )
                assert result.returncode == 0
                value = json.loads(result.stdout)
                assert value["ok"] is (not corrupt), value
                if not corrupt:
                    assert value["result"]["activity_id"] == current["activity_id"]
                    assert value["result"]["result_code"] == 0
                    assert value["result"]["commitment"] == commitment
                outcomes.append(
                    {
                        "commitment": commitment,
                        "corrupt_signature": corrupt,
                        "output": value,
                    }
                )
        assert len(proxy.mutations) == 2, proxy.mutations
        assert proxy.forwarded_paths and all(
            incoming == forwarded == "/" for incoming, forwarded in proxy.forwarded_paths
        ), proxy.forwarded_paths
    args.output.write_text(json.dumps(outcomes, indent=2) + "\n")
    print(
        "Actual wallet verifier accepted executed/batched receipts and refused both corrupted signatures"
    )


if __name__ == "__main__":
    main()
