"""Source proofs for readme-two-domains; no native or live-network qualification."""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
NETWORK = (ROOT / "docs/readme/NETWORK.md").read_text(encoding="utf-8")
PUBLIC_RPC = (ROOT / "docs/readme/PUBLIC-RPC.md").read_text(encoding="utf-8")
RECORD = (ROOT / "spec/layerx-merge/programme/records/tools/qualification/audit_repairs/repair_22_33_959.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(title, level=3):
    """Keep comments inside fenced examples from becoming section boundaries."""
    marker = "#" * level + " " + title + "\n"
    assert NETWORK.count(marker) == 1, title
    lines = []
    fence = None
    for line in NETWORK.split(marker, 1)[1].splitlines(keepends=True):
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


def test_readme_two_domains_integrated_paxd_and_bft_finality():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "The LayerX kernel runs inside every `paxd` node.", "chain ID **125** (`0x7d`)",
        "It executes each transaction when it arrives, assigns the next position "
        "in one global sequence, returns a signed execution receipt and builds "
        "each block.",
        "Validators check that block and commit it.", "A receipt is final when the validators commit its block.",
        "A receipt is final when the validators commit its block.",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(diagrams) == 1
    for fact in (
        "subgraph Network[Paxeer X / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in diagrams[0], fact


def test_readme_two_domains_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Send HTTPS JSON-RPC requests to the root path without a route suffix." in quickstart
    assert "The former unified router and its prefixed RPC routes are retired." in PUBLIC_RPC
    urls = re.findall(r"https?://[^\s`<>)]+", PUBLIC_RPC)
    public = [urlsplit(url) for url in urls if "mainnet-beta.paxeer.network" in url]
    assert public
    hosts = {f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)}
    hosts.add("api-mainnet-beta.paxeer.network")
    for url in public:
        assert url.scheme == "https" and url.hostname in hosts
        assert url.path in ("", "/") and not url.query and not url.fragment
    examples = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(examples) == 1
    argv = shlex.split(examples[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert argv[argv.index("-H") - 1] == "$PAXEER_RPC_URL"
    assert '# Expected result: "0x7d"' in examples[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_two_domains_transaction_to_committed_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "EIP-1559 transaction for chain ID `125`", "`eth_getTransactionCount`",
        "`eth_sendRawTransaction`", "transaction hash", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`pax_sequencerReceipt`",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "No separate service endpoint is required",
        "authenticate the block commitment", "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
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
    accepter = source("node/sequencer/accepter.go")
    assert accepter.index("a.rep.Release(context.Background(), &wal.Accept{") < accepter.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
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


def test_readme_two_domains_retired_executors_and_historical_settlement():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads", "archive-only",
        "archived settlement witness formats", "support historical interpretation",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_readme_two_domains_build_and_run_guidance_has_no_daemon():
    build = section("Build")
    for fact in (
        "Build the merged node from the repository root", "make paxd-build",
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "operator-provisioned, initialized node home", "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "For kernel library and development tools without rebuilding the node",
        "make build", "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    # The owning rewrite removed historical daemon commands entirely; the current
    # kernel-library target is explicitly distinguished from the network runtime.
    for stale in (
        r"\blayerxd\b", r"two (?:execution )?domains", r"kernel still runs separately",
        r"merge is (?:only )?a draft", r"make\s+(?:layerxd|daemon)-build",
    ):
        assert not re.search(stale, NETWORK, re.I), stale


def test_readme_two_domains_record_maps_finding_and_proof_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["readme-two-domains"]
    assert finding["audit"] == "22.30.28"
    assert finding["file"] == "README.md" and finding["owner_domain"] == 1
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["959"]
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_readme_single_execution_domain.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["source_proof"] and task["review_only"] and task["baseline"]
    wiring = record["wiring"]["readme"]
    assert wiring["path"] == "README.md" and wiring["owner_task"] == "22.31.470"
    assert wiring["owner_domain"] == 1 and wiring["status"] == "satisfied"
