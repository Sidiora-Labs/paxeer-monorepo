package layerxledger_test

import (
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
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestLedgerGasConformance(t *testing.T) {
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithChainID("pax-test").WithBlockHeight(100).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	a.RefreshPrecompiles(ctx)
	address := common.HexToAddress("0x101A")
	require.Equal(t, common.HexToAddress(layerxledger.LedgerAddress), address)
	require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx), address)
	require.Equal(t, app.MergeUpgrade, a.EvmKeeper.GetCustomPrecompilesVersions(ctx)[address])
	contractABI := layerxledger.GetABI()
	require.Len(t, layerxledger.GasTable, len(contractABI.Methods))
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
	bind(recipient, 2)
	seen := map[string]bool{}
	index := 0
	call := func(signer utils.TestAcct, name string, args ...interface{}) []interface{} {
		t.Helper()
		var values []interface{}
		require.True(t, t.Run(name, func(t *testing.T) {
			method, found := contractABI.Methods[name]
			require.True(t, found, "method must exist in the ABI")
			row, found := layerxledger.GasTable[name]
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
			switch name {
			case "register", "openAccount", "issueGrant":
				outputs = method.Inputs[:1]
				if name == "issueGrant" {
					outputs = contractABI.Methods["getPayerGrant"].Inputs
				}
			}
			values, err = outputs.Unpack(response.ReturnData)
			require.NoError(t, err)
			seen[name] = true
			index++
		}))
		return values
	}

	payer, err := a.LayerXLedgerKeeper.Actor(ctx, sender.EvmAddress)
	require.NoError(t, err)
	payee, err := a.LayerXLedgerKeeper.Actor(ctx, recipient.EvmAddress)
	require.NoError(t, err)
	require.Equal(t, public, payer.DIDKey)
	salt := [32]byte{0x71}
	material := append([]byte("LX:ASSET:v1"), payer.Authority.Actor[:]...)
	asset := sha256.Sum256(append(material, salt[:]...))
	require.Equal(t, []interface{}{asset}, call(sender, "register", asset, salt, "GAS", "Gas conformance", uint8(6),
		big.NewInt(1000), uint8(types.AssetIssuerNative), []byte{}))
	call(sender, "pause", asset)
	call(sender, "unpause", asset)
	from := call(sender, "openAccount", asset)[0].([32]byte)
	expectedFrom, err := ledgerkeeper.AccountID(payer.DIDKey, asset)
	require.NoError(t, err)
	require.Equal(t, expectedFrom, from)
	to := call(recipient, "openAccount", asset)[0].([32]byte)
	call(sender, "mint", asset, from, big.NewInt(251))
	call(sender, "burn", asset, from, big.NewInt(1))
	main := transferMainAccount(t, &harness{
		ctx: ctx, host: a.LayerXHost, hostKey: a.GetKey(app.LayerXHostStoreKey),
	}, payer, asset)
	now := uint64(ctx.BlockTime().UnixMilli())
	call(sender, "payExact", from, main, asset, big.NewInt(100), now+1000, [32]byte{0x72})
	sendEnvelope := func(from, to, asset [32]byte, amount int64, sequence uint64, key [32]byte) []byte {
		t.Helper()
		material := append(append(append([]byte(nil), from[:]...), to[:]...), asset[:]...)
		var encoded [16]byte
		big.NewInt(amount).FillBytes(encoded[:])
		material = append(material, encoded[:]...)
		contextHash, err := codec.DomainHash(codec.DomainContextHash, append(material, key[:]...))
		require.NoError(t, err)
		payload, err := types.EncodeSendPayload(types.SendArgs{
			From: from, To: to, Asset: asset, Amount: sdk.NewInt(amount), Sequence: sequence,
			IdempotencyKey: key, ExpiresAt: uint64(ctx.BlockTime().UnixMilli()) + 1000,
			ContextHash: contextHash, PublicKey: payer.DIDKey,
			NetworkID: uint32(a.EvmKeeper.ChainID(ctx).Uint64()), ProtocolVersion: codec.StateCommitmentProtocolVersion,
		})
		require.NoError(t, err)
		var seed [ed25519.SeedSize]byte
		seed[0] = 1
		private := ed25519.NewKeyFromSeed(seed[:])
		authorization := len(payload) - 167
		message := append(append([]byte(nil), payload[:2]...), payload[4:authorization]...)
		message = append(message, payload[authorization:authorization+33]...)
		message = append(message, payload[authorization+129:]...)
		digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
		require.NoError(t, err)
		copy(payload[authorization+65:authorization+129], ed25519.Sign(private, digest[:]))
		require.NoError(t, types.ValidateSendEnvelope(payload))
		return payload
	}
	payload := sendEnvelope(main, to, asset, 40, 0, [32]byte{0x73})
	call(sender, "send", payload)
	purpose := [32]byte{0x74}
	contextHash, err := codec.DomainHash(codec.DomainContextHash, purpose[:])
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
	var seed [ed25519.SeedSize]byte
	seed[0] = 1
	private := ed25519.NewKeyFromSeed(seed[:])
	grant := append(append([]byte(nil), grantID[:]...), body...)
	grant = append(grant, ed25519.Sign(private, grantID[:])...)
	require.NoError(t, types.ValidateIssueGrantPayload(grant))
	require.Equal(t, []interface{}{grantID}, call(sender, "issueGrant", grant))
	sequence := call(recipient, "nextSequence", payee.DIDKey, asset)[0].(uint64)
	call(recipient, "draw", grantID, to, big.NewInt(15), [32]byte{0x75}, sequence, purpose, contextHash)
	revokeSequence, err := a.LayerXHost.ReadIdentitySequence(ctx, payer.DID)
	require.NoError(t, err)
	call(sender, "revokeGrant", grantID, revokeSequence)
	call(sender, "getPayerGrant", grantID)
	call(sender, "getAsset", asset)
	require.Zero(t, call(sender, "balanceOf", from, asset)[0].(*big.Int).Cmp(big.NewInt(135)))
	require.Zero(t, call(sender, "totalUnits", asset)[0].(*big.Int).Cmp(big.NewInt(250)))
	require.Equal(t, []interface{}{from}, call(sender, "accountId", payer.DIDKey, asset))
	require.Equal(t, []interface{}{payer.Authority.Actor}, call(sender, "accountOwner", from))
	require.Len(t, seen, len(contractABI.Methods))
	for name := range contractABI.Methods {
		require.True(t, seen[name], "%s must execute a successful node-delivered gas assertion", name)
	}
}
