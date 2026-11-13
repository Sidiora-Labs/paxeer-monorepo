//! Matching assertion for present test and fixture values.

/// Requires a present value and returns it.
///
/// # Panics
/// Panics with the caller's context when the option is `None`.
#[track_caller]
pub fn required<T>(option: Option<T>, context: &str) -> T {
    match option {
        Some(value) => value,
        None => panic!("{context}"),
    }
}
