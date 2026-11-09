"""Offline source proof for audit 22.30.136/readme-separate-daemon.

README ownership is 22.31.470. These checks qualify its documented contracts,
not native execution, restart recovery or public endpoint availability.
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


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Read headings only outside Markdown code fences."""
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


def make_rule(path, target):
    """Read a declared target's prerequisites and tab-indented recipe."""
    matches = re.findall(
        r"^" + re.escape(target) + r":([^\n]*)\n((?:\t[^\n]*\n)*)",
        source(path),
        re.M,
    )
    assert len(matches) == 1, (path, target)
    dependencies, recipe = matches[0]
    return dependencies.split(), recipe


def test_readme_separate_daemon_merged_topology():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    topology = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(topology) == 1
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in topology[0], fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    host = source("modules/lxphost/host.go")
    assert 'import "C"' in host
    assert "-llayerx -llayerx_programs_sandbox" in host
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_readme_separate_daemon_root_rpc():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "root path of these hosts, without a route suffix",
        "`https://api1.mainnet-beta.paxeer.network` through",
        "`https://api14.mainnet-beta.paxeer.network`",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    urls = [
        urlsplit(url) for url in re.findall(r"https?://[^\s`<>)]+", README)
        if "mainnet-beta.paxeer.network" in url
    ]
    assert urls
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    for url in urls:
        assert url.scheme == "https" and url.hostname in allowed_hosts
        assert url.path in ("", "/") and not url.query and not url.fragment
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
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_separate_daemon_transaction_and_commit():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "commitment does not mean execution succeeded", "`eth_getTransactionReceipt`",
        "`eth_call` at an explicit committed height", "No separate service endpoint is required",
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
    assert "if err == nil && ack.Status != AckOK" in release
    receipt = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    calls = [
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))",
        "if err != nil || !committed",
        "a.tmClient.Block(ctx, &height)",
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted",
    ]
    positions = [receipt.index(call) for call in calls]
    assert positions == sorted(positions)
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(diagrams) == 1
    events = [
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash", "Committed status, logs and native state",
    ]
    positions = [diagrams[0].index(event) for event in events]
    assert positions == sorted(positions)


def test_readme_separate_daemon_paxd_build():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    commands = [
        shlex.split(line, comments=True)
        for block in re.findall(r"```sh\n(.*?)\n```", build, re.S)
        for line in block.splitlines() if line.strip()
    ]
    assert ["make", "paxd-build"] in commands
    assert ["./build/paxd", "version"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands
    dependencies, recipe = make_rule("Makefile", "paxd-build")
    assert dependencies == ["paxd-native-libs"]
    assert "tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    dependencies, _ = make_rule("Makefile", "paxd-native-libs")
    assert dependencies == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
    dependencies, recipe = make_rule("chain.mk", "build")
    assert dependencies == ["paxd-native-libs"]
    assert ["go", "build", "$(BUILD_FLAGS)", "-o", "./build/paxd", "./daemon/paxd"] == shlex.split(recipe)


def test_readme_separate_daemon_retired_history():
    native = section("EVM and Native Precompiles")
    paragraphs = [p for p in native.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(paragraphs) == 1
    historical = paragraphs[0]
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "archive-only", "`v7.0-anchor-store-delete`",
    ):
        assert fact in historical, fact
    current = README.replace(historical, "")
    assert not re.search(r"\blayerx(?:Verify|Custody|Anchor)\b", current, re.I)
    assert not re.search(r"\blayerxd\b", README, re.I)
    assert "External checkpoint settlement is retired" in README
    assert "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired" in native
    assert "archived formats add no settlement step to current block finality" in native
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_repair_record_maps_the_finding_and_proof_limits():
    record = tomllib.loads(Path(__file__).with_name("repair_22_33_317.kvx").read_text(encoding="utf-8"))
    assert record["task"]["id"] == "22.33.317"
    assert record["task"]["audit"] == "22.30.136"
    assert record["task"]["criterion"] == "22998.1"
    assert record["task"]["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_317.py"
    assert record["task"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same revision" in record["task"]["verified_revision"]
    assert "No baseline tests ran" in record["task"]["baseline"]
    assert "is executed" in record["task"]["review_only"]
    assert record["task"]["missing_facts"] == []
    assert set(record["finding"]) == {"readme-separate-daemon"}
    finding = record["finding"]["readme-separate-daemon"]
    assert finding["file"] == "README.md" and finding["owner_task"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_readme_")
    }
    assert len(declared) == 5 and set(finding["tests"]) == declared
    request = record["wiring"]["root_readme"]["request"]
    assert "README.md" in request and "domain 17 / task 22.31.470" in request
