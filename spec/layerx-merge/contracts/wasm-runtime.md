# Wasm runtime contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## errors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.5.err-wasmvm-out-of-gas</code> | <code>error_code</code> | <code>stable</code> | <code>wasm-runtime/types/types.go:131</code> | <code>type OutOfGasError struct{}; Error() "Out of gas"</code> |
| <code>interface.5.err-wasmvm-system-error</code> | <code>error_code</code> | <code>stable</code> | <code>wasm-runtime/types/systemerror.go:10</code> | <code>type SystemError struct {invalid_request, invalid_response, no_such_contract, no_such_code, unknown, unsupported_request} (JSON error returned to the VM from Go queriers; exactly one field set)</code> |
| <code>interface.5.err-wasmvm-system-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>wasm-runtime/types/systemerror.go:47</code> | <code>SystemError.invalid_request {error, request} (type InvalidRequest)</code> |
| <code>interface.5.err-wasmvm-system-invalid-response</code> | <code>error_code</code> | <code>stable</code> | <code>wasm-runtime/types/systemerror.go:56</code> | <code>SystemError.invalid_response {error, response} (type InvalidResponse)</code> |
| <code>interface.5.err-wasmvm-system-no-such-code</code> | <code>error_code</code> | <code>stable</code> | <code>wasm-runtime/types/systemerror.go:73</code> | <code>SystemError.no_such_code {code_id} (type NoSuchCode)</code> |
| <code>interface.5.err-wasmvm-system-no-such-contract</code> | <code>error_code</code> | <code>stable</code> | <code>wasm-runtime/types/systemerror.go:65</code> | <code>SystemError.no_such_contract {addr} (type NoSuchContract)</code> |
| <code>interface.5.err-wasmvm-system-unknown</code> | <code>error_code</code> | <code>stable</code> | <code>wasm-runtime/types/systemerror.go:81</code> | <code>SystemError.unknown {} (type Unknown)</code> |
| <code>interface.5.err-wasmvm-system-unsupported-request</code> | <code>error_code</code> | <code>stable</code> | <code>wasm-runtime/types/systemerror.go:87</code> | <code>SystemError.unsupported_request {kind} (type UnsupportedRequest)</code> |
| <code>interface.5.err-wasmvm-to-system-error</code> | <code>error_code</code> | <code>stable</code> | <code>wasm-runtime/types/systemerror.go:104</code> | <code>func ToSystemError(err error) *SystemError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.5.err-wasmvm-out-of-gas</code> | notes | 5 in-repo callers (5 non-test, 0 test). |
| <code>interface.5.err-wasmvm-system-error</code> | notes | 25 in-repo callers (19 non-test, 6 test). |
| <code>interface.5.err-wasmvm-system-invalid-request</code> | notes | 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.5.err-wasmvm-system-invalid-response</code> | notes | 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.err-wasmvm-system-no-such-code</code> | notes | Not listed in the error interface assertion block at wasm-runtime/types/systemerror.go:19-26, although it implements error and ToSystemError converts it. 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.5.err-wasmvm-system-no-such-contract</code> | notes | 6 in-repo callers (2 non-test, 4 test). |
| <code>interface.5.err-wasmvm-system-unknown</code> | notes | 5 in-repo callers (5 non-test, 0 test). |
| <code>interface.5.err-wasmvm-system-unsupported-request</code> | notes | 21 in-repo callers (21 non-test, 0 test). |
| <code>interface.5.err-wasmvm-to-system-error</code> | notes | Returns nil for any error that is not one of the six variants. 2 in-repo callers (2 non-test, 0 test). |

## types

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.5.go-wasmvmtypes-abstain</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:168</code> | <code>Abstain</code> |
| <code>interface.5.go-wasmvmtypes-allbalancesquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:139</code> | <code>type AllBalancesQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-allbalancesresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:144</code> | <code>type AllBalancesResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-alldelegationsquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:323</code> | <code>type AllDelegationsQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-alldelegationsresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:333</code> | <code>type AllDelegationsResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-allvalidatorsquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:270</code> | <code>type AllValidatorsQuery struct{}</code> |
| <code>interface.5.go-wasmvmtypes-allvalidatorsresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:273</code> | <code>type AllValidatorsResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-analysisreport</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/types.go:157</code> | <code>type AnalysisReport struct</code> |
| <code>interface.5.go-wasmvmtypes-balancequery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:129</code> | <code>type BalanceQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-balanceresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:135</code> | <code>type BalanceResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-bankmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:110</code> | <code>type BankMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-bankquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:112</code> | <code>type BankQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-blockinfo</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/env.go:16</code> | <code>type BlockInfo struct</code> |
| <code>interface.5.go-wasmvmtypes-bondeddenomresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:434</code> | <code>type BondedDenomResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-burnmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:125</code> | <code>type BurnMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-channelquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:249</code> | <code>type ChannelQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-channelresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:255</code> | <code>type ChannelResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-checksum</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/checksum.go:11</code> | <code>type Checksum []byte</code> |
| <code>interface.5.go-wasmvmtypes-clearadminmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:366</code> | <code>type ClearAdminMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-closechannelmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:222</code> | <code>type CloseChannelMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-coin</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/types.go:15</code> | <code>type Coin struct</code> |
| <code>interface.5.go-wasmvmtypes-coins</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/types.go:28</code> | <code>type Coins []Coin</code> |
| <code>interface.5.go-wasmvmtypes-contractinfo</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/env.go:24</code> | <code>type ContractInfo struct</code> |
| <code>interface.5.go-wasmvmtypes-contractinfoquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:496</code> | <code>type ContractInfoQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-contractinforesponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:501</code> | <code>type ContractInfoResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-contractresult</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:12</code> | <code>type ContractResult struct</code> |
| <code>interface.5.go-wasmvmtypes-cosmosmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:99</code> | <code>type CosmosMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-delegatemsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:232</code> | <code>type DelegateMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-delegation</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:362</code> | <code>type Delegation struct</code> |
| <code>interface.5.go-wasmvmtypes-delegationquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:327</code> | <code>type DelegationQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-delegationresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:422</code> | <code>type DelegationResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-delegations</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:337</code> | <code>type Delegations []Delegation</code> |
| <code>interface.5.go-wasmvmtypes-distributionmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:248</code> | <code>type DistributionMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-env</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/env.go:10</code> | <code>type Env struct</code> |
| <code>interface.5.go-wasmvmtypes-event</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:60</code> | <code>type Event struct</code> |
| <code>interface.5.go-wasmvmtypes-eventattribute</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:92</code> | <code>type EventAttribute struct</code> |
| <code>interface.5.go-wasmvmtypes-eventattributes</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:66</code> | <code>type EventAttributes []EventAttribute</code> |
| <code>interface.5.go-wasmvmtypes-events</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:35</code> | <code>type Events []Event</code> |
| <code>interface.5.go-wasmvmtypes-executemsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:298</code> | <code>type ExecuteMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-fulldelegation</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:426</code> | <code>type FullDelegation struct</code> |
| <code>interface.5.go-wasmvmtypes-gas</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/gas.go:3</code> | <code>type Gas = uint64</code> |
| <code>interface.5.go-wasmvmtypes-gasmeter</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/gas.go:8</code> | <code>type GasMeter interface</code> |
| <code>interface.5.go-wasmvmtypes-gasreport</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/types.go:139</code> | <code>type GasReport struct</code> |
| <code>interface.5.go-wasmvmtypes-goapi</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/api.go:8</code> | <code>type GoAPI struct</code> |
| <code>interface.5.go-wasmvmtypes-govmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:135</code> | <code>type GovMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-humanaddress</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/types.go:9</code> | <code>type HumanAddress = string</code> |
| <code>interface.5.go-wasmvmtypes-ibc3channelopenresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:209</code> | <code>type IBC3ChannelOpenResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcacknowledgement</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:186</code> | <code>type IBCAcknowledgement struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcbasicresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:227</code> | <code>type IBCBasicResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcbasicresult</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:220</code> | <code>type IBCBasicResult struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcchannel</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:8</code> | <code>type IBCChannel struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcchannelclosemsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:102</code> | <code>type IBCChannelCloseMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcchannelconnectmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:59</code> | <code>type IBCChannelConnectMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcchannelopenmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:16</code> | <code>type IBCChannelOpenMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcchannelopenresult</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:203</code> | <code>type IBCChannelOpenResult struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcchannels</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:198</code> | <code>type IBCChannels []IBCChannel</code> |
| <code>interface.5.go-wasmvmtypes-ibccloseconfirm</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:125</code> | <code>type IBCCloseConfirm struct</code> |
| <code>interface.5.go-wasmvmtypes-ibccloseinit</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:115</code> | <code>type IBCCloseInit struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcendpoint</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:3</code> | <code>type IBCEndpoint struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:129</code> | <code>type IBCMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcopenack</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:81</code> | <code>type IBCOpenAck struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcopenconfirm</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:92</code> | <code>type IBCOpenConfirm struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcopeninit</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:38</code> | <code>type IBCOpenInit struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcopentry</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:48</code> | <code>type IBCOpenTry struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcorder</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:154</code> | <code>type IBCOrder = string</code> |
| <code>interface.5.go-wasmvmtypes-ibcpacket</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:190</code> | <code>type IBCPacket struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcpacketackmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:140</code> | <code>type IBCPacketAckMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcpacketreceivemsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:135</code> | <code>type IBCPacketReceiveMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcpackettimeoutmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:146</code> | <code>type IBCPacketTimeoutMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:172</code> | <code>type IBCQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcreceiveresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:258</code> | <code>type IBCReceiveResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-ibcreceiveresult</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:247</code> | <code>type IBCReceiveResult struct</code> |
| <code>interface.5.go-wasmvmtypes-ibctimeout</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:180</code> | <code>type IBCTimeout struct</code> |
| <code>interface.5.go-wasmvmtypes-ibctimeoutblock</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/ibc.go:166</code> | <code>type IBCTimeoutBlock struct</code> |
| <code>interface.5.go-wasmvmtypes-instantiatemsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:311</code> | <code>type InstantiateMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-iterator</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/store.go:48</code> | <code>type Iterator interface</code> |
| <code>interface.5.go-wasmvmtypes-kvstore</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/store.go:6</code> | <code>type KVStore interface</code> |
| <code>interface.5.go-wasmvmtypes-listchannelsquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:188</code> | <code>type ListChannelsQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-listchannelsresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:193</code> | <code>type ListChannelsResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-messageinfo</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/env.go:38</code> | <code>type MessageInfo struct</code> |
| <code>interface.5.go-wasmvmtypes-metrics</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/types.go:164</code> | <code>type Metrics struct</code> |
| <code>interface.5.go-wasmvmtypes-migratemsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:345</code> | <code>type MigrateMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-newcoin</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/types.go:20</code> | <code>func NewCoin(amount uint64, denom string) Coin</code> |
| <code>interface.5.go-wasmvmtypes-no</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:167</code> | <code>No</code> |
| <code>interface.5.go-wasmvmtypes-nosuchcontract</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/systemerror.go:65</code> | <code>type NoSuchContract struct</code> |
| <code>interface.5.go-wasmvmtypes-nowithveto</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:169</code> | <code>NoWithVeto</code> |
| <code>interface.5.go-wasmvmtypes-outofgaserror</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/types.go:131</code> | <code>type OutOfGasError struct{}</code> |
| <code>interface.5.go-wasmvmtypes-portidquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:178</code> | <code>type PortIDQuery struct{}</code> |
| <code>interface.5.go-wasmvmtypes-portidresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:180</code> | <code>type PortIDResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-querier</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:37</code> | <code>type Querier interface</code> |
| <code>interface.5.go-wasmvmtypes-queryrequest</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:102</code> | <code>type QueryRequest struct</code> |
| <code>interface.5.go-wasmvmtypes-queryresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:11</code> | <code>type QueryResponse queryResponseImpl</code> |
| <code>interface.5.go-wasmvmtypes-rawquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:490</code> | <code>type RawQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-redelegatemsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:242</code> | <code>type RedelegateMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-reply</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/submessages.go:64</code> | <code>type Reply struct</code> |
| <code>interface.5.go-wasmvmtypes-replyalways</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/submessages.go:12</code> | <code>ReplyAlways</code> |
| <code>interface.5.go-wasmvmtypes-replyerror</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/submessages.go:14</code> | <code>ReplyError</code> |
| <code>interface.5.go-wasmvmtypes-replynever</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/submessages.go:15</code> | <code>ReplyNever</code> |
| <code>interface.5.go-wasmvmtypes-replysuccess</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/submessages.go:13</code> | <code>ReplySuccess</code> |
| <code>interface.5.go-wasmvmtypes-response</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:19</code> | <code>type Response struct</code> |
| <code>interface.5.go-wasmvmtypes-rustquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:56</code> | <code>func RustQuery(querier Querier, binRequest []byte, gasLimit uint64) QuerierResult</code> |
| <code>interface.5.go-wasmvmtypes-sendmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:117</code> | <code>type SendMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-sendpacketmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:216</code> | <code>type SendPacketMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-setwithdrawaddressmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:256</code> | <code>type SetWithdrawAddressMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-smartquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:483</code> | <code>type SmartQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-stakingmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:226</code> | <code>type StakingMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-stakingquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:260</code> | <code>type StakingQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-stargatemsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:277</code> | <code>type StargateMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-stargatequery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:467</code> | <code>type StargateQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-submsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/submessages.go:57</code> | <code>type SubMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-submsgresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/submessages.go:79</code> | <code>type SubMsgResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-submsgresult</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/submessages.go:71</code> | <code>type SubMsgResult struct</code> |
| <code>interface.5.go-wasmvmtypes-tosystemerror</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/systemerror.go:104</code> | <code>func ToSystemError(err error) *SystemError</code> |
| <code>interface.5.go-wasmvmtypes-transactioninfo</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/env.go:29</code> | <code>type TransactionInfo struct</code> |
| <code>interface.5.go-wasmvmtypes-transfermsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:209</code> | <code>type TransferMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-ufraction</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/fraction.go:16</code> | <code>type UFraction struct</code> |
| <code>interface.5.go-wasmvmtypes-unbondingdelegationentry</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:456</code> | <code>type UnbondingDelegationEntry struct</code> |
| <code>interface.5.go-wasmvmtypes-unbondingdelegationresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:443</code> | <code>type UnbondingDelegationResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-unbondingdelegationsresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:452</code> | <code>type UnbondingDelegationsResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-undelegatemsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:237</code> | <code>type UndelegateMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-unknown</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/systemerror.go:81</code> | <code>type Unknown struct{}</code> |
| <code>interface.5.go-wasmvmtypes-unsupportedrequest</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/systemerror.go:87</code> | <code>type UnsupportedRequest struct</code> |
| <code>interface.5.go-wasmvmtypes-updateadminmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:357</code> | <code>type UpdateAdminMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-validator</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:313</code> | <code>type Validator struct</code> |
| <code>interface.5.go-wasmvmtypes-validatorquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:303</code> | <code>type ValidatorQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-validatorresponse</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:309</code> | <code>type ValidatorResponse struct</code> |
| <code>interface.5.go-wasmvmtypes-votemsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:144</code> | <code>type VoteMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-wasmmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:282</code> | <code>type WasmMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-wasmquery</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:475</code> | <code>type WasmQuery struct</code> |
| <code>interface.5.go-wasmvmtypes-withdrawdelegatorrewardmsg</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:263</code> | <code>type WithdrawDelegatorRewardMsg struct</code> |
| <code>interface.5.go-wasmvmtypes-yes</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:166</code> | <code>Yes</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.5.go-wasmvmtypes-abstain</code> | notes | Exported const used outside its tree; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-allbalancesquery</code> | notes | Exported type used outside its tree; 5 in-repo callers (4 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-allbalancesresponse</code> | notes | Exported type used outside its tree; 10 in-repo callers (7 non-test, 3 test). |
| <code>interface.5.go-wasmvmtypes-alldelegationsquery</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-alldelegationsresponse</code> | notes | Exported type used outside its tree; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-allvalidatorsquery</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-allvalidatorsresponse</code> | notes | Exported type used outside its tree; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-analysisreport</code> | notes | Exported type used outside its tree; 13 in-repo callers (7 non-test, 6 test). |
| <code>interface.5.go-wasmvmtypes-balancequery</code> | notes | Exported type used outside its tree; 8 in-repo callers (6 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-balanceresponse</code> | notes | Exported type used outside its tree; 13 in-repo callers (9 non-test, 4 test). |
| <code>interface.5.go-wasmvmtypes-bankmsg</code> | notes | Exported type used outside its tree; 34 in-repo callers (3 non-test, 31 test). |
| <code>interface.5.go-wasmvmtypes-bankquery</code> | notes | Exported type used outside its tree; 19 in-repo callers (15 non-test, 4 test). |
| <code>interface.5.go-wasmvmtypes-blockinfo</code> | notes | Exported type used outside its tree; 5 in-repo callers (3 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-bondeddenomresponse</code> | notes | Exported type used outside its tree; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-burnmsg</code> | notes | Exported type used outside its tree; 5 in-repo callers (0 non-test, 5 test). |
| <code>interface.5.go-wasmvmtypes-channelquery</code> | notes | Exported type used outside its tree; 5 in-repo callers (0 non-test, 5 test). |
| <code>interface.5.go-wasmvmtypes-channelresponse</code> | notes | Exported type used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-checksum</code> | notes | Exported type used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-clearadminmsg</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-closechannelmsg</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-coin</code> | notes | Exported type used outside its tree; 39 in-repo callers (16 non-test, 23 test). |
| <code>interface.5.go-wasmvmtypes-coins</code> | notes | Exported type used outside its tree; 58 in-repo callers (16 non-test, 42 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-contractinfo</code> | notes | Exported type used outside its tree; 5 in-repo callers (3 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-contractinfoquery</code> | notes | Exported type used outside its tree; 5 in-repo callers (0 non-test, 5 test). |
| <code>interface.5.go-wasmvmtypes-contractinforesponse</code> | notes | Exported type used outside its tree; 6 in-repo callers (1 non-test, 5 test). |
| <code>interface.5.go-wasmvmtypes-contractresult</code> | notes | Exported type used outside its tree; 26 in-repo callers (10 non-test, 16 test). |
| <code>interface.5.go-wasmvmtypes-cosmosmsg</code> | notes | Exported type used outside its tree; 118 in-repo callers (11 non-test, 107 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-delegatemsg</code> | notes | Exported type used outside its tree; 2 in-repo callers (0 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-delegation</code> | notes | Exported type used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-delegationquery</code> | notes | Exported type used outside its tree; 2 in-repo callers (0 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-delegationresponse</code> | notes | Exported type used outside its tree; 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-delegations</code> | notes | Exported type used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-distributionmsg</code> | notes | Exported type used outside its tree; 5 in-repo callers (3 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-env</code> | notes | Exported type used outside its tree; 124 in-repo callers (47 non-test, 77 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-event</code> | notes | Exported type used outside its tree; 13 in-repo callers (3 non-test, 10 test). |
| <code>interface.5.go-wasmvmtypes-eventattribute</code> | notes | Exported type used outside its tree; 61 in-repo callers (10 non-test, 51 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-eventattributes</code> | notes | Exported type used outside its tree; 2 in-repo callers (0 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-events</code> | notes | Exported type used outside its tree; 23 in-repo callers (4 non-test, 19 test). |
| <code>interface.5.go-wasmvmtypes-executemsg</code> | notes | Exported type used outside its tree; 5 in-repo callers (0 non-test, 5 test). |
| <code>interface.5.go-wasmvmtypes-fulldelegation</code> | notes | Exported type used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-gas</code> | notes | Exported type used outside its tree; 14 in-repo callers (14 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-gasmeter</code> | notes | Exported type used outside its tree; 116 in-repo callers (42 non-test, 74 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-gasreport</code> | notes | Exported type used outside its tree; 30 in-repo callers (30 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-goapi</code> | notes | Exported type used outside its tree; 48 in-repo callers (44 non-test, 4 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-govmsg</code> | notes | Exported type used outside its tree; 6 in-repo callers (2 non-test, 4 test). |
| <code>interface.5.go-wasmvmtypes-humanaddress</code> | notes | Exported type used outside its tree; 8 in-repo callers (6 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-ibc3channelopenresponse</code> | notes | Exported type used outside its tree; 13 in-repo callers (3 non-test, 10 test). |
| <code>interface.5.go-wasmvmtypes-ibcacknowledgement</code> | notes | Exported type used outside its tree; 4 in-repo callers (3 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-ibcbasicresponse</code> | notes | Exported type used outside its tree; 72 in-repo callers (13 non-test, 59 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-ibcbasicresult</code> | notes | Exported type used outside its tree; 8 in-repo callers (8 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-ibcchannel</code> | notes | Exported type used outside its tree; 15 in-repo callers (11 non-test, 4 test). |
| <code>interface.5.go-wasmvmtypes-ibcchannelclosemsg</code> | notes | Exported type used outside its tree; 24 in-repo callers (15 non-test, 9 test). |
| <code>interface.5.go-wasmvmtypes-ibcchannelconnectmsg</code> | notes | Exported type used outside its tree; 23 in-repo callers (15 non-test, 8 test). |
| <code>interface.5.go-wasmvmtypes-ibcchannelopenmsg</code> | notes | Exported type used outside its tree; 23 in-repo callers (15 non-test, 8 test). |
| <code>interface.5.go-wasmvmtypes-ibcchannelopenresult</code> | notes | Exported type used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-ibcchannels</code> | notes | Exported type used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-ibccloseconfirm</code> | notes | Exported type used outside its tree; 3 in-repo callers (3 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-ibccloseinit</code> | notes | Exported type used outside its tree; 5 in-repo callers (3 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-ibcendpoint</code> | notes | Exported type used outside its tree; 28 in-repo callers (20 non-test, 8 test). |
| <code>interface.5.go-wasmvmtypes-ibcmsg</code> | notes | Exported type used outside its tree; 13 in-repo callers (4 non-test, 9 test). |
| <code>interface.5.go-wasmvmtypes-ibcopenack</code> | notes | Exported type used outside its tree; 3 in-repo callers (3 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-ibcopenconfirm</code> | notes | Exported type used outside its tree; 5 in-repo callers (3 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-ibcopeninit</code> | notes | Exported type used outside its tree; 3 in-repo callers (3 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-ibcopentry</code> | notes | Exported type used outside its tree; 5 in-repo callers (3 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-ibcorder</code> | notes | Exported type used outside its tree; 14 in-repo callers (14 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-ibcpacket</code> | notes | Exported type used outside its tree; 13 in-repo callers (6 non-test, 7 test). |
| <code>interface.5.go-wasmvmtypes-ibcpacketackmsg</code> | notes | Exported type used outside its tree; 18 in-repo callers (10 non-test, 8 test). |
| <code>interface.5.go-wasmvmtypes-ibcpacketreceivemsg</code> | notes | Exported type used outside its tree; 20 in-repo callers (10 non-test, 10 test). |
| <code>interface.5.go-wasmvmtypes-ibcpackettimeoutmsg</code> | notes | Exported type used outside its tree; 18 in-repo callers (10 non-test, 8 test). |
| <code>interface.5.go-wasmvmtypes-ibcquery</code> | notes | Exported type used outside its tree; 14 in-repo callers (3 non-test, 11 test). |
| <code>interface.5.go-wasmvmtypes-ibcreceiveresponse</code> | notes | Exported type used outside its tree; 13 in-repo callers (0 non-test, 13 test). |
| <code>interface.5.go-wasmvmtypes-ibcreceiveresult</code> | notes | Exported type used outside its tree; 20 in-repo callers (5 non-test, 15 test). |
| <code>interface.5.go-wasmvmtypes-ibctimeout</code> | notes | Exported type used outside its tree; 16 in-repo callers (3 non-test, 13 test). |
| <code>interface.5.go-wasmvmtypes-ibctimeoutblock</code> | notes | Exported type used outside its tree; 14 in-repo callers (4 non-test, 10 test). |
| <code>interface.5.go-wasmvmtypes-instantiatemsg</code> | notes | Exported type used outside its tree; 2 in-repo callers (0 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-iterator</code> | notes | Exported type used outside its tree; 30 in-repo callers (24 non-test, 6 test). |
| <code>interface.5.go-wasmvmtypes-kvstore</code> | notes | Exported type used outside its tree; 44 in-repo callers (40 non-test, 4 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-listchannelsquery</code> | notes | Exported type used outside its tree; 3 in-repo callers (0 non-test, 3 test). |
| <code>interface.5.go-wasmvmtypes-listchannelsresponse</code> | notes | Exported type used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-messageinfo</code> | notes | Exported type used outside its tree; 103 in-repo callers (43 non-test, 60 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-metrics</code> | notes | Exported type used outside its tree; 14 in-repo callers (8 non-test, 6 test). |
| <code>interface.5.go-wasmvmtypes-migratemsg</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-newcoin</code> | notes | Exported func used outside its tree; 61 in-repo callers (13 non-test, 48 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-no</code> | notes | Exported const used outside its tree; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-nosuchcontract</code> | notes | Exported type used outside its tree; 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-nowithveto</code> | notes | Exported const used outside its tree; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-outofgaserror</code> | notes | Exported type used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-portidquery</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-portidresponse</code> | notes | Exported type used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-querier</code> | notes | Exported type used outside its tree; 11 in-repo callers (9 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-queryrequest</code> | notes | Exported type used outside its tree; 40 in-repo callers (17 non-test, 23 test). |
| <code>interface.5.go-wasmvmtypes-queryresponse</code> | notes | Exported type used outside its tree; 28 in-repo callers (2 non-test, 26 test). |
| <code>interface.5.go-wasmvmtypes-rawquery</code> | notes | Exported type used outside its tree; 2 in-repo callers (0 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-redelegatemsg</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-reply</code> | notes | Exported type used outside its tree; 54 in-repo callers (9 non-test, 45 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-replyalways</code> | notes | Exported const used outside its tree; 8 in-repo callers (1 non-test, 7 test). |
| <code>interface.5.go-wasmvmtypes-replyerror</code> | notes | Exported const used outside its tree; 10 in-repo callers (2 non-test, 8 test). |
| <code>interface.5.go-wasmvmtypes-replynever</code> | notes | Exported const used outside its tree; 31 in-repo callers (3 non-test, 28 test). |
| <code>interface.5.go-wasmvmtypes-replysuccess</code> | notes | Exported const used outside its tree; 7 in-repo callers (2 non-test, 5 test). |
| <code>interface.5.go-wasmvmtypes-response</code> | notes | Exported type used outside its tree; 64 in-repo callers (20 non-test, 44 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-rustquery</code> | notes | Exported func used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-sendmsg</code> | notes | Exported type used outside its tree; 13 in-repo callers (0 non-test, 13 test). |
| <code>interface.5.go-wasmvmtypes-sendpacketmsg</code> | notes | Exported type used outside its tree; 7 in-repo callers (0 non-test, 7 test). |
| <code>interface.5.go-wasmvmtypes-setwithdrawaddressmsg</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-smartquery</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-stakingmsg</code> | notes | Exported type used outside its tree; 7 in-repo callers (3 non-test, 4 test). |
| <code>interface.5.go-wasmvmtypes-stakingquery</code> | notes | Exported type used outside its tree; 11 in-repo callers (3 non-test, 8 test). |
| <code>interface.5.go-wasmvmtypes-stargatemsg</code> | notes | Exported type used outside its tree; 6 in-repo callers (3 non-test, 3 test). |
| <code>interface.5.go-wasmvmtypes-stargatequery</code> | notes | Exported type used outside its tree; 7 in-repo callers (3 non-test, 4 test). |
| <code>interface.5.go-wasmvmtypes-submsg</code> | notes | Exported type used outside its tree; 64 in-repo callers (5 non-test, 59 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-submsgresponse</code> | notes | Exported type used outside its tree; 11 in-repo callers (1 non-test, 10 test). |
| <code>interface.5.go-wasmvmtypes-submsgresult</code> | notes | Exported type used outside its tree; 20 in-repo callers (3 non-test, 17 test). |
| <code>interface.5.go-wasmvmtypes-tosystemerror</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-transactioninfo</code> | notes | Exported type used outside its tree; 4 in-repo callers (3 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-transfermsg</code> | notes | Exported type used outside its tree; 4 in-repo callers (0 non-test, 4 test). |
| <code>interface.5.go-wasmvmtypes-ufraction</code> | notes | Exported type used outside its tree; 118 in-repo callers (44 non-test, 74 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvmtypes-unbondingdelegationentry</code> | notes | Exported type used outside its tree; 3 in-repo callers (3 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-unbondingdelegationresponse</code> | notes | Exported type used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-unbondingdelegationsresponse</code> | notes | Exported type used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-undelegatemsg</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-unknown</code> | notes | Exported type used outside its tree; 3 in-repo callers (3 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-unsupportedrequest</code> | notes | Exported type used outside its tree; 16 in-repo callers (16 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-updateadminmsg</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-validator</code> | notes | Exported type used outside its tree; 3 in-repo callers (3 non-test, 0 test). |
| <code>interface.5.go-wasmvmtypes-validatorquery</code> | notes | Exported type used outside its tree; 2 in-repo callers (0 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-validatorresponse</code> | notes | Exported type used outside its tree; 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.5.go-wasmvmtypes-votemsg</code> | notes | Exported type used outside its tree; 4 in-repo callers (0 non-test, 4 test). |
| <code>interface.5.go-wasmvmtypes-wasmmsg</code> | notes | Exported type used outside its tree; 13 in-repo callers (3 non-test, 10 test). |
| <code>interface.5.go-wasmvmtypes-wasmquery</code> | notes | Exported type used outside its tree; 13 in-repo callers (3 non-test, 10 test). |
| <code>interface.5.go-wasmvmtypes-withdrawdelegatorrewardmsg</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvmtypes-yes</code> | notes | Exported const used outside its tree; 2 in-repo callers (1 non-test, 1 test). |

## vm-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.5.go-wasmvm-checksum</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib_no_cgo.go:14</code> | <code>type Checksum = types.Checksum</code> |
| <code>interface.5.go-wasmvm-deserialize-response</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:565</code> | <code>func DeserializeResponse(gasLimit uint64, deserCost types.UFraction, gasReport *types.GasReport, data []byte, response any) error</code> |
| <code>interface.5.go-wasmvm-gasmeter</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib_no_cgo.go:29</code> | <code>type GasMeter = types.GasMeter</code> |
| <code>interface.5.go-wasmvm-goapi</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib_no_cgo.go:23</code> | <code>type GoAPI = types.GoAPI</code> |
| <code>interface.5.go-wasmvm-kvstore</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib_no_cgo.go:20</code> | <code>type KVStore = types.KVStore</code> |
| <code>interface.5.go-wasmvm-libwasmvmversion</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib_no_cgo.go:36</code> | <code>func LibwasmvmVersion() (string, error)</code> |
| <code>interface.5.go-wasmvm-newvm</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:31</code> | <code>func NewVM(dataDir string, supportedCapabilities string, memoryLimit uint32, printDebug bool, cacheSize uint32) (*VM, error)</code> |
| <code>interface.5.go-wasmvm-querier</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib_no_cgo.go:26</code> | <code>type Querier = types.Querier</code> |
| <code>interface.5.go-wasmvm-vm</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:18</code> | <code>type VM struct (cgo build only; wraps the Rust VM cache); never named outside the package, constructed by NewVM and held as the WasmerEngine interface</code> |
| <code>interface.5.go-wasmvm-vm-analyzecode</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:102</code> | <code>func (vm *VM) AnalyzeCode(checksum Checksum) (*types.AnalysisReport, error)</code> |
| <code>interface.5.go-wasmvm-vm-cleanup</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:40</code> | <code>func (vm *VM) Cleanup()</code> |
| <code>interface.5.go-wasmvm-vm-create</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:45</code> | <code>func (vm *VM) Create(code WasmCode) (Checksum, error)</code> |
| <code>interface.5.go-wasmvm-vm-execute</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:161</code> | <code>func (vm *VM) Execute( checksum Checksum, env types.Env, info types.MessageInfo, executeMsg []byte, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.Response, uint64, error)</code> |
| <code>interface.5.go-wasmvm-vm-getcode</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:80</code> | <code>func (vm *VM) GetCode(checksum Checksum) (WasmCode, error)</code> |
| <code>interface.5.go-wasmvm-vm-getmetrics</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:107</code> | <code>func (vm *VM) GetMetrics() (*types.Metrics, error)</code> |
| <code>interface.5.go-wasmvm-vm-ibcchannelclose</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:420</code> | <code>func (vm *VM) IBCChannelClose( checksum Checksum, env types.Env, msg types.IBCChannelCloseMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResponse, uint64, error)</code> |
| <code>interface.5.go-wasmvm-vm-ibcchannelconnect</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:383</code> | <code>func (vm *VM) IBCChannelConnect( checksum Checksum, env types.Env, msg types.IBCChannelConnectMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResponse, uint64, error)</code> |
| <code>interface.5.go-wasmvm-vm-ibcchannelopen</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:346</code> | <code>func (vm *VM) IBCChannelOpen( checksum Checksum, env types.Env, msg types.IBCChannelOpenMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBC3ChannelOpenResponse, uint64, error)</code> |
| <code>interface.5.go-wasmvm-vm-ibcpacketack</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:492</code> | <code>func (vm *VM) IBCPacketAck( checksum Checksum, env types.Env, msg types.IBCPacketAckMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResponse, uint64, error)</code> |
| <code>interface.5.go-wasmvm-vm-ibcpacketreceive</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:457</code> | <code>func (vm *VM) IBCPacketReceive( checksum Checksum, env types.Env, msg types.IBCPacketReceiveMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCReceiveResult, uint64, error)</code> |
| <code>interface.5.go-wasmvm-vm-ibcpackettimeout</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:530</code> | <code>func (vm *VM) IBCPacketTimeout( checksum Checksum, env types.Env, msg types.IBCPacketTimeoutMsg, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.IBCBasicResponse, uint64, error)</code> |
| <code>interface.5.go-wasmvm-vm-instantiate</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:119</code> | <code>func (vm *VM) Instantiate( checksum Checksum, env types.Env, info types.MessageInfo, initMsg []byte, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.Response, uint64, error)</code> |
| <code>interface.5.go-wasmvm-vm-migrate</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:237</code> | <code>func (vm *VM) Migrate( checksum Checksum, env types.Env, migrateMsg []byte, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.Response, uint64, error)</code> |
| <code>interface.5.go-wasmvm-vm-pin</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:87</code> | <code>func (vm *VM) Pin(checksum Checksum) error</code> |
| <code>interface.5.go-wasmvm-vm-query</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:200</code> | <code>func (vm *VM) Query( checksum Checksum, env types.Env, queryMsg []byte, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) ([]byte, uint64, error)</code> |
| <code>interface.5.go-wasmvm-vm-removecode</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:69</code> | <code>func (vm *VM) RemoveCode(checksum Checksum) error</code> |
| <code>interface.5.go-wasmvm-vm-reply</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:309</code> | <code>func (vm *VM) Reply( checksum Checksum, env types.Env, reply types.Reply, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.Response, uint64, error)</code> |
| <code>interface.5.go-wasmvm-vm-storecode</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:59</code> | <code>func (vm *VM) StoreCode(code WasmCode) (Checksum, error)</code> |
| <code>interface.5.go-wasmvm-vm-storecodeunchecked</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:65</code> | <code>func (vm *VM) StoreCodeUnchecked(code WasmCode) (Checksum, error)</code> |
| <code>interface.5.go-wasmvm-vm-sudo</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:274</code> | <code>func (vm *VM) Sudo( checksum Checksum, env types.Env, sudoMsg []byte, store KVStore, goapi GoAPI, querier Querier, gasMeter GasMeter, gasLimit uint64, deserCost types.UFraction) (*types.Response, uint64, error)</code> |
| <code>interface.5.go-wasmvm-vm-unpin</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib.go:95</code> | <code>func (vm *VM) Unpin(checksum Checksum) error</code> |
| <code>interface.5.go-wasmvm-wasmcode</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm-runtime/lib_no_cgo.go:17</code> | <code>type WasmCode []byte</code> |
| <code>interface.6.go-wasm-artifacts-v152-new-vm</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/artifacts/v152/lib.go:33</code> | <code>func NewVM(dataDir string, supportedCapabilities string, memoryLimit uint32, printDebug bool, cacheSize uint32) (*VM, error)</code> |
| <code>interface.6.go-wasm-artifacts-v152-vm</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/artifacts/v152/lib.go:20</code> | <code>type VM struct; same method set as wasm-runtime/lib.go VM (Create through IBCPacketTimeout) taking wasm-runtime types</code> |
| <code>interface.6.go-wasm-artifacts-v155-new-vm</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/artifacts/v155/lib.go:33</code> | <code>func NewVM(dataDir string, supportedCapabilities string, memoryLimit uint32, printDebug bool, cacheSize uint32) (*VM, error)</code> |
| <code>interface.6.go-wasm-artifacts-v155-vm</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/artifacts/v155/lib.go:20</code> | <code>type VM struct; same method set as wasm-runtime/lib.go VM (Create through IBCPacketTimeout) taking wasm-runtime types</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.5.go-wasmvm-checksum</code> | notes | Defined in a file with no build tag, so it is compiled with and without cgo. Exported type used outside its tree; 147 in-repo callers (36 non-test, 111 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvm-deserialize-response</code> | notes | Charges deserialisation gas before JSON decoding a VM response. 26 in-repo callers (26 non-test, 0 test). |
| <code>interface.5.go-wasmvm-gasmeter</code> | notes | Defined in a file with no build tag, so it is compiled with and without cgo. Exported type used outside its tree; 93 in-repo callers (19 non-test, 74 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvm-goapi</code> | notes | Defined in a file with no build tag, so it is compiled with and without cgo. Exported type used outside its tree; 93 in-repo callers (19 non-test, 74 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvm-kvstore</code> | notes | Defined in a file with no build tag, so it is compiled with and without cgo. Exported type used outside its tree; 96 in-repo callers (20 non-test, 76 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvm-libwasmvmversion</code> | notes | Defined in a file with no build tag, so it is compiled with and without cgo. Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.go-wasmvm-newvm</code> | notes | Exported func used outside its tree; 3 in-repo callers (3 non-test, 0 test). |
| <code>interface.5.go-wasmvm-querier</code> | notes | Defined in a file with no build tag, so it is compiled with and without cgo. Exported type used outside its tree; 92 in-repo callers (18 non-test, 74 test), first 40 listed with non-test callers first. |
| <code>interface.5.go-wasmvm-vm</code> | notes | Constructed three times by the wasm keeper (consensus, simulation and rpc VMs at wasm/x/wasm/keeper/keeper.go:124, 128 and 132, sharing one data dir); getWasmer (wasm/x/wasm/keeper/keeper.go:180) swaps in the v152 and v155 artifact VMs for traced pacific-1 calls below heights 102491599 and 139936278. 4 in-repo callers (4 non-test, 0 test). |
| <code>interface.5.go-wasmvm-vm-analyzecode</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 5 in-repo callers (3 non-test, 2 test). |
| <code>interface.5.go-wasmvm-vm-cleanup</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.5.go-wasmvm-vm-create</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 4 in-repo callers (4 non-test, 0 test). |
| <code>interface.5.go-wasmvm-vm-execute</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.5.go-wasmvm-vm-getcode</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.5.go-wasmvm-vm-getmetrics</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 8 in-repo callers (0 non-test, 8 test). |
| <code>interface.5.go-wasmvm-vm-ibcchannelclose</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.go-wasmvm-vm-ibcchannelconnect</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.5.go-wasmvm-vm-ibcchannelopen</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.5.go-wasmvm-vm-ibcpacketack</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.go-wasmvm-vm-ibcpacketreceive</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.5.go-wasmvm-vm-ibcpackettimeout</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.go-wasmvm-vm-instantiate</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 10 in-repo callers (2 non-test, 8 test). |
| <code>interface.5.go-wasmvm-vm-migrate</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.5.go-wasmvm-vm-pin</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 4 in-repo callers (3 non-test, 1 test). |
| <code>interface.5.go-wasmvm-vm-query</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.5.go-wasmvm-vm-removecode</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 2 in-repo callers (0 non-test, 2 test). |
| <code>interface.5.go-wasmvm-vm-reply</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 3 in-repo callers (2 non-test, 1 test). |
| <code>interface.5.go-wasmvm-vm-storecode</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 14 in-repo callers (2 non-test, 12 test). |
| <code>interface.5.go-wasmvm-vm-storecodeunchecked</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.5.go-wasmvm-vm-sudo</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.5.go-wasmvm-vm-unpin</code> | notes | Reached through the WasmerEngine interface of the wasm keeper. 3 in-repo callers (2 non-test, 1 test). |
| <code>interface.5.go-wasmvm-wasmcode</code> | notes | Defined in a file with no build tag, so it is compiled with and without cgo. Exported type used outside its tree; 18 in-repo callers (11 non-test, 7 test). |
| <code>interface.6.go-wasm-artifacts-v152-new-vm</code> | notes | Frozen VM build kept for replaying historical traced calls; used only by getWasmer when tracing pacific-1 below its cut-over height. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-artifacts-v152-vm</code> | notes | Methods are reached through the WasmerEngine interface held as rpcWasmVM152. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-artifacts-v155-new-vm</code> | notes | Frozen VM build kept for replaying historical traced calls; used only by getWasmer when tracing pacific-1 below its cut-over height. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-artifacts-v155-vm</code> | notes | Methods are reached through the WasmerEngine interface held as rpcWasmVM155. 1 in-repo callers (1 non-test, 0 test). |
