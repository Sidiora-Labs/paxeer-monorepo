package com.sidiora.layerx.sdk.verify;

import com.sidiora.layerx.sdk.PlatformSdkException;
import com.sidiora.layerx.sdk.ProgramsClient;
import com.sidiora.layerx.sdk.NativeProgramCall;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.GeneratedReceiptContract.ReceiptCheck;
import java.io.ByteArrayOutputStream;
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.KeyFactory;
import java.security.MessageDigest;
import java.security.Signature;
import java.security.spec.X509EncodedKeySpec;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.List;
import java.util.Objects;
import java.util.Set;
import org.bouncycastle.crypto.ec.CustomNamedCurves;
import org.bouncycastle.crypto.digests.KeccakDigest;
import org.bouncycastle.crypto.params.ECDomainParameters;
import org.bouncycastle.crypto.params.ECPublicKeyParameters;
import org.bouncycastle.crypto.signers.ECDSASigner;
import org.bouncycastle.math.ec.ECAlgorithms;
import org.bouncycastle.math.ec.ECPoint;

/** Trustless 402LXP receipt, Merkle, batch, and checkpoint verification. */
public final class LocalVerifier {
    private LocalVerifier() {}

    private static final byte[] MERKLE_LEAF_DOMAIN = ascii("LXP/v1/merkle-leaf\0");
    private static final byte[] MERKLE_INTERNAL_DOMAIN = ascii("LXP/v1/merkle-internal\0");
    private static final byte[] BATCH_HEADER_DOMAIN = ascii("LXP/v1/batch-header\0");
    private static final byte[] RECEIPT_DOMAIN = ascii("LXP/v1/receipt\0");
    private static final byte[] CHECKPOINT_DOMAIN = ascii("LXP/v2/checkpoint-certificate\0");
    private static final byte[] GUARANTOR_ATTESTATION_DOMAIN = ascii("LXP/v2/guarantor-attestation\0");
    private static final byte[] ED25519_X509_PREFIX = hex("302a300506032b6570032100");
    private static final int BATCH_HEADER_BYTES = 354;
    private static final int MAX_MESSAGE_BYTES = 1_048_576;
    private static final int MAX_EFFECTS = 512;
    private static final int MAX_EFFECT_BODY = 256;
    private static final int ALL_AVAILABILITY_CLASSES = 0x1f;
    private static final int CURRENT_PROTOCOL_VERSION = 2;
    private static final long PROGRAM_OUTCOME_V1 = 0x50524731L;
    private static final long PROGRAM_OUTCOME_V2 = 0x50524732L;
    private static final long PROGRAM_OUTCOME_V3 = 0x50524733L;
    private static final long PROGRAM_OUTCOME_V4 = 0x50524734L;
    private static final BigInteger MAX_U128 = BigInteger.ONE.shiftLeft(128).subtract(BigInteger.ONE);

    public record MerkleProof(long leafIndex, long leafCount, List<byte[]> siblings) {
        public MerkleProof { siblings = List.copyOf(siblings); }
    }
    public record BatchHeader(int protocolVersion, long networkId, BigInteger epoch, BigInteger batchNumber,
        BigInteger firstSequence, BigInteger lastSequence, byte[] previousStateRoot, byte[] resultingStateRoot,
        byte[] activityMerkleRoot, byte[] receiptMerkleRoot, byte[] eventMerkleRoot,
        byte[] dataAvailabilityRoot, byte[] oracleRoot, BigInteger timestampMs, byte[] sequencerId) {}
    public record SequencerAuthorization(byte[] sequencerId, byte[] publicKey,
        BigInteger firstBatchNumber, BigInteger lastBatchNumber) {}
    public enum InclusionKind { ACTIVITY, RECEIPT, EVENT, STATE }
    public enum VerificationLevel {
        RECEIPTED("receipted"), SEQUENCER_SIGNED("sequencer-signed"), BATCH_INCLUDED("batch-included"),
        STATE_PROVEN("state-proven"), CHECKPOINT_FINALISED("checkpoint-finalised"),
        SETTLEMENT_ANCHORED("settlement-anchored");
        private final String wire;
        VerificationLevel(String wire) { this.wire = wire; }
        public String wire() { return wire; }
    }
    public record InclusionVerification(VerificationLevel level, BatchHeader header,
        byte[] headerDigest, byte[] root) {}
    public record CheckpointAttestation(int protocolVersion, long networkId, BigInteger paxeerChainId,
        byte[] settlementContract, BigInteger epoch, byte[] checkpointId, byte[] checkpointHash,
        byte[] guarantorId, BigInteger batchNumber, byte[] dataAvailabilityRoot, boolean replayed,
        boolean dataPossessed, int availabilityClassMask, BigInteger attestedAtMs, byte[] signer,
        byte[] signature, int signatureV) {}
    public record GuarantorKey(byte[] guarantorId, byte[] publicKey, boolean bonded) {}
    public record CheckpointCertificate(byte[] canonicalHeader, byte[] validityProof,
        List<CheckpointAttestation> attestations, int threshold, byte[] settlementReference) {
        public CheckpointCertificate { attestations = List.copyOf(attestations); }
    }
    public record CheckpointVerificationInput(CheckpointCertificate certificate, List<GuarantorKey> bondedSet,
        byte[] registeredCheckpointId, BigInteger expectedPaxeerChainId, byte[] expectedSettlementContract,
        byte[] registeredSettlementReference, boolean availabilityObtained) {
        public CheckpointVerificationInput { bondedSet = List.copyOf(bondedSet); }
    }
    public record CheckpointVerification(VerificationLevel level, byte[] checkpointId, int achieved,
        int required, BatchHeader header) {}
    @FunctionalInterface public interface LocalSignatureVerifier {
        boolean verifyRecoverableSecp256k1(byte[] publicKey, byte[] signature, int signatureV,
            byte[] signer, byte[] digest);
    }
    public record ReceiptEffect(int moduleId, int ordinal, int eventType, int kind, boolean monetary,
        byte[] transferSetRoot, byte[] body) {}
    public record ProgramReceiptOutcome(int encodingVersion, int terminalKind, int resultCode,
        int runtimeVersion, int abiVersion, long feeScheduleVersion, long meteringScheduleVersion,
        BigInteger cpuFuel, BigInteger memoryBytes, BigInteger storageReadBytes,
        BigInteger storageWriteBytes, long outputValues, BigInteger outputBytes,
        BigInteger occupancyByteBatches, BigInteger occupancyFeeUnits, List<BigInteger> feeSchedulePrices,
        byte[] occupancyAssetId, byte[] occupancyEvidenceDigest, byte[] occupancyTransferRoot,
        BigInteger feeUnits, byte[] callGraphRoot, byte[] terminalPayloadRoot, byte[] transferRoot, byte[] appliedLegsDigest) {
        public ProgramReceiptOutcome { feeSchedulePrices = List.copyOf(feeSchedulePrices); }
    }
    public record ProtocolReceipt(int protocolVersion, byte[] activityId, BigInteger globalSequence,
        byte[] previousStateRoot, byte[] resultingStateRoot, byte[] activityRoot, int resultCode,
        List<ReceiptEffect> effects, BigInteger feeCharged, byte[] batchId, int moduleId,
        long moduleVersion, long parameterVersion, int operation, byte[] asset, BigInteger amount,
        byte[] from, BigInteger fromBalanceBefore, BigInteger fromBalanceAfter, BigInteger fromSequence,
        byte[] to, BigInteger toBalanceBefore, BigInteger toBalanceAfter, byte[] transferSetRoot,
        byte[] authorizationHash, byte[] contextHash, BigInteger timestamp,
        ProgramReceiptOutcome programOutcome, byte[] sequencerSignature) {
        public ProtocolReceipt { effects = List.copyOf(effects); }
    }
    public record AuthorizedReceiptBatch(byte[] batchId, byte[] asset, byte[] previousStateRoot,
        byte[] resultingStateRoot, byte[] sequencerPublicKey) {}
    public record ReceiptVerification(VerificationLevel level, ProtocolReceipt receipt,
        byte[] canonicalBytes, byte[] receiptDigest) {}
    public record ReceiptInclusionVerification(ReceiptVerification receipt,
        InclusionVerification inclusion) {}
    private record DecodedReceipt(ProtocolReceipt receipt, byte[] unsignedBytes) {}

    public record ReceiptKey(byte[] keyId, byte[] publicKey, BigInteger firstHeight,
                             BigInteger lastHeight) {
        public ReceiptKey {
            if (keyId == null || keyId.length != 32 || allZero(keyId)
                    || publicKey == null || publicKey.length != 32 || allZero(publicKey)
                    || firstHeight == null || lastHeight == null || firstHeight.signum() < 0
                    || lastHeight.bitLength() > 64 || firstHeight.compareTo(lastHeight) > 0)
                throw PlatformSdkException.invalidArgument();
            keyId = keyId.clone();
            publicKey = publicKey.clone();
        }
        @Override public byte[] keyId() { return keyId.clone(); }
        @Override public byte[] publicKey() { return publicKey.clone(); }
    }
    public record ReceiptKeys(List<ReceiptKey> entries) {
        public ReceiptKeys {
            if (entries == null || entries.isEmpty()) throw PlatformSdkException.invalidArgument();
            entries = List.copyOf(entries);
            Set<String> seen = new HashSet<>();
            for (ReceiptKey entry : entries) {
                if (!seen.add(java.util.HexFormat.of().formatHex(entry.keyId())))
                    throw PlatformSdkException.invalidArgument();
            }
        }
    }
    public record SequencerReceiptVerification(VerificationLevel level, ProtocolReceipt receipt,
        byte[] canonicalBytes, byte[] receiptDigest, int status, PlatformSdkException revert) {
        public void requireSuccess() {
            if (status == 0) {
                if (revert != null) throw revert;
                throw PlatformSdkException.receiptedRevert(null);
            }
        }
    }

    public static SequencerReceiptVerification verifySequencerReceipt(byte[] receiptBytes,
            byte[] rawTransaction, ReceiptKeys keys) {
        if (keys == null) throw PlatformSdkException.invalidArgument();
        DecodedReceipt decoded = decodeSequencerReceipt(receiptBytes);
        ProtocolReceipt receipt = decoded.receipt();
        if (receipt.protocolVersion() != GeneratedReceiptContract.RECEIPT_VERSION_V2
                && receipt.protocolVersion() != GeneratedReceiptContract.RECEIPT_VERSION_V3)
            fail(ReceiptCheck.RECEIPT_VERSION);
        if (allZero(receipt.activityId())) fail(ReceiptCheck.ACTIVITY_ID);
        ReceiptKey key = keys.entries().stream()
            .filter(candidate -> equal(candidate.keyId(), receipt.contextHash()))
            .findFirst().orElseThrow(() -> failure(ReceiptCheck.KEY_ID));
        if (!allZero(Arrays.copyOf(receipt.batchId(), 24))) fail(ReceiptCheck.KEY_HEIGHT);
        BigInteger height = new BigInteger(1, Arrays.copyOfRange(receipt.batchId(), 24, 32));
        if (height.compareTo(key.firstHeight()) < 0 || height.compareTo(key.lastHeight()) > 0)
            fail(ReceiptCheck.KEY_HEIGHT);
        byte[] digest = sha256(RECEIPT_DOMAIN, decoded.unsignedBytes());
        if (!verifyEd25519(key.publicKey(), receipt.sequencerSignature(), digest))
            fail(ReceiptCheck.SIGNATURE);
        if (rawTransaction == null || rawTransaction.length == 0 || rawTransaction.length > MAX_MESSAGE_BYTES)
            fail(ReceiptCheck.RAW_TRANSACTION);
        KeccakDigest keccak = new KeccakDigest(256);
        keccak.update(rawTransaction, 0, rawTransaction.length);
        byte[] transactionHash = new byte[32];
        keccak.doFinal(transactionHash, 0);
        if (!equal(transactionHash, receipt.activityId())) fail(ReceiptCheck.RAW_TRANSACTION);
        if (receipt.globalSequence().signum() == 0 || receipt.timestamp().signum() == 0
                || receipt.moduleId() <= 11 || receipt.moduleVersion() != 1
                || receipt.parameterVersion() != 0 || receipt.operation() != 2
                || receipt.programOutcome() != null || receipt.resultCode() < 0 || receipt.resultCode() > 1
                || !allZero(receipt.transferSetRoot()) || !allZero(receipt.authorizationHash()))
            fail(ReceiptCheck.FIELDS);
        verifySequencerEffectLayout(receipt);
        if (!equal(receipt.resultingStateRoot(), sha256(
                ascii(GeneratedReceiptContract.SEQUENCE_DOMAIN + "\0"), receipt.previousStateRoot(),
                unsigned(receipt.globalSequence(), 8), receipt.activityId()))) fail(ReceiptCheck.SEQUENCE_HASH);
        return new SequencerReceiptVerification(VerificationLevel.RECEIPTED, receipt, receiptBytes.clone(),
            digest, receipt.resultCode() == 0 ? 1 : 0, null);
    }

    public static SequencerReceiptVerification verifySequencerReceipt(byte[] receiptBytes,
            byte[] rawTransaction, ReceiptKeys keys, JsonNode view, byte[] previousSequenceHash) {
        SequencerReceiptVerification verified = verifySequencerReceipt(receiptBytes, rawTransaction, keys);
        verifySequencerView(verified, view);
        if (previousSequenceHash == null || previousSequenceHash.length != 32
                || !equal(previousSequenceHash, verified.receipt().previousStateRoot()))
            fail(ReceiptCheck.SEQUENCE_HASH);
        PlatformSdkException revert = verified.status() == 0
            ? PlatformSdkException.receiptedRevert(decodeRevertReason(jsonBytes(view, "returnData", -1,
                ReceiptCheck.RETURN_DATA))) : null;
        return new SequencerReceiptVerification(verified.level(), verified.receipt(), verified.canonicalBytes(),
            verified.receiptDigest(), verified.status(), revert);
    }

    private static DecodedReceipt decodeSequencerReceipt(byte[] bytes) {
        if (bytes == null || bytes.length == 0 || bytes.length > MAX_MESSAGE_BYTES) fail(ReceiptCheck.DECODE);
        try {
            Decoder d = new Decoder(bytes);
            int envelope = d.u16();
            if (d.u16() != 0x5201) fail(ReceiptCheck.DECODE);
            int version = d.u16();
            if (version != envelope) fail(ReceiptCheck.RECEIPT_VERSION);
            byte[] activity = d.bounded(32);
            BigInteger sequence = d.integer(8);
            byte[] previous = d.bounded(32), resulting = d.bounded(32), activityRoot = d.bounded(32);
            int result = d.i32();
            long count = d.u32();
            if (count > MAX_EFFECTS) fail(ReceiptCheck.DECODE);
            List<ReceiptEffect> effects = new ArrayList<>((int) count);
            for (int i = 0; i < count; i++) {
                int module = d.u16(), ordinal = d.u16(), event = d.u16(), kind = d.u8(), monetary = d.u8();
                if (kind < 1 || kind > 3 || monetary > 1 || monetary == 1 && kind != 2)
                    fail(ReceiptCheck.CANONICAL_ENCODING);
                effects.add(new ReceiptEffect(module, ordinal, event, kind, monetary == 1,
                    d.bounded(32), d.boundedAtMost(MAX_EFFECT_BODY)));
            }
            BigInteger fee = d.integer(16);
            byte[] batch = d.bounded(32);
            int module = d.u16();
            long moduleVersion = d.u32(), parameters = d.u32();
            int operation = d.u8();
            byte[] asset = d.bounded(32);
            BigInteger amount = d.integer(16);
            byte[] from = d.bounded(32);
            BigInteger fromBefore = d.integer(16), fromAfter = d.integer(16), nonce = d.integer(8);
            byte[] to = d.bounded(32);
            BigInteger toBefore = d.integer(16), toAfter = d.integer(16);
            byte[] transfers = d.bounded(32), authorization = d.bounded(32), context = d.bounded(32);
            BigInteger timestamp = d.integer(8);
            int flag = d.u8();
            if (flag > 1) fail(ReceiptCheck.CANONICAL_ENCODING);
            if (flag == 0) fail(ReceiptCheck.SIGNATURE);
            byte[] signature = d.bounded(64);
            d.finish();
            ProtocolReceipt receipt = new ProtocolReceipt(version, activity, sequence, previous, resulting,
                activityRoot, result, effects, fee, batch, module, moduleVersion, parameters, operation,
                asset, amount, from, fromBefore, fromAfter, nonce, to, toBefore, toAfter, transfers,
                authorization, context, timestamp, null, signature);
            byte[] unsigned = encodeSequencerReceiptUnsigned(receipt);
            byte[] canonical = concat(Arrays.copyOf(unsigned, unsigned.length - 1), new byte[]{1},
                u32(signature.length), signature);
            if (!equal(bytes, canonical)) fail(ReceiptCheck.CANONICAL_ENCODING);
            return new DecodedReceipt(receipt, unsigned);
        } catch (PlatformSdkException error) {
            if (error.receiptCheck() != null) throw error;
            throw failure(ReceiptCheck.DECODE);
        }
    }

    private static byte[] encodeSequencerReceiptUnsigned(ProtocolReceipt r) {
        var out = new ByteArrayOutputStream();
        out.writeBytes(unsigned(BigInteger.valueOf(r.protocolVersion()), 2));
        out.writeBytes(new byte[]{0x52, 1});
        out.writeBytes(unsigned(BigInteger.valueOf(r.protocolVersion()), 2));
        writeBounded(out, r.activityId()); out.writeBytes(unsigned(r.globalSequence(), 8));
        writeBounded(out, r.previousStateRoot()); writeBounded(out, r.resultingStateRoot());
        writeBounded(out, r.activityRoot()); out.writeBytes(u32(r.resultCode()));
        out.writeBytes(u32(r.effects().size()));
        for (ReceiptEffect effect : r.effects()) {
            out.writeBytes(unsigned(BigInteger.valueOf(effect.moduleId()), 2));
            out.writeBytes(unsigned(BigInteger.valueOf(effect.ordinal()), 2));
            out.writeBytes(unsigned(BigInteger.valueOf(effect.eventType()), 2));
            out.write(effect.kind()); out.write(effect.monetary() ? 1 : 0);
            writeBounded(out, effect.transferSetRoot()); writeBounded(out, effect.body());
        }
        out.writeBytes(unsigned(r.feeCharged(), 16)); writeBounded(out, r.batchId());
        out.writeBytes(unsigned(BigInteger.valueOf(r.moduleId()), 2));
        out.writeBytes(u32((int) r.moduleVersion())); out.writeBytes(u32((int) r.parameterVersion()));
        out.write(r.operation()); writeBounded(out, r.asset()); out.writeBytes(unsigned(r.amount(), 16));
        writeBounded(out, r.from()); out.writeBytes(unsigned(r.fromBalanceBefore(), 16));
        out.writeBytes(unsigned(r.fromBalanceAfter(), 16)); out.writeBytes(unsigned(r.fromSequence(), 8));
        writeBounded(out, r.to()); out.writeBytes(unsigned(r.toBalanceBefore(), 16));
        out.writeBytes(unsigned(r.toBalanceAfter(), 16)); writeBounded(out, r.transferSetRoot());
        writeBounded(out, r.authorizationHash()); writeBounded(out, r.contextHash());
        out.writeBytes(unsigned(r.timestamp(), 8)); out.write(0);
        return out.toByteArray();
    }

    private static void writeBounded(ByteArrayOutputStream out, byte[] bytes) {
        out.writeBytes(u32(bytes.length)); out.writeBytes(bytes);
    }

    private static void verifySequencerEffectLayout(ProtocolReceipt receipt) {
        if (receipt.effects().isEmpty()) fail(ReceiptCheck.FIELDS);
        ReceiptEffect summary = receipt.effects().get(0);
        verifySequencerEffect(summary, receipt.moduleId(), 0, 1, summary.body(), ReceiptCheck.FIELDS);
        try {
            Decoder d = new Decoder(summary.body());
            d.fixed(8 * 3 + 4 + 20 * 2 + 32);
            long returnLength = d.u32(), logCount = d.u32();
            int denomLength = d.u8();
            if (returnLength > MAX_MESSAGE_BYTES || logCount != receipt.effects().size() - 1
                    || denomLength == 0 || denomLength > 64) fail(ReceiptCheck.FIELDS);
            byte[] denom = d.fixed(denomLength);
            StandardCharsets.UTF_8.newDecoder().decode(ByteBuffer.wrap(denom));
            if (!equal(sha256(denom), receipt.asset())) fail(ReceiptCheck.FIELDS);
            d.finish();
        } catch (java.nio.charset.CharacterCodingException | PlatformSdkException error) {
            throw failure(ReceiptCheck.FIELDS);
        }
        for (int i = 1; i < receipt.effects().size(); i++) {
            ReceiptEffect effect = receipt.effects().get(i);
            verifySequencerEffect(effect, receipt.moduleId(), i, 3, effect.body(), ReceiptCheck.LOGS);
            byte[] body = effect.body();
            if (body.length < 21 || (body[20] & 0xff) > 4
                    || body.length != 21 + ((body[20] & 0xff) + 1) * 32) fail(ReceiptCheck.LOGS);
        }
    }

    private static void verifySequencerView(SequencerReceiptVerification verified, JsonNode view) {
        ReceiptCheck fields = ReceiptCheck.FIELDS;
        if (view == null || !view.isObject()) fail(fields);
        ProtocolReceipt r = verified.receipt();
        if (!"receipted".equals(jsonText(view, "rung", fields))
                || !equal(jsonBytes(view, "receipt", -1, fields), verified.canonicalBytes())
                || !equal(jsonBytes(view, "txHash", 32, fields), r.activityId())
                || !jsonQuantity(view, "seq", 8, fields).equals(r.globalSequence())
                || !jsonQuantity(view, "height", 8, fields).equals(new BigInteger(1, r.batchId()))
                || !jsonQuantity(view, "timestamp", 8, fields).equals(r.timestamp())
                || !jsonQuantity(view, "nonce", 8, fields).equals(r.fromSequence())
                || !jsonQuantity(view, "status", 1, fields).equals(BigInteger.valueOf(verified.status()))
                || !jsonQuantity(view, "feeAmount", 16, fields).equals(r.feeCharged())
                || !equal(jsonBytes(view, "keyId", 32, fields), r.contextHash())
                || !equal(concat(new byte[12], jsonBytes(view, "from", 20, fields)), r.from())
                || !equal(concat(new byte[12], jsonBytes(view, "to", 20, fields)), r.to())) fail(fields);
        byte[] returned = jsonBytes(view, "returnData", -1, ReceiptCheck.RETURN_DATA);
        if (!equal(sha256(returned), r.activityRoot())) fail(ReceiptCheck.RETURN_DATA);
        JsonNode logs = view.get("logs");
        if (logs == null || !logs.isArray() || logs.size() >= MAX_EFFECTS
                || r.effects().size() != logs.size() + 1) fail(ReceiptCheck.LOGS);
        byte[] denom = ascii(jsonText(view, "feeDenom", fields));
        if (denom.length < 1 || denom.length > 64 || !equal(sha256(denom), r.asset())) fail(fields);
        byte[] summary = concat(unsigned(jsonQuantity(view, "gasUsed", 8, fields), 8),
            unsigned(jsonQuantity(view, "cumulativeGasUsed", 8, fields), 8),
            unsigned(jsonQuantity(view, "effectiveGasPrice", 8, fields), 8),
            unsigned(jsonQuantity(view, "index", 4, fields), 4),
            jsonBytes(view, "contractAddress", 20, fields), jsonBytes(view, "sponsor", 20, fields),
            sha256(ascii(jsonText(view, "vmError", fields))), u32(returned.length), u32(logs.size()),
            new byte[]{(byte) denom.length}, denom);
        verifySequencerEffect(r.effects().get(0), r.moduleId(), 0, 1, summary, fields);
        for (int i = 0; i < logs.size(); i++) {
            JsonNode log = logs.get(i), topics = log.get("topics");
            if (topics == null || !topics.isArray() || topics.size() > 4
                    || !jsonQuantity(log, "logIndex", 8, ReceiptCheck.LOGS).equals(BigInteger.valueOf(i)))
                fail(ReceiptCheck.LOGS);
            var body = new ByteArrayOutputStream();
            body.writeBytes(jsonBytes(log, "address", 20, ReceiptCheck.LOGS)); body.write(topics.size());
            for (JsonNode topic : topics) body.writeBytes(parseJsonBytes(topic, 32, ReceiptCheck.LOGS));
            body.writeBytes(sha256(jsonBytes(log, "data", -1, ReceiptCheck.LOGS)));
            verifySequencerEffect(r.effects().get(i + 1), r.moduleId(), i + 1, 3,
                body.toByteArray(), ReceiptCheck.LOGS);
        }
    }

    private static void verifySequencerEffect(ReceiptEffect effect, int module, int ordinal,
            int kind, byte[] body, ReceiptCheck check) {
        if (effect.moduleId() != module || effect.ordinal() != ordinal || effect.eventType() != 0
                || effect.kind() != kind || effect.monetary() || !allZero(effect.transferSetRoot())
                || !equal(effect.body(), body)) fail(check);
    }

    private static String jsonText(JsonNode view, String field, ReceiptCheck check) {
        JsonNode value = view == null ? null : view.get(field);
        if (value == null || !value.isTextual()) throw failure(check);
        return value.textValue();
    }

    private static BigInteger jsonQuantity(JsonNode view, String field, int bytes, ReceiptCheck check) {
        String text = jsonText(view, field, check);
        if (text.length() > 2 + bytes * 2 || !text.matches("0x(?:0|[1-9a-fA-F][0-9a-fA-F]*)"))
            throw failure(check);
        return new BigInteger(text.substring(2), 16);
    }

    private static byte[] jsonBytes(JsonNode view, String field, int length, ReceiptCheck check) {
        return parseJsonBytes(view == null ? null : view.get(field), length, check);
    }

    private static byte[] parseJsonBytes(JsonNode node, int length, ReceiptCheck check) {
        if (node == null || !node.isTextual()) throw failure(check);
        String text = node.textValue();
        if (text.length() > 2 + MAX_MESSAGE_BYTES * 2 || !text.startsWith("0x")
                || (text.length() & 1) != 0 || length >= 0 && text.length() != 2 + length * 2)
            throw failure(check);
        try {
            return hex(text.substring(2));
        } catch (IllegalArgumentException error) {
            throw failure(check);
        }
    }

    private static String decodeRevertReason(byte[] data) {
        if (data.length < 4 || !equal(Arrays.copyOf(data, 4), hex("08c379a0"))) return null;
        if (data.length < 68 || !new BigInteger(1, Arrays.copyOfRange(data, 4, 36)).equals(BigInteger.valueOf(32)))
            return null;
        BigInteger size = new BigInteger(1, Arrays.copyOfRange(data, 36, 68));
        if (size.bitLength() > 20) return null;
        int length = size.intValue(), padded = (length + 31) / 32 * 32;
        if (data.length != 68 + padded || !allZero(Arrays.copyOfRange(data, 68 + length, data.length))) return null;
        try {
            return StandardCharsets.UTF_8.newDecoder().decode(ByteBuffer.wrap(data, 68, length)).toString();
        } catch (java.nio.charset.CharacterCodingException error) {
            return null;
        }
    }

    public static void verifyMerkleInclusion(byte[] canonicalLeaf, MerkleProof proof, byte[] expectedRoot) {
        Objects.requireNonNull(canonicalLeaf, "canonicalLeaf");
        Objects.requireNonNull(proof, "proof");
        if (proof.leafCount() <= 0 || proof.leafCount() > 0xffff_ffffL || proof.leafIndex() < 0
                || proof.leafIndex() >= proof.leafCount() || proof.siblings().size() > 32
                || proof.siblings().size() != proofDepth(proof.leafCount())) fail();
        byte[] current = sha256(MERKLE_LEAF_DOMAIN, canonicalLeaf);
        long index = proof.leafIndex();
        long levelCount = proof.leafCount();
        for (byte[] siblingValue : proof.siblings()) {
            byte[] sibling = exact(siblingValue, 32);
            if ((index ^ 1) >= levelCount && !equal(sibling, current)) fail();
            current = (index & 1) == 0
                ? sha256(MERKLE_INTERNAL_DOMAIN, current, sibling)
                : sha256(MERKLE_INTERNAL_DOMAIN, sibling, current);
            index /= 2;
            levelCount = (levelCount + 1) / 2;
        }
        if (!equal(current, exact(expectedRoot, 32))) fail();
    }

    public static BatchHeader decodeBatchHeader(byte[] canonicalHeader) {
        if (canonicalHeader == null || canonicalHeader.length != BATCH_HEADER_BYTES) fail();
        Decoder decoder = new Decoder(canonicalHeader);
        if (decoder.u16() != 1 || decoder.u16() != 0x1701 || decoder.u8() != 15) fail();
        field(decoder, 1); int protocolVersion = decoder.u16();
        field(decoder, 2); long networkId = decoder.u32();
        field(decoder, 3); BigInteger epoch = decoder.integer(8);
        field(decoder, 4); BigInteger batchNumber = decoder.integer(8);
        field(decoder, 5); BigInteger firstSequence = decoder.integer(8);
        field(decoder, 6); BigInteger lastSequence = decoder.integer(8);
        field(decoder, 7); byte[] previousStateRoot = decoder.bounded(32);
        field(decoder, 8); byte[] resultingStateRoot = decoder.bounded(32);
        field(decoder, 9); byte[] activityMerkleRoot = decoder.bounded(32);
        field(decoder, 10); byte[] receiptMerkleRoot = decoder.bounded(32);
        field(decoder, 11); byte[] eventMerkleRoot = decoder.bounded(32);
        field(decoder, 12); byte[] dataAvailabilityRoot = decoder.bounded(32);
        field(decoder, 13); byte[] oracleRoot = decoder.bounded(32);
        field(decoder, 14); BigInteger timestampMs = decoder.integer(8);
        field(decoder, 15); byte[] sequencerId = decoder.bounded(32);
        decoder.finish();
        return new BatchHeader(protocolVersion, networkId, epoch, batchNumber, firstSequence, lastSequence,
            previousStateRoot, resultingStateRoot, activityMerkleRoot, receiptMerkleRoot, eventMerkleRoot,
            dataAvailabilityRoot, oracleRoot, timestampMs, sequencerId);
    }

    public static InclusionVerification verifyBatchInclusion(InclusionKind kind, byte[] canonicalLeaf,
        MerkleProof proof, byte[] canonicalHeader, byte[] headerSignature, SequencerAuthorization authorization, int... selectedProtocol) {
        BatchHeader header = decodeBatchHeader(canonicalHeader);
        if (!selectedProtocolMatches(header.protocolVersion(), selectedProtocol)) fail();
        if (header.batchNumber().compareTo(authorization.firstBatchNumber()) < 0
                || header.batchNumber().compareTo(authorization.lastBatchNumber()) > 0
                || !equal(header.sequencerId(), exact(authorization.sequencerId(), 32))) fail();
        byte[] digest = sha256(BATCH_HEADER_DOMAIN, canonicalHeader);
        if (!verifyEd25519(authorization.publicKey(), headerSignature, digest)) fail();
        byte[] root = switch (kind) {
            case ACTIVITY -> header.activityMerkleRoot();
            case RECEIPT -> header.receiptMerkleRoot();
            case EVENT -> header.eventMerkleRoot();
            case STATE -> header.resultingStateRoot();
        };
        verifyMerkleInclusion(canonicalLeaf, proof, root);
        return new InclusionVerification(kind == InclusionKind.STATE ? VerificationLevel.STATE_PROVEN
            : VerificationLevel.BATCH_INCLUDED, header, digest, root.clone());
    }

    public static CheckpointVerification verifyCheckpoint(CheckpointVerificationInput input,
                                                            LocalSignatureVerifier signatures, int... selectedProtocol) {
        Objects.requireNonNull(input, "input"); Objects.requireNonNull(signatures, "signatures");
        CheckpointCertificate certificate = input.certificate();
        if (!input.availabilityObtained() || certificate.validityProof().length > 0xffff_ffffL) fail();
        BatchHeader header = decodeBatchHeader(certificate.canonicalHeader());
        if (!selectedProtocolMatches(header.protocolVersion(), selectedProtocol)) fail();
        byte[] checkpointId = sha256(CHECKPOINT_DOMAIN, certificate.canonicalHeader(),
            u32(certificate.validityProof().length), certificate.validityProof());
        byte[] expectedSettlementContract = exact(input.expectedSettlementContract(), 20);
        if (!equal(checkpointId, exact(input.registeredCheckpointId(), 32)) || certificate.threshold() <= 0
                || input.expectedPaxeerChainId() == null || input.expectedPaxeerChainId().signum() <= 0
                || allZero(expectedSettlementContract)) fail();
        Set<String> seen = new HashSet<>();
        int achieved = 0;
        for (CheckpointAttestation attestation : certificate.attestations()) {
            byte[] guarantorId = exact(attestation.guarantorId(), 32);
            byte[] attestationSettlementContract = exact(attestation.settlementContract(), 20);
            if (!seen.add(java.util.HexFormat.of().formatHex(guarantorId))
                    || attestation.protocolVersion() != header.protocolVersion()
                    || attestation.networkId() != header.networkId()
                    || !attestation.epoch().equals(header.epoch())
                    || !attestation.paxeerChainId().equals(input.expectedPaxeerChainId())
                    || !equal(attestationSettlementContract, expectedSettlementContract)
                    || !equal(attestation.checkpointId(), checkpointId)
                    || !equal(attestation.checkpointHash(), checkpointId)
                    || !attestation.batchNumber().equals(header.batchNumber())
                    || !equal(attestation.dataAvailabilityRoot(), header.dataAvailabilityRoot())
                    || !attestation.replayed() || !attestation.dataPossessed()
                    || attestation.availabilityClassMask() != ALL_AVAILABILITY_CLASSES
                    || attestation.attestedAtMs().signum() <= 0
                    || allZero(exact(attestation.signer(), 20))
                    || (attestation.signatureV() != 27 && attestation.signatureV() != 28)) fail();
            GuarantorKey member = input.bondedSet().stream().filter(candidate -> candidate.bonded()
                && equal(candidate.guarantorId(), guarantorId)).findFirst().orElseThrow(LocalVerifier::failure);
            byte[] digest = sha256(GUARANTOR_ATTESTATION_DOMAIN, attestationMessage(attestation));
            if (!signatures.verifyRecoverableSecp256k1(exact(member.publicKey(), 33),
                    exact(attestation.signature(), 64), attestation.signatureV(),
                    exact(attestation.signer(), 20), digest)) fail();
            achieved++;
        }
        if (achieved < certificate.threshold()) fail();
        byte[] settlement = certificate.settlementReference();
        if (settlement != null && (settlement.length == 0 || input.registeredSettlementReference() == null
                || !equal(settlement, input.registeredSettlementReference()))) fail();
        return new CheckpointVerification(settlement == null ? VerificationLevel.CHECKPOINT_FINALISED
            : VerificationLevel.SETTLEMENT_ANCHORED, checkpointId, achieved, certificate.threshold(), header);
    }

    public static CheckpointVerification verifyCheckpoint(CheckpointVerificationInput input) {
        return verifyCheckpoint(input, LocalVerifier::verifyRecoverableSecp256k1);
    }

    /** Production recoverable secp256k1 verifier for registered EVM attestations. */
    public static boolean verifyRecoverableSecp256k1(byte[] publicKey, byte[] signature,
                                                       int signatureV, byte[] signer, byte[] digest) {
        try {
            exact(publicKey, 33); exact(signature, 64); exact(signer, 20); exact(digest, 32);
            if (signatureV != 27 && signatureV != 28) return false;
            var params = CustomNamedCurves.getByName("secp256k1");
            var domain = new ECDomainParameters(params.getCurve(), params.getG(), params.getN(), params.getH());
            var key = new ECPublicKeyParameters(params.getCurve().decodePoint(publicKey), domain);
            var r = new BigInteger(1, Arrays.copyOfRange(signature, 0, 32));
            var s = new BigInteger(1, Arrays.copyOfRange(signature, 32, 64));
            if (r.signum() <= 0 || r.compareTo(params.getN()) >= 0 || s.signum() <= 0
                    || s.compareTo(params.getN().shiftRight(1)) > 0) return false;
            var verifier = new ECDSASigner();
            verifier.init(false, key);
            if (!verifier.verifySignature(digest, r, s)) return false;
            byte[] compressed = new byte[33];
            compressed[0] = (byte)(2 + (signatureV - 27));
            byte[] x = unsigned(r, 32);
            System.arraycopy(x, 0, compressed, 1, x.length);
            ECPoint recoveredR = params.getCurve().decodePoint(compressed);
            if (!recoveredR.multiply(params.getN()).isInfinity()) return false;
            BigInteger inverseR = r.modInverse(params.getN());
            BigInteger e = new BigInteger(1, digest);
            ECPoint recovered = ECAlgorithms.sumOfTwoMultiplies(
                params.getG(), e.negate().mod(params.getN()).multiply(inverseR).mod(params.getN()),
                recoveredR, s.multiply(inverseR).mod(params.getN())).normalize();
            if (!equal(recovered.getEncoded(true), publicKey)) return false;
            byte[] uncompressed = recovered.getEncoded(false);
            KeccakDigest keccak = new KeccakDigest(256);
            keccak.update(uncompressed, 1, uncompressed.length - 1);
            byte[] addressHash = new byte[32];
            keccak.doFinal(addressHash, 0);
            return equal(Arrays.copyOfRange(addressHash, 12, 32), signer);
        } catch (RuntimeException error) {
            return false;
        }
    }

    public static ReceiptVerification verifyReceiptOutcome(byte[] canonicalReceipt,
                                                              AuthorizedReceiptBatch authorized, int... selectedProtocol) {
        return verifyReceiptOutcomeInternal(canonicalReceipt, authorized, false, selectedProtocol);
    }

    public static ReceiptVerification verifyProgramTerminalV5Receipt(byte[] canonicalReceipt,
            AuthorizedReceiptBatch authorized, byte[] expectedActivity, NativeProgramCall expectedCall,
            byte[] signedActivity, ObjectNode outcome, byte[] terminal, byte[] graph) {
        if (outcome == null) fail(ReceiptCheck.FIELDS);
        return verifyProgramTerminalV5ReceiptInternal(canonicalReceipt, authorized, expectedActivity,
            expectedCall, signedActivity, outcome, terminal, graph, false);
    }

    public static ReceiptVerification verifyProgramTerminalV5Receipt(byte[] canonicalReceipt,
            AuthorizedReceiptBatch authorized, byte[] expectedActivity, NativeProgramCall expectedCall,
            byte[] signedActivity, byte[] terminal, byte[] graph) {
        return verifyProgramTerminalV5ReceiptInternal(canonicalReceipt, authorized, expectedActivity,
            expectedCall, signedActivity, null, terminal, graph, true);
    }

    private static ReceiptVerification verifyProgramTerminalV5ReceiptInternal(byte[] canonicalReceipt,
            AuthorizedReceiptBatch authorized, byte[] expectedActivity, NativeProgramCall expectedCall,
            byte[] signedActivity, ObjectNode outcome, byte[] terminal, byte[] graph, boolean capturedOutcome) {
        if (expectedCall == null) fail(ReceiptCheck.FIELDS);
        int expectedGuestAbi = expectedCall.guestAbi();
        byte[] expectedProgram = expectedCall.programId();
        try { expectedCall.bindSignedActivity(signedActivity); }
        catch (RuntimeException error) { fail(ReceiptCheck.FIELDS); }
        if (!equal(sha256("LXP/v1/activity-id\0".getBytes(StandardCharsets.UTF_8), signedActivity),
                exact(expectedActivity, 32))) fail(ReceiptCheck.ACTIVITY_ID);
        if (expectedGuestAbi != GeneratedReceiptContract.PROGRAM_ABI_V3
                && expectedGuestAbi != GeneratedReceiptContract.PROGRAM_ABI_V4) fail(ReceiptCheck.RECEIPT_VERSION);
        ReceiptVerification verified = verifyReceiptOutcomeInternal(canonicalReceipt, authorized, true, 3);
        ProtocolReceipt receipt = verified.receipt();
        if (!equal(receipt.asset(), exact(authorized.asset(), 32))) fail(ReceiptCheck.FIELDS);
        ProgramReceiptOutcome program = receipt.programOutcome();
        if (receipt.moduleId() != 9 || receipt.operation() != 3 || program == null
                || program.abiVersion() != expectedGuestAbi || program.resultCode() != receipt.resultCode()
                || !equal(receipt.activityId(), exact(expectedActivity, 32))
                || terminal == null || terminal.length == 0 || terminal.length > ProgramsClient.MAX_CALLDATA_BYTES
                || graph == null || graph.length == 0 || graph.length > ProgramsClient.MAX_CALLDATA_BYTES
                || !equal(sha256(terminal), program.terminalPayloadRoot())
                || !equal(sha256(graph), program.callGraphRoot())) fail(ReceiptCheck.FIELDS);
        if (capturedOutcome) ProgramsClient.verifyTerminalV5Capture(terminal, graph, exact(expectedProgram, 32), program);
        else ProgramsClient.verifyTerminalV5(terminal, graph, exact(expectedProgram, 32), outcome, program);
        return verified;
    }

    private static ReceiptVerification verifyReceiptOutcomeInternal(byte[] canonicalReceipt,
            AuthorizedReceiptBatch authorized, boolean terminalV5, int... selectedProtocol) {
        DecodedReceipt decoded = decodeProtocolReceipt(canonicalReceipt);
        ProtocolReceipt receipt = decoded.receipt();
        if (!selectedProtocolMatches(receipt.protocolVersion(), selectedProtocol)) fail(ReceiptCheck.RECEIPT_VERSION);
        exact(authorized.asset(), 32);
        boolean program = receipt.moduleId() == 9 && (receipt.operation() == 0 || receipt.operation() == 3);
        if (program) {
            long module = receipt.moduleVersion();
            boolean validModule = receipt.protocolVersion() == 3 ? module == 4 || module == 5
                : module == 2 || module == 3 || receipt.operation() == 3 && module == 1;
            if (!validModule) fail(ReceiptCheck.FIELDS);
            if (receipt.operation() == 0 && receipt.resultCode() != 0) fail(ReceiptCheck.FIELDS);
            if (receipt.operation() == 3) {
                ProgramReceiptOutcome outcome = receipt.programOutcome();
                if (outcome == null) fail(ReceiptCheck.FIELDS);
                if (terminalV5) {
                    if (receipt.protocolVersion() != 3 || outcome.runtimeVersion() != 1
                            || (outcome.abiVersion() != GeneratedReceiptContract.PROGRAM_ABI_V3
                                && outcome.abiVersion() != GeneratedReceiptContract.PROGRAM_ABI_V4
                                && outcome.abiVersion() != GeneratedReceiptContract.PROGRAM_ABI_V5))
                        fail(ReceiptCheck.RECEIPT_VERSION);
                    if (outcome.abiVersion() > module) fail(ReceiptCheck.FIELDS);
                } else {
                    if ((outcome.abiVersion() != 1 && outcome.abiVersion() != 2) || outcome.runtimeVersion() != 1)
                        fail(ReceiptCheck.RECEIPT_VERSION);
                }
            }
        }
        if (!program && receipt.operation() == 0) fail(ReceiptCheck.FIELDS);
        if (allZero(receipt.activityId())) fail(ReceiptCheck.ACTIVITY_ID);
        if (!program && allZero(receipt.asset())) fail(ReceiptCheck.FIELDS);
        if (!equal(receipt.batchId(), exact(authorized.batchId(), 32))) fail(ReceiptCheck.FIELDS);
        if (!program && !equal(receipt.asset(), exact(authorized.asset(), 32))) fail(ReceiptCheck.FIELDS);
        if (!equal(receipt.previousStateRoot(), exact(authorized.previousStateRoot(), 32)))
            fail(ReceiptCheck.FIELDS);
        if (!equal(receipt.resultingStateRoot(), exact(authorized.resultingStateRoot(), 32)))
            fail(ReceiptCheck.FIELDS);
        if (!program && receipt.resultCode() == 0) {
            BigInteger expectedFrom = receipt.fromBalanceBefore().subtract(receipt.amount());
            BigInteger expectedTo = receipt.toBalanceBefore().add(receipt.amount());
            if (receipt.fromBalanceBefore().compareTo(receipt.amount()) < 0
                    || !expectedFrom.equals(receipt.fromBalanceAfter())) fail(ReceiptCheck.FIELDS);
            if (expectedTo.compareTo(MAX_U128) > 0 || !expectedTo.equals(receipt.toBalanceAfter()))
                fail(ReceiptCheck.FIELDS);
        }
        byte[] digest = sha256(RECEIPT_DOMAIN, decoded.unsignedBytes());
        if (!verifyEd25519(authorized.sequencerPublicKey(), receipt.sequencerSignature(), digest))
            fail(ReceiptCheck.SIGNATURE);
        return new ReceiptVerification(VerificationLevel.SEQUENCER_SIGNED, receipt,
            canonicalReceipt.clone(), digest);
    }

    public static ReceiptVerification verifyProgramLifecycleReceipt(byte[] canonical, byte[] expectedActivity, byte[] sequencer) {
        DecodedReceipt decoded = decodeProtocolReceipt(canonical);
        ProtocolReceipt receipt = decoded.receipt();
        if (receipt.protocolVersion() != 3 || receipt.moduleId() != 9 || receipt.moduleVersion() != 4
                || receipt.operation() != 0 || receipt.programOutcome() != null || allZero(exact(expectedActivity, 32))
                || !equal(receipt.activityId(), expectedActivity) || allZero(exact(sequencer, 32))) fail(ReceiptCheck.FIELDS);
        byte[] digest = sha256(RECEIPT_DOMAIN, decoded.unsignedBytes());
        if (!verifyEd25519(sequencer, receipt.sequencerSignature(), digest)) fail(ReceiptCheck.SIGNATURE);
        return new ReceiptVerification(VerificationLevel.SEQUENCER_SIGNED, receipt, canonical.clone(), digest);
    }

    public static ReceiptVerification verifyReceipt(byte[] canonicalReceipt, AuthorizedReceiptBatch authorized, int... selectedProtocol) {
        ReceiptVerification verified = verifyReceiptOutcome(canonicalReceipt, authorized, selectedProtocol);
        if (verified.receipt().resultCode() != 0) fail(ReceiptCheck.FIELDS);
        return verified;
    }

    public static ReceiptInclusionVerification verifyReceiptInBatch(byte[] canonicalReceipt,
        AuthorizedReceiptBatch authorizedReceipt, MerkleProof proof, byte[] canonicalHeader,
        byte[] headerSignature, SequencerAuthorization sequencerAuthorization, int... selectedProtocol) {
        ReceiptVerification receipt = verifyReceipt(canonicalReceipt, authorizedReceipt, selectedProtocol);
        InclusionVerification inclusion = verifyBatchInclusion(InclusionKind.RECEIPT, canonicalReceipt,
            proof, canonicalHeader, headerSignature, sequencerAuthorization, selectedProtocol);
        if (!equal(inclusion.header().previousStateRoot(), authorizedReceipt.previousStateRoot())
                || !equal(inclusion.header().resultingStateRoot(), authorizedReceipt.resultingStateRoot())
                || !equal(inclusion.header().sequencerId(), sequencerAuthorization.sequencerId())) fail();
        return new ReceiptInclusionVerification(receipt, inclusion);
    }

    private static DecodedReceipt decodeProtocolReceipt(byte[] canonicalReceipt) {
        try {
            return decodeProtocolReceiptInner(canonicalReceipt);
        } catch (PlatformSdkException error) {
            if (error.receiptCheck() != null) throw error;
            throw failure(ReceiptCheck.DECODE);
        }
    }

    private static DecodedReceipt decodeProtocolReceiptInner(byte[] canonicalReceipt) {
        if (canonicalReceipt == null || canonicalReceipt.length == 0 || canonicalReceipt.length > MAX_MESSAGE_BYTES)
            fail(ReceiptCheck.FIELDS);
        Decoder d = new Decoder(canonicalReceipt);
        int envelopeVersion = d.u16();
        if ((envelopeVersion != 1 && envelopeVersion != 2 && envelopeVersion != 3) || d.u16() != 0x5201)
            fail(ReceiptCheck.DECODE);
        int protocolVersion = d.u16();
        if (protocolVersion != envelopeVersion) fail(ReceiptCheck.RECEIPT_VERSION);
        byte[] activityId = d.bounded(32); BigInteger globalSequence = d.integer(8);
        byte[] previousStateRoot = d.bounded(32); byte[] resultingStateRoot = d.bounded(32);
        byte[] activityRoot = d.bounded(32); int resultCode = d.i32(); long effectCount = d.u32();
        if (effectCount > MAX_EFFECTS) fail();
        List<ReceiptEffect> effects = new ArrayList<>((int) effectCount);
        for (int i = 0; i < effectCount; i++) {
            int moduleId = d.u16(), ordinal = d.u16(), eventType = d.u16(), kind = d.u8(), monetary = d.u8();
            if (kind < 1 || kind > 3 || monetary > 1 || (monetary == 1 && kind != 2)) fail();
            effects.add(new ReceiptEffect(moduleId, ordinal, eventType, kind, monetary == 1,
                d.bounded(32), d.boundedAtMost(MAX_EFFECT_BODY)));
        }
        BigInteger feeCharged = d.integer(16); byte[] batchId = d.bounded(32); int moduleId = d.u16();
        long moduleVersion = d.u32(), parameterVersion = d.u32(); int operation = d.u8();
        byte[] asset = d.bounded(32); BigInteger amount = d.integer(16); byte[] from = d.bounded(32);
        BigInteger fromBefore = d.integer(16), fromAfter = d.integer(16), fromSequence = d.integer(8);
        byte[] to = d.bounded(32); BigInteger toBefore = d.integer(16), toAfter = d.integer(16);
        byte[] transferSetRoot = d.bounded(32), authorizationHash = d.bounded(32), contextHash = d.bounded(32);
        BigInteger timestamp = d.integer(8);
        if (globalSequence.signum() == 0) fail(ReceiptCheck.FIELDS);
        if (moduleId == 0) fail(ReceiptCheck.FIELDS);
        if (moduleVersion == 0) fail(ReceiptCheck.FIELDS);
        if (timestamp.signum() == 0) fail(ReceiptCheck.FIELDS);
        if (allZero(activityId)) fail(ReceiptCheck.ACTIVITY_ID);
        if (allZero(resultingStateRoot)) fail(ReceiptCheck.FIELDS);
        ProgramReceiptOutcome programOutcome;
        try {
            programOutcome = d.remaining() > 69
                ? decodeProgramReceiptOutcomeFrom(d, protocolVersion) : null;
        } catch (PlatformSdkException error) {
            if (error.receiptCheck() != null) throw error;
            throw failure(ReceiptCheck.FIELDS);
        }
        if (programOutcome != null && (moduleId != 9
                || programOutcome.resultCode() != resultCode
                || (programOutcome.terminalKind() == 1 && !equal(programOutcome.transferRoot(), transferSetRoot))
                || (programOutcome.terminalKind() != 1 && !allZero(transferSetRoot))))
            fail(ReceiptCheck.FIELDS);
        int signatureFlagOffset = d.position();
        if (d.u8() != 1) fail(ReceiptCheck.SIGNATURE);
        byte[] signature = d.bounded(64); d.finish();
        ProtocolReceipt receipt = new ProtocolReceipt(protocolVersion, activityId, globalSequence,
            previousStateRoot, resultingStateRoot, activityRoot, resultCode, effects, feeCharged, batchId,
            moduleId, moduleVersion, parameterVersion, operation, asset, amount, from, fromBefore, fromAfter,
            fromSequence, to, toBefore, toAfter, transferSetRoot, authorizationHash, contextHash, timestamp,
            programOutcome, signature);
        byte[] unsigned = Arrays.copyOf(canonicalReceipt, signatureFlagOffset + 1);
        unsigned[signatureFlagOffset] = 0;
        return new DecodedReceipt(receipt, unsigned);
    }

    private static ProgramReceiptOutcome decodeProgramReceiptOutcomeFrom(Decoder d, int protocolVersion) {
        long tag = d.u32();
        int encodingVersion = tag == PROGRAM_OUTCOME_V1 ? 1
            : tag == PROGRAM_OUTCOME_V2 ? 2 : tag == PROGRAM_OUTCOME_V3 ? 3 : tag == PROGRAM_OUTCOME_V4 ? 4 : 0;
        if (encodingVersion == 0) fail();
        int terminalKind = d.u8(); int resultCode = d.i32(); int runtimeVersion = d.u16();
        int abiVersion = d.u16(); long feeScheduleVersion = d.u32();
        long meteringScheduleVersion = encodingVersion >= 3 ? d.u32() : 1;
        BigInteger cpuFuel = d.integer(8), memoryBytes = d.integer(8);
        BigInteger storageReadBytes = d.integer(8), storageWriteBytes = d.integer(8);
        long outputValues = d.u32(); BigInteger outputBytes = d.integer(8);
        BigInteger occupancyByteBatches = BigInteger.ZERO, occupancyFeeUnits = BigInteger.ZERO;
        List<BigInteger> feeSchedulePrices = new ArrayList<>(7);
        byte[] occupancyAssetId = new byte[32], occupancyEvidenceDigest = new byte[32];
        byte[] occupancyTransferRoot = new byte[32];
        if (encodingVersion >= 2) {
            occupancyByteBatches = d.integer(16); occupancyFeeUnits = d.integer(16);
            for (int index = 0; index < 7; index++) feeSchedulePrices.add(d.integer(8));
            occupancyAssetId = d.bounded(32); occupancyEvidenceDigest = d.bounded(32);
            occupancyTransferRoot = d.bounded(32);
        } else {
            for (int index = 0; index < 7; index++) feeSchedulePrices.add(BigInteger.ZERO);
        }
        BigInteger feeUnits = d.integer(16); byte[] callGraphRoot = d.bounded(32);
        byte[] terminalPayloadRoot = d.bounded(32); byte[] transferRoot = d.bounded(32);
        byte[] appliedLegsDigest = encodingVersion == 4 ? d.bounded(32) : new byte[32];
        boolean occupancyZero = occupancyByteBatches.signum() == 0 && occupancyFeeUnits.signum() == 0
            && allZero(occupancyAssetId) && allZero(occupancyEvidenceDigest) && allZero(occupancyTransferRoot);
        boolean validVersion = protocolVersion == 1 && (encodingVersion == 1 || encodingVersion == 3)
            || (protocolVersion == 2 || protocolVersion == 3) && (encodingVersion == 2 || encodingVersion == 3)
            || protocolVersion == 3 && encodingVersion == 4;
        if ((encodingVersion == 4) == allZero(appliedLegsDigest) || terminalKind < 1 || terminalKind > 3 || runtimeVersion == 0 || abiVersion == 0
                || feeScheduleVersion == 0 || meteringScheduleVersion != 1 || allZero(terminalPayloadRoot)
                || terminalKind == 1 && resultCode != 0
                || terminalKind != 1 && (resultCode == 0 || resultCode <= -1000)
                || terminalKind != 1 && !allZero(transferRoot) || !validVersion
                || encodingVersion == 1 && !occupancyZero
                || encodingVersion >= 2 && terminalKind != 1 && !occupancyZero
                || encodingVersion == 2 && terminalKind == 1
                    && (allZero(occupancyAssetId) || allZero(occupancyEvidenceDigest))
                || encodingVersion >= 3 && allZero(occupancyAssetId) != allZero(occupancyEvidenceDigest)
                || protocolVersion == 1 && encodingVersion >= 3 && !occupancyZero
                || (protocolVersion == 2 || protocolVersion == 3) && encodingVersion >= 3 && terminalKind == 1
                    && (allZero(occupancyAssetId) || allZero(occupancyEvidenceDigest))) fail();
        return new ProgramReceiptOutcome(encodingVersion, terminalKind, resultCode, runtimeVersion,
            abiVersion, feeScheduleVersion, meteringScheduleVersion, cpuFuel, memoryBytes,
            storageReadBytes, storageWriteBytes, outputValues, outputBytes, occupancyByteBatches,
            occupancyFeeUnits, feeSchedulePrices, occupancyAssetId, occupancyEvidenceDigest,
            occupancyTransferRoot, feeUnits, callGraphRoot, terminalPayloadRoot, transferRoot, appliedLegsDigest);
    }

    public static ProgramReceiptOutcome decodeProgramReceiptOutcome(byte[] canonicalOutcome,
                                                                     int protocolVersion) {
        if (canonicalOutcome == null || canonicalOutcome.length == 0
                || canonicalOutcome.length > MAX_MESSAGE_BYTES) fail(ReceiptCheck.FIELDS);
        try {
            Decoder decoder = new Decoder(canonicalOutcome.clone());
            ProgramReceiptOutcome outcome = decodeProgramReceiptOutcomeFrom(decoder, protocolVersion);
            decoder.finish();
            return outcome;
        } catch (PlatformSdkException error) {
            if (error.receiptCheck() != null) throw error;
            throw failure(ReceiptCheck.FIELDS);
        }
    }

    private static byte[] attestationMessage(CheckpointAttestation a) {
        return concat(unsigned(BigInteger.valueOf(a.protocolVersion()), 2),
            unsigned(BigInteger.valueOf(a.networkId()), 4), unsigned(a.paxeerChainId(), 8),
            exact(a.settlementContract(), 20), unsigned(a.epoch(), 8),
            exact(a.checkpointId(), 32), exact(a.checkpointHash(), 32), exact(a.guarantorId(), 32),
            unsigned(a.batchNumber(), 8), exact(a.dataAvailabilityRoot(), 32),
            new byte[]{(byte) (a.replayed() ? 1 : 0), (byte) (a.dataPossessed() ? 1 : 0),
                (byte) a.availabilityClassMask()}, unsigned(a.attestedAtMs(), 8));
    }
    private static int proofDepth(long count) { int depth = 0; while (count > 1) { count = (count + 1) / 2; depth++; } return depth; }
    private static void field(Decoder decoder, int expected) { if (decoder.u8() != expected) fail(); }
    private static boolean allZero(byte[] value) { int aggregate = 0; for (byte item : value) aggregate |= item; return aggregate == 0; }
    private static boolean equal(byte[] left, byte[] right) { return MessageDigest.isEqual(left, right); }
    private static byte[] exact(byte[] value, int length) { if (value == null || value.length != length) fail(); return value; }
    private static byte[] u32(int value) { return ByteBuffer.allocate(4).putInt(value).array(); }
    private static byte[] unsigned(BigInteger value, int length) {
        if (value == null || value.signum() < 0 || value.bitLength() > length * 8) fail();
        byte[] raw = value.toByteArray(), out = new byte[length];
        int copy = Math.min(raw.length, length);
        System.arraycopy(raw, raw.length - copy, out, length - copy, copy);
        return out;
    }
    private static byte[] sha256(byte[]... values) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            for (byte[] value : values) digest.update(value);
            return digest.digest();
        } catch (java.security.GeneralSecurityException impossible) { throw new AssertionError(impossible); }
    }
    private static boolean verifyEd25519(byte[] publicKey, byte[] signature, byte[] digest) {
        try {
            byte[] encoded = concat(ED25519_X509_PREFIX, exact(publicKey, 32));
            var key = KeyFactory.getInstance("Ed25519").generatePublic(new X509EncodedKeySpec(encoded));
            var verifier = Signature.getInstance("Ed25519"); verifier.initVerify(key); verifier.update(exact(digest, 32));
            return verifier.verify(exact(signature, 64));
        } catch (java.security.GeneralSecurityException | RuntimeException error) { return false; }
    }
    private static byte[] concat(byte[]... values) {
        var output = new ByteArrayOutputStream();
        for (byte[] value : values) output.writeBytes(value);
        return output.toByteArray();
    }
    private static byte[] ascii(String value) { return value.getBytes(StandardCharsets.UTF_8); }
    private static byte[] hex(String value) { return java.util.HexFormat.of().parseHex(value); }
    private static PlatformSdkException failure() { return PlatformSdkException.verificationFailure(); }
    private static PlatformSdkException failure(ReceiptCheck check) {
        return PlatformSdkException.receiptVerification(check);
    }
    private static void fail() { throw failure(); }
    private static void fail(ReceiptCheck check) { throw failure(check); }

    private static final class Decoder {
        private final byte[] bytes; private int offset;
        private Decoder(byte[] bytes) { this.bytes = bytes; }
        int u8() { return fixed(1)[0] & 0xff; }
        int u16() { return integer(2).intValueExact(); }
        long u32() { return integer(4).longValueExact(); }
        int i32() { long value = u32(); return value > 0x7fff_ffffL ? (int) (value - 0x1_0000_0000L) : (int) value; }
        int position() { return offset; }
        int remaining() { return bytes.length - offset; }
        byte[] fixed(int length) {
            if (length < 0 || offset > bytes.length - length) fail();
            byte[] value = Arrays.copyOfRange(bytes, offset, offset + length); offset += length; return value;
        }
        byte[] bounded(int length) { if (u32() != length) fail(); return fixed(length); }
        byte[] boundedAtMost(int maximum) { long length = u32(); if (length > maximum) fail(); return fixed((int) length); }
        BigInteger integer(int length) { return new BigInteger(1, fixed(length)); }
        void finish() { if (offset != bytes.length) fail(); }
    }
    private static boolean selectedProtocolMatches(int actual, int[] selected) {
        if (selected.length > 1) return false;
        int version = selected.length == 0 ? CURRENT_PROTOCOL_VERSION : selected[0];
        return (version == 2 || version == 3) && actual == version;
    }
}
