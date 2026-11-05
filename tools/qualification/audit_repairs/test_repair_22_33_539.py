"""Offline source proof for audit 22.30.156/readme_retired_architecture.

Read the production README and its cited implementations without executing a
native toolchain or contacting a network. Runtime qualification is separate.
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
RECORD = Path(__file__).with_name("repair_22_33_539.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep shell comments inside fences when locating Markdown boundaries."""
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


def test_readme_retired_architecture_integrated_network():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network", "chain ID **125** (`0x7d`)",
        "kernel is embedded in the node", "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "no separate kernel daemon, sequencer service or checkpoint deployment",
        "External checkpoint settlement is retired",
    ):
        assert fact in overview, fact
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert "var _ tmtypes.SequencerSource = (*ConsensusSource)(nil)" in source(
        "node/sequencer/consensus_source.go"
    )


def test_readme_retired_architecture_root_rpc():
    quickstart = section("Quick start: connect to chain 125")
    for fact in (
        "Public node JSON-RPC uses the root URL",
        "https://api-mainnet-beta.paxeer.network",
        "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`",
        "root path of these hosts, without a route suffix",
        "gateway contract does not establish method availability at public node URLs",
    ):
        assert fact in quickstart, fact
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].split("\n# Expected result:")[0].replace("\\\n", " "))
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert [arg for arg in argv if arg.startswith("https://")] == [
        "https://api1.mainnet-beta.paxeer.network"
    ]
    assert '# Expected result: "0x7d"' in blocks[0]
    for url in re.findall(r"https?://[^\s`<>)]+", README):
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.scheme == "https" and parsed.path in ("", "/")
            assert parsed.hostname in {
                "api-mainnet-beta.paxeer.network",
                *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
            }
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_readme_retired_architecture_transaction_and_commit_flow():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "authenticate the block commitment", "commitment does not mean execution succeeded",
        "`eth_getTransactionReceipt`", "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    release = source("node/sequencer/accepter.go")
    assert release.index("a.rep.Release(context.Background(), &wal.Accept{") < release.index(
        "a.src.Record(common.Hash(acc.ActivityID), send)"
    )
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed"
    ]
    assert "a.coveringHeight(decoded.GlobalSequence, decoded.ResultingStateRoot, uint64(view.Height))" in rpc
    assert "result.Rung, result.BlockNumber, result.BlockHash = RungCommitted" in rpc


def test_readme_retired_architecture_retained_verification():
    proof = section("Receipt and proof verification")
    for fact in (
        "ProtocolReceipt v2", "separate from the validator consensus key",
        "pinned for the receipt height", "canonical receipt bytes and their signature",
        "public key supplied beside an untrusted receipt does not establish trust",
        "`committed` rung requires coverage", "`MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit",
        "trusted validator information", "authenticated root at the relevant height and key",
    ):
        assert fact in proof, fact
    assert "ed25519.Sign(k.private, digest[:])" in source("node/sequencer/receiptkey.go")
    verifier = source("layerxproof/verify/receipt.go")
    assert "codec.DecodeReceipt(receiptBytes)" in verifier
    assert "Ed25519Digest(sequencerPublicKey, *receipt.SequencerSignature, digest)" in verifier
    root = "programs/crates/layerx-programs-registry/src/chain_proof/"
    assert "self.verify_header_commit(header, commit)?" in source(root + "head_verify.rs")
    assert "proof.verify(&head.data_hash(), &hash)?" in source(root + "tx.rs")
    assert "verify_store_root(store_name, root, &ops[1].data, next.app_hash())?" in source(root + "state.rs")


def test_readme_retired_architecture_retirement_and_no_old_onboarding():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`)", "`layerxCustody` (`0x1013`)",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces", "not active funding or checkpoint executors",
        "`LXP_MODULE_DISABLED`", "Historical anchor reads before store deletion are archive-only",
        "retained [batch-header codec]", "archived settlement witness formats",
        "guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert "custody deposit and credit-attestation funding flow is retired" in section(
        "Accounts and native funding"
    )
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    for stale in (
        r"\blayerxd\b", r"cmd/layerxd", r"make\s+layerxd",
        r"two (?:execution )?domains", r"kernel still runs separately",
        r"merge is (?:only )?a draft",
    ):
        assert not re.search(stale, README, re.I), stale
    for paragraph in README.split("\n\n"):
        if re.search(r"layerxAnchor|layerxCustody|guarantor|L0–L4", paragraph):
            assert re.search(r"retired|historical|archived", paragraph), paragraph
    for block in re.findall(r"```(?:sh|bash)\n(.*?)\n```", README, re.S):
        assert not re.search(r"layerxd|layerxAnchor|checkpoint|guarantor", block, re.I)


def test_readme_retired_architecture_build_and_merge_status():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "./build/paxd version", './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "running network entry point is `paxd`",
    ):
        assert fact in build, fact
    makefile = source("Makefile")
    assert "\npaxd-native-libs: $(LIBRARY) programs-release-build\n" in makefile
    assert "\npaxd-build: paxd-native-libs\n" in makefile
    assert "-o ./build/paxd ./daemon/paxd" in source("chain.mk")
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release
    assert "`v7.0` merge handler activates the merged interfaces" in release
    assert "retires the old custody/checkpoint write paths" in release
    for body in (build, release, section("Receipt and proof verification")):
        links = re.findall(r"\]\(([^)]+)\)", body)
        assert links
        for target in links:
            assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_readme_retired_architecture_record_maps_proofs_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["539"]
    finding = record["finding"]["readme_retired_architecture"]
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert finding["audit"] == "22.30.156" and finding["file"] == "README.md"
    assert finding["owner_domain"] == 17 and finding["owner_task"] == "22.31.470"
    assert set(finding["tests"]) == declared
    assert task["wiring_requests"] == []
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_539.py"
    )
