package com.sidiora.layerx.sdk;

import java.io.ByteArrayOutputStream;
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.charset.CharacterCodingException;
import java.nio.charset.CodingErrorAction;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import org.bouncycastle.crypto.digests.KeccakDigest;

public final class Chains {
    public static final String ADDRESS = "0x000000000000000000000000000000000000101e";
    private static final Map<String, String> METHODS = Map.ofEntries(
        Map.entry("createChain", "createChain(string,uint64,uint64,uint16)"),
        Map.entry("fundChain", "fundChain(uint32)"),
        Map.entry("pauseChain", "pauseChain(uint32)"),
        Map.entry("resumeChain", "resumeChain(uint32)"),
        Map.entry("retireChain", "retireChain(uint32)"),
        Map.entry("governancePause", "governancePause(uint32,bool)"),
        Map.entry("setOperator", "setOperator(uint32,address)"),
        Map.entry("getChain", "getChain(uint32)"),
        Map.entry("chainByName", "chainByName(string)"),
        Map.entry("chainCount", "chainCount()"),
        Map.entry("chainIdOf", "chainIdOf(uint32)"),
        Map.entry("params", "params()"));
    private static final Map<String, String> EVENTS = Map.of(
        "ChainCreated", "ChainCreated(uint32,string,address,uint64)",
        "ChainStatusChanged", "ChainStatusChanged(uint32,uint8)",
        "ChainFunded", "ChainFunded(uint32,address,uint256)",
        "ChainOperatorChanged", "ChainOperatorChanged(uint32,address)");
    private static final String[] CHAIN_TYPES = {
        "uint32", "string", "address", "uint8", "uint64", "uint64", "uint64",
        "uint16", "uint256", "uint256", "uint64"
    };
    private static final String[] PARAM_TYPES = {
        "address", "uint256", "uint256", "uint64", "uint64", "uint64", "uint32"
    };

    private Chains() {}

    public record Chain(long id, String name, String operator, int status, BigInteger chainId,
                        BigInteger gasAllowance, BigInteger byteAllowance, int feeShareBps,
                        BigInteger bond, BigInteger baseFeePerGas, BigInteger createdHeight) {}

    public record Params(String authority, BigInteger minBond, BigInteger creationFee,
                         BigInteger minGasAllowance, BigInteger maxGasAllowance,
                         BigInteger totalGasAllowanceLeft, long maxChains) {}

    public record Call(String method, List<Object> arguments) {
        public Call { arguments = List.copyOf(arguments); }
    }

    public record Event(String name, Map<String, Object> arguments) {
        public Event { arguments = Map.copyOf(arguments); }
    }

    public static byte[] selector(String method) {
        return Arrays.copyOf(hash(signature(METHODS, method)), 4);
    }

    public static byte[] eventTopic0(String event) {
        return hash(signature(EVENTS, event));
    }

    public static byte[] encodeCall(String method, Object... arguments) {
        var output = new ByteArrayOutputStream();
        output.writeBytes(selector(method));
        output.writeBytes(encode(types(signature(METHODS, method)), arguments));
        return output.toByteArray();
    }

    public static Call decodeCall(byte[] calldata) {
        Objects.requireNonNull(calldata, "calldata");
        if (calldata.length < 4) throw invalid("truncated selector");
        for (String method : METHODS.keySet()) {
            if (Arrays.equals(selector(method), Arrays.copyOf(calldata, 4))) {
                return new Call(method, decode(types(METHODS.get(method)),
                    Arrays.copyOfRange(calldata, 4, calldata.length)));
            }
        }
        throw invalid("unknown selector");
    }

    public static Chain decodeGetChain(byte[] data) {
        List<Object> v = decode(CHAIN_TYPES, data);
        return new Chain(((BigInteger) v.get(0)).longValueExact(), (String) v.get(1),
            (String) v.get(2), ((BigInteger) v.get(3)).intValueExact(), (BigInteger) v.get(4),
            (BigInteger) v.get(5), (BigInteger) v.get(6), ((BigInteger) v.get(7)).intValueExact(),
            (BigInteger) v.get(8), (BigInteger) v.get(9), (BigInteger) v.get(10));
    }

    public static byte[] encodeGetChain(Chain chain) {
        return encode(CHAIN_TYPES, new Object[] {chain.id(), chain.name(), chain.operator(),
            chain.status(), chain.chainId(), chain.gasAllowance(), chain.byteAllowance(),
            chain.feeShareBps(), chain.bond(), chain.baseFeePerGas(), chain.createdHeight()});
    }

    public static Params decodeParams(byte[] data) {
        List<Object> v = decode(PARAM_TYPES, data);
        return new Params((String) v.get(0), (BigInteger) v.get(1), (BigInteger) v.get(2),
            (BigInteger) v.get(3), (BigInteger) v.get(4), (BigInteger) v.get(5),
            ((BigInteger) v.get(6)).longValueExact());
    }

    public static byte[] encodeParams(Params params) {
        return encode(PARAM_TYPES, new Object[] {params.authority(), params.minBond(),
            params.creationFee(), params.minGasAllowance(), params.maxGasAllowance(),
            params.totalGasAllowanceLeft(), params.maxChains()});
    }

    public static Event decodeEvent(List<byte[]> topics, byte[] data) {
        Objects.requireNonNull(topics, "topics");
        if (topics.isEmpty()) throw invalid("missing event topic");
        for (byte[] topic : topics) {
            if (topic == null || topic.length != 32) throw invalid("invalid event topic");
        }
        String name = EVENTS.keySet().stream()
            .filter(event -> Arrays.equals(eventTopic0(event), topics.get(0)))
            .findFirst().orElseThrow(() -> invalid("unknown event topic"));
        int count = name.equals("ChainStatusChanged") ? 2 : 3;
        if (topics.size() != count) throw invalid("wrong event topic count");
        var args = new LinkedHashMap<String, Object>();
        args.put("id", decode(new String[] {"uint32"}, topics.get(1)).get(0));
        switch (name) {
            case "ChainCreated" -> {
                List<Object> body = decode(new String[] {"string", "uint64"}, data);
                args.put("name", body.get(0));
                args.put("operator", decode(new String[] {"address"}, topics.get(2)).get(0));
                args.put("chainId", body.get(1));
            }
            case "ChainStatusChanged" ->
                args.put("status", decode(new String[] {"uint8"}, data).get(0));
            case "ChainFunded" -> {
                args.put("from", decode(new String[] {"address"}, topics.get(2)).get(0));
                args.put("amount", decode(new String[] {"uint256"}, data).get(0));
            }
            case "ChainOperatorChanged" -> {
                decode(new String[0], data);
                args.put("operator", decode(new String[] {"address"}, topics.get(2)).get(0));
            }
            default -> throw invalid("unknown event");
        }
        return new Event(name, args);
    }

    public static long virtualChainId(long id) {
        unsigned(BigInteger.valueOf(id), 32);
        return 125000000L + id;
    }

    public static String chainRpcUrl(String base, long id) {
        virtualChainId(id);
        Objects.requireNonNull(base, "base");
        int end = base.length();
        while (end > 0 && base.charAt(end - 1) == '/') end--;
        if (end == 0) throw invalid("empty RPC base");
        return base.substring(0, end) + "/chain/" + id;
    }

    private static byte[] encode(String[] types, Object[] values) {
        if (values.length != types.length) throw invalid("wrong argument count");
        var head = new ByteArrayOutputStream();
        var tail = new ByteArrayOutputStream();
        for (int i = 0; i < types.length; i++) {
            if (types[i].equals("string")) {
                if (!(values[i] instanceof String text)) throw invalid("expected string");
                byte[] bytes = utf8(text);
                head.writeBytes(word(BigInteger.valueOf(types.length * 32L + tail.size()), 256));
                tail.writeBytes(word(BigInteger.valueOf(bytes.length), 256));
                tail.writeBytes(Arrays.copyOf(bytes, Math.toIntExact((bytes.length + 31L) / 32 * 32)));
            } else {
                head.writeBytes(encodeWord(types[i], values[i]));
            }
        }
        head.writeBytes(tail.toByteArray());
        return head.toByteArray();
    }

    private static List<Object> decode(String[] types, byte[] data) {
        Objects.requireNonNull(data, "data");
        if (data.length < types.length * 32 || data.length % 32 != 0)
            throw invalid("truncated ABI tuple");
        var values = new ArrayList<Object>();
        for (int i = 0; i < types.length; i++) {
            BigInteger number = number(data, i * 32);
            String type = types[i];
            if (type.equals("string")) {
                if (number.bitLength() > 31) throw invalid("string offset overflow");
                int offset = number.intValueExact();
                if (offset < types.length * 32 || offset % 32 != 0)
                    throw invalid("invalid string offset");
                BigInteger size = number(data, offset);
                if (size.bitLength() > 31 || size.longValue() > data.length - (long) offset - 32)
                    throw invalid("truncated string");
                try {
                    values.add(StandardCharsets.UTF_8.newDecoder()
                        .onMalformedInput(CodingErrorAction.REPORT)
                        .onUnmappableCharacter(CodingErrorAction.REPORT)
                        .decode(ByteBuffer.wrap(data, offset + 32, size.intValueExact())).toString());
                } catch (CharacterCodingException error) {
                    throw invalid("invalid UTF-8");
                }
            } else if (type.equals("address")) {
                unsigned(number, 160);
                values.add("0x" + HexFormat.of().formatHex(data, i * 32 + 12, i * 32 + 32));
            } else if (type.equals("bool")) {
                unsigned(number, 1);
                values.add(number.equals(BigInteger.ONE));
            } else {
                unsigned(number, Integer.parseInt(type.substring(4)));
                values.add(number);
            }
        }
        if (!Arrays.equals(data, encode(types, values.toArray())))
            throw invalid("noncanonical ABI tuple");
        return List.copyOf(values);
    }

    private static byte[] encodeWord(String type, Object value) {
        if (type.equals("address")) {
            if (!(value instanceof String text) || !text.matches("0x[0-9a-fA-F]{40}"))
                throw invalid("invalid address");
            return word(new BigInteger(text.substring(2), 16), 160);
        }
        if (type.equals("bool")) {
            if (!(value instanceof Boolean flag)) throw invalid("expected boolean");
            return word(flag ? BigInteger.ONE : BigInteger.ZERO, 1);
        }
        BigInteger number;
        if (value instanceof BigInteger integer) number = integer;
        else if (value instanceof Byte || value instanceof Short || value instanceof Integer || value instanceof Long)
            number = BigInteger.valueOf(((Number) value).longValue());
        else throw invalid("expected integer");
        return word(number, Integer.parseInt(type.substring(4)));
    }

    private static byte[] word(BigInteger value, int bits) {
        unsigned(value, bits);
        byte[] bytes = value.toByteArray();
        byte[] result = new byte[32];
        int length = Math.min(bytes.length, 32);
        System.arraycopy(bytes, bytes.length - length, result, 32 - length, length);
        return result;
    }

    private static BigInteger number(byte[] data, int offset) {
        if (offset < 0 || offset > data.length - 32) throw invalid("truncated ABI word");
        return new BigInteger(1, Arrays.copyOfRange(data, offset, offset + 32));
    }

    private static void unsigned(BigInteger value, int bits) {
        if (value.signum() < 0 || value.bitLength() > bits) throw invalid("uint" + bits + " overflow");
    }

    private static byte[] utf8(String text) {
        try {
            ByteBuffer buffer = StandardCharsets.UTF_8.newEncoder()
                .onMalformedInput(CodingErrorAction.REPORT)
                .onUnmappableCharacter(CodingErrorAction.REPORT).encode(java.nio.CharBuffer.wrap(text));
            byte[] bytes = new byte[buffer.remaining()];
            buffer.get(bytes);
            return bytes;
        } catch (CharacterCodingException error) {
            throw invalid("invalid UTF-8 string");
        }
    }

    private static String signature(Map<String, String> signatures, String name) {
        String signature = signatures.get(name);
        if (signature == null) throw invalid("unknown ABI name: " + name);
        return signature;
    }

    private static String[] types(String signature) {
        String parameters = signature.substring(signature.indexOf('(') + 1, signature.length() - 1);
        return parameters.isEmpty() ? new String[0] : parameters.split(",");
    }

    private static byte[] hash(String signature) {
        byte[] bytes = signature.getBytes(StandardCharsets.US_ASCII);
        var digest = new KeccakDigest(256);
        digest.update(bytes, 0, bytes.length);
        byte[] result = new byte[32];
        digest.doFinal(result, 0);
        return result;
    }

    private static IllegalArgumentException invalid(String message) {
        return new IllegalArgumentException(message);
    }
}
