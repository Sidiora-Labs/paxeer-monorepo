package keeper_test

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/keeper"
	bridgetestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

func repair182Suite(t *testing.T) *suite {
	t.Helper()
	a := app.Setup(t, true, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	at := time.Unix(1_800_000_000, 0).UTC()
	genesis, err := json.Marshal(app.NewDefaultGenesisState(a.AppCodec()))
	require.NoError(t, err)
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: at, ChainId: "pax-test", InitialHeight: 1,
		ConsensusParams: app.DefaultConsensusParams, AppStateBytes: genesis,
	})
	require.NoError(t, err)
	return &suite{t: t, app: a, k: a.LayerXBridgeKeeper,
		ctx:       a.NewContext(false, tmproto.Header{ChainID: "pax-test", Height: 1, Time: at}),
		attestors: bridgetestutil.Attestors(3), denom: types.SidioraDenom()}
}

func repair182Govern(s *suite, msgs ...sdk.Msg) error {
	s.t.Helper()
	submit, content := s.submitted(msgs...)
	require.NoError(s.t, submit.ValidateBasic())
	proposal, err := s.app.GovKeeper.SubmitProposal(s.ctx, content)
	require.NoError(s.t, err)
	stored, found := s.app.GovKeeper.GetProposal(s.ctx, proposal.ProposalId)
	require.True(s.t, found)
	require.Equal(s.t, types.RouterKey, stored.ProposalRoute())
	handler := s.app.GovKeeper.Router().GetRoute(stored.ProposalRoute())
	require.NotNil(s.t, handler)
	return handler(s.ctx, stored.GetContent())
}

func repair182AssertNoGenericSidiora(s *suite) {
	s.t.Helper()
	denom := types.Denom(types.SidioraHomeChainID, sidioraAsset())
	_, found := s.k.GetAssetByDenom(s.ctx, denom)
	require.False(s.t, found)
	_, found = s.k.GetCap(s.ctx, denom)
	require.False(s.t, found)
	_, found = s.app.BankKeeper.GetDenomMetaData(s.ctx, denom)
	require.False(s.t, found)
	denoms, err := s.app.TokenFactoryKeeper.DenomsFromCreator(sdk.WrapSDKContext(s.ctx),
		&tokenfactorytypes.QueryDenomsFromCreatorRequest{Creator: s.k.ModuleAddress().String()})
	require.NoError(s.t, err)
	require.NotContains(s.t, denoms.Denoms, denom)
	require.True(s.t, s.app.BankKeeper.GetSupply(s.ctx, denom).Amount.IsZero())
}

func repair182RoundTrip(s *suite) *suite {
	s.t.Helper()
	wantBridge := s.k.ExportGenesis(s.ctx)
	wantBank := s.app.BankKeeper.ExportGenesis(s.ctx)
	wantFactory := s.app.TokenFactoryKeeper.ExportGenesis(s.ctx)
	require.NoError(s.t, wantBridge.Validate())
	require.NoError(s.t, s.app.SetSealedStateToCommit(s.ctx, s.ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err := s.app.Commit(context.Background())
	require.NoError(s.t, err)
	exported, err := s.app.ExportAppStateAndValidators(false, nil)
	require.NoError(s.t, err)
	var state app.GenesisState
	require.NoError(s.t, json.Unmarshal(exported.AppState, &state))
	var bridge types.GenesisState
	require.NoError(s.t, json.Unmarshal(state[types.ModuleName], &bridge))
	require.NoError(s.t, bridge.Validate())
	require.Equal(s.t, wantBridge, bridge)
	var bank banktypes.GenesisState
	require.NoError(s.t, s.app.AppCodec().UnmarshalAsJSON(state[banktypes.ModuleName], &bank))
	var wantBankJSON banktypes.GenesisState
	require.NoError(s.t, s.app.AppCodec().UnmarshalAsJSON(s.app.AppCodec().MustMarshalJSON(wantBank), &wantBankJSON))
	require.Equal(s.t, &wantBankJSON, &bank)
	var factory tokenfactorytypes.GenesisState
	require.NoError(s.t, s.app.AppCodec().UnmarshalAsJSON(state[tokenfactorytypes.ModuleName], &factory))
	var wantFactoryJSON tokenfactorytypes.GenesisState
	require.NoError(s.t, s.app.AppCodec().UnmarshalAsJSON(s.app.AppCodec().MustMarshalJSON(wantFactory), &wantFactoryJSON))
	require.Equal(s.t, &wantFactoryJSON, &factory)

	restored := app.Setup(s.t, true, false, false)
	s.t.Cleanup(func() { require.NoError(s.t, restored.Close()) })
	_, err = restored.InitChain(context.Background(), &abci.RequestInitChain{
		Time: s.ctx.BlockTime(), ChainId: "pax-test", InitialHeight: exported.Height,
		ConsensusParams: exported.ConsensusParams, AppStateBytes: exported.AppState,
	})
	require.NoError(s.t, err)
	ctx := restored.NewContext(false, tmproto.Header{ChainID: "pax-test", Height: exported.Height, Time: s.ctx.BlockTime()})
	fresh := &suite{t: s.t, app: restored, ctx: ctx, k: restored.LayerXBridgeKeeper,
		attestors: s.attestors, denom: s.denom}
	require.Equal(s.t, wantBridge, fresh.k.ExportGenesis(ctx))
	importedBank := restored.BankKeeper.ExportGenesis(ctx)
	require.Equal(s.t, wantBank.Balances, importedBank.Balances)
	require.Equal(s.t, wantBank.Supply, importedBank.Supply)
	require.Equal(s.t, wantFactory, restored.TokenFactoryKeeper.ExportGenesis(ctx))
	repair182AssertNoGenericSidiora(fresh)
	return fresh
}

func TestRepair_182_CapBeforePairRefusesBeforeWrites(t *testing.T) {
	s := repair182Suite(t)
	require.NoError(t, repair182Govern(s, &types.MsgRegisterChain{Authority: authority, Chain: solanaChain}))
	before := s.k.ExportGenesis(s.ctx)
	bank := s.app.BankKeeper.ExportGenesis(s.ctx)
	factory := s.app.TokenFactoryKeeper.ExportGenesis(s.ctx)
	events := append(sdk.Events(nil), s.ctx.EventManager().Events()...)
	response, err := keeper.NewMsgServerImpl(s.k).SetCap(sdk.WrapSDKContext(s.ctx), sidioraCap(authority))
	require.ErrorIs(t, err, types.ErrInvalidRequest)
	require.Nil(t, response)
	require.Equal(t, before, s.k.ExportGenesis(s.ctx))
	require.Equal(t, bank, s.app.BankKeeper.ExportGenesis(s.ctx))
	require.Equal(t, factory, s.app.TokenFactoryKeeper.ExportGenesis(s.ctx))
	require.Equal(t, events, s.ctx.EventManager().Events())
	_, found := s.k.GetAsset(s.ctx, types.SidioraHomeChainID, sidioraAsset())
	require.False(t, found)
	repair182AssertNoGenericSidiora(s)
	s = repair182RoundTrip(s)
	_, found = s.k.GetAsset(s.ctx, types.SidioraHomeChainID, sidioraAsset())
	require.False(t, found)
}

func TestRepair_182_GovernanceOrdersRoundTrip(t *testing.T) {
	for _, capFirst := range []bool{true, false} {
		name := "pair_first"
		if capFirst {
			name = "cap_first"
		}
		t.Run(name, func(t *testing.T) {
			s := repair182Suite(t)
			require.NoError(t, repair182Govern(s,
				&types.MsgRegisterChain{Authority: authority, Chain: solanaChain},
				&types.MsgRegisterChain{Authority: authority, Chain: types.Chain{ChainID: chainID, Vault: vault, FinalityDepth: 64, Enabled: true}},
				&types.MsgSetAttestors{Authority: authority, Set: bridgetestutil.Set(s.attestors, 1_000_000, 2)},
				&types.MsgSetCap{Authority: authority, ChainID: chainID, Asset: asset, MaxInFlight: sdk.NewInt(1500), MaxPerTx: sdk.NewInt(1000)}))
			ordinary := deposit(19, 333)
			_, err := s.k.BridgeIn(s.ctx, ordinary, s.signed(ordinary, s.attestors[0], s.attestors[1]))
			require.NoError(t, err)
			pair := &types.MsgRegisterSidioraPair{Authority: authority, ChainID: types.SidioraHomeChainID}
			if capFirst {
				before := s.k.ExportGenesis(s.ctx)
				bank := s.app.BankKeeper.ExportGenesis(s.ctx)
				factory := s.app.TokenFactoryKeeper.ExportGenesis(s.ctx)
				for _, messages := range [][]sdk.Msg{{sidioraCap(authority)}, {sidioraCap(authority), pair}} {
					require.ErrorIs(t, repair182Govern(s, messages...), types.ErrInvalidRequest)
					require.Equal(t, before, s.k.ExportGenesis(s.ctx))
					require.Equal(t, bank, s.app.BankKeeper.ExportGenesis(s.ctx))
					require.Equal(t, factory, s.app.TokenFactoryKeeper.ExportGenesis(s.ctx))
					require.Len(t, s.events(types.EventCapSet), 1)
					require.Empty(t, s.events(types.EventSidioraPair))
					repair182AssertNoGenericSidiora(s)
				}
				s = repair182RoundTrip(s)
			}

			bankBefore := s.app.BankKeeper.ExportGenesis(s.ctx)
			require.NoError(t, repair182Govern(s, pair, sidioraCap(authority)))
			require.Equal(t, bankBefore.Balances, s.app.BankKeeper.ExportGenesis(s.ctx).Balances)
			require.Equal(t, bankBefore.Supply, s.app.BankKeeper.ExportGenesis(s.ctx).Supply)
			repair182AssertNoGenericSidiora(s)
			in := sidioraDeposit(1, 700)
			in.ChainID = types.SidioraHomeChainID
			minted, err := s.k.BridgeIn(s.ctx, in, s.signed(in, s.attestors[0], s.attestors[1]))
			require.NoError(t, err)
			require.Equal(t, types.SidioraDenom(), minted.Denom)
			_, err = s.k.BridgeOut(s.ctx, recipient, types.SidioraHomeChainID, sidioraAsset(), big.NewInt(200), types.Address20{0x33})
			require.NoError(t, err)
			require.Equal(t, sdk.NewInt(500), s.balance(recipient))
			require.Equal(t, sdk.NewInt(500), s.app.BankKeeper.GetSupply(s.ctx, s.denom).Amount)
			bankBefore = s.app.BankKeeper.ExportGenesis(s.ctx)
			require.NoError(t, repair182Govern(s, sidioraCap(authority), pair))
			require.Equal(t, bankBefore, s.app.BankKeeper.ExportGenesis(s.ctx))
			metadata, found := s.app.BankKeeper.GetDenomMetaData(s.ctx, s.denom)
			require.True(t, found)
			require.Equal(t, keeper.SidioraMetadata(), metadata)

			s = repair182RoundTrip(s)
			record, found := s.k.GetAsset(s.ctx, types.SidioraHomeChainID, sidioraAsset())
			require.True(t, found)
			require.Equal(t, types.BridgedAsset{ChainID: types.SidioraHomeChainID, Asset: sidioraAsset(), Denom: s.denom}, record)
			reverse, found := s.k.GetAssetByDenom(s.ctx, s.denom)
			require.True(t, found)
			require.Equal(t, record, reverse)
			cap, found := s.k.GetCap(s.ctx, s.denom)
			require.True(t, found)
			require.Equal(t, types.Cap{Denom: s.denom, MaxInFlight: sdk.NewInt(1500), MaxPerTx: sdk.NewInt(1000)}, cap)
			require.Equal(t, sdk.NewInt(500), s.k.InFlight(s.ctx, s.denom))
			require.Equal(t, sdk.NewInt(500), s.balance(recipient))
			require.Equal(t, sdk.NewInt(500), s.app.BankKeeper.GetSupply(s.ctx, s.denom).Amount)
			require.Equal(t, sdk.NewInt(333), s.app.BankKeeper.GetBalance(s.ctx, s.app.EvmKeeper.GetPaxAddressOrDefault(s.ctx, recipient), types.Denom(chainID, asset)).Amount)
			require.True(t, s.k.IsNullified(s.ctx, in.Nullifier()))
			require.Equal(t, uint64(1), s.k.OutboundNonce(s.ctx, types.SidioraHomeChainID))
			admin, err := s.app.TokenFactoryKeeper.GetAuthorityMetadata(s.ctx, s.denom)
			require.NoError(t, err)
			require.Equal(t, s.k.ModuleAddress().String(), admin.Admin)
			_, err = s.k.BridgeIn(s.ctx, in, s.signed(in, s.attestors[0], s.attestors[1]))
			require.ErrorIs(t, err, types.ErrNullified)
			out, err := s.k.BridgeOut(s.ctx, recipient, types.SidioraHomeChainID, sidioraAsset(), big.NewInt(100), types.Address20{0x33})
			require.NoError(t, err)
			require.Equal(t, uint64(2), out.Nonce)
			require.Equal(t, sdk.NewInt(400), s.balance(recipient))
			require.Equal(t, sdk.NewInt(400), s.k.InFlight(s.ctx, s.denom))
			require.Equal(t, sdk.NewInt(400), s.app.BankKeeper.GetSupply(s.ctx, s.denom).Amount)
		})
	}
}
