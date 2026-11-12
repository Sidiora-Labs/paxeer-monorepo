/// Extracts a required value in an assertion or test producer.
pub trait RequiredValue {
    type Value;

    /// Returns the value.
    ///
    /// # Panics
    /// Panics when the value is absent or the operation returned an error.
    #[track_caller]
    fn required(self) -> Self::Value;

    /// Returns the value with a caller-supplied assertion message.
    ///
    /// # Panics
    /// Panics with `message` when the value is absent or the operation returned an error.
    #[track_caller]
    fn required_with(self, message: &str) -> Self::Value;
}

impl<T, E: core::fmt::Debug> RequiredValue for Result<T, E> {
    type Value = T;

    #[track_caller]
    fn required(self) -> T {
        match self {
            Ok(value) => value,
            Err(error) => panic!("called `Result::unwrap()` on an `Err` value: {error:?}"),
        }
    }

    #[track_caller]
    fn required_with(self, message: &str) -> T {
        match self {
            Ok(value) => value,
            Err(error) => panic!("{message}: {error:?}"),
        }
    }
}

impl<T> RequiredValue for Option<T> {
    type Value = T;

    #[track_caller]
    fn required(self) -> T {
        match self {
            Some(value) => value,
            None => panic!("called `Option::unwrap()` on a `None` value"),
        }
    }

    #[track_caller]
    fn required_with(self, message: &str) -> T {
        match self {
            Some(value) => value,
            None => panic!("{message}"),
        }
    }
}

/// Extracts an error that an assertion requires an operation to return.
pub trait RequiredError {
    type Error;

    /// Returns the error.
    ///
    /// # Panics
    /// Panics when the operation succeeds.
    #[track_caller]
    fn required_error(self) -> Self::Error;
}

impl<T: core::fmt::Debug, E> RequiredError for Result<T, E> {
    type Error = E;

    #[track_caller]
    fn required_error(self) -> E {
        match self {
            Err(error) => error,
            Ok(value) => panic!("called `Result::unwrap_err()` on an `Ok` value: {value:?}"),
        }
    }
}
