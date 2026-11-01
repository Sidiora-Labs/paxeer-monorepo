#[path = "../src/payer.rs"]
mod payer;

use layerx_agentd::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
use layerx_client::sequencer::{ExecutedReceipt, ReceiptKeyEntry, SequencerLog};
use layerx_crypto::evm_transaction::transaction_hash;
use layerx_crypto::signer::{LocalSigner, Signer};
use layerx_mcp::tools::web::WebPayerError;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::{MAX_MESSAGE_BYTES, MAX_SEQUENCER_EFFECTS, PROTOCOL_VERSION};
use sha2::{Digest as _, Sha256};
use std::os::unix::fs::OpenOptionsExt as _;
use std::process::Command;
use std::sync::atomic::{AtomicU64, Ordering};

const KEY_ID: [u8; 32] = [0x33; 32];
const HEIGHT: u64 = 100;
const SENDER: [u8; 20] = [0x11; 20];
const TX_HASH: [u8; 32] = [0x22; 32];
const PAYER: [u8; 32] = [0x41; 32];
const RECIPIENT: [u8; 32] = [0x42; 32];
const ASSET: [u8; 32] = [0x43; 32];
const REFERENCE: [u8; 32] = [0x44; 32];
const AMOUNT: u128 = (1_u128 << 80) + 123_456;

type Result<T = ()> = std::result::Result<T, Box<dyn std::error::Error>>;

fn wire<T>(result: std::result::Result<T, layerx_wire::WireError>) -> Result<T> {
    result.map_err(|error| format!("{error:?}").into())
}

fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

pub fn published_keys() -> Vec<ReceiptKeyEntry> {
    vec![ReceiptKeyEntry {
        key_id: KEY_ID,
        public_key: LocalSigner::new([7; 32]).public_key(),
        first_height: HEIGHT,
        last_height: HEIGHT,
    }]
}

fn exact_call() -> PrecompileCall {
    let mut amount = [0; 32];
    amount[16..].copy_from_slice(&AMOUNT.to_be_bytes());
    PrecompileCall {
        target: PrecompileTarget::Ledger,
        selector: [0x78, 0xea, 0x11, 0xdb],
        arguments: [PAYER, RECIPIENT, ASSET, amount, [0; 32], REFERENCE]
            .into_iter()
            .map(AbiArgument::Word)
            .collect(),
    }
}

fn sign_digest(seed: [u8; 32], digest: &[u8; 32]) -> Result<[u8; 64]> {
    static NEXT: AtomicU64 = AtomicU64::new(0);
    let directory = std::env::temp_dir().join(format!(
        "layerx-payer-receipt-{}-{}",
        std::process::id(),
        NEXT.fetch_add(1, Ordering::Relaxed)
    ));
    std::fs::create_dir(&directory)?;
    let result = (|| {
        let key = directory.join("key.der");
        let input = directory.join("digest");
        let mut pkcs8 = vec![
            0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x04, 0x22,
            0x04, 0x20,
        ];
        pkcs8.extend_from_slice(&seed);
        use std::io::Write as _;
        std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&key)?
            .write_all(&pkcs8)?;
        std::fs::write(&input, digest)?;
        let output = Command::new("openssl")
            .args(["pkeyutl", "-sign", "-rawin", "-keyform", "DER", "-inkey"])
            .arg(&key)
            .arg("-in")
            .arg(&input)
            .output()?;
        if !output.status.success() {
            return Err(String::from_utf8_lossy(&output.stderr).into_owned().into());
        }
        output
            .stdout
            .try_into()
            .map_err(|_| "Ed25519 signature width".into())
    })();
    std::fs::remove_dir_all(directory)?;
    result
}

fn encode_execution(execution: &ExecutedReceipt, signature: Option<[u8; 64]>) -> Result<Vec<u8>> {
    let module = 12;
    let mut summary = Vec::new();
    summary.extend_from_slice(&execution.gas_used.to_be_bytes());
    summary.extend_from_slice(&execution.cumulative_gas_used.to_be_bytes());
    summary.extend_from_slice(&execution.effective_gas_price.to_be_bytes());
    summary.extend_from_slice(&u32::try_from(execution.index)?.to_be_bytes());
    summary.extend_from_slice(&execution.contract_address);
    summary.extend_from_slice(&execution.sponsor);
    summary.extend_from_slice(&sha256(execution.vm_error.as_bytes()));
    summary.extend_from_slice(&u32::try_from(execution.return_data.len())?.to_be_bytes());
    summary.extend_from_slice(&u32::try_from(execution.logs.len())?.to_be_bytes());
    summary.push(u8::try_from(execution.fee_denom.len())?);
    summary.extend_from_slice(execution.fee_denom.as_bytes());
    let mut effects = vec![(1, summary)];
    for log in &execution.logs {
        let mut body = log.address.to_vec();
        body.push(u8::try_from(log.topics.len())?);
        for topic in &log.topics {
            body.extend_from_slice(topic);
        }
        body.extend_from_slice(&sha256(&log.data));
        effects.push((3, body));
    }
    let previous = [0x55; 32];
    let mut sequence = b"PAXEER/v1/sequence\0".to_vec();
    sequence.extend_from_slice(&previous);
    sequence.extend_from_slice(&execution.seq.to_be_bytes());
    sequence.extend_from_slice(&execution.tx_hash);
    let mut height = [0; 32];
    height[24..].copy_from_slice(&execution.height.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&execution.from);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&execution.to);
    let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
    wire(encoder.structure_header_version(0x5201, PROTOCOL_VERSION))?;
    wire(encoder.u16(PROTOCOL_VERSION))?;
    wire(encoder.bytes(&execution.tx_hash, 32))?;
    wire(encoder.u64(execution.seq))?;
    wire(encoder.bytes(&previous, 32))?;
    wire(encoder.bytes(&sha256(&sequence), 32))?;
    wire(encoder.bytes(&sha256(&execution.return_data), 32))?;
    wire(encoder.i32(0))?;
    wire(encoder.sequence_length(effects.len(), MAX_SEQUENCER_EFFECTS))?;
    for (ordinal, (kind, body)) in effects.iter().enumerate() {
        wire(encoder.u16(module))?;
        wire(encoder.u16(u16::try_from(ordinal)?))?;
        wire(encoder.u16(0))?;
        wire(encoder.tag(*kind, 3))?;
        wire(encoder.u8(0))?;
        wire(encoder.bytes(&[0; 32], 32))?;
        wire(encoder.bytes(body, 256))?;
    }
    wire(encoder.u128(1))?;
    wire(encoder.bytes(&height, 32))?;
    wire(encoder.u16(module))?;
    wire(encoder.u32(1))?;
    wire(encoder.u32(0))?;
    wire(encoder.u8(2))?;
    wire(encoder.bytes(&sha256(execution.fee_denom.as_bytes()), 32))?;
    wire(encoder.u128(0))?;
    wire(encoder.bytes(&from, 32))?;
    wire(encoder.u128(0))?;
    wire(encoder.u128(0))?;
    wire(encoder.u64(execution.nonce))?;
    wire(encoder.bytes(&to, 32))?;
    wire(encoder.u128(0))?;
    wire(encoder.u128(0))?;
    wire(encoder.bytes(&[0; 32], 32))?;
    wire(encoder.bytes(&[0; 32], 32))?;
    wire(encoder.bytes(&execution.key_id, 32))?;
    wire(encoder.u64(execution.timestamp))?;
    wire(encoder.u8(u8::from(signature.is_some())))?;
    if let Some(signature) = signature {
        wire(encoder.bytes(&signature, 64))?;
    }
    Ok(encoder.finish())
}

pub fn signed_execution(
    sender: [u8; 20],
    tx_hash: [u8; 32],
    nonce: u64,
    data: &[u8],
    seed: [u8; 32],
) -> Result<ExecutedReceipt> {
    let word = |offset: usize| -> [u8; 32] {
        data[offset..offset + 32].try_into().expect("calldata word")
    };
    let (topics, log_data) = match &data[..4] {
        [0x78, 0xea, 0x11, 0xdb] => (
            vec![
                transaction_hash(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"),
                word(164),
                word(4),
                word(36),
            ],
            [word(68), word(100)].concat(),
        ),
        [0x2e, 0x89, 0x49, 0x0f] => (
            vec![
                transaction_hash(
                    b"PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)",
                ),
                word(4),
                PAYER,
                word(36),
            ],
            [ASSET, word(68), word(100), word(196)].concat(),
        ),
        [0xd0, 0xd5, 0x34, 0x80] => (
            vec![transaction_hash(b"PayerGrantIssued(bytes32)"), word(68)],
            vec![],
        ),
        _ => return Err("unexpected ledger call".into()),
    };
    let mut fee = [0; 32];
    fee[31] = 1;
    let mut execution = ExecutedReceipt {
        tx_hash,
        nonce,
        seq: 42,
        height: HEIGHT,
        index: 0,
        timestamp: 0x123,
        success: true,
        return_data: vec![],
        logs: vec![SequencerLog {
            address: PrecompileTarget::Ledger.address(),
            topics,
            data: log_data,
            log_index: 0,
        }],
        gas_used: 0x10000,
        cumulative_gas_used: 0x10000,
        effective_gas_price: 1_000_000_000,
        fee_denom: "upax".to_owned(),
        fee_amount: fee,
        sponsor: [0; 20],
        from: sender,
        to: PrecompileTarget::Ledger.address(),
        contract_address: [0; 20],
        vm_error: String::new(),
        key_id: KEY_ID,
        receipt: vec![],
        signing_key: LocalSigner::new(seed).public_key(),
    };
    let unsigned = encode_execution(&execution, None)?;
    let signature = sign_digest(seed, &wire(receipt_digest(&unsigned))?)?;
    execution.receipt = encode_execution(&execution, Some(signature))?;
    Ok(execution)
}

#[test]
fn web_payment_receipt_must_be_signed_by_a_published_key() -> Result {
    let call = exact_call();
    let data = call.encode().map_err(|error| format!("{error:?}"))?;
    let published = published_keys();
    let valid = signed_execution(SENDER, TX_HASH, 7, &data, [7; 32])?;
    assert!(payer::receipt(valid.clone(), TX_HASH, SENDER, &call, &published).is_ok());
    let foreign = signed_execution(SENDER, TX_HASH, 7, &data, [8; 32])?;
    let mut foreign_keys = published.clone();
    foreign_keys[0].public_key = foreign.signing_key;
    assert!(payer::receipt(foreign.clone(), TX_HASH, SENDER, &call, &foreign_keys).is_ok());
    assert_eq!(
        payer::receipt(foreign, TX_HASH, SENDER, &call, &published),
        Err(WebPayerError::Refused)
    );
    assert_eq!(
        payer::receipt(valid.clone(), TX_HASH, SENDER, &call, &[]),
        Err(WebPayerError::Refused)
    );
    let mut expired = published.clone();
    expired[0].last_height = HEIGHT - 1;
    assert_eq!(
        payer::receipt(valid.clone(), TX_HASH, SENDER, &call, &expired),
        Err(WebPayerError::Refused)
    );
    let mut future = published.clone();
    future[0].first_height = HEIGHT + 1;
    assert_eq!(
        payer::receipt(valid.clone(), TX_HASH, SENDER, &call, &future),
        Err(WebPayerError::Refused)
    );
    let mut other_id = published.clone();
    other_id[0].key_id[0] ^= 1;
    assert_eq!(
        payer::receipt(valid.clone(), TX_HASH, SENDER, &call, &other_id),
        Err(WebPayerError::Refused)
    );
    let mut ambiguous = published.clone();
    ambiguous.push(published[0]);
    assert_eq!(
        payer::receipt(valid, TX_HASH, SENDER, &call, &ambiguous),
        Err(WebPayerError::Refused)
    );
    Ok(())
}

#[test]
fn web_payment_reports_the_pay_exact_log_amount() -> Result {
    let call = exact_call();
    let execution = signed_execution(
        SENDER,
        TX_HASH,
        7,
        &call.encode().map_err(|error| format!("{error:?}"))?,
        [7; 32],
    )?;
    let payment = payer::receipt(execution.clone(), TX_HASH, SENDER, &call, &published_keys())
        .map_err(|error| format!("{error:?}"))?;
    let protocol = wire(layerx_wire::receipt::decode(&payment.receipt))?;
    assert_eq!(protocol.protocol().ok_or("protocol receipt")?.amount(), 0);
    assert_eq!(payment.tx_hash, TX_HASH);
    assert_eq!(payment.receipt, execution.receipt);
    assert_eq!(payment.logs.len(), 1);
    let log = &payment.logs[0];
    assert_eq!(log.address(), PrecompileTarget::Ledger.address());
    assert_eq!(
        log.topics(),
        &[
            transaction_hash(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"),
            REFERENCE,
            PAYER,
            RECIPIENT
        ]
    );
    assert_eq!(log.data().len(), 64);
    assert_eq!(&log.data()[..32], &ASSET);
    assert_eq!(&log.data()[32..48], &[0; 16]);
    assert_eq!(u128::from_be_bytes(log.data()[48..64].try_into()?), AMOUNT);
    for index in [0, 48, 63] {
        let mut tampered = execution.clone();
        tampered.logs[0].data[index] ^= 1;
        assert_eq!(
            payer::receipt(tampered, TX_HASH, SENDER, &call, &published_keys()),
            Err(WebPayerError::Refused)
        );
    }
    let mut recipient = execution.clone();
    recipient.logs[0].topics[3][0] ^= 1;
    assert_eq!(
        payer::receipt(recipient, TX_HASH, SENDER, &call, &published_keys()),
        Err(WebPayerError::Refused)
    );
    let mut wrong_call = call.clone();
    wrong_call.arguments[1] = AbiArgument::Word([0x99; 32]);
    assert_eq!(
        payer::receipt(
            execution.clone(),
            TX_HASH,
            SENDER,
            &wrong_call,
            &published_keys()
        ),
        Err(WebPayerError::Refused)
    );
    assert_eq!(
        payer::receipt(execution, [0x99; 32], SENDER, &call, &published_keys()),
        Err(WebPayerError::Refused)
    );
    Ok(())
}
