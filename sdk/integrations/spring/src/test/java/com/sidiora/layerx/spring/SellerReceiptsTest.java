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
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import static org.junit.jupiter.api.Assertions.*;

public final class SellerReceiptsTest {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final byte[] RESOURCE = "recorded native payment released this resource"
        .getBytes(StandardCharsets.UTF_8);

    @TempDir(factory = ReceiptKeysConfigTest.OwnerStorage.class)
    Path storageDirectory;

    private static JsonNode vector() throws Exception {
        return recording("V1-layerxledger-pay-exact");
    }

    private static JsonNode recording(String id) throws Exception {
        Path fixture = Path.of("../../platform/conformance/fixtures/sequencer-receipt-v1.json");
        JsonNode transcript = JSON.readTree(fixture.toFile());
        assertEquals("rpc/sequencer_receipt_vectors_test.go",
            transcript.required("provenance").required("generator").textValue());
        for (JsonNode entry : transcript.required("cases")) {
            if (id.equals(entry.required("id").textValue())) return entry;
        }
        throw new AssertionError("missing recorded conformance case " + id);
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

    private static X402.PaymentRequirements requirements(JsonNode vector) {
        JsonNode log = vector.required("receiptJson").required("logs").get(0);
        byte[] data = bytes(log, "data");
        ObjectNode extra = JSON.createObjectNode();
        extra.putObject("layerx").put("commitment", "receipted")
            .put("purposeHash", log.required("topics").get(1).textValue().substring(2))
            .put("payer", log.required("topics").get(2).textValue().substring(2));
        return new X402.PaymentRequirements("exact", "eip155:125",
            new BigInteger(1, Arrays.copyOfRange(data, 32, 64)).toString(),
            "0x" + X402.hex(Arrays.copyOf(data, 32)), log.required("topics").get(3).textValue(), 120, extra);
    }

    private static ObjectNode evidence(JsonNode vector, LocalVerifier.ReceiptKey key) {
        return X402.ReceiptEvidence.receipted(vector.required("rawTransaction").textValue(),
            vector.required("signedReceipt").textValue(),
            "0x" + X402.hex(NativePaymentVerifier.receiptDigest(bytes(vector, "signedReceipt"))), key).toNode();
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
    public void settlesSignedSequencerReceipt() throws Exception {
        JsonNode vector = vector();
        LocalVerifier.ReceiptKey key = pinned(vector);
        LocalVerifier.ReceiptKeys keys = new LocalVerifier.ReceiptKeys(List.of(key));
        var verified = LocalVerifier.verifySequencerReceipt(bytes(vector, "signedReceipt"),
            bytes(vector, "rawTransaction"), keys);
        X402.PaymentRequirements requirements = requirements(vector);
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
        assertEquals(BigInteger.ZERO, verified.receipt().amount());
        assertEquals("000000000000000000000000000000000000000000000000000000000000101a",
            X402.hex(verified.receipt().to()));
        assertEquals("1000000", requirements.amount());
        assertEquals(vector.required("receiptJson").required("txHash").textValue(),
            decision.settlement().transaction());
        assertEquals("0x" + X402.hex(verified.receipt().activityId()), decision.settlement().transaction());
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
        MiddlewareException retiredError = assertThrows(MiddlewareException.class, () ->
            seller.handle("merchant", X402.encodePaymentPayloadHeader(payload(retired, requirements)),
                () -> { throw new AssertionError("retired receipt released a resource"); }));
        assertEquals(MiddlewareException.Code.INVALID_PAYMENT_PAYLOAD, retiredError.code());
        assertEquals(1, releases.get());
    }

    @Test
    void durableStoreRoundTripsReceiptKey() throws Exception {
        durableStoreRoundTripsReceiptKey(storageDirectory);
    }

    public void durableStoreRoundTripsReceiptKey(Path storageDirectory) throws Exception {
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
            new LocalVerifier.ReceiptKeys(List.of(restored.receiptKey())), requirements(vector));
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

    @Test
    public void nativePaymentRefusesMismatchedTermsAndNonpaymentReceipts() throws Exception {
        JsonNode vector = vector();
        LocalVerifier.ReceiptKey key = pinned(vector);
        ObjectNode valid = requirements(vector).toNode();
        List<ObjectNode> invalid = new ArrayList<>();
        invalid.add(valid.deepCopy().put("amount", "999999"));
        invalid.add(valid.deepCopy().put("asset", "0x" + "11".repeat(32)));
        invalid.add(valid.deepCopy().put("payTo", "0x" + "22".repeat(32)));
        invalid.add(valid.deepCopy().put("scheme", "unsupported"));
        for (String field : List.of("purposeHash", "payer")) {
            ObjectNode changed = valid.deepCopy();
            ((ObjectNode) changed.required("extra").required("layerx")).put(field, "33".repeat(32));
            invalid.add(changed);
        }
        for (String scheme : List.of("metered", "subscription")) {
            ObjectNode missingPurpose = valid.deepCopy().put("scheme", scheme);
            ((ObjectNode) missingPurpose.required("extra").required("layerx")).remove("purposeHash");
            invalid.add(missingPurpose);
        }
        ObjectNode stronger = valid.deepCopy();
        ((ObjectNode) stronger.required("extra").required("layerx")).put("commitment", "committed");
        invalid.add(stronger);
        for (ObjectNode terms : invalid) {
            X402.PaymentRequirements offer = X402.parseRequirements(terms);
            refuses(seller(vector, offer), evidence(vector, key), offer);
        }
        X402.PaymentRequirements offer = requirements(vector);
        SellerMiddleware seller = seller(vector, offer);
        for (String id : List.of("V1-pax-transfer", "V1-erc20-approve", "V2-layerxledger-revert")) {
            JsonNode nonpayment = recording(id);
            refuses(seller, evidence(nonpayment, pinned(nonpayment)), offer);
        }
        byte[] wrongChain = bytes(vector, "rawTransaction");
        assertEquals(125, wrongChain[4]);
        wrongChain[4] = 1;
        refuses(seller, evidence(vector, key).put("rawTransaction", "0x" + X402.hex(wrongChain)), offer);
        for (String network : List.of("layerx:mainnet", "eip155:1", "eip155:0125")) {
            assertEquals(MiddlewareException.Code.UNSUPPORTED_PAYMENT,
                assertThrows(MiddlewareException.class, () ->
                    X402.parseNetwork(JSON.getNodeFactory().textNode(network))).code());
        }
    }

    @Test
    public void registeredFilterRoundTripsCurrentEvidenceToBuyerAndReplays() throws Exception {
        JsonNode vector = vector();
        LocalVerifier.ReceiptKey key = pinned(vector);
        ObjectNode proof = evidence(vector, key);
        Set<String> fields = new HashSet<>();
        proof.fieldNames().forEachRemaining(fields::add);
        assertEquals(Set.of("format", "verificationLevel", "rawTransaction", "signedReceipt",
            "receiptDigest", "receiptKey"), fields);
        assertEquals("layerx-receipt-proof-v2", proof.required("format").textValue());
        assertEquals("receipted", proof.required("verificationLevel").textValue());
        assertEquals("0x3e8", proof.required("receiptKey").required("firstHeight").textValue());
        assertEquals("0x7cf", proof.required("receiptKey").required("lastHeight").textValue());
        for (String scheme : List.of("exact", "metered", "subscription")) {
            X402.PaymentRequirements offer = X402.parseRequirements(
                requirements(vector).toNode().put("scheme", scheme));
            try (PaymentServer server = new PaymentServer(vector, offer)) {
                HttpResponse<byte[]> challenge = server.get(null);
                assertEquals(402, challenge.statusCode());
                assertEquals(0, server.releases.get());
                X402.PaymentRequired required = X402.decodePaymentRequiredHeader(
                    challenge.headers().firstValue(X402.PAYMENT_REQUIRED_HEADER).orElseThrow());
                assertEquals("eip155:125", required.accepts().get(0).network());
                assertEquals(offer.toNode(), required.accepts().get(0).toNode());
                assertEquals(X402.canonicalJson(required.toNode()),
                    X402.canonicalJson(JSON.readTree(challenge.body())));
                String header = X402.encodePaymentPayloadHeader(payload(proof, required.accepts().get(0)));
                assertEquals(proof,
                    X402.parseReceiptEvidence(X402.decodePaymentPayloadHeader(header).payload()).toNode());
                HttpResponse<byte[]> paid = server.get(header);
                assertEquals(200, paid.statusCode(), scheme);
                assertArrayEquals(RESOURCE, paid.body());
                assertEquals("text/plain", paid.headers().firstValue("content-type").orElseThrow());
                String responseHeader = paid.headers().firstValue(X402.PAYMENT_RESPONSE_HEADER).orElseThrow();
                X402.SettlementResponse settlement = X402.decodeSettlementHeader(responseHeader);
                assertTrue(settlement.success());
                assertEquals(offer.network(), settlement.network());
                assertEquals(offer.amount(), settlement.amount());
                assertEquals(proof, settlement.extensions().get("layerx"));
                X402.ReceiptEvidence consumed = X402.parseReceiptEvidence(payload(
                    (ObjectNode) settlement.extensions().get("layerx"), offer).payload());
                byte[] raw = X402.parseReceiptHex(consumed.rawTransaction(),
                    MiddlewareException.Code.VERIFICATION_FAILURE);
                byte[] signed = X402.parseReceiptHex(consumed.signedReceipt(),
                    MiddlewareException.Code.VERIFICATION_FAILURE);
                LocalVerifier.ReceiptKeys pins = new LocalVerifier.ReceiptKeys(List.of(key));
                var decoded = LocalVerifier.verifySequencerReceipt(signed, raw, pins);
                var buyer = LocalVerifier.verifySequencerReceipt(signed, raw, pins,
                    vector.required("receiptJson"), decoded.receipt().previousStateRoot());
                buyer.requireSuccess();
                assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, buyer.level());
                assertArrayEquals(bytes(vector, "signedReceipt"), buyer.canonicalBytes());
                assertEquals("0x" + X402.hex(buyer.receipt().activityId()), settlement.transaction());
                assertEquals(vector.required("receiptJson").required("txHash").textValue(),
                    settlement.transaction());
                assertEquals(X402.hex(buyer.receipt().from()), settlement.payer());
                assertEquals("0x" + X402.hex(NativePaymentVerifier.receiptDigest(signed)),
                    consumed.receiptDigest());
                assertEquals(settlement.transaction(), paid.headers().firstValue("layerx-transaction").orElseThrow());
                assertEquals(consumed.receiptDigest().substring(2),
                    paid.headers().firstValue("layerx-receipt-digest").orElseThrow());
                assertEquals(1, server.releases.get());
                HttpResponse<byte[]> replay = server.get(header);
                assertEquals(200, replay.statusCode());
                assertArrayEquals(paid.body(), replay.body());
                assertEquals(responseHeader,
                    replay.headers().firstValue(X402.PAYMENT_RESPONSE_HEADER).orElseThrow());
                assertEquals(1, server.releases.get());
            }
        }
    }

    @Test
    public void registeredFilterRefusesAlteredPortableEvidenceBeforeRelease() throws Exception {
        JsonNode vector = vector();
        ObjectNode proof = evidence(vector, pinned(vector));
        List<ObjectNode> invalid = new ArrayList<>();
        ObjectNode missing = proof.deepCopy();
        missing.remove("format");
        invalid.add(missing);
        invalid.add(proof.deepCopy().put("format", "layerx-receipt-proof-v1"));
        invalid.add(proof.deepCopy().put("verificationLevel", "sequencer-signed"));
        invalid.add(proof.deepCopy().put("unexpected", true));
        invalid.add(proof.deepCopy().put("rawTransaction", "0x0102"));
        invalid.add(proof.deepCopy().put("receiptDigest", "0x" + "11".repeat(32)));
        byte[] signed = bytes(vector, "signedReceipt");
        signed[signed.length - 1] ^= 1;
        invalid.add(proof.deepCopy().put("signedReceipt", "0x" + X402.hex(signed))
            .put("receiptDigest", "0x" + X402.hex(NativePaymentVerifier.receiptDigest(signed))));
        for (String field : List.of("keyId", "publicKey", "firstHeight", "lastHeight")) {
            ObjectNode changed = proof.deepCopy();
            ((ObjectNode) changed.required("receiptKey")).put(field,
                field.endsWith("Height") ? "0x3e9" : "0x" + "55".repeat(32));
            invalid.add(changed);
        }
        try (PaymentServer server = new PaymentServer(vector, requirements(vector))) {
            for (ObjectNode rejected : invalid) {
                HttpResponse<byte[]> response = server.get(X402.encodePaymentPayloadHeader(
                    payload(rejected, requirements(vector))));
                assertEquals(402, response.statusCode(), rejected.toString());
                assertTrue(response.headers().firstValue(X402.PAYMENT_RESPONSE_HEADER).isEmpty());
                assertTrue(JSON.readTree(response.body()).required("error").isTextual());
                assertEquals(0, server.releases.get());
            }
            assertEquals(200, server.get(X402.encodePaymentPayloadHeader(
                payload(proof, requirements(vector)))).statusCode());
            assertEquals(1, server.releases.get());
        }
    }

    private static SellerMiddleware seller(JsonNode vector, X402.PaymentRequirements offer) {
        return new SellerMiddleware(new X402.PaymentRequired(
            X402.ResourceInfo.of("https://merchant.example/paid"), List.of(offer), null, null),
            new SellerMiddleware.ReceiptPayloadAuthority(new LocalVerifier.ReceiptKeys(List.of(pinned(vector)))),
            new Fulfillments.InMemoryFulfillmentRepository());
    }

    private static final class PaymentServer implements AutoCloseable {
        final AtomicInteger releases = new AtomicInteger();
        private final AtomicReference<Throwable> failure = new AtomicReference<>();
        private final ExecutorService executor = Executors.newVirtualThreadPerTaskExecutor();
        private final HttpServer server;
        private final HttpClient client = HttpClient.newHttpClient();
        private final URI uri;

        PaymentServer(JsonNode recording, X402.PaymentRequirements offer) throws Exception {
            X402.PaymentRequired required = new X402.PaymentRequired(
                X402.ResourceInfo.of("https://merchant.example/paid"), List.of(offer), null, null);
            LayerXDeclaredConfig config = new LayerXDeclaredConfig("merchant", "/paid", required, offer,
                new LocalVerifier.ReceiptKeys(List.of(pinned(recording))), "/webhooks", Map.of(), 1000, 1000);
            LayerXAutoConfiguration auto = new LayerXAutoConfiguration();
            var registration = auto.layerXPaymentGateRegistration(config,
                auto.layerXSellerMiddleware(config, auto.layerXPaymentAuthority(config),
                    new Fulfillments.InMemoryFulfillmentRepository()), request -> {
                releases.incrementAndGet();
                return new LayerXResource("text/plain", RESOURCE);
            });
            assertEquals(Set.of("/paid", "/paid/*"), new HashSet<>(registration.getUrlPatterns()));
            LayerXPaymentGateFilter filter = registration.getFilter();
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
            HttpRequest.Builder request = HttpRequest.newBuilder(uri).timeout(Duration.ofSeconds(10)).GET();
            if (paymentHeader != null) request.header(X402.PAYMENT_SIGNATURE_HEADER, paymentHeader);
            HttpResponse<byte[]> response = client.send(request.build(), HttpResponse.BodyHandlers.ofByteArray());
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
