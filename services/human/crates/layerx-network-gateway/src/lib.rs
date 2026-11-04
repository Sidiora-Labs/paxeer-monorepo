//! One client for the Paxeer X Network gateway's `px_*` reads.
//!
//! The network gateway serves a single JSON-RPC endpoint that joins the two
//! execution domains: `px_resolveAccount`, `px_getAccount`, `px_getBalances`,
//! `px_listAssets` and `px_getNetwork`, alongside the byte-faithful `eth_*`
//! relay. Every service that reads those joins shares this client rather than
//! restating the transport, the JSON-RPC envelope or the answer grammar.
//!
//! Nothing here carries a protocol proof. Every value this client returns is
//! what the gateway reported, tagged [`Evidence::GatewayReported`], so no
//! caller can mistake a gateway answer for receipt-verified state. An
//! unavailable or malformed answer is a typed [`GatewayError`]: this client
//! never substitutes a zero, an empty table or a default for a read it could
//! not complete.

use std::fmt;
use std::io::{Read, Write};
use std::net::{TcpStream, ToSocketAddrs as _};
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Duration;

use serde_json::Value;

const DID_PREFIX: &str = "did:layerx:";
const ANSWER_LIMIT: u64 = 8 * 1024 * 1024;
const IO_TIMEOUT: Duration = Duration::from_secs(10);
const MAXIMUM_JOINED_ASSETS: usize = 1_024;

/// Longest accepted `pax_address` text.
pub const PAX_ADDRESS_LIMIT: usize = 128;
/// Longest accepted asset denomination text.
pub const DENOM_LIMIT: usize = 128;
/// Longest accepted network identifier text.
pub const NETWORK_ID_LIMIT: usize = 64;

static NEXT_REQUEST: AtomicU64 = AtomicU64::new(1);

/// How a fact reached a caller. A gateway answer is never upgraded to a
/// verified level: the two sources stay distinguishable at every hop.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum Evidence {
    /// Reported by the network gateway without a caller-checkable proof.
    GatewayReported,
}

impl Evidence {
    /// Returns the stable lowercase label used in APIs and logs.
    #[must_use]
    pub const fn label(self) -> &'static str {
        match self {
            Self::GatewayReported => "gateway-reported",
        }
    }
}

/// Refusal for a spelling that is not one of the three public account forms.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct IdentifierError;

impl fmt::Display for IdentifierError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(
            "account identifier is not an EVM address, a did:layerx identifier or a LayerX account",
        )
    }
}

impl std::error::Error for IdentifierError {}

/// One of the three public spellings of the same unified account.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum AccountIdentifier {
    /// A twenty-byte Paxeer EVM address.
    Evm([u8; 20]),
    /// A `LayerX` decentralised identifier public key.
    Did([u8; 32]),
    /// A `LayerX` account identifier.
    Account([u8; 32]),
}

impl AccountIdentifier {
    /// Parses and normalises any of the three public spellings.
    ///
    /// # Errors
    /// Refuses every other spelling, including partial or over-long hexadecimal.
    pub fn parse(text: &str) -> Result<Self, IdentifierError> {
        let trimmed = text.trim();
        let lowered = trimmed.to_ascii_lowercase();
        if let Some(body) = lowered.strip_prefix("0x") {
            return decode_evm(body).map(Self::Evm);
        }
        if let Some(body) = lowered.strip_prefix(DID_PREFIX) {
            return decode_digest(body).map(Self::Did);
        }
        decode_digest(&lowered).map(Self::Account)
    }

    /// Renders the exact normalised spelling this identifier is addressed by.
    #[must_use]
    pub fn canonical_text(self) -> String {
        match self {
            Self::Evm(address) => format!("0x{}", encode_hex(&address)),
            Self::Did(key) => format!("{DID_PREFIX}{}", encode_hex(&key)),
            Self::Account(account) => encode_hex(&account),
        }
    }

    /// The EVM address this identifier names directly, if it names one.
    #[must_use]
    pub const fn evm_address(self) -> Option<[u8; 20]> {
        match self {
            Self::Evm(address) => Some(address),
            Self::Did(_) | Self::Account(_) => None,
        }
    }
}

impl fmt::Display for AccountIdentifier {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(&self.canonical_text())
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum LookupSelector {
    Evm([u8; 20]),
    Did([u8; 32]),
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct NativeAccountReference(pub [u8; 32]);

impl TryFrom<AccountIdentifier> for LookupSelector {
    type Error = GatewayError;
    fn try_from(value: AccountIdentifier) -> Result<Self, Self::Error> {
        match value {
            AccountIdentifier::Evm(address) => Ok(Self::Evm(address)),
            AccountIdentifier::Did(key) => Ok(Self::Did(key)),
            AccountIdentifier::Account(_) => Err(GatewayError::Refused {
                code: -32001,
                message: "native_account_reverse_resolution_unsupported".to_owned(),
            }),
        }
    }
}

impl LookupSelector {
    #[must_use]
    pub const fn identifier(self) -> AccountIdentifier {
        match self {
            Self::Evm(address) => AccountIdentifier::Evm(address),
            Self::Did(key) => AccountIdentifier::Did(key),
        }
    }
    #[must_use]
    pub fn canonical_text(self) -> String {
        self.identifier().canonical_text()
    }
}

const DIGITS: &[u8; 16] = b"0123456789abcdef";

/// Encodes bytes as lowercase hexadecimal.
#[must_use]
pub fn encode_hex(bytes: &[u8]) -> String {
    let mut encoded = String::with_capacity(bytes.len().saturating_mul(2));
    for byte in bytes {
        encoded.push(char::from(DIGITS[usize::from(byte >> 4)]));
        encoded.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    encoded
}

fn nibble(byte: u8) -> Option<u8> {
    match byte {
        b'0'..=b'9' => Some(byte - b'0'),
        b'a'..=b'f' => Some(byte - b'a' + 10),
        b'A'..=b'F' => Some(byte - b'A' + 10),
        _ => None,
    }
}

/// Decodes an even-length hexadecimal string.
///
/// # Errors
/// Refuses odd lengths and non-hexadecimal characters.
pub fn decode_hex(text: &str) -> Result<Vec<u8>, IdentifierError> {
    if !text.len().is_multiple_of(2) {
        return Err(IdentifierError);
    }
    let mut bytes = Vec::with_capacity(text.len() / 2);
    for pair in text.as_bytes().chunks_exact(2) {
        let high = nibble(pair[0]).ok_or(IdentifierError)?;
        let low = nibble(pair[1]).ok_or(IdentifierError)?;
        bytes.push((high << 4) | low);
    }
    Ok(bytes)
}

/// Decodes exactly thirty-two hexadecimal-encoded bytes.
///
/// # Errors
/// Refuses wrong lengths and non-hexadecimal characters.
pub fn decode_digest(text: &str) -> Result<[u8; 32], IdentifierError> {
    if text.len() != 64 {
        return Err(IdentifierError);
    }
    let bytes = decode_hex(text)?;
    <[u8; 32]>::try_from(bytes.as_slice()).map_err(|_| IdentifierError)
}

fn decode_evm(body: &str) -> Result<[u8; 20], IdentifierError> {
    let bytes = decode_hex(body)?;
    <[u8; 20]>::try_from(bytes.as_slice()).map_err(|_| IdentifierError)
}

/// Both identities of one account as the gateway reports them.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResolvedIdentities {
    pub lookup_selector: Option<LookupSelector>,
    /// The bound Paxeer EVM address, when the network knows one.
    pub evm_address: Option<[u8; 20]>,
    /// The bech32 Paxeer address, when the network reports one.
    pub pax_address: Option<String>,
    /// The bound `LayerX` decentralised identifier public key.
    pub layerx_did: Option<[u8; 32]>,
    /// The bound `LayerX` account identifier.
    pub layerx_account: Option<[u8; 32]>,
    /// Whether the two halves are bound to each other.
    pub bound: bool,
    /// How this fact reached the caller.
    pub evidence: Evidence,
}

impl ResolvedIdentities {
    #[must_use]
    pub fn native_reference(&self) -> Option<NativeAccountReference> {
        self.layerx_account.map(NativeAccountReference)
    }

    pub fn retain_lookup(
        &mut self,
        requested: AccountIdentifier,
    ) -> Result<LookupSelector, GatewayError> {
        let lookup = LookupSelector::try_from(requested)?;
        if self.bound != (self.layerx_did.is_some() && self.layerx_account.is_some())
            || self.layerx_did.is_some() != self.layerx_account.is_some()
            || (self.bound && self.evm_address.is_none())
        {
            return Err(GatewayError::Unbound);
        }
        let matches = match lookup {
            LookupSelector::Evm(address) => self.evm_address == Some(address),
            LookupSelector::Did(key) => self.layerx_did == Some(key),
        };
        if !matches {
            return Err(GatewayError::Unbound);
        }
        self.lookup_selector = Some(lookup);
        Ok(lookup)
    }

    /// The native display/reference key, never a reverse-resolution selector.
    /// Repeated unified reads use `lookup_selector` instead.
    #[must_use]
    pub fn canonical(&self, requested: AccountIdentifier) -> AccountIdentifier {
        if let Some(account) = self.layerx_account {
            return AccountIdentifier::Account(account);
        }
        if let Some(address) = self.evm_address {
            if requested.evm_address().is_some() {
                return AccountIdentifier::Evm(address);
            }
        }
        requested
    }
}

/// An exact unsigned on-chain amount, including the full uint256 domain.
#[derive(Clone, Copy, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub struct Quantity([u8; 32]);

impl Quantity {
    /// Preserves a complete big-endian uint256 word.
    #[must_use]
    pub const fn from_be_bytes(bytes: [u8; 32]) -> Self {
        Self(bytes)
    }

    /// Returns the complete big-endian uint256 word.
    #[must_use]
    pub const fn to_be_bytes(self) -> [u8; 32] {
        self.0
    }

    /// Narrows an amount only for the explicitly named business operation.
    ///
    /// # Errors
    /// Refuses amounts that the operation's 128-bit amount cannot represent.
    pub fn for_operation(self, operation: &'static str) -> Result<u128, GatewayError> {
        if self.0[..16].iter().any(|byte| *byte != 0) {
            return Err(GatewayError::AmountOutOfRange { operation });
        }
        let mut bytes = [0; 16];
        bytes.copy_from_slice(&self.0[16..]);
        Ok(u128::from_be_bytes(bytes))
    }

    fn counted(self) -> Result<u64, GatewayError> {
        if self.0[..24].iter().any(|byte| *byte != 0) {
            return Err(GatewayError::MalformedAnswer);
        }
        let mut bytes = [0; 8];
        bytes.copy_from_slice(&self.0[24..]);
        Ok(u64::from_be_bytes(bytes))
    }
}

impl From<u128> for Quantity {
    fn from(value: u128) -> Self {
        let mut bytes = [0; 32];
        bytes[16..].copy_from_slice(&value.to_be_bytes());
        Self(bytes)
    }
}

impl fmt::Display for Quantity {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        let mut value = self.0;
        let mut digits = [b'0'; 78];
        let mut start = digits.len();
        loop {
            let mut remainder = 0_u16;
            for byte in &mut value {
                let current = (remainder << 8) | u16::from(*byte);
                *byte = (current / 10) as u8;
                remainder = current % 10;
            }
            start -= 1;
            digits[start] = b'0' + remainder as u8;
            if value.iter().all(|byte| *byte == 0) {
                break;
            }
        }
        for byte in &digits[start..] {
            write!(formatter, "{}", char::from(*byte))?;
        }
        Ok(())
    }
}

/// The Paxeer half of one account as `px_getAccount` reports it.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PaxeerAccount {
    /// The EVM address the balance and nonce were read at.
    pub address: [u8; 20],
    /// The native balance at the reported block height.
    pub balance: Quantity,
    /// The transaction count at the reported block height.
    pub nonce: u64,
    /// The block height at which the balance and nonce were read.
    pub height: u64,
}

/// One chain account as `px_getAccount` reports it.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AccountJoin {
    /// The resolved identities of the account.
    pub account: ResolvedIdentities,
    /// The Paxeer half, absent when the account is not bound to an address.
    pub paxeer: Option<PaxeerAccount>,
    /// How this fact reached the caller.
    pub evidence: Evidence,
}

/// One bank coin as `px_getBalances` reports it.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AssetBalance {
    /// The bank denomination.
    pub denom: String,
    /// The exact balance at the answer height.
    pub amount: Quantity,
}

/// The bounded bank balance table for one account at one height.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AccountBalances {
    /// The resolved identities the table was read for.
    pub account: ResolvedIdentities,
    /// The block height at which every coin was read.
    pub height: u64,
    /// One row per bank coin, in the gateway's own order.
    pub balances: Vec<AssetBalance>,
    /// How this fact reached the caller.
    pub evidence: Evidence,
}

impl AccountBalances {
    fn validate_denominations(&self) -> Result<(), GatewayError> {
        let mut seen = std::collections::BTreeSet::new();
        for row in &self.balances {
            if !seen.insert(row.denom.as_str()) {
                return Err(GatewayError::MalformedAnswer);
            }
        }
        Ok(())
    }

    /// The row for one bank denomination, when the table carries it.
    #[must_use]
    pub fn asset(&self, denom: &str) -> Option<&AssetBalance> {
        self.balances.iter().find(|row| row.denom == denom)
    }
}

/// The dependency-free wallet observation consumed by human journeys.
///
/// Domain-specific address and amount wrappers are applied by the human
/// service after this shared read and acceptance boundary.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WalletObservation {
    wallet: [u8; 20],
    balances: AccountBalances,
}

impl WalletObservation {
    /// Accepts the public balance result at the human consumer boundary.
    ///
    /// # Errors
    /// Refuses an unbound wallet or ambiguous denomination rows.
    pub fn accept(balances: AccountBalances) -> Result<Self, GatewayError> {
        balances.validate_denominations()?;
        if !balances.account.bound {
            return Err(GatewayError::Unbound);
        }
        let wallet = balances.account.evm_address.ok_or(GatewayError::Unbound)?;
        Ok(Self { wallet, balances })
    }

    /// The bound wallet reported by the producer.
    #[must_use]
    pub const fn wallet(&self) -> [u8; 20] {
        self.wallet
    }

    /// The exact bank amount, absent when the producer supplied no row.
    #[must_use]
    pub fn balance(&self, denom: &str) -> Option<Quantity> {
        self.balances.asset(denom).map(|row| row.amount)
    }

    /// Reads a balance for a named operation with a 128-bit business amount.
    ///
    /// # Errors
    /// Refuses an existing balance too large for that operation; absence stays
    /// distinct from both refusal and an observed zero.
    pub fn balance_for_operation(
        &self,
        denom: &str,
        operation: &'static str,
    ) -> Result<Option<u128>, GatewayError> {
        self.balance(denom)
            .map(|amount| amount.for_operation(operation))
            .transpose()
    }

    /// The original joined result, including its evidence and height.
    #[must_use]
    pub const fn joined(&self) -> &AccountBalances {
        &self.balances
    }
}

/// One chain asset from `px_listAssets`.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AssetEntry {
    /// The indexer's asset identifier, including bank denomination identifiers.
    pub asset_id: String,
    /// The bank denomination, absent for assets without one.
    pub denom: Option<String>,
    /// The indexer's asset kind.
    pub kind: String,
    /// The chain name, absent when the metadata read failed.
    pub name: Option<String>,
    /// The chain symbol, absent when the metadata read failed.
    pub symbol: Option<String>,
    /// The chain decimal precision, absent when the metadata read failed.
    pub decimals: Option<u8>,
    /// The chain supply, absent when the metadata read failed.
    pub supply: Option<Quantity>,
}

/// The bounded chain asset table at one height.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AssetTable {
    /// The block height at which the chain metadata was read.
    pub height: u64,
    /// One entry per joined asset, in the gateway's own order.
    pub assets: Vec<AssetEntry>,
    /// The number of assets the gateway joins per answer.
    pub joined_limit: u64,
    /// How this fact reached the caller.
    pub evidence: Evidence,
}

/// The consensus head reported by the gateway.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ConsensusHead {
    /// The latest consensus block height.
    pub latest_height: u64,
    /// The latest consensus block hash.
    pub latest_block_hash: [u8; 32],
    /// The latest consensus application hash.
    pub latest_app_hash: [u8; 32],
    /// Whether the consensus node is catching up.
    pub catching_up: bool,
}

/// One head for the whole network, as `px_getNetwork` reports it.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NetworkHead {
    /// The network identifier text.
    pub network_id: String,
    /// The Paxeer EVM chain identifier.
    pub chain_id: u64,
    /// The Paxeer block the gateway currently reports as latest.
    pub latest_block: u64,
    /// The consensus head, absent when consensus is unavailable.
    pub consensus: Option<ConsensusHead>,
    /// How this fact reached the caller.
    pub evidence: Evidence,
}

/// Every way a gateway read can fail to produce a usable answer.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum GatewayError {
    /// The endpoint is not `http://<host>[:<port>]` or `https://<host>[:<port>]`.
    InvalidEndpoint,
    /// The endpoint could not be reached or answered outside HTTP.
    Transport(String),
    /// The gateway answered a JSON-RPC error.
    Refused {
        /// The gateway's own refusal code.
        code: i64,
        /// The gateway's own refusal message.
        message: String,
    },
    /// The answer is not a JSON-RPC answer for the request that was sent.
    Unbound,
    /// The answer is not the documented gateway document.
    MalformedAnswer,
    /// A valid chain amount exceeds a named operation's business amount.
    AmountOutOfRange {
        /// The operation that requested the narrower amount.
        operation: &'static str,
    },
}

impl fmt::Display for GatewayError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::InvalidEndpoint => formatter.write_str("gateway endpoint is not http(s)://host"),
            Self::Transport(error) => write!(formatter, "gateway transport failed: {error}"),
            Self::Refused { code, message } => {
                write!(formatter, "gateway refused the read: {code} {message}")
            }
            Self::Unbound => formatter.write_str("gateway answer is bound to another request"),
            Self::MalformedAnswer => formatter.write_str("gateway answer is malformed"),
            Self::AmountOutOfRange { operation } => {
                write!(
                    formatter,
                    "chain amount exceeds the 128-bit amount for {operation}"
                )
            }
        }
    }
}

impl std::error::Error for GatewayError {}

/// The network gateway's JSON-RPC endpoint: one endpoint for `px_*` joins and
/// the unchanged `eth_*` reads.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct GatewayEndpoint {
    secure: bool,
    host: String,
    port: u16,
    path: String,
}

impl GatewayEndpoint {
    /// Parses `http://host[:port][/path]` or `https://host[:port][/path]`.
    ///
    /// # Errors
    /// Refuses another scheme, an empty host, a zero port and a host outside
    /// the ASCII host grammar.
    pub fn parse(endpoint: &str) -> Result<Self, GatewayError> {
        let trimmed = endpoint.trim();
        let (secure, rest) = if let Some(rest) = trimmed.strip_prefix("https://") {
            (true, rest)
        } else if let Some(rest) = trimmed.strip_prefix("http://") {
            (false, rest)
        } else {
            return Err(GatewayError::InvalidEndpoint);
        };
        let (authority, path) = rest
            .split_once('/')
            .map_or((rest, "/".to_owned()), |(authority, path)| {
                (authority, format!("/{path}"))
            });
        let (host, port) = match authority.rsplit_once(':') {
            Some((host, port)) => (
                host,
                port.parse::<u16>()
                    .map_err(|_| GatewayError::InvalidEndpoint)?,
            ),
            None => (authority, if secure { 443 } else { 80 }),
        };
        if port == 0
            || host.is_empty()
            || !host
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || byte == b'.' || byte == b'-')
        {
            return Err(GatewayError::InvalidEndpoint);
        }
        Ok(Self {
            secure,
            host: host.to_ascii_lowercase(),
            port,
            path,
        })
    }

    /// Reads the endpoint the deployment declares in
    /// `LAYERX_NETWORK_GATEWAY_ENDPOINT`.
    ///
    /// # Errors
    /// Refuses an unset, empty or malformed endpoint. A service that reads the
    /// network through the gateway never falls back to a default address.
    pub fn from_environment() -> Result<Self, GatewayError> {
        let declared =
            std::env::var(ENDPOINT_VARIABLE).map_err(|_| GatewayError::InvalidEndpoint)?;
        Self::parse(&declared)
    }

    /// Posts one JSON-RPC request and returns its bounded answer body.
    ///
    /// # Errors
    /// Reports connection, TLS and HTTP framing failures.
    pub fn post(&self, body: &str) -> Result<Vec<u8>, GatewayError> {
        let head = format!(
            "POST {} HTTP/1.1\r\nHost: {}:{}\r\nContent-Type: application/json\r\nAccept: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
            self.path,
            self.host,
            self.port,
            body.len()
        );
        let mut last = "gateway endpoint has no address".to_owned();
        let mut connected = None;
        for address in (self.host.as_str(), self.port)
            .to_socket_addrs()
            .map_err(|error| GatewayError::Transport(format!("resolution failed: {error}")))?
        {
            match TcpStream::connect_timeout(&address, IO_TIMEOUT) {
                Ok(stream) => {
                    connected = Some(stream);
                    break;
                }
                Err(error) => last = format!("connection failed: {error}"),
            }
        }
        let stream = connected.ok_or(GatewayError::Transport(last))?;
        stream
            .set_read_timeout(Some(IO_TIMEOUT))
            .and_then(|()| stream.set_write_timeout(Some(IO_TIMEOUT)))
            .map_err(|error| GatewayError::Transport(format!("timeout setup failed: {error}")))?;
        let answer = if self.secure {
            let connector = native_tls::TlsConnector::builder()
                .build()
                .map_err(|error| GatewayError::Transport(format!("TLS setup failed: {error}")))?;
            let mut stream = connector.connect(&self.host, stream).map_err(|error| {
                GatewayError::Transport(format!("TLS handshake failed: {error}"))
            })?;
            exchange(&mut stream, &head, body.as_bytes())?
        } else {
            let mut stream = stream;
            exchange(&mut stream, &head, body.as_bytes())?
        };
        http_body(&answer)
    }

    /// Posts one `px_*` or `eth_*` call and returns its JSON-RPC result.
    ///
    /// # Errors
    /// Reports the first transport, binding or refusal failure.
    pub fn call(&self, method: &str, params: &[Value]) -> Result<Value, GatewayError> {
        let id = NEXT_REQUEST.fetch_add(1, Ordering::Relaxed);
        let answer = self.post(&rpc_request(id, method, params))?;
        rpc_result(id, &answer)
    }

    /// Reads `px_resolveAccount` for one account spelling.
    ///
    /// # Errors
    /// Reports the first transport, refusal or decoding failure.
    pub fn resolve_account(
        &self,
        account: AccountIdentifier,
    ) -> Result<ResolvedIdentities, GatewayError> {
        let key = Value::String(LookupSelector::try_from(account)?.canonical_text());
        let mut identities = decode_identities(&self.call("px_resolveAccount", &[key])?)?;
        identities.retain_lookup(account)?;
        Ok(identities)
    }

    /// Reads `px_getAccount` for one account spelling.
    ///
    /// # Errors
    /// Reports the first transport, refusal or decoding failure.
    pub fn get_account(&self, account: AccountIdentifier) -> Result<AccountJoin, GatewayError> {
        let key = Value::String(LookupSelector::try_from(account)?.canonical_text());
        let mut joined = decode_account(&self.call("px_getAccount", &[key])?)?;
        joined.account.retain_lookup(account)?;
        if joined
            .paxeer
            .as_ref()
            .is_some_and(|pax| Some(pax.address) != joined.account.evm_address)
        {
            return Err(GatewayError::Unbound);
        }
        Ok(joined)
    }

    /// Reads `px_getBalances` for one account spelling.
    ///
    /// # Errors
    /// Reports the first transport, refusal or decoding failure.
    pub fn get_balances(
        &self,
        account: AccountIdentifier,
    ) -> Result<AccountBalances, GatewayError> {
        let key = Value::String(LookupSelector::try_from(account)?.canonical_text());
        let mut balances = decode_account_balances(&self.call("px_getBalances", &[key])?)?;
        balances.account.retain_lookup(account)?;
        Ok(balances)
    }

    /// Reads the joined asset map through `px_listAssets`.
    ///
    /// # Errors
    /// Reports the first transport, refusal or decoding failure.
    pub fn list_assets(&self) -> Result<AssetTable, GatewayError> {
        decode_assets(&self.call("px_listAssets", &[])?)
    }

    /// Reads the network head through `px_getNetwork`.
    ///
    /// # Errors
    /// Reports the first transport, refusal or decoding failure.
    pub fn get_network(&self) -> Result<NetworkHead, GatewayError> {
        decode_network(&self.call("px_getNetwork", &[])?)
    }
}

/// The deployment variable every service reads the gateway endpoint from.
pub const ENDPOINT_VARIABLE: &str = "LAYERX_NETWORK_GATEWAY_ENDPOINT";

fn exchange<S: Read + Write>(
    stream: &mut S,
    head: &str,
    body: &[u8],
) -> Result<Vec<u8>, GatewayError> {
    stream
        .write_all(head.as_bytes())
        .and_then(|()| stream.write_all(body))
        .and_then(|()| stream.flush())
        .map_err(|error| GatewayError::Transport(format!("request failed: {error}")))?;
    let mut answer = Vec::new();
    Read::take(&mut *stream, ANSWER_LIMIT + 1)
        .read_to_end(&mut answer)
        .map_err(|error| GatewayError::Transport(format!("answer failed: {error}")))?;
    if u64::try_from(answer.len()).map_or(true, |length| length > ANSWER_LIMIT) {
        return Err(GatewayError::Transport(
            "answer exceeds its size limit".to_owned(),
        ));
    }
    Ok(answer)
}

/// Extracts the body of one HTTP/1.1 answer, failing closed on a refusal.
///
/// # Errors
/// Refuses an unframed answer, a non-200 status and a truncated body.
pub fn http_body(answer: &[u8]) -> Result<Vec<u8>, GatewayError> {
    let end = answer
        .windows(4)
        .position(|value| value == b"\r\n\r\n")
        .ok_or_else(|| GatewayError::Transport("answer has no headers".to_owned()))?;
    let head = std::str::from_utf8(answer.get(..end).unwrap_or_default())
        .map_err(|_| GatewayError::Transport("answer headers are not UTF-8".to_owned()))?;
    let body = answer.get(end + 4..).unwrap_or_default();
    let status = head
        .lines()
        .next()
        .and_then(|line| line.split_ascii_whitespace().nth(1))
        .and_then(|code| code.parse::<u16>().ok())
        .ok_or_else(|| GatewayError::Transport("answer has no status".to_owned()))?;
    if status != 200 {
        return Err(GatewayError::Transport(format!(
            "gateway answered HTTP {status}"
        )));
    }
    Ok(body.to_vec())
}

/// Renders one positional-parameter JSON-RPC request.
#[must_use]
pub fn rpc_request(id: u64, method: &str, params: &[Value]) -> String {
    serde_json::json!({
        "jsonrpc": "2.0",
        "id": id,
        "method": method,
        "params": params,
    })
    .to_string()
}

/// Interprets one JSON-RPC answer for the request `id`, failing closed.
///
/// # Errors
/// Refuses a malformed envelope, an answer for another request, and returns
/// the gateway's own refusal as [`GatewayError::Refused`].
pub fn rpc_result(id: u64, answer: &[u8]) -> Result<Value, GatewayError> {
    let document: Value =
        serde_json::from_slice(answer).map_err(|_| GatewayError::MalformedAnswer)?;
    if document["jsonrpc"] != Value::String("2.0".to_owned()) {
        return Err(GatewayError::MalformedAnswer);
    }
    if document["id"].as_u64() != Some(id) {
        return Err(GatewayError::Unbound);
    }
    if let Some(error) = document.get("error").filter(|value| !value.is_null()) {
        return Err(GatewayError::Refused {
            code: error["code"].as_i64().unwrap_or(0),
            message: error["message"]
                .as_str()
                .unwrap_or("unspecified")
                .to_owned(),
        });
    }
    document
        .get("result")
        .filter(|value| !value.is_null())
        .cloned()
        .ok_or(GatewayError::MalformedAnswer)
}

/// Reads an optional bounded text field.
///
/// # Errors
/// Refuses a non-text value and text beyond `limit`.
pub fn optional_text(value: &Value, limit: usize) -> Result<Option<String>, GatewayError> {
    match value {
        Value::Null => Ok(None),
        Value::String(text) if !text.is_empty() && text.len() <= limit => Ok(Some(text.clone())),
        _ => Err(GatewayError::MalformedAnswer),
    }
}

/// Reads a required bounded text field.
///
/// # Errors
/// Refuses an absent, empty, non-text or over-long value.
pub fn required_text(value: &Value, limit: usize) -> Result<String, GatewayError> {
    optional_text(value, limit)?.ok_or(GatewayError::MalformedAnswer)
}

/// Reads an optional `0x`-prefixed twenty-byte address.
///
/// # Errors
/// Refuses every other spelling.
pub fn optional_address(value: &Value) -> Result<Option<[u8; 20]>, GatewayError> {
    match value {
        Value::Null => Ok(None),
        Value::String(text) if text.is_empty() => Ok(None),
        Value::String(text) => address(text).map(Some),
        _ => Err(GatewayError::MalformedAnswer),
    }
}

/// Reads a `0x`-prefixed twenty-byte address.
///
/// # Errors
/// Refuses every other spelling.
pub fn address(text: &str) -> Result<[u8; 20], GatewayError> {
    let lowered = text.trim().to_ascii_lowercase();
    let body = lowered
        .strip_prefix("0x")
        .ok_or(GatewayError::MalformedAnswer)?;
    decode_evm(body).map_err(|_| GatewayError::MalformedAnswer)
}

/// Reads an optional thirty-two byte digest in any of its public spellings.
///
/// # Errors
/// Refuses every other spelling.
pub fn optional_digest(value: &Value) -> Result<Option<[u8; 32]>, GatewayError> {
    match value {
        Value::Null => Ok(None),
        Value::String(text) if text.is_empty() => Ok(None),
        Value::String(text) => digest(text).map(Some),
        _ => Err(GatewayError::MalformedAnswer),
    }
}

/// Reads a thirty-two byte digest in any of its public spellings.
///
/// # Errors
/// Refuses every other spelling.
pub fn digest(text: &str) -> Result<[u8; 32], GatewayError> {
    let lowered = text.trim().to_ascii_lowercase();
    let body = lowered
        .strip_prefix(DID_PREFIX)
        .or_else(|| lowered.strip_prefix("0x"))
        .unwrap_or(&lowered);
    decode_digest(body).map_err(|_| GatewayError::MalformedAnswer)
}

/// Reads a boolean field.
///
/// # Errors
/// Refuses every other value.
pub fn boolean(value: &Value) -> Result<bool, GatewayError> {
    value.as_bool().ok_or(GatewayError::MalformedAnswer)
}

/// Decodes an unsigned decimal string, a `0x` quantity or a JSON integer
/// without narrowing an on-chain amount.
///
/// # Errors
/// Refuses malformed quantities and values beyond 256 bits. JSON numbers
/// must be exact unsigned integers representable by the JSON decoder.
pub fn quantity(value: &Value) -> Result<Quantity, GatewayError> {
    match value {
        Value::Number(number) => number
            .as_u64()
            .map(|value| Quantity::from(u128::from(value)))
            .ok_or(GatewayError::MalformedAnswer),
        Value::String(text) => {
            let trimmed = text.trim();
            let (digits, radix) = if let Some(body) = trimmed
                .strip_prefix("0x")
                .or_else(|| trimmed.strip_prefix("0X"))
            {
                if body.len() > 64 {
                    return Err(GatewayError::MalformedAnswer);
                }
                (body, 16_u16)
            } else {
                (trimmed, 10_u16)
            };
            if digits.is_empty() {
                return Err(GatewayError::MalformedAnswer);
            }
            let mut bytes = [0_u8; 32];
            for byte in digits.bytes() {
                let digit = nibble(byte).ok_or(GatewayError::MalformedAnswer)?;
                if u16::from(digit) >= radix {
                    return Err(GatewayError::MalformedAnswer);
                }
                let mut carry = u16::from(digit);
                for byte in bytes.iter_mut().rev() {
                    let current = u16::from(*byte) * radix + carry;
                    *byte = (current & 0xff) as u8;
                    carry = current >> 8;
                }
                if carry != 0 {
                    return Err(GatewayError::MalformedAnswer);
                }
            }
            Ok(Quantity(bytes))
        }
        _ => Err(GatewayError::MalformedAnswer),
    }
}

/// Reads an optional gateway quantity.
///
/// # Errors
/// Refuses every spelling [`quantity`] refuses.
pub fn optional_quantity(value: &Value) -> Result<Option<Quantity>, GatewayError> {
    if value.is_null() {
        return Ok(None);
    }
    quantity(value).map(Some)
}

/// Reads a gateway quantity that must fit in sixty-four bits.
///
/// # Errors
/// Refuses every spelling [`quantity`] refuses and any value beyond 64 bits.
pub fn counted(value: &Value) -> Result<u64, GatewayError> {
    quantity(value)?.counted()
}

/// Decodes the `px_resolveAccount` result.
///
/// # Errors
/// Refuses a document that is not the declared shape.
pub fn decode_identities(result: &Value) -> Result<ResolvedIdentities, GatewayError> {
    if !result.is_object() {
        return Err(GatewayError::MalformedAnswer);
    }
    Ok(ResolvedIdentities {
        lookup_selector: optional_digest(&result["layerx_did"])?
            .map(LookupSelector::Did)
            .or(optional_address(&result["evm_address"])?.map(LookupSelector::Evm)),
        evm_address: optional_address(&result["evm_address"])?,
        pax_address: optional_text(&result["pax_address"], PAX_ADDRESS_LIMIT)?,
        layerx_did: optional_digest(&result["layerx_did"])?,
        layerx_account: optional_digest(&result["layerx_account"])?,
        bound: boolean(&result["bound"])?,
        evidence: Evidence::GatewayReported,
    })
}

/// Decodes the `px_getAccount` result.
///
/// # Errors
/// Refuses a document that is not the declared shape.
pub fn decode_account(result: &Value) -> Result<AccountJoin, GatewayError> {
    if !result.is_object() {
        return Err(GatewayError::MalformedAnswer);
    }
    let account = decode_identities(&result["account"])?;
    let paxeer = match &result["paxeer"] {
        Value::Null => None,
        document if document.is_object() => Some(PaxeerAccount {
            address: address(
                document["address"]
                    .as_str()
                    .ok_or(GatewayError::MalformedAnswer)?,
            )?,
            balance: quantity(&document["balance"])?,
            nonce: counted(&document["nonce"])?,
            height: counted(&document["height"])?,
        }),
        _ => return Err(GatewayError::MalformedAnswer),
    };
    Ok(AccountJoin {
        account,
        paxeer,
        evidence: Evidence::GatewayReported,
    })
}

fn asset_metadata_text(value: &Value) -> Result<Option<String>, GatewayError> {
    match value {
        Value::Null => Ok(None),
        Value::String(text) => Ok(Some(text.clone())),
        _ => Err(GatewayError::MalformedAnswer),
    }
}

/// Decodes the `px_getBalances` result.
///
/// # Errors
/// Refuses a document that is not the declared shape or exceeds the joined
/// asset ceiling.
pub fn decode_account_balances(result: &Value) -> Result<AccountBalances, GatewayError> {
    let rows = result["balances"]
        .as_array()
        .ok_or(GatewayError::MalformedAnswer)?;
    if rows.len() > MAXIMUM_JOINED_ASSETS {
        return Err(GatewayError::MalformedAnswer);
    }
    let mut balances = Vec::with_capacity(rows.len());
    for row in rows {
        balances.push(AssetBalance {
            denom: required_text(&row["denom"], DENOM_LIMIT)?,
            amount: quantity(&row["amount"])?,
        });
    }
    let balances = AccountBalances {
        account: decode_identities(&result["account"])?,
        height: counted(&result["height"])?,
        balances,
        evidence: Evidence::GatewayReported,
    };
    balances.validate_denominations()?;
    Ok(balances)
}

/// Decodes the `px_listAssets` result.
///
/// # Errors
/// Refuses a document that is not the declared shape or exceeds the joined
/// asset ceiling.
pub fn decode_assets(result: &Value) -> Result<AssetTable, GatewayError> {
    let rows = result["assets"]
        .as_array()
        .ok_or(GatewayError::MalformedAnswer)?;
    if rows.len() > MAXIMUM_JOINED_ASSETS {
        return Err(GatewayError::MalformedAnswer);
    }
    let mut assets = Vec::with_capacity(rows.len());
    for row in rows {
        assets.push(AssetEntry {
            asset_id: row["asset_id"]
                .as_str()
                .filter(|text| !text.is_empty())
                .ok_or(GatewayError::MalformedAnswer)?
                .to_owned(),
            denom: optional_text(&row["denom"], DENOM_LIMIT)?,
            kind: row["kind"]
                .as_str()
                .filter(|text| !text.is_empty())
                .ok_or(GatewayError::MalformedAnswer)?
                .to_owned(),
            name: asset_metadata_text(&row["name"])?,
            symbol: asset_metadata_text(&row["symbol"])?,
            decimals: optional_quantity(&row["decimals"])?
                .map(|quantity| {
                    u8::try_from(quantity.counted()?).map_err(|_| GatewayError::MalformedAnswer)
                })
                .transpose()?,
            supply: optional_quantity(&row["supply"])?,
        });
    }
    Ok(AssetTable {
        height: counted(&result["height"])?,
        assets,
        joined_limit: counted(&result["joined_limit"])?,
        evidence: Evidence::GatewayReported,
    })
}

/// Decodes the `px_getNetwork` result.
///
/// # Errors
/// Refuses a document that is not the declared shape.
pub fn decode_network(result: &Value) -> Result<NetworkHead, GatewayError> {
    let paxeer = &result["paxeer"];
    let consensus = match result.get("consensus") {
        Some(Value::Null) if result["reason"] == "consensus_unavailable" => None,
        Some(head @ Value::Object(_)) => Some(ConsensusHead {
            latest_height: counted(&head["latest_height"])?,
            latest_block_hash: digest(
                head["latest_block_hash"]
                    .as_str()
                    .ok_or(GatewayError::MalformedAnswer)?,
            )?,
            latest_app_hash: digest(
                head["latest_app_hash"]
                    .as_str()
                    .ok_or(GatewayError::MalformedAnswer)?,
            )?,
            catching_up: boolean(&head["catching_up"])?,
        }),
        _ => return Err(GatewayError::MalformedAnswer),
    };
    Ok(NetworkHead {
        network_id: required_text(&result["network_id"], NETWORK_ID_LIMIT)?,
        chain_id: counted(&paxeer["chain_id"])?,
        latest_block: counted(&paxeer["latest_block"])?,
        consensus,
        evidence: Evidence::GatewayReported,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    use serde_json::json;

    #[test]
    fn endpoint_parsing_admits_only_http_and_https_authorities() {
        let endpoint = GatewayEndpoint::parse("https://gateway.example/rpc")
            .unwrap_or_else(|error| panic!("endpoint: {error}"));
        assert_eq!(endpoint.host, "gateway.example");
        assert_eq!(endpoint.port, 443);
        assert_eq!(endpoint.path, "/rpc");
        assert!(endpoint.secure);
        for refused in [
            "ftp://gateway.example",
            "https://",
            "https://gateway.example:0",
            "gateway.example",
        ] {
            assert_eq!(
                GatewayEndpoint::parse(refused),
                Err(GatewayError::InvalidEndpoint)
            );
        }
    }

    #[test]
    fn rpc_answers_bind_to_their_own_request_identifier() {
        let answer = br#"{"jsonrpc":"2.0","id":7,"result":{"bound":true}}"#;
        assert_eq!(rpc_result(7, answer), Ok(json!({"bound": true})));
        assert_eq!(rpc_result(8, answer), Err(GatewayError::Unbound));
        let refusal =
            br#"{"jsonrpc":"2.0","id":7,"error":{"code":-32001,"message":"unavailable"}}"#;
        assert_eq!(
            rpc_result(7, refusal),
            Err(GatewayError::Refused {
                code: -32001,
                message: "unavailable".to_owned()
            })
        );
    }

    #[test]
    fn balances_decode_the_declared_nested_join_shape() {
        let result = json!({
            "account": {
                "evm_address": "0x1111111111111111111111111111111111111111",
                "pax_address": "pax1example",
                "layerx_did": format!("did:layerx:{}", "22".repeat(32)),
                "layerx_account": "33".repeat(32),
                "bound": true
            },
            "height": 16,
            "balances": [{"denom": "upax", "amount": "250"}]
        });
        let balances =
            decode_account_balances(&result).unwrap_or_else(|error| panic!("balances: {error}"));
        assert_eq!(balances.height, 16);
        assert!(balances.account.bound);
        assert_eq!(balances.balances.len(), 1);
        let row = balances
            .asset("upax")
            .unwrap_or_else(|| panic!("asset row"));
        assert_eq!(row.denom, "upax");
        assert_eq!(row.amount, Quantity::from(250));
        assert_eq!(balances.evidence, Evidence::GatewayReported);
    }

    #[test]
    fn an_unavailable_half_decodes_as_absent_and_never_as_zero() {
        let result = json!({
            "account": {
                "evm_address": Value::Null,
                "pax_address": Value::Null,
                "layerx_did": Value::Null,
                "layerx_account": Value::Null,
                "bound": false
            },
            "paxeer": Value::Null,
            "layerx": Value::Null
        });
        let join = decode_account(&result).unwrap_or_else(|error| panic!("account: {error}"));
        assert!(join.paxeer.is_none());
        assert_eq!(join.account.evm_address, None);
        assert!(!join.account.bound);
        assert_eq!(join.evidence, Evidence::GatewayReported);
    }

    #[test]
    fn network_head_decodes_every_rung_the_gateway_reports() {
        let result = json!({
            "network_id": "paxeer-x",
            "paxeer": {"chain_id": "0x1f", "latest_block": 4_096},
            "consensus": {
                "latest_height": "4096",
                "latest_block_hash": "A1".repeat(32),
                "latest_app_hash": "B2".repeat(32),
                "catching_up": false
            }
        });
        let head = decode_network(&result).unwrap_or_else(|error| panic!("network: {error}"));
        assert_eq!(head.network_id, "paxeer-x");
        assert_eq!(head.chain_id, 31);
        assert_eq!(head.latest_block, 4_096);
        assert_eq!(
            head.consensus,
            Some(ConsensusHead {
                latest_height: 4_096,
                latest_block_hash: [0xa1; 32],
                latest_app_hash: [0xb2; 32],
                catching_up: false,
            })
        );
    }

    #[test]
    fn a_malformed_quantity_is_refused_rather_than_defaulted() {
        assert_eq!(quantity(&json!("12")), Ok(Quantity::from(12)));
        assert_eq!(quantity(&json!("0x0c")), Ok(Quantity::from(12)));
        assert_eq!(quantity(&json!(12)), Ok(Quantity::from(12)));
        for refused in [json!("-1"), json!("0x"), json!("zz"), json!(true)] {
            assert_eq!(quantity(&refused), Err(GatewayError::MalformedAnswer));
        }
    }
}

#[cfg(test)]
mod identity_selector_contract {
    use super::*;
    #[test]
    fn native_account_never_becomes_a_did_selector() {
        let bytes = [0x61; 32];
        assert!(LookupSelector::try_from(AccountIdentifier::Account(bytes)).is_err());
        assert_eq!(
            LookupSelector::try_from(AccountIdentifier::Did(bytes)),
            Ok(LookupSelector::Did(bytes))
        );
        let endpoint = GatewayEndpoint::parse("http://127.0.0.1:1").unwrap();
        for result in [
            endpoint
                .resolve_account(AccountIdentifier::Account(bytes))
                .map(|_| ()),
            endpoint
                .get_account(AccountIdentifier::Account(bytes))
                .map(|_| ()),
            endpoint
                .get_balances(AccountIdentifier::Account(bytes))
                .map(|_| ()),
        ] {
            assert_eq!(
                result,
                Err(GatewayError::Refused {
                    code: -32001,
                    message: "native_account_reverse_resolution_unsupported".to_owned()
                })
            );
        }
    }

    #[test]
    fn real_repeated_account_and_balance_lookup_preserves_owner() {
        let path = std::env::var("PAXEER_X_IDENTITY_FIXTURE")
            .expect("real isolated identity fixture required");
        let fixture: Value = serde_json::from_slice(&std::fs::read(path).expect("fixture file"))
            .expect("fixture JSON");
        let endpoint = GatewayEndpoint::parse(
            fixture["targets"]["healthy"]["url"]
                .as_str()
                .expect("endpoint"),
        )
        .expect("endpoint parse");
        for field in ["evm", "did"] {
            let requested =
                AccountIdentifier::parse(fixture[field].as_str().expect("real selector"))
                    .expect("selector parse");
            let mut identities = endpoint
                .resolve_account(requested)
                .expect("real resolution");
            assert_eq!(
                identities.layerx_account,
                Some(
                    decode_digest(fixture["native_account"].as_str().expect("native account"))
                        .expect("native digest")
                )
            );
            let lookup = identities
                .retain_lookup(requested)
                .expect("original lookup");
            assert_eq!(lookup.identifier(), requested);
            assert_eq!(identities.lookup_selector, Some(lookup));
            assert_eq!(
                identities.canonical(requested),
                AccountIdentifier::Account(
                    identities.native_reference().expect("native reference").0
                )
            );
            for _ in 0..2 {
                assert_eq!(
                    endpoint
                        .resolve_account(lookup.identifier())
                        .expect("repeat resolution"),
                    identities
                );
                assert_eq!(
                    endpoint
                        .get_account(lookup.identifier())
                        .expect("repeat account")
                        .account,
                    identities
                );
                assert_eq!(
                    endpoint
                        .get_balances(lookup.identifier())
                        .expect("repeat balance")
                        .account,
                    identities
                );
            }
            assert_eq!(
                identities.retain_lookup(AccountIdentifier::Did(
                    identities.native_reference().expect("native reference").0
                )),
                Err(GatewayError::Unbound)
            );
        }
    }
}

#[cfg(test)]
mod identity_selector_contract_bound_shape {
    use super::*;

    #[test]
    fn retaining_lookup_refuses_incomplete_or_conflicting_joins() {
        let mut identities = ResolvedIdentities {
            lookup_selector: None,
            evm_address: Some([9; 20]),
            pax_address: None,
            layerx_did: Some([7; 32]),
            layerx_account: Some([8; 32]),
            bound: true,
            evidence: Evidence::GatewayReported,
        };
        let requested = AccountIdentifier::Evm([9; 20]);
        assert_eq!(
            identities.retain_lookup(requested),
            Ok(LookupSelector::Evm([9; 20]))
        );
        let preserved = identities.clone();
        identities.layerx_account = None;
        assert_eq!(
            identities.retain_lookup(requested),
            Err(GatewayError::Unbound)
        );
        identities = preserved.clone();
        identities.bound = false;
        assert_eq!(
            identities.retain_lookup(requested),
            Err(GatewayError::Unbound)
        );
        identities = preserved.clone();
        identities.evm_address = None;
        assert_eq!(
            identities.retain_lookup(AccountIdentifier::Did([7; 32])),
            Err(GatewayError::Unbound)
        );
        identities = preserved;
        assert_eq!(
            identities.retain_lookup(AccountIdentifier::Did([8; 32])),
            Err(GatewayError::Unbound)
        );
        assert_eq!(
            identities.lookup_selector,
            Some(LookupSelector::Evm([9; 20]))
        );
        assert_eq!(
            identities.native_reference(),
            Some(NativeAccountReference([8; 32]))
        );
    }
}
