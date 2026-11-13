//! Solana port integration proofs exercise custody, token and cryptographic host calls.
use layerx_program_sdk::{
    crypto::{Ed25519Message, HashInput, RecoveryId, MAX_HASH_INPUT_BYTES},
    lxt20::{Request, REFERENCE_ASSET, REFERENCE_ISSUER, REFERENCE_METADATA, REFERENCE_SUPPLY},
    payments::PreparedProgramAccount,
    AccountId, Amount, AssetId, Principal,
};
use layerx_programs::hex;
use layerx_programs_runtime::{
    abi::{Calldata, UnavailableReceiptOracle},
    derive_program_account, hash_bytes, AbiError, ActivityBudgetBinding, AuthorizationContext,
    AuthorizedExecutionRequest, BudgetedAuthorizedExecutionRequest, Capability, CapabilitySet,
    CompositionContext, DeclaredBudget, ExecutionError, Executor, HashAlgorithm, PrincipalId,
    ProgramId, Storage, TransferSource, V2AuthorizedExecutionRecord, WasmEngine, ABI_V5_VERSION,
};
use programs_porting_solana::{
    error::PortRefusal,
    monetary::{ProgramAccountTransferPlan, TranslatedValueFlow, ValueFlow},
    reference::program_account_payout,
    wasm::{Code, ModuleBuilder, I32, I32_NE, I64},
};

const GUIDE: &str = include_str!("../MIGRATION.md");
const TOKEN: &[u8] = include_bytes!("../../../fixtures/pay5/token-lxt20.wasm");
const INITIALIZE: &[u8] = &[b'L', b'X', 20, 0, 1, 0x20, 0, 0, 0, 0];

trait Checked<T> {
    #[track_caller]
    fn checked(self) -> T;
}

impl<T, E: std::fmt::Debug> Checked<T> for Result<T, E> {
    fn checked(self) -> T {
        match self {
            Ok(value) => value,
            Err(error) => panic!("expected success: {error:?}"),
        }
    }
}

impl<T> Checked<T> for Option<T> {
    fn checked(self) -> T {
        match self {
            Some(value) => value,
            None => panic!("expected value"),
        }
    }
}

fn program() -> ProgramId {
    ProgramId::new([0x55; 32]).checked()
}

fn principal(bytes: [u8; 32]) -> Principal {
    Principal::new(bytes).checked()
}

fn account(owner: Principal) -> AccountId {
    AccountId::new(
        derive_program_account(program(), &owner.bytes())
            .checked()
            .bytes(),
    )
    .checked()
}

fn storage_grants() -> Vec<Capability> {
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

// The public qualification seam supplies authenticated context at ABI 5. The
// committed ABI-2-importing guest is admitted explicitly at ABI 5; its bytes
// and handlers are unchanged. No subprocess or native harness is needed.
fn invoke(
    wasm: &[u8],
    storage: &mut Storage,
    caller: Principal,
    method: &str,
    input: &[u8],
    grants: Vec<Capability>,
) -> V2AuthorizedExecutionRecord {
    try_invoke(wasm, storage, caller, method, input, grants).checked()
}

fn try_invoke(
    wasm: &[u8],
    storage: &mut Storage,
    caller: Principal,
    method: &str,
    input: &[u8],
    grants: Vec<Capability>,
) -> Result<V2AuthorizedExecutionRecord, ExecutionError> {
    let module = WasmEngine::declared()
        .checked()
        .validate_versioned(ABI_V5_VERSION, wasm)
        .checked();
    let payer = PrincipalId::new(caller.bytes()).checked();
    let binding = ActivityBudgetBinding::new([0x63; 32]).checked();
    let executor = Executor::declared();
    let budget = executor
        .admit_activity_budget_for_qualification(
            DeclaredBudget::protocol_maximum(),
            payer,
            binding,
            u128::MAX,
        )
        .checked();
    executor.execute_authorized_v5_budgeted_for_qualification(
        storage,
        BudgetedAuthorizedExecutionRequest::new(
            AuthorizedExecutionRequest {
                module: &module,
                program: program(),
                authorization: AuthorizationContext::new(
                    payer,
                    CapabilitySet::new(grants).checked(),
                ),
                receipts: &UnavailableReceiptOracle,
                entrypoint: method,
                calldata: input,
                composition: CompositionContext::isolated(),
                response_capacity: 70,
            },
            budget,
            payer,
            binding,
        ),
        1,
        1,
    )
}

fn call(
    storage: &mut Storage,
    caller: Principal,
    request: Request,
    extra: Vec<Capability>,
) -> V2AuthorizedExecutionRecord {
    let mut grants = storage_grants();
    grants.extend(extra);
    invoke(
        TOKEN,
        storage,
        caller,
        request.method(),
        request.encode().checked().as_slice(),
        grants,
    )
}

fn amount(record: &V2AuthorizedExecutionRecord) -> u128 {
    let bytes = &record.response().checked().bytes;
    assert_eq!(&bytes[..6], &[1, 0x20, 0, 0, 0, 16]);
    u128::from_be_bytes(bytes[6..].try_into().checked())
}

fn initialize(storage: &mut Storage) -> V2AuthorizedExecutionRecord {
    let mut grants = storage_grants();
    grants.push(funding());
    invoke(
        TOKEN,
        storage,
        principal(REFERENCE_ISSUER),
        "initialize",
        INITIALIZE,
        grants,
    )
}

fn assert_refused(record: &V2AuthorizedExecutionRecord, storage: &Storage, before: &Storage) {
    assert!(
        record.response().is_none(),
        "unexpected success: {record:?}"
    );
    assert!(
        record.effects().is_none(),
        "refusal retained effects: {record:?}"
    );
    assert_eq!(storage, before);
}

#[test]
fn solana_custody_guide_funds_then_pays_later_with_owner_bound_authority() {
    assert!(GUIDE.contains("pay them out in a later invocation"));
    assert!(GUIDE.contains("owner-program frame"));
    let issuer = principal(REFERENCE_ISSUER);
    let recipient = principal([0x22; 32]);
    let seed = issuer.bytes();
    let prepared = PreparedProgramAccount::new(
        layerx_program_sdk::ProgramId::new(program().bytes()).checked(),
        &seed,
        AssetId::new(REFERENCE_ASSET).checked(),
    )
    .checked();
    assert_eq!(prepared.account(), account(issuer));
    let registration = prepared.registration_payload().checked();
    assert_eq!(&registration.as_slice()[..32], &program().bytes());
    assert_eq!(&registration.as_slice()[32..37], b"LXPA1");
    assert_eq!(&registration.as_slice()[37..69], &REFERENCE_ASSET);
    assert_eq!(&registration.as_slice()[69..73], &32_u32.to_be_bytes());
    assert_eq!(&registration.as_slice()[73..], &seed);

    let mut storage = Storage::new();
    let funded = initialize(&mut storage);
    let legs = &funded.effects().checked().transfers;
    assert_eq!(legs.len(), 1);
    assert_eq!(legs[0].amount, REFERENCE_SUPPLY);
    assert_eq!(legs[0].to, prepared.account().bytes());
    assert!(matches!(
        legs[0].source(),
        TransferSource::ProgramFunding { .. }
    ));

    // Register the recipient in the real token store, then use a distinct
    // invocation to request a bounded payout from the previously funded vault.
    assert!(call(
        &mut storage,
        recipient,
        Request::Approve {
            spender: issuer,
            amount: Amount::ZERO
        },
        vec![]
    )
    .response()
    .is_some());
    let flow = ValueFlow::ProgramAuthorityFunded {
        authority: prepared.account().bytes(),
        recipient: account(recipient).bytes(),
        amount: 40,
    };
    let translated = flow
        .translate_with_program_account(
            REFERENCE_ASSET,
            issuer.bytes(),
            program(),
            &seed,
            prepared.account().bytes(),
        )
        .checked();
    let TranslatedValueFlow::ProgramAccount(plan) = translated else {
        panic!("custody source lost")
    };
    assert_eq!(
        plan,
        program_account_payout(
            program(),
            &seed,
            prepared.account().bytes(),
            REFERENCE_ASSET,
            account(recipient).bytes(),
            40
        )
        .checked()
    );
    let encoded = prepared
        .spend_grant(account(recipient), Amount::from_u128(40))
        .checked();
    assert_eq!(
        CapabilitySet::decode_v2_canonical(encoded.as_slice()).checked(),
        vec![plan.capability()]
    );
    let transfer = Request::Transfer {
        to: account(recipient),
        amount: Amount::from_u128(40),
    };
    assert_custody_refusals(&mut storage, issuer, &seed, &plan, transfer);
    assert_custody_payout(
        &mut storage,
        issuer,
        recipient,
        &seed,
        &plan,
        transfer,
        &flow,
    );
}

fn assert_custody_refusals(
    storage: &mut Storage,
    issuer: Principal,
    seed: &[u8],
    plan: &ProgramAccountTransferPlan,
    transfer: Request,
) {
    let foreign = ProgramId::new([0x66; 32]).checked();
    assert_eq!(
        ProgramAccountTransferPlan::new(
            foreign,
            seed,
            plan.source(),
            REFERENCE_ASSET,
            plan.to(),
            40
        ),
        Err(PortRefusal::InvalidProgramAccount)
    );
    assert_eq!(
        ProgramAccountTransferPlan::new(
            program(),
            b"wrong-seed",
            plan.source(),
            REFERENCE_ASSET,
            plan.to(),
            40
        ),
        Err(PortRefusal::InvalidProgramAccount)
    );
    // A foreign owner is refused at admission, before guest execution can
    // produce a record. Assert that exact boundary and unchanged state.
    let foreign_grant = program_account_payout(
        foreign,
        seed,
        derive_program_account(foreign, seed).checked().bytes(),
        REFERENCE_ASSET,
        plan.to(),
        40,
    )
    .checked()
    .capability();
    let before = storage.clone();
    let mut grants = storage_grants();
    grants.push(foreign_grant);
    let refused = try_invoke(
        TOKEN,
        storage,
        issuer,
        transfer.method(),
        transfer.encode().checked().as_slice(),
        grants,
    );
    assert!(
        matches!(
            refused,
            Err(ExecutionError::Abi(AbiError::InvalidCapability))
        ),
        "foreign owner admission: {refused:?}"
    );
    assert_eq!(*storage, before);
    for grant in [
        None,
        Some(
            program_account_payout(
                program(),
                seed,
                plan.source(),
                REFERENCE_ASSET,
                plan.to(),
                39,
            )
            .checked()
            .capability(),
        ),
        Some(
            program_account_payout(program(), seed, plan.source(), [0x77; 32], plan.to(), 40)
                .checked()
                .capability(),
        ),
    ] {
        let before = storage.clone();
        let refused = call(storage, issuer, transfer, grant.into_iter().collect());
        assert_refused(&refused, storage, &before);
    }
}

fn assert_custody_payout(
    storage: &mut Storage,
    issuer: Principal,
    recipient: Principal,
    seed: &[u8],
    plan: &ProgramAccountTransferPlan,
    transfer: Request,
    flow: &ValueFlow,
) {
    let paid = call(storage, issuer, transfer, vec![plan.capability()]);
    assert!(paid.response().is_some(), "payout: {paid:?}");
    let legs = &paid.effects().checked().transfers;
    assert_eq!(legs.len(), 1);
    assert_eq!(
        (legs[0].asset, legs[0].to, legs[0].amount),
        (plan.asset(), plan.to(), plan.amount())
    );
    let TransferSource::Program(authority) = legs[0].source() else {
        panic!("not a custody payout")
    };
    assert_eq!(authority.owner_program(), program());
    assert_eq!(authority.source_account(), plan.source());
    assert_eq!(authority.seed(), seed);
    assert_eq!(
        amount(&call(
            storage,
            issuer,
            Request::BalanceOf { owner: issuer },
            vec![]
        )),
        REFERENCE_SUPPLY - 40
    );
    assert_eq!(
        amount(&call(
            storage,
            recipient,
            Request::BalanceOf { owner: recipient },
            vec![]
        )),
        40
    );
    assert_eq!(
        flow.translate(REFERENCE_ASSET, issuer.bytes()),
        Err(PortRefusal::LamportMutation)
    );
    assert_eq!(
        ValueFlow::RentSweep {
            recipient: plan.to()
        }
        .translate_with_program_account(
            REFERENCE_ASSET,
            issuer.bytes(),
            program(),
            seed,
            plan.source()
        ),
        Err(PortRefusal::UnboundedRentSweep)
    );
}

#[test]
fn solana_lxt20_guide_initialization_authorization_and_refusal_are_atomic() {
    assert!(GUIDE.contains("eight operations"));
    assert!(GUIDE.contains("separately encoded `initialize`"));
    let issuer = principal(REFERENCE_ISSUER);
    let other = principal([0x22; 32]);
    assert!(Request::decode(INITIALIZE).is_err());
    assert!(Calldata::from_bytes(&INITIALIZE[4..]).is_ok());
    let mut storage = Storage::new();
    assert_eq!(
        amount(&call(&mut storage, issuer, Request::TotalSupply, vec![])),
        0
    );
    for (caller, input, grant) in [
        (other, INITIALIZE.to_vec(), Some(funding())),
        (issuer, INITIALIZE.to_vec(), None),
        (
            issuer,
            INITIALIZE.to_vec(),
            Some(Capability::Transfer402 {
                asset: REFERENCE_ASSET,
                to: account(issuer).bytes(),
                maximum_amount: REFERENCE_SUPPLY - 1,
            }),
        ),
        (
            issuer,
            INITIALIZE.to_vec(),
            Some(Capability::Transfer402 {
                asset: [0x77; 32],
                to: account(issuer).bytes(),
                maximum_amount: REFERENCE_SUPPLY,
            }),
        ),
        (
            issuer,
            INITIALIZE.to_vec(),
            Some(Capability::Transfer402 {
                asset: REFERENCE_ASSET,
                to: account(other).bytes(),
                maximum_amount: REFERENCE_SUPPLY,
            }),
        ),
        (issuer, [INITIALIZE, &[0]].concat(), Some(funding())),
    ] {
        let before = storage.clone();
        let mut grants = storage_grants();
        grants.extend(grant);
        let record = invoke(TOKEN, &mut storage, caller, "initialize", &input, grants);
        assert_refused(&record, &storage, &before);
    }
    let record = initialize(&mut storage);
    assert_eq!(record.response().checked().bytes, [1, 0x20, 0, 0, 0, 0]);
    assert_eq!(
        record.effects().checked().transfers[0].amount,
        REFERENCE_SUPPLY
    );
    assert_eq!(
        amount(&call(&mut storage, issuer, Request::TotalSupply, vec![])),
        REFERENCE_SUPPLY
    );
    assert_eq!(
        amount(&call(
            &mut storage,
            issuer,
            Request::BalanceOf { owner: issuer },
            vec![]
        )),
        REFERENCE_SUPPLY
    );
    let metadata = call(&mut storage, issuer, Request::Metadata, vec![]);
    assert_eq!(
        &metadata.response().checked().bytes[6..],
        REFERENCE_METADATA
    );
    let before = storage.clone();
    let repeated = initialize(&mut storage);
    assert_refused(&repeated, &storage, &before);

    assert_lxt20_requests(&mut storage, issuer, other);
}

fn assert_lxt20_requests(storage: &mut Storage, issuer: Principal, other: Principal) {
    // All seven codec requests travel through the real guest dispatcher, in
    // addition to the separate initialize operation above.
    assert!(call(
        storage,
        other,
        Request::Approve {
            spender: other,
            amount: Amount::ZERO
        },
        vec![]
    )
    .response()
    .is_some());
    assert!(call(
        storage,
        issuer,
        Request::Approve {
            spender: other,
            amount: Amount::from_u128(100)
        },
        vec![]
    )
    .response()
    .is_some());
    let delegated = Request::TransferFrom {
        owner: issuer,
        to: account(other),
        amount: Amount::from_u128(40),
    };
    let before = storage.clone();
    let refused = call(storage, other, delegated, vec![]);
    assert_refused(&refused, storage, &before);
    let plan = program_account_payout(
        program(),
        &issuer.bytes(),
        account(issuer).bytes(),
        REFERENCE_ASSET,
        account(other).bytes(),
        40,
    )
    .checked();
    assert!(call(storage, other, delegated, vec![plan.capability()])
        .response()
        .is_some());
    assert_eq!(
        amount(&call(
            storage,
            other,
            Request::Allowance {
                owner: issuer,
                spender: other
            },
            vec![]
        )),
        60
    );
    let returned = program_account_payout(
        program(),
        &other.bytes(),
        account(other).bytes(),
        REFERENCE_ASSET,
        account(issuer).bytes(),
        15,
    )
    .checked();
    assert!(call(
        storage,
        other,
        Request::Transfer {
            to: account(issuer),
            amount: Amount::from_u128(15)
        },
        vec![returned.capability()]
    )
    .response()
    .is_some());
    assert_eq!(
        amount(&call(
            storage,
            other,
            Request::BalanceOf { owner: other },
            vec![]
        )),
        25
    );
    assert!(call(
        storage,
        issuer,
        Request::Approve {
            spender: other,
            amount: Amount::ZERO
        },
        vec![]
    )
    .response()
    .is_some());
    let before = storage.clone();
    let revoked = call(storage, other, delegated, vec![plan.capability()]);
    assert_refused(&revoked, storage, &before);
}

// Emit an actual guest using the production Solana emitter. Host outputs cross
// linear memory and response_write before the runtime consumer observes them.
fn crypto_guest(
    name: &str,
    args: &[i32],
    segments: &[(u32, &[u8])],
    expected_status: i32,
    output_len: i32,
    spend: bool,
) -> Vec<u8> {
    let mut module = ModuleBuilder::new(1);
    let host_type = module.signature(&vec![I32; args.len()], &[I32]);
    let host = module.import("layerx_v2", name, host_type);
    let response_type = module.signature(&[I32; 3], &[I32]);
    let response = module.import("layerx_v2", "response_write", response_type);
    let transfer_type = module.signature(&[I64, I64, I32, I32, I32, I32], &[I32]);
    let transfer = module.import("layerx_v1", "transfer_402", transfer_type);
    let reserve_type = module.signature(&[I32], &[I32]);
    let mut reserve = Code::new();
    reserve.i32_const(4096);
    reserve.end();
    let reserve = module.function(reserve_type, &[], &reserve);
    module.export_function("layerx_reserve", reserve);
    let entry_type = module.signature(&[I32, I32], &[I32]);
    let mut code = Code::new();
    for arg in args {
        code.i32_const(*arg);
    }
    code.call(host);
    code.i32_const(expected_status);
    code.op(I32_NE);
    code.trap_if();
    if spend {
        code.i64_const(0);
        code.i64_const(1);
        for value in [2048, 32, 2080, 32] {
            code.i32_const(value);
        }
        code.call(transfer);
        code.trap_unless_ok();
    }
    for value in [0, 512, output_len] {
        code.i32_const(value);
    }
    code.call(response);
    code.trap_unless_ok();
    code.i32_const(0);
    code.end();
    let entry = module.function(entry_type, &[], &code);
    module.export_function("layerx_call", entry);
    module.export_memory("memory");
    for (offset, bytes) in segments {
        module.segment(*offset, bytes);
    }
    module.segment(2048, &REFERENCE_ASSET);
    module.segment(2080, &[0x22; 32]);
    module.finish()
}

#[test]
fn solana_crypto_guide_guest_host_calls_preserve_transaction_authority() {
    assert!(GUIDE.contains("Transaction signatures, invoking-principal authentication"));
    assert!(GUIDE.contains("Secp256k1PublicKey"));
    assert!(HashInput::new(&vec![0; MAX_HASH_INPUT_BYTES]).is_ok());
    assert!(HashInput::new(&vec![0; MAX_HASH_INPUT_BYTES + 1]).is_err());
    assert!(Ed25519Message::new(&[0; 64]).is_ok());
    assert!(Ed25519Message::new(&[0; 65]).is_err());
    assert!(RecoveryId::new(3).is_ok());
    assert!(RecoveryId::new(4).is_err());
    let caller = principal([0x33; 32]);
    let mut storage = Storage::new();
    for (id, algorithm) in [
        (1, HashAlgorithm::Sha256),
        (2, HashAlgorithm::Keccak256),
        (3, HashAlgorithm::Blake3),
    ] {
        let guest = crypto_guest("hash", &[id, 0, 3, 512], &[(0, b"abc")], 0, 32, false);
        let record = invoke(&guest, &mut storage, caller, "layerx_call", b"", vec![]);
        assert_eq!(
            record.response().checked().bytes,
            hash_bytes(algorithm, b"abc").checked()
        );
        assert!(record.effects().checked().transfers.is_empty());
    }
    // RFC 8032 section 7.1, TEST 1: fixed Ed25519 key/signature and empty message.
    let key =
        hex::decode("d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a").checked();
    let signature = hex::decode("e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b").checked();
    let args = [1, 128, 0, 0, 32, 32, 64];
    let segments = [(0, key.as_slice()), (32, signature.as_slice())];
    let guest = crypto_guest("signature_verify", &args, &segments, 0, 0, false);
    assert!(
        invoke(&guest, &mut storage, caller, "layerx_call", b"", vec![])
            .response()
            .is_some()
    );
    let mut bad = signature.clone();
    bad[0] ^= 1;
    let invalid = crypto_guest(
        "signature_verify",
        &args,
        &[(0, &key), (32, &bad)],
        0,
        0,
        false,
    );
    let before = storage.clone();
    assert_refused(
        &invoke(&invalid, &mut storage, caller, "layerx_call", b"", vec![]),
        &storage,
        &before,
    );
    let debit = crypto_guest("signature_verify", &args, &segments, 0, 0, true);
    assert_refused(
        &invoke(&debit, &mut storage, caller, "layerx_call", b"", vec![]),
        &storage,
        &before,
    );
    // The same valid signature and debit succeed only with host spending authority.
    let authorized = invoke(
        &debit,
        &mut storage,
        caller,
        "layerx_call",
        b"",
        vec![Capability::Transfer402 {
            asset: REFERENCE_ASSET,
            to: [0x22; 32],
            maximum_amount: 1,
        }],
    );
    assert_eq!(authorized.effects().checked().transfers.len(), 1);
    assert_eq!(
        authorized.effects().checked().transfers[0].source(),
        &TransferSource::Principal(PrincipalId::new(caller.bytes()).checked())
    );

    assert_secp256k1_calls(&mut storage, caller, &before);
}

fn assert_secp256k1_calls(storage: &mut Storage, caller: Principal, before: &Storage) {
    let digest = hash_bytes(HashAlgorithm::Sha256, b"Solana port crypto proof").checked();
    let signing = k256::ecdsa::SigningKey::from_slice(&[9; 32]).checked();
    let (sig, recovery) = signing.sign_prehash_recoverable(&digest).checked();
    let sig = sig.to_bytes();
    for compressed in [true, false] {
        let public = signing.verifying_key().to_encoded_point(compressed);
        let guest = crypto_guest(
            "signature_verify",
            &[
                2,
                0,
                32,
                32,
                i32::try_from(public.as_bytes().len()).checked(),
                128,
                64,
            ],
            &[(0, &digest), (32, public.as_bytes()), (128, &sig)],
            0,
            0,
            false,
        );
        assert!(invoke(&guest, storage, caller, "layerx_call", b"", vec![])
            .response()
            .is_some());
    }
    let guest = crypto_guest(
        "signature_recover",
        &[0, 32, 128, 64, i32::from(recovery.to_byte()), 512, 65],
        &[(0, &digest), (128, &sig)],
        65,
        65,
        false,
    );
    let recovered = invoke(&guest, storage, caller, "layerx_call", b"", vec![]);
    assert_eq!(
        recovered.response().checked().bytes,
        signing.verifying_key().to_encoded_point(false).as_bytes()
    );
    let bad_recovery = crypto_guest(
        "signature_recover",
        &[0, 32, 128, 64, 4, 512, 65],
        &[(0, &digest), (128, &sig)],
        65,
        65,
        false,
    );
    assert_refused(
        &invoke(&bad_recovery, storage, caller, "layerx_call", b"", vec![]),
        storage,
        before,
    );
}
