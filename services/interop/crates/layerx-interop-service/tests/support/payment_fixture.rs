use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

type TestResult<T = ()> = Result<T, Box<dyn std::error::Error + Send + Sync>>;
fn wire<T>(result: Result<T, layerx_wire::WireError>) -> TestResult<T> {
    result.map_err(|error| format!("wire fixture: {error:?}").into())
}
fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut encoded = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        encoded.push(char::from(DIGITS[usize::from(byte >> 4)]));
        encoded.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    encoded
}
fn bytes(text: &str) -> TestResult<Vec<u8>> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| Ok(u8::from_str_radix(std::str::from_utf8(pair)?, 16)?))
        .collect()
}
fn recorded_case() -> TestResult<Value> {
    let fixture: Value = serde_json::from_slice(include_bytes!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )))?;
    fixture["cases"]
        .as_array()
        .ok_or("cases")?
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .cloned()
        .ok_or_else(|| "recorded payment".into())
}

// Preserve the captured payment terms, but sign a currently valid payer deadline.
// Receipt identities, log commitments and signatures are then emitted anew.
/// Produces a signed payment with a current payer deadline.
///
/// # Errors
/// Returns an error when the recorded transaction cannot be decoded or signed.
pub fn payment_case(reference: Option<[u8; 32]>) -> TestResult<Value> {
    let case = recorded_case()?;
    let raw = bytes(case["rawTransaction"].as_str().ok_or("raw transaction")?)?;
    let (mut transaction, _) = decode_signed(&raw)?;
    transaction.data[156..164].copy_from_slice(&(payment_now_millis()? + 600_000).to_be_bytes());
    if let Some(reference) = reference {
        transaction.data[164..196].copy_from_slice(&reference);
    }
    transaction_case(&transaction.sign(&[0x3b; 32])?)
}

/// Produces receipt evidence for the signed payment bytes.
///
/// # Errors
/// Returns an error when the transaction or receipt cannot be decoded.
///
/// # Panics
/// Panics when the transaction does not match the expected chain and call shape.
pub fn transaction_case(raw: &[u8]) -> TestResult<Value> {
    let mut case = recorded_case()?;
    let (transaction, signature) = decode_signed(raw)?;
    assert_eq!(transaction.chain_id, 125);
    assert_eq!(&transaction.data[..4], &[0x78, 0xea, 0x11, 0xdb]);
    assert_eq!(transaction.data.len(), 196);
    let sender = recover_sender(&transaction, &signature)?;
    let hash = transaction_hash(raw);
    let word = |index: usize| &transaction.data[4 + index * 32..4 + (index + 1) * 32];
    let log = json!({
        "address": case["receiptJson"]["logs"][0]["address"],
        "topics": [case["receiptJson"]["logs"][0]["topics"][0],
            format!("0x{}", hex(word(5))), format!("0x{}", hex(word(0))),
            format!("0x{}", hex(word(1)))],
        "data": format!("0x{}", hex(&[word(2), word(3)].concat())), "logIndex": "0x0"
    });
    let receipt = emit_node_receipt(raw, &log)?;
    let parsed = wire(layerx_wire::receipt::decode(&receipt))?;
    let protocol = parsed.protocol().ok_or("protocol receipt")?;
    let key = SigningKey::from_bytes(&[0x72; 32]);
    let public = format!("0x{}", hex(key.verifying_key().as_bytes()));
    case["rawTransaction"] = json!(format!("0x{}", hex(raw)));
    case["signedReceipt"] = json!(format!("0x{}", hex(&receipt)));
    case["receiptDigest"] = json!(format!(
        "0x{}",
        hex(&layerx_interop_gateway::evidence::canonical_receipt_digest(
            &receipt
        )?)
    ));
    case["prevSeqHash"] = json!(format!("0x{}", "00".repeat(32)));
    case["pinnedKey"]["publicKey"] = json!(public);
    let view = &mut case["receiptJson"];
    view["txHash"] = json!(format!("0x{}", hex(&hash)));
    view["nonce"] = json!(format!("{:#x}", transaction.nonce));
    view["seq"] = json!("0x1");
    view["index"] = json!("0x0");
    view["timestamp"] = json!(format!("{:#x}", protocol.timestamp()));
    view["receipt"] = json!(format!("0x{}", hex(&receipt)));
    view["from"] = json!(format!("0x{}", hex(&sender)));
    view["logs"] = json!([log]);
    view["signingKey"] = json!(public);
    Ok(case)
}

fn payment_now_millis() -> Result<u64, String> {
    let elapsed = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_err(|error| error.to_string())?;
    u64::try_from(elapsed.as_millis()).map_err(|error| error.to_string())
}

/// Encodes a sequencer receipt for the signed payment and log.
///
/// # Errors
/// Returns an error when the transaction, log, or receipt cannot be decoded or encoded.
///
/// # Panics
/// Panics when recorded receipt fields are absent or have an unexpected shape.
pub fn emit_node_receipt(raw: &[u8], log: &Value) -> TestResult<Vec<u8>> {
    use layerx_wire::encode::Encoder;
    // Sequencer layout from rpc/sequencer_receipt.go and the canonical
    // ProtocolReceipt encoder in layerx-wire; this isolated fixture starts at seq 1.
    let recorded = recorded_case()?;
    let old = wire(layerx_wire::receipt::decode(&bytes(
        recorded["signedReceipt"]
            .as_str()
            .unwrap_or_else(|| panic!("expected Some value")),
    )?))?;
    let old = old.protocol().ok_or("protocol receipt")?;
    let (transaction, signature) = decode_signed(raw)?;
    let sender = recover_sender(&transaction, &signature)?;
    let tx_hash = transaction_hash(raw);
    let seq = 1_u64;
    let mut chain = Sha256::new();
    chain.update(b"PAXEER/v1/sequence\0");
    chain.update([0; 32]);
    chain.update(seq.to_be_bytes());
    chain.update(tx_hash);
    let resulting: [u8; 32] = chain.finalize().into();
    let mut summary = old.effects()[0].body().to_vec();
    summary[24..28].copy_from_slice(&0_u32.to_be_bytes());
    let mut log_body = bytes(
        log["address"]
            .as_str()
            .unwrap_or_else(|| panic!("expected Some value")),
    )?;
    log_body.push(4);
    for topic in log["topics"]
        .as_array()
        .unwrap_or_else(|| panic!("expected Some value"))
    {
        log_body.extend(bytes(
            topic
                .as_str()
                .unwrap_or_else(|| panic!("expected Some value")),
        )?);
    }
    log_body.extend(Sha256::digest(bytes(
        log["data"]
            .as_str()
            .unwrap_or_else(|| panic!("expected Some value")),
    )?));
    let mut encoder = Encoder::new(512 * 1024);
    wire(encoder.structure_header_version(0x5201, 2))?;
    wire(encoder.u16(2))?;
    wire(encoder.bytes(&tx_hash, 32))?;
    wire(encoder.u64(seq))?;
    wire(encoder.bytes(&[0; 32], 32))?;
    wire(encoder.bytes(&resulting, 32))?;
    wire(encoder.bytes(&Sha256::digest([]), 32))?;
    wire(encoder.i32(0))?;
    wire(encoder.sequence_length(2, 512))?;
    for (index, body) in [summary, log_body].into_iter().enumerate() {
        let effect = &old.effects()[index];
        wire(encoder.u16(effect.module_id()))?;
        wire(encoder.u16(u16::try_from(index)?))?;
        wire(encoder.u16(effect.event_type()))?;
        wire(encoder.u8(effect.kind()))?;
        wire(encoder.u8(u8::from(effect.monetary())))?;
        wire(encoder.bytes(&effect.transfer_set_root(), 32))?;
        wire(encoder.bytes(&body, 256))?;
    }
    wire(encoder.u128(old.fee_charged()))?;
    wire(encoder.bytes(&old.batch_id(), 32))?;
    wire(encoder.u16(old.module_id()))?;
    wire(encoder.u32(old.module_version()))?;
    wire(encoder.u32(old.parameter_version()))?;
    wire(encoder.u8(old.operation()))?;
    wire(encoder.bytes(&old.asset(), 32))?;
    wire(encoder.u128(old.amount()))?;
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    wire(encoder.bytes(&from, 32))?;
    wire(encoder.u128(old.debit_balance_before()))?;
    wire(encoder.u128(old.debit_balance_after()))?;
    wire(encoder.u64(transaction.nonce))?;
    wire(encoder.bytes(&old.to(), 32))?;
    wire(encoder.u128(old.credit_balance_before()))?;
    wire(encoder.u128(old.credit_balance_after()))?;
    wire(encoder.bytes(&old.transfer_set_root(), 32))?;
    wire(encoder.bytes(&old.authorization_hash(), 32))?;
    wire(encoder.bytes(&old.context_hash(), 32))?;
    wire(encoder.u64(payment_now_millis()?))?;
    wire(encoder.u8(0))?;
    let mut unsigned = encoder.finish();
    let mut hasher = Sha256::new();
    hasher.update(b"LXP/v1/receipt\0");
    hasher.update(&unsigned);
    let digest: [u8; 32] = hasher.finalize().into();
    assert_eq!(unsigned.pop(), Some(0));
    unsigned.push(1);
    unsigned.extend(64_u32.to_be_bytes());
    unsigned.extend(SigningKey::from_bytes(&[0x72; 32]).sign(&digest).to_bytes());
    let decoded = wire(layerx_wire::receipt::decode(&unsigned))?;
    assert_eq!(wire(layerx_wire::receipt::encode(&decoded))?, unsigned);
    assert_eq!(
        layerx_interop_gateway::evidence::canonical_receipt_digest(&unsigned)?,
        digest
    );
    Ok(unsigned)
}
