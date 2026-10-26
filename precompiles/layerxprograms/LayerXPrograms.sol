// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

address constant LAYERX_PROGRAMS_ADDRESS = 0x000000000000000000000000000000000000101D;

interface ILayerXPrograms {
    struct TransferLeg {
        bytes32 from;
        bytes32 asset;
        bytes32 to;
        uint128 amount;
    }

    event ProgramDeployed(bytes32 indexed programId, bytes32 codeHash);

    event ProgramUpgraded(bytes32 indexed programId, bytes32 oldCodeHash, bytes32 newCodeHash);

    event RegistryRead(bytes32 indexed programId);

    event ProgramTransferred(bytes32 indexed programId, bytes32 transferSetRoot);

    event ProgramGuestEvent(
        bytes32 indexed programId,
        bytes32 principal,
        bytes32 activityId,
        uint64 framePath,
        uint8 depth,
        uint32 eventIndex,
        uint16 topicLength,
        bytes32 topicDigest,
        uint32 dataLength,
        bytes32 dataDigest
    );

    event ProgramCallOutcome(
        bytes32 indexed programId,
        bytes32 principal,
        bytes32 activityId,
        uint64 framePath,
        uint8 depth,
        uint16 runtimeVersion,
        uint16 abiVersion,
        uint32 feeScheduleVersion,
        uint32 meteringVersion,
        int32 terminalResult,
        bytes32 transferSetRoot,
        bytes32 callGraphDigest,
        bytes32 terminalDetailDigest,
        bytes32 eventEnvelopeDigest
    );

    event ProgramAccountRegistered(
        bytes32 indexed programId,
        bytes32 accountId,
        bytes32 assetId,
        bytes32 seedDigest,
        uint16 seedLength,
        uint64 registeredSequence
    );

    event ExitRouteSet(bytes32 indexed programId, bytes32 accountId, bytes32 assetId, bytes32 destination);

    event ProgramDeprecated(
        bytes32 indexed programId,
        uint8 priorStatus,
        uint8 targetStatus,
        bytes32 principal,
        bytes32 exitProgram,
        uint64 deadline,
        uint64 sequence,
        uint16 accountCount,
        uint16 liveCount,
        bytes32 accountRoot
    );

    event ProgramTombstoned(
        bytes32 indexed programId,
        uint8 priorStatus,
        uint8 targetStatus,
        bytes32 principal,
        bytes32 exitProgram,
        uint64 deadline,
        uint64 sequence,
        uint16 accountCount,
        uint16 liveCount,
        bytes32 accountRoot
    );

    event ProgramValueExited(bytes32 indexed programId, bytes32 accountId, bytes32 transferSetRoot);

    event SandboxUsageReceipt(
        bytes32 indexed activityId,
        uint64 expectedSequence,
        uint16 chunkIndex,
        uint16 chunkCount,
        uint32 receiptLength,
        bytes chunk
    );

    event SandboxDestroyed(bytes32 indexed leaseId, uint8 part, bytes terminalFrame);

    event StorageRetired(bytes32 indexed activityId, uint8 frameIndex, bytes frame);

    function call(
        bytes32 programId,
        uint16 abiVersion,
        string calldata entrypoint,
        bytes calldata callData,
        bytes calldata capabilities,
        bytes calldata accessDeclaration,
        uint32 responseCapacity,
        uint64[7] calldata budget,
        uint128 feeLimit
    ) external returns (int32 terminalResult, uint8 terminalKind, bytes memory response);

    function deploy(
        bytes32 programId,
        uint16 abiVersion,
        uint8 policy,
        bytes32 authority,
        bytes32 codeHash,
        bytes calldata interfaceEncoding,
        bytes calldata wasm
    ) external;

    function deprecate(bytes32 programId, bytes32 exitProgram, uint64 deadline) external;

    function exitValue(bytes32 programId, bytes32 account) external;

    function exitValueBounded(bytes32 programId, bytes32 account, uint128 maxAmount) external;

    function getAccountProfile(bytes32 programId) external view returns (uint8 profile, bytes32 principal);

    function getCurrentFeeSchedule() external view returns (uint32 version, uint64[7] memory prices, bytes32 feeAsset);

    function getCurrentMeteringSchedule() external view returns (uint32 version, uint64[9] memory coefficients);

    function getExitRoute(bytes32 programId, bytes32 accountId)
        external
        view
        returns (bytes32 asset, bytes32 destination, bytes memory seed);

    function getFeeSchedule(uint32 version) external view returns (uint64[7] memory prices, bytes32 feeAsset);

    function getInterface(bytes32 programId)
        external
        view
        returns (uint32 version, bytes32 digest, bytes memory encoding);

    function getMeteringSchedule(uint32 version)
        external
        view
        returns (uint64[9] memory coefficients, uint8 authorityKind);

    function getPendingFeeSchedule()
        external
        view
        returns (uint64[7] memory prices, bytes32 asset, uint64[6] memory demand, uint64 activationBatch);

    function getProgram(bytes32 programId)
        external
        view
        returns (uint8 policy, bytes32 authority, bytes32 codeHash, uint16 abiVersion, uint32 version, bool active);

    function getProgramAccount(
        bytes32 programId,
        bytes32 assetId,
        bytes calldata seed
    ) external view returns (bytes32 accountId, bytes32 registeredAssetId, uint64 registeredSequence);

    function getProgramAccountById(bytes32 accountId)
        external
        view
        returns (bytes32 programId, bytes32 assetId, bytes memory seed);

    function getProgramOwner(bytes32 programId) external view returns (bytes32 owner);

    function getRetirementControl() external view returns (bytes memory control);

    function getSandbox(bytes32 leaseId, uint8 kind) external view returns (bytes memory state);

    function getStateHead(bytes32 programId) external view returns (bytes memory stateRecord);

    function getValueAccount(bytes32 programId, bytes32 accountId) external view returns (uint128 balance, bool frozen);

    function getWebRequest(bytes32 programId, uint64 requestId) external view returns (bytes memory record);

    function getWindDown(bytes32 programId)
        external
        view
        returns (
            uint8 status,
            bytes32 exitProgram,
            uint64 deadline,
            uint64 effectiveSequence,
            uint16 accountCount,
            uint16 liveCount
        );

    function quoteCallFee(uint64[7] calldata budget)
        external
        view
        returns (uint32 version, uint128 executionCeiling, bytes32 feeAsset);

    function registerAccount(bytes32 programId, uint8 profile, bytes32 assetId, bytes calldata seed) external;

    function registryRead(bytes32 programId, bytes calldata body) external;

    function retireStorage(bytes calldata request) external;

    function sandboxActivate(bytes32 leaseId, bytes32 expectedLeaseDigest, uint64 expectedSequence) external;

    function sandboxExecute(
        bytes32 leaseId,
        uint64 expectedSequence,
        bytes32 expectedLeaseDigest,
        bytes32 escrowAccount,
        bytes32 asset,
        bytes32 feeDestination,
        uint32 feeScheduleVersion,
        uint64[7] calldata feeSchedule,
        bytes calldata callPayload,
        uint128 feeLimit
    ) external returns (int32 terminalResult, uint8 terminalKind, bytes memory response);

    function sandboxFund(
        bytes32 leaseId,
        bytes32 tenant,
        bytes32 hostProgram,
        bytes32 escrowAccount,
        bytes32 asset,
        uint128 amount,
        uint64 expiry,
        uint32 feeScheduleVersion,
        uint64[7] calldata feeSchedule,
        bytes calldata leaseRequest,
        bytes32 transferProgramId,
        TransferLeg[] calldata fundingLegs
    ) external;

    function setExitRoute(
        bytes32 programId,
        bytes32 account,
        bytes32 asset,
        bytes32 destination,
        bytes calldata seed
    ) external;

    function simulateCall(
        bytes32 programId,
        uint16 abiVersion,
        string calldata entrypoint,
        bytes calldata callData,
        bytes calldata capabilities,
        bytes calldata accessDeclaration,
        uint32 responseCapacity,
        uint64[7] calldata budget,
        uint128 feeLimit
    ) external view returns (int32 terminalResult, uint8 terminalKind, bytes memory response);

    function tombstone(bytes32 programId) external;

    function transfer(bytes32 programId, TransferLeg[] calldata legs) external;

    function upgrade(
        bytes32 programId,
        uint16 abiVersion,
        uint8 flags,
        bytes32 oldCodeHash,
        bytes32 newCodeHash,
        bytes calldata migrationHook,
        bytes calldata interfaceEncoding,
        bytes calldata wasm,
        uint128 feeLimit
    ) external;
}
