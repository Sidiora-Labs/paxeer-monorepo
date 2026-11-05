package com.sidiora.layerx.android;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sidiora.layerx.sdk.ProtocolAmount;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.security.MessageDigest;
import java.util.ArrayList;
import java.util.Base64;
import java.util.concurrent.CompletionException;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import static org.junit.jupiter.api.Assertions.*;

public final class ReceiptGateTest {
    @TempDir Path directory;

    @Test
    void verifiesSignedSequencerReceipt() throws Exception {
        JsonNode vector = transferVector();
        LocalVerifier.ReceiptKey pin = key(vector);
        ReceiptGate.Evidence evidence = evidence(vector, pin);
        PublishableConfiguration configuration = PublishableConfiguration.ofEnvironment(Map.of(
            "LAYERX_SERVICE_URL", "https://service.example",
            "LAYERX_SESSION_BROKER_URL", "https://broker.example",
            "LAYERX_EVENT_KEY_ID", "events",
            "LAYERX_EVENT_PUBLIC_KEY", HexFormat.of().formatHex(pin.publicKey()),
            "LAYERX_RECEIPT_KEYS_JSON", "[" + vector.required("pinnedKey") + "]"));
        LocalVerifier.ReceiptKey configured = configuration.receiptKeys().orElseThrow().entries().getFirst();
        assertArrayEquals(pin.keyId(), configured.keyId());
        assertArrayEquals(pin.publicKey(), configured.publicKey());
        assertEquals(pin.firstHeight(), configured.firstHeight());
        assertEquals(pin.lastHeight(), configured.lastHeight());
        assertTrue(PublishableConfiguration.declaredKeyNames().contains(PublishableConfiguration.RECEIPT_KEYS_KEY));
        assertEquals(PublishableConfiguration.RECEIPT_KEYS_KEY,
            PublishableConfiguration.declaredKeyForEnvironmentVariable("LAYERX_RECEIPT_KEYS_JSON"));
        for (byte[] publicValue : List.of(pin.keyId(), pin.publicKey())) {
            assertTrue(configuration.exemptScannerValues().contains(HexFormat.of().formatHex(publicValue)));
            assertTrue(configuration.exemptScannerValues().contains("0x" + HexFormat.of().formatHex(publicValue)));
        }
        ReceiptGate gate;
        ReceiptGate.Verified verified;
        try (HumanPeer human = new HumanPeer(humanFixture());
                LayerXAndroid android = LayerXAndroid.create(human.configuration(vector),
                    directory.resolve("events.json"))) {
            gate = android.gate();
            verified = assertInstanceOf(ReceiptGate.Verified.class,
                gate.track(human.journeyId(), expectation()).toCompletableFuture().join());
            assertEquals(List.of("POST /session", "GET /v1/journeys/" + human.journeyId(),
                "GET /v1/evidence/" + human.evidenceId()), human.requests());
        }
        assertEquals("receipted", verified.level());
        assertEquals(HexFormat.of().formatHex(bytes(vector, "receiptDigest")), verified.receiptDigest());
        byte[] exposedTransaction = evidence.rawTransaction();
        exposedTransaction[0] ^= 1;
        byte[] exposedReceipt = evidence.signedReceipt();
        exposedReceipt[0] ^= 1;
        assertArrayEquals(bytes(vector, "rawTransaction"), evidence.rawTransaction());
        assertArrayEquals(bytes(vector, "signedReceipt"), evidence.signedReceipt());
        assertInstanceOf(ReceiptGate.Verified.class, gate.verify(evidence, expectation()));

        byte[] tamperedReceipt = evidence.signedReceipt();
        tamperedReceipt[tamperedReceipt.length - 1] ^= 1;
        assertRefused(gate, new ReceiptGate.Evidence(evidence.rawTransaction(), tamperedReceipt, pin));
        byte[] otherTransaction = evidence.rawTransaction();
        otherTransaction[otherTransaction.length - 1] ^= 1;
        assertRefused(gate, new ReceiptGate.Evidence(otherTransaction, evidence.signedReceipt(), pin));

        ReceiptGate.Expectation expected = expectation();
        byte[] otherAsset = expected.asset().clone();
        otherAsset[0] ^= 1;
        byte[] otherRecipient = expected.recipient().clone();
        otherRecipient[0] ^= 1;
        for (ReceiptGate.Expectation mismatch : List.of(
                new ReceiptGate.Expectation(otherAsset, expected.recipient(), expected.amount()),
                new ReceiptGate.Expectation(expected.asset(), otherRecipient, expected.amount()),
                new ReceiptGate.Expectation(expected.asset(), expected.recipient(),
                    new ProtocolAmount(expected.amount().value().add(BigInteger.ONE))))) {
            MobileIntegrationException error = assertThrows(MobileIntegrationException.class,
                () -> gate.verify(evidence, mismatch));
            assertEquals(MobileIntegrationException.Code.VERIFICATION_FAILURE, error.code());
        }
    }

    @Test
    void refusesReceiptUnderUnpinnedKey() throws Exception {
        JsonNode vector = transferVector();
        LocalVerifier.ReceiptKey pin = key(vector);
        ReceiptGate.Evidence evidence = evidence(vector, pin);
        BigInteger height = new BigInteger(vector.required("receiptJson").required("height").asText().substring(2), 16);
        LocalVerifier.ReceiptKey boundary = new LocalVerifier.ReceiptKey(
            pin.keyId(), pin.publicKey(), height, height);
        ReceiptGate.Evidence atBoundary = evidence(vector, boundary);
        assertInstanceOf(ReceiptGate.Verified.class,
            gate(atBoundary, boundary).verify(atBoundary, expectation()));

        byte[] otherId = pin.keyId();
        otherId[0] ^= 1;
        byte[] otherPublicKey = pin.publicKey();
        otherPublicKey[0] ^= 1;
        for (LocalVerifier.ReceiptKey unpinned : List.of(
                new LocalVerifier.ReceiptKey(otherId, pin.publicKey(), pin.firstHeight(), pin.lastHeight()),
                new LocalVerifier.ReceiptKey(pin.keyId(), otherPublicKey, pin.firstHeight(), pin.lastHeight()),
                new LocalVerifier.ReceiptKey(pin.keyId(), pin.publicKey(), height.add(BigInteger.ONE), pin.lastHeight()),
                new LocalVerifier.ReceiptKey(pin.keyId(), pin.publicKey(), BigInteger.ZERO, height.subtract(BigInteger.ONE)))) {
            ReceiptGate.Evidence claimed = evidence(vector, unpinned);
            assertRefused(gate(claimed, unpinned), claimed);
            assertRefused(gate(evidence, pin), claimed);
        }

        Map<String, String> declared = declaredKeys(pin);
        PublishableConfiguration withoutReceipts = PublishableConfiguration.of(declared);
        assertTrue(withoutReceipts.receiptKeys().isEmpty());
        try (LayerXAndroid android = LayerXAndroid.create(withoutReceipts, directory.resolve("events.json"))) {
            MobileIntegrationException error = assertThrows(MobileIntegrationException.class,
                () -> android.gate(reference -> evidence));
            assertEquals(MobileIntegrationException.Code.INVALID_CONFIGURATION, error.code());
        }
        JsonNode keyJson = vector.required("pinnedKey");
        for (String invalid : List.of("[]", "{}", "null", "not-json", "[" + keyJson + "] true",
                "[" + keyJson + "," + keyJson + "]",
                "[" + keyJson.deepCopy().withObject("").put("keyId", "0x00") + "]",
                "[" + keyJson.deepCopy().withObject("").put("firstHeight", -1) + "]",
                "[" + keyJson.deepCopy().withObject("").put("lastHeight", 0) + "]",
                "[" + keyJson.deepCopy().withObject("").put("firstHeight", 1000.5) + "]",
                "[" + keyJson.deepCopy().withObject("").put("lastHeight", "1999") + "]",
                "[" + keyJson.deepCopy().withObject("").put("privateKey", "forbidden") + "]")) {
            declared.put(PublishableConfiguration.RECEIPT_KEYS_KEY, invalid);
            MobileIntegrationException error = assertThrows(MobileIntegrationException.class,
                () -> PublishableConfiguration.of(declared));
            assertEquals(MobileIntegrationException.Code.INVALID_CONFIGURATION, error.code());
        }
        declared.put(PublishableConfiguration.RECEIPT_KEYS_KEY, "[" + keyJson + "]");
        assertArrayEquals(pin.keyId(), PublishableConfiguration.of(declared)
            .receiptKeys().orElseThrow().entries().getFirst().keyId());
    }

    @Test
    void completedHumanJourneyRequiresMatchingPinnedEvidence() throws Exception {
        JsonNode vector = transferVector();
        JsonNode fixture = humanFixture();
        String digest = HexFormat.of().formatHex(bytes(vector, "receiptDigest"));
        for (String state : List.of("done", "done-finalised")) {
            ObjectNode response = fixture.deepCopy();
            ObjectNode journey = (ObjectNode) response.required("journey").required("result");
            journey.put("state", state);
            journey.put("state_copy_key", "journey.state." + state);
            if (state.equals("done-finalised")) {
                ((ObjectNode) journey.required("evidence").get(0)).put("verification", "checkpoint-finalised");
                ((ObjectNode) journey.required("stages").get(0).required("evidence").get(0))
                    .put("verification", "checkpoint-finalised");
                ((ObjectNode) response.required("evidence").required("result"))
                    .put("verification", "checkpoint-finalised").put("rung", "committed");
            }
            try (HumanPeer human = new HumanPeer(response);
                    LayerXAndroid android = LayerXAndroid.create(human.configuration(vector),
                        directory.resolve(state + ".json"))) {
                ReceiptGate.Verified verified = assertInstanceOf(ReceiptGate.Verified.class,
                    android.gate().track(human.journeyId(), expectation()).toCompletableFuture().join());
                assertEquals("receipted", verified.level());
                assertEquals(digest, verified.receiptDigest());
                assertEquals(3, human.requests().size());
            }
        }
        for (String mutation : List.of("signature", "transaction", "digest", "evidence-id", "public-key")) {
            ObjectNode response = fixture.deepCopy();
            ObjectNode material = (ObjectNode) response.required("evidence").required("result");
            ObjectMapper mapper = new ObjectMapper();
            ObjectNode proof = (ObjectNode) mapper.readTree(
                Base64.getDecoder().decode(material.required("bytes_base64").textValue()));
            switch (mutation) {
                case "signature" -> proof.put("signedReceipt", flipped(proof.required("signedReceipt").textValue()));
                case "transaction" -> proof.put("rawTransaction", flipped(proof.required("rawTransaction").textValue()));
                case "digest" -> proof.put("receiptDigest", flipped(proof.required("receiptDigest").textValue()));
                case "evidence-id" -> material.put("evidence_id", "evd_" + "0".repeat(64));
                case "public-key" -> ((ObjectNode) proof.required("receiptKey")).put("publicKey",
                    flipped(proof.required("receiptKey").required("publicKey").textValue()));
                default -> throw new AssertionError(mutation);
            }
            if (mutation.equals("signature")) {
                String id = "evd_" + HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(
                    bytes(proof, "signedReceipt")));
                material.put("evidence_id", id);
                ObjectNode journey = (ObjectNode) response.required("journey").required("result");
                ((ObjectNode) journey.required("evidence").get(0)).put("evidence_id", id);
                ((ObjectNode) journey.required("stages").get(0).required("evidence").get(0)).put("evidence_id", id);
            }
            material.put("bytes_base64", Base64.getEncoder().encodeToString(mapper.writeValueAsBytes(proof)));
            assertHumanRefused(response, vector, expectation());
        }
        ObjectNode otherPin = vector.deepCopy();
        ((ObjectNode) otherPin.required("pinnedKey")).put("publicKey",
            flipped(vector.required("pinnedKey").required("publicKey").textValue()));
        assertHumanRefused(fixture, otherPin, expectation());
        ReceiptGate.Expectation expected = expectation();
        byte[] otherAsset = expected.asset().clone();
        otherAsset[0] ^= 1;
        byte[] otherRecipient = expected.recipient().clone();
        otherRecipient[0] ^= 1;
        for (ReceiptGate.Expectation mismatch : List.of(
                new ReceiptGate.Expectation(otherAsset, expected.recipient(), expected.amount()),
                new ReceiptGate.Expectation(expected.asset(), otherRecipient, expected.amount()),
                new ReceiptGate.Expectation(expected.asset(), expected.recipient(),
                    new ProtocolAmount(expected.amount().value().add(BigInteger.ONE))))) {
            assertHumanRefused(fixture, vector, mismatch);
        }
    }

    private void assertHumanRefused(JsonNode response, JsonNode vector,
                                    ReceiptGate.Expectation expectation) throws Exception {
        try (HumanPeer human = new HumanPeer(response);
                LayerXAndroid android = LayerXAndroid.create(human.configuration(vector),
                    directory.resolve("refused.json"))) {
            CompletionException error = assertThrows(CompletionException.class,
                () -> android.gate().track(human.journeyId(), expectation).toCompletableFuture().join());
            MobileIntegrationException cause = assertInstanceOf(MobileIntegrationException.class, error.getCause());
            assertEquals(MobileIntegrationException.Code.VERIFICATION_FAILURE, cause.code());
            assertEquals(3, human.requests().size());
        }
    }

    private static String flipped(String encoded) {
        byte[] bytes = HexFormat.of().parseHex(encoded.substring(2));
        bytes[bytes.length - 1] ^= 1;
        return "0x" + HexFormat.of().formatHex(bytes);
    }

    private static JsonNode humanFixture() throws Exception {
        Path root = Path.of("").toAbsolutePath();
        while (root != null) {
            Path fixture = root.resolve("sdk/integrations/android/src/test/java/com/sidiora/layerx/android/"
                + "human-receipt-journey-22-33-622.json");
            if (Files.isRegularFile(fixture)) return new ObjectMapper().readTree(Files.readString(fixture));
            root = root.getParent();
        }
        throw new IllegalStateException("Human receipt journey fixture not found");
    }

    private static final class HumanPeer implements AutoCloseable {
        private final HttpServer server;
        private final JsonNode fixture;
        private final List<String> requests = new ArrayList<>();
        private final List<String> violations = new ArrayList<>();
        private final ObjectMapper mapper = new ObjectMapper();

        HumanPeer(JsonNode fixture) throws IOException {
            this.fixture = fixture;
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", this::respond);
            server.start();
        }

        String journeyId() { return fixture.required("journey").required("result").required("journey_id").textValue(); }
        String evidenceId() { return fixture.required("journey").required("result").required("evidence").get(0)
            .required("evidence_id").textValue(); }
        synchronized List<String> requests() { return List.copyOf(requests); }

        PublishableConfiguration configuration(JsonNode vector) {
            String origin = "http://127.0.0.1:" + server.getAddress().getPort();
            return PublishableConfiguration.ofEnvironment(Map.of(
                "LAYERX_SERVICE_URL", origin,
                "LAYERX_SESSION_BROKER_URL", origin + "/session",
                "LAYERX_EVENT_KEY_ID", "events",
                "LAYERX_EVENT_PUBLIC_KEY", vector.required("pinnedKey").required("publicKey").textValue().substring(2),
                "LAYERX_RECEIPT_KEYS_JSON", "[" + vector.required("pinnedKey") + "]"));
        }

        private synchronized void respond(HttpExchange exchange) throws IOException {
            try (exchange) {
                String request = exchange.getRequestMethod() + " " + exchange.getRequestURI().toString();
                requests.add(request);
                JsonNode response;
                if (request.equals("POST /session")) {
                    JsonNode body = mapper.readTree(exchange.getRequestBody());
                    if (!mapper.createObjectNode().put("audience", "layerx-human-api").equals(body)) {
                        violations.add("wrong broker audience");
                    }
                    long now = System.currentTimeMillis();
                    response = mapper.createObjectNode().put("session_token", "android-receipt-test-session")
                        .put("issued_at_ms", now).put("expires_at_ms", now + 300_000);
                } else {
                    if (!"Bearer android-receipt-test-session".equals(
                            exchange.getRequestHeaders().getFirst("Authorization"))) {
                        violations.add("missing brokered authorization");
                    }
                    if (request.equals("GET /v1/journeys/" + journeyId())) {
                        response = fixture.required("journey");
                    } else if (request.equals("GET /v1/evidence/" + evidenceId())) {
                        response = fixture.required("evidence");
                    } else {
                        violations.add("unexpected request: " + request);
                        exchange.sendResponseHeaders(404, -1);
                        return;
                    }
                }
                byte[] body = mapper.writeValueAsBytes(response);
                exchange.getResponseHeaders().set("Content-Type", "application/json");
                exchange.sendResponseHeaders(200, body.length);
                exchange.getResponseBody().write(body);
            }
        }

        @Override public void close() {
            server.stop(0);
            assertEquals(List.of(), violations);
        }
    }

    private static Map<String, String> declaredKeys(LocalVerifier.ReceiptKey pin) {
        return new LinkedHashMap<>(Map.of(
            PublishableConfiguration.SERVICE_URL_KEY, "https://service.example",
            PublishableConfiguration.SESSION_BROKER_URL_KEY, "https://broker.example",
            PublishableConfiguration.EVENT_PUBLIC_KEY_PREFIX + "events", HexFormat.of().formatHex(pin.publicKey())));
    }

    private static ReceiptGate gate(ReceiptGate.Evidence evidence, LocalVerifier.ReceiptKey pin) {
        return new ReceiptGate(reference -> {
            assertEquals("signed-transfer", reference);
            return evidence;
        }, new LocalVerifier.ReceiptKeys(List.of(pin)));
    }

    private static void assertRefused(ReceiptGate gate, ReceiptGate.Evidence evidence) {
        MobileIntegrationException error = assertThrows(MobileIntegrationException.class,
            () -> gate.verify(evidence, expectation()));
        assertEquals(MobileIntegrationException.Code.VERIFICATION_FAILURE, error.code());
    }

    private static ReceiptGate.Expectation expectation() {
        return new ReceiptGate.Expectation(
            HexFormat.of().parseHex("0e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f73"),
            HexFormat.of().parseHex("0000000000000000000000008ba1f109551bd432803012645ac136ddd64dba72"),
            new ProtocolAmount(new BigInteger("1000000000000000000")));
    }

    private static ReceiptGate.Evidence evidence(JsonNode vector, LocalVerifier.ReceiptKey pin) {
        return new ReceiptGate.Evidence(bytes(vector, "rawTransaction"), bytes(vector, "signedReceipt"), pin);
    }

    private static LocalVerifier.ReceiptKey key(JsonNode vector) {
        JsonNode pin = vector.required("pinnedKey");
        return new LocalVerifier.ReceiptKey(bytes(pin, "keyId"), bytes(pin, "publicKey"),
            pin.required("firstHeight").bigIntegerValue(), pin.required("lastHeight").bigIntegerValue());
    }

    private static byte[] bytes(JsonNode node, String field) {
        String value = node.required(field).textValue();
        assertNotNull(value, field);
        assertTrue(value.startsWith("0x"), field);
        return HexFormat.of().parseHex(value.substring(2));
    }

    private static JsonNode transferVector() throws Exception {
        Path directory = Path.of("").toAbsolutePath();
        while (directory != null) {
            Path fixture = directory.resolve("sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json");
            if (Files.isRegularFile(fixture)) {
                for (JsonNode vector : new ObjectMapper().readTree(Files.readString(fixture)).required("cases")) {
                    if ("V1-pax-transfer".equals(vector.required("id").textValue())) return vector;
                }
                throw new IllegalStateException("V1-pax-transfer not found");
            }
            directory = directory.getParent();
        }
        throw new IllegalStateException("sequencer receipt conformance fixture not found");
    }
}
