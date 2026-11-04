"""Runtime proofs for the seven audit findings; provider suites remain intact."""
import asyncio
import base64
import copy
import dataclasses
import hashlib
import http.server
import importlib
import importlib.util
import json
import shlex
import struct
from pathlib import Path
import tempfile
import threading
import unittest

from jsonschema import ValidationError

from coordinator import SPEC_PATH, validate_coordinator
from phase2_http import HTTP
from phase2_receivers import ReceiverState, WebSocket
from phase2_runner import OPERATIONS, resolve_routes
from phase2_types import Bindings, ContractAuthority, ROOT, canonical, digest, validate_v2
from phase3_runner import SkewReceiverState
from phase4_runner import validate_runtime
from protocol import CONTRACTS, CONTRACTS_SHA256, subscribe, validate
from workload import MIX, VERSION

HERE = Path(__file__).parent


def write_document(directory, name, value):
    path = directory / name
    raw = canonical(value)
    path.write_bytes(raw)
    return {"path": str(path), "sha256": digest(raw)}


class RepairM965(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.bindings = Bindings(*[str(index) * 64 for index in range(1, 5)])

    def test_python_capability(self):
        # Import the actual modules using the interpreter that runs every proof;
        # missing dependencies or incompatible Python syntax must fail the gate.
        for name, symbol in (
            ("coordinator", "validate_coordinator"),
            ("programme", "load_programme"),
            ("phase2_types", "ContractAuthority"),
            ("phase2_http", "HTTP"),
            ("phase2_receivers", "WebSocket"),
            ("phase2_runner", "run"),
            ("phase3_runner", "run"),
            ("phase4_runner", "validate_runtime"),
            ("protocol", "validate"),
        ):
            with self.subTest(module=name):
                module = importlib.import_module(name)
                self.assertEqual(Path(module.__file__).resolve(), (HERE / (name + ".py")).resolve())
                self.assertTrue(callable(getattr(module, symbol)))

    def contracts(self):
        schema = ROOT / "sdk/launchpad/packages/production-contracts-v2/src/schemas.gen.json"
        api = HERE / "testdata/repair_m965_openapi.json"
        entry = write_document(self.directory, "contracts.json", {
            "bindings": dataclasses.asdict(self.bindings),
            "schemas": {"path": str(schema), "sha256": digest(schema.read_bytes())},
            "openapi": {"path": str(api), "sha256": digest(api.read_bytes())},
        })
        config = {"contract_authority_path": entry["path"], "contract_authority_sha256": entry["sha256"]}
        return config, ContractAuthority(config, self.bindings)

    def test_local_command_doc(self):
        readme = (HERE / "README.md").read_text()
        local = readme.split("## Local workload tests", 1)[1].split("## Accounting", 1)[0]
        self.assertIn("-p 'test_load.py'", local)
        self.assertIn("Only this selection has the loopback-only", local)
        for phase in range(2, 5):
            for suffix in ("CONFIG_FILE", "CONFIG_SHA256", "OUTPUT_DIR"):
                self.assertIn(f"LOAD_PHASE{phase}_{suffix}", readme)
        for name in ("test_phase2_journeys.py", "test_phase3_faults.py", "test_phase4_capacity.py"):
            self.assertIn(name, readme)
            self.assertTrue((HERE / name).is_file())
        self.assertIn("25,000 connections", readme)
        self.assertIn("faults and recovery", readme)
        self.assertIn("native `eventdriver`", readme)

    def test_local_manifest_command(self):
        manifest = json.loads((HERE / "phase-1-scenario-manifest.json").read_text())
        self.assertEqual(manifest["command"], ["python3", "-m", "unittest", "discover", "-s", "tools/launchpad/production/load", "-p", "test_load.py", "-v"])
        self.assertFalse(manifest["capacity_qualified"])
        loader = unittest.TestLoader()
        suite = loader.discover(str(HERE), pattern=manifest["command"][-2])
        def cases(group):
            for case in group:
                if isinstance(case, unittest.TestSuite):
                    yield from cases(case)
                else:
                    yield case.id()
        ids = list(cases(suite))
        self.assertGreater(len(ids), 0)
        self.assertTrue(all(identifier.startswith("test_load.") for identifier in ids), ids)
        self.assertIn("test_load.WorkloadTests.test_protocol_bounds", ids)
        self.assertIn("test_load.NetworkTests.test_real_websocket_masked_frozen_subscribe", ids)

    def test_v1_contract(self):
        self.assertEqual(hashlib.sha256(CONTRACTS.read_bytes()).hexdigest(), CONTRACTS_SHA256)
        topics = ["market.pool.0x" + "1" * 40]
        message = subscribe(topics)
        self.assertEqual(validate("IF_SUBSCRIBE", json.loads(canonical(message))), message)
        for invalid in ([], ["same", "same"], [str(index) for index in range(21)]):
            with self.assertRaises(Exception):
                subscribe(invalid)
        # Drive the existing positive and negative production-schema test directly.
        from test_load import WorkloadTests
        WorkloadTests("test_actual_frozen_positive_market_event").debug()
        with self.assertRaises(ValueError):
            validate("UNREVIEWED", message)

    def test_v2_contract(self):
        config, authority = self.contracts()
        pool = "0x" + "1" * 40
        message = {"op": "subscribe", "subscriptionId": "repair-m965", "topics": ["market.price." + pool], "afterCursor": None}
        self.assertEqual(validate_v2("Subscribe", json.loads(canonical(message)), authority), message)
        for mutation in ({"topics": []}, {"topics": message["topics"] * 2}, {"op": "unreviewed"}):
            with self.assertRaises(ValidationError):
                validate_v2("Subscribe", {**message, **mutation}, authority)
        with self.assertRaises(ValueError):
            validate_v2("Subscribe", message, None)
        for binding in ("candidate", "interface", "topology", "generator"):
            with self.subTest(binding=binding), self.assertRaises(ValueError):
                ContractAuthority(config, dataclasses.replace(self.bindings, **{binding: "a" * 64}))
        with self.assertRaises(ValueError):
            ContractAuthority({**config, "contract_authority_sha256": "0" * 64}, self.bindings)
        with self.assertRaises(ValueError):
            ContractAuthority({}, self.bindings)
        descriptor = json.loads(Path(config["contract_authority_path"]).read_bytes())
        for document in ("schemas", "openapi"):
            # The admitted descriptor cannot authenticate changed transport bytes.
            entry = write_document(self.directory, f"{document}-tampered.json", {})
            changed = copy.deepcopy(descriptor)
            changed[document]["path"] = entry["path"]
            rebound = write_document(self.directory, "tampered-authority.json", changed)
            with self.subTest(document=document), self.assertRaisesRegex(ValueError, "document digest differs"):
                ContractAuthority({"contract_authority_path": rebound["path"], "contract_authority_sha256": rebound["sha256"]}, self.bindings)
        # Even a correctly bound authority cannot fetch an unreviewed remote schema.
        definitions = copy.deepcopy(authority.definitions)
        definitions["$defs"]["Subscribe"] = {"$ref": "https://unreviewed.example/schema"}
        remote = write_document(self.directory, "remote-schema.json", definitions)
        rebound = write_document(self.directory, "remote-authority.json", {**descriptor, "schemas": remote})
        with self.assertRaisesRegex(ValueError, "references must remain inside"):
            ContractAuthority({"contract_authority_path": rebound["path"], "contract_authority_sha256": rebound["sha256"]}, self.bindings)
        # Actual loopback transport: the HTTP consumer validates bytes from its peer.
        class Responses(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                payload = canonical(message if self.path == "/valid" else {**message, "topics": []})
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(payload)))
                self.end_headers()
                self.wfile.write(payload)
            def log_message(self, *args):
                pass
        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Responses)
        thread = threading.Thread(target=server.serve_forever)
        thread.start()
        try:
            client = HTTP(f"http://127.0.0.1:{server.server_port}", "https://application.example", owned_local=True, authority=authority)
            self.assertEqual(client.request("GET", "/valid", schema="Subscribe"), message)
            with self.assertRaises(ValidationError):
                client.request("GET", "/invalid", schema="Subscribe")
        finally:
            server.shutdown()
            server.server_close()
            thread.join()
        snapshot = {"topics": [{"topic": message["topics"][0], "sequence": "0", "revision": "1", "payload": None}], "resumeCursor": "cursor"}
        # The actual frozen schema decides validity; malformed snapshots cannot enter
        # either receiver even when a different authority is present in another run.
        for receiver in (ReceiverState, SkewReceiverState):
            with self.assertRaises(ValidationError):
                receiver(message["topics"], snapshot, authority=authority)
        recorded = json.loads((ROOT / "sdk/launchpad/protocol/contracts/v1/examples.json").read_bytes())
        snapshot = recorded["endpoints"]["IF_TOPIC_SNAPSHOT_READ"]["successes"][0]["body"]["data"]["snapshot"]
        for receiver in (ReceiverState, SkewReceiverState):
            state = receiver([snapshot["topics"][0]["topic"]], snapshot, authority=authority)
            self.assertIs(state.authority, authority)
            self.assertEqual(state.sequence, {snapshot["topics"][0]["topic"]: 1})
            self.assertEqual(state.cursor, snapshot["resumeCursor"])
            malformed = copy.deepcopy(snapshot)
            malformed["topics"][0]["sequence"] = 1
            with self.assertRaises(ValidationError):
                receiver([snapshot["topics"][0]["topic"]], malformed, authority=authority)

        async def websocket_roundtrip():
            received = asyncio.get_running_loop().create_future()

            async def peer(reader, writer):
                try:
                    headers = await reader.readuntil(b"\r\n\r\n")
                    key = next(row.split(b":", 1)[1].strip() for row in headers.split(b"\r\n") if row.lower().startswith(b"sec-websocket-key:"))
                    accept = base64.b64encode(hashlib.sha1(key + b"258EAFA5-E914-47DA-95CA-C5AB0DC85B11").digest())
                    writer.write(b"HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Protocol: sidiora.v2\r\nSec-WebSocket-Accept: " + accept + b"\r\n\r\n")
                    await writer.drain()
                    opcode, length = await reader.readexactly(2)
                    self.assertEqual(opcode, 0x81)
                    self.assertTrue(length & 0x80)
                    length &= 0x7f
                    if length == 126:
                        length = struct.unpack("!H", await reader.readexactly(2))[0]
                    mask = await reader.readexactly(4)
                    payload = await reader.readexactly(length)
                    payload = bytes(value ^ mask[index % 4] for index, value in enumerate(payload))
                    decoded = validate_v2("Subscribe", json.loads(payload), authority)
                    self.assertEqual(decoded["topics"], message["topics"])
                    self.assertEqual(decoded["afterCursor"], snapshot["resumeCursor"])
                    writer.write(b"\x81" + (bytes([length]) if length < 126 else b"\x7e" + struct.pack("!H", length)) + payload)
                    await writer.drain()
                    received.set_result(decoded)
                    await reader.read()
                except Exception as error:
                    if not received.done():
                        received.set_exception(error)
                    else:
                        raise
                finally:
                    writer.close()
                    await writer.wait_closed()

            server = await asyncio.start_server(peer, "127.0.0.1", 0)
            port = server.sockets[0].getsockname()[1]
            socket = WebSocket(f"http://127.0.0.1:{port}", "https://application.example", authority=authority, owned_local=True)
            try:
                await socket.connect()
                await socket.subscribe(message["topics"], snapshot["resumeCursor"])
                echoed = await socket.receive()
                self.assertEqual(validate_v2("Subscribe", echoed, authority), await received)
                with self.assertRaises(ValidationError):
                    await socket.subscribe([], None)
            finally:
                await socket.close()
                server.close()
                await server.wait_closed()

        asyncio.run(asyncio.wait_for(websocket_roundtrip(), 10))

    def test_route_catalog(self):
        config, authority = self.contracts()
        api = authority.openapi
        producer = api["x-producer"]
        raw = (ROOT / producer["path"]).read_bytes()
        self.assertEqual(digest(raw), producer["sha256"])
        text = raw.decode()
        start = text.index("export const operationDefinitions = ") + len("export const operationDefinitions = ")
        end = text.index("\n} as const;", start) + 2
        emitted = json.loads(text[start:end])
        routes = {category: {"template": emitted[identifier]["path"], "method": emitted[identifier]["method"]} for category, identifier in OPERATIONS.items()}
        resolved = resolve_routes(routes, authority)
        self.assertEqual(set(resolved), set(MIX))
        for category, identifier in OPERATIONS.items():
            self.assertEqual(resolved[category]["response_schema"], emitted[identifier]["response"])
            self.assertEqual(resolved[category]["request_schema"], emitted[identifier]["body"])
            self.assertEqual(resolved[category]["authenticated"], emitted[identifier]["authenticated"])
        for key, value in (("method", "DELETE"), ("template", "/wrong"), ("expected_statuses", [200])):
            altered = copy.deepcopy(routes)
            altered["chart"][key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                resolve_routes(altered, authority)
        with self.assertRaises(ValueError):
            resolve_routes({}, authority)
        with self.assertRaises(ValueError):
            resolve_routes(routes, None)
        # Duplicate and missing operations are transported as newly bound documents,
        # then refused by the real consumer even when their file digests match.
        descriptor = json.loads(Path(config["contract_authority_path"]).read_text())
        for change in ("duplicate", "missing"):
            changed = copy.deepcopy(api)
            path = emitted["IF_CANDLES"]["path"]
            if change == "duplicate":
                changed["paths"]["/duplicate"] = copy.deepcopy(changed["paths"][path])
            else:
                del changed["paths"][path]
            api_entry = write_document(self.directory, "changed-api.json", changed)
            altered = write_document(self.directory, "changed-authority.json", {**descriptor, "openapi": api_entry})
            rebound = ContractAuthority({"contract_authority_path": altered["path"], "contract_authority_sha256": altered["sha256"]}, self.bindings)
            with self.subTest(change=change), self.assertRaises(ValueError):
                resolve_routes(routes, rebound)

    def test_final_admission(self):
        programme = self.directory / SPEC_PATH
        programme.parent.mkdir(parents=True)
        programme.write_bytes((HERE / "testdata/repair_m965_programme.kvx").read_bytes())
        hosts = [f"generator-{index}" for index in range(4)]
        runtime = {
            "bindings": dataclasses.asdict(self.bindings), "workload_version": VERSION,
            "generator_hosts": hosts,
            "generators": [{"host_id": host, "application_cluster_member": False, "clock_offset_ns": 0, "clock_uncertainty_ns": 1, "host_attestation_sha256": "a" * 64} for host in hosts],
            "lane_runtime_bindings": {f"L{index:02d}": {"candidate": self.bindings.candidate, "interface": self.bindings.interface, "topology": self.bindings.topology, "tool_version_sha256": "b" * 64} for index in range(1, 51) if index != 48},
        }
        # These real task states exercise the migration interface only. They are
        # not a root approval to use a repair task as a capacity execution task.
        coordinator = {
            "bindings": dataclasses.asdict(self.bindings),
            "spec": {"path": SPEC_PATH, "sha256": digest(programme.read_bytes())},
            "freeze_task": "22.31.41", "final_task": "22.33.965",
        }
        entry = write_document(self.directory, "coordinator.json", coordinator)
        runtime.update(coordinator_authority_sha256=entry["sha256"], final_task=coordinator["final_task"], coordinator_source_sha256=self.bindings.candidate)
        native = write_document(self.directory, "runtime.json", runtime)
        config = {"generator_hosts": hosts, "native_runtime_authority_path": native["path"], "native_runtime_authority_sha256": native["sha256"], "coordinator_authority_path": entry["path"], "coordinator_authority_sha256": entry["sha256"]}
        self.assertEqual(validate_runtime(config, self.bindings, final=True, root=self.directory), runtime)
        # Stop and reopen the authority documents: the exact source bindings survive.
        self.assertEqual(validate_coordinator(json.loads(canonical(config)), self.bindings, json.loads(canonical(runtime)), root=self.directory), coordinator)
        for change in ("lane", "lane_candidate", "lane_interface", "lane_topology", "candidate", "final_task", "freeze", "spec_digest", "freeze_status", "final_status", "dependency", "duplicate_section", "legacy_spec"):
            bad_runtime, bad_coordinator = copy.deepcopy(runtime), copy.deepcopy(coordinator)
            if change == "lane":
                del bad_runtime["lane_runtime_bindings"]["L01"]
            elif change.startswith("lane_"):
                bad_runtime["lane_runtime_bindings"]["L01"][change.removeprefix("lane_")] = "f" * 64
            elif change == "candidate":
                bad_runtime["coordinator_source_sha256"] = "f" * 64
            elif change == "final_task":
                bad_runtime["final_task"] = "9.1"
            elif change == "freeze":
                bad_coordinator["freeze_task"] = bad_coordinator["final_task"]
            elif change in ("freeze_status", "final_status", "dependency", "duplicate_section"):
                original = (HERE / "testdata/repair_m965_programme.kvx").read_text()
                if change == "freeze_status":
                    altered = original.replace('status = "done"', 'status = "in_progress"')
                elif change == "final_status":
                    altered = original.replace('status = "in_progress"', 'status = "pending"')
                elif change == "dependency":
                    altered = original.replace('requires = ["22.31.41"]', 'requires = []')
                else:
                    altered = original + '\n[task.22.31.41]\nstatus = "done"\n'
                programme.write_text(altered)
                bad_coordinator["spec"]["sha256"] = digest(programme.read_bytes())
            elif change == "legacy_spec":
                bad_coordinator["spec"]["path"] = "launchpad/spec/production-system/spec.kvx"
            else:
                bad_coordinator["spec"]["sha256"] = "0" * 64
            authority_entry = write_document(self.directory, "bad-coordinator.json", bad_coordinator)
            bad_runtime["coordinator_authority_sha256"] = authority_entry["sha256"]
            runtime_entry = write_document(self.directory, "bad-runtime.json", bad_runtime)
            bad_config = {**config, "native_runtime_authority_path": runtime_entry["path"], "native_runtime_authority_sha256": runtime_entry["sha256"], "coordinator_authority_path": authority_entry["path"], "coordinator_authority_sha256": authority_entry["sha256"]}
            with self.subTest(change=change), self.assertRaises(ValueError):
                validate_runtime(bad_config, self.bindings, final=True, root=self.directory)
            programme.write_bytes((HERE / "testdata/repair_m965_programme.kvx").read_bytes())

    def test_phase4_qualification(self):
        location = HERE.parent / "qualify.py"
        spec = importlib.util.spec_from_file_location("repair_m965_qualifier", location)
        qualifier = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(qualifier)
        manifest = json.loads((HERE / "phase-4-scenario-manifest.json").read_text())
        self.assertEqual(manifest["task"], qualifier.LOAD_REPAIR_TASK)
        argv = shlex.split(manifest["verify_cmd"])
        self.assertEqual(argv[:2], ["python3", "tools/launchpad/production/qualify.py"])
        arguments = qualifier.argument_parser().parse_args(argv[2:])
        self.assertEqual(arguments.task, manifest["task"])
        self.assertEqual(arguments.spec.as_posix(), SPEC_PATH)
        self.assertTrue(arguments.require_current_source)
        self.assertEqual(manifest["verify_cmd"], f"python3 tools/launchpad/production/qualify.py --task {qualifier.LOAD_REPAIR_TASK} --spec {qualifier.LOAD_PROGRAMME_SPEC} --require-current-source")
        task, raw = qualifier.load_repair_admission(ROOT, ROOT / qualifier.LOAD_PROGRAMME_SPEC, qualifier.LOAD_REPAIR_TASK)
        self.assertIn("^TestRepairM_965_", task["verify_cmd"])
        self.assertEqual(raw, (ROOT / SPEC_PATH).read_bytes())
        with self.assertRaises(qualifier.QualificationError):
            qualifier.load_repair_admission(ROOT, ROOT / "launchpad/spec/production-system/spec.kvx", qualifier.LOAD_REPAIR_TASK)
        with self.assertRaises(qualifier.QualificationError):
            qualifier.load_repair_admission(ROOT, ROOT / SPEC_PATH, "4.48")
        from workload import final_scenarios
        self.assertEqual(manifest["final_shared_scenarios"], final_scenarios())
        self.assertFalse(manifest["capacity_qualified"])
        self.assertFalse(manifest["release_qualified"])


if __name__ == "__main__":
    unittest.main()
