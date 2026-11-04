//! Task 22.33.27, audit 22.30.102, acceptance 22998.1.
//! provider-private-address-fixture: provider_public_wallet_isolation_and_restart.
//! export-loss-test: export_envelope_disconnect_restart_acknowledgement_and_isolation.
//! eip1559-private-address-fixture: eip1559_public_sender_and_negative_envelopes.
//! Baseline: not run; the explicit bounded gate order forbids unchanged-tree verification.
//! The gate uses production KMS server/store/codec sources and the real remote client.
//! Loopback signing peers use the production EIP-1559 producer with independent fixture
//! keys; threshold execution is covered by provider.rs and sign_eip1559_requests.rs
//! against the real attestor cluster, outside this dependency-free focused gate.
//! Wiring request (Human domain): services/human/crates/layerx-human-service/src/custody/mod.rs::export_primary_once must retain a recipient secret and use v8 export/acknowledgement instead of the retired raw-seed API.
//! The user forbids spec edits; this module is the finding-to-test task record.

#![allow(dead_code)]

pub use layerx_human_kms::attestor;
#[path = "../src/config.rs"]
mod config;
#[path = "../src/evm.rs"]
mod evm;
#[path = "../src/evm_types.rs"]
mod evm_types;
#[path = "../src/export.rs"]
mod export;
#[path = "../src/send.rs"]
mod send;
#[path = "../src/store.rs"]
mod store;
#[path = "../src/wire.rs"]
mod wire;
mod server {
    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/src/server.rs"));

    pub fn serve_connection(
        tcp: std::net::TcpStream,
        config: &crate::config::Config,
        store: &std::sync::Mutex<crate::store::Store>,
        clock: &dyn layerx_types::clock::Clock,
    ) -> crate::wire::Result<()> {
        connection(tcp, config, store, clock)
    }
}
#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;
#[path = "support/repair_22_33_27_host.rs"]
mod fixture;

use fixture::{checked, Host, Result, SERIAL};
use layerx_crypto::disclosure::call::bind_call;
use layerx_crypto::evm_transaction::{recover_sender, Eip1559Call};
use layerx_crypto::signer::AgentSignature;
use layerx_human_kms::export::{ExportAcknowledgement, ExportRecipient};
use layerx_human_service::custody::{
    KeyClass, KmsError, KmsProvider, PrincipalKeyBinding, ProviderKeyReference,
};
use layerx_types::clock::Deadline;
use ring::signature::KeyPair;
use std::io::Write;
use std::time::Duration;

fn call(nonce: u64) -> Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1a]);
    let mut data =
        layerx_crypto::disclosure::call::ledger_selector("openAccount(bytes32)").to_vec();
    data.extend([3; 32]);
    Eip1559Call {
        chain_id: 125,
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to,
        data,
    }
}

#[test]
fn provider_public_wallet_isolation_and_restart() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
    let mut host = Host::new()?;
    let remote = host.remote(false)?;
    let mut descriptions = Vec::new();
    let mut wallets = Vec::new();
    for binding in &host.bindings {
        let description = remote.create_key(binding)?;
        assert_eq!(
            remote.describe_key(binding, description.reference())?,
            description
        );
        let descriptor = remote.wallet_descriptor(binding, description.reference())?;
        assert_eq!(descriptor.native_ed25519, description.public_key());
        assert_ne!(descriptor.evm_secp256k1, [0; 20]);
        let call = call(0);
        let disclosure = checked(bind_call(&call, descriptor.evm_secp256k1))?;
        let signature = fixture::run(remote.sign_wallet_call(
            binding,
            description.reference(),
            &call,
            &disclosure,
        ))?;
        assert_eq!(
            recover_sender(&call, &checked(AgentSignature::new(signature))?)?,
            descriptor.evm_secp256k1
        );
        descriptions.push(description);
        wallets.push(descriptor);
    }
    assert_ne!(wallets[0].evm_secp256k1, wallets[1].evm_secp256k1);
    assert!(remote
        .wallet_descriptor(&host.bindings[1], descriptions[0].reference())
        .is_err());
    assert!(fixture::run(remote.sign_wallet_call(
        &host.bindings[1],
        descriptions[0].reference(),
        &call(1),
        &checked(bind_call(&call(1), wallets[0].evm_secp256k1))?
    ))
    .is_err());
    host.restart()?;
    let remote = host.remote(false)?;
    for (index, binding) in host.bindings.iter().enumerate() {
        assert_eq!(
            remote.describe_key(binding, descriptions[index].reference())?,
            descriptions[index]
        );
        assert_eq!(
            remote.wallet_descriptor(binding, descriptions[index].reference())?,
            wallets[index]
        );
        let call = call(1);
        let disclosure = checked(bind_call(&call, wallets[index].evm_secp256k1))?;
        let signature = fixture::run(remote.sign_wallet_call(
            binding,
            descriptions[index].reference(),
            &call,
            &disclosure,
        ))?;
        assert_eq!(
            recover_sender(&call, &checked(AgentSignature::new(signature))?)?,
            wallets[index].evm_secp256k1
        );
    }
    Ok(())
}

#[test]
fn eip1559_public_sender_and_negative_envelopes() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
    let host = Host::new()?;
    let remote = host.remote(false)?;
    let binding = &host.bindings[0];
    let description = remote.create_key(binding)?;
    assert_eq!(
        remote.describe_key(binding, description.reference())?,
        description
    );
    let wallet = remote.wallet_descriptor(binding, description.reference())?;
    let call = call(7);
    let canonical = attestor::unsigned_evm_transaction(&call);
    assert_eq!(attestor::decode_unsigned_evm_transaction(&canonical)?, call);
    let disclosure = checked(bind_call(&call, wallet.evm_secp256k1))?;
    let encoded = checked(wire::disclosure(&disclosure))?;
    let sign = host.sign_frame(
        binding,
        description.reference(),
        wallet.evm_secp256k1,
        &canonical,
        &encoded,
    )?;
    let response = host.exchange(&sign, false)?;
    assert_eq!(&response[..8], b"LXKP\0\x07\x05\0");
    assert_eq!(response.len(), 73);
    let signature = checked(AgentSignature::new(response[8..].try_into()?))?;
    assert_eq!(recover_sender(&call, &signature)?, wallet.evm_secp256k1);

    for mutate in [
        (|v: &mut layerx_crypto::disclosure::Disclosure| v.asset[0] ^= 1) as fn(&mut _),
        |v| v.fee_limit += 1,
        |v| v.authority[0] ^= 1,
    ] {
        let mut changed = disclosure.clone();
        mutate(&mut changed);
        let frame = host.sign_frame(
            binding,
            description.reference(),
            wallet.evm_secp256k1,
            &canonical,
            &checked(wire::disclosure(&changed))?,
        )?;
        assert_eq!(host.exchange(&frame, false)?[7], 1);
    }
    for mutate in [
        (|v: &mut Eip1559Call| v.chain_id = 126) as fn(&mut _),
        |v| v.to[0] ^= 1,
        |v| v.data[0] ^= 1,
        |v| v.data[4] ^= 1,
    ] {
        let mut changed = call.clone();
        mutate(&mut changed);
        let frame = host.sign_frame(
            binding,
            description.reference(),
            wallet.evm_secp256k1,
            &attestor::unsigned_evm_transaction(&changed),
            &encoded,
        )?;
        assert_eq!(host.exchange(&frame, false)?[7], 1);
    }
    let mut value = canonical.clone();
    let destination = value
        .windows(21)
        .position(|v| v[0] == 0x94 && v[1..] == call.to)
        .ok_or("destination missing")?;
    assert_eq!(value[destination + 21], 0x80);
    value[destination + 21] = 1;
    let mut trailing = canonical.clone();
    trailing.push(0);
    let mut legacy = canonical.clone();
    legacy[0] = 1;
    for invalid in [
        call.sign(&[1; 32])?,
        vec![2, 0xc0],
        trailing,
        legacy,
        value,
        b"not a type-2 transaction".to_vec(),
    ] {
        let frame = host.sign_frame(
            binding,
            description.reference(),
            wallet.evm_secp256k1,
            &invalid,
            &encoded,
        )?;
        assert_eq!(host.exchange(&frame, false)?[7], 1);
    }
    let mut bad_digest = sign.clone();
    let digest_at = bad_digest.len() - canonical.len() - encoded.len() - 8 - 20 - 32;
    bad_digest[digest_at] ^= 1;
    assert_eq!(host.exchange(&bad_digest, false)?[7], 5);
    let mut wrong_network = sign.clone();
    let decoded = checked(wire::Request::decode(&sign))?;
    let network_at = 11 + decoded.provider.len() + 32;
    wrong_network[network_at + 3] ^= 1;
    assert_eq!(host.exchange(&wrong_network, false)?[7], 1);
    let mut wrong_sender = wallet.evm_secp256k1;
    wrong_sender[0] ^= 1;
    let changed = checked(bind_call(&call, wrong_sender))?;
    assert_eq!(
        host.exchange(
            &host.sign_frame(
                binding,
                description.reference(),
                wrong_sender,
                &canonical,
                &checked(wire::disclosure(&changed))?
            )?,
            false
        )?[7],
        5
    );
    let zero = host.sign_frame(
        binding,
        description.reference(),
        [0; 20],
        &canonical,
        &encoded,
    )?;
    assert!(matches!(
        wire::Request::decode(&zero),
        Err(wire::Error::Refused)
    ));
    assert!(host.exchange(&zero, false).is_err());
    let mut v1 = sign;
    v1[4..6].copy_from_slice(&1_u16.to_be_bytes());
    assert!(matches!(
        wire::Request::decode(&v1),
        Err(wire::Error::Refused)
    ));
    assert!(host.exchange(&v1, false).is_err());
    Ok(())
}

#[test]
fn export_envelope_disconnect_restart_acknowledgement_and_isolation() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
    let mut host = Host::new()?;
    let remote = host.remote(false)?;
    let binding = host.bindings[0].clone();
    let other = host.bindings[1].clone();
    let description = remote.create_key(&binding)?;
    let other_description = remote.create_key(&other)?;
    let reference = description.reference();
    let wallet = remote.wallet_descriptor(&binding, reference)?;
    let transaction = call(0);
    let sign = host.sign_frame(
        &binding,
        reference,
        wallet.evm_secp256k1,
        &attestor::unsigned_evm_transaction(&transaction),
        &checked(wire::disclosure(&checked(bind_call(
            &transaction,
            wallet.evm_secp256k1,
        ))?))?,
    )?;
    assert_eq!(host.exchange(&sign, false)?[7], 0);
    let recipient = ExportRecipient::new([11; 32])?;
    let ceremony = recipient.ceremony([12; 32])?;
    let frame = remote.encode_export_request(&binding, reference, &ceremony)?;
    // A partial transport frame cannot disable managed authority.
    {
        let mut tls = host.connection(false)?;
        tls.write_all(&u32::try_from(frame.len())?.to_be_bytes())?;
        tls.write_all(&frame[..frame.len() / 2])?;
        tls.flush()?;
    }
    host.restart()?;
    assert_eq!(remote.describe_key(&binding, reference)?, description);
    assert_eq!(remote.wallet_descriptor(&binding, reference)?, wallet);
    assert_eq!(host.exchange(&sign, false)?[7], 0);
    assert_eq!(
        host.remote(true)?
            .export_envelope(&binding, reference, &ceremony),
        Err(KmsError::Refused)
    );
    assert_eq!(
        remote.export_primary_key(&binding, reference).err(),
        Some(KmsError::Refused)
    );
    let agent = PrincipalKeyBinding::from_digest([92; 32], 125, KeyClass::AgentPrimary)?;
    let agent_description = remote.create_key(&agent)?;
    assert_eq!(
        remote.export_envelope(&agent, agent_description.reference(), &ceremony),
        Err(KmsError::Refused)
    );
    let wrong_class =
        PrincipalKeyBinding::from_digest(binding.digest(), 125, KeyClass::AgentPrimary)?;
    assert_eq!(
        remote.export_envelope(&wrong_class, reference, &ceremony),
        Err(KmsError::Integrity)
    );
    assert_eq!(
        remote.export_envelope(
            &binding,
            &ProviderKeyReference::new(vec![33; 32])?,
            &ceremony
        ),
        Err(KmsError::Integrity)
    );
    let missing = PrincipalKeyBinding::from_digest([93; 32], 125, KeyClass::HumanPrimary)?;
    assert_eq!(
        remote.export_envelope(&missing, reference, &ceremony),
        Err(KmsError::KeyNotFound)
    );
    for version in [1u16, 3, 5] {
        let mut invalid = frame.clone();
        invalid[4..6].copy_from_slice(&version.to_be_bytes());
        assert!(host.exchange(&invalid, false).is_err());
    }
    // Refuse delivery until the durable store write succeeds, including after reopen.
    let blocker = host.root.join("state/state.next");
    std::fs::create_dir(&blocker)?;
    assert_eq!(
        remote.export_envelope(&binding, reference, &ceremony),
        Err(KmsError::Unavailable)
    );
    assert_eq!(
        remote.describe_key(&binding, reference),
        Err(KmsError::Unavailable)
    );
    assert_eq!(host.exchange(&sign, false)?[7], 4);
    host.stop()?;
    std::fs::remove_dir(&blocker)?;
    host.start()?;
    assert_eq!(remote.describe_key(&binding, reference)?, description);
    assert_eq!(host.exchange(&sign, false)?[7], 0);
    let managed_ciphertext = std::fs::read(host.root.join("state/state.aead"))?;
    {
        let mut tls = host.connection(false)?;
        checked(layerx_human_service::frame::write_frame(
            &mut tls,
            &frame,
            wire::MAX_FRAME,
        ))?;
        tls.flush()?;
        let mut deadline = Deadline::start(host.clock.as_ref(), Duration::from_secs(5))?;
        loop {
            if remote.evm_operation(6, &binding, reference, &[]) == Err(KmsError::SelfCustodied) {
                break;
            }
            if deadline.remaining(host.clock.as_ref())?.is_zero() {
                return Err("export publication deadline".into());
            }
            std::thread::sleep(Duration::from_millis(2));
        }
        // Deliberately disconnect without reading the durable envelope reply.
    }
    let envelope = remote.export_envelope(&binding, reference, &ceremony)?;
    host.restart()?;
    assert_eq!(
        remote.export_envelope(&binding, reference, &ceremony)?,
        envelope
    );
    assert_eq!(
        remote.export_envelope(&binding, reference, &ceremony)?,
        envelope
    );
    let (seed, ack) = recipient.open(
        fixture::PROVIDER,
        binding.digest(),
        reference.as_bytes(),
        &envelope,
    )?;
    let primary = checked(ring::signature::Ed25519KeyPair::from_seed_unchecked(&*seed))?;
    assert_eq!(primary.public_key().as_ref(), description.public_key());
    assert!(!managed_ciphertext.windows(32).any(|window| window == *seed));
    let encrypted_state = std::fs::read(host.root.join("state/state.aead"))?;
    assert!(!encrypted_state.windows(32).any(|window| window == *seed));
    let wrong = ExportRecipient::new([13; 32])?;
    assert!(wrong
        .open(
            fixture::PROVIDER,
            binding.digest(),
            reference.as_bytes(),
            &envelope
        )
        .is_err());
    assert!(recipient
        .open(
            "other-kms",
            binding.digest(),
            reference.as_bytes(),
            &envelope
        )
        .is_err());
    assert!(recipient
        .open(
            fixture::PROVIDER,
            other.digest(),
            reference.as_bytes(),
            &envelope
        )
        .is_err());
    assert!(recipient
        .open(
            fixture::PROVIDER,
            binding.digest(),
            b"other-handle",
            &envelope
        )
        .is_err());
    let mut tampered = envelope.clone();
    tampered.ciphertext[0] ^= 1;
    assert!(recipient
        .open(
            fixture::PROVIDER,
            binding.digest(),
            reference.as_bytes(),
            &tampered
        )
        .is_err());
    assert_eq!(
        remote.export_envelope(&binding, reference, &recipient.ceremony([14; 32])?),
        Err(KmsError::Conflict)
    );
    assert_eq!(
        remote.export_envelope(&binding, reference, &wrong.ceremony(ceremony.id)?),
        Err(KmsError::Conflict)
    );
    assert_eq!(
        remote.export_envelope(&other, reference, &ceremony),
        Err(KmsError::Integrity)
    );
    assert_eq!(
        remote.describe_key(&other, other_description.reference())?,
        other_description
    );
    assert_eq!(
        remote.acknowledge_export(
            &binding,
            reference,
            &ExportAcknowledgement {
                ceremony: ceremony.clone(),
                proof: [0; 32]
            }
        ),
        Err(KmsError::Refused)
    );
    assert!(remote.acknowledge_export(&other, reference, &ack).is_err());
    host.restart()?;
    let restored = ExportRecipient::new([11; 32])?;
    let recovered_envelope = remote.export_envelope(&binding, reference, &ceremony)?;
    assert_eq!(recovered_envelope, envelope);
    let (recovered, _) = restored.open(
        fixture::PROVIDER,
        binding.digest(),
        reference.as_bytes(),
        &recovered_envelope,
    )?;
    assert!(layerx_crypto::ct::eq_fixed(&*recovered, &*seed));
    remote.acknowledge_export(&binding, reference, &ack)?;
    host.restart()?;
    remote.acknowledge_export(&binding, reference, &ack)?;
    assert_eq!(
        remote.export_envelope(&binding, reference, &ceremony),
        Err(KmsError::SelfCustodied)
    );
    assert_eq!(
        remote.evm_operation(6, &binding, reference, &[]),
        Err(KmsError::SelfCustodied)
    );
    assert_eq!(host.exchange(&sign, false)?[7], 6);
    assert_eq!(
        remote.rotate_key_if_current(&binding, reference, description.public_key()),
        Err(KmsError::SelfCustodied)
    );
    assert_eq!(remote.create_key(&binding), Err(KmsError::Conflict));
    assert_eq!(remote.describe_key(&binding, reference)?, description);
    assert_eq!(
        remote.describe_key(&other, other_description.reference())?,
        other_description
    );
    Ok(())
}
