"""Finding 61: check the production README consumed by network developers.

These are documentation/source proofs, not node execution or deployment tests.
The README owner is task 22.31.470; this consumer never rewrites that file.
"""

import json
from pathlib import Path
import re
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
README = (ROOT / "README.md").read_text(encoding="utf-8")


def section(title):
    heading = re.search(rf"^(#{{2,3}}) {re.escape(title)}$", README, re.M)
    assert heading is not None, f"README.md missing section: {title}"
    remainder = README[heading.end():]
    following = re.search(rf"^#{{2,{len(heading[1])}}} ", remainder, re.M)
    return remainder[:following.start()] if following else remainder


def test_finding_61_one_paxd_network_and_tenant_namespaces():
    overview = section("What is Paxeer X Network?")
    assert "one BFT network with EVM chain ID **125** (`0x7d`)" in overview
    assert "kernel is embedded in the node" in overview
    assert "proposes blocks inside `paxd`" in overview
    assert "validators commit those blocks through BFT consensus" in overview
    assert "no separate kernel daemon, sequencer service or checkpoint deployment" in overview
    assert "Sub-chains are tenant domains stored in the database" in overview
    assert "namespaces state within the shared network" in overview
    assert "does not require new node infrastructure" in overview
    assert "modules/layerxchains" in overview
    assert (ROOT / "modules/layerxchains").is_dir()


def test_finding_61_public_root_hosts_and_chain_probe():
    quickstart = section("Quick start: connect to chain 125")
    urls = re.findall(r"https://api[^\s`\\]+", quickstart)
    assert set(urls) == {
        "https://api-mainnet-beta.paxeer.network",
        "https://api1.mainnet-beta.paxeer.network",
        "https://api14.mainnet-beta.paxeer.network",
    }
    assert (
        "`https://api1.mainnet-beta.paxeer.network` through "
        "`https://api14.mainnet-beta.paxeer.network`"
    ) in quickstart
    for url in urls:
        parsed = urlsplit(url)
        assert parsed.scheme == "https"
        assert parsed.path in ("", "/")
        assert not parsed.query and not parsed.fragment
    assert "root path of these hosts, without a route suffix" in quickstart
    payload = re.search(r"-d '([^']+)'", quickstart)
    assert payload is not None
    assert json.loads(payload[1]) == {
        "jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []
    }
    assert '# Expected result: "0x7d"' in quickstart
    assert "compare `eth_blockNumber`" in quickstart
    assert "never a JSON-RPC endpoint" in quickstart


def test_finding_61_hosted_methods_have_an_explicit_server_contract():
    quickstart = section("Quick start: connect to chain 125")
    contract_path = "services/platform/hosted/gateway/openrpc.json"
    assert "hosted gateway's `px_` API" in quickstart
    assert contract_path in quickstart
    assert "deployment URL and credentials from the operator" in quickstart
    assert "does not establish method availability at public node URLs" in quickstart
    assert "`px_getReceiptKeys` is registered by the node" in quickstart
    assert "`px_register` and `px_requestFunds` are operations of the hosted identity and faucet services" in quickstart
    contract = json.loads((ROOT / contract_path).read_text(encoding="utf-8"))
    methods = {method["name"] for method in contract["methods"]}
    assert {"px_getReceiptKeys", "px_getAccount", "px_getBalances"} <= methods
    registrations = (ROOT / "rpc/server.go").read_text(encoding="utf-8")
    for service in ("pxAccountAPI", "pxHistoryAPI", "receiptKeysAPI"):
        assert f"&{service}" in registrations
    topology = section("Integrated network topology")
    assert "hosted gateway is optional developer tooling" in topology
    assert "outside consensus" in topology


def test_finding_61_transaction_to_committed_receipt_flow():
    flow = section("Submit a transaction and follow its committed receipt")
    steps = re.findall(r"^\d+\. (.+)$", flow, re.M)
    assert len(steps) == 4
    assert "EIP-1559 transaction for chain ID `125`" in steps[0]
    assert "`0x101A`–`0x101D`" in steps[0]
    assert "`eth_sendRawTransaction`" in steps[1]
    assert "root JSON-RPC URL" in steps[1]
    assert "result is a transaction hash" in steps[1]
    assert "durable WAL acknowledgement" in steps[1]
    assert "Receipt order is proposal order" in steps[1]
    assert "`pax_sequencerReceipt` with the transaction hash" in steps[2]
    assert "`submitted` has no execution receipt yet" in steps[2]
    assert "`receipted` includes signed execution evidence" in steps[2]
    assert "`committed` additionally names the block" in steps[2]
    assert "`MsgSequenceBatch` covers that sequence" in steps[2]
    assert "Wait for `committed` and authenticate the block commitment" in steps[3]
    assert "`eth_getTransactionReceipt`" in steps[3]
    assert "inspect its status and logs" in steps[3]
    assert "commitment does not mean execution succeeded" in steps[3]
    assert "`eth_call` at an explicit committed height" in steps[3]
    for source in ("rpc/send.go", "node/sequencer/accepter.go"):
        assert source in flow
        assert (ROOT / source).is_file()


def test_finding_61_native_committed_block_qualification():
    overview = section("What is Paxeer X Network?")
    assert "A receipt is final once its block is committed" in overview
    proof = section("Receipt and proof verification")
    assert "signed ProtocolReceipt v2 bytes" in proof
    assert "must be pinned for the receipt height" in proof
    assert "Verify canonical receipt bytes and their signature" in proof
    assert "`committed` rung requires coverage by the committed block's `MsgSequenceBatch`" in proof
    assert "authenticate the block header and CometBFT commit" in proof
    assert "trusted validator information" in proof
    assert "verify transaction or state inclusion against the authenticated root" in proof
    assert "Neither a transaction hash nor a signed execution receipt alone" in proof
    for source in (
        "node/sequencer/receipt.go", "node/sequencer/receiptkey.go",
        "layerxproof/verify/receipt.go",
        "programs/crates/layerx-programs-registry/src/chain_proof/",
    ):
        assert source in proof
        assert (ROOT / source).exists()


def test_finding_61_retired_custody_and_checkpoint_instructions_removed():
    assert re.search(r"\blayerxd\b", README, re.I) is None
    assert "`getLayerXDid(address)`" in section("Accounts and native funding")
    shell_blocks = "\n".join(re.findall(r"```sh\n(.*?)```", README, re.S))
    assert not re.search(
        r"\b(?:daemon-build|daemon-start|layerx-daemon|checkpoint-build|checkpoint-start)\b",
        shell_blocks,
    )
    funding = section("Accounts and native funding")
    assert "custody deposit and credit-attestation funding flow is retired" in funding
    interfaces = section("EVM and Native Precompiles")
    assert "`layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`)" in interfaces
    assert "interfaces are retired at the merge" in interfaces
    assert "`layerxAnchor` (`0x1014`) rejects writes after the merge" in interfaces
    assert "not active funding or checkpoint executors" in interfaces
    assert "`LXP_MODULE_DISABLED`" in interfaces
    assert "archive-only" in interfaces
    assert "External checkpoint registration, guarantor attestations and the L0–L4 settlement ladder are retired" in interfaces
    assert "archived formats add no settlement step to current block finality" in interfaces


def test_finding_61_current_executable_and_workspace_paths():
    build = section("Build")
    assert "make paxd-build" in build
    assert "./build/paxd start --home" in build
    assert "./build/paxd version" in build
    assert "build/liblayerx.a" in build
    assert "programs/target/release/liblayerx_programs_sandbox.a" in build
    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
    assert re.search(r"^paxd-build:.*\bpaxd-native-libs\b", makefile, re.M)
    for target in ("agent-build", "interop-build", "human-build", "programs-build"):
        assert f"make {target}" in build
        assert re.search(rf"^{target}:", makefile, re.M)
    assert "make platform-build" in build
    layout = section("Directory Structure")
    for path in (
        "services/agent", "services/interop", "services/platform", "services/human",
        "services/wallet", "services/explorer", "services/launchpad", "sdk/platform",
    ):
        assert path in layout
        assert (ROOT / path).is_dir()
    for path in ("daemon/paxd", "modules/lxphost", "node/sequencer"):
        assert path in section("What is Paxeer X Network?")
        assert (ROOT / path).is_dir()
