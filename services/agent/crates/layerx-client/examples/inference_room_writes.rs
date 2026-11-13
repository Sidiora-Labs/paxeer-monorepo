//! Emits native inference-room writes using the maintained client and program codecs.

use std::fmt::Write as _;
use std::io::{self, Read as _};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::ai_market::{
    inference_market as client, market_call_terms, market_program as market,
    market_value_call_terms, MarketSpendingGrant,
};
use layerx_crypto::disclosure::call::encode_program_call;
use layerx_types::program_call::{NativeProgramCall, Resources};
use market::codec::{self, Envelope, Writer};
use market::policy::{PolicyCommitments, RoomProfile, TaskPolicyV1, TASK_POLICY_BYTES};
use market::state::{ActorSlot, Section};
use market::types::{
    AccountId, AssetId, Authentication, ChainDomain, Digest32, MarketId, Presence, PrincipalId,
    ProgramId, PublicKey32, RequestId, RosterDigest, RubricDigest, Signature64, WorkerId,
};
use serde_json::{json, Value};

#[derive(Debug)]
struct Failure(String);

impl std::fmt::Display for Failure {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(&self.0)
    }
}

macro_rules! convert_error {
    ($($kind:ty),+ $(,)?) => {
        $(impl From<$kind> for Failure {
            fn from(error: $kind) -> Self { Self(error.to_string()) }
        })+
    };
}

convert_error!(
    String,
    &str,
    market::errors::ApplicationError,
    std::io::Error,
    serde_json::Error,
    std::num::ParseIntError,
    std::num::TryFromIntError,
    std::array::TryFromSliceError,
);
type Result<T> = std::result::Result<T, Failure>;

fn hex(bytes: &[u8]) -> String {
    let mut text = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        let _ = write!(text, "{byte:02x}");
    }
    text
}

fn text<'a>(value: &'a Value, field: &str) -> Result<&'a str> {
    value[field]
        .as_str()
        .ok_or_else(|| format!("{field} requires a string").into())
}

fn bytes(value: &Value, field: &str) -> Result<Vec<u8>> {
    let input = text(value, field)?;
    if input.len() % 2 != 0
        || !input
            .bytes()
            .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
    {
        return Err(format!("{field} requires canonical lowercase hex").into());
    }
    (0..input.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&input[i..i + 2], 16).map_err(Into::into))
        .collect()
}

fn fixed<const N: usize>(value: &Value, field: &str) -> Result<[u8; N]> {
    bytes(value, field)?
        .try_into()
        .map_err(|_| format!("{field} requires {N} bytes").into())
}

fn number(value: &Value, field: &str) -> Result<u64> {
    value[field]
        .as_u64()
        .map_or_else(|| text(value, field)?.parse().map_err(Into::into), Ok)
}

fn amount(value: &Value, field: &str) -> Result<u128> {
    text(value, field)?.parse().map_err(Into::into)
}

fn digest(value: &Value, field: &str) -> Result<Digest32> {
    Ok(Digest32::new(fixed(value, field)?)?)
}

fn operation(name: &str) -> Result<market::dispatch::Operation> {
    market::dispatch::OPERATIONS
        .iter()
        .find(|item| item.name == name)
        .map(|item| item.operation)
        .ok_or_else(|| format!("unknown operation {name}").into())
}

fn profile(value: &Value) -> Result<RoomProfile> {
    Ok(RoomProfile {
        minimum_bond: amount(value, "minimum_bond")?,
        canary_ratio_bps: number(value, "canary_ratio_bps")?.try_into()?,
        lap_size_tokens: number(value, "lap_size_tokens")?,
        challenge_window_blocks: number(value, "challenge_window_blocks")?,
        slash_fraction_bps: number(value, "slash_fraction_bps")?.try_into()?,
        host_share_bps: number(value, "host_share_bps")?.try_into()?,
        protocol_share_bps: number(value, "protocol_share_bps")?.try_into()?,
        settlement_account: AccountId::new(fixed(value, "settlement_account")?)?,
        max_generated_tokens: number(value, "max_generated_tokens")?,
        queue_depth: number(value, "queue_depth")?.try_into()?,
        liveness_timeout_ms: number(value, "liveness_timeout_ms")?,
        acknowledgement_window_ms: number(value, "acknowledgement_window_ms")?,
        canary_relative_tolerance_bps: number(value, "canary_relative_tolerance_bps")?
            .try_into()?,
    })
}

fn create_market(value: &Value) -> Result<Value> {
    let program = ProgramId::new(fixed(value, "program")?)?;
    let chain = ChainDomain::new(market::merged_chain_domain(125))?;
    let asset = AssetId::new(fixed(value, "asset")?)?;
    let rewards = market::registry::derive_rewards_account(program, asset)?;
    let commitment = fixed(value, "commitment_digest")?;
    let policy = TaskPolicyV1::bounded_default(
        1,
        1,
        PolicyCommitments {
            model_artifact: digest(value, "model_digest")?,
            dataset_artifact: [0; 32],
            benchmark_suite: Digest32::new(commitment)?,
            rubric: RubricDigest::new(commitment)?,
            task_schema: Digest32::new(commitment)?,
            result_schema: Digest32::new(commitment)?,
            service_terms: Digest32::new(commitment)?,
        },
        amount(value, "epoch_budget_cap")?,
        amount(value, "minimum_epoch_funding")?,
    )?;
    let mut policy_bytes = [0; TASK_POLICY_BYTES];
    policy.encode(&mut policy_bytes)?;
    let mut payload = vec![0; 500];
    let mut writer = Writer::new(&mut payload);
    writer.put(&fixed::<32>(value, "owner")?)?;
    writer.put(asset.as_bytes())?;
    writer.put(rewards.as_bytes())?;
    writer.put(&fixed::<32>(value, "refund")?)?;
    let treasury = if value.get("treasury").is_some() {
        Presence::Present(PrincipalId::new(fixed(value, "treasury")?)?)
    } else {
        Presence::Absent
    };
    writer.presence(&treasury, |out, principal| out.put(principal.as_bytes()))?;
    writer.put(&policy_bytes)?;
    writer.put(&commitment)?;
    let length = writer.len();
    payload.truncate(length);
    Ok(
        json!({"payload":hex(&payload),"selector":market::dispatch::CREATE.selector(),
              "market":hex(codec::derive_market(chain,program)?.as_bytes()),
              "rewards":hex(rewards.as_bytes()),"policy_digest":hex(policy.digest()?.as_bytes())}),
    )
}

fn manifest(value: &Value) -> Result<Value> {
    let mut legacy = vec![0; market::workers::MAX_MANIFEST_BYTES];
    let mut writer = Writer::new(&mut legacy);
    writer.u16(1)?;
    for field in ["market", "worker", "owner"] {
        writer.put(&fixed::<32>(value, field)?)?;
    }
    for n in [
        1,
        1,
        2,
        number(value, "valid_from")?,
        number(value, "expiry")?,
    ] {
        writer.u64(n)?;
    }
    writer.put(&fixed::<32>(value, "runtime_digest")?)?;
    writer.u16(1)?;
    writer.u8(1)?;
    writer.u8(1)?;
    for field in [
        "model_digest",
        "commitment_digest",
        "commitment_digest",
        "commitment_digest",
        "commitment_digest",
    ] {
        writer.put(&fixed::<32>(value, field)?)?;
    }
    for n in [1024, 1024, 1024, 1024] {
        writer.u32(n)?;
    }
    writer.u8(1)?;
    writer.u32(1000)?;
    writer.u16(2)?;
    writer.u8(1)?;
    writer.u16(1)?;
    writer.u8(1)?;
    writer.bytes(
        text(value, "endpoint")?.as_bytes(),
        market::workers::MAX_URI_BYTES,
    )?;
    writer.put(&fixed::<32>(value, "commitment_digest")?)?;
    writer.u8(1)?;
    writer.u16(1)?;
    writer.u16(1)?;
    writer.put(&fixed::<32>(value, "commitment_digest")?)?;
    writer.put(&fixed::<32>(value, "commitment_digest")?)?;
    writer.u32(0)?;
    let length = writer.len();
    legacy.truncate(length);
    market::workers::decode_manifest(&legacy)?;
    let region: [u8; 2] = text(value, "region_code")?.as_bytes().try_into()?;
    let capabilities = market::workers::CapabilityFields {
        accelerator_memory_mib: number(value, "accelerator_memory_mib")?.try_into()?,
        system_memory_mib: number(value, "system_memory_mib")?.try_into()?,
        bandwidth_class: number(value, "bandwidth_class")?.try_into()?,
        region_code: region,
        attestation_family: number(value, "attestation_family")?.try_into()?,
        runtime_kind: number(value, "runtime_kind")?.try_into()?,
    };
    let mut encoded = vec![0; market::workers::MAX_MANIFEST_BYTES];
    let length = codec::encode_manifest_v2(
        &market::workers::ManifestV2 {
            manifest_v1: &legacy,
            capabilities,
        },
        &mut encoded,
    )?;
    encoded.truncate(length);
    let summary = market::workers::decode_manifest(&encoded)?;
    Ok(
        json!({"manifest_v1":hex(&legacy),"manifest_v2":hex(&encoded),
              "metadata_digest":hex(summary.digest.as_bytes())}),
    )
}

fn state_info(value: &Value) -> Result<Value> {
    let bytes = bytes(value, "state")?;
    let shared = market::state::decode_shared_state(&bytes)?;
    let policy =
        market::registry_ops::PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?;
    let epoch =
        market::registry::market_clock(policy.header.origin_height, number(value, "height")?)?
            .epoch;
    let admission = market::admission::AdmissionTable::decode(
        market::reputation_transition::joint_table(shared.section(Section::ReputationAdmission)?)?,
    )?;
    let roster =
        market::roster::roster_digest(&admission, admission.current_epoch().unwrap_or(epoch))?;
    let mut actors = serde_json::Map::new();
    for index in 0..43 {
        if let Some(actor) = shared.control.replay.actor(ActorSlot::from_index(index)?) {
            let next = actor.last.map_or(Ok(1), |last| {
                last.sequence.checked_add(1).ok_or("sequence overflow")
            })?;
            actors.insert(hex(actor.principal.as_bytes()), json!(next));
        }
    }
    let worker_bytes = market::evaluators::authority::split_identity_section(
        shared.section(Section::IdentityRoster)?,
    )?
    .0;
    let workers = market::workers::WorkerTable::decode(worker_bytes)?;
    let workers: Vec<_> = workers.iter().map(|worker| json!({
        "worker":hex(worker.worker.as_bytes()),"owner":hex(worker.owner.as_bytes()),
        "delegate":hex(&worker.delegate.0),"metadata":hex(worker.metadata.as_bytes()),
        "generation":worker.generation,"key_version":worker.key_version,
        "metadata_revision":worker.metadata_revision,"valid_from":worker.valid_from,"expiry":worker.expiry,
        "state":worker.state as u8,
    })).collect();
    Ok(
        json!({"market":hex(policy.header.market_id.as_bytes()),"revision":shared.revision,
        "epoch":epoch,"config":policy.header.active_config_version,"roster":hex(roster.as_bytes()),
        "origin_height":policy.header.origin_height,"actors":actors,"workers":workers,
        "admission_epoch":admission.current_epoch(),
        "policy_digest":hex(policy.current.digest()?.as_bytes())}),
    )
}

fn committed_state(value: &Value) -> Result<Value> {
    use layerx_client::rpc::commit::verify_commit_material;
    use layerx_programs_registry::chain_proof::{
        trust_store::TrustRoot,
        validators::{TrustedValidator, ValidatorSet},
        ChainProofVerifier,
    };
    let validators = value["validators"]
        .as_array()
        .ok_or("validators requires array")?
        .iter()
        .map(|validator| {
            Ok(TrustedValidator::new(
                fixed(validator, "public_key")?,
                number(validator, "voting_power")?,
            ))
        })
        .collect::<Result<Vec<_>>>()?;
    let verifier = ChainProofVerifier {
        chain_id: text(value, "chain_id")?.to_owned(),
        validators: ValidatorSet::new(validators).map_err(|error| format!("{error:?}"))?,
    };
    let height = number(value, "height")?;
    let material = &value["commit"];
    let head = verify_commit_material(&material["signedHeader"], &verifier)
        .map_err(|error| format!("{error:?}"))?;
    if u64::try_from(head.height()).ok() != Some(height)
        || head.app_hash().as_slice() != fixed::<32>(value, "native_root")?.as_slice()
    {
        return Err("committed state root differs from projection".into());
    }
    let trust = TrustRoot::from_head(&head, verifier.validators.clone())
        .and_then(|root| root.canonical_bytes())
        .map_err(|error| format!("{error:?}"))?;
    let state = text(value, "state")?;
    let info = state_info(&json!({"state":state,"height":height}))?;
    Ok(json!({"state":state,"info":info,"trust_root":hex(&trust)}))
}

fn deployment_commitment(
    value: &Value,
) -> Result<layerx_programs_registry::chain_proof::decode::CommitmentProof> {
    use layerx_programs_registry::chain_proof::decode::{CommitmentProof, ProofOp};
    let ops = value["ops"]
        .as_array()
        .ok_or("store proof operations required")?
        .iter()
        .map(|op| {
            Ok(ProofOp {
                op_type: text(op, "type")?.to_owned(),
                key: bytes(op, "key")?,
                data: bytes(op, "data")?,
            })
        })
        .collect::<Result<Vec<_>>>()?;
    Ok(CommitmentProof {
        signed_header: bytes(value, "signed_header")?,
        commit: bytes(value, "commit")?,
        validator_set: bytes(value, "validator_set")?,
        store_name: "lxphost".to_owned(),
        key: bytes(value, "key")?,
        value: if value["value"].is_null() {
            None
        } else {
            Some(bytes(value, "value")?)
        },
        proof_ops: ops,
    })
}
fn deployment_merkle(
    value: &Value,
) -> Result<layerx_programs_registry::chain_proof::merkle::SimpleProof> {
    use layerx_programs_registry::chain_proof::merkle::SimpleProof;
    Ok(SimpleProof {
        total: value["total"].as_i64().ok_or("proof total required")?,
        index: value["index"].as_i64().ok_or("proof index required")?,
        leaf_hash: fixed(value, "leaf_hash")?,
        aunts: value["aunts"]
            .as_array()
            .ok_or("proof aunts required")?
            .iter()
            .map(|aunt| fixed(&json!({"aunt":aunt}), "aunt"))
            .collect::<Result<Vec<_>>>()?,
    })
}

fn deployment_admission(value: &Value) -> Result<Value> {
    use layerx_programs_registry as registry;
    use registry::chain_evidence::{
        deployment::ChainDeploymentProof, lifecycle::ChainLifecycleProof,
        program_state::ChainProgramStateProof, wind_down::ChainWindDownProof,
    };
    use registry::chain_proof::{results::TxResult, validators::TrustedValidator};
    use registry::{
        DeploymentExecutionProof, DeploymentProof, ProgramLifecycleProof, ProgramStateProof,
    };

    let program =
        registry::ProgramId::new(fixed(value, "program")?).map_err(|error| format!("{error:?}"))?;
    let current = deployment_commitment(&value["current"])?;
    let interface = deployment_commitment(&value["interface"])?;
    let status = deployment_commitment(&value["status"])?;
    let state = ChainProgramStateProof {
        program,
        ops: current.proof_ops.clone(),
    };
    let deployment =
        ChainDeploymentProof::from_state_proofs(state.clone(), bytes(value, "code")?, &interface)
            .map_err(|error| format!("{error:?}"))?;
    let execution = &value["execution"];
    let result = &execution["result"];
    let execution_proof = DeploymentExecutionProof {
        transaction: bytes(execution, "transaction")?,
        previous: deployment_commitment(&value["previous"])?,
        result: TxResult {
            code: u32::try_from(number(result, "code")?)?,
            data: bytes(result, "data")?,
            gas_wanted: result["gas_wanted"]
                .as_i64()
                .ok_or("result gas wanted required")?,
            gas_used: result["gas_used"]
                .as_i64()
                .ok_or("result gas used required")?,
        },
        result_proof: deployment_merkle(&execution["result_proof"])?,
        interface_ops: interface.proof_ops,
        migration: None,
        migration_ops: Vec::new(),
        migration_previous_ops: Vec::new(),
    };
    let mut proof = DeploymentProof {
        state: ProgramStateProof {
            commitment: current.clone(),
            state,
            lifecycle: ProgramLifecycleProof(ChainLifecycleProof {
                program,
                record: current.value,
                record_ops: current.proof_ops,
                status: status.value.clone(),
                status_ops: status.proof_ops.clone(),
            }),
            wind_down: ChainWindDownProof {
                program,
                value: status.value,
                ops: status.proof_ops,
            },
            transaction: Vec::new(),
            transaction_proof: deployment_merkle(&execution["transaction_proof"])?,
        },
        deployment,
    };
    proof
        .bind_execution(&execution_proof)
        .map_err(|error| format!("{error:?}"))?;
    let validators = value["validators"]
        .as_array()
        .ok_or("deployment validators required")?
        .iter()
        .map(|validator| {
            Ok(TrustedValidator::new(
                fixed(validator, "public_key")?,
                number(validator, "voting_power")?,
            ))
        })
        .collect::<Result<Vec<_>>>()?;
    let verifier = registry::ProtocolDeploymentVerifier::from_chain_trust(
        text(value, "chain_id")?,
        validators,
        120_000,
    )
    .map_err(|error| format!("{error:?}"))?;
    let evidence = verifier
        .verify_deployment(&proof, number(value, "now_ms")?)
        .map_err(|error| format!("deployment admission: {error:?}"))?;
    Ok(json!({"digest":hex(&evidence.receipt_digest()),
        "admission":hex(&proof.canonical_encoding().map_err(|error| format!("{error:?}"))?),
        "deployment":hex(&evidence.record().canonical_encoding())}))
}

fn transition_result(value: &Value) -> Result<Value> {
    use market::dispatch::{Buffers, Routed};
    let current = bytes(value, "state")?;
    let input = if value.get("call").is_some() {
        let (call, _) =
            layerx_crypto::disclosure::call::decode_program_call(&bytes(value, "call")?)
                .map_err(|error| format!("{error:?}"))?;
        call.calldata
    } else {
        bytes(value, "envelope")?
    };
    let decoded = codec::decode_envelope(&input)?;
    let context = market::registry_ops::CallContext {
        chain: decoded.envelope.chain,
        program: decoded.envelope.program,
        principal: PrincipalId::new(fixed(value, "actor")?)?,
        height: number(value, "height")?,
    };
    let mut next = vec![0; market::MAX_STATE_BYTES];
    let mut scratch = vec![0; market::dispatch::SCRATCH_BYTES];
    let mut event = vec![0; market::MAX_EVENT_BYTES];
    let mut result = vec![0; market::MAX_RESULT_BYTES];
    let outcome = market::dispatch::route(
        &context,
        &input,
        Some(&current),
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?;
    let (Routed::Applied { result_len, .. }
    | Routed::Unchanged { result_len }
    | Routed::Refused { result_len }) = outcome;
    let frame = codec::decode_result(&result[..result_len])?;
    Ok(
        json!({"status":frame.status as u16,"error":frame.error.map(market::errors::ApplicationError::code),
              "revision":frame.revision,"result":hex(&result[..result_len])}),
    )
}

fn envelope(value: &Value) -> Result<Value> {
    let op = operation(text(value, "operation")?)?;
    let payload = bytes(value, "payload")?;
    let program = ProgramId::new(fixed(value, "program")?)?;
    let actor = PrincipalId::new(fixed(value, "actor")?)?;
    let chain = ChainDomain::new(market::merged_chain_domain(125))?;
    let mut env = Envelope {
        operation: op,
        chain,
        program,
        market: codec::derive_market(chain, program)?,
        actor,
        epoch: 0,
        config: 1,
        roster: Presence::Absent,
        sequence: u64::from(
            op.metadata().sequence != market::dispatch::SequencePolicy::ObjectLocal,
        ),
        expiry: number(value, "expiry")?,
        request: RequestId::new(fixed(value, "request")?)?,
        payload: &payload,
        authentication: Authentication::Native,
    };
    if value.get("state").is_some() {
        let info = state_info(value)?;
        env.config = number(&info, "config")?;
        env.epoch = number(&info, "epoch")?;
        if op.metadata().sequence == market::dispatch::SequencePolicy::Role {
            env.sequence = info["actors"][hex(actor.as_bytes())].as_u64().unwrap_or(1);
        }
        if (0x0801..=0x0809).contains(&op.selector()) {
            env.epoch = info["admission_epoch"].as_u64().unwrap_or(0);
            if !info["admission_epoch"].is_null() {
                let state = bytes(value, "state")?;
                let shared = market::state::decode_shared_state(&state)?;
                env.roster = Presence::Present(
                    market::epoch::captured_roster(&shared)?
                        .ok_or("opened roster required")?
                        .binding
                        .roster,
                );
            }
        }
        if (0x0b01..=0x0b0b).contains(&op.selector()) {
            env.roster = Presence::Present(RosterDigest::new(fixed(&info, "roster")?)?);
        }
        if matches!(
            op,
            market::dispatch::SEAL_TASK_SET
                | market::dispatch::BeginAggregation
                | market::dispatch::ProcessAggregation
                | market::dispatch::FinalizeAggregation
        ) {
            let state = bytes(value, "state")?;
            let shared = market::state::decode_shared_state(&state)?;
            let frozen = market::epoch::captured_roster(&shared)?
                .ok_or("opened roster required")?
                .binding;
            env.epoch = frozen.epoch;
            env.config = frozen.config.get();
            env.roster = Presence::Present(frozen.roster);
        }
        if op == market::dispatch::OPEN_EPOCH {
            let state = bytes(value, "state")?;
            let mut next = vec![0; market::MAX_STATE_BYTES];
            let mut scratch = vec![0; market::epoch::OPEN_SCRATCH_BYTES];
            let frozen = market::epoch::preview_open(
                &state,
                number(value, "height")?,
                &mut next,
                &mut scratch,
            )?;
            env.epoch = frozen.epoch;
            env.config = frozen.config.get();
            env.roster = Presence::Present(frozen.roster);
        }
    }
    if let Some(sequence) = value.get("sequence") {
        env.sequence = sequence.as_u64().ok_or("sequence requires integer")?;
    }
    let mut encoded = vec![0; market::MAX_ENVELOPE_BYTES];
    let length = codec::encode_envelope(&env, &mut encoded)?;
    encoded.truncate(length);
    let digest = codec::decode_envelope(&encoded)?.request_digest()?;
    if value.get("delegate_seed").is_some() {
        let key = SigningKey::from_bytes(&fixed(value, "delegate_seed")?);
        env.authentication = Authentication::Delegate {
            key: PublicKey32(key.verifying_key().to_bytes()),
            signature: Signature64(key.sign(digest.as_bytes()).to_bytes()),
        };
        encoded.resize(market::MAX_ENVELOPE_BYTES, 0);
        let length = codec::encode_envelope(&env, &mut encoded)?;
        encoded.truncate(length);
    }
    Ok(
        json!({"payload":hex(&encoded),"request_digest":hex(digest.as_bytes()),
        "selector":op.selector(),"sequence":env.sequence,"epoch":env.epoch}),
    )
}

fn native_call(value: &Value) -> Result<Value> {
    let payload = bytes(value, "payload")?;
    let operation = operation(text(value, "operation")?)?;
    let program = fixed(value, "program")?;
    let actor = fixed(value, "actor")?;
    let terms = if matches!(
        operation,
        market::dispatch::FUND | market::dispatch::LockBond | market::dispatch::ChallengeLap
    ) {
        market_value_call_terms(
            operation,
            program,
            actor,
            MarketSpendingGrant::Deposit {
                asset: fixed(value, "asset")?,
                rewards_account: fixed(value, "rewards")?,
                amount: amount(value, "amount")?,
            },
        )
    } else if matches!(
        operation,
        market::dispatch::CLAIM
            | market::dispatch::REFUND_FREE
            | market::dispatch::ADVANCE_CLOSE
            | market::dispatch::RefundBond
            | market::dispatch::ResolveChallenge
    ) {
        market_value_call_terms(
            operation,
            program,
            actor,
            MarketSpendingGrant::Payment {
                owner_program: program,
                seed: bytes(value, "seed")?,
                source_account: fixed(value, "source_account")?,
                asset: fixed(value, "asset")?,
                recipient: fixed(value, "recipient")?,
                amount: amount(value, "amount")?,
            },
        )
    } else {
        market_call_terms(operation, program, actor)
    }
    .map_err(|error| format!("{error:?}"))?;
    let resource_values = value["resources"]
        .as_array()
        .ok_or("resources requires array")?;
    let mut resources = [0; 7];
    if resource_values.len() != resources.len() {
        return Err("seven resources required".into());
    }
    for (out, item) in resources.iter_mut().zip(resource_values) {
        *out = item.as_u64().ok_or("resource requires integer")?;
    }
    let call = NativeProgramCall {
        program_id: layerx_types::intent::ProgramId::new(program),
        guest_abi: 5,
        entrypoint: b"layerx_call",
        calldata: &payload,
        capabilities: &terms.capabilities,
        access_declaration: &terms.access_declaration,
        response_capacity: 16384,
        resources: Resources(resources),
    };
    let native = call.encode().map_err(|error| format!("{error:?}"))?;
    Ok(
        json!({"payload":hex(&encode_program_call(&call,amount(value,"fee_limit")?)),
        "native_call":hex(&native),
        "to":"000000000000000000000000000000000000101d"}),
    )
}

fn worker_payload(name: &str, value: &Value) -> Result<Value> {
    let mut payload = vec![0; market::MAX_PAYLOAD_BYTES];
    let mut writer = Writer::new(&mut payload);
    match name {
        "EnrollWorker" => {
            for field in ["owner", "nonce", "delegate", "metadata_digest"] {
                writer.put(&fixed::<32>(value, field)?)?;
            }
            writer.u64(number(value, "expiry")?)?;
        }
        "AcceptEnrollment" => {
            let state = bytes(value, "state")?;
            let shared = market::state::decode_shared_state(&state)?;
            let policy = market::registry_ops::PolicySection::decode(
                shared.section(Section::PolicyLifecycle)?,
            )?;
            let workers = market::workers::WorkerTable::decode(
                market::evaluators::authority::split_identity_section(
                    shared.section(Section::IdentityRoster)?,
                )?
                .0,
            )?;
            let worker = workers
                .get(WorkerId::new(fixed(value, "worker")?)?)
                .ok_or("worker not found")?;
            let consent = market::workers::consent_digest(
                &policy.header,
                worker.worker,
                worker.owner,
                worker.delegate,
                worker.generation,
                worker.key_version,
                worker.metadata,
                worker.expiry,
            )?;
            let key = SigningKey::from_bytes(&fixed(value, "delegate_seed")?);
            if key.verifying_key().to_bytes() != worker.delegate.0 {
                return Err("delegate seed does not match worker".into());
            }
            writer.put(worker.worker.as_bytes())?;
            writer.u64(worker.generation)?;
            writer.u64(worker.key_version)?;
            writer.put(worker.metadata.as_bytes())?;
            writer.put(&key.sign(consent.as_bytes()).to_bytes())?;
        }
        "PublishMetadata" => {
            let manifest = bytes(value, "manifest")?;
            let summary = market::workers::decode_manifest(&manifest)?;
            writer.put(summary.worker.as_bytes())?;
            writer.u64(
                summary
                    .revision
                    .checked_sub(1)
                    .ok_or("manifest revision underflow")?,
            )?;
            writer.u64(summary.revision)?;
            writer.put(summary.digest.as_bytes())?;
            writer.u64(summary.valid_from)?;
            writer.u64(summary.expiry)?;
            writer.bytes(&manifest, market::workers::MAX_MANIFEST_BYTES)?;
        }
        _ => return Err("unknown worker payload".into()),
    }
    let length = writer.len();
    payload.truncate(length);
    let op = operation(name)?;
    op.validate_payload_length(payload.len())?;
    Ok(json!({"payload":hex(&payload),"selector":op.selector()}))
}

fn evaluator_consent(
    value: &Value,
    header: &market::registry::MarketHeader,
    identity: &[u8],
    evaluator: market::types::EvaluatorId,
    approval: market::admission::Approval,
) -> Result<Vec<u8>> {
    use market::admission::EvaluatorConsent;
    use market::evaluators::authority;
    let grant = authority::evaluator_region(identity)?.stored_grant(evaluator)?;
    let consent = EvaluatorConsent {
        chain: header.deployment_chain_domain,
        program: header.program_id,
        market: header.market_id,
        evaluator,
        owner: grant.principal,
        signing_key: grant.signing_key,
        enrollment_nonce: fixed(value, "nonce")?,
        rubric: grant.rubric,
        approval_digest: approval.digest,
        request: RequestId::new(fixed(value, "request")?)?,
        grant_version: grant.grant_version.get(),
        key_version: grant.key_version.get(),
        effective_epoch: approval.effective_epoch,
        config_version: approval.config_version,
        expiry_height: approval.expiry_height,
    };
    let mut encoded = [0; market::evaluators::model::CONSENT_BYTES];
    consent.encode(&mut encoded)?;
    let signer = SigningKey::from_bytes(&fixed(value, "delegate_seed")?);
    if signer.verifying_key().to_bytes() != grant.signing_key.0 {
        return Err("evaluator signer does not match the grant".into());
    }
    let mut payload = encoded.to_vec();
    payload.extend_from_slice(&signer.sign(consent.digest()?.as_bytes()).to_bytes());
    Ok(payload)
}

fn admission_payload(name: &str, value: &Value) -> Result<Value> {
    use market::admission::{AdmissionTable, Participant};
    use market::evaluators::{authority, codec::encode_grant};

    let state = bytes(value, "state")?;
    let shared = market::state::decode_shared_state(&state)?;
    let policy =
        market::registry_ops::PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?;
    let header = &policy.header;
    let identity = shared.section(Section::IdentityRoster)?;
    let table = AdmissionTable::decode(market::reputation_transition::joint_table(
        shared.section(Section::ReputationAdmission)?,
    )?)?;
    let worker = name == "AdmitWorker" || value["kind"] == "worker";
    let (participant, owner, key, generation, commitment) = if worker {
        let workers =
            market::workers::WorkerTable::decode(authority::split_identity_section(identity)?.0)?;
        let record = workers
            .get(WorkerId::new(fixed(value, "worker")?)?)
            .ok_or("worker not found")?;
        (
            Participant::Worker(record.worker),
            record.owner,
            record.delegate,
            record.key_version,
            record.proposal_digest(header)?,
        )
    } else {
        let owner = PrincipalId::new(fixed(value, "principal")?)?;
        let id = codec::derive_evaluator(header.market_id, owner, fixed(value, "nonce")?)?;
        let grant = authority::evaluator_region(identity)?.stored_grant(id)?;
        let mut encoded = [0; market::evaluators::model::GRANT_BYTES];
        let n = encode_grant(&grant, &mut encoded)?;
        (
            Participant::Evaluator(id),
            owner,
            grant.signing_key,
            grant.key_version.get(),
            codec::domain_hash("PAXAI/evaluator-grant/v1", &encoded[..n])?,
        )
    };
    let mut payload = vec![0; market::MAX_PAYLOAD_BYTES];
    let mut writer = Writer::new(&mut payload);
    if name == "ApproveAdmission" {
        writer.u8(participant.role() as u8)?;
        writer.put(&participant.bytes())?;
        writer.put(owner.as_bytes())?;
        writer.put(&fixed::<32>(value, "nonce")?)?;
        writer.put(&key.0)?;
        writer.u64(generation)?;
        writer.put(commitment.as_bytes())?;
        let effective = if table.current_epoch().is_none() {
            0
        } else {
            market::registry::market_clock(header.origin_height, number(value, "height")?)?
                .epoch
                .checked_add(1)
                .ok_or("admission epoch overflow")?
        };
        writer.u64(effective)?;
        writer.u64(header.active_config_version)?;
        writer
            .u64(market::state::HeightWindow::epoch(header.origin_height, effective, 0, 64)?.end)?;
    } else {
        let approval = table
            .get(participant)
            .and_then(|meta| meta.approval)
            .ok_or("participant approval not found")?;
        if name == "AdmitWorker" {
            writer.put(&participant.bytes())?;
            writer.u64(generation)?;
            writer.u64(approval.effective_epoch)?;
            writer.u64(approval.config_version)?;
            writer.put(approval.digest.as_bytes())?;
        } else {
            let Participant::Evaluator(evaluator) = participant else {
                return Err("evaluator admission requires an evaluator".into());
            };
            writer.put(&evaluator_consent(
                value, header, identity, evaluator, approval,
            )?)?;
        }
    }
    let n = writer.len();
    payload.truncate(n);
    operation(name)?.validate_payload_length(n)?;
    Ok(json!({"payload":hex(&payload)}))
}

fn gateway_key(value: &Value) -> Result<Value> {
    use layerx_platform_gateway::store::{KeyRecord, RedisEndpoint, RedisStore};
    use layerx_platform_gateway::IssuedKey;
    use sha2::{Digest as _, Sha256};
    let hash = |parts: &[&[u8]]| -> Result<String> {
        let mut digest = Sha256::new();
        for part in parts {
            digest.update(
                u64::try_from(part.len())
                    .map_err(|error| error.to_string())?
                    .to_be_bytes(),
            );
            digest.update(part);
        }
        Ok(hex(&digest.finalize()))
    };
    let key = IssuedKey::generate().map_err(|error| format!("{error:?}"))?;
    let salt = hash(&[b"gateway-key-salt-v1", key.id().as_bytes()])?;
    let principal = text(value, "principal")?;
    let store = RedisStore::new(
        RedisEndpoint::parse(text(value, "redis_url")?)?,
        native_tls::Certificate::from_der(&std::fs::read(text(value, "ca_der")?)?)
            .map_err(|error| error.to_string())?,
        text(value, "username")?.to_owned().into(),
        text(value, "password")?.to_owned().into(),
    );
    store.issue_key(
        &KeyRecord {
            key_id: key.id().to_owned(),
            principal_digest: principal.to_owned(),
            secret_digest: hash(&[b"gateway-key-v1", salt.as_bytes(), key.secret().as_bytes()])?,
            salt,
            signer_public_key: text(value, "public_key")?.to_owned(),
            scopes: "program:read,state:read".to_owned(),
            quota_requests: 10000,
            quota_window_seconds: 3600,
            epoch: 1,
            disabled: false,
        },
        &format!(
            "{principal}:{}",
            hash(&[b"room-key-issue", key.id().as_bytes()])?
        ),
    )?;
    Ok(json!({"credential":format!("{}:{}", key.id(), key.secret())}))
}

fn epoch_payload(name: &str, value: &Value) -> Result<Value> {
    match name {
        "FUND" => {
            let request = market::rewards::FundRequest {
                amount: amount(value, "amount")?,
                refund_recipient: AccountId::new(fixed(value, "refund")?)?,
                policy_version: market::rewards::FUNDING_POLICY_VERSION,
                consent: true,
            };
            let mut payload = [0; 57];
            let n = request.encode(&mut payload)?;
            Ok(json!({"payload":hex(&payload[..n])}))
        }
        "ScheduleEvaluator" => {
            let state = bytes(value, "state")?;
            let shared = market::state::decode_shared_state(&state)?;
            let policy = market::registry_ops::PolicySection::decode(
                shared.section(Section::PolicyLifecycle)?,
            )?;
            let mut payload = [0; market::evaluators::authority::SCHEDULE_PAYLOAD_BYTES];
            let mut writer = Writer::new(&mut payload);
            writer.put(&fixed::<32>(value, "principal")?)?;
            writer.put(&fixed::<32>(value, "nonce")?)?;
            writer.put(policy.current.commitments.rubric.as_bytes())?;
            writer.put(&fixed::<32>(value, "peer_key")?)?;
            for number in [1, 1, 0, 32] {
                writer.u64(number)?;
            }
            Ok(json!({"payload":hex(&payload)}))
        }
        _ => Err("unknown epoch payload".into()),
    }
}

fn epoch_journey(name: &str, value: &Value) -> Result<Value> {
    let state = bytes(value, "state")?;
    let shared = market::state::decode_shared_state(&state)?;
    let policy =
        market::registry_ops::PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?;
    let frozen = market::epoch::captured_roster(&shared)?
        .ok_or("opened roster required")?
        .binding;
    let windows = market::types::EpochWindows::new(policy.header.origin_height, frozen.epoch)?;
    if name == "EpochJourney" {
        return Ok(
            json!({"settlement_height": windows.settlement, "next_epoch_height":
            market::types::EpochWindows::new(policy.header.origin_height, frozen.epoch + 1)?.start}),
        );
    }
    let mut payload = vec![0; market::MAX_PAYLOAD_BYTES];
    let mut writer = Writer::new(&mut payload);
    if name == "SEAL_TASK_SET" {
        let binding = market::tasks::SetBinding {
            market: policy.header.market_id,
            epoch: frozen.epoch,
            config: frozen.config,
            policy: policy.current.digest()?,
            roster: frozen.roster,
        };
        let set = market::tasks::TaskSet::decode(policy.task_region)?;
        writer.u64(frozen.epoch)?;
        writer.u64(frozen.config.get())?;
        writer.put(market::tasks::task_set_digest(&binding, &set)?.as_bytes())?;
    } else {
        let progress = market::aggregation::progress(&state)?;
        if name == "AggregationProgress" {
            return Ok(
                json!({"phase":progress.phase as u8,"cursor":progress.cursor,"workers":progress.worker_count}),
            );
        }
        let Presence::Present(input) = progress.input else {
            return Err("sealed aggregation required".into());
        };
        writer.put(input.as_bytes())?;
        if name == "ProcessAggregation" {
            writer.u16(progress.cursor)?;
        }
    }
    let n = writer.len();
    payload.truncate(n);
    Ok(json!({"payload":hex(&payload)}))
}

fn run(name: &str, value: &Value) -> Result<Value> {
    match name {
        "GatewayKey" => gateway_key(value),
        "Identity" => {
            let public_key = fixed::<32>(value, "public_key")?;
            let did = text(value, "did")?;
            if did != format!("did:layerx:{}", hex(&public_key)) {
                return Err("DID does not bind the declared public key".into());
            }
            let did = layerx_types::ids::Did::new(did.as_bytes())
                .map_err(|error| format!("{error:?}"))?;
            let account_name = format!("agent:{}:main", text(value, "did")?);
            let account = layerx_types::account::AccountId::parse(&account_name)
                .map_err(|error| format!("{error:?}"))?;
            let principal = layerx_wire::hash::did_id_for_protocol(&did, 3)
                .map_err(|error| format!("{error:?}"))?;
            let account = layerx_wire::hash::account_id_for_protocol(&account, 3)
                .map_err(|error| format!("{error:?}"))?;
            Ok(
                json!({"principal":hex(&principal),"main_account":hex(&account),"account_name":account_name}),
            )
        }
        "CreateMarket" => create_market(value),
        "Manifest" => manifest(value),
        "StateInfo" => state_info(value),
        "CommittedState" => committed_state(value),
        "DeploymentAdmission" => deployment_admission(value),
        "TransitionResult" => transition_result(value),
        "Envelope" => envelope(value),
        "NativeCall" => native_call(value),
        "ApproveAdmission" | "AdmitWorker" | "AdmitEvaluator" => admission_payload(name, value),
        "OPEN_EPOCH" | "BeginAggregation" => Ok(json!({"payload":""})),
        "EpochJourney"
        | "AggregationProgress"
        | "SEAL_TASK_SET"
        | "ProcessAggregation"
        | "FinalizeAggregation" => epoch_journey(name, value),
        "FUND" | "ScheduleEvaluator" => epoch_payload(name, value),
        "EnrollWorker" | "AcceptEnrollment" | "PublishMetadata" => worker_payload(name, value),
        "DeriveWorker" => {
            let worker = codec::derive_worker(
                MarketId::new(fixed(value, "market")?)?,
                PrincipalId::new(fixed(value, "owner")?)?,
                fixed(value, "nonce")?,
            )?;
            Ok(json!({"worker":hex(worker.as_bytes())}))
        }
        "OpenRoom" => {
            let op = client::open_room(&client::OpenRoomRequest {
                room_id: digest(value, "room_id")?,
                model_digest: digest(value, "model_digest")?,
                runtime_digest: digest(value, "runtime_digest")?,
                profile: profile(&value["profile"])?,
            })
            .map_err(|error| format!("{error:?}"))?;
            Ok(json!({"payload":hex(op.payload()),"selector":op.operation().selector()}))
        }
        "PostAsk" => {
            let op = client::post_ask(&client::PostAskRequest {
                room_id: digest(value, "room_id")?,
                ask_id: digest(value, "ask_id")?,
                model_digest: digest(value, "model_digest")?,
                maximum_price_per_thousand_tokens: amount(
                    value,
                    "maximum_price_per_thousand_tokens",
                )?,
                stream_id: amount(value, "stream_id")?,
            })
            .map_err(|error| format!("{error:?}"))?;
            Ok(json!({"payload":hex(op.payload()),"selector":op.operation().selector()}))
        }
        "PostBid" => {
            let op = client::post_bid(&client::PostBidRequest {
                room_id: digest(value, "room_id")?,
                bid_id: digest(value, "bid_id")?,
                capability_digest: digest(value, "capability_digest")?,
                minimum_price_per_thousand_tokens: amount(
                    value,
                    "minimum_price_per_thousand_tokens",
                )?,
                bond_amount: amount(value, "bond_amount")?,
                layer_capacity: number(value, "layer_capacity")?.try_into()?,
            })
            .map_err(|error| format!("{error:?}"))?;
            Ok(json!({"payload":hex(op.payload()),"selector":op.operation().selector()}))
        }
        "LockBond" => {
            let request = market::rewards::bonds::LockBondRequest {
                room: digest(value, "room_id")?,
                bid: digest(value, "bid_id")?,
                amount: amount(value, "amount")?,
            };
            let op = client::lock_bond(request.encode()?).map_err(|error| format!("{error:?}"))?;
            Ok(json!({"payload":hex(op.payload()),"selector":op.operation().selector()}))
        }
        _ => Err(format!("unsupported operation {name}").into()),
    }
}

fn main() -> Result<()> {
    let mut args = std::env::args().skip(1);
    let name = args.next().ok_or("named operation required")?;
    if args.next().is_some() {
        return Err("only one operation argument is permitted".into());
    }
    let mut input = String::new();
    io::stdin().take(4_194_305).read_to_string(&mut input)?;
    if input.len() > 4_194_304 {
        return Err("encoder input exceeds bound".into());
    }
    let value = serde_json::from_str(&input)?;
    println!("{}", run(&name, &value)?);
    Ok(())
}
