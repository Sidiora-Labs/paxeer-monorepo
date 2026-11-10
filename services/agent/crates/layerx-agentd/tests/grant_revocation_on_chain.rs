#[path = "support/repair_22_33_1529.rs"]
mod repair_support;
mod support;

use std::collections::BTreeSet;
use std::fs;

use layerx_agentd::identity::{register, ProtocolAuthority};
use layerx_agentd::session::{
    invalidate_on_revocation, open, InvalidationReason, OpenRequest, RevocationEvent, SessionError,
    SessionId, SessionRef, SessionRegistry, Token,
};
use layerx_agentd::store::{Store, TenantId};
use layerx_types::ids::Did;

fn open_session(
    store: &mut Store,
    sessions: &mut SessionRegistry,
    tenant: &TenantId,
    did: &Did,
    session_byte: u8,
    authority: ProtocolAuthority,
) -> Token {
    let (mut node_authority, worker) = repair_support::authority_peer(vec![(
        200,
        repair_support::identity_document(1, false, std::slice::from_ref(&authority)),
    )]);
    let peer = repair_support::peer();
    let mut resolver = repair_support::AuthorityIdentity {
        authority: &mut node_authority,
        peer: &peer,
    };
    let identity = register(store, tenant.clone(), did.clone(), &mut resolver)
        .unwrap_or_else(|error| panic!("identity: {error:?}"));
    assert_ok(worker.join(), "verified node authority transport");
    let request = OpenRequest {
        session_id: SessionId([session_byte; 32]),
        token_id: [session_byte.wrapping_add(1); 32],
        tenant: tenant.clone(),
        agent: did.clone(),
        authority,
        permitted_activity_types: BTreeSet::from([7]),
        scopes: BTreeSet::from(["prepare".to_owned()]),
        expiry_sequence: 100,
        expiry_seconds: None,
        opening_client: "mcp".to_owned(),
        policy_version: "v1".to_owned(),
    };
    open(store, sessions, &identity, request, 1)
        .unwrap_or_else(|error| panic!("session: {error:?}"))
}

#[test]
fn on_chain_grant_revocation_invalidates_capability_grant_sessions() {
    for (case, _authority) in [
        ProtocolAuthority::SessionKey([3; 32]),
        ProtocolAuthority::CapabilityGrant([3; 32]),
    ]
    .into_iter()
    .enumerate()
    {
        let path = std::env::temp_dir().join(format!(
            "layerx-agentd-grant-revocation-on-chain-{}-{case}",
            std::process::id()
        ));
        let mut store = Store::open(&path).unwrap_or_else(|error| panic!("store: {error}"));
        let mut sessions = SessionRegistry::default();
        let tenant = TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}"));
        let did = Did::new(b"did:layerx:model").unwrap_or_else(|error| panic!("DID: {error:?}"));
        let other_did =
            Did::new(b"did:layerx:other").unwrap_or_else(|error| panic!("other DID: {error:?}"));
        let tokens = revocation_tokens(&mut store, &mut sessions, &tenant, &did, &other_did);
        for token in &tokens {
            assert_eq!(sessions.generation(&tenant, token.session_id()), Some(1));
            assert_eq!(
                token.authorize(&sessions, &tenant, token.agent(), "prepare", 1),
                Ok(token.session_id())
            );
        }

        let event = chain_revocation(&mut store, &did, [3; 32]);
        assert_eq!(
            event.authority,
            Some(ProtocolAuthority::SessionKey([3; 32]))
        );
        let report = invalidate_on_revocation(&mut store, &mut sessions, &mut [], &event)
            .unwrap_or_else(|error| panic!("invalidation: {error:?}"));
        assert_eq!(report.reason, InvalidationReason::GrantRevokedOnChain);
        assert_eq!(report.observed_sequence, 8);
        assert_eq!(
            report.invalidated_sessions,
            vec![
                SessionRef::new(tenant.clone(), SessionId([1; 32])),
                SessionRef::new(tenant.clone(), SessionId([2; 32])),
            ]
        );
        assert_eq!(
            report.invalidated_generations,
            report
                .invalidated_sessions
                .iter()
                .cloned()
                .map(|session| (session, 1))
                .collect::<Vec<_>>()
        );
        for token in &tokens[..2] {
            assert_eq!(sessions.generation(&tenant, token.session_id()), Some(2));
            assert!(
                !sessions
                    .get(&tenant, token.session_id())
                    .unwrap_or_else(|| panic!("session missing"))
                    .open
            );
            assert_eq!(
                token.authorize(&sessions, &tenant, &did, "prepare", 8),
                Err(SessionError::Revoked)
            );
        }
        for token in &tokens[2..] {
            assert_eq!(sessions.generation(&tenant, token.session_id()), Some(1));
            assert!(
                sessions
                    .get(&tenant, token.session_id())
                    .unwrap_or_else(|| panic!("session missing"))
                    .open
            );
            assert_eq!(
                token.authorize(&sessions, &tenant, token.agent(), "prepare", 8),
                Ok(token.session_id())
            );
        }
        drop(sessions);
        drop(store);
        assert_revoked_sessions_after_restart(&path, &tenant, &tokens);
        fs::remove_dir_all(path).unwrap_or_else(|error| panic!("remove store: {error}"));
    }
}

// The committed 41-byte GrantRevoked vector is the record used by
// session_grants_layerxauth.rs::revoke_receipt and modules/layerxauth/types/revoke.go.
fn chain_revocation(store: &mut Store, did: &Did, grant: [u8; 32]) -> RevocationEvent {
    use layerx_agentd::{
        human_runtime::lookup_human_receipt_by_outbox_hash,
        node_rpc::{NodeRpc, NodeRpcConfig},
        outbox::Outbox,
        prepare::evm::PrecompileTarget,
    };
    use layerx_client::{
        rpc::receipt::Lookup,
        sequencer::{SequencerEndpoint, TransactionReceipt},
    };
    use layerx_crypto::evm_transaction::transaction_hash;
    use serde_json::{json, Value};
    let (raw, emitted) = signed_revocation(grant);
    let (view, key) = repair_support::signed_node_receipt(&raw, true, &emitted);
    let hash = transaction_hash(&raw);
    let mut outbox = Outbox::default();
    assert_eq!(
        assert_ok(
            outbox.enqueue(store, support::tenant(), raw),
            "retain revocation bytes"
        ),
        hash
    );
    let (url, server) = repair_support::rpc_peer(vec![
        ("eth_chainId", json!("0x7d")),
        (
            "px_getReceiptKeys",
            json!({"keys":[{"keyId":format!("0x{}",hex::encode(key.key_id)),"publicKey":format!("0x{}",hex::encode(key.public_key)),"firstHeight":key.first_height,"lastHeight":key.last_height}]}),
        ),
        ("pax_sequencerReceipt", view.clone()),
        ("pax_sequencerReceipt", view),
        ("eth_getTransactionReceipt", Value::Null),
    ]);
    let node = assert_ok(
        NodeRpc::connect(&NodeRpcConfig {
            url: url.clone(),
            chain_id: 125,
            deadline: std::time::Duration::from_secs(3),
        }),
        "verified node adapter",
    );
    let endpoint = assert_ok(SequencerEndpoint::new(&url, &[]), "node endpoint");
    let (lookup, keys) = assert_ok(
        lookup_human_receipt_by_outbox_hash(&node, &endpoint, &outbox, hash, hash),
        "signed chain evidence",
    );
    assert_eq!(keys, vec![key]);
    let Lookup::Verified(verified) = lookup else {
        panic!("authenticated revocation required");
    };
    let execution = verified.execution();
    assert_eq!(execution.logs, vec![emitted]);
    assert_eq!(verified.activity_id(), hash);
    assert_eq!(verified.height(), 8);
    assert_eq!(verified.result_code().raw(), 0);
    assert!(execution.success);
    assert_eq!(execution.nonce, 7);
    assert_eq!(execution.to, PrecompileTarget::Auth.address());
    let receipt = TransactionReceipt {
        status: execution.success,
        block_number: execution.height,
        transaction_index: execution.index,
        gas_used: execution.gas_used,
        logs: execution.logs.clone(),
    };
    let event = assert_ok(
        RevocationEvent::from_revoke_grant_receipt(did.clone(), grant, &receipt),
        "verified revocation adapter",
    );
    let mut wrong = receipt.clone();
    wrong.logs[0].data[64] ^= 1;
    assert!(RevocationEvent::from_revoke_grant_receipt(did.clone(), grant, &wrong).is_err());
    let mut failed = receipt;
    failed.status = false;
    assert!(RevocationEvent::from_revoke_grant_receipt(did.clone(), grant, &failed).is_err());
    let requests = assert_ok(server.join(), "revocation transcript");
    assert_eq!(requests.len(), 5);
    for request in &requests[2..] {
        assert_eq!(
            request["params"],
            json!([format!("0x{}", hex::encode(hash))])
        );
    }
    event
}

#[track_caller]
fn assert_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{message}: {error:?}"),
    }
}

fn revocation_tokens(
    store: &mut Store,
    sessions: &mut SessionRegistry,
    tenant: &TenantId,
    did: &Did,
    other_did: &Did,
) -> [Token; 4] {
    [
        open_session(
            store,
            sessions,
            tenant,
            did,
            1,
            ProtocolAuthority::CapabilityGrant([3; 32]),
        ),
        open_session(
            store,
            sessions,
            tenant,
            did,
            2,
            ProtocolAuthority::SessionKey([3; 32]),
        ),
        open_session(
            store,
            sessions,
            tenant,
            did,
            3,
            ProtocolAuthority::CapabilityGrant([4; 32]),
        ),
        open_session(
            store,
            sessions,
            tenant,
            other_did,
            4,
            ProtocolAuthority::CapabilityGrant([3; 32]),
        ),
    ]
}

fn assert_revoked_sessions_after_restart(
    path: &std::path::Path,
    tenant: &TenantId,
    tokens: &[Token],
) {
    let store = assert_ok(Store::open(path), "reopen revocation store");
    let mut sessions = SessionRegistry::default();
    assert_ok(
        sessions.restore_tenant(&store, tenant),
        "restore session registry",
    );
    for token in &tokens[..2] {
        assert_eq!(sessions.generation(tenant, token.session_id()), Some(2));
        assert_eq!(
            token.authorize(&sessions, tenant, token.agent(), "prepare", 8),
            Err(SessionError::Revoked)
        );
    }
    for token in &tokens[2..] {
        assert_eq!(sessions.generation(tenant, token.session_id()), Some(1));
        assert_eq!(
            token.authorize(&sessions, tenant, token.agent(), "prepare", 8),
            Ok(token.session_id())
        );
    }
    drop(sessions);
    drop(store);
}

fn signed_revocation(grant: [u8; 32]) -> (Vec<u8>, layerx_client::sequencer::SequencerLog) {
    use layerx_agentd::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
    use layerx_client::sequencer::SequencerLog;
    use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
    let mut record = grant.to_vec();
    record.push(1);
    record.extend_from_slice(&43u64.to_be_bytes());
    let wrapped = assert_ok(
        PrecompileCall {
            target: PrecompileTarget::Auth,
            selector: [0; 4],
            arguments: vec![AbiArgument::Bytes(record)],
        }
        .encode(),
        "production dynamic ABI codec",
    );
    let emitted = SequencerLog {
        address: PrecompileTarget::Auth.address(),
        topics: vec![transaction_hash(b"GrantRevoked(bytes32,bytes)"), grant],
        data: wrapped[4..].to_vec(),
        log_index: 0,
    };
    let word = |value: u64| {
        let mut word = [0; 32];
        word[24..].copy_from_slice(&value.to_be_bytes());
        word
    };
    let call = PrecompileCall {
        target: PrecompileTarget::Auth,
        selector: assert_ok(
            transaction_hash(b"revokeGrant(bytes32,uint8,uint64)")[..4].try_into(),
            "selector",
        ),
        arguments: vec![
            AbiArgument::Word(grant),
            AbiArgument::Word(word(1)),
            AbiArgument::Word(word(43)),
        ],
    };
    let raw = assert_ok(
        Eip1559Call {
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
            to: PrecompileTarget::Auth.address(),
            data: assert_ok(call.encode(), "revocation call"),
        }
        .sign(&[0xa5; 32]),
        "sign revoke call",
    );
    (raw, emitted)
}
