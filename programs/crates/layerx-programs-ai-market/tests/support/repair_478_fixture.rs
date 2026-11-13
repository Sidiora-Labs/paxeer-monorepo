use layerx_programs_ai_market::{
    codec::{self, Envelope},
    dispatch::{self, Buffers, Routed},
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    registry::derive_rewards_account,
    registry_ops::CallContext,
    rewards::{FundRequest, FUNDING_POLICY_VERSION},
    types::{
        AccountId, AssetId, Authentication, ChainDomain, Digest32, Presence, PrincipalId,
        ProgramId, RequestId, RubricDigest,
    },
    CodecResult, MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
    MERGED_CHAIN_DOMAIN,
};

pub const HEIGHT: u64 = 1002;
pub const AMOUNT: u128 = 100;
pub const PROGRAM: [u8; 32] = [11; 32];
pub const REFUND: [u8; 32] = [14; 32];

pub struct Fixture {
    pub create: Vec<u8>,
    pub fund: Vec<u8>,
    pub current: Vec<u8>,
    pub next: Vec<u8>,
    pub result: Vec<u8>,
    pub event: Vec<u8>,
    pub rewards: [u8; 32],
}

pub fn fixture(owner: [u8; 32], asset: [u8; 32]) -> CodecResult<Fixture> {
    let ctx = CallContext {
        chain: ChainDomain::new(MERGED_CHAIN_DOMAIN)?,
        program: ProgramId::new(PROGRAM)?,
        principal: PrincipalId::new(owner)?,
        height: HEIGHT,
    };
    let rewards = derive_rewards_account(ctx.program, AssetId::new(asset)?)?;
    let create_payload = fixture_create_payload(owner, asset, rewards)?;
    let mut fund_payload = vec![0; 57];
    let n = FundRequest {
        amount: AMOUNT,
        refund_recipient: AccountId::new(REFUND)?,
        policy_version: FUNDING_POLICY_VERSION,
        consent: true,
    }
    .encode(&mut fund_payload)?;
    fund_payload.truncate(n);
    let create = fixture_envelope(&ctx, dispatch::CREATE, 1, &create_payload)?;
    let fund = fixture_envelope(&ctx, dispatch::FUND, 2, &fund_payload)?;
    let mut current = vec![0; MAX_STATE_BYTES];
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    let Routed::Applied { state_len, .. } = dispatch::route(
        &ctx,
        &create,
        None,
        Buffers {
            next: &mut current,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?
    else {
        return Err(layerx_programs_ai_market::errors::NON_CANONICAL);
    };
    current.truncate(state_len);
    let Routed::Applied {
        state_len,
        event_len,
        result_len,
        ..
    } = dispatch::route(
        &ctx,
        &fund,
        Some(&current),
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?
    else {
        return Err(layerx_programs_ai_market::errors::NON_CANONICAL);
    };
    next.truncate(state_len);
    if current == next {
        return Err(layerx_programs_ai_market::errors::NON_CANONICAL);
    }
    event.truncate(event_len);
    result.truncate(result_len);
    Ok(Fixture {
        create,
        fund,
        current,
        next,
        event,
        result,
        rewards: rewards.bytes(),
    })
}

/// Decodes the two canonical terminal forms the native kernel publishes.
/// Synthetic refusals have their own domain and carry no applied-legs wrapper.
pub fn decode_kernel_terminal(
    kind: u8,
    bytes: &[u8],
) -> Result<layerx_programs_runtime::terminal::DecodedTerminal, String> {
    use layerx_programs_runtime::terminal::{decode_terminal_payload, PRE_RUNTIME_FAILURE};
    let detail = if bytes.starts_with(PRE_RUNTIME_FAILURE) {
        bytes
    } else {
        layerx_wire::receipt::decode_applied_terminal(bytes)
            .map_err(|error| format!("canonical kernel applied-legs envelope: {error:?}"))?
            .0
    };
    decode_terminal_payload(kind, 5, detail)
        .map_err(|error| format!("canonical kernel terminal: {error:?}"))
}

/// A diagnostic includes the frozen application registry name, not just its code.
pub fn application_error(frame: &[u8]) -> String {
    match codec::decode_result(frame) {
        Ok(result) => match result.error {
            Some(error) => {
                let entry = layerx_programs_ai_market::errors::APPLICATION_ERRORS
                    .iter()
                    .find(|entry| entry.code == error.code())
                    .unwrap_or_else(|| panic!("decoded application error is registered"));
                format!("{}::{} ({:04x})", entry.feature, entry.name, error.code())
            }
            None => format!("{:?}", result.status),
        },
        Err(error) => format!("invalid application result: {error:?}"),
    }
}

/// Historical balance facts are transported from the real kernel execution.
/// The current guest has no sight grant or receipt authority.
struct KernelBalanceEvidence {
    view: Option<layerx_programs_runtime::BalanceView>,
}
impl layerx_programs_runtime::ReceiptOracle for KernelBalanceEvidence {
    fn verified_receipt(
        &self,
        _: [u8; 32],
    ) -> Result<layerx_programs_runtime::ReceiptView, layerx_programs_runtime::AbiError> {
        Err(layerx_programs_runtime::AbiError::ReceiptMismatch)
    }
    fn verified_balance(
        &self,
        account: [u8; 32],
        asset: [u8; 32],
        receipt: [u8; 32],
    ) -> Result<layerx_programs_runtime::BalanceView, layerx_programs_runtime::AbiError> {
        let view = self
            .view
            .as_ref()
            .ok_or(layerx_programs_runtime::AbiError::BalanceEvidenceUnavailable)?;
        if (account, asset, receipt) != (view.account, view.asset, view.receipt_digest) {
            return Err(layerx_programs_runtime::AbiError::ReceiptMismatch);
        }
        Ok(view.clone())
    }
}

/// Independently executes the exact kernel fixture on its candidate guest under
/// `Executor::declared` and `DeclaredBudget::protocol_maximum`. The Go producer writes
/// these inputs before Fund, so a guest refusal can be diagnosed too. This proves
/// CPU completion and staged effects; it applies no native ledger leg.
pub fn execute_kernel_fund(dir: &std::path::Path) -> Result<(u64, Result<(), String>), String> {
    execute_kernel_fund_with_budget(dir, None)
}

pub fn measure_kernel_fund(dir: &std::path::Path, allowance: u64) -> Result<u64, String> {
    let (fuel, verdict) = execute_kernel_fund_with_budget(dir, Some(allowance))?;
    verdict?;
    Ok(fuel)
}

fn execute_kernel_fund_with_budget(
    dir: &std::path::Path,
    diagnostic_allowance: Option<u64>,
) -> Result<(u64, Result<(), String>), String> {
    use layerx_programs_runtime::{
        self as rt, AccessDeclaration, ActivityBudgetBinding, AuthorizationContext,
        AuthorizedExecutionRequest, BudgetedAuthorizedExecutionRequest, CompositionContext,
        DeclaredBudget, Executor, FeeSchedule, FuelSchedule, Storage, StorageNamespace, WasmEngine,
        ABI_V5_VERSION, CALL_ENTRY_EXPORT,
    };
    let read = |name| std::fs::read(dir.join(name)).map_err(|e| format!("{name}: {e}"));
    let owner: [u8; 32] = read("owner")?.try_into().map_err(|_| "owner length")?;
    let asset: [u8; 32] = read("asset")?.try_into().map_err(|_| "asset length")?;
    let f = fixture(owner, asset).map_err(|e| format!("fixture: {e:?}"))?;
    for (name, bytes) in [
        ("current", &f.current),
        ("fund", &f.fund),
        ("next", &f.next),
    ] {
        if read(name)? != *bytes {
            return Err(format!("kernel {name} differs from producer"));
        }
    }
    let view = read_kernel_balance(dir, asset, f.rewards)?;
    let module = WasmEngine::declared()
        .map_err(|e| e.to_string())?
        .validate_versioned_metered(ABI_V5_VERSION, &read("wasm")?, FuelSchedule::WASMI_0_31_2)
        .map_err(|e| e.to_string())?;
    let program = rt::ProgramId::new(PROGRAM).map_err(|e| e.to_string())?;
    let actor = rt::PrincipalId::new(owner).map_err(|e| e.to_string())?;
    let namespace = StorageNamespace::shared(program);
    let mut storage = Storage::new();
    let mut seed = storage.transaction(namespace);
    seed.write(layerx_programs_ai_market::SHARED_STATE_KEY, &f.current)
        .map_err(|e| e.to_string())?;
    assert_eq!(seed.commit(), 1);
    let caps = fund_capabilities(asset, f.rewards, view.as_ref())?;
    let declaration = AccessDeclaration::absent();
    let budget = kernel_fund_budget(diagnostic_allowance);
    let canonical_call = rt::native_contract::CanonicalCall {
        program,
        abi: ABI_V5_VERSION,
        entrypoint: CALL_ENTRY_EXPORT,
        calldata: &f.fund,
        capabilities: &caps,
        declaration: &declaration,
        response_capacity: u32::try_from(MAX_RESULT_BYTES)
            .unwrap_or_else(|error| panic!("result capacity: {error:?}")),
        budget,
    }
    .canonical_bytes(None)
    .map_err(|e| format!("canonical Fund CALL: {e:?}"))?;
    if read("call")? != canonical_call {
        return Err("kernel Fund CALL differs from executor profile, budget or inputs".into());
    }
    let executor = Executor::new(budget, FeeSchedule::declared());
    let binding = ActivityBudgetBinding::new(
        codec::decode_envelope(&f.fund)
            .map_err(|e| e.to_string())?
            .request_digest()
            .map_err(|e| e.to_string())?
            .bytes(),
    )
    .map_err(|e| e.to_string())?;
    let oracle = KernelBalanceEvidence { view };
    let request = AuthorizedExecutionRequest {
        module: &module,
        program,
        authorization: AuthorizationContext::new(actor, caps),
        receipts: &oracle,
        entrypoint: CALL_ENTRY_EXPORT,
        calldata: &f.fund,
        composition: CompositionContext::isolated(),
        response_capacity: MAX_RESULT_BYTES,
    };
    let record = if diagnostic_allowance.is_some() {
        // The explicit-budget qualification interface meters the same guest and
        // transported kernel facts without admitting an over-cap native CALL.
        executor.execute_authorized_versioned_for_qualification(&mut storage, request)
    } else {
        let admitted = executor
            .admit_activity_budget_for_qualification(
                DeclaredBudget::protocol_maximum(),
                actor,
                binding,
                u128::MAX,
            )
            .map_err(|e| e.to_string())?;
        executor.execute_authorized_v5_budgeted_for_qualification(
            &mut storage,
            BudgetedAuthorizedExecutionRequest::new(request, admitted, actor, binding),
            u64::from_be_bytes(
                read("global-sequence")?
                    .try_into()
                    .map_err(|_| "kernel global sequence length")?,
            ),
            HEIGHT + 1,
        )
    }
    .map_err(|e| format!("Fund ExecutionError: {e:?}"))?;
    let fuel = record.execution().usage().cpu_fuel;
    let verdict = kernel_fund_verdict(record.outcome(), &mut storage, namespace, &f, asset, actor);
    println!("Fund executor CPU fuel={fuel} verdict={verdict:?}");
    Ok((fuel, verdict))
}

fn fixture_envelope(
    ctx: &CallContext,
    operation: dispatch::Operation,
    sequence: u64,
    payload: &[u8],
) -> CodecResult<Vec<u8>> {
    let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
    let n = codec::encode_envelope(
        &Envelope {
            operation,
            chain: ctx.chain,
            program: ctx.program,
            market: codec::derive_market(ctx.chain, ctx.program)?,
            actor: ctx.principal,
            epoch: 0,
            config: 1,
            roster: Presence::Absent,
            sequence,
            expiry: HEIGHT + 1000,
            request: RequestId::new(
                [u8::try_from(sequence)
                    .map_err(|_| layerx_programs_ai_market::errors::ARITHMETIC)?;
                    32],
            )?,
            payload,
            authentication: Authentication::Native,
        },
        &mut encoded,
    )?;
    encoded.truncate(n);
    Ok(encoded)
}

fn fixture_create_payload(
    owner: [u8; 32],
    asset: [u8; 32],
    rewards: AccountId,
) -> CodecResult<Vec<u8>> {
    let policy = TaskPolicyV1::bounded_default(
        1,
        1,
        PolicyCommitments {
            model_artifact: Digest32::new([1; 32])?,
            dataset_artifact: [2; 32],
            benchmark_suite: Digest32::new([3; 32])?,
            rubric: RubricDigest::new([4; 32])?,
            task_schema: Digest32::new([5; 32])?,
            result_schema: Digest32::new([6; 32])?,
            service_terms: Digest32::new([7; 32])?,
        },
        100,
        1,
    )?;
    let mut policy_bytes = vec![0; TASK_POLICY_BYTES];
    policy.encode(&mut policy_bytes)?;
    let mut create_payload = owner.to_vec();
    create_payload.extend_from_slice(&asset);
    create_payload.extend_from_slice(rewards.as_bytes());
    create_payload.extend_from_slice(&REFUND);
    create_payload.push(0);
    create_payload.extend_from_slice(&policy_bytes);
    create_payload.extend_from_slice(&[16; 32]);
    Ok(create_payload)
}

fn read_kernel_balance(
    dir: &std::path::Path,
    asset: [u8; 32],
    rewards: [u8; 32],
) -> Result<Option<layerx_programs_runtime::BalanceView>, String> {
    use layerx_programs_runtime::BalanceView;
    let read = |name| std::fs::read(dir.join(name)).map_err(|e| format!("{name}: {e}"));
    if dir.join("sight").exists() {
        let sight = read("sight")?;
        if sight.len() != 152 {
            return Err("kernel balance observation length".into());
        }
        let array = |at| {
            <[u8; 32]>::try_from(&sight[at..at + 32])
                .unwrap_or_else(|_| panic!("fixed sight field"))
        };
        let view = BalanceView {
            account: array(0),
            asset: array(32),
            balance: u128::from_be_bytes(
                sight[64..80]
                    .try_into()
                    .unwrap_or_else(|error| panic!("fixed sight field: {error:?}")),
            ),
            receipt_digest: array(80),
            state_root: array(112),
            observed_sequence: u64::from_be_bytes(
                sight[144..152]
                    .try_into()
                    .unwrap_or_else(|error| panic!("fixed sight field: {error:?}")),
            ),
        };
        let receipt = layerx_wire::receipt::decode(&read("prior-receipt")?)
            .map_err(|e| format!("kernel receipt: {e:?}"))?;
        let unsigned = layerx_wire::receipt::encode_unsigned(&receipt)
            .map_err(|e| format!("kernel unsigned receipt: {e:?}"))?;
        let digest = layerx_wire::hash::receipt_digest(&unsigned)
            .map_err(|e| format!("kernel receipt digest: {e:?}"))?;
        let prior = receipt.protocol().ok_or("protocol receipt required")?;
        if prior.result_code() != 0
            || digest != view.receipt_digest
            || prior.resulting_state_root() != view.state_root
            || prior.global_sequence() != view.observed_sequence
            || view.asset != asset
            || view.account != rewards
        {
            return Err("kernel balance observation binding".into());
        }
        Ok(Some(view))
    } else {
        Ok(None)
    }
}

fn kernel_fund_verdict(
    outcome: &layerx_programs_runtime::V2ActivityOutcome,
    storage: &mut layerx_programs_runtime::Storage,
    namespace: layerx_programs_runtime::StorageNamespace,
    f: &Fixture,
    asset: [u8; 32],
    actor: layerx_programs_runtime::PrincipalId,
) -> Result<(), String> {
    use layerx_programs_runtime::{TransferSource, V2ActivityOutcome};
    match outcome {
        V2ActivityOutcome::Success { response, effects } => {
            let stored = storage
                .transaction(namespace)
                .read(layerx_programs_ai_market::SHARED_STATE_KEY)
                .map_err(|e| e.to_string())?;
            let mut topic = [0; 64];
            let n = codec::event_topic(dispatch::FUND, &mut topic).map_err(|e| e.to_string())?;
            let transfers = &effects.transfers;
            if response.code != 0 || response.bytes != f.result {
                Err(format!(
                    "Fund result differs: {}",
                    application_error(&response.bytes)
                ))
            } else if stored.as_deref() != Some(f.next.as_slice()) {
                Err("Fund state differs".into())
            } else if effects.events.len() != 1
                || effects.events[0].topic != topic[..n]
                || effects.events[0].data != f.event
            {
                Err("Fund event differs".into())
            } else if transfers.len() != 1
                || transfers[0].asset != asset
                || transfers[0].to != f.rewards
                || transfers[0].amount != AMOUNT
                || !matches!(&transfers[0].source, TransferSource::ProgramFunding { principal, .. } if *principal == actor)
            {
                Err(format!("Fund transfer differs: {transfers:?}"))
            } else {
                Ok(())
            }
        }
        V2ActivityOutcome::Failure(failure) => Err(format!(
            "Fund application refusal: {}",
            application_error(failure.reason().bytes())
        )),
        V2ActivityOutcome::Resource(refusal) => Err(format!("Fund resource refusal: {refusal:?}")),
    }
}

fn kernel_fund_budget(
    diagnostic_allowance: Option<u64>,
) -> layerx_programs_runtime::ResourceBudget {
    use layerx_programs_runtime::{DeclaredBudget, ResourceBudget};
    let maximum = DeclaredBudget::protocol_maximum().resource_budget();
    match diagnostic_allowance {
        Some(cpu) => ResourceBudget::new_complete(
            cpu,
            maximum.memory_bytes(),
            maximum.storage_read_bytes(),
            maximum.storage_write_bytes(),
            maximum.output_values(),
            maximum.output_bytes(),
            maximum.table_elements(),
        ),
        None => maximum,
    }
}

fn fund_capabilities(
    asset: [u8; 32],
    rewards: [u8; 32],
    view: Option<&layerx_programs_runtime::BalanceView>,
) -> Result<layerx_programs_runtime::CapabilitySet, String> {
    use layerx_programs_runtime::{Capability, CapabilitySet};
    let mut granted = vec![
        Capability::EmitEvent,
        Capability::SharedStorageRead,
        Capability::SharedStorageWrite,
        Capability::Transfer402 {
            asset,
            to: rewards,
            maximum_amount: AMOUNT,
        },
    ];
    if let Some(view) = view {
        granted.push(Capability::BalanceView {
            account: rewards,
            asset,
            receipt_digest: view.receipt_digest,
        });
    }
    CapabilitySet::new(granted).map_err(|e| e.to_string())
}
