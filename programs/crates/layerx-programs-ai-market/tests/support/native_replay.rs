use layerx_ai_market_test_values::RequiredValue as _;
use layerx_programs_ai_market::{codec, registry_ops::CallContext, SHARED_STATE_KEY};
use layerx_programs_runtime::{
    self as rt, abi::UnavailableReceiptOracle, ActivityBudgetBinding, AuthorizationContext,
    AuthorizedExecutionRequest, BudgetedAuthorizedExecutionRequest, Capability, CapabilitySet,
    CompositionContext, DeclaredBudget, Executor, FuelSchedule, Storage, StorageNamespace,
    WasmEngine, ABI_V5_VERSION, CALL_ENTRY_EXPORT,
};
use std::{
    fs,
    io::{BufRead, Write},
    path::PathBuf,
};

pub fn prepare_native_call(
    input: &mut impl Write,
    output: &mut impl BufRead,
    height: u64,
) -> std::io::Result<()> {
    writeln!(input, "advance {height}")?;
    input.flush()?;
    let mut line = String::new();
    output.read_line(&mut line)?;
    if line.trim() != format!("advanced {height}") {
        return Err(std::io::Error::other(format!(
            "native occupancy advance: {line}"
        )));
    }
    Ok(())
}

pub fn assert_native_call(ctx: &CallContext, current: Option<&[u8]>, envelope: &[u8]) {
    let manifest = std::env::var_os("PAXAI_HOST_BOUNDARY_MANIFEST").map_or_else(
        || {
            PathBuf::from(env!("CARGO_MANIFEST_DIR"))
                .join("../../../build/paxai-host-boundary/manifest")
        },
        PathBuf::from,
    );
    let text = fs::read_to_string(&manifest).required_with("built guest manifest");
    let lines: Vec<_> = text.lines().collect();
    assert_eq!(lines.len(), 3, "{}", manifest.display());
    let artifact = PathBuf::from(lines[0]);
    let artifact = if artifact.is_absolute() {
        artifact
    } else {
        manifest
            .parent()
            .required_with("manifest directory")
            .join(artifact)
    };
    let wasm = fs::read(artifact).required_with("built guest");
    let module = WasmEngine::declared()
        .required()
        .validate_versioned_metered(ABI_V5_VERSION, &wasm, FuelSchedule::WASMI_0_31_2)
        .required();
    let program = rt::ProgramId::new(ctx.program.bytes()).required();
    let actor = rt::PrincipalId::new(ctx.principal.bytes()).required();
    let namespace = StorageNamespace::shared(program);
    let mut storage = Storage::new();
    if let Some(bytes) = current {
        let mut seed = storage.transaction(namespace);
        seed.write(SHARED_STATE_KEY, bytes).required();
        let _ = seed.commit();
    }
    let capabilities = CapabilitySet::new([
        Capability::SharedStorageRead,
        Capability::SharedStorageWrite,
        Capability::EmitEvent,
    ])
    .required();
    let executor = Executor::declared();
    let binding = ActivityBudgetBinding::new(
        codec::decode_envelope(envelope)
            .required()
            .request_digest()
            .required()
            .bytes(),
    )
    .required();
    let budget = DeclaredBudget::new(
        1_000_000, 16_777_216, 1_048_576, 1_048_576, 64, 1_048_576, 4096,
    )
    .required();
    let admitted = executor
        .admit_activity_budget_for_qualification(budget, actor, binding, u128::MAX)
        .required();
    let request = BudgetedAuthorizedExecutionRequest::new(
        AuthorizedExecutionRequest {
            module: &module,
            program,
            authorization: AuthorizationContext::new(actor, capabilities),
            receipts: &UnavailableReceiptOracle,
            entrypoint: CALL_ENTRY_EXPORT,
            calldata: envelope,
            composition: CompositionContext::isolated(),
            response_capacity: 16_384,
        },
        admitted,
        actor,
        binding,
    );
    // This is the public authenticated seam into execute_authorized_v2_budgeted's
    // execution implementation, preserving the manifest's ABI 5 and native payer.
    let result = executor.execute_authorized_authenticated(&mut storage, request, 1, ctx.height);
    assert!(
        result.is_ok(),
        "ordinary native CALL at height {}: ExecutionError {result:?}",
        ctx.height
    );
    assert!(result.required().replay_record().is_none());
}

/// Paid rent of this actor, projected from the canonical receipt attachment. Total
/// accrued rent can belong to another payer and can include unpaid arrears.
pub fn occupancy_fee(
    kind: u8,
    abi: u16,
    terminal: &[u8],
    actor: [u8; 32],
    accrued: u64,
) -> std::io::Result<u64> {
    use layerx_programs_runtime::{
        occupancy::OccupancySettlement,
        terminal::{decode_terminal_payload, TerminalAttachment, PRE_RUNTIME_FAILURE},
    };
    let malformed = |error| std::io::Error::other(format!("native occupancy: {error:?}"));
    let detail = if terminal.starts_with(PRE_RUNTIME_FAILURE) {
        terminal
    } else {
        layerx_wire::receipt::decode_applied_terminal(terminal)
            .map_err(|error| std::io::Error::other(format!("applied terminal: {error:?}")))?
            .0
    };
    let decoded = decode_terminal_payload(kind, abi, detail).map_err(malformed)?;
    let mut total = 0u128;
    let mut paid = 0u128;
    for attachment in decoded.attachments {
        if let TerminalAttachment::Occupancy(bytes) = attachment {
            let settlement = OccupancySettlement::canonical_decode(&bytes)
                .map_err(|error| std::io::Error::other(format!("occupancy evidence: {error:?}")))?;
            total = total
                .checked_add(settlement.usage().fee_units)
                .ok_or_else(|| std::io::Error::other("occupancy total overflow"))?;
            assert_eq!(
                settlement.usage().arrears_fee_units,
                0,
                "native fixture rent must be paid under its signed mandate"
            );
            for charge in settlement.charges() {
                if charge.paid() && charge.payer().bytes() == actor {
                    paid = paid
                        .checked_add(charge.amount_due())
                        .ok_or_else(|| std::io::Error::other("occupancy actor overflow"))?;
                }
            }
        }
    }
    assert_eq!(
        total,
        u128::from(accrued),
        "receipt occupancy total must match its canonical evidence"
    );
    u64::try_from(paid)
        .map_err(|error| std::io::Error::other(format!("occupancy actor amount: {error}")))
}
