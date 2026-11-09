"""Finding 051: offline source qualification of the README owned by 22.31.470.

These checks trace documentation to production sources, without claiming native
execution, cryptographic verification, recovery or public endpoint availability.
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
RECORD = Path(__file__).with_name("repair_22_33_1419.kvx")


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


def require(text, *facts):
    for fact in facts:
        assert fact in text, fact


def test_051_single_paxd_network_and_embedded_block_producer():
    overview = section("What is Paxeer X Network?", 2)
    require(
        overview, "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface", "outside consensus",
        "durable WAL for recovery", "without an external settlement service",
    )
    require(source("node/app.go"), "app.LayerXHost, err = NewHost(")
    require(source("modules/lxphost/host.go"), 'import "C"')
    require(
        source("node/sequencer/consensus_source.go"),
        "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)",
    )


def test_051_root_rpc_hosts_and_chain_probe_match_node_transport():
    quickstart = section("Quick start: connect to chain 125")
    require(
        quickstart, "Public node JSON-RPC uses the root URL",
        "root path of these hosts, without a route suffix",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "compare `eth_blockNumber`",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    )
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network",
    ]
    assert '# Expected result: "0x7d"' in blocks[0]
    assert hex(125) == "0x7d"
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    hosts = {f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)}
    for url in urls:
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.scheme == "https"
            assert parsed.path in ("", "/") and not parsed.query and not parsed.fragment
            assert parsed.hostname in hosts | {"api-mainnet-beta.paxeer.network"}
    require(source("rpc/rpcstack.go"), 'if path == "" {\n\t\treturn r.URL.Path == "/"')
    require(
        source("rpc/info.go"), "func (i *InfoAPI) ChainId(ctx context.Context) *hexutil.Big",
        "return (*hexutil.Big)(i.keeper.ChainID(i.ctxProvider(LatestCtxHeight)))",
    )
    require(source("rpc/server.go"), 'Namespace: "eth",', "Service:   NewInfoAPI(")


def test_051_hosted_contract_is_distinct_from_public_node_methods():
    quickstart = section("Quick start: connect to chain 125")
    require(
        quickstart, "services/platform/hosted/gateway/openrpc.json",
        "Obtain its deployment URL and credentials from the operator",
        "gateway contract does not establish method availability at public node URLs",
        "`px_getReceiptKeys` is registered by the node",
        "gateway relays its governed key ranges",
    )
    contract = json.loads(source("services/platform/hosted/gateway/openrpc.json"))
    methods = {method["name"] for method in contract["methods"]}
    assert {"px_getBalances", "px_getReceiptKeys"} <= methods
    require(
        source("rpc/server.go"), '{Namespace: "px", Service: pxAPI}',
        'Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}',
    )


def test_051_transaction_and_committed_receipt_trace_real_sources():
    flow = section("Submit a transaction and follow its committed receipt")
    require(
        flow, "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "served by sequencer", "commitment does not mean execution succeeded",
        "`eth_getTransactionReceipt`", "`eth_call` at an explicit committed height",
    )
    require(
        source("rpc/send.go"), "func (s *SendAPI) SendRawTransaction(",
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)", "s.sendConfig.Sequencer.Accept(txbz)",
    )
    accepter = source("node/sequencer/accepter.go")
    assert accepter.index("a.rep.Release(context.Background(), &wal.Accept{") < accepter.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    require(accepter, "if err == nil && ack.Status != AckOK", "a.halted = true")
    rpc = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed",
    ]
    require(
        rpc, 'const SequencerReceiptMethod = "pax_sequencerReceipt"',
        'return SequencerReceiptMethod + " served by sequencer"',
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))",
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted",
        'BlockNumber *hexutil.Uint64 `json:"blockNumber,omitempty"`',
        'BlockHash   *common.Hash    `json:"blockHash,omitempty"`',
    )
    proof = section("Receipt and proof verification")
    require(
        proof, "ProtocolReceipt v2", "pinned for the receipt height",
        "public key supplied beside an untrusted receipt does not establish trust",
        "authenticate the block header and CometBFT commit",
        "authenticated root at the relevant height and key",
    )
    require(source("node/sequencer/receipt.go"), "receiptProtocolVersion uint16 = 2")
    require(source("node/sequencer/receiptkey.go"), "ed25519.Sign(k.private, digest[:])")
    require(rpc, "keys.KeyForHeight(decoded.ContextHash, uint64(want.Height))",
            "verify.ReceiptSignature(r.Receipt, [32]byte(r.SigningKey))")
    require(source("layerxproof/verify/receipt.go"), "codec.DecodeReceipt(receiptBytes)")


def test_051_custody_and_checkpoint_executors_are_retired_refusals():
    native = section("EVM and Native Precompiles")
    require(
        native, "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "Historical anchor reads before store deletion are archive-only",
        "guarantor attestations and the L0–L4 settlement ladder are retired",
    )
    require(section("Accounts and native funding"),
            "custody deposit and credit-attestation funding flow is retired")
    require(source("precompiles/layerxcustody/layerxcustody.go"),
            "if p.keeper.Retired(ctx) {\n\t\treturn nil, errors.New(RetiredRevert)",
            'RetiredRevert        = "LXP_MODULE_DISABLED: module disabled; "')
    require(source("precompiles/layerxverify/layerxverify.go"),
            "if pcommon.RetiredAt(ctx, p.upgrades, pcommon.MergeUpgrade, ctx.BlockHeight()) {",
            'return pcommon.RetiredRevert("layerxVerify"), vm.ErrExecutionReverted')
    require(source("precompiles/layerxanchor/layerxanchor.go"),
            "if !isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade",
            "if pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade")
    require(source("node/upgrades.go"),
            'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"')


def test_051_build_commands_target_the_merged_node_without_obsolete_steps():
    build = section("Build")
    require(
        build, "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    )
    makefile = source("Makefile")
    require(makefile, "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n",
            "\npaxd-build: paxd-native-libs\n",
            "PAXEER_MAKE := $(MAKE) -f $(PAXEER_DIR)/chain.mk",
            "version=$$(jq -er .version tools/release/version.json)",
            'GOPROXY=off $(PAXEER_MAKE) build VERSION="$$version"')
    require(source("chain.mk"), "\nbuild: paxd-native-libs\n",
            "go build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd",
            "sdk/version.Version=$(VERSION)",
            "BUILD_FLAGS = -tags \"$(build_tags)\" -ldflags '$(paxd_ldflags)'")
    require(source("daemon/paxd/cmd/root.go"), "server.AddCommands(")
    release = section("Releases and Network Upgrades")
    require(release, "executes kernel activity inside `paxd`",
            "commits it with the BFT chain state")
    for stale in (
        r"\blayerxd\b", r"\blx_sendActivity\b", r"/rpc(?:/|\b)",
        r"merge is (?:only )?a draft", r"kernel still runs separately",
    ):
        assert not re.search(stale, README, re.I), stale


def test_051_repair_record_maps_finding_proofs_and_private_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["1419"]
    finding = record["finding"]["051"]
    assert finding["audit"] == "22.30.79"
    assert finding["file"] == "README.md" and finding["owner_domain"] == 16
    assert finding["repair_owner"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_051_repair_record_maps_finding_proofs_and_private_evidence",
    }
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1419.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    require(task["evidence"], "same candidate revision", "command, exit code and log path")
    require(task["review_only"], "not executed", "restart/recovery")
    assert task["wiring_requests"] == []
    require(task["baseline"], "No pre-change verification", "No baseline failures are claimed")
