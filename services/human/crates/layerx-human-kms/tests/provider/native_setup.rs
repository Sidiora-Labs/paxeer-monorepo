use super::{
    checked, construct_signing_call, encoded_disclosure, evm_signing_request, facts, registry,
    request, verify_evm_signature, Host, Result,
};
use layerx_crypto::disclosure::{
    DisclosedNativeBudgetCreate, DisclosedNativeOperation, Disclosure, DisclosureError,
};
use layerx_crypto::onboarding::OnboardingConsent;
use layerx_intents::canonical as hash;
use layerx_types::account::AccountId;
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, Payload};

fn qualification_finding(message: impl std::fmt::Display) -> Result<()> {
    use std::io::Write as _;
    writeln!(std::io::stderr(), "qualification finding: {message}")?;
    Ok(())
}

fn account(name: &str) -> Result<[u8; 32]> {
    checked(hash::account_id_for_protocol(
        &checked(AccountId::parse(name))?,
        3,
    ))
}

fn payload(module: ModuleId, ordinal: u16, bytes: &[u8]) -> Result<Payload> {
    checked(Payload::new(
        &registry()?,
        checked(ActivityType::new(module, ordinal))?,
        bytes,
    ))
}

fn native_budget(version: u16) -> Result<Vec<u8>> {
    checked(layerx_intents::vectors::native_budget_create(
        version,
        account(&format!(
            "agent:did:layerx:alice:budget:{}",
            "08".repeat(32)
        ))?,
        account(&format!("agent:did:layerx:alice:asset:{}", "03".repeat(32)))?,
    ))
}

fn recovery(extended: bool) -> Result<Vec<u8>> {
    checked(layerx_intents::vectors::native_recovery_policy(
        extended,
        checked(hash::did_id_for_protocol(
            &checked(Did::new(b"did:layerx:alice"))?,
            3,
        ))?,
    ))
}

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
        Some(DisclosedNativeOperation::ProgramDeploy(_))
        | Some(DisclosedNativeOperation::ProgramUpgrade(_))
        | Some(DisclosedNativeOperation::ProgramCall(_))
        | Some(DisclosedNativeOperation::ProgramWindDown(_))
        | Some(DisclosedNativeOperation::LegacyProgramCall(_))
        | Some(DisclosedNativeOperation::BudgetFund(_))
        | Some(DisclosedNativeOperation::BudgetDefund(_))
        | Some(DisclosedNativeOperation::BudgetRevoke(_))
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
                Some(DisclosedNativeOperation::ProgramDeploy(_))
                | Some(DisclosedNativeOperation::ProgramUpgrade(_))
                | Some(DisclosedNativeOperation::ProgramCall(_))
                | Some(DisclosedNativeOperation::ProgramWindDown(_))
                | Some(DisclosedNativeOperation::LegacyProgramCall(_))
                | Some(DisclosedNativeOperation::BudgetFund(_))
                | Some(DisclosedNativeOperation::BudgetDefund(_))
                | Some(DisclosedNativeOperation::BudgetRevoke(_)) => {
                    panic!("unexpected native disclosure with zero mutations")
                }
                None => panic!("missing native disclosure"),
            }
            changed
        })
        .collect()
}

fn sign_setup(
    host: &Host,
    (binding, handle, _public): ([u8; 32], &[u8], [u8; 32]),
    (canonical, _call_disclosure): &(Vec<u8>, Disclosure),
    disclosure: &Disclosure,
) -> Result<[u8; 65]> {
    let encoded = encoded_disclosure(disclosure)?;
    let (request, digest) = evm_signing_request(host, binding, handle, canonical, &encoded)?;
    let response = host.call(&request)?;
    assert_eq!(response[7], 0);
    assert_eq!(response.len(), 73);
    verify_evm_signature(host, binding, handle, &digest, &response[8..])?;
    for changed in mutated_native(disclosure) {
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
    if disclosure.onboarding.is_some() {
        assert_eq!(encoded[0], 4);
        let mut changed = encoded.clone();
        let last = changed.len() - 1;
        changed[last] ^= 1;
        assert_eq!(
            host.call(&evm_signing_request(host, binding, handle, canonical, &changed)?.0)?[7],
            1
        );
    }
    Ok(response[8..].try_into()?)
}

pub(super) fn setup_call(
    payload: Payload,
    (sequence, fee): (u64, u128),
) -> Result<layerx_crypto::evm_transaction::Eip1559Call> {
    use layerx_crypto::disclosure::call::ledger_selector;
    let bytes = payload.as_bytes();
    let (suffix, data) = match payload.activity_type().module() {
        ModuleId::Budget => {
            if bytes.len() != 211 {
                let version = bytes
                    .get(..2)
                    .map(|bytes| u16::from_be_bytes([bytes[0], bytes[1]]))
                    .ok_or("truncated budget version")?;
                return Err(format!(
                    "budget version {version} has no budgetCreate call form for {} bytes",
                    bytes.len()
                )
                .into());
            }
            let mut data = ledger_selector("budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)").to_vec();
            for (start, length) in [
                (2, 32),
                (34, 32),
                (66, 32),
                (98, 32),
                (130, 16),
                (146, 16),
                (162, 16),
                (178, 8),
                (186, 8),
                (194, 8),
                (202, 8),
                (210, 1),
            ] {
                let mut word = [0; 32];
                word[32 - length..]
                    .copy_from_slice(bytes.get(start..start + length).ok_or("truncated budget")?);
                data.extend(word);
            }
            (0x1b, data)
        }
        ModuleId::Governance => {
            let mut data = ledger_selector("recordLifecycle(bytes,bytes32)").to_vec();
            let mut offset = [0; 32];
            offset[31] = 64;
            data.extend(offset);
            data.extend([4; 32]);
            let mut length = [0; 32];
            length[24..].copy_from_slice(&(bytes.len() as u64).to_be_bytes());
            data.extend(length);
            data.extend(bytes);
            data.resize(4 + 96 + bytes.len().div_ceil(32) * 32, 0);
            (0x1c, data)
        }
        ModuleId::Asset => {
            let payment = checked(layerx_crypto::payments::Payment::decode(
                ModuleId::Asset,
                payload.activity_type().ordinal(),
                bytes,
                b"did:layerx:alice",
            ))?;
            (
                0x1a,
                checked(layerx_crypto::disclosure::call::encode_payment_call(
                    &payment,
                    b"did:layerx:alice",
                ))?,
            )
        }
        _ => return Err("unsupported setup payload".into()),
    };
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, suffix]);
    Ok(layerx_crypto::evm_transaction::Eip1559Call {
        chain_id: 125,
        nonce: sequence,
        max_priority_fee_per_gas: 0,
        max_fee_per_gas: fee,
        gas_limit: 1,
        to,
        data,
    })
}

fn setup_disclosure(_public: [u8; 32], payload: &Payload, call: &Disclosure) -> Result<Disclosure> {
    use layerx_crypto::disclosure::call::ConstructMethod;
    match call.construct_method()? {
        Some(ConstructMethod::AuthLifecycle {
            payload: actual, ..
        })
        | Some(ConstructMethod::PayBudgetCreate { payload: actual }) => {
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
    // Kernel Governance::OnboardingConsent has no recordLifecycle call form.
    let consent = OnboardingConsent {
        sponsor: checked(hash::did_id_for_protocol(
            &checked(Did::new(b"did:layerx:sponsor"))?,
            3,
        ))?,
        target: checked(Did::new(b"did:layerx:alice"))?,
        target_public_key: target_public,
        native_asset: [3; 32],
        action_key: [4; 32],
        expires_at: 1010,
    };
    let mut target_operations = vec![payload(
        ModuleId::Governance,
        1,
        &checked(consent.payload())?,
    )?];
    for extended in [false, true] {
        target_operations.push(payload(ModuleId::Governance, 3, &recovery(extended)?)?);
    }
    target_operations.push(payload(ModuleId::Budget, 1, &native_budget(1)?)?);
    let opening = checked(
        layerx_crypto::payments::Payment::OpenAccount { asset: [3; 32] }
            .encode(b"did:layerx:alice"),
    )?;
    target_operations.push(payload(ModuleId::Asset, 4, &opening)?);
    let sender = host.sender(target_binding, &target_handle)?;
    let mut calls = Vec::new();

    for payload in &target_operations {
        let activity = payload.activity_type();
        let limits = if activity.module() == ModuleId::Governance && activity.ordinal() == 1 {
            (0, 0)
        } else {
            (7, 1)
        };
        match construct_signing_call(&setup_call(payload.clone(), limits)?, sender) {
            Ok(call) => calls.push((payload.clone(), call)),
            Err(error) => qualification_finding(format!(
                "native creation {activity:?}, payload shape {:02x?}/{} refused by bind_call: {error}",
                &payload.as_bytes()[..4], payload.as_bytes().len()
            ))?,
        }
    }
    assert!(!calls.is_empty(), "no native creation call binds");
    let (initial_payload, initial_call) = &calls[0];
    let initial_disclosure = setup_disclosure(target_public, initial_payload, &initial_call.1)?;
    let signature = sign_setup(&host, target, &initial_call, &initial_disclosure)?;
    let mut first = Vec::new();
    for pass in 0..2 {
        if pass == 1 {
            host.stop();
            host.start()?;
        }
        let mut signatures = Vec::new();
        for (payload, _) in &calls {
            let limits = if payload.activity_type().module() == ModuleId::Governance
                && payload.activity_type().ordinal() == 1
            {
                (0, 0)
            } else {
                (7, 1)
            };
            let call = construct_signing_call(
                &setup_call(payload.clone(), limits)?,
                host.sender(target_binding, &target_handle)?,
            )?;
            let disclosure = setup_disclosure(target_public, payload, &call.1)?;
            signatures.push(sign_setup(&host, target, &call, &disclosure)?);
        }
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
    let unsupported = setup_call(payload(ModuleId::Budget, 1, &native_budget(2)?)?, (7, 1))
        .expect_err("version-2 budgets have no budgetCreate call form");
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
            let result = layerx_crypto::disclosure::call::bind_call(&call, [0x41; 20]);
            assert!(
                result.is_err(),
                "budget version {version}, corruption {index} was admitted"
            );
        }
        let mut trailing = setup_call(payload(ModuleId::Budget, 1, &original)?, (7, 1))?;
        trailing.data.push(0);
        assert!(layerx_crypto::disclosure::call::bind_call(&trailing, [0x41; 20]).is_err());
    }
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
            layerx_crypto::disclosure::call::bind_call(call, sender).is_err(),
            "lifecycle shape corruption {index} was admitted"
        );
    }
    for (start, length) in [(4, 32), (36, 32), (68, 2)] {
        let mut bytes = valid.clone();
        bytes[start..start + length].fill(0);
        let call = setup_call(payload(ModuleId::Governance, 3, &bytes)?, (7, 1))?;
        let (canonical, disclosure) = construct_signing_call(&call, sender)?;
        let (request, digest) = evm_signing_request(
            &host,
            binding,
            &handle,
            &canonical,
            &encoded_disclosure(&disclosure)?,
        )?;
        let response = host.call(&request)?;
        if response[7] == 0 {
            qualification_finding(format!(
                "provider signs recordLifecycle recovery body with bytes {start}..{} zeroed",
                start + length
            ))?;
            verify_evm_signature(&host, binding, &handle, &digest, &response[8..])?;
        } else {
            assert_eq!(response[7], 1, "recovery body corruption refusal");
        }
    }
    let consent = OnboardingConsent {
        sponsor: checked(hash::did_id_for_protocol(
            &checked(Did::new(b"did:layerx:sponsor"))?,
            3,
        ))?,
        target: checked(Did::new(b"did:layerx:alice"))?,
        target_public_key: public,
        native_asset: [3; 32],
        action_key: [4; 32],
        expires_at: 1010,
    };
    for (sequence, fee) in [(1, 0), (0, 1)] {
        let call = setup_call(
            payload(ModuleId::Governance, 1, &checked(consent.payload())?)?,
            (sequence, fee),
        )?;
        assert!(layerx_crypto::disclosure::call::bind_call(&call, [0x41; 20]).is_err());
    }
    Ok(())
}
