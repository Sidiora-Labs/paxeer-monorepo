"""Source proof for audit 22.30.133/root-readme-topology, without native tools.

These checks consume the production README and its source references directly;
they do not certify node execution, recovery, or public endpoint availability.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
RECORD = (ROOT / "spec/layerx-merge/programme/records/tools/qualification/audit_repairs/repair_22_33_287.kvx")
README_PATH = ROOT / "docs/readme/NETWORK.md"
README = README_PATH.read_text(encoding="utf-8")
PUBLIC_RPC = (ROOT / "docs/readme/PUBLIC-RPC.md").read_text(encoding="utf-8")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


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


def test_root_readme_topology_is_one_integrated_chain_125_network():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "The LayerX kernel runs inside every `paxd` node.",
        "EVM chain ID **125** (`0x7d`)",
        "It executes each transaction when it arrives, assigns the next position "
        "in one global sequence, returns a signed execution receipt and builds "
        "each block.",
        "Validators check that block and commit it.",
        "A receipt is final when the validators commit its block.",
        "A receipt is final when the validators commit its block.",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
        "outside consensus",
    ):
        assert fact in overview, fact
    assert "There is no separate LayerX interface, router, or unified `/rpc` endpoint." in source("docs/wiki/Protocol.md")
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    topology = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(topology) == 1
    for fact in (
        "subgraph Network[Paxeer X / EVM chain 125]",
        "subgraph Node[paxd]",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in topology[0], fact


def test_root_readme_public_rpc_hosts_and_chain_probe_use_the_root():
    quickstart = section("Quick start: connect to chain 125")
    assert "Send HTTPS JSON-RPC requests to the root path without a route suffix" in quickstart
    assert "gateway contract does not establish method availability at public node URLs" in quickstart
    assert "Use `api1` through `api14.mainnet-beta.paxeer.network`" in PUBLIC_RPC
    assert "`node.hyperpaxeer.com` is only the hpx library mirror, never a JSON-RPC endpoint" in PUBLIC_RPC
    hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = [
        urlsplit(url) for url in re.findall(r"https?://[^\s`<>)]+", PUBLIC_RPC)
        if "mainnet-beta.paxeer.network" in url
    ]
    assert {url.hostname for url in urls} == hosts
    for url in urls:
        assert url.scheme == "https" and url.hostname in hosts
        assert url.path in ("", "/") and not url.query and not url.fragment
        assert url.port is None and url.username is None and url.password is None
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:", 1)[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert argv[argv.index("-sS") + 1] == "$PAXEER_RPC_URL"
    assert "Set `PAXEER_RPC_URL` to a node root URL from the [public RPC reference]" in quickstart
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_root_readme_distinguishes_hosted_and_node_px_contracts():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "services/platform/hosted/gateway/openrpc.json",
        "Obtain its deployment URL and credentials from the operator",
        "gateway contract does not establish method availability at public node URLs",
        "native `px_` account/history and receipt-key methods",
        "`px_getReceiptKeys` is registered by the node",
        "gateway relays its governed key ranges",
    ):
        assert fact in quickstart, fact
    contract = json.loads(source("services/platform/hosted/gateway/openrpc.json"))
    methods = {method["name"] for method in contract["methods"]}
    assert {"px_getBalances", "px_getReceiptKeys"} <= methods
    server = source("rpc/server.go")
    assert '{Namespace: "px", Service: pxAPI}' in server
    assert '{Namespace: "px", Service: &pxHistoryAPI{account: pxAPI}}' in server
    assert "Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}" in server


def test_root_readme_transaction_and_committed_receipt_follow_node_sources():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`eth_getTransactionCount`",
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_sendRawTransaction` to a public node's root JSON-RPC URL",
        "transaction hash",
        "durable WAL acknowledgement",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block",
        "`MsgSequenceBatch` covers that sequence",
        "authenticate the block commitment before treating the result as final",
        "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    assert "A receipt is final once its block is committed" in README
    send = source("rpc/send.go")
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt


def test_root_readme_retires_custody_checkpoint_and_separate_daemon_guidance():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces",
        "not active funding or checkpoint executors",
        "Historical anchor reads before store deletion are archive-only",
        "retained [batch-header codec]",
        "archived settlement witness formats",
        "support historical interpretation",
        "guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for stale in (
        r"\blayerxd\b", r"kernel still runs separately",
        r"merge is (?:only )?a draft", r"two execution domains",
    ):
        assert not re.search(stale, README, re.I), stale
    assert "External checkpoint settlement is retired" in README


def test_root_readme_build_and_release_target_the_merged_paxd():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    recipe = makefile.split("\npaxd-build: paxd-native-libs\n", 1)[1].split("\n\n", 1)[0]
    assert "tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "validators commit it in the same block" in release
    assert "`v7.0` merge handler activates the merged interfaces" in release
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")


def test_root_readme_repair_record_maps_the_finding_and_proof_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["287"]
    finding = record["finding"]["root-readme-topology"]
    assert finding["audit"] == "22.30.133"
    assert finding["file"] == "README.md" and finding["line"] == 47
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.133.kvx"))
    original = audit["finding"]["root-readme-topology"]
    assert original["class"] == "doc-drift" and original["severity"] == "medium"
    assert finding["expected"] == original["expected"]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert task["wiring_requests"] == []
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    wiring = record["wiring"]["root_readme"]
    assert wiring["path"] == "README.md" and wiring["owner_task"] == "22.31.470"
    assert wiring["owner_domain"] == 16 and wiring["status"] == "satisfied"
