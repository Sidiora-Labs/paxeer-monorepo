"""Offline source proof for audit 22.30.121/doc-03, consuming task 22.31.470.

Read the actual README and production wiring; do not execute native toolchains,
contact public nodes, or claim runtime/finality certification from documentation.
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
RECORD = Path(__file__).with_name("repair_22_33_216.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    marker = "#" * level + " " + heading + "\n"
    assert README.count(marker) == 1, heading
    body = []
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
        body.append(line)
    return "".join(body)


def make_rule(text, target):
    match = re.search(r"^" + re.escape(target) + r":([^\n]*)\n((?:\t[^\n]*\n)*)", text, re.M)
    assert match, target
    return match.group(1).split(), match.group(2)


def test_doc03_one_paxd_network_embeds_kernel_and_commits_finality():
    overview = section("What is Paxeer X Network?", level=2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "participates in block proposal and deterministic execution",
        "kernel is part of the network and proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "There is no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "Sub-chains are tenant domains stored in the database",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert len(diagrams) == 2
    topology, sequence = diagrams
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native", "Validators -->|Commit| State",
    ):
        assert fact in topology, fact
    events = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL", "Release signed execution receipt",
        "receipted (execution evidence)", "Propose MsgSequenceBatch in receipt order",
        "Commit block", "committed, blockNumber, blockHash",
    )
    positions = [sequence.index(event) for event in events]
    assert positions == sorted(positions)


def test_doc03_root_rpc_range_and_chain_125_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert "gateway contract does not establish method availability at public node URLs" in quickstart
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    allowed = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = [
        urlsplit(url) for url in re.findall(r"https?://[^\s`<>)]+", README)
        if "mainnet-beta.paxeer.network" in url
    ]
    assert urls
    for url in urls:
        assert url.scheme == "https" and url.hostname in allowed
        assert url.path in ("", "/") and not url.query and not url.fragment
        assert url.port is None and url.username is None and url.password is None
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:", 1)[0].replace("\\\n", " "))
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


def test_doc03_transaction_and_receipt_producer_transport_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`eth_getTransactionCount`", "secp256k1 EIP-1559 transaction for chain ID `125`",
        "Programs payload bytes go to `0x101D`", "`eth_sendRawTransaction`",
        "transaction hash", "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet", "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height", "No separate service endpoint is required",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    for call in (
        "func (s *SendAPI) SendRawTransaction(", "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)", "s.sendConfig.Sequencer.Accept(txbz)",
    ):
        assert call in send, call
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == ["submitted", "receipted", "committed"]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt
    proof = section("Receipt and proof verification")
    assert "authenticate the block header and CometBFT commit" in proof
    assert "trusted validator information" in proof
    assert "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`" in proof


def test_doc03_programs_precompile_matches_production_registration():
    native = section("EVM and Native Precompiles")
    rows = re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native)
    assert len(rows) == len(dict(rows)) == 13
    registered = re.search(
        r"var MergeAddressTable = \[\]MergeAddress\{(.*?)\n\}", source("precompiles/setup.go"), re.S
    )
    assert registered
    entries = re.findall(r'\{ecommon.HexToAddress\("(0x[0-9A-Fa-f]+)"\), "([^"]+)"\}', registered.group(1))
    assert len(entries) == 13
    assert {int(address, 16): name for address, name in rows} == {
        int(address, 16): name for address, name in entries
    }
    assert dict(rows)["0x101D"] == "layerxprograms"
    assert "gates activation by upgrade and module state" in native
    programs = source("precompiles/layerxprograms/layerxprograms.go")
    assert "precompiles.RegisterMergeConstructor(Address, NewMergePrecompile)" in programs
    assert "NewPrecompile(wired.LayerXHost(), evm, upgrade, planName)" in programs


def test_doc03_custody_and_checkpoint_executors_are_retired():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility, not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads before store deletion are archive-only",
        "they do not provide current funding or finality paths",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    custody = source("precompiles/layerxcustody/layerxcustody.go")
    assert "if p.keeper.Retired(ctx) {\n\t\treturn nil, errors.New(RetiredRevert)" in custody
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    paragraphs = [p for p in README.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(paragraphs) == 1
    current = README.replace(paragraphs[0], "")
    assert not re.search(r"\blayerx(?:Verify|Custody|Anchor)\b", current)
    assert not re.search(r"\blayerxd\b|\bmake\b[^\n]*\blayerxd\b", README)


def test_doc03_build_and_merged_developer_path():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd start --home \"$PAXD_HOME\"", "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    prerequisites, recipe = make_rule(makefile, "paxd-build")
    assert prerequisites == ["paxd-native-libs"]
    assert "tools/release/version.json" in recipe and "$(PAXEER_MAKE) build" in recipe
    prerequisites, _ = make_rule(makefile, "paxd-native-libs")
    assert prerequisites == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
    assert "LIBRARY := $(BUILD_DIR)/liblayerx.a" in makefile
    chain = source("chain.mk")
    prerequisites, recipe = make_rule(chain, "paxd-native-libs")
    assert prerequisites == []
    assert "-f Makefile paxd-native-libs" in recipe
    assert "-o ./build/paxd ./daemon/paxd" in chain
    docs = section("Documentation", level=2)
    assert "Build the merged node using the [commands below](#build)" in docs
    for path in (
        "docs/site/paxeer-docs/src/app/public-rpc/page.tsx", "docs/wiki/Getting-Started-Beta.md",
        "docs/wiki/PaymentsQuickstart.md", "node/sequencer/config.go",
    ):
        assert path in README and (ROOT / path).is_file(), path
    for target in re.findall(r"\]\(([^)]+)\)", README):
        if not target.startswith(("https:", "http:", "#")):
            assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_doc03_record_maps_finding_to_proofs_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["216"]
    finding = record["finding"]["doc-03"]
    assert finding["audit"] == "22.30.121" and finding["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert len(finding["tests"]) == len(set(finding["tests"]))
    assert set(finding["tests"]) == declared
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.121.kvx"))
    assert audit["finding"]["doc-03"]["file"] == finding["file"]
    assert audit["finding"]["doc-03"]["class"] == "doc-drift"
    assert audit["finding"]["doc-03"]["severity"] == "medium"
    assert task["base_revision"] == "bae5ccc55ef250ddf26713cac5178b04aaa48803"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_216.py"
    assert task["wiring_requests"] == []
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
