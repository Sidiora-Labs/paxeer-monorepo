use crate::storage::StorageError;
use crate::{
    AbiError, BudgetedV1FailureCause, CompositionRefusal, EntrypointRefusal, ExecutionFault,
    MeterRefusal, ResourceKind, ResponseRefusal,
};

const LENGTH_LIMIT: i32 = -5;

fn put_usize(out: &mut Vec<u8>, value: usize) -> Result<(), i32> {
    out.extend_from_slice(
        &u64::try_from(value)
            .map_err(|_| LENGTH_LIMIT)?
            .to_be_bytes(),
    );
    Ok(())
}
fn put_text(out: &mut Vec<u8>, value: &str) -> Result<(), i32> {
    out.extend_from_slice(
        &u32::try_from(value.len())
            .map_err(|_| LENGTH_LIMIT)?
            .to_be_bytes(),
    );
    out.extend_from_slice(value.as_bytes());
    Ok(())
}
const fn meter_kind(value: ResourceKind) -> u8 {
    match value {
        ResourceKind::Cpu => 1,
        ResourceKind::Memory => 2,
        ResourceKind::StorageRead => 3,
        ResourceKind::StorageWrite => 4,
        ResourceKind::StorageOccupancy => 5,
        ResourceKind::Output => 6,
        ResourceKind::OutputBytes => 7,
    }
}
fn meter_payload(value: MeterRefusal) -> Vec<u8> {
    let mut out = Vec::new();
    match value {
        MeterRefusal::BudgetExceeded {
            resource,
            limit,
            attempted,
        } => {
            out.extend([1, meter_kind(resource)]);
            out.extend_from_slice(&limit.to_be_bytes());
            out.extend_from_slice(&attempted.to_be_bytes());
        }
        MeterRefusal::CounterOverflow { resource } => out.extend([2, meter_kind(resource)]),
        MeterRefusal::FeeOverflow => out.push(3),
    }
    out
}
const fn storage_tag(value: StorageError) -> u8 {
    match value {
        StorageError::InvalidProgram => 1,
        StorageError::InvalidPrincipal => 2,
        StorageError::EmptyKey => 3,
        StorageError::KeyTooLarge => 4,
        StorageError::ValueTooLarge => 5,
        StorageError::PrefixTooLarge => 6,
        StorageError::InvalidScanCursor => 7,
        StorageError::InvalidScanLimits => 8,
        StorageError::ScanCeilingExceeded => 9,
        StorageError::SizeOverflow => 10,
        StorageError::FrozenNamespace => 11,
    }
}
fn abi_payload(value: &AbiError) -> Vec<u8> {
    match value {
        AbiError::WrongVersion => vec![1],
        AbiError::InvalidCapability => vec![2],
        AbiError::DuplicateCapability => vec![3],
        AbiError::CapabilityDenied => vec![4],
        AbiError::CapabilityEscalation => vec![5],
        AbiError::EventBounds => vec![6],
        AbiError::CallBounds => vec![7],
        AbiError::AmountBounds => vec![8],
        AbiError::ReceiptMismatch => vec![9],
        AbiError::BalanceAbsent => vec![13],
        AbiError::BalanceEvidenceUnavailable => vec![14],
        AbiError::AccessDeclaration => vec![15],
        AbiError::OracleUnknownMarket => vec![16],
        AbiError::OracleMarketHalted => vec![17],
        AbiError::EffectLimit => vec![18],
        AbiError::InvalidEncoding => vec![10],
        AbiError::Storage(error) => vec![11, storage_tag(*error)],
        AbiError::Meter(error) => {
            let mut out = vec![12];
            out.extend(meter_payload(*error));
            out
        }
    }
}
fn fault_payload(value: &ExecutionFault) -> Result<Vec<u8>, i32> {
    let mut out = Vec::new();
    match value {
        ExecutionFault::UnknownExport { name } => {
            out.push(1);
            put_text(&mut out, name)?;
        }
        ExecutionFault::NotAFunction { name } => {
            out.push(2);
            put_text(&mut out, name)?;
        }
        ExecutionFault::UnreachableExecuted => out.push(3),
        ExecutionFault::MemoryOutOfBounds => out.push(4),
        ExecutionFault::TableOutOfBounds => out.push(5),
        ExecutionFault::IndirectCallToNull => out.push(6),
        ExecutionFault::IntegerDivisionByZero => out.push(7),
        ExecutionFault::IntegerOverflow => out.push(8),
        ExecutionFault::BadConversionToInteger => out.push(9),
        ExecutionFault::StackExhausted => out.push(10),
        ExecutionFault::BadSignature => out.push(11),
        ExecutionFault::OutOfFuel => out.push(12),
        ExecutionFault::GrowthLimited => out.push(13),
        ExecutionFault::Resource { refusal } => {
            out.push(14);
            out.extend(meter_payload(*refusal));
        }
        ExecutionFault::NonIntegerValue => out.push(15),
        ExecutionFault::EngineFault { reason } => {
            out.push(16);
            put_text(&mut out, reason)?;
        }
    }
    Ok(out)
}
fn response_payload(value: &ResponseRefusal) -> Result<Vec<u8>, i32> {
    let mut out = Vec::new();
    match value {
        ResponseRefusal::TooLarge { bytes, limit } => {
            out.push(1);
            put_usize(&mut out, *bytes)?;
            put_usize(&mut out, *limit)?;
        }
        ResponseRefusal::CapacityExceeded { bytes, capacity } => {
            out.push(2);
            put_usize(&mut out, *bytes)?;
            put_usize(&mut out, *capacity)?;
        }
        ResponseRefusal::DuplicatePublication => out.push(3),
        ResponseRefusal::InvalidPublication => out.push(4),
        ResponseRefusal::CodeMismatch {
            published,
            returned,
        } => {
            out.push(5);
            out.extend_from_slice(&published.to_be_bytes());
            out.extend_from_slice(&returned.to_be_bytes());
        }
        ResponseRefusal::Meter(error) => {
            out.push(6);
            out.extend(meter_payload(*error));
        }
    }
    Ok(out)
}
fn entry_payload(value: &EntrypointRefusal) -> Result<Vec<u8>, i32> {
    let mut out = Vec::new();
    match value {
        EntrypointRefusal::InputTooLarge { bytes, limit } => {
            out.push(1);
            put_usize(&mut out, *bytes)?;
            put_usize(&mut out, *limit)?;
        }
        EntrypointRefusal::MissingAllocator => out.push(2),
        EntrypointRefusal::MissingMemory => out.push(3),
        EntrypointRefusal::MissingEntry => out.push(4),
        EntrypointRefusal::AllocationRefused { code } => {
            out.push(5);
            out.extend_from_slice(&code.to_be_bytes());
        }
        EntrypointRefusal::GuestRefused { code } => {
            out.push(6);
            out.extend_from_slice(&code.to_be_bytes());
        }
        EntrypointRefusal::Fault(fault) => {
            out.push(7);
            out.extend(fault_payload(fault)?);
        }
        EntrypointRefusal::Resource(error) => {
            out.push(8);
            out.extend(meter_payload(*error));
        }
    }
    Ok(out)
}
fn composition_payload(value: &CompositionRefusal) -> Result<Vec<u8>, i32> {
    let mut out = Vec::new();
    match value {
        CompositionRefusal::NotComposable => out.push(1),
        CompositionRefusal::ActivityEvidenceRequired => out.push(20),
        CompositionRefusal::ActivityEvidenceMismatch => out.push(21),
        CompositionRefusal::ActivityEvidenceReused => out.push(22),
        CompositionRefusal::WrongVersion { expected, actual } => {
            out.extend([
                2,
                u8::try_from(crate::abi_policy::abi_version(*expected))
                    .map_err(|_| LENGTH_LIMIT)?,
                u8::try_from(crate::abi_policy::abi_version(*actual)).map_err(|_| LENGTH_LIMIT)?,
            ]);
        }
        CompositionRefusal::MeteringPlanMismatch { expected, actual } => {
            out.push(23);
            out.extend_from_slice(expected);
            out.extend_from_slice(actual);
        }
        CompositionRefusal::UnknownProgram { program } => {
            out.push(3);
            out.extend_from_slice(&program.bytes());
        }
        CompositionRefusal::Reentrancy { program } => {
            out.push(4);
            out.extend_from_slice(&program.bytes());
        }
        CompositionRefusal::DepthExceeded { limit, attempted } => {
            out.push(5);
            out.extend_from_slice(&limit.to_be_bytes());
            out.extend_from_slice(&attempted.to_be_bytes());
        }
        CompositionRefusal::EdgesExceeded { limit, attempted } => {
            out.push(6);
            out.extend_from_slice(&limit.to_be_bytes());
            out.extend_from_slice(&attempted.to_be_bytes());
        }
        CompositionRefusal::FanoutExceeded { limit, attempted } => {
            out.push(7);
            out.extend_from_slice(&limit.to_be_bytes());
            out.extend_from_slice(&attempted.to_be_bytes());
        }
        CompositionRefusal::VisitsExceeded {
            program,
            limit,
            attempted,
        } => {
            out.push(8);
            out.extend_from_slice(&program.bytes());
            out.extend_from_slice(&limit.to_be_bytes());
            out.extend_from_slice(&attempted.to_be_bytes());
        }
        CompositionRefusal::MissingEntry => out.push(9),
        CompositionRefusal::MissingAllocator => out.push(10),
        CompositionRefusal::MissingMemory => out.push(11),
        CompositionRefusal::AllocationRefused { code } => {
            out.push(12);
            out.extend_from_slice(&code.to_be_bytes());
        }
        CompositionRefusal::InputTooLarge { bytes, limit } => {
            out.push(13);
            put_usize(&mut out, *bytes)?;
            put_usize(&mut out, *limit)?;
        }
        CompositionRefusal::GuestRefused { program, code } => {
            out.push(14);
            out.extend_from_slice(&program.bytes());
            out.extend_from_slice(&code.to_be_bytes());
        }
        CompositionRefusal::Program(failure) => {
            out.push(15);
            out.extend(failure.canonical_encode());
        }
        CompositionRefusal::Authority(error) => {
            out.push(16);
            out.extend(abi_payload(error));
        }
        CompositionRefusal::Fault(fault) => {
            out.push(17);
            out.extend(fault_payload(fault)?);
        }
        CompositionRefusal::Resource(error) => {
            out.push(18);
            out.extend(meter_payload(*error));
        }
        CompositionRefusal::Response(error) => {
            out.push(19);
            out.extend(response_payload(error)?);
        }
    }
    Ok(out)
}

/// Encodes the typed terminal failure transported by the Programs call bridge.
///
/// # Errors
///
/// Returns the bridge's length refusal (-5) if a field cannot be represented.
pub fn encode_failure_payload(cause: &BudgetedV1FailureCause) -> Result<Vec<u8>, i32> {
    let mut encoded = super::FAILURE.to_vec();
    let (tag, payload) = match cause {
        BudgetedV1FailureCause::Program(failure) => (1_u8, failure.canonical_encode()),
        BudgetedV1FailureCause::Composition(refusal) => (2, composition_payload(refusal)?),
        BudgetedV1FailureCause::Entrypoint(refusal) => (3, entry_payload(refusal)?),
        BudgetedV1FailureCause::Abi(refusal) => (4, abi_payload(refusal)),
    };
    encoded.push(tag);
    encoded.extend_from_slice(
        &u32::try_from(payload.len())
            .map_err(|_| LENGTH_LIMIT)?
            .to_be_bytes(),
    );
    encoded.extend_from_slice(&payload);
    Ok(encoded)
}
