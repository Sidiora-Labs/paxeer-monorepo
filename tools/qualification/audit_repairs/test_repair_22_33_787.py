"""Offline checks of the production README for audit 22.30.175.

These checks qualify documentation facts, not node execution or live RPCs.
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
RECORD_PATH = Path(__file__).with_name("repair_22_33_787.kvx")


def section(heading):
    marker = re.search(r"^(#{2,3}) " + re.escape(heading) + r"$", README, re.M)
    assert marker is not None, heading
    lines = []
    fence = None
    for line in README[marker.end():].splitlines(keepends=True):
        delimiter = re.match(r"^ {0,3}(`{3,}|~{3,})(.*)$", line)
        if delimiter:
            ticks, suffix = delimiter.groups()
            if fence is None:
                fence = ticks
            elif ticks[0] == fence[0] and len(ticks) >= len(fence) and not suffix.strip():
                fence = None
        elif fence is None and re.match(r"^#{1," + str(len(marker[1])) + r"} ", line):
            break
        lines.append(line)
    return "".join(lines)


def test_readme_premerge_network_embedded_paxd_and_committed_finality():
    overview = section("What is Paxeer X Network?")
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "LayerX kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "runs inside `paxd` through [`modules/lxphost`]",
        "A receipt is final once its block is committed",
        "signed execution receipt can arrive before that commitment",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
    ):
        assert fact in overview, fact
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release
    for obsolete in (
        r"\blayerxd\b", r"two execution domains", r"merge is (?:only )?a draft",
        r"kernel still runs separately", r"make\s+layerxd",
    ):
        assert not re.search(obsolete, README, re.I), obsolete


def test_readme_premerge_network_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "https://api1.mainnet-beta.paxeer.network",
        "through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    assert "there is no router and no separate LayerX interface" in README
    urls = [
        urlsplit(url)
        for url in re.findall(r"https?://[^\s`<>)]+", README)
        if urlsplit(url).hostname.endswith("mainnet-beta.paxeer.network")
    ]
    assert urls
    allowed = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    for url in urls:
        assert url.hostname in allowed
        assert url.scheme == "https" and url.path in ("", "/")
        assert not url.query and not url.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(
        blocks[0].split("\n# Expected result:")[0].replace("\\\n", " ")
    )
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert "https://api1.mainnet-beta.paxeer.network" in argv
    assert '# Expected result: "0x7d"' in blocks[0]


def test_readme_premerge_network_transaction_to_committed_receipt():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`eth_getTransactionCount`",
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_sendRawTransaction` to a public node's root JSON-RPC URL",
        "result is a transaction hash",
        "integrated sequencing path or forwards to the configured proposing node",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block",
        "`MsgSequenceBatch` covers that sequence",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt` for the committed EVM result",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
        "No separate service endpoint is required",
    ):
        assert fact in flow, fact
    diagram = re.search(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert diagram is not None
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
    positions = [diagram[1].index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_readme_premerge_network_historical_executors_refuse():
    native = section("EVM and Native Precompiles")
    for fact in (
        "pre-merge `layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "retirement errors (including `LXP_MODULE_DISABLED`)",
        "Historical anchor reads before store deletion are archive-only",
        "do not provide current funding or finality paths",
        "archived settlement witness formats",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "Constructor and ABI inventories include historical versions" in native
    assert "Source directory names describe code organization, not separate network deployments" in README


def test_readme_premerge_network_native_balance_and_proof_replacements():
    accounts = section("Accounts and native funding")
    for fact in (
        "Native sub-accounts map to derived bank addresses inside the node",
        "`0x101A`", "`openAccount(bytes32)`", "`mint(bytes32,bytes32,uint128)`",
        "`payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)`",
        "`balanceOf(bytes32,bytes32)`", "authorized issuer mint",
        "Wait for commit and read `balanceOf` to confirm funding",
        'eth_getBalance(address,"latest")',
        "custody deposit and credit-attestation funding flow is retired",
    ):
        assert fact in accounts, fact
    proof = section("Receipt and proof verification")
    for fact in (
        "ProtocolReceipt v2", "pinned for the receipt height",
        "a public key supplied beside an untrusted receipt does not establish trust",
        "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit against trusted validator information",
        "verify transaction or state inclusion against the authenticated root",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ):
        assert fact in proof, fact


def test_readme_premerge_network_build_and_run_merged_node():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "EVM chain ID `125` is the signing domain",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    commands = [
        shlex.split(line, comments=True)
        for block in blocks for line in block.splitlines() if line.strip()
    ]
    assert ["make", "paxd-build"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands


def test_repair_record_maps_readme_premerge_network_to_declared_proofs():
    record = tomllib.loads(RECORD_PATH.read_text(encoding="utf-8"))
    finding = record["finding"]["readme-premerge-network"]
    assert finding["audit"] == "22.30.175"
    assert finding["file"] == "README.md"
    assert finding["owner_domain"] == 17
    assert finding["implementation_task"] == "22.31.470"
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef)
        and node.name.startswith("test_readme_premerge_network_")
    }
    assert len(declared) == 6
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["787"]
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_787.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert task["wiring_requests"] == []
