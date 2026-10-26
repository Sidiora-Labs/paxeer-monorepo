# Bridge vectors contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## chain-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.fmt-bridge-evm-chain-config-arbitrum-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/evm/chains/arbitrum/config.json:1</code> | <code>contracts/bridge/evm/chains/arbitrum/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-avalanche-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/evm/chains/avalanche/config.json:1</code> | <code>contracts/bridge/evm/chains/avalanche/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-base-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/evm/chains/base/config.json:1</code> | <code>contracts/bridge/evm/chains/base/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-bnb-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/evm/chains/bnb/config.json:1</code> | <code>contracts/bridge/evm/chains/bnb/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-ethereum-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/evm/chains/ethereum/config.json:1</code> | <code>contracts/bridge/evm/chains/ethereum/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-hyperevm-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/evm/chains/hyperevm/config.json:1</code> | <code>contracts/bridge/evm/chains/hyperevm/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-optimism-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/evm/chains/optimism/config.json:1</code> | <code>contracts/bridge/evm/chains/optimism/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-evm-chain-config-polygon-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/evm/chains/polygon/config.json:1</code> | <code>contracts/bridge/evm/chains/polygon/config.json (PaxeerXVault deployment configuration)</code> |
| <code>interface.6.fmt-bridge-solana-chain-config-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/chains/solana/config.json:1</code> | <code>contracts/bridge/solana/chains/solana/config.json (Solana program deployment configuration)</code> |

## solana

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-bridge-vectors-address20-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:38</code> | <code>type Address20 [20]byte</code> |
| <code>interface.6.go-bridge-vectors-chainidfromlabel-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:296</code> | <code>func ChainIDFromLabel(label string) (uint64, error)</code> |
| <code>interface.6.go-bridge-vectors-createprogramaddress-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:404</code> | <code>func CreateProgramAddress(programID Key32, seeds [][]byte, bump uint8) (Key32, error)</code> |
| <code>interface.6.go-bridge-vectors-decodebase58-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:353</code> | <code>func DecodeBase58(encoded string) ([]byte, error)</code> |
| <code>interface.6.go-bridge-vectors-digest-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:41</code> | <code>type Digest [32]byte</code> |
| <code>interface.6.go-bridge-vectors-encodebase58-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:379</code> | <code>func EncodeBase58(raw []byte) string</code> |
| <code>interface.6.go-bridge-vectors-findprogramaddress-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:428</code> | <code>func FindProgramAddress(programID Key32, seeds [][]byte) (Key32, uint8, error)</code> |
| <code>interface.6.go-bridge-vectors-handle-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:269</code> | <code>func Handle(key Key32) Address20</code> |
| <code>interface.6.go-bridge-vectors-inbound-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:171</code> | <code>type Inbound struct</code> |
| <code>interface.6.go-bridge-vectors-inboundtxhash-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:279</code> | <code>func InboundTxHash(signature Signature64) Digest</code> |
| <code>interface.6.go-bridge-vectors-inboundvector-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:233</code> | <code>var InboundVector = Inbound</code> |
| <code>interface.6.go-bridge-vectors-key-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:326</code> | <code>func Key(address string) (Key32, error)</code> |
| <code>interface.6.go-bridge-vectors-key32-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:34</code> | <code>type Key32 [32]byte</code> |
| <code>interface.6.go-bridge-vectors-outbound-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:201</code> | <code>type Outbound struct</code> |
| <code>interface.6.go-bridge-vectors-outboundvector-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:250</code> | <code>var OutboundVector = Outbound</code> |
| <code>interface.6.go-bridge-vectors-paxeerrecipient-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:287</code> | <code>func PaxeerRecipient(address Address20) Digest</code> |
| <code>interface.6.go-bridge-vectors-sidioraassetid-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:140</code> | <code>SidioraAssetID = mustAddress(SidioraAssetIDHex)</code> |
| <code>interface.6.go-bridge-vectors-sidioraassetidhex-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:81</code> | <code>SidioraAssetIDHex = "21f7b20a555199fa73A238B1a91FD0f549068fEe"</code> |
| <code>interface.6.go-bridge-vectors-sidioradecimals-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:68</code> | <code>SidioraDecimals uint8 = 6</code> |
| <code>interface.6.go-bridge-vectors-sidioramint-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:128</code> | <code>SidioraMint = mustKey(SidioraMintBase58)</code> |
| <code>interface.6.go-bridge-vectors-sidioramintbase58-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:64</code> | <code>SidioraMintBase58 = "5w3wVdJaESaJKyLmStM6Hv9UyUkmZ1b9DLQquAqqpump"</code> |
| <code>interface.6.go-bridge-vectors-signature64-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:44</code> | <code>type Signature64 [64]byte</code> |
| <code>interface.6.go-bridge-vectors-solanachainid-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:55</code> | <code>SolanaChainID uint64 = 91600046870081</code> |
| <code>interface.6.go-bridge-vectors-solanachainidlabel-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:49</code> | <code>SolanaChainIDLabel = "SOLANA"</code> |
| <code>interface.6.go-bridge-vectors-vaultauthority-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:444</code> | <code>func VaultAuthority(programID Key32) (Key32, uint8, error)</code> |
| <code>interface.6.go-bridge-vectors-vaultauthorityseed-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:60</code> | <code>VaultAuthoritySeed = "vault-authority"</code> |
| <code>interface.6.go-bridge-vectors-vaulthandle-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:450</code> | <code>func VaultHandle(programID Key32) (Address20, error)</code> |
| <code>interface.6.go-bridge-vectors-vectordepositsignature-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:159</code> | <code>VectorDepositSignature = mustSignature(</code> |
| <code>interface.6.go-bridge-vectors-vectordepositsignaturelabel-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:98</code> | <code>VectorDepositSignatureLabel = "PAXEERX_BRIDGE_SOLANA_VECTOR_DEPOSIT_SIGNATURE"</code> |
| <code>interface.6.go-bridge-vectors-vectorpaxeerburnlabel-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:110</code> | <code>VectorPaxeerBurnLabel = "PAXEERX_BRIDGE_SOLANA_VECTOR_BURN"</code> |
| <code>interface.6.go-bridge-vectors-vectorpaxeerrecipientlabel-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:102</code> | <code>VectorPaxeerRecipientLabel = "PAXEERX_BRIDGE_SOLANA_VECTOR_PAXEER_RECIPIENT"</code> |
| <code>interface.6.go-bridge-vectors-vectorprogramid-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:148</code> | <code>VectorProgramID = mustKeyHex("875f91f2b7ddd837d8584366fa8d7a8c73905b2212713a4826d4a89faa8d40fc")</code> |
| <code>interface.6.go-bridge-vectors-vectorprogramidlabel-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:94</code> | <code>VectorProgramIDLabel = "PAXEERX_BRIDGE_SOLANA_VECTOR_PROGRAM"</code> |
| <code>interface.6.go-bridge-vectors-vectorsolanarecipientlabel-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:106</code> | <code>VectorSolanaRecipientLabel = "PAXEERX_BRIDGE_SOLANA_VECTOR_RECIPIENT"</code> |
| <code>interface.6.go-bridge-vectors-vectorvaultauthority-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:151</code> | <code>VectorVaultAuthority = mustKeyHex("ed349d8496a5f534b3bb8f5d240d8633acf502427eca37504ec7cf68766aa71d")</code> |
| <code>interface.6.go-bridge-vectors-vectorvaultauthoritybump-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:167</code> | <code>const VectorVaultAuthorityBump uint8 = 255</code> |
| <code>interface.6.go-bridge-vectors-vectorvaulthandle-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:155</code> | <code>VectorVaultHandle = mustAddress("334121a65b47bd45c3f6381537d9180e98e445bc")</code> |
| <code>interface.6.go-bridge-vectors-wrappedsolassetid-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:144</code> | <code>WrappedSolAssetID = mustAddress("cf996523b5d068a26f0aa8a116602fe5033ee3a1")</code> |
| <code>interface.6.go-bridge-vectors-wrappedsoldecimals-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:76</code> | <code>WrappedSolDecimals uint8 = 9</code> |
| <code>interface.6.go-bridge-vectors-wrappedsolmint-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:131</code> | <code>WrappedSolMint = mustKey(WrappedSolMintBase58)</code> |
| <code>interface.6.go-bridge-vectors-wrappedsolmintbase58-j36</code> | <code>go_exported</code> | <code>stable</code> | <code>contracts/bridge/vectors/solana.go:73</code> | <code>WrappedSolMintBase58 = "So11111111111111111111111111111111111111112"</code> |
