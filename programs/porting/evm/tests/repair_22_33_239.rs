use layerx_program_sdk::{
    lxt20::{Request, REFERENCE_ASSET, REFERENCE_ISSUER, REFERENCE_METADATA, REFERENCE_SUPPLY},
    payments::PreparedProgramAccount,
    AccountId, Amount, AssetId, Principal,
};
use layerx_programs::lxt20::reference_interface;
use layerx_programs_runtime::{
    derive_program_account, AbiEffects, AbiError, ActivityBudgetBinding, AuthorizationContext,
    AuthorizedExecutionRequest, BudgetedAuthorizedExecutionRequest, Capability, CapabilitySet,
    CompositionContext, DeclaredBudget, ExecutionError, Executor, PrincipalId, ProgramId, Storage,
    TransferSource, V2ActivityOutcome, V2AuthorizedExecutionRecord, ValidatedModule, WasmEngine,
    ABI_V5_VERSION,
};
use programs_porting_evm::{
    AbsentReceipts, PortRefusal, ProgramAccountTransferPlan, TranslatedValueFlow, ValueFlow,
};

const GUIDE: &str = include_str!("../MIGRATION.md");
const GUEST: &[u8] = include_bytes!("../../../fixtures/pay5/token-lxt20.wasm");
const INTERFACE: &[u8] = include_bytes!("../../../fixtures/pay5/token-lxt20.interface");
const INITIALIZE: &[u8] = &[0x4c, 0x58, 0x14, 0, 1, 0x20, 0, 0, 0, 0];

fn program() -> ProgramId {
    ProgramId::new([0x55; 32]).expect("PAY5 program")
}

fn principal(bytes: [u8; 32]) -> Principal {
    Principal::new(bytes).expect("DID identity")
}

fn prepared(seed: &[u8]) -> PreparedProgramAccount<'_> {
    PreparedProgramAccount::new(
        layerx_program_sdk::ProgramId::new(program().bytes()).expect("SDK program"),
        seed,
        AssetId::new(REFERENCE_ASSET).expect("backing asset"),
    )
    .expect("prepared program account")
}

fn account(owner: Principal) -> AccountId {
    prepared(&owner.bytes()).account()
}

fn base_grants() -> Vec<Capability> {
    vec![
        Capability::SharedStorageRead,
        Capability::SharedStorageWrite,
    ]
}

fn funding() -> Capability {
    Capability::Transfer402 {
        asset: REFERENCE_ASSET,
        to: account(principal(REFERENCE_ISSUER)).bytes(),
        maximum_amount: REFERENCE_SUPPLY,
    }
}

struct Token {
    module: ValidatedModule,
    storage: Storage,
    sequence: u64,
}

impl Token {
    fn new() -> Self {
        // The committed ABI-v2 guest is also admitted by ABI v5. This public
        // qualification seam supplies authenticated context without native FFI.
        let module = WasmEngine::declared()
            .expect("engine")
            .validate_versioned(ABI_V5_VERSION, GUEST)
            .expect("unmodified committed guest under ABI v5");
        Self {
            module,
            storage: Storage::new(),
            sequence: 0,
        }
    }

    fn invoke(
        &mut self,
        caller: Principal,
        entry: &str,
        input: &[u8],
        grants: Vec<Capability>,
    ) -> V2AuthorizedExecutionRecord {
        self.execute(caller, entry, input, grants)
            .expect("production executor")
    }

    fn execute(
        &mut self,
        caller: Principal,
        entry: &str,
        input: &[u8],
        grants: Vec<Capability>,
    ) -> Result<V2AuthorizedExecutionRecord, ExecutionError> {
        self.sequence += 1;
        let payer = PrincipalId::new(caller.bytes()).expect("authenticated caller");
        let binding = ActivityBudgetBinding::new([0x39; 32]).expect("activity binding");
        let executor = Executor::declared();
        let admitted = executor
            .admit_activity_budget_for_qualification(
                DeclaredBudget::protocol_maximum(),
                payer,
                binding,
                u128::MAX,
            )
            .expect("admitted execution budget");
        let request = BudgetedAuthorizedExecutionRequest::new(
            AuthorizedExecutionRequest {
                module: &self.module,
                program: program(),
                authorization: AuthorizationContext::new(
                    payer,
                    CapabilitySet::new(grants).expect("canonical grants"),
                ),
                receipts: &AbsentReceipts,
                entrypoint: entry,
                calldata: input,
                composition: CompositionContext::isolated(),
                response_capacity: 70,
            },
            admitted,
            payer,
            binding,
        );
        executor.execute_authorized_v5_budgeted_for_qualification(
            &mut self.storage,
            request,
            self.sequence,
            1,
        )
    }

    fn call(
        &mut self,
        caller: Principal,
        request: Request,
        extra: Vec<Capability>,
    ) -> V2AuthorizedExecutionRecord {
        let input = request.encode().expect("SDK producer");
        assert_eq!(Request::decode(input.as_slice()), Ok(request));
        let interface = reference_interface(GUEST, program()).expect("bound interface");
        let (entry, _) = interface
            .decode_call(input.as_slice())
            .expect("registry routing");
        assert_eq!(entry.name, request.method());
        let mut grants = base_grants();
        grants.extend(extra);
        self.invoke(caller, &entry.name, input.as_slice(), grants)
    }

    fn initialize(&mut self) -> V2AuthorizedExecutionRecord {
        let mut grants = base_grants();
        grants.push(funding());
        self.invoke(
            principal(REFERENCE_ISSUER),
            "initialize",
            INITIALIZE,
            grants,
        )
    }

    fn query(&mut self, request: Request) -> u128 {
        let record = self.call(principal(REFERENCE_ISSUER), request, vec![]);
        let (payload, effects) = success(&record);
        assert_eq!(effects, &AbiEffects::default());
        u128::from_be_bytes(payload.try_into().expect("16-byte query response"))
    }
}

fn success(record: &V2AuthorizedExecutionRecord) -> (&[u8], &AbiEffects) {
    let V2ActivityOutcome::Success { response, effects } = record.outcome() else {
        panic!("expected success: {record:?}");
    };
    assert_eq!(response.code, 0);
    assert_eq!(&response.bytes[..2], &[1, 0x20]);
    let length = u32::from_be_bytes(response.bytes[2..6].try_into().expect("response length"));
    assert_eq!(
        response.bytes.len(),
        6 + usize::try_from(length).expect("bounded length")
    );
    (&response.bytes[6..], effects)
}

fn refused(record: &V2AuthorizedExecutionRecord, storage: &Storage, before: &Storage) {
    assert!(
        matches!(record.outcome(), V2ActivityOutcome::Failure(_)),
        "{record:?}"
    );
    assert!(record.response().is_none());
    assert!(record.effects().is_none());
    assert_eq!(storage, before, "refusal must roll back every staged write");
}

#[test]
fn evm_custody_guide_registered_later_payout() {
    let escrow = GUIDE
        .split("### Escrow, splitting and refunds")
        .nth(1)
        .expect("escrow guide")
        .split("## Failure and gas")
        .next()
        .expect("escrow section");
    for instruction in [
        "ProgramAccountTransferPlan::new",
        "prepared.registration_payload()",
        "pay_from_program_account",
        "original depositor need not be present",
        "maximum_amount = 40",
    ] {
        assert!(
            escrow.contains(instruction),
            "missing custody instruction: {instruction}"
        );
    }
    let issuer = principal(REFERENCE_ISSUER);
    let recipient = principal([0x22; 32]);
    let spender = principal([0x33; 32]);
    let seed = issuer.bytes();
    let prepared = prepared(&seed);
    assert_eq!(
        prepared.account().bytes(),
        derive_program_account(program(), &seed)
            .expect("runtime derivation")
            .bytes()
    );
    let registration = prepared
        .registration_payload()
        .expect("native registration payload");
    // The signed native activity is a committed producer transcript, not a
    // registration performed by the token's approve handler.
    let native_registration =
        include_bytes!("../../../fixtures/pay5/native-token/02-account-register.activity");
    assert!(native_registration
        .windows(registration.len())
        .any(|bytes| bytes == registration.as_slice()));

    let mut token = Token::new();
    let deposit = token.initialize();
    let (_, effects) = success(&deposit);
    assert_eq!(effects.transfers.len(), 1);
    assert_eq!(effects.transfers[0].amount, REFERENCE_SUPPLY);
    match effects.transfers[0].source() {
        TransferSource::ProgramFunding { principal, binding } => {
            assert_eq!(principal.bytes(), issuer.bytes());
            assert_eq!(binding.destination_account(), prepared.account().bytes());
            assert_eq!(binding.owner_program(), program());
            assert_eq!(binding.seed(), seed);
            assert_eq!(binding.asset(), REFERENCE_ASSET);
        }
        source => panic!("initialization must fund the account: {source:?}"),
    }
    for (caller, amount) in [(recipient, 0), (issuer, 100)] {
        let record = token.call(
            caller,
            Request::Approve {
                spender,
                amount: Amount::from_u128(amount),
            },
            vec![],
        );
        let (payload, effects) = success(&record);
        assert!(payload.is_empty());
        assert_eq!(effects, &AbiEffects::default());
    }

    let flow = ValueFlow::ContractFunded {
        recipient: account(recipient).bytes(),
        amount: 40,
    };
    assert_eq!(
        flow.translate(REFERENCE_ASSET, spender.bytes()),
        Err(PortRefusal::ContractHeldBalance)
    );
    let TranslatedValueFlow::ProgramAccount(plan) = flow
        .translate_with_program_account(
            REFERENCE_ASSET,
            spender.bytes(),
            program(),
            &seed,
            prepared.account().bytes(),
        )
        .expect("context-aware custody plan")
    else {
        panic!("program source required")
    };
    assert_eq!(
        plan,
        ProgramAccountTransferPlan::new(
            program(),
            &seed,
            prepared.account().bytes(),
            REFERENCE_ASSET,
            account(recipient).bytes(),
            40
        )
        .expect("guide example")
    );
    let encoded = prepared
        .spend_grant(account(recipient), Amount::from_u128(40))
        .expect("SDK spend transport");
    let decoded = CapabilitySet::decode_v2_canonical(encoded.as_slice()).expect("runtime consumer");
    assert_eq!(decoded, vec![plan.capability()]);
    let request = Request::TransferFrom {
        owner: issuer,
        to: account(recipient),
        amount: Amount::from_u128(40),
    };
    let interface = reference_interface(GUEST, program()).expect("interface");
    let calldata = request.encode().expect("request");
    assert!(interface
        .authorize_call(program(), calldata.as_slice(), &decoded)
        .is_ok());
    assert!(interface
        .authorize_call(program(), calldata.as_slice(), &[])
        .is_err());

    let mut too_small = plan.capability();
    if let Capability::ProgramSpend { maximum_amount, .. } = &mut too_small {
        *maximum_amount = 39;
    }
    let mut wrong_asset = plan.capability();
    if let Capability::ProgramSpend { asset, .. } = &mut wrong_asset {
        *asset = [0x77; 32];
    }
    let mut wrong_recipient = plan.capability();
    if let Capability::ProgramSpend { to, .. } = &mut wrong_recipient {
        *to = account(issuer).bytes();
    }
    let other_seed = recipient.bytes();
    let other_source = ProgramAccountTransferPlan::new(
        program(),
        &other_seed,
        account(recipient).bytes(),
        REFERENCE_ASSET,
        account(recipient).bytes(),
        40,
    )
    .expect("different valid source")
    .capability();
    let foreign = ProgramId::new([0x66; 32]).expect("foreign program");
    let foreign_source = derive_program_account(foreign, &seed)
        .expect("foreign source")
        .bytes();
    let wrong_owner = ProgramAccountTransferPlan::new(
        foreign,
        &seed,
        foreign_source,
        REFERENCE_ASSET,
        account(recipient).bytes(),
        40,
    )
    .expect("foreign owner plan")
    .capability();
    for extra in [
        vec![],
        vec![too_small],
        vec![wrong_asset],
        vec![wrong_recipient],
        vec![other_source],
    ] {
        let before = token.storage.clone();
        let record = token.call(spender, request, extra);
        refused(&record, &token.storage, &before);
        assert_eq!(
            token.query(Request::Allowance {
                owner: issuer,
                spender
            }),
            100
        );
    }
    let before = token.storage.clone();
    let mut foreign_grants = base_grants();
    foreign_grants.push(wrong_owner);
    assert_eq!(
        token.execute(
            spender,
            request.method(),
            calldata.as_slice(),
            foreign_grants
        ),
        Err(ExecutionError::Abi(AbiError::InvalidCapability)),
        "foreign-owner authority must be rejected before guest execution"
    );
    assert_eq!(
        token.storage, before,
        "admission refusal must preserve all state"
    );
    assert_eq!(
        token.query(Request::Allowance {
            owner: issuer,
            spender
        }),
        100
    );
    let payout = token.call(spender, request, decoded);
    let (payload, effects) = success(&payout);
    assert!(payload.is_empty());
    assert_eq!(effects.transfers.len(), 1);
    let payment = &effects.transfers[0];
    assert_eq!(
        payment.principal.bytes(),
        spender.bytes(),
        "the original funder is absent"
    );
    assert_eq!(payment.program, program());
    assert_eq!(payment.asset, plan.asset());
    assert_eq!(payment.to, plan.to());
    assert_eq!(payment.amount, plan.amount());
    let TransferSource::Program(authority) = payment.source() else {
        panic!("payout must debit custody")
    };
    assert_eq!(authority.owner_program(), plan.owner_program());
    assert_eq!(authority.seed(), plan.seed());
    assert_eq!(authority.source_account(), plan.source());
    assert_eq!(authority.staging_frame(), payment.frame);
    assert_eq!(
        token.query(Request::BalanceOf { owner: issuer }),
        REFERENCE_SUPPLY - 40
    );
    assert_eq!(token.query(Request::BalanceOf { owner: recipient }), 40);
    assert_eq!(
        token.query(Request::Allowance {
            owner: issuer,
            spender
        }),
        60
    );

    let returned = token.call(
        recipient,
        Request::Transfer {
            to: account(issuer),
            amount: Amount::from_u128(15),
        },
        vec![ProgramAccountTransferPlan::new(
            program(),
            &other_seed,
            account(recipient).bytes(),
            REFERENCE_ASSET,
            account(issuer).bytes(),
            15,
        )
        .expect("bounded return")
        .capability()],
    );
    assert_eq!(success(&returned).1.transfers[0].amount, 15);
    assert_eq!(token.query(Request::BalanceOf { owner: recipient }), 25);
    assert_eq!(token.query(Request::TotalSupply), REFERENCE_SUPPLY);
    let revoke = token.call(
        issuer,
        Request::Approve {
            spender,
            amount: Amount::ZERO,
        },
        vec![],
    );
    assert!(success(&revoke).0.is_empty());
    let before = token.storage.clone();
    let denied = token.call(spender, request, vec![plan.capability()]);
    refused(&denied, &token.storage, &before);
    assert_eq!(
        token.query(Request::Allowance {
            owner: issuer,
            spender
        }),
        0
    );
    assert_eq!(
        ProgramAccountTransferPlan::new(
            program(),
            &seed,
            account(recipient).bytes(),
            REFERENCE_ASSET,
            account(recipient).bytes(),
            40
        ),
        Err(PortRefusal::InvalidProgramAccount)
    );
    assert_eq!(
        ValueFlow::SelfDestructSweep {
            recipient: account(recipient).bytes()
        }
        .translate_with_program_account(
            REFERENCE_ASSET,
            spender.bytes(),
            program(),
            &seed,
            prepared.account().bytes()
        ),
        Err(PortRefusal::UnboundedBalanceSweep)
    );
}

#[test]
fn evm_lxt20_guide_complete_codec_and_native_interface() {
    let guide = GUIDE
        .split("## ERC-20, LXT-20 and native assets")
        .nth(1)
        .expect("token guide");
    assert!(guide.contains("eight exports"));
    assert!(guide.contains("4c 58 14 00 01 20 00 00 00 00"));
    assert!(guide.contains("02-account-register` through `04-account-register"));
    let interface = reference_interface(GUEST, program()).expect("real guest interface");
    assert_eq!(interface.canonical_encoding(), INTERFACE);
    assert_eq!(interface.entries().len(), 8);
    let owner = principal(REFERENCE_ISSUER);
    let spender = principal([0x33; 32]);
    let requests = [
        Request::Transfer {
            to: account(spender),
            amount: Amount::from_u128(1),
        },
        Request::Approve {
            spender,
            amount: Amount::ZERO,
        },
        Request::TransferFrom {
            owner,
            to: account(spender),
            amount: Amount::from_u128(1),
        },
        Request::BalanceOf { owner },
        Request::Allowance { owner, spender },
        Request::TotalSupply,
        Request::Metadata,
    ];
    for (ordinal, request) in (1_u8..).zip(requests) {
        let bytes = request.encode().expect("canonical SDK request");
        assert_eq!(bytes.as_slice()[3], ordinal);
        assert_eq!(Request::decode(bytes.as_slice()), Ok(request));
        let (entry, _) = interface
            .decode_call(bytes.as_slice())
            .expect("real registry consumer");
        assert_eq!(entry.name, request.method());
        assert!(guide.contains(request.method()));
    }
    assert!(
        Request::decode(INITIALIZE).is_err(),
        "initialize has its own input, not a Request variant"
    );
    assert_eq!(
        interface
            .decode_call(INITIALIZE)
            .expect("initializer export")
            .0
            .name,
        "initialize"
    );
    let mut token = Token::new();
    assert_eq!(token.query(Request::TotalSupply), 0);
    let metadata = token.call(owner, Request::Metadata, vec![]);
    assert_eq!(success(&metadata).0, REFERENCE_METADATA);
    let approval = token.call(
        owner,
        Request::Approve {
            spender,
            amount: Amount::ZERO,
        },
        vec![],
    );
    assert!(success(&approval).0.is_empty());
    assert_eq!(
        token.query(Request::TotalSupply),
        0,
        "approval cannot create supply"
    );
}

#[test]
fn evm_lxt20_guide_initialize_authority_and_refusal_rollback() {
    let issuer = principal(REFERENCE_ISSUER);
    let mut token = Token::new();
    let mut correct = base_grants();
    correct.push(funding());
    let mut cases = vec![(principal([0x22; 32]), INITIALIZE.to_vec(), correct.clone())];
    for grant in [
        None,
        Some(Capability::Transfer402 {
            asset: REFERENCE_ASSET,
            to: account(issuer).bytes(),
            maximum_amount: REFERENCE_SUPPLY - 1,
        }),
        Some(Capability::Transfer402 {
            asset: [0x77; 32],
            to: account(issuer).bytes(),
            maximum_amount: REFERENCE_SUPPLY,
        }),
        Some(Capability::Transfer402 {
            asset: REFERENCE_ASSET,
            to: account(principal([0x22; 32])).bytes(),
            maximum_amount: REFERENCE_SUPPLY,
        }),
    ] {
        let mut grants = base_grants();
        grants.extend(grant);
        cases.push((issuer, INITIALIZE.to_vec(), grants));
    }
    for missing in [
        Capability::SharedStorageRead,
        Capability::SharedStorageWrite,
    ] {
        cases.push((
            issuer,
            INITIALIZE.to_vec(),
            correct
                .iter()
                .filter(|grant| **grant != missing)
                .cloned()
                .collect(),
        ));
    }
    let mut malformed = INITIALIZE.to_vec();
    malformed.push(0);
    cases.push((issuer, malformed, correct.clone()));
    for (caller, input, grants) in cases {
        let before = token.storage.clone();
        let record = token.invoke(caller, "initialize", &input, grants);
        refused(&record, &token.storage, &before);
        assert_eq!(token.query(Request::TotalSupply), 0);
        assert_eq!(token.query(Request::BalanceOf { owner: issuer }), 0);
    }
    let initialized = token.initialize();
    let (payload, effects) = success(&initialized);
    assert!(payload.is_empty());
    assert_eq!(effects.transfers.len(), 1);
    assert_eq!(effects.transfers[0].amount, REFERENCE_SUPPLY);
    assert!(matches!(
        effects.transfers[0].source(),
        TransferSource::ProgramFunding { .. }
    ));
    assert_eq!(token.query(Request::TotalSupply), REFERENCE_SUPPLY);
    assert_eq!(
        token.query(Request::BalanceOf { owner: issuer }),
        REFERENCE_SUPPLY
    );
    let before = token.storage.clone();
    let repeated = token.initialize();
    refused(&repeated, &token.storage, &before);
    assert_eq!(token.query(Request::TotalSupply), REFERENCE_SUPPLY);
}
