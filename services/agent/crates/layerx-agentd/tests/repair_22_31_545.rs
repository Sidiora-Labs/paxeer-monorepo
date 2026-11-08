include!("native_send_preview_evm.rs");

use layerx_agent_api::identity::{ContractError, NativeSendPurposeV2, SignedNativeSendPurposeV2};
use layerx_agentd::capability::binding::{
    native_send_purpose_digest, restore_native_signed_send_purpose, stage_native_send_local_grant,
    verify_native_owner_send_purpose, BindingError, SignedNativeLocalGrantV1,
};
use layerx_agentd::capability::timed::NativeTimedCapabilityV1;
use layerx_agentd::prepare::{self, ProductionCorePreparationBoundary};
use layerx_agentd::session::NativeSessionScopeV1;
use layerx_agentd::sign::SigningPreparation;
use layerx_agentd::store::{ObjectKind, TenantKey};

fn owner_preview_over_unix(
    fixture: &mut Fixture,
    prepare: &HumanPrepare,
    grant: &layerx_agent_api::identity::NativeLocalGrantConsentV1,
) -> Vec<u8> {
    let bytes = |out: &mut Vec<u8>, value: &[u8]| {
        out.extend_from_slice(&must(u32::try_from(value.len())).to_be_bytes());
        out.extend_from_slice(value);
    };
    let mut request = b"LXHAGT01".to_vec();
    request.push(layerx_agentd::human::NATIVE_SEND_OWNER_PREVIEW_V1);
    request.extend_from_slice(&1_u16.to_be_bytes());
    bytes(
        &mut request,
        fixture.credential.tenant().as_str().as_bytes(),
    );
    request.extend_from_slice(&fixture.credential.session_id().0);
    request.extend_from_slice(&fixture.credential.token_id());
    request.extend_from_slice(&fixture.credential.generation().to_be_bytes());
    request.extend_from_slice(&71_u64.to_be_bytes());
    request.extend_from_slice(&prepare.activity_type.to_be_bytes());
    bytes(&mut request, prepare.actor.as_bytes());
    bytes(&mut request, prepare.authority.as_bytes());
    request.extend_from_slice(&prepare.nonce.to_be_bytes());
    request.extend_from_slice(&prepare.not_before_height.to_be_bytes());
    request.extend_from_slice(&prepare.not_after_height.to_be_bytes());
    bytes(&mut request, prepare.idempotency_key.as_bytes());
    request.extend_from_slice(&prepare.fee_limit.to_be_bytes());
    bytes(&mut request, &prepare.payload);
    request.extend_from_slice(&prepare.payload_hash);
    request.extend_from_slice(&prepare.capability_id.expect("capability"));
    request.extend_from_slice(&fixture.account.layerx_public_key());
    request.extend_from_slice(&prepare.not_after_height.to_be_bytes());
    request.extend_from_slice(&[11; 32]);
    request.extend_from_slice(&fixture.credential.session_id().0);
    bytes(&mut request, &grant.capability);
    bytes(&mut request, &grant.session_scope);
    request.extend_from_slice(&grant.expires_at_ms.to_be_bytes());
    request.extend_from_slice(&grant.owner_public_key);
    request.extend_from_slice(&grant.signature);

    let deadline = Duration::from_secs(10);
    let maximum = 1_048_576;
    let (mut client, server) = must(UnixStream::pair());
    must(client.set_read_timeout(Some(deadline)));
    must(client.set_write_timeout(Some(deadline)));
    let mut transport = must(AcceptedTransport::new(server, maximum, deadline));
    must(client.write_all(&must(u32::try_from(request.len())).to_be_bytes()));
    must(client.write_all(&request));
    must(serve_one(&mut transport, &fixture.peer, &mut fixture.owner));
    let mut prefix = [0; 4];
    must(client.read_exact(&mut prefix));
    let length = u32::from_be_bytes(prefix) as usize;
    assert!(length <= maximum);
    let mut response = vec![0; length];
    must(client.read_exact(&mut response));
    response
}

fn signed_consent(
    purpose: NativeSendPurposeV2,
    key: &ed25519_dalek::SigningKey,
) -> SignedNativeSendPurposeV2 {
    let digest = must(native_send_purpose_digest(&purpose));
    SignedNativeSendPurposeV2 {
        purpose,
        signature: key.sign(&digest).to_bytes(),
    }
}

fn retained_authorization(fixture: &Fixture, human: &HumanPrepare) -> prepare::Prepared {
    let activity = must(ActivityType::from_u32(human.activity_type));
    let registry = must(layerx_types::payload::ModuleRegistry::new(&[must(
        layerx_types::payload::ModuleRegistration::new(activity.module(), &[activity]),
    )]));
    let node = must(NodeRpc::connect(&must(NodeRpcConfig::from_environment())));
    let mut boundary = ProductionCorePreparationBoundary::new(&node, registry);
    must(prepare::prepare_activity_for_protocol(
        &mut boundary,
        prepare::PreparationDefaults {
            timestamp_span: 30,
            fee_limit: layerx_types::amount::Amount::from_u128(human.fee_limit),
            maximum_payload_bytes: 1_048_576,
        },
        prepare::PrepareRequest {
            actor: must(Did::new(fixture.account.did().as_bytes())),
            authority: must(layerx_types::activity::Authority::owner(
                &fixture.account.layerx_public_key(),
            )),
            activity_type: activity,
            expected_account_sequence: Some(human.nonce),
            timestamp_bound: Some(must(layerx_types::activity::TimestampBound::new(
                human.not_before_height,
                human.not_after_height,
            ))),
            fee_limit: Some(layerx_types::amount::Amount::from_u128(human.fee_limit)),
            idempotency_key: layerx_types::ids::IdempotencyKey::new(
                must(hex::decode(&human.idempotency_key))
                    .try_into()
                    .expect("key32"),
            ),
            payload: human.payload.clone(),
            declared_payload_limit: 1_048_576,
        },
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    ))
}

fn assert_v1_wire_refused(
    fixture: &Fixture,
    human: &HumanPrepare,
    purpose: &NativeSendPurposeV2,
    signature: [u8; 64],
) {
    let key = hex::encode(fixture.account.layerx_public_key());
    let legacy = json!({
        "version":"1", "tenant":purpose.tenant.as_str(), "agent_did":purpose.agent_did.as_str(),
        "owner_did":purpose.owner_did.as_str(), "owner_public_key":key,
        "session_id":purpose.session_id.as_str(), "generation":purpose.generation.to_string(),
        "expires_at_ms":purpose.expires_at_ms.to_string(), "capability_id":purpose.capability_id.as_str(),
        "protocol_version":"3", "network_id":"125",
        "activity":{"version":"1", "module":"1", "ordinal":"5"},
        "preparation_id":hex::encode(purpose.preparation_id), "canonical_digest":hex::encode(purpose.canonical_digest),
        "economic_action":hex::encode(purpose.economic_action), "idempotency_key":hex::encode(purpose.idempotency_key),
        "commitment":hex::encode(purpose.commitment)
    });
    let envelope = json!({
        "version":1, "request_id":"72", "operation":"prepare", "idempotency_key":human.idempotency_key,
        "credential": {"tenant":fixture.credential.tenant().as_str(),
            "session_id":hex::encode(fixture.credential.session_id().0), "token_id":hex::encode(fixture.credential.token_id()),
            "generation":fixture.credential.generation().to_string()},
        "request": {"variant":"native_send_v1", "activity":{"version":"1", "module":"1", "ordinal":"5"},
            "actor":human.actor, "authority":human.authority, "account_sequence":human.nonce.to_string(),
            "not_before":human.not_before_height.to_string(), "not_after":human.not_after_height.to_string(),
            "idempotency_key":human.idempotency_key, "fee_limit":human.fee_limit.to_string(),
            "payload":hex::encode(&human.payload), "payload_hash":hex::encode(human.payload_hash),
            "capability_id":purpose.capability_id.as_str(), "local_grant":null,
            "purpose":{"purpose":legacy, "owner_public_key":key, "signature":hex::encode(signature)}}
    });
    let response =
        layerx_agentd::agent_rpc::handle_rpc(&fixture.owner, &must(serde_json::to_vec(&envelope)));
    assert_eq!(
        response.status,
        400,
        "{}",
        String::from_utf8_lossy(&response.body)
    );
    let body: Value = must(serde_json::from_slice(&response.body));
    assert_eq!(body["reason"], "envelope.malformed", "{body}");
}

fn legacy_signed_record(purpose: &NativeSendPurposeV2, key: &ed25519_dalek::SigningKey) -> Vec<u8> {
    let mut canonical = layerx_agent_api::identity::NativeSendPurposeV1::DOMAIN.to_vec();
    canonical.push(layerx_agent_api::identity::NativeSendPurposeV1::VERSION);
    for value in [
        purpose.tenant.as_str(),
        purpose.agent_did.as_str(),
        purpose.owner_did.as_str(),
    ] {
        canonical.extend_from_slice(&must(u32::try_from(value.len())).to_be_bytes());
        canonical.extend_from_slice(value.as_bytes());
    }
    canonical.extend_from_slice(&purpose.consent_public_key);
    canonical.extend_from_slice(&must(purpose.session_id.to_bytes()));
    canonical.extend_from_slice(&purpose.generation.to_be_bytes());
    canonical.extend_from_slice(&purpose.expires_at_ms.to_be_bytes());
    canonical.extend_from_slice(&must(purpose.capability_id.to_bytes()));
    canonical.extend_from_slice(&3_u16.to_be_bytes());
    canonical.extend_from_slice(&125_u32.to_be_bytes());
    canonical.extend_from_slice(&must(purpose.activity.encode()));
    for value in [
        purpose.preparation_id,
        purpose.canonical_digest,
        purpose.economic_action,
        purpose.idempotency_key,
        purpose.commitment,
    ] {
        canonical.extend_from_slice(&value);
    }
    assert!(matches!(
        layerx_agent_api::identity::NativeSendPurposeV1::from_canonical_bytes(&canonical),
        Err(ContractError::Mismatch("native_send.legacy_version"))
    ));
    let signature = key.sign(&Sha256::digest(&canonical)).to_bytes();
    let mut record = b"LXNS\x01".to_vec();
    record.extend_from_slice(&canonical);
    record.extend_from_slice(&purpose.consent_public_key);
    record.extend_from_slice(&signature);
    record
}

#[test]
fn native_send_v2_unix_consent_binding_persists_and_reopens_with_current_owner_checks() {
    let _guard = must(FIXTURE_LOCK.lock());
    let mut fixture = Fixture::new();
    let (human, consent) = fixture.request();
    let response = owner_preview_over_unix(&mut fixture, &human, &consent);
    assert_eq!(&response[..9], b"LXHAGT01\x00");
    let mut input = &response[9..];
    assert_eq!(&input[..2], &1_u16.to_be_bytes());
    input = &input[2..];
    let canonical = read_bytes(&mut input).to_vec();
    let signing_hash: [u8; 32] = read_bytes(&mut input).try_into().expect("signing hash32");
    let purpose = must(NativeSendPurposeV2::from_canonical_bytes(read_bytes(
        &mut input,
    )));
    assert_eq!(purpose.chain_id, 125);
    assert_eq!(purpose.transaction_type, 2);
    assert_eq!(purpose.evm_sender, fixture.account.evm_address());
    assert_eq!(
        purpose.consent_public_key,
        fixture.account.layerx_public_key()
    );
    assert_eq!(
        purpose.owner_did.as_str(),
        fixture.peer.subject.as_ref().expect("owner").owner
    );
    assert_eq!(
        purpose.canonical_digest,
        <[u8; 32]>::from(Sha256::digest(&canonical))
    );
    assert_eq!(purpose.preparation_id, purpose.canonical_digest);
    assert_eq!(purpose.commitment, [11; 32]);
    assert_eq!(input.len(), 56);
    let head = u64::from_be_bytes(input[..8].try_into().expect("head"));
    let now = u64::from_be_bytes(input[8..16].try_into().expect("timestamp"));
    assert_eq!(head, HEAD);
    assert_eq!(&input[16..48], &purpose.consent_public_key);
    let call = must(layerx_crypto::evm_transaction::decode_unsigned(&canonical));
    let disclosure = must(prepare::disclose::bind(
        &call,
        fixture.account.evm_address(),
    ));
    let transaction = SigningPreparation {
        transaction: must(prepare::disclose::restore_call(
            &canonical,
            fixture.account.evm_address(),
            head,
            disclosure.digest,
            signing_hash,
        )),
        actor_address: fixture.account.evm_address(),
    };
    let authorization = retained_authorization(&fixture, &human);
    let signing = ed25519_dalek::SigningKey::from_bytes(fixture.account.layerx_seed());
    let signed = signed_consent(purpose.clone(), &signing);
    // The RPC envelope decoder owns this refusal; managed-agent reasons do not replace it.
    assert_v1_wire_refused(&fixture, &human, &purpose, signed.signature);

    let (control, owner) = {
        let mut daemon = must(fixture.owner.lock());
        let control = daemon.session_control.clone();
        let permit = must(control.authorize(
            &fixture.credential,
            layerx_agentd::tenant::Operation::Prepare,
            layerx_agentd::tenant::Surface::Contract,
            head,
            None,
        ));
        let context = must(layerx_agentd::agent_rpc_peer::bind(&mut daemon, &permit));
        let owner = must(daemon.native_send_purpose_owner(
            &context,
            fixture.account.layerx_public_key(),
            head,
        ));
        (control, owner)
    };
    let retained = control.store();
    let sessions = control.registry();
    let preparation_id = purpose.preparation_id;
    let authorization_key = must(TenantKey::new(
        fixture.credential.tenant().clone(),
        ObjectKind::PreparedActivity,
        b"repair-native-send-authorization".to_vec(),
    ));
    let replay_key;
    {
        let mut store = must(retained.lock());
        let registry = must(sessions.read());
        let grant = SignedNativeLocalGrantV1 {
            capability: must(NativeTimedCapabilityV1::decode(&consent.capability)),
            session: must(NativeSessionScopeV1::decode(&consent.session_scope)),
            expires_at_ms: consent.expires_at_ms,
            owner_public_key: consent.owner_public_key,
            signature: consent.signature,
        };
        let staged = must(stage_native_send_local_grant(
            &store, &registry, &owner, grant, head, now,
        ));
        for (key, bytes) in must(staged.updates(&store)) {
            must(store.put_local(key, bytes));
        }
        let token = must(registry.authenticate(&fixture.credential));
        let session = must(session::admit_native(
            &store,
            &registry,
            &token,
            token.tenant(),
            token.agent(),
            purpose.activity,
            head,
        ));
        let verified = must(verify_native_owner_send_purpose(
            &store,
            &registry,
            &session,
            &owner,
            &authorization,
            &transaction,
            &preparation_id,
            &signed,
            head,
            now,
        ));
        assert_eq!(
            verified.binding().canonical_digest(),
            purpose.canonical_digest
        );
        let fee = must(layerx_agentd::budget::native_fee_policy(
            &fixture.endpoint,
            head,
        ));
        let constraints = must(
            layerx_agentd::capability::binding::inspect_native_send_capability(
                &store,
                &verified,
                &authorization,
                &transaction,
                now,
                &fee,
                head,
            ),
        );
        assert_eq!(constraints.binding(), verified.binding());
        let (key, bytes) = must(verified.replay_update(&store));
        assert_eq!(
            must(SignedNativeSendPurposeV2::from_canonical_bytes(&bytes)),
            signed
        );
        replay_key = key.clone();
        must(store.put_local(key, bytes));
        must(transaction.retain(
            &mut store,
            fixture.credential.tenant().clone(),
            preparation_id,
        ));
        must(store.put_local(
            authorization_key.clone(),
            authorization.canonical_bytes.clone(),
        ));
        assert_eq!(
            must(restore_native_signed_send_purpose(
                &store,
                fixture.credential.tenant(),
                &preparation_id
            )),
            Some(signed.clone())
        );

        for changed in [
            NativeSendPurposeV2 {
                consent_public_key: ed25519_dalek::SigningKey::from_bytes(&[42; 32])
                    .verifying_key()
                    .to_bytes(),
                ..purpose.clone()
            },
            NativeSendPurposeV2 {
                evm_sender: [42; 20],
                ..purpose.clone()
            },
            NativeSendPurposeV2 {
                owner_did: must(layerx_agent_api::identity::AgentDid::new(format!(
                    "did:layerx:{}",
                    "42".repeat(32)
                ))),
                ..purpose.clone()
            },
        ] {
            let key = if changed.consent_public_key == purpose.consent_public_key {
                signing.clone()
            } else {
                ed25519_dalek::SigningKey::from_bytes(&[42; 32])
            };
            let changed = signed_consent(changed, &key);
            assert!(matches!(
                verify_native_owner_send_purpose(
                    &store,
                    &registry,
                    &session,
                    &owner,
                    &authorization,
                    &transaction,
                    &preparation_id,
                    &changed,
                    head,
                    now
                ),
                Err(BindingError::NativeSend(ContractError::Mismatch(
                    "native_send.current_owner"
                )))
            ));
        }
        let mut invalid = signed.clone();
        invalid.signature[0] ^= 1;
        assert!(matches!(
            verify_native_owner_send_purpose(
                &store,
                &registry,
                &session,
                &owner,
                &authorization,
                &transaction,
                &preparation_id,
                &invalid,
                head,
                now
            ),
            Err(BindingError::Unbound)
        ));
        let mut changed_call = transaction.clone();
        changed_call.transaction.call.nonce += 1;
        let changed_disclosure = must(prepare::disclose::bind(
            &changed_call.transaction.call,
            changed_call.actor_address,
        ));
        changed_call.transaction.disclosure = changed_disclosure.disclosure;
        changed_call.transaction.disclosure_digest = changed_disclosure.digest;
        changed_call.transaction.signing_hash = changed_call.transaction.call.signing_hash();
        assert!(matches!(
            verify_native_owner_send_purpose(
                &store,
                &registry,
                &session,
                &owner,
                &authorization,
                &changed_call,
                &preparation_id,
                &signed,
                head,
                now
            ),
            Err(BindingError::NativeSend(ContractError::Mismatch(
                "native_send.prepared_transaction"
            )))
        ));
    }
    drop(owner);
    drop(sessions);
    drop(control);
    {
        let mut store = must(retained.lock());
        let replacement = must(Store::open(fixture.root.join("closed-store")));
        drop(std::mem::replace(&mut *store, replacement));
        *store = must(Store::open(fixture.root.join("store")));
    }
    let (control, owner) = {
        let mut daemon = must(fixture.owner.lock());
        let control = daemon.session_control.clone();
        let permit = must(control.authorize(
            &fixture.credential,
            layerx_agentd::tenant::Operation::Prepare,
            layerx_agentd::tenant::Surface::Contract,
            head,
            None,
        ));
        let context = must(layerx_agentd::agent_rpc_peer::bind(&mut daemon, &permit));
        let owner = must(daemon.native_send_purpose_owner(
            &context,
            fixture.account.layerx_public_key(),
            head,
        ));
        (control, owner)
    };
    let mut registry = SessionRegistry::default();
    let mut store = must(retained.lock());
    must(registry.restore_tenant(&store, fixture.credential.tenant()));
    let token = must(registry.authenticate(&fixture.credential));
    let session = must(session::admit_native(
        &store,
        &registry,
        &token,
        token.tenant(),
        token.agent(),
        purpose.activity,
        head,
    ));
    let restored = must(restore_native_signed_send_purpose(
        &store,
        fixture.credential.tenant(),
        &preparation_id,
    ))
    .expect("persisted V2 consent");
    assert_eq!(restored, signed);
    let transaction = must(SigningPreparation::restore(
        &store,
        fixture.credential.tenant().clone(),
        preparation_id,
    ));
    let activity = authorization.envelope.activity_type();
    let modules = must(layerx_types::payload::ModuleRegistry::new(&[must(
        layerx_types::payload::ModuleRegistration::new(activity.module(), &[activity]),
    )]));
    let authorization = must(prepare::restore_canonical(
        store
            .get(&authorization_key)
            .expect("retained authorization")
            .bytes(),
        head,
        &modules,
        Some(authorization.envelope.authority()),
    ));
    let verified = must(verify_native_owner_send_purpose(
        &store,
        &registry,
        &session,
        &owner,
        &authorization,
        &transaction,
        &preparation_id,
        &restored,
        head,
        now,
    ));
    assert_eq!(must(verified.replay_update(&store)).0, replay_key);
    assert_eq!(
        store.get(&replay_key).expect("V2 record").bytes(),
        must(restored.canonical_bytes())
    );
    must(store.update_local_batch(vec![(
        replay_key.clone(),
        legacy_signed_record(&purpose, &signing),
    )]));
    assert!(matches!(
        restore_native_signed_send_purpose(&store, fixture.credential.tenant(), &preparation_id),
        Err(BindingError::Corrupt)
    ));
    assert!(matches!(
        verified.replay_update(&store),
        Err(BindingError::Conflict)
    ));
    drop(store);
    drop(control);
}
