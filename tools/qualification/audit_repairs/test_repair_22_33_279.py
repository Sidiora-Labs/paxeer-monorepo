"""Offline source proof for audit 22.30.132's root-network-description finding.

README.md is owned by task 22.31.470. No native runtime or endpoint is exercised.
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
RECORD = Path(__file__).with_name("repair_22_33_279.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep fenced example comments from terminating Markdown sections."""
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


def test_root_network_description_embeds_kernel_in_one_bft_node():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network",
        "EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "runs inside `paxd` through [`modules/lxphost`]",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_root_rpc_hosts_and_chain_probe_use_https_root():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert "`https://api1.mainnet-beta.paxeer.network` through" in quickstart
    assert "`https://api14.mainnet-beta.paxeer.network`" in quickstart
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.hostname in allowed_hosts, url
            assert parsed.scheme == "https" and parsed.path in ("", "/"), url
            assert not parsed.query and not parsed.fragment, url
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
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
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_transaction_and_committed_receipt_follow_production_sources():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`",
        "`eth_sendRawTransaction`",
        "transaction hash",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block",
        "`MsgSequenceBatch` covers that sequence",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in rpc
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in rpc
    proof = section("Receipt and proof verification")
    assert "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`" in proof
    assert "authenticate the block header and CometBFT commit" in proof
    assert "trusted validator information" in proof
    assert "A receipt is final once its block is committed" in section(
        "What is Paxeer X Network?", 2
    )


def test_topology_and_receipt_diagrams_preserve_integrated_commit_order():
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert len(diagrams) == 2
    topology, sequence = diagrams
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in topology, fact
    events = [
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
        "Committed status, logs and native state",
    ]
    positions = [sequence.index(event) for event in events]
    assert positions == sorted(positions)


def test_build_and_specification_describe_implemented_paxd_runtime():
    build = section("Build")
    for fact in (
        "make paxd-build",
        "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    specification = section("Specification", 2)
    assert "tracks the integrated network's requirements and implementation tasks" in specification
    assert "Runtime behavior is implemented in the node, kernel host and native modules" in specification
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release
    for stale in (
        r"\blayerxd\b",
        r"merge (?:is|remains) (?:only )?(?:a )?draft",
        r"kernel (?:still )?runs separately",
    ):
        assert not re.search(stale, README, re.I), stale


def test_custody_and_checkpoint_executors_are_historical_refusal_surfaces():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
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
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    for paragraph in README.split("\n\n"):
        if re.search(r"\blayerx(?:Anchor|Custody|Verify)\b|guarantor|L0–L4", paragraph):
            assert re.search(r"retired|historical|archived", paragraph), paragraph
    for body, target in (
        (section("What is Paxeer X Network?", 2), "docs/site/paxeer-docs/src/app/checkpoints/page.tsx"),
        (native, "docs/site/paxeer-docs/src/app/proofs/page.tsx"),
    ):
        assert target in body
        assert (ROOT / target).is_file()


def test_repair_record_maps_finding_to_all_proofs_and_qualification_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["279"]
    finding = record["finding"]["root-network-description"]
    assert finding["audit"] == "22.30.132"
    assert finding["file"] == "README.md"
    assert finding["owner_domain"] == 17
    assert finding["readme_owner_task"] == "22.31.470"
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert task["wiring_requests"] == []
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
