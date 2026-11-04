"""Offline source-fact proof for audit 22.30.118/doc-01, owned by 22.31.470.

Read the actual README consumed by developers; do not execute its examples or
claim native execution, endpoint availability, recovery or consensus coverage.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_178.kvx")


def section(heading, level=3):
    marker = "#" * level + " " + heading + "\n"
    assert README.count(marker) == 1, heading
    lines = []
    fence = None
    for line in README.split(marker, 1)[1].splitlines(keepends=True):
        delimiter = re.match(r"^ {0,3}(`{3,}|~{3,})(.*)$", line)
        if delimiter:
            ticks, suffix = delimiter.groups()
            if fence is None:
                fence = ticks
            elif ticks[0] == fence[0] and len(ticks) >= len(fence) and not suffix.strip():
                fence = None
        elif fence is None and re.match(r"^#{1," + str(level) + r"} ", line):
            break
        lines.append(line)
    return "".join(lines)


def test_doc_01_integrated_paxd_network_and_block_finality():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "C17 kernel",
        "runs inside `paxd`",
        "modules/lxphost",
        "A receipt is final once its block is committed",
        "signed execution receipt can arrive before that commitment",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "no router and no separate LayerX interface",
        "outside consensus",
    ):
        assert fact in overview, fact
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(diagrams) == 1
    topology = diagrams[0]
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State[Committed block and chain state]",
    ):
        assert fact in topology, fact


def test_doc_01_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert (
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`"
    ) in quickstart
    urls = re.findall(r"https://[^\s`]+", quickstart)
    assert {urlsplit(url).hostname for url in urls} == {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    for url in urls:
        parsed = urlsplit(url)
        assert parsed.scheme == "https"
        assert parsed.path in {"", "/"}
        assert not parsed.query and not parsed.fragment
        assert parsed.username is None and parsed.password is None
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert "https://api1.mainnet-beta.paxeer.network" in argv
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "eth_chainId",
        "params": [],
    }
    assert '# Expected result: "0x7d"' in blocks[0]


def test_doc_01_transaction_transport_and_committed_receipt_flow():
    flow = section("Submit a transaction and follow its committed receipt")
    steps = re.findall(r"^\d+\. (.+)$", flow, re.M)
    assert len(steps) == 4
    for step, facts in zip(
        steps,
        (
            ("eth_chainId", "0x7d", "eth_getTransactionCount", "chain ID `125`"),
            (
                "eth_sendRawTransaction",
                "root JSON-RPC URL",
                "transaction hash",
                "integrated sequencing path",
                "durable WAL acknowledgement",
            ),
            (
                "pax_sequencerReceipt",
                "transaction hash",
                "`submitted`",
                "`receipted`",
                "signed execution evidence",
                "`committed`",
                "MsgSequenceBatch",
                "No separate service endpoint is required",
            ),
            (
                "Wait for `committed`",
                "authenticate the block commitment",
                "eth_getTransactionReceipt",
                "status and logs",
                "commitment does not mean execution succeeded",
                "eth_call",
                "explicit committed height",
            ),
        ),
        strict=True,
    ):
        for fact in facts:
            assert fact in step, fact
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    ordered = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    )
    positions = [sequence[0].index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_doc_01_retired_executors_and_historical_background():
    native = section("EVM and Native Precompiles")
    for fact in (
        "pre-merge `layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "Historical anchor reads before store deletion are archive-only",
        "do not provide current funding or finality paths",
        "archived settlement witness formats",
        "support historical interpretation",
        "External checkpoint registration, guarantor attestations",
        "L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    funding = section("Accounts and native funding")
    assert "custody deposit and credit-attestation funding flow is retired" in funding
    assert not re.search(r"\blayerxd\b|\bmake\s+layerxd\b", README, re.I)
    assert not re.search(r"https://[^\s`]+/(?:rpc|evm)(?:/|\b)", README)


def test_doc_01_build_and_runtime_examples_use_integrated_paxd():
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    assert len(blocks) == 4
    commands = [
        shlex.split(line, comments=True)
        for block in blocks
        for line in block.splitlines()
        if line.strip()
    ]
    assert commands[0] == ["make", "paxd-build"]
    assert ["./build/paxd", "version"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands
    for fact in (
        "Build the merged node from the repository root",
        "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "before invoking `chain.mk`",
        "operator-provisioned, initialized node home",
        "operator-provided genesis and consensus chain identifier",
        "EVM chain ID `125` is the signing domain",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact


def test_doc_01_repair_record_maps_finding_to_executable_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["doc-01"]
    assert finding["audit"] == "22.30.118"
    assert finding["path"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    assert finding["owner_domain"] == 17
    assert finding["wiring_requests"] == []
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_doc_01_repair_record_maps_finding_to_executable_proofs"
    }
    task = record["task"]["22"]["33"]["178"]
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_178.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "revision" in task["verified_revision"]
    assert "not executed" in task["review_only"]
    assert task["baseline_tests_run"] is False
    assert task["baseline_failed_tests"] == "Unknown; no pre-change verification run."
