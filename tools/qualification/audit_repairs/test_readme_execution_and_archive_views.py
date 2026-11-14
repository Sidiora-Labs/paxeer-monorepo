"""Offline source proof for audit 22.30.154/readme_split_execution.

The network reference and public RPC page describe the node and retained
history. These checks read those pages and their production source references.
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
PUBLIC_RPC = (ROOT / "docs/readme/PUBLIC-RPC.md").read_text(encoding="utf-8")
RECORD = ROOT / "spec/layerx-merge/programme/records/tools/qualification/audit_repairs/repair_22_33_526.kvx"


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Read a section while preserving headings and comments inside fences."""
    text = NETWORK_PAGE.read_text(encoding="utf-8")
    marker = "#" * level + " " + heading + "\n"
    assert text.count(marker) == 1, heading
    lines = []
    fence = None
    for line in text.split(marker, 1)[1].splitlines(keepends=True):
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


def test_readme_split_execution_one_embedded_paxd_network():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "The LayerX kernel runs inside every `paxd` node.", "chain ID **125** (`0x7d`)",
        "It executes each transaction when it arrives, assigns the next position "
        "in one global sequence, returns a signed execution receipt and builds "
        "each block.",
        "Validators check that block and commit it.", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "C17 kernel", "runs inside `paxd`", "modules/lxphost",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
    ):
        assert fact in overview, fact
    for fact in (
        "`api2.mainnet-beta.paxeer.network` is the archive node and keeps\nfull history",
        "the other thirteen numbered nodes prune",
        "Historical queries\ndepend on the node's retained state and receipt availability",
        "Error `-32000` alone does not identify a retention limit",
    ):
        assert fact in PUBLIC_RPC, fact
    topology = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(topology) == 1
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Native bank, ledger, payments, authority and programs",
        "Validators -->|Commit| State",
    ):
        assert fact in topology[0], fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_readme_split_execution_root_json_rpc_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Use `api1` through `api14.mainnet-beta.paxeer.network` or",
        "`api-mainnet-beta.paxeer.network`, all at the root path",
        "node.hyperpaxeer.com` is only the hpx library mirror, never a JSON-RPC endpoint",
    ):
        assert fact in PUBLIC_RPC, fact
    for fact in (
        "Set `PAXEER_RPC_URL` to a node root URL",
        "Send HTTPS JSON-RPC requests to the root path without a route suffix",
        "compare `eth_blockNumber`",
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
    assert '# Expected result: "0x7d"' in blocks[0]
    assert argv[argv.index("-sS") + 1] == "$PAXEER_RPC_URL"
    for url in re.findall(r"https?://[^\s`<>)]+", PUBLIC_RPC):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.scheme == "https"
            assert parsed.path in ("", "/")
            assert parsed.hostname in {
                "api-mainnet-beta.paxeer.network",
                *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
            }
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_split_execution_transaction_to_committed_receipt():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "Native ledger, payment, authority and Programs operations use `0x101A`–`0x101D`",
        "Programs payload bytes go to `0x101D`", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "commitment does not mean execution succeeded", "`eth_getTransactionReceipt`",
        "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    ordered = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    )
    positions = [sequence[0].index(fact) for fact in ordered]
    assert positions == sorted(positions)
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


def test_readme_split_execution_activation_retirement_and_archive_reads():
    native = section("EVM and Native Precompiles")
    for fact in (
        "merge plan is `v7.0`", "`layerxVerify` (`0x1012`)",
        "`layerxCustody` (`0x1013`)", "`layerxAnchor` (`0x1014`)",
        "retired at the merge", "rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads before store deletion are archive-only",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    release = section("Releases and Network Upgrades")
    assert "`v7.0` merge handler activates the merged interfaces" in release
    assert "`v7.0-anchor-store-delete` removes the historical anchor store" in release


def test_readme_split_execution_examples_use_paxd_and_library_tools():
    build = section("Build")
    blocks = [shlex.split(block, comments=True) for block in re.findall(
        r"```sh\n(.*?)\n```", build, re.S
    )]
    assert ["make", "paxd-build"] in blocks
    assert ["./build/paxd", "version", "./build/paxd", "start", "--home", "$PAXD_HOME"] in blocks
    assert "kernel library and development tools without rebuilding the node" in build
    assert "running network entry point is `paxd`" in build
    assert "build/liblayerx.a" in build
    assert "programs/target/release/liblayerx_programs_sandbox.a" in build
    assert "\npaxd-build: paxd-native-libs\n" in source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in source("Makefile")
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    for stale in (
        r"\blayerxd\b", r"two (?:execution )?domains",
        r"kernel still runs separately", r"make\s+checkpoint",
    ):
        assert not re.search(stale, NETWORK_PAGE.read_text(encoding="utf-8"), re.I), stale


def test_readme_split_execution_repair_record_maps_proof_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["readme_split_execution"]
    assert finding["audit"] == "22.30.154"
    assert finding["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["526"]
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_readme_execution_and_archive_views.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert task["wiring_requests"] == []
