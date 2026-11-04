package keeper

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
)

const serviceIndexVersionKey = "service/index-version"
const serviceLatestPrefix = "service/latest-delivery/v1/"
const serviceProgressPrefix = "service/progress-high-water/v1/"

func serviceIndexKey(prefix string, id [32]byte) []byte { return append([]byte(prefix), id[:]...) }

// Service indexes are derived pay-store state, never kernel records. Recovery
// validates canonical records, discards old indexes and rebuilds before commit.
func validateServiceRecords(records []kv.Pair) error {
	values := make(map[string][]byte)
	for _, record := range records {
		if bytes.HasPrefix(record.Key, types.ServiceKVPrefix) {
			values[string(record.Key)] = record.Value
		}
	}
	get := func(key []byte) []byte { return values[string(key)] }
	for _, record := range records {
		if !bytes.HasPrefix(record.Key, types.ServiceKVPrefix) {
			continue
		}
		if len(record.Key) < 11 {
			return fmt.Errorf("%w: service key", ErrServiceCorruptState)
		}
		prefix := string(record.Key[3:11])
		size := 43
		if prefix == "delitm:1" {
			size++
		}
		if len(record.Key) != size {
			return fmt.Errorf("%w: %s key length", ErrServiceCorruptState, prefix)
		}
		id := [32]byte(record.Key[11:43])
		if id == ([32]byte{}) {
			return fmt.Errorf("%w: zero service id", ErrServiceCorruptState)
		}
		var err error
		switch prefix {
		case "offer::1":
			_, _, err = decodeServiceView(record.Value, id, types.DecodeServiceOffer)
		case "agree::1":
			_, _, err = decodeServiceView(record.Value, id, types.DecodeServiceAgreement)
		case "commit:1":
			_, _, err = decodeServiceView(record.Value, id, types.DecodeServiceCommitment)
		case "execut:1":
			_, _, err = decodeServiceView(record.Value, id, types.DecodeServiceExecution)
		case "disput:1":
			_, _, err = decodeServiceView(record.Value, id, types.DecodeServiceDispute)
		case "progrs:1":
			var progress types.ServiceProgress
			progress, _, err = decodeServiceView(record.Value, id, types.DecodeServiceProgress)
			if err == nil && progress.ProgressBPS > 10000 {
				err = fmt.Errorf("%w: progress bounds", ErrServiceCorruptState)
			}
		case "delivr:1":
			_, _, err = readServiceDelivery(get, id)
		case "delitm:1":
			header := get(serviceKey("delivr:1", id))
			if len(header) != 138 || record.Key[43] >= header[129] || len(record.Value) != 73 || record.Value[0] != 1 {
				err = fmt.Errorf("%w: orphan or invalid delivery item", ErrServiceCorruptState)
			}
		default:
			err = fmt.Errorf("%w: unknown service prefix", ErrServiceCorruptState)
		}
		if err != nil {
			return err
		}
		if record.Value == nil {
			return fmt.Errorf("%w: nil service record", ErrServiceCorruptState)
		}
	}
	return nil
}

func (k Keeper) updateServiceDeliveryIndex(ctx sdk.Context, id [32]byte) error {
	record, found, err := k.ServiceViews().GetDelivery(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: writer delivery missing", ErrServiceCorruptState)
	}
	store := ctx.KVStore(k.storeKey)
	key := serviceIndexKey(serviceLatestPrefix, record.AgreementID)
	old := store.Get(key)
	if old != nil && len(old) != 40 {
		return fmt.Errorf("%w: latest delivery index", ErrServiceCorruptState)
	}
	// Equal sequence ties match the kernel's ascending-key scan: greatest id wins.
	if old != nil && (binary.BigEndian.Uint64(old[:8]) > record.GlobalSequence ||
		(binary.BigEndian.Uint64(old[:8]) == record.GlobalSequence && bytes.Compare(old[8:], id[:]) > 0)) {
		return nil
	}
	value := make([]byte, 40)
	binary.BigEndian.PutUint64(value[:8], record.GlobalSequence)
	copy(value[8:], id[:])
	store.Set(key, value)
	return nil
}

func (k Keeper) updateServiceProgressIndex(ctx sdk.Context, id [32]byte) error {
	record, found, err := k.ServiceViews().GetProgress(ctx, id)
	if err != nil {
		return err
	}
	if !found || record.ProgressBPS > 10000 {
		return fmt.Errorf("%w: writer progress", ErrServiceCorruptState)
	}
	store := ctx.KVStore(k.storeKey)
	key := serviceIndexKey(serviceProgressPrefix, record.CommitmentID)
	old := store.Get(key)
	if old != nil && (len(old) != 4 || binary.BigEndian.Uint32(old) == 0 || binary.BigEndian.Uint32(old) > 10000) {
		return fmt.Errorf("%w: progress index", ErrServiceCorruptState)
	}
	if old != nil && binary.BigEndian.Uint32(old) > record.ProgressBPS {
		return nil
	}
	value := make([]byte, 4)
	binary.BigEndian.PutUint32(value, record.ProgressBPS)
	store.Set(key, value)
	return nil
}

// RebuildServiceIndexes is an explicit migration/recovery operation. Queries
// never rebuild, scan, repair or mutate state. Failure publishes no writes.
func (k Keeper) RebuildServiceIndexes(ctx sdk.Context) error {
	var records []kv.Pair
	iterator := sdk.KVStorePrefixIterator(ctx.KVStore(k.hostKey), types.ServiceKVPrefix)
	for ; iterator.Valid(); iterator.Next() {
		records = append(records, kv.Pair{Key: bytes.Clone(iterator.Key()), Value: bytes.Clone(iterator.Value())})
	}
	iterator.Close()
	if err := validateServiceRecords(records); err != nil {
		return err
	}
	branch, commit := ctx.CacheContext()
	store := branch.KVStore(k.storeKey)
	for _, prefix := range []string{serviceLatestPrefix, serviceProgressPrefix} {
		var keys [][]byte
		it := sdk.KVStorePrefixIterator(store, []byte(prefix))
		for ; it.Valid(); it.Next() {
			keys = append(keys, bytes.Clone(it.Key()))
		}
		it.Close()
		for _, key := range keys {
			store.Delete(key)
		}
	}
	for _, record := range records {
		id := [32]byte(record.Key[11:43])
		var err error
		switch string(record.Key[3:11]) {
		case "delivr:1":
			err = k.updateServiceDeliveryIndex(branch, id)
		case "progrs:1":
			err = k.updateServiceProgressIndex(branch, id)
		}
		if err != nil {
			return err
		}
	}
	store.Set([]byte(serviceIndexVersionKey), []byte{1})
	commit()
	return nil
}
