#[path = "support/repair_22_33_1529.rs"]
mod repair_support;
mod support;

use layerx_agentd::boot::Gate;
use layerx_agentd::config::{ReceiptKeySource, StartupConfig};
use layerx_agentd::human_runtime::lookup_human_receipt_by_outbox_hash;
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::ops::program::ProgramOperations;
use layerx_agentd::outbox::{Outbox, SubmissionState};
use layerx_agentd::store::Store;
use layerx_agentd::{finality::VerifiedReceipt, receipt};
use layerx_client::rpc::receipt::Lookup;
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::disclosure::call::decode_program_call;
use layerx_crypto::evm_transaction::{decode_signed, transaction_hash};
use layerx_types::program_call::NativeProgramCall;
use serde_json::{json, Value};
use std::collections::{BTreeMap, BTreeSet};
use std::os::unix::fs::PermissionsExt;
use std::time::Duration;

fn unhex(value: &Value) -> Vec<u8> {
    lint_success(
        hex::decode(lint_present(value.as_str(), "hex text").trim_start_matches("0x")),
        "hex bytes",
    )
}

fn recording() -> Value {
    let corpus: Value = lint_success(
        serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        )),
        "committed real sequencer recording",
    );
    assert_eq!(
        corpus["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    lint_present(
        lint_present(corpus["cases"].as_array(), "cases")
            .iter()
            .find(|case| case["id"] == "V1-layerxprograms-call"),
        "real program-call case",
    )
    .clone()
}

fn key(case: &Value) -> ReceiptKeyEntry {
    ReceiptKeyEntry {
        key_id: lint_success(unhex(&case["pinnedKey"]["keyId"]).try_into(), "key id"),
        public_key: lint_success(
            unhex(&case["pinnedKey"]["publicKey"]).try_into(),
            "public key",
        ),
        first_height: lint_present(case["pinnedKey"]["firstHeight"].as_u64(), "first height"),
        last_height: lint_present(case["pinnedKey"]["lastHeight"].as_u64(), "last height"),
    }
}

#[test]
fn protocol2_recovers_identical_signed_program_call_evidence() {
    let root = support::directory("protocol2-program-call-reopen");
    produce_program_evidence(&root); // Drop all transport, store, outbox and receipt objects before recovery.
    recover_program_evidence(&root);
    lint_success(std::fs::remove_dir_all(root), "remove store");
}

#[test]
fn protocol2_gate_reload_requires_fresh_readiness() {
    let root = support::directory("protocol2-gate-reload");
    lint_success(std::fs::create_dir_all(&root), "gate root");
    let key = ed25519_dalek::SigningKey::from_bytes(&[0x4c; 32])
        .verifying_key()
        .to_bytes();
    let path = root.join("authority.csv");
    lint_success(
        std::fs::write(
            &path,
            format!(
                "layerx-sequencer-authority-v1\n{},{},2,1,3,active\n",
                hex::encode(key),
                hex::encode(key)
            ),
        ),
        "protected policy",
    );
    lint_success(
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o600)),
        "policy permissions",
    );
    let config = StartupConfig {
        network_id: 42,
        node_endpoint: "http://127.0.0.1:0".into(),
        expected_chain_id: 42,
        tenants: BTreeSet::from([support::tenant()]),
        policy_sources: BTreeMap::new(),
        signer_configurations: BTreeMap::new(),
        verification_defaults: BTreeMap::new(),
        receipt_key_source: ReceiptKeySource::PinnedFile(path),
    };
    let mut gate = lint_success(Gate::new(&config), "load gate");
    for restarting in [false, true] {
        if restarting {
            gate.disconnected();
            drop(gate);
            gate = lint_success(Gate::new(&config), "reloaded gate");
        }
        assert!(gate.guard_write(|| ()).is_err());
        assert!(gate.evidence_authority().is_err());
        let (url, server) = repair_support::rpc_peer(vec![
            ("eth_chainId", json!("0x2a")),
            (
                "px_getReceiptKeys",
                json!({"keys":[{"keyId":format!("0x{}",hex::encode(key)),"publicKey":format!("0x{}",hex::encode(key)),"firstHeight":1,"lastHeight":3}]}),
            ),
        ]);
        let node = lint_success(
            NodeRpc::connect(&NodeRpcConfig {
                url,
                chain_id: 42,
                deadline: Duration::from_secs(3),
            }),
            "node readiness",
        );
        let ready = lint_success(gate.readiness_gate(&node), "fresh readiness");
        assert_eq!(ready.chain_id, 42);
        assert_eq!(ready.receipt_keys[0].public_key, key);
        assert_eq!(gate.guard_write(|| ()), Ok(()));
        assert!(gate.evidence_authority().is_ok());
        lint_success(server.join(), "readiness transcript");
    }
    gate.disconnected();
    assert!(gate.evidence_authority().is_err());
    assert!(gate.guard_write(|| ()).is_err());
    lint_success(std::fs::remove_dir_all(root), "remove gate policy");
}

#[track_caller]
fn lint_success<T, E: std::fmt::Debug>(result: Result<T, E>, context: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{context}: {error:?}"),
    }
}

#[track_caller]
fn lint_present<T>(value: Option<T>, context: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{context}: required value is absent"),
    }
}

fn produce_program_evidence(root: &std::path::Path) {
    let case = recording();
    let raw = unhex(&case["rawTransaction"]);
    let (call, _) = lint_success(decode_signed(&raw), "real type-2 call");
    assert_eq!(
        call.to,
        layerx_agentd::prepare::PrecompileTarget::Programs.address()
    );
    let (program, fee) = lint_success(decode_program_call(&call.data), "real Programs ABI");
    let produced = lint_success(
        ProgramOperations::call_precompile(
            NativeProgramCall {
                program_id: program.program_id,
                guest_abi: program.guest_abi,
                entrypoint: &program.entrypoint,
                calldata: &program.calldata,
                capabilities: &program.capabilities,
                access_declaration: &program.access_declaration,
                response_capacity: program.response_capacity,
                resources: program.resources,
            },
            fee,
        ),
        "production program-call producer",
    );
    assert_eq!(lint_success(produced.encode(), "ABI encoding"), call.data);
    let hash = transaction_hash(&raw);
    let mut store = lint_success(Store::open(root), "store");
    let mut outbox = Outbox::default();
    assert_eq!(
        lint_success(
            outbox.enqueue(&mut store, support::tenant(), raw),
            "enqueue"
        ),
        hash
    );
    lint_success(
        outbox.transition(
            &mut store,
            hash,
            SubmissionState::Submitted,
            "sent exact program call",
            None,
        ),
        "submit",
    );
    let (url, server) = repair_support::rpc_peer(vec![
        ("eth_chainId", json!("0x7d")),
        (
            "px_getReceiptKeys",
            json!({"keys":[case["pinnedKey"].clone()]}),
        ),
        ("pax_sequencerReceipt", case["receiptJson"].clone()),
        ("pax_sequencerReceipt", case["receiptJson"].clone()),
        ("eth_getTransactionReceipt", Value::Null),
    ]);
    let node = lint_success(
        NodeRpc::connect(&NodeRpcConfig {
            url: url.clone(),
            chain_id: 125,
            deadline: Duration::from_secs(3),
        }),
        "node",
    );
    let endpoint = lint_success(SequencerEndpoint::new(&url, &[]), "endpoint");
    let (lookup, keys) = lint_success(
        lookup_human_receipt_by_outbox_hash(&node, &endpoint, &outbox, hash, hash),
        "real receipt transport",
    );
    assert_eq!(keys, vec![key(&case)]);
    let Lookup::Verified(verified) = lookup else {
        panic!("verified program outcome required");
    };
    assert_eq!(verified.activity_id(), hash);
    assert_eq!(verified.result_code().raw(), 0);
    assert!(verified.execution().success);
    assert_eq!(verified.execution().logs.len(), 1);
    assert_eq!(verified.execution().logs[0].address, call.to);
    assert_eq!(verified.canonical_bytes(), unhex(&case["signedReceipt"]));
    lint_success(
        receipt::store_verified_if_absent(
            &mut store,
            support::tenant(),
            hash,
            verified.canonical_bytes(),
            &verified,
        ),
        "persist signed receipt/outcome",
    );
    let requests = lint_success(server.join(), "receipt transcript");
    for request in &requests[2..] {
        assert_eq!(
            request["params"],
            json!([format!("0x{}", hex::encode(hash))])
        );
    }
}

fn recover_program_evidence(root: &std::path::Path) {
    let case = recording(); // Reload only committed producer inputs, not process objects.
    let raw = unhex(&case["rawTransaction"]);
    let hash = transaction_hash(&raw);
    let store = lint_success(Store::open(root), "reopen consumer");
    let mut outbox = Outbox::default();
    lint_success(
        outbox.restore(&store, support::tenant(), hash),
        "restore outbox",
    );
    assert_eq!(
        lint_success(outbox.exact_signed_bytes(hash), "exact signed call"),
        raw
    );
    assert_eq!(
        lint_present(outbox.status(hash), "restored transport state").state,
        SubmissionState::Submitted
    );
    let restored = lint_present(
        lint_success(
            receipt::serve_evidence(&store, support::tenant(), hash),
            "durable evidence",
        ),
        "receipt present",
    );
    assert_eq!(restored.canonical_receipt, unhex(&case["signedReceipt"]));
    assert_eq!(restored.metadata.transaction_hash, hash);
    assert!(restored.metadata.status);
    let decoded = lint_success(
        layerx_wire::receipt::decode(&restored.canonical_receipt),
        "receipt codec",
    );
    assert_eq!(
        lint_present(decoded.protocol(), "protocol receipt").protocol_version(),
        2
    );
    let verified = lint_success(
        VerifiedReceipt::verify(hash, &restored.canonical_receipt, &[key(&case)]),
        "reverify recovered exact signed outcome",
    );
    assert_eq!(verified.canonical_bytes(), restored.canonical_receipt);
    let mut corrupted = restored.canonical_receipt;
    let last = corrupted.len() - 1;
    corrupted[last] ^= 1;
    assert!(VerifiedReceipt::verify(hash, &corrupted, &[key(&case)]).is_err());
}
