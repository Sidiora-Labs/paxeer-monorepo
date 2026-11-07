"""Committed SQLite/HTTP protocol peer for offline restart qualification.

This fixture owns durable blocks and application state. It does not implement
paxd, governance, upgrade handlers or native state migration.
"""

import argparse
from contextlib import closing
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import signal
import sqlite3
import threading
import time
from urllib.parse import parse_qs, urlparse


def connect(store):
    return sqlite3.connect(store, timeout=1)


def initialize(store):
    with closing(connect(store)) as db, db:
        db.execute("PRAGMA journal_mode=WAL")
        db.execute("CREATE TABLE IF NOT EXISTS blocks (height INTEGER PRIMARY KEY, hash TEXT NOT NULL)")
        db.execute("CREATE TABLE IF NOT EXISTS application (key TEXT PRIMARY KEY, value TEXT NOT NULL)")
        db.execute("INSERT OR IGNORE INTO application VALUES ('balance', '100')")
        if not db.execute("SELECT 1 FROM blocks LIMIT 1").fetchone():
            db.execute("INSERT INTO blocks VALUES (1, ?)", (hashlib.sha256(b"pax-genesis").hexdigest().upper(),))


def commit_block(store):
    with closing(connect(store)) as db, db:
        height, previous = db.execute("SELECT height, hash FROM blocks ORDER BY height DESC LIMIT 1").fetchone()
        block_hash = hashlib.sha256(f"{height + 1}:{previous}".encode()).hexdigest().upper()
        db.execute("INSERT INTO blocks VALUES (?, ?)", (height + 1, block_hash))


def serve(store, address, identity, stalled):
    initialize(store)
    stopped = threading.Event()

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            parsed = urlparse(self.path)
            with closing(connect(store)) as db:
                if parsed.path == "/status":
                    height, block_hash = db.execute("SELECT height, hash FROM blocks ORDER BY height DESC LIMIT 1").fetchone()
                    value = {"result": {"node_info": {"id": identity, "network": "pax"},
                                        "sync_info": {"latest_block_height": str(height),
                                                      "latest_block_hash": block_hash,
                                                      "catching_up": False}}}
                elif parsed.path == "/block":
                    height = int(parse_qs(parsed.query)["height"][0])
                    row = db.execute("SELECT hash FROM blocks WHERE height=?", (height,)).fetchone()
                    value = ({"result": {"block_id": {"hash": row[0]}, "block": {
                        "header": {"height": str(height), "chain_id": "pax"}}}}
                             if row else {"error": "committed block missing"})
                elif parsed.path == "/application":
                    value = dict(db.execute("SELECT key, value FROM application"))
                else:
                    self.send_error(404)
                    return
            data = json.dumps(value).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def log_message(self, *args):
            pass

    # The port is retained in the address file so RPC consumers reconnect after restart.
    port = int(address.read_text().rsplit(":", 1)[1]) if address.exists() else 0
    server = ThreadingHTTPServer(("127.0.0.1", port), Handler)
    server.daemon_threads = True
    server.timeout = 0.02
    address.write_text(f"http://127.0.0.1:{server.server_port}")
    print(f"opened durable peer {identity}", flush=True)

    def stop(signum, frame):
        stopped.set()

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)

    def produce():
        while not stopped.wait(0.01):
            if not stalled:
                commit_block(store)

    producer = threading.Thread(target=produce)
    producer.start()
    try:
        while not stopped.is_set():
            server.handle_request()
    finally:
        stopped.set()
        producer.join(timeout=1)
        server.server_close()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--store", type=Path, required=True)
    parser.add_argument("--address", type=Path, required=True)
    parser.add_argument("--identity", required=True)
    parser.add_argument("--stalled", action="store_true")
    parser.add_argument("action", choices=("start", "version"))
    parser.add_argument("--chain-id", default="pax", choices=("pax",))
    parser.add_argument("--inv-check-period", type=int, default=0)
    args = parser.parse_args()
    if args.action == "version":
        print("committed-peer-1")
    else:
        serve(args.store, args.address, args.identity, args.stalled)


if __name__ == "__main__":
    main()
