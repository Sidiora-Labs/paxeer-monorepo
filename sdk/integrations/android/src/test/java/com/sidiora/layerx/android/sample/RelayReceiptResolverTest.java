package com.sidiora.layerx.android.sample;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.android.ReceiptGate;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sun.net.httpserver.HttpServer;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HexFormat;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

public final class RelayReceiptResolverTest {
    @Test
    void parsesSignedReceiptPayload() throws Exception {
        ObjectMapper mapper = new ObjectMapper();
        JsonNode vector = transferVector(mapper);
        JsonNode key = vector.required("pinnedKey");
        ObjectNode payload = mapper.createObjectNode()
            .put("raw_transaction", vector.required("rawTransaction").textValue())
            .put("signed_receipt", vector.required("signedReceipt").textValue());
        payload.set("receipt_key", key);
        assertFalse(payload.has("authorized_batch"));
        byte[] response = mapper.writeValueAsBytes(payload);
        AtomicReference<String> requestedPath = new AtomicReference<>();
        AtomicReference<String> requestedMethod = new AtomicReference<>();
        HttpServer relay = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        relay.createContext("/receipts/", exchange -> {
            try (exchange) {
                requestedPath.set(exchange.getRequestURI().getRawPath());
                requestedMethod.set(exchange.getRequestMethod());
                exchange.getResponseHeaders().set("Content-Type", "application/json");
                exchange.sendResponseHeaders(200, response.length);
                exchange.getResponseBody().write(response);
            }
        });
        relay.start();
        try {
            RelayReceiptResolver resolver = new RelayReceiptResolver(
                URI.create("http://127.0.0.1:" + relay.getAddress().getPort() + "/receipts/"), mapper, 5_000);
            ReceiptGate.Evidence evidence = resolver.resolve("signed-transfer");
            assertEquals("GET", requestedMethod.get());
            assertEquals("/receipts/signed-transfer", requestedPath.get());
            assertArrayEquals(bytes(vector, "rawTransaction"), evidence.rawTransaction());
            assertArrayEquals(bytes(vector, "signedReceipt"), evidence.signedReceipt());
            LocalVerifier.ReceiptKey receiptKey = evidence.receiptKey();
            assertArrayEquals(bytes(key, "keyId"), receiptKey.keyId());
            assertArrayEquals(bytes(key, "publicKey"), receiptKey.publicKey());
            assertEquals(key.required("firstHeight").bigIntegerValue(), receiptKey.firstHeight());
            assertEquals(key.required("lastHeight").bigIntegerValue(), receiptKey.lastHeight());
        } finally {
            relay.stop(0);
        }
    }

    private static byte[] bytes(JsonNode node, String field) {
        String value = node.required(field).textValue();
        assertNotNull(value, field);
        assertTrue(value.startsWith("0x"), field);
        return HexFormat.of().parseHex(value.substring(2));
    }

    private static JsonNode transferVector(ObjectMapper mapper) throws Exception {
        Path directory = Path.of("").toAbsolutePath();
        while (directory != null) {
            Path fixture = directory.resolve("sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json");
            if (Files.isRegularFile(fixture)) {
                for (JsonNode vector : mapper.readTree(Files.readString(fixture)).required("cases")) {
                    if ("V1-pax-transfer".equals(vector.required("id").textValue())) return vector;
                }
                throw new IllegalStateException("V1-pax-transfer not found");
            }
            directory = directory.getParent();
        }
        throw new IllegalStateException("sequencer receipt conformance fixture not found");
    }
}
