#!/usr/bin/env python3
from __future__ import annotations

import argparse
import base64
import hashlib
import importlib.util
import json
import os
import re
import secrets
import shutil
import subprocess
import sys
import time
import tomllib
from collections.abc import Callable, Mapping, Sequence
from datetime import datetime, timezone
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[2]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from tools.qualification.release_runner import (
    EVIDENCE_SPECS,
    SCHEMA,
    QualificationFailure,
    atomic_json,
    file_digest,
    validated_url,
)

DRIVER_GATES = (
    "platform-qualify-adoption",
    "programs-qualify",
    "interop-qualify",
    "multichain-qualify",
)
STACK_FLAGS = (
    ("node", "--node-url"),
    ("agentd", "--agentd-url"),
    ("human_service", "--human-service-url"),
    ("paxeer_testnet", "--paxeer-testnet-url"),
)
MAKE = ("make", "--no-print-directory")
FIVE_MINUTES_SECONDS = 300.0
HTTP_TIMEOUT_SECONDS = 20.0
TAIL_LINES = 40
SAMPLES = ROOT / "services" / "platform" / "docs" / "samples"
SAMPLE_INPUTS = (
    "LAYERX_API_TOKEN",
    "LAYERX_SOURCE",
    "LAYERX_DESTINATION",
    "LAYERX_AMOUNT",
    "LAYERX_CURRENCY",
)
CLI_INPUTS = ("LAYERX_NETWORK_ID", "LAYERX_SEQUENCER_TRUST_ANCHOR")
CLI_ENVIRONMENT_INPUT = "LAYERX_CLI_ENVIRONMENT"
CLI_BINARY_INPUT = "LAYERX_BIN"
SETTLED_STATES = frozenset(("done", "done-finalised"))
MIRROR_VERIFY_INPUTS = (
    "LAYERX_MIRROR_VERIFY_CONFIG",
    "LAYERX_MIRROR_CANONICAL_REQUEST",
    "LAYERX_MIRROR_FAILOVER_REQUEST",
    "LAYERX_MIRROR_DIVERGENCE_REQUEST",
    "LAYERX_MIRROR_TAMPER_REQUEST",
    "LAYERX_MIRROR_TS_CONFORMANCE",
    "LAYERX_MIRROR_PYTHON_CONFORMANCE",
    "LAYERX_MIRROR_GO_CONFORMANCE",
    "LAYERX_MIRROR_JVM_CONFORMANCE",
    "LAYERX_MIRROR_SWIFT_CONFORMANCE",
    "LAYERX_MIRROR_DOTNET_CONFORMANCE",
)
MIRROR_TAMPER_INPUTS = (
    "LAYERX_MIRROR_VERIFY_CONFIG",
    "LAYERX_MIRROR_CANONICAL_REQUEST",
    "LAYERX_MIRROR_TAMPER_REQUEST",
)
MIRROR_LIVE_FORBIDDEN = ("LAYERX_NODE_URL", "LAYERX_GATEWAY_URL", "LAYERX_EXPLORER_API_ORIGIN")
MIGRATION_INPUTS = (
    "LAYERX_ETHEREUM_CONFIG",
    "LAYERX_ETHEREUM_ASSET_EVIDENCE",
    "LAYERX_ETHEREUM_HISTORY_EVIDENCE",
    "LAYERX_ETHEREUM_OWNERSHIP_EVIDENCE",
    "LAYERX_SOLANA_CONFIG",
    "LAYERX_SOLANA_ASSET_EVIDENCE",
    "LAYERX_SOLANA_HISTORY_EVIDENCE",
    "LAYERX_SOLANA_OWNERSHIP_EVIDENCE",
    "LAYERX_MIGRATION_SECRET_DIR",
)
RAMP_INPUTS = (
    "LAYERX_RAMP_URL",
    "LAYERX_RAMP_CA_PEM",
    "LAYERX_RAMP_CUSTOMER_TOKEN",
    "LAYERX_RAMP_OPERATOR_URL",
    "LAYERX_RAMP_OPERATOR_TOKEN",
    "LAYERX_RAMP_ON_QUOTE_ID",
    "LAYERX_RAMP_OFF_QUOTE_ID",
    "LAYERX_RAMP_OFF_GRANT_JSON",
    "LAYERX_RAMP_ON_ACCOUNT_SEQUENCE",
    "LAYERX_RAMP_OFF_RECEIVER_SEQUENCE",
)
PAXEER_CHAIN_INPUT = "LAYERX_PAXEER_CHAIN_ID"
CARGO_TEST_LINE = re.compile(r"^test (\S+) \.\.\. ok$", re.MULTILINE)
PIN_CONSTANT = re.compile(r'const ([A-Z][A-Z0-9_]*): &str = "([^"]*)";')
FAULT_BOUNDARY = re.compile(r"^\s*(LXP_FAULT_[A-Z_]+) = (\d+)", re.MULTILINE)
GAUNTLET_INVENTORY = ROOT / "programs" / "tests" / "gauntlet" / "attack-inventory.tsv"
FAULT_HEADER = ROOT / "include" / "layerx" / "lxp_fault.h"
PROGRAMS_DIRECTORY = ROOT / "programs"
AGENT_MANIFEST = ROOT / "services" / "agent" / "Cargo.toml"
NETWORK_TRUST_INPUT = "LAYERX_QUALIFICATION_NETWORK_TRUST"
HUMAN_MANIFEST = ROOT / "services" / "human" / "Cargo.toml"
INTEROP_MANIFEST = ROOT / "services" / "interop" / "Cargo.toml"
PLATFORM_MANIFEST = ROOT / "services" / "platform" / "Cargo.toml"
SCHEMA_CHECK_MANIFEST = ROOT / "services" / "human" / "tools" / "schema-check" / "Cargo.toml"


class CaseFailure(Exception):
    pass


class Stack:
    def __init__(self, node: str, agentd: str, human_service: str, paxeer_testnet: str) -> None:
        self.node = node
        self.agentd = agentd
        self.human_service = human_service
        self.paxeer_testnet = paxeer_testnet

    def components(self) -> dict[str, str]:
        return {
            "node": self.node,
            "agentd": self.agentd,
            "human_service": self.human_service,
            "paxeer_testnet": self.paxeer_testnet,
        }


class Execution:
    def __init__(
        self,
        label: str,
        argv: Sequence[str],
        cwd: Path,
        exit_code: int,
        log: Path,
        output: str,
        elapsed_seconds: float,
    ) -> None:
        self.label = label
        self.argv = tuple(argv)
        self.cwd = cwd
        self.exit_code = exit_code
        self.log = log
        self.output = output
        self.elapsed_seconds = elapsed_seconds

    def tail(self) -> str:
        lines = self.output.rstrip().splitlines()
        return "\n".join(lines[-TAIL_LINES:])

    def cargo_tests(self) -> list[str]:
        return CARGO_TEST_LINE.findall(self.output)

    def native_tests(self) -> list[str]:
        return re.findall(r"^--- PASS: (\w+) \(", self.output, re.MULTILINE)

    def last_json(self) -> dict[str, object]:
        for line in reversed(self.output.splitlines()):
            candidate = line.strip()
            if not candidate.startswith("{"):
                continue
            try:
                parsed = json.loads(candidate)
            except json.JSONDecodeError:
                continue
            if isinstance(parsed, dict):
                return parsed
        raise CaseFailure(f"{self.label} printed no JSON object")


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z")


def json_bytes(payload: object) -> bytes:
    return (json.dumps(payload, indent=2, sort_keys=True) + "\n").encode("utf-8")


def slug(value: str) -> str:
    return re.sub(r"[^A-Za-z0-9_.-]+", "-", value).strip("-")


def bytes_digest(content: bytes) -> str:
    return hashlib.sha256(content).hexdigest()


class CaseContext:
    def __init__(
        self,
        gate: str,
        case_id: str,
        output_root: Path,
        stack: Stack,
        source_identity: str,
        environment: Mapping[str, str],
    ) -> None:
        self.gate = gate
        self.case_id = case_id
        self.output_root = output_root
        self.stack = stack
        self.source_identity = source_identity
        self.environment = dict(environment)
        self.directory = output_root / "cases" / slug(case_id)
        self.directory.mkdir(parents=True, exist_ok=False)
        self.artifacts: list[dict[str, str]] = []
        self.executions: list[Execution] = []
        self.paxd_evidence: list[dict[str, object]] = []
        self.started_at = utc_now()
        self.started = time.monotonic()

    def elapsed(self) -> float:
        return time.monotonic() - self.started

    def relative(self, path: Path) -> str:
        return path.resolve().relative_to(self.output_root.resolve()).as_posix()

    def require_env(self, *names: str) -> dict[str, str]:
        values: dict[str, str] = {}
        missing: list[str] = []
        for name in names:
            value = self.environment.get(name, "")
            if value:
                values[name] = value
            else:
                missing.append(name)
        if missing:
            raise CaseFailure(
                "owner input missing; set " + ", ".join(missing) + " in the driver environment"
            )
        return values

    def require_file(self, path: Path, description: str) -> Path:
        if not path.is_file():
            raise CaseFailure(f"{description} not found at {path}")
        return path

    def require_executable(self, path: Path, description: str) -> Path:
        if not path.is_file() or not os.access(path, os.X_OK):
            raise CaseFailure(f"{description} is not an executable file at {path}")
        return path

    def run(
        self,
        argv: Sequence[str],
        *,
        label: str,
        cwd: Path = ROOT,
        env: Mapping[str, str] | None = None,
        stdin_text: str | None = None,
        timeout: float | None = None,
        allow_failure: bool = False,
    ) -> Execution:
        log_directory = self.directory / "logs"
        log_directory.mkdir(parents=True, exist_ok=True)
        log = log_directory / f"{len(self.executions) + 1:02d}-{slug(label)}.log"
        started = time.monotonic()
        print(f"case {self.case_id}: $ {' '.join(argv)}", flush=True)
        try:
            completed = subprocess.run(
                list(argv),
                cwd=cwd,
                env=dict(env) if env is not None else self.environment,
                input=stdin_text.encode("utf-8") if stdin_text is not None else None,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                timeout=timeout,
                check=False,
            )
        except FileNotFoundError as error:
            raise CaseFailure(f"{label} could not start: {error}") from error
        except subprocess.TimeoutExpired as error:
            partial = error.stdout or b""
            log.write_bytes(partial)
            raise CaseFailure(f"{label} exceeded {timeout} seconds") from error
        elapsed = time.monotonic() - started
        header = (
            f"$ {' '.join(argv)}\ncwd: {cwd}\nexit: {completed.returncode}\n"
            f"elapsed_seconds: {elapsed:.3f}\n\n"
        ).encode("utf-8")
        log.write_bytes(header + completed.stdout)
        execution = Execution(
            label,
            argv,
            cwd,
            completed.returncode,
            log,
            completed.stdout.decode("utf-8", errors="replace"),
            elapsed,
        )
        self.executions.append(execution)
        if completed.returncode != 0 and not allow_failure:
            raise CaseFailure(
                f"{label} exited {completed.returncode}: {' '.join(argv)}\n{execution.tail()}"
            )
        return execution

    def make(self, *targets: str, label: str | None = None, env: Mapping[str, str] | None = None) -> Execution:
        return self.run(MAKE + targets, label=label or "make-" + "-".join(targets), env=env)

    def cargo_test(
        self,
        manifest: Path,
        package: str,
        *arguments: str,
        label: str,
        cwd: Path = ROOT,
        env: Mapping[str, str] | None = None,
    ) -> Execution:
        argv = ("cargo", "test", "--manifest-path", str(manifest), "--locked", "-p", package, *arguments)
        execution = self.run(argv, label=label, cwd=cwd, env=env)
        if not execution.cargo_tests():
            raise CaseFailure(f"{label} executed no test")
        return execution

    def http(
        self,
        url: str,
        *,
        method: str = "GET",
        payload: object | None = None,
        headers: Mapping[str, str] | None = None,
    ) -> tuple[int, bytes]:
        body = json_bytes(payload) if payload is not None else None
        request = Request(url, data=body, method=method)
        request.add_header("Accept", "application/json")
        if body is not None:
            request.add_header("Content-Type", "application/json")
        for name, value in (headers or {}).items():
            request.add_header(name, value)
        try:
            with urlopen(request, timeout=HTTP_TIMEOUT_SECONDS) as response:
                return response.status, response.read()
        except HTTPError as error:
            return error.code, error.read()
        except (URLError, OSError, ValueError) as error:
            raise CaseFailure(f"{method} {url} is unreachable: {error}") from error

    def http_json(self, url: str, **arguments: object) -> dict[str, object]:
        status, body = self.http(url, **arguments)
        if status < 200 or status >= 300:
            raise CaseFailure(f"{url} answered HTTP {status}: {body[:512].decode('utf-8', 'replace')}")
        try:
            parsed = json.loads(body)
        except json.JSONDecodeError as error:
            raise CaseFailure(f"{url} did not answer JSON: {error}") from error
        if not isinstance(parsed, dict):
            raise CaseFailure(f"{url} did not answer a JSON object")
        return parsed

    def register(self, kind: str, path: Path) -> Path:
        if any(artifact["kind"] == kind for artifact in self.artifacts):
            raise CaseFailure(f"artifact kind {kind} registered twice")
        if not path.is_file() or path.stat().st_size == 0:
            raise CaseFailure(f"artifact {kind} at {path} is not a non-empty file")
        self.artifacts.append(
            {"kind": kind, "path": self.relative(path), "sha256": file_digest(path)}
        )
        return path

    def artifact(self, kind: str, name: str, content: bytes) -> Path:
        directory = self.directory / "artifacts"
        directory.mkdir(parents=True, exist_ok=True)
        path = directory / name
        if path.exists():
            raise CaseFailure(f"artifact file {name} already exists")
        path.write_bytes(content)
        return self.register(kind, path)

    def artifact_json(self, kind: str, name: str, payload: object) -> Path:
        return self.artifact(kind, name, json_bytes(payload))

    def artifact_copy(self, kind: str, source: Path) -> Path:
        directory = self.directory / "artifacts"
        directory.mkdir(parents=True, exist_ok=True)
        destination = directory / f"{kind}-{source.name}"
        if destination.exists():
            raise CaseFailure(f"artifact file {destination.name} already exists")
        shutil.copyfile(source, destination)
        return self.register(kind, destination)

    def artifact_log(self, kind: str, execution: Execution) -> Path:
        return self.register(kind, execution.log)

    def execution_records(self) -> list[dict[str, object]]:
        return [
            {
                "label": execution.label,
                "argv": list(execution.argv),
                "cwd": str(execution.cwd),
                "exit_code": execution.exit_code,
                "elapsed_seconds": round(execution.elapsed_seconds, 3),
                "log": self.relative(execution.log),
                "log_sha256": file_digest(execution.log),
            }
            for execution in self.executions
        ]

    def result(self, **details: object) -> Path:
        payload = {
            "case": self.case_id,
            "gate": self.gate,
            "source_identity": self.source_identity,
            "components": self.stack.components(),
            "executions": self.execution_records(),
            **details,
            "scope": "paxd-committed" if self.paxd_evidence else "local-suite",
            "real_stack": bool(self.paxd_evidence),
            "paxd_evidence": list(self.paxd_evidence),
        }
        return self.artifact_json("result", "result.json", payload)

    def timing(self, bound_seconds: float) -> Path:
        elapsed = self.elapsed()
        payload = {
            "case": self.case_id,
            "started_at": self.started_at,
            "finished_at": utc_now(),
            "elapsed_seconds": round(elapsed, 3),
            "bound_seconds": bound_seconds,
            "within_bound": elapsed <= bound_seconds,
            "steps": [
                {"label": execution.label, "elapsed_seconds": round(execution.elapsed_seconds, 3)}
                for execution in self.executions
            ],
        }
        path = self.artifact_json("timing", "timing.json", payload)
        if elapsed > bound_seconds:
            raise CaseFailure(f"took {elapsed:.1f} seconds, over the {bound_seconds:.0f} second bound")
        return path

    def finish(self, kinds: frozenset[str]) -> None:
        present = {artifact["kind"] for artifact in self.artifacts}
        missing = sorted(kinds - present)
        if missing:
            raise CaseFailure("case produced no artifact of kind " + ", ".join(missing))


CaseFunction = Callable[[CaseContext], None]
CASES: dict[str, dict[str, CaseFunction]] = {gate: {} for gate in DRIVER_GATES}


def case(gate: str, case_id: str) -> Callable[[CaseFunction], CaseFunction]:
    def register(function: CaseFunction) -> CaseFunction:
        if case_id in CASES[gate]:
            raise ValueError(f"case {case_id} registered twice for {gate}")
        CASES[gate][case_id] = function
        return function

    return register


def crate_identity(directory: Path) -> dict[str, object]:
    manifest = directory / "Cargo.toml"
    text = manifest.read_text(encoding="utf-8")
    name = re.search(r'^name = "([^"]+)"', text, re.MULTILINE)
    version = re.search(r'^version = "([^"]+)"', text, re.MULTILINE)
    workspace_version = re.search(r"^version(?:\.workspace)? = true", text, re.MULTILINE)
    pins: dict[str, str] = {}
    for source in sorted((directory / "src").glob("*.rs")):
        pins.update(PIN_CONSTANT.findall(source.read_text(encoding="utf-8")))
    return {
        "crate": name.group(1) if name else directory.name,
        "version": version.group(1) if version else ("workspace" if workspace_version else None),
        "manifest": str(manifest.relative_to(ROOT)),
        "manifest_sha256": file_digest(manifest),
        "pinned_constants": pins,
    }


def agent_workspace_identity(manifest: Path = AGENT_MANIFEST) -> dict[str, object]:
    try:
        workspace = tomllib.loads(manifest.read_text(encoding="utf-8"))["workspace"]
        members = workspace["members"]
        identities = {}
        for member, expected in (
            ("crates/layerx-agentd", "layerx-agentd"),
            ("crates/layerx-agentd-host", "layerx-agentd-host"),
        ):
            if member not in members:
                raise CaseFailure(f"agent workspace is missing {member}")
            path = manifest.parent / member / "Cargo.toml"
            document = tomllib.loads(path.read_text(encoding="utf-8"))
            if document["package"]["name"] != expected:
                raise CaseFailure(f"agent package identity mismatch at {path}")
            identities[expected] = {"manifest": str(path), "sha256": file_digest(path)}
            if expected == "layerx-agentd-host":
                binaries = [entry for entry in document.get("bin", []) if entry["name"] == "layerx-agentd"]
                source = manifest.parent / "crates/layerx-agentd/src/main.rs"
                if (len(binaries) != 1 or not source.is_file()
                        or (path.parent / binaries[0]["path"]).resolve() != source.resolve()):
                    raise CaseFailure("layerx-agentd binary must be owned by layerx-agentd-host")
                identities[expected]["binary"] = "layerx-agentd"
                identities[expected]["source_sha256"] = file_digest(source)
        return {"manifest": str(manifest), "sha256": file_digest(manifest), "packages": identities}
    except (OSError, ValueError, KeyError, TypeError) as error:
        raise CaseFailure(f"invalid agent workspace identity: {error}") from error


def network_trust(ctx: CaseContext) -> tuple[Path, dict[str, object]]:
    path = ctx.require_file(
        Path(ctx.require_env(NETWORK_TRUST_INPUT)[NETWORK_TRUST_INPUT]), "independent network trust"
    )
    try:
        trust = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(trust, dict) or set(trust) != {
            "node_url", "comet_url", "chain_id", "evm_chain_id", "receipt_keys", "validators"
        }:
            raise ValueError("network trust fields are incomplete")
        if validated_url(trust["node_url"], "trusted node") != ctx.stack.node:
            raise ValueError("node endpoint differs from independent network trust")
        validated_url(trust["comet_url"], "trusted Comet endpoint")
        if (not isinstance(trust["chain_id"], str) or not trust["chain_id"]
                or type(trust["evm_chain_id"]) is not int or trust["evm_chain_id"] <= 0
                or not isinstance(trust["receipt_keys"], list) or not trust["receipt_keys"]
                or not isinstance(trust["validators"], list) or not trust["validators"]):
            raise ValueError("independent network trust facts are missing")
    except (OSError, ValueError, TypeError, QualificationFailure) as error:
        raise CaseFailure(f"invalid independent network trust: {error}") from error
    return path, trust


def protobuf_fields(message: bytes) -> dict[int, list[bytes | int]]:
    """Read the length-delimited and uint fields of the native TxRaw envelope."""
    if not message or len(message) > 8388608:
        raise ValueError("invalid native transaction envelope size")
    offset = 0

    def integer() -> int:
        nonlocal offset
        value = 0
        for shift in range(0, 70, 7):
            if offset >= len(message):
                raise ValueError("truncated native protobuf integer")
            byte = message[offset]
            offset += 1
            value |= (byte & 127) << shift
            if byte < 128:
                if value >= 1 << 64:
                    raise ValueError("native protobuf integer overflow")
                return value
        raise ValueError("native protobuf integer overflow")

    fields: dict[int, list[bytes | int]] = {}
    while offset < len(message):
        tag = integer()
        field, wire = tag >> 3, tag & 7
        if not 0 < field < 1 << 29:
            raise ValueError("invalid native protobuf tag")
        if wire == 0:
            value = integer()
        elif wire == 2:
            size = integer()
            if offset + size > len(message):
                raise ValueError("truncated native protobuf field")
            value = message[offset:offset + size]
            offset += size
        else:
            raise ValueError("unsupported native protobuf wire type")
        fields.setdefault(field, []).append(value)
    return fields


def bind_native_transaction(envelope: bytes, raw_transaction: bytes) -> None:
    """Bind TxRaw/TxBody/Any/MsgEVMTransaction to the exact signed EVM call."""
    from layerx_sdk.evm_transaction import LegacyTransaction, decode_signed_transaction

    def one(fields: dict, field: int) -> bytes:
        values = fields[field]
        if len(values) != 1 or not isinstance(values[0], bytes):
            raise ValueError("ambiguous native transaction envelope")
        return values[0]

    outer = protobuf_fields(envelope)
    body = protobuf_fields(one(outer, 1))
    message = protobuf_fields(one(body, 1))
    if one(message, 1) != b"/paxprotocol.paxchain.evm.MsgEVMTransaction":
        raise ValueError("native transaction is not an EVM producer message")
    evm = protobuf_fields(one(message, 2))
    data = protobuf_fields(one(evm, 1))
    tx, signature = decode_signed_transaction(raw_transaction)
    parity = signature[64] - 27
    if isinstance(tx, LegacyTransaction):
        kind = b"/paxprotocol.paxchain.eth.LegacyTx"
        expected = {
            1: tx.nonce, 2: str(tx.gas_price).encode(), 3: tx.gas_limit,
            4: tx.to, 5: str(tx.value).encode(), 6: tx.data,
            7: (tx.chain_id * 2 + 35 + parity).to_bytes(9, "big").lstrip(b"\0"),
            8: signature[:32].lstrip(b"\0"), 9: signature[32:64].lstrip(b"\0"),
        }
        address_field = 4
    else:
        kind = b"/paxprotocol.paxchain.eth.DynamicFeeTx"
        expected = {
            1: str(tx.chain_id).encode(), 2: tx.nonce,
            3: str(tx.max_priority_fee_per_gas).encode(), 4: str(tx.max_fee_per_gas).encode(),
            5: tx.gas_limit, 6: tx.to, 7: str(tx.value).encode(), 8: tx.data,
            10: bytes([parity]) if parity else b"", 11: signature[:32].lstrip(b"\0"),
            12: signature[32:64].lstrip(b"\0"),
        }
        address_field = 6
    if one(data, 1) != kind:
        raise ValueError("native EVM transaction type mismatch")
    fields = protobuf_fields(one(data, 2))
    if set(fields) - set(expected):
        raise ValueError("unexpected native EVM transaction fields")
    for field, expected_value in expected.items():
        # Proto3 omits zero uints and empty bytes; these are schema defaults.
        values = fields.get(field, [0 if type(expected_value) is int else b""])
        if len(values) != 1:
            raise ValueError("duplicate native EVM transaction field")
        value = values[0]
        if field == address_field:
            address = value.decode("ascii")
            if re.fullmatch(r"0x[0-9a-fA-F]{40}", address) is None:
                raise ValueError("invalid native EVM destination")
            value = bytes.fromhex(address[2:])
        if type(value) is not type(expected_value) or value != expected_value:
            raise ValueError("native envelope differs from signed EVM transaction")


def verify_paxd_commit(
    ctx: CaseContext, transaction_hash: str, *, receipt_hex: str | None = None,
) -> dict[str, object]:
    """Bind a case's transaction to a live receipt and independently trusted commit."""
    trust_path, trust = network_trust(ctx)
    sdk = str(ROOT / "services/agent/sdk/python")
    if sdk not in sys.path:
        sys.path.insert(0, sdk)
    from layerx_sdk.committed import CommitTrust
    from layerx_sdk.evm_transaction import decode_signed_transaction
    from layerx_sdk.x402_rpc import (
        PaymentRpc, _transaction_bytes, _verified_rpc_receipt, decode_receipt_keys,
    )

    try:
        _transaction_bytes(transaction_hash, 32)
        rpc = PaymentRpc(ctx.stack.node)
        chain = rpc.call("eth_chainId", [])
        if not isinstance(chain, str) or int(chain, 16) != trust["evm_chain_id"]:
            raise ValueError("live node chain differs from independent network trust")
        raw = _transaction_bytes(rpc.call("eth_getRawTransactionByHash", [transaction_hash]))
        transaction, _ = decode_signed_transaction(raw)
        if transaction.chain_id != trust["evm_chain_id"]:
            raise ValueError("transaction chain differs from independent network trust")
        receipt = rpc.receipt(transaction_hash)
        landing = rpc.status(transaction_hash)
        if receipt is None or landing is None or receipt.get("rung") != "committed":
            raise ValueError("committed receipt verification was not performed: transaction is not committed")
        if receipt_hex is not None and _transaction_bytes(receipt_hex) != _transaction_bytes(receipt["receipt"]):
            raise ValueError("CLI receipt differs from live paxd receipt")
        height = int(landing["blockNumber"], 16)
        if (height <= 0 or landing["blockHash"] != receipt["blockHash"]
                or landing["transactionIndex"] != receipt["index"]):
            raise ValueError("committed transaction position mismatch")
        comet = str(trust["comet_url"]).rstrip("/")
        commit_response = ctx.http_json(f"{comet}/commit?height={height}")
        block_response = ctx.http_json(f"{comet}/block?height={height}")
        tx_index = int(landing["transactionIndex"], 16)
        if tx_index < 0:
            raise ValueError("invalid native transaction index")
        envelope = base64.b64decode(block_response["result"]["block"]["data"]["txs"][tx_index], validate=True)
        bind_native_transaction(envelope, raw)
        tx_digest = hashlib.sha256(envelope).hexdigest().upper()
        tx_response = ctx.http_json(f"{comet}/tx?hash=0x{tx_digest}&prove=true")
        signed_header = commit_response["result"]["signed_header"]
        if commit_response["result"]["canonical"] is not True:
            raise ValueError("Comet did not report a canonical committed block")
        header = signed_header["header"]
        # Comet RPC uses decimal heights and base64 signatures and proof aunts.
        commit = json.loads(json.dumps(signed_header["commit"]))
        commit["height"] = int(commit["height"])
        commit["round"] = int(commit["round"])
        for signature in commit["signatures"]:
            if signature["block_id_flag"] == 2:
                signature["signature"] = base64.b64decode(signature["signature"], validate=True).hex()
        tx = tx_response["result"]
        wire_proof = tx["proof"]
        proof = wire_proof["proof"]
        # ExecTxResult.code is a protobuf uint32 with json omitempty: absent is zero.
        code = tx["tx_result"].get("code", 0)
        if (int(tx["height"]) != height or commit["height"] != height
                or tx["hash"].upper() != tx_digest or type(code) is not int or code != 0
                or base64.b64decode(tx["tx"], validate=True) != envelope
                or int(tx["index"]) != int(landing["transactionIndex"], 16)
                or int(proof["index"]) != int(tx["index"])):
            raise ValueError("Comet committed result differs from case transaction")
        evidence = {
            "header": header, "commit": commit,
            "proof": {
                "data": base64.b64decode(wire_proof["data"], validate=True).hex(),
                "index": int(proof["index"]), "total": int(proof["total"]),
                "aunts": [base64.b64decode(aunt, validate=True).hex() for aunt in proof["aunts"]],
            },
        }
        keys = decode_receipt_keys({"keys": trust["receipt_keys"]})
        authority = CommitTrust(trust["chain_id"], height, tuple(trust["validators"]))
        verified = _verified_rpc_receipt(receipt, raw, keys)
        if verified is None or verified.receipt.result_code != 0:
            raise ValueError("signed receipt verification was not successful")
        committed = authority.verify(envelope, verified.height, evidence)
        if (_transaction_bytes(receipt["blockHash"], 32) != committed.block_hash
                or bytes.fromhex(block_response["result"]["block_id"]["hash"]) != committed.block_hash):
            raise ValueError("committed block identity mismatch")
    except Exception as error:
        raise CaseFailure(f"committed receipt verification failed: {error}") from error
    record = {
        "case": ctx.case_id, "performed": True, "verified": True, "endpoint": ctx.stack.node,
        "comet_endpoint": trust["comet_url"], "chain_id": trust["chain_id"],
        "evm_chain_id": trust["evm_chain_id"], "transaction_hash": transaction_hash,
        "raw_transaction_sha256": bytes_digest(raw), "receipt_sha256": bytes_digest(verified.canonical_bytes),
        "native_transaction_sha256": bytes_digest(envelope),
        "height": height, "block_hash": receipt["blockHash"],
        "trust_sha256": file_digest(trust_path),
        "commit_response_sha256": bytes_digest(json_bytes(commit_response)),
        "block_response_sha256": bytes_digest(json_bytes(block_response)),
        "transaction_response_sha256": bytes_digest(json_bytes(tx_response)),
    }
    ctx.paxd_evidence.append(record)
    ctx.artifact_json("paxd-commit", "paxd-commit.json", {
        **record, "receipt": receipt, "transaction_receipt": landing,
        "commit_response": commit_response, "block_response": block_response, "transaction_response": tx_response,
    })
    return record


def verify_first_payment_receipt(
    ctx: CaseContext, receipt_id: str, receipt: dict[str, object],
) -> dict[str, object]:
    receipt_hex = receipt.get("receipt")
    if not isinstance(receipt_hex, str) or not receipt_hex:
        raise CaseFailure(f"receipt read returned no receipt bytes: {json.dumps(receipt)}")
    if receipt.get("txHash") != receipt_id:
        raise CaseFailure("receipt read changed the payment transaction identity")
    return verify_paxd_commit(ctx, receipt_id, receipt_hex=receipt_hex)


def file_inventory(paths: Sequence[Path]) -> list[dict[str, object]]:
    return [
        {
            "path": str(path.relative_to(ROOT)),
            "sha256": file_digest(path),
            "bytes": path.stat().st_size,
        }
        for path in paths
    ]


def node_package(sample: Path, name: str) -> dict[str, object]:
    for parent in (sample, *sample.parents):
        manifest = parent / "node_modules" / name / "package.json"
        if manifest.is_file():
            resolved = manifest.resolve()
            metadata = json.loads(resolved.read_text(encoding="utf-8"))
            record: dict[str, object] = {
                "package": metadata.get("name"),
                "version": metadata.get("version"),
                "resolved_from": str(manifest),
                "path": str(resolved),
                "package_json_sha256": file_digest(resolved),
            }
            main = metadata.get("main")
            if isinstance(main, str) and (resolved.parent / main).is_file():
                record["main"] = main
                record["main_sha256"] = file_digest(resolved.parent / main)
            return record
    raise CaseFailure(f"{name} is not installed for {sample.relative_to(ROOT)}; run make platform-sample-install")


def sample_environment(ctx: CaseContext, ecosystem: str) -> dict[str, str]:
    inputs = ctx.require_env(*SAMPLE_INPUTS)
    environment = dict(ctx.environment)
    environment.update(inputs)
    environment["LAYERX_API_URL"] = ctx.stack.human_service
    environment["LAYERX_PAYMENT_KEY"] = f"beta-{ecosystem}-first-payment-{secrets.token_hex(8)}"
    return environment


def settled_journey(execution: Execution) -> dict[str, object]:
    journey = execution.last_json()
    state = journey.get("state")
    if state not in SETTLED_STATES:
        raise CaseFailure(f"{execution.label} settled in state {state!r}: {json.dumps(journey)}")
    receipts = journey.get("receipts")
    if not isinstance(receipts, list) or not receipts:
        raise CaseFailure(f"{execution.label} reported no receipt evidence: {json.dumps(journey)}")
    return journey


def first_payment(
    ctx: CaseContext,
    ecosystem: str,
    sample: Path,
    published: dict[str, object],
    steps: Sequence[tuple[str, Sequence[str]]],
    environment: Mapping[str, str],
) -> None:
    execution: Execution | None = None
    for label, argv in steps:
        execution = ctx.run(
            argv, label=label, cwd=sample, env=environment, timeout=FIVE_MINUTES_SECONDS
        )
    assert execution is not None
    journey = settled_journey(execution)
    ctx.artifact_json(
        "published-artifact",
        "published-artifact.json",
        {"ecosystem": ecosystem, "sample": str(sample.relative_to(ROOT)), **published},
    )
    ctx.timing(FIVE_MINUTES_SECONDS)
    ctx.result(
        ecosystem=ecosystem,
        endpoint=ctx.stack.human_service,
        payment_key=environment["LAYERX_PAYMENT_KEY"],
        journey=journey,
    )


@case("platform-qualify-adoption", "typescript/first-payment")
def typescript_first_payment(ctx: CaseContext) -> None:
    sample = SAMPLES / "first-payment-typescript"
    environment = sample_environment(ctx, "typescript")
    published = {
        "packages": [
            node_package(sample, "@sidiora/layerx-sdk"),
            node_package(sample, "@sidiora/layerx-buyer-middleware"),
        ],
        "node": ctx.run(("node", "--version"), label="node-version").output.strip(),
    }
    first_payment(ctx, "typescript", sample, published, (("run", ("node", "index.mjs")),), environment)


@case("platform-qualify-adoption", "python/first-payment")
def python_first_payment(ctx: CaseContext) -> None:
    sample = SAMPLES / "first-payment-python"
    environment = sample_environment(ctx, "python")
    interpreter = Path(ctx.environment.get("PLATFORM_PYTHON_ENV", str(ROOT / "build" / "platform-python"))) / "bin" / "python"
    if not interpreter.is_file():
        raise CaseFailure(
            f"python sample dependencies are not installed at {interpreter}; run make platform-sample-install"
        )
    identity = ctx.run(
        (
            str(interpreter),
            "-c",
            "import importlib.metadata, json, layerx_sdk, layerx_transport\n"
            "print(json.dumps({'distribution': 'layerx-sdk', 'version': importlib.metadata.version('layerx-sdk'),"
            " 'layerx_sdk': layerx_sdk.__file__, 'layerx_transport': layerx_transport.__file__}))",
        ),
        label="sdk-identity",
        cwd=sample,
    ).last_json()
    published = {"interpreter": str(interpreter), "distribution": identity}
    first_payment(ctx, "python", sample, published, (("run", (str(interpreter), "main.py")),), environment)


@case("platform-qualify-adoption", "go/first-payment")
def go_first_payment(ctx: CaseContext) -> None:
    sample = SAMPLES / "first-payment-go"
    environment = sample_environment(ctx, "go")
    environment["GOPROXY"] = "off"
    module = None
    for line in (sample / "go.mod").read_text(encoding="utf-8").splitlines():
        if line.startswith("require "):
            module = line.split()[1]
            break
    if module is None:
        raise CaseFailure("go sample declares no required LayerX module")
    listing = ctx.run(
        ("go", "list", "-m", "-json", module), label="sdk-identity", cwd=sample, env=environment
    ).last_json()
    published = {"module": listing, "go": ctx.run(("go", "version"), label="go-version").output.strip()}
    first_payment(ctx, "go", sample, published, (("run", ("go", "run", ".")),), environment)


@case("platform-qualify-adoption", "jvm/first-payment")
def jvm_first_payment(ctx: CaseContext) -> None:
    sample = SAMPLES / "first-payment-jvm"
    environment = sample_environment(ctx, "jvm")
    pom = (sample / "pom.xml").read_text(encoding="utf-8")
    match = re.search(r"<artifactId>layerx-sdk</artifactId>\s*<version>([^<]+)</version>", pom)
    if match is None:
        raise CaseFailure("jvm sample does not depend on com.sidiora.layerx:layerx-sdk")
    version = match.group(1)
    repository = Path(
        ctx.environment.get("PLATFORM_MAVEN_REPOSITORY", str(Path.home() / ".m2" / "repository"))
    )
    jar = repository / "com" / "sidiora" / "layerx" / "layerx-sdk" / version / f"layerx-sdk-{version}.jar"
    ctx.require_file(jar, f"com.sidiora.layerx:layerx-sdk:{version}")
    published = {
        "artifact": f"com.sidiora.layerx:layerx-sdk:{version}",
        "jar": str(jar),
        "jar_sha256": file_digest(jar),
    }
    first_payment(
        ctx,
        "jvm",
        sample,
        published,
        (("build", ("mvn", "-o", "-q", "package")), ("run", ("mvn", "-o", "-q", "exec:java"))),
        environment,
    )


@case("platform-qualify-adoption", "swift/first-payment")
def swift_first_payment(ctx: CaseContext) -> None:
    sample = SAMPLES / "first-payment-swift"
    environment = sample_environment(ctx, "swift")
    ctx.run(
        ("swift", "build"), label="build", cwd=sample, env=environment, timeout=FIVE_MINUTES_SECONDS
    )
    dependencies = ctx.run(
        ("swift", "package", "show-dependencies", "--format", "json"),
        label="sdk-identity",
        cwd=sample,
        env=environment,
    )
    published = {
        "dependencies": json.loads(dependencies.output[dependencies.output.index("{") :]),
        "swift": ctx.run(("swift", "--version"), label="swift-version").output.strip(),
    }
    first_payment(ctx, "swift", sample, published, (("run", ("swift", "run", "FirstPayment")),), environment)


@case("platform-qualify-adoption", "dotnet/first-payment")
def dotnet_first_payment(ctx: CaseContext) -> None:
    sample = SAMPLES / "first-payment-csharp"
    environment = sample_environment(ctx, "dotnet")
    environment.setdefault("DOTNET_CLI_TELEMETRY_OPTOUT", "1")
    ctx.run(
        ("dotnet", "build", "--configuration", "Release"),
        label="build",
        cwd=sample,
        env=environment,
        timeout=FIVE_MINUTES_SECONDS,
    )
    assemblies = sorted(sample.glob("bin/Release/*/LayerX.Sdk.dll"))
    if not assemblies:
        raise CaseFailure("dotnet build produced no LayerX.Sdk.dll under bin/Release")
    published = {
        "assemblies": [{"path": str(path), "sha256": file_digest(path)} for path in assemblies],
        "dotnet": ctx.run(("dotnet", "--version"), label="dotnet-version").output.strip(),
    }
    first_payment(
        ctx,
        "dotnet",
        sample,
        published,
        (("run", ("dotnet", "run", "--configuration", "Release")),),
        environment,
    )


def layerx_binary(ctx: CaseContext) -> Path:
    configured = ctx.environment.get(CLI_BINARY_INPUT, "")
    if configured:
        return ctx.require_executable(Path(configured), CLI_BINARY_INPUT)
    located = shutil.which("layerx")
    if located is None:
        raise CaseFailure(f"layerx CLI not found; set {CLI_BINARY_INPUT} or put layerx on PATH")
    return Path(located)


def cli_environment(ctx: CaseContext, endpoint: str) -> tuple[Path, dict[str, str], str]:
    binary = layerx_binary(ctx)
    inputs = ctx.require_env(*CLI_INPUTS)
    environment = dict(ctx.environment)
    home = ctx.directory / "cli"
    home.mkdir(exist_ok=True)
    environment["LAYERX_CONFIG"] = str(home / "config.json")
    environment["LAYERX_INSTALL_ROOT"] = str(home)
    name = ctx.environment.get(CLI_ENVIRONMENT_INPUT, "emulator")
    ctx.run(
        (
            str(binary),
            "--json",
            "environment",
            "use",
            name,
            "--endpoint",
            endpoint,
            "--network-id",
            inputs["LAYERX_NETWORK_ID"],
            "--sequencer-trust-anchor",
            inputs["LAYERX_SEQUENCER_TRUST_ANCHOR"],
        ),
        label="environment-use",
        env=environment,
    )
    return binary, environment, name


def cli_output(execution: Execution, kind: str) -> dict[str, object]:
    if execution.exit_code != 0:
        raise CaseFailure(f"{execution.label} exited {execution.exit_code}")
    try:
        document = json.loads(execution.output)
    except json.JSONDecodeError as error:
        raise CaseFailure(f"{execution.label} did not print exactly one JSON object") from error
    if not isinstance(document, dict):
        raise CaseFailure(f"{execution.label} printed no JSON object")
    if document.get("ok") is not True or document.get("kind") != kind:
        raise CaseFailure(f"{execution.label} did not report {kind}: {json.dumps(document)}")
    data = document.get("data")
    if not isinstance(data, dict):
        raise CaseFailure(f"{execution.label} carried no data object")
    return data


@case("platform-qualify-adoption", "rust/first-payment")
def rust_first_payment(ctx: CaseContext) -> None:
    network_trust(ctx)
    inputs = ctx.require_env(*SAMPLE_INPUTS)
    binary, environment, name = cli_environment(ctx, ctx.stack.human_service)
    ctx.run(
        (str(binary), "--json", "auth", "set", "--environment", name),
        label="auth-set",
        env=environment,
        stdin_text=inputs["LAYERX_API_TOKEN"] + "\n",
    )
    payment_key = f"beta-rust-first-payment-{secrets.token_hex(8)}"
    payment = cli_output(
        ctx.run(
            (
                str(binary),
                "--json",
                "payment",
                "test",
                "--from",
                inputs["LAYERX_SOURCE"],
                "--to",
                inputs["LAYERX_DESTINATION"],
                "--currency",
                inputs["LAYERX_CURRENCY"],
                "--amount",
                inputs["LAYERX_AMOUNT"],
                "--idempotency-key",
                payment_key,
            ),
            label="payment-test",
            env=environment,
        ),
        "payment.started",
    )
    journey = payment.get("journey")
    result = journey.get("result") if isinstance(journey, dict) else None
    if not isinstance(result, dict) or result.get("state") not in SETTLED_STATES:
        raise CaseFailure(f"payment test did not settle: {json.dumps(payment)}")
    evidence = result.get("evidence")
    if not isinstance(evidence, list) or not evidence:
        raise CaseFailure(f"payment test carried no receipt evidence: {json.dumps(result)}")
    first = evidence[0]
    if not isinstance(first, dict) or first.get("verification") != "receipt-verified":
        raise CaseFailure(f"payment evidence is not receipt-verified: {json.dumps(first)}")
    evidence_id = first.get("evidence_id")
    if not isinstance(evidence_id, str) or not evidence_id:
        raise CaseFailure("payment evidence carries no evidence_id")
    source_ref = first.get("source_ref")
    receipt_id = (
        source_ref.rsplit("/", 1)[1]
        if isinstance(source_ref, str) and source_ref.startswith("/v1/receipts/")
        else evidence_id
    )
    receipt = cli_output(
        ctx.run(
            (str(binary), "--json", "receipt", "get", receipt_id),
            label="receipt-get",
            env=environment,
        ),
        "receipt.read",
    )
    verification = verify_first_payment_receipt(ctx, receipt_id, receipt)
    version = ctx.run((str(binary), "--version"), label="cli-version").output.strip()
    ctx.artifact_json(
        "published-artifact",
        "published-artifact.json",
        {
            "ecosystem": "rust",
            "binary": str(binary),
            "binary_sha256": file_digest(binary),
            "version": version,
            "credential_store": ctx.environment.get("LAYERX_CREDENTIAL_STORE", "operating-system"),
        },
    )
    ctx.timing(FIVE_MINUTES_SECONDS)
    ctx.result(
        ecosystem="rust",
        endpoint=ctx.stack.human_service,
        payment_key=payment_key,
        journey=result,
        receipt={
            "evidence_id": evidence_id,
            "receipt_id": receipt_id,
            "receipt_sha256": verification["receipt_sha256"],
        },
        local_verification=verification,
    )


@case("platform-qualify-adoption", "middleware/ten-line-integration")
def middleware_ten_line_integration(ctx: CaseContext) -> None:
    build_site = ROOT / "services" / "platform" / "docs" / "build" / "build_site.py"
    specification = importlib.util.spec_from_file_location("layerx_build_site", build_site)
    if specification is None or specification.loader is None:
        raise CaseFailure(f"could not load {build_site}")
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    try:
        samples = module.load_samples(ROOT / "services" / "platform" / "docs")
        measurements = module.measure_samples(samples)
    except module.DocumentationError as error:
        raise CaseFailure(f"documentation line budget failed: {error}") from error
    if not measurements:
        raise CaseFailure("the documentation gate measured no sample")
    workspaces = (
        "@sidiora/layerx-seller-middleware",
        "@sidiora/layerx-buyer-middleware",
        "@sidiora/layerx-merchant-middleware",
        "@sidiora/layerx-agent-middleware",
        "@sidiora/layerx-middleware-conformance",
    )
    for workspace in workspaces:
        ctx.run(
            ("npm", "run", "build", "--workspace", workspace),
            label=f"build-{workspace.split('/')[-1]}",
            timeout=FIVE_MINUTES_SECONDS,
        )
    conformance = ctx.run(
        ("npm", "run", "conformance", "--workspace", "@sidiora/layerx-middleware-conformance"),
        label="conformance",
        timeout=FIVE_MINUTES_SECONDS,
    )
    sample = SAMPLES / "first-payment-typescript"
    ctx.artifact_json(
        "published-artifact",
        "published-artifact.json",
        {
            "ecosystem": "middleware",
            "packages": [
                node_package(sample, "@sidiora/layerx-buyer-middleware"),
                node_package(sample, "@sidiora/layerx-seller-middleware"),
                node_package(sample, "@sidiora/layerx-merchant-middleware"),
                node_package(sample, "@sidiora/layerx-agent-middleware"),
            ],
        },
    )
    ctx.timing(FIVE_MINUTES_SECONDS)
    ctx.result(
        measured_region="integration",
        measurements=measurements,
        conformance={"log": ctx.relative(conformance.log), "tail": conformance.tail()},
    )


@case("platform-qualify-adoption", "programs/five-minute-deploy-and-paid-call")
def programs_five_minute_deploy_and_paid_call(ctx: CaseContext) -> None:
    quickstart = ROOT / "programs" / "sdk" / "rust" / "quickstart"
    binary, environment, _ = cli_environment(ctx, ctx.stack.node)
    registry = cli_output(
        ctx.run(
            (str(binary), "--json", "program", "registry", "list"),
            label="registry-list",
            env=environment,
        ),
        "program.registry_list",
    )
    build_environment = {
        name: value for name, value in ctx.environment.items() if name != "CARGO_TARGET_DIR"
    }
    build = ctx.run(
        ("sh", str(quickstart / "build.sh"), "all"),
        label="quickstart-build",
        env=build_environment,
        timeout=FIVE_MINUTES_SECONDS,
    )
    printed = [line.strip() for line in build.output.splitlines() if line.strip()]
    if not printed:
        raise CaseFailure("quickstart build printed no artifact path")
    wasm = ctx.require_file(Path(printed[-1]), "quickstart artifact layerx_quickstart.wasm")
    deployment_key = f"beta-programs-deploy-{secrets.token_hex(8)}"
    deployment = cli_output(
        ctx.run(
            (
                str(binary),
                "--json",
                "program",
                "deploy",
                str(wasm),
                "--idempotency-key",
                deployment_key,
            ),
            label="program-deploy",
            env=environment,
        ),
        "program.deployment_started",
    )
    walkthrough_environment = dict(environment)
    walkthrough_environment["PATH"] = f"{binary.parent}{os.pathsep}{environment.get('PATH', '')}"
    walkthrough = ctx.run(
        ("bash", str(ROOT / "services" / "platform" / "examples" / "agent-program-call" / "walkthrough.sh")),
        label="walkthrough",
        env=walkthrough_environment,
    )
    ctx.artifact_json(
        "published-artifact",
        "published-artifact.json",
        {
            "ecosystem": "programs",
            "artifact": str(wasm),
            "artifact_sha256": file_digest(wasm),
            "cli": str(binary),
            "cli_sha256": file_digest(binary),
        },
    )
    ctx.timing(FIVE_MINUTES_SECONDS)
    ctx.result(
        endpoint=ctx.stack.node,
        registry=registry,
        deployment_key=deployment_key,
        deployment=deployment,
        walkthrough_tail=walkthrough.tail(),
    )


def gauntlet_rows() -> list[dict[str, str]]:
    lines = GAUNTLET_INVENTORY.read_text(encoding="utf-8").splitlines()
    header = lines[0].split("\t")
    return [dict(zip(header, line.split("\t"))) for line in lines[1:] if line.strip()]


def fault_boundaries() -> list[dict[str, object]]:
    text = FAULT_HEADER.read_text(encoding="utf-8")
    return [{"boundary": name, "value": int(value)} for name, value in FAULT_BOUNDARY.findall(text)]


def runtime_test(ctx: CaseContext, label: str, *arguments: str) -> Execution:
    return ctx.cargo_test(
        PROGRAMS_DIRECTORY / "Cargo.toml",
        "layerx-programs-runtime",
        *arguments,
        label=label,
        cwd=PROGRAMS_DIRECTORY,
    )


def agent_test(ctx: CaseContext, label: str, suite: str, *filters: str) -> Execution:
    agent_workspace_identity()
    arguments = ("--test", suite) + (("--", *filters) if filters else ())
    return ctx.cargo_test(AGENT_MANIFEST, "layerx-agentd", *arguments, label=label)


def native_test_command(package: str, tests: Sequence[str]) -> tuple[str, ...]:
    if not tests or any(re.fullmatch(r"Test[A-Za-z0-9_]+", name) is None for name in tests):
        raise CaseFailure("native qualification requires explicit test identities")
    return ("go", "test", package, "-count=1", "-v", "-run", "^(" + "|".join(tests) + ")$")


def native_test(ctx: CaseContext, label: str, package: str, *tests: str) -> Execution:
    execution = ctx.run(native_test_command(package, tests), label=label)
    passed = set(execution.native_tests())
    if not set(tests) <= passed:
        raise CaseFailure(f"{label} did not execute every declared native test: {sorted(set(tests) - passed)}")
    return execution


def built_binary(ctx: CaseContext, name: str, label: str) -> Execution:
    relative = f"build/tests/{name}"
    ctx.make(relative, label=f"build-{name}")
    return ctx.run((str(ROOT / relative),), label=label)


def programs_case(
    ctx: CaseContext,
    *,
    result: Sequence[Execution],
    ledger_proof: Execution,
    inventory: dict[str, object],
) -> None:
    executed = [
        {"label": execution.label, "argv": list(execution.argv),
         "tests": execution.native_tests() if execution.argv[:2] == ("go", "test") else execution.cargo_tests()}
        for execution in result
    ]
    ctx.artifact_json("inventory", "inventory.json", {"case": ctx.case_id, "executed": executed, **inventory})
    ctx.artifact_log("ledger-proof", ledger_proof)
    ctx.result(ledger_proof={"label": ledger_proof.label, "argv": list(ledger_proof.argv)})


@case("programs-qualify", "real-node/hostile-program-gauntlet")
def hostile_program_gauntlet(ctx: CaseContext) -> None:
    rows = gauntlet_rows()
    suites = sorted({row["suite"] for row in rows})
    execution = runtime_test(ctx, "gauntlet", *(argument for suite in suites for argument in ("--test", suite)))
    executed = set(execution.cargo_tests())
    missing = sorted(row["id"] for row in rows if row["test"] not in executed)
    if missing:
        raise CaseFailure("attack inventory rows did not execute: " + ", ".join(missing))
    ledger = built_binary(ctx, "programs_monetary_law", "kernel-monetary-law")
    ctx.artifact_copy("inventory", GAUNTLET_INVENTORY)
    ctx.artifact_log("ledger-proof", ledger)
    ctx.result(
        attacks=[{"id": row["id"], "suite": row["suite"], "test": row["test"]} for row in rows],
        executed_tests=sorted(executed),
    )


@case("programs-qualify", "real-node/isolation")
def isolation(ctx: CaseContext) -> None:
    execution = runtime_test(ctx, "isolation", "--test", "isolation")
    ledger = runtime_test(ctx, "runtime-monetary-law", "--test", "monetary_law")
    programs_case(ctx, result=(execution,), ledger_proof=ledger, inventory={"suite": "isolation"})


@case("programs-qualify", "real-node/determinism-differential")
def determinism_differential(ctx: CaseContext) -> None:
    execution = runtime_test(ctx, "replay-and-determinism", "--test", "replay", "--test", "determinism")
    ledger = built_binary(ctx, "programs_parallel_differential", "parallel-differential")
    programs_case(ctx, result=(execution,), ledger_proof=ledger, inventory={"suites": ["replay", "determinism"]})


@case("programs-qualify", "real-node/metering")
def metering(ctx: CaseContext) -> None:
    execution = ctx.make("test-metering")
    ledger = built_binary(ctx, "programs_metering_schedule", "metering-schedule")
    programs_case(ctx, result=(execution,), ledger_proof=ledger, inventory={"binaries": ["build/tests/test_metering"]})


@case("programs-qualify", "real-node/concurrent-same-account-transfers")
def concurrent_same_account_transfers(ctx: CaseContext) -> None:
    gateway = ctx.make("test-gateway-send")
    agent = agent_test(
        ctx, "agent-concurrent-duplicates", "idempotency", "concurrent_duplicates_produce_exactly_one_economic_effect"
    )
    ledger = native_test(ctx, "native-concurrent-commit", "./node/sequencer", "TestProofSequencerExecutorTransportRecovery")
    programs_case(
        ctx,
        result=(gateway, agent),
        ledger_proof=ledger,
        inventory={"binaries": ["build/tests/test_gateway_send"]},
    )


@case("programs-qualify", "real-node/duplicate-idempotency-keys")
def duplicate_idempotency_keys(ctx: CaseContext) -> None:
    kernel = ctx.make("test-idempotency")
    agent = agent_test(
        ctx, "agent-repeated-key", "idempotency", "repeated_key_returns_original_and_changed_body_conflicts"
    )
    ledger = native_test(ctx, "native-idempotent-recovery", "./node/sequencer", "TestCrashAfterReceiptReplaysIdenticalReceipt")
    programs_case(
        ctx,
        result=(kernel, agent),
        ledger_proof=ledger,
        inventory={"binaries": ["build/tests/lxp_test_idempotency"]},
    )


@case("programs-qualify", "real-node/lost-response-after-successful-commit")
def lost_response_after_successful_commit(ctx: CaseContext) -> None:
    agent = agent_test(ctx, "agent-lost-response", "unknown")
    ledger = native_test(ctx, "native-lost-response", "./node/sequencer", "TestCrashAfterReceiptReplaysIdenticalReceipt")
    programs_case(
        ctx,
        result=(agent,),
        ledger_proof=ledger,
        inventory={},
    )


CRASH_POINTS: dict[str, tuple[str, tuple[str, ...], tuple[str, ...]]] = {
    "real-node/crash-before-batch-wal": (
        "./node/sequencer/wal", ("TestWALGroupSyncBeforeRelease", "TestWALCorruptTailStopsReplay"), (),
    ),
    "real-node/crash-after-batch-wal": (
        "./node/sequencer/wal", ("TestWALReplayReproducesReceipts",), (),
    ),
    "real-node/crash-before-state-mutation": (
        "./node/sequencer", ("TestProofSequencerExecutorTransportRecovery",), (),
    ),
    "real-node/crash-after-state-mutation": (
        "./node", ("TestRepair475_CommitHashesSurviveEveryRestart", "TestRepair475_RejectCorruptedCommittedHash"), (),
    ),
    "real-node/crash-before-receipt-publication": (
        "./node/sequencer", ("TestProofSequencerExecutorTransportRecovery",), (),
    ),
    "real-node/crash-after-receipt-publication": (
        "./node/sequencer", ("TestCrashAfterReceiptReplaysIdenticalReceipt",), (),
    ),
    "real-node/crash-before-acknowledgement": (
        "./node", ("TestRepair475_ReconcileSealAfterCommitCrash",),
        ("idempotency", "post_restart_retry_reuses_original_result_and_pending_bytes"),
    ),
    "real-node/crash-after-acknowledgement": (
        "./node/sequencer", ("TestCrashAfterReceiptReplaysIdenticalReceipt",),
        ("unknown", "acknowledgement_loss_is_resolved_only_by_the_existing_receipt"),
    ),
}


def crash_case(case_id: str) -> CaseFunction:
    package, tests, agent = CRASH_POINTS[case_id]

    def run_crash_point(ctx: CaseContext) -> None:
        results = [native_test(ctx, "native-crash-recovery", package, *tests)]
        if agent:
            results.append(agent_test(ctx, f"agent-{agent[0]}", *agent))
        ledger = native_test(ctx, "native-commit-boundary", "./node", "TestRepair_102_StrictResultCommitment")
        programs_case(
            ctx,
            result=results,
            ledger_proof=ledger,
            inventory={
                "crash_point": case_id.rsplit("/", 1)[1],
                "package": package,
                "native_tests": list(tests),
                "agent_tests": list(agent[1:]),
                "scope": "local-native-restart",
            },
        )

    return run_crash_point


for crash_point in CRASH_POINTS:
    case("programs-qualify", crash_point)(crash_case(crash_point))


@case("programs-qualify", "ported-reference-contracts")
def ported_reference_contracts(ctx: CaseContext) -> None:
    execution = ctx.make("programs-porting-v2-references")
    references = sorted((ROOT / "programs" / "porting").glob("*/reference-v2/target/wasm32-unknown-unknown/release/*.wasm"))
    if not references:
        raise CaseFailure("no ported reference artifact was produced under programs/porting")
    ledger = built_binary(ctx, "programs_call_activity", "call-activity")
    programs_case(
        ctx,
        result=(execution,),
        ledger_proof=ledger,
        inventory={"references": file_inventory(references)},
    )


@case("programs-qualify", "program-heavy-monetary-law-replay")
def program_heavy_monetary_law_replay(ctx: CaseContext) -> None:
    conservation = ctx.make("programs-conservation")
    replay = native_test(
        ctx, "native-monetary-replay", "./node/sequencer", "TestCrashAfterReceiptReplaysIdenticalReceipt"
    )
    programs_case(
        ctx,
        result=(conservation,),
        ledger_proof=replay,
        inventory={
            "suites": ["programs-conservation", "TestCrashAfterReceiptReplaysIdenticalReceipt"],
            "scope": "local-native-restart",
        },
    )


def interop_test(ctx: CaseContext, label: str, package: str, *arguments: str) -> Execution:
    return ctx.cargo_test(INTEROP_MANIFEST, package, *arguments, label=label)


def interop_case(
    ctx: CaseContext,
    *,
    crate: str,
    result: Execution,
    proof: Execution,
    compatibility: dict[str, object],
) -> None:
    ctx.artifact_json(
        "compatibility",
        "compatibility.json",
        {"case": ctx.case_id, **crate_identity(ROOT / "services" / "interop" / "crates" / crate), **compatibility},
    )
    ctx.artifact_log("proof", proof)
    ctx.result(
        conformance_tests=result.cargo_tests(),
        proof={"label": proof.label, "argv": list(proof.argv), "tests": proof.cargo_tests()},
    )


def pinned_conformance(ctx: CaseContext, crate: str, package: str, target: str, proof_suite: str) -> None:
    pinned = interop_test(ctx, "pinned-spec", package, "--test", "pinned_spec")
    result = ctx.make(target)
    proof = interop_test(ctx, proof_suite, package, "--test", proof_suite)
    vectors = sorted(path for path in (ROOT / "services" / "interop" / "crates" / crate / "tests").rglob("*") if path.is_file() and path.suffix != ".rs")
    interop_case(
        ctx,
        crate=crate,
        result=result,
        proof=proof,
        compatibility={"pinned_spec_tests": pinned.cargo_tests(), "vectors": file_inventory(vectors)},
    )


@case("interop-qualify", "x402-v2/all-transports")
def x402_all_transports(ctx: CaseContext) -> None:
    pinned_conformance(ctx, "layerx-x402", "layerx-x402", "interop-test-x402", "transports")


@case("interop-qualify", "ap2/pinned-conformance")
def ap2_pinned_conformance(ctx: CaseContext) -> None:
    pinned_conformance(ctx, "layerx-ap2", "layerx-ap2", "interop-test-mandates", "mandates")


@case("interop-qualify", "ucp/pinned-conformance")
def ucp_pinned_conformance(ctx: CaseContext) -> None:
    pinned_conformance(ctx, "layerx-ucp", "layerx-ucp", "interop-test-ucp", "conformance_vectors")


@case("interop-qualify", "visa-trusted-agent/pinned-conformance")
def visa_trusted_agent_pinned_conformance(ctx: CaseContext) -> None:
    pinned_conformance(ctx, "layerx-visa-tap", "layerx-visa-tap", "interop-test-visa-tap", "conformance")


@case("interop-qualify", "portable-verification/layerx-to-external")
def portable_verification_layerx_to_external(ctx: CaseContext) -> None:
    vectors = interop_test(ctx, "receipt-vectors", "layerx-portable", "--test", "receipt_vectors")
    result = ctx.make("interop-test-portable")
    proof = interop_test(ctx, "external-verification", "layerx-portable", "--test", "external_verification")
    interop_case(
        ctx,
        crate="layerx-portable",
        result=result,
        proof=proof,
        compatibility={"direction": "layerx-to-external", "receipt_vector_tests": vectors.cargo_tests()},
    )


@case("interop-qualify", "portable-verification/external-to-layerx")
def portable_verification_external_to_layerx(ctx: CaseContext) -> None:
    vectors = interop_test(ctx, "receipt-vectors", "layerx-portable", "--test", "receipt_vectors")
    result = interop_test(ctx, "independent-verifier", "layerx-portable", "--test", "independent_verifier")
    proof = interop_test(
        ctx,
        "independent-verifier-rejection",
        "layerx-portable",
        "--test",
        "independent_verifier",
        "--",
        "independent_verifier_rejects_batch_mismatch",
        "independent_verifier_no_layerx_infrastructure_required",
    )
    interop_case(
        ctx,
        crate="layerx-portable",
        result=result,
        proof=proof,
        compatibility={"direction": "external-to-layerx", "receipt_vector_tests": vectors.cargo_tests()},
    )


@case("interop-qualify", "migration/fault-injection")
def migration_fault_injection(ctx: CaseContext) -> None:
    ctx.require_env(*MIGRATION_INPUTS)
    local = ctx.make("interop-test-migration")
    result = ctx.make("interop-test-migration-testnets")
    interop_case(
        ctx,
        crate="layerx-migrate",
        result=result,
        proof=local,
        compatibility={"testnet_inputs": list(MIGRATION_INPUTS), "live_tests": result.cargo_tests()},
    )


@case("interop-qualify", "fiat/fault-injection")
def fiat_fault_injection(ctx: CaseContext) -> None:
    result = interop_test(ctx, "fiat-adapter", "layerx-fiat")
    proof = interop_test(
        ctx,
        "fiat-fault-injection",
        "layerx-fiat",
        "--test",
        "adapter",
        "--",
        "provider_fault_injection_refuses_settlement_and_surfaces_honest_state",
        "idempotency_binds_provider_settlement_to_exactly_one_outcome",
        "receipt_mismatch_refuses_credit_and_preserves_honest_state",
    )
    toolkit = ctx.cargo_test(PLATFORM_MANIFEST, "layerx-ramp-toolkit", label="ramp-toolkit")
    interop_case(
        ctx,
        crate="layerx-fiat",
        result=result,
        proof=proof,
        compatibility={"ramp_toolkit_tests": toolkit.cargo_tests()},
    )


def mirror_verifier(ctx: CaseContext) -> Path:
    ctx.run(
        (
            "cargo",
            "build",
            "--locked",
            "--release",
            "--manifest-path",
            str(INTEROP_MANIFEST),
            "--package",
            "layerx-mirror",
            "--bin",
            "layerx-mirror-verify",
        ),
        label="build-mirror-verify",
    )
    target = Path(ctx.environment.get("CARGO_TARGET_DIR", str(ROOT / "services" / "interop" / "target")))
    return ctx.require_executable(target / "release" / "layerx-mirror-verify", "layerx-mirror-verify")


def mirror_verify(ctx: CaseContext, verifier: Path, config: str, request: str, label: str) -> tuple[Execution, dict[str, object]]:
    execution = ctx.run(
        (str(verifier), config),
        label=label,
        stdin_text=Path(request).read_text(encoding="utf-8"),
        allow_failure=True,
    )
    document = execution.last_json()
    return execution, document


@case("multichain-qualify", "mirrors/offline-verification")
def mirrors_offline_verification(ctx: CaseContext) -> None:
    inputs = ctx.require_env(*MIRROR_VERIFY_INPUTS)
    verifier = mirror_verifier(ctx)
    environment = {name: value for name, value in ctx.environment.items() if name not in MIRROR_LIVE_FORBIDDEN}
    environment["LAYERX_MIRROR_VERIFY_BIN"] = str(verifier)
    canonical, verdict = mirror_verify(
        ctx, verifier, inputs["LAYERX_MIRROR_VERIFY_CONFIG"], inputs["LAYERX_MIRROR_CANONICAL_REQUEST"], "canonical"
    )
    verification = verdict.get("verification")
    if (
        verdict.get("ok") is not True
        or not isinstance(verification, dict)
        or verification.get("provenance") != "Canonical"
        or not verification.get("sourceId")
    ):
        raise CaseFailure(f"canonical mirror verification was not accepted: {json.dumps(verdict)}")
    divergence, refusal = mirror_verify(
        ctx, verifier, inputs["LAYERX_MIRROR_VERIFY_CONFIG"], inputs["LAYERX_MIRROR_DIVERGENCE_REQUEST"], "divergence"
    )
    if refusal.get("ok") is not False or refusal.get("error") != "divergent":
        raise CaseFailure(f"divergent mirrors were not refused: {json.dumps(refusal)}")
    live = ctx.run(
        ("bash", str(ROOT / "scripts" / "qualify-mirror-verification-live.sh")),
        label="mirror-verification-live",
        env=environment,
    )
    ctx.artifact_log("proof", canonical)
    ctx.artifact_log("tamper-rejection", divergence)
    ctx.result(verifier=str(verifier), verifier_sha256=file_digest(verifier), live_tail=live.tail())


@case("multichain-qualify", "mirrors/tamper-rejection")
def mirrors_tamper_rejection(ctx: CaseContext) -> None:
    inputs = ctx.require_env(*MIRROR_TAMPER_INPUTS)
    verifier = mirror_verifier(ctx)
    canonical, verdict = mirror_verify(
        ctx, verifier, inputs["LAYERX_MIRROR_VERIFY_CONFIG"], inputs["LAYERX_MIRROR_CANONICAL_REQUEST"], "canonical"
    )
    if verdict.get("ok") is not True:
        raise CaseFailure(f"canonical mirror verification was not accepted: {json.dumps(verdict)}")
    tamper, refusal = mirror_verify(
        ctx, verifier, inputs["LAYERX_MIRROR_VERIFY_CONFIG"], inputs["LAYERX_MIRROR_TAMPER_REQUEST"], "tamper"
    )
    if refusal.get("ok") is not False or refusal.get("error") != "verification":
        raise CaseFailure(f"tampered mirror content was not refused: {json.dumps(refusal)}")
    archive = ctx.make("interop-test-mirrors")
    ctx.artifact_log("proof", canonical)
    ctx.artifact_log("tamper-rejection", tamper)
    ctx.result(verifier=str(verifier), verifier_sha256=file_digest(verifier), archive_tests=archive.cargo_tests())


@case("multichain-qualify", "surfaces/paxeer-exclusivity")
def paxeer_exclusivity(ctx: CaseContext) -> None:
    expected_chain = ctx.require_env(PAXEER_CHAIN_INPUT)[PAXEER_CHAIN_INPUT]
    chain = ctx.http_json(
        ctx.stack.paxeer_testnet,
        method="POST",
        payload={"jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []},
    )
    observed = chain.get("result")
    if not isinstance(observed, str) or int(observed, 16) != int(expected_chain, 0):
        raise CaseFailure(
            f"{ctx.stack.paxeer_testnet} reports chain {observed!r}, not {PAXEER_CHAIN_INPUT}={expected_chain}"
        )
    block = ctx.http_json(
        ctx.stack.paxeer_testnet,
        method="POST",
        payload={"jsonrpc": "2.0", "id": 2, "method": "eth_blockNumber", "params": []},
    )
    proof = ctx.artifact_json(
        "proof",
        "proof.json",
        {
            "paxeer_testnet": ctx.stack.paxeer_testnet,
            "expected_chain_id": expected_chain,
            "eth_chainId": chain,
            "eth_blockNumber": block,
            "observed_at": utc_now(),
        },
    )
    ctx.result(proof=ctx.relative(proof))


@case("multichain-qualify", "surfaces/one-ledger")
def one_ledger(ctx: CaseContext) -> None:
    schema = ctx.run(
        (
            "cargo",
            "run",
            "--manifest-path",
            str(SCHEMA_CHECK_MANIFEST),
            "--locked",
            "--",
            str(ROOT / "services" / "human" / "schema" / "human-api"),
        ),
        label="schema-check",
    )
    if "human-api schema conformance passed" not in schema.output:
        raise CaseFailure(f"schema check did not report conformance:\n{schema.tail()}")
    rejection = native_test(
        ctx, "native-ledger-boundary-rejections", "./node",
        "TestRepair_102_TenantReplay", "TestRepair_102_StrictResultCommitment",
        "TestRepair475_RejectCorruptedCommittedHash", "TestHaltedBlockLeavesCommittedStateAtPreviousHeight",
    )
    conformance = ctx.run(
        ("cargo", "test", "--manifest-path", str(SCHEMA_CHECK_MANIFEST), "--locked"),
        label="schema-conformance",
    )
    ctx.artifact_log("proof", schema)
    ctx.artifact_log("tamper-rejection", rejection)
    ctx.result(conformance_tests=conformance.cargo_tests())


@case("multichain-qualify", "reference-ramp/end-to-end")
def reference_ramp_end_to_end(ctx: CaseContext) -> None:
    ctx.require_env(*RAMP_INPUTS)
    journey = ctx.make("interop-test-ramps-sandbox")
    proof = ctx.cargo_test(
        PLATFORM_MANIFEST,
        "layerx-ramp-toolkit",
        "--test",
        "contracts",
        "--",
        "done_requires_both_verified_legs_and_external_label",
        label="ramp-done-contract",
    )
    rejection = ctx.cargo_test(
        PLATFORM_MANIFEST,
        "layerx-ramp-toolkit",
        "--test",
        "contracts",
        "--",
        "digest_binds_authenticated_customer_and_direction",
        label="ramp-digest-binding",
    )
    ctx.artifact_log("proof", proof)
    ctx.artifact_log("tamper-rejection", rejection)
    ctx.result(sandbox_tail=journey.tail())


@case("multichain-qualify", "reference-ramp/labelling")
def reference_ramp_labelling(ctx: CaseContext) -> None:
    ctx.require_env(*RAMP_INPUTS)
    toolkit = ctx.make("interop-test-ramps")
    proof = ctx.cargo_test(
        PLATFORM_MANIFEST,
        "layerx-ramp-toolkit",
        "--test",
        "contracts",
        "--",
        "done_requires_both_verified_legs_and_external_label",
        "payment_direction_selects_direct_send_or_customer_grant",
        label="ramp-label-contract",
    )
    rejection = ctx.run(
        (
            "cargo",
            "test",
            "--manifest-path",
            str(SCHEMA_CHECK_MANIFEST),
            "--locked",
            "--",
            "retired_movement_failure_vectors_answer_operation_retired",
            "retired_binding_failure_vectors_answer_operation_retired",
        ),
        label="custody-label-rejections",
    )
    if not {
        "retired_movement_failure_vectors_answer_operation_retired",
        "retired_binding_failure_vectors_answer_operation_retired",
    } <= set(rejection.cargo_tests()):
        raise CaseFailure(f"retired movement rejection did not execute:\n{rejection.tail()}")
    ctx.artifact_log("proof", proof)
    ctx.artifact_log("tamper-rejection", rejection)
    ctx.result(toolkit_tests=toolkit.cargo_tests())


def run_case(
    gate: str,
    case_id: str,
    output: Path,
    stack: Stack,
    source_identity: str,
    environment: Mapping[str, str],
) -> dict[str, object]:
    ctx = CaseContext(gate, case_id, output, stack, source_identity, environment)
    status = "passed"
    reason = None
    try:
        CASES[gate][case_id](ctx)
        ctx.finish(EVIDENCE_SPECS[gate].artifact_kinds)
    except CaseFailure as failure:
        status, reason = "failed", str(failure)
    except Exception as error:
        status, reason = "failed", f"{type(error).__name__}: {error}"
    elapsed = ctx.elapsed()
    if status == "passed":
        print(f"case {case_id}: passed ({elapsed:.1f}s)", flush=True)
    else:
        print(f"case {case_id}: failed: {reason}", flush=True)
    return {
        "id": case_id,
        "status": status,
        "reason": reason,
        "started_at": ctx.started_at,
        "finished_at": utc_now(),
        "elapsed_seconds": round(elapsed, 3),
        "artifacts": ctx.artifacts if status == "passed" else [],
        "executions": ctx.execution_records(),
        "scope": "paxd-committed" if status == "passed" and ctx.paxd_evidence else "local-suite",
        "real_stack": status == "passed" and bool(ctx.paxd_evidence),
        "paxd_evidence": list(ctx.paxd_evidence) if status == "passed" else [],
    }


def parse_arguments(arguments: Sequence[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(prog="beta_driver")
    commands = parser.add_subparsers(dest="command", required=True)
    run = commands.add_parser("run")
    run.add_argument("--gate", required=True, choices=DRIVER_GATES)
    run.add_argument("--output", required=True)
    run.add_argument("--source-identity", required=True)
    for _, flag in STACK_FLAGS:
        run.add_argument(flag, required=True)
    return parser.parse_args(arguments)


def qualification_manifest(
    gate: str, source_identity: str, stack: Stack, started_at: str,
    records: Sequence[dict[str, object]], agent_identity: dict[str, object],
) -> dict[str, object]:
    def live(record: dict[str, object]) -> bool:
        proofs = record.get("paxd_evidence")
        return (
            record.get("status") == "passed" and record.get("real_stack") is True
            and record.get("scope") == "paxd-committed" and isinstance(proofs, list) and bool(proofs)
            and all(
                isinstance(proof, dict) and proof.get("case") == record["id"]
                and proof.get("endpoint") == stack.node and proof.get("performed") is True
                and proof.get("verified") is True
                and type(proof.get("height")) is int and proof["height"] > 0
                and all(
                    isinstance(proof.get(name), str) and re.fullmatch(r"0x[0-9a-fA-F]{64}", proof[name])
                    for name in ("transaction_hash", "block_hash")
                )
                and all(
                    isinstance(proof.get(name), str) and re.fullmatch(r"[0-9a-f]{64}", proof[name])
                    for name in ("receipt_sha256", "trust_sha256", "native_transaction_sha256",
                                 "commit_response_sha256", "block_response_sha256", "transaction_response_sha256")
                )
                for proof in proofs
            )
        )
    real_stack = bool(records) and all(live(record) for record in records)
    return {
        "schema": SCHEMA, "gate": gate, "real_stack": real_stack,
        "scope": "paxd-committed" if real_stack else "local-suites-or-partial-evidence",
        "agent_workspace": agent_identity, "source_identity": source_identity,
        "driver_sha256": file_digest(Path(__file__).resolve()), "components": stack.components(),
        "started_at": started_at, "finished_at": utc_now(), "cases": list(records),
    }


def beta_driver(arguments: Sequence[str], environment: Mapping[str, str] | None = None) -> int:
    parsed = parse_arguments(arguments)
    environment = dict(os.environ if environment is None else environment)
    try:
        stack = Stack(
            *(
                validated_url(getattr(parsed, flag[2:].replace("-", "_")), flag)
                for _, flag in STACK_FLAGS
            )
        )
    except QualificationFailure as failure:
        print(f"beta_driver: {failure}", file=sys.stderr)
        return 2
    try:
        agent_identity = agent_workspace_identity()
    except CaseFailure as failure:
        print(f"beta_driver: {failure}", file=sys.stderr)
        return 2
    gate = parsed.gate
    spec = EVIDENCE_SPECS[gate]
    registered = set(CASES[gate])
    if registered != spec.cases:
        print(
            f"beta_driver: case inventory for {gate} drifted; missing={sorted(spec.cases - registered)!r}"
            f" unexpected={sorted(registered - spec.cases)!r}",
            file=sys.stderr,
        )
        return 2
    output = Path(parsed.output).resolve()
    if output.exists() and not output.is_dir():
        print(f"beta_driver: output {output} is not a directory", file=sys.stderr)
        return 2
    output.mkdir(parents=True, exist_ok=True)
    if (output / "cases").exists():
        print(f"beta_driver: output {output} already holds case evidence", file=sys.stderr)
        return 2
    started_at = utc_now()
    print(f"beta_driver: {gate} against {json.dumps(stack.components())}", flush=True)
    records = [
        run_case(gate, case_id, output, stack, parsed.source_identity, environment)
        for case_id in sorted(spec.cases)
    ]
    failed = [record for record in records if record["status"] != "passed"]
    manifest = qualification_manifest(gate, parsed.source_identity, stack, started_at, records, agent_identity)
    real_stack = manifest["real_stack"]
    atomic_json(output / "evidence.json", manifest)
    if not real_stack:
        print("beta_driver: real-stack qualification refused: every case needs its own live paxd committed evidence", file=sys.stderr)
    print(
        f"beta_driver: {gate} {len(records) - len(failed)}/{len(records)} cases passed;"
        f" evidence at {output / 'evidence.json'}",
        flush=True,
    )
    return 1 if failed or not real_stack else 0


if __name__ == "__main__":
    sys.exit(beta_driver(sys.argv[1:]))
