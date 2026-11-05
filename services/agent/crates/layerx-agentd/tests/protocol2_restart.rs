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
    hex::decode(value.as_str().expect("hex text").trim_start_matches("0x")).expect("hex bytes")
}

fn recording() -> Value {
    let corpus: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .expect("committed real sequencer recording");
    assert_eq!(
        corpus["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    corpus["cases"]
        .as_array()
        .expect("cases")
        .iter()
        .find(|case| case["id"] == "V1-layerxprograms-call")
        .expect("real program-call case")
        .clone()
}

fn key(case: &Value) -> ReceiptKeyEntry {
    ReceiptKeyEntry {
        key_id: unhex(&case["pinnedKey"]["keyId"])
            .try_into()
            .expect("key id"),
        public_key: unhex(&case["pinnedKey"]["publicKey"])
            .try_into()
            .expect("public key"),
        first_height: case["pinnedKey"]["firstHeight"]
            .as_u64()
            .expect("first height"),
        last_height: case["pinnedKey"]["lastHeight"]
            .as_u64()
            .expect("last height"),
    }
}

#[test]
fn protocol2_recovers_identical_signed_program_call_evidence() {
    let root = support::directory("protocol2-program-call-reopen");
    {
        let case = recording();
        let raw = unhex(&case["rawTransaction"]);
        let (call, _) = decode_signed(&raw).expect("real type-2 call");
        assert_eq!(
            call.to,
            layerx_agentd::prepare::PrecompileTarget::Programs.address()
        );
        let (program, fee) = decode_program_call(&call.data).expect("real Programs ABI");
        let produced = ProgramOperations::call_precompile(
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
        )
        .expect("production program-call producer");
        assert_eq!(produced.encode().expect("ABI encoding"), call.data);
        let hash = transaction_hash(&raw);
        let mut store = Store::open(&root).expect("store");
        let mut outbox = Outbox::default();
        assert_eq!(
            outbox
                .enqueue(&mut store, support::tenant(), raw)
                .expect("enqueue"),
            hash
        );
        outbox
            .transition(
                &mut store,
                hash,
                SubmissionState::Submitted,
                "sent exact program call",
                None,
            )
            .expect("submit");
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
        let node = NodeRpc::connect(&NodeRpcConfig {
            url: url.clone(),
            chain_id: 125,
            deadline: Duration::from_secs(3),
        })
        .expect("node");
        let endpoint = SequencerEndpoint::new(&url, &[]).expect("endpoint");
        let (lookup, keys) =
            lookup_human_receipt_by_outbox_hash(&node, &endpoint, &outbox, hash, hash)
                .expect("real receipt transport");
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
        receipt::store_verified_if_absent(
            &mut store,
            support::tenant(),
            hash,
            verified.canonical_bytes(),
            &verified,
        )
        .expect("persist signed receipt/outcome");
        let requests = server.join().expect("receipt transcript");
        for request in &requests[2..] {
            assert_eq!(
                request["params"],
                json!([format!("0x{}", hex::encode(hash))])
            );
        }
    } // Drop all transport, store, outbox and receipt objects before recovery.
    {
        let case = recording(); // Reload only committed producer inputs, not process objects.
        let raw = unhex(&case["rawTransaction"]);
        let hash = transaction_hash(&raw);
        let store = Store::open(&root).expect("reopen consumer");
        let mut outbox = Outbox::default();
        outbox
            .restore(&store, support::tenant(), hash)
            .expect("restore outbox");
        assert_eq!(
            outbox.exact_signed_bytes(hash).expect("exact signed call"),
            raw
        );
        assert_eq!(
            outbox.status(hash).expect("restored transport state").state,
            SubmissionState::Submitted
        );
        let restored = receipt::serve_evidence(&store, support::tenant(), hash)
            .expect("durable evidence")
            .expect("receipt present");
        assert_eq!(restored.canonical_receipt, unhex(&case["signedReceipt"]));
        assert_eq!(restored.metadata.transaction_hash, hash);
        assert!(restored.metadata.status);
        let decoded =
            layerx_wire::receipt::decode(&restored.canonical_receipt).expect("receipt codec");
        assert_eq!(
            decoded
                .protocol()
                .expect("protocol receipt")
                .protocol_version(),
            2
        );
        let verified = VerifiedReceipt::verify(hash, &restored.canonical_receipt, &[key(&case)])
            .expect("reverify recovered exact signed outcome");
        assert_eq!(verified.canonical_bytes(), restored.canonical_receipt);
        let mut corrupted = restored.canonical_receipt;
        let last = corrupted.len() - 1;
        corrupted[last] ^= 1;
        assert!(VerifiedReceipt::verify(hash, &corrupted, &[key(&case)]).is_err());
    }
    std::fs::remove_dir_all(root).expect("remove store");
}

#[test]
fn protocol2_gate_reload_requires_fresh_readiness() {
    let root = support::directory("protocol2-gate-reload");
    std::fs::create_dir_all(&root).expect("gate root");
    let key = ed25519_dalek::SigningKey::from_bytes(&[0x4c; 32])
        .verifying_key()
        .to_bytes();
    let path = root.join("authority.csv");
    std::fs::write(
        &path,
        format!(
            "layerx-sequencer-authority-v1\n{},{},2,1,3,active\n",
            hex::encode(key),
            hex::encode(key)
        ),
    )
    .expect("protected policy");
    std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o600))
        .expect("policy permissions");
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
    let mut gate = Gate::new(&config).expect("load gate");
    for restarting in [false, true] {
        if restarting {
            gate.disconnected();
            drop(gate);
            gate = Gate::new(&config).expect("reloaded gate");
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
        let node = NodeRpc::connect(&NodeRpcConfig {
            url,
            chain_id: 42,
            deadline: Duration::from_secs(3),
        })
        .expect("node readiness");
        let ready = gate.readiness_gate(&node).expect("fresh readiness");
        assert_eq!(ready.chain_id, 42);
        assert_eq!(ready.receipt_keys[0].public_key, key);
        assert_eq!(gate.guard_write(|| ()), Ok(()));
        assert!(gate.evidence_authority().is_ok());
        server.join().expect("readiness transcript");
    }
    gate.disconnected();
    assert!(gate.evidence_authority().is_err());
    assert!(gate.guard_write(|| ()).is_err());
    std::fs::remove_dir_all(root).expect("remove gate policy");
}
