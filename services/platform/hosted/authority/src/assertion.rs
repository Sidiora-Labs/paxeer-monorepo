pub trait Must<T> {
    fn must(self) -> T;
    fn must_with(self, message: &str) -> T;
}

impl<T> Must<T> for Option<T> {
    fn must(self) -> T {
        self.unwrap_or_else(|| panic!("required fixture value is absent"))
    }

    fn must_with(self, message: &str) -> T {
        self.unwrap_or_else(|| panic!("{message}"))
    }
}

impl<T, E: std::fmt::Debug> Must<T> for Result<T, E> {
    fn must(self) -> T {
        self.unwrap_or_else(|error| panic!("required fixture result failed: {error:?}"))
    }

    fn must_with(self, message: &str) -> T {
        self.unwrap_or_else(|error| panic!("{message}: {error:?}"))
    }
}
