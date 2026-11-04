"""Prove finding 39's README contracts against committed production sources.

These are offline documentation/source checks, not native runtime qualification.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_9.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(heading, level=3):
    marker = "#" * level + " " + heading + "\n"
    assert README.count(marker) == 1
    body = README.split(marker, 1)[1]
    lines = []
    fence = None
    for line in body.splitlines(keepends=True):
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


def abi_methods(path):
    return {
        entry["name"]: entry
        for entry in json.loads(source(path))
        if entry["type"] == "function"
    }


def test_f39_one_bft_network_and_embedded_kernel():
    overview = section("What is Paxeer X Network?", level=2)
    assert "one BFT network" in overview
    assert "chain ID **125** (`0x7d`)" in overview
    assert "kernel is embedded in the node" in overview
    assert "participates in block proposal and deterministic execution" in overview
    assert "Sub-chains are tenant domains stored in the database" in overview
    assert "does not require new node infrastructure" in overview
    assert "app.LayerXHost, err = NewHost(" in source("node/app.go")
    assert 'import "C"' in source("modules/lxphost/host.go")
    assert not re.search(
        r"two (?:execution )?domains|kernel still runs separately|merge is (?:only )?a draft",
        README,
        re.I,
    )


def test_f39_binding_preserves_domain_separated_dual_consent():
    accounts = section("Accounts and native funding")
    assert "secp256k1" in accounts and "Ed25519" in accounts
    assert "did:layerx:<64 lowercase hex characters of the Ed25519 public key>" in accounts
    assert "`0x1004`" in accounts
    assert '"LX:PAXEER-BIND:v1" || chainId:uint256be || evmAddress:20 || nonce:uint64be' in accounts
    assert "64-byte Ed25519 signature" in accounts
    assert "EVM transaction supplies the address owner's consent" in accounts
    assert "Binding does not fund an account" in accounts
    binding = abi_methods("precompiles/addr/abi.json")
    for name, inputs, outputs, mutability in (
        ("layerXBindNonce", ["address"], ["uint64"], "view"),
        ("bindLayerX", ["bytes32", "bytes"], [], "nonpayable"),
        ("getLayerXDid", ["address"], ["bytes32", "string"], "view"),
        ("getUnifiedAccount", ["address"], ["address", "string", "bytes32", "bytes32"], "view"),
    ):
        assert name in accounts
        assert [item["type"] for item in binding[name]["inputs"]] == inputs
        assert [item["type"] for item in binding[name]["outputs"]] == outputs
        assert binding[name]["stateMutability"] == mutability
    message = source("modules/evm/types/layerx_binding.go")
    assert 'LayerXBindDomain = "LX:PAXEER-BIND:v1"' in message
    assert "var chain [32]byte" in message
    assert "chainID.FillBytes(chain[:])" in message
    assert "message = append(message, evmAddress[:]...)" in message
    assert "binary.BigEndian.AppendUint64(message, nonce)" in message
    assert "EIP-712/HKDF-SHA256" in accounts and "original derivation context" in accounts
    assert "docs/site/docs/concepts/one-account.md" in accounts


def test_f39_native_funding_matches_abi_dispatch_and_keeper():
    accounts = section("Accounts and native funding")
    methods = abi_methods("precompiles/layerxledger/abi.json")
    signatures = re.findall(r"\| `([A-Za-z]+)\(([^)]*)\)` \|", accounts)
    assert {name for name, _ in signatures} == {
        "openAccount", "accountId", "mint", "payExact", "balanceOf"
    }
    for name, args in signatures:
        assert args.split(",") == [item["type"] for item in methods[name]["inputs"]]
        expected = "view" if name in {"accountId", "balanceOf"} else "nonpayable"
        assert methods[name]["stateMutability"] == expected
    assert "Native sub-accounts map to derived bank addresses inside the node" in accounts
    assert "`0x101A`" in accounts
    assert "Account ID derivation alone does not open or fund an account" in accounts
    assert "authorized issuer mint" in accounts
    assert "funded account owned by the bound caller" in accounts
    assert "chain ID `125` with zero EVM `value`" in accounts
    assert "expiry in Unix milliseconds and a payment reference" in accounts
    assert "Wait for commit and read `balanceOf`" in accounts
    assert 'eth_getBalance(address,"latest")' in accounts
    assert "custody deposit and credit-attestation funding flow is retired" in accounts

    mapping = source("modules/lxphost/accountmap.go")
    assert 'name := "agent:" + evmtypes.LayerXDid(didKey) + ":asset:"' in mapping
    assert "codec.DeriveAccountID([]byte(name))" in mapping
    assert "return address.Module(bankModuleName, accountID[:])" in mapping
    dispatch = source("precompiles/layerxledger/layerxledger.go")
    assert 'LedgerAddress  = "0x000000000000000000000000000000000000101A"' in dispatch
    assert "pcommon.ValidateNonPayable(value)" in dispatch
    assert "actor, err = p.keeper.Actor(ctx, caller)" in dispatch
    transfers = source("precompiles/layerxledger/transfers.go")
    assert 'ledgerMethods["openAccount"] = (*PrecompileExecutor).openAccount' in transfers
    assert "p.keeper.OpenAccount(ctx, actor, assetID, key)" in transfers
    payments = source("precompiles/layerxledger/payments.go")
    assert 'ledgerMethods["payExact"] = (*PrecompileExecutor).payExact' in payments
    assert "p.keeper.PayExact(ctx, actor, ledgerkeeper.PayExactArgs{" in payments
    keeper = source("modules/layerxledger/keeper/payexact.go")
    assert "now := ctx.BlockTime().UnixMilli()" in keeper
    assert "if args.ExpiresAt <= uint64(now)" in keeper
    assert "!actor.Authority.Bound || actor.DIDKey != actor.Authority.VerifiedKey" in keeper
    assert "k.sendSource(ctx, actor, args.From, args.Asset)" in keeper
    assert "k.Mirror(branch, activityType, result.Receipt)" in keeper


def test_f39_signed_execution_and_committed_proofs_remain_distinct():
    overview = section("What is Paxeer X Network?", level=2)
    evidence = section("Receipt and proof verification")
    assert "A receipt is final once its block is committed" in overview
    assert "`submitted`, `receipted` and `committed`" in overview
    assert "ProtocolReceipt v2" in evidence
    assert "separate from the validator consensus key" in evidence
    assert "pinned for the receipt height" in evidence
    assert "public key supplied beside an untrusted receipt does not establish trust" in evidence
    assert "`committed` rung requires coverage" in evidence
    assert "`MsgSequenceBatch`" in evidence
    assert "authenticate the block header and CometBFT commit" in evidence
    assert "trusted validator information" in evidence
    assert "authenticated root at the relevant height and key" in evidence
    rpc = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in rpc
    rungs = re.findall(r'Rung\w+ Rung = "(\w+)"', rpc)
    assert rungs == ["submitted", "receipted", "committed"]
    assert re.search(r"sequencerReceiptProtocolVersion\s+uint16 = 2", rpc)
    assert "covered by the MsgSequenceBatch of a committed block" in rpc
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


def test_f39_operator_build_uses_native_paxd_prerequisites():
    build = section("Build")
    assert "make paxd-build" in build
    assert "build/liblayerx.a" in build
    assert "programs/target/release/liblayerx_programs_sandbox.a" in build
    assert "running network entry point is `paxd`" in build
    assert not re.search(r"\bmake\b[^\n]*\blayerxd\b", README)
    makefile = source("Makefile")
    assert re.search(r"^paxd-native-libs: \$\(LIBRARY\) programs-release-build$", makefile, re.M)
    assert re.search(r"^paxd-build: paxd-native-libs$", makefile, re.M)
    recipe = makefile.split("\npaxd-build: paxd-native-libs\n", 1)[1].split("\n\n", 1)[0]
    assert "tools/release/version.json" in recipe
    assert "$(PAXEER_MAKE) build" in recipe
    release = section("Releases and Network Upgrades")
    assert "executes kernel activity inside `paxd`" in release
    assert "commits it with the BFT chain state" in release


def test_f39_rpc_example_targets_chain_125_at_node_root():
    documentation = section("Documentation", level=2)
    blocks = re.findall(r"```sh\n(.*?)\n```", documentation, re.S)
    assert len(blocks) == 1
    command = blocks[0].split("\n# Expected result:", 1)[0].replace("\\\n", " ")
    argv = shlex.split(command)
    assert argv[0] == "curl"
    assert argv[argv.index("--max-time") + 1] == "10"
    urls = [arg for arg in argv if arg.startswith("https://")]
    assert urls == ["https://api1.mainnet-beta.paxeer.network"]
    assert argv[argv.index("-H") + 1] == "Content-Type: application/json"
    assert json.loads(argv[argv.index("-d") + 1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in blocks[0]
    assert "Public node JSON-RPC uses the root URL" in documentation
    assert "api14.mainnet-beta.paxeer.network" in documentation
    assert "node.hyperpaxeer.com` is only the hpx library mirror" in documentation
    assert "gateway contract does not establish method availability at public node URLs" in documentation
    contract = json.loads(source("services/platform/hosted/gateway/openrpc.json"))
    assert "px_getBalances" in {method["name"] for method in contract["methods"]}
    assert not re.search(r"https?://[^\s`]+/rpc(?:[/\s`]|$)", README)


def test_f39_historical_formats_and_source_links_are_explicit():
    specification = section("EVM and Native Precompiles")
    assert "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired" in specification
    assert "`layerxAnchor` (`0x1014`) rejects writes after the merge" in specification
    assert "historical views retire at `v7.0-anchor-store-delete`" in specification
    assert "retained [batch-header codec]" in specification
    assert "archived settlement witness formats" in specification
    assert "support historical interpretation" in specification
    assert "guarantor attestations and the L0–L4 settlement ladder are retired" in specification
    assert "archived formats add no settlement step to current block finality" in specification
    upgrades = source("node/upgrades.go")
    assert 'const AnchorStoreDeleteUpgrade = "v7.0-anchor-store-delete"' in upgrades
    assert "func DecodeBatchHeader(" in source("layerxproof/codec/batch.go")
    for paragraph in README.split("\n\n"):
        if re.search(r"\blayerxd\b|\blayerxAnchor\b|guarantor|L0–L4", paragraph):
            assert re.search(r"retired|historical|archived", paragraph), paragraph
    for body in (section("Accounts and native funding"), section("Receipt and proof verification"), specification):
        links = re.findall(r"\]\(([^)]+)\)", body)
        assert links
        for target in links:
            assert not target.startswith(("http:", "https:"))
            assert (ROOT / target.split("#", 1)[0]).exists(), target


def test_f39_record_maps_finding_to_executed_proofs_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["9"]
    finding = record["finding"]["39"]
    assert finding["file"] == "README.md" and finding["audit"] == "22.30.1"
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert task["wiring_requests"] == []
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
