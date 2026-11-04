fn quote_amount(amount: u64) -> u64 {
    amount * 3
}

#[cfg(test)]
mod tests {
    use super::quote_amount;

    #[test]
    fn quotes_triple_amount() {
        assert_eq!(quote_amount(4), 12);
    }
}
