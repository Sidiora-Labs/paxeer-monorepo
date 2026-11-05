package keeper

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const serviceKeyBytes = 40

var ErrServiceCorruptState = errors.New("LXP_NON_CANONICAL: corrupt service state")
var ErrServiceIndexesUnavailable = errors.New("LXP_NON_CANONICAL: service indexes require rebuild")

func serviceKey(prefix string, id [32]byte) []byte {
	key := make([]byte, serviceKeyBytes)
	copy(key[:8], prefix)
	copy(key[8:], id[:])
	return types.KernelKVKey(types.ServiceModuleID, key)
}

// ServiceViews exposes the error-bearing read boundary for service consumers.
// Absence returns a zero value, false, nil; corrupt bytes return a typed error.
type ServiceViews struct{ keeper Keeper }

func (k Keeper) ServiceViews() ServiceViews { return ServiceViews{keeper: k} }

func decodeServiceView[T any](bz []byte, id [32]byte, decode func([]byte) (T, error)) (T, bool, error) {
	var zero T
	if bz == nil {
		return zero, false, nil
	}
	record, err := decode(bz)
	if err != nil {
		return zero, false, fmt.Errorf("%w: %v", ErrServiceCorruptState, err)
	}
	if len(bz) < 33 || !bytes.Equal(bz[1:33], id[:]) || !serviceFieldsCanonical(record) {
		return zero, false, fmt.Errorf("%w: record identifier", ErrServiceCorruptState)
	}
	return record, true, nil
}

func (v ServiceViews) GetOffer(ctx sdk.Context, id [32]byte) (types.ServiceOffer, bool, error) {
	return decodeServiceView(ctx.KVStore(v.keeper.hostKey).Get(serviceKey("offer::1", id)), id, types.DecodeServiceOffer)
}

func (v ServiceViews) GetAgreement(ctx sdk.Context, id [32]byte) (types.ServiceAgreement, bool, error) {
	return decodeServiceView(ctx.KVStore(v.keeper.hostKey).Get(serviceKey("agree::1", id)), id, types.DecodeServiceAgreement)
}

func (v ServiceViews) GetCommitment(ctx sdk.Context, id [32]byte) (types.ServiceCommitment, bool, error) {
	return decodeServiceView(ctx.KVStore(v.keeper.hostKey).Get(serviceKey("commit:1", id)), id, types.DecodeServiceCommitment)
}

func (v ServiceViews) GetExecution(ctx sdk.Context, id [32]byte) (types.ServiceExecution, bool, error) {
	return decodeServiceView(ctx.KVStore(v.keeper.hostKey).Get(serviceKey("execut:1", id)), id, types.DecodeServiceExecution)
}

func (v ServiceViews) GetProgress(ctx sdk.Context, id [32]byte) (types.ServiceProgress, bool, error) {
	return decodeServiceView(ctx.KVStore(v.keeper.hostKey).Get(serviceKey("progrs:1", id)), id, types.DecodeServiceProgress)
}

func (v ServiceViews) GetDispute(ctx sdk.Context, id [32]byte) (types.ServiceDispute, bool, error) {
	return decodeServiceView(ctx.KVStore(v.keeper.hostKey).Get(serviceKey("disput:1", id)), id, types.DecodeServiceDispute)
}

func (v ServiceViews) GetDelivery(ctx sdk.Context, id [32]byte) (types.ServiceDelivery, bool, error) {
	return readServiceDelivery(func(key []byte) []byte { return ctx.KVStore(v.keeper.hostKey).Get(key) }, id)
}

func readServiceDelivery(get func([]byte) []byte, id [32]byte) (types.ServiceDelivery, bool, error) {
	header := get(serviceKey("delivr:1", id))
	if header == nil {
		return types.ServiceDelivery{}, false, nil
	}
	if len(header) != 138 || header[0] != 1 || [32]byte(header[33:65]) == ([32]byte{}) || header[129] == 0 || int(header[129]) > types.ServiceMaxDeliverables {
		return types.ServiceDelivery{}, false, fmt.Errorf("%w: delivery header", ErrServiceCorruptState)
	}
	bz := append([]byte(nil), header...)
	key := append(serviceKey("delitm:1", id), byte(0))
	for i := byte(0); i < header[129]; i++ {
		key[len(key)-1] = i
		item := get(key)
		if len(item) != 73 || item[0] != 1 {
			return types.ServiceDelivery{}, false, fmt.Errorf("%w: delivery item %d", ErrServiceCorruptState, i)
		}
		bz = append(bz, item...)
	}
	return decodeServiceView(bz, id, types.DecodeServiceDelivery)
}

func (v ServiceViews) LatestDelivery(ctx sdk.Context, agreementID [32]byte) (types.ServiceDelivery, bool, error) {
	store := ctx.KVStore(v.keeper.storeKey)
	if !bytes.Equal(store.Get([]byte(serviceIndexVersionKey)), []byte{1}) {
		_, found, err := v.GetAgreement(ctx, agreementID)
		if err != nil {
			return types.ServiceDelivery{}, false, err
		}
		if !found {
			return types.ServiceDelivery{}, false, nil
		}
		return types.ServiceDelivery{}, false, ErrServiceIndexesUnavailable
	}
	bz := store.Get(serviceIndexKey(serviceLatestPrefix, agreementID))
	if bz == nil {
		return types.ServiceDelivery{}, false, nil
	}
	if len(bz) != 40 {
		return types.ServiceDelivery{}, false, fmt.Errorf("%w: latest delivery index", ErrServiceCorruptState)
	}
	record, found, err := v.GetDelivery(ctx, [32]byte(bz[8:40]))
	if err != nil {
		return types.ServiceDelivery{}, false, err
	}
	if !found || record.AgreementID != agreementID || record.GlobalSequence != binary.BigEndian.Uint64(bz[:8]) {
		return types.ServiceDelivery{}, false, fmt.Errorf("%w: latest delivery index target", ErrServiceCorruptState)
	}
	return record, true, nil
}

func (v ServiceViews) ProgressHighWater(ctx sdk.Context, commitmentID [32]byte) (uint32, bool, error) {
	store := ctx.KVStore(v.keeper.storeKey)
	if !bytes.Equal(store.Get([]byte(serviceIndexVersionKey)), []byte{1}) {
		_, found, err := v.GetCommitment(ctx, commitmentID)
		if err != nil {
			return 0, false, err
		}
		if !found {
			return 0, false, nil
		}
		return 0, false, ErrServiceIndexesUnavailable
	}
	bz := store.Get(serviceIndexKey(serviceProgressPrefix, commitmentID))
	if bz == nil {
		return 0, false, nil
	}
	if len(bz) != 4 || binary.BigEndian.Uint32(bz) == 0 || binary.BigEndian.Uint32(bz) > 10000 {
		return 0, false, fmt.Errorf("%w: progress index", ErrServiceCorruptState)
	}
	return binary.BigEndian.Uint32(bz), true, nil
}

// Legacy two-result views retain their invariant-panic contract until the
// precompile consumer migrates to ServiceViews, which returns explicit errors.
func legacyServiceView[T any](record T, found bool, err error) (T, bool) {
	if err != nil {
		panic(err)
	}
	return record, found
}

func (k Keeper) GetOffer(ctx sdk.Context, id [32]byte) (types.ServiceOffer, bool) {
	return legacyServiceView(k.ServiceViews().GetOffer(ctx, id))
}

func (k Keeper) GetAgreement(ctx sdk.Context, id [32]byte) (types.ServiceAgreement, bool) {
	return legacyServiceView(k.ServiceViews().GetAgreement(ctx, id))
}

func (k Keeper) GetCommitment(ctx sdk.Context, id [32]byte) (types.ServiceCommitment, bool) {
	return legacyServiceView(k.ServiceViews().GetCommitment(ctx, id))
}

func (k Keeper) GetExecution(ctx sdk.Context, id [32]byte) (types.ServiceExecution, bool) {
	return legacyServiceView(k.ServiceViews().GetExecution(ctx, id))
}

func (k Keeper) GetProgress(ctx sdk.Context, id [32]byte) (types.ServiceProgress, bool) {
	return legacyServiceView(k.ServiceViews().GetProgress(ctx, id))
}

func (k Keeper) GetDispute(ctx sdk.Context, id [32]byte) (types.ServiceDispute, bool) {
	return legacyServiceView(k.ServiceViews().GetDispute(ctx, id))
}

func (k Keeper) GetDelivery(ctx sdk.Context, id [32]byte) (types.ServiceDelivery, bool) {
	return legacyServiceView(k.ServiceViews().GetDelivery(ctx, id))
}

func (k Keeper) LatestDelivery(ctx sdk.Context, id [32]byte) (types.ServiceDelivery, bool) {
	return legacyServiceView(k.ServiceViews().LatestDelivery(ctx, id))
}

func (k Keeper) ProgressHighWater(ctx sdk.Context, id [32]byte) (uint32, bool) {
	return legacyServiceView(k.ServiceViews().ProgressHighWater(ctx, id))
}

func serviceFieldsCanonical(value any) bool {
	nonzero := func(ids ...[32]byte) bool {
		for _, id := range ids {
			if id == ([32]byte{}) {
				return false
			}
		}
		return true
	}
	hashes := func(ids [][32]byte, required bool) bool {
		if required && len(ids) == 0 {
			return false
		}
		for i, id := range ids {
			if !nonzero(id) {
				return false
			}
			for j := 0; j < i; j++ {
				if ids[j] == id {
					return false
				}
			}
		}
		return true
	}
	switch r := value.(type) {
	case types.ServiceOffer:
		return nonzero(r.OfferID, r.ActivityID, r.OfferingAgent, r.AssetID, r.TermsHash, r.DeliverableSpecHash) && !r.Price.IsZero() && r.DeliveryDeadlineMs != 0 && r.AcceptanceWindowMs != 0 && r.DisputeWindowMs != 0 && r.OfferExpiryMs != 0 && r.DefaultOutcome >= 1 && r.DefaultOutcome <= 2
	case types.ServiceAgreement:
		return nonzero(r.AgreementID, r.OfferID, r.Provider, r.Buyer, r.TermsHash) && r.DefaultOutcome >= 1 && r.DefaultOutcome <= 2 && r.State >= 1 && r.State <= 8 && hashes(r.ContestedHashes, false)
	case types.ServiceCommitment:
		return nonzero(r.CommitmentID, r.ActivityID, r.Provider, r.AgreementID, r.TaskHash) && r.DeadlineMs != 0 && r.ResourceBound != 0 && (r.Abandoned == (r.AbandonReason != 0))
	case types.ServiceProgress:
		return nonzero(r.ReportID, r.ActivityID, r.CommitmentID, r.AgreementID, r.Provider, r.NoteHash, r.AvailabilityReference) && r.ProgressBPS > 0 && r.ProgressBPS <= 10000
	case types.ServiceDelivery:
		if !nonzero(r.AgreementID, r.Provider) {
			return false
		}
		for _, item := range r.Deliverables {
			if !nonzero(item.Hash, item.AvailabilityReference) || item.ArtifactSize == 0 {
				return false
			}
		}
		return len(r.Deliverables) > 0
	case types.ServiceDispute:
		return nonzero(r.DisputeID, r.ActivityID, r.AgreementID, r.Raiser) && r.ProviderBPS <= 10000 && hashes(r.EvidenceHashes, true) && (!r.Resolved || (r.Ruling != 0 && nonzero(r.EscrowResolutionID)))
	case types.ServiceExecution:
		return nonzero(r.AttestationID, r.ActivityID, r.AgreementID, r.CommitmentID, r.ToolID, r.InputCommitmentHash, r.OutputCommitmentHash, r.AttestorIdentity, r.AvailabilityReference, r.PublicKey) && r.ExecutionEnd >= r.ExecutionStart
	}
	return false
}
