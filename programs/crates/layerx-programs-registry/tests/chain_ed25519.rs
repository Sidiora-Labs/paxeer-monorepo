#[path = "../src/chain_proof/ed25519.rs"]
mod ed25519;

use curve25519_dalek::{
    constants::{ED25519_BASEPOINT_POINT, EIGHT_TORSION},
    scalar::Scalar,
};
use ed25519::verify_zip215;
use ed25519_dalek::{Signer, SigningKey};
use sha2::{Digest, Sha512};

const MESSAGE: &[u8] = b"validator vote under ZIP-215";
const GROUP_ORDER: [u8; 32] = [
    0xed, 0xd3, 0xf5, 0x5c, 0x1a, 0x63, 0x12, 0x58, 0xd6, 0x9c, 0xf7, 0xa2, 0xde, 0xf9, 0xde, 0x14,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
];

#[test]
fn chain_zip215_accepts_noncanonical_and_small_order_cases() {
    let signing_key = SigningKey::from_bytes(&[42; 32]);
    let signature = signing_key.sign(MESSAGE);
    let public_key = signing_key.verifying_key();
    assert!(public_key.verify_strict(MESSAGE, &signature).is_ok());
    assert!(verify_zip215(
        public_key.as_bytes(),
        MESSAGE,
        &signature.to_bytes()
    ));

    for a in EIGHT_TORSION {
        for r in EIGHT_TORSION {
            let public_key = a.compress().to_bytes();
            let mut signature = [0; 64];
            signature[..32].copy_from_slice(r.compress().as_bytes());
            assert!(verify_zip215(&public_key, MESSAGE, &signature));
        }
    }

    let mut noncanonical_identity = [0xff; 32];
    noncanonical_identity[0] = 0xee;
    noncanonical_identity[31] = 0x7f;
    let mut negative_zero_identity = [0; 32];
    negative_zero_identity[0] = 1;
    negative_zero_identity[31] = 0x80;
    let mut noncanonical_order_four = noncanonical_identity;
    noncanonical_order_four[0] = 0xed;
    let encodings = [
        noncanonical_identity,
        negative_zero_identity,
        noncanonical_order_four,
    ];
    for public_key in encodings {
        for r in encodings {
            let mut signature = [0; 64];
            signature[..32].copy_from_slice(&r);
            assert!(verify_zip215(&public_key, MESSAGE, &signature));
        }
    }

    let secret = Scalar::from(7_u64);
    let public_key = (secret * ED25519_BASEPOINT_POINT).compress().to_bytes();
    let mut signature = [0; 64];
    signature[..32].copy_from_slice(&noncanonical_identity);
    let mut hash = Sha512::new();
    hash.update(noncanonical_identity);
    hash.update(public_key);
    hash.update(MESSAGE);
    let challenge = Scalar::from_bytes_mod_order_wide(&hash.finalize().into());
    signature[32..].copy_from_slice(&(challenge * secret).to_bytes());
    assert!(verify_zip215(&public_key, MESSAGE, &signature));
    assert!(!verify_zip215(&public_key, b"different vote", &signature));

    signature[..32].fill(0);
    signature[0] = 1;
    assert!(!verify_zip215(&public_key, MESSAGE, &signature));
}

#[test]
fn chain_zip215_refuses_noncanonical_s() {
    let signing_key = SigningKey::from_bytes(&[43; 32]);
    let public_key = signing_key.verifying_key().to_bytes();
    let mut signature = signing_key.sign(MESSAGE).to_bytes();
    assert!(verify_zip215(&public_key, MESSAGE, &signature));
    assert!(!verify_zip215(&public_key, b"different vote", &signature));

    let mut carry = 0_u16;
    for (byte, order_byte) in signature[32..].iter_mut().zip(GROUP_ORDER) {
        let sum = u16::from(*byte) + u16::from(order_byte) + carry;
        *byte = sum.to_le_bytes()[0];
        carry = sum >> 8;
    }
    assert_eq!(carry, 0);
    assert!(!verify_zip215(&public_key, MESSAGE, &signature));

    let mut identity = [0; 32];
    identity[0] = 1;
    let mut signature = [0; 64];
    signature[..32].copy_from_slice(&identity);
    assert!(verify_zip215(&identity, MESSAGE, &signature));
    signature[32..].copy_from_slice(&GROUP_ORDER);
    assert!(!verify_zip215(&identity, MESSAGE, &signature));
    signature[32..].fill(0xff);
    assert!(!verify_zip215(&identity, MESSAGE, &signature));
}
