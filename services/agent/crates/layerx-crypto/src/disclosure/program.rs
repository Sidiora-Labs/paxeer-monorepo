use layerx_types::intent::{ProgramCall, ProgramId, PROGRAM_CALL_PAYLOAD_DOMAIN};
use layerx_types::program_call::{NativeProgramCall, Resources};
use layerx_types::program_lifecycle::{
    NativeProgramDeploy, NativeProgramUpgrade, NativeProgramWindDown, ProgramUpgradePolicy,
    ProgramWindDownOperation,
};
use sha2::{Digest, Sha256};

use super::call::{abi_tail, abi_word, ledger_selector, tail, uint, word};
use super::{
    Activity, DisclosedNativeOperation, DisclosureError, DisclosureFields, Encoder, Expiry,
};

const PROGRAM_CALL_SIGNATURE: &str =
    "call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)";
const PROGRAM_CALL_HEAD_BYTES: usize = 15 * 32;
const MAX_PROGRAM_CALL_BYTES: usize = 2_163_428;

/// Encodes the Programs precompile ABI, including its separate execution fee budget.
///
/// # Panics
/// Panics if `call` is not a valid native program call.
#[must_use]
pub fn encode_program_call(call: &NativeProgramCall<'_>, fee_limit: u128) -> Vec<u8> {
    assert!(call.encode().is_ok(), "invalid native program call");
    let mut head = ledger_selector(PROGRAM_CALL_SIGNATURE).to_vec();
    head.extend_from_slice(&call.program_id.bytes());
    head.extend_from_slice(&abi_word(call.guest_abi.into()));
    let mut tails = Vec::new();
    for part in [
        call.entrypoint,
        call.calldata,
        call.capabilities,
        call.access_declaration,
    ] {
        head.extend_from_slice(&abi_word((PROGRAM_CALL_HEAD_BYTES + tails.len()) as u128));
        tails.extend_from_slice(&abi_tail(part));
    }
    head.extend_from_slice(&abi_word(call.response_capacity.into()));
    for ceiling in call.resources.0 {
        head.extend_from_slice(&abi_word(ceiling.into()));
    }
    head.extend_from_slice(&abi_word(fee_limit));
    head.extend_from_slice(&tails);
    head
}

/// Decodes only the canonical fifteen-word head and four ordered, zero-padded tails.
///
/// # Errors
/// Refuses invalid native calls, integer overflow, noncanonical offsets or padding,
/// truncation and trailing bytes as `MalformedPayload`.
pub fn decode_program_call(data: &[u8]) -> Result<(DisclosedProgramCall, u128), DisclosureError> {
    if !(484..=MAX_PROGRAM_CALL_BYTES).contains(&data.len())
        || data[..4] != ledger_selector(PROGRAM_CALL_SIGNATURE)
    {
        return Err(DisclosureError::MalformedPayload);
    }
    let args = &data[4..];
    let mut cursor = PROGRAM_CALL_HEAD_BYTES;
    let entrypoint = tail(args, 2, &mut cursor)?;
    let calldata = tail(args, 3, &mut cursor)?;
    let capabilities = tail(args, 4, &mut cursor)?;
    let access_declaration = tail(args, 5, &mut cursor)?;
    if cursor != args.len() {
        return Err(DisclosureError::MalformedPayload);
    }
    let mut resources = [0; 7];
    for (index, ceiling) in resources.iter_mut().enumerate() {
        *ceiling = u64::from_be_bytes(uint(word(args, index + 7)?)?);
    }
    let native = NativeProgramCall {
        program_id: ProgramId::new(word(args, 0)?),
        guest_abi: u16::from_be_bytes(uint(word(args, 1)?)?),
        entrypoint,
        calldata,
        capabilities,
        access_declaration,
        response_capacity: u32::from_be_bytes(uint(word(args, 6)?)?),
        resources: Resources(resources),
    };
    native
        .encode()
        .map_err(|_| DisclosureError::MalformedPayload)?;
    let fee_limit = u128::from_be_bytes(uint(word(args, 14)?)?);
    if encode_program_call(&native, fee_limit) != data {
        return Err(DisclosureError::MalformedPayload);
    }
    Ok((
        DisclosedProgramCall {
            program_id: native.program_id,
            guest_abi: native.guest_abi,
            entrypoint: entrypoint.to_vec(),
            calldata: calldata.to_vec(),
            capabilities: capabilities.to_vec(),
            access_declaration: access_declaration.to_vec(),
            response_capacity: native.response_capacity,
            resources: native.resources,
        },
        fee_limit,
    ))
}

// PAXAI's fixed prefix carries expiry and retry identity independently of the
// operation payload. Other programs do not have these envelope fields.
pub(super) fn paxai_envelope_terms(
    call: &DisclosedProgramCall,
) -> Result<Option<([u8; 32], Expiry)>, DisclosureError> {
    let data = &call.calldata;
    if !data.starts_with(b"PAXAI1") {
        return Ok(None);
    }
    let malformed = || DisclosureError::MalformedPayload;
    if !(239..=16_384).contains(&data.len())
        || data[6..8] != [0, 1]
        || data[42..74] != call.program_id.bytes()
    {
        return Err(malformed());
    }
    let payload_length = u32::from_be_bytes(data[234..238].try_into().map_err(|_| malformed())?);
    let auth_offset = 238_usize
        .checked_add(usize::try_from(payload_length).map_err(|_| malformed())?)
        .ok_or_else(malformed)?;
    let auth_length = match data.get(auth_offset) {
        Some(0) => 1,
        Some(1) => 97,
        _ => return Err(malformed()),
    };
    if payload_length > 15_965 || auth_offset.checked_add(auth_length) != Some(data.len()) {
        return Err(malformed());
    }
    let not_after = u64::from_be_bytes(data[194..202].try_into().map_err(|_| malformed())?);
    let idempotency_key = data[202..234].try_into().map_err(|_| malformed())?;
    if not_after == 0 || idempotency_key == [0; 32] {
        return Err(malformed());
    }
    Ok(Some((
        idempotency_key,
        Expiry {
            not_before: 0,
            not_after,
            payload_expires_at: not_after,
        },
    )))
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DisclosedProgramDeploy {
    pub program_id: ProgramId,
    pub guest_abi: u16,
    pub policy: ProgramUpgradePolicy,
    pub new_hash: [u8; 32],
    pub interface: Option<Vec<u8>>,
    pub wasm: Vec<u8>,
}

impl DisclosedProgramDeploy {
    #[must_use]
    pub fn native(&self) -> NativeProgramDeploy<'_> {
        NativeProgramDeploy {
            program_id: self.program_id,
            guest_abi: self.guest_abi,
            policy: self.policy,
            new_hash: self.new_hash,
            interface: self.interface.as_deref(),
            wasm: &self.wasm,
        }
    }

    pub(super) fn encode_audit(&self, out: &mut Encoder) -> Result<(), DisclosureError> {
        self.native()
            .encode()
            .map_err(|_| DisclosureError::MalformedPayload)?;
        out.u8(9)?;
        out.fixed(&self.program_id.bytes())?;
        out.u16(self.guest_abi)?;
        match self.policy {
            ProgramUpgradePolicy::Immutable => out.u8(0)?,
            ProgramUpgradePolicy::Authority(authority) => {
                out.u8(1)?;
                out.fixed(&authority)?;
            }
        }
        out.fixed(&self.new_hash)?;
        option_commitment(out, self.interface.as_deref())?;
        commitment(out, &self.wasm)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DisclosedProgramUpgrade {
    pub program_id: ProgramId,
    pub guest_abi: u16,
    pub old_hash: [u8; 32],
    pub new_hash: [u8; 32],
    pub migration_hook: Vec<u8>,
    pub clear_interface: bool,
    pub interface: Option<Vec<u8>>,
    pub wasm: Vec<u8>,
}

impl DisclosedProgramUpgrade {
    #[must_use]
    pub fn native(&self) -> NativeProgramUpgrade<'_> {
        NativeProgramUpgrade {
            program_id: self.program_id,
            guest_abi: self.guest_abi,
            old_hash: self.old_hash,
            new_hash: self.new_hash,
            migration_hook: &self.migration_hook,
            clear_interface: self.clear_interface,
            interface: self.interface.as_deref(),
            wasm: &self.wasm,
        }
    }

    pub(super) fn encode_audit(&self, out: &mut Encoder) -> Result<(), DisclosureError> {
        self.native()
            .encode()
            .map_err(|_| DisclosureError::MalformedPayload)?;
        out.u8(10)?;
        out.fixed(&self.program_id.bytes())?;
        out.u16(self.guest_abi)?;
        out.fixed(&self.old_hash)?;
        out.fixed(&self.new_hash)?;
        out.bytes(&self.migration_hook, 128)?;
        out.u8(u8::from(self.clear_interface))?;
        option_commitment(out, self.interface.as_deref())?;
        commitment(out, &self.wasm)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DisclosedProgramCall {
    pub program_id: ProgramId,
    pub guest_abi: u16,
    pub entrypoint: Vec<u8>,
    pub calldata: Vec<u8>,
    pub capabilities: Vec<u8>,
    pub access_declaration: Vec<u8>,
    pub response_capacity: u32,
    pub resources: Resources,
}

impl DisclosedProgramCall {
    #[must_use]
    pub fn native(&self) -> NativeProgramCall<'_> {
        NativeProgramCall {
            program_id: self.program_id,
            guest_abi: self.guest_abi,
            entrypoint: &self.entrypoint,
            calldata: &self.calldata,
            capabilities: &self.capabilities,
            access_declaration: &self.access_declaration,
            response_capacity: self.response_capacity,
            resources: self.resources,
        }
    }

    pub(super) fn encode_audit(&self, out: &mut Encoder) -> Result<(), DisclosureError> {
        self.native()
            .encode()
            .map_err(|_| DisclosureError::MalformedPayload)?;
        out.u8(11)?;
        out.fixed(&self.program_id.bytes())?;
        out.u16(self.guest_abi)?;
        out.bytes(&self.entrypoint, 128)?;
        out.u32(self.response_capacity)?;
        for ceiling in self.resources.0 {
            out.u64(ceiling)?;
        }
        commitment(out, &self.calldata)?;
        commitment(out, &self.capabilities)?;
        commitment(out, &self.access_declaration)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DisclosedProgramWindDownOperation {
    Route {
        account: [u8; 32],
        asset: [u8; 32],
        destination: [u8; 32],
        seed: Vec<u8>,
    },
    Deprecate {
        exit_program: [u8; 32],
        deadline_batch: u64,
    },
    Tombstone,
    Exit {
        account: [u8; 32],
    },
    BoundedExit {
        account: [u8; 32],
        maximum_exit_amount: u128,
    },
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DisclosedProgramWindDown {
    pub program_id: ProgramId,
    pub operation: DisclosedProgramWindDownOperation,
}

impl DisclosedProgramWindDown {
    #[must_use]
    pub fn native(&self) -> NativeProgramWindDown<'_> {
        let operation = match &self.operation {
            DisclosedProgramWindDownOperation::Route {
                account,
                asset,
                destination,
                seed,
            } => ProgramWindDownOperation::Route {
                account: *account,
                asset: *asset,
                destination: *destination,
                seed,
            },
            DisclosedProgramWindDownOperation::Deprecate {
                exit_program,
                deadline_batch,
            } => ProgramWindDownOperation::Deprecate {
                exit_program: *exit_program,
                deadline_batch: *deadline_batch,
            },
            DisclosedProgramWindDownOperation::Tombstone => ProgramWindDownOperation::Tombstone,
            DisclosedProgramWindDownOperation::Exit { account } => {
                ProgramWindDownOperation::Exit { account: *account }
            }
            DisclosedProgramWindDownOperation::BoundedExit {
                account,
                maximum_exit_amount,
            } => ProgramWindDownOperation::BoundedExit {
                account: *account,
                maximum_exit_amount: *maximum_exit_amount,
            },
        };
        NativeProgramWindDown {
            program_id: self.program_id,
            operation,
        }
    }

    pub(super) fn encode_audit(&self, out: &mut Encoder) -> Result<(), DisclosureError> {
        let encoded = self
            .native()
            .encode()
            .map_err(|_| DisclosureError::MalformedPayload)?;
        out.u8(12)?;
        out.bytes(&encoded, 512)?;
        Ok(())
    }
}

pub(super) fn encode_legacy(call: &ProgramCall, out: &mut Encoder) -> Result<(), DisclosureError> {
    out.u8(13)?;
    out.fixed(&call.callee().bytes())?;
    out.u64(call.budget().fuel())?;
    out.fixed(&call.budget().fee_limit().to_be_bytes())?;
    out.sequence_length(call.capabilities().as_slice().len(), 5)?;
    for capability in call.capabilities().as_slice() {
        out.u8(capability.tag())?;
    }
    commitment(out, call.calldata().as_bytes())
}

fn commitment(out: &mut Encoder, value: &[u8]) -> Result<(), DisclosureError> {
    out.u32(u32::try_from(value.len()).map_err(|_| DisclosureError::MalformedPayload)?)?;
    let digest: [u8; 32] = Sha256::digest(value).into();
    out.fixed(&digest)?;
    Ok(())
}

fn option_commitment(out: &mut Encoder, value: Option<&[u8]>) -> Result<(), DisclosureError> {
    if let Some(value) = value {
        out.u8(1)?;
        commitment(out, value)
    } else {
        out.u8(0)?;
        Ok(())
    }
}

fn wind_down_operation(
    operation: ProgramWindDownOperation<'_>,
) -> DisclosedProgramWindDownOperation {
    match operation {
        ProgramWindDownOperation::Route {
            account,
            asset,
            destination,
            seed,
        } => DisclosedProgramWindDownOperation::Route {
            account,
            asset,
            destination,
            seed: seed.to_vec(),
        },
        ProgramWindDownOperation::Deprecate {
            exit_program,
            deadline_batch,
        } => DisclosedProgramWindDownOperation::Deprecate {
            exit_program,
            deadline_batch,
        },
        ProgramWindDownOperation::Tombstone => DisclosedProgramWindDownOperation::Tombstone,
        ProgramWindDownOperation::Exit { account } => {
            DisclosedProgramWindDownOperation::Exit { account }
        }
        ProgramWindDownOperation::BoundedExit {
            account,
            maximum_exit_amount,
        } => DisclosedProgramWindDownOperation::BoundedExit {
            account,
            maximum_exit_amount,
        },
    }
}

pub(super) fn fields(activity: &Activity) -> Result<DisclosureFields, DisclosureError> {
    let malformed = || DisclosureError::MalformedPayload;
    let payload = activity.payload();
    let ordinal = activity.activity_type().ordinal();
    let legacy = ordinal == 3 && payload.starts_with(PROGRAM_CALL_PAYLOAD_DOMAIN);
    if !legacy && (activity.protocol_version() != 3 || activity.network_id() == 0) {
        return Err(malformed());
    }
    let operation = match ordinal {
        1 => {
            let value = NativeProgramDeploy::decode(payload).map_err(|_| malformed())?;
            if value.encode().map_err(|_| malformed())? != payload {
                return Err(malformed());
            }
            DisclosedNativeOperation::ProgramDeploy(Box::new(DisclosedProgramDeploy {
                program_id: value.program_id,
                guest_abi: value.guest_abi,
                policy: value.policy,
                new_hash: value.new_hash,
                interface: value.interface.map(<[u8]>::to_vec),
                wasm: value.wasm.to_vec(),
            }))
        }
        2 => {
            let value = NativeProgramUpgrade::decode(payload).map_err(|_| malformed())?;
            if value.encode().map_err(|_| malformed())? != payload {
                return Err(malformed());
            }
            DisclosedNativeOperation::ProgramUpgrade(Box::new(DisclosedProgramUpgrade {
                program_id: value.program_id,
                guest_abi: value.guest_abi,
                old_hash: value.old_hash,
                new_hash: value.new_hash,
                migration_hook: value.migration_hook.to_vec(),
                clear_interface: value.clear_interface,
                interface: value.interface.map(<[u8]>::to_vec),
                wasm: value.wasm.to_vec(),
            }))
        }
        3 if legacy => DisclosedNativeOperation::LegacyProgramCall(Box::new(
            ProgramCall::from_canonical_payload(payload).map_err(|_| malformed())?,
        )),
        3 => {
            let value = NativeProgramCall::decode(payload).map_err(|_| malformed())?;
            if value.encode().map_err(|_| malformed())? != payload {
                return Err(malformed());
            }
            DisclosedNativeOperation::ProgramCall(Box::new(DisclosedProgramCall {
                program_id: value.program_id,
                guest_abi: value.guest_abi,
                entrypoint: value.entrypoint.to_vec(),
                calldata: value.calldata.to_vec(),
                capabilities: value.capabilities.to_vec(),
                access_declaration: value.access_declaration.to_vec(),
                response_capacity: value.response_capacity,
                resources: value.resources,
            }))
        }
        7 => {
            let value = NativeProgramWindDown::decode(payload).map_err(|_| malformed())?;
            if value.encode().map_err(|_| malformed())? != payload {
                return Err(malformed());
            }
            DisclosedNativeOperation::ProgramWindDown(Box::new(DisclosedProgramWindDown {
                program_id: value.program_id,
                operation: wind_down_operation(value.operation),
            }))
        }
        _ => {
            return Err(DisclosureError::UnsupportedActivity(
                activity.activity_type().value(),
            ))
        }
    };
    let bound = activity.timestamp_bound();
    Ok(DisclosureFields {
        activity_type: activity.activity_type(),
        actor: activity.actor_did().to_vec(),
        authority: activity.authority().to_vec(),
        counterparties: Vec::new(),
        amounts: Vec::new(),
        asset: [0; 32],
        fee_limit: activity.fee_limit(),
        expiry: Expiry {
            not_before: bound.not_before,
            not_after: bound.not_after,
            payload_expires_at: bound.not_after,
        },
        idempotency_key: activity.idempotency_key(),
        evm_payout_binding: None,
        withdrawal: None,
        payment: None,
        authority_grant: None,
        session_grant: None,
        onboarding: None,
        native_operation: Some(operation),
    })
}
