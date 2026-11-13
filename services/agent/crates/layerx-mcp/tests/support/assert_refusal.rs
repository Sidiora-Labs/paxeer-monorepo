//! Matching assertion for refused test and fixture results.

/// Requires a refused result and returns its error.
///
/// # Panics
/// Panics with the caller's context when the result succeeds.
#[track_caller]
pub fn refused<T, E>(result: Result<T, E>, context: &str) -> E {
    match result {
        Err(error) => error,
        Ok(_) => panic!("{context}: expected a refusal"),
    }
}
