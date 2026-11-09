//! Matching assertion for successful test and fixture results.

use std::fmt::Debug;

/// Requires a successful result and returns its value.
///
/// # Panics
/// Panics with the caller's context and error when the result is `Err`.
#[track_caller]
pub fn required<T, E: Debug>(result: Result<T, E>, context: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{context}: {error:?}"),
    }
}
