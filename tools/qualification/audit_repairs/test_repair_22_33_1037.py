"""Offline source proof for audit 22.30.4/028, consuming README owner 22.31.470.

No native execution, deployment, endpoint availability or recovery is certified.
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
RECORD = Path(__file__).with_name("repair_22_33_1037.kvx")


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


def test_f028_architecture_is_the_integrated_paxd_network():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "[`daemon/paxd`](daemon/paxd/)", "[`modules/lxphost`](modules/lxphost/)",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    host = source("modules/lxphost/host.go")
    assert 'import "C"' in host
    assert "-llayerx -llayerx_programs_sandbox" in host
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    topology = section("Integrated network topology")
    assert "subgraph Network[One Paxeer BFT network / EVM chain 125]" in topology
    assert "subgraph Node[paxd]" in topology
    assert "RPC --> Order --> Host --> Native" in topology
    assert "Validators -->|Commit| State" in topology


def test_f028_public_rpc_example_uses_chain_125_at_the_node_root():
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
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.scheme == "https" and parsed.path in ("", "/")
            assert parsed.hostname in {
                "api-mainnet-beta.paxeer.network",
                *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
            }
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_f028_transaction_and_committed_receipt_follow_the_real_sources():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet", "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    assert "A receipt is final once its block is committed" in section("What is Paxeer X Network?", 2)
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    accepter = source("node/sequencer/accepter.go")
    assert accepter.index("a.rep.Release(context.Background(), &wal.Accept{") < accepter.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipts = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipts
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipts) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipts
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipts
    proof = section("Receipt and proof verification")
    assert "pinned for the receipt height" in proof
    assert "authenticate the block header and CometBFT commit" in proof
    assert "trusted validator information" in proof
    assert "codec.DecodeReceipt(receiptBytes)" in source("layerxproof/verify/receipt.go")


def test_f028_specification_describes_implemented_upgrade_control():
    specification = section("Specification", 2)
    assert "maintained architecture entry point" in specification
    assert "integrated network's requirements and implementation tasks" in specification
    assert "Runtime behavior is implemented in the node, kernel host and native modules" in specification
    assert "activation is controlled by the upgrade handlers" in specification
    assert "draft" not in specification.lower()
    upgrades = source("node/upgrades.go")
    assert "const MergeUpgrade = pcommon.MergeUpgrade" in upgrades
    assert "app.UpgradeKeeper.SetUpgradeHandler(MergeUpgrade, app.mergeUpgradePlan)" in upgrades
    assert 'MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")


def test_f028_current_precompile_table_matches_the_merge_registry():
    native = section("EVM and Native Precompiles")
    documented = re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native)
    assert len(documented) == 13
    table = source("precompiles/setup.go").split("var MergeAddressTable = []MergeAddress{\n", 1)[1].split("\n}", 1)[0]
    registered = re.findall(r'\{ecommon.HexToAddress\("(0x[0-9A-Fa-f]+)"\), "([^"]+)"\}', table)
    assert len(registered) == 13
    assert {int(address, 16): name for address, name in documented} == {
        int(address, 16): name for address, name in registered
    }
    assert {int(address, 16) for address, _ in documented} == {
        *range(0x101A, 0x101F), *range(0x0901, 0x0909)
    }
    assert not {0x1012, 0x1013, 0x1014} & {int(address, 16) for address, _ in documented}


def test_f028_retired_executors_and_daemons_are_not_deployment_guidance():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    # The owner removed the old daemons entirely, so no dated migration account
    # is needed to keep them out of current architecture and build instructions.
    assert not re.search(r"\b(?:layerxd|layerxctl)\b", README)
    for paragraph in README.split("\n\n"):
        if re.search(r"\blayerx(?:Verify|Custody|Anchor)\b", paragraph):
            assert "retired refusal surfaces" in paragraph
            assert "archive-only" in paragraph


def test_f028_build_and_start_target_the_linked_paxd_binary():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    rules = dict(re.findall(r"^([\w-]+): ([^\n]*)$", source("Makefile"), re.M))
    assert rules["paxd-native-libs"].split() == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
    assert rules["paxd-build"].split() == ["paxd-native-libs"]
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    releases = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in releases
    assert "commits it with the BFT chain state" in releases
    for body in (section("What is Paxeer X Network?", 2), section("Specification", 2), build):
        links = re.findall(r"\]\(([^)]+)\)", body)
        assert links
        for target in links:
            if not target.startswith(("http:", "https:", "#")):
                assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_f028_repair_record_names_its_proofs_ownership_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["1037"]
    finding = record["finding"]["028"]
    assert finding["audit"] == "22.30.4"
    assert finding["file"] == "README.md" and finding["owner_domain"] == 17
    assert finding["owner_task"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["readme_revision"] == "38801bb427090ac96d47c86a026a63ec05e67315"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1037.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["missing_facts"] == []
    assert len(task["wiring_requests"]) == 1
    request = task["wiring_requests"][0]
    assert all(name in request for name in ("README.md", "domain 17", "22.31.470", "satisfied"))
