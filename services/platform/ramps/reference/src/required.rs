pub(crate) trait Required<T> {
    fn required(self, context: &str) -> T;
}

impl<T, E: std::fmt::Debug> Required<T> for Result<T, E> {
    fn required(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> Required<T> for Option<T> {
    fn required(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}
