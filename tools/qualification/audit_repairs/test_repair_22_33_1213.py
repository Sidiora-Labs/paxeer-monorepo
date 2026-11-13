"""Offline source-fact proof for audit 22.30.57/readme-network-model."""

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


def test_readme_describes_one_block_proposing_paxd_network():
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
    topology = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(topology) == 1
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in topology[0], fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    for stale in (
        r"\blayerxd\b",
        r"two (?:execution )?domains",
        r"kernel still runs separately",
        r"merge is (?:only )?a draft",
    ):
        assert not re.search(stale, README, re.I), stale


def test_readme_links_root_rpc_contract_and_chain_125_probe():
    quickstart = section("Quick start: connect to chain 125")
    contract_path = "docs/site/paxeer-docs/src/app/public-rpc/page.tsx"
    assert f"]({contract_path})" in README
    contract = source(contract_path)
    assert "JSON-RPC is served over HTTPS at the root path" in contract
    assert "EVM chain ID `125` (`0x7d`)" in contract
    assert "root path of these hosts, without a route suffix" in quickstart
    assert "`https://api1.mainnet-beta.paxeer.network` through" in quickstart
    assert "`https://api14.mainnet-beta.paxeer.network`" in quickstart
    endpoints = {
        f"https://api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)
    }
    contract_urls = re.findall(r"`(https://[^`]+)`", contract)
    assert endpoints <= set(contract_urls)
    for url in re.findall(r"https://[^\s`<>)]+", README) + contract_urls:
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.path in ("", "/"), url
            assert not parsed.query and not parsed.fragment, url
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
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_transaction_receipt_flow_requires_block_commit():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`",
        "`eth_sendRawTransaction`",
        "transaction hash",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block",
        "`MsgSequenceBatch`",
        "authenticate the block commitment before treating the result as final",
        "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
        "No separate service endpoint is required",
    ):
        assert fact in flow, fact
    events = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
    )
    diagram = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(diagram) == 1
    positions = [diagram[0].index(event) for event in events]
    assert positions == sorted(positions)
    send = source("rpc/send.go")
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt


def test_readme_precompile_availability_separates_retired_executors():
    native = section("EVM and Native Precompiles")
    rows = dict(re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native))
    expected = {
        "0x101A": "layerxledger", "0x101B": "layerxpay", "0x101C": "layerxauth",
        "0x101D": "layerxprograms", "0x101E": "layerxchains", "0x0901": "orob",
        "0x0902": "clearing", "0x0903": "oracle aggregator", "0x0904": "pofq",
        "0x0905": "scheduler", "0x0906": "streams", "0x0907": "tee attestor",
        "0x0908": "eip712",
    }
    assert rows == expected
    for fact in (
        "`v7.0`", "gates activation by upgrade and module state",
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads before store deletion are archive-only",
        "External checkpoint registration", "settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")


def test_readme_local_run_guidance_uses_merged_paxd_lifecycle():
    build = section("Build")
    commands = [
        shlex.split(line, comments=True)
        for block in re.findall(r"```sh\n(.*?)\n```", build, re.S)
        for line in block.splitlines()
        if line.strip()
    ]
    assert ["make", "paxd-build"] in commands
    assert ["./build/paxd", "version"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands
    assert all("layerxd" not in arg and "checkpoint" not in arg for cmd in commands for arg in cmd)
    for fact in (
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "operator-provided genesis and consensus chain identifier",
        "Building the binary alone does not provision network membership",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release


def test_repair_record_maps_finding_to_declared_source_proofs():
    record = tomllib.loads(source("tools/qualification/audit_repairs/repair_22_33_1213.kvx"))
    finding = tomllib.loads(source(
        "spec/layerx-merge/programme/audit/records/22.30.57.kvx"
    ))["finding"]["readme-network-model"]
    closure = record["finding"]["readme-network-model"]
    assert finding["file"] == closure["file"] == "README.md"
    assert finding["class"] == closure["class"] == "doc-drift"
    assert finding["severity"] == closure["severity"] == "medium"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(closure["tests"]) == declared - {
        "test_repair_record_maps_finding_to_declared_source_proofs"
    }
    assert closure["owner_task"] == "22.31.470"
    assert closure["wiring_requests"] == []
    assert record["repair"]["task"] == "22.33.1213"
    assert record["qualification"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1213.py"
    )
    assert record["qualification"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not executed" in record["proof"]["review_only"]
