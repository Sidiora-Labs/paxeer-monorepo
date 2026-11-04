pub fn marker_text() -> &'static str {
    let _brace = '}';
    let _byte = b'{';
    let _escaped = "\"#[cfg(test)] mod imaginary { }";
    r###"
#[cfg(test)]
mod imaginary { }
"###
}

/* #[cfg(test)] mod imaginary {
   /* nested comment with } */
} */
// #[cfg(test)] mod imaginary { }
pub fn after_comments() -> u64 {
    7
}

#[allow(dead_code)]
#[cfg(all(test, not(feature = "unused")))]
mod actual_tests {
    #[test]
    fn marker_is_a_literal() {
        assert!(super::marker_text().contains("#[cfg(test)]"));
        assert_eq!(super::after_comments(), 7);
    }
}

pub fn after_tests() -> u64 {
    8
}
