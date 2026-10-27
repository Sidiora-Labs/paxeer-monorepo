package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sidiora.layerx.sdk.verify.Committed;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import java.io.ByteArrayOutputStream;
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.List;
import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters;
import org.bouncycastle.crypto.signers.Ed25519Signer;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

public final class CommittedReceiptTest {
    private static final String CHAIN = "paxeer-committed-offline";
    private static final Committed.Timestamp TIME = new Committed.Timestamp(1700000123L, 987654321);
    private static final byte[] DOMAIN = utf8("PAXEER/v1/sequence-batch\0");

    private record Bundle(LocalVerifier.SequencerReceiptVerification receipt, Committed.Header header,
                          Committed.ReceiptInclusion inclusion, Committed.Commit commit,
                          List<Committed.Validator> validators) {
        Committed.Verification verify() {
            return Committed.verify(receipt, header, inclusion, commit, validators);
        }
    }

    @Test
    void committedRungAcceptsTwoThirdsCommit() throws Exception {
        Bundle bundle = bundle(false, new long[]{10, 10, 10, 10});
        var committed = bundle.verify();
        assertEquals("committed", committed.level().wire());
        assertEquals(bundle.header().height(), committed.height());
        assertArrayEquals(bundle.header().hash(), committed.blockHash());
        assertSame(bundle.receipt(), committed.receipted());
        assertEquals(1, committed.status());
        assertDoesNotThrow(committed::requireSuccess);
        assertEquals(3, bundle.commit().signatures().stream().filter(s -> s.blockIdFlag() == 2).count());
        assertEquals(3, bundle.inclusion().resultProof().leafCount());
        Bundle later = bundle(false, new long[]{10, 10, 10, 10}, 1);
        assertEquals(bundle.header().height() + 1, later.verify().height());

        Bundle reverted = bundle(true, new long[]{10, 10, 10, 10});
        var refusedExecution = reverted.verify();
        assertEquals("committed", refusedExecution.level().wire());
        assertEquals(45, reverted.inclusion().result().code());
        assertEquals(0, refusedExecution.status());
        assertEquals(PlatformSdkException.Code.REVERT,
            assertThrows(PlatformSdkException.class, refusedExecution::requireSuccess).code());

        var weighted = validators(new long[]{40, 30, 20, 10});
        assertEquals("0321b92eefbfb6129dcbfb8c9066c149bb1091c312f388e2142ef4bb4ba86fb0",
            HexFormat.of().formatHex(Committed.validatorsHash(weighted)));
        var reference = new Committed.Header(11, 2, "paxeer-committed-π", 1234567890123L, TIME,
            new Committed.BlockId(fill(0x11, 32), 257, fill(0x22, 32)), fill(0x33, 32),
            fill(0x44, 32), Committed.validatorsHash(weighted), fill(0x55, 32), fill(0x66, 32),
            new byte[]{0, 1, 2, (byte) 0xff, (byte) 0x80}, fill(0x77, 32), fill(0x88, 32),
            HexFormat.of().parseHex("34750f98bd59fcfc946da45aaabe933be154a4b5"));
        assertEquals("6ea46ec595bac2dc17d1a567a4bff602c059522020a3b5afafa4e8607ea4db99",
            HexFormat.of().formatHex(reference.hash()));
    }

    @Test
    void committedRungRefusesInsufficientPower() throws Exception {
        Bundle bundle = bundle(false, new long[]{10, 10, 10, 10});
        var signatures = new ArrayList<>(bundle.commit().signatures());
        signatures.set(2, Committed.CommitSignature.absent());
        var half = new Committed.Commit(bundle.commit().height(), bundle.commit().round(),
            bundle.commit().blockId(), signatures);
        assertRefused(() -> Committed.verify(bundle.receipt(), bundle.header(), bundle.inclusion(),
            half, bundle.validators()));

        Bundle exact = bundle(false, new long[]{1, 1, 2, 2});
        assertEquals(4, exact.validators().subList(0, 3).stream().mapToLong(Committed.Validator::votingPower).sum());
        assertEquals(6, exact.validators().stream().mapToLong(Committed.Validator::votingPower).sum());
        assertRefused(exact::verify);

        Bundle weighted = bundle(false, new long[]{1, 1, 1, 100});
        assertRefused(weighted::verify);
        var duplicate = new ArrayList<>(bundle.commit().signatures());
        duplicate.set(2, duplicate.get(0));
        assertRefused(() -> Committed.verify(bundle.receipt(), bundle.header(), bundle.inclusion(),
            new Committed.Commit(bundle.commit().height(), bundle.commit().round(),
                bundle.commit().blockId(), duplicate), bundle.validators()));
        byte[] signature = bundle.commit().signatures().get(0).signature().clone();
        signature[0] ^= 1;
        var invalid = new ArrayList<>(bundle.commit().signatures());
        var original = invalid.get(0);
        invalid.set(0, new Committed.CommitSignature(2, original.validatorAddress(), original.timestamp(), signature));
        assertRefused(() -> Committed.verify(bundle.receipt(), bundle.header(), bundle.inclusion(),
            new Committed.Commit(bundle.commit().height(), bundle.commit().round(),
                bundle.commit().blockId(), invalid), bundle.validators()));
    }

    @Test
    void committedRungRefusesTamperedInclusion() throws Exception {
        Bundle bundle = bundle(false, new long[]{10, 10, 10, 10});
        var original = bundle.inclusion();
        byte[] sibling = original.resultProof().siblings().get(0).clone();
        sibling[0] ^= 1;
        var aunts = new ArrayList<>(original.resultProof().siblings());
        aunts.set(0, sibling);
        var changedResult = new LocalVerifier.MerkleProof(original.resultProof().leafIndex(),
            original.resultProof().leafCount(), aunts);
        assertRefused(() -> Committed.verify(bundle.receipt(), bundle.header(),
            inclusion(original, changedResult, original.batchProof(), original.resultsCommit()),
            bundle.commit(), bundle.validators()));

        byte[] batchSibling = original.batchProof().siblings().get(0).clone();
        batchSibling[0] ^= 1;
        var batchAunts = new ArrayList<>(original.batchProof().siblings());
        batchAunts.set(0, batchSibling);
        var changedBatch = new LocalVerifier.MerkleProof(0, original.batchProof().leafCount(), batchAunts);
        assertRefused(() -> Committed.verify(bundle.receipt(), bundle.header(),
            inclusion(original, original.resultProof(), changedBatch, original.resultsCommit()),
            bundle.commit(), bundle.validators()));

        var nextCommit = original.resultsCommit();
        var nextSignatures = new ArrayList<>(nextCommit.signatures());
        nextSignatures.set(2, Committed.CommitSignature.absent());
        var unauthenticatedResults = new Committed.Commit(nextCommit.height(), nextCommit.round(),
            nextCommit.blockId(), nextSignatures);
        assertRefused(() -> Committed.verify(bundle.receipt(), bundle.header(),
            inclusion(original, original.resultProof(), original.batchProof(), unauthenticatedResults),
            bundle.commit(), bundle.validators()));

        Bundle earlier = bundle(false, new long[]{10, 10, 10, 10}, -1);
        assertRefused(earlier::verify);
        var extraAunt = new ArrayList<>(original.resultProof().siblings());
        extraAunt.add(new byte[32]);
        assertRefused(() -> Committed.verifyProof(original.result().encode(),
            new LocalVerifier.MerkleProof(original.resultProof().leafIndex(), 3, extraAunt),
            original.resultsHeader().lastResultsHash()));
    }

    private static Committed.ReceiptInclusion inclusion(Committed.ReceiptInclusion original,
            LocalVerifier.MerkleProof result, LocalVerifier.MerkleProof batch, Committed.Commit nextCommit) {
        return new Committed.ReceiptInclusion(original.result(), result, original.batchTransaction(), batch,
            original.activityIds(), original.resultsHeader(), nextCommit, original.resultsValidators());
    }

    private static Bundle bundle(boolean revert, long[] powers) throws Exception {
        return bundle(revert, powers, 0);
    }

    private static Bundle bundle(boolean revert, long[] powers, int heightOffset) throws Exception {
        JsonNode vector = vector(revert);
        JsonNode pin = vector.required("pinnedKey");
        var keys = new LocalVerifier.ReceiptKeys(List.of(new LocalVerifier.ReceiptKey(
            bytes(pin, "keyId"), bytes(pin, "publicKey"), pin.required("firstHeight").bigIntegerValue(),
            pin.required("lastHeight").bigIntegerValue())));
        var verified = LocalVerifier.verifySequencerReceipt(bytes(vector, "signedReceipt"),
            bytes(vector, "rawTransaction"), keys, vector.required("receiptJson"), bytes(vector, "prevSeqHash"));
        var receipt = verified.receipt();
        long height = new BigInteger(1, receipt.batchId()).longValueExact() + heightOffset;
        long sequence = receipt.globalSequence().longValueExact();
        var validators = validators(powers);
        JsonNode other = vector(!revert);
        byte[] otherActivity = bytes(other.required("receiptJson"), "txHash");
        byte[] last = hash(utf8("PAXEER/v1/sequence\0"), receipt.resultingStateRoot(),
            ByteBuffer.allocate(8).putLong(sequence + 1).array(), otherActivity);
        byte[] unsignedBatch = join(DOMAIN, ByteBuffer.allocate(24).putLong(height).putLong(sequence)
            .putLong(sequence + 1).array(), receipt.previousStateRoot(), last,
            hash(receipt.activityId(), otherActivity), receipt.contextHash());
        assertArrayEquals(bytes(pin, "publicKey"), privateKey(0x5e).generatePublicKey().getEncoded());
        byte[] batchDigest = hash(DOMAIN, utf8(CHAIN),
            Arrays.copyOfRange(unsignedBatch, DOMAIN.length, unsignedBatch.length - 32));
        byte[] batch = join(unsignedBatch, sign(privateKey(0x5e), batchDigest));
        byte[] tx1 = bytes(vector, "rawTransaction"), tx2 = bytes(other, "rawTransaction");
        List<byte[]> transactionHashes = List.of(hash(batch), hash(tx1), hash(tx2));
        long gasUsed = ByteBuffer.wrap(receipt.effects().get(0).body()).getLong();
        byte[] response = join(number(1, gasUsed), field(2, utf8(vector.required("receiptJson").required("vmError").asText())),
            field(3, bytes(vector.required("receiptJson"), "returnData")),
            field(4, utf8(vector.required("receiptJson").required("txHash").asText())));
        byte[] resultData = field(1, join(field(1, utf8("/paxprotocol.paxchain.evm.MsgEVMTransaction")), field(2, response)));
        var result = new Committed.ExecTxResult(revert ? 45 : 0, resultData, 1000000, gasUsed);
        byte[] encodedResult = join(number(1, revert ? 45 : 0), field(2, resultData),
            number(5, 1000000), number(6, gasUsed));
        assertArrayEquals(encodedResult, result.encode());
        List<byte[]> results = List.of(new byte[0], encodedResult, new byte[0]);
        byte[] validatorsHash = Committed.validatorsHash(validators);
        var header = header(height, root(transactionHashes), hash(new byte[0]), validatorsHash,
            new Committed.BlockId(fill(0x10, 32), 1, fill(0x11, 32)));
        var commit = commit(header, validators, 3);
        var next = header(height + 1, hash(new byte[0]), root(results), validatorsHash, commit.blockId());
        var inclusion = new Committed.ReceiptInclusion(result, proof(results, 1), batch,
            proof(transactionHashes, 0), List.of(receipt.activityId(), otherActivity),
            next, commit(next, validators, 3), validators);
        return new Bundle(verified, header, inclusion, commit, validators);
    }

    private static Committed.Header header(long height, byte[] dataHash, byte[] resultsHash,
                                           byte[] validatorsHash, Committed.BlockId previous) {
        return new Committed.Header(11, 2, CHAIN, height, TIME, previous, fill(0x33, 32), dataHash,
            validatorsHash, validatorsHash, fill(0x66, 32), fill(0x77, 32), resultsHash,
            hash(new byte[0]), fill(0x12, 20));
    }

    private static List<Committed.Validator> validators(long[] powers) {
        var validators = new ArrayList<Committed.Validator>();
        for (int index = 0; index < powers.length; index++)
            validators.add(new Committed.Validator(privateKey(index + 1).generatePublicKey().getEncoded(), powers[index]));
        return validators;
    }

    private static Committed.Commit commit(Committed.Header header, List<Committed.Validator> validators, int signers) {
        var block = new Committed.BlockId(header.hash(), 2, fill(0x22, 32));
        var signatures = new ArrayList<Committed.CommitSignature>();
        var unsigned = new Committed.Commit(header.height(), 7, block, List.of());
        for (int index = 0; index < validators.size(); index++) {
            if (index >= signers) { signatures.add(Committed.CommitSignature.absent()); continue; }
            byte[] address = Arrays.copyOf(hash(validators.get(index).publicKey()), 20);
            var vote = new Committed.CommitSignature(2, address, TIME, new byte[0]);
            byte[] message = Committed.voteSignBytes(header.chainId(), unsigned, vote);
            signatures.add(new Committed.CommitSignature(2, address, TIME, sign(privateKey(index + 1), message)));
        }
        return new Committed.Commit(header.height(), 7, block, signatures);
    }

    private static Ed25519PrivateKeyParameters privateKey(int seed) {
        return new Ed25519PrivateKeyParameters(fill(seed, 32), 0);
    }
    private static byte[] sign(Ed25519PrivateKeyParameters key, byte[] message) {
        var signer = new Ed25519Signer(); signer.init(true, key); signer.update(message, 0, message.length);
        return signer.generateSignature();
    }
    private static LocalVerifier.MerkleProof proof(List<byte[]> leaves, int index) {
        var aunts = new ArrayList<byte[]>();
        aunts(leaves, index, aunts);
        return new LocalVerifier.MerkleProof(index, leaves.size(), aunts);
    }
    private static void aunts(List<byte[]> leaves, int index, List<byte[]> output) {
        if (leaves.size() == 1) return;
        int split = Integer.highestOneBit(leaves.size() - 1);
        if (index < split) {
            aunts(leaves.subList(0, split), index, output); output.add(root(leaves.subList(split, leaves.size())));
        } else {
            aunts(leaves.subList(split, leaves.size()), index - split, output); output.add(root(leaves.subList(0, split)));
        }
    }
    private static byte[] root(List<byte[]> items) {
        if (items.size() == 1) return hash(new byte[]{0}, items.get(0));
        int split = Integer.highestOneBit(items.size() - 1);
        return hash(new byte[]{1}, root(items.subList(0, split)), root(items.subList(split, items.size())));
    }
    private static JsonNode vector(boolean revert) throws Exception {
        Path directory = Path.of("").toAbsolutePath();
        while (directory != null) {
            Path path = directory.resolve("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");
            if (Files.isRegularFile(path)) {
                for (JsonNode item : new ObjectMapper().readTree(Files.readString(path)).required("cases"))
                    if (item.required("vector").asText().equals(revert ? "V2" : "V1")) return item;
                throw new IllegalStateException("missing receipt vector");
            }
            directory = directory.getParent();
        }
        throw new IllegalStateException("missing receipt fixture");
    }
    private static byte[] bytes(JsonNode object, String name) {
        return HexFormat.of().parseHex(object.required(name).textValue().substring(2));
    }
    private static void assertRefused(Runnable operation) {
        assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE,
            assertThrows(PlatformSdkException.class, operation::run).code());
    }
    private static byte[] fill(int value, int count) {
        byte[] output = new byte[count]; Arrays.fill(output, (byte) value); return output;
    }
    private static byte[] utf8(String text) { return text.getBytes(StandardCharsets.UTF_8); }
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
    private static byte[] number(int tag, long value) {
        return value == 0 ? new byte[0] : join(varint(tag << 3), varint(value));
    }
    private static byte[] field(int tag, byte[] value) {
        return value.length == 0 ? new byte[0] : join(varint((tag << 3) | 2), varint(value.length), value);
    }
    private static byte[] varint(long value) {
        var output = new ByteArrayOutputStream();
        while ((value & ~127L) != 0) { output.write((int) (value & 127) | 128); value >>>= 7; }
        output.write((int) value); return output.toByteArray();
    }
}
