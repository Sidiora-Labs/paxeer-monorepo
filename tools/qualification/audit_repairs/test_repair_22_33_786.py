"""Offline source-contract proof for audit 22.30.174/doc-1 (decision 2525)."""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_786.kvx")


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


def require_facts(text, facts):
    for fact in facts:
        assert fact in text, fact


def test_doc_1_merged_paxd_network_programs_and_tenant_domains():
    require_facts(section("What is Paxeer X Network?", 2), (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "programs sandbox", "Sub-chains are tenant domains stored in the database",
        "managed by [`modules/layerxchains`]", "namespaces state within the shared network",
        "does not require new node infrastructure", "no router and no separate LayerX interface",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
    ))
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    config = source("modules/evm/config/config.go")
    assert re.search(r'"hyperpax_125-1":\s*int64\(125\)', config)


def test_doc_1_public_root_rpc_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    require_facts(quickstart, (
        "Public node JSON-RPC uses the root URL",
        "https://api-mainnet-beta.paxeer.network",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix", "compare `eth_blockNumber`",
        "never a JSON-RPC endpoint",
    ))
    hosts = {"api-mainnet-beta.paxeer.network"} | {
        f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)
    }
    urls = [urlsplit(url) for url in re.findall(r"https?://[^\s`<>)]+", README)]
    rpc_urls = [url for url in urls if url.hostname in hosts]
    assert rpc_urls
    for url in rpc_urls:
        assert url.scheme == "https"
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


def test_doc_1_transaction_transport_and_committed_receipt_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    require_facts(flow, (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "Programs payload bytes go to `0x101D`", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "served by sequencer", "No separate service endpoint is required",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ))
    send = source("rpc/send.go")
    require_facts(send, (
        "func (s *SendAPI) SendRawTransaction(", "tx.UnmarshalBinary(input)",
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ))
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    require_facts(receipt, (
        'const SequencerReceiptMethod = "pax_sequencerReceipt"',
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))",
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted",
        'return SequencerReceiptMethod + " served by sequencer"',
    ))


def test_doc_1_gateway_contract_is_distinct_from_public_node():
    require_facts(section("Quick start: connect to chain 125"), (
        "services/platform/hosted/gateway/openrpc.json",
        "Obtain its deployment URL and credentials from the operator",
        "gateway contract does not establish method availability at public node URLs",
        "`px_getReceiptKeys` is registered by the node",
        "gateway relays its governed key ranges",
    ))
    contract = json.loads(source("services/platform/hosted/gateway/openrpc.json"))
    methods = {method["name"]: method for method in contract["methods"]}
    assert {"pax_sequencerReceipt", "px_getReceiptKeys", "px_getBalances"} <= methods.keys()
    params = methods["pax_sequencerReceipt"]["params"]
    assert len(params) == 1
    assert params[0]["name"] == "txHash" and params[0]["required"] is True
    assert params[0]["schema"]["pattern"] == "^0x[0-9a-fA-F]{64}$"
    assert 'Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}' in source("rpc/server.go")


def test_doc_1_retired_executors_and_no_standalone_deployment():
    require_facts(README, (
        "External checkpoint settlement is retired",
        "custody deposit and credit-attestation funding flow is retired",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "archived formats add no settlement step to current block finality",
    ))
    for stale in (r"\blayerxd\b", r"\bmake\s+checkpoint\b", r"\bmake\s+router\b",
                  r"merge is (?:only )?a (?:future )?draft"):
        assert not re.search(stale, README, re.I), stale
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_doc_1_operator_build_and_start_match_declared_producers():
    build = section("Build")
    require_facts(build, (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ))
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    commands = [shlex.split(line, comments=True) for block in blocks for line in block.splitlines()]
    assert [argv for argv in commands if argv and argv[0] == "./build/paxd"] == [
        ["./build/paxd", "version"], ["./build/paxd", "start", "--home", "$PAXD_HOME"]
    ]
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "tools/release/version.json" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    assert "cmd.NewRootCmd()" in source("daemon/paxd/main.go")
    for target in re.findall(r"\]\(([^)]+)\)", build):
        if not target.startswith(("https:", "http:", "#")):
            assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_doc_1_record_maps_finding_to_declared_proofs_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["doc-1"]
    assert finding["audit"] == "22.30.174" and finding["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470" and finding["owner_domain"] == 16
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_doc_1_")
    }
    assert set(finding["tests"]) == declared - {
        "test_doc_1_record_maps_finding_to_declared_proofs_and_evidence"
    }
    assert finding["wiring_requests"] == []
    qualification = record["qualification"]
    assert qualification["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_786.py"
    )
    assert qualification["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in qualification["evidence"]
    assert "not run" in record["baseline"]["pre_change_verify"]
