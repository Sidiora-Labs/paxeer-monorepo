package com.sidiora.layerx;

import com.sidiora.layerx.spring.ReceiptKeysConfigTest;
import com.sidiora.layerx.spring.SellerReceiptsTest;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

public final class Repair2233621Test {
    @TempDir(factory = ReceiptKeysConfigTest.OwnerStorage.class)
    Path storageDirectory;

    @Test
    void finding030NativePaymentRetainsPinnedKeyAndSignatureRefusals() throws Exception {
        new SellerReceiptsTest().settlesSignedSequencerReceipt();
    }

    @Test
    void finding030NativePaymentBindsTermsChainAndExecutionEffects() throws Exception {
        new SellerReceiptsTest().nativePaymentRefusesMismatchedTermsAndNonpaymentReceipts();
    }

    @Test
    void finding030RegisteredFilterResponseReachesBuyerAndReplays() throws Exception {
        new SellerReceiptsTest().registeredFilterRoundTripsCurrentEvidenceToBuyerAndReplays();
    }

    @Test
    void finding030RegisteredFilterRefusesTamperedPortableEvidence() throws Exception {
        new SellerReceiptsTest().registeredFilterRefusesAlteredPortableEvidenceBeforeRelease();
    }

    @Test
    void finding030DurableStoreReopensReceiptKeyAndRefusesConflicts() throws Exception {
        new SellerReceiptsTest().durableStoreRoundTripsReceiptKey(storageDirectory);
    }
}
