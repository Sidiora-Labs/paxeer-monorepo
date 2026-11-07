package com.sidiora.layerx.spring;

import jakarta.servlet.http.HttpServletRequest;
import java.io.IOException;

@FunctionalInterface
public interface LayerXResourceHandler {
    LayerXResource release(HttpServletRequest request) throws IOException;

    interface Recoverable extends LayerXResourceHandler {
        LayerXResource release(HttpServletRequest request, String idempotencyKey) throws IOException;
        ResourceRelease.Recovery recover(HttpServletRequest request, String idempotencyKey) throws IOException;

        @Override
        default LayerXResource release(HttpServletRequest request) throws IOException {
            throw new IOException("recoverable resource handler requires an idempotency key");
        }
    }
}
