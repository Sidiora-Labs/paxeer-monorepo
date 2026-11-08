"""Finding 22: qualify README source contracts without native or live services.

These checks consume the README supplied by 22.31.470 and its production sources;
they do not certify execution, consensus, recovery, or endpoint availability.
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
RECORD = Path(__file__).with_name("repair_22_33_834.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep fenced shell comments from becoming Markdown section boundaries."""
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


def test_f22_one_bft_network_embeds_kernel_in_paxd():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
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
    assert not re.search(r"two (?:execution )?domains|\blayerxd\b", README, re.I)


def test_f22_chain_125_probe_uses_numbered_node_root_urls():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert "`https://api1.mainnet-beta.paxeer.network` through" in quickstart
    assert "`https://api14.mainnet-beta.paxeer.network`" in quickstart
    assert "https://api-mainnet-beta.paxeer.network" in quickstart
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    node_urls = [
        urlsplit(url) for url in re.findall(r"https?://[^\s`<>)]+", README)
        if urlsplit(url).hostname in allowed_hosts
    ]
    assert node_urls
    for url in node_urls:
        assert url.scheme == "https" and url.path in ("", "/")
        assert not url.query and not url.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    command = blocks[0].split("\n# Expected result:", 1)[0].replace("\\\n", " ")
    argv = shlex.split(command)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert "compare `eth_blockNumber`" in quickstart
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_f22_current_node_and_hosted_gateway_interfaces_are_separate():
    quickstart = section("Quick start: connect to chain 125")
    assert "gateway contract does not establish method availability at public node URLs" in quickstart
    assert "Obtain its deployment URL and credentials from the operator" in quickstart
    assert "`px_getReceiptKeys` is registered by the node" in quickstart
    assert "no router and no separate LayerX interface" in README
    assert "outside consensus" in README
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    paths = (
        "docs/site/docs/reference/public-rpc.md", "rpc/server.go",
        "docs/site/docs/platform/gateway-rpc.md",
        "services/platform/hosted/gateway/openrpc.json",
    )
    for path in paths:
        assert path in README and (ROOT / path).is_file(), path
    methods = {
        method["name"] for method in json.loads(source(paths[-1]))["methods"]
    }
    assert {"px_getBalances", "px_getReceiptKeys"} <= methods
    server = source("rpc/server.go")
    assert '{Namespace: "px", Service: pxAPI}' in server
    assert 'Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}' in server


def test_f22_submission_receipt_and_commit_follow_production_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "commitment does not mean execution succeeded",
        "`eth_getTransactionReceipt`", "`eth_call` at an explicit committed height",
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
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == ["submitted", "receipted", "committed"]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in rpc
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in rpc
    assert "return SequencerReceiptMethod + \" served by sequencer\"" in rpc
    proofs = section("Receipt and proof verification")
    assert "authenticate the block header and CometBFT commit" in proofs
    assert "Neither a transaction hash nor a signed execution receipt alone establishes block commitment" in proofs
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(diagrams) == 1
    order = [
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL", "Release signed execution receipt",
        "receipted (execution evidence)", "Propose MsgSequenceBatch in receipt order",
        "Commit block", "committed, blockNumber, blockHash",
    ]
    positions = [diagrams[0].index(fact) for fact in order]
    assert positions == sorted(positions)


def test_f22_custody_and_checkpoint_executors_are_explicitly_retired():
    historical = [p for p in README.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(historical) == 1
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "Historical anchor reads before store deletion are archive-only",
        "`v7.0-anchor-store-delete`", "`LXP_MODULE_DISABLED`",
    ):
        assert fact in historical[0], fact
    current = README.replace(historical[0], "")
    assert not re.search(r"layerxAnchor|layerxCustody|layerxVerify", current)
    assert "External checkpoint settlement is retired" in README
    assert "archived formats add no settlement step to current block finality" in README
    assert "custody deposit and credit-attestation funding flow is retired" in README
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    custody = source("precompiles/layerxcustody/layerxcustody.go")
    assert "if p.keeper.Retired(ctx)" in custody
    assert 'RetiredRevert        = "LXP_MODULE_DISABLED:' in custody
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_f22_build_examples_produce_and_start_merged_paxd():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    native = re.search(r"^paxd-native-libs: ([^\n]+)$", makefile, re.M)
    assert native and native.group(1).split() == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
    node = re.search(r"^paxd-build: ([^\n]+)\n((?:\t[^\n]*\n)+)", makefile, re.M)
    assert node and node.group(1).split() == ["paxd-native-libs"]
    assert "tools/release/version.json" in node.group(2)
    assert "$(PAXEER_MAKE) build" in node.group(2)
    chain = re.search(r"^build: ([^\n]+)\n((?:\t[^\n]*\n)+)", source("chain.mk"), re.M)
    assert chain and chain.group(1).split() == ["paxd-native-libs"]
    assert "-o ./build/paxd ./daemon/paxd" in chain.group(2)
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    assert not re.search(r"\bmake\b[^\n]*\blayerxd\b", README)


def test_f22_record_maps_finding_to_declared_checks_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["22"]
    assert finding["audit"] == "22.30.18" and finding["file"] == "README.md"
    assert finding["severity"] == "medium" and finding["class"] == "doc-drift"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert len(finding["tests"]) == len(set(finding["tests"]))
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["834"]
    assert task["acceptance"] == "22998.1"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_834.py"
    assert task["build_cmd"] == "python3 --version"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert task["missing_facts"] == []
    assert record["wiring"]["readme"]["path"] == "README.md"
    assert record["wiring"]["readme"]["owner_task"] == "22.31.470"
