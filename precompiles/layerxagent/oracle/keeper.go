package oracle

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// Keeper uses the live host state, independently of legacy intent retirement.
// The activity audit stays in the exchange store; prices stay in the host store.
type Keeper struct {
	host              *lxphost.Host
	hostStoreName     string
	activityStoreName string
}

func NewKeeper(host *lxphost.Host, hostStoreName, activityStoreName string) (*Keeper, error) {
	if host == nil || hostStoreName == "" || activityStoreName == "" {
		return nil, fmt.Errorf("%w: committed oracle stores are unavailable", agentcommon.ErrNoContext)
	}
	return &Keeper{host, hostStoreName, activityStoreName}, nil
}

func oracleStoreKey(ctx sdk.Context, name string) (sdk.StoreKey, error) {
	if ctx.MultiStore() != nil {
		for _, key := range ctx.MultiStore().StoreKeys() {
			if key.Name() == name {
				return key, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: oracle store %s is not mounted", agentcommon.ErrNoContext, name)
}

func (k *Keeper) SubmitOracleObservation(ctx sdk.Context, activity lxphost.Activity, feed [32]byte, price sdk.Int, confidence uint64) error {
	if feed == ([32]byte{}) || price.IsNil() || !price.IsPositive() || price.BigInt().BitLen() > 128 || activity.Type != oraclePushActivity || len(activity.Payload) != 137 || activity.Payload[0] != 1 || !bytes.Equal(activity.Payload[1:33], feed[:]) || new(big.Int).SetBytes(activity.Payload[41:57]).Cmp(price.BigInt()) != 0 {
		return fmt.Errorf("%w: signed oracle observation does not match submission", agentcommon.ErrABIDecode)
	}
	if !bytes.Equal(activity.ActorDID, oracleDID(activity.AuthorityKey)) {
		return fmt.Errorf("%w: oracle actor does not match authority", agentcommon.ErrOracleNotSigner)
	}
	hostKey, err := oracleStoreKey(ctx, k.hostStoreName)
	if err != nil {
		return err
	}
	auditKey, err := oracleStoreKey(ctx, k.activityStoreName)
	if err != nil {
		return err
	}
	id, err := lxphost.DIDID(activity.ActorDID)
	if err != nil {
		return err
	}
	branch, commit := ctx.CacheContext()
	sequence, err := lxphost.NextSequence(branch, hostKey)
	if err != nil {
		return err
	}
	if sequence == 0 {
		return fmt.Errorf("%w: oracle host sequence exhausted", agentcommon.ErrOverflow)
	}
	authority := lxphost.KernelAuthority{Actor: id, Principal: id, VerifiedKey: activity.AuthorityKey}
	receipt, err := k.host.Execute(lxphost.WithSequence(branch, sequence), activity, authority)
	if err != nil {
		return err
	}
	if receipt.ResultCode != 0 {
		return receipt.ResultCode
	}
	digest := sha256.Sum256(activity.AuthorityKey[:])
	record, err := json.Marshal(struct {
		Validator  sdk.ValAddress
		Confidence uint64
		Activity   lxphost.Activity
		Receipt    lxphost.Receipt
	}{sdk.ValAddress(digest[:20]), confidence, activity, receipt})
	if err != nil {
		return err
	}
	branch.KVStore(auditKey).Set(append([]byte("oracle/activity/"), activity.IdempotencyKey[:]...), record)
	commit()
	return nil
}
