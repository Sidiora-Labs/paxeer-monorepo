"""Offline source proof for audit 22.30.75/root-network-overview.

README.md is owned by task 22.31.470. These checks read its real documentation
and production wiring; they do not certify native execution or public services.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
RECORD = Path(__file__).with_name("repair_22_33_1382.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Select one README section, preserving its actual commands and diagrams."""
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


def test_root_network_overview_is_one_embedded_paxd_chain():
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
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    for stale in (
        r"\blayerxd\b",
        r"two (?:execution )?domains",
        r"kernel still runs separately",
    ):
        assert not re.search(stale, source("README.md"), re.I), stale


def test_root_network_overview_documents_root_rpc_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "Confirm the chain ID before signing or broadcasting",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    command, expected = blocks[0].split("\n# Expected result: ")
    assert expected == '"0x7d"'
    argv = shlex.split(command.replace("\\\n", " "))
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    allowed_hosts = {"api-mainnet-beta.paxeer.network"} | {
        f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)
    }
    for url in re.findall(r"https?://[^\s`<>)]+", source("README.md")):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.hostname in allowed_hosts, url
            assert parsed.scheme == "https" and parsed.path in ("", "/"), url
            assert not parsed.query and not parsed.fragment, url
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source(
        "rpc/rpcstack.go"
    )


def test_root_network_overview_submission_to_committed_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`eth_getTransactionCount`",
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_sendRawTransaction`",
        "public node's root JSON-RPC URL",
        "result is a transaction hash",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block whose `MsgSequenceBatch` covers that sequence",
        "authenticate the block commitment before treating the result as final",
        "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    ordered = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
    )
    positions = [flow.index(fact) for fact in ordered]
    assert positions == sorted(positions)
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
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert receipt.index("a.coveringHeight(decoded.GlobalSequence") < receipt.index(
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted"
    )


def test_root_network_overview_retires_custody_and_checkpoint_executors():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'return pcommon.RetiredRevert("layerxVerify"), vm.ErrExecutionReverted' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx) {\n\t\treturn nil, errors.New(RetiredRevert)" in source(
        "precompiles/layerxcustody/layerxcustody.go"
    )
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    for gate in (
        "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade, ctx.BlockHeight())",
        "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade, ctx.BlockHeight())",
    ):
        assert gate + " {\n\t\treturn pcommon.RetiredRevert(PrecompileName)" in anchor


def test_root_network_overview_uses_supported_paxd_startup():
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    commands = [
        shlex.split(line, comments=True)
        for block in blocks for line in block.splitlines() if line.strip()
    ]
    assert ["make", "paxd-build"] in commands
    assert ["./build/paxd", "version"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands
    assert "operator-provided genesis and consensus chain identifier" in build
    assert "running network entry point is `paxd`" in build
    makefile = source("Makefile")
    assert re.search(r"^paxd-native-libs: \$\(LIBRARY\) programs-release-build$", makefile, re.M)
    assert re.search(r"^paxd-build: paxd-native-libs$", makefile, re.M)
    assert "build/liblayerx.a" in build
    assert "programs/target/release/liblayerx_programs_sandbox.a" in build
    assert "\nbuild: paxd-native-libs\n\tgo build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd" in source(
        "chain.mk"
    )
    assert "server.AddCommands(\n\t\trootCmd," in source("daemon/paxd/cmd/root.go")


def test_root_network_overview_record_maps_finding_to_executed_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["repair"]["task"] == "22.33.1382"
    assert record["repair"]["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert record["repair"]["readme_owner"] == "22.31.470 / domain 17"
    finding = record["finding"]["root-network-overview"]
    assert finding["audit"] == "22.30.75" and finding["file"] == "README.md"
    audit = source("spec/layerx-merge/programme/audit/records/22.30.75.kvx")
    block = re.search(r"^\[finding.root-network-overview\]\n(.*?)(?=^\[|\Z)", audit, re.M | re.S)
    assert block is not None
    original = tomllib.loads(block.group(1))
    assert original["class"] == "doc-drift" and original["severity"] == "medium"
    assert finding["expected"] == original["expected"]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_root_network_overview_record_maps_finding_to_executed_proofs"
    }
    assert len(finding["tests"]) == 5
    assert finding["wiring_requests"] == []
    assert "not run" in record["repair"]["baseline"]
    assert "not executed" in record["repair"]["review_only"]
    assert record["evidence"]["ledger"] == "/out/run/gates.jsonl"
    assert record["evidence"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1382.py"
    )
    assert "same candidate revision" in record["evidence"]["verified_revision"]
