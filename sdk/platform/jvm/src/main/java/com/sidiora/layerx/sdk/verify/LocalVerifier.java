package com.sidiora.layerx.sdk.verify;

import com.sidiora.layerx.sdk.PlatformSdkException;
import com.fasterxml.jackson.databind.JsonNode;
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
import org.bouncycastle.crypto.digests.KeccakDigest;

/** Local sequencer receipt verification. */
public final class LocalVerifier {
    private LocalVerifier() {}

    private static final byte[] MERKLE_LEAF_DOMAIN = ascii("LXP/v1/merkle-leaf\0");
    private static final byte[] MERKLE_INTERNAL_DOMAIN = ascii("LXP/v1/merkle-internal\0");
    private static final byte[] RECEIPT_DOMAIN = ascii("LXP/v1/receipt\0");
    private static final byte[] ED25519_X509_PREFIX = hex("302a300506032b6570032100");
    private static final int MAX_MESSAGE_BYTES = 1_048_576;
    private static final int MAX_EFFECTS = 512;
    private static final int MAX_EFFECT_BODY = 256;
    private static final long PROGRAM_OUTCOME_V1 = 0x50524731L;
    private static final long PROGRAM_OUTCOME_V2 = 0x50524732L;
    private static final long PROGRAM_OUTCOME_V3 = 0x50524733L;
    private static final long PROGRAM_OUTCOME_V4 = 0x50524734L;

    public record MerkleProof(long leafIndex, long leafCount, List<byte[]> siblings) {
        public MerkleProof { siblings = List.copyOf(siblings); }
    }
    public enum VerificationLevel {
        RECEIPTED("receipted"), COMMITTED("committed");
        private final String wire;
        VerificationLevel(String wire) { this.wire = wire; }
        public String wire() { return wire; }
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
    public record ReceiptVerification(VerificationLevel level, ProtocolReceipt receipt,
        byte[] canonicalBytes, byte[] receiptDigest) {}
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
                || receipt.parameterVersion() != 0
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
            if (module <= 11) fail(ReceiptCheck.DECODE);
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

    private static int proofDepth(long count) { int depth = 0; while (count > 1) { count = (count + 1) / 2; depth++; } return depth; }
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
}
