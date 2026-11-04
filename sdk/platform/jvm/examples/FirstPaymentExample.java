package com.sidiora.layerx.sdk.examples;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import com.sidiora.layerx.sdk.IdempotencyKey;
import com.sidiora.layerx.sdk.PlatformSdkException;
import com.sidiora.layerx.sdk.ProgramsClient;
import com.sidiora.layerx.sdk.SecretBytes;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;
import java.nio.file.attribute.PosixFilePermissions;
import java.security.MessageDigest;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.concurrent.CompletionStage;

/** Native PAX payment over root node EVM RPC; the authenticated Agent API is separate. */
public final class FirstPaymentExample {
    private FirstPaymentExample() {}

    public static CompletionStage<Eip1559Payment.Transaction> prepare(
            ProgramsClient.Programs node, Eip1559Payment.Payment payment, String sender) {
        return node.request("eth_chainId", JsonNodeFactory.instance.arrayNode()).thenCompose(chain -> {
            if (!BigInteger.valueOf(125).equals(Eip1559Payment.quantity(chain))) {
                throw PlatformSdkException.verificationFailure();
            }
            return node.request("eth_getTransactionCount", JsonNodeFactory.instance.arrayNode()
                .add(sender).add("pending"));
        }).thenCompose(nonce -> {
            var estimate = JsonNodeFactory.instance.objectNode().put("type", "0x2")
                .put("chainId", "0x7d").put("from", sender).put("to", payment.recipient())
                .put("nonce", "0x" + Eip1559Payment.quantity(nonce).toString(16))
                .put("value", "0x" + payment.amount().toString(16)).put("data", "0x")
                .put("maxFeePerGas", "0x" + payment.maximumFee().toString(16))
                .put("maxPriorityFeePerGas", "0x" + payment.priorityFee().toString(16));
            estimate.putArray("accessList");
            return node.request("eth_estimateGas", JsonNodeFactory.instance.arrayNode()
                .add(estimate).add("pending")).thenApply(gas -> new Eip1559Payment.Transaction(
                    payment, Eip1559Payment.quantity(nonce), Eip1559Payment.quantity(gas)));
        });
    }

    public static CompletionStage<LocalVerifier.SequencerReceiptVerification> pay(
            ProgramsClient.Programs node, Eip1559Payment.Payment payment, SecretBytes privateKey,
            LocalVerifier.ReceiptKeys pinnedKeys, Path journalDirectory, IdempotencyKey paymentKey) {
        if (paymentKey == null || !paymentKey.value().matches("[0-9a-f]{64}")) {
            throw PlatformSdkException.invalidArgument();
        }
        String sender = Eip1559Payment.sender(privateKey);
        return prepare(node, payment, sender).thenCompose(prepared -> {
            byte[] raw = Eip1559Payment.sign(prepared, privateKey);
            byte[] hash = ProgramsClient.Programs.transactionHash(raw);
            // EVM has no idempotency field: retain this key and exact transaction for recovery.
            try {
                Files.createDirectories(journalDirectory);
                Path retained = journalDirectory.resolve(paymentKey.value() + ".raw");
                Files.createFile(retained, PosixFilePermissions.asFileAttribute(
                    PosixFilePermissions.fromString("rw-------")));
                try (var output = java.nio.channels.FileChannel.open(retained, StandardOpenOption.WRITE)) {
                    var buffer = java.nio.ByteBuffer.wrap(raw);
                    while (buffer.hasRemaining()) output.write(buffer);
                    output.force(true);
                }
            } catch (java.io.IOException error) {
                throw new java.io.UncheckedIOException(error);
            }
            String txHash = "0x" + HexFormat.of().formatHex(hash);
            return node.request("eth_sendRawTransaction", JsonNodeFactory.instance.arrayNode()
                .add("0x" + HexFormat.of().formatHex(raw))).thenCompose(returned -> {
                    if (!returned.isTextual() || !txHash.equals(returned.textValue())) throw unknownOutcome();
                    return node.request("pax_sequencerReceipt", JsonNodeFactory.instance.arrayNode().add(txHash));
                }).thenApply(view -> verifyPayment(view, raw, pinnedKeys, payment, sender));
        });
    }

    public static LocalVerifier.SequencerReceiptVerification verifyPayment(JsonNode view, byte[] raw,
            LocalVerifier.ReceiptKeys pinnedKeys, Eip1559Payment.Payment payment, String sender) {
        if ("submitted".equals(view.path("rung").textValue())) throw unknownOutcome();
        if (!java.util.Set.of("receipted", "committed").contains(view.path("rung").asText())) {
            throw PlatformSdkException.verificationFailure();
        }
        var verified = ProgramsClient.verifyReceipt(ExampleConfiguration.hex(view.required("receipt")),
            raw, pinnedKeys, view);
        verified.requireSuccess();
        var receipt = verified.receipt();
        try {
            byte[] asset = MessageDigest.getInstance("SHA-256").digest("uhpx".getBytes(StandardCharsets.US_ASCII));
            if (!Arrays.equals(asset, receipt.asset()) || !payment.amount().equals(receipt.amount())
                    || !Arrays.equals(principal(sender), receipt.from())
                    || !Arrays.equals(principal(payment.recipient()), receipt.to())) {
                throw PlatformSdkException.verificationFailure();
            }
        } catch (java.security.NoSuchAlgorithmException impossible) {
            throw new IllegalStateException(impossible);
        }
        return verified;
    }

    private static PlatformSdkException unknownOutcome() {
        return new PlatformSdkException(PlatformSdkException.Code.UNKNOWN_OUTCOME,
            PlatformSdkException.Retry.UNKNOWN_OUTCOME, null, null, null);
    }

    private static byte[] principal(String address) {
        byte[] value = new byte[32];
        System.arraycopy(HexFormat.of().parseHex(address.substring(2)), 0, value, 12, 20);
        return value;
    }

    public static void main(String[] args) throws Exception {
        var payment = new Eip1559Payment.Payment(ExampleConfiguration.required("LAYERX_RECIPIENT"),
            new BigInteger(ExampleConfiguration.required("LAYERX_AMOUNT")),
            new BigInteger(ExampleConfiguration.required("LAYERX_MAX_FEE_PER_GAS")),
            new BigInteger(ExampleConfiguration.required("LAYERX_PRIORITY_FEE_PER_GAS")));
        var paymentKey = ExampleConfiguration.randomKey();
        var journal = Path.of(ExampleConfiguration.required("LAYERX_PAYMENT_JOURNAL"));
        System.out.println("Payment journal reference: " + paymentKey.value());
        try (var node = new ProgramsClient.Programs(ExampleConfiguration.nodeOrigin(
                ExampleConfiguration.required("LAYERX_NODE_RPC")));
                var privateKey = new SecretBytes(HexFormat.of().parseHex(Files.readString(Path.of(
                    ExampleConfiguration.required("LAYERX_EVM_KEY_FILE"))).trim()))) {
            var verified = pay(node, payment, privateKey, ExampleConfiguration.receiptKeys(Path.of(
                ExampleConfiguration.required("LAYERX_RECEIPT_PINS"))), journal, paymentKey)
                .toCompletableFuture().join();
            System.out.println("Payment verified at " + verified.level().wire() + ": 0x"
                + HexFormat.of().formatHex(verified.receipt().activityId()));
        }
    }
}
