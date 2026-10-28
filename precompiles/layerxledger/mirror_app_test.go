package layerxledger_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestBankMirrorMatchesKernelBalances(t *testing.T) {
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(100).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	a.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
	address := common.HexToAddress("0x101A")
	served := a.EvmKeeper.CustomPrecompiles(ctx)
	require.Contains(t, served, address)
	p, ok := served[address].(precompiles.IPrecompile)
	require.True(t, ok)
	require.Equal(t, common.HexToAddress(layerxledger.LedgerAddress), p.Address())
	require.Equal(t, precompiles.MergeUpgrade, a.EvmKeeper.GetCustomPrecompilesVersions(ctx)[address])
	db := state.NewDBImpl(ctx, &a.EvmKeeper, true)
	ctx = db.Ctx()
	evm := &vm.EVM{StateDB: db}
	abi := p.GetABI()
	call := func(caller common.Address, name string, args ...interface{}) []interface{} {
		t.Helper()
		input, err := abi.Pack(name, args...)
		require.NoError(t, err, name)
		evm.TxContext.Origin = caller
		data, err := p.Run(evm, caller, caller, input, nil, abi.Methods[name].IsConstant(), false, nil)
		require.NoError(t, err, "%s returned %x", name, data)
		outputs := abi.Methods[name].Outputs
		switch name {
		case "register", "openAccount", "issueGrant":
			outputs = abi.Methods["balanceOf"].Inputs[:1]
		}
		values, err := outputs.Unpack(data)
		require.NoError(t, err, name)
		return values
	}
	bind := func(caller common.Address, seedByte byte) (ed25519.PrivateKey, ledgerkeeper.Actor) {
		t.Helper()
		var seed [ed25519.SeedSize]byte
		seed[0] = seedByte
		private := ed25519.NewKeyFromSeed(seed[:])
		var key [32]byte
		copy(key[:], private.Public().(ed25519.PublicKey))
		message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), caller, 0)
		require.NoError(t, err)
		var signature [ed25519.SignatureSize]byte
		copy(signature[:], ed25519.Sign(private, message))
		_, err = a.EvmKeeper.BindLayerX(ctx, caller, key, signature)
		require.NoError(t, err)
		actor, err := a.LayerXLedgerKeeper.Actor(ctx, caller)
		require.NoError(t, err)
		return private, actor
	}
	payerCaller := common.HexToAddress("0x2320000000000000000000000000000000000001")
	payeeCaller := common.HexToAddress("0x2320000000000000000000000000000000000002")
	private, payer := bind(payerCaller, 1)
	_, payee := bind(payeeCaller, 2)
	salt := [32]byte{0x41}
	material := append([]byte("LX:ASSET:v1"), payer.Authority.Actor[:]...)
	asset := sha256.Sum256(append(material, salt[:]...))
	const cap int64 = 1000
	denom := types.MirrorDenom(asset)
	_, issuance, err := lxphost.IssuanceAccountID(asset)
	require.NoError(t, err)
	main := transferMainAccount(t, &harness{
		ctx: ctx, host: a.LayerXHost, hostKey: a.GetKey(app.LayerXHostStoreKey),
	}, payer, asset)
	balances := map[[32]byte]int64{issuance: cap, main: 0}
	assertMirror := func(stage string, units int64) {
		t.Helper()
		var sum int64
		for account, want := range balances {
			kernel := call(payeeCaller, "balanceOf", account, asset)[0].(*big.Int)
			require.Zero(t, kernel.Cmp(big.NewInt(want)), "%s: kernel account %x", stage, account)
			bank := a.BankKeeper.GetBalance(ctx, lxphost.BankAddress(account), denom)
			require.Equal(t, denom, bank.Denom, stage)
			require.Zero(t, bank.Amount.BigInt().Cmp(kernel), "%s: bank account %x", stage, account)
			sum += want
		}
		require.Equal(t, cap, sum, stage)
		supply := a.BankKeeper.GetSupply(ctx, denom)
		require.Equal(t, denom, supply.Denom, stage)
		require.Zero(t, supply.Amount.BigInt().Cmp(big.NewInt(cap)), stage)
		total := call(payeeCaller, "totalUnits", asset)[0].(*big.Int)
		require.Zero(t, total.Cmp(big.NewInt(units)), stage)
		require.Zero(t, new(big.Int).Add(total, big.NewInt(balances[issuance])).Cmp(supply.Amount.BigInt()), stage)
	}

	registered := call(payerCaller, "register", asset, salt, "MIR", "Mirror integration", uint8(6),
		big.NewInt(cap), uint8(types.AssetIssuerNative), []byte{})
	require.Equal(t, []interface{}{asset}, registered)
	assertMirror("register", 0)
	from := call(payerCaller, "openAccount", asset)[0].([32]byte)
	wantFrom, err := ledgerkeeper.AccountID(payer.DIDKey, asset)
	require.NoError(t, err)
	require.Equal(t, wantFrom, from)
	balances[from] = 0
	assertMirror("payer openAccount", 0)
	to := call(payeeCaller, "openAccount", asset)[0].([32]byte)
	wantTo, err := ledgerkeeper.AccountID(payee.DIDKey, asset)
	require.NoError(t, err)
	require.Equal(t, wantTo, to)
	balances[to] = 0
	assertMirror("payee openAccount", 0)
	call(payerCaller, "mint", asset, from, big.NewInt(250))
	balances[issuance], balances[from] = 750, 250
	assertMirror("mint", 250)

	now := uint64(ctx.BlockTime().UnixMilli())
	call(payerCaller, "payExact", from, main, asset, big.NewInt(100), now+1000, [32]byte{0x42})
	balances[from], balances[main] = 150, 100
	assertMirror("fund main with payExact", 250)
	key := [32]byte{0x43}
	material = append(append(append([]byte(nil), main[:]...), to[:]...), asset[:]...)
	var amount [16]byte
	big.NewInt(40).FillBytes(amount[:])
	material = append(material, amount[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(material, key[:]...))
	require.NoError(t, err)
	mainName, _, err := lxphost.MainAccountID(payer.DIDKey)
	require.NoError(t, err)
	mainAccount, err := a.LayerXHost.ReadAccount(ctx, mainName, main)
	require.NoError(t, err)
	payload, err := types.EncodeSendPayload(types.SendArgs{
		From: main, To: to, Asset: asset, Amount: mainAccount.Balance.SubRaw(60),
		Sequence: mainAccount.NextSequence, IdempotencyKey: key, ExpiresAt: now + 1000,
		ContextHash: contextHash, PublicKey: payer.DIDKey,
		NetworkID: uint32(a.EvmKeeper.ChainID(ctx).Uint64()), ProtocolVersion: codec.StateCommitmentProtocolVersion,
	})
	require.NoError(t, err)
	authorization := len(payload) - 167
	message := append(append([]byte(nil), payload[:2]...), payload[4:authorization]...)
	message = append(message, payload[authorization:authorization+33]...)
	message = append(message, payload[authorization+129:]...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	copy(payload[authorization+65:authorization+129], ed25519.Sign(private, digest[:]))
	require.NoError(t, types.ValidateSendEnvelope(payload))
	call(payerCaller, "send", payload)
	balances[main], balances[to] = 60, 40
	assertMirror("send", 250)

	purpose := [32]byte{0x44}
	contextHash, err = codec.DomainHash(codec.DomainContextHash, purpose[:])
	require.NoError(t, err)
	body := append([]byte(nil), from[:]...)
	body = append(body, to[:]...)
	body = append(body, asset[:]...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, 40)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, 100)
	body = append(body, 0)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, now+1000)
	body = append(body, purpose[:]...)
	body = append(body, 0)
	body = append(body, make([]byte, 32)...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = append(body, payer.DIDKey[:]...)
	grantID, err := codec.DomainHash(codec.DomainAuthorityHash, append([]byte("LXP:GRANT:v1"), body...))
	require.NoError(t, err)
	grant := append(append([]byte(nil), grantID[:]...), body...)
	grant = append(grant, ed25519.Sign(private, grantID[:])...)
	require.NoError(t, types.ValidateIssueGrantPayload(grant))
	require.Equal(t, []interface{}{grantID}, call(payerCaller, "issueGrant", grant))
	assertMirror("issueGrant", 250)
	sequence := call(payeeCaller, "nextSequence", payee.DIDKey, asset)[0].(uint64)
	call(payeeCaller, "draw", grantID, to, big.NewInt(15), [32]byte{0x45}, sequence, purpose, contextHash)
	balances[from], balances[to] = 135, 55
	assertMirror("draw", 250)
	call(payerCaller, "payExact", from, to, asset, big.NewInt(3), now+1000, [32]byte{0x46})
	balances[from], balances[to] = 132, 58
	assertMirror("payExact", 250)
}
