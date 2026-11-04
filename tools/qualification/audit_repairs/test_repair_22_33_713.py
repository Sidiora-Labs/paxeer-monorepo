"""Qualify readme-network against the real README and linked production sources.

These offline source checks prove documentation facts, not native execution.
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
RECORD = Path(__file__).with_name("repair_22_33_713.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep fenced examples intact when finding the next Markdown heading."""
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


def test_readme_network_embeds_kernel_and_block_proposal_in_paxd():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network",
        "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "participates in block proposal and deterministic execution",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
        "no router and no separate LayerX interface",
        "outside consensus",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(diagrams) == 1
    for fact in (
        "One Paxeer BFT network / EVM chain 125",
        "subgraph Node[paxd]",
        "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native",
        "BFT validators",
        "Committed block and chain state",
    ):
        assert fact in diagrams[0], fact


def test_readme_network_chain_probe_and_numbered_hosts_use_root_rpc():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    hosts = {f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)}
    hosts.add("api-mainnet-beta.paxeer.network")
    urls = [
        urlsplit(url)
        for url in re.findall(r"https?://[^\s`<>)]+", README)
        if urlsplit(url).hostname.endswith("mainnet-beta.paxeer.network")
    ]
    assert {url.hostname for url in urls} == {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    for url in urls:
        assert url.hostname in hosts
        assert url.scheme == "https"
        assert url.path in ("", "/")
        assert not url.query and not url.fragment and url.port is None
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_network_transaction_path_reaches_durable_receipt_producer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`",
        "`eth_sendRawTransaction`",
        "public node's root JSON-RPC URL",
        "transaction hash",
        "accepts on the node's integrated sequencing path or forwards",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "No separate service endpoint is required",
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


def test_readme_network_finality_requires_committed_block_coverage():
    overview = section("What is Paxeer X Network?", 2)
    assert "A receipt is final once its block is committed" in overview
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`pax_sequencerReceipt`",
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
    rpc = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed"
    ]
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in rpc
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in rpc
    proofs = section("Receipt and proof verification")
    assert "Neither a transaction hash nor a signed execution receipt alone establishes block commitment" in proofs
    assert "authenticate the block header and CometBFT commit" in proofs
    assert "trusted validator information" in proofs
    diagram = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(diagram) == 1
    assert diagram[0].index("receipted (execution evidence)") < diagram[0].index(
        "BFT->>BFT: Commit block"
    ) < diagram[0].index("committed, blockNumber, blockHash")


def test_readme_network_legacy_custody_and_checkpoints_are_history_only():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "retained [batch-header codec]",
        "archived settlement witness formats",
        "support historical interpretation",
        "guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for stale in (
        r"\blayerxd\b",
        r"kernel still runs separately",
        r"two (?:execution )?domains",
        r"merge is (?:only )?a draft",
    ):
        assert not re.search(stale, README, re.I), stale


def test_readme_network_operator_build_targets_the_integrated_node():
    build = section("Build")
    for fact in (
        "make paxd-build",
        "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "EVM chain ID `125` is the signing domain",
        "Building the binary alone does not provision network membership",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release
    assert "retires the old custody/checkpoint write paths" in release


def test_readme_network_record_maps_the_finding_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    repair = record["repair"]
    assert repair["task"] == "22.33.713"
    assert repair["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_713.py"
    )
    assert repair["build_cmd"] == "python3 --version"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    assert "not executed" in repair["review_only"]
    assert "No baseline test was run" in repair["baseline"]
    finding = record["finding"]["readme-network"]
    assert finding["audit"] == "22.30.17"
    assert finding["file"] == "README.md"
    assert finding["class"] == "doc-drift"
    assert finding["severity"] == "medium"
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert len(finding["tests"]) == len(declared) == 7
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.17.kvx"))
    original = audit["finding"]["readme-network"]
    assert original["file"] == finding["file"]
    assert original["severity"] == finding["severity"]
    assert original["class"] == finding["class"]
    assert "integrated block-proposing kernel and committed-block receipt finality" in original["proposal"]
    wiring = record["wiring"]["root_readme"]
    assert wiring["path"] == "README.md"
    assert wiring["owner_task"] == "22.31.470"
    assert wiring["owner_domain"] == 17
    assert wiring["missing_facts"] == []
    assert wiring["disposition"] == "Supplied by 22.31.470; asserted here without editing README.md."
