//! Repair record for audit 22.30.127, task 22.31.261.
//! Finding 1: `finding_1_single_field_complete_message_ceiling` and
//! `finding_1_accumulated_fields_share_the_transport_ceiling`.
//! Finding 2: `finding_2_all_frozen_refusals_round_trip`,
//! `finding_2_unknown_and_malformed_refusals_still_reject`, and
//! `finding_2_authorized_cached_refusal_survives_capture_and_restore`.
//! Baseline verification was not run: the task's ordered gate instructions
//! require implementation first and prohibit invoking `verify_cmd` by hand.
//! No out-of-path wiring is required by either repair proposal.

use layerx_programs_runtime::abi::{Calldata, CodecError, MAX_CALLDATA_BYTES};
use layerx_programs_runtime::replay::{CapturedSemanticReplayV2, ReplayWitnessError};
use layerx_programs_runtime::storage::StorageError;
use layerx_programs_runtime::test_support::add_module;
use layerx_programs_runtime::{
    Abi, AbiError, AuthorizationContext, Capability, CapabilitySet, Meter, MeterRefusal,
    PrincipalId, ProgramId, ResourceKind, Storage, StorageNamespace, UnavailableReceiptOracle,
    WasmEngine, ABI_V2_VERSION,
};

#[test]
fn finding_1_single_field_complete_message_ceiling() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    assert_eq!(MAX_CALLDATA_BYTES, 1_048_576);
    let maximum_value = vec![0xa5; MAX_CALLDATA_BYTES - 6];
    let mut calldata = Calldata::new();
    calldata
        .encode_bytes(&maximum_value)
        .assert_success("test value");
    let transport = calldata.as_bytes();
    assert_eq!(transport.len(), MAX_CALLDATA_BYTES);
    let parsed = Calldata::from_bytes(&transport).assert_success("test value");
    assert_eq!(parsed.payload(), calldata.payload());
    assert_eq!(&parsed.payload()[5..], maximum_value);
    assert_eq!(parsed.as_bytes(), transport);

    let mut oversized_value = maximum_value;
    oversized_value.push(0xa5);
    let mut fresh = Calldata::new();
    let before = fresh.as_bytes();
    assert_eq!(
        fresh.encode_bytes(&oversized_value),
        Err(CodecError::InputTooLarge)
    );
    assert_eq!(fresh.as_bytes(), before);
    assert!(Calldata::from_bytes(&fresh.as_bytes()).is_ok());

    let mut oversized_transport = transport;
    oversized_transport[2..6].copy_from_slice(
        &u32::try_from(oversized_value.len())
            .assert_success("test value")
            .to_be_bytes(),
    );
    oversized_transport.push(0xa5);
    assert_eq!(oversized_transport.len(), MAX_CALLDATA_BYTES + 1);
    assert!(matches!(
        Calldata::from_bytes(&oversized_transport),
        Err(CodecError::InputTooLarge)
    ));
}

#[test]
fn finding_1_accumulated_fields_share_the_transport_ceiling() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let mut calldata = Calldata::new();
    calldata.encode_u64(u64::MAX).assert_success("test value");
    calldata
        .encode_bytes(&[0x3c; 257])
        .assert_success("test value");
    calldata.encode_option_none().assert_success("test value");
    let prefix = calldata.as_bytes();
    let remaining = MAX_CALLDATA_BYTES - prefix.len() - 5;
    let mut final_value = vec![0x5a; remaining + 1];

    assert_eq!(
        calldata.encode_bytes(&final_value),
        Err(CodecError::InputTooLarge)
    );
    assert_eq!(calldata.as_bytes(), prefix);
    assert_eq!(
        Calldata::from_bytes(&prefix)
            .assert_success("test value")
            .as_bytes(),
        prefix
    );
    final_value.pop();
    calldata
        .encode_bytes(&final_value)
        .assert_success("test value");
    let transport = calldata.as_bytes();
    assert_eq!(transport.len(), MAX_CALLDATA_BYTES);
    assert_eq!(&transport[..prefix.len()], prefix);
    let parsed = Calldata::from_bytes(&transport).assert_success("test value");
    assert_eq!(parsed.as_bytes(), transport);
    assert_eq!(
        &parsed.payload()[parsed.payload().len() - remaining..],
        final_value
    );

    assert_eq!(calldata.encode_bytes(&[]), Err(CodecError::InputTooLarge));
    assert_eq!(calldata.encode_u8(1), Err(CodecError::InputTooLarge));
    assert_eq!(calldata.as_bytes(), transport);
    let mut oversized_transport = transport;
    let length_offset = prefix.len() + 1;
    oversized_transport[length_offset..length_offset + 4].copy_from_slice(
        &u32::try_from(remaining + 1)
            .assert_success("test value")
            .to_be_bytes(),
    );
    oversized_transport.push(0x5a);
    assert!(matches!(
        Calldata::from_bytes(&oversized_transport),
        Err(CodecError::InputTooLarge)
    ));
}

fn frozen_error_tag(error: &AbiError) -> u8 {
    match error {
        AbiError::WrongVersion => 0,
        AbiError::InvalidCapability => 1,
        AbiError::DuplicateCapability => 2,
        AbiError::CapabilityDenied => 3,
        AbiError::CapabilityEscalation => 4,
        AbiError::EventBounds => 5,
        AbiError::CallBounds => 6,
        AbiError::AmountBounds => 7,
        AbiError::ReceiptMismatch => 8,
        AbiError::BalanceAbsent => 9,
        AbiError::BalanceEvidenceUnavailable => 10,
        AbiError::InvalidEncoding => 11,
        AbiError::Storage(_) => 12,
        AbiError::Meter(_) => 13,
        AbiError::AccessDeclaration => 14,
        AbiError::OracleUnknownMarket => 15,
        AbiError::OracleMarketHalted => 16,
        AbiError::EffectLimit => 17,
    }
}

fn refusal_cases() -> Vec<(AbiError, Vec<u8>)> {
    let mut cases = Vec::new();
    for error in [
        AbiError::WrongVersion,
        AbiError::InvalidCapability,
        AbiError::DuplicateCapability,
        AbiError::CapabilityDenied,
        AbiError::CapabilityEscalation,
        AbiError::EventBounds,
        AbiError::CallBounds,
        AbiError::AmountBounds,
        AbiError::ReceiptMismatch,
        AbiError::BalanceAbsent,
        AbiError::BalanceEvidenceUnavailable,
        AbiError::InvalidEncoding,
        AbiError::AccessDeclaration,
        AbiError::OracleUnknownMarket,
        AbiError::OracleMarketHalted,
        AbiError::EffectLimit,
    ] {
        let tag = frozen_error_tag(&error);
        cases.push((error, vec![tag]));
    }
    for error in [
        StorageError::InvalidProgram,
        StorageError::InvalidPrincipal,
        StorageError::EmptyKey,
        StorageError::KeyTooLarge,
        StorageError::ValueTooLarge,
        StorageError::PrefixTooLarge,
        StorageError::InvalidScanCursor,
        StorageError::InvalidScanLimits,
        StorageError::ScanCeilingExceeded,
        StorageError::FrozenNamespace,
        StorageError::SizeOverflow,
    ] {
        let code = match error {
            StorageError::InvalidProgram => 0,
            StorageError::InvalidPrincipal => 1,
            StorageError::EmptyKey => 2,
            StorageError::KeyTooLarge => 3,
            StorageError::ValueTooLarge => 4,
            StorageError::PrefixTooLarge => 5,
            StorageError::InvalidScanCursor => 6,
            StorageError::InvalidScanLimits => 7,
            StorageError::ScanCeilingExceeded => 8,
            StorageError::FrozenNamespace => 9,
            StorageError::SizeOverflow => 10,
        };
        cases.push((AbiError::Storage(error), vec![12, code]));
    }
    for resource in [
        ResourceKind::Cpu,
        ResourceKind::Memory,
        ResourceKind::StorageRead,
        ResourceKind::StorageWrite,
        ResourceKind::StorageOccupancy,
        ResourceKind::Output,
        ResourceKind::OutputBytes,
    ] {
        let code = match resource {
            ResourceKind::Cpu => 0,
            ResourceKind::Memory => 1,
            ResourceKind::StorageRead => 2,
            ResourceKind::StorageWrite => 3,
            ResourceKind::StorageOccupancy => 4,
            ResourceKind::Output => 5,
            ResourceKind::OutputBytes => 6,
        };
        let mut expected = vec![13, 0, code];
        expected.extend_from_slice(&(u64::MAX - 1).to_be_bytes());
        expected.extend_from_slice(&u64::MAX.to_be_bytes());
        cases.push((
            AbiError::Meter(MeterRefusal::BudgetExceeded {
                resource,
                limit: u64::MAX - 1,
                attempted: u64::MAX,
            }),
            expected,
        ));
        cases.push((
            AbiError::Meter(MeterRefusal::CounterOverflow { resource }),
            vec![13, 1, code],
        ));
    }
    cases.push((AbiError::Meter(MeterRefusal::FeeOverflow), vec![13, 2]));
    cases
}

#[test]
fn finding_2_all_frozen_refusals_round_trip() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let cases = refusal_cases();
    assert_eq!(cases.len(), 42);
    let mut seen = [false; 18];
    for (error, expected) in cases {
        let encoded = error.canonical_replay_bytes();
        assert_eq!(encoded, expected, "frozen encoding for {error:?}");
        seen[usize::from(frozen_error_tag(&error))] = true;
        let decoded = AbiError::from_canonical_replay_bytes(&encoded).assert_success("test value");
        assert_eq!(decoded, error);
        assert_eq!(decoded.canonical_replay_bytes(), encoded);
    }
    assert!(seen.into_iter().all(|present| present));
}

#[test]
fn finding_2_unknown_and_malformed_refusals_still_reject() {
    for (error, _) in refusal_cases() {
        let encoded = error.canonical_replay_bytes();
        for end in 0..encoded.len() {
            assert_eq!(
                AbiError::from_canonical_replay_bytes(&encoded[..end]),
                Err(ReplayWitnessError::Encoding),
                "truncated {error:?} at {end}"
            );
        }
        let mut trailing = encoded;
        trailing.push(0);
        assert_eq!(
            AbiError::from_canonical_replay_bytes(&trailing),
            Err(ReplayWitnessError::Encoding)
        );
    }
    for code in 18..=u8::MAX {
        assert_eq!(
            AbiError::from_canonical_replay_bytes(&[code]),
            Err(ReplayWitnessError::Encoding)
        );
    }
    for code in 11..=u8::MAX {
        assert_eq!(
            AbiError::from_canonical_replay_bytes(&[12, code]),
            Err(ReplayWitnessError::Encoding)
        );
    }
    for code in 3..=u8::MAX {
        assert_eq!(
            AbiError::from_canonical_replay_bytes(&[13, code]),
            Err(ReplayWitnessError::Encoding)
        );
    }
    for code in 7..=u8::MAX {
        for subtype in [0, 1] {
            let mut encoded = vec![13, subtype, code];
            encoded.extend_from_slice(&[0; 16]);
            assert_eq!(
                AbiError::from_canonical_replay_bytes(&encoded),
                Err(ReplayWitnessError::Encoding)
            );
        }
    }
}

const REPLAY_BYTES: usize = 1_048_576;
const ACCOUNT: [u8; 32] = [0x31; 32];
const ASSET: [u8; 32] = [0x32; 32];
const RECEIPT: [u8; 32] = [0x33; 32];

fn capture_cached_refusal(principal: PrincipalId) -> CapturedSemanticReplayV2 {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let program = ProgramId::new([0x11; 32]).assert_success("test value");
    let capabilities = CapabilitySet::new([
        Capability::StorageRead,
        Capability::BalanceView {
            account: ACCOUNT,
            asset: ASSET,
            receipt_digest: RECEIPT,
        },
    ])
    .assert_success("test value");
    let mut storage = Storage::new();
    let mut transaction = storage.transaction(StorageNamespace::principal(program, principal));
    transaction
        .write(b"persisted", b"authorized storage")
        .assert_success("test value");
    let _ = transaction.commit();
    let abi = Abi::new(
        ABI_V2_VERSION,
        program,
        AuthorizationContext::new(principal, capabilities),
        storage.clone(),
        &UnavailableReceiptOracle,
    )
    .assert_success("test value");
    assert_eq!(
        abi.balance_read(ACCOUNT, ASSET),
        Err(AbiError::BalanceEvidenceUnavailable)
    );
    assert_eq!(
        abi.balance_read([0x34; 32], ASSET),
        Err(AbiError::CapabilityDenied)
    );
    let module = WasmEngine::declared()
        .assert_success("test value")
        .validate_v2(&add_module())
        .assert_success("test value");
    let instance = module
        .instantiate_sandbox(Meter::declared(), abi)
        .assert_success("test value");
    assert_eq!(instance.storage_snapshot(), Some(storage));
    let captured = instance
        .capture_semantic_replay_source(REPLAY_BYTES)
        .assert_success("test value");
    drop(instance);
    captured
}

fn take<'a>(bytes: &mut &'a [u8], length: usize) -> &'a [u8] {
    let (value, rest) = bytes.split_at(length);
    *bytes = rest;
    value
}

fn field<'a>(bytes: &mut &'a [u8]) -> &'a [u8] {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let length = u32::from_be_bytes(take(bytes, 4).try_into().assert_success("test value"));
    take(bytes, usize::try_from(length).assert_success("test value"))
}

fn captured_balance_refusal(encoded: &[u8]) -> AbiError {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let mut semantic = encoded;
    let semantic_domain = b"LayerX/programs/semantic-replay-source/v2\0";
    assert_eq!(take(&mut semantic, semantic_domain.len()), semantic_domain);
    let mut host = field(&mut semantic);
    let host_domain = b"LayerX/programs/replay-host-witness/v1\0";
    assert_eq!(take(&mut host, host_domain.len()), host_domain);
    take(&mut host, 32);
    assert_eq!(take(&mut host, 1), [1]);
    let mut abi = field(&mut host);
    let abi_domain = b"LayerX/programs/v2/host-state\0";
    assert_eq!(take(&mut abi, abi_domain.len()), abi_domain);
    take(&mut abi, 2 + 32 + 32 + 9);
    field(&mut abi);
    field(&mut abi);
    field(&mut abi);
    let storage_domain = b"storage-overlay/v1\0";
    assert_eq!(take(&mut abi, storage_domain.len()), storage_domain);
    field(&mut abi);
    take(&mut abi, 8);
    assert_eq!(take(&mut abi, 4), 0_u32.to_be_bytes());
    assert_eq!(take(&mut abi, 4), 1_u32.to_be_bytes());
    assert_eq!(take(&mut abi, 32), ACCOUNT);
    assert_eq!(take(&mut abi, 32), ASSET);
    assert_eq!(take(&mut abi, 1), [0]);
    let refusal = field(&mut abi);
    let decoded = AbiError::from_canonical_replay_bytes(refusal).assert_success("test value");
    assert_eq!(decoded.canonical_replay_bytes(), refusal);
    decoded
}

#[test]
fn finding_2_authorized_cached_refusal_survives_capture_and_restore() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let captured =
        capture_cached_refusal(PrincipalId::new([0x22; 32]).assert_success("test value"));
    let encoded = captured
        .canonical_bytes(REPLAY_BYTES)
        .assert_success("test value");
    assert_eq!(
        captured_balance_refusal(&encoded),
        AbiError::BalanceEvidenceUnavailable
    );
    let restored = captured
        .restore_untrusted_bytes(&encoded, REPLAY_BYTES)
        .assert_success("test value");
    let recaptured = restored
        .recapture(REPLAY_BYTES)
        .assert_success("test value");
    let recovered = recaptured
        .canonical_bytes(REPLAY_BYTES)
        .assert_success("test value");
    assert_eq!(recovered, encoded);
    assert_eq!(
        captured_balance_refusal(&recovered),
        AbiError::BalanceEvidenceUnavailable
    );
    drop(restored);
    let reopened = recaptured
        .restore_untrusted_bytes(&recovered, REPLAY_BYTES)
        .assert_success("test value");
    assert_eq!(
        reopened
            .recapture(REPLAY_BYTES)
            .assert_success("test value")
            .canonical_bytes(REPLAY_BYTES)
            .assert_success("test value"),
        encoded
    );

    let other_authority =
        capture_cached_refusal(PrincipalId::new([0x23; 32]).assert_success("test value"));
    assert!(matches!(
        other_authority.restore_untrusted_bytes(&encoded, REPLAY_BYTES),
        Err(ReplayWitnessError::Binding)
    ));
    let mut tampered = encoded;
    let last = tampered.len() - 1;
    tampered[last] ^= 1;
    assert!(captured
        .restore_untrusted_bytes(&tampered, REPLAY_BYTES)
        .is_err());
}
