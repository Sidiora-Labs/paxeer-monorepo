package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.PortableReceipt;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpHandler;
import com.sun.net.httpserver.HttpServer;
import com.sun.net.httpserver.HttpsConfigurator;
import com.sun.net.httpserver.HttpsServer;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.net.URI;
import java.net.http.HttpClient;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyPairGenerator;
import java.security.KeyStore;
import java.security.Signature;
import java.security.cert.CertificateFactory;
import java.time.Duration;
import java.time.Instant;
import java.util.Arrays;
import java.util.Date;
import java.util.HexFormat;
import java.util.List;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import org.bouncycastle.asn1.ASN1EncodableVector;
import org.bouncycastle.asn1.ASN1Integer;
import org.bouncycastle.asn1.DERBitString;
import org.bouncycastle.asn1.DERNull;
import org.bouncycastle.asn1.DERSequence;
import org.bouncycastle.asn1.pkcs.PKCSObjectIdentifiers;
import org.bouncycastle.asn1.x500.X500Name;
import org.bouncycastle.asn1.x509.AlgorithmIdentifier;
import org.bouncycastle.asn1.x509.Extension;
import org.bouncycastle.asn1.x509.ExtensionsGenerator;
import org.bouncycastle.asn1.x509.GeneralName;
import org.bouncycastle.asn1.x509.GeneralNames;
import org.bouncycastle.asn1.x509.SubjectPublicKeyInfo;
import org.bouncycastle.asn1.x509.Time;
import org.bouncycastle.asn1.x509.V3TBSCertificateGenerator;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;
import static org.junit.jupiter.api.Assertions.*;

@Timeout(60)
public final class Repair22331389Test {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final int LIMIT = 524288;
    private static final BigInteger FEE = BigInteger.valueOf(1000);
    private static final Duration DEADLINE = Duration.ofMillis(750);
    private static final IdempotencyKey KEY = new IdempotencyKey("44".repeat(32));

    @Test
    void programReadMetadataFollowsRetainedTransactions() throws Exception {
        JsonNode vector = programReceipt();
        byte[] raw = bytes(vector, "rawTransaction");
        byte[] hash = ProgramsClient.Programs.transactionHash(raw);
        NativeProgramCall call = NativeProgramCall.decodeSignedActivity(raw);
        BigInteger fee = ProgramsClient.Programs.number(ProgramsClient.Programs.transactionData(raw), 452, 128);
        List<String> methods = new CopyOnWriteArrayList<>();
        try (var server = new Loopback(exchange -> {
            JsonNode request = JSON.readTree(exchange.getRequestBody());
            String method = request.required("method").textValue();
            methods.add(method);
            assertEquals("0x" + HexFormat.of().formatHex(hash), request.required("params").required(0).textValue());
            ObjectNode result;
            if ("pax_sequencerReceipt".equals(method)) result = vector.required("receiptJson").deepCopy();
            else {
                assertEquals("eth_getTransactionByHash", method);
                result = JSON.createObjectNode().put("hash", "0x" + HexFormat.of().formatHex(hash))
                    .put("input", "0x" + HexFormat.of().formatHex(ProgramsClient.Programs.transactionData(raw)));
            }
            rpcReply(exchange, request, result);
        }); var connection = new ProgramsClient.Programs(server.uri())) {
            ProgramsClient client = client(connection, vector);
            var submission = await(client.receipt(new ProgramsClient.Call(call, fee, raw), KEY));
            assertEquals(ProgramsClient.SubmissionState.EXECUTED, submission.state());
            assertArrayEquals(raw, submission.retainedSignedTransaction());
            assertArrayEquals(bytes(vector, "receiptDigest"), submission.execution().receiptDigest());
            assertArrayEquals(submission.execution().canonicalBytes(), await(client.receipt(KEY, hash)).execution().canonicalBytes());
            assertEquals("0x" + HexFormat.of().formatHex(hash), await(client.activity(hash)).required("hash").textValue());
            assertEquals(List.of("pax_sequencerReceipt", "pax_sequencerReceipt", "eth_getTransactionByHash"), methods);
            var receipt = GeneratedSchema.programTransport(GeneratedSchema.AgentOperations.PROGRAM_RECEIPT);
            var activity = GeneratedSchema.programTransport(GeneratedSchema.AgentOperations.PROGRAM_ACTIVITY);
            assertEquals(methods.getFirst(), receipt.rpcMethod());
            assertEquals(methods.getLast(), activity.rpcMethod());
            assertEquals("", receipt.to());
            assertEquals("", activity.to());
            assertEquals(List.of("pax_sequencerReceipt(retainedTransactionHash)"), receipt.calls());
            assertEquals(List.of("eth_getTransactionByHash(retainedTransactionHash)"), activity.calls());
            assertEquals(List.of("unverified", "receipt-verified", "checkpoint-finalised"),
                Arrays.stream(GeneratedSchema.HumanModels.HumanVerificationLevel.values()).map(value -> value.wire()).toList());
            byte[] portable = PortableReceipt.export(raw, JSON.writeValueAsBytes(vector.required("receiptJson")), key(vector));
            var offline = PortableReceipt.verify(portable, keys(vector));
            assertEquals("receipted", offline.verificationLevel());
            assertArrayEquals(bytes(vector, "receiptDigest"), offline.receipted().receiptDigest());
            assertArrayEquals(portable, PortableReceipt.export(offline));
            byte[] corrupted = raw.clone(); corrupted[corrupted.length - 1] ^= 1;
            assertThrows(PlatformSdkException.class, () -> ProgramsClient.verifyReceipt(
                bytes(vector, "signedReceipt"), corrupted, keys(vector), vector.required("receiptJson")));
        }
    }

    @Test
    void recordedPayloadsPreservePrecompileWireShapes() throws Exception {
        for (String version : List.of("v3", "v4")) {
            JsonNode call = fixture("native-program-call-" + version + ".json");
            assertArrayEquals(bytes(call.required("evm"), "calldata"),
                NativeProgramCall.decode(bytes(call, "payload_hex")).calldata(new BigInteger(call.required("fee_limit").textValue())));
            for (String name : List.of("deploy", "upgrade")) {
                JsonNode recorded = fixture("native-program-" + name + "-" + version + ".json");
                var operation = NativeProgramLifecycle.decode(recorded.required("ordinal").intValue(), bytes(recorded, "payload_hex"));
                assertArrayEquals(bytes(recorded.required("evm"), "calldata"), NativeProgramLifecycleRequest.calldata(operation, FEE));
            }
        }
    }

    @Test
    void nativeCallTotalBoundIncludesEveryFieldBeforeEvmPreparation() throws Exception {
        NativeProgramCall seed = callFixture();
        int overhead = 106 + seed.entrypoint().length() + seed.capabilities().length + seed.accessDeclaration().length;
        NativeProgramCall exact = call(seed, new byte[LIMIT - overhead], seed.capabilities(), seed.accessDeclaration());
        assertEquals(LIMIT, exact.encode().length);
        assertArrayEquals(exact.encode(), NativeProgramCall.decode(exact.encode()).encode());
        assertArrayEquals(exact.encode(), NativeProgramCall.decodeCalldata(exact.calldata(FEE)).encode());
        assertArrayEquals(Arrays.copyOf(bytes(fixture("native-program-call-v4.json").required("evm"), "calldata"), 4),
            Arrays.copyOf(exact.calldata(FEE), 4));
        byte[] signed = ReceiptFixtureTest.signedTransaction(exact.calldata(FEE));
        assertArrayEquals(ProgramsClient.Programs.transactionHash(signed), exact.bindSignedActivity(signed));
        NativeProgramCall oversized = call(seed, new byte[LIMIT - overhead + 1], seed.capabilities(), seed.accessDeclaration());
        assertThrows(IllegalArgumentException.class, oversized::encode);
        assertThrows(IllegalArgumentException.class, () -> oversized.calldata(FEE));
        NativeProgramCall combined = call(seed, new byte[LIMIT / 2], seed.capabilities(), new byte[LIMIT / 2]);
        assertThrows(IllegalArgumentException.class, combined::encode);
        byte[] wire = Arrays.copyOf(exact.encode(), LIMIT + 1);
        ByteBuffer.wrap(wire).putInt(36, exact.calldata().length + 1);
        assertThrows(IllegalArgumentException.class, () -> NativeProgramCall.decode(wire));
        try (var archive = preparationArchive(); var connection = new ProgramsClient.Programs(archive.uri())) {
            var client = client(connection, programReceipt());
            var prepared = await(client.call(exact, FEE));
            assertEquals(ProgramsClient.PROGRAMS_ADDRESS, prepared.transaction().required("to").textValue());
            assertArrayEquals(exact.calldata(FEE), ProgramsClient.Programs.data(prepared.transaction().required("data")));
            assertThrows(IllegalArgumentException.class, () -> client.call(oversized, FEE));
            assertEquals(2, archive.requests.get());
        }
    }

    @Test
    void individualCallBoundsRemainEnforced() throws Exception {
        NativeProgramCall seed = callFixture();
        for (NativeProgramCall invalid : List.of(
                call(seed, new byte[1048577], seed.capabilities(), seed.accessDeclaration()),
                call(seed, seed.calldata(), new byte[65536], seed.accessDeclaration()),
                call(seed, seed.calldata(), seed.capabilities(), new byte[1048577]),
                new NativeProgramCall(seed.programId(), seed.guestAbi(), "x".repeat(129), seed.calldata(),
                    seed.capabilities(), seed.accessDeclaration(), seed.responseCapacity(), seed.resources()),
                new NativeProgramCall(seed.programId(), seed.guestAbi(), seed.entrypoint(), seed.calldata(),
                    seed.capabilities(), seed.accessDeclaration(), 1048577, seed.resources()),
                new NativeProgramCall(seed.programId(), seed.guestAbi(), seed.entrypoint(), seed.calldata(),
                    seed.capabilities(), seed.accessDeclaration(), seed.responseCapacity(), new long[8]))) {
            assertThrows(IllegalArgumentException.class, invalid::encode);
        }
    }

    @Test
    void deployTotalBoundIncludesOptionalInterfaceFraming() throws Exception {
        NativeProgramDeploy seed = deployFixture();
        for (boolean framed : new boolean[] {false, true}) {
            byte[] wasm = paddedWasm(seed.wasm(), LIMIT - (framed ? 108 + seed.programInterface().length : 104));
            byte[] iface = framed ? interfaceFor(seed, wasm) : null;
            var exact = new NativeProgramDeploy(seed.programId(), seed.guestAbi(), seed.policy(), seed.authority(),
                NativeProgramLifecycle.hash(wasm), iface, wasm);
            assertEquals(LIMIT, exact.encode().length);
            assertArrayEquals(exact.encode(), NativeProgramDeploy.decode(exact.encode()).encode());
            assertLifecycleTransaction(exact);
            byte[] oversizedWasm = paddedWasm(seed.wasm(), wasm.length + 1);
            assertThrows(IllegalArgumentException.class, () -> new NativeProgramDeploy(seed.programId(), seed.guestAbi(),
                seed.policy(), seed.authority(), NativeProgramLifecycle.hash(oversizedWasm),
                framed ? interfaceFor(seed, oversizedWasm) : null, oversizedWasm));
            assertThrows(IllegalArgumentException.class, () -> NativeProgramDeploy.decode(Arrays.copyOf(exact.encode(), LIMIT + 1)));
        }
        byte[] tooLarge = paddedWasm(seed.wasm(), 1048577);
        assertThrows(IllegalArgumentException.class, () -> new NativeProgramDeploy(seed.programId(), seed.guestAbi(),
            seed.policy(), seed.authority(), NativeProgramLifecycle.hash(tooLarge), null, tooLarge));
        assertThrows(IllegalArgumentException.class, () -> new NativeProgramDeploy(seed.programId(), seed.guestAbi(),
            seed.policy(), seed.authority(), seed.newHash(), new byte[953], seed.wasm()));
    }

    @Test
    void upgradeTotalBoundIncludesHookInterfaceAndClearFrame() throws Exception {
        NativeProgramDeploy seed = deployFixture();
        NativeProgramUpgrade recorded = NativeProgramUpgrade.decode(bytes(fixture("native-program-upgrade-v4.json"), "payload_hex"));
        for (int mode = 0; mode < 3; mode++) {
            boolean clear = mode == 2;
            int ifaceLength = mode == 1 ? seed.programInterface().length : 0;
            byte[] hook = recorded.migrationHook();
            byte[] wasm = paddedWasm(seed.wasm(), LIMIT - 106 - hook.length - (mode == 0 ? 0 : 4 + ifaceLength));
            byte[] iface = mode == 0 ? null : clear ? new byte[0] : interfaceFor(seed, wasm);
            var exact = new NativeProgramUpgrade(seed.programId(), seed.guestAbi(), seed.newHash(),
                NativeProgramLifecycle.hash(wasm), hook, clear, iface, wasm);
            assertEquals(LIMIT, exact.encode().length);
            assertArrayEquals(exact.encode(), NativeProgramUpgrade.decode(exact.encode()).encode());
            assertLifecycleTransaction(exact);
            byte[] largerHook = Arrays.copyOf(hook, hook.length + 1);
            assertThrows(IllegalArgumentException.class, () -> new NativeProgramUpgrade(seed.programId(), seed.guestAbi(),
                seed.newHash(), NativeProgramLifecycle.hash(wasm), largerHook, clear, iface, wasm));
            assertThrows(IllegalArgumentException.class, () -> NativeProgramUpgrade.decode(Arrays.copyOf(exact.encode(), LIMIT + 1)));
        }
        assertThrows(IllegalArgumentException.class, () -> new NativeProgramUpgrade(seed.programId(), seed.guestAbi(),
            seed.newHash(), seed.newHash(), new byte[65536], false, null, seed.wasm()));
        assertThrows(IllegalArgumentException.class, () -> new NativeProgramUpgrade(seed.programId(), seed.guestAbi(),
            seed.newHash(), seed.newHash(), new byte[0], false, new byte[953], seed.wasm()));
        assertThrows(IllegalArgumentException.class, () -> NativeProgramPayload.size(Long.MAX_VALUE));
    }

    @Test
    void productionBodyDeadlineCoversBothPlanesAndMutationUncertainty() throws Exception {
        for (boolean agent : new boolean[] {false, true}) {
            for (boolean mutation : new boolean[] {false, true}) {
                try (var server = stalledArchive(); var http = http(); var session = session()) {
                    var transport = transport(http, server.uri(), DEADLINE, session);
                    var pending = transport.call(operation(agent, mutation), JSON.constructType(JsonNode.class));
                    assertTrue(server.headers.await(5, TimeUnit.SECONDS), "server sent headers before body deadline");
                    PlatformSdkException failure = refused(pending);
                    assertEquals(mutation ? PlatformSdkException.Code.UNKNOWN_OUTCOME : PlatformSdkException.Code.TRANSPORT_FAILURE,
                        failure.code());
                    assertEquals(mutation ? PlatformSdkException.Retry.UNKNOWN_OUTCOME : PlatformSdkException.Retry.SAFE,
                        failure.retry());
                    assertEquals(1, server.requests.get());
                    assertEquals(1, server.release.getCount(), "body remains unfinished when operation terminates");
                }
            }
        }
    }

    @Test
    void throwingTelemetryPreservesSuccessfulMutationAndOriginalServiceFailure() throws Exception {
        for (boolean agent : new boolean[] {false, true}) {
            for (boolean refusal : new boolean[] {false, true}) {
                try (var server = outcomeArchive(agent, refusal); var http = http(); var session = session()) {
                    var transport = transport(http, server.uri(), Duration.ofSeconds(5), session);
                    var event = new AtomicReference<ProductionClient.TelemetryEvent>();
                    var recorded = new CountDownLatch(1);
                    var client = new ProductionClient(transport, JSON, value -> {
                        event.set(value); recorded.countDown(); throw new IllegalStateException("telemetry unavailable");
                    });
                    ProductionTransport.Call call = operation(agent, true);
                    CompletionStage<? extends SchemaTypes.Response> pending = agent
                        ? client.agent(new SchemaTypes.AgentRequest(OperationCatalog.agent(call.operation().wireName()), call.request()),
                            ProductionClient.Options.idempotent(KEY))
                        : client.human(new SchemaTypes.HumanRequest(OperationCatalog.human(call.operation().wireName()), call.request()),
                            ProductionClient.Options.idempotent(KEY));
                    if (refusal) {
                        var failure = refused(pending);
                        assertEquals(agent ? PlatformSdkException.Code.POLICY_REFUSAL : PlatformSdkException.Code.CORE_REJECTION, failure.code());
                        assertEquals(PlatformSdkException.Retry.NEVER, failure.retry());
                        assertEquals(agent ? "0" : fixturePath("services/human/schema/human-api/golden/session.open.failure.json")
                            .required("body").required("trace").textValue(), failure.requestId());
                        if (agent) assertEquals(SchemaErrors.AgentClass.POLICY_REFUSAL, failure.agentClass());
                        else assertEquals(SchemaErrors.HumanCode.CHALLENGE_EXPIRED, failure.humanCode());
                    } else {
                        JsonNode expected = agent ? JSON.createObjectNode().put("account", "ready")
                            : fixturePath("services/human/schema/human-api/golden/session.open.response.json").required("body").required("result");
                        assertEquals(expected, await(pending).value());
                    }
                    assertTrue(recorded.await(5, TimeUnit.SECONDS));
                    assertEquals(1, server.requests.get());
                    long end = System.nanoTime() + TimeUnit.SECONDS.toNanos(5);
                    while (client.telemetryFailureCount() == 0 && System.nanoTime() < end)
                        java.util.concurrent.locks.LockSupport.parkNanos(TimeUnit.MILLISECONDS.toNanos(1));
                    assertEquals(1, client.telemetryFailureCount());
                    assertEquals(refusal ? "refused" : "completed", event.get().outcome());
                    assertEquals(refusal ? agent ? PlatformSdkException.Code.POLICY_REFUSAL : PlatformSdkException.Code.CORE_REJECTION
                        : null, event.get().code());
                }
            }
        }
    }

    @Test
    void programPlaintextOriginsAreLoopbackOnly() throws Exception {
        try (var http = http()) {
            for (String origin : List.of("http://rpc.example", "http://192.0.2.1", "http://127.evil.example",
                    "http://127.0.0.1.evil.example", "http://127.999.0.1", "http://[2001:db8::1]",
                    "https://user@rpc.example", "https://rpc.example/#fragment", "https://rpc.example/?query")) {
                var failure = assertThrows(PlatformSdkException.class,
                    () -> new ProgramsClient.Programs(URI.create(origin), http, Duration.ofSeconds(5)), origin);
                assertEquals(PlatformSdkException.Code.INVALID_ARGUMENT, failure.code());
                assertThrows(PlatformSdkException.class, () -> transport(http, URI.create(origin), Duration.ofSeconds(5), null));
            }
            for (String origin : List.of("http://localhost", "http://127.0.0.1", "http://127.2.3.4", "http://[::1]")) {
                assertEquals(URI.create(origin), HttpProductionTransport.requireHttpUri(URI.create(origin)));
            }
        }
    }

    @Test
    void authenticatedHttpsDeliversRecordedReceiptAndRejectsUntrustedPeer() throws Exception {
        JsonNode vector = programReceipt();
        Tls tls = certificate();
        try (var server = new Loopback(exchange -> {
            JsonNode request = JSON.readTree(exchange.getRequestBody());
            assertEquals("pax_sequencerReceipt", request.required("method").textValue());
            rpcReply(exchange, request, vector.required("receiptJson"));
        }, tls.server()); var http = HttpClient.newBuilder().sslContext(tls.client()).build();
                var connection = new ProgramsClient.Programs(server.uri(), http, Duration.ofSeconds(5))) {
            var receipt = await(client(connection, vector).sequencerReceipt(bytes(vector, "rawTransaction")));
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, receipt.level());
            assertArrayEquals(bytes(vector, "receiptDigest"), receipt.receiptDigest());
            assertEquals(1, server.requests.get());
            try (var untrusted = new ProgramsClient.Programs(server.uri(), http(), Duration.ofSeconds(5))) {
                assertEquals(PlatformSdkException.Code.TRANSPORT_FAILURE, refused(untrusted.request(
                    "pax_sequencerReceipt", JSON.createArrayNode())).code());
            }
            assertEquals(1, server.requests.get(), "untrusted TLS peer never receives an HTTP request");
        }
    }

    @Test
    void programStreamingResponseOverflowCancelsBeforeReadingWholeEntity() throws Exception {
        try (var server = new Loopback(exchange -> {
            JSON.readTree(exchange.getRequestBody());
            exchange.getResponseHeaders().set("Content-Type", "application/json");
            exchange.sendResponseHeaders(200, 0);
            byte[] chunk = new byte[16384]; Arrays.fill(chunk, (byte) ' ');
            exchange.getResponseBody().write("{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"".getBytes(StandardCharsets.US_ASCII));
            try {
                for (int i = 0; i < 4096; i++) { exchange.getResponseBody().write(chunk); exchange.getResponseBody().flush(); }
                fail("client consumed an unbounded body");
            } catch (IOException cancelled) { /* Cancellation closes the real TCP response. */ }
            finally { serverCancelled.countDown(); }
        }); var connection = new ProgramsClient.Programs(server.uri(), http(), Duration.ofSeconds(10))) {
            var failure = refused(connection.request("pax_sequencerReceipt", JSON.createArrayNode()));
            assertEquals(PlatformSdkException.Code.DECODE_FAILURE, failure.code());
            assertEquals(PlatformSdkException.Retry.NEVER, failure.retry());
            assertTrue(serverCancelled.await(5, TimeUnit.SECONDS), "overflow cancels the upstream response");
            assertEquals(1, server.requests.get());
        }
    }
    private final CountDownLatch serverCancelled = new CountDownLatch(1);

    @Test
    void programResponseExactByteLimitIsAcceptedAndDeclaredOverflowRefused() throws Exception {
        for (boolean oversized : new boolean[] {false, true}) {
            try (var server = new Loopback(exchange -> {
                JsonNode request = JSON.readTree(exchange.getRequestBody());
                ObjectNode response = JSON.createObjectNode().put("jsonrpc", "2.0").put("result", "0x7d");
                response.set("id", request.required("id"));
                byte[] prefix = JSON.writeValueAsBytes(response);
                exchange.sendResponseHeaders(200, ProgramsClient.Programs.MAXIMUM_RESPONSE_BYTES + (oversized ? 1L : 0L));
                if (oversized) { exchange.getResponseBody().flush(); return; }
                exchange.getResponseBody().write(prefix);
                byte[] spaces = new byte[8192]; Arrays.fill(spaces, (byte) ' ');
                int remaining = ProgramsClient.Programs.MAXIMUM_RESPONSE_BYTES - prefix.length;
                while (remaining > 0) { int length = Math.min(remaining, spaces.length); exchange.getResponseBody().write(spaces, 0, length); remaining -= length; }
            }); var connection = new ProgramsClient.Programs(server.uri(), http(), Duration.ofSeconds(10))) {
                var pending = connection.request("eth_chainId", JSON.createArrayNode());
                if (oversized) assertEquals(PlatformSdkException.Code.DECODE_FAILURE, refused(pending).code());
                else assertEquals("0x7d", await(pending).textValue());
            }
        }
    }

    @Test
    void programBodyDeadlineIncludesStalledBodyAndPreservesMutationUncertainty() throws Exception {
        for (String method : List.of("pax_sequencerReceipt", "eth_sendRawTransaction")) {
            try (var server = stalledArchive(); var connection = new ProgramsClient.Programs(server.uri(), http(), DEADLINE)) {
                var pending = connection.request(method, JSON.createArrayNode());
                assertTrue(server.headers.await(5, TimeUnit.SECONDS));
                var failure = refused(pending);
                boolean mutation = "eth_sendRawTransaction".equals(method);
                assertEquals(mutation ? PlatformSdkException.Code.UNKNOWN_OUTCOME : PlatformSdkException.Code.TRANSPORT_FAILURE,
                    failure.code());
                assertEquals(mutation ? PlatformSdkException.Retry.UNKNOWN_OUTCOME : PlatformSdkException.Retry.SAFE,
                    failure.retry());
                assertEquals(1, server.release.getCount());
            }
        }
    }

    private static NativeProgramCall call(NativeProgramCall seed, byte[] data, byte[] caps, byte[] access) {
        return new NativeProgramCall(seed.programId(), seed.guestAbi(), seed.entrypoint(), data, caps, access,
            seed.responseCapacity(), seed.resources());
    }
    private static NativeProgramCall callFixture() throws Exception {
        return NativeProgramCall.decode(bytes(fixture("native-program-call-v4.json"), "payload_hex"));
    }
    private static NativeProgramDeploy deployFixture() throws Exception {
        return NativeProgramDeploy.decode(bytes(fixture("native-program-deploy-v4.json"), "payload_hex"));
    }
    private static void assertLifecycleTransaction(NativeProgramLifecycle value) throws Exception {
        byte[] data = NativeProgramLifecycleRequest.calldata(value, FEE);
        NativeProgramLifecycle decoded;
        byte[] program = Arrays.copyOfRange(data, 4, 36);
        int abi = ProgramsClient.Programs.number(data, 36, 16).intValueExact();
        byte[] code = ProgramsClient.Programs.dynamic(data, 4, value instanceof NativeProgramDeploy ? 6 : 7);
        byte[] iface = ProgramsClient.Programs.dynamic(data, 4, value instanceof NativeProgramDeploy ? 5 : 6);
        if (value instanceof NativeProgramDeploy) {
            decoded = new NativeProgramDeploy(program, abi, ProgramsClient.Programs.number(data, 68, 8).intValueExact(),
                Arrays.copyOfRange(data, 100, 132), Arrays.copyOfRange(data, 132, 164), iface.length == 0 ? null : iface, code);
        } else {
            int flags = ProgramsClient.Programs.number(data, 68, 8).intValueExact();
            decoded = new NativeProgramUpgrade(program, abi, Arrays.copyOfRange(data, 100, 132),
                Arrays.copyOfRange(data, 132, 164), ProgramsClient.Programs.dynamic(data, 4, 5), (flags & 2) != 0,
                iface.length == 0 && (flags & 2) == 0 ? null : iface, code);
        }
        assertArrayEquals(value.encode(), decoded.encode());
        byte[] signed = ReceiptFixtureTest.signedTransaction(data);
        var bound = new NativeProgramLifecycleRequest(decoded, signed, KEY);
        assertArrayEquals(value.encode(), bound.payload());
        assertArrayEquals(ProgramsClient.Programs.transactionHash(signed), bound.transactionHash());
        assertArrayEquals(data, ProgramsClient.Programs.transactionData(signed));
        try (var server = preparationArchive(); var connection = new ProgramsClient.Programs(server.uri())) {
            ProgramsClient client = client(connection, programReceipt());
            var prepared = await(value instanceof NativeProgramDeploy deploy ? client.deploy(deploy)
                : client.upgrade((NativeProgramUpgrade) value, FEE));
            assertEquals(ProgramsClient.PROGRAMS_ADDRESS, prepared.transaction().required("to").textValue());
            assertArrayEquals(data, ProgramsClient.Programs.data(prepared.transaction().required("data")));
            assertEquals(2, server.requests.get());
        }
    }
    private static byte[] paddedWasm(byte[] seed, int size) throws IOException {
        int padding = size - seed.length;
        for (int leb = 1; leb <= 5; leb++) {
            int content = padding - 1 - leb;
            if (content < 1) continue;
            ByteArrayOutputStream length = new ByteArrayOutputStream();
            int remaining = content;
            do { int next = remaining & 127; remaining >>>= 7; length.write(next | (remaining > 0 ? 128 : 0)); } while (remaining > 0);
            if (length.size() != leb) continue;
            ByteArrayOutputStream result = new ByteArrayOutputStream(size);
            result.write(seed); result.write(0); result.write(length.toByteArray()); result.write(new byte[content]);
            assertEquals(size, result.size());
            return result.toByteArray();
        }
        throw new IllegalArgumentException("cannot frame Wasm custom section");
    }
    private static byte[] interfaceFor(NativeProgramDeploy seed, byte[] wasm) {
        byte[] iface = seed.programInterface();
        byte[] old = seed.newHash(), hash = NativeProgramLifecycle.hash(wasm);
        for (int i = 0; i <= iface.length - 32; i++) {
            if (Arrays.equals(old, Arrays.copyOfRange(iface, i, i + 32))) {
                System.arraycopy(hash, 0, iface, i, 32); return iface;
            }
        }
        throw new AssertionError("recorded interface does not bind recorded Wasm");
    }
    private static HttpClient http() { return HttpClient.newBuilder().version(HttpClient.Version.HTTP_1_1).build(); }
    private static HttpProductionTransport.AgentSessionCredential session() {
        return new HttpProductionTransport.AgentSessionCredential("tenant", "11".repeat(32),
            new SecretBytes("22".repeat(32).getBytes(StandardCharsets.US_ASCII)), "7");
    }
    private static HttpProductionTransport transport(HttpClient http, URI endpoint, Duration timeout,
            HttpProductionTransport.AgentSessionCredential session) {
        return new HttpProductionTransport(http, JSON, endpoint, endpoint, timeout, null, session);
    }
    private static ProductionTransport.Call operation(boolean agent, boolean mutation) throws IOException {
        SchemaTypes.Operation operation = agent ? OperationCatalog.agent(mutation ? "approval.approve" : "session.list")
            : OperationCatalog.human(mutation ? "session.open" : "session.list");
        ObjectNode request = !agent && mutation
            ? (ObjectNode) fixturePath("services/human/schema/human-api/golden/session.open.request.json").required("body")
            : JSON.createObjectNode();
        return new ProductionTransport.Call(operation, request, null, mutation ? KEY : null);
    }
    private static Loopback stalledArchive() throws Exception {
        AtomicReference<Loopback> reference = new AtomicReference<>();
        Loopback server = new Loopback(exchange -> {
            exchange.getRequestBody().readAllBytes();
            exchange.getResponseHeaders().set("Content-Type", "application/json");
            exchange.sendResponseHeaders(200, 0);
            exchange.getResponseBody().write('{'); exchange.getResponseBody().flush();
            reference.get().headers.countDown();
            try { assertTrue(reference.get().release.await(10, TimeUnit.SECONDS), "test releases stalled response"); }
            catch (InterruptedException error) { Thread.currentThread().interrupt(); throw new IOException(error); }
        });
        reference.set(server); return server;
    }
    private static Loopback outcomeArchive(boolean agent, boolean refusal) throws Exception {
        return new Loopback(exchange -> {
            JsonNode request = JSON.readTree(exchange.getRequestBody());
            assertEquals(KEY.value(), agent ? request.required("idempotency_key").textValue()
                : exchange.getRequestHeaders().getFirst("Idempotency-Key"));
            ObjectNode body;
            if (agent) {
                if (refusal) {
                    body = fixturePath("services/agent/schema/agent-api/golden/agent-http-refusal.json").deepCopy();
                    body.put("version", 2).put("request_id", "0");
                } else {
                    body = JSON.createObjectNode().put("version", 2);
                    body.set("request_id", request.required("request_id"));
                    body.putObject("value").put("account", "ready");
                    body.putObject("verification_status").put("state", "achieved").put("level", "Unverified");
                }
            } else {
                JsonNode recorded = fixturePath("services/human/schema/human-api/golden/session.open."
                    + (refusal ? "failure" : "response") + ".json");
                send(exchange, recorded.required("status").intValue(), JSON.writeValueAsBytes(recorded.required("body")));
                return;
            }
            send(exchange, refusal ? 403 : 200, JSON.writeValueAsBytes(body));
        });
    }
    private static Loopback preparationArchive() throws Exception {
        return new Loopback(exchange -> {
            JsonNode request = JSON.readTree(exchange.getRequestBody());
            String method = request.required("method").textValue();
            assertEquals("pending", request.required("params").required(1).textValue());
            if ("eth_getTransactionCount".equals(method)) rpcReply(exchange, request, JSON.getNodeFactory().textNode("0x3"));
            else {
                assertEquals("eth_estimateGas", method);
                assertEquals(ProgramsClient.PROGRAMS_ADDRESS, request.required("params").required(0).required("to").textValue());
                rpcReply(exchange, request, JSON.getNodeFactory().textNode("0x5208"));
            }
        });
    }
    private static void rpcReply(HttpExchange exchange, JsonNode request, JsonNode result) throws IOException {
        ObjectNode reply = JSON.createObjectNode().put("jsonrpc", "2.0");
        reply.set("id", request.required("id")); reply.set("result", result);
        send(exchange, 200, JSON.writeValueAsBytes(reply));
    }
    private static void send(HttpExchange exchange, int status, byte[] body) throws IOException {
        exchange.getResponseHeaders().set("Content-Type", "application/json");
        exchange.sendResponseHeaders(status, body.length); exchange.getResponseBody().write(body);
    }
    private static <T> T await(CompletionStage<T> pending) throws Exception { return pending.toCompletableFuture().get(10, TimeUnit.SECONDS); }
    private static PlatformSdkException refused(CompletionStage<?> pending) {
        var failure = assertThrows(java.util.concurrent.ExecutionException.class,
            () -> pending.toCompletableFuture().get(10, TimeUnit.SECONDS));
        Throwable cause = failure.getCause();
        while (cause instanceof java.util.concurrent.CompletionException && cause.getCause() != null) cause = cause.getCause();
        return assertInstanceOf(PlatformSdkException.class, cause);
    }
    private static ProgramsClient client(ProgramsClient.Programs connection, JsonNode vector) {
        return new ProgramsClient(connection, vector.required("receiptJson").required("from").textValue(),
            BigInteger.ONE, BigInteger.TWO, keys(vector), key(vector).keyId());
    }
    private static LocalVerifier.ReceiptKey key(JsonNode vector) {
        JsonNode pin = vector.required("pinnedKey");
        return new LocalVerifier.ReceiptKey(bytes(pin, "keyId"), bytes(pin, "publicKey"),
            pin.required("firstHeight").bigIntegerValue(), pin.required("lastHeight").bigIntegerValue());
    }
    private static LocalVerifier.ReceiptKeys keys(JsonNode vector) { return new LocalVerifier.ReceiptKeys(List.of(key(vector))); }
    private static JsonNode programReceipt() throws Exception {
        for (JsonNode vector : fixture("sequencer-receipt-v1.json").required("cases"))
            if ("V1-layerxprograms-call".equals(vector.required("id").textValue())) return vector;
        throw new AssertionError("missing recorded program receipt");
    }
    private static JsonNode fixture(String name) throws Exception { return fixturePath("sdk/platform/conformance/fixtures/" + name); }
    private static JsonNode fixturePath(String name) throws IOException {
        Path root = Path.of("").toAbsolutePath();
        while (root != null) {
            Path path = root.resolve(name);
            if (Files.isRegularFile(path)) return JSON.readTree(Files.readAllBytes(path));
            root = root.getParent();
        }
        throw new IOException("missing committed fixture: " + name);
    }
    private static byte[] bytes(JsonNode node, String field) {
        String value = node.required(field).textValue();
        return HexFormat.of().parseHex(value.startsWith("0x") ? value.substring(2) : value);
    }

    private record Tls(SSLContext server, SSLContext client) {}
    private static Tls certificate() throws Exception {
        var generator = KeyPairGenerator.getInstance("RSA"); generator.initialize(2048);
        var pair = generator.generateKeyPair();
        var algorithm = new AlgorithmIdentifier(PKCSObjectIdentifiers.sha256WithRSAEncryption, DERNull.INSTANCE);
        var tbs = new V3TBSCertificateGenerator();
        tbs.setSerialNumber(new ASN1Integer(BigInteger.ONE)); tbs.setSignature(algorithm);
        tbs.setIssuer(new X500Name("CN=localhost")); tbs.setSubject(new X500Name("CN=localhost"));
        tbs.setStartDate(new Time(Date.from(Instant.now().minusSeconds(3600))));
        tbs.setEndDate(new Time(Date.from(Instant.now().plusSeconds(86400))));
        tbs.setSubjectPublicKeyInfo(SubjectPublicKeyInfo.getInstance(pair.getPublic().getEncoded()));
        var extensions = new ExtensionsGenerator();
        extensions.addExtension(Extension.subjectAlternativeName, false,
            new GeneralNames(new GeneralName[] {new GeneralName(GeneralName.iPAddress, "127.0.0.1")}));
        tbs.setExtensions(extensions.generate());
        var certificateBody = tbs.generateTBSCertificate();
        var signer = Signature.getInstance("SHA256withRSA"); signer.initSign(pair.getPrivate());
        signer.update(certificateBody.getEncoded("DER"));
        var fields = new ASN1EncodableVector(); fields.add(certificateBody); fields.add(algorithm); fields.add(new DERBitString(signer.sign()));
        var certificate = CertificateFactory.getInstance("X.509").generateCertificate(
            new ByteArrayInputStream(new DERSequence(fields).getEncoded("DER")));
        certificate.verify(pair.getPublic());
        var store = KeyStore.getInstance("PKCS12"); store.load(null, null);
        store.setKeyEntry("loopback", pair.getPrivate(), new char[0], new java.security.cert.Certificate[] {certificate});
        var managers = KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm()); managers.init(store, new char[0]);
        var server = SSLContext.getInstance("TLS"); server.init(managers.getKeyManagers(), null, null);
        var trust = KeyStore.getInstance("PKCS12"); trust.load(null, null); trust.setCertificateEntry("loopback", certificate);
        var factory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm()); factory.init(trust);
        var client = SSLContext.getInstance("TLS"); client.init(null, factory.getTrustManagers(), null);
        return new Tls(server, client);
    }

    private static final class Loopback implements AutoCloseable {
        final HttpServer server;
        final ExecutorService executor = Executors.newVirtualThreadPerTaskExecutor();
        final AtomicReference<Throwable> failure = new AtomicReference<>();
        final AtomicInteger requests = new AtomicInteger();
        final CountDownLatch headers = new CountDownLatch(1);
        final CountDownLatch release = new CountDownLatch(1);
        Loopback(HttpHandler handler) throws Exception { this(handler, null); }
        Loopback(HttpHandler handler, SSLContext tls) throws Exception {
            if (tls == null) server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            else {
                HttpsServer secure = HttpsServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
                secure.setHttpsConfigurator(new HttpsConfigurator(tls)); server = secure;
            }
            server.setExecutor(executor);
            server.createContext("/", exchange -> {
                requests.incrementAndGet();
                try { handler.handle(exchange); }
                catch (IOException disconnected) { /* Real clients may cancel a response. */ }
                catch (Throwable error) { failure.compareAndSet(null, error); }
                finally { exchange.close(); }
            });
            server.start();
        }
        URI uri() { return URI.create((server instanceof HttpsServer ? "https" : "http")
            + "://127.0.0.1:" + server.getAddress().getPort()); }
        @Override public void close() {
            release.countDown(); server.stop(0); executor.close();
            if (failure.get() != null) throw new AssertionError("loopback peer failed", failure.get());
        }
    }
}
