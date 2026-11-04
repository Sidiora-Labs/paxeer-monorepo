package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.DatabindException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.lang.reflect.RecordComponent;
import java.math.BigInteger;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

public final class GeneratedCatalogueProgramsTest {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final String PROGRAMS = "0x000000000000000000000000000000000000101d";

    @Test
    void programOperationsNameLayerxprogramsCalls() throws Exception {
        Map<String, List<String>> calls = Map.of(
            "program.activity", List.of(),
            "program.call", List.of("call"),
            "program.deploy", List.of("deploy"),
            "program.discover", List.of("getProgram"),
            "program.interface", List.of("getInterface"),
            "program.receipt", List.of(),
            "program.simulate", List.of("simulateCall"),
            "program.upgrade", List.of("upgrade"),
            "program.wind-down", List.of("setExitRoute", "deprecate", "tombstone", "exitValue"));
        Map<String, String> reads = Map.of(
            "program.activity", retainedTransactionMethod("program.activity"),
            "program.discover", "eth_call",
            "program.interface", "eth_call",
            "program.receipt", retainedTransactionMethod("program.receipt"),
            "program.simulate", "eth_call");
        assertEquals(53, GeneratedContract.AGENT_OPERATIONS.size());
        assertEquals(Set.class, GeneratedContract.class.getDeclaredField("AGENT_OPERATIONS").getType());
        assertEquals(0, GeneratedContract.class.getDeclaredClasses().length);
        assertEquals(Set.of("HUMAN_ROUTES"), Arrays.stream(GeneratedContract.class.getDeclaredFields())
            .filter(field -> Map.class.isAssignableFrom(field.getType()))
            .map(java.lang.reflect.Field::getName).collect(Collectors.toSet()));
        assertFalse(GeneratedContract.AGENT_OPERATIONS.contains("faucet.claim"));
        assertEquals(GeneratedContract.AGENT_OPERATIONS, GeneratedSchema.AGENT.keySet());
        assertEquals(calls.keySet(), GeneratedContract.AGENT_OPERATIONS.stream()
            .filter(name -> name.startsWith("program.")).collect(Collectors.toSet()));
        assertEquals(calls.keySet(), GeneratedSchema.AGENT.keySet().stream()
            .filter(name -> name.startsWith("program.")).collect(Collectors.toSet()));

        var checks = new ArrayList<org.junit.jupiter.api.function.Executable>();
        for (var entry : calls.entrySet()) {
            checks.add(() -> {
                String name = entry.getKey();
                var descriptor = GeneratedSchema.AGENT.get(name);
                var transport = GeneratedSchema.programTransport(descriptor);
                String constant = name.toUpperCase(Locale.ROOT).replace('.', '_').replace('-', '_');
                assertAll(name,
                    () -> assertEquals(reads.getOrDefault(name, "eth_sendRawTransaction"), transport.rpcMethod(), name),
                    () -> assertEquals(entry.getValue().isEmpty() ? "" : PROGRAMS, transport.to(), name),
                    () -> assertEquals(entry.getValue(), transport.calls().stream()
                        .map(call -> call.substring("layerxprograms.".length(), call.indexOf('('))).toList(), name),
                    () -> assertTrue(transport.calls().stream().allMatch(call -> call.startsWith("layerxprograms.")
                        && call.endsWith(")")), name),
                    () -> assertFalse(JSON.writeValueAsString(transport).contains("/v1/programs"), name),
                    () -> assertSame(transport, GeneratedSchema.programTransport(descriptor)),
                    () -> assertSame(transport, GeneratedSchema.programTransport(OperationCatalog.agent(name))),
                    () -> assertSame(transport,
                        GeneratedSchema.AgentOperations.class.getField(constant + "_TRANSPORT").get(null)),
                    () -> assertThrows(UnsupportedOperationException.class, () -> transport.calls().add("invalid")));
            });
        }
        assertEquals(List.of("layerxprograms.call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)"),
            GeneratedSchema.AgentOperations.PROGRAM_CALL_TRANSPORT.calls());
        assertEquals(List.of("layerxprograms.getProgram(bytes32)"),
            GeneratedSchema.AgentOperations.PROGRAM_DISCOVER_TRANSPORT.calls());
        assertFalse(GeneratedSchema.AGENT.containsKey("program.unknown"));
        assertThrows(PlatformSdkException.class,
            () -> GeneratedSchema.programTransport(GeneratedSchema.AgentOperations.PREPARE));
        assertThrows(PlatformSdkException.class,
            () -> GeneratedSchema.programTransport(GeneratedSchema.HumanOperations.JOURNEY_GET));
        assertThrows(UnsupportedOperationException.class, () -> GeneratedContract.AGENT_OPERATIONS.remove("program.call"));
        assertAll("canonical program transports", checks);
    }

    private static String retainedTransactionMethod(String operation) throws Exception {
        JsonNode operations = Repair22331391Test.fixture("sdk/platform/conformance/operations.json")
            .required("operations");
        assertTrue(operations.isObject(), "canonical operations are grouped by operation id");
        for (JsonNode group : operations) {
            assertTrue(group.isArray(), "each canonical operation group must be an array");
            for (JsonNode descriptor : group) {
                if ("agent".equals(descriptor.path("plane").textValue())
                        && operation.equals(descriptor.path("name").textValue())) {
                    assertEquals("evm", descriptor.required("transport").textValue(), operation);
                    assertEquals("", descriptor.required("path").textValue(), operation);
                    assertEquals("eth_getTransactionByHash", descriptor.required("method").textValue(), operation);
                    return descriptor.required("method").textValue();
                }
            }
        }
        throw new AssertionError("Missing canonical operation: " + operation);
    }

    @Test
    void programReceiptTracesRetainedTransactionThroughHttp() throws Exception {
        JsonNode vector = null;
        for (JsonNode candidate : Repair22331391Test.fixture(
                "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").required("cases")) {
            if ("V1-layerxprograms-call".equals(candidate.required("id").textValue())) vector = candidate;
        }
        assertNotNull(vector, "the recorded native Programs receipt must execute");
        byte[] raw = Repair22331391Test.bytes(vector, "rawTransaction");
        byte[] hash = ProgramsClient.Programs.transactionHash(raw);
        assertArrayEquals(hash, Repair22331391Test.bytes(vector.required("receiptJson"), "txHash"));
        assertEquals(PROGRAMS, vector.required("receiptJson").required("to").textValue());
        assertArrayEquals(hash, NativeProgramCall.decodeSignedActivity(raw).bindSignedActivity(raw));
        var retained = new ProgramsClient.ProgramSubmission(hash, raw, null);
        try (var archive = new Repair22331391Test.RecordedReceiptArchive(vector.required("receiptJson"));
                var connection = new ProgramsClient.Programs(archive.uri())) {
            var client = Repair22331391Test.client(connection, vector);
            for (int lookup = 0; lookup < 2; lookup++) {
                var resolved = client.receipt(retained).toCompletableFuture().get(10, TimeUnit.SECONDS);
                assertFalse(resolved.pending());
                assertArrayEquals(raw, resolved.signedTransaction());
                assertArrayEquals(hash, resolved.transactionHash());
                assertArrayEquals(Repair22331391Test.bytes(vector, "receiptDigest"), resolved.receipt().receiptDigest());
                assertArrayEquals(Repair22331391Test.bytes(vector, "signedReceipt"), resolved.receipt().canonicalBytes());
                assertEquals(1, resolved.receipt().status());
            }
            archive.assertLookups(2, hash);
        }
    }

    @Test
    void nativePrepareReturnsEip1559Transaction() throws Exception {
        Set<String> fields = Set.of("chainId", "nonce", "gas", "to", "data");
        for (Class<?> type : List.of(GeneratedSchema.AgentModels.NativePrepareResultV1.class,
                GeneratedSchema.AgentOperations.PrepareResponse.class)) {
            assertTrue(type.isRecord());
            Set<String> components = Arrays.stream(type.getRecordComponents())
                .map(RecordComponent::getName).collect(Collectors.toSet());
            assertEquals(fields, components);
            assertFalse(components.contains("activity"));
            assertFalse(components.contains("moduleId"));
        }
        assertSame(GeneratedSchema.AgentOperations.PrepareResponse.class,
            GeneratedSchema.AgentOperations.PREPARE.responseType());
        assertSame(GeneratedSchema.AgentModels.NativePrepareResultV1.class,
            GeneratedSchema.AgentModels.McpNativeWriteV1.class.getMethod("preparation").getReturnType());
        ObjectNode transaction = JSON.createObjectNode()
            .put("chainId", BigInteger.valueOf(125)).put("nonce", BigInteger.valueOf(7))
            .put("gas", BigInteger.valueOf(21000))
            .put("to", PROGRAMS).put("data", "0xb1ffb3d4" + "00".repeat(32));
        var nativeResult = JSON.treeToValue(transaction, GeneratedSchema.AgentModels.NativePrepareResultV1.class);
        assertEquals(BigInteger.valueOf(125), nativeResult.chainId());
        assertEquals(BigInteger.valueOf(7), nativeResult.nonce());
        assertEquals(BigInteger.valueOf(21000), nativeResult.gas());
        assertEquals(PROGRAMS, nativeResult.to());
        assertEquals(transaction.get("data").textValue(), nativeResult.data());
        assertEquals(transaction, JSON.valueToTree(nativeResult));
        var prepared = JSON.treeToValue(transaction, GeneratedSchema.AgentOperations.PREPARE.responseType());
        assertEquals(nativeResult.chainId(), prepared.chainId());
        assertEquals(nativeResult.nonce(), prepared.nonce());
        assertEquals(nativeResult.gas(), prepared.gas());
        assertEquals(nativeResult.to(), prepared.to());
        assertEquals(nativeResult.data(), prepared.data());
        assertEquals(transaction, JSON.valueToTree(prepared));

        for (Class<?> type : List.of(GeneratedSchema.AgentModels.NativePrepareResultV1.class,
                GeneratedSchema.AgentOperations.PREPARE.responseType())) {
            for (String field : fields) {
                ObjectNode missing = transaction.deepCopy();
                missing.remove(field);
                assertThrows(DatabindException.class, () -> JSON.treeToValue(missing, type), field);
            }
            ObjectNode kernel = transaction.deepCopy();
            kernel.set("activity", JSON.createObjectNode().put("version", "1").put("module", "9").put("ordinal", "1"));
            assertThrows(DatabindException.class, () -> JSON.treeToValue(kernel, type));
            for (String field : List.of("chainId", "nonce", "gas")) {
                ObjectNode negative = transaction.deepCopy().put(field, -1);
                assertThrows(DatabindException.class, () -> JSON.treeToValue(negative, type), field);
                ObjectNode overflow = transaction.deepCopy().put(field, BigInteger.ONE.shiftLeft(64));
                assertThrows(DatabindException.class, () -> JSON.treeToValue(overflow, type), field);
            }
        }
    }
}
