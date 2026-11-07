"""TLS framing for the real Rust daemon handlers used by the CLI integration test."""
import http.server
import ssl
import sys
from pathlib import Path

root = Path(sys.argv[1])
expected = "LayerX-Key " + (root / "gateway-key").read_text().strip()
requests = sys.stdin.buffer
responses = sys.stdout.buffer


class DaemonHTTP(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *_):
        pass

    def do_POST(self):
        if self.path != "/v1/agent/rpc":
            self.send_error(404)
            return
        import hmac
        if not hmac.compare_digest(self.headers.get("Authorization", ""), expected):
            self.send_error(401)
            return
        length = int(self.headers["Content-Length"])
        if not 0 < length <= 1_048_576:
            self.send_error(413)
            return
        body = self.rfile.read(length)
        responses.write(str(len(body)).encode() + b"\n" + body)
        responses.flush()
        length_line = requests.readline()
        if not length_line:
            raise RuntimeError("daemon handler exited before responding")
        body = requests.read(int(length_line))
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(body)
        self.close_connection = True


server = http.server.HTTPServer(("127.0.0.1", 0), DaemonHTTP)
context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
context.load_cert_chain(root / "server.pem", root / "server.key")
server.socket = context.wrap_socket(server.socket, server_side=True)
responses.write(str(server.server_port).encode() + b"\n")
responses.flush()
server.serve_forever()
