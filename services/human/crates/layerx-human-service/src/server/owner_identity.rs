use layerx_types::ids::Did;
use layerx_wire::hash::{self, Domain};
use sha2::{Digest as _, Sha256};

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct OwnerIdentityState {
    pub primary_public_key: [u8; 32],
    pub pending_public_key: Option<[u8; 32]>,
    pub begin: u64,
    pub end: u64,
    pub effective_sequence: u64,
    pub revocation_sequence: u64,
    pub observed_sequence: u64,
    pub announcement: [u8; 32],
}

impl OwnerIdentityState {
    pub(crate) fn decode(bytes: &[u8], owner: &Did) -> Result<Self, OwnerIdentityError> {
        if bytes.len() != 223
            || &bytes[..5] != b"LXGI1"
            || bytes[5..37]
                != hash::did_id_for_protocol(owner, 3).map_err(|_| OwnerIdentityError::Encoding)?
        {
            return Err(OwnerIdentityError::Binding);
        }
        let primary_public_key = bytes[37..69]
            .try_into()
            .map_err(|_| OwnerIdentityError::Encoding)?;
        let pending: [u8; 32] = bytes[111..143]
            .try_into()
            .map_err(|_| OwnerIdentityError::Encoding)?;
        let number = |offset| -> Result<u64, OwnerIdentityError> {
            Ok(u64::from_be_bytes(
                bytes[offset..offset + 8]
                    .try_into()
                    .map_err(|_| OwnerIdentityError::Encoding)?,
            ))
        };
        let begin = number(143)?;
        let end = number(151)?;
        let effective_sequence = number(159)?;
        let revocation_sequence = number(69)?;
        let observed_sequence = number(215)?;
        if !public_key_is_canonical(&primary_public_key)
            || revocation_sequence == 0
            || observed_sequence < revocation_sequence
            || (pending != [0; 32]
                && (!public_key_is_canonical(&pending)
                    || pending == primary_public_key
                    || begin == 0
                    || end <= begin
                    || effective_sequence == 0))
        {
            return Err(OwnerIdentityError::Binding);
        }
        let mut digest = Sha256::new();
        digest.update(Domain::ContextHash.tag());
        digest.update(bytes);
        Ok(Self {
            primary_public_key,
            pending_public_key: (pending != [0; 32]).then_some(pending),
            begin,
            end,
            effective_sequence,
            revocation_sequence,
            observed_sequence,
            announcement: digest.finalize().into(),
        })
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum OwnerIdentityError {
    Encoding,
    Binding,
}

const FIELD_PRIME: [u8; 32] = [
    0xed, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f,
];

fn little_endian_less(left: &[u8; 32], right: &[u8; 32]) -> bool {
    for index in (0..32).rev() {
        if left[index] < right[index] {
            return true;
        }
        if left[index] > right[index] {
            return false;
        }
    }
    false
}

fn public_key_is_canonical(public_key: &[u8; 32]) -> bool {
    let mut y = *public_key;
    y[31] &= 0x7f;
    if y.iter().all(|byte| *byte == 0) || (y[0] == 1 && y[1..].iter().all(|byte| *byte == 0)) {
        return false;
    }
    little_endian_less(&y, &FIELD_PRIME)
}
