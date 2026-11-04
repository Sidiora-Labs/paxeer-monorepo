"""Offline source proof for audit 22.30.182/root-network-guide.

The production README is the consumer-facing contract. These checks establish
its source facts and links, not node execution, recovery or public availability.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
RECORD = Path(__file__).with_name("repair_22_33_869.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading):
    readme = source("README.md")
    marker = re.compile(r"^(#{2,3}) " + re.escape(heading) + r"$", re.M)
    matches = list(marker.finditer(readme))
    assert len(matches) == 1, heading
    match = matches[0]
    lines = []
    fence = None
    for line in readme[match.end():].splitlines(keepends=True):
        delimiter = re.match(r"^ {0,3}(`{3,}|~{3,})(.*)$", line)
        if delimiter:
            ticks, suffix = delimiter.groups()
            if fence is None:
                fence = ticks
            elif ticks[0] == fence[0] and len(ticks) >= len(fence) and not suffix.strip():
                fence = None
        elif fence is None and re.match(r"^#{1," + str(len(match[1])) + r"} ", line):
            break
        lines.append(line)
    return "".join(lines)


def test_root_network_guide_merged_paxd_and_finality():
    overview = section("What is Paxeer X Network?")
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface", "outside consensus",
        "durable WAL for recovery",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    readme = source("README.md")
    for stale in (
        r"\blayerxd\b", r"two (?:execution )?domains",
        r"merge is (?:only )?a draft", r"kernel still runs separately",
    ):
        assert not re.search(stale, readme, re.I), stale


def test_root_network_guide_root_rpc_and_hosted_gateway():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "root path of these hosts, without a route suffix",
        "`https://api1.mainnet-beta.paxeer.network` through",
        "`https://api14.mainnet-beta.paxeer.network`",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
        "gateway contract does not establish method availability at public node URLs",
        "Obtain its deployment URL and credentials from the operator",
    ):
        assert fact in quickstart, fact
    urls = re.findall(r"https?://[^\s`<>)]+", source("README.md"))
    public_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    for url in urls:
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.hostname in public_hosts
            assert parsed.scheme == "https" and parsed.path in ("", "/")
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    command, expected = blocks[0].split("\n# Expected result: ")
    args = shlex.split(command.replace("\\\n", " "))
    assert args[0] == "curl"
    assert [arg for arg in args if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert json.loads(args[args.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert json.loads(expected) == "0x7d"
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")
    contract = "services/platform/hosted/gateway/openrpc.json"
    assert f"]({contract})" in quickstart
    methods = {method["name"] for method in json.loads(source(contract))["methods"]}
    assert {"px_getBalances", "px_getReceiptKeys"} <= methods


def test_root_network_guide_transaction_to_committed_receipt():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "EIP-1559 transaction for chain ID `125`", "`eth_getTransactionCount`",
        "`eth_sendRawTransaction`", "transaction hash", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
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
    receipts = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipts) == [
        "submitted", "receipted", "committed"
    ]
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipts
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipts
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipts


def test_root_network_guide_native_funding_and_recovery_links():
    accounts = section("Accounts and native funding")
    for fact in (
        "Native sub-accounts map to derived bank addresses inside the node",
        "`0x101A`", "Binding does not fund an account",
        "Account ID derivation alone does not open or fund an account",
        "authorized issuer mint", "chain ID `125` with zero EVM `value`",
        "Wait for commit and read `balanceOf`", 'eth_getBalance(address,"latest")',
        "custody deposit and credit-attestation funding flow is retired",
        "EIP-712/HKDF-SHA256", "preserve the original derivation context",
    ):
        assert fact in accounts, fact
    for path in (
        "docs/wiki/PaymentsQuickstart.md", "docs/site/docs/concepts/one-account.md",
    ):
        assert f"]({path})" in accounts
        assert (ROOT / path).is_file()
    methods = {
        item["name"]: item
        for item in json.loads(source("precompiles/layerxledger/abi.json"))
        if item["type"] == "function"
    }
    signatures = re.findall(r"\| `([A-Za-z]+)\(([^)]*)\)` \|", accounts)
    assert {name for name, _ in signatures} == {
        "openAccount", "accountId", "mint", "payExact", "balanceOf"
    }
    for name, args in signatures:
        assert args.split(",") == [item["type"] for item in methods[name]["inputs"]]
    assert "return address.Module(bankModuleName, accountID[:])" in source(
        "modules/lxphost/accountmap.go"
    )


def test_root_network_guide_retired_executors_and_merged_build():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "historical views retire at `v7.0-anchor-store-delete`",
        "archive-only", "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    assert shlex.split(blocks[0], comments=True) == ["make", "paxd-build"]
    assert shlex.split(blocks[1].splitlines()[1]) == [
        "./build/paxd", "start", "--home", "$PAXD_HOME"
    ]
    for fact in (
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    assert "\npaxd-build: paxd-native-libs\n" in source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build\n" in source("Makefile")
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    upgrades = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in upgrades
    assert "commits it with the BFT chain state" in upgrades


def test_root_network_guide_record_names_all_proofs_and_scope():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["task"]["id"] == "22.33.869"
    finding = record["finding"]["root-network-guide"]
    assert finding["audit"] == "22.30.182"
    assert finding["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert record["task"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_869.py"
    )
    assert record["task"]["baseline_tests_run"] is False
    assert record["task"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert record["task"]["review_only"]
    assert record["task"]["wiring_requests"] == []
