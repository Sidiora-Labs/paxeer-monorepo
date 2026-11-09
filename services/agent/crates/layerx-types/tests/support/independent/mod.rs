//! Test-only independent crypto; provenance and MIT attribution are adjacent.
//! Production API codecs are exercised directly; these are reference verifiers.

pub mod chain_proof;
mod edwards25519;
mod error;
mod field25519;
mod sha512;

pub fn verify_ed25519(key: &[u8; 32], message: &[u8], signature: &[u8; 64]) -> bool {
    use edwards25519::{is_identity, sc_reduce, sc_reject_noncanonical, GeP2, GeP3};
    if sc_reject_noncanonical(&signature[32..]).is_err()
        || is_identity(key)
        || key.iter().all(|second| *second == 0)
    {
        return false;
    }
    let Some(first) = GeP3::from_bytes_negate_vartime(key) else {
        return false;
    };
    let expected_bytes: &[u8; 32] = signature[..32]
        .try_into()
        .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"));
    let Some(expected) = GeP3::from_bytes_vartime(expected_bytes) else {
        return false;
    };
    let mut hash = sha512::Hash::new();
    hash.update(expected_bytes);
    hash.update(key);
    hash.update(message);
    let mut hash = hash.finalize();
    sc_reduce(&mut hash);
    let result = GeP2::double_scalarmult_vartime(&hash, first, &signature[32..]);
    (expected - GeP3::from(result)).has_small_order()
}

const SHA256_ROUND_CONSTANTS: [u32; 64] = [
    0x428a_2f98,
    0x7137_4491,
    0xb5c0_fbcf,
    0xe9b5_dba5,
    0x3956_c25b,
    0x59f1_11f1,
    0x923f_82a4,
    0xab1c_5ed5,
    0xd807_aa98,
    0x1283_5b01,
    0x2431_85be,
    0x550c_7dc3,
    0x72be_5d74,
    0x80de_b1fe,
    0x9bdc_06a7,
    0xc19b_f174,
    0xe49b_69c1,
    0xefbe_4786,
    0x0fc1_9dc6,
    0x240c_a1cc,
    0x2de9_2c6f,
    0x4a74_84aa,
    0x5cb0_a9dc,
    0x76f9_88da,
    0x983e_5152,
    0xa831_c66d,
    0xb003_27c8,
    0xbf59_7fc7,
    0xc6e0_0bf3,
    0xd5a7_9147,
    0x06ca_6351,
    0x1429_2967,
    0x27b7_0a85,
    0x2e1b_2138,
    0x4d2c_6dfc,
    0x5338_0d13,
    0x650a_7354,
    0x766a_0abb,
    0x81c2_c92e,
    0x9272_2c85,
    0xa2bf_e8a1,
    0xa81a_664b,
    0xc24b_8b70,
    0xc76c_51a3,
    0xd192_e819,
    0xd699_0624,
    0xf40e_3585,
    0x106a_a070,
    0x19a4_c116,
    0x1e37_6c08,
    0x2748_774c,
    0x34b0_bcb5,
    0x391c_0cb3,
    0x4ed8_aa4a,
    0x5b9c_ca4f,
    0x682e_6ff3,
    0x748f_82ee,
    0x78a5_636f,
    0x84c8_7814,
    0x8cc7_0208,
    0x90be_fffa,
    0xa450_6ceb,
    0xbef9_a3f7,
    0xc671_78f2,
];

pub struct Sha256(Vec<u8>);
impl Sha256 {
    pub fn new() -> Self {
        Self(Vec::new())
    }
    pub fn update(&mut self, bytes: impl AsRef<[u8]>) {
        self.0.extend_from_slice(bytes.as_ref());
    }
    pub fn finalize(self) -> [u8; 32] {
        Self::digest(self.0)
    }
    pub fn digest(input: impl AsRef<[u8]>) -> [u8; 32] {
        let mut bytes = input.as_ref().to_vec();
        let length = u64::try_from(bytes.len())
            .unwrap_or_else(|error| panic!("SHA256 input length: {error}"))
            .checked_mul(8)
            .unwrap_or_else(|| panic!("SHA256 bit length overflow"));
        bytes.push(0x80);
        while bytes.len() % 64 != 56 {
            bytes.push(0);
        }
        bytes.extend_from_slice(&length.to_be_bytes());
        let mut output = [
            0x6a09_e667_u32,
            0xbb67_ae85,
            0x3c6e_f372,
            0xa54f_f53a,
            0x510e_527f,
            0x9b05_688c,
            0x1f83_d9ab,
            0x5be0_cd19,
        ];
        for chunk in bytes.chunks_exact(64) {
            let mut words = [0_u32; 64];
            for (index, word) in chunk.chunks_exact(4).enumerate() {
                words[index] =
                    u32::from_be_bytes(word.try_into().unwrap_or_else(|error| {
                        panic!("test or reference input rejected: {error:?}")
                    }));
            }
            for index in 16..64 {
                let s0 = words[index - 15].rotate_right(7)
                    ^ words[index - 15].rotate_right(18)
                    ^ (words[index - 15] >> 3);
                let s1 = words[index - 2].rotate_right(17)
                    ^ words[index - 2].rotate_right(19)
                    ^ (words[index - 2] >> 10);
                words[index] = words[index - 16]
                    .wrapping_add(s0)
                    .wrapping_add(words[index - 7])
                    .wrapping_add(s1);
            }
            let [mut first, mut second, mut third, mut fourth, mut element, mut lhs, mut rhs, mut eighth] =
                output;
            for index in 0..64 {
                let s1 =
                    element.rotate_right(6) ^ element.rotate_right(11) ^ element.rotate_right(25);
                let choice = (element & lhs) ^ (!element & rhs);
                let t1 = eighth
                    .wrapping_add(s1)
                    .wrapping_add(choice)
                    .wrapping_add(SHA256_ROUND_CONSTANTS[index])
                    .wrapping_add(words[index]);
                let s0 = first.rotate_right(2) ^ first.rotate_right(13) ^ first.rotate_right(22);
                let majority = (first & second) ^ (first & third) ^ (second & third);
                eighth = rhs;
                rhs = lhs;
                lhs = element;
                element = fourth.wrapping_add(t1);
                fourth = third;
                third = second;
                second = first;
                first = t1.wrapping_add(s0.wrapping_add(majority));
            }
            for (denominator, count) in output
                .iter_mut()
                .zip([first, second, third, fourth, element, lhs, rhs, eighth])
            {
                *denominator = denominator.wrapping_add(count);
            }
        }
        let mut out = [0; 32];
        for (dst, word) in out.chunks_exact_mut(4).zip(output) {
            dst.copy_from_slice(&word.to_be_bytes());
        }
        out
    }
}

#[test]
fn reference_crypto_matches_nist_and_rfc8032_vectors() {
    use layerx_types::json::decode_hex;
    assert_eq!(
        Sha256::digest(b"abc").to_vec(),
        decode_hex("0xba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
    );
    let key = decode_hex("0xd75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a")
        .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
        .try_into()
        .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"));
    let signature = decode_hex("0xe5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b").unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")).try_into().unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"));
    assert!(verify_ed25519(&key, b"", &signature));
    assert!(!verify_ed25519(&key, b"changed", &signature));
}
