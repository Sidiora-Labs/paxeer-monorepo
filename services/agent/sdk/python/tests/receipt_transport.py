from __future__ import annotations

import json
import threading
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from layerx_sdk.x402_rpc import PaymentRpc


@contextmanager
def recorded_payment_rpc(vector, *, raw_transaction):
    exchanges = []
    errors = []

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            try:
                assert self.path == "/"
                request = json.loads(
                    self.rfile.read(int(self.headers["Content-Length"]))
                )
                assert request["jsonrpc"] == "2.0"
                method = request["method"]
                if method == "eth_sendRawTransaction":
                    assert request["params"] == [vector["rawTransaction"]]
                    result = vector["receiptJson"]["txHash"]
                elif method == "pax_sequencerReceipt":
                    assert request["params"] == [vector["receiptJson"]["txHash"]]
                    result = vector["receiptJson"]
                elif method == "eth_getRawTransactionByHash":
                    assert request["params"] == [vector["receiptJson"]["txHash"]]
                    result = raw_transaction
                else:
                    raise AssertionError(method)
                exchanges.append((method, request["params"]))
                body = json.dumps(
                    {"jsonrpc": "2.0", "id": request["id"], "result": result}
                ).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
            except Exception as error:
                errors.append(error)
                self.send_error(500)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    try:
        yield PaymentRpc(f"http://127.0.0.1:{server.server_port}"), exchanges
    finally:
        server.shutdown()
        server.server_close()
        worker.join()
        assert errors == []
