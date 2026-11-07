include!("bootstrap_bind_layerx.rs");

use layerx_agentd::human::HumanOperationError;
use layerx_types::verify::VerificationLevel;

fn bound_owner_result(fixture: &OwnerFixture) -> Result<(), HumanOperationError> {
    let mut owner = must(fixture.owner.lock());
    let permit = must(owner.session_control.authorize(
        &fixture.credential,
        layerx_agentd::tenant::Operation::AgentRegister,
        layerx_agentd::tenant::Surface::Contract,
        HEAD,
        None,
    ));
    let context = layerx_agentd::agent_rpc_peer::bind(&mut owner, &permit)?;
    let public_key = bytes(text(&fixture.document["agent_register"], "authority_ref"));
    owner
        .native_send_purpose_owner(&context, public_key, HEAD)
        .map(|_| ())
}

#[test]
fn repair_22_31_549_committed_and_legacy_identity_allow_bound_owner_operation() {
    let _serial = must(FIXTURE_LOCK.lock());
    let fixture = OwnerFixture::new();
    for label in [
        VerificationLevel::COMMITTED.wire_label(),
        "checkpoint_finalised",
        "settlement_anchored",
    ] {
        must(fixture.labels.lock()).identity = label.to_owned();
        assert_eq!(
            bound_owner_result(&fixture),
            Ok(()),
            "identity label {label}"
        );
        let request = fixture.document["agent_register"].clone();
        let from = bytes(text(&request, "owner_address"));
        fixture.assert_prepared(
            fixture.call(&fixture.envelope("agent.register", request.clone())),
            from,
            layerx_client::account_binding::ADDR_PRECOMPILE,
            [0xdd, 0x9a, 0xa6, 0x28],
            must(text(&request, "fee_limit").parse()),
        );
    }
    assert!(fixture.outbox().is_empty());
}

#[test]
fn repair_22_31_549_below_committed_and_unknown_identity_refuse_bound_owner_operation() {
    let _serial = must(FIXTURE_LOCK.lock());
    let fixture = OwnerFixture::new();
    assert_eq!(bound_owner_result(&fixture), Ok(()));
    let before = fixture.outbox();
    for label in [
        VerificationLevel::SEQUENCER_SIGNED.wire_label(),
        VerificationLevel::BATCH_INCLUDED.wire_label(),
        VerificationLevel::STATE_PROVEN.wire_label(),
        VerificationLevel::UNVERIFIED.wire_label(),
        "unknown",
    ] {
        must(fixture.labels.lock()).identity = label.to_owned();
        assert_eq!(
            bound_owner_result(&fixture),
            Err(HumanOperationError::Refused),
            "identity label {label}"
        );
        assert_refused(
            fixture.call(
                &fixture.envelope("agent.register", fixture.document["agent_register"].clone()),
            ),
            403,
            "owner.refused",
        );
    }
    must(fixture.labels.lock()).identity = VerificationLevel::COMMITTED.wire_label().to_owned();
    assert_eq!(bound_owner_result(&fixture), Ok(()));
    assert_eq!(fixture.outbox(), before);
}

#[test]
fn repair_22_31_549_subject_context_requires_decoded_committed_level() {
    let _serial = must(FIXTURE_LOCK.lock());
    let mut fixture = OwnerFixture::new();
    for label in [
        VerificationLevel::COMMITTED.wire_label(),
        "checkpoint_finalised",
        "settlement_anchored",
    ] {
        must(fixture.labels.lock()).context = label.to_owned();
        assert_eq!(
            fixture.owner.authorize_subject(&fixture.peer),
            Ok(()),
            "context label {label}"
        );
    }
    for label in [
        VerificationLevel::UNVERIFIED.wire_label(),
        VerificationLevel::SEQUENCER_SIGNED.wire_label(),
        VerificationLevel::BATCH_INCLUDED.wire_label(),
        VerificationLevel::STATE_PROVEN.wire_label(),
        "unknown",
        "",
        "COMMITTED",
    ] {
        must(fixture.labels.lock()).context = label.to_owned();
        assert_eq!(
            fixture.owner.authorize_subject(&fixture.peer),
            Err(HumanOperationError::Refused),
            "context label {label}"
        );
    }
}

#[test]
fn repair_22_31_549_identity_parser_uses_current_labels_and_legacy_mapping_over_tls() {
    let account = must(derive_from_seed(&random::<64>(), 0));
    let key = account.layerx_public_key();
    let principal = hex::encode(key);
    let peer = HumanPeer {
        uid: 7,
        tenant: must(layerx_identity_binding::subject_namespace(
            "bootstrap-service",
            &principal,
        )),
        principal,
        subject: Some(HumanSubject {
            transport_tenant: "bootstrap-service".to_owned(),
            transport_principal: "bootstrap-owner".to_owned(),
            owner: account.did(),
            account: format!("agent:{}:main", account.did()),
            asset: random(),
            registration: None,
        }),
    };
    let did = must(Did::new(account.did().as_bytes()));
    let labels = Arc::new(Mutex::new(ServedLabels::default()));
    let (mut authority, _worker) = authority_peer_with_labels(
        peer.clone(),
        CoreIdentity {
            canonical_bytes: key.to_vec(),
            head_sequence: HEAD,
            revocation_sequence: 1,
            verification_level: VerificationLevel::COMMITTED,
            frozen: false,
            authorities: vec![ProtocolAuthority::PrimaryKey(key)],
        },
        must(ActivityType::new(ModuleId::Asset, 5)),
        Arc::clone(&labels),
    );
    for level in [
        VerificationLevel::SEQUENCER_SIGNED,
        VerificationLevel::BATCH_INCLUDED,
        VerificationLevel::STATE_PROVEN,
        VerificationLevel::COMMITTED,
    ] {
        must(labels.lock()).identity = level.wire_label().to_owned();
        assert_eq!(
            must(authority.core_identity(&peer, &did)).verification_level,
            level
        );
    }
    for label in ["checkpoint_finalised", "settlement_anchored"] {
        must(labels.lock()).identity = label.to_owned();
        assert!(VerificationLevel::from_wire_label(label).is_err());
        assert_eq!(
            must(authority.core_identity(&peer, &did)).verification_level,
            VerificationLevel::COMMITTED
        );
    }
    for label in [
        VerificationLevel::UNVERIFIED.wire_label(),
        "unknown",
        "",
        "COMMITTED",
    ] {
        must(labels.lock()).identity = label.to_owned();
        assert_eq!(
            authority.core_identity(&peer, &did),
            Err(IdentityError::Unverified)
        );
    }
}
