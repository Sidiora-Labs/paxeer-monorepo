package layerxbridge_test

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge"
	bridgetestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func repairM1269Documents(t *testing.T) []string {
	t.Helper()
	var docs []string
	for _, path := range []string{"ATTESTATION.md", "../../contracts/bridge/evm/ATTESTATION.md"} {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		docs = append(docs, string(data))
	}
	return docs
}

func TestRepairM_1269_DenomDocumentationAndKeeper(t *testing.T) {
	for _, doc := range repairM1269Documents(t) {
		text := strings.Join(strings.Fields(doc), " ")
		for _, fact := range []string{
			"factory/{bridge module address}/lxb{hex(keccak256(uint64 chainId || asset)[:20])}",
			"factory/{bridge module address}/usid", "Solana", "0x0000534f4c414e41",
			types.SidioraRemoteAddress, "1 SID = 1,000,000 `usid` base units",
			"Governance must register the chain, call `RegisterSidioraPair`, and then set its caps",
			"integer base units, with no rescaling by the bridge",
		} {
			require.Contains(t, text, fact)
		}
	}

	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(8).WithBlockTime(time.Unix(1_800_000_000, 0))
	k, ctx := bridgetestutil.NewKeeper(a, ctx)
	k.InitGenesis(ctx, *types.DefaultGenesis())
	authority := types.DefaultAuthority()
	govern := func(msgs ...sdk.Msg) error {
		proposal, err := types.NewBridgeProposal("Bridge denom registration", "Execute the documented registration order", msgs...)
		require.NoError(t, err)
		wire, err := a.AppCodec().MarshalInterface(proposal)
		require.NoError(t, err)
		var content govtypes.Content
		require.NoError(t, a.AppCodec().UnmarshalInterface(wire, &content))
		require.NoError(t, content.ValidateBasic())
		return layerxbridge.NewProposalHandler(k)(ctx, content)
	}
	canonical := types.Address20(common.HexToAddress(types.SidioraRemoteAddress))
	pair := &types.MsgRegisterSidioraPair{Authority: authority, ChainID: types.SidioraHomeChainID}
	capFor := func(chain uint64, asset types.Address20) *types.MsgSetCap {
		return &types.MsgSetCap{Authority: authority, ChainID: chain, Asset: asset,
			MaxInFlight: sdk.NewInt(2_000_000), MaxPerTx: sdk.NewInt(1_000_000)}
	}
	before := k.ExportGenesis(ctx)
	require.ErrorIs(t, govern(pair), types.ErrUnknownChain)
	require.Equal(t, before, k.ExportGenesis(ctx))
	vault := types.Address20(common.HexToAddress("0x1111111111111111111111111111111111111111"))
	for _, chain := range []uint64{types.SidioraHomeChainID, 1} {
		require.NoError(t, govern(&types.MsgRegisterChain{Authority: authority,
			Chain: types.Chain{ChainID: chain, Vault: vault, FinalityDepth: 64, Enabled: true}}))
	}
	before = k.ExportGenesis(ctx)
	bankBefore := a.BankKeeper.ExportGenesis(ctx)
	factoryBefore := a.TokenFactoryKeeper.ExportGenesis(ctx)
	require.ErrorIs(t, govern(capFor(types.SidioraHomeChainID, canonical)), types.ErrInvalidRequest)
	require.Equal(t, before, k.ExportGenesis(ctx))
	require.Equal(t, bankBefore, a.BankKeeper.ExportGenesis(ctx))
	require.Equal(t, factoryBefore, a.TokenFactoryKeeper.ExportGenesis(ctx))
	require.NoError(t, govern(pair, capFor(types.SidioraHomeChainID, canonical)))

	denom := types.SidioraDenom()
	metadata, found := a.BankKeeper.GetDenomMetaData(ctx, denom)
	require.True(t, found)
	require.NoError(t, metadata.Validate())
	require.Equal(t, denom, metadata.Base)
	require.Equal(t, "SID", metadata.Display)
	require.Equal(t, "SID", metadata.Symbol)
	require.Len(t, metadata.DenomUnits, 2)
	require.Equal(t, denom, metadata.DenomUnits[0].Denom)
	require.Zero(t, metadata.DenomUnits[0].Exponent)
	require.Equal(t, "SID", metadata.DenomUnits[1].Denom)
	require.Equal(t, uint32(6), metadata.DenomUnits[1].Exponent)
	require.Equal(t, int64(1_000_000), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(metadata.DenomUnits[1].Exponent)), nil).Int64())

	attestors := bridgetestutil.Attestors(3)
	require.NoError(t, govern(&types.MsgSetAttestors{Authority: authority, Set: bridgetestutil.Set(attestors, 1_000_000, 2)}))
	otherAsset := types.Address20(common.HexToAddress("0x4444444444444444444444444444444444444444"))
	for _, tc := range []struct {
		name  string
		chain uint64
		asset types.Address20
		denom string
	}{
		{"canonical_sidiora", types.SidioraHomeChainID, canonical, denom},
		{"same_asset_other_chain", 1, canonical, types.Denom(1, canonical)},
		{"same_chain_other_asset", types.SidioraHomeChainID, otherAsset, types.Denom(types.SidioraHomeChainID, otherAsset)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name != "canonical_sidiora" {
				var chainBytes [8]byte
				binary.BigEndian.PutUint64(chainBytes[:], tc.chain)
				hash := crypto.Keccak256(chainBytes[:], tc.asset[:])
				derived := fmt.Sprintf("factory/%s/lxb%s", types.ModuleAddress(), hex.EncodeToString(hash[:20]))
				require.Equal(t, derived, tc.denom)
				require.NotEqual(t, denom, tc.denom)
				require.NoError(t, govern(capFor(tc.chain, tc.asset)))
			}
			require.Equal(t, tc.denom, types.AssetDenom(tc.chain, tc.asset))
			record, found := k.GetAsset(ctx, tc.chain, tc.asset)
			require.True(t, found)
			require.Equal(t, tc.denom, record.Denom)
			reverse, found := k.GetAssetByDenom(ctx, tc.denom)
			require.True(t, found)
			require.Equal(t, record, reverse)
			admin, err := a.TokenFactoryKeeper.GetAuthorityMetadata(ctx, tc.denom)
			require.NoError(t, err)
			require.Equal(t, k.ModuleAddress().String(), admin.Admin)
			recipient := common.HexToAddress("0x00000000000000000000000000000000000a11ce")
			in := types.BridgeIn{ChainID: tc.chain, Vault: vault, TxHash: types.Hash32{0x12}, LogIndex: 7,
				Recipient: types.Hash32(common.BytesToHash(recipient.Bytes())), Asset: tc.asset, Amount: big.NewInt(1_000_000)}
			signatures := bridgetestutil.Sign(types.InboundDigest(in), attestors[0], attestors[1])
			// Separate the two deposits on the Solana chain without changing their units.
			if tc.name == "same_chain_other_asset" {
				in.LogIndex++
				signatures = bridgetestutil.Sign(types.InboundDigest(in), attestors[0], attestors[1])
			}
			minted, err := k.BridgeIn(ctx, in, signatures)
			require.NoError(t, err)
			require.Equal(t, tc.denom, minted.Denom)
			require.Equal(t, sdk.NewInt(1_000_000), minted.Amount)
			require.Equal(t, types.InboundDigest(in), minted.Digest)
			account := a.EvmKeeper.GetPaxAddressOrDefault(ctx, recipient)
			require.Equal(t, sdk.NewInt(1_000_000), a.BankKeeper.GetBalance(ctx, account, tc.denom).Amount)
			require.Equal(t, sdk.NewInt(1_000_000), a.BankKeeper.GetSupply(ctx, tc.denom).Amount)
			require.True(t, k.IsNullified(ctx, in.Nullifier()))
			burned, err := k.BridgeOut(ctx, recipient, tc.chain, tc.asset, big.NewInt(250_000), types.Address20{0x33})
			require.NoError(t, err)
			require.Equal(t, tc.denom, burned.Denom)
			require.Equal(t, sdk.NewInt(750_000), a.BankKeeper.GetBalance(ctx, account, tc.denom).Amount)
			require.Equal(t, sdk.NewInt(750_000), a.BankKeeper.GetSupply(ctx, tc.denom).Amount)
			require.Equal(t, sdk.NewInt(750_000), k.InFlight(ctx, tc.denom))
		})
	}
	require.NoError(t, k.ExportGenesis(ctx).Validate())
	_, found = k.GetAssetByDenom(ctx, types.Denom(types.SidioraHomeChainID, canonical))
	require.False(t, found, "the canonical pair must never create a generic denom")
}

func TestRepairM_1269_DigestDomainParity(t *testing.T) {
	docs := repairM1269Documents(t)
	vault := types.Address20(common.HexToAddress("0x1111111111111111111111111111111111111111"))
	hash := types.Hash32(common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222"))
	asset := types.Address20(common.HexToAddress("0x4444444444444444444444444444444444444444"))
	amount := big.NewInt(1_000_000_000_000_000_000)
	in := types.BridgeIn{ChainID: 1, Vault: vault, TxHash: hash, LogIndex: 7, Asset: asset, Amount: amount,
		Recipient: types.Hash32(common.HexToHash("0x5555555555555555555555555555555555555555555555555555555555555555"))}
	out := types.BridgeOut{ChainID: 1, Vault: vault, PaxeerTxHash: hash, PaxeerNonce: 7, Asset: asset, Amount: amount,
		Recipient: types.Address20(common.HexToAddress("0x3333333333333333333333333333333333333333"))}
	require.Len(t, types.InboundPreimage(in), 196)
	require.Len(t, types.OutboundPreimage(out), 185)
	require.Equal(t, types.DomainIn, string(types.InboundPreimage(in)[:20]))
	require.Equal(t, types.DomainOut, string(types.OutboundPreimage(out)[:21]))
	require.Equal(t, "0x511964ae9566f9536604258667400b0d76335e2a6e60ab0f700bf2433bc97918", common.Hash(types.InboundDigest(in)).Hex())
	require.Equal(t, "0xbd35888e4b158986238ce7abe73957702e2f6e78fe6157197878ebd13edf5b37", common.Hash(types.OutboundDigest(out)).Hex())
	for _, doc := range docs {
		for _, fact := range []string{
			"Domain `" + types.DomainIn + "`. Preimage length 196 bytes.",
			"Domain `" + types.DomainOut + "`. Preimage length 185 bytes.",
			common.Hash(types.InboundDigest(in)).Hex(), common.Hash(types.OutboundDigest(out)).Hex(),
		} {
			require.Contains(t, doc, fact)
		}
	}
	for _, pattern := range []string{`\b0x[0-9a-fA-F]{64}\b`, `(?m)^\|.*\|$`, "\\b(?:Domain|ASCII)\\s+`([^`]+)`"} {
		re := regexp.MustCompile(pattern)
		moduleFields := re.FindAllString(docs[0], -1)
		require.NotEmpty(t, moduleFields)
		require.Equal(t, moduleFields, re.FindAllString(docs[1], -1), "both documents must retain identical signed layouts, domains and vectors")
	}
}
