package layerxauth_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestAuthGasConformance(t *testing.T) {
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithChainID("pax-test").WithBlockHeight(100).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	a.RefreshPrecompiles(ctx)
	address := common.HexToAddress("0x101C")
	require.Equal(t, common.HexToAddress(layerxauth.AuthAddress), address)
	require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx), address)
	require.Equal(t, app.MergeUpgrade, a.EvmKeeper.GetCustomPrecompilesVersions(ctx)[address])
	contractABI := layerxauth.GetABI()
	require.Len(t, layerxauth.GasTable, len(contractABI.Methods))
	sender, recipient := utils.NewSigner(), utils.NewSigner()
	for _, signer := range []utils.TestAcct{sender, recipient} {
		a.EvmKeeper.SetAddressMapping(ctx, signer.AccountAddress, signer.EvmAddress)
		funds := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000_000))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funds))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", signer.AccountAddress, funds))
	}
	bind := func(signer utils.TestAcct, seedByte byte) [32]byte {
		t.Helper()
		var seed [ed25519.SeedSize]byte
		seed[0] = seedByte
		private := ed25519.NewKeyFromSeed(seed[:])
		var public [32]byte
		copy(public[:], private.Public().(ed25519.PublicKey))
		message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), signer.EvmAddress, 0)
		require.NoError(t, err)
		var signature [ed25519.SignatureSize]byte
		copy(signature[:], ed25519.Sign(private, message))
		_, err = a.EvmKeeper.BindLayerX(ctx, signer.EvmAddress, public, signature)
		require.NoError(t, err)
		return public
	}
	public := bind(sender, 1)
	capabilityKey := bind(recipient, 2)
	seen := map[string]bool{}
	index := 0
	call := func(signer utils.TestAcct, name string, args ...interface{}) []interface{} {
		t.Helper()
		var values []interface{}
		require.True(t, t.Run(name, func(t *testing.T) {
			method, found := contractABI.Methods[name]
			require.True(t, found, "method must exist in the ABI")
			row, found := layerxauth.GasTable[name]
			require.True(t, found, "method must have a gas row")
			require.Equal(t, !method.IsConstant(), row.Write)
			input, err := contractABI.Pack(name, args...)
			require.NoError(t, err)
			intrinsic, err := core.IntrinsicGas(input, nil, nil, false, true, true, true)
			require.NoError(t, err)
			cost := row.Gas(input[4:])
			require.Equal(t, pcommon.DefaultGasCost(input[4:], row.Write)+row.Surcharge, cost)
			want := intrinsic + cost
			signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
				Nonce:    a.EvmKeeper.GetNonce(ctx, signer.EvmAddress),
				GasPrice: big.NewInt(500_000_000_000), Gas: 2_000_000,
				To: &address, Value: big.NewInt(0), Data: input,
			}), ethtypes.LatestSignerForChainID(a.EvmKeeper.ChainID(ctx)), signer.EvmPrivateKey)
			require.NoError(t, err)
			txData, err := ethtx.NewLegacyTx(signed)
			require.NoError(t, err)
			msg, err := evmtypes.NewMsgEVMTransaction(txData)
			require.NoError(t, err)
			builder := a.GetTxConfig().NewTxBuilder()
			require.NoError(t, builder.SetMsgs(msg))
			builder.SetGasLimit(signed.Gas())
			encoded, err := a.GetTxConfig().TxEncoder()(builder.GetTx())
			require.NoError(t, err)
			tx, err := a.GetTxConfig().TxDecoder()(encoded)
			require.NoError(t, err)
			at := ctx.WithTxIndex(index).WithEventManager(sdk.NewEventManager())
			result := a.DeliverTx(at, abci.RequestDeliverTxV2{Tx: encoded}, tx, sha256.Sum256(encoded))
			require.Zero(t, result.Code, result.Log)
			require.NotNil(t, result.EvmTxInfo)
			require.Empty(t, result.EvmTxInfo.VmError)
			var messages sdk.TxMsgData
			require.NoError(t, messages.Unmarshal(result.Data))
			require.Len(t, messages.Data, 1)
			var response evmtypes.MsgEVMTransactionResponse
			require.NoError(t, response.Unmarshal(messages.Data[0].Data))
			require.Empty(t, response.VmError)
			receipt, err := a.EvmKeeper.GetTransientReceipt(ctx, signed.Hash(), uint64(index))
			require.NoError(t, err)
			require.NotNil(t, receipt)
			require.Empty(t, receipt.VmError)
			require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status, receipt.VmError)
			require.Equal(t, signed.Hash().Hex(), receipt.TxHashHex)
			require.Equal(t, address.Hex(), receipt.To)
			require.Equal(t, want, receipt.GasUsed, "receipt gas must equal intrinsic gas plus the method row")
			require.Equal(t, want, response.GasUsed, "response gas must equal intrinsic gas plus the method row")
			outputs := method.Outputs
			values, err = outputs.Unpack(response.ReturnData)
			require.NoError(t, err)
			seen[name] = true
			index++
		}))
		return values
	}

	actor, err := a.LayerXAuthKeeper.Actor(ctx, sender.EvmAddress)
	require.NoError(t, err)
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	registration := append(append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...), public[:]...)
	registered, err := a.LayerXHost.Execute(lxphost.WithSequence(ctx, 41), lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00070001, ActorDID: actor.DID, AuthorityKey: public,
		NotBefore: timestamp, NotAfter: timestamp, IdempotencyKey: [32]byte{0x41}, Payload: registration,
	}, actor.Authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), registered.ResultCode)
	fixture := &grantFixture{actor: actor, timestamp: timestamp, asset: [32]byte{0x11}, purpose: [32]byte{0x22}}
	hostKey := a.GetKey(app.LayerXHostStoreKey)
	var policyKey, policy [32]byte
	copy(policyKey[:], "native-fee-authority-version")
	policy[31] = 2
	ctx.KVStore(hostKey).Set(append([]byte{1, 0, 7}, policyKey[:]...), policy[:])
	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		fee = binary.BigEndian.AppendUint64(fee, 1)
	}
	fee = append(fee, fixture.asset[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, value)
	}
	fee = append(fee, bytes.Repeat([]byte{0xa5}, 32)...)
	fee = append(fee, make([]byte, 16)...)
	require.Len(t, fee, 217)
	ctx.KVStore(hostKey).Set(append([]byte{1, 0, 9}, []byte("progfee/active/v1")...), fee)
	var sessionKey, successorKey [32]byte
	for i, key := range []*[32]byte{&sessionKey, &successorKey} {
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(74 + i)}, ed25519.SeedSize))
		copy(key[:], private.Public().(ed25519.PublicKey))
	}
	body := fixture.feeSessionBody(sessionKey, 41)
	action := [32]byte{0x33}
	sessionID, err := codec.DomainHash(codec.DomainAuthorityHash, body)
	require.NoError(t, err)
	require.Equal(t, []interface{}{sessionID}, call(sender, "grantSession", body, uint64(100), action))
	capabilityBody := fixture.body(3, capabilityKey)
	capabilityID, err := codec.DomainHash(codec.DomainAuthorityHash, capabilityBody)
	require.NoError(t, err)
	require.Equal(t, []interface{}{capabilityID}, call(sender, "grantCapability", capabilityBody))
	require.Equal(t, capabilityBody, call(sender, "getGrant", capabilityID)[0])
	call(sender, "sessionKey", sessionID)
	call(sender, "capabilityScope", capabilityID)
	next, err := lxphost.NextSequence(ctx, hostKey)
	require.NoError(t, err)
	require.Equal(t, []interface{}{true, ""}, call(sender, "isLive", capabilityID, next-1, timestamp))
	require.Equal(t, []interface{}{""}, call(sender, "checkActivity", capabilityID, uint32(0x00090004), fixture.asset, big.NewInt(5)))
	input, err := contractABI.Pack("grantSession", body, uint64(100), action)
	require.NoError(t, err)
	preimage := append([]byte("layerxauth/grants/v1"), public[:]...)
	idempotency := sha256.Sum256(append(preimage, input...))
	recorded := call(sender, "resultByIdempotency", public, idempotency)[0].([]byte)
	require.Empty(t, recorded)
	call(sender, "revokeGrant", sessionID, uint8(1), next)
	predecessor, err := a.LayerXHost.ReadGrant(ctx, sessionID)
	require.NoError(t, err)
	require.True(t, predecessor.Revoked)
	require.True(t, predecessor.FeeBudget.Present)
	revocation, err := a.LayerXAuthKeeper.RevocationSequence(ctx, public)
	require.NoError(t, err)
	preimage = append([]byte("LXP/session-fee-replacement/v1\x00"), predecessor.GrantID[:]...)
	preimage = binary.BigEndian.AppendUint64(preimage, predecessor.RevokedAtSequence)
	preimage = append(preimage, predecessor.GrantID[:]...)
	for _, amount := range []sdk.Int{predecessor.FeeBudget.SpentTotal, predecessor.FeeBudget.SpentThisPeriod} {
		var encoded [16]byte
		amount.BigInt().FillBytes(encoded[:])
		preimage = append(preimage, encoded[:]...)
	}
	preimage = binary.BigEndian.AppendUint64(preimage, predecessor.FeeBudget.PeriodStart)
	commitment := sha256.Sum256(preimage)
	successorBody := fixture.feeSessionBody(successorKey, revocation)
	successorID, err := codec.DomainHash(codec.DomainAuthorityHash, successorBody)
	require.NoError(t, err)
	require.Equal(t, []interface{}{successorID}, call(sender, "grantSessionSuccessor", successorBody, uint64(120),
		[32]byte{0x34}, sessionID, commitment))
	successor, err := a.LayerXHost.ReadGrant(ctx, successorID)
	require.NoError(t, err)
	require.False(t, successor.Revoked)
	require.Equal(t, predecessor.FeeBudget, successor.FeeBudget)
	require.Len(t, seen, len(contractABI.Methods))
	for name := range contractABI.Methods {
		require.True(t, seen[name], "%s must execute a successful node-delivered gas assertion", name)
	}
}
