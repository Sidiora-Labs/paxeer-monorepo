"""Offline source proof for audit 22.30.135/root-readme-network-model.

Read the production README inherited from 22.31.470 and its real node sources.
These checks qualify documentation facts, not native execution or deployment.
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
RECORD = Path(__file__).with_name("repair_22_33_313.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep fenced shell comments from being interpreted as Markdown headings."""
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


def test_merged_paxd_network_and_tenant_namespaces():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
        "no router and no separate LayerX interface", "outside consensus",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    assert "modules/layerxchains/" in overview
    assert (ROOT / "modules/layerxchains").is_dir()
    assert not re.search(
        r"two (?:execution )?domains|kernel still runs separately|merge (?:is|remains) (?:only )?(?:a )?draft",
        README, re.I,
    )


def test_root_json_rpc_hosts_and_chain_125_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "Confirm the chain ID before signing or broadcasting",
        "compare `eth_blockNumber`",
        "gateway contract does not establish method availability at public node URLs",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    hosts = {"api-mainnet-beta.paxeer.network"}
    hosts.update(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15))
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    node_urls = [urlsplit(url) for url in urls if "mainnet-beta.paxeer.network" in url]
    assert node_urls
    for url in node_urls:
        assert url.scheme == "https" and url.hostname in hosts
        assert url.path in ("", "/") and not url.query and not url.fragment
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


def test_transaction_transport_and_committed_receipt_consumption():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height", "No separate service endpoint is required",
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
    proof = section("Receipt and proof verification")
    assert "pinned for the receipt height" in proof
    assert "authenticate the block header and CometBFT commit" in proof
    assert "trusted validator information" in proof
    assert "ed25519.Sign(k.private, digest[:])" in source("node/sequencer/receiptkey.go")
    assert "codec.DecodeReceipt(receiptBytes)" in source("layerxproof/verify/receipt.go")


def test_retired_executors_are_separate_from_active_contracts():
    native = section("EVM and Native Precompiles")
    rows = dict(re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native))
    assert rows["0x101A"] == "layerxledger"
    assert rows["0x101B"] == "layerxpay"
    assert rows["0x101C"] == "layerxauth"
    assert rows["0x101D"] == "layerxprograms"
    assert not {"0x1012", "0x1013", "0x1014"} & rows.keys()
    retired = [p for p in native.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(retired) == 1
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "archive-only",
    ):
        assert fact in retired[0], fact
    current = README.replace(retired[0], "")
    assert not re.search(r"layerx(?:Custody|Anchor|Verify)", current)
    assert "archived formats add no settlement step to current block finality" in native
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")


def test_current_node_build_operations_and_no_separate_deployment():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "node/upgrades.go", "node/sequencer/config.go", "services/explorer/",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build\n" in source("Makefile")
    assert "\npaxd-build: paxd-native-libs\n" in source("Makefile")
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    for target in re.findall(r"\]\(([^)]+)\)", build):
        assert (ROOT / target.split("#", 1)[0]).exists(), target
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release
    assert "no separate kernel daemon, sequencer service or checkpoint deployment" in README
    assert "External checkpoint settlement is retired" in README
    assert not re.search(r"\blayerxd\b", README, re.I)


def test_repair_record_maps_finding_to_real_source_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["313"]
    finding = record["finding"]["root-readme-network-model"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.135.kvx"))
    original = audit["finding"]["root-readme-network-model"]
    assert finding["audit"] == "22.30.135"
    assert finding["file"] == original["file"] == "README.md"
    assert finding["class"] == original["class"] == "doc-drift"
    assert original["severity"] == "medium"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert task["wiring_requests"] == []
    assert task["readme_owner"] == "domain 17; task 22.31.470"
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_313.py"
    )
