package com.sidiora.layerx.sdk.examples;

import com.fasterxml.jackson.databind.JsonNode;
import com.sidiora.layerx.sdk.PlatformSdkException;
import com.sidiora.layerx.sdk.ProgramsClient;
import com.sidiora.layerx.sdk.SecretBytes;
import java.io.ByteArrayOutputStream;
import java.math.BigInteger;
import java.util.Arrays;
import java.util.HexFormat;
import org.bouncycastle.asn1.sec.SECNamedCurves;
import org.bouncycastle.crypto.digests.SHA256Digest;
import org.bouncycastle.crypto.params.ECDomainParameters;
import org.bouncycastle.crypto.params.ECPrivateKeyParameters;
import org.bouncycastle.crypto.signers.ECDSASigner;
import org.bouncycastle.crypto.signers.HMacDSAKCalculator;

/** Typed native-transfer signer for this example: chain 125 and an empty access list. */
public final class Eip1559Payment {
    private static final org.bouncycastle.asn1.x9.X9ECParameters CURVE = SECNamedCurves.getByName("secp256k1");
    private static final ECDomainParameters DOMAIN = new ECDomainParameters(
        CURVE.getCurve(), CURVE.getG(), CURVE.getN(), CURVE.getH());

    private Eip1559Payment() {}

    public record Payment(String recipient, BigInteger amount, BigInteger maximumFee, BigInteger priorityFee) {
        public Payment {
            if (recipient == null || !recipient.matches("0x[0-9a-f]{40}")
                    || recipient.equals("0x" + "00".repeat(20))) throw PlatformSdkException.invalidArgument();
            unsigned(amount, 256);
            unsigned(maximumFee, 256);
            unsigned(priorityFee, 256);
            if (amount.signum() == 0 || maximumFee.compareTo(priorityFee) < 0) {
                throw PlatformSdkException.invalidArgument();
            }
        }
    }

    public record Transaction(Payment payment, BigInteger nonce, BigInteger gas) {
        public Transaction {
            java.util.Objects.requireNonNull(payment, "payment");
            unsigned(nonce, 64);
            unsigned(gas, 64);
            if (gas.compareTo(BigInteger.valueOf(21_000)) < 0) throw PlatformSdkException.invalidArgument();
        }
    }

    public static BigInteger quantity(JsonNode value) {
        if (value == null || !value.isTextual() || !value.textValue().matches("0x(?:0|[1-9a-f][0-9a-f]*)")) {
            throw PlatformSdkException.invalidArgument();
        }
        var result = new BigInteger(value.textValue().substring(2), 16);
        unsigned(result, 256);
        return result;
    }

    public static String sender(SecretBytes privateKey) {
        return privateKey.use(bytes -> {
            byte[] point = CURVE.getG().multiply(secret(bytes)).normalize().getEncoded(false);
            byte[] hash = ProgramsClient.Programs.transactionHash(Arrays.copyOfRange(point, 1, point.length));
            return "0x" + HexFormat.of().formatHex(Arrays.copyOfRange(hash, 12, 32));
        });
    }

    public static byte[] sign(Transaction transaction, SecretBytes privateKey) {
        return privateKey.use(bytes -> {
            BigInteger key = secret(bytes);
            byte[][] fields = fields(transaction);
            byte[] digest = ProgramsClient.Programs.transactionHash(typed(list(fields)));
            var signer = new ECDSASigner(new HMacDSAKCalculator(new SHA256Digest()));
            signer.init(true, new ECPrivateKeyParameters(key, DOMAIN));
            BigInteger[] signature = signer.generateSignature(digest);
            BigInteger r = signature[0];
            BigInteger s = signature[1].min(CURVE.getN().subtract(signature[1]));
            var publicKey = CURVE.getG().multiply(key).normalize();
            int parity = -1;
            for (int candidate = 0; candidate < 2; candidate++) {
                byte[] compressed = new byte[33];
                compressed[0] = (byte) (2 + candidate);
                byte[] x = magnitude(r);
                System.arraycopy(x, 0, compressed, 33 - x.length, x.length);
                var point = CURVE.getCurve().decodePoint(compressed);
                var recovered = point.multiply(s).subtract(CURVE.getG().multiply(new BigInteger(1, digest)))
                    .multiply(r.modInverse(CURVE.getN())).normalize();
                if (recovered.equals(publicKey)) parity = candidate;
            }
            if (parity < 0) throw PlatformSdkException.verificationFailure();
            byte[][] signed = Arrays.copyOf(fields, 12);
            signed[9] = scalar(BigInteger.valueOf(parity));
            signed[10] = scalar(r);
            signed[11] = scalar(s);
            return typed(list(signed));
        });
    }

    private static byte[][] fields(Transaction transaction) {
        Payment payment = transaction.payment();
        return new byte[][] {scalar(BigInteger.valueOf(125)), scalar(transaction.nonce()),
            scalar(payment.priorityFee()), scalar(payment.maximumFee()), scalar(transaction.gas()),
            data(HexFormat.of().parseHex(payment.recipient().substring(2))), scalar(payment.amount()),
            data(new byte[0]), list(new byte[0][])};
    }

    private static BigInteger secret(byte[] bytes) {
        if (bytes.length != 32) throw PlatformSdkException.invalidArgument();
        var value = new BigInteger(1, bytes);
        if (value.signum() == 0 || value.compareTo(CURVE.getN()) >= 0) throw PlatformSdkException.invalidArgument();
        return value;
    }

    private static void unsigned(BigInteger value, int bits) {
        if (value == null || value.signum() < 0 || value.bitLength() > bits) throw PlatformSdkException.invalidArgument();
    }

    private static byte[] magnitude(BigInteger value) {
        if (value.signum() == 0) return new byte[0];
        byte[] bytes = value.toByteArray();
        return bytes[0] == 0 ? Arrays.copyOfRange(bytes, 1, bytes.length) : bytes;
    }

    private static byte[] scalar(BigInteger value) { return data(magnitude(value)); }

    private static byte[] data(byte[] bytes) {
        if (bytes.length == 1 && Byte.toUnsignedInt(bytes[0]) < 128) return bytes;
        return prefix(bytes, 0x80, 0xb7);
    }

    private static byte[] list(byte[][] fields) {
        var body = new ByteArrayOutputStream();
        for (byte[] field : fields) body.writeBytes(field);
        return prefix(body.toByteArray(), 0xc0, 0xf7);
    }

    private static byte[] prefix(byte[] body, int shortOffset, int longOffset) {
        var output = new ByteArrayOutputStream();
        if (body.length <= 55) {
            output.write(shortOffset + body.length);
        } else {
            byte[] length = magnitude(BigInteger.valueOf(body.length));
            output.write(longOffset + length.length);
            output.writeBytes(length);
        }
        output.writeBytes(body);
        return output.toByteArray();
    }

    private static byte[] typed(byte[] rlp) {
        byte[] result = new byte[rlp.length + 1];
        result[0] = 2;
        System.arraycopy(rlp, 0, result, 1, rlp.length);
        return result;
    }
}
