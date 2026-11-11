use super::*;
use ed25519_dalek::SigningKey;

pub(super) fn peer(case: &Case) -> layerx_agentd::human::HumanPeer {
    let owner = require_some(case.value["did"].as_str(), "required test value").to_owned();
    layerx_agentd::human::HumanPeer {
        uid: 1000,
        principal: "operator".into(),
        tenant: case.tenant().as_str().into(),
        subject: Some(layerx_agentd::human::HumanSubject {
            transport_tenant: "native-budget-peer".into(),
            transport_principal: "unix-owner".into(),
            account: format!("agent:{owner}:main"),
            owner,
            asset: case.id("fee_asset"),
            registration: None,
        }),
    }
}

pub(super) fn retain(
    case: &Case,
    sources: &VerifiedResolvedProgramCharges,
    native: &EvmPrepared,
    submission: &VerifiedSubmission,
    registry: &ModuleRegistry,
) -> PathBuf {
    use std::os::unix::fs::PermissionsExt;
    let root = temporary(require_some(
        case.value["store"].as_str(),
        "corpus store producer path",
    ));
    let mut store = require_ok(Store::open(&root), "required test value");
    let budget = require_ok(BudgetLimiter::new(vec![]), "required test value");
    let configs = sources
        .charges()
        .iter()
        .enumerate()
        .map(|(index, _)| LimitConfig {
            id: LimitId([require_ok(u8::try_from(index), "test value fits u8") + 1; 16]),
            name: format!("native debit {index}"),
            scope: LimitScope::Tenant(sources.actor()),
            ceiling: 1_000_000,
            consumed: 0,
        })
        .collect::<Vec<_>>();
    let installed = require_ok(
        install_enrolment_limits(&mut store, &budget, &case.tenant(), &configs),
        "required test value",
    );
    let allocations = require_ok(
        sources.budget_allocations(|charge| {
            let index = require_some(
                sources.charges().iter().position(|value| value == charge),
                "required test value",
            );
            Ok(vec![installed[index].limit_id])
        }),
        "required test value",
    );
    let path = root.join("denominations.json");
    let limits = sources.charges().iter().zip(&installed).map(|(charge,limit)| json!({
        "stable_id":hex::encode(limit.stable_id),"asset":hex::encode(charge.asset),"source":hex::encode(charge.source.account())
    })).collect::<Vec<_>>();
    require_ok(
        std::fs::write(
            &path,
            require_ok(
                serde_json::to_vec(
                    &json!({"version":1,"tenant":case.tenant().as_str(),"limits":limits}),
                ),
                "required test value",
            ),
        ),
        "required test value",
    );
    require_ok(
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o600)),
        "required test value",
    );
    let verified = require_ok(
        layerx_agentd::config::load_program_budget_denominations(&path, &case.tenant()),
        "required test value",
    );
    finish_retain_2_budget_native_capture(FinishRetain2BudgetNativeCaptureContext {
        case,
        native,
        submission,
        registry,
        root,
        store,
        budget,
        allocations,
        verified,
    })
}

struct FinishRetain2BudgetNativeCaptureContext<'a> {
    case: &'a Case,
    native: &'a layerx_agentd::prepare::Prepared,
    submission: &'a VerifiedSubmission,
    registry: &'a ModuleRegistry,
    root: PathBuf,
    store: Store,
    budget: BudgetLimiter,
    allocations: VerifiedProgramBudgetAllocations,
    verified: layerx_agentd::enrolment::VerifiedProgramBudgetDenominations,
}

fn finish_retain_2_budget_native_capture(
    context: FinishRetain2BudgetNativeCaptureContext<'_>,
) -> PathBuf {
    use layerx_agentd::approval::native_effect::NativeEffectApprovalCarrier as Effect;
    use layerx_agentd::approval::native_program::NativeProgramApprovalCarrier as Program;

    let FinishRetain2BudgetNativeCaptureContext {
        case,
        native,
        submission,
        registry,
        root,
        mut store,
        budget,
        allocations,
        verified,
    } = context;

    require_ok(
        install_program_enrolment_denominations(&mut store, &budget, &verified),
        "required test value",
    );
    let now = require_some(case.value["now_ms"].as_u64(), "required test value");
    let request = ProgramReservationRequest {
        id: allocations.preparation_digest(),
        charges: allocations.charges().to_vec(),
        expiry_sequence: 18,
        current_sequence: 8,
        core_deadline: Some(CoreTimestampMs(now + 30_000)),
    };
    let staged = require_ok(
        budget.stage_program_allocation(&request, &allocations, CoreTimestampMs(now)),
        "required test value",
    );
    let public = SigningKey::from_bytes(&[8; 32]).verifying_key().to_bytes();
    let keys = [SequencerAuthorization::new([1; 32], public, 1, 100)];
    let peer = peer(case);
    let id = allocations.preparation_digest();
    let mut outbox = layerx_agentd::outbox::Outbox::default();
    if native.call.to == PrecompileTarget::Programs.address() {
        let held = require_ok(
            Program::hold_call(&mut store, &peer, [3; 32], 1, native, staged, now, &keys),
            "production lifecycle approval hold",
        );
        finish_retain_1_budget_native_capture(FinishRetain1BudgetNativeCaptureContext {
            submission,
            registry,
            store: &mut store,
            budget: &budget,
            now,
            peer: &peer,
            id,
            outbox: &mut outbox,
            held,
        });
    } else {
        let held = require_ok(
            Effect::hold_call(&mut store, &peer, [3; 32], 1, native, staged, now, &keys),
            "production burn approval hold",
        );
        require_ok(
            Effect::decide_call(
                &mut store,
                &budget,
                &peer,
                id,
                require_ok(held.call_held_digest(), "required test value"),
                [4; 32],
                true,
                8,
                now + 1,
            ),
            "required test value",
        );
        require_ok(
            Effect::queue_call(
                &mut store,
                &mut outbox,
                &peer,
                id,
                submission,
                registry,
                8,
                now + 2,
            ),
            "production native effect queue",
        );
    }
    complete_finish_retain_2_budget_native_capture_1_budget_native_capture(
        CompleteFinishRetain2BudgetNativeCapture1BudgetNativeCaptureContext {
            case,
            submission,
            root,
            store,
            budget,
            outbox,
        },
    )
}

struct CompleteFinishRetain2BudgetNativeCapture1BudgetNativeCaptureContext<'a> {
    case: &'a Case,
    submission: &'a VerifiedSubmission,
    root: PathBuf,
    store: Store,
    budget: BudgetLimiter,
    outbox: layerx_agentd::outbox::Outbox,
}

fn complete_finish_retain_2_budget_native_capture_1_budget_native_capture(
    context: CompleteFinishRetain2BudgetNativeCapture1BudgetNativeCaptureContext<'_>,
) -> PathBuf {
    let CompleteFinishRetain2BudgetNativeCapture1BudgetNativeCaptureContext {
        case,
        submission,
        root,
        store,
        budget,
        outbox,
    } = context;

    assert_eq!(
        require_ok(
            outbox.bytes_for_transmission(submission.activity_id()),
            "required test value"
        ),
        submission.exact_bytes()
    );
    assert_eq!(
        require_ok(
            case.endpoint.send_raw_transaction(submission.exact_bytes()),
            "submit exact producer bytes to loopback peer"
        ),
        submission.activity_id()
    );
    drop(store);
    drop(budget);
    drop(outbox);
    root
}

struct FinishRetain1BudgetNativeCaptureContext<'a> {
    submission: &'a VerifiedSubmission,
    registry: &'a ModuleRegistry,
    store: &'a mut Store,
    budget: &'a BudgetLimiter,
    now: u64,
    peer: &'a layerx_agentd::human::HumanPeer,
    id: [u8; 32],
    outbox: &'a mut layerx_agentd::outbox::Outbox,
    held: layerx_agentd::approval::native_program::NativeProgramApprovalCarrier,
}

fn finish_retain_1_budget_native_capture(context: FinishRetain1BudgetNativeCaptureContext<'_>) {
    use layerx_agentd::approval::native_program::NativeProgramApprovalCarrier as Program;

    let FinishRetain1BudgetNativeCaptureContext {
        submission,
        registry,
        store,
        budget,
        now,
        peer,
        id,
        outbox,
        held,
    } = context;

    require_ok(
        Program::decide_call(
            store,
            budget,
            peer,
            id,
            require_ok(held.call_held_digest(), "required test value"),
            [4; 32],
            true,
            8,
            now + 1,
        ),
        "required test value",
    );
    require_ok(
        Program::queue_call(store, outbox, peer, id, submission, registry, 8, now + 2),
        "production native program queue",
    );
}
