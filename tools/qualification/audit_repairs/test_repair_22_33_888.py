"""Offline source proof for audit 22.30.183/f051 after README task 22.31.470.

This qualifies documentation; it does not execute the native node or contact RPC.
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
RECORD = Path(__file__).with_name("repair_22_33_888.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
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


def make_rule(text, target):
    """Read concrete prerequisites and tabbed recipes, without invoking make."""
    matches = list(re.finditer(r"^" + re.escape(target) + r":([^\n]*)\n", text, re.M))
    assert matches, target
    prerequisites = set()
    recipes = []
    for match in matches:
        prerequisites.update(match.group(1).split())
        for line in text[match.end():].splitlines():
            if not line.startswith("\t"):
                break
            recipes.append(line.lstrip("\t"))
    return prerequisites, "\n".join(recipes)


def test_f051_one_bft_network_embeds_kernel_in_paxd():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
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
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State[Committed block and chain state]",
    ):
        assert fact in topology[0], fact


def test_f051_chain_125_probe_uses_numbered_root_json_rpc_hosts():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "Confirm the chain ID before signing or broadcasting",
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
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert '# Expected result: "0x7d"' in blocks[0]
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    node_urls = [
        urlsplit(url) for url in re.findall(r"https?://[^\s`<>)]+", README)
        if "mainnet-beta.paxeer.network" in url
    ]
    assert node_urls
    for url in node_urls:
        assert url.scheme == "https" and url.hostname in allowed_hosts
        assert url.path in ("", "/") and not url.query and not url.fragment
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_f051_transaction_to_committed_receipt_source_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
        "No separate service endpoint is required",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    accepter = source("node/sequencer/accepter.go")
    assert accepter.index("a.rep.Release(context.Background(), &wal.Accept{") < accepter.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt
    proof = section("Receipt and proof verification")
    for fact in (
        "ProtocolReceipt v2", "pinned for the receipt height",
        "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit",
        "trusted validator information", "authenticated root at the relevant height and key",
    ):
        assert fact in proof, fact
    assert "codec.DecodeReceipt(receiptBytes)" in source("layerxproof/verify/receipt.go")


def test_f051_custody_and_checkpoint_executors_are_explicitly_retired():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
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
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for stale in (
        r"\blayerxd\b", r"\blx_[A-Za-z0-9_*]*", r"/rpc(?:/|\b)",
        r"kernel still runs separately", r"merge (?:is|remains) (?:only )?(?:a )?draft",
        r"checkpoints settle through",
    ):
        assert not re.search(stale, README, re.I), stale


def test_f051_retained_build_targets_match_real_makefile_recipes():
    build = section("Build")
    for fact in (
        "make paxd-build", "make paxeer-build", "make -f chain.mk build",
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "For kernel library and development tools without rebuilding the node",
        "This builds the kernel archive, genesis builder and module registry",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    commands = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    assert any(shlex.split(block, comments=True) == ["make", "build"] for block in commands)
    makefile = source("Makefile")
    assert make_rule(makefile, "build")[0] == {
        "$(LIBRARY)", "layerx-genesis-build", "layerx-module-registry"
    }
    assert re.search(r"^LIBRARY := \$\(BUILD_DIR\)/liblayerx\.a$", makefile, re.M)
    assert make_rule(makefile, "paxd-native-libs")[0] == {"$(LIBRARY)", "programs-release-build"}
    prerequisites, recipe = make_rule(makefile, "paxd-build")
    assert prerequisites == {"paxd-native-libs"}
    assert "jq -er .version tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build LDFLAGS=" in recipe
    assert "sdk/version.Version=$$version" in recipe
    assert "$(PAXEER_MAKE) build" in make_rule(makefile, "paxeer-build")[1]
    chain = source("chain.mk")
    prerequisites, recipe = make_rule(chain, "build")
    assert prerequisites == {"paxd-native-libs"}
    assert "go build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd" in recipe
    assert "-f Makefile paxd-native-libs" in make_rule(chain, "paxd-native-libs")[1]
    archives = re.search(r"^NATIVE_ARCHIVES := (.+)$", chain, re.M)
    assert archives is not None
    assert archives.group(1).split() == [
        "$(PAXEER_PROJECT_HOME)/build/liblayerx.a",
        "$(PAXEER_PROJECT_HOME)/programs/target/release/liblayerx_programs_sandbox.a",
    ]


def test_f051_release_status_describes_merged_runtime_and_upgrade_gates():
    release = section("Releases and Network Upgrades")
    for fact in (
        "merged runtime executes kernel activity inside `paxd`",
        "commits it with the BFT chain state", "`v7.0` merge handler",
        "retires the old custody/checkpoint write paths",
        "`v7.0-anchor-store-delete` removes the historical anchor store",
        "network's recorded upgrade height",
    ):
        assert fact in release, fact
    upgrades = source("node/upgrades.go")
    assert "SetUpgradeHandler(MergeUpgrade, app.mergeUpgradePlan)" in upgrades
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in upgrades
    assert "SetUpgradeHandler(AnchorStoreDeleteUpgrade," in upgrades
    assert "maintained architecture entry point" in section("Specification", 2)


def test_f051_record_maps_all_proofs_and_preserves_scope_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["888"]
    finding = record["finding"]["f051"]
    assert finding["audit"] == "22.30.183" and finding["file"] == "README.md"
    audit = source("spec/layerx-merge/programme/audit/records/22.30.183.kvx")
    block = re.search(r"^\[finding\.f051\]\n.*?(?=^\[|\Z)", audit, re.M | re.S)
    assert block is not None
    original = tomllib.loads(block.group(0))["finding"]["f051"]
    assert original["class"] == finding["class"] == "doc-drift"
    assert original["severity"] == finding["severity"] == "medium"
    assert original["expected"] == finding["expected"]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_888.py"
    )
    assert task["build_cmd"] == "python3 --version"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["wiring_requests"] == []
    assert record["wiring"]["readme"]["path"] == "README.md"
    assert record["wiring"]["readme"]["owner_domain"] == 17
    assert record["wiring"]["readme"]["owner_task"] == "22.31.470"
    assert record["wiring"]["readme"]["missing_facts"] == []
