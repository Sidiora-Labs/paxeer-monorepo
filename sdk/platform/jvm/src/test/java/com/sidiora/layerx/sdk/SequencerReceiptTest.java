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
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
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
        assertAll("published V1-V3 and fee-token cases",
            this::sequencerReceiptV1CasesVerifyReceipted,
            this::sequencerReceiptV2RevertDecodesReason,
            this::sequencerReceiptV3RefusalsNameTheirCheck,
            this::sequencerFeeTokenVectorBindsIndependentPaymentAndFeeAssets,
            this::sequencerFeeTokenSignedReceiptRejectsIndependentDenominationMutations);
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
    void sequencerFeeTokenVectorBindsIndependentPaymentAndFeeAssets() throws Exception {
        JsonNode vectors = fixture("sequencer-receipt-fee-token-v1.json");
        assertEquals("rpc/sequencer_receipt_vectors_test.go",
            vectors.required("provenance").required("generator").textValue());
        int count = 0;
        for (JsonNode vector : vectors.required("cases")) {
            count++;
            assertEquals("V1-fee-token-payment", vector.required("id").textValue());
            assertEquals("accepts", vector.required("expected").required("verifier").textValue());
            var verified = LocalVerifier.verifySequencerReceipt(bytes(vector, "signedReceipt"),
                bytes(vector, "rawTransaction"), new LocalVerifier.ReceiptKeys(List.of(key(vector))));
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, verified.level());
            assertEquals(1, verified.status());
            assertDoesNotThrow(verified::requireSuccess);
            assertArrayEquals(bytes(vector, "receiptDigest"), verified.receiptDigest());
            assertArrayEquals(bytes(vector, "signedReceipt"), verified.canonicalBytes());
            assertArrayEquals(ProgramsClient.Programs.transactionHash(bytes(vector, "rawTransaction")),
                verified.receipt().activityId());
            assertEquals("usid", vector.required("paymentDenom").textValue());
            assertEquals("uhpx", vector.required("feeDenom").textValue());
            assertNotEquals(vector.required("paymentDenom"), vector.required("feeDenom"));
            assertArrayEquals(bytes(vector, "asset"), verified.receipt().asset());
            assertArrayEquals(MessageDigest.getInstance("SHA-256").digest(
                vector.required("paymentDenom").textValue().getBytes(StandardCharsets.UTF_8)),
                verified.receipt().asset());
            assertFalse(MessageDigest.isEqual(verified.receipt().asset(), MessageDigest.getInstance("SHA-256")
                .digest(vector.required("feeDenom").textValue().getBytes(StandardCharsets.UTF_8))));
            assertEquals(1, verified.receipt().effects().size());
            byte[] summary = verified.receipt().effects().getFirst().body();
            assertEquals(113, summary.length);
            assertEquals(4, Byte.toUnsignedInt(summary[108]));
            assertArrayEquals("uhpx".getBytes(StandardCharsets.UTF_8), Arrays.copyOfRange(summary, 109, summary.length));
            var amounts = java.nio.ByteBuffer.wrap(summary);
            assertEquals(21000, amounts.getLong());
            assertEquals(21000, amounts.getLong());
            assertEquals(2000000000L, amounts.getLong());
            assertEquals(BigInteger.valueOf(21000).multiply(BigInteger.valueOf(2000000000L)),
                verified.receipt().feeCharged());
            byte[] wrongRaw = bytes(vector, "rawTransaction");
            wrongRaw[wrongRaw.length - 1] ^= 1;
            assertRefusal(ReceiptCheck.RAW_TRANSACTION, () -> LocalVerifier.verifySequencerReceipt(
                bytes(vector, "signedReceipt"), wrongRaw, new LocalVerifier.ReceiptKeys(List.of(key(vector)))));
        }
        assertEquals(1, count, "the separately published fee-token vector must execute");
    }

    @Test
    void sequencerFeeTokenSignedReceiptRejectsIndependentDenominationMutations() throws Exception {
        JsonNode vectors = fixture("sequencer-receipt-fee-token-v1.json").required("cases");
        assertEquals(1, vectors.size());
        JsonNode vector = vectors.required(0);
        byte[] raw = bytes(vector, "rawTransaction");
        byte[] signed = bytes(vector, "signedReceipt");
        var keys = new LocalVerifier.ReceiptKeys(List.of(key(vector)));
        var verified = LocalVerifier.verifySequencerReceipt(signed, raw, keys);
        assertArrayEquals(bytes(vector, "receiptDigest"), verified.receiptDigest());
        assertArrayEquals(ProgramsClient.Programs.transactionHash(raw), verified.receipt().activityId());
        byte[] paymentAsset = MessageDigest.getInstance("SHA-256").digest(
            vector.required("paymentDenom").textValue().getBytes(StandardCharsets.UTF_8));
        byte[] feeAsset = MessageDigest.getInstance("SHA-256").digest(
            vector.required("feeDenom").textValue().getBytes(StandardCharsets.UTF_8));
        byte[] feeDenom = vector.required("feeDenom").textValue().getBytes(StandardCharsets.UTF_8);
        byte[] paymentDenom = vector.required("paymentDenom").textValue().getBytes(StandardCharsets.UTF_8);
        assertFalse(MessageDigest.isEqual(paymentAsset, feeAsset));
        assertArrayEquals(paymentAsset, verified.receipt().asset());
        byte[] summary = verified.receipt().effects().getFirst().body();
        assertEquals(feeDenom.length, Byte.toUnsignedInt(summary[108]));
        assertArrayEquals(feeDenom, Arrays.copyOfRange(summary, 109, summary.length));
        int assetOffset = uniqueReceiptFieldOffset(signed, paymentAsset);
        int feeOffset = uniqueReceiptFieldOffset(signed, summary) + 109;
        assertTrue(assetOffset >= feeOffset + feeDenom.length);

        byte[] changedPayment = signed.clone();
        System.arraycopy(feeAsset, 0, changedPayment, assetOffset, feeAsset.length);
        assertArrayEquals(feeAsset, Arrays.copyOfRange(changedPayment, assetOffset, assetOffset + feeAsset.length));
        assertArrayEquals(summary, Arrays.copyOfRange(changedPayment, feeOffset - 109,
            feeOffset - 109 + summary.length));
        assertRefusal(ReceiptCheck.SIGNATURE,
            () -> LocalVerifier.verifySequencerReceipt(changedPayment, raw, keys));

        byte[] changedFee = signed.clone();
        assertEquals(feeDenom.length, paymentDenom.length);
        System.arraycopy(paymentDenom, 0, changedFee, feeOffset, paymentDenom.length);
        assertArrayEquals(paymentDenom, Arrays.copyOfRange(changedFee, feeOffset, feeOffset + paymentDenom.length));
        assertArrayEquals(paymentAsset, Arrays.copyOfRange(changedFee, assetOffset, assetOffset + paymentAsset.length));
        assertRefusal(ReceiptCheck.SIGNATURE,
            () -> LocalVerifier.verifySequencerReceipt(changedFee, raw, keys));
        assertArrayEquals(bytes(vector, "signedReceipt"), signed);
    }

    private static int uniqueReceiptFieldOffset(byte[] receipt, byte[] field) {
        String encoded = HexFormat.of().formatHex(receipt);
        String value = HexFormat.of().formatHex(field);
        int offset = encoded.indexOf(value);
        assertTrue(offset >= 0, "the producer receipt must contain the bound field");
        assertEquals(0, offset % 2, "the bound field must start at a byte boundary");
        assertEquals(offset, encoded.lastIndexOf(value), "the field mutation must be unambiguous");
        return offset / 2;
    }

    @Test
    void sequencerRecordedNodeDenominationMutationsAreRefusedIndependently() throws Exception {
        JsonNode vector = fixture("vector-v4-committed.json").required("positive");
        JsonNode node = Repair22331391Test.fixture("daemon/committed-fixture/vectors/vector-v4-committed.json")
            .required("settlement").required("receipt");
        assertRecordedNodeDenominations(vector, node);
    }

    private static void assertRecordedNodeDenominations(JsonNode vector, JsonNode node) throws Exception {
        byte[] raw = bytes(vector, "rawTransaction");
        byte[] hash = ProgramsClient.Programs.transactionHash(raw);
        byte[] snapshot = JSON.writeValueAsBytes(node);
        var keys = new LocalVerifier.ReceiptKeys(List.of(key(vector)));
        try (var archive = new Repair22331391Test.RecordedReceiptArchive(node);
                var connection = new ProgramsClient.Programs(archive.uri())) {
            var client = Repair22331391Test.client(connection, vector);
            var verified = client.sequencerReceipt(raw).toCompletableFuture().get(10, TimeUnit.SECONDS);
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, verified.level());
            assertEquals(1, verified.status());
            assertArrayEquals(bytes(vector, "receiptDigest"), verified.receiptDigest());
            assertArrayEquals(bytes(vector, "signedReceipt"), verified.canonicalBytes());
            assertArrayEquals(MessageDigest.getInstance("SHA-256").digest(
                node.required("paymentDenom").textValue().getBytes(StandardCharsets.UTF_8)), verified.receipt().asset());
            byte[] summary = verified.receipt().effects().getFirst().body();
            assertEquals(node.required("feeDenom").textValue().getBytes(StandardCharsets.UTF_8).length,
                Byte.toUnsignedInt(summary[108]));
            assertArrayEquals(node.required("feeDenom").textValue().getBytes(StandardCharsets.UTF_8),
                Arrays.copyOfRange(summary, 109, summary.length));
            for (String field : List.of("paymentDenom", "feeDenom")) {
                ObjectNode changed = node.deepCopy();
                String other = "usid".equals(node.required(field).textValue()) ? "uhpx" : "usid";
                changed.put(field, other);
                String independent = field.equals("paymentDenom") ? "feeDenom" : "paymentDenom";
                assertEquals(node.required(independent), changed.required(independent));
                assertEquals(node.required("receipt"), changed.required("receipt"));
                assertRefusal(ReceiptCheck.FIELDS, () -> ProgramsClient.verifyReceipt(
                    bytes(vector, "signedReceipt"), raw, keys, changed));
                archive.receipt.set(changed);
                ExecutionException error = assertThrows(ExecutionException.class,
                    () -> client.sequencerReceipt(raw).toCompletableFuture().get(10, TimeUnit.SECONDS), field);
                assertInstanceOf(PlatformSdkException.class, error.getCause());
                assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE,
                    ((PlatformSdkException) error.getCause()).code());
                assertEquals(ReceiptCheck.FIELDS, ((PlatformSdkException) error.getCause()).receiptCheck(), field);
                ObjectNode missing = node.deepCopy();
                missing.remove(field);
                assertRefusal(ReceiptCheck.FIELDS, () -> ProgramsClient.verifyReceipt(
                    bytes(vector, "signedReceipt"), raw, keys, missing));
            }
            archive.assertLookups(3, hash);
        }
        assertArrayEquals(snapshot, JSON.writeValueAsBytes(node));
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
