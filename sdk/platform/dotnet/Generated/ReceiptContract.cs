// Code generated from sdk/platform/generators/receipt.kvx. DO NOT EDIT.
#nullable enable

namespace LayerX.Sdk;

public enum ReceiptCheck
{
    Decode,
    CanonicalEncoding,
    ReceiptVersion,
    ActivityId,
    KeyId,
    KeyHeight,
    RawTransaction,
    Fields,
    ReturnData,
    Logs,
    SequenceHash,
    Signature,
}

public static class GeneratedReceiptContract
{
    public const ushort ReceiptVersionV2 = 2;
    public const ushort ReceiptVersionV3 = 3;
    public const string ReceiptDomain = "LXP/v1/receipt";
    public const string SequenceDomain = "PAXEER/v1/sequence";

    public static string MachineCode(this ReceiptCheck check) => check switch
    {
        ReceiptCheck.Decode => "decode",
        ReceiptCheck.CanonicalEncoding => "canonical-encoding",
        ReceiptCheck.ReceiptVersion => "receipt-version",
        ReceiptCheck.ActivityId => "activity-id",
        ReceiptCheck.KeyId => "key-id",
        ReceiptCheck.KeyHeight => "key-height",
        ReceiptCheck.RawTransaction => "raw-transaction",
        ReceiptCheck.Fields => "fields",
        ReceiptCheck.ReturnData => "return-data",
        ReceiptCheck.Logs => "logs",
        ReceiptCheck.SequenceHash => "sequence-hash",
        ReceiptCheck.Signature => "signature",
        _ => throw new ArgumentOutOfRangeException(nameof(check)),
    };

    public const ushort ProgramAbiV1 = 1;
    public const ushort ProgramAbiV2 = 2;
    public const ushort ProgramAbiV3 = 3;
    public const ushort ProgramAbiV4 = 4;
    public const ushort ProgramAbiV5 = 5;
    public static bool SupportsProgramGuestAbi(ushort version) => version switch
    {
        ProgramAbiV1 or ProgramAbiV2 or ProgramAbiV3 or ProgramAbiV4 or ProgramAbiV5 => true,
        _ => false,
    };
}
