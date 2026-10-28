use super::*;
use crate::onboarding::gas_station::{Call, GasStationClient, QuoteRequest, CHAIN_ID};
use crate::store::TenancyMap;
use layerx_platform_internal::tls::{Origin, Upstream};
use serde::Deserialize;
use sha3::{Digest as _, Keccak256};

const LAYERX_HUMAN_GAS_STATION_ENDPOINT: &str = "LAYERX_HUMAN_GAS_STATION_ENDPOINT";
const STATION_PREFIX: &str = "LAYERX_HUMAN_GAS_STATION";
const ADDR: &str = "0x0000000000000000000000000000000000001004";

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct RegistryDocument {
    chain_id: u64,
    gas_station: Option<String>,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Prepare {
    account: String,
    did_public_key: String,
    signature: String,
    bind_nonce: u64,
    nonce: String,
    max_token_amount: u128,
    gas_cost: u128,
    token: String,
    decimals: u8,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SigningRequest {
    batch: serde_json::Value,
    call: serde_json::Value,
    authorization: serde_json::Value,
    #[serde(rename = "accountSignature")]
    account_signature: String,
    #[serde(rename = "relayerSignature")]
    relayer_signature: String,
}

struct Runtime {
    station: GasStationClient,
}

fn refused<E>(_: E) -> String {
    "onboarding sponsor authority or durable state refused".to_owned()
}

/// # Errors
/// Refuses invalid input, unavailable sponsorship and receipts without a matching binding.
pub fn onboarding_sponsor_command(operation: &str, input: &[u8]) -> Result<Vec<u8>, String> {
    if !matches!(operation, "initialize-store" | "prepare" | "sign") {
        return Err("unknown sponsor operation".to_owned());
    }
    if input.len() > 1_048_576 {
        return Err("sponsor request exceeds bound".to_owned());
    }
    if operation == "initialize-store" {
        if !input.is_empty() {
            return Err("store initialization has no input body".to_owned());
        }
        let digest = initialize_store()?;
        return serde_json::to_vec(
            &json!({"tenancy_digest": URL_SAFE_NO_PAD.encode(digest.bytes())}),
        )
        .map_err(refused);
    }
    let runtime = Runtime::open()?;
    let value = match operation {
        "prepare" => runtime.prepare(&serde_json::from_slice(input).map_err(refused)?)?,
        "sign" => runtime.sign(&serde_json::from_slice(input).map_err(refused)?)?,
        _ => return Err("unknown sponsor operation".to_owned()),
    };
    serde_json::to_vec(&value).map_err(refused)
}

fn initialize_store() -> Result<crate::store::TenancyDigest, String> {
    use std::os::unix::fs::{MetadataExt as _, PermissionsExt as _};
    let root = absolute("LAYERX_HUMAN_STORE_ROOT")?;
    let parent = root.parent().ok_or_else(|| refused(()))?;
    if fs::canonicalize(parent).map_err(refused)? != parent {
        return Err(refused(()));
    }
    let map = TenancyMap::new([]).map_err(refused)?;
    let sealed = map.seal().map_err(refused)?;
    match fs::create_dir(&root) {
        Ok(()) => fs::set_permissions(&root, fs::Permissions::from_mode(0o700)).map_err(refused)?,
        Err(error) if error.kind() == std::io::ErrorKind::AlreadyExists => (),
        Err(error) => return Err(refused(error)),
    }
    let info = fs::symlink_metadata(&root).map_err(refused)?;
    if !info.is_dir()
        || info.uid() != rustix::process::geteuid().as_raw()
        || info.mode() & 0o077 != 0
    {
        return Err(refused(()));
    }
    for entry in fs::read_dir(&root).map_err(refused)? {
        let entry = entry.map_err(refused)?;
        let name = entry.file_name();
        if name != "tenancy.map" && name != "tenancy.map.tmp" {
            return Err(refused(()));
        }
        let info = fs::symlink_metadata(entry.path()).map_err(refused)?;
        if !info.is_file()
            || info.nlink() != 1
            || info.uid() != rustix::process::geteuid().as_raw()
            || info.mode() & 0o077 != 0
            || fs::read(entry.path()).map_err(refused)? != sealed
        {
            return Err(refused(()));
        }
    }
    let digest = map.install(&root).map_err(refused)?;
    fs::File::open(parent)
        .and_then(|file| file.sync_all())
        .map_err(refused)?;
    Ok(digest)
}

impl Runtime {
    fn open() -> Result<Self, String> {
        let variable = "LAYERX_HUMAN_ONBOARDING_REGISTRY_FILE";
        let path = absolute(variable)
            .map_err(|_| format!("{variable} must name an absolute registry file"))?;
        let invalid = || {
            format!(
                "{variable} ({}): expected chain_id 125 and an optional gas_station HTTPS origin",
                path.display()
            )
        };
        let document: RegistryDocument =
            serde_json::from_slice(&read_nonempty(&path).map_err(|_| invalid())?)
                .map_err(|_| invalid())?;
        if document.chain_id != CHAIN_ID {
            return Err(invalid());
        }
        let endpoint = match document.gas_station {
            Some(endpoint) => endpoint,
            None => required(LAYERX_HUMAN_GAS_STATION_ENDPOINT).map_err(|_| {
                format!(
                    "{variable} ({}) and {LAYERX_HUMAN_GAS_STATION_ENDPOINT} name no gas station",
                    path.display()
                )
            })?,
        };
        let origin = Origin::parse(&endpoint).map_err(|_| invalid())?;
        let upstream = Upstream::from_environment(STATION_PREFIX)?;
        if upstream.origin() != &origin {
            return Err(
                "gas station upstream URL differs from the onboarding registry or endpoint"
                    .to_owned(),
            );
        }
        Ok(Self {
            station: GasStationClient::new(upstream),
        })
    }

    fn prepare(&self, request: &Prepare) -> Result<serde_json::Value, String> {
        let account = fixed_hex::<20>(&request.account)?;
        let public_key = fixed_hex::<32>(&request.did_public_key)?;
        let signature = fixed_hex::<64>(&request.signature)?;
        if account == [0; 20] || public_key == [0; 32] {
            return Err(refused(()));
        }
        let mut message = b"LX:PAXEER-BIND:v1".to_vec();
        message.extend_from_slice(&word(u128::from(CHAIN_ID)));
        message.extend_from_slice(&account);
        message.extend_from_slice(&request.bind_nonce.to_be_bytes());
        layerx_crypto::ed25519::verify_message(&public_key, &signature, &message)
            .map_err(refused)?;
        let quote_request = QuoteRequest {
            account,
            nonce: decimal_word(&request.nonce)?,
            calls: vec![Call {
                to: fixed_hex::<20>(ADDR)?,
                value: [0; 32],
                data: bindLayerX(public_key, signature),
            }],
            max_token_amount: request.max_token_amount,
            gas_cost: request.gas_cost,
            token: fixed_hex::<20>(&request.token)?,
            decimals: request.decimals,
        };
        let body =
            GasStationClient::quote_body(&quote_request).map_err(|error| error.to_string())?;
        let signed = self
            .station
            .quote(&quote_request)
            .map_err(|error| error.to_string())?;
        let quote = signed.quote;
        Ok(json!({
            "batch": {
                "chainId": body["chainId"], "account": body["account"],
                "nonce": body["nonce"], "calls": body["calls"],
                "quote": {
                    "sponsor": evm_hex(&quote.sponsor), "token": evm_hex(&quote.token),
                    "maxTokenAmount": decimal(&quote.max_token_amount),
                    "tokenAmount": decimal(&quote.token_amount), "deadline": decimal(&quote.deadline),
                    "quoteNonce": decimal(&quote.nonce), "gasCost": decimal(&quote.gas_cost),
                    "decimals": signed.decimals,
                },
            },
            "relayerSignature": evm_hex(&signed.signature),
        }))
    }

    fn sign(&self, request: &SigningRequest) -> Result<serde_json::Value, String> {
        let batch = &request.batch;
        let account = fixed_hex::<20>(batch["account"].as_str().ok_or_else(|| refused(()))?)?;
        let calls = batch["calls"].as_array().ok_or_else(|| refused(()))?;
        let first = calls.first().ok_or_else(|| refused(()))?;
        if first["to"] != ADDR || first["value"] != "0" {
            return Err(refused(()));
        }
        let data = hex_data(first["data"].as_str().ok_or_else(|| refused(()))?)?;
        if data.len() != 164 || data[..4] != Keccak256::digest(b"bindLayerX(bytes32,bytes)")[..4] {
            return Err(refused(()));
        }
        let public_key: [u8; 32] = data[4..36].try_into().map_err(refused)?;
        let signature: [u8; 64] = data[100..164].try_into().map_err(refused)?;
        if data != bindLayerX(public_key, signature) {
            return Err(refused(()));
        }
        let body = json!({
            "batch": batch, "call": request.call, "authorization": request.authorization,
            "accountSignature": request.account_signature, "relayerSignature": request.relayer_signature,
        });
        let receipt = self
            .station
            .submit(&body)
            .map_err(|error| error.to_string())?;
        match receipt["rung"].as_str() {
            Some("submitted") => Ok(json!({"complete": false, "receipt": receipt})),
            Some("receipted" | "committed") => {
                if receipt["status"] != "1" || !LayerXBound(&receipt, account, public_key)? {
                    return Err(
                        "onboarding receipt has no successful matching LayerXBound log".to_owned(),
                    );
                }
                Ok(json!({"complete": true, "receipt": receipt}))
            }
            _ => Err("onboarding gas station returned no batch receipt rung".to_owned()),
        }
    }
}

#[allow(non_snake_case)]
fn bindLayerX(public_key: [u8; 32], signature: [u8; 64]) -> Vec<u8> {
    let mut data = Keccak256::digest(b"bindLayerX(bytes32,bytes)")[..4].to_vec();
    data.extend_from_slice(&public_key);
    data.extend_from_slice(&word(64));
    data.extend_from_slice(&word(64));
    data.extend_from_slice(&signature);
    data
}

#[allow(non_snake_case)]
fn LayerXBound(
    receipt: &serde_json::Value,
    account: [u8; 20],
    public_key: [u8; 32],
) -> Result<bool, String> {
    let topic = evm_hex(&Keccak256::digest(b"LayerXBound(address,bytes32,uint64)"));
    let mut evm = [0; 32];
    evm[12..].copy_from_slice(&account);
    let logs = receipt["logs"].as_array().ok_or_else(|| refused(()))?;
    for log in logs {
        let Some(topics) = log["topics"].as_array() else {
            return Err(refused(()));
        };
        if log["address"] == ADDR
            && topics.len() == 3
            && topics[0] == topic
            && topics[1] == evm_hex(&evm)
            && topics[2] == evm_hex(&public_key)
            && log.get("removed") != Some(&json!(true))
        {
            let nonce = fixed_hex::<32>(log["data"].as_str().ok_or_else(|| refused(()))?)?;
            if nonce[..24] == [0; 24] {
                return Ok(true);
            }
        }
    }
    Ok(false)
}

fn hex_data(raw: &str) -> Result<Vec<u8>, String> {
    layerx_platform_internal::secret::unhex(raw.strip_prefix("0x").ok_or_else(|| refused(()))?)
        .ok_or_else(|| refused(()))
}

fn fixed_hex<const N: usize>(raw: &str) -> Result<[u8; N], String> {
    if raw.len() != 2 + N * 2 {
        return Err(refused(()));
    }
    hex_data(raw)?.try_into().map_err(refused)
}

fn word(value: u128) -> [u8; 32] {
    let mut result = [0; 32];
    result[16..].copy_from_slice(&value.to_be_bytes());
    result
}

fn decimal_word(raw: &str) -> Result<[u8; 32], String> {
    if raw.is_empty()
        || raw.len() > 78
        || (raw.len() > 1 && raw.starts_with('0'))
        || !raw.bytes().all(|byte| byte.is_ascii_digit())
    {
        return Err(refused(()));
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
            return Err(refused(()));
        }
    }
    Ok(result)
}

fn decimal(value: &[u8; 32]) -> String {
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

fn evm_hex(bytes: &[u8]) -> String {
    format!("0x{}", layerx_platform_internal::secret::hex(bytes))
}
