package mint_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	"gopkg.in/yaml.v2"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint"
	mintcli "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	govcli "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/cli"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
)

func repairM571Keeper(t *testing.T) (sdk.Context, keeper.Keeper, *codec.ProtoCodec) {
	t.Helper()
	db := tmdb.NewMemDB()
	stores := store.NewCommitMultiStore(db)
	t.Cleanup(func() { require.NoError(t, stores.Close()) })
	keys := sdk.NewKVStoreKeys(types.StoreKey, authtypes.StoreKey, paramtypes.ModuleName)
	for _, key := range keys {
		stores.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	transient := sdk.NewTransientStoreKey("transient_params")
	stores.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	require.NoError(t, stores.LoadLatestVersion())
	ctx := sdk.NewContext(stores, tmproto.Header{Height: 1}, false)
	registry := cdctypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	govtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	params := paramskeeper.NewKeeper(cdc, codec.NewLegacyAmino(), keys[paramtypes.ModuleName], transient)
	accounts := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], params.Subspace(authtypes.ModuleName),
		authtypes.ProtoBaseAccount, map[string][]string{types.ModuleName: {authtypes.Minter}})
	k := keeper.NewKeeper(cdc, keys[types.StoreKey], params.Subspace(types.ModuleName),
		nil, accounts, nil, nil, authtypes.FeeCollectorName)
	k.SetParams(ctx, types.DefaultParams())
	k.SetMinter(ctx, types.NewMinter("2030-01-01", "2030-01-11", "uhpx", 500))
	return ctx, k, cdc
}

func repairM571Refusal(t *testing.T, ctx sdk.Context, k keeper.Keeper, proposal *types.UpdateMinterProposal, want error) {
	t.Helper()
	before := append([]byte(nil), ctx.KVStore(k.GetStoreKey()).Get(types.MinterKey)...)
	params := k.GetParams(ctx)
	events := append(sdk.Events{}, ctx.EventManager().Events()...)
	for _, execute := range []func() error{
		func() error { return mint.HandleUpdateMinterProposal(ctx, &k, proposal) },
		func() error { return mint.NewProposalHandler(k)(ctx, proposal) },
		func() error { return mint.HandleUpdateMinterProposal(sdk.Context{}, nil, proposal) },
	} {
		var err error
		require.NotPanics(t, func() { err = execute() })
		require.EqualError(t, err, want.Error())
		require.Equal(t, before, ctx.KVStore(k.GetStoreKey()).Get(types.MinterKey))
		require.Equal(t, params, k.GetParams(ctx))
		require.Equal(t, events, ctx.EventManager().Events())
	}
}

func TestRepairM_571_MissingContent(t *testing.T) {
	for _, transport := range []string{"omitted", "null", "protobuf"} {
		t.Run(transport, func(t *testing.T) {
			ctx, k, cdc := repairM571Keeper(t)
			var proposal types.UpdateMinterProposal
			if transport == "protobuf" {
				original := &types.UpdateMinterProposal{Title: "Update minter", Description: "Replace the release"}
				msg, err := govtypes.NewMsgSubmitProposal(original, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1)), sdk.AccAddress(bytes.Repeat([]byte{1}, 20)))
				require.NoError(t, err)
				wire, err := cdc.Marshal(msg)
				require.NoError(t, err)
				var decoded govtypes.MsgSubmitProposal
				require.NoError(t, cdc.Unmarshal(wire, &decoded))
				content, ok := decoded.GetContent().(*types.UpdateMinterProposal)
				require.True(t, ok)
				proposal = *content
			} else {
				body := `{"title":"Update minter","description":"Replace the release"}`
				if transport == "null" {
					body = `{"title":"Update minter","description":"Replace the release","minter":null}`
				}
				require.NoError(t, cdc.UnmarshalAsJSON([]byte(body), &proposal))
			}
			require.Nil(t, proposal.Minter)
			var err error
			require.NotPanics(t, func() { err = proposal.ValidateBasic() })
			require.ErrorIs(t, err, govtypes.ErrInvalidProposalContent)
			require.ErrorContains(t, err, "minter cannot be nil")
			repairM571Refusal(t, ctx, k, &proposal, err)
		})
	}
}

func TestRepairM_571_NilProposal(t *testing.T) {
	ctx, k, _ := repairM571Keeper(t)
	var proposal *types.UpdateMinterProposal
	var err error
	require.NotPanics(t, func() { err = proposal.ValidateBasic() })
	require.ErrorIs(t, err, govtypes.ErrInvalidProposalContent)
	require.ErrorContains(t, err, "proposal cannot be nil")
	repairM571Refusal(t, ctx, k, proposal, err)
	before := append([]byte(nil), ctx.KVStore(k.GetStoreKey()).Get(types.MinterKey)...)
	require.Error(t, mint.NewProposalHandler(k)(ctx, nil))
	require.Equal(t, before, ctx.KVStore(k.GetStoreKey()).Get(types.MinterKey))
}

func TestRepairM_571_InvalidMinter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*types.Minter)
	}{
		{"missing_last_date", func(m *types.Minter) { m.LastMintDate = "" }},
		{"reversed_dates", func(m *types.Minter) { m.EndDate = "2029-01-01" }},
		{"excess_remaining", func(m *types.Minter) { m.RemainingMintAmount = m.TotalMintAmount + 1 }},
		{"invalid_denom", func(m *types.Minter) { m.Denom = "invalid" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, k, cdc := repairM571Keeper(t)
			proposal := types.NewUpdateMinterProposalHandler("Update minter", "Replace the release", types.NewMinter("2030-01-01", "2030-01-11", "uhpx", 1000))
			tc.change(proposal.Minter)
			wire, err := cdc.Marshal(proposal)
			require.NoError(t, err)
			var decoded types.UpdateMinterProposal
			require.NoError(t, cdc.Unmarshal(wire, &decoded))
			want := decoded.ValidateBasic()
			require.Error(t, want)
			repairM571Refusal(t, ctx, k, &decoded, want)
		})
	}
}

func repairM571Generate(t *testing.T, cdc *codec.ProtoCodec, body []byte) *types.UpdateMinterProposal {
	t.Helper()
	path := filepath.Join(t.TempDir(), "minter_prop.json")
	require.NoError(t, os.WriteFile(path, body, 0o600))
	var output bytes.Buffer
	proposer := sdk.AccAddress(bytes.Repeat([]byte{0x57}, 20))
	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	clientCtx := client.Context{}.WithCodec(cdc).WithInterfaceRegistry(cdc.InterfaceRegistry()).
		WithTxConfig(txConfig).WithKeyring(keyring.NewInMemory()).WithChainID("mint-repair-571").
		WithFrom(proposer.String()).WithFromAddress(proposer).WithOffline(true).WithOutput(&output)
	root := govcli.NewTxCmd([]*cobra.Command{mintcli.UpdateMinterHandler.CLIHandler()})
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"submit-proposal", "update-minter", path, "--node", "", "--generate-only", "--offline", "--from", proposer.String(), "--deposit", "1uhpx"})
	require.NoError(t, root.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx)))
	transaction, err := txConfig.TxJSONDecoder()(output.Bytes())
	require.NoError(t, err)
	require.Len(t, transaction.GetMsgs(), 1)
	msg, ok := transaction.GetMsgs()[0].(*govtypes.MsgSubmitProposal)
	require.True(t, ok)
	require.NoError(t, msg.ValidateBasic())
	require.Equal(t, proposer.String(), msg.Proposer)
	require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1)), msg.InitialDeposit)
	wire, err := cdc.Marshal(msg)
	require.NoError(t, err)
	var decoded govtypes.MsgSubmitProposal
	require.NoError(t, cdc.Unmarshal(wire, &decoded))
	require.NoError(t, decoded.ValidateBasic())
	content, ok := decoded.GetContent().(*types.UpdateMinterProposal)
	require.True(t, ok)
	return content
}

func TestRepairM_571_ReadmeProposals(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	require.NoError(t, err)
	blocks := regexp.MustCompile("(?s)```json\\n(.*?)\\n```").FindAllSubmatch(readme, -1)
	require.Len(t, blocks, 3, "README contains fresh, ongoing and parameter proposals")
	beforeBlocks := regexp.MustCompile("(?s)```yaml\\n(.*?)\\n```").FindAllSubmatch(readme, -1)
	require.Len(t, beforeBlocks, 1)
	for index, name := range []string{"fresh", "ongoing"} {
		t.Run(name, func(t *testing.T) {
			ctx, k, cdc := repairM571Keeper(t)
			var submitted types.UpdateMinterProposal
			require.NoError(t, cdc.UnmarshalAsJSON(blocks[index][1], &submitted))
			require.NoError(t, submitted.ValidateBasic())
			proposal := repairM571Generate(t, cdc, blocks[index][1])
			require.Equal(t, &submitted, proposal)
			if name == "fresh" {
				want := types.NewMinter("2030-01-01", "2030-01-11", "uhpx", 100000)
				require.Equal(t, want, *proposal.Minter)
				require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10000)), proposal.Minter.GetReleaseAmountToday(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)))
			} else {
				var fields map[string]string
				require.NoError(t, yaml.Unmarshal(beforeBlocks[0][1], &fields))
				wire, err := json.Marshal(fields)
				require.NoError(t, err)
				var before types.Minter
				require.NoError(t, cdc.UnmarshalAsJSON(wire, &before))
				require.NoError(t, types.ValidateMinter(before))
				k.SetMinter(ctx, before)
				require.Equal(t, uint64(40000), before.TotalMintAmount-before.RemainingMintAmount)
				require.Equal(t, before.TotalMintAmount-before.RemainingMintAmount, proposal.Minter.TotalMintAmount-proposal.Minter.RemainingMintAmount)
				require.Equal(t, before.LastMintAmount, proposal.Minter.LastMintAmount)
				require.Equal(t, before.LastMintDate, proposal.Minter.LastMintDate)
				require.Equal(t, before.LastMintHeight, proposal.Minter.LastMintHeight)
				require.Equal(t, uint64(150000), proposal.Minter.TotalMintAmount)
				require.Equal(t, uint64(110000), proposal.Minter.RemainingMintAmount)
				require.Equal(t, "2030-01-16", proposal.Minter.EndDate)
				require.True(t, proposal.Minter.GetReleaseAmountToday(time.Date(2030, 1, 4, 12, 0, 0, 0, time.UTC)).IsZero())
				require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10000)), proposal.Minter.GetReleaseAmountToday(time.Date(2030, 1, 5, 0, 0, 0, 0, time.UTC)))
			}
			require.NoError(t, mint.NewProposalHandler(k)(ctx, proposal))
			require.Equal(t, *proposal.Minter, k.GetMinter(ctx))
			response, err := keeper.NewQuerier(k).Minter(sdk.WrapSDKContext(ctx), &types.QueryMinterRequest{})
			require.NoError(t, err)
			require.Equal(t, *proposal.Minter, types.Minter(*response))
		})
	}
}
