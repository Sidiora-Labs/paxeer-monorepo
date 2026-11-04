"""Source proof for audit 22.30.62/readme-retired-architecture.

Decision 2525 assigns README edits to 22.31.470. Read that production document
and its actual source connections offline; do not simulate native execution.
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
RECORD = Path(__file__).with_name("repair_22_33_1270.kvx")


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


def test_readme_one_paxd_network_embeds_kernel_and_commits_blocks():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
    ):
        assert fact in overview, fact
    topology = section("Integrated network topology")
    for edge in (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]",
        "Embedded C17 kernel and Rust programs host",
        "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ):
        assert edge in topology, edge
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_readme_chain_125_probe_and_numbered_hosts_use_root_json_rpc():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert (
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`"
    ) in quickstart
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    endpoints = [
        urlsplit(url) for url in urls
        if urlsplit(url).hostname.endswith("mainnet-beta.paxeer.network")
    ]
    assert endpoints
    allowed_hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    for endpoint in endpoints:
        assert endpoint.scheme == "https"
        assert endpoint.hostname in allowed_hosts
        assert endpoint.path in ("", "/")
        assert not endpoint.query and not endpoint.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert argv[:4] == ["curl", "--max-time", "10", "-sS"]
    assert argv[4] == "https://api1.mainnet-beta.paxeer.network"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert int("0x7d", 16) == 125
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_transaction_transport_and_committed_receipt_consumer():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`",
        "`eth_sendRawTransaction` to a public node's root JSON-RPC URL",
        "result is a transaction hash",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
        "`pax_sequencerReceipt` with the transaction hash",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block whose `MsgSequenceBatch`",
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
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt
    sequence = re.search(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert sequence is not None
    steps = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
    )
    positions = [sequence.group(1).index(step) for step in steps]
    assert positions == sorted(positions)


def test_readme_build_and_launch_use_paxd_with_linked_native_libraries():
    build = section("Build")
    commands = [
        shlex.split(line, comments=True)
        for block in re.findall(r"```sh\n(.*?)\n```", build, re.S)
        for line in block.splitlines() if line.strip()
    ]
    assert ["make", "paxd-build"] in commands
    assert ["./build/paxd", "version"] in commands
    assert ["./build/paxd", "start", "--home", "$PAXD_HOME"] in commands
    assert "operator-provided genesis and consensus chain identifier" in build
    assert "EVM chain ID `125` is the signing domain" in build
    assert "running network entry point is `paxd`" in build
    for library in (
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a"
    ):
        assert library in build
    makefile = source("Makefile")
    targets = dict(re.findall(r"^([\w-]+):([^\n]*)$", makefile, re.M))
    assert targets["paxd-build"].split() == ["paxd-native-libs"]
    assert targets["paxd-native-libs"].split() == ["$(LIBRARY)", "programs-release-build"]
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")


def test_readme_retirement_lifecycle_keeps_external_native_bridge_distinct():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)",
        "interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "external native bridge at `0x1016` remains distinct from retired kernel custody",
        "outward transfers use its native `bridgeOut` interface",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
    ):
        assert fact in native, fact
    active = dict(re.findall(r"\| `(0x[0-9A-Fa-f]+)` \| `([^`]+)` \|", native))
    assert {"0x1012", "0x1013", "0x1014"}.isdisjoint(active)
    assert {address: active[address] for address in ("0x101A", "0x101B", "0x101C", "0x101D")} == {
        "0x101A": "layerxledger", "0x101B": "layerxpay",
        "0x101C": "layerxauth", "0x101D": "layerxprograms",
    }
    verify = source("precompiles/layerxverify/layerxverify.go")
    assert "pcommon.RetiredAt(ctx, p.upgrades, pcommon.MergeUpgrade" in verify
    assert 'pcommon.RetiredRevert("layerxVerify")' in verify
    custody = source("precompiles/layerxcustody/layerxcustody.go")
    assert "if p.keeper.Retired(ctx)" in custody
    assert 'RetiredRevert        = "LXP_MODULE_DISABLED: module disabled; "' in custody
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    bridge = json.loads(source("precompiles/layerxbridge/abi.json"))
    outward = [item for item in bridge if item.get("name") == "bridgeOut"]
    assert len(outward) == 1
    assert outward[0]["type"] == "function"
    assert outward[0]["stateMutability"] == "nonpayable"
    assert [item["type"] for item in outward[0]["inputs"]] == [
        "uint64", "address", "uint256", "address"
    ]
    assert "result, err := p.keeper.BridgeOut(ctx, caller, chainID" in source(
        "precompiles/layerxbridge/layerxbridge.go"
    )


def test_readme_has_no_separate_daemon_build_or_checkpoint_funding_guidance():
    assert not re.search(r"\blayerxd\b", README, re.I)
    for stale in (r"two (?:execution )?domains", r"kernel still runs separately", r"\bLNI\b"):
        assert not re.search(stale, README, re.I), stale
    accounts = section("Accounts and native funding")
    assert "custody deposit and credit-attestation funding flow is retired" in accounts
    assert "Native sub-accounts map to derived bank addresses inside the node" in accounts
    assert "Wait for commit and read `balanceOf` to confirm funding" in accounts
    paragraphs = [p for p in README.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(paragraphs) == 1
    assert "retired refusal surfaces" in paragraphs[0]
    current = README.replace(paragraphs[0], "")
    assert not re.search(r"layerx(?:Verify|Custody|Anchor)", current, re.I)


def test_repair_record_maps_the_audit_finding_to_every_source_proof():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["readme-retired-architecture"]
    assert finding["audit"] == "spec/layerx-merge/programme/audit/records/22.30.62.kvx"
    audit = tomllib.loads(source(finding["audit"]))["finding"]["readme-retired-architecture"]
    assert audit["file"] == finding["file"] == "README.md"
    assert audit["class"] == "doc-drift" and audit["severity"] == "medium"
    assert audit["owner_domain"] == finding["owner_domain"] == 1
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_repair_record_maps_the_audit_finding_to_every_source_proof"
    }
    task = record["task"]["22"]["33"]["1270"]
    assert task["readme_owner_task"] == "22.31.470"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1270.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert len(task["wiring_requests"]) == 1
    assert task["wiring_requests"][0].startswith("README.md | domain 1 | task 22.31.470 |")
    assert record["wiring"]["readme"]["status"] == "satisfied-by-prerequisite"
    assert record["wiring"]["readme"]["missing_facts"] == []
