# Bridge vectors contract

Source paths are relative to the repository root. Declaration tables identify the interfaces in the source files; runtime behavior is defined by their implementations.

## chain-config

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.6.fmt-bridge-evm-chain-config-arbitrum-j36</code> | <code>file_format</code> | <code>contracts/bridge/evm/chains/arbitrum/config.json</code> | <code>contracts/bridge/evm/chains/arbitrum/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-avalanche-j36</code> | <code>file_format</code> | <code>contracts/bridge/evm/chains/avalanche/config.json</code> | <code>contracts/bridge/evm/chains/avalanche/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-base-j36</code> | <code>file_format</code> | <code>contracts/bridge/evm/chains/base/config.json</code> | <code>contracts/bridge/evm/chains/base/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-bnb-j36</code> | <code>file_format</code> | <code>contracts/bridge/evm/chains/bnb/config.json</code> | <code>contracts/bridge/evm/chains/bnb/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-ethereum-j36</code> | <code>file_format</code> | <code>contracts/bridge/evm/chains/ethereum/config.json</code> | <code>contracts/bridge/evm/chains/ethereum/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-hyperevm-j36</code> | <code>file_format</code> | <code>contracts/bridge/evm/chains/hyperevm/config.json</code> | <code>contracts/bridge/evm/chains/hyperevm/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-optimism-j36</code> | <code>file_format</code> | <code>contracts/bridge/evm/chains/optimism/config.json</code> | <code>contracts/bridge/evm/chains/optimism/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-polygon-j36</code> | <code>file_format</code> | <code>contracts/bridge/evm/chains/polygon/config.json</code> | <code>contracts/bridge/evm/chains/polygon/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-solana-chain-config-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/chains/solana/config.json</code> | <code>contracts/bridge/solana/chains/solana/config.json (Solana program deployment configuration)</code> |

## solana

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.6.go-bridge-vectors-address20-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>type Address20 [20]byte</code> |
| <code>interface.6.go-bridge-vectors-chainidfromlabel-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>func ChainIDFromLabel(label string) (uint64, error)</code> |
| <code>interface.6.go-bridge-vectors-createprogramaddress-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>func CreateProgramAddress(programID Key32, seeds [][]byte, bump uint8) (Key32, error)</code> |
| <code>interface.6.go-bridge-vectors-decodebase58-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>func DecodeBase58(encoded string) ([]byte, error)</code> |
| <code>interface.6.go-bridge-vectors-digest-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>type Digest [32]byte</code> |
| <code>interface.6.go-bridge-vectors-encodebase58-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>func EncodeBase58(raw []byte) string</code> |
| <code>interface.6.go-bridge-vectors-findprogramaddress-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>func FindProgramAddress(programID Key32, seeds [][]byte) (Key32, uint8, error)</code> |
| <code>interface.6.go-bridge-vectors-handle-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>func Handle(key Key32) Address20</code> |
| <code>interface.6.go-bridge-vectors-inbound-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>type Inbound struct</code> |
| <code>interface.6.go-bridge-vectors-inboundtxhash-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>func InboundTxHash(signature Signature64) Digest</code> |
| <code>interface.6.go-bridge-vectors-inboundvector-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>var InboundVector = Inbound</code> |
| <code>interface.6.go-bridge-vectors-key-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>func Key(address string) (Key32, error)</code> |
| <code>interface.6.go-bridge-vectors-key32-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>type Key32 [32]byte</code> |
| <code>interface.6.go-bridge-vectors-outbound-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>type Outbound struct</code> |
| <code>interface.6.go-bridge-vectors-outboundvector-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>var OutboundVector = Outbound</code> |
| <code>interface.6.go-bridge-vectors-paxeerrecipient-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>func PaxeerRecipient(address Address20) Digest</code> |
| <code>interface.6.go-bridge-vectors-sidioraassetid-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>SidioraAssetID = mustAddress(SidioraAssetIDHex)</code> |
| <code>interface.6.go-bridge-vectors-sidioraassetidhex-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>SidioraAssetIDHex = "21f7b20a555199fa73A238B1a91FD0f549068fEe"</code> |
| <code>interface.6.go-bridge-vectors-sidioradecimals-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>SidioraDecimals uint8 = 6</code> |
| <code>interface.6.go-bridge-vectors-sidioramint-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>SidioraMint = mustKey(SidioraMintBase58)</code> |
| <code>interface.6.go-bridge-vectors-sidioramintbase58-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>SidioraMintBase58 = "5w3wVdJaESaJKyLmStM6Hv9UyUkmZ1b9DLQquAqqpump"</code> |
| <code>interface.6.go-bridge-vectors-signature64-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>type Signature64 [64]byte</code> |
| <code>interface.6.go-bridge-vectors-solanachainid-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>SolanaChainID uint64 = 91600046870081</code> |
| <code>interface.6.go-bridge-vectors-solanachainidlabel-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>SolanaChainIDLabel = "SOLANA"</code> |
| <code>interface.6.go-bridge-vectors-vaultauthority-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>func VaultAuthority(programID Key32) (Key32, uint8, error)</code> |
| <code>interface.6.go-bridge-vectors-vaultauthorityseed-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>VaultAuthoritySeed = "vault-authority"</code> |
| <code>interface.6.go-bridge-vectors-vaulthandle-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>func VaultHandle(programID Key32) (Address20, error)</code> |
| <code>interface.6.go-bridge-vectors-vectordepositsignature-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>VectorDepositSignature = mustSignature(</code> |
| <code>interface.6.go-bridge-vectors-vectordepositsignaturelabel-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>VectorDepositSignatureLabel = "PAXEERX_BRIDGE_SOLANA_VECTOR_DEPOSIT_SIGNATURE"</code> |
| <code>interface.6.go-bridge-vectors-vectorpaxeerburnlabel-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>VectorPaxeerBurnLabel = "PAXEERX_BRIDGE_SOLANA_VECTOR_BURN"</code> |
| <code>interface.6.go-bridge-vectors-vectorpaxeerrecipientlabel-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>VectorPaxeerRecipientLabel = "PAXEERX_BRIDGE_SOLANA_VECTOR_PAXEER_RECIPIENT"</code> |
| <code>interface.6.go-bridge-vectors-vectorprogramid-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>VectorProgramID = mustKeyHex("875f91f2b7ddd837d8584366fa8d7a8c73905b2212713a4826d4a89faa8d40fc")</code> |
| <code>interface.6.go-bridge-vectors-vectorprogramidlabel-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>VectorProgramIDLabel = "PAXEERX_BRIDGE_SOLANA_VECTOR_PROGRAM"</code> |
| <code>interface.6.go-bridge-vectors-vectorsolanarecipientlabel-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>VectorSolanaRecipientLabel = "PAXEERX_BRIDGE_SOLANA_VECTOR_RECIPIENT"</code> |
| <code>interface.6.go-bridge-vectors-vectorvaultauthority-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>VectorVaultAuthority = mustKeyHex("ed349d8496a5f534b3bb8f5d240d8633acf502427eca37504ec7cf68766aa71d")</code> |
| <code>interface.6.go-bridge-vectors-vectorvaultauthoritybump-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>const VectorVaultAuthorityBump uint8 = 255</code> |
| <code>interface.6.go-bridge-vectors-vectorvaulthandle-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>VectorVaultHandle = mustAddress("334121a65b47bd45c3f6381537d9180e98e445bc")</code> |
| <code>interface.6.go-bridge-vectors-wrappedsolassetid-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>WrappedSolAssetID = mustAddress("cf996523b5d068a26f0aa8a116602fe5033ee3a1")</code> |
| <code>interface.6.go-bridge-vectors-wrappedsoldecimals-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>WrappedSolDecimals uint8 = 9</code> |
| <code>interface.6.go-bridge-vectors-wrappedsolmint-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>WrappedSolMint = mustKey(WrappedSolMintBase58)</code> |
| <code>interface.6.go-bridge-vectors-wrappedsolmintbase58-j36</code> | <code>go_exported</code> | <code>contracts/bridge/vectors/solana.go</code> | <code>WrappedSolMintBase58 = "So11111111111111111111111111111111111111112"</code> |
