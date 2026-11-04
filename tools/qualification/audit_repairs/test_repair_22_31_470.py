"""Offline README contract checks against the integrated node's production sources.

This gate qualifies documentation, not native execution or endpoint availability.
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
RECORD_PATH = Path(__file__).with_name("repair_22_31_470.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Read a Markdown section without interpreting fenced code as headings."""
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


def abi_methods(path):
    return {
        item["name"]: item
        for item in json.loads(source(path))
        if item["type"] == "function"
    }


def test_single_network_embeds_the_block_proposing_kernel():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
    ):
        assert fact in overview
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    assert "no router and no separate LayerX interface" in overview
    assert "outside consensus" in overview
    assert "durable WAL for recovery" in overview


def test_root_rpc_hosts_chain_probe_and_library_mirror():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert "api1.mainnet-beta.paxeer.network" in quickstart
    assert "through `https://api14.mainnet-beta.paxeer.network`" in quickstart
    assert "https://api-mainnet-beta.paxeer.network" in quickstart
    assert "`test` is restricted to non-live chain IDs" in quickstart
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
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
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    assert "compare `eth_blockNumber`" in quickstart
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.scheme == "https" and parsed.path in ("", "/")
            assert parsed.hostname in {
                "api-mainnet-beta.paxeer.network",
                *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
            }
    stack = source("rpc/rpcstack.go")
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in stack


def test_hosted_contract_and_native_px_registrations_are_distinguished():
    quickstart = section("Quick start: connect to chain 125")
    assert "gateway contract does not establish method availability at public node URLs" in quickstart
    assert "Obtain its deployment URL and credentials from the operator" in quickstart
    assert "`px_getReceiptKeys` is registered by the node" in quickstart
    assert "gateway relays its governed key ranges" in quickstart
    assert "native `px_` account/history and receipt-key methods" in quickstart
    methods = {
        method["name"]
        for method in json.loads(source("services/platform/hosted/gateway/openrpc.json"))["methods"]
    }
    assert {"px_getBalances", "px_getReceiptKeys"} <= methods
    server = source("rpc/server.go")
    assert '{Namespace: "px", Service: pxAPI}' in server
    assert '{Namespace: "px", Service: &pxHistoryAPI{account: pxAPI}}' in server
    assert 'Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}' in server
    assert "config.EnableTestAPI && !evmCfg.IsLiveChainID(ctx)" in server


def test_submission_and_committed_receipt_follow_real_node_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "`0x101A`–`0x101D`", "Programs payload bytes go to `0x101D`",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet", "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "served by sequencer", "commitment does not mean execution succeeded",
        "`eth_getTransactionReceipt`", "`eth_call` at an explicit committed height",
    ):
        assert fact in flow
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    rpc = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == ["submitted", "receipted", "committed"]
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in rpc
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in rpc
    assert "return SequencerReceiptMethod + \" served by sequencer\"" in rpc


def test_diagrams_keep_execution_inside_the_network_and_commit_finality():
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert len(diagrams) == 2
    topology, sequence = diagrams
    assert "subgraph Network[One Paxeer BFT network / EVM chain 125]" in topology
    assert "subgraph Node[paxd]" in topology
    assert "Embedded C17 kernel and Rust programs host" in topology
    assert "RPC --> Order --> Host --> Native" in topology
    assert "Proposed block in execution order" in topology
    assert "Validators -->|Commit| State" in topology
    assert "Optional hosted developer gateway" in topology
    ordered = [
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    ]
    positions = [sequence.index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_binding_native_funding_and_recovery_facts_are_preserved():
    accounts = section("Accounts and native funding")
    for fact in (
        "secp256k1", "did:layerx:<64 lowercase hex characters of the Ed25519 public key>",
        "`0x1004`", '"LX:PAXEER-BIND:v1" || chainId:uint256be || evmAddress:20 || nonce:uint64be',
        "64-byte Ed25519 signature", "EVM transaction supplies the address owner's consent",
        "Binding does not fund an account", "EIP-712/HKDF-SHA256",
        "preserve the original derivation context", "Native sub-accounts map to derived bank addresses",
        "`0x101A`", "Account ID derivation alone does not open or fund an account",
        "authorized issuer mint", "funded account owned by the bound caller",
        "chain ID `125` with zero EVM `value`", "expiry in Unix milliseconds",
        "Wait for commit and read `balanceOf`", 'eth_getBalance(address,"latest")',
        "custody deposit and credit-attestation funding flow is retired",
    ):
        assert fact in accounts
    binding = abi_methods("precompiles/addr/abi.json")
    for name, args in (
        ("layerXBindNonce", ["address"]), ("bindLayerX", ["bytes32", "bytes"]),
        ("getLayerXDid", ["address"]), ("getUnifiedAccount", ["address"]),
    ):
        assert name in accounts
        assert [item["type"] for item in binding[name]["inputs"]] == args
    ledger = abi_methods("precompiles/layerxledger/abi.json")
    signatures = re.findall(r"\| `([A-Za-z]+)\(([^)]*)\)` \|", accounts)
    assert {name for name, _ in signatures} == {"openAccount", "accountId", "mint", "payExact", "balanceOf"}
    for name, args in signatures:
        assert args.split(",") == [item["type"] for item in ledger[name]["inputs"]]
        assert ledger[name]["stateMutability"] == (
            "view" if name in {"accountId", "balanceOf"} else "nonpayable"
        )
    assert "return address.Module(bankModuleName, accountID[:])" in source("modules/lxphost/accountmap.go")


def test_signed_execution_and_authenticated_commit_proofs_are_preserved():
    proof = section("Receipt and proof verification")
    for fact in (
        "ProtocolReceipt v2", "separate from the validator consensus key",
        "pinned for the receipt height", "public key supplied beside an untrusted receipt does not establish trust",
        "`committed` rung requires coverage", "`MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit", "trusted validator information",
        "authenticated root at the relevant height and key",
    ):
        assert fact in proof
    assert "ed25519.Sign(k.private, digest[:])" in source("node/sequencer/receiptkey.go")
    assert "codec.DecodeReceipt(receiptBytes)" in source("layerxproof/verify/receipt.go")
    root = "programs/crates/layerx-programs-registry/src/chain_proof/"
    assert "self.verify_header_commit(header, commit)?" in source(root + "head_verify.rs")
    assert "proof.verify(&head.data_hash(), &hash)?" in source(root + "tx.rs")
    assert "verify_store_root(store_name, root, &ops[1].data, next.app_hash())?" in source(root + "state.rs")


def test_active_interfaces_retired_refusals_and_historical_formats():
    native = section("EVM and Native Precompiles")
    rows = dict(re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native))
    assert rows == {
        "0x101A": "layerxledger", "0x101B": "layerxpay", "0x101C": "layerxauth",
        "0x101D": "layerxprograms", "0x101E": "layerxchains", "0x0901": "orob",
        "0x0902": "clearing", "0x0903": "oracle aggregator", "0x0904": "pofq",
        "0x0905": "scheduler", "0x0906": "streams", "0x0907": "tee attestor", "0x0908": "eip712",
    }
    for fact in (
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "`0x1012`", "`0x1013`", "`0x1014`",
        "rejects writes after the merge", "historical views retire at `v7.0-anchor-store-delete`",
        "Historical anchor reads before store deletion are archive-only",
        "external native bridge at `0x1016` remains distinct", "`bridgeOut`",
        "retained [batch-header codec]", "archived settlement witness formats",
        "guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
        "serves `perps` at `0x1015` in place of the earlier `layerxExchange`",
    ):
        assert fact in native
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")


def test_retired_construct_names_and_routes_do_not_leak_into_current_guidance():
    # The three retained ABI names and source links are required historical
    # compatibility facts; permit them only in the single explicit refusal paragraph.
    retired_paragraphs = [p for p in README.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(retired_paragraphs) == 1
    paragraph = retired_paragraphs[0]
    assert "retired refusal surfaces" in paragraph and "archive-only" in paragraph
    current = README.replace(paragraph, "")
    retained_names = {
        "layerxverify", "layerxVerify", "layerxcustody", "layerxCustody",
        "layerxanchor", "layerxAnchor",
    }
    inventory = tomllib.loads(source("spec/layerx-merge/programme/data/retired.kvx"))
    for component in inventory["component"].values():
        for name in component["symbols"] + component["deleted_trees"]:
            pattern = r"(?<![A-Za-z0-9_])" + re.escape(name)
            if not name.endswith(("_", "/")):
                pattern += r"(?![A-Za-z0-9_])"
            assert not re.search(pattern, current), name
            if name not in retained_names:
                assert not re.search(pattern, paragraph), name
    for stale in (
        r"\blayerxd\b", r"\blx_[A-Za-z0-9_*]*", r"/rpc(?:/|\b)",
        r"two (?:execution )?domains", r"kernel still runs separately",
        r"merge is (?:only )?a draft", r"\bLNI\b",
    ):
        assert not re.search(stale, README, re.I), stale


def test_paxd_build_start_and_source_navigation_match_current_layout():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier", "make build",
        "running network entry point is `paxd`",
    ):
        assert fact in build
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release
    assert "maintained architecture entry point" in README
    assert "Source directory names describe code organization, not separate network deployments" in README
    for target in re.findall(r"\]\(([^)]+)\)", README):
        if not target.startswith(("https:", "http:", "#")):
            assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_repair_record_covers_every_medium_root_finding_and_names_its_proofs():
    record = tomllib.loads(RECORD_PATH.read_text(encoding="utf-8"))
    task = record["task"]["22"]["31"]["470"]
    expected = set()
    for path in (ROOT / "spec/layerx-merge/programme/audit/records").glob("*.kvx"):
        for block in re.split(r"(?=^\[)", path.read_text(encoding="utf-8"), flags=re.M):
            if re.search(r'^file = "README.md"$', block, re.M) and 'severity = "medium"' in block:
                finding = re.match(r"\[finding\.([^]]+)\]", block).group(1)
                expected.add((path.stem, finding))
    entries = record["coverage"].values()
    assert len(expected) == 97
    assert {(entry["audit"], entry["finding"]) for entry in entries} == expected
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    mapped = set()
    for entry in entries:
        assert entry["file"] == "README.md"
        assert entry["tests"] and set(entry["tests"]) <= declared
        mapped.update(entry["tests"])
    assert mapped == declared - {"test_repair_record_covers_every_medium_root_finding_and_names_its_proofs"}
    assert task["base_revision"] == "76bd4ca9414014055f61431cd0c7be0dc9ac5fe2"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_31_470.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert task["wiring_requests"] == []
    assert "22.33.33" in record["reconciliation"]["prior_work"]
    assert "rpc/server.go" in record["conflict"]["px_methods"]["authority"]
    assert "v7.0-anchor-store-delete" in record["conflict"]["anchor_reads"]["resolution"]
