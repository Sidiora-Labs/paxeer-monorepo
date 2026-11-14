"""Offline source proofs for audit 22.30.137's README topology finding.

The production README is supplied by task 22.31.470. These checks neither
execute the native node nor contact the documented network endpoints.
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
RECORD_PATH = (
    ROOT / "spec/layerx-merge/programme/records/tools/qualification/audit_repairs"
    / "repair_22_33_335.kvx"
)


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Select a unique Markdown section without treating shell comments as headings."""
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


def test_readme_topology_integrates_the_kernel_in_one_paxd_bft_network():
    boundary = (ROOT / "docs/wiki/PaxeerBoundary.md").read_text(encoding="utf-8")
    assert (
        "There is no separate LayerX interface, router, or sequencer service to deploy"
    ) in boundary
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "The LayerX kernel runs inside every `paxd` node.",
        "EVM chain ID **125** (`0x7d`)",
        "It executes each transaction when it arrives, assigns the next position "
        "in one global sequence, returns a signed execution receipt and builds "
        "each block.",
        "Validators check that block and commit it.",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "runs inside `paxd` through [`modules/lxphost`](../../modules/lxphost/)",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "hosted gateway is optional developer tooling",
        "outside consensus",
    ):
        assert fact in overview, fact
    topology = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(topology) == 1
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native",
        "Proposed block in execution order",
        "Validators -->|Commit| State",
    ):
        assert fact in topology[0], fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_readme_topology_public_rpc_hosts_are_root_urls_for_chain_125():
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


def test_readme_topology_transaction_receipts_follow_the_real_commit_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`",
        "`eth_sendRawTransaction` to a public node's root JSON-RPC URL",
        "result is a transaction hash",
        "accepts on the node's integrated sequencing path or forwards",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block whose `MsgSequenceBatch` covers that sequence",
        "No separate service endpoint is required",
        "authenticate the block commitment before treating the result as final",
        "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(diagrams) == 1
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
    positions = [diagrams[0].index(fact) for fact in ordered]
    assert positions == sorted(positions)
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    accepter = source("node/sequencer/accepter.go")
    assert accepter.index("a.rep.Release(context.Background(), &wal.Accept{") < accepter.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt


def test_readme_topology_custody_and_checkpoint_executors_are_historical():
    native = section("EVM and Native Precompiles")
    for fact in (
        "pre-merge `layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "do not provide current funding or finality paths",
        "external native bridge at `0x1016` remains distinct from retired kernel custody",
        "archived settlement witness formats",
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
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    for stale in (
        r"\blayerxd\b",
        r"\bmerge is (?:only )?(?:a )?draft\b",
        r"\bkernel (?:still )?runs separately\b",
        r"\bseparate checkpoint settlement\b",
    ):
        assert not re.search(stale, README, re.I), stale


def test_readme_topology_build_and_launch_use_the_merged_paxd_entry_point():
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    assert shlex.split(blocks[0], comments=True) == ["make", "paxd-build"]
    assert [shlex.split(line) for line in blocks[1].splitlines()] == [
        ["./build/paxd", "version"],
        ["./build/paxd", "start", "--home", "$PAXD_HOME"],
    ]
    for fact in (
        "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "operator-provided genesis and consensus chain identifier",
        "EVM chain ID `125` is the signing domain",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    upgrades = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in upgrades
    assert "commits it with the BFT chain state" in upgrades


def test_repair_record_maps_the_audit_finding_to_declared_source_proofs():
    record = tomllib.loads(RECORD_PATH.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["335"]
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["inspected_revision"] == "2a9f3e4910ac253ea6da6046126cae29313b65d1"
    assert task["readme_owner_task"] == "22.31.470"
    assert task["readme_owner_revision"] == "38801bb427090ac96d47c86a026a63ec05e67315"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_readme_kernel_topology.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["wiring_requests"] == []
    assert set(record["finding"]) == {"readme_topology"}
    finding = record["finding"]["readme_topology"]
    assert finding["audit"] == "22.30.137"
    assert finding["file"] == "README.md"
    assert finding["owner_domain"] == 8
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    audit = source("spec/layerx-merge/programme/audit/records/22.30.137.kvx")
    block = re.search(r"^\[finding.readme_topology\]\n.*?(?=^\[|\Z)", audit, re.M | re.S)
    assert block is not None
    original = tomllib.loads(block.group())["finding"]["readme_topology"]
    for key in ("file", "class", "severity", "owner_domain", "expected", "proposal"):
        assert finding[key] == original[key], key
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert len(finding["tests"]) == len(set(finding["tests"]))
    assert set(finding["tests"]) == declared
