package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sun.net.httpserver.HttpServer;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HexFormat;
import java.util.List;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

public final class Repair22331391Test {
    private static final ObjectMapper JSON = new ObjectMapper();

    @Test
    void programMetadataMatchesCanonicalRetainedTransactionContract() {
        var catalogue = new GeneratedCatalogueProgramsTest();
        assertAll("program-metadata-test",
            catalogue::programOperationsNameLayerxprogramsCalls,
            catalogue::nativePrepareReturnsEip1559Transaction);
    }

    @Test
    void programReceiptTracesTheRealRetainedTransactionRequestSequence() throws Exception {
        new GeneratedCatalogueProgramsTest().programReceiptTracesRetainedTransactionThroughHttp();
    }

    @Test
    void portableCommittedResponsePreservesProducerFieldsAcrossRestart() throws Exception {
        new PortableReceiptTest().portableCommittedNodeResponseSurvivesTransitionAndRestart();
    }

    @Test
    void portableNormalizationRemainsAnExplicitIndependentVectorTest() throws Exception {
        new PortableReceiptTest().portableExportMatchesPublishedVectorV5();
    }

    @Test
    void feeTokenSignedVectorBindsPaymentAssetAndIndependentFeeSummary() throws Exception {
        new SequencerReceiptTest().sequencerFeeTokenVectorBindsIndependentPaymentAndFeeAssets();
    }

    @Test
    void feeTokenSignedReceiptRejectsIndependentDenominationMutations() throws Exception {
        new SequencerReceiptTest().sequencerFeeTokenSignedReceiptRejectsIndependentDenominationMutations();
    }

    @Test
    void recordedNodeHttpDecoderRejectsIndependentDenominationMutations() throws Exception {
        new SequencerReceiptTest().sequencerRecordedNodeDenominationMutationsAreRefusedIndependently();
    }

    static JsonNode fixture(String name) throws Exception {
        Path directory = Path.of("").toAbsolutePath();
        while (directory != null) {
            Path path = directory.resolve(name);
            if (Files.isRegularFile(path)) return JSON.readTree(Files.readAllBytes(path));
            directory = directory.getParent();
        }
        throw new IllegalStateException("Missing recorded fixture: " + name);
    }

    static byte[] bytes(JsonNode value, String field) {
        String hex = value.required(field).textValue();
        assertNotNull(hex, field);
        assertTrue(hex.startsWith("0x"), field);
        return HexFormat.of().parseHex(hex.substring(2));
    }

    static LocalVerifier.ReceiptKey key(JsonNode vector) {
        JsonNode pin = vector.required("pinnedKey");
        return new LocalVerifier.ReceiptKey(bytes(pin, "keyId"), bytes(pin, "publicKey"),
            pin.required("firstHeight").bigIntegerValue(), pin.required("lastHeight").bigIntegerValue());
    }

    static ProgramsClient client(ProgramsClient.Programs connection, JsonNode vector) {
        var pin = key(vector);
        return new ProgramsClient(connection, vector.required("receiptJson").required("from").textValue(),
            BigInteger.ZERO, BigInteger.ONE, new LocalVerifier.ReceiptKeys(List.of(pin)), pin.keyId());
    }

    static final class RecordedReceiptArchive implements AutoCloseable {
        private final HttpServer server;
        final AtomicReference<JsonNode> receipt;
        final List<JsonNode> requests = new CopyOnWriteArrayList<>();
        final List<JsonNode> delivered = new CopyOnWriteArrayList<>();

        RecordedReceiptArchive(JsonNode recorded) throws Exception {
            receipt = new AtomicReference<>(recorded);
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", exchange -> {
                try (exchange) {
                    JsonNode request = JSON.readTree(exchange.getRequestBody());
                    requests.add(request);
                    JsonNode current = receipt.get();
                    if (!"POST".equals(exchange.getRequestMethod())
                            || !"2.0".equals(request.path("jsonrpc").textValue())
                            || !"pax_sequencerReceipt".equals(request.path("method").textValue())
                            || !request.path("params").isArray() || request.path("params").size() != 1
                            || !current.required("txHash").equals(request.path("params").path(0))) {
                        exchange.sendResponseHeaders(400, -1);
                        return;
                    }
                    ObjectNode response = JSON.createObjectNode().put("jsonrpc", "2.0");
                    response.set("id", request.required("id"));
                    response.set("result", current);
                    byte[] body = JSON.writeValueAsBytes(response);
                    delivered.add(current.deepCopy());
                    exchange.getResponseHeaders().set("Content-Type", "application/json");
                    exchange.sendResponseHeaders(200, body.length);
                    exchange.getResponseBody().write(body);
                }
            });
            server.start();
        }

        URI uri() { return URI.create("http://127.0.0.1:" + server.getAddress().getPort()); }

        void assertLookups(int count, byte[] transactionHash) {
            assertEquals(count, requests.size());
            assertEquals(count, delivered.size());
            for (JsonNode request : requests) {
                assertEquals("pax_sequencerReceipt", request.required("method").textValue());
                assertEquals(JSON.createArrayNode().add("0x" + HexFormat.of().formatHex(transactionHash)),
                    request.required("params"));
            }
        }

        @Override public void close() { server.stop(0); }
    }
}
