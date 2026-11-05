package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.ConsensusTrust;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.PortableReceipt;
import com.sun.net.httpserver.HttpServer;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.PrintStream;
import java.lang.reflect.InvocationTargetException;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.net.URI;
import java.net.URL;
import java.net.URLClassLoader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import java.util.ArrayList;
import java.util.Base64;
import java.util.HexFormat;
import java.util.List;
import java.util.Set;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
import javax.tools.DiagnosticCollector;
import javax.tools.JavaFileObject;
import javax.tools.ToolProvider;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import static org.junit.jupiter.api.Assertions.*;

/** Compiles and runs the published examples against the real SDK and recorded producer evidence. */
public final class Repair22331392Test {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final String PACKAGE = "com.sidiora.layerx.sdk.examples.";
    // Public transaction-vector key from rpc/sequencer_receipt_vectors_test.go; never a funded credential.
    private static final String VECTOR_SECRET = "8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63";
    private static final String RECIPIENT = "0x8ba1f109551bd432803012645ac136ddd64dba72";
    private static final String SENDER = "0xfe3b557e8fb62b89f4916b721be55ceb828dbd73";
    private static Path root;
    private static URLClassLoader examples;
    private static Class<?> paymentType;
    @TempDir static Path compiled;
    @TempDir Path work;

    @BeforeAll
    static void compileEveryPublishedExampleInProcess() throws Exception {
        root = Path.of("").toAbsolutePath();
        while (!Files.isDirectory(root.resolve("sdk/platform/jvm/examples"))) {
            root = root.getParent();
            assertNotNull(root, "repository root must be discoverable from the Maven basedir");
        }
        String classpath = System.getProperty("surefire.test.class.path", System.getProperty("java.class.path"));
        var compiler = ToolProvider.getSystemJavaCompiler();
        assertNotNull(compiler, "the declared Java 21 build must supply its in-process compiler");
        var diagnostics = new DiagnosticCollector<JavaFileObject>();
        try (var files = compiler.getStandardFileManager(diagnostics, null, StandardCharsets.UTF_8);
                var sources = Files.list(root.resolve("sdk/platform/jvm/examples"))) {
            var javaSources = sources.filter(path -> path.toString().endsWith(".java")).toList();
            assertEquals(4, javaSources.size(), "all Java examples and their support must compile");
            boolean compiledJava = compiler.getTask(null, files, diagnostics,
                List.of("--release", "21", "-classpath", classpath, "-d", compiled.toString()), null,
                files.getJavaFileObjectsFromPaths(javaSources)).call();
            assertTrue(compiledJava, () -> diagnostics.getDiagnostics().toString());
        }
        // The root POM already declares Kotlin's Maven compiler. Use those resolved Java libraries,
        // in this JVM, rather than requiring kotlinc, a subprocess, an environment variable or a download.
        Path stdlib = Path.of(kotlin.Unit.class.getProtectionDomain().getCodeSource().getLocation().toURI());
        Path kotlinGroup = stdlib.getParent().getParent().getParent();
        Path jetbrains = kotlinGroup.getParent();
        List<URL> libraries = new ArrayList<>();
        try (var paths = Files.walk(jetbrains)) {
            for (Path path : paths.filter(path -> path.toString().endsWith(".jar")).sorted().toList()) {
                libraries.add(path.toUri().toURL());
            }
        }
        assertTrue(libraries.stream().anyMatch(url -> url.toString().contains("/kotlin-compiler/")),
            "the declared Kotlin Maven plugin must have resolved its compiler dependency");
        var output = new ByteArrayOutputStream();
        ClassLoader previous = Thread.currentThread().getContextClassLoader();
        try (var kotlinCompiler = new URLClassLoader(libraries.toArray(URL[]::new), ClassLoader.getPlatformClassLoader());
                var messages = new PrintStream(output, true, StandardCharsets.UTF_8)) {
            Thread.currentThread().setContextClassLoader(kotlinCompiler);
            Class<?> entry = kotlinCompiler.loadClass("org.jetbrains.kotlin.cli.jvm.K2JVMCompiler");
            Object result = entry.getMethod("exec", PrintStream.class, String[].class).invoke(
                entry.getConstructor().newInstance(), messages, new String[] {
                    "-no-stdlib", "-no-reflect", "-jvm-target", "21", "-classpath",
                    classpath + File.pathSeparator + compiled, "-d", compiled.toString(),
                    root.resolve("sdk/platform/jvm/examples/ServiceLifecycleExample.kt").toString()});
            assertEquals("OK", result.toString(), () -> output.toString(StandardCharsets.UTF_8));
        } finally {
            Thread.currentThread().setContextClassLoader(previous);
        }
        examples = new URLClassLoader(new URL[] {compiled.toUri().toURL()}, Repair22331392Test.class.getClassLoader());
        paymentType = examples.loadClass(PACKAGE + "Eip1559Payment$Payment");
        assertNotNull(examples.loadClass(PACKAGE + "ServiceLifecycleExample").getMethod("main", String[].class));
    }

    @AfterAll
    static void closeExampleLoader() throws Exception {
        if (examples != null) examples.close();
    }

    @Test
    void firstPaymentUsesRootEvmRpcAndVerifiesTheRecordedProducerReceipt() throws Exception {
        JsonNode vector = transfer();
        IdempotencyKey key = randomKey();
        try (var server = new RecordedNode(vector); var node = new ProgramsClient.Programs(server.uri());
                var secret = new SecretBytes(HexFormat.of().parseHex(VECTOR_SECRET))) {
            var verified = await(call("FirstPaymentExample", "pay",
                new Class<?>[] {ProgramsClient.Programs.class, paymentType, SecretBytes.class,
                    LocalVerifier.ReceiptKeys.class, Path.class, IdempotencyKey.class},
                node, payment(BigInteger.TEN.pow(18)), secret, keys(vector), work, key));
            var receipt = assertInstanceOf(LocalVerifier.SequencerReceiptVerification.class, verified);
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, receipt.level());
            assertEquals(1, receipt.status());
            assertDoesNotThrow(receipt::requireSuccess);
            assertArrayEquals(bytes(vector, "signedReceipt"), receipt.canonicalBytes());
            assertArrayEquals(bytes(vector, "receiptDigest"), receipt.receiptDigest());
            assertEquals(BigInteger.TEN.pow(18), receipt.receipt().amount());
            assertArrayEquals(bytes(vector, "rawTransaction"), Files.readAllBytes(work.resolve(key.value() + ".raw")));
            assertEquals(PosixFilePermissions.fromString("rw-------"),
                Files.getPosixFilePermissions(work.resolve(key.value() + ".raw")));
            assertEquals(List.of("eth_chainId", "eth_getTransactionCount", "eth_estimateGas",
                "eth_sendRawTransaction", "pax_sequencerReceipt"), server.methods());
            assertEquals(List.of("/", "/", "/", "/", "/"), server.paths);
            assertTrue(server.authorization.stream().allMatch(String::isEmpty), "agent bearer must never reach node RPC");
            assertEquals(vector.required("rawTransaction").textValue(),
                server.requests.get(3).required("params").get(0).textValue(),
                "the real signer must reproduce the transaction whose receipt the node recorded");
            assertNull(server.failure.get());
        }
    }

    @Test
    void firstPaymentRefusesWrongChainBeforeSigningOrBroadcast() throws Exception {
        try (var server = new RecordedNode(transfer()); var node = new ProgramsClient.Programs(server.uri());
                var secret = new SecretBytes(HexFormat.of().parseHex(VECTOR_SECRET))) {
            server.chain = "0x1";
            PlatformSdkException error = refused(() -> await(call("FirstPaymentExample", "pay",
                new Class<?>[] {ProgramsClient.Programs.class, paymentType, SecretBytes.class,
                    LocalVerifier.ReceiptKeys.class, Path.class, IdempotencyKey.class},
                node, payment(BigInteger.TEN.pow(18)), secret, keys(transfer()), work, randomKey())));
            assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, error.code());
            assertEquals(List.of("eth_chainId"), server.methods());
            try (var journal = Files.list(work)) { assertEquals(0, journal.count()); }
            assertNull(server.failure.get());
        }
    }

    @Test
    void firstPaymentRefusesTamperingWrongAmountAndRecipient() throws Exception {
        JsonNode vector = transfer();
        Class<?>[] signature = {JsonNode.class, byte[].class, LocalVerifier.ReceiptKeys.class, paymentType, String.class};
        Object expected = payment(BigInteger.TEN.pow(18));
        ObjectNode tampered = vector.required("receiptJson").deepCopy();
        byte[] signed = bytes(vector, "signedReceipt");
        signed[signed.length - 1] ^= 1;
        tampered.put("receipt", "0x" + HexFormat.of().formatHex(signed));
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, refused(() -> call("FirstPaymentExample",
            "verifyPayment", signature, tampered, bytes(vector, "rawTransaction"), keys(vector), expected, SENDER)).code());
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, refused(() -> call("FirstPaymentExample",
            "verifyPayment", signature, vector.required("receiptJson"), bytes(vector, "rawTransaction"),
            keys(vector), payment(BigInteger.TWO), SENDER)).code());
        Object otherRecipient = paymentType.getConstructor(String.class, BigInteger.class, BigInteger.class, BigInteger.class)
            .newInstance(SENDER, BigInteger.TEN.pow(18), BigInteger.valueOf(3_000_000_000L), BigInteger.valueOf(1_000_000_000L));
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, refused(() -> call("FirstPaymentExample",
            "verifyPayment", signature, vector.required("receiptJson"), bytes(vector, "rawTransaction"),
            keys(vector), otherRecipient, SENDER)).code());
        byte[] wrongRaw = bytes(vector, "rawTransaction");
        wrongRaw[wrongRaw.length - 1] ^= 1;
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, refused(() -> call("FirstPaymentExample",
            "verifyPayment", signature, vector.required("receiptJson"), wrongRaw, keys(vector), expected, SENDER)).code());
    }

    @Test
    void offlineExampleVerifiesBothUnmodifiedPublishedPortableBundles() throws Exception {
        JsonNode published = fixture("sdk/platform/conformance/fixtures/vector-v5-portable.json");
        JsonNode committed = fixture("sdk/platform/conformance/fixtures/vector-v4-committed.json")
            .required("positive");
        // Trust comes from independent producer-key fixtures and a separately pinned consensus root.
        var pins = new LocalVerifier.ReceiptKeys(List.of(key(transfer()), key(committed)));
        var levels = new java.util.HashSet<String>();
        for (JsonNode bundle : published.required("bundles")) {
            Path proof = work.resolve(bundle.required("verificationLevel").textValue() + ".json");
            byte[] unchanged = JSON.writeValueAsBytes(bundle);
            Files.write(proof, unchanged);
            Path raw = work.resolve("transaction.raw");
            Files.write(raw, bytes(bundle, "rawTransaction"));
            var verified = offline(proof, raw, pins, trust());
            assertTrue(levels.add(verified.verificationLevel()));
            assertEquals(bundle.required("verificationLevel").textValue(), verified.verificationLevel());
            assertEquals(1, verified.status());
            assertArrayEquals(bytes(bundle, "signedReceipt"), verified.receipted().canonicalBytes());
            assertArrayEquals(bytes(bundle, "receiptDigest"), verified.receipted().receiptDigest());
            assertArrayEquals(unchanged, PortableReceipt.export(verified));
            assertArrayEquals(unchanged, Files.readAllBytes(proof));
            if ("committed".equals(verified.verificationLevel())) {
                assertNotNull(verified.committed());
                assertEquals(10, verified.committed().height());
                assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE,
                    refused(() -> offline(proof, raw, pins, null)).code());
            } else {
                assertNull(verified.committed());
            }
        }
        assertEquals(Set.of("receipted", "committed"), levels);
    }

    @Test
    void offlineExampleReadsSeparatePinsAndRefusesTamperingAndTransactionSubstitution() throws Exception {
        JsonNode bundle = fixture("sdk/platform/conformance/fixtures/vector-v5-portable.json").required("bundles").get(0);
        Path proof = work.resolve("bundle.json");
        Path raw = work.resolve("transaction.raw");
        Path pinsFile = work.resolve("pins.json");
        ObjectNode pin = transfer().required("pinnedKey").deepCopy();
        pin.put("firstHeight", "1000").put("lastHeight", "1999");
        Files.write(pinsFile, JSON.writeValueAsBytes(JSON.createArrayNode().add(pin)));
        var pins = (LocalVerifier.ReceiptKeys) call("ExampleConfiguration", "receiptKeys",
            new Class<?>[] {Path.class}, pinsFile);
        Files.write(proof, JSON.writeValueAsBytes(bundle));
        Files.write(raw, bytes(bundle, "rawTransaction"));
        call("OfflineVerificationExample", "main", new Class<?>[] {String[].class},
            (Object) new String[] {proof.toString(), raw.toString(), pinsFile.toString()});
        ObjectNode tampered = bundle.deepCopy();
        byte[] signed = bytes(bundle, "signedReceipt");
        signed[signed.length - 1] ^= 1;
        tampered.put("signedReceipt", "0x" + HexFormat.of().formatHex(signed));
        Files.write(proof, JSON.writeValueAsBytes(tampered));
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, refused(() -> offline(proof, raw, pins, null)).code());
        Files.write(proof, JSON.writeValueAsBytes(bundle));
        byte[] substituted = bytes(bundle, "rawTransaction");
        substituted[substituted.length - 1] ^= 1;
        Files.write(raw, substituted);
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, refused(() -> offline(proof, raw, pins, null)).code());
        Files.write(raw, bytes(bundle, "rawTransaction"));
        byte[] wrongPublicKey = key(transfer()).publicKey();
        wrongPublicKey[0] ^= 1;
        var untrusted = new LocalVerifier.ReceiptKeys(List.of(new LocalVerifier.ReceiptKey(
            key(transfer()).keyId(), wrongPublicKey, BigInteger.valueOf(1000), BigInteger.valueOf(1999))));
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE,
            refused(() -> offline(proof, raw, untrusted, null)).code());
    }

    @Test
    void kotlinBudgetSendsTypedRequestWithOwnerSessionAndRefusesUnmodifiedDaemonEnvelope() throws Exception {
        // This is the committed daemon-produced refusal, unmodified. The current daemon envelope is
        // v1 while the SDK requires v2; record the owning-domain wiring, never fabricate a v2 reply.
        byte[] refusal = Files.readAllBytes(root.resolve("services/agent/schema/agent-api/golden/agent-http-refusal.json"));
        List<JsonNode> requests = new CopyOnWriteArrayList<>();
        List<String> paths = new CopyOnWriteArrayList<>();
        List<String> authorization = new CopyOnWriteArrayList<>();
        AtomicReference<Throwable> handlerFailure = new AtomicReference<>();
        HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.createContext("/", exchange -> {
            try {
                requests.add(JSON.readTree(exchange.getRequestBody().readAllBytes()));
                paths.add(exchange.getRequestURI().getPath());
                authorization.add(exchange.getRequestHeaders().getFirst("Authorization"));
                exchange.getResponseHeaders().set("Content-Type", "application/json");
                exchange.sendResponseHeaders(403, refusal.length);
                exchange.getResponseBody().write(refusal);
            } catch (Throwable error) {
                handlerFailure.set(error);
            } finally { exchange.close(); }
        });
        server.start();
        IdempotencyKey key = randomKey();
        try (var bearer = new HttpProductionTransport.BearerCredential(
                    new SecretBytes("public-loopback-bearer".getBytes(StandardCharsets.US_ASCII)));
                var session = (HttpProductionTransport.AgentSessionCredential) call("ExampleConfiguration", "session",
                    new Class<?>[] {String.class, String.class, String.class, String.class},
                    "tenant", "11".repeat(32), "22".repeat(32), "7")) {
            URI agent = URI.create("http://127.0.0.1:" + server.getAddress().getPort());
            // A different, unused Human origin proves the mutation chooses only the Agent boundary.
            URI human = URI.create("http://127.0.0.1:1");
            var error = refused(() -> await(call("ServiceLifecycleExample", "createBudget",
                new Class<?>[] {URI.class, URI.class, HttpProductionTransport.BearerCredential.class,
                    HttpProductionTransport.AgentSessionCredential.class, String.class, String.class,
                    String.class, ProtocolAmount.class, String.class, IdempotencyKey.class},
                human, agent, bearer, session, "tenant", "did:layerx:alice", "asset",
                ProtocolAmount.parse("10"), "100", key)));
            assertEquals(PlatformSdkException.Code.PROTOCOL_INCOMPATIBILITY, error.code());
            assertEquals(PlatformSdkException.Retry.NEVER, error.retry());
            assertEquals(1, requests.size());
            assertEquals(List.of("/v1/agent/rpc"), paths);
            assertEquals(List.of("Bearer public-loopback-bearer"), authorization);
            JsonNode request = requests.getFirst();
            assertEquals(2, request.required("version").intValue());
            assertEquals("budget.create", request.required("operation").textValue());
            assertEquals(key.value(), request.required("idempotency_key").textValue());
            JsonNode credential = request.required("credential");
            assertEquals("tenant", credential.required("tenant").textValue());
            assertEquals("11".repeat(32), credential.required("session_id").textValue());
            assertEquals("22".repeat(32), credential.required("token_id").textValue());
            assertEquals("7", credential.required("generation").textValue());
            JsonNode body = request.required("request");
            assertEquals(JSON.readTree("{\"tenant\":\"tenant\",\"agent_did\":\"did:layerx:alice\","
                + "\"asset\":\"asset\",\"limit\":\"10\",\"enforcement\":\"DaemonLimit\",\"expiry\":\"100\"}"), body);
            assertNull(handlerFailure.get());
            assertEquals("PolicyRefusal", JSON.readTree(refusal).required("class").textValue());
            assertArrayEquals(refusal, Files.readAllBytes(root.resolve(
                "services/agent/schema/agent-api/golden/agent-http-refusal.json")));
        } finally { server.stop(0); }
    }

    @Test
    void configuredOriginsAndRandomKeysSatisfyTheSupportedBoundaries() throws Exception {
        var generated = new java.util.HashSet<String>();
        for (int i = 0; i < 64; i++) {
            IdempotencyKey key = randomKey();
            assertTrue(key.value().matches("[0-9a-f]{64}"));
            assertTrue(generated.add(key.value()), "each operation receives a fresh random key");
        }
        for (String method : List.of("nodeOrigin", "serviceOrigin")) {
            for (String origin : List.of("https://api1.mainnet-beta.paxeer.network", "https://deployment.example",
                    "http://127.0.0.1:1234")) {
                assertEquals(URI.create(origin), call("ExampleConfiguration", method, new Class<?>[] {String.class}, origin));
            }
            for (String invalid : List.of("https://deployment.example/rpc", "https://deployment.example/v1/agent/rpc",
                    "https://token@deployment.example", "https://deployment.example?token=x", "http://deployment.example")) {
                assertEquals(PlatformSdkException.Code.INVALID_ARGUMENT, refused(() -> call("ExampleConfiguration",
                    method, new Class<?>[] {String.class}, invalid)).code());
            }
        }
        assertEquals(PlatformSdkException.Code.INVALID_ARGUMENT, refused(() -> call("ExampleConfiguration", "session",
            new Class<?>[] {String.class, String.class, String.class, String.class},
            "tenant", "short", "22".repeat(32), "7")).code());
    }

    private static JsonNode fixture(String name) throws Exception { return JSON.readTree(Files.readAllBytes(root.resolve(name))); }

    private static JsonNode transfer() throws Exception {
        JsonNode document = fixture("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");
        assertEquals("rpc/sequencer_receipt_vectors_test.go", document.required("provenance").required("generator").textValue());
        return java.util.stream.StreamSupport.stream(document.required("cases").spliterator(), false)
            .filter(vector -> "V1-pax-transfer".equals(vector.required("id").textValue())).findFirst().orElseThrow();
    }

    private static byte[] bytes(JsonNode node, String field) { return HexFormat.of().parseHex(node.required(field).textValue().substring(2)); }

    private static LocalVerifier.ReceiptKey key(JsonNode vector) {
        JsonNode pin = vector.required("pinnedKey");
        return new LocalVerifier.ReceiptKey(bytes(pin, "keyId"), bytes(pin, "publicKey"),
            new BigInteger(pin.required("firstHeight").asText()), new BigInteger(pin.required("lastHeight").asText()));
    }

    private static LocalVerifier.ReceiptKeys keys(JsonNode vector) { return new LocalVerifier.ReceiptKeys(List.of(key(vector))); }

    private static ConsensusTrust trust() {
        return new ConsensusTrust("committed-fixture", 10,
            Base64.getDecoder().decode("ZawkHY2ZdOJMgK5e+LSb009M3VIrCogeqWCijke7bCk="));
    }

    private static Object payment(BigInteger amount) throws Exception {
        return paymentType.getConstructor(String.class, BigInteger.class, BigInteger.class, BigInteger.class)
            .newInstance(RECIPIENT, amount, BigInteger.valueOf(3_000_000_000L), BigInteger.valueOf(1_000_000_000L));
    }

    private static IdempotencyKey randomKey() throws Exception {
        return (IdempotencyKey) call("ExampleConfiguration", "randomKey", new Class<?>[0]);
    }

    private static PortableReceipt.Verification offline(Path proof, Path raw, LocalVerifier.ReceiptKeys keys,
            ConsensusTrust trust) throws Exception {
        return (PortableReceipt.Verification) call("OfflineVerificationExample", "verify",
            new Class<?>[] {Path.class, Path.class, LocalVerifier.ReceiptKeys.class, ConsensusTrust.class, List.class},
            proof, raw, keys, trust, List.of());
    }

    private static Object call(String type, String method, Class<?>[] signature, Object... args) throws Exception {
        try {
            return examples.loadClass(PACKAGE + type).getMethod(method, signature).invoke(null, args);
        } catch (InvocationTargetException error) {
            if (error.getCause() instanceof Exception cause) throw cause;
            throw (Error) error.getCause();
        }
    }

    private static Object await(Object stage) throws Exception {
        return ((CompletionStage<?>) stage).toCompletableFuture().get(10, TimeUnit.SECONDS);
    }

    private static PlatformSdkException refused(org.junit.jupiter.api.function.Executable action) {
        Throwable error = assertThrows(Throwable.class, action);
        while (error instanceof java.util.concurrent.ExecutionException || error instanceof java.util.concurrent.CompletionException) {
            error = error.getCause();
        }
        return assertInstanceOf(PlatformSdkException.class, error);
    }

    private static final class RecordedNode implements AutoCloseable {
        final HttpServer server;
        final JsonNode vector;
        final List<JsonNode> requests = new CopyOnWriteArrayList<>();
        final List<String> paths = new CopyOnWriteArrayList<>();
        final List<String> authorization = new CopyOnWriteArrayList<>();
        final AtomicReference<Throwable> failure = new AtomicReference<>();
        volatile String chain = "0x7d";

        RecordedNode(JsonNode vector) throws Exception {
            this.vector = vector;
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", exchange -> {
                try {
                    JsonNode request = JSON.readTree(exchange.getRequestBody().readAllBytes());
                    requests.add(request);
                    paths.add(exchange.getRequestURI().getPath());
                    String auth = exchange.getRequestHeaders().getFirst("Authorization");
                    authorization.add(auth == null ? "" : auth);
                    assertEquals("POST", exchange.getRequestMethod());
                    assertEquals("/", exchange.getRequestURI().getPath());
                    Object result = switch (request.required("method").textValue()) {
                        case "eth_chainId" -> chain;
                        case "eth_getTransactionCount" -> {
                            assertEquals(List.of(SENDER, "pending"), JSON.convertValue(request.required("params"), List.class));
                            yield vector.required("receiptJson").required("nonce");
                        }
                        case "eth_estimateGas" -> {
                            JsonNode tx = request.required("params").get(0);
                            assertEquals("0x2", tx.required("type").textValue());
                            assertEquals("0x7d", tx.required("chainId").textValue());
                            assertEquals(RECIPIENT, tx.required("to").textValue());
                            assertEquals(SENDER, tx.required("from").textValue());
                            assertEquals("0xde0b6b3a7640000", tx.required("value").textValue());
                            assertEquals("0x3b9aca00", tx.required("maxPriorityFeePerGas").textValue());
                            assertEquals("0xb2d05e00", tx.required("maxFeePerGas").textValue());
                            assertEquals("0x", tx.required("data").textValue());
                            assertTrue(tx.required("accessList").isEmpty());
                            assertEquals("pending", request.required("params").get(1).textValue());
                            yield "0x5208";
                        }
                        case "eth_sendRawTransaction" -> {
                            assertEquals(vector.required("rawTransaction"), request.required("params").get(0));
                            yield vector.required("receiptJson").required("txHash");
                        }
                        case "pax_sequencerReceipt" -> {
                            assertEquals(vector.required("receiptJson").required("txHash"), request.required("params").get(0));
                            yield vector.required("receiptJson");
                        }
                        default -> throw new AssertionError("unexpected RPC method " + request);
                    };
                    ObjectNode response = JSON.createObjectNode().put("jsonrpc", "2.0");
                    response.set("id", request.required("id"));
                    response.set("result", JSON.valueToTree(result));
                    byte[] encoded = JSON.writeValueAsBytes(response);
                    exchange.getResponseHeaders().set("Content-Type", "application/json");
                    exchange.sendResponseHeaders(200, encoded.length);
                    exchange.getResponseBody().write(encoded);
                } catch (Throwable error) {
                    failure.set(error);
                    exchange.sendResponseHeaders(500, -1);
                } finally { exchange.close(); }
            });
            server.start();
        }

        URI uri() { return URI.create("http://127.0.0.1:" + server.getAddress().getPort()); }
        List<String> methods() { return requests.stream().map(request -> request.required("method").textValue()).toList(); }
        @Override public void close() { server.stop(0); }
    }
}
