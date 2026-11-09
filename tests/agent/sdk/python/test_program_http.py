from __future__ import annotations

import importlib.util
import json
import os
import stat
import time
import unittest
from dataclasses import dataclass, replace
from hashlib import sha256
from pathlib import Path

from layerx_sdk.agent_http import AgentHttpTransport, LayerXKeyCredential
from layerx_sdk.native_program_call import NativeProgramCall, decode_native_program_call, encode_native_program_call
from layerx_sdk.production import PlatformSdkError, ProductionClient, SecretBytes
from layerx_sdk.programs import ProgramOperations, ProgramTrustContext, _discovery, _verify_program_transaction
from layerx_sdk.evm_transaction import activity_id, decode_signed
from layerx_sdk.ai_market import _decode_programs_call, _encode_programs_call, decode_programs_call_result
from layerx_sdk.x402_rpc import PaymentRpc
from layerx_sdk.verifier import ReceiptKey

ROOT = Path(__file__).resolve().parents[4]


def protected(path: str | Path, maximum: int = 1_048_576) -> bytes:
    path = Path(path)
    info = path.lstat()
    parent = path.parent.lstat()
    if (not path.is_absolute() or path.resolve() != path or not stat.S_ISREG(info.st_mode)
            or info.st_uid != os.geteuid() or info.st_nlink != 1 or stat.S_IMODE(info.st_mode) != 0o600
            or parent.st_uid != os.geteuid() or stat.S_IMODE(parent.st_mode) != 0o700
            or not 0 < info.st_size <= maximum):
        raise ValueError("protected real Programs fixture input required")
    return path.read_bytes()


def unique(items: list[tuple[str, object]]) -> dict[str, object]:
    result = {}
    for key, value in items:
        if key in result:
            raise ValueError("duplicate Programs fixture field")
        result[key] = value
    return result


@dataclass(frozen=True)
class ProgramTransaction:
    raw: bytes
    native_call: NativeProgramCall
    fee_limit: int

    @property
    def program_id(self):
        return self.native_call.program_id.hex()

    @property
    def transaction_hash(self):
        return "0x" + activity_id(self.raw).hex()


class ProgramHttp(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        path = os.environ.get("LAYERX_PYTHON_PROGRAM_HTTP_FIXTURE")
        if not path:
            raise RuntimeError("actual protected gateway/native cluster fixture is required; no skipped HTTP evidence")
        cls.fixture = json.loads(protected(path), object_pairs_hook=unique)
        required = {"version", "isolated", "approved_program_calls", "endpoint", "network_id",
                    "sequencer_public_key", "gateway_key_id", "gateway_key_file", "gateway_pid",
                    "gateway_source_manifest", "node_pid", "node_source_manifest", "node_rpc", "receipt_keys", "programs", "refused_call", "unknown_call"}
        if (set(cls.fixture) != required or cls.fixture["version"] != 2 or cls.fixture["isolated"] is not True
                or cls.fixture["approved_program_calls"] is not True
                or type(cls.fixture["network_id"]) is not int or not 0 < cls.fixture["network_id"] < 1 << 32
                or type(cls.fixture["gateway_pid"]) is not int or cls.fixture["gateway_pid"] <= 0
                or type(cls.fixture["node_pid"]) is not int or cls.fixture["node_pid"] <= 0):
            raise ValueError("exact approved disposable real HTTP fixture contract required")
        manifest = json.loads(protected(cls.fixture["gateway_source_manifest"]), object_pairs_hook=unique)
        executable = Path(os.readlink(f'/proc/{cls.fixture["gateway_pid"]}/exe'))
        if (executable.name != "layerx-gateway" or not executable.is_file()
                or manifest["binary_sha256"] != sha256(executable.read_bytes()).hexdigest()):
            raise ValueError("actual prebuilt production gateway process required")
        for relative in ("services/platform/hosted/gateway/src/main.rs", "services/platform/hosted/gateway/src/routes.rs", "services/platform/hosted/gateway/src/paxeer.rs"):
            if manifest["sources"][relative] != sha256((ROOT / relative).read_bytes()).hexdigest():
                raise ValueError("gateway process source differs from actual candidate")
        node_manifest = json.loads(protected(cls.fixture["node_source_manifest"]), object_pairs_hook=unique)
        node_executable = Path(os.readlink(f'/proc/{cls.fixture["node_pid"]}/exe'))
        if (node_executable.name != "paxd" or not node_executable.is_file()
                or node_manifest["binary_sha256"] != sha256(node_executable.read_bytes()).hexdigest()):
            raise ValueError("actual prebuilt production node process required")
        for relative in ("node/upgrades.go", "precompiles/layerxprograms/method_call.go",
                         "precompiles/layerxprograms/method_simulate.go", "precompiles/layerxprograms/principal.go"):
            if node_manifest["sources"][relative] != sha256((ROOT / relative).read_bytes()).hexdigest():
                raise ValueError("node process source differs from actual candidate")
        cls.rpc = PaymentRpc(cls.fixture["node_rpc"])
        receipt_keys = tuple(ReceiptKey(bytes.fromhex(row["key_id"]), bytes.fromhex(row["public_key"]),
                                       row["first_height"], row["last_height"])
                             for row in cls.fixture["receipt_keys"])
        spec = importlib.util.spec_from_file_location("program_http_signatures", ROOT / "sdk/integrations/fastapi/layerx_fastapi/signatures.py")
        if spec is None or spec.loader is None:
            raise ValueError("actual signature verifier unavailable")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        cls.signatures = module.LayerXSignatureVerifier()
        cls.key = SecretBytes(protected(cls.fixture["gateway_key_file"], 128))
        cls.transport = AgentHttpTransport(cls.fixture["endpoint"], credential=LayerXKeyCredential(cls.fixture["gateway_key_id"], cls.key), timeout=15)
        cls.client = ProductionClient(cls.transport)
        cls.trust = ProgramTrustContext(bytes.fromhex(cls.fixture["sequencer_public_key"]), protocol_version=3, network_id=cls.fixture["network_id"], receipt_keys=receipt_keys)
        cls.operations = ProgramOperations(cls.client, cls.signatures, cls.trust)
        if not isinstance(cls.fixture["programs"], list) or {row["guest_abi"] for row in cls.fixture["programs"]} != {1, 2, 3, 4, 5} or len(cls.fixture["programs"]) != 5:
            raise ValueError("all five real registered ABI paths required")

    @classmethod
    def tearDownClass(cls) -> None:
        cls.key.destroy()

    def request(self, row: dict[str, object]) -> ProgramTransaction:
        self.assertEqual(set(row), {"guest_abi", "payload_file", "raw_transaction_file", "fee_limit", "expected_result_code"})
        payload = protected(row["payload_file"])
        native = decode_native_program_call(payload)
        self.assertEqual(encode_native_program_call(native), payload)
        self.assertEqual(native.guest_abi, row["guest_abi"])
        raw = protected(row["raw_transaction_file"])
        transaction, signature = decode_signed(raw)
        self.assertEqual(transaction.chain_id, self.trust.network_id)
        self.assertEqual(transaction.to.hex(), "000000000000000000000000000000000000101d")
        self.assertEqual(transaction.value, 0)
        fee = row["fee_limit"]
        self.assertIsInstance(fee, str)
        self.assertEqual(str(int(fee)), fee)
        self.assertEqual(_encode_programs_call(native, int(fee)), transaction.data)
        self.assertEqual(_decode_programs_call(transaction.data), (native, int(fee)))
        self.assertEqual(transaction.attach_signature(signature, transaction.recover_signer(signature)), raw)
        return ProgramTransaction(raw, native, int(fee))

    def verified_receipt(self, request):
        receipt = self.rpc.receipt(request.transaction_hash)
        self.assertIsNotNone(receipt)
        self.assertEqual(receipt["txHash"], request.transaction_hash)
        verified = _verify_program_transaction(bytes.fromhex(receipt["receipt"][2:]), request.raw,
                                               self.trust.receipt_keys, self.trust.network_id, request.program_id)
        self.assertEqual(verified.receipt.activity_id.hex(), request.transaction_hash[2:])
        return receipt

    def test_real_unified_http_discovery_interface_simulation_call_and_recovery(self) -> None:
        for row in self.fixture["programs"]:
            with self.subTest(abi=row["guest_abi"]):
                request = self.request(row)
                discovered = self.operations.discover(request.program_id)
                self.assertEqual(discovered.abi_version, request.native_call.guest_abi)
                self.assertTrue(discovered.sequencer_signature_verified)
                interface = self.operations.interface(request.program_id)
                self.assertEqual(interface.abi_version, discovered.abi_version)
                self.assertEqual(interface.code_hash, discovered.code_hash)
                self.assertEqual(interface.receipt_digest, discovered.deployment_receipt_digest)
                self.assertEqual(sha256(interface.interface).hexdigest(), interface.interface_digest)
                transaction, _ = decode_signed(request.raw)
                output = self.rpc.call("eth_call", [{"to": "0x" + transaction.to.hex(),
                                                     "data": "0x" + transaction.data.hex()}, "latest"])
                result_code, _, _ = decode_programs_call_result(bytes.fromhex(output[2:]), request.native_call.response_capacity)
                self.assertEqual(result_code, row["expected_result_code"])
                self.assertEqual(self.rpc.send("0x" + request.raw.hex()), request.transaction_hash)
                receipt = self.verified_receipt(request)
                self.assertEqual(self.rpc.send("0x" + request.raw.hex()), request.transaction_hash)
                self.assertEqual(self.verified_receipt(request), receipt)

    def test_real_discovery_signature_pin_and_signed_call_refusals(self) -> None:
        row = next(row for row in self.fixture["programs"] if row["guest_abi"] == 3)
        request = self.request(row)
        document = self.client.agent("program.discover", {"program_id": request.program_id, "requested_verification_level": "sequencer-signed"})
        _discovery(document, request.program_id, self.trust.now_milliseconds(), self.signatures, self.trust)
        for field in ("code_hash", "state_root", "discovery_public_key", "discovery_signature"):
            changed = dict(document)
            original = changed[field]
            changed[field] = ("0" if original[0] != "0" else "1") + original[1:]
            with self.subTest(field=field), self.assertRaises(ValueError):
                _discovery(changed, request.program_id, self.trust.now_milliseconds(), self.signatures, self.trust)
        for field in ("discovery_public_key", "discovery_signature"):
            changed = dict(document)
            del changed[field]
            with self.assertRaises(ValueError):
                _discovery(changed, request.program_id, self.trust.now_milliseconds(), self.signatures, self.trust)
        with self.assertRaises(ValueError):
            _discovery(document, request.program_id, int(document["valid_through"]) + 1, self.signatures, self.trust)
        for changed in (request.raw[:-1], request.raw + b"\0", b"\x03" + request.raw[1:]):
            with self.assertRaises(ValueError):
                decode_signed(changed)
        for abi in (0, 6, 65535, True, 3.0):
            with self.subTest(abi=abi), self.assertRaises(ValueError):
                encode_native_program_call(replace(request.native_call, guest_abi=abi))
        receipt = self.verified_receipt(request)
        signed = request.raw
        for raw, network, program in ((signed[:-1] + bytes([signed[-1] ^ 1]), self.trust.network_id, request.program_id),
                                       (request.raw, self.trust.network_id + 1, request.program_id),
                                       (request.raw, self.trust.network_id, "00" * 32)):
            with self.assertRaises(ValueError):
                _verify_program_transaction(bytes.fromhex(receipt["receipt"][2:]), raw,
                                            self.trust.receipt_keys, network, program)
        anonymous = ProgramOperations(ProductionClient(AgentHttpTransport(self.fixture["endpoint"], timeout=15)), self.signatures, self.trust)
        with self.assertRaises(PlatformSdkError):
            anonymous.discover(request.program_id)

    def test_real_native_refused_and_unknown_calls_keep_exact_binding(self) -> None:
        refused = self.request(self.fixture["refused_call"])
        self.assertEqual(self.fixture["refused_call"]["expected_result_code"], 1)
        self.assertEqual(self.rpc.send("0x" + refused.raw.hex()), refused.transaction_hash)
        result = self.verified_receipt(refused)
        self.assertEqual(int(result["status"], 16), 0)
        unknown = self.request(self.fixture["unknown_call"])
        control_path = os.environ.get("PAXEER_X_PROGRAM_HTTP_CONTROL")
        if not control_path:
            raise RuntimeError("genuine disposable native disconnect controller required")
        control = json.loads(protected(control_path), object_pairs_hook=unique)
        self.assertEqual(set(control), {"refused_activity_id", "node_pid", "node_disconnect_request_file", "node_disconnected_file"})
        self.assertEqual(control["refused_activity_id"], refused.transaction_hash[2:])
        requested = Path(control["node_disconnect_request_file"])
        acknowledged = Path(control["node_disconnected_file"])
        self.assertTrue(requested.is_absolute() and requested.resolve() == requested)
        self.assertEqual((requested.parent.stat().st_uid, stat.S_IMODE(requested.parent.stat().st_mode)), (os.geteuid(), 0o700))
        fd = os.open(requested, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "wb") as stream:
            stream.write(bytes.fromhex(refused.transaction_hash[2:]))
            stream.flush()
            os.fsync(stream.fileno())
        deadline = time.monotonic() + 30
        while not acknowledged.exists():
            if time.monotonic() >= deadline:
                raise RuntimeError("actual native disconnection was not acknowledged")
            time.sleep(0.05)
        acknowledgment = json.loads(protected(acknowledged), object_pairs_hook=unique)
        self.assertEqual(set(acknowledgment), {"node_pid", "refused_activity_id", "refused_receipt_file", "state"})
        self.assertEqual(acknowledgment["node_pid"], control["node_pid"])
        self.assertEqual(acknowledgment["refused_activity_id"], refused.transaction_hash[2:])
        self.assertEqual(acknowledgment["state"], "native-node-disconnected-after-verified-refusal")
        self.assertEqual(protected(acknowledgment["refused_receipt_file"]).hex(), result["receipt"][2:])
        self.assertFalse(Path(f'/proc/{control["node_pid"]}').exists())
        with self.assertRaises(PlatformSdkError) as caught:
            self.rpc.send("0x" + unknown.raw.hex())
        from layerx_sdk.production import SdkErrorCode
        self.assertEqual(caught.exception.code, SdkErrorCode.UNKNOWN_OUTCOME)
        self.assertEqual(unknown.transaction_hash, "0x" + activity_id(unknown.raw).hex())
        # The producer must restart the same node/store and acknowledge readiness.
        recovered_path = Path(control["node_disconnected_file"] + ".recovered")
        deadline = time.monotonic() + 30
        while not recovered_path.exists():
            if time.monotonic() >= deadline:
                raise RuntimeError("node recovery was not acknowledged")
            time.sleep(0.05)
        recovered = json.loads(protected(recovered_path), object_pairs_hook=unique)
        self.assertEqual(recovered["transaction_hash"], unknown.transaction_hash)
        self.assertEqual(recovered["state"], "same-store-reopened")
        self.assertEqual(self.rpc.send("0x" + unknown.raw.hex()), unknown.transaction_hash)
        receipt = self.verified_receipt(unknown)
        self.assertEqual(self.rpc.send("0x" + unknown.raw.hex()), unknown.transaction_hash)
        self.assertEqual(self.verified_receipt(unknown), receipt)


def load_tests(loader, tests, pattern):
    del loader, tests, pattern
    return unittest.TestSuite(ProgramHttp(name) for name in (
        "test_real_discovery_signature_pin_and_signed_call_refusals",
        "test_real_unified_http_discovery_interface_simulation_call_and_recovery",
        "test_real_native_refused_and_unknown_calls_keep_exact_binding",
    ))


if __name__ == "__main__":
    unittest.main()
