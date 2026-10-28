package com.sidiora.layerx.sdk.verify;

import com.fasterxml.jackson.core.JsonGenerator;
import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.databind.DeserializationContext;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonDeserializer;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.JsonSerializer;
import com.fasterxml.jackson.databind.MapperFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializerProvider;
import com.fasterxml.jackson.databind.json.JsonMapper;
import com.fasterxml.jackson.databind.module.SimpleModule;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.PlatformSdkException;
import java.io.IOException;
import java.math.BigInteger;
import java.security.MessageDigest;
import java.util.HashSet;
import java.util.HexFormat;
import java.util.List;
import java.util.Set;

public final class PortableReceipt {
    private PortableReceipt() {}

    public static final String FORMAT = "layerx-receipt-proof-v2";
    private static final int MAX_BUNDLE_BYTES = 16 * 1_048_576;
    private static final Set<String> RECEIPTED_FIELDS = Set.of("format", "verificationLevel",
        "rawTransaction", "signedReceipt", "receiptDigest", "receiptKey");
    private static final Set<String> COMMITTED_FIELDS = Set.of("format", "verificationLevel",
        "rawTransaction", "signedReceipt", "receiptDigest", "receiptKey", "blockHeader",
        "receiptInclusion", "commit", "validatorSet");
    private static final Set<String> RETIRED_FIELDS = Set.of("batchId", "asset",
        "previousStateRoot", "resultingStateRoot");
    private static final ObjectMapper JSON = mapper();

    public static final class Verification {
        private final byte[] bundle;
        private final LocalVerifier.SequencerReceiptVerification receipted;
        private final Committed.Verification committed;

        private Verification(byte[] bundle, LocalVerifier.SequencerReceiptVerification receipted,
                             Committed.Verification committed) {
            this.bundle = bundle.clone();
            this.receipted = receipted;
            this.committed = committed;
        }

        public String verificationLevel() { return committed == null ? "receipted" : "committed"; }
        public LocalVerifier.SequencerReceiptVerification receipted() { return receipted; }
        public Committed.Verification committed() { return committed; }
        public int status() { return receipted.status(); }
        public void requireSuccess() { receipted.requireSuccess(); }
    }

    public static byte[] export(byte[] rawTransaction, byte[] signedReceipt,
                                LocalVerifier.ReceiptKey receiptKey) {
        return encode(base(rawTransaction, signedReceipt, receiptKey));
    }

    public static byte[] export(byte[] rawTransaction, byte[] signedReceipt,
                                LocalVerifier.ReceiptKey receiptKey, Committed.Header blockHeader,
                                Committed.ReceiptInclusion receiptInclusion, Committed.Commit commit,
                                List<Committed.Validator> validatorSet) {
        ObjectNode bundle = base(rawTransaction, signedReceipt, receiptKey);
        bundle.put("verificationLevel", "committed");
        bundle.set("blockHeader", JSON.valueToTree(blockHeader));
        bundle.set("receiptInclusion", JSON.valueToTree(receiptInclusion));
        bundle.set("commit", JSON.valueToTree(commit));
        bundle.set("validatorSet", JSON.valueToTree(validatorSet));
        byte[] encoded = encode(bundle);
        verify(encoded, new LocalVerifier.ReceiptKeys(List.of(receiptKey)));
        return encoded;
    }

    public static byte[] export(Verification verified) {
        require(verified != null);
        return verified.bundle.clone();
    }

    public static Verification verify(byte[] bundle, LocalVerifier.ReceiptKeys receiptKeys) {
        try {
            require(bundle != null && bundle.length > 0 && bundle.length <= MAX_BUNDLE_BYTES);
            byte[] snapshot = bundle.clone();
            JsonNode document = JSON.readTree(snapshot);
            require(document != null && document.isObject() && receiptKeys != null);
            refuseRetiredFields(document);
            require(FORMAT.equals(text(document, "format")));
            String level = text(document, "verificationLevel");
            require(level.equals("receipted") || level.equals("committed"));
            shape(document, level.equals("receipted") ? RECEIPTED_FIELDS : COMMITTED_FIELDS);

            JsonNode carriedKey = document.required("receiptKey");
            shape(carriedKey, Set.of("keyId", "publicKey", "firstHeight", "lastHeight"));
            byte[] keyId = hex(carriedKey, "keyId", 32, 32);
            var pinned = receiptKeys.entries().stream()
                .filter(key -> MessageDigest.isEqual(key.keyId(), keyId))
                .findFirst().orElseThrow(PlatformSdkException::verificationFailure);
            require(MessageDigest.isEqual(pinned.publicKey(), hex(carriedKey, "publicKey", 32, 32))
                && pinned.firstHeight().equals(quantity(carriedKey, "firstHeight"))
                && pinned.lastHeight().equals(quantity(carriedKey, "lastHeight")));

            var receipted = LocalVerifier.verifySequencerReceipt(
                hex(document, "signedReceipt", 1, 1_048_576),
                hex(document, "rawTransaction", 1, 1_048_576), receiptKeys);
            require(MessageDigest.isEqual(receipted.receiptDigest(),
                hex(document, "receiptDigest", 32, 32)));
            require(MessageDigest.isEqual(receipted.receipt().contextHash(), pinned.keyId()));
            Committed.Verification committed = null;
            if (level.equals("committed")) {
                var header = JSON.treeToValue(document.required("blockHeader"), Committed.Header.class);
                var inclusion = JSON.treeToValue(document.required("receiptInclusion"),
                    Committed.ReceiptInclusion.class);
                var commit = JSON.treeToValue(document.required("commit"), Committed.Commit.class);
                JsonNode validators = document.required("validatorSet");
                require(validators.isArray() && !validators.isEmpty() && validators.size() <= 10000);
                List<Committed.Validator> validatorSet = JSON.readerForListOf(Committed.Validator.class)
                    .readValue(validators);
                committed = Committed.verify(receipted, header, inclusion, commit, validatorSet);
            }
            return new Verification(snapshot, receipted, committed);
        } catch (PlatformSdkException error) {
            throw error;
        } catch (IOException | RuntimeException error) {
            throw PlatformSdkException.verificationFailure();
        }
    }

    private static ObjectNode base(byte[] rawTransaction, byte[] signedReceipt,
                                   LocalVerifier.ReceiptKey receiptKey) {
        require(receiptKey != null);
        var verified = LocalVerifier.verifySequencerReceipt(signedReceipt, rawTransaction,
            new LocalVerifier.ReceiptKeys(List.of(receiptKey)));
        ObjectNode bundle = JSON.createObjectNode();
        bundle.put("format", FORMAT);
        bundle.put("verificationLevel", "receipted");
        bundle.put("rawTransaction", hex(rawTransaction));
        bundle.put("signedReceipt", hex(verified.canonicalBytes()));
        bundle.put("receiptDigest", hex(verified.receiptDigest()));
        ObjectNode key = bundle.putObject("receiptKey");
        key.put("keyId", hex(receiptKey.keyId()));
        key.put("publicKey", hex(receiptKey.publicKey()));
        key.put("firstHeight", "0x" + receiptKey.firstHeight().toString(16));
        key.put("lastHeight", "0x" + receiptKey.lastHeight().toString(16));
        return bundle;
    }

    private static ObjectMapper mapper() {
        var module = new SimpleModule();
        module.addSerializer(byte[].class, new JsonSerializer<byte[]>() {
            @Override public void serialize(byte[] value, JsonGenerator output,
                                             SerializerProvider provider) throws IOException {
                output.writeString(hex(value));
            }
        });
        module.addDeserializer(byte[].class, new JsonDeserializer<byte[]>() {
            @Override public byte[] deserialize(JsonParser input, DeserializationContext context)
                    throws IOException {
                require(input.currentToken() == com.fasterxml.jackson.core.JsonToken.VALUE_STRING);
                return decodeHex(input.getText(), 0, 1_048_576);
            }
        });
        return JsonMapper.builder().addModule(module)
            .enable(JsonParser.Feature.STRICT_DUPLICATE_DETECTION)
            .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS)
            .enable(DeserializationFeature.FAIL_ON_MISSING_CREATOR_PROPERTIES)
            .enable(DeserializationFeature.FAIL_ON_NULL_CREATOR_PROPERTIES)
            .disable(DeserializationFeature.ACCEPT_FLOAT_AS_INT)
            .disable(MapperFeature.ALLOW_COERCION_OF_SCALARS).build();
    }

    private static byte[] encode(JsonNode document) {
        try {
            byte[] encoded = JSON.writeValueAsBytes(document);
            require(encoded.length <= MAX_BUNDLE_BYTES);
            return encoded;
        } catch (IOException error) {
            throw PlatformSdkException.verificationFailure();
        }
    }

    private static void shape(JsonNode node, Set<String> fields) {
        require(node.isObject());
        var actual = new HashSet<String>();
        node.fieldNames().forEachRemaining(actual::add);
        require(actual.equals(fields));
    }

    private static void refuseRetiredFields(JsonNode node) {
        if (node.isObject()) {
            node.fieldNames().forEachRemaining(name -> require(!RETIRED_FIELDS.contains(name)));
        }
        if (node.isContainerNode()) node.forEach(PortableReceipt::refuseRetiredFields);
    }

    private static String text(JsonNode node, String name) {
        JsonNode value = node.required(name);
        require(value.isTextual());
        return value.textValue();
    }

    private static BigInteger quantity(JsonNode node, String name) {
        String value = text(node, name);
        require(value.matches("0x(?:0|[1-9a-f][0-9a-f]{0,15})"));
        return new BigInteger(value.substring(2), 16);
    }

    private static byte[] hex(JsonNode node, String name, int minimum, int maximum) {
        return decodeHex(text(node, name), minimum, maximum);
    }

    private static byte[] decodeHex(String value, int minimum, int maximum) {
        require(value.length() >= 2 + minimum * 2 && value.length() <= 2 + maximum * 2
            && value.startsWith("0x") && value.length() % 2 == 0);
        for (int index = 2; index < value.length(); index++) {
            char digit = value.charAt(index);
            require(digit >= '0' && digit <= '9' || digit >= 'a' && digit <= 'f');
        }
        return HexFormat.of().parseHex(value.substring(2));
    }

    private static String hex(byte[] value) { return "0x" + HexFormat.of().formatHex(value); }
    private static void require(boolean valid) {
        if (!valid) throw PlatformSdkException.verificationFailure();
    }
}
