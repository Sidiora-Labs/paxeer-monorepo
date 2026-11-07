#[path = "export.rs"]
mod export_fixture;

use std::sync::Mutex;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use export_fixture::{canonical, must, storage_node, storage_vector, Fixture};
use layerx_agent_api::export::FactRef;
use layerx_agentd::export::{
    decode_storage_witnesses, deserialize_export, encode_commit, encode_header,
    encode_storage_witnesses, produce, retain_bundle, serialize_export, verify, ExportOwnerContext,
    ExportProduceError, OfflineExport,
};
use layerx_agentd::human_runtime::retain_state_export;
use layerx_agentd::protocol_evidence::EvidenceAuthority;
use layerx_agentd::store::{Store, TenantId};
use layerx_crypto::evm_transaction::transaction_hash;
use layerx_types::payload::ModuleRegistry;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};

fn state_fact(fixture: &Fixture) -> FactRef {
    must(FactRef::new(format!(
        "state:{}:{}",
        hex::encode(transaction_hash(
            fixture.artifact.bundles[0].raw_transaction.as_bytes()
        )),
        hex::encode(fixture.account_id)
    )))
}

fn with_produced_state(run: impl FnOnce(&Fixture, &OfflineExport)) {
    let fixture = Fixture::new(8, true);
    let vector = storage_vector();
    let slot: [u8; 32] = must(hex::decode(vector["slot"].as_str().unwrap()))
        .try_into()
        .unwrap();
    let block = json!({
        "number":"0x8", "hash":format!("0x{}",hex::encode(fixture.header.block_hash())),
        "parentHash":format!("0x{}",hex::encode([0;32])),
        "timestamp":format!("{:#x}",fixture.header.time.seconds),
        "transactionsRoot":format!("0x{}",hex::encode([1;32])),
        "receiptsRoot":format!("0x{}",hex::encode(fixture.header.last_results_hash.unwrap())),
        "stateRoot":format!("0x{}",vector["app_hash"].as_str().unwrap()),
        "transactions":[],
    });
    let (endpoint, server) = storage_node(vec![
        (
            "eth_getProof",
            json!([
                vector["response"]["address"],
                [format!("0x{}", hex::encode(slot))],
                "0x8"
            ]),
            vector["response"].clone(),
        ),
        ("eth_getBlockByNumber", json!(["0x8", true]), block.clone()),
        ("eth_getBlockByNumber", json!(["0x8", false]), block),
    ]);
    let directory = std::env::temp_dir().join(format!(
        "repair-22-31-561-{}-{}",
        std::process::id(),
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_nanos()
    ));
    let tenant = must(TenantId::new("state-export"));
    let registry = must(ModuleRegistry::new(&[]));
    let authority = must(EvidenceAuthority::pinned_to_handshake(
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
        10,
        fixture.receipt_key.public_key,
    ));
    let store = Mutex::new(must(Store::open(&directory)));
    must(retain_bundle(
        &mut store.lock().unwrap(),
        tenant.clone(),
        fixture.artifact.clone(),
        &fixture.trust,
    ));
    let context = ExportOwnerContext {
        store: &store,
        tenant: &tenant,
        bound_account: Some(fixture.account_id),
        registry: &registry,
        authority: &authority,
        trust: &fixture.trust,
        first_correlation: 1,
        availability_deadline: Duration::from_secs(1),
    };
    assert!(matches!(
        produce(
            &context,
            &[state_fact(&fixture)],
            VerificationLevel::COMMITTED
        ),
        Err(ExportProduceError::NotOwned { fact: 0 })
    ));
    must(retain_state_export(
        &context,
        &endpoint,
        transaction_hash(fixture.artifact.bundles[0].raw_transaction.as_bytes()),
        &[slot],
    ));
    server.join().unwrap();
    let produced = must(produce(
        &context,
        &[state_fact(&fixture)],
        VerificationLevel::COMMITTED,
    ));
    assert_eq!(produced.response.achieved_rung, Rung::Committed);
    assert_eq!(produced.response.value.facts, [state_fact(&fixture)]);
    let proof = produced.response.value.bundles[0]
        .account_proof
        .as_ref()
        .unwrap();
    assert_eq!(proof.as_bytes(), fixture.account_proof);
    let witnesses = must(decode_storage_witnesses(proof.as_bytes()));
    assert_eq!(witnesses.len(), 1);
    assert_eq!(witnesses[0].slot, slot);
    assert_eq!(hex::encode(witnesses[0].value), vector["value"]);
    let bytes = must(serialize_export(&produced.response.value));
    let transported = must(deserialize_export(&bytes));
    assert_eq!(
        must(verify(&transported, &fixture.trust)).verified_inclusions,
        1
    );
    drop(context);
    drop(store);
    let recovered_store = Mutex::new(must(Store::open(&directory)));
    let context = ExportOwnerContext {
        store: &recovered_store,
        tenant: &tenant,
        bound_account: Some(fixture.account_id),
        registry: &registry,
        authority: &authority,
        trust: &fixture.trust,
        first_correlation: 2,
        availability_deadline: Duration::from_secs(1),
    };
    let recovered = must(produce(
        &context,
        &[state_fact(&fixture)],
        VerificationLevel::COMMITTED,
    ));
    assert_eq!(must(serialize_export(&recovered.response.value)), bytes);
    run(&fixture, &transported);
    drop(context);
    drop(recovered_store);
    must(std::fs::remove_dir_all(directory));
}

#[test]
fn producer_transport_and_offline_consumer_preserve_ics23_storage_evidence() {
    with_produced_state(|fixture, artifact| {
        let verified = must(verify(artifact, &fixture.trust));
        assert_eq!(verified.height, 8);
        assert_eq!(verified.achieved_rung, Rung::Committed);
        assert_eq!(verified.block_hash, fixture.header.block_hash());
    });
}

#[test]
fn offline_consumer_rejects_tampered_operations_app_hash_account_and_slot() {
    with_produced_state(|fixture, artifact| {
        let original = must(decode_storage_witnesses(
            artifact.bundles[0]
                .account_proof
                .as_ref()
                .unwrap()
                .as_bytes(),
        ));
        for mutation in 0..6 {
            let mut witnesses = original.clone();
            match mutation {
                0 => witnesses[0].ops[0].data[10] ^= 1,
                1 => witnesses[0].ops[1].data[10] ^= 1,
                2 => witnesses[0].address[0] ^= 1,
                3 => witnesses[0].slot[0] ^= 1,
                4 => witnesses[0].value[0] ^= 1,
                _ => witnesses[0].ops[0].op_type.push('x'),
            }
            let mut altered = artifact.clone();
            altered.bundles[0].account_proof = Some(must(encode_storage_witnesses(&witnesses)));
            assert!(
                matches!(
                    verify(&altered, &fixture.trust),
                    Err(ExportProduceError::Contract)
                ),
                "mutation {mutation}"
            );
        }
        let mut other_account = artifact.clone();
        let mut account = fixture.account_id;
        account[12] ^= 1;
        other_account.facts[0] = must(FactRef::new(format!(
            "state:{}:{}",
            hex::encode(transaction_hash(
                artifact.bundles[0].raw_transaction.as_bytes()
            )),
            hex::encode(account)
        )));
        assert!(matches!(
            verify(&other_account, &fixture.trust),
            Err(ExportProduceError::Contract)
        ));
        let mut altered = artifact.clone();
        let mut header = fixture.header.clone();
        header.app_hash[0] ^= 1;
        let mut commit = fixture.commit.clone();
        commit.block_id.hash = header.block_hash();
        for (index, vote) in commit.clone().votes.iter().enumerate() {
            commit.votes[index].signature = SigningKey::from_bytes(&[index as u8 + 1; 32])
                .sign(&commit.vote_sign_bytes(&header.chain_id, vote.timestamp))
                .to_bytes();
        }
        altered.bundles[0].block_header = Some(canonical(encode_header(&header)));
        altered.bundles[0].commit = Some(canonical(encode_commit(&commit, &fixture.validators)));
        assert!(matches!(
            verify(&altered, &fixture.trust),
            Err(ExportProduceError::Contract)
        ));
    });
}

#[test]
fn witness_codec_rejects_noncanonical_empty_duplicate_and_unbounded_evidence() {
    let fixture = Fixture::new(8, true);
    let witnesses = must(decode_storage_witnesses(&fixture.account_proof));
    assert_eq!(
        must(encode_storage_witnesses(&witnesses)).as_bytes(),
        fixture.account_proof
    );
    assert!(encode_storage_witnesses(&[]).is_err());
    assert!(encode_storage_witnesses(&[witnesses[0].clone(), witnesses[0].clone()]).is_err());
    assert!(encode_storage_witnesses(&vec![witnesses[0].clone(); 1025]).is_err());
    let mut invalid = witnesses.clone();
    invalid[0].ops.pop();
    assert!(encode_storage_witnesses(&invalid).is_err());
    let mut extra: Value = must(serde_json::from_slice(&fixture.account_proof));
    extra[0]["untrusted"] = json!(true);
    let mut duplicate = witnesses[0].clone();
    duplicate.ops[0].data = vec![0; layerx_agent_api::proof::MAX_PROOF_BUNDLE_BYTES];
    assert!(encode_storage_witnesses(&[duplicate]).is_err());
    for bytes in [
        b"[]".to_vec(),
        b"null".to_vec(),
        [b" ".as_slice(), fixture.account_proof.as_slice()].concat(),
        must(serde_json::to_vec(&extra)),
        vec![b' '; layerx_agent_api::proof::MAX_PROOF_BUNDLE_BYTES + 1],
    ] {
        assert!(matches!(
            decode_storage_witnesses(&bytes),
            Err(ExportProduceError::Contract)
        ));
    }
}
