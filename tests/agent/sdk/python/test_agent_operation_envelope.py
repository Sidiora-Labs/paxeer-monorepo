import json
import os
import secrets
import ssl
import sys
import time
import unittest
from collections.abc import Callable
from http.client import HTTPSConnection
from pathlib import Path
from urllib.error import HTTPError
from urllib.parse import urlsplit
from urllib.request import HTTPSHandler, Request, build_opener

sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "services/agent/sdk/python"))

from layerx_sdk.agent_http import (  # noqa: E402
    AgentDaemonEnvelopeTransport,
    AgentEnvelopeSuccess,
    AgentEnvelopeTransport,
    AgentSessionCredential,
    LayerXKeyCredential,
    _envelope_reply,
    check_proof_bundle_response,
    encode_proof_bundle_request,
)
from layerx_sdk.generated.client import encode_native_prepare_request, encode_native_approval_decision

from layerx_sdk.production import (  # noqa: E402
    IdempotencyKey,
    PlatformSdkError,
    ProductionClient,
    SdkErrorCode,
    SecretBytes,
)

_CASE_ENV = "PAXEER_X_AGENT_ENVELOPE_CASE"
_ROUTE = "/v1/agent/rpc"
_KEYS = frozenset({
    "endpoint", "server_name", "ca_pem", "ca_der", "gateway_api_key_file", "program_bearer_file",
    "credential_file", "requests", "operations", "cases", "phase", "state_file", "response_dir",
    "retry_state_file", "daemon_endpoint", "client_cert_file", "client_key_file", "server_ca_file",
    "admission_file", "recovery_file", "admission_public_key_file",
})
_DAEMON_KEYS = frozenset({"daemon_endpoint", "client_cert_file", "client_key_file", "server_ca_file"})
_PHASES = frozenset({"read", "pre-restart", "post-restart"})
_REFUSALS = frozenset({
    SdkErrorCode.POLICY_REFUSAL,
    SdkErrorCode.CAPABILITY_REFUSAL,
    SdkErrorCode.BUDGET_REFUSAL,
})


class ProbeRefused(Exception):
    pass


def _absolute_file(case: dict[str, object], name: str) -> Path:
    value = case.get(name)
    if not isinstance(value, str) or not os.path.isabs(value) or not os.path.isfile(value):
        raise ProbeRefused(f"{name} must name an existing absolute file")
    return Path(value)


class Probe:
    def __init__(self, case_path: str) -> None:
        checker = unittest.TestCase()
        self.assertEqual = checker.assertEqual
        self.assertIn = checker.assertIn
        self.assertIsInstance = checker.assertIsInstance
        self.assertIsNotNone = checker.assertIsNotNone
        self.assertTrue = checker.assertTrue
        self.assertRaises = checker.assertRaises
        with open(case_path, "rb") as source:
            case = json.loads(source.read().decode("utf-8"))
        if not isinstance(case, dict) or not set(case) <= _KEYS or not _KEYS - {"state_file", "retry_state_file"} - _DAEMON_KEYS <= set(case):
            raise ProbeRefused("case file keys do not match the frozen protocol")
        if case["phase"] not in _PHASES:
            raise ProbeRefused("unknown phase")
        endpoint = case["endpoint"]
        if not isinstance(endpoint, str) or not endpoint.startswith("https://") or not endpoint.endswith(_ROUTE):
            raise ProbeRefused("endpoint must be the https gateway agent route")
        self.base = endpoint[: -len(_ROUTE)]
        self.endpoint = endpoint
        self.ca_pem = str(_absolute_file(case, "ca_pem"))
        _absolute_file(case, "ca_der")
        key_line = _absolute_file(case, "gateway_api_key_file").read_bytes().decode("ascii")
        if not key_line.endswith("\n") or key_line.count("\n") != 1 or ":" not in key_line:
            raise ProbeRefused("gateway_api_key_file must be one line <key_id>:<secret>")
        self.key_id, secret = key_line[:-1].split(":", 1)
        self.secret = secret.encode("ascii")
        credential = json.loads(_absolute_file(case, "credential_file").read_bytes().decode("utf-8"))
        if not isinstance(credential, dict) or set(credential) != {"tenant", "session_id", "token_id", "generation"}:
            raise ProbeRefused("credential_file must hold exactly tenant, session_id, token_id, generation")
        generation = credential["generation"]
        if not isinstance(generation, str) or not generation.isascii() or not generation.isdigit() or str(int(generation)) != generation:
            raise ProbeRefused("credential generation must be a canonical decimal string")
        self.credential = credential
        requests = case["requests"]
        cases = case["cases"]
        operations = case["operations"]
        if not isinstance(requests, dict) or not isinstance(cases, list) or not cases or any(not isinstance(item, str) for item in cases):
            raise ProbeRefused("requests and cases must be an object and a non-empty array of ids")
        if not isinstance(operations, list) or len(operations) != 50 or operations != sorted(operations):
            raise ProbeRefused("operations must be the 50 sorted catalogue names")
        self.requests = requests
        self.cases = cases
        response_dir = case["response_dir"]
        if not isinstance(response_dir, str) or not os.path.isabs(response_dir) or not os.path.isdir(response_dir):
            raise ProbeRefused("response_dir must name an existing absolute directory")
        self.response_dir = Path(response_dir)
        self.phase = case["phase"]
        self.restart_evidence = {name: case.get(name) for name in
                                 ("admission_file", "recovery_file", "admission_public_key_file")}
        state_file = case.get("state_file")
        if self.phase != "read" and (not isinstance(state_file, str) or not os.path.isabs(state_file)):
            raise ProbeRefused("state_file must name an absolute path for restart phases")
        self.state_file = None if state_file is None else Path(str(state_file))
        retry_state_file = case.get("retry_state_file")
        if retry_state_file is not None and (not isinstance(retry_state_file, str) or not os.path.isabs(retry_state_file)):
            raise ProbeRefused("retry_state_file must name an absolute path")
        self.retry_state_file = None if retry_state_file is None else Path(retry_state_file)
        present = _DAEMON_KEYS & set(case)
        if present and present != _DAEMON_KEYS:
            raise ProbeRefused("daemon_endpoint, client_cert_file, client_key_file and server_ca_file must be given together")
        if present:
            daemon_endpoint = case["daemon_endpoint"]
            if not isinstance(daemon_endpoint, str) or not daemon_endpoint.startswith("https://"):
                raise ProbeRefused("daemon_endpoint must be an https base")
            self.daemon: tuple[str, str, str, str] | None = (
                daemon_endpoint,
                str(_absolute_file(case, "client_cert_file")),
                str(_absolute_file(case, "client_key_file")),
                str(_absolute_file(case, "server_ca_file")),
            )
        else:
            self.daemon = None

    def _key(self) -> LayerXKeyCredential:
        return LayerXKeyCredential(self.key_id, SecretBytes(self.secret))

    def _session(self, **override: object) -> AgentSessionCredential:
        coordinates: dict[str, object] = {
            "tenant": self.credential["tenant"],
            "session_id": self.credential["session_id"],
            "token_id": self.credential["token_id"],
            "generation": int(self.credential["generation"]),
        }
        coordinates.update(override)
        return AgentSessionCredential(**coordinates)  # type: ignore[arg-type]

    def _client(self, session: AgentSessionCredential | None = None) -> ProductionClient:
        return ProductionClient(AgentEnvelopeTransport(
            self.base,
            gateway_key=self._key(),
            session=session if session is not None else self._session(),
            ca_file=self.ca_pem,
        ))

    def _daemon_client(self) -> ProductionClient:
        if self.daemon is None:
            raise ProbeRefused("daemon-surface cases need daemon_endpoint, client_cert_file, client_key_file and server_ca_file")
        endpoint, cert_file, key_file, ca_file = self.daemon
        context = ssl.create_default_context(cafile=ca_file)
        context.load_cert_chain(cert_file, key_file)
        return ProductionClient(AgentDaemonEnvelopeTransport(endpoint, ssl_context=context, session=self._session()))

    def _request(self, case_id: str) -> dict[str, object]:
        request = self.requests.get(case_id)
        if not isinstance(request, dict) or not {"operation", "request"} <= set(request) <= {"operation", "request", "idempotency_key"}:
            raise ProbeRefused(f"requests has no valid entry for {case_id}")
        if not isinstance(request["operation"], str) or not isinstance(request["request"], dict):
            raise ProbeRefused(f"requests entry for {case_id} is malformed")
        return request

    def _record(self, case_id: str, status: int, body: object) -> None:
        (self.response_dir / f"{case_id}.json").write_text(json.dumps({"status": status, "body": body}))

    def _success(self, case_id: str, entry: str | None = None, client: ProductionClient | None = None) -> AgentEnvelopeSuccess:
        request = self._request(entry or case_id)
        key = request.get("idempotency_key")
        result = (client or self._client()).agent(
            request["operation"],  # type: ignore[arg-type]
            request["request"],
            idempotency_key=None if key is None else IdempotencyKey(str(key)),
        )
        self.assertIsInstance(result, AgentEnvelopeSuccess)
        assert isinstance(result, AgentEnvelopeSuccess)
        self.assertTrue(result.request_id.isdigit())
        self.assertIn(result.verification_status.get("state"), {"achieved", "unverified"})
        self._record(case_id, 200, {
            "request_id": result.request_id,
            "value": result.value,
            "verification_status": dict(result.verification_status),
        })
        return result

    def _refusal(self, case_id: str, session: AgentSessionCredential) -> None:
        request = self._request("read")
        with self.assertRaises(PlatformSdkError) as raised:
            self._client(session).agent(request["operation"], request["request"])  # type: ignore[arg-type]
        self.assertIn(raised.exception.code, _REFUSALS)
        self.assertIsNotNone(raised.exception.request_id)
        self._record(case_id, 0, raised.exception.to_dict())

    def _raw(self, case_id: str, body: bytes) -> dict[str, object]:
        context = ssl.create_default_context(cafile=self.ca_pem)
        request = Request(
            self.endpoint,
            data=body,
            method="POST",
            headers={
                "Accept": "application/json",
                "Content-Type": "application/json",
                "Content-Length": str(len(body)),
                "Authorization": self._key().use(),
            },
        )
        try:
            with build_opener(HTTPSHandler(context=context)).open(request, timeout=30) as response:
                status, parsed = response.status, json.loads(response.read())
        except HTTPError as error:
            try:
                status, parsed = error.code, json.loads(error.read())
            finally:
                error.close()
        self._record(case_id, status, parsed)
        return {"status": status, "body": parsed}

    def _raw_envelope(self, **override: object) -> bytes:
        request = self._request("read")
        envelope: dict[str, object] = {
            "version": 1,
            "request_id": "7",
            "operation": request["operation"],
            "request": request["request"],
            "credential": dict(self.credential),
            "idempotency_key": None,
        }
        envelope.update(override)
        return json.dumps(envelope).encode()

    def _raw_refusal(self, case_id: str, body: bytes, status: int, reason: str, request_id: str | None = None) -> None:
        raw = self._raw(case_id, body)
        error = raw["body"]
        assert isinstance(error, dict)
        self.assertEqual((raw["status"], error.get("class"), error.get("reason")), (status, "ProtocolIncompatibility", reason))
        if request_id is not None:
            self.assertEqual(error.get("request_id"), request_id)

    def proof_bundle_case(self, case_id: str) -> None:
        entry = self.requests.get(case_id)
        if not isinstance(entry, dict) or not isinstance(entry.get("request"), dict):
            raise ProbeRefused("proof request entry is absent")
        self.assertEqual(entry.get("operation"), "read.proof_bundle")
        self.assertTrue("idempotency_key" not in entry)
        request = entry["request"]
        transport = AgentEnvelopeTransport(self.base, gateway_key=self._key(), session=self._session(), ca_file=self.ca_pem)
        observed: list[tuple[int, object]] = []
        real_opener = transport._opener

        def observe_response(response):
            real_read = response.read

            def read(count=-1):
                body = real_read(count)
                parsed = json.loads(body.decode("utf-8"))
                observed.append((response.status, parsed))
                self._record(case_id, response.status, parsed)
                return body

            response.read = read
            return response

        class ResponseObserver:
            def open(self, *args, **kwargs):
                try:
                    return observe_response(real_opener.open(*args, **kwargs))
                except HTTPError as error:
                    observe_response(error)
                    raise

        transport._opener = ResponseObserver()
        if case_id.startswith("proof_bundle.malformed_request."):
            with self.assertRaises(PlatformSdkError) as raised:
                transport.call("agent", "read.proof_bundle", request, None)
            self.assertEqual(raised.exception.code, SdkErrorCode.INVALID_ARGUMENT)
            self.assertEqual(observed, [])
            return
        if case_id.startswith("proof_bundle.refusal."):
            with self.assertRaises(PlatformSdkError) as raised:
                transport.read_proof_bundle(encode_proof_bundle_request(request))
            self.assertIsNotNone(raised.exception.request_id)
            self.assertEqual(len(observed), 1)
            status, body = observed[0]
            self.assertEqual(status, entry.get("expected_status", 403))
            self.assertEqual(body["class"], entry["expected_class"])
            self.assertEqual(body["reason"], entry["expected_reason"])
            return
        typed = encode_proof_bundle_request(request)
        typed_response = transport.read_proof_bundle(typed)
        response = AgentEnvelopeSuccess(typed_response.request_id, typed_response.value, typed_response.verification_status)
        self.assertEqual(len(observed), 1)
        self.assertEqual(observed[0][0], 200)
        proof = check_proof_bundle_response(typed, response)
        self.assertEqual(proof.variant, entry["expected_variant"])
        fields = [proof.canonical_value, proof.native_proof]
        if proof.variant == 4:
            self.assertIsNotNone(proof.activity_receipt)
            self.assertIsNotNone(proof.activity_receipt_sp1)
            fields.extend([proof.activity_receipt, proof.activity_receipt_sp1])
        framed = b"LXPB1" + bytes([proof.variant]) + b"".join(len(value).to_bytes(4, "big") + value for value in fields)
        value = response.value
        self.assertEqual(framed.hex(), value["value"]["proofs"][0])

        def reject(mutated, status=response.verification_status):
            with self.assertRaises(PlatformSdkError) as raised:
                check_proof_bundle_response(typed, AgentEnvelopeSuccess(response.request_id, mutated, status))
            self.assertEqual(raised.exception.code, SdkErrorCode.DECODE_FAILURE)

        def reject_record(raw):
            reject({**value, "value": {**value["value"], "proofs": [raw.hex()]}})

        bad_magic, bad_variant, empty_field, overrun = (bytearray(framed) for _ in range(4))
        bad_magic[0], bad_variant[5] = 0, 0
        empty_field[6:10] = (0).to_bytes(4, "big")
        overrun[6:10] = len(framed).to_bytes(4, "big")
        oversized = (framed * ((262_145 + len(framed) - 1) // len(framed)))[:262_145]
        for raw in [framed[:-1], framed + b"\0", bad_magic, bad_variant, empty_field, overrun, oversized]:
            reject_record(raw)
        reject({**value, "value": {**value["value"], "proofs": []}})
        reject({**value, "value": {**value["value"], "proofs": value["value"]["proofs"] * 2}})
        other = bytearray.fromhex(typed["target"])
        other[3] ^= 1
        reject({**value, "value": {**value["value"], "target": other.hex()}})
        other_rung = "receipted" if value["achieved_verification_level"] == "committed" else "committed"
        reject({**value, "achieved_verification_level": other_rung})
        reject(value, {"state": "achieved", "rung": other_rung})
        reject(value, {"state": "achieved", "level": value["achieved_verification_level"]})
        reject({**value, "unexpected": True})
        for head in [0, "00", "18446744073709551616"]:
            reject({**value, "freshness": {**value["freshness"], "chain_head": head}})
        reject({**value, "freshness": {**value["freshness"], "relative_to": {"checkpoint": value["freshness"]["latest_finalised_checkpoint"]}}})
        if proof.variant == 4:
            reject_record(framed[:6 + 4 + len(proof.canonical_value) + 4 + len(proof.native_proof)])
        self.assertEqual(len(observed), 1)

    def native_case(self, case_id: str) -> None:
        entry = self.requests.get(case_id)
        if not isinstance(entry, dict) or not isinstance(entry.get("request"), dict):
            raise ProbeRefused("native request entry is absent")
        operation, request = entry.get("operation"), entry["request"]
        if operation not in {"prepare", "approval.list", "approval.get", "approval.approve", "approval.reject"}:
            raise ProbeRefused("native operation is invalid")
        key = None if "idempotency_key" not in entry else IdempotencyKey(entry["idempotency_key"])
        transport = AgentEnvelopeTransport(self.base, gateway_key=self._key(), session=self._session(), ca_file=self.ca_pem)
        observed: list[tuple[int, object]] = []
        real_opener = transport._opener

        def observe_response(response):
            real_read = response.read
            def read(count=-1):
                body = real_read(count)
                parsed = json.loads(body.decode("utf-8"))
                observed.append((response.status, parsed))
                self._record(case_id, response.status, parsed)
                return body
            response.read = read
            return response

        class ResponseObserver:
            def open(self, *args, **kwargs):
                try:
                    return observe_response(real_opener.open(*args, **kwargs))
                except HTTPError as error:
                    observe_response(error)
                    raise

        transport._opener = ResponseObserver()
        if case_id.startswith("native_refusal."):
            with self.assertRaises(PlatformSdkError) as raised:
                transport.call("agent", operation, request, key)
            self.assertIsNotNone(raised.exception.request_id)
            self.assertEqual(len(observed), 1)
            status, body = observed[0]
            self.assertTrue(status >= 400)
            self.assertEqual(body["class"], entry["expected_class"])
            self.assertEqual(body["reason"], entry["expected_reason"])
            return
        if case_id == "native_prepare":
            self.assertEqual(operation, "prepare")
            self.assertIsNotNone(key)
            result = transport.prepare_native(encode_native_prepare_request(request), key)
            self.assertEqual(result.value["approval_required"], entry["expected_approval_required"])
        elif case_id == "native_approval_list":
            self.assertEqual(operation, "approval.list")
            self.assertEqual(key, None)
            result = transport.approval_list_native()
            self.assertEqual(sum(row["approval_id"] == entry["expected_approval_id"] for row in result.value["approvals"]), 1)
        elif case_id == "native_approval_get":
            self.assertEqual(operation, "approval.get")
            self.assertEqual(key, None)
            result = transport.approval_get_native(request["approval_id"])
            self.assertEqual(result.value["state"], entry["expected_state"])
            self.assertEqual(result.value["held_digest"], entry["expected_held_digest"])
        else:
            expected = "approval.approve" if case_id == "native_approval_approve" else "approval.reject"
            self.assertEqual(operation, expected)
            self.assertIn(case_id, {"native_approval_approve", "native_approval_reject"})
            self.assertIsNotNone(key)
            result = transport.approval_decide_native(encode_native_approval_decision(request), operation == "approval.approve", key)
            self.assertEqual(result.value["state"], entry["expected_state"])
            self.assertEqual(result.value["held_digest"], entry["expected_held_digest"])
        self.assertEqual(len(observed), 1)
        self.assertEqual(observed[0][0], 200)

    def case_read(self) -> None:
        self._success("read")

    def case_program_read(self) -> None:
        self._success("program_read")

    def case_approval_list(self) -> None:
        self._success("approval_list")

    def case_daemon_read(self) -> None:
        self._success("daemon_read", "read", self._daemon_client())

    def case_read_decode_failure(self) -> None:
        request = self._request("read_decode_failure")
        if "idempotency_key" in request:
            raise ProbeRefused("read_decode_failure must be a non-mutating request")
        with self.assertRaises(PlatformSdkError) as raised:
            self._client().agent(request["operation"], request["request"])  # type: ignore[arg-type]
        self.assertEqual(raised.exception.code, SdkErrorCode.DECODE_FAILURE)
        self.assertEqual(raised.exception.retry, "never")
        self._record("read_decode_failure", 0, raised.exception.to_dict())

    def case_mutation_decode_unknown(self) -> None:
        request = self._request("mutation_decode_unknown")
        key = request.get("idempotency_key")
        if not isinstance(key, str):
            raise ProbeRefused("mutation_decode_unknown carries no idempotency_key")
        with self.assertRaises(PlatformSdkError) as raised:
            self._client().agent(request["operation"], request["request"], idempotency_key=IdempotencyKey(key))  # type: ignore[arg-type]
        self.assertEqual(raised.exception.code, SdkErrorCode.UNKNOWN_OUTCOME)
        self.assertEqual(raised.exception.retry, "unknown-outcome")
        self._record("mutation_decode_unknown", 0, raised.exception.to_dict())

    def case_allowed_mutation(self) -> None:
        self._success("allowed_mutation")

    def case_mutation_duplicate_same_result(self) -> None:
        first = self._success("mutation_duplicate_same_result", "allowed_mutation")
        second = self._success("mutation_duplicate_same_result", "allowed_mutation")
        self.assertEqual(first.value, second.value)
        self.assertEqual(first.verification_status, second.verification_status)

    def _restart_envelope(self, state: dict[str, object]) -> bytes:
        return json.dumps({
            "version": 1,
            "request_id": state["request_id"],
            "operation": state["operation"],
            "request": state["request"],
            "credential": dict(self.credential),
            "idempotency_key": state["idempotency_key"],
        }, ensure_ascii=True, separators=(",", ":")).encode()

    def case_restart_unknown_pending(self) -> None:
        if self.phase != "pre-restart" or self.state_file is None:
            raise ProbeRefused("restart_unknown_pending needs the pre-restart phase")
        request = self._request("allowed_mutation")
        key = request.get("idempotency_key")
        if not isinstance(key, str):
            raise ProbeRefused("allowed_mutation carries no idempotency_key")
        state: dict[str, object] = {
            "request_id": str(secrets.randbits(64)),
            "idempotency_key": key,
            "operation": request["operation"],
            "request": request["request"],
        }
        body = self._restart_envelope(state)
        parsed = urlsplit(self.endpoint)
        connection = HTTPSConnection(str(parsed.hostname), parsed.port or 443, context=ssl.create_default_context(cafile=self.ca_pem), timeout=30)
        try:
            connection.request("POST", parsed.path, body=body, headers={
                "Accept": "application/json",
                "Content-Type": "application/json",
                "Content-Length": str(len(body)),
                "Authorization": self._key().use(),
            })
            # The controller must fsync and authenticate admission before we drop
            # the response. A locally completed write is not server admission.
            admission = self._restart_record("admission_file", state, "pending")
        finally:
            connection.close()
        state["admission"] = admission
        self.state_file.write_text(json.dumps(state, sort_keys=True))
        self._record("restart_unknown_pending", 0, state)

    def case_restart_unknown_reconcile(self) -> None:
        if self.phase != "post-restart" or self.state_file is None:
            raise ProbeRefused("restart_unknown_reconcile needs the post-restart phase")
        if not self.state_file.is_file():
            raise ProbeRefused("post-restart phase found no pre-restart state_file")
        state = json.loads(self.state_file.read_text())
        if not isinstance(state, dict) or set(state) != {"request_id", "idempotency_key", "operation", "request", "admission"}:
            raise ProbeRefused("state_file is not the restart_unknown_pending record")
        first = self._raw("restart_unknown_reconcile", self._restart_envelope(state))
        second = self._raw("restart_unknown_reconcile", self._restart_envelope(state))
        for raw in (first, second):
            body = raw["body"]
            assert isinstance(body, dict)
            self.assertEqual(raw["status"], 200)
            self.assertEqual(set(body), {"request_id", "value", "verification_status"})
            self.assertEqual(body["request_id"], state["request_id"])
            self.assertIn(body["verification_status"].get("state"), {"achieved", "unverified"})
        self.assertEqual(first["body"]["value"], second["body"]["value"])
        self.assertEqual(first["body"]["verification_status"], second["body"]["verification_status"])
        recovered = self._restart_record("recovery_file", state, "committed")
        self.assertEqual(recovered["operation_id"], state["admission"]["operation_id"])
        self.assertTrue(int(recovered["journal_sequence"]) >= int(state["admission"]["journal_sequence"]))
        self.assertEqual(recovered["value"], first["body"]["value"])

    def _restart_record(self, field: str, state: dict[str, object], phase: str) -> dict[str, object]:
        from hashlib import sha256
        from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

        path = self.restart_evidence.get(field)
        key_path = self.restart_evidence.get("admission_public_key_file")
        if not isinstance(path, str) or not os.path.isabs(path) or not isinstance(key_path, str):
            raise ProbeRefused("authenticated durable journal evidence is required")
        deadline = time.monotonic() + 30
        while not Path(path).is_file():
            if time.monotonic() >= deadline:
                raise ProbeRefused("durable journal evidence was not acknowledged")
            time.sleep(0.01)
        from test_program_http import protected, unique
        key = protected(key_path, 32)
        document = json.loads(protected(path), object_pairs_hook=unique)
        if not isinstance(document, dict) or set(document) != {"record", "signature"}:
            raise ProbeRefused("exact authenticated journal envelope required")
        record = document["record"]
        fields = {"state", "request_sha256", "operation_id", "journal_sequence", "durable", "committed_effects", "value"}
        if not isinstance(record, dict) or set(record) != fields:
            raise ProbeRefused("exact durable journal record required")
        encoded = json.dumps(record, sort_keys=True, separators=(",", ":"), ensure_ascii=True).encode()
        Ed25519PublicKey.from_public_bytes(key).verify(bytes.fromhex(document["signature"]), b"LayerX/restart-admission/v1\0" + encoded)
        sequence = record["journal_sequence"]
        identity = record["operation_id"]
        if (record["state"] != phase or record["durable"] is not True
                or type(record["committed_effects"]) is not int
                or record["committed_effects"] != (0 if phase == "pending" else 1)
                or record["request_sha256"] != sha256(self._restart_envelope(state)).hexdigest()
                or not isinstance(identity, str) or len(identity) != 64
                or any(c not in "0123456789abcdef" for c in identity)
                or not isinstance(sequence, str) or not sequence.isascii() or not sequence.isdigit()
                or str(int(sequence)) != sequence or not 0 < int(sequence) < 1 << 64):
            raise ProbeRefused("durable admission or recovered effect binding failed")
        return record

    def case_restart_retry_same_result(self) -> None:
        if self.retry_state_file is None:
            raise ProbeRefused("restart_retry_same_result needs retry_state_file in the case file")
        if self.phase == "read":
            raise ProbeRefused("restart_retry_same_result needs the pre-restart or post-restart phase")
        result = self._success("restart_retry_same_result", "allowed_mutation")
        observed = {"value": result.value, "verification_status": dict(result.verification_status)}
        if self.phase == "pre-restart":
            self.retry_state_file.write_text(json.dumps(observed, sort_keys=True))
            return
        if not self.retry_state_file.is_file():
            raise ProbeRefused("post-restart phase found no pre-restart retry_state_file")
        self.assertEqual(json.loads(self.retry_state_file.read_text()), json.loads(json.dumps(observed, sort_keys=True)))

    def case_missing_idempotency_key(self) -> None:
        request = self._request("allowed_mutation")
        with self.assertRaises(PlatformSdkError) as raised:
            self._client().agent(request["operation"], request["request"])  # type: ignore[arg-type]
        self.assertEqual(raised.exception.code, SdkErrorCode.IDEMPOTENCY_REQUIRED)
        self._record("missing_idempotency_key", 0, raised.exception.to_dict())

    def case_wrong_generation(self) -> None:
        generation = int(self.credential["generation"])
        self._refusal("wrong_generation", self._session(generation=generation + 1 if generation < (1 << 64) - 1 else generation - 1))

    def case_wrong_tenant(self) -> None:
        tenant = str(self.credential["tenant"])
        self._refusal("wrong_tenant", self._session(tenant=tenant[:-1] + ("a" if tenant[-1] != "a" else "b")))

    def case_wrong_session(self) -> None:
        session_id = str(self.credential["session_id"])
        self._refusal("wrong_session", self._session(session_id=("1" if session_id[0] == "0" else "0") + session_id[1:]))

    def case_wrong_token(self) -> None:
        token_id = str(self.credential["token_id"])
        self._refusal("wrong_token", self._session(token_id=("1" if token_id[0] == "0" else "0") + token_id[1:]))

    def case_faucet_retired(self) -> None:
        with self.assertRaises(PlatformSdkError) as raised:
            self._client().agent("faucet.claim", {})
        self.assertEqual(raised.exception.code, SdkErrorCode.UNAVAILABLE_CAPABILITY)
        self.assertEqual(raised.exception.retry, "never")
        self.assertIsNotNone(raised.exception.request_id)
        self._record("faucet_retired", 0, raised.exception.to_dict())

    def case_unknown_operation(self) -> None:
        with self.assertRaises(PlatformSdkError) as raised:
            self._client().agent("faucet.unknown", {})  # type: ignore[arg-type]
        self.assertEqual(raised.exception.code, SdkErrorCode.UNAVAILABLE_CAPABILITY)
        self._raw_refusal("unknown_operation", self._raw_envelope(operation="faucet.unknown"), 404, "envelope.unknown_operation")

    def case_malformed_body(self) -> None:
        self._raw_refusal("malformed_body", b'{"version":1,', 400, "envelope.malformed", "0")

    def case_unknown_field(self) -> None:
        self._raw_refusal("unknown_field", self._raw_envelope(principal="forged"), 400, "envelope.unknown_field")

    def case_bad_version(self) -> None:
        self._raw_refusal("bad_version", self._raw_envelope(version=2), 400, "envelope.version")

    def case_noncanonical_integer(self) -> None:
        credential = dict(self.credential)
        credential["generation"] = "0" + str(credential["generation"])
        self._raw_refusal("noncanonical_integer", self._raw_envelope(credential=credential), 400, "envelope.noncanonical_integer")


def test_agent_operation_envelope_reads_daemon_rung() -> None:
    golden = Path(__file__).resolve().parents[4] / "services/agent/schema/agent-api/golden/agent-http-success.json"
    encoded = golden.read_bytes()
    expected = json.loads(encoded)

    response = _envelope_reply(200, "application/json", encoded, "7", False)

    assert isinstance(response, AgentEnvelopeSuccess)
    assert response.request_id == expected["request_id"]
    assert response.value == expected["value"]
    assert response.verification_status == {"state": "achieved", "rung": "submitted"}


def main() -> int:
    path = os.environ.get(_CASE_ENV)
    if not path or not os.path.isabs(path) or not os.path.isfile(path):
        print(f"agent operation envelope probe refused: {_CASE_ENV} must name an absolute case file", file=sys.stderr)
        return 2
    try:
        probe = Probe(path)
    except (ProbeRefused, OSError, ValueError, UnicodeDecodeError) as refusal:
        print(f"agent operation envelope probe refused: {refusal}", file=sys.stderr)
        return 2
    passed = 0
    failed = False
    for case_id in probe.cases:
        handler: Callable[[], None] | None = (lambda: probe.proof_bundle_case(case_id)) if case_id.startswith("proof_bundle.") else (lambda: probe.native_case(case_id)) if case_id.startswith("native_") else getattr(probe, "case_" + case_id, None) if case_id.isidentifier() else None
        if handler is None:
            print(f"agent operation envelope probe: case {case_id} is not implemented by the Python probe", file=sys.stderr)
            failed = True
            continue
        try:
            handler()
        except (AssertionError, PlatformSdkError, ProbeRefused, OSError, ValueError) as failure:
            print(f"agent operation envelope probe: case {case_id} failed: {type(failure).__name__}: {failure}", file=sys.stderr)
            failed = True
            continue
        passed += 1
        print(f"PAXEER_X_AGENT_ENVELOPE_CASE {case_id} passed", flush=True)
    print(f"PAXEER_X_AGENT_ENVELOPE_CASES={passed}", flush=True)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
