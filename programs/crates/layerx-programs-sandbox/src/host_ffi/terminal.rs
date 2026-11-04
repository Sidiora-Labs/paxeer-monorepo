use crate::DurableUsageState;
use layerx_programs_runtime::{hash_bytes, HashAlgorithm};
use std::cell::RefCell;

const OK: i32 = 0;
const NON_CANONICAL: i32 = -3;
const CONTEXT_MISMATCH: i32 = -213;

struct DestroyTerminal {
    token: u64,
    bytes: Vec<u8>,
}
thread_local! {
    static DESTROY_TERMINAL: RefCell<Option<DestroyTerminal>> = const { RefCell::new(None) };
}

unsafe extern "C" {
    fn layerx_programs_sandbox_destroy_archive(
        token: u64,
        kind: u16,
        bytes: *const u8,
        length: u32,
    ) -> i32;
}

pub(super) fn publish_destroy(
    token: u64,
    state: &DurableUsageState,
    ledger_bytes: &[u8],
    usage_lease_bytes: &[u8],
    receipt_digest: [u8; 32],
    transfer_root: [u8; 32],
    amount: u128,
) -> Result<(), i32> {
    let lease_bytes = state
        .lease
        .canonical_state_bytes()
        .map_err(|_| NON_CANONICAL)?;
    let escrow_bytes = state.escrow.canonical_state();
    for (kind, bytes) in [
        (0u16, lease_bytes.as_slice()),
        (1, escrow_bytes.as_slice()),
        (2, ledger_bytes),
        (3, usage_lease_bytes),
    ] {
        if unsafe {
            layerx_programs_sandbox_destroy_archive(
                token,
                kind,
                bytes.as_ptr(),
                u32::try_from(bytes.len()).map_err(|_| NON_CANONICAL)?,
            )
        } != OK
        {
            return Err(NON_CANONICAL);
        }
    }
    let mut terminal = Vec::new();
    terminal.extend_from_slice(&receipt_digest);
    terminal.extend_from_slice(&transfer_root);
    terminal.extend_from_slice(&amount.to_be_bytes());
    terminal.extend_from_slice(&state.lease.state_digest().map_err(|_| NON_CANONICAL)?);
    terminal.extend_from_slice(
        &hash_bytes(HashAlgorithm::Sha256, &escrow_bytes).map_err(|_| NON_CANONICAL)?,
    );
    DESTROY_TERMINAL.with(|slot| {
        *slot.borrow_mut() = Some(DestroyTerminal {
            token,
            bytes: terminal,
        });
    });
    Ok(())
}

#[no_mangle]
pub extern "C" fn layerx_programs_sandbox_destroy_terminal(
    token: u64,
    output: *mut u8,
    capacity: u32,
) -> i32 {
    std::panic::catch_unwind(|| {
        if output.is_null() {
            return Err(NON_CANONICAL);
        }
        DESTROY_TERMINAL.with(|slot| {
            let mut slot = slot.borrow_mut();
            let value = slot.as_ref().ok_or(NON_CANONICAL)?;
            if value.token != token {
                return Err(CONTEXT_MISMATCH);
            }
            if value.bytes.len() > capacity as usize {
                return Err(NON_CANONICAL);
            }
            unsafe {
                core::ptr::copy_nonoverlapping(value.bytes.as_ptr(), output, value.bytes.len());
            }
            *slot = None;
            Ok(())
        })
    })
    .unwrap_or(Err(NON_CANONICAL))
    .map_or_else(|error| error, |()| OK)
}

#[cfg(test)]
mod abi_guard {
    use super::*;

    #[test]
    fn native_terminal_reentrant_read_returns_refusal_without_unwinding() {
        let mut output = [0xa5; 144];
        DESTROY_TERMINAL.with(|slot| {
            let _held = slot.borrow();
            assert_eq!(
                layerx_programs_sandbox_destroy_terminal(1, output.as_mut_ptr(), 144),
                NON_CANONICAL
            );
        });
        assert_eq!(output, [0xa5; 144]);
    }
}
