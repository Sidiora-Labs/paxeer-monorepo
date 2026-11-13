use super::{AbciResultProof, CheckpointReadError};
use layerx_programs::chain_proof::merkle::{hash_from_byte_slices, SimpleProof};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::time::Duration;

/// The node's native Comet RPC, explicitly configured separately from EVM RPC.
pub struct NativeResultSource {
    agent: ureq::Agent,
    url: String,
}

impl NativeResultSource {
    /// # Errors
    /// Returns an error if node acquisition fails or the committed checkpoint, target, result
    /// root or proof cannot be verified.
    pub fn connect(url: &str, ca_der: Option<&[u8]>) -> Result<Self, CheckpointReadError> {
        let roots: Vec<_> = ca_der.into_iter().collect();
        layerx_client::sequencer::SequencerEndpoint::new(url, &roots)
            .map_err(CheckpointReadError::Rpc)?;
        let tls = match ca_der {
            Some(ca) => crate::outbound_tls::private_ca(ca),
            None => crate::outbound_tls::system(url),
        }
        .ok_or(CheckpointReadError::InvalidProof)?;
        let agent = ureq::Agent::config_builder()
            .tls_config(tls)
            .timeout_global(Some(Duration::from_secs(30)))
            .max_redirects(0)
            .build()
            .into();
        Ok(Self {
            agent,
            url: url.to_owned(),
        })
    }

    pub(crate) fn rpc(&self, method: &str, params: &Value) -> Result<Value, CheckpointReadError> {
        let mut response = self
            .agent
            .post(&self.url)
            .send_json(json!({
                "jsonrpc":"2.0", "id":1, "method":method, "params":params
            }))
            .map_err(|_| CheckpointReadError::NotCommitted)?;
        let bytes = response
            .body_mut()
            .with_config()
            .limit(1_048_577)
            .read_to_vec()
            .map_err(|_| CheckpointReadError::InvalidProof)?;
        if bytes.len() > 1_048_576 {
            return Err(CheckpointReadError::Arithmetic);
        }
        let envelope: Value =
            serde_json::from_slice(&bytes).map_err(|_| CheckpointReadError::InvalidProof)?;
        if envelope["jsonrpc"] != "2.0"
            || envelope["id"] != 1
            || envelope.get("error").is_some()
            || !envelope["result"].is_object()
        {
            return Err(CheckpointReadError::InvalidProof);
        }
        Ok(envelope["result"].clone())
    }

    fn historical_query(&self, params: &Value) -> Result<Value, CheckpointReadError> {
        for retry in 0..=8 {
            let result = self.rpc("abci_query", params)?;
            let response = &result["response"];
            if response["code"].as_u64() != Some(36)
                || !response["log"]
                    .as_str()
                    .is_some_and(|log| log.starts_with("historical proof rate limited"))
                || retry == 8
            {
                return Ok(result);
            }
            std::thread::sleep(Duration::from_millis(250));
        }
        unreachable!()
    }

    /// # Errors
    /// Returns an error if node acquisition fails or the committed checkpoint, target, result
    /// root or proof cannot be verified.
    pub fn checkpoint(
        &self,
        height: u64,
        chain_id: &str,
        validators: &[super::CommitValidator],
    ) -> Result<super::ServedCheckpoint, CheckpointReadError> {
        let material = self.rpc("commit", &json!({"height":height.to_string()}))?;
        super::ServedCheckpoint::from_comet_json(
            height,
            chain_id,
            &material["signed_header"],
            validators,
        )
    }

    pub(crate) fn validators(
        &self,
        height: u64,
    ) -> Result<Vec<super::CommitValidator>, CheckpointReadError> {
        let mut validators = Vec::new();
        let mut expected_total = None;
        for page in 1..=41 {
            let result = self.rpc(
                "validators",
                &json!({"height":height.to_string(), "page":page.to_string(), "per_page":"100"}),
            )?;
            let decimal = |name: &str| {
                result[name]
                    .as_str()
                    .and_then(|text| text.parse::<u64>().ok())
            };
            let entries = result["validators"]
                .as_array()
                .ok_or(CheckpointReadError::Commit)?;
            let total = decimal("total")
                .filter(|n| *n > 0 && *n <= 4096)
                .ok_or(CheckpointReadError::Commit)?;
            if decimal("block_height") != Some(height)
                || decimal("count") != Some(entries.len() as u64)
                || entries.is_empty()
                || entries.len() > 100
                || expected_total.is_some_and(|n| n != total)
            {
                return Err(CheckpointReadError::Commit);
            }
            expected_total = Some(total);
            for entry in entries {
                if entry["pub_key"]["type"] != "tendermint/PubKeyEd25519" {
                    return Err(CheckpointReadError::Commit);
                }
                validators.push(super::CommitValidator {
                    address: layerx_programs::hex::decode(
                        entry["address"]
                            .as_str()
                            .ok_or(CheckpointReadError::Commit)?,
                    )
                    .map_err(|_| CheckpointReadError::Commit)?
                    .try_into()
                    .map_err(|_| CheckpointReadError::Commit)?,
                    public_key: decode_base64(
                        entry["pub_key"]["value"]
                            .as_str()
                            .ok_or(CheckpointReadError::Commit)?,
                    )?
                    .try_into()
                    .map_err(|_| CheckpointReadError::Commit)?,
                    voting_power: entry["voting_power"]
                        .as_str()
                        .and_then(|n| n.parse::<u64>().ok())
                        .filter(|n| *n > 0)
                        .ok_or(CheckpointReadError::Commit)?,
                });
            }
            if validators.len() as u64 == total {
                return Ok(validators);
            }
            if validators.len() as u64 > total {
                return Err(CheckpointReadError::Commit);
            }
        }
        Err(CheckpointReadError::Commit)
    }

    pub(crate) fn sequence_batch(
        &self,
        height: u64,
        root: [u8; 32],
    ) -> Result<(Vec<u8>, SimpleProof), CheckpointReadError> {
        const DOMAIN: &[u8] = b"PAXEER/v1/sequence-batch\0";

        let result = self.rpc("block", &json!({"height":height.to_string()}))?;
        let block = &result["block"];
        if block["header"]["height"]
            .as_str()
            .and_then(|text| text.parse::<u64>().ok())
            != Some(height)
        {
            return Err(CheckpointReadError::TargetMismatch);
        }
        let txs = block["data"]["txs"]
            .as_array()
            .filter(|txs| !txs.is_empty() && txs.len() <= 4096)
            .ok_or(CheckpointReadError::InvalidProof)?;
        let transactions = txs
            .iter()
            .map(|tx| decode_base64(tx.as_str().ok_or(CheckpointReadError::InvalidProof)?))
            .collect::<Result<Vec<_>, _>>()?;
        let transaction = transactions[0].clone();

        if !transaction.starts_with(DOMAIN) || transaction.len() != DOMAIN.len() + 216 {
            return Err(CheckpointReadError::InvalidProof);
        }
        let batch = &transaction[DOMAIN.len()..];
        let u64_at = |start| {
            u64::from_be_bytes(
                batch[start..start + 8]
                    .try_into()
                    .unwrap_or_else(|error| panic!("bounded sequence batch: {error:?}")),
            )
        };
        if u64_at(0) != height || u64_at(8) == 0 || u64_at(16) < u64_at(8) - 1 {
            return Err(CheckpointReadError::InvalidProof);
        }
        let leaves = transactions
            .iter()
            .map(|tx| Sha256::digest(tx).to_vec())
            .collect::<Vec<_>>();
        let proof = simple_proof(&leaves, 0)?;
        proof
            .verify(&root, &leaves[0])
            .map_err(|_| CheckpointReadError::RootMismatch)?;
        Ok((transaction, proof))
    }

    /// Returns the complete deterministic result leaves. The caller authenticates their
    /// Merkle root against the successor commit, including the empty tree.
    /// # Errors
    /// Returns an error if node acquisition fails or the committed checkpoint, target, result
    /// root or proof cannot be verified.
    pub fn all_results(&self, height: u64) -> Result<Vec<Vec<u8>>, CheckpointReadError> {
        let response = self.rpc("block_results", &json!({"height": height.to_string()}))?;
        if response["height"].as_str() != Some(height.to_string().as_str()) {
            return Err(CheckpointReadError::TargetMismatch);
        }
        let rows = response["txs_results"]
            .as_array()
            .ok_or(CheckpointReadError::InvalidProof)?;
        if rows.len() > 4096 {
            return Err(CheckpointReadError::Arithmetic);
        }
        rows.iter().map(result_bytes).collect()
    }

    /// # Errors
    /// Returns an error if node acquisition fails or the committed checkpoint, target, result
    /// root or proof cannot be verified.
    pub fn acquire(&self, height: u64, index: u64) -> Result<AbciResultProof, CheckpointReadError> {
        if height == 0 || height > i64::MAX as u64 {
            return Err(CheckpointReadError::TargetMismatch);
        }
        let result = self.rpc("block_results", &json!({"height":height.to_string()}))?;
        if result["height"]
            .as_str()
            .and_then(|text| text.parse::<u64>().ok())
            != Some(height)
        {
            return Err(CheckpointReadError::TargetMismatch);
        }
        Self::from_results(&result["txs_results"], index)
    }

    /// Encodes exactly the four fields retained by abci.MarshalTxResults.
    /// # Errors
    /// Returns an error if node acquisition fails or the committed checkpoint, target, result
    /// root or proof cannot be verified.
    pub fn from_results(
        results: &Value,
        index: u64,
    ) -> Result<AbciResultProof, CheckpointReadError> {
        let results = results
            .as_array()
            .filter(|results| !results.is_empty() && results.len() <= 4096)
            .ok_or(CheckpointReadError::InvalidProof)?;
        let index = usize::try_from(index)
            .ok()
            .filter(|index| *index < results.len())
            .ok_or(CheckpointReadError::TargetMismatch)?;
        let leaves = results
            .iter()
            .map(result_bytes)
            .collect::<Result<Vec<_>, _>>()?;
        if leaves.iter().map(Vec::len).sum::<usize>() > 1_048_576 {
            return Err(CheckpointReadError::Arithmetic);
        }
        Ok(AbciResultProof {
            result: leaves[index].clone(),
            proof: simple_proof(&leaves, index)?,
        })
    }
}

pub(crate) fn result_bytes(value: &Value) -> Result<Vec<u8>, CheckpointReadError> {
    let code = match value.get("code") {
        None => 0,
        Some(value) => value
            .as_u64()
            .and_then(|code| u32::try_from(code).ok())
            .ok_or(CheckpointReadError::InvalidProof)?,
    };
    let integer = |name: &str| -> Result<i64, CheckpointReadError> {
        if value.get(name).is_none() {
            return Ok(0);
        }
        value[name]
            .as_i64()
            .or_else(|| value[name].as_str()?.parse().ok())
            .ok_or(CheckpointReadError::InvalidProof)
    };
    let data = match &value["data"] {
        Value::Null => Vec::new(),
        Value::String(text) => decode_base64(text)?,
        _ => return Err(CheckpointReadError::InvalidProof),
    };
    let mut bytes = Vec::new();
    scalar(&mut bytes, 1, u64::from(code));
    blob(&mut bytes, 2, &data);
    scalar(
        &mut bytes,
        5,
        u64::from_le_bytes(integer("gas_wanted")?.to_le_bytes()),
    );
    scalar(
        &mut bytes,
        6,
        u64::from_le_bytes(integer("gas_used")?.to_le_bytes()),
    );
    Ok(bytes)
}

pub(crate) fn varint(bytes: &mut Vec<u8>, mut value: u64) {
    while value >= 128 {
        bytes.push((value.to_le_bytes()[0] & 127) | 128);
        value >>= 7;
    }
    bytes.push(value.to_le_bytes()[0]);
}
pub(crate) fn scalar(bytes: &mut Vec<u8>, field: u32, value: u64) {
    if value != 0 {
        varint(bytes, u64::from(field) << 3);
        varint(bytes, value);
    }
}
pub(crate) fn blob(bytes: &mut Vec<u8>, field: u32, value: &[u8]) {
    if !value.is_empty() {
        varint(bytes, (u64::from(field) << 3) | 2);
        varint(bytes, value.len() as u64);
        bytes.extend_from_slice(value);
    }
}

pub(crate) fn decode_base64(text: &str) -> Result<Vec<u8>, CheckpointReadError> {
    let bad = || CheckpointReadError::InvalidProof;
    if text.len() > 1_398_104 || !text.len().is_multiple_of(4) {
        return Err(bad());
    }
    let padding = text.bytes().rev().take_while(|byte| *byte == b'=').count();
    if padding > 2 {
        return Err(bad());
    }
    let mut bytes = Vec::with_capacity(text.len() / 4 * 3);
    let mut bits = 0u32;
    let mut count = 0;
    for byte in text.bytes().take(text.len() - padding) {
        let digit = match byte {
            b'A'..=b'Z' => byte - b'A',
            b'a'..=b'z' => byte - b'a' + 26,
            b'0'..=b'9' => byte - b'0' + 52,
            b'+' => 62,
            b'/' => 63,
            _ => return Err(bad()),
        };
        bits = (bits << 6) | u32::from(digit);
        count += 6;
        if count >= 8 {
            count -= 8;
            bytes.push((bits >> count).to_le_bytes()[0]);
        }
    }
    if bits & ((1 << count) - 1) != 0
        || (count == 0 && padding != 0)
        || (count == 2 && padding != 1)
        || (count == 4 && padding != 2)
        || count == 6
    {
        return Err(bad());
    }
    Ok(bytes)
}

fn simple_proof(leaves: &[Vec<u8>], index: usize) -> Result<SimpleProof, CheckpointReadError> {
    fn path(leaves: &[Vec<u8>], index: usize, aunts: &mut Vec<[u8; 32]>) {
        if leaves.len() <= 1 {
            return;
        }
        let split = 1usize << (leaves.len() - 1).ilog2();
        let (next, other, index) = if index < split {
            (&leaves[..split], &leaves[split..], index)
        } else {
            (&leaves[split..], &leaves[..split], index - split)
        };
        path(next, index, aunts);
        aunts.push(hash_from_byte_slices(
            &other.iter().map(Vec::as_slice).collect::<Vec<_>>(),
        ));
    }

    if leaves.is_empty() || index >= leaves.len() {
        return Err(CheckpointReadError::TargetMismatch);
    }

    let mut aunts = Vec::new();
    path(leaves, index, &mut aunts);
    let mut hash = Sha256::new();
    hash.update([0]);
    hash.update(&leaves[index]);
    Ok(SimpleProof {
        total: i64::try_from(leaves.len()).map_err(|_| CheckpointReadError::Arithmetic)?,
        index: i64::try_from(index).map_err(|_| CheckpointReadError::Arithmetic)?,
        leaf_hash: hash.finalize().into(),
        aunts,
    })
}

impl NativeResultSource {
    /// Reads the declared Programs query and authenticates its state record at H
    /// against the signed application root in checkpoint H+1. The storage key is
    /// supplied by the owning Programs deployment, never inferred from RPC data.
    /// # Errors
    /// Returns an error if node acquisition fails or the committed checkpoint, target, result
    /// root or proof cannot be verified.
    pub fn program_state_record(
        &self,
        program: [u8; 32],
        key: &[u8],
        store_name: &str,
        checkpoint: &super::ServedCheckpoint,
    ) -> Result<Vec<u8>, CheckpointReadError> {
        use super::abci_result::{fields, ProtoValue};

        crate::reject_if(
            program == [0; 32]
                || key.is_empty()
                || key.len() > 1024
                || store_name.is_empty()
                || store_name.len() > 128
                || checkpoint.height <= 1,
            || CheckpointReadError::TargetMismatch,
        )?;
        let height = checkpoint.height - 1;
        let mut request = Vec::new();
        blob(&mut request, 1, &program);
        blob(&mut request, 2, key);
        scalar(&mut request, 3, height);
        scalar(&mut request, 4, 1);
        let result = self.historical_query(&json!({
            "path":"/paxprotocol.paxchain.layerxprograms.Query/ProgramState",
            "data":layerx_programs::hex::encode(&request),
            "height":height.to_string(), "prove":true
        }))?;
        let response = &result["response"];
        crate::reject_if(
            response["code"].as_u64().unwrap_or(0) != 0
                || response["height"]
                    .as_str()
                    .and_then(|n| n.parse::<u64>().ok())
                    != Some(height),
            || CheckpointReadError::InvalidProof,
        )?;
        let bytes = decode_base64(
            response["value"]
                .as_str()
                .ok_or(CheckpointReadError::InvalidProof)?,
        )?;
        let mut identity = None;
        let mut served_key = None;
        let mut record = None;
        let mut served_height = None;
        let mut root = None;
        let mut ops = None;
        let mut previous = 0;
        for (number, value) in fields(&bytes)? {
            crate::reject_if(number <= previous, || CheckpointReadError::InvalidProof)?;
            previous = number;
            match (number, value) {
                (1, ProtoValue::Bytes(v)) => identity = Some(v),
                (2, ProtoValue::Bytes(v)) => served_key = Some(v),

                (4, ProtoValue::Bytes(v)) => record = Some(v),
                (5, ProtoValue::Integer(v)) => served_height = Some(v),
                (6, ProtoValue::Bytes(v)) => root = Some(v),
                (3 | 7, ProtoValue::Bytes(_)) => {}
                (8, ProtoValue::Bytes(v)) => ops = Some(v),
                _ => return Err(CheckpointReadError::InvalidProof),
            }
        }
        crate::reject_if(
            identity != Some(program.as_slice())
                || served_key != Some(key)
                || served_height != Some(height)
                || root != Some(checkpoint.app_hash.as_slice()),
            || CheckpointReadError::TargetMismatch,
        )?;
        let record = record
            .filter(|v| !v.is_empty())
            .ok_or(CheckpointReadError::InvalidProof)?;
        Self::verify_program_state_ops(
            record,
            ops.ok_or(CheckpointReadError::InvalidProof)?,
            key,
            store_name,
            checkpoint,
        )?;
        Ok(record.to_vec())
    }
    fn verify_program_state_ops(
        record: &[u8],
        ops: &[u8],
        key: &[u8],
        store_name: &str,
        checkpoint: &super::ServedCheckpoint,
    ) -> Result<(), CheckpointReadError> {
        use super::abci_result::{fields, ProtoValue};
        use layerx_programs::chain_proof::ics23::{verify_existence, ExistenceProof, ProofSpec};
        use layerx_programs::chain_proof::multistore::verify_store_root;
        let mut decoded = Vec::new();
        for (number, value) in fields(ops)? {
            let (1, ProtoValue::Bytes(op)) = (number, value) else {
                return Err(CheckpointReadError::InvalidProof);
            };
            let op = fields(op)?;
            let [(1, ProtoValue::Bytes(kind)), (2, ProtoValue::Bytes(key)), (3, ProtoValue::Bytes(data))] =
                op.as_slice()
            else {
                return Err(CheckpointReadError::InvalidProof);
            };
            decoded.push((kind.to_vec(), key.to_vec(), data.to_vec()));
        }
        crate::reject_if(
            decoded.len() != 2
                || decoded[0].0 != b"ics23:iavl"
                || decoded[0].1 != key
                || decoded[1].0 != b"ics23:simple"
                || decoded[1].1 != store_name.as_bytes(),
            || CheckpointReadError::InvalidProof,
        )?;
        let existence = |data: &[u8]| -> Result<ExistenceProof, CheckpointReadError> {
            let parsed = fields(data)?;
            let [(1, ProtoValue::Bytes(proof))] = parsed.as_slice() else {
                return Err(CheckpointReadError::InvalidProof);
            };
            ExistenceProof::decode(proof).map_err(|_| CheckpointReadError::InvalidProof)
        };
        let store = existence(&decoded[1].2)?;
        let store_root: [u8; 32] = store
            .value
            .as_slice()
            .try_into()
            .map_err(|_| CheckpointReadError::InvalidProof)?;
        let proof = existence(&decoded[0].2)?;
        verify_existence(&proof, &ProofSpec::iavl(), &store_root, key, record)
            .map_err(|_| CheckpointReadError::RootMismatch)?;
        verify_store_root(store_name, store_root, &decoded[1].2, checkpoint.app_hash)
            .map_err(|_| CheckpointReadError::RootMismatch)?;
        Ok(())
    }
}

impl NativeResultSource {
    /// Adds the real host-store witness required by the Programs head verifier.
    /// Key 0x02 is lxphost's persisted lastSequenceKey; its absence, when proven,
    /// is meaningful too. No empty witness is substituted for unavailable state.
    pub(crate) fn host_commitment(
        &self,
        checkpoint: &super::ServedCheckpoint,
    ) -> Result<layerx_programs::chain_proof::decode::CommitmentProof, CheckpointReadError> {
        use layerx_programs::chain_proof::{
            decode::ProofOp,
            state::{verify_absent, verify_state},
            ChainProofVerifier,
        };
        let mut commitment = checkpoint.commitment_proof()?;
        let height = checkpoint
            .height
            .checked_sub(1)
            .filter(|height| *height > 1)
            .ok_or(CheckpointReadError::TargetMismatch)?;
        let key = vec![0x02];
        let result = self.historical_query(&json!({
            "path":"/store/lxphost/key", "data":"02", "height":height.to_string(), "prove":true
        }))?;
        let response = &result["response"];
        let code = response
            .get("code")
            .map(|value| value.as_u64().ok_or(CheckpointReadError::InvalidProof))
            .transpose()?
            .unwrap_or(0);
        if code != 0
            || response["height"]
                .as_str()
                .and_then(|text| text.parse::<u64>().ok())
                != Some(height)
            || decode_base64(
                response["key"]
                    .as_str()
                    .ok_or(CheckpointReadError::InvalidProof)?,
            )? != key
        {
            eprintln!(
                "agent host commitment store query code={code}: {}",
                response["log"]
            );
            return Err(CheckpointReadError::TargetMismatch);
        }
        let value = match &response["value"] {
            Value::Null => None,
            Value::String(text) if text.is_empty() => None,
            Value::String(text) => Some(decode_base64(text)?),
            _ => return Err(CheckpointReadError::InvalidProof),
        };
        let ops = response["proofOps"]["ops"]
            .as_array()
            .filter(|ops| ops.len() == 2)
            .ok_or(CheckpointReadError::InvalidProof)?;
        let ops = ops
            .iter()
            .map(|op| {
                Ok(ProofOp {
                    op_type: op["type"]
                        .as_str()
                        .ok_or(CheckpointReadError::InvalidProof)?
                        .to_owned(),
                    key: decode_base64(
                        op["key"]
                            .as_str()
                            .ok_or(CheckpointReadError::InvalidProof)?,
                    )?,
                    data: decode_base64(
                        op["data"]
                            .as_str()
                            .ok_or(CheckpointReadError::InvalidProof)?,
                    )?,
                })
            })
            .collect::<Result<Vec<_>, CheckpointReadError>>()?;
        let (header, commit, validators) =
            layerx_programs::chain_evidence::commitment_trust_step(&commitment)
                .map_err(|_| CheckpointReadError::InvalidProof)?;
        let verified = ChainProofVerifier {
            chain_id: header.chain_id.clone(),
            validators,
        }
        .verify_head(&header, &commit)
        .map_err(|_| CheckpointReadError::InvalidProof)?;
        match &value {
            Some(value) => verify_state(&verified, "lxphost", &key, value, &ops),
            None => verify_absent(&verified, "lxphost", &key, &ops),
        }
        .map_err(|_| CheckpointReadError::RootMismatch)?;
        commitment.key = key;
        commitment.value = value;
        commitment.proof_ops = ops;
        Ok(commitment)
    }
}
