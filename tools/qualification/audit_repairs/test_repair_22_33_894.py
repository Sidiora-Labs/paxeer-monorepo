"""Source proofs for readme-two-domains; no native or live-network qualification."""

import ast
import hashlib
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_894.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(title, level=3):
    """Keep comments inside fenced examples from becoming section boundaries."""
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


def test_readme_two_domains_integrated_paxd_and_bft_finality():
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


def test_readme_two_domains_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    assert (
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`"
    ) in quickstart
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    public = [urlsplit(url) for url in urls if "mainnet-beta.paxeer.network" in url]
    assert public
    hosts = {f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)}
    hosts.add("api-mainnet-beta.paxeer.network")
    for url in public:
        assert url.scheme == "https" and url.hostname in hosts
        assert url.path in ("", "/") and not url.query and not url.fragment
    examples = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(examples) == 1
    argv = shlex.split(examples[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert '# Expected result: "0x7d"' in examples[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_two_domains_transaction_to_committed_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "EIP-1559 transaction for chain ID `125`", "`eth_getTransactionCount`",
        "`eth_sendRawTransaction`", "transaction hash", "durable WAL acknowledgement",
        "Receipt order is proposal order", "`pax_sequencerReceipt`",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "No separate service endpoint is required",
        "authenticate the block commitment", "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
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
    accepter = source("node/sequencer/accepter.go")
    assert accepter.index("a.rep.Release(context.Background(), &wal.Accept{") < accepter.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in receipt
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in receipt
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in receipt
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    positions = [sequence[0].index(fact) for fact in (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "Release signed execution receipt", "receipted (execution evidence)",
        "Propose MsgSequenceBatch in receipt order", "Commit block",
        "committed, blockNumber, blockHash",
    )]
    assert positions == sorted(positions)


def test_readme_two_domains_retired_executors_and_historical_settlement():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads", "archive-only",
        "archived settlement witness formats", "support historical interpretation",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_readme_two_domains_build_and_run_guidance_has_no_daemon():
    build = section("Build")
    for fact in (
        "Build the merged node from the repository root", "make paxd-build",
        "build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a",
        "operator-provisioned, initialized node home", "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "For kernel library and development tools without rebuilding the node",
        "make build", "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build evmone-lib\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    # The owning rewrite removed historical daemon commands entirely; the current
    # kernel-library target is explicitly distinguished from the network runtime.
    for stale in (
        r"\blayerxd\b", r"two (?:execution )?domains", r"kernel still runs separately",
        r"merge is (?:only )?a draft", r"make\s+(?:layerxd|daemon)-build",
        r"/rpc(?:/|\b)",
    ):
        assert not re.search(stale, README, re.I), stale


def make_rule(path, target):
    """Read literal prerequisites and recipes without invoking a build tool."""
    matches = list(re.finditer(r"^" + re.escape(target) + r":([^\n]*)\n", source(path), re.M))
    assert len(matches) == 1, (path, target)
    match = matches[0]
    recipe = []
    for line in source(path)[match.end():].splitlines():
        if not line.startswith("\t"):
            break
        recipe.append(line[1:])
    return match.group(1).split(), "\n".join(recipe)


def test_readme_two_domains_node_and_hosted_contracts_are_distinct():
    quickstart = section("Quick start: connect to chain 125")
    contract_path = "services/platform/hosted/gateway/openrpc.json"
    for path in (contract_path, "rpc/server.go", "docs/site/docs/reference/public-rpc.md"):
        assert f"]({path})" in README, path
        assert (ROOT / path).is_file(), path
    for fact in (
        "hosted gateway's `px_` API is defined by", "deployment URL and credentials from the operator",
        "does not establish method availability at public node URLs",
        "`px_getReceiptKeys` is registered by the node",
    ):
        assert fact in quickstart, fact
    topology = section("Integrated network topology")
    assert "hosted gateway is optional developer tooling" in topology
    assert "outside consensus" in topology
    assert "does not provide another execution or finality domain" in topology
    contract = json.loads(source(contract_path))
    assert contract["openrpc"] == "1.3.2"
    methods = [method["name"] for method in contract["methods"]]
    assert len(methods) == len(set(methods))
    assert {"pax_sequencerReceipt", "px_getReceiptKeys", "px_getCommit"} <= set(methods)
    description = contract["info"]["description"]
    for fact in (
        "one execution domain in paxd", "chain ID 125 (0x7d)",
        "root path", "Hosted JSON-RPC uses JSON-RPC 2.0 POST /",
        "Public node availability does not establish hosted px_* API deployment",
        "submitted and receipted do not establish finality",
    ):
        assert fact in description, fact
    registrations = source("rpc/server.go")
    assert re.search(r'Namespace: "eth",\s*Service:\s*sendAPI', registrations)
    assert re.search(r'Namespace: "pax",\s*Service:\s*sequencerReceiptAPI', registrations)
    assert re.search(r'Namespace: "px",\s*Service:\s*&receiptKeysAPI', registrations)


def test_readme_two_domains_native_bank_and_program_entry_points():
    funding = section("Accounts and native funding")
    assert "Native sub-accounts map to derived bank addresses inside the node" in funding
    assert "](modules/lxphost/accountmap.go)" in funding
    assert "`layerxledger` precompile" in funding and "`0x101A`" in funding
    ledger = {
        item["name"]: item for item in json.loads(source("precompiles/layerxledger/abi.json"))
        if item.get("type") == "function"
    }
    for name, inputs, mutability in (
        ("openAccount", ["bytes32"], "nonpayable"),
        ("accountId", ["bytes32", "bytes32"], "view"),
        ("mint", ["bytes32", "bytes32", "uint128"], "nonpayable"),
        ("payExact", ["bytes32", "bytes32", "bytes32", "uint128", "uint64", "bytes32"], "nonpayable"),
        ("balanceOf", ["bytes32", "bytes32"], "view"),
    ):
        signature = name + "(" + ",".join(inputs) + ")"
        assert f"`{signature}`" in funding, signature
        assert [item["type"] for item in ledger[name]["inputs"]] == inputs
        assert ledger[name]["stateMutability"] == mutability
    assert "zero EVM `value`; these methods are nonpayable" in funding
    assert "Wait for commit and read `balanceOf`" in funding
    flow = section("Submit a transaction and follow its committed receipt")
    assert "Programs payload bytes go to `0x101D`" in flow
    native = section("EVM and Native Precompiles")
    assert "| `0x101D` | `layerxprograms` |" in native
    assert "base set includes `bank`" in native
    registry = source("precompiles/setup.go")
    assert '{ecommon.HexToAddress("0x000000000000000000000000000000000000101D"), "layerxprograms"}' in registry
    assert '{ecommon.HexToAddress("0x000000000000000000000000000000000000101A"), "layerxledger"}' in registry
    assert 'bank.GetVersioned(latestUpgrade, keepers)' in registry


def test_readme_two_domains_directory_build_and_release_are_merged():
    directory = section("Directory Structure", 2)
    assert "Source directory names describe code organization, not separate network deployments" in directory
    for path in ("modules", "precompiles", "node", "rpc", "programs"):
        assert f"`{path}`" in directory, path
        assert (ROOT / path).is_dir(), path
    spec = section("Specification", 2)
    assert "This README is the maintained architecture entry point" in spec
    assert "translated and archived pages must be read against this network model" in spec
    build = section("Build")
    blocks = re.findall(r"```sh\n(.*?)\n```", build, re.S)
    assert len(blocks) == 4
    assert shlex.split(blocks[0], comments=True) == ["make", "paxd-build"]
    assert [shlex.split(line) for line in blocks[1].splitlines()] == [
        ["./build/paxd", "version"], ["./build/paxd", "start", "--home", "$PAXD_HOME"],
    ]
    deps, recipe = make_rule("Makefile", "paxd-build")
    assert deps == ["paxd-native-libs"]
    assert "jq -er .version tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    assert make_rule("Makefile", "paxd-native-libs")[0] == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
    assert make_rule("chain.mk", "build") == (
        ["paxd-native-libs"], "go build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd",
    )
    assert "tools/release/build-paxd.sh" in build
    assert "packages the merged `paxd`, the price sidecar and the required shared libraries" in build
    assert "](RELEASE.md)" in build
    release = section("Releases and Network Upgrades")
    for fact in (
        "merged runtime executes kernel activity inside `paxd`",
        "commits it with the BFT chain state", "`v7.0` merge handler",
        "retires the old custody/checkpoint write paths",
        "`v7.0-anchor-store-delete` removes the historical anchor store",
    ):
        assert fact in release, fact
    assert 'MergeUpgrade = "v7.0"' in source("precompiles/common/precompiles.go")
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")


def test_readme_two_domains_record_maps_finding_and_proof_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["readme-two-domains"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.184.kvx"))
    original = audit["finding"]["readme-two-domains"]
    assert finding["audit"] == "22.30.184"
    for key in ("file", "line", "class", "severity", "expected", "owner_domain"):
        assert finding[key] == original[key], key
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert len(finding["tests"]) == len(declared) == 9
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["894"]
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_894.py"
    )
    assert task["base_revision"] == "7988614413df79a83ae525988b7636ae368938a2"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["source_proof"] and task["review_only"] and task["baseline"]
    assert task["baseline_failures_claimed"] == []
    assert task["missing_readme_facts"] == []
    assert "same candidate revision" in task["verified_revision_source"]
    assert hashlib.sha256((ROOT / "README.md").read_bytes()).hexdigest() == task["readme_sha256"]
    wiring = record["wiring"]["readme"]
    assert wiring["path"] == "README.md" and wiring["owner_task"] == "22.31.470"
    assert wiring["owner_domain"] == 17 and wiring["status"] == "satisfied"
    assert len(task["wiring_requests"]) == 1
    assert "README.md" in task["wiring_requests"][0]
    assert "domain 17/task 22.31.470" in task["wiring_requests"][0]
