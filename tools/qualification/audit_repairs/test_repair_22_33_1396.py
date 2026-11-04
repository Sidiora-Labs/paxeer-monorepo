"""Offline source proof for audit 22.30.77/root-overview; no native execution."""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Extract a section, preserving headings and comments inside code fences."""
    readme = source("README.md")
    marker = "#" * level + " " + heading + "\n"
    assert readme.count(marker) == 1, heading
    lines = []
    fence = None
    for line in readme.split(marker, 1)[1].splitlines(keepends=True):
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


def test_root_overview_embeds_kernel_in_one_paxd_bft_network():
    overview = section("What is Paxeer X Network?", 2)
    assert_facts(overview, (
        "one BFT network", "EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface",
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State[Committed block and chain state]",
    ))
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_root_json_rpc_range_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert_facts(quickstart, (
        "root path of these hosts, without a route suffix",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "`node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ))
    allowed_hosts = {"api-mainnet-beta.paxeer.network"} | {
        f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)
    }
    urls = re.findall(r"https?://[^\s`<>)]+", source("README.md"))
    node_urls = [urlsplit(url) for url in urls if "mainnet-beta.paxeer.network" in url]
    assert node_urls
    for url in node_urls:
        assert url.scheme == "https" and url.hostname in allowed_hosts
        assert url.path in ("", "/") and not url.query and not url.fragment
        assert url.port is None and url.username is None and url.password is None
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


def test_transaction_producer_transport_and_committed_receipt_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    assert_facts(flow, (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "Programs payload bytes go to `0x101D`",
        "`eth_sendRawTransaction`", "transaction hash", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "commitment does not mean execution succeeded", "`eth_getTransactionReceipt`",
        "`eth_call` at an explicit committed height",
    ))
    send = source("rpc/send.go")
    assert_facts(send, (
        "func (s *SendAPI) SendRawTransaction(",
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ))
    accepter = source("node/sequencer/accepter.go")
    assert accepter.index("a.rep.Release(context.Background(), &wal.Accept{") < accepter.index(
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
    sequence = re.findall(r"```mermaid\n(sequenceDiagram.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    steps = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL", "Release signed execution receipt",
        "receipted (execution evidence)", "Propose MsgSequenceBatch in receipt order",
        "Commit block", "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    )
    positions = [sequence[0].index(step) for step in steps]
    assert positions == sorted(positions)


def test_execution_signature_and_block_commitment_have_distinct_trust():
    proof = section("Receipt and proof verification")
    assert_facts(proof, (
        "ProtocolReceipt v2", "pinned for the receipt height",
        "public key supplied beside an untrusted receipt does not establish trust",
        "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit against trusted validator information",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ))
    assert "ed25519.Sign(k.private, digest[:])" in source("node/sequencer/receiptkey.go")
    assert "codec.DecodeReceipt(receiptBytes)" in source("layerxproof/verify/receipt.go")
    assert "self.verify_header_commit(header, commit)?" in source(
        "programs/crates/layerx-programs-registry/src/chain_proof/head_verify.rs"
    )


def test_custody_checkpoint_executors_are_retired_and_bringup_is_paxd():
    native = section("EVM and Native Precompiles")
    assert_facts(native, (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces retained for compatibility, not active funding or checkpoint executors",
        "Historical anchor reads before store deletion are archive-only",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
    ))
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in source(
        "precompiles/layerxanchor/layerxanchor.go"
    )
    funding = section("Accounts and native funding")
    assert "custody deposit and credit-attestation funding flow is retired" in funding
    assert "chain ID `125` with zero EVM `value`" in funding
    build = section("Build")
    assert_facts(build, (
        "make paxd-build", './build/paxd start --home "$PAXD_HOME"',
        "running network entry point is `paxd`",
    ))
    assert "\npaxd-build: paxd-native-libs\n" in source("Makefile")
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    for stale in (
        r"\blayerxd\b", r"\bCheckpointRegistry\b", r"\bGuarantorBond\b",
        r"/rpc(?:/|\b)", r"two (?:execution )?domains",
    ):
        assert not re.search(stale, source("README.md"), re.I), stale


def test_repair_record_maps_root_overview_to_declared_proofs():
    record = tomllib.loads(source("tools/qualification/audit_repairs/repair_22_33_1396.kvx"))
    finding = record["finding"]["root-overview"]
    assert finding["audit"] == "22.30.77"
    assert finding["file"] == "README.md" and finding["owner_task"] == "22.31.470"
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.77.kvx"))
    assert finding["proposal"] == audit["finding"]["root-overview"]["proposal"]
    tree = ast.parse(Path(__file__).read_text(encoding="utf-8"))
    tests = {node.name for node in tree.body if isinstance(node, ast.FunctionDef)
             and node.name.startswith("test_")}
    assert set(finding["tests"]) == tests - {"test_repair_record_maps_root_overview_to_declared_proofs"}
    assert record["task"]["22"]["33"]["1396"]["wiring_requests"] == []
    assert "not executed" in record["task"]["22"]["33"]["1396"]["review_only"]
