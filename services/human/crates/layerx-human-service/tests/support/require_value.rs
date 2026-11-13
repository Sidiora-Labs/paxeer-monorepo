// Required values in integration assertions.

/// Requires a value and reports its context when absent or invalid.
pub trait RequireValue<T> {
    /// Returns the value.
    ///
    /// # Panics
    /// Panics with the supplied context when the value is absent or invalid.
    fn or_fail(self, context: &str) -> T;
}

impl<T, E: std::fmt::Debug> RequireValue<T> for Result<T, E> {
    fn or_fail(self, context: &str) -> T {
        match self {
            Ok(value) => value,
            Err(error) => panic!("{context}: {error:?}"),
        }
    }
}

impl<T> RequireValue<T> for Option<T> {
    fn or_fail(self, context: &str) -> T {
        match self {
            Some(value) => value,
            None => panic!("{context}"),
        }
    }
}
