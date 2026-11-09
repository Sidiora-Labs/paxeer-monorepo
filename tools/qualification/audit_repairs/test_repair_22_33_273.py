"""Offline source contracts for audit 22.30.131/root-network-description.

Read the production README and its referenced implementations. These checks
qualify documentation facts, not native execution or public endpoint health.
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
RECORD = Path(__file__).with_name("repair_22_33_273.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(title, level=3):
    """Keep fenced shell comments inside their Markdown section."""
    marker = "#" * level + " " + title + "\n"
    assert README.count(marker) == 1, title
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


def assert_facts(text, facts):
    for fact in facts:
        assert fact in text, fact


def test_root_network_description_integrates_kernel_and_bft_finality():
    overview = section("What is Paxeer X Network?", 2)
    assert_facts(overview, (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface", "outside consensus",
    ))
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    assert "Native[Native bank, ledger, payments, authority and programs]" in overview
    assert "Validators -->|Commit| State[Committed block and chain state]" in overview


def test_root_network_description_public_rpc_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert_facts(quickstart, (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "Confirm the chain ID before signing or broadcasting",
        "compare `eth_blockNumber`",
    ))
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = [
        urlsplit(url)
        for url in re.findall(r"https?://[^\s`<>)]+", README)
        if urlsplit(url).hostname in allowed_hosts
    ]
    assert {url.hostname for url in urls} == {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api2.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    for url in urls:
        assert url.scheme == "https"
        assert url.path in ("", "/") and not url.query and not url.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert '"hyperpax_125-1": int64(125)' in source("modules/evm/config/config.go")
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_root_network_description_transaction_and_committed_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    assert_facts(flow, (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`",
        "public node's root JSON-RPC URL", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`pax_sequencerReceipt`", "`submitted`", "`receipted`", "`committed`",
        "`MsgSequenceBatch`", "No separate service endpoint is required",
        "authenticate the block commitment before treating the result as final",
        "`eth_getTransactionReceipt`", "inspect its status and logs",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ))
    send = source("rpc/send.go")
    assert_facts(send, (
        "func (s *SendAPI) SendRawTransaction(",
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ))
    receipts = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipts
    for rung in ("Submitted", "Receipted", "Committed"):
        assert re.search(r"Rung" + rung + r'\s+Rung = "' + rung.lower() + '"', receipts)
    assert_facts(receipts, (
        "a.option.Source.SequencerSend(txHash)", "if send.Receipt == nil",
        "result.Verify(keys)",
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))",
        "if err != nil || !committed", "a.tmClient.Block(ctx, &height)",
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted",
    ))
    proof = section("Receipt and proof verification")
    assert_facts(proof, (
        "authenticate the block header and CometBFT commit",
        "trusted validator information", "authenticated root at the relevant height and key",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
    ))


def test_root_network_description_build_and_operations_use_paxd():
    build = section("Build")
    assert_facts(build, (
        "Build the merged node from the repository root", "make paxd-build",
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "EVM chain ID `125` is the signing domain",
        "integrated proposer and standby", "running network entry point is `paxd`",
    ))
    targets = dict(re.findall(r"^([\w-]+):([^\n]*)$", source("Makefile"), re.M))
    assert targets["paxd-build"].split() == ["paxd-native-libs"]
    assert targets["paxd-native-libs"].split() == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    releases = section("Releases and Network Upgrades")
    assert_facts(releases, (
        "executes kernel activity inside `paxd`", "commits it with the BFT chain state",
        "retires the old custody/checkpoint write paths",
    ))


def test_root_network_description_retires_separate_services_and_executors():
    for stale in (
        r"\blayerxd\b", r"two (?:execution )?domains",
        r"kernel still runs separately", r"\blx_[A-Za-z0-9_*]*",
        r"https?://api\d*\.mainnet-beta\.paxeer\.network/[^\s`<>)]+",
    ):
        assert not re.search(stale, README, re.I), stale
    native = section("EVM and Native Precompiles")
    assert_facts(native, (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge", "`layerxAnchor` (`0x1014`) rejects writes",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ))
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_repair_record_maps_finding_to_declared_proofs_and_owner():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["273"]
    finding = record["finding"]["root-network-description"]
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_273.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert finding["audit"] == "22.30.131"
    assert finding["file"] == "README.md"
    assert finding["owner_task"] == "22.31.470"
    assert finding["owner_domain"] == 17
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_root_network_description_")
    }
    assert set(finding["tests"]) == declared
    assert len(declared) == 5
    audit = source("spec/layerx-merge/programme/audit/records/22.30.131.kvx")
    block = re.search(r"(?ms)^\[finding\.root-network-description\]\n(.*?)(?=^\[|\Z)", audit)
    assert block is not None
    original = tomllib.loads(block.group(0))["finding"]["root-network-description"]
    assert original["file"] == finding["file"]
    assert original["class"] == "doc-drift" and original["severity"] == "medium"
    assert original["owner_domain"] == finding["owner_domain"]
    assert record["wiring"]["root_readme"]["request"] == (
        "README.md -> domain 17 / task 22.31.470: maintain the integrated paxd network, "
        "chain-125 root RPC, committed receipt flow and retired deployment guidance."
    )
    assert record["wiring"]["root_readme"]["missing_facts"] == []
