use super::{
    checked, encoded_disclosure, evm_signing_request, facts, registry, request,
    verify_evm_signature, Host, Result,
};
use layerx_crypto::disclosure::{
    DisclosedNativeBudgetCreate, DisclosedNativeOperation, Disclosure, DisclosureError,
};
use layerx_human_kms::call::{bind_call, CallBinding};
use layerx_types::payload::{ModuleId, Payload};

pub(super) fn mutated_native(disclosure: &Disclosure) -> Vec<Disclosure> {
    let mutations: &[fn(&mut DisclosedNativeBudgetCreate)] = &[
        |v| v.encoding_version ^= 1,
        |v| v.budget_id[0] ^= 1,
        |v| v.budget_account[0] ^= 1,
        |v| v.asset[0] ^= 1,
        |v| v.purpose[0] ^= 1,
        |v| v.per_period_limit += 1,
        |v| v.carry_cap += 1,
        |v| v.initial_amount += 1,
        |v| v.period_length_ms += 1,
        |v| v.period_start_ms += 1,
        |v| v.expiry_ms += 1,
        |v| v.revocation_sequence += 1,
        |v| v.rollover ^= 1,
        |v| v.source_account[0] ^= 1,
        |v| v.source_sequence += 1,
    ];
    let count = match &disclosure.native_operation {
        Some(DisclosedNativeOperation::IdentityRegistration(_)) => 2,
        Some(DisclosedNativeOperation::BudgetCreate(_)) => mutations.len(),
        Some(DisclosedNativeOperation::RecoveryPolicy(_)) => 4,
        Some(
            DisclosedNativeOperation::ProgramDeploy(_)
            | DisclosedNativeOperation::ProgramUpgrade(_)
            | DisclosedNativeOperation::ProgramCall(_)
            | DisclosedNativeOperation::ProgramWindDown(_)
            | DisclosedNativeOperation::LegacyProgramCall(_)
            | DisclosedNativeOperation::BudgetFund(_)
            | DisclosedNativeOperation::BudgetDefund(_)
            | DisclosedNativeOperation::BudgetRevoke(_),
        )
        | None => 0,
    };
    (0..count)
        .map(|field| {
            let mut changed = disclosure.clone();
            match changed.native_operation.as_mut() {
                Some(DisclosedNativeOperation::IdentityRegistration(value)) => match field {
                    0 => value.did_id[0] ^= 1,
                    _ => value.primary_key[0] ^= 1,
                },
                Some(DisclosedNativeOperation::BudgetCreate(value)) => mutations[field](value),
                Some(DisclosedNativeOperation::RecoveryPolicy(value)) => match field {
                    0 => value.did_id[0] ^= 1,
                    1 => value.recovery_root[0] ^= 1,
                    2 => value.threshold += 1,
                    _ => value.delay_bounds = Some((1, 2)),
                },
                Some(
                    DisclosedNativeOperation::ProgramDeploy(_)
                    | DisclosedNativeOperation::ProgramUpgrade(_)
                    | DisclosedNativeOperation::ProgramCall(_)
                    | DisclosedNativeOperation::ProgramWindDown(_)
                    | DisclosedNativeOperation::LegacyProgramCall(_)
                    | DisclosedNativeOperation::BudgetFund(_)
                    | DisclosedNativeOperation::BudgetDefund(_)
                    | DisclosedNativeOperation::BudgetRevoke(_),
                ) => {
                    panic!("unexpected native disclosure with zero mutations")
                }
                None => panic!("missing native disclosure"),
            }
            changed
        })
        .collect()
}

fn sign_setup(
    kind: &str,
    host: &Host,
    (binding, handle, _public): ([u8; 32], &[u8], [u8; 32]),
    (canonical, _call_disclosure): &(Vec<u8>, CallBinding),
    disclosure: &CallBinding,
) -> Result<[u8; 65]> {
    let encoded = disclosure.disclosure_bytes()?;
    let (request, digest) = evm_signing_request(host, binding, handle, canonical, &encoded)?;
    let response = host.call(&request)?;
    assert_eq!(response[7], 0, "native creation {kind} signing refused");
    assert_eq!(
        response.len(),
        73,
        "native creation {kind} signature length"
    );
    verify_evm_signature(host, binding, handle, &digest, &response[8..])?;
    if let Some(shared) = disclosure.shared_disclosure() {
        for changed in mutated_native(shared) {
            assert_eq!(
                changed.reencode(),
                Err(DisclosureError::FieldMismatch("native_operation"))
            );
            assert_eq!(
                host.call(
                    &evm_signing_request(
                        host,
                        binding,
                        handle,
                        canonical,
                        &encoded_disclosure(&changed)?
                    )?
                    .0
                )?[7],
                1
            );
        }
        if shared.onboarding.is_some() {
            assert_eq!(encoded[0], 4);
            let mut changed = encoded.clone();
            let last = changed.len() - 1;
            changed[last] ^= 1;
            assert_eq!(
                host.call(&evm_signing_request(host, binding, handle, canonical, &changed)?.0)?[7],
                1
            );
        }
    } else {
        assert_eq!(encoded[0], 1, "native consent account-commitment view");
        for index in [0, 1, 9, 33, encoded.len() - 1] {
            let mut changed = encoded.clone();
            changed[index] ^= 1;
            assert_eq!(
                host.call(&evm_signing_request(host, binding, handle, canonical, &changed)?.0)?[7],
                1,
                "native consent disclosure corruption {index}"
            );
        }
    }
    Ok(response[8..].try_into()?)
}

#[path = "native_setup_vectors.rs"]
mod vectors;
pub(super) use vectors::setup_call;
use vectors::{creation_cases, creation_limits, native_budget, payload, recovery, CREATION_KINDS};

fn construct_signing_call(
    call: &layerx_crypto::evm_transaction::Eip1559Call,
    from: [u8; 20],
) -> Result<(Vec<u8>, CallBinding)> {
    let bound = bind_call(call, from)?;
    let canonical = bound.reencode()?;
    assert_eq!(
        canonical,
        layerx_human_kms::attestor::unsigned_evm_transaction(call)
    );
    Ok((canonical, bound))
}

fn setup_disclosure(
    _public: [u8; 32],
    payload: &Payload,
    call: &CallBinding,
) -> Result<CallBinding> {
    use layerx_crypto::disclosure::call::ConstructMethod;
    match call.construct_method()? {
        Some(
            ConstructMethod::AuthLifecycle {
                payload: actual, ..
            }
            | ConstructMethod::PayBudgetCreate { payload: actual },
        ) => {
            assert_eq!(actual, payload.as_bytes());
        }
        Some(ConstructMethod::LedgerOpenAccount { asset }) => {
            assert_eq!(asset, payload.as_bytes()[2..34]);
        }
        other => return Err(format!("unexpected native creation method: {other:?}").into()),
    }
    Ok(call.clone())
}

#[test]
fn native_creation_disclosures_sign_through_real_kms_before_and_after_restart() -> Result<()> {
    let mut host = Host::for_bindings(&[75])?;
    let target_binding = [75; 32];
    let (target_handle, target_public) =
        facts(&host.call(&request(1, target_binding, &[], None)?)?)?;
    let target = (target_binding, target_handle.as_slice(), target_public);
    let target_operations = creation_cases(target_public)?;
    assert_eq!(
        target_operations
            .iter()
            .map(|(kind, _)| *kind)
            .collect::<Vec<_>>(),
        CREATION_KINDS,
        "declared native creation inventory"
    );
    let sender = host.sender(target_binding, &target_handle)?;
    let mut calls = Vec::new();

    for (kind, payload) in &target_operations {
        let call = construct_signing_call(
            &setup_call(payload.clone(), creation_limits(payload))?,
            sender,
        )
        .map_err(|error| format!("native creation {kind} refused by bind_call: {error}"))?;
        calls.push((*kind, payload.clone(), call));
    }
    assert_eq!(
        calls.iter().map(|(kind, _, _)| *kind).collect::<Vec<_>>(),
        CREATION_KINDS
    );
    let (initial_kind, initial_payload, initial_call) = &calls[0];
    let initial_disclosure = setup_disclosure(target_public, initial_payload, &initial_call.1)?;
    let signature = sign_setup(
        initial_kind,
        &host,
        target,
        initial_call,
        &initial_disclosure,
    )?;
    let mut first = Vec::new();
    for pass in 0..2 {
        if pass == 1 {
            host.stop();
            host.start()?;
        }
        let mut signatures = Vec::new();
        let mut covered = Vec::new();
        for (kind, payload, _) in &calls {
            let call = construct_signing_call(
                &setup_call(payload.clone(), creation_limits(payload))?,
                host.sender(target_binding, &target_handle)?,
            )
            .map_err(|error| format!("native creation {kind}, pass {pass}: bind_call: {error}"))?;
            let disclosure =
                setup_disclosure(target_public, payload, &call.1).map_err(|error| {
                    format!("native creation {kind}, pass {pass}: disclosure: {error}")
                })?;
            signatures.push(sign_setup(kind, &host, target, &call, &disclosure)?);
            covered.push(*kind);
        }
        assert_eq!(
            covered, CREATION_KINDS,
            "native creation coverage on pass {pass}"
        );
        assert_eq!(signatures.len(), CREATION_KINDS.len());
        assert_eq!(signatures[0], signature);
        if pass == 0 {
            first = signatures;
        } else {
            assert_eq!(signatures, first);
        }
    }
    Ok(())
}

#[test]
fn native_setup_refuses_wrong_accounts_limits_and_payload_shapes() -> Result<()> {
    use layerx_crypto::signer::Signer as _;
    let public = layerx_crypto::local::LocalSigner::new([0x62; 32]).public_key();
    assert_budget_call_rejections()?;
    let host = Host::for_bindings(&[76])?;
    let binding = [76; 32];
    let (handle, _) = facts(&host.call(&request(1, binding, &[], None)?)?)?;
    let sender = host.sender(binding, &handle)?;
    let valid = recovery(false)?;
    let valid_call = setup_call(payload(ModuleId::Governance, 3, &valid)?, (7, 1))?;
    let mut malformed_calls = Vec::new();
    let mut wrong_target = valid_call.clone();
    wrong_target.to[19] = 0x1d;
    malformed_calls.push(wrong_target);
    let mut wrong_selector = valid_call.clone();
    wrong_selector.data[0] ^= 1;
    malformed_calls.push(wrong_selector);
    for index in 0..4 {
        let mut bytes = valid.clone();
        bytes[index] ^= 0x80;
        malformed_calls.push(setup_call(
            payload(ModuleId::Governance, 3, &bytes)?,
            (7, 1),
        )?);
    }
    for length in [3, 69, 1025] {
        let mut bytes = valid.clone();
        bytes.resize(length, 0);
        malformed_calls.push(setup_call(
            payload(ModuleId::Governance, 3, &bytes)?,
            (7, 1),
        )?);
    }
    for (index, call) in malformed_calls.iter().enumerate() {
        assert!(
            bind_call(call, sender).is_err(),
            "lifecycle shape corruption {index} was admitted"
        );
    }
    for extended in [false, true] {
        for (field, start, length) in [
            ("did_id", 4, 32),
            ("recovery_root", 36, 32),
            ("threshold", 68, 2),
        ] {
            let mut bytes = recovery(extended)?;
            bytes[start..start + length].fill(0);
            let call = setup_call(payload(ModuleId::Governance, 3, &bytes)?, (7, 1))?;
            let (canonical, disclosure) = construct_signing_call(&call, sender)?;
            let (request, _) = evm_signing_request(
                &host,
                binding,
                &handle,
                &canonical,
                &disclosure.disclosure_bytes()?,
            )?;
            let response = host.call(&request)?;
            assert_eq!(
                response.as_slice(),
                b"LXKP\0\x07\x05\x01",
                "zeroed recovery {field}, extended={extended} must return only the LXKP Refused response"
            );
        }
    }
    let consent = vectors::historical_consent_payload(public)?;
    for (sequence, fee) in [(1, 0), (0, 1)] {
        let call = setup_call(payload(ModuleId::Governance, 1, &consent)?, (sequence, fee))?;
        assert!(bind_call(&call, [0x41; 20]).is_err());
    }
    Ok(())
}

fn assert_budget_call_rejections() -> Result<()> {
    let unsupported = setup_call(payload(ModuleId::Budget, 1, &native_budget(2)?)?, (7, 1))
        .err()
        .ok_or("version-2 budgets have no budgetCreate call form")?;
    assert!(unsupported.to_string().contains("budget version 2"));
    for version in [1] {
        let original = native_budget(version)?;
        let mut corruptions = Vec::new();
        for start in [2, 34] {
            let mut changed = original.clone();
            changed[start..start + 32].fill(0);
            corruptions.push(changed);
        }
        for (start, length) in [(130, 16), (162, 16), (178, 8), (194, 8), (210, 1)] {
            let mut changed = original.clone();
            changed[start..start + length].fill(0);
            corruptions.push(changed);
        }
        let mut changed = original.clone();
        changed[210] = 3;
        corruptions.push(changed);
        let mut changed = original.clone();
        changed[210] = 1;
        changed[146..162].copy_from_slice(&1_u128.to_be_bytes());
        corruptions.push(changed);
        for (index, bytes) in corruptions.into_iter().enumerate() {
            let call = setup_call(payload(ModuleId::Budget, 1, &bytes)?, (7, 1))?;
            let result = bind_call(&call, [0x41; 20]);
            assert!(
                result.is_err(),
                "budget version {version}, corruption {index} was admitted"
            );
        }
        let mut trailing = setup_call(payload(ModuleId::Budget, 1, &original)?, (7, 1))?;
        trailing.data.push(0);
        assert!(bind_call(&trailing, [0x41; 20]).is_err());
    }
    Ok(())
}
