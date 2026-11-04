from __future__ import annotations

import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import cast

from layerx_sdk import AgentHttpTransport, ProductionClient
from layerx_sdk.agent_http import AgentEnvelopeTransport, AgentSessionCredential, LayerXKeyCredential
from layerx_sdk.production import SecretBytes
from layerx_sdk.programs import (
    ProgramDiscovery,
    ProgramInterface,
    ProgramOperations,
    ProgramSource,
    ProgramTrustContext,
)
from layerx_sdk.verifier import LocalSignatureVerifier


class TypedProgramResponseTests(unittest.TestCase):
    def test_current_envelope_preserves_achieved_and_unverified(self) -> None:
        from test_program_trust import discovery, envelope
        from layerx_sdk.programs import _discovery

        statuses = []
        current = discovery(9, 1000, 2000, "44" * 32)

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                statuses.append((self.path, self.headers["Content-Type"], request))
                body = json.loads(envelope(current, achieved=len(statuses) == 2))
                body["request_id"] = request["request_id"]
                encoded = json.dumps(body).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)

            def log_message(self, _format, *args):
                pass

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        secret = SecretBytes(b"44" * 32)
        try:
            session = AgentSessionCredential("owner", "11" * 32, "22" * 32, 7)
            client = ProductionClient(AgentEnvelopeTransport(
                f"http://127.0.0.1:{server.server_port}",
                gateway_key=LayerXKeyCredential("owner", secret), session=session))
            expected_statuses = (
                {"state": "unverified", "requested": "receipted", "achieved": "submitted",
                 "reason": "server_side_receipt_verification_only"},
                {"state": "achieved", "rung": "receipted"},
            )
            for expected in expected_statuses:
                response = client.agent("program.discover", {"program_id": current["program_id"]})
                self.assertEqual(dict(response.verification_status), expected)
                typed = _discovery(response.value, current["program_id"], 1500)
                self.assertIsInstance(typed, ProgramDiscovery)
                self.assertEqual(typed.verification, "server-side-receipt-verification-only")
                self.assertFalse(typed.sequencer_signature_verified)
            self.assertEqual(len(statuses), 2)
            for path, content_type, request in statuses:
                self.assertEqual((path, content_type), ("/v1/agent/rpc", "application/json"))
                self.assertEqual(request["credential"], session.coordinates())
                self.assertIsNone(request["idempotency_key"])
        finally:
            secret.destroy()
            server.shutdown()
            server.server_close()
            thread.join()

    def test_discovery_and_interface_are_typed_and_honest_about_server_verification(self) -> None:
        program_id = "11" * 32
        common = {
            "program_id": program_id,
            "version": 7,
            "code_hash": "22" * 32,
            "abi_version": 2,
            "receipt_digest": "33" * 32,
            "state_root": "44" * 32,
            "observed_sequence": "9",
            "observed_at": "1000",
            "valid_through": "2000",
        }
        discovery = dict(common, lifecycle="active", verification="registry-receipt-and-current-head-verified")
        interface = dict(common, interface="00",
            interface_digest="6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d",
            source={"status": "verified", "source_digest": "55" * 32,
                "environment_digest": "66" * 32,
                "pipeline": "sha256-source-artifact-reproducible-build-v1"},
            verification="deployment-interface-and-current-head-verified")
        class Handler(BaseHTTPRequestHandler):
            def do_GET(self) -> None:
                value = interface if self.path.endswith("/interface") else discovery
                encoded = json.dumps({"request_id": "1", "value": value,
                    "verification_status": {"state": "unverified", "requested": "receipted",
                        "achieved": "submitted", "reason": "server_side_receipt_verification_only"}},
                    separators=(",", ":")).encode()
                self.send_response(200); self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(encoded))); self.end_headers(); self.wfile.write(encoded)

            def log_message(self, _format: str, *args: object) -> None:
                del args

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True); thread.start()
        try:
            operations = ProgramOperations(ProductionClient(AgentHttpTransport(
                f"http://127.0.0.1:{server.server_port}")), cast(LocalSignatureVerifier, None),
                ProgramTrustContext(bytes([1]) * 32, lambda: 1500))
            typed_discovery = operations.discover(program_id)
            self.assertIsInstance(typed_discovery, ProgramDiscovery)
            self.assertEqual(typed_discovery.observed_sequence, 9)
            self.assertEqual(typed_discovery.verification, "server-side-receipt-verification-only")

            typed_interface = operations.interface(program_id)
            self.assertIsInstance(typed_interface, ProgramInterface)
            self.assertEqual(typed_interface.interface, b"\x00")
            self.assertIsInstance(typed_interface.source, ProgramSource)
            self.assertEqual(typed_interface.source.status, "verified")
            self.assertEqual(typed_interface.verification, "server-side-receipt-verification-only")
        finally:
            server.shutdown(); server.server_close(); thread.join()


if __name__ == "__main__":
    unittest.main()
