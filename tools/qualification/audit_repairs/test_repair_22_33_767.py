"""Offline source proof for audit 22.30.173/doc-1, consuming README owner 22.31.470.

No native execution, network availability or deployment qualification is claimed.
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
RECORD = Path(__file__).with_name("repair_22_33_767.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Respect fenced examples when finding the end of a Markdown section."""
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


def make_rule(path, target):
    """Read a literal target's prerequisites and recipe without invoking make."""
    pattern = r"^" + re.escape(target) + r":([^\n]*)\n((?:\t[^\n]*\n)*)"
    matches = re.findall(pattern, source(path), re.M)
    assert len(matches) == 1, (path, target)
    prerequisites, recipe = matches[0]
    return prerequisites.split(), recipe


def test_doc1_one_network_embeds_kernel_and_commits_finality():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface", "outside consensus",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    topology = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(topology) == 1
    for edge in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert edge in topology[0]


def test_doc1_root_node_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "gateway contract does not establish method availability at public node URLs",
        "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
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
    allowed = {"api-mainnet-beta.paxeer.network"} | {
        f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)
    }
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.hostname in allowed
            assert parsed.scheme == "https" and parsed.path in ("", "/")
            assert not parsed.query and not parsed.fragment
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_doc1_transaction_receipt_producer_transport_and_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "root JSON-RPC URL", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`pax_sequencerReceipt`",
        "`submitted` has no execution receipt yet", "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "authenticate the block commitment", "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded", "`eth_call` at an explicit committed height",
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
    assert "if err == nil && ack.Status != AckOK" in release
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == ["submitted", "receipted", "committed"]
    coverage = "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))"
    committed = "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted"
    assert rpc.index(coverage) < rpc.index(committed)
    assert "if err != nil || !committed {\n\t\treturn result, err" in rpc
    assert "a.tmClient.Block(ctx, &height)" in rpc
    proof = section("Receipt and proof verification")
    assert "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`" in proof
    assert "authenticate the block header and CometBFT commit against trusted validator information" in proof


def test_doc1_custody_checkpoint_retirement_and_historical_labels():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces retained for compatibility, not active funding or checkpoint executors",
        "Historical anchor reads before store deletion are archive-only",
        "retained [batch-header codec]", "archived settlement witness formats",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "custody deposit and credit-attestation funding flow is retired" in section("Accounts and native funding")
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for stale in (r"\blayerxd\b", r"\blx_[A-Za-z0-9_*]*", r"/rpc(?:/|\b)"):
        assert not re.search(stale, README), stale


def test_doc1_build_and_release_describe_integrated_paxd():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "For kernel library and development tools without rebuilding the node",
        "The running network entry point is `paxd`",
    ):
        assert fact in build, fact
    prerequisites, recipe = make_rule("Makefile", "paxd-build")
    assert prerequisites == ["paxd-native-libs"]
    assert "tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert make_rule("Makefile", "paxd-native-libs")[0] == ["$(LIBRARY)", "programs-release-build"]
    prerequisites, recipe = make_rule("chain.mk", "build")
    assert prerequisites == ["paxd-native-libs"]
    assert "go build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd" in recipe
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd` and commits it with the BFT chain state" in release
    assert "retires the old custody/checkpoint write paths" in release
    assert "`v7.0-anchor-store-delete` removes the historical anchor store" in release
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    # No historical standalone build command is retained; current library builds
    # are explicitly distinguished from the running network entry point above.
    assert not re.search(r"\blayerxd\b|checkpoint.*(?:start|deploy)", build, re.I)


def test_doc1_repair_record_maps_finding_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["doc-1"]
    assert finding["audit"] == "22.30.173"
    assert finding["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["767"]
    assert task["verified_source_revision"] == "c27258d5023424d9619291dcc627e5a369726b61"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_767.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert task["wiring_requests"] == []
