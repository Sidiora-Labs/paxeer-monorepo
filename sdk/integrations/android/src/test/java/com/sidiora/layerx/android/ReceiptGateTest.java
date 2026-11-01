package com.sidiora.layerx.android;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sidiora.layerx.sdk.ProtocolAmount;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
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
        try (LayerXAndroid android = LayerXAndroid.create(configuration, directory.resolve("events.json"))) {
            gate = android.gate(reference -> {
                assertEquals("signed-transfer", reference);
                return evidence;
            });
            verified = assertInstanceOf(ReceiptGate.Verified.class,
                gate.project(new ObjectMapper().readTree(
                    "{\"state\":\"settled\",\"receipt_ref\":\"signed-transfer\"}"), expectation()));
        }
        assertEquals("receipted", verified.level());
        assertEquals(HexFormat.of().formatHex(bytes(vector, "receiptDigest")), verified.receiptDigest());

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
