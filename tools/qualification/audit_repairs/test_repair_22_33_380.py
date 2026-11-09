"""Source qualification for root-retired-daemon against the maintained README.

These checks inspect real documentation and production wiring in-process. They
do not execute the native node, consensus, upgrades or a public RPC endpoint.
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
RECORD = Path(__file__).with_name("repair_22_33_380.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Respect fenced code when locating a Markdown section's end."""
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


def make_target(path, name):
    """Extract one concrete target's prerequisites and recipe without make."""
    text = source(path)
    matches = list(re.finditer(r"^" + re.escape(name) + r":([^\n]*)\n", text, re.M))
    assert len(matches) == 1, (path, name)
    match = matches[0]
    recipe = []
    for line in text[match.end():].splitlines():
        if not line.startswith("\t"):
            break
        recipe.append(line[1:])
    return match.group(1).split(), "\n".join(recipe)


def test_root_retired_daemon_merged_network_and_operator_components():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "`paxd` is built from [`daemon/paxd`](daemon/paxd/)",
        "runs inside `paxd` through [`modules/lxphost`](modules/lxphost/)",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    assert "cmd.NewRootCmd()" in source("daemon/paxd/main.go")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    assert not re.search(r"\blayerxd\b|make\s+layerxd", README, re.I)


def test_root_retired_daemon_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    hosts = {f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)}
    hosts.add("api-mainnet-beta.paxeer.network")
    urls = re.findall(r"https?://[^\s`<>)]+", quickstart)
    assert urls
    for url in urls:
        parsed = urlsplit(url)
        assert parsed.scheme == "https" and parsed.hostname in hosts
        assert parsed.path in ("", "/") and not parsed.query and not parsed.fragment
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
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_root_retired_daemon_submission_receipt_and_commit_source_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "No separate service endpoint is required",
        "authenticate the block commitment", "commitment does not mean execution succeeded",
        "`eth_getTransactionReceipt`", "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    producer = source("node/sequencer/accepter.go")
    assert producer.index("a.rep.Release(context.Background(), &wal.Accept{") < producer.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    consumer = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in consumer
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', consumer) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in consumer
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in consumer
    sequence = re.findall(r"```mermaid\n(sequenceDiagram.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    events = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL", "Release signed execution receipt",
        "receipted (execution evidence)", "Propose MsgSequenceBatch in receipt order",
        "Commit block", "committed, blockNumber, blockHash",
    )
    positions = [sequence[0].index(event) for event in events]
    assert positions == sorted(positions)


def test_root_retired_daemon_build_targets_produce_linked_paxd():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/paxd", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a", "tools/release/version.json",
        "make paxeer-build", "make -f chain.mk build", "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"', "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    dependencies, recipe = make_target("Makefile", "paxd-build")
    assert dependencies == ["paxd-native-libs"]
    assert "jq -er .version tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    dependencies, _ = make_target("Makefile", "paxd-native-libs")
    assert dependencies == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
    _, recipe = make_target("chain.mk", "paxd-native-libs")
    assert "$(MAKE) -C $(PAXEER_PROJECT_HOME) -f Makefile paxd-native-libs" in recipe
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")


def test_root_retired_daemon_active_precompile_addresses_match_upgrade_table():
    native = section("EVM and Native Precompiles")
    rows = re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native)
    documented = {int(address, 16): name for address, name in rows}
    assert len(rows) == len(documented) == 13
    setup = source("precompiles/setup.go")
    table = setup.split("var MergeAddressTable = []MergeAddress{", 1)[1].split("\n}", 1)[0]
    registered = re.findall(r'\{ecommon.HexToAddress\("(0x[0-9A-Fa-f]+)"\), "([^"]+)"\}', table)
    assert len(registered) == 13
    assert documented == {int(address, 16): name for address, name in registered}
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")
    assert "const MergeUpgrade = pcommon.MergeUpgrade" in setup
    upgrades = source("node/upgrades.go")
    assert "const MergeUpgrade = pcommon.MergeUpgrade" in upgrades
    assert "app.UpgradeKeeper.SetUpgradeHandler(MergeUpgrade, app.mergeUpgradePlan)" in upgrades
    assert "merge plan is `v7.0`" in native
    assert "select the interface for the upgrade active at the queried height" in native


def test_root_retired_daemon_legacy_executors_are_historical_refusals():
    native = section("EVM and Native Precompiles")
    paragraphs = [part for part in native.split("\n\n") if part.startswith("The pre-merge ")]
    assert len(paragraphs) == 1
    historical = paragraphs[0]
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "retired at the merge", "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads before store deletion are archive-only",
        "do not provide current funding or finality paths",
    ):
        assert fact in historical, fact
    current = README.replace(historical, "")
    assert not re.search(r"layerx(?:Verify|Custody|Anchor)", current, re.I)
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    custody = source("precompiles/layerxcustody/layerxcustody.go")
    assert "if p.keeper.Retired(ctx)" in custody
    assert 'RetiredRevert        = "LXP_MODULE_DISABLED: module disabled; "' in custody
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    assert "archived formats add no settlement step to current block finality" in native
    for target in re.findall(r"\]\(([^)]+)\)", native):
        assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_root_retired_daemon_record_names_proofs_limits_and_wiring_owner():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["380"]
    finding = record["finding"]["root-retired-daemon"]
    assert finding["audit"] == "22.30.140"
    assert finding["file"] == "README.md" and finding["class"] == "doc-drift"
    functions = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == functions
    assert len(finding["tests"]) == len(functions) == 7
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_380.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "README.md" in task["wiring_request"]
    assert "domain 17" in task["wiring_request"] and "22.31.470" in task["wiring_request"]
