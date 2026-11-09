"""Offline source contract for audit 22.30.70/docs-root-readme.

Consume the README owned by 22.31.470; do not execute its operator commands.
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
RECORD = Path(__file__).with_name("repair_22_33_1325.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep shell comments inside fenced examples out of the heading parser."""
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


def assert_facts(text, facts):
    for fact in facts:
        assert fact in text, fact


def test_docs_root_readme_single_paxd_network_and_commit_finality():
    overview = section("What is Paxeer X Network?", 2)
    assert_facts(overview, (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface",
    ))
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    release = section("Releases and Network Upgrades")
    assert_facts(release, (
        "executes kernel activity inside `paxd`",
        "commits it with the BFT chain state",
        "retires the old custody/checkpoint write paths",
    ))


def test_docs_root_readme_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert_facts(quickstart, (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through",
        "`https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ))
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    public = [urlsplit(url) for url in urls if "mainnet-beta.paxeer.network" in url]
    assert public
    allowed = {"api-mainnet-beta.paxeer.network"} | {
        f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)
    }
    for url in public:
        assert url.scheme == "https" and url.hostname in allowed
        assert url.path in ("", "/") and not url.query and not url.fragment
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


def test_docs_root_readme_hosted_contract_and_node_px_methods():
    quickstart = section("Quick start: connect to chain 125")
    assert_facts(quickstart, (
        "hosted gateway's `px_` API is defined by",
        "services/platform/hosted/gateway/openrpc.json",
        "gateway contract does not establish method availability at public node URLs",
        "`px_getReceiptKeys` is registered by the node",
        "native `px_` account/history and receipt-key methods",
    ))
    gateway = json.loads(source("services/platform/hosted/gateway/openrpc.json"))
    assert {"px_getBalances", "px_getReceiptKeys"} <= {
        method["name"] for method in gateway["methods"]
    }
    assert '{Namespace: "px", Service: pxAPI}' in source("rpc/server.go")


def test_docs_root_readme_transaction_to_committed_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    assert_facts(flow, (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_sendRawTransaction` to a public node's root JSON-RPC URL",
        "transaction hash", "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "inspect its status and logs",
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


def test_docs_root_readme_retired_executors_and_no_live_legacy_setup():
    native = section("EVM and Native Precompiles")
    assert_facts(native, (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
    ))
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for stale in (
        r"\blayerxd\b", r"kernel (?:currently|still) runs separately",
        r"merge is (?:only )?(?:a )?draft", r"/rpc(?:/|\b)",
    ):
        assert not re.search(stale, README, re.I), stale


def test_docs_root_readme_supported_build_and_runtime_commands():
    build = section("Build")
    assert_facts(build, (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "make paxeer-build", "make -f chain.mk build",
        "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "For kernel library and development tools without rebuilding the node",
        "The running network entry point is `paxd`",
    ))
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")


def test_docs_root_readme_record_maps_the_finding_to_declared_tests():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["docs-root-readme"]
    assert finding["audit"] == "22.30.70"
    assert finding["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_docs_root_readme_record_maps_the_finding_to_declared_tests"
    }
    task = record["task"]["22"]["33"]["1325"]
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1325.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert task["wiring_requests"] == []
    assert "not run" in task["baseline"]
    assert "not executed" in task["review_only"]
