//! Versioned contracts shared by the native CALL controller and access planner.

use crate::{PrincipalId, ProgramId};

pub const CALL_FIXED_BYTES: usize = 106;
pub const REPLAY_PROFILE_DOMAIN: &[u8] = b"LXP/program-replay-profile/v1\0";

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct CallPayloadLayout {
    pub call_offset: usize,
    pub capabilities_offset: usize,
    pub capabilities_length: usize,
    pub declaration_offset: usize,
    pub declaration_length: usize,
}

/// Decodes offsets into the complete authenticated activity, including its replay wrapper.
pub fn call_payload_layout(payload: &[u8]) -> Option<CallPayloadLayout> {
    let offset = if payload.get(..34)? == [0; 34] {
        let prefix = 34 + REPLAY_PROFILE_DOMAIN.len();
        if payload.get(34..prefix)? != REPLAY_PROFILE_DOMAIN {
            return None;
        }
        let fields = payload.get(prefix..prefix + 14)?;
        if u16::from_be_bytes(fields[..2].try_into().ok()?) != 1 {
            return None;
        }
        let boundaries = u32::from_be_bytes(fields[2..6].try_into().ok()?);
        let bytes = u32::from_be_bytes(fields[6..10].try_into().ok()?);
        let original = u32::from_be_bytes(fields[10..14].try_into().ok()?) as usize;
        if !(2..=crate::replay_record::MAX_PROGRAM_REPLAY_BOUNDARIES).contains(&boundaries)
            || !(512..=crate::replay_record::MAX_PROGRAM_REPLAY_BYTES).contains(&bytes)
            || original != payload.len().checked_sub(prefix + 14)?
        {
            return None;
        }
        prefix + 14
    } else {
        0
    };
    let header = payload.get(offset..offset.checked_add(CALL_FIXED_BYTES)?)?;
    ProgramId::new(header[..32].try_into().ok()?).ok()?;
    crate::abi_policy::abi_revision(u16::from_be_bytes(header[32..34].try_into().ok()?)).ok()?;
    let entrypoint = usize::from(u16::from_be_bytes(header[34..36].try_into().ok()?));
    let calldata = u32::from_be_bytes(header[36..40].try_into().ok()?) as usize;
    let capabilities_length = usize::from(u16::from_be_bytes(header[40..42].try_into().ok()?));
    let declaration_length = u32::from_be_bytes(header[42..46].try_into().ok()?) as usize;
    let capabilities_offset = offset
        .checked_add(CALL_FIXED_BYTES)?
        .checked_add(entrypoint)?
        .checked_add(calldata)?;
    let declaration_offset = capabilities_offset.checked_add(capabilities_length)?;
    if declaration_offset.checked_add(declaration_length)? != payload.len() {
        return None;
    }
    Some(CallPayloadLayout {
        call_offset: offset,
        capabilities_offset,
        capabilities_length,
        declaration_offset,
        declaration_length,
    })
}

/// Canonical CALL producer for clients of the native versioned transport.
pub struct CanonicalCall<'a> {
    pub program: ProgramId,
    pub abi: u16,
    pub entrypoint: &'a str,
    pub calldata: &'a [u8],
    pub capabilities: &'a crate::CapabilitySet,
    pub declaration: &'a crate::AccessDeclaration,
    pub response_capacity: u32,
    pub budget: crate::ResourceBudget,
}

impl CanonicalCall<'_> {
    pub fn canonical_bytes(
        &self,
        profile: Option<crate::ProgramReplayProfile>,
    ) -> Result<Vec<u8>, crate::AbiError> {
        use crate::AbiError::InvalidEncoding;
        crate::abi_policy::abi_revision(self.abi).map_err(|_| InvalidEncoding)?;
        if self.entrypoint.is_empty()
            || self.entrypoint.len() > 64
            || !self
                .entrypoint
                .bytes()
                .all(|b| b.is_ascii_alphanumeric() || b == b'_' || b == b'.')
        {
            return Err(InvalidEncoding);
        }
        let grants = self.capabilities.canonical_encoding();
        let declaration = self
            .declaration
            .canonical_bytes()
            .map_err(|_| InvalidEncoding)?;
        let mut call = self.program.bytes().to_vec();
        call.extend_from_slice(&self.abi.to_be_bytes());
        call.extend_from_slice(
            &u16::try_from(self.entrypoint.len())
                .map_err(|_| InvalidEncoding)?
                .to_be_bytes(),
        );
        call.extend_from_slice(
            &u32::try_from(self.calldata.len())
                .map_err(|_| InvalidEncoding)?
                .to_be_bytes(),
        );
        call.extend_from_slice(
            &u16::try_from(grants.len())
                .map_err(|_| InvalidEncoding)?
                .to_be_bytes(),
        );
        call.extend_from_slice(
            &u32::try_from(declaration.len())
                .map_err(|_| InvalidEncoding)?
                .to_be_bytes(),
        );
        call.extend_from_slice(&self.response_capacity.to_be_bytes());
        for amount in [
            self.budget.cpu_fuel(),
            self.budget.memory_bytes(),
            self.budget.storage_read_bytes(),
            self.budget.storage_write_bytes(),
            u64::from(self.budget.output_values()),
            self.budget.output_bytes(),
            u64::from(self.budget.table_elements()),
        ] {
            call.extend_from_slice(&amount.to_be_bytes());
        }
        call.extend_from_slice(self.entrypoint.as_bytes());
        call.extend_from_slice(self.calldata);
        call.extend_from_slice(&grants);
        call.extend_from_slice(&declaration);
        let encoded = if let Some(profile) = profile {
            let mut payload = vec![0; 34];
            payload.extend_from_slice(REPLAY_PROFILE_DOMAIN);
            payload.extend_from_slice(&1u16.to_be_bytes());
            payload.extend_from_slice(&profile.maximum_boundaries().to_be_bytes());
            payload.extend_from_slice(&profile.maximum_bytes().to_be_bytes());
            payload.extend_from_slice(
                &u32::try_from(call.len())
                    .map_err(|_| InvalidEncoding)?
                    .to_be_bytes(),
            );
            payload.extend(call);
            payload
        } else {
            call
        };
        call_payload_layout(&encoded).ok_or(InvalidEncoding)?;
        Ok(encoded)
    }
}

pub fn sandbox_execution_principal(
    host_program: ProgramId,
    lease: [u8; 32],
) -> Option<PrincipalId> {
    if lease == [0; 32] {
        return None;
    }
    let mut preimage = b"LayerX/programs/sandbox/namespace/v1\0".to_vec();
    preimage.extend_from_slice(&host_program.bytes());
    preimage.extend_from_slice(&lease);
    PrincipalId::new(crate::hash_bytes(crate::HashAlgorithm::Sha256, &preimage).ok()?).ok()
}

/// # Safety
/// `payload` points to `length` readable bytes for the duration of this call.
#[no_mangle]
pub unsafe extern "C" fn layerx_programs_call_payload_offset(
    payload: *const u8,
    length: usize,
) -> i32 {
    if payload.is_null() || length > isize::MAX as usize {
        return -1;
    }
    let payload = unsafe { core::slice::from_raw_parts(payload, length) };
    call_payload_layout(payload)
        .and_then(|layout| i32::try_from(layout.call_offset).ok())
        .unwrap_or(-1)
}

/// # Safety
/// The input buffers contain 32 readable bytes and `out` has 32 writable bytes.
#[no_mangle]
pub unsafe extern "C" fn layerx_programs_sandbox_namespace(
    host: *const u8,
    lease: *const u8,
    out: *mut u8,
) -> i32 {
    if host.is_null() || lease.is_null() || out.is_null() {
        return -1;
    }
    let host = unsafe { core::ptr::read_unaligned(host.cast::<[u8; 32]>()) };
    let lease = unsafe { core::ptr::read_unaligned(lease.cast::<[u8; 32]>()) };
    let Some(principal) = ProgramId::new(host)
        .ok()
        .and_then(|host| sandbox_execution_principal(host, lease))
    else {
        return -1;
    };
    unsafe { core::ptr::copy_nonoverlapping(principal.bytes().as_ptr(), out, 32) };
    0
}

/// # Safety
/// Evidence is readable for `length` bytes; `principal` is readable for 32 bytes.
#[no_mangle]
pub unsafe extern "C" fn layerx_programs_event_envelope_validate(
    evidence: *const u8,
    length: usize,
    principal: *const u8,
    count: u32,
) -> i32 {
    if evidence.is_null() || principal.is_null() || length > isize::MAX as usize {
        return -1;
    }
    let evidence = unsafe { core::slice::from_raw_parts(evidence, length) };
    let principal = unsafe { core::ptr::read_unaligned(principal.cast::<[u8; 32]>()) };
    let Ok(effects) = crate::AbiEffects::decode_program_event_envelope(evidence) else {
        return -1;
    };
    if count as usize > crate::abi::MAX_EVENTS_PER_ACTIVITY
        || effects.events.len() != count as usize
        || effects
            .events
            .iter()
            .any(|event| event.principal.bytes() != principal)
    {
        return -1;
    }
    0
}
