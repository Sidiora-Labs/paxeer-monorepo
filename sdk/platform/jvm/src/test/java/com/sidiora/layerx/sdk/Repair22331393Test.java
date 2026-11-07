package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.ConsensusTrust;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.PortableReceipt;
import java.io.StringWriter;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.net.URLClassLoader;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Base64;
import java.util.HexFormat;
import java.util.List;
import javax.tools.ToolProvider;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import static org.junit.jupiter.api.Assertions.*;

public final class Repair22331393Test {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final String FIXTURES = "sdk/platform/conformance/fixtures/";
    private static final String NODE = "daemon/committed-fixture/vectors/vector-v4-committed.json";
    @TempDir static Path classes;
    @TempDir Path temporary;
    private static Path root;
    private static URLClassLoader loader;
    private static Class<?> conformance;

    @BeforeAll
    static void loadRealConformanceEntryPoint() throws Exception {
        root = Path.of("").toAbsolutePath();
        while (root != null && !Files.isRegularFile(root.resolve(FIXTURES + "vector-v4-committed.json"))) {
            root = root.getParent();
        }
        assertNotNull(root, "Committed producer fixtures must be available");
        var compiler = ToolProvider.getSystemJavaCompiler();
        assertNotNull(compiler, "The Maven JDK must expose its in-process compiler");
        String classpath = System.getProperty("surefire.test.class.path");
        assertNotNull(classpath, "Surefire must supply the production test classpath");
        Path source = root.resolve("sdk/platform/jvm/src/conformance/java/com/sidiora/layerx/sdk/ConformanceMain.java");
        StringWriter diagnostics = new StringWriter();
        try (var manager = compiler.getStandardFileManager(null, null, null)) {
            boolean compiled = compiler.getTask(diagnostics, manager, null,
                List.of("--release", "21", "-classpath", classpath, "-d", classes.toString()), null,
                manager.getJavaFileObjects(source.toFile())).call();
            assertTrue(compiled, diagnostics.toString());
        }
        loader = new URLClassLoader(new java.net.URL[] {classes.toUri().toURL()},
            Repair22331393Test.class.getClassLoader());
        conformance = Class.forName("com.sidiora.layerx.sdk.ConformanceMain", true, loader);
    }

    @AfterAll
    static void closeConformanceClasses() throws Exception {
        if (loader != null) loader.close();
    }

    @Test
    void producerFullResponsesAndSeparateFeeAssetVerify() throws Exception {
        invoke("invokeLocalVerification", root);
        assertEquals(5, invoke("verifyReceiptViews", root));
        JsonNode fee = read(root.resolve(FIXTURES + "sequencer-receipt-fee-token-v1.json"))
            .required("cases").required(0);
        var verified = LocalVerifier.verifySequencerReceipt(bytes(fee, "signedReceipt"),
            bytes(fee, "rawTransaction"), keys(fee.required("pinnedKey")));
        assertEquals("usid", fee.required("paymentDenom").textValue());
        assertEquals("uhpx", fee.required("feeDenom").textValue());
        assertArrayEquals(bytes(fee, "asset"), verified.receipt().asset());
        assertArrayEquals(bytes(fee, "receiptDigest"), verified.receiptDigest());
        assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, verified.level());
        assertEquals(1, verified.status());
    }

    @Test
    void realRpcCommitmentAndPortableStateRecoverAfterClientReopen() throws Exception {
        Path recovery = temporary.resolve("recovery");
        JsonNode result = (JsonNode) invoke("verifyRpcReceiptLifecycle", root, recovery);
        assertEquals(4, result.required("rpcReads").intValue());
        assertEquals("committed", result.required("recoveredLevel").textValue());
        assertEquals(10, result.required("height").longValue());
        assertEquals(2, result.required("portableBundles").intValue());
        JsonNode vector = read(root.resolve(FIXTURES + "vector-v4-committed.json")).required("positive");
        JsonNode recorded = read(root.resolve(NODE)).required("settlement").required("receipt");
        byte[] saved = Files.readAllBytes(recovery.resolve("committed-receipt.json"));
        byte[] raw = Files.readAllBytes(recovery.resolve("raw-transaction.bin"));
        JsonNode pin = read(recovery.resolve("receipt-key.json"));
        assertEquals(vector.required("pinnedKey"), pin);
        assertArrayEquals(bytes(vector, "rawTransaction"), raw);
        assertEquals(recorded, JSON.readTree(saved).required("receiptJson"));
        assertEquals(vector.required("receiptDigest"), result.required("receiptDigest"));
        var recovered = PortableReceipt.verify(saved, keys(pin), trust());
        assertEquals("committed", recovered.verificationLevel());
        assertEquals(10, recovered.committed().height());
        assertArrayEquals(bytes(recorded, "blockHash"), recovered.committed().blockHash());
        assertArrayEquals(bytes(vector, "signedReceipt"), recovered.receipted().canonicalBytes());
        assertArrayEquals(bytes(vector, "receiptDigest"), recovered.receipted().receiptDigest());
        assertArrayEquals(saved, PortableReceipt.export(recovered));
        assertEquals(1, recovered.status());
        assertDoesNotThrow(recovered::requireSuccess);
        assertRefusal(() -> PortableReceipt.verify(saved, keys(pin)));
        assertRefusal(() -> PortableReceipt.verify(saved, keys(pin), new ConsensusTrust("wrong-chain", 10,
            Base64.getDecoder().decode("ZawkHY2ZdOJMgK5e+LSb009M3VIrCogeqWCijke7bCk="))));
        ObjectNode changed = JSON.readTree(saved).deepCopy();
        ((ObjectNode) changed.required("receiptJson")).put("blockHash", "0x" + "01".repeat(32));
        assertRefusal(() -> PortableReceipt.verify(JSON.writeValueAsBytes(changed), keys(pin), trust()));
    }

    @Test
    void completeReceiptViewMismatchesFailTheConformanceEntryPoint() throws Exception {
        Path corpus = copyCorpus();
        Path path = corpus.resolve(FIXTURES + "vector-v1-receipted-positive.json");
        JsonNode original = read(path);
        for (int index = 0; index < original.required("cases").size(); index++) {
            for (String field : List.of("status", "returnData", "feeAmount", "paymentDenom", "feeDenom", "logs")) {
                ObjectNode altered = original.deepCopy();
                ObjectNode view = (ObjectNode) altered.required("cases").required(index).required("receiptJson");
                switch (field) {
                    case "status", "feeAmount" -> view.put(field, "0x0");
                    case "returnData" -> view.put(field, "0x01");
                    case "paymentDenom", "feeDenom" -> view.put(field, "usid");
                    case "logs" -> view.remove(field);
                    default -> throw new AssertionError(field);
                }
                Files.write(path, JSON.writeValueAsBytes(altered));
                assertRefusal(() -> invoke("invokeLocalVerification", corpus));
            }
        }
    }

    @Test
    void contradictoryCommittedRpcMetadataFailsBeforeRecovery() throws Exception {
        Path corpus = copyCorpus();
        Path path = corpus.resolve(NODE);
        JsonNode original = read(path);
        for (String field : List.of("blockNumber", "blockHash", "feeDenom", "logs")) {
            ObjectNode altered = original.deepCopy();
            ObjectNode view = (ObjectNode) altered.required("settlement").required("receipt");
            switch (field) {
                case "blockNumber" -> view.put(field, "0x9");
                case "blockHash" -> view.put(field, "0x" + "01".repeat(32));
                case "feeDenom" -> view.put(field, "usid");
                case "logs" -> ((ObjectNode) view.required("logs").required(0)).put("data", "0x01");
                default -> throw new AssertionError(field);
            }
            Files.write(path, JSON.writeValueAsBytes(altered));
            assertRefusal(() -> invoke("verifyRpcReceiptLifecycle", corpus, temporary.resolve(field)));
        }
    }

    private Path copyCorpus() throws Exception {
        Path corpus = temporary.resolve("corpus");
        for (String name : List.of(FIXTURES + "vector-v1-receipted-positive.json",
                FIXTURES + "sequencer-receipt-fee-token-v1.json", FIXTURES + "vector-v4-committed.json",
                FIXTURES + "vector-v5-portable.json", NODE)) {
            Path target = corpus.resolve(name);
            Files.createDirectories(target.getParent());
            Files.copy(root.resolve(name), target);
        }
        return corpus;
    }

    private static Object invoke(String name, Path... arguments) throws Exception {
        Class<?>[] types = new Class<?>[arguments.length];
        java.util.Arrays.fill(types, Path.class);
        Method method = conformance.getDeclaredMethod(name, types);
        method.setAccessible(true);
        try {
            return method.invoke(null, (Object[]) arguments);
        } catch (InvocationTargetException error) {
            if (error.getCause() instanceof Exception cause) throw cause;
            if (error.getCause() instanceof Error cause) throw cause;
            throw new AssertionError(error.getCause());
        }
    }

    private static void assertRefusal(org.junit.jupiter.api.function.Executable action) {
        Throwable error = assertThrows(Exception.class, action);
        while (error instanceof java.util.concurrent.ExecutionException) error = error.getCause();
        var sdk = assertInstanceOf(PlatformSdkException.class, error);
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, sdk.code());
    }

    private static LocalVerifier.ReceiptKeys keys(JsonNode pin) {
        return new LocalVerifier.ReceiptKeys(List.of(new LocalVerifier.ReceiptKey(bytes(pin, "keyId"),
            bytes(pin, "publicKey"), pin.required("firstHeight").bigIntegerValue(),
            pin.required("lastHeight").bigIntegerValue())));
    }

    private static ConsensusTrust trust() {
        return new ConsensusTrust("committed-fixture", 10,
            Base64.getDecoder().decode("ZawkHY2ZdOJMgK5e+LSb009M3VIrCogeqWCijke7bCk="));
    }

    private static byte[] bytes(JsonNode node, String field) {
        return HexFormat.of().parseHex(node.required(field).textValue().substring(2));
    }

    private static JsonNode read(Path path) throws Exception {
        return JSON.readTree(Files.readAllBytes(path));
    }
}
