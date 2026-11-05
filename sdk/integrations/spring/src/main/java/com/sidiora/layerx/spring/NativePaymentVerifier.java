package com.sidiora.layerx.spring;

import com.fasterxml.jackson.databind.JsonNode;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import org.bouncycastle.crypto.digests.KeccakDigest;

final class NativePaymentVerifier {
    private NativePaymentVerifier() {}

    private static final byte[] LEDGER = java.util.HexFormat.of()
        .parseHex("000000000000000000000000000000000000101a");
    private static final byte[] EXACT_SELECTOR = Arrays.copyOf(hash(
        "payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)"), 4);
    private static final byte[] EXACT_TOPIC = hash("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)");
    private static final BigInteger SECP256K1_ORDER = new BigInteger(
        "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16);

    static byte[] receiptDigest(byte[] signedReceipt) {
        KeccakDigest digest = new KeccakDigest(256);
        digest.update(signedReceipt, 0, signedReceipt.length);
        byte[] result = new byte[32];
        digest.doFinal(result, 0);
        return result;
    }

    private static byte[] hash(String value) {
        return receiptDigest(value.getBytes(StandardCharsets.US_ASCII));
    }

    static void verify(LocalVerifier.SequencerReceiptVerification verified, byte[] rawTransaction,
                       X402.PaymentRequirements requirements) {
        require(X402.NATIVE_NETWORK.equals(requirements.network())
            && List.of("exact", "metered", "subscription").contains(requirements.scheme()));
        JsonNode terms = null;
        if (requirements.extra() != null) {
            require(requirements.extra().isObject());
            terms = requirements.extra().get("layerx");
            if (terms != null) {
                require(terms.isObject());
                JsonNode commitment = terms.get("commitment");
                require(commitment != null && commitment.isTextual()
                    && "receipted".equals(commitment.textValue()));
            }
        }
        byte[] purpose = term(terms, "purposeHash", !"exact".equals(requirements.scheme()));
        byte[] payer = term(terms, "payer", false);
        MiddlewareException.Code code = MiddlewareException.Code.VERIFICATION_FAILURE;
        byte[] asset = X402.parseHex32(requirements.asset(), code);
        byte[] payee = X402.parseHex32(requirements.payTo(), code);
        BigInteger amount = new BigInteger(X402.parseAmount(
            com.fasterxml.jackson.databind.node.TextNode.valueOf(requirements.amount())));

        List<Rlp> fields = transaction(rawTransaction);
        require(number(fields.get(0), 256).equals(BigInteger.valueOf(125)));
        require(Arrays.equals(fields.get(5).value(), LEDGER)
            && number(fields.get(6), 256).signum() == 0);
        byte[] data = fields.get(7).value();
        require(data.length == 196 && Arrays.equals(Arrays.copyOf(data, 4), EXACT_SELECTOR));
        byte[] from = word(data, 0);
        byte[] reference = word(data, 5);
        require(Arrays.equals(word(data, 1), payee) && Arrays.equals(word(data, 2), asset)
            && new BigInteger(1, word(data, 3)).equals(amount)
            && new BigInteger(1, word(data, 4)).bitLength() <= 64
            && (payer == null || Arrays.equals(from, payer))
            && (purpose == null || Arrays.equals(reference, purpose)));

        LocalVerifier.ProtocolReceipt receipt = verified.receipt();
        require(receipt.resultCode() == 0 && receipt.operation() == 2 && receipt.amount().signum() == 0
            && Arrays.equals(receipt.to(), paddedLedger()));
        byte[] eventData = new byte[64];
        System.arraycopy(asset, 0, eventData, 0, 32);
        System.arraycopy(word(data, 3), 0, eventData, 32, 32);
        byte[] expectedDataHash = X402.sha256(eventData);
        int payments = 0;
        for (int index = 1; index < receipt.effects().size(); index++) {
            LocalVerifier.ReceiptEffect effect = receipt.effects().get(index);
            byte[] body = effect.body();
            require(effect.moduleId() == receipt.moduleId() && effect.ordinal() == index
                && effect.eventType() == 0 && effect.kind() == 3 && !effect.monetary());
            if (!Arrays.equals(Arrays.copyOfRange(body, 0, 20), LEDGER)
                    || !Arrays.equals(Arrays.copyOfRange(body, 21, 53), EXACT_TOPIC)) continue;
            require(body.length == 181 && body[20] == 4
                && Arrays.equals(Arrays.copyOfRange(body, 53, 85), reference)
                && Arrays.equals(Arrays.copyOfRange(body, 85, 117), from)
                && Arrays.equals(Arrays.copyOfRange(body, 117, 149), payee)
                && Arrays.equals(Arrays.copyOfRange(body, 149, 181), expectedDataHash));
            payments++;
        }
        require(payments == 1);
    }

    private static byte[] paddedLedger() {
        byte[] result = new byte[32];
        System.arraycopy(LEDGER, 0, result, 12, 20);
        return result;
    }

    private static byte[] word(byte[] data, int index) {
        return Arrays.copyOfRange(data, 4 + index * 32, 36 + index * 32);
    }

    private static byte[] term(JsonNode terms, String name, boolean required) {
        JsonNode value = terms == null ? null : terms.get(name);
        if (value == null && !required) return null;
        require(value != null && value.isTextual() && X402.isLowerHex32(value.textValue())
            && !value.textValue().equals("0".repeat(64)));
        return X402.parseHex32(value.textValue(), MiddlewareException.Code.VERIFICATION_FAILURE);
    }

    private static List<Rlp> transaction(byte[] raw) {
        require(raw != null && raw.length > 1 && raw[0] == 2 && raw.length <= X402.MAX_HEADER_BYTES);
        RlpReader reader = new RlpReader(raw);
        Rlp root = reader.read(raw.length, 0);
        require(reader.offset == raw.length);
        List<Rlp> fields = root.items();
        require(fields.size() == 12);
        number(fields.get(1), 64);
        BigInteger priority = number(fields.get(2), 256);
        require(priority.compareTo(number(fields.get(3), 256)) <= 0);
        number(fields.get(4), 64);
        fields.get(5).value();
        fields.get(7).value();
        for (Rlp access : fields.get(8).items()) {
            List<Rlp> entry = access.items();
            require(entry.size() == 2 && entry.get(0).value().length == 20);
            for (Rlp key : entry.get(1).items()) require(key.value().length == 32);
        }
        require(number(fields.get(9), 1).compareTo(BigInteger.ONE) <= 0);
        BigInteger r = number(fields.get(10), 256), s = number(fields.get(11), 256);
        require(r.signum() > 0 && r.compareTo(SECP256K1_ORDER) < 0
            && s.signum() > 0 && s.compareTo(SECP256K1_ORDER.shiftRight(1)) <= 0);
        return fields;
    }

    private static BigInteger number(Rlp node, int bits) {
        byte[] bytes = node.value();
        require(bytes.length <= (bits + 7) / 8 && (bytes.length == 0 || bytes[0] != 0));
        BigInteger value = new BigInteger(1, bytes);
        require(value.bitLength() <= bits);
        return value;
    }

    private record Rlp(byte[] bytes, List<Rlp> children) {
        byte[] value() { require(bytes != null); return bytes; }
        List<Rlp> items() { require(children != null); return children; }
    }

    private static final class RlpReader {
        private final byte[] raw;
        private int offset = 1;

        RlpReader(byte[] raw) { this.raw = raw; }

        Rlp read(int limit, int depth) {
            require(offset < limit && depth <= 4);
            int prefix = raw[offset++] & 0xff;
            if (prefix < 128) return new Rlp(new byte[]{(byte) prefix}, null);
            boolean list = prefix >= 192;
            int length = prefix - (list ? 192 : 128);
            if (length > 55) {
                int width = length - 55;
                require(width <= 4 && width <= limit - offset && raw[offset] != 0);
                long count = 0;
                for (int index = 0; index < width; index++) count = count * 256 + (raw[offset++] & 0xff);
                require(count >= 56 && count <= limit - offset);
                length = (int) count;
            }
            require(length <= limit - offset);
            int end = offset + length;
            if (!list) {
                require(length != 1 || (raw[offset] & 0xff) >= 128);
                byte[] value = Arrays.copyOfRange(raw, offset, end);
                offset = end;
                return new Rlp(value, null);
            }
            List<Rlp> children = new ArrayList<>();
            while (offset < end) children.add(read(end, depth + 1));
            return new Rlp(null, List.copyOf(children));
        }
    }

    private static void require(boolean condition) {
        if (!condition) throw MiddlewareException.of(MiddlewareException.Code.VERIFICATION_FAILURE);
    }
}
