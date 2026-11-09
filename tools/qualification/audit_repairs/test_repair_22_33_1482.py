"""Offline source proof for audit 22.30.86/doc-root-readme.

Read the actual README and its production producer/transport/consumer sources.
No native execution, network availability or restart qualification is claimed.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
RECORD_PATH = Path(__file__).with_name("repair_22_33_1482.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep fenced shell comments out of Markdown heading detection."""
    readme = source("README.md")
    marker = "#" * level + " " + heading + "\n"
    assert readme.count(marker) == 1, heading
    lines = []
    fence = None
    for line in readme.split(marker, 1)[1].splitlines(keepends=True):
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


def test_doc_root_readme_merged_network_and_tenants():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "Sub-chains are tenant domains stored in the database",
        "does not require new node infrastructure",
        "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", overview, re.S)
    assert len(diagrams) == 1
    for fact in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert fact in diagrams[0], fact


def test_doc_root_readme_root_rpc_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "https://api-mainnet-beta.paxeer.network",
        "`https://api1.mainnet-beta.paxeer.network` through",
        "`https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "gateway contract does not establish method availability at public node URLs",
    ):
        assert fact in quickstart, fact
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
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    for url in re.findall(r"https?://[^\s`<>)]+", source("README.md")):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.hostname in allowed_hosts, url
            assert parsed.scheme == "https" and parsed.path in ("", "/"), url
            assert not parsed.query and not parsed.fragment, url
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_doc_root_readme_transaction_to_committed_receipt():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "public node's root JSON-RPC URL", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`pax_sequencerReceipt`",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "authenticate the block commitment", "commitment does not mean execution succeeded",
        "`eth_getTransactionReceipt`", "`eth_call` at an explicit committed height",
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
    producer = source("node/sequencer/accepter.go")
    assert producer.index("a.rep.Release(context.Background(), &wal.Accept{") < producer.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    consumer = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in consumer
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', consumer) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in consumer
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in consumer
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    stages = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    )
    positions = [sequence[0].index(stage) for stage in stages]
    assert positions == sorted(positions)


def test_doc_root_readme_retired_executors_and_deployments():
    readme = source("README.md")
    for stale in (
        r"\blayerxd\b", r"\bcmd/layerxd\b", r"two (?:execution )?domains",
        r"kernel still runs separately", r"merge is (?:only )?a draft",
    ):
        assert not re.search(stale, readme, re.I), stale
    for fact in (
        "External checkpoint settlement is retired",
        "custody deposit and credit-attestation funding flow is retired",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads before store deletion are archive-only",
        "guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in readme, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    custody = source("precompiles/layerxcustody/layerxcustody.go")
    assert "if p.keeper.Retired(ctx)" in custody
    assert "return nil, errors.New(RetiredRevert)" in custody
    assert 'RetiredRevert        = "LXP_MODULE_DISABLED: module disabled; "' in custody
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_doc_root_readme_paxd_build_and_operator_commands():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "operator-provisioned, initialized node home",
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    commands = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    assert ["make", "paxd-build"] in [
        shlex.split(line, comments=True) for block in commands for line in block.splitlines()
    ]
    assert any(
        [shlex.split(line) for line in block.splitlines()] == [
            ["./build/paxd", "version"],
            ["./build/paxd", "start", "--home", "$PAXD_HOME"],
        ]
        for block in commands
    )
    makefile = source("Makefile")
    assert re.search(r"^paxd-native-libs: \$\(LIBRARY\) programs-release-build evmone-lib$", makefile, re.M)
    assert re.search(r"^paxd-build: paxd-native-libs$", makefile, re.M)
    recipe = makefile.split("\npaxd-build: paxd-native-libs\n", 1)[1].split("\n\n", 1)[0]
    assert "tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")


def test_doc_root_readme_integration_links_and_wiring_request():
    readme = source("README.md")
    links = set(re.findall(r"\]\(([^)]+)\)", readme))
    assert "docs/wiki/PaymentsQuickstart.md" in links
    assert "modules/layerxchains/" in links
    for path in ("docs/wiki/PaymentsQuickstart.md", "modules/layerxchains/"):
        assert (ROOT / path).exists(), path
    payments = source("docs/wiki/PaymentsQuickstart.md")
    assert "merged Paxeer network" in payments
    assert "successful payment is final when its block is" in payments
    tenant_guide = "docs/wiki/VirtualChains.md"
    assert "Creating one starts no process and requires no new infrastructure" in source(tenant_guide)
    record = tomllib.loads(RECORD_PATH.read_text(encoding="utf-8"))
    request = record["wiring"]["tenant_integration"]
    assert request["path"] == "README.md"
    assert request["owner_task"] == "22.31.470" and request["owner_domain"] == 17
    assert request["target"] == tenant_guide
    assert request["request"] == "README.md: domain 17 / task 22.31.470 must link docs/wiki/VirtualChains.md as the current tenant integration guide."
    assert tenant_guide not in links, "Reconcile the wiring record when its owner adds the link"


def test_doc_root_readme_record_binds_finding_proofs_and_evidence():
    record = tomllib.loads(RECORD_PATH.read_text(encoding="utf-8"))
    finding = record["finding"]["doc-root-readme"]
    assert finding["audit"] == "22.30.86" and finding["file"] == "README.md"
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    audit = source("spec/layerx-merge/programme/audit/records/22.30.86.kvx")
    block = audit.split("[finding.doc-root-readme]\n", 1)[1].split("\n[", 1)[0]
    original = tomllib.loads(block)
    assert original["file"] == finding["file"] and original["class"] == finding["class"]
    assert original["severity"] == finding["severity"]
    assert "one BFT chain 125 network whose paxd kernel proposes blocks" in original["expected"]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared and len(finding["tests"]) == len(declared) == 7
    task = record["task"]
    assert task["id"] == "22.33.1482"
    assert task["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1482.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["wiring_requests"] == ["tenant_integration"]
