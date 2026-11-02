//go:build cgo

package lxphost

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func ExecuteActivity(ctx sdk.Context, host *Host, authority KernelAuthority, activityType uint32, payload []byte, sequence uint64) (Receipt, error) {
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	idempotency := binary.BigEndian.AppendUint32(nil, activityType)
	idempotency = binary.BigEndian.AppendUint64(idempotency, sequence)
	idempotency = append(idempotency, payload...)
	activity := Activity{
		ProtocolVersion: codec.StateCommitmentProtocolVersion,
		Type:            activityType,
		ActorDID:        []byte("did:layerx:" + hex.EncodeToString(authority.VerifiedKey[:])),
		AuthorityKey:    authority.VerifiedKey,
		AccountSequence: sequence,
		NotBefore:       timestamp,
		NotAfter:        timestamp,
		IdempotencyKey:  sha256.Sum256(idempotency),
		Payload:         payload,
	}
	return host.Execute(ctx, activity, authority)
}

func ResultError(code Code) error {
	if code == 0 {
		return nil
	}
	return errors.New(strings.Replace(code.Error(), "LXP_ERR_", "LXP_", 1))
}
