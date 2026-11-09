use std::fmt::Debug;

pub(crate) trait TestOutcome {
    type Value;
    fn require(self, context: &str) -> Self::Value;
}

impl<T, E: Debug> TestOutcome for Result<T, E> {
    type Value = T;
    fn require(self, context: &str) -> T {
        match self {
            Ok(value) => value,
            Err(error) => panic!("{context}: {error:?}"),
        }
    }
}

impl<T> TestOutcome for Option<T> {
    type Value = T;
    fn require(self, context: &str) -> T {
        match self {
            Some(value) => value,
            None => panic!("{context}"),
        }
    }
}

pub(crate) fn require<T: TestOutcome>(outcome: T, context: &str) -> T::Value {
    outcome.require(context)
}
