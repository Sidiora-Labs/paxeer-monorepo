"""Run the private TLS control service with an HTTP liveness probe."""

import http.client
import os
import signal
import socket
import ssl
import subprocess
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path


HEALTH_PATH = "/livez"
MAX_RESPONSE = 65536


def probe(context, server_name, port):
    with socket.create_connection(("::1", port), timeout=3) as connection:
        with context.wrap_socket(connection, server_hostname=server_name) as secured:
            secured.sendall(
                (
                    f"GET {HEALTH_PATH} HTTP/1.1\r\nHost: {server_name}\r\n"
                    "Connection: close\r\n\r\n"
                ).encode("ascii")
            )
            response = http.client.HTTPResponse(secured)
            response.begin()
            body = response.read(MAX_RESPONSE + 1)
            if response.status != 200 or len(body) > MAX_RESPONSE:
                raise ValueError("control liveness probe refused")
            return body


class HealthServer(HTTPServer):
    address_family = socket.AF_INET6


def run():
    port = int(os.environ["PORT"])
    if not 1024 <= port <= 65535 or port in (9443, 9444):
        raise ValueError("HTTP probe port must be unprivileged and distinct from TLS")
    server_name = os.environ["LAYERX_TESTNET_HEALTH_SERVER_NAME"]
    if not server_name or any(
        character not in "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-"
        for character in server_name
    ):
        raise ValueError("TLS probe server name must be a DNS name")
    ca = Path(os.environ["LAYERX_OUTBOUND_CA_DER"]).read_bytes()
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
    context.load_verify_locations(cadata=ssl.DER_cert_to_PEM_cert(ca))
    listen = os.environ["LAYERX_TESTNET_PUBLIC_LISTEN"]
    if listen != "[::]:9443":
        raise ValueError("public TLS listener must use IPv6 port 9443")
    if os.environ["LAYERX_TESTNET_ADMIN_LISTEN"] != "[::]:9444":
        raise ValueError("admin TLS listener must use IPv6 port 9444")

    class Handler(BaseHTTPRequestHandler):
        def setup(self):
            super().setup()
            self.connection.settimeout(3)

        def do_GET(self):
            if self.path != HEALTH_PATH:
                self.send_error(404)
                return
            try:
                body = probe(context, server_name, 9443)
                status = 200
            except (OSError, ValueError, http.client.HTTPException):
                body = b'{"status":"unavailable"}'
                status = 503
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Cache-Control", "no-store")
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format, *args):
            return

    stopping = False

    def stop(signum, frame):
        nonlocal stopping
        stopping = True

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    with HealthServer(("::", port), Handler) as server:
        server.timeout = 0.5
        child = subprocess.Popen(["/bin/sh", "/opt/layerx/run-testnet.sh"])
        try:
            while not stopping and child.poll() is None:
                server.handle_request()
        finally:
            if child.poll() is None:
                child.terminate()
                try:
                    child.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    child.kill()
                    child.wait()
    return 0 if stopping else child.returncode or 1


if __name__ == "__main__":
    try:
        sys.exit(run())
    except (KeyError, OSError, ValueError):
        print("testnet control startup refused: invalid runtime inputs", file=sys.stderr)
        sys.exit(2)
