"""Offline source proof for audit 22.30.92/doc_9 (decision 2525).

Read the production README owned by 22.31.470 and its node source references.
These assertions qualify documentation, not native execution or deployment.
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
RECORD = Path(__file__).with_name("repair_22_33_1524.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep fenced examples intact while finding the next peer heading."""
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


def test_doc_9_integrated_paxd_and_native_constructs():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "runs inside `paxd` through",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "subgraph Node[paxd]",
        "Native bank, ledger, payments, authority and programs",
        "RPC --> Order --> Host --> Native",
    ):
        assert fact in overview, fact
    native = section("EVM and Native Precompiles")
    rows = dict(re.findall(r"^\| `(0x[0-9A-F]+)` \| `([^`]+)` \|$", native, re.M))
    assert {
        address: rows[address]
        for address in ("0x101A", "0x101B", "0x101C", "0x101D", "0x101E")
    } == {
        "0x101A": "layerxledger",
        "0x101B": "layerxpay",
        "0x101C": "layerxauth",
        "0x101D": "layerxprograms",
        "0x101E": "layerxchains",
    }
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    build = section("Build")
    assert "make paxd-build" in build
    assert './build/paxd start --home "$PAXD_HOME"' in build
    assert not re.search(r"\blayerxd\b", README, re.I)


def test_doc_9_chain_125_and_root_rpc_hosts():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
    ):
        assert fact in quickstart, fact
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    command = blocks[0].split("\n# Expected result:", 1)[0].replace("\\\n", " ")
    argv = shlex.split(command)
    assert argv[0] == "curl"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = [
        urlsplit(url) for url in re.findall(r"https?://[^\s`<>)]+", README)
        if urlsplit(url).hostname.endswith("mainnet-beta.paxeer.network")
    ]
    assert urls
    for url in urls:
        assert url.hostname in allowed_hosts
        assert url.scheme == "https" and url.path in ("", "/")
        assert not url.query and not url.fragment and url.port is None
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_doc_9_transaction_transport_and_committed_receipt():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`eth_chainId` returns `0x7d`",
        "`eth_getTransactionCount`",
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_sendRawTransaction` to a public node's root JSON-RPC URL",
        "transaction hash",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block",
        "`MsgSequenceBatch` covers that sequence",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "No separate service endpoint is required",
    ):
        assert fact in flow, fact
    sequence = re.search(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert sequence is not None
    events = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    )
    positions = [sequence[1].index(event) for event in events]
    assert positions == sorted(positions)
    send = source("rpc/send.go")
    for fact in (
        "func (s *SendAPI) SendRawTransaction(",
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ):
        assert fact in send, fact
    accepter = source("node/sequencer/accepter.go")
    assert accepter.index("a.rep.Release(context.Background(), &wal.Accept{") < accepter.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "if err != nil || !committed {\n\t\treturn result, err" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt


def test_doc_9_finality_requires_authenticated_block_commit():
    assert "A receipt is final once its block is committed" in section(
        "What is Paxeer X Network?", 2
    )
    proof = section("Receipt and proof verification")
    for fact in (
        "signed ProtocolReceipt v2 bytes",
        "must be pinned for the receipt height",
        "a public key supplied beside an untrusted receipt does not establish trust",
        "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit against trusted validator information",
        "verify transaction or state inclusion against the authenticated root",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ):
        assert fact in proof, fact


def test_doc_9_upgrades_retire_custody_and_checkpoint_execution():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "Historical anchor reads before store deletion are archive-only",
        "archived settlement witness formats",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    releases = section("Releases and Network Upgrades")
    for fact in (
        "executes kernel activity inside `paxd`",
        "commits it with the BFT chain state",
        "`v7.0` merge handler activates the merged interfaces",
        "retires the old custody/checkpoint write paths",
        "`v7.0-anchor-store-delete` removes the historical anchor store",
    ):
        assert fact in releases, fact
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source(
        "node/upgrades.go"
    )
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for paragraph in README.split("\n\n"):
        if re.search(r"\blayerx(?:Verify|Custody|Anchor)\b", paragraph):
            assert "pre-merge" in paragraph and "retired" in paragraph, paragraph


def test_doc_9_record_maps_finding_and_declares_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["doc_9"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.92.kvx"))
    original = audit["finding"]["doc_9"]
    assert finding["audit"] == "22.30.92"
    for key in ("file", "line", "class", "severity", "observed", "expected", "proposal", "owner_domain"):
        assert finding[key] == original[key], key
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["1524"]
    assert task["readme_owner_task"] == "22.31.470"
    assert task["wiring_requests"] == []
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1524.py"
    )
    assert task["build_cmd"] == "python3 --version"
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["verified_revision"]
