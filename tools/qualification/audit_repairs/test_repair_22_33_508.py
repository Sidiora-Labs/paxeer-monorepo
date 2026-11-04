"""Offline source proof for audit 22.30.151/doc-root-two-domains.

README is the production documentation delivered to developers. These checks
qualify its published contract and source links, not a running node or endpoint.
"""

import ast
import hashlib
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_508.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Extract a named section, ignoring headings inside fenced examples."""
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


def test_doc_root_two_domains_finished_paxd_network_and_commit_finality():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release
    for stale in (
        r"\blayerxd\b", r"two (?:execution )?domains",
        r"kernel still runs separately", r"merge is (?:only )?a draft",
        r"/rpc(?:/|\b)",
    ):
        assert not re.search(stale, README, re.I), stale


def test_doc_root_two_domains_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert (
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`"
    ) in quickstart
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    node_urls = [urlsplit(url) for url in urls if "mainnet-beta.paxeer.network" in url]
    assert node_urls
    for url in node_urls:
        assert url.scheme == "https" and url.hostname in allowed_hosts
        assert url.path in ("", "/") and not url.query and not url.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_doc_root_two_domains_transaction_transport_and_committed_receipt():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`",
        "public node's root JSON-RPC URL", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
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
        "submitted", "receipted", "committed",
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt
    sequence = re.findall(r"```mermaid\n(sequenceDiagram.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    ordered = [
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    ]
    positions = [sequence[0].index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_doc_root_two_domains_native_accounts_replace_custody_funding():
    accounts = section("Accounts and native funding")
    for fact in (
        "`bindLayerX(bytes32,bytes)`", "Binding does not fund an account",
        "Native sub-accounts map to derived bank addresses inside the node",
        "`0x101A`", "authorized issuer mint",
        "Account ID derivation alone does not open or fund an account",
        "chain ID `125` with zero EVM `value`", "expiry in Unix milliseconds",
        "Wait for commit and read `balanceOf`",
        "custody deposit and credit-attestation funding flow is retired",
    ):
        assert fact in accounts, fact
    abi = {
        item["name"]: item
        for item in json.loads(source("precompiles/layerxledger/abi.json"))
        if item["type"] == "function"
    }
    signatures = re.findall(r"\| `([A-Za-z]+)\(([^)]*)\)` \|", accounts)
    assert {name for name, _ in signatures} == {
        "openAccount", "accountId", "mint", "payExact", "balanceOf",
    }
    for name, args in signatures:
        assert args.split(",") == [item["type"] for item in abi[name]["inputs"]]
    assert "return address.Module(bankModuleName, accountID[:])" in source(
        "modules/lxphost/accountmap.go"
    )


def test_doc_root_two_domains_retired_executors_are_historical_only():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "Historical anchor reads before store deletion are archive-only",
        "External checkpoint registration", "L0–L4 settlement ladder are retired",
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
    for target in re.findall(r"\]\(([^)]+)\)", README):
        if not target.startswith(("https:", "http:", "#")):
            assert (ROOT / target.split("#", 1)[0]).exists(), target
    build = section("Build")
    assert "make paxd-build" in build
    assert './build/paxd start --home "$PAXD_HOME"' in build
    assert "running network entry point is `paxd`" in build


def test_repair_record_maps_finding_to_proofs_and_revision_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["doc-root-two-domains"]
    assert finding["audit"] == "22.30.151"
    assert finding["file"] == "README.md" and finding["owner_domain"] == 8
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["508"]
    assert task["verified_source_revision"] == "c2ff8c424e3d66a1fc57e4e6cbb26ce61367da13"
    assert task["readme_sha256"] == hashlib.sha256((ROOT / "README.md").read_bytes()).hexdigest()
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_508.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert task["missing_facts"] == []
    assert "README.md" in task["wiring_request"]
    assert "22.31.470" in task["wiring_request"]
    assert "domain 8" in task["wiring_request"]
