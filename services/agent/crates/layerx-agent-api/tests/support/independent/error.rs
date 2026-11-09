// From ed25519-compact 2.1.1 (MIT); see PROVENANCE.md.
use core::fmt::{self, Display};

#[derive(Debug, Copy, Clone, Eq, PartialEq)]
pub enum Error {
    /// Non-canonical encoding.
    NonCanonical,
}

impl Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::NonCanonical => f.write_str("Non-canonical encoding"),
        }
    }
}
