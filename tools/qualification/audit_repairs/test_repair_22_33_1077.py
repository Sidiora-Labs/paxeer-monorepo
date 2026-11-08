"""Offline source proof for audit 22.30.43/f025, consuming task 22.31.470's README.

These checks qualify documentation and source connections, not native execution.
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
RECORD = Path(__file__).with_name("repair_22_33_1077.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Read headings outside fenced examples, preserving shell comments."""
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


def test_f025_embedded_paxd_is_one_block_committing_network():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "C17 kernel in `src/` and `include/` runs inside `paxd`",
        "through [`modules/lxphost`](modules/lxphost/)",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
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
        "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in topology[0], fact


def test_f025_root_rpc_hosts_and_chain_probe_are_unambiguous():
    quickstart = section("Quick start: connect to chain 125")
    assert "root path of these hosts, without a route suffix" in quickstart
    assert "`https://api1.mainnet-beta.paxeer.network` through " in quickstart
    assert "`https://api14.mainnet-beta.paxeer.network`" in quickstart
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    rpc_urls = [urlsplit(url) for url in urls if "mainnet-beta.paxeer.network" in url]
    assert rpc_urls
    for url in rpc_urls:
        assert url.scheme == "https"
        assert url.hostname in {
            "api-mainnet-beta.paxeer.network",
            *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
        }
        assert url.path in ("", "/")
        assert not url.query and not url.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
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


def test_f025_hosted_px_contract_is_distinct_from_public_node_methods():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "hosted gateway's `px_` API",
        "services/platform/hosted/gateway/openrpc.json",
        "Obtain its deployment URL and credentials from the operator",
        "gateway contract does not establish method availability at public node URLs",
        "`px_getReceiptKeys` is registered by the node",
        "native `px_` account/history and receipt-key methods",
    ):
        assert fact in quickstart, fact
    contract = json.loads(source("services/platform/hosted/gateway/openrpc.json"))
    methods = {method["name"] for method in contract["methods"]}
    assert {"px_getBalances", "px_getReceiptKeys"} <= methods
    server = source("rpc/server.go")
    assert '{Namespace: "px", Service: pxAPI}' in server
    assert '{Namespace: "px", Service: &pxHistoryAPI{account: pxAPI}}' in server
    assert 'Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}' in server


def test_f025_transaction_to_committed_receipt_matches_source_connections():
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
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(diagrams) == 1
    sequence = diagrams[0]
    order = [
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    ]
    positions = [sequence.index(event) for event in order]
    assert positions == sorted(positions)
    proof = section("Receipt and proof verification")
    assert "pinned for the receipt height" in proof
    assert "authenticate the block header and CometBFT commit" in proof
    assert "Neither a transaction hash nor a signed execution receipt alone establishes block commitment" in proof
    assert "codec.DecodeReceipt(receiptBytes)" in source("layerxproof/verify/receipt.go")


def test_f025_custody_and_checkpoint_executors_are_retired_refusal_surfaces():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for stale in (
        r"\blayerxd\b", r"two (?:execution )?domains",
        r"merge is (?:only )?a draft", r"kernel still runs separately",
        r"https?://api\d*\.mainnet-beta\.paxeer\.network/(?:rpc|evm|layerx)\b",
    ):
        assert not re.search(stale, README, re.I), stale


def test_f025_supported_build_tools_and_layout_match_the_merged_tree():
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    commands = [
        shlex.split(line, comments=True)
        for block in blocks for line in block.splitlines() if line.strip()
    ]
    assert ["make", "paxd-build"] in commands
    assert ["./build/paxd", "version"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands
    assert ["make", "build"] in commands
    makefile = source("Makefile")
    assert re.search(r"^paxd-build: paxd-native-libs$", makefile, re.M)
    assert re.search(r"^paxd-native-libs: \$\(LIBRARY\) programs-release-build evmone-lib$", makefile, re.M)
    chain = source("chain.mk")
    assert re.search(r"^build: paxd-native-libs$", chain, re.M)
    assert "go build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd" in chain
    for archive in ("build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a"):
        assert archive in build and archive in chain
    assert "running network entry point is `paxd`" in build
    assert "Source directory names describe code organization, not separate network deployments" in README
    layout = section("Directory Structure", 2)
    rows = re.findall(r"^\| (`[^|]+`) \|", layout, re.M)
    assert rows
    for cell in rows:
        for path in re.findall(r"`([^`]+)`", cell):
            assert (ROOT / path).is_dir(), path
    for path in ("src", "include", "cmd", "modules", "precompiles", "node", "rpc", "programs"):
        assert f"`{path}`" in layout, path
        assert (ROOT / path).is_dir(), path
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release


def test_f025_repair_record_names_all_proofs_and_honest_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["f025"]
    assert finding["audit"] == "22.30.43"
    assert finding["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_f025_repair_record_names_all_proofs_and_honest_evidence_limits"
    }
    task = record["repair"]
    assert task["task"] == "22.33.1077"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1077.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert task["wiring_requests"] == []
