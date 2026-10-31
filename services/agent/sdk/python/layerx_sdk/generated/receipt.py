# Code generated from sdk/platform/generators/receipt.kvx. DO NOT EDIT.

from enum import Enum

RECEIPT_VERSION_V2 = 2
RECEIPT_VERSION_V3 = 3
RECEIPT_DOMAIN = "LXP/v1/receipt"
SEQUENCE_DOMAIN = "PAXEER/v1/sequence"

class ReceiptFailureCode(str, Enum):
    DECODE = "decode"
    CANONICAL_ENCODING = "canonical-encoding"
    RECEIPT_VERSION = "receipt-version"
    ACTIVITY_ID = "activity-id"
    KEY_ID = "key-id"
    KEY_HEIGHT = "key-height"
    RAW_TRANSACTION = "raw-transaction"
    FIELDS = "fields"
    RETURN_DATA = "return-data"
    LOGS = "logs"
    SEQUENCE_HASH = "sequence-hash"
    SIGNATURE = "signature"

PROGRAM_ABI_V1 = 1
PROGRAM_ABI_V2 = 2
PROGRAM_ABI_V3 = 3
PROGRAM_ABI_V4 = 4
PROGRAM_ABI_V5 = 5
def supports_program_guest_abi(version: int) -> bool:
    return type(version) is int and version in (PROGRAM_ABI_V1, PROGRAM_ABI_V2, PROGRAM_ABI_V3, PROGRAM_ABI_V4, PROGRAM_ABI_V5)
