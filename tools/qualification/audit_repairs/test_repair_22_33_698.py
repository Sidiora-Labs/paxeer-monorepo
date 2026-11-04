"""Offline source proofs for audit 22.30.168/readme-retired-runtime.

README.md is owned by 22.31.470. These checks consume its production text;
they do not start a daemon, execute transactions or contact public endpoints.
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
RECORD = Path(__file__).with_name("repair_22_33_698.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep shell comments inside fences from becoming Markdown headings."""
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


def test_readme_retired_runtime_network():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    assert "subgraph Node[paxd]" in overview
    assert "RPC --> Order --> Host --> Native" in overview
    assert "Validators -->|Commit| State" in overview


def test_readme_retired_runtime_public_rpc():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert (
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`"
    ) in quickstart
    hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = [
        urlsplit(url)
        for url in re.findall(r"https?://[^\s`<>)]+", README)
        if "mainnet-beta.paxeer.network" in url
    ]
    assert {url.hostname for url in urls} == {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    for url in urls:
        assert url.hostname in hosts
        assert url.scheme == "https" and url.path in ("", "/")
        assert not url.query and not url.fragment and not url.username
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
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
    assert "gateway contract does not establish method availability at public node URLs" in quickstart
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart


def test_readme_retired_runtime_transaction_receipts():
    flow = section("Submit a transaction and follow its committed receipt")
    ordered = (
        "Sign a secp256k1 EIP-1559 transaction for chain ID `125`",
        "Broadcast the signed bytes with `eth_sendRawTransaction`",
        "durable WAL acknowledgement",
        "Query `pax_sequencerReceipt` with the transaction hash",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block",
        "`MsgSequenceBatch` covers that sequence",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    )
    positions = [flow.index(fact) for fact in ordered]
    assert positions == sorted(positions)
    assert "Receipt order is proposal order" in flow
    send = source("rpc/send.go")
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    consumer = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in consumer
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', consumer) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in consumer
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in consumer
    proof = section("Receipt and proof verification")
    assert "authenticate the block header and CometBFT commit" in proof
    assert "Neither a transaction hash nor a signed execution receipt alone establishes block commitment" in proof


def test_readme_retired_runtime_build_commands():
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    commands = [
        shlex.split(line, comments=True)
        for block in blocks for line in block.splitlines() if line.strip()
    ]
    for command in (
        ["make", "paxd-build"],
        ["./build/paxd", "version"],
        ["./build/paxd", "start", "--home", "$PAXD_HOME"],
    ):
        assert command in commands
    assert "build/liblayerx.a" in build
    assert "programs/target/release/liblayerx_programs_sandbox.a" in build
    assert "operator-provided genesis and consensus chain identifier" in build
    assert "running network entry point is `paxd`" in build
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release


def test_readme_retired_runtime_historical_retirement():
    native = section("EVM and Native Precompiles")
    paragraphs = [p for p in native.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(paragraphs) == 1
    retired = paragraphs[0]
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "`layerxAnchor` (`0x1014`)", "retired refusal surfaces",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "rejects writes after the merge", "historical views retire at `v7.0-anchor-store-delete`",
        "Historical anchor reads before store deletion are archive-only",
    ):
        assert fact in retired, fact
    current = README.replace(retired, "")
    assert not re.search(r"layerx(?:verify|custody|anchor)", current, re.I)
    for stale in (
        r"\blayerxd\b", r"two execution domains", r"kernel (?:still )?runs separately",
        r"make\s+checkpoint", r"/rpc(?:/|\b)",
    ):
        assert not re.search(stale, README, re.I), stale
    assert "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired" in native
    assert "archived formats add no settlement step to current block finality" in native
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_repair_record_maps_finding_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["readme-retired-runtime"]
    assert finding["audit"] == "22.30.168"
    assert finding["file"] == "README.md"
    assert finding["owner_domain"] == 16
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {"test_repair_record_maps_finding_and_evidence"}
    repair = record["repair"]
    assert repair["task"] == "22.33.698"
    assert repair["readme_owner_task"] == "22.31.470"
    assert repair["wiring_requests"] == []
    assert repair["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_698.py"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    assert "not executed" in repair["review_only"]
    assert "No baseline failures are claimed" in repair["baseline"]
