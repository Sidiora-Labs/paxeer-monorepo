"""Qualify root-separate-daemon against the README owned by task 22.31.470.

These are offline documentation and production-source checks, not execution of
the node, a recovery exercise, or a claim about public endpoint availability.
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
RECORD = Path(__file__).with_name("repair_22_33_763.kvx")


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
            elif (
                ticks[0] == fence[0]
                and len(ticks) >= len(fence)
                and not suffix.strip()
            ):
                fence = None
        elif fence is None and re.match(r"^#{1," + str(level) + r"} ", line):
            break
        lines.append(line)
    return "".join(lines)


def test_root_separate_daemon_is_replaced_by_one_integrated_paxd_network():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "Paxeer mainnet-beta is one BFT network",
        "EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    assert re.search(r"\blayerxd\b", README, re.I) is None


def test_mainnet_beta_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert (
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`"
    ) in quickstart
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    endpoints = [
        urlsplit(url) for url in urls
        if urlsplit(url).hostname in allowed_hosts
    ]
    assert {url.hostname for url in endpoints} >= {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    for url in map(urlsplit, urls):
        if url.hostname and url.hostname.endswith("mainnet-beta.paxeer.network"):
            assert url.hostname in allowed_hosts
            assert url.scheme == "https" and url.path in ("", "/")
            assert not url.query and not url.fragment and url.port is None
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    command = blocks[0].split("\n# Expected result:")[0].replace("\\\n", " ")
    argv = shlex.split(command)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert '# Expected result: "0x7d"' in blocks[0]
    assert "never a JSON-RPC endpoint" in quickstart
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source(
        "rpc/rpcstack.go"
    )


def test_transaction_receipt_producer_transport_and_commit_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`eth_getTransactionCount`",
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_sendRawTransaction`", "root JSON-RPC URL", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`pax_sequencerReceipt`", "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "No separate service endpoint is required",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "inspect its status and logs",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    accepter = source("node/sequencer/accepter.go")
    assert accepter.index("a.rep.Release(context.Background(), &wal.Accept{") < (
        accepter.index("a.src.Record(common.Hash(acc.ActivityID), send)")
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed",
    ]
    assert (
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, "
        "uint64(view.Height))"
    ) in receipt
    assert (
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted"
    ) in receipt
    assert "A receipt is final once its block is committed" in README
    proof = section("Receipt and proof verification")
    assert "pinned for the receipt height" in proof
    assert "authenticate the block header and CometBFT commit" in proof
    assert "Neither a transaction hash nor a signed execution receipt alone" in proof


def test_topology_deployment_and_recovery_stay_inside_paxd():
    topology = section("Integrated network topology")
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State", "outside consensus",
        "durable WAL for recovery", "restart and recovery belong to the node lifecycle",
        "without an external settlement service",
    ):
        assert fact in topology, fact
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    # Read target declarations, not a build toolchain or shell subprocess.
    targets = dict(re.findall(r"^([\w-]+):([^\n]*)$", source("Makefile"), re.M))
    assert targets["paxd-native-libs"].split() == [
        "$(LIBRARY)", "programs-release-build", "evmone-lib",
    ]
    assert targets["paxd-build"].split() == ["paxd-native-libs"]
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release


def test_custody_and_checkpoint_executors_are_explicitly_retired():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "archive-only",
        "External checkpoint registration, guarantor attestations",
        "L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source(
        "precompiles/layerxcustody/layerxcustody.go"
    )
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert (
        "!isView(method.Name) && "
        "pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade"
    ) in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for stale in (r"\blayerxd\b", r"\blx_[A-Za-z0-9_*]*", r"/rpc(?:/|\b)"):
        assert re.search(stale, README, re.I) is None, stale


def test_repair_record_maps_finding_to_executable_proofs_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["root-separate-daemon"]
    assert finding["audit"] == "22.30.172"
    assert finding["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["repair"]
    assert task["id"] == "22.33.763"
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_763.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert "README.md" in task["wiring_request"]
    assert "domain 16" in task["wiring_request"]
    assert "22.31.470" in task["wiring_request"]
    assert task["missing_facts"] == []
