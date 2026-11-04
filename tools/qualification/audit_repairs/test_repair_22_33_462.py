"""Offline source proof for audit 22.30.148/readme-retired-topology.

Consume the production README and linked sources without executing a toolchain
or simulating a network. Runtime behavior remains outside this documentation gate.
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
RECORD = Path(__file__).with_name("repair_22_33_462.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level):
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
    assert fence is None, heading
    return "".join(lines)


def require_facts(text, facts):
    for fact in facts:
        assert fact in text, fact


def make_target(path, name):
    matches = re.findall(
        r"^" + re.escape(name) + r":([^\n]*)\n((?:\t[^\n]*\n|\n)*)",
        source(path),
        re.M,
    )
    assert len(matches) == 1, (path, name)
    dependencies, recipe = matches[0]
    return dependencies.split(), recipe


def test_readme_describes_one_embedded_paxd_chain_125_network():
    overview = section("What is Paxeer X Network?", 2)
    require_facts(overview, (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "no router and no separate LayerX interface",
        "hosted gateway is optional developer tooling", "outside consensus",
        "does not provide another execution or finality domain",
    ))
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert len(diagrams) == 2
    assert re.findall(r"```mermaid\n(.*?)\n```", overview, re.S) == [diagrams[0]]
    require_facts(diagrams[0], (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ))
    for stale in (r"\blayerxd\b", r"two (?:execution )?domains", r"\blx_[A-Za-z0-9_*]*"):
        assert not re.search(stale, README, re.I), stale


def test_public_rpc_urls_and_chain_probe_use_the_root():
    quickstart = section("Quick start: connect to chain 125", 3)
    require_facts(quickstart, (
        "Public node JSON-RPC uses the root URL",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "https://api-mainnet-beta.paxeer.network",
        "root path of these hosts, without a route suffix",
        "node.hyperpaxeer.com` is only the hpx library mirror",
        "never a JSON-RPC endpoint",
    ))
    hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    public_urls = []
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.hostname in hosts, url
            assert parsed.scheme == "https"
            assert parsed.path in ("", "/")
            assert not parsed.query and not parsed.fragment and parsed.port is None
            public_urls.append(parsed)
    assert {url.hostname for url in public_urls} == {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_transaction_and_committed_receipt_follow_production_sources():
    flow = section("Submit a transaction and follow its committed receipt", 3)
    require_facts(flow, (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`pax_sequencerReceipt`", "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "authenticate the block commitment", "`eth_getTransactionReceipt`",
        "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height", "No separate service endpoint is required",
    ))
    require_facts(source("rpc/send.go"), (
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ))
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    receipt = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', receipt) == [
        "submitted", "receipted", "committed"
    ]
    require_facts(receipt, (
        'const SequencerReceiptMethod = "pax_sequencerReceipt"',
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))",
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted",
    ))
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


def test_custody_and_checkpoint_executors_are_explicitly_retired():
    native = section("EVM and Native Precompiles", 3)
    require_facts(native, (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "archive-only", "`v7.0-anchor-store-delete`",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ))
    assert "custody deposit and credit-attestation funding flow is retired" in README
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    require_facts(source("precompiles/layerxanchor/layerxanchor.go"), (
        "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade",
        "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade",
    ))
    retirements = [p for p in README.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(retirements) == 1
    assert not re.search(
        r"layerx(?:Custody|Anchor|Verify)", README.replace(retirements[0], ""), re.I
    )


def test_build_and_development_guidance_matches_make_targets():
    build = section("Build", 3)
    require_facts(build, (
        "make paxd-build", "build/paxd", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a", "`chain.mk`",
        "make paxeer-build", "make -f chain.mk build", "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "kernel library and development tools without rebuilding the node",
        "make build", "kernel archive, genesis builder and module registry",
        "The running network entry point is `paxd`",
    ))
    dependencies, recipe = make_target("Makefile", "paxd-build")
    assert dependencies == ["paxd-native-libs"]
    require_facts(recipe, ("tools/release/version.json", "$(PAXEER_MAKE) build"))
    dependencies, _ = make_target("Makefile", "paxd-native-libs")
    assert dependencies == ["$(LIBRARY)", "programs-release-build"]
    require_facts(source("Makefile"), (
        "LIBRARY := $(BUILD_DIR)/liblayerx.a",
        "PAXEER_MAKE := $(MAKE) -f $(PAXEER_DIR)/chain.mk",
        "PROGRAMS_RELEASE_RUNTIME_LIB := programs/target/release/liblayerx_programs_sandbox.a",
        "build: $(LIBRARY)", "build: layerx-genesis-build", "build: layerx-module-registry",
    ))
    _, recipe = make_target("Makefile", "paxeer-build")
    assert "$(PAXEER_MAKE) build" in recipe
    dependencies, recipe = make_target("chain.mk", "build")
    assert dependencies == ["paxd-native-libs"]
    assert "go build $(BUILD_FLAGS) -o ./build/paxd ./daemon/paxd" in recipe
    _, recipe = make_target("chain.mk", "paxd-native-libs")
    assert "$(MAKE) -C $(PAXEER_PROJECT_HOME) -f Makefile paxd-native-libs" in recipe
    releases = section("Releases and Network Upgrades", 3)
    require_facts(releases, (
        "executes kernel activity inside `paxd`", "commits it with the BFT chain state",
        "retires the old custody/checkpoint write paths",
    ))


def test_repair_record_maps_the_finding_to_executable_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    repair = record["repair"]
    assert repair["task"] == "22.33.462"
    assert repair["audit"] == "22.30.148"
    assert repair["acceptance"] == "22998.1"
    assert re.fullmatch(r"[0-9a-f]{40}", repair["inspected_revision"])
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_462.py"
    )
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["verified_revision"]
    assert "no pre-change test failure is claimed" in repair["baseline"]
    assert "22.31.470" in repair["readme_owner"]
    assert repair["wiring_requests"] == []
    assert set(record["finding"]) == {"readme-retired-topology"}
    finding = record["finding"]["readme-retired-topology"]
    assert finding["file"] == "README.md"
    assert finding["line"] == 47
    assert finding["class"] == "doc-drift"
    assert finding["severity"] == "medium"
    declarations = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declarations - {
        "test_repair_record_maps_the_finding_to_executable_proofs"
    }
    assert len(finding["tests"]) == len(set(finding["tests"]))
