"""Offline source proof for audit 22.30.72/readme-separate-daemon."""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_1337.kvx")


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


def test_readme_describes_one_bft_paxd_with_embedded_kernel():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "C17 kernel in `src/` and `include/` runs inside `paxd`",
        "no router and no separate LayerX interface",
        "outside consensus",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    topology = section("Integrated network topology")
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in topology, fact


def test_root_json_rpc_range_and_chain_125_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert (
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`"
    ) in quickstart
    urls = re.findall(r"https://[A-Za-z0-9./_-]+", quickstart)
    assert set(urls) == {
        "https://api-mainnet-beta.paxeer.network",
        "https://api1.mainnet-beta.paxeer.network",
        "https://api14.mainnet-beta.paxeer.network",
    }
    for url in urls:
        parsed = urlsplit(url)
        assert parsed.scheme == "https"
        assert parsed.path in ("", "/")
        assert not parsed.query and not parsed.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[:4] == ["curl", "--max-time", "10", "-sS"]
    assert argv[4] == "https://api1.mainnet-beta.paxeer.network"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    result = re.search(r'# Expected result: "(0x[0-9a-f]+)"', blocks[0])
    assert result is not None
    assert int(result.group(1), 16) == 125
    assert "`eth_blockNumber`" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart


def test_eip1559_submission_follows_production_send_and_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    steps = re.findall(r"^\d+\. (.+)$", flow, re.M)
    assert len(steps) == 4
    for fact in (
        "`eth_chainId` returns `0x7d`", "`eth_getTransactionCount`",
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`0x101A`–`0x101D`", "Programs payload bytes go to `0x101D`",
    ):
        assert fact in steps[0], fact
    for fact in (
        "`eth_sendRawTransaction`", "root JSON-RPC URL", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
    ):
        assert fact in steps[1], fact
    assert "`pax_sequencerReceipt` with the transaction hash" in steps[2]
    assert "No separate service endpoint is required" in steps[2]
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < (
        release.index("a.src.Record(common.Hash(acc.ActivityID), send)")
    )


def test_finality_requires_committed_block_and_receipt_consumer():
    assert "A receipt is final once its block is committed" in README
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "authenticate the block commitment", "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed"
    ]
    assert (
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, "
        "uint64(view.Height))"
    ) in rpc
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in rpc
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    positions = [sequence[0].index(fact) for fact in (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    )]
    assert positions == sorted(positions)
    proof = section("Receipt and proof verification")
    assert "authenticate the block header and CometBFT commit" in proof
    assert "trusted validator information" in proof


def test_custody_and_checkpoint_executors_are_retired():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "External checkpoint registration, guarantor attestations",
        "L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "custody deposit and credit-attestation funding flow is retired" in (
        section("Accounts and native funding")
    )
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source(
        "precompiles/layerxcustody/layerxcustody.go"
    )
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_build_and_release_guidance_uses_integrated_paxd():
    build = section("Build")
    for fact in (
        "Build the merged node from the repository root",
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "running network entry point is `paxd`",
        "EVM chain ID `125` is the signing domain",
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
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build\n" in source("Makefile")
    assert "\npaxd-build: paxd-native-libs\n" in source("Makefile")
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    for stale in (
        r"\blayerxd\b", r"make\s+(?:checkpoint|settlement)(?:[-\w]*)",
        r"merge is (?:only )?a draft", r"kernel still runs separately",
    ):
        assert not re.search(stale, README, re.I), stale
    assert "External checkpoint settlement is retired" in README
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release


def test_repair_record_maps_finding_to_declared_source_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    repair = record["repair"]
    assert repair["task"] == "22.33.1337"
    assert repair["requirement"] == "22998.1"
    assert repair["input_revision"] == "6a96588c94c1a9ec5464168975c00cc2120b9922"
    assert repair["scope"] == [
        "tools/qualification/audit_repairs/repair_22_33_1337.kvx",
        "tools/qualification/audit_repairs/test_repair_22_33_1337.py",
    ]
    finding = record["finding"]["readme-separate-daemon"]
    assert finding["audit"] == "22.30.72"
    assert finding["path"] == "README.md"
    assert finding["owner_domain"] == 8
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_repair_record_maps_finding_to_declared_source_proofs"
    }
    assert len(finding["tests"]) == 6
    assert "22.31.470" in repair["completed_wiring_request"]
    assert "README.md" in repair["completed_wiring_request"]
    assert repair["missing_fact_wiring_requests"] == []
    assert "No pre-change verification was run" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    assert repair["build_cmd"] == "python3 --version"
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/"
        "test_repair_22_33_1337.py"
    )
