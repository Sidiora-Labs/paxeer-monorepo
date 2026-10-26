# Tokenfactory contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cli

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.cli-tokenfactory-query-denom-allow-list-j35</code> | <code>cli_command</code> | <code>new</code> | <code>modules/tokenfactory/client/cli/query.go</code> (planned) | <code>paxd query tokenfactory denom-allow-list [denom]</code> |
| <code>interface.6.cli-tokenfactory-query-denom-metadata-j35</code> | <code>cli_command</code> | <code>new</code> | <code>modules/tokenfactory/client/cli/query.go</code> (planned) | <code>paxd query tokenfactory denom-metadata [denom]</code> |
| <code>interface.6.cli-tokenfactory-query-denom-paused-j35</code> | <code>cli_command</code> | <code>new</code> | <code>modules/tokenfactory/client/cli/query.go</code> (planned) | <code>paxd query tokenfactory denom-paused [denom]</code> |
| <code>interface.6.cli-tokenfactory-query-root</code> | <code>cli_command</code> | <code>change</code> | <code>modules/tokenfactory/client/cli/query.go:16</code> | <code>paxd query tokenfactory</code> |
| <code>interface.6.cli-tokenfactory-tx-root</code> | <code>cli_command</code> | <code>change</code> | <code>modules/tokenfactory/client/cli/tx.go:34</code> | <code>paxd tx tokenfactory</code> |
| <code>interface.6.cli-tokenfactory-tx-set-denom-paused-j35</code> | <code>cli_command</code> | <code>new</code> | <code>modules/tokenfactory/client/cli/tx.go</code> (planned) | <code>paxd tx tokenfactory set-denom-paused [denom] [true&#124;false]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.cli-tokenfactory-query-denom-allow-list-j35</code> | notes | DenomAllowList rpc has no CLI at HEAD |
| <code>interface.6.cli-tokenfactory-query-denom-metadata-j35</code> | notes | DenomMetadata rpc has no CLI at HEAD |
| <code>interface.6.cli-tokenfactory-query-denom-paused-j35</code> | notes | planned GetCmdDenomPaused |
| <code>interface.6.cli-tokenfactory-query-root</code> | notes | Use string at modules/tokenfactory/client/cli/query.go:19 |
| <code>interface.6.cli-tokenfactory-query-root</code> | overridden_by | override.J35.4 |
| <code>interface.6.cli-tokenfactory-tx-root</code> | notes | Use string at modules/tokenfactory/client/cli/tx.go:36 |
| <code>interface.6.cli-tokenfactory-tx-root</code> | overridden_by | override.J35.5 |
| <code>interface.6.cli-tokenfactory-tx-set-denom-paused-j35</code> | notes | planned NewSetDenomPausedCmd |

Contract context <code>contract_context.6.modules-tokenfactory-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 10 surfaces under modules/tokenfactory (4 cli_command, 4 go_exported, 2 wasm_binding; 2 change, 8 new) frozen at the anchors contracts/tokenfactory.md#cli, contracts/tokenfactory.md#invariants, contracts/tokenfactory.md#wasm: paxd query tokenfactory denom-allow-list [denom] (modules/tokenfactory/client/cli/query.go (planned)); paxd query tokenfactory denom-metadata [denom] (modules/tokenfactory/client/cli/query.go (planned)); paxd query tokenfactory denom-paused [denom] (modules/tokenfactory/client/cli/query.go (planned)); paxd tx tokenfactory set-denom-paused [denom] [true&#124;false] (modules/tokenfactory/client/cli/tx.go (planned)); func (am AppModule) RegisterInvariants(_ sdk.InvariantRegistry) (modules/tokenfactory/module.go:163); func EncodeTokenFactorySetDenomPaused(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sd (modules/tokenfactory/client/wasm/encoder.go (planned)); type PaxTokenFactoryQuery struct { DenomAuthorityMetadata; DenomsFromCreator } (modules/tokenfactory/client/wasm/bindings/queries.go:5); func (handler TokenFactoryWasmQueryHandler) GetDenomPaused(ctx sdk.Context, req *types.Que (modules/tokenfactory/client/wasm/query.go (planned)); PaxTokenFactoryQuery.DenomPaused json:"denom_paused" -&gt; QueryDenomPausedResponse (modules/tokenfactory/client/wasm/bindings/queries.go (planned)); PaxWasmMessage.SetDenomPaused json:"set_denom_paused" { denom string; paused bool } (modules/tokenfactory/client/wasm/bindings/msgs.go (planned)) |
| storage | no storage_key item in this context; the modules/tokenfactory items read or write state only through the keepers and stores frozen by the other domain-6 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-6 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-6 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 6.23.3, 6.23.4, 6.23.5, 6.24.2, 6.24.3, 6.26.4 in waves 1, 2, 5 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 6 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.256.1</code> | req | 6256.1 |
| <code>acceptance.6.256.1</code> | scenario | Given the tokenfactory query root command, when denom-paused is looked up and run with zero and with one argument, then zero arguments fail argument validation and one argument builds QueryDenomPausedRequest{Denom: arg} |
| <code>acceptance.6.256.1</code> | test | modules/tokenfactory/client/cli/query_denom_paused_test.go::TestGetCmdDenomPausedArgs (planned) |
| <code>acceptance.6.256.1</code> | qualification | UNRUN |
| <code>req.6256</code> | ac_1 | GetQueryCmd (modules/tokenfactory/client/cli/query.go:16) registers `denom-paused [denom]` built by GetCmdDenomPaused with cobra.ExactArgs(1), beside the commands added at modules/tokenfactory/client/cli/query.go:28-29. |
| <code>acceptance.6.257.1</code> | req | 6257.1 |
| <code>acceptance.6.257.1</code> | scenario | Given the tokenfactory query root command, when denom-metadata is run with one argument against a mock-free in-process network, then it builds QueryDenomMetadataRequest{Denom: arg} and with zero arguments fails validation |
| <code>acceptance.6.257.1</code> | test | modules/tokenfactory/client/cli/query_metadata_cli_test.go::TestGetCmdDenomMetadataArgs (planned) |
| <code>acceptance.6.257.1</code> | qualification | UNRUN |
| <code>req.6257</code> | ac_1 | GetQueryCmd registers `denom-metadata [denom]` calling the DenomMetadata rpc (api/tokenfactory/query.proto:29) that has no CLI at modules/tokenfactory/client/cli/query.go:16-34. |
| <code>acceptance.6.257.2</code> | req | 6257.2 |
| <code>acceptance.6.257.2</code> | scenario | Given the tokenfactory query root command, when denom-allow-list is run with one argument, then it builds QueryDenomAllowListRequest{Denom: arg} and with two arguments fails validation |
| <code>acceptance.6.257.2</code> | test | modules/tokenfactory/client/cli/query_metadata_cli_test.go::TestGetCmdDenomAllowListArgs (planned) |
| <code>acceptance.6.257.2</code> | qualification | UNRUN |
| <code>req.6257</code> | ac_2 | GetQueryCmd registers `denom-allow-list [denom]` calling the DenomAllowList rpc (api/tokenfactory/query.proto:40) that has no CLI today. |
| <code>acceptance.6.261.1</code> | req | 6261.1 |
| <code>acceptance.6.261.1</code> | scenario | Given the tokenfactory tx root command, when set-denom-paused is parsed with factory/&lt;creator&gt;/a true, with factory/&lt;creator&gt;/a maybe and with one argument, then the first builds MsgSetDenomPaused{Denom: factory/&lt;creator&gt;/a, Paused: true} from the from-address, the second fails boolean parsing and the third fails argument validation |
| <code>acceptance.6.261.1</code> | test | modules/tokenfactory/client/cli/tx_set_denom_paused_test.go::TestNewSetDenomPausedCmd (planned) |
| <code>acceptance.6.261.1</code> | qualification | UNRUN |
| <code>req.6261</code> | ac_1 | GetTxCmd (modules/tokenfactory/client/cli/tx.go:34-53) registers `set-denom-paused [denom] [true&#124;false]` built by NewSetDenomPausedCmd that broadcasts NewMsgSetDenomPaused (modules/tokenfactory/types/msgs.go:243). |
| <code>capability.6.23.3</code> | title | Add the denom-paused tokenfactory query CLI command |
| <code>capability.6.23.3</code> | reqs | 6256.1 |
| <code>capability.6.23.4</code> | title | Add denom-metadata and denom-allow-list tokenfactory query CLI commands |
| <code>capability.6.23.4</code> | reqs | 6257.1<br>6257.2 |
| <code>capability.6.24.2</code> | title | Add the set-denom-paused tokenfactory tx CLI command |
| <code>capability.6.24.2</code> | reqs | 6261.1 |
| <code>override.J35.4</code> | target | interface.6.cli-tokenfactory-query-root |
| <code>override.J35.4</code> | status | change |
| <code>override.J35.4</code> | basis | GetQueryCmd at modules/tokenfactory/client/cli/query.go:16-34 registers no command for the DenomMetadata, DenomAllowList or planned DenomPaused queries. |
| <code>override.J35.5</code> | target | interface.6.cli-tokenfactory-tx-root |
| <code>override.J35.5</code> | status | change |
| <code>override.J35.5</code> | basis | GetTxCmd at modules/tokenfactory/client/cli/tx.go:34-53 registers no command for MsgSetDenomPaused (modules/tokenfactory/types/msgs.go:243). |

## errors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.err-sdk-tokenfactory-28-issuer-mismatch-j35</code> | <code>error_code</code> | <code>new</code> | <code>modules/tokenfactory/types/errors.go</code> (planned) | <code>ErrIssuerMismatch = Register("tokenfactory", 28, "LXP_UNAUTHORIZED_DEBIT: caller is not the denom issuer")</code> |
| <code>interface.6.err-sdk-tokenfactory-29-encode-set-denom-paused-j35</code> | <code>error_code</code> | <code>new</code> | <code>modules/tokenfactory/types/errors.go</code> (planned) | <code>ErrEncodeTokenFactorySetDenomPaused = Register("tokenfactory", 29, "Error while encoding tokenfactory set denom paused msg in wasmd")</code> |
| <code>interface.6.err-sdk-tokenfactory-30-encoding-denom-paused-j35</code> | <code>error_code</code> | <code>new</code> | <code>modules/tokenfactory/types/errors.go</code> (planned) | <code>ErrEncodingDenomPaused = Register("tokenfactory", 30, "Error encoding denom paused as JSON")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.err-sdk-tokenfactory-28-issuer-mismatch-j35</code> | notes | kernel refusal of src/modules/asset/lx_asset_execution.h:240-242 |
| <code>interface.6.err-sdk-tokenfactory-29-encode-set-denom-paused-j35</code> | notes | wasm encoder refusal |
| <code>interface.6.err-sdk-tokenfactory-30-encoding-denom-paused-j35</code> | notes | wasm query refusal |

Contract context <code>contract_context.6.tokenfactory-pause-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | tokenfactory Msg SetDenomPaused (api/tokenfactory/tx.proto), planned Query DenomPaused GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused, keeper SetDenomPausedByIssuer, MintByIssuer, BurnByIssuer and the IssuerKeeper interface consumed by layerxledger, CLI set-denom-paused and denom-paused, wasm set_denom_paused and denom_paused |
| storage | Root store of the tokenfactory key: paused&#124;&lt;denom&gt; -&gt; 0x01 (modules/tokenfactory/types/keys.go:55), deleted on unpause; issuer_did inside DenomAuthorityMetadata under denoms&#124;&lt;denom&gt;&#124;authoritymetadata; GenesisState field 3 paused_denoms carries the paused set across export |
| errors | ErrDenomPaused 26 LXP_ASSET_PAUSED, ErrPauseUnchanged 27 LXP_PAUSED_SCOPE (modules/tokenfactory/types/errors.go:34-35), planned ErrIssuerMismatch 28 LXP_UNAUTHORIZED_DEBIT, ErrEncodeTokenFactorySetDenomPaused 29, ErrEncodingDenomPaused 30; bank paths refuse with ErrSendDisabled |
| events | set_denom_paused { denom, paused, issuer_did on the issuer path }, set_issuer_did { denom, issuer_did } |
| gas | No fixed surcharge: Cosmos store gas of one Has per IsDenomPaused and one Set or Delete per pause change |
| activation | Bank msg-server checks are live at HEAD through RegisterSendEnabledChecker (node/app.go:766); the GigaBankKeeper registration and the issuer path land at the merge revision with no height gate because a pause only exists after an explicit pause call |
| invariants | Every paused&#124; key names a denom with authority metadata (PausedDenomsInvariant); pause never changes supply; an issuer-bound denom changes pause only through its issuer DID |
| pause | Per-denom pause refuses MsgSend, MsgMultiSend, GigaBankKeeper SendCoins and SendCoinsAndWei, tokenfactory Mint and Burn and MintByIssuer and BurnByIssuer; keeper-level x/bank SendCoins and module-account flows are not gated (decision.621); there is no governance per-denom pause (decision.623) |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.247.1</code> | req | 6247.1 |
| <code>acceptance.6.247.1</code> | scenario | Given the tokenfactory errors package, when codes 28, 29 and 30 are looked up, then ErrIssuerMismatch, ErrEncodeTokenFactorySetDenomPaused and ErrEncodingDenomPaused carry those ABCI codes in codespace tokenfactory and the code 28 text begins LXP_UNAUTHORIZED_DEBIT |
| <code>acceptance.6.247.1</code> | test | modules/tokenfactory/types/errors_registry_test.go::TestErrorCodesTwentyEightToThirty (planned) |
| <code>acceptance.6.247.1</code> | qualification | UNRUN |
| <code>req.6247</code> | ac_1 | The tokenfactory codespace registers ErrIssuerMismatch code 28 "LXP_UNAUTHORIZED_DEBIT: caller is not the denom issuer", ErrEncodeTokenFactorySetDenomPaused code 29 and ErrEncodingDenomPaused code 30 after ErrPauseUnchanged (modules/tokenfactory/types/errors.go:35). |
| <code>capability.6.22.2</code> | title | Register tokenfactory error codes 28 to 30 for issuer and wasm pause paths |
| <code>capability.6.22.2</code> | reqs | 6247.1 |

## events

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.evt-tokenfactory-attr-paused-j35</code> | <code>event</code> | <code>new</code> | <code>modules/tokenfactory/types/events.go</code> (planned) | <code>AttributePaused = "paused"</code> |
| <code>interface.6.evt-tokenfactory-attr-transfer-from-address-j35</code> | <code>event</code> | <code>retire</code> | <code>modules/tokenfactory/types/events.go:13</code> | <code>AttributeTransferFromAddress = "transfer_from_address"</code> |
| <code>interface.6.evt-tokenfactory-attr-transfer-to-address-j35</code> | <code>event</code> | <code>retire</code> | <code>modules/tokenfactory/types/events.go:14</code> | <code>AttributeTransferToAddress = "transfer_to_address"</code> |
| <code>interface.6.evt-tokenfactory-set-denom-paused</code> | <code>event</code> | <code>change</code> | <code>modules/tokenfactory/types/msgs.go:17</code> | <code>set_denom_paused(denom, paused)</code> |
| <code>interface.6.evt-tokenfactory-set-issuer-did-j35</code> | <code>event</code> | <code>new</code> | <code>modules/tokenfactory/types/events.go</code> (planned) | <code>set_issuer_did { denom, issuer_did }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.evt-tokenfactory-attr-paused-j35</code> | notes | replaces the literal at modules/tokenfactory/keeper/msg_server.go:240 |
| <code>interface.6.evt-tokenfactory-attr-transfer-from-address-j35</code> | notes | no Go reference |
| <code>interface.6.evt-tokenfactory-attr-transfer-to-address-j35</code> | notes | no Go reference |
| <code>interface.6.evt-tokenfactory-set-denom-paused</code> | notes | 1 emit sites; event type reuses the Msg type constant |
| <code>interface.6.evt-tokenfactory-set-denom-paused</code> | overridden_by | override.J35.2 |
| <code>interface.6.evt-tokenfactory-set-issuer-did-j35</code> | notes | emitted by SetIssuerDid on a new binding |

Contract context <code>contract_context.6.tokenfactory-pause-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | tokenfactory Msg SetDenomPaused (api/tokenfactory/tx.proto), planned Query DenomPaused GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused, keeper SetDenomPausedByIssuer, MintByIssuer, BurnByIssuer and the IssuerKeeper interface consumed by layerxledger, CLI set-denom-paused and denom-paused, wasm set_denom_paused and denom_paused |
| storage | Root store of the tokenfactory key: paused&#124;&lt;denom&gt; -&gt; 0x01 (modules/tokenfactory/types/keys.go:55), deleted on unpause; issuer_did inside DenomAuthorityMetadata under denoms&#124;&lt;denom&gt;&#124;authoritymetadata; GenesisState field 3 paused_denoms carries the paused set across export |
| errors | ErrDenomPaused 26 LXP_ASSET_PAUSED, ErrPauseUnchanged 27 LXP_PAUSED_SCOPE (modules/tokenfactory/types/errors.go:34-35), planned ErrIssuerMismatch 28 LXP_UNAUTHORIZED_DEBIT, ErrEncodeTokenFactorySetDenomPaused 29, ErrEncodingDenomPaused 30; bank paths refuse with ErrSendDisabled |
| events | set_denom_paused { denom, paused, issuer_did on the issuer path }, set_issuer_did { denom, issuer_did } |
| gas | No fixed surcharge: Cosmos store gas of one Has per IsDenomPaused and one Set or Delete per pause change |
| activation | Bank msg-server checks are live at HEAD through RegisterSendEnabledChecker (node/app.go:766); the GigaBankKeeper registration and the issuer path land at the merge revision with no height gate because a pause only exists after an explicit pause call |
| invariants | Every paused&#124; key names a denom with authority metadata (PausedDenomsInvariant); pause never changes supply; an issuer-bound denom changes pause only through its issuer DID |
| pause | Per-denom pause refuses MsgSend, MsgMultiSend, GigaBankKeeper SendCoins and SendCoinsAndWei, tokenfactory Mint and Burn and MintByIssuer and BurnByIssuer; keeper-level x/bank SendCoins and module-account flows are not gated (decision.621); there is no governance per-denom pause (decision.623) |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.260.1</code> | req | 6260.1 |
| <code>acceptance.6.260.1</code> | scenario | Given the tokenfactory types package, when the attribute constants are listed, then AttributePaused equals paused, every remaining constant keeps its HEAD string, and no constant named transfer_from_address or transfer_to_address exists |
| <code>acceptance.6.260.1</code> | test | modules/tokenfactory/types/events_test.go::TestEventAttributeNamesFrozen (planned) |
| <code>acceptance.6.260.1</code> | qualification | UNRUN |
| <code>req.6260</code> | ac_1 | The tokenfactory event attribute set is frozen: AttributePaused = "paused" replaces the literal at modules/tokenfactory/keeper/msg_server.go:240, and the unused AttributeTransferFromAddress and AttributeTransferToAddress (modules/tokenfactory/types/events.go:13-14) are removed. |
| <code>capability.6.24.1</code> | title | Add AttributePaused and drop the unused tokenfactory transfer attributes |
| <code>capability.6.24.1</code> | reqs | 6260.1 |
| <code>override.J35.2</code> | target | interface.6.evt-tokenfactory-set-denom-paused |
| <code>override.J35.2</code> | status | change |
| <code>override.J35.2</code> | basis | set_denom_paused is emitted with the literal paused attribute at modules/tokenfactory/keeper/msg_server.go:240 and gains the issuer_did attribute on the issuer path of modules/tokenfactory/keeper/issuer.go (planned). |

## genesis

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.gen-tokenfactory-denom-allowlist-max-size-j35</code> | <code>genesis_param</code> | <code>stable</code> | <code>api/tokenfactory/params.proto:8</code> | <code>uint32 denom_allowlist_max_size = 1 in message paxprotocol.paxchain.tokenfactory.Params</code> |
| <code>interface.6.gen-tokenfactory-paused-denoms-j35</code> | <code>genesis_param</code> | <code>new</code> | <code>api/tokenfactory/genesis.proto:11</code> | <code>repeated string paused_denoms = 3 in message paxprotocol.paxchain.tokenfactory.GenesisState</code> |
| <code>interface.6.go-tokenfactory-exportgenesis-j35</code> | <code>go_exported</code> | <code>change</code> | <code>modules/tokenfactory/keeper/genesis.go:33</code> | <code>func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState</code> |
| <code>interface.6.go-tokenfactory-genesisstate-validate-j35</code> | <code>go_exported</code> | <code>change</code> | <code>modules/tokenfactory/types/genesis.go:23</code> | <code>func (gs GenesisState) Validate() error</code> |
| <code>interface.6.go-tokenfactory-initgenesis-j35</code> | <code>go_exported</code> | <code>change</code> | <code>modules/tokenfactory/keeper/genesis.go:11</code> | <code>func (k Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState)</code> |
| <code>interface.6.proto-tokenfactory-genesisstate</code> | <code>proto_message</code> | <code>change</code> | <code>api/tokenfactory/genesis.proto:11</code> | <code>message paxprotocol.paxchain.tokenfactory.GenesisState { Params params = 1; repeated GenesisDenom factory_denoms = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.gen-tokenfactory-denom-allowlist-max-size-j35</code> | notes | param key allowlistmaxsize; validateDenomAllowListMaxSize checks the type only |
| <code>interface.6.gen-tokenfactory-paused-denoms-j35</code> | notes | planned field 3; denoms whose paused&#124; key is set at export |
| <code>interface.6.go-tokenfactory-exportgenesis-j35</code> | notes | exports paused_denoms |
| <code>interface.6.go-tokenfactory-genesisstate-validate-j35</code> | notes | gains paused_denoms checks and DenomAuthorityMetadata.Validate per denom |
| <code>interface.6.go-tokenfactory-initgenesis-j35</code> | notes | restores paused_denoms |
| <code>interface.6.proto-tokenfactory-genesisstate</code> | notes | Go type GenesisState at modules/tokenfactory/types/genesis.pb.go:27 |
| <code>interface.6.proto-tokenfactory-genesisstate</code> | overridden_by | override.J35.1 |

Contract context <code>contract_context.6.tokenfactory-pause-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | tokenfactory Msg SetDenomPaused (api/tokenfactory/tx.proto), planned Query DenomPaused GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused, keeper SetDenomPausedByIssuer, MintByIssuer, BurnByIssuer and the IssuerKeeper interface consumed by layerxledger, CLI set-denom-paused and denom-paused, wasm set_denom_paused and denom_paused |
| storage | Root store of the tokenfactory key: paused&#124;&lt;denom&gt; -&gt; 0x01 (modules/tokenfactory/types/keys.go:55), deleted on unpause; issuer_did inside DenomAuthorityMetadata under denoms&#124;&lt;denom&gt;&#124;authoritymetadata; GenesisState field 3 paused_denoms carries the paused set across export |
| errors | ErrDenomPaused 26 LXP_ASSET_PAUSED, ErrPauseUnchanged 27 LXP_PAUSED_SCOPE (modules/tokenfactory/types/errors.go:34-35), planned ErrIssuerMismatch 28 LXP_UNAUTHORIZED_DEBIT, ErrEncodeTokenFactorySetDenomPaused 29, ErrEncodingDenomPaused 30; bank paths refuse with ErrSendDisabled |
| events | set_denom_paused { denom, paused, issuer_did on the issuer path }, set_issuer_did { denom, issuer_did } |
| gas | No fixed surcharge: Cosmos store gas of one Has per IsDenomPaused and one Set or Delete per pause change |
| activation | Bank msg-server checks are live at HEAD through RegisterSendEnabledChecker (node/app.go:766); the GigaBankKeeper registration and the issuer path land at the merge revision with no height gate because a pause only exists after an explicit pause call |
| invariants | Every paused&#124; key names a denom with authority metadata (PausedDenomsInvariant); pause never changes supply; an issuer-bound denom changes pause only through its issuer DID |
| pause | Per-denom pause refuses MsgSend, MsgMultiSend, GigaBankKeeper SendCoins and SendCoinsAndWei, tokenfactory Mint and Burn and MintByIssuer and BurnByIssuer; keeper-level x/bank SendCoins and module-account flows are not gated (decision.621); there is no governance per-denom pause (decision.623) |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.241.1</code> | req | 6241.1 |
| <code>acceptance.6.241.1</code> | scenario | Given a GenesisState holding PausedDenoms [factory/&lt;creator&gt;/paused] built from the regenerated modules/tokenfactory/types/genesis.pb.go, when it is marshalled and unmarshalled with the gogoproto codec, then the bytes carry tag 0x1a for field 3, the list round-trips unchanged and a state encoded without field 3 decodes with an empty PausedDenoms |
| <code>acceptance.6.241.1</code> | test | modules/tokenfactory/types/genesis_paused_test.go::TestGenesisPausedDenomsWireField (planned) |
| <code>acceptance.6.241.1</code> | qualification | UNRUN |
| <code>req.6241</code> | ac_1 | GenesisState (api/tokenfactory/genesis.proto:11-19) carries repeated string paused_denoms as field 3 after params = 1 (api/tokenfactory/genesis.proto:13) and factory_denoms = 2 (api/tokenfactory/genesis.proto:15), and a state without it decodes with an empty list. |
| <code>acceptance.6.242.1</code> | req | 6242.1 |
| <code>acceptance.6.242.1</code> | scenario | Given a GenesisState whose factory_denoms list factory/&lt;creator&gt;/a and whose paused_denoms list factory/&lt;creator&gt;/b, when Validate runs, then it returns ErrInvalidGenesis naming factory/&lt;creator&gt;/b, and with paused_denoms [factory/&lt;creator&gt;/a] it returns nil |
| <code>acceptance.6.242.1</code> | test | modules/tokenfactory/types/genesis_paused_validate_test.go::TestGenesisPausedDenomsMustBeKnown (planned) |
| <code>acceptance.6.242.1</code> | qualification | UNRUN |
| <code>req.6242</code> | ac_1 | GenesisState.Validate (modules/tokenfactory/types/genesis.go:23-51) refuses a paused_denoms entry that names no factory_denoms entry with ErrInvalidGenesis (modules/tokenfactory/types/errors.go:18). |
| <code>acceptance.6.242.2</code> | req | 6242.2 |
| <code>acceptance.6.242.2</code> | scenario | Given a GenesisState with factory denom factory/&lt;creator&gt;/a and paused_denoms [factory/&lt;creator&gt;/a, factory/&lt;creator&gt;/a], when Validate runs, then it returns ErrInvalidGenesis with the text duplicate paused denom |
| <code>acceptance.6.242.2</code> | test | modules/tokenfactory/types/genesis_paused_validate_test.go::TestGenesisPausedDenomsUnique (planned) |
| <code>acceptance.6.242.2</code> | qualification | UNRUN |
| <code>req.6242</code> | ac_2 | GenesisState.Validate refuses a paused_denoms list that repeats a denom with ErrInvalidGenesis, using the seen-denom pattern of modules/tokenfactory/types/genesis.go:29-34. |
| <code>acceptance.6.244.1</code> | req | 6244.1 |
| <code>acceptance.6.244.1</code> | scenario | Given a keeper with denoms a and b where a is paused, when ExportGenesis output is passed to InitGenesis of a fresh app, then IsDenomPaused(a) is true, IsDenomPaused(b) is false and a MsgSend of a is refused with ErrSendDisabled |
| <code>acceptance.6.244.1</code> | test | modules/tokenfactory/keeper/genesis_paused_test.go::TestGenesisPausedRoundTrip (planned) |
| <code>acceptance.6.244.1</code> | qualification | UNRUN |
| <code>req.6244</code> | ac_1 | ExportGenesis (modules/tokenfactory/keeper/genesis.go:33-55) writes GetPausedDenoms into paused_denoms and InitGenesis (modules/tokenfactory/keeper/genesis.go:11-30) restores each entry, so a paused denom is still paused after export and import. |
| <code>acceptance.6.244.2</code> | req | 6244.2 |
| <code>acceptance.6.244.2</code> | scenario | Given a GenesisState that bypasses Validate and lists paused_denoms [factory/&lt;creator&gt;/missing] with no factory denom, when InitGenesis runs, then it panics with ErrDenomDoesNotExist |
| <code>acceptance.6.244.2</code> | test | modules/tokenfactory/keeper/genesis_paused_test.go::TestInitGenesisPausedUnknownDenomPanics (planned) |
| <code>acceptance.6.244.2</code> | qualification | UNRUN |
| <code>req.6244</code> | ac_2 | InitGenesis panics on a paused_denoms entry with no factory denom, the panic rule of modules/tokenfactory/keeper/genesis.go:19-27, so an invalid state never boots. |
| <code>acceptance.6.245.1</code> | req | 6245.1 |
| <code>acceptance.6.245.1</code> | scenario | Given an app with a paused denom bound to issuer did:layerx:&lt;64 hex&gt;, when the app state is exported and a new app runs InitChain from it, then IsDenomPaused is true and IssuerDid returns the same DID |
| <code>acceptance.6.245.1</code> | test | modules/tokenfactory/keeper/app_export_paused_test.go::TestAppExportImportKeepsPausedDenoms (planned) |
| <code>acceptance.6.245.1</code> | qualification | UNRUN |
| <code>req.6245</code> | ac_1 | An app-level export through the module manager ExportGenesis (modules/tokenfactory/module.go:178) and InitChain from that state keep a paused denom paused and its issuer_did bound (modules/tokenfactory/keeper/admins.go:73). |
| <code>capability.6.21.1</code> | title | Add paused_denoms field 3 to the tokenfactory GenesisState proto |
| <code>capability.6.21.1</code> | reqs | 6241.1 |
| <code>capability.6.21.2</code> | title | Validate paused_denoms in tokenfactory GenesisState.Validate |
| <code>capability.6.21.2</code> | reqs | 6242.1<br>6242.2 |
| <code>capability.6.21.4</code> | title | Import and export paused denoms in tokenfactory genesis |
| <code>capability.6.21.4</code> | reqs | 6244.1<br>6244.2 |
| <code>capability.6.21.5</code> | title | Prove an app export and re-import keeps paused tokenfactory denoms |
| <code>capability.6.21.5</code> | reqs | 6245.1 |
| <code>override.J35.1</code> | target | interface.6.proto-tokenfactory-genesisstate |
| <code>override.J35.1</code> | status | change |
| <code>override.J35.1</code> | basis | GenesisState at api/tokenfactory/genesis.proto:11-19 has no paused_denoms field, so ExportGenesis at modules/tokenfactory/keeper/genesis.go:51-54 drops the paused&#124; keys written by setDenomPaused at modules/tokenfactory/keeper/pause.go:17-29. |
| <code>resolution.17.722</code> | title | decision.626 binds: tokenfactory consensus version and historical migrations stay |
| <code>resolution.17.722</code> | applies_to | decision.626<br>task.6.21.1<br>task.6.21.4 |
| <code>resolution.17.722</code> | source | decision.626 in fragments/J35/decision.kvx; tasks from affects |

## go-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-tokenfactory-client-cli-getcmddenomauthoritymetadata-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/cli/query.go:63</code> | <code>func GetCmdDenomAuthorityMetadata() *cobra.Command</code> |
| <code>interface.6.go-tokenfactory-client-cli-getcmddenomsfromcreator-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/cli/query.go:92</code> | <code>func GetCmdDenomsFromCreator() *cobra.Command</code> |
| <code>interface.6.go-tokenfactory-client-cli-getparams-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/cli/query.go:36</code> | <code>func GetParams() *cobra.Command</code> |
| <code>interface.6.go-tokenfactory-client-cli-newburncmd-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/cli/tx.go:185</code> | <code>func NewBurnCmd() *cobra.Command</code> |
| <code>interface.6.go-tokenfactory-client-cli-newchangeadmincmd-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/cli/tx.go:217</code> | <code>func NewChangeAdminCmd() *cobra.Command</code> |
| <code>interface.6.go-tokenfactory-client-cli-newcreatedenomcmd-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/cli/tx.go:56</code> | <code>func NewCreateDenomCmd() *cobra.Command</code> |
| <code>interface.6.go-tokenfactory-client-cli-newmintcmd-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/cli/tx.go:153</code> | <code>func NewMintCmd() *cobra.Command</code> |
| <code>interface.6.go-tokenfactory-client-cli-newsetdenommetadatacmd-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/cli/tx.go:244</code> | <code>func NewSetDenomMetadataCmd() *cobra.Command</code> |
| <code>interface.6.go-tokenfactory-client-cli-newupdatedenomcmd-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/cli/tx.go:104</code> | <code>func NewUpdateDenomCmd() *cobra.Command</code> |
| <code>interface.6.go-tokenfactory-client-cli-parseallowlistjson-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/cli/tx.go:316</code> | <code>func ParseAllowListJSON(allowListFile string, queryClient evmtypes.QueryClient) (banktypes.AllowList, error)</code> |
| <code>interface.6.go-tokenfactory-client-cli-parsemetadatajson-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/cli/tx.go:301</code> | <code>func ParseMetadataJSON(cdc *codec.LegacyAmino, metadataFile string) (banktypes.Metadata, error)</code> |
| <code>interface.6.go-tokenfactory-client-wasm-encodetokenfactoryburn-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/wasm/encoder.go:35</code> | <code>func EncodeTokenFactoryBurn(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sdk.Msg, error)</code> |
| <code>interface.6.go-tokenfactory-client-wasm-encodetokenfactorychangeadmin-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/wasm/encoder.go:47</code> | <code>func EncodeTokenFactoryChangeAdmin(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sdk.Msg, error)</code> |
| <code>interface.6.go-tokenfactory-client-wasm-encodetokenfactorycreatedenom-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/wasm/encoder.go:11</code> | <code>func EncodeTokenFactoryCreateDenom(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sdk.Msg, error)</code> |
| <code>interface.6.go-tokenfactory-client-wasm-encodetokenfactorymint-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/wasm/encoder.go:23</code> | <code>func EncodeTokenFactoryMint(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sdk.Msg, error)</code> |
| <code>interface.6.go-tokenfactory-client-wasm-encodetokenfactorysetmetadata-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/wasm/encoder.go:60</code> | <code>func EncodeTokenFactorySetMetadata(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sdk.Msg, error)</code> |
| <code>interface.6.go-tokenfactory-client-wasm-newtokenfactorywasmqueryhandler-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/wasm/query.go:13</code> | <code>func NewTokenFactoryWasmQueryHandler(keeper *tokenfactorykeeper.Keeper) *TokenFactoryWasmQueryHandler</code> |
| <code>interface.6.go-tokenfactory-client-wasm-tokenfactorywasmqueryhandler-getdenomauthoritymetadata-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/wasm/query.go:19</code> | <code>func (handler TokenFactoryWasmQueryHandler) GetDenomAuthorityMetadata(ctx sdk.Context, req *types.QueryDenomAuthorityMetadataRequest) (*types.QueryDenomAuthorityMetadataResponse, error)</code> |
| <code>interface.6.go-tokenfactory-client-wasm-tokenfactorywasmqueryhandler-getdenomsfromcreator-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/wasm/query.go:24</code> | <code>func (handler TokenFactoryWasmQueryHandler) GetDenomsFromCreator(ctx sdk.Context, req *types.QueryDenomsFromCreatorRequest) (*types.QueryDenomsFromCreatorResponse, error)</code> |
| <code>interface.6.go-tokenfactory-client-wasm-tokenfactorywasmqueryhandler-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/client/wasm/query.go:9</code> | <code>type TokenFactoryWasmQueryHandler struct</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-createdenom-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/createdenom.go:13</code> | <code>func (k Keeper) CreateDenom(ctx sdk.Context, creatorAddr string, subdenom string) (newTokenDenom string, err error)</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-createmoduleaccount-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/keeper.go:71</code> | <code>func (k Keeper) CreateModuleAccount(ctx sdk.Context)</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-denomallowlist-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/grpc_query.go:64</code> | <code>func (k Keeper) DenomAllowList(c context.Context, req *types.QueryDenomAllowListRequest) (*types.QueryDenomAllowListResponse, error)</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-denomauthoritymetadata-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/grpc_query.go:22</code> | <code>func (k Keeper) DenomAuthorityMetadata(ctx context.Context, req *types.QueryDenomAuthorityMetadataRequest) (*types.QueryDenomAuthorityMetadataResponse, error)</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-denommetadata-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/grpc_query.go:43</code> | <code>func (k Keeper) DenomMetadata(c context.Context, req *types.QueryDenomMetadataRequest) (*types.QueryDenomMetadataResponse, error)</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-denomsfromcreator-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/grpc_query.go:33</code> | <code>func (k Keeper) DenomsFromCreator(ctx context.Context, req *types.QueryDenomsFromCreatorRequest) (*types.QueryDenomsFromCreatorResponse, error)</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-getalldenomsfromcreator-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/creators.go:28</code> | <code>func (k Keeper) GetAllDenomsFromCreator(ctx sdk.Context, creator string) []string</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-getalldenomsiterator-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/creators.go:39</code> | <code>func (k Keeper) GetAllDenomsIterator(ctx sdk.Context) sdk.Iterator</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-getauthoritymetadata-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/admins.go:11</code> | <code>func (k Keeper) GetAuthorityMetadata(ctx sdk.Context, denom string) (types.DenomAuthorityMetadata, error)</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-getcreatorprefixstore-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/keeper.go:56</code> | <code>func (k Keeper) GetCreatorPrefixStore(ctx sdk.Context, creator string) sdk.KVStore</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-getcreatorsprefixstore-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/keeper.go:62</code> | <code>func (k Keeper) GetCreatorsPrefixStore(ctx sdk.Context) sdk.KVStore</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-getdenomallowlistmaxsize-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/keeper.go:76</code> | <code>func (k Keeper) GetDenomAllowListMaxSize(ctx sdk.Context) uint32</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-getdenomprefixstore-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/keeper.go:50</code> | <code>func (k Keeper) GetDenomPrefixStore(ctx sdk.Context, denom string) sdk.KVStore</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-getparams-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/params.go:10</code> | <code>func (k Keeper) GetParams(ctx sdk.Context) (params types.Params)</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-isdenompaused-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/pause.go:12</code> | <code>func (k Keeper) IsDenomPaused(ctx sdk.Context, denom string) bool</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-issuerdid-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/admins.go:73</code> | <code>func (k Keeper) IssuerDid(ctx sdk.Context, denom string) (string, error)</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-params-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/grpc_query.go:15</code> | <code>func (k Keeper) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error)</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-sendenabledchecker-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/pause.go:32</code> | <code>func (k Keeper) SendEnabledChecker(ctx sdk.Context, denom string) bool</code> |
| <code>interface.6.go-tokenfactory-keeper-keeper-setparams-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/params.go:16</code> | <code>func (k Keeper) SetParams(ctx sdk.Context, params types.Params)</code> |
| <code>interface.6.go-tokenfactory-keeper-migrator-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/migrations.go:16</code> | <code>type Migrator struct</code> |
| <code>interface.6.go-tokenfactory-keeper-migrator-migrate2to3-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/migrations.go:26</code> | <code>func (m Migrator) Migrate2to3(ctx sdk.Context) error</code> |
| <code>interface.6.go-tokenfactory-keeper-migrator-migrate3to4-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/migrations.go:44</code> | <code>func (m Migrator) Migrate3to4(ctx sdk.Context) error</code> |
| <code>interface.6.go-tokenfactory-keeper-migrator-migrate4to5-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/migrations.go:61</code> | <code>func (m Migrator) Migrate4to5(ctx sdk.Context) error</code> |
| <code>interface.6.go-tokenfactory-keeper-migrator-setmetadata-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/migrations.go:68</code> | <code>func (m Migrator) SetMetadata(denomMetadata *banktypes.Metadata)</code> |
| <code>interface.6.go-tokenfactory-keeper-newmigrator-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/migrations.go:21</code> | <code>func NewMigrator(keeper Keeper) Migrator</code> |
| <code>interface.6.go-tokenfactory-keeper-newmsgserverimpl-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/keeper/msg_server.go:18</code> | <code>func NewMsgServerImpl(keeper Keeper) types.MsgServer</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-exportgenesis-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:178</code> | <code>func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-exportgenesisstream-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:184</code> | <code>func (am AppModule) ExportGenesisStream(ctx sdk.Context, cdc codec.JSONCodec) &lt;-chan json.RawMessage</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-generategenesisstate-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:212</code> | <code>func (am AppModule) GenerateGenesisState(simState *module.SimulationState)</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-initgenesis-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:167</code> | <code>func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, gs json.RawMessage) []abci.ValidatorUpdate</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:102</code> | <code>type AppModule struct</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-legacyquerierhandler-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:137</code> | <code>func (am AppModule) LegacyQuerierHandler(_ *codec.LegacyAmino) sdk.Querier</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-name-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:124</code> | <code>func (am AppModule) Name() string</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-proposalcontents-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:217</code> | <code>func (am AppModule) ProposalContents(_ module.SimulationState) []simtypes.WeightedProposalContent</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-randomizedparams-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:222</code> | <code>func (am AppModule) RandomizedParams(_ *rand.Rand) []simtypes.ParamChange</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-registerservices-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:143</code> | <code>func (am AppModule) RegisterServices(cfg module.Configurator)</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-registerstoredecoder-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:227</code> | <code>func (am AppModule) RegisterStoreDecoder(_ sdk.StoreDecoderRegistry)</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-route-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:129</code> | <code>func (am AppModule) Route() sdk.Route</code> |
| <code>interface.6.go-tokenfactory-module-appmodule-weightedoperations-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:231</code> | <code>func (am AppModule) WeightedOperations(_ module.SimulationState) []simtypes.WeightedOperation</code> |
| <code>interface.6.go-tokenfactory-module-appmodulebasic-gettxcmd-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:88</code> | <code>func (a AppModuleBasic) GetTxCmd() *cobra.Command</code> |
| <code>interface.6.go-tokenfactory-module-appmodulebasic-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:43</code> | <code>type AppModuleBasic struct{}</code> |
| <code>interface.6.go-tokenfactory-module-appmodulebasic-registerinterfaces-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:59</code> | <code>func (a AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry)</code> |
| <code>interface.6.go-tokenfactory-module-appmodulebasic-validategenesisstream-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:194</code> | <code>func (am AppModuleBasic) ValidateGenesisStream(cdc codec.JSONCodec, config client.TxEncodingConfig, genesisCh &lt;-chan json.RawMessage) error</code> |
| <code>interface.6.go-tokenfactory-module-basic-defaultgenesis-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:64</code> | <code>func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage</code> |
| <code>interface.6.go-tokenfactory-module-basic-getquerycmd-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:93</code> | <code>func (AppModuleBasic) GetQueryCmd() *cobra.Command</code> |
| <code>interface.6.go-tokenfactory-module-basic-name-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:50</code> | <code>func (AppModuleBasic) Name() string</code> |
| <code>interface.6.go-tokenfactory-module-basic-registergrpcgatewayroutes-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:83</code> | <code>func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux)</code> |
| <code>interface.6.go-tokenfactory-module-basic-registerlegacyaminocodec-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:54</code> | <code>func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino)</code> |
| <code>interface.6.go-tokenfactory-module-basic-registerrestroutes-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:79</code> | <code>func (AppModuleBasic) RegisterRESTRoutes(_ client.Context, _ *mux.Router)</code> |
| <code>interface.6.go-tokenfactory-module-basic-validategenesis-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:69</code> | <code>func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error</code> |
| <code>interface.6.go-tokenfactory-module-module-consensusversion-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:205</code> | <code>func (AppModule) ConsensusVersion() uint64 { return 5 }</code> |
| <code>interface.6.go-tokenfactory-module-module-querierroute-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:134</code> | <code>func (AppModule) QuerierRoute() string { return types.QuerierRoute }</code> |
| <code>interface.6.go-tokenfactory-module-newappmodule-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:110</code> | <code>func NewAppModule(</code> |
| <code>interface.6.go-tokenfactory-module-newappmodulebasic-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/module.go:45</code> | <code>func NewAppModuleBasic() AppModuleBasic</code> |
| <code>interface.6.go-tokenfactory-module-newproposalhandler-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/handler.go:11</code> | <code>func NewProposalHandler(_ keeper.Keeper) govtypes.Handler</code> |
| <code>interface.6.go-tokenfactory-types-accountkeeper-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/expected_keepers.go:33</code> | <code>type AccountKeeper interface</code> |
| <code>interface.6.go-tokenfactory-types-deconstructdenom-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/denoms.go:43</code> | <code>func DeconstructDenom(denom string) (creator string, subdenom string, err error)</code> |
| <code>interface.6.go-tokenfactory-types-defaultgenesis-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/genesis.go:14</code> | <code>func DefaultGenesis() *GenesisState</code> |
| <code>interface.6.go-tokenfactory-types-defaultparams-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/params.go:18</code> | <code>func DefaultParams() Params</code> |
| <code>interface.6.go-tokenfactory-types-denomauthoritymetadata-validate-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/authority_metadata.go:14</code> | <code>func (metadata DenomAuthorityMetadata) Validate() error</code> |
| <code>interface.6.go-tokenfactory-types-getcreatorprefix-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/keys.go:45</code> | <code>func GetCreatorPrefix(creator string) []byte</code> |
| <code>interface.6.go-tokenfactory-types-getcreatorsprefix-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/keys.go:50</code> | <code>func GetCreatorsPrefix() []byte</code> |
| <code>interface.6.go-tokenfactory-types-getdenompausedkey-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/keys.go:55</code> | <code>func GetDenomPausedKey(denom string) []byte</code> |
| <code>interface.6.go-tokenfactory-types-getdenomprefixstore-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/keys.go:39</code> | <code>func GetDenomPrefixStore(denom string) []byte</code> |
| <code>interface.6.go-tokenfactory-types-gettokendenom-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/denoms.go:26</code> | <code>func GetTokenDenom(creator, subdenom string) (string, error)</code> |
| <code>interface.6.go-tokenfactory-types-msgburn-getsignbytes-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:150</code> | <code>func (m MsgBurn) GetSignBytes() []byte</code> |
| <code>interface.6.go-tokenfactory-types-msgburn-getsigners-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:154</code> | <code>func (m MsgBurn) GetSigners() []sdk.AccAddress</code> |
| <code>interface.6.go-tokenfactory-types-msgburn-route-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:135</code> | <code>func (m MsgBurn) Route() string { return RouterKey }</code> |
| <code>interface.6.go-tokenfactory-types-msgburn-type-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:136</code> | <code>func (m MsgBurn) Type() string  { return TypeMsgBurn }</code> |
| <code>interface.6.go-tokenfactory-types-msgburn-validatebasic-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:137</code> | <code>func (m MsgBurn) ValidateBasic() error</code> |
| <code>interface.6.go-tokenfactory-types-msgchangeadmin-getsignbytes-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:191</code> | <code>func (m MsgChangeAdmin) GetSignBytes() []byte</code> |
| <code>interface.6.go-tokenfactory-types-msgchangeadmin-getsigners-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:195</code> | <code>func (m MsgChangeAdmin) GetSigners() []sdk.AccAddress</code> |
| <code>interface.6.go-tokenfactory-types-msgchangeadmin-route-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:170</code> | <code>func (m MsgChangeAdmin) Route() string { return RouterKey }</code> |
| <code>interface.6.go-tokenfactory-types-msgchangeadmin-type-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:171</code> | <code>func (m MsgChangeAdmin) Type() string  { return TypeMsgChangeAdmin }</code> |
| <code>interface.6.go-tokenfactory-types-msgchangeadmin-validatebasic-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:172</code> | <code>func (m MsgChangeAdmin) ValidateBasic() error</code> |
| <code>interface.6.go-tokenfactory-types-msgcreatedenom-getsignbytes-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:46</code> | <code>func (m MsgCreateDenom) GetSignBytes() []byte</code> |
| <code>interface.6.go-tokenfactory-types-msgcreatedenom-getsigners-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:50</code> | <code>func (m MsgCreateDenom) GetSigners() []sdk.AccAddress</code> |
| <code>interface.6.go-tokenfactory-types-msgcreatedenom-route-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:30</code> | <code>func (m MsgCreateDenom) Route() string { return RouterKey }</code> |
| <code>interface.6.go-tokenfactory-types-msgcreatedenom-type-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:31</code> | <code>func (m MsgCreateDenom) Type() string  { return TypeMsgCreateDenom }</code> |
| <code>interface.6.go-tokenfactory-types-msgcreatedenom-validatebasic-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:32</code> | <code>func (m MsgCreateDenom) ValidateBasic() error</code> |
| <code>interface.6.go-tokenfactory-types-msgmint-getsignbytes-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:116</code> | <code>func (m MsgMint) GetSignBytes() []byte</code> |
| <code>interface.6.go-tokenfactory-types-msgmint-getsigners-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:120</code> | <code>func (m MsgMint) GetSigners() []sdk.AccAddress</code> |
| <code>interface.6.go-tokenfactory-types-msgmint-route-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:101</code> | <code>func (m MsgMint) Route() string { return RouterKey }</code> |
| <code>interface.6.go-tokenfactory-types-msgmint-type-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:102</code> | <code>func (m MsgMint) Type() string  { return TypeMsgMint }</code> |
| <code>interface.6.go-tokenfactory-types-msgmint-validatebasic-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:103</code> | <code>func (m MsgMint) ValidateBasic() error</code> |
| <code>interface.6.go-tokenfactory-types-msgsetdenommetadata-getsignbytes-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:231</code> | <code>func (m MsgSetDenomMetadata) GetSignBytes() []byte</code> |
| <code>interface.6.go-tokenfactory-types-msgsetdenommetadata-getsigners-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:235</code> | <code>func (m MsgSetDenomMetadata) GetSigners() []sdk.AccAddress</code> |
| <code>interface.6.go-tokenfactory-types-msgsetdenommetadata-route-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:210</code> | <code>func (m MsgSetDenomMetadata) Route() string { return RouterKey }</code> |
| <code>interface.6.go-tokenfactory-types-msgsetdenommetadata-type-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:211</code> | <code>func (m MsgSetDenomMetadata) Type() string  { return TypeMsgSetDenomMetadata }</code> |
| <code>interface.6.go-tokenfactory-types-msgsetdenommetadata-validatebasic-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:212</code> | <code>func (m MsgSetDenomMetadata) ValidateBasic() error</code> |
| <code>interface.6.go-tokenfactory-types-msgsetdenompaused-getsignbytes-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:272</code> | <code>func (m MsgSetDenomPaused) GetSignBytes() []byte</code> |
| <code>interface.6.go-tokenfactory-types-msgsetdenompaused-getsigners-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:276</code> | <code>func (m MsgSetDenomPaused) GetSigners() []sdk.AccAddress</code> |
| <code>interface.6.go-tokenfactory-types-msgsetdenompaused-route-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:251</code> | <code>func (m MsgSetDenomPaused) Route() string { return RouterKey }</code> |
| <code>interface.6.go-tokenfactory-types-msgsetdenompaused-type-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:252</code> | <code>func (m MsgSetDenomPaused) Type() string  { return TypeMsgSetDenomPaused }</code> |
| <code>interface.6.go-tokenfactory-types-msgsetdenompaused-validatebasic-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:253</code> | <code>func (m MsgSetDenomPaused) ValidateBasic() error</code> |
| <code>interface.6.go-tokenfactory-types-msgupdatedenom-getsignbytes-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:82</code> | <code>func (m MsgUpdateDenom) GetSignBytes() []byte</code> |
| <code>interface.6.go-tokenfactory-types-msgupdatedenom-getsigners-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:86</code> | <code>func (m MsgUpdateDenom) GetSigners() []sdk.AccAddress</code> |
| <code>interface.6.go-tokenfactory-types-msgupdatedenom-route-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:66</code> | <code>func (m MsgUpdateDenom) Route() string { return RouterKey }</code> |
| <code>interface.6.go-tokenfactory-types-msgupdatedenom-type-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:67</code> | <code>func (m MsgUpdateDenom) Type() string  { return TypeMsgUpdateDenom }</code> |
| <code>interface.6.go-tokenfactory-types-msgupdatedenom-validatebasic-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:68</code> | <code>func (m MsgUpdateDenom) ValidateBasic() error</code> |
| <code>interface.6.go-tokenfactory-types-newmsgburn-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:128</code> | <code>func NewMsgBurn(sender string, amount sdk.Coin) *MsgBurn</code> |
| <code>interface.6.go-tokenfactory-types-newmsgchangeadmin-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:162</code> | <code>func NewMsgChangeAdmin(sender, denom, newAdmin string) *MsgChangeAdmin</code> |
| <code>interface.6.go-tokenfactory-types-newmsgcreatedenom-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:23</code> | <code>func NewMsgCreateDenom(sender, subdenom string) *MsgCreateDenom</code> |
| <code>interface.6.go-tokenfactory-types-newmsgmint-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:94</code> | <code>func NewMsgMint(sender string, amount sdk.Coin) *MsgMint</code> |
| <code>interface.6.go-tokenfactory-types-newmsgsetdenommetadata-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:203</code> | <code>func NewMsgSetDenomMetadata(sender string, metadata banktypes.Metadata) *MsgSetDenomMetadata</code> |
| <code>interface.6.go-tokenfactory-types-newmsgsetdenompaused-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:243</code> | <code>func NewMsgSetDenomPaused(sender, denom string, paused bool) *MsgSetDenomPaused</code> |
| <code>interface.6.go-tokenfactory-types-newmsgupdatedenom-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/msgs.go:58</code> | <code>func NewMsgUpdateDenom(sender, denom string, allowList *banktypes.AllowList) *MsgUpdateDenom</code> |
| <code>interface.6.go-tokenfactory-types-newtokenfactorydenommintcoinsrestriction-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/denoms.go:74</code> | <code>func NewTokenFactoryDenomMintCoinsRestriction() bankkeeper.MintingRestrictionFn</code> |
| <code>interface.6.go-tokenfactory-types-paramkeytable-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/params.go:13</code> | <code>func ParamKeyTable() paramtypes.KeyTable</code> |
| <code>interface.6.go-tokenfactory-types-params-paramsetpairs-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/params.go:33</code> | <code>func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs</code> |
| <code>interface.6.go-tokenfactory-types-params-validate-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/params.go:25</code> | <code>func (p Params) Validate() error</code> |
| <code>interface.6.go-tokenfactory-types-registercodec-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/codec.go:12</code> | <code>func RegisterCodec(cdc *codec.LegacyAmino)</code> |
| <code>interface.6.go-tokenfactory-types-registerinterfaces-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/tokenfactory/types/codec.go:22</code> | <code>func RegisterInterfaces(registry cdctypes.InterfaceRegistry)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-tokenfactory-client-cli-getcmddenomauthoritymetadata-j35</code> | notes | exported func; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-client-cli-getcmddenomsfromcreator-j35</code> | notes | exported func; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-client-cli-getparams-j35</code> | notes | exported func; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-client-cli-newburncmd-j35</code> | notes | exported func; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-client-cli-newchangeadmincmd-j35</code> | notes | exported func; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-client-cli-newcreatedenomcmd-j35</code> | notes | exported func; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-client-cli-newmintcmd-j35</code> | notes | exported func; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-client-cli-newsetdenommetadatacmd-j35</code> | notes | exported func; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-client-cli-newupdatedenomcmd-j35</code> | notes | exported func; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-client-cli-parseallowlistjson-j35</code> | notes | exported func; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-client-cli-parsemetadatajson-j35</code> | notes | exported func; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-client-wasm-encodetokenfactoryburn-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-client-wasm-encodetokenfactorychangeadmin-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-client-wasm-encodetokenfactorycreatedenom-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-client-wasm-encodetokenfactorymint-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-client-wasm-encodetokenfactorysetmetadata-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-client-wasm-newtokenfactorywasmqueryhandler-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-client-wasm-tokenfactorywasmqueryhandler-getdenomauthoritymetadata-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-client-wasm-tokenfactorywasmqueryhandler-getdenomsfromcreator-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-client-wasm-tokenfactorywasmqueryhandler-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-keeper-createdenom-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-keeper-createmoduleaccount-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-keeper-denomallowlist-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-keeper-denomauthoritymetadata-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-keeper-denommetadata-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-keeper-denomsfromcreator-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-keeper-getalldenomsfromcreator-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-keeper-getalldenomsiterator-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-keeper-getauthoritymetadata-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-keeper-getcreatorprefixstore-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-keeper-getcreatorsprefixstore-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-keeper-getdenomallowlistmaxsize-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-keeper-getdenomprefixstore-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-keeper-getparams-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-keeper-isdenompaused-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-keeper-issuerdid-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-keeper-params-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-keeper-sendenabledchecker-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-keeper-setparams-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-migrator-j35</code> | notes | exported type; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-migrator-migrate2to3-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-migrator-migrate3to4-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-migrator-migrate4to5-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-migrator-setmetadata-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-keeper-newmigrator-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-keeper-newmsgserverimpl-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-module-appmodule-exportgenesis-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-appmodule-exportgenesisstream-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-appmodule-generategenesisstate-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-appmodule-initgenesis-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-module-appmodule-j35</code> | notes | exported type; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-appmodule-legacyquerierhandler-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-appmodule-name-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-module-appmodule-proposalcontents-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-appmodule-randomizedparams-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-appmodule-registerservices-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-module-appmodule-registerstoredecoder-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-appmodule-route-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-appmodule-weightedoperations-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-appmodulebasic-gettxcmd-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-appmodulebasic-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-tokenfactory-module-appmodulebasic-registerinterfaces-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-module-appmodulebasic-validategenesisstream-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-basic-defaultgenesis-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-basic-getquerycmd-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-basic-name-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-module-basic-registergrpcgatewayroutes-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-module-basic-registerlegacyaminocodec-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-module-basic-registerrestroutes-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-module-basic-validategenesis-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-module-consensusversion-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-module-querierroute-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-module-newappmodule-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-module-newappmodulebasic-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-module-newproposalhandler-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-accountkeeper-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-tokenfactory-types-deconstructdenom-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-defaultgenesis-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-defaultparams-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-denomauthoritymetadata-validate-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-getcreatorprefix-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-getcreatorsprefix-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-getdenompausedkey-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-getdenomprefixstore-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-gettokendenom-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgburn-getsignbytes-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgburn-getsigners-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgburn-route-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgburn-type-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgburn-validatebasic-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgchangeadmin-getsignbytes-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgchangeadmin-getsigners-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgchangeadmin-route-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgchangeadmin-type-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgchangeadmin-validatebasic-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgcreatedenom-getsignbytes-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgcreatedenom-getsigners-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgcreatedenom-route-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgcreatedenom-type-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgcreatedenom-validatebasic-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgmint-getsignbytes-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgmint-getsigners-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgmint-route-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgmint-type-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgmint-validatebasic-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgsetdenommetadata-getsignbytes-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgsetdenommetadata-getsigners-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgsetdenommetadata-route-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgsetdenommetadata-type-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgsetdenommetadata-validatebasic-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgsetdenompaused-getsignbytes-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgsetdenompaused-getsigners-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgsetdenompaused-route-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgsetdenompaused-type-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgsetdenompaused-validatebasic-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgupdatedenom-getsignbytes-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgupdatedenom-getsigners-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgupdatedenom-route-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgupdatedenom-type-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-msgupdatedenom-validatebasic-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-newmsgburn-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-newmsgchangeadmin-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-newmsgcreatedenom-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-newmsgmint-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-newmsgsetdenommetadata-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-newmsgsetdenompaused-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-newmsgupdatedenom-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-newtokenfactorydenommintcoinsrestriction-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-paramkeytable-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-params-paramsetpairs-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-tokenfactory-types-params-validate-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-tokenfactory-types-registercodec-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-tokenfactory-types-registerinterfaces-j35</code> | notes | exported func; referenced across packages |

## invariants

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-tokenfactory-appmodule-registerinvariants-j35</code> | <code>go_exported</code> | <code>change</code> | <code>modules/tokenfactory/module.go:163</code> | <code>func (am AppModule) RegisterInvariants(_ sdk.InvariantRegistry)</code> |
| <code>interface.6.go-tokenfactory-pauseddenomsinvariant-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/tokenfactory/keeper/invariants.go</code> (planned) | <code>func PausedDenomsInvariant(k Keeper) sdk.Invariant</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-tokenfactory-appmodule-registerinvariants-j35</code> | notes | empty at HEAD; registers PausedDenomsInvariant |
| <code>interface.6.go-tokenfactory-pauseddenomsinvariant-j35</code> | notes | route tokenfactory/paused-denoms |

Contract context <code>contract_context.6.modules-tokenfactory-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 10 surfaces under modules/tokenfactory (4 cli_command, 4 go_exported, 2 wasm_binding; 2 change, 8 new) frozen at the anchors contracts/tokenfactory.md#cli, contracts/tokenfactory.md#invariants, contracts/tokenfactory.md#wasm: paxd query tokenfactory denom-allow-list [denom] (modules/tokenfactory/client/cli/query.go (planned)); paxd query tokenfactory denom-metadata [denom] (modules/tokenfactory/client/cli/query.go (planned)); paxd query tokenfactory denom-paused [denom] (modules/tokenfactory/client/cli/query.go (planned)); paxd tx tokenfactory set-denom-paused [denom] [true&#124;false] (modules/tokenfactory/client/cli/tx.go (planned)); func (am AppModule) RegisterInvariants(_ sdk.InvariantRegistry) (modules/tokenfactory/module.go:163); func EncodeTokenFactorySetDenomPaused(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sd (modules/tokenfactory/client/wasm/encoder.go (planned)); type PaxTokenFactoryQuery struct { DenomAuthorityMetadata; DenomsFromCreator } (modules/tokenfactory/client/wasm/bindings/queries.go:5); func (handler TokenFactoryWasmQueryHandler) GetDenomPaused(ctx sdk.Context, req *types.Que (modules/tokenfactory/client/wasm/query.go (planned)); PaxTokenFactoryQuery.DenomPaused json:"denom_paused" -&gt; QueryDenomPausedResponse (modules/tokenfactory/client/wasm/bindings/queries.go (planned)); PaxWasmMessage.SetDenomPaused json:"set_denom_paused" { denom string; paused bool } (modules/tokenfactory/client/wasm/bindings/msgs.go (planned)) |
| storage | no storage_key item in this context; the modules/tokenfactory items read or write state only through the keepers and stores frozen by the other domain-6 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-6 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-6 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 6.23.3, 6.23.4, 6.23.5, 6.24.2, 6.24.3, 6.26.4 in waves 1, 2, 5 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 6 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

Contract context <code>contract_context.6.tokenfactory-pause-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | tokenfactory Msg SetDenomPaused (api/tokenfactory/tx.proto), planned Query DenomPaused GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused, keeper SetDenomPausedByIssuer, MintByIssuer, BurnByIssuer and the IssuerKeeper interface consumed by layerxledger, CLI set-denom-paused and denom-paused, wasm set_denom_paused and denom_paused |
| storage | Root store of the tokenfactory key: paused&#124;&lt;denom&gt; -&gt; 0x01 (modules/tokenfactory/types/keys.go:55), deleted on unpause; issuer_did inside DenomAuthorityMetadata under denoms&#124;&lt;denom&gt;&#124;authoritymetadata; GenesisState field 3 paused_denoms carries the paused set across export |
| errors | ErrDenomPaused 26 LXP_ASSET_PAUSED, ErrPauseUnchanged 27 LXP_PAUSED_SCOPE (modules/tokenfactory/types/errors.go:34-35), planned ErrIssuerMismatch 28 LXP_UNAUTHORIZED_DEBIT, ErrEncodeTokenFactorySetDenomPaused 29, ErrEncodingDenomPaused 30; bank paths refuse with ErrSendDisabled |
| events | set_denom_paused { denom, paused, issuer_did on the issuer path }, set_issuer_did { denom, issuer_did } |
| gas | No fixed surcharge: Cosmos store gas of one Has per IsDenomPaused and one Set or Delete per pause change |
| activation | Bank msg-server checks are live at HEAD through RegisterSendEnabledChecker (node/app.go:766); the GigaBankKeeper registration and the issuer path land at the merge revision with no height gate because a pause only exists after an explicit pause call |
| invariants | Every paused&#124; key names a denom with authority metadata (PausedDenomsInvariant); pause never changes supply; an issuer-bound denom changes pause only through its issuer DID |
| pause | Per-denom pause refuses MsgSend, MsgMultiSend, GigaBankKeeper SendCoins and SendCoinsAndWei, tokenfactory Mint and Burn and MintByIssuer and BurnByIssuer; keeper-level x/bank SendCoins and module-account flows are not gated (decision.621); there is no governance per-denom pause (decision.623) |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.268.1</code> | req | 6268.1 |
| <code>acceptance.6.268.1</code> | scenario | Given a keeper with one paused factory denom, when the invariant runs it is not broken; after a paused&#124; key for factory/&lt;creator&gt;/ghost is written straight to the store it reports broken naming that denom |
| <code>acceptance.6.268.1</code> | test | modules/tokenfactory/keeper/invariants_test.go::TestPausedDenomsInvariantBroken (planned) |
| <code>acceptance.6.268.1</code> | qualification | UNRUN |
| <code>req.6268</code> | ac_1 | PausedDenomsInvariant in modules/tokenfactory/keeper/invariants.go (planned) reports broken when a paused&#124; key names a denom without authority metadata, and RegisterInvariants (modules/tokenfactory/module.go:163) registers it as tokenfactory/paused-denoms. |
| <code>capability.6.26.4</code> | title | Register the tokenfactory PausedDenomsInvariant |
| <code>capability.6.26.4</code> | reqs | 6268.1 |

## issuer-did

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-tokenfactory-burnbyissuer-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/tokenfactory/keeper/issuer_supply.go</code> (planned) | <code>func (k Keeper) BurnByIssuer(ctx sdk.Context, callerDid string, amount sdk.Coin, from string) error</code> |
| <code>interface.6.go-tokenfactory-issuerkeeper-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/tokenfactory/types/expected_issuer.go</code> (planned) | <code>type IssuerKeeper interface { CreateDenom; SetIssuerDid; IssuerDid; IsDenomPaused; SetDenomPausedByIssuer; MintByIssuer; BurnByIssuer }</code> |
| <code>interface.6.go-tokenfactory-mintbyissuer-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/tokenfactory/keeper/issuer_supply.go</code> (planned) | <code>func (k Keeper) MintByIssuer(ctx sdk.Context, callerDid string, amount sdk.Coin, to string) error</code> |
| <code>interface.6.go-tokenfactory-setdenompausedbyissuer-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/tokenfactory/keeper/issuer.go</code> (planned) | <code>func (k Keeper) SetDenomPausedByIssuer(ctx sdk.Context, denom, callerDid string, paused bool) error</code> |
| <code>interface.6.go-tokenfactory-setissuerdid-j35</code> | <code>go_exported</code> | <code>change</code> | <code>modules/tokenfactory/keeper/admins.go:54</code> | <code>func (k Keeper) SetIssuerDid(ctx sdk.Context, denom string, issuerDid string) error</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-tokenfactory-burnbyissuer-j35</code> | notes | issuer-bound burn through burnFrom |
| <code>interface.6.go-tokenfactory-issuerkeeper-j35</code> | notes | the tokenfactory surface layerxledger consumes |
| <code>interface.6.go-tokenfactory-mintbyissuer-j35</code> | notes | issuer-bound mint through mintTo |
| <code>interface.6.go-tokenfactory-setdenompausedbyissuer-j35</code> | notes | issuer-bound pause; consumed by layerxledger through IssuerKeeper |
| <code>interface.6.go-tokenfactory-setissuerdid-j35</code> | notes | no production caller at HEAD; gains the set_issuer_did event |

Contract context <code>contract_context.6.tokenfactory-pause-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | tokenfactory Msg SetDenomPaused (api/tokenfactory/tx.proto), planned Query DenomPaused GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused, keeper SetDenomPausedByIssuer, MintByIssuer, BurnByIssuer and the IssuerKeeper interface consumed by layerxledger, CLI set-denom-paused and denom-paused, wasm set_denom_paused and denom_paused |
| storage | Root store of the tokenfactory key: paused&#124;&lt;denom&gt; -&gt; 0x01 (modules/tokenfactory/types/keys.go:55), deleted on unpause; issuer_did inside DenomAuthorityMetadata under denoms&#124;&lt;denom&gt;&#124;authoritymetadata; GenesisState field 3 paused_denoms carries the paused set across export |
| errors | ErrDenomPaused 26 LXP_ASSET_PAUSED, ErrPauseUnchanged 27 LXP_PAUSED_SCOPE (modules/tokenfactory/types/errors.go:34-35), planned ErrIssuerMismatch 28 LXP_UNAUTHORIZED_DEBIT, ErrEncodeTokenFactorySetDenomPaused 29, ErrEncodingDenomPaused 30; bank paths refuse with ErrSendDisabled |
| events | set_denom_paused { denom, paused, issuer_did on the issuer path }, set_issuer_did { denom, issuer_did } |
| gas | No fixed surcharge: Cosmos store gas of one Has per IsDenomPaused and one Set or Delete per pause change |
| activation | Bank msg-server checks are live at HEAD through RegisterSendEnabledChecker (node/app.go:766); the GigaBankKeeper registration and the issuer path land at the merge revision with no height gate because a pause only exists after an explicit pause call |
| invariants | Every paused&#124; key names a denom with authority metadata (PausedDenomsInvariant); pause never changes supply; an issuer-bound denom changes pause only through its issuer DID |
| pause | Per-denom pause refuses MsgSend, MsgMultiSend, GigaBankKeeper SendCoins and SendCoinsAndWei, tokenfactory Mint and Burn and MintByIssuer and BurnByIssuer; keeper-level x/bank SendCoins and module-account flows are not gated (decision.621); there is no governance per-denom pause (decision.623) |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.246.1</code> | req | 6246.1 |
| <code>acceptance.6.246.1</code> | scenario | Given a GenesisState whose factory denom carries issuer_did did:layerx:XYZ, when Validate runs, then it returns ErrInvalidAuthorityMetadata, and with did:layerx: followed by 64 lowercase hex characters it returns nil |
| <code>acceptance.6.246.1</code> | test | modules/tokenfactory/types/genesis_issuer_test.go::TestGenesisRejectsMalformedIssuerDid (planned) |
| <code>acceptance.6.246.1</code> | qualification | UNRUN |
| <code>req.6246</code> | ac_1 | GenesisState.Validate replaces the admin-only bech32 check (modules/tokenfactory/types/genesis.go:42-47) with DenomAuthorityMetadata.Validate (modules/tokenfactory/types/authority_metadata.go:14), so a malformed issuer_did is refused before InitGenesis reaches setAuthorityMetadata (modules/tokenfactory/keeper/admins.go:23-24). |
| <code>acceptance.6.248.1</code> | req | 6248.1 |
| <code>acceptance.6.248.1</code> | scenario | Given a denom bound by SetIssuerDid to did:layerx:&lt;key A&gt;, when SetDenomPausedByIssuer is called with did:layerx:&lt;key A&gt; and true, then IsDenomPaused is true and one set_denom_paused event carries issuer_did did:layerx:&lt;key A&gt;; a second call with true returns ErrPauseUnchanged |
| <code>acceptance.6.248.1</code> | test | modules/tokenfactory/keeper/issuer_pause_test.go::TestSetDenomPausedByIssuerMatches (planned) |
| <code>acceptance.6.248.1</code> | qualification | UNRUN |
| <code>req.6248</code> | ac_1 | SetDenomPausedByIssuer(ctx, denom, callerDid, paused) in modules/tokenfactory/keeper/issuer.go (planned) pauses and unpauses a denom whose IssuerDid (modules/tokenfactory/keeper/admins.go:73) equals callerDid and emits set_denom_paused with denom, paused and issuer_did attributes. |
| <code>acceptance.6.248.2</code> | req | 6248.2 |
| <code>acceptance.6.248.2</code> | scenario | Given a denom bound to did:layerx:&lt;key A&gt; and a second denom with no issuer, when SetDenomPausedByIssuer is called with did:layerx:&lt;key B&gt; on the first and with did:layerx:&lt;key A&gt; on the second, then both return ErrIssuerMismatch and neither denom is paused |
| <code>acceptance.6.248.2</code> | test | modules/tokenfactory/keeper/issuer_pause_test.go::TestSetDenomPausedByIssuerRefusesOtherDid (planned) |
| <code>acceptance.6.248.2</code> | qualification | UNRUN |
| <code>req.6248</code> | ac_2 | SetDenomPausedByIssuer refuses a callerDid other than the bound issuer, and any caller of a denom with an empty issuer_did, with ErrIssuerMismatch, the kernel rule at src/modules/asset/lx_asset_execution.h:240-242. |
| <code>acceptance.6.249.1</code> | req | 6249.1 |
| <code>acceptance.6.249.1</code> | scenario | Given a denom bound to did:layerx:&lt;key A&gt;, when MintByIssuer mints 50 to an account, then the balance is 50; after SetDenomPausedByIssuer pauses it the same call returns ErrDenomPaused and the balance stays 50 |
| <code>acceptance.6.249.1</code> | test | modules/tokenfactory/keeper/issuer_supply_test.go::TestMintByIssuerRespectsPause (planned) |
| <code>acceptance.6.249.1</code> | qualification | UNRUN |
| <code>req.6249</code> | ac_1 | MintByIssuer(ctx, callerDid, amount, to) in modules/tokenfactory/keeper/issuer_supply.go (planned) mints through mintTo (modules/tokenfactory/keeper/bankactions.go:11) only for the bound issuer and returns ErrDenomPaused for a paused denom (modules/tokenfactory/keeper/bankactions.go:18-20). |
| <code>acceptance.6.249.2</code> | req | 6249.2 |
| <code>acceptance.6.249.2</code> | scenario | Given an account holding 50 of a denom bound to did:layerx:&lt;key A&gt;, when BurnByIssuer is called with did:layerx:&lt;key B&gt;, then it returns ErrIssuerMismatch and the balance stays 50; with did:layerx:&lt;key A&gt; and 20 the balance becomes 30 |
| <code>acceptance.6.249.2</code> | test | modules/tokenfactory/keeper/issuer_supply_test.go::TestBurnByIssuerRefusesOtherDid (planned) |
| <code>acceptance.6.249.2</code> | qualification | UNRUN |
| <code>req.6249</code> | ac_2 | BurnByIssuer(ctx, callerDid, amount, from) burns through burnFrom (modules/tokenfactory/keeper/bankactions.go:39) only for the bound issuer and refuses another DID with ErrIssuerMismatch. |
| <code>acceptance.6.250.1</code> | req | 6250.1 |
| <code>acceptance.6.250.1</code> | scenario | Given a created denom with no issuer, when SetIssuerDid binds did:layerx:&lt;key A&gt; twice, then the first call emits exactly one set_issuer_did event with issuer_did did:layerx:&lt;key A&gt; and the second emits none |
| <code>acceptance.6.250.1</code> | test | modules/tokenfactory/keeper/issuer_event_test.go::TestSetIssuerDidEmitsEvent (planned) |
| <code>acceptance.6.250.1</code> | qualification | UNRUN |
| <code>req.6250</code> | ac_1 | SetIssuerDid (modules/tokenfactory/keeper/admins.go:54-70) emits one set_issuer_did event with denom and issuer_did attributes when it binds a new issuer and emits nothing on the same-value no-op at modules/tokenfactory/keeper/admins.go:62-64. |
| <code>acceptance.6.252.1</code> | req | 6252.1 |
| <code>acceptance.6.252.1</code> | scenario | Given the app TokenFactoryKeeper held as a types.IssuerKeeper, when a denom is created, bound to did:layerx:&lt;key A&gt;, minted 10 and paused through the interface, then every call succeeds, IsDenomPaused is true and MintByIssuer then returns ErrDenomPaused |
| <code>acceptance.6.252.1</code> | test | modules/tokenfactory/keeper/issuer_keeper_assert_test.go::TestKeeperImplementsIssuerKeeper (planned) |
| <code>acceptance.6.252.1</code> | qualification | UNRUN |
| <code>req.6252</code> | ac_1 | types.IssuerKeeper in modules/tokenfactory/types/expected_issuer.go (planned) lists CreateDenom (modules/tokenfactory/keeper/createdenom.go:13), SetIssuerDid (modules/tokenfactory/keeper/admins.go:54), IssuerDid, IsDenomPaused (modules/tokenfactory/keeper/pause.go:12), SetDenomPausedByIssuer, MintByIssuer and BurnByIssuer, and the app keeper satisfies it. |
| <code>acceptance.6.253.1</code> | req | 6253.1 |
| <code>acceptance.6.253.1</code> | scenario | Given the integrated app with the layerxledger register entry point frozen by anchor 2.0.3, when a DID registers an asset and then pauses it through layerxledger, then IssuerDid of the created denom equals that DID and IsDenomPaused is true |
| <code>acceptance.6.253.1</code> | test | modules/tokenfactory/keeper/ledger_issuer_bind_test.go::TestLedgerRegisterBindsIssuerDid (planned) |
| <code>acceptance.6.253.1</code> | qualification | UNRUN |
| <code>req.6253</code> | ac_1 | A layerxledger register validated by ValidateRegisterPayload (modules/layerxledger/types/msgs.go:62) creates the tokenfactory denom with issuer_did equal to the registering DID, and a ledger pause mirrors into IsDenomPaused (modules/tokenfactory/keeper/pause.go:12). |
| <code>acceptance.6.269.1</code> | req | 6269.1 |
| <code>acceptance.6.269.1</code> | scenario | Given a denom with supply 1000000 bound to did:layerx:&lt;key A&gt;, when it is paused and unpaused through SetDenomPausedByIssuer, then bank supply stays 1000000 and matches the supplyStart and supplyEnd of both receipt fixtures |
| <code>acceptance.6.269.1</code> | test | modules/tokenfactory/keeper/issuer_pause_fixture_test.go::TestIssuerPauseFixtureParity (planned) |
| <code>acceptance.6.269.1</code> | qualification | UNRUN |
| <code>req.6269</code> | ac_1 | Pausing and unpausing through SetDenomPausedByIssuer leaves supply unchanged and reproduces the zero-amount, equal-supply outcome that denomPauseReadReceipt (modules/tokenfactory/keeper/denom_pause_test.go:83) reads from tests/fixtures/asset/supply-pause.receipt and tests/fixtures/asset/supply-unpause.receipt. |
| <code>capability.6.22.1</code> | title | Run DenomAuthorityMetadata.Validate inside tokenfactory GenesisState.Validate |
| <code>capability.6.22.1</code> | reqs | 6246.1 |
| <code>capability.6.22.3</code> | title | Add issuer-bound SetDenomPausedByIssuer to the tokenfactory keeper |
| <code>capability.6.22.3</code> | reqs | 6248.1<br>6248.2 |
| <code>capability.6.22.4</code> | title | Add MintByIssuer and BurnByIssuer keeper entry points |
| <code>capability.6.22.4</code> | reqs | 6249.1<br>6249.2 |
| <code>capability.6.22.5</code> | title | Emit a set_issuer_did event from SetIssuerDid |
| <code>capability.6.22.5</code> | reqs | 6250.1 |
| <code>capability.6.22.7</code> | title | Freeze the IssuerKeeper interface that layerxledger consumes |
| <code>capability.6.22.7</code> | reqs | 6252.1 |
| <code>capability.6.22.8</code> | title | Prove a layerxledger-registered denom carries its issuer DID and pause |
| <code>capability.6.22.8</code> | reqs | 6253.1 |
| <code>capability.6.27.1</code> | title | Check issuer pause outcomes against the supply-pause and supply-unpause receipts |
| <code>capability.6.27.1</code> | reqs | 6269.1 |
| <code>resolution.17.719</code> | title | decision.623 binds: Denom pause authority is issuer-bound and not governance-routed |
| <code>resolution.17.719</code> | applies_to | decision.623<br>task.6.22.3<br>task.6.22.6<br>task.6.27.2<br>task.6.28.6<br>task.6.25.2 |
| <code>resolution.17.719</code> | source | decision.623 in fragments/J35/decision.kvx; tasks from affects, task text naming decision.623 |

## keeper

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-tokenfactory-bankkeeper-j35</code> | <code>go_exported</code> | <code>change</code> | <code>modules/tokenfactory/types/expected_keepers.go:9</code> | <code>type BankKeeper interface</code> |
| <code>interface.6.go-tokenfactory-distrkeeper-j35</code> | <code>go_exported</code> | <code>retire</code> | <code>modules/tokenfactory/types/expected_keepers.go:40</code> | <code>type DistrKeeper interface { FundCommunityPool(ctx sdk.Context, amount sdk.Coins, sender sdk.AccAddress) error }</code> |
| <code>interface.6.go-tokenfactory-newkeeper-j35</code> | <code>go_exported</code> | <code>change</code> | <code>modules/tokenfactory/keeper/keeper.go:27</code> | <code>func NewKeeper(cdc codec.Codec, storeKey sdk.StoreKey, paramSpace paramtypes.Subspace, accountKeeper types.AccountKeeper, bankKeeper types.BankKeeper, distrKeeper types.DistrKeeper) Keeper</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-tokenfactory-bankkeeper-j35</code> | notes | trimmed to the nine called methods |
| <code>interface.6.go-tokenfactory-distrkeeper-j35</code> | notes | field stored, never called |
| <code>interface.6.go-tokenfactory-newkeeper-j35</code> | notes | drops the unused distrKeeper parameter |

Contract context <code>contract_context.6.tokenfactory-pause-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | tokenfactory Msg SetDenomPaused (api/tokenfactory/tx.proto), planned Query DenomPaused GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused, keeper SetDenomPausedByIssuer, MintByIssuer, BurnByIssuer and the IssuerKeeper interface consumed by layerxledger, CLI set-denom-paused and denom-paused, wasm set_denom_paused and denom_paused |
| storage | Root store of the tokenfactory key: paused&#124;&lt;denom&gt; -&gt; 0x01 (modules/tokenfactory/types/keys.go:55), deleted on unpause; issuer_did inside DenomAuthorityMetadata under denoms&#124;&lt;denom&gt;&#124;authoritymetadata; GenesisState field 3 paused_denoms carries the paused set across export |
| errors | ErrDenomPaused 26 LXP_ASSET_PAUSED, ErrPauseUnchanged 27 LXP_PAUSED_SCOPE (modules/tokenfactory/types/errors.go:34-35), planned ErrIssuerMismatch 28 LXP_UNAUTHORIZED_DEBIT, ErrEncodeTokenFactorySetDenomPaused 29, ErrEncodingDenomPaused 30; bank paths refuse with ErrSendDisabled |
| events | set_denom_paused { denom, paused, issuer_did on the issuer path }, set_issuer_did { denom, issuer_did } |
| gas | No fixed surcharge: Cosmos store gas of one Has per IsDenomPaused and one Set or Delete per pause change |
| activation | Bank msg-server checks are live at HEAD through RegisterSendEnabledChecker (node/app.go:766); the GigaBankKeeper registration and the issuer path land at the merge revision with no height gate because a pause only exists after an explicit pause call |
| invariants | Every paused&#124; key names a denom with authority metadata (PausedDenomsInvariant); pause never changes supply; an issuer-bound denom changes pause only through its issuer DID |
| pause | Per-denom pause refuses MsgSend, MsgMultiSend, GigaBankKeeper SendCoins and SendCoinsAndWei, tokenfactory Mint and Burn and MintByIssuer and BurnByIssuer; keeper-level x/bank SendCoins and module-account flows are not gated (decision.621); there is no governance per-denom pause (decision.623) |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.266.1</code> | req | 6266.1 |
| <code>acceptance.6.266.1</code> | scenario | Given the integrated app built with the new NewKeeper signature, when a denom is created and minted through the msg server, then both succeed and the keeper struct has no distrKeeper field |
| <code>acceptance.6.266.1</code> | test | modules/tokenfactory/keeper/keeper_deps_test.go::TestNewKeeperWithoutDistrKeeper (planned) |
| <code>acceptance.6.266.1</code> | qualification | UNRUN |
| <code>req.6266</code> | ac_1 | keeper.NewKeeper (modules/tokenfactory/keeper/keeper.go:27-47) takes no DistrKeeper: the distrKeeper field (modules/tokenfactory/keeper/keeper.go:22) and DistrKeeper interface (modules/tokenfactory/types/expected_keepers.go:40) are removed and the app keeper still creates and mints a denom. |
| <code>acceptance.6.267.1</code> | req | 6267.1 |
| <code>acceptance.6.267.1</code> | scenario | Given the tokenfactory types package, when the method set of types.BankKeeper is read by reflection, then it equals the nine-method list, and the sdk bank keeper still satisfies it |
| <code>acceptance.6.267.1</code> | test | modules/tokenfactory/types/expected_keepers_test.go::TestBankKeeperMethodSetFrozen (planned) |
| <code>acceptance.6.267.1</code> | qualification | UNRUN |
| <code>req.6267</code> | ac_1 | types.BankKeeper (modules/tokenfactory/types/expected_keepers.go:9-31) lists exactly GetDenomMetaData, SetDenomMetaData, SetDenomAllowList, GetDenomAllowList, HasSupply, SendCoinsFromModuleToAccount, SendCoinsFromAccountToModule, MintCoins and BurnCoins; IterateTotalSupply, DeferredSendCoinsFromAccountToModule, DelegateCoinsFromAccountToModule, UndelegateCoinsFromModuleToAccount and SendCoins are removed. |
| <code>capability.6.26.2</code> | title | Drop the unused distrKeeper from the tokenfactory keeper |
| <code>capability.6.26.2</code> | reqs | 6266.1 |
| <code>capability.6.26.3</code> | title | Trim the tokenfactory BankKeeper expected interface to the methods it calls |
| <code>capability.6.26.3</code> | reqs | 6267.1 |

## pause-authority

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.msg-tokenfactory-setdenompaused</code> | <code>cosmos_msg</code> | <code>change</code> | <code>api/tokenfactory/tx.proto:18</code> | <code>paxprotocol.paxchain.tokenfactory.Msg/SetDenomPaused: rpc SetDenomPaused(MsgSetDenomPaused) returns (MsgSetDenomPausedResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.msg-tokenfactory-setdenompaused</code> | notes | handler modules/tokenfactory/keeper/msg_server.go:220; no in-repo caller constructs the request type; no CLI tx command (modules/tokenfactory/client/cli/tx.go:34 mounts six commands, none for pause) |
| <code>interface.6.msg-tokenfactory-setdenompaused</code> | overridden_by | override.J35.3 |

Contract context <code>contract_context.6.tokenfactory-pause-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | tokenfactory Msg SetDenomPaused (api/tokenfactory/tx.proto), planned Query DenomPaused GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused, keeper SetDenomPausedByIssuer, MintByIssuer, BurnByIssuer and the IssuerKeeper interface consumed by layerxledger, CLI set-denom-paused and denom-paused, wasm set_denom_paused and denom_paused |
| storage | Root store of the tokenfactory key: paused&#124;&lt;denom&gt; -&gt; 0x01 (modules/tokenfactory/types/keys.go:55), deleted on unpause; issuer_did inside DenomAuthorityMetadata under denoms&#124;&lt;denom&gt;&#124;authoritymetadata; GenesisState field 3 paused_denoms carries the paused set across export |
| errors | ErrDenomPaused 26 LXP_ASSET_PAUSED, ErrPauseUnchanged 27 LXP_PAUSED_SCOPE (modules/tokenfactory/types/errors.go:34-35), planned ErrIssuerMismatch 28 LXP_UNAUTHORIZED_DEBIT, ErrEncodeTokenFactorySetDenomPaused 29, ErrEncodingDenomPaused 30; bank paths refuse with ErrSendDisabled |
| events | set_denom_paused { denom, paused, issuer_did on the issuer path }, set_issuer_did { denom, issuer_did } |
| gas | No fixed surcharge: Cosmos store gas of one Has per IsDenomPaused and one Set or Delete per pause change |
| activation | Bank msg-server checks are live at HEAD through RegisterSendEnabledChecker (node/app.go:766); the GigaBankKeeper registration and the issuer path land at the merge revision with no height gate because a pause only exists after an explicit pause call |
| invariants | Every paused&#124; key names a denom with authority metadata (PausedDenomsInvariant); pause never changes supply; an issuer-bound denom changes pause only through its issuer DID |
| pause | Per-denom pause refuses MsgSend, MsgMultiSend, GigaBankKeeper SendCoins and SendCoinsAndWei, tokenfactory Mint and Burn and MintByIssuer and BurnByIssuer; keeper-level x/bank SendCoins and module-account flows are not gated (decision.621); there is no governance per-denom pause (decision.623) |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.251.1</code> | req | 6251.1 |
| <code>acceptance.6.251.1</code> | scenario | Given no denom factory/&lt;creator&gt;/missing exists, when its creator sends MsgSetDenomPaused true, then the handler returns ErrDenomDoesNotExist and no paused&#124; key is written |
| <code>acceptance.6.251.1</code> | test | modules/tokenfactory/keeper/set_denom_paused_refusal_test.go::TestSetDenomPausedMissingDenom (planned) |
| <code>acceptance.6.251.1</code> | qualification | UNRUN |
| <code>req.6251</code> | ac_1 | MsgSetDenomPaused for a denom with no authority metadata returns ErrDenomDoesNotExist (modules/tokenfactory/types/errors.go:21) instead of the ErrUnauthorized that the empty-admin comparison at modules/tokenfactory/keeper/msg_server.go:228-230 yields today. |
| <code>acceptance.6.251.2</code> | req | 6251.2 |
| <code>acceptance.6.251.2</code> | scenario | Given denom a bound to did:layerx:&lt;key A&gt; and denom b with no issuer, both administered by account X, when X sends MsgSetDenomPaused true for a and for b, then a returns ErrIssuerMismatch and stays unpaused and b is paused |
| <code>acceptance.6.251.2</code> | test | modules/tokenfactory/keeper/set_denom_paused_refusal_test.go::TestSetDenomPausedRefusesAdminWhenIssuerBound (planned) |
| <code>acceptance.6.251.2</code> | qualification | UNRUN |
| <code>req.6251</code> | ac_2 | MsgSetDenomPaused from the bech32 admin of a denom whose issuer_did is set returns ErrIssuerMismatch, so only SetDenomPausedByIssuer changes the pause of an issuer-bound denom; denoms without an issuer keep the admin rule of modules/tokenfactory/keeper/msg_server.go:228. |
| <code>acceptance.6.270.1</code> | req | 6270.1 |
| <code>acceptance.6.270.1</code> | scenario | Given an issuer-bound denom and an unbound denom, when pause is attempted by the issuer, another DID, the bech32 admin and the issuer again, then the outcomes are success, ErrIssuerMismatch, ErrIssuerMismatch and ErrPauseUnchanged, and on the unbound denom SetDenomPausedByIssuer returns ErrIssuerMismatch while the admin MsgSetDenomPaused succeeds |
| <code>acceptance.6.270.1</code> | test | modules/tokenfactory/keeper/issuer_pause_matrix_test.go::TestIssuerPauseAuthorityMatrix (planned) |
| <code>acceptance.6.270.1</code> | qualification | UNRUN |
| <code>req.6270</code> | ac_1 | The tokenfactory pause authority matches src/modules/asset/lx_asset_execution.h:240-243: issuer DID allowed, other DID and unbound issuer refused with LXP_UNAUTHORIZED_DEBIT, repeat refused with LXP_PAUSED_SCOPE, and the bech32 admin refused for an issuer-bound denom. |
| <code>capability.6.22.6</code> | title | Refuse SetDenomPaused for a missing or issuer-bound denom |
| <code>capability.6.22.6</code> | reqs | 6251.1<br>6251.2 |
| <code>capability.6.27.2</code> | title | Check the issuer pause authority matrix against the kernel asset rules |
| <code>capability.6.27.2</code> | reqs | 6270.1 |
| <code>override.J35.3</code> | target | interface.6.msg-tokenfactory-setdenompaused |
| <code>override.J35.3</code> | status | change |
| <code>override.J35.3</code> | basis | SetDenomPaused at modules/tokenfactory/keeper/msg_server.go:220-245 answers a missing denom with ErrUnauthorized through the empty-admin comparison at :228-230 and lets the bech32 admin pause an issuer-bound denom. |
| <code>resolution.17.720</code> | title | decision.624 binds: LXP_PAUSED_SCOPE text in two codespaces is kept |
| <code>resolution.17.720</code> | applies_to | decision.624<br>task.6.27.2<br>task.6.28.6 |
| <code>resolution.17.720</code> | source | decision.624 in fragments/J35/decision.kvx; tasks from affects |
| <code>resolution.17.729</code> | title | decision.633 binds: An issuer binding closes the bech32 admin pause path |
| <code>resolution.17.729</code> | applies_to | decision.633<br>task.6.22.6 |
| <code>resolution.17.729</code> | source | decision.633 in fragments/J35/decision.kvx; tasks from affects |

## pause-bank-paths

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.263.1</code> | req | 6263.1 |
| <code>acceptance.6.263.1</code> | scenario | Given the integrated app with a paused denom held by account X, when GigaBankKeeper.SendCoins and SendCoinsAndWei move it from X to Y, then both return ErrSendDisabled and balances are unchanged; after unpause the same SendCoins succeeds and an unrelated denom always succeeds |
| <code>acceptance.6.263.1</code> | test | modules/tokenfactory/keeper/denom_pause_giga_test.go::TestDenomPauseGigaBankRefuses (planned) |
| <code>acceptance.6.263.1</code> | qualification | UNRUN |
| <code>req.6263</code> | ac_1 | A paused tokenfactory denom is refused by GigaBankKeeper IsSendEnabledCoin (engine/deps/xbank/keeper/send.go:366), which today reads only params, and by its SendCoins (engine/deps/xbank/keeper/send.go:171) once SendEnabledChecker (modules/tokenfactory/keeper/pause.go:32) is registered on it beside node/app.go:918. |
| <code>acceptance.6.264.1</code> | req | 6264.1 |
| <code>acceptance.6.264.1</code> | scenario | Given the integrated app with paused denom a and unpaused denom b, when each of the six paths moves a and then b, then every move of a returns ErrSendDisabled or ErrDenomPaused with balances unchanged and every move of b succeeds |
| <code>acceptance.6.264.1</code> | test | modules/tokenfactory/keeper/denom_pause_conformance_test.go::TestDenomPauseBankPathConformance (planned) |
| <code>acceptance.6.264.1</code> | qualification | UNRUN |
| <code>req.6264</code> | ac_1 | With one denom paused, MsgSend and MsgMultiSend (sdk/x/bank/keeper/msg_server.go:34, sdk/x/bank/keeper/msg_server.go:95), GigaBankKeeper SendCoins, tokenfactory Mint and Burn (modules/tokenfactory/keeper/bankactions.go:18-20, :46-48) and MintByIssuer all refuse it, while every path accepts an unpaused denom. |
| <code>capability.6.25.1</code> | title | Refuse paused tokenfactory denoms on the GigaBankKeeper send path |
| <code>capability.6.25.1</code> | reqs | 6263.1 |
| <code>capability.6.25.2</code> | title | Check pause refusals across every user bank path of a tokenfactory denom |
| <code>capability.6.25.2</code> | reqs | 6264.1 |
| <code>resolution.17.717</code> | title | decision.621 binds: Denom pause gates user transfer paths, not keeper-level module flows |
| <code>resolution.17.717</code> | applies_to | decision.621<br>task.6.25.1<br>task.6.25.2 |
| <code>resolution.17.717</code> | source | decision.621 in fragments/J35/decision.kvx; tasks from affects |
| <code>resolution.17.718</code> | title | decision.622 binds: GigaBankKeeper consults the tokenfactory send-enabled checker |
| <code>resolution.17.718</code> | applies_to | decision.622<br>task.6.25.1<br>task.5.0.3<br>task.5.56.1 |
| <code>resolution.17.718</code> | source | decision.622 in fragments/J35/decision.kvx; tasks from affects, tasks naming the affected blocks |

## pause-store

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-tokenfactory-getpauseddenoms-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/tokenfactory/keeper/pause.go</code> (planned) | <code>func (k Keeper) GetPausedDenoms(ctx sdk.Context) []string</code> |
| <code>interface.6.key-tokenfactory-admin-j35</code> | <code>storage_key</code> | <code>retire</code> | <code>modules/tokenfactory/types/keys.go:31</code> | <code>AdminPrefixKey = "admin"</code> |
| <code>interface.6.key-tokenfactory-allowlistmaxsize-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/tokenfactory/types/keys.go:34</code> | <code>DenomAllowListMaxSizeKey = []byte("allowlistmaxsize")</code> |
| <code>interface.6.key-tokenfactory-authoritymetadata-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/tokenfactory/types/keys.go:28</code> | <code>DenomAuthorityMetadataKey = "authoritymetadata" under denoms&#124;&lt;denom&gt;&#124;</code> |
| <code>interface.6.key-tokenfactory-createdenomfeewhitelist-j35</code> | <code>storage_key</code> | <code>retire</code> | <code>modules/tokenfactory/types/keys.go:33</code> | <code>CreateDenomFeeWhitelistKey = "createdenomfeewhitelist"</code> |
| <code>interface.6.key-tokenfactory-creator-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/tokenfactory/types/keys.go:30</code> | <code>CreatorPrefixKey = "creator"; GetCreatorPrefix(creator) = creator&#124;&lt;creator&gt;&#124;</code> |
| <code>interface.6.key-tokenfactory-denoms-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/tokenfactory/types/keys.go:29</code> | <code>DenomsPrefixKey = "denoms"; GetDenomPrefixStore(denom) = denoms&#124;&lt;denom&gt;&#124;</code> |
| <code>interface.6.key-tokenfactory-paused-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/tokenfactory/types/keys.go:32</code> | <code>DenomPausedPrefixKey = "paused"; GetDenomPausedKey(denom) = paused&#124;&lt;denom&gt; -&gt; 0x01</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-tokenfactory-getpauseddenoms-j35</code> | notes | planned beside IsDenomPaused |
| <code>interface.6.key-tokenfactory-admin-j35</code> | notes | never read or written |
| <code>interface.6.key-tokenfactory-allowlistmaxsize-j35</code> | notes | root store of the tokenfactory store key |
| <code>interface.6.key-tokenfactory-authoritymetadata-j35</code> | notes | root store of the tokenfactory store key |
| <code>interface.6.key-tokenfactory-createdenomfeewhitelist-j35</code> | notes | never read or written |
| <code>interface.6.key-tokenfactory-creator-j35</code> | notes | root store of the tokenfactory store key |
| <code>interface.6.key-tokenfactory-denoms-j35</code> | notes | root store of the tokenfactory store key |
| <code>interface.6.key-tokenfactory-paused-j35</code> | notes | root store of the tokenfactory store key |

Contract context <code>contract_context.6.tokenfactory-pause-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | tokenfactory Msg SetDenomPaused (api/tokenfactory/tx.proto), planned Query DenomPaused GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused, keeper SetDenomPausedByIssuer, MintByIssuer, BurnByIssuer and the IssuerKeeper interface consumed by layerxledger, CLI set-denom-paused and denom-paused, wasm set_denom_paused and denom_paused |
| storage | Root store of the tokenfactory key: paused&#124;&lt;denom&gt; -&gt; 0x01 (modules/tokenfactory/types/keys.go:55), deleted on unpause; issuer_did inside DenomAuthorityMetadata under denoms&#124;&lt;denom&gt;&#124;authoritymetadata; GenesisState field 3 paused_denoms carries the paused set across export |
| errors | ErrDenomPaused 26 LXP_ASSET_PAUSED, ErrPauseUnchanged 27 LXP_PAUSED_SCOPE (modules/tokenfactory/types/errors.go:34-35), planned ErrIssuerMismatch 28 LXP_UNAUTHORIZED_DEBIT, ErrEncodeTokenFactorySetDenomPaused 29, ErrEncodingDenomPaused 30; bank paths refuse with ErrSendDisabled |
| events | set_denom_paused { denom, paused, issuer_did on the issuer path }, set_issuer_did { denom, issuer_did } |
| gas | No fixed surcharge: Cosmos store gas of one Has per IsDenomPaused and one Set or Delete per pause change |
| activation | Bank msg-server checks are live at HEAD through RegisterSendEnabledChecker (node/app.go:766); the GigaBankKeeper registration and the issuer path land at the merge revision with no height gate because a pause only exists after an explicit pause call |
| invariants | Every paused&#124; key names a denom with authority metadata (PausedDenomsInvariant); pause never changes supply; an issuer-bound denom changes pause only through its issuer DID |
| pause | Per-denom pause refuses MsgSend, MsgMultiSend, GigaBankKeeper SendCoins and SendCoinsAndWei, tokenfactory Mint and Burn and MintByIssuer and BurnByIssuer; keeper-level x/bank SendCoins and module-account flows are not gated (decision.621); there is no governance per-denom pause (decision.623) |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.243.1</code> | req | 6243.1 |
| <code>acceptance.6.243.1</code> | scenario | Given three created factory denoms of which b and a are paused through MsgSetDenomPaused, when GetPausedDenoms is called, then it returns [a, b]; after a is unpaused it returns [b] |
| <code>acceptance.6.243.1</code> | test | modules/tokenfactory/keeper/paused_denoms_test.go::TestGetPausedDenomsSorted (planned) |
| <code>acceptance.6.243.1</code> | qualification | UNRUN |
| <code>req.6243</code> | ac_1 | Keeper.GetPausedDenoms returns every denom stored under GetDenomPausedKey (modules/tokenfactory/types/keys.go:55) in ascending byte order and drops a denom once setDenomPaused (modules/tokenfactory/keeper/pause.go:17) unpauses it. |
| <code>acceptance.6.265.1</code> | req | 6265.1 |
| <code>acceptance.6.265.1</code> | scenario | Given the tokenfactory types package, when GetDenomPrefixStore(d), GetCreatorPrefix(c), GetCreatorsPrefix() and GetDenomPausedKey(d) are computed, then they equal denoms&#124;d&#124;, creator&#124;c&#124;, creator&#124; and paused&#124;d byte for byte |
| <code>acceptance.6.265.1</code> | test | modules/tokenfactory/types/keys_layout_test.go::TestStoreKeyLayoutFrozen (planned) |
| <code>acceptance.6.265.1</code> | qualification | UNRUN |
| <code>req.6265</code> | ac_1 | The tokenfactory store layout is frozen to authoritymetadata, denoms, creator, paused and allowlistmaxsize (modules/tokenfactory/types/keys.go:28-34) with KeySeparator &#124; (modules/tokenfactory/types/keys.go:25); the unused AdminPrefixKey and CreateDenomFeeWhitelistKey (modules/tokenfactory/types/keys.go:31, :33) are removed. |
| <code>capability.6.21.3</code> | title | Add GetPausedDenoms iterating the tokenfactory paused&#124; prefix |
| <code>capability.6.21.3</code> | reqs | 6243.1 |
| <code>capability.6.26.1</code> | title | Freeze the tokenfactory store key layout and drop unused prefixes |
| <code>capability.6.26.1</code> | reqs | 6265.1 |

## queries

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.proto-tokenfactory-querydenompausedrequest-j35</code> | <code>proto_message</code> | <code>new</code> | <code>api/tokenfactory/query.proto</code> (planned) | <code>message paxprotocol.paxchain.tokenfactory.QueryDenomPausedRequest { string denom = 1; }</code> |
| <code>interface.6.proto-tokenfactory-querydenompausedresponse-j35</code> | <code>proto_message</code> | <code>new</code> | <code>api/tokenfactory/query.proto</code> (planned) | <code>message paxprotocol.paxchain.tokenfactory.QueryDenomPausedResponse { bool paused = 1; string issuer_did = 2; }</code> |
| <code>interface.6.qry-tokenfactory-denompaused-j35</code> | <code>grpc_query</code> | <code>new</code> | <code>api/tokenfactory/query.proto</code> (planned) | <code>paxprotocol.paxchain.tokenfactory.Query/DenomPaused: rpc DenomPaused(QueryDenomPausedRequest) returns (QueryDenomPausedResponse)</code> |
| <code>interface.6.rest-tokenfactory-denompaused-j35</code> | <code>rest_route</code> | <code>new</code> | <code>api/tokenfactory/query.proto</code> (planned) | <code>GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.proto-tokenfactory-querydenompausedrequest-j35</code> | notes | planned |
| <code>interface.6.proto-tokenfactory-querydenompausedresponse-j35</code> | notes | planned |
| <code>interface.6.qry-tokenfactory-denompaused-j35</code> | notes | planned rpc after DenomAllowList |
| <code>interface.6.rest-tokenfactory-denompaused-j35</code> | notes | gateway route of DenomPaused |

Contract context <code>contract_context.6.tokenfactory-pause-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | tokenfactory Msg SetDenomPaused (api/tokenfactory/tx.proto), planned Query DenomPaused GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused, keeper SetDenomPausedByIssuer, MintByIssuer, BurnByIssuer and the IssuerKeeper interface consumed by layerxledger, CLI set-denom-paused and denom-paused, wasm set_denom_paused and denom_paused |
| storage | Root store of the tokenfactory key: paused&#124;&lt;denom&gt; -&gt; 0x01 (modules/tokenfactory/types/keys.go:55), deleted on unpause; issuer_did inside DenomAuthorityMetadata under denoms&#124;&lt;denom&gt;&#124;authoritymetadata; GenesisState field 3 paused_denoms carries the paused set across export |
| errors | ErrDenomPaused 26 LXP_ASSET_PAUSED, ErrPauseUnchanged 27 LXP_PAUSED_SCOPE (modules/tokenfactory/types/errors.go:34-35), planned ErrIssuerMismatch 28 LXP_UNAUTHORIZED_DEBIT, ErrEncodeTokenFactorySetDenomPaused 29, ErrEncodingDenomPaused 30; bank paths refuse with ErrSendDisabled |
| events | set_denom_paused { denom, paused, issuer_did on the issuer path }, set_issuer_did { denom, issuer_did } |
| gas | No fixed surcharge: Cosmos store gas of one Has per IsDenomPaused and one Set or Delete per pause change |
| activation | Bank msg-server checks are live at HEAD through RegisterSendEnabledChecker (node/app.go:766); the GigaBankKeeper registration and the issuer path land at the merge revision with no height gate because a pause only exists after an explicit pause call |
| invariants | Every paused&#124; key names a denom with authority metadata (PausedDenomsInvariant); pause never changes supply; an issuer-bound denom changes pause only through its issuer DID |
| pause | Per-denom pause refuses MsgSend, MsgMultiSend, GigaBankKeeper SendCoins and SendCoinsAndWei, tokenfactory Mint and Burn and MintByIssuer and BurnByIssuer; keeper-level x/bank SendCoins and module-account flows are not gated (decision.621); there is no governance per-denom pause (decision.623) |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.254.1</code> | req | 6254.1 |
| <code>acceptance.6.254.1</code> | scenario | Given the regenerated modules/tokenfactory/types/query.pb.go, when QueryDenomPausedResponse{Paused: true, IssuerDid: did:layerx:&lt;key A&gt;} is marshalled and the gateway pattern for DenomPaused is read, then field 1 is a varint, field 2 a string and the pattern is /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused |
| <code>acceptance.6.254.1</code> | test | modules/tokenfactory/types/query_denom_paused_test.go::TestQueryDenomPausedWireShape (planned) |
| <code>acceptance.6.254.1</code> | qualification | UNRUN |
| <code>req.6254</code> | ac_1 | service Query (api/tokenfactory/query.proto:14-43) gains rpc DenomPaused(QueryDenomPausedRequest{string denom = 1}) returns (QueryDenomPausedResponse{bool paused = 1; string issuer_did = 2}) at GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused. |
| <code>acceptance.6.255.1</code> | req | 6255.1 |
| <code>acceptance.6.255.1</code> | scenario | Given a denom bound to did:layerx:&lt;key A&gt; and paused, when DenomPaused is queried through the app query router, then the response is paused true with issuer_did did:layerx:&lt;key A&gt;, and after unpause paused is false |
| <code>acceptance.6.255.1</code> | test | modules/tokenfactory/keeper/grpc_denom_paused_test.go::TestQueryDenomPausedReportsState (planned) |
| <code>acceptance.6.255.1</code> | qualification | UNRUN |
| <code>req.6255</code> | ac_1 | Keeper.DenomPaused answers paused from IsDenomPaused (modules/tokenfactory/keeper/pause.go:12) and issuer_did from GetAuthorityMetadata (modules/tokenfactory/keeper/admins.go:11) for an existing denom. |
| <code>acceptance.6.255.2</code> | req | 6255.2 |
| <code>acceptance.6.255.2</code> | scenario | Given the keeper, when DenomPaused is called with nil, with denom "" and with factory/&lt;creator&gt;/missing, then the status codes are InvalidArgument, InvalidArgument and NotFound |
| <code>acceptance.6.255.2</code> | test | modules/tokenfactory/keeper/grpc_denom_paused_test.go::TestQueryDenomPausedRejectsEmptyDenom (planned) |
| <code>acceptance.6.255.2</code> | qualification | UNRUN |
| <code>req.6255</code> | ac_2 | Keeper.DenomPaused returns codes.InvalidArgument for a nil request or empty denom and codes.NotFound for an unknown denom, the shape of DenomMetadata at modules/tokenfactory/keeper/grpc_query.go:43-61. |
| <code>acceptance.6.259.1</code> | req | 6259.1 |
| <code>acceptance.6.259.1</code> | scenario | Given the integrated app with a paused denom a and an unpaused denom b, when DenomPaused is read through the gRPC query router, the REST gateway and the wasm handler, then all three report a paused and b unpaused with the same issuer_did |
| <code>acceptance.6.259.1</code> | test | modules/tokenfactory/keeper/denom_paused_surface_test.go::TestDenomPausedSurfaceConformance (planned) |
| <code>acceptance.6.259.1</code> | qualification | UNRUN |
| <code>req.6259</code> | ac_1 | For one paused and one unpaused denom the gRPC DenomPaused answer, the gateway handler registered from modules/tokenfactory/types/query.pb.gw.go:1 and the wasm GetDenomPaused answer agree with IsDenomPaused (modules/tokenfactory/keeper/pause.go:12). |
| <code>capability.6.23.1</code> | title | Add the DenomPaused query rpc and REST route to tokenfactory query.proto |
| <code>capability.6.23.1</code> | reqs | 6254.1 |
| <code>capability.6.23.2</code> | title | Serve the DenomPaused gRPC query from the tokenfactory keeper |
| <code>capability.6.23.2</code> | reqs | 6255.1<br>6255.2 |
| <code>capability.6.23.6</code> | title | Check DenomPaused answers agree across gRPC, REST gateway and wasm |
| <code>capability.6.23.6</code> | reqs | 6259.1 |

## wasm

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-tokenfactory-encodetokenfactorysetdenompaused-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/tokenfactory/client/wasm/encoder.go</code> (planned) | <code>func EncodeTokenFactorySetDenomPaused(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sdk.Msg, error)</code> |
| <code>interface.6.go-tokenfactory-paxtokenfactoryquery-j35</code> | <code>go_exported</code> | <code>change</code> | <code>modules/tokenfactory/client/wasm/bindings/queries.go:5</code> | <code>type PaxTokenFactoryQuery struct { DenomAuthorityMetadata; DenomsFromCreator }</code> |
| <code>interface.6.go-tokenfactory-wasm-getdenompaused-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/tokenfactory/client/wasm/query.go</code> (planned) | <code>func (handler TokenFactoryWasmQueryHandler) GetDenomPaused(ctx sdk.Context, req *types.QueryDenomPausedRequest) (*types.QueryDenomPausedResponse, error)</code> |
| <code>interface.6.wasm-pax-tokenfactory-denom-paused-j35</code> | <code>wasm_binding</code> | <code>new</code> | <code>modules/tokenfactory/client/wasm/bindings/queries.go</code> (planned) | <code>PaxTokenFactoryQuery.DenomPaused json:"denom_paused" -&gt; QueryDenomPausedResponse</code> |
| <code>interface.6.wasm-pax-tokenfactory-set-denom-paused-j35</code> | <code>wasm_binding</code> | <code>new</code> | <code>modules/tokenfactory/client/wasm/bindings/msgs.go</code> (planned) | <code>PaxWasmMessage.SetDenomPaused json:"set_denom_paused" { denom string; paused bool }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-tokenfactory-encodetokenfactorysetdenompaused-j35</code> | notes | planned |
| <code>interface.6.go-tokenfactory-paxtokenfactoryquery-j35</code> | notes | gains DenomPaused |
| <code>interface.6.go-tokenfactory-wasm-getdenompaused-j35</code> | notes | planned |
| <code>interface.6.wasm-pax-tokenfactory-denom-paused-j35</code> | notes | dispatched by node/wasmbinding/queries.go:99 after wiring |
| <code>interface.6.wasm-pax-tokenfactory-set-denom-paused-j35</code> | notes | encoded by EncodeTokenFactorySetDenomPaused |

Contract context <code>contract_context.6.modules-tokenfactory-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 10 surfaces under modules/tokenfactory (4 cli_command, 4 go_exported, 2 wasm_binding; 2 change, 8 new) frozen at the anchors contracts/tokenfactory.md#cli, contracts/tokenfactory.md#invariants, contracts/tokenfactory.md#wasm: paxd query tokenfactory denom-allow-list [denom] (modules/tokenfactory/client/cli/query.go (planned)); paxd query tokenfactory denom-metadata [denom] (modules/tokenfactory/client/cli/query.go (planned)); paxd query tokenfactory denom-paused [denom] (modules/tokenfactory/client/cli/query.go (planned)); paxd tx tokenfactory set-denom-paused [denom] [true&#124;false] (modules/tokenfactory/client/cli/tx.go (planned)); func (am AppModule) RegisterInvariants(_ sdk.InvariantRegistry) (modules/tokenfactory/module.go:163); func EncodeTokenFactorySetDenomPaused(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sd (modules/tokenfactory/client/wasm/encoder.go (planned)); type PaxTokenFactoryQuery struct { DenomAuthorityMetadata; DenomsFromCreator } (modules/tokenfactory/client/wasm/bindings/queries.go:5); func (handler TokenFactoryWasmQueryHandler) GetDenomPaused(ctx sdk.Context, req *types.Que (modules/tokenfactory/client/wasm/query.go (planned)); PaxTokenFactoryQuery.DenomPaused json:"denom_paused" -&gt; QueryDenomPausedResponse (modules/tokenfactory/client/wasm/bindings/queries.go (planned)); PaxWasmMessage.SetDenomPaused json:"set_denom_paused" { denom string; paused bool } (modules/tokenfactory/client/wasm/bindings/msgs.go (planned)) |
| storage | no storage_key item in this context; the modules/tokenfactory items read or write state only through the keepers and stores frozen by the other domain-6 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-6 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-6 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 6.23.3, 6.23.4, 6.23.5, 6.24.2, 6.24.3, 6.26.4 in waves 1, 2, 5 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 6 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.258.1</code> | req | 6258.1 |
| <code>acceptance.6.258.1</code> | scenario | Given a paused denom, when TokenFactoryWasmQueryHandler.GetDenomPaused is called with QueryDenomPausedRequest{Denom: d}, then it returns paused true, and for an unknown denom it returns the NotFound status |
| <code>acceptance.6.258.1</code> | test | modules/tokenfactory/client/wasm/denom_paused_query_test.go::TestWasmGetDenomPaused (planned) |
| <code>acceptance.6.258.1</code> | qualification | UNRUN |
| <code>req.6258</code> | ac_1 | PaxTokenFactoryQuery (modules/tokenfactory/client/wasm/bindings/queries.go:5-10) gains DenomPaused *types.QueryDenomPausedRequest `json:"denom_paused,omitempty"` and TokenFactoryWasmQueryHandler.GetDenomPaused answers it from the keeper query. |
| <code>acceptance.6.262.1</code> | req | 6262.1 |
| <code>acceptance.6.262.1</code> | scenario | Given a contract address and {"denom":"factory/&lt;creator&gt;/a","paused":true}, when EncodeTokenFactorySetDenomPaused runs, then it returns one MsgSetDenomPaused with that sender, denom and paused true, and for {"denom":1} it returns ErrEncodeTokenFactorySetDenomPaused |
| <code>acceptance.6.262.1</code> | test | modules/tokenfactory/client/wasm/encoder_set_denom_paused_test.go::TestEncodeTokenFactorySetDenomPaused (planned) |
| <code>acceptance.6.262.1</code> | qualification | UNRUN |
| <code>req.6262</code> | ac_1 | EncodeTokenFactorySetDenomPaused(rawMsg, sender) decodes bindings.SetDenomPaused{Denom, Paused} from modules/tokenfactory/client/wasm/bindings/msgs.go (planned) into MsgSetDenomPaused with Sender = contract address, and malformed JSON returns ErrEncodeTokenFactorySetDenomPaused, the pattern of modules/tokenfactory/client/wasm/encoder.go:11-21. |
| <code>capability.6.23.5</code> | title | Answer the DenomPaused tokenfactory wasm query |
| <code>capability.6.23.5</code> | reqs | 6258.1 |
| <code>capability.6.24.3</code> | title | Encode the wasm set_denom_paused tokenfactory message |
| <code>capability.6.24.3</code> | reqs | 6262.1 |
