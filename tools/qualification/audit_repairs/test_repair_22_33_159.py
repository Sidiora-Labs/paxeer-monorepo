"""Offline source qualification of audit 22.30.117 finding 15.

Read the production README and its cited wiring; do not execute native tools or RPC.
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
RECORD = Path(__file__).with_name("repair_22_33_159.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep code-fence comments from terminating a Markdown section."""
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


def test_f15_single_paxd_network_and_block_finality():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    assert "subgraph Network[One Paxeer BFT network / EVM chain 125]" in overview
    assert "subgraph Node[paxd]" in overview
    assert "RPC --> Order --> Host --> Native" in overview
    assert "Validators -->|Commit| State" in overview
    assert not re.search(
        r"\blayerxd\b|kernel still runs separately|merge is (?:only )?a draft",
        README, re.I,
    )


def test_f15_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`" in quickstart
    assert "https://api-mainnet-beta.paxeer.network" in quickstart
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    command = blocks[0].split("\n# Expected result:", 1)[0].replace("\\\n", " ")
    argv = shlex.split(command)
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
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.scheme == "https" and parsed.path in ("", "/")
            assert not parsed.query and not parsed.fragment
            assert parsed.hostname in {
                "api-mainnet-beta.paxeer.network",
                *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
            }
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")
    assert "gateway contract does not establish method availability at public node URLs" in quickstart


def test_f15_transaction_and_committed_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`pax_sequencerReceipt`", "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "commitment does not mean execution succeeded", "`eth_getTransactionReceipt`",
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


def test_f15_merge_inventory_matches_activation_sources():
    native = section("EVM and Native Precompiles")
    rows = re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native)
    assert len(rows) == 13
    documented = {int(address, 16): name for address, name in rows}
    setup = source("precompiles/setup.go")
    table = setup.split("var MergeAddressTable = []MergeAddress{\n", 1)[1].split("\n}", 1)[0]
    entries = re.findall(r'\{ecommon.HexToAddress\("(0x[0-9A-Fa-f]{40})"\), "([^"]+)"\}', table)
    assert len(entries) == 13
    assert documented == {int(address, 16): name for address, name in entries}
    assert "`MergeAddressTable`" in native
    assert "gates activation by upgrade and module state" in native
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")
    upgrades = source("node/upgrades.go")
    assert "app.UpgradeKeeper.SetUpgradeHandler(MergeUpgrade, app.mergeUpgradePlan)" in upgrades
    gate = upgrades.split("func mergePrecompileGate()", 1)[1].split("\nfunc ", 1)[0]
    assert "Upgrade: MergeUpgrade" in gate
    gated = re.findall(r'common.HexToAddress\("(0x[0-9A-Fa-f]{40})"\):', gate)
    assert len(gated) == 13
    assert {int(address, 16) for address in gated} == set(documented)
    assert "serves `perps` at `0x1015` in place of the earlier `layerxExchange`" in native
    assert "select the interface for the upgrade active at the queried height" in native


def test_f15_retired_executors_and_archives_are_explicit():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads before store deletion are archive-only",
        "archived settlement witness formats", "support historical interpretation",
        "guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    paragraphs = [p for p in README.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(paragraphs) == 1
    current = README.replace(paragraphs[0], "")
    assert not re.search(r"\blayerx(?:Verify|Custody|Anchor)\b", current)
    assert "custody deposit and credit-attestation funding flow is retired" in README


def test_f15_paxd_build_and_release_examples():
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
    rules = dict(re.findall(r"^([A-Za-z0-9_-]+):([^\n]*)$", makefile, re.M))
    assert rules["paxd-native-libs"].split() == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
    assert rules["paxd-build"].split() == ["paxd-native-libs"]
    recipe = makefile.split("\npaxd-build: paxd-native-libs\n", 1)[1].split("\n\n", 1)[0]
    assert "jq -er .version tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert 'VERSION="$$version"' in recipe
    assert "sdk/version.Version=$(VERSION)" in source("chain.mk")
    assert "BUILD_FLAGS = -tags \"$(build_tags)\" -ldflags '$(paxd_ldflags)'" in source("chain.mk")
    version = json.loads(source("tools/release/version.json"))
    assert re.fullmatch(r"v\d+\.\d+\.\d+", version["version"])
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release
    assert "`v7.0` merge handler activates the merged interfaces" in release
    assert "`v7.0-anchor-store-delete` removes the historical anchor store" in release
    assert "network's recorded upgrade height" in release


def test_f15_record_maps_audit_to_proofs_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["159"]
    finding = record["finding"]["15"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.117.kvx"))
    for key in ("file", "line", "class", "severity", "owner_domain"):
        assert finding[key] == audit["finding"]["15"][key]
    assert finding["audit"] == "22.30.117"
    assert finding["repair_owner"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert len(finding["tests"]) == len(declared)
    assert task["wiring_requests"] == []
    assert "README.md, owning domain 1 and task 22.31.470" in task["wiring_policy"]
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_159.py"
    )
    assert task["build_cmd"] == "python3 --version"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
