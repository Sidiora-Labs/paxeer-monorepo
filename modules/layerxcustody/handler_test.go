package layerxcustody_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/testvectors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const sidPointer = "0x21f7b20a555199fa73A238B1a91FD0f549068fEe"

// custodyApp is a chain that has not run the v6.12.0 plan, so each test
// activates custody governance itself through the real upgrade keeper; the
// shared EVM test app already records the plan done at height 1.
var custodyApp = paxapp.SetupWithDefaultHome(false, true, false)

func assetID(symbol string) string {
	sum := sha256.Sum256([]byte("layerx-asset:125:" + symbol))
	return hex.EncodeToString(sum[:])
}

func depositRootAuthority() string {
	seed := sha256.Sum256([]byte("custody proposal test"))
	key := ed25519.NewKeyFromSeed(seed[:])
	return hex.EncodeToString(key.Public().(ed25519.PublicKey))
}

func newCtx() sdk.Context {
	return activate(inactiveCtx(), 7)
}

func inactiveCtx() sdk.Context {
	ctx, _ := custodyApp.NewContext(false, tmtypes.Header{}).WithBlockHeight(7).CacheContext()
	return ctx
}

// activate records the custody governance activation plan done at height
// through the real upgrade keeper, the record the custody gate reads, and
// returns ctx at that height. The full v6.12.0 handler needs the seeded
// release state, so node/gov_custody_test.go applies it end to end.
func activate(ctx sdk.Context, height int64) sdk.Context {
	ctx = ctx.WithBlockHeight(height)
	custodyApp.UpgradeKeeper.SetDone(ctx, types.GovernanceActivationUpgrade)
	return ctx
}

func sidAsset() types.AssetMapping {
	return types.AssetMapping{AssetId: assetID("SID"), Denom: "usid", Pointer: sidPointer, Enabled: true, MinimumDeposit: "1"}
}

func TestCustodyProposalSetsAssetAndDepositRootAuthority(t *testing.T) {
	ctx := newCtx()
	k := custodyApp.LayerXCustodyKeeper
	governance := types.GovernanceAuthority()
	params := k.GetParams(ctx)
	params.DepositRootAuthority = depositRootAuthority()
	proposal, err := types.NewCustodyProposal("Custody assets", "Map SID and set the deposit root authority",
		&types.MsgSetAsset{Authority: governance, Asset: sidAsset()},
		&types.MsgUpdateParams{Authority: governance, Params: params})
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())

	require.NoError(t, layerxcustody.NewProposalHandler(k)(ctx, proposal))

	id, err := types.ParseHash32(assetID("SID"))
	require.NoError(t, err)
	got, found := k.GetAsset(ctx, id)
	require.True(t, found)
	require.Equal(t, "usid", got.Denom)
	byPointer, found := k.GetAssetByPointer(ctx, mustAddress(t, sidPointer))
	require.True(t, found)
	require.Equal(t, got.AssetId, byPointer.AssetId)
	require.Equal(t, depositRootAuthority(), k.GetParams(ctx).DepositRootAuthority)
}

func mustAddress(t *testing.T, text string) common.Address {
	t.Helper()
	address, err := types.ParseAddress(text)
	require.NoError(t, err)
	return address
}

func TestCustodyProposalRefusals(t *testing.T) {
	ctx := newCtx()
	k := custodyApp.LayerXCustodyKeeper
	handler := layerxcustody.NewProposalHandler(k)
	governance := types.GovernanceAuthority()

	empty := &types.CustodyProposal{Title: "Empty", Description: "No message"}
	require.ErrorIs(t, empty.ValidateBasic(), govtypes.ErrInvalidProposalContent)
	require.ErrorIs(t, handler(ctx, empty), govtypes.ErrInvalidProposalContent)

	foreign := &banktypes.MsgSend{FromAddress: governance, ToAddress: governance, Amount: sdk.NewCoins(sdk.NewInt64Coin(sdk.MustGetBaseDenom(), 1))}
	_, err := types.NewCustodyProposal("Foreign", "A bank message", foreign)
	require.ErrorIs(t, err, govtypes.ErrInvalidProposalContent)

	outsider := sdk.AccAddress(make([]byte, 20)).String()
	wrong, err := types.NewCustodyProposal("Wrong authority", "Not governance",
		&types.MsgSetAsset{Authority: outsider, Asset: sidAsset()})
	require.NoError(t, err)
	require.ErrorIs(t, wrong.ValidateBasic(), sdkerrors.ErrUnauthorized)
	require.ErrorIs(t, handler(ctx, wrong), sdkerrors.ErrUnauthorized)

	id, err := types.ParseHash32(assetID("SID"))
	require.NoError(t, err)
	_, found := k.GetAsset(ctx, id)
	require.False(t, found, "a refused proposal must not map the asset")
}

func TestSubmitCustodyProposalFile(t *testing.T) {
	governance := types.GovernanceAuthority()
	proposal, err := types.NewCustodyProposal("Custody assets", "Map SID",
		&types.MsgSetAsset{Authority: governance, Asset: types.AssetMapping{
			AssetId: assetID("SID"), Denom: "usid", Pointer: sidPointer, Enabled: true}})
	require.NoError(t, err)
	proposer := sdk.AccAddress(make([]byte, 20))
	cdc := custodyApp.AppCodec()

	msg, err := govtypes.NewMsgSubmitProposal(proposal, sdk.NewCoins(sdk.NewInt64Coin("usei", 1000)), proposer)
	require.NoError(t, err)
	content, ok := msg.GetContent().(*types.CustodyProposal)
	require.True(t, ok)
	require.Len(t, content.Messages, 1)

	file, err := cdc.MarshalAsJSON(content)
	require.NoError(t, err)
	var unknown map[string]any
	require.NoError(t, json.Unmarshal([]byte(file), &unknown))
	unknown["extra"] = true
	withUnknown, err := json.Marshal(unknown)
	require.NoError(t, err)
	err = cdc.UnmarshalAsJSON(withUnknown, &types.CustodyProposal{})
	require.Error(t, err)
}

func authorityProposal(t *testing.T, authority string) *types.CustodyProposal {
	t.Helper()
	params := custodyApp.LayerXCustodyKeeper.GetParams(newCtx())
	params.DepositRootAuthority = authority
	proposal, err := types.NewCustodyProposal("Deposit root authority", "Set the deposit root authority",
		&types.MsgUpdateParams{Authority: types.GovernanceAuthority(), Params: params})
	require.NoError(t, err)
	return proposal
}

func TestCustodyProposalExecutesOnlyFromActivationHeight(t *testing.T) {
	k := custodyApp.LayerXCustodyKeeper
	handler := layerxcustody.NewProposalHandler(k)
	proposal := authorityProposal(t, depositRootAuthority())

	never := inactiveCtx()
	before := k.GetParams(never)
	require.ErrorIs(t, handler(never, proposal), types.ErrGovernanceNotActive)
	require.Equal(t, before, k.GetParams(never))
	require.Empty(t, never.EventManager().Events())

	ctx := activate(inactiveCtx(), 20)
	require.Equal(t, int64(20), custodyApp.UpgradeKeeper.GetDoneHeight(ctx, types.GovernanceActivationUpgrade))
	early := ctx.WithBlockHeight(19)
	require.ErrorIs(t, handler(early, proposal), types.ErrGovernanceNotActive)
	require.Equal(t, before, k.GetParams(early))
	for _, height := range []int64{20, 21} {
		at, _ := ctx.WithBlockHeight(height).CacheContext()
		require.NoError(t, handler(at, proposal))
		require.Equal(t, depositRootAuthority(), k.GetParams(at).DepositRootAuthority)
	}
}

func TestCustodyProposalRejectsWrongContent(t *testing.T) {
	ctx := newCtx()
	k := custodyApp.LayerXCustodyKeeper
	before := k.GetParams(ctx)
	err := layerxcustody.NewProposalHandler(k)(ctx, govtypes.NewTextProposal("Text", "Not custody content", false))
	require.ErrorIs(t, err, sdkerrors.ErrUnknownRequest)
	require.Equal(t, before, k.GetParams(ctx))
	require.Empty(t, ctx.EventManager().Events())
}

func TestCustodyProposalRejectsUnauthorizedMessages(t *testing.T) {
	ctx := newCtx()
	k := custodyApp.LayerXCustodyKeeper
	handler := layerxcustody.NewProposalHandler(k)
	outsider := sdk.AccAddress(make([]byte, 20)).String()
	params := k.GetParams(ctx)
	before := params
	params.DepositRootAuthority = depositRootAuthority()
	for _, msg := range []sdk.Msg{
		&types.MsgUpdateParams{Authority: outsider, Params: params},
		&types.MsgSetAsset{Authority: outsider, Asset: sidAsset()},
		&types.MsgRegisterCheckpoint{Authority: outsider, BatchNumber: 1, StateRoot: assetID("state"), ReceiptRoot: assetID("receipt")},
		&types.MsgSetEmergency{Authority: outsider, Enabled: true},
		&types.MsgCancelClaim{Authority: outsider, ClaimId: assetID("claim")},
	} {
		proposal, err := types.NewCustodyProposal("Outsider", "Not governance", msg)
		require.NoError(t, err)
		require.ErrorIs(t, handler(ctx, proposal), sdkerrors.ErrUnauthorized, "%T", msg)
	}
	require.Equal(t, before, k.GetParams(ctx))
	require.Empty(t, ctx.EventManager().Events())
}

func TestCustodyProposalPreservesMessageOrder(t *testing.T) {
	ctx := newCtx()
	k := custodyApp.LayerXCustodyKeeper
	governance := types.GovernanceAuthority()
	first, second := k.GetParams(ctx), k.GetParams(ctx)
	first.DepositRootAuthority = depositRootAuthority()
	seed := sha256.Sum256([]byte("second authority"))
	second.DepositRootAuthority = hex.EncodeToString(ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey))
	proposal, err := types.NewCustodyProposal("Ordered", "The last update wins",
		&types.MsgUpdateParams{Authority: governance, Params: first},
		&types.MsgUpdateParams{Authority: governance, Params: second})
	require.NoError(t, err)
	require.NoError(t, layerxcustody.NewProposalHandler(k)(ctx, proposal))
	require.Equal(t, second.DepositRootAuthority, k.GetParams(ctx).DepositRootAuthority)
}

func TestCustodyProposalRollsBackStateIndexesAndEvents(t *testing.T) {
	ctx := newCtx()
	k := custodyApp.LayerXCustodyKeeper
	governance := types.GovernanceAuthority()
	params := k.GetParams(ctx)
	before := params
	params.DepositRootAuthority = depositRootAuthority()
	proposal, err := types.NewCustodyProposal("Partial", "A lawful update then a missing pending claim",
		&types.MsgSetAsset{Authority: governance, Asset: sidAsset()},
		&types.MsgUpdateParams{Authority: governance, Params: params},
		&types.MsgCancelClaim{Authority: governance, ClaimId: assetID("missing claim")})
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())
	eventsBefore := len(ctx.EventManager().Events())

	require.Error(t, layerxcustody.NewProposalHandler(k)(ctx, proposal))

	require.Equal(t, before, k.GetParams(ctx))
	id, err := types.ParseHash32(assetID("SID"))
	require.NoError(t, err)
	_, found := k.GetAsset(ctx, id)
	require.False(t, found)
	_, found = k.GetAssetByPointer(ctx, mustAddress(t, sidPointer))
	require.False(t, found)
	require.Len(t, ctx.EventManager().Events(), eventsBefore)
}

func custodyGovernanceContext(t *testing.T) (sdk.Context, *keeper.Keeper) {
	t.Helper()
	app := custodyApp
	ctx := newCtx().WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	k := keeper.NewKeeper(app.AppCodec(), app.GetKey(types.StoreKey),
		app.AccountKeeper, app.BankKeeper, &app.EvmKeeper, app.UpgradeKeeper)
	k.InitGenesis(ctx, *types.DefaultGenesis())
	vectors, err := testvectors.Load()
	require.NoError(t, err)
	require.NotEmpty(t, vectors["withdrawal"])
	v := vectors["withdrawal"][0]
	decode := func(name string) []byte {
		value, err := v.Bytes(name)
		require.NoError(t, err)
		return value
	}
	before := k.ExportGenesis(ctx)
	events := len(ctx.EventManager().Events())
	response, err := keeper.NewMsgServerImpl(k).RequestWithdrawal(sdk.WrapSDKContext(ctx), &types.MsgRequestWithdrawal{
		Receipt: decode("receipt"), Proof: decode("proof"), Header: decode("header"), HeaderSignature: decode("header_signature")})
	require.ErrorIs(t, err, types.ErrModuleRetired)
	require.EqualError(t, err, "custody module retired by the merge plan")
	require.Nil(t, response)
	require.Equal(t, before, k.ExportGenesis(ctx))
	require.Len(t, ctx.EventManager().Events(), events)
	return ctx, k
}

func governanceMessages(t *testing.T, ctx sdk.Context, k *keeper.Keeper) ([]sdk.Msg, types.Params, *types.MsgRegisterCheckpoint) {
	t.Helper()
	governance := types.GovernanceAuthority()
	params := k.GetParams(ctx)
	params.DepositRootAuthority = depositRootAuthority()
	checkpoint := &types.MsgRegisterCheckpoint{Authority: governance, BatchNumber: 2,
		StateRoot: assetID("governed state"), ReceiptRoot: assetID("governed receipt")}
	return []sdk.Msg{
		&types.MsgUpdateParams{Authority: governance, Params: params},
		&types.MsgSetAsset{Authority: governance, Asset: sidAsset()},
		checkpoint,
		&types.MsgSetEmergency{Authority: governance, Enabled: true},
		&types.MsgCancelClaim{Authority: governance, ClaimId: assetID("missing claim")},
	}, params, checkpoint
}

func custodyStoreSnapshot(t *testing.T, ctx sdk.Context) map[string][]byte {
	t.Helper()
	iterator := ctx.KVStore(custodyApp.GetKey(types.StoreKey)).Iterator(nil, nil)
	defer func() { require.NoError(t, iterator.Close()) }()
	state := make(map[string][]byte)
	for ; iterator.Valid(); iterator.Next() {
		state[string(iterator.Key())] = append([]byte(nil), iterator.Value()...)
	}
	return state
}

func TestCustodyProposalRetirementRefusesAllGovernanceMessages(t *testing.T) {
	ctx, k := custodyGovernanceContext(t)
	messages, _, _ := governanceMessages(t, ctx, k)
	custodyApp.UpgradeKeeper.SetDone(ctx, types.MergePlanUpgrade)
	before := k.ExportGenesis(ctx)
	state := custodyStoreSnapshot(t, ctx)
	events := append(sdk.Events{}, ctx.EventManager().Events()...)
	proposal, err := types.NewCustodyProposal("Retired custody", "Refuse every governed custody message", messages...)
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())
	require.ErrorIs(t, layerxcustody.NewProposalHandler(k)(ctx, proposal), types.ErrModuleRetired)
	for _, msg := range messages {
		one, err := types.NewCustodyProposal("Retired custody", "Refuse a governed message", msg)
		require.NoError(t, err)
		require.ErrorIs(t, layerxcustody.NewProposalHandler(k)(ctx, one), types.ErrModuleRetired, "%T", msg)
	}
	require.Equal(t, before, k.ExportGenesis(ctx))
	require.Equal(t, state, custodyStoreSnapshot(t, ctx))
	require.Equal(t, events, ctx.EventManager().Events())
}

func TestCustodyProposalWithoutUpgradeReaderPreservesStateAndEvents(t *testing.T) {
	ctx, k := custodyGovernanceContext(t)
	app := custodyApp
	withoutReader := keeper.NewKeeper(app.AppCodec(), app.GetKey(types.StoreKey),
		app.AccountKeeper, app.BankKeeper, &app.EvmKeeper, nil)
	messages, _, _ := governanceMessages(t, ctx, k)
	proposal, err := types.NewCustodyProposal("Missing activation reader", "Every governed message must be refused", messages...)
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())
	require.NoError(t, k.GovernanceExecutionActive(ctx))
	before := custodyStoreSnapshot(t, ctx)
	genesisBefore := k.ExportGenesis(ctx)
	eventsBefore := append(sdk.Events{}, ctx.EventManager().Events()...)

	require.ErrorIs(t, layerxcustody.NewProposalHandler(withoutReader)(ctx, proposal), types.ErrGovernanceNotActive)

	require.Equal(t, before, custodyStoreSnapshot(t, ctx))
	require.Equal(t, genesisBefore, k.ExportGenesis(ctx))
	require.Equal(t, eventsBefore, ctx.EventManager().Events())
}

func TestCustodyProposalRollsBackAllGovernanceStateAndEvents(t *testing.T) {
	for _, failure := range []string{"missing claim", "immutable checkpoint", "unauthorized"} {
		t.Run(failure, func(t *testing.T) {
			ctx, k := custodyGovernanceContext(t)
			messages, _, checkpoint := governanceMessages(t, ctx, k)
			messages = messages[:len(messages)-1]
			var expectedError error
			switch failure {
			case "missing claim":
				messages = append(messages, &types.MsgCancelClaim{Authority: types.GovernanceAuthority(), ClaimId: assetID("missing claim")})
				expectedError = types.ErrClaimNotPending
			case "immutable checkpoint":
				require.NoError(t, k.RegisterCheckpoint(ctx, 1, [32]byte{1}, [32]byte{2}))
				messages = append(messages, &types.MsgRegisterCheckpoint{Authority: types.GovernanceAuthority(),
					BatchNumber: 1, StateRoot: checkpoint.StateRoot, ReceiptRoot: checkpoint.ReceiptRoot})
				expectedError = types.ErrInvalidCheckpoint
			case "unauthorized":
				messages = append(messages, &types.MsgSetEmergency{Authority: sdk.AccAddress(make([]byte, 20)).String(), Enabled: false})
				expectedError = sdkerrors.ErrUnauthorized
			}
			proposal, err := types.NewCustodyProposal("Atomic custody", "Refuse without preserving partial mutations", messages...)
			require.NoError(t, err)
			before := custodyStoreSnapshot(t, ctx)
			genesisBefore := k.ExportGenesis(ctx)
			eventsBefore := append(sdk.Events{}, ctx.EventManager().Events()...)
			require.ErrorIs(t, layerxcustody.NewProposalHandler(k)(ctx, proposal), expectedError)
			require.Equal(t, before, custodyStoreSnapshot(t, ctx))
			require.Equal(t, genesisBefore, k.ExportGenesis(ctx))
			require.Equal(t, eventsBefore, ctx.EventManager().Events())
			_, found := k.GetCheckpoint(ctx, checkpoint.BatchNumber)
			require.False(t, found)
			_, found = k.GetAssetByPointer(ctx, mustAddress(t, sidPointer))
			require.False(t, found)
			_, found = k.GetAssetByDenom(ctx, "usid")
			require.False(t, found)
		})
	}
}

func TestCustodyProposalRefusesTypedNilWithoutMutation(t *testing.T) {
	ctx := newCtx()
	k := custodyApp.LayerXCustodyKeeper
	before := k.ExportGenesis(ctx)
	events := len(ctx.EventManager().Events())
	var proposal *types.CustodyProposal
	require.ErrorIs(t, layerxcustody.NewProposalHandler(k)(ctx, proposal), govtypes.ErrInvalidProposalContent)
	require.Equal(t, before, k.ExportGenesis(ctx))
	require.Len(t, ctx.EventManager().Events(), events)
}
