package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import org.junit.jupiter.api.Test;
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.List;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import static org.junit.jupiter.api.Assertions.*;

public final class ReceiptFixtureTest {
    @Test
    void nativeLifecycleCFixtures() throws Exception {
        for (String version : List.of("v3", "v4")) {
            for (String name : new String[]{"deploy", "upgrade", "wind-down-route", "wind-down-deprecate", "wind-down-tombstone", "wind-down-exit"}) {
                JsonNode fixture = JSON.readTree(Files.readString(FIXTURE_ROOT.resolve("native-program-" + name + "-" + version + ".json")));
                assertEquals(3, fixture.get("protocol_version").intValue()); assertEquals(9, fixture.get("module").intValue());
                int ordinal = fixture.get("ordinal").intValue(); byte[] payload = hexDecode(fixture.get("payload_hex").asText());
                NativeProgramLifecycle value = NativeProgramLifecycle.decode(ordinal, payload); assertArrayEquals(payload, value.encode());
                byte[] calldata = hexDecode(fixture.required("evm").required("calldata").asText().substring(2));
                assertArrayEquals(calldata, NativeProgramLifecycleRequest.calldata(value, BigInteger.valueOf(1000)));
                byte[] signed = signedTransaction(calldata);
                assertThrows(IllegalArgumentException.class, () -> new NativeProgramLifecycleRequest(value,
                    hexDecode(fixture.required("signed_activity_hex").asText())));
                var request = new NativeProgramLifecycleRequest(value, signed,
                    new IdempotencyKey(fixture.required("idempotency_key_hex").asText()));
                assertArrayEquals(ProgramsClient.Programs.transactionHash(signed), request.activityId());
                assertArrayEquals(signed, request.signedTransaction());
                assertEquals(ordinal, request.ordinal());
                assertArrayEquals(payload, request.payload());
                assertEquals(fixture.get("idempotency_key_hex").asText(), request.idempotencyKey());
                for (int length = 0; length < payload.length; length++) {
                    byte[] prefix = java.util.Arrays.copyOf(payload, length);
                    assertThrows(IllegalArgumentException.class, () -> NativeProgramLifecycle.decode(ordinal, prefix));
                }
                assertThrows(IllegalArgumentException.class, () -> NativeProgramLifecycle.decode(ordinal, java.util.Arrays.copyOf(payload, payload.length + 1)));
                byte[] changedPayload = payload.clone(); changedPayload[0] ^= 1;
                assertThrows(IllegalArgumentException.class, () -> new NativeProgramLifecycleRequest(NativeProgramLifecycle.decode(ordinal, changedPayload), signed));
                for (byte[] changed : List.of(new byte[] {0}, Arrays.copyOf(signed, signed.length + 1),
                        signedTransaction(new byte[] {0, 1, 2, 3}))) {
                    assertThrows(IllegalArgumentException.class, () -> new NativeProgramLifecycleRequest(value, changed));
                }
                for (int length = 0; length < signed.length; length++) {
                    byte[] prefix = java.util.Arrays.copyOf(signed, length);
                    assertThrows(IllegalArgumentException.class, () -> new NativeProgramLifecycleRequest(value, prefix));
                }
                if (ordinal == 1 || ordinal == 2) {
                    for (int offset : new int[]{35, 68, payload.length - 1}) {
                        byte[] changed = payload.clone(); changed[offset] ^= 1;
                        assertThrows(IllegalArgumentException.class, () -> NativeProgramLifecycle.decode(ordinal, changed));
                    }
                }
            }
        }
    }

    private static final String PROGRAM_OUTCOME_V3 = "505247330100000000000100010000000700000001000000000000000b000000000000000c000000000000000d000000000000000e00000001000000000000000f0000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000020000000000000003000000000000000400000000000000050000000000000006000000000000000700000020000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000201111111111111111111111111111111111111111111111111111111111111111000000202222222222222222222222222222222222222222222222222222222222222222000000200000000000000000000000000000000000000000000000000000000000000000";

    @Test
    void nativeSignedBinding() throws Exception {
        JsonNode fixture = JSON.readTree(Files.readString(FIXTURE_ROOT.resolve("native-program-call-v4.json")));
        byte[] payload = hexDecode(fixture.get("payload_hex").asText());
        NativeProgramCall nativeCall = NativeProgramCall.decode(payload);
        assertArrayEquals(payload, nativeCall.encode());
        byte[] signed = signedTransaction(nativeCall.calldata(BigInteger.valueOf(1000)));
        assertArrayEquals(ProgramsClient.Programs.transactionHash(signed), nativeCall.bindSignedActivity(signed));
        assertArrayEquals(payload, NativeProgramCall.decodeSignedActivity(signed).encode());
        NativeProgramCall changed = new NativeProgramCall(nativeCall.programId(), nativeCall.guestAbi(), nativeCall.entrypoint(), nativeCall.calldata(), nativeCall.capabilities(), nativeCall.accessDeclaration(), (nativeCall.responseCapacity() + 1) % 1_048_577, nativeCall.resources());
        assertNotEquals(nativeCall.responseCapacity(), changed.responseCapacity());
        assertThrows(IllegalArgumentException.class, () -> changed.bindSignedActivity(signed));
        assertThrows(PlatformSdkException.class, () -> nativeCall.bindSignedActivity(
            hexDecode(fixture.required("signed_activity_hex").asText())));
        for (int length = 0; length < signed.length; length++) {
            byte[] truncated = Arrays.copyOf(signed, length);
            assertThrows(PlatformSdkException.class, () -> nativeCall.bindSignedActivity(truncated));
        }
        for (int length = 0; length < payload.length; length++) {
            byte[] truncated = java.util.Arrays.copyOf(payload, length);
            assertThrows(IllegalArgumentException.class, () -> NativeProgramCall.decode(truncated));
        }
    }

    @Test
    void programOutcomeV3VectorDecodes() {
        var outcome = LocalVerifier.decodeProgramReceiptOutcome(hexDecode(PROGRAM_OUTCOME_V3), 1);
        assertEquals(3, outcome.encodingVersion());
        assertEquals(1, outcome.abiVersion());
        assertEquals(java.math.BigInteger.valueOf(16), outcome.feeUnits());
        org.junit.jupiter.api.Assertions.assertArrayEquals(hexDecode("11".repeat(32)), outcome.callGraphRoot());
        org.junit.jupiter.api.Assertions.assertArrayEquals(hexDecode("22".repeat(32)), outcome.terminalPayloadRoot());
    }
    static final String SENDER = "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf";

    static ProgramsClient.ProgramCall transaction(byte[] calldata) {
        var tx = JSON.createObjectNode().put("type", "0x2").put("chainId", "0x7d")
            .put("from", SENDER).put("to", ProgramsClient.PROGRAMS_ADDRESS)
            .put("nonce", "0x3").put("gas", "0x5208").put("maxPriorityFeePerGas", "0x1")
            .put("maxFeePerGas", "0x2").put("value", "0x0")
            .put("data", "0x" + HexFormat.of().formatHex(calldata));
        tx.putArray("accessList");
        return new ProgramsClient.ProgramCall(tx);
    }

    static byte[] signedTransaction(byte[] calldata) {
        return sign(transaction(calldata));
    }

    private static byte[] sign(ProgramsClient.ProgramCall call) {
        var curve = org.bouncycastle.crypto.ec.CustomNamedCurves.getByName("secp256k1");
        var domain = new org.bouncycastle.crypto.params.ECDomainParameters(curve.getCurve(), curve.getG(), curve.getN());
        var signer = new org.bouncycastle.crypto.signers.ECDSASigner(
            new org.bouncycastle.crypto.signers.HMacDSAKCalculator(new org.bouncycastle.crypto.digests.SHA256Digest()));
        signer.init(true, new org.bouncycastle.crypto.params.ECPrivateKeyParameters(BigInteger.ONE, domain));
        byte[] digest = ProgramsClient.Programs.transactionHash(call.signingPayload());
        BigInteger[] signature = signer.generateSignature(digest);
        BigInteger lowS = signature[1].min(curve.getN().subtract(signature[1]));
        signer.init(false, new org.bouncycastle.crypto.params.ECPublicKeyParameters(curve.getG(), domain));
        assertTrue(signer.verifySignature(digest, signature[0], lowS));
        List<byte[]> fields = transactionFields(call.signingPayload());
        for (int parity = 0; parity < 2; parity++) {
            var encoded = new java.io.ByteArrayOutputStream();
            for (int i = 0; i < fields.size(); i++) encoded.writeBytes(ProgramsClient.Programs.rlp(fields.get(i), i == 8));
            encoded.writeBytes(ProgramsClient.Programs.rlp(parity == 0 ? new byte[0] : new byte[] {1}, false));
            for (BigInteger component : new BigInteger[] {signature[0], lowS}) {
                byte[] bytes = component.toByteArray();
                if (bytes[0] == 0) bytes = Arrays.copyOfRange(bytes, 1, bytes.length);
                encoded.writeBytes(ProgramsClient.Programs.rlp(bytes, false));
            }
            byte[] signed = ProgramsClient.Programs.concat(new byte[] {2}, ProgramsClient.Programs.rlp(encoded.toByteArray(), true));
            try { ProgramsClient.Programs.bind(call, signed); return signed; }
            catch (PlatformSdkException error) { assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, error.code()); }
        }
        throw new AssertionError("neither recovery parity matches the signing key");
    }

    private static List<byte[]> transactionFields(byte[] raw) {
        ByteBuffer input = ByteBuffer.wrap(raw);
        assertEquals(2, input.get());
        ByteBuffer list = ByteBuffer.wrap(rlpValue(input));
        assertFalse(input.hasRemaining());
        List<byte[]> fields = new ArrayList<>();
        while (list.hasRemaining()) fields.add(rlpValue(list));
        return fields;
    }
    private static byte[] rlpValue(ByteBuffer input) {
        int prefix = input.get() & 255;
        if (prefix < 128) return new byte[] {(byte) prefix};
        int base = prefix >= 192 ? 192 : 128;
        int size = prefix - base;
        if (size > 55) {
            int lengthBytes = size - 55; size = 0;
            for (int i = 0; i < lengthBytes; i++) size = size * 256 + (input.get() & 255);
        }
        byte[] value = new byte[size]; input.get(value); return value;
    }

    private static final ObjectMapper JSON = new ObjectMapper();
    private static final Path FIXTURE_ROOT = Paths
        .get(System.getProperty("layerx.repo.root", "../../.."))
        .resolve("sdk/platform/conformance/fixtures");

    private static byte[] hexDecode(String hex) {
        assertEquals(0, hex.length() & 1, "hex must have even length");
        byte[] data = new byte[hex.length() / 2];
        for (int i = 0; i < hex.length(); i += 2) {
            data[i / 2] = (byte) ((Character.digit(hex.charAt(i), 16) << 4)
                + Character.digit(hex.charAt(i + 1), 16));
        }
        return data;
    }

}
