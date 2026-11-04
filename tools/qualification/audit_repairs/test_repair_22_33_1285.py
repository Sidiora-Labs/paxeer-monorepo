"""Offline source contracts for audit 22.30.65/readme-separated-network."""

import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_1285.kvx")


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


def test_readme_separated_network_integrated_paxd():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network",
        "EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "runs inside `paxd` through [`modules/lxphost`]",
        "A receipt is final once its block is committed",
        "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    build = section("Build")
    assert "make paxd-build" in build
    assert './build/paxd start --home "$PAXD_HOME"' in build
    assert "running network entry point is `paxd`" in build


def test_readme_separated_network_root_rpc_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert (
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`"
    ) in quickstart
    hosts = {f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)}
    hosts.add("api-mainnet-beta.paxeer.network")
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    public_urls = [
        urlsplit(url)
        for url in urls
        if (urlsplit(url).hostname or "").endswith("mainnet-beta.paxeer.network")
    ]
    assert public_urls
    for url in public_urls:
        assert url.hostname in hosts
        assert url.scheme == "https"
        assert url.path in ("", "/")
        assert not url.query and not url.fragment and url.port is None
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert "`node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_separated_network_transaction_and_committed_receipt():
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
        "Wait for `committed` and authenticate the block commitment",
        "commitment does not mean execution succeeded",
        "`eth_getTransactionReceipt`",
        "`eth_call` at an explicit committed height",
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
    assert (
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))"
    ) in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt
    proof = section("Receipt and proof verification")
    assert "authenticate the block header and CometBFT commit" in proof
    assert "trusted validator information" in proof
    assert (
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment"
    ) in proof


def test_readme_separated_network_diagrams_preserve_commit_order():
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert len(diagrams) == 2
    topology, sequence = diagrams
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in topology, fact
    steps = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    )
    positions = [sequence.index(step) for step in steps]
    assert positions == sorted(positions)


def test_readme_separated_network_served_and_retired_precompiles():
    native = section("EVM and Native Precompiles")
    rows = dict(re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native))
    table = source("precompiles/setup.go").split("var MergeAddressTable = []MergeAddress{", 1)[1]
    table = table.split("\n}", 1)[0]
    registered = {
        "0x" + address[-4:].upper(): name
        for address, name in re.findall(r'HexToAddress\("(0x[0-9A-Fa-f]+)"\), "([^"]+)"', table)
    }
    assert len(registered) == 13
    assert rows == registered
    assert {"0x101A", "0x101B", "0x101C", "0x101D", "0x101E"} <= rows.keys()
    assert not {"0x1012", "0x1013", "0x1014"} & rows.keys()
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "remaining historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "pcommon.RetiredAt(ctx, p.upgrades, pcommon.MergeUpgrade, ctx.BlockHeight())" in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    custody = source("precompiles/layerxcustody/layerxcustody.go")
    assert '"LXP_MODULE_DISABLED: module disabled; "' in custody
    assert "if p.keeper.Retired(ctx) {\n\t\treturn nil, errors.New(RetiredRevert)" in custody
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade, ctx.BlockHeight())" in anchor
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor


def test_readme_separated_network_no_standalone_deployment_guidance():
    assert "layerxd" not in re.findall(r"[a-z_]\w*", README.lower())
    for obsolete in ("local kernel cluster", "proposed draft", "make devnet-up"):
        assert obsolete not in README.lower(), obsolete
    assert "There is no separate kernel daemon, sequencer service or checkpoint deployment" in README
    assert "External checkpoint settlement is retired" in README
    assert "custody deposit and credit-attestation funding flow is retired" in README
    assert "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired" in README
    assert "No separate service endpoint is required" in README


def test_readme_separated_network_record_maps_all_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["1285"]
    assert task["audit"] == "22.30.65"
    assert task["verified_revision"]
    finding = record["finding"]["readme-separated-network"]
    assert finding["file"] == "README.md"
    assert finding["owner_domain"] == 17
    assert finding["wiring_requests"] == []
    proofs = {name for name in globals() if name.startswith("test_readme_separated_network_")}
    assert set(finding["tests"]) == proofs
    assert "not run" in task["baseline"]
    assert "runtime" in task["review_only"]
