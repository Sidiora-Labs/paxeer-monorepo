use std::fmt::Debug;

pub trait TestResultExt<T> {
    /// Returns the verified value.
    ///
    /// # Panics
    ///
    /// Panics with the supplied context when verification returns an error.
    #[track_caller]
    fn or_panic(self, message: &str) -> T;
}

impl<T, E: Debug> TestResultExt<T> for Result<T, E> {
    #[track_caller]
    fn or_panic(self, message: &str) -> T {
        match self {
            Ok(value) => value,
            Err(error) => panic!("{message}: {error:?}"),
        }
    }
}
