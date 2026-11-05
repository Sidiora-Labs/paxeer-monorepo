//! External-by-default signing and bounded protocol session-key self-signing.

use std::collections::BTreeSet;

use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
use layerx_crypto::session::{issue_session_key, IssuedSessionKey, SessionKeyRequest};
use layerx_crypto::signer::SignError;
use layerx_types::activity::ActivityBuildError;
use layerx_types::payload::{ActivityType, ModuleRegistry};
use layerx_wire::WireError;

use crate::prepare::{
    verify_call_binding, CallBindingError, DisclosureBindingError, DisclosureDigest, EvmPrepared,
};

#[path = "sign_verify.rs"]
mod verification;

#[path = "sign_legacy.rs"]
mod legacy;
pub use legacy::{attach_kernel_signature, verify_kernel_before_submit, VerifiedKernelArtifact};

pub use verification::{SubmissionBindingAudit, VerifiedSubmission};

/// Signing location made explicit in every response and audit record.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SigningMode {
    External,
    ProtocolSessionKey,
}

/// Key-free package returned for signing outside the daemon.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ExternalSigningPackage {
    pub canonical_bytes: Vec<u8>,
    pub signing_preimage: [u8; 32],
    pub disclosure_digest: DisclosureDigest,
    pub mode: SigningMode,
}

/// Distinct audit evidence for one self-signed preparation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SelfSigningAudit {
    pub mode: SigningMode,
    pub session_public_key: [u8; 32],
    pub activity_type: ActivityType,
    pub disclosure_digest: DisclosureDigest,
    pub authority_bytes: Vec<u8>,
    pub expires_at: u64,
    pub revocation_sequence: u64,
}

/// EVM preparation paired with its signing actor.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SigningPreparation {
    pub transaction: EvmPrepared,
    pub actor_address: [u8; 20],
}

/// Output from a bounded in-daemon protocol session key.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SignedPreparation {
    pub canonical_bytes: Vec<u8>,
    pub signed_canonical_bytes: Vec<u8>,
    pub signature: [u8; 65],
    pub signer_address: [u8; 20],
    pub mode: SigningMode,
    pub audit: SelfSigningAudit,
}

/// The provisioned secp256k1 signer returned by the durable session registry.
pub use crate::session_keys::ProvisionedEvmSessionKey as ProvisionedSessionKey;

/// Reconstructs and validates the exact issued session grant transferred by Human.
///
/// The daemon's existing permission registry is ordinal-based, while the
/// canonical protocol grant independently carries its complete module mask.
/// This function requires the transferred ordinals to equal the canonical
/// range and then reissues the complete module/ordinal Cartesian scope before
/// accepting byte-for-byte equality.
///
/// # Errors
///
/// Returns an error if the registration payload, authority, or permitted session scope is invalid.
pub fn validate_issued_session(
    registration_payload: &[u8],
    grantor: [u8; 32],
    session_public_key: [u8; 32],
    not_before: u64,
    expires_at: u64,
    revocation_sequence: u64,
    permitted_ordinals: &[u16],
) -> Result<IssuedSessionKey, SigningError> {
    let issued = layerx_crypto::session::decode_session_key(registration_payload)
        .map_err(|_| SigningError::InvalidProvisioning)?;
    if issued.purpose != layerx_crypto::session::SessionPurpose::Activity {
        return Err(SigningError::InvalidProvisioning);
    }
    let allowed: BTreeSet<_> = issued
        .permitted_activity_types
        .iter()
        .map(|kind| kind.ordinal())
        .collect();
    let transferred: BTreeSet<_> = permitted_ordinals.iter().copied().collect();
    if issued.session_public_key != session_public_key
        || issued.expires_at != expires_at
        || issued.revocation_sequence != revocation_sequence
        || transferred.len() != permitted_ordinals.len()
        || allowed != transferred
    {
        return Err(SigningError::InvalidProvisioning);
    }
    let mut expected = issue_session_key(&SessionKeyRequest {
        grantor,
        session_public_key,
        not_before,
        expires_at: Some(expires_at),
        permitted_activity_types: issued.permitted_activity_types.clone(),
        revocation_sequence: Some(revocation_sequence),
        fee_budget: issued.fee_budget,
        purpose: issued.purpose,
    })
    .map_err(|_| SigningError::InvalidProvisioning)?;
    if expected.registration_payload != registration_payload {
        return Err(SigningError::InvalidProvisioning);
    }
    expected.registration_payload = issued.registration_payload;
    Ok(expected)
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SigningError {
    Disclosure(DisclosureBindingError),
    NotProvisioned,
    InvalidProvisioning,
    KeyMismatch,
    ScopeDenied,
    Expired,
    Revoked,
    AuthorityMismatch,
    Crypto(SignError),
    SignatureEncoding(ActivityBuildError),
    Wire(WireError),
    PreparedBytesChanged,
    SignatureInvalid,
    SignerMismatch,
}

impl From<CallBindingError> for SigningError {
    fn from(error: CallBindingError) -> Self {
        match error {
            CallBindingError::Disclosure(error) => {
                Self::Disclosure(DisclosureBindingError::Decode(error))
            }
            CallBindingError::DigestMismatch => {
                Self::Disclosure(DisclosureBindingError::DigestMismatch)
            }
            CallBindingError::SigningHashMismatch => Self::PreparedBytesChanged,
        }
    }
}

/// Returns the default key-free package for an external signer.
///
/// # Errors
///
/// Returns the disclosure binding failure when the preparation no longer re-encodes to its exact
/// canonical bytes and digest.
pub fn external(prepared: &SigningPreparation) -> Result<ExternalSigningPackage, SigningError> {
    verify_call_binding(&prepared.transaction).map_err(SigningError::from)?;
    if prepared.transaction.signing_hash != prepared.transaction.call.signing_hash() {
        return Err(SigningError::PreparedBytesChanged);
    }
    Ok(ExternalSigningPackage {
        canonical_bytes: prepared.transaction.call.encode_unsigned(),
        signing_preimage: prepared.transaction.signing_hash,
        disclosure_digest: DisclosureDigest(prepared.transaction.disclosure_digest),
        mode: SigningMode::External,
    })
}

/// Signs only through an explicitly provisioned, unexpired protocol session key.
///
/// # Errors
///
/// Refuses an absent, expired, or revoked session key, an activity type outside its permitted set,
/// and an authority other than the issued one. Also returns the disclosure, signing, encoding, and
/// wire failures raised while producing the signed bytes.
pub async fn self_sign(
    provisioned: Option<&ProvisionedSessionKey>,
    prepared: &SigningPreparation,
    _registry: &ModuleRegistry,
    protocol_time: u64,
    current_revocation_sequence: u64,
) -> Result<SignedPreparation, SigningError> {
    let provisioned = provisioned.ok_or(SigningError::NotProvisioned)?;
    provisioned
        .with_secret(|_| ())
        .map_err(|_| SigningError::Revoked)?;
    verify_call_binding(&prepared.transaction).map_err(SigningError::from)?;
    if protocol_time < provisioned.issued.not_before
        || protocol_time >= provisioned.issued.expires_at
    {
        return Err(SigningError::Expired);
    }
    if current_revocation_sequence != provisioned.issued.revocation_sequence {
        return Err(SigningError::Revoked);
    }
    if provisioned
        .network_id
        .is_some_and(|network| u64::from(network) != prepared.transaction.call.chain_id)
    {
        return Err(SigningError::AuthorityMismatch);
    }
    let activity_type = prepared.transaction.disclosure.activity_type;
    if !provisioned
        .issued
        .permitted_activity_types
        .contains(&activity_type)
    {
        return Err(SigningError::ScopeDenied);
    }
    if prepared.transaction.signing_hash != prepared.transaction.call.signing_hash() {
        return Err(SigningError::PreparedBytesChanged);
    }
    let raw = provisioned
        .with_secret(|seed| prepared.transaction.call.sign(seed))
        .map_err(|_| SigningError::Revoked)?
        .map_err(|_| SigningError::Crypto(SignError::KeyRejected))?;
    let (call, signature) = decode_signed(&raw).map_err(|_| SigningError::SignatureInvalid)?;
    let signer_address =
        recover_sender(&call, &signature).map_err(|_| SigningError::SignatureInvalid)?;
    if signer_address != provisioned.evm_address()
        || signer_address != prepared.actor_address
        || signer_address != prepared.transaction.from
    {
        return Err(SigningError::AuthorityMismatch);
    }
    let public_key = provisioned.issued.session_public_key;
    let authority_bytes = provisioned.issued.authority.as_bytes().to_vec();
    let signature_bytes = *signature.as_bytes();
    let signed_canonical_bytes = verification::attach_signature(prepared, signature_bytes)?;
    Ok(SignedPreparation {
        canonical_bytes: prepared.transaction.call.encode_unsigned(),
        signed_canonical_bytes,
        signature: signature_bytes,
        signer_address,
        mode: SigningMode::ProtocolSessionKey,
        audit: SelfSigningAudit {
            mode: SigningMode::ProtocolSessionKey,
            session_public_key: public_key,
            activity_type,
            disclosure_digest: DisclosureDigest(prepared.transaction.disclosure_digest),
            authority_bytes,
            expires_at: provisioned.issued.expires_at,
            revocation_sequence: provisioned.issued.revocation_sequence,
        },
    })
}

/// Attaches a returned external signature to the prepared EIP-1559 call.
///
/// # Errors
///
/// Refuses invalid signatures and signatures recovering to a different prepared actor.
/// The call and retained disclosure must still agree before accepting the signature.
pub fn attach_external_signature(
    prepared: &SigningPreparation,
    signature: [u8; 65],
) -> Result<Vec<u8>, SigningError> {
    let bytes = verification::attach_signature(prepared, signature)?;
    verify_call_binding(&prepared.transaction).map_err(SigningError::from)?;
    if prepared.actor_address != prepared.transaction.from {
        return Err(SigningError::SignerMismatch);
    }
    Ok(bytes)
}

/// Verifies the exact signed bytes and returns the only submit-capable wrapper.
///
/// # Errors
///
/// Refuses non-canonical EIP-1559 bytes, a decoded call different from the prepared call,
/// or a signature recovering to a different actor or supplied signer address.
/// Also returns the disclosure binding failure.
pub fn verify_before_submit(
    signed_canonical_bytes: &[u8],
    prepared: &SigningPreparation,
    signer_address: &[u8; 20],
    registry: &ModuleRegistry,
) -> Result<VerifiedSubmission, SigningError> {
    verification::verify_exact(signed_canonical_bytes, prepared, signer_address, registry)
}

impl SigningPreparation {
    /// Retains the exact unsigned transaction, actor, and disclosure for crash recovery.
    pub fn retain(
        &self,
        store: &mut crate::store::Store,
        tenant: crate::store::TenantId,
        reference: [u8; 32],
    ) -> Result<(), SigningError> {
        verify_call_binding(&self.transaction).map_err(SigningError::from)?;
        if self.actor_address != self.transaction.from {
            return Err(SigningError::SignerMismatch);
        }
        let key = signing_key(tenant, reference)?;
        let bytes = serde_json::to_vec(&(
            self.actor_address,
            self.transaction.observed_head_height,
            self.transaction.disclosure_digest,
            self.transaction.call.encode_unsigned(),
        ))
        .map_err(|_| SigningError::PreparedBytesChanged)?;
        if let Some(old) = store.get(&key) {
            if old.bytes() != bytes {
                return Err(SigningError::PreparedBytesChanged);
            }
            return Ok(());
        }
        store
            .put_local(key, bytes)
            .map_err(|_| SigningError::PreparedBytesChanged)
    }

    pub fn restore(
        store: &crate::store::Store,
        tenant: crate::store::TenantId,
        reference: [u8; 32],
    ) -> Result<Self, SigningError> {
        let key = signing_key(tenant, reference)?;
        let value = store.get(&key).ok_or(SigningError::PreparedBytesChanged)?;
        if value.class() != crate::store::StorageClass::LocalOnly {
            return Err(SigningError::PreparedBytesChanged);
        }
        let (actor_address, observed_head_height, digest, unsigned): (
            [u8; 20],
            u64,
            [u8; 32],
            Vec<u8>,
        ) = serde_json::from_slice(value.bytes())
            .map_err(|_| SigningError::PreparedBytesChanged)?;
        let call = layerx_crypto::evm_transaction::decode_unsigned(&unsigned)
            .map_err(|_| SigningError::PreparedBytesChanged)?;
        let disclosure = layerx_crypto::disclosure::call::bind_call(&call, actor_address)
            .map_err(|_| SigningError::PreparedBytesChanged)?;
        if disclosure
            .signing_digest()
            .map_err(|_| SigningError::PreparedBytesChanged)?
            != digest
        {
            return Err(SigningError::PreparedBytesChanged);
        }
        let prepared = Self {
            transaction: EvmPrepared {
                from: actor_address,
                signing_hash: call.signing_hash(),
                call,
                disclosure,
                disclosure_digest: digest,
                observed_head_height,
            },
            actor_address,
        };
        verify_call_binding(&prepared.transaction).map_err(SigningError::from)?;
        Ok(prepared)
    }
}

fn signing_key(
    tenant: crate::store::TenantId,
    reference: [u8; 32],
) -> Result<crate::store::TenantKey, SigningError> {
    let mut id = b"evm-signing-preparation:".to_vec();
    id.extend(reference);
    crate::store::TenantKey::new(tenant, crate::store::ObjectKind::Configuration, id)
        .map_err(|_| SigningError::PreparedBytesChanged)
}
