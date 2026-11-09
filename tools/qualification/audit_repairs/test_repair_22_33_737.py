"""Offline source proof for audit 22.30.171/021 against the owning README.

Read committed production sources; do not execute a node or contact an endpoint.
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
RECORD = Path(__file__).with_name("repair_22_33_737.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Extract a section, ignoring Markdown-like text inside code fences."""
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


def test_021_architecture_is_one_paxd_bft_network():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
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
        "subgraph Node[paxd]", "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native", "Validators -->|Commit| State",
    ):
        assert fact in topology[0], fact


def test_021_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert "`https://api1.mainnet-beta.paxeer.network` through " in quickstart
    assert "`https://api14.mainnet-beta.paxeer.network`" in quickstart
    urls = re.findall(r"https?://[^\s`<>)]+", quickstart)
    public_urls = [
        urlsplit(url) for url in urls
        if urlsplit(url).hostname.endswith("mainnet-beta.paxeer.network")
    ]
    assert {url.hostname for url in public_urls} == {
        "api-mainnet-beta.paxeer.network", "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    for url in public_urls:
        assert url.scheme == "https" and url.path in ("", "/")
        assert not url.query and not url.fragment and url.port is None
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
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert int("0x7d", 16) == 125
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_021_transaction_transport_and_committed_receipt_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`eth_getTransactionCount`", "EIP-1559 transaction for chain ID `125`",
        "`eth_sendRawTransaction`", "transaction hash", "integrated sequencing path",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`pax_sequencerReceipt`", "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "No separate service endpoint is required", "authenticate the block commitment",
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
    proof = section("Receipt and proof verification")
    assert "authenticate the block header and CometBFT commit" in proof
    assert "public key supplied beside an untrusted receipt does not establish trust" in proof
    assert "codec.DecodeReceipt(receiptBytes)" in source("layerxproof/verify/receipt.go")
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    steps = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    )
    positions = [sequence[0].index(step) for step in steps]
    assert positions == sorted(positions)


def test_021_retained_hosted_services_are_outside_consensus():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "agent daemon and MCP server", "interop gateway and bridge relayer",
        "hosted developer platform", "human control plane", "embedded wallet service",
        "explorer and launchpad services", "hosted gateway is optional developer tooling",
        "outside consensus", "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    quickstart = section("Quick start: connect to chain 125")
    assert "Obtain its deployment URL and credentials from the operator" in quickstart
    assert "gateway contract does not establish method availability at public node URLs" in quickstart
    assert "`px_getReceiptKeys` is registered by the node" in quickstart
    contract = json.loads(source("services/platform/hosted/gateway/openrpc.json"))
    assert {"px_getReceiptKeys", "px_getBalances"} <= {
        method["name"] for method in contract["methods"]
    }
    assert '{Namespace: "px", Service: pxAPI}' in source("rpc/server.go")


def test_021_custody_and_checkpoint_executors_are_retired():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge", "`layerxAnchor` (`0x1014`) rejects writes",
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
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    assert not re.search(r"\blayerxd\b|\blx_[A-Za-z0-9_*]+", README, re.I)
    assert "no separate kernel daemon, sequencer service or checkpoint deployment" in README


def test_021_build_and_release_guidance_targets_merged_paxd():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "running network entry point is `paxd`", "make agent-build",
        "make interop-build", "make platform-build", "make human-build",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    release = section("Releases and Network Upgrades")
    for fact in (
        "[CHANGELOG.md](CHANGELOG.md)", "executes kernel activity inside `paxd`",
        "commits it with the BFT chain state", "`v7.0` merge handler",
        "retires the old custody/checkpoint write paths", "`v7.0-anchor-store-delete`",
    ):
        assert fact in release, fact
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source(
        "node/upgrades.go"
    )


def test_021_repair_record_maps_finding_to_executing_source_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["737"]
    finding = record["finding"]["021"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.171.kvx"))
    original = audit["finding"]["021"]
    assert original["file"] == finding["file"] == "README.md"
    assert original["class"] == finding["class"] == "doc-drift"
    assert original["severity"] == "medium"
    assert finding["audit"] == "22.30.171"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_021_repair_record_maps_finding_to_executing_source_proofs"
    }
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_737.py"
    )
    assert task["build_cmd"] == "python3 --version"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "README.md" in task["wiring_request"]
    assert "22.31.470" in task["wiring_request"]
    assert "domain 17" in task["wiring_request"]
    assert task["missing_facts"] == []
