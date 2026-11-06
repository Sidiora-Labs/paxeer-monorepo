//! Kernel status translation for captured native Programs CALL results.

const LXP_ERR_REPLAY_BOUND: i32 = -740;
const LXP_ERR_ENGINE_FAULT: i32 = -741;

/// Preserve retained CALL records, including guest traps, and translate executor errors.
///
/// # Errors
///
/// Returns -740 for the capture producer's Bounds engine fault, -741 for every
/// other executor fault, and -3 for other execution errors.
pub fn from_captured<T>(result: Result<T, crate::ExecutionError>) -> Result<T, i32> {
    result.map_err(|error| match error {
        crate::ExecutionError::Fault(crate::ExecutionFault::EngineFault { reason })
            if reason == "program replay record refused: Bounds" =>
        {
            LXP_ERR_REPLAY_BOUND
        }
        crate::ExecutionError::Fault(_) => LXP_ERR_ENGINE_FAULT,
        _ => -3,
    })
}
