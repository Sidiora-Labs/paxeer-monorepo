# Layerxgov contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## go-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-layerxgov-client-cli-decodelayerxproposal-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/client/cli/tx.go:98</code> | <code>func DecodeLayerXProposal(cdc codec.JSONCodec, body []byte) (*types.LayerXProposal, error)</code> |
| <code>interface.6.go-layerxgov-client-cli-layerxproposalrequest-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/client/cli/tx.go:181</code> | <code>type LayerXProposalRequest struct</code> |
| <code>interface.6.go-layerxgov-client-cli-layerxproposalresthandler-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/client/cli/tx.go:187</code> | <code>func LayerXProposalRESTHandler(clientCtx client.Context) govrest.ProposalRESTHandler</code> |
| <code>interface.6.go-layerxgov-client-cli-newsubmitlayerxproposalcmd-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/client/cli/tx.go:35</code> | <code>func NewSubmitLayerXProposalCmd() *cobra.Command</code> |
| <code>interface.6.go-layerxgov-client-cli-newsubmitlayerxproposalmsg-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/client/cli/tx.go:70</code> | <code>func NewSubmitLayerXProposalMsg(cdc codec.JSONCodec, body []byte, depositInput string, proposer sdk.AccAddress) (*govtypes.MsgSubmitProposal, error)</code> |
| <code>interface.6.go-layerxgov-module-newproposalhandler-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/handler.go:11</code> | <code>func NewProposalHandler(router *baseapp.MsgServiceRouter) govtypes.Handler</code> |
| <code>interface.6.go-layerxgov-types-checkpausescope-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/types/proposal.go:38</code> | <code>func CheckPauseScope(module string, paused bool, method string) error</code> |
| <code>interface.6.go-layerxgov-types-governanceauthority-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/types/proposal.go:77</code> | <code>func GovernanceAuthority() string { return authtypes.NewModuleAddress(govtypes.ModuleName).String() }</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-getdescription-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/types/proposal.go:93</code> | <code>func (p *LayerXProposal) GetDescription() string { return p.Description }</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-getmessages-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/types/proposal.go:124</code> | <code>func (p *LayerXProposal) GetMessages() ([]sdk.Msg, error)</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-gettitle-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/types/proposal.go:92</code> | <code>func (p *LayerXProposal) GetTitle() string       { return p.Title }</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-proposalroute-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/types/proposal.go:94</code> | <code>func (p *LayerXProposal) ProposalRoute() string  { return RouterKey }</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-proposaltype-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/types/proposal.go:95</code> | <code>func (p *LayerXProposal) ProposalType() string   { return ProposalTypeLayerX }</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-string-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/types/proposal.go:159</code> | <code>func (p LayerXProposal) String() string</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-unpackinterfaces-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/types/proposal.go:147</code> | <code>func (p LayerXProposal) UnpackInterfaces(unpacker cdctypes.AnyUnpacker) error</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-validatebasic-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/types/proposal.go:96</code> | <code>func (p *LayerXProposal) ValidateBasic() error</code> |
| <code>interface.6.go-layerxgov-types-registercodec-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/types/proposal.go:71</code> | <code>func RegisterCodec(cdc *codec.LegacyAmino)</code> |
| <code>interface.6.go-layerxgov-types-registerinterfaces-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxgov/types/proposal.go:74</code> | <code>func RegisterInterfaces(registry cdctypes.InterfaceRegistry)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-layerxgov-client-cli-decodelayerxproposal-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxgov-client-cli-layerxproposalrequest-j35</code> | notes | exported type; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxgov-client-cli-layerxproposalresthandler-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxgov-client-cli-newsubmitlayerxproposalcmd-j35</code> | notes | exported func; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxgov-client-cli-newsubmitlayerxproposalmsg-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxgov-module-newproposalhandler-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxgov-types-checkpausescope-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxgov-types-governanceauthority-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxgov-types-layerxproposal-getdescription-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxgov-types-layerxproposal-getmessages-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxgov-types-layerxproposal-gettitle-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxgov-types-layerxproposal-proposalroute-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxgov-types-layerxproposal-proposaltype-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxgov-types-layerxproposal-string-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxgov-types-layerxproposal-unpackinterfaces-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxgov-types-layerxproposal-validatebasic-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxgov-types-registercodec-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxgov-types-registerinterfaces-j35</code> | notes | exported func; referenced across packages |

## pause-routing

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-layerxgov-newlayerxproposal-j35</code> | <code>go_exported</code> | <code>change</code> | <code>modules/layerxgov/types/proposal.go:78</code> | <code>func NewLayerXProposal(title, description string, msgs ...sdk.Msg) (*LayerXProposal, error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-layerxgov-newlayerxproposal-j35</code> | notes | accepted message set gains layerxpay MsgUpdateParams through governanceMessage |

Contract context <code>contract_context.6.layerxgov-routing-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | LayerXProposal (api/layerxgov/proposal.proto:7) submitted through the gov module and executed by NewProposalHandler (modules/layerxgov/handler.go:11) through the app MsgServiceRouter; CheckPauseScope (modules/layerxgov/types/proposal.go:38) consumed by paused modules |
| storage | No store of its own: layerxgov has no keeper and no store key; every effect is a routed message of another module |
| errors | ErrPausedScope 735 LXP_PAUSED_SCOPE (modules/layerxgov/types/proposal.go:26); ErrInvalidProposalContent for a message outside governanceMessage or with a non-gov authority; ErrUnknownRequest for an unrouted message |
| events | The events of the routed messages, re-emitted from the CacheContext after every message succeeds (modules/layerxgov/handler.go:34-39) |
| gas | Gov EndBlocker execution; no layerxgov surcharge |
| activation | Live at HEAD; layerxpay MsgUpdateParams joins the accepted set when domain 2 lands it (anchor 2.0.2) |
| invariants | All messages of one proposal commit together or none does; every accepted message carries the gov module account as authority |
| pause | Module-scope pause only: layerxpay, layerxexchange, layerxledger and layerxauth exit sets in pauseExitPaths (modules/layerxgov/types/proposal.go:28-36); tokenfactory denom pause is issuer-bound and not routed here |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.272.1</code> | req | 6272.1 |
| <code>acceptance.6.272.1</code> | scenario | Given a layerxpay MsgUpdateParams with Authority GovernanceAuthority() and Params.Paused true, when NewLayerXProposal wraps it and ValidateBasic runs, then both succeed and the amino name is layerxpay/MsgUpdateParams, and with a foreign authority ValidateBasic returns ErrInvalidProposalContent |
| <code>acceptance.6.272.1</code> | test | modules/layerxgov/types/pay_pause_route_test.go::TestLayerXProposalRoutesPayPause (planned) |
| <code>acceptance.6.272.1</code> | qualification | UNRUN |
| <code>req.6272</code> | ac_1 | governanceMessage (modules/layerxgov/types/proposal.go:169-222) accepts layerxpay MsgUpdateParams with the governance authority, so the pay pause exit set in pauseExitPaths (modules/layerxgov/types/proposal.go:29-32) can be switched on through a LayerXProposal; at HEAD the pay import serves only ModuleName (modules/layerxgov/types/proposal.go:12). |
| <code>acceptance.6.273.1</code> | req | 6273.1 |
| <code>acceptance.6.273.1</code> | scenario | Given the integrated app with layerxpay unpaused, when the gov route handles a LayerXProposal carrying MsgUpdateParams Paused true, then the layerxpay params read Paused true; when a second proposal carries that message plus a foreign-authority message, then the handler returns an error and Paused keeps its prior value |
| <code>acceptance.6.273.1</code> | test | modules/layerxgov/handler_pay_pause_live_test.go::TestLayerXProposalPausesPayLive (planned) |
| <code>acceptance.6.273.1</code> | qualification | UNRUN |
| <code>req.6273</code> | ac_1 | NewProposalHandler (modules/layerxgov/handler.go:11) dispatches a layerxpay MsgUpdateParams through the app MsgServiceRouter (node/app.go:969) inside one CacheContext (modules/layerxgov/handler.go:24), so pay Params.Paused flips and the write commits only when every message succeeds. |
| <code>acceptance.6.274.1</code> | req | 6274.1 |
| <code>acceptance.6.274.1</code> | scenario | Given the integrated app with the layerxledger and layerxauth msg servers frozen by anchor 2.0.3, when the gov route handles a LayerXProposal pausing both modules, then both params read paused and CheckPauseScope for a non-exit method of each returns ErrPausedScope while the revokeGrant exit returns nil |
| <code>acceptance.6.274.1</code> | test | modules/layerxgov/handler_pause_live_test.go::TestLayerXProposalPausesLedgerAndAuthLive (planned) |
| <code>acceptance.6.274.1</code> | qualification | UNRUN |
| <code>req.6274</code> | ac_1 | A LayerXProposal carrying layerxledger and layerxauth MsgUpdateParams (modules/layerxgov/types/proposal.go:190-193) executes through the app gov route (node/app.go:969) once domain 2 serves both messages, and CheckPauseScope (modules/layerxgov/types/proposal.go:38) then refuses a non-exit method of each with ErrPausedScope code 735. |
| <code>acceptance.6.276.1</code> | req | 6276.1 |
| <code>acceptance.6.276.1</code> | scenario | Given the pauseExitPaths table at the integrated revision, when CheckPauseScope is called for every module with each exit method and one non-exit method, paused and unpaused, then the outcomes match the table and a module named tokenfactory returns ErrInvalidRequest |
| <code>acceptance.6.276.1</code> | test | modules/layerxgov/types/pause_conformance_test.go::TestGovernancePauseConformance (planned) |
| <code>acceptance.6.276.1</code> | qualification | UNRUN |
| <code>req.6276</code> | ac_1 | For each module key of pauseExitPaths (modules/layerxgov/types/proposal.go:28-36) CheckPauseScope returns nil when unpaused, nil for every listed exit when paused, ErrPausedScope for any other method when paused, and ErrInvalidRequest for a module outside the table. |
| <code>capability.6.28.2</code> | title | Route the layerxpay MsgUpdateParams pause through LayerXProposal |
| <code>capability.6.28.2</code> | reqs | 6272.1 |
| <code>capability.6.28.3</code> | title | Prove a passed LayerXProposal pauses layerxpay through the app gov router |
| <code>capability.6.28.3</code> | reqs | 6273.1 |
| <code>capability.6.28.4</code> | title | Prove a passed LayerXProposal pauses layerxledger and layerxauth live |
| <code>capability.6.28.4</code> | reqs | 6274.1 |
| <code>capability.6.28.6</code> | title | Check governance pause scope conformance for every routed module |
| <code>capability.6.28.6</code> | reqs | 6276.1 |

## proposal-codec

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-layerxgov-proposal-amino-registry-j35</code> | <code>go_exported</code> | <code>change</code> | <code>modules/layerxgov/types/proposal.go:55</code> | <code>func init() { govtypes.RegisterProposalTypeCodec(...) x13 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-layerxgov-proposal-amino-registry-j35</code> | notes | omits the six layerxbridge messages and layerxpay MsgUpdateParams |

Contract context <code>contract_context.6.layerxgov-routing-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | LayerXProposal (api/layerxgov/proposal.proto:7) submitted through the gov module and executed by NewProposalHandler (modules/layerxgov/handler.go:11) through the app MsgServiceRouter; CheckPauseScope (modules/layerxgov/types/proposal.go:38) consumed by paused modules |
| storage | No store of its own: layerxgov has no keeper and no store key; every effect is a routed message of another module |
| errors | ErrPausedScope 735 LXP_PAUSED_SCOPE (modules/layerxgov/types/proposal.go:26); ErrInvalidProposalContent for a message outside governanceMessage or with a non-gov authority; ErrUnknownRequest for an unrouted message |
| events | The events of the routed messages, re-emitted from the CacheContext after every message succeeds (modules/layerxgov/handler.go:34-39) |
| gas | Gov EndBlocker execution; no layerxgov surcharge |
| activation | Live at HEAD; layerxpay MsgUpdateParams joins the accepted set when domain 2 lands it (anchor 2.0.2) |
| invariants | All messages of one proposal commit together or none does; every accepted message carries the gov module account as authority |
| pause | Module-scope pause only: layerxpay, layerxexchange, layerxledger and layerxauth exit sets in pauseExitPaths (modules/layerxgov/types/proposal.go:28-36); tokenfactory denom pause is issuer-bound and not routed here |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.271.1</code> | req | 6271.1 |
| <code>acceptance.6.271.1</code> | scenario | Given a LayerXProposal built by NewLayerXProposal with a governance-authority layerxbridge MsgPause and MsgSetCap, when a MsgSubmitProposal carrying it is marshalled to legacy amino JSON for sign bytes, then marshalling succeeds and the JSON names layerxbridge/MsgPause and layerxbridge/MsgSetCap |
| <code>acceptance.6.271.1</code> | test | modules/layerxgov/types/amino_bridge_test.go::TestLayerXProposalBridgeAminoJSON (planned) |
| <code>acceptance.6.271.1</code> | qualification | UNRUN |
| <code>req.6271</code> | ac_1 | init (modules/layerxgov/types/proposal.go:55-70) registers bridge MsgRegisterChain, MsgSetAttestors, MsgSetCap, MsgPause, MsgUnpause and MsgRegisterSidioraPair with govtypes.RegisterProposalTypeCodec, which governanceMessage already accepts (modules/layerxgov/types/proposal.go:194-205) but the amino registry omits. |
| <code>capability.6.28.1</code> | title | Register the six layerxbridge governance messages in the LayerX proposal amino codec |
| <code>capability.6.28.1</code> | reqs | 6271.1 |
