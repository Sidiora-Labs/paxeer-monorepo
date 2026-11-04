package types

// AvailabilityGuidance describes the mainnet availability of the library's CLI.
const AvailabilityGuidance = "ICA is unsupported on mainnet paxd: controller and host channels, account registration, remote transactions, gRPC queries and REST queries are unavailable. These commands are library tools for applications that explicitly wire ICA; packet-events requires an ICA-enabled chain's historical events. Library defaults and simapp support do not enable ICA on mainnet."
