use super::{decode64, decode_commit, decode_transaction_proof, Error, Result};
use layerx_client::ai_market::OperationRecord;
use layerx_intents::canonical::ProtocolReceipt;
use layerx_programs::chain_proof::{
    decode::decode_commitment_proof,
    header::ChainHeader,
    results::{verify_tx_result, TxResult},
    state::verify_state,
    ChainProofVerifier, VerifiedChainHead,
};
use layerx_proof::{inclusion::SequencerAuthorization, receipt::verify_sequencer_signature};
use serde::{Deserialize, Serialize};
use sha2::{Digest as _, Sha256};

#[derive(Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct RecoveryTransactionEvidence {
    pub containing_transaction: String,
    pub transaction_proof: String,
    pub execution_header: String,
    pub execution_commit: String,
    pub activity_ids: Vec<[u8; 32]>,
    pub receipt_index: u32,
    pub raw_transaction: String,
    pub receipt_keys_proof: String,
    pub result_code: u32,
    pub result_data: String,
    pub gas_wanted: i64,
    pub gas_used: i64,
    pub result_proof: String,
    pub recovery_did: [u8; 32],
    pub recovery_root: [u8; 32],
    pub recovery_threshold: u16,
}

impl RecoveryTransactionEvidence {
    pub(super) fn verify(
        &self,
        verifier: &ChainProofVerifier,
        head: &VerifiedChainHead,
        header: &ChainHeader,
        receipt_bytes: &[u8],
        receipt: &ProtocolReceipt,
    ) -> Result<()> {
        let execution_bytes = decode64(&self.execution_header)?;
        let execution = ChainHeader::decode(&execution_bytes).map_err(|_| Error::Refused)?;
        let execution_commit =
            decode_commit(&decode64(&self.execution_commit)?).map_err(|_| Error::Refused)?;
        let execution_head = verifier
            .verify_head(&execution, &execution_commit)
            .map_err(|_| Error::Refused)?;
        if execution.height.checked_add(1) != Some(head.height())
            || execution.height <= 0
            || header.last_block_id.hash != execution_head.header_hash()
            || execution.next_validators_hash != header.validators_hash
        {
            return Err(Error::Refused);
        }
        let transaction = decode64(&self.containing_transaction)?;
        let proof = decode_transaction_proof(&decode64(&self.transaction_proof)?)
            .map_err(|_| Error::Refused)?;
        if proof.index != 0 || proof.total.checked_sub(1) != Some(self.activity_ids.len() as i64) {
            return Err(Error::Refused);
        }
        layerx_programs::verify_tx(&execution_head, &transaction, &proof)
            .map_err(|_| Error::Refused)?;
        let batch = transaction
            .strip_prefix(b"PAXEER/v1/sequence-batch\0")
            .filter(|body| body.len() == 216)
            .ok_or(Error::Refused)?;
        let height = integer(&batch[..8])?;
        let first = integer(&batch[8..16])?;
        let last = integer(&batch[16..24])?;
        let selected = usize::try_from(self.receipt_index).map_err(|_| Error::Refused)?;
        if height != execution.height as u64
            || first == 0
            || last.checked_sub(first).and_then(|n| n.checked_add(1))
                != Some(self.activity_ids.len() as u64)
            || self.activity_ids.get(selected) != Some(&receipt.activity_id())
            || first.checked_add(u64::from(self.receipt_index)) != Some(receipt.global_sequence())
            || receipt.batch_id()[..24] != [0; 24]
            || integer(&receipt.batch_id()[24..])? != height
        {
            return Err(Error::Refused);
        }
        let mut previous: [u8; 32] = batch[24..56].try_into().map_err(|_| Error::Refused)?;
        let mut root = Sha256::new();
        for (index, activity) in self.activity_ids.iter().enumerate() {
            root.update(activity);
            let mut hash = Sha256::new();
            hash.update(b"PAXEER/v1/sequence\0");
            hash.update(previous);
            hash.update((first + index as u64).to_be_bytes());
            hash.update(activity);
            let resulting: [u8; 32] = hash.finalize().into();
            if index == selected
                && (receipt.previous_state_root() != previous
                    || receipt.resulting_state_root() != resulting)
            {
                return Err(Error::Refused);
            }
            previous = resulting;
        }
        if previous != batch[56..88]
            || root.finalize().as_slice() != &batch[88..120]
            || receipt.context_hash() != batch[120..152]
        {
            return Err(Error::Refused);
        }
        let key_proof = decode_commitment_proof(&decode64(&self.receipt_keys_proof)?)
            .map_err(|_| Error::Refused)?;
        if key_proof.store_name != "sequencer" || key_proof.key != [3] {
            return Err(Error::Refused);
        }
        // The key proof belongs to the same authenticated successor, never to a caller-selected root.
        let key_header =
            ChainHeader::decode(&key_proof.signed_header).map_err(|_| Error::Refused)?;
        if key_header != *header {
            return Err(Error::Refused);
        }
        let keys = key_proof.value.as_deref().ok_or(Error::Refused)?;
        verify_state(head, "sequencer", &[3], keys, &key_proof.proof_ops)
            .map_err(|_| Error::Refused)?;
        if keys.is_empty() || keys.len() % 80 != 0 {
            return Err(Error::Refused);
        }
        let mut pins = Vec::new();
        for entry in keys.chunks_exact(80) {
            let key_id: [u8; 32] = entry[..32].try_into().map_err(|_| Error::Refused)?;
            let public_key = entry[32..64].try_into().map_err(|_| Error::Refused)?;
            let from = integer(&entry[64..72])?;
            let to = integer(&entry[72..80])?;
            if key_id == [0; 32] || from == 0 || from > to {
                return Err(Error::Refused);
            }
            pins.push(SequencerAuthorization::new(key_id, public_key, from, to));
        }
        let signed = verify_sequencer_signature(receipt_bytes, pins.as_slice())
            .map_err(|_| Error::Refused)?;
        if signed.protocol() != Some(receipt) {
            return Err(Error::Refused);
        }
        let summary =
            layerx_proof::sequencer_logs::decode_summary(&signed).map_err(|_| Error::Refused)?;
        let tx_index = self.receipt_index.checked_add(1).ok_or(Error::Refused)?;
        if summary.tx_index() != tx_index
            || summary.vm_error_hash() != <[u8; 32]>::from(Sha256::digest([]))
        {
            return Err(Error::Refused);
        }
        let raw = decode64(&self.raw_transaction)?;
        let identity = OperationRecord::from_signed_transaction(raw.clone(), [0; 32], 0)
            .map_err(|_| Error::Refused)?;
        if identity.activity_id() != receipt.activity_id()
            || identity.signer_public_key() != receipt.from()
        {
            return Err(Error::Refused);
        }
        let result_proof =
            decode_transaction_proof(&decode64(&self.result_proof)?).map_err(|_| Error::Refused)?;
        let result = TxResult {
            code: self.result_code,
            data: decode64(&self.result_data)?,
            gas_wanted: self.gas_wanted,
            gas_used: self.gas_used,
        };
        if result.gas_used < 0
            || result.gas_wanted < 0
            || result.gas_used as u64 != summary.gas_used()
            || result_proof.index != i64::from(tx_index)
            || result_proof.total != proof.total
        {
            return Err(Error::Refused);
        }
        verify_tx_result(head, &result, &result_proof).map_err(|_| Error::Refused)?;
        let output = evm_output(&result.data, receipt.activity_id())?;
        if output.len() != 32
            || output[..24] != [0; 24]
            || integer(&output[24..])? == 0
            || summary.return_data_len() != 32
            || receipt.activity_root() != <[u8; 32]>::from(Sha256::digest(output))
        {
            return Err(Error::Refused);
        }
        Ok(())
    }

    pub(super) fn verify_recovery_policy(&self, receipt: &ProtocolReceipt) -> Result<()> {
        let raw = decode64(&self.raw_transaction)?;
        let mut bytes = raw.strip_prefix(&[2]).ok_or(Error::Refused)?;
        let (list, is_list) = rlp(&mut bytes)?;
        if !is_list || !bytes.is_empty() {
            return Err(Error::Refused);
        }
        let mut fields = list;
        let mut values = Vec::new();
        while !fields.is_empty() {
            values.push(rlp(&mut fields)?.0);
        }
        if values.len() != 12 {
            return Err(Error::Refused);
        }
        const AUTH: [u8; 20] = [
            0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1c,
        ];
        if values[5] != AUTH || receipt.to()[..12] != [0; 12] || receipt.to()[12..] != AUTH {
            return Err(Error::Refused);
        }
        let call = values[7];
        if call.len() != 196
            || call[..4] != [0x87, 0xd3, 0xa0, 0x67]
            || call[4..35] != [0; 31]
            || call[35] != 64
            || call[68..99] != [0; 31]
            || call[99] != 70
            || call[100..104] != [0x71, 3, 0, 3]
            || call[104..136] != self.recovery_did
            || call[136..168] != self.recovery_root
            || call[168..170] != self.recovery_threshold.to_be_bytes()
            || call[170..] != [0; 26]
            || self.recovery_did == [0; 32]
            || self.recovery_root == [0; 32]
            || self.recovery_threshold == 0
        {
            return Err(Error::Refused);
        }
        Ok(())
    }
}

fn integer(bytes: &[u8]) -> Result<u64> {
    Ok(u64::from_be_bytes(
        bytes.try_into().map_err(|_| Error::Refused)?,
    ))
}

fn rlp<'a>(input: &mut &'a [u8]) -> Result<(&'a [u8], bool)> {
    let first = *input.first().ok_or(Error::Refused)?;
    let (prefix, length, list) = match first {
        0..=0x7f => (0, 1, false),
        0x80..=0xb7 => (1, usize::from(first - 0x80), false),
        0xc0..=0xf7 => (1, usize::from(first - 0xc0), true),
        _ => {
            let list = first >= 0xf8;
            let size = usize::from(first - if list { 0xf7 } else { 0xb7 });
            let bytes = input.get(1..1 + size).ok_or(Error::Refused)?;
            let mut length = 0usize;
            for byte in bytes {
                length = length
                    .checked_mul(256)
                    .and_then(|n| n.checked_add(usize::from(*byte)))
                    .ok_or(Error::Refused)?;
            }
            (1 + size, length, list)
        }
    };
    let end = prefix.checked_add(length).ok_or(Error::Refused)?;
    let value = input.get(prefix..end).ok_or(Error::Refused)?;
    *input = input.get(end..).ok_or(Error::Refused)?;
    Ok((value, list))
}

fn evm_output(data: &[u8], activity: [u8; 32]) -> Result<Vec<u8>> {
    use layerx_programs::chain_proof::proto::{ProtoReader, ProtoValue};
    let mut tx = ProtoReader::new(data);
    let message = match tx.next_field().map_err(|_| Error::Refused)? {
        Some(field) if field.number == 1 => match field.value {
            ProtoValue::Bytes(b) => b,
            _ => return Err(Error::Refused),
        },
        _ => return Err(Error::Refused),
    };
    tx.finish().map_err(|_| Error::Refused)?;
    let mut msg = ProtoReader::new(message);
    let mut body = None;
    let mut kind = None;
    while let Some(field) = msg.next_field().map_err(|_| Error::Refused)? {
        match (field.number, field.value) {
            (1, ProtoValue::Bytes(b)) if kind.is_none() => kind = Some(b),
            (2, ProtoValue::Bytes(b)) if body.is_none() => body = Some(b),
            _ => return Err(Error::Refused),
        }
    }
    msg.finish().map_err(|_| Error::Refused)?;
    if kind != Some(b"/paxprotocol.paxchain.evm.MsgEVMTransaction".as_slice()) {
        return Err(Error::Refused);
    }
    let mut response = ProtoReader::new(body.ok_or(Error::Refused)?);
    let mut output = None;
    let mut hash = None;
    let mut gas = None;
    while let Some(field) = response.next_field().map_err(|_| Error::Refused)? {
        match (field.number, field.value) {
            (1, ProtoValue::Varint(n)) if gas.is_none() => gas = Some(n),
            (2, ProtoValue::Bytes([])) => {}
            (3, ProtoValue::Bytes(b)) if output.is_none() => output = Some(b.to_vec()),
            (4, ProtoValue::Bytes(b)) if hash.is_none() => hash = Some(b),
            (5, ProtoValue::Bytes(_)) => {}
            _ => return Err(Error::Refused),
        }
    }
    response.finish().map_err(|_| Error::Refused)?;
    let expected = format!(
        "0x{}",
        activity
            .iter()
            .map(|b| format!("{b:02x}"))
            .collect::<String>()
    );
    if hash != Some(expected.as_bytes()) {
        return Err(Error::Refused);
    }
    output.ok_or(Error::Refused)
}
