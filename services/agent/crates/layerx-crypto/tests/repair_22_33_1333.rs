//! Repair 22.33.1333: payment-codec-retired-dependency.
//! Proving cases and the baseline constraint are recorded in `src/repair_22_33_1333.kvx`.

#[path = "support/assertions.rs"]
mod assertions;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::disclosure::call::{
    encode_payment_call, reencode, validate_call, validate_canonical, ConstructMethod,
};
use layerx_crypto::evm_transaction::{decode_signed, decode_unsigned, recover_sender, Eip1559Call};
use layerx_crypto::payments::{bind_call, Payment, TransferLeg};
use layerx_types::payload::ModuleId;
use sha2::{Digest as _, Sha256};

const ACTOR: &[u8] = b"did:layerx:alice";
const VECTORS: &[(ModuleId, u16, &str)] = &[
    (
        ModuleId::Asset,
        1,
        include_str!("fixtures/payments/1-1.hex"),
    ),
    (
        ModuleId::Asset,
        2,
        include_str!("fixtures/payments/1-2.hex"),
    ),
    (
        ModuleId::Asset,
        3,
        include_str!("fixtures/payments/1-3.hex"),
    ),
    (
        ModuleId::Asset,
        4,
        include_str!("fixtures/payments/1-4.hex"),
    ),
    (
        ModuleId::Asset,
        6,
        include_str!("fixtures/payments/native-1-6.hex"),
    ),
    (
        ModuleId::Asset,
        7,
        include_str!("fixtures/payments/native-1-7.hex"),
    ),
    (
        ModuleId::Asset,
        8,
        include_str!("fixtures/payments/1-8.hex"),
    ),
    (
        ModuleId::Asset,
        10,
        include_str!("fixtures/payments/1-10.hex"),
    ),
    (
        ModuleId::Asset,
        11,
        include_str!("fixtures/payments/1-11.hex"),
    ),
    (
        ModuleId::Programs,
        5,
        include_str!("fixtures/payments/9-5.hex"),
    ),
    (
        ModuleId::Programs,
        6,
        include_str!("fixtures/payments/9-6.hex"),
    ),
];

fn hex(text: &str) -> Vec<u8> {
    let text = text.trim();
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            assertions::require(
                u8::from_str_radix(
                    assertions::require(std::str::from_utf8(pair), "fixture UTF-8"),
                    16,
                ),
                "fixture hex",
            )
        })
        .collect()
}

#[test]
fn payment_codec_preserves_native_bytes_and_canonical_refusals(
) -> Result<(), Box<dyn std::error::Error>> {
    assert!(!include_str!("../src/payments.rs").contains("layerx_wire"));
    assert!(!include_str!("../src/payments/codec.rs").contains("layerx_wire"));
    for &(module, ordinal, fixture) in VECTORS {
        let bytes = hex(fixture);
        let payment = Payment::decode(module, ordinal, &bytes, ACTOR)
            .map_err(|error| format!("recorded payload: {error:?}"))?;
        assert_eq!(payment.activity_type(), (module, ordinal));
        assert_eq!(
            payment
                .encode(ACTOR)
                .map_err(|error| format!("encode: {error:?}"))?,
            bytes
        );
        for length in 0..bytes.len() {
            assert!(
                Payment::decode(module, ordinal, &bytes[..length], ACTOR).is_err(),
                "{module:?}/{ordinal} truncated at {length}"
            );
        }
        let mut trailing = bytes.clone();
        trailing.push(0);
        assert!(Payment::decode(module, ordinal, &trailing, ACTOR).is_err());
        if module == ModuleId::Asset && matches!(ordinal, 1 | 2 | 3 | 4 | 8 | 10 | 11) {
            for version in [0_u16, 2, 3, u16::MAX] {
                let mut changed = bytes.clone();
                changed[..2].copy_from_slice(&version.to_be_bytes());
                assert!(Payment::decode(module, ordinal, &changed, ACTOR).is_err());
            }
        }
    }
    for ordinal in [5, 9] {
        assert!(Payment::decode(ModuleId::Asset, ordinal, &[0; 80], ACTOR).is_err());
    }
    let registration = hex(VECTORS[0].2);
    assert!(Payment::decode(ModuleId::Asset, 1, &registration, b"did:layerx:bob").is_err());

    Ok(())
}

#[test]
fn payment_codec_keeps_bounded_lengths_counts_and_boolean_encodings(
) -> Result<(), Box<dyn std::error::Error>> {
    let mut account = Payment::ProgramAccount {
        program: [1; 32],
        asset: [2; 32],
        seed: vec![3; 128],
    };
    let bytes = account
        .encode(ACTOR)
        .map_err(|error| format!("maximum seed: {error:?}"))?;
    assert_eq!(
        Payment::decode(ModuleId::Programs, 6, &bytes, ACTOR)
            .map_err(|error| format!("decode: {error:?}"))?,
        account
    );
    for length in [129_u32, u32::MAX] {
        let mut changed = bytes.clone();
        changed[69..73].copy_from_slice(&length.to_be_bytes());
        assert!(Payment::decode(ModuleId::Programs, 6, &changed, ACTOR).is_err());
    }
    for offset in 32..37 {
        let mut changed = bytes.clone();
        changed[offset] ^= 1;
        assert!(Payment::decode(ModuleId::Programs, 6, &changed, ACTOR).is_err());
    }
    if let Payment::ProgramAccount { seed, .. } = &mut account {
        seed.push(0);
    }
    assert!(account.encode(ACTOR).is_err());

    let mut transfer = Payment::ProgramTransfer {
        program: [1; 32],
        legs: vec![
            TransferLeg {
                from: [2; 32],
                asset: [3; 32],
                to: [4; 32],
                amount: u128::MAX,
            };
            256
        ],
    };
    let bytes = transfer
        .encode(ACTOR)
        .map_err(|error| format!("maximum leg count: {error:?}"))?;
    assert_eq!(
        Payment::decode(ModuleId::Programs, 5, &bytes, ACTOR)
            .map_err(|error| format!("decode: {error:?}"))?,
        transfer
    );
    for count in [0_u16, 257, u16::MAX] {
        let mut changed = bytes.clone();
        changed[32..34].copy_from_slice(&count.to_be_bytes());
        assert!(Payment::decode(ModuleId::Programs, 5, &changed, ACTOR).is_err());
    }
    if let Payment::ProgramTransfer { legs, .. } = &mut transfer {
        legs.push(legs[0].clone());
    }
    assert!(transfer.encode(ACTOR).is_err());

    let grant = hex(VECTORS[5].2);
    for offset in [160, 209] {
        for flag in [2, 255] {
            let mut changed = grant.clone();
            changed[offset] = flag;
            assert!(Payment::decode(ModuleId::Asset, 7, &changed, ACTOR).is_err());
        }
    }
    let mut registration = Payment::decode(ModuleId::Asset, 1, &hex(VECTORS[0].2), ACTOR)
        .map_err(|error| format!("registration: {error:?}"))?;
    if let Payment::Register(r) = &mut registration {
        r.symbol = "A".repeat(17);
    }
    assert!(registration.encode(ACTOR).is_err());

    Ok(())
}

fn resign_receiver(payment: &mut Payment, domain: &[u8]) {
    let Payment::Receive {
        from,
        to,
        asset,
        amount,
        grant,
        sequence,
        idempotency_key,
        context_hash,
        receiver_authorization: auth,
        ..
    } = payment
    else {
        panic!("receive");
    };
    let key = SigningKey::from_bytes(&[0x53; 32]);
    let mut preimage = b"LXP:RECEIVE:v1".to_vec();
    for bytes in [
        from.as_slice(),
        to.as_slice(),
        asset.as_slice(),
        &amount.to_be_bytes(),
        grant.as_slice(),
        &sequence.to_be_bytes(),
        idempotency_key.as_slice(),
        context_hash.as_slice(),
        &[auth.kind],
        &auth.controller,
        &auth.signed_context_hash,
        &auth.network_id.to_be_bytes(),
        &auth.protocol_version.to_be_bytes(),
    ] {
        preimage.extend_from_slice(bytes);
    }
    let mut hash = Sha256::new();
    hash.update(domain);
    hash.update(preimage);
    auth.public_key = key.verifying_key().to_bytes();
    auth.signature = key.sign(&hash.finalize()).to_bytes();
}

#[test]
fn payment_codec_preserves_domains_versions_and_signature_refusals(
) -> Result<(), Box<dyn std::error::Error>> {
    for &(module, ordinal, fixture) in &VECTORS[4..6] {
        let bytes = hex(fixture);
        for offset in 0..bytes.len() {
            let mut changed = bytes.clone();
            changed[offset] ^= 1;
            assert!(
                Payment::decode(module, ordinal, &changed, ACTOR).is_err(),
                "forged ordinal {ordinal} byte {offset}"
            );
        }
    }
    let grant_bytes = hex(VECTORS[5].2);
    let Payment::IssueGrant(grant) = Payment::decode(ModuleId::Asset, 7, &grant_bytes, ACTOR)
        .map_err(|error| format!("grant: {error:?}"))?
    else {
        panic!("grant");
    };
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/authority-hash\0");
    digest.update(b"LXP:GRANT:v1");
    digest.update(&grant_bytes[32..282]);
    assert_eq!(
        grant
            .signing_digest()
            .map_err(|error| format!("digest: {error:?}"))?,
        <[u8; 32]>::from(digest.finalize())
    );
    assert_eq!(
        grant
            .signing_digest()
            .map_err(|error| format!("digest: {error:?}"))?,
        grant.id
    );

    let receive = Payment::decode(ModuleId::Asset, 6, &hex(VECTORS[4].2), ACTOR)
        .map_err(|error| format!("receive: {error:?}"))?;
    for version in [1_u16, 2, 3, 0, 4, u16::MAX] {
        let mut payment = receive.clone();
        if let Payment::Receive {
            receiver_authorization: auth,
            ..
        } = &mut payment
        {
            auth.protocol_version = version;
        }
        resign_receiver(&mut payment, b"LXP/v1/signature-preimage\0");
        if (1..=3).contains(&version) {
            let bytes = payment
                .encode(ACTOR)
                .map_err(|error| format!("supported version and valid signature: {error:?}"))?;
            assert_eq!(
                Payment::decode(ModuleId::Asset, 6, &bytes, ACTOR)
                    .map_err(|error| format!("decode: {error:?}"))?,
                payment
            );
        } else {
            assert!(payment.encode(ACTOR).is_err());
        }
    }
    let mut wrong_domain = receive.clone();
    resign_receiver(&mut wrong_domain, b"LXP/v1/authority-hash\0");
    assert!(wrong_domain.encode(ACTOR).is_err());
    let mut zero_network = receive;
    if let Payment::Receive {
        receiver_authorization: auth,
        ..
    } = &mut zero_network
    {
        auth.network_id = 0;
    }
    resign_receiver(&mut zero_network, b"LXP/v1/signature-preimage\0");
    assert!(zero_network.encode(ACTOR).is_err());

    Ok(())
}

#[test]
fn payment_codec_reaches_the_transaction_and_disclosure_consumer(
) -> Result<(), Box<dyn std::error::Error>> {
    let payment = Payment::decode(ModuleId::Asset, 7, &hex(VECTORS[5].2), ACTOR)?;
    let key = [0x61; 32];
    let evm_key =
        k256::ecdsa::SigningKey::from_slice(&key).map_err(|error| format!("{error:?}"))?;
    let from = layerx_crypto::secp256k1::evm_address(
        evm_key.verifying_key().to_encoded_point(false).as_bytes(),
    )
    .map_err(|error| format!("{error:?}"))?;
    let mut to = [0; 20];
    to[18..].copy_from_slice(&0x101a_u16.to_be_bytes());
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
        to,
        data: encode_payment_call(&payment, ACTOR)?,
    };
    let transported = decode_unsigned(&call.encode_unsigned())?;
    assert_eq!(transported, call);
    let disclosure = bind_call(&transported, from)?;
    assert_eq!(disclosure.payment, Some(payment.clone()));
    let Payment::IssueGrant(grant) = &payment else {
        panic!("grant");
    };
    assert_eq!(
        disclosure.construct_method()?,
        Some(ConstructMethod::LedgerIssueGrant {
            grant: grant.clone()
        })
    );
    assert_eq!(disclosure.counterparties[0].account, grant.from);
    assert_eq!(disclosure.counterparties[1].account, grant.recipient);
    assert_eq!(disclosure.amounts[0].value, grant.per_draw_maximum);
    assert_eq!(disclosure.amounts[1].value, grant.allowance);
    assert_eq!(reencode(&disclosure)?, call.encode_unsigned());
    validate_call(&call, &disclosure, from)?;
    validate_canonical(&disclosure, &call.encode_unsigned())?;
    let signed = call.sign(&key)?;
    let (received, signature) = decode_signed(&signed)?;
    assert_eq!(recover_sender(&received, &signature)?, from);
    assert_eq!(bind_call(&received, from)?, disclosure);
    let mut changed = disclosure.clone();
    if let Some(Payment::IssueGrant(grant)) = &mut changed.payment {
        grant.allowance += 1;
    }
    assert!(reencode(&changed).is_err());
    assert!(validate_call(&call, &changed, from).is_err());
    for offset in 0..call.data.len() {
        let mut changed = call.clone();
        changed.data[offset] ^= 1;
        assert!(validate_call(&changed, &disclosure, from).is_err());
        assert!(validate_canonical(&disclosure, &changed.encode_unsigned()).is_err());
    }
    Ok(())
}
