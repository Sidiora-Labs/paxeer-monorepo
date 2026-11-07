fn quote_amount(amount: u64) -> u64 {
    amount * 2
}

#[cfg(test)]
mod tests {
    use super::quote_amount;

    #[test]
    fn quotes_double_amount() {
        assert_eq!(quote_amount(4), 8);
    }
}
