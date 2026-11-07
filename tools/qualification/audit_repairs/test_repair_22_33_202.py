"""Offline source qualification of audit 22.30.12/f048's README contract.

Read the maintained README and its production sources; do not execute native
components or imply that a documentation gate certifies a live network.
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
RECORD = Path(__file__).with_name("repair_22_33_202.kvx")


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


def test_f048_one_bft_network_embeds_kernel_and_commits_finality():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
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
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(diagrams) == 1
    topology = diagrams[0]
    assert "subgraph Network[One Paxeer BFT network / EVM chain 125]" in topology
    assert "subgraph Node[paxd]" in topology
    assert "RPC --> Order --> Host --> Native" in topology
    assert "Validators -->|Commit| State[Committed block and chain state]" in topology


def test_f048_root_rpc_hosts_and_chain_probe_are_concrete():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through",
        "`https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = [
        urlsplit(url) for url in re.findall(r"https?://[^\s`<>)]+", README)
        if "mainnet-beta.paxeer.network" in url
    ]
    assert urls
    for url in urls:
        assert url.hostname in allowed_hosts
        assert url.scheme == "https" and url.path in ("", "/")
        assert not url.query and not url.fragment and url.port is None
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    args = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert args[0] == "curl"
    assert args[args.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(args[args.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert [arg for arg in args if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_f048_hosted_contract_is_distinct_from_node_methods():
    quickstart = section("Quick start: connect to chain 125")
    contract = "services/platform/hosted/gateway/openrpc.json"
    assert contract in quickstart
    assert "Obtain its deployment URL and credentials from the operator" in quickstart
    assert "gateway contract does not establish method availability at public node URLs" in quickstart
    assert "native `px_` account/history and receipt-key methods" in quickstart
    assert "`px_getReceiptKeys` is registered by the node" in quickstart
    methods = {entry["name"] for entry in json.loads(source(contract))["methods"]}
    assert {"px_getReceiptKeys", "px_getBalances"} <= methods
    server = source("rpc/server.go")
    assert '{Namespace: "px", Service: pxAPI}' in server
    assert '{Namespace: "px", Service: &pxHistoryAPI{account: pxAPI}}' in server
    assert 'Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}' in server


def test_f048_transaction_receipt_journey_tracks_producer_transport_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "commitment does not mean execution succeeded",
        "`eth_getTransactionReceipt`", "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed",
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    positions = [sequence[0].index(event) for event in (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    )]
    assert positions == sorted(positions)


def test_f048_retired_executors_are_refusals_without_separate_deployments():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "`layerxAnchor` (`0x1014`)", "retired refusal surfaces",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "rejects writes after the merge", "historical views retire at `v7.0-anchor-store-delete`",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    custody = source("precompiles/layerxcustody/layerxcustody.go")
    assert "if p.keeper.Retired(ctx)" in custody
    assert "return nil, errors.New(RetiredRevert)" in custody
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    accounts = section("Accounts and native funding")
    assert "custody deposit and credit-attestation funding flow is retired" in accounts
    assert "Wait for commit and read `balanceOf` to confirm funding" in accounts
    for stale in (
        r"\blayerxd\b", r"/rpc(?:/|\b)", r"\blx_[A-Za-z0-9_*]*",
        r"two (?:execution )?domains", r"kernel still runs separately",
    ):
        assert not re.search(stale, README, re.I), stale
    build = section("Build")
    assert "make paxd-build" in build
    assert './build/paxd start --home "$PAXD_HOME"' in build
    assert "running network entry point is `paxd`" in build


def test_f048_record_maps_the_finding_proofs_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["f048"]
    assert finding["audit"] == "22.30.12"
    assert finding["file"] == "README.md"
    assert finding["owner_domain"] == 17
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    tests = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == tests
    task = record["task"]["22"]["33"]["202"]
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_202.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert task["wiring_requests"] == []
    assert "README.md" in record["wiring"]["readme"]["request"]
    assert "domain 17" in record["wiring"]["readme"]["request"]
    assert "22.31.470" in record["wiring"]["readme"]["request"]
