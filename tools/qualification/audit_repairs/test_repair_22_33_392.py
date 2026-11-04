"""Offline source contracts for audit 22.30.141's standalone-kernel drift.

Read the production README and its build/RPC sources without executing examples,
building native code, or contacting public nodes.
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


def section(title, level=3):
    """Read a section while preserving heading-like comments in fenced code."""
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


def make_rule(text, target):
    match = re.search(
        r"^" + re.escape(target) + r":([^\n]*)\n((?:\t[^\n]*\n)*)",
        text,
        re.M,
    )
    assert match is not None, target
    return match[1].split(), match[2]


def test_root_readme_integrates_kernel_and_committed_finality():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network",
        "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
        "no router and no separate LayerX interface",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")


def test_root_readme_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "root path of these hosts, without a route suffix",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ):
        assert fact in quickstart, fact
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    hosts = {f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)}
    hosts.add("api-mainnet-beta.paxeer.network")
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.hostname in hosts, url
            assert parsed.scheme == "https" and parsed.path in ("", "/"), url
            assert not parsed.query and not parsed.fragment, url
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_root_readme_transaction_to_committed_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`",
        "`eth_sendRawTransaction` to a public node's root JSON-RPC URL",
        "transaction hash",
        "durable WAL acknowledgement",
        "Receipt order is proposal order",
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
    sequence = re.search(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert sequence is not None
    steps = (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order",
        "Commit block",
        "committed, blockNumber, blockHash",
        "eth_getTransactionReceipt / eth_call at committed height",
    )
    positions = [sequence[1].index(step) for step in steps]
    assert positions == sorted(positions)
    send = source("rpc/send.go")
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    receipt = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt


def test_root_readme_retires_custody_checkpoint_and_standalone_operations():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "custody deposit and credit-attestation funding flow is retired" in README
    for stale in (r"\blayerxd\b", r"cmd/layerxd", r"\bmake\s+layerxd\b"):
        assert not re.search(stale, README, re.I), stale


def test_root_readme_builds_and_starts_current_paxd_target():
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    assert len(blocks) == 4
    assert shlex.split(blocks[0], comments=True) == ["make", "paxd-build"]
    assert [shlex.split(line) for line in blocks[1].splitlines()] == [
        ["./build/paxd", "version"],
        ["./build/paxd", "start", "--home", "$PAXD_HOME"],
    ]
    for fact in (
        "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    dependencies, recipe = make_rule(makefile, "paxd-build")
    assert dependencies == ["paxd-native-libs"]
    assert "tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    native_dependencies, _ = make_rule(makefile, "paxd-native-libs")
    assert native_dependencies == ["$(LIBRARY)", "programs-release-build"]
    dependencies, recipe = make_rule(source("chain.mk"), "build")
    assert dependencies == ["paxd-native-libs"]
    assert shlex.split(recipe) == [
        "go", "build", "$(BUILD_FLAGS)", "-o", "./build/paxd", "./daemon/paxd"
    ]


def test_repair_record_maps_finding_proofs_and_readme_owner():
    record = tomllib.loads(source("tools/qualification/audit_repairs/repair_22_33_392.kvx"))
    assert record["repair"]["task"] == "22.33.392"
    assert record["repair"]["base_revision"] == "399c122802fbe3326c62ef9251760c3e53ee4060"
    finding = record["finding"]["root-readme-standalone-kernel"]
    assert finding["audit"] == "22.30.141"
    assert finding["file"] == "README.md"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_root_readme_")
    }
    assert set(finding["tests"]) == declared
    assert len(finding["tests"]) == len(declared) == 5
    wiring = record["wiring"]["root_readme"]
    assert wiring["path"] == "README.md"
    assert wiring["owner_task"] == "22.31.470"
    assert wiring["missing_facts"] == []
    assert "not run" in record["baseline"]["verification"]
    assert "not executed" in record["proof"]["review_only"]
    assert record["evidence"]["ledger"] == "/out/run/gates.jsonl"
