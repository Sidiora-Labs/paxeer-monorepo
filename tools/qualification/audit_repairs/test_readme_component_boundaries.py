"""Offline doc-01 proof against the README and committed production sources.

These checks qualify documentation facts, not node execution or deployment.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
NETWORK_PAGE = ROOT / "docs/readme/NETWORK.md"
README = NETWORK_PAGE.read_text(encoding="utf-8")
RPC_REFERENCE = (ROOT / "docs/readme/PUBLIC-RPC.md").read_text(encoding="utf-8")
RECORD = (
    ROOT / "spec/layerx-merge/programme/records/tools/qualification/audit_repairs"
    / "repair_22_33_359.kvx"
)


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Read a Markdown section, ignoring heading-like lines in code fences."""
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


def test_doc_01_one_paxd_bft_network_embeds_kernel_execution():
    overview = section("What is Paxeer X Network?", 2)
    assert "There is no separate LayerX interface, router, or sequencer service to deploy." in source(
        "docs/wiki/PaxeerBoundary.md"
    )
    for fact in (
        "The LayerX kernel runs inside every `paxd` node.", "chain ID **125** (`0x7d`)",
        "It executes each transaction when it arrives, assigns the next position "
        "in one global sequence, returns a signed execution receipt and builds "
        "each block.",
        "Validators check that block and commit it.",
        "returns a signed execution receipt and builds each block.",
        "proposes blocks inside `paxd`", "validators commit those blocks through BFT consensus",
        "C17 kernel", "runs inside `paxd`", "modules/lxphost",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "The public API reaches the node directly at the root path on EVM chain ID 125",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    topology = section("Integrated network topology")
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native", "Validators -->|Commit| State",
        "outside consensus", "The public API reaches the node directly at the root path on EVM chain ID 125",
    ):
        assert fact in topology, fact

    assert "The embedded kernel and Go node share consensus and committed-state authority." in source(
        "docs/MONOREPO.md"
    )
    assert "Hosted services, agents and applications consume that network;" in source("docs/MONOREPO.md")


def test_doc_01_chain_125_root_rpc_hosts_and_request():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Send HTTPS JSON-RPC requests to the root path without a route suffix.",
        "Use `api1` through `api14.mainnet-beta.paxeer.network` or",
        "root path without a route suffix",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint", "compare `eth_blockNumber`",
    ):
        assert fact in quickstart + RPC_REFERENCE, fact
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = re.findall(r"https?://[^\s`<>)]+", RPC_REFERENCE)
    public_urls = [urlsplit(url) for url in urls if "mainnet-beta.paxeer.network" in url]
    assert public_urls
    for url in public_urls:
        assert url.scheme == "https" and url.hostname in allowed_hosts
        assert url.path in ("", "/") and not url.query and not url.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    command = blocks[0].split("\n# Expected result:", 1)[0].replace("\\\n", " ")
    argv = shlex.split(command)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert [arg for arg in argv if arg.startswith(("https://", "http://", "$"))] == [
        "$PAXEER_RPC_URL"
    ]
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_doc_01_transaction_receipt_path_requires_block_commit():
    overview = section("What is Paxeer X Network?", 2)
    assert "A receipt is final once its block is committed" in overview
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "accepts on the node's integrated sequencing path or forwards",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`pax_sequencerReceipt`", "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "No separate service endpoint is required",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    producer = source("node/sequencer/accepter.go")
    assert producer.index("a.rep.Release(context.Background(), &wal.Accept{") < producer.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    consumer = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in consumer
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', consumer) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in consumer
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in consumer
    diagrams = re.findall(r"```mermaid\n(sequenceDiagram.*?)\n```", flow, re.S)
    assert len(diagrams) == 1
    ordered = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    )
    positions = [diagrams[0].index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_doc_01_custody_and_checkpoint_executors_are_retired():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "support historical interpretation", "guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    for stale in (
        r"\blayerxd\b", r"two (?:execution )?domains", r"kernel still runs separately",
        r"merge is (?:only )?a draft", r"https?://[^\s`]+/rpc(?:[/\s`]|$)",
    ):
        assert not re.search(stale, README, re.I), stale


def test_doc_01_source_names_are_distinct_from_running_components():
    layout = section("Directory Structure", 2)
    assert "Source directory names describe code organization, not separate network deployments" in layout
    assert "`core/deps/` | Pinned snapshots of the upstream libraries the node builds from" in layout
    assert "the framework fork in `sdk`" in layout
    assert (ROOT / "core/deps").is_dir() and (ROOT / "sdk").is_dir()
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        './build/paxd start --home "$PAXD_HOME"', "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release


def test_doc_01_repair_record_maps_the_finding_to_declared_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["359"]
    finding = record["finding"]["doc-01"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.139.kvx"))
    original = audit["finding"]["doc-01"]
    assert finding["audit"] == "22.30.139"
    assert finding["file"] == original["file"] == "README.md"
    assert finding["class"] == original["class"] == "doc-drift"
    assert finding["severity"] == original["severity"] == "medium"
    assert finding["expected"] == original["expected"]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert finding["readme_owner_task"] == "22.31.470"
    assert finding["owner_domain"] == original["owner_domain"] == 17
    assert task["wiring_requests"] == []
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_readme_component_boundaries.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["verified_revision"]
