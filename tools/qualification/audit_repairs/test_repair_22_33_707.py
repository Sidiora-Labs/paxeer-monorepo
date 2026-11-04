"""Offline source-contract proof for audit 22.30.169/doc-index-7.

Read the production README and its actual build/RPC consumers. These checks do
not execute native code, start a node, or contact the documented endpoints.
"""

import ast
import hashlib
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_707.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Extract a section, keeping heading-shaped comments inside code fences."""
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


def make_rule(text, name):
    """Extract the declared prerequisites and recipe without invoking make."""
    matches = re.findall(
        r"^" + re.escape(name) + r":([^\n]*)\n((?:\t[^\n]*\n)*)",
        text,
        re.M,
    )
    assert len(matches) == 1, name
    prerequisites, recipe = matches[0]
    return prerequisites.split(), recipe


def test_doc_index_7_one_paxd_embeds_native_execution():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network",
        "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "node links `build/liblayerx.a`",
        "programs sandbox",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    host = source("modules/lxphost/host.go")
    assert 'import "C"' in host
    assert "-llayerx -llayerx_programs_sandbox" in host
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    topology = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(topology) == 1
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in topology[0], fact


def test_doc_index_7_chain_125_root_rpc_and_gateway_contract():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "root path of these hosts, without a route suffix",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
        "gateway contract does not establish method availability at public node URLs",
        "Obtain its deployment URL and credentials from the operator",
    ):
        assert fact in quickstart, fact
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert '# Expected result: "0x7d"' in blocks[0]
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = [
        urlsplit(url) for url in re.findall(r"https?://[^\s`<>)]+", README)
        if "mainnet-beta.paxeer.network" in url
    ]
    assert urls
    for url in urls:
        assert url.scheme == "https"
        assert url.hostname in allowed_hosts
        assert url.path in ("", "/")
        assert not url.query and not url.fragment and url.port is None
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")
    contract = "services/platform/hosted/gateway/openrpc.json"
    assert contract in quickstart
    methods = {entry["name"] for entry in json.loads(source(contract))["methods"]}
    assert {"px_getBalances", "px_getReceiptKeys"} <= methods
    assert "`px_getReceiptKeys` is registered by the node" in quickstart
    assert 'Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}' in source("rpc/server.go")


def test_doc_index_7_signed_transaction_to_committed_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`",
        "Broadcast the signed bytes with `eth_sendRawTransaction`",
        "transaction hash",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block",
        "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    assert "A receipt is final once its block is committed" in README
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
    assert "ed25519.Sign(k.private, digest[:])" in source("node/sequencer/receiptkey.go")
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    positions = [sequence[0].index(fact) for fact in (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
    )]
    assert positions == sorted(positions)


def test_doc_index_7_retired_executors_and_no_separate_deployment():
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
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for stale in (
        r"\blayerxd\b", r"\bLNI\b", r"\blx_[A-Za-z0-9_*]*",
        r"merge is (?:still )?(?:only )?(?:a )?draft",
        r"two (?:execution )?domains",
    ):
        assert not re.search(stale, README, re.I), stale
    assert "External checkpoint settlement is retired" in README
    assert "no separate kernel daemon, sequencer service or checkpoint deployment" in README


def test_doc_index_7_build_and_release_inventory_match_paxd():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/paxd", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    prerequisites, recipe = make_rule(makefile, "paxd-build")
    assert prerequisites == ["paxd-native-libs"]
    assert "jq -er .version tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert "sdk/version.Version=$$version" in recipe
    prerequisites, _ = make_rule(makefile, "paxd-native-libs")
    assert prerequisites == ["$(LIBRARY)", "programs-release-build"]
    assert re.search(r"^LIBRARY := \$\(BUILD_DIR\)/liblayerx\.a$", makefile, re.M)
    assert "PAXEER_MAKE := $(MAKE) -f $(PAXEER_DIR)/chain.mk" in makefile
    chain = source("chain.mk")
    prerequisites, recipe = make_rule(chain, "build")
    assert prerequisites == ["paxd-native-libs"]
    assert "go build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd" in recipe
    archives = re.findall(r"^NATIVE_ARCHIVES := (.+)$", chain, re.M)
    assert len(archives) == 1
    assert archives[0].split() == [
        "$(PAXEER_PROJECT_HOME)/build/liblayerx.a",
        "$(PAXEER_PROJECT_HOME)/programs/target/release/liblayerx_programs_sandbox.a",
    ]
    identity = json.loads(source("tools/release/version.json"))
    assert re.fullmatch(r"v\d+\.\d+\.\d+", identity["version"])
    assert identity["upgrade"] == identity["version"]
    assert f"The `{identity['version']}` set" in section("EVM and Native Precompiles")
    release = section("Releases and Network Upgrades")
    for fact in (
        "executes kernel activity inside `paxd`",
        "commits it with the BFT chain state",
        "`v7.0` merge handler activates the merged interfaces",
        "retires the old custody/checkpoint write paths",
        "network's recorded upgrade height",
    ):
        assert fact in release, fact
    assert 'const MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")


def test_doc_index_7_record_maps_finding_to_declared_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["doc-index-7"]
    assert finding["audit"] == "22.30.169"
    assert finding["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    assert finding["wiring_requests"] == []
    assert finding["readme_sha256"] == hashlib.sha256((ROOT / "README.md").read_bytes()).hexdigest()
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_doc_index_7_record_maps_finding_to_declared_proofs"
    }
    task = record["task"]["22"]["33"]["707"]
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_707.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not run" in task["baseline"]
    assert "not executed" in task["review_only"]
