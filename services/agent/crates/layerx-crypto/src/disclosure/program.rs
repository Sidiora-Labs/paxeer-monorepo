use layerx_types::intent::{ProgramCall, ProgramId};
use layerx_types::program_call::{NativeProgramCall, Resources};
use layerx_types::program_lifecycle::{
    NativeProgramDeploy, NativeProgramUpgrade, NativeProgramWindDown, ProgramUpgradePolicy,
    ProgramWindDownOperation,
};
use sha2::{Digest, Sha256};

use super::call::{abi_tail, abi_word, ledger_selector, tail, uint, word};
use super::{DisclosureError, Encoder, Expiry};

pub(super) const PROGRAM_CALL_SIGNATURE: &str =
    "call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)";
pub(super) const PROGRAM_DEPLOY_SIGNATURE: &str =
    "deploy(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes)";
pub(super) const PROGRAM_UPGRADE_SIGNATURE: &str =
    "upgrade(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes,bytes,uint128)";
pub(super) const PROGRAM_ROUTE_SIGNATURE: &str =
    "setExitRoute(bytes32,bytes32,bytes32,bytes32,bytes)";
pub(super) const PROGRAM_DEPRECATE_SIGNATURE: &str = "deprecate(bytes32,bytes32,uint64)";
pub(super) const PROGRAM_TOMBSTONE_SIGNATURE: &str = "tombstone(bytes32)";
pub(super) const PROGRAM_EXIT_SIGNATURE: &str = "exitValue(bytes32,bytes32)";
pub(super) const PROGRAM_BOUNDED_EXIT_SIGNATURE: &str = "exitValueBounded(bytes32,bytes32,uint128)";
const PROGRAM_CALL_HEAD_BYTES: usize = 15 * 32;
const MAX_PROGRAM_CALL_BYTES: usize = 2_163_428;

/// Encodes a validated native deployment as a canonical Programs precompile call.
///
/// # Panics
/// Panics if `deploy` is not a valid native deployment.
#[must_use]
pub fn encode_program_deploy(deploy: &NativeProgramDeploy<'_>) -> Vec<u8> {
    assert!(deploy.encode().is_ok(), "invalid native program deployment");
    let (policy, authority) = match deploy.policy {
        ProgramUpgradePolicy::Immutable => (0, [0; 32]),
        ProgramUpgradePolicy::Authority(authority) => (1, authority),
    };
    let interface = abi_tail(deploy.interface.unwrap_or_default());
    [
        ledger_selector(PROGRAM_DEPLOY_SIGNATURE).to_vec(),
        [
            deploy.program_id.bytes(),
            abi_word(deploy.guest_abi.into()),
            abi_word(policy),
            authority,
            deploy.new_hash,
            abi_word(7 * 32),
            abi_word((7 * 32 + interface.len()) as u128),
        ]
        .concat(),
        interface,
        abi_tail(deploy.wasm),
    ]
    .concat()
}

/// Decodes a canonical deployment, refusing malformed ABI or native terms.
///
/// # Errors
/// Returns `MalformedPayload` for invalid words, tails, padding or lengths.
pub fn decode_program_deploy(data: &[u8]) -> Result<DisclosedProgramDeploy, DisclosureError> {
    if data.get(..4) != Some(ledger_selector(PROGRAM_DEPLOY_SIGNATURE).as_slice()) {
        return Err(DisclosureError::MalformedPayload);
    }
    let args = &data[4..];
    let mut cursor = 7 * 32;
    let interface = tail(args, 5, &mut cursor)?;
    let wasm = tail(args, 6, &mut cursor)?;
    if cursor != args.len() {
        return Err(DisclosureError::MalformedPayload);
    }
    let policy = match (uint::<1>(word(args, 2)?)?[0], word(args, 3)?) {
        (0, authority) if authority == [0; 32] => ProgramUpgradePolicy::Immutable,
        (1, authority) if authority != [0; 32] => ProgramUpgradePolicy::Authority(authority),
        _ => return Err(DisclosureError::MalformedPayload),
    };
    let native = NativeProgramDeploy {
        program_id: ProgramId::new(word(args, 0)?),
        guest_abi: u16::from_be_bytes(uint(word(args, 1)?)?),
        policy,
        new_hash: word(args, 4)?,
        interface: (!interface.is_empty()).then_some(interface),
        wasm,
    };
    native
        .encode()
        .map_err(|_| DisclosureError::MalformedPayload)?;
    if encode_program_deploy(&native) != data {
        return Err(DisclosureError::MalformedPayload);
    }
    Ok(DisclosedProgramDeploy {
        program_id: native.program_id,
        guest_abi: native.guest_abi,
        policy,
        new_hash: native.new_hash,
        interface: native.interface.map(<[u8]>::to_vec),
        wasm: wasm.to_vec(),
    })
}

/// Encodes an upgrade and its separate execution fee budget.
///
/// # Panics
/// Panics if `upgrade` is not a valid native upgrade.
#[must_use]
pub fn encode_program_upgrade(upgrade: &NativeProgramUpgrade<'_>, fee_limit: u128) -> Vec<u8> {
    assert!(upgrade.encode().is_ok(), "invalid native program upgrade");
    let hook = abi_tail(upgrade.migration_hook);
    let interface = abi_tail(upgrade.interface.unwrap_or_default());
    let flags =
        u8::from(!upgrade.migration_hook.is_empty()) | (u8::from(upgrade.clear_interface) << 1);
    [
        ledger_selector(PROGRAM_UPGRADE_SIGNATURE).to_vec(),
        [
            upgrade.program_id.bytes(),
            abi_word(upgrade.guest_abi.into()),
            abi_word(flags.into()),
            upgrade.old_hash,
            upgrade.new_hash,
            abi_word(9 * 32),
            abi_word((9 * 32 + hook.len()) as u128),
            abi_word((9 * 32 + hook.len() + interface.len()) as u128),
            abi_word(fee_limit),
        ]
        .concat(),
        hook,
        interface,
        abi_tail(upgrade.wasm),
    ]
    .concat()
}

/// Decodes an upgrade with ordered tails and canonical hook/interface flags.
///
/// # Errors
/// Returns `MalformedPayload` for invalid words, tails, padding or native terms.
pub fn decode_program_upgrade(
    data: &[u8],
) -> Result<(DisclosedProgramUpgrade, u128), DisclosureError> {
    if data.get(..4) != Some(ledger_selector(PROGRAM_UPGRADE_SIGNATURE).as_slice()) {
        return Err(DisclosureError::MalformedPayload);
    }
    let args = &data[4..];
    let mut cursor = 9 * 32;
    let migration_hook = tail(args, 5, &mut cursor)?;
    let interface = tail(args, 6, &mut cursor)?;
    let wasm = tail(args, 7, &mut cursor)?;
    let flags = uint::<1>(word(args, 2)?)?[0];
    if cursor != args.len() || flags & !3 != 0 || (flags & 1 != 0) == migration_hook.is_empty() {
        return Err(DisclosureError::MalformedPayload);
    }
    let clear_interface = flags & 2 != 0;
    let native = NativeProgramUpgrade {
        program_id: ProgramId::new(word(args, 0)?),
        guest_abi: u16::from_be_bytes(uint(word(args, 1)?)?),
        old_hash: word(args, 3)?,
        new_hash: word(args, 4)?,
        migration_hook,
        clear_interface,
        interface: (clear_interface || !interface.is_empty()).then_some(interface),
        wasm,
    };
    native
        .encode()
        .map_err(|_| DisclosureError::MalformedPayload)?;
    let fee_limit = u128::from_be_bytes(uint(word(args, 8)?)?);
    if encode_program_upgrade(&native, fee_limit) != data {
        return Err(DisclosureError::MalformedPayload);
    }
    Ok((
        DisclosedProgramUpgrade {
            program_id: native.program_id,
            guest_abi: native.guest_abi,
            old_hash: native.old_hash,
            new_hash: native.new_hash,
            migration_hook: migration_hook.to_vec(),
            clear_interface,
            interface: native.interface.map(<[u8]>::to_vec),
            wasm: wasm.to_vec(),
        },
        fee_limit,
    ))
}

/// Encodes one of the five native wind-down operations.
///
/// # Panics
/// Panics if `wind_down` is not a valid native wind-down operation.
#[must_use]
pub fn encode_program_wind_down(wind_down: &NativeProgramWindDown<'_>) -> Vec<u8> {
    assert!(
        wind_down.encode().is_ok(),
        "invalid native program wind-down"
    );
    let (signature, args) = match wind_down.operation {
        ProgramWindDownOperation::Route {
            account,
            asset,
            destination,
            seed,
        } => (
            PROGRAM_ROUTE_SIGNATURE,
            [
                [
                    wind_down.program_id.bytes(),
                    account,
                    asset,
                    destination,
                    abi_word(5 * 32),
                ]
                .concat(),
                abi_tail(seed),
            ]
            .concat(),
        ),
        ProgramWindDownOperation::Deprecate {
            exit_program,
            deadline_batch,
        } => (
            PROGRAM_DEPRECATE_SIGNATURE,
            [
                wind_down.program_id.bytes(),
                exit_program,
                abi_word(deadline_batch.into()),
            ]
            .concat(),
        ),
        ProgramWindDownOperation::Tombstone => (
            PROGRAM_TOMBSTONE_SIGNATURE,
            wind_down.program_id.bytes().to_vec(),
        ),
        ProgramWindDownOperation::Exit { account } => (
            PROGRAM_EXIT_SIGNATURE,
            [wind_down.program_id.bytes(), account].concat(),
        ),
        ProgramWindDownOperation::BoundedExit {
            account,
            maximum_exit_amount,
        } => (
            PROGRAM_BOUNDED_EXIT_SIGNATURE,
            [
                wind_down.program_id.bytes(),
                account,
                abi_word(maximum_exit_amount),
            ]
            .concat(),
        ),
    };
    [ledger_selector(signature).to_vec(), args].concat()
}

/// Decodes only exact wind-down heads and the ordered, zero-padded route seed.
///
/// # Errors
/// Returns `MalformedPayload` for an unknown selector or noncanonical ABI/native terms.
pub fn decode_program_wind_down(data: &[u8]) -> Result<DisclosedProgramWindDown, DisclosureError> {
    let selector = data.get(..4).ok_or(DisclosureError::MalformedPayload)?;
    let args = &data[4..];
    let operation = if selector == ledger_selector(PROGRAM_ROUTE_SIGNATURE) {
        let mut cursor = 5 * 32;
        let seed = tail(args, 4, &mut cursor)?;
        if cursor != args.len() {
            return Err(DisclosureError::MalformedPayload);
        }
        ProgramWindDownOperation::Route {
            account: word(args, 1)?,
            asset: word(args, 2)?,
            destination: word(args, 3)?,
            seed,
        }
    } else if selector == ledger_selector(PROGRAM_DEPRECATE_SIGNATURE) && args.len() == 3 * 32 {
        ProgramWindDownOperation::Deprecate {
            exit_program: word(args, 1)?,
            deadline_batch: u64::from_be_bytes(uint(word(args, 2)?)?),
        }
    } else if selector == ledger_selector(PROGRAM_TOMBSTONE_SIGNATURE) && args.len() == 32 {
        ProgramWindDownOperation::Tombstone
    } else if selector == ledger_selector(PROGRAM_EXIT_SIGNATURE) && args.len() == 2 * 32 {
        ProgramWindDownOperation::Exit {
            account: word(args, 1)?,
        }
    } else if selector == ledger_selector(PROGRAM_BOUNDED_EXIT_SIGNATURE) && args.len() == 3 * 32 {
        ProgramWindDownOperation::BoundedExit {
            account: word(args, 1)?,
            maximum_exit_amount: u128::from_be_bytes(uint(word(args, 2)?)?),
        }
    } else {
        return Err(DisclosureError::MalformedPayload);
    };
    let native = NativeProgramWindDown {
        program_id: ProgramId::new(word(args, 0)?),
        operation,
    };
    native
        .encode()
        .map_err(|_| DisclosureError::MalformedPayload)?;
    if encode_program_wind_down(&native) != data {
        return Err(DisclosureError::MalformedPayload);
    }
    Ok(DisclosedProgramWindDown {
        program_id: native.program_id,
        operation: wind_down_operation(operation),
    })
}

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
