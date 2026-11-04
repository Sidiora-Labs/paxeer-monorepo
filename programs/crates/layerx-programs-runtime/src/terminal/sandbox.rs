//! Bounded decoding and receipt authentication of native sandbox settlements.

use super::{Cursor, TerminalDecodeError};
use crate::{FeeSchedule, FeeScheduleParameters, MeteredUsage};

pub const SANDBOX_SETTLEMENT_DOMAIN: &[u8] = b"LXP/programs/sandbox-settlement/v1\0";
const USAGE_DOMAIN: &[u8] = b"LayerX/programs/sandbox/usage-receipt/v4\0";
pub const MAX_SANDBOX_USAGE_RECEIPT_BYTES: usize = 4096;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SandboxSettlement {
    pub receipt: Vec<u8>,
    pub receipt_digest: [u8; 32],
    pub transfer_root: [u8; 32],
    pub lease: [u8; 32],
    pub sequence: u64,
    pub observed_batch: u64,
    pub activity_id: [u8; 32],
    pub program: [u8; 32],
    pub kind: u8,
    pub usage: MeteredUsage,
    pub fees: FeeSchedule,
    pub charged: u128,
    pub lease_terms_digest: [u8; 32],
    pub expected_lease_digest: [u8; 32],
    pub resulting_lease_digest: [u8; 32],
    pub fee_destination: [u8; 32],
    pub previous_receipt: [u8; 32],
    pub previous_accumulator: [u8; 32],
    pub cumulative_usage: [u64; 8],
    pub cumulative_spent: u128,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct SandboxReceiptBinding {
    pub activity_id: [u8; 32],
    pub program: [u8; 32],
    pub kind: u8,
    pub fee_schedule_version: u32,
    pub usage: MeteredUsage,
    pub usage_receipt_digest: [u8; 32],
    pub transfer_root: [u8; 32],
}

impl SandboxSettlement {
    /// Encodes the frozen native usage-receipt contract and validates conservation.
    /// The host authenticates lease transition fields and transfer inclusion separately.
    pub fn canonical_usage_receipt(&self) -> Result<Vec<u8>, TerminalDecodeError> {
        let mut bytes = Vec::new();
        self.append_usage_receipt(&mut bytes);
        Self::read(&bytes, self.transfer_root, false)?;
        Ok(bytes)
    }

    fn append_usage_receipt(&self, bytes: &mut Vec<u8>) {
        bytes.extend_from_slice(USAGE_DOMAIN);
        bytes.extend_from_slice(&self.lease);
        bytes.extend_from_slice(&self.sequence.to_be_bytes());
        bytes.extend_from_slice(&self.observed_batch.to_be_bytes());
        for field in [
            self.activity_id,
            self.lease_terms_digest,
            self.expected_lease_digest,
            self.resulting_lease_digest,
            self.fee_destination,
            self.previous_receipt,
            self.previous_accumulator,
        ] {
            bytes.extend_from_slice(&field);
        }
        bytes.push(self.kind);
        bytes.extend_from_slice(&self.program);
        for field in [
            self.usage.cpu_fuel,
            self.usage.memory_bytes,
            self.usage.storage_read_bytes,
            self.usage.storage_write_bytes,
            u64::from(self.usage.output_values),
            self.usage.output_bytes,
        ] {
            bytes.extend_from_slice(&field.to_be_bytes());
        }
        for field in [
            self.usage.occupancy_byte_batches,
            self.usage.occupancy_fee_units,
            self.usage.fee_units,
        ] {
            bytes.extend_from_slice(&field.to_be_bytes());
        }
        for field in self.cumulative_usage {
            bytes.extend_from_slice(&field.to_be_bytes());
        }
        bytes.extend_from_slice(&self.fees.version().to_be_bytes());
        for price in [
            self.fees.cpu_price(),
            self.fees.memory_byte_price(),
            self.fees.storage_read_byte_price(),
            self.fees.storage_write_byte_price(),
            self.fees.output_value_price(),
            self.fees.output_byte_price(),
            self.fees.occupancy_byte_batch_price(),
        ] {
            bytes.extend_from_slice(&price.to_be_bytes());
        }
        bytes.extend_from_slice(&self.charged.to_be_bytes());
        bytes.extend_from_slice(&self.cumulative_spent.to_be_bytes());
        bytes.extend_from_slice(&self.transfer_root);
    }

    /// Authenticates the attachment against fields of the included native receipt.
    pub fn authenticate(&self, binding: SandboxReceiptBinding) -> Result<(), TerminalDecodeError> {
        if self.activity_id != binding.activity_id
            || self.program != binding.program
            || self.kind != binding.kind
            || self.usage != binding.usage
            || self.fees.version() != binding.fee_schedule_version
            || self.receipt_digest != binding.usage_receipt_digest
            || self.transfer_root != binding.transfer_root
        {
            return Err(TerminalDecodeError::Malformed);
        }
        Ok(())
    }

    pub(super) fn decode(
        receipt: &[u8],
        transfer_root: [u8; 32],
    ) -> Result<Self, TerminalDecodeError> {
        Self::read(receipt, transfer_root, true)
    }

    fn read(
        receipt: &[u8],
        transfer_root: [u8; 32],
        retain: bool,
    ) -> Result<Self, TerminalDecodeError> {
        use TerminalDecodeError::Malformed;
        if receipt.len() > MAX_SANDBOX_USAGE_RECEIPT_BYTES {
            return Err(Malformed);
        }
        let mut c = Cursor::new(receipt);
        if c.take(USAGE_DOMAIN.len())? != USAGE_DOMAIN {
            return Err(Malformed);
        }
        let lease = c.array()?;
        let sequence = c.u64()?;
        let observed_batch = c.u64()?;
        let activity_id = c.array()?;
        let terms = c.array::<32>()?;
        let expected = c.array::<32>()?;
        let resulting = c.array::<32>()?;
        let destination = c.array::<32>()?;
        let previous_receipt = c.array()?;
        let previous_accumulator = c.array()?;
        let kind = c.byte()?;
        let program = c.array()?;
        let usage = MeteredUsage {
            cpu_fuel: c.u64()?,
            memory_bytes: c.u64()?,
            storage_read_bytes: c.u64()?,
            storage_write_bytes: c.u64()?,
            output_values: u32::try_from(c.u64()?).map_err(|_| Malformed)?,
            output_bytes: c.u64()?,
            occupancy_byte_batches: u128::from_be_bytes(c.array()?),
            occupancy_fee_units: u128::from_be_bytes(c.array()?),
            fee_units: u128::from_be_bytes(c.array()?),
        };
        let mut cumulative_usage = [0; 8];
        for field in &mut cumulative_usage {
            *field = c.u64()?;
        }
        let fees = FeeSchedule::new_complete(FeeScheduleParameters {
            version: c.u32()?,
            fee_units_per_cpu_fuel: c.u64()?,
            fee_units_per_memory_byte: c.u64()?,
            fee_units_per_storage_read_byte: c.u64()?,
            fee_units_per_storage_write_byte: c.u64()?,
            fee_units_per_output_value: c.u64()?,
            fee_units_per_output_byte: c.u64()?,
            fee_units_per_occupancy_byte_batch: c.u64()?,
        });
        let charged = u128::from_be_bytes(c.array()?);
        let cumulative_spent = u128::from_be_bytes(c.array()?);
        let recorded_root = c.array::<32>()?;
        c.end()?;
        if [
            lease,
            activity_id,
            terms,
            expected,
            resulting,
            destination,
            program,
            transfer_root,
        ]
        .contains(&[0; 32])
            || sequence == 0
            || !(1..=3).contains(&kind)
            || !fees.is_valid()
            || recorded_root != transfer_root
            || charged == 0
            || cumulative_spent < charged
        {
            return Err(Malformed);
        }
        let execution_fee = [
            (usage.cpu_fuel, fees.cpu_price()),
            (usage.memory_bytes, fees.memory_byte_price()),
            (usage.storage_read_bytes, fees.storage_read_byte_price()),
            (usage.storage_write_bytes, fees.storage_write_byte_price()),
            (u64::from(usage.output_values), fees.output_value_price()),
            (usage.output_bytes, fees.output_byte_price()),
        ]
        .into_iter()
        .try_fold(0u128, |total, (units, price)| {
            total.checked_add(u128::from(units).checked_mul(u128::from(price))?)
        })
        .ok_or(Malformed)?;
        let occupancy_fee = usage
            .occupancy_byte_batches
            .checked_mul(u128::from(fees.occupancy_byte_batch_price()))
            .ok_or(Malformed)?;
        if usage.fee_units != execution_fee
            || usage.occupancy_fee_units != occupancy_fee
            || charged != execution_fee.checked_add(occupancy_fee).ok_or(Malformed)?
        {
            return Err(Malformed);
        }
        let receipt_digest =
            crate::hash_bytes(crate::HashAlgorithm::Sha256, receipt).map_err(|_| Malformed)?;
        Ok(Self {
            receipt: if retain { receipt.to_vec() } else { Vec::new() },
            receipt_digest,
            transfer_root,
            lease,
            sequence,
            observed_batch,
            activity_id,
            program,
            kind,
            usage,
            fees,
            charged,
            lease_terms_digest: terms,
            expected_lease_digest: expected,
            resulting_lease_digest: resulting,
            fee_destination: destination,
            previous_receipt,
            previous_accumulator,
            cumulative_usage,
            cumulative_spent,
        })
    }
}

/// Wraps the actual execution bytes and canonical usage receipt for native publication.
pub fn attach_sandbox_settlement(
    execution: &mut Vec<u8>,
    receipt: &[u8],
    transfer_root: [u8; 32],
) -> Result<(), TerminalDecodeError> {
    let settlement = SandboxSettlement::read(receipt, transfer_root, false)?;
    let execution_length = execution.len();
    let prefix = SANDBOX_SETTLEMENT_DOMAIN.len() + 4;
    let length = prefix
        .checked_add(execution_length)
        .and_then(|n| n.checked_add(4 + receipt.len() + 32))
        .ok_or(TerminalDecodeError::Malformed)?;
    let encoded_length =
        u32::try_from(execution_length).map_err(|_| TerminalDecodeError::Malformed)?;
    execution.reserve(length - execution_length);
    execution.resize(prefix + execution_length, 0);
    execution.copy_within(0..execution_length, prefix);
    execution[..SANDBOX_SETTLEMENT_DOMAIN.len()].copy_from_slice(SANDBOX_SETTLEMENT_DOMAIN);
    execution[SANDBOX_SETTLEMENT_DOMAIN.len()..prefix]
        .copy_from_slice(&encoded_length.to_be_bytes());
    execution.extend_from_slice(&(receipt.len() as u32).to_be_bytes());
    settlement.append_usage_receipt(execution);
    execution.extend_from_slice(&transfer_root);
    Ok(())
}
