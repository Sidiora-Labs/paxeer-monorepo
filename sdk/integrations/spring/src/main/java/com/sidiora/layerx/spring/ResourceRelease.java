package com.sidiora.layerx.spring;

import java.io.IOException;

@FunctionalInterface
public interface ResourceRelease {
    LayerXResource release() throws IOException;

    sealed interface Recovery permits NotReleased, Recovered {}

    /** The effect authority has established that no effect occurred for this key. */
    record NotReleased() implements Recovery {}

    record Recovered(LayerXResource resource) implements Recovery {
        public Recovered { java.util.Objects.requireNonNull(resource, "resource"); }
    }

    /**
     * Effects must durably bind their result to this key. Recovery must reconcile
     * with that authority, throwing IOException when the outcome is uncertain.
     */
    interface Recoverable extends ResourceRelease {
        LayerXResource release(String idempotencyKey) throws IOException;
        Recovery recover(String idempotencyKey) throws IOException;

        @Override
        default LayerXResource release() throws IOException {
            throw new IOException("recoverable resource release requires an idempotency key");
        }
    }
}
