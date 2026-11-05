package com.sidiora.layerx.spring;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import jakarta.servlet.DispatcherType;
import jakarta.servlet.ServletOutputStream;
import jakarta.servlet.WriteListener;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.lang.reflect.Proxy;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Base64;
import java.util.Collections;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import org.bouncycastle.crypto.digests.KeccakDigest;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import static org.junit.jupiter.api.Assertions.*;

public final class Repair2233620Test {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final byte[] RESOURCE = "recorded native payment released this resource"
        .getBytes(StandardCharsets.UTF_8);

    private static JsonNode recording(String id) throws Exception {
        Path source = Path.of("../../platform/conformance/fixtures/sequencer-receipt-v1.json");
        JsonNode transcript = JSON.readTree(source.toFile());
        assertEquals("rpc/sequencer_receipt_vectors_test.go",
            transcript.required("provenance").required("generator").textValue());
        for (JsonNode entry : transcript.required("cases")) {
            if (id.equals(entry.required("id").textValue())) return entry;
        }
        throw new AssertionError("missing recorded node transaction " + id);
    }

    private static JsonNode payment() throws Exception {
        return recording("V1-layerxledger-pay-exact");
    }

    private static byte[] bytes(JsonNode node, String field) {
        return X402.parseReceiptHex(node.required(field).textValue(),
            MiddlewareException.Code.INVALID_PAYMENT_PAYLOAD);
    }

    private static String hex(byte[] value) { return "0x" + X402.hex(value); }

    private static byte[] keccak(byte[] value) {
        KeccakDigest digest = new KeccakDigest(256);
        digest.update(value, 0, value.length);
        byte[] result = new byte[32];
        digest.doFinal(result, 0);
        return result;
    }

    private static LocalVerifier.ReceiptKey key(JsonNode recording) {
        JsonNode pin = recording.required("pinnedKey");
        return new LocalVerifier.ReceiptKey(bytes(pin, "keyId"), bytes(pin, "publicKey"),
            pin.required("firstHeight").bigIntegerValue(), pin.required("lastHeight").bigIntegerValue());
    }

    private static LocalVerifier.ReceiptKeys keys(JsonNode recording) {
        return new LocalVerifier.ReceiptKeys(List.of(key(recording)));
    }

    private static ObjectNode evidence(JsonNode recording) {
        return X402.ReceiptEvidence.receipted(recording.required("rawTransaction").textValue(),
            recording.required("signedReceipt").textValue(), hex(keccak(bytes(recording, "signedReceipt"))),
            key(recording)).toNode();
    }

    private static ObjectNode terms(JsonNode recording) {
        JsonNode log = recording.required("receiptJson").required("logs").get(0);
        ObjectNode extra = JSON.createObjectNode();
        extra.putObject("layerx").put("commitment", "receipted")
            .put("purposeHash", log.required("topics").get(1).textValue().substring(2))
            .put("payer", log.required("topics").get(2).textValue().substring(2));
        return extra;
    }

    private static X402.PaymentRequirements offer(JsonNode recording, String scheme, JsonNode extra) {
        JsonNode log = recording.required("receiptJson").required("logs").get(0);
        byte[] data = bytes(log, "data");
        return new X402.PaymentRequirements(scheme, "eip155:125",
            new BigInteger(1, Arrays.copyOfRange(data, 32, 64)).toString(),
            hex(Arrays.copyOf(data, 32)), log.required("topics").get(3).textValue(), 120, extra);
    }

    private static X402.PaymentRequirements offer(JsonNode recording) {
        return offer(recording, "exact", terms(recording));
    }

    private static X402.PaymentRequired required(X402.PaymentRequirements offer) {
        return new X402.PaymentRequired(X402.ResourceInfo.of("https://merchant.example/paid"),
            List.of(offer), null, null);
    }

    private static Map<String, JsonNode> fields(JsonNode object) {
        Map<String, JsonNode> fields = new LinkedHashMap<>();
        object.fields().forEachRemaining(entry -> fields.put(entry.getKey(), entry.getValue()));
        return fields;
    }

    private static String header(JsonNode evidence, X402.PaymentRequirements offer) {
        return X402.encodePaymentPayloadHeader(new X402.PaymentPayload(fields(evidence), offer, null, null));
    }

    @TempDir
    Path storage;

    private record Payment(JsonNode recording, X402.PaymentRequirements offer, String header) {
        static Payment recorded() throws Exception {
            JsonNode node = payment();
            X402.PaymentRequirements offer = Repair2233620Test.offer(node);
            return new Payment(node, offer, Repair2233620Test.header(evidence(node), offer));
        }

        SellerMiddleware seller(Fulfillments.FulfillmentRepository repository) {
            return new SellerMiddleware(required(offer),
                new SellerMiddleware.ReceiptPayloadAuthority(keys(recording)), repository);
        }

        String idempotencyKey() {
            X402.PaymentPayload payload = X402.decodePaymentPayloadHeader(header);
            return X402.paymentIdempotencyKey("merchant",
                X402.sha256(X402.canonicalJson(payload.toNode()).getBytes(StandardCharsets.UTF_8)));
        }

        void assertResponse(HttpResponse<byte[]> response) {
            assertEquals(200, response.statusCode());
            assertArrayEquals(RESOURCE, response.body());
            X402.SettlementResponse settlement = X402.decodeSettlementHeader(
                response.headers().firstValue(X402.PAYMENT_RESPONSE_HEADER).orElseThrow());
            X402.ReceiptEvidence proof = X402.parseReceiptEvidence(fields(settlement.extensions().get("layerx")));
            assertEquals(evidence(recording), proof.toNode());
            var verified = SellerMiddleware.verifyPaymentReceipt(
                X402.parseReceiptHex(proof.signedReceipt(), MiddlewareException.Code.VERIFICATION_FAILURE),
                X402.parseReceiptHex(proof.rawTransaction(), MiddlewareException.Code.VERIFICATION_FAILURE),
                keys(recording), offer);
            verified.requireSuccess();
            assertEquals("0x" + X402.hex(verified.receipt().activityId()), settlement.transaction());
        }
    }

    private static final class InterruptedRelease extends IOException {
        InterruptedRelease(String message) { super(message); }
    }

    // The peer authority retains the external effect across seller/store restarts.
    private static final class EffectAuthority implements LayerXResourceHandler.Recoverable {
        private final Map<String, LayerXResource> results = new ConcurrentHashMap<>();
        private final List<String> keys = Collections.synchronizedList(new ArrayList<>());
        private final AtomicInteger releases = new AtomicInteger();
        private final AtomicInteger recoveries = new AtomicInteger();
        private final AtomicInteger effects = new AtomicInteger();
        private String interruption;
        private final Path storage;

        EffectAuthority(Path storage, String interruption) {
            this.storage = storage;
            this.interruption = interruption;
        }

        @Override
        public LayerXResource release(HttpServletRequest request, String key) throws IOException {
            assertEquals("claimed", admission(storage).required("state").textValue());
            keys.add(key);
            releases.incrementAndGet();
            if ("before".equals(interruption)) throw new InterruptedRelease("stopped before external release");
            LayerXResource resource = new LayerXResource("text/plain", RESOURCE);
            assertNull(results.putIfAbsent(key, resource), "a completed effect must be recovered, not released again");
            effects.incrementAndGet();
            if ("after".equals(interruption)) throw new InterruptedRelease("stopped after external release");
            return resource;
        }

        @Override
        public ResourceRelease.Recovery recover(HttpServletRequest request, String key) throws IOException {
            keys.add(key);
            recoveries.incrementAndGet();
            if ("unknown".equals(interruption)) throw new InterruptedRelease("external outcome is uncertain");
            LayerXResource result = results.get(key);
            return result == null ? new ResourceRelease.NotReleased() : new ResourceRelease.Recovered(result);
        }
    }

    private static JsonNode admission(Path root) throws IOException {
        try (var paths = Files.list(root.resolve("fulfillments"))) {
            List<Path> records = paths.filter(path -> path.toString().endsWith(".json")).toList();
            assertEquals(1, records.size());
            return JSON.readTree(records.get(0).toFile());
        }
    }

    private void assertClaim(Payment payment) throws IOException {
        JsonNode claim = admission(storage);
        assertEquals("claimed", claim.required("state").textValue());
        assertEquals(payment.idempotencyKey(), claim.required("idempotencyKey").textValue());
        assertArrayEquals(bytes(payment.recording(), "signedReceipt"),
            Base64.getDecoder().decode(claim.required("canonicalReceipt").textValue()));
        assertFalse(claim.has("resource"));
    }

    @Test
    void finding028CrashBeforeReleaseReopensAndReconcilesBeforeExecuting() throws Exception {
        Payment payment = Payment.recorded();
        EffectAuthority effect = new EffectAuthority(storage, "before");
        try (PaymentServer server = new PaymentServer(payment, DurableStores.fulfillments(storage), effect)) {
            assertEquals(500, server.get(payment.header()).statusCode());
        }
        assertClaim(payment);
        assertEquals(0, effect.effects.get());
        effect.interruption = null;
        try (PaymentServer reopened = new PaymentServer(payment, DurableStores.fulfillments(storage), effect)) {
            payment.assertResponse(reopened.get(payment.header()));
        }
        assertEquals(1, effect.recoveries.get());
        assertEquals(2, effect.releases.get());
        assertEquals(1, effect.effects.get());
        assertEquals(List.of(payment.idempotencyKey(), payment.idempotencyKey(), payment.idempotencyKey()), effect.keys);
        assertEquals("completed", admission(storage).required("state").textValue());
        var completed = DurableStores.fulfillments(storage);
        ResourceRelease forbidden = () -> {
            throw new AssertionError("a stored result or conflict must not execute an effect");
        };
        for (var conflict : conflicts(proposed(payment))) {
            assertEquals(MiddlewareException.Code.FULFILLMENT_CONFLICT,
                assertThrows(MiddlewareException.class, () -> completed.fulfill(conflict, forbidden)).code());
        }
        try (PaymentServer reopened = new PaymentServer(payment, DurableStores.fulfillments(storage), effect)) {
            payment.assertResponse(reopened.get(payment.header()));
        }
        assertEquals(1, effect.recoveries.get());
        assertEquals(2, effect.releases.get());
    }

    @Test
    void finding028CrashAfterReleaseReopensAndRecoversWithoutRepeatingEffect() throws Exception {
        Payment payment = Payment.recorded();
        EffectAuthority effect = new EffectAuthority(storage, "after");
        try (PaymentServer server = new PaymentServer(payment, DurableStores.fulfillments(storage), effect)) {
            assertEquals(500, server.get(payment.header()).statusCode());
        }
        assertClaim(payment);
        assertEquals(1, effect.effects.get());
        effect.interruption = "unknown";
        try (PaymentServer reopened = new PaymentServer(payment, DurableStores.fulfillments(storage), effect)) {
            assertEquals(500, reopened.get(payment.header()).statusCode());
        }
        assertClaim(payment);
        assertEquals(1, effect.releases.get());
        effect.interruption = null;
        try (PaymentServer reopened = new PaymentServer(payment, DurableStores.fulfillments(storage), effect)) {
            payment.assertResponse(reopened.get(payment.header()));
            payment.assertResponse(reopened.get(payment.header()));
        }
        assertEquals(2, effect.recoveries.get());
        assertEquals(1, effect.releases.get());
        assertEquals(1, effect.effects.get());
        assertEquals(List.of(payment.idempotencyKey(), payment.idempotencyKey(), payment.idempotencyKey()), effect.keys);
        assertEquals("completed", admission(storage).required("state").textValue());
    }

    private static Fulfillments.ProposedFulfillment proposed(Payment payment) {
        X402.PaymentPayload decoded = X402.decodePaymentPayloadHeader(payment.header());
        X402.ReceiptEvidence proof = X402.parseReceiptEvidence(decoded.payload());
        byte[] signed = X402.parseReceiptHex(proof.signedReceipt(), MiddlewareException.Code.VERIFICATION_FAILURE);
        byte[] raw = X402.parseReceiptHex(proof.rawTransaction(), MiddlewareException.Code.VERIFICATION_FAILURE);
        var verified = SellerMiddleware.verifyPaymentReceipt(signed, raw, keys(payment.recording()), payment.offer());
        return new Fulfillments.ProposedFulfillment(payment.idempotencyKey(),
            X402.hex(X402.sha256(X402.canonicalJson(decoded.toNode()).getBytes(StandardCharsets.UTF_8))),
            signed, proof.receiptKey(), verified.receipt().activityId());
    }

    private static List<Fulfillments.ProposedFulfillment> conflicts(Fulfillments.ProposedFulfillment original) {
        byte[] receipt = original.canonicalReceipt().clone();
        receipt[0] ^= 1;
        byte[] hash = original.transactionHash().clone();
        hash[0] ^= 1;
        var key = original.receiptKey();
        byte[] publicKey = key.publicKey().clone();
        publicKey[0] ^= 1;
        var changedKey = new LocalVerifier.ReceiptKey(key.keyId(), publicKey, key.firstHeight(), key.lastHeight());
        return List.of(
            new Fulfillments.ProposedFulfillment(original.idempotencyKey(), "different-digest",
                original.canonicalReceipt(), key, original.transactionHash()),
            new Fulfillments.ProposedFulfillment(original.idempotencyKey(), original.requestDigest(), receipt, key,
                original.transactionHash()),
            new Fulfillments.ProposedFulfillment(original.idempotencyKey(), original.requestDigest(),
                original.canonicalReceipt(), changedKey, original.transactionHash()),
            new Fulfillments.ProposedFulfillment(original.idempotencyKey(), original.requestDigest(),
                original.canonicalReceipt(), key, hash));
    }

    @Test
    void finding028InterruptedLegacyReleaseAndConflictingClaimsCannotReplay() throws Exception {
        Payment payment = Payment.recorded();
        AtomicInteger releases = new AtomicInteger();
        ResourceRelease interrupted = () -> {
            releases.incrementAndGet();
            throw new InterruptedRelease("legacy effect interrupted");
        };
        assertThrows(InterruptedRelease.class,
            () -> payment.seller(DurableStores.fulfillments(storage)).handle("merchant", payment.header(), interrupted));
        assertClaim(payment);
        var reopened = DurableStores.fulfillments(storage);
        for (var conflict : conflicts(proposed(payment))) {
            assertEquals(MiddlewareException.Code.FULFILLMENT_CONFLICT,
                assertThrows(MiddlewareException.class, () -> reopened.fulfill(conflict, interrupted)).code());
        }
        assertThrows(IOException.class,
            () -> payment.seller(reopened).handle("merchant", payment.header(), interrupted));
        assertEquals(1, releases.get());
        assertClaim(payment);
    }

    @Test
    void finding029SimultaneousEqualRequestsReleaseOnceAndOtherKeysProceed() throws Exception {
        Payment payment = Payment.recorded();
        var repository = new Fulfillments.InMemoryFulfillmentRepository();
        var seller = payment.seller(repository);
        AtomicInteger releases = new AtomicInteger();
        CountDownLatch firstRelease = new CountDownLatch(1);
        CountDownLatch allowRelease = new CountDownLatch(1);
        CountDownLatch secondRequest = new CountDownLatch(1);
        ResourceRelease release = () -> {
            if (releases.incrementAndGet() == 1) {
                firstRelease.countDown();
                try {
                    if (!allowRelease.await(10, TimeUnit.SECONDS)) throw new IOException("release was not unblocked");
                } catch (InterruptedException error) {
                    Thread.currentThread().interrupt();
                    throw new IOException(error);
                }
            }
            return new LayerXResource("text/plain", RESOURCE);
        };
        try (ExecutorService executor = Executors.newVirtualThreadPerTaskExecutor()) {
            Future<SellerMiddleware.SellerDecision> first = executor.submit(
                () -> seller.handle("merchant", payment.header(), release));
            try {
                assertTrue(firstRelease.await(5, TimeUnit.SECONDS));
                Future<SellerMiddleware.SellerDecision> second = executor.submit(() -> {
                    secondRequest.countDown();
                    return seller.handle("merchant", payment.header(), release);
                });
                assertTrue(secondRequest.await(5, TimeUnit.SECONDS));
                assertThrows(TimeoutException.class, () -> second.get(200, TimeUnit.MILLISECONDS));
                Future<SellerMiddleware.SellerDecision> other = executor.submit(() ->
                    seller.handle("other-principal", payment.header(), () -> new LayerXResource("text/plain", RESOURCE)));
                var otherDecision = (SellerMiddleware.ReleasedDecision) other.get(5, TimeUnit.SECONDS);
                SellerMiddleware.assertReceiptBacked(otherDecision, payment.offer());
                allowRelease.countDown();
                var firstDecision = (SellerMiddleware.ReleasedDecision) first.get(5, TimeUnit.SECONDS);
                var secondDecision = (SellerMiddleware.ReleasedDecision) second.get(5, TimeUnit.SECONDS);
                assertEquals(1, releases.get());
                assertEquals(firstDecision.header(), secondDecision.header());
                assertSame(firstDecision.resource(), secondDecision.resource());
                SellerMiddleware.assertReceiptBacked(firstDecision, payment.offer());
                SellerMiddleware.assertReceiptBacked(secondDecision, payment.offer());
                for (var conflict : conflicts(proposed(payment))) {
                    assertEquals(MiddlewareException.Code.FULFILLMENT_CONFLICT,
                        assertThrows(MiddlewareException.class, () -> repository.fulfill(conflict, release)).code());
                }
                assertEquals(1, releases.get());
            } finally {
                allowRelease.countDown();
            }
        }
    }

    private static final class PaymentServer implements AutoCloseable {
        private final AtomicReference<Throwable> failure = new AtomicReference<>();
        private final ExecutorService executor = Executors.newVirtualThreadPerTaskExecutor();
        private final HttpServer server;
        private final HttpClient client = HttpClient.newHttpClient();
        private final URI uri;

        PaymentServer(Payment payment, Fulfillments.FulfillmentRepository repository,
                      LayerXResourceHandler resources) throws Exception {
            LayerXDeclaredConfig config = new LayerXDeclaredConfig("merchant", "/paid", required(payment.offer()),
                payment.offer(), keys(payment.recording()), "/webhooks", Map.of(), 1000, 1000);
            LayerXPaymentGateFilter filter = new LayerXPaymentGateFilter(config, payment.seller(repository), resources);
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.setExecutor(executor);
            server.createContext("/paid", exchange -> {
                ServletExchange bridge = new ServletExchange(exchange);
                try {
                    filter.doFilter(bridge.request(), bridge.response(), (request, response) -> {
                        throw new AssertionError("protected payment path bypassed the filter");
                    });
                    byte[] body = bridge.body.toByteArray();
                    exchange.sendResponseHeaders(bridge.status, body.length == 0 ? -1 : body.length);
                    exchange.getResponseBody().write(body);
                } catch (InterruptedRelease expected) {
                    exchange.sendResponseHeaders(500, -1);
                } catch (Throwable error) {
                    failure.compareAndSet(null, error);
                    exchange.sendResponseHeaders(500, -1);
                } finally {
                    exchange.close();
                }
            });
            server.start();
            uri = URI.create("http://127.0.0.1:" + server.getAddress().getPort() + "/paid");
        }

        HttpResponse<byte[]> get(String paymentHeader) throws Exception {
            HttpRequest request = HttpRequest.newBuilder(uri).timeout(Duration.ofSeconds(10))
                .header(X402.PAYMENT_SIGNATURE_HEADER, paymentHeader).GET().build();
            HttpResponse<byte[]> response = client.send(request, HttpResponse.BodyHandlers.ofByteArray());
            assertNull(failure.get(), () -> "loopback HTTP exchange failed: " + failure.get());
            return response;
        }

        @Override
        public void close() {
            client.close();
            server.stop(0);
            executor.close();
        }
    }

    private static final class ServletExchange {
        private final HttpExchange exchange;
        private final Map<String, Object> attributes = new HashMap<>();
        private final ByteArrayOutputStream body = new ByteArrayOutputStream();
        private int status = 200;

        ServletExchange(HttpExchange exchange) { this.exchange = exchange; }

        HttpServletRequest request() {
            return (HttpServletRequest) Proxy.newProxyInstance(getClass().getClassLoader(),
                new Class<?>[]{HttpServletRequest.class}, (proxy, method, args) -> switch (method.getName()) {
                    case "getRequestURI", "getServletPath" -> exchange.getRequestURI().getRawPath();
                    case "getContextPath" -> "";
                    case "getMethod" -> exchange.getRequestMethod();
                    case "getDispatcherType" -> DispatcherType.REQUEST;
                    case "getAttribute" -> attributes.get((String) args[0]);
                    case "setAttribute" -> attributes.put((String) args[0], args[1]);
                    case "removeAttribute" -> attributes.remove((String) args[0]);
                    case "getAttributeNames" -> Collections.enumeration(attributes.keySet());
                    case "getHeader" -> exchange.getRequestHeaders().getFirst((String) args[0]);
                    case "getHeaders" -> Collections.enumeration(exchange.getRequestHeaders()
                        .getOrDefault((String) args[0], List.of()));
                    case "getHeaderNames" -> Collections.enumeration(exchange.getRequestHeaders().keySet());
                    case "isAsyncStarted", "isAsyncSupported" -> false;
                    default -> throw new UnsupportedOperationException("HTTP request adapter: " + method.getName());
                });
        }

        HttpServletResponse response() {
            ServletOutputStream output = new ServletOutputStream() {
                @Override public void write(int value) { body.write(value); }
                @Override public void write(byte[] bytes, int offset, int length) { body.write(bytes, offset, length); }
                @Override public boolean isReady() { return true; }
                @Override public void setWriteListener(WriteListener listener) {
                    throw new UnsupportedOperationException("loopback adapter is synchronous");
                }
            };
            return (HttpServletResponse) Proxy.newProxyInstance(getClass().getClassLoader(),
                new Class<?>[]{HttpServletResponse.class}, (proxy, method, args) -> {
                    switch (method.getName()) {
                        case "setStatus": status = (Integer) args[0]; return null;
                        case "getStatus": return status;
                        case "setHeader": exchange.getResponseHeaders().set((String) args[0], (String) args[1]); return null;
                        case "addHeader": exchange.getResponseHeaders().add((String) args[0], (String) args[1]); return null;
                        case "setContentType": exchange.getResponseHeaders().set("Content-Type", (String) args[0]); return null;
                        case "setContentLength", "setContentLengthLong":
                            exchange.getResponseHeaders().set("Content-Length", args[0].toString()); return null;
                        case "getOutputStream": return output;
                        case "getHeader": return exchange.getResponseHeaders().getFirst((String) args[0]);
                        case "getHeaders": return exchange.getResponseHeaders().getOrDefault((String) args[0], List.of());
                        case "getHeaderNames": return exchange.getResponseHeaders().keySet();
                        case "isCommitted": return false;
                        default: throw new UnsupportedOperationException("HTTP response adapter: " + method.getName());
                    }
                });
        }
    }
}
