use ed25519_dalek::{Signer as _, SigningKey};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::PROTOCOL_VERSION;
use sha2::{Digest, Sha256};
use std::error::Error;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{Ipv4Addr, TcpListener};
use std::os::unix::fs::DirBuilderExt;
use std::os::unix::process::ExitStatusExt;
use std::path::PathBuf;
use std::process::{ExitCode, ExitStatus, Output};
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{Duration, Instant};

type TestResult = Result<(), Box<dyn Error>>;
static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);
struct Directory(PathBuf);
impl Directory {
    fn new() -> Result<Self, Box<dyn Error>> {
        let path = std::env::temp_dir().join(format!(
            "layerxctl-receipts-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
        fs::DirBuilder::new().mode(0o700).create(&path)?;
        Ok(Self(path))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
fn ctl(arguments: &[&str]) -> Result<Output, Box<dyn Error>> {
    let (text, status) = crate::run(
        &arguments
            .iter()
            .map(|s| (*s).to_owned())
            .collect::<Vec<_>>(),
    )?;
    let code = if status == ExitCode::SUCCESS {
        0
    } else if status == ExitCode::FAILURE {
        1
    } else {
        assert_eq!(status, ExitCode::from(3));
        3
    };
    Ok(Output {
        status: ExitStatus::from_raw(code << 8),
        stdout: format!("{text}\n").into_bytes(),
        stderr: Vec::new(),
    })
}
fn json_string<'a>(text: &'a str, key: &str) -> Result<&'a str, Box<dyn Error>> {
    let (_, value) = text
        .split_once(&format!("\"{key}\""))
        .ok_or_else(|| format!("missing JSON field {key}"))?;
    let (_, value) = value.split_once(':').ok_or("missing JSON colon")?;
    value
        .trim_start()
        .strip_prefix('"')
        .and_then(|text| text.split('"').next())
        .ok_or_else(|| format!("JSON field {key} is not a string").into())
}

fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    bytes.iter().fold(String::new(), |mut text, byte| {
        let _ = write!(text, "{byte:02x}");
        text
    })
}

fn unhex(text: &str) -> Result<Vec<u8>, Box<dyn Error>> {
    let digits = text.strip_prefix("0x").ok_or("missing hex prefix")?;
    if !digits.len().is_multiple_of(2) || !digits.is_ascii() {
        return Err("invalid hex".into());
    }
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| Ok(u8::from_str_radix(std::str::from_utf8(pair)?, 16)?))
        .collect()
}

fn answer_send(
    listener: &TcpListener,
    method: &str,
    answer: &str,
) -> Result<String, Box<dyn Error>> {
    let (stream, _) = listener.accept()?;
    stream.set_read_timeout(Some(Duration::from_secs(5)))?;
    stream.set_write_timeout(Some(Duration::from_secs(5)))?;
    let mut reader = BufReader::new(stream);
    let mut length = 0_usize;
    loop {
        let mut line = String::new();
        if reader.read_line(&mut line)? == 0 {
            return Err("truncated HTTP request".into());
        }
        if line == "\r\n" {
            break;
        }
        if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
            length = value.trim().parse()?;
        }
    }
    let mut body = vec![0_u8; length];
    reader.read_exact(&mut body)?;
    let body = String::from_utf8(body)?;
    assert!(body.contains(&format!("\"method\":\"{method}\"")));
    let id: u64 = body
        .split_once("\"id\":")
        .ok_or("request has no id")?
        .1
        .split(|character: char| !character.is_ascii_digit())
        .next()
        .ok_or("request id is not a number")?
        .parse()?;
    let answer = answer
        .replace("{id}", &id.to_string())
        .replace("{other}", &(id + 1).to_string());
    let mut stream = reader.into_inner();
    write!(
            stream,
            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",
            answer.len()
        )?;
    stream.flush()?;
    Ok(body)
}

fn transaction_hash(raw: &[u8]) -> String {
    format!(
        "0x{}",
        hex(&layerx_crypto::evm_transaction::transaction_hash(raw))
    )
}

fn encoded_receipt(
    transaction: [u8; 32],
    key_id: [u8; 32],
    height: [u8; 32],
    result: i32,
    signature: Option<[u8; 64]>,
) -> Vec<u8> {
    let mut encoder = Encoder::new(4096);
    assert_eq!(
        encoder.structure_header_version(0x5201, PROTOCOL_VERSION),
        Ok(())
    );
    assert_eq!(encoder.u16(PROTOCOL_VERSION), Ok(()));
    assert_eq!(encoder.bytes(&transaction, 32), Ok(()));
    assert_eq!(encoder.u64(7), Ok(()));
    assert_eq!(encoder.bytes(&[2; 32], 32), Ok(()));
    assert_eq!(encoder.bytes(&[3; 32], 32), Ok(()));
    assert_eq!(encoder.bytes(&[4; 32], 32), Ok(()));
    assert_eq!(encoder.i32(result), Ok(()));
    assert_eq!(encoder.sequence_length(0, 512), Ok(()));
    assert_eq!(encoder.u128(1), Ok(()));
    assert_eq!(encoder.bytes(&height, 32), Ok(()));
    assert_eq!(encoder.u16(1), Ok(()));
    assert_eq!(encoder.u32(1), Ok(()));
    assert_eq!(encoder.u32(1), Ok(()));
    assert_eq!(encoder.u8(1), Ok(()));
    assert_eq!(encoder.bytes(&Sha256::digest(b"upax"), 32), Ok(()));
    assert_eq!(encoder.u128(25), Ok(()));
    assert_eq!(encoder.bytes(&[6; 32], 32), Ok(()));
    assert_eq!(encoder.u128(100), Ok(()));
    assert_eq!(encoder.u128(75), Ok(()));
    assert_eq!(encoder.u64(1), Ok(()));
    assert_eq!(encoder.bytes(&[7; 32], 32), Ok(()));
    assert_eq!(encoder.u128(10), Ok(()));
    assert_eq!(encoder.u128(35), Ok(()));
    assert_eq!(encoder.bytes(&[8; 32], 32), Ok(()));
    assert_eq!(encoder.bytes(&[9; 32], 32), Ok(()));
    assert_eq!(encoder.bytes(&key_id, 32), Ok(()));
    assert_eq!(encoder.u64(1_000), Ok(()));
    assert_eq!(encoder.u8(u8::from(signature.is_some())), Ok(()));
    if let Some(signature) = signature {
        assert_eq!(encoder.bytes(&signature, 64), Ok(()));
    }
    encoder.finish()
}

fn signed_receipt(
    signer: &SigningKey,
    transaction: [u8; 32],
    key_id: [u8; 32],
    height: [u8; 32],
    result: i32,
) -> Result<Vec<u8>, Box<dyn Error>> {
    let unsigned = encoded_receipt(transaction, key_id, height, result, None);
    let digest = receipt_digest(&unsigned).map_err(|error| format!("receipt digest: {error:?}"))?;
    Ok(encoded_receipt(
        transaction,
        key_id,
        height,
        result,
        Some(signer.sign(&digest).to_bytes()),
    ))
}

fn pinned_key(key_id: [u8; 32], public_key: [u8; 32], first: u64, last: u64) -> String {
    format!("{{\"keys\":[{{\"keyId\":\"0x{}\",\"publicKey\":\"0x{}\",\"firstHeight\":{first},\"lastHeight\":{last}}}]}}", hex(&key_id), hex(&public_key))
}

fn submit_receipt(bytes: &[u8], keys: &str, committed: bool) -> Result<Output, Box<dyn Error>> {
    let directory = Directory::new()?;
    let path = directory.0.join("transaction");
    let raw = [0x02, 0xf8, 0x6b];
    fs::write(&path, raw)?;
    let hash = transaction_hash(&raw);
    let rung = if committed { "committed" } else { "receipted" };
    let landing = if committed {
        format!(
            ",\"blockNumber\":\"0x96\",\"blockHash\":\"0x{}\"",
            hex(&[10; 32])
        )
    } else {
        String::new()
    };
    let receipt = format!(
            "{{\"rung\":\"{rung}\",\"txHash\":\"{hash}\",\"nonce\":\"0x1\",\"seq\":\"0x7\",\"height\":\"0x96\",\"index\":\"0x0\",\"timestamp\":\"0x3e8\",\"status\":\"0x1\",\"returnData\":\"0x\",\"logs\":[],\"gasUsed\":\"0x5208\",\"cumulativeGasUsed\":\"0x5208\",\"effectiveGasPrice\":\"0x1\",\"paymentDenom\":\"upax\",\"signingKey\":\"{}\",\"feeDenom\":\"upax\",\"feeAmount\":\"0x5208\",\"sponsor\":\"0x{}\",\"from\":\"0x{}\",\"to\":\"0x{}\",\"contractAddress\":\"0x{}\",\"vmError\":\"\",\"keyId\":\"0x{}\",\"receipt\":\"0x{}\"{landing}}}",
            json_string(keys, "publicKey")?, hex(&[0; 20]), hex(&[6; 20]), hex(&[7; 20]), hex(&[0; 20]), hex(&[11; 32]), hex(bytes)
        );
    let entry = RecordedSubmit {
        name: "negative-signature".to_owned(),
        expected: "refused".to_owned(),
        raw: format!("0x{}", hex(&raw)),
        hash,
        receipt,
        keys: keys.to_owned(),
    };
    submit_recorded(&entry)
}

struct RecordedSubmit {
    name: String,
    expected: String,
    raw: String,
    hash: String,
    receipt: String,
    keys: String,
}

fn recorded_submissions(vector: &str) -> Result<Vec<RecordedSubmit>, Box<dyn Error>> {
    let entries = match vector {
        "vector-v1-receipted-positive.json" => recorded_vectors::positive(),
        "vector-v2-receipted-revert.json" => recorded_vectors::revert(),
        "vector-v3-receipt-refusals.json" => recorded_vectors::refusals(),
        "vector-v4-committed.json" => recorded_vectors::committed(),
        _ => return Err(format!("unknown committed vector {vector}").into()),
    };
    Ok(entries)
}
#[path = "recorded_vectors.rs"]
mod recorded_vectors;

fn submit_recorded(entry: &RecordedSubmit) -> Result<Output, Box<dyn Error>> {
    let directory = Directory::new()?;
    let path = directory.0.join("transaction");
    let raw = unhex(&entry.raw)?;
    assert_eq!(transaction_hash(&raw), entry.hash, "{}", entry.name);
    fs::write(&path, raw)?;
    let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0))?;
    listener.set_nonblocking(true)?;
    let url = format!("http://{}/", listener.local_addr()?);
    let exchanges = [
        (
            "eth_sendRawTransaction",
            format!("[\"{}\"]", entry.raw),
            format!("\"{}\"", entry.hash),
        ),
        (
            "pax_sequencerReceipt",
            format!("[\"{}\"]", entry.hash),
            entry.receipt.clone(),
        ),
        ("px_getReceiptKeys", "[]".to_owned(), entry.keys.clone()),
    ];
    let (finished, completion) = std::sync::mpsc::channel();
    let server = std::thread::spawn(move || -> Result<(), String> {
        let deadline = Instant::now() + Duration::from_secs(10);
        for (method, params, result) in exchanges {
            let answer = format!("{{\"jsonrpc\":\"2.0\",\"id\":{{id}},\"result\":{result}}}");
            loop {
                if Instant::now() >= deadline {
                    return Err(format!("timed out waiting for {method}"));
                }
                if completion.try_recv().is_ok() {
                    return Ok(());
                }
                match answer_send(&listener, method, &answer) {
                    Ok(body) => {
                        assert!(body.contains(&format!("\"params\":{params}")));
                        break;
                    }
                    Err(error)
                        if error.downcast_ref::<std::io::Error>().is_some_and(|error| {
                            error.kind() == std::io::ErrorKind::WouldBlock
                        }) =>
                    {
                        std::thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => return Err(error.to_string()),
                }
            }
        }
        Ok(())
    });
    let output = ctl(&[
        "submit",
        "--rpc-url",
        &url,
        "--transaction",
        path.to_str().ok_or("path is not UTF-8")?,
    ]);
    let _ = finished.send(());
    server.join().map_err(|_| "server thread panicked")??;
    output
}

#[test]
fn submit_verifies_the_recorded_v1_vector() -> TestResult {
    let entries = recorded_submissions("vector-v1-receipted-positive.json")?;
    assert_eq!(entries.len(), 1);
    let entry = &entries[0];
    assert_eq!(entry.expected, "receipted");
    let output = submit_recorded(entry)?;
    assert_eq!(
        output.status.code(),
        Some(0),
        "{}: expected receipted for transaction {} (raw prefix {:?}); stdout {:?}, stderr {:?}",
        entry.name,
        entry.hash,
        entry.raw.get(..4),
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    assert!(output.stderr.is_empty());
    let text = String::from_utf8(output.stdout)?;
    assert_eq!(json_string(&text, "state")?, entry.expected);
    assert_eq!(json_string(&text, "transaction_hash")?, entry.hash);
    Ok(())
}

#[test]
fn submit_refuses_every_recorded_v3_mutation() -> TestResult {
    let entries = recorded_submissions("vector-v3-receipt-refusals.json")?;
    let expected = [
        "key-id-not-pinned",
        "height-outside-range",
        "activity-id-not-raw-hash",
        "sequence-hash-mismatch",
        "tampered-log",
        "non-canonical-receipt-bytes",
    ];
    assert_eq!(entries.len(), expected.len());
    let mut seen = std::collections::BTreeSet::new();
    let mut failures = Vec::new();
    for entry in entries {
        assert!(expected.contains(&entry.name.as_str()), "{}", entry.name);
        assert!(seen.insert(entry.name.clone()), "{}", entry.name);
        assert!(!entry.expected.is_empty(), "{}", entry.name);
        let output = submit_recorded(&entry)?;
        let text = String::from_utf8(output.stdout)?;
        let conforms = output.status.code() == Some(3)
            && output.stderr.is_empty()
            && json_string(&text, "state").is_ok_and(|state| state == "submitted")
            && json_string(&text, "transaction_hash").is_ok_and(|hash| hash == entry.hash)
            && if matches!(
                entry.name.as_str(),
                "activity-id-not-raw-hash" | "non-canonical-receipt-bytes"
            ) {
                text.contains("\"receipt_error\":{\"transport_error\":\"Malformed\"}")
            } else {
                json_string(&text, "verification_failure").is_ok_and(|failure| {
                    failure
                        == match entry.name.as_str() {
                            "key-id-not-pinned" | "height-outside-range" => "SequencerSignature",
                            "sequence-hash-mismatch" => "ResultingStateRoot",
                            "tampered-log" => "ReceiptView",
                            _ => unreachable!(),
                        }
                })
            };
        if !conforms {
            failures.push(format!(
                    "{} ({}): expected submitted with a named verification_failure; exit {:?}, stdout {text:?}, stderr {:?}",
                    entry.name,
                    entry.expected,
                    output.status.code(),
                    String::from_utf8_lossy(&output.stderr)
                ));
        }
    }
    assert!(failures.is_empty(), "{}", failures.join("\n"));
    Ok(())
}

#[test]
fn submit_reports_receipted_only_with_a_pinned_key_signature() -> TestResult {
    let mut entry = recorded_submissions("vector-v1-receipted-positive.json")?.remove(0);
    let key_id: [u8; 32] = unhex(json_string(&entry.keys, "keyId")?)?
        .try_into()
        .map_err(|_| "key id length")?;
    let public: [u8; 32] = unhex(json_string(&entry.keys, "publicKey")?)?
        .try_into()
        .map_err(|_| "public key length")?;
    let height = u64::from_str_radix(
        json_string(&entry.receipt, "height")?.trim_start_matches("0x"),
        16,
    )?;
    let original = entry.receipt.clone();
    for (first, last, committed) in [
        (height - 1, height + 1, false),
        (height, height + 1, false),
        (height - 1, height, false),
        (height, height, true),
    ] {
        entry.keys = pinned_key(key_id, public, first, last);
        entry.receipt = original.clone();
        if committed {
            entry.receipt = entry.receipt.replacen("\"receipted\"", "\"committed\"", 1);
            let closing = entry.receipt.rfind('}').ok_or("receipt object")?;
            entry.receipt.insert_str(
                closing,
                &format!(
                    ",\"blockNumber\":\"{height:#x}\",\"blockHash\":\"0x{}\"",
                    hex(&[10; 32])
                ),
            );
        }
        assert_output(&submit_recorded(&entry)?, 0, &format!("{{\"state\":\"receipted\",\"transaction_hash\":\"{}\",\"code\":\"LXP_OK\",\"result_code\":0,\"class\":\"terminal\"}}\n", entry.hash))?;
    }
    entry.receipt = original;
    for keys in [
        pinned_key([12; 32], public, height - 1, height + 1),
        pinned_key(
            key_id,
            SigningKey::from_bytes(&[0x49; 32])
                .verifying_key()
                .to_bytes(),
            height - 1,
            height + 1,
        ),
        pinned_key(key_id, public, height + 1, height + 2),
        pinned_key(key_id, public, height - 2, height - 1),
    ] {
        entry.keys = keys;
        assert_output(&submit_recorded(&entry)?, 3, &format!("{{\"state\":\"submitted\",\"transaction_hash\":\"{}\",\"verification_failure\":\"SequencerSignature\"}}\n",entry.hash))?;
    }
    let bytes = unhex(json_string(&entry.receipt, "receipt")?)?;
    let failure = layerx_proof::receipt::verify_sequencer_signature(
        &bytes,
        crate::ReceiptKeys::HeightRanges(&[]),
    )
    .err()
    .ok_or("empty pins accepted")?;
    assert_eq!(failure.check, crate::ReceiptCheck::SequencerSignature);
    entry.keys = "{\"keys\":[]}".to_owned();
    assert_output(&submit_recorded(&entry)?,3,&format!("{{\"state\":\"submitted\",\"transaction_hash\":\"{}\",\"receipt_keys_error\":{{\"transport_error\":\"Malformed\"}}}}\n",entry.hash))?;
    let mut revert = recorded_submissions("vector-v2-receipted-revert.json")?.remove(0);
    assert_output(&submit_recorded(&revert)?, 1, &format!("{{\"state\":\"receipted\",\"transaction_hash\":\"{}\",\"code\":\"LXP_UNKNOWN\",\"result_code\":1,\"class\":\"terminal\"}}\n",revert.hash))?;
    resign_result(&mut revert, -400, 1)?;
    assert_output(&submit_recorded(&revert)?, 1, &format!("{{\"state\":\"receipted\",\"transaction_hash\":\"{}\",\"code\":\"LXP_INSUFFICIENT_BALANCE\",\"result_code\":-400,\"class\":\"retriable\"}}\n",revert.hash))?;
    resign_result(&mut revert, -300, -400)?;
    assert_output(&submit_recorded(&revert)?,1,&format!("{{\"state\":\"receipted\",\"transaction_hash\":\"{}\",\"code\":\"LXP_SEQUENCE_GAP\",\"result_code\":-300,\"class\":\"terminal\"}}\n",revert.hash))?;
    Ok(())
}

fn assert_output(output: &Output, code: i32, expected: &str) -> TestResult {
    assert_eq!(
        output.status.code(),
        Some(code),
        "{}",
        String::from_utf8_lossy(&output.stdout)
    );
    assert!(output.stderr.is_empty());
    assert_eq!(std::str::from_utf8(&output.stdout)?, expected);
    Ok(())
}

fn resign_result(entry: &mut RecordedSubmit, result: i32, previous_result: i32) -> TestResult {
    let old = json_string(&entry.receipt, "receipt")?.to_owned();
    let mut signed = unhex(&old)?;
    let decoded = layerx_wire::receipt::decode(&signed).map_err(|e| format!("decode: {e:?}"))?;
    // The fixed prefix ends with the result code, before the effect sequence.
    assert_eq!(
        decoded.protocol().ok_or("protocol receipt")?.result_code(),
        previous_result
    );
    assert_eq!(&signed[158..162], &previous_result.to_be_bytes());
    signed[158..162].copy_from_slice(&result.to_be_bytes());
    let changed = layerx_wire::receipt::decode(&signed).map_err(|e| format!("changed: {e:?}"))?;
    assert_eq!(
        changed.protocol().ok_or("protocol receipt")?.result_code(),
        result
    );
    let unsigned =
        layerx_wire::receipt::encode_unsigned(&changed).map_err(|e| format!("unsigned: {e:?}"))?;
    let signer = SigningKey::from_bytes(&[0x47; 32]);
    let digest = receipt_digest(&unsigned).map_err(|e| format!("digest: {e:?}"))?;
    let signature_start = signed.len().checked_sub(64).ok_or("signature length")?;
    signed[signature_start..].copy_from_slice(&signer.sign(&digest).to_bytes());
    let changed = layerx_wire::receipt::decode(&signed).map_err(|e| format!("signed: {e:?}"))?;
    assert_eq!(
        layerx_wire::receipt::encode(&changed).map_err(|e| format!("canonical: {e:?}"))?,
        signed
    );
    let replacement = format!("0x{}", hex(&signed));
    assert_ne!(old, replacement);
    assert_eq!(entry.receipt.matches(&old).count(), 1);
    entry.receipt = entry.receipt.replacen(&old, &replacement, 1);
    assert_eq!(json_string(&entry.receipt, "receipt")?, replacement);
    let transported =
        layerx_wire::receipt::decode(&unhex(json_string(&entry.receipt, "receipt")?)?)
            .map_err(|e| format!("transported: {e:?}"))?;
    assert_eq!(
        transported
            .protocol()
            .ok_or("protocol receipt")?
            .result_code(),
        result
    );
    let key_id: [u8; 32] = unhex(json_string(&entry.receipt, "keyId")?)?
        .try_into()
        .map_err(|_| "key id length")?;
    let old_public = json_string(&entry.receipt, "signingKey")?.to_owned();
    entry.receipt = entry.receipt.replace(
        &old_public,
        &format!("0x{}", hex(&signer.verifying_key().to_bytes())),
    );
    entry.keys = pinned_key(key_id, signer.verifying_key().to_bytes(), 1000, 1999);
    Ok(())
}

#[test]
fn submit_names_the_failing_verification_step() -> TestResult {
    let signer = SigningKey::from_bytes(&[0x48; 32]);
    let key_id = [11; 32];
    let keys = pinned_key(key_id, signer.verifying_key().to_bytes(), 100, 200);
    let transaction = layerx_crypto::evm_transaction::transaction_hash(&[0x02, 0xf8, 0x6b]);
    let mut height = [0; 32];
    height[24..].copy_from_slice(&150_u64.to_be_bytes());
    let mut tampered = signed_receipt(&signer, transaction, key_id, height, 0)?;
    let last = tampered.last_mut().ok_or("empty signed receipt")?;
    *last ^= 1;
    let mut invalid_height = height;
    invalid_height[0] = 1;
    for (bytes, check) in [
        (vec![0xff], "Decode"),
        (
            encoded_receipt(transaction, key_id, height, 0, None),
            "MissingSignature",
        ),
        (tampered, "SequencerSignature"),
        (
            signed_receipt(&signer, transaction, key_id, invalid_height, 0)?,
            "BatchId",
        ),
        (
            signed_receipt(&signer, [0; 32], key_id, height, 0)?,
            "ActivityId",
        ),
        (
            signed_receipt(&signer, [13; 32], key_id, height, 0)?,
            "ActivityId",
        ),
    ] {
        let output = submit_receipt(&bytes, &keys, false)?;
        assert_eq!(output.status.code(), Some(3), "{check}");
        if check == "Decode" {
            let failure = layerx_proof::receipt::verify_sequencer_signature(
                &bytes,
                crate::ReceiptKeys::HeightRanges(&[]),
            )
            .err()
            .ok_or("malformed receipt accepted")?;
            assert_eq!(failure.check, crate::ReceiptCheck::Decode);
            assert_eq!(String::from_utf8(output.stdout)?, format!("{{\"state\":\"submitted\",\"transaction_hash\":\"0x{}\",\"receipt_error\":{{\"transport_error\":\"Malformed\"}}}}\n",hex(&transaction)));
        } else {
            assert_eq!(String::from_utf8(output.stdout)?, format!("{{\"state\":\"submitted\",\"transaction_hash\":\"0x{}\",\"verification_failure\":\"{check}\"}}\n", hex(&transaction)));
        }
    }
    Ok(())
}

#[test]
fn gateway_envelope_and_all_receipt_fields_are_required() -> TestResult {
    let mut control = recorded_vectors::refusal_control().remove(0);
    assert_output(&submit_recorded(&control)?, 0, &format!("{{\"state\":\"receipted\",\"transaction_hash\":\"{}\",\"code\":\"LXP_OK\",\"result_code\":0,\"class\":\"terminal\"}}\n",control.hash))?;
    let keys = control.keys.clone();
    let bare = keys
        .strip_prefix("{\"keys\":")
        .and_then(|s| s.strip_suffix('}'))
        .ok_or("key envelope")?;
    for malformed in [
        bare.to_owned(),
        keys.replace("\"firstHeight\":1000", "\"firstHeight\":\"0x3e8\"")
            .replace("\"lastHeight\":1999", "\"lastHeight\":\"0x7cf\""),
    ] {
        assert_ne!(malformed, keys);
        control.keys = malformed;
        assert_output(&submit_recorded(&control)?,3,&format!("{{\"state\":\"submitted\",\"transaction_hash\":\"{}\",\"receipt_keys_error\":{{\"transport_error\":\"Malformed\"}}}}\n",control.hash))?;
    }
    control.keys = keys;
    let original = control.receipt.clone();
    for field in ["paymentDenom", "signingKey"] {
        let value = json_string(&original, field)?;
        let mut start = original
            .find(&format!("\"{field}\""))
            .ok_or("required field")?;
        let mut end = start + original[start..].find(value).ok_or("field value")? + value.len() + 1;
        if original[end..].trim_start().starts_with(',') {
            end += original[end..].find(',').ok_or("following comma")? + 1;
        } else {
            start = original[..start].rfind(',').ok_or("preceding comma")?;
        }
        assert!(original[start..end].contains(value));
        control.receipt = format!("{}{}", &original[..start], &original[end..]);
        assert_output(&submit_recorded(&control)?,3,&format!("{{\"state\":\"submitted\",\"transaction_hash\":\"{}\",\"receipt_error\":{{\"transport_error\":\"Malformed\"}}}}\n",control.hash))?;
    }
    Ok(())
}

#[test]
fn submit_verifies_the_recorded_v4_chain_125_transaction() -> TestResult {
    let entries = recorded_submissions("vector-v4-committed.json")?;
    assert_eq!(entries.len(), 1);
    let entry = &entries[0];
    let raw = unhex(&entry.raw)?;
    assert_eq!(raw[0], 2, "committed transaction must be EIP-1559");
    let payload_start = if raw[1] > 0xf7 {
        2 + usize::from(raw[1] - 0xf7)
    } else {
        2
    };
    assert_eq!(
        raw[payload_start], 125,
        "committed transaction must use chain 125"
    );
    assert_output(&submit_recorded(entry)?, 0, &format!("{{\"state\":\"receipted\",\"transaction_hash\":\"{}\",\"code\":\"LXP_OK\",\"result_code\":0,\"class\":\"terminal\"}}\n", entry.hash))?;
    Ok(())
}
