package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.DatabindException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.lang.reflect.RecordComponent;
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
    void nativePrepareCarriesThePreparationRecord() throws Exception {
        Set<String> nativeFields = Set.of("version", "preparation_id", "canonical_bytes",
            "signing_preimage", "activity", "approval_required", "approval_id");
        Set<String> prepareFields = Set.of("preparation_ref", "unsigned_canonical_bytes",
            "signing_preimage", "disclosure", "expiry");
        ObjectNode nativeRecord = JSON.createObjectNode()
            .put("version", "1").put("preparation_id", "0x" + "11".repeat(32))
            .put("canonical_bytes", "0x01020304").put("signing_preimage", "0x" + "22".repeat(32))
            .put("approval_required", true).put("approval_id", "0x" + "33".repeat(32));
        nativeRecord.set("activity", JSON.valueToTree(
            new GeneratedSchema.AgentModels.NativeActivityV1("1", "9", "1")));
        ObjectNode prepareRecord = JSON.createObjectNode()
            .put("preparation_ref", nativeRecord.required("preparation_id").textValue())
            .put("unsigned_canonical_bytes", nativeRecord.required("canonical_bytes").textValue())
            .put("signing_preimage", nativeRecord.required("signing_preimage").textValue())
            .put("expiry", "1800000000");
        prepareRecord.set("disclosure", JSON.createObjectNode().put("purpose", "program.call"));
        ObjectNode kernel = JSON.createObjectNode().put("version", "1").put("module", "9").put("ordinal", "1");
        var checks = new ArrayList<org.junit.jupiter.api.function.Executable>();
        checks.add(() -> assertSame(GeneratedSchema.AgentOperations.PrepareResponse.class,
            GeneratedSchema.AgentOperations.PREPARE.responseType()));
        checks.add(() -> assertSame(GeneratedSchema.AgentModels.NativePrepareResultV1.class,
            GeneratedSchema.AgentModels.McpNativeWriteV1.class.getMethod("preparation").getReturnType()));
        for (Class<?> type : List.of(GeneratedSchema.AgentModels.NativePrepareResultV1.class,
                GeneratedSchema.AgentOperations.PREPARE.responseType())) {
            boolean nativeResult = type == GeneratedSchema.AgentModels.NativePrepareResultV1.class;
            Set<String> fields = nativeResult ? nativeFields : prepareFields;
            ObjectNode record = nativeResult ? nativeRecord : prepareRecord;
            checks.add(() -> {
                assertTrue(type.isRecord());
                Set<String> components = Arrays.stream(type.getRecordComponents())
                    .map(RecordComponent::getName).collect(Collectors.toSet());
                assertAll(type.getSimpleName(),
                    () -> assertEquals(fields, components),
                    () -> assertEquals(nativeResult, components.contains("activity")),
                    () -> assertFalse(components.contains("moduleId")));
            });
            checks.add(() -> {
                Object decoded = JSON.treeToValue(record, type);
                assertEquals(record, JSON.valueToTree(decoded));
                assertEquals(decoded, JSON.readValue(JSON.writeValueAsBytes(decoded), type));
                if (decoded instanceof GeneratedSchema.AgentModels.NativePrepareResultV1 value) {
                    assertEquals(record.required("version").textValue(), value.version());
                    assertEquals(record.required("preparation_id").textValue(), value.preparation_id());
                    assertEquals(record.required("canonical_bytes").textValue(), value.canonical_bytes());
                    assertEquals(record.required("signing_preimage").textValue(), value.signing_preimage());
                    assertEquals(record.required("activity"), JSON.valueToTree(value.activity()));
                    assertEquals(record.required("approval_required").booleanValue(), value.approval_required());
                    assertEquals(record.required("approval_id").textValue(), value.approval_id());
                    assertEquals(value.approval_id() != null, value.approval_required());
                } else {
                    var value = (GeneratedSchema.AgentOperations.PrepareResponse) decoded;
                    assertEquals(record.required("preparation_ref"), value.preparation_ref());
                    assertEquals(record.required("unsigned_canonical_bytes"), value.unsigned_canonical_bytes());
                    assertEquals(record.required("signing_preimage"), value.signing_preimage());
                    assertEquals(record.required("disclosure"), value.disclosure());
                    assertEquals(record.required("expiry"), value.expiry());
                }
            });
            for (String field : fields) {
                checks.add(() -> {
                    ObjectNode missing = record.deepCopy();
                    missing.remove(field);
                    assertThrows(DatabindException.class, () -> JSON.treeToValue(missing, type),
                        type.getSimpleName() + " missing " + field);
                });
            }
            checks.add(() -> assertThrows(DatabindException.class, () -> JSON.treeToValue(kernel, type),
                type.getSimpleName() + " refuses a kernel activity object"));
            for (String field : nativeResult ? List.of("moduleId") : List.of("activity", "moduleId")) {
                checks.add(() -> {
                    ObjectNode internal = record.deepCopy();
                    if ("activity".equals(field)) internal.set(field, kernel);
                    else internal.put(field, "9");
                    assertThrows(DatabindException.class, () -> JSON.treeToValue(internal, type),
                        type.getSimpleName() + " refuses " + field);
                });
            }
        }
        assertAll("native preparation records", checks);
    }
}
