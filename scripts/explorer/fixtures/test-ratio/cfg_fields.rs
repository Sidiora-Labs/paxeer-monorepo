pub struct Counter {
    #[cfg(test)]
    test_seed: u64,
    pub value: u64,
}

pub fn read_counter(counter: &Counter) -> u64 {
    counter.value
}

#[cfg(test)]
mod tests {
    #[test]
    fn reads_counter() {
        let counter = super::Counter {
            test_seed: 9,
            value: 5,
        };
        assert_eq!(super::read_counter(&counter), 5);
        assert_eq!(counter.test_seed, 9);
    }
}
