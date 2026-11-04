"""Finding 54: qualify the production README's merged runtime and build contract.

Offline source checks only; no native execution, endpoint or deployment proof.
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
RECORD = Path(__file__).with_name("repair_22_33_134.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Read a section without treating comments inside code fences as headings."""
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
    """Extract a declared Make rule's prerequisites and tab-indented recipe."""
    match = re.search(
        r"^" + re.escape(target) + r":([^\n]*)\n((?:\t[^\n]*\n|\n)*)",
        source(path),
        re.M,
    )
    assert match is not None, (path, target)
    return match.group(1).split(), match.group(2)


def test_f54_one_paxd_network_embeds_the_kernel():
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
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native", "Validators -->|Commit| State",
    ):
        assert fact in topology[0], fact


def test_f54_chain_125_probe_and_numbered_hosts_use_root_json_rpc():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api-mainnet-beta.paxeer.network`",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "compare `eth_blockNumber`", "never a JSON-RPC endpoint",
        "node.hyperpaxeer.com` is only the hpx library mirror",
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
    allowed = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.hostname in allowed, url
            assert parsed.scheme == "https" and parsed.path in ("", "/"), url
            assert not parsed.query and not parsed.fragment, url
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_f54_node_rpc_names_and_hosted_contract_are_distinct():
    quickstart = section("Quick start: connect to chain 125")
    server = source("rpc/server.go")
    registered = set(re.findall(r'Namespace:\s*"([a-z0-9]+)"', server))
    for name in ("debug", "echo", "eth", "net", "pax", "txpool", "web3", "pax2"):
        assert f"`{name}`" in quickstart and name in registered, name
    assert "`test` is restricted to non-live chain IDs" in quickstart
    assert "config.EnableTestAPI && !evmCfg.IsLiveChainID(ctx)" in server
    assert "gateway contract does not establish method availability at public node URLs" in quickstart
    assert "Obtain its deployment URL and credentials from the operator" in quickstart
    assert "native `px_` account/history and receipt-key methods" in quickstart
    assert "`px_getReceiptKeys` is registered by the node" in quickstart
    assert "gateway relays its governed key ranges" in quickstart
    assert '{Namespace: "px", Service: pxAPI}' in server
    assert '{Namespace: "px", Service: &pxHistoryAPI{account: pxAPI}}' in server
    assert "Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}" in server
    gateway = json.loads(source("services/platform/hosted/gateway/openrpc.json"))
    assert gateway["openrpc"] == "1.3.2"
    methods = {entry["name"] for entry in gateway["methods"]}
    assert {"px_getBalances", "px_getReceiptKeys", "pax_sequencerReceipt"} <= methods


def test_f54_transaction_to_committed_receipt_matches_production_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet", "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height", "No separate service endpoint is required",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    for call in (
        "func (s *SendAPI) SendRawTransaction(",
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ):
        assert call in send, call
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
    assert "c.CallContext(ctx, &result, SequencerReceiptMethod, txHash)" in receipt
    proof = section("Receipt and proof verification")
    assert "Neither a transaction hash nor a signed execution receipt alone establishes block commitment" in proof
    assert "authenticate the block header and CometBFT commit against trusted validator information" in proof
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(diagrams) == 1
    positions = [diagrams[0].index(fact) for fact in (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "receipted (execution evidence)", "Propose MsgSequenceBatch in receipt order",
        "Commit block", "committed, blockNumber, blockHash",
    )]
    assert positions == sorted(positions)


def test_f54_retired_executors_are_labeled_history_not_deployment():
    native = section("EVM and Native Precompiles")
    paragraphs = [p for p in native.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(paragraphs) == 1
    history = paragraphs[0]
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "`layerxAnchor` (`0x1014`)", "retired refusal surfaces",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "rejects writes after the merge", "historical views retire at `v7.0-anchor-store-delete`",
        "Historical anchor reads before store deletion are archive-only",
    ):
        assert fact in history, fact
    current = README.replace(history, "")
    assert not re.search(r"layerx(?:Verify|Custody|Anchor)", current, re.I)
    assert not re.search(
        r"\blayerxd\b|\blx_[A-Za-z0-9_*]*|two (?:execution )?domains|merge is (?:only )?a draft",
        README, re.I,
    )
    assert "custody deposit and credit-attestation funding flow is retired" in section("Accounts and native funding")
    assert "External checkpoint settlement is retired" in README
    assert "guarantor attestations and the L0–L4 settlement ladder are retired" in native
    assert "archived formats add no settlement step to current block finality" in native
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")


def test_f54_build_guidance_links_the_kernel_into_paxd():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a", "tools/release/version.json",
        "make paxeer-build", "make -f chain.mk build", "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    prerequisites, recipe = make_rule("Makefile", "paxd-build")
    assert prerequisites == ["paxd-native-libs"]
    assert "jq -er .version tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    prerequisites, _ = make_rule("Makefile", "paxd-native-libs")
    assert prerequisites == ["$(LIBRARY)", "programs-release-build"]
    _, recipe = make_rule("Makefile", "paxeer-build")
    assert "$(PAXEER_MAKE) build" in recipe
    prerequisites, recipe = make_rule("chain.mk", "build")
    assert prerequisites == ["paxd-native-libs"]
    assert "go build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd" in recipe
    _, recipe = make_rule("chain.mk", "paxd-native-libs")
    assert "-f Makefile paxd-native-libs" in recipe
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release
    assert "retires the old custody/checkpoint write paths" in release


def test_f54_repair_record_maps_the_audit_to_executed_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["134"]
    finding = record["finding"]["54"]
    assert finding["audit"] == "22.30.114"
    assert finding["file"] == "README.md" and finding["class"] == "doc-drift"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {"test_f54_repair_record_maps_the_audit_to_executed_proofs"}
    audit = source("spec/layerx-merge/programme/audit/records/22.30.114.kvx")
    block = re.search(r"^\[finding\.54\]\n(.*?)(?=^\[|\Z)", audit, re.M | re.S)
    assert block is not None
    original = tomllib.loads(block.group(0))["finding"]["54"]
    assert original["file"] == finding["file"] and original["severity"] == "medium"
    assert finding["expected"] == original["expected"]
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_134.py"
    assert task["build_cmd"] == "python3 --version"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert task["missing_facts"] == []
    wiring = record["wiring"]["readme"]
    assert wiring["path"] == "README.md" and wiring["owner_domain"] == 17
    assert wiring["owning_task"] == "22.31.470" and wiring["status"] == "supplied"
    owner = tomllib.loads(source("tools/qualification/audit_repairs/repair_22_31_470.kvx"))
    coverage = owner["coverage"]["audit_22_30_114"]
    assert coverage["finding"] == "54" and coverage["consumer_tasks"] == ["22.33.134"]
