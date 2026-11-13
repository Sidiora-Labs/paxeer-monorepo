"""Offline regression proof for audit 22.30.145's root README finding.

Read the maintained README itself, including its client commands and diagrams.
These checks qualify documentation; they do not execute the node or contact RPC.
"""

import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
NETWORK_PATH = ROOT / "docs/readme/NETWORK.md"
README = NETWORK_PATH.read_text(encoding="utf-8")
RECORD = (
    ROOT / "spec/layerx-merge/programme/records/tools/qualification/audit_repairs"
    / "repair_22_33_425.kvx"
)


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


def test_root_readme_separate_kernel_integrated_network():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
    ):
        assert fact in overview, fact
    for path in (
        "daemon/paxd/", "node/", "consensus/", "modules/", "rpc/",
        "modules/lxphost/", "programs/", "node/sequencer/",
    ):
        assert f"](../../{path})" in overview, path
        assert (NETWORK_PATH.parent / "../.." / path).is_dir(), path
    assert "C17 kernel in `src/` and `include/` runs inside `paxd`" in overview
    assert "The node links `build/liblayerx.a`" in overview


def test_root_readme_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Set `PAXEER_RPC_URL` to a node root URL",
        "Send HTTPS JSON-RPC requests to the root path without a route suffix",
        "Confirm the chain ID before signing or broadcasting",
        "compare `eth_blockNumber`",
        "gateway contract does not establish method availability at public node URLs",
    ):
        assert fact in quickstart, fact
    links = dict(re.findall(r"\[([^]]+)\]\(([^)]+)\)", quickstart))
    reference_path = NETWORK_PATH.parent / links["public RPC reference"]
    assert reference_path.resolve() == (
        ROOT / "docs/site/paxeer-docs/src/app/public-rpc/page.tsx"
    ).resolve()
    reference = reference_path.read_text(encoding="utf-8")
    assert "HTTP JSON-RPC requests use the endpoint root" in reference
    assert "125 / <code>0x7d</code>" in reference
    public_rpc = (ROOT / "docs/readme/PUBLIC-RPC.md").read_text(encoding="utf-8")
    for fact in (
        "Paxeer X Network serves JSON-RPC at the root path of its public node names",
        "Use `api1` through `api14.mainnet-beta.paxeer.network` or",
        "`api-mainnet-beta.paxeer.network`, all at the root path",
        "`node.hyperpaxeer.com` is only the hpx library mirror, never a JSON-RPC endpoint",
    ):
        assert fact in public_rpc, fact
    hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = [
        urlsplit(url)
        for url in re.findall(r"https?://[^\s`<>)]+", public_rpc)
    ]
    assert {url.hostname for url in urls} == hosts
    for url in urls:
        assert url.scheme == "https" and url.hostname in hosts
        assert url.path in ("", "/")
        assert not (url.query or url.fragment or url.username or url.password)
        assert url.port is None
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert argv[argv.index("-sS") + 1] == "$PAXEER_RPC_URL"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]


def test_root_readme_transaction_to_committed_receipt():
    flow = section("Submit a transaction and follow its committed receipt")
    steps = re.findall(r"^\d+\. (.+)$", flow, re.M)
    assert len(steps) == 4
    for fact in (
        "eth_getTransactionCount", "secp256k1 EIP-1559",
        "chain ID `125`", "Programs payload bytes go to `0x101D`",
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
        "Wait for `committed`", "authenticate the block commitment",
        "eth_getTransactionReceipt", "commitment does not mean execution succeeded",
        "status and logs", "`eth_call` at an explicit committed height",
    ):
        assert fact in steps[3], fact
    proof = section("Receipt and proof verification")
    assert "public key supplied beside an untrusted receipt does not establish trust" in proof
    assert "authenticate the block header and CometBFT commit" in proof
    assert "trusted validator information" in proof
    assert "Neither a transaction hash nor a signed execution receipt alone" in proof


def test_root_readme_diagrams_preserve_execution_and_commit_order():
    boundary = (ROOT / "docs/wiki/PaxeerBoundary.md").read_text(encoding="utf-8")
    assert (
        "There is no separate LayerX interface, router, or sequencer service to deploy"
    ) in boundary
    topology = section("Integrated network topology")
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native", "Validators -->|Commit| State",
        "outside consensus",
    ):
        assert fact in topology, fact
    flow = section("Submit a transaction and follow its committed receipt")
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(diagrams) == 1
    events = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    )
    positions = [diagrams[0].index(event) for event in events]
    assert positions == sorted(positions)


def test_root_readme_retired_custody_checkpoint_and_archive_behavior():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "retirement errors (including `LXP_MODULE_DISABLED`)",
        "Historical anchor reads before store deletion are archive-only",
        "do not provide current funding or finality paths",
        "retained [batch-header codec]", "archived settlement witness formats",
        "guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
        "external native bridge at `0x1016` remains distinct from retired kernel custody",
    ):
        assert fact in native, fact
    accounts = section("Accounts and native funding")
    assert "custody deposit and credit-attestation funding flow is retired" in accounts
    assert "Wait for commit and read `balanceOf` to confirm funding" in accounts


def test_root_readme_build_and_run_only_integrated_node():
    build = section("Build")
    commands = "\n".join(re.findall(r"```sh\n(.*?)\n```", build, re.S))
    assert re.search(r"^make paxd-build\s*(?:#.*)?$", commands, re.M)
    assert './build/paxd start --home "$PAXD_HOME"' in commands
    assert "./build/paxd version" in commands
    for fact in (
        "`build/liblayerx.a`", "`programs/target/release/liblayerx_programs_sandbox.a`",
        "`chain.mk`", "operator-provisioned, initialized node home",
        "network genesis and configuration", "EVM chain ID `125` is the signing domain",
        "The running network entry point is `paxd`",
    ):
        assert fact in build, fact
    assert not re.search(r"\blayerxd\b", README, re.I)
    for block in re.findall(r"```(?:sh|bash|shell)\n(.*?)\n```", README, re.S):
        assert not re.search(r"\b(?:checkpoint|layerxanchor|layerxcustody)\b", block, re.I)
    assert "two execution domains" not in README.lower()


def test_root_readme_repair_record_maps_finding_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["425"]
    assert task["audit"] == "22.30.145"
    assert task["readme_owner"] == "22.31.470"
    assert task["missing_facts"] == []
    assert "README.md" in task["wiring_request"]
    assert "domain 17" in task["wiring_request"]
    assert "22.31.470" in task["wiring_request"]
    finding = record["finding"]["root-readme-separate-kernel"]
    assert finding["file"] == "README.md"
    tests = finding["tests"]
    assert len(tests) == 6 and len(set(tests)) == 6
    for name in tests:
        assert name.startswith("test_root_readme_")
        assert callable(globals()[name]), name
    assert "hf-done" in task["evidence"]
    assert "revision, command, exit and log" in task["evidence"]
