"""Offline source proof for readme-two-domains; no native execution is claimed."""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_418.kvx")


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


def test_readme_two_domains_is_one_integrated_chain_125_network():
    overview = section("What is Paxeer X Network?", level=2)
    assert "one BFT network" in overview
    assert "chain ID **125** (`0x7d`)" in overview
    assert "kernel is embedded in the node" in overview
    assert "participates in block proposal and deterministic execution" in overview
    assert "proposes blocks inside `paxd`" in overview
    assert "validators commit those blocks through BFT consensus" in overview
    assert "Sub-chains are tenant domains stored in the database" in overview
    assert "does not require new node infrastructure" in overview
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert not re.search(
        r"two (?:execution )?domains|kernel still runs separately|merge is (?:only )?a draft",
        README,
        re.I,
    )


def test_readme_two_domains_root_rpc_hosts_and_chain_id_example():
    quickstart = section("Quick start: connect to chain 125")
    assert "Public node JSON-RPC uses the root URL" in quickstart
    assert "root path of these hosts, without a route suffix" in quickstart
    urls = re.findall(r"`(https://[^`]+)`", quickstart)
    assert urls == [
        "https://api-mainnet-beta.paxeer.network",
        "https://api1.mainnet-beta.paxeer.network",
        "https://api14.mainnet-beta.paxeer.network",
    ]
    assert f"`{urls[1]}` through `{urls[2]}`" in quickstart
    for url in urls:
        parsed = urlsplit(url)
        assert parsed.scheme == "https" and parsed.path in {"", "/"}
        assert not parsed.query and not parsed.fragment and not parsed.username
    blocks = re.findall(r"```sh\n(.*?)\n```", quickstart, re.S)
    assert len(blocks) == 1
    argv = shlex.split(blocks[0].replace("\\\n", " "), comments=True)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    assert [arg for arg in argv if arg.startswith("https://")] == [urls[1]]
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert "gateway contract does not establish method availability at public node URLs" in quickstart
    assert not re.search(r"https?://[^\s`]+/rpc(?:[/\s`]|$)", README)


def test_readme_two_domains_transaction_to_committed_receipt_path():
    flow = section("Submit a transaction and follow its committed receipt")
    assert "Sign a secp256k1 EIP-1559 transaction for chain ID `125`" in flow
    assert "`eth_sendRawTransaction` to a public node's root JSON-RPC URL" in flow
    assert "Its result is a transaction hash" in flow
    assert "accepts on the node's integrated sequencing path or forwards" in flow
    assert "Execution produces a signed receipt" in flow
    assert "waits for durable WAL acknowledgement" in flow
    assert "Receipt order is proposal order" in flow
    assert "Query `pax_sequencerReceipt` with the transaction hash" in flow
    assert "`submitted` has no execution receipt yet" in flow
    assert "`receipted` includes signed execution evidence" in flow
    assert "`committed` additionally names the block whose `MsgSequenceBatch` covers that sequence" in flow
    assert "Wait for `committed` and authenticate the block commitment" in flow
    assert "`eth_getTransactionReceipt`" in flow and "inspect its status and logs" in flow
    assert "commitment does not mean execution succeeded" in flow
    assert "`eth_call` at an explicit committed height" in flow
    assert "No separate service endpoint is required" in flow
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    assert re.findall(r'Rung\w+ Rung = "(\w+)"', rpc) == [
        "submitted", "receipted", "committed"
    ]
    assert "covered by the MsgSequenceBatch of a committed block" in rpc
    for target in re.findall(r"\]\(([^)]+)\)", flow):
        assert (ROOT / target).is_file(), target


def test_readme_two_domains_signed_execution_is_distinct_from_finality():
    overview = section("What is Paxeer X Network?", level=2)
    evidence = section("Receipt and proof verification")
    assert "A receipt is final once its block is committed" in overview
    assert "A signed execution receipt can arrive before that commitment" in overview
    assert "ProtocolReceipt v2" in evidence
    assert "separate from the validator consensus key" in evidence
    assert "pinned for the receipt height" in evidence
    assert "public key supplied beside an untrusted receipt does not establish trust" in evidence
    assert "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`" in evidence
    assert "authenticate the block header and CometBFT commit against trusted validator information" in evidence
    assert "inclusion against the authenticated root at the relevant height and key" in evidence
    assert "Neither a transaction hash nor a signed execution receipt alone establishes block commitment" in evidence
    producer = source("node/sequencer/receiptkey.go")
    assert "keyID, err := k.KeyID(in.Accepted.Height)" in producer
    assert "digest := codec.ReceiptDigest(unsigned)" in producer
    assert "ed25519.Sign(k.private, digest[:])" in producer
    verifier = source("layerxproof/verify/receipt.go")
    assert "codec.DecodeReceipt(receiptBytes)" in verifier
    assert "Ed25519Digest(sequencerPublicKey, *receipt.SequencerSignature, digest)" in verifier
    proof_root = "programs/crates/layerx-programs-registry/src/chain_proof/"
    assert "self.verify_header_commit(header, commit)?" in source(proof_root + "head_verify.rs")
    assert "proof.verify(&head.data_hash(), &hash)?" in source(proof_root + "tx.rs")
    assert "verify_store_root(store_name, root, &ops[1].data, next.app_hash())?" in source(proof_root + "state.rs")


def test_readme_two_domains_retired_executors_and_deployment_history():
    overview = section("What is Paxeer X Network?", level=2)
    precompiles = section("EVM and Native Precompiles")
    assert "There is no separate kernel daemon, sequencer service or checkpoint deployment" in overview
    assert "External checkpoint settlement is retired" in overview
    assert "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired" in precompiles
    assert "`layerxAnchor` (`0x1014`) rejects writes after the merge" in precompiles
    assert "historical views retire at `v7.0-anchor-store-delete`" in precompiles
    assert "not active funding or checkpoint executors" in precompiles
    assert "Historical anchor reads before store deletion are archive-only" in precompiles
    assert "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired" in precompiles
    assert "archived formats add no settlement step to current block finality" in precompiles
    assert "custody deposit and credit-attestation funding flow is retired" in section("Accounts and native funding")
    for paragraph in README.split("\n\n"):
        if re.search(r"\blayerxd\b|\blayerxAnchor\b|guarantor|L0–L4", paragraph):
            assert re.search(r"historical|archived|retired", paragraph, re.I), paragraph
    assert not re.search(r"\bmake\b[^\n]*\blayerxd\b", README)


def test_readme_two_domains_build_and_release_use_integrated_paxd():
    build = section("Build")
    assert "make paxd-build" in build
    assert "build/liblayerx.a" in build
    assert "programs/target/release/liblayerx_programs_sandbox.a" in build
    assert './build/paxd start --home "$PAXD_HOME"' in build
    assert "running network entry point is `paxd`" in build
    makefile = source("Makefile")
    assert re.search(r"^paxd-native-libs: \$\(LIBRARY\) programs-release-build$", makefile, re.M)
    assert re.search(r"^paxd-build: paxd-native-libs$", makefile, re.M)
    recipe = makefile.split("\npaxd-build: paxd-native-libs\n", 1)[1].split("\n\n", 1)[0]
    assert "$(PAXEER_MAKE) build" in recipe
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release


def test_readme_two_domains_record_maps_finding_proofs_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["418"]
    finding = record["finding"]["readme-two-domains"]
    assert finding["audit"] == "22.30.144"
    assert finding["file"] == "README.md" and finding["owner_domain"] == 16
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert task["readme_owner_task"] == "22.31.470"
    assert task["wiring_requests"] == []
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
