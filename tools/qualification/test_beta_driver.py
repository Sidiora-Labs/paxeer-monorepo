from __future__ import annotations

import hashlib
import json
import os
import socket
import shutil
import subprocess
import sys
import tempfile
import unittest
from copy import deepcopy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread

import pytest

from tools.qualification.release_runner import EVIDENCE_SPECS, SCHEMA


ROOT = Path(__file__).resolve().parents[2]
DRIVER = ROOT / "tools" / "qualification" / "beta_driver.py"
PLATFORM_TOOLS_MANIFEST = ROOT / "tools" / "platform" / "Cargo.toml"
NETWORK_ID = "125"
SOURCE_IDENTITY = "beta-driver-test-source-identity"
DRIVER_TIMEOUT_SECONDS = 3600.0
READY_TIMEOUT_SECONDS = 60.0


def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as candidate:
        candidate.bind(("127.0.0.1", 0))
        return candidate.getsockname()[1]


def cli_json(argv: list[str], environment: dict[str, str], stdin_text: str | None = None) -> dict:
    completed = subprocess.run(
        argv,
        cwd=ROOT,
        env=environment,
        input=stdin_text,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if completed.returncode != 0:
        raise RuntimeError(
            f"{' '.join(argv)} exited {completed.returncode}: {completed.stdout}{completed.stderr}"
        )
    for line in reversed(completed.stdout.splitlines()):
        candidate = line.strip()
        if candidate.startswith("{"):
            document = json.loads(candidate)
            if document.get("ok") is not True:
                raise RuntimeError(f"{' '.join(argv)} did not succeed: {candidate}")
            return document["data"]
    raise RuntimeError(f"{' '.join(argv)} printed no JSON object: {completed.stdout}")


class LocalPaxdStack:
    """Replay merged-node recordings; never implement execution or mint authority.

    The native transfer corpus is produced by the node RPC vector generator.
    The program recording includes the node's actual stop/reopen observations.
    This peer serves those bytes at the node's root JSON-RPC endpoint.
    """

    def __init__(self, work: Path) -> None:
        self.work = work
        corpus_path = ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        corpus = json.loads(corpus_path.read_text())
        if corpus["provenance"]["generator"] != "rpc/sequencer_receipt_vectors_test.go":
            raise RuntimeError("native beta evidence must be node-produced")
        self.native = deepcopy(
            next(case for case in corpus["cases"] if case["id"] == "V1-pax-transfer")
        )
        self.recovery_path = ROOT / "tests/daemon/fixtures/programs-commit-reopen.json"
        self.recovery = json.loads(self.recovery_path.read_text())
        if (
            self.recovery["schema"] != "paxd/programs-commit-reopen/v1"
            or self.recovery["provenance"]["generator"] != "tests/daemon/recorder/main.go"
            or self.recovery["node_stopped_between_phases"] is not True
            or self.recovery["anchor_store_absent_after_reopen"] is not True
            or "layerxanchor" in self.recovery["stores_after_reopen"]
        ):
            raise RuntimeError("merged-node stop/reopen evidence required")
        self.source = self.native["receiptJson"]["from"]
        self.destination = self.native["receiptJson"]["to"]
        self.server: ThreadingHTTPServer | None = None
        self.thread: Thread | None = None
        self.url = ""
        self.exchanges: list[dict] = []
        self.requests: list[dict] = []
        self.errors: list[str] = []

    def start(self, phase: str = "native") -> None:
        if self.server is not None:
            raise RuntimeError("stop the recorded node before reopening it")
        if phase == "native":
            self.exchanges = [
                {
                    "method": "eth_sendRawTransaction",
                    "params": [self.native["rawTransaction"]],
                    "result": self.native["receiptJson"]["txHash"],
                },
                {
                    "method": "pax_sequencerReceipt",
                    "params": [self.native["receiptJson"]["txHash"]],
                    "result": self.native["receiptJson"],
                },
            ]
        else:
            # Unknown phases fail closed; there is no invented recovery answer.
            self.exchanges = deepcopy(self.recovery["phases"][phase]["rpc"])
        self.requests = []
        self.errors = []
        peer = self

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self) -> None:
                try:
                    if self.path != "/" or self.headers["Content-Type"] != "application/json":
                        raise ValueError("root JSON-RPC and JSON content type required")
                    length = int(self.headers["Content-Length"])
                    if not 0 < length <= 1_048_576:
                        raise ValueError("invalid request length")
                    request = json.loads(self.rfile.read(length))
                    if (
                        set(request) != {"jsonrpc", "id", "method", "params"}
                        or request["jsonrpc"] != "2.0"
                        or type(request["id"]) is not int or request["id"] <= 0
                    ):
                        raise ValueError("invalid JSON-RPC request")
                    index = len(peer.requests)
                    peer.requests.append(request)
                    recorded = peer.exchanges[index]
                    if (
                        request["method"] != recorded["method"]
                        or request["params"] != recorded["params"]
                    ):
                        raise ValueError("request differs from node recording")
                    payload = {"jsonrpc": "2.0", "id": request["id"], "result": recorded["result"]}
                    body = json.dumps(payload).encode()
                    self.send_response(200)
                    self.send_header("Content-Type", "application/json")
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                except Exception as error:
                    peer.errors.append(str(error))
                    self.send_error(400, "request differs from node recording")

            def log_message(self, *args: object) -> None:
                pass

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self.server.server_port}"
        self.thread = Thread(
            target=self.server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True
        )
        self.thread.start()

    def assert_consumed(self) -> None:
        if self.errors or len(self.requests) != len(self.exchanges):
            raise AssertionError(
                f"node recording was not consumed exactly: {self.errors}, {self.requests}"
            )

    def stop(self) -> None:
        if self.server is None:
            return
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)
        if self.thread.is_alive():
            raise RuntimeError("recorded node listener did not stop")
        self.server = None
        self.thread = None

    def driver_environment(self) -> dict[str, str]:
        environment = dict(os.environ)
        for name in tuple(environment):
            if name.startswith("LAYERX_"):
                environment.pop(name)
        environment.update(
            {
                "LAYERX_CLI_ENVIRONMENT": "beta",
                "LAYERX_NETWORK_ID": NETWORK_ID,
                "LAYERX_SOURCE": self.source,
                "LAYERX_DESTINATION": self.destination,
                "LAYERX_CURRENCY": "PAX",
                "LAYERX_AMOUNT": "1000000000000000000",
            }
        )
        return environment


class BetaDriverTests(unittest.TestCase):
    stack: LocalPaxdStack
    work: Path
    agentd_url: str
    paxeer_url: str

    @classmethod
    def setUpClass(cls) -> None:
        build_root = ROOT / "build" / "beta-driver-test"
        build_root.mkdir(parents=True, exist_ok=True)
        cls.work = Path(tempfile.mkdtemp(prefix="run-", dir=build_root))
        cls.stack = LocalPaxdStack(cls.work)
        cls.stack.start()
        cls.agentd_url = f"http://127.0.0.1:{free_port()}"
        cls.paxeer_url = f"http://127.0.0.1:{free_port()}"

    @classmethod
    def tearDownClass(cls) -> None:
        cls.stack.stop()

    def run_driver(self, gate: str, output: Path) -> tuple[subprocess.CompletedProcess[str], dict]:
        argv = [
            sys.executable,
            str(DRIVER),
            "run",
            "--gate",
            gate,
            "--output",
            str(output),
            "--source-identity",
            SOURCE_IDENTITY,
            "--node-url",
            self.stack.url,
            "--agentd-url",
            self.agentd_url,
            "--human-service-url",
            self.stack.url,
            "--paxeer-testnet-url",
            self.paxeer_url,
        ]
        completed = subprocess.run(
            argv,
            cwd=ROOT,
            env=self.stack.driver_environment(),
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            timeout=DRIVER_TIMEOUT_SECONDS,
        )
        (output / "driver-output.log").write_text(completed.stdout, encoding="utf-8")
        manifest = output / "evidence.json"
        self.assertTrue(manifest.is_file(), completed.stdout[-4000:])
        evidence = json.loads(manifest.read_text(encoding="utf-8"))
        return completed, evidence

    def assert_manifest_shape(self, gate: str, evidence: dict, output: Path) -> dict[str, dict]:
        self.assertEqual(evidence["schema"], SCHEMA)
        self.assertEqual(evidence["gate"], gate)
        self.assertIs(evidence["real_stack"], False)
        self.assertEqual(evidence["scope"], "local-suites-or-partial-evidence")
        self.assertEqual(evidence["source_identity"], SOURCE_IDENTITY)
        self.assertEqual(evidence["driver_sha256"], hashlib.sha256(DRIVER.read_bytes()).hexdigest())
        self.assertEqual(
            evidence["components"],
            {
                "node": self.stack.url,
                "agentd": self.agentd_url,
                "human_service": self.stack.url,
                "paxeer_testnet": self.paxeer_url,
            },
        )
        cases = {record["id"]: record for record in evidence["cases"]}
        self.assertEqual(len(cases), len(evidence["cases"]))
        self.assertEqual(set(cases), set(EVIDENCE_SPECS[gate].cases))
        for case_id, record in cases.items():
            self.assertIn(record["status"], ("passed", "failed"), case_id)
            self.assertTrue(record["executions"] or record["status"] == "failed", case_id)
            if record["status"] == "passed":
                self.assertIsNone(record["reason"], case_id)
                kinds = {artifact["kind"] for artifact in record["artifacts"]}
                self.assertEqual(kinds, set(EVIDENCE_SPECS[gate].artifact_kinds), case_id)
                for artifact in record["artifacts"]:
                    path = output / artifact["path"]
                    self.assertTrue(path.is_file() and path.stat().st_size > 0, artifact)
                    self.assertEqual(
                        hashlib.sha256(path.read_bytes()).hexdigest(), artifact["sha256"], artifact
                    )
            else:
                self.assertIsInstance(record["reason"], str, case_id)
                self.assertTrue(record["reason"], case_id)
                self.assertEqual(record["artifacts"], [], case_id)
        return cases

    def test_adoption_inventory_executes_against_the_local_paxd_stack(self) -> None:
        gate = "platform-qualify-adoption"
        output = self.work / gate
        completed, evidence = self.run_driver(gate, output)
        cases = self.assert_manifest_shape(gate, evidence, output)
        rust = cases["rust/first-payment"]
        self.assertEqual(rust["status"], "failed")
        self.assertIn("LAYERX_QUALIFICATION_NETWORK_TRUST", rust["reason"])
        self.assertEqual(rust["artifacts"], [])
        self.assertIs(rust["real_stack"], False)
        self.assertEqual(rust["paxd_evidence"], [])
        deploy = cases["programs/five-minute-deploy-and-paid-call"]
        self.assertEqual(deploy["status"], "failed")
        if shutil.which("layerx") is None:
            self.assertEqual(deploy["reason"], "layerx CLI not found; set LAYERX_BIN or put layerx on PATH")
            self.skipTest("missing tool layerx; exact command: cargo build --manifest-path tools/platform/Cargo.toml --bin layerx")
        self.assertIn("registry-list", deploy["reason"])
        failed = sorted(case_id for case_id, record in cases.items() if record["status"] == "failed")
        self.assertTrue(failed)
        self.assertEqual(completed.returncode, 1, completed.stdout[-4000:])
        for case_id in failed:
            self.assertIn(f"case {case_id}: failed: ", completed.stdout)
        self.assertIn("case rust/first-payment: failed:", completed.stdout)

    @pytest.mark.native
    def test_multichain_inventory_reports_absent_infrastructure_as_failed(self) -> None:
        gate = "multichain-qualify"
        output = self.work / gate
        completed, evidence = self.run_driver(gate, output)
        cases = self.assert_manifest_shape(gate, evidence, output)
        one_ledger = cases["surfaces/one-ledger"]
        executions = {execution["label"]: execution for execution in one_ledger["executions"]}
        self.assertEqual(executions["schema-check"]["exit_code"], 0, one_ledger["reason"])
        self.assertEqual(
            executions["native-ledger-boundary-rejections"]["exit_code"], 0, one_ledger["reason"]
        )
        self.assertIn("schema-conformance", executions)
        failed_steps = [label for label, execution in executions.items() if execution["exit_code"]]
        if failed_steps:
            self.assertEqual(one_ledger["status"], "failed")
            self.assertIn(failed_steps[0], one_ledger["reason"])
        else:
            self.assertEqual(one_ledger["status"], "passed", one_ledger["reason"])
        expected_inputs = {
            "mirrors/offline-verification": "LAYERX_MIRROR_VERIFY_CONFIG",
            "mirrors/tamper-rejection": "LAYERX_MIRROR_VERIFY_CONFIG",
            "surfaces/paxeer-exclusivity": "LAYERX_PAXEER_CHAIN_ID",
            "reference-ramp/end-to-end": "LAYERX_RAMP_URL",
            "reference-ramp/labelling": "LAYERX_RAMP_URL",
        }
        for case_id, variable in expected_inputs.items():
            record = cases[case_id]
            self.assertEqual(record["status"], "failed", case_id)
            self.assertIn("owner input missing", record["reason"], case_id)
            self.assertIn(variable, record["reason"], case_id)
            self.assertIn(f"case {case_id}: failed: ", completed.stdout)
        self.assertEqual(completed.returncode, 1, completed.stdout[-4000:])

    def test_driver_refuses_to_overwrite_existing_case_evidence(self) -> None:
        output = self.work / "occupied"
        (output / "cases").mkdir(parents=True)
        completed = subprocess.run(
            [
                sys.executable,
                str(DRIVER),
                "run",
                "--gate",
                "multichain-qualify",
                "--output",
                str(output),
                "--source-identity",
                SOURCE_IDENTITY,
                "--node-url",
                self.stack.url,
                "--agentd-url",
                self.agentd_url,
                "--human-service-url",
                self.stack.url,
                "--paxeer-testnet-url",
                self.paxeer_url,
            ],
            cwd=ROOT,
            env=self.stack.driver_environment(),
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        self.assertEqual(completed.returncode, 2)
        self.assertIn("already holds case evidence", completed.stderr)
        self.assertFalse((output / "evidence.json").exists())


if __name__ == "__main__":
    unittest.main()
