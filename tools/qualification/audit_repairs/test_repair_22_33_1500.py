"""Offline install-guide contracts against the maintained production sources.

This documentation gate parses the real tool schemas and documented arguments,
and checks the signer -> transport -> receipt source path. It does not execute
Rust, cryptography, a runtime installation or a live transaction.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
INSTALL_PATH = "docs/platform/content/install.md"
CLI = "tools/platform/cli/"
RECORD = Path(__file__).with_name("repair_22_33_1500.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def prose():
    return re.sub(r"\s+", " ", source(INSTALL_PATH))


def assert_facts(text, *facts):
    for fact in facts:
        assert fact in text, fact


def tool_schema(name):
    """Parse the literal JSON macro arm from the production A2A catalogue."""
    schemas = source(CLI + "src/toolset.rs").split("pub fn schema(", 1)[1]
    schemas = schemas.split("pub fn invoke(", 1)[0]
    match = re.search(
        re.escape(json.dumps(name)) + r" => json!\((\{.*?\n        \})\),",
        schemas,
        re.S,
    )
    assert match, name
    literal = re.sub(r"(?<=\d)_(?=\d)", "", match[1])
    literal = re.sub(r",\s*([}\]])", r"\1", literal)
    return json.loads(literal)


def documented_arguments(key):
    matches = re.findall(r"`(\{" + re.escape(json.dumps(key)) + r":.*?\})`", source(INSTALL_PATH))
    assert len(matches) == 1, key
    return json.loads(matches[0])


def test_install_guide_inputs_maintained_cli_link():
    links = re.findall(r"\[CLI wallet quickstart\]\(([^)]+)\)", source(INSTALL_PATH))
    assert len(links) == 1
    url = urlsplit(links[0])
    assert url.scheme == "https" and url.netloc == "github.com"
    prefix = "/Sidiora-Labs/paxeer-monorepo/blob/main/"
    assert url.path == prefix + CLI + "README.md"
    assert url.fragment == "wallet-quickstart" and not url.query
    guide = source(url.path.removeprefix(prefix))
    assert "## Wallet quickstart\n" in guide
    assert_facts(guide, "--evm-key-file", "--receipt-policy", "--fee-limit", "EIP-1559")


def test_install_guide_inputs_signing_and_disclosure():
    assert_facts(
        prose(),
        "Ed25519 seed", "secp256k1 key export", "`--evm-key-file`",
        "independently trusted `--receipt-policy`", "`--fee-limit` in integer base units",
        "chain-125 EIP-1559 transactions", "source account's `next_sequence`",
        "Disclosures are written to stderr before signing",
        "sender, recipient, asset, amount, calldata, chain ID, nonce, gas limit and fee caps",
        "The EVM nonce and native source-account sequence are distinct",
        "Before signing for A2A, review the same payment and EIP-1559 disclosures",
        "does not construct the payment or sign it from the installed CLI key",
    )
    signing = source(CLI + "src/wallet_signing.rs")
    assert_facts(
        signing, "if call.chain_id != 125", "call.gas_limit == 0",
        "call.max_priority_fee_per_gas > call.max_fee_per_gas",
        "bind_call(&self.call, self.from).map_err(debug)? != self.disclosure",
        "recover_sender(&decoded, &signature).map_err(debug)? != self.from",
        '"chain_id":self.call.chain_id', '"nonce":self.call.nonce.to_string()',
        '"max_fee_per_gas"', '"max_priority_fee_per_gas"', '"data"',
        "export does not match the selected wallet identity",
    )
    wallet = source(CLI + "src/wallet.rs")
    ordered = [
        "let prepared = PreparedTransaction::new(call, from)?;",
        "print_disclosure(&prepared.confirmation())?;",
        "let (raw, expected_hash) = prepared.sign(&evm_key)?;",
        "let hash = submit_transaction(",
        "reconcile_submission(",
    ]
    send = wallet[wallet.index("let prepared = PreparedTransaction::new(call, from)?;"):]
    positions = [send.index(fact) for fact in ordered]
    journal = wallet.split("pub(crate) fn submit_transaction(", 1)[1].split("pub(crate) fn reconcile_submission(", 1)[0]
    assert journal.index("save_submission(journal, &record)?") < journal.index("client.send_raw_transaction(raw)")
    assert 'Ok(hash) if hash == expected_hash' in journal
    assert 'state: "unknown".into()' in journal
    assert positions == sorted(positions)
    debit = wallet.split("let debit =\n", 1)[1].split("fn identity_sequence(", 1)[0]
    assert debit.index("print_disclosure(") < debit.index("debit.sign(signer)")
    assert 'std::io::stderr().lock()' in wallet
    assert 'snapshot["next_sequence"]' in wallet


def test_install_guide_inputs_raw_transaction_producer_transport():
    schema = tool_schema("activity.submit")
    arguments = documented_arguments("raw_transaction")
    assert schema["type"] == "object"
    assert schema["required"] == list(arguments) == ["raw_transaction"]
    assert schema["additionalProperties"] is False
    assert set(schema["properties"]) == {"raw_transaction"}
    raw_schema = schema["properties"]["raw_transaction"]
    assert raw_schema["type"] == "string"
    assert raw_schema["pattern"] == "^0x02([0-9a-f]{2})+$"
    assert raw_schema["maxLength"] == 1_048_578
    assert arguments == {"raw_transaction": "0x02..."}
    assert "replace the ellipsis with complete signed transaction bytes" in prose()
    assert "already signed EIP-1559 transaction" in prose()
    toolset = source(CLI + "src/toolset.rs")
    submit = toolset.split("fn submit_classified(", 1)[1].split("fn submission_result(", 1)[0]
    assert_facts(
        submit, ".filter(|object| object.len() == 1)", '.get("raw_transaction")',
        'rpc::request("eth_sendRawTransaction", &json!([raw]))',
        "rpc::prepare_submission(&bytes, Some(u64::from(runtime.network_id)))?",
        '.post("/", prepared.request(), None)', "prepared.resolve(response)",
    )
    assert submit.index("rpc::prepare_submission(") < submit.index('.post("/", prepared.request(), None)')
    rpc = source(CLI + "src/rpc.rs")
    validation = rpc.split("pub fn prepare_submission(", 1)[1].split("pub fn submission_outcome(", 1)[0]
    assert_facts(validation, "decode_signed(raw)", "recover_sender(&call, &signature)",
                 "network_id.is_some_and(|id| id != call.chain_id)",
                 "transaction_hash(raw)", "Ok(PreparedSubmission { command, tx_hash })")



def test_install_guide_inputs_txhash_reconciliation_consumer():
    assert_facts(
        prose(),
        "checks that the node's returned hash matches the locally computed hash",
        "returns `txHash` with `submitted` rung",
        "without proving execution or commitment",
        "return `txHash` with `unknown` rung",
        "Keep that hash and reconcile it before retrying",
        "Set `activity_id` to the same `txHash`",
        "`transaction_pending`", "`wallet receipt`",
        "fresh send invocation generates a new native idempotency key",
        "independently pinned keys with height ranges",
        "fetched bytes alone are not a verification or finality claim",
    )
    toolset = source(CLI + "src/toolset.rs")
    assert_facts(
        toolset,
        'let expected = format!("0x{}", hex_encode(&transaction_hash(raw)))',
        "if hash.as_str() != Some(expected.as_str())",
        'Ok(json!({"txHash": hash, "rung": "submitted"}))',
        'let hash = text(arguments, "activity_id")?;',
        'fixed_hex::<32>("transaction hash", digits)?;',
        'rpc::request("pax_sequencerReceipt", &json!([hash]))',
        'rpc::decode_response("pax_sequencerReceipt", &response)',
    )
    rpc = source(CLI + "src/rpc.rs")
    assert_facts(rpc, 'Ok(json!({"txHash":tx_hash,"rung":"unknown","detail":detail}))',
                 "if hash.as_str() != Some(tx_hash.as_str())",
                 'Ok(json!({"txHash":hash,"rung":"submitted"}))')
    schema = tool_schema("receipt.get")
    arguments = documented_arguments("activity_id")
    assert schema["required"] == list(arguments) == ["activity_id"]
    assert set(schema["properties"]) == {"activity_id"}
    assert schema["additionalProperties"] is False
    assert schema["properties"]["activity_id"]["pattern"] == "^0x[0-9a-fA-F]{64}$"
    assert arguments == {"activity_id": "0x<64 hex digits>"}
    wallet = source(CLI + "src/wallet.rs")
    assert 'transaction_pending: {hash_text}' in wallet
    assert 'getrandom::fill(&mut idempotency_key)' in wallet


def test_install_guide_inputs_preserves_bootstrap_and_mcp_requirements():
    install = source(INSTALL_PATH)
    bootstrap = install.split("<!-- layerx:bootstrap-sequence -->\n```bash\n", 1)[1].split("```", 1)[0]
    commands = bootstrap.replace("\\\n", "").splitlines()
    assert len(commands) == 3
    assert shlex.split(commands[0]) == ["layerx", "emulator", "provision"]
    assert shlex.split(commands[1]) == [
        "layerx", "emulator", "up", "--sequencer-seed-file",
        "$HOME/.config/layerx/emulator/sequencer.seed", "&",
    ]
    assert shlex.split(commands[2]) == [
        "layerx", "environment", "use", "emulator", "--endpoint",
        "http://127.0.0.1:9402", "--network-id", "402",
        "--sequencer-trust-anchor-file", "$HOME/.config/layerx/emulator/sequencer.anchor",
    ]
    assert_facts(
        prose(), "`core_height`", "`core_sequence` field is refused",
        "`mcp.binding_retired_key`", "`agent.readiness`",
        "authenticated HTTPS gateway and its protected credential file",
        "`--daemon-binding <path>`", "`--read-only`",
    )
    emulator = source(CLI + "src/emulator.rs")
    assert_facts(emulator, "pub fn provision(", '"sequencer_seed_file"', '"sequencer_trust_anchor_file"')
    binding = source("services/agent/crates/layerx-mcp/src/binding.rs")
    assert_facts(
        binding, 'if root.contains_key("core_sequence")', '"mcp.binding_retired_key"',
        'core_height: unsigned(root, "core_height")?', '"agent.readiness.gateway_key_file"',
        '"readiness HTTPS endpoint or trust anchors are invalid"',
    )
    installer = source(CLI + "src/install/mcp.rs")
    assert_facts(installer, "open_binding(&daemon_binding)", "InstalledSession::open(&binding, request.read_only)")


def test_install_guide_inputs_record_maps_finding_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["install_guide_inputs"]
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.89.kvx"))
    original = audit["finding"]["install_guide_inputs"]
    assert finding["audit"] == "22.30.89"
    for field in ("file", "class", "severity", "expected"):
        assert finding[field] == original[field]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["1500"]
    assert task["wiring_requests"] == []
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1500.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["proof_limits"]
    assert "same candidate revision" in task["evidence"]
