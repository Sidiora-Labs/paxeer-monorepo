package com.sidiora.layerx.sdk.verify;

import com.sidiora.layerx.sdk.PlatformSdkException;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.List;

/** A chain and validator root obtained independently of the portable evidence. */
public record ConsensusTrust(String chainId, long height, byte[] validatorsHash) {
    public ConsensusTrust {
        if (chainId == null || chainId.isEmpty()
                || chainId.getBytes(StandardCharsets.UTF_8).length > 50 || height <= 0
                || validatorsHash == null || validatorsHash.length != 32)
            throw PlatformSdkException.invalidArgument();
        validatorsHash = validatorsHash.clone();
    }

    @Override public byte[] validatorsHash() { return validatorsHash.clone(); }

    public record Step(Committed.Header header, Committed.Commit commit,
                       List<Committed.Validator> validators) {
        public Step { validators = List.copyOf(validators); }
    }

    public void verify(Committed.Header header, Committed.Commit commit,
            List<Committed.Validator> validators, List<Step> transitions) {
        try {
            authenticate(header, transitions);
            Committed.verifyCommit(header, commit, validators);
        } catch (PlatformSdkException error) {
            throw error;
        } catch (RuntimeException error) {
            throw PlatformSdkException.verificationFailure();
        }
    }

    void authenticate(Committed.Header target, List<Step> transitions) {
        require(transitions != null && transitions.size() <= 10000 && target != null
            && target.height() >= height && target.height() - height == transitions.size());
        byte[] expectedValidators = validatorsHash();
        Committed.Header previous = null;
        Committed.Commit previousCommit = null;
        long expectedHeight = height;
        for (Step step : transitions) {
            Committed.Header header = step.header();
            authenticateHeader(header, expectedHeight, expectedValidators, previous, previousCommit);
            Committed.verifyCommit(header, step.commit(), step.validators());
            expectedValidators = header.nextValidatorsHash();
            previous = header;
            previousCommit = step.commit();
            expectedHeight++;
        }
        authenticateHeader(target, expectedHeight, expectedValidators, previous, previousCommit);
    }

    private void authenticateHeader(Committed.Header header, long expectedHeight,
            byte[] expectedValidators, Committed.Header previous, Committed.Commit previousCommit) {
        require(header != null && chainId.equals(header.chainId()) && header.height() == expectedHeight
            && MessageDigest.isEqual(expectedValidators, header.validatorsHash()));
        if (previous != null) {
            require(MessageDigest.isEqual(previous.hash(), header.lastBlockId().hash())
                && header.lastBlockId().partsTotal() == previousCommit.blockId().partsTotal()
                && MessageDigest.isEqual(header.lastBlockId().partsHash(), previousCommit.blockId().partsHash()));
        }
    }

    private static void require(boolean valid) {
        if (!valid) throw PlatformSdkException.verificationFailure();
    }
}
