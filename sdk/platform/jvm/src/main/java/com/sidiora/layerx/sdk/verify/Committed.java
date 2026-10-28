package com.sidiora.layerx.sdk.verify;

import com.sidiora.layerx.sdk.PlatformSdkException;
import java.io.ByteArrayOutputStream;
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.HexFormat;
import java.util.List;
import org.bouncycastle.crypto.params.Ed25519PublicKeyParameters;
import org.bouncycastle.crypto.signers.Ed25519Signer;

public final class Committed {
    private Committed() {}

    private static final byte[] BATCH_DOMAIN = utf8("PAXEER/v1/sequence-batch\0");
    private static final byte[] SEQUENCE_DOMAIN = utf8("PAXEER/v1/sequence\0");
    private static final long MAX_POWER = Long.MAX_VALUE / 8;

    public enum VerificationLevel {
        COMMITTED;
        public String wire() { return "committed"; }
    }

    public record Verification(VerificationLevel level, long height, byte[] blockHash,
                               LocalVerifier.SequencerReceiptVerification receipted) {
        public Verification { blockHash = blockHash.clone(); }
        @Override public byte[] blockHash() { return blockHash.clone(); }
        public int status() { return receipted.status(); }
        public void requireSuccess() { receipted.requireSuccess(); }
    }

    public record Timestamp(long seconds, int nanos) {}
    public record BlockId(byte[] hash, int partsTotal, byte[] partsHash) {}
    public record Validator(byte[] publicKey, long votingPower) {}
    public record Header(long blockVersion, long appVersion, String chainId, long height,
                         Timestamp time, BlockId lastBlockId, byte[] lastCommitHash,
                         byte[] dataHash, byte[] validatorsHash, byte[] nextValidatorsHash,
                         byte[] consensusHash, byte[] appHash, byte[] lastResultsHash,
                         byte[] evidenceHash, byte[] proposerAddress) {
        public byte[] hash() {
            require(height > 0 && chainId != null && !chainId.isEmpty()
                && utf8(chainId).length <= 50);
            exact(validatorsHash, 32);
            return root(List.of(join(number(1, blockVersion), number(2, appVersion)),
                bytes(1, utf8(chainId)), number(1, height), timestamp(time), blockId(lastBlockId),
                bytes(1, lastCommitHash), bytes(1, dataHash), bytes(1, validatorsHash),
                bytes(1, nextValidatorsHash), bytes(1, consensusHash), bytes(1, appHash),
                bytes(1, lastResultsHash), bytes(1, evidenceHash), bytes(1, proposerAddress)));
        }
    }

    public record CommitSignature(int blockIdFlag, byte[] validatorAddress,
                                  Timestamp timestamp, byte[] signature) {
        public static CommitSignature absent() {
            return new CommitSignature(1, new byte[0], new Timestamp(-62135596800L, 0), new byte[0]);
        }
    }
    public record Commit(long height, int round, BlockId blockId, List<CommitSignature> signatures) {
        public Commit { signatures = List.copyOf(signatures); }
    }
    public record ExecTxResult(long code, byte[] data, long gasWanted, long gasUsed) {
        public byte[] encode() {
            require(code >= 0 && code <= 0xffff_ffffL && data != null && data.length <= 1_048_576);
            return join(number(1, code), bytes(2, data), number(5, gasWanted), number(6, gasUsed));
        }
    }
    public record ReceiptInclusion(ExecTxResult result, LocalVerifier.MerkleProof resultProof,
                                   byte[] batchTransaction, LocalVerifier.MerkleProof batchProof,
                                   List<byte[]> activityIds, Header resultsHeader, Commit resultsCommit,
                                   List<Validator> resultsValidators) {
        public ReceiptInclusion {
            activityIds = List.copyOf(activityIds);
            resultsValidators = List.copyOf(resultsValidators);
        }
    }

    public static Verification verify(LocalVerifier.SequencerReceiptVerification receiptVerification,
                                      Header blockHeader, ReceiptInclusion receiptInclusion,
                                      Commit commit, List<Validator> validatorSet) {
        try {
            require(receiptVerification != null
                && receiptVerification.level() == LocalVerifier.VerificationLevel.RECEIPTED);
            var receipt = receiptVerification.receipt();
            exact(receipt.batchId(), 32);
            BigInteger receiptHeight = new BigInteger(1, receipt.batchId());
            require(receiptHeight.signum() > 0
                && BigInteger.valueOf(blockHeader.height()).compareTo(receiptHeight) >= 0);
            require(receipt.resultCode() == 0 || receipt.resultCode() == 1);
            require(receiptVerification.status() == 1 - receipt.resultCode());
            byte[] headerHash = blockHeader.hash();
            verifyCommit(blockHeader, commit, validatorSet);

            var inclusion = receiptInclusion;
            Header resultsHeader = inclusion.resultsHeader();
            require(blockHeader.height() < Long.MAX_VALUE
                && resultsHeader.height() == blockHeader.height() + 1
                && blockHeader.chainId().equals(resultsHeader.chainId())
                && equal(resultsHeader.lastBlockId().hash(), headerHash)
                && resultsHeader.lastBlockId().partsTotal() == commit.blockId().partsTotal()
                && equal(resultsHeader.lastBlockId().partsHash(), commit.blockId().partsHash())
                && equal(blockHeader.nextValidatorsHash(), resultsHeader.validatorsHash()));
            verifyCommit(resultsHeader, inclusion.resultsCommit(), inclusion.resultsValidators());

            var result = inclusion.result();
            require(result.code() == 0 || result.code() == 45);
            require(receipt.resultCode() == (result.code() == 0 ? 0 : 1));
            require(result.gasWanted() >= 0 && result.gasUsed() >= 0);
            byte[] summary = receipt.effects().get(0).body();
            require(summary.length >= 8 && new BigInteger(1, Arrays.copyOf(summary, 8))
                .equals(BigInteger.valueOf(result.gasUsed())));
            verifyProof(result.encode(), inclusion.resultProof(), resultsHeader.lastResultsHash());
            verifyResultData(receipt, result.data());
            verifyBatch(receipt, blockHeader, inclusion);
            return new Verification(VerificationLevel.COMMITTED, blockHeader.height(), headerHash,
                receiptVerification);
        } catch (PlatformSdkException error) {
            throw error;
        } catch (RuntimeException error) {
            throw PlatformSdkException.verificationFailure();
        }
    }

    private static void verifyResultData(LocalVerifier.ProtocolReceipt receipt, byte[] data) {
        Proto transaction = new Proto(data, 1);
        Proto message = new Proto(transaction.single(1), 2);
        require(equal(message.single(1), utf8("/paxprotocol.paxchain.evm.MsgEVMTransaction")));
        Proto response = new Proto(message.single(2), 5);
        byte[] summary = receipt.effects().get(0).body();
        require(response.integer(1) == ByteBuffer.wrap(summary).getLong()
            && equal(sha256(response.optional(3)), receipt.activityRoot())
            && equal(sha256(response.optional(2)), Arrays.copyOfRange(summary, 68, 100))
            && response.optional(3).length == ByteBuffer.wrap(summary, 100, 4).getInt()
            && response.fields(5).size() == receipt.effects().size() - 1);
        require(equal(hexText(response.single(4), 32), receipt.activityId()));
        for (int index = 0; index < response.fields(5).size(); index++) {
            Proto log = new Proto(response.fields(5).get(index), 5);
            var body = new ByteArrayOutputStream();
            body.writeBytes(hexText(log.single(1), 20));
            require(log.fields(2).size() <= 4 && log.integer(4) == index && log.integer(5) == 0);
            body.write(log.fields(2).size());
            for (byte[] topic : log.fields(2)) body.writeBytes(hexText(topic, 32));
            body.writeBytes(sha256(log.optional(3)));
            require(equal(body.toByteArray(), receipt.effects().get(index + 1).body()));
        }
    }

    private static byte[] hexText(byte[] text, int length) {
        String value = new String(text, StandardCharsets.US_ASCII);
        require(value.startsWith("0x") && value.length() == length * 2 + 2);
        return exact(HexFormat.of().parseHex(value.substring(2)), length);
    }

    private static final class Proto {
        private final List<List<byte[]>> values = new ArrayList<>();
        private final List<Long> integers = new ArrayList<>();
        private final List<Boolean> present = new ArrayList<>();
        private final ByteBuffer input;

        private Proto(byte[] data, int maximumField) {
            require(data != null && data.length <= 1_048_576);
            input = ByteBuffer.wrap(data);
            for (int index = 0; index <= maximumField; index++) {
                values.add(new ArrayList<>()); integers.add(0L); present.add(false);
            }
            while (input.hasRemaining()) {
                long tag = readVarint();
                int field = (int) (tag >>> 3), wire = (int) (tag & 7);
                require(tag >>> 3 == field && field > 0 && field <= maximumField);
                if (wire == 0) {
                    require(!present.get(field) && values.get(field).isEmpty());
                    integers.set(field, readVarint()); present.set(field, true);
                } else {
                    require(wire == 2 && !present.get(field));
                    long length = readVarint();
                    require(length >= 0 && length <= input.remaining());
                    byte[] value = new byte[(int) length]; input.get(value); values.get(field).add(value);
                }
            }
        }
        private long readVarint() {
            long value = 0;
            for (int shift = 0; shift < 70; shift += 7) {
                require(input.hasRemaining());
                int next = input.get() & 0xff;
                require(shift != 63 || next <= 1);
                value |= (long) (next & 127) << shift;
                if ((next & 128) == 0) return value;
            }
            throw PlatformSdkException.verificationFailure();
        }
        private List<byte[]> fields(int field) { require(!present.get(field)); return values.get(field); }
        private byte[] single(int field) { require(fields(field).size() == 1); return fields(field).get(0); }
        private byte[] optional(int field) {
            require(fields(field).size() <= 1);
            return fields(field).isEmpty() ? new byte[0] : fields(field).get(0);
        }
        private long integer(int field) { require(values.get(field).isEmpty()); return integers.get(field); }
    }

    private static void verifyBatch(LocalVerifier.ProtocolReceipt receipt, Header header,
                                    ReceiptInclusion inclusion) {
        byte[] tx = inclusion.batchTransaction();
        require(tx != null && tx.length == BATCH_DOMAIN.length + 216
            && equal(Arrays.copyOf(tx, BATCH_DOMAIN.length), BATCH_DOMAIN));
        var fields = ByteBuffer.wrap(tx, BATCH_DOMAIN.length, 216);
        BigInteger height = unsigned64(fields), from = unsigned64(fields), to = unsigned64(fields);
        byte[] previous = new byte[32], last = new byte[32], txRoot = new byte[32], keyId = new byte[32];
        fields.get(previous).get(last).get(txRoot).get(keyId);
        require(height.equals(BigInteger.valueOf(header.height())) && from.signum() > 0
            && from.compareTo(receipt.globalSequence()) <= 0 && to.compareTo(receipt.globalSequence()) >= 0
            && to.subtract(from).add(BigInteger.ONE).equals(BigInteger.valueOf(inclusion.activityIds().size()))
            && equal(keyId, receipt.contextHash()));
        var ids = new ByteArrayOutputStream();
        byte[] chain = previous;
        int receiptIndex = receipt.globalSequence().subtract(from).intValueExact();
        for (int index = 0; index < inclusion.activityIds().size(); index++) {
            byte[] activity = exact(inclusion.activityIds().get(index), 32);
            ids.writeBytes(activity);
            if (index == receiptIndex) require(equal(activity, receipt.activityId())
                && equal(chain, receipt.previousStateRoot()));
            chain = sha256(SEQUENCE_DOMAIN, chain, uint64(from.add(BigInteger.valueOf(index))), activity);
            if (index == receiptIndex) require(equal(chain, receipt.resultingStateRoot()));
        }
        require(equal(sha256(ids.toByteArray()), txRoot) && equal(chain, last));
        var batchProof = inclusion.batchProof();
        var resultProof = inclusion.resultProof();
        require(batchProof.leafIndex() == 0 && resultProof.leafIndex() == receiptIndex + 1L
            && batchProof.leafCount() == resultProof.leafCount()
            && batchProof.leafCount() == inclusion.activityIds().size() + 1L);
        verifyProof(sha256(tx), batchProof, header.dataHash());
    }

    private static void verifyCommit(Header header, Commit commit, List<Validator> validators) {
        require(validators != null && !validators.isEmpty() && validators.size() <= 10000
            && commit.signatures().size() == validators.size() && commit.height() == header.height()
            && commit.round() >= 0 && equal(commit.blockId().hash(), header.hash())
            && commit.blockId().partsTotal() > 0 && commit.blockId().partsTotal() <= 101);
        exact(commit.blockId().partsHash(), 32);
        require(equal(validatorsHash(validators), header.validatorsHash()));
        long total = 0, signed = 0;
        var seen = new HashSet<String>();
        for (int index = 0; index < validators.size(); index++) {
            Validator validator = validators.get(index);
            byte[] key = exact(validator.publicKey(), 32);
            require(validator.votingPower() > 0 && validator.votingPower() <= MAX_POWER
                && seen.add(HexFormat.of().formatHex(key)));
            require(total <= MAX_POWER - validator.votingPower());
            total += validator.votingPower();
            CommitSignature signature = commit.signatures().get(index);
            if (signature.blockIdFlag() == 1) {
                require(signature.signature().length == 0 && signature.validatorAddress().length == 0
                    && signature.timestamp().seconds() == -62135596800L && signature.timestamp().nanos() == 0);
                continue;
            }
            require(signature.blockIdFlag() == 2 || signature.blockIdFlag() == 3);
            require(equal(exact(signature.validatorAddress(), 20), Arrays.copyOf(sha256(key), 20)));
            byte[] message = voteSignBytes(header.chainId(), commit, signature);
            var verifier = new Ed25519Signer();
            verifier.init(false, new Ed25519PublicKeyParameters(key, 0));
            verifier.update(message, 0, message.length);
            require(verifier.verifySignature(exact(signature.signature(), 64)));
            if (signature.blockIdFlag() == 2) signed += validator.votingPower();
        }
        require(signed * 3 > total * 2);
    }

    public static byte[] validatorsHash(List<Validator> validators) {
        var encoded = new ArrayList<byte[]>();
        for (Validator validator : validators) {
            require(validator.votingPower() > 0);
            encoded.add(join(message(1, message(1, exact(validator.publicKey(), 32))),
                number(2, validator.votingPower())));
        }
        return root(encoded);
    }

    public static byte[] voteSignBytes(String chainId, Commit commit, CommitSignature signature) {
        byte[] vote = join(number(1, 2), fixed64(2, commit.height()), fixed64(3, commit.round()),
            signature.blockIdFlag() == 2 ? message(4, blockId(commit.blockId())) : new byte[0],
            message(5, timestamp(signature.timestamp())), bytes(6, utf8(chainId)));
        return join(varint(vote.length), vote);
    }

    public static void verifyProof(byte[] item, LocalVerifier.MerkleProof proof, byte[] expectedRoot) {
        require(proof != null && proof.leafCount() > 0 && proof.leafIndex() >= 0
            && proof.leafIndex() < proof.leafCount() && proof.siblings().size() <= 63);
        byte[] hash = proofRoot(sha256(new byte[]{0}, item), proof.leafIndex(), proof.leafCount(),
            proof.siblings(), proof.siblings().size());
        require(equal(hash, exact(expectedRoot, 32)));
    }

    private static byte[] proofRoot(byte[] leaf, long index, long count, List<byte[]> aunts, int size) {
        if (count == 1) { require(index == 0 && size == 0); return leaf; }
        require(size > 0);
        long split = Long.highestOneBit(count - 1);
        byte[] aunt = exact(aunts.get(size - 1), 32);
        return index < split
            ? sha256(new byte[]{1}, proofRoot(leaf, index, split, aunts, size - 1), aunt)
            : sha256(new byte[]{1}, aunt, proofRoot(leaf, index - split, count - split, aunts, size - 1));
    }

    private static byte[] root(List<byte[]> items) {
        if (items.isEmpty()) return sha256(new byte[0]);
        if (items.size() == 1) return sha256(new byte[]{0}, items.get(0));
        int split = Integer.highestOneBit(items.size() - 1);
        return sha256(new byte[]{1}, root(items.subList(0, split)), root(items.subList(split, items.size())));
    }

    private static byte[] blockId(BlockId id) {
        return join(bytes(1, id.hash()), message(2, join(number(1, id.partsTotal()), bytes(2, id.partsHash()))));
    }
    private static byte[] timestamp(Timestamp time) {
        require(time.seconds() >= -62135596800L && time.seconds() <= 253402300799L
            && time.nanos() >= 0 && time.nanos() < 1000000000);
        return join(number(1, time.seconds()), number(2, time.nanos()));
    }
    private static byte[] fixed64(int field, long value) {
        return value == 0 ? new byte[0] : join(varint((field << 3) | 1),
            ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putLong(value).array());
    }
    private static byte[] number(int field, long value) {
        return value == 0 ? new byte[0] : join(varint(field << 3), varint(value));
    }
    private static byte[] bytes(int field, byte[] value) {
        require(value != null);
        return value.length == 0 ? new byte[0] : message(field, value);
    }
    private static byte[] message(int field, byte[] value) {
        return join(varint((field << 3) | 2), varint(value.length), value);
    }
    private static byte[] varint(long value) {
        var output = new ByteArrayOutputStream();
        while ((value & ~0x7fL) != 0) { output.write((int) (value & 0x7f) | 0x80); value >>>= 7; }
        output.write((int) value);
        return output.toByteArray();
    }
    private static BigInteger unsigned64(ByteBuffer input) {
        byte[] bytes = new byte[8]; input.get(bytes); return new BigInteger(1, bytes);
    }
    private static byte[] uint64(BigInteger value) {
        require(value.signum() >= 0 && value.bitLength() <= 64);
        byte[] raw = value.toByteArray(), out = new byte[8];
        int count = Math.min(raw.length, 8);
        System.arraycopy(raw, raw.length - count, out, 8 - count, count);
        return out;
    }
    private static byte[] join(byte[]... values) {
        var output = new ByteArrayOutputStream();
        for (byte[] value : values) output.writeBytes(value);
        return output.toByteArray();
    }
    private static byte[] sha256(byte[]... values) {
        try {
            var hash = MessageDigest.getInstance("SHA-256");
            for (byte[] value : values) hash.update(value);
            return hash.digest();
        } catch (java.security.NoSuchAlgorithmException error) { throw new AssertionError(error); }
    }
    private static byte[] utf8(String value) { return value.getBytes(StandardCharsets.UTF_8); }
    private static boolean equal(byte[] left, byte[] right) { return MessageDigest.isEqual(left, right); }
    private static byte[] exact(byte[] value, int size) { require(value != null && value.length == size); return value; }
    private static void require(boolean valid) { if (!valid) throw PlatformSdkException.verificationFailure(); }
}
