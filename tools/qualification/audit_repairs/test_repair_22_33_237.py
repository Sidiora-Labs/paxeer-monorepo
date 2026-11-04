"""Offline source proof for audit 22.30.128/readme-network (decision 2525).

Read the real README and production sources; do not run a node or contact RPC.
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


def test_readme_network_embeds_kernel_and_proposes_committed_blocks():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network",
        "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "runs inside `paxd` through",
        "no router and no separate LayerX interface",
        "A receipt is final once its block is committed",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release


def test_readme_network_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert "`https://api1.mainnet-beta.paxeer.network` through " in quickstart
    assert "`https://api14.mainnet-beta.paxeer.network`" in quickstart
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    allowed = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = [
        urlsplit(url)
        for url in re.findall(r"https?://[^\s`<>)]+", README)
        if "mainnet-beta.paxeer.network" in url
    ]
    assert urls
    for url in urls:
        assert url.scheme == "https" and url.hostname in allowed
        assert url.path in ("", "/") and not url.query and not url.fragment
        assert url.port is None and url.username is None and url.password is None
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.DOTALL)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_network_transaction_transport_and_committed_receipt():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`",
        "`eth_sendRawTransaction`",
        "transaction hash",
        "durable WAL acknowledgement",
        "`pax_sequencerReceipt`",
        "`submitted`", "`receipted`", "`committed`",
        "`MsgSequenceBatch`",
        "Wait for `committed` and authenticate the block commitment",
        "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
        "No separate service endpoint is required",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "forwardRawTransaction(ctx, s.sendConfig.SequencerURL, hash, input)" in send
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
    diagram = re.search(r"```mermaid\nsequenceDiagram\n(.*?)\n```", flow, re.DOTALL)
    assert diagram is not None
    ordered = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    )
    positions = [diagram[1].index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_readme_network_requires_block_commitment_for_finality():
    proof = section("Receipt and proof verification")
    for fact in (
        "ProtocolReceipt v2",
        "pinned for the receipt height",
        "public key supplied beside an untrusted receipt does not establish trust",
        "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit",
        "trusted validator information",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ):
        assert fact in proof, fact
    chain_proof = "programs/crates/layerx-programs-registry/src/chain_proof/"
    assert "self.verify_header_commit(header, commit)?" in source(chain_proof + "head_verify.rs")
    assert "proof.verify(&head.data_hash(), &hash)?" in source(chain_proof + "tx.rs")


def test_readme_network_retires_custody_checkpoint_and_daemon_deployments():
    overview = section("What is Paxeer X Network?", 2)
    assert "no separate kernel daemon, sequencer service or checkpoint deployment" in overview
    assert "External checkpoint settlement is retired" in overview
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    retirement = [p for p in README.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(retirement) == 1
    for fact in (
        "`layerxCustody` (`0x1013`)",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "historical views retire at `v7.0-anchor-store-delete`",
        "archive-only",
    ):
        assert fact in retirement[0], fact
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    current = README.replace(retirement[0], "")
    for name in ("layerxCustody", "layerxcustody", "layerxAnchor", "layerxanchor"):
        assert name not in current, name
    for stale in (
        r"\blayerxd\b", r"kernel still runs separately", r"two execution domains",
        r"merge is (?:only )?a draft", r"/rpc(?:/|\b)",
    ):
        assert not re.search(stale, README, re.IGNORECASE), stale
    build = section("Build")
    assert "make paxd-build" in build
    assert './build/paxd start --home "$PAXD_HOME"' in build
    assert "The running network entry point is `paxd`" in build


def test_readme_network_record_maps_finding_to_declared_proofs():
    record = tomllib.loads(Path(__file__).with_name("repair_22_33_237.kvx").read_text())
    finding = record["finding"]["readme-network"]
    assert finding["audit"] == "22.30.128"
    assert finding["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    assert finding["owner_domain"] == 13
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_readme_network_record_maps_finding_to_declared_proofs"
    }
    task = record["task"]["22"]["33"]["237"]
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_237.py"
    )
    assert task["wiring_requests"] == []
    assert task["source_proof"] and task["review_only"] and task["baseline"]
