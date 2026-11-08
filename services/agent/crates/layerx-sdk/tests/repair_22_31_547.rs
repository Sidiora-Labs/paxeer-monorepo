// Reuse the daemon's preview fixture with its authenticated preparation reads and policy.
include!("../../layerx-agentd/tests/repair_22_31_560.rs");

use layerx_agent_api::error::RequestId;
use layerx_agent_api::idempotency::Key;
use layerx_agent_api::identity::NativeActivity;
use layerx_programs_registry as layerx_programs;
use layerx_sdk::agent_envelope::{AgentEnvelopeTransport, EnvelopeCredential, EnvelopeError};
use layerx_sdk::native_effect::{
    encode_native_send_prepare, native_send_prepare_digest, sign_native_send_purpose,
    NativeSendPrepareRequestV2,
};

fn assert_native_owner_resolution(fixture: &mut Fixture, observed_head: u64) {
    use layerx_agentd::human_node::HumanNode;
    use layerx_agentd::prepare::ProductionEvmPreparationBoundary;
    use layerx_agentd::tenant::{Operation, Surface};

    assert_eq!(observed_head, HEAD);
    let chain = must(NodeRpc::connect(&must(NodeRpcConfig::from_environment())));
    assert_eq!(
        must(chain.bound_address(&fixture.account.layerx_public_key())),
        Some(fixture.account.evm_address()),
        "native_send_purpose_owner: pending addr-precompile lookup"
    );
    let config = must(NodeRpcConfig::from_environment());
    let boundary = must(ProductionEvmPreparationBoundary::new(&config.url, &[]));
    let snapshot = must(boundary.preparation_state(&fixture.account.evm_address()));
    assert_eq!(snapshot.head_height, observed_head);
    assert_eq!(must(chain.head()).number, snapshot.head_height);

    let mut daemon = must(fixture.owner.lock());
    let control = daemon.session_control.clone();
    let permit = must(control.authorize(
        &fixture.credential,
        Operation::Prepare,
        Surface::Contract,
        observed_head,
        None,
    ));
    assert_eq!(permit.principal().tenant, *fixture.credential.tenant());
    assert_eq!(
        permit.principal().session_id,
        fixture.credential.session_id()
    );
    assert_eq!(
        permit.principal().agent.as_bytes(),
        fixture.account.did().as_bytes()
    );
    let context = must(layerx_agentd::agent_rpc_peer::bind(&mut daemon, &permit));
    assert_eq!(context.peer(), &fixture.peer);
    assert_eq!(context.peer().uid, 7);
    assert_eq!(context.peer().tenant, fixture.credential.tenant().as_str());
    assert_eq!(
        context
            .peer()
            .subject
            .as_ref()
            .expect("owner subject")
            .owner,
        fixture.account.did()
    );
    assert!(
        daemon
            .native_send_purpose_owner(
                &context,
                fixture.account.layerx_public_key(),
                snapshot.head_height,
            )
            .is_ok(),
        "native_purpose_owner and native_send_purpose_owner must accept the authenticated fixture owner"
    );
}

#[test]
fn native_sdk_v2_consent_crosses_daemon_rpc_and_legacy_is_malformed() {
    with_fixture_lock(|| {
        let mut fixture = fixture_with_receipt_keys();
        let (mut prepare, local_grant, payload_expires_at) = fixture.request_with_payload_expiry();
        let config = must(NodeRpcConfig::from_environment());
        let fee_preparation = must(layerx_agentd::prepare::evm::prepare_activity(
            &must(layerx_agentd::prepare::ProductionEvmPreparationBoundary::new(&config.url, &[])),
            layerx_agentd::prepare::EvmPreparationDefaults {
                max_priority_fee_per_gas: 0,
                max_fee_per_gas: prepare.fee_limit,
                maximum_payload_bytes: layerx_types::limits::MAX_PAYLOAD_BYTES,
            },
            layerx_agentd::prepare::EvmPrepareRequest {
                from: fixture.account.evm_address(),
                call: layerx_agentd::prepare::PrecompileCall::ledger_transfer(
                    prepare.payload.clone(),
                ),
                expected_nonce: Some(prepare.nonce),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: layerx_types::limits::MAX_PAYLOAD_BYTES,
            },
        ));
        prepare.fee_limit = fee_preparation.disclosure.fee_limit;
        let preview = must(fixture.owner.native_send_owner_preview_v1(
            &fixture.peer,
            HumanNativeSendOwnerPreviewRequestV1 {
                credential: fixture.credential.clone(),
                request_id: 71,
                prepare: prepare.clone(),
                owner_public_key: fixture.account.layerx_public_key(),
                purpose_expires_at_ms: prepare.not_after_height,
                commitment: [11; 32],
                economic_action: [8; 32],
                local_grant: local_grant.clone(),
            },
        ));
        let mut input = preview.bytes();
        assert_eq!(&input[..2], &1_u16.to_be_bytes());
        input = &input[2..];
        let canonical = read_bytes(&mut input).to_vec();
        let signing_preimage = read_bytes(&mut input).to_vec();
        let purpose = must(NativeSendPurposeV2::from_canonical_bytes(read_bytes(
            &mut input,
        )));
        assert_eq!(input.len(), 56);
        let observed_head = u64::from_be_bytes(must(input[..8].try_into()));
        assert_eq!(&input[16..48], &fixture.account.layerx_public_key());
        assert_native_owner_resolution(&mut fixture, observed_head);
        assert_eq!(purpose.evm_sender, fixture.account.evm_address());
        assert_eq!(
            purpose.consent_public_key,
            fixture.account.layerx_public_key()
        );
        assert_eq!(purpose.chain_id, 125);
        assert_eq!(purpose.transaction_type, 2);
        let consent_key = ed25519_dalek::SigningKey::from_bytes(fixture.account.layerx_seed());
        let signed = must(sign_native_send_purpose(purpose.clone(), &consent_key));
        let digest: [u8; 32] = Sha256::digest(must(purpose.canonical_bytes())).into();
        must(layerx_crypto::ed25519::verify_digest(
            &purpose.consent_public_key,
            &signed.signature,
            &digest,
        ));
        assert_eq!(
            sign_native_send_purpose(
                purpose.clone(),
                &ed25519_dalek::SigningKey::from_bytes(&[3; 32])
            ),
            Err(EnvelopeError::InvalidCredential)
        );
        assert_eq!(
            purpose.tenant.as_str(),
            fixture.credential.tenant().as_str()
        );
        assert_eq!(
            must(purpose.session_id.to_bytes()),
            fixture.credential.session_id().0
        );
        assert_eq!(purpose.generation, fixture.credential.generation());
        assert_eq!(
            purpose.preparation_id,
            <[u8; 32]>::from(Sha256::digest(&canonical))
        );
        assert_eq!(purpose.canonical_digest, purpose.preparation_id);
        let expected_prepare = json!({
            "variant": "native_send_v1",
            "activity": {"version":"1", "module":"1", "ordinal":"5"},
            "actor": prepare.actor, "authority": prepare.authority,
            "account_sequence": prepare.nonce.to_string(),
            "not_before": prepare.not_before_height.to_string(),
            "not_after": prepare.not_after_height.to_string(),
            "idempotency_key": prepare.idempotency_key,
            "fee_limit": prepare.fee_limit.to_string(),
            "payload": hex::encode(&prepare.payload),
            "payload_hash": hex::encode(prepare.payload_hash),
            "capability_id": hex::encode(prepare.capability_id.expect("preview capability")),
            "local_grant": {
                "version":"1", "capability":hex::encode(&local_grant.capability),
                "session_scope":hex::encode(&local_grant.session_scope),
                "expires_at_ms":local_grant.expires_at_ms.to_string(),
                "owner_public_key":hex::encode(local_grant.owner_public_key),
                "signature":hex::encode(local_grant.signature)
            }
        });
        let request = NativeSendPrepareRequestV2 {
            activity: must(NativeActivity::new(1, 5)),
            actor: purpose.agent_did.clone(),
            authority: prepare.authority,
            account_sequence: prepare.nonce,
            not_before: prepare.not_before_height,
            not_after: prepare.not_after_height,
            idempotency_key: purpose.idempotency_key,
            fee_limit: prepare.fee_limit,
            payload: prepare.payload,
            payload_hash: prepare.payload_hash,
            capability_id: purpose.capability_id.clone(),
            purpose: signed,
            local_grant: Some(local_grant),
        };
        let credential = must(EnvelopeCredential::new(
            fixture.credential.tenant().as_str(),
            fixture.credential.session_id().0,
            fixture.credential.token_id(),
            fixture.credential.generation(),
        ));
        let body = must(encode_native_send_prepare(&request));
        let body_digest: [u8; 32] = Sha256::new()
            .chain_update(layerx_sdk::native_effect::SEND_PREPARE_DOMAIN)
            .chain_update(must(serde_json::to_vec(&body)))
            .finalize()
            .into();
        assert_eq!(must(native_send_prepare_digest(&request)), body_digest);
        let mut actual_prepare = body.clone();
        actual_prepare
            .as_object_mut()
            .expect("prepare body")
            .remove("purpose");
        assert_eq!(
            actual_prepare, expected_prepare,
            "SDK must preserve every preview request field"
        );
        assert_eq!(body["purpose"]["purpose"]["version"], "2");
        assert_eq!(
            body["purpose"].as_object().expect("signed purpose").len(),
            2
        );
        assert_eq!(
            body["purpose"]["purpose"]["consent_public_key"],
            hex::encode(purpose.consent_public_key)
        );
        assert_eq!(
            body["purpose"]["purpose"]["evm_sender"],
            hex::encode(purpose.evm_sender)
        );
        for retired in ["owner_public_key", "protocol_version", "network_id"] {
            assert!(body["purpose"]["purpose"].get(retired).is_none());
        }
        must(layerx_agentd::agent_rpc::validate_mcp_native_request(
            layerx_agentd::tenant::Operation::Prepare,
            &body,
        ));

        let (ca, ca_key) = certificate(None);
        let (cert, key) = certificate(Some((&ca, &ca_key)));
        let roots = fixture.root.join("sdk-rpc-roots.pem");
        must(fs::write(&roots, must(ca.to_pem())));
        let acceptor = must(TlsAcceptor::new(must(Identity::from_pkcs8(
            &must(cert.to_pem()),
            &must(key.private_key_to_pem_pkcs8()),
        ))));
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let endpoint = format!("https://localhost:{}", must(listener.local_addr()).port());
        must(listener.set_nonblocking(true));
        let owner = fixture.owner.clone();
        let expected_body = body.clone();
        let observations = Arc::new(Mutex::new(Vec::new()));
        let observed = Arc::clone(&observations);
        let rpc = Peer::spawn(move |stop| {
            while !stop.load(Ordering::Acquire) {
                let Some(stream) = incoming(&listener) else {
                    continue;
                };
                let mut stream = must(acceptor.accept(stream));
                let (headers, bytes) = http_request(&mut stream);
                assert!(headers.starts_with("POST /v1/agent/rpc HTTP/1.1\r\n"));
                let envelope: Value = must(serde_json::from_slice(&bytes));
                if envelope["request"]["purpose"]["purpose"]["version"] == "2" {
                    assert_eq!(envelope["request"], expected_body);
                }
                let response = layerx_agentd::agent_rpc::handle_rpc(&owner, &bytes);
                let document: Value = must(serde_json::from_slice(&response.body));
                must(observed.lock()).push((response.status, document));
                must(write!(stream, "HTTP/1.1 {} Result\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.status, response.body.len()));
                must(stream.write_all(&response.body));
                must(stream.flush());
            }
        });
        let transport = must(AgentEnvelopeTransport::connect(
            &endpoint,
            None,
            Some(&roots),
        ));
        let transport =
            must(transport.with_native_authority(purpose.consent_public_key, purpose.evm_sender));
        let key = must(Key::new(request.idempotency_key));
        let mut legacy = body;
        let legacy_purpose = legacy["purpose"]["purpose"]
            .as_object_mut()
            .expect("purpose");
        legacy_purpose.insert("version".into(), json!("1"));
        legacy_purpose.insert(
            "owner_public_key".into(),
            json!(hex::encode(purpose.consent_public_key)),
        );
        legacy_purpose.insert("protocol_version".into(), json!("3"));
        legacy_purpose.insert("network_id".into(), json!("125"));
        for field in [
            "consent_public_key",
            "evm_sender",
            "transaction_type",
            "chain_id",
        ] {
            legacy_purpose.remove(field);
        }
        legacy["purpose"]["owner_public_key"] = json!(hex::encode(purpose.consent_public_key));
        let refused = transport.send_operation(
            layerx_sdk::Operation::Prepare,
            RequestId(72),
            &legacy,
            Some(&credential),
            Some(key),
        );
        assert!(
            matches!(refused, Err(EnvelopeError::Refused(ref error)) if error.reason.as_str() == "envelope.malformed"),
            "{refused:?}"
        );
        {
            let observed = must(observations.lock());
            assert_eq!(observed[0].0, 400);
            assert_eq!(observed[0].1["reason"], "envelope.malformed");
        }
        let registry = must(layerx_types::payload::ModuleRegistry::new(&[]));
        let accepted =
            transport.prepare_native_send(RequestId(73), key, &request, &credential, &registry);
        drop(rpc);
        let observed = must(observations.lock());
        assert_eq!(observed.len(), 2);
        assert_eq!(
            observed[1].0, 200,
            "daemon V2 acceptance: {}",
            observed[1].1
        );
        let response = &observed[1].1["value"];
        assert_eq!(response["version"], "1", "version");
        let preparation = layerx_sdk::agent_envelope::decode_native_preparation(response)
            .expect("decode_native_preparation: accepted daemon body");
        assert_eq!(
            preparation.preparation_id, purpose.preparation_id,
            "preparation_id against purpose.preparation_id"
        );
        assert_eq!(
            preparation.preparation_id,
            <[u8; 32]>::from(Sha256::digest(&preparation.canonical_bytes)),
            "preparation_id against SHA256(canonical_bytes)"
        );
        assert_eq!(
            purpose.canonical_digest, preparation.preparation_id,
            "canonical_digest"
        );
        assert_eq!(preparation.activity, request.activity, "activity");
        let call = layerx_crypto::evm_transaction::decode_unsigned(&preparation.canonical_bytes)
            .expect("canonical_bytes: unsigned EIP-1559 call");
        assert_eq!(
            call.encode_unsigned(),
            preparation.canonical_bytes,
            "canonical_bytes re-encoding"
        );
        let disclosure = layerx_crypto::disclosure::call::bind_call(&call, purpose.evm_sender)
            .expect("unsigned call disclosure");
        let chain = must(NodeRpc::connect(&must(NodeRpcConfig::from_environment())));
        let authority: [u8; 32] = must(must(hex::decode(&request.authority)).try_into());
        let sender = must(chain.bound_address(&authority)).expect("request sender binding");
        assert_eq!(disclosure.actor, sender, "sender");
        let actor_account = must(layerx_wire::hash::account_id_for_protocol(
            &must(AccountId::for_asset(
                request.actor.as_str(),
                disclosure.asset,
                disclosure.asset,
            )),
            3,
        ));
        let payer = disclosure
            .counterparties
            .iter()
            .find(|party| party.role == layerx_crypto::disclosure::CounterpartyRole::Payer)
            .expect("actor: payer in unsigned call");
        assert_eq!(payer.account, actor_account, "actor");
        assert_eq!(call.nonce, request.account_sequence, "account_sequence");
        assert_eq!(
            disclosure.activity_type,
            must(request.activity.activity_type()),
            "unsigned call activity"
        );
        assert_eq!(
            disclosure.expiry.not_before, 0,
            "not_before: ledger send carries no activity window"
        );
        assert_eq!(
            disclosure.expiry.not_after,
            u64::MAX,
            "not_after: ledger send carries no activity window"
        );
        assert_eq!(
            disclosure.expiry.payload_expires_at, payload_expires_at,
            "payload_expires_at: expiry of the send envelope signed by the fixture"
        );
        assert_eq!(
            disclosure.idempotency_key, request.idempotency_key,
            "idempotency_key"
        );
        assert_eq!(disclosure.fee_limit, request.fee_limit, "fee_limit");
        let method = must(disclosure.construct_method()).expect("payload: construct method");
        let layerx_crypto::disclosure::call::ConstructMethod::LedgerSend { payload } = method
        else {
            panic!("payload: expected LedgerSend");
        };
        assert_eq!(payload, request.payload, "payload");
        assert_eq!(
            <[u8; 32]>::from(
                Sha256::new()
                    .chain_update(layerx_wire::hash::Domain::PayloadHash.tag())
                    .chain_update(&payload)
                    .finalize()
            ),
            request.payload_hash,
            "payload_hash"
        );
        assert_eq!(
            preparation.signing_preimage,
            call.signing_hash(),
            "signing_preimage"
        );
        assert_eq!(call.chain_id, purpose.chain_id, "chain_id");
        assert_eq!(
            preparation.canonical_bytes[0], purpose.transaction_type,
            "transaction_type"
        );
        assert_eq!(
            preparation.approval_id,
            preparation
                .approval_required
                .then_some(purpose.preparation_id),
            "approval_id"
        );
        let accepted = must(accepted);
        assert_eq!(accepted.value.canonical_bytes, canonical);
        assert_eq!(accepted.value.preparation_id, purpose.preparation_id);
        assert_eq!(accepted.value.signing_preimage.as_slice(), signing_preimage);
        assert_eq!(fixture.writes.load(Ordering::SeqCst), 0);
    });
}
