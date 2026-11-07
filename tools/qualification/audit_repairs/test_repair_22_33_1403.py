"""Qualify root-doc-two-domains against the inherited production README offline."""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_1403.kvx")


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


def test_root_doc_two_domains_integrated_network_and_finality():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "runs inside `paxd` through",
        "modules/lxphost",
        "A receipt is final once its block is committed",
        "A signed execution receipt can arrive before that commitment",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
        "no router and no separate LayerX interface",
        "outside consensus and does not provide another execution or finality domain",
    ):
        assert fact in overview, fact
    assert not re.search(r"\blayerxd\b", README, re.I)


def test_root_doc_two_domains_root_rpc_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert (
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`"
    ) in quickstart
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    public_urls = [
        urlsplit(url)
        for url in urls
        if urlsplit(url).hostname
        and urlsplit(url).hostname.endswith("mainnet-beta.paxeer.network")
    ]
    assert {url.hostname for url in public_urls} >= {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    for url in public_urls:
        assert url.scheme == "https" and url.hostname in allowed_hosts
        assert url.path in ("", "/") and not url.query and not url.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    assert "gateway contract does not establish method availability at public node URLs" in quickstart


def test_root_doc_two_domains_transaction_and_committed_receipt_flow():
    flow = section("Submit a transaction and follow its committed receipt")
    steps = re.findall(r"^\d+\. (.+)$", flow, re.M)
    assert len(steps) == 4
    for fact in (
        "eth_chainId", "eth_getTransactionCount",
        "secp256k1 EIP-1559 transaction for chain ID `125`", "`0x101A`–`0x101D`",
    ):
        assert fact in steps[0], fact
    for fact in (
        "eth_sendRawTransaction", "root JSON-RPC URL", "transaction hash",
        "integrated sequencing path", "durable WAL acknowledgement",
        "Receipt order is proposal order",
    ):
        assert fact in steps[1], fact
    for fact in (
        "pax_sequencerReceipt", "transaction hash",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "MsgSequenceBatch",
        "No separate service endpoint is required",
    ):
        assert fact in steps[2], fact
    for fact in (
        "Wait for `committed` and authenticate the block commitment",
        "eth_getTransactionReceipt", "status and logs",
        "commitment does not mean execution succeeded",
        "eth_call` at an explicit committed height",
    ):
        assert fact in steps[3], fact
    proof = section("Receipt and proof verification")
    assert "must be pinned for the receipt height" in proof
    assert "Neither a transaction hash nor a signed execution receipt alone establishes block commitment" in proof
    assert "authenticate the block header and CometBFT commit" in proof


def test_root_doc_two_domains_diagrams_show_integrated_execution_and_commit():
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert len(diagrams) == 2
    topology, sequence = diagrams
    stack = []
    for line in topology.splitlines():
        line = line.strip()
        if line.startswith("subgraph "):
            stack.append(line)
        elif line == "end":
            assert stack, "Unmatched topology subgraph end"
            stack.pop()
        elif line.startswith(("RPC[", "Order[", "Host[", "Native[")):
            assert stack == [
                "subgraph Network[One Paxeer BFT network / EVM chain 125]",
                "subgraph Node[paxd]",
            ], line
    assert not stack
    for fact in (
        "RPC[Node JSON-RPC]", "Order[Integrated sequencing and block proposal]",
        "Host[Embedded C17 kernel and Rust programs host]",
        "Native[Native bank, ledger, payments, authority and programs]",
        "RPC --> Order --> Host --> Native",
        "Order -->|Proposed block in execution order| Validators[BFT validators]",
        "Validators -->|Commit| State[Committed block and chain state]",
        "State -->|Transaction receipts, logs and proofs| Client",
    ):
        assert fact in topology, fact
    ordered = (
        "Client->>Node: eth_sendRawTransaction(signed chain-125 bytes)",
        "Node->>Kernel: Accept or forward transaction",
        "Kernel->>Kernel: Execute, sign receipt, durably acknowledge WAL",
        "Kernel-->>Node: Release signed execution receipt",
        "Node-->>Client: receipted (execution evidence)",
        "Kernel->>BFT: Propose MsgSequenceBatch in receipt order",
        "BFT->>BFT: Commit block",
        "Node-->>Client: committed, blockNumber, blockHash",
        "Node-->>Client: Committed status, logs and native state",
    )
    assert "participant Kernel as Integrated paxd proposer and kernel" in sequence
    positions = [sequence.index(fact) for fact in ordered]
    assert positions == sorted(positions)
    assert not re.search(r"layerxd|custody|checkpoint|settlement", "\n".join(diagrams), re.I)


def test_root_doc_two_domains_current_constructs_and_retired_executors():
    interfaces = section("EVM and Native Precompiles")
    table = dict(re.findall(r"^\| `(0x[0-9A-F]+)` \| `([^`]+)` \|$", interfaces, re.M))
    assert table == {
        "0x101A": "layerxledger", "0x101B": "layerxpay",
        "0x101C": "layerxauth", "0x101D": "layerxprograms",
        "0x101E": "layerxchains", "0x0901": "orob", "0x0902": "clearing",
        "0x0903": "oracle aggregator", "0x0904": "pofq", "0x0905": "scheduler",
        "0x0906": "streams", "0x0907": "tee attestor", "0x0908": "eip712",
    }
    retirement = next(p for p in interfaces.split("\n\n") if p.startswith("The pre-merge "))
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge", "`layerxAnchor` (`0x1014`) rejects writes",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "LXP_MODULE_DISABLED",
        "Historical anchor reads before store deletion are archive-only",
        "do not provide current funding or finality paths",
    ):
        assert fact in retirement, fact
    assert "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired" in interfaces
    assert "archived formats add no settlement step to current block finality" in interfaces


def test_root_doc_two_domains_record_maps_finding_to_executing_tests():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["root-doc-two-domains"]
    assert finding["audit"] == "22.30.78"
    assert finding["file"] == "README.md" and finding["owner_domain"] == 17
    assert finding["owning_task"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_root_doc_two_domains_record_maps_finding_to_executing_tests"
    }
    assert finding["wiring_requests"] == []
    task = record["task"]["22"]["33"]["1403"]
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1403.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
