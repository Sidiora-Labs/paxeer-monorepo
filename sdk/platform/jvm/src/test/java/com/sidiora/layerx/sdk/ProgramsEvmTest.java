package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sun.net.httpserver.HttpServer;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.ByteBuffer;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.List;
import java.util.concurrent.CopyOnWriteArrayList;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

public final class ProgramsEvmTest {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final String SENDER = "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf";

    @Test
    void programCallBuildsLayerxprogramsCallTransaction() throws Exception {
        JsonNode vector = fixture("native-program-call-v4.json");
        NativeProgramCall nativeCall = NativeProgramCall.decode(bytes(vector, "payload_hex"));
        try (RpcFixture endpoint = new RpcFixture()) {
            ProgramsClient programs = client(endpoint);
            var call = programs.call(nativeCall, new BigInteger(vector.required("fee_limit").asText()))
                .toCompletableFuture().join();
            assertTransaction(call, vector, endpoint.requests);
            assertArrayEquals(nativeCall.encode(), NativeProgramCall.decodeCalldata(bytes(vector.required("evm"), "calldata")).encode());
            byte[] signed = sign(call);
            var submitted = programs.submit(call, signed).toCompletableFuture().join();
            assertTrue(submitted.pending());
            assertArrayEquals(ProgramsClient.Programs.transactionHash(signed), submitted.transactionHash());
            assertArrayEquals(submitted.transactionHash(), nativeCall.bindSignedActivity(signed));
            assertArrayEquals(nativeCall.encode(), NativeProgramCall.decodeSignedActivity(signed).encode());
            assertEquals("eth_sendRawTransaction", endpoint.requests.getLast().required("method").asText());
            assertEquals("0x" + HexFormat.of().formatHex(signed), endpoint.requests.getLast().required("params").get(0).asText());
            assertEquals("0x" + HexFormat.of().formatHex(submitted.transactionHash()),
                programs.activity(submitted.transactionHash()).toCompletableFuture().join().required("hash").asText());
            var pending = assertThrows(java.util.concurrent.CompletionException.class,
                () -> programs.receipt(submitted).toCompletableFuture().join());
            assertEquals(PlatformSdkException.Code.UNKNOWN_OUTCOME, ((PlatformSdkException) pending.getCause()).code());
            var altered = call.transaction(); altered.put("nonce", "0x4");
            assertThrows(PlatformSdkException.class, () -> programs.submit(new ProgramsClient.ProgramCall(altered), signed));
            ObjectNode replacement = call.transaction();
            replacement.put("maxFeePerGas", "0x3");
            var replacementCall = new ProgramsClient.ProgramCall(replacement);
            byte[] replacementSigned = sign(replacementCall);
            int sentBeforeReplacement = endpoint.requests.size();
            assertThrows(PlatformSdkException.class, () -> programs.submit(replacementCall, replacementSigned));
            assertEquals(sentBeforeReplacement, endpoint.requests.size());
            assertThrows(PlatformSdkException.class, () -> programs.call(nativeCall, BigInteger.ONE.shiftLeft(128)));
            byte[] changed = nativeCall.calldata(); changed[0] ^= 1;
            assertArrayEquals(bytes(vector, "payload_hex"), nativeCall.encode());
            ObjectNode changedTransaction = call.transaction(); changedTransaction.put("to", SENDER);
            assertEquals(ProgramsClient.PROGRAMS_ADDRESS, call.transaction().required("to").asText());

            JsonNode receiptVector = fixture("sequencer-receipt-v1.json").required("cases").get(0);
            var keys = keys(receiptVector);
            var verified = ProgramsClient.verifyReceipt(bytes(receiptVector, "signedReceipt"),
                bytes(receiptVector, "rawTransaction"), keys, receiptVector.required("receiptJson"));
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, verified.level());
            byte[] mutated = bytes(receiptVector, "rawTransaction"); mutated[mutated.length - 1] ^= 1;
            assertThrows(PlatformSdkException.class, () -> ProgramsClient.verifyReceipt(
                bytes(receiptVector, "signedReceipt"), mutated, keys, receiptVector.required("receiptJson")));
        }
    }

    @Test
    void programSimulateUsesPendingEthCall() throws Exception {
        JsonNode vector = fixture("native-program-call-v4.json");
        var nativeCall = NativeProgramCall.decode(bytes(vector, "payload_hex"));
        try (RpcFixture endpoint = new RpcFixture()) {
            var result = client(endpoint).simulate(nativeCall, new BigInteger(vector.required("fee_limit").asText()))
                .toCompletableFuture().join();
            assertEquals(0, result.terminalResult());
            assertEquals(1, result.terminalKind());
            assertArrayEquals(HexFormat.of().parseHex("61ff"), result.response());
            assertFalse(result.committed());
            assertEquals(1, endpoint.requests.size());
            JsonNode request = endpoint.requests.getFirst();
            assertEquals("eth_call", request.required("method").asText());
            assertEquals("pending", request.required("params").get(1).asText());
            JsonNode transaction = request.required("params").get(0);
            assertEquals(ProgramsClient.PROGRAMS_ADDRESS, transaction.required("to").asText());
            assertEquals(SENDER, transaction.required("from").asText());
            String expected = "0x3044bd56" + vector.required("evm").required("calldata").asText().substring(10);
            assertEquals(expected, transaction.required("data").asText());
            assertEquals(List.of("/rpc"), endpoint.paths);
            assertTrue(endpoint.requests.stream().noneMatch(value -> value.path("method").asText().startsWith("program.")));
        }
    }

    @Test
    void programDeployBuildsLayerxprogramsDeployTransaction() throws Exception {
        JsonNode vector = fixture("native-program-deploy-v4.json");
        NativeProgramDeploy deploy = NativeProgramDeploy.decode(bytes(vector, "payload_hex"));
        try (RpcFixture endpoint = new RpcFixture()) {
            var transaction = client(endpoint).deploy(deploy).toCompletableFuture().join();
            assertTransaction(transaction, vector, endpoint.requests);
            assertArrayEquals(bytes(vector.required("evm"), "calldata"),
                NativeProgramLifecycleRequest.calldata(deploy, BigInteger.ZERO));
            assertEquals(List.of("/rpc", "/rpc"), endpoint.paths);
            assertThrows(IllegalArgumentException.class,
                () -> new NativeProgramLifecycleRequest(deploy, bytes(vector, "signed_activity_hex")));
            byte[] signed = sign(transaction);
            var request = new NativeProgramLifecycleRequest(deploy, signed);
            assertArrayEquals(ProgramsClient.Programs.transactionHash(signed), request.transactionHash());
            JsonNode upgradeVector = fixture("native-program-upgrade-v4.json");
            var upgrade = NativeProgramUpgrade.decode(bytes(upgradeVector, "payload_hex"));
            assertArrayEquals(bytes(upgradeVector.required("evm"), "calldata"),
                NativeProgramLifecycleRequest.calldata(upgrade, BigInteger.valueOf(1000)));
        }
    }

    @Test
    void programDiscoverReadsWindDownLifecycle() throws Exception {
        record Scenario(boolean active, int status, int length, ProgramsClient.Lifecycle lifecycle) {}
        for (Scenario scenario : List.of(
            new Scenario(true, 1, 192, ProgramsClient.Lifecycle.ACTIVE),
            new Scenario(false, 2, 192, ProgramsClient.Lifecycle.DEPRECATED),
            new Scenario(false, 3, 192, ProgramsClient.Lifecycle.TOMBSTONED),
            new Scenario(false, 1, 192, null), new Scenario(false, 0, 192, null),
            new Scenario(false, 4, 192, null), new Scenario(false, 258, 192, null),
            new Scenario(false, 2, 0, null), new Scenario(false, 2, 31, null),
            new Scenario(false, 2, 32, null), new Scenario(false, 3, 191, null))) {
            byte[] program = new byte[32]; program[0] = 0x42;
            byte[] head = new byte[192];
            head[63] = 0x11; head[95] = 0x22; head[127] = 1; head[159] = 7;
            head[191] = (byte) (scenario.active() ? 1 : 0);
            byte[] windDown = new byte[192];
            ByteBuffer.wrap(windDown).putInt(28, scenario.status());
            Arrays.fill(windDown, 32, 64, (byte) 0x33);
            for (int i = 2; i < 6; i++) ByteBuffer.wrap(windDown).putInt(i * 32 + 28, 9 - i);
            String getProgram = "0xb1ffb3d4" + HexFormat.of().formatHex(program);
            String getWindDown = "0x98e4e41d" + HexFormat.of().formatHex(program);
            try (RpcFixture endpoint = new RpcFixture(request -> {
                String calldata = request.required("params").get(0).required("data").asText();
                if (getProgram.equals(calldata)) return "0x" + HexFormat.of().formatHex(head);
                if (getWindDown.equals(calldata)) return "0x" + HexFormat.of().formatHex(Arrays.copyOf(windDown, scenario.length()));
                throw new IllegalArgumentException("Unexpected calldata: " + calldata);
            })) {
                var discovery = client(endpoint).discover(program).toCompletableFuture();
                if (scenario.lifecycle() == null) {
                    var failure = assertThrows(java.util.concurrent.CompletionException.class, discovery::join);
                    assertEquals(PlatformSdkException.Code.DECODE_FAILURE,
                        assertInstanceOf(PlatformSdkException.class, failure.getCause()).code());
                } else {
                    var result = discovery.join();
                    assertEquals(scenario.lifecycle(), result.lifecycle());
                    assertArrayEquals(program, result.programId());
                    assertEquals(7, result.version()); assertEquals(1, result.abiVersion());
                    assertArrayEquals(Arrays.copyOfRange(head, 64, 96), result.codeHash());
                }
                assertEquals(scenario.active() ? 1 : 2, endpoint.requests.size());
                for (int i = 0; i < endpoint.requests.size(); i++) {
                    JsonNode request = endpoint.requests.get(i);
                    assertEquals("eth_call", request.required("method").asText());
                    assertEquals("2.0", request.required("jsonrpc").asText());
                    JsonNode parameters = request.required("params");
                    assertEquals(2, parameters.size());
                    assertEquals("pending", parameters.get(1).asText());
                    assertEquals(ProgramsClient.PROGRAMS_ADDRESS, parameters.get(0).required("to").asText());
                    assertEquals(i == 0 ? getProgram : getWindDown, parameters.get(0).required("data").asText());
                }
            }
        }
    }

    private static void assertTransaction(ProgramsClient.ProgramCall call, JsonNode vector, List<JsonNode> requests) {
        ObjectNode tx = call.transaction();
        assertEquals("0x2", tx.required("type").asText());
        assertEquals("0x7d", tx.required("chainId").asText());
        assertEquals(125, vector.required("evm").required("chainId").intValue());
        assertEquals(ProgramsClient.PROGRAMS_ADDRESS, tx.required("to").asText());
        assertEquals("0x3", tx.required("nonce").asText());
        assertEquals("0x5208", tx.required("gas").asText());
        assertEquals("0x1", tx.required("maxPriorityFeePerGas").asText());
        assertEquals("0x2", tx.required("maxFeePerGas").asText());
        assertEquals("0x0", tx.required("value").asText());
        assertTrue(tx.required("accessList").isEmpty());
        assertFalse(tx.has("r")); assertFalse(tx.has("s")); assertFalse(tx.has("yParity"));
        assertEquals(vector.required("evm").required("calldata").asText(), tx.required("data").asText());
        assertEquals(2, requests.size());
        assertEquals("eth_getTransactionCount", requests.get(0).required("method").asText());
        assertEquals(SENDER, requests.get(0).required("params").get(0).asText());
        assertEquals("pending", requests.get(0).required("params").get(1).asText());
        assertEquals("eth_estimateGas", requests.get(1).required("method").asText());
        assertEquals("pending", requests.get(1).required("params").get(1).asText());
        ObjectNode estimate = tx.deepCopy(); estimate.remove("gas");
        assertEquals(estimate, requests.get(1).required("params").get(0));
        List<byte[]> fields = transactionFields(call.unsignedTransaction());
        assertEquals(12, fields.size());
        assertEquals(BigInteger.valueOf(125), new BigInteger(1, fields.get(0)));
        assertEquals(BigInteger.valueOf(3), new BigInteger(1, fields.get(1)));
        assertEquals(BigInteger.valueOf(21_000), new BigInteger(1, fields.get(4)));
        assertArrayEquals(HexFormat.of().parseHex(ProgramsClient.PROGRAMS_ADDRESS.substring(2)), fields.get(5));
        assertArrayEquals(bytes(vector.required("evm"), "calldata"), fields.get(7));
        for (int index : new int[] {6, 8, 9, 10, 11}) assertEquals(0, fields.get(index).length);
        assertEquals(9, transactionFields(call.signingPayload()).size());
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
    private static ProgramsClient client(RpcFixture endpoint) throws Exception {
        JsonNode vector = fixture("sequencer-receipt-v1.json").required("cases").get(0);
        return new ProgramsClient(new ProgramsClient.Programs(endpoint.uri()), SENDER,
            BigInteger.ONE, BigInteger.TWO, keys(vector), bytes(vector.required("pinnedKey"), "keyId"));
    }
    private static LocalVerifier.ReceiptKeys keys(JsonNode vector) {
        JsonNode key = vector.required("pinnedKey");
        return new LocalVerifier.ReceiptKeys(List.of(new LocalVerifier.ReceiptKey(bytes(key, "keyId"), bytes(key, "publicKey"),
            key.required("firstHeight").bigIntegerValue(), key.required("lastHeight").bigIntegerValue())));
    }
    private static JsonNode fixture(String name) throws Exception {
        Path root = Path.of("").toAbsolutePath();
        while (root != null) {
            Path path = root.resolve("sdk/platform/conformance/fixtures/" + name);
            if (Files.isRegularFile(path)) return JSON.readTree(Files.readString(path));
            root = root.getParent();
        }
        throw new IllegalStateException("fixture not found: " + name);
    }
    private static byte[] bytes(JsonNode node, String field) {
        String value = node.required(field).asText();
        return HexFormat.of().parseHex(value.startsWith("0x") ? value.substring(2) : value);
    }

    private static final class RpcFixture implements AutoCloseable {
        private final HttpServer server;
        private final List<JsonNode> requests = new CopyOnWriteArrayList<>();
        private final List<String> paths = new CopyOnWriteArrayList<>();
        RpcFixture() throws Exception {
            this(request -> "0x" + "00".repeat(32) + "00".repeat(31) + "01"
                + "00".repeat(31) + "60" + "00".repeat(31) + "02" + "61ff" + "00".repeat(30));
        }
        RpcFixture(java.util.function.Function<JsonNode, String> ethCallResponse) throws Exception {
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", exchange -> {
                try {
                    paths.add(exchange.getRequestURI().getPath());
                    if (!"/rpc".equals(exchange.getRequestURI().getPath()) || !"POST".equals(exchange.getRequestMethod())) {
                        exchange.sendResponseHeaders(404, -1); return;
                    }
                    JsonNode request = JSON.readTree(exchange.getRequestBody()); requests.add(request);
                    ObjectNode response = JSON.createObjectNode().put("jsonrpc", "2.0"); response.set("id", request.required("id"));
                    switch (request.required("method").asText()) {
                        case "eth_getTransactionCount" -> response.put("result", "0x3");
                        case "eth_estimateGas" -> response.put("result", "0x5208");
                        case "eth_sendRawTransaction" -> response.put("result", "0x" + HexFormat.of().formatHex(
                            ProgramsClient.Programs.transactionHash(HexFormat.of().parseHex(request.required("params").get(0).asText().substring(2)))));
                        case "eth_getTransactionByHash" -> response.putObject("result").put("hash", request.required("params").get(0).asText());
                        case "pax_sequencerReceipt" -> response.putObject("result").put("rung", "submitted")
                            .put("txHash", request.required("params").get(0).asText()).put("nonce", "0x3");
                        case "eth_call" -> response.put("result", ethCallResponse.apply(request));
                        default -> response.putObject("error").put("code", -32601).put("message", "Method not found");
                    }
                    byte[] body = JSON.writeValueAsBytes(response);
                    exchange.getResponseHeaders().set("Content-Type", "application/json");
                    exchange.sendResponseHeaders(200, body.length); exchange.getResponseBody().write(body);
                } finally { exchange.close(); }
            });
            server.start();
        }
        URI uri() { return URI.create("http://127.0.0.1:" + server.getAddress().getPort() + "/rpc"); }
        @Override public void close() { server.stop(0); }
    }
}
