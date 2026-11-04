"""Qualify root-network-model against the inherited README and real sources.

Offline documentation evidence only: no native runtime, network or subprocesses.
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
RECORD = Path(__file__).with_name("repair_22_33_1024.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep fenced comments inside their Markdown section."""
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


def test_root_network_model_embeds_kernel_in_chain_125():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no router and no separate LayerX interface",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
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
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State", "outside consensus",
    ):
        assert fact in topology, fact


def test_root_network_model_public_rpc_hosts_and_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "compare `eth_blockNumber`", "never a JSON-RPC endpoint",
        "node.hyperpaxeer.com` is only the hpx library mirror",
    ):
        assert fact in quickstart, fact
    urls = re.findall(r"https?://[^\s`<>)]+", quickstart)
    node_urls = [urlsplit(url) for url in urls if "mainnet-beta.paxeer.network" in url]
    assert {url.hostname for url in node_urls} == {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    assert all(url.scheme == "https" and url.path in ("", "/") for url in node_urls)
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
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
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_root_network_model_hosted_contract_is_explicit():
    quickstart = section("Quick start: connect to chain 125")
    contract_path = "services/platform/hosted/gateway/openrpc.json"
    assert f"]({contract_path})" in quickstart
    for fact in (
        "Obtain its deployment URL and credentials from the operator",
        "gateway contract does not establish method availability at public node URLs",
        "`px_getReceiptKeys` is registered by the node",
        "gateway relays its governed key ranges",
    ):
        assert fact in quickstart, fact
    contract = json.loads(source(contract_path))
    assert "openrpc" in contract
    assert {"px_getBalances", "px_getReceiptKeys"} <= {
        method["name"] for method in contract["methods"]
    }
    server = source("rpc/server.go")
    assert '{Namespace: "px", Service: pxAPI}' in server
    assert '{Namespace: "px", Service: &pxHistoryAPI{account: pxAPI}}' in server
    assert 'Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}' in server
    assert "optional developer tooling" in section("Integrated network topology")


def test_root_network_model_transaction_producer_transport_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "Programs payload bytes go to `0x101D`", "durable WAL acknowledgement",
        "Receipt order is proposal order", "configured proposing node",
        "No separate service endpoint is required",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    for fact in (
        "func (s *SendAPI) SendRawTransaction(",
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ):
        assert fact in send, fact
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    ordered = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    )
    positions = [sequence[0].index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_root_network_model_committed_receipts_and_proof_consumers():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "commitment does not mean execution succeeded",
        "`eth_getTransactionReceipt`", "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in rpc
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in rpc
    proofs = section("Receipt and proof verification")
    for fact in (
        "pinned for the receipt height", "authenticate the block header and CometBFT commit",
        "trusted validator information", "authenticated root at the relevant height and key",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ):
        assert fact in proofs, fact
    assert "Ed25519Digest(sequencerPublicKey, *receipt.SequencerSignature, digest)" in source(
        "layerxproof/verify/receipt.go"
    )
    proof_root = "programs/crates/layerx-programs-registry/src/chain_proof/"
    assert "self.verify_header_commit(header, commit)?" in source(proof_root + "head_verify.rs")
    assert "proof.verify(&head.data_hash(), &hash)?" in source(proof_root + "tx.rs")
    assert "verify_store_root(store_name, root, &ops[1].data, next.app_hash())?" in source(proof_root + "state.rs")


def test_root_network_model_retired_executors_and_deployments():
    overview = section("What is Paxeer X Network?", 2)
    assert "no separate kernel daemon, sequencer service or checkpoint deployment" in overview
    assert "External checkpoint settlement is retired" in overview
    for stale in (
        r"\blayerxd\b", r"two (?:execution )?domains",
        r"kernel still runs separately", r"merge is (?:only )?a draft",
    ):
        assert not re.search(stale, README, re.I), stale
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    custody = source("precompiles/layerxcustody/layerxcustody.go")
    assert re.search(r"if p\.keeper\.Retired\(ctx\) \{\s+return nil, errors.New\(RetiredRevert\)", custody)
    assert 'RetiredRevert        = "LXP_MODULE_DISABLED:' in custody
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")


def test_root_network_model_supported_build_and_source_links():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    targets = dict(re.findall(r"^([A-Za-z0-9_-]+):([^\n]*)$", makefile, re.M))
    assert targets["paxd-native-libs"].split() == ["$(LIBRARY)", "programs-release-build"]
    assert targets["paxd-build"].split() == ["paxd-native-libs"]
    recipe = makefile.split("\npaxd-build: paxd-native-libs\n", 1)[1].split("\n\n", 1)[0]
    assert "tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    for body in (build, section("Documentation", 2), section("What is Paxeer X Network?", 2)):
        links = re.findall(r"\]\(([^)]+)\)", body)
        assert links
        for target in links:
            if not target.startswith(("https:", "http:", "#")):
                assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_root_network_model_record_binds_finding_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["1024"]
    finding = record["finding"]["root-network-model"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.38.kvx"))
    original = audit["finding"]["root-network-model"]
    assert set(record["finding"]) == {"root-network-model"}
    for key in ("file", "class", "severity", "owner_domain"):
        assert finding[key] == original[key]
    assert finding["audit"] == task["audit"] == "22.30.38"
    assert finding["file"] == "README.md"
    assert finding["repair_owner"] == "22.31.470"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert len(finding["tests"]) == len(declared) == 8
    assert set(finding["tests"]) == declared
    assert task["source_revision"] == "10748f58948a4ac9653274f920da50ecd3a226ca"
    assert task["wiring_requests"] == []
    wiring = record["wiring"]["readme"]
    assert wiring["path"] == "README.md" and wiring["owner_domain"] == 17
    assert wiring["owner_task"] == "22.31.470" and wiring["status"] == "inherited"
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1024.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
