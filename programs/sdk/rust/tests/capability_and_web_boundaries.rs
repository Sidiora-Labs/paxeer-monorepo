//! Checks SDK capability and web answer boundaries with production code.

use std::sync::Arc;

use layerx_program_sdk::payments::{PaymentGrant, ProgramPaymentCapabilities};
use layerx_program_sdk::{
    web::{Answer, ANSWER_HEADER_BYTES, MAX_RESPONSE_BYTES, RECORD_BYTES},
    AccountId, Amount, AssetId, Capability, CapabilitySet, Field, HostRefusal, ProgramError,
    ProgramId, Reason, ReceiptDigest,
};
use layerx_programs_runtime::test_support::{
    code_section, func_body, function_section, import_section, module, raw_section, type_section,
    unsigned_leb, TYPE_I32,
};
use layerx_programs_runtime::{
    Abi, AbiError, AuthorizationContext, Capability as HostCapability,
    CapabilitySet as HostCapabilities, CommittedWebAnswers, FeeSchedule, Meter, PrincipalId,
    ProgramId as HostProgramId, ResourceBudget, Storage, UnavailableReceiptOracle, WasmEngine,
    WasmValue, WebAnswer, ABI_V1_VERSION, ABI_V4_VERSION,
};

fn transfer(maximum_amount: Amount) -> Capability {
    Capability::Transfer402 {
        asset: AssetId::new([2; 32]).unwrap_or_else(|error| panic!("test value: {error:?}")),
        to: AccountId::new([3; 32]).unwrap_or_else(|error| panic!("test value: {error:?}")),
        maximum_amount,
    }
}

#[test]
fn finding_012_direct_zero_ceiling_is_refused_without_mutating_either_set() {
    let invalid = transfer(Amount::ZERO);
    let error = ProgramError::value(Field::Amount, Reason::Zero);
    let mut basic = CapabilitySet::<2>::from_grants(&[Capability::StorageRead])
        .unwrap_or_else(|error| panic!("test value: {error:?}"));
    let before = basic;
    assert_eq!(basic.insert(invalid), Err(error));
    assert_eq!(basic, before);
    assert_eq!(CapabilitySet::<1>::from_grants(&[invalid]), Err(error));
    assert_eq!(
        Capability::transfer(
            AssetId::new([2; 32]).unwrap_or_else(|error| panic!("test value: {error:?}")),
            AccountId::new([3; 32]).unwrap_or_else(|error| panic!("test value: {error:?}")),
            Amount::ZERO,
        ),
        Err(error)
    );

    let mut payments = ProgramPaymentCapabilities::<2>::empty();
    payments
        .insert(PaymentGrant::Basic(Capability::StorageRead))
        .unwrap_or_else(|error| panic!("test value: {error:?}"));
    let before = payments;
    assert_eq!(payments.insert(PaymentGrant::Basic(invalid)), Err(error));
    assert_eq!(payments, before);

    // Refusal also leaves the previously held authority's wire bytes unchanged.
    let mut basic_bytes = [0xaa; 16];
    let mut payment_bytes = [0xaa; 16];
    assert_eq!(basic.encode_into(&mut basic_bytes), Ok(3));
    assert_eq!(payments.encode_into(&mut payment_bytes), Ok(3));
    assert_eq!(&basic_bytes[..3], &[0, 1, 1]);
    assert_eq!(payment_bytes, basic_bytes);
}

#[test]
fn finding_012_sdk_refuses_zero_before_producing_an_encoding() {
    let mut output = [0xa5; 83];
    let refused = CapabilitySet::<1>::from_grants(&[transfer(Amount::ZERO)])
        .and_then(|set| set.encode_into(&mut output));
    assert_eq!(
        refused,
        Err(ProgramError::value(Field::Amount, Reason::Zero))
    );
    assert_eq!(output, [0xa5; 83]);

    let mut payments = ProgramPaymentCapabilities::<1>::empty();
    let refused = payments
        .insert(PaymentGrant::Basic(transfer(Amount::ZERO)))
        .and_then(|()| payments.encode_into(&mut output));
    assert_eq!(
        refused,
        Err(ProgramError::value(Field::Amount, Reason::Zero))
    );
    assert_eq!(output, [0xa5; 83]);
    assert!(payments.is_empty());
}

#[test]
fn finding_012_all_public_variants_reach_the_real_runtime_codec() {
    for maximum in [1, u128::MAX] {
        let grants = [
            Capability::SharedStorageWrite,
            Capability::SharedStorageRead,
            Capability::ReceiptRead {
                receipt_digest: ReceiptDigest::new([4; 32])
                    .unwrap_or_else(|error| panic!("test value: {error:?}")),
            },
            transfer(Amount::from_u128(maximum)),
            Capability::Call {
                program: ProgramId::new([1; 32])
                    .unwrap_or_else(|error| panic!("test value: {error:?}")),
            },
            Capability::EmitEvent,
            Capability::StorageWrite,
            Capability::StorageRead,
        ];
        let set = CapabilitySet::<8>::from_grants(&grants)
            .unwrap_or_else(|error| panic!("test value: {error:?}"));
        let mut encoded = vec![0; set.encoded_len()];
        assert_eq!(set.encode_into(&mut encoded), Ok(encoded.len()));
        let expected = HostCapabilities::new([
            HostCapability::StorageRead,
            HostCapability::StorageWrite,
            HostCapability::EmitEvent,
            HostCapability::Call {
                program: HostProgramId::new([1; 32])
                    .unwrap_or_else(|error| panic!("test value: {error:?}")),
            },
            HostCapability::Transfer402 {
                asset: [2; 32],
                to: [3; 32],
                maximum_amount: maximum,
            },
            HostCapability::ReceiptRead {
                receipt_digest: [4; 32],
            },
            HostCapability::SharedStorageRead,
            HostCapability::SharedStorageWrite,
        ])
        .unwrap_or_else(|error| panic!("test value: {error:?}"));
        assert_eq!(encoded, expected.canonical_encoding());
        assert_eq!(
            HostCapabilities::decode_canonical(&encoded).and_then(HostCapabilities::new),
            Ok(expected.clone())
        );
        assert_eq!(
            HostCapabilities::decode_v2_canonical(&encoded).and_then(HostCapabilities::new),
            Ok(expected)
        );

        let mut payments = ProgramPaymentCapabilities::<8>::empty();
        for grant in grants {
            payments
                .insert(PaymentGrant::Basic(grant))
                .unwrap_or_else(|error| panic!("test value: {error:?}"));
        }
        let mut payment_bytes = vec![0; payments.encoded_len()];
        assert_eq!(payments.encode_into(&mut payment_bytes), Ok(encoded.len()));
        assert_eq!(payment_bytes, encoded);
        assert_eq!(
            payments.insert(PaymentGrant::Basic(Capability::EmitEvent)),
            Err(ProgramError::value(Field::Capability, Reason::TooLarge))
        );
    }
    let mut set = CapabilitySet::<2>::from_grants(&[transfer(Amount::from_u128(1))])
        .unwrap_or_else(|error| panic!("test value: {error:?}"));
    assert_eq!(
        set.insert(transfer(Amount::from_u128(2))),
        Err(ProgramError::value(Field::Capability, Reason::Duplicate))
    );
}

// A committed guest fixture calls the real program_call linker registration.
// Its data comes from the SDK producer; only the ceiling is corrupted below.
fn forwarding_guest(callee: HostProgramId, encoded: &[u8]) -> Vec<u8> {
    let mut exports = unsigned_leb(2);
    for (name, kind, index) in [("forward", 0, 1), ("memory", 2, 0)] {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.extend([kind, index]);
    }
    let mut payload = callee.bytes().to_vec();
    payload.extend_from_slice(encoded);
    let mut data = vec![1, 0, 0x41, 0, 0x0b];
    data.extend(unsigned_leb(payload.len() as u64));
    data.extend(payload);
    module(&[
        type_section(&[(&[TYPE_I32; 6], &[TYPE_I32]), (&[TYPE_I32], &[TYPE_I32])]),
        import_section(&[("layerx_v1", "program_call", 0)]),
        function_section(&[1]),
        raw_section(5, &[1, 1, 1, 1]),
        raw_section(7, &exports),
        code_section(&[func_body(
            &[],
            &[
                0x41, 0, 0x41, 32, 0x41, 0, 0x41, 0, 0x41, 32, 0x20, 0, 0x10, 0, 0x0b,
            ],
        )]),
        raw_section(11, &data),
    ])
}

#[test]
fn finding_012_real_host_still_rejects_a_zero_ceiling_on_the_wire() {
    let set = CapabilitySet::<1>::from_grants(&[transfer(Amount::from_u128(1))])
        .unwrap_or_else(|error| panic!("test value: {error:?}"));
    let mut encoded = vec![0; set.encoded_len()];
    set.encode_into(&mut encoded)
        .unwrap_or_else(|error| panic!("test value: {error:?}"));
    let amount_offset = encoded.len() - 16;
    encoded[amount_offset..].fill(0);
    assert_eq!(
        HostCapabilities::decode_canonical(&encoded),
        Err(AbiError::InvalidCapability)
    );
    assert_eq!(
        HostCapabilities::decode_v2_canonical(&encoded),
        Err(AbiError::InvalidCapability)
    );
    assert_eq!(
        HostCapabilities::new([HostCapability::Transfer402 {
            asset: [2; 32],
            to: [3; 32],
            maximum_amount: 0,
        }]),
        Err(AbiError::InvalidCapability)
    );

    let engine = WasmEngine::declared().unwrap_or_else(|error| panic!("test value: {error:?}"));
    let module = engine
        .validate(&forwarding_guest(
            HostProgramId::new([5; 32]).unwrap_or_else(|error| panic!("test value: {error:?}")),
            &encoded,
        ))
        .unwrap_or_else(|error| panic!("test value: {error:?}"));
    let abi = Abi::new(
        ABI_V1_VERSION,
        HostProgramId::new([6; 32]).unwrap_or_else(|error| panic!("test value: {error:?}")),
        AuthorizationContext::new(
            PrincipalId::new([7; 32]).unwrap_or_else(|error| panic!("test value: {error:?}")),
            HostCapabilities::empty(),
        ),
        Storage::new(),
        &UnavailableReceiptOracle,
    )
    .unwrap_or_else(|error| panic!("test value: {error:?}"));
    let mut instance = module
        .instantiate_sandbox(
            Meter::new(ResourceBudget::declared(), FeeSchedule::declared()),
            abi,
        )
        .unwrap_or_else(|error| panic!("test value: {error:?}"));
    assert_eq!(
        instance
            .call(
                "forward",
                &[WasmValue::I32(
                    i32::try_from(encoded.len())
                        .unwrap_or_else(|error| panic!("test value: {error:?}"))
                )]
            )
            .unwrap_or_else(|error| panic!("test value: {error:?}")),
        vec![WasmValue::I32(-2)]
    );
}

#[test]
fn finding_017_retirement_guidance_names_refusal_query_and_ownership() {
    let source = include_str!("../src/web.rs");
    let module_docs = source
        .split("use crate::error")
        .next()
        .unwrap_or_else(|| panic!("module documentation"));
    let read_docs = source
        .split("/// Calls the retired")
        .nth(1)
        .unwrap_or_else(|| panic!("retired call documentation"))
        .split("#[cfg(target_arch")
        .next()
        .unwrap_or_else(|| panic!("retired call boundary"));
    for docs in [module_docs, read_docs] {
        for fact in [
            "compatibility-only",
            "LXP_ERR_MODULE_DISABLED",
            "HostRefusal::Invalid",
            "(-2)",
            "Query.ProgramResult",
            "GetProgramResult",
        ] {
            assert!(docs.contains(fact), "missing documented fact {fact}");
        }
    }
    assert!(module_docs.contains("program_id"));
    assert!(module_docs.contains("request_id"));
    assert!(module_docs.contains("not caller authentication"));
    assert!(module_docs.contains("little-endian"));
    assert!(module_docs.contains("protobuf layout"));
    assert!(read_docs.contains("None"));
    assert!(read_docs.contains("historical absence"));
    assert_eq!(
        ProgramError::from_status(-2),
        Err(ProgramError::Host(HostRefusal::Invalid))
    );
}

#[test]
fn finding_017_frozen_decoder_consumes_real_runtime_records_with_owner_isolation() {
    let owner = HostProgramId::new([8; 32]).unwrap_or_else(|error| panic!("test value: {error:?}"));
    let foreign =
        HostProgramId::new([9; 32]).unwrap_or_else(|error| panic!("test value: {error:?}"));
    let mut store = CommittedWebAnswers::new();
    let response = vec![0x5a; MAX_RESPONSE_BYTES];
    store
        .commit(
            owner,
            42,
            WebAnswer {
                content_digest: [0x33; 32],
                full_length: 9_000,
                response: response.clone(),
            },
        )
        .unwrap_or_else(|error| panic!("test value: {error:?}"));
    let store = Arc::new(store);
    let read = |program| {
        Abi::new(
            ABI_V4_VERSION,
            program,
            AuthorizationContext::new(
                PrincipalId::new([7; 32]).unwrap_or_else(|error| panic!("test value: {error:?}")),
                HostCapabilities::empty(),
            ),
            Storage::new(),
            &UnavailableReceiptOracle,
        )
        .unwrap_or_else(|error| panic!("test value: {error:?}"))
        .with_committed_web(store.clone())
    };
    let abi = read(owner);
    assert_eq!(abi.web_read(43), Ok(None));
    assert_eq!(read(foreign).web_read(42), Ok(None));
    let committed = abi
        .web_read(42)
        .unwrap_or_else(|error| panic!("test value: {error:?}"))
        .unwrap_or_else(|| panic!("committed web answer"));
    let record = committed
        .canonical_bytes()
        .unwrap_or_else(|error| panic!("test value: {error:?}"));
    assert_eq!(record.len(), RECORD_BYTES);
    let answer =
        Answer::from_record(&record).unwrap_or_else(|error| panic!("test value: {error:?}"));
    assert_eq!(answer.content_digest, committed.content_digest);
    assert_eq!(answer.full_length, committed.full_length);
    assert_eq!(answer.response, response);
    assert!(answer.is_truncated());

    let malformed = Err(ProgramError::value(Field::Buffer, Reason::Malformed));
    assert_eq!(
        Answer::from_record(&record[..ANSWER_HEADER_BYTES - 1]),
        malformed
    );
    assert_eq!(Answer::from_record(&record[..record.len() - 1]), malformed);
    let mut bad_length = record.clone();
    bad_length[32..36].copy_from_slice(&0_u32.to_le_bytes());
    assert_eq!(Answer::from_record(&bad_length), malformed);
    let oversized = WebAnswer {
        content_digest: [1; 32],
        full_length: 5_000,
        response: vec![0; MAX_RESPONSE_BYTES + 1],
    };
    assert_eq!(oversized.canonical_bytes(), Err(AbiError::InvalidEncoding));
    let complete = WebAnswer {
        content_digest: [2; 32],
        full_length: 0,
        response: vec![],
    };
    let record = complete
        .canonical_bytes()
        .unwrap_or_else(|error| panic!("test value: {error:?}"));
    assert!(!Answer::from_record(&record)
        .unwrap_or_else(|error| panic!("test value: {error:?}"))
        .is_truncated());
}
