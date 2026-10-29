package com.sidiora.layerx.sdk;

import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import static com.sidiora.layerx.sdk.NativeProgramLifecycle.*;

public final class NativeProgramLifecycleRequest {
    private final NativeProgramLifecycle operation;
    private final byte[] signed;
    private final byte[] transactionHash;
    private final IdempotencyKey idempotencyKey;

    public NativeProgramLifecycleRequest(NativeProgramLifecycle operation, byte[] signedActivity) {
        this(operation, signedActivity, null);
    }
    public NativeProgramLifecycleRequest(NativeProgramLifecycle operation, byte[] signedActivity, IdempotencyKey key) {
        require(operation != null && signedActivity != null);
        this.operation = NativeProgramLifecycle.decode(operation.ordinal(), operation.encode());
        this.signed = signedActivity.clone();
        byte[] data;
        try { data = ProgramsClient.Programs.transactionData(signed); }
        catch (PlatformSdkException error) { throw new IllegalArgumentException("invalid signed program transaction", error); }
        java.math.BigInteger fee = operation instanceof NativeProgramUpgrade
            ? ProgramsClient.Programs.number(data, 4 + 8 * 32, 128) : java.math.BigInteger.ZERO;
        require(Arrays.equals(calldata(operation, fee), data));
        idempotencyKey = key;
        transactionHash = ProgramsClient.Programs.transactionHash(signed);
    }
    public int ordinal() { return operation.ordinal(); }
    public byte[] payload() { return operation.encode(); }
    public byte[] signedActivity() { return signed.clone(); }
    public byte[] activityId() { return transactionHash(); }
    public String idempotencyKey() {
        require(idempotencyKey != null);
        return idempotencyKey.value();
    }
    public byte[] transactionHash() { return transactionHash.clone(); }
    public byte[] signedTransaction() { return signed.clone(); }

    public static byte[] calldata(NativeProgramLifecycle operation, java.math.BigInteger feeLimit) {
        require(operation != null);
        if (operation instanceof NativeProgramDeploy deploy) {
            return ProgramsClient.Programs.abi("bde3ff66", new Object[] {deploy.programId(),
                ProgramsClient.Programs.word(java.math.BigInteger.valueOf(deploy.guestAbi()), 16),
                ProgramsClient.Programs.word(java.math.BigInteger.valueOf(deploy.policy()), 8), deploy.authority(), deploy.newHash(),
                deploy.programInterface() == null ? new byte[0] : deploy.programInterface(), deploy.wasm()}, 5, 6);
        }
        if (operation instanceof NativeProgramUpgrade upgrade) {
            int flags = (upgrade.migrationHook().length > 0 ? 1 : 0) | (upgrade.clearInterface() ? 2 : 0);
            return ProgramsClient.Programs.abi("a860929f", new Object[] {upgrade.programId(),
                ProgramsClient.Programs.word(java.math.BigInteger.valueOf(upgrade.guestAbi()), 16),
                ProgramsClient.Programs.word(java.math.BigInteger.valueOf(flags), 8), upgrade.oldHash(), upgrade.newHash(),
                upgrade.migrationHook(), upgrade.programInterface() == null ? new byte[0] : upgrade.programInterface(),
                upgrade.wasm(), ProgramsClient.Programs.word(feeLimit, 128)}, 5, 6, 7);
        }
        NativeProgramWindDown wind = (NativeProgramWindDown) operation;
        byte[] payload = wind.encode();
        return switch (wind.operation()) {
            case 1 -> ProgramsClient.Programs.abi(ProgramsClient.Programs.selector("setExitRoute(bytes32,bytes32,bytes32,bytes32,bytes)"),
                new Object[] {wind.programId(), Arrays.copyOfRange(payload, 33, 65), Arrays.copyOfRange(payload, 65, 97),
                    Arrays.copyOfRange(payload, 97, 129), Arrays.copyOfRange(payload, 131, payload.length)}, 4);
            case 2 -> ProgramsClient.Programs.abi(ProgramsClient.Programs.selector("deprecate(bytes32,bytes32,uint64)"),
                new Object[] {wind.programId(), Arrays.copyOfRange(payload, 33, 65),
                    ProgramsClient.Programs.word(new java.math.BigInteger(1, Arrays.copyOfRange(payload, 65, 73)), 64)});
            case 3 -> ProgramsClient.Programs.abi(ProgramsClient.Programs.selector("tombstone(bytes32)"), new Object[] {wind.programId()});
            case 4 -> ProgramsClient.Programs.abi(ProgramsClient.Programs.selector("exitValue(bytes32,bytes32)"),
                new Object[] {wind.programId(), Arrays.copyOfRange(payload, 33, 65)});
            default -> throw new IllegalArgumentException("wind-down operation");
        };
    }

    static byte[] bind(int ordinal, byte[] expected, byte[] signed) {
        require(signed != null && signed.length > 0 && signed.length <= 1048576);
        try {
            ByteBuffer cursor = ByteBuffer.wrap(signed);
            require(cursor.getShort() == 3 && cursor.getShort() == 0x1001 && cursor.get() == 12 && cursor.get() == 1 && cursor.getShort() == 3 && cursor.get() == 2);
            cursor.getInt(); require(cursor.get() == 3 && cursor.getInt() == (0x00090000 | ordinal) && cursor.get() == 4);
            bounded(cursor, 255); require(cursor.get() == 5); bounded(cursor, 524288);
            require(cursor.get() == 6); cursor.getLong(); require(cursor.get() == 7);
            long before = cursor.getLong(); long after = cursor.getLong(); require(Long.compareUnsigned(after, before) >= 0 && cursor.get() == 8);
            byte[] key = bounded(cursor, 32); require(key.length == 32 && cursor.get() == 9);
            cursor.getLong(); cursor.getLong(); require(cursor.get() == 10); byte[] digest = bounded(cursor, 32);
            require(digest.length == 32 && cursor.get() == 11); byte[] payload = bounded(cursor, 524288);
            require(cursor.get() == 12); bounded(cursor, 128);
            require(!cursor.hasRemaining() && Arrays.equals(payload, expected) && Arrays.equals(digest, hash("LXP/v1/payload-hash\0".getBytes(StandardCharsets.UTF_8), payload)));
            return key;
        } catch (java.nio.BufferUnderflowException error) { throw new IllegalArgumentException("Truncated signed activity", error); }
    }

    static byte[] bounded(ByteBuffer cursor, int maximum) {
        long length = Integer.toUnsignedLong(cursor.getInt()); require(length <= maximum && length <= cursor.remaining());
        byte[] value = new byte[(int) length]; cursor.get(value); return value;
    }
}
