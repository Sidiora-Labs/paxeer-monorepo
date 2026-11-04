#[cfg(any(test, feature = "runtime"))]
pub fn available_in_production() -> u64 {
    1
}

#[cfg(not(test))]
pub fn production_only() -> u64 {
    2
}

pub mod nested {
    pub fn nested_production() -> u64 {
        3
    }

    #[cfg(test)]
    mod nested_tests {
        #[test]
        fn nested_check() {
            assert_eq!(super::nested_production(), 3);
        }
    }
}

#[test]
fn standalone_test() {
    assert_eq!(1 + 1, 2);
}

pub mod only_tests {
    #![cfg(test)]

    #[test]
    fn inner_attribute_test() {
        assert_eq!(3 + 3, 6);
    }
}
