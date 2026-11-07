use layerx_programs_runtime::{
    abi::UnavailableReceiptOracle,
    abi_policy::{abi_revision, abi_version},
    terminal::{
        decode_terminal_payload, encode_failure_payload, FailureFields, FailureTerminal,
        TerminalDecodeError, TerminalDetail,
    },
    test_support::{
        code_section, func_body, function_section, import_section,
        invoke_composed_for_qualification, module, raw_section, type_section, unsigned_leb,
        TYPE_I32,
    },
    AuthorizationContext, AuthorizedExecutionRequest, BudgetedV1FailureCause, Capability,
    CapabilitySet, CompositionContext, CompositionRefusal, CompositionRules, ExecutionError,
    PrincipalId, ProgramCatalog, ProgramId, Storage, WasmEngine, ABI_MODULE, CALL_ENTRY_EXPORT,
};

const FAILURE: &[u8] = b"LXP/programs/failure-detail/v1\0";

fn id(byte: u8) -> ProgramId {
    ProgramId::new([byte; 32]).unwrap()
}

fn push_i32(out: &mut Vec<u8>, mut value: i32) {
    out.push(0x41);
    loop {
        let byte = u8::try_from(value & 0x7f).unwrap();
        value >>= 7;
        let done = (value == 0 && byte & 0x40 == 0) || (value == -1 && byte & 0x40 != 0);
        out.push(if done { byte } else { byte | 0x80 });
        if done {
            break;
        }
    }
}

fn guest(callee: Option<ProgramId>) -> Vec<u8> {
    let grants = CapabilitySet::empty().canonical_encoding();
    let imports = usize::from(callee.is_some());
    let mut exports = unsigned_leb(3);
    for (name, kind, index) in [
        ("layerx_reserve", 0, imports),
        (CALL_ENTRY_EXPORT, 0, imports + 1),
        ("memory", 2, 0),
    ] {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.push(kind);
        exports.extend(unsigned_leb(index as u64));
    }
    let mut entry = Vec::new();
    let mut data = Vec::new();
    if let Some(callee) = callee {
        for value in [0, 32, 32, 0, 32, i32::try_from(grants.len()).unwrap()] {
            push_i32(&mut entry, value);
        }
        entry.extend([0x10, 0]);
        data.extend_from_slice(&callee.bytes());
        data.extend(grants);
    } else {
        push_i32(&mut entry, 0);
    }
    entry.push(0x0b);
    let mut data_section = vec![1, 0, 0x41, 0, 0x0b];
    data_section.extend(unsigned_leb(data.len() as u64));
    data_section.extend(data);
    module(&[
        type_section(&[
            (&[TYPE_I32; 6], &[TYPE_I32]),
            (&[TYPE_I32], &[TYPE_I32]),
            (&[TYPE_I32; 2], &[TYPE_I32]),
        ]),
        import_section(&if callee.is_some() {
            vec![(ABI_MODULE, "program_call", 0)]
        } else {
            vec![]
        }),
        function_section(&[1, 2]),
        raw_section(5, &[1, 1, 1, 1]),
        raw_section(7, &exports),
        code_section(&[func_body(&[], &[0x41, 0, 0x0b]), func_body(&[], &entry)]),
        raw_section(11, &data_section),
    ])
}

fn invoke(expected: u16, actual: u16) -> Result<i32, ExecutionError> {
    let engine = WasmEngine::declared().unwrap();
    let root = engine
        .validate_versioned_metered(
            expected,
            &guest(Some(id(2))),
            layerx_programs_runtime::FuelSchedule::WASMI_0_31_2,
        )
        .unwrap();
    let child = engine
        .validate_versioned_metered(
            actual,
            &guest(None),
            layerx_programs_runtime::FuelSchedule::WASMI_0_31_2,
        )
        .unwrap();
    assert_eq!(root.abi_revision(), abi_revision(expected).unwrap());
    assert_eq!(child.abi_revision(), abi_revision(actual).unwrap());
    let mut catalog = ProgramCatalog::new();
    catalog.insert(id(2), child);
    let storage = Storage::new();
    let before = storage.clone();
    let outcome = invoke_composed_for_qualification(
        &storage,
        AuthorizedExecutionRequest {
            module: &root,
            program: id(1),
            authorization: AuthorizationContext::new(
                PrincipalId::new([3; 32]).unwrap(),
                CapabilitySet::new([Capability::Call { program: id(2) }]).unwrap(),
            ),
            receipts: &UnavailableReceiptOracle,
            entrypoint: CALL_ENTRY_EXPORT,
            calldata: &[],
            composition: CompositionContext::catalog(catalog, CompositionRules::declared()),
            response_capacity: 0,
        },
    );
    assert_eq!(storage, before);
    outcome
}

fn refusal_payload(expected: u16, actual: u16) -> Vec<u8> {
    let Err(ExecutionError::Composition(refusal)) = invoke(expected, actual) else {
        panic!("mixed ABI call did not produce a composition refusal");
    };
    assert_eq!(
        refusal,
        CompositionRefusal::WrongVersion {
            expected: abi_revision(expected).unwrap(),
            actual: abi_revision(actual).unwrap(),
        }
    );
    encode_failure_payload(&BudgetedV1FailureCause::Composition(refusal)).unwrap()
}

fn assert_receipt(payload: &[u8], expected: u8, actual: u8) {
    let decoded = decode_terminal_payload(2, u16::from(expected), payload).unwrap();
    assert_eq!(
        decoded.detail,
        TerminalDetail::Failure(FailureTerminal::Composition {
            tag: 2,
            fields: FailureFields::Revisions { expected, actual },
        })
    );
    assert!(decoded.attachments.is_empty());
    assert_eq!(decoded.execution_encoding_version(), None);
}

#[test]
fn finding_002_mixed_abi_3_4_and_4_5_receipts_round_trip() {
    for (expected, actual) in [(3, 4), (4, 3), (4, 5), (5, 4)] {
        let payload = refusal_payload(expected, actual);
        assert_receipt(&payload, expected as u8, actual as u8);
    }
}

#[test]
fn finding_002_every_allocated_revision_round_trips_real_refusals() {
    for expected in 1..=5 {
        assert_eq!(
            invoke(expected, expected).unwrap(),
            0,
            "same ABI {expected}"
        );
        for actual in 1..=5 {
            if expected == actual {
                continue;
            }
            let payload = refusal_payload(expected, actual);
            assert_eq!(&payload[..FAILURE.len()], FAILURE);
            assert_eq!(
                &payload[FAILURE.len()..],
                &[2, 0, 0, 0, 3, 2, expected as u8, actual as u8]
            );
            assert_receipt(&payload, expected as u8, actual as u8);
            assert_eq!(abi_version(abi_revision(expected).unwrap()), expected);
        }
    }
}

#[test]
fn finding_002_unallocated_tags_and_malformed_receipts_still_refuse() {
    let payload = refusal_payload(4, 5);
    let expected_offset = FAILURE.len() + 6;
    let actual_offset = expected_offset + 1;
    for tag in 0..=u8::MAX {
        assert_eq!(abi_revision(u16::from(tag)).is_ok(), (1..=5).contains(&tag));
        if (1..=5).contains(&tag) {
            continue;
        }
        for offset in [expected_offset, actual_offset] {
            let mut unknown = payload.clone();
            unknown[offset] = tag;
            assert_eq!(
                decode_terminal_payload(2, 4, &unknown),
                Err(TerminalDecodeError::Malformed),
                "unallocated revision tag {tag} at {offset}"
            );
        }
    }
    for end in 0..payload.len() {
        assert!(decode_terminal_payload(2, 4, &payload[..end]).is_err());
    }
    let mut trailing = payload.clone();
    trailing.push(0);
    assert_eq!(
        decode_terminal_payload(2, 4, &trailing),
        Err(TerminalDecodeError::Malformed)
    );
    for kind in [1, 3] {
        assert_eq!(
            decode_terminal_payload(kind, 4, &payload),
            Err(TerminalDecodeError::MismatchedKind)
        );
    }
    assert_receipt(&payload, 4, 5);
}
