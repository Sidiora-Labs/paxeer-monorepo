"""Qualify readme-architecture-drift against the maintained production sources.

These are offline documentation/source checks, not native execution, deployment,
cryptographic verification or restart/recovery certification.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
RECORD = Path(__file__).with_name("repair_22_33_1296.kvx")
README = (ROOT / "README.md").read_text(encoding="utf-8")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Respect code fences, including the chain probe's '# Expected' comment."""
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


def make_rule(path, target):
    """Parse a literal target's prerequisites and recipe without invoking make."""
    text = source(path)
    matches = list(re.finditer(r"^" + re.escape(target) + r":([^\n]*)\n", text, re.M))
    assert len(matches) == 1, (path, target)
    match = matches[0]
    recipe = []
    for line in text[match.end():].splitlines():
        if not line.startswith("\t"):
            break
        recipe.append(line[1:])
    return match.group(1).split(), "\n".join(recipe)


def test_single_paxd_network_embeds_kernel_and_commits_receipts():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
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
    upgrades = section("Releases and Network Upgrades")
    assert "merged runtime executes kernel activity inside `paxd`" in upgrades
    assert "commits it with the BFT chain state" in upgrades
    for stale in (
        r"\blayerxd\b", r"\bmake\s+checkpoint\b", r"\bLNI\b",
        r"merge is (?:only )?a draft", r"kernel still runs separately",
        r"two (?:execution )?domains", r"\blx_[A-Za-z0-9_*]*",
    ):
        assert not re.search(stale, README, re.I), stale


def test_public_rpc_range_and_chain_probe_use_https_root_urls():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    match = re.search(
        r"numbered nodes `https://api(\d+)\.mainnet-beta\.paxeer\.network` "
        r"through `https://api(\d+)\.mainnet-beta\.paxeer\.network`", quickstart,
    )
    assert match is not None
    assert tuple(map(int, match.groups())) == (1, 14)
    hosts = {f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)}
    hosts.add("api-mainnet-beta.paxeer.network")
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    public_urls = [urlsplit(url) for url in urls if urlsplit(url).hostname in hosts]
    assert public_urls
    for url in public_urls:
        assert url.scheme == "https"
        assert url.path in ("", "/") and not url.query and not url.fragment
    assert not re.search(r"https?://api[^\s`]*paxeer\.network/(?:rpc|evm)\b", README)
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
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_transaction_receipt_path_matches_acceptance_wal_and_commit_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height", "No separate service endpoint is required",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    release = source("node/sequencer/accepter.go")
    positions = [release.index(item) for item in (
        "a.rep.Release(context.Background(), &wal.Accept{",
        "if err == nil && ack.Status != AckOK",
        "a.src.Record(common.Hash(acc.ActivityID), send)",
        "return evmrpc.AcceptedSend{Receipt: receipt, Height: acc.Height, Exec: exec}, nil",
    )]
    assert positions == sorted(positions)
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed",
    ]
    positions = [rpc.index(item) for item in (
        "if err := result.Verify(keys); err != nil",
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))",
        "block, err := a.tmClient.Block(ctx, &height)",
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted",
    )]
    assert positions == sorted(positions)
    assert "a.option.Keeper.LastSeq(a.ctxProvider(height))" in rpc
    assert "if committedHash != seqHash" in rpc


def test_architecture_and_receipt_diagrams_preserve_the_same_network_flow():
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert len(diagrams) == 2
    topology, receipt = diagrams
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native", "Proposed block in execution order",
        "Validators -->|Commit| State",
    ):
        assert fact in topology, fact
    positions = [receipt.index(fact) for fact in (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    )]
    assert positions == sorted(positions)


def test_live_features_match_registry_and_retired_executors_are_historical():
    features = section("EVM and Native Precompiles")
    rows = re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", features)
    table = re.search(
        r"var MergeAddressTable = \[\]MergeAddress\{(.*?)\n\}",
        source("precompiles/setup.go"), re.S,
    )
    assert table is not None
    entries = re.findall(r'HexToAddress\("(0x[0-9A-Fa-f]+)"\), "([^"]+)"', table[1])
    assert len(rows) == len(entries) == 13
    assert {(int(address, 16), name) for address, name in rows} == {
        (int(address, 16), name) for address, name in entries
    }
    for fact in (
        "`0x1012`", "`0x1013`", "`0x1014`", "retired refusal surfaces",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "rejects writes after the merge", "historical views retire at `v7.0-anchor-store-delete`",
        "Historical anchor reads before store deletion are archive-only",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in features, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")


def test_operator_commands_build_linked_paxd_and_start_that_binary():
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    assert len(blocks) == 4
    assert shlex.split(blocks[0], comments=True) == ["make", "paxd-build"]
    assert [shlex.split(line) for line in blocks[1].splitlines()] == [
        ["./build/paxd", "version"], ["./build/paxd", "start", "--home", "$PAXD_HOME"],
    ]
    for fact in (
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    deps, recipe = make_rule("Makefile", "paxd-build")
    assert deps == ["paxd-native-libs"]
    assert "jq -er .version tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert make_rule("Makefile", "paxd-native-libs")[0] == ["$(LIBRARY)", "programs-release-build"]
    makefile = source("Makefile")
    assert "LIBRARY := $(BUILD_DIR)/liblayerx.a" in makefile
    assert "PAXEER_MAKE := $(MAKE) -f $(PAXEER_DIR)/chain.mk" in makefile
    assert "PROGRAMS_RELEASE_RUNTIME_LIB := programs/target/release/liblayerx_programs_sandbox.a" in makefile
    assert make_rule("chain.mk", "build") == (
        ["paxd-native-libs"], "go build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd",
    )
    root_command = source("daemon/paxd/cmd/root.go")
    assert 'Use:           "paxd"' in root_command
    assert "server.AddCommands(" in root_command


def test_maintained_architecture_source_and_cited_paths_are_current():
    spec = section("Specification", 2)
    assert "This README is the maintained architecture entry point" in spec
    assert "translated and archived pages must be read against this network model" in spec
    assert "Source directory names describe code organization, not separate network deployments" in README
    for target in re.findall(r"\]\(([^)]+)\)", README):
        if not target.startswith(("https:", "http:", "#")):
            assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_repair_record_maps_the_finding_to_all_proofs_and_private_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["readme-architecture-drift"]
    assert finding["audit"] == "22.30.68"
    assert finding["file"] == "README.md"
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.68.kvx"))
    original = audit["finding"]["readme-architecture-drift"]
    assert finding["expected"] == original["expected"]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert len(finding["tests"]) == len(declared) == 8
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["1296"]
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1296.py"
    assert task["inspected_revision"] == "f6003d1b92ad50295bc110e36646d6f4b7adb3dc"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert task["missing_readme_facts"] == []
    assert task["readme_owner_task"] == "22.31.470"
    assert len(task["wiring_requests"]) == 1
    translation_request = task["wiring_requests"][0]
    assert "domain 5" in translation_request and "README.md" in translation_request
    for language in ("pt-BR", "fr", "ja", "ru", "zh-CN", "de"):
        assert f"docs/readme/README.{language}.md" in translation_request
