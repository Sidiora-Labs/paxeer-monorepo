package com.sidiora.layerx.sdk;

final class NativeProgramPayload {
    static final int MAX_BYTES = 524288;

    private NativeProgramPayload() {}

    static int size(long encodedBytes) {
        if (encodedBytes < 0 || encodedBytes > MAX_BYTES)
            throw new IllegalArgumentException("Native Programs payload exceeds protocol limit");
        return (int) encodedBytes;
    }
}
