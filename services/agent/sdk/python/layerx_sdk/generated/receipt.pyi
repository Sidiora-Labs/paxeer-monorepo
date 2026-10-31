# Code generated from sdk/platform/generators/receipt.kvx. DO NOT EDIT.

from enum import Enum

RECEIPT_VERSION_V2: int
RECEIPT_VERSION_V3: int
RECEIPT_DOMAIN: str
SEQUENCE_DOMAIN: str

class ReceiptFailureCode(str, Enum):
    DECODE: str
    CANONICAL_ENCODING: str
    RECEIPT_VERSION: str
    ACTIVITY_ID: str
    KEY_ID: str
    KEY_HEIGHT: str
    RAW_TRANSACTION: str
    FIELDS: str
    RETURN_DATA: str
    LOGS: str
    SEQUENCE_HASH: str
    SIGNATURE: str

PROGRAM_ABI_V1: int
PROGRAM_ABI_V2: int
PROGRAM_ABI_V3: int
PROGRAM_ABI_V4: int
PROGRAM_ABI_V5: int
def supports_program_guest_abi(version: int) -> bool: ...
