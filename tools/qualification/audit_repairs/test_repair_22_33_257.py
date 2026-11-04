"""Offline source proof for audit 22.30.13/034 against the real root README.

The documentation consumes the integrated node's production contracts. These
checks do not execute the native node or contact the public RPC endpoints.
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
RECORD = Path(__file__).with_name("repair_22_33_257.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(title, level=3):
    """Extract a real heading and its children, ignoring fenced examples."""
    marker = "#" * level + " " + title + "\n"
    assert README.count(marker) == 1, title
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


def test_034_one_chain_125_embeds_kernel_in_paxd():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "C17 kernel in `src/` and `include/` runs inside `paxd`",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    assert re.search(r"\blayerxd\b", README, re.I) is None


def test_034_public_root_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "`https://api1.mainnet-beta.paxeer.network` through " in quickstart
    assert "`https://api14.mainnet-beta.paxeer.network`" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    allowed = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    public_urls = [
        urlsplit(url)
        for url in re.findall(r"https?://[^\s`<>)]+", README)
        if urlsplit(url).hostname in allowed
    ]
    assert {url.hostname for url in public_urls} >= {
        "api-mainnet-beta.paxeer.network", "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    assert all(url.scheme == "https" and url.path in ("", "/") for url in public_urls)
    examples = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(examples) == 1
    argv = shlex.split(examples[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert '# Expected result: "0x7d"' in examples[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_034_transaction_producer_transport_and_receipt_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "root JSON-RPC URL", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`pax_sequencerReceipt`",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
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
        "submitted", "receipted", "committed",
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt


def test_034_finality_requires_authenticated_block_commitment():
    overview = section("What is Paxeer X Network?", 2)
    assert "A receipt is final once its block is committed" in overview
    proof = section("Receipt and proof verification")
    for fact in (
        "A valid signature proves execution evidence",
        "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit against trusted validator information",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ):
        assert fact in proof, fact
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    topology = [diagram for diagram in diagrams if "flowchart LR" in diagram]
    sequence = [diagram for diagram in diagrams if "sequenceDiagram" in diagram]
    assert len(topology) == len(sequence) == 1
    assert "subgraph Node[paxd]" in topology[0]
    assert "Validators -->|Commit| State" in topology[0]
    events = [
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "receipted (execution evidence)", "Propose MsgSequenceBatch in receipt order",
        "Commit block", "committed, blockNumber, blockHash",
    ]
    positions = [sequence[0].index(event) for event in events]
    assert positions == sorted(positions)


def test_034_retired_custody_checkpoint_executors_are_refusal_surfaces():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
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


def test_034_current_repository_paths_and_merge_status():
    for path in (
        "daemon/paxd", "node/", "consensus/", "modules/", "rpc/",
        "modules/lxphost", "node/sequencer/", "programs/", "src/", "include/",
    ):
        assert path in README, path
        assert (ROOT / path).is_dir(), path
    releases = section("Releases and Network Upgrades")
    assert "merged runtime executes kernel activity inside `paxd`" in releases
    assert "commits it with the BFT chain state" in releases
    assert "`v7.0` merge handler activates the merged interfaces" in releases
    assert "retires the old custody/checkpoint write paths" in releases
    assert not re.search(r"\bmerge\b[^.\n]*\bdraft\b|\bdraft\b[^.\n]*\bmerge\b", README, re.I)
    assert "planned destinations under `core/` and `sdk/programs`" in README


def test_034_record_maps_proof_limits_and_external_revision_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["034"]
    assert finding["audit"] == "22.30.13"
    assert finding["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    assert finding["wiring_requests"] == []
    declared = {
        item.name for item in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(item, ast.FunctionDef) and item.name.startswith("test_034_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["257"]
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_257.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "No baseline failures are claimed" in task["baseline"]
    assert "not runtime" in task["review_only"]
