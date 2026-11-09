"""Offline contract for audit 22.30.97/doc_root against the real root README."""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_1555.kvx")


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
    assert fence is None, f"Unclosed code fence in {heading}"
    return "".join(lines)


def blocks(text, language):
    return re.findall(r"```" + re.escape(language) + r"\n(.*?)\n```", text, re.S)


def test_doc_root_merged_network_and_embedded_proposer():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network",
        "EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "no router and no separate LayerX interface",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
    ):
        assert fact in overview, fact
    topology, = blocks(section("Integrated network topology"), "mermaid")
    assert "subgraph Network[One Paxeer BFT network / EVM chain 125]" in topology
    assert "subgraph Node[paxd]" in topology
    assert "RPC --> Order --> Host --> Native" in topology
    assert "Validators -->|Commit| State[Committed block and chain state]" in topology


def test_doc_root_public_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert (
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`"
    ) in quickstart
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    assert "compare `eth_blockNumber`" in quickstart
    assert "gateway contract does not establish method availability at public node URLs" in quickstart
    seen = set()
    allowed = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.hostname in allowed, url
            assert parsed.scheme == "https" and parsed.path in ("", "/"), url
            assert parsed.port is None and parsed.username is None, url
            assert not parsed.query and not parsed.fragment, url
            seen.add(parsed.hostname)
    assert seen == {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api2.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    example, = blocks(quickstart, "sh")
    argv = shlex.split(example.replace("\\\n", " "), comments=True)
    assert argv[:4] == ["curl", "--max-time", "10", "-sS"]
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in example


def test_doc_root_transaction_transport_and_committed_receipt():
    flow = section("Submit a transaction and follow its committed receipt")
    steps = re.findall(r"^\d+\. (.*)$", flow, re.M)
    assert len(steps) == 4
    for step, facts in zip(steps, (
        ("eth_chainId", "0x7d", "eth_getTransactionCount", "EIP-1559", "chain ID `125`"),
        ("eth_sendRawTransaction", "root JSON-RPC URL", "transaction hash", "durable WAL acknowledgement"),
        ("pax_sequencerReceipt", "`submitted`", "`receipted`", "`committed`", "MsgSequenceBatch"),
        ("Wait for `committed`", "authenticate the block commitment", "eth_getTransactionReceipt",
         "status and logs", "commitment does not mean execution succeeded"),
    )):
        for fact in facts:
            assert fact in step, fact
    sequence, = blocks(flow, "mermaid")
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
    positions = [sequence.index(fact) for fact in ordered]
    assert positions == sorted(positions)
    proof = section("Receipt and proof verification")
    assert "Neither a transaction hash nor a signed execution receipt alone establishes block commitment" in proof
    assert "authenticate the block header and CometBFT commit against trusted validator information" in proof
    assert "trusted" in proof and "MsgSequenceBatch" in proof


def test_doc_root_custody_and_checkpoint_executors_are_retired():
    interfaces = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "LXP_MODULE_DISABLED",
        "Historical anchor reads before store deletion are archive-only",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in interfaces, fact
    accounts = section("Accounts and native funding")
    assert "custody deposit and credit-attestation funding flow is retired" in accounts
    assert "`getLayerXDid(address)`" in accounts
    # Match the retired daemon name, including paths/commands, without treating
    # the current getLayerXDid ABI identifier as that daemon.
    assert re.search(r"\blayerxd\b", README, re.I) is None
    assert not re.search(r"\bmake\s+[^\n#]*\b(?:checkpoint|settlement)[-_]deploy\b", README)


def test_doc_root_build_operations_and_supported_layout():
    build = section("Build")
    shell = blocks(build, "sh")
    assert len(shell) == 4
    commands = [
        shlex.split(line, comments=True)
        for block in shell for line in block.splitlines() if line.strip()
    ]
    assert ["make", "paxd-build"] in commands
    assert ["./build/paxd", "version"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands
    assert "operator-provisioned, initialized node home" in build
    assert "network genesis and configuration" in build
    assert "`paxd-build` builds `build/liblayerx.a`" in build
    assert "programs/target/release/liblayerx_programs_sandbox.a" in build
    assert "The running network entry point is `paxd`" in build
    assert "without rebuilding the node" in build
    layout = section("Directory Structure", 2)
    for path in ("apps/", "services/", "sdk/", "contracts/", "core/deps/", "tools/", "spec/"):
        assert f"| `{path}` |" in layout, path
        assert (ROOT / path).is_dir(), path
    overview = section("What is Paxeer X Network?", 2)
    for path in ("daemon/paxd/", "node/", "consensus/", "modules/", "rpc/", "modules/lxphost/", "programs/"):
        assert f"]({path})" in overview, path
        assert (ROOT / path).is_dir(), path
    assert "Source directory names describe code organization, not separate network deployments" in layout


def test_doc_root_repair_record_maps_the_real_finding_to_declared_tests():
    audit = tomllib.loads((ROOT / "spec/layerx-merge/programme/audit/records/22.30.97.kvx").read_text())
    finding = audit["finding"]["doc_root"]
    assert (finding["file"], finding["line"], finding["class"], finding["severity"], finding["owner_domain"]) == (
        "README.md", 47, "doc-drift", "medium", 17
    )
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    coverage = record["finding"]["doc_root"]
    assert coverage["audit"] == "22.30.97"
    assert coverage["file"] == finding["file"]
    assert coverage["owner_task"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_doc_root_")
    }
    assert set(coverage["tests"]) == declared - {
        "test_doc_root_repair_record_maps_the_real_finding_to_declared_tests"
    }
    assert record["task"]["id"] == "22.33.1555"
    assert record["task"]["wiring_requests"] == []
    assert record["qualification"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1555.py"
    )
