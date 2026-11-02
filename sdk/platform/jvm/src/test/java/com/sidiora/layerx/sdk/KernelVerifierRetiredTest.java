package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.Committed;
import com.sidiora.layerx.sdk.verify.GeneratedReceiptContract.ReceiptCheck;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.List;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

public final class KernelVerifierRetiredTest {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final String KERNEL_RECEIPT_V3 = "0003520100030000002001010101010101010101010101010101010101010101010101010101010101010000000000000009000000200202020202020202020202020202020202020202020202020202020202020202000000200303030303030303030303030303030303030303030303030303030303030303000000200808080808080808080808080808080808080808080808080808080808080808000000000000000000000000000000000000000000000001000000200404040404040404040404040404040404040404040404040404040404040404000100000001000000010100000020050505050505050505050505050505050505050505050505050505050505050500000000000000000000000000000019000000200606060606060606060606060606060606060606060606060606060606060606000000000000000000000000000000640000000000000000000000000000004b00000000000000010000002007070707070707070707070707070707070707070707070707070707070707070000000000000000000000000000000a00000000000000000000000000000023000000200909090909090909090909090909090909090909090909090909090909090909000000200a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a000000200b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b00000000000003e80100000040d933bba9e0fed523766e49289851174abbce76403c16c574f0a516553d105e34c61ca73bd7cb501d2b6d106c43841c5c6d737733cfdd8e885a3d3e308a634c0a";

    @Test
    void verificationRungsAreReceiptedAndCommittedOnly() {
        assertEquals(List.of("receipted", "committed"), Arrays.stream(LocalVerifier.VerificationLevel.values())
            .map(LocalVerifier.VerificationLevel::wire).toList());
        assertArrayEquals(new LocalVerifier.VerificationLevel[]{LocalVerifier.VerificationLevel.RECEIPTED,
            LocalVerifier.VerificationLevel.COMMITTED}, LocalVerifier.VerificationLevel.values());
        VerifyBatchInclusion();
        VerifyCheckpoint();
        AuthorizedBatch();
    }

    private static void VerifyBatchInclusion() {
        assertMethodsAbsent("decodeBatchHeader", "verifyBatchInclusion", "verifyReceiptInBatch");
        assertTypesAbsent("BatchHeader", "SequencerAuthorization", "InclusionKind",
            "InclusionVerification", "ReceiptInclusionVerification");
    }

    private static void VerifyCheckpoint() {
        assertMethodsAbsent("verifyCheckpoint", "verifyRecoverableSecp256k1");
        assertTypesAbsent("CheckpointAttestation", "GuarantorKey", "CheckpointCertificate",
            "CheckpointVerificationInput", "CheckpointVerification", "LocalSignatureVerifier");
    }

    private static void AuthorizedBatch() {
        assertTypesAbsent("AuthorizedReceiptBatch");
        assertMethodsAbsent("verifyReceipt", "verifyReceiptOutcome", "verifyProgramTerminalV5Receipt",
            "verifyProgramLifecycleReceipt");
    }

    @Test
    void kernelReceiptBytesAreRefusedBySequencerVerifier() throws Exception {
        JsonNode vector = fixture("vector-v1-receipted-positive.json").required("cases").get(0);
        var keys = keys(vector);
        byte[] raw = bytes(vector, "rawTransaction");
        assertEquals(LocalVerifier.VerificationLevel.RECEIPTED,
            LocalVerifier.verifySequencerReceipt(bytes(vector, "signedReceipt"), raw, keys).level());
        PlatformSdkException failure = assertThrows(PlatformSdkException.class,
            () -> LocalVerifier.verifySequencerReceipt(HexFormat.of().parseHex(KERNEL_RECEIPT_V3), raw, keys));
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, failure.code());
        assertEquals(ReceiptCheck.DECODE, failure.receiptCheck());
        assertEquals("decode", failure.safeDetails().get("receiptCheck"));
    }

    @Test
    void committedVectorV4Verifies() throws Exception {
        JsonNode fixture = fixture("vector-v4-committed.json");
        JsonNode positive = fixture.required("positive");
        var receipt = LocalVerifier.verifySequencerReceipt(bytes(positive, "signedReceipt"),
            bytes(positive, "rawTransaction"), keys(positive), positive.required("receiptJson"),
            bytes(positive, "prevSeqHash"));
        assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, receipt.level());
        assertArrayEquals(bytes(positive, "receiptDigest"), receipt.receiptDigest());
        var header = Committed.decodeHeader(positive.required("blockHeader"));
        var commit = Committed.decodeCommit(positive.required("commit"));
        var validators = Committed.decodeValidators(positive.required("validatorSet"));
        var committed = Committed.verify(receipt, bytes(positive, "rawTransaction"), header,
            Committed.decodeInclusion(positive.required("receiptInclusion")), commit, validators);
        assertEquals(positive.required("expected").required("rung").asText(), committed.level().wire());
        assertEquals(LocalVerifier.VerificationLevel.COMMITTED.wire(), committed.level().wire());
        assertEquals(header.height(), committed.height());
        assertArrayEquals(header.hash(), committed.blockHash());
        assertSame(receipt, committed.receipted());
        assertEquals(1, committed.status());
        assertDoesNotThrow(committed::requireSuccess);

        JsonNode refusals = fixture.required("refusals");
        assertTrue(refusals.isArray());
        assertFalse(refusals.isEmpty());
        for (JsonNode refusal : refusals) {
            String id = refusal.required("id").asText();
            String check = refusal.required("expectedCheck").asText();
            ObjectNode inclusion = positive.required("receiptInclusion").deepCopy();
            switch (check) {
                case "committed-next-commit" -> inclusion.set("NextCommit", refusal.required("nextCommit"));
                default -> fail("Unhandled committed refusal check: " + check);
            }
            var refusedInclusion = Committed.decodeInclusion(inclusion);
            PlatformSdkException failure = assertThrows(PlatformSdkException.class,
                () -> Committed.verify(receipt, bytes(positive, "rawTransaction"), header,
                    refusedInclusion, commit, validators), id + ": " + check);
            assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, failure.code(), id + ": " + check);
        }
    }

    private static void assertMethodsAbsent(String... names) {
        List<String> methods = Arrays.stream(LocalVerifier.class.getDeclaredMethods())
            .map(java.lang.reflect.Method::getName).toList();
        for (String name : names) assertFalse(methods.contains(name), name);
    }

    private static void assertTypesAbsent(String... names) {
        List<String> types = Arrays.stream(LocalVerifier.class.getDeclaredClasses())
            .map(Class::getSimpleName).toList();
        for (String name : names) assertFalse(types.contains(name), name);
    }

    private static LocalVerifier.ReceiptKeys keys(JsonNode vector) {
        JsonNode pin = vector.required("pinnedKey");
        return new LocalVerifier.ReceiptKeys(List.of(new LocalVerifier.ReceiptKey(bytes(pin, "keyId"),
            bytes(pin, "publicKey"), pin.required("firstHeight").bigIntegerValue(),
            pin.required("lastHeight").bigIntegerValue())));
    }

    private static byte[] bytes(JsonNode node, String field) {
        String value = node.required(field).textValue();
        assertNotNull(value, field);
        assertTrue(value.startsWith("0x"), field);
        return HexFormat.of().parseHex(value.substring(2));
    }

    private static JsonNode fixture(String name) throws Exception {
        Path directory = Path.of(System.getProperty("layerx.repo.root", ".")).toAbsolutePath();
        while (directory != null) {
            Path path = directory.resolve("sdk/platform/conformance/fixtures").resolve(name);
            if (Files.isRegularFile(path)) return JSON.readTree(Files.readString(path));
            directory = directory.getParent();
        }
        throw new IllegalStateException(name + " not found");
    }
}
