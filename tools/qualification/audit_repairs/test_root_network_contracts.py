"""Offline source contracts for audit 22.30.147/root-network.

Read the production README and its linked implementations without contacting a
node, executing a toolchain, or substituting a simulated runtime.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
NETWORK_PAGE = ROOT / "docs/readme/NETWORK.md"
README = NETWORK_PAGE.read_text(encoding="utf-8")
RECORD = ROOT / "spec/layerx-merge/programme/records/tools/qualification/audit_repairs/repair_22_33_446.kvx"


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level):
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


def require_facts(text, facts):
    for fact in facts:
        assert fact in text, fact


def test_root_network_embeds_kernel_in_one_chain_125_paxd():
    overview = section("What is Paxeer X Network?", 2)
    require_facts(overview, (
        "The LayerX kernel runs inside every `paxd` node.", "chain ID **125** (`0x7d`)",
        "It executes each transaction when it arrives, assigns the next position "
        "in one global sequence, returns a signed execution receipt and builds "
        "each block.",
        "Validators check that block and commit it.", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
    ))
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    release = section("Releases and Network Upgrades", 3)
    require_facts(release, (
        "executes kernel activity inside `paxd`",
        "commits it with the BFT chain state",
        "retires the old custody/checkpoint write paths",
    ))
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert len(diagrams) == 2
    require_facts(diagrams[0], (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ))


def test_public_rpc_hosts_use_root_and_probe_chain_125():
    quickstart = section("Quick start: connect to chain 125", 3)
    require_facts(quickstart, (
        "Set `PAXEER_RPC_URL` to a node root URL",
        "Send HTTPS JSON-RPC requests to the root path without a route suffix",
        "Confirm the chain ID before signing or broadcasting",
    ))
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert "$PAXEER_RPC_URL" in argv
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_transaction_to_committed_receipt_matches_production_source_path():
    flow = section("Submit a transaction and follow its committed receipt", 3)
    require_facts(flow, (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`",
        "transaction hash", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`pax_sequencerReceipt`",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "authenticate the block commitment", "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ))
    send = source("rpc/send.go")
    require_facts(send, (
        "func (s *SendAPI) SendRawTransaction(",
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
    require_facts(receipt, (
        'const SequencerReceiptMethod = "pax_sequencerReceipt"',
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))",
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted",
    ))
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    ordered = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    )
    positions = [sequence[0].index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_hosted_px_contract_and_private_services_are_explicit():
    quickstart = section("Quick start: connect to chain 125", 3)
    require_facts(quickstart, (
        "Obtain its deployment URL and credentials from the operator",
        "gateway contract does not establish method availability at public node URLs",
        "`px_` API", "`px_getReceiptKeys` is registered by the node",
        "native `px_` account/history and receipt-key methods",
        "`test` is restricted to non-live chain IDs",
        "Mainnet-beta uses real value",
        "hosted funding and private test-environment faucets have their own access policies",
    ))
    overview = section("What is Paxeer X Network?", 2)
    require_facts(overview, (
        "hosted gateway is optional developer tooling",
        "outside consensus",
    ))
    contract = json.loads(source("services/platform/hosted/gateway/openrpc.json"))
    assert contract["openrpc"] == "1.3.2"
    methods = {method["name"] for method in contract["methods"]}
    assert {"px_getBalances", "px_getReceiptKeys", "pax_sequencerReceipt"} <= methods
    server = source("rpc/server.go")
    require_facts(server, (
        '{Namespace: "px", Service: pxAPI}',
        '{Namespace: "px", Service: &pxHistoryAPI{account: pxAPI}}',
        "Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}",
        "config.EnableTestAPI && !evmCfg.IsLiveChainID(ctx)",
    ))


def test_custody_and_checkpoint_executors_are_retired_not_deployments():
    native = section("EVM and Native Precompiles", 3)
    require_facts(native, (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "archive-only", "`v7.0-anchor-store-delete`",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ))
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert "custody deposit and credit-attestation funding flow is retired" in README
    for stale in (
        r"\blayerxd\b", r"\blx_[A-Za-z0-9_*]*", r"two (?:execution )?domains",
        r"https?://[^\s`]+mainnet-beta\.paxeer\.network/(?:rpc|evm|layerx)\b",
    ):
        assert not re.search(stale, README, re.I), stale
    retirement = [p for p in README.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(retirement) == 1
    current = README.replace(retirement[0], "")
    assert not re.search(r"layerx(?:Custody|Anchor|Verify)", current, re.I)


def test_readme_links_actual_node_and_hosted_contracts():
    links = {
        str((NETWORK_PAGE.parent / target.split("#", 1)[0]).resolve().relative_to(ROOT))
        for target in re.findall(r"\]\(([^)]+)\)", README)
        if not target.startswith(("https:", "http:", "#"))
    }
    expected = {
        "daemon/paxd", "modules/lxphost", "node/sequencer",
        "rpc/server.go", "rpc/send.go", "rpc/sequencer_receipt.go",
        "services/platform/hosted/gateway/openrpc.json",
        "docs/site/paxeer-docs/src/app/platform-api/page.tsx",
        "docs/site/paxeer-docs/src/app/public-rpc/page.tsx",
    }
    assert expected <= links
    for path in expected:
        assert (ROOT / path).exists(), path


def test_repair_record_maps_root_network_to_declared_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["repair"]
    assert task["task"] == "22.33.446"
    assert task["audit"] == "22.30.147"
    assert re.fullmatch(r"[0-9a-f]{40}", task["inspected_revision"])
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_root_network_contracts.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["verified_revision"]
    assert "not run" in task["baseline"]
    assert task["wiring_requests"] == []
    assert "22.31.470" in task["readme_owner"]
    finding = record["finding"]["root-network"]
    assert finding["file"] == "README.md"
    assert finding["class"] == "doc-drift"
    assert finding["severity"] == "medium"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_repair_record_maps_root_network_to_declared_proofs"
    }
    assert len(finding["tests"]) == len(set(finding["tests"]))
