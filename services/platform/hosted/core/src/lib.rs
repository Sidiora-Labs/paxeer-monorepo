//! Canonical treasury SEND construction shared by the core boundary binary and
//! its real-node tests, together with the treasury signer boundary the binary
//! signs through.

mod custody;

use custody::TreasurySigner as DidSigner;
pub use custody::{did_for_public_key, SendError};
use layerx_crypto::payments::{ledger_payment_call, CallOptions};
use layerx_crypto::signer::{
    AgentSignature, SignError, SignFuture, Signer, SigningRequest, TransactionSignFuture,
};
use layerx_platform_internal::evm_sender::EvmSender;
use std::sync::Arc;

pub trait TreasurySigner: Send + Sync {
    fn public_key(&self) -> [u8; 32];
    /// # Errors
    /// Returns an error when the signer cannot authorize the digest.
    fn sign_digest(&self, digest: &[u8; 32]) -> Result<[u8; 64], String>;
    fn transaction_signer(&self) -> &dyn Signer;
}

#[derive(Clone)]
pub struct TreasuryIdentity<D> {
    did: D,
    evm: Result<Arc<EvmSender>, String>,
}

pub type SeedSigner = TreasuryIdentity<custody::SeedSigner>;
pub type SocketSigner = TreasuryIdentity<custody::SocketSigner>;

impl SeedSigner {
    #[must_use]
    pub fn new(seed: &[u8; 32]) -> Self {
        let evm = layerx_crypto::account_derivation::derive_from_seed(seed, 0)
            .map_err(|error| error.to_string())
            .and_then(|account| {
                let secret = account.evm_secret().ok_or("EVM key unavailable")?;
                let encoded = zeroize::Zeroizing::new(hex_encode(secret));
                EvmSender::from_hex_secret(&encoded)
                    .map(Arc::new)
                    .map_err(|error| error.to_string())
            });
        Self {
            did: custody::SeedSigner::new(seed),
            evm,
        }
    }
}

impl SocketSigner {
    /// # Errors
    /// Returns an error when the signer socket cannot be opened.
    pub fn connect(path: &std::path::Path) -> Result<Self, String> {
        Self::connect_with_timeout(path, custody::SIGNER_TIMEOUT)
    }

    /// # Errors
    /// Returns an error when the signer socket cannot be opened within the timeout.
    pub fn connect_with_timeout(
        path: &std::path::Path,
        timeout: std::time::Duration,
    ) -> Result<Self, String> {
        let did = custody::SocketSigner::connect_with_timeout(path, timeout)?;
        let evm = EvmSender::from_env("LAYERX_CORE_FUNDING_SENDER")
            .map(Arc::new)
            .map_err(|error| error.to_string());
        Ok(Self { did, evm })
    }

    /// # Errors
    /// Returns an error when the signer socket cannot be opened.
    pub fn connect_with_evm_sender(path: &std::path::Path, evm: EvmSender) -> Result<Self, String> {
        Ok(Self {
            did: custody::SocketSigner::connect(path)?,
            evm: Ok(Arc::new(evm)),
        })
    }

    #[must_use]
    pub fn path(&self) -> &std::path::Path {
        self.did.path()
    }
}

impl<D: DidSigner> std::fmt::Debug for TreasuryIdentity<D> {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("TreasuryIdentity")
            .field("public_key", &hex_encode(&self.did.public_key()))
            .finish_non_exhaustive()
    }
}

impl<D: DidSigner> TreasurySigner for TreasuryIdentity<D> {
    fn public_key(&self) -> [u8; 32] {
        self.did.public_key()
    }
    fn sign_digest(&self, digest: &[u8; 32]) -> Result<[u8; 64], String> {
        self.did.sign_digest(digest)
    }
    fn transaction_signer(&self) -> &dyn Signer {
        self
    }
}

impl<D: DidSigner> Signer for TreasuryIdentity<D> {
    fn public_key(&self) -> [u8; 32] {
        self.did.public_key()
    }
    fn sign<'a>(&'a self, _request: SigningRequest<'a>) -> SignFuture<'a> {
        Box::pin(std::future::ready(Err(SignError::KeystoreRefused)))
    }
    fn evm_address(&self) -> Option<[u8; 20]> {
        self.evm.as_ref().ok().map(|sender| sender.address())
    }
    fn sign_transaction<'a>(
        &'a self,
        call: &'a layerx_crypto::evm_transaction::Eip1559Call,
    ) -> TransactionSignFuture<'a> {
        Box::pin(async move {
            let sender = self
                .evm
                .as_ref()
                .map_err(|_| SignError::EvmKeyUnavailable)?;
            let signature = sender
                .sign_prehash(&call.signing_hash())
                .map_err(|_| SignError::KeyRejected)?;
            AgentSignature::new(signature).map_err(|_| SignError::KeyRejected)
        })
    }
}

fn complete<F: std::future::Future>(future: F) -> F::Output {
    struct WakeThread(std::thread::Thread);
    impl std::task::Wake for WakeThread {
        fn wake(self: Arc<Self>) {
            self.0.unpark();
        }
        fn wake_by_ref(self: &Arc<Self>) {
            self.0.unpark();
        }
    }
    let waker = std::task::Waker::from(Arc::new(WakeThread(std::thread::current())));
    let mut context = std::task::Context::from_waker(&waker);
    let mut future = std::pin::pin!(future);
    loop {
        match future.as_mut().poll(&mut context) {
            std::task::Poll::Ready(result) => return result,
            std::task::Poll::Pending => std::thread::park(),
        }
    }
}

use ed25519_dalek::SigningKey;
use layerx_intents::{compile, Intent, IntentKind, LxpSend};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, Did, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, NetworkId, ProtocolVersion, PublicKey, SendAuthorization,
    SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::Domain;
use sha2::{Digest as _, Sha256};

/// Asset module ordinal of the canonical SEND activity.
pub const SEND_ACTIVITY: u16 = 5;

/// Every input needed to build one owner-authorised asset SEND.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SendRequest {
    pub network_id: u32,
    pub source_did: String,
    pub destination_did: String,
    pub asset: [u8; 32],
    pub amount: u128,
    pub account_sequence: u64,
    pub idempotency_key: [u8; 32],
    pub not_before_ms: u64,
    pub expires_at_ms: u64,
    pub fee_limit: u128,
}

/// A signed EIP-1559 SEND ready for node JSON-RPC, with its identifying facts.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SignedSend {
    pub canonical: Vec<u8>,
    pub activity_id: [u8; 32],
    pub source_account: [u8; 32],
    pub destination_account: [u8; 32],
    pub signer_public_key: [u8; 32],
    pub idempotency_key: [u8; 32],
}

/// Returns the canonical account identifier of `agent:<did>:main`.
///
/// # Errors
///
/// Returns the account-name validation failure as text.
pub fn main_account(did: &str) -> Result<[u8; 32], String> {
    let account = AccountId::parse(&format!("agent:{did}:main"))
        .map_err(|error| format!("account name for {did} is invalid: {error:?}"))?;
    layerx_wire::hash::account_id_for_protocol(
        &account,
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    )
    .map_err(|error| format!("account id for {did} cannot be derived: {error:?}"))
}

/// Builds the asset module registry that declares SEND.
///
/// # Errors
///
/// Returns the registry construction failure as text.
pub fn asset_registry() -> Result<(ModuleRegistry, ActivityType), String> {
    let activity_type = ActivityType::new(ModuleId::Asset, SEND_ACTIVITY)
        .map_err(|error| format!("asset send activity is unavailable: {error:?}"))?;
    let registration = ModuleRegistration::new(ModuleId::Asset, &[activity_type])
        .map_err(|error| format!("asset module registration is invalid: {error:?}"))?;
    let registry = ModuleRegistry::new(&[registration])
        .map_err(|error| format!("asset module registry is invalid: {error:?}"))?;
    Ok((registry, activity_type))
}

/// Computes the send context commitment the core recomputes during execution.
#[must_use]
pub fn send_context_hash(
    source: &[u8; 32],
    destination: &[u8; 32],
    asset: &[u8; 32],
    amount: u128,
    idempotency: &[u8; 32],
) -> [u8; 32] {
    layerx_crypto::payments::send_context_hash(source, destination, asset, amount, idempotency)
}

/// Hashes bytes under one canonical wire domain tag.
#[must_use]
pub fn domain_hash(domain: Domain, bytes: &[u8]) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(domain.tag());
    digest.update(bytes);
    digest.finalize().into()
}

/// Builds, compiles with layerx-intents and signs one SEND from a seed held
/// in process.
///
/// # Errors
///
/// Returns every construction, compilation or encoding failure as text.
pub fn build_send(seed: &[u8; 32], request: &SendRequest) -> Result<SignedSend, String> {
    build_send_with_identity_sequence(seed, request.account_sequence, request)
}

/// Builds one SEND from a seed held in process whose actor sequence is
/// independent of its source-account sequence.
///
/// # Errors
///
/// Returns every construction, compilation or encoding failure as text.
pub fn build_send_with_identity_sequence(
    seed: &[u8; 32],
    identity_sequence: u64,
    request: &SendRequest,
) -> Result<SignedSend, String> {
    build_send_with_signer(&SeedSigner::new(seed), identity_sequence, request)
        .map_err(|error| error.to_string())
}

/// Builds one SEND whose authorization and transaction signatures come from `signer`, which may hold the
/// treasury identity in process or reach it over the treasury signer socket.
///
/// # Errors
///
/// Returns [`SendError::Signer`] when the signer refuses or cannot be reached
/// and [`SendError::Invalid`] for every construction, compilation or encoding
/// failure.
pub fn build_send_with_signer(
    signer: &dyn TreasurySigner,
    identity_sequence: u64,
    request: &SendRequest,
) -> Result<SignedSend, SendError> {
    validate_send_request(request)?;
    let from = AccountId::parse(&format!("agent:{}:main", request.source_did))
        .map_err(|error| SendError::Invalid(format!("source account is invalid: {error:?}")))?;
    let to =
        AccountId::parse(&format!("agent:{}:main", request.destination_did)).map_err(|error| {
            SendError::Invalid(format!("destination account is invalid: {error:?}"))
        })?;
    build_send_for_accounts(signer, identity_sequence, request, from, to)
}

fn build_send_for_accounts(
    signer: &dyn TreasurySigner,
    identity_sequence: u64,
    request: &SendRequest,
    from: AccountId,
    to: AccountId,
) -> Result<SignedSend, SendError> {
    validate_send_request(request)?;
    let public_key = signer.public_key();
    let source = layerx_wire::hash::account_id_for_protocol(
        &from,
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    )
    .map_err(|error| SendError::Invalid(format!("source account is invalid: {error:?}")))?;
    let destination = layerx_wire::hash::account_id_for_protocol(
        &to,
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    )
    .map_err(|error| SendError::Invalid(format!("destination account is invalid: {error:?}")))?;
    let context = send_context_hash(
        &source,
        &destination,
        &request.asset,
        request.amount,
        &request.idempotency_key,
    );
    let authorization = send_authorization(signer, &source, &destination, request, &context)?;
    let intent = LxpSend::new(
        from,
        to,
        AssetId::new(request.asset),
        Amount::from_u128(request.amount),
        Sequence::from_u64(request.account_sequence),
        IdempotencyKey::new(request.idempotency_key),
        TimestampSeconds::from_u64(request.expires_at_ms),
        ContextHash::new(context),
        SendAuthorization::new(
            SendAuthorizationKind::Owner,
            PublicKey::new(public_key),
            AuthorizationSignature::new(authorization),
        ),
        NetworkId::new(request.network_id)
            .map_err(|error| SendError::Invalid(format!("network id is invalid: {error:?}")))?,
        ProtocolVersion::new(layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION).map_err(
            |error| SendError::Invalid(format!("protocol version is invalid: {error:?}")),
        )?,
    )
    .map_err(|error| SendError::Invalid(format!("send intent is invalid: {error:?}")))?;
    let (registry, activity_type) = asset_registry().map_err(SendError::Invalid)?;
    let compiled = compile(&Intent::v1(IntentKind::LxpSend(intent)), &registry)
        .map_err(|error| SendError::Invalid(format!("send intent does not compile: {error:?}")))?;
    if compiled.activity_type() != activity_type {
        return Err(SendError::Invalid(
            "compiled intent is not an asset send".into(),
        ));
    }
    let transaction_signer = signer.transaction_signer();
    let sender = transaction_signer
        .evm_address()
        .ok_or_else(|| SendError::Signer("treasury EVM key is unavailable".to_owned()))?;
    let disclosed = ledger_payment_call(
        compiled.payload().as_bytes(),
        &CallOptions {
            from: sender,
            chain_id: 125,
            nonce: identity_sequence,
            max_priority_fee_per_gas: 0,
            max_fee_per_gas: 1,
            gas_limit: u64::try_from(request.fee_limit).map_err(|_| {
                SendError::Invalid("fee limit exceeds the gas budget range".to_owned())
            })?,
        },
    )
    .map_err(|error| SendError::Invalid(format!("send call is invalid: {error:?}")))?;
    // Validate the disclosure and recover the secp256k1 sender before submission.
    let signature = complete(layerx_crypto::signer::sign_disclosed_call(
        transaction_signer,
        &disclosed.call,
        &disclosed.disclosure,
        sender,
    ))
    .map_err(|error| SendError::Signer(error.to_string()))?;
    let canonical = layerx_crypto::evm_transaction::assemble(&disclosed.call, &signature);
    let recovered = layerx_crypto::evm_transaction::recover_sender(&disclosed.call, &signature)
        .map_err(|error| SendError::Signer(error.to_string()))?;
    if recovered != sender {
        return Err(SendError::Signer(
            "treasury transaction sender differs".to_owned(),
        ));
    }
    let activity_id = layerx_crypto::evm_transaction::transaction_hash(&canonical);
    Ok(SignedSend {
        canonical,
        activity_id,
        source_account: source,
        destination_account: destination,
        signer_public_key: public_key,
        idempotency_key: request.idempotency_key,
    })
}

fn validate_send_request(request: &SendRequest) -> Result<(), SendError> {
    if request.amount == 0 {
        return Err(SendError::Invalid(
            "amount must be greater than zero".into(),
        ));
    }
    if request.expires_at_ms <= request.not_before_ms {
        return Err(SendError::Invalid(
            "expiry must follow the validity start".into(),
        ));
    }
    Ok(())
}

fn send_authorization(
    signer: &dyn TreasurySigner,
    source: &[u8; 32],
    destination: &[u8; 32],
    request: &SendRequest,
    context: &[u8; 32],
) -> Result<[u8; 64], SendError> {
    let mut authorization = Encoder::new(512);
    authorization
        .u16(0x5301)
        .and_then(|()| authorization.fixed(source))
        .and_then(|()| authorization.fixed(destination))
        .and_then(|()| authorization.fixed(&request.asset))
        .and_then(|()| authorization.u128(request.amount))
        .and_then(|()| authorization.u64(request.account_sequence))
        .and_then(|()| authorization.fixed(&request.idempotency_key))
        .and_then(|()| authorization.u64(request.expires_at_ms))
        .and_then(|()| authorization.fixed(context))
        .and_then(|()| authorization.u8(0))
        .and_then(|()| authorization.u8(SendAuthorizationKind::Owner as u8))
        .and_then(|()| authorization.fixed(source))
        .and_then(|()| authorization.fixed(context))
        .and_then(|()| authorization.u32(request.network_id))
        .and_then(|()| authorization.u16(layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION))
        .map_err(|error| {
            SendError::Invalid(format!("send authorization is too large: {error:?}"))
        })?;
    let digest = domain_hash(Domain::SignaturePreimage, &authorization.finish());
    signer.sign_digest(&digest).map_err(SendError::Signer)
}

/// Encodes bytes as lowercase hexadecimal.
#[must_use]
pub fn hex_encode(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut text = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        text.push(char::from(DIGITS[usize::from(byte >> 4)]));
        text.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    text
}

/// Decodes hexadecimal text of any case into bytes.
///
/// # Errors
///
/// Returns a description when the text is odd-length or not hexadecimal.
pub fn hex_decode(text: &str) -> Result<Vec<u8>, String> {
    if !text.len().is_multiple_of(2) {
        return Err("hex text has odd length".into());
    }
    let mut bytes = Vec::with_capacity(text.len() / 2);
    let raw = text.as_bytes();
    for pair in raw.chunks(2) {
        let high = hex_nibble(pair[0])?;
        let low = hex_nibble(pair[1])?;
        bytes.push((high << 4) | low);
    }
    Ok(bytes)
}

/// Decodes exactly `N` bytes of hexadecimal text.
///
/// # Errors
///
/// Returns a description naming the expected width on mismatch.
pub fn fixed_hex<const N: usize>(name: &str, text: &str) -> Result<[u8; N], String> {
    let bytes = hex_decode(text).map_err(|error| format!("{name}: {error}"))?;
    <[u8; N]>::try_from(bytes).map_err(|_| format!("{name} must be {N} bytes of hex"))
}

fn hex_nibble(byte: u8) -> Result<u8, String> {
    match byte {
        b'0'..=b'9' => Ok(byte - b'0'),
        b'a'..=b'f' => Ok(byte - b'a' + 10),
        b'A'..=b'F' => Ok(byte - b'A' + 10),
        _ => Err("hex text contains a non-hexadecimal character".into()),
    }
}

/// Derives the beta treasury DID `did:layerx:<public key hex>` from a seed.
#[must_use]
pub fn treasury_did(seed: &[u8; 32]) -> String {
    did_for_public_key(&SigningKey::from_bytes(seed).verifying_key().to_bytes())
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RegisteredCustodyAsset {
    asset: [u8; 32],
    profile: Vec<u8>,
    trusted_height: u64,
    pub denom: String,
    pub pointer: [u8; 20],
    pub total_supply: [u8; 32],
    network_id: u32,
    state_root: [u8; 32],
}

impl RegisteredCustodyAsset {
    #[must_use]
    pub const fn asset(&self) -> [u8; 32] {
        self.asset
    }
    #[must_use]
    pub fn profile(&self) -> &[u8] {
        &self.profile
    }
    #[must_use]
    pub fn profile_hash(&self) -> [u8; 32] {
        Sha256::digest(&self.profile).into()
    }
    #[must_use]
    pub fn trusted_height(&self) -> u64 {
        self.trusted_height
    }
    /// # Errors
    /// Returns an error when the derived asset account name is invalid.
    pub fn account_name(&self, did: &str) -> Result<String, String> {
        let name = format!("agent:{did}:asset:{}", hex_encode(&self.asset));
        AccountId::parse(&name).map_err(|error| format!("asset account is invalid: {error:?}"))?;
        Ok(name)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthenticatedCustodyAsset {
    registered: RegisteredCustodyAsset,
    source_did: String,
    source_account: [u8; 32],
    account_sequence: u64,
    trusted_height: u64,
}

impl AuthenticatedCustodyAsset {
    #[must_use]
    pub const fn asset(&self) -> [u8; 32] {
        self.registered.asset()
    }
    #[must_use]
    pub fn profile(&self) -> &[u8] {
        self.registered.profile()
    }
    #[must_use]
    pub const fn trusted_height(&self) -> u64 {
        self.trusted_height
    }
    #[must_use]
    pub const fn account_sequence(&self) -> u64 {
        self.account_sequence
    }
    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.registered.state_root
    }
    /// # Errors
    /// Returns an error when the derived asset account name is invalid.
    pub fn account_name(&self, did: &str) -> Result<String, String> {
        self.registered.account_name(did)
    }
}

const BANK_PRECOMPILE: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x01,
];
const POINTERVIEW_PRECOMPILE: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x0a,
];

fn custody_selector(signature: &str) -> [u8; 4] {
    let hash = layerx_crypto::evm_transaction::transaction_hash(signature.as_bytes());
    [hash[0], hash[1], hash[2], hash[3]]
}

fn abi_word(bytes: &[u8], index: usize) -> Result<[u8; 32], String> {
    let offset = index.checked_mul(32).ok_or("ABI offset overflow")?;
    bytes
        .get(offset..offset.checked_add(32).ok_or("ABI offset overflow")?)
        .ok_or("truncated custody ABI word")?
        .try_into()
        .map_err(|_| "invalid custody ABI word".into())
}

fn abi_u64(bytes: &[u8], index: usize) -> Result<u64, String> {
    let word = abi_word(bytes, index)?;
    if word[..24] != [0; 24] {
        return Err("custody ABI integer exceeds u64".into());
    }
    Ok(u64::from_be_bytes(
        word[24..].try_into().map_err(|_| "invalid ABI integer")?,
    ))
}

fn abi_bool(bytes: &[u8], index: usize) -> Result<bool, String> {
    match abi_u64(bytes, index)? {
        0 => Ok(false),
        1 => Ok(true),
        _ => Err("invalid custody ABI boolean".into()),
    }
}

#[cfg(test)]
#[allow(dead_code)]
struct LegacyCustodyAsset {
    asset: [u8; 32],
    network_id: u32,
    denom: String,
    pointer: [u8; 20],
    custody_cap: [u8; 32],
    custodied: [u8; 32],
    profile: Vec<u8>,
    state_root: [u8; 32],
    trusted_height: u64,
}
#[cfg(test)]
impl LegacyCustodyAsset {
    fn asset(&self) -> [u8; 32] {
        self.asset
    }
    fn profile(&self) -> &[u8] {
        &self.profile
    }
}

#[cfg(test)]
fn decode_custody_asset(
    bytes: Vec<u8>,
    asset: [u8; 32],
    network_id: u32,
) -> Result<LegacyCustodyAsset, String> {
    if abi_u64(&bytes, 0)? != 32 {
        return Err("invalid custody tuple offset".into());
    }
    let tuple = bytes.get(32..).ok_or("missing custody tuple")?;
    let offset = usize::try_from(abi_u64(tuple, 1)?).map_err(|_| "denom offset overflow")?;
    if offset < 320 || offset % 32 != 0 {
        return Err("invalid custody denom offset".into());
    }
    let length =
        usize::try_from(abi_u64(tuple, offset / 32)?).map_err(|_| "denom length overflow")?;
    let start = offset.checked_add(32).ok_or("denom offset overflow")?;
    let end = start.checked_add(length).ok_or("denom length overflow")?;
    let padded_end = end.checked_add(31).ok_or("denom length overflow")? / 32 * 32;
    let denom = std::str::from_utf8(tuple.get(start..end).ok_or("truncated custody denom")?)
        .map_err(|_| "invalid custody denom")?
        .to_owned();
    if tuple
        .get(end..padded_end)
        .ok_or("truncated denom padding")?
        .iter()
        .any(|byte| *byte != 0)
    {
        return Err("invalid custody denom padding".into());
    }
    let pointer = abi_word(tuple, 2)?;
    if abi_word(tuple, 0)? != asset
        || !abi_bool(tuple, 3)?
        || abi_bool(tuple, 4)?
        || denom.is_empty()
        || pointer[..12] != [0; 12]
    {
        return Err("asset is not enabled in the custody registry".into());
    }
    let custody_cap = abi_word(tuple, 6)?;
    let custodied = abi_word(tuple, 7)?;
    abi_word(tuple, 9)?;
    Ok(LegacyCustodyAsset {
        asset,
        network_id,
        denom,
        pointer: pointer[12..]
            .try_into()
            .map_err(|_| "invalid custody pointer")?,
        custody_cap,
        custodied,
        profile: bytes,
        state_root: [0; 32],
        trusted_height: 0,
    })
}

/// The native committed block replaces the retired checkpoint authority.
///
/// # Errors
/// Returns an error when the committed block or its state root is unavailable or changes.
pub fn custody_latest_finalized(
    sequencer: &layerx_client::sequencer::SequencerEndpoint,
    committed_height: u64,
) -> Result<u64, String> {
    native_committed_state(sequencer, committed_height)?;
    Ok(committed_height)
}

/// # Errors
/// Returns an error when the committed block or its state root is unavailable or changes.
pub fn native_committed_state(
    sequencer: &layerx_client::sequencer::SequencerEndpoint,
    height: u64,
) -> Result<(layerx_client::sequencer::SequencerBlock, [u8; 32]), String> {
    let block = sequencer
        .block_by_number(height)
        .map_err(|error| format!("native committed block: {error:?}"))?
        .ok_or("native committed block unavailable")?;
    let root = sequencer
        .state_root(height)
        .map_err(|error| format!("native app hash: {error:?}"))?;
    if height == 0 || block.number != height || block.hash == [0; 32] || root == [0; 32] {
        return Err("invalid native committed block".into());
    }
    let confirmation = sequencer
        .block_by_number(height)
        .map_err(|error| format!("native block confirmation: {error:?}"))?
        .ok_or("native committed block unavailable")?;
    let confirmed_root = sequencer
        .state_root(height)
        .map_err(|error| format!("native app hash confirmation: {error:?}"))?;
    if confirmation != block || confirmed_root != root {
        return Err("native committed block changed".into());
    }
    Ok((block, root))
}

/// # Errors
/// Returns an error when the requested block is outside the committed range or cannot be read.
pub fn custody_checkpoint_state_root(
    sequencer: &layerx_client::sequencer::SequencerEndpoint,
    committed_height: u64,
    batch: u64,
) -> Result<Option<[u8; 32]>, String> {
    if batch == 0 || batch > committed_height {
        return Err("native block is outside the committed range".into());
    }
    native_committed_state(sequencer, batch).map(|(_, root)| Some(root))
}

fn native_string_call(signature: &str, denom: &str) -> Result<Vec<u8>, String> {
    if denom.is_empty()
        || denom.len() > 256
        || !denom.is_ascii()
        || denom
            .bytes()
            .any(|byte| byte.is_ascii_control() || byte == b' ')
    {
        return Err("invalid native denomination".into());
    }
    let mut data = custody_selector(signature).to_vec();
    data.extend_from_slice(&[0; 24]);
    data.extend_from_slice(&32_u64.to_be_bytes());
    data.extend_from_slice(&[0; 24]);
    data.extend_from_slice(&(denom.len() as u64).to_be_bytes());
    data.extend_from_slice(denom.as_bytes());
    data.resize(4 + 64 + denom.len().div_ceil(32) * 32, 0);
    Ok(data)
}

fn native_abi_string(bytes: &[u8]) -> Result<String, String> {
    if abi_u64(bytes, 0)? != 32 {
        return Err("invalid native string offset".into());
    }
    let length = usize::try_from(abi_u64(bytes, 1)?).map_err(|_| "native string overflow")?;
    if length == 0 || length > 256 || bytes.len() != 64 + length.div_ceil(32) * 32 {
        return Err("invalid native metadata length".into());
    }
    if bytes[64 + length..].iter().any(|byte| *byte != 0) {
        return Err("invalid native metadata padding".into());
    }
    std::str::from_utf8(&bytes[64..64 + length])
        .map(str::to_owned)
        .map_err(|_| "invalid native metadata text".into())
}

/// # Errors
/// Returns an error when the native asset identity, metadata, or committed snapshot is invalid.
pub fn read_native_asset_profile(
    sequencer: &layerx_client::sequencer::SequencerEndpoint,
    network_id: u32,
    committed_height: u64,
    asset: [u8; 32],
    denom: &str,
) -> Result<RegisteredCustodyAsset, String> {
    if network_id == 0 || asset == [0; 32] {
        return Err("invalid native asset identity".into());
    }
    let (block, state_root) = native_committed_state(sequencer, committed_height)?;
    let read = |signature, address| {
        let data = native_string_call(signature, denom)?;
        sequencer
            .call_at(committed_height, &address, &data)
            .map_err(|error| format!("native asset read: {error:?}"))
    };
    let name = native_abi_string(&read("name(string)", BANK_PRECOMPILE)?)?;
    let symbol = native_abi_string(&read("symbol(string)", BANK_PRECOMPILE)?)?;
    let decimals_bytes = read("decimals(string)", BANK_PRECOMPILE)?;
    if decimals_bytes.len() != 32 || abi_u64(&decimals_bytes, 0)? > 255 {
        return Err("invalid native decimals".into());
    }
    let decimals = abi_u64(&decimals_bytes, 0)?;
    let supply = read("supply(string)", BANK_PRECOMPILE)?;
    if supply.len() != 32 {
        return Err("invalid native supply".into());
    }
    let total_supply = abi_word(&supply, 0)?;
    let pointer_bytes = read("getNativePointer(string)", POINTERVIEW_PRECOMPILE)?;
    let pointer_word = abi_word(&pointer_bytes, 0)?;
    let version = abi_u64(&pointer_bytes, 1)?;
    if pointer_bytes.len() != 96
        || pointer_word[..12] != [0; 12]
        || pointer_word[12..] == [0; 20]
        || version > u64::from(u16::MAX)
        || !abi_bool(&pointer_bytes, 2)?
    {
        return Err("native asset pointer unavailable".into());
    }
    let pointer: [u8; 20] = pointer_word[12..]
        .try_into()
        .map_err(|_| "invalid native pointer")?;
    let (confirmed, confirmed_root) = native_committed_state(sequencer, committed_height)?;
    if confirmed != block || confirmed_root != state_root {
        return Err("native asset snapshot changed".into());
    }
    let profile = serde_json::to_vec(&serde_json::json!({
        "version": 2, "asset_id": hex_encode(&asset), "denom": denom,
        "name": name, "symbol": symbol, "decimals": decimals,
        "pointer": hex_encode(&pointer), "pointer_version": version,
        "total_supply": hex_encode(&total_supply), "height": committed_height,
        "block_hash": hex_encode(&block.hash), "app_hash": hex_encode(&state_root),
        "units": "native_bank_base_units"
    }))
    .map_err(|error| error.to_string())?;
    Ok(RegisteredCustodyAsset {
        asset,
        profile,
        trusted_height: committed_height,
        denom: denom.to_owned(),
        pointer,
        total_supply,
        network_id,
        state_root,
    })
}

/// # Errors
/// Returns an error when the asset denomination is unbound or its profile cannot be read.
pub fn read_custody_profile(
    sequencer: &layerx_client::sequencer::SequencerEndpoint,
    network_id: u32,
    committed_height: u64,
    asset: [u8; 32],
) -> Result<RegisteredCustodyAsset, String> {
    // Asset IDs do not encode bank denominations. Require the owning domain's
    // explicit binding rather than inventing a mirror or a custody profile.
    let bindings: std::collections::BTreeMap<String, String> = serde_json::from_str(
        &std::env::var("LAYERX_CORE_NATIVE_ASSET_DENOMS")
            .map_err(|_| "native asset denomination bindings unavailable")?,
    )
    .map_err(|_| "invalid native asset denomination bindings")?;
    let denom = bindings
        .get(&hex_encode(&asset))
        .ok_or("native asset denomination unbound")?;
    read_native_asset_profile(sequencer, network_id, committed_height, asset, denom)
}

/// # Errors
/// Returns an error when the source DID is invalid or has no bound EVM address.
pub fn custody_source_address(
    network: &layerx_client::px::PxEndpoint,
    source_did: &str,
) -> Result<[u8; 20], String> {
    Did::new(source_did.as_bytes()).map_err(|error| format!("custody DID: {error:?}"))?;
    layerx_client::rpc::history::did_evm_address(network, source_did)
        .map_err(|error| format!("custody DID accounts: {error:?}"))?
        .ok_or_else(|| "custody DID has no bound evm_address".to_owned())
}

/// # Errors
/// Returns an error when the asset profile or source account cannot be authenticated.
pub fn read_custody_asset(
    sequencer: &layerx_client::sequencer::SequencerEndpoint,
    network: &layerx_client::px::PxEndpoint,
    network_id: u32,
    committed_height: u64,
    asset: [u8; 32],
    source_did: &str,
) -> Result<AuthenticatedCustodyAsset, String> {
    let registered = read_custody_profile(sequencer, network_id, committed_height, asset)?;
    let address = custody_source_address(network, source_did)?;
    let account_sequence = sequencer
        .pending_nonce(&address)
        .map_err(|error| format!("custody source nonce: {error:?}"))?;
    let name = registered.account_name(source_did)?;
    let account =
        AccountId::parse(&name).map_err(|error| format!("source asset account: {error:?}"))?;
    let source_account = layerx_wire::hash::account_id_for_protocol(&account, 3)
        .map_err(|error| format!("source asset account id: {error:?}"))?;
    let trusted_height = registered.trusted_height();
    Ok(AuthenticatedCustodyAsset {
        registered,
        source_did: source_did.into(),
        source_account,
        account_sequence,
        trusted_height,
    })
}

/// # Errors
/// Returns an error when the signer or request cannot produce a valid asset send.
pub fn build_asset_send_with_identity_sequence(
    seed: &[u8; 32],
    identity_sequence: u64,
    request: &SendRequest,
    asset: &AuthenticatedCustodyAsset,
) -> Result<SignedSend, String> {
    build_asset_send_with_signer(&SeedSigner::new(seed), identity_sequence, request, asset)
        .map_err(|error| error.to_string())
}

/// # Errors
/// Returns an error when the request differs from the authenticated asset or signer.
pub fn build_asset_send_with_signer(
    signer: &dyn TreasurySigner,
    identity_sequence: u64,
    request: &SendRequest,
    asset: &AuthenticatedCustodyAsset,
) -> Result<SignedSend, SendError> {
    if request.asset != asset.asset()
        || request.network_id != asset.registered.network_id
        || request.source_did != asset.source_did
        || request.account_sequence != asset.account_sequence
        || did_for_public_key(&signer.public_key()) != request.source_did
    {
        return Err(SendError::Invalid(
            "asset Send differs from authenticated registry, source or owner".into(),
        ));
    }
    let from = AccountId::parse(
        &asset
            .account_name(&request.source_did)
            .map_err(SendError::Invalid)?,
    )
    .map_err(|error| SendError::Invalid(format!("source asset account: {error:?}")))?;
    let to = AccountId::parse(
        &asset
            .account_name(&request.destination_did)
            .map_err(SendError::Invalid)?,
    )
    .map_err(|error| SendError::Invalid(format!("destination asset account: {error:?}")))?;
    let signed_send = build_send_for_accounts(signer, identity_sequence, request, from, to)?;
    if signed_send.source_account != asset.source_account {
        return Err(SendError::Invalid(
            "source asset account substitution".into(),
        ));
    }
    Ok(signed_send)
}

#[cfg(test)]
mod custody_rpc_tests {
    use super::{custody_selector, decode_custody_asset};

    #[test]
    fn custody_precompile_selectors_and_dynamic_asset_tuple() {
        assert_eq!(
            custody_selector("getAsset(bytes32)"),
            [0x2c, 0xc3, 0xce, 0x80]
        );
        assert_eq!(
            custody_selector("latestFinalized()"),
            [0x6c, 0xdd, 0x45, 0xae]
        );
        assert_eq!(
            custody_selector("checkpoint(uint64)"),
            [0x2d, 0x58, 0x8b, 0x18]
        );
        let asset = [0x41; 32];
        let mut bytes = vec![0; 13 * 32];
        bytes[31] = 32;
        bytes[32..64].copy_from_slice(&asset);
        bytes[88..96].copy_from_slice(&320_u64.to_be_bytes());
        bytes[108..128].copy_from_slice(&[0x52; 20]);
        bytes[159] = 1;
        bytes[224..256].copy_from_slice(&[0xfe; 32]);
        bytes[287] = 7;
        bytes[383] = 4;
        bytes[384..388].copy_from_slice(b"upax");
        let decoded = decode_custody_asset(bytes.clone(), asset, 125)
            .unwrap_or_else(|error| panic!("getAsset tuple: {error}"));
        assert_eq!(decoded.asset(), asset);
        assert_eq!(decoded.denom, "upax");
        assert_eq!(decoded.pointer, [0x52; 20]);
        assert_eq!(decoded.custody_cap, [0xfe; 32]);
        assert_eq!(decoded.custodied[31], 7);
        assert_eq!(decoded.profile(), bytes);
        assert!(decode_custody_asset(bytes.clone(), [0x42; 32], 125).is_err());
        let mut paused = bytes.clone();
        paused[191] = 1;
        assert!(decode_custody_asset(paused, asset, 125).is_err());
        let mut disabled = bytes.clone();
        disabled[159] = 0;
        assert!(decode_custody_asset(disabled, asset, 125).is_err());
        let mut empty = bytes.clone();
        empty[383] = 0;
        empty[384..388].fill(0);
        assert!(decode_custody_asset(empty, asset, 125).is_err());
        assert!(decode_custody_asset(bytes[..387].to_vec(), asset, 125).is_err());
        bytes[95] = 1;
        assert!(decode_custody_asset(bytes, asset, 125).is_err());
    }
}
