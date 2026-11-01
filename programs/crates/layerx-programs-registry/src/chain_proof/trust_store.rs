use super::{
    decode::MAX_FIELD_BYTES,
    validators::{TrustedValidator, ValidatorSet},
    ChainProofError, VerifiedChainHead,
};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TrustRoot {
    pub chain_id: String,
    pub height: u64,
    pub time_ms: u64,
    pub header_hash: [u8; 32],
    pub validators: ValidatorSet,
}

impl TrustRoot {
    pub fn from_head(
        head: &VerifiedChainHead,
        validators: ValidatorSet,
    ) -> Result<Self, ChainProofError> {
        let root = Self {
            chain_id: head.chain_id().to_owned(),
            height: u64::try_from(head.height()).map_err(|_| ChainProofError::NonCanonical)?,
            time_ms: u64::try_from(head.time_ms()).map_err(|_| ChainProofError::NonCanonical)?,
            header_hash: head.header_hash(),
            validators,
        };
        root.canonical_bytes()?;
        Ok(root)
    }

    pub fn canonical_bytes(&self) -> Result<Vec<u8>, ChainProofError> {
        if self.chain_id.is_empty() || self.height == 0 {
            return Err(ChainProofError::NonCanonical);
        }
        let mut bytes = b"LXTR\x01".to_vec();
        put_field(&mut bytes, self.chain_id.as_bytes())?;
        put_field(&mut bytes, &self.height.to_be_bytes())?;
        put_field(&mut bytes, &self.time_ms.to_be_bytes())?;
        put_field(&mut bytes, &self.header_hash)?;
        let mut validators: Vec<_> = self.validators.validators().iter().collect();
        validators.sort_unstable_by_key(|validator| validator.address);
        let count = u32::try_from(validators.len()).map_err(|_| ChainProofError::NonCanonical)?;
        bytes.extend_from_slice(&count.to_be_bytes());
        for validator in validators {
            put_field(&mut bytes, &validator.address)?;
            put_field(&mut bytes, &validator.public_key)?;
            put_field(&mut bytes, &validator.voting_power.to_be_bytes())?;
        }
        Ok(bytes)
    }

    pub fn decode(bytes: &[u8]) -> Result<Self, ChainProofError> {
        let mut reader = TrustRootReader { remaining: bytes };
        if reader.take(4)? != b"LXTR" || reader.take(1)? != [1] {
            return Err(ChainProofError::NonCanonical);
        }
        let chain_id = std::str::from_utf8(reader.field()?)
            .map_err(|_| ChainProofError::NonCanonical)?
            .to_owned();
        let height = u64::from_be_bytes(reader.array()?);
        let time_ms = u64::from_be_bytes(reader.array()?);
        let header_hash = reader.array()?;
        let count = usize::try_from(u32::from_be_bytes(reader.take_array()?))
            .map_err(|_| ChainProofError::NonCanonical)?;
        if chain_id.is_empty() || height == 0 {
            return Err(ChainProofError::NonCanonical);
        }
        // Each validator occupies three length prefixes and 60 payload bytes.
        if count > reader.remaining.len() / 72 {
            return Err(ChainProofError::Truncated);
        }
        let mut validators: Vec<TrustedValidator> = Vec::new();
        for _ in 0..count {
            let address = reader.array()?;
            if validators
                .last()
                .is_some_and(|previous| previous.address >= address)
            {
                return Err(ChainProofError::NonCanonical);
            }
            validators.push(TrustedValidator {
                address,
                public_key: reader.array()?,
                voting_power: u64::from_be_bytes(reader.array()?),
            });
        }
        if !reader.remaining.is_empty() {
            return Err(ChainProofError::NonCanonical);
        }
        validators.sort_unstable_by(|left, right| {
            right
                .voting_power
                .cmp(&left.voting_power)
                .then_with(|| left.address.cmp(&right.address))
        });
        Ok(Self {
            chain_id,
            height,
            time_ms,
            header_hash,
            validators: ValidatorSet::new(validators)?,
        })
    }
}

fn put_field(bytes: &mut Vec<u8>, field: &[u8]) -> Result<(), ChainProofError> {
    if field.len() > MAX_FIELD_BYTES {
        return Err(ChainProofError::NonCanonical);
    }
    let length = u32::try_from(field.len()).map_err(|_| ChainProofError::NonCanonical)?;
    bytes.extend_from_slice(&length.to_be_bytes());
    bytes.extend_from_slice(field);
    Ok(())
}

struct TrustRootReader<'a> {
    remaining: &'a [u8],
}

impl<'a> TrustRootReader<'a> {
    fn take(&mut self, length: usize) -> Result<&'a [u8], ChainProofError> {
        let (field, remaining) = self
            .remaining
            .split_at_checked(length)
            .ok_or(ChainProofError::Truncated)?;
        self.remaining = remaining;
        Ok(field)
    }

    fn take_array<const N: usize>(&mut self) -> Result<[u8; N], ChainProofError> {
        self.take(N)?
            .try_into()
            .map_err(|_| ChainProofError::NonCanonical)
    }

    fn field(&mut self) -> Result<&'a [u8], ChainProofError> {
        let length = usize::try_from(u32::from_be_bytes(self.take_array()?))
            .map_err(|_| ChainProofError::NonCanonical)?;
        if length > MAX_FIELD_BYTES {
            return Err(ChainProofError::NonCanonical);
        }
        self.take(length)
    }

    fn array<const N: usize>(&mut self) -> Result<[u8; N], ChainProofError> {
        self.field()?
            .try_into()
            .map_err(|_| ChainProofError::NonCanonical)
    }
}
