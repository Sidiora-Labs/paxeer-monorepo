import json
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "services/agent/sdk/python"))

from layerx_sdk.agent_http import (  # noqa: E402
    AgentEnvelopeTransport,
    AgentSessionCredential,
    LayerXKeyCredential,
)
from layerx_sdk.production import (  # noqa: E402
    IdempotencyKey,
    PlatformSdkError,
    SdkErrorCode,
    SecretBytes,
)


class AgentEnvelopeBootstrapCredentialTest(unittest.TestCase):
    def test_bootstrap_operations_carry_the_owner_session_credential(self) -> None:
        observed: list[tuple[str, dict[str, object]]] = []

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self) -> None:
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                observed.append((self.path, body))
                encoded = json.dumps({
                    "request_id": body["request_id"],
                    "value": {},
                    "verification_status": {"state": "achieved", "level": "SequencerSigned"},
                }).encode("utf-8")
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)

            def log_message(self, _format: str, *args: object) -> None:
                del args

        session = AgentSessionCredential("owner", "11" * 32, "22" * 32, 7)
        key = IdempotencyKey("33" * 32)
        with ThreadingHTTPServer(("127.0.0.1", 0), Handler) as server:
            thread = threading.Thread(target=server.serve_forever)
            thread.start()
            try:
                transport = AgentEnvelopeTransport(
                    f"http://127.0.0.1:{server.server_port}",
                    gateway_key=LayerXKeyCredential("owner", SecretBytes(b"44" * 32)),
                    session=session,
                )
                for index, operation in enumerate(("agent.register", "session.open"), start=1):
                    with self.subTest(operation=operation):
                        response = transport.call("agent", operation, {}, key)
                        self.assertEqual(len(observed), index)
                        path, body = observed[-1]
                        self.assertEqual(path, "/v1/agent/rpc")
                        self.assertEqual(body["operation"], operation)
                        self.assertEqual(body["credential"], session.coordinates())
                        self.assertEqual(body["idempotency_key"], str(key))
                        self.assertEqual(response.request_id, body["request_id"])
                        self.assertEqual(response.value, {})
                        self.assertEqual(response.verification_status, {
                            "state": "achieved", "level": "SequencerSigned",
                        })
            finally:
                server.shutdown()
                thread.join()

    def test_bootstrap_operations_refuse_a_missing_session_before_io(self) -> None:
        transport = AgentEnvelopeTransport(
            "http://127.0.0.1:1",
            gateway_key=LayerXKeyCredential("owner", SecretBytes(b"44" * 32)),
            session=None,
        )
        for operation in ("agent.register", "session.open"):
            with self.subTest(operation=operation):
                with self.assertRaises(PlatformSdkError) as raised:
                    transport.call("agent", operation, {}, IdempotencyKey("33" * 32))
                self.assertEqual(raised.exception.code, SdkErrorCode.INVALID_ARGUMENT)
