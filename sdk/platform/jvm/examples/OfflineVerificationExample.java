package com.sidiora.layerx.sdk.examples;

import com.sidiora.layerx.sdk.PlatformSdkException;
import com.sidiora.layerx.sdk.verify.ConsensusTrust;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.PortableReceipt;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.List;

/** Verify unmodified portable v2 evidence with independent trust and the retained transaction. */
public final class OfflineVerificationExample {
    private OfflineVerificationExample() {}

    public static PortableReceipt.Verification verify(Path bundle, Path rawTransaction,
            LocalVerifier.ReceiptKeys pinnedKeys, ConsensusTrust consensusTrust,
            List<ConsensusTrust.Step> transitions) throws java.io.IOException {
        byte[] encoded = Files.readAllBytes(bundle);
        byte[] retained = Files.readAllBytes(rawTransaction);
        var verified = PortableReceipt.verify(encoded, pinnedKeys, consensusTrust, transitions);
        byte[] carried = ExampleConfiguration.hex(ExampleConfiguration.JSON.readTree(encoded)
            .required("rawTransaction"));
        if (!Arrays.equals(retained, carried)) throw PlatformSdkException.verificationFailure();
        verified.requireSuccess();
        return verified;
    }

    public static void main(String[] args) throws Exception {
        if (args.length < 3 || args.length > 5) {
            throw new IllegalArgumentException("Usage: OfflineVerificationExample bundle.json transaction.raw "
                + "independent-pins.json [independent-consensus.json [transitions.json]]");
        }
        var trust = args.length >= 4 ? ExampleConfiguration.consensusTrust(Path.of(args[3])) : null;
        var steps = args.length == 5 ? ExampleConfiguration.transitions(Path.of(args[4])) : List.<ConsensusTrust.Step>of();
        var verified = verify(Path.of(args[0]), Path.of(args[1]),
            ExampleConfiguration.receiptKeys(Path.of(args[2])), trust, steps);
        System.out.println("Receipt verified at " + verified.verificationLevel() + ": 0x"
            + java.util.HexFormat.of().formatHex(verified.receipted().receipt().activityId()));
    }
}
