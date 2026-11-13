# LayerX governance contract

The [governance module](../../../modules/layerxgov) validates `LayerXProposal` messages and executes them through the message service router in one cached context. Message writes and events commit together only after all messages succeed.

## go-api

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.6.go-layerxgov-client-cli-decodelayerxproposal-j35</code> | <code>go_exported</code> | [modules/layerxgov/client/cli/tx.go](../../../modules/layerxgov/client/cli/tx.go) | <code>func DecodeLayerXProposal(cdc codec.JSONCodec, body []byte) (*types.LayerXProposal, error)</code> |
| <code>interface.6.go-layerxgov-client-cli-layerxproposalrequest-j35</code> | <code>go_exported</code> | [modules/layerxgov/client/cli/tx.go](../../../modules/layerxgov/client/cli/tx.go) | <code>type LayerXProposalRequest struct</code> |
| <code>interface.6.go-layerxgov-client-cli-layerxproposalresthandler-j35</code> | <code>go_exported</code> | [modules/layerxgov/client/cli/tx.go](../../../modules/layerxgov/client/cli/tx.go) | <code>func LayerXProposalRESTHandler(clientCtx client.Context) govrest.ProposalRESTHandler</code> |
| <code>interface.6.go-layerxgov-client-cli-newsubmitlayerxproposalcmd-j35</code> | <code>go_exported</code> | [modules/layerxgov/client/cli/tx.go](../../../modules/layerxgov/client/cli/tx.go) | <code>func NewSubmitLayerXProposalCmd() *cobra.Command</code> |
| <code>interface.6.go-layerxgov-client-cli-newsubmitlayerxproposalmsg-j35</code> | <code>go_exported</code> | [modules/layerxgov/client/cli/tx.go](../../../modules/layerxgov/client/cli/tx.go) | <code>func NewSubmitLayerXProposalMsg(cdc codec.JSONCodec, body []byte, depositInput string, proposer sdk.AccAddress) (*govtypes.MsgSubmitProposal, error)</code> |
| <code>interface.6.go-layerxgov-module-newproposalhandler-j35</code> | <code>go_exported</code> | [modules/layerxgov/handler.go](../../../modules/layerxgov/handler.go) | <code>func NewProposalHandler(router *baseapp.MsgServiceRouter) govtypes.Handler</code> |
| <code>interface.6.go-layerxgov-types-checkpausescope-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func CheckPauseScope(module string, paused bool, method string) error</code> |
| <code>interface.6.go-layerxgov-types-governanceauthority-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func GovernanceAuthority() string { return authtypes.NewModuleAddress(govtypes.ModuleName).String() }</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-getdescription-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func (p *LayerXProposal) GetDescription() string { return p.Description }</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-getmessages-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func (p *LayerXProposal) GetMessages() ([]sdk.Msg, error)</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-gettitle-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func (p *LayerXProposal) GetTitle() string       { return p.Title }</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-proposalroute-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func (p *LayerXProposal) ProposalRoute() string  { return RouterKey }</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-proposaltype-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func (p *LayerXProposal) ProposalType() string   { return ProposalTypeLayerX }</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-string-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func (p LayerXProposal) String() string</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-unpackinterfaces-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func (p LayerXProposal) UnpackInterfaces(unpacker cdctypes.AnyUnpacker) error</code> |
| <code>interface.6.go-layerxgov-types-layerxproposal-validatebasic-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func (p *LayerXProposal) ValidateBasic() error</code> |
| <code>interface.6.go-layerxgov-types-registercodec-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func RegisterCodec(cdc *codec.LegacyAmino)</code> |
| <code>interface.6.go-layerxgov-types-registerinterfaces-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func RegisterInterfaces(registry cdctypes.InterfaceRegistry)</code> |

## pause-routing

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.6.go-layerxgov-newlayerxproposal-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func NewLayerXProposal(title, description string, msgs ...sdk.Msg) (*LayerXProposal, error)</code> |

| Field | Rule |
|---|---|
| interface | LayerXProposal (api/layerxgov/proposal.proto) submitted through the gov module and executed by NewProposalHandler (modules/layerxgov/handler.go) through the app MsgServiceRouter; CheckPauseScope (modules/layerxgov/types/proposal.go) consumed by paused modules |
| storage | No store of its own: layerxgov has no keeper and no store key; every effect is a routed message of another module |
| errors | ErrPausedScope 735 LXP_PAUSED_SCOPE (modules/layerxgov/types/proposal.go); ErrInvalidProposalContent for a message outside governanceMessage or with a non-gov authority; ErrUnknownRequest for an unrouted message |
| events | The events of the routed messages, re-emitted from the CacheContext after every message succeeds (modules/layerxgov/handler.go) |
| gas | Gov EndBlocker execution; no layerxgov surcharge |
| invariants | All messages of one proposal commit together or none does; every accepted message carries the gov module account as authority |
| pause | Module-scope pause covers layerxpay, layerxexchange, layerxledger, layerxauth and layerxagent. Each module retains its declared exit methods while paused; tokenfactory denomination pause is issuer-bound. |

## proposal-codec

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.6.go-layerxgov-proposal-amino-registry-j35</code> | <code>go_exported</code> | [modules/layerxgov/types/proposal.go](../../../modules/layerxgov/types/proposal.go) | <code>func init()</code> |

| Field | Rule |
|---|---|
| interface | LayerXProposal (api/layerxgov/proposal.proto) submitted through the gov module and executed by NewProposalHandler (modules/layerxgov/handler.go) through the app MsgServiceRouter; CheckPauseScope (modules/layerxgov/types/proposal.go) consumed by paused modules |
| storage | No store of its own: layerxgov has no keeper and no store key; every effect is a routed message of another module |
| errors | ErrPausedScope 735 LXP_PAUSED_SCOPE (modules/layerxgov/types/proposal.go); ErrInvalidProposalContent for a message outside governanceMessage or with a non-gov authority; ErrUnknownRequest for an unrouted message |
| events | The events of the routed messages, re-emitted from the CacheContext after every message succeeds (modules/layerxgov/handler.go) |
| gas | Gov EndBlocker execution; no layerxgov surcharge |
| invariants | All messages of one proposal commit together or none does; every accepted message carries the gov module account as authority |
| pause | Module-scope pause covers layerxpay, layerxexchange, layerxledger, layerxauth and layerxagent. Each module retains its declared exit methods while paused; tokenfactory denomination pause is issuer-bound. |
