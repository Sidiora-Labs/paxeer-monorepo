//! Fail-closed inspection of untrusted version and kind discriminants.

use layerx_wire::decode::Decoder;
use layerx_wire::WireError;

/// The closed set of executable V1 intent kinds.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u16)]
pub enum IntentKindTag {
    DidRegistration = 1,
    KeyRotation = 2,
    RecoveryRegistration = 3,
    LxpSend = 5,
    LxpReceive = 6,
    PayerGrantRegistration = 7,
    BudgetCreate = 8,
    BudgetFund = 9,
    BudgetDefund = 10,
    SessionGrant = 13,
    SessionRevoke = 14,
    AuthorityGrant = 15,
    NativeReceive = 17,
}

impl IntentKindTag {
    /// Encodes the V1 classification header consumed by `inspect_intent`.
    /// This header carries no payload and does not authorize execution.
    #[must_use]
    pub const fn header_bytes(self) -> [u8; 4] {
        let kind = (self as u16).to_be_bytes();
        [0, 1, kind[0], kind[1]]
    }

    const fn from_u16(value: u16) -> Result<Self, RejectReason> {
        match value {
            1 => Ok(Self::DidRegistration),
            2 => Ok(Self::KeyRotation),
            3 => Ok(Self::RecoveryRegistration),
            5 => Ok(Self::LxpSend),
            6 => Ok(Self::LxpReceive),
            7 => Ok(Self::PayerGrantRegistration),
            8 => Ok(Self::BudgetCreate),
            9 => Ok(Self::BudgetFund),
            10 => Ok(Self::BudgetDefund),
            13 => Ok(Self::SessionGrant),
            14 => Ok(Self::SessionRevoke),
            15 => Ok(Self::AuthorityGrant),
            17 => Ok(Self::NativeReceive),
            4 | 11 | 12 | 16 => Err(RejectReason::RetiredShape),
            _ => Err(RejectReason::UnknownKind(value)),
        }
    }
}

/// A recognised intent header. Recognition alone never executes an intent.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct IntentHeader {
    pub version: u16,
    pub kind: IntentKindTag,
}

/// Typed reason an untrusted intent was preserved instead of executed.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum RejectReason {
    Malformed(WireError),
    UnknownVersion(u16),
    UnknownKind(u16),
    RetiredShape,
}

/// Zero-copy diagnostic record retaining the exact untrusted input.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct RejectedIntent<'a> {
    input: &'a [u8],
    reason: RejectReason,
}

impl<'a> RejectedIntent<'a> {
    #[must_use]
    pub const fn input(&self) -> &'a [u8] {
        self.input
    }

    #[must_use]
    pub const fn reason(&self) -> RejectReason {
        self.reason
    }
}

/// Inspects only the version and kind of an untrusted intent envelope.
///
/// This function cannot execute or compile input. Unknown values retain a
/// zero-copy reference to every original byte for bounded diagnosis.
///
/// # Errors
///
/// Returns a typed rejection for truncated input, every version other than V1,
/// and every unknown or retired kind. V2 retired kinds retain the more specific
/// `RetiredShape` diagnosis; other V2 input is rejected as `UnknownVersion`.
pub fn inspect_intent(input: &[u8]) -> Result<IntentHeader, RejectedIntent<'_>> {
    let mut decoder = Decoder::new(input, 0);
    let version = decoder.u16().map_err(|error| RejectedIntent {
        input,
        reason: RejectReason::Malformed(error),
    })?;
    if version != 1 && version != 2 {
        return Err(RejectedIntent {
            input,
            reason: RejectReason::UnknownVersion(version),
        });
    }
    let raw_kind = decoder.u16().map_err(|error| RejectedIntent {
        input,
        reason: RejectReason::Malformed(error),
    })?;
    let kind =
        IntentKindTag::from_u16(raw_kind).map_err(|reason| RejectedIntent { input, reason })?;
    if version == 2 {
        return Err(RejectedIntent {
            input,
            reason: RejectReason::UnknownVersion(version),
        });
    }
    Ok(IntentHeader { version, kind })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn unknown_version_and_kind_preserve_every_input_byte() {
        let unknown_version = [0, 2, 0, 5, 0xaa, 0xbb];
        let version_rejection = inspect_intent(&unknown_version)
            .err()
            .unwrap_or_else(|| panic!("unknown version accepted"));
        assert_eq!(version_rejection.input(), unknown_version);
        assert_eq!(version_rejection.reason(), RejectReason::UnknownVersion(2));

        let unknown_kind = [0, 1, 0xff, 0xfe, 0xcc];
        let kind_rejection = inspect_intent(&unknown_kind)
            .err()
            .unwrap_or_else(|| panic!("unknown kind accepted"));
        assert_eq!(kind_rejection.input(), unknown_kind);
        assert_eq!(kind_rejection.reason(), RejectReason::UnknownKind(65_534));
    }

    #[test]
    fn retired_shapes_preserve_v1_and_v2_input_without_allocation() {
        for version in [1_u16, 2] {
            for tag in [4_u16, 11, 12, 16] {
                let version = version.to_be_bytes();
                let tag = tag.to_be_bytes();
                let input = [version[0], version[1], tag[0], tag[1], 0xde, 0xad];
                for bytes in [&input[..4], input.as_slice()] {
                    let rejected = inspect_intent(bytes)
                        .err()
                        .unwrap_or_else(|| panic!("retired shape accepted"));
                    assert_eq!(rejected.reason(), RejectReason::RetiredShape);
                    assert_eq!(rejected.input(), bytes);
                    assert_eq!(rejected.input().as_ptr(), bytes.as_ptr());
                }
            }
        }
    }

    #[test]
    fn recognised_header_is_classification_not_execution() {
        assert_eq!(
            inspect_intent(&[0, 1, 0, 17, 0xde, 0xad]),
            Ok(IntentHeader {
                version: 1,
                kind: IntentKindTag::NativeReceive,
            })
        );
    }
}
