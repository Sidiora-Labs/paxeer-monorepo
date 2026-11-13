# Bridge solana contract

Source paths are relative to the repository root. Declaration tables identify the interfaces in the source files; runtime behavior is defined by their implementations.

## admin-cli

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.6.cli-solana-bridge-admin-apply-j36</code> | <code>cli_command</code> | <code>contracts/bridge/solana/admin/src/main.rs</code> | <code>paxeer-x-bridge-solana-admin apply</code> |
| <code>interface.6.cli-solana-bridge-admin-initialise-j36</code> | <code>cli_command</code> | <code>contracts/bridge/solana/admin/src/main.rs</code> | <code>paxeer-x-bridge-solana-admin initialise</code> |
| <code>interface.6.cli-solana-bridge-admin-pause-j36</code> | <code>cli_command</code> | <code>contracts/bridge/solana/admin/src/main.rs</code> | <code>paxeer-x-bridge-solana-admin pause</code> |
| <code>interface.6.cli-solana-bridge-admin-register-asset-j36</code> | <code>cli_command</code> | <code>contracts/bridge/solana/admin/src/main.rs</code> | <code>paxeer-x-bridge-solana-admin register-asset</code> |
| <code>interface.6.cli-solana-bridge-admin-register-recipient-j36</code> | <code>cli_command</code> | <code>contracts/bridge/solana/admin/src/main.rs</code> | <code>paxeer-x-bridge-solana-admin register-recipient</code> |
| <code>interface.6.cli-solana-bridge-admin-set-cap-j36</code> | <code>cli_command</code> | <code>contracts/bridge/solana/admin/src/main.rs</code> | <code>paxeer-x-bridge-solana-admin set-cap</code> |
| <code>interface.6.cli-solana-bridge-admin-show-j36</code> | <code>cli_command</code> | <code>contracts/bridge/solana/admin/src/main.rs</code> | <code>paxeer-x-bridge-solana-admin show</code> |
| <code>interface.6.cli-solana-bridge-admin-unpause-j36</code> | <code>cli_command</code> | <code>contracts/bridge/solana/admin/src/main.rs</code> | <code>paxeer-x-bridge-solana-admin unpause</code> |

## errors

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.6.err-solana-bridge-account-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Account = ProgramError::Custom(12)</code> |
| <code>interface.6.err-solana-bridge-asset-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Asset = ProgramError::Custom(9)</code> |
| <code>interface.6.err-solana-bridge-attestationmalformed-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::AttestationMalformed = ProgramError::Custom(17)</code> |
| <code>interface.6.err-solana-bridge-attestationmalleable-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::AttestationMalleable = ProgramError::Custom(21)</code> |
| <code>interface.6.err-solana-bridge-attestationmessage-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::AttestationMessage = ProgramError::Custom(18)</code> |
| <code>interface.6.err-solana-bridge-attestationmissing-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::AttestationMissing = ProgramError::Custom(16)</code> |
| <code>interface.6.err-solana-bridge-attestationsigner-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::AttestationSigner = ProgramError::Custom(20)</code> |
| <code>interface.6.err-solana-bridge-attestationthreshold-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::AttestationThreshold = ProgramError::Custom(19)</code> |
| <code>interface.6.err-solana-bridge-attestors-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Attestors = ProgramError::Custom(8)</code> |
| <code>interface.6.err-solana-bridge-authority-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Authority = ProgramError::Custom(2)</code> |
| <code>interface.6.err-solana-bridge-binding-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Binding = ProgramError::Custom(13)</code> |
| <code>interface.6.err-solana-bridge-bounds-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Bounds = ProgramError::Custom(7)</code> |
| <code>interface.6.err-solana-bridge-cap-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Cap = ProgramError::Custom(10)</code> |
| <code>interface.6.err-solana-bridge-conflict-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Conflict = ProgramError::Custom(4)</code> |
| <code>interface.6.err-solana-bridge-instruction-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Instruction = ProgramError::Custom(1)</code> |
| <code>interface.6.err-solana-bridge-notinitialised-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::NotInitialised = ProgramError::Custom(5)</code> |
| <code>interface.6.err-solana-bridge-outstanding-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Outstanding = ProgramError::Custom(15)</code> |
| <code>interface.6.err-solana-bridge-paused-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Paused = ProgramError::Custom(6)</code> |
| <code>interface.6.err-solana-bridge-pda-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Pda = ProgramError::Custom(3)</code> |
| <code>interface.6.err-solana-bridge-recipient-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Recipient = ProgramError::Custom(11)</code> |
| <code>interface.6.err-solana-bridge-replayed-j36</code> | <code>error_code</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>BridgeError::Replayed = ProgramError::Custom(14)</code> |

## instructions

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.6.fmt-solana-bridge-instruction-magic-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>pub const INSTRUCTION_MAGIC: &amp;[u8; 4] = b"PXBR";</code> |
| <code>interface.6.fmt-solana-bridge-instruction-version-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>pub const INSTRUCTION_VERSION: u16 = 1;</code> |
| <code>interface.6.fmt-solana-bridge-op-accept-owner-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>pub const OP_ACCEPT_OWNER: u8 = 3;</code> |
| <code>interface.6.fmt-solana-bridge-op-deposit-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>pub const OP_DEPOSIT: u8 = 8;</code> |
| <code>interface.6.fmt-solana-bridge-op-initialise-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>pub const OP_INITIALISE: u8 = 1;</code> |
| <code>interface.6.fmt-solana-bridge-op-propose-owner-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>pub const OP_PROPOSE_OWNER: u8 = 2;</code> |
| <code>interface.6.fmt-solana-bridge-op-register-asset-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>pub const OP_REGISTER_ASSET: u8 = 5;</code> |
| <code>interface.6.fmt-solana-bridge-op-register-recipient-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>pub const OP_REGISTER_RECIPIENT: u8 = 9;</code> |
| <code>interface.6.fmt-solana-bridge-op-release-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>pub const OP_RELEASE: u8 = 10;</code> |
| <code>interface.6.fmt-solana-bridge-op-set-attestors-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>pub const OP_SET_ATTESTORS: u8 = 4;</code> |
| <code>interface.6.fmt-solana-bridge-op-set-cap-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>pub const OP_SET_CAP: u8 = 6;</code> |
| <code>interface.6.fmt-solana-bridge-op-set-pause-j36</code> | <code>file_format</code> | <code>contracts/bridge/solana/src/lib.rs</code> | <code>pub const OP_SET_PAUSE: u8 = 7;</code> |

## state

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.6.rs-solana-bridge-asset-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/state.rs</code> | <code>pub struct Asset</code> |
| <code>interface.6.rs-solana-bridge-config-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/state.rs</code> | <code>pub struct Config</code> |
| <code>interface.6.rs-solana-bridge-find-asset-address-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/state.rs</code> | <code>pub fn find_asset_address(program_id: &amp;Pubkey, mint: &amp;Pubkey) -&gt; (Pubkey, u8)</code> |
| <code>interface.6.rs-solana-bridge-find-config-address-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/state.rs</code> | <code>pub fn find_config_address(program_id: &amp;Pubkey) -&gt; (Pubkey, u8)</code> |
| <code>interface.6.rs-solana-bridge-find-recipient-address-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/state.rs</code> | <code>pub fn find_recipient_address(program_id: &amp;Pubkey, handle: &amp;[u8; HANDLE_BYTES]) -&gt; (Pubkey, u8)</code> |
| <code>interface.6.rs-solana-bridge-find-vault-authority-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/identity.rs</code> | <code>pub fn find_vault_authority(program_id: &amp;Pubkey) -&gt; (Pubkey, u8)</code> |
| <code>interface.6.rs-solana-bridge-handle-bytes-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/identity.rs</code> | <code>pub const HANDLE_BYTES: usize = 20;</code> |
| <code>interface.6.rs-solana-bridge-hex-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/identity.rs</code> | <code>pub fn hex(bytes: &amp;[u8]) -&gt; String</code> |
| <code>interface.6.rs-solana-bridge-max-attestors-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/state.rs</code> | <code>pub const MAX_ATTESTORS: usize = 64;</code> |
| <code>interface.6.rs-solana-bridge-pubkey-handle-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/identity.rs</code> | <code>pub fn pubkey_handle(key: &amp;Pubkey) -&gt; [u8; HANDLE_BYTES]</code> |
| <code>interface.6.rs-solana-bridge-recipientrecord-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/state.rs</code> | <code>pub struct RecipientRecord</code> |
| <code>interface.6.rs-solana-bridge-sidiora-asset-id-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/identity.rs</code> | <code>pub const SIDIORA_ASSET_ID: [u8; HANDLE_BYTES] = [</code> |
| <code>interface.6.rs-solana-bridge-sidiora-mint-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/identity.rs</code> | <code>pub const SIDIORA_MINT: Pubkey =</code> |
| <code>interface.6.rs-solana-bridge-solana-chain-id-j36</code> | <code>rust_pub_item</code> | <code>contracts/bridge/solana/src/identity.rs</code> | <code>pub const SOLANA_CHAIN_ID: u64 = 91_600_046_870_081;</code> |
