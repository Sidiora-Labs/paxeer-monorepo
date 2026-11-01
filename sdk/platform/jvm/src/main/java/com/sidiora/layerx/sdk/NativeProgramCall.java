package com.sidiora.layerx.sdk;

import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import com.sidiora.layerx.sdk.verify.GeneratedReceiptContract;

public record NativeProgramCall(byte[] programId, int guestAbi, String entrypoint, byte[] calldata,
        byte[] capabilities, byte[] accessDeclaration, int responseCapacity, long[] resources) {
    public byte[] encode() {
        if (programId.length != 32 || Arrays.equals(programId, new byte[32]) || !GeneratedReceiptContract.supportsProgramGuestAbi(guestAbi)
                || !entrypoint.matches("[A-Za-z0-9_.]{1,128}") || calldata.length > 1048576
                || capabilities.length > 65535 || accessDeclaration.length > 1048576
                || responseCapacity < 0 || responseCapacity > 1048576 || resources.length != 7)
            throw new IllegalArgumentException("invalid native program call");
        byte[] entry = entrypoint.getBytes(StandardCharsets.US_ASCII);
        ByteBuffer out = ByteBuffer.allocate(106 + entry.length + calldata.length + capabilities.length + accessDeclaration.length);
        out.put(programId).putShort((short)guestAbi).putShort((short)entry.length).putInt(calldata.length)
            .putShort((short)capabilities.length).putInt(accessDeclaration.length).putInt(responseCapacity);
        for (long resource : resources) out.putLong(resource);
        return out.put(entry).put(calldata).put(capabilities).put(accessDeclaration).array();
    }

    public NativeProgramCall {
        programId = programId.clone(); calldata = calldata.clone();
        capabilities = capabilities.clone(); accessDeclaration = accessDeclaration.clone();
        resources = resources.clone();
    }
    @Override public byte[] programId() { return programId.clone(); }
    @Override public byte[] calldata() { return calldata.clone(); }
    @Override public byte[] capabilities() { return capabilities.clone(); }
    @Override public byte[] accessDeclaration() { return accessDeclaration.clone(); }
    @Override public long[] resources() { return resources.clone(); }

    public byte[] calldata(java.math.BigInteger feeLimit) {
        encode();
        Object[] fields = new Object[15];
        fields[0] = programId; fields[1] = ProgramsClient.Programs.word(java.math.BigInteger.valueOf(guestAbi), 16);
        fields[2] = entrypoint; fields[3] = calldata; fields[4] = capabilities; fields[5] = accessDeclaration;
        fields[6] = ProgramsClient.Programs.word(java.math.BigInteger.valueOf(responseCapacity), 32);
        for (int i = 0; i < 7; i++) fields[7 + i] = ProgramsClient.Programs.word(new java.math.BigInteger(Long.toUnsignedString(resources[i])), 64);
        fields[14] = ProgramsClient.Programs.word(feeLimit, 128);
        return ProgramsClient.Programs.abi("54bd2b48", fields, 2, 3, 4, 5);
    }

    public byte[] simulateCalldata(java.math.BigInteger feeLimit) {
        byte[] data = calldata(feeLimit);
        System.arraycopy(java.util.HexFormat.of().parseHex("3044bd56"), 0, data, 0, 4);
        return data;
    }

    public byte[] bindSignedActivity(byte[] signedActivity) {
        byte[] data = ProgramsClient.Programs.transactionData(signedActivity);
        NativeProgramCall decoded = decodeCalldata(data);
        if (!Arrays.equals(encode(), decoded.encode())) throw new IllegalArgumentException("program transaction differs from call");
        return ProgramsClient.Programs.transactionHash(signedActivity);
    }

    public static NativeProgramCall decodeSignedActivity(byte[] signedActivity) {
        return decodeCalldata(ProgramsClient.Programs.transactionData(signedActivity));
    }

    public static NativeProgramCall decodeCalldata(byte[] data) {
        if (data.length < 484 || !Arrays.equals(Arrays.copyOf(data, 4), java.util.HexFormat.of().parseHex("54bd2b48")))
            throw new IllegalArgumentException("invalid program call selector");
        long[] resources = new long[7];
        for (int i = 0; i < 7; i++) resources[i] = ProgramsClient.Programs.number(data, 4 + (7 + i) * 32, 64).longValue();
        NativeProgramCall call = new NativeProgramCall(Arrays.copyOfRange(data, 4, 36),
            ProgramsClient.Programs.number(data, 36, 16).intValueExact(),
            new String(ProgramsClient.Programs.dynamic(data, 4, 2), StandardCharsets.US_ASCII),
            ProgramsClient.Programs.dynamic(data, 4, 3), ProgramsClient.Programs.dynamic(data, 4, 4),
            ProgramsClient.Programs.dynamic(data, 4, 5), ProgramsClient.Programs.number(data, 196, 32).intValueExact(), resources);
        java.math.BigInteger fee = ProgramsClient.Programs.number(data, 452, 128);
        if (!Arrays.equals(call.calldata(fee), data)) throw new IllegalArgumentException("non-canonical program calldata");
        return call;
    }

    public static NativeProgramCall decode(byte[] payload) {
        if (payload.length < 106) throw new IllegalArgumentException("invalid native program call");
        ByteBuffer input = ByteBuffer.wrap(payload);
        byte[] program = new byte[32]; input.get(program);
        int abi = Short.toUnsignedInt(input.getShort());
        int entry = Short.toUnsignedInt(input.getShort()); long data = Integer.toUnsignedLong(input.getInt());
        int caps = Short.toUnsignedInt(input.getShort()); long access = Integer.toUnsignedLong(input.getInt());
        int capacity = input.getInt(); long[] resources = new long[7];
        for (int i = 0; i < 7; i++) resources[i] = input.getLong();
        if (106L + entry + data + caps + access != payload.length) throw new IllegalArgumentException("invalid native program call");
        byte[] entryBytes = new byte[entry], dataBytes = new byte[(int)data], capBytes = new byte[caps], accessBytes = new byte[(int)access];
        input.get(entryBytes).get(dataBytes).get(capBytes).get(accessBytes);
        NativeProgramCall call = new NativeProgramCall(program, abi, new String(entryBytes, StandardCharsets.US_ASCII), dataBytes, capBytes, accessBytes, capacity, resources);
        if (!Arrays.equals(call.encode(), payload)) throw new IllegalArgumentException("invalid native program call");
        return call;
    }
}
