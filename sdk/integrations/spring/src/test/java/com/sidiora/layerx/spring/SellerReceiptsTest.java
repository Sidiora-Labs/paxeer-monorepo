package com.sidiora.layerx.spring;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import static org.junit.jupiter.api.Assertions.*;

public final class SellerReceiptsTest {
    private static final ObjectMapper JSON = new ObjectMapper();

    @TempDir
    Path storageDirectory;

    private static JsonNode vector() throws Exception {
        Path fixture = Path.of("../../platform/conformance/fixtures/vector-v1-receipted-positive.json");
        for (JsonNode entry : JSON.readTree(fixture.toFile()).required("cases")) {
            if ("V1-pax-transfer".equals(entry.required("id").textValue())) return entry;
        }
        throw new AssertionError("missing V1-pax-transfer conformance case");
    }

    private static byte[] bytes(JsonNode node, String field) {
        return X402.parseReceiptHex(node.required(field).textValue(),
            MiddlewareException.Code.INVALID_PAYMENT_PAYLOAD);
    }

    private static LocalVerifier.ReceiptKey pinned(JsonNode vector) {
        JsonNode key = vector.required("pinnedKey");
        return new LocalVerifier.ReceiptKey(bytes(key, "keyId"), bytes(key, "publicKey"),
            key.required("firstHeight").bigIntegerValue(), key.required("lastHeight").bigIntegerValue());
    }

    private static X402.PaymentRequirements requirements(LocalVerifier.SequencerReceiptVerification verified) {
        return new X402.PaymentRequirements("exact", "layerx:mainnet", verified.receipt().amount().toString(),
            X402.hex(verified.receipt().asset()), X402.hex(verified.receipt().to()), 120, null);
    }

    private static ObjectNode evidence(JsonNode vector, LocalVerifier.ReceiptKey key) {
        return X402.ReceiptEvidence.receipted(vector.required("rawTransaction").textValue(),
            vector.required("signedReceipt").textValue(), vector.required("receiptDigest").textValue(), key)
            .toNode();
    }

    private static X402.PaymentPayload payload(ObjectNode evidence, X402.PaymentRequirements requirements) {
        Map<String, JsonNode> fields = new LinkedHashMap<>();
        evidence.fields().forEachRemaining(entry -> fields.put(entry.getKey(), entry.getValue()));
        return new X402.PaymentPayload(fields, requirements, null, null);
    }

    private static void refuses(SellerMiddleware seller, ObjectNode evidence,
                                X402.PaymentRequirements requirements) {
        MiddlewareException error = assertThrows(MiddlewareException.class, () ->
            seller.handle("merchant", X402.encodePaymentPayloadHeader(payload(evidence, requirements)),
                () -> { throw new AssertionError("invalid receipt released a resource"); }));
        assertEquals(MiddlewareException.Code.VERIFICATION_FAILURE, error.code());
    }

    @Test
    void settlesSignedSequencerReceipt() throws Exception {
        JsonNode vector = vector();
        LocalVerifier.ReceiptKey key = pinned(vector);
        LocalVerifier.ReceiptKeys keys = new LocalVerifier.ReceiptKeys(List.of(key));
        var verified = LocalVerifier.verifySequencerReceipt(bytes(vector, "signedReceipt"),
            bytes(vector, "rawTransaction"), keys);
        X402.PaymentRequirements requirements = requirements(verified);
        X402.PaymentRequired required = new X402.PaymentRequired(
            X402.ResourceInfo.of("https://merchant.example/paid"), List.of(requirements), null, null);
        SellerMiddleware.ReceiptPayloadAuthority authority = new SellerMiddleware.ReceiptPayloadAuthority(keys);
        ObjectNode evidence = evidence(vector, key);
        var settled = assertInstanceOf(SellerMiddleware.Settled.class, authority.settle(
            new SellerMiddleware.SettlementRequest("merchant", payload(evidence, requirements), requirements,
                "idempotency", "request")));
        assertSame(key, settled.receiptKey());
        assertArrayEquals(bytes(vector, "rawTransaction"), settled.rawTransaction());
        assertArrayEquals(bytes(vector, "signedReceipt"), settled.signedReceipt());

        SellerMiddleware seller = new SellerMiddleware(required, authority,
            new Fulfillments.InMemoryFulfillmentRepository());
        AtomicInteger releases = new AtomicInteger();
        String header = X402.encodePaymentPayloadHeader(payload(evidence, requirements));
        ResourceRelease release = () -> {
            releases.incrementAndGet();
            return new LayerXResource("text/plain", "paid resource".getBytes(StandardCharsets.UTF_8));
        };
        var decision = assertInstanceOf(SellerMiddleware.ReleasedDecision.class,
            seller.handle("merchant", header, release));
        assertEquals(200, decision.status());
        assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, decision.verification().level());
        assertArrayEquals(bytes(vector, "receiptDigest"), decision.verification().receiptDigest());
        assertEquals("lxp:" + X402.hex(verified.receiptDigest()), decision.settlement().transaction());
        assertEquals(X402.hex(verified.receipt().from()), decision.settlement().payer());
        assertEquals(evidence, decision.settlement().extensions().get("layerx"));
        SellerMiddleware.assertReceiptBacked(decision, requirements);
        assertInstanceOf(SellerMiddleware.ReleasedDecision.class, seller.handle("merchant", header, release));
        assertEquals(1, releases.get());

        for (String field : List.of("keyId", "publicKey", "firstHeight", "lastHeight")) {
            ObjectNode changed = evidence.deepCopy();
            ObjectNode carriedKey = (ObjectNode) changed.required("receiptKey");
            carriedKey.put(field, field.endsWith("Height") ? "0x3e9" : "0x" + "55".repeat(32));
            refuses(seller, changed, requirements);
        }
        ObjectNode wrongDigest = evidence.deepCopy();
        wrongDigest.put("receiptDigest", "0x" + "11".repeat(32));
        refuses(seller, wrongDigest, requirements);
        ObjectNode wrongTransaction = evidence.deepCopy();
        wrongTransaction.put("rawTransaction", "0x0102");
        refuses(seller, wrongTransaction, requirements);
        ObjectNode wrongSignature = evidence.deepCopy();
        String signedReceipt = vector.required("signedReceipt").textValue();
        wrongSignature.put("signedReceipt", signedReceipt.substring(0, signedReceipt.length() - 2) + "00");
        refuses(seller, wrongSignature, requirements);
        ObjectNode retired = JSON.createObjectNode();
        retired.put("verificationLevel", "sequencer-signed");
        retired.put("receipt", X402.encodeBase64(bytes(vector, "signedReceipt")));
        retired.put("receiptDigest", vector.required("receiptDigest").textValue());
        refuses(seller, retired, requirements);
        assertEquals(1, releases.get());
    }

    @Test
    void durableStoreRoundTripsReceiptKey() throws Exception {
        JsonNode vector = vector();
        LocalVerifier.ReceiptKey fixtureKey = pinned(vector);
        LocalVerifier.ReceiptKey key = new LocalVerifier.ReceiptKey(fixtureKey.keyId(), fixtureKey.publicKey(),
            fixtureKey.firstHeight(), BigInteger.ONE.shiftLeft(64).subtract(BigInteger.ONE));
        var verified = LocalVerifier.verifySequencerReceipt(bytes(vector, "signedReceipt"),
            bytes(vector, "rawTransaction"), new LocalVerifier.ReceiptKeys(List.of(key)));
        var proposed = new Fulfillments.ProposedFulfillment("payment-id", "request-digest",
            bytes(vector, "signedReceipt"), key, verified.receipt().activityId());
        AtomicInteger releases = new AtomicInteger();
        ResourceRelease release = () -> {
            releases.incrementAndGet();
            return new LayerXResource("text/plain", "durable resource".getBytes(StandardCharsets.UTF_8));
        };
        var stored = DurableStores.fulfillments(storageDirectory).fulfill(proposed, release);
        var restored = DurableStores.fulfillments(storageDirectory).fulfill(proposed, release);
        assertEquals(1, releases.get());
        assertEquals(proposed.idempotencyKey(), restored.idempotencyKey());
        assertEquals(proposed.requestDigest(), restored.requestDigest());
        assertArrayEquals(proposed.canonicalReceipt(), restored.canonicalReceipt());
        assertArrayEquals(key.keyId(), restored.receiptKey().keyId());
        assertArrayEquals(key.publicKey(), restored.receiptKey().publicKey());
        assertEquals(key.firstHeight(), restored.receiptKey().firstHeight());
        assertEquals(key.lastHeight(), restored.receiptKey().lastHeight());
        assertArrayEquals(verified.receipt().activityId(), restored.transactionHash());
        assertEquals(stored.resource().contentType(), restored.resource().contentType());
        assertArrayEquals(stored.resource().body(), restored.resource().body());
        SellerMiddleware.verifyPaymentReceipt(restored.canonicalReceipt(), bytes(vector, "rawTransaction"),
            new LocalVerifier.ReceiptKeys(List.of(restored.receiptKey())), requirements(verified));
        try (var files = Files.list(storageDirectory.resolve("fulfillments"))) {
            Path record = files.filter(path -> path.toString().endsWith(".json")).findFirst().orElseThrow();
            JsonNode encoded = JSON.readTree(record.toFile());
            assertFalse(encoded.has("authorizedBatch"));
            assertTrue(encoded.has("receiptKey"));
            assertTrue(encoded.has("transactionHash"));
        }
        var changedKey = new Fulfillments.ProposedFulfillment(proposed.idempotencyKey(), proposed.requestDigest(),
            proposed.canonicalReceipt(), fixtureKey, proposed.transactionHash());
        assertEquals(MiddlewareException.Code.FULFILLMENT_CONFLICT,
            assertThrows(MiddlewareException.class, () ->
                DurableStores.fulfillments(storageDirectory).fulfill(changedKey, release)).code());
        byte[] otherHash = proposed.transactionHash().clone();
        otherHash[0] ^= 1;
        var changedHash = new Fulfillments.ProposedFulfillment(proposed.idempotencyKey(), proposed.requestDigest(),
            proposed.canonicalReceipt(), key, otherHash);
        assertEquals(MiddlewareException.Code.FULFILLMENT_CONFLICT,
            assertThrows(MiddlewareException.class, () ->
                DurableStores.fulfillments(storageDirectory).fulfill(changedHash, release)).code());
        assertEquals(1, releases.get());
    }
}
