//! Audit 22.30.89 contract proofs. The producers/consumers are production modules;
//! their remote peers are bounded loopback services and committed ABI vectors.
use layerx_platform_cli::{
    config::Configuration, credential, wallet_gateway as producer, wallet_reads, wallet_send,
    wallet_signing::SigningFacts,
};
use layerx_sdk::rpc::RpcClient as Sdk;
use serde_json::{json, Value};
use std::io::{Read, Write};
use std::net::TcpListener;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::time::Duration;

type Outcome = Result<(), Box<dyn std::error::Error>>;
fn debug_error(error: impl std::fmt::Debug) -> Box<dyn std::error::Error> {
    std::io::Error::other(format!("{error:?}")).into()
}

struct Server {
    endpoint: String,
    stop: Arc<AtomicBool>,
    thread: Option<std::thread::JoinHandle<()>>,
}
impl Server {
    fn start(mut answer: impl FnMut(Value) -> Value + Send + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let endpoint = format!("http://{}/rpc", listener.local_addr().unwrap());
        listener.set_nonblocking(true).unwrap();
        let stop = Arc::new(AtomicBool::new(false));
        let closing = stop.clone();
        let thread = std::thread::spawn(move || {
            while !closing.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(socket) => socket,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("loopback accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let mut header = Vec::new();
                while !header.ends_with(b"\r\n\r\n") {
                    assert!(header.len() < 8192);
                    let mut byte = [0];
                    stream.read_exact(&mut byte).unwrap();
                    header.push(byte[0]);
                }
                let header = std::str::from_utf8(&header).unwrap();
                assert!(header.starts_with("POST /rpc HTTP/1.1\r\n"));
                let length = header
                    .lines()
                    .find_map(|line| {
                        let (key, value) = line.split_once(':')?;
                        key.eq_ignore_ascii_case("content-length")
                            .then(|| value.trim().parse::<usize>().unwrap())
                    })
                    .unwrap();
                assert!(length < 1_048_576);
                let mut body = vec![0; length];
                stream.read_exact(&mut body).unwrap();
                let request: Value = serde_json::from_slice(&body).unwrap();
                let result = answer(request.clone());
                let reply = json!({"jsonrpc":"2.0","id":request["id"],"result":result});
                let body = serde_json::to_vec(&reply).unwrap();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap();
                stream.write_all(&body).unwrap();
            }
        });
        Self {
            endpoint,
            stop,
            thread: Some(thread),
        }
    }
}
impl Drop for Server {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(thread) = self.thread.take() {
            thread.join().unwrap();
        }
    }
}
fn bytes(text: &str) -> Vec<u8> {
    layerx_platform_cli::encoding::hex_decode("fixture", text).unwrap()
}
fn word(value: u128) -> Vec<u8> {
    let mut out = vec![0; 16];
    out.extend_from_slice(&value.to_be_bytes());
    out
}
fn did() -> String {
    format!("did:layerx:{}", "61".repeat(32))
}
fn native() -> [u8; 32] {
    let mut asset = [0; 32];
    asset[0] = 1;
    asset
}
fn account_id(did: &str, asset: [u8; 32]) -> [u8; 32] {
    let name = layerx_wire::account::account_name_for_asset(did, asset, native()).unwrap();
    layerx_wire::hash::account_id_for_protocol(&name, 3).unwrap()
}
fn abi_fixture(name: &str) -> Value {
    let fixture: Value = serde_json::from_str(include_str!(
        "../../../../services/platform/hosted/gateway/tests/fixtures/paxeer-abi-vectors.json"
    ))
    .unwrap();
    fixture["cases"]
        .as_array()
        .unwrap()
        .iter()
        .find(|row| row["name"] == name)
        .unwrap()
        .clone()
}

#[test]
fn wallet_balance_shape() -> Outcome {
    let fixture = abi_fixture("bank.all_balances");
    let coins = producer::evm::decode_coins(&bytes(fixture["result"].as_str().unwrap()))
        .map_err(debug_error)?;
    let rows = coins.into_iter().map(|(denom, amount)| json!({"denom":denom,"amount":producer::evm::uint256_decimal(&amount)})).collect();
    let address = layerx_platform_cli::encoding::fixed_hex::<20>(
        "fixture address",
        fixture["in"]["acc"]
            .as_str()
            .unwrap()
            .strip_prefix("0x")
            .unwrap(),
    )?;
    let document = producer::account_document(
        Some(address),
        Some("pax1exampleaccount".into()),
        Some([0x61; 32]),
        Some(account_id(&did(), native())),
    );
    let expected = producer::balances(document, 91, rows);
    let unchanged = expected.clone();
    let server = Server::start(move |request| {
        assert_eq!(request["method"], "px_getBalances");
        assert_eq!(request["params"], json!([did()]));
        unchanged.clone()
    });
    let sdk = Sdk::connect(&server.endpoint, None).map_err(debug_error)?;
    let result = wallet_reads::balances(
        sdk.get_balances(&did()).map_err(debug_error)?.into_value(),
        &did(),
        None,
    )?;
    assert_eq!(result, expected);
    assert_eq!(result["balances"], fixture["out"]["response"]);
    assert_eq!(result["height"], 91);
    assert_eq!(result["units"], "base");
    assert!(result.get("verification").is_none());
    assert!(wallet_reads::balances(result.clone(), "did:layerx:someoneelse", None).is_err());
    let filtered = wallet_reads::balances(result.clone(), &did(), Some(native()))?;
    assert_eq!(filtered["balances"], json!([]));
    let mut invalid = result;
    invalid["balances"][0]["amount"] = json!("01");
    assert!(wallet_reads::balances(invalid, &did(), None).is_err());
    Ok(())
}

#[test]
fn wallet_account_shape() -> Outcome {
    let owner = ed25519_dalek::SigningKey::from_bytes(&[0x11; 32])
        .verifying_key()
        .to_bytes();
    let actor = format!("did:layerx:{}", producer::hex(&owner));
    let asset = native();
    let from = account_id(&actor, asset);
    let source_actor = actor.clone();
    let mut source_sequence = 7;
    let server = Server::start(move |request| match request["method"].as_str().unwrap() {
        "eth_getTransactionCount" => {
            assert_eq!(request["params"][1], "pending");
            json!("0x29")
        }
        "px_getAccount" => {
            assert_eq!(
                request["params"],
                json!([source_actor, producer::hex(&asset)])
            );
            let result =
                producer::ledger_account(&source_actor, &producer::hex(&asset), 73, &mut |data| {
                    match &data[..4] {
                        [0x3f, 0x05, 0x0c, 0xbc] => {
                            assert_eq!(&data[4..36], &owner);
                            assert_eq!(&data[36..], &asset);
                            Ok(from.to_vec())
                        }
                        [0x37, 0x95, 0x5a, 0xb2] => {
                            assert_eq!(&data[4..], &from);
                            Ok(owner.to_vec())
                        }
                        [0xce, 0x2f, 0x92, 0x1f] => {
                            assert_eq!(&data[4..36], &from);
                            assert_eq!(&data[36..], &asset);
                            Ok(word(source_sequence))
                        }
                        [0xb1, 0xff, 0xc0, 0x3e] => Ok(word(1_000)),
                        selector => panic!("unexpected native selector: {selector:?}"),
                    }
                })
                .unwrap();
            source_sequence += 1;
            result
        }
        method => panic!("unexpected method {method}"),
    });
    let sdk = Sdk::connect(&server.endpoint, None).map_err(debug_error)?;
    let evm_nonce = sdk
        .get_identity_sequence("0x102132435465768798a9bacbdcedfe0f1e2d3c4b")
        .map_err(debug_error)?;
    let facts = SigningFacts {
        actor: &actor,
        public_key: owner,
        network_id: 125,
        identity_next_sequence: evm_nonce,
        not_before_ms: 1_000,
        expires_at_ms: 2_000,
        fee_limit: 1_000_000,
        idempotency_key: [4; 32],
    };
    let first = wallet_send::native_send_debit(&sdk, &facts, asset, [8; 32], 10)?;
    let second = wallet_send::native_send_debit(&sdk, &facts, asset, [8; 32], 10)?;
    assert_eq!(first.from, from);
    assert_eq!(first.source_sequence, 7);
    assert_eq!(second.source_sequence, 8);
    assert_eq!(facts.identity_next_sequence, 41);
    assert_ne!(
        first.authorization_message()?,
        second.authorization_message()?
    );
    let signer = layerx_crypto::signer::LocalSigner::new([0x11; 32]);
    let mut signing = std::pin::pin!(first.sign(&signer));
    let mut context = std::task::Context::from_waker(std::task::Waker::noop());
    let std::task::Poll::Ready(payload) = std::future::Future::poll(signing.as_mut(), &mut context)
    else {
        panic!("local signer must complete");
    };
    let payload = payload.map_err(debug_error)?;
    let call = wallet_send::payment_call(
        &payload,
        &layerx_crypto::payments::CallOptions {
            from: [9; 20],
            chain_id: 125,
            nonce: evm_nonce,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 200_000,
        },
    )?;
    assert_eq!(call.nonce, 41);
    assert_eq!(&call.to[18..], &[0x10, 0x1a]);
    let mut snapshot = sdk
        .get_ledger_account(&actor, asset)
        .map_err(debug_error)?
        .into_value();
    snapshot["owner"] = json!("62".repeat(32));
    assert!(layerx_sdk::rpc::LedgerAccountSnapshot::decode(snapshot, &actor, asset).is_err());
    assert!(wallet_send::native_send_debit(&sdk, &facts, asset, from, 10).is_err());
    Ok(())
}

#[test]
fn wallet_assets_shape() -> Outcome {
    // Bank custody rows are the current hosted product; no native supply/proof fields are invented.
    let assets: Vec<_> = (1..=18).map(|id| json!({"asset_id":format!("{:064x}", 19 - id),"kind":"bank","denom":format!("factory/asset{id}"),
        "name":format!("Asset {id}"),"symbol":"UNIT","decimals":6,"supply":"1000000"})).collect();
    let source = assets.clone();
    let server = Server::start(move |request| {
        assert_eq!(request["method"], "px_listAssets");
        let (cursor, limit) = producer::page_params(Some(&request["params"])).unwrap();
        // Committed in-memory indexer peer: registration positions are independent of asset IDs.
        let start = usize::try_from(cursor.unwrap_or(0)).unwrap();
        let end = (start + limit).min(source.len());
        let next = (end < source.len()).then_some(end as u64);
        producer::assets_page(source[start..end].to_vec(), 94, cursor, limit, next).unwrap()
    });
    let sdk = Sdk::connect(&server.endpoint, None).map_err(debug_error)?;
    assert!(matches!(
        sdk.list_assets_page(None, None),
        Err(layerx_sdk::rpc::RpcError::MissingFinalityTrust)
    ));
    let first = sdk
        .list_custody_assets_page(None, None)
        .map_err(debug_error)?;
    assert_eq!(first.assets, assets[..16]);
    assert_eq!(first.height, 94);
    let next = first.next_cursor.unwrap();
    assert_eq!(next, 16);
    let second = sdk
        .list_custody_assets_page(Some(next), None)
        .map_err(debug_error)?;
    assert_eq!(second.assets, assets[16..]);
    assert_eq!(second.next_cursor, None);
    assert!(first.clone().into_value().get("state_root").is_none());
    let info = sdk
        .get_custody_asset(producer::key(assets[17]["asset_id"].as_str().unwrap())?)
        .map_err(debug_error)?;
    assert_eq!(info["asset"], assets[17]);
    assert_eq!(info["height"], 94);
    let explicit = sdk
        .list_custody_assets_page(None, Some(2))
        .map_err(debug_error)?;
    assert_eq!(explicit.assets, assets[..2]);
    assert!(sdk.list_custody_assets_page(None, Some(17)).is_err());
    assert!(producer::page_params(Some(&json!([null, 0]))).is_err());
    let mut invalid = first.clone().into_value();
    invalid["next_cursor"] = json!("016");
    assert!(layerx_sdk::rpc::CustodyAssetPage::decode(invalid, None, 16).is_err());
    for bad in [
        json!(["0"]),
        json!(["01"]),
        json!(["9223372036854775808"]),
        json!([null, 17]),
    ] {
        assert!(producer::page_params(Some(&bad)).is_err());
    }
    let mut repeated_cursor = first.clone().into_value();
    repeated_cursor["next_cursor"] = json!("16");
    assert!(layerx_sdk::rpc::CustodyAssetPage::decode(repeated_cursor, Some(16), 16).is_err());
    let mut duplicate = first.clone().into_value();
    duplicate["assets"][1] = duplicate["assets"][0].clone();
    assert!(layerx_sdk::rpc::CustodyAssetPage::decode(duplicate, None, 16).is_err());
    assert!(producer::assets_page(assets[..2].to_vec(), 94, Some(2), 2, Some(2)).is_err());
    assert!(producer::assets_page(
        vec![assets[0].clone(), assets[0].clone()],
        94,
        None,
        2,
        None
    )
    .is_err());
    let id = [0x11; 32];
    let mut abi = Vec::new();
    for head in [
        id.to_vec(),
        word(384),
        word(448),
        word(6),
        word(1_000_000),
        vec![0x22; 32],
        vec![0x33; 32],
        word(1),
        word(0),
        word(512),
        word(1),
        word(42),
    ] {
        abi.extend(head);
    }
    abi.extend(word(3));
    abi.extend(b"LED");
    abi.extend([0; 29]);
    abi.extend(word(12));
    abi.extend(b"Ledger Asset");
    abi.extend([0; 20]);
    abi.extend(word(0));
    let mut row = json!({"asset_id":producer::hex(&id),"kind":"ledger","denom":format!("layerx/{}",producer::hex(&id))});
    row.as_object_mut().unwrap().extend(
        producer::ledger_asset_metadata(&abi, id)?
            .as_object()
            .unwrap()
            .clone(),
    );
    assert_eq!(row["paused"], true);
    assert_eq!(row["supply"], "42");
    assert_eq!(row["decimals"], 6);
    let native_page = producer::assets_page(vec![row.clone()], 94, None, 16, None)?;
    let native_server = Server::start(move |_| native_page.clone());
    let native_sdk = Sdk::connect(&native_server.endpoint, None).map_err(debug_error)?;
    assert_eq!(
        native_sdk
            .list_custody_assets_page(None, None)
            .map_err(debug_error)?
            .assets,
        vec![row]
    );
    Ok(())
}

#[test]
fn wallet_bind_rpc() -> Outcome {
    use layerx_client::account_binding::{plan_bind, sign_bind_transaction, BindFees, BindPlan};
    use layerx_crypto::account_derivation::derive_from_mnemonic;
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
    let account = derive_from_mnemonic("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", "", 0)?;
    let BindPlan::Bind(call) = plan_bind(&account, 125, None, 7)? else {
        panic!("unbound fixture")
    };
    call.signed_binding().verify().map_err(debug_error)?;
    let expected = sign_bind_transaction(
        &account,
        125,
        &call,
        BindFees {
            evm_nonce: 41,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 5,
            gas_limit: 200_000,
        },
    )?;
    let hash = format!("0x{}", producer::hex(&transaction_hash(&expected)));
    let expected_hash = hash.clone();
    let raw = expected.clone();
    let address = account.evm_address();
    let public = account.layerx_public_key();
    let calldata = call.data().to_vec();
    let submitted = Arc::new(std::sync::atomic::AtomicUsize::new(0));
    let count = submitted.clone();
    let mut bound = false;
    let server = Server::start(move |request| {
        match request["method"].as_str().unwrap() {
            "eth_chainId" => json!("0x7d"),
            "eth_call" => {
                let data = bytes(
                    request["params"][0]["data"]
                        .as_str()
                        .unwrap()
                        .strip_prefix("0x")
                        .unwrap(),
                );
                assert_eq!(&data[16..], &address);
                assert_eq!(request["params"][1], "latest");
                let answer = match &data[..4] {
                    [0x35, 0x7f, 0xee, 0xd6] => {
                        // IAddr ABI: address, string offset, DID, main account, dynamic pax address.
                        let mut answer = vec![0; 160];
                        answer[12..32].copy_from_slice(&address);
                        answer[63] = 128;
                        if bound {
                            answer[64..96].copy_from_slice(&public);
                        }
                        answer
                    }
                    [0xce, 0xdd, 0x9b, 0xa2] => bytes(
                        abi_fixture("addr.layerXBindNonce")["result"]
                            .as_str()
                            .unwrap(),
                    ),
                    data => panic!("unexpected native read {data:?}"),
                };
                json!(format!("0x{}", producer::hex(&answer)))
            }
            "eth_getTransactionCount" => {
                assert_eq!(request["params"][1], "pending");
                json!("0x29")
            }
            "eth_gasPrice" => json!("0x2"),
            "eth_maxPriorityFeePerGas" => json!("0x1"),
            "eth_estimateGas" => {
                assert_eq!(request["params"][1], "pending");
                let tx = &request["params"][0];
                assert_eq!(tx["type"], "0x2");
                assert_eq!(tx["chainId"], "0x7d");
                assert_eq!(tx["nonce"], "0x29");
                assert_eq!(tx["maxPriorityFeePerGas"], "0x1");
                assert_eq!(tx["maxFeePerGas"], "0x5");
                assert_eq!(tx["value"], "0x0");
                assert_eq!(tx["accessList"], json!([]));
                assert_eq!(tx["data"], format!("0x{}", producer::hex(&calldata)));
                assert_eq!(tx["from"], format!("0x{}", producer::hex(&address)));
                assert_eq!(tx["to"], "0x0000000000000000000000000000000000001004");
                json!("0x30d40")
            }
            "eth_sendRawTransaction" => {
                assert_eq!(
                    request["params"],
                    json!([format!("0x{}", producer::hex(&raw))])
                );
                let (tx, signature) = decode_signed(&raw).unwrap();
                assert_eq!(recover_sender(&tx, &signature).unwrap(), address);
                assert_eq!(tx.data, calldata);
                assert_eq!(tx.nonce, 41);
                bound = true;
                count.fetch_add(1, Ordering::SeqCst);
                json!(expected_hash)
            }
            method => panic!("unexpected method {method}"),
        }
    });
    let client = layerx_platform_cli::rpc::RpcClient::new(&server.endpoint, None)?;
    let result = layerx_platform_cli::wallet_derive::bind(&client, &account)?;
    assert_eq!(result["transaction_hash"], hash);
    assert_eq!(result["bind_nonce"], 7);
    let again = layerx_platform_cli::wallet_derive::bind(&client, &account)?;
    assert_eq!(again["status"], "already_bound");
    assert_eq!(submitted.load(Ordering::SeqCst), 1);
    assert!(layerx_platform_cli::rpc::request(
        "eth_call",
        &json!([{"to":"0x01","data":"0x"},"latest"])
    )
    .is_err());
    assert!(layerx_platform_cli::rpc::decode_response(
        "eth_chainId",
        &json!({"jsonrpc":"2.0","id":1,"result":"0x07d"})
    )
    .is_err());
    assert!(layerx_platform_cli::rpc::decode_response(
        "eth_call",
        &json!({"jsonrpc":"2.0","id":1,"result":"0x0"})
    )
    .is_err());
    Ok(())
}

// A Secret Service wire peer. The native zbus/keyring backend and its DH/AES codec
// run unchanged. The peer retains opaque encrypted secret tuples in memory.
#[derive(Clone, Debug, PartialEq)]
enum BusValue {
    Byte(u8),
    Uint(u32),
    Bool(bool),
    Text(String),
    Path(String),
    Signature(String),
    Bytes(Vec<u8>),
    Array(String, Vec<BusValue>),
    Tuple(Vec<BusValue>),
    Variant(Box<BusValue>),
}
impl BusValue {
    fn text(&self) -> &str {
        match self {
            Self::Text(text) | Self::Path(text) | Self::Signature(text) => text,
            _ => panic!("expected text: {self:?}"),
        }
    }
    fn values(&self) -> &[Self] {
        match self {
            Self::Array(_, values) | Self::Tuple(values) => values,
            _ => panic!("expected array: {self:?}"),
        }
    }
    fn inner(&self) -> &Self {
        match self {
            Self::Variant(value) => value,
            _ => panic!("expected variant: {self:?}"),
        }
    }
    fn signature(&self) -> String {
        match self {
            Self::Byte(_) => "y".into(),
            Self::Uint(_) => "u".into(),
            Self::Bool(_) => "b".into(),
            Self::Text(_) => "s".into(),
            Self::Path(_) => "o".into(),
            Self::Signature(_) => "g".into(),
            Self::Bytes(_) => "ay".into(),
            Self::Array(kind, _) => format!("a{kind}"),
            Self::Tuple(values) => format!(
                "({})",
                values.iter().map(Self::signature).collect::<String>()
            ),
            Self::Variant(_) => "v".into(),
        }
    }
    fn encode(&self, out: &mut Vec<u8>) {
        match self {
            Self::Byte(value) => out.push(*value),
            Self::Uint(value) => {
                pad(out, 4);
                out.extend_from_slice(&value.to_le_bytes());
            }
            Self::Bool(value) => Self::Uint(u32::from(*value)).encode(out),
            Self::Text(text) | Self::Path(text) => {
                pad(out, 4);
                out.extend_from_slice(&(text.len() as u32).to_le_bytes());
                out.extend_from_slice(text.as_bytes());
                out.push(0);
            }
            Self::Signature(text) => {
                out.push(text.len() as u8);
                out.extend_from_slice(text.as_bytes());
                out.push(0);
            }
            Self::Bytes(bytes) => {
                pad(out, 4);
                out.extend_from_slice(&(bytes.len() as u32).to_le_bytes());
                out.extend_from_slice(bytes);
            }
            Self::Tuple(values) => {
                pad(out, 8);
                for value in values {
                    value.encode(out);
                }
            }
            Self::Variant(value) => {
                Self::Signature(value.signature()).encode(out);
                value.encode(out);
            }
            Self::Array(kind, values) => {
                pad(out, 4);
                let size = out.len();
                out.extend_from_slice(&[0; 4]);
                pad(out, alignment(kind.as_bytes()[0]));
                let start = out.len();
                for value in values {
                    value.encode(out);
                }
                let length = (out.len() - start) as u32;
                out[size..size + 4].copy_from_slice(&length.to_le_bytes());
            }
        }
    }
}
fn pad(out: &mut Vec<u8>, align: usize) {
    while out.len() % align != 0 {
        out.push(0);
    }
}
fn aligned(offset: usize, align: usize) -> usize {
    (offset + align - 1) / align * align
}
fn alignment(kind: u8) -> usize {
    match kind {
        b'y' | b'g' | b'v' => 1,
        b'(' | b'{' => 8,
        _ => 4,
    }
}
fn type_end(sig: &[u8], pos: usize) -> usize {
    match sig[pos] {
        b'a' => type_end(sig, pos + 1),
        b'(' | b'{' => {
            let close = if sig[pos] == b'(' { b')' } else { b'}' };
            let mut end = pos + 1;
            while sig[end] != close {
                end = type_end(sig, end);
            }
            end + 1
        }
        _ => pos + 1,
    }
}
fn parse_value(sig: &[u8], pos: &mut usize, data: &[u8], offset: &mut usize) -> BusValue {
    let kind = sig[*pos];
    *pos += 1;
    *offset = aligned(*offset, alignment(kind));
    match kind {
        b'y' => {
            let value = data[*offset];
            *offset += 1;
            BusValue::Byte(value)
        }
        b'u' | b'b' => {
            let value = u32::from_le_bytes(data[*offset..*offset + 4].try_into().unwrap());
            *offset += 4;
            if kind == b'u' {
                BusValue::Uint(value)
            } else {
                BusValue::Bool(value != 0)
            }
        }
        b's' | b'o' | b'g' => {
            let length = if kind == b'g' {
                let length = usize::from(data[*offset]);
                *offset += 1;
                length
            } else {
                let length =
                    u32::from_le_bytes(data[*offset..*offset + 4].try_into().unwrap()) as usize;
                *offset += 4;
                length
            };
            let text = String::from_utf8(data[*offset..*offset + length].to_vec()).unwrap();
            *offset += length + 1;
            match kind {
                b's' => BusValue::Text(text),
                b'o' => BusValue::Path(text),
                _ => BusValue::Signature(text),
            }
        }
        b'v' => {
            let text = parse_value(b"g", &mut 0, data, offset);
            BusValue::Variant(Box::new(parse_value(
                text.text().as_bytes(),
                &mut 0,
                data,
                offset,
            )))
        }
        b'(' | b'{' => {
            let end = if kind == b'(' { b')' } else { b'}' };
            let mut values = Vec::new();
            while sig[*pos] != end {
                values.push(parse_value(sig, pos, data, offset));
            }
            *pos += 1;
            BusValue::Tuple(values)
        }
        b'a' => {
            let length =
                u32::from_le_bytes(data[*offset..*offset + 4].try_into().unwrap()) as usize;
            *offset += 4;
            let start = *pos;
            let end = type_end(sig, start);
            *pos = end;
            *offset = aligned(*offset, alignment(sig[start]));
            let limit = *offset + length;
            if sig[start] == b'y' {
                let bytes = data[*offset..limit].to_vec();
                *offset = limit;
                return BusValue::Bytes(bytes);
            }
            let mut values = Vec::new();
            while *offset < limit {
                values.push(parse_value(sig, &mut { start }, data, offset));
            }
            assert_eq!(*offset, limit);
            BusValue::Array(
                std::str::from_utf8(&sig[start..end]).unwrap().to_owned(),
                values,
            )
        }
        _ => panic!("unhandled D-Bus signature {}", String::from_utf8_lossy(sig)),
    }
}
fn bus_text(value: &str) -> BusValue {
    BusValue::Text(value.into())
}
fn bus_path(value: &str) -> BusValue {
    BusValue::Path(value.into())
}
fn variant(value: BusValue) -> BusValue {
    BusValue::Variant(Box::new(value))
}
fn dict(rows: Vec<(&str, BusValue)>) -> BusValue {
    BusValue::Array(
        "{sv}".into(),
        rows.into_iter()
            .map(|(key, value)| BusValue::Tuple(vec![bus_text(key), variant(value)]))
            .collect(),
    )
}
fn lookup<'a>(value: &'a BusValue, key: &str) -> &'a BusValue {
    value
        .values()
        .iter()
        .map(BusValue::values)
        .find(|row| row[0].text() == key)
        .unwrap()[1]
        .inner()
}
fn reply_bus(
    stream: &mut std::os::unix::net::UnixStream,
    reply: u32,
    serial: u32,
    values: Vec<BusValue>,
) {
    let signature = values.iter().map(BusValue::signature).collect::<String>();
    let mut headers = Vec::new();
    for (code, value) in [
        (5, BusValue::Uint(reply)),
        (6, bus_text(":1.1")),
        (7, bus_text(":1.0")),
        (8, BusValue::Signature(signature)),
    ] {
        BusValue::Tuple(vec![BusValue::Byte(code), variant(value)]).encode(&mut headers);
    }
    let mut body = Vec::new();
    for value in values {
        value.encode(&mut body);
    }
    let mut out = vec![b'l', 2, 0, 1];
    out.extend_from_slice(&(body.len() as u32).to_le_bytes());
    out.extend_from_slice(&serial.to_le_bytes());
    out.extend_from_slice(&(headers.len() as u32).to_le_bytes());
    out.extend(headers);
    pad(&mut out, 8);
    out.extend(body);
    stream.write_all(&out).unwrap();
}
struct SecretBus {
    _directory: tempfile::TempDir,
    path: std::path::PathBuf,
    stop: Arc<AtomicBool>,
    thread: Option<std::thread::JoinHandle<()>>,
}
impl SecretBus {
    fn start() -> Self {
        use std::os::unix::net::UnixListener;
        let directory = tempfile::tempdir().unwrap();
        let path = directory.path().join("bus");
        let listener = UnixListener::bind(&path).unwrap();
        listener.set_nonblocking(true).unwrap();
        let stop = Arc::new(AtomicBool::new(false));
        let closing = stop.clone();
        let thread = std::thread::spawn(move || {
            let mut stream = loop {
                if closing.load(Ordering::Acquire) {
                    return;
                }
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1))
                    }
                    Err(error) => panic!("{error}"),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap();
            let mut byte = [0];
            stream.read_exact(&mut byte).unwrap();
            assert_eq!(byte, [0]);
            loop {
                let mut line = Vec::new();
                while !line.ends_with(b"\r\n") {
                    stream.read_exact(&mut byte).unwrap();
                    line.push(byte[0]);
                }
                let line = std::str::from_utf8(&line).unwrap();
                if line.starts_with("AUTH ") {
                    stream
                        .write_all(b"OK 0123456789abcdef0123456789abcdef\r\n")
                        .unwrap();
                } else if line.starts_with("NEGOTIATE_UNIX_FD") {
                    stream
                        .write_all(b"ERROR Unix FD passing is not supported\r\n")
                        .unwrap();
                } else if line == "BEGIN\r\n" {
                    break;
                } else {
                    panic!("unexpected SASL: {line}");
                }
            }
            stream
                .set_read_timeout(Some(Duration::from_millis(100)))
                .unwrap();
            // path -> (attributes, encrypted Secret tuple)
            let mut items = std::collections::BTreeMap::<String, (BusValue, BusValue)>::new();
            let mut serial = 1;
            let mut item_id = 1;
            loop {
                let mut header = [0; 16];
                let mut got = 0;
                while got < header.len() {
                    match stream.read(&mut header[got..]) {
                        Ok(0) => return,
                        Ok(count) => got += count,
                        Err(error)
                            if matches!(
                                error.kind(),
                                std::io::ErrorKind::WouldBlock | std::io::ErrorKind::TimedOut
                            ) =>
                        {
                            if closing.load(Ordering::Acquire) {
                                return;
                            }
                        }
                        Err(error) => panic!("bus header: {error}"),
                    }
                }
                assert_eq!(header[0], b'l');
                let size = u32::from_le_bytes(header[4..8].try_into().unwrap()) as usize;
                let reply = u32::from_le_bytes(header[8..12].try_into().unwrap());
                let fields = u32::from_le_bytes(header[12..16].try_into().unwrap()) as usize;
                assert!(size < 1_048_576 && fields < 8192);
                let mut data = vec![0; aligned(16 + fields, 8) + size];
                data[..16].copy_from_slice(&header);
                stream.read_exact(&mut data[16..]).unwrap();
                let mut cursor = 16;
                let mut method = String::new();
                let mut object = String::new();
                let mut signature = String::new();
                while cursor < 16 + fields {
                    let field = parse_value(b"(yv)", &mut 0, &data, &mut cursor);
                    let values = field.values();
                    match values[0] {
                        BusValue::Byte(1) => object = values[1].inner().text().into(),
                        BusValue::Byte(3) => method = values[1].inner().text().into(),
                        BusValue::Byte(8) => signature = values[1].inner().text().into(),
                        _ => (),
                    }
                }
                if header[1] != 1 {
                    continue;
                }
                let mut args = Vec::new();
                let mut pos = 0;
                cursor = aligned(16 + fields, 8);
                while pos < signature.len() {
                    args.push(parse_value(
                        signature.as_bytes(),
                        &mut pos,
                        &data,
                        &mut cursor,
                    ));
                }
                let result = match method.as_str() {
                    "Hello" => vec![bus_text(":1.1")],
                    "GetNameOwner" => vec![bus_text(":1.0")],
                    "NameHasOwner" => vec![BusValue::Bool(true)],
                    "AddMatch" | "RemoveMatch" => vec![],
                    "OpenSession" => {
                        assert_eq!(args[0].text(), "dh-ietf1024-sha256-aes128-cbc-pkcs7");
                        vec![
                            variant(BusValue::Bytes(vec![2])),
                            bus_path("/org/freedesktop/secrets/session/s1"),
                        ]
                    }
                    "ReadAlias" => vec![bus_path("/org/freedesktop/secrets/collection/default")],
                    "SearchItems" => {
                        let found = items
                            .iter()
                            .filter(|(_, (attrs, _))| {
                                args[0]
                                    .values()
                                    .iter()
                                    .all(|query| attrs.values().contains(query))
                            })
                            .map(|(path, _)| bus_path(path))
                            .collect();
                        vec![
                            BusValue::Array("o".into(), found),
                            BusValue::Array("o".into(), vec![]),
                        ]
                    }
                    "Get" => {
                        assert_eq!(args[1].text(), "Locked");
                        vec![variant(BusValue::Bool(false))]
                    }
                    "GetAll" => vec![dict(vec![("Locked", BusValue::Bool(false))])],
                    "CreateItem" => {
                        let attrs =
                            lookup(&args[0], "org.freedesktop.Secret.Item.Attributes").clone();
                        let path = items
                            .iter()
                            .find(|(_, (existing, _))| existing == &attrs)
                            .map(|(path, _)| path.clone())
                            .unwrap_or_else(|| {
                                let path = format!("/org/freedesktop/secrets/item/i{item_id}");
                                item_id += 1;
                                path
                            });
                        items.insert(path.clone(), (attrs, args[1].clone()));
                        vec![bus_path(&path), bus_path("/")]
                    }
                    "GetSecret" => vec![items
                        .get(&object)
                        .expect("existing Secret Service item")
                        .1
                        .clone()],
                    "SetSecret" => {
                        items.get_mut(&object).unwrap().1 = args[0].clone();
                        vec![]
                    }
                    "Delete" => {
                        assert!(items.remove(&object).is_some());
                        vec![bus_path("/")]
                    }
                    "Close" => vec![],
                    method => {
                        panic!("unhandled Secret Service operation {method}({args:?}) on {object}")
                    }
                };
                reply_bus(&mut stream, reply, serial, result);
                serial += 1;
            }
        });
        Self {
            _directory: directory,
            path,
            stop,
            thread: Some(thread),
        }
    }
}
impl Drop for SecretBus {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(thread) = self.thread.take() {
            thread.join().unwrap();
        }
    }
}

#[test]
fn credential_profile_key_loss() -> Outcome {
    let bus = SecretBus::start();
    // All environment inputs are supplied by this fixture; no external session or daemon is needed.
    std::env::set_var(
        "DBUS_SESSION_BUS_ADDRESS",
        format!("unix:path={}", bus.path.display()),
    );
    std::env::set_var("LAYERX_CREDENTIAL_STORE", "os");
    let directory = tempfile::tempdir()?;
    let a = directory.path().join("a.json");
    let b = directory.path().join("b.json");
    std::env::set_var("LAYERX_CONFIG", &a);
    let mut ca = Configuration::default();
    assert!(
        keyring::Entry::store_status().is_ok(),
        "native Secret Service backend must initialise"
    );
    let original = keyring_core::Entry::new("dev.layerx.cli", "key:same")?;
    original.set_password(&"22".repeat(32))?;
    let ma = credential::create_key(&mut ca, "same", None)?;
    let seed_a = credential::key_seed("same")?;
    std::env::set_var("LAYERX_CONFIG", &b);
    let mut cb = Configuration::default();
    let mb = credential::create_key(&mut cb, "same", None)?;
    let seed_b = credential::key_seed("same")?;
    assert_ne!(ma.public_key, mb.public_key);
    assert_ne!(*seed_a, *seed_b);
    // A stale/empty configuration cannot overwrite an orphaned existing OS entry.
    assert!(credential::create_key(&mut Configuration::default(), "same", None).is_err());
    assert_eq!(*credential::key_seed("same")?, *seed_b);
    let failed = directory.path().join("cannot-save.json");
    std::fs::create_dir(&failed)?;
    std::env::set_var("LAYERX_CONFIG", &failed);
    let mut failing = Configuration::default();
    assert!(credential::create_key(&mut failing, "same", None).is_err());
    assert!(failing.keys.is_empty());
    let failed_entry = keyring_core::Entry::new(
        "dev.layerx.cli",
        &credential::key_entry_identity(&failed, "same")?,
    )?;
    assert!(matches!(
        failed_entry.get_password(),
        Err(keyring_core::Error::NoEntry)
    ));
    std::env::set_var("LAYERX_CONFIG", &a);
    let reopened = Configuration::load()?;
    assert_eq!(reopened.keys["same"].public_key, ma.public_key);
    assert_eq!(*credential::key_seed("same")?, *seed_a);
    credential::delete_key(&mut ca, "same")?;
    std::env::set_var("LAYERX_CONFIG", &b);
    let reopened = Configuration::load()?;
    assert_eq!(reopened.keys["same"].public_key, mb.public_key);
    assert_eq!(*credential::key_seed("same")?, *seed_b);
    // Migration is explicit, copies only a public-key match, and preserves the shared legacy alias.
    let legacy = keyring_core::Entry::new("dev.layerx.cli", "key:legacy")?;
    legacy.set_password(&"11".repeat(32))?;
    let public = ed25519_dalek::SigningKey::from_bytes(&[0x11; 32])
        .verifying_key()
        .to_bytes();
    cb.keys.insert(
        "legacy".into(),
        layerx_platform_cli::config::KeyMetadata {
            did: format!("did:layerx:{}", producer::hex(&public)),
            public_key: producer::hex(&public),
        },
    );
    cb.save()?;
    credential::migrate_legacy_key(&cb, "legacy")?;
    assert_eq!(*credential::key_seed("legacy")?, [0x11; 32]);
    assert_eq!(legacy.get_password()?, "11".repeat(32));
    assert!(credential::migrate_legacy_key(&cb, "legacy").is_err());
    let other = directory.path().join("other.json");
    std::env::set_var("LAYERX_CONFIG", &other);
    let mut wrong = Configuration::default();
    wrong.keys.insert(
        "legacy".into(),
        layerx_platform_cli::config::KeyMetadata {
            did: did(),
            public_key: "61".repeat(32),
        },
    );
    assert!(credential::migrate_legacy_key(&wrong, "legacy").is_err());
    assert_eq!(legacy.get_password()?, "11".repeat(32));
    assert_eq!(original.get_password()?, "22".repeat(32));
    assert_eq!(
        credential::profile_namespace(&b)?,
        credential::profile_namespace(&directory.path().join("./b.json"))?
    );
    // Resolve parent symlinks before applying '..', matching the filesystem's configuration identity.
    let real = directory.path().join("real");
    std::fs::create_dir_all(real.join("sub"))?;
    let alias = directory.path().join("alias");
    std::os::unix::fs::symlink(real.join("sub"), &alias)?;
    assert_eq!(
        credential::profile_namespace(&alias.join("../profile.json"))?,
        credential::profile_namespace(&real.join("profile.json"))?
    );
    assert_ne!(
        credential::profile_namespace(&alias.join("../profile.json"))?,
        credential::profile_namespace(&directory.path().join("profile.json"))?
    );
    Ok(())
}
