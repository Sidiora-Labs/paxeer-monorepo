use layerx_platform_internal::secret::{hex as lowercase_hex, unhex};
use layerx_platform_internal::tls::Upstream;
use serde_json::{json, Value};

pub const CHAIN_ID: u64 = 125;
pub type Address = [u8; 20];
pub type Word = [u8; 32];

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Call {
    pub to: Address,
    pub value: Word,
    pub data: Vec<u8>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct QuoteRequest {
    pub account: Address,
    pub nonce: Word,
    pub calls: Vec<Call>,
    pub max_token_amount: u128,
    pub gas_cost: u128,
    pub token: Address,
    pub decimals: u8,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Quote {
    pub sponsor: Address,
    pub token: Address,
    pub max_token_amount: Word,
    pub token_amount: Word,
    pub deadline: Word,
    pub nonce: Word,
    pub gas_cost: Word,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SignedQuote {
    pub quote: Quote,
    pub signature: [u8; 65],
    pub decimals: u8,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum GasStationError {
    Configuration,
    Transport,
    Http(u16),
    Malformed,
    ChainMismatch,
}

impl std::fmt::Display for GasStationError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Configuration => formatter.write_str("invalid gas station configuration"),
            Self::Transport => formatter.write_str("gas station transport failed"),
            Self::Http(status) => write!(formatter, "gas station returned HTTP {status}"),
            Self::Malformed => formatter.write_str("malformed gas station message"),
            Self::ChainMismatch => formatter.write_str("gas station chain must be 125"),
        }
    }
}

impl std::error::Error for GasStationError {}

pub struct GasStationClient {
    upstream: Upstream,
}

impl GasStationClient {
    #[must_use]
    pub fn new(upstream: Upstream) -> Self {
        Self { upstream }
    }

    pub fn from_environment(prefix: &str) -> Result<Self, GasStationError> {
        Upstream::from_environment(prefix)
            .map(Self::new)
            .map_err(|_| GasStationError::Configuration)
    }

    pub fn quote_body(request: &QuoteRequest) -> Result<Value, GasStationError> {
        if request.max_token_amount == 0
            || request.calls.len() > 256
            || request.calls.iter().any(|call| call.data.len() > 65_536)
        {
            return Err(GasStationError::Malformed);
        }
        let calls: Vec<Value> = request
            .calls
            .iter()
            .map(|call| {
                json!({
                    "to": hex(&call.to),
                    "value": decimal(&call.value),
                    "data": hex(&call.data),
                })
            })
            .collect();
        Ok(json!({
            "account": hex(&request.account),
            "nonce": decimal(&request.nonce),
            "calls": calls,
            "maxTokenAmount": request.max_token_amount.to_string(),
            "gasCost": request.gas_cost.to_string(),
            "chainId": CHAIN_ID.to_string(),
            "token": hex(&request.token),
            "decimals": request.decimals,
        }))
    }

    pub fn quote(&self, request: &QuoteRequest) -> Result<SignedQuote, GasStationError> {
        let response = self.post("/quote", &Self::quote_body(request)?)?;
        let signed = Self::decode_quote(&response)?;
        if signed.quote.token != request.token
            || signed.decimals != request.decimals
            || signed.quote.max_token_amount != word(request.max_token_amount)
            || signed.quote.gas_cost != word(request.gas_cost)
        {
            return Err(GasStationError::Malformed);
        }
        Ok(signed)
    }

    pub fn decode_quote(answer: &Value) -> Result<SignedQuote, GasStationError> {
        let envelope = answer.as_object().ok_or(GasStationError::Malformed)?;
        let quote = envelope
            .get("quote")
            .and_then(Value::as_object)
            .ok_or(GasStationError::Malformed)?;
        // The station omits chainId; validate it wherever an answer supplies it.
        for object in [envelope, quote] {
            if let Some(chain) = object.get("chainId") {
                check_chain(chain)?;
            }
        }
        let field = |name: &str| quote.get(name).ok_or(GasStationError::Malformed);
        let decoded = Quote {
            sponsor: fixed_hex(field("sponsor")?)?,
            token: fixed_hex(field("token")?)?,
            max_token_amount: decimal_word(field("maxTokenAmount")?)?,
            token_amount: decimal_word(field("tokenAmount")?)?,
            deadline: decimal_word(field("deadline")?)?,
            nonce: decimal_word(field("quoteNonce")?)?,
            gas_cost: decimal_word(field("gasCost")?)?,
        };
        if decoded.max_token_amount == [0; 32] || decoded.token_amount > decoded.max_token_amount {
            return Err(GasStationError::Malformed);
        }
        Ok(SignedQuote {
            quote: decoded,
            signature: fixed_hex(
                envelope
                    .get("relayerSignature")
                    .ok_or(GasStationError::Malformed)?,
            )?,
            decimals: field("decimals")?
                .as_u64()
                .and_then(|value| u8::try_from(value).ok())
                .ok_or(GasStationError::Malformed)?,
        })
    }

    pub fn submit(&self, body: &Value) -> Result<Value, GasStationError> {
        check_chain(
            body.get("batch")
                .and_then(|batch| batch.get("chainId"))
                .ok_or(GasStationError::Malformed)?,
        )?;
        self.post("/submit", body)
    }

    pub fn status(&self, body: &Value) -> Result<Value, GasStationError> {
        self.post("/status", body)
    }

    fn post(&self, path: &str, body: &Value) -> Result<Value, GasStationError> {
        let encoded = serde_json::to_vec(body).map_err(|_| GasStationError::Malformed)?;
        let response = self
            .upstream
            .post(path, &encoded)
            .map_err(|_| GasStationError::Transport)?;
        if !(200..300).contains(&response.status) {
            return Err(GasStationError::Http(response.status));
        }
        if response.content_type.split(';').next().map(str::trim) != Some("application/json") {
            return Err(GasStationError::Malformed);
        }
        let answer: Value =
            serde_json::from_slice(&response.body).map_err(|_| GasStationError::Malformed)?;
        if !answer.is_object() {
            return Err(GasStationError::Malformed);
        }
        Ok(answer)
    }
}

fn check_chain(value: &Value) -> Result<(), GasStationError> {
    match (value.as_str(), value.as_u64()) {
        (Some("125"), _) | (_, Some(CHAIN_ID)) => Ok(()),
        _ => Err(GasStationError::ChainMismatch),
    }
}

fn fixed_hex<const N: usize>(value: &Value) -> Result<[u8; N], GasStationError> {
    let raw = value.as_str().ok_or(GasStationError::Malformed)?;
    if raw.len() != 2 + N * 2 || !raw.starts_with("0x") {
        return Err(GasStationError::Malformed);
    }
    unhex(&raw[2..])
        .and_then(|bytes| bytes.try_into().ok())
        .ok_or(GasStationError::Malformed)
}

fn hex(bytes: &[u8]) -> String {
    format!("0x{}", lowercase_hex(bytes))
}

fn word(value: u128) -> Word {
    let mut result = [0; 32];
    result[16..].copy_from_slice(&value.to_be_bytes());
    result
}

fn decimal_word(value: &Value) -> Result<Word, GasStationError> {
    let raw = value.as_str().ok_or(GasStationError::Malformed)?;
    if raw.is_empty()
        || raw.len() > 78
        || (raw.len() > 1 && raw.starts_with('0'))
        || !raw.bytes().all(|byte| byte.is_ascii_digit())
    {
        return Err(GasStationError::Malformed);
    }
    let mut result = [0_u8; 32];
    for digit in raw.bytes() {
        let mut carry = u16::from(digit - b'0');
        for byte in result.iter_mut().rev() {
            let next = u16::from(*byte) * 10 + carry;
            *byte = next.to_be_bytes()[1];
            carry = next >> 8;
        }
        if carry != 0 {
            return Err(GasStationError::Malformed);
        }
    }
    Ok(result)
}

fn decimal(value: &Word) -> String {
    let mut remaining = *value;
    let mut digits = Vec::new();
    loop {
        let mut rest = 0_u16;
        for byte in &mut remaining {
            let current = (rest << 8) | u16::from(*byte);
            *byte = (current / 10).to_be_bytes()[1];
            rest = current % 10;
        }
        digits.push(char::from(b'0' + rest.to_be_bytes()[1]));
        if remaining == [0; 32] {
            break;
        }
    }
    digits.iter().rev().collect()
}
