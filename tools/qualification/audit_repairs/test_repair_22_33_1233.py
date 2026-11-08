"""Offline source proof for audit 22.30.59 finding 22 against the real README.

The gate checks documentation and production wiring, not native execution,
endpoint availability, cryptographic verification or restart recovery.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
RECORD = Path(__file__).with_name("repair_22_33_1233.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Extract the named Markdown section, respecting fenced examples."""
    readme = source("README.md")
    marker = "#" * level + " " + heading + "\n"
    assert readme.count(marker) == 1, heading
    lines = []
    fence = None
    for line in readme.split(marker, 1)[1].splitlines(keepends=True):
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


def assert_facts(text, facts):
    for fact in facts:
        assert fact in text, fact


def test_finding_22_overview_is_one_embedded_paxd_bft_network():
    overview = section("What is Paxeer X Network?", 2)
    assert_facts(overview, (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "no router and no separate LayerX interface", "outside consensus",
    ))
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    topology = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(topology) == 1
    assert_facts(topology[0], (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ))


def test_finding_22_public_rpc_hosts_and_chain_probe_use_the_root():
    quickstart = section("Quick start: connect to chain 125")
    assert_facts(quickstart, (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ))
    examples = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(examples) == 1
    argv = shlex.split(examples[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network",
    ]
    assert '# Expected result: "0x7d"' in examples[0]
    allowed = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = [
        urlsplit(url)
        for url in re.findall(r"https?://[^\s`<>)]+", source("README.md"))
        if urlsplit(url).hostname in allowed
    ]
    assert {url.hostname for url in urls} >= {
        "api1.mainnet-beta.paxeer.network", "api14.mainnet-beta.paxeer.network",
    }
    for url in urls:
        assert url.scheme == "https" and url.path in ("", "/")
        assert not url.query and not url.fragment and url.port is None
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_finding_22_transaction_to_committed_receipt_matches_real_sources():
    flow = section("Submit a transaction and follow its committed receipt")
    assert_facts(flow, (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`",
        "transaction hash", "root JSON-RPC URL", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`pax_sequencerReceipt`",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ))
    send = source("rpc/send.go")
    assert_facts(send, (
        "func (s *SendAPI) SendRawTransaction(",
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ))
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed",
    ]
    assert_facts(receipt, (
        'const SequencerReceiptMethod = "pax_sequencerReceipt"',
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))",
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted",
    ))
    ordered = [
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    ]
    positions = [flow.index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_finding_22_build_and_upgrade_instructions_target_merged_paxd():
    build = section("Build")
    assert_facts(build, (
        "Build the merged node from the repository root", "make paxd-build",
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "running network entry point is `paxd`",
    ))
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in source("Makefile")
    assert "\npaxd-build: paxd-native-libs\n" in source("Makefile")
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    upgrades = section("Releases and Network Upgrades")
    assert_facts(upgrades, (
        "executes kernel activity inside `paxd`", "commits it with the BFT chain state",
        "`v7.0` merge handler", "retires the old custody/checkpoint write paths",
        "`v7.0-anchor-store-delete` removes the historical anchor store",
    ))
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")
    assert_facts(source("node/upgrades.go"), (
        "app.UpgradeKeeper.SetUpgradeHandler(MergeUpgrade, app.mergeUpgradePlan)",
        'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"',
        "app.UpgradeKeeper.SetUpgradeHandler(AnchorStoreDeleteUpgrade,",
    ))


def test_finding_22_retired_executors_stay_historical_and_out_of_operator_steps():
    readme = source("README.md")
    retired = [p for p in readme.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(retired) == 1
    assert_facts(retired[0], (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "retired at the merge", "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads before store deletion are archive-only",
        "they do not provide current funding or finality paths",
    ))
    current = readme.replace(retired[0], "")
    for name in ("layerxVerify", "layerxCustody", "layerxAnchor"):
        assert not re.search(re.escape(name), current, re.I), name
    for stale in (
        r"\blayerxd\b", r"two (?:execution )?domains",
        r"merge is (?:only )?a draft", r"kernel still runs separately",
    ):
        assert not re.search(stale, readme, re.I), stale
    assert "custody deposit and credit-attestation funding flow is retired" in readme
    assert "archived formats add no settlement step to current block finality" in readme
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    assert_facts(source("precompiles/layerxanchor/layerxanchor.go"), (
        "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade",
        "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade",
    ))


def test_repair_record_maps_finding_22_to_executed_proofs_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["22"]
    assert finding["audit"] == "22.30.59"
    assert finding["file"] == "README.md"
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["1233"]
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert task["wiring_requests"] == []
    assert_facts(record["wiring"]["readme"]["request"], (
        "README.md", "domain 17", "22.31.470",
    ))
    assert record["wiring"]["readme"]["status"] == "supplied by 22.31.470; asserted here"
