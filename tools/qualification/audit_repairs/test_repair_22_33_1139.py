"""Offline source proof for audit finding 22.30.5.29 on the real root README."""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_1139.kvx")


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


def assert_facts(text, facts):
    for fact in facts:
        assert fact in text, fact


def test_merged_paxd_network_and_commit_finality():
    overview = section("What is Paxeer X Network?", 2)
    assert_facts(overview, (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no router and no separate LayerX interface",
        "Sub-chains are tenant domains stored in the database",
    ))
    diagrams = re.findall(r"```mermaid\n(.*?)\n```", README, re.S)
    assert len(diagrams) == 2
    topology, _ = diagrams
    assert_facts(topology, (
        "subgraph Network[One Paxeer BFT network / EVM chain 125]",
        "subgraph Node[paxd]", "RPC --> Order --> Host --> Native",
        "Validators -->|Commit| State",
    ))
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_root_rpc_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    assert_facts(quickstart, (
        "Public node JSON-RPC uses the root URL",
        "root path of these hosts, without a route suffix",
        "https://api-mainnet-beta.paxeer.network",
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`",
        "node.hyperpaxeer.com` is only the hpx library mirror",
    ))
    hosts = {"api-mainnet-beta.paxeer.network"}
    hosts.update(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15))
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    public = [urlsplit(url) for url in urls
              if urlsplit(url).hostname.endswith("mainnet-beta.paxeer.network")]
    assert public
    for url in public:
        assert url.hostname in hosts
        assert url.scheme == "https" and url.path in ("", "/")
        assert not url.query and not url.fragment
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert "https://api1.mainnet-beta.paxeer.network" in argv
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": [],
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_transaction_to_committed_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    assert_facts(flow, (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "`0x101A`–`0x101D`", "Programs payload bytes go to `0x101D`",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "`eth_getTransactionReceipt`", "commitment does not mean execution succeeded",
        "`eth_call` at an explicit committed height",
    ))
    send = source("rpc/send.go")
    assert_facts(send, (
        "return hash, s.acceptOnSequencer(ctx, tx)",
        "return s.forwardToSequencer(ctx, hash, input)",
        "s.sendConfig.Sequencer.Accept(txbz)",
    ))
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    rpc = source("rpc/sequencer_receipt.go")
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed",
    ]
    assert_facts(rpc, (
        'const SequencerReceiptMethod = "pax_sequencerReceipt"',
        "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))",
        "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted",
    ))
    sequence = re.findall(r"```mermaid\n(.*?)\n```", flow, re.S)
    assert len(sequence) == 1
    positions = [sequence[0].index(fact) for fact in (
        "eth_sendRawTransaction(signed chain-125 bytes)",
        "Execute, sign receipt, durably acknowledge WAL",
        "receipted (execution evidence)", "Propose MsgSequenceBatch in receipt order",
        "Commit block", "committed, blockNumber, blockHash",
    )]
    assert positions == sorted(positions)


def test_native_funding_and_proof_guidance():
    funding = section("Accounts and native funding")
    assert_facts(funding, (
        "Native sub-accounts map to derived bank addresses", "`0x101A`",
        "authorized issuer mint", "chain ID `125` with zero EVM `value`",
        "Wait for commit and read `balanceOf`",
        "custody deposit and credit-attestation funding flow is retired",
        "docs/wiki/PaymentsQuickstart.md",
    ))
    abi = {entry["name"]: entry for entry in json.loads(
        source("precompiles/layerxledger/abi.json")
    ) if entry["type"] == "function"}
    signatures = re.findall(r"\| `([A-Za-z]+)\(([^)]*)\)` \|", funding)
    assert {name for name, _ in signatures} == {
        "openAccount", "accountId", "mint", "payExact", "balanceOf",
    }
    for name, args in signatures:
        assert args.split(",") == [arg["type"] for arg in abi[name]["inputs"]]
        assert abi[name]["stateMutability"] == (
            "view" if name in {"accountId", "balanceOf"} else "nonpayable"
        )
    proof = section("Receipt and proof verification")
    assert_facts(proof, (
        "ProtocolReceipt v2", "pinned for the receipt height",
        "separate from the validator consensus key",
        "`committed` rung requires coverage", "`MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit",
        "trusted validator information", "authenticated root at the relevant height and key",
        "docs/site/docs/concepts/checkpoints.md",
    ))
    for target in re.findall(r"\]\(([^)]+)\)", funding + proof):
        assert (ROOT / target).exists(), target
    assert "ed25519.Sign(k.private, digest[:])" in source("node/sequencer/receiptkey.go")
    assert "codec.DecodeReceipt(receiptBytes)" in source("layerxproof/verify/receipt.go")
    proof_root = "programs/crates/layerx-programs-registry/src/chain_proof/"
    assert "self.verify_header_commit(header, commit)?" in source(proof_root + "head_verify.rs")
    assert "proof.verify(&head.data_hash(), &hash)?" in source(proof_root + "tx.rs")
    assert "verify_store_root(store_name, root, &ops[1].data, next.app_hash())?" in source(
        proof_root + "state.rs"
    )


def test_retired_executors_and_no_separate_deployment():
    assert "no separate kernel daemon, sequencer service or checkpoint deployment" in README
    assert "External checkpoint settlement is retired" in README
    assert not re.search(r"\blayerxd\b|/rpc\b|\blx_\w*", README, re.I)
    assert not re.search(r"merge is (?:only )?(?:a )?(?:draft|future)", README, re.I)
    paragraphs = [p for p in README.split("\n\n") if p.startswith("The pre-merge ")]
    assert len(paragraphs) == 1
    retired = paragraphs[0]
    assert_facts(retired, (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "`layerxAnchor` (`0x1014`)", "rejects writes after the merge",
        "retired refusal surfaces", "`LXP_MODULE_DISABLED`",
        "historical views retire at `v7.0-anchor-store-delete`", "archive-only",
    ))
    inventory = tomllib.loads(source("spec/layerx-merge/programme/data/retired.kvx"))
    current = README.replace(retired, "")
    for component in inventory["component"].values():
        for name in component["symbols"] + component["deleted_trees"]:
            pattern = r"(?<![A-Za-z0-9_])" + re.escape(name)
            if not name.endswith(("_", "/")):
                pattern += r"(?![A-Za-z0-9_])"
            assert not re.search(pattern, current), name
    assert 'pcommon.RetiredRevert("layerxVerify")' in source(
        "precompiles/layerxverify/layerxverify.go"
    )
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor


def test_paxd_build_and_run_guidance():
    build = section("Build")
    assert_facts(build, (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "EVM chain ID `125` is the signing domain",
        "running network entry point is `paxd`",
    ))
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")


def test_repair_record_maps_finding_and_qualification_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["task"]["id"] == "22.33.1139"
    assert record["task"]["finding"] == "22.30.5.29"
    assert record["task"]["wiring_requests"] == []
    assert "domain 17" in record["task"]["wiring_resolution"]
    assert "22.31.470" in record["task"]["repair"]
    finding = record["finding"]["22.30.5.29"]
    assert finding["file"] == "README.md"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_repair_record_maps_finding_and_qualification_limits",
    }
    assert "do not execute" in finding["review_only"]
    assert "no pre-change failures are claimed" in record["baseline"]["pre_change_tests"]
    assert record["qualification"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in record["qualification"]["evidence_policy"]
    assert "final verify row" in record["qualification"]["verified_revision"]
