//! Recorded node evidence through the production API and Programs verifier.
//!
//! Native proof rules use independent test crypto and reference vectors;
//! no executable, native artifact or extra Cargo dependency is required.
//! Source attribution: independent/PROVENANCE.md.
#[path = "independent/mod.rs"]
mod independent;
use self::independent::chain_proof::{
    header::ChainHeader,
    merkle::{hash_from_byte_slices, SimpleProof},
    proto::{ProtoReader, ProtoValue},
    tx::verify_tx,
    validators::{TrustedValidator, ValidatorSet},
    vote::{BlockId, CommitSig, PartSetHeader},
    ChainProofError, ChainProofVerifier, Commit,
};
use self::independent::{verify_ed25519, Sha256};
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::proof::ProofBundle;
use layerx_agent_api::read::{Freshness, RelativeTo, VerifiedRead};
use layerx_agent_api::verify::Rung;
use layerx_client::rpc::receipt_commit::SequenceBatch;
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{decode_summary, verify_logs, SequencerLog};
use layerx_types::json::{decode_hex, parse, JsonValue};

pub const RECORDED: &str = include_str!("../fixtures/committed-landing-block.json");

pub fn document() -> JsonValue {
    parse(RECORDED).expect("committed node recording")
}

fn quantity(value: &JsonValue, field: &str) -> u64 {
    u64::from_str_radix(value.str_at(field).unwrap().strip_prefix("0x").unwrap(), 16).unwrap()
}

pub fn produced() -> VerifiedRead<ProofBundle> {
    let recording = document();
    let value = recording.field("bundle").unwrap();
    let bytes = |name| CanonicalBytes::new(value.hex_at(name).unwrap()).unwrap();
    let height = quantity(value, "receiptJson.height");
    VerifiedRead::new(
        ProofBundle {
            format: value.str_at("format").unwrap().into(),
            verification_level: Rung::Committed,
            raw_transaction: bytes("rawTransaction"),
            signed_receipt: bytes("signedReceipt"),
            receipt_digest: value.hex_array_at("receiptDigest").unwrap(),
            receipt_key: ReceiptKeyEntry {
                key_id: value.hex_array_at("receiptKey.keyId").unwrap(),
                public_key: value.hex_array_at("receiptKey.publicKey").unwrap(),
                first_height: quantity(value, "receiptKey.firstHeight"),
                last_height: quantity(value, "receiptKey.lastHeight"),
            },
            block_header: Some(bytes("blockHeader")),
            receipt_inclusion: Some(bytes("receiptInclusion")),
            commit: Some(bytes("commit")),
            validator_set: Some(bytes("validatorSet")),
            account_proof: None,
        },
        Rung::Committed,
        Freshness {
            chain_head: height + 1,
            value_height: height,
            relative_to: RelativeTo::Block {
                height,
                hash: recording.hex_array_at("executionBlockHash").unwrap(),
            },
        },
    )
}

fn fields(bytes: &[u8]) -> Result<Vec<(u32, ProtoValue<'_>)>, ChainProofError> {
    let mut reader = ProtoReader::new(bytes);
    let mut result = Vec::new();
    while let Some(field) = reader.next_field()? {
        result.push((field.number, field.value));
    }
    reader.finish()?;
    Ok(result)
}

fn data<'a>(fields: &[(u32, ProtoValue<'a>)], number: u32) -> Result<&'a [u8], ChainProofError> {
    let mut values = fields.iter().filter(|(n, _)| *n == number);
    let value = match values.next() {
        Some((_, ProtoValue::Bytes(bytes))) => *bytes,
        None => &[],
        _ => return Err(ChainProofError::NonCanonical),
    };
    if values.next().is_some() {
        return Err(ChainProofError::NonCanonical);
    }
    Ok(value)
}

fn integer(fields: &[(u32, ProtoValue<'_>)], number: u32) -> Result<u64, ChainProofError> {
    let mut values = fields.iter().filter(|(n, _)| *n == number);
    let value = match values.next() {
        Some((_, ProtoValue::Varint(value))) => *value,
        None => 0,
        _ => return Err(ChainProofError::NonCanonical),
    };
    if values.next().is_some() {
        return Err(ChainProofError::NonCanonical);
    }
    Ok(value)
}

fn block_id(bytes: &[u8]) -> Result<BlockId, ChainProofError> {
    let f = fields(bytes)?;
    let parts = fields(data(&f, 2)?)?;
    Ok(BlockId {
        hash: data(&f, 1)?.to_vec(),
        part_set_header: PartSetHeader {
            total: integer(&parts, 1)?
                .try_into()
                .map_err(|_| ChainProofError::NonCanonical)?,
            hash: data(&parts, 2)?.to_vec(),
        },
    })
}

fn commit(bytes: &[u8]) -> Result<Commit, ChainProofError> {
    let f = fields(bytes)?;
    let mut signatures = Vec::new();
    for (number, value) in &f {
        match (number, value) {
            (1..=3, _) => {}
            (4, ProtoValue::Bytes(bytes)) => signatures.push(CommitSig::decode(bytes)?),
            _ => return Err(ChainProofError::UnknownField),
        }
    }
    Ok(Commit {
        height: integer(&f, 1)?
            .try_into()
            .map_err(|_| ChainProofError::NonCanonical)?,
        round: integer(&f, 2)?
            .try_into()
            .map_err(|_| ChainProofError::NonCanonical)?,
        block_id: block_id(data(&f, 3)?)?,
        signatures,
    })
}

fn validator(bytes: &[u8]) -> Result<TrustedValidator, ChainProofError> {
    let f = fields(bytes)?;
    let key = fields(data(&f, 2)?)?;
    Ok(TrustedValidator {
        address: data(&f, 1)?
            .try_into()
            .map_err(|_| ChainProofError::NonCanonical)?,
        public_key: data(&key, 1)?
            .try_into()
            .map_err(|_| ChainProofError::NonCanonical)?,
        voting_power: integer(&f, 3)?,
    })
}

fn validators(bytes: &[u8]) -> Result<ValidatorSet, ChainProofError> {
    let f = fields(bytes)?;
    let mut values = Vec::new();
    for (number, value) in &f {
        match (number, value) {
            (1, ProtoValue::Bytes(bytes)) => values.push(validator(bytes)?),
            (2, ProtoValue::Bytes(_)) | (3, ProtoValue::Varint(_)) => {}
            _ => return Err(ChainProofError::UnknownField),
        }
    }
    let proposer = validator(data(&f, 2)?)?;
    if !values.contains(&proposer) {
        return Err(ChainProofError::Mismatch);
    }
    let set = ValidatorSet::new(values)?;
    if set.total_power() != integer(&f, 3)? {
        return Err(ChainProofError::Mismatch);
    }
    Ok(set)
}

fn check(condition: bool) -> Result<(), String> {
    condition
        .then_some(())
        .ok_or_else(|| "evidence binding mismatch".into())
}

fn hex_text(bytes: &[u8]) -> Result<Vec<u8>, String> {
    decode_hex(&format!(
        "0x{}",
        std::str::from_utf8(bytes)
            .map_err(|e| e.to_string())?
            .trim_start_matches("0x")
    ))
    .map_err(str::to_owned)
}

fn recorded_proof(recording: &JsonValue, name: &str) -> SimpleProof {
    // Transaction proofs commit SHA256(tx), whereas result proofs commit the
    // deterministic protobuf result directly. Keep the node's leaf_hash.
    let p = recording.field(name).unwrap();
    SimpleProof {
        total: p
            .str_at("total")
            .unwrap()
            .parse::<u64>()
            .unwrap()
            .try_into()
            .unwrap(),
        index: p
            .str_at("index")
            .unwrap()
            .parse::<u64>()
            .unwrap()
            .try_into()
            .unwrap(),
        leaf_hash: decode_base64(p.str_at("leaf_hash").unwrap())
            .try_into()
            .unwrap(),
        aunts: p
            .array_at("aunts")
            .unwrap()
            .iter()
            .map(|a| {
                let JsonValue::String(text) = a else {
                    panic!("recorded aunt string")
                };
                decode_base64(text).try_into().unwrap()
            })
            .collect(),
    }
}

// Only a representation conversion for committed RPC fixture strings. All
// hashes and signatures are checked by the independent verifier below.
fn decode_base64(text: &str) -> Vec<u8> {
    let alphabet = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut word = 0_u32;
    let mut bits = 0;
    let mut out = Vec::new();
    for byte in text.bytes().take_while(|b| *b != b'=') {
        let digit = alphabet
            .iter()
            .position(|b| *b == byte)
            .expect("base64 digit");
        word = (word << 6) | digit as u32;
        bits += 6;
        if bits >= 8 {
            bits -= 8;
            out.push((word >> bits) as u8);
        }
    }
    out
}

fn minimal_integer(number: u128) -> Vec<u8> {
    let bytes = number.to_be_bytes();
    bytes[bytes.iter().position(|b| *b != 0).unwrap_or(bytes.len())..].to_vec()
}
fn rlp(payload: &[u8], list: bool) -> Vec<u8> {
    if !list && payload.len() == 1 && payload[0] < 128 {
        return payload.to_vec();
    }
    let prefix = if list { 0xc0 } else { 0x80 };
    let mut result = if payload.len() < 56 {
        vec![prefix + payload.len() as u8]
    } else {
        let length = minimal_integer(payload.len() as u128);
        let mut r = vec![prefix + 55 + length.len() as u8];
        r.extend(length);
        r
    };
    result.extend_from_slice(payload);
    result
}
fn legacy_transaction(transaction: &[u8]) -> Result<Vec<u8>, String> {
    let err = |e: ChainProofError| e.to_string();
    let raw = fields(transaction).map_err(err)?;
    let body = fields(data(&raw, 1).map_err(err)?).map_err(err)?;
    let any = fields(data(&body, 1).map_err(err)?).map_err(err)?;
    check(data(&any, 1).map_err(err)? == b"/paxprotocol.paxchain.evm.MsgEVMTransaction")?;
    let msg = fields(data(&any, 2).map_err(err)?).map_err(err)?;
    let envelope = fields(data(&msg, 1).map_err(err)?).map_err(err)?;
    check(data(&envelope, 1).map_err(err)? == b"/paxprotocol.paxchain.eth.LegacyTx")?;
    let tx = fields(data(&envelope, 2).map_err(err)?).map_err(err)?;
    let decimal = |number| -> Result<Vec<u8>, String> {
        let text =
            std::str::from_utf8(data(&tx, number).map_err(err)?).map_err(|e| e.to_string())?;
        Ok(minimal_integer(
            text.parse::<u128>().map_err(|e| e.to_string())?,
        ))
    };
    let mut items = Vec::new();
    for value in [
        minimal_integer(u128::from(integer(&tx, 1).map_err(err)?)),
        decimal(2)?,
        minimal_integer(u128::from(integer(&tx, 3).map_err(err)?)),
        hex_text(data(&tx, 4).map_err(err)?)?,
        decimal(5)?,
        data(&tx, 6).map_err(err)?.to_vec(),
        data(&tx, 7).map_err(err)?.to_vec(),
        data(&tx, 8).map_err(err)?.to_vec(),
        data(&tx, 9).map_err(err)?.to_vec(),
    ] {
        items.extend(rlp(&value, false));
    }
    Ok(rlp(&items, true))
}

/// Independent authentication after the shared daemon/SDK codec crossing.
/// Trust pins come from the committed test recording, never from the response.
pub fn verify(read: &VerifiedRead<ProofBundle>) -> Result<(), String> {
    let err = |e: ChainProofError| e.to_string();
    read.validate().map_err(|e| format!("{e:?}"))?;
    let recording = document();
    let trusted = produced();
    let pinned_key = trusted.value.receipt_key;
    let bundle = &read.value;
    check(bundle.receipt_key == pinned_key)?;
    let pins = [SequencerAuthorization::new(
        pinned_key.key_id,
        pinned_key.public_key,
        pinned_key.first_height,
        pinned_key.last_height,
    )];
    let receipt = verify_sequencer_signature(bundle.signed_receipt.as_bytes(), &pins)
        .map_err(|e| format!("{e:?}"))?;
    let protocol = receipt.protocol().ok_or("protocol receipt")?;
    let summary = decode_summary(&receipt).map_err(|e| format!("{e:?}"))?;
    // The production verifier enforced canonical encoding; replace its final
    // optional signature tail with the canonical absent-signature marker.
    let signed = bundle.signed_receipt.as_bytes();
    let tail = signed.len().checked_sub(69).ok_or("signature tail")?;
    check(signed[tail..tail + 5] == [1, 0, 0, 0, 64])?;
    let mut unsigned = signed[..tail].to_vec();
    unsigned.push(0);
    let mut hasher = Sha256::new();
    hasher.update(b"LXP/v1/receipt\0");
    hasher.update(unsigned);
    let digest = hasher.finalize();
    check(bundle.receipt_digest == digest)?;
    let required = |value: &Option<CanonicalBytes>| {
        value.as_ref().cloned().ok_or("missing evidence".to_owned())
    };
    let header = ChainHeader::decode(required(&bundle.block_header)?.as_bytes()).map_err(err)?;
    let commit = commit(required(&bundle.commit)?.as_bytes()).map_err(err)?;
    let served_validators = validators(required(&bundle.validator_set)?.as_bytes()).map_err(err)?;
    let pinned_validators =
        validators(required(&trusted.value.validator_set)?.as_bytes()).map_err(err)?;
    check(served_validators == pinned_validators)?;
    let verifier = ChainProofVerifier {
        chain_id: "committed-fixture".into(),
        validators: pinned_validators,
    };
    let head = verifier.verify_head(&header, &commit).map_err(err)?;
    check(head.header_hash() == recording.hex_array_at("commitmentBlockHash").unwrap())?;
    let execution =
        ChainHeader::decode(&recording.hex_at("executionHeader").unwrap()).map_err(err)?;
    let execution_commit =
        self::commit(&recording.hex_at("executionCommit").unwrap()).map_err(err)?;
    let landing = verifier
        .verify_head(&execution, &execution_commit)
        .map_err(err)?;
    check(
        header.height == execution.height + 1
            && header.last_block_id.hash == landing.header_hash()
            && execution.next_validators_hash == header.validators_hash
            && landing.header_hash() == recording.hex_array_at::<32>("executionBlockHash").unwrap()
            && read.freshness == trusted.freshness,
    )?;
    let transaction = recording.hex_at("transaction").unwrap();
    let tx_proof = recorded_proof(&recording, "transactionProof");
    verify_tx(&landing, &transaction, &tx_proof).map_err(err)?;
    check(legacy_transaction(&transaction)? == bundle.raw_transaction.as_bytes())?;
    let batch = recording.hex_at("batchTransaction").unwrap();
    let batch_proof = recorded_proof(&recording, "batchProof");
    verify_tx(&landing, &batch, &batch_proof).map_err(err)?;
    let batch = SequenceBatch::decode(&batch).map_err(|e| format!("{e:?}"))?;
    check(
        batch_proof.index == 0
            && batch.height == read.freshness.value_height
            && batch.from_sequence == protocol.global_sequence()
            && batch.to_sequence == protocol.global_sequence()
            && batch.key_id == pinned_key.key_id
            && pinned_key.first_height <= batch.height
            && batch.height <= pinned_key.last_height
            && batch.previous_root == protocol.previous_state_root()
            && batch.resulting_root == protocol.resulting_state_root()
            && batch.activity_root == Sha256::digest(protocol.activity_id()),
    )?;
    let mut sequence = Sha256::new();
    sequence.update(b"PAXEER/v1/sequence\0");
    sequence.update(batch.previous_root);
    sequence.update(batch.from_sequence.to_be_bytes());
    sequence.update(protocol.activity_id());
    check(sequence.finalize() == batch.resulting_root)?;
    let digest = batch
        .signing_digest(landing.chain_id())
        .map_err(|e| format!("{e:?}"))?;
    check(verify_ed25519(
        &pinned_key.public_key,
        &digest,
        &batch.signature,
    ))?;
    let inclusion = required(&bundle.receipt_inclusion)?;
    let inclusion = parse(std::str::from_utf8(inclusion.as_bytes()).map_err(|e| e.to_string())?)
        .map_err(|e| format!("{e:?}"))?;
    check(
        inclusion.str_at("algorithm").unwrap() == "comet-abci-results-v1"
            && inclusion.u64_at("height").unwrap() == read.freshness.value_height,
    )?;
    let result =
        decode_hex(&format!("0x{}", inclusion.str_at("result").unwrap())).map_err(str::to_owned)?;
    let proof = SimpleProof {
        total: inclusion.u64_at("total").unwrap().try_into().unwrap(),
        index: inclusion.u64_at("index").unwrap().try_into().unwrap(),
        leaf_hash: decode_hex(&format!("0x{}", inclusion.str_at("leaf_hash").unwrap()))
            .unwrap()
            .try_into()
            .unwrap(),
        aunts: inclusion
            .array_at("aunts")
            .unwrap()
            .iter()
            .map(|a| {
                let JsonValue::String(text) = a else {
                    panic!("hex aunt")
                };
                decode_hex(&format!("0x{text}"))
                    .unwrap()
                    .try_into()
                    .unwrap()
            })
            .collect(),
    };
    check(
        proof.index == i64::from(summary.tx_index())
            && proof.index == tx_proof.index
            && proof.total == tx_proof.total,
    )?;
    proof
        .verify(&head.last_results_hash(), &result)
        .map_err(err)?;
    check(hash_from_byte_slices(&[&result]) == proof.leaf_hash)?;
    let result_fields = fields(&result).map_err(err)?;
    check(
        integer(&result_fields, 1).map_err(err)? == 0
            && integer(&result_fields, 6).map_err(err)? == summary.gas_used(),
    )?;
    let messages = fields(data(&result_fields, 2).map_err(err)?).map_err(err)?;
    let message = fields(data(&messages, 1).map_err(err)?).map_err(err)?;
    check(data(&message, 1).map_err(err)? == b"/paxprotocol.paxchain.evm.MsgEVMTransaction")?;
    let response = fields(data(&message, 2).map_err(err)?).map_err(err)?;
    check(
        hex_text(data(&response, 4).map_err(err)?)? == protocol.activity_id()
            && integer(&response, 1).map_err(err)? == summary.gas_used()
            && data(&response, 2).map_err(err)?.is_empty()
            && protocol.result_code() == 0
            && data(&response, 3).map_err(err)?.len() == summary.return_data_len() as usize
            && Sha256::digest(data(&response, 3).map_err(err)?) == protocol.activity_root()
            && Sha256::digest(data(&response, 2).map_err(err)?) == summary.vm_error_hash(),
    )?;
    let mut logs = Vec::new();
    for (number, value) in &response {
        if *number != 5 {
            continue;
        }
        let ProtoValue::Bytes(bytes) = value else {
            return Err("log wire type".into());
        };
        let log = fields(bytes).map_err(err)?;
        let topics = log
            .iter()
            .filter(|(n, _)| *n == 2)
            .map(|(_, v)| {
                let ProtoValue::Bytes(bytes) = v else {
                    return Err("topic wire type".into());
                };
                hex_text(bytes)?
                    .try_into()
                    .map_err(|_| "topic width".to_owned())
            })
            .collect::<Result<_, String>>()?;
        logs.push(SequencerLog::new(
            hex_text(data(&log, 1).map_err(err)?)?
                .try_into()
                .map_err(|_| "address width")?,
            topics,
            data(&log, 3).map_err(err)?.to_vec(),
            integer(&log, 4).map_err(err)?,
        ));
    }
    verify_logs(&receipt, logs).map_err(|e| format!("{e:?}"))?;
    Ok(())
}

/// Both ends call the same public request/response codecs used by daemon RPC
/// adapters and SDK consumers. The peer is a loopback socket, not a codec double.
pub fn round_trip(
    target: layerx_agent_api::proof::ProofBundleTarget,
    requested: Rung,
) -> VerifiedRead<ProofBundle> {
    use layerx_agent_api::read::ReadRequest;
    use std::io::{Read, Write};
    use std::net::{Shutdown, TcpListener, TcpStream};
    use std::time::Duration;
    let listener = TcpListener::bind(("127.0.0.1", 0)).unwrap();
    let address = listener.local_addr().unwrap();
    let selector = target.encode().unwrap();
    let request =
        ReadRequest::proof_bundle(selector.as_bytes().to_vec(), requested.name()).unwrap();
    let server = std::thread::spawn(move || {
        let (mut stream, _) = listener.accept().unwrap();
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap();
        let mut incoming = Vec::new();
        stream.read_to_end(&mut incoming).unwrap();
        let server_request = ReadRequest::proof_bundle(incoming, requested.name()).unwrap();
        let response = produced();
        server_request.check_proof_response(&response).unwrap();
        stream
            .write_all(response.encode_json().unwrap().as_bytes())
            .unwrap();
    });
    let mut stream = TcpStream::connect(address).unwrap();
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap();
    stream.write_all(selector.as_bytes()).unwrap();
    stream.shutdown(Shutdown::Write).unwrap();
    let mut response = String::new();
    stream.read_to_string(&mut response).unwrap();
    server.join().unwrap();
    let observed = VerifiedRead::<ProofBundle>::decode_json(&response).unwrap();
    request.check_proof_response(&observed).unwrap();
    assert_eq!(observed, produced());
    observed
}
