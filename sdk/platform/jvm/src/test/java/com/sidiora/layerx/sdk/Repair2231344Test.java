package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.GeneratedReceiptContract;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.PortableReceipt;
import com.sun.net.httpserver.HttpServer;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.ByteBuffer;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.stream.Collectors;
import java.util.stream.StreamSupport;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

public final class Repair2231344Test {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final BigInteger FEE = BigInteger.valueOf(1000);

    @Test
    void lifecycleAdmissionMatchesSharedGuestAbiContract() throws Exception {
        NativeProgramDeploy seed = deployFixture();
        NativeProgramCall call = NativeProgramCall.decode(bytes(fixture(
            "sdk/platform/conformance/fixtures/native-program-call-v4.json"), "payload_hex"));
        for (int abi = 1; abi <= 5; abi++) {
            assertTrue(GeneratedReceiptContract.supportsProgramGuestAbi(abi));
            assertEquals(abi, GeneratedSchema.AgentModels.ProgramGuestAbi.fromWire(abi).wire());
            NativeProgramDeploy deploy = deployment(seed, abi);
            assertLifecycleTransaction(deploy, abi);
            NativeProgramUpgrade upgrade = new NativeProgramUpgrade(seed.programId(), abi,
                seed.newHash(), seed.newHash(), new byte[0], false, null, seed.wasm());
            assertLifecycleTransaction(upgrade, abi);
            NativeProgramCall versioned = new NativeProgramCall(call.programId(), abi, call.entrypoint(),
                call.calldata(), call.capabilities(), call.accessDeclaration(), call.responseCapacity(), call.resources());
            assertArrayEquals(versioned.encode(), NativeProgramCall.decode(versioned.encode()).encode());
            byte[] signed = ReceiptFixtureTest.signedTransaction(versioned.calldata(FEE));
            assertEquals(abi, NativeProgramCall.decodeSignedActivity(signed).guestAbi());
            assertArrayEquals(ProgramsClient.Programs.transactionHash(signed), versioned.bindSignedActivity(signed));
        }
    }

    @Test
    void abi4AndAbi5DeployAndMonotonicUpgradeTransactionsBindAt101D() throws Exception {
        NativeProgramDeploy seed = deployFixture();
        for (int abi : new int[] {4, 5}) assertLifecycleTransaction(deployment(seed, abi), abi);

        NativeProgramDeploy deployed = deployment(seed, 3);
        assertLifecycleTransaction(deployed, 3);
        byte[] oldHash = deployed.newHash();
        byte[] wasm = deployed.wasm();
        int currentAbi = deployed.guestAbi();
        for (int abi : new int[] {4, 4, 5, 5}) {
            // A valid Wasm custom section makes each upgrade replace the previous code hash.
            wasm = ProgramsClient.Programs.concat(wasm, new byte[] {0, 2, 1, (byte) abi});
            NativeProgramUpgrade upgrade = new NativeProgramUpgrade(deployed.programId(), abi,
                oldHash, NativeProgramLifecycle.hash(wasm), new byte[0], false, null, wasm);
            assertTrue(abi >= currentAbi);
            assertArrayEquals(oldHash, upgrade.oldHash());
            assertFalse(Arrays.equals(oldHash, upgrade.newHash()));
            assertLifecycleTransaction(upgrade, abi);
            oldHash = upgrade.newHash();
            currentAbi = upgrade.guestAbi();
        }
    }

    @Test
    void unsupportedGuestAbisAreRefusedByConstructorsAndWireDecoders() throws Exception {
        NativeProgramDeploy deploy = deployFixture();
        NativeProgramUpgrade upgrade = NativeProgramUpgrade.decode(bytes(fixture(
            "sdk/platform/conformance/fixtures/native-program-upgrade-v4.json"), "payload_hex"));
        NativeProgramCall call = NativeProgramCall.decode(bytes(fixture(
            "sdk/platform/conformance/fixtures/native-program-call-v4.json"), "payload_hex"));
        for (int abi : new int[] {-1, Integer.MIN_VALUE, 0, 6, 255, 65535, 65536, 65537, Integer.MAX_VALUE}) {
            assertFalse(GeneratedReceiptContract.supportsProgramGuestAbi(abi));
            assertThrows(IllegalArgumentException.class, () -> deployment(deploy, abi));
            assertThrows(IllegalArgumentException.class, () -> new NativeProgramUpgrade(upgrade.programId(), abi,
                upgrade.oldHash(), upgrade.newHash(), upgrade.migrationHook(), upgrade.clearInterface(),
                upgrade.programInterface(), upgrade.wasm()));
            NativeProgramCall invalidCall = new NativeProgramCall(call.programId(), abi, call.entrypoint(),
                call.calldata(), call.capabilities(), call.accessDeclaration(), call.responseCapacity(), call.resources());
            assertThrows(IllegalArgumentException.class, invalidCall::encode);
            if (abi >= 0 && abi <= 65535) {
                for (NativeProgramLifecycle operation : List.of(deploy, upgrade)) {
                    byte[] invalid = operation.encode();
                    ByteBuffer.wrap(invalid).putShort(32, (short) abi);
                    assertThrows(IllegalArgumentException.class,
                        () -> NativeProgramLifecycle.decode(operation.ordinal(), invalid));
                }
                byte[] invalid = call.encode();
                ByteBuffer.wrap(invalid).putShort(32, (short) abi);
                assertThrows(IllegalArgumentException.class, () -> NativeProgramCall.decode(invalid));
            }
        }
    }

    @Test
    void newGuestAbisPreserveCodeIntegrityAndSignedTransactionBinding() throws Exception {
        NativeProgramDeploy seed = deployFixture();
        for (int abi : new int[] {4, 5}) {
            NativeProgramDeploy deploy = deployment(seed, abi);
            NativeProgramUpgrade upgrade = new NativeProgramUpgrade(seed.programId(), abi,
                seed.newHash(), seed.newHash(), new byte[0], false, null, seed.wasm());
            for (NativeProgramLifecycle operation : List.of(deploy, upgrade)) {
                for (int offset : new int[] {35, 68, operation.encode().length - 1}) {
                    byte[] corrupted = operation.encode();
                    corrupted[offset] ^= 1;
                    assertThrows(IllegalArgumentException.class,
                        () -> NativeProgramLifecycle.decode(operation.ordinal(), corrupted));
                }
                byte[] truncated = Arrays.copyOf(operation.encode(), operation.encode().length - 1);
                assertThrows(IllegalArgumentException.class,
                    () -> NativeProgramLifecycle.decode(operation.ordinal(), truncated));
                byte[] calldata = NativeProgramLifecycleRequest.calldata(operation, FEE);
                byte[] altered = calldata.clone();
                altered[67] = (byte) (abi == 4 ? 5 : 4);
                byte[] signedOtherAbi = ReceiptFixtureTest.signedTransaction(altered);
                assertThrows(IllegalArgumentException.class,
                    () -> new NativeProgramLifecycleRequest(operation, signedOtherAbi));
            }
            for (byte[] program : new byte[][] {new byte[32], new byte[31]}) {
                assertThrows(IllegalArgumentException.class,
                    () -> NativeProgramLifecycle.code(program, abi, seed.newHash(), seed.wasm()));
            }
            for (byte[] invalidWasm : new byte[][] {null, new byte[7], new byte[8], new byte[1048577]}) {
                assertThrows(IllegalArgumentException.class,
                    () -> NativeProgramLifecycle.code(seed.programId(), abi, seed.newHash(), invalidWasm));
            }
        }
    }

    @Test
    void recordedNativeLifecycleEncodersRoundTripUnchanged() throws Exception {
        for (String version : List.of("v3", "v4")) {
            for (String name : List.of("deploy", "upgrade")) {
                JsonNode vector = fixture("sdk/platform/conformance/fixtures/native-program-" + name + "-" + version + ".json");
                byte[] payload = bytes(vector, "payload_hex");
                NativeProgramLifecycle operation = NativeProgramLifecycle.decode(vector.required("ordinal").intValue(), payload);
                assertArrayEquals(payload, operation.encode());
                assertArrayEquals(bytes(vector.required("evm"), "calldata"),
                    NativeProgramLifecycleRequest.calldata(operation, FEE));
                assertArrayEquals(bytes(vector, "idempotency_key_hex"), NativeProgramLifecycleRequest.bind(
                    operation.ordinal(), payload, bytes(vector, "signed_activity_hex")));
                assertLifecycleTransaction(operation, NativeProgramLifecycle.u16(payload, 32));
            }
        }
    }

    @Test
    void recordedProgramReceiptTraversesHttpVerifierAndPortableDecoder() throws Exception {
        JsonNode vectors = fixture("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");
        JsonNode vector = StreamSupport.stream(vectors.required("cases").spliterator(), false)
            .filter(value -> "V1-layerxprograms-call".equals(value.required("id").textValue()))
            .findFirst().orElseThrow();
        JsonNode node = vector.required("receiptJson");
        assertEquals(ProgramsClient.PROGRAMS_ADDRESS, node.required("to").textValue());
        byte[] raw = bytes(vector, "rawTransaction");
        NativeProgramCall call = NativeProgramCall.decodeSignedActivity(raw);
        assertTrue(GeneratedReceiptContract.supportsProgramGuestAbi(call.guestAbi()));
        assertArrayEquals(bytes(node, "txHash"), call.bindSignedActivity(raw));
        JsonNode pin = vector.required("pinnedKey");
        var key = new LocalVerifier.ReceiptKey(bytes(pin, "keyId"), bytes(pin, "publicKey"),
            pin.required("firstHeight").bigIntegerValue(), pin.required("lastHeight").bigIntegerValue());
        var keys = new LocalVerifier.ReceiptKeys(List.of(key));
        byte[] portable;
        try (var archive = new RecordedReceiptArchive(node);
                var connection = new ProgramsClient.Programs(archive.uri())) {
            var client = new ProgramsClient(connection, node.required("from").textValue(),
                BigInteger.ONE, BigInteger.TWO, keys, key.keyId());
            var receipt = client.sequencerReceipt(raw).toCompletableFuture().get(10, TimeUnit.SECONDS);
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, receipt.level());
            assertEquals(1, receipt.status());
            assertArrayEquals(bytes(vector, "receiptDigest"), receipt.receiptDigest());
            assertArrayEquals(bytes(vector, "signedReceipt"), receipt.canonicalBytes());
            JsonNode delivered = connection.request("pax_sequencerReceipt", JSON.createArrayNode()
                .add(node.required("txHash"))).toCompletableFuture().get(10, TimeUnit.SECONDS);
            assertEquals(node, delivered);
            portable = PortableReceipt.export(raw, JSON.writeValueAsBytes(delivered), key);
            assertEquals(2, archive.reads.get());
        }
        var offline = PortableReceipt.verify(portable, keys);
        assertEquals("receipted", offline.verificationLevel());
        assertEquals(1, offline.status());
        assertArrayEquals(bytes(vector, "receiptDigest"), offline.receipted().receiptDigest());
        assertArrayEquals(portable, PortableReceipt.export(offline));
        byte[] altered = raw.clone();
        altered[altered.length - 1] ^= 1;
        assertThrows(PlatformSdkException.class,
            () -> ProgramsClient.verifyReceipt(bytes(vector, "signedReceipt"), altered, keys, node));
    }

    private static NativeProgramDeploy deployFixture() throws Exception {
        return NativeProgramDeploy.decode(bytes(fixture(
            "sdk/platform/conformance/fixtures/native-program-deploy-v4.json"), "payload_hex"));
    }

    private static NativeProgramDeploy deployment(NativeProgramDeploy seed, int abi) {
        return new NativeProgramDeploy(seed.programId(), abi, seed.policy(), seed.authority(),
            seed.newHash(), null, seed.wasm());
    }

    private static void assertLifecycleTransaction(NativeProgramLifecycle operation, int abi) throws Exception {
        byte[] payload = operation.encode();
        assertEquals(abi, NativeProgramLifecycle.u16(payload, 32));
        NativeProgramLifecycle decoded = NativeProgramLifecycle.decode(operation.ordinal(), payload);
        assertArrayEquals(payload, decoded.encode());
        byte[] calldata = NativeProgramLifecycleRequest.calldata(decoded, FEE);
        assertPrecompileArguments(decoded, abi, calldata);
        var transaction = ReceiptFixtureTest.transaction(calldata);
        assertEquals(ProgramsClient.PROGRAMS_ADDRESS, transaction.transaction().required("to").textValue());
        byte[] signed = ReceiptFixtureTest.signedTransaction(calldata);
        ProgramsClient.Programs.bind(transaction, signed);
        assertArrayEquals(calldata, ProgramsClient.Programs.transactionData(signed));
        var request = new NativeProgramLifecycleRequest(decoded, signed, new IdempotencyKey("44".repeat(32)));
        assertEquals(operation.ordinal(), request.ordinal());
        assertArrayEquals(payload, request.payload());
        assertArrayEquals(signed, request.signedTransaction());
        assertArrayEquals(ProgramsClient.Programs.transactionHash(signed), request.transactionHash());
    }

    private static void assertPrecompileArguments(NativeProgramLifecycle operation, int abi, byte[] data) throws Exception {
        boolean upgrading = operation instanceof NativeProgramUpgrade;
        String name = upgrading ? "upgrade" : "deploy";
        JsonNode method = StreamSupport.stream(fixture("precompiles/layerxprograms/abi.json").spliterator(), false)
            .filter(value -> "function".equals(value.path("type").textValue()) && name.equals(value.path("name").textValue()))
            .findFirst().orElseThrow();
        String types = StreamSupport.stream(method.required("inputs").spliterator(), false)
            .map(value -> value.required("type").textValue()).collect(Collectors.joining(","));
        assertArrayEquals(HexFormat.of().parseHex(ProgramsClient.Programs.selector(name + "(" + types + ")")),
            Arrays.copyOf(data, 4));
        assertEquals("uint16", method.required("inputs").required(1).required("type").textValue());
        assertEquals(BigInteger.valueOf(abi), ProgramsClient.Programs.number(data, 36, 16));
        if (operation instanceof NativeProgramDeploy deploy) {
            assertArrayEquals(deploy.programId(), Arrays.copyOfRange(data, 4, 36));
            assertEquals(BigInteger.valueOf(deploy.policy()), ProgramsClient.Programs.number(data, 68, 8));
            assertArrayEquals(deploy.authority(), Arrays.copyOfRange(data, 100, 132));
            assertArrayEquals(deploy.newHash(), Arrays.copyOfRange(data, 132, 164));
            assertArrayEquals(deploy.programInterface() == null ? new byte[0] : deploy.programInterface(),
                ProgramsClient.Programs.dynamic(data, 4, 5));
            assertArrayEquals(deploy.wasm(), ProgramsClient.Programs.dynamic(data, 4, 6));
        } else if (operation instanceof NativeProgramUpgrade upgrade) {
            assertArrayEquals(upgrade.programId(), Arrays.copyOfRange(data, 4, 36));
            int flags = (upgrade.migrationHook().length > 0 ? 1 : 0) | (upgrade.clearInterface() ? 2 : 0);
            assertEquals(BigInteger.valueOf(flags), ProgramsClient.Programs.number(data, 68, 8));
            assertArrayEquals(upgrade.oldHash(), Arrays.copyOfRange(data, 100, 132));
            assertArrayEquals(upgrade.newHash(), Arrays.copyOfRange(data, 132, 164));
            assertArrayEquals(upgrade.migrationHook(), ProgramsClient.Programs.dynamic(data, 4, 5));
            assertArrayEquals(upgrade.programInterface() == null ? new byte[0] : upgrade.programInterface(),
                ProgramsClient.Programs.dynamic(data, 4, 6));
            assertArrayEquals(upgrade.wasm(), ProgramsClient.Programs.dynamic(data, 4, 7));
            assertEquals(FEE, ProgramsClient.Programs.number(data, 260, 128));
        } else {
            fail("Expected a deploy or upgrade operation");
        }
    }

    private static JsonNode fixture(String name) throws Exception {
        Path directory = Path.of("").toAbsolutePath();
        while (directory != null) {
            Path path = directory.resolve(name);
            if (Files.isRegularFile(path)) return JSON.readTree(Files.readAllBytes(path));
            directory = directory.getParent();
        }
        throw new IllegalStateException("Missing recorded fixture: " + name);
    }

    private static byte[] bytes(JsonNode value, String field) {
        String hex = value.required(field).textValue();
        return HexFormat.of().parseHex(hex.startsWith("0x") ? hex.substring(2) : hex);
    }

    private static final class RecordedReceiptArchive implements AutoCloseable {
        private final HttpServer server;
        private final AtomicInteger reads = new AtomicInteger();

        private RecordedReceiptArchive(JsonNode recorded) throws Exception {
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", exchange -> {
                try (exchange) {
                    JsonNode request = JSON.readTree(exchange.getRequestBody());
                    if (!"pax_sequencerReceipt".equals(request.path("method").textValue())
                            || !request.path("params").isArray() || request.path("params").size() != 1
                            || !recorded.required("txHash").equals(request.path("params").path(0))) {
                        exchange.sendResponseHeaders(400, -1);
                        return;
                    }
                    ObjectNode response = JSON.createObjectNode().put("jsonrpc", "2.0");
                    response.set("id", request.required("id"));
                    response.set("result", recorded);
                    byte[] body = JSON.writeValueAsBytes(response);
                    reads.incrementAndGet();
                    exchange.getResponseHeaders().set("Content-Type", "application/json");
                    exchange.sendResponseHeaders(200, body.length);
                    exchange.getResponseBody().write(body);
                }
            });
            server.start();
        }

        private URI uri() { return URI.create("http://127.0.0.1:" + server.getAddress().getPort()); }
        @Override public void close() { server.stop(0); }
    }
}
