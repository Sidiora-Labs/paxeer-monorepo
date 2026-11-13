use layerx_ai_market_test_values::RequiredValue as _;
use layerx_programs_ai_market::{
    codec, dispatch, registry_ops::CallContext, rewards, state, MAX_RESULT_BYTES,
    REWARDS_ACCOUNT_SEED, SHARED_STATE_KEY,
};
use layerx_programs_runtime::{
    self as rt, ActivityBudgetBinding, AuthorizationContext, AuthorizedExecutionRequest,
    BudgetedAuthorizedExecutionRequest, Capability, CapabilitySet, CompositionContext,
    DeclaredBudget, Executor, FuelSchedule, Storage, StorageNamespace, V2ActivityOutcome,
    ValidatedModule, WasmEngine, ABI_V5_VERSION, CALL_ENTRY_EXPORT,
};
use std::{collections::BTreeMap, path::Path, sync::Arc};

pub type Checked<T = ()> = Result<T, Box<dyn std::error::Error>>;
pub const DEPLOYMENT_ENVELOPE_BYTES: usize = 104;
pub const DEPLOYMENT_LIMIT: usize = 524_288;
pub const BUILD_OPT_LEVEL: &str = "z";
pub const BUILD_POSTPROCESS: &str =
    "wasm-opt -Oz --flatten --rereloop -Oz --converge --merge-similar-functions -Oz --converge --strip-debug --strip-producers";
fn assert_build_recipe() {
    let recipe = include_str!("../fixtures/guest-recipe.sh");
    assert!(recipe
        .lines()
        .any(|line| line == format!("PAXAI_GUEST_OPT_LEVEL={BUILD_OPT_LEVEL}")));
    let passes = recipe
        .split_once("PAXAI_GUEST_PASSES=(\n")
        .required()
        .1
        .split_once("\n)")
        .required()
        .0
        .split_whitespace()
        .collect::<Vec<_>>()
        .join(" ");
    assert_eq!(BUILD_POSTPROCESS, format!("wasm-opt {passes}"));
}

pub const CPU_FUEL: u64 = DeclaredBudget::protocol_maximum().cpu_fuel();

/// Isolated measurements of the original readers and byte-copy primitive.
/// This artifact has a diagnostic entrypoint, not a deployment entrypoint.
pub fn aggregation_baseline_module() -> Checked<Arc<ValidatedModule>> {
    assert_build_recipe();
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures");
    let hex = std::fs::read_to_string(root.join("ai-market-aggregation-baseline.hex"))?;
    let hex = hex.trim();
    assert_eq!(hex.len() % 2, 0);
    let wasm = (0..hex.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&hex[i..i + 2], 16))
        .collect::<Result<Vec<_>, _>>()?;
    let metadata = std::fs::read_to_string(root.join("ai-market-aggregation-baseline.kvx"))?;
    assert!(metadata
        .lines()
        .any(|line| line == "purpose = \"aggregation-pass-measurement\""));
    assert!(metadata.lines().any(|line| line == "opt_level = \"z\""));
    let source = source_digest()?;
    assert!(metadata
        .lines()
        .any(|line| line == format!("source_sha256 = \"{source}\"")));
    assert!(metadata
        .lines()
        .any(|line| line == format!("wasm_bytes = {}", wasm.len())));
    assert!(metadata
        .lines()
        .any(|line| line == format!("postprocess = \"{BUILD_POSTPROCESS}\"")));
    let hash = codec::sha256([wasm.as_slice()])
        .map_err(|e| format!("{e:?}"))?
        .iter()
        .fold(String::new(), |mut text, byte| {
            use core::fmt::Write as _;
            write!(text, "{byte:02x}").unwrap_or_else(|error| panic!("hex formatting: {error}"));
            text
        });
    assert!(metadata
        .lines()
        .any(|line| line == format!("sha256 = \"{hash}\"")));
    assert!(wasm.len() + DEPLOYMENT_ENVELOPE_BYTES <= DEPLOYMENT_LIMIT);
    Ok(Arc::new(
        WasmEngine::declared()
            .map_err(|e| format!("{e:?}"))?
            .validate_versioned_metered(ABI_V5_VERSION, &wasm, FuelSchedule::WASMI_0_31_2)
            .map_err(|e| format!("{e:?}"))?,
    ))
}

fn source_digest() -> Checked<String> {
    fn sources(dir: &Path, files: &mut Vec<std::path::PathBuf>) -> std::io::Result<()> {
        for entry in std::fs::read_dir(dir)? {
            let path = entry?.path();
            if path.is_dir() {
                sources(&path, files)?;
            } else if path.extension().is_some_and(|ext| ext == "rs") {
                files.push(path);
            }
        }
        Ok(())
    }
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("src");
    let mut files = Vec::new();
    sources(&root, &mut files)?;
    files.sort();
    let mut bytes = Vec::new();
    for path in files {
        bytes.extend_from_slice(
            path.strip_prefix(&root)?
                .to_str()
                .ok_or("source path")?
                .as_bytes(),
        );
        bytes.push(0);
        bytes.extend_from_slice(&std::fs::read(path)?);
        bytes.push(0);
    }
    Ok(codec::sha256([bytes.as_slice()])
        .map_err(|e| format!("{e:?}"))?
        .iter()
        .fold(String::new(), |mut text, byte| {
            use core::fmt::Write as _;
            write!(text, "{byte:02x}").unwrap_or_else(|error| panic!("hex formatting: {error}"));
            text
        }))
}

pub fn module() -> Checked<Arc<ValidatedModule>> {
    assert_build_recipe();
    let hex = std::fs::read_to_string(
        Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/ai-market-opt-z.hex"),
    )
    .map_err(|e| format!("{e:?}"))?;
    let hex = hex.trim();
    if hex.len() % 2 != 0 {
        return Err("odd guest hex length".into());
    }
    let wasm = (0..hex.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&hex[i..i + 2], 16))
        .collect::<Result<Vec<_>, _>>()
        .map_err(|e| format!("{e:?}"))?;
    let metadata = std::fs::read_to_string(
        Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/ai-market-opt-z.kvx"),
    )?;
    assert!(metadata.lines().any(|line| line == "opt_level = \"z\""));
    assert!(metadata
        .lines()
        .any(|line| line == "crate_type = \"cdylib\""));
    assert!(metadata.lines().any(|line| line == "lto = true"));
    assert!(metadata
        .lines()
        .any(|line| line
            == "llvm_passes = \"enable-merge-functions ir-outliner inline-threshold=40\""));
    assert!(metadata.lines().any(|line| line == "chain_id = 125"));
    assert!(metadata
        .lines()
        .any(|line| line == "optimizer_version = \"133\""));
    assert!(metadata.lines().any(|line| line == "profile = \"release\""));
    let domain = layerx_programs_ai_market::MERGED_CHAIN_DOMAIN.iter().fold(
        String::new(),
        |mut text, byte| {
            use core::fmt::Write as _;
            write!(text, "{byte:02x}").unwrap_or_else(|error| panic!("hex formatting: {error}"));
            text
        },
    );
    assert!(metadata
        .lines()
        .any(|line| line == format!("chain_domain = \"{domain}\"")));
    assert!(metadata
        .lines()
        .any(|line| line == format!("postprocess = \"{BUILD_POSTPROCESS}\"")));
    let source = source_digest()?;
    assert!(
        metadata
            .lines()
            .any(|line| line == format!("source_sha256 = \"{source}\"")),
        "compiled guest is stale; regenerate it from the current production source"
    );
    let hash = codec::sha256([wasm.as_slice()]).map_err(|e| format!("{e:?}"))?;
    let hash = hash.iter().fold(String::new(), |mut text, byte| {
        use core::fmt::Write as _;
        write!(text, "{byte:02x}").unwrap_or_else(|error| panic!("hex formatting: {error}"));
        text
    });
    assert!(
        metadata
            .lines()
            .any(|line| line == format!("sha256 = \"{hash}\"")),
        "fixture provenance digest differs"
    );
    assert!(metadata
        .lines()
        .any(|line| line == format!("wasm_bytes = {}", wasm.len())));
    assert_eq!(BUILD_OPT_LEVEL, "z");
    assert!(
        wasm.len()
            .checked_add(DEPLOYMENT_ENVELOPE_BYTES)
            .ok_or("guest length overflow")
            .map_err(|e| format!("{e:?}"))?
            <= DEPLOYMENT_LIMIT,
        "guest plus deployment envelope exceeds protocol limit"
    );
    Ok(Arc::new(
        WasmEngine::declared()
            .map_err(|e| format!("{e:?}"))?
            .validate_versioned_metered(ABI_V5_VERSION, &wasm, FuelSchedule::WASMI_0_31_2)
            .map_err(|e| format!("{e:?}"))?,
    ))
}

/// Caller-owned in-memory account store, the peer of the production transfer host.
/// Its restart image is generated from the same committed balances.
#[derive(Default)]
pub struct Ledger {
    pub balances: BTreeMap<[u8; 32], u128>,
    pub transfers: usize,
}
impl Ledger {
    pub fn balance(&self, account: [u8; 32]) -> u128 {
        self.balances.get(&account).copied().unwrap_or(0)
    }
    pub fn total(&self) -> u128 {
        self.balances.values().copied().sum()
    }
    pub fn image(&self) -> Vec<u8> {
        let mut out = (self.transfers as u64).to_be_bytes().to_vec();
        for (account, balance) in &self.balances {
            out.extend_from_slice(account);
            out.extend_from_slice(&balance.to_be_bytes());
        }
        out
    }
    pub fn reopen(bytes: &[u8]) -> Checked<Self> {
        if bytes.len() < 8 || !(bytes.len() - 8).is_multiple_of(48) {
            return Err("invalid ledger image".into());
        }
        let mut out = Self {
            transfers: usize::try_from(u64::from_be_bytes(
                bytes[..8].try_into().map_err(|e| format!("{e:?}"))?,
            ))
            .map_err(|e| format!("{e:?}"))?,
            ..Self::default()
        };
        for row in bytes[8..].chunks_exact(48) {
            let account = row[..32].try_into().map_err(|e| format!("{e:?}"))?;
            if out
                .balances
                .insert(
                    account,
                    u128::from_be_bytes(row[32..].try_into().map_err(|e| format!("{e:?}"))?),
                )
                .is_some()
            {
                return Err("duplicate ledger account".into());
            }
        }
        Ok(out)
    }
    fn apply(&mut self, legs: &[rt::TransferRequest]) -> Checked {
        let before = self.total();
        let mut next = self.balances.clone();
        for leg in legs {
            let source = leg.source.account();
            let balance = next
                .get(&source)
                .copied()
                .ok_or("missing transfer source")
                .map_err(|e| format!("{e:?}"))?;
            next.insert(
                source,
                balance
                    .checked_sub(leg.amount)
                    .ok_or("insufficient balance")
                    .map_err(|e| format!("{e:?}"))?,
            );
            let credit = next
                .get(&leg.to)
                .copied()
                .unwrap_or(0)
                .checked_add(leg.amount)
                .ok_or("balance overflow")
                .map_err(|e| format!("{e:?}"))?;
            next.insert(leg.to, credit);
        }
        assert_eq!(next.values().copied().sum::<u128>(), before);
        self.balances = next;
        self.transfers += legs.len();
        Ok(())
    }
}

pub struct Guest {
    pub module: Arc<ValidatedModule>,
    pub storage: Storage,
    pub ledger: Ledger,
    pub program: rt::ProgramId,
    pub asset: [u8; 32],
    pub account: [u8; 32],
}
pub struct Observed {
    pub fuel: u64,
    pub usage: rt::meter::MeteredUsage,
    pub program: rt::ProgramId,
    pub abi_version: u16,
    pub response: Vec<u8>,
    pub events: Vec<rt::ProgramEvent>,
    pub transfers: usize,
}
impl Guest {
    pub fn new(module: Arc<ValidatedModule>, program: [u8; 32], asset: [u8; 32]) -> Checked<Self> {
        let account = layerx_programs_ai_market::registry::derive_rewards_account(
            layerx_programs_ai_market::ProgramId::new(program).map_err(|e| format!("{e:?}"))?,
            layerx_programs_ai_market::AssetId::new(asset).map_err(|e| format!("{e:?}"))?,
        )
        .map_err(|e| format!("{e:?}"))?
        .bytes();
        let mut ledger = Ledger::default();
        ledger.balances.insert(account, 0);
        Ok(Self {
            module,
            storage: Storage::new(),
            ledger,
            program: rt::ProgramId::new(program).map_err(|e| format!("{e:?}"))?,
            asset,
            account,
        })
    }
    pub fn state(&mut self) -> Checked<Option<Vec<u8>>> {
        Ok(self
            .storage
            .transaction(StorageNamespace::shared(self.program))
            .read(SHARED_STATE_KEY)
            .map_err(|e| format!("{e:?}"))?)
    }
    pub fn seed(&mut self, bytes: &[u8]) -> Checked {
        let mut tx = self
            .storage
            .transaction(StorageNamespace::shared(self.program));
        tx.write(SHARED_STATE_KEY, bytes)
            .map_err(|e| format!("{e:?}"))?;
        let _ = tx.commit();
        Ok(())
    }
    pub fn restart(self) -> Checked<Self> {
        let storage = self
            .storage
            .replay_state_bytes(16 * 1024 * 1024)
            .map_err(|e| format!("{e:?}"))?;
        let balances = self.ledger.image();
        let Self {
            module,
            program,
            asset,
            account,
            ..
        } = self;
        Ok(Self {
            module,
            program,
            asset,
            account,
            storage: Storage::decode_untrusted_replay_state(&storage, 16 * 1024 * 1024)
                .map_err(|e| format!("{e:?}"))?,
            ledger: Ledger::reopen(&balances).map_err(|e| format!("{e:?}"))?,
        })
    }
    pub fn call(
        &mut self,
        ctx: &CallContext,
        envelope: &[u8],
        expected_next: Option<&[u8]>,
    ) -> Checked<Observed> {
        self.call_expect(ctx, envelope, expected_next, None)
    }
    pub fn call_refusal(
        &mut self,
        ctx: &CallContext,
        envelope: &[u8],
        error: layerx_programs_ai_market::errors::ApplicationError,
    ) -> Checked<Observed> {
        let before = self.state()?;
        let ledger = self.ledger.image();
        let observed = self.call_expect(ctx, envelope, None, Some(error))?;
        assert_eq!(self.state()?, before);
        assert_eq!(self.ledger.image(), ledger);
        Ok(observed)
    }
    fn call_expect(
        &mut self,
        ctx: &CallContext,
        envelope: &[u8],
        expected_next: Option<&[u8]>,
        expected_error: Option<layerx_programs_ai_market::errors::ApplicationError>,
    ) -> Checked<Observed> {
        let (e, digest) = Self::decoded_guest_envelope(envelope)?;
        let actor = rt::PrincipalId::new(ctx.principal.bytes()).map_err(|e| format!("{e:?}"))?;
        let mut caps = self.deposit_capabilities(&e)?;
        self.append_payout_capability(ctx, &e, expected_next, &mut caps)?;
        self.append_canary_capability(&e, expected_next, &mut caps)?;
        let record = self.execute_guest(ctx, envelope, actor, digest, caps)?;
        let fuel = record.execution().usage().cpu_fuel;
        assert!(
            fuel > 0,
            "every guest execution must report measured CPU fuel"
        );
        assert_eq!(CPU_FUEL, rt::meter::DEFAULT_CPU_FUEL);
        assert!(fuel <= CPU_FUEL);
        match record.outcome() {
            V2ActivityOutcome::Success { response, effects } => {
                assert!(expected_error.is_none(), "expected an application refusal");
                assert_eq!(response.code, 0);
                for leg in &effects.transfers {
                    assert_eq!(leg.asset, self.asset);
                    match &leg.source {
                        rt::TransferSource::Program(a) => {
                            assert_eq!(a.owner_program(), self.program);
                            assert_eq!(a.source_account(), self.account);
                            assert_eq!(a.seed(), REWARDS_ACCOUNT_SEED);
                            assert_eq!(a.amount(), leg.amount);
                            assert_eq!(a.to(), leg.to);
                        }
                        rt::TransferSource::ProgramFunding { principal, binding } => {
                            assert_eq!(*principal, actor);
                            assert_eq!(binding.owner_program(), self.program);
                            assert_eq!(binding.destination_account(), self.account);
                        }
                        rt::TransferSource::Principal(_) => {
                            return Err("market emitted unbound principal transfer".into())
                        }
                    }
                }
                self.ledger
                    .apply(&effects.transfers)
                    .map_err(|e| format!("{e:?}"))?;
                Ok(Observed {
                    fuel,
                    usage: record.execution().usage(),
                    program: record.root_program(),
                    abi_version: rt::abi_policy::abi_version(record.abi_revision()),
                    response: response.bytes.clone(),
                    events: effects.events.clone(),
                    transfers: effects.transfers.len(),
                })
            }
            V2ActivityOutcome::Failure(failure) => {
                if let Some(error) = expected_error {
                    let frame = codec::decode_result(failure.reason().bytes())
                        .map_err(|e| format!("{e:?}"))?;
                    assert_eq!(frame.error, Some(error));
                    return Ok(Observed {
                        fuel,
                        usage: record.execution().usage(),
                        program: record.root_program(),
                        abi_version: rt::abi_policy::abi_version(record.abi_revision()),
                        response: failure.reason().bytes().to_vec(),
                        events: vec![],
                        transfers: 0,
                    });
                }
                let detail = match codec::decode_result(failure.reason().bytes()) {
                    Ok(result) => match result.error {
                        Some(error) => {
                            let name = layerx_programs_ai_market::errors::APPLICATION_ERRORS
                                .iter()
                                .find(|entry| entry.code == error.code())
                                .map_or("UNREGISTERED_APPLICATION_ERROR", |entry| entry.name);
                            format!("{name} (0x{:04x})", error.code())
                        }
                        None => format!("{failure:?}"),
                    },
                    Err(_) => format!("{failure:?}"),
                };
                Err(format!(
                    "guest {} at height {} refused: {detail}",
                    e.operation.metadata().name,
                    ctx.height
                )
                .into())
            }
            other @ V2ActivityOutcome::Resource(_) => Err(format!(
                "guest {} at height {} refused: {other:?}",
                e.operation.metadata().name,
                ctx.height
            )
            .into()),
        }
    }
    fn append_canary_capability(
        &mut self,
        e: &codec::Envelope<'_>,
        expected_next: Option<&[u8]>,
        caps: &mut Vec<Capability>,
    ) -> Checked {
        if let (Some(before), Some(after)) = (self.state()?, expected_next) {
            use layerx_programs_ai_market::{canary, value_adapter::ValueAction};
            if matches!(
                e.operation,
                dispatch::ChallengeLap | dispatch::ResolveChallenge
            ) {
                match canary::value_action(e.operation, &before, after)
                    .map_err(|e| format!("{e:?}"))?
                {
                    ValueAction::Fund { amount } => caps.push(Capability::Transfer402 {
                        asset: self.asset,
                        to: self.account,
                        maximum_amount: amount,
                    }),
                    ValueAction::Pay { recipient, amount } => caps.push(Capability::ProgramSpend {
                        owner_program: self.program,
                        seed: REWARDS_ACCOUNT_SEED.to_vec(),
                        source_account: self.account,
                        asset: self.asset,
                        to: recipient.bytes(),
                        maximum_amount: amount,
                    }),
                    ValueAction::None => {}
                }
            }
        }
        Ok(())
    }

    fn deposit_capabilities(&self, e: &codec::Envelope<'_>) -> Checked<Vec<Capability>> {
        let mut caps = vec![
            Capability::SharedStorageRead,
            Capability::SharedStorageWrite,
            Capability::EmitEvent,
        ];
        if matches!(e.operation, dispatch::FUND | dispatch::LockBond) {
            let amount = if e.operation == dispatch::FUND {
                rewards::FundRequest::decode(e.payload)
                    .map_err(|e| format!("{e:?}"))?
                    .amount
            } else {
                let mut r = codec::Reader::new(e.payload);
                r.take(64).map_err(|e| format!("{e:?}"))?;
                let amount = r.u128().map_err(|e| format!("{e:?}"))?;
                r.finish().map_err(|e| format!("{e:?}"))?;
                amount
            };
            caps.push(Capability::Transfer402 {
                asset: self.asset,
                to: self.account,
                maximum_amount: amount,
            });
        }
        Ok(caps)
    }
    fn append_payout_capability(
        &mut self,
        ctx: &CallContext,
        e: &codec::Envelope<'_>,
        expected_next: Option<&[u8]>,
        caps: &mut Vec<Capability>,
    ) -> Checked {
        if let (Some(before), Some(after)) =
            (self.state().map_err(|e| format!("{e:?}"))?, expected_next)
        {
            let old = state::decode_shared_state(&before).map_err(|e| format!("{e:?}"))?;
            let new = state::decode_shared_state(after).map_err(|e| format!("{e:?}"))?;
            let ledger = |state: &state::SharedState<'_>| -> Checked<rewards::RewardLedger> {
                if state.feature_sections[state::Section::SettlementClaims.index()].is_empty() {
                    let header = layerx_programs_ai_market::registry_ops::PolicySection::decode(
                        state.feature_sections[state::Section::PolicyLifecycle.index()],
                    )
                    .map_err(|e| format!("{e:?}"))?
                    .header;
                    Ok(rewards::RewardLedger::new(
                        header.funding_asset,
                        header.rewards_account,
                        header.refund_recipient_account,
                    )
                    .map_err(|e| format!("{e:?}"))?)
                } else {
                    Ok(state::settlement_rewards(state, None)
                        .map_err(|e| format!("{e:?}"))?
                        .ledger()
                        .map_err(|e| format!("{e:?}"))?)
                }
            };
            let a = ledger(&old).map_err(|e| format!("{e:?}"))?;
            let b = ledger(&new).map_err(|e| format!("{e:?}"))?;
            let paid = b
                .total_claimed
                .checked_sub(a.total_claimed)
                .ok_or("paid counter regressed")
                .map_err(|e| format!("{e:?}"))?;
            let refunded = b
                .tracked_refunds
                .checked_sub(a.tracked_refunds)
                .ok_or("refund counter regressed")
                .map_err(|e| format!("{e:?}"))?;
            let amount = paid
                .checked_add(refunded)
                .ok_or("payout overflow")
                .map_err(|e| format!("{e:?}"))?;
            if amount > 0 {
                let to = if e.operation == dispatch::RefundBond {
                    ctx.principal.bytes()
                } else if paid > 0 {
                    rewards::ClaimRequest::decode(e.payload)
                        .map_err(|e| format!("{e:?}"))?
                        .recipient
                        .bytes()
                } else {
                    a.refund_recipient.bytes()
                };
                caps.push(Capability::ProgramSpend {
                    owner_program: self.program,
                    seed: REWARDS_ACCOUNT_SEED.to_vec(),
                    source_account: self.account,
                    asset: self.asset,
                    to,
                    maximum_amount: amount,
                });
            }
        }
        Ok(())
    }
    fn execute_guest(
        &mut self,
        ctx: &CallContext,
        envelope: &[u8],
        actor: rt::PrincipalId,
        digest: layerx_programs_ai_market::RequestDigest,
        caps: Vec<Capability>,
    ) -> Checked<rt::V2AuthorizedExecutionRecord> {
        let executor = Executor::declared();
        let binding = ActivityBudgetBinding::new(digest.bytes()).map_err(|e| format!("{e:?}"))?;
        let admitted = executor
            .admit_activity_budget_for_qualification(
                DeclaredBudget::protocol_maximum(),
                actor,
                binding,
                u128::MAX,
            )
            .map_err(|e| format!("{e:?}"))?;
        let no_receipts = rt::UnavailableReceiptOracle;
        let request = BudgetedAuthorizedExecutionRequest::new(
            AuthorizedExecutionRequest {
                module: &self.module,
                program: self.program,
                authorization: AuthorizationContext::new(
                    actor,
                    CapabilitySet::new(caps).map_err(|e| format!("{e:?}"))?,
                ),
                receipts: &no_receipts,
                entrypoint: CALL_ENTRY_EXPORT,
                calldata: envelope,
                composition: CompositionContext::isolated(),
                response_capacity: MAX_RESULT_BYTES,
            },
            admitted,
            actor,
            binding,
        );
        let record = executor
            .execute_authorized_v5_budgeted_for_qualification(
                &mut self.storage,
                request,
                1,
                ctx.height,
            )
            .map_err(|e| format!("{e:?}"))?;
        Ok(record)
    }
    fn decoded_guest_envelope(
        envelope: &[u8],
    ) -> Checked<(
        codec::Envelope<'_>,
        layerx_programs_ai_market::RequestDigest,
    )> {
        let selector = u16::from_be_bytes(envelope[8..10].try_into()?);
        let (e, digest) = match selector {
            0x0b10..=0x0b13 => {
                let request = layerx_programs_ai_market::queries::decode_view_envelope(envelope)
                    .map_err(|e| format!("{e:?}"))?;
                let digest = request.request_digest().map_err(|e| format!("{e:?}"))?;
                (request.envelope, digest)
            }
            0x0b14 => {
                let request = layerx_programs_ai_market::placement::decode_book_envelope(envelope)
                    .map_err(|e| format!("{e:?}"))?;
                let digest = request.request_digest().map_err(|e| format!("{e:?}"))?;
                (request.envelope, digest)
            }
            _ => {
                let request = codec::decode_envelope(envelope).map_err(|e| format!("{e:?}"))?;
                let digest = request.request_digest().map_err(|e| format!("{e:?}"))?;
                (request.envelope, digest)
            }
        };
        Ok((e, digest))
    }
}
