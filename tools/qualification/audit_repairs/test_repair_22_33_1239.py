"""Offline source-fact proof for audit 22.30.6/readme-retired-daemon.

Read the production README and its implementation references directly. This
documentation gate does not execute a node, a build toolchain or a live RPC.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
RECORD = Path(__file__).with_name("repair_22_33_1239.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Extract one Markdown section, respecting fenced examples."""
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


def test_readme_merged_paxd_execution_and_committed_finality():
    overview = section("What is Paxeer X Network?", 2)
    assert_facts(overview, (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "modules/lxphost", "node/sequencer",
        "A receipt is final once its block is committed",
        "signed execution receipt can arrive before that commitment",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "no router and no separate LayerX interface",
    ))
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    assert not re.search(r"\blayerxd\b|\bmake\s+layerxd\b", source("README.md"))


def test_readme_root_rpc_range_and_chain_125_request():
    quickstart = section("Quick start: connect to chain 125")
    assert_facts(quickstart, (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint", "compare `eth_blockNumber`",
    ))
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
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
    urls = [urlsplit(url) for url in re.findall(
        r"https?://[^\s`<>)]+", source("README.md")
    )]
    public = [url for url in urls if url.hostname and
              url.hostname.endswith("mainnet-beta.paxeer.network")]
    assert public
    for url in public:
        assert url.hostname in hosts
        assert url.scheme == "https" and url.path in ("", "/")
        assert not url.query and not url.fragment
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_transaction_transport_and_committed_receipt_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    assert_facts(flow, (
        "`eth_getTransactionCount`", "EIP-1559 transaction for chain ID `125`",
        "`eth_sendRawTransaction`", "root JSON-RPC URL", "transaction hash",
        "integrated sequencing path", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`pax_sequencerReceipt`",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "authenticate the block commitment", "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ))
    send = source("rpc/send.go")
    assert_facts(send, (
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
        "submitted", "receipted", "committed"
    ]
    assert_facts(receipt, (
        'const SequencerReceiptMethod = "pax_sequencerReceipt"',
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))",
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted",
    ))


def test_readme_paxd_build_and_deployment_commands_match_producers():
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    commands = [shlex.split(line, comments=True) for block in blocks
                for line in block.splitlines() if line.strip()]
    assert ["make", "paxd-build"] in commands
    assert ["./build/paxd", "version"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands
    assert_facts(build, (
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "operator-provided genesis and consensus chain identifier",
        "Building the binary alone does not provision network membership",
        "running network entry point is `paxd`",
    ))
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    recipe = makefile.split("\npaxd-build: paxd-native-libs\n", 1)[1].split("\n\n", 1)[0]
    assert_facts(recipe, ("tools/release/version.json", "$(PAXEER_MAKE) build"))
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")


def test_readme_active_and_replacement_precompiles_match_address_table():
    native = section("EVM and Native Precompiles")
    rows = {int(address, 16): name for address, name in re.findall(
        r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native
    )}
    table = source("precompiles/setup.go").split(
        "var MergeAddressTable = []MergeAddress{", 1
    )[1].split("\n}", 1)[0]
    production = {int(address, 16): name for address, name in re.findall(
        r'\{ecommon.HexToAddress\("(0x[0-9A-Fa-f]+)"\), "([^"]+)"\}', table
    )}
    assert len(rows) == len(production) == 13
    assert rows == production
    assert_facts(native, (
        "upgrade active at the queried height", "`v6.12.0`", "`v6120Contracts`",
        "serves `perps` at `0x1015` in place of the earlier `layerxExchange`",
        "second-generation `launchpad` at `0x1017`", "`pricefeed` at `0x1020`",
        "`spot_pecor`", "`spot_orders`", "`spot_stoporders`", "`spot_quoter`", "`spot_vault`",
        "`v7.0`", "node/upgrades.go",
    ))
    app = source("node/app.go").split("func (app *App) v6120Contracts()", 1)[1]
    assert_facts(app.split("\n}\n", 1)[0], (
        "perpsprecompile.NewPrecompile(keepers)",
        "common.HexToAddress(layerxexchangetypes.ExchangeAddress): {perps, exchange}",
        "launchpadprecompile.NewPrecompile(keepers)",
        "pricefeedprecompile.NewPrecompile(keepers)", "spotprecompile.NewServedSet(keepers)",
    ))


def test_readme_post_plan_refusals_and_supported_replacements():
    native = section("EVM and Native Precompiles")
    assert_facts(native, (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "retired at the merge", "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads before store deletion are archive-only",
        "they do not provide current funding or finality paths",
        "external native bridge at `0x1016` remains distinct", "`bridgeOut`",
        "archived formats add no settlement step to current block finality",
    ))
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")
    assert 'return LXPRevert("LXP_MODULE_DISABLED", name+" retired")' in source(
        "precompiles/common/retire.go"
    )
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    custody = source("precompiles/layerxcustody/layerxcustody.go")
    assert "if p.keeper.Retired(ctx) {\n\t\treturn nil, errors.New(RetiredRevert)" in custody
    assert 'RetiredRevert        = "LXP_MODULE_DISABLED: module disabled; "' in custody
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert_facts(anchor, (
        "if pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade, ctx.BlockHeight())",
        "if !isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade",
    ))
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    accounts = section("Accounts and native funding")
    assert_facts(accounts, (
        "custody deposit and credit-attestation funding flow is retired",
        "authorized issuer mint", "Wait for commit and read `balanceOf`",
    ))
    ledger = {item["name"]: item for item in json.loads(
        source("precompiles/layerxledger/abi.json")
    ) if item["type"] == "function"}
    for name in ("openAccount", "mint", "payExact", "balanceOf"):
        assert name in accounts
        assert ledger[name]["stateMutability"] == ("view" if name == "balanceOf" else "nonpayable")


def test_repair_record_maps_the_audit_finding_to_declared_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["readme-retired-daemon"]
    audit = tomllib.loads(source(record["meta"]["audit_record"]))
    assert finding["path"] == audit["finding"]["readme-retired-daemon"]["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    declared = {node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
                if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    assert set(finding["tests"]) == declared - {
        "test_repair_record_maps_the_audit_finding_to_declared_proofs"
    }
    assert record["meta"]["task"] == "22.33.1239"
    assert record["qualification"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1239.py"
    )
    assert record["qualification"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not run" in record["baseline"]["verification"]
    assert "not executed" in record["proof"]["review_only"]
    assert record["wiring"]["requests"] == []
