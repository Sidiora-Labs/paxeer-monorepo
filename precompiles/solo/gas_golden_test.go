package solo_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/solo"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSoloGasGolden(t *testing.T) {
	const (
		planHeight  = int64(200)
		suppliedGas = uint64(1_000_000)
	)
	golden := map[string]uint64{
		"claim":         27_406,
		"claimSpecific": 28_247,
	}
	app := testkeeper.EVMTestApp
	k := &app.EvmKeeper
	p, err := solo.NewPrecompile(app.GetPrecompileKeepers())
	require.NoError(t, err)
	servedABI := p.GetABI()
	require.Len(t, servedABI.Methods, len(golden), "every ABI method needs a gas golden")
	for name := range servedABI.Methods {
		_, ok := golden[name]
		require.True(t, ok, "missing gas golden for %s", name)
	}

	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithChainID("pax-test").WithBlockTime(time.Unix(1_700_000_000, 0)).
		WithIsEVM(true).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	app.UpgradeKeeper.SetDone(ctx.WithBlockHeight(planHeight), pcommon.MergeUpgrade)
	require.Equal(t, planHeight, app.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))

	key := secp256k1.GenPrivKeyFromSecret([]byte("solo-gas-golden-sender"))
	sender := sdk.AccAddress(key.PubKey().Address())
	caller := common.HexToAddress("0x1234567890123456789012345678901234567890")
	recipient := sdk.AccAddress(caller.Bytes())
	account := authtypes.NewBaseAccount(sender, key.PubKey(), 10, 0)
	k.AccountKeeper().SetAccount(ctx, account)
	k.AccountKeeper().SetAccount(ctx, authtypes.NewBaseAccount(recipient, nil, 11, 0))
	coins := sdk.NewCoins(sdk.NewInt64Coin("foo", 7))
	require.NoError(t, k.BankKeeper().AddCoins(ctx, sender, coins, false))
	require.Equal(t, coins, k.BankKeeper().GetAllBalances(ctx, sender))
	require.Empty(t, k.BankKeeper().GetAllBalances(ctx, recipient))

	for _, name := range []string{"claim", "claimSpecific"} {
		t.Run(name, func(t *testing.T) {
			var msg sdk.Msg
			switch name {
			case "claim":
				msg = evmtypes.NewMsgClaim(sender, caller)
			case "claimSpecific":
				msg = evmtypes.NewMsgClaimSpecific(sender, caller, &evmtypes.Asset{
					AssetType: evmtypes.AssetType_TYPENATIVE,
					Denom:     "foo",
				})
			}
			payload := signClaimMsg(t, msg, sender, caller, account, key)
			input, err := servedABI.Pack(name, payload)
			require.NoError(t, err)
			method := servedABI.Methods[name]
			want, err := method.Outputs.Pack(true)
			require.NoError(t, err)

			var charged [2]uint64
			for i, height := range []int64{planHeight - 1, planHeight} {
				t.Run(fmt.Sprintf("height_%d", height), func(t *testing.T) {
					callCtx, _ := ctx.WithBlockHeight(height).CacheContext()
					require.Equal(t, height >= planHeight, pcommon.MergePlanActive(callCtx, k))
					db := state.NewDBImpl(callCtx, k, false)
					evm := &vm.EVM{StateDB: db}
					ret, remaining, err := p.RunAndCalculateGas(evm, caller, caller,
						input, suppliedGas, big.NewInt(0), nil, false, false)
					require.NoError(t, err, "precompile error: %v", db.GetPrecompileError())
					require.NoError(t, db.GetPrecompileError())
					require.Equal(t, want, ret)
					require.Less(t, remaining, suppliedGas)
					charged[i] = suppliedGas - remaining
					t.Logf("%s height=%d dynamic gas=%d", name, height, charged[i])
					assert.Equal(t, golden[name], charged[i], "frozen dynamic gas for %s", name)
					require.Empty(t, k.BankKeeper().GetAllBalances(db.Ctx(), sender))
					require.Equal(t, coins, k.BankKeeper().GetAllBalances(db.Ctx(), recipient))
					require.Equal(t, uint64(1), k.AccountKeeper().GetAccount(db.Ctx(), sender).GetSequence())
				})
			}
			require.NotZero(t, charged[0])
			require.Equal(t, charged[0], charged[1], "dynamic gas changed at merge plan height")
		})
	}
}
