use layerx_crypto::account_derivation::{derive_from_seed, DerivedAccount};
use layerx_crypto::payments::{bind_call, ledger_payment_call, CallOptions};
use layerx_platform_cli::wallet_send::{Authorization, Send};
use layerx_platform_cli::wallet_signing::{EvmSigningKey, PreparedPayment, SigningFacts};
use std::io::Write as _;
use std::os::unix::fs::OpenOptionsExt as _;
use std::path::Path;
use std::process::Command;

fn vector() -> Send {
    let mut send = Send {
        from: [1; 32],
        to: [2; 32],
        asset: [3; 32],
        amount: (1_u128 << 127) + 257,
        source_next_sequence: 0x0102_0304_0506_0708,
        idempotency_key: [4; 32],
        expires_at: 2000,
        context_hash: [5; 32],
        conditions: vec![(1, 1000), (2, 2000)],
        authorization: Authorization {
            kind: 1,
            controller: [1; 32],
            public_key: [6; 32],
            signature: [7; 64],
            signed_context_hash: [5; 32],
            network_id: 402,
            protocol_version: 3,
        },
    };
    resign(&mut send).unwrap_or_else(|error| panic!("{error}"));
    send
}

fn resign(send: &mut Send) -> Result<(), String> {
    resign_with_seed(send, &[6; 32])
}

fn resign_with_seed(send: &mut Send, seed: &[u8; 32]) -> Result<(), String> {
    use ed25519_dalek::Signer as _;
    use sha2::Digest as _;
    let key = ed25519_dalek::SigningKey::from_bytes(seed);
    send.authorization.public_key = key.verifying_key().to_bytes();
    let mut hash = sha2::Sha256::new();
    hash.update(layerx_wire::hash::Domain::SignaturePreimage.tag());
    hash.update(send.authorization_message()?);
    send.authorization.signature = key.sign(&hash.finalize()).to_bytes();
    Ok(())
}

#[test]
fn native_send_and_authorization_bytes_match() -> Result<(), Box<dyn std::error::Error>> {
    let cli = Path::new(env!("CARGO_MANIFEST_DIR"));
    let root = cli.join("../../..");
    let directory = tempfile::tempdir()?;
    let binary = directory.path().join("native-send");
    let output = Command::new("cc")
        .args([
            "-std=c11",
            "-Wall",
            "-Wextra",
            "-Werror",
            "-ffunction-sections",
            "-fdata-sections",
            "-Wl,--gc-sections",
        ])
        .arg("-I")
        .arg(root.join("include"))
        .arg(cli.join("tests/fixtures/native-send.c"))
        .arg(root.join("src/ledger/lxp_send.c"))
        .arg(root.join("src/protocol/lxp_u128.c"))
        .arg("-lcrypto")
        .arg("-o")
        .arg(&binary)
        .output()?;
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let output = Command::new(&binary).output()?;
    assert!(output.status.success());
    let send = vector();
    let mut encoded = send.encode()?;
    assert_eq!(&encoded[..4], &[0x53, 1, 0, 10]);
    encoded.extend(send.authorization_message()?);
    assert_eq!(encoded, output.stdout);
    Ok(())
}

fn evm_key(account: &DerivedAccount) -> Result<EvmSigningKey, String> {
    let directory = tempfile::tempdir().map_err(|e| e.to_string())?;
    let path = directory.path().join("wallet.json");
    let hex = |bytes: &[u8]| bytes.iter().map(|b| format!("{b:02x}")).collect::<String>();
    let mut file = std::fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(&path)
        .map_err(|e| e.to_string())?;
    let export = serde_json::json!({
        "evm_address": format!("0x{}", hex(&account.evm_address())),
        "evm_private_key": hex(account.evm_secret().ok_or("EVM key missing")?),
        "layerx_seed": hex(account.layerx_seed()), "did": account.did(), "index": 0,
    });
    file.write_all(export.to_string().as_bytes())
        .map_err(|e| e.to_string())?;
    let public_key = ed25519_dalek::SigningKey::from_bytes(account.layerx_seed())
        .verifying_key()
        .to_bytes();
    EvmSigningKey::read(&path, &account.did(), public_key)
}

#[test]
fn envelope_and_payload_sequences_are_independent() -> Result<(), String> {
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
    let mut send = vector();
    let account = derive_from_seed(&[6; 64], 0).map_err(|e| format!("{e:?}"))?;
    resign_with_seed(&mut send, account.layerx_seed())?;
    let key = evm_key(&account)?;
    let actor = account.did();
    let facts = SigningFacts {
        actor: &actor,
        public_key: send.authorization.public_key,
        network_id: 402,
        identity_next_sequence: 17,
        not_before_ms: 1000,
        expires_at_ms: 2000,
        fee_limit: 42000,
        idempotency_key: [4; 32],
    };
    let options = CallOptions {
        from: key.address(),
        chain_id: 125,
        nonce: 17,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 21000,
    };
    let prepared = send.prepare(&facts, &options)?;
    let payload = send.encode()?;
    let shared = ledger_payment_call(&payload, &options).map_err(|e| format!("{e:?}"))?;
    assert_eq!(prepared.call(), &shared.call);
    let mut expected = vec![0x2b, 0xb1, 0xae, 0x7c];
    let mut offset = [0; 32];
    offset[31] = 32;
    expected.extend_from_slice(&offset);
    let mut length = [0; 32];
    length[16..].copy_from_slice(&(payload.len() as u128).to_be_bytes());
    expected.extend_from_slice(&length);
    expected.extend_from_slice(&payload);
    expected.resize(68 + payload.len().div_ceil(32) * 32, 0);
    assert_eq!(prepared.call().data, expected);
    assert_eq!(prepared.call().nonce, 17);
    assert_eq!(&prepared.call().to[18..], &[0x10, 0x1a]);
    let disclosure = prepared.confirmation();
    assert_eq!(disclosure["nonce"], "17");
    assert_eq!(
        disclosure["source_account_sequence"],
        send.source_next_sequence.to_string()
    );
    assert_eq!(disclosure["amounts"][0]["value"], send.amount.to_string());
    assert_eq!(disclosure["fee_limit"], "42000");
    let call = prepared.call().clone();
    let (canonical, id) = prepared.sign_with_id(&key)?;
    let (decoded, signature) = decode_signed(&canonical).map_err(|e| format!("{e:?}"))?;
    assert_eq!(decoded, call);
    assert_eq!(transaction_hash(&canonical), id);
    assert_eq!(
        recover_sender(&decoded, &signature).map_err(|e| format!("{e:?}"))?,
        key.address()
    );
    assert_eq!(
        bind_call(&decoded, key.address()).map_err(|e| format!("{e:?}"))?,
        shared.disclosure
    );
    let prepared = PreparedPayment::from_send(&payload, &facts, &options)?;
    assert_eq!(prepared.call(), &call);
    assert_eq!(
        prepared.confirmation()["source_account_sequence"],
        send.source_next_sequence.to_string()
    );
    assert_eq!(prepared.confirmation()["nonce"], "17");
    let mut changed = send.clone();
    changed.source_next_sequence = u64::MAX;
    assert!(changed.encode().is_err());
    resign_with_seed(&mut changed, account.layerx_seed())?;
    assert_ne!(payload, changed.encode()?);
    assert_eq!(changed.prepare(&facts, &options)?.call().nonce, 17);
    let mut wrong_actor = facts;
    wrong_actor.public_key = [8; 32];
    assert!(send.prepare(&wrong_actor, &options).is_err());
    let other = derive_from_seed(&[8; 64], 0).map_err(|e| format!("{e:?}"))?;
    assert!(prepared.sign(&evm_key(&other)?).is_err());
    Ok(())
}

#[test]
fn send_bounds_and_bindings_are_checked() -> Result<(), String> {
    let good = vector();
    let mut bad = good.clone();
    bad.amount = 0;
    assert!(bad.encode().is_err());
    let mut bad = good.clone();
    bad.to = bad.from;
    assert!(bad.encode().is_err());
    let mut bad = good.clone();
    bad.conditions = vec![(1, 0); 9];
    assert!(bad.encode().is_err());
    let mut bad = good.clone();
    bad.conditions = vec![(3, 0)];
    assert!(bad.encode().is_err());
    let mut bad = good.clone();
    bad.authorization.kind = 7;
    assert!(bad.encode().is_err());
    let mut bad = good.clone();
    bad.authorization.controller = [9; 32];
    assert!(bad.encode().is_err());
    let mut bad = good.clone();
    bad.authorization.signed_context_hash = [9; 32];
    assert!(bad.encode().is_err());
    let mut bounds = good;
    bounds.amount = u128::MAX;
    bounds.source_next_sequence = u64::MAX;
    bounds.conditions = vec![(2, u64::MAX); 8];
    resign(&mut bounds)?;
    assert_eq!(bounds.encode()?.len(), 436);
    Ok(())
}
