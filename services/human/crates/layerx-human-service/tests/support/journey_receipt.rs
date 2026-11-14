use std::fmt::Debug;
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::{Ipv4Addr, TcpListener};
use std::thread;
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::outbox::{Outbox, SubmissionState, resolve_unknown};
use layerx_agentd::store::Store;
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_human_service::journeys::{AgentBoundaryError, ReceiptMaterial};
use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::{rung::Rung, verify::VerificationLevel};
use serde_json::{Value, json};
use sha2::{Digest as _, Sha256};

pub const ASSET: [u8; 32] = [
    0x80, 0xfa, 0xce, 0x2b, 0x4a, 0x14, 0x30, 0x8e, 0x32, 0x93, 0x26, 0xee, 0x2f, 0x81, 0x41, 0xd2,
    0x49, 0x07, 0xae, 0x84, 0x75, 0x54, 0xcb, 0xba, 0xb6, 0x48, 0x54, 0xc6, 0xec, 0xc9, 0x11, 0xf7,
];
const KEY_ID: [u8; 32] = [0x94; 32];

fn checked<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("journey receipt: {error:?}"))
}

fn bytes(output: &mut Vec<u8>, value: &[u8]) {
    output.extend_from_slice(&checked(u32::try_from(value.len())).to_be_bytes());
    output.extend_from_slice(value);
}

pub fn material(raw: &[u8], marker: u8) -> ReceiptMaterial {
    let (call, signature) = checked(decode_signed(raw));
    let sender = checked(recover_sender(&call, &signature));
    let tx_hash = transaction_hash(raw);
    let sequence = u64::from(marker);
    let previous = [marker.saturating_add(1); 32];
    let resulting = [marker.saturating_add(2); 32];
    let mut height = [0; 32];
    height[24..].copy_from_slice(&sequence.to_be_bytes());
    let signer = SigningKey::from_bytes(&[marker.saturating_add(4); 32]);
    let mut summary = Vec::new();
    for gas_field in [1_u64, 1, 1] {
        summary.extend_from_slice(&gas_field.to_be_bytes());
    }
    summary.extend_from_slice(&0_u32.to_be_bytes());
    summary.extend_from_slice(&[0; 40]);
    summary.extend_from_slice(&Sha256::digest(b""));
    summary.extend_from_slice(&0_u32.to_be_bytes());
    summary.extend_from_slice(&0_u32.to_be_bytes());
    summary.push(4);
    summary.extend_from_slice(b"alxp");

    let mut encoded = Vec::new();
    encoded.extend_from_slice(&2_u16.to_be_bytes());
    encoded.extend_from_slice(&0x5201_u16.to_be_bytes());
    encoded.extend_from_slice(&2_u16.to_be_bytes());
    bytes(&mut encoded, &tx_hash);
    encoded.extend_from_slice(&sequence.to_be_bytes());
    bytes(&mut encoded, &previous);
    bytes(&mut encoded, &resulting);
    bytes(&mut encoded, &Sha256::digest(b""));
    encoded.extend_from_slice(&0_i32.to_be_bytes());
    encoded.extend_from_slice(&1_u32.to_be_bytes());
    encoded.extend_from_slice(&12_u16.to_be_bytes());
    encoded.extend_from_slice(&0_u16.to_be_bytes());
    encoded.extend_from_slice(&0_u16.to_be_bytes());
    encoded.push(1);
    encoded.push(0);
    bytes(&mut encoded, &[0; 32]);
    bytes(&mut encoded, &summary);
    encoded.extend_from_slice(&1_u128.to_be_bytes());
    bytes(&mut encoded, &height);
    encoded.extend_from_slice(&12_u16.to_be_bytes());
    encoded.extend_from_slice(&1_u32.to_be_bytes());
    encoded.extend_from_slice(&0_u32.to_be_bytes());
    encoded.push(2);
    bytes(&mut encoded, &ASSET);
    encoded.extend_from_slice(&0_u128.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    bytes(&mut encoded, &from);
    encoded.extend_from_slice(&10_u128.to_be_bytes());
    encoded.extend_from_slice(&9_u128.to_be_bytes());
    encoded.extend_from_slice(&call.nonce.to_be_bytes());
    let mut to = [0; 32];
    to[12..].copy_from_slice(&call.to);
    bytes(&mut encoded, &to);
    encoded.extend_from_slice(&0_u128.to_be_bytes());
    encoded.extend_from_slice(&0_u128.to_be_bytes());
    bytes(&mut encoded, &[0; 32]);
    bytes(&mut encoded, &[0; 32]);
    bytes(&mut encoded, &KEY_ID);
    encoded.extend_from_slice(&1_000_u64.to_be_bytes());
    encoded.push(0);
    let receipt = checked(layerx_wire::receipt::decode(&encoded));
    let unsigned = checked(layerx_wire::receipt::encode_unsigned(&receipt));
    assert_eq!(unsigned, encoded);
    let digest = checked(layerx_wire::hash::receipt_digest(&unsigned));
    encoded.pop();
    encoded.push(1);
    bytes(&mut encoded, &signer.sign(&digest).to_bytes());
    let canonical = checked(layerx_wire::receipt::encode(&checked(
        layerx_wire::receipt::decode(&encoded),
    )));
    assert_eq!(canonical, encoded);
    let material = ReceiptMaterial {
        receipt_hash: Sha256::digest(&canonical).into(),
        signed_receipt: canonical.clone(),
        canonical_bytes: canonical,
        tx_hash,
        block_height: sequence,
        rung: Rung::Receipted,
        receipt_key: layerx_human_service::journeys::ReceiptKeyEntry {
            key_id: KEY_ID,
            public_key: signer.verifying_key().to_bytes(),
            first_height: 1,
            last_height: u64::MAX,
        },
        inclusion_height: sequence,
        authorised_batch: AuthorizedBatch::new(
            height,
            ASSET,
            previous,
            resulting,
            signer.verifying_key().to_bytes(),
        ),
        verification_level: VerificationLevel::SEQUENCER_SIGNED,
    };
    checked(layerx_proof::receipt::verify(
        &material.canonical_bytes,
        &material.authorised_batch,
    ));
    material
}

fn hex(value: &[u8]) -> String {
    let mut output = String::from("0x");
    for byte in value {
        use std::fmt::Write as _;
        checked(write!(output, "{byte:02x}"));
    }
    output
}

pub fn record_execution(
    outbox: &mut Outbox,
    store: &mut Store,
    submission_id: [u8; 32],
    material: &ReceiptMaterial,
) -> Result<(), AgentBoundaryError> {
    let raw = outbox
        .exact_signed_bytes(submission_id)
        .map_err(|_| AgentBoundaryError::CorruptResponse)?;
    assert_eq!(transaction_hash(raw), material.tx_hash);
    let (call, signature) = checked(decode_signed(raw));
    let sender = checked(recover_sender(&call, &signature));
    let receipt = checked(layerx_wire::receipt::decode(&material.canonical_bytes));
    let protocol = receipt
        .protocol()
        .unwrap_or_else(|| panic!("protocol receipt required"));
    assert_eq!(
        protocol.previous_state_root(),
        material.authorised_batch.previous_state_root()
    );
    assert_eq!(
        protocol.resulting_state_root(),
        material.authorised_batch.resulting_state_root()
    );
    let response = json!({
        "rung": "receipted", "txHash": hex(&material.tx_hash),
        "nonce": format!("{:#x}", call.nonce), "seq": format!("{:#x}", protocol.global_sequence()),
        "height": format!("{:#x}", material.block_height), "index": "0x0", "timestamp": "0x3e8",
        "status": "0x1", "returnData": "0x", "logs": [], "gasUsed": "0x1",
        "cumulativeGasUsed": "0x1", "effectiveGasPrice": "0x1", "feeDenom": "alxp",
        "paymentDenom": "alxp", "feeAmount": "0x1", "sponsor": hex(&[0; 20]),
        "from": hex(&sender), "to": hex(&call.to), "contractAddress": hex(&[0; 20]),
        "vmError": "", "keyId": hex(&material.receipt_key.key_id),
        "signingKey": hex(&material.receipt_key.public_key),
        "receipt": hex(&material.canonical_bytes)
    });
    let listener = checked(TcpListener::bind((Ipv4Addr::LOCALHOST, 0)));
    checked(listener.set_nonblocking(true));
    let mut endpoint = checked(SequencerEndpoint::new_node(
        &format!("http://{}/", checked(listener.local_addr())),
        &[],
    ));
    let hash = hex(&material.tx_hash);
    let worker = thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(5);
        let mut served = Vec::new();
        while served.len() < 2 {
            let stream = match listener.accept() {
                Ok((stream, _)) => stream,
                Err(error)
                    if error.kind() == std::io::ErrorKind::WouldBlock
                        && Instant::now() < deadline =>
                {
                    thread::sleep(Duration::from_millis(1));
                    continue;
                }
                Err(error) => panic!("receipt transport: {error}"),
            };
            checked(stream.set_read_timeout(Some(Duration::from_secs(2))));
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(checked(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some((name, value)) = line.split_once(':') {
                    if name.eq_ignore_ascii_case("content-length") {
                        length = Some(checked(value.trim().parse::<usize>()));
                    }
                }
            }
            let mut body = vec![0; length.unwrap_or_else(|| panic!("RPC content length required"))];
            checked(reader.read_exact(&mut body));
            let request: Value = checked(serde_json::from_slice(&body));
            assert_eq!(request["params"], json!([hash]));
            let method = request["method"]
                .as_str()
                .unwrap_or_else(|| panic!("RPC method required"));
            let result = match method {
                "pax_sequencerReceipt" => response.clone(),
                "eth_getTransactionReceipt" => Value::Null,
                _ => panic!("unexpected receipt method: {method}"),
            };
            served.push(method.to_owned());
            let body = checked(serde_json::to_vec(
                &json!({"jsonrpc":"2.0", "id":request["id"], "result":result}),
            ));
            checked(write!(
                reader.get_mut(),
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                body.len()
            ));
            checked(reader.get_mut().write_all(&body));
        }
        assert_eq!(
            served,
            ["pax_sequencerReceipt", "eth_getTransactionReceipt"]
        );
    });
    outbox
        .transition(
            store,
            submission_id,
            SubmissionState::Unknown,
            "receipt lookup",
            None,
        )
        .map_err(|_| AgentBoundaryError::Refused)?;
    let resolved = resolve_unknown(
        outbox,
        store,
        submission_id,
        1_000,
        &[material.receipt_key],
        &mut endpoint,
    );
    checked(worker.join());
    let resolved = resolved.map_err(|_| AgentBoundaryError::CorruptResponse)?;
    assert_eq!(
        resolved.state,
        SubmissionState::Executed,
        "authenticated receipt resolution: {resolved:?}"
    );
    Ok(())
}
