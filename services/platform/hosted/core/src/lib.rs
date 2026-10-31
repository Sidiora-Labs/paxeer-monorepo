//! Canonical treasury SEND construction shared by the core boundary binary and
//! its real-node tests, together with the treasury signer boundary the binary
//! signs through.

mod custody;

pub use custody::{did_for_public_key, SeedSigner, SendError, SocketSigner, TreasurySigner};

use ed25519_dalek::SigningKey;
use layerx_crypto::SignatureMessage;
use layerx_intents::{compile, Intent, IntentKind, LxpSend};
use layerx_types::account::AccountId;
use layerx_types::activity::{Authority, EnvelopeBuilder, Signature, TimestampBound};
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

/// A fully signed SEND ready for the LNI together with its identifying facts.
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

/// Builds one SEND whose two signatures come from `signer`, which may hold the
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
    let actor = Did::new(request.source_did.as_bytes())
        .map_err(|error| SendError::Invalid(format!("source DID is invalid: {error:?}")))?;
    let authority = Authority::owner(&public_key)
        .map_err(|error| SendError::Invalid(format!("owner authority is invalid: {error:?}")))?;
    let timestamp = TimestampBound::new(request.not_before_ms, request.expires_at_ms)
        .map_err(|error| SendError::Invalid(format!("timestamp bound is invalid: {error:?}")))?;
    let mut builder = EnvelopeBuilder::new();
    builder
        .protocol_version(layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION)
        .and_then(|value| value.network_id(request.network_id))
        .and_then(|value| value.activity_type(activity_type))
        .and_then(|value| value.actor_did(actor))
        .and_then(|value| value.authority(authority))
        .and_then(|value| value.account_sequence(identity_sequence))
        .and_then(|value| value.timestamp_bound(timestamp))
        .and_then(|value| value.idempotency_key(IdempotencyKey::new(request.idempotency_key)))
        .and_then(|value| value.fee_limit(Amount::from_u128(request.fee_limit)))
        .and_then(|value| value.payload_hash(compiled.payload_hash()))
        .and_then(|value| value.payload(compiled.payload().clone()))
        .map_err(|error| SendError::Invalid(format!("send envelope is invalid: {error:?}")))?;
    let unsigned = builder
        .build()
        .map_err(|error| SendError::Invalid(format!("send envelope is incomplete: {error:?}")))?;
    let unsigned_bytes =
        layerx_wire::activity::encode_unsigned_envelope(&unsigned).map_err(|error| {
            SendError::Invalid(format!("send signing bytes are invalid: {error:?}"))
        })?;
    let digest = domain_hash(Domain::SignaturePreimage, &unsigned_bytes);
    let signature = disclosed_signature(signer, &unsigned_bytes, request.network_id)?;
    layerx_crypto::ed25519::verify_digest(&public_key, &signature, &digest).map_err(|error| {
        SendError::Invalid(format!("send signature does not verify: {error:?}"))
    })?;
    let envelope = unsigned
        .attach_signature(Signature::new(&signature).map_err(|error| {
            SendError::Invalid(format!("send signature is invalid: {error:?}"))
        })?);
    let canonical = layerx_wire::activity::encode_signed_envelope(&envelope)
        .map_err(|error| SendError::Invalid(format!("signed send is invalid: {error:?}")))?;
    let decoded = layerx_wire::activity::decode_signed(&canonical, &registry)
        .map_err(|error| SendError::Invalid(format!("signed send does not decode: {error:?}")))?;
    let activity_id = layerx_wire::hash::activity_id(&decoded)
        .map_err(|error| SendError::Invalid(format!("send activity id is invalid: {error:?}")))?;
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

fn disclosed_signature(
    signer: &dyn TreasurySigner,
    canonical: &[u8],
    network_id: u32,
) -> Result<[u8; 64], SendError> {
    let message = SignatureMessage::new(
        Domain::SignaturePreimage,
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
        network_id,
        canonical,
    )
    .map_err(|error| SendError::Invalid(format!("send signing scope is invalid: {error:?}")))?;
    let signature = signer
        .sign_digest(&message.digest())
        .map_err(SendError::Signer)?;
    layerx_crypto::ed25519::verify(&signer.public_key(), &signature, message).map_err(|error| {
        SendError::Signer(format!("treasury signature does not verify: {error:?}"))
    })?;
    Ok(signature)
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
    pub custody_cap: [u8; 32],
    pub custodied: [u8; 32],
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
    pub fn account_name(&self, did: &str) -> Result<String, String> {
        self.registered.account_name(did)
    }
}

const CUSTODY_PRECOMPILE: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x13,
];
const ANCHOR_PRECOMPILE: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x14,
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

fn decode_custody_asset(
    bytes: Vec<u8>,
    asset: [u8; 32],
    network_id: u32,
) -> Result<RegisteredCustodyAsset, String> {
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
    Ok(RegisteredCustodyAsset {
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

pub fn custody_latest_finalized(
    sequencer: &layerx_client::sequencer::SequencerEndpoint,
    committed_height: u64,
) -> Result<u64, String> {
    let bytes = sequencer
        .call_at(
            committed_height,
            &ANCHOR_PRECOMPILE,
            &custody_selector("latestFinalized()"),
        )
        .map_err(|error| format!("custody latest finalized: {error:?}"))?;
    if bytes.len() != 64 || !abi_bool(&bytes, 1)? {
        return Err("custody has no finalized checkpoint".into());
    }
    abi_u64(&bytes, 0)
}

fn custody_checkpoint(
    sequencer: &layerx_client::sequencer::SequencerEndpoint,
    committed_height: u64,
    batch: u64,
) -> Result<Vec<u8>, String> {
    let mut data = custody_selector("checkpoint(uint64)").to_vec();
    data.extend_from_slice(&[0; 24]);
    data.extend_from_slice(&batch.to_be_bytes());
    let bytes = sequencer
        .call_at(committed_height, &ANCHOR_PRECOMPILE, &data)
        .map_err(|error| format!("custody checkpoint: {error:?}"))?;
    if bytes.len() != 18 * 32 {
        return Err("invalid anchor checkpoint tuple".into());
    }
    Ok(bytes)
}

pub fn custody_checkpoint_state_root(
    sequencer: &layerx_client::sequencer::SequencerEndpoint,
    committed_height: u64,
    batch: u64,
) -> Result<Option<[u8; 32]>, String> {
    let bytes = custody_checkpoint(sequencer, committed_height, batch)?;
    if abi_u64(&bytes, 12)? == 0 {
        return Ok(None);
    }
    if abi_u64(&bytes, 0)? != batch {
        return Err("anchor checkpoint batch mismatch".into());
    }
    Ok(Some(abi_word(&bytes, 7)?))
}

pub fn read_custody_profile(
    sequencer: &layerx_client::sequencer::SequencerEndpoint,
    network_id: u32,
    committed_height: u64,
    asset: [u8; 32],
) -> Result<RegisteredCustodyAsset, String> {
    let mut data = custody_selector("getAsset(bytes32)").to_vec();
    data.extend_from_slice(&asset);
    let bytes = sequencer
        .call_at(committed_height, &CUSTODY_PRECOMPILE, &data)
        .map_err(|error| format!("custody registry read: {error:?}"))?;
    let mut registered = decode_custody_asset(bytes, asset, network_id)?;
    let batch = custody_latest_finalized(sequencer, committed_height)?;
    let checkpoint = custody_checkpoint(sequencer, committed_height, batch)?;
    let root = abi_word(&checkpoint, 7)?;
    if abi_u64(&checkpoint, 0)? != batch
        || abi_u64(&checkpoint, 12)? != 2
        || abi_u64(&checkpoint, 17)? > committed_height
        || root == [0; 32]
    {
        return Err("custody checkpoint is not finalized at the committed height".into());
    }
    registered.state_root = root;
    registered.trusted_height = batch;
    Ok(registered)
}

pub fn custody_source_address(
    network: &layerx_client::px::PxEndpoint,
    source_did: &str,
) -> Result<[u8; 20], String> {
    Did::new(source_did.as_bytes()).map_err(|error| format!("custody DID: {error:?}"))?;
    let accounts = layerx_client::rpc::history::did_accounts(network, source_did)
        .map_err(|error| format!("custody DID accounts: {error:?}"))?;
    accounts
        .first()
        .and_then(|account| account.get("evm_address"))
        .and_then(serde_json::Value::as_str)
        .and_then(|address| address.strip_prefix("0x"))
        .ok_or_else(|| "custody DID has no bound evm_address".to_owned())
        .and_then(|address| fixed_hex::<20>("evm_address", address))
}

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

pub fn build_asset_send_with_identity_sequence(
    seed: &[u8; 32],
    identity_sequence: u64,
    request: &SendRequest,
    asset: &AuthenticatedCustodyAsset,
) -> Result<SignedSend, String> {
    build_asset_send_with_signer(&SeedSigner::new(seed), identity_sequence, request, asset)
        .map_err(|error| error.to_string())
}

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
    let signed = build_send_for_accounts(signer, identity_sequence, request, from, to)?;
    if signed.source_account != asset.source_account {
        return Err(SendError::Invalid(
            "source asset account substitution".into(),
        ));
    }
    Ok(signed)
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
