//! Versioned native-send consent carriers. Authority and signature verification
//! belong to the consumer; structural validation never establishes either.

use super::{
    native_purpose_fixed, native_purpose_hex32, native_purpose_text, strict_hex32, AgentDid,
    CapabilityId, ContractError, NativeActivity, SessionId, TenantId,
};

/// A stable principal's consent to one prepared EIP-1559 Send on Paxeer.
/// The consent key authenticates this statement; `evm_sender` is independently
/// recovered from the transaction signature. Neither is derived from the DID.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NativeSendPurposeV2 {
    pub tenant: TenantId,
    pub agent_did: AgentDid,
    pub owner_did: AgentDid,
    pub consent_public_key: [u8; 32],
    pub evm_sender: [u8; 20],
    pub session_id: SessionId,
    pub generation: u64,
    pub expires_at_ms: u64,
    pub capability_id: CapabilityId,
    pub transaction_type: u8,
    pub chain_id: u64,
    pub activity: NativeActivity,
    pub preparation_id: [u8; 32],
    /// SHA-256 of the exact prepared unsigned type-2 transaction bytes.
    pub canonical_digest: [u8; 32],
    pub economic_action: [u8; 32],
    pub idempotency_key: [u8; 32],
    pub commitment: [u8; 32],
}

impl NativeSendPurposeV2 {
    pub const VERSION: u8 = 2;
    pub const DOMAIN: &'static [u8] = b"LayerX/native/send-purpose/v2\0";
    pub const CHAIN_ID: u64 = 125;
    pub const TRANSACTION_TYPE: u8 = 2;
    pub const MAX_CANONICAL_BYTES: usize = 2048;

    /// # Errors
    /// Returns an error for malformed identifiers, zero keys or commitments, a zero lifetime, or a wrong transaction type, chain or activity.
    pub fn validate(self) -> Result<Self, ContractError> {
        self.validate_fields()?;
        Ok(self)
    }

    fn validate_fields(&self) -> Result<(), ContractError> {
        let tenant = self.tenant.as_str().as_bytes();
        if tenant.is_empty() || tenant.len() > 255 || tenant.contains(&0) {
            return Err(ContractError::Malformed("tenant"));
        }
        for did in [&self.agent_did, &self.owner_did] {
            layerx_types::ids::Did::new(did.as_str().as_bytes())
                .map_err(|_| ContractError::Malformed("native_send.did"))?;
            let identifier = did
                .as_str()
                .strip_prefix("did:layerx:")
                .ok_or(ContractError::Malformed("native_send.did"))?;
            strict_hex32(identifier, "native_send.did")?;
        }
        if self.consent_public_key == [0; 32] {
            return Err(ContractError::Zero("native_send.consent_public_key"));
        }
        if self.evm_sender == [0; 20] {
            return Err(ContractError::Zero("native_send.evm_sender"));
        }
        self.session_id.to_bytes()?;
        self.capability_id.to_bytes()?;
        if self.generation == 0 || self.expires_at_ms == 0 {
            return Err(ContractError::Zero("native_send.lifetime"));
        }
        if self.transaction_type != Self::TRANSACTION_TYPE
            || self.chain_id != Self::CHAIN_ID
            || self.activity != NativeActivity::new(1, 5)?
        {
            return Err(ContractError::Mismatch("native_send.domain"));
        }
        for (name, value) in [
            ("preparation_id", self.preparation_id),
            ("canonical_digest", self.canonical_digest),
            ("economic_action", self.economic_action),
            ("idempotency_key", self.idempotency_key),
            ("commitment", self.commitment),
        ] {
            if value == [0; 32] {
                return Err(ContractError::Zero(name));
            }
        }
        Ok(())
    }

    /// Compare with independently authenticated, current owner resolution.
    /// A matching key alone never authorizes another stable principal.
    /// # Errors
    /// Returns a purpose validation error or a mismatch with the current owner DID, consent key or EVM sender.
    pub fn check_current_owner(
        &self,
        owner_did: &AgentDid,
        consent_public_key: &[u8; 32],
        evm_sender: &[u8; 20],
    ) -> Result<(), ContractError> {
        self.validate_fields()?;
        if &self.owner_did != owner_did
            || &self.consent_public_key != consent_public_key
            || &self.evm_sender != evm_sender
        {
            return Err(ContractError::Mismatch("native_send.current_owner"));
        }
        Ok(())
    }

    /// Compare with the retained preparation and independently recovered sender.
    /// Consumers must additionally check session, expiry, capability and economic
    /// commitment against their current authoritative state before release.
    /// # Errors
    /// Returns a purpose validation error or a mismatch with the prepared transaction type, chain, recovered sender or digest.
    pub fn check_prepared_transaction(
        &self,
        transaction_type: u8,
        chain_id: u64,
        recovered_sender: &[u8; 20],
        canonical_digest: &[u8; 32],
    ) -> Result<(), ContractError> {
        self.validate_fields()?;
        if transaction_type != self.transaction_type
            || chain_id != self.chain_id
            || recovered_sender != &self.evm_sender
            || canonical_digest != &self.canonical_digest
        {
            return Err(ContractError::Mismatch("native_send.prepared_transaction"));
        }
        Ok(())
    }

    /// Domain and version, length-prefixed UTF-8 tenant/agent/owner, key32,
    /// sender20, session32, generation/expiry u64BE, capability32, type u8,
    /// chain u64BE, activity5, then the five digest32 fields in declaration order.
    /// The consent signature covers SHA-256 of exactly these bytes.
    /// # Errors
    /// Returns a purpose validation error or an error if a text length or the complete frame exceeds its encoding bound.
    pub fn canonical_bytes(&self) -> Result<Vec<u8>, ContractError> {
        self.validate_fields()?;
        let mut bytes = Self::DOMAIN.to_vec();
        bytes.push(Self::VERSION);
        for (field, value) in [
            ("tenant", self.tenant.as_str()),
            ("agent_did", self.agent_did.as_str()),
            ("owner_did", self.owner_did.as_str()),
        ] {
            let length =
                u32::try_from(value.len()).map_err(|_| ContractError::OutOfRange(field))?;
            bytes.extend_from_slice(&length.to_be_bytes());
            bytes.extend_from_slice(value.as_bytes());
        }
        bytes.extend_from_slice(&self.consent_public_key);
        bytes.extend_from_slice(&self.evm_sender);
        bytes.extend_from_slice(&self.session_id.to_bytes()?);
        bytes.extend_from_slice(&self.generation.to_be_bytes());
        bytes.extend_from_slice(&self.expires_at_ms.to_be_bytes());
        bytes.extend_from_slice(&self.capability_id.to_bytes()?);
        bytes.push(self.transaction_type);
        bytes.extend_from_slice(&self.chain_id.to_be_bytes());
        bytes.extend_from_slice(&self.activity.encode()?);
        for value in [
            self.preparation_id,
            self.canonical_digest,
            self.economic_action,
            self.idempotency_key,
            self.commitment,
        ] {
            bytes.extend_from_slice(&value);
        }
        if bytes.len() > Self::MAX_CANONICAL_BYTES {
            return Err(ContractError::OutOfRange("native_send.length"));
        }
        Ok(bytes)
    }

    /// # Errors
    /// Returns an error for a malformed, truncated, oversized or noncanonical frame, a wrong domain or version, or invalid purpose fields.
    pub fn from_canonical_bytes(bytes: &[u8]) -> Result<Self, ContractError> {
        if bytes.len() > Self::MAX_CANONICAL_BYTES {
            return Err(ContractError::OutOfRange("native_send.length"));
        }
        let mut rest = bytes
            .strip_prefix(Self::DOMAIN)
            .ok_or(ContractError::Mismatch("native_send.domain"))?;
        if take::<1>(&mut rest)? != [Self::VERSION] {
            return Err(ContractError::Mismatch("native_send.version"));
        }
        let (tenant, next) = native_purpose_text(rest, 255, "tenant")?;
        let (agent, next) =
            native_purpose_text(next, layerx_types::limits::MAX_DID_BYTES, "agent_did")?;
        let (owner, next) =
            native_purpose_text(next, layerx_types::limits::MAX_DID_BYTES, "owner_did")?;
        rest = next;
        let value = Self {
            tenant: TenantId::new(tenant)?,
            agent_did: AgentDid::new(agent)?,
            owner_did: AgentDid::new(owner)?,
            consent_public_key: take(&mut rest)?,
            evm_sender: take(&mut rest)?,
            session_id: SessionId::new(native_purpose_hex32(&take(&mut rest)?))?,
            generation: u64::from_be_bytes(take(&mut rest)?),
            expires_at_ms: u64::from_be_bytes(take(&mut rest)?),
            capability_id: CapabilityId::new(native_purpose_hex32(&take(&mut rest)?))?,
            transaction_type: take::<1>(&mut rest)?[0],
            chain_id: u64::from_be_bytes(take(&mut rest)?),
            activity: NativeActivity::decode(&take::<5>(&mut rest)?)?,
            preparation_id: take(&mut rest)?,
            canonical_digest: take(&mut rest)?,
            economic_action: take(&mut rest)?,
            idempotency_key: take(&mut rest)?,
            commitment: take(&mut rest)?,
        }
        .validate()?;
        if !rest.is_empty() || value.canonical_bytes()?.as_slice() != bytes {
            return Err(ContractError::Malformed("native_send.canonical_bytes"));
        }
        Ok(value)
    }
}

fn take<const N: usize>(bytes: &mut &[u8]) -> Result<[u8; N], ContractError> {
    let (value, rest) = bytes
        .split_at_checked(N)
        .ok_or(ContractError::Malformed("native_send.length"))?;
    let value = native_purpose_fixed(value)?;
    *bytes = rest;
    Ok(value)
}

/// Ed25519 consent over SHA-256 of the purpose's canonical bytes, verified with
/// its current `consent_public_key`. The EVM signature remains a separate proof.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SignedNativeSendPurposeV2 {
    pub purpose: NativeSendPurposeV2,
    pub signature: [u8; 64],
}

impl SignedNativeSendPurposeV2 {
    pub const DOMAIN: &'static [u8] = b"LayerX/native/signed-send-purpose/v2\0";
    pub const MAX_CANONICAL_BYTES: usize =
        Self::DOMAIN.len() + 4 + NativeSendPurposeV2::MAX_CANONICAL_BYTES + 64;

    /// # Errors
    /// Returns a purpose validation error or an error for an all-zero signature.
    pub fn validate(self) -> Result<Self, ContractError> {
        self.purpose.validate_fields()?;
        if self.signature == [0; 64] {
            return Err(ContractError::Zero("native_send.signature"));
        }
        Ok(self)
    }

    /// The signed frame is domain, u32BE purpose length, canonical purpose, sig64.
    /// # Errors
    /// Returns an error for invalid purpose fields, an all-zero signature, or an unrepresentable purpose length.
    pub fn canonical_bytes(&self) -> Result<Vec<u8>, ContractError> {
        self.clone().validate()?;
        let purpose = self.purpose.canonical_bytes()?;
        let length = u32::try_from(purpose.len())
            .map_err(|_| ContractError::OutOfRange("native_send.length"))?;
        let mut bytes = Self::DOMAIN.to_vec();
        bytes.extend_from_slice(&length.to_be_bytes());
        bytes.extend_from_slice(&purpose);
        bytes.extend_from_slice(&self.signature);
        Ok(bytes)
    }

    /// # Errors
    /// Returns an error for an oversized, truncated or malformed frame, a wrong domain, invalid purpose fields, or an all-zero signature.
    pub fn from_canonical_bytes(bytes: &[u8]) -> Result<Self, ContractError> {
        if bytes.len() > Self::MAX_CANONICAL_BYTES {
            return Err(ContractError::OutOfRange("native_send.length"));
        }
        let mut rest = bytes
            .strip_prefix(Self::DOMAIN)
            .ok_or(ContractError::Mismatch("native_send.domain"))?;
        let length = usize::try_from(u32::from_be_bytes(take(&mut rest)?))
            .map_err(|_| ContractError::OutOfRange("native_send.length"))?;
        let (purpose, signature) = rest
            .split_at_checked(length)
            .ok_or(ContractError::Malformed("native_send.length"))?;
        Self {
            purpose: NativeSendPurposeV2::from_canonical_bytes(purpose)?,
            signature: native_purpose_fixed(signature)?,
        }
        .validate()
    }
}
