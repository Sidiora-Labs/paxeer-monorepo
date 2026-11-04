"""Offline source proof for audit 22.30.143/root-readme-topology."""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_414.kvx")


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


def test_root_readme_topology_is_one_embedded_paxd_network():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
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
    topology = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(topology) == 1
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in topology[0], fact


def test_root_readme_public_rpc_is_chain_125_at_root_hosts():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "compare `eth_blockNumber`", "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    node_urls = [urlsplit(url) for url in urls if "mainnet-beta.paxeer.network" in url]
    assert node_urls
    for url in node_urls:
        assert url.scheme == "https" and url.hostname in hosts
        assert url.path in ("", "/") and not url.query and not url.fragment
        assert url.port is None and url.username is None and url.password is None
    assert "https://api-mainnet-beta.paxeer.network" in quickstart
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_root_readme_transaction_to_committed_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`eth_getTransactionCount`", "EIP-1559 transaction for chain ID `125`",
        "`eth_sendRawTransaction`", "transaction hash", "root JSON-RPC URL",
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
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    positions = [sequence[0].index(fact) for fact in (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    )]
    assert positions == sorted(positions)
    proof = section("Receipt and proof verification")
    assert "Neither a transaction hash nor a signed execution receipt alone establishes block commitment" in proof
    assert "authenticate the block header and CometBFT commit against trusted validator information" in proof


def test_root_readme_separates_retired_executors_and_active_addresses():
    native = section("EVM and Native Precompiles")
    paragraphs = [p for p in native.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(paragraphs) == 1
    retired = paragraphs[0]
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge", "`layerxAnchor` (`0x1014`)",
        "rejects writes after the merge", "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "Historical anchor reads before store deletion are archive-only",
        "they do not provide current funding or finality paths",
    ):
        assert fact in retired, fact
    assert "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired" in native
    assert "archived formats add no settlement step to current block finality" in native
    rows = re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native)
    documented = {int(address, 16): name for address, name in rows}
    registered = {
        int(address, 16): name for address, name in re.findall(
            r'\{ecommon.HexToAddress\("(0x[0-9A-Fa-f]+)"\), "([^"]+)"\}',
            source("precompiles/setup.go")
        )
    }
    assert len(rows) == len(documented) == 13
    assert documented == registered
    assert not {0x1012, 0x1013, 0x1014} & documented.keys()
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_root_readme_upgrade_and_operating_guidance_is_merged():
    build = section("Build")
    assert "make paxd-build" in build
    assert './build/paxd start --home "$PAXD_HOME"' in build
    assert "running network entry point is `paxd`" in build
    upgrade = section("Releases and Network Upgrades")
    for fact in (
        "executes kernel activity inside `paxd`", "commits it with the BFT chain state",
        "`v7.0` merge handler activates the merged interfaces",
        "retires the old custody/checkpoint write paths",
        "`v7.0-anchor-store-delete` removes the historical anchor store",
    ):
        assert fact in upgrade, fact
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    for stale in (
        r"\blayerxd\b", r"kernel (?:still )?runs separately",
        r"merge is (?:only )?a draft", r"https?://[^\s`]+/rpc\b",
    ):
        assert not re.search(stale, README, re.I), stale


def test_root_readme_repair_record_maps_audit_finding_to_source_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]
    assert task["id"] == "22.33.414"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_414.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert task["wiring_requests"] == []
    finding = record["finding"]["root-readme-topology"]
    audit = source("spec/layerx-merge/programme/audit/records/22.30.143.kvx")
    block = audit.split("[finding.root-readme-topology]\n", 1)[1].split("\n[", 1)[0]
    original = tomllib.loads(block)
    assert finding["file"] == original["file"] == "README.md"
    assert finding["class"] == original["class"] == "doc-drift"
    assert finding["severity"] == original["severity"] == "medium"
    assert finding["expected"] == original["expected"]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_root_readme_repair_record_maps_audit_finding_to_source_proofs"
    }
    wiring = record["wiring"]["root_readme"]
    assert wiring["path"] == "README.md"
    assert wiring["owner_domain"] == original["owner_domain"] == 1
    assert wiring["owner_task"] == "22.31.470"
    assert wiring["status"] == "satisfied_by_prerequisite"
    assert wiring["missing_facts"] == []
