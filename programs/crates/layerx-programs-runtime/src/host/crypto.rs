//! Host functions for deterministic cryptographic primitives.

use wasmi::{Caller, Linker};

use crate::crypto::hash::MAX_HASH_INPUT_BYTES;
use crate::crypto::{hash_bytes, HashAlgorithm, HashRefusal};
use crate::execute::ExecutionFault;

use super::memory::{nonnegative, validate_input, validate_output};
use super::{linker_fault, RuntimeState, STATUS_BOUNDS, STATUS_INVALID, STATUS_METER};

pub(super) fn register(linker: &mut Linker<RuntimeState>) -> Result<(), ExecutionFault> {
    linker
        .func_wrap(
            crate::abi::ABI_V2_MODULE,
            "hash",
            |mut caller: Caller<'_, RuntimeState>,
             algorithm_id: i32,
             input_pointer: i32,
             input_length: i32,
             output_pointer: i32|
             -> i32 {
                let algorithm_id = match nonnegative(algorithm_id)
                    .and_then(|id| u32::try_from(id).map_err(|_| STATUS_BOUNDS))
                {
                    Ok(id) => id,
                    Err(status) => return status,
                };
                let Ok(algorithm) = HashAlgorithm::from_identifier(algorithm_id) else {
                    return STATUS_INVALID;
                };
                let input_length = match nonnegative(input_length) {
                    Ok(length) => length,
                    Err(status) => return status,
                };
                let Ok(length) = u64::try_from(input_length) else {
                    return STATUS_BOUNDS;
                };
                if length > MAX_HASH_INPUT_BYTES {
                    return STATUS_BOUNDS;
                }
                let input = match validate_input(&caller, input_pointer, input_length) {
                    Ok(input) => input,
                    Err(status) => return status,
                };
                let Ok(output_length) = i32::try_from(algorithm.output_length()) else {
                    return STATUS_BOUNDS;
                };
                let output = match validate_output(&caller, output_pointer, output_length) {
                    Ok(output) => output,
                    Err(status) => return status,
                };
                let Some(fuel_cost) = length.checked_mul(algorithm.fuel_per_byte()) else {
                    return STATUS_BOUNDS;
                };
                if let Err(refusal) = super::charge_host_cpu(&mut caller, fuel_cost) {
                    caller
                        .data_mut()
                        .record_refusal(crate::calls::CompositionRefusal::Resource(refusal));
                    return STATUS_METER;
                }
                let input = input.read(&caller);
                let digest = match hash_bytes(algorithm, &input) {
                    Ok(digest) => digest,
                    Err(HashRefusal::InputTooLong { .. }) => return STATUS_BOUNDS,
                    Err(HashRefusal::UnknownAlgorithm { .. }) => return STATUS_INVALID,
                };
                if let Err(status) = output.write(&mut caller, &digest) {
                    return status;
                }
                0
            },
        )
        .map_err(|error| linker_fault(&error))?;
    Ok(())
}
