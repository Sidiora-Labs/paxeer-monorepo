package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.GeneratedReceiptContract;
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.time.Clock;
import java.time.Duration;
import java.security.KeyFactory;
import java.security.MessageDigest;
import java.security.Signature;
import java.security.spec.X509EncodedKeySpec;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionStage;

public final class ProgramsClient {
    public static final String PROGRAMS_ADDRESS = "0x000000000000000000000000000000000000101d";
    public static final String AUTH_ADDRESS = "0x000000000000000000000000000000000000101c";
    public static final BigInteger CHAIN_ID = BigInteger.valueOf(125);

    public record ProgramCall(ObjectNode transaction) {
        public ProgramCall {
            transaction = copy(transaction);
            if (!"0x2".equals(transaction.path("type").asText())
                    || !"0x7d".equals(transaction.path("chainId").asText())
                    || !PROGRAMS_ADDRESS.equals(transaction.path("to").asText())
                    || !"0x0".equals(transaction.path("value").asText())
                    || !transaction.path("accessList").isArray() || !transaction.path("accessList").isEmpty()) invalid();
            transaction.put("from", Programs.address(transaction.path("from").asText()));
            for (String name : List.of("nonce", "maxPriorityFeePerGas", "maxFeePerGas", "gas"))
                Programs.quantity(transaction.path(name));
            if (Programs.quantity(transaction.path("maxPriorityFeePerGas")).compareTo(
                    Programs.quantity(transaction.path("maxFeePerGas"))) > 0
                    || Programs.quantity(transaction.path("gas")).signum() == 0) invalid();
            Programs.data(transaction.path("data"));
        }
        @Override public ObjectNode transaction() { return transaction.deepCopy(); }
        public byte[] unsignedTransaction() { return Programs.encodeTransaction(transaction, true); }
        public byte[] signingPayload() { return Programs.encodeTransaction(transaction, false); }
    }

    public record ProgramSubmission(byte[] transactionHash, byte[] signedTransaction,
                                    LocalVerifier.SequencerReceiptVerification receipt) {
        public ProgramSubmission {
            transactionHash = exactArgument(transactionHash, 32);
            signedTransaction = signedTransaction.clone();
            if (!MessageDigest.isEqual(transactionHash, Programs.transactionHash(signedTransaction))
                    || receipt != null && !MessageDigest.isEqual(transactionHash, receipt.receipt().activityId())) invalidVerification();
        }
        @Override public byte[] transactionHash() { return transactionHash.clone(); }
        @Override public byte[] signedTransaction() { return signedTransaction.clone(); }
        public boolean pending() { return receipt == null; }
    }

    public record ProgramSimulation(int terminalResult, int terminalKind, byte[] response) {
        public ProgramSimulation {
            if (terminalKind < 1 || terminalKind > 3 || response == null || response.length > MAX_CALLDATA_BYTES) invalid();
            response = response.clone();
        }
        @Override public byte[] response() { return response.clone(); }
        public boolean committed() { return false; }
    }

    public static final class Programs {
        private final java.net.URI endpoint;
        private final java.net.http.HttpClient http;
        private final com.fasterxml.jackson.databind.ObjectMapper json = new com.fasterxml.jackson.databind.ObjectMapper();
        private final java.util.concurrent.atomic.AtomicLong ids = new java.util.concurrent.atomic.AtomicLong();

        public Programs(java.net.URI endpoint) {
            this.endpoint = Objects.requireNonNull(endpoint, "endpoint");
            if (!Set.of("http", "https").contains(endpoint.getScheme()) || endpoint.getHost() == null
                    || endpoint.getUserInfo() != null || endpoint.getFragment() != null) invalid();
            this.http = java.net.http.HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build();
        }

        public CompletionStage<JsonNode> request(String method, ArrayNode params) {
            long id = ids.incrementAndGet();
            ObjectNode body = object().put("jsonrpc", "2.0").put("id", id).put("method", method);
            body.set("params", params);
            var request = java.net.http.HttpRequest.newBuilder(endpoint).timeout(Duration.ofSeconds(30))
                .header("Content-Type", "application/json")
                .POST(java.net.http.HttpRequest.BodyPublishers.ofString(body.toString())).build();
            return http.sendAsync(request, java.net.http.HttpResponse.BodyHandlers.ofString()).thenApply(response -> {
                if (response.statusCode() != 200) throw new PlatformSdkException(PlatformSdkException.Code.TRANSPORT_FAILURE,
                    PlatformSdkException.Retry.SAFE, null, null, null);
                try {
                    JsonNode value = json.readTree(response.body());
                    if (value == null || !value.isObject() || !"2.0".equals(value.path("jsonrpc").asText())
                            || !value.path("id").isIntegralNumber() || value.path("id").longValue() != id
                            || value.has("error") == value.has("result")) throw decodeFailure();
                    if (value.has("error")) throw new PlatformSdkException(PlatformSdkException.Code.CAPABILITY_REFUSAL,
                        PlatformSdkException.Retry.NEVER, null, null, null);
                    return value.get("result");
                } catch (java.io.IOException error) { throw decodeFailure(); }
            });
        }

        static byte[] word(BigInteger value, int bits) {
            if (value == null || value.signum() < 0 || value.bitLength() > bits || bits > 256) invalid();
            byte[] output = new byte[32], bytes = value.toByteArray();
            int length = Math.min(bytes.length, 32);
            System.arraycopy(bytes, bytes.length - length, output, 32 - length, length);
            return output;
        }
        static byte[] abi(String selector, Object[] fields, int... dynamic) {
            Set<Integer> offsets = new java.util.HashSet<>();
            for (int index : dynamic) offsets.add(index);
            var head = new java.io.ByteArrayOutputStream();
            var tail = new java.io.ByteArrayOutputStream();
            for (int index = 0; index < fields.length; index++) {
                byte[] field = fields[index] instanceof String text ? text.getBytes(StandardCharsets.US_ASCII) : (byte[]) fields[index];
                if (offsets.contains(index)) {
                    head.writeBytes(word(BigInteger.valueOf(fields.length * 32L + tail.size()), 256));
                    tail.writeBytes(word(BigInteger.valueOf(field.length), 256));
                    tail.writeBytes(Arrays.copyOf(field, (field.length + 31) / 32 * 32));
                } else {
                    if (field.length != 32) invalid();
                    head.writeBytes(field);
                }
            }
            return concat(HexFormat.of().parseHex(selector), head.toByteArray(), tail.toByteArray());
        }
        static BigInteger number(byte[] data, int offset, int bits) {
            if (offset < 0 || offset > data.length - 32) throw decodeFailure();
            BigInteger number = new BigInteger(1, Arrays.copyOfRange(data, offset, offset + 32));
            if (number.bitLength() > bits) throw decodeFailure();
            return number;
        }
        static byte[] dynamic(byte[] data, int base, int index) {
            int relative = number(data, base + index * 32, 31).intValueExact();
            if (relative % 32 != 0 || relative < (index + 1) * 32) throw decodeFailure();
            int offset = Math.addExact(base, relative);
            int length = number(data, offset, 31).intValueExact();
            int start = Math.addExact(offset, 32);
            int padded = Math.toIntExact((length + 31L) / 32 * 32);
            if (start > data.length - padded || length > MAX_CALLDATA_BYTES) throw decodeFailure();
            for (int i = start + length; i < start + padded; i++) if (data[i] != 0) throw decodeFailure();
            return Arrays.copyOfRange(data, start, start + length);
        }
        static byte[] concat(byte[]... parts) {
            var out = new java.io.ByteArrayOutputStream();
            for (byte[] part : parts) out.writeBytes(part);
            return out.toByteArray();
        }
        static String selector(String signature) {
            return HexFormat.of().formatHex(Arrays.copyOf(transactionHash(signature.getBytes(StandardCharsets.US_ASCII)), 4));
        }
        public static byte[] transactionHash(byte[] signed) {
            var digest = new org.bouncycastle.crypto.digests.KeccakDigest(256);
            digest.update(signed, 0, signed.length);
            byte[] output = new byte[32]; digest.doFinal(output, 0); return output;
        }
        static String address(String value) {
            if (value == null || !value.matches("0x[0-9a-fA-F]{40}")) invalid();
            return value.toLowerCase(java.util.Locale.ROOT);
        }
        static byte[] data(JsonNode value) {
            if (value == null || !value.isTextual() || !value.textValue().matches("0x(?:[0-9a-fA-F]{2})*")) throw decodeFailure();
            return HexFormat.of().parseHex(value.textValue().substring(2));
        }
        static BigInteger quantity(JsonNode value) {
            if (value == null || !value.isTextual() || !value.textValue().matches("0x(?:0|[1-9a-f][0-9a-f]*)")) throw decodeFailure();
            BigInteger result = new BigInteger(value.textValue().substring(2), 16);
            if (result.bitLength() > 256) throw decodeFailure();
            return result;
        }
        static String quantity(BigInteger value) { word(value, 256); return "0x" + value.toString(16); }
        static String hexData(byte[] data) { return "0x" + hex(data); }
        private static byte[] magnitude(BigInteger value) {
            if (value.signum() == 0) return new byte[0];
            byte[] bytes = value.toByteArray();
            return bytes[0] == 0 ? Arrays.copyOfRange(bytes, 1, bytes.length) : bytes;
        }
        static byte[] rlp(byte[] value, boolean list) {
            if (!list && value.length == 1 && (value[0] & 255) < 128) return value.clone();
            int prefix = list ? 192 : 128;
            if (value.length <= 55) return concat(new byte[] {(byte) (prefix + value.length)}, value);
            byte[] length = magnitude(BigInteger.valueOf(value.length));
            return concat(new byte[] {(byte) (prefix + 55 + length.length)}, length, value);
        }
        static byte[] encodeTransaction(ObjectNode tx, boolean zeroSignature) {
            var fields = new java.io.ByteArrayOutputStream();
            for (String name : List.of("chainId", "nonce", "maxPriorityFeePerGas", "maxFeePerGas", "gas"))
                fields.writeBytes(rlp(magnitude(quantity(tx.path(name))), false));
            fields.writeBytes(rlp(HexFormat.of().parseHex(address(tx.path("to").asText()).substring(2)), false));
            fields.writeBytes(rlp(magnitude(quantity(tx.path("value"))), false));
            fields.writeBytes(rlp(data(tx.path("data")), false));
            fields.writeBytes(rlp(new byte[0], true));
            if (zeroSignature) for (int i = 0; i < 3; i++) fields.writeBytes(rlp(new byte[0], false));
            return concat(new byte[] {2}, rlp(fields.toByteArray(), true));
        }
        private record Rlp(byte[] encoded, byte[] value, boolean list) {}
        private static Rlp readRlp(ByteBuffer input) {
            int start = input.position();
            if (!input.hasRemaining()) throw decodeFailure();
            int prefix = input.get() & 255;
            if (prefix < 128) return new Rlp(new byte[] {(byte) prefix}, new byte[] {(byte) prefix}, false);
            boolean list = prefix >= 192;
            int shortBase = list ? 192 : 128, longBase = shortBase + 55;
            int length = prefix - shortBase;
            if (prefix > longBase) {
                int lengthBytes = prefix - longBase;
                if (lengthBytes > 4 || input.remaining() < lengthBytes || input.get(input.position()) == 0) throw decodeFailure();
                long size = 0;
                for (int i = 0; i < lengthBytes; i++) size = (size << 8) | (input.get() & 255);
                if (size < 56 || size > MAX_CALLDATA_BYTES + 65536L) throw decodeFailure();
                length = (int) size;
            }
            if (length > input.remaining()) throw decodeFailure();
            byte[] bytes = new byte[length]; input.get(bytes);
            if (!list && length == 1 && (bytes[0] & 255) < 128) throw decodeFailure();
            return new Rlp(Arrays.copyOfRange(input.array(), start, input.position()), bytes, list);
        }
        static List<Rlp> transactionFields(byte[] transaction) {
            if (transaction == null || transaction.length < 2 || transaction.length > MAX_CALLDATA_BYTES + 65536 || transaction[0] != 2) invalid();
            ByteBuffer input = ByteBuffer.wrap(transaction); input.get();
            Rlp root = readRlp(input);
            if (!root.list() || input.hasRemaining()) throw decodeFailure();
            ByteBuffer fields = ByteBuffer.wrap(root.value());
            List<Rlp> result = new ArrayList<>();
            while (fields.hasRemaining()) result.add(readRlp(fields));
            if (result.size() != 12) throw decodeFailure();
            for (int i = 0; i < 12; i++) if (result.get(i).list() != (i == 8)) throw decodeFailure();
            for (int i : new int[] {0, 1, 2, 3, 4, 6, 9, 10, 11}) {
                byte[] value = result.get(i).value();
                if (value.length > 32 || value.length > 0 && value[0] == 0) throw decodeFailure();
            }
            if (!new BigInteger(1, result.get(0).value()).equals(CHAIN_ID)
                    || !Arrays.equals(result.get(5).value(), HexFormat.of().parseHex(PROGRAMS_ADDRESS.substring(2)))
                    || result.get(6).value().length != 0 || result.get(8).value().length != 0
                    || result.get(1).value().length > 8 || result.get(4).value().length > 8
                    || new BigInteger(1, result.get(4).value()).signum() == 0
                    || new BigInteger(1, result.get(2).value()).compareTo(new BigInteger(1, result.get(3).value())) > 0
                    || new BigInteger(1, result.get(9).value()).compareTo(BigInteger.ONE) > 0
                    || result.get(10).value().length == 0 || result.get(11).value().length == 0) invalid();
            BigInteger order = new BigInteger("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16);
            if (new BigInteger(1, result.get(10).value()).compareTo(order) >= 0
                    || new BigInteger(1, result.get(11).value()).compareTo(order.shiftRight(1)) > 0) invalid();
            return result;
        }
        static byte[] transactionData(byte[] signedTransaction) { return transactionFields(signedTransaction).get(7).value().clone(); }
        static void bind(ProgramCall expected, byte[] signedTransaction) {
            List<Rlp> fields = transactionFields(signedTransaction);
            var first = new java.io.ByteArrayOutputStream();
            for (int i = 0; i < 9; i++) first.writeBytes(fields.get(i).encoded());
            if (!Arrays.equals(expected.signingPayload(), concat(new byte[] {2}, rlp(first.toByteArray(), true)))) invalidVerification();
            var curve = org.bouncycastle.crypto.ec.CustomNamedCurves.getByName("secp256k1");
            BigInteger r = new BigInteger(1, fields.get(10).value()), sigS = new BigInteger(1, fields.get(11).value());
            byte[] compressed = concat(new byte[] {(byte) (2 + new BigInteger(1, fields.get(9).value()).intValue())}, word(r, 256));
            var point = curve.getCurve().decodePoint(compressed);
            if (!point.multiply(curve.getN()).isInfinity()) invalidVerification();
            BigInteger e = new BigInteger(1, transactionHash(expected.signingPayload()));
            var publicPoint = point.multiply(sigS).subtract(curve.getG().multiply(e.mod(curve.getN()))).multiply(r.modInverse(curve.getN())).normalize();
            byte[] publicKey = publicPoint.getEncoded(false);
            byte[] sender = Arrays.copyOfRange(transactionHash(Arrays.copyOfRange(publicKey, 1, publicKey.length)), 12, 32);
            if (!expected.transaction().path("from").asText().equals(hexData(sender))) invalidVerification();
        }
    }

    public static final int MAX_CALLDATA_BYTES = 1_048_576;
    public static final int MAX_CAPABILITIES = 5;
    public static final int PROGRAMS_RECEIPT_MODULE_ID = 9;
    public static final int CALL_OPERATION = 3;
    private static final int MAX_LEGACY_VALUES = 512;
    private static final int MAX_INTERFACE_BYTES = 952;
    private static final String SEQUENCER_SIGNED = "sequencer-signed";
    private static final String EXECUTION_VERIFICATION = "receipt-terminal-and-call-graph-verified";
    private static final BigInteger MAX_U32 = BigInteger.ONE.shiftLeft(32).subtract(BigInteger.ONE);
    private static final BigInteger MAX_U64 = BigInteger.ONE.shiftLeft(64).subtract(BigInteger.ONE);
    private static final BigInteger MAX_U128 = BigInteger.ONE.shiftLeft(128).subtract(BigInteger.ONE);
    private static final byte[] ED25519_X509_PREFIX = HexFormat.of().parseHex("302a300506032b6570032100");
    private static final byte[] TRANSFER_SET_V1_DOMAIN =
        "LayerX/programs/402LXP/transfer-set/v1\0".getBytes(StandardCharsets.UTF_8);
    private static final byte[] TRANSFER_SET_V2_DOMAIN =
        "LayerX/programs/402LXP/transfer-set/v2\0".getBytes(StandardCharsets.UTF_8);
    private static final byte[] PROGRAM_AUTHORITY_DOMAIN =
        "LayerX/programs/402LXP/program-authority/v1\0".getBytes(StandardCharsets.UTF_8);
    private static final byte[] PROGRAM_FUNDING_DOMAIN =
        "LayerX/programs/402LXP/program-funding/v1\0".getBytes(StandardCharsets.UTF_8);
    private static final byte[] PROGRAM_ACCOUNT_DOMAIN =
        "LayerX/programs/program-account/v1\0".getBytes(StandardCharsets.UTF_8);
    private static final byte[] EVENT_ENVELOPE_DOMAIN =
        "LayerX/programs/events/v1\0".getBytes(StandardCharsets.UTF_8);
    private static final byte[] OCCUPANCY_V1_DOMAIN =
        "LXP/storage-occupancy-settlement/v1\0".getBytes(StandardCharsets.UTF_8);
    private static final byte[] OCCUPANCY_V2_DOMAIN =
        "LXP/storage-occupancy-settlement/v2\0".getBytes(StandardCharsets.UTF_8);
    private static final byte[] OCCUPANCY_V3_DOMAIN =
        "LXP/storage-occupancy-settlement/v3\0".getBytes(StandardCharsets.UTF_8);
    private static final byte[] OCCUPANCY_MANDATE_DOMAIN =
        "LXP/storage-occupancy-mandate/v1\0".getBytes(StandardCharsets.UTF_8);
    private static final byte[] MERKLE_LEAF_DOMAIN =
        "LXP/v1/merkle-leaf\0".getBytes(StandardCharsets.UTF_8);
    private static final byte[] MERKLE_INTERNAL_DOMAIN =
        "LXP/v1/merkle-internal\0".getBytes(StandardCharsets.UTF_8);
    private static final byte[] ACCOUNT_DERIVATION_DOMAIN =
        "LX:ACCOUNT:v1".getBytes(StandardCharsets.UTF_8);
    private static final byte[] FEE_TREASURY_LABEL = "system:fees".getBytes(StandardCharsets.UTF_8);
    public enum Capability {
        STORAGE_READ("storage_read"), STORAGE_WRITE("storage_write"), TRANSFER("transfer"),
        EMIT_EVENT("emit_event"), COMPOSE("compose");
        private final String wire;
        Capability(String wire) { this.wire = wire; }
        public String wire() { return wire; }
    }

    public enum SubmissionState {
        REFUSED("refused"), UNKNOWN("unknown"), EXECUTED("executed");
        private final String wire;
        SubmissionState(String wire) { this.wire = wire; }
        public String wire() { return wire; }
        private static SubmissionState parse(String value) {
            for (SubmissionState state : values()) if (state.wire.equals(value)) return state;
            throw decodeFailure();
        }
    }

    public record Budget(BigInteger fuel, BigInteger feeLimit) {
        public Budget {
            if (fuel == null || fuel.signum() <= 0 || fuel.bitLength() > 64
                    || feeLimit == null || feeLimit.signum() < 0 || feeLimit.bitLength() > 128) invalid();
        }
    }

    public record Call(byte[] programId, byte[] calldata, Budget budget,
                       List<Capability> capabilities, byte[] signedTransaction, NativeProgramCall nativeCall) {
        public Call(byte[] programId, byte[] calldata, Budget budget, List<Capability> capabilities, byte[] signedTransaction) {
            this(programId, calldata, budget, capabilities, signedTransaction, null);
        }
        public Call(NativeProgramCall nativeCall, BigInteger feeLimit, byte[] signedTransaction) {
            this(nativeCall.programId(), nativeCall.calldata(), new Budget(new BigInteger(Long.toUnsignedString(nativeCall.resources()[0])), feeLimit), List.of(), signedTransaction, nativeCall);
        }
        public Call {
            programId = exactArgument(programId, 32);
            if (calldata == null || calldata.length > MAX_CALLDATA_BYTES || budget == null
                    || signedTransaction == null || signedTransaction.length == 0
                    || signedTransaction.length > MAX_CALLDATA_BYTES || capabilities == null
                    || capabilities.size() > MAX_CAPABILITIES || allZero(programId)) invalid();
            capabilities = List.copyOf(capabilities);
            if (capabilities.stream().anyMatch(Objects::isNull)) invalid();
            for (int index = 1; index < capabilities.size(); index++) {
                if (capabilities.get(index - 1).ordinal() >= capabilities.get(index).ordinal()) invalid();
            }
            calldata = calldata.clone();
            signedTransaction = signedTransaction.clone();
        }
        @Override public byte[] programId() { return programId.clone(); }
        @Override public byte[] calldata() { return calldata.clone(); }
        @Override public byte[] signedTransaction() { return signedTransaction.clone(); }
        public byte[] signedActivity() { return signedTransaction(); }
        public byte[] transactionHash() {
            Programs.transactionFields(signedTransaction);
            return Programs.transactionHash(signedTransaction);
        }
    }

    public enum Lifecycle { ACTIVE, DEPRECATED, TOMBSTONED }

    public record Source(String status, byte[] sourceDigest, byte[] environmentDigest, String pipeline,
                         byte[] expectedCodeHash, byte[] reproducedArtifactDigest) {
        public Source {
            status = Objects.requireNonNull(status, "status");
            sourceDigest = cloneNullable(sourceDigest); environmentDigest = cloneNullable(environmentDigest);
            expectedCodeHash = cloneNullable(expectedCodeHash); reproducedArtifactDigest = cloneNullable(reproducedArtifactDigest);
        }
        @Override public byte[] sourceDigest() { return cloneNullable(sourceDigest); }
        @Override public byte[] environmentDigest() { return cloneNullable(environmentDigest); }
        @Override public byte[] expectedCodeHash() { return cloneNullable(expectedCodeHash); }
        @Override public byte[] reproducedArtifactDigest() { return cloneNullable(reproducedArtifactDigest); }
    }

    public record Discovery(byte[] programId, Lifecycle lifecycle, long version, byte[] codeHash,
                            int abiVersion, byte[] receiptDigest, byte[] stateRoot,
                            BigInteger observedSequence, BigInteger observedAt, BigInteger validThrough,
                            String verification) {
        public Discovery {
            programId = programId.clone(); codeHash = codeHash.clone(); receiptDigest = cloneNullable(receiptDigest); stateRoot = cloneNullable(stateRoot);
        }
        @Override public byte[] programId() { return programId.clone(); }
        @Override public byte[] codeHash() { return cloneNullable(codeHash); }
        @Override public byte[] receiptDigest() { return cloneNullable(receiptDigest); }
        @Override public byte[] stateRoot() { return cloneNullable(stateRoot); }
    }

    public record Interface(byte[] programId, long version, byte[] codeHash, int abiVersion,
                            byte[] interfaceBytes, byte[] interfaceDigest, byte[] receiptDigest,
                            byte[] stateRoot, BigInteger observedSequence, BigInteger observedAt,
                            BigInteger validThrough, Source source, String verification) {
        public Interface {
            programId = programId.clone(); codeHash = cloneNullable(codeHash); interfaceBytes = interfaceBytes.clone();
            interfaceDigest = interfaceDigest.clone(); receiptDigest = cloneNullable(receiptDigest); stateRoot = cloneNullable(stateRoot);
        }
        @Override public byte[] programId() { return programId.clone(); }
        @Override public byte[] codeHash() { return cloneNullable(codeHash); }
        @Override public byte[] interfaceBytes() { return interfaceBytes.clone(); }
        @Override public byte[] interfaceDigest() { return interfaceDigest.clone(); }
        @Override public byte[] receiptDigest() { return cloneNullable(receiptDigest); }
        @Override public byte[] stateRoot() { return cloneNullable(stateRoot); }
    }

    public record VerifiedExecution(ObjectNode document, LocalVerifier.ReceiptVerification receipt,
                                    byte[] terminalPayload, byte[] callGraph, String transferVerification) {
        public VerifiedExecution {
            document = copy(document);
            Objects.requireNonNull(receipt, "receipt");
            terminalPayload = terminalPayload.clone();
            callGraph = callGraph.clone();
        }
        @Override public byte[] terminalPayload() { return terminalPayload.clone(); }
        @Override public byte[] callGraph() { return callGraph.clone(); }
        public int guestAbiVersion() { return requiredU16(document.get("guest_abi_version")); }
        public byte[] terminalPayloadRoot() {
            return receipt.receipt().programOutcome().terminalPayloadRoot().clone();
        }
        public byte[] callGraphRoot() {
            return receipt.receipt().programOutcome().callGraphRoot().clone();
        }
    }

    public record Simulation(ObjectNode value, VerifiedExecution execution) {
        public Simulation {
            value = copy(value);
            Objects.requireNonNull(execution, "execution");
        }
        public boolean committed() { return false; }
    }

    public record Submission(SubmissionState state, byte[] transactionHash, String idempotencyKey,
                             byte[] retainedSignedTransaction, LocalVerifier.SequencerReceiptVerification execution, ObjectNode value) {
        public Submission {
            Objects.requireNonNull(state, "state");
            transactionHash = exactVerification(transactionHash, 32);
            if (!canonicalLowerHex(idempotencyKey, 32)) invalidVerification();
            retainedSignedTransaction = retainedSignedTransaction == null ? null : retainedSignedTransaction.clone();
            if (state == SubmissionState.UNKNOWN && execution != null
                    || state != SubmissionState.UNKNOWN && execution == null) invalidVerification();
            value = copy(value);
        }
        @Override public byte[] transactionHash() { return transactionHash.clone(); }
        @Override public byte[] retainedSignedTransaction() { return cloneNullable(retainedSignedTransaction); }
        public byte[] activityId() { return transactionHash(); }
        public byte[] retainedSignedActivity() { return retainedSignedTransaction(); }
        public boolean unknown() { return state == SubmissionState.UNKNOWN; }
    }

    public CompletionStage<LocalVerifier.ReceiptVerification> lifecycleReceipt(NativeProgramLifecycleRequest request) {
        return sequencerReceipt(request.signedActivity()).thenApply(ProgramsClient::legacyReceiptView);
    }

    public CompletionStage<ProgramCall> deploy(NativeProgramDeploy operation) {
        return prepare(NativeProgramLifecycleRequest.calldata(operation, BigInteger.ZERO));
    }
    public CompletionStage<ProgramCall> upgrade(NativeProgramUpgrade operation, BigInteger feeLimit) {
        return prepare(NativeProgramLifecycleRequest.calldata(operation, feeLimit));
    }
    public CompletionStage<ProgramCall> windDown(NativeProgramWindDown operation) {
        return prepare(NativeProgramLifecycleRequest.calldata(operation, BigInteger.ZERO));
    }
    public CompletionStage<LocalVerifier.ReceiptVerification> deploy(NativeProgramDeploy operation, byte[] signedTransaction, IdempotencyKey key) {
        return lifecycle(operation, signedTransaction, key);
    }
    public CompletionStage<LocalVerifier.ReceiptVerification> upgrade(NativeProgramUpgrade operation, byte[] signedTransaction, IdempotencyKey key) {
        return lifecycle(operation, signedTransaction, key);
    }
    public CompletionStage<LocalVerifier.ReceiptVerification> windDown(NativeProgramWindDown operation, byte[] signedTransaction, IdempotencyKey key) {
        return lifecycle(operation, signedTransaction, key);
    }
    private CompletionStage<LocalVerifier.ReceiptVerification> lifecycle(NativeProgramLifecycle operation, byte[] signedTransaction, IdempotencyKey key) {
        Objects.requireNonNull(key, "key");
        new NativeProgramLifecycleRequest(operation, signedTransaction, key);
        return broadcast(signedTransaction).thenCompose(submission -> sequencerReceipt(submission.signedTransaction()))
            .thenApply(ProgramsClient::legacyReceiptView);
    }
    private static LocalVerifier.ReceiptVerification legacyReceiptView(LocalVerifier.SequencerReceiptVerification receipt) {
        return new LocalVerifier.ReceiptVerification(receipt.level(), receipt.receipt(), receipt.canonicalBytes(), receipt.receiptDigest());
    }

    private final ProductionClient client;
    private Programs rpc;
    private String sender;
    private BigInteger priorityFee;
    private BigInteger maximumFee;
    private LocalVerifier.ReceiptKeys receiptKeys;
    private byte[] didId;
    private final Map<String, byte[]> submittedTransactions = new java.util.concurrent.ConcurrentHashMap<>();
    private final Map<String, String> submittedNonces = new java.util.concurrent.ConcurrentHashMap<>();

    public ProgramsClient(Programs rpc, String sender, BigInteger priorityFee, BigInteger maximumFee,
                          LocalVerifier.ReceiptKeys receiptKeys, byte[] didId) {
        this.client = null;
        this.rpc = Objects.requireNonNull(rpc, "rpc");
        this.sender = Programs.address(sender);
        Programs.word(priorityFee, 256); Programs.word(maximumFee, 256);
        if (priorityFee.compareTo(maximumFee) > 0) invalid();
        this.priorityFee = priorityFee; this.maximumFee = maximumFee;
        this.receiptKeys = Objects.requireNonNull(receiptKeys, "receiptKeys");
        this.didId = exactArgument(didId, 32);
        this.sequencerPublicKey = receiptKeys.entries().getFirst().publicKey();
        this.protocolVersion = 3; this.clock = Clock.systemUTC();
        this.maximumSimulationAgeMillis = BigInteger.valueOf(300_000);
    }

    private CompletionStage<JsonNode> rpc(String method, JsonNode... parameters) {
        if (rpc == null) throw new PlatformSdkException(PlatformSdkException.Code.UNAVAILABLE_CAPABILITY,
            PlatformSdkException.Retry.NEVER, null, null, null);
        ArrayNode params = JsonNodeFactory.instance.arrayNode();
        for (JsonNode parameter : parameters) params.add(parameter);
        return rpc.request(method, params);
    }
    private static JsonNode string(String value) { return JsonNodeFactory.instance.textNode(value); }
    private CompletionStage<byte[]> ethCall(String address, byte[] calldata) {
        ObjectNode tx = object().put("to", address).put("data", Programs.hexData(calldata));
        if (sender != null) tx.put("from", sender);
        return rpc("eth_call", tx, string("pending")).thenApply(Programs::data);
    }
    private CompletionStage<ProgramCall> prepare(byte[] calldata) {
        return rpc("eth_getTransactionCount", string(sender), string("pending")).thenCompose(nonce -> {
            ObjectNode tx = object().put("type", "0x2").put("chainId", "0x7d").put("from", sender)
                .put("to", PROGRAMS_ADDRESS).put("nonce", Programs.quantity(Programs.quantity(nonce)))
                .put("maxPriorityFeePerGas", Programs.quantity(priorityFee)).put("maxFeePerGas", Programs.quantity(maximumFee))
                .put("value", "0x0").put("data", Programs.hexData(calldata));
            tx.putArray("accessList");
            return rpc("eth_estimateGas", tx.deepCopy(), string("pending")).thenApply(gas -> {
                tx.put("gas", Programs.quantity(Programs.quantity(gas)));
                return new ProgramCall(tx);
            });
        });
    }
    public CompletionStage<ProgramCall> call(NativeProgramCall call, BigInteger feeLimit) {
        return prepare(call.calldata(feeLimit));
    }
    public CompletionStage<ProgramCall> call(Call call) {
        return call(nativeCall(call), call.budget().feeLimit());
    }
    private static NativeProgramCall nativeCall(Call call) {
        Objects.requireNonNull(call, "call");
        NativeProgramCall nativeCall = call.nativeCall() == null
            ? NativeProgramCall.decodeSignedActivity(call.signedActivity()) : call.nativeCall();
        if (!Arrays.equals(nativeCall.programId(), call.programId()) || !Arrays.equals(nativeCall.calldata(), call.calldata())
                || !new BigInteger(Long.toUnsignedString(nativeCall.resources()[0])).equals(call.budget().fuel())) invalid();
        return nativeCall;
    }
    public CompletionStage<ProgramSubmission> submit(ProgramCall prepared, byte[] signedTransaction) {
        Programs.bind(prepared, signedTransaction);
        return broadcast(signedTransaction);
    }
    private CompletionStage<ProgramSubmission> broadcast(byte[] signedTransaction) {
        byte[] raw = signedTransaction.clone();
        List<Programs.Rlp> fields = Programs.transactionFields(raw);
        ObjectNode tx = object().put("type", "0x2").put("chainId", "0x7d").put("from", sender)
            .put("to", PROGRAMS_ADDRESS).put("value", "0x0").put("data", Programs.hexData(fields.get(7).value()));
        String[] quantities = {"chainId", "nonce", "maxPriorityFeePerGas", "maxFeePerGas", "gas"};
        for (int i = 0; i < quantities.length; i++) tx.put(quantities[i], Programs.quantity(new BigInteger(1, fields.get(i).value())));
        tx.putArray("accessList");
        Programs.bind(new ProgramCall(tx), raw);
        byte[] hash = Programs.transactionHash(raw);
        String nonce = tx.path("nonce").asText();
        String prior = submittedNonces.putIfAbsent(nonce, hex(hash));
        if (prior != null && !prior.equals(hex(hash))) invalid();
        submittedTransactions.put(hex(hash), raw.clone());
        return rpc("eth_sendRawTransaction", string(Programs.hexData(raw))).thenApply(result -> {
            if (!Arrays.equals(Programs.data(result), hash)) throw unknownOutcome();
            return new ProgramSubmission(hash, raw, null);
        });
    }
    public CompletionStage<ProgramSubmission> receipt(ProgramSubmission submission) {
        return sequencerReceipt(submission.signedTransaction()).thenApply(receipt ->
            new ProgramSubmission(submission.transactionHash(), submission.signedTransaction(), receipt));
    }
    public CompletionStage<LocalVerifier.SequencerReceiptVerification> sequencerReceipt(byte[] signedTransaction) {
        byte[] raw = signedTransaction.clone();
        byte[] hash = Programs.transactionHash(raw);
        return rpc("pax_sequencerReceipt", string(Programs.hexData(hash))).thenApply(view -> {
            if (!view.isObject() || !Arrays.equals(Programs.data(view.path("txHash")), hash)) invalidVerification();
            String rung = view.path("rung").asText();
            if ("submitted".equals(rung)) throw unknownOutcome();
            if (!Set.of("receipted", "committed").contains(rung)) invalidVerification();
            return verifyReceipt(Programs.data(view.path("receipt")), raw, receiptKeys, view);
        });
    }
    public static LocalVerifier.SequencerReceiptVerification verifyReceipt(byte[] receipt, byte[] signedTransaction,
            LocalVerifier.ReceiptKeys keys, JsonNode view) {
        var verified = LocalVerifier.verifySequencerReceipt(receipt, signedTransaction, keys);
        return LocalVerifier.verifySequencerReceipt(receipt, signedTransaction, keys, view, verified.receipt().previousStateRoot());
    }
    private final byte[] sequencerPublicKey;
    private final int protocolVersion;
    private final Clock clock;
    private final BigInteger maximumSimulationAgeMillis;

    public ProgramsClient(ProductionClient client, byte[] sequencerPublicKey) {
        this(client, sequencerPublicKey, Clock.systemUTC(), Duration.ofMinutes(5));
    }

    public ProgramsClient(ProductionClient client, byte[] sequencerPublicKey, Clock clock,
                          Duration maximumSimulationAge, int... selectedProtocol) {
        if (selectedProtocol.length > 1) invalid();
        this.protocolVersion = selectedProtocol.length == 0 ? 2 : selectedProtocol[0];
        if (protocolVersion != 2 && protocolVersion != 3) invalid();
        this.client = Objects.requireNonNull(client, "client");
        this.sequencerPublicKey = exactArgument(sequencerPublicKey, 32);
        if (allZero(this.sequencerPublicKey)) invalid();
        this.clock = Objects.requireNonNull(clock, "clock");
        Objects.requireNonNull(maximumSimulationAge, "maximumSimulationAge");
        if (maximumSimulationAge.isZero() || maximumSimulationAge.isNegative()) invalid();
        this.maximumSimulationAgeMillis = durationMillis(maximumSimulationAge);
    }

    public CompletionStage<Discovery> discover(byte[] programId) { return discover(programId, "pending"); }
    public CompletionStage<Discovery> discover(byte[] programId, String verificationLevel) {
        if (!"pending".equals(verificationLevel)) invalid();
        byte[] id = exactArgument(programId, 32);
        return ethCall(PROGRAMS_ADDRESS, Programs.abi("b1ffb3d4", new Object[] {id})).thenCompose(data -> {
            if (data.length != 192 || Programs.number(data, 0, 8).intValue() > 1) throw decodeFailure();
            int active = Programs.number(data, 160, 1).intValue();
            CompletionStage<Lifecycle> lifecycle = active == 1
                ? CompletableFuture.completedFuture(Lifecycle.ACTIVE)
                : ethCall(PROGRAMS_ADDRESS, Programs.abi("98e4e41d", new Object[] {id})).thenApply(windDown -> {
                    if (windDown.length != 192) throw decodeFailure();
                    return switch (Programs.number(windDown, 0, 8).intValue()) {
                        case 2 -> Lifecycle.DEPRECATED;
                        case 3 -> Lifecycle.TOMBSTONED;
                        default -> throw decodeFailure();
                    };
                });
            return lifecycle.thenApply(value -> new Discovery(id, value,
                Programs.number(data, 128, 32).longValueExact(), Arrays.copyOfRange(data, 64, 96),
                Programs.number(data, 96, 16).intValueExact(), null, null, null, null, null, "pending"));
        });
    }
    public CompletionStage<Interface> interfaceAt(byte[] programId) { return interfaceAt(programId, "pending"); }
    public CompletionStage<Interface> interfaceAt(byte[] programId, String verificationLevel) {
        if (!"pending".equals(verificationLevel)) invalid();
        byte[] id = exactArgument(programId, 32);
        return ethCall(PROGRAMS_ADDRESS, Programs.abi("8efc7240", new Object[] {id})).thenApply(data -> {
            long version = Programs.number(data, 0, 32).longValueExact();
            byte[] digest = Arrays.copyOfRange(data, 32, 64), encoding = Programs.dynamic(data, 0, 2);
            if (!Arrays.equals(sha256(encoding), digest)) invalidVerification();
            return new Interface(id, version, null, 0, encoding, digest, null, null, null, null, null, null, "pending");
        }).thenCompose(iface -> discover(id).thenApply(program -> {
            if (iface.version() != program.version()) invalidVerification();
            return new Interface(id, iface.version(), program.codeHash(), program.abiVersion(), iface.interfaceBytes(),
                iface.interfaceDigest(), null, null, null, null, null, null, "pending");
        }));
    }
    public CompletionStage<ProgramSimulation> simulate(NativeProgramCall call, BigInteger feeLimit) {
        return ethCall(PROGRAMS_ADDRESS, call.simulateCalldata(feeLimit)).thenApply(data -> {
            if (data.length < 128) throw decodeFailure();
            BigInteger signed = new BigInteger(Arrays.copyOfRange(data, 0, 32));
            int result = signed.intValueExact();
            int kind = Programs.number(data, 32, 8).intValueExact();
            if (kind < 1 || kind > 3) throw decodeFailure();
            byte[] response = Programs.dynamic(data, 0, 2);
            if (!Arrays.equals(data, Programs.abi("", new Object[] {
                    fixedUnsigned(signed.signum() < 0 ? signed.add(BigInteger.ONE.shiftLeft(256)) : signed, 32),
                    Programs.word(BigInteger.valueOf(kind), 8), response}, 2))) throw decodeFailure();
            return new ProgramSimulation(result, kind, response);
        });
    }
    public CompletionStage<ProgramSimulation> simulate(Call call) {
        return simulate(nativeCall(call), call.budget().feeLimit());
    }
    public CompletionStage<Submission> submit(Call call, IdempotencyKey key) {
        if (key == null || !canonicalLowerHex(key.value(), 32)) invalid();
        NativeProgramCall nativeCall = nativeCall(call);
        byte[] signed = call.signedActivity();
        if (!Arrays.equals(Programs.transactionData(signed), nativeCall.calldata(call.budget().feeLimit()))) invalid();
        return broadcast(signed).thenApply(result -> pendingSubmission(result.transactionHash(), key.value(), signed, object()));
    }
    private static Submission pendingSubmission(byte[] hash, String key, byte[] raw, ObjectNode value) {
        return new Submission(SubmissionState.UNKNOWN, hash, key, raw, null, value);
    }
    public CompletionStage<Submission> receipt(Call call, IdempotencyKey key) {
        byte[] signed = call.signedActivity();
        nativeCall(call).bindSignedActivity(signed);
        submittedTransactions.put(hex(Programs.transactionHash(signed)), signed.clone());
        return receipt(key, Programs.transactionHash(signed));
    }
    public CompletionStage<Submission> receipt(IdempotencyKey key, byte[] expectedTransactionHash) {
        return receipt(key, expectedTransactionHash, "receipted");
    }
    public CompletionStage<Submission> receipt(IdempotencyKey key, byte[] expectedTransactionHash, String verificationLevel) {
        if (key == null || !canonicalLowerHex(key.value(), 32) || !"receipted".equals(verificationLevel)) invalid();
        byte[] hash = exactArgument(expectedTransactionHash, 32);
        byte[] raw = submittedTransactions.get(hex(hash));
        if (raw == null) throw new PlatformSdkException(PlatformSdkException.Code.INVALID_ARGUMENT,
            PlatformSdkException.Retry.NEVER, null, null, null);
        return receiptByIdempotency(key, raw).thenApply(verified -> {
            ObjectNode value = object().put("transaction_hash", Programs.hexData(hash)).put("verification", "receipted");
            return new Submission(verified.status() == 1 ? SubmissionState.EXECUTED : SubmissionState.REFUSED,
                hash, key.value(), raw, verified, value);
        });
    }
    public CompletionStage<LocalVerifier.SequencerReceiptVerification> receiptByIdempotency(IdempotencyKey key, byte[] signedTransaction) {
        if (key == null || !canonicalLowerHex(key.value(), 32)) invalid();
        byte[] raw = signedTransaction.clone();
        return ethCall(AUTH_ADDRESS, Programs.abi(Programs.selector("resultByIdempotency(bytes32,bytes32)"),
            new Object[] {didId, hex32(key.value())})).thenApply(data -> {
                byte[] receipt = Programs.dynamic(data, 0, 0);
                if (!Arrays.equals(data, Programs.abi("", new Object[] {receipt}, 0))) throw decodeFailure();
                return LocalVerifier.verifySequencerReceipt(receipt, raw, receiptKeys);
            });
    }
    public CompletionStage<JsonNode> activity(byte[] transactionHash) { return activity(transactionHash, "pending"); }
    public CompletionStage<JsonNode> activity(byte[] transactionHash, String verificationLevel) {
        if (!"pending".equals(verificationLevel)) invalid();
        byte[] hash = exactArgument(transactionHash, 32);
        return rpc("eth_getTransactionByHash", string(Programs.hexData(hash))).thenApply(value -> {
            if (!value.isNull() && (!value.isObject() || !Arrays.equals(Programs.data(value.path("hash")), hash))) invalidVerification();
            return value.deepCopy();
        });
    }

    private static byte[] cloneNullable(byte[] value) { return value == null ? null : value.clone(); }

    private record TerminalUsage(BigInteger cpu, BigInteger memory, BigInteger read, BigInteger write,
                                 long values, BigInteger outputBytes, BigInteger fee) {}

    static record TerminalAttachments(byte[] inner, byte[] occupancy, byte[] authorization,
                                      byte[] transferRoot) {}

    private record ProgramAuthorityBinding(byte[] owner, byte[] frame, byte[] source,
                                           byte[] asset, byte[] destination, BigInteger amount) {}

    private record ProgramFundingBinding(byte[] owner, byte[] destination, byte[] asset) {}


    private record OccupancyCharge(byte[] payer, BigInteger amountDue, boolean paid,
                                   BigInteger arrearsAfter) {}

    private record OccupancySettlement(BigInteger byteBatches, BigInteger feeUnits,
                                       List<OccupancyCharge> charges) {}

    private record StorageNamespace(byte[] canonical, byte[] wire, byte[] program,
                                    byte[] principal) {}

    private record PayerAggregate(byte[] payer, BigInteger due, BigInteger paid,
                                  BigInteger arrears) {}

    public static String verifyTerminalV5(byte[] encoded, byte[] availableGraph, byte[] expectedProgram,
            ObjectNode documentOutcome, LocalVerifier.ProgramReceiptOutcome receipt) {
        if (documentOutcome == null) invalidVerification();
        return verifyTerminalV5Internal(encoded, availableGraph, expectedProgram, documentOutcome, receipt, false);
    }

    public static String verifyTerminalV5Capture(byte[] encoded, byte[] availableGraph, byte[] expectedProgram,
            LocalVerifier.ProgramReceiptOutcome receipt) {
        return verifyTerminalV5Internal(encoded, availableGraph, expectedProgram, null, receipt, true);
    }

    private static String verifyTerminalV5Internal(byte[] encoded, byte[] availableGraph, byte[] expectedProgram,
            ObjectNode documentOutcome, LocalVerifier.ProgramReceiptOutcome receipt, boolean capturedOutcome) {
        try {
            if (receipt.abiVersion() != GeneratedReceiptContract.PROGRAM_ABI_V3
                    && receipt.abiVersion() != GeneratedReceiptContract.PROGRAM_ABI_V4) invalidVerification();
            byte[] inner = unwrapTerminal(unwrapAppliedTerminal(encoded, receipt)).inner();
            boolean refusal = receipt.terminalKind() != 1
                && (starts(inner, "LXP/programs/callback-failure/v1\0")
                    || starts(inner, "LXP/programs/settlement-failure/v1\0"));
            if (!starts(inner, "LXP/program-execution/v5\0") && !refusal) invalidVerification();
            return verifyTerminalInternal(encoded, availableGraph, expectedProgram, documentOutcome, 3, receipt, capturedOutcome);
        } catch (IllegalArgumentException error) { invalidVerification(); throw new AssertionError(); }
    }

    static String verifyTerminal(byte[] encoded, byte[] availableGraph, byte[] expectedProgram,
            ObjectNode documentOutcome, int protocolVersion, LocalVerifier.ProgramReceiptOutcome receipt) {
        return verifyTerminalInternal(encoded, availableGraph, expectedProgram, documentOutcome, protocolVersion, receipt, false);
    }

    private static String verifyTerminalInternal(byte[] encoded, byte[] availableGraph, byte[] expectedProgram,
            ObjectNode documentOutcome, int protocolVersion, LocalVerifier.ProgramReceiptOutcome receipt,
            boolean capturedOutcome) {
        try {
            if (encoded.length == 0 || encoded.length > MAX_CALLDATA_BYTES || !MessageDigest.isEqual(sha256(encoded), receipt.terminalPayloadRoot())
                    || availableGraph.length == 0 || !MessageDigest.isEqual(sha256(availableGraph), receipt.callGraphRoot())) throw new IllegalArgumentException();
            TerminalAttachments attachments = unwrapTerminal(unwrapAppliedTerminal(encoded, receipt));
            byte[] inner = attachments.inner();
            TerminalCursor cursor;
            boolean versionFive = starts(inner, "LXP/program-execution/v5\0");
            boolean candidate = starts(inner, "LXP/program-execution/v4\0") || versionFive;
            boolean successful = false;
            if (starts(inner, "LXP/program-execution/v2\0")
                    || starts(inner, "LXP/program-execution/v3\0")) {
                boolean traced = starts(inner, "LXP/program-execution/v3\0");
                cursor = new TerminalCursor(inner, (traced ? "LXP/program-execution/v3\0"
                    : "LXP/program-execution/v2\0").getBytes(StandardCharsets.UTF_8).length);
                int runtime = cursor.u16();
                int abi = cursor.u16();
                long metering = cursor.u32();
                BigInteger countValue = cursor.integer(16);
                if (countValue.bitLength() > 31) throw new IllegalArgumentException();
                int count = countValue.intValue();
                JsonNode values = documentOutcome.get("values");
                if (!"legacy_completed".equals(text(documentOutcome, "kind")) || !values.isArray()
                        || values.size() != count || runtime != receipt.runtimeVersion()
                        || abi != 1 || abi != receipt.abiVersion()
                        || metering != receipt.meteringScheduleVersion()) throw new IllegalArgumentException();
                for (int index = 0; index < count; index++) {
                    if (!(values.get(index) instanceof ObjectNode value)) throw new IllegalArgumentException();
                    int tag = cursor.u8();
                    if (tag == 1) {
                        if (!"i32".equals(text(value, "type"))
                                || cursor.i32() != requiredI32(value.get("value"))) throw new IllegalArgumentException();
                    } else if (tag == 2) {
                        if (!"i64".equals(text(value, "type"))
                                || cursor.i64() != signedI64(value.get("value"))) throw new IllegalArgumentException();
                    } else throw new IllegalArgumentException();
                }
                TerminalUsage usage = new TerminalUsage(cursor.u64(), cursor.u64(), cursor.u64(),
                    cursor.u64(), cursor.u32(), BigInteger.ZERO, cursor.integer(16));
                if (traced) {
                    if (cursor.u8() != 1 || cursor.sized64().length > 34 + 512 * 52) {
                        throw new IllegalArgumentException();
                    }
                }
                cursor.finish();
                if (receipt.terminalKind() != 1 || requiredI32(documentOutcome.get("code")) < 0) {
                    throw new IllegalArgumentException();
                }
                matchTerminalUsage(usage, receipt);
                successful = true;
            } else if (candidate) {
                cursor = new TerminalCursor(inner, (versionFive ? "LXP/program-execution/v5\0"
                    : "LXP/program-execution/v4\0").getBytes(StandardCharsets.UTF_8).length);
                int runtime = cursor.u16();
                long feeSchedule = cursor.u32();
                long metering = cursor.u32();
                BigInteger countValue = cursor.u64();
                if (countValue.bitLength() > 31) throw new IllegalArgumentException();
                int count = countValue.intValue();
                for (int index = 0; index < count; index++) {
                    int tag = cursor.u8();
                    if (tag == 1) cursor.i32();
                    else if (tag == 2) cursor.i64();
                    else throw new IllegalArgumentException();
                }
                TerminalUsage usage = new TerminalUsage(cursor.u64(), cursor.u64(), cursor.u64(),
                    cursor.u64(), cursor.u32(), cursor.u64(), cursor.integer(16));
                int traceTag = cursor.u8();
                if (traceTag == 1) {
                    if (cursor.sized64().length > 34 + 512 * 52) throw new IllegalArgumentException();
                } else if (traceTag != 0) throw new IllegalArgumentException();
                byte[] program = cursor.take(32);
                int abi = cursor.u16();
                if (versionFive ? protocolVersion != 3
                        || (abi != GeneratedReceiptContract.PROGRAM_ABI_V3
                            && abi != GeneratedReceiptContract.PROGRAM_ABI_V4)
                        : abi != GeneratedReceiptContract.PROGRAM_ABI_V2) throw new IllegalArgumentException();
                int outcomeTag = cursor.u8();
                String expectedKind;
                if (outcomeTag == 0) {
                    int code = cursor.i32();
                    byte[] response = cursor.sized64();
                    if (capturedOutcome) {
                        documentOutcome = JsonNodeFactory.instance.objectNode();
                        documentOutcome.put("kind", "completed");
                        documentOutcome.put("code", code);
                        documentOutcome.put("response", hex(response));
                    }
                    if (code < 0 || response.length > MAX_CALLDATA_BYTES
                            || !"completed".equals(text(documentOutcome, "kind"))
                            || code != requiredI32(documentOutcome.get("code"))
                            || !MessageDigest.isEqual(response,
                                responseBytes(documentOutcome))) {
                        throw new IllegalArgumentException();
                    }
                    expectedKind = "completed";
                    successful = true;
                } else if (outcomeTag == 1) {
                    validateAuthenticatedProgramFailure(cursor.sized64());
                    expectedKind = "guest_refused";
                } else if (outcomeTag == 2) {
                    validateCandidateResource(cursor, usage);
                    expectedKind = "resource";
                } else throw new IllegalArgumentException();
                byte[] graph = cursor.sized64();
                cursor.finish();
                if (graph.length > MAX_CALLDATA_BYTES || !MessageDigest.isEqual(graph, availableGraph)
                        || !MessageDigest.isEqual(program, expectedProgram) || abi != receipt.abiVersion()
                        || runtime != receipt.runtimeVersion() || feeSchedule != receipt.feeScheduleVersion()
                        || metering != receipt.meteringScheduleVersion()) throw new IllegalArgumentException();
                matchTerminalUsage(usage, receipt);
                if (outcomeTag == 0) {
                    if (receipt.terminalKind() != 1) throw new IllegalArgumentException();
                } else {
                    if (capturedOutcome) documentOutcome = capturedRefusal(expectedKind, receipt.resultCode());
                    requireRefusal(documentOutcome, expectedKind, receipt.resultCode());
                    if (receipt.terminalKind() == 1) throw new IllegalArgumentException();
                }
            } else if (starts(inner, "LXP/programs/failure-detail/v1\0")) {
                cursor = new TerminalCursor(inner, "LXP/programs/failure-detail/v1\0".getBytes(StandardCharsets.UTF_8).length);
                int tag = cursor.u8();
                byte[] payload = cursor.sized32();
                cursor.finish();
                if (tag < 1 || tag > 4 || payload.length == 0) throw new IllegalArgumentException();
                validateFailureDetail(tag, payload);
                requireRefusal(documentOutcome, "guest_refused", receipt.resultCode());
            } else if (starts(inner, "LXP/programs/resource-detail/v1\0")) {
                cursor = new TerminalCursor(inner, "LXP/programs/resource-detail/v1\0".getBytes(StandardCharsets.UTF_8).length);
                validateLegacyResource(cursor);
                cursor.finish();
                requireRefusal(documentOutcome, "resource", receipt.resultCode());
            } else if (starts(inner, "LXP/programs/settlement-failure/v1\0")) {
                cursor = new TerminalCursor(inner, "LXP/programs/settlement-failure/v1\0".getBytes(StandardCharsets.UTF_8).length);
                if (!Set.of(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12).contains(cursor.u8())) {
                    throw new IllegalArgumentException();
                }
                cursor.finish();
                if (capturedOutcome) documentOutcome = capturedRefusal("guest_refused", receipt.resultCode());
                requireRefusal(documentOutcome, "guest_refused", receipt.resultCode());
            } else if (starts(inner, "LXP/programs/callback-failure/v1\0")) {
                cursor = new TerminalCursor(inner, "LXP/programs/callback-failure/v1\0".getBytes(StandardCharsets.UTF_8).length);
                cursor.u8();
                cursor.i32();
                cursor.finish();
                if (capturedOutcome) documentOutcome = capturedRefusal("guest_refused", receipt.resultCode());
                requireRefusal(documentOutcome, "guest_refused", receipt.resultCode());
            } else throw new IllegalArgumentException();
            return verifyTerminalAttachments(attachments, candidate, successful, protocolVersion, receipt);
        } catch (RuntimeException error) {
            PlatformSdkException failure = PlatformSdkException.verificationFailure();
            failure.initCause(error);
            throw failure;
        }
    }

    private static ObjectNode capturedRefusal(String kind, int code) {
        ObjectNode outcome = JsonNodeFactory.instance.objectNode();
        outcome.put("kind", "refused");
        ObjectNode failure = outcome.putObject("failure");
        failure.put("kind", kind);
        if ("guest_refused".equals(kind)) failure.put("code", code);
        return outcome;
    }

    private static void requireRefusal(ObjectNode outcome, String expected, int code) {
        if (!"refused".equals(text(outcome, "kind"))) throw new IllegalArgumentException();
        ObjectNode failure = object(outcome, "failure");
        if (!expected.equals(text(failure, "kind"))) throw new IllegalArgumentException();
        if ("guest_refused".equals(expected) && requiredI32(failure.get("code")) != code) {
            throw new IllegalArgumentException();
        }
    }

    private static void matchTerminalUsage(TerminalUsage usage,
                                           LocalVerifier.ProgramReceiptOutcome receipt) {
        if (!usage.cpu().equals(receipt.cpuFuel()) || !usage.memory().equals(receipt.memoryBytes())
                || !usage.read().equals(receipt.storageReadBytes())
                || !usage.write().equals(receipt.storageWriteBytes())
                || usage.values() != receipt.outputValues()
                || !usage.outputBytes().equals(receipt.outputBytes())
                || !usage.fee().equals(receipt.feeUnits())) throw new IllegalArgumentException();
    }

    private static void validateAuthenticatedProgramFailure(byte[] encoded) {
        TerminalCursor cursor = new TerminalCursor(encoded, 0);
        byte[] program = cursor.take(32);
        long failureClass = cursor.u32();
        byte[] reason = cursor.sized32();
        cursor.finish();
        if (allZero(program) || !Set.of(1L, 2L, 3L, 4L, 5L, 254L, 255L).contains(failureClass)
                || reason.length > 4_096 || (failureClass == 254 || failureClass == 255) && reason.length != 0) {
            throw new IllegalArgumentException();
        }
    }

    private static void validateFailureDetail(int family, byte[] encoded) {
        if (family == 1) {
            validateAuthenticatedProgramFailure(encoded);
            return;
        }
        TerminalCursor cursor = new TerminalCursor(encoded, 0);
        int tag = cursor.u8();
        if (family == 2) validateCompositionFailure(cursor, tag);
        else if (family == 3) validateEntrypointFailure(cursor, tag);
        else if (family == 4) validateAbiFailure(cursor, tag);
        else throw new IllegalArgumentException();
        cursor.finish();
    }

    private static void validateCompositionFailure(TerminalCursor cursor, int tag) {
        switch (tag) {
            case 1, 9, 10, 11, 20, 21, 22 -> { }
            case 2 -> {
                int expected = cursor.u8();
                int actual = cursor.u8();
                if (expected < 1 || expected > 2 || actual < 1 || actual > 2) {
                    throw new IllegalArgumentException();
                }
            }
            case 3, 4 -> requireNonzero(cursor.take(32));
            case 5, 6, 7 -> { cursor.u32(); cursor.u32(); }
            case 8 -> { requireNonzero(cursor.take(32)); cursor.u32(); cursor.u32(); }
            case 12 -> cursor.i32();
            case 13 -> { cursor.u64(); cursor.u64(); }
            case 14 -> { requireNonzero(cursor.take(32)); cursor.i32(); }
            case 15 -> validateAuthenticatedProgramFailure(cursor.rest());
            case 16 -> validateNestedAbi(cursor);
            case 17 -> validateFault(cursor);
            case 18 -> validateMeterFailure(cursor);
            case 19 -> validateResponseFailure(cursor);
            case 23 -> { cursor.take(76); cursor.take(76); }
            default -> throw new IllegalArgumentException();
        }
    }

    private static void validateEntrypointFailure(TerminalCursor cursor, int tag) {
        switch (tag) {
            case 1 -> { cursor.u64(); cursor.u64(); }
            case 2, 3, 4 -> { }
            case 5, 6 -> cursor.i32();
            case 7 -> validateFault(cursor);
            case 8 -> validateMeterFailure(cursor);
            default -> throw new IllegalArgumentException();
        }
    }

    private static void validateAbiFailure(TerminalCursor cursor, int tag) {
        if (tag >= 1 && tag <= 10 || tag >= 13 && tag <= 15) return;
        if (tag == 11) {
            int storage = cursor.u8();
            if (storage < 1 || storage > 11) throw new IllegalArgumentException();
        } else if (tag == 12) validateMeterFailure(cursor);
        else throw new IllegalArgumentException();
    }

    private static void validateNestedAbi(TerminalCursor cursor) {
        validateAbiFailure(cursor, cursor.u8());
    }

    private static void validateMeterFailure(TerminalCursor cursor) {
        int tag = cursor.u8();
        if (tag == 1) {
            int resource = cursor.u8();
            BigInteger limit = cursor.u64();
            BigInteger attempted = cursor.u64();
            if (resource < 1 || resource > 7 || attempted.compareTo(limit) <= 0) {
                throw new IllegalArgumentException();
            }
        } else if (tag == 2) {
            int resource = cursor.u8();
            if (resource < 1 || resource > 7) throw new IllegalArgumentException();
        } else if (tag != 3) throw new IllegalArgumentException();
    }

    private static void validateFault(TerminalCursor cursor) {
        int tag = cursor.u8();
        if (tag == 1 || tag == 2 || tag == 16) {
            byte[] name = cursor.sized32();
            if (!Arrays.equals(name, new String(name, StandardCharsets.UTF_8)
                    .getBytes(StandardCharsets.UTF_8))) throw new IllegalArgumentException();
        } else if (tag >= 3 && tag <= 13 || tag == 15) {
            return;
        } else if (tag == 14) validateMeterFailure(cursor);
        else throw new IllegalArgumentException();
    }

    private static void validateResponseFailure(TerminalCursor cursor) {
        int tag = cursor.u8();
        if (tag == 1 || tag == 2) { cursor.u64(); cursor.u64(); }
        else if (tag == 3 || tag == 4) return;
        else if (tag == 5) { cursor.i32(); cursor.i32(); }
        else if (tag == 6) validateMeterFailure(cursor);
        else throw new IllegalArgumentException();
    }

    private static void requireNonzero(byte[] value) {
        if (allZero(value)) throw new IllegalArgumentException();
    }

    private static void validateCandidateResource(TerminalCursor cursor, TerminalUsage usage) {
        int tag = cursor.u8();
        int resource = cursor.u8();
        if (resource < 0 || resource > 6) throw new IllegalArgumentException();
        if (tag == 0) {
            BigInteger limit = cursor.u64();
            BigInteger attempted = cursor.u64();
            if (attempted.compareTo(limit) <= 0 || candidateUsage(usage, resource).compareTo(limit) > 0) {
                throw new IllegalArgumentException();
            }
        } else if (tag != 1) throw new IllegalArgumentException();
    }

    private static BigInteger candidateUsage(TerminalUsage usage, int resource) {
        return switch (resource) {
            case 0 -> usage.cpu();
            case 1 -> usage.memory();
            case 2 -> usage.read();
            case 3 -> usage.write();
            case 4 -> BigInteger.valueOf(usage.values());
            case 5 -> usage.outputBytes();
            case 6 -> BigInteger.ZERO;
            default -> throw new IllegalArgumentException();
        };
    }

    private static void validateLegacyResource(TerminalCursor cursor) {
        int tag = cursor.u8();
        int resource = cursor.u8();
        if (resource < 1 || resource > 7) throw new IllegalArgumentException();
        if (tag == 1) {
            BigInteger limit = cursor.u64();
            if (cursor.u64().compareTo(limit) <= 0) throw new IllegalArgumentException();
        } else if (tag != 2) throw new IllegalArgumentException();
    }

    private static String verifyTerminalAttachments(TerminalAttachments attachments, boolean candidate,
            boolean successful, int protocolVersion, LocalVerifier.ProgramReceiptOutcome receipt) {
        boolean occupancyRequired = (protocolVersion == 2 || protocolVersion == 3) && successful;
        if (occupancyRequired != (attachments.occupancy() != null)) throw new IllegalArgumentException();
        if (attachments.occupancy() != null) {
            byte[] occupancy = attachments.occupancy();
            if (occupancy.length == 0) {
                if (!allZero(receipt.occupancyEvidenceDigest()) || !allZero(receipt.occupancyTransferRoot())
                        || receipt.occupancyByteBatches().signum() != 0
                        || receipt.occupancyFeeUnits().signum() != 0) throw new IllegalArgumentException();
            } else {
                if (!MessageDigest.isEqual(sha256(occupancy), receipt.occupancyEvidenceDigest())) {
                    throw new IllegalArgumentException();
                }
                verifyOccupancyBinding(occupancy, receipt.occupancyAssetId(),
                    receipt.occupancyByteBatches(), receipt.occupancyFeeUnits(),
                    receipt.occupancyTransferRoot());
            }
        } else if (!allZero(receipt.occupancyEvidenceDigest())
                || !allZero(receipt.occupancyTransferRoot())
                || receipt.occupancyByteBatches().signum() != 0
                || receipt.occupancyFeeUnits().signum() != 0) {
            throw new IllegalArgumentException();
        }
        boolean transferPresent = !allZero(receipt.transferRoot());
        boolean recorded = receipt.encodingVersion() != 4 && attachments.authorization() == null && transferPresent;
        boolean authorityRequired = candidate || receipt.encodingVersion() == 4 && successful;
        if (!recorded && (authorityRequired ? (attachments.authorization() != null) != transferPresent
                : attachments.authorization() != null)) throw new IllegalArgumentException();
        if (attachments.authorization() != null) {
            if (attachments.authorization().length == 0 || attachments.transferRoot() == null
                    || !MessageDigest.isEqual(attachments.transferRoot(), receipt.transferRoot())) {
                throw new IllegalArgumentException();
            }
            if (receipt.encodingVersion() == 4 && !authorizationV2(attachments.authorization())) throw new IllegalArgumentException();
            verifyAuthorizationRoot(attachments.authorization(), attachments.transferRoot());
        }
        if (protocolVersion != 1 && protocolVersion != 2 && protocolVersion != 3) throw new IllegalArgumentException();
        return recorded ? "recorded_terminal_root_not_locally_reconstructable" : "reconstructed";
    }

    static TerminalAttachments unwrapTerminal(byte[] encoded) {
        byte[] current = encoded;
        byte[] authorization = null;
        byte[] transferRoot = null;
        byte[] occupancy = null;
        byte[] authorityDomain = "LXP/program-execution-with-transfer-authority/v2\0"
            .getBytes(StandardCharsets.UTF_8);
        byte[] occupancyDomain = "LXP/program-execution-with-occupancy/v1\0"
            .getBytes(StandardCharsets.UTF_8);
        if (starts(current, authorityDomain)) {
            TerminalCursor cursor = new TerminalCursor(current, authorityDomain.length);
            current = cursor.sized32(MAX_CALLDATA_BYTES);
            authorization = cursor.sized32(MAX_CALLDATA_BYTES);
            transferRoot = cursor.take(32);
            cursor.finish();
        }
        if (starts(current, occupancyDomain)) {
            TerminalCursor cursor = new TerminalCursor(current, occupancyDomain.length);
            current = cursor.sized32(MAX_CALLDATA_BYTES);
            occupancy = cursor.sized32(65_536);
            cursor.finish();
        }
        if (starts(current, authorityDomain) || starts(current, occupancyDomain)) {
            throw new IllegalArgumentException();
        }
        return new TerminalAttachments(current, occupancy, authorization, transferRoot);
    }

    private static boolean authorizationV2(byte[] encoded) {
        byte[] domain = "LayerX/programs/402LXP/account-bound-set/v1\0".getBytes(StandardCharsets.UTF_8);
        if (starts(encoded, domain)) encoded = new TerminalCursor(encoded, domain.length).sized32(MAX_CALLDATA_BYTES);
        return starts(encoded, TRANSFER_SET_V2_DOMAIN);
    }

    private static byte[] principalPaymentAccount(byte[] principal, byte[] asset, byte[] name) {
        if (name.length > 512) throw new IllegalArgumentException();
        for (byte b : name) if (!(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '.' || b == '_' || b == '-' || b == ':')) throw new IllegalArgumentException();
        String text = new String(name, StandardCharsets.US_ASCII);
        if (!text.startsWith("agent:") || text.contains("::")) throw new IllegalArgumentException();
        String suffix = text.endsWith(":main") ? ":main" : ":asset:" + java.util.HexFormat.of().formatHex(asset);
        if (!text.endsWith(suffix) || text.length() <= 6 + suffix.length()) throw new IllegalArgumentException();
        byte[] did = text.substring(6, text.length() - suffix.length()).getBytes(StandardCharsets.US_ASCII);
        if (did[0] == ':' || did[did.length - 1] == ':' || !MessageDigest.isEqual(sha256(
                "LXP/v1/did-id\0".getBytes(StandardCharsets.UTF_8), fixedUnsigned(BigInteger.valueOf(did.length), 2), did), principal)) throw new IllegalArgumentException();
        return sha256("LX:ACCOUNT:v1".getBytes(StandardCharsets.UTF_8), fixedUnsigned(BigInteger.valueOf(name.length), 4), name);
    }

    static void verifyAuthorizationRoot(byte[] encoded, byte[] expected) {
        if (encoded == null || encoded.length == 0 || encoded.length > MAX_CALLDATA_BYTES
                || expected == null || expected.length != 32) throw new IllegalArgumentException();
        byte[] boundDomain = "LayerX/programs/402LXP/account-bound-set/v1\0".getBytes(StandardCharsets.UTF_8);
        TerminalCursor names = null;
        if (starts(encoded, boundDomain)) {
            names = new TerminalCursor(encoded, boundDomain.length);
            encoded = names.sized32(MAX_CALLDATA_BYTES);
            if (starts(encoded, boundDomain)) throw new IllegalArgumentException();
        }
        boolean candidate = starts(encoded, TRANSFER_SET_V2_DOMAIN);
        byte[] domain = candidate ? TRANSFER_SET_V2_DOMAIN : TRANSFER_SET_V1_DOMAIN;
        TerminalCursor cursor = new TerminalCursor(encoded, 0);
        if (!starts(encoded, domain)
                || !MessageDigest.isEqual(cursor.take(domain.length), domain)) {
            throw new IllegalArgumentException();
        }
        requireNonzero(cursor.take(32));
        byte[] principal = cursor.take(32);
        requireNonzero(principal);
        requireNonzero(cursor.take(32));
        decodeFrame(cursor);
        decodeEventEnvelope(cursor.sized32(MAX_CALLDATA_BYTES));
        BigInteger callCount = cursor.u64();
        if (callCount.compareTo(BigInteger.valueOf(64)) > 0) throw new IllegalArgumentException();
        for (int index = 0; index < callCount.intValue(); index++) {
            requireNonzero(cursor.take(32));
            requireNonzero(cursor.take(32));
            requireNonzero(cursor.take(32));
            decodeFrame(cursor);
            decodeFrame(cursor);
            decodeCapabilitySet(cursor.sized32(65_535), candidate);
        }
        BigInteger legCount = cursor.u64();
        if (legCount.signum() == 0 || legCount.compareTo(BigInteger.valueOf(256)) > 0) {
            throw new IllegalArgumentException();
        }
        List<byte[]> kernelLegs = new ArrayList<>();
        BigInteger total = BigInteger.ZERO;
        for (int index = 0; index < legCount.intValue(); index++) {
            byte[] frame = decodeFrame(cursor);
            byte[] source = principal;
            ProgramAuthorityBinding authority = null;
            ProgramFundingBinding funding = null;
            if (candidate) {
                int sourceTag = cursor.u8();
                if (sourceTag == 1) {
                    source = cursor.take(32);
                    requireNonzero(source);
                    if (!MessageDigest.isEqual(source, principal)) throw new IllegalArgumentException();
                } else if (sourceTag == 2) {
                    authority = decodeProgramAuthority(cursor.sized32(MAX_CALLDATA_BYTES));
                    source = authority.source();
                } else if (sourceTag == 3) {
                    source = cursor.take(32);
                    requireNonzero(source);
                    if (!MessageDigest.isEqual(source, principal)) throw new IllegalArgumentException();
                    funding = decodeProgramFunding(cursor.sized32(MAX_CALLDATA_BYTES));
                } else throw new IllegalArgumentException();
            }
            byte[] asset = cursor.take(32);
            byte[] destination = cursor.take(32);
            BigInteger amount = cursor.integer(16);
            byte[] program = cursor.take(32);
            requireNonzero(asset);
            requireNonzero(destination);
            requireNonzero(program);
            if (amount.signum() == 0) throw new IllegalArgumentException();
            if (authority != null && (!MessageDigest.isEqual(authority.owner(), program)
                    || !MessageDigest.isEqual(authority.frame(), frame)
                    || !MessageDigest.isEqual(authority.asset(), asset)
                    || !MessageDigest.isEqual(authority.destination(), destination)
                    || !authority.amount().equals(amount))) throw new IllegalArgumentException();
            if (funding != null && (!MessageDigest.isEqual(funding.owner(), program)
                    || !MessageDigest.isEqual(funding.destination(), destination)
                    || !MessageDigest.isEqual(funding.asset(), asset))) {
                throw new IllegalArgumentException();
            }
            if (names != null) {
                byte[] name = names.take(names.u16());
                if (authority != null) { if (name.length != 0) throw new IllegalArgumentException(); }
                else source = principalPaymentAccount(source, asset, name);
            }
            total = checkedU128Add(total, amount);
            kernelLegs.add(concatenate(new byte[] {0}, source, destination, asset,
                fixedUnsigned(amount, 16), fixedUnsigned(BigInteger.ONE, 2)));
        }
        cursor.finish();
        if (names != null) names.finish();
        if (!MessageDigest.isEqual(merkleRoot(kernelLegs), expected)) {
            throw new IllegalArgumentException();
        }
    }

    private static ProgramAuthorityBinding decodeProgramAuthority(byte[] encoded) {
        TerminalCursor cursor = new TerminalCursor(encoded, 0);
        if (!MessageDigest.isEqual(cursor.take(PROGRAM_AUTHORITY_DOMAIN.length),
                PROGRAM_AUTHORITY_DOMAIN)) throw new IllegalArgumentException();
        byte[] owner = cursor.take(32);
        requireNonzero(owner);
        int seedLength = cursor.u16();
        if (seedLength > 128) throw new IllegalArgumentException();
        byte[] seed = cursor.take(seedLength);
        byte[] source = cursor.take(32);
        byte[] frame = decodeFrame(cursor);
        byte[] asset = cursor.take(32);
        byte[] destination = cursor.take(32);
        BigInteger amount = cursor.integer(16);
        cursor.finish();
        requireNonzero(asset);
        requireNonzero(destination);
        if (amount.signum() == 0
                || !MessageDigest.isEqual(deriveProgramAccount(owner, seed), source)) {
            throw new IllegalArgumentException();
        }
        return new ProgramAuthorityBinding(owner, frame, source, asset, destination, amount);
    }

    private static ProgramFundingBinding decodeProgramFunding(byte[] encoded) {
        TerminalCursor cursor = new TerminalCursor(encoded, 0);
        if (!MessageDigest.isEqual(cursor.take(PROGRAM_FUNDING_DOMAIN.length),
                PROGRAM_FUNDING_DOMAIN)) throw new IllegalArgumentException();
        byte[] owner = cursor.take(32);
        requireNonzero(owner);
        int seedLength = cursor.u16();
        if (seedLength > 128) throw new IllegalArgumentException();
        byte[] seed = cursor.take(seedLength);
        byte[] destination = cursor.take(32);
        byte[] asset = cursor.take(32);
        cursor.finish();
        requireNonzero(destination);
        requireNonzero(asset);
        if (!MessageDigest.isEqual(deriveProgramAccount(owner, seed), destination)) {
            throw new IllegalArgumentException();
        }
        return new ProgramFundingBinding(owner, destination, asset);
    }

    private static byte[] deriveProgramAccount(byte[] owner, byte[] seed) {
        return sha256(PROGRAM_ACCOUNT_DOMAIN, owner,
            fixedUnsigned(BigInteger.valueOf(seed.length), 4), seed);
    }

    private static void decodeEventEnvelope(byte[] encoded) {
        TerminalCursor cursor = new TerminalCursor(encoded, 0);
        if (!MessageDigest.isEqual(cursor.take(EVENT_ENVELOPE_DOMAIN.length),
                EVENT_ENVELOPE_DOMAIN)) throw new IllegalArgumentException();
        long count = cursor.u32();
        if (count > 64) throw new IllegalArgumentException();
        for (int index = 0; index < (int) count; index++) {
            requireNonzero(cursor.take(32));
            requireNonzero(cursor.take(32));
            decodeFrame(cursor);
            cursor.sized32(64);
            cursor.sized32(65_536);
        }
        cursor.finish();
    }

    private static byte[] decodeFrame(TerminalCursor cursor) {
        byte[] path = cursor.take(8);
        int depth = cursor.u8();
        if (depth > 8) throw new IllegalArgumentException();
        for (int index = 0; index < path.length; index++) {
            if (index < depth && path[index] == 0 || index >= depth && path[index] != 0) {
                throw new IllegalArgumentException();
            }
        }
        return concatenate(path, new byte[] {(byte) depth});
    }

    private static void decodeCapabilitySet(byte[] encoded, boolean v2) {
        for (var grant : NativeCapabilitySet.decode(encoded)) {
            if (!v2 && (grant instanceof NativeCapabilitySet.ProgramSpend || grant instanceof NativeCapabilitySet.BalanceView)) {
                throw new IllegalArgumentException("ABI v2 capability required");
            }
        }
    }

    static void verifyOccupancyBinding(byte[] encoded, byte[] asset, BigInteger expectedByteBatches,
                                       BigInteger expectedFeeUnits, byte[] expectedTransferRoot) {
        OccupancySettlement settlement = decodeOccupancySettlement(encoded);
        if (expectedByteBatches == null || expectedFeeUnits == null
                || !settlement.byteBatches().equals(expectedByteBatches)
                || !settlement.feeUnits().equals(expectedFeeUnits)
                || !MessageDigest.isEqual(occupancyTransferRoot(settlement, asset),
                    exactInternal(expectedTransferRoot, 32))) throw new IllegalArgumentException();
    }

    private static OccupancySettlement decodeOccupancySettlement(byte[] encoded) {
        if (encoded == null || encoded.length > 65_536) throw new IllegalArgumentException();
        if (starts(encoded, OCCUPANCY_V1_DOMAIN) || starts(encoded, OCCUPANCY_V2_DOMAIN)) {
            return decodeLegacyOccupancy(encoded);
        }
        TerminalCursor cursor = new TerminalCursor(encoded, 0);
        if (!MessageDigest.isEqual(cursor.take(OCCUPANCY_V3_DOMAIN.length),
                OCCUPANCY_V3_DOMAIN)) throw new IllegalArgumentException();
        BigInteger batch = cursor.u64();
        BigInteger occupancyPrice = decodeOccupancySchedule(cursor, true);
        BigInteger declaredUnits = cursor.integer(16);
        BigInteger declaredFee = cursor.integer(16);
        BigInteger declaredPaid = cursor.integer(16);
        BigInteger declaredArrears = cursor.integer(16);
        long count = cursor.u32();
        if (count > 256) throw new IllegalArgumentException();
        BigInteger byteBatches = BigInteger.ZERO;
        BigInteger feeUnits = BigInteger.ZERO;
        BigInteger paidUnits = BigInteger.ZERO;
        BigInteger arrearsUnits = BigInteger.ZERO;
        byte[] priorNamespace = null;
        List<OccupancyCharge> charges = new ArrayList<>();
        for (int index = 0; index < (int) count; index++) {
            StorageNamespace namespace = decodeStorageNamespace(cursor);
            if (priorNamespace != null && compareBytes(priorNamespace, namespace.canonical()) >= 0) {
                throw new IllegalArgumentException();
            }
            priorNamespace = namespace.canonical();
            byte[] payer = cursor.take(32);
            requireNonzero(payer);
            if (namespace.principal() != null
                    && !MessageDigest.isEqual(namespace.principal(), payer)) {
                throw new IllegalArgumentException();
            }
            byte[] rootProgram = cursor.take(32);
            requireNonzero(rootProgram);
            byte[] activity = cursor.take(32);
            BigInteger fromBatch = cursor.u64();
            BigInteger toBatch = cursor.u64();
            BigInteger recordedBytes = cursor.u64();
            BigInteger finalBytes = cursor.u64();
            BigInteger units = cursor.integer(16);
            BigInteger price = cursor.u64();
            BigInteger accrued = cursor.integer(16);
            BigInteger priorArrears = cursor.integer(16);
            BigInteger amountDue = cursor.integer(16);
            BigInteger authorizedAdded = cursor.integer(16);
            int disposition = cursor.u8();
            if (disposition < 1 || disposition > 5) throw new IllegalArgumentException();
            BigInteger arrearsAfter = cursor.integer(16);
            BigInteger maximumBytes = cursor.u64();
            BigInteger maximumPrice = cursor.u64();
            cursor.integer(16);
            byte[] mandate = cursor.take(32);
            if (toBatch.compareTo(fromBatch) < 0) throw new IllegalArgumentException();
            BigInteger expectedUnits = checkedU128Multiply(recordedBytes,
                toBatch.subtract(fromBatch));
            BigInteger expectedFee = checkedU128Multiply(expectedUnits, price);
            BigInteger expectedDue = checkedU128Add(priorArrears, expectedFee);
            boolean migration = disposition == 5;
            if (!toBatch.equals(batch) || !migration && !price.equals(occupancyPrice)
                    || !units.equals(expectedUnits) || !accrued.equals(expectedFee)
                    || !amountDue.equals(expectedDue) || finalBytes.compareTo(maximumBytes) > 0
                    || !migration && (allZero(mandate) || allZero(activity))
                    || migration && (price.signum() != 0 || accrued.signum() != 0
                        || priorArrears.signum() != 0 || amountDue.signum() != 0
                        || arrearsAfter.signum() != 0 || !allZero(mandate) || !allZero(activity)
                        || !MessageDigest.isEqual(rootProgram, namespace.program()))
                    || (disposition == 4) != (price.compareTo(maximumPrice) > 0)
                    || disposition == 1 && arrearsAfter.signum() != 0
                    || disposition != 1 && !arrearsAfter.equals(amountDue)) {
                throw new IllegalArgumentException();
            }
            if (authorizedAdded.signum() != 0) {
                byte[] expectedMandate = sha256(OCCUPANCY_MANDATE_DOMAIN, payer, rootProgram,
                    activity, namespace.wire(), fixedUnsigned(maximumBytes, 8),
                    fixedUnsigned(maximumPrice, 8), fixedUnsigned(authorizedAdded, 16));
                if (!MessageDigest.isEqual(mandate, expectedMandate)) {
                    throw new IllegalArgumentException();
                }
            }
            byteBatches = checkedU128Add(byteBatches, units);
            feeUnits = checkedU128Add(feeUnits, accrued);
            if (disposition == 1) paidUnits = checkedU128Add(paidUnits, amountDue);
            else arrearsUnits = checkedU128Add(arrearsUnits, arrearsAfter);
            charges.add(new OccupancyCharge(payer, amountDue, disposition == 1, arrearsAfter));
        }
        cursor.finish();
        if (!byteBatches.equals(declaredUnits) || !feeUnits.equals(declaredFee)
                || !paidUnits.equals(declaredPaid) || !arrearsUnits.equals(declaredArrears)) {
            throw new IllegalArgumentException();
        }
        return new OccupancySettlement(byteBatches, feeUnits, List.copyOf(charges));
    }

    private static OccupancySettlement decodeLegacyOccupancy(byte[] encoded) {
        boolean versioned = starts(encoded, OCCUPANCY_V2_DOMAIN);
        byte[] domain = versioned ? OCCUPANCY_V2_DOMAIN : OCCUPANCY_V1_DOMAIN;
        TerminalCursor cursor = new TerminalCursor(encoded, 0);
        if (!MessageDigest.isEqual(cursor.take(domain.length), domain)) {
            throw new IllegalArgumentException();
        }
        BigInteger batch = cursor.u64();
        BigInteger occupancyPrice = decodeOccupancySchedule(cursor, versioned);
        BigInteger declaredUnits = cursor.integer(16);
        BigInteger declaredFee = cursor.integer(16);
        BigInteger count = cursor.u64();
        if (count.compareTo(BigInteger.valueOf(256)) > 0) throw new IllegalArgumentException();
        BigInteger byteBatches = BigInteger.ZERO;
        BigInteger feeUnits = BigInteger.ZERO;
        List<OccupancyCharge> charges = new ArrayList<>();
        for (int index = 0; index < count.intValue(); index++) {
            decodeStorageNamespace(cursor);
            byte[] payer = cursor.take(32);
            requireNonzero(payer);
            BigInteger fromBatch = cursor.u64();
            BigInteger toBatch = cursor.u64();
            BigInteger recordedBytes = cursor.u64();
            cursor.u64();
            BigInteger units = cursor.integer(16);
            BigInteger price = cursor.u64();
            BigInteger accrued = cursor.integer(16);
            if (toBatch.compareTo(fromBatch) < 0) throw new IllegalArgumentException();
            BigInteger expectedUnits = checkedU128Multiply(recordedBytes,
                toBatch.subtract(fromBatch));
            if (!toBatch.equals(batch) || !units.equals(expectedUnits)
                    || !price.equals(occupancyPrice)
                    || !accrued.equals(checkedU128Multiply(units, price))) {
                throw new IllegalArgumentException();
            }
            byteBatches = checkedU128Add(byteBatches, units);
            feeUnits = checkedU128Add(feeUnits, accrued);
            charges.add(new OccupancyCharge(payer, accrued, true, BigInteger.ZERO));
        }
        cursor.finish();
        if (!byteBatches.equals(declaredUnits) || !feeUnits.equals(declaredFee)) {
            throw new IllegalArgumentException();
        }
        return new OccupancySettlement(byteBatches, feeUnits, List.copyOf(charges));
    }

    private static BigInteger decodeOccupancySchedule(TerminalCursor cursor, boolean versioned) {
        long version = versioned ? cursor.u32() : 1;
        if (version == 0) throw new IllegalArgumentException();
        BigInteger occupancyPrice = BigInteger.ZERO;
        for (int index = 0; index < 7; index++) occupancyPrice = cursor.u64();
        return occupancyPrice;
    }

    private static StorageNamespace decodeStorageNamespace(TerminalCursor cursor) {
        int length = cursor.u8();
        if (length != 33 && length != 65) throw new IllegalArgumentException();
        byte[] canonical = cursor.take(length);
        byte[] program = Arrays.copyOf(canonical, 32);
        requireNonzero(program);
        int tag = canonical[32] & 0xff;
        byte[] principal = null;
        if (tag == 0 && length == 65) {
            principal = Arrays.copyOfRange(canonical, 33, 65);
            requireNonzero(principal);
        } else if (!(tag == 1 && length == 33) && !(tag == 2 && length == 65)) {
            throw new IllegalArgumentException();
        }
        return new StorageNamespace(canonical,
            concatenate(new byte[] {(byte) length}, canonical), program, principal);
    }

    private static byte[] occupancyTransferRoot(OccupancySettlement settlement, byte[] asset) {
        byte[] boundedAsset = exactInternal(asset, 32);
        requireNonzero(boundedAsset);
        Map<String, PayerAggregate> payers = new HashMap<>();
        for (OccupancyCharge charge : settlement.charges()) {
            String key = hex(charge.payer());
            PayerAggregate prior = payers.getOrDefault(key, new PayerAggregate(charge.payer(),
                BigInteger.ZERO, BigInteger.ZERO, BigInteger.ZERO));
            BigInteger due = checkedU128Add(prior.due(), charge.amountDue());
            BigInteger paid = charge.paid()
                ? checkedU128Add(prior.paid(), charge.amountDue()) : prior.paid();
            BigInteger arrears = checkedU128Add(prior.arrears(), charge.arrearsAfter());
            payers.put(key, new PayerAggregate(prior.payer(), due, paid, arrears));
        }
        byte[] treasury = sha256(ACCOUNT_DERIVATION_DOMAIN,
            fixedUnsigned(BigInteger.valueOf(11), 4), FEE_TREASURY_LABEL);
        List<PayerAggregate> ordered = new ArrayList<>(payers.values());
        ordered.removeIf(value -> value.due().signum() == 0 && value.arrears().signum() == 0);
        ordered.sort((left, right) -> compareBytes(left.payer(), right.payer()));
        List<byte[]> legs = new ArrayList<>();
        for (PayerAggregate payer : ordered) {
            if (payer.paid().signum() != 0) {
                legs.add(concatenate(new byte[] {0}, payer.payer(), treasury, boundedAsset,
                    fixedUnsigned(payer.paid(), 16), fixedUnsigned(BigInteger.valueOf(23), 2)));
            }
        }
        return merkleRoot(legs);
    }

    static byte[] unwrapAppliedTerminal(byte[] encoded, LocalVerifier.ProgramReceiptOutcome receipt) {
        if (receipt.encodingVersion() != 4) return encoded;
        byte[] domain = "LXP/programs/terminal-applied-legs/v1\0".getBytes(StandardCharsets.UTF_8);
        TerminalCursor cursor = new TerminalCursor(encoded, 0);
        if (!MessageDigest.isEqual(cursor.take(domain.length), domain)) throw new IllegalArgumentException();
        byte[] inner = cursor.sized32(), legs = cursor.sized32();
        cursor.finish();
        if (inner.length == 0 || inner.length > MAX_CALLDATA_BYTES || legs.length > 256 * 115 || legs.length % 115 != 0
                || !MessageDigest.isEqual(sha256(legs), receipt.appliedLegsDigest())) throw new IllegalArgumentException();
        List<byte[]> entries = new ArrayList<>();
        for (int offset = 0; offset < legs.length; offset += 115) {
            byte[] leg = Arrays.copyOfRange(legs, offset, offset + 115);
            if (leg[0] != 0 || leg[113] != 0 || leg[114] != 1 || allZero(Arrays.copyOfRange(leg, 1, 33))
                    || allZero(Arrays.copyOfRange(leg, 33, 65)) || allZero(Arrays.copyOfRange(leg, 65, 97))
                    || allZero(Arrays.copyOfRange(leg, 97, 113))) throw new IllegalArgumentException();
            entries.add(leg);
        }
        if (!MessageDigest.isEqual(merkleRoot(entries), receipt.transferRoot())) throw new IllegalArgumentException("applied transfer root");
        return inner;
    }

    private static byte[] merkleRoot(List<byte[]> legs) {
        if (legs.isEmpty()) return new byte[32];
        List<byte[]> level = new ArrayList<>();
        for (byte[] leg : legs) level.add(sha256(MERKLE_LEAF_DOMAIN, leg));
        while (level.size() > 1) {
            List<byte[]> next = new ArrayList<>();
            for (int index = 0; index < level.size(); index += 2) {
                byte[] left = level.get(index);
                byte[] right = index + 1 < level.size() ? level.get(index + 1) : left;
                next.add(sha256(MERKLE_INTERNAL_DOMAIN, left, right));
            }
            level = next;
        }
        return level.get(0);
    }

    private static BigInteger checkedU128Add(BigInteger left, BigInteger right) {
        BigInteger result = left.add(right);
        if (left.signum() < 0 || right.signum() < 0 || result.compareTo(MAX_U128) > 0) {
            throw new IllegalArgumentException();
        }
        return result;
    }

    private static BigInteger checkedU128Multiply(BigInteger left, BigInteger right) {
        BigInteger result = left.multiply(right);
        if (left.signum() < 0 || right.signum() < 0 || result.compareTo(MAX_U128) > 0) {
            throw new IllegalArgumentException();
        }
        return result;
    }

    private static int compareBytes(byte[] left, byte[] right) {
        int length = Math.min(left.length, right.length);
        for (int index = 0; index < length; index++) {
            int order = Integer.compare(left[index] & 0xff, right[index] & 0xff);
            if (order != 0) return order;
        }
        return Integer.compare(left.length, right.length);
    }

    private static byte[] concatenate(byte[]... values) {
        int length = 0;
        for (byte[] value : values) length = Math.addExact(length, value.length);
        byte[] result = new byte[length];
        int offset = 0;
        for (byte[] value : values) {
            System.arraycopy(value, 0, result, offset, value.length);
            offset += value.length;
        }
        return result;
    }

    private static byte[] exactInternal(byte[] value, int length) {
        if (value == null || value.length != length) throw new IllegalArgumentException();
        return value;
    }

    private static boolean starts(byte[] value, String prefix) {
        return starts(value, prefix.getBytes(StandardCharsets.UTF_8));
    }

    private static boolean starts(byte[] value, byte[] prefix) {
        return value.length >= prefix.length
            && MessageDigest.isEqual(Arrays.copyOf(value, prefix.length), prefix);
    }

    private static String validateOutcome(ObjectNode outcome) {
        String kind = text(outcome, "kind");
        switch (kind) {
            case "completed" -> {
                requireFields(outcome, "kind", "code", "response");
                requiredI32(outcome.get("code"));
                responseBytes(outcome);
            }
            case "legacy_completed" -> {
                requireFields(outcome, "kind", "code", "values");
                requiredI32(outcome.get("code"));
                JsonNode values = outcome.get("values");
                if (values == null || !values.isArray() || values.size() > MAX_LEGACY_VALUES) throw decodeFailure();
                values.forEach(ProgramsClient::validateLegacyValue);
            }
            case "refused" -> {
                requireFields(outcome, "kind", "failure");
                validateFailure(object(outcome, "failure"));
            }
            default -> throw decodeFailure();
        }
        return kind;
    }

    private static void validateLegacyValue(JsonNode value) {
        if (!(value instanceof ObjectNode object)) throw decodeFailure();
        requireFields(object, "type", "value");
        switch (text(object, "type")) {
            case "i32" -> requiredI32(object.get("value"));
            case "i64" -> signedI64(object.get("value"));
            default -> throw decodeFailure();
        }
    }

    private static void validateFailure(ObjectNode failure) {
        String kind = text(failure, "kind");
        switch (kind) {
            case "unknown_program", "reentrancy", "authority", "resource", "response", "fault" ->
                requireFields(failure, "kind");
            case "depth_exceeded", "fanout_exceeded" -> {
                requireFields(failure, "kind", "limit", "attempted");
                requiredU32(failure.get("limit"));
                requiredU32(failure.get("attempted"));
            }
            case "guest_refused" -> {
                requireFields(failure, "kind", "code");
                requiredI32(failure.get("code"));
            }
            default -> throw decodeFailure();
        }
    }

    private static ObjectNode encode(Call call) {
        ObjectNode value = object().put("program_id", hex(call.programId()))
            .put("calldata", hex(call.calldata())).put("signed_activity", hex(call.signedActivity()));
        value.set("budget", object().put("fuel", call.budget().fuel().toString())
            .put("fee_limit", call.budget().feeLimit().toString()));
        if (call.nativeCall() != null) {
            return object().put("payload", hex(call.nativeCall().encode())).put("signed_activity", hex(call.signedActivity()));
        }
        ArrayNode capabilities = value.putArray("capabilities");
        call.capabilities().forEach(item -> capabilities.add(item.wire()));
        return value;
    }

    private static byte[] fixedUnsigned(BigInteger value, int length) {
        byte[] raw = value.toByteArray();
        byte[] fixed = new byte[length];
        int copy = Math.min(raw.length, length);
        System.arraycopy(raw, raw.length - copy, fixed, length - copy, copy);
        return fixed;
    }

    private static BigInteger durationMillis(Duration value) {
        try {
            long millis = value.toMillis();
            if (millis <= 0) invalid();
            return BigInteger.valueOf(millis);
        } catch (ArithmeticException error) {
            invalid();
            throw new AssertionError();
        }
    }

    private static void requireSequencerSigned(String value) {
        if (!SEQUENCER_SIGNED.equals(value)) invalid();
    }

    private static void requireFields(ObjectNode value, String... fields) {
        if (value == null || value.size() != fields.length) throw decodeFailure();
        for (String field : fields) if (!value.has(field)) throw decodeFailure();
    }

    private static ObjectNode object(JsonNode parent, String field) {
        JsonNode value = parent == null ? null : parent.get(field);
        if (!(value instanceof ObjectNode object)) throw decodeFailure();
        return object;
    }

    private static byte[] responseBytes(JsonNode outcome) {
        JsonNode response = outcome.get("response");
        if (response == null || !response.isTextual()) throw decodeFailure();
        return boundedHex(response.textValue(), true);
    }

    private static String text(JsonNode parent, String field) {
        JsonNode value = parent == null ? null : parent.get(field);
        if (value == null || !value.isTextual() || value.textValue().isEmpty()) throw decodeFailure();
        return value.textValue();
    }

    private static int requiredU16(JsonNode value) {
        BigInteger integer = jsonInteger(value);
        if (integer.signum() < 0 || integer.bitLength() > 16) throw decodeFailure();
        return integer.intValue();
    }

    private static long requiredU32(JsonNode value) {
        BigInteger integer = jsonInteger(value);
        if (integer.signum() < 0 || integer.compareTo(MAX_U32) > 0) throw decodeFailure();
        return integer.longValue();
    }

    private static int requiredI32(JsonNode value) {
        BigInteger integer = jsonInteger(value);
        if (integer.compareTo(BigInteger.valueOf(Integer.MIN_VALUE)) < 0
                || integer.compareTo(BigInteger.valueOf(Integer.MAX_VALUE)) > 0) throw decodeFailure();
        return integer.intValue();
    }

    private static BigInteger jsonInteger(JsonNode value) {
        if (value == null || !value.isIntegralNumber()) throw decodeFailure();
        return value.bigIntegerValue();
    }

    private static BigInteger unsigned(JsonNode value, int bits) {
        if (value == null || !value.isTextual()) throw decodeFailure();
        String text = value.textValue();
        if (text.isEmpty() || text.length() > 1 && text.charAt(0) == '0'
                || !text.chars().allMatch(character -> character >= '0' && character <= '9')) {
            throw decodeFailure();
        }
        try {
            BigInteger parsed = new BigInteger(text);
            if (parsed.signum() < 0 || parsed.bitLength() > bits) throw decodeFailure();
            return parsed;
        } catch (NumberFormatException error) {
            throw decodeFailure();
        }
    }

    private static long signedI64(JsonNode value) {
        if (value == null || !value.isTextual()) throw decodeFailure();
        String text = value.textValue();
        if (text.isEmpty() || "-0".equals(text) || text.charAt(0) == '0' && text.length() > 1
                || text.charAt(0) == '-' && (text.length() == 1
                    || text.charAt(1) == '0' && text.length() > 2)) throw decodeFailure();
        try {
            return Long.parseLong(text);
        } catch (NumberFormatException error) {
            throw decodeFailure();
        }
    }

    private static byte[] boundedHex(String value, boolean emptyAllowed) {
        if (value == null || (value.isEmpty() && !emptyAllowed) || value.length() > MAX_CALLDATA_BYTES * 2
                || (value.length() & 1) != 0 || !canonicalLowerHex(value, value.length() / 2)) {
            throw decodeFailure();
        }
        try {
            return HexFormat.of().parseHex(value);
        } catch (IllegalArgumentException error) {
            throw decodeFailure();
        }
    }

    private static byte[] hex32(String value) {
        if (!canonicalLowerHex(value, 32)) throw decodeFailure();
        return HexFormat.of().parseHex(value);
    }

    private static boolean canonicalLowerHex(String value, int bytes) {
        if (value == null || value.length() != bytes * 2) return false;
        for (int index = 0; index < value.length(); index++) {
            char current = value.charAt(index);
            if (!(current >= '0' && current <= '9' || current >= 'a' && current <= 'f')) return false;
        }
        return true;
    }

    private static boolean allZero(byte[] value) {
        int aggregate = 0;
        for (byte current : value) aggregate |= current;
        return aggregate == 0;
    }

    private static byte[] exactArgument(byte[] value, int length) {
        if (value == null || value.length != length) invalid();
        return value.clone();
    }

    private static byte[] exactVerification(byte[] value, int length) {
        if (value == null || value.length != length) invalidVerification();
        return value.clone();
    }

    private static ObjectNode object() { return JsonNodeFactory.instance.objectNode(); }

    private static ObjectNode copy(ObjectNode value) {
        if (value == null) throw decodeFailure();
        return value.deepCopy();
    }

    private static String hex(byte[] value) { return HexFormat.of().formatHex(value); }

    private static byte[] sha256(byte[]... values) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            for (byte[] value : values) digest.update(value);
            return digest.digest();
        } catch (java.security.GeneralSecurityException impossible) {
            throw new AssertionError(impossible);
        }
    }

    private static boolean verifyEd25519(byte[] publicKey, byte[] signature, byte[] digest) {
        try {
            byte[] encoded = new byte[ED25519_X509_PREFIX.length + publicKey.length];
            System.arraycopy(ED25519_X509_PREFIX, 0, encoded, 0, ED25519_X509_PREFIX.length);
            System.arraycopy(publicKey, 0, encoded, ED25519_X509_PREFIX.length, publicKey.length);
            var key = KeyFactory.getInstance("Ed25519").generatePublic(new X509EncodedKeySpec(encoded));
            var verifier = Signature.getInstance("Ed25519");
            verifier.initVerify(key);
            verifier.update(digest);
            return verifier.verify(signature);
        } catch (java.security.GeneralSecurityException | RuntimeException error) {
            return false;
        }
    }

    private static PlatformSdkException decodeFailure() {
        return new PlatformSdkException(PlatformSdkException.Code.DECODE_FAILURE,
            PlatformSdkException.Retry.NEVER, null, null, null);
    }

    private static void invalid() { throw PlatformSdkException.invalidArgument(); }

    private static void invalidVerification() { throw PlatformSdkException.verificationFailure(); }

    private static PlatformSdkException unknownOutcome() {
        return new PlatformSdkException(PlatformSdkException.Code.UNKNOWN_OUTCOME,
            PlatformSdkException.Retry.UNKNOWN_OUTCOME, null, null, null);
    }

    private static final class BinaryCursor {
        private final byte[] bytes;
        private int offset;
        private BinaryCursor(byte[] bytes) { this.bytes = bytes; }
        private int u8() { return take(1)[0] & 0xff; }
        private int u16() { return integer(2).intValue(); }
        private long u32() { return integer(4).longValue(); }
        private BigInteger u64() { return integer(8); }
        private BigInteger integer(int length) { return new BigInteger(1, take(length)); }
        private void tag(int expected) { if (u8() != expected) throw new IllegalArgumentException(); }
        private byte[] bounded(int maximum, boolean emptyAllowed) {
            long declared = u32();
            if (declared > maximum || declared > Integer.MAX_VALUE || declared == 0 && !emptyAllowed) {
                throw new IllegalArgumentException();
            }
            return take((int) declared);
        }
        private byte[] take(int length) {
            if (length < 0 || offset > bytes.length - length) throw new IllegalArgumentException();
            byte[] value = Arrays.copyOfRange(bytes, offset, offset + length);
            offset += length;
            return value;
        }
        private void finish() { if (offset != bytes.length) throw new IllegalArgumentException(); }
    }

    private static final class TerminalCursor {
        private final byte[] bytes;
        private int offset;
        private TerminalCursor(byte[] bytes, int offset) {
            this.bytes = bytes;
            this.offset = offset;
            if (offset < 0 || offset > bytes.length) throw new IllegalArgumentException();
        }
        private int u8() { return take(1)[0] & 0xff; }
        private int u16() { return integer(2).intValue(); }
        private long u32() { return integer(4).longValue(); }
        private BigInteger u64() { return integer(8); }
        private int i32() { return ByteBuffer.wrap(take(4)).getInt(); }
        private long i64() { return ByteBuffer.wrap(take(8)).getLong(); }
        private BigInteger integer(int length) { return new BigInteger(1, take(length)); }
        private byte[] sized32() {
            return sized32(Integer.MAX_VALUE);
        }
        private byte[] sized32(int maximum) {
            long length = u32();
            if (length > maximum || length > Integer.MAX_VALUE) throw new IllegalArgumentException();
            return take((int) length);
        }
        private byte[] sized64() {
            BigInteger length = u64();
            if (length.bitLength() > 31) throw new IllegalArgumentException();
            return take(length.intValue());
        }
        private byte[] take(int length) {
            if (length < 0 || offset > bytes.length - length) throw new IllegalArgumentException();
            byte[] value = Arrays.copyOfRange(bytes, offset, offset + length);
            offset += length;
            return value;
        }
        private byte[] rest() { return take(bytes.length - offset); }
        private void finish() { if (offset != bytes.length) throw new IllegalArgumentException(); }
    }
}
