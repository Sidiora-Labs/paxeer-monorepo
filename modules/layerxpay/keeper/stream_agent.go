package keeper

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/ethereum/go-ethereum/common"
)

type AgentStream struct {
	ID            *big.Int
	Payer         common.Address
	Payee         common.Address
	Token         common.Address
	RatePerSecond *big.Int
	Cap           *big.Int
	StartTime     uint64
	StopTime      uint64
	Settled       *big.Int
	Active        bool
}

const agentStreamMetadataPrefix = types.AgentStreamMetadataPrefix

func agentStreamMetadataKey(id [32]byte) []byte {
	return append([]byte(agentStreamMetadataPrefix), id[:]...)
}

func agentStreamID(id *big.Int) ([32]byte, error) {
	var key [32]byte
	if id == nil || id.Sign() <= 0 || id.BitLen() > 256 {
		return key, fmt.Errorf("LXP_NON_CANONICAL: agent stream id")
	}
	id.FillBytes(key[:])
	return key, nil
}

func agentStreamAmount(value *big.Int) (codec.U128, error) {
	if value == nil || value.Sign() <= 0 {
		return codec.U128{}, fmt.Errorf("LXP_NON_CANONICAL: agent stream amount")
	}
	if value.BitLen() > 128 {
		return codec.U128{}, agentcommon.ErrOverflow
	}
	var b [16]byte
	value.FillBytes(b[:])
	return codec.U128{Hi: binary.BigEndian.Uint64(b[:8]), Lo: binary.BigEndian.Uint64(b[8:])}, nil
}

func agentStreamBigInt(value codec.U128) *big.Int {
	if value.IsZero() {
		return new(big.Int)
	}
	b := value.Bytes()
	return new(big.Int).SetBytes(b[:])
}

// agentStreamNativeAssetID follows lxp_genesis_custody_asset_id's PAX entry.
func agentStreamNativeAssetID() [32]byte {
	return sha256.Sum256([]byte("layerx-asset:125:PAX"))
}

func agentStreamDenomAsset(denom, baseDenom string) ([32]byte, bool) {
	if denom == baseDenom && baseDenom != "" {
		return agentStreamNativeAssetID(), true
	}
	if asset, err := ledgertypes.ParseMirrorDenom(denom); err == nil {
		return asset, asset != ([32]byte{})
	}
	// Ledger issuer denoms encode the asset ID as a canonical base-62 subdenom.
	creator, subdenom, err := tokenfactorytypes.DeconstructDenom(denom)
	if err != nil || creator != authtypes.NewModuleAddress(ledgertypes.ModuleName).String() {
		return [32]byte{}, false
	}
	value, ok := new(big.Int).SetString(subdenom, 62)
	if !ok || value.Sign() <= 0 || value.BitLen() > 256 || fmt.Sprintf("%043s", value.Text(62)) != subdenom {
		return [32]byte{}, false
	}
	var asset [32]byte
	value.FillBytes(asset[:])
	return asset, true
}

func (k Keeper) agentStreamAsset(ctx sdk.Context, token common.Address) ([32]byte, error) {
	var mapping custodytypes.AssetMapping
	var found bool
	var denom string
	baseDenom, baseErr := sdk.GetBaseDenom()
	if token == (common.Address{}) {
		if baseErr != nil {
			return [32]byte{}, agentcommon.ErrStreamsUnknownToken
		}
		denom = baseDenom
		if k.custody != nil {
			mapping, found = k.custody.GetAssetByDenom(ctx, denom)
		}
	} else {
		if k.custody != nil {
			mapping, found = k.custody.GetAssetByPointer(ctx, token)
		}
		if !found {
			if reader, ok := k.evm.(interface {
				GetNativePointee(sdk.Context, string) (string, uint16, bool)
			}); ok {
				pointee, _, exists := reader.GetNativePointee(ctx, token.Hex())
				if exists {
					denom = pointee
					if k.custody != nil && strings.HasPrefix(denom, "factory/") {
						mapping, found = k.custody.GetAssetByDenom(ctx, denom)
					}
				}
			}
		}
	}
	if found {
		asset, err := custodytypes.ParseNonZeroHash32(mapping.AssetId)
		if err != nil {
			return [32]byte{}, fmt.Errorf("%w: invalid custody asset id", agentcommon.ErrStreamsUnknownToken)
		}
		return asset, nil
	}
	asset, known := agentStreamDenomAsset(denom, baseDenom)
	if !known || k.host == nil {
		return [32]byte{}, agentcommon.ErrStreamsUnknownToken
	}
	record, err := k.host.ReadAsset(ctx, asset)
	if errors.Is(err, lxphost.Code(-402)) {
		return [32]byte{}, agentcommon.ErrStreamsUnknownToken
	}
	if err != nil {
		return [32]byte{}, err
	}
	return record.AssetID, nil
}

func (k Keeper) OpenAgentStream(ctx sdk.Context, payer, payee, token common.Address, ratePerSecond *big.Int, startTime, stopTime uint64, cap *big.Int) (*big.Int, error) {
	if payer == payee {
		return nil, agentcommon.ErrStreamsSelfPayment
	}
	payerAuthority, err := k.AuthorityFor(ctx, payer)
	if err != nil {
		return nil, err
	}
	payeeAuthority, err := k.AuthorityFor(ctx, payee)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", agentcommon.ErrStreamsInvalidPayee, err)
	}
	_, payerAccount, err := lxphost.MainAccountID(payerAuthority.AuthorityKey)
	if err != nil {
		return nil, err
	}
	_, payeeAccount, err := lxphost.MainAccountID(payeeAuthority.AuthorityKey)
	if err != nil {
		return nil, err
	}
	if payerAccount == payeeAccount {
		return nil, agentcommon.ErrStreamsSelfPayment
	}
	asset, err := k.agentStreamAsset(ctx, token)
	if err != nil {
		return nil, err
	}
	rate, err := agentStreamAmount(ratePerSecond)
	if err != nil {
		return nil, err
	}
	funding, err := agentStreamAmount(cap)
	if err != nil {
		return nil, err
	}
	if startTime > math.MaxUint64/1000 || stopTime > math.MaxUint64/1000 {
		return nil, agentcommon.ErrOverflow
	}
	metadataCount := 0
	iterator := sdk.KVStorePrefixIterator(ctx.KVStore(k.storeKey), []byte(agentStreamMetadataPrefix))
	for ; iterator.Valid() && metadataCount < types.MaxKernelRecords; iterator.Next() {
		metadataCount++
	}
	iterator.Close()
	if metadataCount == types.MaxKernelRecords {
		return nil, fmt.Errorf("LXP_LENGTH_LIMIT: agent stream metadata count: %w", lxphost.Code(-5))
	}
	branch, commit := ctx.CacheContext()
	txHash := [32]byte(ctx.EVMTxHash())
	ordinal, err := nextPayCallOrdinal(branch, txHash)
	if err != nil {
		return nil, err
	}
	id := types.DeriveStreamID(txHash, ordinal, payerAccount)
	streamAccount, err := codec.DeriveAccountID([]byte("agent:" + string(payerAuthority.ActorDID) + ":stream:" + hex.EncodeToString(id[:])))
	if err != nil {
		return nil, err
	}
	_, err = k.StreamOpen(branch, payer, types.StreamOpen{
		StreamID: id, StreamAccount: streamAccount, Recipient: payeeAccount, AssetID: asset,
		Mode: types.StreamModeTime, Rate: rate, RateUnit: 1000,
		StartTimestamp: startTime * 1000, EndTimestamp: stopTime * 1000,
		TotalCap: funding, InitialFunding: funding,
	})
	if err != nil {
		return nil, err
	}
	metadata := append(append(append([]byte(nil), payer.Bytes()...), payee.Bytes()...), token.Bytes()...)
	branch.KVStore(k.storeKey).Set(agentStreamMetadataKey(id), metadata)
	commit()
	return new(big.Int).SetBytes(id[:]), nil
}

func (k Keeper) SettleAgentStream(ctx sdk.Context, caller common.Address, streamID *big.Int) (*big.Int, error) {
	id, err := agentStreamID(streamID)
	if err != nil {
		return nil, err
	}
	branch, commit := ctx.CacheContext()
	key, err := nextPayIdempotencyKey(branch, [4]byte{0x8d, 0xf8, 0x28, 0x00})
	if err != nil {
		return nil, err
	}
	if _, err = k.StreamSettle(branch, caller, id, key); err != nil {
		return nil, err
	}
	result, err := k.GetStreamResult(branch, key)
	if err != nil {
		return nil, err
	}
	commit()
	return agentStreamBigInt(result.Paid), nil
}

func (k Keeper) CloseAgentStream(ctx sdk.Context, caller common.Address, streamID *big.Int) (finalPaid, refund *big.Int, err error) {
	id, err := agentStreamID(streamID)
	if err != nil {
		return nil, nil, err
	}
	branch, commit := ctx.CacheContext()
	key, err := nextPayIdempotencyKey(branch, [4]byte{0x0a, 0xeb, 0xeb, 0x4e})
	if err != nil {
		return nil, nil, err
	}
	if _, err = k.StreamClose(branch, caller, id, key); err != nil {
		return nil, nil, err
	}
	result, err := k.GetStreamResult(branch, key)
	if err != nil {
		return nil, nil, err
	}
	commit()
	return agentStreamBigInt(result.Paid), agentStreamBigInt(result.Refunded), nil
}

func (k Keeper) UpdateAgentStreamRate(ctx sdk.Context, caller common.Address, streamID *big.Int, newRate *big.Int) (*big.Int, error) {
	id, err := agentStreamID(streamID)
	if err != nil {
		return nil, err
	}
	rate, err := agentStreamAmount(newRate)
	if err != nil {
		return nil, err
	}
	record, err := k.GetStream(ctx, id)
	if err != nil {
		return nil, err
	}
	branch, commit := ctx.CacheContext()
	key, err := nextPayIdempotencyKey(branch, [4]byte{0x40, 0x5a, 0xbb, 0x41})
	if err != nil {
		return nil, err
	}
	if _, err = k.StreamRateChange(branch, caller, id, rate, key); err != nil {
		return nil, err
	}
	commit()
	return agentStreamBigInt(record.Rate), nil
}

func (k Keeper) AgentStreamView(ctx sdk.Context, streamID *big.Int) (AgentStream, bool, error) {
	id, err := agentStreamID(streamID)
	if err != nil {
		return AgentStream{}, false, err
	}
	record, err := k.GetStream(ctx, id)
	if errors.Is(err, lxphost.Code(-7)) {
		return AgentStream{}, false, nil
	}
	if err != nil {
		return AgentStream{}, false, err
	}
	metadata := ctx.KVStore(k.storeKey).Get(agentStreamMetadataKey(id))
	if len(metadata) != 60 {
		return AgentStream{}, false, fmt.Errorf("LXP_NON_CANONICAL: agent stream address metadata")
	}
	return AgentStream{
		ID:    new(big.Int).SetBytes(record.StreamID[:]),
		Payer: common.BytesToAddress(metadata[:20]), Payee: common.BytesToAddress(metadata[20:40]), Token: common.BytesToAddress(metadata[40:]),
		RatePerSecond: agentStreamBigInt(record.Rate), Cap: agentStreamBigInt(record.TotalCap),
		StartTime: record.StartTimestamp / 1000, StopTime: record.EndTimestamp / 1000,
		Settled: agentStreamBigInt(record.SettledTotal), Active: !record.Closed && !record.Paused,
	}, true, nil
}
