package com.sidiora.layerx.sdk.examples;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sidiora.layerx.sdk.HttpProductionTransport;
import com.sidiora.layerx.sdk.IdempotencyKey;
import com.sidiora.layerx.sdk.PlatformSdkException;
import com.sidiora.layerx.sdk.SecretBytes;
import com.sidiora.layerx.sdk.verify.Committed;
import com.sidiora.layerx.sdk.verify.ConsensusTrust;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import java.math.BigInteger;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.SecureRandom;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.List;

/** Shared configuration. Network addresses and trust material have no implicit defaults. */
public final class ExampleConfiguration {
    static final ObjectMapper JSON = new ObjectMapper();
    private static final SecureRandom RANDOM = new SecureRandom();

    private ExampleConfiguration() {}

    public static String required(String name) {
        String value = System.getenv(name);
        if (value == null || value.isBlank()) throw new IllegalArgumentException("Set " + name);
        return value;
    }

    public static URI serviceOrigin(String value) {
        URI uri = URI.create(value);
        boolean loopback = "127.0.0.1".equals(uri.getHost()) || "localhost".equals(uri.getHost())
            || "[::1]".equals(uri.getHost());
        if (uri.getHost() == null || !("https".equals(uri.getScheme()) || loopback && "http".equals(uri.getScheme()))
                || uri.getUserInfo() != null || uri.getQuery() != null || uri.getFragment() != null
                || !(uri.getPath().isEmpty() || "/".equals(uri.getPath()))) {
            throw PlatformSdkException.invalidArgument();
        }
        return uri;
    }

    public static URI nodeOrigin(String value) { return serviceOrigin(value); }

    public static IdempotencyKey randomKey() {
        byte[] key = new byte[32];
        RANDOM.nextBytes(key);
        return new IdempotencyKey(HexFormat.of().formatHex(key));
    }

    public static HttpProductionTransport.AgentSessionCredential session(String tenant, String sessionId,
            String tokenId, String generation) {
        return new HttpProductionTransport.AgentSessionCredential(tenant, sessionId,
            new SecretBytes(tokenId.getBytes(StandardCharsets.US_ASCII)), generation);
    }

    public static byte[] hex(JsonNode value) {
        if (!value.isTextual() || !value.textValue().matches("0x(?:[0-9a-f]{2})*")) {
            throw PlatformSdkException.invalidArgument();
        }
        return HexFormat.of().parseHex(value.textValue().substring(2));
    }

    public static LocalVerifier.ReceiptKeys receiptKeys(Path path) throws java.io.IOException {
        JsonNode document = JSON.readTree(Files.readAllBytes(path));
        if (document == null || !document.isArray()) throw PlatformSdkException.invalidArgument();
        var entries = new ArrayList<LocalVerifier.ReceiptKey>();
        for (JsonNode pin : document) {
            entries.add(new LocalVerifier.ReceiptKey(hex(pin.required("keyId")), hex(pin.required("publicKey")),
                decimal(pin.required("firstHeight")), decimal(pin.required("lastHeight"))));
        }
        return new LocalVerifier.ReceiptKeys(entries);
    }

    public static ConsensusTrust consensusTrust(Path path) throws java.io.IOException {
        JsonNode pin = JSON.readTree(Files.readAllBytes(path));
        return new ConsensusTrust(pin.required("chainId").textValue(), decimal(pin.required("height")).longValueExact(),
            hex(pin.required("validatorsHash")));
    }

    public static List<ConsensusTrust.Step> transitions(Path path) throws java.io.IOException {
        JsonNode document = JSON.readTree(Files.readAllBytes(path));
        if (document == null || !document.isArray()) throw PlatformSdkException.invalidArgument();
        var steps = new ArrayList<ConsensusTrust.Step>();
        for (JsonNode step : document) {
            steps.add(new ConsensusTrust.Step(Committed.decodeHeader(step.required("header")),
                Committed.decodeCommit(step.required("commit")),
                Committed.decodeValidators(step.required("validatorSet"))));
        }
        return List.copyOf(steps);
    }

    private static BigInteger decimal(JsonNode value) {
        if (!value.isTextual() || !value.textValue().matches("0|[1-9][0-9]*")) {
            throw PlatformSdkException.invalidArgument();
        }
        return new BigInteger(value.textValue());
    }
}
