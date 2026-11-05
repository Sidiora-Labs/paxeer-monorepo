package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.Committed;
import com.sidiora.layerx.sdk.verify.ConsensusTrust;
import com.sidiora.layerx.sdk.verify.GeneratedReceiptContract.ReceiptCheck;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.PortableReceipt;
import com.sun.net.httpserver.HttpServer;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.util.Arrays;
import java.util.Base64;
import java.util.HexFormat;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters;
import org.bouncycastle.crypto.signers.Ed25519Signer;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

public final class Repair2231343Test {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final String CHAIN = "committed-fixture";
    private static final byte[] VALIDATOR_ROOT = Base64.getDecoder()
        .decode("ZawkHY2ZdOJMgK5e+LSb009M3VIrCogeqWCijke7bCk=");

    @Test
    void receiptCommittedRungSurvivesClientRestart() throws Exception {
        JsonNode vector = committedVector();
        JsonNode before = vector.required("receiptJson");
        JsonNode after = recordedNodeReceipt();
        byte[] raw = bytes(vector, "rawTransaction");
        assertEquals("receipted", before.required("rung").textValue());
        assertEquals("committed", after.required("rung").textValue());
        assertEquals(before.required("receipt"), after.required("receipt"));
        try (var archive = new RecordedReceiptArchive(before);
                var connection = new ProgramsClient.Programs(archive.uri())) {
            ProgramsClient client = client(connection, key(vector));
            var receipted = client.sequencerReceipt(raw).toCompletableFuture().get(10, TimeUnit.SECONDS);
            archive.receipt.set(after);
            var committedView = client.sequencerReceipt(raw).toCompletableFuture().get(10, TimeUnit.SECONDS);
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, committedView.level());
            assertArrayEquals(receipted.receiptDigest(), committedView.receiptDigest());
            assertArrayEquals(bytes(vector, "signedReceipt"), committedView.canonicalBytes());
            connection.close();
            try (var reopened = new ProgramsClient.Programs(archive.uri())) {
                var recovered = client(reopened, key(vector)).sequencerReceipt(raw)
                    .toCompletableFuture().get(10, TimeUnit.SECONDS);
                assertArrayEquals(committedView.receiptDigest(), recovered.receiptDigest());
                assertEquals(1, recovered.status());
                assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, recovered.level());
            }
            assertEquals(3, archive.reads.get());
            assertEquals("committed", after.required("rung").textValue());
        }
    }

    @Test
    void receiptPaymentAssetAndSignedFeeDenominationAreIndependent() throws Exception {
        JsonNode fee = fixture("sdk/platform/conformance/fixtures/sequencer-receipt-fee-token-v1.json")
            .required("cases").required(0);
        var verified = LocalVerifier.verifySequencerReceipt(bytes(fee, "signedReceipt"),
            bytes(fee, "rawTransaction"), keys(key(fee)));
        assertEquals("usid", fee.required("paymentDenom").textValue());
        assertEquals("uhpx", fee.required("feeDenom").textValue());
        assertArrayEquals(bytes(fee, "asset"), verified.receipt().asset());
        assertArrayEquals(bytes(fee, "receiptDigest"), verified.receiptDigest());
        byte[] summary = verified.receipt().effects().getFirst().body();
        assertEquals(4, summary[108]);
        assertArrayEquals("uhpx".getBytes(java.nio.charset.StandardCharsets.UTF_8),
            Arrays.copyOfRange(summary, 109, summary.length));
        assertFalse(MessageDigest.isEqual(verified.receipt().asset(),
            MessageDigest.getInstance("SHA-256").digest(Arrays.copyOfRange(summary, 109, summary.length))));

        JsonNode vector = committedVector();
        JsonNode node = recordedNodeReceipt();
        try (var archive = new RecordedReceiptArchive(node);
                var connection = new ProgramsClient.Programs(archive.uri())) {
            var payment = client(connection, key(vector)).sequencerReceipt(bytes(vector, "rawTransaction"))
                .toCompletableFuture().get(10, TimeUnit.SECONDS);
            assertEquals(1, payment.status());
            assertArrayEquals(bytes(vector, "receiptDigest"), payment.receiptDigest());
            assertEquals(2, payment.receipt().effects().size() - 1);
        }
        for (String field : List.of("paymentDenom", "feeDenom")) {
            ObjectNode tampered = node.deepCopy();
            tampered.put(field, "usid");
            PlatformSdkException error = refused(() -> ProgramsClient.verifyReceipt(bytes(vector, "signedReceipt"),
                bytes(vector, "rawTransaction"), keys(key(vector)), tampered));
            assertEquals(ReceiptCheck.FIELDS, error.receiptCheck(), field);
            ObjectNode missing = node.deepCopy();
            missing.remove(field);
            assertEquals(ReceiptCheck.FIELDS, refused(() -> ProgramsClient.verifyReceipt(
                bytes(vector, "signedReceipt"), bytes(vector, "rawTransaction"), keys(key(vector)), missing))
                .receiptCheck(), field);
        }
    }

    @Test
    void portableNodeShapeRoundTripsThroughHttpAndOfflineImport() throws Exception {
        JsonNode vector = committedVector();
        JsonNode node = recordedNodeReceipt();
        byte[] raw = bytes(vector, "rawTransaction");
        byte[] exported;
        try (var archive = new RecordedReceiptArchive(node);
                var connection = new ProgramsClient.Programs(archive.uri())) {
            JsonNode delivered = connection.request("pax_sequencerReceipt", JSON.createArrayNode()
                .add(node.required("txHash"))).toCompletableFuture().get(10, TimeUnit.SECONDS);
            assertEquals(node, delivered);
            exported = PortableReceipt.export(raw, encode(delivered), key(vector));
        }
        var offline = PortableReceipt.verify(exported, keys(key(vector)));
        assertEquals("receipted", offline.verificationLevel());
        assertEquals(node, JSON.readTree(exported).required("receiptJson"));
        assertArrayEquals(exported, PortableReceipt.export(offline));
        byte[] committed = PortableReceipt.export(raw, encode(node), key(vector), header(vector),
            inclusion(vector), commit(vector), validators(vector), trust());
        var proven = PortableReceipt.verify(committed, keys(key(vector)), trust());
        assertEquals("committed", proven.verificationLevel());
        assertArrayEquals(bytes(node, "blockHash"), proven.committed().blockHash());
        assertEquals(10, proven.committed().height());
        assertArrayEquals(committed, PortableReceipt.export(proven));

        ObjectNode unknown = node.deepCopy();
        unknown.put("unrecognized", true);
        refused(() -> PortableReceipt.export(raw, encode(unknown), key(vector)));
        ObjectNode missing = node.deepCopy();
        missing.remove("paymentDenom");
        refused(() -> PortableReceipt.export(raw, encode(missing), key(vector)));
        ObjectNode falseBlock = node.deepCopy();
        falseBlock.put("blockHash", "0x" + "01".repeat(32));
        refused(() -> PortableReceipt.export(raw, encode(falseBlock), key(vector), header(vector),
            inclusion(vector), commit(vector), validators(vector), trust()));
    }

    @Test
    void portablePublishedV5BundlesVerifyUnmodifiedWithIndependentTrust() throws Exception {
        JsonNode bundles = fixture("sdk/platform/conformance/fixtures/vector-v5-portable.json").required("bundles");
        assertEquals(2, bundles.size());
        JsonNode receiptVector = fixture("sdk/platform/conformance/fixtures/vector-v1-receipted-positive.json")
            .required("cases").required(0);
        var receiptKeys = new LocalVerifier.ReceiptKeys(List.of(key(receiptVector), key(committedVector())));
        var levels = new java.util.HashSet<String>();
        for (JsonNode bundle : bundles) {
            byte[] unchanged = encode(bundle);
            var verified = PortableReceipt.verify(unchanged, receiptKeys, trust());
            assertTrue(levels.add(verified.verificationLevel()));
            assertEquals(bundle.required("verificationLevel").textValue(), verified.verificationLevel());
            assertArrayEquals(bytes(bundle, "signedReceipt"), verified.receipted().canonicalBytes());
            assertArrayEquals(bytes(bundle, "receiptDigest"), verified.receipted().receiptDigest());
            assertArrayEquals(unchanged, PortableReceipt.export(verified));
            assertEquals(1, verified.status());
            assertDoesNotThrow(verified::requireSuccess);
            if (verified.committed() == null) {
                assertEquals("receipted", PortableReceipt.verify(unchanged, receiptKeys).verificationLevel());
            } else {
                refused(() -> PortableReceipt.verify(unchanged, receiptKeys));
            }
        }
        assertEquals(java.util.Set.of("receipted", "committed"), levels);
    }

    @Test
    void portableConsensusTrustAuthenticatesTransitionsToBothProofHeights() throws Exception {
        JsonNode vector = committedVector();
        var block = header(vector);
        var proof = inclusion(vector);
        var step = new ConsensusTrust.Step(block, commit(vector), validators(vector));
        trust().verify(proof.resultsHeader(), proof.resultsCommit(), proof.resultsValidators(), List.of(step));
        refused(() -> trust().verify(proof.resultsHeader(), proof.resultsCommit(), proof.resultsValidators(), List.of()));
        refused(() -> new ConsensusTrust("other-chain", 10, VALIDATOR_ROOT).verify(
            proof.resultsHeader(), proof.resultsCommit(), proof.resultsValidators(), List.of(step)));
        byte[] wrongRoot = VALIDATOR_ROOT.clone();
        wrongRoot[0] ^= 1;
        refused(() -> new ConsensusTrust(CHAIN, 10, wrongRoot).verify(proof.resultsHeader(),
            proof.resultsCommit(), proof.resultsValidators(), List.of(step)));
        refused(() -> new ConsensusTrust(CHAIN, 9, VALIDATOR_ROOT).verify(proof.resultsHeader(),
            proof.resultsCommit(), proof.resultsValidators(), List.of(step)));
        var badCommit = Committed.decodeCommit(fixture("sdk/platform/conformance/fixtures/vector-v4-committed.json")
            .required("refusals").required(0).required("nextCommit"));
        refused(() -> trust().verify(proof.resultsHeader(), badCommit, proof.resultsValidators(), List.of(step)));
        byte[] rootInput = VALIDATOR_ROOT.clone();
        var isolated = new ConsensusTrust(CHAIN, 10, rootInput);
        rootInput[0] ^= 1;
        isolated.validatorsHash()[0] ^= 1;
        isolated.verify(proof.resultsHeader(), proof.resultsCommit(), proof.resultsValidators(), List.of(step));
    }

    @Test
    void portableConsensusTrustRejectsAttackerSignedHeadersAroundGenuineBatch() throws Exception {
        JsonNode bundle = fixture("sdk/platform/conformance/fixtures/vector-v5-portable.json")
            .required("bundles").required(1);
        JsonNode vector = committedVector();
        var receiptKeys = keys(key(vector));
        var validReceipt = ProgramsClient.verifyReceipt(bytes(bundle, "signedReceipt"),
            bytes(bundle, "rawTransaction"), receiptKeys, bundle.required("receiptJson"));
        var privateKey = new Ed25519PrivateKeyParameters(new java.security.SecureRandom());
        var attackerSet = List.of(new Committed.Validator(privateKey.generatePublicKey().getEncoded(), 10));
        byte[] attackerRoot = Committed.validatorsHash(attackerSet);
        var original = Committed.decodeHeader(bundle.required("blockHeader"));
        var proof = Committed.decodeInclusion(bundle.required("receiptInclusion"));
        var forgedHeader = copyHeader(original, original.chainId(), original.lastBlockId(), attackerRoot, attackerRoot);
        var forgedCommit = sign(forgedHeader, Committed.decodeCommit(bundle.required("commit")), privateKey);
        var forgedNext = copyHeader(proof.resultsHeader(), original.chainId(), forgedCommit.blockId(), attackerRoot, attackerRoot);
        var forgedNextCommit = sign(forgedNext, proof.resultsCommit(), privateKey);
        var forgedInclusion = new Committed.ReceiptInclusion(proof.result(), proof.resultProof(),
            proof.batchTransaction(), proof.batchProof(), proof.activityIds(), forgedNext, forgedNextCommit,
            attackerSet, proof.transaction(), proof.transactionProof());
        assertEquals(Committed.VerificationLevel.COMMITTED, Committed.verify(validReceipt,
            bytes(bundle, "rawTransaction"), forgedHeader, forgedInclusion, forgedCommit, attackerSet).level());
        ObjectNode forged = bundle.deepCopy();
        forged.set("blockHeader", Committed.encodeHeader(forgedHeader));
        forged.set("commit", Committed.encodeCommit(forgedCommit));
        forged.set("validatorSet", Committed.encodeValidators(attackerSet));
        forged.set("receiptInclusion", Committed.encodeInclusion(forgedInclusion));
        assertEquals(bundle.required("signedReceipt"), forged.required("signedReceipt"));
        assertArrayEquals(proof.batchTransaction(), forgedInclusion.batchTransaction());
        refused(() -> PortableReceipt.verify(encode(forged), receiptKeys, trust()));
        refused(() -> PortableReceipt.verify(encode(forged), receiptKeys));
        refused(() -> PortableReceipt.export(bytes(bundle, "rawTransaction"), encode(bundle.required("receiptJson")),
            key(vector), forgedHeader, forgedInclusion, forgedCommit, attackerSet));

        var switchedNext = copyHeader(proof.resultsHeader(), original.chainId(),
            Committed.decodeCommit(bundle.required("commit")).blockId(), attackerRoot, attackerRoot);
        var switchedCommit = sign(switchedNext, proof.resultsCommit(), privateKey);
        ObjectNode switched = bundle.deepCopy();
        ObjectNode switchedProof = (ObjectNode) switched.required("receiptInclusion");
        switchedProof.set("NextHeader", Committed.encodeHeader(switchedNext));
        switchedProof.set("NextCommit", Committed.encodeCommit(switchedCommit));
        switchedProof.set("NextValidators", Committed.encodeValidators(attackerSet));
        refused(() -> PortableReceipt.verify(encode(switched), receiptKeys, trust()));
        refused(() -> PortableReceipt.verify(encode(bundle), receiptKeys,
            new ConsensusTrust("different-network", 10, VALIDATOR_ROOT)));
    }

    @Test
    void receiptCommittedShapeRefusesMissingAndContradictoryBlockMetadata() throws Exception {
        JsonNode vector = committedVector();
        JsonNode recorded = recordedNodeReceipt();
        for (String field : List.of("blockNumber", "blockHash")) {
            ObjectNode missing = recorded.deepCopy();
            missing.remove(field);
            refused(() -> ProgramsClient.verifyReceipt(bytes(vector, "signedReceipt"),
                bytes(vector, "rawTransaction"), keys(key(vector)), missing));
        }
        ObjectNode tooEarly = recorded.deepCopy();
        tooEarly.put("blockNumber", "0x9");
        refused(() -> ProgramsClient.verifyReceipt(bytes(vector, "signedReceipt"),
            bytes(vector, "rawTransaction"), keys(key(vector)), tooEarly));
        ObjectNode extraBlock = vector.required("receiptJson").deepCopy();
        extraBlock.set("blockHash", recorded.required("blockHash"));
        refused(() -> ProgramsClient.verifyReceipt(bytes(vector, "signedReceipt"),
            bytes(vector, "rawTransaction"), keys(key(vector)), extraBlock));
    }

    private static Committed.Header copyHeader(Committed.Header source, String chainId,
            Committed.BlockId previous, byte[] root, byte[] nextRoot) {
        return new Committed.Header(source.blockVersion(), source.appVersion(), chainId, source.height(),
            source.time(), previous, source.lastCommitHash(), source.dataHash(), root, nextRoot,
            source.consensusHash(), source.appHash(), source.lastResultsHash(), source.evidenceHash(),
            source.proposerAddress());
    }

    private static Committed.Commit sign(Committed.Header header, Committed.Commit template,
            Ed25519PrivateKeyParameters privateKey) throws Exception {
        var id = new Committed.BlockId(header.hash(), template.blockId().partsTotal(), template.blockId().partsHash());
        byte[] address = Arrays.copyOf(MessageDigest.getInstance("SHA-256")
            .digest(privateKey.generatePublicKey().getEncoded()), 20);
        var unsigned = new Committed.CommitSignature(2, address, header.time(), new byte[64]);
        var unsignedCommit = new Committed.Commit(header.height(), template.round(), id, List.of(unsigned));
        byte[] message = Committed.voteSignBytes(header.chainId(), unsignedCommit, unsigned);
        var signer = new Ed25519Signer();
        signer.init(true, privateKey);
        signer.update(message, 0, message.length);
        return new Committed.Commit(header.height(), template.round(), id, List.of(
            new Committed.CommitSignature(2, address, header.time(), signer.generateSignature())));
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

    private static JsonNode committedVector() throws Exception {
        return fixture("sdk/platform/conformance/fixtures/vector-v4-committed.json").required("positive");
    }

    private static JsonNode recordedNodeReceipt() throws Exception {
        return fixture("daemon/committed-fixture/vectors/vector-v4-committed.json")
            .required("settlement").required("receipt");
    }

    private static ConsensusTrust trust() { return new ConsensusTrust(CHAIN, 10, VALIDATOR_ROOT); }
    private static Committed.Header header(JsonNode vector) { return Committed.decodeHeader(vector.required("blockHeader")); }
    private static Committed.Commit commit(JsonNode vector) { return Committed.decodeCommit(vector.required("commit")); }
    private static Committed.ReceiptInclusion inclusion(JsonNode vector) {
        return Committed.decodeInclusion(vector.required("receiptInclusion"));
    }
    private static List<Committed.Validator> validators(JsonNode vector) {
        return Committed.decodeValidators(vector.required("validatorSet"));
    }
    private static LocalVerifier.ReceiptKey key(JsonNode vector) {
        JsonNode key = vector.required("pinnedKey");
        return new LocalVerifier.ReceiptKey(bytes(key, "keyId"), bytes(key, "publicKey"),
            key.required("firstHeight").bigIntegerValue(), key.required("lastHeight").bigIntegerValue());
    }
    private static LocalVerifier.ReceiptKeys keys(LocalVerifier.ReceiptKey key) {
        return new LocalVerifier.ReceiptKeys(List.of(key));
    }
    private static ProgramsClient client(ProgramsClient.Programs connection, LocalVerifier.ReceiptKey key) {
        return new ProgramsClient(connection, "0x2b1e0f87874643a65818291ca1656e62b3dcc7d4",
            BigInteger.ZERO, BigInteger.ONE, keys(key), new byte[32]);
    }
    private static byte[] bytes(JsonNode node, String field) {
        String value = node.required(field).textValue();
        assertNotNull(value);
        assertTrue(value.startsWith("0x"));
        return HexFormat.of().parseHex(value.substring(2));
    }
    private static byte[] encode(JsonNode value) {
        try { return JSON.writeValueAsBytes(value); }
        catch (java.io.IOException error) { throw new AssertionError(error); }
    }
    private static PlatformSdkException refused(org.junit.jupiter.api.function.Executable operation) {
        var error = assertThrows(PlatformSdkException.class, operation);
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, error.code());
        return error;
    }

    private static final class RecordedReceiptArchive implements AutoCloseable {
        private final HttpServer server;
        private final AtomicReference<JsonNode> receipt;
        private final AtomicInteger reads = new AtomicInteger();

        private RecordedReceiptArchive(JsonNode recorded) throws Exception {
            receipt = new AtomicReference<>(recorded);
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", exchange -> {
                try (exchange) {
                    JsonNode request = JSON.readTree(exchange.getRequestBody());
                    JsonNode current = receipt.get();
                    if (!"pax_sequencerReceipt".equals(request.path("method").asText())
                            || !request.path("params").isArray() || request.path("params").size() != 1
                            || !current.required("txHash").equals(request.path("params").path(0))) {
                        exchange.sendResponseHeaders(400, -1);
                        return;
                    }
                    ObjectNode response = JSON.createObjectNode().put("jsonrpc", "2.0");
                    response.set("id", request.required("id"));
                    response.set("result", current);
                    byte[] body = encode(response);
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
