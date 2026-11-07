use curve25519_dalek::{
    edwards::{CompressedEdwardsY, EdwardsPoint},
    scalar::Scalar,
    traits::IsIdentity,
};
use sha2::{Digest, Sha512};

pub(crate) fn verify_zip215(public_key: &[u8; 32], message: &[u8], signature: &[u8; 64]) -> bool {
    let mut s_bytes = [0; 32];
    s_bytes.copy_from_slice(&signature[32..]);
    let Some(s) = Option::<Scalar>::from(Scalar::from_canonical_bytes(s_bytes)) else {
        return false;
    };

    let Some(a) = CompressedEdwardsY(*public_key).decompress() else {
        return false;
    };
    let mut r_bytes = [0; 32];
    r_bytes.copy_from_slice(&signature[..32]);
    let Some(r) = CompressedEdwardsY(r_bytes).decompress() else {
        return false;
    };

    let mut hash = Sha512::new();
    hash.update(r_bytes);
    hash.update(public_key);
    hash.update(message);
    let k = Scalar::from_bytes_mod_order_wide(&hash.finalize().into());

    (EdwardsPoint::vartime_double_scalar_mul_basepoint(&-k, &a, &s) - r)
        .mul_by_cofactor()
        .is_identity()
}
