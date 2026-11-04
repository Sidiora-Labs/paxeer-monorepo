"""Offline source proof for audit 22.30.134/doc_1 and decision 2525.

Read the production README and its referenced implementation without executing
native code, making network requests, or claiming runtime/recovery qualification.
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
RECORD = Path(__file__).with_name("repair_22_33_296.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep fenced shell comments from terminating a Markdown section."""
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


def test_doc_1_one_merged_paxd_network_and_block_finality():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_doc_1_root_rpc_hosts_and_chain_125_example():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "root path of these hosts, without a route suffix",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "gateway contract does not establish method availability at public node URLs",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    urls = [
        urlsplit(url)
        for url in re.findall(r"https?://[^\s`<>)]+", README)
        if "mainnet-beta.paxeer.network" in url
    ]
    assert urls
    for url in urls:
        assert url.scheme == "https" and url.path in ("", "/")
        assert url.hostname in {
            "api-mainnet-beta.paxeer.network",
            *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
        }
        assert not url.query and not url.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_doc_1_transaction_transport_and_committed_receipt_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "`pax_sequencerReceipt`", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence", "`committed` additionally names the block",
        "`MsgSequenceBatch`", "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    for fact in (
        "func (s *SendAPI) SendRawTransaction(",
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ):
        assert fact in send, fact
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
    proof = section("Receipt and proof verification")
    assert "authenticate the block header and CometBFT commit" in proof
    assert "trusted validator information" in proof
    assert "Neither a transaction hash nor a signed execution receipt alone establishes block commitment" in proof


def test_doc_1_active_precompile_inventory_matches_merge_table():
    native = section("EVM and Native Precompiles")
    rows = re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native)
    table = source("precompiles/setup.go").split(
        "var MergeAddressTable = []MergeAddress{\n", 1
    )[1].split("\n}", 1)[0]
    entries = re.findall(r'\{ecommon.HexToAddress\("(0x[0-9A-Fa-f]+)"\), "([^"]+)"\}', table)
    assert len(rows) == len(entries) == 13
    assert [(int(address, 16), name) for address, name in rows] == [
        (int(address, 16), name) for address, name in entries
    ]
    for fact in (
        "`layerxBridge` (`0x1016`)", "`feeToken` (`0x1018`", "`xweb` (`0x1019`",
        "serves `perps` at `0x1015` in place of the earlier `layerxExchange`",
        "`launchpad` at `0x1017`", "`pricefeed` at `0x1020`",
        "`spot_pecor`, `spot_orders`, `spot_stoporders`, `spot_quoter` and `spot_vault`",
        "select the interface for the upgrade active at the queried height",
    ):
        assert fact in native, fact


def test_doc_1_retired_executors_are_only_historical_guidance():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads before store deletion are archive-only",
        "retained [batch-header codec]", "archived settlement witness formats",
        "support historical interpretation",
        "guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for paragraph in README.split("\n\n"):
        if re.search(r"layerxVerify|layerxCustody|layerxAnchor|guarantor|L0–L4", paragraph):
            assert re.search(r"retired|historical|archived", paragraph), paragraph
    for stale in (r"\blayerxd\b", r"/rpc(?:/|\b)", r"kernel still runs separately", r"merge is (?:only )?a draft"):
        assert not re.search(stale, README, re.I), stale


def test_doc_1_build_and_source_paths_describe_merged_node():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "Source directory names describe code organization, not separate network deployments" in README
    for target in re.findall(r"\]\(([^)]+)\)", README):
        if not target.startswith(("https:", "http:", "#")):
            assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_doc_1_repair_record_maps_the_finding_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["296"]
    finding = record["finding"]["doc_1"]
    assert finding["audit"] == "22.30.134" and finding["file"] == "README.md"
    assert finding["owner_domain"] == 17 and finding["rewrite_task"] == "22.31.470"
    audit = source("spec/layerx-merge/programme/audit/records/22.30.134.kvx")
    audited = audit.split("[finding.doc_1]\n", 1)[1].split("\n[", 1)[0]
    assert 'file = "README.md"' in audited and 'severity = "medium"' in audited
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert len(finding["tests"]) == len(declared)
    assert set(finding["tests"]) == declared
    assert task["wiring_requests"] == []
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_296.py"
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
