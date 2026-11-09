"""Qualify readme-retired-network against the README and production sources.

Offline source proof only: no native build, node, subprocess or live RPC call.
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
RECORD = Path(__file__).with_name("repair_22_33_1121.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep fenced shell comments from ending a Markdown section."""
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


def make_target(text, name):
    match = re.search(
        r"^" + re.escape(name) + r":([^\n]*)\n((?:\t[^\n]*\n)*)", text, re.M
    )
    assert match is not None, name
    return match[1].split(), match[2]


def test_readme_retired_network_is_one_paxd_bft_chain():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network",
        "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "participates in block proposal and deterministic execution",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "no router and no separate LayerX interface",
        "outside consensus",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    host = source("modules/lxphost/host.go")
    assert 'import "C"' in host
    assert "-llayerx -llayerx_programs_sandbox" in host
    assert '"hyperpax_125-1": int64(125)' in source("modules/evm/config/config.go")
    consensus = source("node/sequencer/consensus_source.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in consensus
    assert "msg := seqtypes.MsgSequenceBatch{" in consensus
    assert "SealBatch(s.app, batch, msg)" in consensus


def test_readme_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "root path of these hosts, without a route suffix",
        "`https://api1.mainnet-beta.paxeer.network` through",
        "`https://api14.mainnet-beta.paxeer.network`",
        "compare `eth_blockNumber`",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
        "gateway contract does not establish method availability at public node URLs",
    ):
        assert fact in quickstart, fact
    urls = re.findall(r"https?://[^\s`<>)]+", quickstart)
    rpc_urls = [urlsplit(url) for url in urls if "mainnet-beta.paxeer.network" in url]
    assert {url.hostname for url in rpc_urls} == {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    for url in rpc_urls:
        assert url.scheme == "https"
        assert url.path in ("", "/") and not url.query and not url.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    command = blocks[0].split("\n# Expected result:", 1)[0].replace("\\\n", " ")
    argv = shlex.split(command)
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


def test_readme_transaction_to_committed_receipt_source_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`",
        "`eth_sendRawTransaction` to a public node's root JSON-RPC URL",
        "transaction hash",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block",
        "`MsgSequenceBatch` covers that sequence",
        "No separate service endpoint is required",
        "authenticate the block commitment before treating the result as final",
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
    accepter = source("node/sequencer/accepter.go")
    publication = accepter.split("func (a *Accepter) publish(", 1)[1]
    ordered = [
        "a.r.send(txBytes, acc, receipt)",
        "a.rep.Release(context.Background(), &wal.Accept{",
        "ack.Status != AckOK",
        "a.src.Record(common.Hash(acc.ActivityID), send)",
        "return evmrpc.AcceptedSend{Receipt: receipt",
    ]
    positions = [publication.index(fact) for fact in ordered]
    assert positions == sorted(positions)
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.option.Source.SequencerSend(txHash)" in rpc
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in rpc
    assert "a.tmClient.Block(ctx, &height)" in rpc
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in rpc
    assert 'return SequencerReceiptMethod + " served by sequencer"' in rpc


def test_readme_diagrams_keep_execution_and_finality_in_paxd():
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert len(diagrams) == 2
    topology, sequence = diagrams
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native",
        "Proposed block in execution order",
        "Validators -->|Commit| State",
    ):
        assert fact in topology, fact
    steps = [
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    ]
    positions = [sequence.index(step) for step in steps]
    assert positions == sorted(positions)


def test_readme_build_and_upgrade_match_integrated_release():
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
    prerequisites, recipe = make_target(makefile, "paxd-build")
    assert prerequisites == ["paxd-native-libs"]
    assert "tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    native, _ = make_target(makefile, "paxd-native-libs")
    assert native == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
    dependencies, command = make_target(source("chain.mk"), "build")
    assert dependencies == ["paxd-native-libs"]
    assert "go build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd" in command
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release
    assert "`v7.0` merge handler" in release
    assert "retires the old custody/checkpoint write paths" in release
    assert "`v7.0-anchor-store-delete` removes the historical anchor store" in release
    upgrades = source("node/upgrades.go")
    assert "app.UpgradeKeeper.SetUpgradeHandler(MergeUpgrade, app.mergeUpgradePlan)" in upgrades
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in upgrades
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")


def test_readme_custody_and_checkpoint_paths_are_historical_refusals():
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
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    custody = source("precompiles/layerxcustody/layerxcustody.go")
    assert "if p.keeper.Retired(ctx) {\n\t\treturn nil, errors.New(RetiredRevert)" in custody
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for stale in (
        r"\blayerxd\b",
        r"merge is (?:only )?a draft",
        r"kernel still runs separately",
        r"two (?:execution )?domains",
        r"https?://[^\s`]+/rpc(?:[/\s`]|$)",
    ):
        assert not re.search(stale, README, re.I), stale
    for paragraph in README.split("\n\n"):
        if re.search(r"layerxAnchor|guarantor|L0–L4", paragraph):
            assert re.search(r"retired|historical|archived", paragraph), paragraph
    for target in re.findall(r"\]\(([^)]+)\)", native):
        assert not target.startswith(("http:", "https:"))
        assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_repair_record_maps_the_audited_finding_and_proof_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["1121"]
    finding = record["finding"]["readme-retired-network"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.48.kvx"))
    expected = audit["finding"]["readme-retired-network"]
    assert finding["audit"] == "22.30.48"
    assert finding["file"] == expected["file"] == "README.md"
    assert finding["class"] == expected["class"] == "doc-drift"
    assert finding["severity"] == expected["severity"] == "medium"
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert len(finding["tests"]) == len(declared)
    assert set(finding["tests"]) == declared
    owner = tomllib.loads(source("tools/qualification/audit_repairs/repair_22_31_470.kvx"))
    coverage = owner["coverage"]["audit_22_30_48"]
    assert coverage["finding"] == "readme-retired-network"
    assert coverage["file"] == "README.md"
    assert "22.33.1121" in coverage["consumer_tasks"]
    assert task["readme_owner"] == "22.31.470 / domain 17"
    assert task["wiring_requests"] == []
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1121.py"
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
