"""Offline source proof for audit 22.30.61/readme-retired-runtime."""

import json
from pathlib import Path
import re
import shlex
import tomllib


ROOT = Path(__file__).resolve().parents[3]
RECORD = ROOT / "spec/layerx-merge/programme/records/tools/qualification/audit_repairs/repair_22_33_1250.kvx"


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    readme = source("docs/readme/NETWORK.md")
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


def test_readme_retired_runtime_one_embedded_network_and_commit_finality():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "The LayerX kernel runs inside every `paxd` node.",
        "chain ID **125** (`0x7d`)",
        "It executes each transaction when it arrives, assigns the next position "
        "in one global sequence, returns a signed execution receipt and builds "
        "each block.",
        "Validators check that block and commit it.",
        "A receipt is final when the validators commit its block.",
        "A receipt is final when the validators commit its block.",
        "A receipt is final once its block is committed",
        "A signed execution receipt can arrive before that commitment",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "validators commit it in the same block" in release
    for stale in (
        r"\blayerxd\b",
        r"two (?:execution )?domains",
        r"kernel still runs separately",
        r"merge is (?:only )?a draft",
    ):
        assert not re.search(stale, source("docs/readme/NETWORK.md"), re.I), stale


def test_readme_retired_runtime_chain_125_root_rpc_hosts_and_request():
    quickstart = section("Quick start: connect to chain 125")
    assert "Set `PAXEER_RPC_URL` to a node root URL from the [public RPC reference]" in quickstart
    assert "Send HTTPS JSON-RPC requests to the root path without a route suffix" in quickstart
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert "$PAXEER_RPC_URL" in argv
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_retired_runtime_transaction_to_committed_receipt_path():
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
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt


def test_readme_retired_runtime_compatibility_surfaces_refuse():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "they do not provide current funding or finality paths",
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


def test_readme_retired_runtime_paxd_build_and_start_replace_daemon():
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    commands = [
        shlex.split(line, comments=True)
        for block in blocks
        for line in block.splitlines()
        if line.strip()
    ]
    assert ["make", "paxd-build"] in commands
    assert ["./build/paxd", "version"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands
    assert "build/liblayerx.a" in build
    assert "programs/target/release/liblayerx_programs_sandbox.a" in build
    assert "running network entry point is `paxd`" in build
    makefile = source("Makefile")
    dependencies = dict(re.findall(r"^([\w-]+):[ \t]*([^\n]*)$", makefile, re.M))
    assert dependencies["paxd-build"].split() == ["paxd-native-libs"]
    assert dependencies["paxd-native-libs"].split() == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")


def test_readme_retired_runtime_record_maps_finding_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["readme-retired-runtime"]
    assert finding["audit"] == "22.30.61"
    assert finding["file"] == "README.md"
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    expected = {
        name for name, value in globals().items()
        if name.startswith("test_readme_retired_runtime_") and callable(value)
    }
    assert set(finding["tests"]) == expected
    assert len(finding["tests"]) == len(expected)
    task = record["task"]["22"]["33"]["1250"]
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_readme_retired_runtime.py"
    )
    assert task["build_cmd"] == "python3 --version"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["readme_owner_task"] == "22.31.470"
    assert task["wiring_requests"] == []
