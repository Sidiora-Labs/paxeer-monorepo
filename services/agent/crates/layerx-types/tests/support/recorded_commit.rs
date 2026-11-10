//! Recorded producer transcript, independently authenticated before current
//! consumer construction. Reference proof/crypto comes from the existing API
//! test support; current consumer modules and shared JSON/result codecs are real.
use crate::independent::chain_proof::{
    header::ChainHeader,
    merkle::SimpleProof,
    proto::{ProtoReader, ProtoValue},
    tx::verify_tx,
    validators::{TrustedValidator, ValidatorSet},
    vote::{BlockId, CommitSig, PartSetHeader},
    ChainProofError, ChainProofVerifier, Commit, VerifiedChainHead,
};
use crate::independent::{verify_ed25519, Sha256};
use crate::write_contract::CanonicalBytes;
use layerx_types::json::{decode_hex, parse, JsonValue};
use layerx_types::result::ResultCode;

pub const RECORDED: &str =
    include_str!("../../../layerx-agent-api/tests/fixtures/committed-landing-block.json");
/// Decode the pinned committed transcript.
///
/// # Panics
/// Panics if the checked-in transcript is malformed.
pub fn document() -> JsonValue {
    parse(RECORDED).unwrap_or_else(|error| panic!("{}: {error:?}", "recorded node output"))
}
fn check(condition: bool) -> Result<(), String> {
    condition
        .then_some(())
        .ok_or_else(|| "evidence binding mismatch".into())
}
fn quantity(doc: &JsonValue, field: &str) -> u64 {
    u64::from_str_radix(
        doc.str_at(field)
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("test or reference input rejected")),
        16,
    )
    .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
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

fn recorded_proof(recording: &JsonValue, name: &str) -> SimpleProof {
    // Transaction proofs commit SHA256(tx), whereas result proofs commit the
    // deterministic protobuf result directly. Keep the node's leaf_hash.
    let p = recording
        .field(name)
        .unwrap_or_else(|| panic!("test or reference input rejected"));
    SimpleProof {
        total: p
            .str_at("total")
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
            .parse::<u64>()
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
            .try_into()
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
        index: p
            .str_at("index")
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
            .parse::<u64>()
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
            .try_into()
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
        leaf_hash: decode_base64(
            p.str_at("leaf_hash")
                .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
        )
        .try_into()
        .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
        aunts: p
            .array_at("aunts")
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
            .iter()
            .map(|a| {
                let JsonValue::String(text) = a else {
                    panic!("recorded aunt string")
                };
                decode_base64(text)
                    .try_into()
                    .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
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
            .unwrap_or_else(|| panic!("base64 digit"));
        word = (word << 6)
            | u32::try_from(digit)
                .unwrap_or_else(|error| panic!("base64 digit out of range: {error}"));
        bits += 6;
        if bits >= 8 {
            bits -= 8;
            out.push((word >> bits).to_le_bytes()[0]);
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
        vec![
            prefix
                + u8::try_from(payload.len())
                    .unwrap_or_else(|error| panic!("short RLP length: {error}")),
        ]
    } else {
        let length = minimal_integer(
            u128::try_from(payload.len())
                .unwrap_or_else(|error| panic!("RLP payload length: {error}")),
        );
        let mut r = vec![
            prefix
                + 55
                + u8::try_from(length.len())
                    .unwrap_or_else(|error| panic!("RLP length prefix: {error}")),
        ];
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

#[derive(Debug)]
pub struct Authenticated {
    pub transaction_id: [u8; 32],
    pub receipt_digest: [u8; 32],
    pub header_digest: [u8; 32],
    pub height: u64,
    pub result: ResultCode,
    pub signed_receipt: CanonicalBytes,
}

fn authenticate_chain(
    response: &JsonValue,
    trusted: &JsonValue,
) -> Result<VerifiedChainHead, String> {
    let err = |e: ChainProofError| e.to_string();
    let pinned = validators(
        &trusted
            .hex_at("bundle.validatorSet")
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
    )
    .map_err(err)?;
    let served = validators(
        &response
            .hex_at("bundle.validatorSet")
            .map_err(|e| e.to_string())?,
    )
    .map_err(err)?;
    check(served == pinned)?;
    let verifier = ChainProofVerifier {
        chain_id: "committed-fixture".into(),
        validators: pinned,
    };
    let header = ChainHeader::decode(
        &response
            .hex_at("bundle.blockHeader")
            .map_err(|e| e.to_string())?,
    )
    .map_err(err)?;
    let terminal = commit(
        &response
            .hex_at("bundle.commit")
            .map_err(|e| e.to_string())?,
    )
    .map_err(err)?;
    let head = verifier.verify_head(&header, &terminal).map_err(err)?;
    check(
        head.header_hash()
            == trusted
                .hex_array_at::<32>("commitmentBlockHash")
                .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
    )?;
    let execution = ChainHeader::decode(
        &response
            .hex_at("executionHeader")
            .map_err(|e| e.to_string())?,
    )
    .map_err(err)?;
    let execution_commit = commit(
        &response
            .hex_at("executionCommit")
            .map_err(|e| e.to_string())?,
    )
    .map_err(err)?;
    let landing = verifier
        .verify_head(&execution, &execution_commit)
        .map_err(err)?;
    check(
        header.height == execution.height + 1
            && header.last_block_id.hash == landing.header_hash()
            && execution.next_validators_hash == header.validators_hash
            && landing.header_hash()
                == trusted
                    .hex_array_at::<32>("executionBlockHash")
                    .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
    )?;

    Ok(landing)
}

fn authenticate_receipt_signature(
    response: &JsonValue,
    trusted: &JsonValue,
    signed: &[u8],
) -> Result<([u8; 32], [u8; 32]), String> {
    let tail = signed.len().checked_sub(69).ok_or("signature tail")?;
    check(signed.get(tail..tail + 5) == Some(&[1, 0, 0, 0, 64]))?;
    let mut unsigned = signed[..tail].to_vec();
    unsigned.push(0);
    let mut hash = Sha256::new();
    hash.update(b"LXP/v1/receipt\0");
    hash.update(&unsigned);
    let digest = hash.finalize();
    check(
        digest
            == response
                .hex_array_at::<32>("bundle.receiptDigest")
                .map_err(|e| e.to_string())?,
    )?;
    let public_key = trusted
        .hex_array_at("bundle.receiptKey.publicKey")
        .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"));
    check(
        response
            .path("bundle.receiptKey")
            .map_err(|e| e.to_string())?
            == trusted
                .path("bundle.receiptKey")
                .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
    )?;
    check(verify_ed25519(
        &public_key,
        &digest,
        signed[tail + 5..]
            .try_into()
            .map_err(|_| "signature length")?,
    ))?;
    Ok((digest, public_key))
}

// These pins are read from the original committed transcript, independently
// of the response under test. Altering a response never alters its trust root.
/// Authenticate the response against the pinned transcript and native proof rules.
///
/// # Errors
/// Returns an error for malformed proofs, invalid signatures, or mismatched commitments.
///
/// # Panics
/// Panics if the checked-in trust transcript is malformed.
pub fn authenticate(response: &JsonValue) -> Result<Authenticated, String> {
    let trusted = document();
    let landing = authenticate_chain(response, &trusted)?;
    let err = |e: ChainProofError| e.to_string();
    let transaction = response.hex_at("transaction").map_err(|e| e.to_string())?;
    let proof = recorded_proof(response, "transactionProof");
    verify_tx(&landing, &transaction, &proof).map_err(err)?;
    check(
        legacy_transaction(&transaction)?
            == response
                .hex_at("bundle.rawTransaction")
                .map_err(|e| e.to_string())?,
    )?;
    let batch = response
        .hex_at("batchTransaction")
        .map_err(|e| e.to_string())?;
    let proof = recorded_proof(response, "batchProof");
    check(proof.index == 0)?;
    verify_tx(&landing, &batch, &proof).map_err(err)?;

    let signed = response
        .hex_at("bundle.signedReceipt")
        .map_err(|e| e.to_string())?;
    let (digest, public_key) = authenticate_receipt_signature(response, &trusted, &signed)?;
    // Fixed prefix of the signed native receipt: structure/version, activity,
    // sequence, state roots, activity root, and exact result code.
    check(signed.len() >= 162 && signed[..10] == [0, 2, 0x52, 1, 0, 2, 0, 0, 0, 32])?;
    let id: [u8; 32] = signed[10..42]
        .try_into()
        .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"));
    check(
        id == response
            .hex_array_at::<32>("bundle.receiptJson.txHash")
            .map_err(|e| e.to_string())?,
    )?;
    let sequence = u64::from_be_bytes(
        signed[42..50]
            .try_into()
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
    );
    let result = ResultCode::from_wire_bytes(
        signed[158..162]
            .try_into()
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
    );
    let domain = b"PAXEER/v1/sequence-batch\0";
    let start = domain.len();
    check(batch.len() == 241 && batch.starts_with(domain))?;
    let height = u64::from_be_bytes(
        batch[start..start + 8]
            .try_into()
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
    );
    let from = u64::from_be_bytes(
        batch[start + 8..start + 16]
            .try_into()
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
    );
    let to = u64::from_be_bytes(
        batch[start + 16..start + 24]
            .try_into()
            .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}")),
    );
    check(
        height == u64::try_from(landing.height()).map_err(|_| "height")?
            && height == quantity(response, "bundle.receiptJson.height")
            && from == sequence
            && to == sequence
            && sequence == quantity(response, "bundle.receiptJson.seq")
            && batch[start + 24..start + 56] == signed[54..86]
            && batch[start + 56..start + 88] == signed[90..122]
            && batch[start + 88..start + 120] == Sha256::digest(id)
            && batch[start + 120..start + 152]
                == trusted
                    .hex_array_at::<32>("bundle.receiptKey.keyId")
                    .unwrap_or_else(|error| panic!("test or reference input rejected: {error:?}"))
            && quantity(&trusted, "bundle.receiptKey.firstHeight") <= height
            && height <= quantity(&trusted, "bundle.receiptKey.lastHeight"),
    )?;
    let mut hash = Sha256::new();
    hash.update(domain);
    hash.update(landing.chain_id());
    hash.update(&batch[start..start + 120]);
    check(verify_ed25519(
        &public_key,
        &hash.finalize(),
        batch[start + 152..]
            .try_into()
            .map_err(|_| "batch signature")?,
    ))?;
    Ok(Authenticated {
        transaction_id: id,
        receipt_digest: digest,
        header_digest: landing.header_hash(),
        height,
        result,
        signed_receipt: CanonicalBytes::new(signed).map_err(|e| format!("{e:?}"))?,
    })
}
