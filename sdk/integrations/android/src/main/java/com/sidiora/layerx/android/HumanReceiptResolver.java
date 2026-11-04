package com.sidiora.layerx.android;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.KeyFactory;
import java.security.MessageDigest;
import java.security.Signature;
import java.security.spec.X509EncodedKeySpec;
import java.util.Arrays;
import java.util.Base64;
import java.util.HashSet;
import java.util.HexFormat;
import java.util.Set;
import org.bouncycastle.crypto.digests.KeccakDigest;

/** Decodes Human EvidenceMaterial; service verification labels never establish device trust. */
final class HumanReceiptResolver {
    private HumanReceiptResolver() {}

    record Payment(byte[] asset, byte[] recipient, BigInteger amount, String digest) {}

    private static final ObjectMapper JSON = new ObjectMapper()
        .enable(JsonParser.Feature.STRICT_DUPLICATE_DETECTION)
        .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);

    static Payment verify(JsonNode material, String id, String verification, LocalVerifier.ReceiptKeys keys) {
        try {
            require(material != null && material.isObject());
            require(id.equals(text(material, "evidence_id")));
            require("layerx-receipt".equals(text(material, "class")));
            require(verification.equals("receipt-verified") || verification.equals("checkpoint-finalised"));
            require(verification.equals(text(material, "verification")));
            String encoded = text(material, "bytes_base64");
            require(!encoded.isEmpty() && encoded.length() <= 2_000_000);
            byte[] bytes = Base64.getDecoder().decode(encoded);
            require(bytes.length > 0 && bytes.length <= 1_500_000);
            require(Base64.getEncoder().encodeToString(bytes).equals(encoded));
            String type = text(material, "content_type");
            JsonNode proof = null;
            byte[] signed;
            if (type.equals("application/vnd.layerx.receipt-proof-v2+json")) {
                proof = JSON.readTree(bytes);
                Set<String> fields = new HashSet<>(Set.of("format", "verificationLevel", "signedReceipt",
                    "receiptDigest", "receiptKey", "inclusionHeight"));
                if (proof.has("rawTransaction")) fields.add("rawTransaction");
                shape(proof, fields);
                require("layerx-receipt-proof-v2".equals(text(proof, "format")));
                require("receipted".equals(text(proof, "verificationLevel")));
                signed = hex(proof, "signedReceipt", -1);
            } else {
                require(type.equals("application/vnd.layerx.receipt"));
                signed = bytes;
            }
            require(signed.length <= 1_048_576 && id.equals(ReceiptGate.evidenceId(signed)));
            Reader wire = new Reader(signed);
            int version = wire.integer(2).intValueExact();
            require(version == 2 || version == 3);
            require(wire.integer(2).intValueExact() == 0x5201);
            require(wire.integer(2).intValueExact() == version);
            byte[] activity = wire.blob(32);
            BigInteger sequence = wire.integer(8);
            byte[] previous = wire.blob(32);
            byte[] resulting = wire.blob(32);
            wire.blob(32); // activity root
            require(wire.integer(4).signum() == 0); // successful outcome
            int count = wire.integer(4).intValueExact();
            require(count <= 512);
            for (int index = 0; index < count; index++) {
                wire.fixed(6); // module, ordinal, event type
                int kind = wire.integer(1).intValueExact();
                int monetary = wire.integer(1).intValueExact();
                require(kind >= 1 && kind <= 3 && monetary <= 1 && (monetary == 0 || kind == 2));
                wire.blob(32);
                wire.bounded(256);
            }
            wire.fixed(16); // fee
            byte[] batch = wire.blob(32);
            require(Arrays.equals(Arrays.copyOf(batch, 24), new byte[24]));
            BigInteger height = new BigInteger(1, Arrays.copyOfRange(batch, 24, 32));
            require(wire.integer(2).intValueExact() > 11); // current execution modules
            require(wire.integer(4).signum() > 0); // module version
            wire.fixed(5); // parameter version and operation
            byte[] asset = wire.blob(32);
            BigInteger amount = wire.integer(16);
            wire.blob(32); // sender
            wire.fixed(40); // balances and sender sequence
            byte[] recipient = wire.blob(32);
            wire.fixed(32); // recipient balances
            wire.blob(32); // transfer root
            wire.blob(32); // authorization
            byte[] keyId = wire.blob(32);
            require(wire.integer(8).signum() > 0);
            int signatureOffset = wire.position();
            require(wire.integer(1).intValueExact() == 1);
            byte[] signature = wire.blob(64);
            require(wire.remaining() == 0 && sequence.signum() > 0);
            require(!Arrays.equals(activity, new byte[32]));
            LocalVerifier.ReceiptKey pin = keys.entries().stream()
                .filter(key -> MessageDigest.isEqual(key.keyId(), keyId)).findFirst().orElseThrow();
            require(height.compareTo(pin.firstHeight()) >= 0 && height.compareTo(pin.lastHeight()) <= 0);
            byte[] unsigned = Arrays.copyOf(signed, signatureOffset + 1);
            unsigned[signatureOffset] = 0;
            byte[] digest = sha256("LXP/v1/receipt\0".getBytes(StandardCharsets.UTF_8), unsigned);
            Signature verifier = Signature.getInstance("Ed25519");
            byte[] x509 = new byte[44];
            System.arraycopy(HexFormat.of().parseHex("302a300506032b6570032100"), 0, x509, 0, 12);
            System.arraycopy(pin.publicKey(), 0, x509, 12, 32);
            verifier.initVerify(KeyFactory.getInstance("Ed25519").generatePublic(new X509EncodedKeySpec(x509)));
            verifier.update(digest);
            require(verifier.verify(signature));
            byte[] sequenceBytes = new byte[8];
            byte[] number = sequence.toByteArray();
            System.arraycopy(number, Math.max(0, number.length - 8), sequenceBytes,
                Math.max(0, 8 - number.length), Math.min(8, number.length));
            require(MessageDigest.isEqual(resulting, sha256("PAXEER/v1/sequence\0".getBytes(StandardCharsets.UTF_8),
                previous, sequenceBytes, activity)));
            if (proof != null) {
                JsonNode key = proof.required("receiptKey");
                shape(key, Set.of("keyId", "publicKey", "firstHeight", "lastHeight"));
                require(MessageDigest.isEqual(pin.keyId(), hex(key, "keyId", 32)));
                require(MessageDigest.isEqual(pin.publicKey(), hex(key, "publicKey", 32)));
                require(pin.firstHeight().equals(quantity(key, "firstHeight")));
                require(pin.lastHeight().equals(quantity(key, "lastHeight")));
                require(MessageDigest.isEqual(digest, hex(proof, "receiptDigest", 32)));
                BigInteger inclusion = quantity(proof, "inclusionHeight");
                require(inclusion.compareTo(height) >= 0);
                if (material.has("inclusion_height")) {
                    require(material.path("inclusion_height").isIntegralNumber());
                    require(inclusion.equals(material.path("inclusion_height").bigIntegerValue()));
                }
                if (proof.has("rawTransaction")) {
                    byte[] raw = hex(proof, "rawTransaction", -1);
                    KeccakDigest keccak = new KeccakDigest(256);
                    keccak.update(raw, 0, raw.length);
                    byte[] transactionHash = new byte[32];
                    keccak.doFinal(transactionHash, 0);
                    require(MessageDigest.isEqual(activity, transactionHash));
                }
            }
            if (material.has("rung")) {
                require((verification.equals("checkpoint-finalised") ? "committed" : "receipted")
                    .equals(text(material, "rung")));
            }
            return new Payment(asset, recipient, amount, PublishableConfiguration.hexadecimal(digest));
        } catch (Exception error) {
            throw MobileIntegrationException.of(MobileIntegrationException.Code.VERIFICATION_FAILURE);
        }
    }

    private static void shape(JsonNode value, Set<String> expected) {
        require(value != null && value.isObject());
        Set<String> actual = new HashSet<>();
        value.fieldNames().forEachRemaining(actual::add);
        require(actual.equals(expected));
    }

    private static String text(JsonNode value, String field) {
        require(value.path(field).isTextual());
        return value.path(field).textValue();
    }

    private static BigInteger quantity(JsonNode value, String field) {
        String text = text(value, field);
        require(text.length() <= 18 && text.matches("0x(?:0|[1-9a-f][0-9a-f]*)"));
        return new BigInteger(text.substring(2), 16);
    }

    private static byte[] hex(JsonNode value, String field, int length) {
        String text = text(value, field);
        require(text.length() >= 4 && text.length() <= 2_097_154
            && text.startsWith("0x") && text.length() % 2 == 0);
        for (int index = 2; index < text.length(); index++) {
            char digit = text.charAt(index);
            require(digit >= '0' && digit <= '9' || digit >= 'a' && digit <= 'f');
        }
        byte[] bytes = HexFormat.of().parseHex(text.substring(2));
        require(length < 0 || bytes.length == length);
        return bytes;
    }

    private static byte[] sha256(byte[]... values) throws Exception {
        MessageDigest hash = MessageDigest.getInstance("SHA-256");
        for (byte[] value : values) hash.update(value);
        return hash.digest();
    }

    private static void require(boolean valid) {
        if (!valid) throw MobileIntegrationException.of(MobileIntegrationException.Code.VERIFICATION_FAILURE);
    }

    private static final class Reader {
        private final ByteBuffer bytes;
        Reader(byte[] bytes) { this.bytes = ByteBuffer.wrap(bytes); }
        int position() { return bytes.position(); }
        int remaining() { return bytes.remaining(); }
        byte[] fixed(int count) {
            require(count >= 0 && count <= remaining());
            byte[] result = new byte[count];
            bytes.get(result);
            return result;
        }
        BigInteger integer(int count) { return new BigInteger(1, fixed(count)); }
        byte[] blob(int count) {
            require(integer(4).intValueExact() == count);
            return fixed(count);
        }
        byte[] bounded(int maximum) {
            int count = integer(4).intValueExact();
            require(count <= maximum);
            return fixed(count);
        }
    }
}
