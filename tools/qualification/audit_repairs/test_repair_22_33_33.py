"""Prove architecture-root README facts against committed deployment sources offline."""

import ast
import json
from pathlib import Path
import re
import shlex
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(document, heading):
    lines = document.splitlines(keepends=True)
    assert heading + "\n" in lines, f"Missing section: {heading}"
    start = lines.index(heading + "\n") + 1
    level = len(heading.split()[0])
    fenced = False
    content = []
    for line in lines[start:]:
        if line.startswith("```"):
            fenced = not fenced
        if not fenced and re.match(rf"^#{{1,{level}}} ", line):
            break
        content.append(line)
    assert not fenced, f"Unclosed code fence in section: {heading}"
    return "".join(content)


def make_rule(document, target):
    match = re.search(
        rf"^{re.escape(target)}:([^\n]*)\n((?:\t[^\n]*\n)*)",
        document,
        re.M,
    )
    assert match, f"Missing make target: {target}"
    return shlex.split(match.group(1)), match.group(2)


def shell_blocks(document):
    return re.findall(r"^```sh\n(.*?)^```$", document, re.M | re.S)


def test_architecture_root_introduction_and_receipt_commit_boundary():
    introduction = section(source("README.md"), "## What is Paxeer X Network?")
    assert "one BFT network" in introduction
    assert "**125** (`0x7d`)" in introduction
    assert "kernel is embedded in the node" in introduction
    assert "A receipt is final once its block is committed." in introduction
    assert "signed execution receipt can arrive before that commitment" in introduction

    receipt = source("rpc/sequencer_receipt.go")
    rungs = dict(re.findall(r'\bRung(\w+)\s+Rung\s*=\s*"(\w+)"', receipt))
    assert rungs == {
        "Submitted": "submitted",
        "Receipted": "receipted",
        "Committed": "committed",
    }
    for rung in rungs.values():
        assert f"`{rung}`" in introduction
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    producer = receipt.split("func (a *SequencerReceiptAPI) SequencerReceipt(", 1)[1]
    producer = producer.split("\nfunc ", 1)[0]
    transitions = [
        "a.option.Source.SequencerSend(txHash)",
        "Rung: RungSubmitted",
        "Rung: RungReceipted",
        "a.coveringHeight(",
        "if err != nil || !committed",
        "a.tmClient.Block(ctx, &height)",
        "block.Block.Height != height",
        "= RungCommitted, &blockNumber, &blockHash",
    ]
    positions = [producer.index(transition) for transition in transitions]
    assert positions == sorted(positions)


def test_architecture_root_component_table_links_real_runtime_boundaries():
    architecture = section(source("README.md"), "### Architecture")
    table = architecture.split("```mermaid", 1)[0]
    rows = [line for line in table.splitlines() if line.startswith("| ")]
    assert [row.split("|")[1].strip() for row in rows[2:]] == [
        "Paxeer node",
        "Embedded kernel and proposer",
        "Native state and finality",
        "Public node RPC",
        "Hosted gateway",
    ]
    paths = re.findall(r"\]\(([^)#]+)\)", table)
    assert paths
    for path in paths:
        assert (ROOT / path).exists(), path
    assert "inside the node" in rows[3]
    assert "configured separately" in rows[-1]
    host = source("modules/lxphost/host.go")
    assert "//go:build cgo" in host
    ldflags = re.search(r"^#cgo LDFLAGS: (.+)$", host, re.M)
    assert ldflags
    assert {"-llayerx", "-llayerx_programs_sandbox"} <= set(
        shlex.split(ldflags.group(1))
    )
    assert "return lxphost.NewHost(lxphost.Config{" in source("node/app.go")


def test_architecture_root_diagram_contains_embedded_execution_and_gateway_relay():
    architecture = section(source("README.md"), "### Architecture")
    diagrams = re.findall(r"^```mermaid\n(.*?)^```$", architecture, re.M | re.S)
    assert len(diagrams) == 1
    diagram = diagrams[0]
    assert diagram.startswith("flowchart TB\n")
    node = diagram.split('subgraph Node["paxd: one BFT network, EVM chain ID 125"]', 1)[1]
    node = node.split("\n  end", 1)[0]
    for component in ("RPC", "Kernel", "Commit", "State"):
        assert re.search(rf'^\s+{component}\["[^"\n]+"\]$', node, re.M)
    assert "Embedded LayerX kernel: modules/lxphost and node/sequencer" in node
    assert "BFT block commit: receipt finality" in node
    assert 'Gateway -->|"Signed transaction relay and node reads"| RPC' in diagram
    assert 'Client -->|"Public node root JSON-RPC"| RPC' in diagram
    assert 'Client -->|"Hosted service requests"| Gateway' in diagram
    assert 'RPC -->|"Transaction admission"| Kernel' in node
    assert 'Kernel -->|"Proposal and deterministic execution"| Commit' in node
    assert "Commit --> State" in node
    assert 'State -->|"Committed reads and receipt evidence"| RPC' in node
    assert "Gateway[" not in node
    for retired in ("layerxd", "layerxAnchor", "0x1014", "LNI", "guarantor"):
        assert retired not in diagram
    assert "does not add a consensus or settlement step" in architecture
    assert "deployment files alone do not certify a running service" in architecture


def test_architecture_root_build_and_run_match_native_producer():
    build = section(source("README.md"), "### Build")
    assert any("make paxd-build" in block for block in shell_blocks(build))
    prerequisites, recipe = make_rule(source("Makefile"), "paxd-build")
    assert prerequisites == ["paxd-native-libs"]
    native, _ = make_rule(source("Makefile"), "paxd-native-libs")
    assert native == ["$(LIBRARY)", "programs-release-build"]
    assert "tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    chain_native, chain_build = make_rule(source("chain.mk"), "build")
    assert chain_native == ["paxd-native-libs"]
    assert "go build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd" in chain_build
    for archive in (
        "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
    ):
        assert archive in source("chain.mk") and archive in build

    commands = [
        shlex.split(line)
        for block in shell_blocks(build)
        for line in block.replace("\\\n", " ").splitlines()
        if line.startswith("./build/paxd ")
    ]
    assert len(commands) == 2
    assert commands[0] == [
        "./build/paxd", "validate-genesis", "--home", "/path/to/prepared-node"
    ]
    assert commands[1][:2] == ["./build/paxd", "start"]
    options = commands[1][2:]
    assert len(options) % 2 == 0
    documented = dict(zip(options[::2], options[1::2]))
    assert len(documented) * 2 == len(options), "duplicate node flags"
    assert documented == {
        "--home": "/path/to/prepared-node",
        "--rpc.laddr": "tcp://127.0.0.1:26657",
        "--evm.http_enabled": "true",
        "--evm.http_address": "127.0.0.1",
        "--evm.http_port": "8545",
        "--sequencer.enable": "true",
        "--sequencer.wal-dir": "/path/to/prepared-node/sequencer-wal",
        "--sequencer.receipt-key-file": "/path/to/protected-receipt.key",
    }
    runtime = ast.parse(source("services/platform/hosted/node/native_runtime.py"))
    start = next(
        node for node in runtime.body
        if isinstance(node, ast.FunctionDef) and node.name == "start_native"
    )
    calls = [
        node for node in ast.walk(start)
        if isinstance(node, ast.Call)
        and isinstance(node.func, ast.Attribute)
        and isinstance(node.func.value, ast.Name)
        and node.func.value.id == "commands"
        and node.func.attr == "start"
    ]
    assert len(calls) == 1
    arguments = calls[0].args[0]
    assert isinstance(arguments, ast.List)
    assert isinstance(arguments.elts[0], ast.Attribute)
    assert arguments.elts[0].attr == "paxd"
    assert ast.literal_eval(arguments.elts[1]) == "start"
    runtime_flags = {
        ast.literal_eval(arguments.elts[index]): arguments.elts[index + 1]
        for index in range(2, len(arguments.elts), 2)
    }
    assert set(documented) == set(runtime_flags)
    for name in ("--evm.http_enabled", "--sequencer.enable"):
        assert documented[name] == ast.literal_eval(runtime_flags[name])
    assert "authoritative genesis" in build
    assert "app_state.sequencer.receipt_keys" in build
    assert "owner-only permissions" in build
    assert "Keep the node home, validator signing state and receipt WAL" in build


def test_architecture_root_public_rpc_example_and_hosted_contract_are_distinct():
    documentation = section(source("README.md"), "## Documentation")
    blocks = shell_blocks(documentation)
    assert len(blocks) == 1
    command = shlex.split(blocks[0].split("# Expected result:", 1)[0].replace("\\\n", " "))
    assert command[0] == "curl"
    assert command[command.index("--max-time") + 1] == "10"
    urls = [token for token in command if token.startswith("https://")]
    assert urls == ["https://api1.mainnet-beta.paxeer.network"]
    assert urlsplit(urls[0]).path == ""
    assert json.loads(command[command.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert 'Expected result: "0x7d"' in blocks[0]
    for origin in (
        "https://api-mainnet-beta.paxeer.network",
        "https://api1.mainnet-beta.paxeer.network",
        "https://api14.mainnet-beta.paxeer.network",
    ):
        assert origin in documentation
        assert origin in source("docs/site/docs/reference/public-rpc.md")
    assert "`debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`" in documentation
    assert "method availability depends on node configuration" in documentation
    assert "Obtain its deployment URL and credentials from the operator" in documentation
    assert "does not establish method availability at public node URLs" in documentation
    contract = json.loads(source("services/platform/hosted/gateway/openrpc.json"))
    methods = {method["name"] for method in contract["methods"]}
    assert {"px_getNetwork", "px_getCommit", "pax_sequencerReceipt"} <= methods
    assert not contract.get("servers"), "Do not invent a public hosted origin"
    relay = source("services/platform/hosted/gateway/src/evm.rs")
    assert 'pub const SEQUENCER_METHODS: [&str; 1] = ["pax_sequencerReceipt"];' in relay
    assert "is_evm_namespace(method) || SEQUENCER_METHODS.contains(&method)" in relay


def test_architecture_root_retired_model_is_labelled_and_never_launched():
    readme = source("README.md")
    historical = section(readme, "#### Historical model (retired)")
    for term in ("layerxd", "layerxAnchor", "0x1014", "checkpoint settlement", "guarantor"):
        assert term in historical
    assert "retired" in historical and "archived deployments" in historical
    assert "not additional processes or settlement instructions" in historical
    current = readme.replace(historical, "")
    assert "layerxd" not in current
    for block in shell_blocks(readme):
        for retired in ("layerxd", "layerxAnchor", "checkpoint", "/rpc/evm", "/rpc/schema"):
            assert retired not in block
