package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import org.junit.jupiter.api.Test;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import static org.junit.jupiter.api.Assertions.*;

public final class ReceiptFixtureTest {
    @Test
    void nativeLifecycleCFixtures() throws Exception {
        for (String name : new String[]{"deploy", "upgrade", "wind-down-route", "wind-down-deprecate", "wind-down-tombstone", "wind-down-exit"}) {
            JsonNode fixture = JSON.readTree(Files.readString(FIXTURE_ROOT.resolve("native-program-" + name + "-v3.json")));
            assertEquals(3, fixture.get("protocol_version").intValue()); assertEquals(9, fixture.get("module").intValue());
            int ordinal = fixture.get("ordinal").intValue(); byte[] payload = hexDecode(fixture.get("payload_hex").asText());
            byte[] signed = hexDecode(fixture.get("signed_activity_hex").asText());
            NativeProgramLifecycle value = NativeProgramLifecycle.decode(ordinal, payload); assertArrayEquals(payload, value.encode());
            var request = new NativeProgramLifecycleRequest(value, signed);
            assertArrayEquals(hexDecode(fixture.get("activity_id_hex").asText()), request.activityId());
            assertEquals(fixture.get("idempotency_key_hex").asText(), request.idempotencyKey());
            for (int length = 0; length < payload.length; length++) {
                byte[] prefix = java.util.Arrays.copyOf(payload, length);
                assertThrows(IllegalArgumentException.class, () -> NativeProgramLifecycle.decode(ordinal, prefix));
            }
            assertThrows(IllegalArgumentException.class, () -> NativeProgramLifecycle.decode(ordinal, java.util.Arrays.copyOf(payload, payload.length + 1)));
            byte[] changedPayload = payload.clone(); changedPayload[0] ^= 1;
            assertThrows(IllegalArgumentException.class, () -> new NativeProgramLifecycleRequest(NativeProgramLifecycle.decode(ordinal, changedPayload), signed));
            for (int offset : new int[]{1, 7, 17}) {
                byte[] changed = signed.clone(); changed[offset] ^= 1;
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

    private static final String PROGRAM_OUTCOME_V3 = "505247330100000000000100010000000700000001000000000000000b000000000000000c000000000000000d000000000000000e00000001000000000000000f0000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000020000000000000003000000000000000400000000000000050000000000000006000000000000000700000020000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000201111111111111111111111111111111111111111111111111111111111111111000000202222222222222222222222222222222222222222222222222222222222222222000000200000000000000000000000000000000000000000000000000000000000000000";

    @Test
    void nativeSignedBinding() throws Exception {
        JsonNode fixture = JSON.readTree(Files.readString(FIXTURE_ROOT.resolve("native-program-call-v3.json")));
        byte[] payload = hexDecode(fixture.get("payload_hex").asText());
        NativeProgramCall nativeCall = NativeProgramCall.decode(payload);
        assertArrayEquals(payload, nativeCall.encode());
        byte[] signed = hexDecode(fixture.get("signed_activity_hex").asText());
        var method = ProgramsClient.class.getDeclaredMethod("decodeSignedCall", ProgramsClient.Call.class);
        method.setAccessible(true);
        method.invoke(null, new ProgramsClient.Call(nativeCall, BigInteger.valueOf(1000), signed));
        assertThrows(java.lang.reflect.InvocationTargetException.class, () -> method.invoke(null, new ProgramsClient.Call(nativeCall, BigInteger.valueOf(999), signed)));
        NativeProgramCall changed = new NativeProgramCall(nativeCall.programId(), nativeCall.guestAbi(), nativeCall.entrypoint(), nativeCall.calldata(), nativeCall.capabilities(), nativeCall.accessDeclaration(), (nativeCall.responseCapacity() + 1) % 1_048_577, nativeCall.resources());
        assertNotEquals(nativeCall.responseCapacity(), changed.responseCapacity());
        changed.encode();
        assertThrows(java.lang.reflect.InvocationTargetException.class, () -> method.invoke(null, new ProgramsClient.Call(changed, BigInteger.valueOf(1000), signed)));
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
