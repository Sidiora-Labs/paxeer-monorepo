use layerx_crypto::evm_transaction::transaction_hash;
use layerx_platform_internal::evm_sender::{Eip1559Call, EvmSender};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::io::Write;
use std::net::{TcpStream, ToSocketAddrs};
use std::sync::{Arc, OnceLock};
use std::time::Duration;

use super::{hex_decode, hex_encode, parse_node_url, refusal, NodeEndpoint, Response};

pub(super) const DEPLOYMENT_CHAIN_ID: u64 = 125;

pub(super) const LAYERX_CORE_CHAIN_RPC_URL: &str = "LAYERX_CORE_CHAIN_RPC_URL";
pub(super) const LAYERX_CORE_FUNDING_SENDER_KEY_FILE: &str = "LAYERX_CORE_FUNDING_SENDER_KEY_FILE";
pub(super) const LAYERX_CORE_FUNDING_SENDER_KMS_KEY_ID: &str =
    "LAYERX_CORE_FUNDING_SENDER_KMS_KEY_ID";

/// Decoded funding configuration, before any connection to the chain.
pub(super) struct FundingEnvironment {
    pub(super) endpoint: NodeEndpoint,
    pub(super) sender: EvmSender,
}

impl FundingEnvironment {
    pub(super) fn from_env() -> Result<Self, &'static str> {
        let endpoint = parse_node_url(
            &std::env::var(LAYERX_CORE_CHAIN_RPC_URL).map_err(|_| "chain_unavailable")?,
        )
        .map_err(|_| "chain_unavailable")?;
        // EvmSender reads the KEY_FILE and KMS_KEY_ID alternatives under this
        // prefix and refuses ambiguous sources before opening either one.
        let prefix = LAYERX_CORE_FUNDING_SENDER_KEY_FILE
            .strip_suffix("_KEY_FILE")
            .ok_or("sender_unavailable")?;
        debug_assert_eq!(
            LAYERX_CORE_FUNDING_SENDER_KMS_KEY_ID,
            format!("{prefix}_KMS_KEY_ID")
        );
        let sender = EvmSender::from_env(prefix).map_err(|_| "sender_unavailable")?;
        Ok(Self { endpoint, sender })
    }
}

pub(super) struct ChainFunding {
    endpoint: NodeEndpoint,
    sender: Arc<EvmSender>,
    pub(super) chain_id: u64,
}

pub(super) struct FundingSlot {
    configured: Result<ChainFunding, &'static str>,
    connected: OnceLock<ChainFunding>,
}

impl FundingSlot {
    pub(super) fn from_env() -> Self {
        Self::configured(ChainFunding::from_env())
    }

    pub(super) fn configured(configured: Result<ChainFunding, &'static str>) -> Self {
        Self {
            configured,
            connected: OnceLock::new(),
        }
    }

    pub(super) fn get(&self) -> Result<&ChainFunding, &'static str> {
        let configured = self.configured.as_ref().map_err(|reason| *reason)?;
        if let Some(funding) = self.connected.get() {
            return Ok(funding);
        }
        let funding = configured.connect()?;
        Ok(self.connected.get_or_init(|| funding))
    }
}

#[derive(Clone, Debug, Eq, PartialEq, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(super) struct SignedFunding {
    pub(super) raw: Vec<u8>,
    pub(super) transaction_hash: String,
}

impl SignedFunding {
    pub(super) fn validate(&self) -> Result<(), ()> {
        if self.raw.first() != Some(&2)
            || self.raw.len() < 64
            || self.raw.len() > 1024
            || self.transaction_hash != format!("0x{}", hex_encode(&transaction_hash(&self.raw)))
        {
            return Err(());
        }
        Ok(())
    }
}

#[derive(Clone, Debug, Eq, PartialEq, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(super) struct NativeTransfer {
    pub(super) version: u8,
    pub(super) signed: SignedFunding,
    pub(super) sender: [u8; 20],
    pub(super) recipient: [u8; 20],
    pub(super) amount: u64,
    pub(super) chain_id: u64,
    pub(super) authorization_key: [u8; 32],
    pub(super) authorization: Vec<u8>,
}

impl NativeTransfer {
    fn digest(&self) -> [u8; 32] {
        use sha2::{Digest as _, Sha256};
        let mut hash = Sha256::new();
        hash.update(b"layerx-core/native-transfer/v2\0");
        hash.update([self.version]);
        hash.update(self.chain_id.to_be_bytes());
        hash.update(self.sender);
        hash.update(self.recipient);
        hash.update(self.amount.to_be_bytes());
        hash.update(&self.signed.raw);
        hash.update(self.authorization_key);
        hash.finalize().into()
    }

    pub(super) fn attest(
        funding: SignedFunding,
        sender: [u8; 20],
        signer: &dyn layerx_platform_core::TreasurySigner,
    ) -> Result<Self, String> {
        let (chain_id, recipient, amount) = transfer_fields(&funding)?;
        let mut value = Self {
            version: 2,
            signed: funding,
            sender,
            recipient,
            amount,
            chain_id,
            authorization_key: signer.public_key(),
            authorization: Vec::new(),
        };
        value.authorization = signer.sign_digest(&value.digest())?.to_vec();
        value.validate(signer.public_key())?;
        Ok(value)
    }

    pub(super) fn validate(&self, key: [u8; 32]) -> Result<(), String> {
        let (chain_id, recipient, amount) = transfer_fields(&self.signed)?;
        if self.version != 2
            || self.chain_id != DEPLOYMENT_CHAIN_ID
            || chain_id != self.chain_id
            || recipient != self.recipient
            || amount != self.amount
            || amount == 0
            || self.sender == [0; 20]
            || key != self.authorization_key
        {
            return Err("native transfer identity differs".into());
        }
        let signature = ed25519_dalek::Signature::from_slice(&self.authorization)
            .map_err(|_| "native transfer authorization malformed")?;
        ed25519_dalek::VerifyingKey::from_bytes(&key)
            .map_err(|_| "native transfer authorization key malformed")?
            .verify_strict(&self.digest(), &signature)
            .map_err(|_| "native transfer authorization differs".into())
    }
}

/// Decode the exact value-bearing type-2 envelope produced below. The shared
/// `Eip1559Call` decoder deliberately refuses value transfers, so it must not be
/// used to reinterpret a native transfer as a legacy ledger call.
fn transfer_fields(signed: &SignedFunding) -> Result<(u64, [u8; 20], u64), String> {
    signed
        .validate()
        .map_err(|()| "invalid native transaction hash")?;
    let mut envelope = &signed.raw[1..];
    let (is_list, mut fields) = take_rlp(&mut envelope)?;
    if !is_list || !envelope.is_empty() {
        return Err("invalid native envelope".into());
    }
    let mut items = Vec::new();
    while !fields.is_empty() {
        items.push(take_rlp(&mut fields)?);
    }
    if items.len() != 12
        || items
            .iter()
            .enumerate()
            .any(|(index, (list, _))| *list != (index == 8))
    {
        return Err("invalid native transfer fields".into());
    }
    let number = |index: usize, width: usize| -> Result<u128, String> {
        let value = items[index].1;
        if value.len() > width || value.first() == Some(&0) {
            return Err("invalid native integer".into());
        }
        let mut bytes = [0; 16];
        bytes[16 - value.len()..].copy_from_slice(value);
        Ok(u128::from_be_bytes(bytes))
    };
    let chain_id = u64::try_from(number(0, 8)?).map_err(|_| "invalid native chain id")?;
    number(1, 8)?;
    if number(2, 16)? > number(3, 16)?
        || number(4, 8)? == 0
        || !items[7].1.is_empty()
        || !items[8].1.is_empty()
    {
        return Err("invalid native transfer parameters".into());
    }
    let recipient = items[5]
        .1
        .try_into()
        .map_err(|_| "invalid native recipient")?;
    let amount = u64::try_from(number(6, 8)?).map_err(|_| "invalid native amount")?;
    let parity = u8::try_from(number(9, 1)?).map_err(|_| "invalid native parity")?;
    if parity > 1 {
        return Err("invalid native parity".into());
    }
    let mut signature = [0; 65];
    for (index, start) in [(10, 0), (11, 32)] {
        let scalar = items[index].1;
        if scalar.is_empty() || scalar.len() > 32 || scalar[0] == 0 {
            return Err("invalid native signature scalar".into());
        }
        signature[start + 32 - scalar.len()..start + 32].copy_from_slice(scalar);
    }
    signature[64] = parity + 27;
    layerx_crypto::signer::AgentSignature::new(signature)
        .map_err(|_| "invalid native signature")?;
    Ok((chain_id, recipient, amount))
}

fn take_rlp<'a>(input: &mut &'a [u8]) -> Result<(bool, &'a [u8]), String> {
    let first = *input.first().ok_or("truncated native RLP")?;
    if first < 0x80 {
        let (value, rest) = input.split_at(1);
        *input = rest;
        return Ok((false, value));
    }
    let list = first >= 0xc0;
    let base = if list { 0xc0 } else { 0x80 };
    let size = usize::from(first - base);
    let (offset, length) = if size <= 55 {
        (1, size)
    } else {
        let width = size - 55;
        if width > std::mem::size_of::<usize>() {
            return Err("native RLP length overflow".into());
        }
        let bytes = input
            .get(1..1 + width)
            .ok_or("truncated native RLP length")?;
        if bytes[0] == 0 {
            return Err("noncanonical native RLP length".into());
        }
        let length = bytes.iter().try_fold(0usize, |length, byte| {
            length
                .checked_mul(256)
                .and_then(|length| length.checked_add(usize::from(*byte)))
                .ok_or("native RLP length overflow")
        })?;
        if length < 56 {
            return Err("noncanonical native RLP length".into());
        }
        (1 + width, length)
    };
    let end = offset
        .checked_add(length)
        .ok_or("native RLP length overflow")?;
    let value = input.get(offset..end).ok_or("truncated native RLP item")?;
    if !list && length == 1 && value[0] < 0x80 {
        return Err("noncanonical native RLP byte".into());
    }
    *input = &input[end..];
    Ok((list, value))
}

impl ChainFunding {
    pub(super) fn from_sender(endpoint: NodeEndpoint, sender: EvmSender, chain_id: u64) -> Self {
        Self {
            endpoint,
            sender: Arc::new(sender),
            chain_id,
        }
    }

    pub(super) fn from_env() -> Result<Self, &'static str> {
        let FundingEnvironment { endpoint, sender } = FundingEnvironment::from_env()?;
        Ok(Self::from_sender(endpoint, sender, DEPLOYMENT_CHAIN_ID))
    }

    fn connect(&self) -> Result<Self, &'static str> {
        self.validate_network()?;
        Ok(Self {
            endpoint: self.endpoint.clone(),
            sender: Arc::clone(&self.sender),
            chain_id: DEPLOYMENT_CHAIN_ID,
        })
    }

    fn validate_network(&self) -> Result<(), &'static str> {
        let chain_id: u64 = quantity(
            &self
                .rpc("eth_chainId", &json!([]))
                .map_err(|_| "chain_unavailable")?,
        )
        .map_err(|_| "chain_unavailable")?
        .try_into()
        .map_err(|_| "chain_unavailable")?;
        if chain_id != DEPLOYMENT_CHAIN_ID || self.chain_id != DEPLOYMENT_CHAIN_ID {
            return Err("chain_unavailable");
        }
        Ok(())
    }

    pub(super) fn readiness(&self) -> Result<Value, &'static str> {
        self.validate_network()?;
        let latest_block: u64 = quantity(
            &self
                .rpc("eth_blockNumber", &json!([]))
                .map_err(|_| "chain_unavailable")?,
        )
        .map_err(|_| "chain_unavailable")?
        .try_into()
        .map_err(|_| "chain_unavailable")?;
        Ok(json!({
            "ready": true,
            "chain_id": DEPLOYMENT_CHAIN_ID,
            "latest_block": latest_block,
            "sender": format!("0x{}", hex_encode(&self.sender.address())),
        }))
    }

    fn rpc(&self, method: &str, params: &Value) -> Result<Value, String> {
        let body =
            json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).to_string();
        let address = (self.endpoint.host.as_str(), self.endpoint.port)
            .to_socket_addrs()
            .map_err(|_| "funding endpoint unavailable")?
            .next()
            .ok_or("funding endpoint unavailable")?;
        let mut stream = TcpStream::connect_timeout(&address, Duration::from_secs(3))
            .map_err(|_| "funding endpoint unavailable")?;
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .and_then(|()| stream.set_write_timeout(Some(Duration::from_secs(5))))
            .map_err(|_| "funding deadline unavailable")?;
        write!(stream, "POST / HTTP/1.1\r\nHost: {}:{}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            self.endpoint.host, self.endpoint.port, body.len(), body)
            .map_err(|_| "funding request unavailable")?;
        let response = layerx_platform_internal::http::read_http_message(&mut stream, 64 * 1024)?;
        if response
            .headers
            .get("")
            .and_then(|line| line.split_whitespace().nth(1))
            != Some("200")
        {
            return Err("funding RPC HTTP refusal".to_owned());
        }
        let response: Value =
            serde_json::from_slice(&response.body).map_err(|_| "funding RPC malformed")?;
        if response["jsonrpc"] != "2.0" || response["id"] != 1 {
            return Err("funding RPC identity differs".to_owned());
        }
        if !response["error"].is_null() {
            if method == "eth_sendRawTransaction"
                && matches!(
                    response["error"]["message"].as_str(),
                    Some("already known" | "known transaction")
                )
            {
                return Err("funding transaction already known".to_owned());
            }
            return Err("funding RPC refused".to_owned());
        }
        response
            .get("result")
            .cloned()
            .ok_or_else(|| "funding RPC result missing".to_owned())
    }

    pub(super) fn prepare(
        &self,
        public_key: &[u8; 32],
        amount: u64,
    ) -> Result<SignedFunding, Response> {
        let unavailable = || refusal(503, "treasury_unavailable", Some(5));
        let to = self.bound_recipient(public_key)?;
        let nonce = quantity(
            &self
                .rpc(
                    "eth_getTransactionCount",
                    &json!([
                        format!("0x{}", hex_encode(&self.sender.address())),
                        "pending"
                    ]),
                )
                .map_err(|_| unavailable())?,
        )
        .map_err(|_| unavailable())?
        .try_into()
        .map_err(|_| unavailable())?;
        let gas_price = quantity(
            &self
                .rpc("eth_gasPrice", &json!([]))
                .map_err(|_| unavailable())?,
        )
        .map_err(|_| unavailable())?;
        let priority = quantity(
            &self
                .rpc("eth_maxPriorityFeePerGas", &json!([]))
                .map_err(|_| unavailable())?,
        )
        .map_err(|_| unavailable())?;
        let max_fee = gas_price
            .checked_mul(2)
            .and_then(|fee| fee.checked_add(priority))
            .ok_or_else(unavailable)?;
        let gas = quantity(
            &self
                .rpc(
                    "eth_estimateGas",
                    &json!([{
                        "from": format!("0x{}", hex_encode(&self.sender.address())),
                        "to": format!("0x{}", hex_encode(&to)),
                        "value": format!("0x{amount:x}"), "data": "0x",
                    }]),
                )
                .map_err(|_| unavailable())?,
        )
        .map_err(|_| unavailable())?;
        self.sign_native(
            to,
            amount,
            nonce,
            gas.try_into().map_err(|_| unavailable())?,
            priority,
            max_fee,
        )
    }

    pub(super) fn bound_recipient(&self, public_key: &[u8; 32]) -> Result<[u8; 20], Response> {
        let unavailable = || refusal(503, "treasury_unavailable", Some(5));
        let result = self
            .rpc(
                "eth_call",
                &json!([{
            "to": "0x0000000000000000000000000000000000001004",
            "data": format!("0xc413bad3{}", hex_encode(public_key)),
        }, "latest"]),
            )
            .map_err(|_| unavailable())?;
        let bound = result
            .as_str()
            .and_then(|text| text.strip_prefix("0x"))
            .and_then(|text| hex_decode(text).ok())
            .filter(|bytes| bytes.len() == 32 && bytes[..12] == [0; 12]);
        let bound = bound.ok_or_else(|| refusal(409, "identity_binding_unavailable", None))?;
        let to: [u8; 20] = bound[12..]
            .try_into()
            .map_err(|_| refusal(409, "identity_binding_unavailable", None))?;
        if to == [0; 20] {
            return Err(refusal(409, "identity_binding_unavailable", None));
        }
        Ok(to)
    }

    pub(super) fn confirmation(
        &self,
        transfer: &NativeTransfer,
    ) -> Result<Option<Value>, Response> {
        transfer
            .validate(transfer.authorization_key)
            .map_err(|_| refusal(503, "journal_unavailable", Some(5)))?;
        if transfer.sender != self.sender.address() || transfer.chain_id != self.chain_id {
            return Err(refusal(409, "funding_sender_conflict", None));
        }
        let receipt = self
            .rpc(
                "eth_getTransactionReceipt",
                &json!([transfer.signed.transaction_hash]),
            )
            .map_err(|_| refusal(503, "receipt_unavailable", Some(5)))?;
        if receipt.is_null() {
            return Ok(None);
        }
        validate_native_confirmation(transfer, &receipt)
            .map_err(|_| refusal(503, "receipt_unavailable", Some(5)))?;
        Ok(Some(receipt))
    }

    pub(super) fn sender_address(&self) -> [u8; 20] {
        self.sender.address()
    }

    pub(super) fn sign_native(
        &self,
        to: [u8; 20],
        amount: u64,
        nonce: u64,
        gas_limit: u64,
        priority: u128,
        max_fee: u128,
    ) -> Result<SignedFunding, Response> {
        let unavailable = || refusal(503, "treasury_unavailable", Some(5));
        if self.chain_id != DEPLOYMENT_CHAIN_ID
            || to == [0; 20]
            || amount == 0
            || gas_limit == 0
            || priority > max_fee
        {
            return Err(unavailable());
        }
        self.validate_network().map_err(|_| unavailable())?;
        let call = Eip1559Call {
            chain_id: self.chain_id,
            nonce,
            max_priority_fee_per_gas: priority,
            max_fee_per_gas: max_fee,
            gas_limit,
            to,
            data: Vec::new(),
        };
        // Eip1559Call's shared encoder fixes value to zero. Encode the same
        // type-2 fields with the funding value before asking EvmSender to sign.
        let mut fields = Vec::new();
        for value in [
            u128::from(call.chain_id),
            u128::from(call.nonce),
            call.max_priority_fee_per_gas,
            call.max_fee_per_gas,
            u128::from(call.gas_limit),
        ] {
            integer(value, &mut fields);
        }
        bytes(&call.to, &mut fields);
        integer(u128::from(amount), &mut fields);
        bytes(&call.data, &mut fields);
        fields.push(0xc0);
        let mut preimage = vec![2];
        preimage.extend(list(&fields));
        let signature = self
            .sender
            .sign_prehash(&transaction_hash(&preimage))
            .map_err(|_| unavailable())?;
        integer(u128::from(signature[64] - 27), &mut fields);
        scalar(&signature[..32], &mut fields);
        scalar(&signature[32..64], &mut fields);
        let mut raw = vec![2];
        raw.extend(list(&fields));
        Ok(SignedFunding {
            transaction_hash: format!("0x{}", hex_encode(&transaction_hash(&raw))),
            raw,
        })
    }

    pub(super) fn submit(&self, signed: &SignedFunding) -> Result<(), Response> {
        let result = match self.rpc(
            "eth_sendRawTransaction",
            &json!([format!("0x{}", hex_encode(&signed.raw))]),
        ) {
            Ok(result) => result,
            Err(error) if error == "funding transaction already known" => return Ok(()),
            Err(_) => {
                if self.funded(signed)? {
                    return Ok(());
                }
                return Err(refusal(503, "treasury_unavailable", Some(5)));
            }
        };
        if result.as_str() != Some(signed.transaction_hash.as_str()) {
            return Err(refusal(503, "treasury_unavailable", Some(5)));
        }
        Ok(())
    }

    pub(super) fn funded(&self, signed: &SignedFunding) -> Result<bool, Response> {
        let unavailable = || refusal(503, "receipt_unavailable", Some(5));
        let receipt = self
            .rpc(
                "eth_getTransactionReceipt",
                &json!([signed.transaction_hash]),
            )
            .map_err(|_| unavailable())?;
        if receipt.is_null() {
            return Ok(false);
        }
        if receipt["transactionHash"].as_str() != Some(signed.transaction_hash.as_str()) {
            return Err(unavailable());
        }
        match quantity(&receipt["status"]).map_err(|_| unavailable())? {
            1 => Ok(true),
            0 => Err(refusal(422, "funding_failed", None)),
            _ => Err(unavailable()),
        }
    }
}

pub(super) fn validate_native_confirmation(
    transfer: &NativeTransfer,
    receipt: &Value,
) -> Result<bool, String> {
    let address = |value: &Value| -> Result<[u8; 20], String> {
        let text = value
            .as_str()
            .and_then(|text| text.strip_prefix("0x"))
            .ok_or("invalid native receipt address")?;
        super::fixed_hex("receipt address", text)
    };
    let block_hash = receipt["blockHash"]
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .ok_or("invalid native receipt block")?;
    let block_hash = super::fixed_hex::<32>("receipt block", block_hash)?;
    if receipt["transactionHash"].as_str() != Some(transfer.signed.transaction_hash.as_str())
        || address(&receipt["from"])? != transfer.sender
        || address(&receipt["to"])? != transfer.recipient
        || quantity(&receipt["blockNumber"])? == 0
        || block_hash == [0; 32]
    {
        return Err("native receipt transfer binding differs".into());
    }
    match quantity(&receipt["status"])? {
        1 => Ok(true),
        0 => Ok(false),
        _ => Err("invalid native receipt status".into()),
    }
}

fn quantity(value: &Value) -> Result<u128, String> {
    let value = value
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .filter(|value| !value.is_empty() && (value.len() == 1 || !value.starts_with('0')))
        .filter(|value| value.bytes().all(|byte| byte.is_ascii_hexdigit()))
        .ok_or("invalid funding RPC quantity")?;
    u128::from_str_radix(value, 16).map_err(|_| "funding RPC quantity overflow".to_owned())
}

fn length(size: usize, short: u8, output: &mut Vec<u8>) {
    if size < 56 {
        output.push(
            short
                + u8::try_from(size)
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "short RLP length")),
        );
    } else {
        let size = size.to_be_bytes();
        let first = size
            .iter()
            .position(|byte| *byte != 0)
            .unwrap_or_else(|| panic!("{}", "nonzero RLP length"));
        output.push(
            short
                + 55
                + u8::try_from(size.len() - first)
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "RLP length width")),
        );
        output.extend_from_slice(&size[first..]);
    }
}

fn bytes(value: &[u8], output: &mut Vec<u8>) {
    if value.len() == 1 && value[0] < 0x80 {
        output.push(value[0]);
    } else {
        length(value.len(), 0x80, output);
        output.extend_from_slice(value);
    }
}

fn scalar(value: &[u8], output: &mut Vec<u8>) {
    let first = value
        .iter()
        .position(|byte| *byte != 0)
        .unwrap_or(value.len());
    bytes(&value[first..], output);
}

fn integer(value: u128, output: &mut Vec<u8>) {
    scalar(&value.to_be_bytes(), output);
}

fn list(payload: &[u8]) -> Vec<u8> {
    let mut output = Vec::new();
    length(payload.len(), 0xc0, &mut output);
    output.extend_from_slice(payload);
    output
}
