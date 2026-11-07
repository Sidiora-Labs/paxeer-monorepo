//! Kernel status translation for captured native Programs CALL results.

const LXP_ERR_REPLAY_BOUND: i32 = -740;
const LXP_ERR_ENGINE_FAULT: i32 = -741;

std::thread_local! {
    static CAPTURED_FAULT: std::cell::RefCell<String> = const {
        std::cell::RefCell::new(String::new())
    };
}

pub(crate) fn clear_captured_fault() {
    CAPTURED_FAULT.with(|fault| fault.borrow_mut().clear());
}

pub(crate) fn copy_captured_fault(buffer: &mut [u8]) -> usize {
    CAPTURED_FAULT.with(|fault| {
        let fault = fault.borrow();
        if !buffer.is_empty() {
            let copied = fault.len().min(buffer.len() - 1);
            buffer[..copied].copy_from_slice(&fault.as_bytes()[..copied]);
            buffer[copied] = 0;
        }
        fault.len()
    })
}

/// Preserve retained CALL records, including guest traps, and translate executor errors.
///
/// # Errors
///
/// Returns -740 for the capture producer's Bounds engine fault, -741 for every
/// other executor fault, and -3 for other execution errors.
pub fn from_captured<T>(result: Result<T, crate::ExecutionError>) -> Result<T, i32> {
    result.map_err(|error| {
        CAPTURED_FAULT.with(|fault| *fault.borrow_mut() = format!("{error:?}"));
        match error {
            crate::ExecutionError::Fault(crate::ExecutionFault::EngineFault { reason })
                if reason == "program replay record refused: Bounds" =>
            {
                LXP_ERR_REPLAY_BOUND
            }
            crate::ExecutionError::Fault(_) => LXP_ERR_ENGINE_FAULT,
            _ => -3,
        }
    })
}
