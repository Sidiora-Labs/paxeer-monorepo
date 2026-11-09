//! Tenant- and scope-bound model context protocol tools.

pub mod approval;
pub mod binding;
pub mod boundary;
pub mod catalogue;
pub mod listener;
mod readonly;
pub mod server;
pub mod stdio;
pub mod tools;
pub mod untrusted;
pub mod validate;

fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    bytes
        .iter()
        .flat_map(|byte| {
            [
                char::from(DIGITS[usize::from(byte >> 4)]),
                char::from(DIGITS[usize::from(byte & 15)]),
            ]
        })
        .collect()
}

#[cfg(test)]
#[path = "../tests/support/assert_result.rs"]
mod result_assertion;

#[cfg(test)]
#[path = "../tests/support/assert_option.rs"]
mod option_assertion;
