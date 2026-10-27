use layerx_sdk::programs::NativeProgramCallRequest;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::program_call::NativeProgramCall;

fn bytes(value: &str) -> Result<Vec<u8>, std::num::ParseIntError> {
    value
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let text = String::from_utf8_lossy(pair);
            u8::from_str_radix(&text, 16)
        })
        .collect()
}

#[test]
fn native_request_binds_real_signed_fixture() -> Result<(), Box<dyn std::error::Error>> {
    let fixture: serde_json::Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/native-program-call-v3.json"
    ))?;
    let signed = bytes(
        fixture["signed_activity_hex"]
            .as_str()
            .ok_or("signed fixture missing")?,
    )?;
    let payload = bytes(
        fixture["payload_hex"]
            .as_str()
            .ok_or("payload fixture missing")?,
    )?;
    let native = NativeProgramCall::decode(&payload).map_err(|_| "invalid fixture")?;
    let registry = ModuleRegistry::new(&[ModuleRegistration::new(
        ModuleId::Programs,
        &[ActivityType::new(ModuleId::Programs, 3).map_err(|_| "activity type invalid")?],
    )
    .map_err(|_| "registration invalid")?])
    .map_err(|_| "registry invalid")?;
    let bound = NativeProgramCallRequest::new(&registry, native, 1000, &signed)
        .map_err(|_| "binding failed")?;
    assert_eq!(bound.signed_activity(), signed);
    assert_eq!(
        bound.bound_activity_id().to_vec(),
        bytes(fixture["activity_id_hex"].as_str().ok_or("id missing")?)?
    );
    assert!(NativeProgramCallRequest::new(&registry, native, 999, &signed).is_err());
    assert!(NativeProgramCallRequest::new(
        &registry,
        NativeProgramCall {
            response_capacity: 17,
            ..native
        },
        1000,
        &signed
    )
    .is_err());
    Ok(())
}

#[test]
fn program_http_decodes_signed_program_calls_as_eip1559() -> Result<(), Box<dyn std::error::Error>>
{
    use layerx_crypto::evm_transaction::{
        decode_signed, recover_sender, transaction_hash, Eip1559Call,
    };
    use layerx_sdk::programs::{
        HttpProgramTransport, NativeProgramTransport, ProgramOperationError, ProgramSubmission,
    };
    use sha2::{Digest as _, Sha256};

    let fixture: serde_json::Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/native-program-call-v4.json"
    ))?;
    let payload = bytes(
        fixture["payload_hex"]
            .as_str()
            .ok_or("missing native payload")?,
    )?;
    let native = NativeProgramCall::decode(&payload).map_err(|_| "invalid native payload")?;
    let data = bytes(
        fixture["evm"]["calldata"]
            .as_str()
            .ok_or("missing EVM calldata")?
            .strip_prefix("0x")
            .ok_or("missing calldata prefix")?,
    )?;
    let to: [u8; 20] = bytes(
        fixture["evm"]["to"]
            .as_str()
            .ok_or("missing precompile")?
            .strip_prefix("0x")
            .ok_or("missing target prefix")?,
    )?
    .try_into()
    .map_err(|_| "invalid target")?;
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 2_000_000,
        to,
        data,
    };
    let raw = call.sign(&[0x42; 32])?;
    assert_eq!(raw[0], 2);
    let (decoded, signature) = decode_signed(&raw)?;
    assert_eq!(decoded, call);
    assert_ne!(recover_sender(&decoded, &signature)?, [0; 20]);
    let request = NativeProgramCallRequest::from_signed_transaction(native, 1000, &raw)
        .map_err(|error| format!("transaction binding: {error:?}"))?;
    let hash = transaction_hash(&raw);
    assert_eq!(request.bound_activity_id(), hash);
    assert_eq!(request.signed_activity(), raw);
    let mut idempotency = Sha256::new();
    idempotency.update(b"layerxprograms/idempotency/v1");
    idempotency.update(hash);
    idempotency.update(0_u32.to_be_bytes());
    idempotency.update([0x54, 0xbd, 0x2b, 0x48]);
    assert_eq!(
        request.bound_idempotency_key(),
        <[u8; 32]>::from(idempotency.finalize())
    );
    assert_ne!(
        hash.to_vec(),
        bytes(
            fixture["activity_id_hex"]
                .as_str()
                .ok_or("missing legacy id")?
        )?
    );

    assert_eq!(
        NativeProgramCallRequest::from_signed_transaction(native, 999, &raw),
        Err(ProgramOperationError::IdentityMismatch)
    );
    for changed in [
        NativeProgramCall {
            response_capacity: native.response_capacity + 1,
            ..native
        },
        NativeProgramCall {
            calldata: b"different",
            ..native
        },
        NativeProgramCall {
            guest_abi: 3,
            ..native
        },
    ] {
        assert_eq!(
            NativeProgramCallRequest::from_signed_transaction(changed, 1000, &raw),
            Err(ProgramOperationError::IdentityMismatch)
        );
    }
    let wrong_target = Eip1559Call {
        to: [0x11; 20],
        ..call.clone()
    }
    .sign(&[0x42; 32])?;
    assert_eq!(
        NativeProgramCallRequest::from_signed_transaction(native, 1000, &wrong_target),
        Err(ProgramOperationError::IdentityMismatch)
    );
    let mut malformed = raw.clone();
    malformed.pop();
    let mut trailing = raw.clone();
    trailing.push(0);
    let mut wrong_type = raw.clone();
    wrong_type[0] = 1;
    let legacy = bytes(
        fixture["signed_activity_hex"]
            .as_str()
            .ok_or("missing legacy envelope")?,
    )?;
    for rejected in [&malformed, &trailing, &wrong_type, &legacy] {
        assert_eq!(
            NativeProgramCallRequest::from_signed_transaction(native, 1000, rejected),
            Err(ProgramOperationError::Decode)
        );
    }
    for index in [4 + 2 * 32, call.data.len() - 1] {
        let mut changed = call.clone();
        changed.data[index] ^= 1;
        let raw = changed.sign(&[0x42; 32])?;
        assert_eq!(
            NativeProgramCallRequest::from_signed_transaction(native, 1000, &raw),
            Err(ProgramOperationError::Decode)
        );
    }
    let mut changed = call.clone();
    changed.data.push(0);
    assert_eq!(
        NativeProgramCallRequest::from_signed_transaction(
            native,
            1000,
            &changed.sign(&[0x42; 32])?
        ),
        Err(ProgramOperationError::Decode)
    );

    let transport = HttpProgramTransport::connect("http://127.0.0.1:0", None, [0x31; 32], || Ok(0))
        .map_err(|error| format!("HTTP transport: {error:?}"))?;
    let submission = transport
        .submit_native(&request, request.bound_idempotency_key())
        .map_err(|error| format!("HTTP submission: {error:?}"))?;
    let ProgramSubmission::Unknown {
        activity_id,
        idempotency_key,
        retained_signed_activity,
    } = submission
    else {
        return Err("unavailable HTTP endpoint must retain the submitted transaction".into());
    };
    assert_eq!(activity_id, hash);
    assert_eq!(idempotency_key, request.bound_idempotency_key());
    assert_eq!(retained_signed_activity.as_deref(), Some(raw.as_slice()));
    Ok(())
}
