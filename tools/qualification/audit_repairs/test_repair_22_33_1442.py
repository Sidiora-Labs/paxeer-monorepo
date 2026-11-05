"""Offline source qualification of audit 22.30.81's doc-readme finding."""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_1442.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Extract a Markdown section without treating shell comments as headings."""
    marker = "#" * level + " " + heading + "\n"
    assert README.count(marker) == 1, heading
    lines = []
    fenced = False
    for line in README.split(marker, 1)[1].splitlines(keepends=True):
        if line.startswith("```"):
            fenced = not fenced
        elif not fenced and re.match(r"^#{1," + str(level) + r"} ", line):
            break
        lines.append(line)
    return "".join(lines)


def assert_facts(text, facts):
    for fact in facts:
        assert fact in text, fact


def make_rule(path, target):
    text = source(path)
    matches = list(re.finditer(r"^" + re.escape(target) + r":([^\n]*)\n", text, re.M))
    assert len(matches) == 1, (path, target)
    match = matches[0]
    recipe = []
    for line in text[match.end():].splitlines():
        if not line.startswith("\t"):
            break
        recipe.append(line[1:])
    return match[1].split(), "\n".join(recipe)


def test_doc_readme_one_paxd_chain_with_linked_kernel():
    assert_facts(section("What is Paxeer X Network?", 2), (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "no router and no separate LayerX interface",
    ))
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert "-llayerx -llayerx_programs_sandbox" in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    for obsolete in (r"\blayerxd\b", r"\blx_[\w*]+", r"merge is (?:only )?a draft"):
        assert not re.search(obsolete, README, re.I), obsolete


def test_doc_readme_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "root path of these hosts, without a route suffix" in quickstart
    bounds = re.search(
        r"numbered nodes `https://api(\d+)\.mainnet-beta\.paxeer\.network` "
        r"through `https://api(\d+)\.mainnet-beta\.paxeer\.network`", quickstart,
    )
    assert bounds is not None
    assert tuple(map(int, bounds.groups())) == (1, 14)
    hosts = {f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)}
    hosts.add("api-mainnet-beta.paxeer.network")
    public_urls = [
        urlsplit(url) for url in re.findall(r"https?://[^\s`<>)]+", README)
        if urlsplit(url).hostname in hosts
    ]
    assert {url.hostname for url in public_urls} == {
        "api-mainnet-beta.paxeer.network", "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    for url in public_urls:
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
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert '# Expected result: "0x7d"' in blocks[0]


def test_doc_readme_transaction_to_committed_receipt_source_path():
    flow = section("Submit a transaction and follow its committed receipt")
    assert_facts(flow, (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "authenticate the block commitment", "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height", "No separate service endpoint is required",
    ))
    assert_facts(source("rpc/send.go"), (
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ))
    release = source("node/sequencer/accepter.go")
    positions = [release.index(fact) for fact in (
        "a.rep.Release(context.Background(), &wal.Accept{",
        "if err == nil && ack.Status != AckOK",
        "a.src.Record(common.Hash(acc.ActivityID), send)",
        "return evmrpc.AcceptedSend{Receipt: receipt, Height: acc.Height, Exec: exec}, nil",
    )]
    assert positions == sorted(positions)
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed",
    ]
    positions = [receipt.index(fact) for fact in (
        "if err := result.Verify(keys); err != nil",
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))",
        "block, err := a.tmClient.Block(ctx, &height)",
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted",
    )]
    assert positions == sorted(positions)
    assert "a.option.Keeper.LastSeq(a.ctxProvider(height))" in receipt
    assert "if committedHash != seqHash" in receipt
    assert_facts(section("Receipt and proof verification"), (
        "authenticate the block header and CometBFT commit against trusted validator information",
        "verify transaction or state inclusion against the authenticated root",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ))


def test_doc_readme_hosted_gateway_contract_and_native_px_subset():
    assert_facts(section("Quick start: connect to chain 125"), (
        "hosted gateway's `px_` API is defined by",
        "services/platform/hosted/gateway/openrpc.json",
        "Obtain its deployment URL and credentials from the operator",
        "gateway contract does not establish method availability at public node URLs",
        "`px_getReceiptKeys` is registered by the node",
        "native `px_` account/history and receipt-key methods",
    ))
    contract = json.loads(source("services/platform/hosted/gateway/openrpc.json"))
    assert contract["openrpc"] == "1.3.2"
    assert {"px_getBalances", "px_getReceiptKeys", "pax_sequencerReceipt"} <= {
        method["name"] for method in contract["methods"]
    }
    assert_facts(source("rpc/server.go"), (
        '{Namespace: "px", Service: pxAPI}',
        '{Namespace: "px", Service: &pxHistoryAPI{account: pxAPI}}',
        "Service:   &receiptKeysAPI{receipts: sequencerReceiptAPI}",
    ))


def test_doc_readme_retired_custody_and_checkpoint_evidence():
    native = section("EVM and Native Precompiles")
    assert_facts(native, (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "support historical interpretation",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ))
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    assert_facts(source("precompiles/layerxanchor/layerxanchor.go"), (
        "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade",
        "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade",
    ))


def test_doc_readme_linked_node_build_start_and_upgrade():
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    assert len(blocks) == 4
    assert shlex.split(blocks[0], comments=True) == ["make", "paxd-build"]
    assert [shlex.split(line) for line in blocks[1].splitlines()] == [
        ["./build/paxd", "version"], ["./build/paxd", "start", "--home", "$PAXD_HOME"],
    ]
    assert_facts(build, (
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ))
    dependencies, recipe = make_rule("Makefile", "paxd-build")
    assert dependencies == ["paxd-native-libs"]
    assert "jq -er .version tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert make_rule("Makefile", "paxd-native-libs")[0] == ["$(LIBRARY)", "programs-release-build"]
    assert make_rule("chain.mk", "build") == (
        ["paxd-native-libs"], "go build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd",
    )
    assert_facts(source("Makefile"), (
        "LIBRARY := $(BUILD_DIR)/liblayerx.a",
        "PROGRAMS_RELEASE_RUNTIME_LIB := programs/target/release/liblayerx_programs_sandbox.a",
    ))
    assert_facts(section("Releases and Network Upgrades"), (
        "merged runtime executes kernel activity inside `paxd`",
        "commits it with the BFT chain state",
        "`v7.0` merge handler activates the merged interfaces",
        "retires the old custody/checkpoint write paths",
        "`v7.0-anchor-store-delete` removes the historical anchor store",
    ))
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")


def test_doc_readme_record_maps_audit_finding_to_all_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["doc-readme"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.81.kvx"))
    original = audit["finding"]["doc-readme"]
    assert finding["audit"] == "22.30.81"
    for key in ("file", "line", "class", "severity", "observed", "expected", "proposal"):
        assert finding[key] == original[key], key
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert len(finding["tests"]) == len(declared) == 7
    task = record["task"]["22"]["33"]["1442"]
    assert task["readme_owner_task"] == "22.31.470"
    assert task["missing_readme_facts"] == task["wiring_requests"] == []
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1442.py"
    )
    assert task["inspected_revision"] == "38471f735e624a7c9ac2f28e4aa811685d7c8ca2"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
