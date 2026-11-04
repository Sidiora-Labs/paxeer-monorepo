fn quote_amount(amount: u64) -> u64 {
    amount * 3
}

#[cfg(test)]
mod tests {
    fn helper() -> u64 {
        1
    }

    #[test]
    fn checks_helper() {
        assert_eq!(helper(), 1);
    }
}
