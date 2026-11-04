//! Audit 22.30.98: retained calls and bind consent through production signers,
//! transaction codecs, encrypted key recovery and bounded loopback peers.

use std::future::Future;
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};

use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::authority_grant::{authority_grant_call, AuthorityGrant, GrantKind};
use layerx_crypto::disclosure::call::{
    bind_call, encode_program_call, CallFeeContext, ExecutionFeeBound, GasFeeBound,
};
use layerx_crypto::disclosure::AmountRole;
use layerx_crypto::evm_transaction::{assemble, decode_signed, recover_sender, Eip1559Call};
use layerx_crypto::keystore::{Keystore, KeystoreEntropy};
use layerx_crypto::send::{
    encode_send_envelope, ledger_payment_call, send_context_hash, CallOptions, EnvelopeOptions,
    SendDebit,
};
use layerx_crypto::signer::{
    sign_disclosed, sign_disclosed_call, BindMessage, BindSigner, KeystoreSigner, LocalSigner,
    SignError, Signer, SigningRequest,
};
use layerx_types::intent::ProgramId;
use layerx_types::program_call::{NativeProgramCall, Resources};
use layerx_wire::hash::Domain;
use sha2::{Digest as _, Sha256};

#[path = "authority_grant.rs"]
mod authority_fixtures;
#[path = "disclosure_call.rs"]
mod call_fixtures;
#[path = "disclosure.rs"]
mod disclosure_fixtures;
#[path = "native_fee_grant.rs"]
mod native_fee_fixtures;
#[path = "remote.rs"]
mod remote_fixtures;
mod support;

struct NoopWake;
impl Wake for NoopWake {
    fn wake(self: Arc<Self>) {}
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(NoopWake));
    match future.as_mut().poll(&mut Context::from_waker(&waker)) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("synchronous signer unexpectedly pending"),
    }
}

fn debit(amount: u128) -> SendDebit {
    SendDebit {
        from: [1; 32],
        to: [2; 32],
        asset: [3; 32],
        amount,
        source_sequence: 9,
        idempotency_key: [4; 32],
        expires_at: 100,
        context_hash: send_context_hash(&[1; 32], &[2; 32], &[3; 32], amount, &[4; 32]),
        conditions: vec![],
        authorization_kind: 1,
        network_id: 17,
        protocol_version: 3,
    }
}

fn options(from: [u8; 20]) -> CallOptions {
    CallOptions {
        from,
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
    }
}

fn old_envelope(public_key: [u8; 32]) -> layerx_crypto::send::DisclosedEnvelope {
    let payer = LocalSigner::new([5; 32]);
    let payload = ready(debit(123).sign(&payer)).expect("real payer authorization");
    encode_send_envelope(
        &payload,
        &EnvelopeOptions {
            actor: "did:layerx:crypto-repair",
            public_key,
            protocol_version: 3,
            network_id: 17,
            identity_sequence: 7,
            idempotency_key: [4; 32],
            fee_limit: 20,
            not_before: 10,
            not_after: 100,
        },
    )
    .expect("production legacy producer for refusal coverage")
}

#[test]
fn disclosure_activity_success_tests_use_sender_bound_evm_calls() {
    let account = derive_from_seed(&[7; 64], 0).expect("derived account");
    let signer = LocalSigner::from_derived_account(&account);
    let from = signer.evm_address().expect("transaction key");
    let payload = ready(debit(123).sign(&signer)).expect("real signed debit");
    let produced = ledger_payment_call(&payload, &options(from)).expect("real ledger producer");
    let consumed = bind_call(&produced.call, from).expect("production call consumer");
    assert_eq!(consumed, produced.disclosure);
    assert_eq!(consumed.actor, from);
    assert_eq!(consumed.authority, from);
    assert_eq!(consumed.asset, [3; 32]);
    assert_eq!(consumed.amounts[0].role, AmountRole::Transfer);
    assert_eq!(consumed.amounts[0].value, 123);
    assert_eq!(consumed.counterparties[0].account, [1; 32]);
    assert_eq!(consumed.counterparties[1].account, [2; 32]);
    assert_eq!(consumed.payload_sequence(), Ok(Some(9)));
    let signature = ready(sign_disclosed_call(
        &signer,
        &produced.call,
        &consumed,
        from,
    ))
    .expect("sender-bound transaction signature");
    let (call, signature) = decode_signed(&assemble(&produced.call, &signature))
        .expect("transaction transport round trip");
    assert_eq!(recover_sender(&call, &signature), Ok(from));
    assert_eq!(bind_call(&call, from), Ok(consumed.clone()));
    let wrong =
        LocalSigner::from_derived_account(&derive_from_seed(&[8; 64], 0).expect("other key"));
    assert_eq!(
        ready(sign_disclosed_call(&wrong, &call, &consumed, from)),
        Err(SignError::DisclosureMismatch("signer"))
    );
    let mut substituted = consumed.clone();
    substituted.amounts[0].value = 1;
    assert_eq!(
        ready(sign_disclosed_call(&signer, &call, &substituted, from)),
        Err(SignError::DisclosureMismatch("call"))
    );
    for length in 0..call.data.len() {
        let mut malformed = call.clone();
        malformed.data.truncate(length);
        assert!(
            bind_call(&malformed, from).is_err(),
            "truncated at {length}"
        );
    }
    let mut malformed = call;
    malformed.data.push(0);
    assert!(bind_call(&malformed, from).is_err());
}

#[test]
fn disclosure_activity_success_tests_refuse_retired_activity_signing() {
    let signer = LocalSigner::new([5; 32]);
    let old = old_envelope(signer.public_key());
    let message =
        layerx_crypto::SignatureMessage::new(Domain::SignaturePreimage, 3, 17, &old.canonical)
            .expect("legacy scope");
    assert_eq!(
        SigningRequest::new(message, &old.disclosure).err(),
        Some(SignError::InvalidDisclosure)
    );
    assert!(BindMessage::decode(&old.canonical).is_err());
    assert_eq!(
        ready(sign_disclosed(
            &signer,
            &old.canonical,
            &old.disclosure,
            &old.registry
        )),
        Err(SignError::InvalidDisclosure),
        "req.9341: the real local signer must refuse the retired activity preimage"
    );
}

#[test]
fn authority_fixture_old_wrapper_preserves_native_grant_scope() {
    for (bytes, kind) in [
        (
            include_bytes!("fixtures/authority-grant-capability.bin").as_slice(),
            GrantKind::DelegatedCapability,
        ),
        (
            include_bytes!("fixtures/authority-grant-budget.bin").as_slice(),
            GrantKind::BudgetAllowance,
        ),
    ] {
        let grant = AuthorityGrant::decode(bytes).expect("native grant vector");
        assert_eq!(grant.kind, kind);
        assert_eq!(grant.encode(), Ok(bytes.to_vec()));
        assert_eq!(grant.grantee, grant.grantor);
        assert_eq!(grant.scope.module_mask, 1 << 7);
        assert_eq!((grant.scope.ordinal_min, grant.scope.ordinal_max), (1, 8));
        assert_eq!(grant.scope.maximum_per_activity, 10);
        assert_eq!(grant.scope.maximum_total, 30);
        assert_eq!(grant.revocation_sequence, 7);
        let options = options(support::call_options(7).from);
        let produced =
            authority_grant_call(&grant, &options).expect("merged authorization producer");
        let signer = support::signer();
        let signature = ready(sign_disclosed_call(
            &signer,
            &produced.call,
            &produced.disclosure,
            options.from,
        ))
        .expect("owner transaction");
        let (received, signature) = decode_signed(&assemble(&produced.call, &signature))
            .expect("real transaction transport");
        assert_eq!(recover_sender(&received, &signature), Ok(options.from));
        let bound = bind_call(&received, options.from).expect("authorization consumer");
        assert_eq!(bound.authority_grant, Some(grant));
        assert_eq!(bound.authority, options.from);
        assert_eq!(bound.expiry.payload_expires_at, grant.not_after);
        let payload = grant.payload().expect("native authorization payload");
        assert_eq!(AuthorityGrant::from_payload(&payload), Ok(grant));
        for length in 0..payload.len() {
            assert!(AuthorityGrant::from_payload(&payload[..length]).is_err());
        }
        let mut extended = payload;
        extended.push(0);
        assert!(AuthorityGrant::from_payload(&extended).is_err());
    }
}

#[test]
fn native_fee_grant_activity_test_preserves_raw_grant_identity() {
    macro_rules! vector {
        ($name:literal) => {
            (
                include_bytes!(concat!(
                    "../../../../../tests/fixtures/authority/native-fee-grants/",
                    $name,
                    "/grant.bin"
                ))
                .as_slice(),
                include_bytes!(concat!(
                    "../../../../../tests/fixtures/authority/native-fee-grants/",
                    $name,
                    "/grant.id"
                )),
            )
        };
    }
    for (bytes, id) in [
        vector!("capability"),
        vector!("budget"),
        vector!("asset-mismatch"),
        vector!("total-bound"),
        vector!("period-bound"),
    ] {
        let grant = AuthorityGrant::decode(bytes).expect("native fee grant");
        assert_eq!(grant.encode(), Ok(bytes.to_vec()));
        assert_eq!(bytes[4], 2);
        let mut hasher = Sha256::new();
        hasher.update(Domain::AuthorityHash.tag());
        hasher.update(bytes);
        assert_eq!(hasher.finalize().as_slice(), id);
        let options = support::call_options(7);
        let produced = authority_grant_call(&grant, &options).expect("native fee grant producer");
        let signature = ready(sign_disclosed_call(
            &support::signer(),
            &produced.call,
            &produced.disclosure,
            options.from,
        ))
        .expect("grant transaction");
        let (received, signature) = decode_signed(&assemble(&produced.call, &signature))
            .expect("grant transaction transport");
        assert_eq!(recover_sender(&received, &signature), Ok(options.from));
        let bound = bind_call(&received, options.from).expect("grant consumer");
        assert_eq!(bound.authority_grant, Some(grant));
        assert_eq!(bound.authority, options.from);
        let mut pin = Sha256::new();
        pin.update(Domain::AuthorityHash.tag());
        pin.update(
            bound
                .authority_grant
                .expect("native identity")
                .encode()
                .expect("body"),
        );
        assert_eq!(pin.finalize().as_slice(), id);
        assert!(grant.fee_budget.is_some());
        for length in 0..bytes.len() {
            assert!(AuthorityGrant::decode(&bytes[..length]).is_err());
        }
    }
}

#[test]
fn monetary_call_tests_miss_common_view_program_execution_fee() {
    let native = NativeProgramCall {
        program_id: ProgramId::new([0x31; 32]),
        guest_abi: 5,
        entrypoint: b"layerx_call",
        calldata: &[1, 2, 3, 4],
        capabilities: &[0, 0],
        access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
        response_capacity: 16,
        resources: Resources([1000, 2000, 3000, 4, 5, 6, 7]),
    };
    let execution_fee = u128::MAX;
    let mut to = [0; 20];
    to[18..].copy_from_slice(&0x101d_u16.to_be_bytes());
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
        to,
        data: encode_program_call(&native, execution_fee),
    };
    let disclosure = bind_call(&call, [0xa1; 20]).expect("real program producer to call binder");
    disclosure
        .validate()
        .expect("monetary consumer validation boundary");
    assert_eq!(disclosure.fee_limit, 3_000_000);
    let view = disclosure
        .call_monetary(CallFeeContext::ProgramExecution {
            fee_asset: [0x72; 32],
        })
        .expect("complete monetary consumer");
    assert_eq!(
        view.gas_fee_bound,
        GasFeeBound {
            chain_id: 125,
            amount_wei: 3_000_000
        }
    );
    assert_eq!(
        view.execution_fee_bound,
        Some(ExecutionFeeBound {
            asset: [0x72; 32],
            amount_base_units: execution_fee
        })
    );
    assert!(view.to_string().contains(&execution_fee.to_string()));
    assert!(disclosure.call_monetary(CallFeeContext::GasOnly).is_err());
}

#[test]
fn ssh_agent_test_expects_old_signature_refuses_before_contact() {
    let key = LocalSigner::new([5; 32]).public_key();
    // No peer is listening: only a refusal before transport contact is valid.
    let signer = KeystoreSigner::new("/nonexistent/layerx-repair-22-33-1560.sock", key)
        .expect("external public key");
    let old = old_envelope(key);
    assert_eq!(
        ready(signer.sign_bind_bytes(&old.canonical)),
        Err(SignError::InvalidDisclosure)
    );
    assert_eq!(
        ready(sign_disclosed(
            &signer,
            &old.canonical,
            &old.disclosure,
            &old.registry
        )),
        Err(SignError::InvalidDisclosure),
        "activity refusal must precede SSH-agent connection or signing"
    );
}

#[test]
fn reopened_agent_signs_old_activity_refuses_before_and_after_key_recovery() {
    let store = Keystore::seal(
        &[5; 32],
        b"fixture operator secret",
        b"repair-key",
        17,
        KeystoreEntropy::new([1; 16], [2; 24]).expect("fixture entropy"),
    )
    .expect("production encrypted store");
    let persisted = store.to_bytes().expect("persisted state");
    let first = store
        .open_with(b"fixture operator secret", b"repair-key", 17, |seed| {
            LocalSigner::new(*seed)
        })
        .expect("initial key opening");
    let binding = support::signing::binding();
    let first_signature = ready(first.sign_bind(&binding)).expect("bind before key recovery");
    assert_eq!(
        binding.verify(&first.public_key(), first_signature.as_bytes()),
        Ok(())
    );
    let public_key = first.public_key();
    let old = old_envelope(public_key);
    let before = ready(sign_disclosed(
        &first,
        &old.canonical,
        &old.disclosure,
        &old.registry,
    ));
    drop(first);
    drop(store);
    let reopened = Keystore::from_bytes(&persisted).expect("reopen production state");
    assert_eq!(reopened.to_bytes(), Ok(persisted));
    let recovered = reopened
        .open_with(b"fixture operator secret", b"repair-key", 17, |seed| {
            LocalSigner::new(*seed)
        })
        .expect("recover key after reopening");
    assert_eq!(recovered.public_key(), public_key);
    let recovered_signature =
        ready(recovered.sign_bind(&binding)).expect("bind after key recovery");
    assert_eq!(recovered_signature, first_signature);
    assert_eq!(
        binding.verify(&recovered.public_key(), recovered_signature.as_bytes()),
        Ok(())
    );
    let after = ready(sign_disclosed(
        &recovered,
        &old.canonical,
        &old.disclosure,
        &old.registry,
    ));
    assert_eq!(
        (before, after),
        (
            Err(SignError::InvalidDisclosure),
            Err(SignError::InvalidDisclosure)
        ),
        "retired activity signing must remain refused across recovery"
    );
}

#[test]
fn remote_success_fixture_is_activity_refuses_before_remote_transport() {
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_crypto::remote::{decode_bind_request, RemoteSigner};
    use openssl::ssl::{SslAcceptor, SslFiletype, SslMethod, SslVerifyMode};
    use std::io::{Read, Write};
    use std::net::TcpListener;
    use std::sync::atomic::{AtomicUsize, Ordering};
    use std::time::Duration;
    let certificates = support::signing::Certificates::create();
    let mut acceptor =
        SslAcceptor::mozilla_intermediate(SslMethod::tls_server()).expect("TLS acceptor");
    acceptor
        .set_certificate_chain_file(certificates.path("server.pem"))
        .expect("server cert");
    acceptor
        .set_private_key_file(certificates.path("server.key"), SslFiletype::PEM)
        .expect("server key");
    acceptor
        .set_ca_file(certificates.path("ca.pem"))
        .expect("client pin");
    acceptor.set_verify(SslVerifyMode::PEER | SslVerifyMode::FAIL_IF_NO_PEER_CERT);
    let acceptor = acceptor.build();
    let listener = TcpListener::bind("127.0.0.1:0").expect("loopback TLS peer");
    listener.set_nonblocking(true).expect("bounded accept");
    let address = listener.local_addr().expect("peer endpoint");
    let requests = Arc::new(AtomicUsize::new(0));
    let observed = Arc::clone(&requests);
    let peer = std::thread::spawn(move || {
        let deadline = std::time::Instant::now() + Duration::from_secs(3);
        let stream = loop {
            match listener.accept() {
                Ok((stream, _)) => break stream,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(std::time::Instant::now() < deadline, "remote peer deadline");
                    std::thread::sleep(Duration::from_millis(5));
                }
                Err(error) => panic!("accept: {error}"),
            }
        };
        observed.fetch_add(1, Ordering::SeqCst);
        stream
            .set_read_timeout(Some(Duration::from_secs(2)))
            .expect("read deadline");
        stream
            .set_write_timeout(Some(Duration::from_secs(2)))
            .expect("write deadline");
        let mut stream = acceptor.accept(stream).expect("mutual TLS");
        let mut length = [0; 4];
        stream.read_exact(&mut length).expect("request frame");
        assert_eq!(u32::from_be_bytes(length), 84);
        let mut body = [0; 84];
        stream.read_exact(&mut body).expect("bind request");
        let message = decode_bind_request(&body).expect("production server decoder");
        assert_eq!(message, support::signing::binding());
        let signature = SigningKey::from_bytes(&[0x67; 32]).sign(message.as_bytes());
        let mut response = vec![0, 0, 0, 65, 0];
        response.extend_from_slice(&signature.to_bytes());
        stream.write_all(&response).expect("signature frame");
        stream.flush().expect("TLS flush");
    });
    let key = ed25519_dalek::SigningKey::from_bytes(&[0x67; 32])
        .verifying_key()
        .to_bytes();
    let signer = RemoteSigner::new(
        address,
        "localhost",
        key,
        certificates.path("ca.pem"),
        certificates.path("client.pem"),
        certificates.path("client.key"),
        Duration::from_secs(2),
    )
    .expect("remote signer");
    let old = old_envelope(key);
    let result = ready(sign_disclosed(
        &signer,
        &old.canonical,
        &old.disclosure,
        &old.registry,
    ));
    assert_eq!(
        (result, requests.load(Ordering::SeqCst)),
        (Err(SignError::InvalidDisclosure), 0),
        "activity refusal precedes TLS contact"
    );
    let binding = support::signing::binding();
    let signature = ready(signer.sign_bind(&binding)).expect("retained bind over mutual TLS");
    assert_eq!(binding.verify(&key, signature.as_bytes()), Ok(()));
    peer.join().expect("TLS peer stopped");
    assert_eq!(requests.load(Ordering::SeqCst), 1);
}

#[test]
fn retained_bind_signing_preserves_raw_chain_address_nonce_scope() {
    let signer = LocalSigner::new([0x61; 32]);
    let consent = support::paxeer_binding::Binding::new(713_714, [0xa1; 20], 7);
    let binding = BindMessage::decode(&consent.message()).expect("real bind producer");
    let signature = ready(signer.sign_bind(&binding)).expect("raw bind signature");
    assert_eq!(*signature.as_bytes(), consent.sign(&[0x61; 32]).signature());
    assert_eq!(
        consent.verify(&signer.public_key(), signature.as_bytes()),
        Ok(())
    );
    for index in [17, 48, 49, 68, 69, 76] {
        let mut changed = *binding.as_bytes();
        changed[index] ^= 1;
        assert!(BindMessage::decode(&changed)
            .expect("different scoped message")
            .verify(&signer.public_key(), signature.as_bytes())
            .is_err());
    }
    for length in 0..77 {
        assert!(BindMessage::decode(&binding.as_bytes()[..length]).is_err());
    }
    let mut extra = binding.as_bytes().to_vec();
    extra.push(0);
    assert!(BindMessage::decode(&extra).is_err());
    let mut wrong = *binding.as_bytes();
    wrong[0] ^= 1;
    assert!(BindMessage::decode(&wrong).is_err());
    let mut chain = [0; 32];
    chain[0] = 1;
    chain[31] = 125;
    let wide = BindMessage::new(chain, [0xa1; 20], 7);
    assert_eq!(
        wide.as_bytes(),
        &support::paxeer_binding::Binding::with_chain_word(chain, [0xa1; 20], 7).message()
    );
}

#[test]
fn remote_bind_codec_refuses_retired_domains_and_malformed_frames() {
    let mut request = b"LXRS\0\x02\x01".to_vec();
    request.extend_from_slice(support::signing::binding().as_bytes());
    assert_eq!(
        layerx_crypto::remote::decode_bind_request(&request),
        Ok(support::signing::binding())
    );
    for length in 0..request.len() {
        assert!(layerx_crypto::remote::decode_bind_request(&request[..length]).is_err());
    }
    let mut extra = request.clone();
    extra.push(0);
    assert!(layerx_crypto::remote::decode_bind_request(&extra).is_err());
    for index in [0, 4, 5, 6, 7] {
        let mut changed = request.clone();
        changed[index] ^= 1;
        assert!(layerx_crypto::remote::decode_bind_request(&changed).is_err());
    }
}

#[cfg(unix)]
#[test]
fn retained_bind_external_key_recovers_across_agent_restart() {
    use std::sync::atomic::Ordering;
    let store = Keystore::seal(
        &[0x67; 32],
        b"fixture secret",
        b"repair-key",
        17,
        KeystoreEntropy::new([1; 16], [2; 24]).expect("entropy"),
    )
    .expect("real persisted external key");
    let persisted = store.to_bytes().expect("persisted key bytes");
    drop(store);
    let binding = support::signing::binding();
    let mut previous = None;
    for _ in 0..2 {
        let peer = support::signing::AgentPeer::open(&persisted);
        let signer = KeystoreSigner::new(peer.socket_path(), peer.public_key)
            .expect("recovered external identity");
        let old = old_envelope(peer.public_key);
        assert_eq!(
            ready(sign_disclosed(
                &signer,
                &old.canonical,
                &old.disclosure,
                &old.registry
            )),
            Err(SignError::InvalidDisclosure)
        );
        assert_eq!(
            ready(signer.sign_bind_bytes(&old.canonical)),
            Err(SignError::InvalidDisclosure)
        );
        assert_eq!(peer.attempts.load(Ordering::SeqCst), 0);
        let signature = ready(signer.sign_bind(&binding)).expect("bind via real SSH codec");
        assert_eq!(
            binding.verify(&peer.public_key, signature.as_bytes()),
            Ok(())
        );
        if let Some(previous) = previous {
            assert_eq!(signature, previous);
        }
        previous = Some(signature);
        let unknown =
            KeystoreSigner::new(peer.socket_path(), LocalSigner::new([8; 32]).public_key())
                .expect("unknown key request");
        assert_eq!(
            ready(unknown.sign_bind(&binding)),
            Err(SignError::KeystoreRefused)
        );
        assert_eq!(peer.attempts.load(Ordering::SeqCst), 2);
        drop(peer);
        assert_eq!(
            ready(sign_disclosed(
                &signer,
                &old.canonical,
                &old.disclosure,
                &old.registry
            )),
            Err(SignError::InvalidDisclosure)
        );
        assert_eq!(
            ready(signer.sign_bind(&binding)),
            Err(SignError::KeystoreUnavailable)
        );
    }
}
