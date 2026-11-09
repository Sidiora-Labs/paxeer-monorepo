"""Offline source contracts for audit 22.30.119/readme-separate-kernel."""

import ast
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading):
    match = re.search(r"^(#{2,3}) " + re.escape(heading) + r"\n", README, re.M)
    assert match is not None, heading
    lines = []
    fence = None
    for line in README[match.end():].splitlines(keepends=True):
        delimiter = re.match(r"^ {0,3}(`{3,}|~{3,})(.*)$", line)
        if delimiter:
            ticks, suffix = delimiter.groups()
            if fence is None:
                fence = ticks
            elif ticks[0] == fence[0] and len(ticks) >= len(fence) and not suffix.strip():
                fence = None
        elif fence is None and re.match(r"^#{1," + str(len(match[1])) + r"} ", line):
            break
        lines.append(line)
    return "".join(lines)


def test_readme_describes_one_merged_paxd_network():
    overview = section("What is Paxeer X Network?")
    for fact in (
        "one BFT network",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "C17 kernel in `src/` and `include/` runs inside `paxd`",
        "A receipt is final once its block is committed",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    topology = section("Integrated network topology")
    assert "subgraph Network[One Paxeer BFT network / EVM chain 125]" in topology
    assert "subgraph Node[paxd]" in topology
    assert "RPC --> Order --> Host --> Native" in topology
    assert "Validators -->|Commit| State" in topology
    assert "there is no router and no separate LayerX interface" in topology


def test_readme_uses_chain_125_and_root_numbered_rpc_hosts():
    assert "EVM chain ID **125** (`0x7d`)" in README
    quickstart = section("Quick start: connect to chain 125")
    first = "https://api1.mainnet-beta.paxeer.network"
    last = "https://api14.mainnet-beta.paxeer.network"
    assert f"`{first}` through `{last}`" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert '"method":"eth_chainId","params":[]' in quickstart
    assert '# Expected result: "0x7d"' in quickstart
    assert "`node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    node_urls = [urlsplit(url) for url in urls if "mainnet-beta.paxeer.network" in url]
    assert node_urls
    allowed = {"api-mainnet-beta.paxeer.network"}
    allowed.update(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15))
    for url in node_urls:
        assert url.scheme == "https" and url.hostname in allowed
        assert url.path in ("", "/") and not url.query and not url.fragment
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_transaction_receipt_flow_matches_production_sources():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`",
        "`eth_sendRawTransaction`",
        "transaction hash",
        "durable WAL acknowledgement",
        "`pax_sequencerReceipt`",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block",
        "`MsgSequenceBatch`",
        "authenticate the block commitment before treating the result as final",
        "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    accepter = source("node/sequencer/accepter.go")
    assert accepter.index("a.rep.Release(context.Background(), &wal.Accept{") < accepter.index(
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
    assert "Neither a transaction hash nor a signed execution receipt alone establishes block commitment" in proof


def test_readme_retires_custody_and_checkpoint_executors():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "historical views retire at `v7.0-anchor-store-delete`",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_readme_build_and_start_use_supported_paxd_targets():
    build = section("Build")
    for fact in (
        "make paxd-build", "make paxeer-build", "make -f chain.mk build",
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "make build",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    commands = [
        shlex.split(line, comments=True)
        for block in re.findall(r"```sh\n(.*?)\n```", build, re.S)
        for line in block.splitlines() if line.strip()
    ]
    assert ["make", "paxd-build"] in commands
    assert ["./build/paxd", "version"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "\npaxeer-build:\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release


def test_readme_has_no_separate_daemon_or_checkpoint_instructions():
    assert "There is no separate kernel daemon, sequencer service or checkpoint deployment" in README
    assert "External checkpoint settlement is retired" in README
    for pattern in (
        r"\blayerxd\b",
        r"two (?:execution )?domains",
        r"kernel (?:still )?runs separately",
        r"merge (?:remains|is) (?:only )?(?:a )?draft",
        r"\b(?:make|start|run|deploy)\s+(?:the\s+)?checkpoint(?:er)?\b",
    ):
        assert not re.search(pattern, README, re.I), pattern


def test_repair_record_maps_the_audit_finding_to_executed_source_checks():
    record = tomllib.loads(source("tools/qualification/audit_repairs/repair_22_33_187.kvx"))
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.119.kvx"))
    finding = record["finding"]["readme-separate-kernel"]
    original = audit["finding"]["readme-separate-kernel"]
    assert set(record["finding"]) == {"readme-separate-kernel"}
    for key in ("file", "class", "severity", "expected"):
        assert finding[key] == original[key]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_repair_record_maps_the_audit_finding_to_executed_source_checks"
    }
    task = record["task"]["22"]["33"]["187"]
    assert task["audit"] == "22.30.119" and task["requirement"] == "22998.1"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_187.py"
    assert task["build_cmd"] == "python3 --version"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "candidate revision" in task["verified_revision"]
    assert "not executed" in task["review_only"]
    assert "No baseline test ran" in task["baseline"]
    assert task["wiring_requests"] == []
    upstream = tomllib.loads(source("tools/qualification/audit_repairs/repair_22_31_470.kvx"))
    coverage = upstream["coverage"]["audit_22_30_119"]
    assert coverage["finding"] == "readme-separate-kernel"
    assert coverage["file"] == "README.md" and "22.33.187" in coverage["consumer_tasks"]
