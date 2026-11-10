use layerx_programs_runtime::{
    derive_program_account, AbiError, AuthorizationContext, AuthorizedExecutionRequest, Capability,
    CapabilitySet, CompositionContext, CompositionRefusal, CompositionRules, ExecutionError,
    Executor, PrincipalId, ProgramCatalog, ProgramId, Storage, WasmEngine, CALL_ENTRY_EXPORT,
};

fn spend(owner: ProgramId, asset: [u8; 32], to: [u8; 32], maximum: u128) -> Capability {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let seed = b"composition/retained";
    Capability::ProgramSpend {
        owner_program: owner,
        seed: seed.to_vec(),
        source_account: derive_program_account(owner, seed)
            .assert_success("derived account")
            .bytes(),
        asset,
        to,
        maximum_amount: maximum,
    }
}

#[test]
fn canonical_program_grants_narrow_at_every_depth_and_repeated_visit() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let owner = ProgramId::new([1; 32]).assert_success("owner");
    let mut inherited =
        CapabilitySet::new([spend(owner, [2; 32], [3; 32], 100)]).assert_success("root grant");
    for maximum in (1..100).rev() {
        let bytes = inherited.canonical_encoding();
        let decoded = CapabilitySet::new(
            CapabilitySet::decode_v2_canonical(&bytes).assert_success("canonical grant"),
        )
        .assert_success("decoded grant");
        assert_eq!(decoded, inherited);
        assert_eq!(
            decoded.narrow([spend(owner, [2; 32], [3; 32], maximum + 2)]),
            Err(AbiError::CapabilityEscalation)
        );
        assert_eq!(
            decoded.narrow([spend(owner, [4; 32], [3; 32], maximum)]),
            Err(AbiError::CapabilityEscalation)
        );
        assert_eq!(
            decoded.narrow([spend(owner, [2; 32], [4; 32], maximum)]),
            Err(AbiError::CapabilityEscalation)
        );
        inherited = decoded
            .narrow([spend(owner, [2; 32], [3; 32], maximum)])
            .assert_success("strictly downward grant");
    }
}

#[test]
fn fanout_does_not_merge_distinct_principal_or_program_authority() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let owner = ProgramId::new([5; 32]).assert_success("owner");
    let child = ProgramId::new([6; 32]).assert_success("child");
    let root = CapabilitySet::new([
        Capability::Transfer402 {
            asset: [7; 32],
            to: [8; 32],
            maximum_amount: 200,
        },
        spend(owner, [7; 32], [8; 32], 100),
    ])
    .assert_success("distinct grants");
    for maximum in [20, 30, 40] {
        let branch = root
            .narrow([spend(owner, [7; 32], [8; 32], maximum)])
            .assert_success("branch");
        assert_eq!(
            branch.narrow([spend(owner, [7; 32], [8; 32], maximum + 1)]),
            Err(AbiError::CapabilityEscalation)
        );
        assert_eq!(
            branch.narrow([spend(child, [7; 32], [8; 32], maximum)]),
            Err(AbiError::CapabilityEscalation)
        );
        assert_eq!(
            branch.narrow([Capability::Transfer402 {
                asset: [7; 32],
                to: [8; 32],
                maximum_amount: maximum,
            }]),
            Err(AbiError::CapabilityDenied)
        );
    }
    assert!(root.narrow([spend(owner, [7; 32], [8; 32], 100)]).is_ok());
}

#[test]
fn encoded_program_grants_never_change_legacy_or_accept_unknown_tags() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let owner = ProgramId::new([9; 32]).assert_success("owner");
    let legacy = CapabilitySet::new([
        Capability::StorageRead,
        Capability::Transfer402 {
            asset: [10; 32],
            to: [11; 32],
            maximum_amount: 5,
        },
    ])
    .assert_success("legacy")
    .canonical_encoding();
    assert_eq!(
        CapabilitySet::decode_canonical(&legacy),
        CapabilitySet::decode_v2_canonical(&legacy)
    );
    let encoded = CapabilitySet::new([spend(owner, [10; 32], [11; 32], 5)])
        .assert_success("program grant")
        .canonical_encoding();
    assert_eq!(
        CapabilitySet::decode_canonical(&encoded),
        Err(AbiError::InvalidEncoding)
    );
    for tag in [0, 11, 255] {
        let mut unknown = encoded.clone();
        unknown[2] = tag;
        assert_eq!(
            CapabilitySet::decode_v2_canonical(&unknown),
            Err(AbiError::InvalidEncoding)
        );
    }
    for end in 0..encoded.len() {
        assert!(CapabilitySet::decode_v2_canonical(&encoded[..end]).is_err());
    }
    let mut trailing = encoded;
    trailing.push(0);
    assert_eq!(
        CapabilitySet::decode_v2_canonical(&trailing),
        Err(AbiError::InvalidEncoding)
    );
}

fn actual_guest(
    variant: u8,
) -> (
    Result<layerx_programs_runtime::V2AuthorizedExecutionRecord, ExecutionError>,
    Storage,
) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let directory = std::path::PathBuf::from(
        std::env::var("PAXEER_X_SPEND_GUESTS")
            .assert_success("source-bound native guest producer is required"),
    );
    let mut owner_bytes = [0; 32];
    owner_bytes[0] = 0x61;
    let mut child_bytes = [0; 32];
    child_bytes[0] = 0x62;
    let mut descendant_bytes = [0; 32];
    descendant_bytes[0] = 0x63;
    let owner = ProgramId::new(owner_bytes).assert_success("owner");
    let child = ProgramId::new(child_bytes).assert_success("child");
    let descendant = ProgramId::new(descendant_bytes).assert_success("descendant");
    let mut asset = [0; 32];
    asset[0] = 9;
    let payee: [u8; 32] = std::fs::read(directory.join("payee.bin"))
        .assert_success("actual native payee")
        .try_into()
        .assert_success("payee width");
    let seed = b"composition/vault";
    let grants = CapabilitySet::new([
        Capability::Call { program: child },
        Capability::Call {
            program: descendant,
        },
        Capability::ProgramSpend {
            owner_program: owner,
            seed: seed.to_vec(),
            source_account: derive_program_account(owner, seed)
                .assert_success("source")
                .bytes(),
            asset,
            to: payee,
            maximum_amount: 20,
        },
    ])
    .assert_success("grants");
    let engine = WasmEngine::declared().assert_success("real engine");
    let module = engine
        .validate_candidate_v2(
            &std::fs::read(directory.join(format!("case{variant}.owner.wasm")))
                .assert_success("owner guest"),
        )
        .assert_success("owner module");
    let callee = engine
        .validate_candidate_v2(
            &std::fs::read(directory.join(format!("case{variant}.child.wasm")))
                .assert_success("child guest"),
        )
        .assert_success("child module");
    let mut catalog = ProgramCatalog::new();
    assert!(catalog.insert(child, callee).is_none());
    let descendant_module = engine
        .validate_candidate_v2(
            &std::fs::read(directory.join(format!("case{variant}.descendant.wasm")))
                .assert_success("descendant guest"),
        )
        .assert_success("descendant module");
    assert!(catalog.insert(descendant, descendant_module).is_none());
    let mut storage = Storage::default();
    let result = Executor::declared().execute_authorized_candidate(
        &mut storage,
        AuthorizedExecutionRequest {
            module: &module,
            program: owner,
            authorization: AuthorizationContext::new(
                PrincipalId::new([3; 32]).assert_success("principal"),
                grants,
            ),
            receipts: &layerx_programs_runtime::abi::UnavailableReceiptOracle,
            entrypoint: CALL_ENTRY_EXPORT,
            calldata: &[],
            composition: CompositionContext::catalog(catalog, CompositionRules::declared()),
            response_capacity: 0,
        },
    );
    (result, storage)
}

#[test]
fn actual_guest_narrowing_preserves_owner_leg_and_repeated_visits() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    for variant in [0, 4] {
        let (result, storage) = actual_guest(variant);
        let record = result.assert_success("real composed guest succeeds");
        let layerx_programs_runtime::V2ActivityOutcome::Success { effects, .. } = record.outcome()
        else {
            panic!("actual guest did not succeed");
        };
        assert_eq!(effects.transfers.len(), 1);
        assert_eq!(effects.transfers[0].amount, 7);
        assert_eq!(effects.calls.len(), if variant == 4 { 8 } else { 1 });
        assert_eq!(storage, Storage::default());
    }
}

#[test]
fn actual_guest_escalation_rolls_back_the_preceding_owner_leg() {
    for variant in [1, 2, 3, 5, 6, 7] {
        let (result, storage) = actual_guest(variant);
        assert_eq!(
            result,
            Err(ExecutionError::Composition(CompositionRefusal::Authority(
                AbiError::CapabilityEscalation
            )))
        );
        assert_eq!(storage, Storage::default());
    }
}

#[test]
fn actual_native_failure_terminal_retains_class_reason_and_rejecting_frame() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    use layerx_programs_runtime::terminal::{
        decode_terminal_payload, CandidateTerminalOutcome, ExecutionTerminal, TerminalDetail,
    };
    let directory = std::path::PathBuf::from(
        std::env::var("PAXEER_X_SPEND_RESULTS")
            .assert_success("actual native signed receipt terminal evidence is required"),
    );
    for variant in [1, 2, 3, 5, 6, 7] {
        let encoded = std::fs::read(directory.join(format!("case{variant}.terminal.bin")))
            .assert_success("actual native terminal");
        let decoded =
            decode_terminal_payload(2, 2, &encoded).assert_success("canonical failure terminal");
        let TerminalDetail::Execution(ExecutionTerminal::CandidateV4 {
            outcome: CandidateTerminalOutcome::Failure(failure),
            ..
        }) = decoded.detail
        else {
            panic!("actual native terminal has the wrong outcome");
        };
        let mut frame = [0; 32];
        frame[0] = if variant == 5 { 0x62 } else { 0x61 };
        assert_eq!(
            failure.program(),
            ProgramId::new(frame).assert_success("rejecting frame")
        );
        assert_eq!(
            failure.class(),
            layerx_programs_runtime::RefusalClass::Unauthorized
        );
        assert_eq!(
            failure.reason().bytes(),
            b"LXP/programs/authority-refusal/v1\0\x05"
        );
    }
}

#[test]
fn actual_returned_proposals_are_staged_by_owner_after_child_return() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    for (variant, amount, legs, calls) in [(8, 7, 1, 1), (9, 7, 1, 2), (10, 7, 2, 2), (11, 8, 8, 8)]
    {
        let (result, storage) = actual_guest(variant);
        let record = result.assert_success("owner-mediated real guest succeeds");
        let layerx_programs_runtime::V2ActivityOutcome::Success { effects, .. } = record.outcome()
        else {
            panic!("owner-mediated guest did not succeed");
        };
        assert_eq!(effects.transfers.len(), legs);
        assert_eq!(
            effects.transfers.iter().map(|leg| leg.amount).sum::<u128>(),
            amount
        );
        assert_eq!(effects.calls.len(), calls);
        assert_eq!(storage, Storage::default());
    }
}

#[test]
fn actual_late_returned_proposal_cannot_escape_atomic_owner_budget() {
    let (result, storage) = actual_guest(12);
    assert_eq!(
        result,
        Err(ExecutionError::Composition(CompositionRefusal::Authority(
            AbiError::CapabilityDenied
        )))
    );
    assert_eq!(storage, Storage::default());
}

#[test]
fn actual_native_owner_budget_denial_has_authenticated_failure_terminal() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    use layerx_programs_runtime::terminal::{
        decode_terminal_payload, CandidateTerminalOutcome, ExecutionTerminal, TerminalDetail,
    };
    let directory = std::path::PathBuf::from(
        std::env::var("PAXEER_X_SPEND_RESULTS")
            .assert_success("actual native signed receipt terminal evidence is required"),
    );
    let encoded = std::fs::read(directory.join("case12.terminal.bin"))
        .assert_success("actual owner budget denial terminal");
    let decoded =
        decode_terminal_payload(2, 2, &encoded).assert_success("canonical failure terminal");
    let TerminalDetail::Execution(ExecutionTerminal::CandidateV4 {
        outcome: CandidateTerminalOutcome::Failure(failure),
        ..
    }) = decoded.detail
    else {
        panic!("actual native terminal has the wrong outcome");
    };
    let mut owner = [0; 32];
    owner[0] = 0x61;
    assert_eq!(
        failure.program(),
        ProgramId::new(owner).assert_success("actual owner frame")
    );
    assert_eq!(
        failure.class(),
        layerx_programs_runtime::RefusalClass::Unauthorized
    );
    assert_eq!(
        failure.reason().bytes(),
        b"LXP/programs/authority-refusal/v1\0\x04"
    );
}
