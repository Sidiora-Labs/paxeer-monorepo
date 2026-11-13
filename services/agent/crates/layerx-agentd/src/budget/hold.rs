//! Durable unknown reservations and fail-closed restart accounting.

#[cfg(test)]
use crate::budget::AssertCase as _;

use crate::protocol_evidence::{EvidenceAuthority, RawReceiptEvidence, ReceiptReplayError};
use crate::store::{ObjectKind, Store, StoreError, TenantId, TenantKey};

use super::{ProtocolBudgetRecord, ProtocolBudgetState};

/// Reservation kept unavailable until a receipt resolves it.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct UnknownReservation {
    pub tenant: TenantId,
    pub id: [u8; 32],
    pub amount: u128,
    pub expiry_sequence: u64,
    pub resolved: Option<UnknownOutcome>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum UnknownOutcome {
    Executed,
    Failed,
}

/// Persisted verified receipt used to rebuild consumed accounting.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PersistedReceipt<E = RawReceiptEvidence> {
    pub expected_activity_id: [u8; 32],
    pub requested_amount: u128,
    pub evidence: E,
}

/// The two independently verified durable receipt evidence formats.
#[derive(Clone, Debug, Eq, PartialEq)]
#[allow(
    clippy::large_enum_variant,
    reason = "Preserve the public receipt evidence variants accepted by budget recovery."
)]
pub enum BudgetReceiptEvidence {
    Legacy(RawReceiptEvidence),
    Current(crate::receipt::TransactionReceiptEvidence),
}

impl From<RawReceiptEvidence> for BudgetReceiptEvidence {
    fn from(evidence: RawReceiptEvidence) -> Self {
        Self::Legacy(evidence)
    }
}

impl From<crate::receipt::TransactionReceiptEvidence> for BudgetReceiptEvidence {
    fn from(evidence: crate::receipt::TransactionReceiptEvidence) -> Self {
        Self::Current(evidence)
    }
}

impl BudgetReceiptEvidence {
    pub(crate) fn verify(
        &self,
        verifier: &EvidenceAuthority,
    ) -> Result<
        crate::protocol_evidence::VerifiedSpendReceipt,
        crate::protocol_evidence::ReceiptEvidenceError,
    > {
        match self {
            Self::Legacy(raw) => verifier.verify_receipt(raw).map(Into::into),
            Self::Current(raw) => verifier.verify_current_receipt(raw),
        }
    }
}

/// Operator-visible restart accounting.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct RestartAccounting {
    pub protocol_consumed: Option<u128>,
    pub receipt_consumed: u128,
    pub held_unresolved: u128,
    pub unresolved_count: usize,
    pub reconciled: bool,
}

impl RestartAccounting {
    /// Refuses write admission until protocol and receipt accounting reconcile.
    ///
    /// # Errors
    ///
    /// Returns `ProtocolStateSchemaUnavailable` until core produces a canonical
    /// budget record and proof, or `Unreconciled` when a future canonical state
    /// disagrees with rebuilt receipt consumption.
    pub fn require_write_ready(self) -> Result<(), RestartError> {
        if self.protocol_consumed.is_none() {
            Err(RestartError::ProtocolStateSchemaUnavailable)
        } else if self.reconciled {
            Ok(())
        } else {
            Err(RestartError::Unreconciled)
        }
    }
}

#[derive(Debug)]
pub enum RestartError {
    Store(StoreError),
    Corrupt,
    UnverifiedProtocol,
    ProtocolStateSchemaUnavailable,
    UnverifiedReceipt,
    DuplicateReceipt,
    DuplicateActivity,
    ReceiptActivityMismatch,
    Unreconciled,
    Arithmetic,
}

impl From<StoreError> for RestartError {
    fn from(value: StoreError) -> Self {
        Self::Store(value)
    }
}

pub(crate) fn persist_unknown(
    store: &mut Store,
    reservation: &UnknownReservation,
) -> Result<(), RestartError> {
    let key = key(reservation.tenant.clone(), reservation.id)?;
    store.put_local(key, encode(reservation))?;
    Ok(())
}

pub(crate) fn rebuild_accounting<E: Clone + Into<BudgetReceiptEvidence>>(
    store: &Store,
    tenant: &TenantId,
    unknown_ids: &[[u8; 32]],
    receipts: &[PersistedReceipt<E>],
    protocol: &ProtocolBudgetState,
    verifier: &EvidenceAuthority,
) -> Result<RestartAccounting, RestartError> {
    let mut replay = EvidenceAuthority::receipt_replay_guard();
    let mut receipt_consumed = 0_u128;
    for receipt in receipts {
        let evidence: BudgetReceiptEvidence = receipt.evidence.clone().into();
        let verified_receipt = evidence
            .verify(verifier)
            .map_err(|_| RestartError::UnverifiedReceipt)?;
        if verified_receipt.activity_id() != receipt.expected_activity_id {
            return Err(RestartError::ReceiptActivityMismatch);
        }
        replay
            .admit_identity(
                verified_receipt.receipt_ref(),
                verified_receipt.activity_id(),
            )
            .map_err(map_replay_error)?;
        if verified_receipt.result_code() == 0 {
            let consumed = match evidence {
                BudgetReceiptEvidence::Legacy(_) => verified_receipt.amount(),
                BudgetReceiptEvidence::Current(_) => receipt.requested_amount,
            };
            receipt_consumed = receipt_consumed
                .checked_add(consumed)
                .ok_or(RestartError::Arithmetic)?;
        }
    }
    let verified_state = verifier
        .verify_state(&protocol.evidence)
        .map_err(|_| RestartError::UnverifiedProtocol)?;
    let protocol_consumed = ProtocolBudgetRecord::decode(verified_state.canonical_state())
        .ok()
        .map(|record| record.spent_this_period);
    let mut held_unresolved = 0_u128;
    let mut unresolved_count = 0_usize;
    for id in unknown_ids {
        let storage_key = key(tenant.clone(), *id)?;
        let Some(value) = store.get(&storage_key) else {
            return Err(RestartError::Corrupt);
        };
        let reservation = decode(tenant, *id, value.bytes())?;
        if reservation.resolved.is_none() {
            held_unresolved = held_unresolved
                .checked_add(reservation.amount)
                .ok_or(RestartError::Arithmetic)?;
            unresolved_count += 1;
        }
    }
    let reconciled = protocol_consumed.is_some_and(|consumed| {
        receipt_consumed
            .checked_add(held_unresolved)
            .is_some_and(|ceiling| consumed >= receipt_consumed && consumed <= ceiling)
    });
    Ok(RestartAccounting {
        protocol_consumed,
        receipt_consumed,
        held_unresolved,
        unresolved_count,
        reconciled,
    })
}

pub(crate) fn requested_amount(
    store: &Store,
    tenant: &TenantId,
    activity_id: [u8; 32],
) -> Result<u128, RestartError> {
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
    if let Some(value) = store.get(&key(tenant.clone(), activity_id)?) {
        return decode(tenant, activity_id, value.bytes()).map(|reservation| reservation.amount);
    }
    let signed_key = TenantKey::new(
        tenant.clone(),
        ObjectKind::PreparedActivity,
        activity_id.to_vec(),
    )?;
    let Some(signed) = store.get(&signed_key) else {
        return Ok(0);
    };
    if transaction_hash(signed.bytes()) != activity_id {
        return Err(RestartError::ReceiptActivityMismatch);
    }
    let (call, signature) = decode_signed(signed.bytes()).map_err(|_| RestartError::Corrupt)?;
    recover_sender(&call, &signature).map_err(|_| RestartError::Corrupt)?;
    let mut pay = [0; 20];
    pay[18..].copy_from_slice(&[0x10, 0x1b]);
    let selector = transaction_hash(b"budgetSpend(bytes32,bytes32,uint128)");
    if call.to != pay || !call.data.starts_with(&selector[..4]) {
        return Ok(0);
    }
    if call.data.len() != 100 || call.data[68..84] != [0; 16] {
        return Err(RestartError::Corrupt);
    }
    let amount = call.data[84..100]
        .try_into()
        .map_err(|_| RestartError::Corrupt)?;
    Ok(u128::from_be_bytes(amount))
}

const fn map_replay_error(error: ReceiptReplayError) -> RestartError {
    match error {
        ReceiptReplayError::DuplicateReceipt => RestartError::DuplicateReceipt,
        ReceiptReplayError::DuplicateActivity => RestartError::DuplicateActivity,
    }
}

fn key(tenant: TenantId, id: [u8; 32]) -> Result<TenantKey, RestartError> {
    let mut object_id = b"unknown-budget:".to_vec();
    object_id.extend_from_slice(&id);
    Ok(TenantKey::new(tenant, ObjectKind::Budget, object_id)?)
}

fn encode(reservation: &UnknownReservation) -> Vec<u8> {
    let mut bytes = Vec::new();
    bytes.extend_from_slice(&reservation.amount.to_be_bytes());
    bytes.extend_from_slice(&reservation.expiry_sequence.to_be_bytes());
    bytes.push(match reservation.resolved {
        None => 0,
        Some(UnknownOutcome::Executed) => 1,
        Some(UnknownOutcome::Failed) => 2,
    });
    bytes
}

fn decode(
    tenant: &TenantId,
    id: [u8; 32],
    bytes: &[u8],
) -> Result<UnknownReservation, RestartError> {
    if bytes.len() != 25 {
        return Err(RestartError::Corrupt);
    }
    let mut amount = [0_u8; 16];
    amount.copy_from_slice(&bytes[..16]);
    let mut expiry = [0_u8; 8];
    expiry.copy_from_slice(&bytes[16..24]);
    let resolved = match bytes[24] {
        0 => None,
        1 => Some(UnknownOutcome::Executed),
        2 => Some(UnknownOutcome::Failed),
        _ => return Err(RestartError::Corrupt),
    };
    Ok(UnknownReservation {
        tenant: tenant.clone(),
        id,
        amount: u128::from_be_bytes(amount),
        expiry_sequence: u64::from_be_bytes(expiry),
        resolved,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::protocol_evidence::RawStateEvidence;
    use crate::receipt::{
        TransactionReceiptEvidence, TransactionReceiptMetadata, NODE_ED25519_SIGNATURE_TAG,
    };
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
    use layerx_proof::merkle::build_proof;
    use layerx_wire::encode::Encoder;

    fn directory(name: &str) -> std::path::PathBuf {
        let suffix = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .assert_case("system time")
            .as_nanos();
        std::env::temp_dir().join(format!(
            "layerx-requested-spend-{name}-{}-{suffix}",
            std::process::id()
        ))
    }

    fn protocol_with_requested_spend(spent: u128) -> (ProtocolBudgetState, EvidenceAuthority) {
        let mut state = vec![0; 278];
        state[1] = 1;
        state[2..34].fill(0x51);
        state[162..178].copy_from_slice(&100_u128.to_be_bytes());
        state[178..194].copy_from_slice(&100_u128.to_be_bytes());
        state[210..226].copy_from_slice(&spent.to_be_bytes());
        state[242..250].copy_from_slice(&2_000_u64.to_be_bytes());
        let (proof, root) = build_proof(&[state.as_slice()], 0).assert_case("state proof");
        let key = SigningKey::from_bytes(&[0x3a; 32]);
        let public = key.verifying_key().to_bytes();
        let mut header = Encoder::new(512);
        header
            .structure_header_version(0x1701, layerx_wire::limits::PROTOCOL_VERSION)
            .assert_case("header");
        header.u8(15).assert_case("field count");
        for (tag, value) in [
            (
                1,
                layerx_wire::limits::PROTOCOL_VERSION.to_be_bytes().to_vec(),
            ),
            (2, 42_u32.to_be_bytes().to_vec()),
            (3, 2_u64.to_be_bytes().to_vec()),
            (4, 7_u64.to_be_bytes().to_vec()),
            (5, 1_u64.to_be_bytes().to_vec()),
            (6, 1_000_u64.to_be_bytes().to_vec()),
            (7, [0x31; 32].to_vec()),
            (8, root.to_vec()),
            (9, [0x32; 32].to_vec()),
            (10, [0x33; 32].to_vec()),
            (11, [0x34; 32].to_vec()),
            (12, [0x35; 32].to_vec()),
            (13, [0x36; 32].to_vec()),
            (14, 1_000_u64.to_be_bytes().to_vec()),
            (15, public.to_vec()),
        ] {
            header.tag(tag, 15).assert_case("tag");
            match tag {
                1 => header.u16(u16::from_be_bytes(value.try_into().assert_case("version"))),
                2 => header.u32(u32::from_be_bytes(value.try_into().assert_case("network"))),
                3..=6 | 14 => {
                    header.u64(u64::from_be_bytes(value.try_into().assert_case("integer")))
                }
                _ => header.bytes(&value, 32),
            }
            .assert_case("header field");
        }
        let header = header.finish();
        let digest = layerx_wire::hash::batch_header_digest(&header).assert_case("header digest");
        let signature = key.sign(&digest).to_bytes();
        let authority = EvidenceAuthority::pinned_to_handshake(
            layerx_wire::limits::PROTOCOL_VERSION,
            42,
            public,
        )
        .assert_case("authority");
        (
            ProtocolBudgetState {
                evidence: RawStateEvidence::new(state, proof, root, header, signature),
            },
            authority,
        )
    }

    #[test]
    fn rebuild_accounting_counts_the_requested_spend_of_a_zero_value_receipt() {
        // This signed zero-value receipt is emitted by the node conformance producer.
        let capture: serde_json::Value = serde_json::from_str(include_str!(
            "../../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))
        .assert_case("node producer capture");
        let case = capture["cases"]
            .as_array()
            .assert_case("cases")
            .iter()
            .find(|case| case["id"] == "V1-erc20-approve")
            .assert_case("zero-value receipt");
        let bytes = |value: &serde_json::Value| {
            hex::decode(value.as_str().assert_case("hex").trim_start_matches("0x"))
                .assert_case("decoded hex")
        };
        let canonical_receipt = bytes(&case["signedReceipt"]);
        let decoded = layerx_wire::receipt::decode(&canonical_receipt).assert_case("node receipt");
        let protocol_receipt = decoded.protocol().assert_case("protocol receipt");
        assert_eq!(protocol_receipt.amount(), 0);
        let activity_id = transaction_hash(&bytes(&case["rawTransaction"]));
        assert_eq!(protocol_receipt.activity_id(), activity_id);
        let mut receipt_signature = [0; 65];
        receipt_signature[..64].copy_from_slice(
            &protocol_receipt
                .sequencer_signature()
                .assert_case("signature"),
        );
        receipt_signature[64] = NODE_ED25519_SIGNATURE_TAG;
        let evidence = TransactionReceiptEvidence {
            metadata: TransactionReceiptMetadata {
                transaction_hash: activity_id,
                block_height: 1_000,
                tx_index: 2,
                status: true,
                gas_used: 46_220,
                logs_bloom: [0; 256],
                receipt_signature,
                rung: layerx_types::rung::Rung::Receipted,
            },
            canonical_receipt,
        };
        let (protocol, verifier) = protocol_with_requested_spend(25);
        let key = &case["pinnedKey"];
        let verifier = verifier
            .with_receipt_keys(vec![layerx_client::sequencer::ReceiptKeyEntry {
                key_id: bytes(&key["keyId"]).try_into().assert_case("key id"),
                public_key: bytes(&key["publicKey"])
                    .try_into()
                    .assert_case("public key"),
                first_height: key["firstHeight"].as_u64().assert_case("first height"),
                last_height: key["lastHeight"].as_u64().assert_case("last height"),
            }])
            .assert_case("receipt keys");
        assert_eq!(
            verifier
                .verify_current_receipt(&evidence)
                .assert_case("verified current receipt")
                .amount(),
            0
        );
        let tenant = TenantId::new("requested-spend").assert_case("tenant");
        let root = directory("zero-value");
        let mut store = Store::open(&root).assert_case("store");
        persist_unknown(
            &mut store,
            &UnknownReservation {
                tenant: tenant.clone(),
                id: activity_id,
                amount: 25,
                expiry_sequence: 2_000,
                resolved: Some(UnknownOutcome::Executed),
            },
        )
        .assert_case("persist requested spend");
        drop(store);
        let store = Store::open(&root).assert_case("reopened store");
        let receipts = [PersistedReceipt {
            expected_activity_id: activity_id,
            requested_amount: requested_amount(&store, &tenant, activity_id)
                .assert_case("retained requested amount"),
            evidence,
        }];
        let accounting = rebuild_accounting(&store, &tenant, &[], &receipts, &protocol, &verifier)
            .assert_case("rebuild");
        assert_eq!(accounting.protocol_consumed, Some(25));
        assert_eq!(accounting.receipt_consumed, 25);
        assert!(accounting.reconciled);
        accounting.require_write_ready().assert_case("write ready");
        drop(store);
        std::fs::remove_dir_all(root).assert_case("cleanup");
    }

    fn assert_invalid_spend_calls(
        mut call: Eip1559Call,
        store: &mut Store,
        tenant: &TenantId,
        pay: [u8; 20],
    ) {
        call.to = [3; 20];
        let signed = call.sign(&[9; 32]).assert_case("other target");
        let id = transaction_hash(&signed);
        store
            .put_local(
                TenantKey::new((*tenant).clone(), ObjectKind::PreparedActivity, id.to_vec())
                    .assert_case("key"),
                signed,
            )
            .assert_case("persist other call");
        assert_eq!(
            requested_amount(store, tenant, id).assert_case("other target amount"),
            0
        );
        call.to = pay;
        call.data[68] = 1;
        let signed = call.sign(&[9; 32]).assert_case("noncanonical uint128");
        let id = transaction_hash(&signed);
        store
            .put_local(
                TenantKey::new((*tenant).clone(), ObjectKind::PreparedActivity, id.to_vec())
                    .assert_case("key"),
                signed.clone(),
            )
            .assert_case("persist invalid calldata");
        assert!(matches!(
            requested_amount(store, tenant, id),
            Err(RestartError::Corrupt)
        ));
        store
            .put_local(
                TenantKey::new(
                    (*tenant).clone(),
                    ObjectKind::PreparedActivity,
                    [4; 32].to_vec(),
                )
                .assert_case("wrong hash key"),
                signed,
            )
            .assert_case("persist wrong hash");
        assert!(matches!(
            requested_amount(store, tenant, [4; 32]),
            Err(RestartError::ReceiptActivityMismatch)
        ));
    }

    #[test]
    fn requested_spend_recovers_signed_calldata_and_prefers_the_persisted_reservation() {
        let tenant = TenantId::new("requested-spend").assert_case("tenant");
        let root = directory("calldata");
        let mut store = Store::open(&root).assert_case("store");
        let mut pay = [0; 20];
        pay[18..].copy_from_slice(&[0x10, 0x1b]);
        let mut data = transaction_hash(b"budgetSpend(bytes32,bytes32,uint128)")[..4].to_vec();
        data.extend([1; 32]);
        data.extend([2; 32]);
        data.extend([0; 16]);
        data.extend(25_u128.to_be_bytes());
        let call = Eip1559Call {
            chain_id: 125,
            nonce: 1,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 2,
            gas_limit: 100_000,
            to: pay,
            data,
        };
        let signed = call.sign(&[9; 32]).assert_case("signed budget spend");
        let activity_id = transaction_hash(&signed);
        let signed_key = TenantKey::new(
            tenant.clone(),
            ObjectKind::PreparedActivity,
            activity_id.to_vec(),
        )
        .assert_case("signed key");
        store
            .put_local(signed_key, signed)
            .assert_case("persist signed transaction");
        drop(store);
        let mut store = Store::open(&root).assert_case("reopen");
        assert_eq!(
            requested_amount(&store, &tenant, activity_id).assert_case("calldata amount"),
            25
        );
        persist_unknown(
            &mut store,
            &UnknownReservation {
                tenant: tenant.clone(),
                id: activity_id,
                amount: 40,
                expiry_sequence: 100,
                resolved: Some(UnknownOutcome::Executed),
            },
        )
        .assert_case("reservation");
        assert_eq!(
            requested_amount(&store, &tenant, activity_id).assert_case("reservation first"),
            40
        );
        assert_eq!(
            requested_amount(&store, &tenant, [0; 32]).assert_case("no submitted spend"),
            0
        );
        assert_invalid_spend_calls(call, &mut store, &tenant, pay);
        drop(store);
        std::fs::remove_dir_all(root).assert_case("cleanup");
    }
}
