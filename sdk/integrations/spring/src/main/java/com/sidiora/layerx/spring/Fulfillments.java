package com.sidiora.layerx.spring;

import com.sidiora.layerx.sdk.verify.LocalVerifier;
import java.io.IOException;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

public final class Fulfillments {
    private Fulfillments() {}

    public record ProposedFulfillment(String idempotencyKey, String requestDigest, byte[] canonicalReceipt,
                                      LocalVerifier.ReceiptKey receiptKey, byte[] transactionHash) {}

    public record StoredFulfillment(String idempotencyKey, String requestDigest, byte[] canonicalReceipt,
                                    LocalVerifier.ReceiptKey receiptKey, byte[] transactionHash, LayerXResource resource) {}

    public interface FulfillmentRepository {
        StoredFulfillment fulfill(ProposedFulfillment proposed, ResourceRelease release) throws IOException;
    }

    public static final class InMemoryFulfillmentRepository implements FulfillmentRepository {
        private final Map<String, StoredFulfillment> entries = new ConcurrentHashMap<>();

        @Override
        public StoredFulfillment fulfill(ProposedFulfillment proposed, ResourceRelease release) throws IOException {
            StoredFulfillment existing = entries.get(proposed.idempotencyKey());
            if (existing != null) return require(existing, proposed);
            LayerXResource resource = release.release();
            StoredFulfillment stored = new StoredFulfillment(proposed.idempotencyKey(), proposed.requestDigest(),
                proposed.canonicalReceipt(), proposed.receiptKey(), proposed.transactionHash(), resource);
            StoredFulfillment raced = entries.putIfAbsent(proposed.idempotencyKey(), stored);
            return raced == null ? stored : require(raced, proposed);
        }

        private static StoredFulfillment require(StoredFulfillment stored, ProposedFulfillment proposed) {
            if (!stored.requestDigest().equals(proposed.requestDigest())
                    || !java.util.Arrays.equals(stored.canonicalReceipt(), proposed.canonicalReceipt())
                    || !SellerMiddleware.sameKey(stored.receiptKey(), proposed.receiptKey())
                    || !java.util.Arrays.equals(stored.transactionHash(), proposed.transactionHash())) {
                throw MiddlewareException.of(MiddlewareException.Code.FULFILLMENT_CONFLICT);
            }
            return stored;
        }
    }
}
