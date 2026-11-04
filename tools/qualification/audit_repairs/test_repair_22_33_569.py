"""Offline source proof for audit 22.30.159 readme-network.

Read the maintained README and its real node/build/receipt contracts. This
documentation gate does not execute native code, shell examples or live RPC.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
RECORD = Path(__file__).with_name("repair_22_33_569.kvx")
README = (ROOT / "README.md").read_text(encoding="utf-8")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep shell comments inside fences from terminating a Markdown section."""
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


def test_readme_network_one_network_embeds_kernel_and_commits_blocks():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
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
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    topology = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(topology) == 1
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in topology[0], fact


def test_readme_network_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "https://api-mainnet-beta.paxeer.network",
        "`https://api1.mainnet-beta.paxeer.network` through",
        "`https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "compare `eth_blockNumber`",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:", 1)[0].replace("\\\n", " "))
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
    hosts = {"api-mainnet-beta.paxeer.network"} | {
        f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)
    }
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.scheme == "https" and parsed.hostname in hosts, url
            assert parsed.path in ("", "/") and not parsed.query and not parsed.fragment, url
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_network_hosted_and_node_method_contracts():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "services/platform/hosted/gateway/openrpc.json",
        "Obtain its deployment URL and credentials from the operator",
        "gateway contract does not establish method availability at public node URLs",
        "`px_getReceiptKeys` is registered by the node",
        "gateway relays its governed key ranges",
        "native `px_` account/history and receipt-key methods",
    ):
        assert fact in quickstart, fact
    contract = json.loads(source("services/platform/hosted/gateway/openrpc.json"))
    assert {"px_getBalances", "px_getReceiptKeys"} <= {
        method["name"] for method in contract["methods"]
    }
    server = source("rpc/server.go")
    assert '{Namespace: "px", Service: pxAPI}' in server
    assert '{Namespace: "px", Service: &pxHistoryAPI{account: pxAPI}}' in server
    assert "Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}" in server


def test_readme_network_transaction_to_committed_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "commitment does not mean execution succeeded",
        "`eth_getTransactionReceipt`", "`eth_call` at an explicit committed height",
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
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt
    diagram = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(diagram) == 1
    stages = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    )
    positions = [diagram[0].index(stage) for stage in stages]
    assert positions == sorted(positions)


def test_readme_network_custody_and_checkpoint_executors_are_retired():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "Historical anchor reads before store deletion are archive-only",
        "guarantor attestations and the L0–L4 settlement ladder are retired",
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
    for stale in (
        r"\blayerxd\b", r"\blx_[A-Za-z0-9_*]*", r"/rpc(?:/|\b)",
        r"kernel (?:currently|still) runs separately", r"merge (?:is|remains) (?:only )?(?:a )?draft",
    ):
        assert not re.search(stale, README, re.I), stale


def test_readme_network_maintained_paxd_build_and_launch():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    assert shlex.split(blocks[0], comments=True) == ["make", "paxd-build"]
    assert [shlex.split(line) for line in blocks[1].splitlines()] == [
        ["./build/paxd", "version"], ["./build/paxd", "start", "--home", "$PAXD_HOME"]
    ]
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    recipe = makefile.split("\npaxd-build: paxd-native-libs\n", 1)[1].split("\n\n", 1)[0]
    assert "tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert "PAXEER_MAKE := $(MAKE) -f $(PAXEER_DIR)/chain.mk" in makefile
    chain = source("chain.mk")
    assert "\nbuild: paxd-native-libs\n" in chain
    assert "-o ./build/paxd ./daemon/paxd" in chain
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release
    assert "`v7.0` merge handler activates the merged interfaces" in release
    assert "retires the old custody/checkpoint write paths" in release
    assert "`v7.0-anchor-store-delete` removes the historical anchor store" in release
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")
    upgrades = source("node/upgrades.go")
    assert "SetUpgradeHandler(MergeUpgrade, app.mergeUpgradePlan)" in upgrades
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in upgrades


def test_readme_network_supported_native_views():
    accounts = section("Accounts and native funding")
    for fact in (
        "`addr` precompile", "`0x1004`", "`getLayerXDid(address)`",
        "`getUnifiedAccount(address)`", "`layerxledger` precompile", "`0x101A`",
        "`accountId(bytes32,bytes32)`", "`balanceOf(bytes32,bytes32)`",
        "read `balanceOf` to confirm funding", "`eth_getBalance(address,\"latest\")`",
    ):
        assert fact in accounts, fact
    flow = section("Submit a transaction and follow its committed receipt")
    assert "Read native precompile views with `eth_call` at an explicit committed height" in flow
    contracts = (
        ("precompiles/addr/abi.json", {
            "getLayerXDid": (["address"], ["bytes32", "string"]),
            "getUnifiedAccount": (["address"], ["address", "string", "bytes32", "bytes32"]),
        }),
        ("precompiles/layerxledger/abi.json", {
            "accountId": (["bytes32", "bytes32"], ["bytes32"]),
            "balanceOf": (["bytes32", "bytes32"], ["uint128"]),
        }),
    )
    for path, expected in contracts:
        methods = {
            method["name"]: method for method in json.loads(source(path))
            if method["type"] == "function"
        }
        for name, (inputs, outputs) in expected.items():
            method = methods[name]
            assert method["stateMutability"] == "view", (path, name)
            assert [item["type"] for item in method["inputs"]] == inputs, (path, name)
            assert [item["type"] for item in method["outputs"]] == outputs, (path, name)
    native = section("EVM and Native Precompiles")
    table = source("precompiles/setup.go").split(
        "var MergeAddressTable = []MergeAddress{", 1
    )[1].split("\n}", 1)[0]
    entries = re.findall(r'HexToAddress\("(0x[0-9A-Fa-f]+)"\), "([a-z0-9 ]+)"', table)
    assert len(entries) == 13
    for address, name in entries:
        short_address = "0x" + address[-4:]
        assert f"| `{short_address}` | `{name}` |" in native, (address, name)


def test_readme_network_record_maps_finding_proofs_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["readme-network"]
    assert finding["audit"] == "22.30.159"
    assert finding["file"] == "README.md" and finding["owner_domain"] == 17
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["repair"]
    assert task["task"] == "22.33.569" and task["readme_owner_task"] == "22.31.470"
    assert task["wiring_requests"] == []
    wiring = record["wiring"]["readme_network"]
    assert wiring["path"] == "README.md"
    assert wiring["owner_domain"] == 17 and wiring["owner_task"] == "22.31.470"
    assert wiring["disposition"] == "fulfilled by the inherited README; no missing facts"
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.159.kvx"))
    observed = audit["finding"]["readme-network"]
    assert finding["class"] == observed["class"] == "doc-drift"
    assert finding["severity"] == observed["severity"] == "medium"
    assert finding["line"] == observed["line"] == 44
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert re.fullmatch(r"[0-9a-f]{40}", task["inspected_revision"])
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
