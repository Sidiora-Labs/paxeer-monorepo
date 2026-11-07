//! Code generated from sdk/platform/generators/receipt.kvx. DO NOT EDIT.

pub const RECEIPT_VERSION_V2: u16 = 2;
pub const RECEIPT_VERSION_V3: u16 = 3;
pub const RECEIPT_DOMAIN: &str = "LXP/v1/receipt";
pub const SEQUENCE_DOMAIN: &str = "PAXEER/v1/sequence";

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ReceiptFailureCode {
    Decode,
    CanonicalEncoding,
    ReceiptVersion,
    ActivityId,
    KeyId,
    KeyHeight,
    RawTransaction,
    Fields,
    ReturnData,
    Logs,
    SequenceHash,
    Signature,
}

impl ReceiptFailureCode {
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Decode => "decode",
            Self::CanonicalEncoding => "canonical-encoding",
            Self::ReceiptVersion => "receipt-version",
            Self::ActivityId => "activity-id",
            Self::KeyId => "key-id",
            Self::KeyHeight => "key-height",
            Self::RawTransaction => "raw-transaction",
            Self::Fields => "fields",
            Self::ReturnData => "return-data",
            Self::Logs => "logs",
            Self::SequenceHash => "sequence-hash",
            Self::Signature => "signature",
        }
    }
}

pub const PROGRAM_ABI_V1: u16 = 1;
pub const PROGRAM_ABI_V2: u16 = 2;
pub const PROGRAM_ABI_V3: u16 = 3;
pub const PROGRAM_ABI_V4: u16 = 4;
pub const PROGRAM_ABI_V5: u16 = 5;
#[must_use]
pub const fn supports_program_guest_abi(version: u16) -> bool {
    matches!(
        version,
        PROGRAM_ABI_V1 | PROGRAM_ABI_V2 | PROGRAM_ABI_V3 | PROGRAM_ABI_V4 | PROGRAM_ABI_V5
    )
}
