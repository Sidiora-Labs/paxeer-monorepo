package com.sidiora.layerx.sdk.verify;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.PlatformSdkException;
import java.io.ByteArrayOutputStream;
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Instant;
import java.time.format.DateTimeFormatterBuilder;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Base64;
import java.util.HashMap;
import java.util.HashSet;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
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
                                   List<Validator> resultsValidators, byte[] transaction,
                                   LocalVerifier.MerkleProof transactionProof) {
        public ReceiptInclusion(byte[] transaction, LocalVerifier.MerkleProof transactionProof,
                ExecTxResult result, LocalVerifier.MerkleProof resultProof, byte[] batchTransaction,
                LocalVerifier.MerkleProof batchProof, Header nextHeader, Commit nextCommit,
                List<Validator> nextValidators) {
            this(result, resultProof, batchTransaction, batchProof, List.of(), nextHeader, nextCommit,
                nextValidators, transaction, transactionProof);
        }
        public ReceiptInclusion(ExecTxResult result, LocalVerifier.MerkleProof resultProof,
                byte[] batchTransaction, LocalVerifier.MerkleProof batchProof, List<byte[]> activityIds,
                Header resultsHeader, Commit resultsCommit, List<Validator> resultsValidators) {
            this(result, resultProof, batchTransaction, batchProof, activityIds, resultsHeader,
                resultsCommit, resultsValidators, null, null);
        }
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

    public static Verification verify(LocalVerifier.SequencerReceiptVerification receiptVerification,
            byte[] rawTransaction, Header header, ReceiptInclusion inclusion, Commit commit,
            List<Validator> validators) {
        try {
            require(receiptVerification != null
                && receiptVerification.level() == LocalVerifier.VerificationLevel.RECEIPTED);
            var receipt = receiptVerification.receipt();
            BigInteger height = new BigInteger(1, exact(receipt.batchId(), 32));
            require(height.signum() > 0 && height.bitLength() <= 64 && header.height() > 0
                && header.height() < Long.MAX_VALUE && BigInteger.valueOf(header.height()).compareTo(height) >= 0
                && receipt.resultCode() >= 0 && receipt.resultCode() <= 1
                && receiptVerification.status() == 1 - receipt.resultCode());
            byte[] headerHash = header.hash();
            verifyCommit(header, commit, validators);
            Header next = inclusion.resultsHeader();
            require(next.height() == header.height() + 1 && next.chainId().equals(header.chainId())
                && equal(next.lastBlockId().hash(), headerHash)
                && next.lastBlockId().partsTotal() == commit.blockId().partsTotal()
                && equal(next.lastBlockId().partsHash(), commit.blockId().partsHash())
                && equal(header.nextValidatorsHash(), next.validatorsHash()));
            verifyCommit(next, inclusion.resultsCommit(), inclusion.resultsValidators());
            require(equal(rawTransaction, committedEVMTransaction(inclusion.transaction())));
            var transactionProof = inclusion.transactionProof();
            var resultProof = inclusion.resultProof();
            var batchProof = inclusion.batchProof();
            byte[] summary = receipt.effects().get(0).body();
            require(summary.length >= 28 && transactionProof.leafIndex() > 0
                && transactionProof.leafIndex() == resultProof.leafIndex()
                && transactionProof.leafCount() == resultProof.leafCount()
                && batchProof.leafCount() == transactionProof.leafCount() && batchProof.leafIndex() == 0
                && transactionProof.leafIndex() == Integer.toUnsignedLong(ByteBuffer.wrap(summary, 24, 4).getInt()));
            verifyProof(sha256(inclusion.transaction()), transactionProof, header.dataHash());
            verifyProof(sha256(inclusion.batchTransaction()), batchProof, header.dataHash());
            var result = inclusion.result();
            require((result.code() == 0 || result.code() == 45)
                && receipt.resultCode() == (result.code() == 0 ? 0 : 1)
                && result.gasUsed() >= 0 && result.gasWanted() >= result.gasUsed()
                && result.gasUsed() == ByteBuffer.wrap(summary).getLong());
            verifyProof(result.encode(), resultProof, next.lastResultsHash());
            verifyResultData(receipt, result.data());
            byte[] batch = inclusion.batchTransaction();
            require(batch.length == BATCH_DOMAIN.length + 216
                && equal(Arrays.copyOf(batch, BATCH_DOMAIN.length), BATCH_DOMAIN));
            var fields = ByteBuffer.wrap(batch, BATCH_DOMAIN.length, 24);
            BigInteger batchHeight = unsigned64(fields), from = unsigned64(fields), to = unsigned64(fields);
            require(batchHeight.equals(BigInteger.valueOf(header.height())) && from.signum() > 0
                && from.compareTo(receipt.globalSequence()) <= 0 && to.compareTo(receipt.globalSequence()) >= 0);
            return new Verification(VerificationLevel.COMMITTED, header.height(), headerHash, receiptVerification);
        } catch (PlatformSdkException error) {
            throw error;
        } catch (RuntimeException error) {
            throw PlatformSdkException.verificationFailure();
        }
    }

    public static byte[] committedEVMTransaction(byte[] blockTransaction) {
        TransactionProto tx = new TransactionProto(blockTransaction);
        TransactionProto body = new TransactionProto(tx.single(1));
        require(!body.fields(1).isEmpty());
        TransactionProto message = new TransactionProto(body.fields(1).get(0));
        String type = new String(message.single(1), StandardCharsets.US_ASCII);
        require(type.equals("/paxprotocol.paxchain.evm.MsgEVMTransaction")
            || type.equals("/ethermint.evm.v1.MsgEthereumTx"));
        TransactionProto wrapped = new TransactionProto(message.single(2));
        byte[] data = wrapped.single(1);
        require(data.length > 0);
        int first = data[0] & 0xff;
        if (first >= 0xc0 || first >= 1 && first <= 4) return data;
        TransactionProto transaction = new TransactionProto(data);
        String kind = new String(transaction.single(1), StandardCharsets.US_ASCII);
        byte[] encodedFields = transaction.single(2);
        require(encodedFields.length > 0);
        TransactionProto fields = new TransactionProto(encodedFields);
        int txType;
        String layout;
        switch (kind) {
            case "/paxprotocol.paxchain.eth.LegacyTx" -> { txType = 0; layout = "uduhdrbbb"; }
            case "/paxprotocol.paxchain.eth.AccessListTx" -> { txType = 1; layout = "duduhdrabbb"; }
            case "/paxprotocol.paxchain.eth.DynamicFeeTx" -> { txType = 2; layout = "dudduhdrabbb"; }
            case "/paxprotocol.paxchain.eth.BlobTx" -> { txType = 3; layout = "dudduhdradx_bbb"; }
            case "/paxprotocol.paxchain.eth.SetCodeTx" -> { txType = 4; layout = "dudduhdraobbb"; }
            default -> throw PlatformSdkException.verificationFailure();
        }
        var payload = new ByteArrayOutputStream();
        for (int index = 0; index < layout.length(); index++) {
            char format = layout.charAt(index);
            if (format == '_') continue;
            payload.writeBytes(format == 'a' || format == 'x' || format == 'o'
                ? evmList(fields.fields(index + 1), format)
                : rlp(evmValue(fields, index + 1, format), false));
        }
        byte[] encoded = rlp(payload.toByteArray(), true);
        return txType == 0 ? encoded : join(new byte[]{(byte) txType}, encoded);
    }

    private static byte[] evmValue(TransactionProto fields, int field, char format) {
        if (format == 'u') return magnitude(new BigInteger(Long.toUnsignedString(fields.integer(field))));
        byte[] value = fields.optional(field);
        if (format == 'd' && value.length > 0) {
            String decimal = new String(value, StandardCharsets.US_ASCII);
            require(decimal.length() <= 78 && decimal.matches("[0-9]+"));
            BigInteger number = new BigInteger(decimal);
            require(number.bitLength() <= 256);
            return magnitude(number);
        }
        if (format == 'h' && value.length > 0) return hexText(value, 20);
        if (format == 'b') {
            require(value.length <= 32);
            int first = 0;
            while (first < value.length && value[first] == 0) first++;
            return Arrays.copyOfRange(value, first, value.length);
        }
        return value;
    }

    private static byte[] evmList(List<byte[]> values, char kind) {
        var payload = new ByteArrayOutputStream();
        for (byte[] value : values) {
            if (kind == 'x') { payload.writeBytes(rlp(exact(value, 32), false)); continue; }
            TransactionProto fields = new TransactionProto(value);
            var tuple = new ByteArrayOutputStream();
            if (kind == 'o') {
                require(fields.optional(1).length > 0 && fields.optional(2).length > 0);
                for (int field = 1; field <= 6; field++)
                    tuple.writeBytes(rlp(evmValue(fields, field, "dhubbb".charAt(field - 1)), false));
            } else {
                tuple.writeBytes(rlp(hexText(fields.single(1), 20), false));
                var keys = new ByteArrayOutputStream();
                for (byte[] key : fields.fields(2)) keys.writeBytes(rlp(hexText(key, 32), false));
                tuple.writeBytes(rlp(keys.toByteArray(), true));
            }
            payload.writeBytes(rlp(tuple.toByteArray(), true));
        }
        return rlp(payload.toByteArray(), true);
    }

    private static final class TransactionProto {
        private record Value(int wire, byte[] data, long number) {}
        private final Map<Integer, List<Value>> values = new HashMap<>();
        private final ByteBuffer input;

        private TransactionProto(byte[] data) {
            require(data != null && data.length <= 1_048_576);
            input = ByteBuffer.wrap(data);
            while (input.hasRemaining()) {
                long tag = readVarint();
                require(tag > 0 && tag >>> 3 > 0 && tag >>> 3 <= (1 << 29) - 1);
                int field = (int) (tag >>> 3), wire = (int) (tag & 7);
                Value value;
                if (wire == 0) value = new Value(wire, new byte[0], readVarint());
                else {
                    require(wire == 1 || wire == 2 || wire == 5);
                    long length = wire == 1 ? 8 : wire == 5 ? 4 : readVarint();
                    require(length >= 0 && length <= input.remaining());
                    byte[] bytes = new byte[(int) length]; input.get(bytes);
                    value = new Value(wire, bytes, 0);
                }
                values.computeIfAbsent(field, ignored -> new ArrayList<>()).add(value);
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

        private List<byte[]> fields(int field) {
            var result = new ArrayList<byte[]>();
            for (Value value : values.getOrDefault(field, List.of())) {
                require(value.wire() == 2); result.add(value.data());
            }
            return result;
        }

        private byte[] single(int field) {
            List<byte[]> entries = fields(field);
            require(entries.size() == 1);
            return entries.get(0);
        }

        private byte[] optional(int field) {
            List<byte[]> entries = fields(field);
            require(entries.size() <= 1);
            return entries.isEmpty() ? new byte[0] : entries.get(0);
        }

        private long integer(int field) {
            List<Value> entries = values.getOrDefault(field, List.of());
            require(entries.size() <= 1);
            if (entries.isEmpty()) return 0;
            require(entries.get(0).wire() == 0);
            return entries.get(0).number();
        }
    }

    private static byte[] magnitude(BigInteger value) {
        if (value.signum() == 0) return new byte[0];
        byte[] encoded = value.toByteArray();
        return encoded[0] == 0 ? Arrays.copyOfRange(encoded, 1, encoded.length) : encoded;
    }

    private static byte[] rlp(byte[] value, boolean list) {
        int prefix = list ? 0xc0 : 0x80;
        if (!list && value.length == 1 && (value[0] & 0xff) < 0x80) return value;
        if (value.length <= 55) return join(new byte[]{(byte) (prefix + value.length)}, value);
        byte[] length = magnitude(BigInteger.valueOf(value.length));
        return join(new byte[]{(byte) (prefix + 55 + length.length)}, length, value);
    }

    private static final ObjectMapper WIRE = new ObjectMapper();

    public static Header decodeHeader(JsonNode node) {
        wireShape(node, "VersionBlock", "VersionApp", "ChainID", "Height", "Time", "LastBlockID",
            "LastCommitHash", "DataHash", "ValidatorsHash", "NextValidatorsHash", "ConsensusHash",
            "AppHash", "LastResultsHash", "EvidenceHash", "ProposerAddress");
        return new Header(wireNumber(node, "VersionBlock"), wireNumber(node, "VersionApp"),
            wireText(node, "ChainID"), wireNumber(node, "Height"), decodeTime(node.required("Time")),
            decodeBlockId(node.required("LastBlockID")), wireBytes(node, "LastCommitHash"),
            wireBytes(node, "DataHash"), wireBytes(node, "ValidatorsHash"), wireBytes(node, "NextValidatorsHash"),
            wireBytes(node, "ConsensusHash"), wireBytes(node, "AppHash"), wireBytes(node, "LastResultsHash"),
            wireBytes(node, "EvidenceHash"), wireBytes(node, "ProposerAddress"));
    }

    public static ObjectNode encodeHeader(Header header) {
        ObjectNode node = WIRE.createObjectNode();
        node.put("VersionBlock", header.blockVersion()); node.put("VersionApp", header.appVersion());
        node.put("ChainID", header.chainId()); node.put("Height", header.height());
        node.put("Time", encodeTime(header.time())); node.set("LastBlockID", encodeBlockId(header.lastBlockId()));
        putBytes(node, "LastCommitHash", header.lastCommitHash()); putBytes(node, "DataHash", header.dataHash());
        putBytes(node, "ValidatorsHash", header.validatorsHash());
        putBytes(node, "NextValidatorsHash", header.nextValidatorsHash());
        putBytes(node, "ConsensusHash", header.consensusHash()); putBytes(node, "AppHash", header.appHash());
        putBytes(node, "LastResultsHash", header.lastResultsHash()); putBytes(node, "EvidenceHash", header.evidenceHash());
        putBytes(node, "ProposerAddress", header.proposerAddress());
        return node;
    }

    private static BlockId decodeBlockId(JsonNode node) {
        wireShape(node, "Hash", "PartSetTotal", "PartSetHash");
        return new BlockId(wireBytes(node, "Hash"), Math.toIntExact(wireNumber(node, "PartSetTotal")),
            wireBytes(node, "PartSetHash"));
    }

    private static ObjectNode encodeBlockId(BlockId id) {
        ObjectNode node = WIRE.createObjectNode();
        putBytes(node, "Hash", id.hash()); node.put("PartSetTotal", id.partsTotal());
        putBytes(node, "PartSetHash", id.partsHash());
        return node;
    }

    public static Commit decodeCommit(JsonNode node) {
        wireShape(node, "Height", "Round", "BlockID", "Signatures");
        var signatures = new ArrayList<CommitSignature>();
        for (JsonNode signature : wireArray(node.required("Signatures"), 10000)) {
            wireShape(signature, "BlockIDFlag", "ValidatorAddress", "Timestamp", "Signature");
            signatures.add(new CommitSignature(Math.toIntExact(wireNumber(signature, "BlockIDFlag")),
                wireBytes(signature, "ValidatorAddress"), decodeTime(signature.required("Timestamp")),
                wireBytes(signature, "Signature")));
        }
        return new Commit(wireNumber(node, "Height"), Math.toIntExact(wireNumber(node, "Round")),
            decodeBlockId(node.required("BlockID")), signatures);
    }

    public static ObjectNode encodeCommit(Commit commit) {
        ObjectNode node = WIRE.createObjectNode();
        node.put("Height", commit.height()); node.put("Round", commit.round());
        node.set("BlockID", encodeBlockId(commit.blockId()));
        ArrayNode signatures = node.putArray("Signatures");
        for (CommitSignature signature : commit.signatures()) {
            ObjectNode item = signatures.addObject();
            item.put("BlockIDFlag", signature.blockIdFlag());
            putBytes(item, "ValidatorAddress", signature.validatorAddress());
            item.put("Timestamp", encodeTime(signature.timestamp())); putBytes(item, "Signature", signature.signature());
        }
        return node;
    }

    public static List<Validator> decodeValidators(JsonNode node) {
        var validators = new ArrayList<Validator>();
        for (JsonNode item : wireArray(node, 10000)) {
            wireShape(item, "PublicKey", "VotingPower");
            validators.add(new Validator(decodeFixedBytes(item.required("PublicKey")), wireNumber(item, "VotingPower")));
        }
        return List.copyOf(validators);
    }

    public static ArrayNode encodeValidators(List<Validator> validators) {
        ArrayNode node = WIRE.createArrayNode();
        for (Validator validator : validators) {
            ObjectNode item = node.addObject();
            item.set("PublicKey", encodeFixedBytes(validator.publicKey())); item.put("VotingPower", validator.votingPower());
        }
        return node;
    }

    private static LocalVerifier.MerkleProof decodeProof(JsonNode node) {
        wireShape(node, "Total", "Index", "Aunts");
        var aunts = new ArrayList<byte[]>();
        JsonNode encoded = node.required("Aunts");
        if (!encoded.isNull()) for (JsonNode aunt : wireArray(encoded, 63)) aunts.add(decodeFixedBytes(aunt));
        return new LocalVerifier.MerkleProof(wireNumber(node, "Index"), wireNumber(node, "Total"), aunts);
    }

    private static ObjectNode encodeProof(LocalVerifier.MerkleProof proof) {
        ObjectNode node = WIRE.createObjectNode();
        node.put("Total", proof.leafCount()); node.put("Index", proof.leafIndex());
        ArrayNode aunts = node.putArray("Aunts");
        for (byte[] aunt : proof.siblings()) aunts.add(encodeFixedBytes(aunt));
        return node;
    }

    public static ReceiptInclusion decodeInclusion(JsonNode node) {
        wireShape(node, "Transaction", "TransactionProof", "Result", "ResultProof", "BatchTransaction",
            "BatchProof", "NextHeader", "NextCommit", "NextValidators");
        JsonNode result = node.required("Result");
        wireShape(result, "Code", "Data", "GasWanted", "GasUsed");
        return new ReceiptInclusion(new ExecTxResult(wireNumber(result, "Code"), wireBytes(result, "Data"),
            wireNumber(result, "GasWanted"), wireNumber(result, "GasUsed")), decodeProof(node.required("ResultProof")),
            wireBytes(node, "BatchTransaction"), decodeProof(node.required("BatchProof")), List.of(),
            decodeHeader(node.required("NextHeader")), decodeCommit(node.required("NextCommit")),
            decodeValidators(node.required("NextValidators")), wireBytes(node, "Transaction"),
            decodeProof(node.required("TransactionProof")));
    }

    public static ObjectNode encodeInclusion(ReceiptInclusion inclusion) {
        require(inclusion.transaction() != null && inclusion.transactionProof() != null);
        ObjectNode node = WIRE.createObjectNode();
        putBytes(node, "Transaction", inclusion.transaction());
        node.set("TransactionProof", encodeProof(inclusion.transactionProof()));
        ObjectNode result = node.putObject("Result");
        result.put("Code", inclusion.result().code()); putBytes(result, "Data", inclusion.result().data());
        result.put("GasWanted", inclusion.result().gasWanted()); result.put("GasUsed", inclusion.result().gasUsed());
        node.set("ResultProof", encodeProof(inclusion.resultProof()));
        putBytes(node, "BatchTransaction", inclusion.batchTransaction()); node.set("BatchProof", encodeProof(inclusion.batchProof()));
        node.set("NextHeader", encodeHeader(inclusion.resultsHeader())); node.set("NextCommit", encodeCommit(inclusion.resultsCommit()));
        node.set("NextValidators", encodeValidators(inclusion.resultsValidators()));
        return node;
    }

    private static Timestamp decodeTime(JsonNode node) {
        require(node.isTextual() && node.textValue().matches(
            "[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\\.[0-9]{1,9})?(?:Z|[+-][0-9]{2}:[0-9]{2})"));
        Instant instant = Instant.parse(node.textValue());
        return new Timestamp(instant.getEpochSecond(), instant.getNano());
    }

    private static String encodeTime(Timestamp time) {
        timestamp(time);
        return new DateTimeFormatterBuilder().appendInstant(-1).toFormatter()
            .format(Instant.ofEpochSecond(time.seconds(), time.nanos()));
    }

    private static byte[] decodeFixedBytes(JsonNode node) {
        require(wireArray(node, 32).size() == 32);
        byte[] bytes = new byte[32];
        for (int index = 0; index < bytes.length; index++) {
            JsonNode value = node.get(index);
            require(value.isIntegralNumber() && value.canConvertToInt() && value.intValue() >= 0 && value.intValue() <= 255);
            bytes[index] = (byte) value.intValue();
        }
        return bytes;
    }

    private static ArrayNode encodeFixedBytes(byte[] value) {
        ArrayNode node = WIRE.createArrayNode();
        for (byte item : exact(value, 32)) node.add(item & 0xff);
        return node;
    }

    private static JsonNode wireArray(JsonNode node, int maximum) {
        require(node.isArray() && node.size() <= maximum);
        return node;
    }

    private static long wireNumber(JsonNode node, String field) {
        JsonNode value = node.required(field);
        require(value.isIntegralNumber() && value.canConvertToLong());
        return value.longValue();
    }

    private static String wireText(JsonNode node, String field) {
        JsonNode value = node.required(field);
        require(value.isTextual());
        return value.textValue();
    }

    private static byte[] wireBytes(JsonNode node, String field) {
        JsonNode value = node.required(field);
        if (value.isNull()) return new byte[0];
        String text = wireText(node, field);
        require(text.length() <= 1_398_104);
        byte[] decoded = Base64.getDecoder().decode(text);
        require(decoded.length <= 1_048_576 && Base64.getEncoder().encodeToString(decoded).equals(text));
        return decoded;
    }

    private static void putBytes(ObjectNode node, String field, byte[] value) {
        require(value != null && value.length <= 1_048_576);
        node.put(field, Base64.getEncoder().encodeToString(value));
    }

    private static void wireShape(JsonNode node, String... fields) {
        require(node.isObject() && node.size() == fields.length);
        for (String field : fields) require(node.has(field));
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
