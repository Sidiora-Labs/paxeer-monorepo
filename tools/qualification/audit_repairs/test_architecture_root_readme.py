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
    introduction = section(source("README.md"), "## What is Paxeer X Network?")
    components = introduction.split("The node integrates the following components:", 1)[1].split("A receipt is final", 1)[0]
    for fact in ("**Node and consensus.**", "**Kernel and programs.**", "runs inside `paxd`",
                 "build/liblayerx.a", "execution ordering and the consensus block source within the node"):
        assert fact in components
    paths = re.findall(r"\]\(([^)#]+)\)", components)
    assert set(paths) == {"daemon/paxd/", "node/", "consensus/", "modules/", "rpc/",
                          "modules/lxphost/", "programs/", "node/sequencer/"}
    for path in paths:
        assert (ROOT / path).is_dir(), path
    host = source("modules/lxphost/host.go")
    assert "//go:build cgo" in host
    ldflags = re.search(r"^#cgo LDFLAGS: (.+)$", host, re.M)
    assert ldflags
    assert {"-llayerx", "-llayerx_programs_sandbox"} <= set(
        shlex.split(ldflags.group(1))
    )
    assert "return lxphost.NewHost(lxphost.Config{" in source("node/app.go")


def test_architecture_root_diagram_contains_embedded_execution_and_gateway_relay():
    architecture = section(source("README.md"), "### Integrated network topology")
    diagrams = re.findall(r"^```mermaid\n(.*?)^```$", architecture, re.M | re.S)
    assert len(diagrams) == 1
    diagram = diagrams[0]
    assert diagram.startswith("flowchart LR\n")
    node = diagram.split("subgraph Node[paxd]", 1)[1].split("\n        end", 1)[0]
    for component in ("RPC", "Order", "Host", "Native"):
        assert re.search(rf'^\s+{component}\[[^\n]+\]$', node, re.M)
    for fact in ("RPC --> Order --> Host --> Native", "Embedded C17 kernel and Rust programs host",
                 "Integrated sequencing and block proposal"):
        assert fact in node
    for fact in ("Hosted[Optional hosted developer gateway] -->|Chain RPC and committed state| RPC",
                 "Client[Wallet, SDK or application] -->|Signed EVM transaction / root JSON-RPC| RPC",
                 "Order -->|Proposed block in execution order| Validators[BFT validators]",
                 "Validators -->|Commit| State[Committed block and chain state]",
                 "State -->|Transaction receipts, logs and proofs| Client"):
        assert fact in diagram
    assert "Hosted[" not in node
    for retired in ("layerxd", "layerxAnchor", "0x1014", "LNI", "guarantor"):
        assert retired not in diagram
    assert "outside consensus" in architecture
    assert "does not provide another execution or finality domain" in architecture


def test_architecture_root_build_and_run_match_native_producer():
    build = section(source("README.md"), "### Build")
    assert any("make paxd-build" in block for block in shell_blocks(build))
    prerequisites, recipe = make_rule(source("Makefile"), "paxd-build")
    assert prerequisites == ["paxd-native-libs"]
    native, _ = make_rule(source("Makefile"), "paxd-native-libs")
    assert native == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
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
        "./build/paxd", "version"
    ]
    assert commands[1][:2] == ["./build/paxd", "start"]
    options = commands[1][2:]
    assert len(options) % 2 == 0
    documented = dict(zip(options[::2], options[1::2]))
    assert len(documented) * 2 == len(options), "duplicate node flags"
    assert documented == {"--home": "$PAXD_HOME"}
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
    assert set(runtime_flags) == {
        "--home", "--rpc.laddr", "--evm.http_enabled", "--evm.http_address",
        "--evm.http_port", "--sequencer.enable", "--sequencer.wal-dir",
        "--sequencer.receipt-key-file",
    }
    for name in ("--evm.http_enabled", "--sequencer.enable"):
        assert ast.literal_eval(runtime_flags[name]) == "true"
    assert "operator-provisioned, initialized node home" in build
    assert "network genesis and configuration" in build
    assert "operator-provided genesis and consensus chain identifier" in build
    assert "EVM chain ID `125` is the signing domain" in build
    assert "Building the binary alone does not provision network membership" in build


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
        assert origin in source("docs/site/paxeer-docs/src/app/public-rpc/page.tsx")
    assert "`debug`, `echo`, `eth`, `net`, `pax`, `px` and `txpool`" in documentation
    assert "Method availability depends on node configuration" in documentation
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
    historical = section(readme, "### EVM and Native Precompiles")
    for term in ("layerxVerify", "layerxCustody", "layerxAnchor", "0x1014", "guarantor"):
        assert term in historical
    assert "retired refusal surfaces retained for compatibility" in historical
    assert "not active funding or checkpoint executors" in historical
    assert "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired" in historical
    introduction = section(readme, "## What is Paxeer X Network?")
    assert "There is no separate kernel daemon, sequencer service or checkpoint deployment" in introduction
    assert "layerxd" not in introduction
    for block in shell_blocks(readme):
        for retired in ("layerxd", "layerxAnchor", "checkpoint", "/rpc/evm", "/rpc/schema"):
            assert retired not in block
