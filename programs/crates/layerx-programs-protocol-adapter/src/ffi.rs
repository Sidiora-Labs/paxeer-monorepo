use core::ffi::c_void;

use layerx_programs::{
    AccountStateHead, CanonicalAccountLeaf, ExitRoute, LifecycleReceipt, ProgramId,
    ProgramLifecycle, ProgramValueAccountBinding, ProvenAccountLeaf, ProvenProgramBinding,
    ReadFreshness, Registry, StateProof, VerifiedAccountSnapshot, WindDownPolicy,
    WindDownStateAccess,
};

use layerx_programs::state_record::lifecycle;

pub use layerx_programs::state_record::{
    restore_verified, restore_verified_profile2, ProtocolAdapterError, ProtocolProgramStateRead,
};

const RESULT_OK: i32 = 0;
const RESULT_UNKNOWN_FIELD: i32 = -7;
const MAX_SEED_BYTES: usize = 128;
const MAX_ACCOUNT_NAME_BYTES: usize = 512;
const MAX_PROOF_DEPTH: usize = 32;
const PROTOCOL_VERSION_ACCOUNT_TREE: u16 = 2;

#[repr(C)]
#[derive(Clone, Copy, Eq, PartialEq)]
struct CAmount {
    hi: u64,
    lo: u64,
}

#[repr(C)]
#[derive(Clone, Copy, Eq, PartialEq)]
struct CProof {
    leaf_index: u32,
    leaf_count: u32,
    depth: u8,
    siblings: [[u8; 32]; MAX_PROOF_DEPTH],
}

#[repr(C)]
#[derive(Clone, Copy, Eq, PartialEq)]
struct CBinding {
    record_version: u8,
    program_id: [u8; 32],
    account_id: [u8; 32],
    asset_id: [u8; 32],
    seed_length: u16,
    seed: [u8; MAX_SEED_BYTES],
    registered_sequence: u64,
    registration_event_digest: [u8; 32],
}

#[repr(C)]
#[derive(Clone, Copy, Eq, PartialEq)]
struct CAccount {
    id: [u8; 32],
    name: [u8; MAX_ACCOUNT_NAME_BYTES],
    name_length: u16,
    kind: i32,
    balance: CAmount,
    asset_id: [u8; 32],
    has_asset: bool,
    next_sequence: u64,
    created_at_sequence: u64,
    frozen: bool,
    has_open_reference: bool,
    authority_key: [u8; 32],
    has_authority_key: bool,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct CValueAccountView {
    binding: CBinding,
    account: CAccount,
    balance: CAmount,
    frozen: bool,
    observed_sequence: u64,
    observed_at: u64,
    receipt_digest: [u8; 32],
    account_root: [u8; 32],
    universal_root: [u8; 32],
    programs_root: [u8; 32],
    state_root: [u8; 32],
    account_proof: CProof,
    account_tree_proof: CProof,
    universal_root_proof: CProof,
    binding_proof: CProof,
    programs_root_proof: CProof,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct CAccountStateHead {
    observed_sequence: u64,
    observed_at: u64,
    receipt_digest: [u8; 32],
    account_root: [u8; 32],
    universal_root: [u8; 32],
    programs_root: [u8; 32],
    state_root: [u8; 32],
    account_tree_proof: CProof,
    universal_root_proof: CProof,
    programs_root_proof: CProof,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct CWindDownView {
    program_id: [u8; 32],
    status: i32,
    exit_program: [u8; 32],
    deadline: u64,
    effective_sequence: u64,
    value_account_count: u16,
    live_value_account_count: u16,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct CExitRoute {
    program_id: [u8; 32],
    account_id: [u8; 32],
    asset_id: [u8; 32],
    destination: [u8; 32],
    seed_length: u16,
    seed: [u8; MAX_SEED_BYTES],
}

#[repr(C)]
#[derive(Clone, Copy)]
struct CHistory {
    program_id: [u8; 32],
    prior: i32,
    current: i32,
    authority: [u8; 32],
    exit_program: [u8; 32],
    deadline: u64,
    effective_sequence: u64,
    value_account_count: u16,
    live_value_account_count: u16,
    account_root: [u8; 32],
}

type ValueVisitor = unsafe extern "C" fn(*const CValueAccountView, *mut c_void) -> i32;
type RouteVisitor = unsafe extern "C" fn(*const CExitRoute, *mut c_void) -> i32;
type HistoryVisitor = unsafe extern "C" fn(*const CHistory, *mut c_void) -> i32;

unsafe extern "C" {
    fn lxp_programs_value_account_iter(
        context: *mut c_void,
        program_id: *const u8,
        receipt_digest: *const u8,
        visitor: ValueVisitor,
        user: *mut c_void,
    ) -> i32;
    fn lxp_programs_account_state_head_read(
        context: *mut c_void,
        program_id: *const u8,
        receipt_digest: *const u8,
        head: *mut CAccountStateHead,
    ) -> i32;
    fn lxp_programs_wind_down_read(
        context: *mut c_void,
        program_id: *const u8,
        view: *mut CWindDownView,
    ) -> i32;
    fn lxp_programs_exit_route_iter(
        context: *mut c_void,
        program_id: *const u8,
        visitor: RouteVisitor,
        user: *mut c_void,
    ) -> i32;
    fn lxp_programs_wind_down_history_iter(
        context: *mut c_void,
        program_id: *const u8,
        visitor: HistoryVisitor,
        user: *mut c_void,
    ) -> i32;
}

unsafe extern "C" fn collect_value(view: *const CValueAccountView, user: *mut c_void) -> i32 {
    if view.is_null() || user.is_null() {
        return -3;
    }
    let values = unsafe { &mut *user.cast::<Vec<CValueAccountView>>() };
    values.push(unsafe { *view });
    RESULT_OK
}

unsafe extern "C" fn collect_route(route: *const CExitRoute, user: *mut c_void) -> i32 {
    if route.is_null() || user.is_null() {
        return -3;
    }
    let routes = unsafe { &mut *user.cast::<Vec<CExitRoute>>() };
    routes.push(unsafe { *route });
    RESULT_OK
}

unsafe extern "C" fn collect_history(history: *const CHistory, user: *mut c_void) -> i32 {
    if history.is_null() || user.is_null() {
        return -3;
    }
    let records = unsafe { &mut *user.cast::<Vec<CHistory>>() };
    records.push(unsafe { *history });
    RESULT_OK
}

fn proof(value: &CProof) -> Result<StateProof, ProtocolAdapterError> {
    let depth = usize::from(value.depth);
    if depth > MAX_PROOF_DEPTH {
        return Err(ProtocolAdapterError::NonCanonicalView);
    }
    Ok(StateProof {
        leaf_index: value.leaf_index,
        leaf_count: value.leaf_count,
        siblings: value.siblings[..depth].to_vec(),
    })
}

fn binding(value: &CBinding) -> Result<ProgramValueAccountBinding, ProtocolAdapterError> {
    let length = usize::from(value.seed_length);
    if length > MAX_SEED_BYTES {
        return Err(ProtocolAdapterError::NonCanonicalView);
    }
    let program =
        ProgramId::new(value.program_id).map_err(|_| ProtocolAdapterError::NonCanonicalView)?;
    Ok(ProgramValueAccountBinding {
        record_version: value.record_version,
        program,
        account_id: value.account_id,
        seed: value.seed[..length].to_vec(),
        asset_id: value.asset_id,
        registered_sequence: value.registered_sequence,
        registration_event_digest: value.registration_event_digest,
    })
}

fn account(value: &CAccount) -> Result<CanonicalAccountLeaf, ProtocolAdapterError> {
    let name_length = usize::from(value.name_length);
    if name_length > MAX_ACCOUNT_NAME_BYTES || value.kind < 0 || value.kind > i32::from(u8::MAX) {
        return Err(ProtocolAdapterError::NonCanonicalView);
    }
    Ok(CanonicalAccountLeaf {
        account_id: value.id,
        name: value.name[..name_length].to_vec(),
        kind: u8::try_from(value.kind).map_err(|_| ProtocolAdapterError::NonCanonicalView)?,
        balance: (u128::from(value.balance.hi) << 64) | u128::from(value.balance.lo),
        asset_id: value.asset_id,
        has_asset: (value.has_asset).into(),
        next_sequence: value.next_sequence,
        created_at_sequence: value.created_at_sequence,
        frozen: (value.frozen).into(),
        has_open_reference: (value.has_open_reference).into(),
        authority_key: value.authority_key,
        has_authority_key: (value.has_authority_key).into(),
    })
}

unsafe fn read_account_head(
    context: *mut c_void,
    program: ProgramId,
    receipt_digest: [u8; 32],
) -> Result<CAccountStateHead, ProtocolAdapterError> {
    let mut head = CAccountStateHead {
        observed_sequence: 0,
        observed_at: 0,
        receipt_digest: [0; 32],
        account_root: [0; 32],
        universal_root: [0; 32],
        programs_root: [0; 32],
        state_root: [0; 32],
        account_tree_proof: CProof {
            leaf_index: 0,
            leaf_count: 0,
            depth: 0,
            siblings: [[0; 32]; MAX_PROOF_DEPTH],
        },
        universal_root_proof: CProof {
            leaf_index: 0,
            leaf_count: 0,
            depth: 0,
            siblings: [[0; 32]; MAX_PROOF_DEPTH],
        },
        programs_root_proof: CProof {
            leaf_index: 0,
            leaf_count: 0,
            depth: 0,
            siblings: [[0; 32]; MAX_PROOF_DEPTH],
        },
    };
    let status = unsafe {
        lxp_programs_account_state_head_read(
            context,
            program.bytes().as_ptr(),
            receipt_digest.as_ptr(),
            &raw mut head,
        )
    };
    if status != RESULT_OK {
        return Err(ProtocolAdapterError::CoreRefused(status));
    }
    if head.receipt_digest != receipt_digest
        || head.observed_sequence == 0
        || head.observed_at == 0
        || head.state_root == [0; 32]
        || head.account_root == [0; 32]
        || head.universal_root == [0; 32]
        || head.programs_root == [0; 32]
    {
        return Err(ProtocolAdapterError::NonCanonicalView);
    }
    Ok(head)
}

unsafe fn read_account_snapshot(
    context: *mut c_void,
    program: ProgramId,
    receipt_digest: [u8; 32],
    head: &CAccountStateHead,
) -> Result<
    (
        Vec<ProgramValueAccountBinding>,
        VerifiedAccountSnapshot,
        usize,
    ),
    ProtocolAdapterError,
> {
    let mut values = Vec::<CValueAccountView>::new();
    let status = unsafe {
        lxp_programs_value_account_iter(
            context,
            program.bytes().as_ptr(),
            receipt_digest.as_ptr(),
            collect_value,
            (&raw mut values).cast(),
        )
    };
    if status != RESULT_OK {
        return Err(ProtocolAdapterError::CoreRefused(status));
    }
    let mut bindings = Vec::with_capacity(values.len());
    let mut proven_bindings = Vec::with_capacity(values.len());
    let mut proven_accounts = Vec::with_capacity(values.len());
    for value in values {
        if value.receipt_digest != head.receipt_digest
            || value.observed_sequence != head.observed_sequence
            || value.observed_at != head.observed_at
            || value.state_root != head.state_root
            || value.account_root != head.account_root
            || value.universal_root != head.universal_root
            || value.programs_root != head.programs_root
            || value.balance.hi != value.account.balance.hi
            || value.balance.lo != value.account.balance.lo
            || value.frozen != value.account.frozen
            || value.account_tree_proof != head.account_tree_proof
            || value.universal_root_proof != head.universal_root_proof
            || value.programs_root_proof != head.programs_root_proof
        {
            return Err(ProtocolAdapterError::NonCanonicalView);
        }
        let binding = binding(&value.binding)?;
        let account = account(&value.account)?;
        if binding.program != program
            || account.account_id != binding.account_id
            || account.asset_id != binding.asset_id
            || account.created_at_sequence != binding.registered_sequence
        {
            return Err(ProtocolAdapterError::NonCanonicalView);
        }
        bindings.push(binding.clone());
        proven_bindings.push(ProvenProgramBinding {
            binding,
            proof: proof(&value.binding_proof)?,
        });
        proven_accounts.push(ProvenAccountLeaf {
            leaf: account,
            proof: proof(&value.account_proof)?,
        });
    }
    proven_bindings.sort_by_key(|value| value.binding.primary_key());
    proven_accounts.sort_by_key(|value| value.leaf.account_id);
    let live_count = proven_accounts
        .iter()
        .filter(|account| account.leaf.balance != 0)
        .count();
    let snapshot = VerifiedAccountSnapshot {
        protocol_version: PROTOCOL_VERSION_ACCOUNT_TREE,
        receipt_digest,
        state_root: head.state_root,
        universal_root: head.universal_root,
        programs_root: head.programs_root,
        account_root: head.account_root,
        account_tree_proof: proof(&head.account_tree_proof)?,
        universal_root_proof: proof(&head.universal_root_proof)?,
        programs_root_proof: proof(&head.programs_root_proof)?,
        freshness: ReadFreshness {
            observed_sequence: head.observed_sequence,
            observed_at: head.observed_at,
        },
        bindings: proven_bindings,
        accounts: proven_accounts,
    };
    Ok((bindings, snapshot, live_count))
}

unsafe fn read_program_lifecycle(
    context: *mut c_void,
    program: ProgramId,
    binding_count: usize,
    live_count: usize,
) -> Result<ProgramLifecycle, ProtocolAdapterError> {
    let mut status_view = CWindDownView {
        program_id: [0; 32],
        status: 0,
        exit_program: [0; 32],
        deadline: 0,
        effective_sequence: 0,
        value_account_count: 0,
        live_value_account_count: 0,
    };
    let status = unsafe {
        lxp_programs_wind_down_read(context, program.bytes().as_ptr(), &raw mut status_view)
    };
    let program_lifecycle = if status == RESULT_UNKNOWN_FIELD {
        ProgramLifecycle::Active
    } else if status == RESULT_OK
        && status_view.program_id == program.bytes()
        && status_view.exit_program == program.bytes()
        && status_view.deadline != 0
        && status_view.effective_sequence != 0
        && usize::from(status_view.value_account_count) == binding_count
        && usize::from(status_view.live_value_account_count) == live_count
    {
        lifecycle(status_view.status)?
    } else if status == RESULT_OK {
        return Err(ProtocolAdapterError::NonCanonicalView);
    } else {
        return Err(ProtocolAdapterError::CoreRefused(status));
    };
    Ok(program_lifecycle)
}

unsafe fn read_exit_routes(
    context: *mut c_void,
    program: ProgramId,
) -> Result<Vec<ExitRoute>, ProtocolAdapterError> {
    let mut c_routes = Vec::<CExitRoute>::new();
    let route_status = unsafe {
        lxp_programs_exit_route_iter(
            context,
            program.bytes().as_ptr(),
            collect_route,
            (&raw mut c_routes).cast(),
        )
    };
    if route_status != RESULT_OK {
        return Err(ProtocolAdapterError::CoreRefused(route_status));
    }
    let mut routes = Vec::with_capacity(c_routes.len());
    for route in c_routes {
        let length = usize::from(route.seed_length);
        if length > MAX_SEED_BYTES || route.program_id != program.bytes() {
            return Err(ProtocolAdapterError::NonCanonicalView);
        }
        routes.push(ExitRoute {
            seed: route.seed[..length].to_vec(),
            account_id: route.account_id,
            asset_id: route.asset_id,
            destination: route.destination,
        });
    }
    Ok(routes)
}

unsafe fn read_lifecycle_history(
    context: *mut c_void,
    program: ProgramId,
    binding_count: usize,
) -> Result<Vec<LifecycleReceipt>, ProtocolAdapterError> {
    let mut c_history = Vec::<CHistory>::new();
    let history_status = unsafe {
        lxp_programs_wind_down_history_iter(
            context,
            program.bytes().as_ptr(),
            collect_history,
            (&raw mut c_history).cast(),
        )
    };
    if history_status != RESULT_OK {
        return Err(ProtocolAdapterError::CoreRefused(history_status));
    }
    let mut history = Vec::with_capacity(c_history.len());
    for record in c_history {
        if record.program_id != program.bytes()
            || record.exit_program != program.bytes()
            || usize::from(record.value_account_count) != binding_count
            || record.live_value_account_count > record.value_account_count
            || record.account_root == [0; 32]
        {
            return Err(ProtocolAdapterError::NonCanonicalView);
        }
        history.push(LifecycleReceipt {
            program,
            prior: lifecycle(record.prior)?,
            current: lifecycle(record.current)?,
            authority: record.authority,
            effective_sequence: record.effective_sequence,
            wind_down: WindDownPolicy {
                exit_program: record.exit_program,
                deadline: record.deadline,
                state_access: WindDownStateAccess::ReadOnly,
            },
            live_value_accounts: u32::from(record.live_value_account_count),
        });
    }
    Ok(history)
}

/// Reads one complete program directly from the committed Programs module and
/// account tree. The C iterator is the sole producer: it verifies the named
/// receipt at the current state head and returns exact primary/account/outer
/// proofs before Rust constructs any public balance value.
///
/// # Errors
///
/// Returns an error when the core refuses the read, the returned view is
/// noncanonical, or account-state, lifecycle, or registry verification fails.
///
/// # Safety
///
/// `context` must be a live read-only `lxp_module_ctx` whose lifetime covers
/// this synchronous call. It must be bound to the canonical kernel account
/// registry and verified-receipt index as required by the C iterator.
pub unsafe fn read_program_state(
    context: *mut c_void,
    registry: &mut Registry,
    program: ProgramId,
    receipt_digest: [u8; 32],
    now: u64,
    staleness_limit: u64,
) -> Result<ProtocolProgramStateRead, ProtocolAdapterError> {
    if context.is_null() || receipt_digest == [0; 32] || now == 0 || staleness_limit == 0 {
        return Err(ProtocolAdapterError::NonCanonicalView);
    }
    let head = unsafe { read_account_head(context, program, receipt_digest) }?;
    let (bindings, snapshot, live_count) =
        unsafe { read_account_snapshot(context, program, receipt_digest, &head) }?;
    let program_lifecycle =
        unsafe { read_program_lifecycle(context, program, bindings.len(), live_count) }?;
    let routes = unsafe { read_exit_routes(context, program) }?;
    let history = unsafe { read_lifecycle_history(context, program, bindings.len()) }?;
    ProtocolProgramStateRead::verify_snapshot(
        registry,
        program,
        snapshot,
        bindings,
        program_lifecycle,
        routes,
        history,
        AccountStateHead {
            receipt_digest,
            state_root: head.state_root,
            freshness: ReadFreshness {
                observed_sequence: head.observed_sequence,
                observed_at: head.observed_at,
            },
        },
        now,
        staleness_limit,
    )
}
