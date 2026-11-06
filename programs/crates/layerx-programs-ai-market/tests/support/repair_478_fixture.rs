use layerx_programs_ai_market::{
    codec::{self, Envelope},
    dispatch::{self, Buffers, Routed},
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    registry::derive_rewards_account,
    registry_ops::CallContext,
    rewards::{FundRequest, FUNDING_POLICY_VERSION},
    types::*,
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
    let mut fund_payload = vec![0; 57];
    let n = FundRequest {
        amount: AMOUNT,
        refund_recipient: AccountId::new(REFUND)?,
        policy_version: FUNDING_POLICY_VERSION,
        consent: true,
    }
    .encode(&mut fund_payload)?;
    fund_payload.truncate(n);
    let envelope = |operation, sequence, payload: &[u8]| -> CodecResult<Vec<u8>> {
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
    };
    let create = envelope(dispatch::CREATE, 1, &create_payload)?;
    let fund = envelope(dispatch::FUND, 2, &fund_payload)?;
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

#[cfg(test)]
#[test]
fn fund_joint_sections_match_the_carried_transition_and_replay() {
    use layerx_programs_ai_market::{
        epoch, rewards,
        state::{decode_shared_state, Section},
    };
    let owner = [12; 32];
    let f = fixture(owner, [13; 32]).unwrap();
    let ctx = CallContext {
        chain: ChainDomain::new(MERGED_CHAIN_DOMAIN).unwrap(),
        program: ProgramId::new(PROGRAM).unwrap(),
        principal: PrincipalId::new(owner).unwrap(),
        height: HEIGHT,
    };
    let envelope = codec::decode_envelope(&f.fund).unwrap();
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; rewards::SCRATCH_BYTES];
    let mut carry = vec![0; epoch::CARRY_SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let (outcome, written) =
        epoch::carry_reputation(&f.current, &mut next, &mut carry, |current, next| {
            let outcome = rewards::apply(&ctx, &envelope, current, next, &mut scratch, &mut event)?;
            let rewards::Outcome::Applied { state_len, .. } = outcome else {
                panic!("first Fund must apply");
            };
            Ok((outcome, Some(state_len)))
        })
        .unwrap();
    assert_eq!(&next[..written.unwrap()], f.next);
    let rewards::Outcome::Applied {
        event_len,
        response,
        ..
    } = outcome
    else {
        panic!("first Fund must apply");
    };
    assert_eq!(&event[..event_len], f.event);
    assert_eq!(
        response.as_bytes(),
        codec::decode_result(&f.result).unwrap().payload
    );
    let before = decode_shared_state(&f.current).unwrap();
    let after = decode_shared_state(&f.next).unwrap();
    let joint = Section::ReputationAdmission.index();
    assert_eq!(&before.feature_sections[joint][..4], b"RP07");
    for section in [
        Section::IdentityRoster,
        Section::CurrentReports,
        Section::ReputationAdmission,
    ] {
        assert_eq!(
            before.feature_sections[section.index()],
            after.feature_sections[section.index()]
        );
    }
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    next.fill(0xa5);
    let retry = dispatch::route(
        &ctx,
        &f.fund,
        Some(&f.next),
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )
    .unwrap();
    let Routed::Unchanged { result_len } = retry else {
        panic!("Fund replay must return the retained result");
    };
    assert_eq!(
        codec::decode_result(&result[..result_len])
            .unwrap()
            .revision,
        after.revision
    );
    assert!(next.iter().all(|byte| *byte == 0xa5));

    let mut corrupt_joint = before.feature_sections[joint].to_vec();
    corrupt_joint[4] ^= 0x80;
    let corrupt_state = before
        .replace_section(Section::ReputationAdmission, &corrupt_joint)
        .unwrap();
    let mut corrupt = vec![0; MAX_STATE_BYTES];
    let mut control = vec![0; Section::Control.payload_cap()];
    let len = layerx_programs_ai_market::state::encode_shared_state(
        &corrupt_state,
        &mut corrupt,
        &mut control,
    )
    .unwrap();
    let refused = dispatch::route(
        &ctx,
        &f.fund,
        Some(&corrupt[..len]),
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )
    .unwrap();
    let Routed::Refused { result_len } = refused else {
        panic!("Fund must validate the joint reputation region");
    };
    assert!(codec::decode_result(&result[..result_len])
        .unwrap()
        .error
        .is_some());
    assert!(next.iter().all(|byte| *byte == 0xa5));
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

#[cfg(test)]
#[test]
fn fund_entry_gas_for_committed_receipt_fixture_is_bounded() {
    measured_fund_cpu_fuel();
}

#[cfg(test)]
pub fn measured_fund_cpu_fuel() -> u64 {
    use layerx_programs_runtime::terminal::{
        CandidateTerminalOutcome, ExecutionTerminal, TerminalDetail,
    };
    let dir = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("../../target/repair-478");
    let read = |name| {
        std::fs::read(dir.join(name)).unwrap_or_else(|error| {
            panic!("{name}: {error}; run the declared Go fixture gate first")
        })
    };
    let owner: [u8; 32] = read("owner").try_into().unwrap();
    let asset: [u8; 32] = read("asset").try_into().unwrap();
    let f = fixture(owner, asset).unwrap();
    assert_eq!(f.create, read("create"));
    assert_eq!(f.fund, read("fund"));
    assert_eq!(f.current, read("current"));
    assert_eq!(f.next, read("next"));
    let terminal = read("terminal");
    let decoded = decode_kernel_terminal(1, &terminal).unwrap();
    let TerminalDetail::Execution(ExecutionTerminal::CandidateV4 {
        program,
        abi_version,
        usage,
        outcome,
        ..
    }) = decoded.detail
    else {
        panic!("expected ABI5 execution");
    };
    assert_eq!(program, PROGRAM);
    assert_eq!(abi_version, 5);
    assert_eq!(
        outcome,
        CandidateTerminalOutcome::Success {
            code: 0,
            response: f.result
        }
    );
    assert!(usage.cpu_fuel < 999_968);
    assert!(usage.cpu_fuel > 0);
    assert!(usage.cpu_fuel <= layerx_programs_runtime::meter::DEFAULT_CPU_FUEL);
    assert!(usage.cpu_fuel < 10_000_000);
    let shared_state_key_bytes =
        u64::try_from(layerx_programs_ai_market::SHARED_STATE_KEY.len()).unwrap();
    let shared_state_value_bytes = u64::try_from(f.current.len()).unwrap();
    // host/balance.rs charges the u128 sight balance returned by balance_read.
    let rewards_balance_read_bytes = u64::try_from(core::mem::size_of::<u128>()).unwrap();
    assert_eq!(shared_state_key_bytes, 14);
    assert_eq!(rewards_balance_read_bytes, 16);
    assert_eq!(
        usage.storage_read_bytes,
        shared_state_key_bytes + shared_state_value_bytes + rewards_balance_read_bytes
    );
    assert_eq!(
        usage.storage_write_bytes,
        shared_state_key_bytes + u64::try_from(f.next.len()).unwrap()
    );
    println!(
        "Fund committed fixture CPU fuel: {} (kernel allowance 10000000)",
        usage.cpu_fuel
    );
    usage.cpu_fuel
}

/// A diagnostic includes the frozen application registry name, not just its code.
pub fn application_error(frame: &[u8]) -> String {
    match codec::decode_result(frame) {
        Ok(result) => match result.error {
            Some(error) => {
                let entry = layerx_programs_ai_market::errors::APPLICATION_ERRORS
                    .iter()
                    .find(|entry| entry.code == error.code())
                    .expect("decoded application error is registered");
                format!("{}::{} ({:04x})", entry.feature, entry.name, error.code())
            }
            None => format!("{:?}", result.status),
        },
        Err(error) => format!("invalid application result: {error:?}"),
    }
}

/// Core balance facts transported from the preceding real kernel execution.
/// No balances or receipt identities are synthesized by the Rust harness.
struct KernelBalanceEvidence {
    view: layerx_programs_runtime::BalanceView,
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
        if (account, asset, receipt)
            != (self.view.account, self.view.asset, self.view.receipt_digest)
        {
            return Err(layerx_programs_runtime::AbiError::ReceiptMismatch);
        }
        Ok(self.view.clone())
    }
}

/// Independently executes the exact kernel fixture on its candidate guest under
/// Executor::declared and DeclaredBudget::protocol_maximum. The Go producer writes
/// these inputs before Fund, so a guest refusal can be diagnosed too. This proves
/// CPU completion and staged effects; it applies no native ledger leg.
pub fn execute_kernel_fund(dir: &std::path::Path) -> Result<(u64, Result<(), String>), String> {
    use layerx_programs_runtime::{self as rt, *};
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
    let sight = read("sight")?;
    if sight.len() != 152 {
        return Err("kernel balance observation length".into());
    }
    let array = |at| <[u8; 32]>::try_from(&sight[at..at + 32]).expect("fixed sight field");
    let view = BalanceView {
        account: array(0),
        asset: array(32),
        balance: u128::from_be_bytes(sight[64..80].try_into().unwrap()),
        receipt_digest: array(80),
        state_root: array(112),
        observed_sequence: u64::from_be_bytes(sight[144..152].try_into().unwrap()),
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
        || view.account != f.rewards
    {
        return Err("kernel balance observation binding".into());
    }
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
    let caps = CapabilitySet::new([
        Capability::EmitEvent,
        Capability::SharedStorageRead,
        Capability::SharedStorageWrite,
        Capability::Transfer402 {
            asset,
            to: f.rewards,
            maximum_amount: AMOUNT,
        },
        Capability::BalanceView {
            account: f.rewards,
            asset,
            receipt_digest: digest,
        },
    ])
    .map_err(|e| e.to_string())?;
    let declaration = AccessDeclaration::absent();
    let canonical_call = rt::native_contract::CanonicalCall {
        program,
        abi: ABI_V5_VERSION,
        entrypoint: CALL_ENTRY_EXPORT,
        calldata: &f.fund,
        capabilities: &caps,
        declaration: &declaration,
        response_capacity: u32::try_from(MAX_RESULT_BYTES).unwrap(),
        budget: DeclaredBudget::protocol_maximum().resource_budget(),
    }
    .canonical_bytes(None)
    .map_err(|e| format!("canonical Fund CALL: {e:?}"))?;
    if read("call")? != canonical_call {
        return Err("kernel Fund CALL differs from executor profile, budget or inputs".into());
    }
    let executor = Executor::declared();
    let binding = ActivityBudgetBinding::new(
        codec::decode_envelope(&f.fund)
            .map_err(|e| e.to_string())?
            .request_digest()
            .map_err(|e| e.to_string())?
            .bytes(),
    )
    .map_err(|e| e.to_string())?;
    let admitted = executor
        .admit_activity_budget_for_qualification(
            DeclaredBudget::protocol_maximum(),
            actor,
            binding,
            u128::MAX,
        )
        .map_err(|e| e.to_string())?;
    let oracle = KernelBalanceEvidence { view };
    let request = BudgetedAuthorizedExecutionRequest::new(
        AuthorizedExecutionRequest {
            module: &module,
            program,
            authorization: AuthorizationContext::new(actor, caps),
            receipts: &oracle,
            entrypoint: CALL_ENTRY_EXPORT,
            calldata: &f.fund,
            composition: CompositionContext::isolated(),
            response_capacity: MAX_RESULT_BYTES,
        },
        admitted,
        actor,
        binding,
    );
    let record = executor
        .execute_authorized_v5_budgeted_for_qualification(
            &mut storage,
            request,
            prior.global_sequence() + 1,
            HEIGHT + 1,
        )
        .map_err(|e| format!("Fund ExecutionError: {e:?}"))?;
    let fuel = record.execution().usage().cpu_fuel;
    let verdict = match record.outcome() {
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
    };
    println!("Fund executor CPU fuel={fuel} verdict={verdict:?}");
    Ok((fuel, verdict))
}
