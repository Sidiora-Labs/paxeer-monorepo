"""Offline source proof for audit 22.30.180's retired-daemon README finding.

Read the production README and its actual node/build paths. No native execution,
network availability, cryptographic operation or restart recovery is certified.
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
FINDING = "network-overview-retired-daemon"
RECORD = Path(__file__).with_name("repair_22_33_838.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    """Keep fenced examples intact when reading a Markdown section."""
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


def make_rule(path, target):
    """Parse the named dependency list and recipe, without invoking make."""
    matches = list(re.finditer(r"^" + re.escape(target) + r":([^\n]*)\n", source(path), re.M))
    assert len(matches) == 1, (path, target)
    match = matches[0]
    recipe = []
    for line in source(path)[match.end():].splitlines():
        if not line.startswith("\t"):
            break
        recipe.append(line)
    return match.group(1).split(), "\n".join(recipe)


def test_network_overview_embeds_the_proposing_kernel_in_paxd():
    overview = section("What is Paxeer X Network?", 2)
    for fact in (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "kernel is embedded in the node",
        "participates in block proposal and deterministic execution",
        "proposes blocks inside `paxd`",
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
    assert "subgraph Network[One Paxeer BFT network / EVM chain 125]" in overview
    assert "subgraph Node[paxd]" in overview
    assert "RPC --> Order --> Host --> Native" in overview
    assert "Validators -->|Commit| State" in overview


def test_public_rpc_numbered_hosts_use_root_and_probe_chain_125():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "`https://api1.mainnet-beta.paxeer.network` through `https://api14.mainnet-beta.paxeer.network`" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    hosts = {
        "api-mainnet-beta.paxeer.network",
        *(f"api{i}.mainnet-beta.paxeer.network" for i in range(1, 15)),
    }
    urls = re.findall(r"https?://[^\s`<>)]+", README)
    public_urls = [urlsplit(url) for url in urls if urlsplit(url).hostname in hosts]
    assert {url.hostname for url in public_urls} >= {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    for url in urls:
        parsed = urlsplit(url)
        if parsed.hostname and parsed.hostname.endswith("mainnet-beta.paxeer.network"):
            assert parsed.hostname in hosts, url
            assert parsed.scheme == "https" and parsed.path in ("", "/"), url
            assert not parsed.query and not parsed.fragment and parsed.port is None, url
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
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart


def test_transaction_submission_and_receipt_transport_match_production():
    flow = section("Submit a transaction and follow its committed receipt")
    for fact in (
        "secp256k1 EIP-1559 transaction for chain ID `125`",
        "`eth_getTransactionCount`", "`eth_sendRawTransaction`",
        "public node's root JSON-RPC URL", "transaction hash",
        "durable WAL acknowledgement", "Receipt order is proposal order",
        "`pax_sequencerReceipt`", "`submitted` has no execution receipt yet",
        "`receipted` includes signed execution evidence",
        "`committed` additionally names the block", "`MsgSequenceBatch`",
        "No separate service endpoint is required",
        "commitment does not mean execution succeeded",
        "`eth_getTransactionReceipt`", "`eth_call` at an explicit committed height",
    ):
        assert fact in flow, fact
    send = source("rpc/send.go")
    assert "func (s *SendAPI) SendRawTransaction(" in send
    assert "return hash, s.acceptOnSequencer(ctx, tx)" in send
    assert "s.sendConfig.Sequencer.Accept(txbz)" in send
    assert "return s.forwardToSequencer(ctx, hash, input)" in send
    assert "return forwardRawTransaction(ctx, s.sendConfig.SequencerURL, hash, input)" in send
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


def test_signed_execution_is_distinct_from_authenticated_block_commitment():
    proof = section("Receipt and proof verification")
    for fact in (
        "ProtocolReceipt v2", "pinned for the receipt height",
        "public key supplied beside an untrusted receipt does not establish trust",
        "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`",
        "authenticate the block header and CometBFT commit against trusted validator information",
        "authenticated root at the relevant height and key",
        "Neither a transaction hash nor a signed execution receipt alone establishes block commitment",
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


def test_custody_and_checkpoint_executors_are_explicitly_retired():
    native = section("EVM and Native Precompiles")
    for fact in (
        "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired at the merge",
        "`layerxAnchor` (`0x1014`) rejects writes after the merge",
        "historical views retire at `v7.0-anchor-store-delete`",
        "retired refusal surfaces retained for compatibility",
        "not active funding or checkpoint executors", "`LXP_MODULE_DISABLED`",
        "Historical anchor reads before store deletion are archive-only",
        "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired",
        "archived formats add no settlement step to current block finality",
    ):
        assert fact in native, fact
    assert 'pcommon.RetiredRevert("layerxVerify")' in source("precompiles/layerxverify/layerxverify.go")
    assert "if p.keeper.Retired(ctx)" in source("precompiles/layerxcustody/layerxcustody.go")
    anchor = source("precompiles/layerxanchor/layerxanchor.go")
    assert "!isView(method.Name) && pcommon.RetiredAt(ctx, p.upgrades(), pcommon.MergeUpgrade" in anchor
    assert "pcommon.RetiredAt(ctx, p.upgrades(), StoreDeleteUpgrade" in anchor
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in source("node/upgrades.go")
    for stale in (
        r"\blayerxd\b", r"two (?:execution )?domains", r"kernel still runs separately",
        r"merge is (?:only )?a draft", r"https?://[^\s`]+/rpc(?:[/\s`]|$)",
    ):
        assert not re.search(stale, README, re.I), stale
    for paragraph in README.split("\n\n"):
        if re.search(r"layerxVerify|layerxCustody|layerxAnchor|guarantor|L0–L4", paragraph):
            assert re.search(r"retired|historical|archived", paragraph), paragraph


def test_kernel_build_and_release_guidance_use_the_paxd_lifecycle():
    build = section("Build")
    for fact in (
        "make paxd-build", "build/liblayerx.a",
        "programs/target/release/liblayerx_programs_sandbox.a",
        "tools/release/version.json", "./build/paxd version",
        './build/paxd start --home "$PAXD_HOME"',
        "operator-provided genesis and consensus chain identifier",
        "For kernel library and development tools without rebuilding the node",
        "make build", "The running network entry point is `paxd`",
    ):
        assert fact in build, fact
    dependencies, _ = make_rule("Makefile", "paxd-native-libs")
    assert dependencies == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]
    dependencies, recipe = make_rule("Makefile", "paxd-build")
    assert dependencies == ["paxd-native-libs"]
    assert "jq -er .version tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    dependencies, recipe = make_rule("chain.mk", "build")
    assert dependencies == ["paxd-native-libs"]
    assert "-o ./build/paxd ./daemon/paxd" in recipe
    assert "server.AddCommands(" in source("daemon/paxd/cmd/root.go")
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd` and commits it with the BFT chain state" in release
    assert "retires the old custody/checkpoint write paths" in release
    assert "`v7.0-anchor-store-delete` removes the historical anchor store" in release


def test_repair_record_maps_the_audit_finding_to_all_proofs_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["838"]
    finding = record["finding"][FINDING]
    audit_text = source("spec/layerx-merge/programme/audit/records/22.30.180.kvx")
    marker = f"[finding.{FINDING}]"
    audit_block = audit_text.split(marker, 1)[1].split("\n[", 1)[0]
    audit = tomllib.loads(marker + audit_block)["finding"][FINDING]
    assert finding["audit"] == "22.30.180"
    assert finding["file"] == audit["file"] == "README.md"
    assert finding["line"] == audit["line"] == 44
    assert finding["class"] == audit["class"] == "doc-drift"
    assert finding["severity"] == audit["severity"] == "medium"
    assert finding["owner_domain"] == audit["owner_domain"] == 17
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    owner = tomllib.loads(source("tools/qualification/audit_repairs/repair_22_31_470.kvx"))
    coverage = owner["coverage"]["audit_22_30_180"]
    assert coverage["finding"] == FINDING and coverage["file"] == "README.md"
    assert "22.33.838" in coverage["consumer_tasks"]
    assert task["readme_owner"] == "README.md | domain 17 | task 22.31.470"
    assert task["wiring_requests"] == []
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_838.py"
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
