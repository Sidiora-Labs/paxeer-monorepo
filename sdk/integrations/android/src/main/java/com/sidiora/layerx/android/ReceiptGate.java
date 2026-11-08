package com.sidiora.layerx.android;

import com.fasterxml.jackson.databind.JsonNode;
import com.sidiora.layerx.sdk.ProtocolAmount;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Objects;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionException;
import java.util.concurrent.CompletionStage;

/** Settlement is believed only after the device itself verifies the protocol receipt. */
public final class ReceiptGate {
    public record Evidence(byte[] rawTransaction, byte[] signedReceipt, LocalVerifier.ReceiptKey receiptKey) {
        public Evidence {
            Objects.requireNonNull(rawTransaction, "rawTransaction");
            Objects.requireNonNull(signedReceipt, "signedReceipt");
            Objects.requireNonNull(receiptKey, "receiptKey");
            rawTransaction = rawTransaction.clone();
            signedReceipt = signedReceipt.clone();
        }
        @Override public byte[] rawTransaction() { return rawTransaction.clone(); }
        @Override public byte[] signedReceipt() { return signedReceipt.clone(); }
    }

    public record Expectation(byte[] asset, byte[] recipient, ProtocolAmount amount) {
        public Expectation {
            if (asset == null || asset.length != 32 || recipient == null || recipient.length != 32
                    || amount == null) {
                throw MobileIntegrationException.of(MobileIntegrationException.Code.INVALID_CONFIGURATION);
            }
            asset = asset.clone();
            recipient = recipient.clone();
        }
    }

    public sealed interface State permits Pending, Verified, Refused {}
    public record Pending(String reference) implements State {}
    public record Verified(String level, String receiptDigest) implements State {}
    public record Refused(String code) implements State {}

    @FunctionalInterface public interface ReceiptResolver {
        Evidence resolve(String evidenceId);
    }

    private final ReceiptResolver receipts;
    private final MobileClient client;
    private final LocalVerifier.ReceiptKeys receiptKeys;

    public ReceiptGate(ReceiptResolver receipts, LocalVerifier.ReceiptKeys receiptKeys) {
        this.receipts = Objects.requireNonNull(receipts, "receipts");
        this.client = null;
        this.receiptKeys = Objects.requireNonNull(receiptKeys, "receiptKeys");
    }

    public ReceiptGate(MobileClient client, LocalVerifier.ReceiptKeys receiptKeys) {
        this.client = Objects.requireNonNull(client, "client");
        this.receipts = null;
        this.receiptKeys = Objects.requireNonNull(receiptKeys, "receiptKeys");
    }

    public State settle(String receiptReference, Expectation expectation) {
        if (receiptReference == null || receiptReference.isEmpty() || receiptReference.length() > 512
                || receiptReference.indexOf('\0') >= 0) {
            throw MobileIntegrationException.of(MobileIntegrationException.Code.DECODE_FAILURE);
        }
        if (receipts == null) {
            throw MobileIntegrationException.of(MobileIntegrationException.Code.INVALID_CONFIGURATION);
        }
        return verify(receipts.resolve(receiptReference), expectation);
    }

    public State verify(Evidence evidence, Expectation expectation) {
        Objects.requireNonNull(expectation, "expectation");
        LocalVerifier.SequencerReceiptVerification verification = verifyEvidence(evidence);
        if (!matches(verification.receipt(), expectation)) {
            throw MobileIntegrationException.of(MobileIntegrationException.Code.VERIFICATION_FAILURE);
        }
        return verified(verification);
    }

    private LocalVerifier.SequencerReceiptVerification verifyEvidence(Evidence evidence) {
        Objects.requireNonNull(evidence, "evidence");
        LocalVerifier.SequencerReceiptVerification verification;
        try {
            verification = LocalVerifier.verifySequencerReceipt(
                evidence.signedReceipt(), evidence.rawTransaction(), receiptKeys);
            LocalVerifier.ReceiptKey carried = evidence.receiptKey();
            LocalVerifier.ReceiptKey pinned = receiptKeys.entries().stream()
                .filter(key -> MessageDigest.isEqual(key.keyId(), verification.receipt().contextHash()))
                .findFirst().orElseThrow();
            if (!MessageDigest.isEqual(carried.keyId(), pinned.keyId())
                    || !MessageDigest.isEqual(carried.publicKey(), pinned.publicKey())
                    || !carried.firstHeight().equals(pinned.firstHeight())
                    || !carried.lastHeight().equals(pinned.lastHeight())) {
                throw MobileIntegrationException.of(MobileIntegrationException.Code.VERIFICATION_FAILURE);
            }
            verification.requireSuccess();
        } catch (RuntimeException error) {
            throw MobileIntegrationException.of(MobileIntegrationException.Code.VERIFICATION_FAILURE);
        }
        return verification;
    }

    private static boolean matches(LocalVerifier.ProtocolReceipt receipt, Expectation expectation) {
        return MessageDigest.isEqual(receipt.asset(), expectation.asset())
            && MessageDigest.isEqual(receipt.to(), expectation.recipient())
            && expectation.amount().value().equals(receipt.amount());
    }

    private static Verified verified(LocalVerifier.SequencerReceiptVerification verification) {
        return new Verified(verification.level().wire(),
            PublishableConfiguration.hexadecimal(verification.receiptDigest()));
    }

    public State project(JsonNode journey, Expectation expectation) {
        if (client != null) {
            try {
                return projectAsync(journey, expectation).toCompletableFuture().join();
            } catch (CompletionException error) {
                if (error.getCause() instanceof RuntimeException cause) throw cause;
                throw error;
            }
        }
        Objects.requireNonNull(expectation, "expectation");
        State state = localState(journey);
        if (state != null) return state;
        if (receipts == null) {
            throw MobileIntegrationException.of(MobileIntegrationException.Code.INVALID_CONFIGURATION);
        }
        for (Reference reference : references(journey)) {
            Evidence evidence = receipts.resolve(reference.id());
            if (!reference.id().equals(evidenceId(evidence.signedReceipt()))) {
                throw MobileIntegrationException.of(MobileIntegrationException.Code.VERIFICATION_FAILURE);
            }
            LocalVerifier.SequencerReceiptVerification verification = verifyEvidence(evidence);
            if (matches(verification.receipt(), expectation)) return verified(verification);
        }
        throw MobileIntegrationException.of(MobileIntegrationException.Code.VERIFICATION_FAILURE);
    }

    public CompletionStage<State> projectAsync(JsonNode journey, Expectation expectation) {
        Objects.requireNonNull(expectation, "expectation");
        State state = localState(journey);
        if (state != null) return CompletableFuture.completedFuture(state);
        if (client == null) return CompletableFuture.completedFuture(project(journey, expectation));
        return resolve(references(journey), 0, expectation);
    }

    /** Re-fetches durable Human state and proof, including after the application reopens. */
    public CompletionStage<State> track(String journeyId, Expectation expectation) {
        if (client == null) {
            throw MobileIntegrationException.of(MobileIntegrationException.Code.INVALID_CONFIGURATION);
        }
        return client.journey(journeyId).thenCompose(journey -> {
            if (!journeyId.equals(journey.path("journey_id").textValue())) {
                throw MobileIntegrationException.of(MobileIntegrationException.Code.DECODE_FAILURE);
            }
            return projectAsync(journey, expectation);
        });
    }

    private CompletionStage<State> resolve(List<Reference> references, int index, Expectation expectation) {
        if (index == references.size()) {
            return CompletableFuture.failedFuture(
                MobileIntegrationException.of(MobileIntegrationException.Code.VERIFICATION_FAILURE));
        }
        Reference reference = references.get(index);
        return client.evidence(reference.id()).thenCompose(material -> {
            HumanReceiptResolver.Payment payment = HumanReceiptResolver.verify(
                material, reference.id(), reference.verification(), receiptKeys);
            if (MessageDigest.isEqual(payment.asset(), expectation.asset())
                    && MessageDigest.isEqual(payment.recipient(), expectation.recipient())
                    && payment.amount().equals(expectation.amount().value())) {
                return CompletableFuture.completedFuture(new Verified("receipted", payment.digest()));
            }
            return resolve(references, index + 1, expectation);
        });
    }

    private static State localState(JsonNode journey) {
        Objects.requireNonNull(journey, "journey");
        if (!journey.isObject() || !journey.path("state").isTextual()
                || !journey.path("journey_id").isTextual()
                || journey.path("journey_id").textValue().isEmpty()) {
            throw MobileIntegrationException.of(MobileIntegrationException.Code.DECODE_FAILURE);
        }
        String state = journey.path("state").textValue();
        return switch (state) {
            case "done", "done-finalised" -> null;
            case "refused" -> new Refused(state);
            case "closed" -> new Refused(state);
            case "getting-ready", "sending", "processing", "still-checking", "waiting-for-you", "awaiting-approval" ->
                new Pending(journey.path("journey_id").textValue());
            default -> throw MobileIntegrationException.of(MobileIntegrationException.Code.DECODE_FAILURE);
        };
    }

    private record Reference(String id, String verification) {}

    private static List<Reference> references(JsonNode journey) {
        if (!journey.path("evidence").isArray() || !journey.path("stages").isArray()) {
            throw MobileIntegrationException.of(MobileIntegrationException.Code.DECODE_FAILURE);
        }
        List<Reference> references = new ArrayList<>();
        Set<String> seen = new HashSet<>();
        collect(journey.path("evidence"), references, seen);
        for (JsonNode stage : journey.path("stages")) {
            if (!stage.isObject() || !stage.path("evidence").isArray()) {
                throw MobileIntegrationException.of(MobileIntegrationException.Code.DECODE_FAILURE);
            }
            collect(stage.path("evidence"), references, seen);
        }
        return references;
    }

    private static void collect(JsonNode evidence, List<Reference> references, Set<String> seen) {
        for (JsonNode reference : evidence) {
            if (!reference.isObject() || !reference.path("evidence_id").isTextual()
                    || !reference.path("class").isTextual() || !reference.path("verification").isTextual()) {
                throw MobileIntegrationException.of(MobileIntegrationException.Code.DECODE_FAILURE);
            }
            String id = reference.path("evidence_id").textValue();
            String verification = reference.path("verification").textValue();
            if (!id.matches("evd_[0-9a-f]{64}")) {
                // Non-receipt references have opaque identifiers and are never resolved here.
                if ("layerx-receipt".equals(reference.path("class").textValue())) {
                    throw MobileIntegrationException.of(MobileIntegrationException.Code.DECODE_FAILURE);
                }
                continue;
            }
            if ("layerx-receipt".equals(reference.path("class").textValue())
                    && (verification.equals("receipt-verified") || verification.equals("checkpoint-finalised"))
                    && seen.add(id)) {
                references.add(new Reference(id, verification));
            }
        }
    }

    static String evidenceId(byte[] signedReceipt) {
        try {
            return "evd_" + PublishableConfiguration.hexadecimal(
                MessageDigest.getInstance("SHA-256").digest(signedReceipt));
        } catch (NoSuchAlgorithmException error) {
            throw MobileIntegrationException.of(MobileIntegrationException.Code.VERIFICATION_FAILURE);
        }
    }
}
