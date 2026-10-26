package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.GeneratedReceiptContract.ReceiptCheck;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

public final class SequencerReceiptTest {
    private static final ObjectMapper JSON = new ObjectMapper();

    private static JsonNode fixture() throws Exception {
        Path directory = Path.of("").toAbsolutePath();
        while (directory != null) {
            Path fixture = directory.resolve("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");
            if (Files.isRegularFile(fixture)) return JSON.readTree(Files.readString(fixture));
            directory = directory.getParent();
        }
        throw new IllegalStateException("sequencer-receipt-v1.json not found");
    }

    private static byte[] bytes(JsonNode node, String field) {
        String text = node.required(field).textValue();
        assertNotNull(text, field);
        assertTrue(text.startsWith("0x"), field);
        return HexFormat.of().parseHex(text.substring(2));
    }

    private static LocalVerifier.ReceiptKey key(JsonNode vector) {
        JsonNode pin = vector.required("pinnedKey");
        return new LocalVerifier.ReceiptKey(bytes(pin, "keyId"), bytes(pin, "publicKey"),
            pin.required("firstHeight").bigIntegerValue(), pin.required("lastHeight").bigIntegerValue());
    }

    private static LocalVerifier.SequencerReceiptVerification verify(JsonNode vector) {
        return LocalVerifier.verifySequencerReceipt(bytes(vector, "signedReceipt"),
            bytes(vector, "rawTransaction"), new LocalVerifier.ReceiptKeys(List.of(key(vector))),
            vector.required("receiptJson"), bytes(vector, "prevSeqHash"));
    }

    private static void assertRefusal(ReceiptCheck check, Runnable operation) {
        PlatformSdkException error = assertThrows(PlatformSdkException.class, operation::run);
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, error.code());
        assertEquals(check, error.receiptCheck());
        assertEquals(check.wire(), error.safeDetails().get("receiptCheck"));
    }

    @Test
    void sequencerReceiptV1CasesVerifyReceipted() throws Exception {
        int count = 0;
        for (JsonNode vector : fixture().required("cases")) {
            if (!vector.required("vector").asText().equals("V1")) continue;
            count++;
            String id = vector.required("id").asText();
            var verified = verify(vector);
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, verified.level(), id);
            assertEquals(vector.required("expected").required("rung").asText(), verified.level().wire(), id);
            assertEquals(1, verified.status(), id);
            assertEquals(0, verified.receipt().resultCode(), id);
            assertNull(verified.revert(), id);
            assertDoesNotThrow(verified::requireSuccess, id);
            assertArrayEquals(bytes(vector, "receiptDigest"), verified.receiptDigest(), id);
            assertArrayEquals(bytes(vector, "signedReceipt"), verified.canonicalBytes(), id);
            assertArrayEquals(key(vector).keyId(), verified.receipt().contextHash(), id);
            assertArrayEquals(bytes(vector.required("receiptJson"), "txHash"), verified.receipt().activityId(), id);
            var keys = new LocalVerifier.ReceiptKeys(List.of(key(vector)));
            var core = LocalVerifier.verifySequencerReceipt(bytes(vector, "signedReceipt"),
                bytes(vector, "rawTransaction"), keys);
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, core.level(), id);
            assertArrayEquals(verified.receiptDigest(), core.receiptDigest(), id);
            BigInteger height = new BigInteger(1, verified.receipt().batchId());
            var boundary = new LocalVerifier.ReceiptKey(key(vector).keyId(), key(vector).publicKey(), height, height);
            assertEquals(1, LocalVerifier.verifySequencerReceipt(bytes(vector, "signedReceipt"),
                bytes(vector, "rawTransaction"), new LocalVerifier.ReceiptKeys(List.of(boundary))).status(), id);

            byte[] signatureChanged = bytes(vector, "signedReceipt");
            signatureChanged[signatureChanged.length - 1] ^= 1;
            assertRefusal(ReceiptCheck.SIGNATURE, () -> LocalVerifier.verifySequencerReceipt(
                signatureChanged, bytes(vector, "rawTransaction"), keys));
            byte[] wrongTransaction = bytes(vector, "rawTransaction");
            wrongTransaction[wrongTransaction.length - 1] ^= 1;
            assertRefusal(ReceiptCheck.RAW_TRANSACTION, () -> LocalVerifier.verifySequencerReceipt(
                bytes(vector, "signedReceipt"), wrongTransaction, keys));
            byte[] version = bytes(vector, "signedReceipt");
            version[1] = 1; version[5] = 1;
            assertRefusal(ReceiptCheck.RECEIPT_VERSION, () -> LocalVerifier.verifySequencerReceipt(
                version, bytes(vector, "rawTransaction"), keys));
            byte[] zeroActivity = bytes(vector, "signedReceipt");
            Arrays.fill(zeroActivity, 10, 42, (byte) 0);
            assertRefusal(ReceiptCheck.ACTIVITY_ID, () -> LocalVerifier.verifySequencerReceipt(
                zeroActivity, bytes(vector, "rawTransaction"), keys));
            assertRefusal(ReceiptCheck.DECODE, () -> LocalVerifier.verifySequencerReceipt(
                new byte[0], bytes(vector, "rawTransaction"), keys));
            ObjectNode returnChanged = vector.required("receiptJson").deepCopy();
            returnChanged.put("returnData", "0x01");
            assertRefusal(ReceiptCheck.RETURN_DATA, () -> LocalVerifier.verifySequencerReceipt(
                bytes(vector, "signedReceipt"), bytes(vector, "rawTransaction"), keys,
                returnChanged, bytes(vector, "prevSeqHash")));
            ObjectNode fieldsChanged = vector.required("receiptJson").deepCopy();
            fieldsChanged.put("status", "0x0");
            assertRefusal(ReceiptCheck.FIELDS, () -> LocalVerifier.verifySequencerReceipt(
                bytes(vector, "signedReceipt"), bytes(vector, "rawTransaction"), keys,
                fieldsChanged, bytes(vector, "prevSeqHash")));
        }
        assertEquals(4, count, "every V1 vector must execute");
    }

    @Test
    void sequencerReceiptV2RevertDecodesReason() throws Exception {
        int count = 0;
        for (JsonNode vector : fixture().required("cases")) {
            if (!vector.required("vector").asText().equals("V2")) continue;
            count++;
            var verified = assertDoesNotThrow(() -> verify(vector));
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, verified.level());
            assertEquals("receipted", verified.level().wire());
            assertEquals(0, verified.status());
            assertEquals(1, verified.receipt().resultCode());
            assertArrayEquals(bytes(vector, "receiptDigest"), verified.receiptDigest());
            assertNotNull(verified.revert());
            assertEquals(PlatformSdkException.Code.REVERT, verified.revert().code());
            assertEquals("LXP_INSUFFICIENT_BALANCE: insufficient balance", verified.revert().revertReason());
            assertEquals(verified.revert().revertReason(), verified.revert().getMessage());
            assertEquals(verified.revert().revertReason(), verified.revert().safeDetails().get("revertReason"));
            assertEquals(PlatformSdkException.Retry.NEVER, verified.revert().retry());
            assertNull(verified.revert().receiptCheck());
            assertSame(verified.revert(), assertThrows(PlatformSdkException.class, verified::requireSuccess));
        }
        assertEquals(1, count, "the V2 vector must execute");
    }

    @Test
    void sequencerReceiptV3RefusalsNameTheirCheck() throws Exception {
        Map<String, ReceiptCheck> checks = Map.of(
            "V3-key-not-pinned", ReceiptCheck.KEY_ID,
            "V3-height-outside-key-range", ReceiptCheck.KEY_HEIGHT,
            "V3-activity-id-not-raw-hash", ReceiptCheck.RAW_TRANSACTION,
            "V3-seq-hash-mismatch", ReceiptCheck.SEQUENCE_HASH,
            "V3-tampered-log", ReceiptCheck.LOGS,
            "V3-non-canonical-bytes", ReceiptCheck.CANONICAL_ENCODING);
        Map<ReceiptCheck, String> rungs = Map.of(ReceiptCheck.KEY_ID, "R3", ReceiptCheck.KEY_HEIGHT, "R3",
            ReceiptCheck.RAW_TRANSACTION, "R4", ReceiptCheck.SEQUENCE_HASH, "R6",
            ReceiptCheck.LOGS, "R5", ReceiptCheck.CANONICAL_ENCODING, "R2");
        int count = 0;
        for (JsonNode vector : fixture().required("cases")) {
            if (!vector.required("vector").asText().equals("V3")) continue;
            count++;
            String id = vector.required("id").asText();
            ReceiptCheck expected = checks.get(id);
            assertNotNull(expected, id);
            assertEquals(rungs.get(expected), vector.required("expected").required("refusedAt").asText(), id);
            assertRefusal(expected, () -> verify(vector));
        }
        assertEquals(checks.size(), count, "every V3 refusal must execute");
    }
}
