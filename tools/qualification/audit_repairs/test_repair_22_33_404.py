"""Offline source proof for audit 22.30.142/46 against the production README.

The declared gate checks documentation and its source connections, not native
execution, endpoint availability, consensus, or restart/recovery behavior.
"""

import ast
import hashlib
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README_PATH = ROOT / "README.md"
README = README_PATH.read_text(encoding="utf-8")
RECORD_PATH = Path(__file__).with_name("repair_22_33_404.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Select one Markdown section, preserving fenced examples and diagrams."""
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
    assert fence is None, heading
    return "".join(lines)


def test_finding_46_one_paxd_network_and_embedded_kernel():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "C17 kernel in `src/` and `include/` runs inside `paxd`",
        "modules/lxphost",
        "no router and no separate LayerX interface",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    topology = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(topology) == 1
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State[Committed block and chain state]",
    ):
        assert fact in topology[0], fact


def test_finding_46_root_rpc_hosts_and_hosted_contract():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "https://api-mainnet-beta.paxeer.network",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "gateway contract does not establish method availability at public node URLs",
        "`px_getReceiptKeys` is registered by the node",
        "native `px_` account/history and receipt-key methods",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    public_urls = [urlsplit(url) for url in urls if urlsplit(url).hostname in hosts]
    assert {url.hostname for url in public_urls} == {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    assert all(
        url.scheme == "https" and url.path in ("", "/")
        and not url.query and not url.fragment
        for url in public_urls
    )
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")
    methods = {
        item["name"] for item in json.loads(
            source("services/platform/hosted/gateway/openrpc.json")
        )["methods"]
    }
    assert {"px_getBalances", "px_getReceiptKeys"} <= methods
    assert '{Namespace: "px", Service: pxAPI}' in source("rpc/server.go")


def test_finding_46_transaction_transport_and_committed_receipt_flow():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`",
        "`eth_sendRawTransaction` to a public node's root JSON-RPC URL",
        "Its result is a transaction hash",
        "accepts on the node's integrated sequencing path or forwards",
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
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipts = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipts) == [
        "submitted", "receipted", "committed"
    ]
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipts
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipts
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipts
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(diagrams) == 1
    events = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    )
    positions = [diagrams[0].index(event) for event in events]
    assert positions == sorted(positions)


def test_finding_46_committed_proofs_are_distinct_from_archive_reads():
    assert "A receipt is final once its block is committed" in section(
        "What is Paxeer X Network?", 2
    )
    proof = section("Receipt and proof verification")
    for fact in (
        "ProtocolReceipt v2",
        "separate from the validator consensus key",
        "pinned for the receipt height",
        "public key supplied beside an untrusted receipt does not establish trust",
        "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit against trusted validator information",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ):
        assert fact in proof, fact
    native = section("EVM and Native Precompiles")
    for fact in (
        "Historical anchor reads before store deletion are archive-only",
        "they do not provide current funding or finality paths",
        "archived settlement witness formats",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact


def test_finding_46_retired_executors_and_no_standalone_deployment():
    overview = section("What is Paxeer X Network?", 2)
    assert "no separate kernel daemon, sequencer service or checkpoint deployment" in overview
    assert "External checkpoint settlement is retired" in overview
    historical = [p for p in README.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(historical) == 1
    paragraph = historical[0]
    for fact in (
        "`layerxVerify` (`0x1012`)",
        "`layerxCustody` (`0x1013`)",
        "retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
    ):
        assert fact in paragraph, fact
    current = README.replace(paragraph, "")
    assert not re.search(r"layerx(?:verify|custody|anchor)", current, re.I)
    assert not re.search(r"\blayerxd\b|\./build/layerxd|make\s+daemon-build", README, re.I)
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_finding_46_build_entry_is_the_integrated_paxd():
    build = section("Build")
    for fact in (
        "make paxd-build",
        "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json",
        "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    targets = dict(re.findall(r"^([A-Za-z0-9_-]+):([^\n]*)$", makefile, re.M))
    assert targets["paxd-build"].split() == ["paxd-native-libs"]
    assert targets["paxd-native-libs"].split() == ["$(LIBRARY)", "programs-release-build"]
    chain = source("chain.mk")
    target = re.search(r"^build:([^\n]*)\n(\t[^\n]+)", chain, re.M)
    assert target is not None
    assert target.group(1).split() == ["paxd-native-libs"]
    assert shlex.split(target.group(2))[-3:] == ["-o", "./build/paxd", "./daemon/paxd"]
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")


def test_finding_46_record_maps_proofs_and_discloses_limits():
    record = tomllib.loads(RECORD_PATH.read_text(encoding="utf-8"))
    repair = record["repair"]
    finding = record["finding"]["46"]
    audit = source("spec/layerx-merge/programme/audit/records/22.30.142.kvx")
    block = re.search(r"^\[finding\.46\]\n(.*?)(?=^\[|\Z)", audit, re.M | re.S)
    assert block is not None
    original = tomllib.loads(block.group(1))
    assert (original["file"], original["class"], original["severity"]) == (
        "README.md", "doc-drift", "medium"
    )
    assert repair["task"] == "22.33.404"
    assert repair["audit"] == "22.30.142"
    assert finding["file"] == original["file"]
    assert finding["expected"] == original["expected"]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_finding_46_record_maps_proofs_and_discloses_limits"
    }
    assert len(finding["tests"]) == 6
    assert repair["readme_source_revision"] == "fc6d0a5ac2222ecf28b4061a8d09e2e730e40bdc"
    assert repair["readme_sha256"] == hashlib.sha256(README_PATH.read_bytes()).hexdigest()
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_404.py"
    )
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    assert "No pre-change test was run" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    wiring = record["wiring"]["readme"]
    assert wiring["path"] == "README.md"
    assert wiring["owner_domain"] == 17
    assert wiring["owning_task"] == "22.31.470"
    assert wiring["status"] == "satisfied by prerequisite; no missing expected fact identified"
