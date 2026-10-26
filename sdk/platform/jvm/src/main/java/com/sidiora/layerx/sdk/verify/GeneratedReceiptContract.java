// Code generated from sdk/platform/generators/receipt.kvx. DO NOT EDIT.

package com.sidiora.layerx.sdk.verify;

public final class GeneratedReceiptContract {
    private GeneratedReceiptContract() {}
    public static final int RECEIPT_VERSION_V2 = 2;
    public static final int RECEIPT_VERSION_V3 = 3;
    public static final String RECEIPT_DOMAIN = "LXP/v1/receipt";
    public static final String SEQUENCE_DOMAIN = "PAXEER/v1/sequence";

    public enum ReceiptCheck {
        DECODE("decode"),
        CANONICAL_ENCODING("canonical-encoding"),
        RECEIPT_VERSION("receipt-version"),
        ACTIVITY_ID("activity-id"),
        KEY_ID("key-id"),
        KEY_HEIGHT("key-height"),
        RAW_TRANSACTION("raw-transaction"),
        FIELDS("fields"),
        RETURN_DATA("return-data"),
        LOGS("logs"),
        SEQUENCE_HASH("sequence-hash"),
        SIGNATURE("signature");
        private final String wire;
        ReceiptCheck(String wire) { this.wire = wire; }
        public String wire() { return wire; }
    }

    public static final int PROGRAM_ABI_V1 = 1;
    public static final int PROGRAM_ABI_V2 = 2;
    public static final int PROGRAM_ABI_V3 = 3;
    public static final int PROGRAM_ABI_V4 = 4;
    public static final int PROGRAM_ABI_V5 = 5;
    public static boolean supportsProgramGuestAbi(int version) {
        return switch (version) {
            case PROGRAM_ABI_V1, PROGRAM_ABI_V2, PROGRAM_ABI_V3, PROGRAM_ABI_V4, PROGRAM_ABI_V5 -> true;
            default -> false;
        };
    }
}
