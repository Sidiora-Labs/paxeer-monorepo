# Bridge solana contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## admin-cli

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.cli-solana-bridge-admin-apply-j36</code> | <code>cli_command</code> | <code>stable</code> | <code>contracts/bridge/solana/admin/src/main.rs:617</code> | <code>paxeer-x-bridge-solana-admin apply</code> |
| <code>interface.6.cli-solana-bridge-admin-initialise-j36</code> | <code>cli_command</code> | <code>stable</code> | <code>contracts/bridge/solana/admin/src/main.rs:618</code> | <code>paxeer-x-bridge-solana-admin initialise</code> |
| <code>interface.6.cli-solana-bridge-admin-pause-j36</code> | <code>cli_command</code> | <code>stable</code> | <code>contracts/bridge/solana/admin/src/main.rs:621</code> | <code>paxeer-x-bridge-solana-admin pause</code> |
| <code>interface.6.cli-solana-bridge-admin-register-asset-j36</code> | <code>cli_command</code> | <code>stable</code> | <code>contracts/bridge/solana/admin/src/main.rs:619</code> | <code>paxeer-x-bridge-solana-admin register-asset</code> |
| <code>interface.6.cli-solana-bridge-admin-register-recipient-j36</code> | <code>cli_command</code> | <code>stable</code> | <code>contracts/bridge/solana/admin/src/main.rs:623</code> | <code>paxeer-x-bridge-solana-admin register-recipient</code> |
| <code>interface.6.cli-solana-bridge-admin-set-cap-j36</code> | <code>cli_command</code> | <code>stable</code> | <code>contracts/bridge/solana/admin/src/main.rs:620</code> | <code>paxeer-x-bridge-solana-admin set-cap</code> |
| <code>interface.6.cli-solana-bridge-admin-show-j36</code> | <code>cli_command</code> | <code>stable</code> | <code>contracts/bridge/solana/admin/src/main.rs:624</code> | <code>paxeer-x-bridge-solana-admin show</code> |
| <code>interface.6.cli-solana-bridge-admin-unpause-j36</code> | <code>cli_command</code> | <code>stable</code> | <code>contracts/bridge/solana/admin/src/main.rs:622</code> | <code>paxeer-x-bridge-solana-admin unpause</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.cli-solana-bridge-admin-apply-j36</code> | notes | no in-repo script invokes the admin binary; contracts/bridge/solana/admin/tests/admin.rs exercises it |
| <code>interface.6.cli-solana-bridge-admin-initialise-j36</code> | notes | no in-repo script invokes the admin binary; contracts/bridge/solana/admin/tests/admin.rs exercises it |
| <code>interface.6.cli-solana-bridge-admin-pause-j36</code> | notes | no in-repo script invokes the admin binary; contracts/bridge/solana/admin/tests/admin.rs exercises it |
| <code>interface.6.cli-solana-bridge-admin-register-asset-j36</code> | notes | no in-repo script invokes the admin binary; contracts/bridge/solana/admin/tests/admin.rs exercises it |
| <code>interface.6.cli-solana-bridge-admin-register-recipient-j36</code> | notes | no in-repo script invokes the admin binary; contracts/bridge/solana/admin/tests/admin.rs exercises it |
| <code>interface.6.cli-solana-bridge-admin-set-cap-j36</code> | notes | no in-repo script invokes the admin binary; contracts/bridge/solana/admin/tests/admin.rs exercises it |
| <code>interface.6.cli-solana-bridge-admin-show-j36</code> | notes | no in-repo script invokes the admin binary; contracts/bridge/solana/admin/tests/admin.rs exercises it |
| <code>interface.6.cli-solana-bridge-admin-unpause-j36</code> | notes | no in-repo script invokes the admin binary; contracts/bridge/solana/admin/tests/admin.rs exercises it |

## errors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.err-solana-bridge-account-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:82</code> | <code>BridgeError::Account = ProgramError::Custom(12)</code> |
| <code>interface.6.err-solana-bridge-asset-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:74</code> | <code>BridgeError::Asset = ProgramError::Custom(9)</code> |
| <code>interface.6.err-solana-bridge-attestationmalformed-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:95</code> | <code>BridgeError::AttestationMalformed = ProgramError::Custom(17)</code> |
| <code>interface.6.err-solana-bridge-attestationmalleable-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:105</code> | <code>BridgeError::AttestationMalleable = ProgramError::Custom(21)</code> |
| <code>interface.6.err-solana-bridge-attestationmessage-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:98</code> | <code>BridgeError::AttestationMessage = ProgramError::Custom(18)</code> |
| <code>interface.6.err-solana-bridge-attestationmissing-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:92</code> | <code>BridgeError::AttestationMissing = ProgramError::Custom(16)</code> |
| <code>interface.6.err-solana-bridge-attestationsigner-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:103</code> | <code>BridgeError::AttestationSigner = ProgramError::Custom(20)</code> |
| <code>interface.6.err-solana-bridge-attestationthreshold-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:100</code> | <code>BridgeError::AttestationThreshold = ProgramError::Custom(19)</code> |
| <code>interface.6.err-solana-bridge-attestors-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:72</code> | <code>BridgeError::Attestors = ProgramError::Custom(8)</code> |
| <code>interface.6.err-solana-bridge-authority-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:59</code> | <code>BridgeError::Authority = ProgramError::Custom(2)</code> |
| <code>interface.6.err-solana-bridge-binding-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:85</code> | <code>BridgeError::Binding = ProgramError::Custom(13)</code> |
| <code>interface.6.err-solana-bridge-bounds-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:69</code> | <code>BridgeError::Bounds = ProgramError::Custom(7)</code> |
| <code>interface.6.err-solana-bridge-cap-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:76</code> | <code>BridgeError::Cap = ProgramError::Custom(10)</code> |
| <code>interface.6.err-solana-bridge-conflict-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:63</code> | <code>BridgeError::Conflict = ProgramError::Custom(4)</code> |
| <code>interface.6.err-solana-bridge-instruction-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:57</code> | <code>BridgeError::Instruction = ProgramError::Custom(1)</code> |
| <code>interface.6.err-solana-bridge-notinitialised-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:65</code> | <code>BridgeError::NotInitialised = ProgramError::Custom(5)</code> |
| <code>interface.6.err-solana-bridge-outstanding-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:89</code> | <code>BridgeError::Outstanding = ProgramError::Custom(15)</code> |
| <code>interface.6.err-solana-bridge-paused-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:67</code> | <code>BridgeError::Paused = ProgramError::Custom(6)</code> |
| <code>interface.6.err-solana-bridge-pda-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:61</code> | <code>BridgeError::Pda = ProgramError::Custom(3)</code> |
| <code>interface.6.err-solana-bridge-recipient-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:80</code> | <code>BridgeError::Recipient = ProgramError::Custom(11)</code> |
| <code>interface.6.err-solana-bridge-replayed-j36</code> | <code>error_code</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:87</code> | <code>BridgeError::Replayed = ProgramError::Custom(14)</code> |

## instructions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.fmt-solana-bridge-instruction-magic-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:36</code> | <code>pub const INSTRUCTION_MAGIC: &amp;[u8; 4] = b"PXBR";</code> |
| <code>interface.6.fmt-solana-bridge-instruction-version-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:38</code> | <code>pub const INSTRUCTION_VERSION: u16 = 1;</code> |
| <code>interface.6.fmt-solana-bridge-op-accept-owner-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:42</code> | <code>pub const OP_ACCEPT_OWNER: u8 = 3;</code> |
| <code>interface.6.fmt-solana-bridge-op-deposit-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:47</code> | <code>pub const OP_DEPOSIT: u8 = 8;</code> |
| <code>interface.6.fmt-solana-bridge-op-initialise-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:40</code> | <code>pub const OP_INITIALISE: u8 = 1;</code> |
| <code>interface.6.fmt-solana-bridge-op-propose-owner-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:41</code> | <code>pub const OP_PROPOSE_OWNER: u8 = 2;</code> |
| <code>interface.6.fmt-solana-bridge-op-register-asset-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:44</code> | <code>pub const OP_REGISTER_ASSET: u8 = 5;</code> |
| <code>interface.6.fmt-solana-bridge-op-register-recipient-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:48</code> | <code>pub const OP_REGISTER_RECIPIENT: u8 = 9;</code> |
| <code>interface.6.fmt-solana-bridge-op-release-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:49</code> | <code>pub const OP_RELEASE: u8 = 10;</code> |
| <code>interface.6.fmt-solana-bridge-op-set-attestors-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:43</code> | <code>pub const OP_SET_ATTESTORS: u8 = 4;</code> |
| <code>interface.6.fmt-solana-bridge-op-set-cap-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:45</code> | <code>pub const OP_SET_CAP: u8 = 6;</code> |
| <code>interface.6.fmt-solana-bridge-op-set-pause-j36</code> | <code>file_format</code> | <code>stable</code> | <code>contracts/bridge/solana/src/lib.rs:46</code> | <code>pub const OP_SET_PAUSE: u8 = 7;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.fmt-solana-bridge-instruction-magic-j36</code> | notes | instruction payloads start with the magic, the u16 version and the opcode byte |
| <code>interface.6.fmt-solana-bridge-instruction-version-j36</code> | notes | instruction payloads start with the magic, the u16 version and the opcode byte |
| <code>interface.6.fmt-solana-bridge-op-accept-owner-j36</code> | notes | instruction payloads start with the magic, the u16 version and the opcode byte |
| <code>interface.6.fmt-solana-bridge-op-deposit-j36</code> | notes | instruction payloads start with the magic, the u16 version and the opcode byte |
| <code>interface.6.fmt-solana-bridge-op-initialise-j36</code> | notes | instruction payloads start with the magic, the u16 version and the opcode byte |
| <code>interface.6.fmt-solana-bridge-op-propose-owner-j36</code> | notes | instruction payloads start with the magic, the u16 version and the opcode byte |
| <code>interface.6.fmt-solana-bridge-op-register-asset-j36</code> | notes | instruction payloads start with the magic, the u16 version and the opcode byte |
| <code>interface.6.fmt-solana-bridge-op-register-recipient-j36</code> | notes | instruction payloads start with the magic, the u16 version and the opcode byte |
| <code>interface.6.fmt-solana-bridge-op-release-j36</code> | notes | instruction payloads start with the magic, the u16 version and the opcode byte |
| <code>interface.6.fmt-solana-bridge-op-set-attestors-j36</code> | notes | instruction payloads start with the magic, the u16 version and the opcode byte |
| <code>interface.6.fmt-solana-bridge-op-set-cap-j36</code> | notes | instruction payloads start with the magic, the u16 version and the opcode byte |
| <code>interface.6.fmt-solana-bridge-op-set-pause-j36</code> | notes | instruction payloads start with the magic, the u16 version and the opcode byte |

## state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.rs-solana-bridge-asset-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/state.rs:165</code> | <code>pub struct Asset</code> |
| <code>interface.6.rs-solana-bridge-config-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/state.rs:52</code> | <code>pub struct Config</code> |
| <code>interface.6.rs-solana-bridge-find-asset-address-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/state.rs:323</code> | <code>pub fn find_asset_address(program_id: &amp;Pubkey, mint: &amp;Pubkey) -&gt; (Pubkey, u8)</code> |
| <code>interface.6.rs-solana-bridge-find-config-address-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/state.rs:435</code> | <code>pub fn find_config_address(program_id: &amp;Pubkey) -&gt; (Pubkey, u8)</code> |
| <code>interface.6.rs-solana-bridge-find-recipient-address-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/state.rs:425</code> | <code>pub fn find_recipient_address(program_id: &amp;Pubkey, handle: &amp;[u8; HANDLE_BYTES]) -&gt; (Pubkey, u8)</code> |
| <code>interface.6.rs-solana-bridge-find-vault-authority-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/identity.rs:58</code> | <code>pub fn find_vault_authority(program_id: &amp;Pubkey) -&gt; (Pubkey, u8)</code> |
| <code>interface.6.rs-solana-bridge-handle-bytes-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/identity.rs:23</code> | <code>pub const HANDLE_BYTES: usize = 20;</code> |
| <code>interface.6.rs-solana-bridge-hex-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/identity.rs:76</code> | <code>pub fn hex(bytes: &amp;[u8]) -&gt; String</code> |
| <code>interface.6.rs-solana-bridge-max-attestors-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/state.rs:33</code> | <code>pub const MAX_ATTESTORS: usize = 64;</code> |
| <code>interface.6.rs-solana-bridge-pubkey-handle-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/identity.rs:51</code> | <code>pub fn pubkey_handle(key: &amp;Pubkey) -&gt; [u8; HANDLE_BYTES]</code> |
| <code>interface.6.rs-solana-bridge-recipientrecord-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/state.rs:331</code> | <code>pub struct RecipientRecord</code> |
| <code>interface.6.rs-solana-bridge-sidiora-asset-id-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/identity.rs:37</code> | <code>pub const SIDIORA_ASSET_ID: [u8; HANDLE_BYTES] = [</code> |
| <code>interface.6.rs-solana-bridge-sidiora-mint-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/identity.rs:32</code> | <code>pub const SIDIORA_MINT: Pubkey =</code> |
| <code>interface.6.rs-solana-bridge-solana-chain-id-j36</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>contracts/bridge/solana/src/identity.rs:28</code> | <code>pub const SOLANA_CHAIN_ID: u64 = 91_600_046_870_081;</code> |
