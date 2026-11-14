"""Offline source proof for audit 22.30.63 finding 016 and README owner 22.31.470.

These assertions qualify documentation, not native execution or deployments.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
NETWORK_PATH = ROOT / "docs/readme/NETWORK.md"
RECORD = ROOT / "spec/layerx-merge/programme/records/tools/qualification/audit_repairs/repair_22_33_1274.kvx"


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Select one README section without treating fenced comments as headings."""
    readme = NETWORK_PATH.read_text(encoding="utf-8")
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


def test_finding_016_integrated_paxd_network_and_block_finality():
    boundary = (ROOT / "docs/wiki/PaxeerBoundary.md").read_text(encoding="utf-8")
    assert (
        "There is no separate LayerX interface, router, or sequencer service to deploy"
    ) in boundary
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "The LayerX kernel runs inside every `paxd` node.",
        "chain ID **125** (`0x7d`)",
        "It executes each transaction when it arrives, assigns the next position "
        "in one global sequence, returns a signed execution receipt and builds "
        "each block.",
        "Validators check that block and commit it.",
        "A receipt is final when the validators commit its block.",
        "A receipt is final when the validators commit its block.",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    host = source("modules/lxphost/host.go")
    assert 'import "C"' in host
    assert "-llayerx -llayerx_programs_sandbox" in host
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_finding_016_architecture_and_receipt_diagrams():
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", NETWORK_PATH.read_text(encoding="utf-8"), re.S)
    assert len(diagrams) == 2
    topology, sequence = diagrams
    assert "subgraph Network[Paxeer X / EVM chain 125]" in topology
    node = re.search(r"subgraph Node\[paxd\]\n(.*?)\n\s*end", topology, re.S)
    assert node is not None
    assert "Embedded C17 kernel and Rust programs host" in node[1]
    assert "RPC --> Order --> Host --> Native" in node[1]
    assert "Order -->|Proposed block in execution order| Validators[Validators check and commit]" in topology
    assert "Validators -->|Commit| State[Committed block and chain state]" in topology
    assert "State -->|Transaction receipts, logs and proofs| Client" in topology
    ordered = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    )
    positions = [sequence.index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_finding_016_chain_125_root_rpc_hosts_and_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Set `PAXEER_RPC_URL` to a node root URL",
        "Send HTTPS JSON-RPC requests to the root path without a route suffix",
        "Confirm the chain ID before signing or broadcasting",
        "compare `eth_blockNumber`",
        "gateway contract does not establish method availability at public node URLs",
    ):
        assert fact in quickstart, fact
    links = dict(re.findall(r"\[([^]]+)\]\(([^)]+)\)", quickstart))
    reference_path = NETWORK_PATH.parent / links["public RPC reference"]
    assert reference_path.resolve() == (
        ROOT / "docs/site/paxeer-docs/src/app/public-rpc/page.tsx"
    ).resolve()
    reference = reference_path.read_text(encoding="utf-8")
    assert "HTTP JSON-RPC requests use the endpoint root" in reference
    assert "125 / <code>0x7d</code>" in reference
    public_rpc = (ROOT / "docs/readme/PUBLIC-RPC.md").read_text(encoding="utf-8")
    for fact in (
        "Paxeer X Network serves JSON-RPC at the root path of its public node names",
        "Use `api1` through `api14.mainnet-beta.paxeer.network` or",
        "`api-mainnet-beta.paxeer.network`, all at the root path",
        "`node.hyperpaxeer.com` is only the hpx library mirror, never a JSON-RPC endpoint",
    ):
        assert fact in public_rpc, fact
    hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = [
        urlsplit(url)
        for url in re.findall(r"https?://[^\s`<>)]+", public_rpc)
    ]
    assert {url.hostname for url in urls} == hosts
    for url in urls:
        assert url.scheme == "https" and url.hostname in hosts
        assert url.path in ("", "/")
        assert not (url.query or url.fragment or url.username or url.password)
        assert url.port is None
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert argv[argv.index("-sS") + 1] == "$PAXEER_RPC_URL"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_finding_016_submission_transport_and_committed_receipt_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`",
        "`eth_sendRawTransaction` to a public node's root JSON-RPC URL",
        "result is a transaction hash",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block whose `MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
        "No separate service endpoint is required",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    for fact in (
        "func (s *SendAPI) SendRawTransaction(",
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ):
        assert fact in send, fact
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    coverage = "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))"
    committed = "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted"
    assert receipt.index(coverage) < receipt.index(committed)
    assert "if err != nil || !committed {\n\t\treturn result, err" in receipt
    assert "block, err := a.tmClient.Block(ctx, &height)" in receipt


def test_finding_016_retired_custody_and_checkpoint_interfaces_are_historical():
    native = section("EVM and Native Precompiles")
    paragraphs = [p for p in native.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(paragraphs) == 1
    historical = paragraphs[0]
    for fact in (
        "`layerxVerify` (`0x1012`)",
        "`layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
    ):
        assert fact in historical, fact
    current = NETWORK_PATH.read_text(encoding="utf-8").replace(historical, "")
    assert not re.search(r"layerx(?:Verify|Custody|Anchor)", current, re.I)
    assert not re.search(r"\blayerxd\b", NETWORK_PATH.read_text(encoding="utf-8"), re.I)
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    assert "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired" in native
    assert "archived formats add no settlement step to current block finality" in native
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    custody = source("precompiles/layerxcustody/layerxcustody.go")
    assert "if p.keeper.Retired(ctx) {\n\t\treturn nil, errors.New(RetiredRevert)" in custody
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "if !isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "if pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source(
        "node/upgrades.go"
    )


def test_finding_016_build_and_start_use_the_integrated_node():
    build = section("Build")
    assert "make paxd-build" in build
    assert "build/liblayerx.a" in build
    assert "programs/target/release/liblayerx_programs_sandbox.a" in build
    assert "operator-provisioned, initialized node home" in build
    assert "running network entry point is `paxd`" in build
    commands = [
        shlex.split(line, comments=True)
        for block in re.findall(r"```sh\n(.*?)\n```", build, re.S)
        for line in block.splitlines()
        if line.strip()
    ]
    assert ["make", "paxd-build"] in commands
    assert ["./build/paxd", "version"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands
    assert "\npaxd-build: paxd-native-libs\n" in source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in source("Makefile")
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")


def test_repair_record_maps_finding_016_to_declared_proofs_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["016"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.63.kvx"))
    original = audit["finding"]["016"]
    assert finding["audit"] == "22.30.63"
    assert finding["file"] == original["file"] == "README.md"
    assert finding["class"] == original["class"] == "doc-drift"
    assert original["severity"] == "medium"
    assert finding["expected"] == original["expected"]
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_finding_016_")
    }
    assert len(declared) == 6
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["1274"]
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_readme_integrated_node_operations.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["wiring_requests"] == []
    wiring = record["wiring"]["readme"]
    assert wiring["path"] == "README.md"
    assert wiring["owner_task"] == "22.31.470" and wiring["owner_domain"] == 17
    assert wiring["missing_facts"] == []
    assert wiring["status"] == "satisfied_by_prerequisite"
