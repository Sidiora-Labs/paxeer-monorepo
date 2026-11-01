//! External-by-default signing and bounded protocol session-key self-signing.

use std::collections::BTreeSet;
use std::fs;
use std::os::unix::fs::{MetadataExt, PermissionsExt};
use std::path::PathBuf;

use layerx_crypto::SignatureMessage;
use layerx_types::activity::Signature;
use layerx_wire::activity::{
    decode_signed as decode_kernel_signed, encode_signed, encode_signed_envelope, encode_unsigned,
};
use layerx_wire::hash::{activity_id, Domain};

use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
use layerx_crypto::session::{issue_session_key, IssuedSessionKey, SessionKeyRequest};
use layerx_crypto::signer::{SignError, Signer};
use layerx_types::activity::{ActivityBuildError, Authority};
use layerx_types::payload::{ActivityType, ModuleRegistry};
use layerx_wire::WireError;
use zeroize::Zeroizing;

use crate::prepare::{
    verify_disclosure_binding, DisclosureBindingError, DisclosureDigest, EvmPrepared, Prepared,
};

#[path = "sign_verify.rs"]
mod verification;

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

/// EVM preparation paired with the actor and the existing session authorization.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SigningPreparation {
    pub transaction: EvmPrepared,
    pub actor_address: [u8; 20],
    pub authorization: Prepared,
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

/// Provisioned signer whose scope is the exact issued protocol authority.
pub struct ProvisionedSessionKey {
    seed: Zeroizing<[u8; 32]>,
    session_public_key: [u8; 32],
    authority: Authority,
    permitted_activity_types: BTreeSet<ActivityType>,
    expires_at: u64,
    revocation_sequence: u64,
    revocation_marker: Option<(PathBuf, u32)>,
}

impl std::fmt::Debug for ProvisionedSessionKey {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("ProvisionedSessionKey")
            .field("signer", &"[redacted]")
            .field("expires_at", &self.expires_at)
            .field("revocation_sequence", &self.revocation_sequence)
            .finish_non_exhaustive()
    }
}

impl ProvisionedSessionKey {
    /// Reconstructs a provisioned signer while an encrypted keystore lends
    /// its zeroizing plaintext seed to the caller.
    ///
    /// # Errors
    ///
    /// Returns the same exact-scope validation failures as [`Self::new`].
    pub fn from_seed(seed: &[u8; 32], issued: IssuedSessionKey) -> Result<Self, SigningError> {
        Self::new(*seed, issued)
    }

    /// Imports key material only when it matches an issued protocol session authority.
    ///
    /// # Errors
    ///
    /// Returns `KeyMismatch` when the seed derives a different public key, and
    /// `InvalidProvisioning` for an empty permitted activity set or a zero expiry or revocation
    /// sequence.
    pub fn new(seed: [u8; 32], issued: IssuedSessionKey) -> Result<Self, SigningError> {
        let signer = layerx_crypto::local::LocalSigner::new(seed);
        if signer.public_key() != issued.session_public_key {
            return Err(SigningError::KeyMismatch);
        }
        if issued.purpose != layerx_crypto::session::SessionPurpose::Activity
            || issued.permitted_activity_types.is_empty()
            || issued.expires_at == 0
            || issued.revocation_sequence == 0
        {
            return Err(SigningError::InvalidProvisioning);
        }
        Ok(Self {
            seed: Zeroizing::new(seed),
            session_public_key: signer.public_key(),
            authority: issued.authority,
            permitted_activity_types: issued.permitted_activity_types.into_iter().collect(),
            expires_at: issued.expires_at,
            revocation_sequence: issued.revocation_sequence,
            revocation_marker: None,
        })
    }

    pub(crate) fn bind_revocation_marker(mut self, marker: PathBuf, owner_uid: u32) -> Self {
        self.revocation_marker = Some((marker, owner_uid));
        self
    }

    fn ensure_not_durably_revoked(&self) -> Result<(), SigningError> {
        let Some((marker, owner_uid)) = &self.revocation_marker else {
            return Ok(());
        };
        match fs::symlink_metadata(marker) {
            Ok(metadata) => {
                if !metadata.file_type().is_file()
                    || metadata.file_type().is_symlink()
                    || metadata.uid() != *owner_uid
                    || metadata.permissions().mode() & 0o077 != 0
                {
                    return Err(SigningError::Revoked);
                }
                fs::read(marker).map_err(|_| SigningError::Revoked)?;
                Err(SigningError::Revoked)
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
            Err(_) => Err(SigningError::Revoked),
        }
    }
}

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

/// Returns the default key-free package for an external signer.
///
/// # Errors
///
/// Returns the disclosure binding failure when the preparation no longer re-encodes to its exact
/// canonical bytes and digest.
pub fn external(prepared: &SigningPreparation) -> Result<ExternalSigningPackage, SigningError> {
    verify_disclosure_binding(&prepared.authorization).map_err(SigningError::Disclosure)?;
    if prepared.transaction.signing_hash != prepared.transaction.call.signing_hash() {
        return Err(SigningError::PreparedBytesChanged);
    }
    Ok(ExternalSigningPackage {
        canonical_bytes: prepared.authorization.canonical_bytes.clone(),
        signing_preimage: prepared.transaction.signing_hash,
        disclosure_digest: prepared.authorization.disclosure_digest,
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
    provisioned.ensure_not_durably_revoked()?;
    verify_disclosure_binding(&prepared.authorization).map_err(SigningError::Disclosure)?;
    if protocol_time >= provisioned.expires_at {
        return Err(SigningError::Expired);
    }
    if current_revocation_sequence != provisioned.revocation_sequence {
        return Err(SigningError::Revoked);
    }
    let activity_type = prepared.authorization.envelope.activity_type();
    if !provisioned
        .permitted_activity_types
        .contains(&activity_type)
    {
        return Err(SigningError::ScopeDenied);
    }
    if prepared.authorization.envelope.authority().as_bytes() != provisioned.authority.as_bytes() {
        return Err(SigningError::AuthorityMismatch);
    }
    if prepared.transaction.signing_hash != prepared.transaction.call.signing_hash() {
        return Err(SigningError::PreparedBytesChanged);
    }
    let raw = prepared
        .transaction
        .call
        .sign(&provisioned.seed)
        .map_err(|_| SigningError::Crypto(SignError::KeyRejected))?;
    let (call, signature) = decode_signed(&raw).map_err(|_| SigningError::SignatureInvalid)?;
    let signer_address =
        recover_sender(&call, &signature).map_err(|_| SigningError::SignatureInvalid)?;
    let public_key = provisioned.session_public_key;
    let authority_bytes = provisioned.authority.as_bytes().to_vec();
    let signature_bytes = *signature.as_bytes();
    let signed_canonical_bytes = verification::attach_signature(prepared, signature_bytes)?;
    Ok(SignedPreparation {
        canonical_bytes: prepared.authorization.canonical_bytes.clone(),
        signed_canonical_bytes,
        signature: signature_bytes,
        signer_address,
        mode: SigningMode::ProtocolSessionKey,
        audit: SelfSigningAudit {
            mode: SigningMode::ProtocolSessionKey,
            session_public_key: public_key,
            activity_type,
            disclosure_digest: prepared.authorization.disclosure_digest,
            authority_bytes,
            expires_at: provisioned.expires_at,
            revocation_sequence: provisioned.revocation_sequence,
        },
    })
}

/// Attaches a returned external signature to the prepared EIP-1559 call.
///
/// # Errors
///
/// Refuses invalid signatures and signatures recovering to a different prepared actor.
pub fn attach_external_signature(
    prepared: &SigningPreparation,
    signature: [u8; 65],
) -> Result<Vec<u8>, SigningError> {
    verify_disclosure_binding(&prepared.authorization).map_err(SigningError::Disclosure)?;
    verification::attach_signature(prepared, signature)
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

pub fn attach_kernel_signature(
    prepared: &Prepared,
    signature: [u8; 64],
) -> Result<Vec<u8>, SigningError> {
    verify_disclosure_binding(prepared).map_err(SigningError::Disclosure)?;
    let signature = Signature::new(&signature).map_err(SigningError::SignatureEncoding)?;
    let signed = prepared.envelope.clone().attach_signature(signature);
    encode_signed_envelope(&signed).map_err(SigningError::Wire)
}

pub fn verify_kernel_before_submit(
    signed_canonical_bytes: &[u8],
    prepared: &Prepared,
    signer_public_key: &[u8; 32],
    registry: &ModuleRegistry,
) -> Result<VerifiedSubmission, SigningError> {
    verify_disclosure_binding(prepared).map_err(SigningError::Disclosure)?;
    let activity =
        decode_kernel_signed(signed_canonical_bytes, registry).map_err(SigningError::Wire)?;
    if encode_signed(&activity).map_err(SigningError::Wire)? != signed_canonical_bytes {
        return Err(SigningError::PreparedBytesChanged);
    }
    let unsigned = encode_unsigned(&activity).map_err(SigningError::Wire)?;
    if unsigned != prepared.canonical_bytes {
        return Err(SigningError::PreparedBytesChanged);
    }
    let signature = activity.signature().ok_or(SigningError::SignatureInvalid)?;
    let signature: &[u8; 64] = signature
        .try_into()
        .map_err(|_| SigningError::SignatureInvalid)?;
    let message = SignatureMessage::new(
        Domain::SignaturePreimage,
        activity.protocol_version(),
        activity.network_id(),
        &unsigned,
    )
    .map_err(|_| SigningError::SignatureInvalid)?;
    let Authority::Owner(owner) = prepared.envelope.authority() else {
        return Err(SigningError::AuthorityMismatch);
    };
    if owner.as_ref() != signer_public_key.as_slice() {
        return Err(SigningError::AuthorityMismatch);
    }
    layerx_crypto::ed25519::verify(signer_public_key, signature, message)
        .map_err(|_| SigningError::SignatureInvalid)?;
    let activity_id = activity_id(&activity).map_err(SigningError::Wire)?;
    Ok(VerifiedSubmission {
        signed_canonical_bytes: signed_canonical_bytes.to_vec(),
        audit: SubmissionBindingAudit {
            activity_id,
            idempotency_key: activity.idempotency_key(),
            signed_byte_length: signed_canonical_bytes.len(),
        },
    })
}
