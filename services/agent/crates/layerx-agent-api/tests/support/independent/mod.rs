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
        || key.iter().all(|b| *b == 0)
    {
        return false;
    }
    let Some(a) = GeP3::from_bytes_negate_vartime(key) else {
        return false;
    };
    let expected_bytes: &[u8; 32] = signature[..32].try_into().unwrap();
    let Some(expected) = GeP3::from_bytes_vartime(expected_bytes) else {
        return false;
    };
    let mut hash = sha512::Hash::new();
    hash.update(expected_bytes);
    hash.update(key);
    hash.update(message);
    let mut hash = hash.finalize();
    sc_reduce(&mut hash);
    let r = GeP2::double_scalarmult_vartime(&hash, a, &signature[32..]);
    (expected - GeP3::from(r)).has_small_order()
}

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
        const K: [u32; 64] = [
            0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4,
            0xab1c5ed5, 0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe,
            0x9bdc06a7, 0xc19bf174, 0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f,
            0x4a7484aa, 0x5cb0a9dc, 0x76f988da, 0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7,
            0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967, 0x27b70a85, 0x2e1b2138, 0x4d2c6dfc,
            0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85, 0xa2bfe8a1, 0xa81a664b,
            0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070, 0x19a4c116,
            0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
            0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7,
            0xc67178f2,
        ];
        let mut bytes = input.as_ref().to_vec();
        let length = (bytes.len() as u64).checked_mul(8).unwrap();
        bytes.push(0x80);
        while bytes.len() % 64 != 56 {
            bytes.push(0);
        }
        bytes.extend_from_slice(&length.to_be_bytes());
        let mut h = [
            0x6a09e667_u32,
            0xbb67ae85,
            0x3c6ef372,
            0xa54ff53a,
            0x510e527f,
            0x9b05688c,
            0x1f83d9ab,
            0x5be0cd19,
        ];
        for chunk in bytes.chunks_exact(64) {
            let mut w = [0_u32; 64];
            for (i, word) in chunk.chunks_exact(4).enumerate() {
                w[i] = u32::from_be_bytes(word.try_into().unwrap());
            }
            for i in 16..64 {
                let s0 = w[i - 15].rotate_right(7) ^ w[i - 15].rotate_right(18) ^ (w[i - 15] >> 3);
                let s1 = w[i - 2].rotate_right(17) ^ w[i - 2].rotate_right(19) ^ (w[i - 2] >> 10);
                w[i] = w[i - 16]
                    .wrapping_add(s0)
                    .wrapping_add(w[i - 7])
                    .wrapping_add(s1);
            }
            let [mut a, mut b, mut c, mut d, mut e, mut f, mut g, mut hh] = h;
            for i in 0..64 {
                let s1 = e.rotate_right(6) ^ e.rotate_right(11) ^ e.rotate_right(25);
                let choice = (e & f) ^ (!e & g);
                let t1 = hh
                    .wrapping_add(s1)
                    .wrapping_add(choice)
                    .wrapping_add(K[i])
                    .wrapping_add(w[i]);
                let s0 = a.rotate_right(2) ^ a.rotate_right(13) ^ a.rotate_right(22);
                let majority = (a & b) ^ (a & c) ^ (b & c);
                hh = g;
                g = f;
                f = e;
                e = d.wrapping_add(t1);
                d = c;
                c = b;
                b = a;
                a = t1.wrapping_add(s0.wrapping_add(majority));
            }
            for (v, n) in h.iter_mut().zip([a, b, c, d, e, f, g, hh]) {
                *v = v.wrapping_add(n);
            }
        }
        let mut out = [0; 32];
        for (dst, word) in out.chunks_exact_mut(4).zip(h) {
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
        decode_hex("0xba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad").unwrap()
    );
    let key = decode_hex("0xd75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a")
        .unwrap()
        .try_into()
        .unwrap();
    let signature = decode_hex("0xe5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b").unwrap().try_into().unwrap();
    assert!(verify_ed25519(&key, b"", &signature));
    assert!(!verify_ed25519(&key, b"changed", &signature));
}
