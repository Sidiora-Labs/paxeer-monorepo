//! Registration commitment and restart cases also run under the library's
//! human_runtime::subject::tests::registration_tests filter.

use layerx_agentd::human::{HumanOperationError, HumanPeer, HumanSubject};
use layerx_agentd::human_runtime::bind_retained_registration;
use layerx_agentd::outbox::{Outbox, SubmissionState};
use layerx_agentd::store::{Store, TenantId};
use layerx_client::account_binding::{plan_bind, sign_bind_transaction, BindFees, BindPlan};
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::evm_transaction::transaction_hash;
use layerx_types::ids::Did;

#[test]
fn subject_registration_test_gap_requires_retained_execution_evidence_after_restart() {
    let root =
        std::env::temp_dir().join(format!("repair-22-33-1514-linked-{}", std::process::id()));
    let account = derive_from_seed(&[0x33; 64], 0).expect("account producer");
    let BindPlan::Bind(call) = plan_bind(&account, 125, None, 0).expect("registration plan") else {
        panic!("new account must produce registration");
    };
    let did = Did::new(call.signed_binding().did().as_bytes()).expect("registered DID");
    let raw = sign_bind_transaction(
        &account,
        125,
        &call,
        BindFees {
            evm_nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
        },
    )
    .expect("production signed registration");
    let peer = HumanPeer {
        uid: 7,
        principal: "alice".into(),
        tenant: layerx_identity_binding::subject_namespace("human-service", "alice")
            .expect("subject namespace"),
        subject: Some(HumanSubject {
            transport_tenant: "human-service".into(),
            transport_principal: "human-transport".into(),
            owner: "did:layerx:alice".into(),
            account: "agent:did:layerx:alice:main".into(),
            asset: [0x24; 32],
            registration: None,
        }),
    };
    let tenant = TenantId::new(peer.tenant.clone()).expect("tenant");
    let mut store = Store::open(&root).expect("store");
    let mut outbox = Outbox::default();
    let hash = outbox
        .enqueue(&mut store, tenant.clone(), raw.clone())
        .expect("retain producer transaction");
    assert_eq!(hash, transaction_hash(&raw));
    for restart in [false, true] {
        if restart {
            drop(outbox);
            drop(store);
            store = Store::open(&root).expect("reopened store");
            outbox = Outbox::default();
            outbox
                .restore(&store, tenant.clone(), hash)
                .expect("restore outbox");
        }
        assert_eq!(
            outbox.status(hash).expect("status").state,
            SubmissionState::Queued
        );
        assert_eq!(
            outbox.exact_signed_bytes(hash).expect("producer bytes"),
            raw
        );
        assert!(matches!(
            bind_retained_registration(&store, &peer, &did, 125, account.evm_address()),
            Err(HumanOperationError::Refused)
        ));
    }
    drop(outbox);
    drop(store);
    std::fs::remove_dir_all(root).expect("remove store");
}
