package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.Committed;
import com.sidiora.layerx.sdk.verify.ConsensusTrust;
import com.sidiora.layerx.sdk.verify.PortableReceipt;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sun.net.httpserver.HttpServer;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.net.URI;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.util.HexFormat;
import java.util.Arrays;
import java.util.Base64;
import java.util.Comparator;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Pattern;

/** Schema-golden conformance gate, compiled only by the conformance Maven profile. */
public final class ConformanceMain {
    private static final ObjectMapper JSON = new ObjectMapper();
    private ConformanceMain() {}

    public static void main(String[] arguments) throws Exception {
        Path root = arguments.length == 0 ? Path.of("../../..") : Path.of(arguments[0]);
        Path golden = root.resolve("services/human/schema/human-api/golden");
        for (Map.Entry<String, OperationCatalog.Route> operation : OperationCatalog.HUMAN_ROUTES.entrySet()) {
            verifyTriplet(golden, operation.getKey(), operation.getValue());
        }
        int agentGoldenValues = verifyAgentGoldens(root.resolve("services/agent/schema/agent-api/golden"));
        verifySchemaParity();
        verifyProtocolIntegers();
        verifyStreamAndSecrets();
        invokeLocalVerification(root);
        Path recovery = Files.createTempDirectory("layerx-receipt-conformance-");
        try {
            verifyRpcReceiptLifecycle(root, recovery);
        } finally {
            try (var files = Files.walk(recovery)) {
                for (Path file : files.sorted(Comparator.reverseOrder()).toList()) Files.delete(file);
            }
        }
        Set<String> sdkCodes = new HashSet<>();
        for (PlatformSdkException.Code code : PlatformSdkException.Code.values()) {
            if (!sdkCodes.add(code.wire())) throw new IllegalStateException("duplicate SDK error code");
        }
        System.out.printf("agent_operations=%d human_operations=%d human_golden_triplets=%d agent_goldens=%d sdk_error_codes=%d%n",
            OperationCatalog.AGENT_OPERATIONS.size(), OperationCatalog.HUMAN_ROUTES.size(),
            OperationCatalog.HUMAN_ROUTES.size(), agentGoldenValues, sdkCodes.size());
    }

    private static void verifyTriplet(Path golden, String operation, OperationCatalog.Route route) throws IOException {
        Path prefix = golden.resolve(operation);
        JsonNode request = read(prefix.resolveSibling(prefix.getFileName() + ".request.json"));
        if (!route.method().equals(text(request, "method")) || !pathMatches(route.path(), text(request, "path"))) {
            throw new IllegalStateException(operation + " request method/path diverges from schema");
        }
        JsonNode headers = request.path("headers");
        if (route.idempotency() && (!headers.isObject() || headers.path("Idempotency-Key").asText().isEmpty())) {
            throw new IllegalStateException(operation + " request omits Idempotency-Key");
        }
        rejectNumericMoney(request.path("body"), "body");
        verifySuccess(operation, read(prefix.resolveSibling(prefix.getFileName() + ".response.json")));
        verifyFailure(operation, read(prefix.resolveSibling(prefix.getFileName() + ".failure.json")));
    }

    private static void verifySuccess(String operation, JsonNode vector) {
        int status = vector.path("status").asInt(-1);
        JsonNode body = vector.path("body");
        if (status < 200 || status >= 300 || !body.path("ok").asBoolean(false)
                || body.path("trace").asText().isEmpty() || !body.has("result") || body.has("error")) {
            throw new IllegalStateException(operation + " success vector is not a typed success envelope");
        }
        rejectNumericMoney(body.path("result"), "result");
    }

    private static void verifyFailure(String operation, JsonNode vector) {
        int status = vector.path("status").asInt(-1);
        JsonNode body = vector.path("body"), error = body.path("error");
        String retry = error.path("retry").asText();
        if ((status >= 200 && status < 300) || body.path("ok").asBoolean(true)
                || body.path("trace").asText().isEmpty()
                || !OperationCatalog.HUMAN_ERROR_CODES.contains(error.path("code").asText())
                || !OperationCatalog.HUMAN_RETRIABILITY.contains(retry)) {
            throw new IllegalStateException(operation + " failure vector is not a typed failure envelope");
        }
        SchemaErrors.HumanCode.fromWire(error.path("code").asText());
        SchemaErrors.HumanRetriability.fromWire(retry);
        if (retry.equals("retriable-after") && !error.path("retry_after_ms").canConvertToLong()) {
            throw new IllegalStateException(operation + " retriable-after failure omits retry_after_ms");
        }
    }

    private static int verifyAgentGoldens(Path root) throws IOException {
        int verified = 0;
        List<Path> schemaFiles;
        try (var paths = Files.list(root)) {
            schemaFiles = paths.filter(file -> file.toString().endsWith(".kvx")).sorted().toList();
        }
        for (Path path : schemaFiles) {
            String section = null;
            for (String line : Files.readAllLines(path)) {
                String trimmed = line.trim();
                if (trimmed.startsWith("[golden.") && trimmed.endsWith("]")) section = trimmed;
                if (section != null && trimmed.startsWith("encoded_hex = \"") && trimmed.endsWith("\"")) {
                    String encoded = trimmed.substring(15, trimmed.length() - 1);
                    byte[] bytes = HexFormat.of().parseHex(encoded);
                    if (bytes.length == 0) throw new IllegalStateException(path + " " + section + " is empty");
                    if (bytes[0] != '{') {
                        boolean versionVector = section.contains(".version")
                            && (bytes.length == 19 || bytes.length == 23)
                            && bytes[0] == 0 && bytes[1] == 1;
                        if (!versionVector) throw new IllegalStateException(path + " " + section + " is not canonical Agent wire data");
                        verified++;
                        continue;
                    }
                    JsonNode value = JSON.readTree(bytes);
                    if (value == null || !value.isObject() || !MessageDigest.isEqual(bytes, JSON.writeValueAsBytes(value))) {
                        throw new IllegalStateException(path + " " + section + " is not canonical JSON");
                    }
                    rejectNumericMoney(value, section);
                    verified++;
                }
            }
        }
        if (verified == 0) throw new IllegalStateException("agent golden corpus is empty");
        return verified;
    }

    private static void verifySchemaParity() {
        if (!GeneratedSchema.AGENT.keySet().equals(OperationCatalog.AGENT_OPERATIONS)
                || !GeneratedSchema.HUMAN.keySet().equals(OperationCatalog.HUMAN_ROUTES.keySet())) {
            throw new IllegalStateException("generated typed operation catalogue diverges from live schemas");
        }
        if (!OperationCatalog.AGENT_ERROR_CLASSES.equals(wires(SchemaErrors.AgentClass.values()))
                || !OperationCatalog.AGENT_RETRIABILITY.equals(wires(SchemaErrors.AgentRetriability.values()))
                || !OperationCatalog.HUMAN_ERROR_CODES.equals(wires(SchemaErrors.HumanCode.values()))
                || !OperationCatalog.HUMAN_RETRIABILITY.equals(wires(SchemaErrors.HumanRetriability.values()))) {
            throw new IllegalStateException("JVM error taxonomy diverges from generated schema metadata");
        }
        OperationCatalog.AGENT_OPERATIONS.forEach(name -> {
            if (!OperationCatalog.agent(name).wireName().equals(name)) throw new IllegalStateException("agent operation drift");
        });
        OperationCatalog.HUMAN_ROUTES.forEach((name, route) -> {
            if (!OperationCatalog.human(name).route().equals(route)) throw new IllegalStateException("human operation drift");
        });
    }

    private static Set<String> wires(SchemaErrors.WireValue[] values) {
        Set<String> wires = new HashSet<>();
        for (SchemaErrors.WireValue value : values) wires.add(value.wire());
        return Set.copyOf(wires);
    }

    private static void verifyProtocolIntegers() throws IOException {
        if (!SchemaTypes.protocolInteger(JSON.readTree("\"340282366920938463463374607431768211455\"")).equals(
                ProtocolAmount.MAX_VALUE)) throw new IllegalStateException("u128 maximum changed");
        for (String rejected : List.of("1.0", "1", "\"01\"", "\"-1\"", "\"340282366920938463463374607431768211456\"")) {
            try {
                SchemaTypes.protocolInteger(JSON.readTree(rejected));
                throw new IllegalStateException("non-canonical protocol integer accepted: " + rejected);
            } catch (PlatformSdkException expected) {
                if (expected.code() != PlatformSdkException.Code.INVALID_ARGUMENT) throw expected;
            }
        }
    }

    private static void verifyStreamAndSecrets() {
        var initial = new ResumableStream.Cursor("cursor-0");
        var next = new ResumableStream.Cursor("cursor-1");
        var stream = new ResumableStream<String>(initial);
        var event = new ResumableStream.Event<>("event-1", initial, next, "value");
        if (!stream.accept(new ResumableStream.Page<>(initial, List.of(event), next)).equals(List.of(event))
                || !stream.cursor().equals(next)) throw new IllegalStateException("cursor did not advance atomically");
        try {
            stream.accept(new ResumableStream.Page<>(next, List.of(event), next));
            throw new IllegalStateException("duplicate stream event accepted");
        } catch (PlatformSdkException expected) {
            if (expected.code() != PlatformSdkException.Code.DECODE_FAILURE) throw expected;
        }
        byte[] input = "conformance-secret".getBytes(java.nio.charset.StandardCharsets.UTF_8);
        SecretBytes secret = new SecretBytes(input);
        input[0] = 0;
        secret.close();
        if (!secret.isDestroyed() || !secret.toString().equals("[REDACTED]")) {
            throw new IllegalStateException("secret lifecycle is not fail-closed");
        }
    }

    private static void invokeLocalVerification(Path repositoryRoot) throws Exception {
        byte[] leaf = "layerx-jvm-conformance".getBytes(java.nio.charset.StandardCharsets.UTF_8);
        byte[] root = MessageDigest.getInstance("SHA-256").digest(concat(
            "LXP/v1/merkle-leaf\0".getBytes(java.nio.charset.StandardCharsets.UTF_8), leaf));
        LocalVerifier.verifyMerkleInclusion(leaf, new LocalVerifier.MerkleProof(0, 1, List.of()), root);
        JsonNode vector = read(repositoryRoot.resolve(
            "sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json")).required("cases").required(0);
        JsonNode pin = vector.required("pinnedKey");
        var keys = new LocalVerifier.ReceiptKeys(List.of(new LocalVerifier.ReceiptKey(
            HexFormat.of().parseHex(text(pin, "keyId").substring(2)),
            HexFormat.of().parseHex(text(pin, "publicKey").substring(2)),
            pin.required("firstHeight").bigIntegerValue(), pin.required("lastHeight").bigIntegerValue())));
        byte[] rawTransaction = HexFormat.of().parseHex(text(vector, "rawTransaction").substring(2));
        var verified = LocalVerifier.verifySequencerReceipt(
            HexFormat.of().parseHex(text(vector, "signedReceipt").substring(2)), rawTransaction, keys);
        if (verified.level() != LocalVerifier.VerificationLevel.RECEIPTED || verified.status() != 1) {
            throw new IllegalStateException("valid sequencer receipt did not verify as receipted success");
        }
        expectVerificationFailure(() -> LocalVerifier.verifySequencerReceipt(new byte[] {0}, rawTransaction, keys));
        verifyReceiptViews(repositoryRoot);
    }

    public static int verifyReceiptViews(Path repositoryRoot) throws Exception {
        JsonNode cases = read(repositoryRoot.resolve(
            "sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json")).required("cases");
        require(cases.isArray() && cases.size() == 5, "producer receipt corpus changed");
        int checked = 0;
        for (JsonNode vector : cases) {
            var key = receiptKey(vector);
            var keys = new LocalVerifier.ReceiptKeys(List.of(key));
            byte[] raw = bytes(vector, "rawTransaction");
            JsonNode view = vector.required("receiptJson");
            var verified = ProgramsClient.verifyReceipt(bytes(vector, "signedReceipt"), raw, keys, view);
            checkReceipt(vector, verified);
            byte[] portable = PortableReceipt.export(raw, JSON.writeValueAsBytes(view), key);
            var offline = PortableReceipt.verify(portable, keys);
            require(offline.verificationLevel().equals("receipted")
                && JSON.readTree(portable).required("receiptJson").equals(view)
                && Arrays.equals(portable, PortableReceipt.export(offline)), "full receipt view changed");
            checked++;
        }
        JsonNode fee = read(repositoryRoot.resolve(
            "sdk/platform/conformance/fixtures/sequencer-receipt-fee-token-v1.json")).required("cases").required(0);
        var feeReceipt = LocalVerifier.verifySequencerReceipt(bytes(fee, "signedReceipt"),
            bytes(fee, "rawTransaction"), new LocalVerifier.ReceiptKeys(List.of(receiptKey(fee))));
        checkReceipt(fee, feeReceipt);
        require(text(fee, "paymentDenom").equals("usid") && text(fee, "feeDenom").equals("uhpx")
            && Arrays.equals(bytes(fee, "asset"), feeReceipt.receipt().asset()), "payment asset changed");
        byte[] summary = feeReceipt.receipt().effects().getFirst().body();
        require(summary.length == 113 && summary[108] == 4
            && Arrays.equals(Arrays.copyOfRange(summary, 109, 113),
                "uhpx".getBytes(java.nio.charset.StandardCharsets.UTF_8)), "signed fee denomination changed");
        return checked;
    }

    public static JsonNode verifyRpcReceiptLifecycle(Path repositoryRoot, Path recoveryDirectory) throws Exception {
        JsonNode vector = read(repositoryRoot.resolve(
            "sdk/platform/conformance/fixtures/vector-v4-committed.json")).required("positive");
        JsonNode recorded = read(repositoryRoot.resolve(
            "daemon/committed-fixture/vectors/vector-v4-committed.json")).required("settlement");
        JsonNode before = vector.required("receiptJson");
        JsonNode after = recorded.required("receipt");
        byte[] raw = bytes(vector, "rawTransaction");
        var key = receiptKey(vector);
        var keys = new LocalVerifier.ReceiptKeys(List.of(key));
        require(text(before, "rung").equals("receipted") && text(after, "rung").equals("committed")
            && before.required("receipt").equals(after.required("receipt"))
            && Arrays.equals(raw, bytes(recorded, "raw_transaction")), "recorded lifecycle changed");
        Files.createDirectories(recoveryDirectory);
        Files.write(recoveryDirectory.resolve("receipt-key.json"), JSON.writeValueAsBytes(vector.required("pinnedKey")));
        Files.write(recoveryDirectory.resolve("raw-transaction.bin"), raw);
        byte[] committedBundle;
        try (var archive = new RecordedReceiptArchive(before)) {
            try (var connection = new ProgramsClient.Programs(archive.uri())) {
                var client = receiptClient(connection, key);
                checkReceipt(vector, client.sequencerReceipt(raw).toCompletableFuture().get(10, TimeUnit.SECONDS));
                archive.receipt.set(after);
                var received = client.sequencerReceipt(raw).toCompletableFuture().get(10, TimeUnit.SECONDS);
                checkReceipt(vector, received);
                require(received.level() == LocalVerifier.VerificationLevel.RECEIPTED,
                    "RPC metadata alone must not grant consensus verification");
                JsonNode delivered = connection.request("pax_sequencerReceipt", JSON.createArrayNode()
                    .add(after.required("txHash"))).toCompletableFuture().get(10, TimeUnit.SECONDS);
                require(after.equals(delivered), "RPC changed the node receipt view");
                committedBundle = PortableReceipt.export(raw, JSON.writeValueAsBytes(delivered), key,
                    Committed.decodeHeader(vector.required("blockHeader")),
                    Committed.decodeInclusion(vector.required("receiptInclusion")),
                    Committed.decodeCommit(vector.required("commit")),
                    Committed.decodeValidators(vector.required("validatorSet")), consensusTrust());
                Files.write(recoveryDirectory.resolve("committed-receipt.json"), committedBundle);
            }
            byte[] recoveredRaw = Files.readAllBytes(recoveryDirectory.resolve("raw-transaction.bin"));
            ObjectNode recoveredVector = JSON.createObjectNode();
            recoveredVector.set("pinnedKey", read(recoveryDirectory.resolve("receipt-key.json")));
            var recoveredKey = receiptKey(recoveredVector);
            try (var reopened = new ProgramsClient.Programs(archive.uri())) {
                var recovered = receiptClient(reopened, recoveredKey).sequencerReceipt(recoveredRaw)
                    .toCompletableFuture().get(10, TimeUnit.SECONDS);
                checkReceipt(vector, recovered);
                require(recovered.level() == LocalVerifier.VerificationLevel.RECEIPTED,
                    "reopened RPC client overstated verification");
            }
            byte[] saved = Files.readAllBytes(recoveryDirectory.resolve("committed-receipt.json"));
            var recovered = PortableReceipt.verify(saved,
                new LocalVerifier.ReceiptKeys(List.of(recoveredKey)), consensusTrust());
            require(recovered.verificationLevel().equals("committed") && recovered.status() == 1
                && recovered.committed().height() == 10
                && Arrays.equals(recovered.committed().blockHash(), bytes(after, "blockHash"))
                && Arrays.equals(recovered.receipted().receiptDigest(), bytes(vector, "receiptDigest"))
                && JSON.readTree(saved).required("receiptJson").equals(after)
                && Arrays.equals(saved, PortableReceipt.export(recovered))
                && Arrays.equals(saved, committedBundle), "committed receipt did not recover");
            expectVerificationFailure(() -> PortableReceipt.verify(saved, keys));
            require(archive.reads.get() == 4, "RPC lifecycle did not traverse every read");
            JsonNode bundles = read(repositoryRoot.resolve(
                "sdk/platform/conformance/fixtures/vector-v5-portable.json")).required("bundles");
            JsonNode receiptedVector = read(repositoryRoot.resolve(
                "sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json")).required("cases").required(0);
            var portableKeys = new LocalVerifier.ReceiptKeys(List.of(key, receiptKey(receiptedVector)));
            Set<String> levels = new HashSet<>();
            for (JsonNode bundle : bundles) {
                byte[] encoded = JSON.writeValueAsBytes(bundle);
                var verified = PortableReceipt.verify(encoded, portableKeys, consensusTrust());
                require(levels.add(verified.verificationLevel()) && verified.status() == 1
                    && Arrays.equals(encoded, PortableReceipt.export(verified)), "published portable bundle changed");
            }
            require(levels.equals(Set.of("receipted", "committed")), "portable lifecycle rung missing");
            return JSON.createObjectNode().put("rpcReads", archive.reads.get()).put("recoveredLevel", "committed")
                .put("height", recovered.committed().height()).put("portableBundles", bundles.size())
                .put("receiptDigest", "0x" + HexFormat.of().formatHex(recovered.receipted().receiptDigest()));
        }
    }

    private static ConsensusTrust consensusTrust() {
        return new ConsensusTrust("committed-fixture", 10,
            Base64.getDecoder().decode("Zc6vQFr0ZD2oXZPWyqjKikf4CkF01FQdfT9YQBmBrKI="));
    }

    private static ProgramsClient receiptClient(ProgramsClient.Programs connection, LocalVerifier.ReceiptKey key) {
        return new ProgramsClient(connection, "0xc08e711fe101411a75cc8eb8cbb26dd0efac584e",
            BigInteger.ZERO, BigInteger.ONE, new LocalVerifier.ReceiptKeys(List.of(key)), key.keyId());
    }

    private static LocalVerifier.ReceiptKey receiptKey(JsonNode vector) {
        JsonNode pin = vector.required("pinnedKey");
        return new LocalVerifier.ReceiptKey(bytes(pin, "keyId"), bytes(pin, "publicKey"),
            pin.required("firstHeight").bigIntegerValue(), pin.required("lastHeight").bigIntegerValue());
    }

    private static void checkReceipt(JsonNode vector, LocalVerifier.SequencerReceiptVerification verified) {
        require(verified.level() == LocalVerifier.VerificationLevel.RECEIPTED && verified.status() == 1
            && Arrays.equals(bytes(vector, "signedReceipt"), verified.canonicalBytes())
            && Arrays.equals(bytes(vector, "receiptDigest"), verified.receiptDigest()), "receipt verification diverged");
        verified.requireSuccess();
    }

    private static byte[] bytes(JsonNode node, String field) {
        return HexFormat.of().parseHex(text(node, field).substring(2));
    }

    private static void require(boolean condition, String message) {
        if (!condition) throw new IllegalStateException(message);
    }

    private static final class RecordedReceiptArchive implements AutoCloseable {
        private final HttpServer server;
        private final AtomicReference<JsonNode> receipt;
        private final AtomicInteger reads = new AtomicInteger();

        private RecordedReceiptArchive(JsonNode recorded) throws IOException {
            receipt = new AtomicReference<>(recorded);
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", exchange -> {
                try (exchange) {
                    JsonNode request = JSON.readTree(exchange.getRequestBody());
                    JsonNode current = receipt.get();
                    if (!"2.0".equals(request.path("jsonrpc").asText())
                            || !"pax_sequencerReceipt".equals(request.path("method").asText())
                            || !request.path("params").isArray() || request.path("params").size() != 1
                            || !current.required("txHash").equals(request.path("params").path(0))) {
                        exchange.sendResponseHeaders(400, -1);
                        return;
                    }
                    ObjectNode response = JSON.createObjectNode().put("jsonrpc", "2.0");
                    response.set("id", request.required("id"));
                    response.set("result", current);
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

    private static void expectVerificationFailure(Runnable action) {
        try {
            action.run();
            throw new IllegalStateException("invalid verification input accepted");
        } catch (PlatformSdkException expected) {
            if (expected.code() != PlatformSdkException.Code.VERIFICATION_FAILURE) throw expected;
        }
    }

    private static byte[] concat(byte[] left, byte[] right) {
        byte[] value = java.util.Arrays.copyOf(left, left.length + right.length);
        System.arraycopy(right, 0, value, left.length, right.length);
        return value;
    }

    private static void rejectNumericMoney(JsonNode node, String name) {
        if (node.isObject()) node.fields().forEachRemaining(field -> rejectNumericMoney(field.getValue(), field.getKey()));
        else if (node.isArray()) node.forEach(child -> rejectNumericMoney(child, name));
        else if ((name.equals("amount") || name.equals("amounts") || name.endsWith("_amount")
                || name.endsWith("_balance") || name.endsWith("_sequence")) && node.isNumber()) {
            throw new IllegalStateException(name + " is encoded as a JSON number");
        }
    }

    private static boolean pathMatches(String template, String actual) {
        StringBuilder expression = new StringBuilder("^");
        int position = 0;
        var parameter = Pattern.compile("\\{[^}]+}").matcher(template);
        while (parameter.find()) {
            expression.append(Pattern.quote(template.substring(position, parameter.start()))).append("[^/]+");
            position = parameter.end();
        }
        expression.append(Pattern.quote(template.substring(position))).append('$');
        return actual.matches(expression.toString());
    }
    private static String text(JsonNode node, String field) {
        JsonNode value = node.path(field);
        if (!value.isTextual()) throw new IllegalStateException(field + " is missing");
        return value.textValue();
    }
    private static JsonNode read(Path path) throws IOException { return JSON.readTree(Files.readAllBytes(path)); }
}
