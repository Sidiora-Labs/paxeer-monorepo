package keeper

import (
	"encoding/hex"
	"math/big"
	"sort"
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type PayExactArgs = types.PayExactArgs

func (k *Keeper) PayExact(ctx sdk.Context, actor Actor, args types.PayExactArgs) (Result, error) {
	if err := k.checkPause(ctx, "payExact"); err != nil {
		return Result{}, err
	}
	now := ctx.BlockTime().UnixMilli()
	if now < 0 {
		return Result{}, types.ErrNonCanonical
	}
	if args.ExpiresAt <= uint64(now) {
		return Result{}, types.ErrExpired
	}
	if !actor.Authority.Bound || actor.DIDKey != actor.Authority.VerifiedKey {
		return Result{}, types.ErrUnauthorizedDebit
	}
	network := config.GetEVMChainID(ctx.ChainID())
	if network.Sign() <= 0 || network.BitLen() > 32 {
		return Result{}, types.ErrNonCanonical
	}
	sequence, err := k.NextSequence(ctx, actor.DIDKey, args.Asset)
	if err != nil {
		return Result{}, err
	}
	send := types.SendArgs{
		From: args.From, To: args.To, Asset: args.Asset, Amount: args.Amount,
		Sequence: sequence, IdempotencyKey: args.PaymentReference, ExpiresAt: args.ExpiresAt,
		PublicKey: actor.Authority.VerifiedKey, NetworkID: uint32(network.Uint64()),
		ProtocolVersion: codec.StateCommitmentProtocolVersion,
	}
	payload, err := types.EncodeSendPayload(send)
	if err != nil {
		return Result{}, err
	}
	material := make([]byte, 0, 144)
	material = append(material, payload[4:116]...)
	material = append(material, args.PaymentReference[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, material)
	if err != nil {
		return Result{}, err
	}
	send.ContextHash = contextHash
	payload, err = types.EncodeSendPayload(send)
	if err != nil {
		return Result{}, err
	}

	branch, write := ctx.CacheContext()
	const activityType uint32 = 0x00010005
	result, err := k.execute(branch, actor, activityType, payload, args.PaymentReference)
	if err != nil {
		return result, err
	}
	if err := k.Mirror(branch, activityType, result.Receipt); err != nil {
		return result, err
	}
	events, err := LedgerEvents(activityType, result.Receipt, EventArgs{
		PayExact: true, PaymentReference: args.PaymentReference, ContextHash: contextHash,
	})
	if err != nil {
		return result, err
	}
	emitted := make(sdk.Events, 0, len(events))
	for _, event := range events {
		if event.Name == "PaymentExact" {
			event.Fields["contextHash"] = contextHash
		}
		keys := make([]string, 0, len(event.Fields))
		for key := range event.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		attributes := make([]sdk.Attribute, 0, len(keys))
		for _, key := range keys {
			var encoded string
			switch value := event.Fields[key].(type) {
			case [32]byte:
				encoded = hex.EncodeToString(value[:])
			case *big.Int:
				encoded = value.String()
			case uint8:
				encoded = strconv.FormatUint(uint64(value), 10)
			default:
				return result, types.ErrNonCanonical
			}
			attributes = append(attributes, sdk.NewAttribute(key, encoded))
		}
		emitted = append(emitted, sdk.NewEvent(event.Name, attributes...))
	}
	write()
	ctx.EventManager().EmitEvents(branch.EventManager().Events())
	ctx.EventManager().EmitEvents(emitted)
	return result, nil
}
