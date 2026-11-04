"""Offline source proof for audit 22.30.8/readme-separate-kernel-network.

Qualify the README inherited from 22.31.470 against the production node paths;
native execution, live endpoints and restart recovery are outside this gate.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
RECORD_PATH = Path(__file__).with_name("repair_22_33_1427.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Extract a Markdown section, ignoring headings inside fenced examples."""
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


def make_rule(path, target):
    """Parse the declared prerequisites and tab-indented recipe of a rule."""
    match = re.search(
        r"^" + re.escape(target) + r":([^\n]*)\n((?:\t[^\n]*\n)*)",
        source(path), re.M,
    )
    assert match is not None, (path, target)
    return match[1].split(), match[2]


def test_readme_describes_one_paxd_network_with_embedded_kernel():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    topology = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(topology) == 1
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native", "Validators -->|Commit| State",
    ):
        assert fact in topology[0], fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_public_rpc_hosts_and_chain_probe_use_the_root_endpoint():
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
    node_urls = []
    for raw_url in re.findall(r"https?://[^\s`<>)]+", source("README.md")):
        url = urlsplit(raw_url)
        if url.hostname and url.hostname.endswith("mainnet-beta.paxeer.network"):
            node_urls.append(url)
            assert url.hostname in allowed_hosts
            assert url.scheme == "https" and url.path in ("", "/")
            assert url.port is None and not url.query and not url.fragment
    assert {url.hostname for url in node_urls} == {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network", "api14.mainnet-beta.paxeer.network",
    }
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
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
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_transaction_and_committed_receipt_flow_matches_production_sources():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "root JSON-RPC URL", "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "No separate service endpoint is required",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(diagrams) == 1
    ordered = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    )
    positions = [diagrams[0].index(fact) for fact in ordered]
    assert positions == sorted(positions)
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        'err = fmt.Errorf("%w: status %d", ErrStandbyRefused, ack.Status)'
    ) < release.index(
        'return evmrpc.AcceptedSend{}, fmt.Errorf("%w: %w", ErrReplicationHalted, err)'
    ) < release.index("a.src.Record(common.Hash(acc.ActivityID), send)")
    receipt = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed",
    ]
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert "send, err := a.option.Source.SequencerSend(txHash)" in receipt
    assert receipt.index(
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))"
    ) < receipt.index("if err != nil || !committed {") < receipt.index(
        "block, err := a.tmClient.Block(ctx, &height)"
    ) < receipt.index("result.Rung, result.BlockNumber, result.BlockHash = RungCommitted")
    proof = section("Receipt and proof verification")
    assert (
        "Neither a transaction hash nor a signed execution receipt alone "
        "establishes block commitment"
    ) in proof
    assert "authenticate the block header and CometBFT commit" in proof


def test_retired_executors_and_release_guidance_preserve_block_finality():
    readme = source("README.md")
    assert "A receipt is final once its block is committed" in readme
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "retired at the merge", "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "Historical anchor reads before store deletion are archive-only",
        "External checkpoint registration, guarantor attestations",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    paragraphs = [p for p in readme.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(paragraphs) == 1
    assert "retired refusal surfaces" in paragraphs[0]
    current = readme.replace(paragraphs[0], "")
    for name in ("layerxVerify", "layerxCustody", "layerxAnchor"):
        assert name not in current, name
    for stale in (
        r"\blayerxd\b", r"kernel still runs separately",
        r"merge is (?:only )?a draft", r"make\s+(?:layerxd|checkpoint)\b",
    ):
        assert re.search(stale, readme, re.I) is None, stale
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release
    assert "retires the old custody/checkpoint write paths" in release
    assert "`v7.0-anchor-store-delete` removes the historical anchor store" in release


def test_build_instructions_match_linked_node_prerequisites():
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    assert len(blocks) == 4
    assert shlex.split(blocks[0], comments=True) == ["make", "paxd-build"]
    assert [shlex.split(line) for line in blocks[1].splitlines()] == [
        ["./build/paxd", "version"],
        ["./build/paxd", "start", "--home", "$PAXD_HOME"],
    ]
    for fact in (
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    prerequisites, recipe = make_rule("Makefile", "paxd-native-libs")
    assert prerequisites == ["$(LIBRARY)", "programs-release-build"]
    prerequisites, recipe = make_rule("Makefile", "paxd-build")
    assert prerequisites == ["paxd-native-libs"]
    assert "jq -er .version tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    prerequisites, recipe = make_rule("chain.mk", "build")
    assert "paxd-native-libs" in prerequisites
    assert "-o ./build/paxd ./daemon/paxd" in recipe


def test_repair_record_maps_the_finding_to_declared_source_proofs():
    record = tomllib.loads(RECORD_PATH.read_text(encoding="utf-8"))
    finding = record["finding"]["readme-separate-kernel-network"]
    audit = source("spec/layerx-merge/programme/audit/records/22.30.8.kvx")
    block = re.search(
        r"^\[finding\.readme-separate-kernel-network\]\n[^\[]*", audit, re.M,
    )
    assert block is not None
    original = tomllib.loads(block[0])["finding"]["readme-separate-kernel-network"]
    for key in ("file", "class", "severity", "owner_domain"):
        assert finding[key] == original[key], key
    assert finding["audit"] == "22.30.8"
    assert finding["implementation_task"] == "22.31.470"
    tests = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == tests
    task = record["task"]["22"]["33"]["1427"]
    assert set(task["paths"]) == {
        "tools/qualification/audit_repairs/repair_22_33_1427.kvx",
        "tools/qualification/audit_repairs/test_repair_22_33_1427.py",
    }
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1427.py"
    )
    assert task["build_cmd"] == "python3 --version"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["verification_evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert task["missing_facts"] == [] and task["wiring_requests"] == []
    wiring = record["wiring"]["readme"]
    for fact in ("README.md", "owner domain 16", "22.31.470"):
        assert fact in wiring["request"], fact
    assert "Already supplied by 22.31.470" in wiring["disposition"]
