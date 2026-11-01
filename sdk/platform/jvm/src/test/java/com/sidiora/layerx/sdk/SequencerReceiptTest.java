package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.GeneratedReceiptContract.ReceiptCheck;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyPairGenerator;
import java.security.MessageDigest;
import java.security.Signature;
import java.util.Arrays;
import java.util.HashSet;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

public final class SequencerReceiptTest {
    private static final ObjectMapper JSON = new ObjectMapper();

    private static JsonNode fixture(String name) throws Exception {
        Path directory = Path.of("").toAbsolutePath();
        while (directory != null) {
            Path fixture = directory.resolve("sdk/platform/conformance/fixtures/" + name);
            if (Files.isRegularFile(fixture)) return JSON.readTree(Files.readString(fixture));
            directory = directory.getParent();
        }
        throw new IllegalStateException(name + " not found");
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

    private static LocalVerifier.SequencerReceiptVerification verifySequencerReceipt(JsonNode vector) {
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
    void publishedVectorFilesMatchRecordedCases() {
        assertAll("published V1-V3 cases",
            this::sequencerReceiptV1CasesVerifyReceipted,
            this::sequencerReceiptV2RevertDecodesReason,
            this::sequencerReceiptV3RefusalsNameTheirCheck);
    }

    @Test
    void sequencerReceiptV1CasesVerifyReceipted() throws Exception {
        int count = 0;
        var seen = new HashSet<String>();
        for (JsonNode vector : fixture("vector-v1-receipted-positive.json").required("cases")) {
            assertEquals("V1", vector.required("vector").asText());
            count++;
            String id = vector.required("id").asText();
            assertTrue(seen.add(id), "duplicate V1 case: " + id);
            assertEquals("accepts", vector.required("expected").required("verifier").asText(), id);
            var verified = verifySequencerReceipt(vector);
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
    void sequencerReceiptWithTopLevelTransferSetRootIsNotAFieldsRefusal() throws Exception {
        JsonNode vector = fixture("vector-v1-receipted-positive.json").required("cases").required(0);
        assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, verifySequencerReceipt(vector).level());
        byte[] receipt = bytes(vector, "signedReceipt");
        int signatureOffset = receipt.length - 64;
        int signatureFlagOffset = signatureOffset - 4 - 1;
        int transferSetRootOffset = signatureFlagOffset - 8 - 3 * (4 + 32) + 4;
        int authorizationHashOffset = transferSetRootOffset + 32 + 4;
        Arrays.fill(receipt, transferSetRootOffset, transferSetRootOffset + 32, (byte) 0x11);
        Arrays.fill(receipt, authorizationHashOffset, authorizationHashOffset + 32, (byte) 0x22);

        byte[] unsigned = Arrays.copyOf(receipt, signatureFlagOffset + 1);
        unsigned[signatureFlagOffset] = 0;
        MessageDigest digest = MessageDigest.getInstance("SHA-256");
        digest.update("LXP/v1/receipt\0".getBytes(StandardCharsets.US_ASCII));
        var signingKey = KeyPairGenerator.getInstance("Ed25519").generateKeyPair();
        Signature signer = Signature.getInstance("Ed25519");
        signer.initSign(signingKey.getPrivate());
        signer.update(digest.digest(unsigned));
        System.arraycopy(signer.sign(), 0, receipt, signatureOffset, 64);
        byte[] publicKey = signingKey.getPublic().getEncoded();
        var pin = key(vector);
        var keys = new LocalVerifier.ReceiptKeys(List.of(new LocalVerifier.ReceiptKey(pin.keyId(),
            Arrays.copyOfRange(publicKey, publicKey.length - 32, publicKey.length),
            pin.firstHeight(), pin.lastHeight())));

        var verified = assertDoesNotThrow(() -> LocalVerifier.verifySequencerReceipt(
            receipt, bytes(vector, "rawTransaction"), keys),
            "non-zero top-level transferSetRoot and authorizationHash must verify without a FIELDS refusal");
        assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, verified.level());
        assertArrayEquals(Arrays.copyOfRange(receipt, transferSetRootOffset, transferSetRootOffset + 32),
            verified.receipt().transferSetRoot());
        assertArrayEquals(Arrays.copyOfRange(receipt, authorizationHashOffset, authorizationHashOffset + 32),
            verified.receipt().authorizationHash());
    }

    @Test
    void sequencerReceiptV2RevertDecodesReason() throws Exception {
        int count = 0;
        for (JsonNode vector : fixture("vector-v2-receipted-revert.json").required("cases")) {
            assertEquals("V2", vector.required("vector").asText());
            count++;
            assertEquals("accepts", vector.required("expected").required("verifier").asText());
            var verified = assertDoesNotThrow(() -> verifySequencerReceipt(vector));
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, verified.level());
            assertEquals("receipted", verified.level().wire());
            assertEquals(vector.required("expected").required("rung").asText(), verified.level().wire());
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
        var seen = new HashSet<String>();
        for (JsonNode vector : fixture("vector-v3-receipt-refusals.json").required("cases")) {
            assertEquals("V3", vector.required("vector").asText());
            count++;
            String id = vector.required("id").asText();
            assertTrue(seen.add(id), "duplicate V3 case: " + id);
            ReceiptCheck expected = checks.get(id);
            assertNotNull(expected, id);
            assertEquals(rungs.get(expected), vector.required("expected").required("refusedAt").asText(), id);
            assertRefusal(expected, () -> verifySequencerReceipt(vector));
        }
        assertEquals(checks.size(), count, "every V3 refusal must execute");
    }
}
