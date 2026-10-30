//! Search and fetch through the configured web sidecar, using layerxledger
//! call preparation and authenticated sequencer receipt logs. Content by
//! digest is unpaid, digest checked, and always treated as untrusted.

use std::fmt::Write as _;
use std::io::{Read as _, Write as _};
use std::net::{TcpStream, ToSocketAddrs as _};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::policy::approval::{
    ApprovalContext, ApprovalRegistry, ApprovalState, ApprovalTicket,
};
use layerx_agentd::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
use layerx_crypto::disclosure::call::encode_payment_call;
use layerx_crypto::payments::{Grant, Payment};
use layerx_interop_gateway::evidence::canonical_receipt_digest;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_types::payload::ModuleId;
use layerx_x402::buyer::BuiltPayment;
use layerx_x402::model::{
    PaymentPayload, PaymentRequired, PaymentRequirements, SettlementResponse, X402Error,
};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use sha3::Keccak256;

use crate::approval::{ApprovalError, ApprovalPolicy};
use crate::catalogue::{self, ArgumentError};

pub const PAYMENT_REQUIRED: &str = "PAYMENT-REQUIRED";
pub const PAYMENT_SIGNATURE: &str = "PAYMENT-SIGNATURE";
pub const PAYMENT_RESPONSE: &str = "PAYMENT-RESPONSE";
pub const PAYER_ADDRESS: &str = "LAYERX-PAYER-ADDRESS";

/// The ASCII domain every canonical content encoding starts with.
pub const CONTENT_DOMAIN: &[u8] = b"PAXEERX_WEB_CONTENT_V1";
/// Maximum approval window in seconds, and in heights for the approval hold.
pub const APPROVAL_WINDOW: u64 = 600;
/// The most results one search returns.
pub const MAX_SEARCH_RESULTS: usize = 10;
/// The largest sidecar response the tool reads.
pub const MAX_RESPONSE_BYTES: usize = 9_437_184;

const PURPOSE_DOMAIN: &[u8] = b"LayerX/x-websearch/v1/purpose\0";
const SPEND_DOMAIN: &str = "LayerX/mcp/web/spend/v1";
const SEQUENCER_SIGNED: &str = "sequencer-signed";
const MAX_RETRY_DELAY: Duration = Duration::from_secs(5);
const MAX_TIMEOUT: Duration = Duration::from_secs(60);
const MAX_PENDING_ATTEMPTS: u8 = 10;
const MAX_DID_BYTES: usize = 255;
const GRANT_OPERATION: u16 = 7;

/// The four assets a sidecar accepts.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum Currency {
    Sid,
    Pax,
    Usdc,
    Usdl,
}

impl Currency {
    #[must_use]
    pub const fn code(self) -> &'static str {
        match self {
            Self::Sid => "SID",
            Self::Pax => "PAX",
            Self::Usdc => "USDC",
            Self::Usdl => "USDL",
        }
    }

    #[must_use]
    pub fn parse(code: &str) -> Option<Self> {
        match code {
            "SID" => Some(Self::Sid),
            "PAX" => Some(Self::Pax),
            "USDC" => Some(Self::Usdc),
            "USDL" => Some(Self::Usdl),
            _ => None,
        }
    }
}

/// How one request is paid: a draw against the payer's grant, or a receipt
/// of a payment the payer already made.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum Scheme {
    Metered,
    Exact,
}

impl Scheme {
    #[must_use]
    pub const fn code(self) -> &'static str {
        match self {
            Self::Metered => "metered",
            Self::Exact => "exact",
        }
    }

    #[must_use]
    pub fn parse(code: &str) -> Option<Self> {
        match code {
            "metered" => Some(Self::Metered),
            "exact" => Some(Self::Exact),
            _ => None,
        }
    }

    const fn activity_type(self) -> u16 {
        match self {
            Self::Metered => GRANT_OPERATION,
            Self::Exact => 5,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WebSearch {
    pub query: String,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WebFetch {
    pub url: String,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct WebContent {
    pub digest: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum WebOperation {
    Search(WebSearch),
    Fetch(WebFetch),
    Content(WebContent),
}

impl WebOperation {
    #[must_use]
    pub const fn tool_name(&self) -> &'static str {
        match self {
            Self::Search(_) => "web.search",
            Self::Fetch(_) => "web.fetch",
            Self::Content(_) => "web.content",
        }
    }

    /// The request target on the sidecar.
    #[must_use]
    pub fn target(&self) -> String {
        match self {
            Self::Search(search) => format!("/search?q={}", percent_encode(&search.query)),
            Self::Fetch(fetch) => format!("/fetch?url={}", percent_encode(&fetch.url)),
            Self::Content(content) => format!("/content/{}", hex(&content.digest)),
        }
    }
}

/// One validated web tool call. The currency, scheme and idempotency key
/// bind the payment of a search or a fetch; content by digest is unpaid.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WebCall {
    pub operation: WebOperation,
    pub currency: Currency,
    pub scheme: Scheme,
    pub idempotency_key: [u8; 32],
}

impl WebCall {
    /// Validates MCP call arguments against the catalogue schema of one web
    /// tool and decodes them.
    ///
    /// # Errors
    /// Refuses a tool outside the web tools and every argument the catalogue
    /// refuses, and a zero idempotency key.
    pub fn from_arguments(name: &str, arguments: &Value) -> Result<Self, ArgumentError> {
        if !catalogue::WEB_TOOLS.iter().any(|tool| tool.name == name) {
            return Err(ArgumentError::Unknown(name.to_owned()));
        }
        catalogue::validate(name, arguments)?;
        let text = |field: &'static str| {
            arguments
                .get(field)
                .and_then(Value::as_str)
                .ok_or(ArgumentError::Missing(field))
        };
        let operation = match name {
            "web.search" => WebOperation::Search(WebSearch {
                query: text("query")?.to_owned(),
            }),
            "web.fetch" => WebOperation::Fetch(WebFetch {
                url: text("url")?.to_owned(),
            }),
            _ => WebOperation::Content(WebContent {
                digest: unhex32(text("digest")?).ok_or(ArgumentError::Malformed("digest"))?,
            }),
        };
        let currency =
            Currency::parse(text("currency")?).ok_or(ArgumentError::Malformed("currency"))?;
        let scheme = Scheme::parse(text("scheme")?).ok_or(ArgumentError::Malformed("scheme"))?;
        let idempotency_key = unhex32(text("idempotency_key")?)
            .filter(|key| *key != [0; 32])
            .ok_or(ArgumentError::Malformed("idempotency_key"))?;
        Ok(Self {
            operation,
            currency,
            scheme,
            idempotency_key,
        })
    }
}

impl WebCall {
    /// Binds the approved exact quote to EVM payer, recipient and token addresses.
    pub fn exact_terms(
        &self,
        config: &WebConfig,
        offer: &PaymentRequirements,
        _current_height: u64,
    ) -> Result<ExactTerms, WebToolError> {
        let facts = offer_facts(config, self, offer)?;
        let prepared = spend_preparation(
            config,
            self,
            &self.operation.target(),
            offer,
            &facts,
            self.idempotency_key,
            None,
        )?;
        self.exact_terms_for_disclosure(config, offer, prepared.disclosure)
    }

    fn exact_terms_for_disclosure(
        &self,
        config: &WebConfig,
        offer: &PaymentRequirements,
        disclosure: Disclosure,
    ) -> Result<ExactTerms, WebToolError> {
        if self.scheme != Scheme::Exact || matches!(self.operation, WebOperation::Content(_)) {
            return Err(WebToolError::OfferMismatch("scheme"));
        }
        if offer.scheme != self.scheme.code()
            || offer.network != config.network
            || offer.amount.value() == 0
            || offer.max_timeout_seconds == 0
            || self.idempotency_key == [0; 32]
            || offer
                .extra
                .as_ref()
                .and_then(|extra| extra.pointer("/layerx/currency"))
                .and_then(Value::as_str)
                != Some(self.currency.code())
        {
            return Err(WebToolError::OfferMismatch("offer"));
        }
        let recipient = unhex20(&offer.pay_to)
            .filter(|address| *address == config.receiver_address)
            .ok_or(WebToolError::OfferMismatch("recipient"))?;
        let asset = unhex20(&offer.asset).ok_or(WebToolError::OfferMismatch("asset"))?;
        Ok(ExactTerms {
            payer_did: config.payer_did.clone(),
            payer_account: unhex20(&config.payer_address)
                .ok_or(WebToolError::Configuration("payer_address"))?,
            recipient,
            recipient_account: offer.pay_to.clone(),
            asset,
            expires_at_ms: expiry_milliseconds(disclosure.expiry.0)?,
            disclosure,
            amount: offer.amount.value(),
            idempotency_key: self.idempotency_key,
        })
    }
}

fn unhex20(value: &str) -> Option<[u8; 20]> {
    let value = value.strip_prefix("0x").unwrap_or(value);
    if value.len() != 40 || !value.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return None;
    }
    let mut result = [0; 20];
    for (index, byte) in result.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&value[index * 2..index * 2 + 2], 16).ok()?;
    }
    Some(result)
}

/// The plain HTTP endpoint of one sidecar, `http://host[:port]`.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SidecarEndpoint {
    host: String,
    port: u16,
    authority: String,
}

impl SidecarEndpoint {
    /// # Errors
    /// Refuses anything but `http://` followed by a host and an optional port
    /// with no path, query or credentials.
    pub fn parse(text: &str) -> Result<Self, WebToolError> {
        let refuse = || WebToolError::Configuration("endpoint");
        let authority = text.strip_prefix("http://").ok_or_else(refuse)?;
        let authority = authority.strip_suffix('/').unwrap_or(authority);
        let (host, port) = if let Some(rest) = authority.strip_prefix('[') {
            let (host, tail) = rest.split_once(']').ok_or_else(refuse)?;
            let port = if tail.is_empty() {
                None
            } else {
                Some(tail.strip_prefix(':').ok_or_else(refuse)?)
            };
            (host, port)
        } else {
            match authority.split_once(':') {
                Some((host, port)) => (host, Some(port)),
                None => (authority, None),
            }
        };
        let port = match port {
            Some(port) if port.bytes().all(|byte| byte.is_ascii_digit()) => {
                port.parse::<u16>().map_err(|_| refuse())?
            }
            Some(_) => return Err(refuse()),
            None => 80,
        };
        if host.is_empty()
            || port == 0
            || !host
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || b".-:".contains(&byte))
        {
            return Err(refuse());
        }
        Ok(Self {
            host: host.to_owned(),
            port,
            authority: authority.to_owned(),
        })
    }
}

/// Everything the tool needs besides the call: the sidecar, the payer, the
/// network the offers must settle on and the sequencer trust.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WebConfig {
    endpoint: SidecarEndpoint,
    payer_did: String,
    payer_address: String,
    receiver_address: [u8; 20],
    network: String,
    sequencer_public_key: [u8; 32],
    sequencer_authorization: SequencerAuthorization,
    timeout: Duration,
    pending_attempts: u8,
}

impl WebConfig {
    /// # Errors
    /// Refuses a malformed endpoint, payer DID, payer address or network, a zero sequencer
    /// key, a timeout outside one to sixty seconds and a pending retry count
    /// outside one to ten, naming the field.
    pub fn new(
        endpoint: &str,
        payer_did: &str,
        payer_address: &str,
        receiver_address: [u8; 20],
        network: &str,
        sequencer_public_key: [u8; 32],
        sequencer_id: [u8; 32],
        first_batch_number: u64,
        last_batch_number: u64,
        timeout: Duration,
        pending_attempts: u8,
    ) -> Result<Self, WebToolError> {
        let endpoint = SidecarEndpoint::parse(endpoint)?;
        if !valid_payer(payer_did) {
            return Err(WebToolError::Configuration("payer_did"));
        }
        if !payer_address.strip_prefix("0x").is_some_and(|address| {
            address.len() == 40 && address.bytes().all(|byte| byte.is_ascii_hexdigit())
        }) {
            return Err(WebToolError::Configuration("payer_address"));
        }
        if !network.strip_prefix("eip155:").is_some_and(|rest| {
            !rest.is_empty()
                && rest.len() <= 64
                && !rest.starts_with('0')
                && rest.bytes().all(|byte| byte.is_ascii_digit())
        }) {
            return Err(WebToolError::Configuration("network"));
        }
        if sequencer_public_key == [0; 32] {
            return Err(WebToolError::Configuration("sequencer_public_key"));
        }
        if sequencer_id == [0; 32] {
            return Err(WebToolError::Configuration("sequencer_id"));
        }
        if first_batch_number > last_batch_number {
            return Err(WebToolError::Configuration("sequencer_batch_range"));
        }
        if timeout.is_zero() || timeout > MAX_TIMEOUT {
            return Err(WebToolError::Configuration("timeout"));
        }
        if pending_attempts == 0 || pending_attempts > MAX_PENDING_ATTEMPTS {
            return Err(WebToolError::Configuration("pending_attempts"));
        }
        Ok(Self {
            endpoint,
            payer_did: payer_did.to_owned(),
            payer_address: payer_address.to_owned(),
            receiver_address,
            network: network.to_owned(),
            sequencer_public_key,
            sequencer_authorization: SequencerAuthorization::new(
                sequencer_id,
                sequencer_public_key,
                first_batch_number,
                last_batch_number,
            ),
            timeout,
            pending_attempts,
        })
    }

    #[must_use]
    pub fn payer_did(&self) -> &str {
        &self.payer_did
    }
}

/// The approval boundary one web spend passes through.
pub struct WebApproval<'a> {
    pub registry: &'a ApprovalRegistry,
    pub policy: ApprovalPolicy,
    /// The audit context; its request id is replaced by the call's key.
    pub context: ApprovalContext,
    pub current_sequence: u64,
}

impl WebApproval<'_> {
    #[must_use]
    pub const fn current_height(&self) -> u64 {
        self.current_sequence
    }
}

/// What the payer's grant must bind for a metered offer.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct GrantTerms {
    pub payer_did: String,
    pub payer_account: [u8; 20],
    pub recipient: [u8; 20],
    pub asset: [u8; 20],
    pub expires_at_ms: u64,
    pub amount: u128,
    pub purpose_hash: [u8; 32],
    pub idempotency_key: [u8; 32],
}

/// What the payer's payment must settle for an exact offer.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ExactTerms {
    pub payer_did: String,
    pub payer_account: [u8; 20],
    pub recipient: [u8; 20],
    pub recipient_account: String,
    pub asset: [u8; 20],
    pub expires_at_ms: u64,
    pub disclosure: Disclosure,
    pub amount: u128,
    pub idempotency_key: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ExactPayment {
    pub tx_hash: [u8; 32],
    pub receipt: Vec<u8>,
    pub logs: Vec<SequencerLog>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum WebPayerError {
    Refused,
    Unavailable,
}

/// The host prepares and submits calls; signing keys stay outside the tool.
pub trait WebPayer {
    fn validate_search_quote(
        &mut self,
        _asset: [u8; 32],
        _amount: u128,
    ) -> Result<(), WebPayerError> {
        Err(WebPayerError::Refused)
    }

    fn sign_grant(&mut self, _terms: &GrantTerms) -> Result<Grant, WebPayerError> {
        Err(WebPayerError::Refused)
    }

    fn grant(&mut self, terms: &GrantTerms) -> Result<PrecompileCall, WebPayerError> {
        let grant = self.sign_grant(terms)?;
        terms.prepare_grant(&grant)
    }

    fn view(&mut self, _call: &PrecompileCall) -> Result<Vec<u8>, WebPayerError> {
        Err(WebPayerError::Unavailable)
    }

    fn pay(&mut self, terms: &ExactTerms) -> Result<PrecompileCall, WebPayerError> {
        Ok(terms.prepare_pay_exact())
    }

    fn draw(
        &mut self,
        terms: &GrantTerms,
        grant_id: [u8; 32],
    ) -> Result<PrecompileCall, WebPayerError> {
        let result = self.view(&terms.next_sequence_call())?;
        let sequence = sequence_result(&result)?;
        Ok(terms.prepare_draw(grant_id, sequence))
    }

    fn submit(&mut self, call: &PrecompileCall) -> Result<ExactPayment, WebPayerError>;
}

fn address_word(address: [u8; 20]) -> [u8; 32] {
    let mut word = [0; 32];
    word[12..].copy_from_slice(&address);
    word
}

fn amount_word(amount: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&amount.to_be_bytes());
    word
}

fn payment_call(target: PrecompileTarget, signature: &str, words: Vec<[u8; 32]>) -> PrecompileCall {
    let digest = Keccak256::digest(signature.as_bytes());
    PrecompileCall {
        target,
        selector: [digest[0], digest[1], digest[2], digest[3]],
        arguments: words.into_iter().map(AbiArgument::Word).collect(),
    }
}

impl ExactTerms {
    #[must_use]
    pub fn prepare_pay_exact(&self) -> PrecompileCall {
        payment_call(
            PrecompileTarget::Ledger,
            "payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)",
            vec![
                address_word(self.payer_account),
                address_word(self.recipient),
                address_word(self.asset),
                amount_word(self.amount),
                amount_word(u128::from(self.expires_at_ms)),
                self.idempotency_key,
            ],
        )
    }
}

impl GrantTerms {
    pub fn prepare_grant(&self, grant: &Grant) -> Result<PrecompileCall, WebPayerError> {
        if grant.from != address_word(self.payer_account)
            || grant.recipient != address_word(self.recipient)
            || grant.asset != address_word(self.asset)
            || grant.per_draw_maximum != self.amount
            || grant.allowance != self.amount
            || grant.recurring
            || grant.window_length != 0
            || grant.expiration != self.expires_at_ms
            || grant.purpose_hash != self.purpose_hash
            || !grant.has_reference
            || grant.reference_hash != self.idempotency_key
            || grant.revocation_sequence != 0
        {
            return Err(WebPayerError::Refused);
        }
        let data = encode_payment_call(
            &Payment::IssueGrant(grant.clone()),
            self.payer_did.as_bytes(),
        )
        .map_err(|_| WebPayerError::Refused)?;
        let payload = Payment::IssueGrant(grant.clone())
            .encode(self.payer_did.as_bytes())
            .map_err(|_| WebPayerError::Refused)?;
        Ok(PrecompileCall {
            target: PrecompileTarget::Ledger,
            selector: data[..4].try_into().map_err(|_| WebPayerError::Refused)?,
            arguments: vec![AbiArgument::Bytes(payload)],
        })
    }

    #[must_use]
    pub fn next_sequence_call(&self) -> PrecompileCall {
        payment_call(
            PrecompileTarget::Ledger,
            "nextSequence(bytes32,bytes32)",
            vec![address_word(self.recipient), address_word(self.asset)],
        )
    }

    #[must_use]
    pub fn prepare_draw(&self, grant_id: [u8; 32], receiver_sequence: u64) -> PrecompileCall {
        payment_call(
            PrecompileTarget::Ledger,
            "draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)",
            vec![
                grant_id,
                address_word(self.recipient),
                amount_word(self.amount),
                self.idempotency_key,
                amount_word(u128::from(receiver_sequence)),
                self.purpose_hash,
                [0; 32],
            ],
        )
    }
}

fn sequence_result(result: &[u8]) -> Result<u64, WebPayerError> {
    if result.len() != 32 || result[..24] != [0; 24] {
        return Err(WebPayerError::Refused);
    }
    Ok(u64::from_be_bytes(
        result[24..]
            .try_into()
            .map_err(|_| WebPayerError::Refused)?,
    ))
}

fn unix_seconds() -> Result<u64, WebToolError> {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|elapsed| elapsed.as_secs())
        .map_err(|_| WebToolError::Configuration("clock"))
}

fn expiry_milliseconds(expiry: u64) -> Result<u64, WebToolError> {
    expiry
        .checked_mul(1_000)
        .ok_or(WebToolError::Configuration("approval_window"))
}

#[derive(Debug)]
pub enum WebToolError {
    Configuration(&'static str),
    Transport,
    Protocol(&'static str),
    Status(u16),
    OfferUnavailable,
    OfferMismatch(&'static str),
    ApprovalRequired(Box<ApprovalTicket>),
    ApprovalPending(Box<ApprovalTicket>),
    ApprovalRefused(ApprovalState),
    ApprovalChanged,
    Approval(ApprovalError),
    Payer(WebPayerError),
    PaymentArtifact(&'static str),
    Payment(X402Error),
    PaymentRejected(String),
    SettlementRefused(Option<String>),
    SettlementPending,
    SettlementUnverified,
    SettlementMismatch,
    DigestMismatch,
    ContentMalformed(&'static str),
}

/// The verified settlement of one request.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WebSettlement {
    pub scheme: Scheme,
    pub currency: Currency,
    pub network: String,
    pub asset: [u8; 32],
    pub pay_to: [u8; 32],
    pub amount: u128,
    pub payer: [u8; 32],
    pub receipt_digest: [u8; 32],
    pub transaction: String,
}

/// One search result; every field is external content.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct UntrustedHit {
    pub url: String,
    pub title: String,
    pub snippet: String,
}

/// The released resource. Every text in it is external content.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum WebResult {
    Search {
        opaque_results: Vec<UntrustedHit>,
    },
    Fetch {
        url: String,
        opaque_final_url: String,
        media_type: String,
        digest: [u8; 32],
        length: u64,
        opaque_text: String,
    },
    Content {
        digest: [u8; 32],
        kind: u8,
        opaque_payload: Vec<u8>,
        media_type: String,
        opaque_text: String,
    },
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WebOutcome {
    pub tool: &'static str,
    /// The verified settlement of a paid search or fetch; `None` for the
    /// unpaid content by digest.
    pub settlement: Option<WebSettlement>,
    pub result: WebResult,
}

impl WebOutcome {
    /// The tool result: the settlement evidence of a paid call and the
    /// external content, marked untrusted.
    #[must_use]
    pub fn evidence(&self) -> Value {
        let content = match &self.result {
            WebResult::Search { opaque_results } => json!({
                "results": opaque_results.iter().map(|hit| json!({
                    "url": hit.url,
                    "title": hit.title,
                    "snippet": hit.snippet,
                })).collect::<Vec<_>>(),
            }),
            WebResult::Fetch {
                url,
                opaque_final_url,
                media_type,
                digest,
                length,
                opaque_text,
            } => json!({
                "url": url,
                "final_url": opaque_final_url,
                "media_type": media_type,
                "digest": hex(digest),
                "length": length,
                "text": opaque_text,
            }),
            WebResult::Content {
                digest,
                kind,
                opaque_payload,
                media_type,
                opaque_text,
            } => json!({
                "digest": hex(digest),
                "kind": kind,
                "payload": String::from_utf8_lossy(opaque_payload),
                "media_type": media_type,
                "text": opaque_text,
            }),
        };
        let mut evidence = json!({
            "tool": self.tool,
            "untrusted": true,
            "content": content,
        });
        if let (Some(settlement), Some(fields)) = (&self.settlement, evidence.as_object_mut()) {
            fields.insert(
                "settlement".to_owned(),
                json!({
                    "scheme": settlement.scheme.code(),
                    "currency": settlement.currency.code(),
                    "network": settlement.network,
                    "asset": hex(&settlement.asset),
                    "payTo": hex(&settlement.pay_to),
                    "amount": settlement.amount.to_string(),
                    "payer": hex(&settlement.payer),
                    "receiptDigest": hex(&settlement.receipt_digest),
                    "transaction": settlement.transaction,
                    "verificationLevel": SEQUENCER_SIGNED,
                }),
            );
        }
        evidence
    }
}

/// Performs one web call against the configured sidecar: a paid search or
/// fetch, or the unpaid content by digest.
///
/// # Errors
/// Refuses an unavailable or inconsistent offer, a spend the approval
/// boundary holds, rejects or has not approved, a payer artifact that does
/// not bind the offer, a settlement that is refused, pending past the retry
/// bound, unverified under the sequencer key or not paying the offer from
/// this payer, a payment challenge on the content path, and content whose
/// digest does not match.
pub fn web_tool(
    config: &WebConfig,
    call: &WebCall,
    approval: &WebApproval<'_>,
    payer: &mut dyn WebPayer,
) -> Result<WebOutcome, WebToolError> {
    if let WebOperation::Content(content) = &call.operation {
        return stored_content(config, content);
    }
    let target = call.operation.target();
    let mut headers = Vec::new();
    if call.scheme == Scheme::Metered {
        headers.push((PAYER_ADDRESS, config.payer_address.clone()));
    }
    let challenge = request(config, &target, &headers)?;
    if challenge.status != 402 {
        return Err(WebToolError::Status(challenge.status));
    }
    let offered = challenge
        .header(PAYMENT_REQUIRED)
        .ok_or(WebToolError::Protocol("payment_required_missing"))?;
    let (required_header, offer) = select_offer(config, call, offered)?;
    let required: PaymentRequired = decode_header(&required_header)
        .ok_or(WebToolError::Protocol("payment_required_malformed"))?;
    let mut plane = WebPlane {
        config,
        call,
        approval,
        payer,
        target: &target,
    };
    let accepted = required
        .accepts
        .into_iter()
        .next()
        .ok_or(WebToolError::OfferUnavailable)?;
    let scheme_payload = plane.authorise(&accepted, call.idempotency_key)?;
    let payload = PaymentPayload {
        x402_version: 2,
        resource: Some(required.resource),
        payload: scheme_payload,
        accepted,
        extensions: required.extensions,
    };
    let header = STANDARD.encode(
        serde_json::to_vec(&payload)
            .map_err(|_| WebToolError::Protocol("payment_signature_encoding"))?,
    );
    let built = BuiltPayment {
        header,
        payload,
        idempotency_key: call.idempotency_key,
    };
    headers.push((PAYMENT_SIGNATURE, built.header.clone()));
    let reply = settle(config, &target, &headers)?;
    let response = reply
        .header(PAYMENT_RESPONSE)
        .ok_or(WebToolError::SettlementUnverified)?;
    let settlement = verify_settlement(config, call, &offer, &built, response)?;
    let result = verify_resource(&call.operation, &reply.body)?;
    Ok(WebOutcome {
        tool: call.operation.tool_name(),
        settlement: Some(settlement),
        result,
    })
}

/// Reads stored content by digest. The sidecar serves it unpaid, so a
/// payment challenge on this path is a protocol mismatch and is never paid,
/// and the body is released only when its digest is the one asked for.
fn stored_content(config: &WebConfig, content: &WebContent) -> Result<WebOutcome, WebToolError> {
    let operation = WebOperation::Content(*content);
    let reply = request(config, &operation.target(), &[])?;
    match reply.status {
        200 => Ok(WebOutcome {
            tool: operation.tool_name(),
            settlement: None,
            result: verify_content(&content.digest, &reply.body)?,
        }),
        402 => Err(WebToolError::Protocol("content_payment_required")),
        status => Err(WebToolError::Status(status)),
    }
}

fn settle(
    config: &WebConfig,
    target: &str,
    headers: &[(&str, String)],
) -> Result<HttpReply, WebToolError> {
    let mut attempts = 0_u8;
    loop {
        let reply = request(config, target, headers)?;
        match reply.status {
            200 => return Ok(reply),
            503 if reply.error_code().as_deref() == Some("payment_pending") => {
                attempts = attempts.saturating_add(1);
                if attempts >= config.pending_attempts {
                    return Err(WebToolError::SettlementPending);
                }
                std::thread::sleep(retry_delay(&reply));
            }
            402 => {
                if let Some(header) = reply.header(PAYMENT_RESPONSE) {
                    let response: SettlementResponse =
                        decode_header(header).ok_or(WebToolError::SettlementUnverified)?;
                    return Err(WebToolError::SettlementRefused(response.error_reason));
                }
                return Err(WebToolError::PaymentRejected(
                    reply.error_code().unwrap_or_default(),
                ));
            }
            status => return Err(WebToolError::Status(status)),
        }
    }
}

/// The facts of the one offer a call pays.
#[derive(Clone, Debug, Eq, PartialEq)]
struct OfferFacts {
    receiver_did: String,
    asset: [u8; 32],
    pay_to: [u8; 32],
    amount: u128,
    payer_accounts: [[u8; 32]; 1],
    metered_payer: Option<[u8; 32]>,
    purpose_hash: Option<[u8; 32]>,
}

fn select_offer(
    config: &WebConfig,
    call: &WebCall,
    header: &str,
) -> Result<(String, OfferFacts), WebToolError> {
    let mut required: PaymentRequired =
        decode_header(header).ok_or(WebToolError::Protocol("payment_required_malformed"))?;
    validate_web_challenge(&required)?;
    if required.resource.url
        != format!(
            "http://{}{}",
            config.endpoint.authority,
            call.operation.target()
        )
    {
        return Err(WebToolError::OfferMismatch("resource"));
    }
    required.accepts.retain(|offer| {
        offer.scheme == call.scheme.code()
            && offer.network == config.network
            && offer
                .extra
                .as_ref()
                .and_then(|extra| extra.pointer("/layerx/currency"))
                .and_then(Value::as_str)
                == Some(call.currency.code())
    });
    let [offer] = required.accepts.as_slice() else {
        return Err(WebToolError::OfferUnavailable);
    };
    let facts = offer_facts(config, call, offer)?;
    let encoded = serde_json::to_vec(&required)
        .map_err(|_| WebToolError::Protocol("payment_required_encoding"))?;
    Ok((STANDARD.encode(encoded), facts))
}

fn validate_web_challenge(required: &PaymentRequired) -> Result<(), WebToolError> {
    required
        .resource
        .validate()
        .map_err(WebToolError::Payment)?;
    if required.x402_version != 2
        || required.accepts.is_empty()
        || required.accepts.len() > 32
        || required
            .error
            .as_ref()
            .is_some_and(|error| error.is_empty() || error.len() > 512 || error.contains('\0'))
        || required.extensions.len() > 32
    {
        return Err(WebToolError::Protocol("payment_required_malformed"));
    }
    for offer in &required.accepts {
        if Scheme::parse(&offer.scheme).is_none()
            || offer.amount.value() == 0
            || offer.max_timeout_seconds == 0
            || unhex20(&offer.asset).is_none()
            || unhex20(&offer.pay_to).is_none()
            || !offer.network.strip_prefix("eip155:").is_some_and(|chain| {
                !chain.is_empty()
                    && chain.len() <= 64
                    && !chain.starts_with('0')
                    && chain.bytes().all(|byte| byte.is_ascii_digit())
            })
        {
            return Err(WebToolError::Protocol("payment_required_malformed"));
        }
    }
    for name in required.extensions.keys() {
        if name.is_empty()
            || name.len() > 32
            || !name
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_' | b'.'))
        {
            return Err(WebToolError::Protocol("payment_required_extensions"));
        }
    }
    Ok(())
}

fn offer_facts(
    config: &WebConfig,
    call: &WebCall,
    offer: &PaymentRequirements,
) -> Result<OfferFacts, WebToolError> {
    let asset_address = unhex20(&offer.asset).ok_or(WebToolError::OfferMismatch("asset"))?;
    let payee_address = unhex20(&offer.pay_to)
        .filter(|address| *address == config.receiver_address)
        .ok_or(WebToolError::OfferMismatch("recipient"))?;
    let asset = address_word(asset_address);
    let pay_to = address_word(payee_address);
    let receiver_did = offer
        .extra
        .as_ref()
        .and_then(|extra| extra.pointer("/layerx/receiverDid"))
        .and_then(Value::as_str)
        .filter(|did| valid_payer(did))
        .ok_or(WebToolError::OfferMismatch("receiver_did"))?
        .to_owned();
    let payer_accounts = [address_word(
        unhex20(&config.payer_address).ok_or(WebToolError::Configuration("payer_address"))?,
    )];
    let layerx = offer.extra.as_ref().and_then(|extra| extra.get("layerx"));
    let (metered_payer, purpose_hash) = match call.scheme {
        Scheme::Metered => {
            let payer = layerx
                .and_then(|terms| terms.get("payer"))
                .and_then(Value::as_str)
                .and_then(unhex20)
                .map(address_word)
                .filter(|payer| payer_accounts.contains(payer))
                .ok_or(WebToolError::OfferMismatch("payer"))?;
            let purpose = layerx
                .and_then(|terms| terms.get("purposeHash"))
                .and_then(Value::as_str)
                .and_then(unhex32)
                .filter(|purpose| purpose_of(&config.receiver_address) == *purpose)
                .ok_or(WebToolError::OfferMismatch("purpose"))?;
            (Some(payer), Some(purpose))
        }
        Scheme::Exact => (None, None),
    };
    Ok(OfferFacts {
        receiver_did,
        asset,
        pay_to,
        amount: offer.amount.value(),
        payer_accounts,
        metered_payer,
        purpose_hash,
    })
}

/// The buyer's plane: approval first, then the payer's artifact, checked
/// against the offer before it becomes the scheme payload.
struct WebPlane<'a, 'p> {
    config: &'a WebConfig,
    call: &'a WebCall,
    approval: &'a WebApproval<'a>,
    payer: &'p mut dyn WebPayer,
    target: &'a str,
}

impl WebPlane<'_, '_> {
    fn authorise(
        &mut self,
        offer: &PaymentRequirements,
        key: [u8; 32],
    ) -> Result<Value, WebToolError> {
        let facts = offer_facts(self.config, self.call, offer)?;
        if matches!(self.call.operation, WebOperation::Search(_))
            && self.call.currency == Currency::Pax
        {
            self.payer
                .validate_search_quote(facts.asset, facts.amount)
                .map_err(WebToolError::Payer)?;
        }
        let disclosure = self.approve(offer, &facts, key)?;
        if matches!(self.call.operation, WebOperation::Search(_))
            && self.call.currency == Currency::Pax
        {
            self.payer
                .validate_search_quote(facts.asset, facts.amount)
                .map_err(WebToolError::Payer)?;
        }
        match self.call.scheme {
            Scheme::Metered => {
                let terms = GrantTerms {
                    payer_did: self.config.payer_did.clone(),
                    payer_account: unhex20(&self.config.payer_address)
                        .ok_or(WebToolError::Configuration("payer_address"))?,
                    recipient: self.config.receiver_address,
                    asset: unhex20(&offer.asset).ok_or(WebToolError::OfferMismatch("asset"))?,
                    expires_at_ms: expiry_milliseconds(disclosure.expiry.0)?,
                    amount: facts.amount,
                    purpose_hash: facts
                        .purpose_hash
                        .ok_or(WebToolError::OfferMismatch("purpose"))?,
                    idempotency_key: key,
                };
                let existing = offer
                    .extra
                    .as_ref()
                    .and_then(|extra| extra.pointer("/layerx/grantId"));
                let grant_id = if let Some(existing) = existing {
                    existing
                        .as_str()
                        .and_then(unhex32)
                        .filter(|id| *id != [0; 32])
                        .ok_or(WebToolError::OfferMismatch("grant_id"))?
                } else {
                    let prepared = self.payer.grant(&terms).map_err(WebToolError::Payer)?;
                    let [AbiArgument::Bytes(payload)] = prepared.arguments.as_slice() else {
                        return Err(WebToolError::PaymentArtifact("grant_call"));
                    };
                    let Payment::IssueGrant(grant) = Payment::decode(
                        ModuleId::Asset,
                        GRANT_OPERATION,
                        payload,
                        terms.payer_did.as_bytes(),
                    )
                    .map_err(|_| WebToolError::PaymentArtifact("grant_call"))?
                    else {
                        return Err(WebToolError::PaymentArtifact("grant_call"));
                    };
                    if prepared != terms.prepare_grant(&grant).map_err(WebToolError::Payer)? {
                        return Err(WebToolError::PaymentArtifact("grant_call"));
                    }
                    let payment = self.payer.submit(&prepared).map_err(WebToolError::Payer)?;
                    check_grant(self.config, &payment, grant.id)?;
                    grant.id
                };
                let prepared = self
                    .payer
                    .draw(&terms, grant_id)
                    .map_err(WebToolError::Payer)?;
                let Some(AbiArgument::Word(sequence_word)) = prepared.arguments.get(4) else {
                    return Err(WebToolError::PaymentArtifact("draw_call"));
                };
                let sequence = sequence_result(sequence_word).map_err(WebToolError::Payer)?;
                if prepared != terms.prepare_draw(grant_id, sequence) {
                    return Err(WebToolError::PaymentArtifact("draw_call"));
                }
                let payment = self.payer.submit(&prepared).map_err(WebToolError::Payer)?;
                check_receipt(self.config, &payment, &facts, Some((grant_id, key)), key)?;
                Ok(json!({ "grantId": hex(&grant_id), "idempotencyKey": hex(&key) }))
            }
            Scheme::Exact => {
                let terms = self
                    .call
                    .exact_terms_for_disclosure(self.config, offer, disclosure)?;
                let prepared = self.payer.pay(&terms).map_err(WebToolError::Payer)?;
                if prepared != terms.prepare_pay_exact() {
                    return Err(WebToolError::PaymentArtifact("pay_exact_call"));
                }
                let payment = self.payer.submit(&prepared).map_err(WebToolError::Payer)?;
                let digest = check_receipt(self.config, &payment, &facts, None, key)?;
                Ok(json!({
                    "idempotencyKey": hex(&key), "receiptDigest": hex(&digest),
                    "txHash": format!("0x{}", hex(&payment.tx_hash)), "verificationLevel": "receipted",
                }))
            }
        }
    }

    fn approve(
        &self,
        offer: &PaymentRequirements,
        facts: &OfferFacts,
        key: [u8; 32],
    ) -> Result<Disclosure, WebToolError> {
        let boundary = self.approval;
        let mut context = boundary.context.clone();
        context.request_id = key;
        if context.agent.as_bytes() != self.config.payer_did.as_bytes() {
            return Err(WebToolError::Configuration("approval_agent"));
        }
        let ticket = boundary
            .registry
            .ticket(key)
            .map_err(|error| WebToolError::Approval(ApprovalError::Daemon(error)))?;
        let prepared = spend_preparation(
            self.config,
            self.call,
            self.target,
            offer,
            facts,
            key,
            ticket.as_ref().map(|ticket| ticket.disclosure.expiry.0),
        )?;
        if let Some(ticket) = ticket {
            if ticket.disclosure.canonical_digest != prepared.disclosure.canonical_digest {
                return Err(WebToolError::ApprovalChanged);
            }
            return match ticket.state {
                ApprovalState::Approved
                    if ticket.expires_at_sequence > boundary.current_height()
                        && ticket.disclosure.expiry.0 > unix_seconds()? =>
                {
                    Ok(ticket.disclosure)
                }
                ApprovalState::Approved => {
                    Err(WebToolError::ApprovalRefused(ApprovalState::Expired))
                }
                ApprovalState::AwaitingApproval => {
                    Err(WebToolError::ApprovalPending(Box::new(ticket)))
                }
                state => Err(WebToolError::ApprovalRefused(state)),
            };
        }
        if facts.amount <= boundary.policy.amount_threshold {
            return Ok(prepared.disclosure);
        }
        let expires_at_sequence = boundary
            .current_height()
            .checked_add(APPROVAL_WINDOW)
            .ok_or(WebToolError::Configuration("approval_window"))?;
        let ticket = layerx_agentd::policy::approval::hold(
            boundary.registry,
            context,
            prepared,
            boundary.current_height(),
            expires_at_sequence,
        )
        .map_err(|error| WebToolError::Approval(ApprovalError::Daemon(error)))?;
        Err(WebToolError::ApprovalRequired(Box::new(ticket)))
    }
}

/// The disclosure an approver sees for one web spend. Its canonical bytes
/// bind the tool, the resource, the whole offer and the idempotency key.
fn spend_preparation(
    config: &WebConfig,
    call: &WebCall,
    target: &str,
    offer: &PaymentRequirements,
    facts: &OfferFacts,
    key: [u8; 32],
    approved_expiry: Option<u64>,
) -> Result<Prepared, WebToolError> {
    let expiry = match approved_expiry {
        Some(expiry) => expiry,
        None => unix_seconds()?
            .checked_add(u64::from(offer.max_timeout_seconds).min(APPROVAL_WINDOW))
            .ok_or(WebToolError::Configuration("approval_window"))?,
    };
    expiry_milliseconds(expiry)?;
    let invalid = |_| WebToolError::Configuration("approval_disclosure");
    let canonical = serde_json::to_vec(&json!({
        "domain": SPEND_DOMAIN,
        "tool": call.operation.tool_name(),
        "target": target,
        "offer": offer,
        "idempotencyKey": hex(&key),
        "expiry": expiry,
    }))
    .map_err(|_| WebToolError::Configuration("approval_disclosure"))?;
    let digest: [u8; 32] = Sha256::digest(&canonical).into();
    let receiver = AgentDid::new(facts.receiver_did.clone()).map_err(invalid)?;
    Ok(Prepared {
        preparation_ref: PreparationRef::new(format!("web-{}", hex(&digest[..8])))
            .map_err(invalid)?,
        unsigned_canonical_bytes: CanonicalBytes::new(canonical).map_err(invalid)?,
        signing_preimage: SigningPreimage::new(digest.to_vec()).map_err(invalid)?,
        disclosure: Disclosure {
            canonical_digest: digest,
            activity_type: ActivityType(call.scheme.activity_type()),
            actor: AgentDid::new(config.payer_did.clone()).map_err(invalid)?,
            authority: AuthorityRef::new(format!("x402:{}:{}", call.scheme.code(), config.network))
                .map_err(invalid)?,
            counterparties: ExplicitSet::allow(vec![receiver.clone()]),
            amounts: ExplicitSet::allow(vec![DisclosedAmount {
                counterparty: receiver,
                amount: Amount(facts.amount),
            }]),
            asset: Asset::new(call.currency.code()).map_err(invalid)?,
            fee_limit: Amount(0),
            expiry: TimestampSeconds(expiry),
            idempotency_key: IdempotencyRef::new(hex(&key)).map_err(invalid)?,
        },
        expiry: TimestampSeconds(expiry),
    })
}

fn check_grant(
    config: &WebConfig,
    payment: &ExactPayment,
    grant_id: [u8; 32],
) -> Result<(), WebToolError> {
    let refuse = || WebToolError::PaymentArtifact("grant_receipt");
    let receipt_keys = [config.sequencer_authorization];
    let decoded =
        verify_sequencer_signature(&payment.receipt, &receipt_keys).map_err(|_| refuse())?;
    let protocol = decoded.protocol().ok_or_else(refuse)?;
    if protocol.result_code() != 0 || protocol.activity_id() != payment.tx_hash {
        return Err(refuse());
    }
    let verified = verify_logs(&decoded, payment.logs.clone()).map_err(|_| refuse())?;
    let event: [u8; 32] = Keccak256::digest(b"PayerGrantIssued(bytes32)").into();
    if verified
        .logs()
        .iter()
        .filter(|log| {
            log.address() == PrecompileTarget::Ledger.address()
                && log.topics() == [event, grant_id]
                && log.data().is_empty()
        })
        .count()
        != 1
    {
        return Err(refuse());
    }
    Ok(())
}

/// Verifies the payer's exact receipt: sequencer-signed under the configured
/// key, with authenticated PaymentExact logs paying the offer's payee from
/// this payer in the offer's asset and amount. Returns its receipt digest.
fn check_receipt(
    config: &WebConfig,
    payment: &ExactPayment,
    facts: &OfferFacts,
    draw: Option<([u8; 32], [u8; 32])>,
    reference: [u8; 32],
) -> Result<[u8; 32], WebToolError> {
    let refuse = || WebToolError::PaymentArtifact("receipt");
    let receipt_keys = [config.sequencer_authorization];
    let decoded =
        verify_sequencer_signature(&payment.receipt, &receipt_keys).map_err(|_| refuse())?;
    let protocol = decoded.protocol().ok_or_else(refuse)?;
    if protocol.result_code() != 0 || protocol.activity_id() != payment.tx_hash {
        return Err(refuse());
    }
    let verified = verify_logs(&decoded, payment.logs.clone()).map_err(|_| refuse())?;
    let event: [u8; 32] = Keccak256::digest(if draw.is_some() {
        b"PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)".as_slice()
    } else {
        b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)".as_slice()
    })
    .into();
    let matches = verified
        .logs()
        .iter()
        .filter(|log| {
            let topics = log.topics();
            let data = log.data();
            log.address() == PrecompileTarget::Ledger.address()
                && topics.len() == 4
                && topics[0] == event
                && (draw.is_some() || topics[1] == reference)
                && facts.payer_accounts.contains(&topics[2])
                && topics[3] == facts.pay_to
                && data.len() == if draw.is_some() { 128 } else { 64 }
                && draw.is_none_or(|(grant, key)| {
                    topics[1] == grant && data[64..96] == key && data[96..128] == [0; 32]
                })
                && data[..32] == facts.asset
                && data[32..48] == [0; 16]
                && data[48..64] == facts.amount.to_be_bytes()
        })
        .count();
    if matches != 1 {
        return Err(refuse());
    }
    canonical_receipt_digest(&payment.receipt).map_err(|_| refuse())
}

fn verify_settlement(
    config: &WebConfig,
    call: &WebCall,
    offer: &OfferFacts,
    built: &BuiltPayment,
    header: &str,
) -> Result<WebSettlement, WebToolError> {
    let response: SettlementResponse =
        decode_header(header).ok_or(WebToolError::SettlementUnverified)?;
    if !response.success {
        return Err(WebToolError::SettlementRefused(response.error_reason));
    }
    response
        .validate_wire()
        .map_err(|_| WebToolError::SettlementMismatch)?;
    let evidence = response
        .extensions
        .get("layerx")
        .ok_or(WebToolError::SettlementUnverified)?;
    if !evidence.as_object().is_some_and(|object| {
        object.keys().all(|key| {
            [
                "receipt",
                "receiptDigest",
                "verificationLevel",
                "logs",
                "purposeHash",
                "blockNumber",
                "blockHash",
                "proof",
            ]
            .contains(&key.as_str())
        })
    }) {
        return Err(WebToolError::SettlementUnverified);
    }
    if evidence.get("verificationLevel").and_then(Value::as_str) != Some("receipted")
        || evidence.get("proof").is_some()
        || evidence.get("blockNumber").is_some()
        || evidence.get("blockHash").is_some()
    {
        return Err(WebToolError::SettlementUnverified);
    }
    let receipt = evidence
        .get("receipt")
        .and_then(Value::as_str)
        .and_then(|value| STANDARD.decode(value).ok())
        .ok_or(WebToolError::SettlementUnverified)?;
    let tx_hash = response
        .transaction
        .strip_prefix("0x")
        .and_then(unhex32)
        .ok_or(WebToolError::SettlementMismatch)?;
    let raw_logs = evidence
        .get("logs")
        .and_then(Value::as_array)
        .filter(|logs| logs.len() <= 512)
        .ok_or(WebToolError::SettlementUnverified)?;
    let mut logs = Vec::with_capacity(raw_logs.len());
    for log in raw_logs {
        if !log.as_object().is_some_and(|object| {
            object
                .keys()
                .all(|key| ["address", "topics", "data", "logIndex"].contains(&key.as_str()))
        }) {
            return Err(WebToolError::SettlementUnverified);
        }
        let address = log
            .get("address")
            .and_then(Value::as_str)
            .and_then(unhex20)
            .ok_or(WebToolError::SettlementUnverified)?;
        let topics = log
            .get("topics")
            .and_then(Value::as_array)
            .ok_or(WebToolError::SettlementUnverified)?
            .iter()
            .map(|topic| {
                topic
                    .as_str()
                    .and_then(|topic| unhex32(topic.strip_prefix("0x").unwrap_or(topic)))
                    .ok_or(WebToolError::SettlementUnverified)
            })
            .collect::<Result<Vec<_>, _>>()?;
        let data = log
            .get("data")
            .and_then(Value::as_str)
            .and_then(unhex_bytes)
            .ok_or(WebToolError::SettlementUnverified)?;
        let log_index = log
            .get("logIndex")
            .and_then(Value::as_str)
            .and_then(|value| value.strip_prefix("0x"))
            .filter(|value| {
                !value.is_empty() && value.len() <= 16 && (*value == "0" || !value.starts_with('0'))
            })
            .and_then(|value| u64::from_str_radix(value, 16).ok())
            .ok_or(WebToolError::SettlementUnverified)?;
        logs.push(SequencerLog::new(address, topics, data, log_index));
    }
    let payment = ExactPayment {
        tx_hash,
        receipt,
        logs,
    };
    let draw = if call.scheme == Scheme::Metered {
        let grant = built
            .payload
            .payload
            .get("grantId")
            .and_then(Value::as_str)
            .and_then(unhex32)
            .ok_or(WebToolError::SettlementMismatch)?;
        Some((grant, built.idempotency_key))
    } else {
        None
    };
    let digest = check_receipt(config, &payment, offer, draw, built.idempotency_key)?;
    let expected_purpose = offer.purpose_hash.map(|purpose| hex(&purpose));
    if evidence.get("purposeHash").and_then(Value::as_str) != expected_purpose.as_deref()
        || evidence.get("receiptDigest").and_then(Value::as_str) != Some(hex(&digest).as_str())
        || response.network != config.network
        || response.amount.map(|amount| amount.value()) != Some(offer.amount)
    {
        return Err(WebToolError::SettlementMismatch);
    }
    let payer = response
        .payer
        .as_deref()
        .and_then(unhex20)
        .map(address_word)
        .ok_or(WebToolError::SettlementMismatch)?;
    if !offer.payer_accounts.contains(&payer)
        || offer
            .metered_payer
            .is_some_and(|expected| expected != payer)
    {
        return Err(WebToolError::SettlementMismatch);
    }
    Ok(WebSettlement {
        scheme: call.scheme,
        currency: call.currency,
        network: response.network,
        asset: offer.asset,
        pay_to: offer.pay_to,
        amount: offer.amount,
        payer,
        receipt_digest: digest,
        transaction: response.transaction,
    })
}

fn unhex_bytes(value: &str) -> Option<Vec<u8>> {
    let value = value.strip_prefix("0x")?;
    if value.len() % 2 != 0 {
        return None;
    }
    (0..value.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(value.get(index..index + 2)?, 16).ok())
        .collect()
}

fn verify_resource(operation: &WebOperation, body: &[u8]) -> Result<WebResult, WebToolError> {
    match operation {
        WebOperation::Search(_) => verify_search(body),
        WebOperation::Fetch(fetch) => verify_fetch(&fetch.url, body),
        WebOperation::Content(content) => verify_content(&content.digest, body),
    }
}

/// Reads a search answer: at most ten results of url, title and snippet.
///
/// # Errors
/// Refuses a body that is not that shape.
pub fn verify_search(body: &[u8]) -> Result<WebResult, WebToolError> {
    let refuse = || WebToolError::ContentMalformed("search");
    let value: Value = serde_json::from_slice(body).map_err(|_| refuse())?;
    let results = value
        .get("results")
        .and_then(Value::as_array)
        .filter(|results| results.len() <= MAX_SEARCH_RESULTS)
        .ok_or_else(refuse)?;
    let mut hits = Vec::with_capacity(results.len());
    for result in results {
        let field = |name: &str| result.get(name).and_then(Value::as_str).map(str::to_owned);
        let (Some(url), Some(title), Some(snippet)) =
            (field("url"), field("title"), field("snippet"))
        else {
            return Err(refuse());
        };
        hits.push(UntrustedHit {
            url,
            title,
            snippet,
        });
    }
    Ok(WebResult::Search {
        opaque_results: hits,
    })
}

/// Reads a fetch answer and recomputes its content digest from the
/// requested URL, the media type and the text.
///
/// # Errors
/// Refuses a malformed answer, an answer for another URL, a length that is
/// not the text's, and a digest that does not match the recomputed one.
pub fn verify_fetch(url: &str, body: &[u8]) -> Result<WebResult, WebToolError> {
    let refuse = || WebToolError::ContentMalformed("fetch");
    let value: Value = serde_json::from_slice(body).map_err(|_| refuse())?;
    let text = |name: &str| value.get(name).and_then(Value::as_str);
    let (Some(answered), Some(final_url), Some(media_type), Some(digest), Some(opaque_text)) = (
        text("url"),
        text("final_url"),
        text("media_type"),
        text("digest").and_then(unhex32),
        text("text"),
    ) else {
        return Err(refuse());
    };
    if answered != url {
        return Err(WebToolError::ContentMalformed("fetch_url"));
    }
    let length = u64::try_from(opaque_text.len()).map_err(|_| refuse())?;
    if value.get("length").and_then(Value::as_u64) != Some(length) {
        return Err(WebToolError::ContentMalformed("fetch_length"));
    }
    let canonical = canonical_bytes(1, url.as_bytes(), media_type, opaque_text)?;
    if content_digest(&canonical) != digest {
        return Err(WebToolError::DigestMismatch);
    }
    Ok(WebResult::Fetch {
        url: url.to_owned(),
        opaque_final_url: final_url.to_owned(),
        media_type: media_essence(media_type).ok_or_else(refuse)?,
        digest,
        length,
        opaque_text: opaque_text.to_owned(),
    })
}

/// Checks stored content against the digest it was requested by and
/// decodes its canonical fields.
///
/// # Errors
/// Refuses bytes whose keccak256 is not the digest and bytes that are not
/// exactly one canonical encoding.
pub fn verify_content(digest: &[u8; 32], bytes: &[u8]) -> Result<WebResult, WebToolError> {
    if content_digest(bytes) != *digest {
        return Err(WebToolError::DigestMismatch);
    }
    let refuse = || WebToolError::ContentMalformed("content");
    let rest = bytes.strip_prefix(CONTENT_DOMAIN).ok_or_else(refuse)?;
    let (&kind, rest) = rest.split_first().ok_or_else(refuse)?;
    if !matches!(kind, 1 | 2) {
        return Err(refuse());
    }
    let (payload, rest) = take_prefixed::<4>(rest).ok_or_else(refuse)?;
    let (media_type, rest) = take_prefixed::<4>(rest).ok_or_else(refuse)?;
    let (text, rest) = take_prefixed::<8>(rest).ok_or_else(refuse)?;
    let media_type = std::str::from_utf8(media_type).map_err(|_| refuse())?;
    if !rest.is_empty() || media_essence(media_type).as_deref() != Some(media_type) {
        return Err(refuse());
    }
    Ok(WebResult::Content {
        digest: *digest,
        kind,
        opaque_payload: payload.to_vec(),
        media_type: media_type.to_owned(),
        opaque_text: String::from_utf8(text.to_vec()).map_err(|_| refuse())?,
    })
}

fn take_prefixed<const N: usize>(bytes: &[u8]) -> Option<(&[u8], &[u8])> {
    let (length, rest) = bytes.split_first_chunk::<N>()?;
    let mut wide = [0_u8; 8];
    wide[8 - N..].copy_from_slice(length);
    let length = usize::try_from(u64::from_be_bytes(wide)).ok()?;
    (rest.len() >= length).then(|| rest.split_at(length))
}

/// The canonical content bytes: the domain, the kind, the payload with a
/// big-endian `u32` length, the media type without parameters in lower case
/// with a `u32` length, and the text with a `u64` length.
///
/// # Errors
/// Refuses a malformed media type and a field longer than its length prefix.
pub fn canonical_bytes(
    kind: u8,
    payload: &[u8],
    media_type: &str,
    text: &str,
) -> Result<Vec<u8>, WebToolError> {
    let refuse = || WebToolError::ContentMalformed("canonical");
    let media_type = media_essence(media_type).ok_or_else(refuse)?;
    let payload_length = u32::try_from(payload.len()).map_err(|_| refuse())?;
    let media_length = u32::try_from(media_type.len()).map_err(|_| refuse())?;
    let text_length = u64::try_from(text.len()).map_err(|_| refuse())?;
    let mut bytes = Vec::with_capacity(
        CONTENT_DOMAIN.len() + 17 + payload.len() + media_type.len() + text.len(),
    );
    bytes.extend_from_slice(CONTENT_DOMAIN);
    bytes.push(kind);
    bytes.extend_from_slice(&payload_length.to_be_bytes());
    bytes.extend_from_slice(payload);
    bytes.extend_from_slice(&media_length.to_be_bytes());
    bytes.extend_from_slice(media_type.as_bytes());
    bytes.extend_from_slice(&text_length.to_be_bytes());
    bytes.extend_from_slice(text.as_bytes());
    Ok(bytes)
}

/// keccak256 of canonical content bytes.
#[must_use]
pub fn content_digest(canonical: &[u8]) -> [u8; 32] {
    Keccak256::digest(canonical).into()
}

fn media_essence(media_type: &str) -> Option<String> {
    let essence = media_type.split(';').next()?.trim().to_ascii_lowercase();
    let (kind, subtype) = essence.split_once('/')?;
    let token = |part: &str| {
        !part.is_empty()
            && part
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || b"!#$&-^_.+".contains(&byte))
    };
    (token(kind) && token(subtype)).then_some(essence)
}

fn purpose_of(receiver_address: &[u8; 20]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(PURPOSE_DOMAIN);
    hasher.update(receiver_address);
    hasher.finalize().into()
}

fn valid_payer(did: &str) -> bool {
    did.len() <= MAX_DID_BYTES
        && did.starts_with("did:")
        && did.split(':').count() >= 3
        && !did.ends_with(':')
        && !did.contains("::")
        && !did.contains(":asset:")
        && did
            .bytes()
            .all(|byte| matches!(byte, b'a'..=b'z' | b'0'..=b'9' | b'.' | b'_' | b':' | b'-'))
}

fn decode_header<T: serde::de::DeserializeOwned>(header: &str) -> Option<T> {
    let bytes = STANDARD.decode(header.trim()).ok()?;
    serde_json::from_slice(&bytes).ok()
}

struct HttpReply {
    status: u16,
    headers: Vec<(String, String)>,
    body: Vec<u8>,
}

impl HttpReply {
    fn header(&self, name: &str) -> Option<&str> {
        self.headers
            .iter()
            .find(|(candidate, _)| candidate.eq_ignore_ascii_case(name))
            .map(|(_, value)| value.as_str())
    }

    fn error_code(&self) -> Option<String> {
        serde_json::from_slice::<Value>(&self.body)
            .ok()?
            .get("error")?
            .as_str()
            .map(str::to_owned)
    }
}

fn retry_delay(reply: &HttpReply) -> Duration {
    reply
        .header("Retry-After")
        .and_then(|value| value.trim().parse::<u64>().ok())
        .map_or(Duration::from_secs(1), Duration::from_secs)
        .min(MAX_RETRY_DELAY)
}

fn request(
    config: &WebConfig,
    target: &str,
    headers: &[(&str, String)],
) -> Result<HttpReply, WebToolError> {
    http_request(config, "GET", target, headers, &[])
}

fn http_request(
    config: &WebConfig,
    method: &str,
    target: &str,
    headers: &[(&str, String)],
    body: &[u8],
) -> Result<HttpReply, WebToolError> {
    let endpoint = &config.endpoint;
    let address = (endpoint.host.as_str(), endpoint.port)
        .to_socket_addrs()
        .map_err(|_| WebToolError::Transport)?
        .next()
        .ok_or(WebToolError::Transport)?;
    let mut stream = TcpStream::connect_timeout(&address, config.timeout)
        .map_err(|_| WebToolError::Transport)?;
    stream
        .set_read_timeout(Some(config.timeout))
        .and_then(|()| stream.set_write_timeout(Some(config.timeout)))
        .map_err(|_| WebToolError::Transport)?;
    let mut head = format!(
        "{method} {target} HTTP/1.1\r\nHost: {}\r\nConnection: close\r\n",
        endpoint.authority
    );
    for (name, value) in headers {
        if value.bytes().any(|byte| byte.is_ascii_control()) {
            return Err(WebToolError::Protocol("header_value"));
        }
        write!(head, "{name}: {value}\r\n").map_err(|_| WebToolError::Protocol("header_value"))?;
    }
    if !body.is_empty() {
        write!(head, "Content-Length: {}\r\n", body.len())
            .map_err(|_| WebToolError::Protocol("request_body"))?;
    }
    head.push_str("\r\n");
    stream
        .write_all(head.as_bytes())
        .and_then(|()| stream.write_all(body))
        .map_err(|_| WebToolError::Transport)?;
    let mut raw = Vec::new();
    let limit = u64::try_from(MAX_RESPONSE_BYTES)
        .map_err(|_| WebToolError::Protocol("response_limit"))?
        .saturating_add(1);
    stream
        .take(limit)
        .read_to_end(&mut raw)
        .map_err(|_| WebToolError::Transport)?;
    if raw.len() > MAX_RESPONSE_BYTES {
        return Err(WebToolError::Protocol("response_too_large"));
    }
    parse_reply(&raw)
}

fn parse_reply(raw: &[u8]) -> Result<HttpReply, WebToolError> {
    let malformed = || WebToolError::Protocol("malformed_response");
    let end = raw
        .windows(4)
        .position(|window| window == b"\r\n\r\n")
        .ok_or_else(malformed)?;
    let head = std::str::from_utf8(&raw[..end]).map_err(|_| malformed())?;
    let mut lines = head.split("\r\n");
    let status = lines
        .next()
        .and_then(|line| {
            line.strip_prefix("HTTP/1.1 ")
                .or_else(|| line.strip_prefix("HTTP/1.0 "))
        })
        .and_then(|rest| rest.get(..3))
        .and_then(|code| code.parse::<u16>().ok())
        .ok_or_else(malformed)?;
    let mut headers = Vec::new();
    for line in lines {
        let (name, value) = line.split_once(':').ok_or_else(malformed)?;
        headers.push((name.trim().to_owned(), value.trim().to_owned()));
    }
    let mut body = raw[end + 4..].to_vec();
    let declared = headers
        .iter()
        .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
        .map(|(_, value)| value.parse::<usize>().map_err(|_| malformed()))
        .transpose()?;
    if let Some(length) = declared {
        if body.len() < length {
            return Err(malformed());
        }
        body.truncate(length);
    }
    Ok(HttpReply {
        status,
        headers,
        body,
    })
}

fn percent_encode(text: &str) -> String {
    let mut encoded = String::with_capacity(text.len());
    for byte in text.bytes() {
        if byte.is_ascii_alphanumeric() || b"-._~".contains(&byte) {
            encoded.push(char::from(byte));
        } else {
            let _ = write!(encoded, "%{byte:02X}");
        }
    }
    encoded
}

fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut output = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        output.push(char::from(DIGITS[usize::from(byte >> 4)]));
        output.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    output
}

fn unhex32(text: &str) -> Option<[u8; 32]> {
    if text.len() != 64 {
        return None;
    }
    let mut output = [0_u8; 32];
    for (index, pair) in text.as_bytes().chunks_exact(2).enumerate() {
        let pair = std::str::from_utf8(pair).ok()?;
        output[index] = u8::from_str_radix(pair, 16).ok()?;
    }
    Some(output)
}
