"""Offline source proof for audit 22.30.179/doc-readme, owned by 22.31.470.

Read the actual README and production sources; do not invoke a native toolchain,
start a node, or infer runtime qualification from documentation assertions.
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
RECORD = Path(__file__).with_name("repair_22_33_824.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep code-fence comments from terminating a Markdown section."""
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


def test_doc_readme_merged_network_and_tenant_topology():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface",
        "Sub-chains are tenant domains stored in the database",
        "namespaces state within the shared network",
        "does not require new node infrastructure",
        "hosted gateway is optional developer tooling", "outside consensus",
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


def test_doc_readme_root_rpc_probe_and_mirror_distinction():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
        "gateway contract does not establish method availability at public node URLs",
    ):
        assert fact in quickstart, fact
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    command = blocks[0].split("\n# Expected result:", 1)[0].replace("\\\n", " ")
    argv = shlex.split(command)
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
    hosts = {f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)}
    hosts.add("api-mainnet-beta.paxeer.network")
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.scheme == "https" and parsed.hostname in hosts, url
            assert parsed.path in ("", "/") and not parsed.query and not parsed.fragment, url
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_doc_readme_transaction_to_committed_receipt_path():
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
    for call in (
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ):
        assert call in send, call
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in rpc
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in rpc
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    events = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "receipted (execution evidence)", "Propose MsgSequenceBatch in receipt order",
        "Commit block", "committed, blockNumber, blockHash",
    )
    positions = [sequence[0].index(event) for event in events]
    assert positions == sorted(positions)


def test_doc_readme_finality_requires_authenticated_commitment():
    assert "A receipt is final once its block is committed" in README
    proof = section("Receipt and proof verification")
    for fact in (
        "ProtocolReceipt v2", "pinned for the receipt height",
        "public key supplied beside an untrusted receipt does not establish trust",
        "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit against trusted validator information",
        "authenticated root at the relevant height and key",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ):
        assert fact in proof, fact
    assert "ed25519.Sign(k.private, digest[:])" in source("node/sequencer/receiptkey.go")
    verifier = source("layerxproof/verify/receipt.go")
    assert "codec.DecodeReceipt(receiptBytes)" in verifier
    assert "Ed25519Digest(sequencerPublicKey, *receipt.SequencerSignature, digest)" in verifier
    base = "programs/crates/layerx-programs-registry/src/chain_proof/"
    assert "self.verify_header_commit(header, commit)?" in source(base + "head_verify.rs")
    assert "proof.verify(&head.data_hash(), &hash)?" in source(base + "tx.rs")
    assert "verify_store_root(store_name, root, &ops[1].data, next.app_hash())?" in source(base + "state.rs")


def test_doc_readme_retired_executors_and_historical_guidance():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads before store deletion are archive-only",
        "archived settlement witness formats", "support historical interpretation",
        "guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for paragraph in README.split("\n\n"):
        if re.search(r"layerxAnchor|layerxCustody|layerxVerify|guarantor|L0–L4", paragraph):
            assert re.search(r"retired|historical|archived", paragraph), paragraph
    for stale in (
        r"\blayerxd\b", r"two (?:execution )?domains", r"kernel still runs separately",
        r"\blx_[A-Za-z0-9_*]*", r"/rpc(?:/|\b)",
    ):
        assert not re.search(stale, README, re.I), stale


def test_doc_readme_current_build_activation_and_repository_paths():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        './build/paxd start --home "$PAXD_HOME"',
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    upgrades = source("node/upgrades.go")
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in upgrades
    release = section("Releases and Network Upgrades")
    assert "`v7.0` merge handler activates the merged interfaces" in release
    assert "retires the old custody/checkpoint write paths" in release
    assert "Source directory names describe code organization, not separate network deployments" in README
    for target in re.findall(r"\]\(([^)]+)\)", README):
        if not target.startswith(("http:", "https:", "#")):
            assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_doc_readme_record_maps_finding_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["doc-readme"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.179.kvx"))
    original = audit["finding"]["doc-readme"]
    for key in ("file", "class", "severity", "expected"):
        assert finding[key] == original[key], key
    assert finding["audit"] == "22.30.179"
    assert finding["owner_task"] == "22.31.470" and finding["owner_domain"] == 17
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["824"]
    assert task["start_revision"] == "bd09b6fa16afd5f9f92bef280322d1be0dc3a5c2"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_824.py"
    assert task["wiring_requests"] == []
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
