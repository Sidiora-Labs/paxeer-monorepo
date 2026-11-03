package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.io.ByteArrayOutputStream;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.Set;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

public final class ChainsTest {
    private static final ObjectMapper JSON = new ObjectMapper();

    @Test
    void chainsVectorsMatchAndRoundTrip() throws Exception {
        JsonNode vectors = fixture();
        assertEquals(vectors.required("address").textValue(), Chains.ADDRESS);
        var methods = new HashSet<String>();
        for (JsonNode vector : vectors.required("methods")) {
            String method = vector.required("name").textValue();
            assertTrue(methods.add(method), "unique method vector");
            var arguments = new ArrayList<Object>();
            for (JsonNode argument : vector.required("args")) {
                arguments.add(argument.isIntegralNumber() ? argument.bigIntegerValue()
                    : argument.isBoolean() ? argument.booleanValue() : argument.textValue());
            }
            byte[] selector = Chains.selector(method);
            byte[] encoded = Chains.encodeCall(method, arguments.toArray());
            assertArrayEquals(bytes(vector.required("selector")), selector, method + " selector");
            assertArrayEquals(bytes(vector.required("calldata")), encoded, method + " calldata");
            assertArrayEquals(selector, Arrays.copyOf(encoded, 4), method);
            Chains.Call decoded = Chains.decodeCall(encoded);
            assertEquals(method, decoded.method());
            assertEquals(arguments, decoded.arguments(), method + " arguments");
            assertArrayEquals(encoded, Chains.encodeCall(decoded.method(), decoded.arguments().toArray()));
        }
        assertEquals(Set.of("createChain", "fundChain", "pauseChain", "resumeChain", "retireChain",
            "governancePause", "setOperator", "getChain", "chainByName", "chainCount", "chainIdOf",
            "params"), methods, "every ABI method must execute");

        byte[] chainBytes = bytes(vectors.required("getChain").required("returndata"));
        Chains.Chain chain = Chains.decodeGetChain(chainBytes);
        ObjectNode chainJson = JSON.valueToTree(chain);
        chainJson.put("bond", chain.bond().toString());
        chainJson.put("baseFeePerGas", chain.baseFeePerGas().toString());
        assertJsonFields(vectors.required("getChain").required("decoded"), chainJson);
        assertArrayEquals(chainBytes, Chains.encodeGetChain(chain), "getChain bytes");
        assertEquals(chain, Chains.decodeGetChain(Chains.encodeGetChain(chain)));

        byte[] paramsBytes = bytes(vectors.required("params").required("returndata"));
        Chains.Params params = Chains.decodeParams(paramsBytes);
        ObjectNode paramsJson = JSON.valueToTree(params);
        paramsJson.put("minBond", params.minBond().toString());
        paramsJson.put("creationFee", params.creationFee().toString());
        assertJsonFields(vectors.required("params").required("decoded"), paramsJson);
        assertArrayEquals(paramsBytes, Chains.encodeParams(params), "params bytes");
        assertEquals(params, Chains.decodeParams(Chains.encodeParams(params)));

        var events = new HashSet<String>();
        for (JsonNode vector : vectors.required("events")) {
            String name = vector.required("name").textValue();
            assertTrue(events.add(name));
            byte[] topic = bytes(vector.required("topic0"));
            assertArrayEquals(topic, Chains.eventTopic0(name), name + " topic0");
            byte[] id = word(chainBytes, 0);
            byte[] operator = word(chainBytes, 2);
            Chains.Event decoded = switch (name) {
                case "ChainCreated" -> Chains.decodeEvent(List.of(topic, id, operator),
                    join(Arrays.copyOfRange(Chains.encodeCall("getChain", 64), 4, 36),
                        word(chainBytes, 4), Arrays.copyOfRange(chainBytes, 11 * 32, chainBytes.length)));
                case "ChainStatusChanged" -> Chains.decodeEvent(List.of(topic, id), word(chainBytes, 3));
                case "ChainFunded" -> Chains.decodeEvent(List.of(topic, id, operator), word(chainBytes, 8));
                case "ChainOperatorChanged" -> Chains.decodeEvent(List.of(topic, id, operator), new byte[0]);
                default -> throw new AssertionError("unexpected event " + name);
            };
            assertEquals(name, decoded.name());
            Map<String, Object> expected = switch (name) {
                case "ChainCreated" -> Map.of("id", BigInteger.ONE, "name", chain.name(),
                    "operator", chain.operator(), "chainId", chain.chainId());
                case "ChainStatusChanged" -> Map.of("id", BigInteger.ONE, "status", BigInteger.ONE);
                case "ChainFunded" -> Map.of("id", BigInteger.ONE, "from", chain.operator(), "amount", chain.bond());
                case "ChainOperatorChanged" -> Map.of("id", BigInteger.ONE, "operator", chain.operator());
                default -> throw new AssertionError(name);
            };
            assertEquals(expected, decoded.arguments(), name + " indexed and data fields");
        }
        assertEquals(Set.of("ChainCreated", "ChainStatusChanged", "ChainFunded", "ChainOperatorChanged"), events);
        assertEquals(chain.chainId().longValueExact(), Chains.virtualChainId(chain.id()));
        assertEquals(125000000L, Chains.virtualChainId(0));
        assertEquals(4419967295L, Chains.virtualChainId(0xffffffffL));
        assertEquals("https://rpc.example/chain/1", Chains.chainRpcUrl("https://rpc.example", 1));
        assertEquals("https://rpc.example/evm/chain/1", Chains.chainRpcUrl("https://rpc.example/evm///", 1));
        assertEquals("https://rpc.example/chain/4294967295",
            Chains.chainRpcUrl("https://rpc.example/", 0xffffffffL));
        boundaries(chain, params);
        malformed(chainBytes, paramsBytes);
    }

    private static void boundaries(Chains.Chain chain, Chains.Params params) {
        BigInteger max64 = BigInteger.ONE.shiftLeft(64).subtract(BigInteger.ONE);
        BigInteger max256 = BigInteger.ONE.shiftLeft(256).subtract(BigInteger.ONE);
        for (String name : List.of("", "x".repeat(31), "x".repeat(32), "x".repeat(33), "鏈🌐".repeat(20))) {
            var decoded = Chains.decodeCall(Chains.encodeCall("createChain", name, max64, max64, 65535));
            assertEquals(List.of(name, max64, max64, BigInteger.valueOf(65535)), decoded.arguments());
            assertEquals(List.of(name), Chains.decodeCall(Chains.encodeCall("chainByName", name)).arguments());
        }
        assertEquals(List.of(BigInteger.valueOf(0xffffffffL), false),
            Chains.decodeCall(Chains.encodeCall("governancePause", 0xffffffffL, false)).arguments());
        var wideChain = new Chains.Chain(0xffffffffL, "鏈🌐", chain.operator(), 255,
            max64, max64, max64, 65535, max256, max256, max64);
        assertEquals(wideChain, Chains.decodeGetChain(Chains.encodeGetChain(wideChain)));
        var wideParams = new Chains.Params(params.authority(), max256, max256, max64, max64, max64, 0xffffffffL);
        assertEquals(wideParams, Chains.decodeParams(Chains.encodeParams(wideParams)));
        assertEquals(max256, Chains.decodeEvent(List.of(Chains.eventTopic0("ChainFunded"),
            word(Chains.encodeGetChain(wideChain), 0), word(Chains.encodeGetChain(wideChain), 2)),
            word(Chains.encodeGetChain(wideChain), 8)).arguments().get("amount"));
    }

    private static void malformed(byte[] chain, byte[] params) {
        assertThrows(IllegalArgumentException.class, () -> Chains.encodeCall("unknown"));
        assertThrows(IllegalArgumentException.class, () -> Chains.eventTopic0("unknown"));
        assertThrows(IllegalArgumentException.class, () -> Chains.encodeCall("fundChain"));
        assertThrows(IllegalArgumentException.class, () -> Chains.encodeCall("chainCount", 1));
        assertThrows(IllegalArgumentException.class, () -> Chains.encodeCall("fundChain", -1));
        assertThrows(IllegalArgumentException.class, () -> Chains.encodeCall("fundChain", 0x100000000L));
        assertThrows(IllegalArgumentException.class, () -> Chains.encodeCall("fundChain", 1.5));
        assertThrows(IllegalArgumentException.class, () -> Chains.encodeCall("governancePause", 1, 1));
        assertThrows(IllegalArgumentException.class, () -> Chains.encodeCall("setOperator", 1, "0x1234"));
        assertThrows(IllegalArgumentException.class, () -> Chains.encodeCall("createChain", "a", BigInteger.ONE.shiftLeft(64), 0, 0));
        assertThrows(IllegalArgumentException.class, () -> Chains.encodeCall("createChain", "a", 0, 0, 65536));
        assertThrows(IllegalArgumentException.class, () -> Chains.encodeCall("chainByName", "\uD800"));
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeCall(new byte[3]));
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeCall(new byte[4]));
        assertThrows(IllegalArgumentException.class, () -> Chains.virtualChainId(-1));
        assertThrows(IllegalArgumentException.class, () -> Chains.virtualChainId(0x100000000L));
        assertThrows(IllegalArgumentException.class, () -> Chains.chainRpcUrl("", 1));
        assertThrows(IllegalArgumentException.class, () -> Chains.chainRpcUrl("https://rpc.example", -1));
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeGetChain(Arrays.copyOf(chain, chain.length - 1)));
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeParams(Arrays.copyOf(params, params.length - 32)));
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeParams(join(params, new byte[32])));
        byte[] badOffset = chain.clone();
        badOffset[63] = 32;
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeGetChain(badOffset));
        byte[] badPadding = chain.clone();
        badPadding[badPadding.length - 1] = 1;
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeGetChain(badPadding));
        byte[] badString = chain.clone();
        badString[12 * 32] = (byte) 0xff;
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeGetChain(badString));
        byte[] badInteger = chain.clone();
        badInteger[0] = 1;
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeGetChain(badInteger));
        byte[] badAddress = params.clone();
        badAddress[0] = 1;
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeParams(badAddress));
        byte[] badBool = Chains.encodeCall("governancePause", 1, false);
        badBool[badBool.length - 1] = 2;
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeCall(badBool));
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeEvent(List.of(), new byte[0]));
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeEvent(List.of(new byte[32]), new byte[0]));
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeEvent(
            List.of(Chains.eventTopic0("ChainFunded"), word(chain, 0)), word(chain, 8)));
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeEvent(
            List.of(Chains.eventTopic0("ChainOperatorChanged"), word(chain, 0), word(chain, 2)), new byte[32]));
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeEvent(
            List.of(Chains.eventTopic0("ChainStatusChanged"), new byte[31]), word(chain, 3)));
        byte[] badStatus = word(chain, 3);
        badStatus[30] = 1;
        assertThrows(IllegalArgumentException.class, () -> Chains.decodeEvent(
            List.of(Chains.eventTopic0("ChainStatusChanged"), word(chain, 0)), badStatus));
    }

    private static void assertJsonFields(JsonNode expected, JsonNode actual) {
        assertEquals(expected.size(), actual.size(), "every tuple field");
        expected.fields().forEachRemaining(entry -> {
            JsonNode value = actual.required(entry.getKey());
            assertEquals(entry.getValue().isTextual(), value.isTextual(), entry.getKey() + " type");
            assertArrayEquals(entry.getValue().asText().getBytes(java.nio.charset.StandardCharsets.UTF_8),
                value.asText().getBytes(java.nio.charset.StandardCharsets.UTF_8), entry.getKey());
        });
    }

    private static JsonNode fixture() throws Exception {
        Path directory = Path.of("").toAbsolutePath();
        while (directory != null) {
            Path path = directory.resolve("tests/vectors/chains/abi-vectors.json");
            if (Files.isRegularFile(path)) return JSON.readTree(Files.readString(path));
            directory = directory.getParent();
        }
        throw new IllegalStateException("tests/vectors/chains/abi-vectors.json not found");
    }

    private static byte[] bytes(JsonNode value) {
        return HexFormat.of().parseHex(value.textValue().substring(2));
    }

    private static byte[] word(byte[] data, int index) {
        return Arrays.copyOfRange(data, index * 32, (index + 1) * 32);
    }

    private static byte[] join(byte[]... parts) {
        var output = new ByteArrayOutputStream();
        for (byte[] part : parts) output.writeBytes(part);
        return output.toByteArray();
    }
}
