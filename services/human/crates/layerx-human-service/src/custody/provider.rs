use std::fmt::{Debug, Formatter};
use std::future::Future;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpStream};
use std::pin::Pin;
use std::sync::Arc;
use std::time::Duration;

use crate::frame::{ConnectionGate, FrameTransport, Limits, MutualTlsConfig, Tls, TransportError};
use layerx_crypto::disclosure::call::bind_call;
use layerx_crypto::disclosure::Disclosure;
use layerx_crypto::evm_transaction::{recover_sender, Eip1559Call};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::signer::{sign_disclosed_call, AgentSignature, Signer as _};
use layerx_types::clock::{Clock, Deadline};
use sha2::{Digest as _, Sha256};
use zeroize::Zeroizing;

use super::{seal_refusal, CustodyError, EnvelopeKms, KeyClass, KeyEntropy, KmsError};

const PROVIDER_REFERENCE_LIMIT: usize = 4096;
const PROVIDER_FRAME_LIMIT: usize = 2_097_152;
const PROVIDER_MAGIC: &[u8; 4] = b"LXKP";
const PROBE: Contract = Contract::new(1, 0);
const CREATE: Contract = Contract::new(1, 1);
const DESCRIBE: Contract = Contract::new(1, 2);
const ROTATE: Contract = Contract::new(1, 3);
const ROTATE_IF_CURRENT: Contract = Contract::new(2, 3);
const DESTROY: Contract = Contract::new(1, 4);
const SIGN: Contract = Contract::new(7, 5);
const EXPORT: Contract = Contract::new(8, 14);
const ACKNOWLEDGE_EXPORT: Contract = Contract::new(8, 16);
const EXPORT_HANDOFF: Contract = Contract::new(9, 14);
const EXECUTOR_PROBE: Contract = Contract::new(6, 15);
const STATUS_OK: u8 = 0;
const STATUS_REFUSED: u8 = 1;
const STATUS_NOT_FOUND: u8 = 2;
const STATUS_CONFLICT: u8 = 3;
const STATUS_UNAVAILABLE: u8 = 4;
const STATUS_INTEGRITY: u8 = 5;
const STATUS_SELF_CUSTODIED: u8 = 6;

/// One LXKP request contract: the version and operation a request frame
/// carries and its response must echo. Each pair matches exactly one payload
/// layout in the provider's request decoder; no frame is re-versioned after
/// encoding.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
struct Contract {
    version: u16,
    operation: u8,
}

impl Contract {
    const fn new(version: u16, operation: u8) -> Self {
        Self { version, operation }
    }

    /// EVM custody contracts; operations 7 through 13 carry one payload blob.
    const fn evm(operation: u8) -> Option<Self> {
        match operation {
            6..=12 => Some(Self::new(3, operation)),
            13 => Some(Self::new(4, operation)),
            _ => None,
        }
    }

    const fn carries_payload(self) -> bool {
        self.operation >= 7 && self.operation <= 13
    }
}

/// Whether a custody provider may be selected by a production service.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProviderDeployment {
    DevelopmentOnly,
    Production,
}

/// Redacted rotation state returned by the key-management boundary.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum RotationState {
    Stable,
    InProgress,
    Failed,
    Unknown,
}

impl RotationState {
    fn from_code(value: u8) -> Result<Self, KmsError> {
        match value {
            0 => Ok(Self::Stable),
            1 => Ok(Self::InProgress),
            2 => Ok(Self::Failed),
            3 => Ok(Self::Unknown),
            _ => Err(KmsError::InvalidResponse),
        }
    }
}

/// Opaque provider-owned handle. It is persisted but never returned by the
/// human API and its debug representation never reveals its bytes.
#[derive(Clone, Eq, PartialEq)]
pub struct ProviderKeyReference(Vec<u8>);

impl ProviderKeyReference {
    /// Creates a bounded, non-empty provider handle.
    ///
    /// # Errors
    ///
    /// Refuses empty and oversized handles before they reach storage or the
    /// remote boundary.
    pub fn new(bytes: Vec<u8>) -> Result<Self, KmsError> {
        if bytes.is_empty() || bytes.len() > PROVIDER_REFERENCE_LIMIT {
            return Err(KmsError::InvalidReference);
        }
        Ok(Self(bytes))
    }

    /// Borrows the opaque bytes for a provider implementation.
    #[must_use]
    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }
}

impl Drop for ProviderKeyReference {
    fn drop(&mut self) {
        zeroize::Zeroize::zeroize(&mut self.0);
    }
}

impl Debug for ProviderKeyReference {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("ProviderKeyReference")
            .field("bytes", &"[redacted]")
            .field("length", &self.0.len())
            .finish()
    }
}

/// Principal-scoped identity presented on every provider operation.
#[derive(Clone, Eq, PartialEq)]
pub struct PrincipalKeyBinding {
    identity: Vec<u8>,
    digest: [u8; 32],
    network_id: u32,
    class: KeyClass,
}

impl PrincipalKeyBinding {
    /// Reconstructs a remote assertion; the KMS checks it against stored custody
    /// and the service-authorized action. This does not grant authorization.
    /// # Errors
    /// Refuses zero bindings and invalid networks.
    pub fn from_digest(
        digest: [u8; 32],
        network_id: u32,
        class: KeyClass,
    ) -> Result<Self, KmsError> {
        if digest == [0; 32] || network_id == 0 {
            return Err(KmsError::Refused);
        }
        Ok(Self {
            identity: Vec::new(),
            digest,
            network_id,
            class,
        })
    }

    /// Derives a binding from the service's canonical custody identity.
    /// # Errors
    /// Refuses identities whose lengths cannot be encoded.
    pub fn new(
        identity: Vec<u8>,
        network_id: u32,
        class: KeyClass,
        provider_reference: &str,
    ) -> Result<Self, CustodyError> {
        let identity_length =
            u32::try_from(identity.len()).map_err(|_| CustodyError::InvalidKeyReference)?;
        let provider_length = u32::try_from(provider_reference.len())
            .map_err(|_| CustodyError::InvalidKeyReference)?;
        let mut hasher = Sha256::new();
        hasher.update(b"layerx-human/provider-key-binding/v1\0");
        hasher.update(identity_length.to_be_bytes());
        hasher.update(&identity);
        hasher.update(provider_length.to_be_bytes());
        hasher.update(provider_reference.as_bytes());
        hasher.update(network_id.to_be_bytes());
        let digest = hasher.finalize().into();
        Ok(Self {
            identity,
            digest,
            network_id,
            class,
        })
    }

    /// Returns the non-reversible principal/key binding sent to a remote KMS.
    #[must_use]
    pub const fn digest(&self) -> [u8; 32] {
        self.digest
    }

    /// Returns the network whose signatures this key may produce.
    #[must_use]
    pub const fn network_id(&self) -> u32 {
        self.network_id
    }

    /// Returns the primary-key class held by the provider.
    #[must_use]
    pub const fn class(&self) -> KeyClass {
        self.class
    }

    pub(super) fn identity(&self) -> &[u8] {
        &self.identity
    }
}

impl Debug for PrincipalKeyBinding {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("PrincipalKeyBinding")
            .field("identity", &"[redacted]")
            .field("digest", &self.digest)
            .field("network_id", &self.network_id)
            .field("class", &self.class)
            .finish()
    }
}

/// Public key facts returned by create, describe and rotate. Private material
/// is absent by construction.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProviderKeyDescription {
    reference: ProviderKeyReference,
    public_key: [u8; 32],
    binding_digest: [u8; 32],
    rotation: RotationState,
}

impl ProviderKeyDescription {
    /// Constructs a complete provider response.
    ///
    /// # Errors
    ///
    /// Refuses zero public keys and zero binding digests.
    pub fn new(
        reference: ProviderKeyReference,
        public_key: [u8; 32],
        binding_digest: [u8; 32],
        rotation: RotationState,
    ) -> Result<Self, KmsError> {
        if public_key == [0; 32] || binding_digest == [0; 32] {
            return Err(KmsError::InvalidResponse);
        }
        Ok(Self {
            reference,
            public_key,
            binding_digest,
            rotation,
        })
    }

    #[must_use]
    pub fn reference(&self) -> &ProviderKeyReference {
        &self.reference
    }

    #[must_use]
    pub const fn public_key(&self) -> [u8; 32] {
        self.public_key
    }

    #[must_use]
    pub const fn binding_digest(&self) -> [u8; 32] {
        self.binding_digest
    }

    #[must_use]
    pub const fn rotation(&self) -> RotationState {
        self.rotation
    }
}

/// Public descriptors keep the native identity and EVM signing identity explicit.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WalletKeyDescriptor {
    pub native_ed25519: [u8; 32],
    pub evm_secp256k1: [u8; 20],
}

/// Exact disclosure-bound EIP-1559 request admitted to a provider signer.
#[derive(Clone, Copy)]
pub struct ProviderSignRequest<'a> {
    call: &'a Eip1559Call,
    disclosure: &'a Disclosure,
    from: [u8; 20],
    expected_public_key: [u8; 32],
}

impl<'a> ProviderSignRequest<'a> {
    pub const fn new(
        call: &'a Eip1559Call,
        disclosure: &'a Disclosure,
        from: [u8; 20],
        expected_public_key: [u8; 32],
    ) -> Self {
        Self {
            call,
            disclosure,
            from,
            expected_public_key,
        }
    }

    #[must_use]
    pub const fn call(&self) -> &'a Eip1559Call {
        self.call
    }

    #[must_use]
    pub const fn disclosure(&self) -> &'a Disclosure {
        self.disclosure
    }

    #[must_use]
    pub const fn from(&self) -> [u8; 20] {
        self.from
    }

    #[must_use]
    pub const fn expected_public_key(&self) -> [u8; 32] {
        self.expected_public_key
    }

    fn validate(self) -> Result<ValidatedSignRequest, CustodyError> {
        validate_disclosed(self.call, self.disclosure, self.from)
    }
}

pub(super) fn validate_disclosed(
    call: &Eip1559Call,
    disclosure: &Disclosure,
    from: [u8; 20],
) -> Result<ValidatedSignRequest, CustodyError> {
    let rebound = bind_call(call, from)
        .map_err(|error| CustodyError::Sign(layerx_crypto::signer::SignError::from(error)))?;
    if rebound != *disclosure {
        return Err(CustodyError::Sign(
            layerx_crypto::signer::SignError::DisclosureMismatch("call"),
        ));
    }
    Ok(ValidatedSignRequest {
        canonical_digest: call.signing_hash(),
        disclosure: encode_disclosure(disclosure)?,
    })
}

pub(super) fn verify_call_signature(
    call: &Eip1559Call,
    from: [u8; 20],
    signature: &[u8; 65],
) -> Result<(), CustodyError> {
    let invalid = || CustodyError::Sign(layerx_crypto::signer::SignError::ReturnedSignatureInvalid);
    let signature = AgentSignature::new(*signature).map_err(|_| invalid())?;
    if recover_sender(call, &signature).map_err(|_| invalid())? != from {
        return Err(invalid());
    }
    Ok(())
}

impl Debug for ProviderSignRequest<'_> {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("ProviderSignRequest")
            .field("call", &"[redacted]")
            .field("disclosure", &"[validated at provider boundary]")
            .field("expected_public_key", &self.expected_public_key)
            .finish()
    }
}

pub(super) struct ValidatedSignRequest {
    pub(super) canonical_digest: [u8; 32],
    pub(super) disclosure: Vec<u8>,
}

#[derive(Clone, Copy, Debug)]
pub enum NativeConsent<'a> {
    PreparationPurpose(&'a layerx_agent_api::identity::NativePreparationPurposeV1),
    LocalGrant(&'a layerx_agent_api::identity::NativeLocalGrantConsentV1),
    SendPurpose(&'a layerx_agent_api::identity::NativeSendPurposeV2),
}

impl NativeConsent<'_> {
    pub fn digest(self) -> Result<[u8; 32], CustodyError> {
        let bytes = match self {
            Self::PreparationPurpose(value) => value
                .canonical_bytes()
                .map_err(|_| CustodyError::InvalidEvidence)?,
            Self::SendPurpose(value) => value
                .canonical_bytes()
                .map_err(|_| CustodyError::InvalidEvidence)?,
            Self::LocalGrant(value) => {
                layerx_human_kms::attestor::native_local_grant_signing_bytes(value)
                    .map_err(|_| CustodyError::InvalidEvidence)?
            }
        };
        Ok(Sha256::digest(bytes).into())
    }

    pub fn validate_at(
        self,
        now_ms: u64,
        expected_public_key: [u8; 32],
    ) -> Result<(), CustodyError> {
        match self {
            Self::PreparationPurpose(value) => {
                let canonical = value
                    .canonical_bytes()
                    .map_err(|_| CustodyError::InvalidEvidence)?;
                let actual =
                    layerx_agent_api::identity::NativePreparationPurposeV1::from_canonical_bytes(
                        &canonical,
                    )
                    .map_err(|_| CustodyError::InvalidEvidence)?;
                if &actual != value || now_ms >= value.expires_at_ms {
                    return Err(CustodyError::InvalidEvidence);
                }
            }
            Self::LocalGrant(value) => {
                let coordinates = layerx_human_kms::attestor::validate_native_local_grant(value)
                    .map_err(|_| CustodyError::InvalidEvidence)?;
                if value.owner_public_key != expected_public_key
                    || now_ms >= value.expires_at_ms
                    || now_ms < coordinates.created_at_ms
                {
                    return Err(CustodyError::InvalidEvidence);
                }
            }
            Self::SendPurpose(value) => {
                let canonical = value
                    .canonical_bytes()
                    .map_err(|_| CustodyError::InvalidEvidence)?;
                let actual = layerx_agent_api::identity::NativeSendPurposeV2::from_canonical_bytes(
                    &canonical,
                )
                .map_err(|_| CustodyError::InvalidEvidence)?;
                if &actual != value
                    || value.consent_public_key != expected_public_key
                    || now_ms >= value.expires_at_ms
                {
                    return Err(CustodyError::InvalidEvidence);
                }
            }
        }
        Ok(())
    }
}

#[derive(Clone, Copy, Debug)]
pub struct ProviderNativeSignRequest<'a> {
    consent: NativeConsent<'a>,
    expected_public_key: [u8; 32],
    now_ms: u64,
}

impl<'a> ProviderNativeSignRequest<'a> {
    pub(super) const fn new(
        consent: NativeConsent<'a>,
        expected_public_key: [u8; 32],
        now_ms: u64,
    ) -> Self {
        Self {
            consent,
            expected_public_key,
            now_ms,
        }
    }
    pub const fn consent(self) -> NativeConsent<'a> {
        self.consent
    }
    pub const fn expected_public_key(self) -> [u8; 32] {
        self.expected_public_key
    }
    pub const fn now_ms(self) -> u64 {
        self.now_ms
    }
}

/// Future returned by object-safe KMS provider signing.
pub type KmsSignFuture<'a> =
    Pin<Box<dyn Future<Output = Result<[u8; 65], CustodyError>> + Send + 'a>>;

/// Public production custody boundary. Implementations own key creation,
/// description, rotation, destruction and signing; none can return private key
/// material through this interface.
pub trait KmsProvider: Debug + Send + Sync {
    /// Binds all provider calls to one absolute operation deadline. Providers
    /// without a cancellable synchronous transport refuse gateway dispatch.
    fn for_deadline(
        &self,
        _deadline: Deadline,
        _clock: Arc<dyn Clock>,
    ) -> Result<Box<dyn KmsProvider>, KmsError> {
        Err(KmsError::InvalidConfiguration)
    }

    /// Returns the bounded, non-secret provider or partition reference.
    #[must_use]
    fn provider_reference(&self) -> &str;

    /// Declares whether this implementation is permitted in production.
    #[must_use]
    fn deployment(&self) -> ProviderDeployment;

    /// Applies additional admission restrictions before Human exposes its operations.
    /// Providers with a narrower contract must refuse service startup here.
    fn admit_human_service(&self) -> Result<(), KmsError> {
        Ok(())
    }

    /// Probes the authenticated provider boundary.
    ///
    /// # Errors
    ///
    /// Returns the exact availability, authentication or provider refusal.
    fn probe(&self) -> Result<(), KmsError>;

    /// Executes an EVM custody operation; unsupported providers refuse.
    /// # Errors
    /// Returns the provider's authorization or availability refusal.
    fn evm_operation(
        &self,
        _operation: u8,
        _binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
        _payload: &[u8],
    ) -> Result<Vec<u8>, KmsError> {
        Err(KmsError::Refused)
    }

    /// Creates a primary key inside the provider.
    ///
    /// # Errors
    ///
    /// Returns a typed provider refusal and never partial key material.
    fn create_key(&self, binding: &PrincipalKeyBinding)
        -> Result<ProviderKeyDescription, KmsError>;

    /// Development-only deterministic creation used by local fixtures.
    ///
    /// # Errors
    ///
    /// Returns `DevelopmentOnly` unless the implementation is explicitly a
    /// development provider, or its typed creation refusal.
    fn create_development_key(
        &self,
        _binding: &PrincipalKeyBinding,
        _entropy: KeyEntropy,
    ) -> Result<ProviderKeyDescription, KmsError> {
        Err(KmsError::DevelopmentOnly)
    }

    /// Describes an existing provider key without exporting it.
    ///
    /// # Errors
    ///
    /// Returns typed absence, availability, authentication and integrity
    /// refusals.
    fn describe_key(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
    ) -> Result<ProviderKeyDescription, KmsError>;

    /// Rotates an existing provider key and returns its new public facts.
    ///
    /// # Errors
    ///
    /// Returns a typed lifecycle or provider refusal.
    fn rotate_key(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
    ) -> Result<ProviderKeyDescription, KmsError>;

    /// Rotates a stable handle only from the expected current public key.
    ///
    /// # Errors
    /// Refuses providers without compare-and-swap rotation support.
    fn rotate_key_if_current(
        &self,
        _binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
        _expected_public_key: [u8; 32],
    ) -> Result<ProviderKeyDescription, KmsError> {
        Err(KmsError::Refused)
    }

    /// Hands the human primary seed to its owner once and leaves the provider
    /// key self-custodied. Providers without an export ceremony refuse.
    ///
    /// # Errors
    ///
    /// Returns `Refused` from any provider that holds no export ceremony,
    /// `SelfCustodied` for an identity that already exported, and the typed
    /// provider, availability and integrity refusals otherwise.
    fn export_primary_key(
        &self,
        _binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
    ) -> Result<Zeroizing<[u8; 32]>, KmsError> {
        Err(KmsError::Refused)
    }

    /// Publishes or recovers the identical recipient-encrypted delivery.
    fn export_envelope(
        &self,
        _binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
        _ceremony: &layerx_human_kms::export::ExportCeremony,
    ) -> Result<layerx_human_kms::export::ExportEnvelope, KmsError> {
        Err(KmsError::Refused)
    }

    /// Retires delivery only after the recipient proves it decrypted the key.
    fn acknowledge_export(
        &self,
        _binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
        _acknowledgement: &layerx_human_kms::export::ExportAcknowledgement,
    ) -> Result<(), KmsError> {
        Err(KmsError::Refused)
    }

    /// Refuses providers without durable, principal-bound export recovery.
    fn check_export_handoff(
        &self,
        _binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
    ) -> Result<(), KmsError> {
        Err(KmsError::Refused)
    }

    /// Retries the same durable handoff, retaining encrypted material until acknowledgement.
    fn recover_export_handoff(
        &self,
        _binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
        _id: [u8; 32],
    ) -> Result<Zeroizing<[u8; 32]>, KmsError> {
        Err(KmsError::Refused)
    }

    /// Idempotently erases only the acknowledged handoff, never resuming signing.
    fn acknowledge_export_handoff(
        &self,
        _binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
        _id: [u8; 32],
    ) -> Result<(), KmsError> {
        Err(KmsError::Refused)
    }

    /// Destroys a provider key. No key bytes are returned.
    ///
    /// # Errors
    ///
    /// Returns a typed lifecycle or provider refusal.
    fn destroy_key(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
    ) -> Result<(), KmsError>;

    fn sign_native<'a>(
        &'a self,
        _binding: &'a PrincipalKeyBinding,
        _reference: &'a ProviderKeyReference,
        _request: ProviderNativeSignRequest<'a>,
    ) -> Pin<Box<dyn Future<Output = Result<[u8; 64], CustodyError>> + Send + 'a>> {
        Box::pin(async { Err(CustodyError::Kms(KmsError::UnsupportedNativeConsent)) })
    }

    /// Signs only a complete disclosure-bound request and returns only the
    /// public signature bytes.
    ///
    /// # Errors
    ///
    /// The returned future resolves to a disclosure, provider, integrity or
    /// signature refusal.
    fn sign<'a>(
        &'a self,
        binding: &'a PrincipalKeyBinding,
        reference: &'a ProviderKeyReference,
        request: ProviderSignRequest<'a>,
    ) -> KmsSignFuture<'a>;
}

/// Signer handle owned by the service but backed exclusively by a provider
/// key reference. It contains no private material and has no export method.
#[derive(Clone)]
pub struct RemoteCustodySigner {
    provider: Arc<dyn KmsProvider>,
    binding: PrincipalKeyBinding,
    reference: ProviderKeyReference,
    public_key: [u8; 32],
    class: KeyClass,
}

impl RemoteCustodySigner {
    pub(super) fn new(
        provider: Arc<dyn KmsProvider>,
        binding: PrincipalKeyBinding,
        description: ProviderKeyDescription,
        class: KeyClass,
    ) -> Self {
        Self {
            provider,
            binding,
            reference: description.reference,
            public_key: description.public_key,
            class,
        }
    }

    /// Returns the public verification key for this provider handle.
    #[must_use]
    pub const fn public_key(&self) -> [u8; 32] {
        self.public_key
    }

    /// Returns the primary-key class without exposing the provider handle.
    #[must_use]
    pub const fn class(&self) -> KeyClass {
        self.class
    }

    pub async fn sign_native_consent(
        &self,
        consent: NativeConsent<'_>,
        now_ms: u64,
    ) -> Result<[u8; 64], CustodyError> {
        consent.validate_at(now_ms, self.public_key)?;
        let digest = consent.digest()?;
        let signature = self
            .provider
            .sign_native(
                &self.binding,
                &self.reference,
                ProviderNativeSignRequest::new(consent, self.public_key, now_ms),
            )
            .await?;
        layerx_crypto::ed25519::verify_digest(&self.public_key, &signature, &digest).map_err(
            |_| CustodyError::Sign(layerx_crypto::signer::SignError::ReturnedSignatureInvalid),
        )?;
        Ok(signature)
    }

    /// Sends the exact EIP-1559 call and its disclosure to the provider.
    pub async fn sign_call(
        &self,
        canonical_bytes: &[u8],
        disclosure: &Disclosure,
    ) -> Result<[u8; 65], CustodyError> {
        let call = crate::server::agent_runtime::decode_unsigned(canonical_bytes)
            .map_err(|_| CustodyError::Sign(layerx_crypto::signer::SignError::InvalidDisclosure))?;
        let from =
            disclosure.actor.as_slice().try_into().map_err(|_| {
                CustodyError::Sign(layerx_crypto::signer::SignError::InvalidDisclosure)
            })?;
        let request = ProviderSignRequest::new(&call, disclosure, from, self.public_key);
        request.validate()?;
        let signature = self
            .provider
            .sign(&self.binding, &self.reference, request)
            .await?;
        verify_call_signature(&call, from, &signature)?;
        Ok(signature)
    }
}

impl Debug for RemoteCustodySigner {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("RemoteCustodySigner")
            .field("provider", &"[remote KMS/HSM]")
            .field("binding", &self.binding)
            .field("reference", &"[redacted]")
            .field("public_key", &self.public_key)
            .field("class", &self.class)
            .finish()
    }
}

/// Production provider speaking the bounded `LayerX` KMS protocol over mutual
/// TLS to a remote KMS/HSM gateway. Every operation opens a fresh authenticated
/// connection, so a failed connection cannot silently reuse cached authority.
#[derive(Clone)]
pub struct RemoteKmsProvider {
    clock: Arc<dyn Clock>,
    provider_reference: String,
    endpoint: SocketAddr,
    server_name: String,
    tls: MutualTlsConfig,
    gate: ConnectionGate,
    limits: Limits,
    deadline_tls: Option<Arc<rustls::ClientConfig>>,
    operation_deadline: Option<Deadline>,
}

impl RemoteKmsProvider {
    /// Configures a production remote provider from an authenticated transport.
    ///
    /// # Errors
    ///
    /// Refuses invalid provider references, empty server identities and
    /// transport limits before the provider can be selected. Production
    /// startup authenticates the parsed DNS identity through `probe`.
    pub fn new(
        provider_reference: impl Into<String>,
        endpoint: SocketAddr,
        server_name: impl Into<String>,
        tls: MutualTlsConfig,
        limits: Limits,
        clock: Arc<dyn Clock>,
    ) -> Result<Self, CustodyError> {
        let provider_reference = provider_reference.into();
        if provider_reference.is_empty()
            || provider_reference.len() > super::KEY_REFERENCE_LIMIT
            || provider_reference.as_bytes().contains(&0)
        {
            return Err(CustodyError::InvalidKeyReference);
        }
        let server_name = server_name.into();
        if server_name.is_empty() || server_name.as_bytes().contains(&0) {
            return Err(CustodyError::Kms(KmsError::Authentication));
        }
        let limits = limits
            .validate()
            .map_err(|_| CustodyError::Kms(KmsError::InvalidConfiguration))?;
        if limits.maximum_frame_bytes > PROVIDER_FRAME_LIMIT {
            return Err(CustodyError::Kms(KmsError::InvalidConfiguration));
        }
        let deadline_tls = Some(tls.deadline_client());
        Ok(Self {
            clock,
            provider_reference,
            endpoint,
            server_name,
            tls,
            gate: ConnectionGate::new(limits.maximum_connections),
            limits,
            deadline_tls,
            operation_deadline: None,
        })
    }

    /// Supplies the authenticated TLS configuration for the absolute-deadline
    /// transport used by gateway dispatch.
    #[must_use]
    pub fn with_deadline_transport(mut self, config: Arc<rustls::ClientConfig>) -> Self {
        self.deadline_tls = Some(config);
        self
    }

    /// Encodes the same validated signing frame sent by the provider, for
    /// authenticated relays that already resolved public wallet facts.
    pub fn encode_signing_request(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        request: ProviderSignRequest<'_>,
    ) -> Result<Vec<u8>, CustodyError> {
        let validated = request.validate()?;
        if request.call.chain_id != u64::from(binding.network_id) {
            return Err(KmsError::Integrity.into());
        }
        Ok(encode_sign_request(
            &self.provider_reference,
            binding,
            reference,
            request,
            &validated,
        )?)
    }

    fn call(&self, contract: Contract, request: &[u8]) -> Result<Vec<u8>, KmsError> {
        if request.len() > self.limits.maximum_frame_bytes {
            return Err(KmsError::InvalidConfiguration);
        }
        let server_name = self
            .server_name
            .clone()
            .try_into()
            .map_err(|_| KmsError::Authentication)?;
        let mut local_deadline = Deadline::start(self.clock.as_ref(), self.limits.deadline)
            .map_err(|_| KmsError::Unavailable)?;
        let mut deadline = match self.operation_deadline {
            Some(mut inherited) => {
                let inherited_remaining = inherited
                    .remaining(self.clock.as_ref())
                    .map_err(|_| KmsError::Unavailable)?;
                let local_remaining = local_deadline
                    .remaining(self.clock.as_ref())
                    .map_err(|_| KmsError::Unavailable)?;
                if inherited_remaining <= local_remaining {
                    inherited
                } else {
                    local_deadline
                }
            }
            None => local_deadline,
        };
        let mut limits = self.limits;
        limits.deadline = deadline
            .remaining(self.clock.as_ref())
            .map_err(|_| KmsError::Unavailable)?;
        if limits.deadline.is_zero() {
            return Err(KmsError::Timeout);
        }
        let response = if let Some(config) = &self.deadline_tls {
            let _permit = self.gate.acquire().map_err(map_transport)?;
            let tcp = TcpStream::connect_timeout(&self.endpoint, limits.deadline)
                .map_err(|_| KmsError::Timeout)?;
            let socket = DeadlineSocket {
                tcp,
                deadline,
                clock: Arc::clone(&self.clock),
            };
            let connection = rustls::ClientConnection::new(Arc::clone(config), server_name)
                .map_err(|_| KmsError::Authentication)?;
            let mut transport = rustls::StreamOwned::new(connection, socket);
            crate::frame::write_frame(&mut transport, request, limits.maximum_frame_bytes)
                .map_err(map_transport)?;
            crate::frame::read_frame(&mut transport, limits.maximum_frame_bytes)
                .map_err(map_transport)?
        } else {
            if self.operation_deadline.is_some() {
                return Err(KmsError::InvalidConfiguration);
            }
            let mut transport =
                Tls::connect(self.endpoint, server_name, &self.tls, &self.gate, limits)
                    .map_err(map_transport)?;
            transport.send(request).map_err(map_transport)?;
            transport.receive().map_err(map_transport)?
        };
        if deadline
            .remaining(self.clock.as_ref())
            .map_err(|_| KmsError::Unavailable)?
            .is_zero()
        {
            return Err(KmsError::Timeout);
        }
        decode_response(contract, &response)
    }

    fn key_operation(
        &self,
        contract: Contract,
        binding: &PrincipalKeyBinding,
        reference: Option<&ProviderKeyReference>,
    ) -> Result<ProviderKeyDescription, KmsError> {
        let request = encode_key_request(contract, &self.provider_reference, binding, reference)?;
        let response = self.call(contract, &request)?;
        decode_description(&response, binding)
    }

    pub fn wallet_descriptor(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
    ) -> Result<WalletKeyDescriptor, KmsError> {
        let description = self.describe_key(binding, reference)?;
        let address = self.evm_operation(6, binding, reference, &[])?;
        let address = address
            .as_slice()
            .try_into()
            .map_err(|_| KmsError::InvalidResponse)?;
        if address == [0; 20] {
            return Err(KmsError::Integrity);
        }
        Ok(WalletKeyDescriptor {
            native_ed25519: description.public_key(),
            evm_secp256k1: address,
        })
    }

    pub async fn sign_wallet_call(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        call: &Eip1559Call,
        disclosure: &Disclosure,
    ) -> Result<[u8; 65], CustodyError> {
        let descriptor = self.wallet_descriptor(binding, reference)?;
        self.sign(
            binding,
            reference,
            ProviderSignRequest::new(
                call,
                disclosure,
                descriptor.evm_secp256k1,
                descriptor.native_ed25519,
            ),
        )
        .await
    }

    /// Publish or recover the envelope for this authenticated recipient ceremony.
    pub fn export_envelope(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        ceremony: &layerx_human_kms::export::ExportCeremony,
    ) -> Result<layerx_human_kms::export::ExportEnvelope, KmsError> {
        ceremony.validate().map_err(|_| KmsError::Refused)?;
        let frame = self.encode_export_request(binding, reference, ceremony)?;
        let response = self.call(EXPORT, &frame)?;
        let envelope: layerx_human_kms::export::ExportEnvelope =
            serde_json::from_slice(&response).map_err(|_| KmsError::InvalidResponse)?;
        if &envelope.ceremony != ceremony || envelope.ciphertext.len() != 80 {
            return Err(KmsError::Integrity);
        }
        Ok(envelope)
    }

    pub fn encode_export_request(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        ceremony: &layerx_human_kms::export::ExportCeremony,
    ) -> Result<Vec<u8>, KmsError> {
        ceremony.validate().map_err(|_| KmsError::Refused)?;
        self.export_frame(
            EXPORT,
            binding,
            reference,
            &serde_json::to_vec(ceremony).map_err(|_| KmsError::Refused)?,
        )
    }

    /// Send only after the recipient has durably retained the decrypted key.
    /// Repeating a delivered acknowledgement is safe after an uncertain reply.
    pub fn acknowledge_export(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        acknowledgement: &layerx_human_kms::export::ExportAcknowledgement,
    ) -> Result<(), KmsError> {
        let frame = self.encode_export_acknowledgement(binding, reference, acknowledgement)?;
        if !self.call(ACKNOWLEDGE_EXPORT, &frame)?.is_empty() {
            return Err(KmsError::InvalidResponse);
        }
        Ok(())
    }
    pub fn encode_export_acknowledgement(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        acknowledgement: &layerx_human_kms::export::ExportAcknowledgement,
    ) -> Result<Zeroizing<Vec<u8>>, KmsError> {
        let payload =
            Zeroizing::new(serde_json::to_vec(acknowledgement).map_err(|_| KmsError::Refused)?);
        self.export_frame(ACKNOWLEDGE_EXPORT, binding, reference, &payload)
            .map(Zeroizing::new)
    }
    fn export_frame(
        &self,
        contract: Contract,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        payload: &[u8],
    ) -> Result<Vec<u8>, KmsError> {
        let frame =
            encode_key_request(contract, &self.provider_reference, binding, Some(reference))?;
        let mut writer = WireWriter::from_bytes(frame);
        writer.bytes(payload, PROVIDER_FRAME_LIMIT)?;
        Ok(writer.finish())
    }

    /// Encodes the exact frame sent by `evm_operation`, for durable dispatch.
    ///
    /// # Errors
    /// Refuses an unsupported operation, a payload on the wallet query, an
    /// empty JSON payload, or a frame beyond the LXKP limit.
    pub fn encode_evm_request(
        &self,
        operation: u8,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        payload: &[u8],
    ) -> Result<Vec<u8>, KmsError> {
        let contract = Contract::evm(operation).ok_or(KmsError::Refused)?;
        let mut frame =
            encode_key_request(contract, &self.provider_reference, binding, Some(reference))?;
        if contract.carries_payload() {
            if payload.is_empty() {
                return Err(KmsError::Refused);
            }
            let mut writer = WireWriter::from_bytes(frame);
            writer.bytes(payload, PROVIDER_FRAME_LIMIT)?;
            frame = writer.finish();
        } else if !payload.is_empty() {
            return Err(KmsError::Refused);
        }
        Ok(frame)
    }

    /// Completes the read-only executor probe under this provider's client
    /// identity: the provider must admit the restricted executor certificate,
    /// echo a fresh challenge and answer for `network_id`. No record is read
    /// for signing, changed or persisted.
    ///
    /// # Errors
    ///
    /// Returns the typed availability, authentication, timeout or provider
    /// refusal, and `Integrity` when the answer does not bind the challenge
    /// and network.
    pub fn probe_executor(&self, network_id: u32) -> Result<(), KmsError> {
        if network_id == 0 {
            return Err(KmsError::InvalidConfiguration);
        }
        let mut challenge = [0; 32];
        getrandom::fill(&mut challenge).map_err(|_| KmsError::Unavailable)?;
        let mut writer =
            WireWriter::from_bytes(encode_header(EXECUTOR_PROBE, &self.provider_reference)?);
        writer.u32(network_id)?;
        writer.fixed(&challenge)?;
        let response = self.call(EXECUTOR_PROBE, &writer.finish())?;
        let mut reader = WireReader::new(&response);
        let echoed: [u8; 32] = reader
            .fixed(32)?
            .try_into()
            .map_err(|_| KmsError::InvalidResponse)?;
        let network = reader.u32()?;
        reader.finish()?;
        if !layerx_crypto::ct::eq_fixed(&echoed, &challenge) || network != network_id {
            return Err(KmsError::Integrity);
        }
        Ok(())
    }
}

impl Debug for RemoteKmsProvider {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("RemoteKmsProvider")
            .field("provider_reference", &self.provider_reference)
            .field("endpoint", &"[mutually authenticated remote endpoint]")
            .field("server_name", &self.server_name)
            .field("limits", &self.limits)
            .finish_non_exhaustive()
    }
}

impl KmsProvider for RemoteKmsProvider {
    fn for_deadline(
        &self,
        deadline: Deadline,
        clock: Arc<dyn Clock>,
    ) -> Result<Box<dyn KmsProvider>, KmsError> {
        if self.deadline_tls.is_none() {
            return Err(KmsError::InvalidConfiguration);
        }
        let mut provider = self.clone();
        provider.clock = clock;
        provider.operation_deadline = Some(deadline);
        Ok(Box::new(provider))
    }

    fn provider_reference(&self) -> &str {
        &self.provider_reference
    }

    fn deployment(&self) -> ProviderDeployment {
        ProviderDeployment::Production
    }

    fn admit_human_service(&self) -> Result<(), KmsError> {
        Err(KmsError::UnsupportedNativeConsent)
    }

    fn export_envelope(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        ceremony: &layerx_human_kms::export::ExportCeremony,
    ) -> Result<layerx_human_kms::export::ExportEnvelope, KmsError> {
        RemoteKmsProvider::export_envelope(self, binding, reference, ceremony)
    }

    fn acknowledge_export(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        acknowledgement: &layerx_human_kms::export::ExportAcknowledgement,
    ) -> Result<(), KmsError> {
        RemoteKmsProvider::acknowledge_export(self, binding, reference, acknowledgement)
    }

    fn probe(&self) -> Result<(), KmsError> {
        let request = encode_header(PROBE, &self.provider_reference)?;
        let response = self.call(PROBE, &request)?;
        if response.is_empty() {
            Ok(())
        } else {
            Err(KmsError::InvalidResponse)
        }
    }

    fn evm_operation(
        &self,
        operation: u8,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        payload: &[u8],
    ) -> Result<Vec<u8>, KmsError> {
        let contract = Contract::evm(operation).ok_or(KmsError::Refused)?;
        let frame = self.encode_evm_request(operation, binding, reference, payload)?;
        self.call(contract, &frame)
    }

    fn create_key(
        &self,
        binding: &PrincipalKeyBinding,
    ) -> Result<ProviderKeyDescription, KmsError> {
        self.key_operation(CREATE, binding, None)
    }

    fn describe_key(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
    ) -> Result<ProviderKeyDescription, KmsError> {
        self.key_operation(DESCRIBE, binding, Some(reference))
    }

    fn rotate_key(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
    ) -> Result<ProviderKeyDescription, KmsError> {
        self.key_operation(ROTATE, binding, Some(reference))
    }

    fn rotate_key_if_current(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        expected_public_key: [u8; 32],
    ) -> Result<ProviderKeyDescription, KmsError> {
        let mut request = encode_key_request(
            ROTATE_IF_CURRENT,
            &self.provider_reference,
            binding,
            Some(reference),
        )?;
        request.extend_from_slice(&expected_public_key);
        let response = self.call(ROTATE_IF_CURRENT, &request)?;
        decode_description(&response, binding)
    }

    fn export_primary_key(
        &self,
        _binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
    ) -> Result<Zeroizing<[u8; 32]>, KmsError> {
        // The legacy API cannot identify a recipient or acknowledge delivery.
        Err(KmsError::Refused)
    }

    fn check_export_handoff(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
    ) -> Result<(), KmsError> {
        let mut request = encode_key_request(
            EXPORT_HANDOFF,
            &self.provider_reference,
            binding,
            Some(reference),
        )?;
        request.push(0);
        let response = self.call(EXPORT_HANDOFF, &request)?;
        if response.is_empty() {
            Ok(())
        } else {
            Err(KmsError::InvalidResponse)
        }
    }

    fn recover_export_handoff(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        id: [u8; 32],
    ) -> Result<Zeroizing<[u8; 32]>, KmsError> {
        let mut request = encode_key_request(
            EXPORT_HANDOFF,
            &self.provider_reference,
            binding,
            Some(reference),
        )?;
        request.push(1);
        request.extend_from_slice(&id);
        let response = Zeroizing::new(self.call(EXPORT_HANDOFF, &request)?);
        let seed = response
            .as_slice()
            .try_into()
            .map_err(|_| KmsError::InvalidResponse)?;
        Ok(Zeroizing::new(seed))
    }

    fn acknowledge_export_handoff(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        id: [u8; 32],
    ) -> Result<(), KmsError> {
        let mut request = encode_key_request(
            EXPORT_HANDOFF,
            &self.provider_reference,
            binding,
            Some(reference),
        )?;
        request.push(2);
        request.extend_from_slice(&id);
        let response = self.call(EXPORT_HANDOFF, &request)?;
        if response.is_empty() {
            Ok(())
        } else {
            Err(KmsError::InvalidResponse)
        }
    }

    fn destroy_key(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
    ) -> Result<(), KmsError> {
        let request =
            encode_key_request(DESTROY, &self.provider_reference, binding, Some(reference))?;
        let response = self.call(DESTROY, &request)?;
        if response.is_empty() {
            Ok(())
        } else {
            Err(KmsError::InvalidResponse)
        }
    }

    fn sign<'a>(
        &'a self,
        binding: &'a PrincipalKeyBinding,
        reference: &'a ProviderKeyReference,
        request: ProviderSignRequest<'a>,
    ) -> KmsSignFuture<'a> {
        Box::pin(async move {
            let validated = request.validate()?;
            if request.call.chain_id != u64::from(binding.network_id) {
                return Err(CustodyError::Kms(KmsError::Integrity));
            }
            let descriptor = self.wallet_descriptor(binding, reference)?;
            if descriptor.native_ed25519 != request.expected_public_key
                || descriptor.evm_secp256k1 != request.from
            {
                return Err(CustodyError::Kms(KmsError::Integrity));
            }
            let frame = encode_sign_request(
                &self.provider_reference,
                binding,
                reference,
                request,
                &validated,
            )?;
            let response = self.call(SIGN, &frame)?;
            let signature: [u8; 65] = response
                .as_slice()
                .try_into()
                .map_err(|_| CustodyError::Kms(KmsError::InvalidResponse))?;
            verify_call_signature(request.call, request.from, &signature)?;
            Ok(signature)
        })
    }
}

impl KmsProvider for EnvelopeKms {
    fn provider_reference(&self) -> &str {
        &self.key_reference
    }

    fn deployment(&self) -> ProviderDeployment {
        ProviderDeployment::DevelopmentOnly
    }

    fn probe(&self) -> Result<(), KmsError> {
        self.root_secret().map(|_| ())
    }

    fn evm_operation(
        &self,
        operation: u8,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        payload: &[u8],
    ) -> Result<Vec<u8>, KmsError> {
        use std::io::{Read as _, Seek as _, Write as _};
        use std::os::unix::fs::OpenOptionsExt as _;

        if operation == 11 {
            if payload.len() > PROVIDER_FRAME_LIMIT {
                return Err(KmsError::Refused);
            }
            let request: super::evm::EvmSendAuthorization =
                serde_json::from_slice(payload).map_err(|_| KmsError::Refused)?;
            let transaction = &request.authorization.transaction;
            let call = Eip1559Call {
                chain_id: transaction.chain_id,
                nonce: transaction.nonce,
                max_priority_fee_per_gas: transaction.max_priority_fee_per_gas.into(),
                max_fee_per_gas: transaction.max_fee_per_gas.into(),
                gas_limit: transaction.gas_limit,
                to: transaction.to,
                data: transaction.calldata.clone(),
            };
            let disclosure =
                bind_call(&call, request.authorization.wallet).map_err(|_| KmsError::Refused)?;
            match disclosure
                .construct_method()
                .map_err(|_| KmsError::Refused)?
            {
                Some(layerx_crypto::disclosure::call::ConstructMethod::BankSend {
                    from,
                    to,
                    denom,
                    amount,
                }) if from == request.authorization.wallet
                    && to == request.disclosure.recipient
                    && denom == request.disclosure.denom
                    && amount == request.disclosure.amount =>
                {
                    ()
                }
                _ => return Err(KmsError::Refused),
            }
            let authorization =
                serde_json::to_vec(&request.authorization).map_err(|_| KmsError::Refused)?;
            self.evm_operation(7, binding, reference, &authorization)?;
            let action = serde_json::to_vec(&request.authorization.action_key)
                .map_err(|_| KmsError::Refused)?;
            let signed: super::EvmAction =
                serde_json::from_slice(&self.evm_operation(8, binding, reference, &action)?)
                    .map_err(|_| KmsError::Integrity)?;
            return Ok(signed.raw_transaction);
        }
        if !matches!(operation, 6..=8) || payload.len() > PROVIDER_FRAME_LIMIT {
            return Err(KmsError::Refused);
        }
        let seed = self.export_primary_key(binding, reference)?;
        let key = k256::ecdsa::SigningKey::from_slice(&*seed).map_err(|_| KmsError::Integrity)?;
        let wallet = layerx_crypto::secp256k1::evm_address(
            key.verifying_key().to_encoded_point(false).as_bytes(),
        )
        .map_err(|_| KmsError::Integrity)?;
        if operation == 6 {
            return if payload.is_empty() {
                Ok(wallet.to_vec())
            } else {
                Err(KmsError::Refused)
            };
        }

        let mut hasher = Sha256::new();
        hasher.update(b"layerx-human/envelope-evm-actions/v1\0");
        hasher.update(binding.digest);
        hasher.update(reference.as_bytes());
        let journal_id: String = hasher
            .finalize()
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect();
        let path = self
            .secret_path
            .with_file_name(format!("{journal_id}.evm-actions"));
        let mut journal = std::fs::OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .mode(0o600)
            .open(path)
            .map_err(|_| KmsError::Unavailable)?;
        rustix::fs::flock(&journal, rustix::fs::FlockOperation::LockExclusive)
            .map_err(|_| KmsError::Unavailable)?;
        if journal.metadata().map_err(|_| KmsError::Unavailable)?.len() > 16_777_216 {
            return Err(KmsError::Integrity);
        }
        let mut bytes = Vec::new();
        journal
            .read_to_end(&mut bytes)
            .map_err(|_| KmsError::Unavailable)?;
        let mut actions: Vec<super::EvmAction> = if bytes.is_empty() {
            Vec::new()
        } else {
            serde_json::from_slice(&bytes).map_err(|_| KmsError::Integrity)?
        };
        let now = layerx_client::runtime_clock::RuntimeClock::from_environment()
            .and_then(|clock| clock.sample(Duration::from_secs(1)))
            .map_err(|_| KmsError::Unavailable)?
            .unix_seconds();
        let action_key = if operation == 7 {
            let authorization: super::EvmPlanAuthorization =
                serde_json::from_slice(payload).map_err(|_| KmsError::Refused)?;
            let transaction = &authorization.transaction;
            if authorization.plan_id == [0; 32]
                || authorization.action_key == [0; 32]
                || authorization.binding_digest != binding.digest
                || authorization.wallet != wallet
                || binding.identity().split(|byte| *byte == 0).nth(1)
                    != Some(authorization.principal.as_bytes())
                || authorization.tenant.is_empty()
                || authorization.tenant.len() > 128
                || authorization.not_before > now
                || authorization.not_after < now
                || authorization.not_after <= authorization.not_before
                || transaction.chain_id != u64::from(binding.network_id)
                || transaction.to == [0; 20]
                || transaction.value != [0; 32]
                || transaction.gas_limit < 21_000
                || transaction.max_fee_per_gas == 0
                || transaction.max_priority_fee_per_gas > transaction.max_fee_per_gas
                || transaction.calldata.len() > 524_288
            {
                return Err(KmsError::Refused);
            }
            let action_key = authorization.action_key;
            if let Some(previous) = actions
                .iter()
                .find(|action| action.authorization.action_key == action_key)
            {
                if previous.authorization != authorization {
                    return Err(KmsError::Conflict);
                }
            } else {
                if actions.len() >= 4096 || transaction.nonce == u64::MAX {
                    return Err(KmsError::Refused);
                }
                if actions.iter().any(|action| {
                    action.authorization.transaction.chain_id == transaction.chain_id
                        && action.authorization.transaction.nonce >= transaction.nonce
                }) {
                    return Err(KmsError::Conflict);
                }
                actions.push(super::EvmAction {
                    authorization,
                    raw_transaction: Vec::new(),
                    transaction_hash: None,
                    acknowledged: false,
                });
            }
            action_key
        } else {
            serde_json::from_slice(payload).map_err(|_| KmsError::Refused)?
        };
        let action = actions
            .iter_mut()
            .find(|action| action.authorization.action_key == action_key)
            .ok_or(KmsError::KeyNotFound)?;
        if operation == 8 {
            let authorization = &action.authorization;
            let transaction = &authorization.transaction;
            if authorization.binding_digest != binding.digest
                || authorization.wallet != wallet
                || now < authorization.not_before
                || now > authorization.not_after
                || transaction.chain_id != u64::from(binding.network_id)
                || transaction.value != [0; 32]
            {
                return Err(KmsError::Refused);
            }
            let call = Eip1559Call {
                chain_id: transaction.chain_id,
                nonce: transaction.nonce,
                max_priority_fee_per_gas: u128::from(transaction.max_priority_fee_per_gas),
                max_fee_per_gas: u128::from(transaction.max_fee_per_gas),
                gas_limit: transaction.gas_limit,
                to: transaction.to,
                data: transaction.calldata.clone(),
            };
            action.raw_transaction = call.sign(&seed).map_err(|_| KmsError::Integrity)?;
            action.transaction_hash = Some(layerx_crypto::evm_transaction::transaction_hash(
                &action.raw_transaction,
            ));
        }
        let response = serde_json::to_vec(action).map_err(|_| KmsError::Integrity)?;
        let bytes = serde_json::to_vec(&actions).map_err(|_| KmsError::Integrity)?;
        journal.rewind().map_err(|_| KmsError::Unavailable)?;
        journal
            .write_all(&bytes)
            .map_err(|_| KmsError::Unavailable)?;
        journal
            .set_len(u64::try_from(bytes.len()).map_err(|_| KmsError::Integrity)?)
            .map_err(|_| KmsError::Unavailable)?;
        journal.sync_all().map_err(|_| KmsError::Unavailable)?;
        Ok(response)
    }

    fn create_key(
        &self,
        binding: &PrincipalKeyBinding,
    ) -> Result<ProviderKeyDescription, KmsError> {
        let mut seed = Zeroizing::new([0_u8; 32]);
        let mut salt = [0_u8; 16];
        let mut nonce = [0_u8; 24];
        getrandom::fill(&mut *seed).map_err(|_| KmsError::Unavailable)?;
        getrandom::fill(&mut salt).map_err(|_| KmsError::Unavailable)?;
        getrandom::fill(&mut nonce).map_err(|_| KmsError::Unavailable)?;
        let entropy = KeyEntropy::new(*seed, salt, nonce).map_err(|_| KmsError::Refused)?;
        self.create_development_key(binding, entropy)
    }

    fn create_development_key(
        &self,
        binding: &PrincipalKeyBinding,
        entropy: KeyEntropy,
    ) -> Result<ProviderKeyDescription, KmsError> {
        let KeyEntropy { seed, envelope } = entropy;
        let public_key = LocalSigner::new(*seed).public_key();
        let root = self.root_secret()?;
        let envelope = layerx_crypto::keystore::Keystore::seal(
            &seed,
            &root,
            binding.identity(),
            binding.network_id,
            envelope,
        )
        .map_err(seal_refusal)?;
        let reference = ProviderKeyReference::new(envelope.to_bytes().map_err(seal_refusal)?)?;
        ProviderKeyDescription::new(reference, public_key, binding.digest, RotationState::Stable)
    }

    fn describe_key(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
    ) -> Result<ProviderKeyDescription, KmsError> {
        let root = self.root_secret()?;
        let envelope = layerx_crypto::keystore::Keystore::from_bytes(reference.as_bytes())
            .map_err(seal_refusal)?;
        let seed = envelope
            .open(&root, binding.identity(), binding.network_id)
            .map_err(seal_refusal)?;
        let public_key = LocalSigner::from_secret(seed).public_key();
        ProviderKeyDescription::new(
            reference.clone(),
            public_key,
            binding.digest,
            RotationState::Stable,
        )
    }

    fn rotate_key(
        &self,
        binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
    ) -> Result<ProviderKeyDescription, KmsError> {
        self.create_key(binding)
    }

    fn export_primary_key(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
    ) -> Result<Zeroizing<[u8; 32]>, KmsError> {
        let root = self.root_secret()?;
        let envelope = layerx_crypto::keystore::Keystore::from_bytes(reference.as_bytes())
            .map_err(seal_refusal)?;
        envelope
            .open_with(&root, binding.identity(), binding.network_id, |seed| {
                Zeroizing::new(*seed)
            })
            .map_err(seal_refusal)
    }

    fn check_export_handoff(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
    ) -> Result<(), KmsError> {
        self.describe_key(binding, reference).map(|_| ())
    }

    fn recover_export_handoff(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        _id: [u8; 32],
    ) -> Result<Zeroizing<[u8; 32]>, KmsError> {
        // The authenticated envelope is the development provider's durable recovery material.
        self.export_primary_key(binding, reference)
    }

    fn acknowledge_export_handoff(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        _id: [u8; 32],
    ) -> Result<(), KmsError> {
        self.describe_key(binding, reference).map(|_| ())
    }

    fn destroy_key(
        &self,
        _binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
    ) -> Result<(), KmsError> {
        Ok(())
    }

    fn sign<'a>(
        &'a self,
        binding: &'a PrincipalKeyBinding,
        reference: &'a ProviderKeyReference,
        request: ProviderSignRequest<'a>,
    ) -> KmsSignFuture<'a> {
        Box::pin(async move {
            let _ = request.validate()?;
            let root = self.root_secret()?;
            let envelope = layerx_crypto::keystore::Keystore::from_bytes(reference.as_bytes())
                .map_err(seal_refusal)?;
            let signer = envelope
                .open_with(&root, binding.identity(), binding.network_id, |seed| {
                    let evm = k256::ecdsa::SigningKey::from_slice(seed).map_err(|_| {
                        CustodyError::Sign(layerx_crypto::signer::SignError::KeyRejected)
                    })?;
                    Ok::<_, CustodyError>(EnvelopeTransactionSigner {
                        identity: LocalSigner::new(*seed),
                        evm,
                    })
                })
                .map_err(seal_refusal)??;
            if signer.public_key() != request.expected_public_key
                || signer.evm_address() != Some(request.from)
                || request.call.chain_id != u64::from(binding.network_id)
            {
                return Err(CustodyError::Kms(KmsError::Integrity));
            }
            let signature =
                sign_disclosed_call(&signer, request.call, request.disclosure, request.from)
                    .await
                    .map_err(CustodyError::Sign)?;
            verify_call_signature(request.call, request.from, signature.as_bytes())?;
            Ok(*signature.as_bytes())
        })
    }
}

struct EnvelopeTransactionSigner {
    identity: LocalSigner,
    evm: k256::ecdsa::SigningKey,
}

impl Debug for EnvelopeTransactionSigner {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str("EnvelopeTransactionSigner([redacted])")
    }
}

impl layerx_crypto::signer::Signer for EnvelopeTransactionSigner {
    fn public_key(&self) -> [u8; 32] {
        self.identity.public_key()
    }

    fn sign<'a>(
        &'a self,
        request: layerx_crypto::signer::SigningRequest<'a>,
    ) -> layerx_crypto::signer::SignFuture<'a> {
        self.identity.sign(request)
    }

    fn evm_address(&self) -> Option<[u8; 20]> {
        let point = self.evm.verifying_key().to_encoded_point(false);
        let mut address = [0; 20];
        address.copy_from_slice(&sha3::Keccak256::digest(&point.as_bytes()[1..])[12..]);
        Some(address)
    }

    fn sign_transaction<'a>(
        &'a self,
        call: &'a Eip1559Call,
    ) -> layerx_crypto::signer::TransactionSignFuture<'a> {
        Box::pin(async move {
            let (signature, recovery) = self
                .evm
                .sign_prehash_recoverable(&call.signing_hash())
                .map_err(|_| layerx_crypto::signer::SignError::KeyRejected)?;
            let mut bytes = [0; 65];
            bytes[..64].copy_from_slice(&signature.to_bytes());
            bytes[64] = 27 + recovery.to_byte();
            AgentSignature::new(bytes)
                .map_err(|_| layerx_crypto::signer::SignError::ReturnedSignatureInvalid)
        })
    }
}

fn encode_header(contract: Contract, provider_reference: &str) -> Result<Vec<u8>, KmsError> {
    let mut writer = WireWriter::new();
    writer.fixed(PROVIDER_MAGIC)?;
    writer.u16(contract.version)?;
    writer.u8(contract.operation)?;
    writer.bytes(provider_reference.as_bytes(), super::KEY_REFERENCE_LIMIT)?;
    Ok(writer.finish())
}

fn encode_key_request(
    contract: Contract,
    provider_reference: &str,
    binding: &PrincipalKeyBinding,
    reference: Option<&ProviderKeyReference>,
) -> Result<Vec<u8>, KmsError> {
    let mut writer = WireWriter::from_bytes(encode_header(contract, provider_reference)?);
    writer.fixed(&binding.digest)?;
    writer.u32(binding.network_id)?;
    writer.u8(binding.class.code())?;
    match reference {
        Some(reference) => writer.bytes(reference.as_bytes(), PROVIDER_REFERENCE_LIMIT)?,
        None => writer.bytes(&[], PROVIDER_REFERENCE_LIMIT)?,
    }
    Ok(writer.finish())
}

fn encode_sign_request(
    provider_reference: &str,
    binding: &PrincipalKeyBinding,
    reference: &ProviderKeyReference,
    request: ProviderSignRequest<'_>,
    validated: &ValidatedSignRequest,
) -> Result<Vec<u8>, CustodyError> {
    let mut writer = WireWriter::from_bytes(encode_header(SIGN, provider_reference)?);
    writer.fixed(&binding.digest)?;
    writer.u32(binding.network_id)?;
    writer.u8(binding.class.code())?;
    writer.bytes(reference.as_bytes(), PROVIDER_REFERENCE_LIMIT)?;
    writer.fixed(&validated.canonical_digest)?;
    writer.fixed(&request.from)?;
    writer.bytes(
        &layerx_human_kms::attestor::unsigned_evm_transaction(request.call),
        PROVIDER_FRAME_LIMIT,
    )?;
    writer.bytes(&validated.disclosure, PROVIDER_FRAME_LIMIT)?;
    let frame = writer.finish();
    if frame.len() > PROVIDER_FRAME_LIMIT {
        return Err(CustodyError::Kms(KmsError::InvalidConfiguration));
    }
    Ok(frame)
}

fn encode_disclosure(disclosure: &Disclosure) -> Result<Vec<u8>, CustodyError> {
    let fee_grant = disclosure
        .authority_grant
        .filter(|grant| grant.fee_budget.is_some());
    let mut writer = WireWriter::new();
    writer.u8(
        if disclosure.onboarding.is_some() || disclosure.native_operation.is_some() {
            4
        } else if disclosure.session_grant.is_some() {
            3
        } else if fee_grant.is_some() {
            2
        } else {
            1
        },
    )?;
    writer.u32(disclosure.activity_type.value())?;
    writer.bytes(&disclosure.actor, 255)?;
    writer.bytes(&disclosure.authority, 524_288)?;
    writer.u32(
        u32::try_from(disclosure.counterparties.len())
            .map_err(|_| CustodyError::Kms(KmsError::InvalidConfiguration))?,
    )?;
    for counterparty in &disclosure.counterparties {
        writer.u8(match counterparty.role {
            layerx_crypto::disclosure::CounterpartyRole::Payer => 1,
            layerx_crypto::disclosure::CounterpartyRole::Recipient => 2,
        })?;
        writer.fixed(&counterparty.account)?;
    }
    writer.u32(
        u32::try_from(disclosure.amounts.len())
            .map_err(|_| CustodyError::Kms(KmsError::InvalidConfiguration))?,
    )?;
    for amount in &disclosure.amounts {
        writer.u8(match amount.role {
            layerx_crypto::disclosure::AmountRole::Transfer => 1,
            layerx_crypto::disclosure::AmountRole::SpendingLimit => 2,
            layerx_crypto::disclosure::AmountRole::SupplyCap => 3,
            layerx_crypto::disclosure::AmountRole::PerDrawMaximum => 4,
            layerx_crypto::disclosure::AmountRole::GrantAllowance => 5,
        })?;
        writer.fixed(&amount.value.to_be_bytes())?;
    }
    writer.fixed(&disclosure.asset)?;
    writer.fixed(&disclosure.fee_limit.to_be_bytes())?;
    writer.fixed(&disclosure.expiry.not_before.to_be_bytes())?;
    writer.fixed(&disclosure.expiry.not_after.to_be_bytes())?;
    writer.fixed(&disclosure.expiry.payload_expires_at.to_be_bytes())?;
    writer.fixed(&disclosure.idempotency_key)?;
    match disclosure.evm_payout_binding {
        Some(binding) => {
            writer.u8(1)?;
            writer.fixed(&binding.did_id)?;
            writer.u32(binding.network_id)?;
            writer.fixed(&binding.payout_address)?;
            writer.fixed(&binding.ownership_signature_digest)?;
        }
        None => writer.u8(0)?,
    }
    if let Some(grant) = fee_grant {
        writer.bytes(
            &grant
                .encode()
                .map_err(|_| CustodyError::Kms(KmsError::InvalidConfiguration))?,
            1024,
        )?;
    }
    if let Some(session) = &disclosure.session_grant {
        writer.bytes(&session.grant.registration_payload, 1024)?;
        writer.fixed(&session.expiry_sequence.to_be_bytes())?;
        writer.fixed(&session.action_key)?;
        if let Some(replacement) = session.replacement {
            writer.u8(1)?;
            writer.fixed(&replacement.predecessor_grant_id)?;
            writer.fixed(&replacement.expected_charge_state)?;
        } else {
            writer.u8(0)?;
        }
    }
    if let Some(onboarding) = &disclosure.onboarding {
        writer.bytes(
            &onboarding
                .encode()
                .map_err(|_| CustodyError::Kms(KmsError::InvalidConfiguration))?,
            2048,
        )?;
    }
    if let Some(operation) = &disclosure.native_operation {
        writer.bytes(
            &operation
                .encode()
                .map_err(|_| CustodyError::Kms(KmsError::InvalidConfiguration))?,
            2048,
        )?;
    }
    Ok(writer.finish())
}

fn decode_response(contract: Contract, bytes: &[u8]) -> Result<Vec<u8>, KmsError> {
    let mut reader = WireReader::new(bytes);
    if reader.fixed(4)? != PROVIDER_MAGIC
        || reader.u16()? != contract.version
        || reader.u8()? != contract.operation
    {
        return Err(KmsError::InvalidResponse);
    }
    let status = reader.u8()?;
    if status == STATUS_OK {
        return Ok(reader.remaining().to_vec());
    }
    if !reader.remaining().is_empty() {
        return Err(KmsError::InvalidResponse);
    }
    match status {
        STATUS_REFUSED => Err(KmsError::Refused),
        STATUS_NOT_FOUND => Err(KmsError::KeyNotFound),
        STATUS_CONFLICT => Err(KmsError::Conflict),
        STATUS_UNAVAILABLE => Err(KmsError::Unavailable),
        STATUS_INTEGRITY => Err(KmsError::Integrity),
        STATUS_SELF_CUSTODIED => Err(KmsError::SelfCustodied),
        _ => Err(KmsError::InvalidResponse),
    }
}

fn decode_description(
    bytes: &[u8],
    binding: &PrincipalKeyBinding,
) -> Result<ProviderKeyDescription, KmsError> {
    let mut reader = WireReader::new(bytes);
    let reference = ProviderKeyReference::new(reader.bytes(PROVIDER_REFERENCE_LIMIT)?.to_vec())?;
    let public_key = reader
        .fixed(32)?
        .try_into()
        .map_err(|_| KmsError::InvalidResponse)?;
    let binding_digest = reader
        .fixed(32)?
        .try_into()
        .map_err(|_| KmsError::InvalidResponse)?;
    let rotation = RotationState::from_code(reader.u8()?)?;
    reader.finish()?;
    if !layerx_crypto::ct::eq_fixed(&binding_digest, &binding.digest) {
        return Err(KmsError::Integrity);
    }
    ProviderKeyDescription::new(reference, public_key, binding_digest, rotation)
}

fn map_transport(error: TransportError) -> KmsError {
    match error {
        TransportError::Deadline => KmsError::Timeout,
        TransportError::TlsConfiguration => KmsError::Authentication,
        TransportError::Frame(_) | TransportError::PeerShutdown | TransportError::Backpressure => {
            KmsError::InvalidResponse
        }
        TransportError::ConnectionFailure(_)
        | TransportError::ConnectionLimit
        | TransportError::StreamLimit => KmsError::Unavailable,
    }
}

struct WireWriter {
    bytes: Vec<u8>,
}

impl WireWriter {
    fn new() -> Self {
        Self { bytes: Vec::new() }
    }

    fn from_bytes(bytes: Vec<u8>) -> Self {
        Self { bytes }
    }

    fn u8(&mut self, value: u8) -> Result<(), KmsError> {
        self.extend(&[value])
    }

    fn u16(&mut self, value: u16) -> Result<(), KmsError> {
        self.extend(&value.to_be_bytes())
    }

    fn u32(&mut self, value: u32) -> Result<(), KmsError> {
        self.extend(&value.to_be_bytes())
    }

    fn fixed(&mut self, value: &[u8]) -> Result<(), KmsError> {
        self.extend(value)
    }

    fn bytes(&mut self, value: &[u8], maximum: usize) -> Result<(), KmsError> {
        if value.len() > maximum {
            return Err(KmsError::InvalidConfiguration);
        }
        let length = u32::try_from(value.len()).map_err(|_| KmsError::InvalidConfiguration)?;
        self.u32(length)?;
        self.extend(value)
    }

    fn extend(&mut self, value: &[u8]) -> Result<(), KmsError> {
        let next = self
            .bytes
            .len()
            .checked_add(value.len())
            .ok_or(KmsError::InvalidConfiguration)?;
        if next > PROVIDER_FRAME_LIMIT {
            return Err(KmsError::InvalidConfiguration);
        }
        self.bytes.extend_from_slice(value);
        Ok(())
    }

    fn finish(self) -> Vec<u8> {
        self.bytes
    }
}

struct WireReader<'a> {
    bytes: &'a [u8],
    offset: usize,
}

impl<'a> WireReader<'a> {
    const fn new(bytes: &'a [u8]) -> Self {
        Self { bytes, offset: 0 }
    }

    fn fixed(&mut self, length: usize) -> Result<&'a [u8], KmsError> {
        let end = self
            .offset
            .checked_add(length)
            .ok_or(KmsError::InvalidResponse)?;
        let value = self
            .bytes
            .get(self.offset..end)
            .ok_or(KmsError::InvalidResponse)?;
        self.offset = end;
        Ok(value)
    }

    fn u8(&mut self) -> Result<u8, KmsError> {
        Ok(self.fixed(1)?[0])
    }

    fn u16(&mut self) -> Result<u16, KmsError> {
        let bytes = self
            .fixed(2)?
            .try_into()
            .map_err(|_| KmsError::InvalidResponse)?;
        Ok(u16::from_be_bytes(bytes))
    }

    fn u32(&mut self) -> Result<u32, KmsError> {
        let bytes = self
            .fixed(4)?
            .try_into()
            .map_err(|_| KmsError::InvalidResponse)?;
        Ok(u32::from_be_bytes(bytes))
    }

    fn bytes(&mut self, maximum: usize) -> Result<&'a [u8], KmsError> {
        let length = usize::try_from(self.u32()?).map_err(|_| KmsError::InvalidResponse)?;
        if length > maximum {
            return Err(KmsError::InvalidResponse);
        }
        self.fixed(length)
    }

    fn remaining(&self) -> &'a [u8] {
        &self.bytes[self.offset..]
    }

    fn finish(self) -> Result<(), KmsError> {
        if self.offset == self.bytes.len() {
            Ok(())
        } else {
            Err(KmsError::InvalidResponse)
        }
    }
}

struct DeadlineSocket {
    tcp: TcpStream,
    deadline: Deadline,
    clock: Arc<dyn Clock>,
}

impl DeadlineSocket {
    fn bound(&mut self) -> std::io::Result<()> {
        let remaining = self
            .deadline
            .remaining(self.clock.as_ref())
            .map_err(|_| std::io::Error::other("provider clock unavailable"))?;
        if remaining.is_zero() {
            return Err(std::io::ErrorKind::TimedOut.into());
        }
        self.tcp.set_read_timeout(Some(remaining))?;
        self.tcp.set_write_timeout(Some(remaining))
    }
}
impl Read for DeadlineSocket {
    fn read(&mut self, bytes: &mut [u8]) -> std::io::Result<usize> {
        self.bound()?;
        self.tcp.read(bytes)
    }
}
impl Write for DeadlineSocket {
    fn write(&mut self, bytes: &[u8]) -> std::io::Result<usize> {
        self.bound()?;
        self.tcp.write(bytes)
    }
    fn flush(&mut self) -> std::io::Result<()> {
        self.bound()?;
        self.tcp.flush()
    }
}
