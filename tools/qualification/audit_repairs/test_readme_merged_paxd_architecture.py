"""Offline source proof for audit 22.30.163/051 and decision 2525.

Read the maintained README and its production connections without executing
the node, build commands, cryptography or network requests.
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
README = NETWORK_PATH.read_text(encoding="utf-8")
RECORD = ROOT / "spec/layerx-merge/programme/records/tools/qualification/audit_repairs/repair_22_33_629.kvx"


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


def require_facts(text, facts):
    for fact in facts:
        assert fact in text, f"Network reference: {fact!r}"


def test_051_merged_paxd_architecture_and_current_status():
    boundary = (ROOT / "docs/wiki/PaxeerBoundary.md").read_text(encoding="utf-8")
    assert (
        "There is no separate LayerX interface, router, or sequencer service to deploy"
    ) in boundary
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
        "External checkpoint settlement is retired",
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ))
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    require_facts(section("Releases and Network Upgrades"), (
        "executes kernel activity inside `paxd`",
        "commits it with the BFT chain state",
        "retires the old custody/checkpoint write paths",
    ))
    assert not re.search(r"\blayerxd\b|merge (?:is|remains) (?:a |only a )?draft", README, re.I)


def test_051_chain_125_and_root_jsonrpc_hosts():
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


def test_051_transaction_receipt_producer_transport_and_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    require_facts(flow, (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet", "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "authenticate the block commitment", "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ))
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    release = source("node/sequencer/accepter.go")
    acknowledgement = release.index("a.rep.Release(context.Background(), &wal.Accept{")
    refusal = release.index("if err == nil && ack.Status != AckOK", acknowledgement)
    halt = release.index('fmt.Errorf("%w: %w", ErrReplicationHalted, err)', refusal)
    publication = release.index("a.src.Record(common.Hash(acc.ActivityID), send)")
    assert acknowledgement < refusal < halt < publication
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed",
    ]
    verified = receipt.index("if err := result.Verify(keys); err != nil")
    coverage = receipt.index(
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))"
    )
    uncommitted = receipt.index("if err != nil || !committed", coverage)
    block = receipt.index("a.tmClient.Block(ctx, &height)", uncommitted)
    committed = receipt.index("result.Rung, result.BlockNumber, result.BlockHash = RungCommitted")
    assert verified < coverage < uncommitted < block < committed
    require_facts(section("Receipt and proof verification"), (
        "ProtocolReceipt v2", "pinned for the receipt height",
        "`committed` rung requires coverage", "`MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ))
    assert "ed25519.Sign(k.private, digest[:])" in source("node/sequencer/receiptkey.go")
    assert "codec.DecodeReceipt(receiptBytes)" in source("layerxproof/verify/receipt.go")
    diagram = re.search(r"```mermaid\nsequenceDiagram\n(.*?)\n```", flow, re.S)
    assert diagram is not None
    positions = [diagram[1].index(event) for event in (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL", "Release signed execution receipt",
        "receipted (execution evidence)", "Propose MsgSequenceBatch in receipt order",
        "Commit block", "committed, blockNumber, blockHash",
    )]
    assert positions == sorted(positions)


def test_051_build_and_start_use_the_merged_node():
    build = section("Build")
    require_facts(build, (
        "Build the merged node from the repository root",
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ))
    commands = [
        shlex.split(line, comments=True)
        for block in re.findall(r"```sh\n(.*?)\n```", build, re.S)
        for line in block.splitlines() if line.strip()
    ]
    assert ["make", "paxd-build"] in commands
    assert ["./build/paxd", "version"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands
    makefile = source("Makefile")
    assert re.search(r"^paxd-build: paxd-native-libs$", makefile, re.M)
    assert re.search(r"^paxd-native-libs: \$\(LIBRARY\) programs-release-build evmone-lib$", makefile, re.M)
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")


def test_051_retired_executors_and_historical_checkpoint_material():
    native = section("EVM and Native Precompiles")
    require_facts(native, (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "historical views retire at `v7.0-anchor-store-delete`",
        "Historical anchor reads before store deletion are archive-only",
        "archived settlement witness formats", "support historical interpretation",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ))
    paragraphs = [p for p in README.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(paragraphs) == 1
    assert "retired refusal surfaces" in paragraphs[0] and "archive-only" in paragraphs[0]
    current = README.replace(paragraphs[0], "")
    assert not re.search(r"layerx(?:Anchor|Custody|Verify)", current, re.I)
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")


def test_051_record_maps_the_finding_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["051"]
    audit = source("spec/layerx-merge/programme/audit/records/22.30.163.kvx")
    block = re.search(r"^\[finding\.051\]\n(.*?)(?=^\[|\Z)", audit, re.M | re.S)
    assert block is not None
    original = tomllib.loads(block[1])
    assert finding["audit"] == "22.30.163"
    for key in ("file", "line", "class", "severity", "owner_domain", "expected"):
        assert finding[key] == original[key]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["629"]
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_readme_merged_paxd_architecture.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "final hf-done JSON" in task["verified_revision"]
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert task["outstanding_wiring_requests"] == []
    request = record["wiring"]["root_readme"]
    assert all(value in request["request"] for value in ("README.md", "domain 15", "22.31.470"))
    assert "no required fact is missing" in request["status"]
