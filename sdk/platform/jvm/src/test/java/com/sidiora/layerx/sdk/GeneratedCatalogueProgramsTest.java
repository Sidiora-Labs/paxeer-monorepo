package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.DatabindException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.lang.reflect.RecordComponent;
import java.math.BigInteger;
import java.util.Arrays;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

public final class GeneratedCatalogueProgramsTest {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final String PROGRAMS = "0x000000000000000000000000000000000000101d";

    @Test
    void programOperationsNameLayerxprogramsCalls() throws Exception {
        Map<String, List<String>> calls = Map.of(
            "program.activity", List.of("ProgramCallOutcome"),
            "program.call", List.of("call"),
            "program.deploy", List.of("deploy"),
            "program.discover", List.of("getProgram"),
            "program.interface", List.of("getInterface"),
            "program.receipt", List.of("ProgramCallOutcome"),
            "program.simulate", List.of("simulateCall"),
            "program.upgrade", List.of("upgrade"),
            "program.wind-down", List.of("setExitRoute", "deprecate", "tombstone", "exitValue"));
        Map<String, String> reads = Map.of(
            "program.activity", "eth_getLogs",
            "program.discover", "eth_call",
            "program.interface", "eth_call",
            "program.receipt", "eth_getLogs",
            "program.simulate", "eth_call");
        assertEquals(calls.keySet(), GeneratedContract.AGENT_OPERATIONS.stream()
            .filter(name -> name.startsWith("program.")).collect(Collectors.toSet()));
        assertEquals(calls.keySet(), GeneratedSchema.AGENT.keySet().stream()
            .filter(name -> name.startsWith("program.")).collect(Collectors.toSet()));

        for (var entry : calls.entrySet()) {
            String name = entry.getKey();
            var descriptor = GeneratedSchema.AGENT.get(name);
            var transport = GeneratedContract.AGENT_OPERATIONS.transport(name);
            assertEquals(PROGRAMS, transport.to(), name);
            assertEquals(reads.getOrDefault(name, "eth_sendRawTransaction"), transport.rpcMethod(), name);
            assertEquals(entry.getValue(), transport.calls().stream()
                .map(call -> call.substring("layerxprograms.".length(), call.indexOf('('))).toList(), name);
            assertTrue(transport.calls().stream().allMatch(call -> call.startsWith("layerxprograms.")
                && call.endsWith(")")), name);
            assertFalse(JSON.writeValueAsString(transport).contains("/v1/programs"), name);
            assertSame(transport, GeneratedSchema.programTransport(descriptor));
            assertSame(transport, GeneratedSchema.programTransport(OperationCatalog.agent(name)));
            String constant = name.toUpperCase(Locale.ROOT).replace('.', '_').replace('-', '_');
            assertSame(transport, GeneratedSchema.AgentOperations.class.getField(constant + "_TRANSPORT").get(null));
            assertThrows(UnsupportedOperationException.class, () -> transport.calls().add("invalid"));
        }
        assertEquals(List.of("layerxprograms.call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)"),
            GeneratedSchema.AgentOperations.PROGRAM_CALL_TRANSPORT.calls());
        assertEquals(List.of("layerxprograms.getProgram(bytes32)"),
            GeneratedSchema.AgentOperations.PROGRAM_DISCOVER_TRANSPORT.calls());
        assertThrows(PlatformSdkException.class, () -> GeneratedContract.AGENT_OPERATIONS.transport("program.unknown"));
        assertThrows(UnsupportedOperationException.class, () -> GeneratedContract.AGENT_OPERATIONS.remove("program.call"));
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
