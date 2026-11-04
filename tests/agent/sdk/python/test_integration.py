from __future__ import annotations

import ast
import importlib.util
import json
import ipaddress
import socket
import ssl
import tempfile
import threading
import unittest
import zipfile
from contextlib import contextmanager
from datetime import datetime, timedelta, timezone
from hashlib import sha256
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from types import MappingProxyType
from typing import get_args

from layerx_sdk import (
    APPROVAL_CONTRACT_INTRODUCED,
    APPROVAL_DECISION_OUTCOMES,
    APPROVAL_ENFORCEMENT_NOTICE,
    APPROVAL_EVENT_KINDS,
    APPROVAL_STATES,
    AgentHttpTransport,
    ApiError,
    ApprovalApproveRequest,
    ApprovalGetRequest,
    ApprovalListRequest,
    ApprovalRejectRequest,
    Client,
    ErrorClass,
    IdempotencyKey,
    IdempotentMutation,
    LayerXKeyCredential,
    PlatformSdkError,
    ProductionClient,
    ProgramCall,
    ProgramOperations,
    ProgramTrustContext,
    Rung,
    SdkErrorCode,
    SecretBytes,
    VerifiedRead,
    layerx_sdk_py_package,
    parse_amount,
    parse_sequence,
    require_verified,
)
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec, utils
from cryptography.x509.oid import NameOID

from layerx_sdk.account_derivation import _N, _recover, keccak256
from layerx_sdk.agent_http import AgentDaemonEnvelopeTransport, AgentSessionCredential
from layerx_sdk.ai_market import _decode_programs_call, _encode_programs_call
from layerx_sdk.evm_transaction import Type2Transaction, decode_signed
from layerx_sdk.native_program_call import NativeProgramCall
from layerx_sdk.programs import NativeProgramRequest

ROOT = Path(__file__).resolve().parents[4] / "services/agent/sdk/python"


def retired_kernel_program_call(program_id: str, idempotency: str) -> bytes:
    payload = b"".join((
        b"LayerX/programs/call/v1\0",
        bytes.fromhex(program_id),
        (1).to_bytes(8, "big"),
        (0).to_bytes(16, "big"),
        (0).to_bytes(2, "big"),
        sized(b"\xaa"),
    ))
    payload_hash = sha256(b"LXP/v1/payload-hash\0" + payload).digest()
    return b"".join((
        (1).to_bytes(2, "big"), (0x1001).to_bytes(2, "big"), b"\x0c",
        b"\x01", (1).to_bytes(2, "big"),
        b"\x02", (1).to_bytes(4, "big"),
        b"\x03", (0x0009_0003).to_bytes(4, "big"),
        b"\x04", sized(b"did:lxp:test"),
        b"\x05", sized(b"\x01"),
        b"\x06", (0).to_bytes(8, "big"),
        b"\x07", (0).to_bytes(8, "big"), (1).to_bytes(8, "big"),
        b"\x08", sized(bytes.fromhex(idempotency)),
        b"\x09", (0).to_bytes(16, "big"),
        b"\x0a", sized(payload_hash),
        b"\x0b", sized(payload),
        b"\x0c", sized(b"\x02"),
    ))


def sized(value: bytes) -> bytes:
    return len(value).to_bytes(4, "big") + value


def prepared_program_call() -> NativeProgramRequest:
    native = NativeProgramCall(
        bytes.fromhex("11" * 32), 1, "main", b"\xaa", b"", b"", 16,
        (1000, 1024, 0, 0, 0, 16, 1000),
    )
    transaction = Type2Transaction(
        125, 7, 1, 2, 200_000, bytes.fromhex("101d".rjust(40, "0")),
        0, _encode_programs_call(native, 1000),
    )
    transaction = Type2Transaction.decode(transaction.encode())
    key = ec.derive_private_key(42, ec.SECP256K1())
    public = key.public_key().public_bytes(
        serialization.Encoding.X962, serialization.PublicFormat.UncompressedPoint,
    )[1:]
    digest = transaction.signing_digest()
    r, s = utils.decode_dss_signature(
        key.sign(digest, ec.ECDSA(utils.Prehashed(hashes.SHA256())))
    )
    s = min(s, _N - s)
    parity = next(p for p in (0, 1) if _recover(digest, r, s, p) == public)
    signature = r.to_bytes(32, "big") + s.to_bytes(32, "big") + bytes([27 + parity])
    raw = transaction.attach_signature(signature, "0x" + keccak256(public)[-20:].hex())
    return NativeProgramRequest(native, 1000, raw)


def program_operations(endpoint: str) -> ProgramOperations:
    path = ROOT.parents[3] / "sdk/integrations/fastapi/layerx_fastapi/signatures.py"
    spec = importlib.util.spec_from_file_location("integration_signatures", path)
    if spec is None or spec.loader is None:
        raise RuntimeError("production signature verifier unavailable")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    credential = LayerXKeyCredential("key_1", SecretBytes(("lxp_live_" + "22" * 32).encode()))
    return ProgramOperations(
        ProductionClient(AgentHttpTransport(endpoint, credential=credential, timeout=2)),
        module.LayerXSignatureVerifier(),
        ProgramTrustContext(bytes([0x44]) * 32, protocol_version=3, network_id=125),
    )


def daemon_tls(directory: Path) -> tuple[ssl.SSLContext, ssl.SSLContext]:
    key = ec.generate_private_key(ec.SECP256R1())
    name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "sdk-loopback")])
    now = datetime.now(timezone.utc)
    certificate = (
        x509.CertificateBuilder().subject_name(name).issuer_name(name)
        .public_key(key.public_key()).serial_number(x509.random_serial_number())
        .not_valid_before(now - timedelta(minutes=1))
        .not_valid_after(now + timedelta(hours=1))
        .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
        .add_extension(x509.SubjectAlternativeName([
            x509.IPAddress(ipaddress.ip_address("127.0.0.1")),
        ]), critical=False)
        .sign(key, hashes.SHA256())
    )
    cert_path, key_path = directory / "cert.pem", directory / "key.pem"
    cert_path.write_bytes(certificate.public_bytes(serialization.Encoding.PEM))
    key_path.write_bytes(key.private_bytes(
        serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption(),
    ))
    server = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    server.load_cert_chain(cert_path, key_path)
    server.load_verify_locations(cafile=str(cert_path))
    server.verify_mode = ssl.CERT_REQUIRED
    client = ssl.create_default_context(cafile=str(cert_path))
    client.load_cert_chain(cert_path, key_path)
    return server, client


@contextmanager
def integration_server(dispatch, *, tls: ssl.SSLContext | None = None):
    observed, failures = [], []

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:
            try:
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                observed.append((self.path, dict(self.headers), body))
                assert self.headers["Content-Type"] == "application/json"
                assert self.headers["Accept"] == "application/json"
                assert self.headers["User-Agent"] == "layerx-python/0.1.0"
                status, response = dispatch(self.path, self.headers, body)
                if status == 0:
                    self.connection.shutdown(socket.SHUT_RDWR)
                    self.close_connection = True
                    return
                encoded = json.dumps(response, separators=(",", ":")).encode()
                self.send_response(status)
                if status == 307:
                    self.send_header("Location", "/target")
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)
            except Exception as error:
                failures.append(error)
                self.close_connection = True

        def log_message(self, _format: str, *args: object) -> None:
            del args

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    if tls is not None:
        server.socket = tls.wrap_socket(server.socket, server_side=True)
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True)
    thread.start()
    try:
        scheme = "https" if tls is not None else "http"
        yield f"{scheme}://127.0.0.1:{server.server_port}", observed
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)
        assert not thread.is_alive()
        assert not failures, failures


def daemon_client(endpoint: str, context: ssl.SSLContext) -> ProductionClient:
    return ProductionClient(AgentDaemonEnvelopeTransport(
        endpoint, ssl_context=context, timeout=2,
        session=AgentSessionCredential("tenant-a", "11" * 32, "22" * 32, 1),
    ))


def check_daemon_request(path, headers, body) -> None:
    assert path == "/rpc"
    assert headers["Authorization"] is None
    assert headers["Idempotency-Key"] is None
    assert set(body) == {"version", "request_id", "operation", "request", "credential", "idempotency_key"}
    assert body["version"] == 1
    assert str(int(body["request_id"])) == body["request_id"]
    assert body["credential"] == {
        "tenant": "tenant-a", "session_id": "11" * 32,
        "token_id": "22" * 32, "generation": "1",
    }


def daemon_balance(request) -> dict[str, object]:
    return {
        "value": {"account": request["account"], "asset": request["asset"], "amount": "9007199254740993"},
        "achieved_verification_level": None,
        "freshness": {"chain_head": "20", "value_height": "20", "relative_to": "20"},
    }


def submission_status(identity: str) -> dict[str, object]:
    return {"tx_hash": identity, "submission_ref": identity, "state": "Submitted",
            "rung": "submitted", "evidence": [], "transitions": []}


def load_build_backend():
    spec = importlib.util.spec_from_file_location("layerx_sdk_build_backend", ROOT / "build_backend.py")
    if spec is None or spec.loader is None:
        raise RuntimeError("build backend unavailable")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class PythonSdkIntegration(unittest.TestCase):
    def test_agent_http_transport_preserves_program_contract_and_authentication(self) -> None:
        request = {"account": "11" * 32, "asset": "33" * 32,
                   "requested_verification_level": "submitted"}

        def dispatch(path, headers, body):
            check_daemon_request(path, headers, body)
            self.assertEqual(body["operation"], "read.balance")
            self.assertEqual(body["request"], request)
            self.assertIsNone(body["idempotency_key"])
            return 200, {"request_id": body["request_id"], "value": daemon_balance(request),
                         "verification_status": {"state": "achieved", "rung": "submitted"}}

        with tempfile.TemporaryDirectory() as directory:
            server_tls, client_tls = daemon_tls(Path(directory))
            with integration_server(dispatch, tls=server_tls) as (endpoint, observed):
                result = daemon_client(endpoint, client_tls).agent("read.balance", request)
                self.assertEqual(result.value, daemon_balance(request))
                self.assertEqual(result.verification_status, {"state": "achieved", "rung": "submitted"})
                self.assertEqual(parse_amount(result.value["value"]["amount"]), 9_007_199_254_740_993)
                self.assertEqual(len(observed), 1)
                self.assertEqual(observed[0][2]["request_id"], result.request_id)

    def test_program_call_ambiguity_retains_canonical_signed_evidence(self) -> None:
        request = prepared_program_call()
        signed = request.signed_activity
        transaction, signature = decode_signed(signed)
        identity = keccak256(signed).hex()
        self.assertEqual(signed[0], 2)
        self.assertEqual(transaction.chain_id, 125)
        self.assertEqual(_decode_programs_call(transaction.data), (request.native_call, request.fee_limit))
        self.assertEqual(transaction.attach_signature(signature, transaction.recover_signer(signature)), signed)
        sent = []

        def dispatch(path, headers, body):
            self.assertEqual(path, "/")
            self.assertEqual(headers["Authorization"], "LayerX-Key key_1:lxp_live_" + "22" * 32)
            self.assertEqual(set(body), {"jsonrpc", "id", "method", "params"})
            self.assertEqual(body["jsonrpc"], "2.0")
            self.assertIs(type(body["id"]), int)
            if body["method"] == "eth_sendRawTransaction":
                self.assertEqual(headers["Idempotency-Key"], identity)
                self.assertEqual(body["params"], ["0x" + signed.hex()])
                sent.append(bytes.fromhex(body["params"][0][2:]))
                if len(sent) == 1:
                    return 0, None
                result = "0x" + identity
            else:
                self.assertEqual(body["method"], "pax_sequencerReceipt")
                self.assertIsNone(headers["Idempotency-Key"])
                self.assertEqual(body["params"], ["0x" + identity])
                result = {"txHash": "0x" + identity, "nonce": hex(transaction.nonce), "rung": "submitted"}
            return 200, {"jsonrpc": "2.0", "id": body["id"], "result": result}

        with tempfile.TemporaryDirectory() as directory:
            with integration_server(dispatch) as (endpoint, observed):
                programs = program_operations(endpoint)
                unknown = programs.submit(request, IdempotencyKey(identity))
                self.assertEqual(unknown, {
                    "state": "unknown", "activity_id": identity, "idempotency_key": identity,
                    "retained_signed_activity": signed.hex(),
                })
                retained = Path(directory) / "retained.json"
                retained.write_text(json.dumps(dict(unknown)))
                del programs, request
            recovered = json.loads(retained.read_text())
            reopened_raw = bytes.fromhex(recovered["retained_signed_activity"])
            restored_tx, _ = decode_signed(reopened_raw)
            native, fee = _decode_programs_call(restored_tx.data)
            restored = NativeProgramRequest(native, fee, reopened_raw)
            with integration_server(dispatch) as (endpoint, recovered_observed):
                programs = program_operations(endpoint)
                pending = programs.submit(restored, IdempotencyKey(recovered["idempotency_key"]))
                self.assertEqual(pending, {**recovered, "state": "pending"})
                self.assertEqual(programs.receipt(identity, identity), pending)
                self.assertEqual([row[2]["method"] for row in observed + recovered_observed], [
                    "eth_sendRawTransaction", "eth_sendRawTransaction", "pax_sequencerReceipt",
                ])
            self.assertEqual(sent, [signed, signed])
            self.assertEqual(recovered["activity_id"], keccak256(reopened_raw).hex())
            self.assertNotEqual(identity, sha256(b"LXP/v1/activity-id\0" + signed).hexdigest())

    def test_agent_http_transport_never_follows_redirects(self) -> None:
        def dispatch(path, headers, body):
            check_daemon_request(path, headers, body)
            return 307, {}

        with tempfile.TemporaryDirectory() as directory:
            server_tls, client_tls = daemon_tls(Path(directory))
            with integration_server(dispatch, tls=server_tls) as (endpoint, observed):
                with self.assertRaises(PlatformSdkError) as caught:
                    daemon_client(endpoint, client_tls).agent("read.balance", {
                        "account": "11" * 32, "asset": "33" * 32,
                        "requested_verification_level": "submitted",
                    })
                self.assertEqual(caught.exception.code, SdkErrorCode.DECODE_FAILURE)
                self.assertEqual([row[0] for row in observed], ["/rpc"])

    def test_approval_contract_operations_events_and_outcomes_are_exact(self) -> None:
        self.assertEqual(APPROVAL_CONTRACT_INTRODUCED, "1.1")
        self.assertIn("confers no protocol authority", APPROVAL_ENFORCEMENT_NOTICE)
        self.assertEqual(APPROVAL_STATES, ("Held", "Granted", "Rejected", "Expired", "Defective"))
        self.assertEqual(
            APPROVAL_DECISION_OUTCOMES,
            ("Granted", "Rejected", "Expired", "Defective", "AlreadyDecided", "Conflict"),
        )
        self.assertEqual(
            APPROVAL_EVENT_KINDS,
            ("Created", "Granted", "Rejected", "Expired", "Defective"),
        )

        class RecordingTransport:
            def __init__(self) -> None:
                self.operations: list[str] = []

            def call(self, operation: str, request: object) -> object:
                self.operations.append(operation)
                return request

        transport = RecordingTransport()
        client = Client(transport)  # type: ignore[arg-type]
        client.approval_list(ApprovalListRequest("tenant-a", None, 50))
        client.approval_get(ApprovalGetRequest("tenant-a", "approval-7"))
        client.approval_approve(ApprovalApproveRequest("tenant-a", "approval-7", "approve-7"))
        client.approval_reject(
            ApprovalRejectRequest("tenant-a", "approval-7", "reject-7", "not expected")
        )
        self.assertEqual(
            transport.operations,
            ["approval.list", "approval.get", "approval.approve", "approval.reject"],
        )

    def test_package_metadata_is_immutable_and_contract_bound(self) -> None:
        metadata = layerx_sdk_py_package()
        self.assertIsInstance(metadata, MappingProxyType)
        self.assertEqual(metadata["name"], "layerx-sdk")
        self.assertEqual(metadata["version"], "0.1.0")
        self.assertEqual(metadata["contract_major"], 2)
        with self.assertRaises(TypeError):
            metadata["contract_major"] = 2  # type: ignore[index]

    def test_consensus_integers_are_exact_and_bounded(self) -> None:
        self.assertEqual(parse_amount("9007199254740993"), 9_007_199_254_740_993)
        self.assertEqual(parse_sequence("18446744073709551615"), 18_446_744_073_709_551_615)
        for invalid in ("", "01", "-1", "1.5", "１"):
            with self.subTest(invalid=invalid), self.assertRaises(ValueError):
                parse_amount(invalid)
        with self.assertRaises(OverflowError):
            parse_sequence("18446744073709551616")

    def test_reads_require_the_declared_verification_level(self) -> None:
        read = VerifiedRead(
            value=10,
            achieved_verification_level=Rung.RECEIPTED,
            chain_head=20,
            latest_batch="batch-19",
            latest_checkpoint="checkpoint-18",
            value_sequence=17,
        )
        self.assertIs(require_verified(Rung.RECEIPTED, read), read)
        with self.assertRaisesRegex(ValueError, "verification_below_requested"):
            require_verified(Rung.COMMITTED, read)
        unverified = VerifiedRead(
            value=10,
            achieved_verification_level=None,
            chain_head=20,
            latest_batch="batch-19",
            latest_checkpoint="checkpoint-18",
            value_sequence=17,
        )
        with self.assertRaisesRegex(ValueError, "unverified_read"):
            require_verified(Rung.SUBMITTED, unverified)

    def test_idempotent_mutation_preserves_request_identity_and_large_amounts(self) -> None:
        mutation = IdempotentMutation(
            request_id=18_446_744_073_709_551_615,
            key=bytes(range(32)),
            body_digest=bytes(reversed(range(32))),
            operation={"amount": 9_007_199_254_740_993},
        )
        self.assertEqual(mutation.operation["amount"], 9_007_199_254_740_993)
        self.assertEqual(mutation.request_id, 18_446_744_073_709_551_615)
        self.assertEqual(mutation.key, bytes(range(32)))
        self.assertEqual(mutation.body_digest, bytes(reversed(range(32))))
        self.assertEqual(len(mutation.key), 32)

    def test_typed_error_taxonomy_is_complete_and_disjoint(self) -> None:
        classes = set(get_args(ErrorClass))
        self.assertEqual(len(classes), 12)
        self.assertIn("TransportFailure", classes)
        self.assertIn("VerificationFailure", classes)
        self.assertNotEqual("TransportFailure", "VerificationFailure")
        error = ApiError("CoreRejection", -77_777, False, 7, "future_code")
        self.assertEqual(error.protocol_result_code, -77_777)
        self.assertEqual(str(error), "CoreRejection:future_code")

    def test_type_stubs_and_examples_are_parseable(self) -> None:
        for path in sorted((ROOT / "layerx_sdk").rglob("*.pyi")):
            ast.parse(path.read_text(), filename=str(path))
        examples = sorted((ROOT / "examples").glob("*.py"))
        self.assertEqual(len(examples), 5)
        for path in examples:
            ast.parse(path.read_text(), filename=str(path))

    def test_wheel_build_is_byte_reproducible_and_complete(self) -> None:
        backend = load_build_backend()
        with tempfile.TemporaryDirectory() as first, tempfile.TemporaryDirectory() as second:
            first_name = backend.build_wheel(first)
            second_name = backend.build_wheel(second)
            first_bytes = (Path(first) / first_name).read_bytes()
            second_bytes = (Path(second) / second_name).read_bytes()
            self.assertEqual(first_bytes, second_bytes)
            with zipfile.ZipFile(Path(first) / first_name) as archive:
                names = set(archive.namelist())
                self.assertIn("layerx_sdk/generated/client.py", names)
                self.assertIn("layerx_sdk/generated/client.pyi", names)
                self.assertIn("layerx_sdk/py.typed", names)
                self.assertEqual(
                    len([name for name in names if "/share/layerx-sdk/examples/" in name]),
                    5,
                )
                metadata = archive.read("layerx_sdk-0.1.0.dist-info/METADATA").decode()
                self.assertIn("Requires-Python: >=3.11", metadata)


if __name__ == "__main__":
    unittest.main()
