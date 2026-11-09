"""Offline source proof for audit 22.30.167/readme-network-doc.

Read the README produced by 22.31.470 and its real implementation anchors.
These checks do not execute the node, build toolchains or network requests.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
RECORD = Path(__file__).with_name("repair_22_33_686.kvx")
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


def test_readme_network_doc_one_paxd_embedded_kernel_and_commit_finality():
    overview = section("What is Paxeer X Network?", level=2)
    for fact in (
        "one BFT network",
        "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "participates in block proposal and deterministic execution",
        "kernel is part of the network and proposes blocks inside `paxd`",
        "A receipt is final once its block is committed",
        "There is no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    topology = section("Integrated network topology")
    assert "subgraph Node[paxd]" in topology
    assert "Embedded C17 kernel and Rust programs host" in topology
    assert "Validators -->|Commit| State" in topology
    for stale in (
        r"\blayerxd\b",
        r"two (?:execution )?domains",
        r"kernel still runs separately",
        r"merge (?:is|remains) (?:only )?(?:a )?draft",
    ):
        assert not re.search(stale, README, re.I), stale


def test_readme_network_doc_chain_125_root_json_rpc_contract():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "https://api-mainnet-beta.paxeer.network",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
    ):
        assert fact in quickstart, fact
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = re.findall(r"https://api[^\s`'\"<>)]*", README)
    assert urls
    for url in urls:
        parsed = urlsplit(url)
        assert parsed.scheme == "https" and parsed.hostname in allowed_hosts, url
        assert parsed.path in ("", "/"), url
        assert not parsed.query and not parsed.fragment, url
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


def test_readme_network_doc_transaction_to_committed_receipt_source_path():
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
        "No separate service endpoint is required",
        "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
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
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in rpc
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in rpc
    proof = section("Receipt and proof verification")
    assert "pinned for the receipt height" in proof
    assert "authenticate the block header and CometBFT commit" in proof
    assert "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`" in proof
    assert "ed25519.Sign(k.private, digest[:])" in source("node/sequencer/receiptkey.go")
    assert "Ed25519Digest(sequencerPublicKey, *receipt.SequencerSignature, digest)" in source(
        "layerxproof/verify/receipt.go"
    )


def test_readme_network_doc_retired_custody_and_checkpoint_executors():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
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
    for paragraph in README.split("\n\n"):
        if re.search(r"\blayerx(?:Verify|Custody|Anchor)\b", paragraph):
            assert "retired refusal surfaces" in paragraph and "archive-only" in paragraph


def test_readme_network_doc_merged_node_build_and_release():
    build = section("Build")
    for fact in (
        "make paxd-build",
        "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json",
        './build/paxd start --home "$PAXD_HOME"',
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    recipe = makefile.split("\npaxd-build: paxd-native-libs\n", 1)[1].split("\n\n", 1)[0]
    assert "tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    release = section("Releases and Network Upgrades")
    for fact in (
        "executes kernel activity inside `paxd`",
        "commits it with the BFT chain state",
        "`v7.0` merge handler",
        "retires the old custody/checkpoint write paths",
        "`v7.0-anchor-store-delete` removes the historical anchor store",
    ):
        assert fact in release, fact
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")


def test_readme_network_doc_record_maps_finding_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["686"]
    finding = record["finding"]["readme-network-doc"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.167.kvx"))
    original = audit["finding"]["readme-network-doc"]
    assert original["file"] == finding["file"] == "README.md"
    assert original["class"] == finding["class"] == "doc-drift"
    assert original["severity"] == finding["severity"] == "medium"
    assert finding["audit"] == "22.30.167"
    assert finding["owner_task"] == "22.31.470"
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_686.py"
    )
    assert task["build_cmd"] == "python3 --version"
    assert task["input_revision"] == "3a58ad3f1861ab11f8cccd9b14c5959263c79c28"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["verified_revision"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert task["missing_facts"] == []
    assert record["wiring"]["readme"]["path"] == "README.md"
    assert record["wiring"]["readme"]["owner_domain"] == 17
    assert record["wiring"]["readme"]["owner_task"] == "22.31.470"
    assert record["wiring"]["readme"]["status"] == "satisfied_by_prerequisite"
