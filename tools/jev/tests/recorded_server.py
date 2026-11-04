"""Loopback HTTP peer for Jev's committed decision-response transcripts."""

from __future__ import annotations

import hashlib
import json
import threading
from contextlib import contextmanager
from dataclasses import dataclass, field
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Iterator

from tools.jev.client import DEFAULT_MODEL, JevClient, canonical_request


FIXTURES = Path(__file__).parent / "fixtures" / "repair_22_33_705"
LOOPBACK_TOKEN = "jev-loopback-transcript"


@dataclass
class Exchange:
    client: JevClient
    response: bytes
    requests: list[tuple[str, dict[str, str], bytes]] = field(default_factory=list)

    def assert_request(self, state: object, questions: dict) -> None:
        assert len(self.requests) == 1
        path, headers, raw = self.requests[0]
        assert path == "/api/alpha/decisions"
        assert headers["authorization"] == f"Bearer {LOOPBACK_TOKEN}"
        assert headers["content-type"] == "application/json"
        payload, digest = canonical_request(DEFAULT_MODEL, state, questions)
        assert raw == payload
        assert hashlib.sha256(raw).hexdigest() == digest
        assert json.loads(raw) == {
            "model": DEFAULT_MODEL, "state": state, "questions": questions
        }

    def assert_evaluation(self, evaluation, state: object, questions: dict) -> None:
        self.assert_request(state, questions)
        assert evaluation.request_sha256 == hashlib.sha256(self.requests[0][2]).hexdigest()
        assert evaluation.response_sha256 == hashlib.sha256(self.response).hexdigest()
        assert set(evaluation.answers) == set(questions)
        assert evaluation.requested_model == DEFAULT_MODEL
        assert evaluation.model == "typesafe/jev-1.13-20260917"
        assert evaluation.request_id == json.loads(self.response)["id"]


@contextmanager
def recorded_client(fixture: str) -> Iterator[Exchange]:
    response = (FIXTURES / fixture).read_bytes()
    # An explicit model, key and URL keep the test independent of the environment.
    exchange: Exchange

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:
            raw = self.rfile.read(int(self.headers["Content-Length"]))
            exchange.requests.append(
                (self.path, {key.lower(): value for key, value in self.headers.items()}, raw)
            )
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(response)))
            self.end_headers()
            self.wfile.write(response)

        def log_message(self, format: str, *args: object) -> None:
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    client = JevClient(
        api_key=LOOPBACK_TOKEN,
        endpoint=f"http://127.0.0.1:{server.server_port}/api/alpha/decisions",
        model=DEFAULT_MODEL,
        timeout_seconds=2.0,
        max_attempts=1,
    )
    exchange = Exchange(client=client, response=response)
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01})
    thread.start()
    try:
        yield exchange
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2.0)
        assert not thread.is_alive()
