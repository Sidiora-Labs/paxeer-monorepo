package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.Committed;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.PortableReceipt;
import java.io.ByteArrayOutputStream;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Base64;
import java.util.HexFormat;
import java.util.List;
import java.util.Set;
import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters;
import org.bouncycastle.crypto.signers.Ed25519Signer;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

public final class PortableReceiptTest {
    private static final ObjectMapper JSON = new ObjectMapper();

    @Test
    void portableReceiptRoundTripsAtReceipted() throws Exception {
        int count = 0;
        for (JsonNode vector : fixture().required("cases")) {
            if (!vector.required("vector").asText().equals("V1")) continue;
            count++;
            var key = key(vector);
            var keys = new LocalVerifier.ReceiptKeys(List.of(key));
            byte[] exported = PortableReceipt.export(bytes(vector, "rawTransaction"),
                encode(vector.required("receiptJson")), key);
            JsonNode bundle = JSON.readTree(exported);
            assertEquals("layerx-receipt-proof-v2", bundle.required("format").textValue());
            assertEquals("receipted", bundle.required("verificationLevel").textValue());
            assertEquals(7, bundle.size());
            for (String name : List.of("rawTransaction", "signedReceipt", "receiptDigest"))
                assertEquals(vector.required(name), bundle.required(name), name);
            assertEquals("0x" + key.firstHeight().toString(16),
                bundle.required("receiptKey").required("firstHeight").textValue());
            assertEquals("0x" + key.lastHeight().toString(16),
                bundle.required("receiptKey").required("lastHeight").textValue());
            assertArrayEquals(key.publicKey(), bytes(bundle.required("receiptKey"), "publicKey"));

            var verified = PortableReceipt.verify(exported, keys);
            assertEquals("receipted", verified.verificationLevel());
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, verified.receipted().level());
            assertNull(verified.committed());
            assertEquals(1, verified.status());
            assertDoesNotThrow(verified::requireSuccess);
            assertArrayEquals(bytes(vector, "signedReceipt"), verified.receipted().canonicalBytes());
            assertArrayEquals(bytes(vector, "receiptDigest"), verified.receipted().receiptDigest());
            assertArrayEquals(exported, PortableReceipt.export(verified));
            assertArrayEquals(exported, PortableReceipt.export(bytes(vector, "rawTransaction"),
                encode(vector.required("receiptJson")), key));
            byte[] copy = PortableReceipt.export(verified);
            copy[0] ^= 1;
            assertArrayEquals(exported, PortableReceipt.export(verified));
            byte[] mutableInput = exported.clone();
            var snapshot = PortableReceipt.verify(mutableInput, keys);
            mutableInput[0] ^= 1;
            assertArrayEquals(exported, PortableReceipt.export(snapshot));

            for (String field : List.of("rawTransaction", "signedReceipt", "receiptDigest")) {
                ObjectNode altered = bundle.deepCopy();
                byte[] changed = bytes(bundle, field);
                changed[changed.length - 1] ^= 1;
                altered.put(field, hex(changed));
                assertRefused(() -> PortableReceipt.verify(encode(altered), keys));
            }
            ObjectNode alteredKey = bundle.deepCopy();
            ((ObjectNode) alteredKey.required("receiptKey")).put("publicKey", hex(new byte[32]));
            assertRefused(() -> PortableReceipt.verify(encode(alteredKey), keys));
            ObjectNode alteredWindow = bundle.deepCopy();
            ((ObjectNode) alteredWindow.required("receiptKey")).put("lastHeight", "0xffff");
            assertRefused(() -> PortableReceipt.verify(encode(alteredWindow), keys));
            byte[] otherId = key.keyId();
            otherId[0] ^= 1;
            var otherKeys = new LocalVerifier.ReceiptKeys(List.of(new LocalVerifier.ReceiptKey(
                otherId, key.publicKey(), key.firstHeight(), key.lastHeight())));
            assertRefused(() -> PortableReceipt.verify(exported, otherKeys));
            if (count == 1) committedRoundTrip(vector);
        }
        assertEquals(4, count, "all published V1 cases must execute");
    }

    @Test
    void portableReceiptRefusesVersionOneFormat() throws Exception {
        int count = 0;
        for (JsonNode vector : fixture().required("cases")) {
            if (!vector.required("vector").asText().equals("V1")) continue;
            count++;
            var keys = new LocalVerifier.ReceiptKeys(List.of(key(vector)));
            byte[] exported = PortableReceipt.export(bytes(vector, "rawTransaction"),
                encode(vector.required("receiptJson")), key(vector));
            ObjectNode bundle = (ObjectNode) JSON.readTree(exported);
            ObjectNode versionOne = bundle.deepCopy();
            versionOne.put("format", "layerx-receipt-proof-v1");
            assertRefused(() -> PortableReceipt.verify(encode(versionOne), keys));
            for (String field : Set.of("batchId", "asset", "previousStateRoot", "resultingStateRoot")) {
                ObjectNode legacy = bundle.deepCopy();
                legacy.put(field, hex(new byte[32]));
                assertRefused(() -> PortableReceipt.verify(encode(legacy), keys));
                legacy.putNull(field);
                assertRefused(() -> PortableReceipt.verify(encode(legacy), keys));
                ObjectNode nested = bundle.deepCopy();
                ((ObjectNode) nested.required("receiptKey")).put(field, hex(new byte[32]));
                assertRefused(() -> PortableReceipt.verify(encode(nested), keys));
            }
            for (String field : List.of("rawTransaction", "signedReceipt", "receiptDigest", "receiptKey", "receiptJson")) {
                ObjectNode missing = bundle.deepCopy();
                missing.remove(field);
                assertRefused(() -> PortableReceipt.verify(encode(missing), keys));
            }
            ObjectNode unsupported = bundle.deepCopy();
            unsupported.put("verificationLevel", "finalised");
            assertRefused(() -> PortableReceipt.verify(encode(unsupported), keys));
            ObjectNode noCommit = bundle.deepCopy();
            noCommit.put("verificationLevel", "committed");
            assertRefused(() -> PortableReceipt.verify(encode(noCommit), keys));
            String json = new String(exported, StandardCharsets.UTF_8);
            byte[] duplicate = ("{\"format\":\"layerx-receipt-proof-v1\"," + json.substring(1))
                .getBytes(StandardCharsets.UTF_8);
            assertRefused(() -> PortableReceipt.verify(duplicate, keys));
            byte[] trailing = (json + " {}").getBytes(StandardCharsets.UTF_8);
            assertRefused(() -> PortableReceipt.verify(trailing, keys));
        }
        assertEquals(4, count, "all V1 bundles must refuse the retired format and fields");
    }

    @Test
    void portableExportMatchesPublishedVectorV5() throws Exception {
        JsonNode published = fixture("sdk/platform/conformance/fixtures/vector-v5-portable.json").required("bundles");
        assertEquals(2, published.size());
        JsonNode source = null;
        for (JsonNode vector : fixture().required("cases"))
            if (vector.required("id").asText().equals("V1-pax-transfer")) source = vector;
        assertNotNull(source, "the original V1 transfer must execute");
        var pin = key(source);
        byte[] receipted = PortableReceipt.export(bytes(source, "rawTransaction"),
            encode(source.required("receiptJson")), pin);
        assertArrayEquals(encode(published.get(0)), receipted);
        var receiptedKeys = new LocalVerifier.ReceiptKeys(List.of(pin));
        var receiptVerification = PortableReceipt.verify(receipted, receiptedKeys);
        assertEquals("receipted", receiptVerification.verificationLevel());
        assertEquals(1, receiptVerification.status());
        assertArrayEquals(receipted, PortableReceipt.export(receiptVerification));

        JsonNode original = fixture("daemon/committed-fixture/vectors/vector-v4-committed.json");
        JsonNode settlement = original.required("settlement");
        ObjectNode view = settlement.required("receipt").deepCopy();
        view.put("rung", "receipted");
        view.remove(List.of("blockNumber", "blockHash"));
        JsonNode authorization = original.required("authorization");
        var committedKey = new LocalVerifier.ReceiptKey(bytes(view, "keyId"), bytes(view, "signingKey"),
            authorization.required("FirstBatchNumber").bigIntegerValue(),
            authorization.required("LastBatchNumber").bigIntegerValue());
        var validators = new ArrayList<Committed.Validator>();
        for (JsonNode validator : settlement.required("validators"))
            validators.add(new Committed.Validator(base64(validator.required("pub_key"), "value"),
                Long.parseLong(validator.required("voting_power").asText())));
        JsonNode result = settlement.required("exec_tx_result");
        var inclusion = new Committed.ReceiptInclusion(new Committed.ExecTxResult(
            result.path("code").asLong(0), base64(result, "data"), result.required("gas_wanted").asLong(),
            result.required("gas_used").asLong()), cometProof(settlement.required("exec_tx_result_proof")),
            base64(settlement.required("sequence_batch_proof"), "data"),
            cometProof(settlement.required("sequence_batch_proof").required("proof")), List.of(),
            cometHeader(settlement.required("header")), cometCommit(settlement.required("commit")), validators,
            base64(settlement.required("transaction_proof"), "data"),
            cometProof(settlement.required("transaction_proof").required("proof")));
        byte[] raw = bytes(settlement, "raw_transaction");
        assertArrayEquals(raw, Committed.committedEVMTransaction(inclusion.transaction()));
        byte[] committed = PortableReceipt.export(raw, encode(view), committedKey,
            cometHeader(settlement.required("execution_header")), inclusion,
            cometCommit(settlement.required("execution_commit")), validators);
        assertArrayEquals(encode(published.get(1)), committed);
        var committedKeys = new LocalVerifier.ReceiptKeys(List.of(committedKey));
        var committedVerification = PortableReceipt.verify(committed, committedKeys);
        assertEquals("committed", committedVerification.verificationLevel());
        assertEquals(10, committedVerification.committed().height());
        assertEquals(1, committedVerification.status());
        assertDoesNotThrow(committedVerification::requireSuccess);
        assertArrayEquals(committed, PortableReceipt.export(committedVerification));

        for (int index = 0; index < published.size(); index++) {
            JsonNode bundle = published.get(index);
            var keys = index == 0 ? receiptedKeys : committedKeys;
            assertEquals(index == 0 ? "receipted" : "committed",
                PortableReceipt.verify(encode(bundle), keys).verificationLevel());
            ObjectNode changedView = bundle.deepCopy();
            ((ObjectNode) changedView.required("receiptJson")).put("gasUsed", "0x0");
            assertRefused(() -> PortableReceipt.verify(encode(changedView), keys));
            ObjectNode missingView = bundle.deepCopy();
            missingView.remove("receiptJson");
            assertRefused(() -> PortableReceipt.verify(encode(missingView), keys));
            for (String field : List.of("signedReceipt", "receiptDigest", "rawTransaction")) {
                ObjectNode changed = bundle.deepCopy();
                byte[] value = bytes(bundle, field); value[value.length - 1] ^= 1;
                changed.put(field, hex(value));
                assertRefused(() -> PortableReceipt.verify(encode(changed), keys));
            }
        }
        for (String field : List.of("Transaction", "BatchTransaction")) {
            ObjectNode changed = published.get(1).deepCopy();
            ObjectNode evidence = (ObjectNode) changed.required("receiptInclusion");
            byte[] value = base64(evidence, field); value[value.length - 1] ^= 1;
            evidence.put(field, Base64.getEncoder().encodeToString(value));
            assertRefused(() -> PortableReceipt.verify(encode(changed), committedKeys));
        }
        for (String field : List.of("TransactionProof", "BatchProof", "ResultProof")) {
            ObjectNode changed = published.get(1).deepCopy();
            ObjectNode proof = (ObjectNode) changed.required("receiptInclusion").required(field);
            proof.put("Index", field.equals("BatchProof") ? 1 : 0);
            assertRefused(() -> PortableReceipt.verify(encode(changed), committedKeys));
        }
        ObjectNode executionCommit = published.get(1).deepCopy();
        ((ObjectNode) executionCommit.required("commit").required("Signatures").get(0))
            .put("Signature", Base64.getEncoder().encodeToString(new byte[64]));
        assertRefused(() -> PortableReceipt.verify(encode(executionCommit), committedKeys));
        ObjectNode successorCommit = published.get(1).deepCopy();
        ((ObjectNode) successorCommit.required("receiptInclusion").required("NextCommit").required("Signatures").get(0))
            .put("Signature", Base64.getEncoder().encodeToString(new byte[64]));
        assertRefused(() -> PortableReceipt.verify(encode(successorCommit), committedKeys));
    }

    private static Committed.Timestamp cometTime(String value) {
        Instant time = Instant.parse(value);
        return new Committed.Timestamp(time.getEpochSecond(), time.getNano());
    }

    private static Committed.BlockId cometBlockId(JsonNode node) {
        return new Committed.BlockId(cometHex(node, "hash"), node.required("parts").required("total").asInt(),
            cometHex(node.required("parts"), "hash"));
    }

    private static Committed.Header cometHeader(JsonNode node) {
        return new Committed.Header(Long.parseLong(node.required("version").required("block").asText()),
            Long.parseLong(node.required("version").required("app").asText()), node.required("chain_id").asText(),
            Long.parseLong(node.required("height").asText()), cometTime(node.required("time").asText()),
            cometBlockId(node.required("last_block_id")), cometHex(node, "last_commit_hash"), cometHex(node, "data_hash"),
            cometHex(node, "validators_hash"), cometHex(node, "next_validators_hash"), cometHex(node, "consensus_hash"),
            cometHex(node, "app_hash"), cometHex(node, "last_results_hash"), cometHex(node, "evidence_hash"),
            cometHex(node, "proposer_address"));
    }

    private static Committed.Commit cometCommit(JsonNode node) {
        var signatures = new ArrayList<Committed.CommitSignature>();
        for (JsonNode signature : node.required("signatures"))
            signatures.add(new Committed.CommitSignature(signature.required("block_id_flag").asInt(),
                cometHex(signature, "validator_address"), cometTime(signature.required("timestamp").asText()),
                base64(signature, "signature")));
        return new Committed.Commit(Long.parseLong(node.required("height").asText()), node.required("round").asInt(),
            cometBlockId(node.required("block_id")), signatures);
    }

    private static LocalVerifier.MerkleProof cometProof(JsonNode node) {
        var aunts = new ArrayList<byte[]>();
        for (JsonNode aunt : node.required("aunts")) aunts.add(Base64.getDecoder().decode(aunt.asText()));
        return new LocalVerifier.MerkleProof(Long.parseLong(node.required("index").asText()),
            Long.parseLong(node.required("total").asText()), aunts);
    }

    private static byte[] cometHex(JsonNode node, String field) {
        return HexFormat.of().parseHex(node.required(field).asText());
    }

    private static byte[] base64(JsonNode node, String field) {
        return Base64.getDecoder().decode(node.required(field).asText());
    }

    private static void committedRoundTrip(JsonNode vector) throws Exception {
        var key = key(vector);
        var keys = new LocalVerifier.ReceiptKeys(List.of(key));
        byte[] raw = bytes(vector, "rawTransaction"), signed = bytes(vector, "signedReceipt");
        var receipt = LocalVerifier.verifySequencerReceipt(signed, raw, keys).receipt();
        assertEquals(1, receipt.effects().size(), "native V1 case has no log effects");
        long height = new java.math.BigInteger(1, receipt.batchId()).longValueExact();
        long sequence = receipt.globalSequence().longValueExact();
        byte[] domain = utf8("PAXEER/v1/sequence-batch\0");
        byte[] unsigned = join(domain, ByteBuffer.allocate(24).putLong(height).putLong(sequence)
            .putLong(sequence).array(), receipt.previousStateRoot(), receipt.resultingStateRoot(),
            hash(receipt.activityId()), receipt.contextHash());
        byte[] receiptSeed = new byte[32];
        Arrays.fill(receiptSeed, (byte) 0x5e);
        var receiptSigner = new Ed25519PrivateKeyParameters(receiptSeed, 0);
        assertArrayEquals(key.publicKey(), receiptSigner.generatePublicKey().getEncoded());
        byte[] batch = join(unsigned, sign(receiptSigner, hash(domain, utf8("portable-test"),
            Arrays.copyOfRange(unsigned, domain.length, unsigned.length - 32))));
        long gasUsed = ByteBuffer.wrap(receipt.effects().get(0).body()).getLong();
        byte[] response = join(number(1, gasUsed), field(3, bytes(vector.required("receiptJson"), "returnData")),
            field(4, utf8(vector.required("receiptJson").required("txHash").textValue())));
        byte[] resultData = field(1, join(field(1, utf8("/paxprotocol.paxchain.evm.MsgEVMTransaction")),
            field(2, response)));
        var result = new Committed.ExecTxResult(0, resultData, 1000000, gasUsed);
        var validatorKey = new Ed25519PrivateKeyParameters(new byte[32], 0);
        var validators = List.of(new Committed.Validator(validatorKey.generatePublicKey().getEncoded(), 10));
        byte[] validatorsHash = Committed.validatorsHash(validators);
        byte[] transaction = field(1, field(1, join(
            field(1, utf8("/paxprotocol.paxchain.evm.MsgEVMTransaction")), field(2, field(1, raw)))));
        byte[] batchLeaf = leaf(hash(batch)), rawLeaf = leaf(hash(transaction));
        var header = header(height, hash(new byte[]{1}, batchLeaf, rawLeaf), hash(new byte[0]),
            validatorsHash, new Committed.BlockId(new byte[32], 1, new byte[32]));
        var commit = commit(header, validatorKey);
        byte[] emptyResultLeaf = leaf(new byte[0]);
        var next = header(height + 1, hash(new byte[0]),
            hash(new byte[]{1}, emptyResultLeaf, leaf(result.encode())), validatorsHash, commit.blockId());
        var inclusion = new Committed.ReceiptInclusion(result,
            new LocalVerifier.MerkleProof(1, 2, List.of(emptyResultLeaf)), batch,
            new LocalVerifier.MerkleProof(0, 2, List.of(rawLeaf)), List.of(receipt.activityId()),
            next, commit(next, validatorKey), validators, transaction,
            new LocalVerifier.MerkleProof(1, 2, List.of(batchLeaf)));
        byte[] exported = PortableReceipt.export(raw, encode(vector.required("receiptJson")), key,
            header, inclusion, commit, validators);
        var verified = PortableReceipt.verify(exported, keys);
        assertEquals("committed", verified.verificationLevel());
        assertEquals(Committed.VerificationLevel.COMMITTED, verified.committed().level());
        assertEquals(height, verified.committed().height());
        assertArrayEquals(header.hash(), verified.committed().blockHash());
        assertArrayEquals(exported, PortableReceipt.export(verified));
        ObjectNode bundle = (ObjectNode) JSON.readTree(exported);
        assertEquals(11, bundle.size());
        for (String field : List.of("blockHeader", "receiptInclusion", "commit", "validatorSet")) {
            ObjectNode missing = bundle.deepCopy();
            missing.remove(field);
            assertRefused(() -> PortableReceipt.verify(encode(missing), keys));
        }
        ObjectNode tampered = bundle.deepCopy();
        ((ObjectNode) tampered.required("commit").required("Signatures").get(0))
            .put("Signature", Base64.getEncoder().encodeToString(new byte[64]));
        assertRefused(() -> PortableReceipt.verify(encode(tampered), keys));
        ObjectNode downgrade = bundle.deepCopy();
        downgrade.put("verificationLevel", "receipted");
        assertRefused(() -> PortableReceipt.verify(encode(downgrade), keys));
    }

    private static Committed.Header header(long height, byte[] data, byte[] results, byte[] validators,
                                           Committed.BlockId previous) {
        return new Committed.Header(11, 2, "portable-test", height, new Committed.Timestamp(1700000123, 0),
            previous, new byte[32], data, validators, validators, new byte[32], new byte[32], results,
            hash(new byte[0]), new byte[20]);
    }

    private static Committed.Commit commit(Committed.Header header, Ed25519PrivateKeyParameters key) {
        var block = new Committed.BlockId(header.hash(), 1, new byte[32]);
        var unsigned = new Committed.Commit(header.height(), 0, block, List.of());
        byte[] address = Arrays.copyOf(hash(key.generatePublicKey().getEncoded()), 20);
        var vote = new Committed.CommitSignature(2, address, header.time(), new byte[0]);
        byte[] signature = sign(key, Committed.voteSignBytes(header.chainId(), unsigned, vote));
        return new Committed.Commit(header.height(), 0, block,
            List.of(new Committed.CommitSignature(2, address, header.time(), signature)));
    }

    private static JsonNode fixture() throws Exception {
        return fixture("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");
    }

    private static JsonNode fixture(String relativePath) throws Exception {
        Path directory = Path.of("").toAbsolutePath();
        while (directory != null) {
            Path path = directory.resolve(relativePath);
            if (Files.isRegularFile(path)) return JSON.readTree(Files.readString(path));
            directory = directory.getParent();
        }
        throw new IllegalStateException(relativePath + " not found");
    }

    private static LocalVerifier.ReceiptKey key(JsonNode vector) {
        JsonNode key = vector.required("pinnedKey");
        return new LocalVerifier.ReceiptKey(bytes(key, "keyId"), bytes(key, "publicKey"),
            key.required("firstHeight").bigIntegerValue(), key.required("lastHeight").bigIntegerValue());
    }

    private static byte[] bytes(JsonNode node, String name) {
        String text = node.required(name).textValue();
        assertNotNull(text);
        assertTrue(text.startsWith("0x"));
        return HexFormat.of().parseHex(text.substring(2));
    }

    private static byte[] encode(JsonNode document) {
        try { return JSON.writeValueAsBytes(document); }
        catch (Exception error) { throw new AssertionError(error); }
    }

    private static void assertRefused(Runnable operation) {
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE,
            assertThrows(PlatformSdkException.class, operation::run).code());
    }

    private static byte[] sign(Ed25519PrivateKeyParameters key, byte[] message) {
        var signer = new Ed25519Signer();
        signer.init(true, key);
        signer.update(message, 0, message.length);
        return signer.generateSignature();
    }

    private static byte[] hash(byte[]... values) {
        try {
            var digest = MessageDigest.getInstance("SHA-256");
            for (byte[] value : values) digest.update(value);
            return digest.digest();
        } catch (Exception error) { throw new AssertionError(error); }
    }

    private static byte[] join(byte[]... values) {
        var output = new ByteArrayOutputStream();
        for (byte[] value : values) output.writeBytes(value);
        return output.toByteArray();
    }

    private static byte[] number(int field, long value) {
        return value == 0 ? new byte[0] : join(varint(field << 3), varint(value));
    }

    private static byte[] field(int field, byte[] value) {
        return value.length == 0 ? new byte[0] : join(varint((field << 3) | 2), varint(value.length), value);
    }

    private static byte[] varint(long value) {
        var output = new ByteArrayOutputStream();
        while ((value & ~127L) != 0) {
            output.write((int) (value & 127) | 128);
            value >>>= 7;
        }
        output.write((int) value);
        return output.toByteArray();
    }

    private static byte[] leaf(byte[] value) { return hash(new byte[]{0}, value); }
    private static byte[] utf8(String value) { return value.getBytes(StandardCharsets.UTF_8); }
    private static String hex(byte[] value) { return "0x" + HexFormat.of().formatHex(value); }
}
