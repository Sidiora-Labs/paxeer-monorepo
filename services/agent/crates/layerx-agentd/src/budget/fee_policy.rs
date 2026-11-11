#[cfg(test)]
use crate::budget::AssertCase as _;

use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use layerx_crypto::evm_transaction::transaction_hash as keccak256;

const fn precompile(last: u8) -> [u8; 20] {
    let mut address = [0; 20];
    address[18] = 0x10;
    address[19] = last;
    address
}
const FEE_TOKEN: [u8; 20] = precompile(0x18);
const LEDGER: [u8; 20] = precompile(0x1a);
const BANK: [u8; 20] = precompile(0x01);

use layerx_client::rpc::payments::{AssetMetadata, NativeFeePolicy as FeePolicyValue};

pub type NativeFeePolicy = layerx_client::rpc::payments::CommittedSnapshot<FeePolicyValue>;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum FeeReadError {
    Rpc(SequencerError),
    Unavailable,
    Snapshot,
    Denomination,
    Metadata,
}

impl From<SequencerError> for FeeReadError {
    fn from(error: SequencerError) -> Self {
        Self::Rpc(error)
    }
}

fn selector(signature: &[u8]) -> Vec<u8> {
    keccak256(signature)[..4].to_vec()
}

fn word(bytes: &[u8], index: usize) -> Result<[u8; 32], FeeReadError> {
    let start = index.checked_mul(32).ok_or(FeeReadError::Metadata)?;
    bytes
        .get(start..start.checked_add(32).ok_or(FeeReadError::Metadata)?)
        .ok_or(FeeReadError::Metadata)?
        .try_into()
        .map_err(|_| FeeReadError::Metadata)
}

fn small(bytes: &[u8], index: usize) -> Result<usize, FeeReadError> {
    let value = word(bytes, index)?;
    if value[..24] != [0; 24] {
        return Err(FeeReadError::Metadata);
    }
    usize::try_from(u64::from_be_bytes(
        value[24..].try_into().map_err(|_| FeeReadError::Metadata)?,
    ))
    .map_err(|_| FeeReadError::Metadata)
}

fn dynamic(
    bytes: &[u8],
    index: usize,
    head_words: usize,
    maximum: usize,
) -> Result<Vec<u8>, FeeReadError> {
    let offset = small(bytes, index)?;
    if offset < head_words * 32 || offset % 32 != 0 {
        return Err(FeeReadError::Metadata);
    }
    let length = small(bytes, offset / 32)?;
    if length > maximum {
        return Err(FeeReadError::Metadata);
    }
    let start = offset.checked_add(32).ok_or(FeeReadError::Metadata)?;
    let end = start.checked_add(length).ok_or(FeeReadError::Metadata)?;
    let padded_end = end
        .checked_add((32 - length % 32) % 32)
        .ok_or(FeeReadError::Metadata)?;
    let value = bytes.get(start..end).ok_or(FeeReadError::Metadata)?;
    if bytes
        .get(end..padded_end)
        .ok_or(FeeReadError::Metadata)?
        .iter()
        .any(|byte| *byte != 0)
    {
        return Err(FeeReadError::Metadata);
    }
    Ok(value.to_vec())
}

fn text(
    bytes: &[u8],
    index: usize,
    head_words: usize,
    maximum: usize,
) -> Result<Vec<u8>, FeeReadError> {
    let value = dynamic(bytes, index, head_words, maximum)?;
    if value.is_empty() || std::str::from_utf8(&value).is_err() {
        return Err(FeeReadError::Metadata);
    }
    Ok(value)
}

fn units(bytes: &[u8], index: usize) -> Result<u128, FeeReadError> {
    let value = word(bytes, index)?;
    if value[..16] != [0; 16] {
        return Err(FeeReadError::Metadata);
    }
    Ok(u128::from_be_bytes(
        value[16..].try_into().map_err(|_| FeeReadError::Metadata)?,
    ))
}

fn denom_call(signature: &[u8], denom: &[u8]) -> Vec<u8> {
    let mut call = selector(signature);
    call.extend([0; 31]);
    call.push(32);
    let mut length = [0; 32];
    length[24..].copy_from_slice(&(denom.len() as u64).to_be_bytes());
    call.extend(length);
    call.extend(denom);
    call.resize(4 + 64 + denom.len().div_ceil(32) * 32, 0);
    call
}

/// # Errors
/// Returns an error if the node fee read fails or the returned fee policy cannot be
/// validated.
pub fn native_fee_policy(
    endpoint: &SequencerEndpoint,
    height: u64,
) -> Result<NativeFeePolicy, FeeReadError> {
    native_fee_policy_for(endpoint, height, [0; 20])
}

/// Reads the selected fee denomination for the authenticated native transaction payer.
/// # Errors
/// Returns an error if the node fee read fails or the returned fee policy cannot be
/// validated.
pub fn native_fee_policy_for(
    endpoint: &SequencerEndpoint,
    height: u64,
    payer: [u8; 20],
) -> Result<NativeFeePolicy, FeeReadError> {
    if height == 0 || endpoint.block_number()? < height {
        return Err(FeeReadError::Snapshot);
    }
    let header = endpoint
        .block_by_number(height)?
        .ok_or(FeeReadError::Snapshot)?;
    let state_root = endpoint
        .state_root(height)
        .map_err(|_| FeeReadError::Unavailable)?;
    if header.number != height || header.hash == [0; 32] || state_root == [0; 32] {
        return Err(FeeReadError::Snapshot);
    }
    let mut call = selector(b"getFeeDenom(address)");
    call.extend([0; 12]);
    call.extend(payer);
    let answer = endpoint.call_at(height, &FEE_TOKEN, &call)?;
    let denom = text(&answer, 0, 1, 128).map_err(|_| FeeReadError::Denomination)?;
    if !denom.iter().all(u8::is_ascii_graphic) {
        return Err(FeeReadError::Denomination);
    }
    let asset = if let Some(hex) = denom.strip_prefix(b"layerx/") {
        read_native_asset_metadata(endpoint, height, hex)?
    } else {
        let symbol = endpoint.call_at(height, &BANK, &denom_call(b"symbol(string)", &denom))?;
        let decimals = endpoint.call_at(height, &BANK, &denom_call(b"decimals(string)", &denom))?;
        if decimals.len() != 32 {
            return Err(FeeReadError::Metadata);
        }
        AssetMetadata {
            asset_id: keccak256(&denom),
            symbol: text(&symbol, 0, 1, 16)?,
            decimals: u8::try_from(small(&decimals, 0)?).map_err(|_| FeeReadError::Metadata)?,
            paused: false,
            name: String::new(),
            custody_kind: 0,
            custody_reference: Vec::new(),
            supply_cap: 0,
            issuer_did: [0; 32],
            issuer_kind: 0,
            total_units: 0,
            salt: [0; 32],
        }
    };
    if asset.asset_id == [0; 32]
        || asset.decimals > 38
        || !asset.symbol.iter().all(u8::is_ascii_alphanumeric)
    {
        return Err(FeeReadError::Metadata);
    }
    if endpoint.block_by_number(height)?.as_ref() != Some(&header) {
        return Err(FeeReadError::Snapshot);
    }
    if endpoint
        .state_root(height)
        .map_err(|_| FeeReadError::Unavailable)?
        != state_root
    {
        return Err(FeeReadError::Snapshot);
    }
    Ok(NativeFeePolicy {
        observed_sequence: height,
        state_root,
        value: FeePolicyValue { version: 1, asset },
    })
}

fn read_native_asset_metadata(
    endpoint: &SequencerEndpoint,
    height: u64,
    hex: &[u8],
) -> Result<AssetMetadata, FeeReadError> {
    if hex.len() != 64 {
        return Err(FeeReadError::Denomination);
    }
    let mut id = [0; 32];
    for (out, pair) in id.iter_mut().zip(hex.chunks_exact(2)) {
        let digit = |byte: u8| match byte {
            b'0'..=b'9' => Ok(byte - b'0'),
            b'a'..=b'f' => Ok(byte - b'a' + 10),
            b'A'..=b'F' => Ok(byte - b'A' + 10),
            _ => Err(FeeReadError::Denomination),
        };
        *out = digit(pair[0])? * 16 + digit(pair[1])?;
    }
    let mut call = selector(b"getAsset(bytes32)");
    call.extend(id);
    let answer = endpoint.call_at(height, &LEDGER, &call)?;
    let returned_id = word(&answer, 0)?;
    let decimals = u8::try_from(small(&answer, 3)?).map_err(|_| FeeReadError::Metadata)?;
    let paused = small(&answer, 10)?;
    if returned_id != id || paused > 1 || answer.len() < 12 * 32 {
        return Err(FeeReadError::Metadata);
    }
    let issuer_kind = u8::try_from(small(&answer, 7)?).map_err(|_| FeeReadError::Metadata)?;
    if issuer_kind > 2 {
        return Err(FeeReadError::Metadata);
    }
    Ok(AssetMetadata {
        asset_id: returned_id,
        symbol: text(&answer, 1, 12, 16)?,
        name: String::from_utf8(dynamic(&answer, 2, 12, 32)?)
            .map_err(|_| FeeReadError::Metadata)?,
        decimals,
        supply_cap: units(&answer, 4)?,
        issuer_did: word(&answer, 5)?,
        salt: word(&answer, 6)?,
        issuer_kind,
        custody_kind: u8::try_from(small(&answer, 8)?).map_err(|_| FeeReadError::Metadata)?,
        custody_reference: dynamic(&answer, 9, 12, 128)?,
        paused: paused == 1,
        total_units: units(&answer, 11)?,
    })
}
#[cfg(test)]
pub(crate) mod tests {
    use super::*;
    use serde_json::{json, Value};
    use std::io::{BufRead, BufReader, Read, Write};
    use std::net::{TcpListener, TcpStream};
    use std::sync::{
        atomic::{AtomicBool, Ordering},
        Arc,
    };

    pub(crate) struct FeeFixture {
        pub endpoint: SequencerEndpoint,
        address: std::net::SocketAddr,
        stop: Arc<AtomicBool>,
        worker: Option<std::thread::JoinHandle<()>>,
    }

    fn put_word(bytes: &mut [u8], index: usize, value: usize) {
        bytes[index * 32 + 24..index * 32 + 32].copy_from_slice(&(value as u64).to_be_bytes());
    }

    fn string_result(value: &[u8]) -> Vec<u8> {
        let mut bytes = vec![0; 64 + value.len().div_ceil(32) * 32];
        put_word(&mut bytes, 0, 32);
        put_word(&mut bytes, 1, value.len());
        bytes[64..64 + value.len()].copy_from_slice(value);
        bytes
    }

    pub(crate) fn asset_result(id: [u8; 32], symbol: &[u8], decimals: u8, paused: bool) -> Vec<u8> {
        let mut bytes = vec![0; 12 * 32];
        bytes[..32].copy_from_slice(&id);
        put_word(&mut bytes, 1, 12 * 32);
        put_word(&mut bytes, 3, usize::from(decimals));
        put_word(&mut bytes, 10, usize::from(paused));
        let encoded = string_result(symbol);
        bytes.extend(&encoded[32..]);
        let name_offset = bytes.len();
        put_word(&mut bytes, 2, name_offset);
        bytes.extend(&string_result(b"Native asset")[32..]);
        let custody_offset = bytes.len();
        put_word(&mut bytes, 9, custody_offset);
        bytes.extend([0; 32]);
        bytes
    }

    fn fixture_result(
        request: &Value,
        blocks: &mut usize,
        root: [u8; 32],
        fault: Option<&str>,
        denom: &str,
        preferences: &[([u8; 20], String, Vec<u8>)],
    ) -> Value {
        match request["method"].as_str().assert_case("method") {
            "eth_blockNumber" => json!("0x7"),
            "eth_getBlockByNumber" => {
                assert_eq!(request["params"][0], "0x7");
                assert!(request["params"][1].is_boolean());
                if request["params"][1] == true {
                    *blocks += 1;
                }
                if fault == Some("missing") {
                    Value::Null
                } else {
                    let hash = if fault == Some("changed") && *blocks > 1 {
                        [9; 32]
                    } else {
                        [3; 32]
                    };
                    let root = if fault == Some("changed_root") && *blocks > 1 {
                        [9; 32]
                    } else {
                        root
                    };
                    json!({"number":"0x7", "hash":format!("0x{}", hex::encode(hash)),
                                    "parentHash":format!("0x{}", hex::encode([2;32])), "timestamp":"0x1",
                                    "transactionsRoot":format!("0x{}", hex::encode([4;32])),
                                    "receiptsRoot":format!("0x{}", hex::encode([5;32])),
                                    "stateRoot": if fault == Some("missing_root") { Value::Null } else { json!(format!("0x{}", hex::encode(root))) },
                                    "transactions":[]})
                }
            }
            "eth_call" => {
                assert_eq!(request["params"][1], "0x7");
                let to = request["params"][0]["to"].as_str().assert_case("to");
                let data = hex::decode(
                    request["params"][0]["data"]
                        .as_str()
                        .assert_case("data")
                        .trim_start_matches("0x"),
                )
                .assert_case("call bytes");
                let bytes = if data.starts_with(&selector(b"getFeeDenom(address)")) {
                    assert_eq!(to, format!("0x{}", hex::encode(FEE_TOKEN)));
                    assert_eq!(data.len(), 36);
                    assert_eq!(&data[4..16], &[0; 12]);
                    let preference = preferences
                        .iter()
                        .find(|(payer, _, _)| data[16..] == *payer)
                        .assert_case("recorded payer preference");
                    string_result(preference.1.as_bytes())
                } else if data.starts_with(&selector(b"getAsset(bytes32)")) {
                    assert_eq!(to, format!("0x{}", hex::encode(LEDGER)));
                    assert_eq!(data.len(), 36);
                    let preference = preferences
                        .iter()
                        .find(|(_, denom, _)| {
                            denom.strip_prefix("layerx/").is_some_and(|id| {
                                hex::decode(id).assert_case("asset id") == data[4..]
                            })
                        })
                        .assert_case("recorded registry asset");
                    preference.2.clone()
                } else if data == denom_call(b"symbol(string)", denom.as_bytes()) {
                    assert_eq!(to, format!("0x{}", hex::encode(BANK)));
                    string_result(b"PAX")
                } else {
                    assert_eq!(to, format!("0x{}", hex::encode(BANK)));
                    assert_eq!(data, denom_call(b"decimals(string)", denom.as_bytes()));
                    let mut bytes = vec![0; 32];
                    bytes[31] = 6;
                    bytes
                };
                json!(format!("0x{}", hex::encode(bytes)))
            }
            other => panic!("unexpected RPC method {other}"),
        }
    }

    impl FeeFixture {
        pub(crate) fn new(denom: &str, root: [u8; 32], asset: Vec<u8>) -> Self {
            Self::with_reply(denom, root, asset, None)
        }

        fn with_reply(
            denom: &str,
            root: [u8; 32],
            asset: Vec<u8>,
            fault: Option<&'static str>,
        ) -> Self {
            let listener = TcpListener::bind("127.0.0.1:0").assert_case("fixture listener");
            let address = listener.local_addr().assert_case("fixture address");
            let endpoint =
                SequencerEndpoint::new(&format!("http://{address}"), &[]).assert_case("endpoint");
            let stop = Arc::new(AtomicBool::new(false));
            let stopped = stop.clone();
            let denom = denom.to_owned();
            let corpus: Value = serde_json::from_slice(include_bytes!(concat!(
                env!("CARGO_MANIFEST_DIR"),
                "/tests/fixtures/repair_22_31_217/corpus.json"
            )))
            .assert_case("committed fee-token keeper answers");
            let mut preferences = vec![([0; 20], denom.clone(), asset)];
            for (seed, id) in [(0xa5, [4; 32]), (0xa6, [5; 32])] {
                let key = k256::ecdsa::SigningKey::from_bytes((&[seed; 32]).into())
                    .assert_case("payer key");
                let payer = layerx_crypto::secp256k1::evm_address(
                    key.verifying_key().to_encoded_point(false).as_bytes(),
                )
                .assert_case("payer address");
                let view = &corpus["peer_views"]["fee_assets"][hex::encode(id)];
                preferences.push((
                    payer,
                    format!("layerx/{}", hex::encode(id)),
                    hex::decode(view["metadata"].as_str().assert_case("keeper metadata"))
                        .assert_case("keeper ABI"),
                ));
            }
            let worker = std::thread::spawn(move || {
                let mut blocks = 0;
                for connection in listener.incoming() {
                    if stopped.load(Ordering::SeqCst) {
                        break;
                    }
                    let mut stream = connection.assert_case("fixture connection");
                    stream
                        .set_read_timeout(Some(std::time::Duration::from_secs(5)))
                        .assert_case("timeout");
                    let mut reader = BufReader::new(&mut stream);
                    let mut line = String::new();
                    reader.read_line(&mut line).assert_case("request line");
                    let mut length = None;
                    loop {
                        line.clear();
                        reader.read_line(&mut line).assert_case("request header");
                        if line == "\r\n" {
                            break;
                        }
                        if let Some((key, value)) = line.split_once(':') {
                            if key.eq_ignore_ascii_case("content-length") {
                                length = Some(
                                    value.trim().parse::<usize>().assert_case("content length"),
                                );
                            }
                        }
                    }
                    let mut body = vec![0; length.assert_case("body length")];
                    reader.read_exact(&mut body).assert_case("request body");
                    let request: Value =
                        serde_json::from_slice(&body).assert_case("JSON-RPC request");
                    let result =
                        fixture_result(&request, &mut blocks, root, fault, &denom, &preferences);
                    let response = serde_json::to_vec(
                        &json!({"jsonrpc":"2.0","id":request["id"],"result":result}),
                    )
                    .assert_case("response");
                    write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",response.len()).assert_case("response headers");
                    stream.write_all(&response).assert_case("response body");
                }
            });
            Self {
                endpoint,
                address,
                stop,
                worker: Some(worker),
            }
        }
    }

    impl Drop for FeeFixture {
        fn drop(&mut self) {
            self.stop.store(true, Ordering::SeqCst);
            let _ = TcpStream::connect(self.address);
            self.worker
                .take()
                .assert_case("fixture worker")
                .join()
                .assert_case("fixture succeeded");
        }
    }

    #[test]
    fn native_fee_policy_reads_distinct_payer_preferences_at_the_committed_height() {
        let fixture = FeeFixture::new("upax", [7; 32], Vec::new());
        let default = native_fee_policy(&fixture.endpoint, 7).assert_case("default denomination");
        let mut assets = Vec::new();
        for (seed, expected) in [(0xa5, [4; 32]), (0xa6, [5; 32])] {
            let key =
                k256::ecdsa::SigningKey::from_bytes((&[seed; 32]).into()).assert_case("payer");
            let payer = layerx_crypto::secp256k1::evm_address(
                key.verifying_key().to_encoded_point(false).as_bytes(),
            )
            .assert_case("address");
            let fee =
                native_fee_policy_for(&fixture.endpoint, 7, payer).assert_case("payer preference");
            assert_eq!((fee.observed_sequence, fee.state_root), (7, [7; 32]));
            assert_eq!(fee.value.asset.asset_id, expected);
            assert_ne!(fee.value.asset.asset_id, default.value.asset.asset_id);
            assets.push(fee.value.asset.asset_id);
        }
        assert_ne!(assets[0], assets[1]);
    }

    #[test]
    fn native_fee_policy_reads_registry_metadata_at_the_committed_height() {
        for paused in [false, true] {
            let fixture = FeeFixture::new(
                &format!("layerx/{}", hex::encode([1; 32])),
                [7; 32],
                asset_result([1; 32], b"PAX", 18, paused),
            );
            let fee = native_fee_policy(&fixture.endpoint, 7).assert_case("registry policy");
            assert_eq!(
                (fee.observed_sequence, fee.state_root, fee.value.version),
                (7, [7; 32], 1)
            );
            assert_eq!(fee.value.asset.asset_id, [1; 32]);
            assert_eq!(fee.value.asset.symbol, b"PAX");
            assert_eq!(fee.value.asset.name, "Native asset");
            assert_eq!(fee.value.asset.decimals, 18);
            assert_eq!(fee.value.asset.paused, paused);
        }
    }

    #[test]
    fn native_fee_policy_reads_bank_metadata_and_hashes_the_denomination() {
        let fixture = FeeFixture::new("upax", [7; 32], Vec::new());
        let fee = native_fee_policy(&fixture.endpoint, 7).assert_case("bank policy");
        assert_eq!(fee.value.asset.asset_id, keccak256(b"upax"));
        assert_eq!(fee.value.asset.symbol, b"PAX");
        assert_eq!(fee.value.asset.decimals, 6);
        assert!(!fee.value.asset.paused);
        assert_eq!(
            (fee.observed_sequence, fee.state_root, fee.value.version),
            (7, [7; 32], 1)
        );
    }

    #[test]
    fn native_fee_policy_refuses_malformed_denominations_and_registry_records() {
        for denom in [
            "",
            "layerx/01",
            "layerx/zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
            "bad denom",
        ] {
            let fixture = FeeFixture::new(denom, [7; 32], Vec::new());
            assert_eq!(
                native_fee_policy(&fixture.endpoint, 7),
                Err(FeeReadError::Denomination)
            );
        }
        let denom = format!("layerx/{}", hex::encode([1; 32]));
        for record in [
            vec![],
            asset_result([2; 32], b"PAX", 18, false),
            asset_result([1; 32], b"PAX", 39, false),
            asset_result([1; 32], b"P AX", 18, false),
            asset_result([1; 32], b"", 18, false),
        ] {
            let fixture = FeeFixture::new(&denom, [7; 32], record);
            assert_eq!(
                native_fee_policy(&fixture.endpoint, 7),
                Err(FeeReadError::Metadata)
            );
        }
        let mut record = asset_result([1; 32], b"PAX", 18, false);
        put_word(&mut record, 10, 2);
        let fixture = FeeFixture::new(&denom, [7; 32], record);
        assert_eq!(
            native_fee_policy(&fixture.endpoint, 7),
            Err(FeeReadError::Metadata)
        );
    }

    #[test]
    fn native_fee_policy_refuses_uncommitted_missing_and_changed_headers() {
        let fixture = FeeFixture::new("upax", [7; 32], Vec::new());
        for height in [0, 8] {
            assert_eq!(
                native_fee_policy(&fixture.endpoint, height),
                Err(FeeReadError::Snapshot)
            );
        }
        let fixture = FeeFixture::new("upax", [0; 32], Vec::new());
        assert_eq!(
            native_fee_policy(&fixture.endpoint, 7),
            Err(FeeReadError::Snapshot)
        );
        for fault in ["missing", "changed", "changed_root"] {
            let fixture = FeeFixture::with_reply("upax", [7; 32], Vec::new(), Some(fault));
            assert_eq!(
                native_fee_policy(&fixture.endpoint, 7),
                Err(FeeReadError::Snapshot)
            );
        }
        let fixture = FeeFixture::with_reply("upax", [7; 32], Vec::new(), Some("missing_root"));
        assert_eq!(
            native_fee_policy(&fixture.endpoint, 7),
            Err(FeeReadError::Unavailable)
        );
    }
}
