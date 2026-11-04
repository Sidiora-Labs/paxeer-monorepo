import json
import threading
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


@contextmanager
def recorded_rpc(answers, *, path="/", token=None):
    requests, failures = [], []
    remaining = list(answers)

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            try:
                raw = self.rfile.read(int(self.headers["Content-Length"]))
                body = json.loads(raw)
                requests.append({"path": self.path, "headers": dict(self.headers), "body": body})
                assert self.path == path
                assert self.headers.get_content_type() == "application/json"
                assert self.headers.get("Authorization") == token
                assert body["jsonrpc"] == "2.0"
                assert type(body["id"]) is int
                answer = remaining.pop(0)
                if answer is None:
                    self.close_connection = True
                    self.connection.close()
                    return
                status, headers, data = answer
                payload = data if isinstance(data, bytes) else json.dumps({"jsonrpc": "2.0", "id": body["id"], **data}).encode()
                self.send_response(status)
                for name, value in headers.items():
                    self.send_header(name, value)
                self.send_header("Content-Length", str(len(payload)))
                self.end_headers()
                self.wfile.write(payload)
            except (AssertionError, KeyError, IndexError) as error:
                failures.append(error)
                self.send_error(500)

        def log_message(self, format, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}{path}", requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()
        assert failures == []
        assert remaining == [], "all recorded responses must be consumed"


def result(value):
    return 200, {"Content-Type": "application/json"}, {"result": value}
