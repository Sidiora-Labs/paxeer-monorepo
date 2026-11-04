"""Replay the Agentd dispatch producer's committed submission transcript over HTTP.

This peer owns only recorded bytes and a private send ledger. It does not replace
Agentd's authorization, native outbox, finality verifier, or MCP implementation.
"""

from contextlib import contextmanager
from copy import deepcopy
import hashlib
import importlib.util
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import re
import socket
import sys
import tempfile
import threading


ROOT = Path(__file__).resolve().parents[4]
PRODUCER = ROOT / "services/agent/crates/layerx-agentd/src/agent_rpc_dispatch.rs"
RECORDING = Path(__file__).with_name("mcp_recovery_transcript.json")
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt

TRANSCRIPT_SHA256 = "5b366b9ce6190ee46c6b8ab5808312a41f794397185c531b88b7dd4669c9546a"
# Public recording coordinates, never operational credentials.
CREDENTIAL = {
    "tenant": "recorded-mcp-tenant",
    "session_id": "11" * 32,
    "token_id": "22" * 32,
    "generation": "1",
}
INVOCATION_KEY = "33" * 32


def produce_recording():
    source = PRODUCER.read_text()
    match = re.search(
        r'const MIDDLEWARE_SUBMISSION_TRANSCRIPT: &str = r#"(.*?)"#;',
        source,
        re.S,
    )
    if match is None or hashlib.sha256(match[1].encode()).hexdigest() != TRANSCRIPT_SHA256:
        raise RuntimeError("committed Agentd producer transcript changed; re-record explicitly")
    recording = json.loads(match[1])
    key = recording["receipt_key"]
    verified = verify_sequencer_receipt(
        bytes.fromhex(recording["committed"]["receipt"]["canonical_bytes"]),
        bytes.fromhex(recording["raw_transaction"]),
        (ReceiptKey(
            bytes.fromhex(key["keyId"][2:]), bytes.fromhex(key["publicKey"][2:]),
            int(key["firstHeight"], 16), int(key["lastHeight"], 16),
        ),),
    )
    # The dispatch vector supplies shapes; the real verifier supplies the reference.
    recording["committed"]["submission"]["receipt_ref"] = verified.receipt_digest.hex()
    return recording


def producer_recording():
    transcript = json.loads(RECORDING.read_text())
    if (transcript["source_transcript_sha256"] != TRANSCRIPT_SHA256
            or transcript["recording"] != produce_recording()):
        raise RuntimeError("recovery recording differs from its pinned producers; regenerate explicitly")
    return transcript["recording"]


class RecordedAgentd:
    def __init__(self, directory, recording=None):
        self.directory = Path(directory)
        self.directory.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.state = self.directory / "recorded-send.json"
        self.recording = producer_recording() if recording is None else recording
        self.fixture = self
        self.server = self.thread = None
        self.starts = self.stops = 0
        self.requests = []
        self.errors = []
        self.recovered = False
        self.recovery_stage = "committed"
        self.transform = None
        self.lookup_credential = deepcopy(CREDENTIAL)

    def envelope(self, name):
        reference = self.recording["pending"]["submission"]["submission_ref"]
        requests = {
            "submit": ("submit", {"raw_transaction": self.recording["raw_transaction"]}),
            "unknown_track": ("track", {"submission_ref": reference}),
            "unknown_receipt": (
                "program.receipt",
                {
                    "idempotency_key": INVOCATION_KEY,
                    "expected_activity_id": reference,
                    "requested_rung": "committed",
                },
            ),
        }
        operation, request = requests[name]
        result = {
            "version": 1,
            "request_id": str(len(self.requests) + 1),
            "operation": operation,
            "request": deepcopy(request),
            "credential": deepcopy(
                self.lookup_credential if name == "unknown_receipt" else CREDENTIAL
            ),
        }
        if name == "submit":
            result["idempotency_key"] = INVOCATION_KEY
        return result

    def retained_send(self):
        return {
            "raw_transaction": self.recording["raw_transaction"],
            "tx_hash": self.recording["pending"]["activity_id"],
            "transmissions": 1,
        }

    def dispatch(self, envelope):
        self.requests.append(deepcopy(envelope))
        identifier = envelope["request_id"]
        if envelope.get("credential") != CREDENTIAL:
            return 403, {
                "request_id": identifier,
                "class": "PolicyRefusal",
                "reason": "session.invalid",
                "retriability": "terminal",
            }
        operation = envelope["operation"]
        case = {"submit": "submit", "track": "unknown_track", "program.receipt": "unknown_receipt"}[operation]
        expected = self.envelope(case)
        expected["request_id"] = identifier
        assert envelope == expected, "request differs from the actual Agentd input schema"
        if operation == "submit":
            assert not self.state.exists(), "economic transmission repeated"
            fd = os.open(self.state, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            with os.fdopen(fd, "w") as stream:
                json.dump(self.retained_send(), stream)
                stream.flush()
                os.fsync(stream.fileno())
            stage = "pending"
        else:
            assert json.loads(self.state.read_text()) == self.retained_send()
            stage = self.recovery_stage if self.recovered else "pending"
        if operation == "program.receipt":
            assert stage == "committed"
            value = {"found": True, "receipt": deepcopy(self.recording[stage]["receipt"])}
        else:
            value = deepcopy(self.recording[stage]["submission"])
        # agent_rpc::success_body wraps decode_tracked/decode_receipt_lookup output.
        response = {
            "request_id": identifier,
            "value": value,
            "verification_status": {"state": "achieved", "rung": self.recording[stage]["submission"]["rung"]},
        }
        if self.transform is not None:
            self.transform(operation, stage, response)
        return 200, response

    def start(self):
        assert self.server is None
        self.starts += 1
        self.recovered = self.state.exists()
        peer = self

        class Handler(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *args):
                pass

            def do_POST(self):
                try:
                    assert self.path == "/rpc"
                    assert self.headers["Host"] == "localhost"
                    assert "Authorization" not in self.headers
                    assert "LayerX-Tenant" not in self.headers
                    length = int(self.headers["Content-Length"])
                    assert 0 < length <= 1_048_576
                    status, response = peer.dispatch(json.loads(self.rfile.read(length)))
                except Exception as error:
                    peer.errors.append(error)
                    status, response = 500, {"error": "recording_mismatch"}
                raw = json.dumps(response, separators=(",", ":")).encode()
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(raw)))
                self.send_header("Connection", "close")
                self.end_headers()
                self.wfile.write(raw)
                self.close_connection = True

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(
            target=self.server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True
        )
        self.thread.start()

    def stop(self):
        if self.server is None:
            return
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)
        assert not self.thread.is_alive()
        self.server = self.thread = None
        self.stops += 1
        assert not self.errors, self.errors

    @contextmanager
    def client(self, identity):
        assert identity == "client" and self.server is not None
        with socket.create_connection(self.server.server_address, timeout=5) as connection:
            yield connection


def recorded_phase(module, peer, directory):
    """Attach the real qualifier to a recorded peer instead of starting native binaries."""
    phase = module.Phase.__new__(module.Phase)
    phase.daemon = phase.fixture = peer
    phase.directory = Path(directory)
    phase.directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    phase.serial = 800_000
    phase.profile = {
        "unknown_track_request": "unknown_track",
        "unknown_receipt_request": "unknown_receipt",
    }
    return phase


def regenerate_recording():
    """Capture the real qualifier's HTTP recovery path using the SDK-produced reference."""
    spec = importlib.util.spec_from_file_location(
        "mcp_recovery_qualifier", Path(__file__).parents[1] / "mcp_daemon_revocation.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    with tempfile.TemporaryDirectory(prefix="mcp-recovery-recording-") as temporary:
        directory = Path(temporary)
        recording = produce_recording()
        peer = RecordedAgentd(directory / "peer", recording)
        phase = recorded_phase(module, peer, directory / "evidence")
        peer.start()
        try:
            phase.rpc(peer.envelope("submit"))
            result = phase.unknown()
            exchanges = [json.loads(path.read_text()) for path in sorted(phase.directory.glob("rpc-*.json"))]
            transcript = {
                "source_transcript_sha256": TRANSCRIPT_SHA256,
                "reference_producer": "layerx_sdk.verifier.verify_sequencer_receipt",
                "recording": recording,
                "requests": peer.requests,
                "exchanges": exchanges,
                "recovered": result,
            }
        finally:
            peer.stop()
    RECORDING.write_text(json.dumps(transcript, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    regenerate_recording()
