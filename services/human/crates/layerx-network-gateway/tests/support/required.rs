trait Required<T> {
    fn required(self) -> T;
    fn required_with(self, message: &str) -> T;
}

impl<T, E: std::fmt::Debug> Required<T> for Result<T, E> {
    fn required(self) -> T {
        match self {
            Ok(value) => value,
            Err(error) => panic!("required result: {error:?}"),
        }
    }

    fn required_with(self, message: &str) -> T {
        match self {
            Ok(value) => value,
            Err(error) => panic!("{message}: {error:?}"),
        }
    }
}

impl<T> Required<T> for Option<T> {
    fn required(self) -> T {
        match self {
            Some(value) => value,
            None => panic!("required value is absent"),
        }
    }

    fn required_with(self, message: &str) -> T {
        match self {
            Some(value) => value,
            None => panic!("{message}"),
        }
    }
}
