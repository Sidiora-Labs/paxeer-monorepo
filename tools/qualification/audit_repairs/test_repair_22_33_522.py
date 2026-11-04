"""Offline source proof for audit 22.30.153/readme-separate-daemon.

Read the actual README and production sources; do not claim native execution,
endpoint availability, consensus, cryptographic verification or recovery.
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
RECORD = Path(__file__).with_name("repair_22_33_522.kvx")


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


def test_readme_describes_one_embedded_paxd_network():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "`modules/lxphost`", "`node/sequencer`",
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
    assert not re.search(r"\blayerxd\b|cmd/layerxd|two execution domains", README, re.I)


def test_root_rpc_hosts_and_chain_probe_are_current():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    hosts = {f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)}
    hosts.add("api-mainnet-beta.paxeer.network")
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.hostname in hosts, url
            assert parsed.scheme == "https" and parsed.path in ("", "/"), url
            assert not parsed.query and not parsed.fragment, url
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
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
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_hosted_px_contract_is_scoped_to_its_server():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "services/platform/hosted/gateway/openrpc.json",
        "Obtain its deployment URL and credentials from the operator",
        "gateway contract does not establish method availability at public node URLs",
        "`px_getReceiptKeys` is registered by the node",
        "native `px_` account/history and receipt-key methods",
    ):
        assert fact in quickstart, fact
    methods = {
        method["name"]
        for method in json.loads(source("services/platform/hosted/gateway/openrpc.json"))["methods"]
    }
    assert {"px_getBalances", "px_getReceiptKeys"} <= methods
    server = source("rpc/server.go")
    assert '{Namespace: "px", Service: pxAPI}' in server
    assert '{Namespace: "px", Service: &pxHistoryAPI{account: pxAPI}}' in server
    assert "Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}" in server
    assert "outside consensus" in section("Integrated network topology")


def test_transaction_and_committed_receipt_follow_production_sources():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "EIP-1559 transaction for chain ID `125`", "`eth_getTransactionCount`",
        "`eth_sendRawTransaction`", "transaction hash", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`pax_sequencerReceipt`",
        "`submitted` has no execution receipt yet", "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height", "No separate service endpoint is required",
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
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == ["submitted", "receipted", "committed"]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in rpc
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in rpc
    assert "A receipt is final once its block is committed" in README
    proof = section("Receipt and proof verification")
    assert "Neither a transaction hash nor a signed execution receipt alone establishes block commitment" in proof
    assert "authenticate the block header and CometBFT commit against trusted validator information" in proof


def test_diagrams_keep_execution_and_commitment_in_one_network():
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert len(diagrams) == 2
    topology, sequence = diagrams
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native", "Validators -->|Commit| State",
    ):
        assert fact in topology, fact
    ordered = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL", "Release signed execution receipt",
        "receipted (execution evidence)", "Propose MsgSequenceBatch in receipt order",
        "Commit block", "committed, blockNumber, blockHash",
    )
    positions = [sequence.index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_precompile_overview_matches_merged_activation_and_retirement():
    native = section("EVM and Native Precompiles")
    documented = re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native)
    table = re.search(
        r"var MergeAddressTable = \[\]MergeAddress\{(.*?)\n\}",
        source("precompiles/setup.go"), re.S,
    )
    assert table is not None
    production = re.findall(r'HexToAddress\("(0x[0-9A-Fa-f]+)"\), "([^"]+)"', table[1])
    assert len(documented) == len(production) == 13
    assert [(int(address, 16), name) for address, name in documented] == [
        (int(address, 16), name) for address, name in production
    ]
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")
    assert "SetUpgradeHandler(MergeUpgrade, app.mergeUpgradePlan)" in source("node/upgrades.go")
    for fact in (
        "merge plan is `v7.0`", "gates activation by upgrade and module state",
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads before store deletion are archive-only",
        "archived settlement witness formats", "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert "custody deposit and credit-attestation funding flow is retired" in README
    build = section("Build")
    assert "make paxd-build" in build
    assert "running network entry point is `paxd`" in build
    assert "./build/paxd start" in build


def test_repair_record_maps_the_finding_to_declared_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["readme-separate-daemon"]
    audit = source("spec/layerx-merge/programme/audit/records/22.30.153.kvx")
    block = re.search(r"^\[finding\.readme-separate-daemon\]\n(.*?)(?=^\[|\Z)", audit, re.M | re.S)
    assert block is not None
    original = tomllib.loads(block[1])
    assert original["file"] == finding["file"] == "README.md"
    assert original["class"] == "doc-drift" and original["severity"] == "medium"
    assert finding["audit"] == "22.30.153" and finding["owner_task"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_repair_record_maps_the_finding_to_declared_proofs"
    }
    task = record["repair"]
    assert task["task"] == "22.33.522"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_522.py"
    assert task["wiring_requests"] == []
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert record["wiring"]["readme"]["path"] == "README.md"
    assert record["wiring"]["readme"]["owner_domain"] == 14
    assert record["wiring"]["readme"]["status"] == "satisfied-by-prerequisite"
