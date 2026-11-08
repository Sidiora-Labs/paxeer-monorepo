"""Offline source proof for audit 22.30.58/readme-retired-topology."""

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


def test_readme_retired_topology_merged_network_and_finality():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "runs inside `paxd` through [`modules/lxphost`](modules/lxphost/)",
        "node links `build/liblayerx.a`",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    topology = section("Integrated network topology")
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State[Committed block and chain state]",
    ):
        assert fact in topology, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    host = source("modules/lxphost/host.go")
    assert 'import "C"' in host
    assert "-llayerx -llayerx_programs_sandbox" in host
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_readme_retired_topology_root_rpc_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert (
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`"
    ) in quickstart
    public_urls = [
        urlsplit(url)
        for url in re.findall(r"https?://[^\s`<>)]+", README)
        if urlsplit(url).hostname
        and urlsplit(url).hostname.endswith("mainnet-beta.paxeer.network")
    ]
    assert public_urls
    hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    for url in public_urls:
        assert url.scheme == "https" and url.hostname in hosts
        assert url.path in ("", "/") and not url.query and not url.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert "https://api1.mainnet-beta.paxeer.network" in argv
    assert '# Expected result: "0x7d"' in blocks[0]
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_retired_topology_transaction_to_committed_receipt():
    flow = section("Submit a transaction and follow its committed receipt")
    steps = re.findall(r"^\d+\. (.*)$", flow, re.M)
    assert len(steps) == 4
    for step, facts in zip(steps, (
        ("chain ID `125`", "`eth_getTransactionCount`", "Sign a secp256k1 EIP-1559"),
        ("`eth_sendRawTransaction`", "transaction hash", "durable WAL acknowledgement"),
        ("`pax_sequencerReceipt`", "`submitted`", "`receipted`", "`committed`", "`MsgSequenceBatch`"),
        ("authenticate the block commitment", "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded"),
    ), strict=True):
        for fact in facts:
            assert fact in step, fact
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    ordered = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
    )
    positions = [sequence[0].index(fact) for fact in ordered]
    assert positions == sorted(positions)
    send = source("rpc/send.go")
    for fact in (
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
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert receipt.index(
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))"
    ) < receipt.index("result.Rung, result.BlockNumber, result.BlockHash = RungCommitted")


def test_readme_retired_topology_executors_are_refusal_surfaces():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "custody deposit and credit-attestation funding flow is retired" in README
    assert "External checkpoint settlement is retired" in README
    assert not re.search(r"\blayerxd\b", README, re.I)
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_readme_retired_topology_linked_node_build_and_commands():
    build = section("Build")
    commands = [
        shlex.split(line, comments=True)
        for block in re.findall(r"```sh\n(.*?)\n```", build, re.S)
        for line in block.splitlines() if line.strip()
    ]
    for command in (
        ["make", "paxd-build"],
        ["./build/paxd", "version"],
        ["./build/paxd", "start", "--home", "$PAXD_HOME"],
        ["make", "build"],
    ):
        assert command in commands, command
    for fact in (
        "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    assert "[commands below](#build)" in README
    makefile = source("Makefile")
    dependencies = dict(re.findall(r"^([\w-]+):([^\n]*)$", makefile, re.M))
    assert dependencies["paxd-build"].split() == ["paxd-native-libs"]
    assert dependencies["paxd-native-libs"].split() == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
    chain = source("chain.mk")
    assert re.search(r"^build: paxd-native-libs\n\tgo build .* -o ./build/paxd ./daemon/paxd$", chain, re.M)
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    for target in re.findall(r"\]\(([^)]+)\)", README):
        if not target.startswith(("https:", "http:", "#")):
            assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_readme_retired_topology_record_maps_finding_to_executable_proofs():
    record = tomllib.loads(source("tools/qualification/audit_repairs/repair_22_33_1224.kvx"))
    assert record["task"]["id"] == "22.33.1224"
    finding = record["finding"]["readme-retired-topology"]
    assert finding["audit"] == "22.30.58" and finding["file"] == "README.md"
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_readme_retired_topology_record_maps_finding_to_executable_proofs"
    }
    wiring = record["wiring"]["readme"]
    assert wiring["path"] == "README.md" and wiring["owner_task"] == "22.31.470"
    assert wiring["owner_domain"] == 17 and wiring["missing_facts"] == []
    assert record["qualification"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1224.py"
    )
