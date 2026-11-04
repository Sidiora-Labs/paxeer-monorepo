//! Payment payload primitives, independent of the retired activity transport.

use crate::disclosure::DisclosureError;

pub(super) const CONTEXT_HASH_DOMAIN: &[u8] = b"LXP/v1/context-hash\0";
pub(super) const SIGNATURE_PREIMAGE_DOMAIN: &[u8] = b"LXP/v1/signature-preimage\0";
pub(super) const AUTHORITY_HASH_DOMAIN: &[u8] = b"LXP/v1/authority-hash\0";

pub(super) const fn protocol_version_supported(version: u16) -> bool {
    matches!(version, 1 | 2 | 3)
}

pub(super) struct Encoder {
    bytes: Vec<u8>,
    limit: usize,
}

impl Encoder {
    pub(super) const fn new(limit: usize) -> Self {
        Self {
            bytes: Vec::new(),
            limit,
        }
    }

    pub(super) fn fixed(&mut self, bytes: &[u8]) -> Result<(), DisclosureError> {
        let required = self
            .bytes
            .len()
            .checked_add(bytes.len())
            .ok_or(DisclosureError::MalformedPayload)?;
        if required > self.limit {
            return Err(DisclosureError::MalformedPayload);
        }
        self.bytes.extend_from_slice(bytes);
        Ok(())
    }

    pub(super) fn u8(&mut self, value: u8) -> Result<(), DisclosureError> {
        self.fixed(&[value])
    }

    pub(super) fn u16(&mut self, value: u16) -> Result<(), DisclosureError> {
        self.fixed(&value.to_be_bytes())
    }

    pub(super) fn u32(&mut self, value: u32) -> Result<(), DisclosureError> {
        self.fixed(&value.to_be_bytes())
    }

    pub(super) fn u64(&mut self, value: u64) -> Result<(), DisclosureError> {
        self.fixed(&value.to_be_bytes())
    }

    pub(super) fn u128(&mut self, value: u128) -> Result<(), DisclosureError> {
        self.fixed(&value.to_be_bytes())
    }

    pub(super) fn bytes(&mut self, bytes: &[u8], maximum: usize) -> Result<(), DisclosureError> {
        let length = u32::try_from(bytes.len()).map_err(|_| DisclosureError::MalformedPayload)?;
        let required = self
            .bytes
            .len()
            .checked_add(4)
            .and_then(|n| n.checked_add(bytes.len()))
            .ok_or(DisclosureError::MalformedPayload)?;
        if bytes.len() > maximum || required > self.limit {
            return Err(DisclosureError::MalformedPayload);
        }
        self.u32(length)?;
        self.fixed(bytes)
    }

    pub(super) fn finish(self) -> Vec<u8> {
        self.bytes
    }
}

pub(super) struct Decoder<'a> {
    remaining: &'a [u8],
}

impl<'a> Decoder<'a> {
    pub(super) const fn new(bytes: &'a [u8]) -> Self {
        Self { remaining: bytes }
    }

    pub(super) fn fixed(&mut self, length: usize) -> Result<&'a [u8], DisclosureError> {
        let value = self
            .remaining
            .get(..length)
            .ok_or(DisclosureError::MalformedPayload)?;
        self.remaining = &self.remaining[length..];
        Ok(value)
    }

    fn integer<const N: usize>(&mut self) -> Result<[u8; N], DisclosureError> {
        self.fixed(N)?
            .try_into()
            .map_err(|_| DisclosureError::MalformedPayload)
    }

    pub(super) fn u8(&mut self) -> Result<u8, DisclosureError> {
        Ok(self.integer::<1>()?[0])
    }

    pub(super) fn u16(&mut self) -> Result<u16, DisclosureError> {
        Ok(u16::from_be_bytes(self.integer()?))
    }

    pub(super) fn u32(&mut self) -> Result<u32, DisclosureError> {
        Ok(u32::from_be_bytes(self.integer()?))
    }

    pub(super) fn u64(&mut self) -> Result<u64, DisclosureError> {
        Ok(u64::from_be_bytes(self.integer()?))
    }

    pub(super) fn u128(&mut self) -> Result<u128, DisclosureError> {
        Ok(u128::from_be_bytes(self.integer()?))
    }

    pub(super) fn bytes(&mut self, maximum: usize) -> Result<&'a [u8], DisclosureError> {
        let length = usize::try_from(self.u32()?).map_err(|_| DisclosureError::MalformedPayload)?;
        if length > maximum {
            return Err(DisclosureError::MalformedPayload);
        }
        self.fixed(length)
    }

    pub(super) fn finish(self) -> Result<(), DisclosureError> {
        if self.remaining.is_empty() {
            Ok(())
        } else {
            Err(DisclosureError::MalformedPayload)
        }
    }
}
