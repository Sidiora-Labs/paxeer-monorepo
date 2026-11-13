"""Finding 042: qualify README contracts against committed production sources.

These in-process source checks do not certify native execution or deployment.
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
RECORD = Path(__file__).with_name("repair_22_33_565.kvx")


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


def test_f042_merged_network_embeds_kernel_and_block_proposal():
    overview = section("What is Paxeer X Network?", level=2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "runs inside `paxd` through", "modules/lxphost/",
        "There is no separate kernel daemon, sequencer service or checkpoint deployment",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    host = source("modules/lxphost/host.go")
    assert 'import "C"' in host
    assert "-llayerx -llayerx_programs_sandbox" in host
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert len(diagrams) == 2
    topology = diagrams[0]
    assert "subgraph Network[One Paxeer BFT network / EVM chain 125]" in topology
    assert "subgraph Node[paxd]" in topology
    assert "RPC --> Order --> Host --> Native" in topology
    assert "Validators -->|Commit| State" in topology
    for stale in (
        r"\blayerxd\b", r"\bmake\s+layerxd\b",
        r"two (?:execution )?domains", r"kernel still runs separately",
    ):
        assert not re.search(stale, README, re.I), stale


def test_f042_chain_probe_and_numbered_hosts_use_root_json_rpc():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "`https://api1.mainnet-beta.paxeer.network` through" in quickstart
    assert "`https://api14.mainnet-beta.paxeer.network`" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert "there is no router and no separate LayerX interface" in README
    assert "`node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    allowed = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = re.findall(r"https://api[^\s`\")]+", README)
    assert urls
    for url in urls:
        parsed = urlsplit(url)
        assert parsed.hostname in allowed
        assert parsed.path in ("", "/")
        assert not parsed.query and not parsed.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:", 1)[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_f042_transaction_receipt_producer_transport_and_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`eth_getTransactionCount`", "EIP-1559 transaction for chain ID `125`",
        "`eth_sendRawTransaction`", "transaction hash", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`pax_sequencerReceipt`",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height", "No separate service endpoint is required",
    ):
        assert fact in flow, fact
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
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in rpc
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in rpc


def test_f042_finality_requires_authenticated_block_commit():
    assert "A receipt is final once its block is committed" in README
    proof = section("Receipt and proof verification")
    for fact in (
        "signed ProtocolReceipt v2", "separate from the validator consensus key",
        "pinned for the receipt height", "`committed` rung requires coverage",
        "committed block's `MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit against trusted validator information",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ):
        assert fact in proof, fact
    assert "ed25519.Sign(k.private, digest[:])" in source("node/sequencer/receiptkey.go")
    assert "codec.DecodeReceipt(receiptBytes)" in source("layerxproof/verify/receipt.go")
    assert "self.verify_header_commit(header, commit)?" in source(
        "programs/crates/layerx-programs-registry/src/chain_proof/head_verify.rs"
    )
    sequence = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)[1]
    ordered = [
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    ]
    positions = [sequence.index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_f042_native_inventory_and_retired_executor_refusals():
    native = section("EVM and Native Precompiles")
    rows = re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native)
    assert len(rows) == 13
    documented = {int(address, 16): name for address, name in rows}
    registry = source("precompiles/setup.go").split(
        "var MergeAddressTable = []MergeAddress{", 1
    )[1].split("\n}", 1)[0]
    declared = {
        int(address, 16): name
        for address, name in re.findall(r'HexToAddress\("(0x[0-9A-Fa-f]+)"\), "([^"]+)"', registry)
    }
    assert len(declared) == 13
    assert documented == declared
    assert {0x101A, 0x101B, 0x101C, 0x101D} <= documented.keys()
    assert not {0x1012, 0x1013, 0x1014} & documented.keys()
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
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


def test_f042_build_examples_produce_and_start_paxd():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    recipe = makefile.split("\npaxd-build: paxd-native-libs\n", 1)[1].split("\n\n", 1)[0]
    assert "tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release


def test_f042_explorer_and_optional_hosted_gateway_entry_points():
    build = section("Build")
    assert "Explorer operators use [`services/explorer`](services/explorer/)" in build
    docs = section("Documentation", level=2)
    assert "[hosted quickstart](docs/wiki/Quickstart.md)" in docs
    assert "[hosted gateway methods](docs/site/paxeer-docs/src/app/platform-api/page.tsx)" in docs
    quickstart = section("Quick start: connect to chain 125")
    assert "services/platform/hosted/gateway/openrpc.json" in quickstart
    assert "Obtain its deployment URL and credentials from the operator" in quickstart
    assert "gateway contract does not establish method availability at public node URLs" in quickstart
    assert "gateway is optional developer tooling" in README
    assert "does not provide another execution or finality domain" in README
    methods = {
        method["name"]
        for method in json.loads(source("services/platform/hosted/gateway/openrpc.json"))["methods"]
    }
    assert {"px_getBalances", "px_getReceiptKeys"} <= methods
    for target in re.findall(r"\]\(([^)]+)\)", docs + build):
        if not target.startswith(("https:", "http:", "#")):
            assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_f042_record_maps_finding_to_proofs_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["565"]
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["input_revision"] == "404ef97e86593776f2f8e3e87ad67afe5ea8a3bb"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_565.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["wiring_requests"] == []
    assert "22.31.470" in task["repair"]
    finding = record["finding"]["042"]
    assert finding["audit"] == "22.30.158"
    assert finding["file"] == "README.md"
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert len(finding["tests"]) == len(declared)
    assert set(finding["tests"]) == declared
