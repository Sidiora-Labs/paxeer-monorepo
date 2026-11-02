//! The EVM half of the unified network endpoint: which Paxeer JSON-RPC
//! methods the public endpoint relays, and the ABI of the native precompiles
//! the `px_*` namespace reads.
//!
//! Every selector and layout here is the one declared by the precompiles'
//! own `abi.json`: addr, bank, exchange, bridge, launchpad, and the merged
//! ledger, pay, auth and programs constructs. Addresses and selectors are
//! pinned by the ABI fixture; calldata and answer vectors use the go-ethereum
//! ABI codec the precompiles run.

/// Method namespaces the unified endpoint relays to the Paxeer node.
pub const EVM_NAMESPACES: [&str; 3] = ["eth_", "net_", "web3_"];

/// Node-side signing and account methods the public endpoint never relays.
/// A hosted node holds no caller keys, so answering these would either lie or
/// expose the operator's accounts.
pub const NODE_SIGNING_METHODS: [&str; 8] = [
    "eth_accounts",
    "eth_coinbase",
    "eth_sendTransaction",
    "eth_sign",
    "eth_signTransaction",
    "eth_signTypedData",
    "eth_signTypedData_v4",
    "eth_mining",
];

/// Subscription methods that need a live upstream socket rather than the
/// request/response relay.
pub const SUBSCRIPTION_METHODS: [&str; 2] = ["eth_subscribe", "eth_unsubscribe"];

pub const SEQUENCER_METHODS: [&str; 1] = ["pax_sequencerReceipt"];

const MAX_METHOD_BYTES: usize = 64;

/// `0x0000000000000000000000000000000000001001`.
pub const BANK_PRECOMPILE: [u8; 20] = precompile(0x10, 0x01);
/// `0x0000000000000000000000000000000000001004`.
pub const ADDR_PRECOMPILE: [u8; 20] = precompile(0x10, 0x04);
/// `0x0000000000000000000000000000000000001015`, live from the Paxeer X fork.
pub const EXCHANGE_PRECOMPILE: [u8; 20] = precompile(0x10, 0x15);
/// `0x0000000000000000000000000000000000001016`, live from the Paxeer X fork.
pub const BRIDGE_PRECOMPILE: [u8; 20] = precompile(0x10, 0x16);
/// `0x0000000000000000000000000000000000001017`, live from the Paxeer X fork.
pub const LAUNCHPAD_PRECOMPILE: [u8; 20] = precompile(0x10, 0x17);
/// `0x000000000000000000000000000000000000101a`.
pub const LEDGER_PRECOMPILE: [u8; 20] = precompile(0x10, 0x1a);
/// Ledger `getAsset(bytes32)`.
pub const SELECTOR_GET_ASSET: [u8; 4] = [0x2c, 0xc3, 0xce, 0x80];
/// `0x000000000000000000000000000000000000101b`.
pub const PAY_PRECOMPILE: [u8; 20] = precompile(0x10, 0x1b);
/// `0x000000000000000000000000000000000000101c`.
pub const AUTH_PRECOMPILE: [u8; 20] = precompile(0x10, 0x1c);
/// `0x000000000000000000000000000000000000101d`.
pub const PROGRAMS_PRECOMPILE: [u8; 20] = precompile(0x10, 0x1d);

const fn precompile(high: u8, low: u8) -> [u8; 20] {
    let mut address = [0_u8; 20];
    address[18] = high;
    address[19] = low;
    address
}

/// `getUnifiedAccount(address)`.
pub const SELECTOR_GET_UNIFIED_ACCOUNT: [u8; 4] = [0x35, 0x7f, 0xee, 0xd6];
/// `getLayerXDid(address)`.
pub const SELECTOR_GET_LAYERX_DID: [u8; 4] = [0xd3, 0x60, 0x0b, 0x24];
/// `getEvmAddrByLayerX(bytes32)`.
pub const SELECTOR_GET_EVM_ADDR_BY_LAYERX: [u8; 4] = [0xc4, 0x13, 0xba, 0xd3];
/// `layerXBindNonce(address)`.
pub const SELECTOR_LAYERX_BIND_NONCE: [u8; 4] = [0xce, 0xdd, 0x9b, 0xa2];
/// `balance(address,string)`.
pub const SELECTOR_BANK_BALANCE: [u8; 4] = [0x16, 0xca, 0xde, 0xab];
/// `all_balances(address)`.
pub const SELECTOR_BANK_ALL_BALANCES: [u8; 4] = [0x0c, 0x5e, 0x27, 0x89];
pub const SELECTOR_BANK_NAME: [u8; 4] = [0x5b, 0x43, 0xbc, 0x99];
pub const SELECTOR_BANK_SYMBOL: [u8; 4] = [0x41, 0xbb, 0x05, 0x59];
pub const SELECTOR_BANK_DECIMALS: [u8; 4] = [0x3b, 0x2b, 0x32, 0x04];
pub const SELECTOR_BANK_SUPPLY: [u8; 4] = [0x69, 0x23, 0xa1, 0xfd];

/// Largest ABI string the endpoint decodes from a precompile answer.
pub const MAX_ABI_STRING_BYTES: usize = 4096;

/// True when the method belongs to a relayed EVM namespace, whether or not
/// the endpoint is willing to relay it.
#[must_use]
pub fn is_evm_namespace(method: &str) -> bool {
    EVM_NAMESPACES
        .iter()
        .any(|namespace| method.starts_with(namespace))
}

/// True when the unified endpoint forwards the method to the Paxeer node.
/// Node-side signing methods and methods outside the wire charset are refused
/// here rather than at the node.
#[must_use]
pub fn relayable_method(method: &str) -> bool {
    if method.len() > MAX_METHOD_BYTES
        || !method
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || byte == b'_')
        || NODE_SIGNING_METHODS.contains(&method)
        || SUBSCRIPTION_METHODS.contains(&method)
    {
        return false;
    }
    is_evm_namespace(method) || SEQUENCER_METHODS.contains(&method)
}

/// Why precompile bytes were refused.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum AbiError {
    /// The answer is shorter than the declared layout.
    Truncated,
    /// A word carries bits the declared type cannot hold.
    Width,
    /// A dynamic offset points outside the answer.
    Offset,
    /// A string is not valid UTF-8 or exceeds its bound.
    Text,
}

/// `IAddr.getUnifiedAccount`.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct UnifiedAccount {
    pub evm: [u8; 20],
    pub pax_address: String,
    pub did_public_key: [u8; 32],
    pub layerx_main_account_id: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LedgerAsset {
    pub asset_id: [u8; 32],
    pub symbol: String,
    pub name: String,
    pub decimals: u8,
    pub supply_cap: u128,
    pub issuer_did: [u8; 32],
    pub salt: [u8; 32],
    pub issuer_kind: u8,
    pub custody_kind: u8,
    pub custody_reference: Vec<u8>,
    pub paused: bool,
    pub total_units: u128,
}

fn calldata_head(selector: [u8; 4], words: usize) -> Vec<u8> {
    let mut calldata = Vec::with_capacity(4 + words * 32);
    calldata.extend_from_slice(&selector);
    calldata
}

/// `selector()` with no arguments.
#[must_use]
pub fn calldata_empty(selector: [u8; 4]) -> Vec<u8> {
    calldata_head(selector, 0)
}

/// `selector(address)`.
#[must_use]
pub fn calldata_address(selector: [u8; 4], address: &[u8; 20]) -> Vec<u8> {
    let mut calldata = calldata_head(selector, 1);
    calldata.extend_from_slice(&[0_u8; 12]);
    calldata.extend_from_slice(address);
    calldata
}

/// `selector(bytes32)`.
#[must_use]
pub fn calldata_word(selector: [u8; 4], word: &[u8; 32]) -> Vec<u8> {
    let mut calldata = calldata_head(selector, 1);
    calldata.extend_from_slice(word);
    calldata
}

/// `selector(uint64)`.
#[must_use]
pub fn calldata_u64(selector: [u8; 4], value: u64) -> Vec<u8> {
    let mut calldata = calldata_head(selector, 1);
    calldata.extend_from_slice(&[0_u8; 24]);
    calldata.extend_from_slice(&value.to_be_bytes());
    calldata
}

#[must_use]
pub fn calldata_string(selector: [u8; 4], text: &str) -> Vec<u8> {
    let mut calldata = calldata_head(selector, 1);
    calldata.extend_from_slice(&word_u64(32));
    calldata.extend_from_slice(&word_u64(u64::try_from(text.len()).unwrap_or(u64::MAX)));
    calldata.extend_from_slice(text.as_bytes());
    calldata.resize(4 + 64 + text.len().div_ceil(32) * 32, 0);
    calldata
}

/// `selector(address,string)`.
#[must_use]
pub fn calldata_address_string(selector: [u8; 4], address: &[u8; 20], text: &str) -> Vec<u8> {
    let mut calldata = calldata_head(selector, 2);
    calldata.extend_from_slice(&[0_u8; 12]);
    calldata.extend_from_slice(address);
    calldata.extend_from_slice(&word_u64(64));
    calldata.extend_from_slice(&word_u64(u64::try_from(text.len()).unwrap_or(u64::MAX)));
    calldata.extend_from_slice(text.as_bytes());
    let remainder = text.len() % 32;
    if remainder != 0 {
        calldata.extend(std::iter::repeat_n(0_u8, 32 - remainder));
    }
    calldata
}

fn word_u64(value: u64) -> [u8; 32] {
    let mut word = [0_u8; 32];
    word[24..].copy_from_slice(&value.to_be_bytes());
    word
}

/// One ABI answer read word by word.
pub struct Answer<'a> {
    bytes: &'a [u8],
}

impl<'a> Answer<'a> {
    #[must_use]
    pub const fn new(bytes: &'a [u8]) -> Self {
        Self { bytes }
    }

    /// # Errors
    /// Refuses an answer shorter than the requested word.
    pub fn word(&self, index: usize) -> Result<[u8; 32], AbiError> {
        let start = index.checked_mul(32).ok_or(AbiError::Offset)?;
        let end = start.checked_add(32).ok_or(AbiError::Offset)?;
        let mut word = [0_u8; 32];
        word.copy_from_slice(self.bytes.get(start..end).ok_or(AbiError::Truncated)?);
        Ok(word)
    }

    /// # Errors
    /// Refuses a word whose high bytes are set.
    pub fn address(&self, index: usize) -> Result<[u8; 20], AbiError> {
        let word = self.word(index)?;
        if word[..12].iter().any(|byte| *byte != 0) {
            return Err(AbiError::Width);
        }
        let mut address = [0_u8; 20];
        address.copy_from_slice(&word[12..]);
        Ok(address)
    }

    /// # Errors
    /// Refuses a word wider than 64 bits.
    pub fn u64(&self, index: usize) -> Result<u64, AbiError> {
        let word = self.word(index)?;
        if word[..24].iter().any(|byte| *byte != 0) {
            return Err(AbiError::Width);
        }
        let mut value = [0_u8; 8];
        value.copy_from_slice(&word[24..]);
        Ok(u64::from_be_bytes(value))
    }

    pub fn u128(&self, index: usize) -> Result<u128, AbiError> {
        let word = self.word(index)?;
        if word[..16].iter().any(|byte| *byte != 0) {
            return Err(AbiError::Width);
        }
        let mut value = [0_u8; 16];
        value.copy_from_slice(&word[16..]);
        Ok(u128::from_be_bytes(value))
    }

    /// # Errors
    /// Refuses a word wider than 32 bits.
    pub fn u32(&self, index: usize) -> Result<u32, AbiError> {
        u32::try_from(self.u64(index)?).map_err(|_| AbiError::Width)
    }

    /// # Errors
    /// Refuses a word wider than 8 bits.
    pub fn u8(&self, index: usize) -> Result<u8, AbiError> {
        u8::try_from(self.u64(index)?).map_err(|_| AbiError::Width)
    }

    /// # Errors
    /// Refuses a word that is neither zero nor one.
    pub fn bool(&self, index: usize) -> Result<bool, AbiError> {
        match self.u64(index)? {
            0 => Ok(false),
            1 => Ok(true),
            _ => Err(AbiError::Width),
        }
    }

    /// Reads a dynamic string whose offset word sits at `index` and whose
    /// offset is measured from `base` bytes into the answer.
    ///
    /// # Errors
    /// Refuses offsets outside the answer and text that is not UTF-8 or
    /// exceeds [`MAX_ABI_STRING_BYTES`].
    pub fn string(&self, index: usize, base: usize) -> Result<String, AbiError> {
        String::from_utf8(self.bytes(index, base)?).map_err(|_| AbiError::Text)
    }

    pub fn bytes(&self, index: usize, base: usize) -> Result<Vec<u8>, AbiError> {
        let offset = usize::try_from(self.u64(index)?).map_err(|_| AbiError::Offset)?;
        let start = base.checked_add(offset).ok_or(AbiError::Offset)?;
        if start % 32 != 0 {
            return Err(AbiError::Offset);
        }
        let length =
            usize::try_from(Self::new(self.bytes.get(start..).ok_or(AbiError::Offset)?).u64(0)?)
                .map_err(|_| AbiError::Offset)?;
        if length > MAX_ABI_STRING_BYTES {
            return Err(AbiError::Text);
        }
        let text_start = start.checked_add(32).ok_or(AbiError::Offset)?;
        let text_end = text_start.checked_add(length).ok_or(AbiError::Offset)?;
        let padded = text_end
            .checked_add((32 - length % 32) % 32)
            .ok_or(AbiError::Offset)?;
        if self.bytes.len() < padded {
            return Err(AbiError::Truncated);
        }
        let text = self
            .bytes
            .get(text_start..text_end)
            .ok_or(AbiError::Truncated)?;
        Ok(text.to_vec())
    }
}

/// Decodes `getUnifiedAccount(address)`.
///
/// # Errors
/// Refuses answers that do not carry the declared layout.
pub fn decode_unified_account(answer: &[u8]) -> Result<UnifiedAccount, AbiError> {
    let answer = Answer::new(answer);
    Ok(UnifiedAccount {
        evm: answer.address(0)?,
        pax_address: answer.string(1, 0)?,
        did_public_key: answer.word(2)?,
        layerx_main_account_id: answer.word(3)?,
    })
}

/// Decodes `getLayerXDid(address)` into the DID public key and rendered DID.
///
/// # Errors
/// Refuses answers that do not carry the declared layout.
pub fn decode_layerx_did(answer: &[u8]) -> Result<([u8; 32], String), AbiError> {
    let answer = Answer::new(answer);
    Ok((answer.word(0)?, answer.string(1, 0)?))
}

/// Decodes one `address` answer.
///
/// # Errors
/// Refuses answers that do not carry the declared layout.
pub fn decode_address(answer: &[u8]) -> Result<[u8; 20], AbiError> {
    Answer::new(answer).address(0)
}

/// Decodes one `bytes32` answer.
///
/// # Errors
/// Refuses answers that do not carry the declared layout.
pub fn decode_word(answer: &[u8]) -> Result<[u8; 32], AbiError> {
    Answer::new(answer).word(0)
}

/// Decodes one `uint64` answer.
///
/// # Errors
/// Refuses answers that do not carry the declared layout.
pub fn decode_u64(answer: &[u8]) -> Result<u64, AbiError> {
    Answer::new(answer).u64(0)
}

/// Decodes one `uint8` answer.
///
/// # Errors
/// Refuses answers that do not carry the declared layout.
pub fn decode_u8(answer: &[u8]) -> Result<u8, AbiError> {
    Answer::new(answer).u8(0)
}

/// Decodes `all_balances(address)` into denomination and full-width amount pairs.
///
/// # Errors
/// Refuses malformed layouts, more than 256 coins, and invalid or oversized text.
pub fn decode_coins(answer: &[u8]) -> Result<Vec<(String, [u8; 32])>, AbiError> {
    let outer = Answer::new(answer);
    let base = usize::try_from(outer.u64(0)?).map_err(|_| AbiError::Offset)?;
    if base < 32 || base % 32 != 0 {
        return Err(AbiError::Offset);
    }
    let array = Answer::new(answer.get(base..).ok_or(AbiError::Offset)?);
    let count = usize::try_from(array.u64(0)?).map_err(|_| AbiError::Width)?;
    if count > 256 {
        return Err(AbiError::Width);
    }
    let head_bytes = count.checked_mul(32).ok_or(AbiError::Offset)?;
    let elements_base = base.checked_add(32).ok_or(AbiError::Offset)?;
    let elements = answer.get(elements_base..).ok_or(AbiError::Truncated)?;
    if elements.len() < head_bytes {
        return Err(AbiError::Truncated);
    }
    let offsets = Answer::new(elements);
    let mut coins = Vec::with_capacity(count);
    for index in 0..count {
        let offset = usize::try_from(offsets.u64(index)?).map_err(|_| AbiError::Offset)?;
        if offset < head_bytes || offset % 32 != 0 {
            return Err(AbiError::Offset);
        }
        let tuple = Answer::new(elements.get(offset..).ok_or(AbiError::Offset)?);
        let amount = tuple.word(0)?;
        let denom_offset = tuple.u64(1)?;
        if denom_offset < 64 || denom_offset % 32 != 0 {
            return Err(AbiError::Offset);
        }
        coins.push((tuple.string(1, 0)?, amount));
    }
    Ok(coins)
}

pub fn decode_ledger_asset(answer: &[u8]) -> Result<LedgerAsset, AbiError> {
    let answer = Answer::new(answer);
    for index in [1, 2, 9] {
        if answer.u64(index)? < 12 * 32 {
            return Err(AbiError::Offset);
        }
    }
    Ok(LedgerAsset {
        asset_id: answer.word(0)?,
        symbol: answer.string(1, 0)?,
        name: answer.string(2, 0)?,
        decimals: answer.u8(3)?,
        supply_cap: answer.u128(4)?,
        issuer_did: answer.word(5)?,
        salt: answer.word(6)?,
        issuer_kind: answer.u8(7)?,
        custody_kind: answer.u8(8)?,
        custody_reference: answer.bytes(9, 0)?,
        paused: answer.bool(10)?,
        total_units: answer.u128(11)?,
    })
}

/// Renders one 256-bit word as its exact decimal value.
#[must_use]
pub fn uint256_decimal(word: &[u8; 32]) -> String {
    let mut digits: Vec<u8> = Vec::with_capacity(78);
    let mut value = *word;
    while value.iter().any(|byte| *byte != 0) {
        let mut remainder = 0_u16;
        for byte in &mut value {
            let current = (remainder << 8) | u16::from(*byte);
            *byte = u8::try_from(current / 10).unwrap_or(0);
            remainder = current % 10;
        }
        digits.push(u8::try_from(remainder).unwrap_or(0) + b'0');
    }
    if digits.is_empty() {
        return "0".to_owned();
    }
    digits.reverse();
    String::from_utf8(digits).unwrap_or_else(|_| "0".to_owned())
}

/// Renders one 256-bit word as the minimal `0x` JSON-RPC quantity.
#[must_use]
pub fn uint256_quantity(word: &[u8; 32]) -> String {
    let mut text = String::with_capacity(66);
    text.push_str("0x");
    let start = word.iter().position(|byte| *byte != 0);
    match start {
        None => text.push('0'),
        Some(start) => {
            for (index, byte) in word[start..].iter().enumerate() {
                if index == 0 && *byte < 16 {
                    text.push(char::from_digit(u32::from(*byte), 16).unwrap_or('0'));
                } else {
                    text.push(char::from_digit(u32::from(byte >> 4), 16).unwrap_or('0'));
                    text.push(char::from_digit(u32::from(byte & 0x0f), 16).unwrap_or('0'));
                }
            }
        }
    }
    text
}

/// Renders 20 address bytes as a lowercase `0x` address.
#[must_use]
pub fn address_hex(address: &[u8; 20]) -> String {
    let mut text = String::with_capacity(42);
    text.push_str("0x");
    for byte in address {
        text.push(char::from_digit(u32::from(byte >> 4), 16).unwrap_or('0'));
        text.push(char::from_digit(u32::from(byte & 0x0f), 16).unwrap_or('0'));
    }
    text
}

/// Parses a `0x`-prefixed 20-byte address, or `None` when the text is not
/// exactly 40 hexadecimal characters.
#[must_use]
pub fn parse_address(value: &str) -> Option<[u8; 20]> {
    let digits = value.strip_prefix("0x").unwrap_or(value);
    if digits.len() != 40 || !digits.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return None;
    }
    let mut address = [0_u8; 20];
    for (index, byte) in address.iter_mut().enumerate() {
        let pair = digits.get(index * 2..index * 2 + 2)?;
        *byte = u8::from_str_radix(pair, 16).ok()?;
    }
    Some(address)
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::Value;

    fn vectors() -> Value {
        serde_json::from_slice(include_bytes!("../tests/fixtures/paxeer-abi-vectors.json"))
            .unwrap_or_else(|error| panic!("{error}"))
    }

    fn bytes(text: &str) -> Vec<u8> {
        (0..text.len() / 2)
            .map(|index| {
                u8::from_str_radix(&text[index * 2..index * 2 + 2], 16)
                    .unwrap_or_else(|error| panic!("{error}"))
            })
            .collect()
    }

    fn case(name: &str) -> (Vec<u8>, Vec<u8>, Value) {
        let document = vectors();
        let cases = document["cases"]
            .as_array()
            .unwrap_or_else(|| panic!("cases missing"))
            .clone();
        let found = cases
            .into_iter()
            .find(|case| case["name"] == name)
            .unwrap_or_else(|| panic!("{name} missing"));
        (
            bytes(found["calldata"].as_str().unwrap_or_default()),
            bytes(found["result"].as_str().unwrap_or_default()),
            found["out"].clone(),
        )
    }

    fn word32(text: &str) -> [u8; 32] {
        let mut word = [0_u8; 32];
        word.copy_from_slice(&bytes(text));
        word
    }

    #[test]
    fn merged_construct_addresses_are_exact() {
        for (address, expected) in [
            (
                LEDGER_PRECOMPILE,
                "0x000000000000000000000000000000000000101a",
            ),
            (PAY_PRECOMPILE, "0x000000000000000000000000000000000000101b"),
            (
                AUTH_PRECOMPILE,
                "0x000000000000000000000000000000000000101c",
            ),
            (
                PROGRAMS_PRECOMPILE,
                "0x000000000000000000000000000000000000101d",
            ),
        ] {
            assert_eq!(address_hex(&address), expected);
        }
    }

    #[test]
    fn all_balances_answer_decodes_exactly() {
        let (calldata, result, out) = case("bank.all_balances");
        let account = parse_address("0x102132435465768798a9bacbdcedfe0f1e2d3c4b")
            .unwrap_or_else(|| panic!("address"));
        assert_eq!(
            calldata_address(SELECTOR_BANK_ALL_BALANCES, &account),
            calldata
        );
        let coins = decode_coins(&result).unwrap_or_else(|error| panic!("{error:?}"));
        let expected = out["response"]
            .as_array()
            .unwrap_or_else(|| panic!("coins"));
        assert_eq!(expected.len(), 2);
        assert_eq!(coins.len(), expected.len());
        for ((denom, amount), expected) in coins.iter().zip(expected) {
            assert_eq!(Value::String(denom.clone()), expected["denom"]);
            assert_eq!(Value::String(uint256_decimal(amount)), expected["amount"]);
        }
        assert_eq!(coins[0].1, word_u64(123456789));
        let mut large = [0_u8; 32];
        large[6] = 1;
        assert_eq!(coins[1].1, large);
        for length in 0..result.len() {
            assert!(decode_coins(&result[..length]).is_err(), "length {length}");
        }
        let mut too_many = result.clone();
        too_many[32..64].copy_from_slice(&word_u64(257));
        assert_eq!(decode_coins(&too_many), Err(AbiError::Width));
        let mut bad_offset = result.clone();
        bad_offset[..32].copy_from_slice(&word_u64(33));
        assert_eq!(decode_coins(&bad_offset), Err(AbiError::Offset));
        bad_offset[..32].copy_from_slice(&word_u64(0));
        assert_eq!(decode_coins(&bad_offset), Err(AbiError::Offset));
        let mut bad_tuple = result.clone();
        bad_tuple[64..96].copy_from_slice(&word_u64(0));
        assert_eq!(decode_coins(&bad_tuple), Err(AbiError::Offset));
        let mut oversized_denom = result.clone();
        oversized_denom[192..224].copy_from_slice(&word_u64(4097));
        assert_eq!(decode_coins(&oversized_denom), Err(AbiError::Text));
        let mut invalid_text = result.clone();
        invalid_text[224] = 0xff;
        assert_eq!(decode_coins(&invalid_text), Err(AbiError::Text));
        let mut empty = word_u64(32).to_vec();
        empty.extend_from_slice(&word_u64(0));
        assert_eq!(decode_coins(&empty), Ok(Vec::new()));
    }

    #[test]
    fn precompile_addresses_and_selectors_match_the_generated_vectors() {
        let document = vectors();
        let precompiles = document["precompiles"]
            .as_object()
            .unwrap_or_else(|| panic!("precompiles missing"));
        for retired in ["custody", "anchor"] {
            assert!(!precompiles.contains_key(retired), "{retired}");
        }
        let addresses = [
            ("bank", BANK_PRECOMPILE),
            ("addr", ADDR_PRECOMPILE),
            ("exchange", EXCHANGE_PRECOMPILE),
            ("bridge", BRIDGE_PRECOMPILE),
            ("launchpad", LAUNCHPAD_PRECOMPILE),
            ("ledger", LEDGER_PRECOMPILE),
            ("pay", PAY_PRECOMPILE),
            ("auth", AUTH_PRECOMPILE),
            ("programs", PROGRAMS_PRECOMPILE),
        ];
        assert_eq!(precompiles.len(), addresses.len());
        for (name, address) in addresses {
            assert_eq!(precompiles[name], address_hex(&address), "{name}");
        }
        let cases = document["cases"]
            .as_array()
            .unwrap_or_else(|| panic!("cases missing"));
        assert_eq!(cases.len(), 6);
        for vector in cases {
            let contract = vector["contract"]
                .as_str()
                .unwrap_or_else(|| panic!("contract missing"));
            let name = vector["name"]
                .as_str()
                .unwrap_or_else(|| panic!("name missing"));
            assert!(matches!(contract, "addr" | "bank"), "{contract}");
            assert!(name.starts_with(&format!("{contract}.")), "{name}");
        }
        for (name, selector) in [
            ("addr.getUnifiedAccount", SELECTOR_GET_UNIFIED_ACCOUNT),
            ("addr.getLayerXDid", SELECTOR_GET_LAYERX_DID),
            ("addr.getEvmAddrByLayerX", SELECTOR_GET_EVM_ADDR_BY_LAYERX),
            ("addr.layerXBindNonce", SELECTOR_LAYERX_BIND_NONCE),
            ("bank.balance", SELECTOR_BANK_BALANCE),
            ("bank.all_balances", SELECTOR_BANK_ALL_BALANCES),
        ] {
            let (calldata, _, _) = case(name);
            assert_eq!(calldata.get(..4), Some(selector.as_slice()), "{name}");
            let vector = cases
                .iter()
                .find(|vector| vector["name"] == name)
                .unwrap_or_else(|| panic!("{name} missing"));
            assert_eq!(vector["selector"], hex_lower(&selector), "{name}");
        }
        let selectors = document["selectors"]
            .as_object()
            .unwrap_or_else(|| panic!("selectors missing"));
        for name in selectors.keys() {
            assert!(
                !name.starts_with("custody.") && !name.starts_with("anchor."),
                "{name}"
            );
        }
        assert_eq!(selectors.len(), 5);
        for (name, selector) in [
            ("bank.name", SELECTOR_BANK_NAME),
            ("bank.symbol", SELECTOR_BANK_SYMBOL),
            ("bank.decimals", SELECTOR_BANK_DECIMALS),
            ("bank.supply", SELECTOR_BANK_SUPPLY),
            ("ledger.getAsset", SELECTOR_GET_ASSET),
        ] {
            assert_eq!(selectors[name], hex_lower(&selector), "{name}");
        }
    }

    #[test]
    fn calldata_matches_the_go_ethereum_encoding() {
        let account = parse_address("0x102132435465768798a9bacbdcedfe0f1e2d3c4b")
            .unwrap_or_else(|| panic!("address"));
        let did = [0x61_u8; 32];
        for (name, calldata) in [
            (
                "addr.getUnifiedAccount",
                calldata_address(SELECTOR_GET_UNIFIED_ACCOUNT, &account),
            ),
            (
                "addr.getLayerXDid",
                calldata_address(SELECTOR_GET_LAYERX_DID, &account),
            ),
            (
                "addr.getEvmAddrByLayerX",
                calldata_word(SELECTOR_GET_EVM_ADDR_BY_LAYERX, &did),
            ),
            (
                "addr.layerXBindNonce",
                calldata_address(SELECTOR_LAYERX_BIND_NONCE, &account),
            ),
            (
                "bank.balance",
                calldata_address_string(SELECTOR_BANK_BALANCE, &account, "ulxp"),
            ),
            (
                "bank.all_balances",
                calldata_address(SELECTOR_BANK_ALL_BALANCES, &account),
            ),
        ] {
            let (expected, _, _) = case(name);
            assert_eq!(calldata, expected, "{name}");
        }
    }

    #[test]
    fn account_answers_decode_exactly() {
        let (_, result, out) = case("addr.getUnifiedAccount");
        let account = decode_unified_account(&result).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(address_hex(&account.evm), out["evm"]);
        assert_eq!(Value::String(account.pax_address), out["paxAddr"]);
        assert_eq!(
            Value::String(hex_lower(&account.did_public_key)),
            out["didPublicKey"]
        );
        assert_eq!(
            Value::String(hex_lower(&account.layerx_main_account_id)),
            out["layerxMainAccountId"]
        );
        let (_, result, out) = case("addr.getLayerXDid");
        let (key, did) = decode_layerx_did(&result).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(Value::String(hex_lower(&key)), out["didPublicKey"]);
        assert_eq!(Value::String(did), out["did"]);
        let (_, result, out) = case("addr.getEvmAddrByLayerX");
        let address = decode_address(&result).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(address_hex(&address), out["evmAddr"]);
        let (_, result, out) = case("addr.layerXBindNonce");
        let nonce = decode_u64(&result).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(Value::String(nonce.to_string()), out["nonce"]);
    }

    #[test]
    fn bank_balance_answer_decodes_exactly() {
        let (_, result, out) = case("bank.balance");
        let balance = decode_word(&result).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(Value::String(uint256_decimal(&balance)), out["amount"]);
    }

    fn hex_lower(bytes: &[u8]) -> String {
        let mut text = String::with_capacity(bytes.len() * 2);
        for byte in bytes {
            text.push(char::from_digit(u32::from(byte >> 4), 16).unwrap_or('0'));
            text.push(char::from_digit(u32::from(byte & 0x0f), 16).unwrap_or('0'));
        }
        text
    }

    #[test]
    fn quantities_and_addresses_render_without_loss() {
        assert_eq!(uint256_decimal(&[0_u8; 32]), "0");
        assert_eq!(uint256_quantity(&[0_u8; 32]), "0x0");
        assert_eq!(uint256_decimal(&word32(&"ff".repeat(32))), u256_max());
        assert_eq!(
            uint256_quantity(&word32(&"ff".repeat(32))),
            format!("0x{}", "ff".repeat(32))
        );
        let mut one = [0_u8; 32];
        one[31] = 1;
        assert_eq!(uint256_quantity(&one), "0x1");
        assert_eq!(uint256_decimal(&one), "1");
        let mut sixteen = [0_u8; 32];
        sixteen[31] = 16;
        assert_eq!(uint256_quantity(&sixteen), "0x10");
        assert_eq!(
            parse_address("0x00000000000000000000000000000000000000a1"),
            Some([0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xa1])
        );
        for invalid in ["0x", "0xzz", "00", &"ab".repeat(21)] {
            assert_eq!(parse_address(invalid), None);
        }
    }

    fn u256_max() -> String {
        "115792089237316195423570985008687907853269984665640564039457584007913129639935".to_owned()
    }

    #[test]
    fn sequencer_receipt_is_the_only_relayed_pax_name() {
        assert!(relayable_method("pax_sequencerReceipt"));
        for method in [
            "pax_other",
            "pax_",
            "pax_sequencerReceipt_other",
            "pax_sequencerreceipt",
            "eth_sign",
        ] {
            assert!(!relayable_method(method), "{method}");
        }
    }

    #[test]
    fn relayed_methods_exclude_node_signing_and_subscriptions() {
        for method in [
            "eth_blockNumber",
            "eth_call",
            "eth_sendRawTransaction",
            "eth_getBalance",
            "net_version",
            "web3_clientVersion",
        ] {
            assert!(relayable_method(method), "{method}");
        }
        for method in [
            "eth_accounts",
            "eth_coinbase",
            "eth_sendTransaction",
            "eth_sign",
            "eth_signTransaction",
            "eth_signTypedData",
            "eth_signTypedData_v4",
            "eth_mining",
            "eth_subscribe",
            "eth_unsubscribe",
            "lx_getNodeInfo",
            "px_getNetwork",
            "eth_call ",
            "eth_",
        ] {
            assert_eq!(relayable_method(method), method == "eth_", "{method}");
        }
        assert!(!relayable_method(&format!("eth_{}", "a".repeat(64))));
        assert!(is_evm_namespace("eth_subscribe"));
        assert!(!is_evm_namespace("px_getAccount"));
    }

    #[test]
    fn malformed_answers_are_refused() {
        assert_eq!(decode_u64(&[0_u8; 31]), Err(AbiError::Truncated));
        assert_eq!(decode_u64(&[0xff_u8; 32]), Err(AbiError::Width));
        assert_eq!(decode_address(&[0xff_u8; 32]), Err(AbiError::Width));
        assert_eq!(
            decode_unified_account(&[0_u8; 32]),
            Err(AbiError::Truncated)
        );
        let mut outside = [0_u8; 32];
        outside[31] = 64;
        assert_eq!(Answer::new(&outside).string(0, 0), Err(AbiError::Offset));
        let mut unaligned = [0_u8; 64];
        unaligned[31] = 8;
        assert_eq!(Answer::new(&unaligned).string(0, 0), Err(AbiError::Offset));
        let mut non_boolean = [0_u8; 64];
        non_boolean[63] = 2;
        assert_eq!(Answer::new(&non_boolean).bool(1), Err(AbiError::Width));
        let (_, result, _) = case("addr.getUnifiedAccount");
        assert_eq!(
            decode_unified_account(&result[..result.len() - 32]),
            Err(AbiError::Truncated)
        );
    }
}
