"""Qualify docs-root-architecture against the README and production source facts.

This offline gate does not execute the native node or certify deployment behavior.
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
RECORD = Path(__file__).with_name("repair_22_33_429.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep fenced examples intact, including shell comments starting with #."""
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


def make_rule(text, target):
    """Read one explicit Make rule and its tab-indented recipe without running it."""
    rules = re.findall(
        r"^" + re.escape(target) + r":([^\n]*)\n((?:\t[^\n]*\n)*)",
        text,
        re.M,
    )
    assert len(rules) == 1, target
    dependencies, recipe = rules[0]
    return dependencies.split(), recipe


def test_docs_root_architecture_embeds_kernel_in_chain_125():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network",
        "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "C17 kernel",
        "modules/lxphost",
        "node/sequencer/",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    host = source("modules/lxphost/host.go")
    assert 'import "C"' in host
    assert "-llayerx -llayerx_programs_sandbox" in host
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_docs_root_architecture_uses_root_node_rpc_hosts():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
        "gateway contract does not establish method availability at public node URLs",
    ):
        assert fact in quickstart, fact
    assert "no router and no separate LayerX interface" in README
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:", 1)[0].replace("\\\n", " "))
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
    hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = [
        urlsplit(url) for url in re.findall(r"https?://[^\s`<>)]+", README)
        if urlsplit(url).hostname in hosts
    ]
    assert {url.hostname for url in urls} == {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    assert all(url.scheme == "https" and url.path in ("", "/") for url in urls)
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_docs_root_architecture_submission_follows_node_wal_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`eth_getTransactionCount`",
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_sendRawTransaction`",
        "transaction hash",
        "`0x101A`–`0x101D`",
        "Programs payload bytes go to `0x101D`",
        "accepts on the node's integrated sequencing path or forwards",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    release = source("node/sequencer/accepter.go")
    start = release.index("ack, err := a.rep.Release(context.Background(), &wal.Accept{")
    publish = release.index("a.src.Record(common.Hash(acc.ActivityID), send)", start)
    acknowledgement = release[start:publish]
    assert "ack.Status != AckOK" in acknowledgement
    assert "if err != nil {" in acknowledgement
    assert "ErrReplicationHalted" in acknowledgement
    assert "return evmrpc.AcceptedSend{}" in acknowledgement


def test_docs_root_architecture_distinguishes_receipt_from_commit():
    assert "A receipt is final once its block is committed" in README
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "`pax_sequencerReceipt`",
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
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed"
    ]
    coverage = rpc.index(
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))"
    )
    commit = rpc.index("result.Rung, result.BlockNumber, result.BlockHash = RungCommitted")
    assert coverage < commit
    assert "if err != nil || !committed {\n\t\treturn result, err" in rpc[coverage:commit]
    assert "a.tmClient.Block(ctx, &height)" in rpc[coverage:commit]
    proof = section("Receipt and proof verification")
    for fact in (
        "ProtocolReceipt v2",
        "pinned for the receipt height",
        "public key supplied beside an untrusted receipt does not establish trust",
        "authenticate the block header and CometBFT commit",
        "trusted validator information",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ):
        assert fact in proof, fact
    assert "ed25519.Sign(k.private, digest[:])" in source("node/sequencer/receiptkey.go")
    assert "codec.DecodeReceipt(receiptBytes)" in source("layerxproof/verify/receipt.go")
    assert "self.verify_header_commit(header, commit)?" in source(
        "programs/crates/layerx-programs-registry/src/chain_proof/head_verify.rs"
    )


def test_docs_root_architecture_native_inventory_and_retired_refusals():
    native = section("EVM and Native Precompiles")
    rows = {
        int(address, 16): name
        for address, name in re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native)
    }
    table = re.search(
        r"var MergeAddressTable = \[\]MergeAddress\{(.*?)^\}",
        source("precompiles/setup.go"), re.M | re.S,
    )
    assert table is not None
    production = {
        int(address, 16): name
        for address, name in re.findall(
            r'HexToAddress\("(0x[0-9A-Fa-f]+)"\), "([^"]+)"', table.group(1)
        )
    }
    assert len(production) == 13
    assert rows == production
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
    ):
        assert fact in native, fact
    verify = source("precompiles/layerxverify/layerxverify.go")
    assert "if pcommon.RetiredAt(ctx, p.upgrades, pcommon.MergeUpgrade, ctx.BlockHeight())" in verify
    assert 'return pcommon.RetiredRevert("layerxVerify"), vm.ErrExecutionReverted' in verify
    custody = source("precompiles/layerxcustody/layerxcustody.go")
    assert "if p.keeper.Retired(ctx) {\n\t\treturn nil, errors.New(RetiredRevert)" in custody
    assert '"LXP_MODULE_DISABLED: module disabled; " + PrecompileName + " retired"' in custody
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    for guard in (
        "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade, ctx.BlockHeight())",
        "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade, ctx.BlockHeight())",
    ):
        assert "if " + guard + " {\n\t\treturn pcommon.RetiredRevert(PrecompileName), vm.ErrExecutionReverted" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")


def test_docs_root_architecture_build_and_start_match_production():
    build = section("Build")
    for fact in (
        "make paxd-build",
        "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json",
        "make paxeer-build",
        "make -f chain.mk build",
        "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "Building the binary alone does not provision network membership",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert make_rule(makefile, "paxd-native-libs")[0] == ["$(LIBRARY)", "programs-release-build"]
    dependencies, recipe = make_rule(makefile, "paxd-build")
    assert dependencies == ["paxd-native-libs"]
    assert "jq -er .version tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert re.search(r"^LIBRARY := \$\(BUILD_DIR\)/liblayerx.a$", makefile, re.M)
    chain = source("chain.mk")
    assert make_rule(chain, "build")[0] == ["paxd-native-libs"]
    assert "-o ./build/paxd ./daemon/paxd" in make_rule(chain, "build")[1]
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    assert isinstance(json.loads(source("tools/release/version.json"))["version"], str)
    for target in re.findall(r"\]\(([^)]+)\)", README):
        if not target.startswith(("https:", "http:", "#")):
            assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_docs_root_architecture_has_no_separate_deployment():
    assert "There is no separate kernel daemon, sequencer service or checkpoint deployment" in README
    assert "External checkpoint settlement is retired" in README
    assert "Source directory names describe code organization, not separate network deployments" in README
    for stale in (
        r"\blayerxd\b", r"\blx_[A-Za-z0-9_*]*", r"/rpc(?:/|\b)",
        r"two (?:execution )?domains", r"kernel still runs separately",
    ):
        assert not re.search(stale, README, re.I), stale
    topology, sequence = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert "subgraph Network[One Paxeer BFT network / EVM chain 125]" in topology
    assert "subgraph Node[paxd]" in topology
    assert "RPC --> Order --> Host --> Native" in topology
    assert "Validators -->|Commit| State" in topology
    ordered = [
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
    ]
    positions = [sequence.index(fact) for fact in ordered]
    assert positions == sorted(positions)


def test_docs_root_architecture_record_maps_finding_to_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["429"]
    finding = record["finding"]["docs-root-architecture"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.146.kvx"))
    original = audit["finding"]["docs-root-architecture"]
    assert original["file"] == finding["file"] == "README.md"
    assert original["class"] == finding["class"] == "doc-drift"
    assert original["severity"] == finding["severity"] == "medium"
    assert finding["audit"] == "22.30.146"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_429.py"
    assert task["build_cmd"] == "python3 --version"
    assert task["input_revision"] == "f0258720ce13493b46a14760693f7343fdcd5aaa"
    assert task["readme_revision"] == "38801bb427090ac96d47c86a026a63ec05e67315"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert task["missing_wiring_requests"] == []
    wiring = record["wiring"]["root_readme"]
    assert wiring["path"] == "README.md"
    assert wiring["owner_domain"] == 17 and wiring["owner_task"] == "22.31.470"
    assert wiring["status"] == "satisfied_by_owner"
