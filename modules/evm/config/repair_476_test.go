package config_test

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	evmante "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	bridgetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/stretchr/testify/require"
)

func TestRepair476_DefaultAndMappings(t *testing.T) {
	require.Equal(t, int64(125), config.DefaultChainID)
	require.Equal(t, map[string]int64{"hyperpax_125-1": 125}, config.ChainIDMapping)
	require.Equal(t, map[int64]string{125: "hyperpax_125-1"}, config.EVMChainIDMapping)
	require.True(t, config.IsLiveChainID(sdk.Context{}.WithChainID("hyperpax_125-1")))
	require.True(t, config.IsLiveEVMChainID(125))
	for _, foreign := range []string{"pacific-1", "atlantic-2", "arctic-1"} {
		require.False(t, config.IsLiveChainID(sdk.Context{}.WithChainID(foreign)))
	}
	for _, foreign := range []int64{1329, 1328, 713715} {
		require.False(t, config.IsLiveEVMChainID(foreign))
	}
	for _, cosmosID := range []string{"", "unmapped-paxeer-test", "hyperpax_125-1", "pacific-1", "atlantic-2", "arctic-1"} {
		t.Run(cosmosID, func(t *testing.T) {
			require.Equal(t, big.NewInt(125), config.GetEVMChainID(cosmosID))
		})
	}
}

func TestRepair476_ProductionSigner(t *testing.T) {
	// Published Anvil key, used only for local signatures.
	key, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	require.NoError(t, err)
	to := common.HexToAddress("0x0000000000000000000000000000000000001004")
	for _, cosmosID := range []string{"unmapped-paxeer-test", "hyperpax_125-1"} {
		t.Run(cosmosID, func(t *testing.T) {
			ctx := sdk.Context{}.WithChainID(cosmosID).WithBlockHeight(1).WithBlockTime(time.Unix(1_800_000_000, 0))
			chainID := config.GetEVMChainID(ctx.ChainID())
			require.Equal(t, big.NewInt(125), chainID)
			ethCfg := evmtypes.DefaultChainConfig().EthereumConfig(chainID)
			signer := ethtypes.MakeSigner(ethCfg, big.NewInt(ctx.BlockHeight()), uint64(ctx.BlockTime().Unix()))
			for name, data := range map[string]ethtypes.TxData{
				"legacy":      &ethtypes.LegacyTx{To: &to, Gas: 21000, GasPrice: big.NewInt(1), Value: big.NewInt(1)},
				"access-list": &ethtypes.AccessListTx{ChainID: chainID, To: &to, Gas: 21000, GasPrice: big.NewInt(1), Value: big.NewInt(1)},
				"dynamic-fee": &ethtypes.DynamicFeeTx{ChainID: chainID, To: &to, Gas: 21000, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2), Value: big.NewInt(1)},
			} {
				t.Run(name, func(t *testing.T) {
					signed, err := ethtypes.SignNewTx(key, signer, data)
					require.NoError(t, err)
					wire, err := signed.MarshalBinary()
					require.NoError(t, err)
					var received ethtypes.Transaction
					require.NoError(t, received.UnmarshalBinary(wire))
					sender, err := ethtypes.Sender(signer, &received)
					require.NoError(t, err)
					require.Equal(t, crypto.PubkeyToAddress(key.PublicKey), sender)
					evmAddr, paxAddr, publicKey, err := evmante.RecoverSenderFromEthTx(ctx, &received, chainID)
					require.NoError(t, err)
					require.Equal(t, sender, evmAddr)
					require.Equal(t, publicKey.Address().Bytes(), paxAddr.Bytes())
					for _, foreignID := range []int64{713714, 713715, 1328, 1329} {
						_, _, _, err := evmante.RecoverSenderFromEthTx(ctx, &received, big.NewInt(foreignID))
						require.ErrorIs(t, err, ethtypes.ErrInvalidChainId)
						foreignSigner := ethtypes.NewLondonSigner(big.NewInt(foreignID))
						foreignTx, err := ethtypes.SignNewTx(key, foreignSigner, &ethtypes.DynamicFeeTx{ChainID: big.NewInt(foreignID), To: &to, Gas: 21000, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2)})
						require.NoError(t, err)
						_, _, _, err = evmante.RecoverSenderFromEthTx(ctx, foreignTx, chainID)
						require.ErrorIs(t, err, ethtypes.ErrInvalidChainId)
					}
				})
			}
		})
	}
}

type repair476Account struct {
	Index           int    `json:"index"`
	EVMAddress      string `json:"evm_address"`
	LayerXPublicKey string `json:"layerx_public_key"`
	DID             string `json:"did"`
	Resolution      struct {
		ChainID int64  `json:"chainId"`
		Data    string `json:"data"`
		Result  string `json:"result"`
	} `json:"resolution"`
	Bind struct {
		ChainID     int64  `json:"chain_id"`
		Nonce       uint64 `json:"nonce"`
		To          string `json:"to"`
		Calldata    string `json:"calldata"`
		Transaction struct {
			Raw string `json:"raw"`
		} `json:"transaction"`
	} `json:"bind"`
}

func TestRepair476_RegeneratedFixture(t *testing.T) {
	raw, err := os.ReadFile("../../../sdk/platform/conformance/fixtures/account-derivation-v1.json")
	require.NoError(t, err)
	var fixture struct {
		ChainID   int64 `json:"chain_id"`
		Mnemonics []struct {
			Accounts []repair476Account `json:"accounts"`
		} `json:"mnemonic_vectors"`
		Wallet struct {
			Vectors []struct {
				repair476Account
				TypedData  apitypes.TypedData `json:"typed_data"`
				Hash       string             `json:"eip712_hash"`
				Signature  string             `json:"signature"`
				Equivalent []string           `json:"equivalent_signatures"`
			} `json:"vectors"`
		} `json:"wallet_signature"`
	}
	require.NoError(t, json.Unmarshal(raw, &fixture))
	require.Equal(t, int64(125), fixture.ChainID)
	require.Len(t, fixture.Mnemonics, 2)
	checkResolution := func(account repair476Account) {
		t.Helper()
		require.Equal(t, int64(125), account.Resolution.ChainID)
		require.Equal(t, "did:layerx:"+account.LayerXPublicKey, account.DID)
		require.Equal(t, "0xc413bad3"+account.LayerXPublicKey, account.Resolution.Data)
		require.Equal(t, "0x"+hex.EncodeToString(common.LeftPadBytes(common.HexToAddress(account.EVMAddress).Bytes(), 32)), account.Resolution.Result)
	}
	for _, mnemonic := range fixture.Mnemonics {
		require.Len(t, mnemonic.Accounts, 2)
		for _, account := range mnemonic.Accounts {
			checkResolution(account)
			require.Equal(t, int64(125), account.Bind.ChainID)
			var tx ethtypes.Transaction
			require.NoError(t, tx.UnmarshalBinary(common.FromHex(account.Bind.Transaction.Raw)))
			require.Equal(t, big.NewInt(125), tx.ChainId())
			require.Equal(t, common.HexToAddress(account.Bind.To), *tx.To())
			require.Equal(t, account.Bind.Calldata, hex.EncodeToString(tx.Data()))
			ctx := sdk.Context{}.WithBlockHeight(1).WithBlockTime(time.Unix(1_800_000_000, 0))
			sender, _, _, err := evmante.RecoverSenderFromEthTx(ctx, &tx, config.GetEVMChainID("fixture-test"))
			require.NoError(t, err)
			require.Equal(t, common.HexToAddress(account.EVMAddress), sender)
			message, err := evmtypes.LayerXBindMessage(tx.ChainId(), sender, account.Bind.Nonce)
			require.NoError(t, err)
			calldata := tx.Data()
			require.Len(t, calldata, 4+32+32+32+64)
			require.Equal(t, "dd9aa628", hex.EncodeToString(calldata[:4]))
			require.Equal(t, account.LayerXPublicKey, hex.EncodeToString(calldata[4:36]))
			require.True(t, ed25519.Verify(common.FromHex(account.LayerXPublicKey), message, calldata[100:]))
			wrongChain, err := evmtypes.LayerXBindMessage(big.NewInt(126), sender, account.Bind.Nonce)
			require.NoError(t, err)
			require.False(t, ed25519.Verify(common.FromHex(account.LayerXPublicKey), wrongChain, calldata[100:]))
		}
	}
	require.Len(t, fixture.Wallet.Vectors, 2)
	for _, vector := range fixture.Wallet.Vectors {
		checkResolution(vector.repair476Account)
		require.Equal(t, int64(125), (*big.Int)(vector.TypedData.Domain.ChainId).Int64())
		digest, _, err := apitypes.TypedDataAndHash(vector.TypedData)
		require.NoError(t, err)
		require.Equal(t, vector.Hash, hex.EncodeToString(digest))
		signature := common.FromHex(vector.Signature)
		require.Len(t, signature, 65)
		canonical := append([]byte(nil), signature...)
		signature[64] -= 27
		pubkey, err := crypto.SigToPub(digest, signature)
		require.NoError(t, err)
		require.Equal(t, common.HexToAddress(vector.EVMAddress), crypto.PubkeyToAddress(*pubkey))
		info := append([]byte("LX:ACCOUNT-KEY:v1"), common.LeftPadBytes(big.NewInt(125).Bytes(), 32)...)
		info = append(info, common.HexToAddress(vector.EVMAddress).Bytes()...)
		info = binary.BigEndian.AppendUint32(info, uint32(vector.Index))
		extract := hmac.New(sha256.New, []byte("paxeer-x-network/layerx-account-key/v1"))
		_, err = extract.Write(canonical)
		require.NoError(t, err)
		expand := hmac.New(sha256.New, extract.Sum(nil))
		_, err = expand.Write(append(info, 1))
		require.NoError(t, err)
		key := ed25519.NewKeyFromSeed(expand.Sum(nil))
		require.Equal(t, vector.LayerXPublicKey, hex.EncodeToString(key.Public().(ed25519.PublicKey)))
		require.Len(t, vector.Equivalent, 2)
		for _, encoded := range vector.Equivalent {
			equivalent := common.FromHex(encoded)
			require.Len(t, equivalent, 65)
			parity := equivalent[64]
			if parity >= 27 {
				parity -= 27
			}
			s := new(big.Int).SetBytes(equivalent[32:64])
			if s.Cmp(new(big.Int).Rsh(new(big.Int).Set(crypto.S256().Params().N), 1)) > 0 {
				s.Sub(crypto.S256().Params().N, s)
				parity ^= 1
			}
			s.FillBytes(equivalent[32:64])
			equivalent[64] = parity + 27
			require.Equal(t, canonical, equivalent)
		}
	}
}

func TestRepair476_AttestationDocumentation(t *testing.T) {
	var corpus struct {
		Vectors []struct {
			Origin   uint8  `json:"origin"`
			Preimage string `json:"preimage"`
			Signer   string `json:"signer"`
		} `json:"vectors"`
	}
	raw, err := os.ReadFile("../../xweb/types/testdata/preimage-vectors.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &corpus))
	doc, err := os.ReadFile("../../xweb/ATTESTATION.md")
	require.NoError(t, err)
	require.NotContains(t, string(doc), "713714")
	for _, vector := range corpus.Vectors {
		if vector.Origin != xwebtypes.OriginEVM {
			continue
		}
		preimage := common.FromHex(vector.Preimage)
		require.Len(t, preimage, xwebtypes.PreimageLength)
		big.NewInt(125).FillBytes(preimage[15:47])
		digest := crypto.Keccak256Hash(preimage)
		start := strings.Index(string(doc), "- digest `"+digest.Hex()+"`")
		require.GreaterOrEqual(t, start, 0)
		section := string(doc)[start:]
		signatureStart := strings.Index(section, "signature\n  `")
		require.GreaterOrEqual(t, signatureStart, 0)
		signature := strings.SplitN(section[signatureStart+len("signature\n  `"):], "`", 2)[0]
		signer, err := bridgetypes.RecoverSigner(bridgetypes.Hash32(digest), common.FromHex(signature))
		require.NoError(t, err)
		require.Contains(t, section[:signatureStart], "- signer `"+strings.ToLower(common.Address(signer).Hex())+"`,")
	}
}
