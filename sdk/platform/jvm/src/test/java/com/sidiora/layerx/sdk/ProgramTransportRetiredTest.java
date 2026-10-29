package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sun.net.httpserver.HttpServer;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.List;
import java.util.concurrent.CompletionException;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.TimeUnit;
import java.util.function.Supplier;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertTrue;

public final class ProgramTransportRetiredTest {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final String SENDER = "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf";

    private record ProgramOperation(String name, String rpcMethod, Supplier<CompletionStage<?>> send) {}
    private record RecordedRequest(String path, String method, JsonNode body) {}

    @Test
    void programOperationsNeverCallProgramsRestIngress() throws Exception {
        List<RecordedRequest> requests = new CopyOnWriteArrayList<>();
        HttpServer recorder = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        recorder.createContext("/", exchange -> {
            try {
                requests.add(new RecordedRequest(exchange.getRequestURI().getPath(),
                    exchange.getRequestMethod(), JSON.readTree(exchange.getRequestBody())));
                exchange.sendResponseHeaders(503, -1);
            } finally {
                exchange.close();
            }
        });
        recorder.start();
        try {
            JsonNode pinned = fixture("sequencer-receipt-v1.json").required("cases").get(0).required("pinnedKey");
            var keys = new LocalVerifier.ReceiptKeys(List.of(new LocalVerifier.ReceiptKey(
                bytes(pinned, "keyId"), bytes(pinned, "publicKey"),
                pinned.required("firstHeight").bigIntegerValue(), pinned.required("lastHeight").bigIntegerValue())));
            URI endpoint = URI.create("http://127.0.0.1:" + recorder.getAddress().getPort() + "/rpc");
            var programs = new ProgramsClient(new ProgramsClient.Programs(endpoint), SENDER,
                BigInteger.ONE, BigInteger.TWO, keys, bytes(pinned, "keyId"));
            List<ProgramOperation> operations = programRoutes(programs);
            assertEquals(List.of("program.discover", "program.interface", "program.simulate", "program.call",
                "program.submit", "program.receipt", "program.activity", "program.deploy", "program.upgrade",
                "program.wind-down.route", "program.wind-down.deprecate", "program.wind-down.tombstone",
                "program.wind-down.exit", "program.sequencer-receipt"),
                operations.stream().map(ProgramOperation::name).toList());
            for (ProgramOperation operation : operations) sendProgram(operation, requests);
            assertEquals(operations.size(), requests.size());
            assertTrue(requests.stream().noneMatch(request -> request.path().startsWith("/v1/programs")));
            assertFalse(Arrays.stream(ProductionTransport.class.getDeclaredMethods())
                .anyMatch(method -> method.getName().equals("callPrograms")));
            assertFalse(Arrays.stream(ProductionTransport.class.getDeclaredClasses())
                .anyMatch(type -> type.getSimpleName().equals("ProgramsCall")));
            assertFalse(Arrays.stream(ProductionClient.class.getDeclaredMethods())
                .anyMatch(method -> method.getName().equals("programs")));
            assertFalse(Arrays.stream(HttpProductionTransport.class.getDeclaredMethods())
                .anyMatch(method -> method.getName().equals("callPrograms") || method.getName().equals("programRequest")));
            assertFalse(Arrays.stream(HttpProductionTransport.class.getDeclaredClasses())
                .anyMatch(type -> type.getSimpleName().equals("ProgramRoute")
                    || type.getSimpleName().equals("ProgramsBearerCredential")));
        } finally {
            recorder.stop(0);
        }
    }

    private static List<ProgramOperation> programRoutes(ProgramsClient programs) throws Exception {
        JsonNode vector = fixture("native-program-call-v4.json");
        NativeProgramCall call = NativeProgramCall.decode(bytes(vector, "payload_hex"));
        BigInteger fee = new BigInteger(vector.required("fee_limit").asText());
        byte[] id = call.programId();
        ObjectNode transaction = JSON.createObjectNode().put("type", "0x2").put("chainId", "0x7d")
            .put("from", SENDER).put("to", ProgramsClient.PROGRAMS_ADDRESS).put("nonce", "0x3")
            .put("maxPriorityFeePerGas", "0x1").put("maxFeePerGas", "0x2").put("gas", "0x5208")
            .put("value", "0x0").put("data", ProgramsClient.Programs.hexData(call.calldata(fee)));
        transaction.putArray("accessList");
        var prepared = new ProgramsClient.ProgramCall(transaction);
        byte[] signed = sign(prepared);
        byte[] hash = ProgramsClient.Programs.transactionHash(signed);
        var deploy = NativeProgramDeploy.decode(bytes(fixture("native-program-deploy-v4.json"), "payload_hex"));
        var upgrade = NativeProgramUpgrade.decode(bytes(fixture("native-program-upgrade-v4.json"), "payload_hex"));
        return List.of(
            new ProgramOperation("program.discover", "eth_call", () -> programs.discover(id)),
            new ProgramOperation("program.interface", "eth_call", () -> programs.interfaceAt(id)),
            new ProgramOperation("program.simulate", "eth_call", () -> programs.simulate(call, fee)),
            new ProgramOperation("program.call", "eth_getTransactionCount", () -> programs.call(call, fee)),
            new ProgramOperation("program.submit", "eth_sendRawTransaction", () -> programs.submit(prepared, signed)),
            new ProgramOperation("program.receipt", "eth_call", () -> programs.receiptByIdempotency(
                new IdempotencyKey("01".repeat(32)), signed)),
            new ProgramOperation("program.activity", "eth_getTransactionByHash", () -> programs.activity(hash)),
            new ProgramOperation("program.deploy", "eth_getTransactionCount", () -> programs.deploy(deploy)),
            new ProgramOperation("program.upgrade", "eth_getTransactionCount", () -> programs.upgrade(upgrade, fee)),
            new ProgramOperation("program.wind-down.route", "eth_getTransactionCount", () -> programs.windDown(
                NativeProgramWindDown.route(id, id, id, id, new byte[0]))),
            new ProgramOperation("program.wind-down.deprecate", "eth_getTransactionCount", () -> programs.windDown(
                NativeProgramWindDown.deprecate(id, id, 1))),
            new ProgramOperation("program.wind-down.tombstone", "eth_getTransactionCount", () -> programs.windDown(
                NativeProgramWindDown.tombstone(id))),
            new ProgramOperation("program.wind-down.exit", "eth_getTransactionCount", () -> programs.windDown(
                NativeProgramWindDown.exit(id, id))),
            new ProgramOperation("program.sequencer-receipt", "pax_sequencerReceipt", () -> programs.sequencerReceipt(signed)));
    }

    private static void sendProgram(ProgramOperation operation, List<RecordedRequest> requests) throws Exception {
        int before = requests.size();
        try {
            operation.send().get().toCompletableFuture().orTimeout(5, TimeUnit.SECONDS).join();
            throw new AssertionError(operation.name() + " accepted a 503 response");
        } catch (CompletionException error) {
            var failure = assertInstanceOf(PlatformSdkException.class, error.getCause(), operation.name());
            assertEquals(PlatformSdkException.Code.TRANSPORT_FAILURE, failure.code(), operation.name());
        }
        assertEquals(before + 1, requests.size(), operation.name());
        RecordedRequest request = requests.get(before);
        assertFalse(request.path().startsWith("/v1/programs"), operation.name());
        assertEquals("/rpc", request.path(), operation.name());
        assertEquals("POST", request.method(), operation.name());
        assertEquals("2.0", request.body().required("jsonrpc").asText(), operation.name());
        assertEquals(operation.rpcMethod(), request.body().required("method").asText(), operation.name());
    }

    private static byte[] sign(ProgramsClient.ProgramCall call) {
        var curve = org.bouncycastle.crypto.ec.CustomNamedCurves.getByName("secp256k1");
        var domain = new org.bouncycastle.crypto.params.ECDomainParameters(curve.getCurve(), curve.getG(), curve.getN());
        var signer = new org.bouncycastle.crypto.signers.ECDSASigner(
            new org.bouncycastle.crypto.signers.HMacDSAKCalculator(new org.bouncycastle.crypto.digests.SHA256Digest()));
        signer.init(true, new org.bouncycastle.crypto.params.ECPrivateKeyParameters(BigInteger.ONE, domain));
        BigInteger[] signature = signer.generateSignature(ProgramsClient.Programs.transactionHash(call.signingPayload()));
        BigInteger lowS = signature[1].min(curve.getN().subtract(signature[1]));
        var fields = new ArrayList<byte[]>();
        ObjectNode transaction = call.transaction();
        for (String field : List.of("chainId", "nonce", "maxPriorityFeePerGas", "maxFeePerGas", "gas")) {
            fields.add(magnitude(ProgramsClient.Programs.quantity(transaction.required(field))));
        }
        fields.add(bytes(transaction, "to"));
        fields.add(new byte[0]);
        fields.add(bytes(transaction, "data"));
        fields.add(new byte[0]);
        for (int parity = 0; parity < 2; parity++) {
            var encoded = new java.io.ByteArrayOutputStream();
            for (int index = 0; index < fields.size(); index++)
                encoded.writeBytes(ProgramsClient.Programs.rlp(fields.get(index), index == 8));
            encoded.writeBytes(ProgramsClient.Programs.rlp(magnitude(BigInteger.valueOf(parity)), false));
            encoded.writeBytes(ProgramsClient.Programs.rlp(magnitude(signature[0]), false));
            encoded.writeBytes(ProgramsClient.Programs.rlp(magnitude(lowS), false));
            byte[] signed = ProgramsClient.Programs.concat(new byte[] {2}, ProgramsClient.Programs.rlp(encoded.toByteArray(), true));
            try {
                ProgramsClient.Programs.bind(call, signed);
                return signed;
            } catch (PlatformSdkException error) {
                assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, error.code());
            }
        }
        throw new AssertionError("neither recovery parity matches the signing key");
    }

    private static byte[] magnitude(BigInteger value) {
        if (value.signum() == 0) return new byte[0];
        byte[] bytes = value.toByteArray();
        return bytes[0] == 0 ? Arrays.copyOfRange(bytes, 1, bytes.length) : bytes;
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
}
