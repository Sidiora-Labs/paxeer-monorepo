use layerx_crypto::disclosure::call::{
    bind_call, decode_program_call, decode_program_deploy, decode_program_upgrade,
    decode_program_wind_down, encode_program_call, encode_program_deploy, encode_program_upgrade,
    encode_program_wind_down, ledger_selector, reencode, CallFeeContext, ConstructMethod,
};
use layerx_crypto::disclosure::{DisclosedNativeOperation, DisclosureError};
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_types::intent::ProgramId;
use layerx_types::payload::{ActivityType, ModuleId};
use layerx_types::program_call::{NativeProgramCall, Resources};
use layerx_types::program_lifecycle::{
    NativeProgramDeploy, NativeProgramUpgrade, NativeProgramWindDown, ProgramUpgradePolicy,
    ProgramWindDownOperation,
};

type TestResult = Result<(), Box<dyn std::error::Error>>;
const SENDER: [u8; 20] = [0xa1; 20];
const WASM: &[u8] = b"\0asm\x01\0\0\0";
const DEPLOY: &str = "deploy(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes)";
const UPGRADE: &str = "upgrade(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes,bytes,uint128)";
const ROUTE: &str = "setExitRoute(bytes32,bytes32,bytes32,bytes32,bytes)";
const DEPRECATE: &str = "deprecate(bytes32,bytes32,uint64)";
const TOMBSTONE: &str = "tombstone(bytes32)";
const EXIT: &str = "exitValue(bytes32,bytes32)";
const BOUNDED_EXIT: &str = "exitValueBounded(bytes32,bytes32,uint128)";

fn transaction(data: Vec<u8>) -> Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&0x101d_u16.to_be_bytes());
    Eip1559Call {
        chain_id: 125,
        nonce: 9,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
        to,
        data,
    }
}

fn deployment() -> NativeProgramDeploy<'static> {
    NativeProgramDeploy {
        program_id: ProgramId::new([1; 32]),
        guest_abi: 5,
        policy: ProgramUpgradePolicy::Authority([2; 32]),
        new_hash: [3; 32],
        interface: Some(b"interface"),
        wasm: WASM,
    }
}

fn upgrade() -> NativeProgramUpgrade<'static> {
    NativeProgramUpgrade {
        program_id: ProgramId::new([1; 32]),
        guest_abi: 5,
        old_hash: [3; 32],
        new_hash: [4; 32],
        migration_hook: b"migrate",
        clear_interface: false,
        interface: Some(b"new interface"),
        wasm: WASM,
    }
}

fn wind_downs() -> [(NativeProgramWindDown<'static>, &'static str); 5] {
    [
        (
            ProgramWindDownOperation::Route {
                account: [5; 32],
                asset: [6; 32],
                destination: [7; 32],
                seed: b"seed",
            },
            ROUTE,
        ),
        (
            ProgramWindDownOperation::Deprecate {
                exit_program: [8; 32],
                deadline_batch: u64::MAX,
            },
            DEPRECATE,
        ),
        (ProgramWindDownOperation::Tombstone, TOMBSTONE),
        (ProgramWindDownOperation::Exit { account: [5; 32] }, EXIT),
        (
            ProgramWindDownOperation::BoundedExit {
                account: [5; 32],
                maximum_exit_amount: u128::MAX,
            },
            BOUNDED_EXIT,
        ),
    ]
    .map(|(operation, signature)| {
        (
            NativeProgramWindDown {
                program_id: ProgramId::new([1; 32]),
                operation,
            },
            signature,
        )
    })
}

fn assert_fields(call: &Eip1559Call, ordinal: u16, signature: &str) -> TestResult {
    let disclosure = bind_call(call, SENDER)?;
    assert_eq!(
        disclosure.activity_type,
        ActivityType::new(ModuleId::Programs, ordinal)
            .map_err(|_| DisclosureError::MalformedPayload)?
    );
    assert_eq!(disclosure.actor, SENDER);
    assert_eq!(disclosure.authority, SENDER);
    assert_eq!(disclosure.fee_limit, 3_000_000);
    assert_eq!(disclosure.asset, [0; 32]);
    assert!(disclosure.counterparties.is_empty());
    assert!(disclosure.amounts.is_empty());
    assert_eq!(
        disclosure
            .construct_method()?
            .expect("construct method")
            .abi_signature(),
        signature
    );
    assert_eq!(&call.data[..4], &ledger_selector(signature));
    assert_eq!(disclosure.signing_digest()?, call.signing_hash());
    disclosure.validate()?;
    assert_eq!(reencode(&disclosure)?, call.encode_unsigned());
    let mut overflow = call.clone();
    overflow.max_fee_per_gas = u128::MAX;
    assert_eq!(
        bind_call(&overflow, SENDER),
        Err(DisclosureError::MalformedPayload)
    );
    Ok(())
}

#[test]
fn deploy_discloses_policy_interface_and_native_operation() -> TestResult {
    for policy in [
        ProgramUpgradePolicy::Immutable,
        ProgramUpgradePolicy::Authority([2; 32]),
    ] {
        for interface in [None, Some(b"interface".as_slice())] {
            let native = NativeProgramDeploy {
                policy,
                interface,
                ..deployment()
            };
            let call = transaction(encode_program_deploy(&native));
            let decoded = decode_program_deploy(&call.data)?;
            assert_eq!(decoded.native(), native);
            assert_eq!(decoded.native().encode(), native.encode());
            assert_eq!(
                NativeProgramDeploy::decode(&decoded.native().encode().expect("native deploy")),
                Ok(native)
            );
            assert_fields(&call, 1, DEPLOY)?;
            let disclosure = bind_call(&call, SENDER)?;
            assert_eq!(
                disclosure.construct_method()?,
                Some(ConstructMethod::ProgramDeploy {
                    deploy: Box::new(decoded.clone())
                })
            );
            assert_eq!(
                disclosure.native_operation,
                Some(DisclosedNativeOperation::ProgramDeploy(Box::new(decoded)))
            );
            let money = disclosure.call_monetary(CallFeeContext::GasOnly)?;
            assert_eq!(money.gas_fee_bound.amount_wei, 3_000_000);
            assert_eq!(money.execution_fee_bound, None);
        }
    }
    Ok(())
}

#[test]
fn upgrade_discloses_flags_interface_and_separate_execution_fee() -> TestResult {
    for migration_hook in [b"".as_slice(), b"migrate".as_slice()] {
        for (clear_interface, interface) in [
            (false, None),
            (false, Some(b"new interface".as_slice())),
            (true, Some(b"".as_slice())),
            (true, Some(b"new interface".as_slice())),
        ] {
            for fee_limit in [0, u128::MAX] {
                let native = NativeProgramUpgrade {
                    migration_hook,
                    clear_interface,
                    interface,
                    ..upgrade()
                };
                let call = transaction(encode_program_upgrade(&native, fee_limit));
                let (decoded, fee) = decode_program_upgrade(&call.data)?;
                assert_eq!(decoded.native(), native);
                assert_eq!(decoded.native().encode(), native.encode());
                assert_eq!(
                    NativeProgramUpgrade::decode(
                        &decoded.native().encode().expect("native upgrade")
                    ),
                    Ok(native)
                );
                assert_eq!(fee, fee_limit);
                assert_fields(&call, 2, UPGRADE)?;
                let disclosure = bind_call(&call, SENDER)?;
                assert_eq!(
                    disclosure.construct_method()?,
                    Some(ConstructMethod::ProgramUpgrade {
                        upgrade: Box::new(decoded.clone()),
                        fee_limit
                    })
                );
                assert_eq!(
                    disclosure.native_operation,
                    Some(DisclosedNativeOperation::ProgramUpgrade(Box::new(decoded)))
                );
                let money = disclosure
                    .call_monetary(CallFeeContext::ProgramExecution { fee_asset: [6; 32] })?;
                assert_eq!(money.gas_fee_bound.amount_wei, 3_000_000);
                let execution = money.execution_fee_bound.expect("execution budget");
                assert_eq!(execution.asset, [6; 32]);
                assert_eq!(execution.amount_base_units, fee_limit);
                assert_eq!(
                    disclosure.call_monetary(CallFeeContext::GasOnly),
                    Err(DisclosureError::FieldMismatch("execution_fee_asset"))
                );
            }
        }
    }
    Ok(())
}

#[test]
fn all_five_wind_down_selectors_disclose_the_exact_native_operation() -> TestResult {
    for (native, signature) in wind_downs() {
        let call = transaction(encode_program_wind_down(&native));
        let decoded = decode_program_wind_down(&call.data)?;
        assert_eq!(decoded.native(), native);
        assert_eq!(decoded.native().encode(), native.encode());
        assert_eq!(
            NativeProgramWindDown::decode(&decoded.native().encode().expect("native wind-down")),
            Ok(native)
        );
        assert_fields(&call, 7, signature)?;
        let disclosure = bind_call(&call, SENDER)?;
        assert_eq!(
            disclosure.construct_method()?,
            Some(ConstructMethod::ProgramWindDown {
                wind_down: Box::new(decoded.clone())
            })
        );
        assert_eq!(
            disclosure.native_operation,
            Some(DisclosedNativeOperation::ProgramWindDown(Box::new(decoded)))
        );
    }
    Ok(())
}

fn lifecycle_calls() -> Vec<Vec<u8>> {
    let mut calls = vec![
        encode_program_deploy(&deployment()),
        encode_program_upgrade(&upgrade(), u128::MAX),
    ];
    calls.extend(wind_downs().map(|(native, _)| encode_program_wind_down(&native)));
    calls
}

fn assert_refused(data: Vec<u8>) {
    assert_eq!(
        bind_call(&transaction(data), SENDER),
        Err(DisclosureError::MalformedPayload)
    );
}

#[test]
fn lifecycle_calls_refuse_every_truncation_trailing_bytes_and_unknown_selector() {
    for data in lifecycle_calls() {
        for length in 0..data.len() {
            assert_refused(data[..length].to_vec());
        }
        for trailing in [vec![0], vec![0; 32], vec![1; 32]] {
            let mut changed = data.clone();
            changed.extend_from_slice(&trailing);
            assert_refused(changed);
        }
        let mut unknown = data;
        unknown[..4].fill(0);
        assert_refused(unknown);
    }
}

#[test]
fn lifecycle_calls_refuse_noncanonical_offsets_lengths_and_padding() {
    for (data, tail_indices) in [
        (encode_program_deploy(&deployment()), vec![5, 6]),
        (encode_program_upgrade(&upgrade(), 1), vec![5, 6, 7]),
        (encode_program_wind_down(&wind_downs()[0].0), vec![4]),
    ] {
        for index in tail_indices {
            let word_start = 4 + index * 32;
            for byte in word_start..word_start + 32 {
                let mut changed = data.clone();
                changed[byte] ^= 1;
                assert_refused(changed);
            }
            let offset = usize::from_be_bytes(
                data[word_start + 32 - std::mem::size_of::<usize>()..word_start + 32]
                    .try_into()
                    .expect("offset"),
            );
            let length_start = 4 + offset;
            for byte in length_start..length_start + 24 {
                let mut changed = data.clone();
                changed[byte] ^= 1;
                assert_refused(changed);
            }
            let mut oversized = data.clone();
            oversized[length_start..length_start + 32].fill(0xff);
            assert_refused(oversized);
            let length = usize::from_be_bytes(
                data[length_start + 32 - std::mem::size_of::<usize>()..length_start + 32]
                    .try_into()
                    .expect("length"),
            );
            let end = length_start + 32 + length;
            let padded_end = (end - 4).div_ceil(32) * 32 + 4;
            assert!(end < padded_end, "sample tail must include padding");
            for byte in end..padded_end {
                let mut changed = data.clone();
                changed[byte] = 1;
                assert_refused(changed);
            }
        }
    }
}

#[test]
fn lifecycle_calls_refuse_noncanonical_scalar_words_and_invalid_native_terms() {
    for (data, indices) in [
        (encode_program_deploy(&deployment()), vec![1, 2]),
        (encode_program_upgrade(&upgrade(), 1), vec![1, 2, 8]),
        (encode_program_wind_down(&wind_downs()[1].0), vec![2]),
        (encode_program_wind_down(&wind_downs()[4].0), vec![2]),
    ] {
        for index in indices {
            let mut changed = data.clone();
            changed[4 + index * 32] = 1;
            assert_refused(changed);
        }
    }
    for data in lifecycle_calls() {
        let mut zero_program = data;
        zero_program[4..36].fill(0);
        assert_refused(zero_program);
    }
    let mut immutable_with_authority = encode_program_deploy(&deployment());
    immutable_with_authority[4 + 2 * 32 + 31] = 0;
    assert_refused(immutable_with_authority);
    let mut zero_authority = encode_program_deploy(&deployment());
    zero_authority[4 + 3 * 32..4 + 4 * 32].fill(0);
    assert_refused(zero_authority);
    for flags in [0, 4, 0xff] {
        let mut invalid = encode_program_upgrade(&upgrade(), 1);
        invalid[4 + 2 * 32 + 31] = flags;
        assert_refused(invalid);
    }
    let mut bounded_zero = encode_program_wind_down(&wind_downs()[4].0);
    bounded_zero[4 + 2 * 32..4 + 3 * 32].fill(0);
    assert_refused(bounded_zero);
    let data = encode_program_deploy(&deployment());
    let mut invalid_wasm = data.clone();
    let wasm_offset = usize::from_be_bytes(
        data[4 + 7 * 32 - std::mem::size_of::<usize>()..4 + 7 * 32]
            .try_into()
            .expect("wasm offset"),
    );
    invalid_wasm[4 + wasm_offset + 32] = 1;
    assert_refused(invalid_wasm);
}

#[test]
fn nine_argument_call_keeps_its_existing_construct_and_execution_fee() -> TestResult {
    let native = NativeProgramCall {
        program_id: ProgramId::new([1; 32]),
        guest_abi: 5,
        entrypoint: b"layerx_call",
        calldata: &[1, 2, 3],
        capabilities: &[0, 0],
        access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
        response_capacity: 16,
        resources: Resources([1_000_000, 16_777_216, 1_048_576, 64, 7, 4096, u64::MAX]),
    };
    let fee_limit = u128::MAX;
    let call = transaction(encode_program_call(&native, fee_limit));
    let (decoded, fee) = decode_program_call(&call.data)?;
    assert_eq!(decoded.native(), native);
    assert_eq!(fee, fee_limit);
    assert_fields(
        &call,
        3,
        "call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)",
    )?;
    let disclosure = bind_call(&call, SENDER)?;
    assert_eq!(
        disclosure.construct_method()?,
        Some(ConstructMethod::ProgramCall {
            call: Box::new(decoded.clone()),
            fee_limit
        })
    );
    assert_eq!(
        disclosure.native_operation,
        Some(DisclosedNativeOperation::ProgramCall(Box::new(decoded)))
    );
    Ok(())
}
