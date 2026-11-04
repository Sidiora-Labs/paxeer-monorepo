package com.sidiora.layerx.spring;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.fasterxml.jackson.databind.node.TextNode;
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
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Base64;
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
import org.bouncycastle.crypto.digests.KeccakDigest;
import org.junit.jupiter.api.Test;
import org.springframework.boot.context.properties.bind.Binder;
import org.springframework.boot.test.util.TestPropertyValues;
import org.springframework.core.env.StandardEnvironment;

import static org.junit.jupiter.api.Assertions.*;

public final class Repair2231417Test {
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

    private static SellerMiddleware seller(JsonNode recording, X402.PaymentRequirements offer) {
        return new SellerMiddleware(required(offer), new SellerMiddleware.ReceiptPayloadAuthority(keys(recording)),
            new Fulfillments.InMemoryFulfillmentRepository());
    }

    @Test
    void finding024RecordedBuyerPaymentCrossesFilterWithEip155ChainBinding() throws Exception {
        JsonNode recording = payment();
        var verified = LocalVerifier.verifySequencerReceipt(bytes(recording, "signedReceipt"),
            bytes(recording, "rawTransaction"), keys(recording));
        verified.requireSuccess();
        assertEquals(BigInteger.ZERO, verified.receipt().amount());
        assertEquals("000000000000000000000000000000000000000000000000000000000000101a",
            X402.hex(verified.receipt().to()));
        X402.PaymentRequirements offer = offer(recording);
        try (PaymentServer server = new PaymentServer(recording, offer)) {
            HttpResponse<byte[]> challenge = server.get(null);
            assertEquals(402, challenge.statusCode());
            X402.PaymentRequired decoded = X402.decodePaymentRequiredHeader(
                challenge.headers().firstValue(X402.PAYMENT_REQUIRED_HEADER).orElseThrow());
            assertEquals("eip155:125", decoded.accepts().get(0).network());
            assertEquals(X402.canonicalJson(required(offer).toNode()), X402.canonicalJson(decoded.toNode()));
            HttpResponse<byte[]> response = server.get(header(evidence(recording), decoded.accepts().get(0)));
            assertEquals(200, response.statusCode());
            assertArrayEquals(RESOURCE, response.body());
            assertEquals(recording.required("receiptJson").required("txHash").textValue(),
                response.headers().firstValue("layerx-transaction").orElseThrow());
            assertEquals(1, server.releases.get());
        }
    }

    @Test
    void finding025VersionedProducerEvidenceSurvivesBothPaymentHeaders() throws Exception {
        JsonNode recording = payment();
        ObjectNode produced = evidence(recording);
        assertEquals(Set.of("format", "verificationLevel", "rawTransaction", "signedReceipt",
            "receiptDigest", "receiptKey"), names(produced));
        assertEquals("layerx-receipt-proof-v2", produced.required("format").textValue());
        X402.PaymentRequirements offer = offer(recording);
        X402.PaymentPayload decoded = X402.decodePaymentPayloadHeader(header(produced, offer));
        assertEquals(produced, X402.parseReceiptEvidence(decoded.payload()).toNode());
        try (PaymentServer server = new PaymentServer(recording, offer)) {
            HttpResponse<byte[]> response = server.get(header(produced, offer));
            assertEquals(200, response.statusCode());
            X402.SettlementResponse settlement = X402.decodeSettlementHeader(
                response.headers().firstValue(X402.PAYMENT_RESPONSE_HEADER).orElseThrow());
            assertEquals(produced, settlement.extensions().get("layerx"));
            assertEquals(produced.required("receiptDigest").textValue().substring(2),
                response.headers().firstValue("layerx-receipt-digest").orElseThrow());
        }
    }

    private static Set<String> names(JsonNode node) {
        Set<String> names = new HashSet<>();
        node.fieldNames().forEachRemaining(names::add);
        return names;
    }

    @Test
    void finding025FilterRefusesMissingUnknownOrMalformedPortableFields() throws Exception {
        JsonNode recording = payment();
        ObjectNode valid = evidence(recording);
        List<ObjectNode> invalid = new ArrayList<>();
        invalid.add(valid.deepCopy().put("format", "layerx-receipt-proof-v1"));
        ObjectNode missing = valid.deepCopy();
        missing.remove("format");
        invalid.add(missing);
        invalid.add(valid.deepCopy().put("format", 2));
        invalid.add(valid.deepCopy().put("unexpected", true));
        invalid.add(valid.deepCopy().put("idempotencyKey", "unversioned-extension"));
        invalid.add(valid.deepCopy().put("receiptDigest", valid.required("receiptDigest").textValue().substring(2)));
        invalid.add(valid.deepCopy().put("verificationLevel", "committed"));
        ObjectNode extraKey = valid.deepCopy();
        ((ObjectNode) extraKey.required("receiptKey")).put("unexpected", true);
        invalid.add(extraKey);
        ObjectNode decimalHeight = valid.deepCopy();
        ((ObjectNode) decimalHeight.required("receiptKey")).put("firstHeight", "1000");
        invalid.add(decimalHeight);
        try (PaymentServer server = new PaymentServer(recording, offer(recording))) {
            for (ObjectNode malformed : invalid) {
                HttpResponse<byte[]> response = server.get(header(malformed, offer(recording)));
                assertEquals(402, response.statusCode(), malformed.toString());
                assertTrue(response.headers().firstValue(X402.PAYMENT_RESPONSE_HEADER).isEmpty());
            }
            assertEquals(0, server.releases.get());
        }
    }

    @Test
    void finding026ExactMeteredAndSubscriptionVerifyNativeEffects() throws Exception {
        JsonNode recording = payment();
        for (String scheme : List.of("exact", "metered", "subscription")) {
            X402.PaymentRequirements offer = offer(recording, scheme, terms(recording));
            try (PaymentServer server = new PaymentServer(recording, offer)) {
                HttpResponse<byte[]> response = server.get(header(evidence(recording), offer));
                assertEquals(200, response.statusCode(), scheme);
                assertArrayEquals(RESOURCE, response.body());
                X402.SettlementResponse settlement = X402.decodeSettlementHeader(
                    response.headers().firstValue(X402.PAYMENT_RESPONSE_HEADER).orElseThrow());
                assertEquals("1000000", settlement.amount());
                assertEquals(1, server.releases.get());
            }
        }
        X402.PaymentRequirements noPurpose = offer(recording, "exact", null);
        try (PaymentServer server = new PaymentServer(recording, noPurpose)) {
            assertEquals(200, server.get(header(evidence(recording), noPurpose)).statusCode());
        }
    }

    @Test
    void finding026FilterRefusesWrongAssetPayeeAmountPayerPurposeSchemeAndCommitment() throws Exception {
        JsonNode recording = payment();
        ObjectNode valid = offer(recording).toNode();
        List<ObjectNode> invalid = new ArrayList<>();
        invalid.add(valid.deepCopy().put("asset", "0x" + "11".repeat(32)));
        invalid.add(valid.deepCopy().put("payTo", "0x" + "22".repeat(32)));
        invalid.add(valid.deepCopy().put("amount", "999999"));
        invalid.add(valid.deepCopy().put("scheme", "unrecognised"));
        for (String field : List.of("payer", "purposeHash")) {
            ObjectNode altered = valid.deepCopy();
            ((ObjectNode) altered.required("extra").required("layerx")).put(field, "33".repeat(32));
            invalid.add(altered);
        }
        for (String scheme : List.of("metered", "subscription")) {
            for (String purpose : Arrays.asList(null, "0".repeat(64), "bad-purpose")) {
                ObjectNode altered = valid.deepCopy().put("scheme", scheme);
                ObjectNode layerx = (ObjectNode) altered.required("extra").required("layerx");
                if (purpose == null) layerx.remove("purposeHash");
                else layerx.put("purposeHash", purpose);
                invalid.add(altered);
            }
        }
        ObjectNode strongerCommitment = valid.deepCopy();
        ((ObjectNode) strongerCommitment.required("extra").required("layerx")).put("commitment", "committed");
        invalid.add(strongerCommitment);
        for (ObjectNode terms : invalid) {
            X402.PaymentRequirements offer = X402.parseRequirements(terms);
            try (PaymentServer server = new PaymentServer(recording, offer)) {
                HttpResponse<byte[]> response = server.get(header(evidence(recording), offer));
                assertEquals(402, response.statusCode(), terms.toString());
                assertEquals("verification-failure", JSON.readTree(response.body()).required("error").textValue());
                assertEquals(0, server.releases.get());
            }
        }
    }

    @Test
    void finding024FilterAndNativeDecoderRefuseWrongChain() throws Exception {
        JsonNode recording = payment();
        X402.PaymentRequirements offer = offer(recording);
        for (String network : List.of("eip155:1", "eip155:126", "eip155:0125", "layerx:mainnet")) {
            MiddlewareException error = assertThrows(MiddlewareException.class,
                () -> X402.parseNetwork(TextNode.valueOf(network)));
            assertEquals(MiddlewareException.Code.UNSUPPORTED_PAYMENT, error.code());
            ObjectNode payload = X402.decodePaymentPayloadHeader(header(evidence(recording), offer)).toNode();
            ((ObjectNode) payload.required("accepted")).put("network", network);
            try (PaymentServer server = new PaymentServer(recording, offer)) {
                assertEquals(402, server.get(X402.encodeBase64(JSON.writeValueAsBytes(payload))).statusCode());
                assertEquals(0, server.releases.get());
            }
        }
        byte[] raw = bytes(recording, "rawTransaction");
        assertEquals(2, raw[0]);
        assertEquals(0xf9, raw[1] & 0xff);
        assertEquals(125, raw[4]);
        byte[] wrongChain = raw.clone();
        wrongChain[4] = 1;
        var verified = LocalVerifier.verifySequencerReceipt(bytes(recording, "signedReceipt"), raw, keys(recording));
        assertDoesNotThrow(() -> NativePaymentVerifier.verify(verified, raw, offer));
        assertEquals(MiddlewareException.Code.VERIFICATION_FAILURE,
            assertThrows(MiddlewareException.class,
                () -> NativePaymentVerifier.verify(verified, wrongChain, offer)).code());
        ObjectNode changed = evidence(recording).put("rawTransaction", hex(wrongChain));
        try (PaymentServer server = new PaymentServer(recording, offer)) {
            assertEquals(402, server.get(header(changed, offer)).statusCode());
            assertEquals(0, server.releases.get());
        }
    }

    @Test
    void finding026RecordedRevertNonpaymentAndTamperedEvidenceNeverRelease() throws Exception {
        JsonNode recording = payment();
        List<ObjectNode> invalid = new ArrayList<>();
        invalid.add(evidence(recording("V1-pax-transfer")));
        invalid.add(evidence(recording("V1-erc20-approve")));
        invalid.add(evidence(recording("V2-layerxledger-revert")));
        invalid.add(evidence(recording).put("receiptDigest", recording.required("receiptDigest").textValue()));
        byte[] corrupt = bytes(recording, "signedReceipt");
        corrupt[corrupt.length - 1] ^= 1;
        invalid.add(evidence(recording).put("signedReceipt", hex(corrupt))
            .put("receiptDigest", hex(keccak(corrupt))));
        ObjectNode unpinned = evidence(recording);
        ((ObjectNode) unpinned.required("receiptKey")).put("publicKey", "0x" + "42".repeat(32));
        invalid.add(unpinned);
        ObjectNode widened = evidence(recording);
        ((ObjectNode) widened.required("receiptKey")).put("firstHeight", "0x0");
        invalid.add(widened);
        try (PaymentServer server = new PaymentServer(recording, offer(recording))) {
            for (ObjectNode proof : invalid) {
                assertEquals(402, server.get(header(proof, offer(recording))).statusCode());
            }
            assertEquals(0, server.releases.get());
        }
    }

    @Test
    void finding026NativeVerifierBindsCalldataToAuthenticatedEffect() throws Exception {
        JsonNode recording = payment();
        byte[] raw = bytes(recording, "rawTransaction");
        var verified = LocalVerifier.verifySequencerReceipt(bytes(recording, "signedReceipt"), raw, keys(recording));
        byte[] selector = java.util.HexFormat.of().parseHex("78ea11db");
        int start = -1;
        for (int index = 0; index <= raw.length - selector.length; index++) {
            if (Arrays.equals(Arrays.copyOfRange(raw, index, index + 4), selector)) {
                start = index;
                break;
            }
        }
        assertTrue(start > 0);
        for (int word : List.of(0, 1, 2, 3, 5)) {
            byte[] altered = raw.clone();
            altered[start + 4 + word * 32 + 31] ^= 1;
            byte[] changedWord = Arrays.copyOfRange(altered, start + 4 + word * 32, start + 36 + word * 32);
            ObjectNode accepted = offer(recording).toNode();
            ObjectNode layerx = (ObjectNode) accepted.required("extra").required("layerx");
            switch (word) {
                case 0 -> layerx.put("payer", X402.hex(changedWord));
                case 1 -> accepted.put("payTo", hex(changedWord));
                case 2 -> accepted.put("asset", hex(changedWord));
                case 3 -> accepted.put("amount", new BigInteger(1, changedWord).toString());
                case 5 -> layerx.put("purposeHash", X402.hex(changedWord));
                default -> throw new AssertionError("uncovered calldata word");
            }
            X402.PaymentRequirements changedOffer = X402.parseRequirements(accepted);
            assertEquals(MiddlewareException.Code.VERIFICATION_FAILURE, assertThrows(MiddlewareException.class,
                () -> NativePaymentVerifier.verify(verified, altered, changedOffer)).code(),
                "calldata and offered terms must also match the authenticated event, word " + word);
            try (PaymentServer server = new PaymentServer(recording, changedOffer)) {
                assertEquals(402, server.get(header(evidence(recording).put("rawTransaction", hex(altered)),
                    changedOffer)).statusCode());
                assertEquals(0, server.releases.get());
            }
        }
    }

    @Test
    void finding027UnmodifiedFilterResponseRoundTripsThroughReceiptConsumer() throws Exception {
        JsonNode recording = payment();
        X402.PaymentRequirements offer = offer(recording);
        try (PaymentServer server = new PaymentServer(recording, offer)) {
            HttpResponse<byte[]> response = server.get(header(evidence(recording), offer));
            assertEquals(200, response.statusCode());
            String unmodified = response.headers().firstValue(X402.PAYMENT_RESPONSE_HEADER).orElseThrow();
            X402.SettlementResponse settlement = X402.decodeSettlementHeader(unmodified);
            assertTrue(settlement.success());
            assertEquals(offer.network(), settlement.network());
            assertEquals(offer.amount(), settlement.amount());
            assertEquals(recording.required("receiptJson").required("txHash").textValue(), settlement.transaction());
            assertEquals(hex(keccak(bytes(recording, "rawTransaction"))), settlement.transaction());
            X402.ReceiptEvidence returned = X402.parseReceiptEvidence(fields(settlement.extensions().get("layerx")));
            assertEquals(evidence(recording), returned.toNode());
            byte[] raw = X402.parseReceiptHex(returned.rawTransaction(), MiddlewareException.Code.VERIFICATION_FAILURE);
            byte[] signed = X402.parseReceiptHex(returned.signedReceipt(), MiddlewareException.Code.VERIFICATION_FAILURE);
            var consumer = SellerMiddleware.verifyPaymentReceipt(signed, raw, keys(recording), offer);
            consumer.requireSuccess();
            assertEquals("0x" + X402.hex(consumer.receipt().activityId()), settlement.transaction());
            assertEquals(hex(keccak(signed)), returned.receiptDigest());
            assertEquals(1, server.releases.get());
        }
    }

    @Test
    void finding027CompletionGuardRejectsSyntheticHashAndAlteredProof() throws Exception {
        JsonNode recording = payment();
        X402.PaymentRequirements offer = offer(recording);
        var decision = (SellerMiddleware.ReleasedDecision) seller(recording, offer).handle("merchant",
            header(evidence(recording), offer), () -> new LayerXResource("text/plain", RESOURCE));
        assertDoesNotThrow(() -> SellerMiddleware.assertReceiptBacked(decision, offer));
        List<ObjectNode> altered = new ArrayList<>();
        ObjectNode settlement = decision.settlement().toNode();
        altered.add(settlement.deepCopy().put("transaction", "lxp:" + X402.hex(decision.verification().receiptDigest())));
        altered.add(settlement.deepCopy().put("transaction", "0x" + "99".repeat(32)));
        altered.add(settlement.deepCopy().put("amount", "1"));
        altered.add(settlement.deepCopy().put("payer", "11".repeat(32)));
        ObjectNode badEvidence = settlement.deepCopy();
        ((ObjectNode) badEvidence.required("extensions").required("layerx"))
            .put("receiptDigest", "0x" + "44".repeat(32));
        altered.add(badEvidence);
        for (ObjectNode value : altered) {
            var parsed = X402.parseSettlement(value);
            var tampered = new SellerMiddleware.ReleasedDecision(parsed, X402.encodeSettlementHeader(parsed),
                decision.verification(), decision.resource());
            assertEquals(MiddlewareException.Code.RECEIPT_NOT_BACKED, assertThrows(MiddlewareException.class,
                () -> SellerMiddleware.assertReceiptBacked(tampered, offer)).code());
        }
    }

    @Test
    void finding026ConfiguredMeteredPurposeReachesRealFilter() throws Exception {
        JsonNode recording = payment();
        LocalVerifier.ReceiptKey pin = key(recording);
        X402.PaymentRequirements recordedOffer = offer(recording);
        StandardEnvironment environment = new StandardEnvironment();
        TestPropertyValues.of(
            "layerx.principal=merchant", "layerx.protected-path=/paid",
            "layerx.resource.url=https://merchant.example/paid", "layerx.payment.scheme=metered",
            "layerx.payment.network=eip155:125", "layerx.payment.price=" + recordedOffer.amount(),
            "layerx.payment.asset=" + recordedOffer.asset(), "layerx.payment.pay-to=" + recordedOffer.payTo(),
            "layerx.payment.timeout-seconds=120", "layerx.payment.purpose-hash="
                + terms(recording).required("layerx").required("purposeHash").textValue(),
            "layerx.receipt-keys.entries[0].key-id=" + hex(pin.keyId()),
            "layerx.receipt-keys.entries[0].public-key=" + hex(pin.publicKey()),
            "layerx.receipt-keys.entries[0].first-height=" + pin.firstHeight(),
            "layerx.receipt-keys.entries[0].last-height=" + pin.lastHeight(),
            "layerx.webhook.path=/webhooks", "layerx.webhook.public-keys.receipts="
                + Base64.getEncoder().encodeToString(pin.publicKey())).applyTo(environment);
        LayerXProperties properties = Binder.get(environment).bind("layerx", LayerXProperties.class).get();
        LayerXDeclaredConfig config = properties.toDeclaredConfig();
        assertEquals(terms(recording).required("layerx").required("purposeHash"),
            config.requirements().extra().required("layerx").required("purposeHash"));
        try (PaymentServer server = new PaymentServer(recording, config.requirements())) {
            assertEquals(200, server.get(header(evidence(recording), config.requirements())).statusCode());
            assertEquals(1, server.releases.get());
        }
        properties.getPayment().setPurposeHash("0".repeat(64));
        assertThrows(MiddlewareException.class, properties::toDeclaredConfig);
    }

    // A loopback HTTP transport adapter: the production filter, authority, decoder,
    // verifier, fulfillment store and resource handler execute without substitutes.
    private static final class PaymentServer implements AutoCloseable {
        final AtomicInteger releases = new AtomicInteger();
        private final AtomicReference<Throwable> failure = new AtomicReference<>();
        private final ExecutorService executor = Executors.newVirtualThreadPerTaskExecutor();
        private final HttpServer server;
        private final HttpClient client = HttpClient.newHttpClient();
        private final URI uri;

        PaymentServer(JsonNode recording, X402.PaymentRequirements offer) throws Exception {
            LayerXDeclaredConfig config = new LayerXDeclaredConfig("merchant", "/paid", required(offer), offer,
                keys(recording), "/webhooks", Map.of(), 1000, 1000);
            LayerXPaymentGateFilter filter = new LayerXPaymentGateFilter(config, seller(recording, offer), request -> {
                releases.incrementAndGet();
                return new LayerXResource("text/plain", RESOURCE);
            });
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
