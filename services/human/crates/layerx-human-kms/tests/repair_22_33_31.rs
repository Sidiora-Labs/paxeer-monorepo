//! Task 22.33.31, audit 22.30.102, acceptance 22998.1.
//! kms-operator-doc is proved by documented_configuration_and_assertion_lifecycle,
//! documented_v7_sign_and_retired_encodings, documented_bank_send_and_recovery,
//! and documented_recipient_export_and_recovery.
//! Baseline: not run; the explicit implementation-first, hf-build/hf-done order
//! prohibits the audit step's unchanged-tree verify. No baseline failures claimed.
//! No out-of-path wiring is needed for this documentation repair. This module is
//! the task record because the task scope prohibits editing spec/ records.
//! The gates attach the exact verified candidate revision and private log path.
//! Production server/store/codec plus the real Human remote client are exercised
//! in-process; the committed loopback peers produce real EIP-1559 signatures
//! using independent fixture keys, without an external threshold toolchain.

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
use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_crypto::signer::AgentSignature;
use layerx_human_kms::export::ExportRecipient;
use layerx_human_service::custody::{KmsError, KmsProvider};
use ring::signature::KeyPair;
use std::fs;
use std::os::unix::fs::PermissionsExt;

const README: &str = include_str!("../README.md");

// The existing committed host uses this production call to provision its peers.
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

// Restore every internally provisioned variable; the gate requires no caller env.
struct Environment(Vec<(String, Option<std::ffi::OsString>)>);
impl Environment {
    fn set(&mut self, suffix: &str, value: &std::ffi::OsStr) {
        let name = format!("LAYERX_HUMAN_KMS_{suffix}");
        self.0.push((name.clone(), std::env::var_os(&name)));
        std::env::set_var(name, value);
    }
}
impl Drop for Environment {
    fn drop(&mut self) {
        for (name, previous) in self.0.drain(..).rev() {
            match previous {
                Some(value) => std::env::set_var(name, value),
                None => std::env::remove_var(name),
            }
        }
    }
}

#[test]
fn documented_configuration_and_assertion_lifecycle() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
    let mut host = Host::new()?;
    let mut environment = Environment(Vec::new());
    let registry = host.root.join("registry.json");
    let snapshot = README
        .split("```json\n")
        .nth(1)
        .ok_or("registry example missing")?
        .split("\n```")
        .next()
        .ok_or("registry example unclosed")?;
    fs::write(&registry, snapshot)?;
    for (suffix, value) in [
        ("LISTEN", "127.0.0.1:0".into()),
        ("STATE_DIR", host.root.join("config-state").into_os_string()),
        ("PROVIDER_REFERENCE", fixture::PROVIDER.into()),
        ("DEADLINE_SECONDS", "5".into()),
        ("REGISTRY_FILE", registry.clone().into_os_string()),
    ] {
        environment.set(suffix, &value);
    }
    for (suffix, name, bytes) in [
        (
            "CLIENT_CA_DER",
            "config-ca.der",
            include_bytes!("fixtures/repair_22_33_27/ca.der").as_slice(),
        ),
        (
            "CLIENT_CERT_DER",
            "config-client.der",
            include_bytes!("fixtures/repair_22_33_27/client.der").as_slice(),
        ),
        (
            "TLS_CERT_DER",
            "config-server.der",
            include_bytes!("fixtures/repair_22_33_27/server.der").as_slice(),
        ),
        (
            "TLS_KEY_DER",
            "config-server.pk8",
            include_bytes!("fixtures/repair_22_33_27/server.pk8").as_slice(),
        ),
        ("SEAL_SECRET_FILE", "config-seal", &[37; 32][..]),
        (
            "EVM_CLIENT_CERT_DER",
            "config-executor.der",
            include_bytes!("fixtures/repair_22_33_27/executor.der").as_slice(),
        ),
    ] {
        let path = host.root.join(name);
        fs::write(&path, bytes)?;
        fs::set_permissions(&path, fs::Permissions::from_mode(0o600))?;
        environment.set(suffix, path.as_os_str());
    }
    let loaded = config::Config::load()?;
    assert_eq!(loaded.chain.chain_id, 125);
    assert_eq!(loaded.provider, fixture::PROVIDER);
    assert_eq!(loaded.seal.len(), 32);
    for invalid in [
        r#"{"chain_id":126}"#,
        r#"{"network_id":125,"protocol_version":1,"modules":[]}"#,
        r#"{"chain_id":125,"network_id":125}"#,
    ] {
        fs::write(&registry, invalid)?;
        assert!(config::Config::load().is_err(), "accepted {invalid}");
    }
    fs::write(&registry, snapshot)?;
    fs::set_permissions(
        host.root.join("config-seal"),
        fs::Permissions::from_mode(0o644),
    )?;
    assert!(config::Config::load().is_err());
    fs::set_permissions(
        host.root.join("config-seal"),
        fs::Permissions::from_mode(0o600),
    )?;
    assert!(config::Config::load().is_ok());

    let remote = host.remote(false)?;
    let binding = host.bindings[0].clone();
    let description = remote.create_key(&binding)?;
    let reference = description.reference();
    let wallet = remote.wallet_descriptor(&binding, reference)?;
    // The running store retains the assertion it loaded. Reopening re-reads it.
    let assertion = host.root.join("owner-0.jwt");
    fs::write(&assertion, " \n")?;
    let tx = call(0);
    let disclosure = checked(bind_call(&tx, wallet.evm_secp256k1))?;
    let signature = fixture::run(remote.sign_wallet_call(&binding, reference, &tx, &disclosure))?;
    assert_eq!(
        recover_sender(&tx, &checked(AgentSignature::new(signature))?)?,
        wallet.evm_secp256k1
    );
    host.stop()?;
    assert!(host.start().is_err());
    fs::write(&assertion, "fixture-owner-renewed\n")?;
    host.start()?;
    let tx = call(1);
    let disclosure = checked(bind_call(&tx, wallet.evm_secp256k1))?;
    let signature = fixture::run(remote.sign_wallet_call(&binding, reference, &tx, &disclosure))?;
    assert_eq!(
        recover_sender(&tx, &checked(AgentSignature::new(signature))?)?,
        wallet.evm_secp256k1
    );
    host.stop()?;
    fs::set_permissions(&assertion, fs::Permissions::from_mode(0o644))?;
    assert!(host.start().is_err());
    fs::set_permissions(&assertion, fs::Permissions::from_mode(0o600))?;
    let attestors = host.root.join("attestors.json");
    let saved = fs::read(&attestors)?;
    fs::write(&attestors, br#"{"bindings":[]}"#)?;
    assert!(
        host.start().is_err(),
        "retained wallet lost its configured identity"
    );
    fs::write(&attestors, saved)?;
    host.start()?;
    assert_eq!(remote.wallet_descriptor(&binding, reference)?, wallet);
    assert!(README.contains("running process does not\nrefresh it"));
    assert!(README.contains("each binding's `assertion_file`"));
    Ok(())
}

#[test]
fn documented_v7_sign_and_retired_encodings() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
    let host = Host::new()?;
    let remote = host.remote(false)?;
    let binding = host.bindings[0].clone();
    let description = remote.create_key(&binding)?;
    let reference = description.reference();
    let wallet = remote.wallet_descriptor(&binding, reference)?;
    let call = call(0);
    let disclosure = checked(bind_call(&call, wallet.evm_secp256k1))?;
    let signature = fixture::run(remote.sign_wallet_call(&binding, reference, &call, &disclosure))?;
    assert!(matches!(signature[64], 27 | 28));
    assert_eq!(
        recover_sender(&call, &checked(AgentSignature::new(signature))?)?,
        wallet.evm_secp256k1
    );
    let frame = host.sign_frame(
        &binding,
        reference,
        wallet.evm_secp256k1,
        &attestor::unsigned_evm_transaction(&call),
        &checked(wire::disclosure(&disclosure))?,
    )?;
    let decoded = checked(wire::Request::decode(&frame))?;
    assert_eq!((decoded.version, decoded.operation), (7, 5));
    assert_eq!(
        host.exchange(&frame, false)?.as_slice(),
        [b"LXKP\0\x07\x05\0".as_slice(), &signature].concat()
    );
    assert_eq!(host.exchange(&frame, true)?[7], 1);
    let mut old = frame.clone();
    old[4..6].copy_from_slice(&1_u16.to_be_bytes());
    assert!(matches!(
        wire::Request::decode(&old),
        Err(wire::Error::Refused)
    ));
    assert!(host.exchange(&old, false).is_err());
    let mut retired = remote.encode_evm_request(6, &binding, reference, &[])?;
    retired[6] = 13;
    assert!(matches!(
        wire::Request::decode(&retired),
        Err(wire::Error::Refused)
    ));
    assert!(host.exchange(&retired, false).is_err());
    // The legacy client encoder still emits v4/op13. Prove refusal at the
    // production boundary documented here, including the actual client bytes.
    let legacy_recipient = remote.encode_evm_request(13, &binding, reference, b"{}")?;
    assert_eq!(&legacy_recipient[..7], b"LXKP\0\x04\x0d");
    assert_eq!(
        wire::Request::decode(&legacy_recipient).err(),
        Some(wire::Error::Refused)
    );
    assert!(host.exchange(&legacy_recipient, false).is_err());
    assert!(README.contains("| 7 | 5 Sign |"));
    assert!(README.contains("operation 13 is refused by the decoder before dispatch"));
    assert!(README.contains("let wallet = remote.wallet_descriptor(&binding, reference)?;"));
    assert!(README.contains("let disclosure = bind_call(&call, wallet.evm_secp256k1)?;"));
    assert!(README.contains(
        "let signature = remote.sign_wallet_call(&binding, reference, &call, &disclosure).await?;"
    ));
    Ok(())
}

fn bank_authorization(host: &Host, wallet: [u8; 20]) -> Result<send::SendAuthorization> {
    let recipient = [0x44; 20];
    let mut amount = [0; 32];
    amount[31] = 42;
    let mut bank = [0; 20];
    bank[18..].copy_from_slice(&[0x10, 0x01]);
    let mut data =
        layerx_crypto::disclosure::call::ledger_selector("send(address,address,string,uint256)")
            .to_vec();
    for address in [wallet, recipient] {
        data.extend([0; 12]);
        data.extend(address);
    }
    let mut offset = [0; 32];
    offset[31] = 128;
    data.extend(offset);
    data.extend(amount);
    let mut length = [0; 32];
    length[31] = 4;
    data.extend(length);
    data.extend(b"apax");
    data.extend([0; 28]);
    let now = host
        .clock
        .sample(std::time::Duration::from_secs(1))?
        .unix_seconds();
    Ok(send::SendAuthorization {
        authorization: evm_types::EvmPlanAuthorization {
            plan_id: [1; 32],
            action_key: [2; 32],
            tenant: "tenant".into(),
            principal: "alice".into(),
            binding_digest: host.bindings[0].digest(),
            wallet,
            not_before: now.saturating_sub(1),
            not_after: now + 300,
            transaction: evm_types::EvmTransaction {
                chain_id: 125,
                nonce: 0,
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 2,
                gas_limit: 100_000,
                to: bank,
                value: [0; 32],
                calldata: data,
            },
        },
        disclosure: send::SendDisclosure {
            recipient,
            denom: "apax".into(),
            amount,
        },
    })
}

#[test]
fn documented_bank_send_and_recovery() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
    let mut host = Host::new()?;
    let remote = host.remote(false)?;
    let binding = host.bindings[0].clone();
    let description = remote.create_key(&binding)?;
    let reference = description.reference();
    let wallet = remote.wallet_descriptor(&binding, reference)?;
    let authorization = bank_authorization(&host, wallet.evm_secp256k1)?;
    let payload = serde_json::to_vec(&authorization)?;
    let frame = remote.encode_evm_request(11, &binding, reference, &payload)?;
    let decoded = checked(wire::Request::decode(&frame))?;
    assert_eq!((decoded.version, decoded.operation), (3, 11));
    let mut mismatch = authorization.clone();
    mismatch.disclosure.amount[31] += 1;
    assert_eq!(
        remote.evm_operation(11, &binding, reference, &serde_json::to_vec(&mismatch)?),
        Err(KmsError::Refused)
    );
    let raw = remote.evm_operation(11, &binding, reference, &payload)?;
    assert_eq!(raw[0], 2);
    let (call, signature) = decode_signed(&raw)?;
    assert_eq!(recover_sender(&call, &signature)?, wallet.evm_secp256k1);
    let expected = &authorization.authorization.transaction;
    assert_eq!(call.chain_id, expected.chain_id);
    assert_eq!(call.nonce, expected.nonce);
    assert_eq!(call.max_fee_per_gas, u128::from(expected.max_fee_per_gas));
    assert_eq!(
        call.max_priority_fee_per_gas,
        u128::from(expected.max_priority_fee_per_gas)
    );
    assert_eq!(call.gas_limit, expected.gas_limit);
    assert_eq!(call.to, expected.to);
    assert_eq!(call.data, expected.calldata);
    let disclosed = checked(bind_call(&call, wallet.evm_secp256k1))?;
    assert_eq!(
        checked(disclosed.construct_method())?,
        Some(ConstructMethod::BankSend {
            from: wallet.evm_secp256k1,
            to: authorization.disclosure.recipient,
            denom: authorization.disclosure.denom.clone(),
            amount: authorization.disclosure.amount,
        })
    );
    host.restart()?;
    assert_eq!(
        remote.evm_operation(11, &binding, reference, &payload)?,
        raw
    );
    let action_key = serde_json::to_vec(&authorization.authorization.action_key)?;
    let recovered = remote.evm_operation(10, &binding, reference, &action_key)?;
    let recovered: evm_types::EvmAction = serde_json::from_slice(&recovered)?;
    assert_eq!(recovered.raw_transaction, raw);
    assert_eq!(recovered.transaction_hash, Some(transaction_hash(&raw)));
    assert!(!recovered.acknowledged);
    let acknowledgement = evm_types::EvmAcknowledgement {
        action_key: authorization.authorization.action_key,
        transaction_hash: transaction_hash(&raw),
    };
    remote.evm_operation(
        9,
        &binding,
        reference,
        &serde_json::to_vec(&acknowledgement)?,
    )?;
    host.restart()?;
    let recovered: evm_types::EvmAction =
        serde_json::from_slice(&remote.evm_operation(10, &binding, reference, &action_key)?)?;
    assert_eq!(recovered.raw_transaction, raw);
    assert!(recovered.acknowledged);
    assert!(README.contains("| 3 | 11 authorize send |"));
    for example in [
        "let payload = serde_json::to_vec(&authorization)?;",
        "let raw = remote.evm_operation(11, &binding, reference, &payload)?;",
        "let action_key = serde_json::to_vec(&authorization.authorization.action_key)?;",
        "let recovered = remote.evm_operation(10, &binding, reference, &action_key)?;",
    ] {
        assert!(
            README.contains(example),
            "missing client example: {example}"
        );
    }
    Ok(())
}

#[test]
fn documented_recipient_export_and_recovery() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
    let mut host = Host::new()?;
    let remote = host.remote(false)?;
    let binding = host.bindings[0].clone();
    let description = remote.create_key(&binding)?;
    let reference = description.reference();
    remote.wallet_descriptor(&binding, reference)?;
    let recipient = ExportRecipient::new([11; 32])?;
    let ceremony = recipient.ceremony([12; 32])?;
    let provider = fixture::PROVIDER;
    let envelope = remote.export_envelope(&binding, reference, &ceremony)?;
    host.restart()?;
    assert_eq!(
        remote.export_envelope(&binding, reference, &ceremony)?,
        envelope
    );
    let (seed, acknowledgement) =
        recipient.open(provider, binding.digest(), reference.as_bytes(), &envelope)?;
    let primary = checked(ring::signature::Ed25519KeyPair::from_seed_unchecked(&*seed))?;
    assert_eq!(primary.public_key().as_ref(), description.public_key());
    assert_eq!(
        remote.export_envelope(&binding, reference, &recipient.ceremony([13; 32])?),
        Err(KmsError::Conflict)
    );
    assert_eq!(
        remote.wallet_descriptor(&binding, reference),
        Err(KmsError::SelfCustodied)
    );
    remote.acknowledge_export(&binding, reference, &acknowledgement)?;
    host.restart()?;
    remote.acknowledge_export(&binding, reference, &acknowledgement)?;
    assert_eq!(
        remote.export_envelope(&binding, reference, &ceremony),
        Err(KmsError::SelfCustodied)
    );
    assert_eq!(remote.describe_key(&binding, reference)?, description);
    assert!(README.contains("| 8 | 14 export, 16 acknowledge export |"));
    for example in [
        "let envelope = remote.export_envelope(&binding, reference, &ceremony)?;",
        "let (seed, acknowledgement) = recipient.open(provider, binding.digest(), reference.as_bytes(), &envelope)?;",
        "remote.acknowledge_export(&binding, reference, &acknowledgement)?;",
    ] { assert!(README.contains(example), "missing client example: {example}"); }
    Ok(())
}
